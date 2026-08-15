package service

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
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
// Expense ledger snapshot needs.
type FxConvertResponse struct {
	ConvertedAmount int64
	SourceCurrency  string
	TargetCurrency  string
	ExchangeRate    string
	RateTimestamp   string
	Source          string
	CacheStatus     string
	ExpiresAt       string
}

// FxClient converts amounts between currencies using the FX Service. The
// Expense service depends on this interface so tests can inject a mock; the
// gRPC implementation is wired in main.go.
type FxClient interface {
	ConvertAmount(ctx context.Context, req FxConvertRequest) (*FxConvertResponse, error)
}

// GRPCFxClient implements FxClient over the FX Service gRPC API.
type GRPCFxClient struct {
	client fxpb.FxServiceClient
}

// NewGRPCFxClient wraps an existing FX gRPC client.
func NewGRPCFxClient(client fxpb.FxServiceClient) *GRPCFxClient {
	return &GRPCFxClient{client: client}
}

// NewGRPCFxClientFromAddr dials the FX Service at addr and returns a client.
// The connection uses insecure transport because FX is compute-network only.
func NewGRPCFxClientFromAddr(addr string) (*GRPCFxClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connecting to fx service at %s: %w", addr, err)
	}
	return NewGRPCFxClient(fxpb.NewFxServiceClient(conn)), nil
}

// ConvertAmount calls the FX Service ConvertAmount RPC and maps gRPC status
// errors to the Expense service's safe CONVERSION_UNAVAILABLE error. Per the
// spec error matrix, CONVERSION_UNAVAILABLE, provider auth failure, provider
// response invalid, and missing live rate all map to the same safe REST error
// and must not result in a ledger write.
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
		SourceCurrency:  resp.GetSourceCurrency(),
		TargetCurrency:  resp.GetTargetCurrency(),
		ExchangeRate:    resp.GetExchangeRate(),
		RateTimestamp:   resp.GetRateTimestamp(),
		Source:          resp.GetSource(),
		CacheStatus:     resp.GetCacheStatus(),
		ExpiresAt:       resp.GetExpiresAt(),
	}, nil
}

// mapFxError maps an FX gRPC error to the Expense service's safe
// CONVERSION_UNAVAILABLE api error. The Expense service validates supported
// currencies before calling FX, so InvalidArgument (UNSUPPORTED_CURRENCY /
// INVALID_AMOUNT) is unexpected from FX; it is still mapped to
// CONVERSION_UNAVAILABLE so no partial ledger row is written. All FX failure
// paths produce the same safe user-facing REST error (503
// CONVERSION_UNAVAILABLE) per the spec error-handling matrix.
func mapFxError(err error) *apierr.Error {
	st, ok := status.FromError(err)
	if !ok {
		return conversionUnavailableError()
	}
	switch st.Code() {
	case codes.Unavailable, codes.FailedPrecondition, codes.InvalidArgument:
		return conversionUnavailableError()
	default:
		return conversionUnavailableError()
	}
}

var _ FxClient = (*GRPCFxClient)(nil)

// ensureFxClient returns the fx client or a conversion-unavailable error when
// no client is wired. This lets same-currency-only tests pass nil without
// special-casing every call site; a foreign-currency request with no client is
// a safe failure (no ledger write).
func ensureFxClient(fx FxClient) (FxClient, *apierr.Error) {
	if fx == nil {
		return nil, conversionUnavailableError()
	}
	return fx, nil
}
