package service

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	fxpb "github.com/ItsThompson/gofin/services/fx/proto/fxpb"
)

// FxCaptureRequest is the Finance-side view of a snapshot-capture request. It
// carries only the currencies known at capture time plus a caller-observed
// request time; FX returns the full USD-based rate map so future target
// currencies can be derived.
type FxCaptureRequest struct {
	RequiredCurrencies []string
	RequestedAt        string
}

// FxClient captures full provider snapshots for pro-rata schedules. Finance
// needs only this one operation: per-installment conversion is delegated to
// Expense, which owns ledger-write conversion.
type FxClient interface {
	CaptureRateSnapshot(ctx context.Context, req FxCaptureRequest) (*model.CapturedRateSnapshot, error)
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
// connection so the caller owns its lifetime and closes it. FX is
// compute-network only, so the transport is insecure.
func NewGRPCFxClientFromAddr(addr string) (*GRPCFxClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("creating fx service client for %s: %w", addr, err)
	}
	return NewGRPCFxClient(fxpb.NewFxServiceClient(conn)), conn, nil
}

// CaptureRateSnapshot calls the FX Service CaptureRateSnapshot RPC. Any failure
// maps to CONVERSION_UNAVAILABLE: the pro-rata schedule must not be written when
// a fresh snapshot cannot be captured (spec 05/06).
func (c *GRPCFxClient) CaptureRateSnapshot(ctx context.Context, req FxCaptureRequest) (*model.CapturedRateSnapshot, error) {
	resp, err := c.client.CaptureRateSnapshot(ctx, &fxpb.CaptureRateSnapshotRequest{
		RequiredCurrencies: req.RequiredCurrencies,
		RequestedAt:        req.RequestedAt,
	})
	if err != nil {
		return nil, fxCaptureUnavailableError()
	}
	return snapshotFromProto(resp.GetSnapshot()), nil
}

func snapshotFromProto(pb *fxpb.CapturedRateSnapshot) *model.CapturedRateSnapshot {
	if pb == nil {
		return nil
	}
	return &model.CapturedRateSnapshot{
		SnapshotVersion: pb.GetSnapshotVersion(),
		Source:          pb.GetSource(),
		BaseCurrency:    pb.GetBaseCurrency(),
		RateTimestamp:   pb.GetRateTimestamp(),
		CapturedAt:      pb.GetCapturedAt(),
		ExpiresAt:       pb.GetExpiresAt(),
		RatesByCurrency: pb.GetRatesByCurrency(),
	}
}

func fxCaptureUnavailableError() *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrConversionUnavailable,
		Message: "Conversion unavailable. Try again later, or enter the manually converted amount in the period currency.",
		Status:  http.StatusServiceUnavailable,
	}
}

var _ FxClient = (*GRPCFxClient)(nil)
