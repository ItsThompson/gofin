package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	pb "github.com/ItsThompson/gofin/services/datarights/proto/datarightspb"
	reporting "github.com/ItsThompson/gofin/services/shared/reporting"
)

// GRPCHandler exposes datarights' internal gRPC operations.
type GRPCHandler struct {
	pb.UnimplementedDatarightsServiceServer
	exportMetricsService *service.ExportMetricsService
}

// NewGRPCHandler creates a gRPC handler backed by the export metrics service.
func NewGRPCHandler(exportMetricsService *service.ExportMetricsService) *GRPCHandler {
	return &GRPCHandler{exportMetricsService: exportMetricsService}
}

// GetExportMetrics returns export counts for the validated reporting windows.
func (h *GRPCHandler) GetExportMetrics(ctx context.Context, req *pb.GetExportMetricsRequest) (*pb.ExportMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	windowSet := req.GetWindowSet()
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	completedExports, err := h.exportMetricsService.GetExportMetrics(ctx, windowSet)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}
		var validationErr *reporting.WindowValidationError
		if errors.As(err, &validationErr) {
			return nil, status.Error(codes.InvalidArgument, validationErr.Error())
		}
		return nil, status.Error(codes.Internal, "failed to get export metrics")
	}

	return &pb.ExportMetricsResponse{
		WindowSet:        windowSet,
		CompletedExports: completedExports,
	}, nil
}
