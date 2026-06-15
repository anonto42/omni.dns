package persistence

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T, ttl time.Duration) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path, Options{SessionTTL: ttl})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestSessionVerifiesWhenFresh(t *testing.T) {
	db := newTestDB(t, time.Hour)
	token, err := db.CreateSession("admin@example.com")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	email, ok := db.VerifySession(token)
	if !ok || email != "admin@example.com" {
		t.Fatalf("expected valid session, got ok=%v email=%q", ok, email)
	}
}

func TestSessionExpires(t *testing.T) {
	db := newTestDB(t, time.Hour)
	token, err := db.CreateSession("admin@example.com")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	// Backdate the expiry to simulate an aged-out session.
	past := time.Now().Add(-time.Minute).Format(time.RFC3339)
	if _, err := db.conn.Exec("UPDATE sessions SET expires_at = ? WHERE token = ?", past, token); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, ok := db.VerifySession(token); ok {
		t.Fatal("expected expired session to be rejected")
	}
	// VerifySession should have deleted it lazily.
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = ?", token).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected expired session deleted, found %d", count)
	}
}

func TestSweepExpiredSessions(t *testing.T) {
	db := newTestDB(t, time.Hour)
	token, err := db.CreateSession("a@example.com")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	past := time.Now().Add(-time.Minute).Format(time.RFC3339)
	if _, err := db.conn.Exec("UPDATE sessions SET expires_at = ? WHERE token = ?", past, token); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	db.SweepExpiredSessions()
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected sweep to remove expired sessions, found %d", count)
	}
}

func TestStatsAggregatesInOneQuery(t *testing.T) {
	db := newTestDB(t, time.Hour)
	for _, action := range []string{"forwarded", "forwarded", "blocked", "custom", "cached"} {
		if _, err := db.conn.Exec("INSERT INTO query_logs (timestamp, domain, action) VALUES (?, 'x', ?)", time.Now().Format(time.RFC3339), action); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	s := db.GetStats()
	if s.QueriesForwarded != 2 || s.QueriesBlocked != 1 || s.QueriesCustom != 1 || s.QueriesCached != 1 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}
