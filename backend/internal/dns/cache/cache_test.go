package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := New(100)
	defer c.Close()
	assert.Equal(t, 0, c.Size())
	assert.Equal(t, int64(0), c.Hits())
	assert.Equal(t, int64(0), c.Misses())
}

func TestSetAndGet(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Set("example.com", dns.TypeA, []string{"1.2.3.4"}, 300)

	entry := c.Get("example.com", dns.TypeA)
	require.NotNil(t, entry)
	assert.Equal(t, "1.2.3.4", entry.First())
	assert.Equal(t, uint32(300), entry.TTL)
	assert.False(t, entry.NXDOMAIN)
}

func TestQtypeIsolation(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Set("example.com", dns.TypeA, []string{"1.2.3.4"}, 300)

	// AAAA for the same name must be a miss even though A is cached.
	assert.Nil(t, c.Get("example.com", dns.TypeAAAA))
	assert.NotNil(t, c.Get("example.com", dns.TypeA))

	c.Set("example.com", dns.TypeAAAA, []string{"::1"}, 300)
	assert.Equal(t, "::1", c.Get("example.com", dns.TypeAAAA).First())
}

func TestGetMiss(t *testing.T) {
	c := New(100)
	defer c.Close()
	assert.Nil(t, c.Get("nonexistent.com", dns.TypeA))
}

func TestGetExpired(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Set("example.com", dns.TypeA, []string{"1.2.3.4"}, 0)
	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, c.Get("example.com", dns.TypeA))
}

func TestHitAndMissCounters(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Get("miss1", dns.TypeA)
	assert.Equal(t, int64(1), c.Misses())

	c.Set("hit.com", dns.TypeA, []string{"1.1.1.1"}, 300)
	c.Get("hit.com", dns.TypeA)
	assert.Equal(t, int64(1), c.Hits())
	assert.Equal(t, int64(1), c.Misses())
}

func TestLRUEviction(t *testing.T) {
	c := New(3)
	defer c.Close()
	c.Set("a.com", dns.TypeA, []string{"1.1.1.1"}, 300)
	c.Set("b.com", dns.TypeA, []string{"2.2.2.2"}, 300)
	c.Set("c.com", dns.TypeA, []string{"3.3.3.3"}, 300)
	assert.Equal(t, 3, c.Size())

	c.Get("a.com", dns.TypeA)
	c.Set("d.com", dns.TypeA, []string{"4.4.4.4"}, 300)
	assert.Equal(t, 3, c.Size())
	assert.Nil(t, c.Get("b.com", dns.TypeA))
	assert.NotNil(t, c.Get("a.com", dns.TypeA))
}

func TestSetOverwrite(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Set("example.com", dns.TypeA, []string{"1.1.1.1"}, 300)
	c.Set("example.com", dns.TypeA, []string{"2.2.2.2"}, 600)
	assert.Equal(t, 1, c.Size())

	entry := c.Get("example.com", dns.TypeA)
	require.NotNil(t, entry)
	assert.Equal(t, "2.2.2.2", entry.First())
	assert.Equal(t, uint32(600), entry.TTL)
}

func TestSetNXDOMAIN(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.SetNXDOMAIN("blocked.com", dns.TypeA)

	entry := c.Get("blocked.com", dns.TypeA)
	require.NotNil(t, entry)
	assert.True(t, entry.NXDOMAIN)
	assert.Empty(t, entry.First())
}

func TestSetNoData(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.SetNoData("v6only.com", dns.TypeA)

	entry := c.Get("v6only.com", dns.TypeA)
	require.NotNil(t, entry)
	assert.True(t, entry.NoData)
	assert.Empty(t, entry.First())
}

func TestNegativeDoesNotOverwritePositive(t *testing.T) {
	c := New(100)
	defer c.Close()
	c.Set("example.com", dns.TypeA, []string{"1.1.1.1"}, 300)
	c.SetNXDOMAIN("example.com", dns.TypeA)

	entry := c.Get("example.com", dns.TypeA)
	require.NotNil(t, entry)
	assert.False(t, entry.NXDOMAIN)
	assert.Equal(t, "1.1.1.1", entry.First())
}

func TestConcurrentAccess(t *testing.T) {
	c := New(1000)
	defer c.Close()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Set("test.com", dns.TypeA, []string{"1.1.1.1"}, 300)
				c.Get("test.com", dns.TypeA)
				c.Size()
			}
		}()
	}
	wg.Wait()
	assert.Greater(t, c.Hits(), int64(0))
}
