package providers

import (
	"context"

	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// Compile-time check that ExpenseDeletionProvider implements deletion.Provider.
var _ deletion.Provider = (*ExpenseDeletionProvider)(nil)

// ExpenseDeletionProvider anonymizes all expense data for a user via gRPC.
type ExpenseDeletionProvider struct {
	expenseClient expensepb.ExpenseServiceClient
}

// NewExpenseDeletionProvider creates an ExpenseDeletionProvider backed by the expense gRPC client.
func NewExpenseDeletionProvider(expenseClient expensepb.ExpenseServiceClient) *ExpenseDeletionProvider {
	return &ExpenseDeletionProvider{expenseClient: expenseClient}
}

// Name returns a human-readable identifier for this provider.
func (p *ExpenseDeletionProvider) Name() string {
	return "expense"
}

// Delete anonymizes all expenses for the given user.
func (p *ExpenseDeletionProvider) Delete(ctx context.Context, userID string) error {
	_, err := p.expenseClient.AnonymizeAllUserExpenses(ctx, &expensepb.AnonymizeRequest{
		UserId: userID,
	})
	return err
}
