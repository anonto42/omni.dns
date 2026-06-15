package persistence

import "context"

// LookupRecord resolves a custom record for (domain, qtype). It returns the IP
// and found=true on a direct hit; otherwise existsOtherType reports whether the
// domain has a record of a different type (so the caller can answer NODATA
// instead of falling through to upstream).
func (db *DB) LookupRecord(ctx context.Context, domain string, qtype uint16) (ip string, found bool, existsOtherType bool) {
	row := db.conn.QueryRowContext(ctx, "SELECT ip FROM custom_records WHERE domain = ? AND qtype = ?", domain, qtype)
	if err := row.Scan(&ip); err == nil {
		return ip, true, false
	}

	var other int
	if err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM custom_records WHERE domain = ?", domain).Scan(&other); err != nil {
		return "", false, false
	}
	return "", false, other > 0
}

// Lookup adapts DB to the resolver's CustomRecords port.
func (db *DB) Lookup(domain string, qtype uint16) (string, bool, bool) {
	return db.LookupRecord(context.Background(), domain, qtype)
}
