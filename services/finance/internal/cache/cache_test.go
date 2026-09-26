package cache

import (
	"context"
	"errors"
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
