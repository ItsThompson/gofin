package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// GRPCHandler implements the ExpenseService gRPC server. Each RPC delegates to
// the shared ExpenseService and maps service errors to gRPC status codes.
type GRPCHandler struct {
	pb.UnimplementedExpenseServiceServer
	expenseService *service.ExpenseService
	logger         *slog.Logger
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(expenseService *service.ExpenseService, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		expenseService: expenseService,
		logger:         logger,
	}
}

func (h *GRPCHandler) CreateExpense(ctx context.Context, req *pb.CreateExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.CreateExpense(ctx, req.GetUserId(), &model.CreateExpenseRequest{
		Name:         req.GetName(),
		Amount:       req.GetAmount(),
		Currency:     req.GetCurrency(),
		ExpenseType:  req.GetExpenseType(),
		TagID:        req.GetTagId(),
		ExpenseDate:  req.GetExpenseDate(),
		PeriodYear:   req.GetPeriodYear(),
		PeriodMonth:  req.GetPeriodMonth(),
		IsProRata:    req.GetIsProRata(),
		ProRataGroup: req.GetProRataGroup(),
		ProRataIndex: req.GetProRataIndex(),
		ProRataTotal: req.GetProRataTotal(),
	})
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) GetExpensesForPeriod(ctx context.Context, req *pb.GetExpensesForPeriodRequest) (*pb.ExpenseListResponse, error) {
	result, err := h.expenseService.GetExpensesForPeriod(ctx, &model.GetExpensesRequest{
		UserID:   req.GetUserId(),
		Year:     req.GetYear(),
		Month:    req.GetMonth(),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoExpenses := make([]*pb.ExpenseData, len(result.Data))
	for i, expense := range result.Data {
		protoExpenses[i] = expenseToProto(expense)
	}

	return &pb.ExpenseListResponse{
		Data:     protoExpenses,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		HasMore:  result.HasMore,
	}, nil
}

func (h *GRPCHandler) GetExpense(ctx context.Context, req *pb.GetExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.GetExpense(ctx, req.GetUserId(), req.GetId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) CorrectExpense(ctx context.Context, req *pb.CorrectExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.CorrectExpense(ctx, req.GetUserId(), req.GetExpenseId(), &model.CorrectExpenseRequest{
		Name:        req.GetName(),
		Amount:      req.GetAmount(),
		ExpenseType: req.GetExpenseType(),
		TagID:       req.GetTagId(),
		ExpenseDate: req.GetExpenseDate(),
	})
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) CountExpensesByTag(ctx context.Context, req *pb.CountExpensesByTagRequest) (*pb.CountExpensesByTagResponse, error) {
	count, err := h.expenseService.CountExpensesByTag(ctx, req.GetUserId(), req.GetTagId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.CountExpensesByTagResponse{
		Count: count,
	}, nil
}

func (h *GRPCHandler) StreamAllUserExpenses(req *pb.StreamAllUserExpensesRequest, stream pb.ExpenseService_StreamAllUserExpensesServer) error {
	err := h.expenseService.StreamAllUserExpenses(stream.Context(), req.GetUserId(), req.GetPageSize(), func(expense *model.Expense) error {
		return stream.Send(expenseToProto(expense))
	})
	if err == nil {
		return nil
	}
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		return mapServiceError(err)
	}
	// Normalize context cancellation / deadline so gRPC reports codes.Canceled /
	// codes.DeadlineExceeded rather than codes.Unknown.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	// Stream-send failures are already gRPC-meaningful; surface them directly.
	return err
}

func (h *GRPCHandler) AnonymizeAllUserExpenses(ctx context.Context, req *pb.AnonymizeRequest) (*pb.AnonymizeResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.expenseService.AnonymizeAllUserExpenses(ctx, userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to anonymize expenses")
	}

	return &pb.AnonymizeResponse{}, nil
}

// expenseToProto converts a domain Expense to a protobuf ExpenseData.
func expenseToProto(e *model.Expense) *pb.ExpenseData {
	return &pb.ExpenseData{
		Id:           e.ID,
		UserId:       e.UserID,
		Name:         e.Name,
		Amount:       e.Amount,
		Currency:     e.Currency,
		ExpenseType:  e.ExpenseType,
		TagId:        e.TagID,
		ExpenseDate:  e.ExpenseDate,
		PeriodYear:   e.PeriodYear,
		PeriodMonth:  e.PeriodMonth,
		Status:       e.Status,
		CorrectsId:   e.CorrectsID,
		IsProRata:    e.IsProRata,
		ProRataGroup: e.ProRataGroup,
		ProRataIndex: e.ProRataIndex,
		ProRataTotal: e.ProRataTotal,
		CreatedAt:    e.CreatedAt,
	}
}

// mapServiceError converts a service-layer error to a gRPC status error.
func mapServiceError(err error) error {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case model.ErrValidationError:
			return status.Error(codes.InvalidArgument, svcErr.Message)
		case model.ErrNotFound:
			return status.Error(codes.NotFound, svcErr.Message)
		case model.ErrAlreadyCorrected:
			return status.Error(codes.FailedPrecondition, svcErr.Message)
		case model.ErrPeriodLocked:
			return status.Error(codes.PermissionDenied, svcErr.Message)
		default:
			return status.Error(codes.Internal, svcErr.Message)
		}
	}
	return status.Error(codes.Internal, "internal error")
}
