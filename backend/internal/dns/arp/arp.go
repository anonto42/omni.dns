// Package arp resolves client IP addresses to MAC addresses using a cached
// snapshot of the system ARP table. The snapshot is refreshed on a background
// interval so that DNS query handling never performs a per-query syscall.
package arp

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Cache holds a periodically refreshed IP -> MAC mapping.
type Cache struct {
	enabled bool
	refresh time.Duration

	mu    sync.RWMutex
	table map[string]string
}

// NewCache builds an ARP cache. When enabled is false (or the platform has no
// ARP table), Lookup always returns an empty string and no goroutine is started.
func NewCache(enabled bool, refresh time.Duration) *Cache {
	if refresh <= 0 {
		refresh = 30 * time.Second
	}
	return &Cache{
		enabled: enabled,
		refresh: refresh,
		table:   make(map[string]string),
	}
}

// Start performs an initial load and then refreshes the table until ctx is
// cancelled. It is a no-op when the cache is disabled.
func (c *Cache) Start(ctx context.Context) {
	if !c.enabled {
		return
	}
	c.reload()

	go func() {
		ticker := time.NewTicker(c.refresh)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.reload()
			}
		}
	}()
}

// Lookup returns the cached MAC address for an IP, or "" if unknown/disabled.
func (c *Cache) Lookup(ip string) string {
	if !c.enabled {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.table[ip]
}

func (c *Cache) reload() {
	table, err := readARPTable()
	if err != nil {
		slog.Debug("arp table refresh failed", "error", err)
		return
	}
	c.mu.Lock()
	c.table = table
	c.mu.Unlock()
}
