package db

import (
	"database/sql"
	"log/slog"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sohidul/dns-server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	conn      *sql.DB
	logChan   chan models.QueryLog
	logBuffer []models.QueryLog
	mu        sync.Mutex
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// Create users table
	_, err = conn.Exec("CREATE TABLE IF NOT EXISTS users (email TEXT PRIMARY KEY, password TEXT)")
	if err != nil {
		return nil, err
	}
	db := &DB{
		conn:    conn,
		logChan: make(chan models.QueryLog, 1000),
	}
	go db.processLogBuffer()
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) InitAdmin(email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("INSERT OR IGNORE INTO users (email, password) VALUES (?, ?)", email, string(hashedPassword))
	return err
}

func (db *DB) VerifyUser(email, password string) bool {
	var hashedPassword string
	err := db.conn.QueryRow("SELECT password FROM users WHERE email = ?", email).Scan(&hashedPassword)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

func (db *DB) LogQuery(domain, clientIP string, action models.Action) {
	db.logChan <- models.QueryLog{
		Timestamp: time.Now(),
		Domain:    domain,
		ClientIP:  clientIP,
		Action:    action,
	}
}

func (db *DB) processLogBuffer() {
	ticker := time.NewTicker(5 * time.Second)
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

	stmt, err := tx.Prepare("INSERT INTO query_logs (timestamp, domain, client_ip, action) VALUES (?, ?, ?, ?)")
	if err != nil {
		slog.Error("flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, log := range logs {
		_, err := stmt.Exec(log.Timestamp, log.Domain, log.ClientIP, log.Action)
		if err != nil {
			slog.Error("flush exec failed", "error", err)
		}
	}
	if err := tx.Commit(); err != nil {
		slog.Error("flush commit failed", "error", err)
	}
}

func (db *DB) PruneLogs(t time.Time)                       {}
func (db *DB) GetStats() models.Stats                      { return models.Stats{} }
func (db *DB) GetLogs(limit int) []models.QueryLog         { return nil }
func (db *DB) ClearLogs()                                  {}
func (db *DB) GetCustomRecords() map[string]string         { return nil }
func (db *DB) AddCustomRecord(domain, ip string)           {}
func (db *DB) DeleteCustomRecord(domain string)            {}
func (db *DB) GetBlocklist() []models.BlockedDomain        { return nil }
func (db *DB) AddToBlocklist(domain string, wildcard bool) {}
func (db *DB) RemoveFromBlocklist(domain string)           {}
func (db *DB) IsBlocked(domain string) bool                { return false }
func (db *DB) GetCustomRecord(domain string) string        { return "" }
