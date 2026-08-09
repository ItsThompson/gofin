package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// explodeIn raises the panic for every test below. It is a named function
// because guardFanout is inlined into its caller, so the recovery closure's own
// frame carries the enclosing service method's name: asserting that name would
// match recovery machinery and pass on a stack that never reached the origin.
// This name appears nowhere in the guard.
func explodeIn(site string) { panic("exploded in " + site) }

// panickingFanoutRepo panics inside one named read and serves canned data for
// every other, so a single fan-out task can panic while its siblings succeed.
// The embedded interface is nil, so an unexpected repo call fails loudly.
type panickingFanoutRepo struct {
	repository.FinanceRepository
	panicOn string
	periods []*model.BudgetPeriod
	current map[[2]int32]*model.BudgetPeriod
}

func (r *panickingFanoutRepo) ListTags(context.Context, string) ([]*model.Tag, error) {
	if r.panicOn == "tags" {
		explodeIn("tag read")
	}
	return []*model.Tag{{ID: "tag-1", UserID: "user-1", Name: "Bills"}}, nil
}

func (r *panickingFanoutRepo) ListPeriods(context.Context, string) ([]*model.BudgetPeriod, error) {
	if r.panicOn == "periods" {
		explodeIn("period read")
	}
	return r.periods, nil
}

func (r *panickingFanoutRepo) GetDefaults(context.Context, string) (*model.DefaultSettings, error) {
	if r.panicOn == "defaults" {
		explodeIn("defaults read")
	}
	return &model.DefaultSettings{UserID: "user-1", Currency: "GBP"}, nil
}

func (r *panickingFanoutRepo) GetCurrentPeriod(_ context.Context, _ string, year, month int32) (*model.BudgetPeriod, error) {
	return r.current[periodKey(year, month)], nil
}

// panickingExpenseClient panics on the period read for (panicYear, panicMonth),
// or on every read when both are zero. Targeting one period matters for the
// health-score path, whose current-month read runs on the caller's goroutine and
// would otherwise panic before the fan-out is reached.
type panickingExpenseClient struct {
	ExpenseClient
	panicYear  int32
	panicMonth int32
}

func (c *panickingExpenseClient) GetExpensesForPeriod(_ context.Context, _ string, year, month int32) ([]ExpenseData, error) {
	if (c.panicYear == 0 && c.panicMonth == 0) || (year == c.panicYear && month == c.panicMonth) {
		explodeIn("expense read")
	}
	return []ExpenseData{{Amount: 1000, ExpenseType: "desires"}}, nil
}

// requireOnePanicRecord asserts the sink holds exactly one error-level record
// carrying the shared fan-out attributes, and that its stack reaches explodeIn
// rather than only the recovery machinery. wantPeriod is the per-iteration
// identity a loop fan-out must record, or "" for a task that runs once.
func requireOnePanicRecord(t *testing.T, sink *serverkittest.Sink, wantTask, wantPeriod string) {
	t.Helper()

	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1, "a recovered panic must produce exactly one error-level record")
	assert.Equal(t, "recovered panic in finance fan-out", records[0]["msg"])
	assert.Equal(t, wantTask, records[0]["task"])
	assert.Equal(t, "user-1", records[0]["user_id"])
	assert.Contains(t, records[0]["stack"], "explodeIn",
		"the stack must reach the panic origin, not only the recovery")

	if wantPeriod == "" {
		assert.NotContains(t, records[0], "period")
		return
	}
	assert.Equal(t, wantPeriod, records[0]["period"],
		"a loop fan-out must name the iteration that panicked, not just the task")
}

// TestGetAllUserData_PanickingRead_ReportsAStackRootedAtTheOrigin is the only
// assertion outside serverkit on the frame skip its recovery helper captures with.
// That skip is a fixed depth, correct only while every call site invokes the
// helper directly from a deferred literal, and this package is one of the five
// that does. Getting it wrong is silent: events still arrive with correct tags,
// and only the grouping degrades, because every panic in the process would then
// share one helper-rooted stack.
//
// It also pins that the hub survives errgroup.WithContext, which is what puts the
// report on the request's hub rather than on a clone of the global one.
func TestGetAllUserData_PanickingRead_ReportsAStackRootedAtTheOrigin(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	repo := &panickingFanoutRepo{panicOn: "tags", periods: fanoutPeriods(1)}
	svc := NewFinanceService(repo, nil, nil, time.Now, logger)

	_, err := svc.GetAllUserData(ctx, "user-1")
	require.Error(t, err)

	events := transport.Events()
	require.Len(t, events, 1, "one panicking task must produce exactly one event")

	event := events[0]
	assert.Equal(t, []string{"{{ default }}", "panic.goroutine.finance_fanout"}, event.Fingerprint)
	assert.Equal(t, "export tag list", event.Contexts["gofin"]["task"],
		"the six tasks share one group key, so the context block has to name which one")

	require.NotEmpty(t, event.Exception)
	stacktrace := event.Exception[len(event.Exception)-1].Stacktrace
	require.NotNil(t, stacktrace, "the outermost exception must carry the stack errkit attached")

	var newestInApp string
	for _, frame := range stacktrace.Frames {
		if frame.InApp {
			newestInApp = frame.Function
		}
	}
	assert.Equal(t, "explodeIn", newestInApp,
		"the reported stack must start at the panicking frame, not at the recovery helper")

	for _, frame := range stacktrace.Frames {
		assert.NotEqual(t, "LogRecoveredPanic", frame.Function,
			"the shared recovery helper must not appear in the stack at all")
	}
}

func fanoutPeriods(months ...int32) []*model.BudgetPeriod {
	periods := make([]*model.BudgetPeriod, 0, len(months))
	for _, month := range months {
		periods = append(periods, &model.BudgetPeriod{
			ID:                "p-" + string(rune('a'+int(month))),
			UserID:            "user-1",
			Year:              2026,
			Month:             month,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		})
	}
	return periods
}

// TestGetAllUserData_PanickingRead_FailsTheRequestInsteadOfTheProcess covers all
// three fan-out reads on the GDPR export path. Without the guard each one takes
// the finance process down: the export's own gRPC recovery wraps the handler
// goroutine, and these run on their own.
func TestGetAllUserData_PanickingRead_FailsTheRequestInsteadOfTheProcess(t *testing.T) {
	cases := map[string]struct {
		panicOn  string
		wantTask string
	}{
		"tag list":         {panicOn: "tags", wantTask: "export tag list"},
		"period list":      {panicOn: "periods", wantTask: "export budget period list"},
		"default settings": {panicOn: "defaults", wantTask: "export default settings"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			logger, sink := serverkittest.NewLogger()
			repo := &panickingFanoutRepo{panicOn: tc.panicOn, periods: fanoutPeriods(1)}
			svc := NewFinanceService(repo, nil, nil, time.Now, logger)

			result, err := svc.GetAllUserData(context.Background(), "user-1")

			require.Error(t, err, "a panicking read must surface as a request error")
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.wantTask+" failed unexpectedly")
			requireOnePanicRecord(t, sink, tc.wantTask, "")
		})
	}
}

func TestGetSpendingTrends_PanickingExpenseRead_FailsTheRequestInsteadOfTheProcess(t *testing.T) {
	logger, sink := serverkittest.NewLogger()
	repo := &fakeFanoutRepo{periods: fanoutPeriods(12, 11)}
	// Only one of the two period reads panics, so the record count is exact and
	// the sibling read exercises the healthy path in the same fan-out.
	svc := NewFinanceService(repo, nil, &panickingExpenseClient{panicYear: 2026, panicMonth: 12}, time.Now, logger)

	result, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, 2)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "spending trends expense read failed unexpectedly")
	requireOnePanicRecord(t, sink, "spending trends expense read", "2026-12")
}

func TestGetHistoricalComparison_PanickingPeriodSpendRead_FailsTheRequestInsteadOfTheProcess(t *testing.T) {
	logger, sink := serverkittest.NewLogger()
	periods := fanoutPeriods(12, 11)
	repo := &panickingFanoutRepo{
		periods: periods,
		current: map[[2]int32]*model.BudgetPeriod{periodKey(2026, 12): periods[0]},
	}
	// Only the current period's spend read panics; the prior period's read
	// succeeds, so exactly one goroutine recovers.
	svc := NewFinanceService(repo, nil, &panickingExpenseClient{panicYear: 2026, panicMonth: 12}, time.Now, logger)

	result, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "historical comparison period spend failed unexpectedly")
	requireOnePanicRecord(t, sink, "historical comparison period spend", "2026-12")
}

func TestGetHealthScore_PanickingDesiresRead_FailsTheRequestInsteadOfTheProcess(t *testing.T) {
	logger, sink := serverkittest.NewLogger()
	// The target month is provisional and read on the caller's goroutine, so the
	// client panics only on the closed prior month the desires window fans out to.
	periods := fanoutPeriods(5, 4)
	repo := &panickingFanoutRepo{
		periods: periods,
		current: map[[2]int32]*model.BudgetPeriod{periodKey(2026, 5): periods[0]},
	}
	now := func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) }
	svc := NewFinanceService(repo, nil, &panickingExpenseClient{panicYear: 2026, panicMonth: 4}, now, logger)

	result, err := svc.GetHealthScore(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "health score desires window failed unexpectedly")
	requireOnePanicRecord(t, sink, "health score desires window", "2026-04")
}

// ---------------------------------------------------------------------------
// The guard itself
// ---------------------------------------------------------------------------

func TestGuardFanout_PassesThroughAHealthyTask(t *testing.T) {
	logger, sink := serverkittest.NewLogger()
	svc := NewFinanceService(nil, nil, nil, time.Now, logger)

	ran := false
	err := svc.guardFanout(context.Background(), "healthy task", "user-1", func() error {
		ran = true
		return nil
	})()

	require.NoError(t, err)
	assert.True(t, ran)
	records, recErr := sink.Records()
	require.NoError(t, recErr)
	assert.Empty(t, records)
}

func TestGuardFanout_PassesThroughATaskError(t *testing.T) {
	logger, sink := serverkittest.NewLogger()
	svc := NewFinanceService(nil, nil, nil, time.Now, logger)
	want := errors.New("upstream read failed")

	err := svc.guardFanout(context.Background(), "failing task", "user-1", func() error { return want })()

	require.ErrorIs(t, err, want, "a task error must pass through unwrapped")
	records, recErr := sink.Records()
	require.NoError(t, recErr)
	assert.Empty(t, records, "an ordinary error is not a recovered panic")
}

// TestEveryFanoutTaskIsGuarded is the only mechanism that can catch a *new*
// unguarded spawn in this package. The sweep for these has been wrong three
// times, and a missed site is invisible: it passes every behavioral test and
// kills the process only in production.
//
// It enforces the call-site style, not just the property: an errgroup task must
// be wrapped in guardFanout on the same line, and a bare `go` statement must not
// appear here at all, because its guard has a different shape and needs a
// deliberate decision plus a test of its own.
//
// It sees only this directory. Promoting it to one repo-level gate over all of
// services/, with the two framework serve loops and the auth ticker loop as an
// explicit allowlist, is recorded as a follow-up in the spec's out-of-scope list.
func TestEveryFanoutTaskIsGuarded(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	taskSite := regexp.MustCompile(`\.Go\(`)
	bareGo := regexp.MustCompile(`(^|[^[:alnum:]_])go (func|[a-zA-Z_][a-zA-Z0-9_.]*\()`)
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		source, err := os.ReadFile(filepath.Clean(name))
		require.NoError(t, err)

		for lineNumber, line := range strings.Split(string(source), "\n") {
			switch {
			case taskSite.MatchString(line):
				checked++
				assert.Contains(t, line, "guardFanout(",
					"%s:%d spawns an errgroup task without guardFanout, so a panic there kills the process",
					name, lineNumber+1)
			case bareGo.MatchString(line):
				assert.Fail(t,
					"unguarded goroutine spawn",
					"%s:%d starts a goroutine directly. recover() does not cross goroutines, so give it its own "+
						"recovery and a test, then teach this test about it",
					name, lineNumber+1)
			}
		}
	}

	assert.Equal(t, 6, checked,
		"expected 6 errgroup task sites in this package; update this count deliberately when adding one")
}
