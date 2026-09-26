package cache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testConfig() Config {
	return Config{Enabled: true, MaxEntries: 4, MaxBytes: 1024, MaxEntryBytes: 1024, MaxAge: time.Hour}
}

func TestCacheCoalescesLoadsAndReturnsCopy(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache := New[string, *[]string](testConfig(), func() time.Time { return now }, func(value *[]string) *[]string {
		if value == nil {
			return nil
		}
		copyValue := append([]string(nil), (*value)...)
		return &copyValue
	}, func(value *[]string) (int64, bool) { return int64(len(*value)), true })

	var calls int
	var gate sync.WaitGroup
	gate.Add(1)
	loader := func(context.Context) (*[]string, error) {
		calls++
		gate.Wait()
		value := []string{"source"}
		return &value, nil
	}

	firstDone := make(chan struct{})
	var first *[]string
	started := make(chan struct{})
	loaderWithSignal := func(ctx context.Context) (*[]string, error) {
		close(started)
		return loader(ctx)
	}
	go func() {
		first, _, _ = cache.Load(context.Background(), "user|summary", LoadOptions{}, loaderWithSignal)
		close(firstDone)
	}()
	<-started
	secondDone := make(chan struct{})
	var second *[]string
	go func() {
		second, _, _ = cache.Load(context.Background(), "user|summary", LoadOptions{}, loaderWithSignal)
		close(secondDone)
	}()
	gate.Done()
	<-firstDone
	<-secondDone

	require.Equal(t, 1, calls)
	require.Equal(t, []string{"source"}, *first)
	(*first)[0] = "caller mutation"
	require.Equal(t, []string{"source"}, *second)
}

func TestCacheGenerationMetadataStaysBoundedAfterUniqueEvictions(t *testing.T) {
	cache := New[string, string](testConfig(), time.Now, nil, nil)
	for i := 0; i < 1000; i++ {
		cache.Evict("evicted-" + strconv.Itoa(i))
	}
	require.Empty(t, cache.generations)

	for i := 0; i < 1000; i++ {
		_, _, err := cache.Load(context.Background(), "loaded-"+strconv.Itoa(i), LoadOptions{}, func(context.Context) (string, error) {
			return "value", nil
		})
		require.NoError(t, err)
	}
	cache.Purge(func(string) bool { return true })
	require.Empty(t, cache.generations)
}

func TestCacheObserverReportsCapacityBypassAndEviction(t *testing.T) {
	var events []Event
	config := testConfig()
	config.MaxEntries = 1
	config.MaxEntryBytes = 2
	config.Observer = func(event Event) { events = append(events, event) }
	cache := New[string, string](config, time.Now, nil, func(value string) (int64, bool) { return int64(len(value)), true })

	_, _, err := cache.Load(context.Background(), "first", LoadOptions{}, func(context.Context) (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	_, _, err = cache.Load(context.Background(), "second", LoadOptions{}, func(context.Context) (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	require.Contains(t, events, EventEviction)

	_, status, err := cache.Load(context.Background(), "oversized", LoadOptions{}, func(context.Context) (string, error) {
		return "too-large", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusOversize, status)
	require.Contains(t, events, EventCapacityBypass)
}

func TestCachePeekExpiredEntryReportsEviction(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var events []Event
	config := testConfig()
	config.MaxAge = time.Hour
	config.Observer = func(event Event) { events = append(events, event) }
	cache := New[string, string](config, func() time.Time { return now }, nil, nil)

	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "value", nil
	})
	require.NoError(t, err)
	now = now.Add(time.Hour)
	_, _, ok := cache.Peek("key")
	require.False(t, ok)
	require.Contains(t, events, EventEviction)
}

func TestCacheBypassReplacesValueAndKeepsPriorValueOnFailure(t *testing.T) {
	cache := New[string, string](testConfig(), time.Now, nil, nil)

	value, status, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "old", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusLoaded, status)
	require.Equal(t, "old", value)

	value, status, err = cache.Load(context.Background(), "key", LoadOptions{Bypass: true}, func(context.Context) (string, error) {
		return "fresh", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusBypassed, status)
	require.Equal(t, "fresh", value)

	value, status, err = cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "unexpected", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusHit, status)
	require.Equal(t, "fresh", value)

	_, _, err = cache.Load(context.Background(), "key", LoadOptions{Bypass: true}, func(context.Context) (string, error) {
		return "", errors.New("forced failure")
	})
	require.EqualError(t, err, "forced failure")
	value, status, err = cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "unexpected", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusHit, status)
	require.Equal(t, "fresh", value)
}

func TestCacheEvictionRetainsGenerationAcrossFailedReplacement(t *testing.T) {
	cache := New[string, string](testConfig(), time.Now, nil, nil)
	oldStarted := make(chan struct{})
	oldRelease := make(chan struct{})
	oldDone := make(chan struct{})
	go func() {
		_, _, _ = cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
			close(oldStarted)
			<-oldRelease
			return "old", nil
		})
		close(oldDone)
	}()
	<-oldStarted
	cache.Evict("key")

	_, _, replacementErr := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "", errors.New("replacement failed")
	})
	require.EqualError(t, replacementErr, "replacement failed")

	close(oldRelease)
	<-oldDone
	require.Equal(t, 0, cache.Len())

	fresh, status, err := cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
		return "fresh", nil
	})
	require.NoError(t, err)
	require.Equal(t, StatusLoaded, status)
	require.Equal(t, "fresh", fresh)
}

func TestCacheEvictionFencesAnInFlightLoad(t *testing.T) {
	cache := New[string, string](testConfig(), time.Now, nil, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	loaded := make(chan struct{})
	go func() {
		_, _, _ = cache.Load(context.Background(), "key", LoadOptions{}, func(context.Context) (string, error) {
			close(started)
			<-release
			close(loaded)
			return "old", nil
		})
	}()
	<-started
	cache.Evict("key")
	close(release)
	<-loaded
	require.Equal(t, 0, cache.Len())
}

func TestCacheExpiresEntriesAtConfiguredTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache := New[string, string](testConfig(), func() time.Time { return now }, nil, nil)
	calls := 0
	loader := func(context.Context) (string, error) {
		calls++
		return "value", nil
	}

	_, status, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	require.Equal(t, StatusLoaded, status)
	now = now.Add(time.Hour)
	_, status, err = cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	require.Equal(t, StatusLoaded, status)
	require.Equal(t, 2, calls)
}

func TestCacheDoesNotRetainErrorsOrOversizedValues(t *testing.T) {
	config := testConfig()
	config.MaxEntryBytes = 2
	cache := New[string, string](config, time.Now, nil, func(value string) (int64, bool) { return int64(len(value)), true })
	calls := 0
	loader := func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("source failed")
		}
		return "large", nil
	}
	_, _, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.Error(t, err)
	_, status, err := cache.Load(context.Background(), "key", LoadOptions{}, loader)
	require.NoError(t, err)
	require.Equal(t, StatusOversize, status)
	require.Equal(t, 2, calls)
	require.Equal(t, 0, cache.Len())
}
