package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// syncBuffer is a log sink that a test goroutine can read while export workers
// are still writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// newRecordingLogger returns a logger that emits JSON records into the returned
// sink, so a test can assert on what the engine recorded.
func newRecordingLogger() (*slog.Logger, *syncBuffer) {
	sink := &syncBuffer{}
	return slog.New(slog.NewJSONHandler(sink, nil)), sink
}

// newReportingLogger is newRecordingLogger with the sink installed as slog.Default
// as well, which is where errkit writes the record that accompanies a report.
//
// Only a test asserting a reported failure wants it. A test that counts records by
// message and drives a site which both records and reports would see two, because
// a recovered panic deliberately writes its own record beside the report's.
func newReportingLogger(t *testing.T) (*slog.Logger, *syncBuffer) {
	t.Helper()

	logger, sink := newRecordingLogger()

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	return logger, sink
}

// errorRecords parses the captured log output and returns the error-level
// records only.
func errorRecords(t *testing.T, sink *syncBuffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["level"] == slog.LevelError.String() {
			records = append(records, record)
		}
	}
	return records
}

// findRecord returns the single record whose msg matches.
func findRecord(t *testing.T, records []map[string]any, msg string) map[string]any {
	t.Helper()

	var found []map[string]any
	for _, record := range records {
		if record["msg"] == msg {
			found = append(found, record)
		}
	}
	require.Len(t, found, 1, "expected exactly one %q record", msg)
	return found[0]
}

// errFinance fails the engine's single upfront finance fetch with a fixed error,
// which is how the finance_fetch stage is driven from a test.
type errFinance struct {
	financepb.FinanceServiceClient
	err error
}

func (f errFinance) GetAllUserData(context.Context, *financepb.GetAllUserDataRequest, ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
	return nil, f.err
}

// TestEngine_ProviderFailure_RecordsCauseServerSideOnly asserts the split the
// export failure path depends on: the reason persisted through FailJob (and
// shown to the user) stays free of the underlying error, while a second
// server-side record carries that error with its stage and job id.
func TestEngine_ProviderFailure_RecordsCauseServerSideOnly(t *testing.T) {
	repo := &mockRepo{}
	logger, sink := newReportingLogger(t)
	cause := "dial tcp 10.0.0.7:5432: connect: connection refused"

	eng := NewEngine(staticProviders(&stubProvider{
		name: "profile",
		err:  fmt.Errorf("querying profile: %s", cause),
	}), nopFinance{}, repo, newMockSender(), 5, 5*time.Minute, logger)
	eng.Submit("job-cause", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "Failed to collect profile data", failed[0].ErrMsg)
	assert.NotContains(t, failed[0].ErrMsg, cause, "the persisted reason is shown to the user")

	records := errorRecords(t, sink)

	userFacing := findRecord(t, records, "export job failed")
	assert.Equal(t, "Failed to collect profile data", userFacing["error"])
	assert.NotContains(t, userFacing["error"], cause)

	serverSide := findRecord(t, records, "export job failure cause")
	assert.Contains(t, serverSide["error"], cause)
	assert.Equal(t, "collection", serverSide["stage"])
	assert.Equal(t, "job-cause", serverSide["job_id"])
	assert.Equal(t, "user-1", serverSide["user_id"])
}

// TestEngine_FinanceFetchTimeout_RecordsUnderlyingError covers the finance_fetch
// timeout site. The record must name the error the finance call actually
// returned, not the context sentinel the timeout guard classifies on: the
// sentinel is only ever DeadlineExceeded or Canceled and would discard the gRPC
// status.
func TestEngine_FinanceFetchTimeout_RecordsUnderlyingError(t *testing.T) {
	repo := &mockRepo{}
	logger, sink := newReportingLogger(t)
	// The shape a timed-out finance RPC returns: a gRPC status that wraps the
	// sentinel, so the timeout guard still classifies it.
	cause := fmt.Errorf("rpc error: code = DeadlineExceeded desc = finance GetAllUserData: %w", context.DeadlineExceeded)

	eng := NewEngine(staticProviders(&stubProvider{name: "profile"}),
		errFinance{err: cause}, repo, newMockSender(), 5, 5*time.Minute, logger)
	eng.Submit("job-timeout", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, "Export timed out", repo.getFailedJobs()[0].ErrMsg)

	serverSide := findRecord(t, errorRecords(t, sink), "export job failure cause")
	assert.Equal(t, cause.Error(), serverSide["error"])
	assert.NotEqual(t, context.DeadlineExceeded.Error(), serverSide["error"],
		"the record must name the real error, not just the context sentinel")
	assert.Equal(t, "finance_fetch", serverSide["stage"])
	assert.Equal(t, "job-timeout", serverSide["job_id"])
}

// TestEngine_EmailFailure_RecordsUnsanitizedCause asserts the sanitized,
// user-facing reason and the full server-side cause diverge at the one stage
// that persists part of the error text.
func TestEngine_EmailFailure_RecordsUnsanitizedCause(t *testing.T) {
	repo := &mockRepo{}
	logger, sink := newReportingLogger(t)
	cause := "Resend API error (status 429): rate limited"

	eng := NewEngine(staticProviders(&stubProvider{
		name:    "profile",
		headers: []string{"username"},
		rows:    [][]string{{"alex"}},
	}), nopFinance{}, repo, &mockSender{err: fmt.Errorf("%s", cause)}, 5, 5*time.Minute, logger)
	eng.Submit("job-email-cause", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "Email delivery failed: rate limited", failed[0].ErrMsg)
	assert.NotContains(t, failed[0].ErrMsg, "Resend API error")

	serverSide := findRecord(t, errorRecords(t, sink), "export job failure cause")
	assert.Equal(t, cause, serverSide["error"])
	assert.Equal(t, "email_delivery", serverSide["stage"])
}
