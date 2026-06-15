# Chapter 7 — The HTTP API: `net/http`, chi, Middleware, JSON

> Goal: build a complete picture of the REST layer — routing with chi,
> middleware (CORS, auth, request-ID), JSON encode/decode, and how handlers stay
> thin by delegating to application services.

---

## 7.1 `net/http` fundamentals

Everything in Go's web stack reduces to one interface:

```go
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

- `*http.Request` (`r`) is the incoming request: method, URL, headers, body.
- `http.ResponseWriter` (`w`) is how you write the response: headers, status,
  body.

A plain function with that signature is a handler via `http.HandlerFunc`. Our
simplest one, in [`internal/api/handlers/handlers.go`](../../internal/api/handlers/handlers.go):

```go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

That's the whole contract. Frameworks just make routing and composition nicer.

---

## 7.2 Routing with chi

We use [`go-chi/chi`](https://github.com/go-chi/chi), a lightweight router that's
100% compatible with `net/http`. The route table is in
[`internal/api/router.go`](../../internal/api/router.go):

```go
func RegisterRoutes(r chi.Router, database *db.DB, h *handlers.Handler) {
	r.Get("/health", h.Health)        // public
	r.Post("/api/login", h.Login)     // public

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(database))   // everything below requires auth

		r.Get("/status", h.GetStatus)
		r.Get("/logs", h.GetLogs)
		r.Delete("/logs", h.ClearLogs)

		r.Get("/records", h.GetRecords)
		r.Post("/records", h.AddRecord)
		r.Delete("/records", h.DeleteRecord)
		// ... blocklist, settings, steering, notifications, profile ...
	})
}
```

Two things to learn:

- **`r.Get`, `r.Post`, `r.Delete`** map an (HTTP method + path) to a handler.
  REST conventions: GET reads, POST creates, PUT updates, DELETE removes.
- **`r.Route("/api", func(r chi.Router) {...})`** creates a **subrouter** with its
  own middleware. `r.Use(middleware.Auth(...))` applies authentication to every
  route declared inside — but **not** to `/health` or `/api/login`, which sit
  outside the group. This is how you protect a whole section of the API in one
  line while keeping login public.

---

## 7.3 Middleware: wrapping handlers

Middleware is a function that **takes a handler and returns a handler**, doing
work before/after the inner one. The signature is always
`func(http.Handler) http.Handler`. The chain is assembled in
[`internal/server/server.go`](../../internal/server/server.go):

```go
r := chi.NewRouter()
r.Use(chimw.Logger)                       // chi's request logger
r.Use(apimw.RequestID)                    // our request-ID
r.Use(apimw.CORS(s.cfg.AllowedOrigin))    // our CORS
api.RegisterRoutes(r, s.database, s.api)
```

Requests flow **outside-in** through `r.Use(...)` middleware, then hit the route,
then (for `/api/*`) the `Auth` middleware, then the handler. Responses flow back
out.

### CORS — the security fix

The old code sent `Access-Control-Allow-Origin: *`, which is dangerous with
bearer tokens (any website could call your API on your behalf). The new CORS
middleware, in [`internal/api/middleware/middleware.go`](../../internal/api/middleware/middleware.go),
echoes a *specific* origin:

```go
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)    // preflight: answer and stop
				return
			}
			next.ServeHTTP(w, r)                 // otherwise continue the chain
		})
	}
}
```

Notice the shape: `CORS(allowedOrigin)` is a **closure factory**. It captures
`allowedOrigin` and returns the actual middleware. That's how you parameterize
middleware with config. The `OPTIONS` branch handles the browser's CORS
**preflight** without falling through to a handler. (Recall from Chapter 2 that
`config.Validate` *refuses to start* if `allowedOrigin == "*"` — defense in
depth.)

### Auth — bearer tokens + context

```go
func Auth(verifier SessionVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			email, ok := verifier.VerifySession(token)   // Ch 6: checks expiry!
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, email)
			next.ServeHTTP(w, r.WithContext(ctx))         // pass identity downstream
		})
	}
}
```

`Auth` depends on a one-method interface `SessionVerifier` (Chapter 3 again — the
middleware doesn't import `db`, it just needs *something* that can
`VerifySession`). On success it stores the user's email in the **request
context** and calls `r.WithContext(ctx)` to attach it. Downstream handlers read
it back:

```go
email, ok := middleware.UserFromContext(r.Context())
```

`context.WithValue` is the standard way to carry **request-scoped** data (the
authenticated user, a request ID) without threading extra parameters through
every function. Use it sparingly — for request metadata, not for passing core
dependencies.

### RequestID — correlation

`RequestID` assigns each request an ID (honoring an inbound `X-Request-ID`),
echoes it on the response, and stashes it in context. In a real deployment you'd
include it in every log line to trace one request across components.

---

## 7.4 JSON: encode and decode

The API speaks JSON. Two shared helpers in `handlers.go` keep every handler tidy.

**Encoding a response:**

```go
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encode json response failed", "error", err)
	}
}
```

`json.NewEncoder(w).Encode(data)` serializes `data` straight to the response
writer. **Order matters:** set headers, then `WriteHeader(status)`, then write the
body. Once you write the body you can't change the status.

**Decoding a request body (safely):**

```go
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)   // cap body size
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}
```

`http.MaxBytesReader(w, r.Body, 1<<16)` caps the request body at 64 KiB — a
**denial-of-service guard** so a client can't make you allocate gigabytes. The
helper returns a `bool` so handlers read cleanly:

```go
var body models.AddRecordRequest
if !decodeJSON(w, r, &body) {
	return        // decodeJSON already wrote the 400
}
```

### Struct tags drive (de)serialization

How does Go know the JSON field names? **Struct tags**, in
[`internal/db/models/models.go`](../../internal/db/models/models.go):

```go
type AddRecordRequest struct {
	Domain string `json:"domain" example:"mydevice.local"`
	IP     string `json:"ip" example:"192.168.1.100"`
}
```

The `` `json:"domain"` `` tag maps the exported Go field `Domain` to the JSON key
`domain`. Tags are metadata read via reflection by `encoding/json` (and the
`example:` tag by Swagger tooling). This is how Go bridges its capitalized-export
convention with lowercase JSON conventions.

---

## 7.5 A handler end to end

Put it together — `AddRecord` in
[`internal/api/handlers/records.go`](../../internal/api/handlers/records.go):

```go
func (h *Handler) AddRecord(w http.ResponseWriter, r *http.Request) {
	var body models.AddRecordRequest
	if !decodeJSON(w, r, &body) {
		return                                  // 1. parse + validate JSON shape
	}
	if err := h.records.Add(r.Context(), body.Domain, body.IP); err != nil {  // 2. delegate
		if writeRecordError(w, err) {           // 3a. domain error -> 400
			return
		}
		slog.Error("add record failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to add record"})  // 3b.
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})   // 4. success
}
```

The handler is **thin on purpose**. It does only HTTP concerns:

1. Decode and size-limit the body.
2. Call the **application service** (`h.records.Add`), passing the request
   `context`.
3. Translate errors: domain validation errors → `400` (via `writeRecordError`'s
   `errors.Is` checks from Chapter 1); unexpected errors → `500`.
4. Encode the success response.

All the *business logic* — validating the domain/IP, persisting, notifying —
lives in the service and domain (Chapter 3). The handler is glue between HTTP and
the use case. This is why the handlers are split into small per-resource files
(`auth.go`, `records.go`, `blocklist.go`, `steering.go`, ...): each is a thin
translation layer for one resource.

---

## 7.6 Serving the single-page app

Non-API routes serve the React UI. [`internal/server/static.go`](../../internal/server/static.go)
has a small but important trick — the SPA fallback:

```go
func spaFileServer(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			r.URL.Path = "/"          // unknown path -> serve index.html
			fileServer.ServeHTTP(w, r)
			return
		}
		defer f.Close()
		// ... if it's a real file, serve it normally ...
		fileServer.ServeHTTP(w, r)
	})
}
```

A single-page app does its own client-side routing. If the user reloads
`/dashboard`, there's no `dashboard` file on disk — so we fall back to
`index.html` and let React Router handle it. Without this, deep links would
404. The `http.FileSystem` it serves is either embedded (prod) or `http.Dir`
(dev) — that's the build-tag choice from Chapter 2 flowing through.

---

## 7.7 The request lifecycle, end to end

```
Browser ──HTTP──► chi.Router
   │
   ├─ chimw.Logger          (log start/end)
   ├─ apimw.RequestID       (assign X-Request-ID -> context)
   ├─ apimw.CORS            (origin allowlist; answer OPTIONS preflight)
   │
   ├─ route match: /api/records POST
   │     └─ apimw.Auth      (Bearer token -> VerifySession -> email in context)
   │           └─ h.AddRecord
   │                 ├─ decodeJSON (size-limit + parse)
   │                 ├─ h.records.Add(ctx, domain, ip)   ── application service
   │                 │     ├─ recordsdomain.New(...)     ── domain validation
   │                 │     ├─ repo.Save(ctx, record)     ── repository -> SQLite
   │                 │     └─ notifier.Notify(...)
   │                 └─ respond(w, 200, {"ok": true})
   ▼
Browser ◄──JSON──
```

Every horizontal layer is a package you've now read. The request crosses from
transport (chi/middleware/handler) into application (service) into domain
(validation) into infrastructure (repository/SQLite) and back — always inward,
never the reverse.

---

## Exercises

1. **Call the API.** Start the server, then:
   ```bash
   TOKEN=$(curl -s localhost:8080/api/login -d '{"email":"admin@omnidns.local","password":"changeme123"}' | jq -r .token)
   curl -s localhost:8080/api/status -H "Authorization: Bearer $TOKEN" | jq
   ```
   Then call `/api/status` *without* the header and observe `401`.
2. **Watch CORS.** `curl -i -X OPTIONS localhost:8080/api/records -H 'Origin: http://evil.example'`
   and then with `-H 'Origin: http://localhost:5173'`. Compare the
   `Access-Control-Allow-Origin` header in each. Explain the difference.
3. **Body limit.** POST a 1 MB JSON body to `/api/records`. Confirm it's rejected
   (the `MaxBytesReader` trips `decodeJSON`). What status do you get?
4. **Add an endpoint.** Add `GET /api/records/count` returning
   `{"count": N}` using the repository `Count` method you wrote in Chapter 6's
   exercises. You'll touch `router.go`, a new handler in `handlers/records.go`,
   and the service — a full vertical slice.
5. **Trace context.** Add `slog.Info("handling", "request_id", middleware.RequestIDFromContext(r.Context()))`
   at the top of `AddRecord` and confirm the ID matches the `X-Request-ID`
   response header.

[← Chapter 6](06-persistence.md) · [Chapter 8: Testing & optimizations →](08-testing-and-optimizations.md)
