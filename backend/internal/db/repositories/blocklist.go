package repositories

import (
	"context"
	"time"

	"github.com/sohidul/dns-server/internal/db"
	blocklistdomain "github.com/sohidul/dns-server/internal/domain/blocklist"
)

// Blocklist persists blocklist entries.
type Blocklist struct {
	db *db.DB
}

// NewBlocklist builds a blocklist repository over the given database.
func NewBlocklist(database *db.DB) *Blocklist {
	return &Blocklist{db: database}
}

// List returns all blocklist entries ordered by domain.
func (r *Blocklist) List(ctx context.Context) ([]blocklistdomain.Entry, error) {
	rows, err := r.db.Conn().QueryContext(ctx, "SELECT domain, added_at, wildcard FROM blocklist ORDER BY domain")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]blocklistdomain.Entry, 0)
	for rows.Next() {
		var domain, addedAt string
		var wildcard int
		if err := rows.Scan(&domain, &addedAt, &wildcard); err != nil {
			return nil, err
		}
		d, err := blocklistdomain.NewDomain(domain)
		if err != nil {
			continue
		}
		var added time.Time
		if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
			added = t
		}
		entries = append(entries, blocklistdomain.NewFromValues(d, wildcard != 0, added))
	}
	return entries, rows.Err()
}

// Save inserts an entry, ignoring duplicates.
func (r *Blocklist) Save(ctx context.Context, entry blocklistdomain.Entry) error {
	wildcard := 0
	if entry.Wildcard() {
		wildcard = 1
	}
	_, err := r.db.Conn().ExecContext(
		ctx,
		"INSERT OR IGNORE INTO blocklist (domain, added_at, wildcard) VALUES (?, ?, ?)",
		entry.Domain().String(), time.Now().Format(time.RFC3339), wildcard,
	)
	return err
}

// Delete removes a blocklist entry by domain.
func (r *Blocklist) Delete(ctx context.Context, domain blocklistdomain.Domain) error {
	_, err := r.db.Conn().ExecContext(ctx, "DELETE FROM blocklist WHERE domain = ?", domain.String())
	return err
}
