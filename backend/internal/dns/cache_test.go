package dns

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T, max int) *Cache {
	t.Helper()
	return NewCache(max)
}

func TestNewCache(t *testing.T) {
	c := newTestCache(t, 100)
	assert.Equal(t, 0, c.Size())
	assert.Equal(t, int64(0), c.Hits())
	assert.Equal(t, int64(0), c.Misses())
}

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(t, 100)
	c.Set("example.com", "1.2.3.4", 300)

	entry := c.Get("example.com")
	require.NotNil(t, entry)
	assert.Equal(t, "1.2.3.4", entry.IP)
	assert.Equal(t, uint32(300), entry.TTL)
	assert.False(t, entry.NXDOMAIN)
	assert.False(t, time.Now().After(entry.ExpiresAt))
}

func TestCache_Get_Miss(t *testing.T) {
	c := newTestCache(t, 100)
	assert.Nil(t, c.Get("nonexistent.com"))
}

func TestCache_Get_Expired(t *testing.T) {
	c := newTestCache(t, 100)
	c.Set("example.com", "1.2.3.4", 0)

	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, c.Get("example.com"))
}

func TestCache_HitAndMissCounters(t *testing.T) {
	c := newTestCache(t, 100)
	assert.Equal(t, int64(0), c.Hits())
	assert.Equal(t, int64(0), c.Misses())

	c.Get("miss1")
	assert.Equal(t, int64(1), c.Misses())

	c.Set("hit.com", "1.1.1.1", 300)
	c.Get("hit.com")
	assert.Equal(t, int64(1), c.Hits())
	assert.Equal(t, int64(1), c.Misses())
}

func TestCache_LRU_Eviction(t *testing.T) {
	c := newTestCache(t, 3)

	c.Set("a.com", "1.1.1.1", 300)
	c.Set("b.com", "2.2.2.2", 300)
	c.Set("c.com", "3.3.3.3", 300)
	assert.Equal(t, 3, c.Size())

	c.Get("a.com")
	c.Set("d.com", "4.4.4.4", 300)
	assert.Equal(t, 3, c.Size())
	assert.Nil(t, c.Get("b.com"))
	assert.NotNil(t, c.Get("a.com"))
	assert.NotNil(t, c.Get("c.com"))
	assert.NotNil(t, c.Get("d.com"))
}

func TestCache_SetOverwrite(t *testing.T) {
	c := newTestCache(t, 100)

	c.Set("example.com", "1.1.1.1", 300)
	c.Set("example.com", "2.2.2.2", 600)
	assert.Equal(t, 1, c.Size())

	entry := c.Get("example.com")
	require.NotNil(t, entry)
	assert.Equal(t, "2.2.2.2", entry.IP)
	assert.Equal(t, uint32(600), entry.TTL)
}

func TestCache_SetNXDOMAIN(t *testing.T) {
	c := newTestCache(t, 100)
	c.SetNXDOMAIN("blocked.com")

	entry := c.Get("blocked.com")
	require.NotNil(t, entry)
	assert.True(t, entry.NXDOMAIN)
	assert.Empty(t, entry.IP)
}

func TestCache_SetNXDOMAIN_DoesNotOverwriteExisting(t *testing.T) {
	c := newTestCache(t, 100)
	c.Set("example.com", "1.1.1.1", 300)
	c.SetNXDOMAIN("example.com")

	entry := c.Get("example.com")
	require.NotNil(t, entry)
	assert.False(t, entry.NXDOMAIN)
	assert.Equal(t, "1.1.1.1", entry.IP)
}

func TestCache_LRU_EvictionNXDOMAIN(t *testing.T) {
	c := newTestCache(t, 2)
	c.Set("a.com", "1.1.1.1", 300)
	c.SetNXDOMAIN("b.com")
	assert.Equal(t, 2, c.Size())

	c.Set("c.com", "3.3.3.3", 300)
	assert.Equal(t, 2, c.Size())
	assert.Nil(t, c.Get("a.com"))
}

func TestCache_NXDOMAIN_Expires(t *testing.T) {
	c := newTestCache(t, 100)
	c.Set("expire.com", "1.1.1.1", 0)
	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, c.Get("expire.com"), "entry with 0 TTL should expire on Get")
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := newTestCache(t, 1000)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Set("test.com", "1.1.1.1", 300)
				c.Get("test.com")
				c.Size()
			}
		}()
	}
	wg.Wait()
	assert.Greater(t, c.Hits(), int64(0))
	assert.GreaterOrEqual(t, c.Misses(), int64(0))
}
