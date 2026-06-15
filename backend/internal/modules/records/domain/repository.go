package records

import "context"

// Repository is the outbound port for persisting custom DNS records.
// Implementations live in the infrastructure layer.
type Repository interface {
	List(ctx context.Context) ([]Record, error)
	Save(ctx context.Context, record Record) error
	Delete(ctx context.Context, domain Domain) error
}

// Notifier is the outbound port for emitting user-facing notifications.
type Notifier interface {
	Notify(ctx context.Context, notifType, title, message string) error
}
