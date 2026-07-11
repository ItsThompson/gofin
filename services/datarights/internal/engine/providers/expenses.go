package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that ExpensesProvider implements DataProvider.
var _ engine.DataProvider = (*ExpensesProvider)(nil)

// expensesPageSize is the server-side keyset page size requested from the
// StreamAllUserExpenses RPC. It caps how many rows the server materializes per
// page, so consuming the stream incrementally keeps peak memory at
// O(expensesPageSize) instead of O(total rows).
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

// Collect streams every expense for the user in chronological order, resolves
// tag names, and returns the formatted CSV rows.
//
// It consumes the StreamAllUserExpenses server stream and formats each row as it
// arrives (see streamExpenses) rather than buffering the whole raw-proto history
// first. The returned [][]string is the DataProvider contract the export engine
// collects and hands to BuildZIP; a sink that writes each row onward keeps the
// consumer itself at O(pageSize) (see the bounded-memory benchmark).
func (p *ExpensesProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	tagMap, err := p.buildTagMap(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching tags for name resolution: %w", err)
	}

	var rows [][]string
	if err := p.streamExpenses(ctx, userID, tagMap, func(row []string) error {
		rows = append(rows, row)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
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

// streamExpenses consumes the StreamAllUserExpenses server stream and invokes
// emit once per expense, formatted into a CSV row via formatRow, in the order
// the server sends them (chronological: created_at ASC, id ASC). It holds at
// most one row in flight, so a sink that writes each row onward (an incremental
// CSV/ZIP writer) keeps peak memory at O(pageSize) regardless of history size.
//
// The context is checked before each receive so a client disconnect or job
// timeout stops the walk promptly; io.EOF ends the stream cleanly and any other
// receive or emit error is propagated.
func (p *ExpensesProvider) streamExpenses(
	ctx context.Context,
	userID string,
	tagMap map[string]string,
	emit func(row []string) error,
) error {
	// Derive a cancellable context and cancel on every return path so the gRPC
	// client stream is torn down deterministically, including an early
	// emit-error return. Without this, a caller whose own context outlives the
	// call (e.g. context.Background()) would leak the stream.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := p.expenseClient.StreamAllUserExpenses(ctx, &expensepb.StreamAllUserExpensesRequest{
		UserId:   userID,
		PageSize: expensesPageSize,
	})
	if err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		exp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		if err := emit(p.formatRow(exp, tagMap)); err != nil {
			return err
		}
	}
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
