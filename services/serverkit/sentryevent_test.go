package serverkit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/serverkit"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// bindRecordingHub binds a client recording into the returned transport to the
// process-wide hub, and restores the previous client afterwards.
//
// This is the shape production has: sentrygin and sentrygrpc both derive their
// per-request hub from sentry.CurrentHub(), so binding there is what makes the
// events a request produces observable. Tests using it must not run in parallel.
func bindRecordingHub(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))

	return transport
}

// groupKeyOf returns the logical half of the event's fingerprint. errkit emits
// {"{{ default }}", groupKey}, so a logical key can only refine Sentry's own
// grouping and never replace it.
func groupKeyOf(t *testing.T, event *sentry.Event) string {
	t.Helper()

	require.Len(t, event.Fingerprint, 2, "expected the two-element refining fingerprint")
	assert.Equal(t, "{{ default }}", event.Fingerprint[0])
	return event.Fingerprint[1]
}

// topInAppFrame returns the newest in-app frame of the event's outermost
// exception. The SDK reverses its exception list, so the outermost error is last,
// and reverses frames too, so the newest frame is last. The runtime.gopanic frame
// above the origin is already gone: the SDK drops every frame in the runtime
// package.
func topInAppFrame(t *testing.T, event *sentry.Event) sentry.Frame {
	t.Helper()

	require.NotEmpty(t, event.Exception, "expected at least one exception entry")
	stacktrace := event.Exception[len(event.Exception)-1].Stacktrace
	require.NotNil(t, stacktrace, "the outermost exception must carry the stack errkit attached")

	for i := len(stacktrace.Frames) - 1; i >= 0; i-- {
		if stacktrace.Frames[i].InApp {
			return stacktrace.Frames[i]
		}
	}

	t.Fatalf("no in-app frame in %d frames", len(stacktrace.Frames))
	return sentry.Frame{}
}

// TestNewRouter_APanickingHandlerYieldsExactlyOneEvent is the assertion that
// catches a recovery re-panicking into sentrygin. sentrygin captures a panic
// itself before honoring Repanic, so a re-panicking recovery produces two events
// for one failure and burns double the quota.
func TestNewRouter_APanickingHandlerYieldsExactlyOneEvent(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	transport := bindRecordingHub(t)

	router := serverkit.NewRouter("expense", false)
	router.GET("/boom", func(*gin.Context) { explodeWith("router handler exploded") })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	requireOnePanicRecord(t, logs)

	// The log side of the same report. The two records are deliberately different:
	// the panic record carries the value and the text stack, and this one carries
	// the taxonomy every reported failure shares, so error_kind:internal returns
	// panics alongside everything else.
	report := requireOneReportRecord(t, logs)
	assert.Equal(t, "recovered panic in HTTP handler", report["msg"])
	assert.Equal(t, "internal", report["error_kind"])
	assert.Equal(t, "panic.http", report["operation"])
	assert.Equal(t, "panic: router handler exploded", report["error"])

	events := transport.Events()
	require.Len(t, events, 1, "two events means a recovery re-panicked into sentrygin")
	assert.Equal(t, "panic.http", groupKeyOf(t, events[0]))
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
	assert.Equal(t, "explodeWith", topInAppFrame(t, events[0]).Function,
		"the report must carry the panicking frame, not the recovery helper's own")
}

func TestNewGRPCServer_APanickingUnaryHandlerYieldsExactlyOneEvent(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	transport := bindRecordingHub(t)

	server := serverkit.NewGRPCServer()
	server.RegisterService(&panicServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, conn.Invoke(ctx, panicUnaryFullMethod, &emptypb.Empty{}, &emptypb.Empty{}))

	requireOnePanicRecord(t, logs)
	events := transport.Events()
	require.Len(t, events, 1, "two events means the recovery re-panicked into sentrygrpc")
	assert.Equal(t, "panic.grpc.serverkit.test.Panic/PanicUnary", groupKeyOf(t, events[0]))
	assert.Equal(t, "panicUnaryHandler", topInAppFrame(t, events[0]).Function)
}

// TestNewGRPCServer_APanickingStreamYieldsOneEventAndKeepsServing covers the
// highest-consequence path: StreamAllUserExpenses is the repo's only streaming
// RPC and it backs the user data export, so a panic there must terminate one
// stream rather than the process.
func TestNewGRPCServer_APanickingStreamYieldsOneEventAndKeepsServing(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	transport := bindRecordingHub(t)

	server := serverkit.NewGRPCServer()
	server.RegisterService(&panicServiceDesc, struct{}{})
	server.RegisterService(&echoServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{StreamName: "PanicStream", ServerStreams: true}, panicStreamFullMethod)
	require.NoError(t, err)
	require.NoError(t, stream.SendMsg(&emptypb.Empty{}))
	require.NoError(t, stream.CloseSend())
	require.NoError(t, stream.RecvMsg(&emptypb.Empty{}), "the handler delivers one message before panicking")

	err = stream.RecvMsg(&emptypb.Empty{})
	require.Error(t, err, "the client must see the stream fail rather than end cleanly")
	assert.Equal(t, codes.Internal, status.Code(err))

	require.NoError(t, conn.Invoke(ctx, echoFullMethod, &emptypb.Empty{}, &emptypb.Empty{}),
		"the process must keep serving after a recovered stream panic")

	requireOnePanicRecord(t, logs)
	events := transport.Events()
	require.Len(t, events, 1, "two events means the recovery re-panicked into sentrygrpc")
	assert.Equal(t, "panic.grpc_stream.serverkit.test.Panic/PanicStream", groupKeyOf(t, events[0]))
	assert.Equal(t, "panicStreamHandler", topInAppFrame(t, events[0]).Function)
}

// TestNewRouter_AHealthyRequestYieldsNoEvent guards the quota floor. Docker
// probes and Prometheus scrapes produce roughly 86,400 requests a day across five
// services, against a 5,000-event monthly allowance shared org-wide.
func TestNewRouter_AHealthyRequestYieldsNoEvent(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	transport := bindRecordingHub(t)

	router := serverkit.NewRouter("expense", false)

	for _, path := range []string{"/health", "/metrics"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code, path)
	}

	assert.Empty(t, transport.Events(), "the probe endpoints must never emit an event")
}
