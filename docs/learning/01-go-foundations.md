# Chapter 1 — Go Foundations, Through This Project

> Goal: understand Go's building blocks — modules, packages, types, methods,
> errors, and logging — by reading real code in this repo.

---

## 1.1 Modules and packages

Every Go project is a **module**, declared in `go.mod`:

```go
// backend/go.mod
module github.com/sohidul/dns-server

go 1.25.0

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/miekg/dns v1.1.72
	github.com/stretchr/testify v1.9.0
	modernc.org/sqlite v1.37.0
)
```

- The `module` line is the **import path prefix**. Every internal package is
  imported as `github.com/sohidul/dns-server/internal/...`.
- `require` lists direct dependencies. `go.sum` pins their cryptographic hashes.
- `go 1.25.0` is the language version.

A **package** is a directory of `.go` files that share the same `package` name
on their first line. Open these two files:

- [`internal/dns/cache/cache.go`](../../backend/internal/dns/cache/cache.go) → `package cache`
- [`internal/dns/cache/cache_test.go`](../../backend/internal/dns/cache/cache_test.go) → `package cache`

Both are in `internal/dns/cache/`, both say `package cache`. They are the same
package split across files. **Files are an organizational convenience; the
package is the real unit.**

### Why `internal/`?

The directory name `internal` is special to the Go toolchain: packages under
`internal/` can only be imported by code rooted at the parent of `internal/`.
So nobody outside this module can import our `internal/db`. This is how Go
enforces "these are private implementation packages."

> **Try it:** rename `internal` to `lib` and run `go build ./...`. It still
> works, but now the privacy guarantee is gone. Rename it back.

### Import paths and aliases

When two imported packages would collide, or a name is ambiguous, Go lets you
**alias**. See [`internal/server/server.go`](../../backend/internal/server/server.go):

```go
import (
	"github.com/sohidul/dns-server/internal/api"
	apimw "github.com/sohidul/dns-server/internal/api/middleware"
	appdns "github.com/sohidul/dns-server/internal/dns"
	blocklistapp "github.com/sohidul/dns-server/internal/application/blocklist"
	// ...
)
```

`apimw` and `appdns` are aliases. We alias `internal/dns` to `appdns` because
`dns` is already taken by the third-party `github.com/miekg/dns` package used
elsewhere. **Aliasing is about readability, not magic.**

---

## 1.2 Exported vs unexported: capitalization *is* the access modifier

Go has no `public`/`private` keywords. Instead, **an identifier starting with a
capital letter is exported** (visible outside its package); lowercase is
package-private.

Open [`internal/dns/cache/cache.go`](../../backend/internal/dns/cache/cache.go):

```go
type Cache struct {        // Exported: other packages can use cache.Cache
	mu     sync.RWMutex    // unexported: only this package touches it
	items  map[string]*list.Element
	max    int
	hits   int64
}

func New(max int) *Cache { ... }   // Exported constructor
func (c *Cache) Get(...) ...        // Exported method
func (c *Cache) evictLoop()         // unexported helper
```

This is the single most important Go convention to internalize:

- The **API** of a package is its exported identifiers.
- Everything lowercase is free to change without breaking callers.

Notice `Cache`'s fields are all lowercase. Callers can't poke at `c.items`
directly — they must go through `Get`, `Set`, `Size`, etc. That is
**encapsulation**, done with capitalization.

---

## 1.3 Structs, methods, and pointer receivers

A `struct` groups fields. A **method** is a function with a *receiver*.

From [`internal/db/models/models.go`](../../backend/internal/db/models/models.go):

```go
type QueryLog struct {
	ID        int64
	Timestamp time.Time
	Domain    string
	ClientIP  string
	Action    Action
	// ...
}
```

This is a *plain data* struct — no methods, just fields. It's a "DTO" (data
transfer object) that flows between layers.

Now a struct *with* methods, from [`internal/domain/records/value_obj.go`](../../backend/internal/domain/records/value_obj.go):

```go
type IP struct {
	value string
	isV4  bool
}

func (ip IP) String() string { return ip.value }   // value receiver

func (ip IP) QType() uint16 {                       // value receiver
	if ip.isV4 {
		return TypeA
	}
	return TypeAAAA
}
```

`(ip IP)` is a **value receiver**: the method gets a *copy* of the `IP`. That's
fine here because `IP` is small and immutable.

Compare with [`internal/dns/cache/cache.go`](../../backend/internal/dns/cache/cache.go):

```go
func (c *Cache) Set(domain string, qtype uint16, ips []string, ttl uint32) {
	c.mu.Lock()
	// ... mutates c.items, c.order
}
```

`(c *Cache)` is a **pointer receiver**. It must be a pointer because:

1. The method **mutates** the cache (a copy would throw the changes away), and
2. `Cache` contains a `sync.RWMutex`, which **must not be copied** — copying a
   mutex is a bug.

**Rule of thumb:** use a pointer receiver if the method mutates the receiver, if
the struct is large, or if the struct contains a mutex/`sync` type. Be
consistent within a type.

---

## 1.4 Named types and constants

From [`internal/db/models/models.go`](../../backend/internal/db/models/models.go):

```go
type Action string

const (
	ActionForwarded Action = "forwarded"
	ActionBlocked   Action = "blocked"
	ActionCustom    Action = "custom"
	ActionCached    Action = "cached"
	ActionError     Action = "error"
)
```

`Action` is a **named type** based on `string`. Why not just use `string`?

- **Type safety.** A function taking `Action` won't accept an arbitrary string
  by mistake. You get a compile error, not a runtime surprise.
- **Documentation.** The set of valid values lives in one `const` block.
- **Methods.** You could attach methods to `Action` later if needed.

The `const (...)` block groups related constants. This is Go's lightweight
substitute for enums.

> Compare with the steering query types in
> [`internal/domain/records/value_obj.go`](../../backend/internal/domain/records/value_obj.go):
> ```go
> const (
> 	TypeA    uint16 = 1
> 	TypeAAAA uint16 = 28
> )
> ```
> These mirror the real DNS protocol numbers (A=1, AAAA=28). Constants give the
> magic numbers names.

---

## 1.5 The zero value: Go has no "uninitialized"

Every type in Go has a **zero value**, and a freshly declared variable always
has it: `0` for numbers, `""` for strings, `false` for bools, `nil` for
pointers/slices/maps/interfaces, and a struct whose fields are each their zero
value.

This shapes idiomatic Go. Look at how a `QueryLog` is built in the resolver
([`internal/dns/resolver/resolver.go`](../../backend/internal/dns/resolver/resolver.go)):

```go
base := models.QueryLog{
	Domain:     domain,
	ClientIP:   clientIP,
	MACAddress: r.mac.Lookup(clientIP),
	Protocol:   protocol,
	QueryType:  qtypeStr,
}
```

We set only five fields. The rest (`ID`, `ResolvedIP`, `AnswerCount`, `TTL`,
`LatencyMs`, ...) are left at their zero values — which is exactly what we want
for a query that hasn't been resolved yet. Then each pipeline step copies
`base` and fills in the outcome:

```go
entry := base
entry.Action = models.ActionBlocked
entry.ResponseCode = "BLOCKED"
```

`entry := base` is a **struct copy** (structs are value types). Mutating `entry`
does not touch `base`, so the next pipeline step starts from a clean template.
This is a clean, allocation-cheap pattern you'll see throughout.

---

## 1.6 Errors are values

Go has no exceptions. Functions that can fail return an `error` as their last
result, and you check it explicitly.

From [`internal/config/config.go`](../../backend/internal/config/config.go):

```go
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.AdminPass) == "" {
		return fmt.Errorf("initial admin password is required; set OMNIDNS_ADMIN_PASSWORD or --admin-password")
	}
	if len(cfg.AdminPass) < minPasswordLen {
		return fmt.Errorf("initial admin password must be at least %d characters", minPasswordLen)
	}
	// ...
	return nil   // nil means "no error"
}
```

And the caller, in [`internal/server/server.go`](../../backend/internal/server/server.go):

```go
database, err := db.Open(cfg.DBPath, db.Options{...})
if err != nil {
	return nil, fmt.Errorf("open database: %w", err)
}
```

Two things to learn here:

### `if err != nil` is the heartbeat of Go

You will write this thousands of times. It's verbose on purpose: every failure
path is visible at the call site. There's no hidden control flow.

### Error wrapping with `%w`

`fmt.Errorf("open database: %w", err)` **wraps** the original error, adding
context ("open database:") while preserving the original underneath. Callers can
later test the chain with `errors.Is` / `errors.As`. We use exactly that in the
HTTP layer — see [`internal/api/handlers/records.go`](../../backend/internal/api/handlers/records.go):

```go
func writeRecordError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, recordsdomain.ErrInvalidDomain):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid domain name"})
		return true
	case errors.Is(err, recordsdomain.ErrInvalidIP):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid IP address"})
		return true
	}
	return false
}
```

`errors.Is(err, recordsdomain.ErrInvalidDomain)` walks the wrap chain looking
for that **sentinel error**, defined once in
[`internal/domain/records/value_obj.go`](../../backend/internal/domain/records/value_obj.go):

```go
var (
	ErrInvalidDomain = errors.New("invalid domain name")
	ErrInvalidIP     = errors.New("invalid IP address")
)
```

**Pattern:** the *domain* defines named errors; the *HTTP layer* maps them to
status codes. The domain doesn't know what HTTP is — it just says "invalid
domain," and the edge decides that means `400`.

---

## 1.7 Structured logging with `slog`

This project never uses `fmt.Println` for logging. It uses the standard
library's structured logger, `log/slog`. Setup lives in
[`internal/logger/logger.go`](../../backend/internal/logger/logger.go):

```go
func Setup(format, levelName string) {
	opts := &slog.HandlerOptions{Level: parseLevel(levelName)}
	if format == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, opts)))
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
}
```

`slog.SetDefault` installs a process-wide logger, so anywhere in the codebase we
can write:

```go
slog.Info("listening", "protocol", "UDP", "port", s.cfg.DNSPort)
slog.Warn("upstream failure", "addr", up.Addr, "error", err)
slog.Debug("dns query", "domain", domain, "qtype", qtypeStr, "client", clientIP)
```

The arguments after the message are **key/value pairs**. Structured logs are
machine-parseable: in `json` mode the line above becomes
`{"level":"INFO","msg":"listening","protocol":"UDP","port":5354}`, which a log
aggregator can index and filter. That's why we prefer it over string
formatting.

**Levels** (`Debug < Info < Warn < Error`) let you turn down noise in
production. Run with `--log-level debug` to see every query; `--log-level warn`
to see only problems.

---

## 1.8 Putting it together: read one whole small file

You now know enough to read an entire file unaided. Open
[`internal/domain/blocklist/value_obj.go`](../../backend/internal/domain/blocklist/value_obj.go)
and identify, by yourself:

1. The package name and what it represents.
2. The exported vs unexported identifiers.
3. The sentinel error and where it's returned.
4. The value-object pattern (`Domain` wraps a validated `string`; you can't
   construct an invalid one because the only constructor, `NewDomain`,
   validates first).
5. Why `String()` uses a value receiver.

---

## Exercises

1. **Add a constant.** In `models`, add `ActionRefused Action = "refused"`. Run
   `go build ./...`. Nothing breaks (it's unused) — now find where actions are
   set in the resolver and imagine where you'd use it.
2. **Trigger error wrapping.** Temporarily change `db.Open` to return
   `fmt.Errorf("boom")` and run the server with a password set. Read the log
   line `main.go` prints. Now wrap it (`%w`) and observe the difference.
3. **Switch log format.** Run the server with `--log-format json --log-level debug`
   and `dig` a domain at it. Read the structured query-log line.
4. **Receiver experiment.** Change `func (ip IP) String()` to a pointer receiver
   `func (ip *IP)` and run `go build ./...`. Read the errors — they teach you
   where `IP` is used as a value.

[← Index](README.md) · [Chapter 2: Entry point & config →](02-entrypoint-and-config.md)
