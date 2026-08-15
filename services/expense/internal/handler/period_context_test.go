package handler

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/ItsThompson/gofin/services/expense/internal/service"
)

type mockPeriodContextClient struct {
	mock.Mock
}

func (m *mockPeriodContextClient) GetPeriodContext(ctx context.Context, userID string, year, month int32) (*service.PeriodContext, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.PeriodContext), args.Error(1)
}

func newTestPeriodClient() *mockPeriodContextClient {
	client := new(mockPeriodContextClient)
	client.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)
	return client
}
