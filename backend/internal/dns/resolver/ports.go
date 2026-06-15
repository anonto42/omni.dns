package resolver

import (
	"github.com/sohidul/dns-server/internal/db/models"
)

// Blocklist reports whether a domain is blocked.
type Blocklist interface {
	IsBlocked(domain string) bool
}

// CustomRecords resolves locally configured records for a domain and query type.
// found is false when no record of that type exists. existsOtherType is true
// when the domain has a record of a different type (so an empty answer should be
// NODATA rather than a fall-through).
type CustomRecords interface {
	Lookup(domain string, qtype uint16) (ip string, found bool, existsOtherType bool)
}

// SteeringRules returns the active steering rules in priority order.
type SteeringRules interface {
	Rules() []models.SteeringRule
}

// QueryLogger records a single resolved query. Implementations must be
// non-blocking from the caller's perspective.
type QueryLogger interface {
	LogQuery(models.QueryLog)
}

// MACResolver maps a client IP to a MAC address ("" if unknown).
type MACResolver interface {
	Lookup(ip string) string
}
