package cache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCache(now *time.Time, config Config) *Cache[string, map[string][]string] {
	return New[string, map[string][]string](config, func() time.Time { return *now }, cloneMap, func(value map[string][]string) (int64, bool) {
		return int64(len(value["value"])), true
	})
}

func cloneMap(value map[string][]string) map[string][]string {
	copyValue := make(map[string][]string, len(value))
	for key, values := range value {
		copyValue[key] = append([]string(nil), values...)
	}
	return copyValue
}

type signallingContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *signallingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

func TestCacheLoadHitAndCopySafety(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	config := DefaultConfig()
	cache := testCache(&now, config)
	loads := 0
	loader := func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {"one", "two"}}, nil
	}

	first, firstStatus, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusLoaded, firstStatus)
	first["value"][0] = "changed"

	second, secondStatus, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusHit, secondStatus)
	assert.Equal(t, []string{"one", "two"}, second["value"])
	assert.Equal(t, 1, loads)
}

func TestCacheSlowHitCloneDoesNotBlockOtherKeys(t *testing.T) {
	var block atomic.Bool
	entered := make(chan struct{})
	release := make(chan struct{})
	cache := New[string, string](DefaultConfig(), nil, func(value string) string {
		if value == "slow" && block.Load() {
			close(entered)
			<-release
		}
		return value
	}, func(value string) (int64, bool) { return int64(len(value)), true })
	for _, key := range []string{"slow", "fast"} {
		_, _, err := cache.Load(context.Background(), key, LoadOptions{}, func(context.Context) (string, error) { return key, nil })
		require.NoError(t, err)
	}

	block.Store(true)
	slowDone := make(chan struct{})
	go func() {
		defer close(slowDone)
		value, status, err := cache.Load(context.Background(), "slow", LoadOptions{}, func(context.Context) (string, error) { return "", nil })
		assert.NoError(t, err)
		assert.Equal(t, StatusHit, status)
		assert.Equal(t, "slow", value)
	}()
	<-entered

	fastDone := make(chan struct{})
	go func() {
		defer close(fastDone)
		value, status, err := cache.Load(context.Background(), "fast", LoadOptions{}, func(context.Context) (string, error) { return "", nil })
		assert.NoError(t, err)
		assert.Equal(t, StatusHit, status)
		assert.Equal(t, "fast", value)
	}()
	everyOtherHitCompleted := false
	select {
	case <-fastDone:
		everyOtherHitCompleted = true
	case <-time.After(time.Second):
	}
	evictDone := make(chan struct{})
	go func() {
		cache.Evict("slow")
		close(evictDone)
	}()
	evictionCompleted := false
	select {
	case <-evictDone:
		evictionCompleted = true
	case <-time.After(time.Second):
	}
	block.Store(false)
	close(release)
	<-slowDone
	<-fastDone
	<-evictDone
	assert.True(t, everyOtherHitCompleted, "a slow clone blocked an unrelated cache hit")
	assert.True(t, evictionCompleted, "a slow clone blocked eviction")
	assert.Equal(t, 1, cache.Len())
}

func TestCacheSlowSizingDoesNotBlockOtherKeysAndMeasuresOnce(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var slowMeasurements atomic.Int32
	var blockOnce sync.Once
	cache := New[string, string](DefaultConfig(), nil, func(value string) string { return value }, func(value string) (int64, bool) {
		if value == "slow" {
			slowMeasurements.Add(1)
			blockOnce.Do(func() {
				close(entered)
				<-release
			})
		}
		return int64(len(value)), true
	})
	_, _, err := cache.Load(context.Background(), "fast", LoadOptions{}, func(context.Context) (string, error) { return "fast", nil })
	require.NoError(t, err)

	slowDone := make(chan struct{})
	go func() {
		defer close(slowDone)
		value, status, loadErr := cache.Load(context.Background(), "slow", LoadOptions{}, func(context.Context) (string, error) { return "slow", nil })
		assert.NoError(t, loadErr)
		assert.Equal(t, StatusLoaded, status)
		assert.Equal(t, "slow", value)
	}()
	<-entered

	fastDone := make(chan struct{})
	go func() {
		defer close(fastDone)
		_, status, loadErr := cache.Load(context.Background(), "fast", LoadOptions{}, func(context.Context) (string, error) { return "", nil })
		assert.NoError(t, loadErr)
		assert.Equal(t, StatusHit, status)
	}()
	completed := false
	select {
	case <-fastDone:
		completed = true
	case <-time.After(time.Second):
	}
	close(release)
	<-slowDone
	<-fastDone
	assert.True(t, completed, "sizing a load blocked an unrelated cache hit")
	assert.Equal(t, int32(1), slowMeasurements.Load())
	assert.Equal(t, int64(8), cache.Bytes())
}

func TestCacheExpiryAndCapacityEviction(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	config := DefaultConfig()
	config.MaxEntries = 1
	config.MaxBytes = 100
	config.MaxEntryBytes = 100
	config.MaxAge = time.Hour
	cache := testCache(&now, config)
	loader := func(value string) func(context.Context) (map[string][]string, error) {
		return func(context.Context) (map[string][]string, error) {
			return map[string][]string{"value": {value}}, nil
		}
	}

	_, _, err := cache.Load(context.Background(), "first", LoadOptions{}, loader("first"))
	require.NoError(t, err)
	_, _, err = cache.Load(context.Background(), "second", LoadOptions{}, loader("second"))
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Len())
	_, status, err := cache.Load(context.Background(), "first", LoadOptions{}, loader("first-reloaded"))
	require.NoError(t, err)
	assert.Equal(t, StatusLoaded, status)

	now = now.Add(time.Hour)
	_, status, err = cache.Load(context.Background(), "first", LoadOptions{}, loader("expired"))
	require.NoError(t, err)
	assert.Equal(t, StatusLoaded, status)
}

func TestCacheStoresValidEmptyValues(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	loads := 0
	loader := func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {}}, nil
	}

	first, status, err := cache.Load(context.Background(), "empty", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusLoaded, status)
	assert.Empty(t, first["value"])
	_, status, err = cache.Load(context.Background(), "empty", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusHit, status)
	assert.Equal(t, 1, loads)
}

func TestCacheOversizeAndErrorsAreNotRetained(t *testing.T) {
	now := time.Now()
	config := DefaultConfig()
	config.MaxBytes = 10
	config.MaxEntryBytes = 3
	cache := testCache(&now, config)
	loads := 0
	oversizeLoader := func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {"1", "2", "3", "4"}}, nil
	}

	_, status, err := cache.Load(context.Background(), "oversize", LoadOptions{}, oversizeLoader)
	require.NoError(t, err)
	assert.Equal(t, StatusOversize, status)
	assert.Equal(t, 0, cache.Len())
	_, _, err = cache.Load(context.Background(), "oversize", LoadOptions{}, oversizeLoader)
	require.NoError(t, err)
	assert.Equal(t, 2, loads)

	expected := errors.New("source failed")
	_, status, err = cache.Load(context.Background(), "error", LoadOptions{}, func(context.Context) (map[string][]string, error) {
		return nil, expected
	})
	assert.ErrorIs(t, err, expected)
	assert.Equal(t, StatusError, status)
	assert.Equal(t, 0, cache.Len())
}

func TestCacheMarksUncacheableValuesWithoutRetention(t *testing.T) {
	now := time.Now()
	config := DefaultConfig()
	uncacheable := New[string, map[string][]string](config, func() time.Time { return now }, cloneMap, func(map[string][]string) (int64, bool) {
		return 0, false
	})
	loads := 0
	loader := func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {"source"}}, nil
	}

	_, status, err := uncacheable.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusUncacheable, status)
	assert.Equal(t, 0, uncacheable.Len())
	_, status, err = uncacheable.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusUncacheable, status)
	assert.Equal(t, 2, loads)
}

func TestCacheSingleFlightAndEvictionFence(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	var lock sync.Mutex
	loads := 0
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(context.Context) (map[string][]string, error) {
		lock.Lock()
		loads++
		lock.Unlock()
		close(started)
		<-release
		return map[string][]string{"value": {"same"}}, nil
	}

	firstResult := make(chan map[string][]string, 1)
	go func() {
		value, _, _ := cache.Load(context.Background(), "same-key", LoadOptions{}, loader)
		firstResult <- value
	}()
	<-started
	secondResult := make(chan map[string][]string, 1)
	secondContext := &signallingContext{Context: context.Background(), entered: make(chan struct{})}
	go func() {
		value, _, _ := cache.Load(secondContext, "same-key", LoadOptions{}, loader)
		secondResult <- value
	}()
	<-secondContext.entered
	lock.Lock()
	assert.Equal(t, 1, loads)
	lock.Unlock()
	close(release)
	assert.Equal(t, "same", (<-firstResult)["value"][0])
	assert.Equal(t, "same", (<-secondResult)["value"][0])

	fencedCache := testCache(&now, DefaultConfig())
	fencedStarted := make(chan struct{})
	fencedRelease := make(chan struct{})
	go func() {
		_, _, _ = fencedCache.Load(context.Background(), "evicted-key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
			close(fencedStarted)
			<-fencedRelease
			return map[string][]string{"value": {"old"}}, nil
		})
	}()
	<-fencedStarted
	fencedCache.Evict("evicted-key")
	close(fencedRelease)
	assert.Eventually(t, func() bool { return fencedCache.Len() == 0 }, time.Second, time.Millisecond)
	assert.Equal(t, 0, fencedCache.Len())
}

func TestCacheWaiterRetriesAfterFirstCallerCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "context error", err: context.Canceled},
		{name: "unwrapped source error", err: errors.New("query interrupted")},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			cache := testCache(&now, DefaultConfig())
			firstCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			firstDone := make(chan error, 1)
			go func() {
				_, _, err := cache.Load(firstCtx, "key", LoadOptions{}, func(ctx context.Context) (map[string][]string, error) {
					close(started)
					<-ctx.Done()
					return nil, test.err
				})
				firstDone <- err
			}()
			<-started

			waiterCtx := &signallingContext{Context: context.Background(), entered: make(chan struct{})}
			loads := 0
			type outcome struct {
				value map[string][]string
				err   error
			}
			secondDone := make(chan outcome, 1)
			go func() {
				value, _, err := cache.Load(waiterCtx, "key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
					loads++
					return map[string][]string{"value": {"fresh"}}, nil
				})
				secondDone <- outcome{value: value, err: err}
			}()
			<-waiterCtx.entered
			cancel()
			require.ErrorIs(t, <-firstDone, test.err)
			result := <-secondDone
			require.NoError(t, result.err)
			assert.Equal(t, []string{"fresh"}, result.value["value"])
			assert.Equal(t, 1, loads)
			assert.Equal(t, 1, cache.Len())
		})
	}
}

func TestCacheFailedForcedReplacementRetainsGenerationFence(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	ordinaryStarted := make(chan struct{})
	ordinaryRelease := make(chan struct{})
	ordinaryDone := make(chan struct{})

	go func() {
		_, _, _ = cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
			close(ordinaryStarted)
			<-ordinaryRelease
			return map[string][]string{"value": {"stale"}}, nil
		})
		close(ordinaryDone)
	}()
	<-ordinaryStarted

	_, _, err := cache.Load(context.Background(), "key", LoadOptions{Bypass: true}, func(context.Context) (map[string][]string, error) {
		return nil, errors.New("forced failure")
	})
	require.EqualError(t, err, "forced failure")

	close(ordinaryRelease)
	<-ordinaryDone
	assert.Equal(t, 0, cache.Len())

	loads := 0
	value, status, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {"fresh"}}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, StatusLoaded, status)
	assert.Equal(t, []string{"fresh"}, value["value"])
	assert.Equal(t, 1, loads)
}

func TestCachePurgeFencesPendingLoads(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _, _ = cache.Load(context.Background(), "user-1|key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
			close(started)
			<-release
			return map[string][]string{"value": {"old"}}, nil
		})
	}()
	<-started
	PurgePrefix(cache, "user-1|")
	close(release)
	assert.Eventually(t, func() bool { return cache.Len() == 0 }, time.Second, time.Millisecond)
}

func TestCachePurgePrefixPreservesOtherUsers(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	for _, key := range []string{"user-1|first", "user-1|second", "user-10|first"} {
		_, _, err := cache.Load(context.Background(), key, LoadOptions{}, func(context.Context) (map[string][]string, error) {
			return map[string][]string{"value": {key}}, nil
		})
		require.NoError(t, err)
	}

	PurgePrefix(cache, "user-1|")
	assert.Equal(t, 1, cache.Len())
	loads := 0
	value, status, err := cache.Load(context.Background(), "user-10|first", LoadOptions{}, func(context.Context) (map[string][]string, error) {
		loads++
		return nil, nil
	})
	require.NoError(t, err)
	assert.Equal(t, StatusHit, status)
	assert.Equal(t, []string{"user-10|first"}, value["value"])
	assert.Zero(t, loads)
}

func TestCacheEvictionAllowsReloadAfterManyUniqueKeys(t *testing.T) {
	cache := New[string, string](DefaultConfig(), time.Now, nil, nil)
	for i := 0; i < 1000; i++ {
		key := "evicted-" + strconv.Itoa(i)
		cache.Evict(key)
		value, status, err := cache.Load(context.Background(), key, LoadOptions{}, func(context.Context) (string, error) {
			return "value", nil
		})
		require.NoError(t, err)
		require.Equal(t, StatusLoaded, status)
		require.Equal(t, "value", value)
		cache.Evict(key)
	}
}

func TestCacheLifecycleObserverReportsCapacityAndEviction(t *testing.T) {
	now := time.Now()
	var events []Event
	config := DefaultConfig()
	config.MaxEntries = 1
	config.Observer = func(event Event) { events = append(events, event) }
	cache := testCache(&now, config)
	loader := func(value string) func(context.Context) (map[string][]string, error) {
		return func(context.Context) (map[string][]string, error) {
			return map[string][]string{"value": {value}}, nil
		}
	}

	_, _, err := cache.Load(context.Background(), "first", LoadOptions{}, loader("first"))
	require.NoError(t, err)
	_, _, err = cache.Load(context.Background(), "second", LoadOptions{}, loader("second"))
	require.NoError(t, err)
	assert.Contains(t, events, EventEviction)

	config.MaxEntryBytes = 1
	oversized := testCache(&now, config)
	_, _, err = oversized.Load(context.Background(), "large", LoadOptions{}, func(context.Context) (map[string][]string, error) {
		return map[string][]string{"value": {"too-large", "still-large"}}, nil
	})
	require.NoError(t, err)
	assert.Contains(t, events, EventCapacityBypass)
}

func TestCacheObserverCanReenterAfterLoad(t *testing.T) {
	now := time.Now()
	config := DefaultConfig()
	config.MaxEntryBytes = 1
	var cache *Cache[string, map[string][]string]
	config.Observer = func(Event) { _ = cache.Len() }
	cache = testCache(&now, config)

	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (map[string][]string, error) {
		return map[string][]string{"value": {"one", "two"}}, nil
	})
	require.NoError(t, err)
}

func TestCacheSingleFlightReportsJoinStatus(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	started := make(chan struct{})
	release := make(chan struct{})
	first := make(chan LoadStatus, 1)
	second := make(chan LoadStatus, 1)
	loader := func(context.Context) (map[string][]string, error) {
		close(started)
		<-release
		return map[string][]string{"value": {"shared"}}, nil
	}
	go func() {
		_, status, _ := cache.Load(context.Background(), "shared", LoadOptions{}, loader)
		first <- status
	}()
	<-started
	secondContext := &signallingContext{Context: context.Background(), entered: make(chan struct{})}
	go func() {
		_, status, _ := cache.Load(secondContext, "shared", LoadOptions{}, loader)
		second <- status
	}()
	<-secondContext.entered
	close(release)
	assert.Equal(t, StatusLoaded, <-first)
	assert.Equal(t, StatusSingleFlightJoin, <-second)
}

func TestCachePeekExpiredEntryReportsEviction(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var events []Event
	config := DefaultConfig()
	config.Observer = func(event Event) { events = append(events, event) }
	cache := New[string, string](config, func() time.Time { return now }, nil, nil)

	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) { return "value", nil })
	require.NoError(t, err)
	now = now.Add(config.MaxAge)
	_, _, ok := cache.Peek("key")
	require.False(t, ok)
	require.Contains(t, events, EventEviction)
}

func TestCacheBypassFailureRetainsPriorValue(t *testing.T) {
	cache := New[string, string](DefaultConfig(), time.Now, nil, nil)
	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) { return "old", nil })
	require.NoError(t, err)

	_, _, err = cache.Load(context.Background(), "key", LoadOptions{Bypass: true}, func(context.Context) (string, error) {
		return "", errors.New("forced failure")
	})
	require.EqualError(t, err, "forced failure")
	value, status, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) { return "unexpected", nil })
	require.NoError(t, err)
	require.Equal(t, StatusHit, status)
	require.Equal(t, "old", value)
}

func TestCachePeekAndRefreshRequireCurrentGeneration(t *testing.T) {
	cache := New[string, string](DefaultConfig(), time.Now, nil, func(value string) (int64, bool) { return int64(len(value)), true })
	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) { return "old", nil })
	require.NoError(t, err)

	value, generation, ok := cache.Peek("key")
	require.True(t, ok)
	require.Equal(t, "old", value)
	require.True(t, cache.Refresh("key", "refreshed", generation))
	value, nextGeneration, ok := cache.Peek("key")
	require.True(t, ok)
	require.Equal(t, "refreshed", value)
	require.Equal(t, generation, nextGeneration)

	cache.Evict("key")
	require.False(t, cache.Refresh("key", "stale", generation))
}

func TestCacheBypassAndDisabledReadSourceDirectly(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	loads := 0
	loader := func(context.Context) (map[string][]string, error) {
		loads++
		return map[string][]string{"value": {"source"}}, nil
	}

	_, status, err := cache.Load(context.Background(), "key", LoadOptions{Bypass: true}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusBypassed, status)
	assert.Equal(t, 1, cache.Len())
	ordinary, status, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusHit, status)
	assert.Equal(t, map[string][]string{"value": {"source"}}, ordinary)

	disabled := DefaultConfig()
	disabled.Enabled = false
	disabledCache := testCache(&now, disabled)
	_, status, err = disabledCache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, status)
	assert.Equal(t, 2, loads)
	assert.Equal(t, 0, disabledCache.Len())
}
