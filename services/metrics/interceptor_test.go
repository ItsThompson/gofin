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
