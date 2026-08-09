package serverkit_test

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/serverkit"
)

// countingTransport records how many times the SDK was asked to flush. It embeds
// the shared recording transport so it stays a valid sentry.Transport if the
// interface grows a method.
type countingTransport struct {
	errkittest.Transport
	flushes atomic.Int64
}

func (t *countingTransport) Flush(timeout time.Duration) bool {
	t.flushes.Add(1)
	return t.Transport.Flush(timeout)
}

// bindFlushCounter binds a client whose transport counts flushes to the
// process-wide hub, which is the hub sentry.Flush reaches, and restores the
// previous client afterwards. Tests using it must not run in parallel.
//
// The client is built here rather than through errkittest.NewClient, which pins
// its own transport against exactly this substitution. Counting is the only way to
// observe the flush at all: sentry.Flush returns the same value whether or not
// anything was buffered.
func bindFlushCounter(t *testing.T) *countingTransport {
	t.Helper()

	transport := &countingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:            unroutableDSN,
		Transport:      transport,
		DisableLogs:    true,
		DisableMetrics: true,
	})
	require.NoError(t, err)

	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(client)

	return transport
}

// freeAddr reserves an ephemeral port, closes it, and returns the address so a
// server under test can bind it. The tiny reserve/rebind window is acceptable
// for local tests.
func freeAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}

// waitForDial blocks until a TCP dial to addr succeeds or the deadline passes.
func waitForDial(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("nothing listening on %s within deadline", addr)
}

// awaitServe returns Serve's result, failing the test if it does not return
// within the timeout (i.e. it hung).
func awaitServe(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return within 5s (possible hang)")
		return nil
	}
}

func TestServe_NormalCancellation_HTTPAndGRPC_ReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	addr := freeAddr(t)
	httpSrv := &http.Server{Addr: addr}

	grpcSrv := serverkit.NewGRPCServer()
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(ctx, httpSrv, grpcSrv, grpcLis) }()

	// Ensure the HTTP server is actually listening before cancelling, so the
	// shutdown path (ListenAndServe -> ErrServerClosed) is genuinely exercised.
	waitForDial(t, addr)

	cancel()

	require.NoError(t, awaitServe(t, errCh))
}

func TestServe_HTTPBindFailure_ReturnsError(t *testing.T) {
	// Pre-occupy the port so the HTTP server's bind fails.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()
	addr := occupied.Addr().String()

	httpSrv := &http.Server{Addr: addr}

	// A live gRPC server on its own port proves it is also cleaned up when HTTP
	// fails: Serve's wg.Wait would hang if GracefulStop were skipped.
	grpcSrv := serverkit.NewGRPCServer()
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// ctx is never cancelled: only the bind failure can end Serve.
	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(context.Background(), httpSrv, grpcSrv, grpcLis) }()

	err = awaitServe(t, errCh)
	require.Error(t, err)
	assert.ErrorContains(t, err, "address already in use")
}

func TestServe_HTTPOnly_NilGRPC_NormalCancellation_ReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	addr := freeAddr(t)
	httpSrv := &http.Server{Addr: addr}

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(ctx, httpSrv, nil, nil) }()

	waitForDial(t, addr)
	cancel()

	require.NoError(t, awaitServe(t, errCh))
}

func TestServe_HTTPOnly_NilGRPC_BindFailure_ReturnsError(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()

	httpSrv := &http.Server{Addr: occupied.Addr().String()}

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(context.Background(), httpSrv, nil, nil) }()

	err = awaitServe(t, errCh)
	require.Error(t, err)
	assert.ErrorContains(t, err, "address already in use")
}

func TestServe_ErrServerClosed_TreatedAsSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	addr := freeAddr(t)
	httpSrv := &http.Server{Addr: addr}

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(ctx, httpSrv, nil, nil) }()

	// Confirm the server reached the accept loop so its eventual exit is via
	// http.ErrServerClosed from Shutdown, not a pre-start short-circuit.
	waitForDial(t, addr)
	cancel()

	// ErrServerClosed is filtered inside Serve (errors.Is), so the observable
	// result is a nil return.
	require.NoError(t, awaitServe(t, errCh))
}

// TestServe_NormalShutdownFlushesSentryExactlyOnce guards the event a deploy
// restart would otherwise discard: the one that caused the shutdown.
func TestServe_NormalShutdownFlushesSentryExactlyOnce(t *testing.T) {
	transport := bindFlushCounter(t)

	ctx, cancel := context.WithCancel(context.Background())
	addr := freeAddr(t)
	httpSrv := &http.Server{Addr: addr}

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(ctx, httpSrv, nil, nil) }()

	waitForDial(t, addr)
	assert.Equal(t, int64(0), transport.flushes.Load(), "nothing may flush while the server is still serving")

	cancel()
	require.NoError(t, awaitServe(t, errCh))

	assert.Equal(t, int64(1), transport.flushes.Load(),
		"the shutdown path must flush once, after the servers stop and before Serve returns")
}

// TestServe_FatalServeErrorStillFlushesSentry covers the path that matters most:
// a bind failure is the error worth keeping, and it is the one that would be lost
// if the flush only ran on cancellation.
func TestServe_FatalServeErrorStillFlushesSentry(t *testing.T) {
	transport := bindFlushCounter(t)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = occupied.Close() }()

	httpSrv := &http.Server{Addr: occupied.Addr().String()}

	errCh := make(chan error, 1)
	go func() { errCh <- serverkit.Serve(context.Background(), httpSrv, nil, nil) }()

	require.Error(t, awaitServe(t, errCh))
	assert.Equal(t, int64(1), transport.flushes.Load())
}
