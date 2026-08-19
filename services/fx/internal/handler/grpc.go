package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
	pb "github.com/ItsThompson/gofin/services/fx/proto/fxpb"
)

type Converter interface {
	CaptureSnapshot(ctx context.Context, requiredCurrencies []string) (*model.SnapshotResult, error)
	Convert(ctx context.Context, request model.ConvertRequest) (*model.ConvertResponse, error)
	ConvertWithSnapshot(request model.ConvertWithSnapshotRequest) (*model.ConvertResponse, error)
}

type GRPCHandler struct {
	pb.UnimplementedFxServiceServer
	converter Converter
}

func NewGRPCHandler(converter Converter) *GRPCHandler {
	return &GRPCHandler{converter: converter}
}

func (h *GRPCHandler) CaptureRateSnapshot(ctx context.Context, request *pb.CaptureRateSnapshotRequest) (*pb.CaptureRateSnapshotResponse, error) {
	response, err := h.converter.CaptureSnapshot(ctx, request.GetRequiredCurrencies())
	if err != nil {
		return nil, grpcError(err)
	}
	return &pb.CaptureRateSnapshotResponse{
		Snapshot:    toProtoSnapshot(response.Snapshot),
		CacheStatus: response.CacheStatus,
	}, nil
}

func (h *GRPCHandler) ConvertAmount(ctx context.Context, request *pb.ConvertAmountRequest) (*pb.ConvertAmountResponse, error) {
	response, err := h.converter.Convert(ctx, model.ConvertRequest{
		Amount:         request.GetAmount(),
		SourceCurrency: request.GetSourceCurrency(),
		TargetCurrency: request.GetTargetCurrency(),
		RequestedAt:    request.GetRequestedAt(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return toProtoConvertResponse(response), nil
}

func (h *GRPCHandler) ConvertWithSnapshot(ctx context.Context, request *pb.ConvertWithSnapshotRequest) (*pb.ConvertAmountResponse, error) {
	response, err := h.converter.ConvertWithSnapshot(model.ConvertWithSnapshotRequest{
		Amount:         request.GetAmount(),
		SourceCurrency: request.GetSourceCurrency(),
		TargetCurrency: request.GetTargetCurrency(),
		RequestedAt:    request.GetRequestedAt(),
		Snapshot:       fromProtoSnapshot(request.GetSnapshot()),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return toProtoConvertResponse(response), nil
}

func grpcError(err error) error {
	var fxErr *model.Error
	if !errors.As(err, &fxErr) {
		return status.Error(codes.Internal, "INTERNAL")
	}
	switch fxErr.Code {
	case model.ErrorUnsupportedCurrency, model.ErrorInvalidAmount:
		return status.Error(codes.InvalidArgument, string(fxErr.Code))
	case model.ErrorConversionUnavailable, model.ErrorProviderAuthFailed, model.ErrorProviderResponseInvalid:
		return status.Error(codes.Unavailable, string(fxErr.Code))
	case model.ErrorRateMissing, model.ErrorSnapshotIntegrityFailure:
		return status.Error(codes.FailedPrecondition, string(fxErr.Code))
	default:
		return status.Error(codes.Internal, string(fxErr.Code))
	}
}

func toProtoConvertResponse(response *model.ConvertResponse) *pb.ConvertAmountResponse {
	return &pb.ConvertAmountResponse{
		ConvertedAmount: response.ConvertedAmount,
		SourceCurrency:  response.SourceCurrency,
		TargetCurrency:  response.TargetCurrency,
		ExchangeRate:    response.ExchangeRate,
		RateTimestamp:   response.RateTimestamp,
		Source:          response.Source,
		CacheStatus:     response.CacheStatus,
		ExpiresAt:       response.ExpiresAt,
	}
}

func toProtoSnapshot(snapshot model.CapturedRateSnapshot) *pb.CapturedRateSnapshot {
	return &pb.CapturedRateSnapshot{
		SnapshotVersion: snapshot.SnapshotVersion,
		Source:          snapshot.Source,
		BaseCurrency:    snapshot.BaseCurrency,
		RateTimestamp:   snapshot.RateTimestamp,
		CapturedAt:      snapshot.CapturedAt,
		ExpiresAt:       snapshot.ExpiresAt,
		RatesByCurrency: cloneRates(snapshot.RatesByCurrency),
	}
}

func fromProtoSnapshot(snapshot *pb.CapturedRateSnapshot) model.CapturedRateSnapshot {
	if snapshot == nil {
		return model.CapturedRateSnapshot{}
	}
	return model.CapturedRateSnapshot{
		SnapshotVersion: snapshot.GetSnapshotVersion(),
		Source:          snapshot.GetSource(),
		BaseCurrency:    snapshot.GetBaseCurrency(),
		RateTimestamp:   snapshot.GetRateTimestamp(),
		CapturedAt:      snapshot.GetCapturedAt(),
		ExpiresAt:       snapshot.GetExpiresAt(),
		RatesByCurrency: cloneRates(snapshot.GetRatesByCurrency()),
	}
}

func cloneRates(rates map[string]string) map[string]string {
	cloned := make(map[string]string, len(rates))
	for code, rate := range rates {
		cloned[code] = rate
	}
	return cloned
}
