// Package cache implements a bounded, TTL-aware LRU cache for DNS answers.
// Entries are keyed by (domain, query type) so that A and AAAA answers for the
// same name are stored independently.
package cache

import (
	"container/list"
	"strconv"
	"sync"
	"time"
)

const (
	defaultNegTTL  = 60
	evictInterval  = 30 * time.Second
	maxEvictPerRun = 100
)

// Entry is a cached DNS answer for a single (domain, qtype) pair.
type Entry struct {
	// IPs holds the resolved addresses (A or AAAA). Empty for NXDOMAIN/NODATA.
	IPs       []string
	TTL       uint32
	ExpiresAt time.Time
	// NXDOMAIN marks a cached negative (name does not exist) response.
	NXDOMAIN bool
	// NoData marks a cached empty (name exists, no records of this type) response.
	NoData bool
}

// First returns the first cached IP, or "" if none.
func (e *Entry) First() string {
	if len(e.IPs) == 0 {
		return ""
	}
	return e.IPs[0]
}

type kv struct {
	key   string
	value *Entry
}

// Cache is a concurrency-safe LRU cache with per-entry TTL expiry.
type Cache struct {
	mu     sync.RWMutex
	items  map[string]*list.Element
	order  *list.List
	max    int
	hits   int64
	misses int64
	negTTL uint32

	stop chan struct{}
}

// New creates a cache holding at most max entries and starts a background
// eviction loop. Call Close to stop the loop.
func New(max int) *Cache {
	if max <= 0 {
		max = 1000
	}
	c := &Cache{
		items:  make(map[string]*list.Element),
		order:  list.New(),
		max:    max,
		negTTL: defaultNegTTL,
		stop:   make(chan struct{}),
	}
	go c.evictLoop()
	return c
}

// Close stops the background eviction loop.
func (c *Cache) Close() {
	close(c.stop)
}

func key(domain string, qtype uint16) string {
	return domain + "|" + strconv.FormatUint(uint64(qtype), 10)
}

// Get returns the cached entry for (domain, qtype), or nil on miss/expiry.
func (c *Cache) Get(domain string, qtype uint16) *Entry {
	k := key(domain, qtype)
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[k]
	if !ok {
		c.misses++
		return nil
	}

	entry := elem.Value.(*kv).value
	if time.Now().After(entry.ExpiresAt) {
		c.removeElement(elem)
		c.misses++
		return nil
	}

	c.order.MoveToFront(elem)
	c.hits++
	return entry
}

// Set stores a positive answer (one or more IPs) for (domain, qtype).
func (c *Cache) Set(domain string, qtype uint16, ips []string, ttl uint32) {
	c.put(key(domain, qtype), &Entry{
		IPs:       ips,
		TTL:       ttl,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}, false)
}

// SetNXDOMAIN caches a negative (name does not exist) response.
func (c *Cache) SetNXDOMAIN(domain string, qtype uint16) {
	c.put(key(domain, qtype), &Entry{
		NXDOMAIN:  true,
		ExpiresAt: time.Now().Add(time.Duration(c.negTTL) * time.Second),
	}, true)
}

// SetNoData caches an empty (name exists, no record of this type) response.
func (c *Cache) SetNoData(domain string, qtype uint16) {
	c.put(key(domain, qtype), &Entry{
		NoData:    true,
		ExpiresAt: time.Now().Add(time.Duration(c.negTTL) * time.Second),
	}, true)
}

// put inserts or updates an entry. When negativeOnly is true the write is
// skipped if a (positive or negative) entry already exists, preserving any
// cached positive answer.
func (c *Cache) put(k string, entry *Entry, negativeOnly bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[k]; ok {
		if negativeOnly {
			return
		}
		c.order.MoveToFront(elem)
		*elem.Value.(*kv).value = *entry
		return
	}

	if c.order.Len() >= c.max {
		if back := c.order.Back(); back != nil {
			c.removeElement(back)
		}
	}
	elem := c.order.PushFront(&kv{key: k, value: entry})
	c.items[k] = elem
}

// removeElement removes an element from both the list and the index.
// Callers must hold the write lock.
func (c *Cache) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	delete(c.items, elem.Value.(*kv).key)
}

// Size returns the number of live entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Hits returns the cumulative cache hit count.
func (c *Cache) Hits() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits
}

// Misses returns the cumulative cache miss count.
func (c *Cache) Misses() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.misses
}

func (c *Cache) evictLoop() {
	ticker := time.NewTicker(evictInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.evictExpired()
		}
	}
}

func (c *Cache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	evicted := 0
	for _, elem := range c.items {
		if now.After(elem.Value.(*kv).value.ExpiresAt) {
			c.removeElement(elem)
			evicted++
			if evicted >= maxEvictPerRun {
				return
			}
		}
	}
}
