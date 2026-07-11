# perf

Shared **test-only** helpers for the gofin latency & efficiency epic. This module
carries no runtime dependencies and is imported only from `_test.go` files in the
gateway, datarights, expense, and finance services. It provides the measurement
primitives every optimization slice (P1-P4) uses to capture baselines and write
regression assertions.

It sits alongside `metrics`, `access`, and `healthcheck` as a shared module and is
registered in `services/go.work`, so `go test ./...` picks it up in the existing
`test-backend` CI job. No new CI job is added.

## Why measure this way

gofin runs on a single VPS with ~5 concurrent users and only 3 days of Prometheus
retention, so neither production traffic nor Prometheus is a durable "did this
help?" signal. The strategy inverts the usual instinct: **the signals we commit
and gate on are the ones that reproduce on any machine.**

| Signal | Portable? | Role |
|--------|-----------|------|
| Upstream call counts (gRPC/repo) | Yes, structural | Committed baseline + CI assertion (`CallCounter`) |
| Query counts / rows scanned | Yes, structural | Committed baseline + CI assertion |
| Complexity-scaling ratio (cost across input sizes) | Yes, the *ratio* is portable | Committed baseline + growth-ratio assertion |
| `allocs/op`, `B/op` | Per arch/compiler only | Baseline is documentation; CI asserts **bounds/ratios**, never committed absolutes (`AssertMaxAllocs`) |
| Wall-clock `ns/op` | No | Same-machine before/after reference only, never a gate |

**Durable, portable signals are allocs bounds, call counts, and scaling ratios.
Committed wall-clock numbers are same-machine references** labeled with machine
spec + date; read them only as the direction and magnitude of a same-session
delta.

### Architecture caveat for allocation counts

`allocs/op` and `B/op` are deterministic for a given compiler, GC, and **CPU
architecture**, but differ across architectures and Go versions. Baselines are
captured on the maintainer's machine (Apple M-series, **arm64**); CI runs on
`ubuntu-latest` (**amd64**). So committed **absolute** alloc numbers are
documentation for the same-machine before/after story: they are **not** valid CI
thresholds. CI alloc assertions use only arch-stable forms:

- `AssertMaxAllocs(t, max, fn)` max-bounds ("≤ N allocations") with headroom.
- Growth-ratios ("allocs must not scale with input size").

Never assert a committed absolute alloc number in CI.

## Helpers

### `CallCounter`

A concurrency-safe counter that spy implementations of gRPC clients / repo
interfaces embed (as a `*perf.CallCounter`) so a test can assert bounds like
"`GetAllUserData` called at most once". Because P2/P4 call spies from inside
`errgroup` fan-out goroutines, all methods are safe for concurrent use.

```go
type financeSpy struct {
    financepb.FinanceServiceClient
    *perf.CallCounter
}

func (s *financeSpy) GetAllUserData(ctx context.Context, in *financepb.GetAllUserDataRequest, _ ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
    s.Record("GetAllUserData")
    return cannedAllUserData, nil
}

// in the test:
spy := &financeSpy{CallCounter: perf.NewCallCounter(), /* ... */}
// ... exercise the code under test ...
if got := spy.Count("GetAllUserData"); got > 1 {
    t.Fatalf("GetAllUserData called %d times; want <=1 (dedup regressed)", got)
}
```

### `AssertMaxAllocs`

Wraps `testing.AllocsPerRun` with a clear observed-vs-allowed failure message.
Pass an arch-stable upper bound with headroom, not the exact number captured on
one machine.

```go
perf.AssertMaxAllocs(t, 8, func() {
    _ = encodeRow(row)
})
// fails with: "allocations exceeded bound: observed 12 allocs/op, allowed <= 8"
```

> **Intentionally not provided:** a `baseline.Load`/`baseline.Compare` helper.
> Nothing would consume it: the before/after comparison is `benchstat old.txt
> new.txt` (local, reviewed in the PR diff) and the CI gate is hand-written
> bound/ratio assertions. Committed baselines are documentation reviewed in the
> PR diff, not a programmatically-read gate.

## Local benchmark / pprof / benchstat workflow

Benchmarks are standard `Benchmark*` functions in `_test.go` files inside the
package under test; where scaling matters, parameterize by input size with
`b.Run` sub-benchmarks.

### Run benchmarks

```sh
# From the service module under test:
go test -bench=. -benchmem -run '^$' ./...
```

`-benchmem` reports `allocs/op` and `B/op` (the portable signals); `-run '^$'`
skips the normal `Test*` functions so only benchmarks run.

### Capture pprof (local diagnostic only)

pprof locates hotspots and confirms the *mechanism* of a win (e.g. OFFSET scan
cost dominates, or streaming removed a large allocation). Absolute CPU numbers are
not portable; alloc profiles corroborate `allocs/op`.

```sh
go test -bench=BenchmarkExportExpenseRead -benchmem \
        -cpuprofile cpu.out -memprofile mem.out -run '^$'
go tool pprof -top cpu.out
go tool pprof -alloc_space -top mem.out
```

### Diff with benchstat

This is the before/after comparison mechanism (same machine, same session):

```sh
go install golang.org/x/perf/cmd/benchstat@latest

# 1. On current main, capture the baseline:
go test -bench=. -benchmem -count=10 -run '^$' ./... > old.txt

# 2. Implement the optimization, then re-run on the SAME machine:
go test -bench=. -benchmem -count=10 -run '^$' ./... > new.txt

# 3. Compare and paste the result into the PR description:
benchstat old.txt new.txt
```

Use `-count=N` so `benchstat` can apply its significance test; rely on
allocs/counts for the hard signal since wall-clock reruns are noisy.

### The `-race` gate

Run the concurrency-safety tests under the race detector locally before pushing
concurrent changes (P2/P4):

```sh
go test -race ./...
```

`-race` is a **local/manual gate**, not part of CI.

## Baseline artifacts

Optimization slices commit baselines under
`services/<service>/perf/baseline/<path>.txt` (plus optional `.pprof`
snapshots). Each file records the portable metrics as primary content, with
wall-clock clearly labeled as a same-machine reference. The committed baseline is
documentation reviewed in the PR diff; the non-flaky regression gate is the
efficiency assertion (`CallCounter` bounds, query-shape checks, `AssertMaxAllocs`
bounds, growth-ratios) that rides the existing `test-backend` job.

## Testing this module

```sh
cd services/perf
go test ./...
go test -race ./...   # exercises CallCounter concurrency
```
