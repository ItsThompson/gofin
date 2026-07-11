package serverkit

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// shutdownTimeout bounds the graceful shutdown of the HTTP server.
const shutdownTimeout = 10 * time.Second

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
	return serveErr
}
