package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ItsThompson/gofin/services/finance/internal/config"
	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/finance/internal/handler"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	"github.com/ItsThompson/gofin/services/metrics"
	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "finance-service: %v\n", err)
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
	logger = logger.With(slog.String("service", "finance"))
	slog.SetDefault(logger)

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

	// Log the expense service address (connection not required yet per ticket notes)
	logger.Info("expense service configured",
		slog.String("addr", cfg.ExpenseServiceAddr),
	)

	// Connect to expense service gRPC for dashboard aggregation
	expenseConn, err := grpc.NewClient(
		cfg.ExpenseServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to expense service at %s: %w", cfg.ExpenseServiceAddr, err)
	}
	defer expenseConn.Close()
	logger.Info("expense service gRPC client created",
		slog.String("addr", cfg.ExpenseServiceAddr),
	)

	// Build dependency graph
	queries := db.New(pool)
	repo := repository.NewPostgresFinanceRepository(queries)
	txBeginner := repository.NewPostgresTxBeginner(pool)
	expenseClient := service.NewGRPCExpenseClient(
		expensepb.NewExpenseServiceClient(expenseConn),
	)
	financeSvc := service.NewFinanceService(repo, txBeginner, logger).WithExpenseClient(expenseClient)

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()),
	)
	grpcHandler := handler.NewGRPCHandler(financeSvc, logger)
	pb.RegisterFinanceServiceServer(grpcServer, grpcHandler)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	go func() {
		logger.Info("gRPC server starting",
			slog.String("port", cfg.GRPCPort),
		)
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("gRPC server failed", slog.String("error", err.Error()))
		}
	}()

	// Start REST server
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(metrics.HTTPMetrics())

	metrics.Register(router)

	restHandler := handler.NewRESTHandler(financeSvc, logger)
	restHandler.RegisterRoutes(router)

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

	logger.Info("finance service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down finance service")

	// Graceful shutdown: give in-flight requests up to 10 seconds
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("REST server shutdown error", slog.String("error", err.Error()))
	}

	return nil
}
