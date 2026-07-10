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

// TestExportProviders_CSVByteIdentical guards the US-EXP-01 byte-identical
// guarantee: every provider's CSV output (headers + rows) for a fixed input
// must match the committed pre-dedup fixture. Regenerate with `-update`.
func TestExportProviders_CSVByteIdentical(t *testing.T) {
	auth := &stubAuthClient{user: cannedUser()}
	expense := &stubExpenseClient{pages: cannedExpensePages()}
	finance := newFinanceSpy(cannedAllUserData(), cannedTagList())

	for _, p := range buildRealProviders(auth, expense, finance) {
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
// set and logs the finance call shape per export. Collection is serial in this
// ticket (the errgroup fan-out is a later slice), so this records the dedup
// story: pre-dedup the finance client is hit GetAllUserData=3 + ListTags=1;
// post-dedup, wrapping the client in a MemoizedFinanceClient collapses that to
// GetAllUserData=1 + ListTags=0.
func BenchmarkExportCollection(b *testing.B) {
	auth := &stubAuthClient{user: cannedUser()}

	observed := newFinanceSpy(cannedAllUserData(), cannedTagList())
	collectAll(b, buildRealProviders(auth, &stubExpenseClient{pages: cannedExpensePages()}, observed))
	b.Logf("finance calls per export: GetAllUserData=%d ListTags=%d total=%d",
		observed.Count("GetAllUserData"), observed.Count("ListTags"), observed.Total())

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		finance := newFinanceSpy(cannedAllUserData(), cannedTagList())
		expense := &stubExpenseClient{pages: cannedExpensePages()}
		collectAll(b, buildRealProviders(auth, expense, finance))
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
