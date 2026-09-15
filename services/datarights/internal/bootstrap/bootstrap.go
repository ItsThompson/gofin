// Package bootstrap assembles datarights dependencies and owns startup order.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
	"github.com/ItsThompson/gofin/services/datarights/proto/datarightspb"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/serverkit"
)

// Run loads configuration, binds the internal gRPC listener, then starts the
// remaining service dependencies and servers.
func Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	return run(ctx)
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	return runWithDependencies(ctx, cfg, net.Listen, start)
}

type listenerBinder func(network, address string) (net.Listener, error)
type starter func(context.Context, *config.Config, net.Listener) error

func runWithDependencies(ctx context.Context, cfg *config.Config, bind listenerBinder, start starter) error {
	grpcLis, err := bind("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}
	defer func() { _ = grpcLis.Close() }()

	return start(ctx, cfg, grpcLis)
}

func start(ctx context.Context, cfg *config.Config, grpcLis net.Listener) error {
	logger := serverkit.NewLogger(cfg.LogLevel, "datarights")
	slog.SetDefault(logger)

	if err := serverkit.InitSentry(serverkit.SentryConfigFromEnv("datarights")); err != nil {
		logger.Error("sentry initialization failed, error reporting is disabled",
			slog.String("error", err.Error()),
		)
	}

	pool, err := serverkit.ConnectPostgres(ctx, cfg.DBUrl, migrations.FS)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connecting to auth service gRPC at %s: %w", cfg.AuthServiceAddr, err)
	}
	defer func() { _ = authConn.Close() }()
	authClient := authpb.NewAuthServiceClient(authConn)

	expenseConn, err := grpc.NewClient(cfg.ExpenseServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connecting to expense service gRPC at %s: %w", cfg.ExpenseServiceAddr, err)
	}
	defer func() { _ = expenseConn.Close() }()
	expenseClient := expensepb.NewExpenseServiceClient(expenseConn)

	financeConn, err := grpc.NewClient(cfg.FinanceServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connecting to finance service gRPC at %s: %w", cfg.FinanceServiceAddr, err)
	}
	defer func() { _ = financeConn.Close() }()
	financeClient := financepb.NewFinanceServiceClient(financeConn)

	repo := repository.NewPostgresJobRepository(pool)
	newExportProviders := func(financeData *financepb.AllUserDataResponse) []engine.DataProvider {
		return []engine.DataProvider{
			providers.NewProfileProvider(authClient),
			providers.NewExpensesProvider(expenseClient, providers.BuildTagMap(financeData), providers.BuildPeriodCurrencyMap(financeData)),
			providers.NewTagsProvider(financeData),
			providers.NewBudgetPeriodsProvider(financeData),
			providers.NewDefaultSettingsProvider(financeData),
		}
	}

	emailSender, err := buildEmailSender(cfg, logger)
	if err != nil {
		return fmt.Errorf("setting up email sender: %w", err)
	}

	exportEngine := engine.NewEngine(newExportProviders, financeClient, repo, emailSender, cfg.MaxConcurrent, cfg.ExportTimeout, logger)
	exportmetrics.SetPoolStats(exportEngine.ActiveJobs, exportEngine.QueuedJobs)

	emailResolver := service.NewAuthUserEmailResolver(authClient)
	recoverJobs(ctx, logger, "export", repo.GetNonTerminalJobs, func(ctx context.Context, job model.RecoverableJob) {
		userEmail, err := emailResolver.ResolveEmail(ctx, job.UserID)
		if err != nil {
			_ = errkit.Report(ctx, err, errkit.Meta{
				Op: "datarights.recover_jobs", Domain: "datarights", Msg: "failed to resolve email for recovered job",
				Data: map[string]any{"job_id": job.ID, "user_id": job.UserID, "kind": "export"},
			})
		}
		logger.Info("re-submitting job", slog.String("job_id", job.ID), slog.String("user_id", job.UserID))
		exportEngine.Submit(job.ID, job.UserID, userEmail)
	})

	exportSvc := service.NewExportService(repo, logger, service.WithEngine(exportEngine), service.WithEmailResolver(emailResolver))
	deletionRepo := repository.NewPostgresDeletionJobRepository(pool)
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

	deletionEngine := deletion.NewEngine(deletionRegistry, deletionRepo, cfg.MaxConcurrent, cfg.DeletionTimeout, logger)
	recoverJobs(ctx, logger, "deletion", deletionRepo.GetNonTerminalJobs, func(_ context.Context, job model.RecoverableDeletionJob) {
		logger.Info("re-submitting deletion job", slog.String("job_id", job.ID), slog.String("user_id", job.UserID))
		deletionEngine.Submit(job.ID, job.UserID)
	})

	deletionSvc := service.NewDeletionService(deletionRepo, logger,
		service.WithDeletionEngine(deletionEngine), service.WithAuthClient(authClient), service.WithExportRepo(repo),
		service.WithProtectedUsernames(cfg.ProtectedUsernames),
	)

	router := serverkit.NewRouter("datarights", cfg.IsProduction())
	handler.RegisterRoutes(router, handler.NewRESTHandler(exportSvc, logger), handler.NewDeletionHandler(deletionSvc, logger))
	httpServer := &http.Server{Addr: ":" + cfg.RESTPort, Handler: router}

	exportMetricsService := service.NewExportMetricsService(repo, logger)
	grpcServer := serverkit.NewGRPCServer()
	datarightspb.RegisterDatarightsServiceServer(grpcServer, handler.NewGRPCHandler(exportMetricsService))

	logger.Info("datarights service ready",
		slog.String("rest_port", cfg.RESTPort), slog.String("grpc_port", cfg.GRPCPort),
		slog.Int("max_concurrent_exports", cfg.MaxConcurrent), slog.Duration("export_timeout", cfg.ExportTimeout),
	)
	return serverkit.Serve(ctx, httpServer, grpcServer, grpcLis)
}

func recoverJobs[J any](ctx context.Context, logger *slog.Logger, kind string, fetch func(context.Context) ([]J, error), submit func(context.Context, J)) {
	jobs, err := fetch(ctx)
	if err != nil {
		_ = errkit.Report(ctx, err, errkit.Meta{Op: "datarights.recover_jobs", Domain: "datarights", Msg: "failed to query recoverable jobs", Data: map[string]any{"kind": kind}})
		return
	}
	if len(jobs) == 0 {
		return
	}
	logger.Info("recovering non-terminal jobs", slog.String("kind", kind), slog.Int("count", len(jobs)))
	for _, job := range jobs {
		submit(ctx, job)
	}
}

func buildEmailSender(cfg *config.Config, logger *slog.Logger) (email.Sender, error) {
	if !cfg.EmailEnabled {
		logger.Info("email disabled: using log sender", slog.String("mode", "dev_log_only"))
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
	logger.Info("email enabled: using Resend sender", slog.String("from", cfg.EmailFrom))
	return sender, nil
}
