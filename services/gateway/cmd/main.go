package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/config"
	"github.com/ItsThompson/gofin/services/gateway/internal/router"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run("8080"))
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api-gateway: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up structured logging via serverkit (JSON slog handler with the
	// "service" attribute attached to every record). Installed as the default so
	// slog.Warn call sites (e.g. config.Load's oversized-timeout warning) share
	// the same handler.
	logger := serverkit.NewLogger(cfg.LogLevel, "gateway")
	slog.SetDefault(logger)

	// Establish gRPC connection to the auth service for token validation.
	grpcConn, err := grpc.NewClient(
		cfg.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to auth service gRPC at %s: %w", cfg.AuthServiceAddr, err)
	}
	defer func() { _ = grpcConn.Close() }()

	logger.Info("gRPC client configured",
		slog.String("auth_service_addr", cfg.AuthServiceAddr),
		slog.Duration("validate_timeout", cfg.ValidateTimeout),
	)

	// Create the token validator wrapping the gRPC client. Its per-call timeout
	// bounds ValidateToken so a hung auth service cannot block a worker.
	validator := NewGRPCTokenValidator(grpcConn, cfg.ValidateTimeout)

	// Parse downstream service URLs.
	authURL, err := url.Parse(cfg.AuthServiceREST)
	if err != nil {
		return fmt.Errorf("parsing AUTH_SERVICE_REST: %w", err)
	}
	expenseURL, err := url.Parse(cfg.ExpenseServiceREST)
	if err != nil {
		return fmt.Errorf("parsing EXPENSE_SERVICE_REST: %w", err)
	}
	financeURL, err := url.Parse(cfg.FinanceServiceREST)
	if err != nil {
		return fmt.Errorf("parsing FINANCE_SERVICE_REST: %w", err)
	}
	datarightsURL, err := url.Parse(cfg.DatarightsServiceREST)
	if err != nil {
		return fmt.Errorf("parsing DATARIGHTS_SERVICE_REST: %w", err)
	}

	// Build the Gin router with all routes and middleware.
	engine := router.New(validator, &router.ServiceURLs{
		AuthREST:       authURL,
		ExpenseREST:    expenseURL,
		FinanceREST:    financeURL,
		DatarightsREST: datarightsURL,
	}, sharedaccess.Prefixes(), logger, cfg.IsProduction())

	// Start the HTTP server.
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: engine,
	}

	logger.Info("API gateway starting",
		slog.String("port", cfg.Port),
		slog.String("auth_rest", cfg.AuthServiceREST),
		slog.String("expense_rest", cfg.ExpenseServiceREST),
		slog.String("finance_rest", cfg.FinanceServiceREST),
		slog.String("datarights_rest", cfg.DatarightsServiceREST),
	)

	// serverkit.Serve owns the serve/shutdown lifecycle and returns any fatal
	// bind error so run() exits non-zero instead of lingering with no listener
	// (the C5 zombie bug). The gateway runs no gRPC server, so both gRPC args
	// are nil.
	return serverkit.Serve(ctx, server, nil, nil)
}
