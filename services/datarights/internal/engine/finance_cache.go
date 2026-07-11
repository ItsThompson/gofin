package engine

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// MemoizedFinanceClient is a per-job wrapper over the shared finance gRPC
// client. It embeds financepb.FinanceServiceClient (anonymous field) so it stays
// a drop-in for the full client interface, and overrides GetAllUserData to fetch
// at most once per instance.
//
// A fresh instance MUST be created per export job; it must NEVER be a startup
// singleton, since sync.Once would cache the first user's data and serve it to
// every subsequent user (a cross-user data leak).
type MemoizedFinanceClient struct {
	financepb.FinanceServiceClient // embedded: all other methods pass through

	once sync.Once
	data *financepb.AllUserDataResponse
	err  error
}

// NewMemoizedFinanceClient wraps inner so GetAllUserData is served at most once.
func NewMemoizedFinanceClient(inner financepb.FinanceServiceClient) *MemoizedFinanceClient {
	return &MemoizedFinanceClient{FinanceServiceClient: inner}
}

// GetAllUserData returns the memoized response, fetching from the wrapped client
// exactly once. Safe for concurrent callers: sync.Once serializes the fetch, so
// the later errgroup fan-out over providers still triggers a single RPC. All
// callers of an instance MUST pass the same request; the in and opts of calls
// after the first are ignored (safe today: each instance serves one
// single-user export job).
func (m *MemoizedFinanceClient) GetAllUserData(
	ctx context.Context, in *financepb.GetAllUserDataRequest, opts ...grpc.CallOption,
) (*financepb.AllUserDataResponse, error) {
	m.once.Do(func() {
		m.data, m.err = m.FinanceServiceClient.GetAllUserData(ctx, in, opts...)
	})
	return m.data, m.err
}
