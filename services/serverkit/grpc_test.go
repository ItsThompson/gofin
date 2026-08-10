package serverkit_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ItsThompson/gofin/services/metrics"
	"github.com/ItsThompson/gofin/services/serverkit"
)

const (
	echoFullMethod       = "/serverkit.test.Echo/Ping"
	echoStreamFullMethod = "/serverkit.test.Echo/PingStream"
)

// echoServiceDesc is a minimal hand-written service used to prove the shared
// metrics interceptors NewGRPCServer wires in actually run, on both the unary and
// the streaming chain.
var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "serverkit.test.Echo",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Ping",
		Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			in := new(emptypb.Empty)
			if err := dec(in); err != nil {
				return nil, err
			}
			handler := func(ctx context.Context, _ any) (any, error) {
				return &emptypb.Empty{}, nil
			}
			if interceptor == nil {
				return handler(ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: echoFullMethod}
			return interceptor(ctx, in, info, handler)
		},
	}},
	Streams: []grpc.StreamDesc{{
		StreamName:    "PingStream",
		Handler:       echoStreamHandler,
		ServerStreams: true,
	}},
}

// echoStreamHandler answers one request message with one response message and
// returns, which is the shape StreamAllUserExpenses has: a server-streaming RPC
// that ends by returning nil.
func echoStreamHandler(_ any, stream grpc.ServerStream) error {
	var in emptypb.Empty
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	return stream.SendMsg(&emptypb.Empty{})
}

func TestNewGRPCServer_WiresMetricsInterceptor(t *testing.T) {
	server := serverkit.NewGRPCServer()
	require.NotNil(t, server)
	server.RegisterService(&echoServiceDesc, struct{}{})

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

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(echoFullMethod, "OK"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, conn.Invoke(ctx, echoFullMethod, &emptypb.Empty{}, &emptypb.Empty{}))

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(echoFullMethod, "OK"))
	assert.Equal(t, float64(1), after-before, "metrics interceptor should record the unary call")
}

// TestNewGRPCServer_WiresStreamMetricsInterceptor is the streaming half of the
// test above. Without it a server-streaming RPC contributes nothing to
// grpc_requests_total, which is what left the one streaming RPC in the tree
// unmeasured.
func TestNewGRPCServer_WiresStreamMetricsInterceptor(t *testing.T) {
	server := serverkit.NewGRPCServer()
	server.RegisterService(&echoServiceDesc, struct{}{})
	conn := serveTestGRPC(t, server)

	before := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(echoStreamFullMethod, "OK"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{StreamName: "PingStream", ServerStreams: true}, echoStreamFullMethod)
	require.NoError(t, err)
	require.NoError(t, stream.SendMsg(&emptypb.Empty{}))
	require.NoError(t, stream.CloseSend())
	require.NoError(t, stream.RecvMsg(&emptypb.Empty{}))
	// The terminal status is what the interceptor turns into the status label, and
	// the client only learns it on the receive after the last message.
	require.ErrorIs(t, stream.RecvMsg(&emptypb.Empty{}), io.EOF)

	after := testutil.ToFloat64(metrics.GRPCRequestsTotal.WithLabelValues(echoStreamFullMethod, "OK"))
	assert.Equal(t, float64(1), after-before, "metrics interceptor should record the streaming call")
}
