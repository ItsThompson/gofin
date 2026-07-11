package engine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// benchProviderLatency is the base per-provider simulated upstream latency for
// the collection scaling benchmark. The five providers get multiples of it
// (1x..5x), so the serial sum (15x) and the fan-out max (5x) are far enough
// apart that the reported ns/op makes the shape unmistakable. Wall-clock is a
// same-machine reference only, never a CI threshold.
const benchProviderLatency = 10 * time.Millisecond

// BenchmarkEngineCollectionFanout measures a full export run over five providers
// with differing simulated latency. It drives engine.runExport end-to-end;
// collection dominates because the stub repo, ZIP assembly, and stub sender are
// effectively instant, so the reported ns/op tracks total collection latency:
// ≈ max(providers) under the errgroup fan-out versus ≈ sum(providers) under the
// old serial loop.
func BenchmarkEngineCollectionFanout(b *testing.B) {
	provs := make([]DataProvider, 5)
	for i := range provs {
		provs[i] = &stubProvider{
			name:    fmt.Sprintf("p%d", i),
			headers: []string{"col"},
			rows:    [][]string{{"val"}},
			delay:   time.Duration(i+1) * benchProviderLatency,
		}
	}

	eng := NewEngine(staticProviders(provs...), nil, &mockRepo{}, newMockSender(), 5, time.Minute, newTestLogger())

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = eng.runExport(context.Background(), "job-bench", "user-1", "alex@example.com")
	}
}
