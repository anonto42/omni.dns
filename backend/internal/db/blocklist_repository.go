package db

import (
	"context"
	"time"

	blocklistdomain "github.com/sohidul/dns-server/internal/domain/blocklist"
)

type BlocklistRepository struct {
	db *DB
}

func NewBlocklistRepository(database *DB) *BlocklistRepository {
	return &BlocklistRepository{db: database}
}

func (r *BlocklistRepository) List(ctx context.Context) ([]blocklistdomain.Entry, error) {
	rows, err := r.db.conn.QueryContext(ctx, "SELECT domain, added_at, wildcard FROM blocklist ORDER BY domain")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]blocklistdomain.Entry, 0)
	for rows.Next() {
		var domain, addedAt string
		var wildcardInt int
		if err := rows.Scan(&domain, &addedAt, &wildcardInt); err != nil {
			return nil, err
		}
		// Reconstitute the aggregate from trusted, already-persisted values.
		d, err := blocklistdomain.NewDomain(domain)
		if err != nil {
			continue
		}
		var added time.Time
		if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
			added = t
		}
		entries = append(entries, blocklistdomain.NewFromValues(d, wildcardInt != 0, added))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *BlocklistRepository) Save(ctx context.Context, entry blocklistdomain.Entry) error {
	wildcard := 0
	if entry.Wildcard() {
		wildcard = 1
	}
	_, err := r.db.conn.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO blocklist (domain, added_at, wildcard) VALUES (?, ?, ?)",
		entry.Domain().String(),
		time.Now().Format(time.RFC3339),
		wildcard,
	)
	return err
}

func (r *BlocklistRepository) Delete(ctx context.Context, domain blocklistdomain.Domain) error {
	_, err := r.db.conn.ExecContext(ctx, "DELETE FROM blocklist WHERE domain = ?", domain.String())
	return err
}
