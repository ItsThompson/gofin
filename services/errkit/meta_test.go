package errkit_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// The fingerprint is the whole mitigation for the wrapper-frame trap: a shared
// helper is the top stack frame of every error it reports, and the Sentry server
// drops the exception message from grouping whenever a stack is present, so
// without a logical key every backend error lands in one issue.
func TestReport_Fingerprint(t *testing.T) {
	tests := []struct {
		name string
		meta errkit.Meta
		want []string
	}{
		{
			name: "Op and Kind",
			meta: errkit.Meta{Op: "expense.create", Kind: errkit.KindDatabase},
			want: []string{"{{ default }}", "expense.create/database"},
		},
		{
			name: "Op empty falls back to Kind alone",
			meta: errkit.Meta{Kind: errkit.KindDatabase},
			want: []string{"{{ default }}", "database"},
		},
		{
			name: "Kind resolves to internal before the key is derived",
			meta: errkit.Meta{Op: "expense.create"},
			want: []string{"{{ default }}", "expense.create/internal"},
		},
		{
			name: "Op and Kind both empty",
			meta: errkit.Meta{},
			want: []string{"{{ default }}", "internal"},
		},
		{
			name: "GroupKey refines Sentry's grouping",
			meta: errkit.Meta{Op: "db.ping", Kind: errkit.KindDatabase, GroupKey: "db.unreachable"},
			want: []string{"{{ default }}", "db.unreachable"},
		},
		{
			name: "GroupExact with a GroupKey replaces Sentry's grouping",
			meta: errkit.Meta{Op: "db.ping", GroupKey: "db.unreachable", GroupExact: true},
			want: []string{"db.unreachable"},
		},
		{
			name: "GroupExact without a GroupKey is ignored",
			meta: errkit.Meta{Op: "expense.create", GroupExact: true},
			want: []string{"{{ default }}", "expense.create/internal"},
		},
		{
			name: "GroupExact on a zero Meta is ignored",
			meta: errkit.Meta{GroupExact: true},
			want: []string{"{{ default }}", "internal"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newReportEnv(t)

			_ = errkit.Report(env.ctx, errors.New("boom"), tc.meta)

			assert.Equal(t, tc.want, env.singleEvent(t).Fingerprint)
		})
	}
}

func TestReport_DistinctOperationsProduceDistinctFingerprints(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Op: "expense.create", Kind: errkit.KindDatabase})
	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Op: "budget.get", Kind: errkit.KindDatabase})

	events := env.events.Events()
	assert.Len(t, events, 2)
	assert.NotEqual(t, events[0].Fingerprint, events[1].Fingerprint)
}

func TestReport_Tags(t *testing.T) {
	tests := []struct {
		name string
		meta errkit.Meta
		want map[string]string
	}{
		{
			name: "zero Meta carries the resolved kind only",
			meta: errkit.Meta{},
			want: map[string]string{"error_kind": "internal"},
		},
		{
			name: "Op and Domain become tags",
			meta: errkit.Meta{Kind: errkit.KindUpstream, Op: "budget.get", Domain: "budgets"},
			want: map[string]string{"error_kind": "upstream", "operation": "budget.get", "domain": "budgets"},
		},
		{
			name: "caller tags are merged",
			meta: errkit.Meta{Op: "budget.get", Tags: map[string]string{"http_status": "503"}},
			want: map[string]string{"error_kind": "internal", "operation": "budget.get", "http_status": "503"},
		},
		{
			name: "a caller key colliding with a derived key wins",
			meta: errkit.Meta{
				Kind:   errkit.KindDatabase,
				Op:     "budget.get",
				Domain: "budgets",
				Tags: map[string]string{
					"error_kind": "timeout",
					"operation":  "budget.list",
					"domain":     "platform",
				},
			},
			want: map[string]string{"error_kind": "timeout", "operation": "budget.list", "domain": "platform"},
		},
		{
			name: "an empty caller value is omitted and leaves the derived value intact",
			meta: errkit.Meta{
				Kind: errkit.KindDatabase,
				Op:   "budget.get",
				Tags: map[string]string{"operation": "", "expected": ""},
			},
			want: map[string]string{"error_kind": "database", "operation": "budget.get"},
		},
		{
			name: "a newline becomes a space",
			meta: errkit.Meta{Tags: map[string]string{"target": "auth\nservice"}},
			want: map[string]string{"error_kind": "internal", "target": "auth service"},
		},
		{
			name: "an empty caller key is omitted",
			meta: errkit.Meta{Tags: map[string]string{"": "orphan"}},
			want: map[string]string{"error_kind": "internal"},
		},
		{
			name: "a caller key matching an init-owned taxonomy tag is dropped",
			meta: errkit.Meta{
				Kind: errkit.KindDatabase,
				Op:   "expense.create",
				Tags: map[string]string{
					"app":         "gofin-web",
					"service":     "gateway",
					"runtime":     "node",
					"http_status": "503",
				},
			},
			want: map[string]string{"error_kind": "database", "operation": "expense.create", "http_status": "503"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newReportEnv(t)

			_ = errkit.Report(env.ctx, errors.New("boom"), tc.meta)

			assert.Equal(t, tc.want, env.singleEvent(t).Tags)
		})
	}
}

// Sentry counts a tag value in characters, so cutting bytes at 200 would split a
// multi-byte character and produce invalid UTF-8.
func TestReport_TruncatesTagValuesRuneSafely(t *testing.T) {
	env := newReportEnv(t)
	value := strings.Repeat("é", 250)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Tags: map[string]string{"target": value}})

	got := env.singleEvent(t).Tags["target"]
	assert.Equal(t, 200, utf8.RuneCountInString(got))
	assert.True(t, utf8.ValidString(got), "truncation split a multi-byte character")
	assert.Equal(t, strings.Repeat("é", 200), got)
}

func TestReport_LeavesATagValueAtTheLimitIntact(t *testing.T) {
	env := newReportEnv(t)
	value := strings.Repeat("a", 200)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{Tags: map[string]string{"target": value}})

	assert.Equal(t, value, env.singleEvent(t).Tags["target"])
}

func TestReport_DefaultsTheEventLevelToError(t *testing.T) {
	env := newReportEnv(t)

	_ = errkit.Report(env.ctx, errors.New("boom"), errkit.Meta{})

	assert.Equal(t, sentry.LevelError, env.singleEvent(t).Level)
}

// The consequence of dropping a caller key that matches the init-owned taxonomy.
// A scope tag beats an option tag, so without the guard an expense handler could
// emit service=gateway and only a cross-project query would ever notice.
func TestReport_InitOwnedTagsSurviveACallerCollision(t *testing.T) {
	installLogRecorder(t)
	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport, func(options *sentry.ClientOptions) {
		options.Tags = map[string]string{"app": "gofin-api", "service": "expense", "runtime": "go"}
	})

	_ = errkit.Report(ctx, errors.New("boom"), errkit.Meta{
		Kind: errkit.KindDatabase,
		Op:   "expense.create",
		Tags: map[string]string{"app": "gofin-web", "service": "gateway", "runtime": "node"},
	})

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, map[string]string{
		"error_kind": "database",
		"operation":  "expense.create",
		"app":        "gofin-api",
		"service":    "expense",
		"runtime":    "go",
	}, events[0].Tags)
}
