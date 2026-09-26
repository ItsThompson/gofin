// Package cache provides bounded, process-local caching for expense reads.
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Config controls storage bounds and expiration. A disabled cache still runs
// the loader directly and never retains a value.
type Config struct {
	Enabled       bool
	MaxEntries    int
	MaxBytes      int64
	MaxEntryBytes int64
	MaxAge        time.Duration
	Observer      func(Event)
}

// Event identifies cache lifecycle events that callers can expose through
// bounded metrics.
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

// LoadStatus identifies how a Load call obtained its value.
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
// successful source result when the cache generation remains current.
type LoadOptions struct {
	Bypass bool
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	sizeBytes int64
}

type flight[V any] struct {
	generation uint64
	bypass     bool
	done       chan struct{}
	value      V
	status     LoadStatus
	err        error
	detached   bool
}

// Cache is a typed, bounded LRU cache. clone must produce a deep copy of a
// value because values commonly contain slices or pointers to domain records.
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

// New constructs a cache. It does not create any external resources.
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
// reads of the same key. The loader runs without the cache lock held.
func (c *Cache[K, V]) Load(ctx context.Context, key K, options LoadOptions, loader func(context.Context) (V, error)) (V, LoadStatus, error) {
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
			return c.waitForFlight(ctx, current)
		}
		c.generations[key]++
		c.detachFlightLocked(key)
	}

	if !options.Bypass {
		now := c.now()
		if element, ok := c.entries[key]; ok {
			cached := element.Value.(*entry[K, V])
			if now.Before(cached.expiresAt) {
				c.lru.MoveToFront(element)
				value := c.clone(cached.value)
				c.mu.Unlock()
				return value, StatusHit, nil
			}
			c.removeElementLocked(element)
			c.observe(EventEviction)
		}
	}

	generation := c.generations[key]
	current := &flight[V]{generation: generation, bypass: options.Bypass, done: make(chan struct{})}
	c.flights[key] = current
	c.mu.Unlock()

	value, err := loader(ctx)
	c.mu.Lock()
	if err != nil {
		current.status = StatusError
		current.err = err
		if c.flights[key] == current {
			delete(c.flights, key)
		}
		c.completeDetachedFlightLocked(key, current)
		close(current.done)
		c.cleanupGenerationLocked(key)
		c.mu.Unlock()
		var zero V
		return zero, StatusError, err
	}

	current.value = c.clone(value)
	current.status = StatusLoaded
	if c.generations[key] == generation {
		if c.canStoreLocked(current.value) {
			c.storeLocked(key, current.value, c.now().Add(c.config.MaxAge))
		} else {
			c.observe(EventCapacityBypass)
			if current.bypass {
				if element, ok := c.entries[key]; ok {
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
	result := c.clone(current.value)
	status := current.status
	if sizeBytes, cacheable := c.size(current.value); !cacheable {
		status = StatusUncacheable
	} else if sizeBytes > c.maxEntryBytes() {
		status = StatusOversize
	}
	c.mu.Unlock()
	return result, status, nil
}

func (c *Cache[K, V]) waitForFlight(ctx context.Context, current *flight[V]) (V, LoadStatus, error) {
	select {
	case <-current.done:
		if current.err != nil {
			var zero V
			return zero, StatusError, current.err
		}
		return c.clone(current.value), StatusSingleFlightJoin, nil
	case <-ctx.Done():
		var zero V
		return zero, StatusError, ctx.Err()
	}
}

func (c *Cache[K, V]) canStoreLocked(value V) bool {
	if c.config.MaxEntries <= 0 || c.config.MaxBytes <= 0 || c.config.MaxAge <= 0 {
		return false
	}
	sizeBytes, cacheable := c.size(value)
	return cacheable && sizeBytes >= 0 && sizeBytes <= c.maxEntryBytes()
}

func (c *Cache[K, V]) maxEntryBytes() int64 {
	if c.config.MaxEntryBytes > 0 && c.config.MaxEntryBytes < c.config.MaxBytes {
		return c.config.MaxEntryBytes
	}
	return c.config.MaxBytes
}

func (c *Cache[K, V]) storeLocked(key K, value V, expiresAt time.Time) {
	sizeBytes, cacheable := c.size(value)
	if !cacheable || sizeBytes < 0 {
		return
	}
	if element, ok := c.entries[key]; ok {
		c.removeElementLocked(element)
	}
	for c.lru.Len() >= c.config.MaxEntries || c.bytes+sizeBytes > c.config.MaxBytes {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.removeElementLocked(oldest)
		c.observe(EventEviction)
	}
	cached := &entry[K, V]{key: key, value: c.clone(value), expiresAt: expiresAt, sizeBytes: sizeBytes}
	c.entries[key] = c.lru.PushFront(cached)
	c.bytes += sizeBytes
}

func (c *Cache[K, V]) cleanupGenerationLocked(key K) {
	if _, hasEntry := c.entries[key]; hasEntry {
		return
	}
	if _, hasFlight := c.flights[key]; hasFlight {
		return
	}
	if c.detachedFlights[key] > 0 {
		return
	}
	delete(c.generations, key)
}

func (c *Cache[K, V]) completeDetachedFlightLocked(key K, current *flight[V]) {
	if !current.detached {
		return
	}
	c.detachedFlights[key]--
	if c.detachedFlights[key] == 0 {
		delete(c.detachedFlights, key)
	}
}

func (c *Cache[K, V]) detachFlightLocked(key K) {
	current, ok := c.flights[key]
	if !ok {
		return
	}
	current.detached = true
	c.detachedFlights[key]++
	delete(c.flights, key)
}

func (c *Cache[K, V]) observe(event Event) {
	if c.config.Observer != nil {
		c.config.Observer(event)
	}
}

func (c *Cache[K, V]) removeElementLocked(element *list.Element) {
	cached := element.Value.(*entry[K, V])
	delete(c.entries, cached.key)
	c.lru.Remove(element)
	c.bytes -= cached.sizeBytes
	if _, hadFlight := c.flights[cached.key]; !hadFlight {
		c.cleanupGenerationLocked(cached.key)
	}
}

// Evict removes one key and fences an in-flight load from storing its result.
func (c *Cache[K, V]) Evict(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.removeElementLocked(element)
		c.observe(EventEviction)
	}
	_, hadFlight := c.flights[key]
	if hadFlight || c.detachedFlights[key] > 0 {
		c.generations[key]++
		c.detachFlightLocked(key)
		return
	}
	delete(c.generations, key)
}

// Purge removes all keys for which match returns true and fences their loads.
func (c *Cache[K, V]) Purge(match func(K) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, element := range c.entries {
		if match(key) {
			c.removeElementLocked(element)
			c.observe(EventEviction)
			if _, hasFlight := c.flights[key]; hasFlight || c.detachedFlights[key] > 0 {
				c.generations[key]++
			}
		}
	}
	for key := range c.flights {
		if match(key) {
			c.generations[key]++
			c.detachFlightLocked(key)
		}
	}
}

// Len returns the number of retained entries. It is intended for diagnostics
// and tests, not for cache policy decisions.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Bytes returns retained value size. It is intended for diagnostics and tests.
func (c *Cache[K, V]) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}
