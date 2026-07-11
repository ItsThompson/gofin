package serverkit_test

import (
	"context"
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

const echoFullMethod = "/serverkit.test.Echo/Ping"

// echoServiceDesc is a minimal hand-written unary service used to prove the
// shared metrics interceptor NewGRPCServer wires in actually runs.
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
