package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that ExpensesProvider implements DataProvider.
var _ engine.DataProvider = (*ExpensesProvider)(nil)

const expensesPageSize = 100

// ExpensesProvider fetches all user expenses with pagination and resolves tag names.
type ExpensesProvider struct {
	expenseClient expensepb.ExpenseServiceClient
	financeClient financepb.FinanceServiceClient
}

// NewExpensesProvider creates an ExpensesProvider backed by expense and finance gRPC clients.
func NewExpensesProvider(
	expenseClient expensepb.ExpenseServiceClient,
	financeClient financepb.FinanceServiceClient,
) *ExpensesProvider {
	return &ExpensesProvider{
		expenseClient: expenseClient,
		financeClient: financeClient,
	}
}

// Name returns the CSV filename for this provider.
func (p *ExpensesProvider) Name() string {
	return "expenses"
}

// Headers returns the CSV column headers for expense data.
func (p *ExpensesProvider) Headers() []string {
	return []string{
		"id", "name", "amount", "currency", "expense_type", "tag_name",
		"expense_date", "period_year", "period_month", "status",
		"corrects_id", "is_pro_rata", "pro_rata_group", "pro_rata_index",
		"pro_rata_total", "created_at",
	}
}

// Collect fetches all expenses for the user, resolves tag names, and returns formatted rows.
func (p *ExpensesProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	tagMap, err := p.buildTagMap(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching tags for name resolution: %w", err)
	}

	expenses, err := p.fetchAllExpenses(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	rows := make([][]string, 0, len(expenses))
	for _, exp := range expenses {
		rows = append(rows, p.formatRow(exp, tagMap))
	}

	return rows, nil
}

// buildTagMap fetches the user's tags and returns a map of tag ID to tag name.
// It derives the tag map from GetAllUserData().GetTags() (shared with the tags,
// budget_periods, and default_settings providers via the per-job memoized
// finance client) instead of a separate ListTags call, so the export hits
// finance once for this data.
func (p *ExpensesProvider) buildTagMap(ctx context.Context, userID string) (map[string]string, error) {
	resp, err := p.financeClient.GetAllUserData(ctx, &financepb.GetAllUserDataRequest{UserId: userID})
	if err != nil {
		return nil, err
	}

	tags := resp.GetTags()
	tagMap := make(map[string]string, len(tags))
	for _, tag := range tags {
		tagMap[tag.GetId()] = tag.GetName()
	}

	return tagMap, nil
}

// fetchAllExpenses paginates through all expenses for the user.
func (p *ExpensesProvider) fetchAllExpenses(ctx context.Context, userID string) ([]*expensepb.ExpenseData, error) {
	var allExpenses []*expensepb.ExpenseData
	page := int32(1)

	for {
		resp, err := p.expenseClient.GetAllUserExpenses(ctx, &expensepb.GetAllUserExpensesRequest{
			UserId:   userID,
			Page:     page,
			PageSize: expensesPageSize,
		})
		if err != nil {
			return nil, err
		}

		allExpenses = append(allExpenses, resp.GetData()...)

		if !resp.GetHasMore() {
			break
		}
		page++
	}

	return allExpenses, nil
}

// formatRow converts a single expense into a CSV row with all transformations applied.
func (p *ExpensesProvider) formatRow(exp *expensepb.ExpenseData, tagMap map[string]string) []string {
	tagName := resolveTagName(exp.GetTagId(), tagMap)
	amount := formatCentsToDollars(exp.GetAmount())
	isProRata := formatBool(exp.GetIsProRata())

	// Pro-rata fields: render empty string for non-pro-rata expenses
	proRataGroup := exp.GetProRataGroup()
	proRataIndex := formatOptionalInt(exp.GetProRataIndex(), exp.GetIsProRata())
	proRataTotal := formatOptionalInt(exp.GetProRataTotal(), exp.GetIsProRata())

	return []string{
		exp.GetId(),
		exp.GetName(),
		amount,
		exp.GetCurrency(),
		exp.GetExpenseType(),
		tagName,
		exp.GetExpenseDate(),
		strconv.FormatInt(int64(exp.GetPeriodYear()), 10),
		strconv.FormatInt(int64(exp.GetPeriodMonth()), 10),
		exp.GetStatus(),
		exp.GetCorrectsId(),
		isProRata,
		proRataGroup,
		proRataIndex,
		proRataTotal,
		exp.GetCreatedAt(),
	}
}
