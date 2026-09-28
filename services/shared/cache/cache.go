// Package cache provides bounded, process-local typed caches.
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Config controls storage bounds and expiration.
type Config struct {
	Enabled       bool
	MaxEntries    int
	MaxBytes      int64
	MaxEntryBytes int64
	MaxAge        time.Duration
	Observer      func(Event)
}

// Event identifies cache lifecycle events.
type Event string

const (
	EventEviction       Event = "eviction"
	EventCapacityBypass Event = "capacity_bypass"
)

// DefaultConfig enables a bounded cache with the rollout defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		MaxEntries:    256,
		MaxBytes:      64 * 1024 * 1024,
		MaxEntryBytes: 16 * 1024 * 1024,
		MaxAge:        48 * time.Hour,
	}
}

// LoadStatus identifies how Load obtained its value.
type LoadStatus string

const (
	StatusHit              LoadStatus = "hit"
	StatusLoaded           LoadStatus = "miss"
	StatusSingleFlightJoin LoadStatus = "single_flight_join"
	StatusBypassed         LoadStatus = "bypass"
	StatusDisabled         LoadStatus = "disabled"
	StatusOversize         LoadStatus = "oversize"
	StatusUncacheable      LoadStatus = "uncacheable"
	StatusError            LoadStatus = "error"
)

// LoadOptions controls one read. Bypass skips retained values but stores a
// successful source result when its cache generation remains current.
type LoadOptions struct {
	Bypass bool
}

type entry[K comparable, V any] struct {
	key        K
	value      V
	expiresAt  time.Time
	sizeBytes  int64
	generation uint64
}

type flight[V any] struct {
	generation uint64
	bypass     bool
	done       chan struct{}
	value      V
	status     LoadStatus
	err        error
	canceled   bool
	detached   bool
}

// Cache is a typed, bounded LRU cache. clone must produce a deep copy when V
// contains mutable slices or pointers.
type Cache[K comparable, V any] struct {
	mu              sync.Mutex
	config          Config
	now             func() time.Time
	clone           func(V) V
	size            func(V) (int64, bool)
	entries         map[K]*list.Element
	lru             *list.List
	flights         map[K]*flight[V]
	generations     map[K]uint64
	detachedFlights map[K]int
	bytes           int64
}

// New constructs a cache without creating external resources.
func New[K comparable, V any](config Config, now func() time.Time, clone func(V) V, size func(V) (int64, bool)) *Cache[K, V] {
	if now == nil {
		now = time.Now
	}
	if clone == nil {
		clone = func(value V) V { return value }
	}
	if size == nil {
		size = func(V) (int64, bool) { return 1, true }
	}
	return &Cache[K, V]{
		config:          config,
		now:             now,
		clone:           clone,
		size:            size,
		entries:         make(map[K]*list.Element),
		lru:             list.New(),
		flights:         make(map[K]*flight[V]),
		generations:     make(map[K]uint64),
		detachedFlights: make(map[K]int),
	}
}

// Load returns a cached value or invokes loader once for concurrent ordinary
// reads of the same key. Loader runs without the cache lock held.
func (c *Cache[K, V]) Load(ctx context.Context, key K, options LoadOptions, loader func(context.Context) (V, error)) (V, LoadStatus, error) {
	var events []Event
retry:
	c.mu.Lock()
	if !c.config.Enabled {
		c.mu.Unlock()
		value, err := loader(ctx)
		if err != nil {
			var zero V
			return zero, StatusError, err
		}
		status := StatusDisabled
		if options.Bypass {
			status = StatusBypassed
		}
		return c.clone(value), status, nil
	}

	if current, ok := c.flights[key]; ok {
		if !options.Bypass || current.bypass {
			c.mu.Unlock()
			result, status, err := c.waitForFlight(ctx, current)
			if err != nil && current.canceled && ctx.Err() == nil {
				goto retry
			}
			return result, status, err
		}
		c.generations[key]++
		c.detachFlightLocked(key)
	}

	if !options.Bypass {
		if element, ok := c.entries[key]; ok {
			cached := element.Value.(*entry[K, V])
			if c.now().Before(cached.expiresAt) {
				c.lru.MoveToFront(element)
				value := cached.value
				c.mu.Unlock()
				return c.clone(value), StatusHit, nil
			}
			c.removeElementLocked(element)
			c.appendEvent(&events, EventEviction)
		}
	}

	generation := c.generations[key]
	current := &flight[V]{generation: generation, bypass: options.Bypass, done: make(chan struct{})}
	c.flights[key] = current
	c.mu.Unlock()

	value, err := loader(ctx)
	var loaded V
	var sizeBytes int64
	var cacheable bool
	if err == nil {
		loaded = c.clone(value)
		sizeBytes, cacheable = c.size(loaded)
	}

	c.mu.Lock()
	if err != nil {
		current.status = StatusError
		current.err = err
		current.canceled = ctx.Err() != nil
		if c.flights[key] == current {
			delete(c.flights, key)
		}
		c.completeDetachedFlightLocked(key, current)
		close(current.done)
		c.cleanupGenerationLocked(key)
		c.mu.Unlock()
		c.emit(events)
		var zero V
		return zero, StatusError, err
	}

	current.value = loaded
	current.status = StatusLoaded
	if c.generations[key] == generation {
		if c.canStoreLocked(sizeBytes, cacheable) {
			c.storeLocked(key, loaded, sizeBytes, c.now().Add(c.config.MaxAge), generation, &events)
		} else {
			c.appendEvent(&events, EventCapacityBypass)
			if current.bypass {
				if element, ok := c.entries[key]; ok && element.Value.(*entry[K, V]).generation == generation {
					c.removeElementLocked(element)
				}
			}
		}
	}
	if current.bypass {
		current.status = StatusBypassed
	}
	if c.flights[key] == current {
		delete(c.flights, key)
	}
	c.completeDetachedFlightLocked(key, current)
	close(current.done)
	c.cleanupGenerationLocked(key)
	status := current.status
	if !cacheable {
		status = StatusUncacheable
	} else if sizeBytes > c.maxEntryBytes() {
		status = StatusOversize
	}
	c.mu.Unlock()
	c.emit(events)
	return c.clone(loaded), status, nil
}
