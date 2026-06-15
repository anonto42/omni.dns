// Package repositories contains SQLite-backed implementations of the domain
// outbound ports (records, blocklist, notifications).
package repositories

import (
	"context"

	"github.com/sohidul/dns-server/internal/db"
	recordsdomain "github.com/sohidul/dns-server/internal/domain/records"
)

// Records persists custom DNS records keyed by (domain, qtype).
type Records struct {
	db *db.DB
}

// NewRecords builds a records repository over the given database.
func NewRecords(database *db.DB) *Records {
	return &Records{db: database}
}

// List returns all custom records, reconstituting domain aggregates from
// trusted persisted values.
func (r *Records) List(ctx context.Context) ([]recordsdomain.Record, error) {
	rows, err := r.db.Conn().QueryContext(ctx, "SELECT domain, ip FROM custom_records")
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
	return records, rows.Err()
}

// Save upserts a record under its domain and address-family qtype, so a domain
// can hold independent A and AAAA records.
func (r *Records) Save(ctx context.Context, record recordsdomain.Record) error {
	_, err := r.db.Conn().ExecContext(
		ctx,
		"INSERT OR REPLACE INTO custom_records (domain, ip, qtype) VALUES (?, ?, ?)",
		record.Domain().String(), record.IP().String(), int(record.IP().QType()),
	)
	return err
}

// Delete removes all records (both families) for a domain.
func (r *Records) Delete(ctx context.Context, domain recordsdomain.Domain) error {
	_, err := r.db.Conn().ExecContext(ctx, "DELETE FROM custom_records WHERE domain = ?", domain.String())
	return err
}
