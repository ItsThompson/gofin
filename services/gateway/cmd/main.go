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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ItsThompson/gofin/services/gateway/internal/config"
	"github.com/ItsThompson/gofin/services/gateway/internal/router"
	"github.com/ItsThompson/gofin/services/healthcheck"
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

	// Set up structured logging.
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
	logger = logger.With(slog.String("service", "gateway"))
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
	}, logger, cfg.IsProduction())

	// Start the HTTP server.
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: engine,
	}

	go func() {
		logger.Info("API gateway starting",
			slog.String("port", cfg.Port),
			slog.String("auth_rest", cfg.AuthServiceREST),
			slog.String("expense_rest", cfg.ExpenseServiceREST),
			slog.String("finance_rest", cfg.FinanceServiceREST),
			slog.String("datarights_rest", cfg.DatarightsServiceREST),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	logger.Info("shutting down API gateway")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
	}

	return nil
}
