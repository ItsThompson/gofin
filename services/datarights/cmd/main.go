package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/db/migrations"
	"github.com/ItsThompson/gofin/services/datarights/internal/config"
	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
	deletionproviders "github.com/ItsThompson/gofin/services/datarights/internal/deletion/providers"
	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine/providers"
	"github.com/ItsThompson/gofin/services/datarights/internal/handler"
	_ "github.com/ItsThompson/gofin/services/datarights/internal/metrics" // register Prometheus metrics
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	"github.com/ItsThompson/gofin/services/dbmigrate"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/metrics"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run("8084"))
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "datarights-service: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up structured logging
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	logger = logger.With(slog.String("service", "datarights"))
	slog.SetDefault(logger)

	// Run database migrations (embedded in binary via go:embed)
	if err := dbmigrate.RunWithFS(cfg.DBUrl, migrations.FS, "."); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// Connect to PostgreSQL
	pool, err := pgxpool.New(ctx, cfg.DBUrl)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	logger.Info("connected to PostgreSQL")

	// Connect to auth service gRPC
	authConn, err := grpc.NewClient(
		cfg.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to auth service gRPC at %s: %w", cfg.AuthServiceAddr, err)
	}
	defer authConn.Close() //nolint:errcheck

	logger.Info("gRPC client configured",
		slog.String("auth_service_addr", cfg.AuthServiceAddr),
	)

	authClient := authpb.NewAuthServiceClient(authConn)

	// Connect to expense service gRPC
	expenseConn, err := grpc.NewClient(
		cfg.ExpenseServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to expense service gRPC at %s: %w", cfg.ExpenseServiceAddr, err)
	}
	defer expenseConn.Close() //nolint:errcheck

	expenseClient := expensepb.NewExpenseServiceClient(expenseConn)

	// Connect to finance service gRPC
	financeConn, err := grpc.NewClient(
		cfg.FinanceServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to finance service gRPC at %s: %w", cfg.FinanceServiceAddr, err)
	}
	defer financeConn.Close() //nolint:errcheck

	financeClient := financepb.NewFinanceServiceClient(financeConn)

	logger.Info("gRPC clients configured",
		slog.String("expense_service_addr", cfg.ExpenseServiceAddr),
		slog.String("finance_service_addr", cfg.FinanceServiceAddr),
	)

	// Build dependency graph
	repo := repository.NewPostgresJobRepository(pool)

	// Set up export engine with a per-job provider factory. The factory closes
	// over the auth and expense clients; the finance-backed providers receive a
	// fresh per-job MemoizedFinanceClient (built inside the engine) so a single
	// GetAllUserData call is shared across providers without leaking data across
	// jobs. Registration/ZIP order: profile, expenses, tags, budget_periods,
	// default_settings.
	newExportProviders := func(finance financepb.FinanceServiceClient) []engine.DataProvider {
		return []engine.DataProvider{
			providers.NewProfileProvider(authClient),
			providers.NewExpensesProvider(expenseClient, finance),
			providers.NewTagsProvider(finance),
			providers.NewBudgetPeriodsProvider(finance),
			providers.NewDefaultSettingsProvider(finance),
		}
	}

	// Set up email sender
	emailSender, err := buildEmailSender(cfg, logger)
	if err != nil {
		return fmt.Errorf("setting up email sender: %w", err)
	}

	exportEngine := engine.NewEngine(newExportProviders, financeClient, repo, emailSender, cfg.MaxConcurrent, cfg.ExportTimeout, logger)

	// Startup recovery: re-submit non-terminal export jobs
	emailResolver := service.NewAuthUserEmailResolver(authClient)
	recoverJobs(ctx, repo, exportEngine, emailResolver, logger)

	exportSvc := service.NewExportService(repo, logger, service.WithEngine(exportEngine), service.WithEmailResolver(emailResolver))

	// Set up deletion engine with provider registry
	deletionRepo := repository.NewPostgresDeletionJobRepository(pool)
	deletionRegistry := deletion.NewDeletionProviderRegistry()
	deletionRegistry.Register(deletionproviders.NewFinanceDeletionProvider(financeClient))
	deletionRegistry.Register(deletionproviders.NewExpenseDeletionProvider(expenseClient))
	deletionRegistry.Register(deletionproviders.NewAuthDeletionProvider(authClient))

	deletionEngine := deletion.NewDeletionEngine(
		deletionRegistry,
		deletionRepo,
		cfg.MaxConcurrent,
		cfg.DeletionTimeout,
		logger,
	)

	// Startup recovery: re-submit non-terminal deletion jobs
	recoverDeletionJobs(ctx, deletionRepo, deletionEngine, logger)

	deletionSvc := service.NewDeletionService(deletionRepo, logger,
		service.WithDeletionEngine(deletionEngine),
		service.WithAuthClient(authClient),
		service.WithExportRepo(repo),
	)

	// Start REST server
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(metrics.HTTPMetrics())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"pool": gin.H{
				"active": exportEngine.ActiveJobs(),
				"max":    exportEngine.MaxConcurrent(),
			},
		})
	})

	metrics.Register(router)

	restHandler := handler.NewRESTHandler(exportSvc, logger)
	deletionHandler := handler.NewDeletionHandler(deletionSvc, logger)
	handler.RegisterRoutes(router, restHandler, deletionHandler)

	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	go func() {
		logger.Info("REST server starting",
			slog.String("port", cfg.RESTPort),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("REST server failed", slog.String("error", err.Error()))
		}
	}()

	logger.Info("datarights service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.Int("max_concurrent_exports", cfg.MaxConcurrent),
		slog.Duration("export_timeout", cfg.ExportTimeout),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down datarights service")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("REST server shutdown error", slog.String("error", err.Error()))
	}

	return nil
}

// recoverJobs re-submits any non-terminal jobs found in the database on startup.
func recoverJobs(ctx context.Context, repo repository.JobRepository, eng *engine.Engine, resolver service.UserEmailResolver, logger *slog.Logger) {
	jobs, err := repo.GetNonTerminalJobs(ctx)
	if err != nil {
		logger.Error("failed to query recoverable jobs", slog.String("error", err.Error()))
		return
	}

	if len(jobs) == 0 {
		return
	}

	logger.Info("recovering non-terminal jobs", slog.Int("count", len(jobs)))

	for _, job := range jobs {
		userEmail, err := resolver.ResolveEmail(ctx, job.UserID)
		if err != nil {
			logger.Error("failed to resolve email for recovered job",
				slog.String("job_id", job.ID),
				slog.String("user_id", job.UserID),
				slog.String("error", err.Error()),
			)
			// Submit anyway: will fail at email step with descriptive error
		}

		logger.Info("re-submitting job",
			slog.String("job_id", job.ID),
			slog.String("user_id", job.UserID),
		)
		eng.Submit(job.ID, job.UserID, userEmail)
	}
}

// recoverDeletionJobs re-submits any non-terminal deletion jobs on startup.
func recoverDeletionJobs(ctx context.Context, repo repository.DeletionJobRepository, eng *deletion.DeletionEngine, logger *slog.Logger) {
	jobs, err := repo.GetNonTerminalJobs(ctx)
	if err != nil {
		logger.Error("failed to query recoverable deletion jobs", slog.String("error", err.Error()))
		return
	}

	if len(jobs) == 0 {
		return
	}

	logger.Info("recovering non-terminal deletion jobs", slog.Int("count", len(jobs)))

	for _, job := range jobs {
		logger.Info("re-submitting deletion job",
			slog.String("job_id", job.ID),
			slog.String("user_id", job.UserID),
		)
		eng.Submit(job.ID, job.UserID)
	}
}

// buildEmailSender creates the appropriate email sender based on configuration.
// When EMAIL_ENABLED=false, returns a LogSender that logs email content for dev/test.
func buildEmailSender(cfg *config.Config, logger *slog.Logger) (email.Sender, error) {
	if !cfg.EmailEnabled {
		logger.Info("email disabled: using log sender",
			slog.String("mode", "dev_log_only"),
		)
		return email.NewLogSender(logger), nil
	}

	if cfg.ResendAPIKey == "" {
		return nil, fmt.Errorf("RESEND_API_KEY is required when EMAIL_ENABLED=true")
	}

	tokensData, err := os.ReadFile(cfg.BrandTokensPath)
	if err != nil {
		return nil, fmt.Errorf("reading brand tokens from %s: %w", cfg.BrandTokensPath, err)
	}

	tokens, err := email.LoadBrandTokens(tokensData)
	if err != nil {
		return nil, fmt.Errorf("parsing brand tokens: %w", err)
	}

	sender, err := email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom, tokens, logger)
	if err != nil {
		return nil, fmt.Errorf("creating Resend sender: %w", err)
	}

	logger.Info("email enabled: using Resend sender",
		slog.String("from", cfg.EmailFrom),
	)

	return sender, nil
}
