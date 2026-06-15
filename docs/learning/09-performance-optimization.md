# Chapter 9 — Performance Optimization in This Project

> Goal: understand, concretely and quantitatively, every performance technique
> OmniDNS uses — *why* it's there, *how* it works in Go, what it costs, and where
> it could go further. This is not generic "Go is fast" advice; every item points
> at real code on a real hot path.

---

## 9.1 First, know your hot path

Optimization without knowing the hot path is guesswork. OmniDNS has exactly one
path that runs at high frequency: **`Resolve` — once per DNS query.** A busy home
network is hundreds to low-thousands of queries per second. Everything else (the
REST API, settings saves, login) runs at human click rates — a few per minute.

So the rule that governs every decision in this codebase:

> **Optimize `Resolve` and everything it touches. Favor clarity everywhere else.**

The query path is: `listeners.go` (read packet) → `interfaces/dns/handler.go` (unpack) →
`resolver.Resolve` (blocklist → steering → records → cache → upstream) →
`r.logger.LogQuery` (record it). We'll walk each cost on that path.

A mental cost model (rough orders of magnitude — *measure on your hardware*):

| Operation | Cost | On hot path? |
|-----------|------|-------------|
| In-memory map read under `RLock` | ~tens of ns | yes (cache, ARP, rules) |
| A goroutine launch | ~hundreds of ns | yes (per packet) |
| One SQLite query (round trip) | ~tens of µs–ms | avoided on hits |
| A filesystem syscall (`/proc/net/arp`) | ~ms | **removed** from hot path |
| A network round trip to upstream | ~ms–tens of ms | only on cache miss |

The whole strategy is: **turn the expensive rows into the rare rows.**

---

## 9.2 Optimization #1 — Cache the answer (the biggest win)

The single most impactful optimization in any forwarding DNS server is the
**response cache**. A cache hit replaces a multi-millisecond upstream network
round trip with a tens-of-nanoseconds map lookup — a **~100,000×** improvement on
that query.

Open [`internal/modules/resolver/engine/cache/cache.go`](../../backend/internal/modules/resolver/engine/cache/cache.go).

### The data structure: O(1) LRU

```go
type Cache struct {
	mu     sync.RWMutex
	items  map[string]*list.Element   // key -> node, for O(1) lookup
	order  *list.List                 // doubly linked list, for O(1) LRU reorder
	max    int
}
```

This is the classic **LRU = hash map + doubly linked list** pairing:

- `items` (a `map`) gives **O(1)** lookup by key.
- `order` (a `container/list` doubly linked list) gives **O(1)** "move to front"
  and "evict from back."

`Get` does both in constant time:

```go
func (c *Cache) Get(domain string, qtype uint16) *Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.items[k]
	if !ok {
		c.misses++
		return nil
	}
	entry := elem.Value.(*kv).value
	if time.Now().After(entry.ExpiresAt) {   // lazy TTL expiry
		c.removeElement(elem)
		c.misses++
		return nil
	}
	c.order.MoveToFront(elem)                // mark as recently used: O(1)
	c.hits++
	return entry
}
```

**Why this matters:** a naive LRU that scans a slice to find the
least-recently-used entry is **O(n)** per eviction. At thousands of QPS with a
1000-entry cache, that's death by a thousand cuts. The map+list design keeps
every cache operation O(1) regardless of cache size.

### Bounded memory: eviction by capacity

The cache never grows past `max` (default 1000) entries:

```go
if c.order.Len() >= c.max {
	if back := c.order.Back(); back != nil {
		c.removeElement(back)     // evict least-recently-used
	}
}
```

This is a **memory** optimization as much as a speed one: a DNS cache must have a
ceiling or it becomes an unbounded memory leak on a long-running server. Bounded
size also keeps the working set in CPU cache.

### Two-tier expiry: lazy + background

Entries expire two ways:

1. **Lazily on `Get`** (above) — the cheapest possible: an expired entry is
   removed exactly when someone asks for it.
2. **Proactively in a background sweep** — `evictLoop` runs every 30s so dead
   entries that nobody queries don't pin memory forever:

```go
func (c *Cache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	evicted := 0
	for _, elem := range c.items {
		if now.After(elem.Value.(*kv).value.ExpiresAt) {
			c.removeElement(elem)
			evicted++
			if evicted >= maxEvictPerRun {     // bound the work per sweep
				return
			}
		}
	}
}
```

Note `maxEvictPerRun` (100): the sweep **bounds its own work** so it can't hold
the write lock for an unbounded time and stall queries. That's a latency
optimization hiding inside a memory optimization — don't let a janitor block the
hot path.

### Negative caching

The cache stores **NXDOMAIN and NODATA** results, not just positive answers
(Chapter 5). Without it, every query for a nonexistent name would hit upstream
*every time*. Malware and misconfigured devices spam nonexistent names; negative
caching turns that flood into cheap local hits.

---

## 9.3 Optimization #2 — Concurrency lock discipline (`RWMutex`)

A cache that's locked wrong is either unsafe or a bottleneck. OmniDNS uses
`sync.RWMutex` deliberately:

- **Many readers, one writer.** `RLock` lets unlimited goroutines read
  concurrently; `Lock` is exclusive. For read-heavy state this dramatically beats
  a plain `Mutex` that serializes even reads.

Where it's applied — the pure reads take `RLock`:

```go
func (c *Cache) Size() int   { c.mu.RLock(); defer c.mu.RUnlock(); return c.order.Len() }
func (c *Cache) Hits() int64 { c.mu.RLock(); defer c.mu.RUnlock(); return c.hits }
```

The ARP cache's hot-path `Lookup` is a pure `RLock` read (per query!):

```go
func (c *Cache) Lookup(ip string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.table[ip]
}
```

### The subtle trap: `Get` must take a full `Lock`

As Chapter 4 noted, the cache's `Get` looks like a read but **mutates** — it
bumps `hits`/`misses` and calls `MoveToFront`. So it takes `Lock`, not `RLock`.
This is the classic performance-vs-correctness tension:

- You *want* `Get` to be a cheap concurrent reader (`RLock`).
- But maintaining LRU order and hit counters is a *write*.

OmniDNS chooses correctness: full `Lock` on `Get`. The honest trade-off is that
cache reads serialize. For a home-scale server this is fine. **If profiling
showed lock contention here, the standard fixes are:** make `hits`/`misses`
atomics (`sync/atomic`) and accept approximate LRU under `RLock`, or shard the
cache into N independently-locked buckets. The current code prefers simple and
correct — the right default until a profiler says otherwise.

> **Lesson:** know whether your "read" writes. Reach for sharding/atomics only
> after `go test -bench` or `pprof` proves contention, not on a hunch.

---

## 9.4 Optimization #3 — Move syscalls off the hot path (ARP cache)

This is the clearest before/after in the project.

**Before:** every query called `lookupMAC(ip)`, which opened and line-scanned
`/proc/net/arp` — a filesystem syscall, ~1 ms, on every single DNS query.

**After:** [`internal/modules/resolver/engine/arp/arp.go`](../../backend/internal/modules/resolver/engine/arp/arp.go)
refreshes the whole table on a 30s background goroutine; the per-query `Lookup`
is the in-memory `RLock` map read shown above.

```go
func (c *Cache) Start(ctx context.Context) {
	if !c.enabled {
		return                       // disabled? no goroutine, no cost at all
	}
	c.reload()                       // one synchronous load at startup
	go func() {
		ticker := time.NewTicker(c.refresh)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.reload()           // swap in a fresh table
			}
		}
	}()
}
```

The pattern — **"read rarely-changing external state on a timer, serve it from
memory"** — is one of the highest-leverage optimizations you can apply. The ARP
table changes on the order of minutes; reading it per query (thousands of times a
second) was pure waste. Cost moved from O(queries) syscalls to O(1 per 30s).

**It's also opt-out:** `EnableMACLookup` can turn it off entirely (no goroutine,
`Lookup` returns `""`), and on non-Linux it's a no-op via build tags (Chapter 2).
Zero cost when unused.

---

## 9.5 Optimization #4 — The read-mostly rule cache

Steering rules are read on **every** query but change **rarely** (an admin edits
them occasionally). Querying SQLite for the rule set per DNS query would add a
database round trip to the hot path. The resolver caches them for 10 seconds
behind a double-checked `RWMutex` — `activeRules` in
[`resolver.go`](../../backend/internal/modules/resolver/engine/resolver.go):

```go
func (r *Resolver) activeRules() []models.SteeringRule {
	r.rulesMu.RLock()
	if time.Since(r.rulesLoadedAt) < 10*time.Second {
		rules := r.rulesCache
		r.rulesMu.RUnlock()
		return rules                 // fast path: cheap RLock, no DB
	}
	r.rulesMu.RUnlock()

	rules := r.steering.Rules()      // slow path: refresh from DB (rare)
	r.rulesMu.Lock()
	r.rulesCache = rules
	r.rulesLoadedAt = time.Now()
	r.rulesMu.Unlock()
	return rules
}
```

Two techniques in one small function:

- **Time-based cache with explicit staleness.** The 10s TTL is a *deliberate*
  trade: rule edits take up to 10s to take effect, in exchange for ~zero
  per-query DB load. For steering rules that's a great deal.
- **Double-checked locking.** Take the cheap `RLock` first; only escalate to the
  exclusive `Lock` when the cache is actually stale. The common case (cache warm)
  never blocks writers or other readers beyond the brief `RLock`.

> **Thread-safety nuance worth teaching:** the fast path returns the *shared*
> `r.rulesCache` slice. That's safe **only because the slice is never mutated in
> place** — a refresh *replaces* the whole slice (`r.rulesCache = rules`).
> Readers hold a reference to the old backing array, which stays valid. Mutating
> a shared slice's elements under `RLock` would be a race. Replace, don't mutate.

---

## 9.6 Optimization #5 — Batch and amortize writes (the log buffer)

Writing to SQLite synchronously on every query would put a disk write on the hot
path — catastrophic at thousands of QPS. OmniDNS makes logging **asynchronous and
batched**. Open [`internal/infrastructure/persistence/sqlite.go`](../../backend/internal/infrastructure/persistence/sqlite.go).

### Hand-off is non-blocking

`LogQuery` (called from the hot path) just sends to a buffered channel and
returns — it never touches the disk and never blocks:

```go
func (db *DB) LogQuery(log models.QueryLog) {
	log.Timestamp = time.Now()
	select {
	case db.logChan <- log:          // enqueue (channel is buffered: 1000)
	default:
		slog.Warn("query log buffer full; dropping entry", "domain", log.Domain)
	}
}
```

The `select/default` is **back-pressure by design**: if the writer falls behind
and the 1000-deep buffer fills, we *drop a log line* rather than slow DNS
resolution. A dropped log is invisible to users; a slow resolver is not. This is
a conscious latency-over-completeness choice for the hot path.

### Writes are batched into one transaction

A single consumer goroutine accumulates logs and flushes them **100 at a time, or
every 5 seconds** (both configurable), in **one transaction with a prepared
statement**:

```go
tx, err := db.conn.Begin()
// ...
stmt, err := tx.Prepare("INSERT INTO query_logs (...) VALUES (?, ?, ...)")
for _, log := range logs {
	stmt.Exec(...)                   // reuse the compiled statement
}
tx.Commit()                          // one fsync for the whole batch
```

Why this is fast:

- **One transaction = one `fsync`** for up to 100 rows, instead of 100 separate
  durable writes. Disk `fsync` is the expensive part; amortizing it across a
  batch is a large win.
- **A prepared statement** is compiled once and reused for every row in the
  batch, skipping per-row SQL parsing/planning.

The `LogFlushSize` / `LogFlushInterval` knobs let you trade durability latency
for throughput: bigger batches = fewer fsyncs = higher throughput but logs appear
later. The refactor made these configurable precisely so you can tune for your
workload.

---

## 9.7 Optimization #6 — Fewer round trips at the database

Round trips dominate database cost. Two examples.

**`GetStats` — one query instead of four.** The old code ran four
`SELECT COUNT(*) ... WHERE action = '...'` queries (four scans, four round
trips). The new one does a single pass with conditional aggregation
([Chapter 6](06-persistence.md)):

```sql
SELECT
  SUM(CASE WHEN action = 'forwarded' THEN 1 ELSE 0 END),
  SUM(CASE WHEN action = 'blocked'   THEN 1 ELSE 0 END),
  SUM(CASE WHEN action = 'custom'    THEN 1 ELSE 0 END),
  SUM(CASE WHEN action = 'cached'    THEN 1 ELSE 0 END)
FROM query_logs
```

One scan, one round trip, four counts. (This is on the *cold* API path, but it's
a clean illustration of the "collapse N queries into 1" principle.)

**SQLite pragmas tuned for a write-heavy workload.** At `Open`:

```go
"PRAGMA journal_mode=WAL",      // readers don't block writers, and vice versa
"PRAGMA busy_timeout=5000",     // wait up to 5s on a lock instead of erroring
"PRAGMA synchronous=NORMAL",    // safe-enough durability, far fewer fsyncs than FULL
```

- **WAL** is the big one: the DNS path writes logs while the API reads them
  concurrently. Under the default rollback journal, those would block each other.
  WAL lets them proceed in parallel.
- **`synchronous=NORMAL`** under WAL skips an fsync per commit (safe across app
  crashes; only a power loss at the wrong instant risks the last transactions —
  acceptable for query logs).

---

## 9.8 Optimization #7 — Cheap concurrency and connection reuse

**Goroutine-per-query.** The UDP loop launches a goroutine per packet
([`listeners.go`](../../backend/internal/server/listeners.go)):

```go
go s.handler.HandleUDP(conn, client, pkt)
```

Goroutines are cheap (a few KB, grown on demand), so this scales to many
in-flight queries without a thread pool. The accept loop stays free to read the
next packet immediately — no head-of-line blocking.

**The buffer-copy that enables it (and prevents a data race):**

```go
buf := make([]byte, 1500)        // allocated ONCE, outside the loop
for {
	n, client, err := conn.ReadFromUDP(buf)
	// ...
	pkt := make([]byte, n)        // small per-packet copy the goroutine owns
	copy(pkt, buf[:n])
	go s.handler.HandleUDP(conn, client, pkt)
}
```

`buf` is reused across iterations (one allocation, not one per packet). But we
can't hand `buf` itself to the goroutine — the next `ReadFromUDP` would overwrite
it mid-use. So we copy just the `n` bytes into a fresh `pkt`. This is the minimal
allocation needed for correctness: **reuse the big buffer, copy only what you
hand off.**

**`*sql.DB` is a connection pool.** Every DNS-logging goroutine and every API
handler shares one `*db.DB`. `database/sql` pools connections internally and is
safe for concurrent use, so we never open a connection per query and never wrap
it in our own mutex. The pool *is* the optimization.

---

## 9.9 Optimization #8 — Health-checked failover (tail-latency)

A dead upstream is a latency disaster: every query waits for a timeout before
failing over. [`forwarder`](../../backend/internal/modules/resolver/engine/forwarder/) probes each
upstream every 30s and marks it healthy/unhealthy, so `Forward` **skips known-bad
upstreams immediately** instead of timing out on them per query:

```go
for i := range p.snapshot() {
	p.mu.RLock()
	up, ok := p.upstreams[i], p.healthy[i]
	p.mu.RUnlock()
	if !ok {
		continue                 // skip unhealthy: no per-query timeout wasted
	}
	// ... try this upstream ...
}
```

This optimizes the **tail** (p99) latency, which on a DNS server is what users
actually feel — a few slow lookups stall a whole page load. Background health
checks convert "every query pays the timeout" into "one probe pays it every 30s."

---

## 9.10 Honest accounting: costs and remaining opportunities

Good performance writing names the trade-offs, not just the wins.

**Deliberate trade-offs already made:**

- **`Get` takes a full `Lock`** (§9.3) — simpler and correct; cache reads
  serialize. Fine at home scale.
- **Dropped logs under burst** (§9.6) — completeness sacrificed to protect
  latency.
- **10s stale rules / lazy + 30s ARP refresh** — bounded staleness bought with
  near-zero per-query cost.
- **`synchronous=NORMAL`** — a sliver of crash durability for far fewer fsyncs.

**Real opportunities, honestly flagged (don't do them without a profiler):**

1. **`LookupRecord` does up to 2 queries on a miss**
   ([`records.go`](../../backend/internal/infrastructure/persistence/records.go)): one for the exact
   `(domain, qtype)`, then a `COUNT(*)` to detect the other family. It could be
   one `SELECT qtype FROM custom_records WHERE domain = ?` and decide in Go. But
   custom-record lookups are *rare* (most queries are cache hits or forwards), so
   this is low priority — a good example of "correct and clear beats clever on a
   cold path."
2. **`forwarder.snapshot()` allocates a slice copy per `Forward` call** to range
   safely under lock. On the hot path that's a small allocation per *cache miss*.
   Could be avoided by indexing under the lock, at the cost of holding it longer.
   Measure before changing.
3. **`strings.Join(ips, ",")`** in the resolver allocates for the log entry's
   `AllAnswers`. It's only built when an answer exists; negligible, but it's a
   real allocation on the forward path.
4. **Cache key is `domain + "|" + qtype` (a string concat)** — one small
   allocation per `Get`. A struct key `{string, uint16}` would avoid it; the
   string key is simpler and the allocation is tiny. Classic readability-vs-µs.

The meta-point: this codebase optimizes the **structural** things that matter at
any scale (O(1) cache, no syscalls/DB on the hot path, batched writes, WAL,
failover) and leaves the **micro** allocations alone until a profiler justifies
touching them. That ordering — algorithms and I/O first, allocations last — is
the heart of performance engineering.

---

## 9.11 How to measure it yourself

Optimization claims mean nothing without measurement. Go gives you the tools:

```bash
# Microbenchmark a function (see the exercise below)
go test -bench=. -benchmem ./internal/modules/resolver/engine/cache/

# CPU profile while load-testing the running server
go test -cpuprofile=cpu.out -bench=BenchmarkCacheGet ./internal/modules/resolver/engine/cache/
go tool pprof cpu.out          # then 'top', 'list Get', 'web'

# Find lock contention
go test -mutexprofile=mu.out -bench=. ./internal/modules/resolver/engine/cache/

# Always validate concurrency correctness alongside perf
go test -race ./...
```

`-benchmem` reports `allocs/op` and `B/op` — the numbers behind §9.10's
allocation notes. **The discipline:** form a hypothesis ("the cache key concat
allocates"), measure it (`-benchmem`), change it, measure again. Never optimize
by feel.

---

## Exercises

1. **Benchmark the cache.** Add to `cache_test.go`:
   ```go
   func BenchmarkCacheGet(b *testing.B) {
   	c := New(1000); defer c.Close()
   	c.Set("example.com", dns.TypeA, []string{"1.2.3.4"}, 300)
   	b.ResetTimer()
   	for i := 0; i < b.N; i++ { c.Get("example.com", dns.TypeA) }
   }
   ```
   Run `go test -bench=BenchmarkCacheGet -benchmem ./internal/modules/resolver/engine/cache/`. Record
   `ns/op` and `allocs/op`. How many allocations per `Get`? Where do they come
   from? (Hint: the `key()` concat.)
2. **Prove the LRU is O(1).** Benchmark `Get` with cache sizes 100, 10_000, and
   1_000_000 entries. Confirm `ns/op` stays flat. Now imagine a slice-scan LRU —
   how would the curve change?
3. **Measure the lock trade-off.** Write a parallel benchmark
   (`b.RunParallel`) hammering `Get` from many goroutines. Then try making
   `hits`/`misses` `atomic.Int64` and `Get` use `RLock` for the lookup. Does
   throughput improve under contention? Is the LRU still correct? (This is the
   §9.3 trade-off, measured.)
4. **Batch size sweep.** Run the server with `--log-flush-size 1` vs `1000` and a
   load generator. Observe CPU/disk. Explain the curve in terms of fsync
   amortization.
5. **Spot a hot-path allocation.** Using `-benchmem`, write a benchmark around
   `Resolve` (reuse the fakes from `resolver_test.go`) for a cache-hit query.
   Find the allocations and map each to a line in §9.10. Which would you fix
   first, and what does the profiler say?

[← Chapter 8](08-testing-and-optimizations.md) · [Back to the index](README.md)
