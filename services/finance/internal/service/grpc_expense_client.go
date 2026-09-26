package service

import (
	"context"
	"fmt"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GRPCExpenseClient implements ExpenseClient by calling the expense service gRPC API.
type GRPCExpenseClient struct {
	client expensepb.ExpenseServiceClient
}

// NewGRPCExpenseClient wraps a gRPC expense service client.
func NewGRPCExpenseClient(client expensepb.ExpenseServiceClient) *GRPCExpenseClient {
	return &GRPCExpenseClient{client: client}
}

const completePeriodPageSize int32 = 50

// ExpenseRevision identifies the expense process and user ledger generation
// used to validate finance-owned cached results.
type ExpenseRevision struct {
	Epoch    string
	Revision uint64
}

func (c *GRPCExpenseClient) GetActiveExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error) {
	cursor := &expensepb.GetActiveExpensesForPeriodPageRequest{
		UserId:   userID,
		Year:     year,
		Month:    month,
		PageSize: completePeriodPageSize,
	}
	expenses := make([]ExpenseData, 0, completePeriodPageSize)

	for {
		resp, err := c.client.GetActiveExpensesForPeriodPage(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: empty response")
		}

		for _, exp := range resp.GetData() {
			expenses = append(expenses, mapExpenseData(exp))
		}
		if !resp.GetHasMore() {
			return expenses, nil
		}
		if len(resp.GetData()) == 0 {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: empty page marked as incomplete")
		}

		nextCursor := &expensepb.GetActiveExpensesForPeriodPageRequest{
			UserId:            userID,
			Year:              year,
			Month:             month,
			CursorExpenseDate: resp.GetNextExpenseDate(),
			CursorCreatedAt:   resp.GetNextCreatedAt(),
			CursorId:          resp.GetNextId(),
			PageSize:          completePeriodPageSize,
		}
		lastExpense := resp.GetData()[len(resp.GetData())-1]
		if nextCursor.GetCursorExpenseDate() == "" || nextCursor.GetCursorCreatedAt() == "" || nextCursor.GetCursorId() == "" {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: incomplete next cursor")
		}
		if nextCursor.GetCursorExpenseDate() != lastExpense.GetExpenseDateIso() ||
			nextCursor.GetCursorCreatedAt() != lastExpense.GetCreatedAt() ||
			nextCursor.GetCursorId() != lastExpense.GetId() {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: next cursor does not match last row")
		}
		if !isCursorStrictlyAfter(cursor, nextCursor) {
			return nil, fmt.Errorf("gRPC GetActiveExpensesForPeriodPage: non-advancing or regressing next cursor")
		}
		cursor = nextCursor
	}
}

func isCursorStrictlyAfter(previous, next *expensepb.GetActiveExpensesForPeriodPageRequest) bool {
	if previous.GetCursorExpenseDate() == "" && previous.GetCursorCreatedAt() == "" && previous.GetCursorId() == "" {
		return true
	}
	if next.GetCursorExpenseDate() != previous.GetCursorExpenseDate() {
		return next.GetCursorExpenseDate() > previous.GetCursorExpenseDate()
	}
	if next.GetCursorCreatedAt() != previous.GetCursorCreatedAt() {
		return next.GetCursorCreatedAt() > previous.GetCursorCreatedAt()
	}
	return next.GetCursorId() > previous.GetCursorId()
}

func mapExpenseData(exp *expensepb.ExpenseData) ExpenseData {
	return ExpenseData{
		ID:                    exp.GetId(),
		ReportingAmount:       exp.GetReportingAmountInMinorUnits(),
		ReportingCurrencyCode: exp.GetReportingCurrencyCode(),
		ExpenseType:           exp.GetExpenseType(),
		TagID:                 exp.GetTagId(),
		ExpenseDate:           exp.GetExpenseDateIso(),
	}
}

// GetExpenseRevision reads process-local expense freshness state without an
// immudb query.
func (c *GRPCExpenseClient) GetExpenseRevision(ctx context.Context, userID string) (ExpenseRevision, error) {
	resp, err := c.client.GetExpenseRevision(ctx, &expensepb.GetExpenseRevisionRequest{UserId: userID})
	if err != nil {
		return ExpenseRevision{}, fmt.Errorf("gRPC GetExpenseRevision: %w", err)
	}
	return ExpenseRevision{Epoch: resp.GetEpoch(), Revision: resp.GetRevision()}, nil
}

func (c *GRPCExpenseClient) CountExpensesByTag(ctx context.Context, userID, tagID string) (int64, error) {
	resp, err := c.client.CountExpensesByTag(ctx, &expensepb.CountExpensesByTagRequest{
		UserId: userID,
		TagId:  tagID,
	})
	if err != nil {
		return 0, fmt.Errorf("gRPC CountExpensesByTag: %w", err)
	}
	return resp.GetCount(), nil
}

func (c *GRPCExpenseClient) CreateExpense(ctx context.Context, req CreateExpenseInput) (*CreatedExpenseData, error) {
	resp, err := c.client.CreateExpense(ctx, &expensepb.CreateExpenseRequest{
		UserId:                                req.UserID,
		Name:                                  req.Name,
		AmountInTransactionCurrencyMinorUnits: req.Amount,
		TransactionCurrencyCode:               req.TransactionCurrency,
		ExpenseType:                           req.ExpenseType,
		TagId:                                 req.TagID,
		ExpenseDateIso:                        req.ExpenseDate,
		PeriodYear:                            req.PeriodYear,
		PeriodMonth:                           req.PeriodMonth,
		IsProRata:                             req.IsProRata,
		ProRataGroup:                          req.ProRataGroup,
		ProRataIndex:                          req.ProRataIndex,
		ProRataTotal:                          req.ProRataTotal,
		ClientGeneratedIdempotencyKey:         req.ClientGeneratedIdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC CreateExpense: %w", err)
	}

	return &CreatedExpenseData{
		ID:        resp.GetExpense().GetId(),
		CreatedAt: resp.GetExpense().GetCreatedAt(),
	}, nil
}

// CreateProRataInstallment calls the Expense internal pro-rata write RPC with
// trusted period context and the captured snapshot. Expense does not re-fetch
// Finance context for this path.
func (c *GRPCExpenseClient) CreateProRataInstallment(ctx context.Context, req CreateProRataInstallmentInput) (*CreatedExpenseData, error) {
	resp, err := c.client.CreateProRataInstallment(ctx, &expensepb.CreateProRataInstallmentRequest{
		UserId: req.UserID,
		PeriodContext: &expensepb.TrustedPeriodContext{
			PeriodId:              req.PeriodContext.PeriodID,
			UserId:                req.PeriodContext.UserID,
			Year:                  req.PeriodContext.Year,
			Month:                 req.PeriodContext.Month,
			ReportingCurrencyCode: req.PeriodContext.ReportingCurrencyCode,
			Source:                req.PeriodContext.Source,
		},
		Name:                                  req.Name,
		AmountInTransactionCurrencyMinorUnits: req.Amount,
		TransactionCurrencyCode:               req.Currency,
		ExpenseType:                           req.ExpenseType,
		TagId:                                 req.TagID,
		ExpenseDateIso:                        req.ExpenseDate,
		ProRataGroup:                          req.ProRataGroup,
		ProRataIndex:                          req.ProRataIndex,
		ProRataTotal:                          req.ProRataTotal,
		CapturedRateSnapshot:                  snapshotToProto(req.CapturedRateSnapshot),
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC CreateProRataInstallment: %w", err)
	}

	return &CreatedExpenseData{
		ID:        resp.GetExpense().GetId(),
		CreatedAt: resp.GetExpense().GetCreatedAt(),
	}, nil
}

func snapshotToProto(s *model.CapturedRateSnapshot) *expensepb.CapturedRateSnapshot {
	if s == nil {
		return nil
	}
	return &expensepb.CapturedRateSnapshot{
		SnapshotVersion: s.SnapshotVersion,
		Source:          s.Source,
		BaseCurrency:    s.BaseCurrency,
		RateTimestamp:   s.RateTimestamp,
		CapturedAt:      s.CapturedAt,
		ExpiresAt:       s.ExpiresAt,
		RatesByCurrency: s.RatesByCurrency,
	}
}
