package serverkit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_EmitsServiceAttributeAsJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "finance")

	logger.Info("ready", slog.String("port", "8080"))

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	assert.Equal(t, "finance", record["service"])
	assert.Equal(t, "ready", record["msg"])
	assert.Equal(t, "8080", record["port"])
	assert.Equal(t, "INFO", record["level"])
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"unknown": slog.LevelInfo,
		"":        slog.LevelInfo,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, want, parseLevel(input))
		})
	}
}
