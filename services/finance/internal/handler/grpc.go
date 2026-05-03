package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// GRPCHandler implements the FinanceService gRPC server.
// CompleteOnboarding and GetDefaults are implemented.
// All other RPCs return Unimplemented (stubs for later tickets).
type GRPCHandler struct {
	pb.UnimplementedFinanceServiceServer
	financeService *service.FinanceService
	logger         *slog.Logger
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(financeService *service.FinanceService, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		financeService: financeService,
		logger:         logger,
	}
}

func (h *GRPCHandler) GetDefaults(ctx context.Context, req *pb.GetDefaultsRequest) (*pb.DefaultsResponse, error) {
	defaults, err := h.financeService.GetDefaults(ctx, req.GetUserId())
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok && svcErr.Code == model.ErrNotFound {
			return nil, status.Error(codes.NotFound, svcErr.Message)
		}
		return nil, status.Error(codes.Internal, "failed to get defaults")
	}

	return &pb.DefaultsResponse{
		Defaults: &pb.DefaultsData{
			UserId:            defaults.UserID,
			BudgetAmount:      defaults.BudgetAmount,
			EssentialsPercent: defaults.EssentialsPercent,
			DesiresPercent:    defaults.DesiresPercent,
			SavingsPercent:    defaults.SavingsPercent,
			Currency:          defaults.Currency,
		},
	}, nil
}

func (h *GRPCHandler) CompleteOnboarding(ctx context.Context, req *pb.CompleteOnboardingRequest) (*pb.DefaultsResponse, error) {
	defaults, err := h.financeService.CompleteOnboarding(ctx, req.GetUserId(), &model.OnboardingRequest{
		BudgetAmount:      req.GetBudgetAmount(),
		EssentialsPercent: req.GetEssentialsPercent(),
		DesiresPercent:    req.GetDesiresPercent(),
		SavingsPercent:    req.GetSavingsPercent(),
		Currency:          req.GetCurrency(),
	})
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			return nil, status.Error(codes.InvalidArgument, svcErr.Message)
		}
		return nil, status.Error(codes.Internal, "failed to complete onboarding")
	}

	return &pb.DefaultsResponse{
		Defaults: &pb.DefaultsData{
			UserId:            defaults.UserID,
			BudgetAmount:      defaults.BudgetAmount,
			EssentialsPercent: defaults.EssentialsPercent,
			DesiresPercent:    defaults.DesiresPercent,
			SavingsPercent:    defaults.SavingsPercent,
			Currency:          defaults.Currency,
		},
	}, nil
}

// Stub RPCs: return Unimplemented for later tickets.
func (h *GRPCHandler) UpdateDefaults(ctx context.Context, req *pb.UpdateDefaultsRequest) (*pb.DefaultsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdateDefaults not yet implemented")
}

func (h *GRPCHandler) GetCurrentPeriod(ctx context.Context, req *pb.GetCurrentPeriodRequest) (*pb.PeriodResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetCurrentPeriod not yet implemented")
}

func (h *GRPCHandler) CreatePeriod(ctx context.Context, req *pb.CreatePeriodRequest) (*pb.PeriodResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreatePeriod not yet implemented")
}

func (h *GRPCHandler) UpdatePeriod(ctx context.Context, req *pb.UpdatePeriodRequest) (*pb.PeriodResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdatePeriod not yet implemented")
}

func (h *GRPCHandler) ListPeriods(ctx context.Context, req *pb.ListPeriodsRequest) (*pb.PeriodListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListPeriods not yet implemented")
}

func (h *GRPCHandler) ListTags(ctx context.Context, req *pb.ListTagsRequest) (*pb.TagListResponse, error) {
	tags, err := h.financeService.ListTags(ctx, req.GetUserId())
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			return nil, status.Error(codes.Internal, svcErr.Message)
		}
		return nil, status.Error(codes.Internal, "failed to list tags")
	}

	pbTags := make([]*pb.TagData, len(tags))
	for i, tag := range tags {
		pbTags[i] = &pb.TagData{
			Id:        tag.ID,
			UserId:    tag.UserID,
			Name:      tag.Name,
			IsDefault: tag.IsDefault,
		}
	}

	return &pb.TagListResponse{Tags: pbTags}, nil
}

func (h *GRPCHandler) CreateTag(ctx context.Context, req *pb.CreateTagRequest) (*pb.TagResponse, error) {
	tag, err := h.financeService.CreateTag(ctx, req.GetUserId(), &model.CreateTagRequest{Name: req.GetName()})
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			switch svcErr.Code {
			case model.ErrValidationError:
				return nil, status.Error(codes.InvalidArgument, svcErr.Message)
			case model.ErrDuplicateTag:
				return nil, status.Error(codes.AlreadyExists, svcErr.Message)
			}
		}
		return nil, status.Error(codes.Internal, "failed to create tag")
	}

	return &pb.TagResponse{Tag: &pb.TagData{Id: tag.ID, UserId: tag.UserID, Name: tag.Name, IsDefault: tag.IsDefault}}, nil
}

func (h *GRPCHandler) UpdateTag(ctx context.Context, req *pb.UpdateTagRequest) (*pb.TagResponse, error) {
	tag, err := h.financeService.UpdateTag(ctx, req.GetUserId(), req.GetTagId(), &model.UpdateTagRequest{Name: req.GetName()})
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			switch svcErr.Code {
			case model.ErrValidationError:
				return nil, status.Error(codes.InvalidArgument, svcErr.Message)
			case model.ErrDuplicateTag:
				return nil, status.Error(codes.AlreadyExists, svcErr.Message)
			case model.ErrNotFound:
				return nil, status.Error(codes.NotFound, svcErr.Message)
			}
		}
		return nil, status.Error(codes.Internal, "failed to update tag")
	}

	return &pb.TagResponse{Tag: &pb.TagData{Id: tag.ID, UserId: tag.UserID, Name: tag.Name, IsDefault: tag.IsDefault}}, nil
}

func (h *GRPCHandler) DeleteTag(ctx context.Context, req *pb.DeleteTagRequest) (*pb.DeleteTagResponse, error) {
	err := h.financeService.DeleteTag(ctx, req.GetUserId(), req.GetTagId())
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			switch svcErr.Code {
			case model.ErrNotFound:
				return nil, status.Error(codes.NotFound, svcErr.Message)
			case model.ErrDefaultTag:
				return nil, status.Error(codes.PermissionDenied, svcErr.Message)
			case model.ErrTagInUse:
				return nil, status.Error(codes.FailedPrecondition, svcErr.Message)
			}
		}
		return nil, status.Error(codes.Internal, "failed to delete tag")
	}

	return &pb.DeleteTagResponse{}, nil
}

func (h *GRPCHandler) CheckTagUsage(ctx context.Context, req *pb.CheckTagUsageRequest) (*pb.TagUsageResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckTagUsage not yet implemented")
}

func (h *GRPCHandler) CreateProRataExpense(ctx context.Context, req *pb.CreateProRataExpenseRequest) (*pb.ProRataResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateProRataExpense not yet implemented")
}

func (h *GRPCHandler) GetUpcomingProRata(ctx context.Context, req *pb.GetUpcomingProRataRequest) (*pb.UpcomingProRataListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetUpcomingProRata not yet implemented")
}

func (h *GRPCHandler) GetPeriodSummary(ctx context.Context, req *pb.GetPeriodSummaryRequest) (*pb.PeriodSummaryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetPeriodSummary not yet implemented")
}

func (h *GRPCHandler) GetSpendingByTag(ctx context.Context, req *pb.GetSpendingByTagRequest) (*pb.TagSpendingListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetSpendingByTag not yet implemented")
}

func (h *GRPCHandler) GetCumulativeSpend(ctx context.Context, req *pb.GetCumulativeSpendRequest) (*pb.CumulativeSpendResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetCumulativeSpend not yet implemented")
}

func (h *GRPCHandler) GetHistoricalComparison(ctx context.Context, req *pb.GetHistoricalComparisonRequest) (*pb.HistoricalComparisonResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetHistoricalComparison not yet implemented")
}
