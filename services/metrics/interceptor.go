package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that records
// grpc_requests_total and grpc_request_duration_seconds for every unary RPC.
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

// StreamServerInterceptor returns a gRPC stream server interceptor that records
// the same two metrics for every streaming RPC, so a streaming method appears in
// grpc_requests_total and grpc_request_duration_seconds alongside the unary ones.
//
// One observation covers the whole stream, from the first message to the terminal
// status, because that is the only duration a server interceptor can see. A
// stream therefore reads as a lifetime rather than as per-message latency, and
// the p95 recording rule keeps it in its own per-method series.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, stream)

		duration := time.Since(start).Seconds()
		st, _ := status.FromError(err)

		GRPCRequestsTotal.WithLabelValues(info.FullMethod, st.Code().String()).Inc()
		GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		return err
	}
}
