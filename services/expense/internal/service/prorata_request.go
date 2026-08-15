package service

import (
	"net/http"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/shared/validator"
)

// TrustedPeriodContext is period context Finance resolved before asking Expense
// to write a pro-rata installment. Expense validates its consistency but never
// calls Finance again for Finance-originated writes.
type TrustedPeriodContext struct {
	PeriodID          string
	UserID            string
	Year              int32
	Month             int32
	ReportingCurrency string
	Source            string
}

// CapturedRateSnapshot is the USD-based provider snapshot captured at schedule
// creation and used to derive installment reporting amounts without live rates.
type CapturedRateSnapshot struct {
	SnapshotVersion int32
	Source          string
	BaseCurrency    string
	RateTimestamp   string
	CapturedAt      string
	ExpiresAt       string
	RatesByCurrency map[string]string
}

// CreateProRataInstallmentRequest is the Finance-originated internal write
// contract. It carries trusted period context and the captured snapshot so the
// Expense service can write a ledger row without a Finance round trip.
type CreateProRataInstallmentRequest struct {
	UserID               string
	PeriodContext        TrustedPeriodContext
	Name                 string
	Amount               int64
	TransactionCurrency  string
	ExpenseType          string
	TagID                string
	ExpenseDate          string
	ProRataGroup         string
	ProRataIndex         int32
	ProRataTotal         int32
	CapturedRateSnapshot *CapturedRateSnapshot
	// LegacyMigration marks a legacy pending pro-rata schedule that has no
	// captured snapshot. When true, the transaction currency must equal the
	// period reporting currency and Expense writes a migration snapshot
	// (exchangeRate = "1", exchangeRateSource = "migration") without calling FX.
	LegacyMigration bool
}

func validateProRataInstallmentRequest(req *CreateProRataInstallmentRequest) *apierr.Error {
	v := validator.New()

	v.Check(req.UserID != "", "userId", "user_id is required")
	v.Check(req.Name != "", "name", "name is required")
	v.Check(req.Amount > 0, "amount", "amount must be positive")
	v.Check(model.ValidExpenseTypes[req.ExpenseType], "expenseType", "expense_type must be one of: essentials, desires, savings")
	v.Check(req.TagID != "", "tagId", "tag_id is required")
	v.Check(req.ExpenseDate != "", "expenseDate", "expense_date is required")
	v.Check(isoDateRegex.MatchString(req.ExpenseDate), "expenseDate", "expense_date must be in ISO format (YYYY-MM-DD)")
	v.Check(req.ProRataGroup != "", "proRataGroup", "pro_rata_group is required")
	v.Check(req.ProRataIndex >= 1, "proRataIndex", "pro_rata_index must be positive")
	v.Check(req.ProRataTotal >= req.ProRataIndex, "proRataTotal", "pro_rata_total must be >= pro_rata_index")

	if v.HasErrors() {
		return apierr.Validation("validation failed", v.Errors())
	}
	return nil
}

// validateTrustedPeriodContext enforces the trusted-context invariants: the
// context must originate from Finance, belong to the requesting user, and
// carry a valid year/month. A user mismatch is a security invariant breach
// (403 FORBIDDEN); a structural violation is an internal contract violation
// (500 INTERNAL_SERVER_ERROR). Neither is user input, so neither is a 400
// VALIDATION_ERROR.
func validateTrustedPeriodContext(userID string, pc TrustedPeriodContext) *apierr.Error {
	if pc.UserID != userID {
		return &apierr.Error{
			Code:    apierr.CodeForbidden,
			Message: "trusted period context user does not match the request user",
			Status:  http.StatusForbidden,
			Fields:  map[string]string{"userId": "mismatch"},
		}
	}
	v := validator.New()
	v.Check(pc.Source == "finance_service", "source", "must be finance_service")
	v.Check(pc.PeriodID != "", "periodId", "required")
	v.Check(pc.Year >= 1, "year", "must be positive")
	v.Check(pc.Month >= 1 && pc.Month <= 12, "month", "must be between 1 and 12")
	if v.HasErrors() {
		return internalContractError("trusted period context is invalid", v.Errors())
	}
	return nil
}

// internalContractError builds a 500 for a trusted-context structural
// violation: the context is Finance's contract to uphold, so a malformed one
// is an internal bug, not caller input.
func internalContractError(msg string, fields map[string]string) *apierr.Error {
	return &apierr.Error{
		Code:    apierr.CodeInternal,
		Message: msg,
		Status:  http.StatusInternalServerError,
		Fields:  fields,
	}
}

func validateSnapshotCoverage(snapshot *CapturedRateSnapshot, currencies ...string) *apierr.Error {
	v := validator.New()
	v.Check(snapshot != nil && len(snapshot.RatesByCurrency) > 0, "snapshot", "snapshot is required")
	if snapshot != nil {
		for _, currency := range currencies {
			_, ok := snapshot.RatesByCurrency[currency]
			v.Check(ok, currency, "currency is missing from the snapshot")
		}
	}
	if v.HasErrors() {
		return snapshotCurrencyMissingError()
	}
	return nil
}

func snapshotCurrencyMissingError() *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrSnapshotCurrencyMissing,
		Message: "The captured rate snapshot does not contain a required currency",
		Status:  http.StatusConflict,
	}
}
