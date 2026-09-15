package handler

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/shared/reporting"
)

func (h *GRPCHandler) GetActivityMetrics(ctx context.Context, req *pb.GetActivityMetricsRequest) (*pb.ActivityMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	windowSet := req.GetWindowSet()
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if h == nil || h.activityService == nil {
		return nil, status.Error(codes.Unimplemented, "activity metrics are not configured")
	}

	metrics, err := h.activityService.GetMetrics(ctx, windowSet)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "activity metrics request canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "activity metrics request deadline exceeded")
		}
		var validationErr *reporting.WindowValidationError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, validationErr.Error())
		}
		return nil, h.mapServiceError(ctx, err, opActivity, "")
	}
	if metrics == nil {
		return nil, h.mapServiceError(ctx, fmt.Errorf("activity metrics service returned nil result"), opActivity, "")
	}

	return activityMetricsToProto(metrics), nil
}

func activityMetricsToProto(metrics *service.ActivityMetrics) *pb.ActivityMetricsResponse {
	return &pb.ActivityMetricsResponse{
		WindowSet:          metrics.WindowSet,
		TotalExpenses:      metrics.TotalExpenses,
		ManualExpenses:     metrics.ManualExpenses,
		CorrectionExpenses: metrics.CorrectionExpenses,
		ProrataExpenses:    metrics.ProrataExpenses,
		ActiveExpenses:     metrics.ActiveExpenses,
	}
}
