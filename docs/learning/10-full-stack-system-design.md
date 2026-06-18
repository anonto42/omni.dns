# Full-Stack System Design Through OmniDNS

This chapter turns the system-design notes from the pasted conversation into a
reusable learning path, then applies that path to OmniDNS. The goal is not to
memorize terms. The goal is to learn how a full-stack engineer moves from a
product need to architecture decisions, then into maintainable frontend and
backend code.

Use this as a decision playbook. When you add a feature, read the relevant
section, make the trade-off explicit, then code the smallest version that keeps
the system honest.

---

## How To Use This For Any Software

You can follow this path for almost any software or technology: web apps, mobile
apps, backend APIs, desktop tools, AI products, games, IoT systems, browser
extensions, internal dashboards, or infrastructure tools. The exact technology
changes, but the engineering questions stay mostly the same.

Use this universal order:

1. Understand the problem.
   Define the user, workflow, inputs, outputs, failure cases, and success
   criteria before choosing tools.

2. Draw the high-level design.
   Identify the major parts: client, API, workers, database, cache, external
   services, queues, storage, deployment, and monitoring.

3. Choose the data model.
   Decide what data exists, who owns it, where it is stored, how it changes, and
   what must be consistent.

4. Design the API or interface contract.
   For a web app this may be REST or GraphQL. For a CLI it may be commands and
   flags. For a library it may be public functions. For a game it may be engine
   events and save files.

5. Choose the low-level design.
   Decide modules, folders, classes/functions, domain rules, services,
   repositories, components, hooks, jobs, and tests.

6. Decide performance and scale.
   Ask where latency matters, where throughput matters, what should be cached,
   what can be async, and what must stay synchronous.

7. Decide reliability and security.
   Plan authentication, authorization, validation, rate limits, retries,
   idempotency, backups, graceful shutdown, and safe error handling.

8. Build the smallest complete version.
   Make a thin vertical slice that touches UI, API, business logic,
   persistence, tests, and docs. Then improve it.

9. Observe and improve.
   Add logs, metrics, traces, profiling, user feedback, and tests. Optimize only
   after you know the real bottleneck.

10. Record decisions.
    Write docs, ADRs, or RFCs for important trade-offs so future you and future
    teammates understand why the system is shaped this way.

The reusable mental model is:

```text
Problem
  -> user workflow
  -> HLD: big components and communication
  -> data ownership and consistency
  -> API/interface contract
  -> LLD: modules, functions, classes, components
  -> tests, operations, security, performance
  -> documentation and iteration
```

OmniDNS is the concrete example in this chapter, but the same path works for
other technologies. If you build with Laravel, Django, Spring, Next.js, Flutter,
Electron, Unity, FastAPI, Go, Rust, or Node.js, replace the framework-specific
names while keeping the same decision order.

---

## 1. The Application Example

OmniDNS is a good full-stack system-design case study because one product
touches many layers:

- A browser dashboard for records, blocklists, logs, settings, auth, and traffic
  steering.
- A Go HTTP API that the dashboard calls.
- A Go DNS resolver that handles UDP/TCP DNS queries on the hot path.
- SQLite persistence for users, sessions, settings, records, blocklists,
  steering rules, notifications, and query logs.
- In-memory DNS cache, ARP cache, upstream forwarder pool, and buffered log
  writer.
- A production build that embeds the React dashboard into the Go binary.

The main code anchors are:

- Frontend app shell: [`frontend/src/App.tsx`](../../frontend/src/App.tsx)
- Frontend API client: [`frontend/src/hooks/api.ts`](../../frontend/src/hooks/api.ts)
- Backend composition root: [`backend/internal/server/server.go`](../../backend/internal/server/server.go)
- HTTP routes: [`backend/internal/interfaces/http/router.go`](../../backend/internal/interfaces/http/router.go)
- DNS pipeline: [`backend/internal/modules/resolver/engine/resolver.go`](../../backend/internal/modules/resolver/engine/resolver.go)
- DNS cache: [`backend/internal/modules/resolver/engine/cache/cache.go`](../../backend/internal/modules/resolver/engine/cache/cache.go)
- Persistence: [`backend/internal/infrastructure/persistence/sqlite.go`](../../backend/internal/infrastructure/persistence/sqlite.go)
- Auth/session persistence: [`backend/internal/infrastructure/persistence/auth.go`](../../backend/internal/infrastructure/persistence/auth.go)

---

## 2. The Step-by-Step Decision Plan

For every new feature or refactor, make decisions in this order.

### Step 1: Define The User Workflow

Ask:

- Who uses this feature?
- What screen or API action starts the workflow?
- What should happen if the network, database, DNS upstream, or browser fails?
- What data must be correct immediately, and what data can be slightly stale?

OmniDNS example: "Add a local DNS record from the dashboard."

User workflow:

1. User opens Records.
2. Frontend fetches existing records.
3. User submits domain, type, and value.
4. API validates the request.
5. Backend stores the record in SQLite.
6. Future DNS queries should answer from the custom record step before cache or
   upstream forwarding.
7. UI refreshes and shows the saved record.

### Step 2: Choose The HLD Shape

High-level design asks what major components are involved.

For the records workflow:

```text
Browser
  -> React feature hook/component
  -> /api/records
  -> HTTP handler
  -> records application service
  -> records repository
  -> SQLite

DNS client
  -> UDP/TCP listener
  -> DNS handler
  -> resolver pipeline
  -> records lookup
  -> DNS response
```

Decision checklist:

- Does this belong in the dashboard, DNS resolver, or both?
- Is this synchronous or asynchronous?
- Does it need persistence?
- Does it need a cache?
- Does it affect the DNS hot path?
- Does it change the public API contract?

### Step 3: Choose The LLD Shape

Low-level design asks how the code inside a component should be structured.

For an OmniDNS backend feature, prefer the existing module pattern:

```text
internal/modules/<feature>/
  domain/             pure entities, value objects, repository interfaces
  application/        use cases and validation orchestration
  infrastructure/     SQLite implementation of domain ports
```

Then wire it in:

- Add HTTP handler methods in `internal/interfaces/http/handlers/`.
- Register routes in `internal/interfaces/http/router.go`.
- Construct services in `internal/server/server.go`.
- Add persistence schema or migrations in
  `internal/infrastructure/persistence/sqlite.go` when needed.

For frontend features, prefer the existing feature slice pattern:

```text
frontend/src/features/<feature>/
  api/                HTTP calls for this feature
  hooks/              stateful view-model logic
  components/         presentational and workflow components
  index.ts            public exports
```

Then mount pages through:

- `frontend/src/pages/`
- `frontend/src/App.tsx`
- `frontend/src/components/layout/Sidebar.tsx` if navigation changes

### Step 4: Decide Data Ownership

Ask:

- What is the source of truth?
- Which layer may mutate it?
- Which layer may cache it?
- How does stale data get refreshed?

OmniDNS examples:

- Users and sessions: SQLite is source of truth.
- DNS cache: memory is a performance copy, not source of truth.
- Dashboard stats: backend is source of truth; frontend derives display values.
- Query logs: DNS resolver writes asynchronously; UI reads paginated views.

### Step 5: Decide Correctness Guarantees

Use this scale:

- Strong consistency: reads must reflect the latest write.
- Eventual consistency: stale data is temporarily acceptable.
- Best effort: data can be dropped under pressure.

OmniDNS examples:

- Login/session verification should be strongly consistent.
- Settings writes should be strongly consistent because they change behavior.
- DNS query logs are best effort; `LogQuery` may drop entries if the buffer is
  full so DNS resolution does not block.
- Cache results are intentionally stale until TTL expiry.

### Step 6: Decide Operational Behavior

Ask:

- What should happen at startup?
- What runs in the background?
- What must shut down cleanly?
- What should be observable in logs or metrics?

OmniDNS examples:

- `server.New` opens SQLite, creates the admin user, applies saved settings,
  and wires services.
- `server.Run` starts UDP, TCP, HTTP, ARP refresh, log pruning, and session
  cleanup.
- `DB.Close` drains the query-log buffer before closing SQLite.

### Step 7: Decide Tests Before Broadening The Feature

Start where the risk is highest:

- Domain rules: unit tests.
- Repository behavior: database tests.
- HTTP contract: handler tests or integration tests.
- Frontend transformations: hook/component tests.
- DNS behavior: resolver tests.
- Concurrency: race detector.

---

## 3. HLD Concepts For This Project

### Statelessness

Why it matters:

Stateless application servers can be duplicated behind a load balancer because
any server can handle any request.

Current OmniDNS shape:

- The Go process stores some runtime state in memory: DNS cache, ARP cache,
  upstream pool health, and buffered logs.
- Auth sessions are persisted in SQLite, so the HTTP API does not depend only on
  process memory for login state.
- The browser stores the session token in `localStorage` through
  [`frontend/src/hooks/api.ts`](../../frontend/src/hooks/api.ts).

Pros:

- Easy to run as a single self-hosted binary.
- Persisted sessions survive process restart.
- Runtime caches keep DNS fast.

Cons:

- If multiple OmniDNS instances share traffic, in-memory DNS cache and ARP cache
  are per instance.
- `localStorage` tokens are simple, but more exposed to XSS than `HttpOnly`
  cookies.
- SQLite is excellent for one node, but not a shared multi-writer store.

How to implement when scaling:

- Keep sessions in a shared store such as Redis or a shared SQL database.
- Move auth tokens to secure `HttpOnly` cookies if the deployment model supports
  it.
- Treat each app server as disposable: config in env/flags, data in external
  stores, no required local state.

OmniDNS decision:

For a home-network DNS product, single-node with persisted sessions and local
caches is a good first architecture. If the project becomes multi-node, sessions
and cache invalidation become explicit design work.

### Load Balancing And High Availability

Why it matters:

A load balancer prevents one app server from being the only entry point. It also
lets you deploy or restart instances with less downtime.

Current OmniDNS shape:

- One Go process serves DNS, API, and embedded static UI.
- Development uses Vite for the dashboard and proxies `/api` to Go.
- Production can run as one small container or binary.

Pros:

- Very easy self-hosting.
- Fewer moving pieces for home users.
- Lower operational complexity.

Cons:

- One process is a single point of failure.
- DNS port 53 and dashboard traffic are tied to the same deployment unit.
- Horizontal scaling needs coordination for SQLite, cache, sessions, and DNS
  listener ownership.

How to implement when scaling:

- Put HTTP dashboard/API behind an L7 reverse proxy.
- Put DNS behind keepalived, anycast, or router failover if high availability is
  required.
- Separate the resolver process from the management API if their scaling needs
  diverge.
- Add health checks for `/health` and DNS probe queries.

### API Gateway And BFF

Why it matters:

An API gateway centralizes auth, rate limiting, request IDs, and routing. A BFF
or backend-for-frontend shapes backend responses for one frontend experience.

Current OmniDNS shape:

- The Go server is effectively both API server and BFF for the React dashboard.
- `router.go` applies auth middleware to `/api` routes.
- Handlers return dashboard-oriented JSON.

Pros:

- Simple and productive.
- Frontend has one API base path: `/api`.
- Auth is centralized in middleware.

Cons:

- Public API and dashboard API are the same contract unless separated later.
- Mobile or CLI clients may need different response shapes.
- Rate limiting is not yet a first-class gateway concern.

How to implement next:

- Version external contracts, for example `/api/v1/records`.
- Keep dashboard-specific aggregation in handlers or a BFF layer.
- Add rate limiting around login and mutation endpoints.
- Add request IDs to logs and frontend error reports.

---

## 4. Caching And Freshness

Caching trades freshness for speed. A full-stack engineer must decide where the
cache lives and how stale it may be.

### Client-Side Cache

Frontend examples:

- Route chunks are lazy-loaded in [`frontend/src/App.tsx`](../../frontend/src/App.tsx).
- Feature hooks fetch API data and derive view models, such as
  [`useStats`](../../frontend/src/features/stats/hooks/useStats.ts).

Pros:

- Reduces repeated requests.
- Makes the UI feel fast.
- Can support offline or reconnect behavior later.

Cons:

- Stale UI can confuse users after mutations.
- Cache invalidation logic can spread across components.

How to implement:

- Keep server state separate from UI state.
- Use feature hooks as the boundary for fetching, loading, refreshing, and
  derived values.
- For more complex server-state caching, consider a library such as TanStack
  Query. Until then, keep refresh logic local and explicit.

### DNS Cache

Backend example:

- [`cache.Cache`](../../backend/internal/modules/resolver/engine/cache/cache.go)
  stores answers by `(domain, qtype)` with TTL and LRU eviction.

Pros:

- Avoids repeated upstream DNS lookups.
- Reduces latency on the resolver hot path.
- Honors TTL-based expiry.

Cons:

- Can return stale answers until TTL expires.
- Cache entries are per process.
- Incorrect cache keys can create subtle DNS bugs.

How to implement safely:

- Key by domain and query type, not just domain.
- Respect TTL.
- Cache negative answers with a shorter TTL.
- Track hits, misses, and size.
- Provide `Close` for background eviction goroutines.

### Application Cache

OmniDNS could add an application cache for read-heavy dashboard data, such as
expensive stats or large log summaries.

Pros:

- Reduces database load.
- Useful for charts and dashboard cards.

Cons:

- Requires invalidation after writes.
- Can hide stale operational data.

Decision rule:

Do not cache dashboard reads until profiling shows the database is the
bottleneck. The DNS cache matters today because DNS resolution is the hot path.

---

## 5. Data Design, SQL, NoSQL, And CAP

### SQL vs NoSQL

Why SQL fits OmniDNS now:

- Data is relational and small enough for a home deployment.
- Records, blocklists, settings, users, sessions, and notifications benefit from
  clear schema and constraints.
- SQLite keeps self-hosting simple.

Pros:

- Strong local consistency.
- Easy backup: one database file.
- Simple development and deployment.

Cons:

- Not designed for many distributed writers.
- Large query-log tables will need pruning, indexing, and perhaps archival.
- Horizontal scaling is harder than with external managed databases.

How to evolve:

- Add indexes based on real query patterns.
- Keep migrations additive and idempotent, like the current `migrate` function.
- Split hot logs from control-plane data if logs grow too large.
- Move logs to a time-series or analytics store only when SQLite becomes the
  measured bottleneck.

### CAP Theorem

For distributed systems, network partitions happen. During a partition, choose
between consistency and availability for each workflow.

OmniDNS decisions:

- DNS resolution should prefer availability. If logging fails, still answer DNS.
- Auth and settings should prefer consistency. A bad permission or wrong setting
  should not be accepted for convenience.
- Dashboard charts can tolerate stale data for a short time.

Practical rule:

Do not choose one CAP behavior for the whole app. Choose it per workflow.

---

## 6. Async Work And Message Queues

Message queues decouple slow work from user-facing requests.

Current OmniDNS shape:

- Query logging is asynchronous in-process using a buffered channel.
- Background workers prune logs and sweep expired sessions.
- No external queue is required yet.

Pros:

- DNS responses are not blocked by every log write.
- Simple deployment.
- Easy to reason about in one process.

Cons:

- Buffer is memory-only.
- Under sustained overload, logs may be dropped.
- Work is not durable before it reaches SQLite.

When to add a real queue:

- Notifications, log export, blocklist imports, or analytics become slow.
- You need retries across process restarts.
- You need multiple workers processing tasks.
- You need backpressure instead of dropped work.

Implementation options:

- In-process queue: simplest, good for best-effort local work.
- SQLite jobs table: durable enough for single-node background jobs.
- Redis/RabbitMQ/SQS/Kafka: useful when work must scale across processes or
  machines.

OmniDNS learning exercise:

Implement blocklist import as a durable job:

1. API accepts a file or URL.
2. Backend creates an `import_jobs` row.
3. Worker validates domains and inserts valid entries in batches.
4. UI polls job status.
5. Notifications report completion or failure.

This teaches API design, async work, persistence, progress UI, and failure
handling without jumping immediately to Kafka.

---

## 7. API Design

An API is a contract. Once the frontend, CLI, or external users depend on it,
changes require migration.

Current OmniDNS shape:

- Dashboard API uses JSON under `/api`.
- Auth uses `POST /api/login`, bearer tokens, and auth middleware.
- Resource routes exist for records, blocklist, logs, settings, profile,
  notifications, and steering.

Good API rules:

- Design around resources, not internal functions.
- Use stable request and response shapes.
- Return useful errors.
- Add pagination for lists.
- Make mutations idempotent when duplicate user actions are possible.
- Version public APIs before external clients depend on them.

REST vs GraphQL:

- REST is the better fit now: simple resources, cacheable reads, predictable
  routes, easy debugging.
- GraphQL may help if many clients need different nested shapes, but it adds
  schema, resolver, auth, and caching complexity.

OmniDNS next decisions:

- Add `/api/v1` before promising external API stability.
- Add idempotency keys for risky mutations like large imports or bulk deletes.
- Keep frontend API calls inside feature `api/` folders so contract changes are
  localized.

---

## 8. Frontend Architecture

Frontend architecture is about user workflows, state boundaries, rendering,
performance, and API contracts.

### App Shell

Current shape:

- [`App.tsx`](../../frontend/src/App.tsx) owns routes, lazy page loading,
  protected routes, dashboard layout, tour provider, metadata, and toasts.

Pros:

- One clear root for cross-cutting browser concerns.
- Lazy pages reduce the initial bundle.
- Protected routes keep auth behavior consistent.

Cons:

- The root can become crowded if every global concern is added here.
- Error boundaries are not yet visible as a first-class pattern.

How to implement well:

- Keep `App.tsx` as orchestration only.
- Put feature logic in feature hooks and components.
- Add route-level error boundaries for risky or data-heavy pages.
- Keep global providers intentional and few.

### Server State vs UI State

Server state:

- Comes from the backend.
- Can be stale.
- Needs loading, error, refresh, and retry behavior.

UI state:

- Exists only for browser interaction.
- Examples: modal open state, selected tab, form draft, table filter.

OmniDNS examples:

- `useStats` derives display values from backend status.
- Record/blocklist/steering hooks manage feature workflows.
- Layout and theme hooks handle UI state.

Decision rule:

Do not store server data in global UI state unless multiple distant features
need the same cached source. Prefer feature hooks first.

### Rendering Strategy

Current choice:

- React + Vite single-page app.
- Go serves the compiled SPA in production.

Pros:

- Good dashboard interactivity.
- Simple static build.
- Works well behind one Go server.

Cons:

- Less SEO value than SSR, though SEO is not important for a private dashboard.
- Browser does more rendering work.

Decision:

CSR is correct for OmniDNS because this is an authenticated operational
dashboard, not a public marketing site.

---

## 9. Backend Architecture

Backend architecture is about dependency direction, correctness, performance,
and lifecycle.

### Clean Architecture And Feature Modules

Current choice:

- Accepted in [ADR 002](../adr/002-modular-clean-architecture.md).
- Feature code is grouped by module.
- Domain code is independent of HTTP, DNS, and SQLite.

Pros:

- Features are easier to find.
- Domain rules are easier to test.
- Infrastructure can change without rewriting domain logic.

Cons:

- More packages and import aliases.
- Small features can feel heavier at first.

How to add a backend feature:

1. Define domain entity/value object.
2. Define repository port in the domain package.
3. Implement use case in application service.
4. Implement SQLite repository in infrastructure.
5. Add handler methods.
6. Register HTTP routes.
7. Wire dependencies in `server.New`.
8. Add focused tests.

### DNS Resolver Hot Path

Current pipeline:

```text
query
  -> blocklist
  -> steering rules
  -> custom records
  -> DNS cache
  -> upstream forwarder
  -> query log
```

Why this order matters:

- Blocklist and steering are policy decisions.
- Custom records are authoritative local answers.
- Cache avoids upstream work.
- Upstream is the fallback.
- Logging records the decision without blocking the answer path too much.

Design rule:

Any change inside the DNS pipeline must be reviewed for latency, locks,
allocation, and failure behavior. This is the hottest backend path.

### Auth And Sessions

Current shape:

- Passwords use Argon2id hashes.
- Sessions are random tokens persisted with expiry.
- Expired sessions are lazily deleted and periodically swept.

Pros:

- Passwords are not stored in plaintext.
- Session expiry is enforced.
- Session cleanup prevents unbounded growth.

Cons:

- Bearer token in `localStorage` has XSS risk.
- No refresh-token flow.
- No rate limiting on login yet.

How to improve:

- Add login rate limiting.
- Consider `HttpOnly`, `Secure`, `SameSite` cookies for production dashboard
  auth.
- Add audit logging for login failures and password changes.

---

## 10. Codebase Management

Good architecture also means the team can change code safely.

### Repository Structure

Current choice:

- One repository contains backend, frontend, Docker, docs, and config.

Pros:

- Atomic full-stack changes are easy.
- Shared docs and scripts stay near the code.
- Good fit for a single product.

Cons:

- CI must avoid running unnecessary checks as the repo grows.
- Boundaries rely on folder discipline.

Decision:

Keep the monorepo. If a future resolver agent, CLI, or mobile app grows large,
split only when independent release cycles become painful.

### Code Ownership And Boundaries

Rules for OmniDNS:

- `frontend/src/features/*` owns feature UI logic.
- `frontend/src/components/ui/*` owns reusable UI primitives.
- `backend/internal/modules/*` owns business behavior.
- `backend/internal/interfaces/*` owns transport adapters.
- `backend/internal/infrastructure/*` owns external technology details.
- `backend/internal/server/*` owns wiring and lifecycle.
- `docs/adr/*` records decisions.
- `docs/rfc/*` proposes future work.
- `docs/learning/*` teaches current code.

### Change Management

Before merging a meaningful change:

- Explain the HLD impact.
- Explain the LLD placement.
- Update API docs or learning docs if behavior changes.
- Add or update tests near the changed behavior.
- Run backend and frontend checks.

Useful commands:

```bash
make test
make test-backend
make test-frontend
```

---

## 11. Design Partners And Product Decisions

Full-stack engineers work with design partners, not only code.

For OmniDNS, design partnership means:

- Turn DNS concepts into understandable workflows.
- Keep dense operational screens scannable.
- Use consistent UI primitives for tables, dialogs, forms, buttons, badges, and
  empty states.
- Make dangerous actions explicit.
- Show loading, empty, error, and success states.
- Avoid hiding system behavior behind decorative UI.

How to collaborate:

1. Start with the workflow, not the component.
2. Define what the user needs to decide on the screen.
3. Define API states: loading, success, empty, validation error, permission
   error, server error.
4. Define copy for destructive actions and DNS-specific terms.
5. Convert repeated patterns into reusable components only after repetition is
   real.

Example: blocklist import screen

- Designer decides layout, progress, error grouping, and confirmation flow.
- Frontend engineer maps those states to components and hooks.
- Backend engineer defines job status and validation errors.
- Together they define the API contract so the UI can show precise feedback.

---

## 12. End-to-End Feature Walkthrough

Feature: "Bulk import a blocklist from a URL."

This single feature teaches HLD, LLD, frontend architecture, backend
architecture, async processing, data modeling, API design, observability, and
testing.

### Decision 1: Product Workflow

User story:

"As a home-network admin, I want to paste a blocklist URL, import it in the
background, see progress, and review how many domains were added or skipped."

Failure cases:

- URL cannot be fetched.
- File is huge.
- Lines contain invalid domains.
- Import is interrupted.
- Duplicate domains already exist.

### Decision 2: HLD

```text
React Blocklist Import Modal
  -> POST /api/blocklist/import-jobs
  -> SQLite import_jobs row
  -> background worker
  -> fetch URL
  -> validate and batch insert domains
  -> update job progress
  -> notification
  -> UI polls GET /api/blocklist/import-jobs/{id}
```

### Decision 3: LLD Backend

Add:

```text
internal/modules/blocklist/domain/import_job.go
internal/modules/blocklist/application/import_service.go
internal/modules/blocklist/infrastructure/import_jobs.go
internal/interfaces/http/handlers/blocklist_import.go
```

Schema:

```sql
CREATE TABLE IF NOT EXISTS blocklist_import_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_url TEXT NOT NULL,
  status TEXT NOT NULL,
  total_lines INTEGER DEFAULT 0,
  imported_count INTEGER DEFAULT 0,
  skipped_count INTEGER DEFAULT 0,
  error TEXT DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

API:

```text
POST /api/blocklist/import-jobs
GET  /api/blocklist/import-jobs/{id}
```

### Decision 4: LLD Frontend

Add:

```text
frontend/src/features/blocklist/api/imports.ts
frontend/src/features/blocklist/hooks/useBlocklistImport.ts
frontend/src/features/blocklist/components/BlocklistImportModal.tsx
```

UI states:

- idle
- submitting
- queued
- running
- completed
- failed

### Decision 5: Consistency And Availability

- Creating the job should be strongly consistent.
- Import processing can be asynchronous.
- UI progress can be eventually consistent.
- Duplicate domains should be skipped idempotently.
- DNS resolution must continue even while import runs.

### Decision 6: Caching

- Do not cache import job writes.
- Poll job status every few seconds while active.
- After completion, refresh the blocklist table.
- If resolver blocklist lookups are cached in memory in the future, invalidate
  or reload after import completion.

### Decision 7: Observability

Log:

- job created
- fetch started
- validation summary
- insert summary
- job failed
- job completed

Expose:

- job status in UI
- notification on completion
- error message with a useful reason

### Decision 8: Tests

Backend:

- domain validation tests
- import service tests with fake fetcher/repository
- SQLite repository tests
- handler contract tests

Frontend:

- hook tests for job states
- component tests for progress and errors

System:

- run `make test`
- manually test a small valid list, invalid URL, duplicate domains, and a large
  file

---

## 13. Full-Stack Engineer Checklist

Use this checklist when learning or designing features.

### Frontend

- Can I explain the user workflow?
- Are server state and UI state separated?
- Are loading, empty, error, and success states designed?
- Is the API client code localized?
- Does the page avoid unnecessary global state?
- Is the route lazy-loaded if appropriate?
- Are destructive actions confirmed?
- Are tables searchable, paginated, or filtered when data can grow?

### Backend

- Does the feature belong in an existing module or a new module?
- Is domain logic free from HTTP and SQL details?
- Are repository interfaces defined at the domain boundary?
- Does the API route match a resource?
- Are inputs validated close to the use case?
- Are database changes additive and idempotent?
- Does the feature affect the DNS hot path?
- What happens on timeout, cancellation, or shutdown?

### Data

- What is the source of truth?
- What must be strongly consistent?
- What can be eventually consistent?
- What can be best effort?
- Which indexes will this query need?
- What data needs pruning or archival?

### Operations

- How does this start?
- How does this stop?
- What should be logged?
- What should be measured?
- What alert would reveal failure?
- Can the system degrade gracefully?

### Codebase

- Is the change in the right folder?
- Is the API contract documented?
- Did tests cover the risky behavior?
- Is an ADR needed for a lasting architecture decision?
- Is an RFC needed for a future feature proposal?

---

## 14. Learning Path From Here

Follow this order if your goal is to become strong at full-stack system design
through this project:

1. Read the backend architecture chapters in this course.
2. Trace one DNS query through `interfaces/dns`, `resolver`, `cache`, and
   `persistence`.
3. Trace one dashboard request from React feature hook to HTTP handler to
   SQLite and back.
4. Add a small synchronous feature, such as a new settings field.
5. Add a small async feature, such as import-job progress.
6. Add tests at domain, repository, handler, and frontend hook levels.
7. Write an ADR for any decision that changes architecture.
8. Profile before optimizing.
9. Revisit this document before each larger feature and fill in the decision
   plan.

The senior-engineer habit is simple: name the trade-off, choose deliberately,
then make the code structure show the decision.
