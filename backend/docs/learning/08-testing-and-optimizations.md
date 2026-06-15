# Chapter 8 — Testing, the Race Detector, and Optimizations

> Goal: learn Go's testing model from the project's real tests, run the race
> detector, then study a catalog of every optimization the refactor made — and
> why each one matters.

---

## 8.1 Go's built-in testing

No framework required. A test is a function `TestXxx(t *testing.T)` in a file
ending `_test.go`. Run them with `go test ./...`.

Open [`internal/domain/records/record_test.go`](../../internal/domain/records/record_test.go):

```go
func TestNewNormalizesAndValidatesRecord(t *testing.T) {
	record, err := New(" Example.LOCAL ", "192.168.1.10")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if got := record.Domain().String(); got != "example.local" {
		t.Fatalf("domain = %q, want %q", got, "example.local")
	}
}
```

- `t.Fatalf` reports a failure and stops *this* test (use `t.Errorf` to report
  but continue).
- The convention is `got` vs `want`. Messages say what was expected.

### Table-driven tests

The idiomatic Go way to test many cases without repetition — same file:

```go
func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		ip      string
		wantErr error
	}{
		{name: "empty domain", domain: "", ip: "192.168.1.10", wantErr: ErrInvalidDomain},
		{name: "bad ip", domain: "example.local", ip: "not-an-ip", wantErr: ErrInvalidIP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {        // a named subtest per case
			_, err := New(tt.domain, tt.ip)
			if !errors.Is(err, tt.wantErr) {       // Ch 1: sentinel-error check
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
```

A **slice of anonymous structs** holds the cases; `t.Run(tt.name, ...)` makes
each a named subtest so failures point at the exact case. Adding a case is one
line. This is the single most useful Go testing pattern — internalize it.

---

## 8.2 Testing with fakes (no database, no network)

Because the resolver depends on **interfaces** (Chapter 3), we test it with
hand-written fakes instead of a real DB or upstream. From
[`internal/dns/resolver/resolver_test.go`](../../internal/dns/resolver/resolver_test.go):

```go
type fakeRecords struct {
	v4 map[string]string   // domain -> IPv4
	v6 map[string]string   // domain -> IPv6
}

func (f fakeRecords) Lookup(domain string, qtype uint16) (string, bool, bool) {
	switch qtype {
	case dns.TypeA:
		if ip, ok := f.v4[domain]; ok {
			return ip, true, false
		}
		_, other := f.v6[domain]
		return "", false, other       // mimics real "other-family exists"
	case dns.TypeAAAA:
		// ... symmetric ...
	}
	return "", false, false
}
```

`fakeRecords` satisfies the `CustomRecords` port with an in-memory map. We inject
it via the `Deps` struct. Now we can assert the AAAA/NODATA fix from Chapter 5
deterministically:

```go
func TestCustomRecordAAAAOverA_ReturnsNoData(t *testing.T) {
	r, lg := newResolver(t, Deps{Records: fakeRecords{v4: map[string]string{"box.local": "192.168.1.10"}}})
	resp := r.Resolve(query("box.local", dns.TypeAAAA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)   // NOERROR
	assert.Empty(t, resp.Answer, "AAAA over A-only record must be NODATA")
	require.Len(t, lg.entries, 1)
	assert.Equal(t, models.ActionCustom, lg.entries[0].Action)
}
```

This test **could not exist** in the old code, where the resolver held a concrete
`*db.DB`. Testability is a direct dividend of dependency inversion.

### `testify`: assertions that read well

This project uses `stretchr/testify`:

- `assert.Equal(t, want, got)` — report and continue.
- `require.NotNil(t, x)` — report and **stop** (like `t.Fatal`). Use `require`
  when continuing would panic (e.g. you're about to dereference `x`).

### `t.Helper()` and `t.Cleanup()`

In `newResolver` and the DB test helper you'll see:

```go
func newTestDB(t *testing.T, ttl time.Duration) *DB {
	t.Helper()                          // failures point at the CALLER, not here
	path := filepath.Join(t.TempDir(), "test.db")   // auto-removed temp dir
	database, err := Open(path, Options{SessionTTL: ttl})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })       // runs at test end
	return database
}
```

- `t.Helper()` marks this as a helper so a failing assertion blames the test that
  called it.
- `t.TempDir()` gives a unique directory cleaned up automatically — perfect for a
  throwaway SQLite file.
- `t.Cleanup(fn)` registers teardown (close the DB) without a manual `defer` in
  every test.

See [`session_test.go`](../../internal/db/session_test.go) for how this tests the
real session-expiry behavior against an actual (temporary) SQLite database.

---

## 8.3 The race detector: your most important tool

Concurrency bugs (Chapter 4) are invisible until they corrupt data in
production. The Go **race detector** finds them. Run:

```bash
go test -race ./...
```

It instruments memory accesses and flags any two goroutines touching the same
memory without synchronization where at least one writes. The cache's
concurrency test exists specifically to exercise this:

```go
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
```

Ten goroutines hammer the cache concurrently. Under `-race`, if the cache's
`RWMutex` usage were wrong (recall the `Get` write-under-read trap from
Chapter 4), this test would fail loudly. **Always run `-race` in CI.** This whole
project passes it.

---

## 8.4 Optimization catalog — what the refactor changed and why

Each entry: the problem, the fix, the file, and the lesson.

### 1. Per-query syscall → cached ARP table
- **Before:** every DNS query opened and parsed `/proc/net/arp` to find the
  client MAC — a filesystem syscall on the hot path, ~1 ms each.
- **After:** [`dns/arp`](../../internal/dns/arp/) refreshes the table on a 30 s
  background goroutine; `Lookup` is a map read under `RLock`.
- **Lesson:** move slow, rarely-changing work *out* of the hot path and cache it.
  Refresh on a timer, read from memory.

### 2. Four COUNT queries → one aggregate
- **Before:** `GetStats` ran 4 separate `SELECT COUNT(*) WHERE action=...` — four
  full scans, four round trips.
- **After:** one `SUM(CASE WHEN ...)` pass (Chapter 6).
- **Lesson:** prefer one query with conditional aggregation over N queries. Fewer
  round trips, one scan.

### 3. Domain-only cache → qtype-keyed cache with negative entries
- **Before:** cache keyed by domain, stored only A records, no negative caching.
  AAAA queries were never cached and sometimes mishandled.
- **After:** [`dns/cache`](../../internal/dns/cache/) keys by `(domain, qtype)`
  and caches positive, NXDOMAIN, **and** NODATA results.
- **Lesson:** the cache key must include everything that distinguishes a result.
  Negative caching cuts upstream load for nonexistent names.

### 4. Synchronous logging → batched async writes
- **Before/now refined:** query logs are buffered through a channel and flushed
  in batches of 100 or every 5 s, in **one transaction** with a prepared
  statement (Chapters 4 & 6). The refactor made the flush **graceful** (no lost
  logs on shutdown) and the size/interval **configurable**.
- **Lesson:** batch high-frequency writes; amortize transaction cost; never block
  the hot path on I/O (`LogQuery` drops rather than blocks when the buffer is
  full).

### 5. Read-mostly steering-rule cache
- **Mechanism:** the resolver caches the rule set for 10 s behind an `RWMutex`,
  refreshing only when stale (Chapter 5).
- **Lesson:** for data read on every request but written rarely, a short-TTL
  in-memory snapshot with a double-checked `RLock` removes per-request DB hits
  and lock contention.

### 6. Health-checked upstream failover
- **Mechanism:** [`dns/forwarder`](../../internal/dns/forwarder/) probes
  upstreams every 30 s, marks them healthy/unhealthy, and `Forward` skips
  unhealthy ones (Chapter 4).
- **Lesson:** don't send traffic to a dead dependency. Background health checks +
  a healthy-flag map give fast failover without per-query probing.

### 7. God-object → narrow interfaces (testability *is* an optimization)
- **Before:** the resolver and handlers held a concrete `*db.DB`; nothing could be
  unit-tested without a real database.
- **After:** small ports (Chapter 3) make every layer fakeable; the test suite
  runs in milliseconds with no I/O.
- **Lesson:** fast, isolated tests are a performance win for *developers*, and
  they catch correctness bugs (like the AAAA fix) before they ship.

### Security fixes (correctness, not speed, but essential)
- **Sessions expire** (`expires_at` + lazy delete + hourly sweep) — Chapter 6.
- **CORS allowlist** instead of `*`, enforced at config validation — Chapters 2 & 7.
- **Body size limits** (`MaxBytesReader`) and **parameterized SQL** everywhere —
  Chapters 6 & 7.
- **Argon2id + constant-time compare** for passwords — Chapter 6.

---

## 8.5 The testing/optimization mindset

A few principles this codebase embodies:

1. **Measure the hot path.** The DNS `Resolve` path runs per query; everything
   there is optimized (cached ARP, cached rules, cached answers, async logging).
   The API path runs per admin click; it favors clarity over micro-optimization.
2. **Depend on interfaces to enable testing.** If something is hard to test, it's
   usually too coupled. The fix (inject an interface) often improves the design.
3. **Cache with the right key and the right invalidation.** Most cache bugs are
   wrong-key (Chapter 5's qtype) or wrong-staleness (the 10 s rule TTL is an
   explicit, documented trade-off).
4. **Run `-race` and keep tests fast.** Fast tests get run; slow tests get
   skipped; unrun tests catch nothing.

---

## 8.6 Capstone exercises

1. **Write a NODATA upstream test.** Add a fake forwarder `Pool` (or a small
   interface around it) and a resolver test proving that an upstream `NOERROR`
   with no answers gets cached via `SetNoData` and served as `cached` on the
   second call.
2. **Benchmark the cache.** Add `func BenchmarkCacheGet(b *testing.B)` in
   `cache_test.go` that pre-populates one entry and loops `c.Get` in `b.N`. Run
   `go test -bench=. ./internal/dns/cache/`. Then try `-benchmem` to see
   allocations.
3. **Table-test `matchCIDR`.** In `internal/dns/resolver`, write a table-driven
   test for `matchCIDR` covering an in-range IP, an out-of-range IP, a bare-IP
   match, and a malformed CIDR. (It's a pure function — easy and high-value.)
4. **Find an N+1.** `LookupRecord` does up to two queries per miss. Sketch how
   you'd collapse it into one query with a `CASE`/`GROUP BY`. Is it worth it?
   (Custom-record lookups are rare vs. cache hits — argue both sides.)
5. **Prove the session sweep.** Extend `session_test.go`: create one fresh and
   one back-dated session, call `SweepExpiredSessions`, and assert exactly one
   remains.
6. **Race a bug into existence.** In a *scratch branch*, remove the `RLock` from
   `arp.Cache.Lookup` and add a test that calls `Lookup` while `reload` runs in
   another goroutine. Confirm `-race` catches it. Throw the branch away.

---

## Where to go next

You've now read the entire OmniDNS backend and the Go concepts behind it. To keep
learning:

- **A Tour of Go** (go.dev/tour) — fill any syntax gaps.
- **Effective Go** (go.dev/doc/effective_go) — the idiom bible.
- **The `database/sql` tutorial** and **`context` blog post** on go.dev.
- Extend OmniDNS: implement real DHCP lease parsing, add Prometheus metrics, or
  add per-client query-rate limiting. Each is a self-contained vertical slice
  through the architecture you now understand.

[← Chapter 7](07-http-api.md) · [Back to the index](README.md)
