package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

// Compile-time check that ExpensesProvider implements DataProvider.
var _ engine.DataProvider = (*ExpensesProvider)(nil)

// expensesPageSize is the server-side keyset page size requested from the
// StreamAllUserExpenses RPC. It caps how many rows the server materializes per
// page, so consuming the stream incrementally keeps peak memory at
// O(expensesPageSize) instead of O(total rows).
const expensesPageSize = 100

// ExpensesProvider streams all user expenses with pagination and resolves tag
// names from a tag map derived once from the shared per-job finance response.
// It also normalizes legacy migration rows to each period's reporting currency
// using the period currency map derived from the same response.
type ExpensesProvider struct {
	expenseClient    expensepb.ExpenseServiceClient
	tagMap           map[string]string
	periodCurrencies map[string]string
}

// NewExpensesProvider creates an ExpensesProvider backed by the expense gRPC
// client. The tag map (tag id -> name) and period currency map ("year:month" ->
// reporting currency) are derived once upfront from the shared finance
// response, so the expenses provider self-fetches only its expense stream and
// never calls finance itself.
func NewExpensesProvider(
	expenseClient expensepb.ExpenseServiceClient,
	tagMap map[string]string,
	periodCurrencies map[string]string,
) *ExpensesProvider {
	return &ExpensesProvider{
		expenseClient:    expenseClient,
		tagMap:           tagMap,
		periodCurrencies: periodCurrencies,
	}
}

// Name returns the CSV filename for this provider.
func (p *ExpensesProvider) Name() string {
	return "expenses"
}

// Headers returns the CSV column headers for expense data.
func (p *ExpensesProvider) Headers() []string {
	return []string{
		"id", "name", "transaction_amount", "transaction_currency",
		"reporting_amount", "reporting_currency", "exchange_rate",
		"exchange_rate_source", "exchange_rate_timestamp", "expense_type",
		"tag_name", "expense_date", "period_year", "period_month", "status",
		"corrects_id", "is_pro_rata", "pro_rata_group", "pro_rata_index",
		"pro_rata_total", "created_at",
	}
}

// Collect streams every expense for the user in chronological order, resolves
// tag names via the injected tag map, and returns the formatted CSV rows.
//
// It consumes the StreamAllUserExpenses server stream and formats each row as it
// arrives (see streamExpenses) rather than buffering the whole raw-proto history
// first. The returned [][]string is the DataProvider contract the export engine
// collects and hands to BuildZIP; a sink that writes each row onward keeps the
// consumer itself at O(pageSize) (see the bounded-memory benchmark).
func (p *ExpensesProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	var rows [][]string
	if err := p.streamExpenses(ctx, userID, func(row []string) error {
		rows = append(rows, row)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	return rows, nil
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

		row, err := p.formatRow(exp)
		if err != nil {
			return err
		}
		if err := emit(row); err != nil {
			return err
		}
	}
}

// formatRow converts a single expense into a CSV row with all transformations applied.
func (p *ExpensesProvider) formatRow(exp *expensepb.ExpenseData) ([]string, error) {
	snapshot, err := p.resolveSnapshot(exp)
	if err != nil {
		return nil, err
	}

	transactionAmount, err := formatMinorUnits(snapshot.transactionAmount, snapshot.transactionCurrency)
	if err != nil {
		return nil, fmt.Errorf("expense %s transaction amount: %w", exp.GetId(), err)
	}
	reportingAmount, err := formatMinorUnits(snapshot.reportingAmount, snapshot.reportingCurrency)
	if err != nil {
		return nil, fmt.Errorf("expense %s reporting amount: %w", exp.GetId(), err)
	}

	tagName := resolveTagName(exp.GetTagId(), p.tagMap)
	isProRata := formatBool(exp.GetIsProRata())
	proRataGroup := exp.GetProRataGroup()
	proRataIndex := formatOptionalInt(exp.GetProRataIndex(), exp.GetIsProRata())
	proRataTotal := formatOptionalInt(exp.GetProRataTotal(), exp.GetIsProRata())

	return []string{
		exp.GetId(),
		exp.GetName(),
		transactionAmount,
		snapshot.transactionCurrency,
		reportingAmount,
		snapshot.reportingCurrency,
		snapshot.exchangeRate,
		snapshot.exchangeRateSource,
		snapshot.exchangeRateTimestamp,
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
	}, nil
}

// expenseSnapshot carries the resolved money fields for one export row.
type expenseSnapshot struct {
	transactionAmount     int64
	transactionCurrency   string
	reportingAmount       int64
	reportingCurrency     string
	exchangeRate          string
	exchangeRateSource    string
	exchangeRateTimestamp string
}

// resolveSnapshot selects the money facts to export for one expense row.
//
// Identity and open_exchange_rates rows must carry a complete snapshot; a
// missing required field fails the export rather than emitting incorrect money
// facts. Legacy migration rows are normalized to the period's reporting
// currency using the per-job period currency map and emitted with
// exchange_rate_source = identity.
func (p *ExpensesProvider) resolveSnapshot(exp *expensepb.ExpenseData) (expenseSnapshot, error) {
	source := exp.GetExchangeRateSource()

	switch source {
	case exchangesource.Identity, exchangesource.OpenExchangeRates:
		if exp.GetTransactionAmount() == 0 || exp.GetTransactionCurrency() == "" ||
			exp.GetReportingAmount() == 0 || exp.GetReportingCurrency() == "" ||
			exp.GetExchangeRate() == "" || exp.GetExchangeRateTimestamp() == "" {
			return expenseSnapshot{}, fmt.Errorf("expense %s has an incomplete money snapshot", exp.GetId())
		}
		return expenseSnapshot{
			transactionAmount:     exp.GetTransactionAmount(),
			transactionCurrency:   exp.GetTransactionCurrency(),
			reportingAmount:       exp.GetReportingAmount(),
			reportingCurrency:     exp.GetReportingCurrency(),
			exchangeRate:          exp.GetExchangeRate(),
			exchangeRateSource:    source,
			exchangeRateTimestamp: exp.GetExchangeRateTimestamp(),
		}, nil

	case exchangesource.Migration:
		currency := p.resolvePeriodCurrency(exp)
		if currency == "" {
			return expenseSnapshot{}, fmt.Errorf("expense %s legacy row has no resolvable period reporting currency", exp.GetId())
		}
		return expenseSnapshot{
			transactionAmount:     exp.GetTransactionAmount(),
			transactionCurrency:   currency,
			reportingAmount:       exp.GetReportingAmount(),
			reportingCurrency:     currency,
			exchangeRate:          "1",
			exchangeRateSource:    exchangesource.Identity,
			exchangeRateTimestamp: exp.GetExchangeRateTimestamp(),
		}, nil

	default:
		return expenseSnapshot{}, fmt.Errorf("expense %s has invalid exchange_rate_source %q", exp.GetId(), source)
	}
}

// resolvePeriodCurrency returns the immutable reporting currency for the row's
// period, falling back to the reporting currency the expense stream already
// resolved when the period is absent from the finance response.
func (p *ExpensesProvider) resolvePeriodCurrency(exp *expensepb.ExpenseData) string {
	if currency, ok := p.periodCurrencies[periodCurrencyKey(exp.GetPeriodYear(), exp.GetPeriodMonth())]; ok {
		return currency
	}
	return exp.GetReportingCurrency()
}
