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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/auth/db/migrations"
	"github.com/ItsThompson/gofin/services/auth/internal/config"
	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/auth/internal/handler"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"

	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

func main() {
	// Support subcommands: "--healthcheck" checks the health endpoint and exits.
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.RESTPort()))
	}

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
	logger := serverkit.NewLogger(cfg.LogLevel, "auth")
	slog.SetDefault(logger)

	// Run migrations, connect to PostgreSQL, and ping (caller owns pool.Close).
	pool, err := serverkit.ConnectPostgres(ctx, cfg.DBUrl, migrations.FS)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// Build dependency graph
	queries := db.New(pool)
	repo := repository.NewPostgresUserRepository(queries)
	blacklistRepo := repository.NewPostgresBlacklistRepository(queries)
	jwtSvc := service.NewJWTService(cfg.JWTSecret,
		service.WithAccessTTL(cfg.JWTAccessTTL),
		service.WithRefreshTTL(cfg.JWTRefreshTTL),
	)
	pwdSvc := service.NewPasswordService(cfg.BcryptCost)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	// Start background workers
	authSvc.StartPeriodicCleanup(ctx, cfg.CleanupInterval, cfg.CleanupTimeout)

	// Build the gRPC server
	grpcServer := serverkit.NewGRPCServer()
	grpcHandler := handler.NewGRPCHandler(authSvc, logger)
	pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	// Build the REST server
	router := serverkit.NewRouter("auth", cfg.IsProduction())
	restHandler := handler.NewRESTHandler(authSvc, logger, cfg.IsProduction(), cfg.CookieDomain, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	restHandler.RegisterRoutes(router)

	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	logger.Info("auth service ready",
		slog.String("rest_port", cfg.RESTPort),
		slog.String("grpc_port", cfg.GRPCPort),
	)

	// Serve blocks until ctx is cancelled or a server fails to bind; a fatal
	// serve error propagates so run() exits non-zero instead of leaving a zombie process.
	return serverkit.Serve(ctx, httpServer, grpcServer, grpcLis)
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

	logger := serverkit.NewLogger(cfg.LogLevel, "auth").With(slog.String("command", "seed-admin"))

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
