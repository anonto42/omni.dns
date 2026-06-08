package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sohidul/dns-server/internal/models"
)

type logEntry struct {
	timestamp string
	domain    string
	clientIP  string
	action    models.Action
}

type DB struct {
	conn        *sql.DB
	insert      *sql.Stmt
	isBlockedSt *sql.Stmt
	wildcardSt  *sql.Stmt
	getCustomSt *sql.Stmt
	logChan     chan logEntry

	customRecords map[string]string
	blocklist     map[string]bool
	wildcards     []string
	mu            sync.RWMutex
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	conn.SetMaxOpenConns(1) // Better for SQLite write-heavy loads

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := conn.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	db := &DB{
		conn:          conn,
		logChan:       make(chan logEntry, 1000),
		customRecords: make(map[string]string),
		blocklist:     make(map[string]bool),
	}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	if err := db.loadCache(); err != nil {
		return nil, fmt.Errorf("load cache: %w", err)
	}

	db.insert, err = conn.Prepare("INSERT INTO query_logs (timestamp, domain, client_ip, action) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("prepare insert: %w", err)
	}

	db.isBlockedSt, err = conn.Prepare("SELECT COUNT(*) FROM blocklist WHERE domain = ?")
	if err != nil {
		return nil, fmt.Errorf("prepare isBlocked: %w", err)
	}

	db.wildcardSt, err = conn.Prepare("SELECT domain FROM blocklist WHERE wildcard = 1")
	if err != nil {
		return nil, fmt.Errorf("prepare wildcard: %w", err)
	}

	db.getCustomSt, err = conn.Prepare("SELECT ip FROM custom_records WHERE domain = ?")
	if err != nil {
		return nil, fmt.Errorf("prepare getCustom: %w", err)
	}

	go db.logWorker()

	return db, nil
}

func (db *DB) Close() error {
	close(db.logChan) // Signal worker to stop and flush
	if db.insert != nil {
		db.insert.Close()
	}
	if db.isBlockedSt != nil {
		db.isBlockedSt.Close()
	}
	if db.wildcardSt != nil {
		db.wildcardSt.Close()
	}
	if db.getCustomSt != nil {
		db.getCustomSt.Close()
	}
	return db.conn.Close()
}

func (db *DB) logWorker() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var batch []logEntry
	for {
		select {
		case entry, ok := <-db.logChan:
			if !ok {
				db.flush(batch)
				return
			}
			batch = append(batch, entry)
			if len(batch) >= 100 {
				db.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				db.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (db *DB) flush(batch []logEntry) {
	if len(batch) == 0 {
		return
	}
	tx, err := db.conn.Begin()
	if err != nil {
		slog.Error("flush begin tx failed", "error", err)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO query_logs (timestamp, domain, client_ip, action) VALUES (?, ?, ?, ?)")
	if err != nil {
		slog.Error("flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, e := range batch {
		if _, err := stmt.Exec(e.timestamp, e.domain, e.clientIP, e.action); err != nil {
			slog.Error("flush exec failed", "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("flush commit failed", "error", err)
	}
}

func (db *DB) loadCache() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Load custom records
	rows, err := db.conn.Query("SELECT domain, ip FROM custom_records")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var domain, ip string
		if err := rows.Scan(&domain, &ip); err != nil {
			return err
		}
		db.customRecords[domain] = ip
	}

	// Load blocklist
	rows, err = db.conn.Query("SELECT domain, wildcard FROM blocklist")
	if err != nil {
		return err
	}
	defer rows.Close()
	db.wildcards = nil
	for rows.Next() {
		var domain string
		var wildcard bool
		if err := rows.Scan(&domain, &wildcard); err != nil {
			return err
		}
		if wildcard {
			db.wildcards = append(db.wildcards, domain)
		} else {
			db.blocklist[domain] = true
		}
	}

	return nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS query_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		domain TEXT NOT NULL,
		client_ip TEXT NOT NULL,
		action TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS custom_records (
		domain TEXT PRIMARY KEY,
		ip TEXT NOT NULL,
		type TEXT DEFAULT 'A'
	);

	CREATE TABLE IF NOT EXISTS blocklist (
		domain TEXT PRIMARY KEY,
		added_at TEXT NOT NULL,
		wildcard INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON query_logs(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_logs_action ON query_logs(action);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) LogQuery(domain, clientIP string, action models.Action) {
	entry := logEntry{
		timestamp: time.Now().UTC().Format(time.RFC3339),
		domain:    domain,
		clientIP:  clientIP,
		action:    action,
	}

	select {
	case db.logChan <- entry:
	default:
		slog.Warn("log channel full, dropping entry", "domain", domain)
	}
}

func (db *DB) GetLogs(limit int) []models.QueryLog {
	rows, err := db.conn.Query(
		"SELECT id, timestamp, domain, client_ip, action FROM query_logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		slog.Error("getLogs query failed", "error", err)
		return nil
	}
	defer rows.Close()

	var logs []models.QueryLog
	for rows.Next() {
		var l models.QueryLog
		var ts string
		var actionStr string
		if err := rows.Scan(&l.ID, &ts, &l.Domain, &l.ClientIP, &actionStr); err != nil {
			slog.Error("getLogs scan failed", "error", err)
			continue
		}
		l.Action = models.Action(actionStr)
		l.Timestamp, _ = time.Parse(time.RFC3339, ts)
		logs = append(logs, l)
	}
	return logs
}

func (db *DB) ClearLogs() {
	if _, err := db.conn.Exec("DELETE FROM query_logs"); err != nil {
		slog.Error("clearLogs failed", "error", err)
	}
}

func (db *DB) GetStats() models.Stats {
	var s models.Stats

	err := db.conn.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN action='forwarded' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='blocked' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='custom' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='cached' THEN 1 ELSE 0 END), 0)
		FROM query_logs
	`).Scan(&s.QueriesForwarded, &s.QueriesBlocked, &s.QueriesCustom, &s.QueriesCached)
	if err != nil {
		slog.Error("getStats query failed", "error", err)
	}

	return s
}

func (db *DB) IsBlocked(domain string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.blocklist[domain] {
		return true
	}

	for _, w := range db.wildcards {
		wild := "." + w
		if len(domain) > len(w) && domain[len(domain)-len(wild):] == wild {
			return true
		}
	}

	return false
}

func (db *DB) GetCustomRecord(domain string) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.customRecords[domain]
}

func (db *DB) GetCustomRecords() map[string]string {
	rows, err := db.conn.Query("SELECT domain, ip FROM custom_records")
	if err != nil {
		slog.Error("getCustomRecords query failed", "error", err)
		return nil
	}
	defer rows.Close()

	recs := make(map[string]string)
	for rows.Next() {
		var domain, ip string
		if err := rows.Scan(&domain, &ip); err != nil {
			slog.Error("getCustomRecords scan failed", "error", err)
			continue
		}
		recs[domain] = ip
	}
	return recs
}

func (db *DB) AddCustomRecord(domain, ip string) {
	if _, err := db.conn.Exec("INSERT OR REPLACE INTO custom_records (domain, ip) VALUES (?, ?)", domain, ip); err != nil {
		slog.Error("addCustomRecord failed", "error", err)
		return
	}

	db.mu.Lock()
	db.customRecords[domain] = ip
	db.mu.Unlock()
}

func (db *DB) DeleteCustomRecord(domain string) {
	if _, err := db.conn.Exec("DELETE FROM custom_records WHERE domain = ?", domain); err != nil {
		slog.Error("deleteCustomRecord failed", "error", err)
		return
	}

	db.mu.Lock()
	delete(db.customRecords, domain)
	db.mu.Unlock()
}

func (db *DB) GetBlocklist() []models.BlockedDomain {
	rows, err := db.conn.Query("SELECT domain, added_at, wildcard FROM blocklist ORDER BY domain")
	if err != nil {
		slog.Error("getBlocklist query failed", "error", err)
		return nil
	}
	defer rows.Close()

	var list []models.BlockedDomain
	for rows.Next() {
		var b models.BlockedDomain
		var addedAt string
		if err := rows.Scan(&b.Domain, &addedAt, &b.Wildcard); err != nil {
			slog.Error("getBlocklist scan failed", "error", err)
			continue
		}
		b.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		list = append(list, b)
	}
	return list
}

func (db *DB) AddToBlocklist(domain string, wildcard bool) {
	w := 0
	if wildcard {
		w = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.conn.Exec("INSERT OR REPLACE INTO blocklist (domain, added_at, wildcard) VALUES (?, ?, ?)",
		domain, now, w); err != nil {
		slog.Error("addToBlocklist failed", "error", err)
		return
	}

	db.mu.Lock()
	if wildcard {
		// Update wildcards list
		found := false
		for _, v := range db.wildcards {
			if v == domain {
				found = true
				break
			}
		}
		if !found {
			db.wildcards = append(db.wildcards, domain)
		}
		delete(db.blocklist, domain)
	} else {
		db.blocklist[domain] = true
		// Remove from wildcards if it was there
		for i, v := range db.wildcards {
			if v == domain {
				db.wildcards = append(db.wildcards[:i], db.wildcards[i+1:]...)
				break
			}
		}
	}
	db.mu.Unlock()
}

func (db *DB) RemoveFromBlocklist(domain string) {
	if _, err := db.conn.Exec("DELETE FROM blocklist WHERE domain = ?", domain); err != nil {
		slog.Error("removeFromBlocklist failed", "error", err)
		return
	}

	db.mu.Lock()
	delete(db.blocklist, domain)
	for i, v := range db.wildcards {
		if v == domain {
			db.wildcards = append(db.wildcards[:i], db.wildcards[i+1:]...)
			break
		}
	}
	db.mu.Unlock()
}

func (db *DB) PruneLogs(before time.Time) {
	ts := before.UTC().Format(time.RFC3339)
	if _, err := db.conn.Exec("DELETE FROM query_logs WHERE timestamp < ?", ts); err != nil {
		slog.Error("pruneLogs failed", "error", err)
	}
}
