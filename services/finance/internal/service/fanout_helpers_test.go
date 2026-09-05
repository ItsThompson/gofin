package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/perf"
)

// --- Fan-out test infrastructure (shared by regression tests and benchmarks) ---

// periodKey is the {year, month} lookup key used across the fan-out fakes.
func periodKey(year, month int32) [2]int32 { return [2]int32{year, month} }

// monthOp is the CallCounter operation name for a period read.
func monthOp(year, month int32) string { return fmt.Sprintf("%d-%02d", year, month) }

// countingExpenseClient is a concurrency-aware fake ExpenseClient shared by the
// dashboard fan-out regression tests and benchmarks. It records one call per
// (year, month) through an embedded *perf.CallCounter, tracks the maximum number
// of reads in flight simultaneously (so the SetLimit bound can be asserted),
// checks its context on entry so a read launched after a sibling failure returns
// without recording (exercising first-error cancellation alongside the goroutine
// error-wrapping path), and can simulate per-read latency so benchmarks show
// fan-out (max) rather than serial (sum) wall-clock. The latency uses a plain
// time.Sleep so it does not allocate, keeping the benchmark's allocs/op a clean
// measure of the production fan-out cost. All methods are safe for concurrent use.
type countingExpenseClient struct {
	counter *perf.CallCounter
	delay   time.Duration
	byMonth map[[2]int32][]ExpenseData
	errs    map[[2]int32]error

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func newCountingExpenseClient() *countingExpenseClient {
	return &countingExpenseClient{
		counter: perf.NewCallCounter(),
		byMonth: make(map[[2]int32][]ExpenseData),
		errs:    make(map[[2]int32]error),
	}
}

// set seeds the expenses returned for a period. Call before the fan-out runs;
// the maps are read-only during concurrent reads.
func (c *countingExpenseClient) set(year, month int32, expenses []ExpenseData) {
	c.byMonth[periodKey(year, month)] = expenses
}

// failOn makes the read for a period return err, exercising the goroutine
// error-wrapping path. Call before the fan-out runs.
func (c *countingExpenseClient) failOn(year, month int32, err error) {
	c.errs[periodKey(year, month)] = err
}

func (c *countingExpenseClient) GetActiveExpensesForPeriod(ctx context.Context, _ string, year, month int32) ([]ExpenseData, error) {
	// Check cancellation before recording: a read launched after a sibling has
	// already failed (and cancelled the errgroup's derived context) returns
	// without recording, so tests can assert siblings were cancelled via the
	// call count. A goroutine handed the parent ctx instead of gctx would not
	// see this cancellation.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.counter.Record(monthOp(year, month))

	c.mu.Lock()
	c.inFlight++
	if c.inFlight > c.maxInFlight {
		c.maxInFlight = c.inFlight
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.inFlight--
		c.mu.Unlock()
	}()

	// Fail fast, before the synthetic delay, so an injected error cancels the
	// group while sibling reads are still in flight.
	if err := c.errs[periodKey(year, month)]; err != nil {
		return nil, err
	}

	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return c.byMonth[periodKey(year, month)], nil
}

// maxConcurrent reports the peak number of reads observed in flight at once.
func (c *countingExpenseClient) maxConcurrent() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxInFlight
}

// CountExpensesByTag and CreateExpense are not used by the dashboard read paths;
// they fail loudly if a path unexpectedly calls them.
func (c *countingExpenseClient) CountExpensesByTag(context.Context, string, string) (int64, error) {
	return 0, fmt.Errorf("CountExpensesByTag not expected in dashboard fan-out tests")
}

func (c *countingExpenseClient) CreateExpense(context.Context, CreateExpenseInput) (*CreatedExpenseData, error) {
	return nil, fmt.Errorf("CreateExpense not expected in dashboard fan-out tests")
}

func (c *countingExpenseClient) CreateProRataInstallment(context.Context, CreateProRataInstallmentInput) (*CreatedExpenseData, error) {
	return nil, fmt.Errorf("CreateProRataInstallment not expected in dashboard fan-out tests")
}

// fakeFanoutRepo returns canned periods for the dashboard read paths. Only the
// two methods those paths call (ListPeriods, GetCurrentPeriod) are implemented;
// the embedded interface is nil, so any other repo call panics and surfaces an
// accidental extra read.
type fakeFanoutRepo struct {
	repository.FinanceRepository
	periods []*model.BudgetPeriod
	current map[[2]int32]*model.BudgetPeriod
}

func (r *fakeFanoutRepo) ListPeriods(context.Context, string) ([]*model.BudgetPeriod, error) {
	return r.periods, nil
}

func (r *fakeFanoutRepo) GetCurrentPeriod(_ context.Context, _ string, year, month int32) (*model.BudgetPeriod, error) {
	return r.current[periodKey(year, month)], nil
}

// newFanoutService wires a FinanceService for fan-out tests/benchmarks with a
// discarded logger and no transaction beginner (the read paths never open a tx).
func newFanoutService(repo repository.FinanceRepository, exp ExpenseClient) *FinanceService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewFinanceService(repo, nil, exp, time.Now, logger)
}

func historicalRepo(periods []*model.BudgetPeriod) *fakeFanoutRepo {
	return &fakeFanoutRepo{
		periods: periods,
		current: map[[2]int32]*model.BudgetPeriod{periodKey(periods[0].Year, periods[0].Month): periods[0]},
	}
}
