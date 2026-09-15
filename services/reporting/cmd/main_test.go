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
