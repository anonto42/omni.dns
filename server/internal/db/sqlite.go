package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sohidul/esp32-dns-server/internal/models"
)

type DB struct {
	conn         *sql.DB
	insert       *sql.Stmt
	isBlockedSt  *sql.Stmt
	wildcardSt   *sql.Stmt
	getCustomSt  *sql.Stmt
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	conn.SetMaxOpenConns(2)

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := conn.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
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

	return db, nil
}

func (db *DB) Close() error {
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
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.insert.Exec(now, domain, clientIP, action); err != nil {
		log.Printf("log insert error: %v", err)
	}
}

func (db *DB) GetLogs(limit int) []models.QueryLog {
	rows, err := db.conn.Query(
		"SELECT id, timestamp, domain, client_ip, action FROM query_logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		log.Printf("getLogs query error: %v", err)
		return nil
	}
	defer rows.Close()

	var logs []models.QueryLog
	for rows.Next() {
		var l models.QueryLog
		var ts string
		var actionStr string
		if err := rows.Scan(&l.ID, &ts, &l.Domain, &l.ClientIP, &actionStr); err != nil {
			log.Printf("getLogs scan error: %v", err)
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
		log.Printf("clearLogs error: %v", err)
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
		log.Printf("getStats error: %v", err)
	}

	return s
}

func (db *DB) IsBlocked(domain string) bool {
	var count int
	if err := db.isBlockedSt.QueryRow(domain).Scan(&count); err != nil {
		log.Printf("isBlocked query error: %v", err)
		return false
	}
	if count > 0 {
		return true
	}

	rows, err := db.wildcardSt.Query()
	if err != nil {
		log.Printf("wildcard query error: %v", err)
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			log.Printf("wildcard scan error: %v", err)
			continue
		}
		wild := "." + w
		if len(domain) > len(w) && domain[len(domain)-len(wild):] == wild {
			return true
		}
	}

	return false
}

func (db *DB) GetCustomRecord(domain string) string {
	var ip string
	err := db.getCustomSt.QueryRow(domain).Scan(&ip)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("getCustomRecord error: %v", err)
	}
	return ip
}

func (db *DB) GetCustomRecords() map[string]string {
	rows, err := db.conn.Query("SELECT domain, ip FROM custom_records")
	if err != nil {
		log.Printf("getCustomRecords error: %v", err)
		return nil
	}
	defer rows.Close()

	recs := make(map[string]string)
	for rows.Next() {
		var domain, ip string
		if err := rows.Scan(&domain, &ip); err != nil {
			log.Printf("getCustomRecords scan error: %v", err)
			continue
		}
		recs[domain] = ip
	}
	return recs
}

func (db *DB) AddCustomRecord(domain, ip string) {
	if _, err := db.conn.Exec("INSERT OR REPLACE INTO custom_records (domain, ip) VALUES (?, ?)", domain, ip); err != nil {
		log.Printf("addCustomRecord error: %v", err)
	}
}

func (db *DB) DeleteCustomRecord(domain string) {
	if _, err := db.conn.Exec("DELETE FROM custom_records WHERE domain = ?", domain); err != nil {
		log.Printf("deleteCustomRecord error: %v", err)
	}
}

func (db *DB) GetBlocklist() []models.BlockedDomain {
	rows, err := db.conn.Query("SELECT domain, added_at, wildcard FROM blocklist ORDER BY domain")
	if err != nil {
		log.Printf("getBlocklist error: %v", err)
		return nil
	}
	defer rows.Close()

	var list []models.BlockedDomain
	for rows.Next() {
		var b models.BlockedDomain
		var addedAt string
		if err := rows.Scan(&b.Domain, &addedAt, &b.Wildcard); err != nil {
			log.Printf("getBlocklist scan error: %v", err)
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
		log.Printf("addToBlocklist error: %v", err)
	}
}

func (db *DB) RemoveFromBlocklist(domain string) {
	if _, err := db.conn.Exec("DELETE FROM blocklist WHERE domain = ?", domain); err != nil {
		log.Printf("removeFromBlocklist error: %v", err)
	}
}

func (db *DB) PruneLogs(before time.Time) {
	ts := before.UTC().Format(time.RFC3339)
	if _, err := db.conn.Exec("DELETE FROM query_logs WHERE timestamp < ?", ts); err != nil {
		log.Printf("pruneLogs error: %v", err)
	}
}
