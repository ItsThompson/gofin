package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

func TestGRPCExpenseClient_ForwardsCacheBypassAcrossCompleteReadPages(t *testing.T) {
	calls := 0
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(_ context.Context, request *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			calls++
			require.True(t, request.GetBypassCache())
			if calls == 1 {
				return &expensepb.CompleteExpensePageResponse{
					Data:            []*expensepb.ExpenseData{{Id: "expense-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"}},
					NextExpenseDate: "2026-05-01",
					NextCreatedAt:   "2026-05-01T00:00:00Z",
					NextId:          "expense-1",
					HasMore:         true,
				}, nil
			}
			return &expensepb.CompleteExpensePageResponse{}, nil
		},
	}

	_, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(WithCacheBypass(context.Background()), "user-1", 2026, 5)

	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
