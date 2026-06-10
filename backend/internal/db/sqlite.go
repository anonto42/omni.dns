package db

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sohidul/dns-server/internal/models"
	"golang.org/x/crypto/argon2"
	_ "modernc.org/sqlite"
)

// lookupMAC reads /proc/net/arp to find the MAC address for a given IP.
// Returns an empty string if not found or the file is unreadable.
func lookupMAC(ip string) string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[0] == ip {
			mac := fields[3]
			if mac != "00:00:00:00:00:00" {
				return mac
			}
		}
	}
	return ""
}

type DB struct {
	conn      *sql.DB
	logChan   chan models.QueryLog
	logBuffer []models.QueryLog
	mu        sync.Mutex
	quit      chan struct{}
}

const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

func verifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, computed) == 1
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, err
	}

	queries := []string{
		"CREATE TABLE IF NOT EXISTS users (email TEXT PRIMARY KEY, password TEXT)",
		"CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, email TEXT, created_at TEXT)",
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT)",
		"CREATE TABLE IF NOT EXISTS query_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, domain TEXT, client_ip TEXT, action TEXT, mac_address TEXT DEFAULT '')",
		"CREATE TABLE IF NOT EXISTS custom_records (domain TEXT PRIMARY KEY, ip TEXT)",
		"CREATE TABLE IF NOT EXISTS blocklist (domain TEXT PRIMARY KEY, added_at TEXT, wildcard INTEGER DEFAULT 0)",
		`CREATE TABLE IF NOT EXISTS steering_rules (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			name     TEXT NOT NULL,
			condition_type  TEXT NOT NULL,
			condition_value TEXT NOT NULL,
			action_type     TEXT NOT NULL,
			action_target   TEXT NOT NULL DEFAULT '',
			priority        INTEGER NOT NULL DEFAULT 0,
			enabled         INTEGER NOT NULL DEFAULT 1
		)`,
	}
	for _, q := range queries {
		if _, err := conn.Exec(q); err != nil {
			return nil, err
		}
	}
	// Add mac_address column to existing databases (ignore error if column already exists).
	conn.Exec("ALTER TABLE query_logs ADD COLUMN mac_address TEXT DEFAULT ''")
	db := &DB{
		conn:    conn,
		logChan: make(chan models.QueryLog, 1000),
		quit:    make(chan struct{}),
	}
	go db.processLogBuffer()
	return db, nil
}

func (db *DB) Close() error {
	close(db.quit)
	db.Flush()
	return db.conn.Close()
}

func (db *DB) InitAdmin(email, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("INSERT OR IGNORE INTO users (email, password) VALUES (?, ?)", email, hashedPassword)
	return err
}

func (db *DB) VerifyUser(email, password string) bool {
	var hashedPassword string
	err := db.conn.QueryRow("SELECT password FROM users WHERE email = ?", email).Scan(&hashedPassword)
	if err != nil {
		return false
	}
	return verifyPassword(password, hashedPassword)
}

func (db *DB) LogQuery(domain, clientIP string, action models.Action) {
	db.logChan <- models.QueryLog{
		Timestamp:  time.Now(),
		Domain:     domain,
		ClientIP:   clientIP,
		MACAddress: lookupMAC(clientIP),
		Action:     action,
	}
}

func (db *DB) processLogBuffer() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case log := <-db.logChan:
			db.mu.Lock()
			db.logBuffer = append(db.logBuffer, log)
			shouldFlush := len(db.logBuffer) >= 100
			db.mu.Unlock()
			if shouldFlush {
				db.Flush()
			}
		case <-ticker.C:
			db.Flush()
		case <-db.quit:
			return
		}
	}
}

func (db *DB) Flush() {
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
			slog.Error("failed to rollback transaction", "error", err)
		}
	}()

	stmt, err := tx.Prepare("INSERT INTO query_logs (timestamp, domain, client_ip, action, mac_address) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		slog.Error("flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, log := range logs {
		_, err := stmt.Exec(log.Timestamp.Format(time.RFC3339), log.Domain, log.ClientIP, log.Action, log.MACAddress)
		if err != nil {
			slog.Error("flush exec failed", "error", err)
		}
	}
	if err := tx.Commit(); err != nil {
		slog.Error("flush commit failed", "error", err)
	}
}

func (db *DB) GetStats() models.Stats {
	var forwarded, blocked, custom, cached int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM query_logs WHERE action = 'forwarded'").Scan(&forwarded); err != nil {
		slog.Error("get stats forwarded failed", "error", err)
	}
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM query_logs WHERE action = 'blocked'").Scan(&blocked); err != nil {
		slog.Error("get stats blocked failed", "error", err)
	}
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM query_logs WHERE action = 'custom'").Scan(&custom); err != nil {
		slog.Error("get stats custom failed", "error", err)
	}
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM query_logs WHERE action = 'cached'").Scan(&cached); err != nil {
		slog.Error("get stats cached failed", "error", err)
	}
	return models.Stats{
		QueriesForwarded: forwarded,
		QueriesBlocked:   blocked,
		QueriesCustom:    custom,
		QueriesCached:    cached,
	}
}

func (db *DB) GetLogs(limit int) []models.QueryLog {
	rows, err := db.conn.Query("SELECT id, timestamp, domain, client_ip, action, COALESCE(mac_address,'') FROM query_logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return []models.QueryLog{}
	}
	defer rows.Close()

	logs := make([]models.QueryLog, 0)
	for rows.Next() {
		var l models.QueryLog
		var ts string
		if err := rows.Scan(&l.ID, &ts, &l.Domain, &l.ClientIP, &l.Action, &l.MACAddress); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			l.Timestamp = t
		}
		logs = append(logs, l)
	}
	return logs
}

func (db *DB) ClearLogs() {
	if _, err := db.conn.Exec("DELETE FROM query_logs"); err != nil {
		slog.Error("clear logs failed", "error", err)
	}
}

func (db *DB) PruneLogs(t time.Time) {
	if _, err := db.conn.Exec("DELETE FROM query_logs WHERE timestamp < ?", t.Format(time.RFC3339)); err != nil {
		slog.Error("prune logs failed", "error", err)
	}
}

func (db *DB) GetCustomRecords() map[string]string {
	rows, err := db.conn.Query("SELECT domain, ip FROM custom_records")
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()

	records := map[string]string{}
	for rows.Next() {
		var domain, ip string
		if err := rows.Scan(&domain, &ip); err == nil {
			records[domain] = ip
		}
	}
	return records
}

func (db *DB) AddCustomRecord(domain, ip string) {
	if _, err := db.conn.Exec("INSERT OR REPLACE INTO custom_records (domain, ip) VALUES (?, ?)", domain, ip); err != nil {
		slog.Error("add custom record failed", "error", err)
	}
}

func (db *DB) DeleteCustomRecord(domain string) {
	if _, err := db.conn.Exec("DELETE FROM custom_records WHERE domain = ?", domain); err != nil {
		slog.Error("delete custom record failed", "error", err)
	}
}

func (db *DB) GetCustomRecord(domain string) string {
	var ip string
	err := db.conn.QueryRow("SELECT ip FROM custom_records WHERE domain = ?", domain).Scan(&ip)
	if err != nil {
		return ""
	}
	return ip
}

func (db *DB) GetBlocklist() []models.BlockedDomain {
	rows, err := db.conn.Query("SELECT domain, added_at, wildcard FROM blocklist ORDER BY domain")
	if err != nil {
		return []models.BlockedDomain{}
	}
	defer rows.Close()

	domains := make([]models.BlockedDomain, 0)
	for rows.Next() {
		var d models.BlockedDomain
		var addedAt string
		var wildcardInt int
		if err := rows.Scan(&d.Domain, &addedAt, &wildcardInt); err != nil {
			continue
		}
		d.Wildcard = wildcardInt != 0
		if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
			d.AddedAt = t
		}
		domains = append(domains, d)
	}
	return domains
}

func (db *DB) AddToBlocklist(domain string, wildcard bool) {
	w := 0
	if wildcard {
		w = 1
	}
	if _, err := db.conn.Exec("INSERT OR IGNORE INTO blocklist (domain, added_at, wildcard) VALUES (?, ?, ?)", domain, time.Now().Format(time.RFC3339), w); err != nil {
		slog.Error("add to blocklist failed", "error", err)
	}
}

func (db *DB) RemoveFromBlocklist(domain string) {
	if _, err := db.conn.Exec("DELETE FROM blocklist WHERE domain = ?", domain); err != nil {
		slog.Error("remove from blocklist failed", "error", err)
	}
}

func (db *DB) IsBlocked(domain string) bool {
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM blocklist WHERE ? LIKE CASE WHEN wildcard THEN '%' || domain ELSE domain END", domain).Scan(&count); err != nil {
		slog.Error("is blocked query failed", "error", err)
	}
	return count > 0
}

func (db *DB) CreateSession(email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	_, err := db.conn.Exec("INSERT INTO sessions (token, email, created_at) VALUES (?, ?, ?)", token, email, time.Now().Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (db *DB) VerifySession(token string) (string, bool) {
	var email string
	err := db.conn.QueryRow("SELECT email FROM sessions WHERE token = ?", token).Scan(&email)
	if err != nil {
		return "", false
	}
	return email, true
}

func (db *DB) DeleteSession(token string) {
	if _, err := db.conn.Exec("DELETE FROM sessions WHERE token = ?", token); err != nil {
		slog.Error("delete session failed", "error", err)
	}
}

func (db *DB) GetSettings() map[string]string {
	rows, err := db.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()
	s := map[string]string{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			s[k] = v
		}
	}
	return s
}

// GetLogs returns logs with optional filtering by action and domain substring,
// and supports a configurable limit (0 = use defaultLimit).
func (db *DB) GetLogsFiltered(limit int, action, domain string) []models.QueryLog {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := "SELECT id, timestamp, domain, client_ip, action, COALESCE(mac_address,'') FROM query_logs WHERE 1=1"
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
		return []models.QueryLog{}
	}
	defer rows.Close()

	logs := make([]models.QueryLog, 0)
	for rows.Next() {
		var l models.QueryLog
		var ts string
		if err := rows.Scan(&l.ID, &ts, &l.Domain, &l.ClientIP, &l.Action, &l.MACAddress); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			l.Timestamp = t
		}
		logs = append(logs, l)
	}
	return logs
}

func (db *DB) GetSteeringRules() []models.SteeringRule {
	rows, err := db.conn.Query("SELECT id, name, condition_type, condition_value, action_type, action_target, priority, enabled FROM steering_rules ORDER BY priority ASC, id ASC")
	if err != nil {
		return []models.SteeringRule{}
	}
	defer rows.Close()
	rules := make([]models.SteeringRule, 0)
	for rows.Next() {
		var r models.SteeringRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.ConditionType, &r.ConditionValue, &r.ActionType, &r.ActionTarget, &r.Priority, &enabled); err != nil {
			continue
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules
}

func (db *DB) AddSteeringRule(r models.AddSteeringRuleRequest) (int64, error) {
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	res, err := db.conn.Exec(
		"INSERT INTO steering_rules (name, condition_type, condition_value, action_type, action_target, priority, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)",
		r.Name, r.ConditionType, r.ConditionValue, r.ActionType, r.ActionTarget, r.Priority, enabled,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) UpdateSteeringRuleEnabled(id int64, enabled bool) {
	e := 0
	if enabled {
		e = 1
	}
	if _, err := db.conn.Exec("UPDATE steering_rules SET enabled = ? WHERE id = ?", e, id); err != nil {
		slog.Error("update steering rule failed", "error", err)
	}
}

func (db *DB) DeleteSteeringRule(id int64) {
	if _, err := db.conn.Exec("DELETE FROM steering_rules WHERE id = ?", id); err != nil {
		slog.Error("delete steering rule failed", "error", err)
	}
}

func (db *DB) ChangePassword(email, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("UPDATE users SET password = ? WHERE email = ?", hash, email)
	return err
}

func (db *DB) SaveSettings(settings map[string]string) {
	tx, err := db.conn.Begin()
	if err != nil {
		slog.Error("settings tx begin failed", "error", err)
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			slog.Error("settings tx rollback failed", "error", err)
		}
	}()
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)")
	if err != nil {
		slog.Error("settings prepare failed", "error", err)
		return
	}
	defer stmt.Close()
	for k, v := range settings {
		if _, err := stmt.Exec(k, v); err != nil {
			slog.Error("settings save failed", "key", k, "error", err)
		}
	}
	if err := tx.Commit(); err != nil {
		slog.Error("settings commit failed", "error", err)
	}
}
