package steering

import "context"

// Repository is the outbound port for persisting steering rules.
type Repository interface {
	List(ctx context.Context) ([]Rule, error)
	Add(ctx context.Context, rule Rule) (int64, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) error
	Delete(ctx context.Context, id int64) error
}

// Notifier is the outbound port for emitting user-facing notifications.
type Notifier interface {
	Notify(ctx context.Context, notifType, title, message string) error
}
