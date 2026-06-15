# Chapter 6 — Persistence with `database/sql` and SQLite

> Goal: learn Go's standard database layer, SQLite specifics, schema migrations,
> the repository pattern, and the security/correctness fixes (session expiry,
> single-query stats, parameterized queries).

---

## 6.1 `database/sql`: the standard interface

Go's `database/sql` package is a **driver-agnostic** API. You import a driver for
its side effects and talk to everything through `*sql.DB`. From
[`internal/db/sqlite.go`](../../backend/internal/db/sqlite.go):

```go
import (
	"database/sql"
	_ "modernc.org/sqlite"   // blank import: registers the "sqlite" driver
)
```

The `_` (blank import) means "run this package's `init()` for its side effects,
but don't reference it directly." The driver's `init()` calls
`sql.Register("sqlite", ...)`. We picked `modernc.org/sqlite` because it's a
**pure-Go** SQLite — no cgo, so cross-compilation and static binaries just work.

### Opening the database

```go
func Open(path string, opts Options) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := conn.Exec(pragma); err != nil {
			return nil, fmt.Errorf("apply %q: %w", pragma, err)
		}
	}
	// ... schema, migrations, start log writer ...
}
```

`sql.Open` does **not** actually connect — it prepares a connection *pool*. The
first real query connects lazily. The three pragmas tune SQLite:

- **`journal_mode=WAL`** (Write-Ahead Logging): readers don't block writers and
  vice versa. Essential when DNS queries write logs while the API reads them.
- **`busy_timeout=5000`**: if the DB is briefly locked, wait up to 5s instead of
  failing instantly.
- **`synchronous=NORMAL`**: a safe-enough durability/speed trade-off under WAL.

> `*sql.DB` is **safe for concurrent use** by many goroutines — it manages the
> pool internally. You don't wrap it in a mutex. That's why the resolver's many
> goroutines can all log through one `*db.DB`.

---

## 6.2 Schema and migrations

`createSchema` runs `CREATE TABLE IF NOT EXISTS` for every table. Note the
custom-records table — the AAAA fix at the storage layer:

```go
// custom_records keyed by (domain, qtype): 1 = A, 28 = AAAA.
"CREATE TABLE IF NOT EXISTS custom_records (domain TEXT, ip TEXT, qtype INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (domain, qtype))",
```

The **composite primary key** `(domain, qtype)` is what lets one domain hold both
an A and an AAAA record without collision. That single schema decision is the
foundation of the NODATA logic in Chapter 5.

### Additive, idempotent migrations

How do we evolve the schema for users upgrading from the old version *without a
migration framework*? Additive `ALTER TABLE`s that tolerate "already exists":

```go
func migrate(conn *sql.DB) error {
	stmts := []string{
		"ALTER TABLE users ADD COLUMN name TEXT DEFAULT 'Administrator'",
		"ALTER TABLE sessions ADD COLUMN expires_at TEXT",          // the session-expiry fix
		"ALTER TABLE custom_records ADD COLUMN qtype INTEGER NOT NULL DEFAULT 1",
		// ... more query_logs columns ...
	}
	for _, q := range stmts {
		if _, err := conn.Exec(q); err != nil && !isDuplicateColumnErr(err) {
			return fmt.Errorf("migrate %q: %w", q, err)
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}
```

The trick: try to add the column; if SQLite says "duplicate column" (it already
exists from a prior run or fresh schema), **swallow that specific error** and
continue. Any *other* error is real and aborts startup. This makes `Open`
**idempotent** — safe to run against a brand-new DB, a one-version-old DB, or a
current DB. The rule is *additive only*: we never drop or rename columns, so old
and new code can share a database file.

> This is a pragmatic choice for a small single-file app. A larger system would
> use versioned migrations (golang-migrate, goose). The principle — forward-only,
> tolerant, deterministic — is the same.

---

## 6.3 Querying: `QueryRow`, `Query`, `Exec`

Three methods cover almost everything:

- **`QueryRow`** — exactly one row. From
  [`internal/db/records.go`](../../backend/internal/db/records.go):

```go
func (db *DB) LookupRecord(ctx context.Context, domain string, qtype uint16) (ip string, found bool, existsOtherType bool) {
	row := db.conn.QueryRowContext(ctx, "SELECT ip FROM custom_records WHERE domain = ? AND qtype = ?", domain, qtype)
	if err := row.Scan(&ip); err == nil {
		return ip, true, false                  // exact (domain,qtype) hit
	}
	var other int
	if err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM custom_records WHERE domain = ?", domain).Scan(&other); err != nil {
		return "", false, false
	}
	return "", false, other > 0                 // other-family exists? -> NODATA upstream
}
```

`Scan` copies column values into the addresses you pass (`&ip`). The `?`
placeholders are **parameters** — never string-concatenate user input into SQL
(SQL injection). The driver binds them safely.

- **`Query`** — many rows; you iterate. From
  [`internal/db/sqlite.go`](../../backend/internal/db/sqlite.go):

```go
rows, err := db.conn.Query(query, args...)
if err != nil {
	return []models.QueryLog{}
}
defer rows.Close()           // ALWAYS close rows

logs := make([]models.QueryLog, 0)
for rows.Next() {            // advance to next row
	var l models.QueryLog
	var ts string
	if err := rows.Scan(&l.ID, &ts, &l.Domain, /* ...many cols... */); err != nil {
		continue
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		l.Timestamp = t
	}
	logs = append(logs, l)
}
return logs
```

The canonical loop: `rows, err := Query(...)` → `defer rows.Close()` →
`for rows.Next() { rows.Scan(...) }`. **Forgetting `defer rows.Close()` leaks a
connection** from the pool — a real, common bug.

- **`Exec`** — writes (INSERT/UPDATE/DELETE), returns a `Result`:

```go
res, err := db.conn.Exec("INSERT INTO steering_rules (...) VALUES (?, ?, ...)", ...)
// res.LastInsertId(), res.RowsAffected()
```

### `Context`-aware variants

`QueryContext`, `QueryRowContext`, `ExecContext` take a `context.Context` as the
first arg, so a cancelled request (Chapter 4) can abort an in-flight query. The
repositories use these and thread `ctx` from the HTTP handler all the way down —
see [`internal/db/repositories/records.go`](../../backend/internal/db/repositories/records.go).

---

## 6.4 The session-expiry fix, in SQL

The old code created sessions that **never expired** — a security hole. The fix
spans three methods in [`internal/db/auth.go`](../../backend/internal/db/auth.go).

**Create with an expiry:**

```go
func (db *DB) CreateSession(email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {     // crypto/rand: secure token
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := time.Now()
	_, err := db.conn.Exec(
		"INSERT INTO sessions (token, email, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, email, now.Format(time.RFC3339), now.Add(db.opts.SessionTTL).Format(time.RFC3339),
	)
	// ...
}
```

**Verify and lazily delete if expired:**

```go
func (db *DB) VerifySession(token string) (string, bool) {
	var email, expiresAt string
	err := db.conn.QueryRow("SELECT email, COALESCE(expires_at, '') FROM sessions WHERE token = ?", token).Scan(&email, &expiresAt)
	if err != nil {
		return "", false
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(exp) {
		db.DeleteSession(token)        // expired (or legacy NULL) -> remove
		return "", false
	}
	return email, true
}
```

Two things to note:

- `COALESCE(expires_at, '')` turns a NULL (legacy rows from before the migration)
  into `''`, which fails to parse, so old sessions are treated as expired. This
  is the deliberate "everyone re-logs-in once after upgrade" behavior.
- Verification **deletes** expired tokens as it finds them ("lazy" cleanup).

**Sweep proactively** (called hourly by the server's janitor, Chapter 4):

```go
func (db *DB) SweepExpiredSessions() {
	res, err := db.conn.Exec("DELETE FROM sessions WHERE expires_at IS NULL OR expires_at < ?", time.Now().Format(time.RFC3339))
	// ... log how many removed
}
```

Lazy deletion handles tokens people present; the sweeper reaps tokens nobody will
ever present again. Together the table can't grow unbounded. Tests in
[`session_test.go`](../../backend/internal/db/session_test.go) pin all three behaviors.

> **Password security bonus:** `auth.go` hashes passwords with **Argon2id**
> (`golang.org/x/crypto/argon2`) and compares them with
> `subtle.ConstantTimeCompare` to avoid timing attacks. Read `hashPassword` /
> `verifyPassword` — they encode the salt and parameters into the stored string,
> the same approach as bcrypt/PHC.

---

## 6.5 The stats fix: one query instead of four

The old `GetStats` ran four separate `SELECT COUNT(*) ... WHERE action = '...'`
queries. The refactor collapses them into a single pass with **conditional
aggregation**:

```go
func (db *DB) GetStats() models.Stats {
	var s models.Stats
	err := db.conn.QueryRow(`
		SELECT
			SUM(CASE WHEN action = 'forwarded' THEN 1 ELSE 0 END),
			SUM(CASE WHEN action = 'blocked'   THEN 1 ELSE 0 END),
			SUM(CASE WHEN action = 'custom'    THEN 1 ELSE 0 END),
			SUM(CASE WHEN action = 'cached'    THEN 1 ELSE 0 END)
		FROM query_logs
	`).Scan(&nullInt{&s.QueriesForwarded}, &nullInt{&s.QueriesBlocked}, &nullInt{&s.QueriesCustom}, &nullInt{&s.QueriesCached})
	if err != nil {
		slog.Error("get stats failed", "error", err)
	}
	return s
}
```

`SUM(CASE WHEN ... THEN 1 ELSE 0 END)` counts rows matching a condition in a
single table scan — four counts, one query, one round trip.

### A custom `sql.Scanner`: `nullInt`

There's a subtlety: if `query_logs` is empty, `SUM(...)` returns **NULL**, not 0,
and scanning NULL into a plain `int` errors. So we define a tiny type that
implements the `sql.Scanner` interface:

```go
type nullInt struct{ dst *int }

func (n *nullInt) Scan(v any) error {
	switch t := v.(type) {
	case int64:
		*n.dst = int(t)
	case nil:
		*n.dst = 0          // NULL -> 0
	}
	return nil
}
```

By having a `Scan(any) error` method, `*nullInt` satisfies `sql.Scanner`, and
`rows.Scan` will call it. This is the same interface-satisfaction idea from
Chapter 3, applied to the database layer: we taught `database/sql` how to handle
our NULL-as-zero case by implementing a one-method interface. The `switch t :=
v.(type)` is a **type switch** — branching on the dynamic type of an `any`.

---

## 6.6 The repository pattern: keeping SQL out of the domain

Chapter 3 showed the domain defining `Repository` interfaces. The
**implementations** live in [`internal/db/repositories/`](../../backend/internal/db/repositories/),
the only place that knows both SQL *and* domain types. Example —
[`repositories/records.go`](../../backend/internal/db/repositories/records.go):

```go
func (r *Records) Save(ctx context.Context, record recordsdomain.Record) error {
	_, err := r.db.Conn().ExecContext(
		ctx,
		"INSERT OR REPLACE INTO custom_records (domain, ip, qtype) VALUES (?, ?, ?)",
		record.Domain().String(), record.IP().String(), int(record.IP().QType()),
	)
	return err
}
```

See how it asks the domain `Record` for its qtype (`record.IP().QType()`, from
Chapter 3's value object) and stores it. The repository **translates** between
domain aggregates and table rows. The domain never imports `database/sql`; the
SQL never imports HTTP. Each concern stays in its package.

`r.db.Conn()` exposes the underlying `*sql.DB` so repositories in a sibling
package can run queries — a deliberate, narrow seam between `db` and
`db/repositories`.

---

## 6.7 Transactions

When several writes must succeed or fail together, use a transaction. From
`SaveSettings` in [`internal/db/queries.go`](../../backend/internal/db/queries.go):

```go
tx, err := db.conn.Begin()
if err != nil { ... }
defer func() {
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		slog.Error("settings rollback failed", "error", err)
	}
}()
stmt, err := tx.Prepare("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)")
// ... loop: stmt.Exec(k, v) ...
if err := tx.Commit(); err != nil { ... }
```

The idiom:

1. `Begin()` starts the transaction.
2. **`defer tx.Rollback()`** — a safety net. If we return early (error path), the
   rollback fires. If we already committed, `Rollback` returns `sql.ErrTxDone`,
   which we ignore. This guarantees we never leave a transaction dangling.
3. `tx.Prepare` compiles the statement once; the loop reuses it for many rows
   (efficient batch insert).
4. `tx.Commit()` makes it durable. If commit isn't reached, the deferred rollback
   undoes everything.

The query-log `flush()` (Chapter 4) uses the same transaction+prepared-statement
pattern to write a whole batch of logs atomically and fast.

---

## Exercises

1. **Inspect the schema.** With the server stopped:
   `sqlite3 data/dns.db '.schema sessions'` and confirm the `expires_at` column
   exists. Then `.schema custom_records` and find the composite PK.
2. **Prove parameterization.** Try adding a custom record with domain
   `'; DROP TABLE custom_records; --` via the API. It's stored as a literal
   string (or rejected by domain validation), never executed. Why? (Answer: `?`
   binding.)
3. **Empty-stats NULL.** Clear logs (`DELETE /api/logs`) and call
   `GET /api/status`. Confirm the counts are `0`, not an error — that's `nullInt`
   doing its job. Temporarily replace `&nullInt{...}` with `&s.QueriesForwarded`
   and watch it break on empty tables.
4. **Write a repository method.** Add `Count(ctx) (int, error)` to the records
   repository using `QueryRowContext` + `COUNT(*)`. Wire nothing else — just make
   it compile and write a quick test against a `t.TempDir()` DB (see
   `session_test.go` for the pattern).
5. **Transaction rollback.** In `SaveSettings`, make the *second* `stmt.Exec`
   fail (e.g. pass a value that violates a constraint you add). Confirm the first
   insert is rolled back, not persisted.

[← Chapter 5](05-dns-resolver-pipeline.md) · [Chapter 7: The HTTP API →](07-http-api.md)
