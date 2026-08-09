package serverkit

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"
)

// Shutdown budget for the two bounded phases. Both are sized to fit inside
// Docker's default 10-second stop grace period together, because a container that
// exceeds it is SIGKILLed and the flush is exactly what a SIGKILL would discard:
//
//	shutdownTimeout + flushTimeout = 8s + 2s = 10s
//
// No compose service overrides stop_grace_period, so 10 seconds is the real
// budget and the two bounds have to share it.
//
// Shutdown is not bounded overall. grpcSrv.GracefulStop() runs first and waits
// for in-flight RPCs with no timeout of its own, so a long-lived stream can
// consume the whole grace period before either bound below is reached. Bounding
// that is a separate change; the arithmetic above covers only these two phases.
const (
	shutdownTimeout = 8 * time.Second
	flushTimeout    = 2 * time.Second
)

// Serve runs httpSrv and, when grpcSrv is non-nil, grpcSrv on grpcLis, then
// blocks until ctx is cancelled or a server fails fatally.
//
// On ctx cancellation it performs a bounded graceful shutdown and returns nil.
// If a server fails to serve (e.g. an HTTP bind failure, the zombie-process
// bug this fixes for all services), Serve returns the first such error after
// shutting the other server down, so the caller's run() can exit non-zero
// instead of lingering with no listener.
//
// grpcSrv and grpcLis may both be nil for the HTTP-only path (gateway,
// datarights). http.ErrServerClosed and the gRPC graceful-stop signal are
// treated as clean exits.
//
// Buffered Sentry events are flushed once both servers have stopped accepting and
// before Serve returns, on the cancellation path and the fatal-error path alike:
// the fatal error is the one most worth keeping.
func Serve(ctx context.Context, httpSrv *http.Server, grpcSrv *grpc.Server, grpcLis net.Listener) error {
	// Buffered so both goroutines can report without blocking even if only the
	// first error is consumed by the select below.
	fatal := make(chan error, 2)
	var wg sync.WaitGroup

	if grpcSrv != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Serve returns nil on GracefulStop and ErrServerStopped only if the
			// server was already stopped; neither is a fatal serve error.
			if err := grpcSrv.Serve(grpcLis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				fatal <- err
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal <- err
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-fatal:
	}

	if grpcSrv != nil {
		grpcSrv.GracefulStop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)

	wg.Wait()

	// Safe to call unconditionally: with no client bound, Flush returns
	// immediately rather than waiting out the timeout.
	sentry.Flush(flushTimeout)

	return serveErr
}
