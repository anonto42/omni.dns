package arp

import (
	"context"
	"testing"
	"time"
)

func TestDisabledCacheReturnsEmpty(t *testing.T) {
	c := NewCache(false, time.Second)
	c.Start(context.Background()) // no-op when disabled
	if got := c.Lookup("192.168.1.1"); got != "" {
		t.Fatalf("disabled cache should return empty MAC, got %q", got)
	}
}

func TestEnabledCacheLoadsTable(t *testing.T) {
	c := NewCache(true, time.Hour)
	c.Start(context.Background()) // performs an initial synchronous reload

	// We cannot assert a specific MAC (platform/host dependent), but an unknown
	// IP must always resolve to empty without panicking.
	if got := c.Lookup("0.0.0.0"); got != "" {
		t.Fatalf("unknown IP should return empty MAC, got %q", got)
	}
}

func TestManualTableLookup(t *testing.T) {
	c := NewCache(true, time.Hour)
	c.mu.Lock()
	c.table = map[string]string{"10.0.0.5": "aa:bb:cc:dd:ee:ff"}
	c.mu.Unlock()

	if got := c.Lookup("10.0.0.5"); got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected cached MAC, got %q", got)
	}
	if got := c.Lookup("10.0.0.6"); got != "" {
		t.Fatalf("expected empty for unknown IP, got %q", got)
	}
}
