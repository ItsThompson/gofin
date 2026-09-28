package cache

import (
	"container/list"
	"context"
	"time"
)

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

func (c *Cache[K, V]) canStoreLocked(sizeBytes int64, cacheable bool) bool {
	return c.config.MaxEntries > 0 && c.config.MaxBytes > 0 && c.config.MaxAge > 0 && cacheable && sizeBytes >= 0 && sizeBytes <= c.maxEntryBytes()
}

func (c *Cache[K, V]) maxEntryBytes() int64 {
	if c.config.MaxEntryBytes > 0 && c.config.MaxEntryBytes < c.config.MaxBytes {
		return c.config.MaxEntryBytes
	}
	return c.config.MaxBytes
}

func (c *Cache[K, V]) storeLocked(key K, value V, sizeBytes int64, expiresAt time.Time, generation uint64, events *[]Event) {
	if element, ok := c.entries[key]; ok {
		c.removeElementLocked(element)
	}
	for c.lru.Len() >= c.config.MaxEntries || c.bytes+sizeBytes > c.config.MaxBytes {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.removeElementLocked(oldest)
		c.appendEvent(events, EventEviction)
	}
	cached := &entry[K, V]{key: key, value: value, expiresAt: expiresAt, sizeBytes: sizeBytes, generation: generation}
	c.generations[key] = generation
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

func (c *Cache[K, V]) appendEvent(events *[]Event, event Event) {
	if events != nil {
		*events = append(*events, event)
	}
}

func (c *Cache[K, V]) emit(events []Event) {
	if c.config.Observer == nil {
		return
	}
	for _, event := range events {
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
