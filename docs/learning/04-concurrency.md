# Chapter 4 — Concurrency: Goroutines, Channels, Mutexes, Context

> Goal: master Go's concurrency model using the live examples in this codebase —
> the cache, the forwarder's health loop, the buffered log writer, the ARP
> refresher, and the server's graceful shutdown.

Concurrency is Go's headline feature. OmniDNS handles many DNS queries at once,
refreshes state in the background, and shuts down cleanly. Every one of those is
a concurrency lesson.

---

## 4.1 Goroutines: cheap concurrent functions

A **goroutine** is a function running concurrently, started with the `go`
keyword. They're extremely lightweight (a few KB of stack), so a Go program
happily runs thousands.

The server spawns one goroutine **per incoming UDP packet**. From
[`internal/server/listeners.go`](../../backend/internal/server/listeners.go):

```go
n, client, err := conn.ReadFromUDP(buf)
if err != nil {
	continue
}
pkt := make([]byte, n)
copy(pkt, buf[:n])
go s.handler.HandleUDP(conn, client, pkt)   // handle this query concurrently
```

Two subtle but vital details:

### Why copy the buffer?

`buf` is a single shared slice reused on every loop iteration. If we passed
`buf[:n]` straight to the goroutine, the **next** `ReadFromUDP` would overwrite
the bytes while the goroutine is still reading them — a classic data race. So we
`copy` the packet into a fresh `pkt` that the goroutine fully owns. **Ownership
transfer via copying** is a core concurrency-safety technique.

### Fire-and-forget is OK here

We don't wait for `HandleUDP` to finish. DNS over UDP is request/response per
packet; the goroutine writes its own reply and exits. If it errors, it logs and
moves on. There's no shared mutable state between concurrent queries except the
cache and DB — and those are made safe below.

---

## 4.2 The accept loop pattern

Look at the surrounding loop in `startUDP`:

```go
for {
	select {
	case <-ctx.Done():
		return
	default:
	}
	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		slog.Error("set udp read deadline failed", "error", err)
	}
	n, client, err := conn.ReadFromUDP(buf)
	if err != nil {
		continue
	}
	// ... spawn handler goroutine ...
}
```

This is a **non-blocking shutdown check** combined with a **read deadline**.
Why both?

- `ReadFromUDP` blocks until a packet arrives. If we never set a deadline, the
  loop could be stuck in `ReadFromUDP` forever, never noticing shutdown.
- `SetReadDeadline(now + 500ms)` makes `ReadFromUDP` return a timeout error
  every 500 ms at worst. On timeout we `continue`, loop back, and re-check
  `ctx.Done()`.

So the loop wakes up at least twice a second to ask "should I stop?". The
`select { case <-ctx.Done(): return; default: }` is a **non-blocking receive**:
if the context is cancelled, take that branch and return; otherwise `default`
falls straight through without blocking. We'll see `ctx` fully in §4.6.

---

## 4.3 Channels: the buffered log writer

A **channel** is a typed pipe that safely passes values between goroutines.
*"Don't communicate by sharing memory; share memory by communicating."*

The query-log writer is the textbook example. Writing to SQLite on every single
DNS query would be slow, so logs are **batched**. Open
[`internal/infrastructure/persistence/sqlite.go`](../../backend/internal/infrastructure/persistence/sqlite.go):

```go
type DB struct {
	conn      *sql.DB
	logChan   chan models.QueryLog   // the pipe
	logBuffer []models.QueryLog
	mu        sync.Mutex
	quit      chan struct{}
	flushDone chan struct{}
}
```

The resolver (running in many goroutines) calls `LogQuery`, which just **sends**
to the channel and returns immediately:

```go
func (db *DB) LogQuery(log models.QueryLog) {
	log.Timestamp = time.Now()
	select {
	case db.logChan <- log:               // try to enqueue
	default:
		slog.Warn("query log buffer full; dropping entry", "domain", log.Domain)
	}
}
```

Again a **non-blocking send** via `select`/`default`: if the channel buffer is
full (the consumer is behind), we drop the log rather than block the DNS hot
path. Dropping a log line is far better than slowing every query. This is a
deliberate **back-pressure** decision.

A single consumer goroutine drains the channel:

```go
func (db *DB) processLogBuffer() {
	defer close(db.flushDone)
	ticker := time.NewTicker(db.opts.LogFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case log := <-db.logChan:               // a log arrived
			db.mu.Lock()
			db.logBuffer = append(db.logBuffer, log)
			full := len(db.logBuffer) >= db.opts.LogFlushSize
			db.mu.Unlock()
			if full {
				db.flush()                       // batch is full -> write
			}
		case <-ticker.C:                         // time elapsed -> write what we have
			db.flush()
		case <-db.quit:                          // shutting down
			db.drain()
			db.flush()
			return
		}
	}
}
```

This one `select` with **three cases** is the heart of the writer:

1. **A log arrived** → buffer it; flush if we hit `LogFlushSize` (default 100).
2. **The ticker fired** (every `LogFlushInterval`, default 5s) → flush whatever's
   buffered, so logs don't sit forever during quiet periods.
3. **Quit signalled** → drain any stragglers and do a final flush.

`select` blocks until *one* case is ready, then runs it. It's how a single
goroutine multiplexes several event sources.

### Graceful shutdown of the writer (a refactor fix)

The old code lost the last batch of logs on shutdown. The new `Close` waits for
the writer to finish:

```go
func (db *DB) Close() error {
	close(db.quit)       // tell the writer to stop
	<-db.flushDone       // BLOCK until it has drained + flushed and exited
	return db.conn.Close()
}
```

- `close(db.quit)` makes the `case <-db.quit` branch fire (a receive on a closed
  channel returns immediately).
- The writer calls `drain()` + `flush()`, then returns, which runs its deferred
  `close(db.flushDone)`.
- `<-db.flushDone` in `Close` unblocks only *after* that final flush.

This is a **completion signal via channel close**: closing `flushDone` broadcasts
"I'm done" to whoever is waiting. No data is lost on shutdown now.

> **Channel idioms recap:**
> - `ch <- v` send, `v := <-ch` receive.
> - `close(ch)` signals "no more values"; receives on a closed channel return
>   the zero value immediately. Great for broadcast/done signals.
> - `chan struct{}` is a *signal-only* channel — `struct{}` carries no data and
>   uses no memory; we only care that something happened.

---

## 4.4 Mutexes: protecting the cache

Channels are great for *handing off* values. But sometimes many goroutines need
to read and write **shared state** — like the DNS cache. For that, Go uses a
**mutex** (mutual exclusion lock).

Open [`internal/modules/resolver/engine/cache/cache.go`](../../backend/internal/modules/resolver/engine/cache/cache.go):

```go
type Cache struct {
	mu     sync.RWMutex
	items  map[string]*list.Element
	order  *list.List
	// ...
}
```

A `sync.RWMutex` has two modes:

- `Lock()` / `Unlock()` — exclusive **write** lock (one goroutine).
- `RLock()` / `RUnlock()` — shared **read** lock (many readers at once, but no
  writer).

Use it for reads that don't mutate:

```go
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}
```

And the full lock for anything that mutates *or that mutates counters*:

```go
func (c *Cache) Get(domain string, qtype uint16) *Entry {
	c.mu.Lock()           // full Lock, not RLock — why?
	defer c.mu.Unlock()
	elem, ok := c.items[key(domain, qtype)]
	if !ok {
		c.misses++        // <-- this mutation is the reason
		return nil
	}
	// ... also does c.order.MoveToFront(elem) and c.hits++
}
```

**Teaching moment:** `Get` looks like a read, but it mutates — it bumps
`hits`/`misses` and calls `MoveToFront` to maintain LRU order. So it needs a full
`Lock`, not `RLock`. Recognizing "this read has a write side-effect" is a common
source of subtle race bugs. The data-race detector (Chapter 8) catches them if
you get it wrong.

### `defer mu.Unlock()` — never forget to unlock

`defer c.mu.Unlock()` right after `Lock()` guarantees the unlock runs on **every**
return path, including early returns and panics. This pattern prevents the
single most common deadlock: forgetting to unlock on one branch.

> One sharp edge to know: `defer` runs at *function* return, not block end. In a
> hot loop you sometimes unlock manually to release the lock sooner. The cache's
> `evictExpired` holds the lock for the whole sweep on purpose (it's bounded by
> `maxEvictPerRun`), so `defer` is fine there.

---

## 4.5 Background workers with tickers

Several components do periodic work on their own goroutine. The pattern is
always: `NewTicker` + `for { select { <-ticker.C ... <-stop ... } }`.

**The forwarder health loop** —
[`internal/modules/resolver/engine/forwarder/health.go`](../../backend/internal/modules/resolver/engine/forwarder/health.go):

```go
func (p *Pool) healthLoop() {
	ticker := time.NewTicker(healthProbeInterval)   // every 30s
	defer ticker.Stop()
	probe := new(dns.Msg)
	probe.SetQuestion("google.com.", dns.TypeA)
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.probeAll(probe)     // ping each upstream, flip healthy flags
		}
	}
}
```

**The cache eviction loop** — `cache.go`'s `evictLoop` runs every 30s and removes
expired entries (so dead entries don't linger between lazy expiries on `Get`).

**The ARP refresher** —
[`internal/modules/resolver/engine/arp/arp.go`](../../backend/internal/modules/resolver/engine/arp/arp.go) — reloads the
IP→MAC table on its interval:

```go
func (c *Cache) Start(ctx context.Context) {
	if !c.enabled {
		return            // disabled: no goroutine at all
	}
	c.reload()            // initial synchronous load
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
```

**This is the ARP performance fix made concrete.** The *old* code read
`/proc/net/arp` from disk **on every DNS query** to resolve the client's MAC —
a syscall in the hot path. Now the table is refreshed **once every 30s** on a
background goroutine, and `Lookup` is a plain in-memory map read under an
`RLock`. Same data, a fraction of the cost.

> `defer ticker.Stop()` matters: a `Ticker` that's never stopped leaks its
> internal goroutine/timer. Always stop tickers you create.

---

## 4.6 `context.Context`: cancellation that propagates

How do all those background loops know to stop? Through a **`context.Context`** —
Go's standard mechanism for cancellation and deadlines that flows across
goroutine boundaries.

Open [`internal/server/server.go`](../../backend/internal/server/server.go), `Run`:

```go
func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.arp.Start(ctx)               // ARP refresher watches ctx

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := s.startUDP(ctx); err != nil {   // listeners watch ctx
		return err
	}
	s.startTCP(ctx)
	s.startHTTP()
	s.startLogPruner(ctx)
	s.startSessionJanitor(ctx)

	<-signals                      // BLOCK here until Ctrl-C / SIGTERM
	slog.Info("shutting down")
	cancel()                       // cancel ctx -> every ctx.Done() fires

	s.closeListeners()
	s.cache.Close()                // stop cache eviction loop
	s.pool.Close()                 // stop forwarder health loop
	s.wg.Wait()                    // wait for all tracked goroutines to exit
	slog.Info("shutdown complete")
	return nil
}
```

The shutdown choreography, step by step:

1. `context.WithCancel` returns a `ctx` and a `cancel` func. Every background
   worker is started with this `ctx` and selects on `ctx.Done()`.
2. `<-signals` blocks the main goroutine until the OS delivers SIGINT/SIGTERM.
   `signal.Notify` routes those signals into the `signals` channel.
3. On signal, `cancel()` closes `ctx.Done()` for **everyone** — the ARP loop,
   the UDP/TCP accept loops, the log pruner, the session janitor all see it and
   return.
4. `closeListeners()` shuts the sockets; `cache.Close()` / `pool.Close()` stop
   the two loops that use their own `stop` channels rather than `ctx`.
5. `s.wg.Wait()` blocks until every goroutine tracked by the `WaitGroup` has
   exited (§4.7), so we don't close the DB out from under a running handler.

This is **graceful shutdown**: no work is abandoned mid-flight, no goroutine is
leaked, and the final log flush (from `Close`, called via `defer srv.Close()` in
`main`) still runs.

> Why two mechanisms (`ctx` *and* `stop` channels)? The listeners and periodic
> jobs are owned by the server and naturally take the request-scoped `ctx`. The
> cache and pool are standalone, reusable components with their own lifecycle, so
> they expose an explicit `Close()`. Both are valid; pick based on ownership.

---

## 4.7 `sync.WaitGroup`: waiting for goroutines to finish

A `WaitGroup` counts running goroutines so the main goroutine can wait for them.
The pattern, seen throughout `server`:

```go
s.wg.Add(1)                 // before starting: increment
go func() {
	defer s.wg.Done()       // on exit: decrement
	// ... work ...
}()
```

and at shutdown, `s.wg.Wait()` blocks until the count hits zero.

Three rules that prevent the classic bugs:

- **`Add` before `go`,** never inside the goroutine — otherwise `Wait` might run
  before `Add` and miss it.
- **`defer wg.Done()`** so it runs even if the goroutine returns early.
- The `WaitGroup` is a field on `Server` (`wg sync.WaitGroup`) and is used by
  pointer (methods on `*Server`). **Never copy a `WaitGroup`** — like a mutex,
  copying it breaks it.

---

## 4.8 Mental model summary

| Need | Tool | Example in repo |
|------|------|-----------------|
| Run work concurrently | `go func()` | per-packet UDP handler |
| Pass values between goroutines | channel | `logChan` query-log pipe |
| Signal "done"/"stop" | `close(chan struct{})` | `quit`, `flushDone`, `stop` |
| Multiplex events | `select` | log writer's 3-case select |
| Protect shared state | `sync.RWMutex` | cache, forwarder, arp |
| Cancel a tree of goroutines | `context.Context` | server shutdown |
| Wait for goroutines to finish | `sync.WaitGroup` | `s.wg.Wait()` |
| Non-blocking try | `select { case ...: default: }` | `LogQuery` send, shutdown check |

---

## Exercises

1. **Watch a race.** In `cache.go`, change `Get`'s `c.mu.Lock()` to
   `c.mu.RLock()` (and `Unlock`→`RUnlock`). Run `go test -race ./internal/modules/resolver/engine/cache/`.
   The race detector should fire on the `c.hits++`/`c.misses++` write under a
   read lock. Revert.
2. **Drop vs block.** Temporarily change `LogQuery`'s `select { case ...: default: }`
   to a plain blocking send `db.logChan <- log`. Reason about what happens to DNS
   latency if SQLite stalls. (Don't ship it.)
3. **Trace a shutdown.** Run the server, send a few `dig` queries, then Ctrl-C.
   In the logs you should see `shutting down` then `shutdown complete`. Add a
   `slog.Info("janitor stopping")` before the `return` in `startSessionJanitor`
   and confirm it prints on shutdown — proof that `cancel()` reached it.
4. **Ticker leak.** Remove `defer ticker.Stop()` from the ARP refresher and
   explain (in a comment) what resource leaks and when it would matter. Put it
   back.
5. **WaitGroup ordering.** Move `s.wg.Add(1)` to *inside* the goroutine in
   `startHTTP`. Reason about the race between `Add` and `Wait`. (Then revert —
   this is a real bug.)

[← Chapter 3](03-interfaces-and-architecture.md) · [Chapter 5: The DNS resolver pipeline →](05-dns-resolver-pipeline.md)
