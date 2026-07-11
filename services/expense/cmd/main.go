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

	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/handler"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.ResolveRESTPort()))
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "expense-service: %v\n", err)
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
	logger := serverkit.NewLogger(cfg.LogLevel, "expense")
	slog.SetDefault(logger)

	// Connect to immudb. expense is immudb-backed (not Postgres), so it does not
	// use serverkit.ConnectPostgres.
	immudbClient, err := connectImmudb(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("connecting to immudb: %w", err)
	}
	logger.Info("connected to immudb",
		slog.String("addr", cfg.ImmudbAddr),
	)

	// Initialize schema (CREATE TABLE IF NOT EXISTS)
	repo := repository.NewImmudbExpenseRepository(immudbClient, logger)
	if err := repo.InitSchema(ctx); err != nil {
		return fmt.Errorf("initializing schema: %w", err)
	}

	// Build dependency graph
	expenseSvc := service.NewExpenseService(repo, logger)

	// Build the gRPC server and pre-bind its listener so a bind failure surfaces.
	grpcServer := serverkit.NewGRPCServer()
	grpcHandler := handler.NewGRPCHandler(expenseSvc, logger)
	pb.RegisterExpenseServiceServer(grpcServer, grpcHandler)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	// Build the REST router and server.
	router := serverkit.NewRouter("expense", cfg.IsProduction())
	restHandler := handler.NewRESTHandler(expenseSvc, logger)
	restHandler.RegisterRoutes(router)

	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	logger.Info("expense service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Serve blocks until ctx is cancelled or a server fails fatally (e.g. a REST
	// bind failure), returning that error so the process exits non-zero (C5).
	return serverkit.Serve(ctx, httpServer, grpcServer, grpcLis)
}
