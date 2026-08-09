package serverkit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"syscall"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
)

// panicStatusMessage is the gRPC status message a recovered panic returns. It
// carries no panic detail: the panic value and the stack stay in the log record.
const panicStatusMessage = "internal server error"

// LogRecoveredPanic writes the single error-level record for a recovered panic.
// recovered is the value returned by recover(); a value that is not an error is
// wrapped into one carrying its formatted form.
//
// Every recovery site in every service routes through it, so a panic is
// queryable by the same "panic" and "stack" attributes wherever it happened.
func LogRecoveredPanic(log *slog.Logger, msg string, recovered any, attrs ...slog.Attr) {
	record := append([]slog.Attr{
		slog.String("panic", panicToError(recovered).Error()),
		slog.String("stack", string(debug.Stack())),
	}, attrs...)

	log.LogAttrs(context.Background(), slog.LevelError, msg, record...)
}

// panicToError normalizes a recover() value into an error. An error value is
// returned unchanged so errors.Is still classifies it (which is how a dead
// connection is recognized); anything else is wrapped with its formatted form.
func panicToError(recovered any) error {
	if err, ok := recovered.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v", recovered)
}

// isConnectionAborted reports whether a recovered panic value means the response
// can no longer be written rather than that the service is defective.
// gin.Recovery() classifies EPIPE, ECONNRESET, and http.ErrAbortHandler this
// way, so they stay below error level here too.
func isConnectionAborted(err error) bool {
	return errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, http.ErrAbortHandler)
}

// Recovery returns Gin middleware that recovers a handler panic, records it
// through slog, and responds with the shared apierr 500 body. It replaces
// gin.Recovery(), which writes the panic and its stack as plaintext to
// gin.DefaultErrorWriter and so never reaches the JSON log stream.
//
// It is terminal: it does not re-panic.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			err := panicToError(recovered)
			if isConnectionAborted(err) {
				log.Warn("aborted HTTP connection during panic recovery",
					slog.String("panic", err.Error()),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
				)
				_ = c.Error(err)
				c.Abort()
				return
			}

			LogRecoveredPanic(log, "recovered panic in HTTP handler", recovered,
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
			)
			c.Abort()
			apierr.Respond(c, err)
		}()

		c.Next()
	}
}

// RecoveryUnaryInterceptor returns a gRPC unary interceptor that recovers a
// panic from anything inner, records it through slog, and returns
// codes.Internal so the process keeps serving. It is terminal: it does not
// re-panic.
func RecoveryUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				LogRecoveredPanic(log, "recovered panic in gRPC handler", recovered,
					slog.String("method", info.FullMethod),
				)
				err = status.Error(codes.Internal, panicStatusMessage)
			}
		}()

		return handler(ctx, req)
	}
}

// RecoveryStreamInterceptor returns a gRPC stream interceptor that recovers a
// panic from anything inner, records it through slog, and terminates the stream
// with codes.Internal so the process keeps serving. It is terminal: it does not
// re-panic.
//
// Messages already sent to the client cannot be recalled, so a consumer must
// treat a stream that ended with an error as a failed read, not a complete one.
func RecoveryStreamInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				LogRecoveredPanic(log, "recovered panic in gRPC stream handler", recovered,
					slog.String("method", info.FullMethod),
				)
				err = status.Error(codes.Internal, panicStatusMessage)
			}
		}()

		return handler(srv, stream)
	}
}
