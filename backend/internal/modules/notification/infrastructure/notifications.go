package infrastructure

import (
	"context"

	db "github.com/sohidul/dns-server/internal/infrastructure/persistence"
)

// Notifications adapts the database to the domain Notifier port.
type Notifications struct {
	db *db.DB
}

// NewNotifications builds a notification repository over the given database.
func NewNotifications(database *db.DB) *Notifications {
	return &Notifications{db: database}
}

// Notify persists a user-facing notification. The context is accepted for
// interface symmetry; the underlying write is fast and unbounded.
func (r *Notifications) Notify(_ context.Context, notifType, title, message string) error {
	return r.db.AddNotification(notifType, title, message)
}
