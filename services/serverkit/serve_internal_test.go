package serverkit

import (
	"testing"
	"time"
)

// dockerStopGracePeriod is Docker's default, and no compose service overrides
// stop_grace_period. A container that takes longer is SIGKILLed, which discards
// exactly the buffered events the flush exists to deliver.
const dockerStopGracePeriod = 10 * time.Second

// TestShutdownBudgetFitsTheDockerStopGracePeriod pins the arithmetic the two
// bounds are chosen for, so raising either one fails here rather than silently
// pushing shutdown past the point where the flush is cut off.
func TestShutdownBudgetFitsTheDockerStopGracePeriod(t *testing.T) {
	if total := shutdownTimeout + flushTimeout; total > dockerStopGracePeriod {
		t.Fatalf("shutdown budget %s exceeds Docker's %s stop grace period", total, dockerStopGracePeriod)
	}
}
