package service

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	fxpb "github.com/ItsThompson/gofin/services/fx/proto/fxpb"
)

// FxConvertRequest is the Expense-service-side view of a conversion request.
// It carries only money facts: no expense name, tag, or user identity. This
// enforces the FX data-minimization rule from the spec.
type FxConvertRequest struct {
	Amount         int64
	SourceCurrency string
	TargetCurrency string
	RequestedAt    string
}

// FxConvertResponse mirrors the FX ConvertAmountResponse fields that the
// Expense ledger snapshot needs. Echoed pair/cache fields are deliberately
// omitted: the snapshot only records the converted amount, rate, source,
// timestamp, and expiry.
type FxConvertResponse struct {
	ConvertedAmount int64
	ExchangeRate    string
	RateTimestamp   string
	Source          string
	ExpiresAt       string
}

// FxCapturedRateSnapshot is the Expense-side view of a captured provider
// snapshot. It carries only the money facts needed for convert-with-snapshot.
type FxCapturedRateSnapshot struct {
	SnapshotVersion int32
	Source          string
	BaseCurrency    string
	RateTimestamp   string
	CapturedAt      string
	ExpiresAt       string
	RatesByCurrency map[string]string
}

// FxConvertWithSnapshotRequest derives a conversion from a previously captured
// snapshot instead of a live provider rate.
type FxConvertWithSnapshotRequest struct {
	Amount         int64
	SourceCurrency string
	TargetCurrency string
	RequestedAt    string
	Snapshot       *FxCapturedRateSnapshot
}

// FxClient converts amounts between currencies using the FX Service. The
// Expense service depends on this interface so tests can inject a mock; the
// gRPC implementation is wired in main.go.
type FxClient interface {
	ConvertAmount(ctx context.Context, req FxConvertRequest) (*FxConvertResponse, error)
	ConvertWithSnapshot(ctx context.Context, req FxConvertWithSnapshotRequest) (*FxConvertResponse, error)
}

// GRPCFxClient implements FxClient over the FX Service gRPC API.
type GRPCFxClient struct {
	client fxpb.FxServiceClient
}

// NewGRPCFxClient wraps an existing FX gRPC client.
func NewGRPCFxClient(client fxpb.FxServiceClient) *GRPCFxClient {
	return &GRPCFxClient{client: client}
}

// NewGRPCFxClientFromAddr builds an FX gRPC client and returns the underlying
// connection so the caller owns its lifetime and closes it. The connection uses
// insecure transport because FX is compute-network only. grpc.NewClient is
// lazy, so this only fails for a malformed target.
func NewGRPCFxClientFromAddr(addr string) (*GRPCFxClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("creating fx service client for %s: %w", addr, err)
	}
	return NewGRPCFxClient(fxpb.NewFxServiceClient(conn)), conn, nil
}

// ConvertAmount converts an amount through the FX Service ConvertAmount RPC.
// Per the spec error matrix, CONVERSION_UNAVAILABLE, provider auth failure,
// provider response invalid, and missing live rate all map to the same safe
// REST error and must not result in a ledger write.
func (c *GRPCFxClient) ConvertAmount(ctx context.Context, req FxConvertRequest) (*FxConvertResponse, error) {
	resp, err := c.client.ConvertAmount(ctx, &fxpb.ConvertAmountRequest{
		Amount:         req.Amount,
		SourceCurrency: req.SourceCurrency,
		TargetCurrency: req.TargetCurrency,
		RequestedAt:    req.RequestedAt,
	})
	if err != nil {
		return nil, mapFxError(err)
	}
	return &FxConvertResponse{
		ConvertedAmount: resp.GetConvertedAmount(),
		ExchangeRate:    resp.GetExchangeRate(),
		RateTimestamp:   resp.GetRateTimestamp(),
		Source:          resp.GetSource(),
		ExpiresAt:       resp.GetExpiresAt(),
	}, nil
}

// ConvertWithSnapshot converts an amount through the FX Service
// ConvertWithSnapshot RPC. It never calls the provider: the snapshot is the
// caller's captured intent.
func (c *GRPCFxClient) ConvertWithSnapshot(ctx context.Context, req FxConvertWithSnapshotRequest) (*FxConvertResponse, error) {
	resp, err := c.client.ConvertWithSnapshot(ctx, &fxpb.ConvertWithSnapshotRequest{
		Amount:         req.Amount,
		SourceCurrency: req.SourceCurrency,
		TargetCurrency: req.TargetCurrency,
		RequestedAt:    req.RequestedAt,
		Snapshot:       snapshotToFxProto(req.Snapshot),
	})
	if err != nil {
		return nil, mapFxError(err)
	}
	return &FxConvertResponse{
		ConvertedAmount: resp.GetConvertedAmount(),
		ExchangeRate:    resp.GetExchangeRate(),
		RateTimestamp:   resp.GetRateTimestamp(),
		Source:          resp.GetSource(),
		ExpiresAt:       resp.GetExpiresAt(),
	}, nil
}

func snapshotToFxProto(s *FxCapturedRateSnapshot) *fxpb.CapturedRateSnapshot {
	if s == nil {
		return nil
	}
	return &fxpb.CapturedRateSnapshot{
		SnapshotVersion: s.SnapshotVersion,
		Source:          s.Source,
		BaseCurrency:    s.BaseCurrency,
		RateTimestamp:   s.RateTimestamp,
		CapturedAt:      s.CapturedAt,
		ExpiresAt:       s.ExpiresAt,
		RatesByCurrency: s.RatesByCurrency,
	}
}

// mapFxError maps an FX gRPC failure to the Expense service's REST error while
// preserving the FX error category (spec 05). Expense validates the amount and
// both currencies before calling FX, so InvalidArgument is normally unreachable,
// but the two services deploy independently: catalog or validation skew across a
// rolling deploy must surface as a client error, not a retryable outage.
//
//   - Unavailable / FailedPrecondition: CONVERSION_UNAVAILABLE (503), no write.
//   - InvalidArgument: 400 UNSUPPORTED_CURRENCY or 400 VALIDATION_ERROR.
//   - Internal / unclassified: 500 internal server error, reported as server failure.
//   - Non-gRPC transport failure: CONVERSION_UNAVAILABLE (503).
func mapFxError(err error) *apierr.Error {
	st, ok := status.FromError(err)
	if !ok {
		// A non-gRPC transport failure (connection refused, deadline) is a
		// conversion outage, not an internal Expense failure.
		return conversionUnavailableError()
	}

	switch st.Code() {
	case codes.Unavailable, codes.FailedPrecondition:
		// CONVERSION_UNAVAILABLE, PROVIDER_AUTH_FAILED,
		// PROVIDER_RESPONSE_INVALID, and RATE_MISSING for live conversion all
		// map to the safe retryable 503.
		return conversionUnavailableError()
	case codes.InvalidArgument:
		// UNSUPPORTED_CURRENCY or INVALID_AMOUNT. Preserve the category so the
		// client sees a 400, not a retryable 503.
		if st.Message() == "UNSUPPORTED_CURRENCY" {
			return &apierr.Error{
				Code:    model.ErrUnsupportedCurrency,
				Message: "The FX service rejected the currency pair",
				Status:  http.StatusBadRequest,
				Fields:  map[string]string{"transactionCurrency": "unsupported currency"},
			}
		}
		return apierr.Validation("validation failed", map[string]string{
			"amount": "The FX service rejected the conversion amount",
		})
	default:
		// Internal and anything unclassified is an FX server failure, not a
		// retryable conversion outage. The 5xx status makes the handler report
		// it as a server error.
		return apierr.Internal("currency conversion failed internally")
	}
}

var _ FxClient = (*GRPCFxClient)(nil)
