package serverkit

import (
	"log/slog"

	sentrygrpc "github.com/getsentry/sentry-go/grpc"
	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/metrics"
)

// NewGRPCServer builds a *grpc.Server preloaded with the Sentry hub
// interceptors, the shared recovery interceptors, and the metrics interceptors.
// Services that expose no gRPC surface (gateway, datarights) skip this and pass a
// nil server to Serve.
//
// The recoveries record panics through slog.Default(), so callers install their
// logger (slog.SetDefault) before building the server.
func NewGRPCServer() *grpc.Server {
	log := slog.Default()

	// The sentrygrpc interceptors exist only to put a hub on the call context and
	// continue an incoming trace, which is what lets each recovery attach request
	// data to its report. Repanic is set explicitly on both and is unreachable by
	// construction: each recovery sits inside its Sentry interceptor and is
	// terminal, so no panic reaches sentrygrpc's own recover. Letting one reach it
	// would capture a second event for one panic.
	return grpc.NewServer(
		// ChainUnaryInterceptor runs its first entry outermost, so recovery is
		// outside metrics and a panic raised in the metrics layer is caught too.
		grpc.ChainUnaryInterceptor(
			sentrygrpc.UnaryServerInterceptor(sentrygrpc.ServerOptions{Repanic: true}),
			RecoveryUnaryInterceptor(log),
			metrics.UnaryServerInterceptor(),
		),
		// Streaming mirrors unary: ChainStreamInterceptor also runs its first entry
		// outermost, so recovery stays outside metrics there too. Both are installed
		// unconditionally so the next streaming RPC inherits them rather than having
		// to remember to ask.
		grpc.ChainStreamInterceptor(
			sentrygrpc.StreamServerInterceptor(sentrygrpc.ServerOptions{Repanic: true}),
			RecoveryStreamInterceptor(log),
			metrics.StreamServerInterceptor(),
		),
	)
}
