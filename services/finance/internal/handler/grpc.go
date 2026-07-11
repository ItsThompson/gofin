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

// GRPCHandler implements the FinanceService gRPC server. RPCs without an
// explicit method are served by the embedded UnimplementedFinanceServiceServer,
// which returns codes.Unimplemented.
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

func (h *GRPCHandler) GetAllUserData(ctx context.Context, req *pb.GetAllUserDataRequest) (*pb.AllUserDataResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	data, err := h.financeService.GetAllUserData(ctx, userID)
	if err != nil {
		h.logger.Error("GetAllUserData failed",
			"user_id", userID,
			"error", err.Error(),
		)
		return nil, status.Error(codes.Internal, "failed to get all user data")
	}

	pbTags := make([]*pb.TagData, len(data.Tags))
	for i, tag := range data.Tags {
		pbTags[i] = &pb.TagData{
			Id:        tag.ID,
			UserId:    tag.UserID,
			Name:      tag.Name,
			IsDefault: tag.IsDefault,
			CreatedAt: tag.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	pbPeriods := make([]*pb.PeriodData, len(data.Periods))
	for i, period := range data.Periods {
		pbPeriods[i] = &pb.PeriodData{
			Id:                period.ID,
			UserId:            period.UserID,
			Year:              period.Year,
			Month:             period.Month,
			BudgetAmount:      period.BudgetAmount,
			EssentialsPercent: period.EssentialsPercent,
			DesiresPercent:    period.DesiresPercent,
			SavingsPercent:    period.SavingsPercent,
			CreatedAt:         period.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	var pbDefaults *pb.DefaultsData
	if data.Defaults != nil {
		pbDefaults = &pb.DefaultsData{
			UserId:            data.Defaults.UserID,
			BudgetAmount:      data.Defaults.BudgetAmount,
			EssentialsPercent: data.Defaults.EssentialsPercent,
			DesiresPercent:    data.Defaults.DesiresPercent,
			SavingsPercent:    data.Defaults.SavingsPercent,
			Currency:          data.Defaults.Currency,
		}
	}

	return &pb.AllUserDataResponse{
		Tags:     pbTags,
		Periods:  pbPeriods,
		Defaults: pbDefaults,
	}, nil
}

func (h *GRPCHandler) DeleteAllUserData(ctx context.Context, req *pb.DeleteAllUserDataRequest) (*pb.DeleteAllUserDataResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.financeService.DeleteAllUserData(ctx, userID); err != nil {
		h.logger.Error("DeleteAllUserData failed",
			"user_id", userID,
			"error", err.Error(),
		)
		return nil, status.Error(codes.Internal, "failed to delete user data")
	}

	return &pb.DeleteAllUserDataResponse{}, nil
}
