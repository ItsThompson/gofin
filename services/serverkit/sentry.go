package serverkit

import (
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
)

// appName identifies the Go backend in Sentry. It is both the app tag and the
// release-name prefix, so the two can never disagree. Release names must be
// unique across every project in the organization, which is why a bare SHA is
// not enough: the frontend deploys the same SHA under gofin-web@.
const appName = "gofin-api"

// Environment variable names for the Sentry configuration, read in this one
// place so no service grows a second spelling of any of them.
//
// The environment reuses the existing ENVIRONMENT variable rather than a
// parallel SENTRY_ENVIRONMENT. Sentry creates an environment from the first
// event that names it and offers no delete, only hide, and events to a hidden
// environment still consume quota, so a typo is permanent.
const (
	envSentryDSN     = "SENTRY_DSN_BACKEND"
	envSentryRelease = "SENTRY_RELEASE"
	envEnvironment   = "ENVIRONMENT"
)

// defaultEnvironment matches the fallback every service config already applies
// to ENVIRONMENT, so a container started without it does not report under an
// empty environment name.
const defaultEnvironment = "development"

// SentryConfig is the primitive input to the Sentry client options.
type SentryConfig struct {
	// Service is the per-binary name: gateway, auth, expense, finance, or
	// datarights. It becomes the service tag, which is what separates five
	// binaries inside one Sentry project.
	Service string

	// DSN empty means reporting is disabled. That is the correct default for a
	// fresh checkout, local development, and CI, so it is never an error.
	DSN string

	// Environment is the deployment environment, e.g. development or production.
	Environment string

	// Release is the bare deploy SHA. SentryOptions applies the appName prefix,
	// so every caller passes an unprefixed value and the prefix is applied
	// exactly once.
	Release string
}

// SentryConfigFromEnv reads the Sentry configuration for the named service from
// the environment.
//
// serverkit otherwise takes primitive inputs only. This function is the
// deliberate exception: the three variable names below would otherwise be
// spelled once per service config package, which is five places for a value that
// has one meaning.
func SentryConfigFromEnv(service string) SentryConfig {
	environment := os.Getenv(envEnvironment)
	if environment == "" {
		environment = defaultEnvironment
	}

	return SentryConfig{
		Service:     service,
		DSN:         os.Getenv(envSentryDSN),
		Environment: environment,
		Release:     os.Getenv(envSentryRelease),
	}
}

// SentryOptions builds the client options for a service. It is pure: no side
// effects and no SDK initialization, so a test can assert the resulting options
// directly.
//
// That matters for one option in particular. sentry.Init does not return its
// options, and sentry.NewClient overwrites DataCollection with a non-nil
// resolved clone before any Options() accessor can see it, so this return value
// is the only place the nil is observable.
func SentryOptions(cfg SentryConfig) sentry.ClientOptions {
	return sentry.ClientOptions{
		Dsn:         cfg.DSN,
		Environment: cfg.Environment,
		Release:     releaseName(cfg.Release),
		// The three tags every event carries. errkit reserves these keys against
		// a call-site override, because a scope tag beats an option tag and the
		// only symptom would be a cross-project query returning nothing.
		Tags: map[string]string{
			"app":     appName,
			"service": cfg.Service,
			"runtime": "go",
		},
		// TracesSampleRate is left at zero alongside this. Both are zero values:
		// the line is explicitness, not configuration, and mirrors the
		// frontend's tracesSampleRate of 0.
		EnableTracing: false,
		// DataCollection is deliberately absent and SendDefaultPII is left
		// false. A non-nil DataCollection pointer, even an empty struct,
		// resolves permissively: user info on, every HTTP body type collected,
		// and cookies and headers on. A nil pointer resolves through the
		// SendDefaultPII=false path instead, which is the locked-down posture
		// this application needs.
	}
}

// releaseName prefixes a bare deploy SHA. An absent SHA yields an empty release
// rather than a dangling prefix, because a release named "gofin-api@" would
// group every unreleased build together.
func releaseName(sha string) string {
	if sha == "" {
		return ""
	}
	return appName + "@" + sha
}

// InitSentry initializes the SDK from SentryOptions when cfg.DSN is non-empty,
// and reports at startup which of the two it did.
//
// An empty DSN disables reporting and returns nil: the service must start and
// serve normally without one. The startup record exists because an empty DSN in
// production is indistinguishable from a broken integration when looking at
// Sentry alone.
//
// Callers install their logger (slog.SetDefault) before calling this.
func InitSentry(cfg SentryConfig) error {
	if cfg.DSN == "" {
		slog.Info("error reporting disabled: no Sentry DSN configured",
			slog.String("service", cfg.Service),
		)
		return nil
	}

	options := SentryOptions(cfg)
	if err := sentry.Init(options); err != nil {
		return err
	}

	slog.Info("error reporting enabled",
		slog.String("service", cfg.Service),
		slog.String("environment", options.Environment),
		slog.String("release", options.Release),
	)
	return nil
}
