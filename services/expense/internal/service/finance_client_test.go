package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	financepb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

type financeClientStub struct {
	financepb.FinanceServiceClient
	evict func(context.Context, *financepb.EvictUserCacheRequest, ...grpc.CallOption) (*financepb.EvictUserCacheResponse, error)
}

func (s *financeClientStub) EvictUserCache(ctx context.Context, request *financepb.EvictUserCacheRequest, options ...grpc.CallOption) (*financepb.EvictUserCacheResponse, error) {
	return s.evict(ctx, request, options...)
}

func TestGRPCPeriodContextClient_EvictsUserCache(t *testing.T) {
	stub := &financeClientStub{
		evict: func(_ context.Context, request *financepb.EvictUserCacheRequest, _ ...grpc.CallOption) (*financepb.EvictUserCacheResponse, error) {
			require.Equal(t, "user-1", request.GetUserId())
			return &financepb.EvictUserCacheResponse{}, nil
		},
	}

	err := (&GRPCPeriodContextClient{client: stub}).EvictUserCache(context.Background(), "user-1")

	require.NoError(t, err)
}

func TestGRPCPeriodContextClient_EvictUserCacheWrapsError(t *testing.T) {
	stub := &financeClientStub{
		evict: func(context.Context, *financepb.EvictUserCacheRequest, ...grpc.CallOption) (*financepb.EvictUserCacheResponse, error) {
			return nil, errors.New("finance unavailable")
		},
	}

	err := (&GRPCPeriodContextClient{client: stub}).EvictUserCache(context.Background(), "user-1")

	require.Error(t, err)
	require.ErrorContains(t, err, "gRPC EvictUserCache")
}
