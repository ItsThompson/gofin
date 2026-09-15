package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/auth/internal/config"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/shared/reporting"
)

type growthMetricsRPC interface {
	GetGrowthMetrics(context.Context, *pb.GetGrowthMetricsRequest) (*pb.GrowthMetricsResponse, error)
}

// GrowthGRPCAdapter validates the shared window contract and maps the service
// result to the auth gRPC response. Query failures are already represented as
// typed metric results by the service and do not become RPC errors.
type GrowthGRPCAdapter struct {
	growthService *service.GrowthService
}

// NewGrowthGRPCAdapter creates the gRPC adapter for the growth service.
func NewGrowthGRPCAdapter(growthService *service.GrowthService) *GrowthGRPCAdapter {
	return &GrowthGRPCAdapter{growthService: growthService}
}

func (a *GrowthGRPCAdapter) GetGrowthMetrics(ctx context.Context, req *pb.GetGrowthMetricsRequest) (*pb.GrowthMetricsResponse, error) {
	if req == nil || req.GetWindowSet() == nil {
		return nil, status.Error(codes.InvalidArgument, "window_set is required")
	}
	windowSet := req.GetWindowSet()
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if a == nil || a.growthService == nil {
		return nil, status.Error(codes.Unimplemented, "growth metrics are not configured")
	}

	result, err := a.growthService.GetGrowthMetrics(ctx, windowSet)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "growth metrics request canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "growth metrics request deadline exceeded")
		}
		var validationErr *reporting.WindowValidationError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, validationErr.Error())
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "auth.get_growth_metrics",
			Domain: config.ReportDomain,
			Msg:    "failed to get growth metrics",
			Data: map[string]any{
				"method": "GetGrowthMetrics",
			},
		})
		return nil, status.Error(codes.Internal, "failed to get growth metrics")
	}

	return &pb.GrowthMetricsResponse{
		WindowSet:             windowSet,
		NewUsers:              result.NewUsers,
		OnboardingCompletions: result.OnboardingCompletions,
	}, nil
}

var _ growthMetricsRPC = (*GrowthGRPCAdapter)(nil)
