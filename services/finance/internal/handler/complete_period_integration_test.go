package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

type completePeriodExpenseClient struct {
	expensepb.ExpenseServiceClient
	rows  []*expensepb.ExpenseData
	calls int
}

func (c *completePeriodExpenseClient) GetActiveExpensesForPeriodPage(_ context.Context, request *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
	c.calls++
	start := 0
	switch request.GetCursorId() {
	case "":
		start = 0
	case "expense-050":
		start = 50
	case "expense-100":
		start = 100
	default:
		return &expensepb.CompleteExpensePageResponse{}, nil
	}

	end := start + 50
	if end > len(c.rows) {
		end = len(c.rows)
	}
	response := &expensepb.CompleteExpensePageResponse{Data: c.rows[start:end]}
	if end < len(c.rows) {
		last := c.rows[end-1]
		response.HasMore = true
		response.NextExpenseDate = last.GetExpenseDateIso()
		response.NextCreatedAt = last.GetCreatedAt()
		response.NextId = last.GetId()
	}
	return response, nil
}

func TestDashboardRESTUsesEveryCompletePeriodPage(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	period := &model.BudgetPeriod{
		ID: "period-complete", UserID: "user-123", Year: 2025, Month: 1,
		BudgetAmount: 500000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}
	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).Return(period, nil).Twice()
	repo.On("ListTags", mock.Anything, "user-123").Return([]*model.Tag{{ID: "tag-food", Name: "Food"}}, nil)

	rows := make([]*expensepb.ExpenseData, 0, 101)
	for i := 1; i <= 100; i++ {
		rows = append(rows, &expensepb.ExpenseData{
			Id: "expense-" + formatThreeDigits(i), ExpenseType: "desires", TagId: "tag-food",
			ExpenseDateIso: "2025-01-01", CreatedAt: "2025-01-01T00:00:00Z", ReportingAmountInMinorUnits: 100,
		})
	}
	rows = append(rows, &expensepb.ExpenseData{
		Id: "expense-101", ExpenseType: "desires", TagId: "tag-food",
		ExpenseDateIso: "2025-01-01", CreatedAt: "2025-01-01T00:00:01Z", ReportingAmountInMinorUnits: 9900,
	})
	expenseClient := &completePeriodExpenseClient{rows: rows}
	r := setupTestRouterWithExpenseClient(repo, txBeginner, service.NewGRPCExpenseClient(expenseClient))

	summaryResponse := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=1", "user-123", nil)
	require.Equal(t, http.StatusOK, summaryResponse.Code)
	var summary model.SummaryResponse
	require.NoError(t, json.Unmarshal(summaryResponse.Body.Bytes(), &summary))
	assert.Equal(t, int64(19900), summary.Summary.TotalSpent)

	tagResponse := doJSONWithUserID(r, "GET", "/api/finance/spending/by-tag?year=2025&month=1", "user-123", nil)
	require.Equal(t, http.StatusOK, tagResponse.Code)
	var tags model.TagSpendingResponse
	require.NoError(t, json.Unmarshal(tagResponse.Body.Bytes(), &tags))
	require.Len(t, tags.TagSpending, 1)
	assert.Equal(t, int64(19900), tags.TagSpending[0].Amount)
	assert.Equal(t, 6, expenseClient.calls)
}

func formatThreeDigits(value int) string {
	return fmt.Sprintf("%03d", value)
}
