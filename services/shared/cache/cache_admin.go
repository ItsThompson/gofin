package cache

import "strings"

// Peek returns a retained value and its generation without running a loader.
func (c *Cache[K, V]) Peek(key K) (V, uint64, bool) {
	var events []Event
	c.mu.Lock()
	if !c.config.Enabled {
		c.mu.Unlock()
		var zero V
		return zero, 0, false
	}
	element, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		var zero V
		return zero, 0, false
	}
	cached := element.Value.(*entry[K, V])
	if !c.now().Before(cached.expiresAt) {
		c.removeElementLocked(element)
		c.appendEvent(&events, EventEviction)
		c.mu.Unlock()
		c.emit(events)
		var zero V
		return zero, 0, false
	}
	c.lru.MoveToFront(element)
	value := cached.value
	generation := cached.generation
	c.mu.Unlock()
	return c.clone(value), generation, true
}

// Refresh replaces a retained value only when its generation is unchanged.
func (c *Cache[K, V]) Refresh(key K, value V, generation uint64) bool {
	cloned := c.clone(value)
	sizeBytes, cacheable := c.size(cloned)
	var events []Event
	c.mu.Lock()
	element, ok := c.entries[key]
	if !ok || c.generations[key] != generation || element.Value.(*entry[K, V]).generation != generation || !c.canStoreLocked(sizeBytes, cacheable) {
		c.mu.Unlock()
		return false
	}
	c.removeElementLocked(element)
	c.storeLocked(key, cloned, sizeBytes, c.now().Add(c.config.MaxAge), generation, &events)
	c.mu.Unlock()
	c.emit(events)
	return true
}

// Evict removes one key and fences an in-flight load from storing its result.
func (c *Cache[K, V]) Evict(key K) {
	var events []Event
	c.mu.Lock()
	if element, ok := c.entries[key]; ok {
		c.removeElementLocked(element)
		c.appendEvent(&events, EventEviction)
	}
	_, hadFlight := c.flights[key]
	if hadFlight || c.detachedFlights[key] > 0 {
		c.generations[key]++
		c.detachFlightLocked(key)
		c.mu.Unlock()
		c.emit(events)
		return
	}
	delete(c.generations, key)
	c.mu.Unlock()
	c.emit(events)
}

// Purge removes matching keys and fences their in-flight loads.
func (c *Cache[K, V]) Purge(match func(K) bool) {
	var events []Event
	c.mu.Lock()
	for key, element := range c.entries {
		if match(key) {
			c.removeElementLocked(element)
			c.appendEvent(&events, EventEviction)
			if _, hasFlight := c.flights[key]; !hasFlight && c.detachedFlights[key] > 0 {
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
	for key := range c.generations {
		if match(key) {
			c.cleanupGenerationLocked(key)
		}
	}
	c.mu.Unlock()
	c.emit(events)
}

// PurgePrefix removes matching string keys.
func PurgePrefix[V any](c *Cache[string, V], prefix string) {
	c.Purge(func(key string) bool { return strings.HasPrefix(key, prefix) })
}

// Len returns the number of retained entries.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Bytes returns retained value size.
func (c *Cache[K, V]) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}
