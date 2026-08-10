package errkit

import (
	"log/slog"

	"github.com/getsentry/sentry-go"
)

const (
	// sentryDefaultGrouping is Sentry's own placeholder for whatever it computed
	// from the stack, the exception types, and the message. Keeping it as the
	// first fingerprint element means a logical key can only refine Sentry's
	// grouping, never replace it, so two issues Sentry already separated can
	// never be merged back together.
	sentryDefaultGrouping = "{{ default }}"

	// defaultMsg is the slog message a report falls back to. Call sites are
	// expected to supply Meta.Msg instead: the container log stream is the only
	// record of a failure past Sentry's 30-day retention.
	defaultMsg = "operation failed"

	// contextBlockName is the one Sentry context block errkit writes. sentry-go
	// removed Scope.SetExtra in v0.46.0, and one block name on both sides of the
	// stack keeps scrubbing predictable.
	contextBlockName = "gofin"

	tagErrorKind = "error_kind"
	tagOperation = "operation"
	tagDomain    = "domain"
)

// initOwnedTags are the taxonomy tags set once per process through
// sentry.ClientOptions.Tags. A scope tag beats an option tag, because
// Scope.ApplyToEvent writes unconditionally while the SDK's global-tags
// integration only backfills a key the event lacks. So a caller key here would
// silently replace a per-process constant, and the only symptom would be a
// cross-project Sentry query returning nothing.
var initOwnedTags = map[string]struct{}{
	"app":     {},
	"service": {},
	"runtime": {},
}

// Meta describes a failure. The zero value is valid: Level defaults to error,
// Kind to internal, Msg to a generic string, and the fingerprint derives from Op
// and Kind.
type Meta struct {
	// Level defaults to sentry.LevelError. It sets both the Sentry event level
	// and the slog level of the accompanying log record, so a warning-level
	// report does not emit an error-level log line.
	Level sentry.Level

	// Kind classifies the failure. Becomes the tag error_kind.
	Kind Kind

	// Op is the logical operation, e.g. "expense.create". Becomes the tag
	// operation. Must come from a bounded set: never interpolate an identifier,
	// which would create one Sentry issue per record.
	Op string

	// Domain is the business area, e.g. "expenses". Becomes the tag domain.
	Domain string

	// Msg is the slog message. Defaults to "operation failed".
	Msg string

	// GroupKey overrides the key derived from Op and Kind. The emitted
	// fingerprint is still {"{{ default }}", GroupKey}, so this refines Sentry's
	// grouping rather than replacing it.
	GroupKey string

	// GroupExact replaces Sentry's grouping entirely with {GroupKey}, collapsing
	// every matching event into one issue. Use only for a generic failure whose
	// stack varies but whose meaning is singular, e.g. "db.unreachable".
	// Ignored without GroupKey.
	GroupExact bool

	// Tags are additional low-cardinality string pairs. Values are truncated to
	// 200 characters.
	//
	// A key matching app, service, or runtime is dropped: those three are set once
	// per process at init. A key matching error_kind, operation, or domain replaces
	// the value derived from Kind, Op, and Domain, but replaces only the tag: the
	// fingerprint derives from Op and Kind, so an overridden operation tag names one
	// operation while the issue groups under another.
	//
	// Never put identifiers, amounts, emails, or URLs here: tags are indexed and
	// searchable, and one high-cardinality tag makes every tag distribution useless.
	Tags map[string]string

	// Data is arbitrary structured metadata, sent as the Sentry context block
	// "gofin". Sentry caps the block at 8 kB, so keep it small and flat.
	Data map[string]any
}

// level resolves the event level, defaulting to error.
func (m Meta) level() sentry.Level {
	if m.Level == "" {
		return sentry.LevelError
	}
	return m.Level
}

// message resolves the slog message, defaulting to a generic string.
func (m Meta) message() string {
	if m.Msg == "" {
		return defaultMsg
	}
	return m.Msg
}

// groupKey derives the logical discriminator that differentiates issues when
// Sentry's own grouping cannot, because every error reported through this package
// shares a helper-rooted stack unless WithStack supplies a better one.
func (m Meta) groupKey() string {
	if m.GroupKey != "" {
		return m.GroupKey
	}

	kind := string(m.Kind.resolve())
	if m.Op == "" {
		return kind
	}
	return m.Op + "/" + kind
}

// fingerprint returns the event fingerprint. GroupExact is honored only with an
// explicit GroupKey: without one it is ignored, because a single-empty-string or
// empty fingerprint would merge every event in the service into one issue, which
// is the exact failure this package exists to prevent.
func (m Meta) fingerprint() []string {
	if m.GroupExact && m.GroupKey != "" {
		return []string{m.GroupKey}
	}
	return []string{sentryDefaultGrouping, m.groupKey()}
}

// tags builds the event's tag set. Meta.Tags is applied last, so a caller that
// deliberately supplies error_kind, operation, or domain wins over the derived
// value. The three init-owned taxonomy tags are the exception: they are
// per-process constants with no legitimate call-site override, so a caller key
// matching one is dropped.
func (m Meta) tags() map[string]string {
	tags := make(map[string]string, len(m.Tags)+3)

	putTag(tags, tagErrorKind, string(m.Kind.resolve()))
	putTag(tags, tagOperation, m.Op)
	putTag(tags, tagDomain, m.Domain)

	for key, value := range m.Tags {
		if _, reserved := initOwnedTags[key]; reserved {
			continue
		}
		putTag(tags, key, value)
	}

	return tags
}

// putTag writes value under key unless either is empty. An empty tag is dropped
// rather than sent, so it never appears as a meaningless row in a tag
// distribution, and dropping it leaves any derived value already under key
// intact.
func putTag(tags map[string]string, key, value string) {
	if key == "" || value == "" {
		return
	}
	tags[key] = truncateTagValue(value)
}

// slogLevel maps a Sentry level onto a slog level, so a deliberate
// warning-level report emits a warn record instead of polluting the error stream.
func slogLevel(level sentry.Level) slog.Level {
	switch level {
	case sentry.LevelDebug:
		return slog.LevelDebug
	case sentry.LevelInfo:
		return slog.LevelInfo
	case sentry.LevelWarning:
		return slog.LevelWarn
	default:
		// LevelError and LevelFatal both map to error: slog has no fatal level.
		return slog.LevelError
	}
}
