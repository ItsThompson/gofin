package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
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

func (m *mockExpenseRepository) GetExpenseByID(ctx context.Context, id string, userID string) (*model.Expense, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	args := m.Called(ctx, userID, tagID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseRepository) CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, original, correction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, expenseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, groupID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetActiveExpenseSuggestionInputs(ctx context.Context, userID string) ([]*model.ExpenseSuggestionInput, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.ExpenseSuggestionInput), args.Error(1)
}

func (m *mockExpenseRepository) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockExpenseRepository) GetExpensesByUserAfter(ctx context.Context, userID string, cursor repository.ExpenseCursor, pageSize int32) ([]*model.Expense, repository.ExpenseCursor, bool, error) {
	args := m.Called(ctx, userID, cursor, pageSize)
	var rows []*model.Expense
	if args.Get(0) != nil {
		rows = args.Get(0).([]*model.Expense)
	}
	return rows, args.Get(1).(repository.ExpenseCursor), args.Bool(2), args.Error(3)
}

func newTestService(repo *mockExpenseRepository) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, time.Now, logger)
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

	repo.On("GetExpenseByID", mock.Anything, "exp-123", "user-1").
		Return(&model.Expense{ID: "exp-123", UserID: "user-1", Name: "Coffee", Status: "active"}, nil)

	expense, err := svc.GetExpense(context.Background(), "user-1", "exp-123")

	require.NoError(t, err)
	assert.Equal(t, "exp-123", expense.ID)
}

func TestGetExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-999", "user-1").Return(nil, nil)

	_, err := svc.GetExpense(context.Background(), "user-1", "exp-999")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestGetExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetExpense(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

// --- CorrectExpense tests ---

func activeExpenseInCurrentPeriod(now time.Time) *model.Expense {
	return &model.Expense{
		ID:          "exp-original",
		UserID:      "user-1",
		Name:        "Coffee",
		Amount:      500,
		Currency:    "USD",
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDate: now.Format("2006-01-02"),
		PeriodYear:  int32(now.Year()),
		PeriodMonth: int32(now.Month()),
		Status:      "active",
		CreatedAt:   now.Format(time.RFC3339),
	}
}

func validCorrectRequest() *model.CorrectExpenseRequest {
	return &model.CorrectExpenseRequest{
		Name:        "Updated Coffee",
		Amount:      600,
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDate: "2026-05-03",
	}
}

func newTestServiceWithClock(repo *mockExpenseRepository, now time.Time) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, func() time.Time { return now }, logger)
}

func TestCorrectExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	original := activeExpenseInCurrentPeriod(now)
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)

	repo.On("CorrectExpense", mock.Anything, original, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			correction := args.Get(2).(*model.Expense)
			// Verify correction fields
			assert.Equal(t, "Updated Coffee", correction.Name)
			assert.Equal(t, int64(600), correction.Amount)
			assert.Equal(t, "active", correction.Status)
			assert.Equal(t, "exp-original", correction.CorrectsID)
			assert.Equal(t, "USD", correction.Currency) // Inherited
			assert.Equal(t, original.PeriodYear, correction.PeriodYear)
			assert.Equal(t, original.PeriodMonth, correction.PeriodMonth)
			assert.NotEmpty(t, correction.ID)
			assert.NotEqual(t, original.ID, correction.ID)
		}).
		Return(&model.Expense{
			ID:          "exp-correction",
			UserID:      "user-1",
			Name:        "Updated Coffee",
			Amount:      600,
			Currency:    "USD",
			ExpenseType: "desires",
			TagID:       "tag-food",
			ExpenseDate: "2026-05-03",
			PeriodYear:  2026,
			PeriodMonth: 5,
			Status:      "active",
			CorrectsID:  "exp-original",
			CreatedAt:   "2026-05-03T10:00:00Z",
		}, nil)

	result, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", validCorrectRequest())

	require.NoError(t, err)
	assert.Equal(t, "exp-correction", result.ID)
	assert.Equal(t, "active", result.Status)
	assert.Equal(t, "exp-original", result.CorrectsID)
	assert.Equal(t, int64(600), result.Amount)
	repo.AssertExpectations(t)
}

func TestCorrectExpense_AlreadyCorrected(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	corrected := activeExpenseInCurrentPeriod(now)
	corrected.Status = "corrected"

	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(corrected, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", validCorrectRequest())

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrAlreadyCorrected, svcErr.Code)
	assert.Equal(t, 409, svcErr.Status)
}

func TestCorrectExpense_PeriodLocked(t *testing.T) {
	repo := new(mockExpenseRepository)
	// Clock is May 2026, but expense is from April 2026
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	pastExpense := &model.Expense{
		ID:          "exp-past",
		UserID:      "user-1",
		Name:        "Old Coffee",
		Amount:      500,
		Currency:    "USD",
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDate: "2026-04-15",
		PeriodYear:  2026,
		PeriodMonth: 4, // Past period
		Status:      "active",
		CreatedAt:   "2026-04-15T10:00:00Z",
	}

	repo.On("GetExpenseByID", mock.Anything, "exp-past", "user-1").Return(pastExpense, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-past", validCorrectRequest())

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrPeriodLocked, svcErr.Code)
	assert.Equal(t, 403, svcErr.Status)
}

func TestCorrectExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	repo.On("GetExpenseByID", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-missing", validCorrectRequest())

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestCorrectExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "", validCorrectRequest())

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

func TestCorrectExpense_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		modify func(req *model.CorrectExpenseRequest)
		field  string
	}{
		{"missing name", func(req *model.CorrectExpenseRequest) { req.Name = "" }, "name"},
		{"zero amount", func(req *model.CorrectExpenseRequest) { req.Amount = 0 }, "amount"},
		{"invalid type", func(req *model.CorrectExpenseRequest) { req.ExpenseType = "luxury" }, "expenseType"},
		{"missing tagId", func(req *model.CorrectExpenseRequest) { req.TagID = "" }, "tagId"},
		{"missing date", func(req *model.CorrectExpenseRequest) { req.ExpenseDate = "" }, "expenseDate"},
		{"invalid date format", func(req *model.CorrectExpenseRequest) { req.ExpenseDate = "05/03/2026" }, "expenseDate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			req := validCorrectRequest()
			tt.modify(req)

			_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-1", req)

			require.Error(t, err)
			svcErr, ok := err.(*ServiceError)
			require.True(t, ok)
			assert.Equal(t, model.ErrValidationError, svcErr.Code)
			assert.NotEmpty(t, svcErr.Fields[tt.field])
		})
	}
}

// --- GetCorrectionHistory tests ---

func TestGetCorrectionHistory_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	chain := []*model.Expense{
		{ID: "exp-1", Name: "Original", Status: "corrected", CorrectsID: ""},
		{ID: "exp-2", Name: "Correction 1", Status: "corrected", CorrectsID: "exp-1"},
		{ID: "exp-3", Name: "Correction 2", Status: "active", CorrectsID: "exp-2"},
	}

	repo.On("GetCorrectionHistory", mock.Anything, "exp-2", "user-1").Return(chain, nil)

	result, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-2")

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "exp-1", result[0].ID)
	assert.Equal(t, "exp-2", result[1].ID)
	assert.Equal(t, "exp-3", result[2].ID)
}

func TestGetCorrectionHistory_SingleEntry(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	chain := []*model.Expense{
		{ID: "exp-1", Name: "Standalone", Status: "active", CorrectsID: ""},
	}

	repo.On("GetCorrectionHistory", mock.Anything, "exp-1", "user-1").Return(chain, nil)

	result, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-1")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "exp-1", result[0].ID)
}

func TestGetCorrectionHistory_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetCorrectionHistory", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	_, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-missing")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
}

func TestGetCorrectionHistory_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetCorrectionHistory(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

// --- GetProRataGroup tests ---

func TestGetProRataGroup_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", ProRataIndex: 1, ProRataTotal: 3},
		{ID: "exp-2", ProRataIndex: 2, ProRataTotal: 3},
	}

	repo.On("GetProRataGroup", mock.Anything, "group-1", "user-1").Return(expenses, nil)

	result, err := svc.GetProRataGroup(context.Background(), "user-1", "group-1")

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetProRataGroup_EmptyGroupID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetProRataGroup(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

// --- AnonymizeAllUserExpenses tests ---

func TestAnonymizeAllUserExpenses_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAnonymizeAllUserExpenses_Idempotent(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	// Calling twice should succeed both times (repo returns nil both times)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")
	require.NoError(t, err)

	err = svc.AnonymizeAllUserExpenses(context.Background(), "user-1")
	require.NoError(t, err)

	repo.AssertNumberOfCalls(t, "AnonymizeAllUserExpenses", 2)
}

func TestAnonymizeAllUserExpenses_EmptyUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "")

	require.Error(t, err)
	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
}

func TestAnonymizeAllUserExpenses_NoExpenses(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	// User has no expenses: repo returns nil (0 rows updated is not an error)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-no-expenses").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-no-expenses")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAnonymizeAllUserExpenses_DatabaseFailure(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").
		Return(fmt.Errorf("connection refused"))

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "anonymizing user expenses")
}
