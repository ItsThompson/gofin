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
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ItsThompson/gofin/services/serverkit"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

const (
	hubUnaryFullMethod  = "/serverkit.test.Hub/HubUnary"
	hubStreamFullMethod = "/serverkit.test.Hub/HubStream"
)

// hubProbe records whether a Sentry hub was on the context inside a handler. The
// hub is what errkit.Report resolves to find the request, trace, and user data
// the Sentry middleware attached on the way in; without one it falls back to a
// clone of the global hub and every report loses that context.
type hubProbe struct {
	seen bool
}

func (p *hubProbe) observe(ctx context.Context) {
	p.seen = sentry.GetHubFromContext(ctx) != nil
}

// hubServiceDesc exposes one unary and one server-streaming method, each of which
// only observes its context. Registering both on a server built by NewGRPCServer
// is what proves the two interceptor chains are wired, not just constructible.
func hubServiceDesc(probe *hubProbe) *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: "serverkit.test.Hub",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "HubUnary",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(emptypb.Empty)
				if err := dec(in); err != nil {
					return nil, err
				}
				handler := func(ctx context.Context, _ any) (any, error) {
					probe.observe(ctx)
					return &emptypb.Empty{}, nil
				}
				if interceptor == nil {
					return handler(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: hubUnaryFullMethod}
				return interceptor(ctx, in, info, handler)
			},
		}},
		Streams: []grpc.StreamDesc{{
			StreamName: "HubStream",
			Handler: func(_ any, stream grpc.ServerStream) error {
				probe.observe(stream.Context())
				return stream.SendMsg(&emptypb.Empty{})
			},
			ServerStreams: true,
		}},
	}
}

// ---------------------------------------------------------------------------
// A hub reaches every handler on all three transports
// ---------------------------------------------------------------------------

func TestNewRouter_PutsAHubOnTheRequestContext(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	var probe hubProbe
	router := serverkit.NewRouter("expense", false)
	router.GET("/probe", func(c *gin.Context) { probe.observe(c.Request.Context()) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, probe.seen, "sentrygin must put a hub on the request context")
}

func TestNewGRPCServer_PutsAHubOnTheUnaryCallContext(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	var probe hubProbe
	server := serverkit.NewGRPCServer()
	server.RegisterService(hubServiceDesc(&probe), struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, conn.Invoke(ctx, hubUnaryFullMethod, &emptypb.Empty{}, &emptypb.Empty{}))
	assert.True(t, probe.seen, "sentrygrpc's unary interceptor must put a hub on the call context")
}

func TestNewGRPCServer_PutsAHubOnTheStreamCallContext(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)

	var probe hubProbe
	server := serverkit.NewGRPCServer()
	server.RegisterService(hubServiceDesc(&probe), struct{}{})
	conn := serveTestGRPC(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{StreamName: "HubStream", ServerStreams: true}, hubStreamFullMethod)
	require.NoError(t, err)
	require.NoError(t, stream.CloseSend())
	require.NoError(t, stream.RecvMsg(&emptypb.Empty{}))

	assert.True(t, probe.seen, "sentrygrpc's stream interceptor must put a hub on the call context")
}
