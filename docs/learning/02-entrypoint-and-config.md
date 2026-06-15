# Chapter 2 — The Entry Point and Configuration

> Goal: follow the program from `func main()` through configuration loading,
> and learn flags vs environment variables, validation, and **build tags**.

---

## 2.1 `func main()` — small on purpose

Open [`cmd/dns-server/main.go`](../../backend/cmd/dns-server/main.go):

```go
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Setup(cfg.LogFormat, cfg.LogLevel)

	srv, err := server.New(cfg, server.StaticFiles{
		Embedded:   isEmbedded(),
		FileSystem: getFileSystem(),
	})
	if err != nil {
		slog.Error("initialize server", "error", err)
		os.Exit(1)
	}
	defer srv.Close()

	if err := srv.Run(); err != nil {
		slog.Error("run server", "error", err)
		os.Exit(1)
	}
}
```

Notice what `main` does **not** do: it doesn't open databases, bind sockets, or
parse flags itself. It **orchestrates** four steps:

1. `config.Load()` — read configuration.
2. `logger.Setup(...)` — configure logging.
3. `server.New(...)` — build everything (the "composition root", Chapter 3).
4. `srv.Run()` — block until shutdown.

This is a deliberate, idiomatic shape: **keep `main` thin.** All the real work
lives in testable packages; `main` just glues them.

### `package main` and the special `main` func

A package named `main` with a function `func main()` is what `go build` turns
into an executable. Every other package in this repo is a *library*; only
`cmd/dns-server` is `package main`.

### `defer srv.Close()`

`defer` schedules a call to run when the surrounding function returns — no
matter how it returns. Here it guarantees the database is closed on the way out.
You'll see `defer` everywhere for cleanup (closing files, rows, unlocking
mutexes). It runs in **LIFO** order if you have several.

### `os.Exit(1)` vs `return`

On a fatal startup error we call `os.Exit(1)` to terminate with a non-zero
status (so a supervisor like systemd knows we failed). **Caveat:** `os.Exit`
does *not* run deferred functions. That's acceptable here because if config or
init failed, there's nothing meaningful to clean up yet.

> **Try it:** run `go run ./cmd/dns-server` with **no** `OMNIDNS_ADMIN_PASSWORD`.
> You'll see `load config ... admin password is required` and exit code 1
> (`echo $?`). That's `config.Load()` returning an error.

---

## 2.2 Configuration: flags, environment, defaults

Open [`internal/shared/config/config.go`](../../backend/internal/shared/config/config.go). The `Config`
struct is the single source of truth for every tunable:

```go
type Config struct {
	DNSPort     int
	DNSAddr     string
	HTTPPort    int
	DBPath      string
	// ... security, sessions, ARP, log buffering ...
	AllowedOrigin    string
	SessionTTL       time.Duration
	EnableMACLookup  bool
	ARPRefresh       time.Duration
	LogFlushInterval time.Duration
	LogFlushSize     int
}
```

### Three layers of precedence

`Load()` resolves each setting with this priority: **command-line flag > environment variable > built-in default**. Look at the flag wiring:

```go
flag.IntVar(&cfg.DNSPort, "dns-port", envInt("OMNIDNS_DNS_PORT", 53), "DNS server port")
```

Read it inside-out:

1. `envInt("OMNIDNS_DNS_PORT", 53)` computes the **default**: the env var if
   set and parseable, otherwise `53`.
2. `flag.IntVar(&cfg.DNSPort, "dns-port", <default>, ...)` registers a `-dns-port`
   flag whose default is that value, writing into `cfg.DNSPort`.
3. `flag.Parse()` (later) reads `os.Args` and overrides with any flag the user
   actually passed.

So the **env var becomes the default that a flag can override**. That's the
clean way to support both 12-factor env config and ad-hoc CLI flags.

### The `env*` helpers — small, total functions

```go
func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

- `os.LookupEnv` returns `(value, ok)` — the `ok` distinguishes "set to empty"
  from "unset". This is the **comma-ok idiom**, ubiquitous in Go (maps, type
  assertions, channel receives all use it).
- If the var is missing *or* unparseable, we fall back. The function can't fail;
  it always returns an `int`. Total functions like this are easy to reason
  about.

There are sibling helpers `envStr`, `envBool`, `envDuration`. `envDuration`
parses strings like `"72h"` or `"30s"` via `time.ParseDuration` — that's why the
config uses `time.Duration` types instead of raw ints.

### Validation: fail fast, fail loud

```go
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.AdminPass) == "" {
		return fmt.Errorf("initial admin password is required; ...")
	}
	if len(cfg.AdminPass) < minPasswordLen {
		return fmt.Errorf("initial admin password must be at least %d characters", minPasswordLen)
	}
	if cfg.AllowedOrigin == "*" {
		return fmt.Errorf("CORS allowed origin must not be '*' with bearer-token auth; ...")
	}
	if cfg.SessionTTL <= 0 {
		return fmt.Errorf("session TTL must be positive")
	}
	return nil
}
```

`Load()` calls `Validate()` and refuses to start on bad config. This is a
**security control expressed as code**: you literally cannot boot OmniDNS with
a wildcard CORS origin or a weak admin password. The refactor added these checks
because the old code silently shipped `Allow-Origin: *`.

> **Design note:** `Validate` is a method on `Config` with a *value* receiver —
> it only reads. It returns the *first* problem it finds. For a tool with this
> few knobs, first-error is fine; a larger system might accumulate all errors.

---

## 2.3 Build tags: two binaries from one codebase

This is a genuinely advanced Go feature, and OmniDNS uses it in two places.

### UI embedding (prod vs dev)

There are two files that define the *same* functions but compile under
different conditions. [`cmd/dns-server/embed_prod.go`](../../backend/cmd/dns-server/embed_prod.go):

```go
//go:build embed

package main

import "embed"
//go:embed static/*
var staticFiles embed.FS

func getFileSystem() http.FileSystem { /* serve embedded files */ }
func isEmbedded() bool { return true }
```

[`cmd/dns-server/embed_dev.go`](../../backend/cmd/dns-server/embed_dev.go):

```go
//go:build !embed

package main

func getFileSystem() http.FileSystem { return http.Dir("./static") }
func isEmbedded() bool { return false }
```

The first line, `//go:build embed` (and `!embed`), is a **build constraint**.

- `go build ./...` (no tags) compiles the **dev** file → UI served from
  `./static` on disk. Great for development; edit the UI and refresh.
- `go build -tags embed ./...` compiles the **prod** file → the React build is
  baked into the binary via `//go:embed`, producing a single self-contained
  executable. Great for deployment.

Both files declare `getFileSystem()` and `isEmbedded()`, but **only one is ever
compiled**, so there's no duplicate-symbol error. `main.go` just calls those
functions without knowing or caring which build it's in. The build tag picks the
implementation at *compile* time, with zero runtime cost.

> **Try it:**
> ```bash
> go build ./cmd/...                # ok (dev)
> go build -tags embed ./cmd/...    # fails: needs cmd/dns-server/static/
> mkdir -p cmd/dns-server/static && echo '<html></html>' > cmd/dns-server/static/index.html
> go build -tags embed ./cmd/...    # ok (prod)
> rm -rf cmd/dns-server/static
> ```
> `//go:embed` requires the `static/` directory to exist at build time — in CI,
> the frontend is built first, then this runs.

### Platform-specific code (Linux vs the rest)

The ARP cache (Chapter 4) reads `/proc/net/arp`, which only exists on Linux.
Same technique, in [`internal/modules/resolver/engine/arp/`](../../backend/internal/modules/resolver/engine/arp/):

- [`table_linux.go`](../../backend/internal/modules/resolver/engine/arp/table_linux.go) → `//go:build linux`,
  parses `/proc/net/arp`.
- [`table_other.go`](../../backend/internal/modules/resolver/engine/arp/table_other.go) → `//go:build !linux`,
  returns an empty table.

So the project **compiles on macOS/Windows** (where it returns empty MACs) and
does the real lookup on Linux — without a single `if runtime.GOOS == "linux"`
at runtime. The constraint is resolved by the compiler.

This is the idiomatic Go answer to "how do I write portable code that uses
OS-specific APIs": split per-platform implementations behind a common function
signature, selected by build tags.

---

## 2.4 The full boot sequence

Tracing it end to end, here's what happens when you start OmniDNS:

```
main()
 ├─ config.Load()           parse env + flags, Validate(); abort on bad config
 ├─ logger.Setup()          install slog default handler
 ├─ server.New(cfg, static) open DB, init admin, build resolver + API (Ch 3)
 └─ server.Run()            start UDP/TCP/HTTP + background workers, block on signal
```

By the time `Run()` blocks waiting for `SIGINT`, every component is wired and
listening. We'll dissect `New` and `Run` in Chapters 3 and 4.

---

## Exercises

1. **Precedence proof.** Start the server three ways and observe the HTTP port
   in the log line `listening protocol=HTTP`:
   - default → `8080`
   - `OMNIDNS_HTTP_PORT=9090 go run ./cmd/dns-server ...` → `9090`
   - `OMNIDNS_HTTP_PORT=9090 go run ./cmd/dns-server --http-port 7070 ...` → `7070`
2. **Break validation.** Run with `OMNIDNS_ALLOWED_ORIGIN='*'`. Confirm the
   server refuses to start and read the exact message. Why is `*` dangerous with
   bearer tokens? (Hint: Chapter 7, CORS.)
3. **Add a knob.** Add a `CacheNegativeTTL time.Duration` field to `Config`,
   wire a `--neg-ttl` flag and `OMNIDNS_NEG_TTL` env var with a default of
   `60s`. Don't use it yet — just make `go build` pass. (You'll wire it into the
   cache in Chapter 4's exercises.)
4. **Read a build tag.** Without running it, predict: does
   `go vet ./internal/modules/resolver/engine/arp/` on macOS check `table_linux.go`? Then verify with
   `go vet -tags linux ...` vs without. (Answer: by default `go vet` checks the
   files for the *host* OS; `table_linux.go` is skipped off-Linux unless you
   cross-target.)

[← Chapter 1](01-go-foundations.md) · [Chapter 3: Interfaces & architecture →](03-interfaces-and-architecture.md)
