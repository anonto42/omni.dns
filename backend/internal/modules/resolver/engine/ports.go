package resolver

import "github.com/sohidul/dns-server/internal/modules/resolver/domain"

// The resolution ports are defined in the resolver domain package. These aliases
// let the engine (and its tests) refer to them unqualified while keeping the
// canonical interface definitions in domain/.
type (
	Blocklist     = domain.Blocklist
	CustomRecords = domain.CustomRecords
	SteeringRules = domain.SteeringRules
	QueryLogger   = domain.QueryLogger
	MACResolver   = domain.MACResolver
)
