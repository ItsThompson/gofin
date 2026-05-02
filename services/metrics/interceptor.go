package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that records
// grpc_requests_total and grpc_request_duration_seconds for every RPC.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start).Seconds()
		st, _ := status.FromError(err)

		GRPCRequestsTotal.WithLabelValues(info.FullMethod, st.Code().String()).Inc()
		GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		return resp, err
	}
}
