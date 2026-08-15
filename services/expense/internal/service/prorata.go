package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/apierr"
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

	var snapshot model.Expense
	if req.LegacyMigration {
		// Legacy pending schedules have no captured snapshot. Finance validates
		// that the target period reporting currency equals the stored schedule
		// currency before requesting a migration write. Expense re-validates the
		// currency match here so a caller error never invents a conversion.
		if transactionCurrency != reportingCurrency {
			return nil, apierr.Validation(
				"legacy migration pro-rata requires the transaction currency to equal the period reporting currency",
				map[string]string{"transactionCurrency": "must match period reporting currency"},
			)
		}
		snapshot = buildMigrationSnapshot(req.Amount, transactionCurrency, reportingCurrency, now)
	} else {
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
		snapshot = buildProviderSnapshot(req.Amount, transactionCurrency, reportingCurrency, fxResp)
	}

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

// buildMigrationSnapshot builds the money snapshot for a legacy same-currency
// pro-rata application: transaction and reporting amounts are equal, the rate
// is "1", and the source is "migration" so the row is auditable as a migration
// conversion rather than an identity conversion or a provider conversion.
func buildMigrationSnapshot(amount int64, transactionCurrency, reportingCurrency, timestamp string) model.Expense {
	return model.Expense{
		MoneySnapshotVersion:  1,
		TransactionAmount:     amount,
		TransactionCurrency:   transactionCurrency,
		ReportingAmount:       amount,
		ReportingCurrency:     reportingCurrency,
		ExchangeRate:          "1",
		ExchangeRateSource:    model.ExchangeSourceMigration,
		ExchangeRateTimestamp: timestamp,
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
