package serverkittest_test

import (
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

func TestSink_RecordsEveryRecordInOrder(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	logger.Info("first")
	logger.Error("second", slog.String("key", "value"))

	records, err := sink.Records()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "first", records[0]["msg"])
	assert.Equal(t, "second", records[1]["msg"])
	assert.Equal(t, "value", records[1]["key"])
}

func TestSink_ErrorRecordsFiltersByLevel(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	logger.Debug("debug")
	logger.Warn("warn")
	logger.Error("error")

	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "error", records[0]["msg"])

	warns, err := sink.RecordsAtLevel("WARN")
	require.NoError(t, err)
	require.Len(t, warns, 1)
	assert.Equal(t, "warn", warns[0]["msg"])
}

func TestSink_DebugRecordsAreNotFilteredOut(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	logger.Debug("debug")

	records, err := sink.Records()
	require.NoError(t, err)
	assert.Len(t, records, 1, "the sink logger must admit debug so below-error paths are assertable")
}

func TestSink_IsSafeForConcurrentWriters(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	var waitGroup sync.WaitGroup
	for i := 0; i < 20; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			logger.Error("concurrent")
		}()
	}
	waitGroup.Wait()

	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	assert.Len(t, records, 20)
}
