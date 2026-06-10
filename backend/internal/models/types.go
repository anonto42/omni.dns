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
	ID         int64     `json:"id" example:"1"`
	Timestamp  time.Time `json:"timestamp" example:"2024-10-25T10:00:00Z"`
	Domain     string    `json:"domain" example:"example.com"`
	ClientIP   string    `json:"client_ip" example:"192.168.1.5"`
	MACAddress string    `json:"mac_address" example:"aa:bb:cc:dd:ee:ff"`
	Action     Action    `json:"action" example:"forwarded"`
}

type CustomRecord struct {
	Domain string `json:"domain" example:"mydevice.local"`
	IP     string `json:"ip" example:"192.168.1.100"`
	Type   string `json:"type" example:"A"`
}

type BlockedDomain struct {
	Domain   string    `json:"domain" example:"ads.example.com"`
	AddedAt  time.Time `json:"added_at" example:"2024-10-25T10:00:00Z"`
	Wildcard bool      `json:"wildcard" example:"false"`
}

type Stats struct {
	QueriesForwarded int     `json:"queries_forwarded" example:"150"`
	QueriesBlocked   int     `json:"queries_blocked" example:"20"`
	QueriesCustom    int     `json:"queries_custom" example:"5"`
	QueriesCached    int     `json:"queries_cached" example:"45"`
	CacheSize        int     `json:"cache_size" example:"100"`
	CacheHits        int64   `json:"cache_hits" example:"45"`
	CacheMisses      int64   `json:"cache_misses" example:"175"`
	UptimeSeconds    float64 `json:"uptime_seconds" example:"3600.5"`
}

// AddRecordRequest is the body for adding a custom DNS record.
type AddRecordRequest struct {
	Domain string `json:"domain" example:"mydevice.local" binding:"required"`
	IP     string `json:"ip" example:"192.168.1.100" binding:"required"`
}

// DeleteRecordRequest is the body for deleting a custom DNS record.
type DeleteRecordRequest struct {
	Domain string `json:"domain" example:"mydevice.local" binding:"required"`
}

// AddBlockRequest is the body for adding a domain to the blocklist.
type AddBlockRequest struct {
	Domain   string `json:"domain" example:"ads.example.com" binding:"required"`
	Wildcard bool   `json:"wildcard" example:"false"`
}

// RemoveBlockRequest is the body for removing a domain from the blocklist.
type RemoveBlockRequest struct {
	Domain string `json:"domain" example:"ads.example.com" binding:"required"`
}

// SteeringRule represents a persistent DNS traffic-steering rule.
type SteeringRule struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	ConditionType  string `json:"condition_type"`  // Domain | Client IP | Query Type | Time Range
	ConditionValue string `json:"condition_value"` // e.g. *.corp.internal
	ActionType     string `json:"action_type"`     // Forward | Block | Redirect
	ActionTarget   string `json:"action_target"`   // IP / upstream addr, empty if Block
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
}

type AddSteeringRuleRequest struct {
	Name           string `json:"name"`
	ConditionType  string `json:"condition_type"`
	ConditionValue string `json:"condition_value"`
	ActionType     string `json:"action_type"`
	ActionTarget   string `json:"action_target"`
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
}

type UpdateSteeringRuleRequest struct {
	ID      int64 `json:"id"`
	Enabled bool  `json:"enabled"`
}

type DeleteSteeringRuleRequest struct {
	ID int64 `json:"id"`
}

// ChangePasswordRequest is the body for changing the admin password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
