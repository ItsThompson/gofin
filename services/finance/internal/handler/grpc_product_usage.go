package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/finance/internal/config"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/shared/reporting"
)

func (h *GRPCHandler) GetProductUsageMetrics(ctx context.Context, req *pb.GetProductUsageMetricsRequest) (*pb.ProductUsageMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	windowSet := req.GetWindowSet()
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if h == nil || h.productUsageMetricsService == nil {
		return nil, status.Error(codes.Unimplemented, "product usage metrics are not configured")
	}

	metrics, err := h.productUsageMetricsService.GetProductUsageMetrics(ctx, windowSet)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "product usage metrics request canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "product usage metrics request deadline exceeded")
		}
		var validationErr *reporting.WindowValidationError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, validationErr.Error())
		}

		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.get_product_usage_metrics",
			Domain: config.ReportDomain,
			Msg:    "failed to get product usage metrics",
		})
		return nil, status.Error(codes.Internal, "failed to get product usage metrics")
	}
	if metrics == nil {
		return nil, status.Error(codes.Internal, "failed to get product usage metrics")
	}

	return &pb.ProductUsageMetricsResponse{
		WindowSet:        windowSet,
		BudgetPeriods:    metrics.BudgetPeriods,
		Tags:             metrics.Tags,
		ProrataSchedules: metrics.ProrataSchedules,
	}, nil
}
