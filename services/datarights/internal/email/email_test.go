package email

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTokens() BrandTokens {
	return BrandTokens{
		Colors: BrandColors{
			Background:        "#ffffff",
			Foreground:        "#0f172a",
			Primary:           "#1a1a1a",
			PrimaryForeground: "#fafafa",
			Muted:             "#f1f5f9",
			MutedForeground:   "#64748b",
			Border:            "#e2e8f0",
		},
		Typography: BrandTypography{
			FontFamily: "'Geist', -apple-system, BlinkMacSystemFont, sans-serif",
			FontSize: map[string]string{
				"sm":   "14px",
				"base": "16px",
				"lg":   "18px",
				"xl":   "24px",
			},
		},
		Spacing: BrandSpacing{
			Sm: "8px",
			Md: "16px",
			Lg: "24px",
			Xl: "32px",
		},
	}
}

func TestResendSender_HTMLTemplateRendering(t *testing.T) {
	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "test@example.com", tokens, testLogger())
	require.NoError(t, err)

	data := sender.buildTemplateData("2026-05-09", "gofin-export-2026-05-09.zip")
	html, err := sender.renderHTML(data)
	require.NoError(t, err)

	// Verify brand colors are substituted (no hardcoded hex in template)
	assert.Contains(t, html, "#f1f5f9", "background color from tokens")
	assert.Contains(t, html, "#0f172a", "foreground color from tokens")
	assert.Contains(t, html, "#e2e8f0", "border color from tokens")
	assert.Contains(t, html, "#64748b", "muted foreground color from tokens")

	// Verify typography tokens
	assert.Contains(t, html, "'Geist', -apple-system, BlinkMacSystemFont, sans-serif")
	assert.Contains(t, html, "24px", "xl font size")
	assert.Contains(t, html, "18px", "lg font size")
	assert.Contains(t, html, "16px", "base font size")
	assert.Contains(t, html, "14px", "sm font size")

	// Verify spacing tokens
	assert.Contains(t, html, "32px", "xl spacing")
	assert.Contains(t, html, "24px", "lg spacing")
	assert.Contains(t, html, "16px", "md spacing")

	// Verify dynamic content
	assert.Contains(t, html, "2026-05-09")
	assert.Contains(t, html, "gofin-export-2026-05-09.zip")

	// Verify structure
	assert.Contains(t, html, "Your data export is ready")
	assert.Contains(t, html, "gofin")
	assert.Contains(t, html, "<!DOCTYPE html>")
}

func TestResendSender_TextTemplateRendering(t *testing.T) {
	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "test@example.com", tokens, testLogger())
	require.NoError(t, err)

	data := sender.buildTemplateData("2026-05-09", "gofin-export-2026-05-09.zip")
	text, err := sender.renderText(data)
	require.NoError(t, err)

	// Verify dynamic content
	assert.Contains(t, text, "2026-05-09")
	assert.Contains(t, text, "gofin-export-2026-05-09.zip")

	// Verify structure
	assert.Contains(t, text, "Your gofin data export is ready")
	assert.Contains(t, text, "CSV files with all your gofin data")
	assert.Contains(t, text, "Export date:")
	assert.Contains(t, text, "File:")

	// Verify no HTML tags in plain text
	assert.NotContains(t, text, "<html>")
	assert.NotContains(t, text, "<table")
	assert.NotContains(t, text, "<td")
}

func TestResendSender_HTMLTemplate_UsesTokensNotHardcodedValues(t *testing.T) {
	// Use distinct non-standard values to ensure the template references tokens
	customTokens := BrandTokens{
		Colors: BrandColors{
			Background:        "#aabbcc",
			Foreground:        "#112233",
			Primary:           "#445566",
			PrimaryForeground: "#778899",
			Muted:             "#ddeeff",
			MutedForeground:   "#001122",
			Border:            "#334455",
		},
		Typography: BrandTypography{
			FontFamily: "CustomFont, monospace",
			FontSize: map[string]string{
				"sm":   "12px",
				"base": "15px",
				"lg":   "20px",
				"xl":   "28px",
			},
		},
		Spacing: BrandSpacing{
			Sm: "4px",
			Md: "12px",
			Lg: "20px",
			Xl: "28px",
		},
	}

	sender, err := NewResendSender("re_test_key", "test@example.com", customTokens, testLogger())
	require.NoError(t, err)

	data := sender.buildTemplateData("2026-01-01", "test.zip")
	html, err := sender.renderHTML(data)
	require.NoError(t, err)

	// Custom colors should appear
	assert.Contains(t, html, "#ddeeff", "custom muted background")
	assert.Contains(t, html, "#112233", "custom foreground")
	assert.Contains(t, html, "#334455", "custom border")
	assert.Contains(t, html, "#001122", "custom muted foreground")
	assert.Contains(t, html, "CustomFont, monospace", "custom font family")
}

func TestLoadBrandTokens_ValidJSON(t *testing.T) {
	jsonData := []byte(`{
		"colors": {
			"background": "#ffffff",
			"foreground": "#0f172a",
			"primary": "#1a1a1a",
			"primaryForeground": "#fafafa",
			"muted": "#f1f5f9",
			"mutedForeground": "#64748b",
			"border": "#e2e8f0",
			"essentials": "#3b82f6",
			"desires": "#f59e0b",
			"savings": "#10b981"
		},
		"typography": {
			"fontFamily": "'Geist', sans-serif",
			"fontSize": {"sm": "14px", "base": "16px", "lg": "18px", "xl": "24px"}
		},
		"spacing": {"sm": "8px", "md": "16px", "lg": "24px", "xl": "32px"}
	}`)

	tokens, err := LoadBrandTokens(jsonData)
	require.NoError(t, err)

	assert.Equal(t, "#ffffff", tokens.Colors.Background)
	assert.Equal(t, "#0f172a", tokens.Colors.Foreground)
	assert.Equal(t, "#1a1a1a", tokens.Colors.Primary)
	assert.Equal(t, "#64748b", tokens.Colors.MutedForeground)
	assert.Equal(t, "#e2e8f0", tokens.Colors.Border)
	assert.Equal(t, "'Geist', sans-serif", tokens.Typography.FontFamily)
	assert.Equal(t, "14px", tokens.Typography.FontSize["sm"])
	assert.Equal(t, "8px", tokens.Spacing.Sm)
	assert.Equal(t, "32px", tokens.Spacing.Xl)
}

func TestLoadBrandTokens_InvalidJSON(t *testing.T) {
	_, err := LoadBrandTokens([]byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing brand tokens")
}

func TestLogSender_LogsInsteadOfSending(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	sender := NewLogSender(logger)

	err := sender.SendExportEmail(context.Background(), "user@example.com", []byte("fake-zip-data"))
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "email delivery disabled")
	assert.Contains(t, output, "user@example.com")
	assert.Contains(t, output, "dev_log_only")
}

func TestResendSender_BuildTemplateData(t *testing.T) {
	tokens := testTokens()
	sender, err := NewResendSender("re_test_key", "from@example.com", tokens, testLogger())
	require.NoError(t, err)

	data := sender.buildTemplateData("2026-03-15", "gofin-export-2026-03-15.zip")

	assert.Equal(t, "2026-03-15", data.ExportDate)
	assert.Equal(t, "gofin-export-2026-03-15.zip", data.FileName)
	assert.Equal(t, tokens.Colors.Background, data.Colors.Background)
	assert.Equal(t, tokens.Colors.Foreground, data.Colors.Foreground)
	assert.Equal(t, tokens.Typography.FontFamily, data.Typography.FontFamily)
	assert.Equal(t, tokens.Spacing.Xl, data.Spacing.Xl)
}
