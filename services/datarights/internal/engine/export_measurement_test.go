package engine_test

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
)

var updateGolden = flag.Bool("update", false, "regenerate golden CSV fixtures")

// providerCSV runs a provider and returns the exact CSV bytes it contributes to
// the export ZIP, exercising the real BuildZIP + encoding/csv shipping path.
func providerCSV(t testing.TB, p engine.DataProvider) []byte {
	t.Helper()
	rows, err := p.Collect(context.Background(), "user-1")
	require.NoError(t, err)

	zipBytes, err := engine.BuildZIP([]engine.CSVFile{
		{Name: p.Name() + ".csv", Headers: p.Headers(), Rows: rows},
	})
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)

	f, err := zr.File[0].Open()
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	content, err := io.ReadAll(f)
	require.NoError(t, err)
	return content
}

// TestExportProviders_CSVByteIdentical guards the byte-identical export guarantee:
// every provider's CSV output (headers + rows) for a fixed input
// must match the committed pre-dedup fixture. Regenerate with `-update`.
func TestExportProviders_CSVByteIdentical(t *testing.T) {
	auth := &stubAuthClient{user: cannedUser()}
	expense := &stubExpenseClient{pages: cannedExpensePages()}

	for _, p := range buildRealProviders(auth, expense, cannedAllUserData()) {
		golden := filepath.Join("testdata", "export", p.Name()+".csv")
		got := providerCSV(t, p)

		if *updateGolden {
			require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
			require.NoError(t, os.WriteFile(golden, got, 0o644)) //nolint:gosec
			continue
		}

		want, err := os.ReadFile(golden)
		require.NoError(t, err, "missing golden %s; run: go test -run CSVByteIdentical -update", golden)
		assert.Equal(t, string(want), string(got), "provider %s CSV drifted from pre-dedup fixture", p.Name())
	}
}

// BenchmarkExportCollection measures serial collection across the full provider
// set. After the fetch-once refactor the finance-backed providers are pure
// mappers over the response the engine fetches once (in execute), so collection
// itself issues no finance RPC; this benchmark isolates the per-provider mapping
// plus expense-stream formatting cost. BenchmarkEngineCollectionFanout measures
// the fan-out latency of a full run; the single-fetch guarantee is asserted by
// TestExport_DedupesFinanceCalls.
func BenchmarkExportCollection(b *testing.B) {
	auth := &stubAuthClient{user: cannedUser()}
	financeData := cannedAllUserData()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		expense := &stubExpenseClient{pages: cannedExpensePages()}
		collectAll(b, buildRealProviders(auth, expense, financeData))
	}
}

func collectAll(tb testing.TB, provs []engine.DataProvider) {
	tb.Helper()
	for _, p := range provs {
		if _, err := p.Collect(context.Background(), "user-1"); err != nil {
			tb.Fatalf("collect %s: %v", p.Name(), err)
		}
	}
}
