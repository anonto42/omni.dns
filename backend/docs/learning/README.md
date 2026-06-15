# Learn Go by Reading OmniDNS — A Project-Based Course

This is a **learning resource** built around the real OmniDNS backend in this
repository. Instead of toy examples, every concept is taught by pointing at
actual code you can open, run, modify, and break.

By the end you will understand the whole backend **line by line**, and along the
way you will have learned idiomatic Go: packages, interfaces, goroutines,
channels, `context`, `database/sql`, HTTP servers, build tags, and testing.

---

## What is OmniDNS?

OmniDNS is a self-hosted DNS server for a home network (a Pi-hole-style tool).
It:

- listens for **DNS queries** over UDP and TCP,
- decides what to do with each query — **block** it, return a **custom record**,
  apply a **steering rule**, serve from **cache**, or **forward** it to an
  upstream resolver over DNS-over-TLS,
- records every query in **SQLite**, and
- exposes a **REST API** (and an embedded React UI) to manage everything.

That single program touches networking, concurrency, databases, HTTP, and
configuration — which makes it an ideal vehicle for learning Go end to end.

---

## How to use this course

1. **Read in order.** The chapters build on each other.
2. **Keep the code open beside the doc.** Every section cites real files and
   line ranges like [`internal/dns/resolver/resolver.go`](../../internal/dns/resolver/resolver.go).
   Open them.
3. **Run things.** Each chapter has a *"Try it"* box with commands.
4. **Do the exercises** at the end of each chapter. They are designed to make
   you change the code, not just read it.

### Prerequisites

- Go 1.25+ installed (`go version`).
- Basic programming experience in *some* language. No prior Go required.

### Setup

```bash
cd backend
go build ./...          # compile everything
go test ./...           # run the test suite
go test -race ./...     # run tests with the data-race detector
```

To run the server locally (it needs an admin password):

```bash
cd backend
OMNIDNS_ADMIN_PASSWORD=changeme123 \
  go run ./cmd/dns-server --dns-port 5354 --http-port 8080
```

> We use `--dns-port 5354` so you don't need root to bind the privileged port 53.
> Query it with: `dig @127.0.0.1 -p 5354 example.com`

---

## The codebase map

The backend follows **clean architecture**: dependencies point *inward*, toward
the domain. Outer layers (HTTP, DNS, SQLite) depend on inner layers (application
services, domain), never the reverse.

```
cmd/dns-server/            Program entry point + build-tag UI embedding
  main.go                  Wires config -> logger -> server, then runs it
  embed_prod.go            //go:build embed   — bakes the React UI into the binary
  embed_dev.go             //go:build !embed  — serves ./static from disk

internal/
  config/                  Load + validate configuration (flags + env)
  logger/                  Configure the slog structured logger

  server/                  Composition root: builds and owns every component
    server.go              Dependency injection + lifecycle (start/stop)
    listeners.go           UDP/TCP/HTTP listeners
    static.go              Serve the single-page app

  dns/                     Everything DNS
    handler.go             Wire-format packet <-> resolver adapter
    resolver/              The resolution pipeline (the heart of the app)
      resolver.go          The pipeline: blocklist -> steering -> records -> cache -> upstream
      responses.go         Build A/AAAA/NXDOMAIN/NODATA replies
      matching.go          Steering condition matching (domain/CIDR/time)
      ports.go             Interfaces the resolver depends on (inversion of control)
    cache/                 qtype-keyed LRU+TTL cache
    forwarder/             Upstream pool with health-checked failover
    arp/                   Cached IP->MAC lookups (Linux build tags)

  api/                     HTTP REST layer
    router.go              Route table
    middleware/            CORS, auth, request-ID
    handlers/              One file per resource (auth, records, blocklist, ...)

  application/             Use-case orchestration (services)
    records/  blocklist/  steering/

  domain/                  Pure business rules, no I/O
    records/  blocklist/  steering/

  db/                      SQLite persistence
    sqlite.go              Connection, schema, migrations, buffered log writer
    auth.go  queries.go  records.go
    models/                Plain data structs shared across layers
    repositories/          Implement the domain's outbound ports
```

### The dependency rule, drawn

```
   HTTP handlers ─┐
                  ├─► application services ─► domain (pure rules)
   DNS resolver ──┘            ▲
                               │ (interfaces / "ports")
   db, cache, forwarder, arp ──┘  (implement the ports)
```

The domain knows nothing about HTTP, DNS, or SQLite. That is what makes it
testable and what we will keep returning to.

---

## Suggested learning path

| Chapter | You will learn | Anchor code |
|--------:|----------------|-------------|
| [1](01-go-foundations.md) | Packages, modules, types, methods, errors, `slog` | `models`, `domain` |
| [2](02-entrypoint-and-config.md) | `main`, flags vs env, build tags, validation | `cmd/`, `config/` |
| [3](03-interfaces-and-architecture.md) | Interfaces, dependency injection, DDD layers | `resolver/ports.go`, `server/` |
| [4](04-concurrency.md) | Goroutines, channels, mutexes, `context`, shutdown | `cache/`, `forwarder/`, `db/` |
| [5](05-dns-resolver-pipeline.md) | The full resolver, line by line; AAAA/NODATA | `dns/` |
| [6](06-persistence.md) | `database/sql`, SQLite, migrations, buffering | `db/` |
| [7](07-http-api.md) | `net/http`, chi, middleware, JSON, auth | `api/` |
| [8](08-testing-and-optimizations.md) | Testing, the race detector, perf wins, exercises | `*_test.go` |

Start with [Chapter 1 →](01-go-foundations.md)

---

## A note on how this code got here

This backend was recently **refactored** (commit `713107d`) from a flatter,
"god-object" layout into the clean architecture you see now, and several real
bugs were fixed (sessions never expired, CORS was wide open, MAC lookups did a
syscall per query, AAAA queries were mishandled). Throughout the course we will
contrast *before* and *after* so you learn not just **what** the code does but
**why it is shaped this way** — the most valuable thing an engineer can learn.
