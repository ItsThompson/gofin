package engine_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestMemoizedFinanceClient_FetchesAtMostOnce(t *testing.T) {
	inner := newFinanceSpy(cannedAllUserData(), cannedTagList())
	mc := engine.NewMemoizedFinanceClient(inner)

	for range 5 {
		resp, err := mc.GetAllUserData(context.Background(), &financepb.GetAllUserDataRequest{UserId: "user-1"})
		require.NoError(t, err)
		assert.Equal(t, cannedAllUserData().GetTags(), resp.GetTags())
	}

	assert.Equal(t, 1, inner.Count("GetAllUserData"), "inner client should be hit exactly once")
}

func TestMemoizedFinanceClient_ConcurrentCallsFetchOnce(t *testing.T) {
	inner := newFinanceSpy(cannedAllUserData(), cannedTagList())
	mc := engine.NewMemoizedFinanceClient(inner)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			resp, err := mc.GetAllUserData(context.Background(), &financepb.GetAllUserDataRequest{UserId: "user-1"})
			assert.NoError(t, err)
			assert.NotNil(t, resp)
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, inner.Count("GetAllUserData"),
		"sync.Once must serialize concurrent fan-out callers into one fetch")
}

func TestMemoizedFinanceClient_MemoizesError(t *testing.T) {
	wantErr := errors.New("finance unavailable")
	inner := newFinanceSpy(nil, cannedTagList())
	inner.dataErr = wantErr
	mc := engine.NewMemoizedFinanceClient(inner)

	for range 3 {
		_, err := mc.GetAllUserData(context.Background(), &financepb.GetAllUserDataRequest{UserId: "user-1"})
		require.ErrorIs(t, err, wantErr)
	}

	assert.Equal(t, 1, inner.Count("GetAllUserData"), "a failed fetch is memoized too, not retried")
}

func TestMemoizedFinanceClient_PassesThroughOtherMethods(t *testing.T) {
	inner := newFinanceSpy(cannedAllUserData(), cannedTagList())
	mc := engine.NewMemoizedFinanceClient(inner)

	// ListTags is not overridden, so it must delegate to the embedded client.
	resp, err := mc.ListTags(context.Background(), &financepb.ListTagsRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, cannedTags(), resp.GetTags())
	assert.Equal(t, 1, inner.Count("ListTags"))
	assert.Equal(t, 0, inner.Count("GetAllUserData"))
}
