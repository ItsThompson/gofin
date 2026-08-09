package serverkit_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/serverkit"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// unroutableDSN is syntactically valid, so a client built from it initializes as
// enabled, and points nowhere, so a test that accidentally emits an event cannot
// reach Sentry.
const unroutableDSN = "https://public@example.invalid/1"

// withUnboundHub restores whatever client the process-wide hub carried before a
// test called InitSentry. sentry.Init binds to that hub, so without this a single
// init would leak into every later test in the package.
func withUnboundHub(t *testing.T) {
	t.Helper()

	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
}

func testSentryConfig() serverkit.SentryConfig {
	return serverkit.SentryConfig{
		Service:     "expense",
		DSN:         unroutableDSN,
		Environment: "production",
		Release:     "abc123",
	}
}

// ---------------------------------------------------------------------------
// The pure option builder
// ---------------------------------------------------------------------------

// TestSentryOptions_CollectsNoPersonalData is the highest-value assertion in the
// Sentry integration, and the only surface it can be made on. sentry.NewClient
// replaces DataCollection with a non-nil resolved clone before any accessor can
// read it, so this fails by construction if asserted after Init.
//
// A non-nil pointer, even &sentry.DataCollection{}, resolves permissively: user
// info on, every HTTP body type collected, cookies and headers on. In a finance
// application that is the worst available one-line change, and it leaves no
// visible symptom.
func TestSentryOptions_CollectsNoPersonalData(t *testing.T) {
	options := serverkit.SentryOptions(testSentryConfig())

	assert.Nil(t, options.DataCollection,
		"DataCollection must never be set: a non-nil pointer opts into permissive collection")
	assert.False(t, options.SendDefaultPII, //nolint:staticcheck // asserted precisely because it is the live input when DataCollection is nil
		"SendDefaultPII is the live input to the nil-DataCollection resolution, not a dead field")
}

func TestSentryOptions_TagsCarryTheConstantTaxonomy(t *testing.T) {
	options := serverkit.SentryOptions(testSentryConfig())

	assert.Equal(t, map[string]string{
		"app":     "gofin-api",
		"service": "expense",
		"runtime": "go",
	}, options.Tags, "these three keys are what errkit reserves against a call-site override")
}

func TestSentryOptions_PassesTheDSNAndEnvironmentThrough(t *testing.T) {
	options := serverkit.SentryOptions(testSentryConfig())

	assert.Equal(t, unroutableDSN, options.Dsn)
	assert.Equal(t, "production", options.Environment)
}

// TestSentryOptions_PrefixesTheBareShaExactlyOnce pins the input contract:
// serverkit is the one owner of the gofin-api@ prefix for the Go services, and
// its input is a bare SHA. The last case is not desirable output, it is the proof
// that no second prefix owner is tolerated by sniffing for an existing one. Three
// consumers share one SENTRY_RELEASE variable and each applies its own prefix, so
// a builder that guessed would make two of them right by accident.
func TestSentryOptions_PrefixesTheBareShaExactlyOnce(t *testing.T) {
	cases := map[string]struct {
		release string
		want    string
	}{
		"bare sha":            {release: "abc123", want: "gofin-api@abc123"},
		"absent sha":          {release: "", want: ""},
		"already prefixed in": {release: "gofin-api@abc123", want: "gofin-api@gofin-api@abc123"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := testSentryConfig()
			cfg.Release = tc.release

			assert.Equal(t, tc.want, serverkit.SentryOptions(cfg).Release)
		})
	}
}

// TestSentryOptions_TracingIsOff pins both halves of the pair, so nobody
// "completes" it by setting a sample rate to match an explicit EnableTracing.
func TestSentryOptions_TracingIsOff(t *testing.T) {
	options := serverkit.SentryOptions(testSentryConfig())

	assert.False(t, options.EnableTracing)
	assert.Zero(t, options.TracesSampleRate)
}

// ---------------------------------------------------------------------------
// Reading the configuration from the environment
// ---------------------------------------------------------------------------

func TestSentryConfigFromEnv_ReadsTheThreeVariables(t *testing.T) {
	t.Setenv("SENTRY_DSN_BACKEND", unroutableDSN)
	t.Setenv("SENTRY_RELEASE", "abc123")
	t.Setenv("ENVIRONMENT", "production")

	assert.Equal(t, serverkit.SentryConfig{
		Service:     "finance",
		DSN:         unroutableDSN,
		Environment: "production",
		Release:     "abc123",
	}, serverkit.SentryConfigFromEnv("finance"))
}

// TestSentryConfigFromEnv_UnsetEnvironmentFallsBackToDevelopment pins that the
// fallback matches every service config's own, so one container started without
// ENVIRONMENT cannot create a second Sentry environment. Sentry environments
// cannot be deleted.
func TestSentryConfigFromEnv_UnsetEnvironmentFallsBackToDevelopment(t *testing.T) {
	t.Setenv("SENTRY_DSN_BACKEND", "")
	t.Setenv("SENTRY_RELEASE", "")
	t.Setenv("ENVIRONMENT", "")

	cfg := serverkit.SentryConfigFromEnv("auth")

	assert.Equal(t, "development", cfg.Environment)
	assert.Empty(t, cfg.DSN)
	assert.Empty(t, cfg.Release)
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------

// TestInitSentry_AnAbsentDSNDisablesReportingWithoutAnError covers the default
// state of a fresh checkout, local development, and CI. A missing DSN must never
// stop a service from starting.
func TestInitSentry_AnAbsentDSNDisablesReportingWithoutAnError(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	withUnboundHub(t)

	cfg := testSentryConfig()
	cfg.DSN = ""

	require.NoError(t, serverkit.InitSentry(cfg))
	assert.Nil(t, sentry.CurrentHub().Client(), "no DSN means no client is bound")

	records, err := logs.RecordsAtLevel("INFO")
	require.NoError(t, err)
	require.Len(t, records, 1, "an operator reading the boot log must be able to tell reporting is off")
	assert.Equal(t, "error reporting disabled: no Sentry DSN configured", records[0]["msg"])
}

func TestInitSentry_BindsAClientCarryingTheBuiltOptions(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	withUnboundHub(t)

	require.NoError(t, serverkit.InitSentry(testSentryConfig()))

	client := sentry.CurrentHub().Client()
	require.NotNil(t, client, "a non-empty DSN must leave a client bound to the hub")
	options := client.Options()
	assert.Equal(t, "gofin-api@abc123", options.Release)
	assert.Equal(t, "production", options.Environment)
	assert.Equal(t, "expense", options.Tags["service"])

	records, err := logs.RecordsAtLevel("INFO")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "error reporting enabled", records[0]["msg"])
	assert.Equal(t, "gofin-api@abc123", records[0]["release"])
}

func TestInitSentry_AMalformedDSNIsReturnedAsAnError(t *testing.T) {
	logger, _ := serverkittest.NewLogger()
	withDefaultLogger(t, logger)
	withUnboundHub(t)

	cfg := testSentryConfig()
	cfg.DSN = "not-a-dsn"

	require.Error(t, serverkit.InitSentry(cfg))
}
