package service

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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
		ExchangeRate:    resp.GetExchangeRate(),
		RateTimestamp:   resp.GetRateTimestamp(),
		Source:          resp.GetSource(),
		ExpiresAt:       resp.GetExpiresAt(),
	}, nil
}

// mapFxError maps every FX gRPC failure to the Expense service's safe
// CONVERSION_UNAVAILABLE error. Expense validates the amount and both
// currencies before calling FX, so the only reachable FX errors are Unavailable
// (CONVERSION_UNAVAILABLE, PROVIDER_AUTH_FAILED, PROVIDER_RESPONSE_INVALID) and
// FailedPrecondition (RATE_MISSING for live conversion). Spec 05 maps all four
// to 503 CONVERSION_UNAVAILABLE with no ledger write. The gRPC status code is
// intentionally not inspected because every FX failure is safe-failed
// identically.
func mapFxError(_ error) *apierr.Error {
	return conversionUnavailableError()
}

var _ FxClient = (*GRPCFxClient)(nil)
