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

func (c *GRPCExpenseClient) GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error) {
	resp, err := c.client.GetExpensesForPeriod(ctx, &expensepb.GetExpensesForPeriodRequest{
		UserId:   userID,
		Year:     year,
		Month:    month,
		Page:     1,
		PageSize: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC GetExpensesForPeriod: %w", err)
	}

	expenses := make([]ExpenseData, len(resp.GetData()))
	for i, exp := range resp.GetData() {
		expenses[i] = ExpenseData{
			ID:                exp.GetId(),
			ReportingAmount:   exp.GetReportingAmount(),
			ReportingCurrency: exp.GetReportingCurrency(),
			ExpenseType:       exp.GetExpenseType(),
			TagID:             exp.GetTagId(),
			ExpenseDate:       exp.GetExpenseDate(),
		}
	}

	return expenses, nil
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
		UserId:              req.UserID,
		Name:                req.Name,
		Amount:              req.Amount,
		TransactionCurrency: req.TransactionCurrency,
		ExpenseType:         req.ExpenseType,
		TagId:               req.TagID,
		ExpenseDate:         req.ExpenseDate,
		PeriodYear:          req.PeriodYear,
		PeriodMonth:         req.PeriodMonth,
		IsProRata:           req.IsProRata,
		ProRataGroup:        req.ProRataGroup,
		ProRataIndex:        req.ProRataIndex,
		ProRataTotal:        req.ProRataTotal,
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
			PeriodId:          req.PeriodContext.PeriodID,
			UserId:            req.PeriodContext.UserID,
			Year:              req.PeriodContext.Year,
			Month:             req.PeriodContext.Month,
			ReportingCurrency: req.PeriodContext.ReportingCurrency,
			Source:            req.PeriodContext.Source,
		},
		Name:                 req.Name,
		Amount:               req.Amount,
		TransactionCurrency:  req.Currency,
		ExpenseType:          req.ExpenseType,
		TagId:                req.TagID,
		ExpenseDate:          req.ExpenseDate,
		ProRataGroup:         req.ProRataGroup,
		ProRataIndex:         req.ProRataIndex,
		ProRataTotal:         req.ProRataTotal,
		CapturedRateSnapshot: snapshotToProto(req.CapturedRateSnapshot),
		LegacyMigration:      req.LegacyMigration,
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
