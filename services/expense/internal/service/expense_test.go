package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// mockExpenseRepository implements repository.ExpenseRepository for tests.
type mockExpenseRepository struct {
	mock.Mock
}

func (m *mockExpenseRepository) CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, expense)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error) {
	args := m.Called(ctx, userID, year, month, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*model.Expense), args.Get(1).(int64), args.Error(2)
}

func (m *mockExpenseRepository) GetExpenseByID(ctx context.Context, id string) (*model.Expense, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func newTestService(repo *mockExpenseRepository) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, logger)
}

func validCreateRequest() *model.CreateExpenseRequest {
	return &model.CreateExpenseRequest{
		Name:        "Grocery shopping",
		Amount:      2500,
		Currency:    "USD",
		ExpenseType: "essentials",
		TagID:       "tag-food",
		ExpenseDate: "2026-05-03",
		PeriodYear:  2026,
		PeriodMonth: 5,
	}
}

// --- CreateExpense validation tests ---

func TestCreateExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Return(&model.Expense{
			ID:          "exp-123",
			UserID:      "user-1",
			Name:        "Grocery shopping",
			Amount:      2500,
			Currency:    "USD",
			ExpenseType: "essentials",
			TagID:       "tag-food",
			ExpenseDate: "2026-05-03",
			PeriodYear:  2026,
			PeriodMonth: 5,
			Status:      "active",
		}, nil)

	expense, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())

	require.NoError(t, err)
	assert.Equal(t, "exp-123", expense.ID)
	assert.Equal(t, "user-1", expense.UserID)
	assert.Equal(t, int64(2500), expense.Amount)
	assert.Equal(t, "essentials", expense.ExpenseType)
	assert.Equal(t, "active", expense.Status)
}

func TestCreateExpense_AmountMustBePositive(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	tests := []struct {
		name   string
		amount int64
	}{
		{"zero amount", 0},
		{"negative amount", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateRequest()
			req.Amount = tt.amount

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr, ok := err.(*ServiceError)
			require.True(t, ok)
			assert.Equal(t, model.ErrValidationError, svcErr.Code)
			assert.Equal(t, 400, svcErr.Status)
		})
	}
}

func TestCreateExpense_RequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(req *model.CreateExpenseRequest)
	}{
		{"missing name", func(req *model.CreateExpenseRequest) { req.Name = "" }},
		{"missing currency", func(req *model.CreateExpenseRequest) { req.Currency = "" }},
		{"missing tagId", func(req *model.CreateExpenseRequest) { req.TagID = "" }},
		{"missing expenseDate", func(req *model.CreateExpenseRequest) { req.ExpenseDate = "" }},
		{"zero periodYear", func(req *model.CreateExpenseRequest) { req.PeriodYear = 0 }},
		{"zero periodMonth", func(req *model.CreateExpenseRequest) { req.PeriodMonth = 0 }},
		{"periodMonth 13", func(req *model.CreateExpenseRequest) { req.PeriodMonth = 13 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			req := validCreateRequest()
			tt.modify(req)

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr, ok := err.(*ServiceError)
			require.True(t, ok)
			assert.Equal(t, model.ErrValidationError, svcErr.Code)
		})
	}
}

func TestCreateExpense_InvalidExpenseType(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	req := validCreateRequest()
	req.ExpenseType = "luxury"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

func TestCreateExpense_ValidExpenseTypes(t *testing.T) {
	for _, expenseType := range []string{"essentials", "desires", "savings"} {
		t.Run(expenseType, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
				Return(&model.Expense{
					ID:          "exp-123",
					UserID:      "user-1",
					ExpenseType: expenseType,
					Status:      "active",
				}, nil)

			req := validCreateRequest()
			req.ExpenseType = expenseType

			expense, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.NoError(t, err)
			assert.Equal(t, expenseType, expense.ExpenseType)
		})
	}
}

func TestCreateExpense_InvalidDateFormat(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	tests := []struct {
		name string
		date string
	}{
		{"wrong format", "05/03/2026"},
		{"datetime", "2026-05-03T12:00:00Z"},
		{"partial", "2026-05"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateRequest()
			req.ExpenseDate = tt.date

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr, ok := err.(*ServiceError)
			require.True(t, ok)
			assert.Equal(t, model.ErrValidationError, svcErr.Code)
		})
	}
}

// --- GetExpensesForPeriod tests ---

func TestGetExpensesForPeriod_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries", Amount: 5000, Status: "active"},
		{ID: "exp-2", Name: "Coffee", Amount: 500, Status: "active"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return(expenses, int64(2), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     1,
		PageSize: 50,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(50), result.PageSize)
	assert.False(t, result.HasMore)
}

func TestGetExpensesForPeriod_HasMore(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(1)).
		Return(expenses, int64(5), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     1,
		PageSize: 1,
	})

	require.NoError(t, err)
	assert.True(t, result.HasMore)
	assert.Equal(t, int64(5), result.Total)
}

func TestGetExpensesForPeriod_DefaultsPagination(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return([]*model.Expense{}, int64(0), nil)

	// page=0 and pageSize=0 should be defaulted
	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     0,
		PageSize: 0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(50), result.PageSize)
}

func TestGetExpensesForPeriod_InvalidMonth(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID: "user-1",
		Year:   2026,
		Month:  13,
	})

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

// --- GetExpense tests ---

func TestGetExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-123").
		Return(&model.Expense{ID: "exp-123", Name: "Coffee", Status: "active"}, nil)

	expense, err := svc.GetExpense(context.Background(), "exp-123")

	require.NoError(t, err)
	assert.Equal(t, "exp-123", expense.ID)
}

func TestGetExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-999").Return(nil, nil)

	_, err := svc.GetExpense(context.Background(), "exp-999")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestGetExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetExpense(context.Background(), "")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}
