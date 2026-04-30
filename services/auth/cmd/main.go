package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/thompsnt/gofin/services/auth/internal/config"
	"github.com/thompsnt/gofin/services/auth/internal/db"
	"github.com/thompsnt/gofin/services/auth/internal/handler"
	"github.com/thompsnt/gofin/services/auth/internal/repository"
	"github.com/thompsnt/gofin/services/auth/internal/service"
	pb "github.com/thompsnt/gofin/services/auth/proto/authpb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "auth-service: %v\n", err)
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
	logger.Info("connected to PostgreSQL", slog.String("service", "auth"))

	// Build dependency graph
	queries := db.New(pool)
	repo := repository.NewPostgresUserRepository(queries)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	pwdSvc := service.NewPasswordService(cfg.BcryptCost)
	authSvc := service.NewAuthService(repo, jwtSvc, pwdSvc, logger)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handler.NewGRPCHandler(authSvc, logger)
	pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	go func() {
		logger.Info("gRPC server starting",
			slog.String("service", "auth"),
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

	restHandler := handler.NewRESTHandler(authSvc, logger, cfg.IsProduction())
	restHandler.RegisterRoutes(router)

	go func() {
		addr := ":" + cfg.RESTPort
		logger.Info("REST server starting",
			slog.String("service", "auth"),
			slog.String("port", cfg.RESTPort),
		)
		if err := router.Run(addr); err != nil {
			logger.Error("REST server failed", slog.String("error", err.Error()))
		}
	}()

	logger.Info("auth service ready",
		slog.String("service", "auth"),
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down auth service", slog.String("service", "auth"))
	grpcServer.GracefulStop()

	return nil
}
