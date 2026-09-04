package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

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

	now := s.clock().UTC().Format(time.RFC3339)

	if err := validateSnapshotCoverage(req.CapturedRateSnapshot, transactionCurrency, reportingCurrency); err != nil {
		return nil, err
	}

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
	snapshot, invErr := buildProviderSnapshot(req.Amount, transactionCurrency, reportingCurrency, fxResp)
	if invErr != nil {
		return nil, invErr
	}

	expense := &model.Expense{
		ID:                                    uuid.New().String(),
		UserID:                                req.UserID,
		Name:                                  req.Name,
		TransactionCurrencyCode:               transactionCurrency,
		ExpenseType:                           req.ExpenseType,
		TagID:                                 req.TagID,
		ExpenseDateIso:                        req.ExpenseDate,
		PeriodYear:                            req.PeriodContext.Year,
		PeriodMonth:                           req.PeriodContext.Month,
		Status:                                "active",
		CorrectsID:                            "",
		IsProRata:                             true,
		ProRataGroup:                          req.ProRataGroup,
		ProRataIndex:                          req.ProRataIndex,
		ProRataTotal:                          req.ProRataTotal,
		CreatedAt:                             now,
		OriginalTransactionAmountInMinorUnits: snapshot.OriginalTransactionAmountInMinorUnits,
		ReportingAmountInMinorUnits:           snapshot.ReportingAmountInMinorUnits,
		ReportingCurrencyCode:                 snapshot.ReportingCurrencyCode,
		SourceToTargetExchangeRate:            snapshot.SourceToTargetExchangeRate,
		ExchangeRateSource:                    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp:                 snapshot.ExchangeRateTimestamp,
		ExchangeRateCacheExpiresAt:            snapshot.ExchangeRateCacheExpiresAt,
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
