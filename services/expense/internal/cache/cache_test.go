package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCache(now *time.Time, config Config) *Cache[string, map[string][]string] {
	return New[string, map[string][]string](config, func() time.Time { return *now }, cloneMap, func(value map[string][]string) int64 {
		return int64(len(value["value"]))
	})
}

func cloneMap(value map[string][]string) map[string][]string {
	copyValue := make(map[string][]string, len(value))
	for key, values := range value {
		copyValue[key] = append([]string(nil), values...)
	}
	return copyValue
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

func TestCacheSingleFlightAndEvictionFence(t *testing.T) {
	now := time.Now()
	cache := testCache(&now, DefaultConfig())
	var lock sync.Mutex
	loads := 0
	loader := func(context.Context) (map[string][]string, error) {
		lock.Lock()
		loads++
		lock.Unlock()
		return map[string][]string{"value": {"same"}}, nil
	}

	start := make(chan struct{})
	results := make(chan map[string][]string, 2)
	for range 2 {
		go func() {
			<-start
			value, _, _ := cache.Load(context.Background(), "same-key", LoadOptions{}, loader)
			results <- value
		}()
	}
	close(start)
	assert.Equal(t, "same", (<-results)["value"][0])
	assert.Equal(t, "same", (<-results)["value"][0])
	assert.Equal(t, 1, loads)

	fencedCache := testCache(&now, DefaultConfig())
	started := make(chan struct{})
	release := make(chan struct{})
	loader = func(context.Context) (map[string][]string, error) {
		lock.Lock()
		loads++
		lock.Unlock()
		close(started)
		<-release
		return map[string][]string{"value": {"old"}}, nil
	}
	go func() {
		_, _, _ = fencedCache.Load(context.Background(), "evicted-key", LoadOptions{}, loader)
	}()
	<-started
	fencedCache.Evict("evicted-key")
	close(release)
	assert.Eventually(t, func() bool { return fencedCache.Len() == 0 }, time.Second, time.Millisecond)
	assert.Equal(t, 0, fencedCache.Len())
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
	cache.Purge(func(key string) bool { return key == "user-1|key" })
	close(release)
	assert.Eventually(t, func() bool { return cache.Len() == 0 }, time.Second, time.Millisecond)
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
	assert.Equal(t, 0, cache.Len())

	disabled := DefaultConfig()
	disabled.Enabled = false
	disabledCache := testCache(&now, disabled)
	_, status, err = disabledCache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, status)
	assert.Equal(t, 2, loads)
	assert.Equal(t, 0, disabledCache.Len())
}
