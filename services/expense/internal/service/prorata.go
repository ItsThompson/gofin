package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
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
}

// CreateProRataInstallment writes a Finance-originated pro-rata installment
// ledger row. Unlike public CreateExpense, it does not call the Finance period
// context client: the period context and captured snapshot arrive from Finance
// and are validated here before any ledger write.
func (s *ExpenseService) CreateProRataInstallment(ctx context.Context, req *CreateProRataInstallmentRequest) (*model.Expense, error) {
	if err := validateProRataInstallmentRequest(req); err != nil {
		return nil, err
	}

	if err := validateTrustedPeriodContext(req.UserID, req.PeriodContext); err != nil {
		return nil, err
	}

	reportingCurrency := normalizeCurrencyCode(req.PeriodContext.ReportingCurrency)
	if err := validateReportingCurrency(reportingCurrency); err != nil {
		return nil, err
	}

	transactionCurrency, err := s.validateTransactionCurrency(normalizeCurrencyCode(req.TransactionCurrency))
	if err != nil {
		return nil, err
	}

	if err := validateSnapshotCoverage(req.CapturedRateSnapshot, transactionCurrency, reportingCurrency); err != nil {
		return nil, err
	}

	now := s.clock().UTC().Format(time.RFC3339)

	// Pro-rata installments always derive their reporting amount from the
	// captured snapshot (spec 06), so same-currency first installments also go
	// through ConvertWithSnapshot. FX returns source/timestamp from the snapshot,
	// which keeps the first installment on the same snapshot facts as future rows.
	fxResp, convErr := s.fxClient.ConvertWithSnapshot(ctx, FxConvertWithSnapshotRequest{
		Amount:         req.Amount,
		SourceCurrency: transactionCurrency,
		TargetCurrency: reportingCurrency,
		RequestedAt:    now,
		Snapshot:       toFxCapturedRateSnapshot(req.CapturedRateSnapshot),
	})
	if convErr != nil {
		return nil, s.handleFxConversionFailure(convErr, transactionCurrency, reportingCurrency)
	}

	snapshot := buildProviderSnapshot(req.Amount, transactionCurrency, reportingCurrency, fxResp)

	expense := &model.Expense{
		ID:                    uuid.New().String(),
		UserID:                req.UserID,
		Name:                  req.Name,
		TransactionCurrency:   transactionCurrency,
		ExpenseType:           req.ExpenseType,
		TagID:                 req.TagID,
		ExpenseDate:           req.ExpenseDate,
		PeriodYear:            req.PeriodContext.Year,
		PeriodMonth:           req.PeriodContext.Month,
		Status:                "active",
		CorrectsID:            "",
		IsProRata:             true,
		ProRataGroup:          req.ProRataGroup,
		ProRataIndex:          req.ProRataIndex,
		ProRataTotal:          req.ProRataTotal,
		CreatedAt:             now,
		TransactionAmount:     snapshot.TransactionAmount,
		ReportingAmount:       snapshot.ReportingAmount,
		ReportingCurrency:     snapshot.ReportingCurrency,
		ExchangeRate:          snapshot.ExchangeRate,
		ExchangeRateSource:    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp: snapshot.ExchangeRateTimestamp,
		ExchangeRateExpiresAt: snapshot.ExchangeRateExpiresAt,
	}

	created, err := s.repo.CreateExpense(ctx, expense)
	if err != nil {
		return nil, fmt.Errorf("creating pro-rata installment: %w", err)
	}

	s.logger.Info("pro-rata installment created",
		slog.String("method", "CreateProRataInstallment"),
		slog.String("user_id", req.UserID),
		slog.String("expense_id", created.ID),
		slog.String("pro_rata_group", req.ProRataGroup),
		slog.Int("pro_rata_index", int(req.ProRataIndex)),
	)

	return created, nil
}

func validateProRataInstallmentRequest(req *CreateProRataInstallmentRequest) *apierr.Error {
	fields := make(map[string]string)

	if req.UserID == "" {
		fields["userId"] = "user_id is required"
	}
	if req.Name == "" {
		fields["name"] = "name is required"
	}
	if req.Amount <= 0 {
		fields["amount"] = "amount must be positive"
	}
	if !model.ValidExpenseTypes[req.ExpenseType] {
		fields["expenseType"] = "expense_type must be one of: essentials, desires, savings"
	}
	if req.TagID == "" {
		fields["tagId"] = "tag_id is required"
	}
	if req.ExpenseDate == "" {
		fields["expenseDate"] = "expense_date is required"
	} else if !isoDateRegex.MatchString(req.ExpenseDate) {
		fields["expenseDate"] = "expense_date must be in ISO format (YYYY-MM-DD)"
	}
	if req.ProRataGroup == "" {
		fields["proRataGroup"] = "pro_rata_group is required"
	}
	if req.ProRataIndex < 1 {
		fields["proRataIndex"] = "pro_rata_index must be positive"
	}
	if req.ProRataTotal < req.ProRataIndex {
		fields["proRataTotal"] = "pro_rata_total must be >= pro_rata_index"
	}

	if len(fields) > 0 {
		return apierr.Validation("validation failed", fields)
	}
	return nil
}

// validateTrustedPeriodContext enforces the trusted-context invariants: the
// context must originate from Finance, belong to the requesting user, and carry
// a valid year/month. A mismatch is a caller error (no ledger write), not a
// Finance round trip.
func validateTrustedPeriodContext(userID string, pc TrustedPeriodContext) *apierr.Error {
	if pc.Source != "finance_service" {
		return apierr.Validation("trusted period context must originate from the finance service", map[string]string{
			"source": "must be finance_service",
		})
	}
	if pc.UserID != userID {
		return apierr.Validation("trusted period context user does not match the request user", map[string]string{
			"userId": "mismatch",
		})
	}
	if pc.PeriodID == "" {
		return apierr.Validation("trusted period context period id is required", map[string]string{
			"periodId": "required",
		})
	}
	if pc.Year < 1 {
		return apierr.Validation("trusted period context year must be positive", map[string]string{
			"year": "must be positive",
		})
	}
	if pc.Month < 1 || pc.Month > 12 {
		return apierr.Validation("trusted period context month must be between 1 and 12", map[string]string{
			"month": "must be between 1 and 12",
		})
	}
	return nil
}

func validateSnapshotCoverage(snapshot *CapturedRateSnapshot, currencies ...string) *apierr.Error {
	if snapshot == nil || len(snapshot.RatesByCurrency) == 0 {
		return snapshotCurrencyMissingError()
	}
	for _, currency := range currencies {
		if _, ok := snapshot.RatesByCurrency[currency]; !ok {
			return snapshotCurrencyMissingError()
		}
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

func toFxCapturedRateSnapshot(s *CapturedRateSnapshot) *FxCapturedRateSnapshot {
	if s == nil {
		return nil
	}
	return &FxCapturedRateSnapshot{
		SnapshotVersion: s.SnapshotVersion,
		Source:          s.Source,
		BaseCurrency:    s.BaseCurrency,
		RateTimestamp:   s.RateTimestamp,
		CapturedAt:      s.CapturedAt,
		ExpiresAt:       s.ExpiresAt,
		RatesByCurrency: s.RatesByCurrency,
	}
}
