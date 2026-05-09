package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/datarights/internal/config"
	"github.com/ItsThompson/gofin/services/datarights/internal/handler"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	"github.com/ItsThompson/gofin/services/metrics"
)

func main() {
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
	logger = logger.With(slog.String("service", "datarights"))
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
	repo := repository.NewPostgresJobRepository(pool)
	exportSvc := service.NewExportService(repo, logger)

	// Start REST server
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(metrics.HTTPMetrics())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	metrics.Register(router)

	restHandler := handler.NewRESTHandler(exportSvc, logger)
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

	logger.Info("datarights service ready",
		slog.String("rest_port", cfg.RESTPort),
	)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down datarights service")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("REST server shutdown error", slog.String("error", err.Error()))
	}

	return nil
}
