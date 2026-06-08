package models

import "time"

type Action string

const (
	ActionForwarded Action = "forwarded"
	ActionBlocked   Action = "blocked"
	ActionCustom    Action = "custom"
	ActionCached    Action = "cached"
	ActionError     Action = "error"
)

type QueryLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Domain    string    `json:"domain"`
	ClientIP  string    `json:"client_ip"`
	Action    Action    `json:"action"`
}

type CustomRecord struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
	Type   string `json:"type"`
}

type BlockedDomain struct {
	Domain   string    `json:"domain"`
	AddedAt  time.Time `json:"added_at"`
	Wildcard bool      `json:"wildcard"`
}

type Stats struct {
	QueriesForwarded int     `json:"queries_forwarded"`
	QueriesBlocked   int     `json:"queries_blocked"`
	QueriesCustom    int     `json:"queries_custom"`
	QueriesCached    int     `json:"queries_cached"`
	CacheSize        int     `json:"cache_size"`
	CacheHits        int64   `json:"cache_hits"`
	CacheMisses      int64   `json:"cache_misses"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
}
