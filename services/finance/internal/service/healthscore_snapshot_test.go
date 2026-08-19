package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// updateSnapshots regenerates the current-version golden snapshot. CI runs the
// golden tests WITHOUT this flag; a released snapshot's lock entry is never
// rewritten, so a formula or shape change with no FormulaVersion bump trips the
// immutability guard rather than silently rewriting history.
var updateSnapshots = flag.Bool("update", false, "regenerate the current-version health-score golden snapshot")

const snapshotDir = "testdata/healthscore"

// Note: -update (and the version-gated test) only ever writes
// v{FormulaVersion}.json. Earlier vN.json files are frozen historical fixtures:
// they are never regenerated, and they exist so the backward-compat test proves
// an older-version snapshot still deserializes into the current model.

// canonicalHealthScore is a fixed, representative input for the golden snapshot:
// a closed month with a configured budget and a full desires window so every
// current-version component (savings, budget, allocation, stability) and the insight appear.
func canonicalHealthScore() *model.HealthScore {
	period := &model.BudgetPeriod{
		Year: 2026, Month: 3, BudgetAmount: 300000,
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}
	expenses := []ExpenseData{
		{ExpenseType: "essentials", Amount: 140000, ReportingAmount: 140000},
		{ExpenseType: "desires", Amount: 95000, ReportingAmount: 95000},
		{ExpenseType: "savings", Amount: 30000, ReportingAmount: 30000},
	}
	desiresWindow := []int64{60000, 90000, 120000, 80000}
	closedNow := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // March 2026 is closed
	return ComputeHealthScore(period, expenses, desiresWindow, 2026, 3, closedNow, "USD")
}

func canonicalSnapshotBytes(t testing.TB) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(canonicalHealthScore(), "", "  ")
	require.NoError(t, err)
	return append(raw, '\n')
}

// TestHealthScoreGolden_CurrentVersion is the version-gated guard: the canonical
// output at model.FormulaVersion must byte-match the committed v{version}.json.
// A formula or JSON-shape change fails here unless the developer bumps
// FormulaVersion and commits the new snapshot (regenerate with -update).
func TestHealthScoreGolden_CurrentVersion(t *testing.T) {
	got := canonicalSnapshotBytes(t)
	path := filepath.Join(snapshotDir, fmt.Sprintf("v%d.json", model.FormulaVersion))

	if *updateSnapshots {
		require.NoError(t, os.MkdirAll(snapshotDir, 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644)) //nolint:gosec
		require.NoError(t, lockSnapshots())
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err,
		"missing golden %s; bump FormulaVersion and run: go test -run HealthScoreGolden -update", path)
	assert.Equal(t, string(want), string(got),
		"v%d output drifted; a formula or shape change requires a FormulaVersion bump and a new snapshot", model.FormulaVersion)
}

// TestHealthScoreGolden_Deterministic asserts the marshaled snapshot is
// byte-identical across runs (components is a slice and the insight is a flat
// struct, so there is no map-ordering nondeterminism).
func TestHealthScoreGolden_Deterministic(t *testing.T) {
	assert.Equal(t, string(canonicalSnapshotBytes(t)), string(canonicalSnapshotBytes(t)))
}

// TestHealthScoreSnapshots_Immutable freezes every released snapshot: its bytes
// must hash to the entry frozen in snapshots.lock. Rewriting a released vN.json
// without a version bump changes its hash but not the locked entry, so this
// fails and forces a bump plus a new snapshot.
func TestHealthScoreSnapshots_Immutable(t *testing.T) {
	lock, err := readLock()
	require.NoError(t, err)

	files := snapshotFiles(t)
	require.NotEmpty(t, files, "expected at least one committed vN.json snapshot")
	for _, file := range files {
		version := versionFromFilename(file)
		data, err := os.ReadFile(file)
		require.NoError(t, err)

		locked, ok := lock[version]
		require.Truef(t, ok, "no snapshots.lock entry for %s; regenerate with -update", filepath.Base(file))
		assert.Equalf(t, locked, sha256Hex(data),
			"%s changed since it was locked; a released snapshot is immutable, so bump FormulaVersion and add a new vN.json",
			filepath.Base(file))
	}
}

// TestHealthScoreSnapshots_BackwardCompatible asserts every
// committed historical snapshot deserializes into the current model.HealthScore
// and carries the expected fields, so a persisted row (same shape) stays
// readable across versions.
func TestHealthScoreSnapshots_BackwardCompatible(t *testing.T) {
	for _, file := range snapshotFiles(t) {
		data, err := os.ReadFile(file)
		require.NoError(t, err)

		var score model.HealthScore
		require.NoErrorf(t, json.Unmarshal(data, &score),
			"%s must deserialize into the current model.HealthScore", filepath.Base(file))

		name := filepath.Base(file)
		assert.NotZerof(t, score.Year, "%s: year", name)
		assert.NotZerof(t, score.Month, "%s: month", name)
		assert.GreaterOrEqualf(t, score.Total, int32(0), "%s: total lower bound", name)
		assert.LessOrEqualf(t, score.Total, int32(100), "%s: total upper bound", name)
		assert.NotEmptyf(t, score.Band, "%s: band", name)
		assert.NotZerof(t, score.FormulaVersion, "%s: formulaVersion", name)
		require.NotEmptyf(t, score.Components, "%s: components", name)
		for _, component := range score.Components {
			assert.NotEmptyf(t, component.Key, "%s: component key", name)
			assert.LessOrEqualf(t, component.Score, component.Max, "%s: component %s within max", name, component.Key)
		}
		assert.NotEmptyf(t, score.Insight.Summary, "%s: insight summary", name)
	}
}

// --- snapshot lock helpers ---

func snapshotFiles(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(snapshotDir, "v*.json"))
	require.NoError(t, err)
	return files
}

// lockSnapshots records the sha256 of every committed snapshot that is not yet
// locked. It is APPEND-ONLY: an existing (released) entry is never rewritten, so
// -update cannot silently re-hash a released snapshot.
func lockSnapshots() error {
	lock, err := readLock()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(snapshotDir, "v*.json"))
	if err != nil {
		return err
	}

	changed := false
	for _, file := range files {
		version := versionFromFilename(file)
		if _, ok := lock[version]; ok {
			continue // append-only: never rewrite a released entry
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		lock[version] = sha256Hex(data)
		changed = true
	}
	if !changed {
		return nil
	}

	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "snapshots.lock"), append(raw, '\n'), 0o644) //nolint:gosec
}

func readLock() (map[string]string, error) {
	lock := map[string]string{}
	data, err := os.ReadFile(filepath.Join(snapshotDir, "snapshots.lock"))
	if errors.Is(err, os.ErrNotExist) {
		return lock, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return lock, nil
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return lock, nil
}

func versionFromFilename(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(strings.TrimPrefix(base, "v"), ".json")
}

func sha256Hex(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
