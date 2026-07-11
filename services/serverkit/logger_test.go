package serverkit_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/serverkit"
)

func TestNewLogger_LevelFiltering(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		level        string
		wantEnabled  slog.Level
		wantDisabled slog.Level
	}{
		{level: "debug", wantEnabled: slog.LevelDebug, wantDisabled: slog.LevelDebug - 1},
		{level: "info", wantEnabled: slog.LevelInfo, wantDisabled: slog.LevelDebug},
		{level: "warn", wantEnabled: slog.LevelWarn, wantDisabled: slog.LevelInfo},
		{level: "error", wantEnabled: slog.LevelError, wantDisabled: slog.LevelWarn},
		// Unrecognized values fall back to info.
		{level: "verbose", wantEnabled: slog.LevelInfo, wantDisabled: slog.LevelDebug},
		{level: "", wantEnabled: slog.LevelInfo, wantDisabled: slog.LevelDebug},
	}

	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			logger := serverkit.NewLogger(tc.level, "svc")
			assert.True(t, logger.Enabled(ctx, tc.wantEnabled), "expected %s enabled at %v", tc.level, tc.wantEnabled)
			assert.False(t, logger.Enabled(ctx, tc.wantDisabled), "expected %s disabled at %v", tc.level, tc.wantDisabled)
		})
	}
}
