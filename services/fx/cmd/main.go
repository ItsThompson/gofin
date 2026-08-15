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

	"github.com/ItsThompson/gofin/services/fx/internal/cache"
	"github.com/ItsThompson/gofin/services/fx/internal/config"
	"github.com/ItsThompson/gofin/services/fx/internal/handler"
	"github.com/ItsThompson/gofin/services/fx/internal/provider"
	"github.com/ItsThompson/gofin/services/fx/internal/service"
	pb "github.com/ItsThompson/gofin/services/fx/proto/fxpb"
	"github.com/ItsThompson/gofin/services/healthcheck"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func main() {
	if healthcheck.ShouldRun(os.Args) {
		os.Exit(healthcheck.Run(config.ResolveRESTPort()))
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fx-service: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := serverkit.NewLogger(cfg.LogLevel, "fx")
	slog.SetDefault(logger)
	if err := serverkit.InitSentry(serverkit.SentryConfigFromEnv("fx")); err != nil {
		logger.Error("sentry initialization failed, error reporting is disabled", slog.String("error", err.Error()))
	}

	httpClient := &http.Client{Timeout: cfg.ProviderTimeout}
	openRatesProvider := provider.NewOpenRatesProvider(
		httpClient,
		cfg.ProviderBaseURL,
		cfg.OpenExchangeRatesAppID,
		cfg.ProviderRetryCount,
		time.Now,
		logger,
	)
	converter := service.NewConverter(openRatesProvider, cache.NewRateCache(cfg.CacheMaxAge), cfg.CacheMaxAge, time.Now, logger)

	grpcServer := serverkit.NewGRPCServer()
	pb.RegisterFxServiceServer(grpcServer, handler.NewGRPCHandler(converter))
	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listening on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	router := serverkit.NewRouter("fx", cfg.IsProduction())
	httpServer := &http.Server{
		Addr:    ":" + cfg.RESTPort,
		Handler: router,
	}

	logger.Info("fx service ready", slog.String("rest_port", cfg.RESTPort), slog.String("grpc_port", cfg.GRPCPort))
	return serverkit.Serve(ctx, httpServer, grpcServer, grpcLis)
}
