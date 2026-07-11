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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/db/migrations"
	"github.com/ItsThompson/gofin/services/finance/internal/config"
	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/finance/internal/handler"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.ResolveRESTPort()))
	}

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
	logger := serverkit.NewLogger(cfg.LogLevel, "finance")
	slog.SetDefault(logger)

	// Run embedded migrations, open the pool, and ping it.
	pool, err := serverkit.ConnectPostgres(ctx, cfg.DBUrl, migrations.FS)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// Connect to expense service gRPC for dashboard aggregation
	expenseConn, err := grpc.NewClient(
		cfg.ExpenseServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to expense service at %s: %w", cfg.ExpenseServiceAddr, err)
	}
	defer func() { _ = expenseConn.Close() }()
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
	financeSvc := service.NewFinanceService(repo, txBeginner, expenseClient, time.Now, logger)

	// Build the gRPC server and pre-bind its listener so a bind failure surfaces.
	grpcServer := serverkit.NewGRPCServer()
	grpcHandler := handler.NewGRPCHandler(financeSvc, logger)
	pb.RegisterFinanceServiceServer(grpcServer, grpcHandler)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	// Build the REST router and server.
	router := serverkit.NewRouter("finance", cfg.IsProduction())
	restHandler := handler.NewRESTHandler(financeSvc, logger)
	restHandler.RegisterRoutes(router)

	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	logger.Info("finance service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Serve blocks until ctx is cancelled or a server fails fatally (e.g. a REST
	// bind failure), returning that error so the process exits non-zero (C5).
	return serverkit.Serve(ctx, httpServer, grpcServer, grpcLis)
}
