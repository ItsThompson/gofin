package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

func TestGetSpendingByTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{TagID: "tag-food", Amount: 30000, ReportingAmount: 30000},
			{TagID: "tag-food", Amount: 20000, ReportingAmount: 20000},
			{TagID: "tag-bills", Amount: 50000, ReportingAmount: 50000},
		}, nil)

	repo.On("ListTags", mock.Anything, "user-123").
		Return([]*model.Tag{
			{ID: "tag-food", Name: "Food"},
			{ID: "tag-bills", Name: "Bills"},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/by-tag?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagSpendingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.TagSpending, 2)
	// Food and Bills both total 50000; tie order is map-iteration dependent, so
	// assert per-tag amounts rather than slice position.
	var foodTag, billsTag model.TagSpending
	for _, tag := range resp.TagSpending {
		switch tag.TagName {
		case "Food":
			foodTag = tag
		case "Bills":
			billsTag = tag
		}
	}
	assert.Equal(t, int64(50000), foodTag.Amount)
	assert.Equal(t, int64(50000), billsTag.Amount)
	assert.InDelta(t, 50.0, foodTag.PercentOfTotal, 0.01)
	assert.InDelta(t, 50.0, billsTag.PercentOfTotal, 0.01)
}

func TestGetSpendingByTagHandler_NoExpenses(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:   "period-abc",
			Year: 2025, Month: 1,
			BudgetAmount:      300000,
			EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{}, nil)

	repo.On("ListTags", mock.Anything, "user-123").
		Return([]*model.Tag{}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/by-tag?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagSpendingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.TagSpending)
}

func TestGetCumulativeSpendHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      310000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{ExpenseDate: "2025-01-01", Amount: 10000, ReportingAmount: 10000},
			{ExpenseDate: "2025-01-03", Amount: 20000, ReportingAmount: 20000},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/cumulative?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.CumulativeSpendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Points, 31)                       // January has 31 days
	assert.Equal(t, int64(10000), resp.Points[0].Actual) // day 1
	assert.Equal(t, int64(10000), resp.Points[1].Actual) // day 2 (carry forward)
	assert.Equal(t, int64(30000), resp.Points[2].Actual) // day 3
	assert.Equal(t, int64(10000), resp.Points[0].Ideal)  // 310000/31*1 = 10000
}

func TestGetCumulativeSpendHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/cumulative", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Tag CRUD Handler Tests ---

func TestListTagsHandler_LazySeeds(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	// User has 0 tags: triggers lazy seeding
	repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(0), nil)
	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	for _, tagName := range service.DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-1", tagName, true).
			Return(&model.Tag{ID: "t-" + tagName, Name: tagName, IsDefault: true}, nil)
	}

	repo.On("ListTags", mock.Anything, "user-1").
		Return([]*model.Tag{{ID: "t-Bills", Name: "Bills", IsDefault: true}}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "GET", "/api/finance/tags", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Tags, 1)
	assert.Equal(t, "Bills", resp.Tags[0].Name)
	tx.AssertCalled(t, "Commit", mock.Anything)
}

func TestListTagsHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/tags", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("CreateTag", mock.Anything, "user-1", "Groceries", false).
		Return(&model.Tag{ID: "tag-new", Name: "Groceries", IsDefault: false}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", map[string]interface{}{
		"name": "Groceries",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.TagResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Groceries", resp.Tag.Name)
	assert.False(t, resp.Tag.IsDefault)
}

func TestCreateTagHandler_DuplicateName(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	// Simulate PG unique constraint violation
	pgErr := &pgconn.PgError{Code: "23505"}
	repo.On("CreateTag", mock.Anything, "user-1", "Bills", false).
		Return(nil, pgErr)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", map[string]interface{}{
		"name": "Bills",
	})

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDuplicateTag, errResp.Code)
}

func TestCreateTagHandler_MissingBody(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "", map[string]interface{}{"name": "Test"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("UpdateTag", mock.Anything, "tag-1", "user-1", "Renamed").
		Return(&model.Tag{ID: "tag-1", Name: "Renamed", IsDefault: true}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/tag-1", "user-1", map[string]interface{}{
		"name": "Renamed",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Renamed", resp.Tag.Name)
}

func TestUpdateTagHandler_NotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("UpdateTag", mock.Anything, "nonexistent", "user-1", "Test").
		Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/nonexistent", "user-1", map[string]interface{}{
		"name": "Test",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/tag-1", "", map[string]interface{}{"name": "X"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-c", "user-1").
		Return(&model.Tag{ID: "tag-c", Name: "Custom", IsDefault: false}, nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-c").
		Return(int64(0), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-c", "user-1").
		Return(int64(0), nil)
	repo.On("DeleteTag", mock.Anything, "tag-c", "user-1").
		Return(nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "user-1", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertCalled(t, "DeleteTag", mock.Anything, "tag-c", "user-1")
}

func TestDeleteTagHandler_DefaultTagForbidden(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-bills", "user-1").
		Return(&model.Tag{ID: "tag-bills", Name: "Bills", IsDefault: true}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-bills", "user-1", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDefaultTag, errResp.Code)
	assert.Contains(t, errResp.Message, "Default tags cannot be deleted")
}

func TestDeleteTagHandler_TagInUse(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-c", "user-1").
		Return(&model.Tag{ID: "tag-c", Name: "Custom", IsDefault: false}, nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-c").
		Return(int64(5), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-c", "user-1").
		Return(int64(0), nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "user-1", nil)

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrTagInUse, errResp.Code)
	assert.Contains(t, errResp.Message, "5 expense(s)")
}

func TestDeleteTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
