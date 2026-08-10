package metrics_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/metrics"
)

// TestUnaryServerInterceptor_RecordsSuccessOutcome drives the interceptor with a
// stub handler that returns no error and asserts grpc_requests_total is
// incremented for the "OK" status label, that the handler ran, and that the
// handler's response and nil error propagate back to the caller.
func TestUnaryServerInterceptor_RecordsSuccessOutcome(t *testing.T) {
	interceptor := metrics.UnaryServerInterceptor()
	// A unique method isolates this test's series from other tests that also
	// touch the process-global GRPCRequestsTotal collector.
	const method = "/test.Service/InterceptorSuccess"
	info := &grpc.UnaryServerInfo{FullMethod: method}

	handlerCalled := false
	handler := func(_ context.Context, req any) (any, error) {
		handlerCalled = true
		return "handled", nil
	}

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()))

	resp, err := interceptor(context.Background(), "req", info, handler)

	require.NoError(t, err)
	assert.True(t, handlerCalled, "interceptor must invoke the wrapped handler")
	assert.Equal(t, "handled", resp, "interceptor must return the handler's response")

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()))
	assert.Equal(t, 1.0, after-before, "success outcome must record one OK request")
}

// TestUnaryServerInterceptor_RecordsErrorOutcome drives the interceptor with a
// stub handler that returns a gRPC status error and asserts grpc_requests_total
// is incremented for that status-code label, and that the handler's error
// propagates back unchanged.
func TestUnaryServerInterceptor_RecordsErrorOutcome(t *testing.T) {
	interceptor := metrics.UnaryServerInterceptor()
	const method = "/test.Service/InterceptorError"
	info := &grpc.UnaryServerInfo{FullMethod: method}

	wantErr := status.Error(codes.NotFound, "missing")
	handlerCalled := false
	handler := func(_ context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, wantErr
	}

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.NotFound.String()))

	resp, err := interceptor(context.Background(), "req", info, handler)

	require.ErrorIs(t, err, wantErr, "interceptor must propagate the handler's error")
	assert.True(t, handlerCalled, "interceptor must invoke the wrapped handler")
	assert.Nil(t, resp, "interceptor must return the handler's nil response on error")

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.NotFound.String()))
	assert.Equal(t, 1.0, after-before, "error outcome must record one NotFound request")
}

// TestStreamServerInterceptor_RecordsSuccessOutcome drives the stream interceptor
// with a stub handler that returns no error and asserts the streaming RPC lands in
// the same two collectors the unary interceptor feeds. The nil ServerStream is
// deliberate: the interceptor passes the stream straight through and reads nothing
// off it, so a nil value proves the recording needs no stream state.
func TestStreamServerInterceptor_RecordsSuccessOutcome(t *testing.T) {
	interceptor := metrics.StreamServerInterceptor()
	const method = "/test.Service/StreamInterceptorSuccess"
	info := &grpc.StreamServerInfo{FullMethod: method, IsServerStream: true}

	handlerCalled := false
	handler := func(_ any, _ grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()))
	durationSeriesBefore := testutil.CollectAndCount(metrics.GRPCRequestDuration, "grpc_request_duration_seconds")

	err := interceptor(nil, nil, info, handler)

	require.NoError(t, err)
	assert.True(t, handlerCalled, "interceptor must invoke the wrapped handler")

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()))
	assert.Equal(t, 1.0, after-before, "success outcome must record one OK request")

	// The method label is unique to this test, so a new series is the observable
	// proof that a streaming RPC now reaches the duration histogram too.
	durationSeriesAfter := testutil.CollectAndCount(metrics.GRPCRequestDuration, "grpc_request_duration_seconds")
	assert.Equal(t, 1, durationSeriesAfter-durationSeriesBefore,
		"a completed stream must be observed in the duration histogram")
}

// TestStreamServerInterceptor_RecordsErrorOutcome asserts a stream that ends with
// a status error is recorded under that status code, which is what makes a
// recovered stream panic visible as an Internal stream rather than as nothing.
func TestStreamServerInterceptor_RecordsErrorOutcome(t *testing.T) {
	interceptor := metrics.StreamServerInterceptor()
	const method = "/test.Service/StreamInterceptorError"
	info := &grpc.StreamServerInfo{FullMethod: method, IsServerStream: true}

	wantErr := status.Error(codes.Internal, "stream failed")
	handlerCalled := false
	handler := func(_ any, _ grpc.ServerStream) error {
		handlerCalled = true
		return wantErr
	}

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.Internal.String()))

	err := interceptor(nil, nil, info, handler)

	require.ErrorIs(t, err, wantErr, "interceptor must propagate the handler's error")
	assert.True(t, handlerCalled, "interceptor must invoke the wrapped handler")

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(method, codes.Internal.String()))
	assert.Equal(t, 1.0, after-before, "error outcome must record one Internal request")
}
