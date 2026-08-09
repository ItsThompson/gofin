package serverkit

import (
	"log/slog"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/metrics"
)

// NewGRPCServer builds a *grpc.Server preloaded with the shared recovery
// interceptors and the unary metrics interceptor. Services that expose no gRPC
// surface (gateway, datarights) skip this and pass a nil server to Serve.
//
// The recoveries record panics through slog.Default(), so callers install their
// logger (slog.SetDefault) before building the server.
func NewGRPCServer() *grpc.Server {
	log := slog.Default()

	return grpc.NewServer(
		// ChainUnaryInterceptor runs its first entry outermost, so recovery is
		// outside metrics and a panic raised in the metrics layer is caught too.
		grpc.ChainUnaryInterceptor(
			RecoveryUnaryInterceptor(log),
			metrics.UnaryServerInterceptor(),
		),
		// Streaming carries recovery only: no metrics stream interceptor exists
		// (the gap is recorded in docs/monitoring.md). It is installed
		// unconditionally so the next streaming RPC inherits it rather than
		// having to remember to ask.
		grpc.ChainStreamInterceptor(
			RecoveryStreamInterceptor(log),
		),
	)
}
