package service

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	financepb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// PeriodContext is the read-only Finance state required before an expense write.
type PeriodContext struct {
	PeriodID          string
	UserID            string
	Year              int32
	Month             int32
	ReportingCurrencyCode string
	IsLocked          bool
}

// PeriodContextClient resolves budget period context from Finance.
type PeriodContextClient interface {
	GetPeriodContext(ctx context.Context, userID string, year, month int32) (*PeriodContext, error)
}

type GRPCPeriodContextClient struct {
	client financepb.FinanceServiceClient
}

func NewGRPCPeriodContextClient(client financepb.FinanceServiceClient) *GRPCPeriodContextClient {
	return &GRPCPeriodContextClient{client: client}
}

func (c *GRPCPeriodContextClient) GetPeriodContext(ctx context.Context, userID string, year, month int32) (*PeriodContext, error) {
	resp, err := c.client.GetPeriodContext(ctx, &financepb.GetPeriodContextRequest{
		UserId: userID,
		Year:   year,
		Month:  month,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, periodNotFoundError(year, month)
		}
		return nil, fmt.Errorf("gRPC GetPeriodContext: %w", err)
	}

	return &PeriodContext{
		PeriodID:          resp.GetPeriodId(),
		UserID:            resp.GetUserId(),
		Year:              resp.GetYear(),
		Month:             resp.GetMonth(),
		ReportingCurrencyCode: resp.GetReportingCurrencyCode(),
		IsLocked:          resp.GetIsLocked(),
	}, nil
}

func periodNotFoundError(year, month int32) *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrPeriodNotFound,
		Message: fmt.Sprintf("No budget period found for %d-%02d", year, month),
		Status:  http.StatusNotFound,
		Fields: map[string]string{
			"periodYear":  fmt.Sprintf("%d", year),
			"periodMonth": fmt.Sprintf("%d", month),
		},
	}
}
