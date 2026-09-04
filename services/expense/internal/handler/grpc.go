package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// reportDomain is the domain tag on every report this service makes.
const reportDomain = "expenses"

// operation identifies one RPC to the reporter. name is the bounded logical
// operation an event is grouped and queried by, shared with the REST route that
// serves the same operation because a failure means the same thing over either
// transport; rpc names the entry point that surfaced it, which is what the log
// record and the Sentry context block carry.
//
// They travel as one value so the pairs are enumerated here rather than spelled at
// each call site, where two adjacent strings could be swapped silently.
type operation struct {
	name string
	rpc  string
}

var (
	opCreate             = operation{name: "expense.create", rpc: "CreateExpense"}
	opProRataInstallment = operation{name: "expense.create_pro_rata_installment", rpc: "CreateProRataInstallment"}
	opList               = operation{name: "expense.list", rpc: "GetExpensesForPeriod"}
	opGet                = operation{name: "expense.get", rpc: "GetExpense"}
	opCorrect            = operation{name: "expense.correct", rpc: "CorrectExpense"}
	opCountByTag         = operation{name: "expense.count_by_tag", rpc: "CountExpensesByTag"}
	opStreamAll          = operation{name: "expense.stream_all", rpc: "StreamAllUserExpenses"}
	opAnonymize          = operation{name: "expense.anonymize", rpc: "AnonymizeAllUserExpenses"}
)

// GRPCHandler implements the ExpenseService gRPC server. Each RPC delegates to
// the shared ExpenseService and maps service errors to gRPC status codes.
type GRPCHandler struct {
	pb.UnimplementedExpenseServiceServer
	expenseService *service.ExpenseService
}

// NewGRPCHandler creates a new GRPCHandler.
func NewGRPCHandler(expenseService *service.ExpenseService) *GRPCHandler {
	return &GRPCHandler{
		expenseService: expenseService,
	}
}

func (h *GRPCHandler) CreateExpense(ctx context.Context, req *pb.CreateExpenseRequest) (*pb.ExpenseResponse, error) {
	expense, err := h.expenseService.CreateExpense(ctx, req.GetUserId(), &model.CreateExpenseRequest{
		Name:                          req.GetName(),
		Amount:                        req.GetAmount(),
		TransactionCurrency:           req.GetTransactionCurrency(),
		ExpenseType:                   req.GetExpenseType(),
		TagID:                         req.GetTagId(),
		ExpenseDate:                   req.GetExpenseDate(),
		PeriodYear:                    req.GetPeriodYear(),
		PeriodMonth:                   req.GetPeriodMonth(),
		IsProRata:                     req.GetIsProRata(),
		ProRataGroup:                  req.GetProRataGroup(),
		ProRataIndex:                  req.GetProRataIndex(),
		ProRataTotal:                  req.GetProRataTotal(),
		ClientGeneratedIdempotencyKey: req.GetClientGeneratedIdempotencyKey(),
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
		Amount:              req.GetAmount(),
		TransactionCurrency: req.GetTransactionCurrency(),
		ExpenseType:         req.GetExpenseType(),
		TagID:               req.GetTagId(),
		ExpenseDate:         req.GetExpenseDate(),
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
			ReportingCurrency: pc.GetReportingCurrency(),
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

func (h *GRPCHandler) GetExpensesForPeriod(ctx context.Context, req *pb.GetExpensesForPeriodRequest) (*pb.ExpenseListResponse, error) {
	result, err := h.expenseService.GetExpensesForPeriod(ctx, &model.GetExpensesRequest{
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
		Name:                req.GetName(),
		Amount:              req.GetAmount(),
		TransactionCurrency: req.GetTransactionCurrency(),
		ExpenseType:         req.GetExpenseType(),
		TagID:               req.GetTagId(),
		ExpenseDate:         req.GetExpenseDate(),
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
	// Normalize context cancellation / deadline so gRPC reports codes.Canceled /
	// codes.DeadlineExceeded rather than codes.Unknown. Neither reports: the caller
	// went away or its deadline expired, which is a client outcome, and one
	// disconnect per request would be an unbounded event source.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	// Stream-send failures are already gRPC-meaningful; surface them directly. They
	// do not report either: a send fails because the consumer stopped reading, and
	// the consumer is the export engine, which fails its own job and reports that
	// failure with the job id. Reporting here would bill a second event for it.
	return err
}

func (h *GRPCHandler) AnonymizeAllUserExpenses(ctx context.Context, req *pb.AnonymizeRequest) (*pb.AnonymizeResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.expenseService.AnonymizeAllUserExpenses(ctx, userID); err != nil {
		reportServerFailure(ctx, err, errkit.Meta{
			Op:     opAnonymize.name,
			Domain: reportDomain,
			Msg:    "failed to anonymize expenses",
			Data: map[string]any{
				"method":  opAnonymize.rpc,
				"user_id": userID,
			},
		})
		return nil, status.Error(codes.Internal, "failed to anonymize expenses")
	}

	return &pb.AnonymizeResponse{}, nil
}

// expenseToProto converts a domain Expense to a protobuf ExpenseData.
func expenseToProto(e *model.Expense) *pb.ExpenseData {
	return &pb.ExpenseData{
		Id:                    e.ID,
		UserId:                e.UserID,
		Name:                  e.Name,
		TransactionCurrency:   e.TransactionCurrency,
		ExpenseType:           e.ExpenseType,
		TagId:                 e.TagID,
		ExpenseDate:           e.ExpenseDate,
		PeriodYear:            e.PeriodYear,
		PeriodMonth:           e.PeriodMonth,
		Status:                e.Status,
		CorrectsId:            e.CorrectsID,
		IsProRata:             e.IsProRata,
		ProRataGroup:          e.ProRataGroup,
		ProRataIndex:          e.ProRataIndex,
		ProRataTotal:          e.ProRataTotal,
		CreatedAt:             e.CreatedAt,
		TransactionAmount:     e.TransactionAmount,
		ReportingAmount:       e.ReportingAmount,
		ReportingCurrency:     e.ReportingCurrency,
		ExchangeRate:          e.ExchangeRate,
		ExchangeRateSource:    e.ExchangeRateSource,
		ExchangeRateTimestamp: e.ExchangeRateTimestamp,
		ExchangeRateExpiresAt: e.ExchangeRateExpiresAt,
	}
}

// mapServiceError converts a service-layer error to a gRPC status error. It
// classifies via errors.As so a %w-wrapped *apierr.Error still maps to the
// correct gRPC status code. The two codes.Internal exits report the underlying
// error against op, because the status returned to the caller carries no internal
// detail.
//
// reportServerFailure reports err unless the client is about to receive a client
// error. Both codes.Internal exits below are reachable with a typed *apierr.Error,
// and a code this handler does not map is not evidence that the failure is the
// service's fault: a future 401 or a new conflict code would otherwise start
// billing error quota for ordinary client input, and nothing would fail.
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

// op comes from the caller rather than a constant here, because one generic
// operation would group every gRPC failure in the service into a single issue,
// which is exactly the collapse a shared reporter risks.
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
				Op:     op.name,
				Domain: reportDomain,
				Msg:    "internal service error",
				Data: map[string]any{
					"method":     op.rpc,
					"user_id":    userID,
					"error_code": apiErr.Code,
				},
			})
			return status.Error(codes.Internal, apiErr.Message)
		}
	}
	reportServerFailure(ctx, err, errkit.Meta{
		Op:     op.name,
		Domain: reportDomain,
		Msg:    "unclassified service error",
		Data: map[string]any{
			"method":  op.rpc,
			"user_id": userID,
		},
	})
	return status.Error(codes.Internal, "internal error")
}
