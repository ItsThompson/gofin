package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// reportDomain is the domain tag on every report this service makes. Budgets,
// periods, tags, and spending are all one business area, and the tag is a query
// dimension shared with the other services, so it comes from one place.
const reportDomain = "budgets"

// GRPCHandler implements the FinanceService gRPC server. RPCs without an
// explicit method are served by the embedded UnimplementedFinanceServiceServer,
// which returns codes.Unimplemented.
type GRPCHandler struct {
	pb.UnimplementedFinanceServiceServer
	financeService *service.FinanceService
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(financeService *service.FinanceService) *GRPCHandler {
	return &GRPCHandler{
		financeService: financeService,
	}
}

// reportServerFailure reports err unless the client is about to receive a client
// error. Every codes.Internal exit below is reachable with a typed *apierr.Error
// whose code the exit does not name, and a code this handler does not map is not
// evidence that the failure is the service's fault: a validation or not-found error
// arriving at one of them would otherwise bill error quota for ordinary client
// input, and nothing would fail.
//
// The gate is the rendered status rather than a list of codes, so it stays correct
// as the code set grows. A gated error leaves no record here, which is right: the
// guard that produced the typed 4xx recorded it where the decision was made, and
// what a client error at an internal exit really signals is a status pairing the
// caller can see.
func reportServerFailure(ctx context.Context, err error, meta errkit.Meta) {
	if apierr.IsServerError(err) {
		_ = errkit.Report(ctx, err, meta)
	}
}

func (h *GRPCHandler) GetDefaults(ctx context.Context, req *pb.GetDefaultsRequest) (*pb.DefaultsResponse, error) {
	defaults, err := h.financeService.GetDefaults(ctx, req.GetUserId())
	if err != nil {
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) && apiErr.Code == apierr.CodeNotFound {
			return nil, status.Error(codes.NotFound, apiErr.Message)
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.get_defaults",
			Domain: reportDomain,
			Msg:    "failed to get defaults",
			Data: map[string]any{
				"method":  "GetDefaults",
				"user_id": req.GetUserId(),
			},
		})
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

func periodToProto(period *model.BudgetPeriod) *pb.PeriodData {
	return &pb.PeriodData{
		Id:                period.ID,
		UserId:            period.UserID,
		Year:              period.Year,
		Month:             period.Month,
		BudgetAmount:      period.BudgetAmount,
		EssentialsPercent: period.EssentialsPercent,
		DesiresPercent:    period.DesiresPercent,
		SavingsPercent:    period.SavingsPercent,
		CreatedAt:         period.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ReportingCurrency: period.ReportingCurrency,
	}
}

func financeErrorStatus(err error) error {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		return nil
	}

	switch apiErr.Code {
	case apierr.CodeValidation, model.ErrUnsupportedCurrency:
		return status.Error(codes.InvalidArgument, apiErr.Message)
	case apierr.CodeNotFound, model.ErrPeriodNotFound:
		return status.Error(codes.NotFound, apiErr.Message)
	case model.ErrPeriodLocked:
		return status.Error(codes.PermissionDenied, apiErr.Message)
	}
	return nil
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
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) {
			return nil, status.Error(codes.InvalidArgument, apiErr.Message)
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.complete_onboarding",
			Domain: reportDomain,
			Msg:    "failed to complete onboarding",
			Data: map[string]any{
				"method":  "CompleteOnboarding",
				"user_id": req.GetUserId(),
			},
		})
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

func (h *GRPCHandler) GetCurrentPeriod(ctx context.Context, req *pb.GetCurrentPeriodRequest) (*pb.PeriodResponse, error) {
	period, err := h.financeService.GetCurrentPeriod(ctx, req.GetUserId(), req.GetYear(), req.GetMonth())
	if err != nil {
		if statusErr := financeErrorStatus(err); statusErr != nil {
			return nil, statusErr
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.get_current_period",
			Domain: reportDomain,
			Msg:    "failed to get current period",
			Data: map[string]any{
				"method":  "GetCurrentPeriod",
				"user_id": req.GetUserId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to get current period")
	}
	return &pb.PeriodResponse{Period: periodToProto(period)}, nil
}

func (h *GRPCHandler) CreatePeriod(ctx context.Context, req *pb.CreatePeriodRequest) (*pb.PeriodResponse, error) {
	result, err := h.financeService.CreatePeriodWithProRata(ctx, req.GetUserId(), &model.CreatePeriodRequest{
		Year:              req.GetYear(),
		Month:             req.GetMonth(),
		BudgetAmount:      req.GetBudgetAmount(),
		EssentialsPercent: req.GetEssentialsPercent(),
		DesiresPercent:    req.GetDesiresPercent(),
		SavingsPercent:    req.GetSavingsPercent(),
		ReportingCurrency: req.GetReportingCurrency(),
	})
	if err != nil {
		if statusErr := financeErrorStatus(err); statusErr != nil {
			return nil, statusErr
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.create_period",
			Domain: reportDomain,
			Msg:    "failed to create period",
			Data: map[string]any{
				"method":  "CreatePeriod",
				"user_id": req.GetUserId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to create period")
	}
	return &pb.PeriodResponse{Period: periodToProto(result.Period)}, nil
}

func (h *GRPCHandler) UpdatePeriod(ctx context.Context, req *pb.UpdatePeriodRequest) (*pb.PeriodResponse, error) {
	period, err := h.financeService.UpdatePeriod(ctx, req.GetUserId(), req.GetPeriodId(), &model.UpdatePeriodRequest{
		BudgetAmount:      req.GetBudgetAmount(),
		EssentialsPercent: req.GetEssentialsPercent(),
		DesiresPercent:    req.GetDesiresPercent(),
		SavingsPercent:    req.GetSavingsPercent(),
	})
	if err != nil {
		if statusErr := financeErrorStatus(err); statusErr != nil {
			return nil, statusErr
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.update_period",
			Domain: reportDomain,
			Msg:    "failed to update period",
			Data: map[string]any{
				"method":    "UpdatePeriod",
				"user_id":   req.GetUserId(),
				"period_id": req.GetPeriodId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to update period")
	}
	return &pb.PeriodResponse{Period: periodToProto(period)}, nil
}

func (h *GRPCHandler) ListPeriods(ctx context.Context, req *pb.ListPeriodsRequest) (*pb.PeriodListResponse, error) {
	periods, err := h.financeService.ListPeriods(ctx, req.GetUserId())
	if err != nil {
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.list_periods",
			Domain: reportDomain,
			Msg:    "failed to list periods",
			Data: map[string]any{
				"method":  "ListPeriods",
				"user_id": req.GetUserId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to list periods")
	}

	pbPeriods := make([]*pb.PeriodData, len(periods))
	for i, period := range periods {
		pbPeriods[i] = periodToProto(period)
	}
	return &pb.PeriodListResponse{Periods: pbPeriods, Total: int32(len(pbPeriods))}, nil
}

func (h *GRPCHandler) ListTags(ctx context.Context, req *pb.ListTagsRequest) (*pb.TagListResponse, error) {
	tags, err := h.financeService.ListTags(ctx, req.GetUserId())
	if err != nil {
		// Both returns below are codes.Internal, so one report covers both.
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.list_tags",
			Domain: reportDomain,
			Msg:    "failed to list tags",
			Data: map[string]any{
				"method":  "ListTags",
				"user_id": req.GetUserId(),
			},
		})
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) {
			return nil, status.Error(codes.Internal, apiErr.Message)
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
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case apierr.CodeValidation:
				return nil, status.Error(codes.InvalidArgument, apiErr.Message)
			case model.ErrDuplicateTag:
				return nil, status.Error(codes.AlreadyExists, apiErr.Message)
			}
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.create_tag",
			Domain: reportDomain,
			Msg:    "failed to create tag",
			Data: map[string]any{
				"method":  "CreateTag",
				"user_id": req.GetUserId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to create tag")
	}

	return &pb.TagResponse{Tag: &pb.TagData{Id: tag.ID, UserId: tag.UserID, Name: tag.Name, IsDefault: tag.IsDefault}}, nil
}

func (h *GRPCHandler) UpdateTag(ctx context.Context, req *pb.UpdateTagRequest) (*pb.TagResponse, error) {
	tag, err := h.financeService.UpdateTag(ctx, req.GetUserId(), req.GetTagId(), &model.UpdateTagRequest{Name: req.GetName()})
	if err != nil {
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case apierr.CodeValidation:
				return nil, status.Error(codes.InvalidArgument, apiErr.Message)
			case model.ErrDuplicateTag:
				return nil, status.Error(codes.AlreadyExists, apiErr.Message)
			case apierr.CodeNotFound:
				return nil, status.Error(codes.NotFound, apiErr.Message)
			}
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.update_tag",
			Domain: reportDomain,
			Msg:    "failed to update tag",
			Data: map[string]any{
				"method":  "UpdateTag",
				"user_id": req.GetUserId(),
				"tag_id":  req.GetTagId(),
			},
		})
		return nil, status.Error(codes.Internal, "failed to update tag")
	}

	return &pb.TagResponse{Tag: &pb.TagData{Id: tag.ID, UserId: tag.UserID, Name: tag.Name, IsDefault: tag.IsDefault}}, nil
}

func (h *GRPCHandler) DeleteTag(ctx context.Context, req *pb.DeleteTagRequest) (*pb.DeleteTagResponse, error) {
	err := h.financeService.DeleteTag(ctx, req.GetUserId(), req.GetTagId())
	if err != nil {
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case apierr.CodeNotFound:
				return nil, status.Error(codes.NotFound, apiErr.Message)
			case model.ErrDefaultTag:
				return nil, status.Error(codes.PermissionDenied, apiErr.Message)
			case model.ErrTagInUse:
				return nil, status.Error(codes.FailedPrecondition, apiErr.Message)
			}
		}
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.delete_tag",
			Domain: reportDomain,
			Msg:    "failed to delete tag",
			Data: map[string]any{
				"method":  "DeleteTag",
				"user_id": req.GetUserId(),
				"tag_id":  req.GetTagId(),
			},
		})
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
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.get_all_user_data",
			Domain: reportDomain,
			Msg:    "failed to get all user data",
			Data: map[string]any{
				"method":  "GetAllUserData",
				"user_id": userID,
			},
		})
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
		pbPeriods[i] = periodToProto(period)
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
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     "finance.delete_all_user_data",
			Domain: reportDomain,
			Msg:    "failed to delete all user data",
			Data: map[string]any{
				"method":  "DeleteAllUserData",
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to delete user data")
	}

	return &pb.DeleteAllUserDataResponse{}, nil
}
