package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
	pb "github.com/ItsThompson/gofin/services/fx/proto/fxpb"
)

type stubConverter struct {
	captureResponse *model.SnapshotResult
	convertResponse *model.ConvertResponse
	err             error
}

func (s *stubConverter) CaptureSnapshot(context.Context, []string) (*model.SnapshotResult, error) {
	return s.captureResponse, s.err
}

func (s *stubConverter) Convert(context.Context, model.ConvertRequest) (*model.ConvertResponse, error) {
	return s.convertResponse, s.err
}

func (s *stubConverter) ConvertWithSnapshot(model.ConvertWithSnapshotRequest) (*model.ConvertResponse, error) {
	return s.convertResponse, s.err
}

func TestGRPCHandler_MapsFxErrorsToStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     model.ErrorCode
		wantCode codes.Code
	}{
		{name: "unsupported currency", code: model.ErrorUnsupportedCurrency, wantCode: codes.InvalidArgument},
		{name: "invalid amount", code: model.ErrorInvalidAmount, wantCode: codes.InvalidArgument},
		{name: "conversion unavailable", code: model.ErrorConversionUnavailable, wantCode: codes.Unavailable},
		{name: "provider auth failure", code: model.ErrorProviderAuthFailed, wantCode: codes.Unavailable},
		{name: "provider response invalid", code: model.ErrorProviderResponseInvalid, wantCode: codes.Unavailable},
		{name: "missing rate", code: model.ErrorRateMissing, wantCode: codes.FailedPrecondition},
		{name: "snapshot integrity failure", code: model.ErrorSnapshotIntegrityFailure, wantCode: codes.FailedPrecondition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGRPCHandler(&stubConverter{err: model.NewError(tt.code, "", fmt.Errorf("boom"))})

			_, err := handler.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{})

			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.Equal(t, string(tt.code), status.Convert(err).Message())
		})
	}
}

func TestGRPCHandler_MapsConvertResponse(t *testing.T) {
	handler := NewGRPCHandler(&stubConverter{convertResponse: &model.ConvertResponse{
		ConvertedAmount: 500,
		SourceCurrency:  "EUR",
		TargetCurrency:  "GBP",
		ExchangeRate:    "0.625",
		RateTimestamp:   "2026-08-15T10:00:00Z",
		Source:          model.SourceOpenExchangeRates,
		CacheStatus:     model.CacheStatusMiss,
		ExpiresAt:       "2026-08-15T13:00:00Z",
	}})

	response, err := handler.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{})

	require.NoError(t, err)
	assert.Equal(t, int64(500), response.GetConvertedAmount())
	assert.Equal(t, "EUR", response.GetSourceCurrency())
	assert.Equal(t, "GBP", response.GetTargetCurrency())
	assert.Equal(t, "0.625", response.GetExchangeRate())
	assert.Equal(t, model.CacheStatusMiss, response.GetCacheStatus())
}

func TestGRPCHandler_MapsCaptureSnapshotResponse(t *testing.T) {
	handler := NewGRPCHandler(&stubConverter{captureResponse: &model.SnapshotResult{
		CacheStatus: model.CacheStatusHit,
		Snapshot: model.CapturedRateSnapshot{
			SnapshotVersion: model.SnapshotVersion,
			Source:          model.SourceOpenExchangeRates,
			BaseCurrency:    model.BaseCurrencyUSD,
			RateTimestamp:   "2026-08-15T10:00:00Z",
			CapturedAt:      "2026-08-15T12:00:00Z",
			ExpiresAt:       "2026-08-15T13:00:00Z",
			RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.8"},
		},
	}})

	response, err := handler.CaptureRateSnapshot(context.Background(), &pb.CaptureRateSnapshotRequest{})

	require.NoError(t, err)
	assert.Equal(t, model.CacheStatusHit, response.GetCacheStatus())
	assert.Equal(t, int32(1), response.GetSnapshot().GetSnapshotVersion())
	assert.Equal(t, "0.8", response.GetSnapshot().GetRatesByCurrency()["EUR"])
}
