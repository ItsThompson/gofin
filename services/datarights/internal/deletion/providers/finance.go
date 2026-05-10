package providers

import (
	"context"

	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that FinanceDeletionProvider implements DeletionProvider.
var _ deletion.DeletionProvider = (*FinanceDeletionProvider)(nil)

// FinanceDeletionProvider deletes all financial data for a user via gRPC.
type FinanceDeletionProvider struct {
	financeClient financepb.FinanceServiceClient
}

// NewFinanceDeletionProvider creates a FinanceDeletionProvider backed by the finance gRPC client.
func NewFinanceDeletionProvider(financeClient financepb.FinanceServiceClient) *FinanceDeletionProvider {
	return &FinanceDeletionProvider{financeClient: financeClient}
}

// Name returns a human-readable identifier for this provider.
func (p *FinanceDeletionProvider) Name() string {
	return "finance"
}

// Delete removes all financial data for the given user.
func (p *FinanceDeletionProvider) Delete(ctx context.Context, userID string) error {
	_, err := p.financeClient.DeleteAllUserData(ctx, &financepb.DeleteAllUserDataRequest{
		UserId: userID,
	})
	return err
}
