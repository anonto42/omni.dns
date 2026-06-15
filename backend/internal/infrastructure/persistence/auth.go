package persistence

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

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
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}
	var memory, t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &t, &threads); err != nil {
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
	computed := argon2.IDKey([]byte(password), salt, t, memory, threads, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, computed) == 1
}

// InitAdmin inserts the bootstrap admin user if no row with that email exists.
func (db *DB) InitAdmin(email, password string) error {
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("INSERT OR IGNORE INTO users (email, password) VALUES (?, ?)", email, hashed)
	return err
}

// VerifyUser reports whether the email/password pair is valid.
func (db *DB) VerifyUser(email, password string) bool {
	var hashed string
	if err := db.conn.QueryRow("SELECT password FROM users WHERE email = ?", email).Scan(&hashed); err != nil {
		return false
	}
	return verifyPassword(password, hashed)
}

// ChangePassword updates the stored password hash for a user.
func (db *DB) ChangePassword(email, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("UPDATE users SET password = ? WHERE email = ?", hash, email)
	return err
}

// CreateSession issues a new session token that expires after the configured
// session TTL.
func (db *DB) CreateSession(email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := time.Now()
	_, err := db.conn.Exec(
		"INSERT INTO sessions (token, email, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, email, now.Format(time.RFC3339), now.Add(db.opts.SessionTTL).Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// VerifySession returns the session's email if the token exists and has not
// expired. Expired sessions are deleted lazily.
func (db *DB) VerifySession(token string) (string, bool) {
	var email, expiresAt string
	err := db.conn.QueryRow("SELECT email, COALESCE(expires_at, '') FROM sessions WHERE token = ?", token).Scan(&email, &expiresAt)
	if err != nil {
		return "", false
	}
	// Treat a missing expiry (legacy row) as expired so it gets refreshed on
	// next login rather than living forever.
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(exp) {
		db.DeleteSession(token)
		return "", false
	}
	return email, true
}

// DeleteSession removes a session token.
func (db *DB) DeleteSession(token string) {
	if _, err := db.conn.Exec("DELETE FROM sessions WHERE token = ?", token); err != nil {
		slog.Error("delete session failed", "error", err)
	}
}

// SweepExpiredSessions deletes all sessions past their expiry. Intended to be
// called periodically by a background janitor.
func (db *DB) SweepExpiredSessions() {
	res, err := db.conn.Exec("DELETE FROM sessions WHERE expires_at IS NULL OR expires_at < ?", time.Now().Format(time.RFC3339))
	if err != nil {
		slog.Error("sweep sessions failed", "error", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		slog.Debug("swept expired sessions", "count", n)
	}
}

// GetProfile returns the display name for a user.
func (db *DB) GetProfile(email string) (string, error) {
	var name string
	if err := db.conn.QueryRow("SELECT name FROM users WHERE email = ?", email).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

// UpdateProfile changes a user's email and name, keeping any active session
// rows pointed at the new email.
func (db *DB) UpdateProfile(oldEmail, newEmail, name string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			slog.Error("profile rollback failed", "error", err)
		}
	}()

	if oldEmail != newEmail {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", newEmail).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("email %s is already in use", newEmail)
		}
	}
	if _, err := tx.Exec("UPDATE users SET email = ?, name = ? WHERE email = ?", newEmail, name, oldEmail); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE sessions SET email = ? WHERE email = ?", newEmail, oldEmail); err != nil {
		return err
	}
	return tx.Commit()
}
