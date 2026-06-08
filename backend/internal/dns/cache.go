package dns

import (
	"container/list"
	"sync"
	"time"
)

const (
	defaultNegTTL  = 60
	evictInterval  = 30 * time.Second
	maxEvictPerRun = 100
)

type cacheEntry struct {
	IP        string
	TTL       uint32
	ExpiresAt time.Time
	NXDOMAIN  bool
}

type Cache struct {
	mu        sync.RWMutex
	items     map[string]*list.Element
	order     *list.List
	max       int
	hits      int64
	misses    int64
	negTTL    uint32
}

type kv struct {
	key   string
	value *cacheEntry
}

func NewCache(max int) *Cache {
	c := &Cache{
		items:  make(map[string]*list.Element),
		order:  list.New(),
		max:    max,
		negTTL: defaultNegTTL,
	}
	go c.evictLoop()
	return c
}

func (c *Cache) Get(domain string) *cacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[domain]
	if !ok {
		c.misses++
		return nil
	}

	entry := elem.Value.(*kv).value
	if time.Now().After(entry.ExpiresAt) {
		c.order.Remove(elem)
		delete(c.items, domain)
		c.misses++
		return nil
	}

	c.order.MoveToFront(elem)
	c.hits++
	return entry
}

func (c *Cache) Set(domain, ip string, ttl uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[domain]; ok {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*kv).value
		entry.IP = ip
		entry.TTL = ttl
		entry.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
		entry.NXDOMAIN = false
		return
	}

	if c.order.Len() >= c.max {
		back := c.order.Back()
		if back != nil {
			c.order.Remove(back)
			delete(c.items, back.Value.(*kv).key)
		}
	}

	entry := &cacheEntry{
		IP:        ip,
		TTL:       ttl,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
	elem := c.order.PushFront(&kv{key: domain, value: entry})
	c.items[domain] = elem
}

func (c *Cache) SetNXDOMAIN(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[domain]; ok {
		return
	}

	if c.order.Len() >= c.max {
		back := c.order.Back()
		if back != nil {
			c.order.Remove(back)
			delete(c.items, back.Value.(*kv).key)
		}
	}

	entry := &cacheEntry{
		NXDOMAIN:  true,
		ExpiresAt: time.Now().Add(time.Duration(c.negTTL) * time.Second),
	}
	elem := c.order.PushFront(&kv{key: domain, value: entry})
	c.items[domain] = elem
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

func (c *Cache) Hits() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits
}

func (c *Cache) Misses() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.misses
}

func (c *Cache) evictLoop() {
	ticker := time.NewTicker(evictInterval)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		evicted := 0
		for k, v := range c.items {
			if now.After(v.Value.(*kv).value.ExpiresAt) {
				c.order.Remove(v)
				delete(c.items, k)
				evicted++
				if evicted >= maxEvictPerRun {
					break
				}
			}
		}
		c.mu.Unlock()
	}
}
