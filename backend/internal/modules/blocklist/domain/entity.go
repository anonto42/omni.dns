package blocklist

import "time"

// Entry is the aggregate root for a single blocklist rule. It is composed of a
// validated Domain value object, so a constructed Entry is always valid.
type Entry struct {
	domain   Domain
	wildcard bool
	addedAt  time.Time
}

// New builds an Entry from a raw domain string, enforcing all invariants.
func New(domain string, wildcard bool) (Entry, error) {
	d, err := NewDomain(domain)
	if err != nil {
		return Entry{}, err
	}
	return Entry{domain: d, wildcard: wildcard}, nil
}

// NewFromValues reconstitutes an Entry from already-validated/persisted values.
func NewFromValues(domain Domain, wildcard bool, addedAt time.Time) Entry {
	return Entry{domain: domain, wildcard: wildcard, addedAt: addedAt}
}

// Domain returns the entry's domain value object.
func (e Entry) Domain() Domain { return e.domain }

// Wildcard reports whether the rule matches subdomains.
func (e Entry) Wildcard() bool { return e.wildcard }

// AddedAt returns when the entry was created (zero value if not yet persisted).
func (e Entry) AddedAt() time.Time { return e.addedAt }
