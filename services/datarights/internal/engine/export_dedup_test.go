package engine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExport_DedupesFinanceCalls is the P2a efficiency regression assertion: a
// full export must hit finance for GetAllUserData at most once and never call
// ListTags. It runs the real provider set through the engine with a finance spy
// as the raw client; the engine wraps it in a per-job MemoizedFinanceClient.
// Reverting the dedup (per-provider raw clients, or expenses.buildTagMap calling
// ListTags) makes this fail.
func TestExport_DedupesFinanceCalls(t *testing.T) {
	finance := newFinanceSpy(cannedAllUserData(), cannedTagList())
	repo := newRecordingRepo()

	eng := newExportEngine(finance, repo)
	eng.Submit("job-dedup", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return repo.completedCount() == 1
	}, 2*time.Second, 10*time.Millisecond, "export job did not complete; failures=%v", repo.failures())

	assert.LessOrEqual(t, finance.Count("GetAllUserData"), 1,
		"export must fetch GetAllUserData at most once (dedup regressed)")
	assert.Equal(t, 0, finance.Count("ListTags"),
		"export must not call ListTags; the tag map derives from shared GetAllUserData")
}
