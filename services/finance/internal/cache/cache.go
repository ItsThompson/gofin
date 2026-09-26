// Package cache provides bounded, process-local caching for finance results.
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type Config struct {
	Enabled       bool
	MaxEntries    int
	MaxBytes      int64
	MaxEntryBytes int64
	MaxAge        time.Duration
}

func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		MaxEntries:    256,
		MaxBytes:      64 * 1024 * 1024,
		MaxEntryBytes: 16 * 1024 * 1024,
		MaxAge:        48 * time.Hour,
	}
}

type LoadStatus string

const (
	StatusHit      LoadStatus = "hit"
	StatusLoaded   LoadStatus = "miss"
	StatusBypassed LoadStatus = "bypass"
	StatusDisabled LoadStatus = "disabled"
	StatusOversize LoadStatus = "oversize"
	StatusError    LoadStatus = "error"
)

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
	done       chan struct{}
	value      V
	status     LoadStatus
	err        error
	detached   bool
}

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

func (c *Cache[K, V]) Load(ctx context.Context, key K, options LoadOptions, loader func(context.Context) (V, error)) (V, LoadStatus, error) {
	if options.Bypass {
		value, err := loader(ctx)
		if err != nil {
			var zero V
			return zero, StatusError, err
		}
		return c.clone(value), StatusBypassed, nil
	}

	c.mu.Lock()
	if !c.config.Enabled {
		c.mu.Unlock()
		value, err := loader(ctx)
		if err != nil {
			var zero V
			return zero, StatusError, err
		}
		return c.clone(value), StatusDisabled, nil
	}
	if element, ok := c.entries[key]; ok {
		cached := element.Value.(*entry[K, V])
		if c.now().Before(cached.expiresAt) {
			c.lru.MoveToFront(element)
			value := c.clone(cached.value)
			c.mu.Unlock()
			return value, StatusHit, nil
		}
		c.removeElementLocked(element)
	}
	if current, ok := c.flights[key]; ok {
		c.mu.Unlock()
		return c.waitForFlight(ctx, current)
	}

	generation := c.generations[key]
	current := &flight[V]{generation: generation, done: make(chan struct{})}
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
	if c.canStoreLocked(current.value) && c.generations[key] == generation {
		c.storeLocked(key, current.value, c.now().Add(c.config.MaxAge), generation)
	}
	if c.flights[key] == current {
		delete(c.flights, key)
	}
	c.completeDetachedFlightLocked(key, current)
	close(current.done)
	c.cleanupGenerationLocked(key)
	result := c.clone(current.value)
	status := current.status
	if sizeBytes, cacheable := c.size(current.value); !cacheable || sizeBytes > c.maxEntryBytes() {
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
		return c.clone(current.value), current.status, nil
	case <-ctx.Done():
		var zero V
		return zero, StatusError, ctx.Err()
	}
}

// Peek returns a retained value and its generation without running a loader.
func (c *Cache[K, V]) Peek(key K) (V, uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.config.Enabled {
		var zero V
		return zero, 0, false
	}
	element, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, 0, false
	}
	cached := element.Value.(*entry[K, V])
	if !c.now().Before(cached.expiresAt) {
		c.removeElementLocked(element)
		var zero V
		return zero, 0, false
	}
	c.lru.MoveToFront(element)
	return c.clone(cached.value), cached.generation, true
}

// Refresh replaces a hit only when its generation is unchanged. It never calls
// external code while holding the cache lock and cannot resurrect an evicted key.
func (c *Cache[K, V]) Refresh(key K, value V, generation uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[key]
	if !ok || c.generations[key] != generation || element.Value.(*entry[K, V]).generation != generation {
		return false
	}
	if !c.canStoreLocked(value) {
		return false
	}
	c.removeElementLocked(element)
	c.storeLocked(key, value, c.now().Add(c.config.MaxAge), generation)
	return true
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

func (c *Cache[K, V]) storeLocked(key K, value V, expiresAt time.Time, generation uint64) {
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
	}
	cached := &entry[K, V]{key: key, value: c.clone(value), expiresAt: expiresAt, sizeBytes: sizeBytes, generation: generation}
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

func (c *Cache[K, V]) removeElementLocked(element *list.Element) {
	cached := element.Value.(*entry[K, V])
	delete(c.entries, cached.key)
	c.lru.Remove(element)
	c.bytes -= cached.sizeBytes
	if _, hadFlight := c.flights[cached.key]; !hadFlight {
		c.cleanupGenerationLocked(cached.key)
	}
}

func (c *Cache[K, V]) Evict(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.removeElementLocked(element)
	}
	c.generations[key]++
	c.detachFlightLocked(key)
}

func (c *Cache[K, V]) Purge(match func(K) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, element := range c.entries {
		if match(key) {
			c.removeElementLocked(element)
			c.generations[key]++
		}
	}
	for key := range c.flights {
		if match(key) {
			c.generations[key]++
			c.detachFlightLocked(key)
		}
	}
}

func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
