package main

import (
	"bytes"
	"context"
	"testing"
)

func TestRunUsesCanonicalReportWeekFlag(t *testing.T) {
	var output bytes.Buffer
	var errors bytes.Buffer
	if got := run(context.Background(), []string{"--date", "2026-09-07", "--dry-run"}, &output, &errors); got != ExitUsage {
		t.Fatalf("exit code = %d, want usage", got)
	}
	if got := run(context.Background(), []string{"--report-week-start", "2026-09-08", "--dry-run"}, &output, &errors); got != ExitUsage {
		t.Fatalf("exit code = %d, want usage for non-Monday", got)
	}
}

func TestRunRejectsInvalidDateBeforeDependencySetup(t *testing.T) {
	t.Setenv("AUTH_SERVICE_ADDR", "bad host:9081")
	t.Setenv("DISCORD_WEBHOOK_URL", "")
	var output bytes.Buffer
	var errors bytes.Buffer
	if got := run(context.Background(), []string{"--report-week-start", "2026-09-08"}, &output, &errors); got != ExitUsage {
		t.Fatalf("exit code = %d, want usage before setup, error=%q", got, errors.String())
	}
	if got := errors.String(); got != "reporting: invalid report week\n" {
		t.Fatalf("error output = %q", got)
	}
}
