package serverkit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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
)

const (
	panicUnaryFullMethod  = "/serverkit.test.Panic/PanicUnary"
	panicStreamFullMethod = "/serverkit.test.Panic/PanicStream"
)

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
			handler := func(context.Context, any) (any, error) {
				panic("unary handler exploded")
			}
			if interceptor == nil {
				return handler(ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: panicUnaryFullMethod}
			return interceptor(ctx, in, info, handler)
		},
	}},
	Streams: []grpc.StreamDesc{{
		StreamName: "PanicStream",
		// One message is sent before the panic so the test covers the live case:
		// a panic after partial delivery, which is what a truncated data export
		// looks like to datarights.
		Handler: func(_ any, stream grpc.ServerStream) error {
			var in emptypb.Empty
			if err := stream.RecvMsg(&in); err != nil {
				return err
			}
			if err := stream.SendMsg(&emptypb.Empty{}); err != nil {
				return err
			}
			panic("stream handler exploded")
		},
		ServerStreams: true,
	}},
}

// capturedRecords parses every JSON slog record written to buf.
func capturedRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for decoder.More() {
		var record map[string]any
		require.NoError(t, decoder.Decode(&record))
		records = append(records, record)
	}
	return records
}

// recordsAtLevel filters captured records down to one slog level.
func recordsAtLevel(records []map[string]any, level string) []map[string]any {
	var matching []map[string]any
	for _, record := range records {
		if record["level"] == level {
			matching = append(matching, record)
		}
	}
	return matching
}

func bufferedLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
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
func routerWithPanic(log *slog.Logger, value any) *gin.Engine {
	router := gin.New()
	router.Use(serverkit.Recovery(log))
	router.GET("/boom", func(*gin.Context) { panic(value) })
	return router
}

// ---------------------------------------------------------------------------
// HTTP recovery
// ---------------------------------------------------------------------------

func TestRecovery_PanicYields500InTheSharedErrorShape(t *testing.T) {
	logger, _ := bufferedLogger()
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
	logger, logs := bufferedLogger()
	router := routerWithPanic(logger, "handler exploded")

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	records := capturedRecords(t, logs)
	require.Len(t, records, 1, "a recovered panic must produce exactly one record")

	record := records[0]
	assert.Equal(t, "ERROR", record["level"])
	assert.Equal(t, "recovered panic in HTTP handler", record["msg"])
	assert.Equal(t, "panic: handler exploded", record["panic"])
	assert.Equal(t, http.MethodGet, record["method"])
	assert.Equal(t, "/boom", record["path"])
	assert.Contains(t, record["stack"], "runtime/debug.Stack")
}

func TestRecovery_ErrorPanicValueIsRecordedAsIs(t *testing.T) {
	logger, logs := bufferedLogger()
	router := routerWithPanic(logger, assert.AnError)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	records := capturedRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, assert.AnError.Error(), records[0]["panic"])
}

// TestRecovery_RuntimeErrorPanicIsRecovered covers the realistic production
// case: the runtime raises the panic, and its value is already an error.
func TestRecovery_RuntimeErrorPanicIsRecovered(t *testing.T) {
	logger, logs := bufferedLogger()
	router := gin.New()
	router.Use(serverkit.Recovery(logger))
	router.GET("/boom", func(c *gin.Context) {
		// The index comes from the request so the out-of-range read is not folded
		// away at compile time.
		segments := strings.Split(c.Request.URL.Path, "/")
		_ = segments[len(segments)]
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	records := capturedRecords(t, logs)
	require.Len(t, records, 1)
	assert.Contains(t, records[0]["panic"], "index out of range")
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
			logger, logs := bufferedLogger()
			router := routerWithPanic(logger, tc.value)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

			require.Equal(t, http.StatusInternalServerError, w.Code)
			records := capturedRecords(t, logs)
			require.Len(t, records, 1, "a wrapped panic value must still produce exactly one record")
			assert.Equal(t, tc.wantPanic, records[0]["panic"])
		})
	}
}

func TestRecovery_AbortedConnectionIsNotAnErrorLevelDefect(t *testing.T) {
	cases := map[string]error{
		"broken pipe":      syscall.EPIPE,
		"connection reset": syscall.ECONNRESET,
		"abort handler":    http.ErrAbortHandler,
	}

	for name, panicValue := range cases {
		t.Run(name, func(t *testing.T) {
			logger, logs := bufferedLogger()
			router := routerWithPanic(logger, panicValue)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

			records := capturedRecords(t, logs)
			assert.Empty(t, recordsAtLevel(records, "ERROR"),
				"a dead connection is not a service defect")
			require.Len(t, recordsAtLevel(records, "WARN"), 1,
				"the abort should still be visible below error level")
			assert.Empty(t, w.Body.String(), "nothing can be written to an aborted connection")
		})
	}
}

func TestRecovery_HealthyRequestIsUntouched(t *testing.T) {
	logger, logs := bufferedLogger()
	router := gin.New()
	router.Use(serverkit.Recovery(logger))
	router.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
	assert.Empty(t, capturedRecords(t, logs))
}

// TestRecovery_The500ReachesARealClient drives the recovery over a real socket
// rather than an httptest recorder, so the response actually passes through
// net/http's write path after the handler unwound.
func TestRecovery_The500ReachesARealClient(t *testing.T) {
	logger, logs := bufferedLogger()
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
	assert.Len(t, recordsAtLevel(capturedRecords(t, logs), "ERROR"), 1)
}

// ---------------------------------------------------------------------------
// gRPC recovery: the interceptors in isolation
// ---------------------------------------------------------------------------

// TestRecoveryUnaryInterceptor_CatchesAPanicFromAnythingInner covers the
// invariant NewGRPCServer's chain order exists for: whatever sits inside the
// recovery (the metrics interceptor, then the handler) may panic, and the
// recovery still turns it into a status.
func TestRecoveryUnaryInterceptor_CatchesAPanicFromAnythingInner(t *testing.T) {
	logger, logs := bufferedLogger()
	interceptor := serverkit.RecoveryUnaryInterceptor(logger)
	info := &grpc.UnaryServerInfo{FullMethod: panicUnaryFullMethod}

	resp, err := interceptor(context.Background(), &emptypb.Empty{}, info,
		func(context.Context, any) (any, error) { panic("inner layer exploded") })

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal server error", status.Convert(err).Message(),
		"the panic value must not reach the caller")

	records := capturedRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "ERROR", records[0]["level"])
	assert.Equal(t, "panic: inner layer exploded", records[0]["panic"])
	assert.Equal(t, panicUnaryFullMethod, records[0]["method"])
	assert.Contains(t, records[0]["stack"], "runtime/debug.Stack")
}

func TestRecoveryUnaryInterceptor_PassesThroughAHealthyCall(t *testing.T) {
	logger, logs := bufferedLogger()
	interceptor := serverkit.RecoveryUnaryInterceptor(logger)
	info := &grpc.UnaryServerInfo{FullMethod: panicUnaryFullMethod}
	want := &emptypb.Empty{}

	resp, err := interceptor(context.Background(), &emptypb.Empty{}, info,
		func(context.Context, any) (any, error) { return want, nil })

	require.NoError(t, err)
	assert.Same(t, want, resp)
	assert.Empty(t, capturedRecords(t, logs))
}

func TestRecoveryStreamInterceptor_CatchesAPanicFromAnythingInner(t *testing.T) {
	logger, logs := bufferedLogger()
	interceptor := serverkit.RecoveryStreamInterceptor(logger)
	info := &grpc.StreamServerInfo{FullMethod: panicStreamFullMethod, IsServerStream: true}

	err := interceptor(nil, nil, info, func(any, grpc.ServerStream) error {
		panic("inner stream layer exploded")
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	records := capturedRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "ERROR", records[0]["level"])
	assert.Equal(t, "panic: inner stream layer exploded", records[0]["panic"])
	assert.Equal(t, panicStreamFullMethod, records[0]["method"])
	assert.Contains(t, records[0]["stack"], "runtime/debug.Stack")
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
	logger, logs := bufferedLogger()
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

	records := recordsAtLevel(capturedRecords(t, logs), "ERROR")
	require.Len(t, records, 1, "the recovered panic must produce exactly one record")
	assert.Equal(t, "recovered panic in gRPC handler", records[0]["msg"])
	assert.Equal(t, panicUnaryFullMethod, records[0]["method"])
}

func TestNewGRPCServer_PanickingStreamHandlerTerminatesTheStreamAndKeepsServing(t *testing.T) {
	logger, logs := bufferedLogger()
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

	records := recordsAtLevel(capturedRecords(t, logs), "ERROR")
	require.Len(t, records, 1, "the recovered panic must produce exactly one record")
	assert.Equal(t, "recovered panic in gRPC stream handler", records[0]["msg"])
	assert.Equal(t, panicStreamFullMethod, records[0]["method"])
}

// TestNewGRPCServer_RecoveryIsOutsideMetrics pins the chain order. A panic
// unwinds past the metrics interceptor, which records after the handler returns,
// so a recovered unary panic leaves no observation. That is only possible with
// metrics inside the recovery; were it outside, it would record Internal. The
// resulting metrics blind spot is recorded in docs/monitoring.md.
func TestNewGRPCServer_RecoveryIsOutsideMetrics(t *testing.T) {
	logger, _ := bufferedLogger()
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
