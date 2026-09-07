package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// operation bundles a logical operation name with the gRPC method that surfaced
// it. opName is shared with the REST route serving the same operation, since a
// failure means the same thing over either transport; bundling the pair here
// prevents two adjacent strings being silently swapped at each call site.
type operation struct {
	opName    string
	rpcMethod string
}

var (
	opCreate             = operation{opName: "expense.create", rpcMethod: "CreateExpense"}
	opProRataInstallment = operation{opName: "expense.create_pro_rata_installment", rpcMethod: "CreateProRataInstallment"}
	opList               = operation{opName: "expense.list", rpcMethod: "GetActiveExpensesForPeriod"}
	opGet                = operation{opName: "expense.get", rpcMethod: "GetExpense"}
	opCorrect            = operation{opName: "expense.correct", rpcMethod: "CorrectExpense"}
	opCountByTag         = operation{opName: "expense.count_by_tag", rpcMethod: "CountExpensesByTag"}
	opStreamAll          = operation{opName: "expense.stream_all", rpcMethod: "StreamAllUserExpenses"}
	opAnonymize          = operation{opName: "expense.anonymize", rpcMethod: "AnonymizeAllUserExpenses"}
)

type GRPCHandler struct {
	pb.UnimplementedExpenseServiceServer
	expenseService *service.ExpenseService
}

func NewGRPCHandler(expenseService *service.ExpenseService) *GRPCHandler {
	return &GRPCHandler{
		expenseService: expenseService,
	}
}

func (h *GRPCHandler) CreateExpense(ctx context.Context, req *pb.CreateExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.CreateExpense(ctx, req.GetUserId(), &model.CreateExpenseRequest{
		Name:                                  req.GetName(),
		AmountInTransactionCurrencyMinorUnits: req.GetAmountInTransactionCurrencyMinorUnits(),
		TransactionCurrencyCode:               req.GetTransactionCurrencyCode(),
		ExpenseType:                           req.GetExpenseType(),
		TagID:                                 req.GetTagId(),
		ExpenseDateIso:                        req.GetExpenseDateIso(),
		PeriodYear:                            req.GetPeriodYear(),
		PeriodMonth:                           req.GetPeriodMonth(),
		IsProRata:                             req.GetIsProRata(),
		ProRataGroup:                          req.GetProRataGroup(),
		ProRataIndex:                          req.GetProRataIndex(),
		ProRataTotal:                          req.GetProRataTotal(),
		ClientGeneratedIdempotencyKey:         req.GetClientGeneratedIdempotencyKey(),
	})
	if err != nil {
		return nil, h.mapServiceError(ctx, err, opCreate, req.GetUserId())
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) CreateProRataInstallment(ctx context.Context, req *pb.CreateProRataInstallmentRequest) (*pb.ExpenseResponse, error) {
	reqModel := &service.CreateProRataInstallmentRequest{
		UserID:              req.GetUserId(),
		Name:                req.GetName(),
		Amount:              req.GetAmountInTransactionCurrencyMinorUnits(),
		TransactionCurrency: req.GetTransactionCurrencyCode(),
		ExpenseType:         req.GetExpenseType(),
		TagID:               req.GetTagId(),
		ExpenseDate:         req.GetExpenseDateIso(),
		ProRataGroup:        req.GetProRataGroup(),
		ProRataIndex:        req.GetProRataIndex(),
		ProRataTotal:        req.GetProRataTotal(),
	}
	if pc := req.GetPeriodContext(); pc != nil {
		reqModel.PeriodContext = service.TrustedPeriodContext{
			PeriodID:          pc.GetPeriodId(),
			UserID:            pc.GetUserId(),
			Year:              pc.GetYear(),
			Month:             pc.GetMonth(),
			ReportingCurrencyCode: pc.GetReportingCurrencyCode(),
			Source:            pc.GetSource(),
		}
	}
	if snap := req.GetCapturedRateSnapshot(); snap != nil {
		reqModel.CapturedRateSnapshot = &service.CapturedRateSnapshot{
			SnapshotVersion: snap.GetSnapshotVersion(),
			Source:          snap.GetSource(),
			BaseCurrency:    snap.GetBaseCurrency(),
			RateTimestamp:   snap.GetRateTimestamp(),
			CapturedAt:      snap.GetCapturedAt(),
			ExpiresAt:       snap.GetExpiresAt(),
			RatesByCurrency: snap.GetRatesByCurrency(),
		}
	}

	expense, err := h.expenseService.CreateProRataInstallment(ctx, reqModel)
	if err != nil {
		return nil, h.mapServiceError(ctx, err, opProRataInstallment, req.GetUserId())
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) GetActiveExpensesForPeriod(ctx context.Context, req *pb.GetActiveExpensesForPeriodRequest) (*pb.ExpenseListResponse, error) {
	result, err := h.expenseService.GetActiveExpensesForPeriod(ctx, &model.GetExpensesRequest{
		UserID:   req.GetUserId(),
		Year:     req.GetYear(),
		Month:    req.GetMonth(),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, h.mapServiceError(ctx, err, opList, req.GetUserId())
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
		return nil, h.mapServiceError(ctx, err, opGet, req.GetUserId())
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) CorrectExpense(ctx context.Context, req *pb.CorrectExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.CorrectExpense(ctx, req.GetUserId(), req.GetExpenseId(), &model.CorrectExpenseRequest{
		Name:                                  req.GetName(),
		AmountInTransactionCurrencyMinorUnits: req.GetAmountInTransactionCurrencyMinorUnits(),
		TransactionCurrencyCode:               req.GetTransactionCurrencyCode(),
		ExpenseType:                           req.GetExpenseType(),
		TagID:                                 req.GetTagId(),
		ExpenseDateIso:                        req.GetExpenseDateIso(),
	})
	if err != nil {
		return nil, h.mapServiceError(ctx, err, opCorrect, req.GetUserId())
	}

	return &pb.ExpenseResponse{
		Expense: expenseToProto(expense),
	}, nil
}

func (h *GRPCHandler) CountExpensesByTag(ctx context.Context, req *pb.CountExpensesByTagRequest) (*pb.CountExpensesByTagResponse, error) {
	count, err := h.expenseService.CountExpensesByTag(ctx, req.GetUserId(), req.GetTagId())
	if err != nil {
		return nil, h.mapServiceError(ctx, err, opCountByTag, req.GetUserId())
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
	var apiErr *apierr.Error
	if errors.As(err, &apiErr) {
		return h.mapServiceError(stream.Context(), err, opStreamAll, req.GetUserId())
	}
	// Normalize cancellation/deadline so gRPC reports Canceled/DeadlineExceeded,
	// not Unknown. Not reported: a client outcome, and one disconnect per request
	// would be an unbounded event source.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	// Stream-send failures are already gRPC-meaningful; surface directly. The
	// consumer (export engine) reports its own failure with the job id, so
	// reporting here would double-bill.
	return err
}

func (h *GRPCHandler) AnonymizeAllUserExpenses(ctx context.Context, req *pb.AnonymizeRequest) (*pb.AnonymizeResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.expenseService.AnonymizeAllUserExpenses(ctx, userID); err != nil {
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     opAnonymize.opName,
			Domain: config.ReportDomain,
			Msg:    "failed to anonymize expenses",
			Data: map[string]any{
				"method":  opAnonymize.rpcMethod,
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to anonymize expenses")
	}

	return &pb.AnonymizeResponse{}, nil
}

func expenseToProto(e *model.Expense) *pb.ExpenseData {
	return &pb.ExpenseData{
		Id:                                    e.ID,
		UserId:                                e.UserID,
		Name:                                  e.Name,
		TransactionCurrencyCode:               e.TransactionCurrencyCode,
		ExpenseType:                           e.ExpenseType,
		TagId:                                 e.TagID,
		ExpenseDateIso:                        e.ExpenseDateIso,
		PeriodYear:                            e.PeriodYear,
		PeriodMonth:                           e.PeriodMonth,
		Status:                                e.Status,
		CorrectsId:                            e.CorrectsID,
		IsProRata:                             e.IsProRata,
		ProRataGroup:                          e.ProRataGroup,
		ProRataIndex:                          e.ProRataIndex,
		ProRataTotal:                          e.ProRataTotal,
		CreatedAt:                             e.CreatedAt,
		OriginalTransactionAmountInMinorUnits: e.OriginalTransactionAmountInMinorUnits,
		ReportingAmountInMinorUnits:           e.ReportingAmountInMinorUnits,
		ReportingCurrencyCode:                 e.ReportingCurrencyCode,
		SourceToTargetExchangeRate:            e.SourceToTargetExchangeRate,
		ExchangeRateSource:                    e.ExchangeRateSource,
		ExchangeRateTimestamp:                 e.ExchangeRateTimestamp,
		ExchangeRateCacheExpiresAt:            e.ExchangeRateCacheExpiresAt,
	}
}

// reportServerFailure reports only server (5xx) errors. Client errors are
// already recorded at the guard that produced them; reporting them here would
// bill error quota for ordinary client input.
func reportServerFailure(ctx context.Context, err error, meta errkit.Meta) {
	if apierr.IsServerError(err) {
		_ = errkit.Report(ctx, err, meta)
	}
}

// mapServiceError converts a service error to a gRPC status. The codes.Internal
// exits report the underlying error against op, since the returned status carries
// no internal detail. op is per-RPC, not a shared constant: one generic operation
// would merge every gRPC failure into a single issue.
func (h *GRPCHandler) mapServiceError(ctx context.Context, err error, op operation, userID string) error {
	var apiErr *apierr.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case apierr.CodeValidation, model.ErrUnsupportedCurrency:
			return status.Error(codes.InvalidArgument, apiErr.Message)
		case apierr.CodeNotFound, model.ErrPeriodNotFound:
			return status.Error(codes.NotFound, apiErr.Message)
		case model.ErrAlreadyCorrected:
			return status.Error(codes.FailedPrecondition, apiErr.Message)
		case model.ErrPeriodLocked:
			return status.Error(codes.PermissionDenied, apiErr.Message)
		case model.ErrConversionUnavailable:
			// FX provider unavailable: a client-retryable outcome, not an internal
			// failure, so it is not reported.
			return status.Error(codes.Unavailable, apiErr.Message)
		case model.ErrSnapshotCurrencyMissing:
			return status.Error(codes.FailedPrecondition, apiErr.Message)
		default:
			reportServerFailure(ctx, err, errkit.Meta{
				Op:     op.opName,
				Domain: config.ReportDomain,
				Msg:    "internal service error",
				Data: map[string]any{
					"method":     op.rpcMethod,
					"user_id":    userID,
					"error_code": apiErr.Code,
				},
			})
			return status.Error(codes.Internal, apiErr.Message)
		}
	}
	reportServerFailure(ctx, err, errkit.Meta{
		Op:     op.opName,
		Domain: config.ReportDomain,
		Msg:    "unclassified service error",
		Data: map[string]any{
			"method":  op.rpcMethod,
			"user_id": userID,
		},
	})
	return status.Error(codes.Internal, "internal error")
}
