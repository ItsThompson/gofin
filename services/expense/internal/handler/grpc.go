package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// GRPCHandler implements the ExpenseService gRPC server.
// CreateExpense, GetExpensesForPeriod, and GetExpense are implemented.
// All other RPCs return Unimplemented (stubs for later tickets).
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
	expense, err := h.expenseService.GetExpense(ctx, req.GetId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

// Stub RPCs: return Unimplemented for later tickets.

func (h *GRPCHandler) CorrectExpense(ctx context.Context, req *pb.CorrectExpenseRequest) (*pb.ExpenseResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CorrectExpense not yet implemented")
}

func (h *GRPCHandler) GetCorrectionHistory(ctx context.Context, req *pb.GetCorrectionHistoryRequest) (*pb.CorrectionHistoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetCorrectionHistory not yet implemented")
}

func (h *GRPCHandler) GetProRataGroup(ctx context.Context, req *pb.GetProRataGroupRequest) (*pb.ExpenseListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetProRataGroup not yet implemented")
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
	if svcErr, ok := err.(*service.ServiceError); ok {
		switch svcErr.Code {
		case model.ErrValidationError:
			return status.Error(codes.InvalidArgument, svcErr.Message)
		case model.ErrNotFound:
			return status.Error(codes.NotFound, svcErr.Message)
		default:
			return status.Error(codes.Internal, svcErr.Message)
		}
	}
	return status.Error(codes.Internal, "internal error")
}
