package db

import (
	"context"
	"time"

	recordsdomain "github.com/sohidul/dns-server/internal/domain/records"
)

type RecordsRepository struct {
	db *DB
}

func NewRecordsRepository(database *DB) *RecordsRepository {
	return &RecordsRepository{db: database}
}

func (r *RecordsRepository) List(ctx context.Context) ([]recordsdomain.Record, error) {
	rows, err := r.db.conn.QueryContext(ctx, "SELECT domain, ip FROM custom_records")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]recordsdomain.Record, 0)
	for rows.Next() {
		var domain, ip string
		if err := rows.Scan(&domain, &ip); err != nil {
			return nil, err
		}
		// Reconstitute the aggregate from trusted, already-persisted values.
		d, err := recordsdomain.NewDomain(domain)
		if err != nil {
			continue
		}
		addr, err := recordsdomain.NewIP(ip)
		if err != nil {
			continue
		}
		records = append(records, recordsdomain.NewFromValues(d, addr))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *RecordsRepository) Save(ctx context.Context, record recordsdomain.Record) error {
	_, err := r.db.conn.ExecContext(
		ctx,
		"INSERT OR REPLACE INTO custom_records (domain, ip) VALUES (?, ?)",
		record.Domain().String(),
		record.IP().String(),
	)
	return err
}

func (r *RecordsRepository) Delete(ctx context.Context, domain recordsdomain.Domain) error {
	_, err := r.db.conn.ExecContext(ctx, "DELETE FROM custom_records WHERE domain = ?", domain.String())
	return err
}

type NotificationRepository struct {
	db *DB
}

func NewNotificationRepository(database *DB) *NotificationRepository {
	return &NotificationRepository{db: database}
}

func (r *NotificationRepository) Notify(ctx context.Context, notifType, title, message string) error {
	_, err := r.db.conn.ExecContext(
		ctx,
		"INSERT INTO notifications (type, title, message, created_at, read) VALUES (?, ?, ?, ?, 0)",
		notifType,
		title,
		message,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
