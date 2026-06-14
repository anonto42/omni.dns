package blocklist

import "context"

// Repository is the outbound port for persisting blocklist entries.
// Implementations live in the infrastructure layer.
type Repository interface {
	List(ctx context.Context) ([]Entry, error)
	Save(ctx context.Context, entry Entry) error
	Delete(ctx context.Context, domain Domain) error
}

// Notifier is the outbound port for emitting user-facing notifications.
type Notifier interface {
	Notify(ctx context.Context, notifType, title, message string) error
}
