package engine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExport_DedupesFinanceCalls is the single-fetch guarantee: a full export
// must fetch finance's GetAllUserData exactly once and never call ListTags. The
// engine fetches once in execute and hands the resolved response to the
// finance-backed providers (now pure mappers) and the derived tag map, so the
// guarantee is structural (by construction), not a memoization side effect.
// Reverting to per-provider finance fetches (or an expenses provider that
// fetches its own tag map) makes this fail.
func TestExport_DedupesFinanceCalls(t *testing.T) {
	finance := newFinanceSpy(cannedAllUserData(), cannedTagList())
	repo := newRecordingRepo()

	eng := newExportEngine(finance, repo)
	eng.Submit("job-dedup", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return repo.completedCount() == 1
	}, 2*time.Second, 10*time.Millisecond, "export job did not complete; failures=%v", repo.failures())

	assert.Equal(t, 1, finance.Count("GetAllUserData"),
		"export must fetch GetAllUserData exactly once (single-fetch guarantee)")
	assert.Equal(t, 0, finance.Count("ListTags"),
		"export must not call ListTags; the tag map derives from the single GetAllUserData")
}
