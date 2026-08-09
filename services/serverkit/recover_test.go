package serverkit_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ItsThompson/gofin/services/metrics"
	"github.com/ItsThompson/gofin/services/serverkit"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

const (
	panicUnaryFullMethod  = "/serverkit.test.Panic/PanicUnary"
	panicStreamFullMethod = "/serverkit.test.Panic/PanicStream"
)

// panicUnaryHandler and panicStreamHandler are named rather than inline so the
// recorded stack carries a frame a test can assert on: an anonymous closure in a
// package-level var initializer shows up only as glob..funcN.
func panicUnaryHandler(context.Context, any) (any, error) {
	panic("unary handler exploded")
}

func panicStreamHandler(_ any, stream grpc.ServerStream) error {
	var in emptypb.Empty
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	// One message is sent before the panic so the test covers the live case: a
	// panic after partial delivery, which is what a truncated data export looks
	// like to datarights.
	if err := stream.SendMsg(&emptypb.Empty{}); err != nil {
		return err
	}
	panic("stream handler exploded")
}

// panicServiceDesc is a hand-written service whose unary and server-streaming
// methods both panic, so the recovery interceptors are exercised through a real
// grpc.Server rather than by calling them directly. The unary handler invokes
// the interceptor itself because that is how grpc-go hands it to a method
// handler; stream interceptors are applied by grpc-go around the handler.
var panicServiceDesc = grpc.ServiceDesc{
	ServiceName: "serverkit.test.Panic",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "PanicUnary",
		Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			in := new(emptypb.Empty)
			if err := dec(in); err != nil {
				return nil, err
			}
			if interceptor == nil {
				return panicUnaryHandler(ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: panicUnaryFullMethod}
			return interceptor(ctx, in, info, panicUnaryHandler)
		},
	}},
	Streams: []grpc.StreamDesc{{
		StreamName:    "PanicStream",
		Handler:       panicStreamHandler,
		ServerStreams: true,
	}},
}

// requireOnePanicRecord asserts the sink holds exactly one error-level record
// and returns it. Every recovery site shares the assertion because the criterion
// is the same everywhere: one record, at error level, per recovered panic.
func requireOnePanicRecord(t *testing.T, sink *serverkittest.Sink) map[string]any {
	t.Helper()

	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1, "a recovered panic must produce exactly one error-level record")
	return records[0]
}

// withDefaultLogger installs log as slog.Default() for the duration of the test.
// NewRouter and NewGRPCServer capture the default logger, so a test that wants
// to read their records has to install its own first.
func withDefaultLogger(t *testing.T, log *slog.Logger) {
	t.Helper()

	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(log)
}

// routerWithPanic builds a router whose GET /boom handler panics with value.
// The panic goes through explodeWith rather than the handler closure, because
// serverkit.Recovery is inlined into its caller: the deferred recovery closure's
// own frame is named after the function that built the router, so asserting
// "routerWithPanic" would match recovery machinery and pass on a stack that
// never reached the origin. Never assert a frame whose name also appears in the
// function that installs the recovery.
func routerWithPanic(log *slog.Logger, value any) *gin.Engine {
	router := gin.New()
	router.Use(serverkit.Recovery(log))
	router.GET("/boom", func(*gin.Context) { explodeWith(value) })
	return router
}

func explodeWith(value any) { panic(value) }

// outOfRangeHandler raises a real runtime panic. The index comes from the
// request so the out-of-range read is not folded away at compile time, and the
// handler is named so the recorded stack carries a frame to assert on.
func outOfRangeHandler(c *gin.Context) {
	segments := strings.Split(c.Request.URL.Path, "/")
	_ = segments[len(segments)]
}

// ---------------------------------------------------------------------------
// HTTP recovery
// ---------------------------------------------------------------------------

func TestRecovery_PanicYields500InTheSharedErrorShape(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	router := routerWithPanic(logger, "handler exploded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t,
		`{"code":"INTERNAL_SERVER_ERROR","message":"An unexpected error occurred"}`,
		w.Body.String(),
	)
}

func TestRecovery_WritesExactlyOneErrorRecordWithPanicAndStack(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	router := routerWithPanic(logger, "handler exploded")

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "ERROR", record["level"])
	assert.Equal(t, "recovered panic in HTTP handler", record["msg"])
	assert.Equal(t, "panic: handler exploded", record["panic"])
	assert.Equal(t, http.MethodGet, record["method"])
	assert.Equal(t, "/boom", record["path"])
	// The panicking frame, not debug.Stack's own first frame and not a name the
	// recovery closure also carries: a stack holding only recovery machinery must
	// fail here.
	assert.Contains(t, record["stack"], "explodeWith")
}

func TestRecovery_ErrorPanicValueIsRecordedAsIs(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	router := routerWithPanic(logger, assert.AnError)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, assert.AnError.Error(), requireOnePanicRecord(t, logs)["panic"])
}

// TestRecovery_RuntimeErrorPanicIsRecovered covers the realistic production
// case: the runtime raises the panic, and its value is already an error.
func TestRecovery_RuntimeErrorPanicIsRecovered(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	router := gin.New()
	router.Use(serverkit.Recovery(logger))
	router.GET("/boom", outOfRangeHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	record := requireOnePanicRecord(t, logs)
	assert.Contains(t, record["panic"], "index out of range")
	assert.Contains(t, record["stack"], "outOfRangeHandler")
}

func TestRecovery_NonErrorPanicValueIsWrappedIntoAnError(t *testing.T) {
	cases := map[string]struct {
		value     any
		wantPanic string
	}{
		"string": {value: "plain string panic", wantPanic: "panic: plain string panic"},
		"int":    {value: 42, wantPanic: "panic: 42"},
		"struct": {value: struct{ Field string }{Field: "x"}, wantPanic: "panic: {x}"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			logger, logs := serverkittest.NewLogger()
			router := routerWithPanic(logger, tc.value)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

			require.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Equal(t, tc.wantPanic, requireOnePanicRecord(t, logs)["panic"],
				"a wrapped panic value must still produce exactly one record")
		})
	}
}

func TestRecovery_AbortedConnectionIsNotAnErrorLevelDefect(t *testing.T) {
	// The bare errnos are what a hand-written panic carries; the wrapped shape is
	// what net/http actually produces, and only errors.Is unwrapping makes the
	// second one classify.
	cases := map[string]error{
		"broken pipe":      syscall.EPIPE,
		"connection reset": syscall.ECONNRESET,
		"abort handler":    http.ErrAbortHandler,
		"net.OpError wrapping os.SyscallError wrapping EPIPE": &net.OpError{
			Op:  "write",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE},
		},
	}

	for name, panicValue := range cases {
		t.Run(name, func(t *testing.T) {
			logger, logs := serverkittest.NewLogger()
			router := routerWithPanic(logger, panicValue)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

			errorRecords, err := logs.ErrorRecords()
			require.NoError(t, err)
			assert.Empty(t, errorRecords, "a dead connection is not a service defect")

			warnRecords, err := logs.RecordsAtLevel("WARN")
			require.NoError(t, err)
			require.Len(t, warnRecords, 1, "the abort should still be visible below error level")

			assert.Empty(t, w.Body.String(), "nothing can be written to an aborted connection")
		})
	}
}

func TestRecovery_HealthyRequestIsUntouched(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	router := gin.New()
	router.Use(serverkit.Recovery(logger))
	router.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
	records, err := logs.Records()
	require.NoError(t, err)
	assert.Empty(t, records)
}

// TestRecovery_The500ReachesARealClient drives the recovery over a real socket
// rather than an httptest recorder, so the response actually passes through
// net/http's write path after the handler unwound.
func TestRecovery_The500ReachesARealClient(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	server := httptest.NewServer(routerWithPanic(logger, "handler exploded"))
	t.Cleanup(server.Close)

	resp, err := server.Client().Get(server.URL + "/boom")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.JSONEq(t,
		`{"code":"INTERNAL_SERVER_ERROR","message":"An unexpected error occurred"}`,
		string(body),
	)
	requireOnePanicRecord(t, logs)
}

// TestLogRecoveredPanic_NilLoggerFallsBackToDefault pins that the last line of
// defense cannot itself become the thing that kills the process. Not reachable
// today (every site passes a real logger), which is exactly why it needs a test.
func TestLogRecoveredPanic_NilLoggerFallsBackToDefault(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	assert.NotPanics(t, func() {
		serverkit.LogRecoveredPanic(nil, "recovered panic with no logger", "boom")
	})

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "recovered panic with no logger", record["msg"])
	assert.Equal(t, "panic: boom", record["panic"])
}

// ---------------------------------------------------------------------------
// gRPC recovery: the interceptors in isolation
// ---------------------------------------------------------------------------

// TestRecoveryUnaryInterceptor_CatchesAPanicFromAnythingInner covers the
// invariant NewGRPCServer's chain order exists for: whatever sits inside the
// recovery (the metrics interceptor, then the handler) may panic, and the
// recovery still turns it into a status.
func TestRecoveryUnaryInterceptor_CatchesAPanicFromAnythingInner(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	interceptor := serverkit.RecoveryUnaryInterceptor(logger)
	info := &grpc.UnaryServerInfo{FullMethod: panicUnaryFullMethod}

	resp, err := interceptor(context.Background(), &emptypb.Empty{}, info, panicUnaryHandler)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal server error", status.Convert(err).Message(),
		"the panic value must not reach the caller")

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "ERROR", record["level"])
	assert.Equal(t, "panic: unary handler exploded", record["panic"])
	assert.Equal(t, panicUnaryFullMethod, record["method"])
	assert.Contains(t, record["stack"], "panicUnaryHandler")
}

func TestRecoveryUnaryInterceptor_PassesThroughAHealthyCall(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	interceptor := serverkit.RecoveryUnaryInterceptor(logger)
	info := &grpc.UnaryServerInfo{FullMethod: panicUnaryFullMethod}
	want := &emptypb.Empty{}

	resp, err := interceptor(context.Background(), &emptypb.Empty{}, info,
		func(context.Context, any) (any, error) { return want, nil })

	require.NoError(t, err)
	assert.Same(t, want, resp)
	records, err := logs.Records()
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestRecoveryStreamInterceptor_CatchesAPanicFromAnythingInner(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	interceptor := serverkit.RecoveryStreamInterceptor(logger)
	info := &grpc.StreamServerInfo{FullMethod: panicStreamFullMethod, IsServerStream: true}

	err := interceptor(nil, nil, info, panicInnerStreamHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "ERROR", record["level"])
	assert.Equal(t, "panic: inner stream layer exploded", record["panic"])
	assert.Equal(t, panicStreamFullMethod, record["method"])
	assert.Contains(t, record["stack"], "panicInnerStreamHandler")
}

// panicInnerStreamHandler stands in for whatever the stream recovery wraps. It
// is named so the recorded stack carries a frame to assert on.
func panicInnerStreamHandler(any, grpc.ServerStream) error {
	panic("inner stream layer exploded")
}

// ---------------------------------------------------------------------------
// gRPC recovery: through a real server built by NewGRPCServer
// ---------------------------------------------------------------------------

// serveTestGRPC starts server on a loopback listener and returns a client
// connection to it, tearing both down when the test ends.
func serveTestGRPC(t *testing.T, server *grpc.Server) *grpc.ClientConn {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(lis) }()
	t.Cleanup(func() {
		server.GracefulStop()
		<-serveErr
	})

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func TestNewGRPCServer_PanickingUnaryHandlerKeepsTheServerServing(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	server := serverkit.NewGRPCServer()
	server.RegisterService(&panicServiceDesc, struct{}{})
	server.RegisterService(&echoServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := conn.Invoke(ctx, panicUnaryFullMethod, &emptypb.Empty{}, &emptypb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	assert.NoError(t, conn.Invoke(ctx, echoFullMethod, &emptypb.Empty{}, &emptypb.Empty{}),
		"the server must still serve after recovering a panic")

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "recovered panic in gRPC handler", record["msg"])
	assert.Equal(t, panicUnaryFullMethod, record["method"])
	assert.Contains(t, record["stack"], "panicUnaryHandler")
}

func TestNewGRPCServer_PanickingStreamHandlerTerminatesTheStreamAndKeepsServing(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	server := serverkit.NewGRPCServer()
	server.RegisterService(&panicServiceDesc, struct{}{})
	server.RegisterService(&echoServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{StreamName: "PanicStream", ServerStreams: true}, panicStreamFullMethod)
	require.NoError(t, err)
	require.NoError(t, stream.SendMsg(&emptypb.Empty{}))
	require.NoError(t, stream.CloseSend())

	// The handler delivers one message before panicking, so the first receive
	// succeeds and the second carries the terminal status.
	require.NoError(t, stream.RecvMsg(&emptypb.Empty{}))
	err = stream.RecvMsg(&emptypb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	assert.NoError(t, conn.Invoke(ctx, echoFullMethod, &emptypb.Empty{}, &emptypb.Empty{}),
		"the server must still serve after recovering a stream panic")

	record := requireOnePanicRecord(t, logs)
	assert.Equal(t, "recovered panic in gRPC stream handler", record["msg"])
	assert.Equal(t, panicStreamFullMethod, record["method"])
	assert.Contains(t, record["stack"], "panicStreamHandler")
}

// TestNewGRPCServer_RecoveryIsOutsideMetrics pins the chain order. A panic
// unwinds past the metrics interceptor, which records after the handler returns,
// so a recovered unary panic leaves no observation. That is only possible with
// metrics inside the recovery; were it outside, it would record Internal. The
// resulting metrics blind spot is recorded in docs/monitoring.md.
func TestNewGRPCServer_RecoveryIsOutsideMetrics(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	server := serverkit.NewGRPCServer()
	server.RegisterService(&panicServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(panicUnaryFullMethod, "Internal"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, conn.Invoke(ctx, panicUnaryFullMethod, &emptypb.Empty{}, &emptypb.Empty{}))

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(panicUnaryFullMethod, "Internal"))
	assert.Equal(t, float64(0), after-before,
		"metrics must sit inside the recovery, so a panic never reaches it")
}
