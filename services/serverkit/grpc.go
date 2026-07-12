package serverkit

import (
	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/metrics"
)

// NewGRPCServer builds a *grpc.Server preloaded with the shared unary metrics
// interceptor. Services that expose no gRPC surface (gateway, datarights) skip
// this and pass a nil server to Serve.
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor()),
	)
}
