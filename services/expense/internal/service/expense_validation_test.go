package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// validCreateReq and validCorrectReq build fully-valid requests for the
// expense validators. They mirror the helpers in expense_test.go but are kept
// local to the validation test file so each case only varies the field under
// test.
func validCreateReq() *model.CreateExpenseRequest {
	return &model.CreateExpenseRequest{
		Name:                                  "Grocery shopping",
		AmountInTransactionCurrencyMinorUnits: 2500,
		TransactionCurrencyCode:               "USD",
		ExpenseType:                           "essentials",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		ClientGeneratedIdempotencyKey:         validTestUUID,
	}
}

func validCorrectReq() *model.CorrectExpenseRequest {
	return &model.CorrectExpenseRequest{
		Name:                                  "Updated Coffee",
		AmountInTransactionCurrencyMinorUnits: 600,
		TransactionCurrencyCode:               "USD",
		ExpenseType:                           "desires",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
	}
}

// --- validateCreateExpenseRequest ---

func TestValidateCreateExpenseRequest_Valid(t *testing.T) {
	assert.Nil(t, validateCreateExpenseRequest(validCreateReq()))
}

func TestValidateCreateExpenseRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(req *model.CreateExpenseRequest)
		field   string
		message string
	}{
		{"missing name", func(r *model.CreateExpenseRequest) { r.Name = "" }, "name", "name is required"},
		{"zero amount", func(r *model.CreateExpenseRequest) { r.AmountInTransactionCurrencyMinorUnits = 0 }, "amountInTransactionCurrencyMinorUnits", "amount must be positive"},
		{"negative amount", func(r *model.CreateExpenseRequest) { r.AmountInTransactionCurrencyMinorUnits = -100 }, "amountInTransactionCurrencyMinorUnits", "amount must be positive"},
		{"invalid expense type", func(r *model.CreateExpenseRequest) { r.ExpenseType = "luxury" }, "expenseType", "expense_type must be one of: essentials, desires, savings"},
		{"missing tagId", func(r *model.CreateExpenseRequest) { r.TagID = "" }, "tagId", "tag_id is required"},
		{"missing expenseDate", func(r *model.CreateExpenseRequest) { r.ExpenseDateIso = "" }, "expenseDateIso", "expense_date is required"},
		{"invalid date format", func(r *model.CreateExpenseRequest) { r.ExpenseDateIso = "05/03/2026" }, "expenseDateIso", "expense_date must be in ISO format (YYYY-MM-DD)"},
		{"zero periodYear", func(r *model.CreateExpenseRequest) { r.PeriodYear = 0 }, "periodYear", "period_year must be positive"},
		{"zero periodMonth", func(r *model.CreateExpenseRequest) { r.PeriodMonth = 0 }, "periodMonth", "period_month must be between 1 and 12"},
		{"periodMonth 13", func(r *model.CreateExpenseRequest) { r.PeriodMonth = 13 }, "periodMonth", "period_month must be between 1 and 12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateReq()
			tt.modify(req)

			verr := validateCreateExpenseRequest(req)
			require.NotNil(t, verr)
			assert.Equal(t, apierr.CodeValidation, verr.Code)
			assert.Equal(t, "validation failed", verr.Message)
			assert.Equal(t, tt.message, verr.Fields[tt.field])
		})
	}
}

func TestValidateCreateExpenseRequest_MultipleInvalid(t *testing.T) {
	req := validCreateReq()
	req.Name = ""
	req.AmountInTransactionCurrencyMinorUnits = 0
	req.ExpenseType = "luxury"

	verr := validateCreateExpenseRequest(req)
	require.NotNil(t, verr)
	assert.Equal(t, apierr.CodeValidation, verr.Code)
	assert.Equal(t, "validation failed", verr.Message)
	// Aggregation: all three invalid fields appear in one pass.
	assert.Contains(t, verr.Fields, "name")
	assert.Contains(t, verr.Fields, "amountInTransactionCurrencyMinorUnits")
	assert.Contains(t, verr.Fields, "expenseType")
	assert.Len(t, verr.Fields, 3)
}

// --- validateCorrectExpenseRequest ---

func TestValidateCorrectExpenseRequest_Valid(t *testing.T) {
	assert.Nil(t, validateCorrectExpenseRequest(validCorrectReq()))
}

func TestValidateCorrectExpenseRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(req *model.CorrectExpenseRequest)
		field   string
		message string
	}{
		{"missing name", func(r *model.CorrectExpenseRequest) { r.Name = "" }, "name", "name is required"},
		{"zero amount", func(r *model.CorrectExpenseRequest) { r.AmountInTransactionCurrencyMinorUnits = 0 }, "amountInTransactionCurrencyMinorUnits", "amount must be positive"},
		{"invalid expense type", func(r *model.CorrectExpenseRequest) { r.ExpenseType = "luxury" }, "expenseType", "expense_type must be one of: essentials, desires, savings"},
		{"missing tagId", func(r *model.CorrectExpenseRequest) { r.TagID = "" }, "tagId", "tag_id is required"},
		{"missing expenseDate", func(r *model.CorrectExpenseRequest) { r.ExpenseDateIso = "" }, "expenseDateIso", "expense_date is required"},
		{"invalid date format", func(r *model.CorrectExpenseRequest) { r.ExpenseDateIso = "2026/05/03" }, "expenseDateIso", "expense_date must be in ISO format (YYYY-MM-DD)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCorrectReq()
			tt.modify(req)

			verr := validateCorrectExpenseRequest(req)
			require.NotNil(t, verr)
			assert.Equal(t, apierr.CodeValidation, verr.Code)
			assert.Equal(t, "validation failed", verr.Message)
			assert.Equal(t, tt.message, verr.Fields[tt.field])
		})
	}
}

func TestValidateCorrectExpenseRequest_MultipleInvalid(t *testing.T) {
	req := validCorrectReq()
	req.Name = ""
	req.TagID = ""
	req.ExpenseDateIso = ""

	verr := validateCorrectExpenseRequest(req)
	require.NotNil(t, verr)
	assert.Equal(t, apierr.CodeValidation, verr.Code)
	// Aggregation: all three invalid fields appear. The empty date surfaces the
	// "required" message (first-error-per-field: the ISO-format check also fails
	// for "" but is recorded second, so "required" wins).
	assert.Equal(t, "name is required", verr.Fields["name"])
	assert.Equal(t, "tag_id is required", verr.Fields["tagId"])
	assert.Equal(t, "expense_date is required", verr.Fields["expenseDateIso"])
	assert.Len(t, verr.Fields, 3)
}

// --- validateIdempotencyKey ---

func TestValidateIdempotencyKey_Valid(t *testing.T) {
	assert.Nil(t, validateIdempotencyKey(validTestUUID))
}

func TestValidateIdempotencyKey_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		message string
	}{
		{"empty key reports required only", "", "clientGeneratedIdempotencyKey is required"},
		{"too-long key reports length only", "550e8400-e29b-41d4-a716-446655440000-extra", "clientGeneratedIdempotencyKey must be at most 36 characters"},
		{"non-UUID of valid length reports UUID only", "not-a-valid-uuid-but-36-chars-long!!", "clientGeneratedIdempotencyKey must be a valid UUID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := validateIdempotencyKey(tt.key)
			require.NotNil(t, verr)
			assert.Equal(t, apierr.CodeValidation, verr.Code)
			assert.Equal(t, "validation failed", verr.Message)
			// First-error-per-field: only one entry for the single field.
			assert.Len(t, verr.Fields, 1)
			assert.Equal(t, tt.message, verr.Fields["clientGeneratedIdempotencyKey"])
		})
	}
}
