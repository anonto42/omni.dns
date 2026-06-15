# ADR 002: Modular Clean Architecture for the Backend

**Date:** 2026-06-15
**Status:** Accepted

### Context

The backend began as a flat layout with a `db.DB` "god object" that the DNS
resolver and HTTP handlers depended on directly. A first refactor introduced
clean-architecture layers (`domain`, `application`, `db/repositories`,
`api`, `dns`, `server`) but kept them as **global, technology-named** folders:
all domains lived together under `domain/`, all services under `application/`,
all persistence under `db/`. As the project grows (more DNS features, more API
resources), a feature's code is scattered across many top-level folders, making
a single feature hard to locate, reason about, or extract.

### Decision

Reorganize `internal/` into **per-feature modules**, each internally split into
the same three layers, plus shared packages, a transport layer, and a
composition root:

```
internal/
  shared/{config,logger,models}          importable by any layer
  modules/
    resolver/{domain, engine/{...,cache,forwarder,arp}}
    blocklist|records|steering/{domain, application, infrastructure}
    notification/infrastructure
  infrastructure/persistence/            SQLite connection + shared db plumbing
  interfaces/{http/{router,middleware,handlers}, dns}
  server/                                composition root (wires modules)
```

The **dependency rule** is enforced by package boundaries:

- `interfaces` → `application` → `domain`
- `infrastructure` → `domain`
- `shared/*` may be imported by any layer
- no layer imports a layer above it

Each module's `domain` package defines its entities, value objects, and
outbound port interfaces (`Repository`, `Notifier`); its `application` package
orchestrates use cases; its `infrastructure` package implements the ports
against SQLite. The resolver module keeps its ports in `domain/` and its engine
(pipeline, cache, forwarder, ARP) under `engine/`, re-exporting the ports via
type aliases so the hot-path code stays unqualified.

### Consequences

* **Pros:**
  * A feature is one directory — easy to find, change, test, or extract into its
    own service later.
  * The dependency rule is visible and grep-able; the domain of each module
    stays free of HTTP/DNS/SQL imports, keeping it unit-testable with fakes.
  * New features follow an obvious template (`domain` / `application` /
    `infrastructure`), reducing bikeshedding about where code goes.
* **Cons:**
  * More packages and deeper import paths; consumers alias imports
    (`blocklistdomain`, `recordsinfra`, …) to disambiguate same-named packages.
  * A large one-time move touching nearly every import. Mitigated by using
    `git mv` (history preserved) and verifying `go build`, `go vet`,
    `go test -race`, and the embedded build at each step.
  * Pure mechanical change — **no behavior change** — so the risk is limited to
    compilation, which the test suite and CI checks cover.

### Notes

This decision builds on ADR 001 (single-binary embed): the `interfaces` and
`server` layers still serve the embedded SPA exactly as before; only the
internal package organization changed. The companion learning course in
`docs/learning/` was updated to describe this layout.
