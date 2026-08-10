package serverkit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/metrics"
)

// panicStatusMessage is the gRPC status message a recovered panic returns. It
// carries no panic detail: the panic value and the stack stay in the log record.
const panicStatusMessage = "internal server error"

// panicGroupPrefix prefixes the Sentry group key of every recovered panic, so
// panics form their own cluster in the issue stream instead of mixing with
// ordinary reported failures. serverkit owns the prefix and each call site names
// only its own site, so no site can accidentally group a panic with a
// non-panic.
const panicGroupPrefix = "panic."

// panicOriginFrameSkip is how many frames sit between the stack capture below and
// the frame that panicked: LogRecoveredPanic itself, and the deferred function
// literal that called it. Every call site invokes LogRecoveredPanic directly from
// a deferred literal, which is what makes one fixed depth correct for all of
// them. The runtime.gopanic frame above them needs no skip, because the SDK drops
// every frame in the runtime package.
const panicOriginFrameSkip = 2

// LogRecoveredPanic writes the record for a recovered panic, counts it, and
// reports it to Sentry. recovered is the value returned by recover(); a value that
// is not an error is wrapped into one carrying its formatted form.
//
// Every recovery site in every service routes through it, so a panic is queryable
// by the same "panic" and "stack" attributes wherever it happened, it is counted
// wherever it happened, and there is one place that decides how a panic is
// reported.
//
// site names where the panic was recovered and becomes the group key
// "panic.<site>" and the value of the metric's site label. It must come from a
// bounded set: never interpolate an identifier, which would create one Sentry
// issue and one time series per occurrence.
//
// The report is what produces a second, ordinary error-level record, through
// errkit: this one carries the panic value and the full text stack, and errkit's
// carries the taxonomy attributes every reported failure shares, so a query like
// error_kind:internal returns panics too. The text stack is deliberately not
// handed to errkit, because the Sentry context block is capped at 8 kB and the
// event already carries the same stack in structured form.
//
// A nil logger falls back to slog.Default() rather than panicking: this is the
// last line of defense, so it must not become the thing that kills the process.
func LogRecoveredPanic(
	ctx context.Context,
	log *slog.Logger,
	site, msg string,
	recovered any,
	attrs ...slog.Attr,
) {
	if log == nil {
		log = slog.Default()
	}

	err := panicToError(recovered)

	// A dead client connection never reaches this helper: Recovery classifies it
	// first and returns. So the counter holds defects only, which is what lets
	// RecoveredPanic page on a single increment.
	metrics.RecoveredPanicsTotal.WithLabelValues(site).Inc()

	record := append([]slog.Attr{
		slog.String("panic", err.Error()),
		slog.String("stack", string(debug.Stack())),
	}, attrs...)

	log.LogAttrs(ctx, slog.LevelError, msg, record...)

	// WithStackSkip roots the captured stack at the panicking frame rather than at
	// this helper, and Report leaves an already-stacked error alone, so the event
	// carries a structured stack pointing at the defect. Without it the stack would
	// start inside errkit and every panic in the backend would group together.
	groupKey := panicGroupPrefix + site
	_ = errkit.Report(ctx, errkit.WithStackSkip(err, panicOriginFrameSkip), errkit.Meta{
		Kind: errkit.KindInternal,
		// Op and GroupKey carry the same value for two different reasons: the group
		// key pins the issue-stream cluster, and the operation tag is what makes the
		// site searchable, because a fingerprint is not.
		Op:       groupKey,
		GroupKey: groupKey,
		Msg:      msg,
		Data:     panicContext(attrs),
	})
}

// panicContext flattens the site attributes into the Sentry context block, so the
// event and the log record name the same occurrence. Nothing else is added: the
// panic value is already the exception and the stack is already structured.
func panicContext(attrs []slog.Attr) map[string]any {
	if len(attrs) == 0 {
		return nil
	}

	data := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		data[attr.Key] = attr.Value.Any()
	}
	return data
}

// grpcPanicSite names the panic site for a gRPC method. A full method arrives as
// "/pkg.Service/Method", so the leading slash is dropped and the group key reads
// as one dotted path. The method set is fixed at compile time, so this cannot
// grow unbounded cardinality.
func grpcPanicSite(transport, fullMethod string) string {
	return transport + "." + strings.TrimPrefix(fullMethod, "/")
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
// way, so they stay below error level here too, and they never reach
// LogRecoveredPanic: they are neither reported nor counted.
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

			LogRecoveredPanic(c.Request.Context(), log, "http", "recovered panic in HTTP handler", recovered,
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
				LogRecoveredPanic(ctx, log, grpcPanicSite("grpc", info.FullMethod),
					"recovered panic in gRPC handler", recovered,
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
				LogRecoveredPanic(streamContext(stream), log, grpcPanicSite("grpc_stream", info.FullMethod),
					"recovered panic in gRPC stream handler", recovered,
					slog.String("method", info.FullMethod),
				)
				err = status.Error(codes.Internal, panicStatusMessage)
			}
		}()

		return handler(srv, stream)
	}
}

// streamContext returns the stream's context, or a background context when the
// stream is nil. A nil stream only reaches here from a test calling the
// interceptor directly; the report must still find its way out rather than
// panicking inside the recovery.
func streamContext(stream grpc.ServerStream) context.Context {
	if stream == nil {
		return context.Background()
	}
	return stream.Context()
}
