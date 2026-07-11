package serverkit

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger builds the service's JSON slog logger and returns it. The level is
// the raw config string ("debug"|"info"|"warn"|"error"); any unrecognized value
// falls back to info. The service name is attached as a "service" attribute on
// every record. Callers own installing it as the default (slog.SetDefault).
func NewLogger(level, service string) *slog.Logger {
	return newLogger(os.Stdout, level, service)
}

// newLogger is the writer-injectable seam behind NewLogger so tests can assert
// the JSON output and the "service" attribute without capturing os.Stdout.
func newLogger(w io.Writer, level, service string) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler).With(slog.String("service", service))
}

// parseLevel maps the config level string to a slog.Level, defaulting to info.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
