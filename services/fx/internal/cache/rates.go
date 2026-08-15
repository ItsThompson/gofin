package cache

import (
	"sync"
	"time"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

type RateCache struct {
	mu       sync.RWMutex
	maxAge   time.Duration
	snapshot *model.ProviderSnapshot
}

func NewRateCache(maxAge time.Duration) *RateCache {
	return &RateCache{maxAge: maxAge}
}

func (c *RateCache) GetFresh(now time.Time) (*model.ProviderSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.snapshot == nil {
		return nil, false
	}
	capturedAt, err := time.Parse(time.RFC3339, c.snapshot.CapturedAt)
	if err != nil {
		return nil, false
	}
	if now.Sub(capturedAt) >= c.maxAge {
		return nil, false
	}
	return cloneProviderSnapshot(c.snapshot), true
}

func (c *RateCache) Store(snapshot model.ProviderSnapshot) model.ProviderSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	stored := cloneProviderSnapshot(&snapshot)
	c.snapshot = stored
	return *cloneProviderSnapshot(stored)
}

func cloneProviderSnapshot(snapshot *model.ProviderSnapshot) *model.ProviderSnapshot {
	if snapshot == nil {
		return nil
	}
	rates := make(map[string]string, len(snapshot.Rates))
	for code, rate := range snapshot.Rates {
		rates[code] = rate
	}
	return &model.ProviderSnapshot{
		Source:        snapshot.Source,
		BaseCurrency:  snapshot.BaseCurrency,
		RateTimestamp: snapshot.RateTimestamp,
		CapturedAt:    snapshot.CapturedAt,
		ExpiresAt:     snapshot.ExpiresAt,
		Rates:         rates,
	}
}
