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

	"github.com/ItsThompson/gofin/services/auth/internal/config"
	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/auth/internal/handler"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/metrics"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

func main() {
	// Support subcommands: "seed-admin" runs the admin seeder and exits.
	if len(os.Args) > 1 && os.Args[1] == "seed-admin" {
		if err := runSeedAdmin(); err != nil {
			fmt.Fprintf(os.Stderr, "seed-admin: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
	logger = logger.With(slog.String("service", "auth"))
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

	// Build dependency graph
	queries := db.New(pool)
	repo := repository.NewPostgresUserRepository(queries)
	blacklistRepo := repository.NewPostgresBlacklistRepository(queries)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	pwdSvc := service.NewPasswordService(cfg.BcryptCost)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()),
	)
	grpcHandler := handler.NewGRPCHandler(authSvc, logger)
	pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

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

	restHandler := handler.NewRESTHandler(authSvc, logger, cfg.IsProduction(), cfg.CookieDomain)
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

	logger.Info("auth service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down auth service")

	// Graceful shutdown: give in-flight requests up to 10 seconds
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("REST server shutdown error", slog.String("error", err.Error()))
	}

	return nil
}

// runSeedAdmin creates an admin user from environment variables.
// Idempotent: skips if the admin user already exists.
func runSeedAdmin() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger = logger.With(slog.String("service", "auth"), slog.String("command", "seed-admin"))

	// Read admin credentials from env
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminUsername == "" || adminEmail == "" || adminPassword == "" {
		return fmt.Errorf("ADMIN_USERNAME, ADMIN_EMAIL, and ADMIN_PASSWORD must all be set")
	}

	pool, err := pgxpool.New(ctx, cfg.DBUrl)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	queries := db.New(pool)
	repo := repository.NewPostgresUserRepository(queries)
	blacklistRepo := repository.NewPostgresBlacklistRepository(queries)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	pwdSvc := service.NewPasswordService(cfg.BcryptCost)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	if err := authSvc.SeedAdmin(ctx, adminUsername, adminEmail, adminPassword); err != nil {
		return fmt.Errorf("seeding admin: %w", err)
	}

	logger.Info("seed-admin completed successfully")
	return nil
}
