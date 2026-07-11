package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/db/migrations"
	"github.com/ItsThompson/gofin/services/datarights/internal/config"
	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine/providers"
	"github.com/ItsThompson/gofin/services/datarights/internal/handler"
	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.RESTPort()))
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
	logger := serverkit.NewLogger(cfg.LogLevel, "datarights")
	slog.SetDefault(logger)

	// Connect to PostgreSQL (runs embedded migrations, opens the pool, pings).
	pool, err := serverkit.ConnectPostgres(ctx, cfg.DBUrl, migrations.FS)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()
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

	// Expose the export pool's live telemetry through the Prometheus pool gauges
	// (scraped by the Grafana dashboard and the stuck-pool alert).
	exportmetrics.SetPoolStats(exportEngine.ActiveJobs, exportEngine.QueuedJobs)

	// Startup recovery: re-submit non-terminal export jobs. Export resolves the
	// user's email first; a failure is logged and the job is submitted anyway (it
	// will fail with a descriptive error at the email step).
	emailResolver := service.NewAuthUserEmailResolver(authClient)
	recoverJobs(ctx, logger, "export", repo.GetNonTerminalJobs, func(ctx context.Context, job model.RecoverableJob) {
		userEmail, err := emailResolver.ResolveEmail(ctx, job.UserID)
		if err != nil {
			logger.Error("failed to resolve email for recovered job",
				slog.String("job_id", job.ID),
				slog.String("user_id", job.UserID),
				slog.String("error", err.Error()),
			)
		}
		logger.Info("re-submitting job",
			slog.String("job_id", job.ID),
			slog.String("user_id", job.UserID),
		)
		exportEngine.Submit(job.ID, job.UserID, userEmail)
	})

	exportSvc := service.NewExportService(repo, logger, service.WithEngine(exportEngine), service.WithEmailResolver(emailResolver))

	// Set up deletion engine with provider registry
	deletionRepo := repository.NewPostgresDeletionJobRepository(pool)

	// Register the deletion providers as name+func pairs. Registration order is
	// execution order: finance and expense first, auth last (a user cannot
	// authenticate once auth data is gone). Each func wraps one idempotent gRPC
	// delete call and discards the response.
	deletionRegistry := deletion.NewRegistry()
	deletionRegistry.Register(deletion.NewFuncProvider("finance", func(ctx context.Context, userID string) error {
		_, err := financeClient.DeleteAllUserData(ctx, &financepb.DeleteAllUserDataRequest{UserId: userID})
		return err
	}))
	deletionRegistry.Register(deletion.NewFuncProvider("expense", func(ctx context.Context, userID string) error {
		_, err := expenseClient.AnonymizeAllUserExpenses(ctx, &expensepb.AnonymizeRequest{UserId: userID})
		return err
	}))
	deletionRegistry.Register(deletion.NewFuncProvider("auth", func(ctx context.Context, userID string) error {
		_, err := authClient.DeleteUserData(ctx, &authpb.DeleteUserDataRequest{UserId: userID})
		return err
	}))

	deletionEngine := deletion.NewEngine(
		deletionRegistry,
		deletionRepo,
		cfg.MaxConcurrent,
		cfg.DeletionTimeout,
		logger,
	)

	// Startup recovery: re-submit non-terminal deletion jobs
	recoverJobs(ctx, logger, "deletion", deletionRepo.GetNonTerminalJobs, func(_ context.Context, job model.RecoverableDeletionJob) {
		logger.Info("re-submitting deletion job",
			slog.String("job_id", job.ID),
			slog.String("user_id", job.UserID),
		)
		deletionEngine.Submit(job.ID, job.UserID)
	})

	deletionSvc := service.NewDeletionService(deletionRepo, logger,
		service.WithDeletionEngine(deletionEngine),
		service.WithAuthClient(authClient),
		service.WithExportRepo(repo),
		service.WithProtectedUsernames(cfg.ProtectedUsernames),
	)

	// Build the shared router (Recovery, HTTP metrics, /metrics, GET /health).
	router := serverkit.NewRouter("datarights", cfg.IsProduction())

	restHandler := handler.NewRESTHandler(exportSvc, logger)
	deletionHandler := handler.NewDeletionHandler(deletionSvc, logger)
	handler.RegisterRoutes(router, restHandler, deletionHandler)

	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	logger.Info("datarights service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.Int("max_concurrent_exports", cfg.MaxConcurrent),
		slog.Duration("export_timeout", cfg.ExportTimeout),
	)

	// Serve blocks until ctx is cancelled or the HTTP server fails to bind.
	// A bind failure returns non-nil so run() (and main) exit non-zero instead
	// of lingering as a zombie with no listener (C5). datarights runs no gRPC
	// server, so both gRPC arguments are nil.
	return serverkit.Serve(ctx, httpServer, nil, nil)
}

// recoverJobs re-submits non-terminal jobs found in the database on startup. It
// shares the query -> empty-check -> per-job submit skeleton across both
// engines; the submit closure supplies the engine-specific step (export
// resolves the user's email before re-submitting).
func recoverJobs[J any](
	ctx context.Context,
	logger *slog.Logger,
	kind string,
	fetch func(context.Context) ([]J, error),
	submit func(context.Context, J),
) {
	jobs, err := fetch(ctx)
	if err != nil {
		logger.Error("failed to query recoverable jobs",
			slog.String("kind", kind),
			slog.String("error", err.Error()),
		)
		return
	}

	if len(jobs) == 0 {
		return
	}

	logger.Info("recovering non-terminal jobs",
		slog.String("kind", kind),
		slog.Int("count", len(jobs)),
	)

	for _, job := range jobs {
		submit(ctx, job)
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
