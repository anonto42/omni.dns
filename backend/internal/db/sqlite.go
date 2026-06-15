// Package db provides SQLite-backed persistence: connection setup, schema
// migrations, and a buffered query-log writer. Domain-specific access lives in
// the repositories subpackage and in the typed methods on DB.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sohidul/dns-server/internal/db/models"
	_ "modernc.org/sqlite"
)

// Options tunes the buffered log writer and session lifetime.
type Options struct {
	LogFlushInterval time.Duration
	LogFlushSize     int
	SessionTTL       time.Duration
}

func (o Options) withDefaults() Options {
	if o.LogFlushInterval <= 0 {
		o.LogFlushInterval = 5 * time.Second
	}
	if o.LogFlushSize <= 0 {
		o.LogFlushSize = 100
	}
	if o.SessionTTL <= 0 {
		o.SessionTTL = 24 * time.Hour
	}
	return o
}

// DB owns the SQLite connection and the asynchronous query-log buffer.
type DB struct {
	conn *sql.DB
	opts Options

	logChan   chan models.QueryLog
	logBuffer []models.QueryLog
	mu        sync.Mutex
	quit      chan struct{}
	flushDone chan struct{}
}

// Open connects to the SQLite database at path, applies pragmas and migrations,
// and starts the background log-buffer flusher.
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
	if err := createSchema(conn); err != nil {
		return nil, err
	}
	if err := migrate(conn); err != nil {
		return nil, err
	}

	db := &DB{
		conn:      conn,
		opts:      opts.withDefaults(),
		logChan:   make(chan models.QueryLog, 1000),
		quit:      make(chan struct{}),
		flushDone: make(chan struct{}),
	}
	go db.processLogBuffer()
	return db, nil
}

// Conn exposes the underlying connection for repository implementations.
func (db *DB) Conn() *sql.DB { return db.conn }

// Close stops the flusher, drains any buffered logs, and closes the connection.
func (db *DB) Close() error {
	close(db.quit)
	<-db.flushDone // wait for the flusher goroutine to drain and exit
	return db.conn.Close()
}

func createSchema(conn *sql.DB) error {
	stmts := []string{
		"CREATE TABLE IF NOT EXISTS users (email TEXT PRIMARY KEY, password TEXT, name TEXT DEFAULT 'Administrator')",
		"CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, email TEXT, created_at TEXT, expires_at TEXT)",
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"CREATE TABLE IF NOT EXISTS query_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, domain TEXT, client_ip TEXT, action TEXT, mac_address TEXT DEFAULT '', protocol TEXT DEFAULT '', query_type TEXT DEFAULT '', response_code TEXT DEFAULT '', resolved_ip TEXT DEFAULT '', all_answers TEXT DEFAULT '', answer_count INTEGER DEFAULT 0, ttl INTEGER DEFAULT 0, upstream_resolver TEXT DEFAULT '', latency_ms REAL DEFAULT 0)",
		// custom_records keyed by (domain, qtype): 1 = A, 28 = AAAA.
		"CREATE TABLE IF NOT EXISTS custom_records (domain TEXT, ip TEXT, qtype INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (domain, qtype))",
		"CREATE TABLE IF NOT EXISTS blocklist (domain TEXT PRIMARY KEY, added_at TEXT, wildcard INTEGER DEFAULT 0)",
		`CREATE TABLE IF NOT EXISTS steering_rules (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			name     TEXT NOT NULL,
			condition_type  TEXT NOT NULL,
			condition_value TEXT NOT NULL,
			action_type     TEXT NOT NULL,
			action_target   TEXT NOT NULL DEFAULT '',
			priority        INTEGER DEFAULT 1,
			enabled         INTEGER DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL,
			read INTEGER DEFAULT 0
		)`,
	}
	for _, q := range stmts {
		if _, err := conn.Exec(q); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

// migrate applies additive, idempotent schema changes for upgrades from older
// databases. Duplicate-column errors are expected and ignored.
func migrate(conn *sql.DB) error {
	stmts := []string{
		"ALTER TABLE users ADD COLUMN name TEXT DEFAULT 'Administrator'",
		"ALTER TABLE sessions ADD COLUMN expires_at TEXT",
		"ALTER TABLE custom_records ADD COLUMN qtype INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE query_logs ADD COLUMN mac_address TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN protocol TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN query_type TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN response_code TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN resolved_ip TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN all_answers TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN answer_count INTEGER DEFAULT 0",
		"ALTER TABLE query_logs ADD COLUMN ttl INTEGER DEFAULT 0",
		"ALTER TABLE query_logs ADD COLUMN upstream_resolver TEXT DEFAULT ''",
		"ALTER TABLE query_logs ADD COLUMN latency_ms REAL DEFAULT 0",
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

// --- buffered query logging ----------------------------------------------

// LogQuery enqueues a query log for asynchronous persistence.
func (db *DB) LogQuery(log models.QueryLog) {
	log.Timestamp = time.Now()
	select {
	case db.logChan <- log:
	default:
		slog.Warn("query log buffer full; dropping entry", "domain", log.Domain)
	}
}

func (db *DB) processLogBuffer() {
	defer close(db.flushDone)
	ticker := time.NewTicker(db.opts.LogFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case log := <-db.logChan:
			db.mu.Lock()
			db.logBuffer = append(db.logBuffer, log)
			full := len(db.logBuffer) >= db.opts.LogFlushSize
			db.mu.Unlock()
			if full {
				db.flush()
			}
		case <-ticker.C:
			db.flush()
		case <-db.quit:
			db.drain()
			db.flush()
			return
		}
	}
}

// drain moves any logs still in the channel into the buffer so a shutdown
// flush does not lose them.
func (db *DB) drain() {
	for {
		select {
		case log := <-db.logChan:
			db.mu.Lock()
			db.logBuffer = append(db.logBuffer, log)
			db.mu.Unlock()
		default:
			return
		}
	}
}

func (db *DB) flush() {
	db.mu.Lock()
	if len(db.logBuffer) == 0 {
		db.mu.Unlock()
		return
	}
	logs := db.logBuffer
	db.logBuffer = nil
	db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		slog.Error("flush begin tx failed", "error", err)
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			slog.Error("flush rollback failed", "error", err)
		}
	}()

	stmt, err := tx.Prepare("INSERT INTO query_logs (timestamp, domain, client_ip, action, mac_address, protocol, query_type, response_code, resolved_ip, all_answers, answer_count, ttl, upstream_resolver, latency_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		slog.Error("flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.Exec(log.Timestamp.Format(time.RFC3339), log.Domain, log.ClientIP, log.Action, log.MACAddress, log.Protocol, log.QueryType, log.ResponseCode, log.ResolvedIP, log.AllAnswers, log.AnswerCount, log.TTL, log.UpstreamResolver, log.LatencyMs); err != nil {
			slog.Error("flush exec failed", "error", err)
		}
	}
	if err := tx.Commit(); err != nil {
		slog.Error("flush commit failed", "error", err)
	}
}

// --- query log reads / maintenance ---------------------------------------

// GetLogsFiltered returns recent logs, optionally filtered by action and a
// domain substring. limit is clamped to (0, 1000]; 0 means the default of 100.
func (db *DB) GetLogsFiltered(limit int, action, domain string) []models.QueryLog {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := "SELECT id, timestamp, domain, client_ip, action, COALESCE(mac_address,''), COALESCE(protocol,''), COALESCE(query_type,''), COALESCE(response_code,''), COALESCE(resolved_ip,''), COALESCE(all_answers,''), COALESCE(answer_count,0), COALESCE(ttl,0), COALESCE(upstream_resolver,''), COALESCE(latency_ms,0) FROM query_logs WHERE 1=1"
	args := []any{}
	if action != "" {
		query += " AND action = ?"
		args = append(args, action)
	}
	if domain != "" {
		query += " AND domain LIKE ?"
		args = append(args, "%"+domain+"%")
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		slog.Error("get logs failed", "error", err)
		return []models.QueryLog{}
	}
	defer rows.Close()

	logs := make([]models.QueryLog, 0)
	for rows.Next() {
		var l models.QueryLog
		var ts string
		if err := rows.Scan(&l.ID, &ts, &l.Domain, &l.ClientIP, &l.Action, &l.MACAddress, &l.Protocol, &l.QueryType, &l.ResponseCode, &l.ResolvedIP, &l.AllAnswers, &l.AnswerCount, &l.TTL, &l.UpstreamResolver, &l.LatencyMs); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			l.Timestamp = t
		}
		logs = append(logs, l)
	}
	return logs
}

// ClearLogs deletes all query logs.
func (db *DB) ClearLogs() {
	if _, err := db.conn.Exec("DELETE FROM query_logs"); err != nil {
		slog.Error("clear logs failed", "error", err)
	}
}

// PruneLogs deletes query logs older than t.
func (db *DB) PruneLogs(t time.Time) {
	if _, err := db.conn.Exec("DELETE FROM query_logs WHERE timestamp < ?", t.Format(time.RFC3339)); err != nil {
		slog.Error("prune logs failed", "error", err)
	}
}

// GetStats returns query-disposition counts using a single aggregate query.
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

// nullInt scans a possibly-NULL SUM() result into an int (NULL -> 0).
type nullInt struct{ dst *int }

func (n *nullInt) Scan(v any) error {
	switch t := v.(type) {
	case int64:
		*n.dst = int(t)
	case nil:
		*n.dst = 0
	}
	return nil
}

// GetCustomRecord returns the IPv4 (A) record for a domain, kept for callers
// that only need A. Prefer LookupRecord for qtype-aware resolution.
func (db *DB) GetCustomRecord(domain string) string {
	ip, _, _ := db.LookupRecord(context.Background(), domain, 1)
	return ip
}
