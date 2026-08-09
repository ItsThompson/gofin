package errkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

var errRepoNotFound = errors.New("no rows in result set")

// logRecorder captures the records Report writes, so the level mapping and the
// no-DSN behavior are assertable without matching message strings against stderr.
type logRecorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (r *logRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *logRecorder) Handle(_ context.Context, record slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record.Clone())
	return nil
}

func (r *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *logRecorder) WithGroup(string) slog.Handler      { return r }

func (r *logRecorder) Records() []slog.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]slog.Record(nil), r.records...)
}

// installLogRecorder makes the recorder the default logger for the test. Enabled
// always returns true so a debug-level report is captured too.
func installLogRecorder(t *testing.T) *logRecorder {
	t.Helper()

	recorder := &logRecorder{}
	previous := slog.Default()
	slog.SetDefault(slog.New(recorder))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return recorder
}

// withoutGlobalClient detaches any client from the process-global hub, so "no DSN
// configured" is the actual state rather than whatever a sibling test left behind.
func withoutGlobalClient(t *testing.T) {
	t.Helper()

	previous := sentry.CurrentHub().Client()
	sentry.CurrentHub().BindClient(nil)
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
}

// withGlobalClient binds a recording client to the process-global hub, which is
// what Report's clone fallback reaches when the context carries no hub.
func withGlobalClient(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })

	return transport
}

// reportEnv is the fixture every Report test needs: a context carrying its own
// recording hub, which is the shape sentrygin and sentrygrpc produce, plus the
// captured log records.
type reportEnv struct {
	ctx    context.Context
	events *errkittest.Transport
	logs   *logRecorder
}

func newReportEnv(t *testing.T) reportEnv {
	t.Helper()

	transport := &errkittest.Transport{}
	return reportEnv{
		ctx:    errkittest.ContextWithHub(context.Background(), transport),
		events: transport,
		logs:   installLogRecorder(t),
	}
}

func (e reportEnv) singleEvent(t *testing.T) *sentry.Event {
	t.Helper()

	events := e.events.Events()
	require.Len(t, events, 1)
	return events[0]
}

func (e reportEnv) singleRecord(t *testing.T) slog.Record {
	t.Helper()

	records := e.logs.Records()
	require.Len(t, records, 1)
	return records[0]
}

func recordAttrs(record slog.Record) map[string]any {
	attrs := make(map[string]any, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	return attrs
}

func TestReport_NilErrorCapturesNothing(t *testing.T) {
	env := newReportEnv(t)

	assert.NoError(t, errkit.Report(env.ctx, nil, errkit.Meta{Op: "expense.create"}))
	assert.Empty(t, env.events.Events())
	assert.Empty(t, env.logs.Records())
}

// Returning the input rather than the stacked carrier is what lets a call site
// write `return errkit.Report(...)` without changing what its own caller matches.
func TestReport_ReturnsTheInputErrorUnchanged(t *testing.T) {
	env := newReportEnv(t)
	err := fmt.Errorf("find budget: %w", errRepoNotFound)

	got := errkit.Report(env.ctx, err, errkit.Meta{Kind: errkit.KindDatabase})

	assert.Same(t, err, got)
	assert.ErrorIs(t, got, errRepoNotFound)
	_, stacked := got.(interface{ StackTrace() []uintptr })
	assert.False(t, stacked, "Report returned the carrier instead of the caller's error")
}

func TestReport_DerivesTheSlogLevelFromMetaLevel(t *testing.T) {
	tests := []struct {
		level     sentry.Level
		wantEvent sentry.Level
		wantLog   slog.Level
	}{
		{level: "", wantEvent: sentry.LevelError, wantLog: slog.LevelError},
		{level: sentry.LevelFatal, wantEvent: sentry.LevelFatal, wantLog: slog.LevelError},
		{level: sentry.LevelError, wantEvent: sentry.LevelError, wantLog: slog.LevelError},
		{level: sentry.LevelWarning, wantEvent: sentry.LevelWarning, wantLog: slog.LevelWarn},
		{level: sentry.LevelInfo, wantEvent: sentry.LevelInfo, wantLog: slog.LevelInfo},
		{level: sentry.LevelDebug, wantEvent: sentry.LevelDebug, wantLog: slog.LevelDebug},
	}

	for _, tc := range tests {
		t.Run(string(tc.level), func(t *testing.T) {
			env := newReportEnv(t)

			_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Level: tc.level})

			assert.Equal(t, tc.wantEvent, env.singleEvent(t).Level)
			assert.Equal(t, tc.wantLog, env.singleRecord(t).Level)
		})
	}
}

// The log record is the only record of a failure past Sentry's 30-day retention,
// and it is the only record at all in local development and CI.
func TestReport_LogsWhenNoDSNIsConfigured(t *testing.T) {
	logs := installLogRecorder(t)
	withoutGlobalClient(t)
	err := errors.New("connection refused")

	got := errkit.Report(context.Background(), err, errkit.Meta{
		Kind: errkit.KindDatabase,
		Op:   "db.ping",
		Msg:  "failed to ping the database",
	})

	assert.Same(t, err, got)
	records := logs.Records()
	require.Len(t, records, 1)
	assert.Equal(t, "failed to ping the database", records[0].Message)
	assert.Equal(t, slog.LevelError, records[0].Level)
	assert.Equal(t, map[string]any{
		"error":      "connection refused",
		"error_kind": "database",
		"operation":  "db.ping",
	}, recordAttrs(records[0]))
}

func TestReport_LogsMetaDataAlongsideTheTags(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{
		Kind:   errkit.KindDatabase,
		Op:     "expense.create",
		Domain: "expenses",
		Msg:    "failed to insert expense",
		Data:   map[string]any{"expense_id": "e-1", "attempt": 2},
	})

	record := env.singleRecord(t)
	assert.Equal(t, "failed to insert expense", record.Message)
	assert.Equal(t, map[string]any{
		"error":      "boom",
		"error_kind": "database",
		"operation":  "expense.create",
		"domain":     "expenses",
		"expense_id": "e-1",
		"attempt":    int64(2),
	}, recordAttrs(record))
}

func TestReport_DefaultsTheLogMessage(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{})

	assert.Equal(t, "operation failed", env.singleRecord(t).Message)
}

func TestReport_UsesTheHubFromTheContext(t *testing.T) {
	globalEvents := withGlobalClient(t)
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Op: "expense.create"})

	assert.Len(t, env.events.Events(), 1)
	assert.Empty(t, globalEvents.Events(), "the report reached the global hub instead of the context hub")
}

// A background job or a startup path has no request hub. Cloning keeps the report
// working with no request, trace, or user data, which is correct rather than a bug.
func TestReport_FallsBackToACloneOfTheGlobalHub(t *testing.T) {
	installLogRecorder(t)
	globalEvents := withGlobalClient(t)

	_ = errkit.Report(context.Background(), errors.New("boom"), errkit.Meta{
		Kind: errkit.KindInternal,
		Op:   "export_job.run",
	})

	events := globalEvents.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "export_job.run", events[0].Tags["operation"])
}

// Scope mutation escaping into the global hub is the intermittent, silent defect
// section 08 calls the second correctness requirement after grouping.
func TestReport_DoesNotMutateTheGlobalScope(t *testing.T) {
	installLogRecorder(t)
	globalEvents := withGlobalClient(t)
	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_ = errkit.Report(ctx, errors.New("boom"), errkit.Meta{
		Kind:   errkit.KindDatabase,
		Op:     "expense.create",
		Domain: "expenses",
		Tags:   map[string]string{"http_status": "503"},
		Data:   map[string]any{"job_id": "j-1"},
	})
	sentry.CaptureException(errors.New("unrelated"))

	require.Len(t, transport.Events(), 1)
	events := globalEvents.Events()
	require.Len(t, events, 1)
	assert.NotContains(t, events[0].Tags, "error_kind")
	assert.NotContains(t, events[0].Tags, "operation")
	assert.NotContains(t, events[0].Tags, "domain")
	assert.NotContains(t, events[0].Tags, "http_status")
	assert.NotContains(t, events[0].Contexts, "gofin")
	assert.Empty(t, events[0].Fingerprint)
}

// Every goroutine reports a distinct operation and an error naming that same
// operation, so an event carrying another goroutine's tags is detectable.
func TestReport_ConcurrentReportsDoNotShareScope(t *testing.T) {
	const goroutines = 64

	installLogRecorder(t)
	transport := &errkittest.Transport{}
	client := errkittest.NewClient(transport)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func() {
			defer wg.Done()
			op := fmt.Sprintf("op.%d", i)
			ctx := sentry.SetHubOnContext(context.Background(), sentry.NewHub(client, sentry.NewScope()))
			_ = errkit.Report(ctx, fmt.Errorf("boom %s", op), errkit.Meta{Op: op})
		}()
	}
	wg.Wait()

	events := transport.Events()
	require.Len(t, events, goroutines)

	seen := make(map[string]struct{}, goroutines)
	for _, event := range events {
		op := event.Tags["operation"]
		require.NotEmpty(t, op)
		assert.Equal(t, "boom "+op, event.Exception[0].Value, "event carries another goroutine's operation tag")
		assert.Equal(t, []string{"{{ default }}", op + "/internal"}, event.Fingerprint)
		seen[op] = struct{}{}
	}

	want := make(map[string]struct{}, goroutines)
	for i := range goroutines {
		want[fmt.Sprintf("op.%d", i)] = struct{}{}
	}
	assert.Equal(t, want, seen)
}

func TestReport_CancelledContextStillReports(t *testing.T) {
	env := newReportEnv(t)
	ctx, cancel := context.WithCancel(env.ctx)
	cancel()

	_ = errkit.Report(ctx, errors.New("boom"), errkit.Meta{Op: "expense.create"})

	assert.Len(t, env.events.Events(), 1)
	assert.Len(t, env.logs.Records(), 1)
}

func TestReport_DataBecomesTheGofinContextBlock(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{
		Op:   "expense.create",
		Data: map[string]any{"expense_id": "e-1", "stage": "insert"},
	})

	event := env.singleEvent(t)
	assert.Equal(t, sentry.Context{"expense_id": "e-1", "stage": "insert"}, event.Contexts["gofin"])

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), `"extra"`)
}

func TestReport_OmitsTheContextBlockWhenDataIsEmpty(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Op: "expense.create"})

	assert.NotContains(t, env.singleEvent(t).Contexts, "gofin")
}

func TestIgnore_MatchedSentinelIsLoggedAtInfoAndNeverCaptured(t *testing.T) {
	env := newReportEnv(t)

	got := errkit.Ignore(env.ctx, errRepoNotFound, errkit.Meta{
		Kind: errkit.KindNotFound,
		Op:   "budget.get",
		Msg:  "budget not found",
	}, errRepoNotFound)

	assert.Same(t, errRepoNotFound, got)
	assert.Empty(t, env.events.Events())
	record := env.singleRecord(t)
	assert.Equal(t, slog.LevelInfo, record.Level)
	assert.Equal(t, "budget not found", record.Message)
}

func TestIgnore_MatchesASentinelWrappedThroughPercentW(t *testing.T) {
	env := newReportEnv(t)
	err := fmt.Errorf("find budget %q: %w", "b-1", errRepoNotFound)

	got := errkit.Ignore(env.ctx, err, errkit.Meta{Kind: errkit.KindNotFound}, errRepoNotFound)

	assert.Same(t, err, got)
	assert.Empty(t, env.events.Events())
	assert.Equal(t, slog.LevelInfo, env.singleRecord(t).Level)
}

func TestIgnore_UnmatchedErrorFallsThroughToReport(t *testing.T) {
	env := newReportEnv(t)
	err := errors.New("connection refused")

	got := errkit.Ignore(env.ctx, err, errkit.Meta{
		Kind: errkit.KindDatabase,
		Op:   "budget.get",
	}, errRepoNotFound)

	assert.Same(t, err, got)
	event := env.singleEvent(t)
	assert.Equal(t, "database", event.Tags["error_kind"])
	assert.Equal(t, slog.LevelError, env.singleRecord(t).Level)
}

func TestIgnore_NoExpectedErrorSuppliedFallsThroughToReport(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Ignore(env.ctx, errRepoNotFound, errkit.Meta{Kind: errkit.KindNotFound, Op: "budget.get"})

	assert.Len(t, env.events.Events(), 1)
}

func TestIgnore_NilErrorCapturesNothing(t *testing.T) {
	env := newReportEnv(t)

	assert.NoError(t, errkit.Ignore(env.ctx, nil, errkit.Meta{}, errRepoNotFound))
	assert.Empty(t, env.events.Events())
	assert.Empty(t, env.logs.Records())
}
