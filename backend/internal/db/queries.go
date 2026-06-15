package db

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/sohidul/dns-server/internal/db/models"
)

// IsBlocked reports whether a domain matches a blocklist entry, honoring
// wildcard entries (a wildcard entry for example.com matches sub.example.com).
func (db *DB) IsBlocked(domain string) bool {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM blocklist WHERE
		  (wildcard = 0 AND domain = ?)
		  OR (wildcard = 1 AND (? = domain OR ? LIKE '%.' || domain))
	`, domain, domain, domain).Scan(&count)
	if err != nil {
		slog.Error("is blocked query failed", "error", err)
	}
	return count > 0
}

// --- settings ------------------------------------------------------------

// GetSettings returns all persisted key/value settings.
func (db *DB) GetSettings() map[string]string {
	rows, err := db.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	return out
}

// SaveSettings upserts a batch of settings in a single transaction.
func (db *DB) SaveSettings(settings map[string]string) {
	tx, err := db.conn.Begin()
	if err != nil {
		slog.Error("settings tx begin failed", "error", err)
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			slog.Error("settings rollback failed", "error", err)
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

// --- steering rules ------------------------------------------------------

// GetSteeringRules returns all steering rules in priority order.
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

// Rules adapts DB to the resolver's SteeringRules port.
func (db *DB) Rules() []models.SteeringRule { return db.GetSteeringRules() }

// AddSteeringRule inserts a rule and returns its new id.
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

// UpdateSteeringRuleEnabled toggles a rule's enabled flag.
func (db *DB) UpdateSteeringRuleEnabled(id int64, enabled bool) {
	e := 0
	if enabled {
		e = 1
	}
	if _, err := db.conn.Exec("UPDATE steering_rules SET enabled = ? WHERE id = ?", e, id); err != nil {
		slog.Error("update steering rule failed", "error", err)
	}
}

// DeleteSteeringRule removes a rule by id.
func (db *DB) DeleteSteeringRule(id int64) {
	if _, err := db.conn.Exec("DELETE FROM steering_rules WHERE id = ?", id); err != nil {
		slog.Error("delete steering rule failed", "error", err)
	}
}

// --- notifications -------------------------------------------------------

// AddNotification stores a new unread notification.
func (db *DB) AddNotification(notifType, title, message string) error {
	_, err := db.conn.Exec(
		"INSERT INTO notifications (type, title, message, created_at, read) VALUES (?, ?, ?, ?, 0)",
		notifType, title, message, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetNotifications returns all notifications, newest first.
func (db *DB) GetNotifications() ([]models.Notification, error) {
	rows, err := db.conn.Query("SELECT id, type, title, message, created_at, read FROM notifications ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Notification
	for rows.Next() {
		var n models.Notification
		var read int
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.CreatedAt, &read); err != nil {
			return nil, err
		}
		n.Read = read == 1
		out = append(out, n)
	}
	return out, nil
}

// MarkNotificationRead marks one notification read.
func (db *DB) MarkNotificationRead(id int64) error {
	_, err := db.conn.Exec("UPDATE notifications SET read = 1 WHERE id = ?", id)
	return err
}

// MarkAllNotificationsRead marks every notification read.
func (db *DB) MarkAllNotificationsRead() error {
	_, err := db.conn.Exec("UPDATE notifications SET read = 1")
	return err
}

// DeleteNotification deletes one notification.
func (db *DB) DeleteNotification(id int64) error {
	_, err := db.conn.Exec("DELETE FROM notifications WHERE id = ?", id)
	return err
}

// ClearAllNotifications deletes every notification.
func (db *DB) ClearAllNotifications() error {
	_, err := db.conn.Exec("DELETE FROM notifications")
	return err
}
