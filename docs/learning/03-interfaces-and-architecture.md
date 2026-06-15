# Chapter 3 — Interfaces and Clean Architecture

> Goal: understand Go interfaces and how they enable **dependency inversion**,
> then see the whole app assembled in the composition root. This is the chapter
> that explains *why the code is shaped the way it is*.

---

## 3.1 Interfaces in Go are different (and better)

In Java/C# you write `class Cache implements CacheInterface`. In Go, **a type
satisfies an interface automatically if it has the right methods.** No
`implements` keyword. This is called **structural typing** (or "duck typing,
checked at compile time").

Open [`internal/modules/resolver/domain/ports.go`](../../backend/internal/modules/resolver/domain/ports.go):

```go
// Blocklist reports whether a domain is blocked.
type Blocklist interface {
	IsBlocked(domain string) bool
}

// CustomRecords resolves locally configured records for a domain and query type.
type CustomRecords interface {
	Lookup(domain string, qtype uint16) (ip string, found bool, existsOtherType bool)
}

type SteeringRules interface {
	Rules() []models.SteeringRule
}

type QueryLogger interface {
	LogQuery(models.QueryLog)
}

type MACResolver interface {
	Lookup(ip string) string
}
```

These are tiny interfaces — most have **one method**. That's deeply idiomatic Go:
*"The bigger the interface, the weaker the abstraction."* (Rob Pike). Small
interfaces are easy to implement, easy to fake in tests, and precise about what
a consumer actually needs.

### Who implements them?

The `*db.DB` type implements **all five**, but it never says so. Look at
[`internal/infrastructure/persistence/queries.go`](../../backend/internal/infrastructure/persistence/queries.go):

```go
func (db *DB) IsBlocked(domain string) bool { ... }     // satisfies Blocklist
func (db *DB) Rules() []models.SteeringRule { ... }     // satisfies SteeringRules
```

and [`internal/infrastructure/persistence/records.go`](../../backend/internal/infrastructure/persistence/records.go):

```go
func (db *DB) Lookup(domain string, qtype uint16) (string, bool, bool) { ... }  // CustomRecords
```

and [`internal/infrastructure/persistence/sqlite.go`](../../backend/internal/infrastructure/persistence/sqlite.go):

```go
func (db *DB) LogQuery(log models.QueryLog) { ... }     // QueryLogger
```

Because `*db.DB` has methods named `IsBlocked`, `Rules`, `Lookup`, and
`LogQuery` with the right signatures, it **automatically** satisfies
`Blocklist`, `SteeringRules`, `CustomRecords`, and `QueryLogger`. The compiler
verifies this where we assign it (Section 3.3). Nobody wrote `implements`.

The `*arp.Cache` type satisfies `MACResolver` the same way — it has a
`Lookup(ip string) string` method.

---

## 3.2 Dependency inversion: who depends on whom

Here's the crucial design idea. The `resolver` package defines the interfaces it
*needs*. It does **not** import `db`. Instead, `db` (an outer layer) provides
types that fit the resolver's interfaces.

```
   resolver  ──defines──►  Blocklist, CustomRecords, ... (ports.go)
       ▲                              ▲
       │ uses                         │ satisfies
       │                              │
   resolver.Resolve()            *db.DB  (in package db)
```

The arrow of **source-code dependency** points from `db` → `resolver` (db
imports the resolver's `models`), while the arrow of **control** at runtime goes
resolver → db. The dependency has been *inverted*. This is the "D" in SOLID, and
it's why:

- The resolver can be **unit-tested** with fake implementations (no SQLite, no
  network) — see [`resolver_test.go`](../../backend/internal/modules/resolver/engine/resolver_test.go)
  and Chapter 8.
- You could swap SQLite for Postgres by writing new types that satisfy the same
  interfaces, **without touching the resolver**.

> **Before/after:** the *old* resolver held a concrete `*db.DB` field and called
> `h.db.IsBlocked(...)` directly. It could not be tested without a real
> database. The refactor introduced these ports precisely to break that
> coupling. The tests in Chapter 8 simply couldn't exist before.

### Where do the interfaces live? (A key Go convention)

Notice the interfaces are defined in `resolver` (the **consumer**), not in `db`
(the **provider**). In Go, **interfaces belong to the code that uses them**, not
the code that implements them. This keeps providers ignorant of their consumers
and lets each consumer ask for exactly the narrow slice it needs.

Contrast with the *domain* packages, which define their own outbound ports —
e.g. [`internal/modules/records/domain/repository.go`](../../backend/internal/modules/records/domain/repository.go):

```go
type Repository interface {
	List(ctx context.Context) ([]Record, error)
	Save(ctx context.Context, record Record) error
	Delete(ctx context.Context, domain Domain) error
}

type Notifier interface {
	Notify(ctx context.Context, notifType, title, message string) error
}
```

The domain says "I need *something* that can persist records and notify users."
The `repositories` package provides the SQLite-backed `something`. Same
inversion, one layer in.

---

## 3.3 The composition root: `server.New`

All these interfaces and implementations have to be connected *somewhere*. That
somewhere is the **composition root** — the one place that knows the concrete
types and wires them together. Open
[`internal/server/server.go`](../../backend/internal/server/server.go):

```go
func New(cfg config.Config, static StaticFiles) (*Server, error) {
	// 1. Open the database (concrete *db.DB).
	database, err := db.Open(cfg.DBPath, db.Options{
		LogFlushInterval: cfg.LogFlushInterval,
		LogFlushSize:     cfg.LogFlushSize,
		SessionTTL:       cfg.SessionTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := database.InitAdmin(cfg.AdminEmail, cfg.AdminPass); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("init admin: %w", err)
	}

	cfg = applySavedSettings(cfg, database.GetSettings())

	// 2. Build the DNS infrastructure.
	arpCache := arp.NewCache(cfg.EnableMACLookup, cfg.ARPRefresh)
	dnsCache := cache.New(cfg.CacheSize)
	pool := forwarder.NewPool([]forwarder.Upstream{
		{Addr: cfg.UpstreamDNS, Timeout: 4 * time.Second, TLS: cfg.UpstreamTLS},
		{Addr: "8.8.8.8:853", Timeout: 6 * time.Second, TLS: cfg.UpstreamTLS},
	})

	// 3. Build the resolver, injecting *db.DB wherever a port is required.
	res := resolver.New(resolver.Deps{
		Blocklist: database,   // *db.DB satisfies Blocklist
		Records:   database,   //                    CustomRecords
		Steering:  database,   //                    SteeringRules
		Logger:    database,   //                    QueryLogger
		MAC:       arpCache,   // *arp.Cache satisfies MACResolver
		Cache:     dnsCache,
		Pool:      pool,
		BlockNX:   cfg.BlockNX,
	})

	// 4. Build the application services and the API handler.
	notifier := repositories.NewNotifications(database)
	apiHandler := handlers.New(
		database,
		res,
		recordsapp.NewService(repositories.NewRecords(database), notifier),
		blocklistapp.NewService(repositories.NewBlocklist(database), notifier),
		steeringapp.NewService(repositories.NewSteering(database), notifier),
	)

	return &Server{ /* hold all the components */ }, nil
}
```

Read step 3 carefully. We pass the **same `database` value** for four different
fields (`Blocklist`, `Records`, `Steering`, `Logger`). That compiles *only
because* `*db.DB` satisfies all four interfaces. This is the exact moment the
compiler checks structural conformance — if you deleted `db`'s `IsBlocked`
method, **this line** would fail to compile with "cannot use database as
resolver.Blocklist."

This is **constructor injection**: dependencies are passed in, never created
inside the component. The resolver doesn't `new` a database; it receives
something that *behaves like* the ports it needs.

### The `Deps` struct pattern

Instead of `resolver.New(blocklist, records, steering, logger, mac, cache, pool, blockNX)`
— eight positional args, easy to mis-order — we pass a single struct:

```go
type Deps struct {
	Blocklist Blocklist
	Records   CustomRecords
	Steering  SteeringRules
	Logger    QueryLogger
	MAC       MACResolver
	Cache     *cache.Cache
	Pool      *forwarder.Pool
	BlockNX   bool
}
```

Named fields are self-documenting and order-independent. This "functional
options-lite" / parameter-object pattern is common for constructors with many
dependencies.

---

## 3.4 The layers, top to bottom

Putting names to the architecture, a request flows through these layers:

| Layer | Packages | Knows about | Never imports |
|-------|----------|-------------|---------------|
| **Transport** | `interfaces/http/handlers`, `interfaces/dns` | HTTP/DNS wire formats | — |
| **Application** | `modules/{records,blocklist,steering}/application` | use cases, DTOs | HTTP, DNS, SQL |
| **Domain** | `modules/{records,blocklist,steering}/domain` | business rules, value objects | everything I/O |
| **Infrastructure** | `infrastructure/persistence`, each module's `infrastructure/`, `resolver/engine/{cache,forwarder,arp}` | SQLite, sockets, syscalls | HTTP handlers |

The golden rule: **dependencies point inward.** The domain is the center and
imports nothing from the outer rings. Let's see one vertical slice.

### Example: adding a custom DNS record

Trace [`internal/modules/records/application/service.go`](../../backend/internal/modules/records/application/service.go):

```go
func (s *Service) Add(ctx context.Context, domain, ip string) error {
	record, err := recordsdomain.New(domain, ip)   // DOMAIN validates
	if err != nil {
		return err
	}
	if err := s.repo.Save(ctx, record); err != nil { // INFRA persists (via port)
		return err
	}
	return s.notifier.Notify(ctx, "success", "DNS Record Created", ...)  // INFRA notifies
}
```

- The **domain** `recordsdomain.New` enforces "a record must have a valid domain
  and a valid IP." If it returns `ErrInvalidIP`, no database call happens.
- The **service** orchestrates: validate → save → notify. It depends only on the
  `Repository` and `Notifier` *interfaces* (defined in the domain), never on
  `db`.
- The concrete `repositories.Records` (which *does* know SQL) is injected at the
  composition root.

The HTTP handler (Chapter 7) sits above this and only translates JSON ↔ service
calls. Each layer has **one reason to change**, the Single Responsibility
Principle made physical via package boundaries.

---

## 3.5 Value objects and aggregates (a taste of DDD)

The domain uses two tactical patterns worth naming.

**Value object** — a small immutable type that is *always valid* because the
only way to construct it validates. [`records.IP`](../../backend/internal/modules/records/domain/value_obj.go):

```go
func NewIP(raw string) (IP, error) {
	v := strings.TrimSpace(raw)
	parsed := net.ParseIP(v)
	if parsed == nil {
		return IP{}, ErrInvalidIP
	}
	return IP{value: v, isV4: parsed.To4() != nil}, nil
}
```

You cannot create an `IP` holding `"not-an-ip"`. Once you have an `IP`, you never
re-check it. Invalid states are unrepresentable.

**Aggregate** — a cluster of value objects with invariants, constructed through
one root. [`records.Record`](../../backend/internal/modules/records/domain/entity.go):

```go
func New(domain, ip string) (Record, error) {
	d, err := NewDomain(domain)
	if err != nil {
		return Record{}, err
	}
	addr, err := NewIP(ip)
	if err != nil {
		return Record{}, err
	}
	return Record{domain: d, ip: addr}, nil
}
```

A `Record` is valid by construction. The steering domain
([`domain/steering/rule.go`](../../backend/internal/modules/steering/domain/rule.go)) does the
same for rules, validating the condition/action combination up front. This is
why the refactor pulled steering into a real domain package — so its rules are
validated in one place instead of scattered across handlers.

---

## Exercises

1. **Prove structural typing.** In `infrastructure/persistence/queries.go`, rename `IsBlocked` to
   `IsBlockedDomain`. Run `go build ./...`. The error appears in
   **`server.go`**, at the `Blocklist: database` line — not in the resolver.
   Read it, then rename back. You just watched the compiler enforce an
   interface.
2. **Add a port.** Suppose the resolver wanted to count queries via a metrics
   sink. Add `type Metrics interface { Inc(action string) }` to `ports.go`, a
   `Metrics` field to `Deps`, and a no-op implementation. Wire it in `server.go`.
   (You don't have to call it yet.)
3. **Find the inversion.** Open `modules/blocklist/application/service.go`. List every
   interface it depends on and find where each concrete implementation is
   created. (Answer: `Repository` and `Notifier`, both created in `server.New`.)
4. **Why DTOs?** `modules/records/application` returns `RecordDTO`, not the domain
   `Record`. Why doesn't it return the domain type directly to the HTTP layer?
   (Hint: the domain `Record`'s fields are unexported; and you don't want HTTP
   coupled to domain internals.)

[← Chapter 2](02-entrypoint-and-config.md) · [Chapter 4: Concurrency →](04-concurrency.md)
