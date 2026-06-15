// Package config loads and validates all runtime configuration from command-line
// flags and environment variables. Flags take precedence over environment
// variables, which take precedence over built-in defaults.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the server.
type Config struct {
	// DNS listener
	DNSPort int
	DNSAddr string

	// HTTP/API listener
	HTTPPort int

	// Storage
	DBPath string

	// Resolver behaviour
	BlockNX     bool
	CacheSize   int
	UpstreamDNS string
	UpstreamTLS bool

	// Static UI
	StaticDir string

	// Logging
	LogPrune  time.Duration
	LogFormat string
	LogLevel  string

	// Admin bootstrap
	AdminEmail string
	AdminPass  string

	// Security / CORS
	AllowedOrigin string

	// Sessions
	SessionTTL time.Duration

	// ARP / MAC lookups
	EnableMACLookup bool
	ARPRefresh      time.Duration

	// Log buffering
	LogFlushInterval time.Duration
	LogFlushSize     int
}

const (
	defaultAdminEmail    = "admin@omnidns.local"
	defaultAllowedOrigin = "http://localhost:5173"
	minPasswordLen       = 8
)

// Load reads configuration from flags and the environment and returns a
// validated Config. It calls flag.Parse, so it must be invoked exactly once.
func Load() (Config, error) {
	cfg := Config{}

	flag.IntVar(&cfg.DNSPort, "dns-port", envInt("OMNIDNS_DNS_PORT", 53), "DNS server port")
	flag.StringVar(&cfg.DNSAddr, "dns-addr", os.Getenv("OMNIDNS_DNS_ADDR"), "DNS bind address (default: all interfaces)")
	flag.IntVar(&cfg.HTTPPort, "http-port", envInt("OMNIDNS_HTTP_PORT", 8080), "HTTP API port")
	flag.StringVar(&cfg.DBPath, "db", envStr("OMNIDNS_DB_PATH", "data/dns.db"), "SQLite database path")
	flag.BoolVar(&cfg.BlockNX, "block-nxdomain", envBool("OMNIDNS_BLOCK_NXDOMAIN", false), "Return NXDOMAIN for blocked domains")
	flag.IntVar(&cfg.CacheSize, "cache-size", envInt("OMNIDNS_CACHE_SIZE", 1000), "DNS cache size")
	flag.StringVar(&cfg.UpstreamDNS, "upstream", envStr("OMNIDNS_UPSTREAM", "1.1.1.1:853"), "Upstream DNS server (host:port)")
	flag.BoolVar(&cfg.UpstreamTLS, "upstream-tls", envBool("OMNIDNS_UPSTREAM_TLS", true), "Use DNS-over-TLS for upstream queries")
	flag.StringVar(&cfg.StaticDir, "static", os.Getenv("OMNIDNS_STATIC_DIR"), "Directory with static files to serve at /")
	flag.DurationVar(&cfg.LogPrune, "log-prune", envDuration("OMNIDNS_LOG_PRUNE", 0), "Auto-prune logs older than this (e.g. 72h)")
	flag.StringVar(&cfg.LogFormat, "log-format", envStr("OMNIDNS_LOG_FORMAT", "text"), "Log format: text or json")
	flag.StringVar(&cfg.LogLevel, "log-level", envStr("OMNIDNS_LOG_LEVEL", "info"), "Log level: debug, info, warn, error")
	flag.StringVar(&cfg.AdminEmail, "admin-email", "", "Initial admin email (or OMNIDNS_ADMIN_EMAIL)")
	flag.StringVar(&cfg.AdminPass, "admin-password", "", "Initial admin password (or OMNIDNS_ADMIN_PASSWORD)")
	flag.StringVar(&cfg.AllowedOrigin, "allowed-origin", "", "Allowed CORS origin (or OMNIDNS_ALLOWED_ORIGIN)")
	flag.DurationVar(&cfg.SessionTTL, "session-ttl", envDuration("OMNIDNS_SESSION_TTL", 24*time.Hour), "Session lifetime before re-login is required")
	flag.BoolVar(&cfg.EnableMACLookup, "enable-mac-lookup", envBool("OMNIDNS_ENABLE_MAC_LOOKUP", true), "Resolve client MAC addresses from the ARP table (Linux only)")
	flag.DurationVar(&cfg.ARPRefresh, "arp-refresh", envDuration("OMNIDNS_ARP_REFRESH", 30*time.Second), "How often to refresh the cached ARP table")
	flag.DurationVar(&cfg.LogFlushInterval, "log-flush-interval", envDuration("OMNIDNS_LOG_FLUSH_INTERVAL", 5*time.Second), "How often to flush buffered query logs")
	flag.IntVar(&cfg.LogFlushSize, "log-flush-size", envInt("OMNIDNS_LOG_FLUSH_SIZE", 100), "Flush buffered query logs once this many accumulate")
	flag.Parse()

	cfg.AdminEmail = strings.TrimSpace(firstNonEmpty(cfg.AdminEmail, os.Getenv("OMNIDNS_ADMIN_EMAIL"), defaultAdminEmail))
	cfg.AdminPass = firstNonEmpty(cfg.AdminPass, os.Getenv("OMNIDNS_ADMIN_PASSWORD"))
	cfg.AllowedOrigin = strings.TrimSpace(firstNonEmpty(cfg.AllowedOrigin, os.Getenv("OMNIDNS_ALLOWED_ORIGIN"), defaultAllowedOrigin))

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate enforces invariants that must hold before the server starts.
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.AdminPass) == "" {
		return fmt.Errorf("initial admin password is required; set OMNIDNS_ADMIN_PASSWORD or --admin-password")
	}
	if len(cfg.AdminPass) < minPasswordLen {
		return fmt.Errorf("initial admin password must be at least %d characters", minPasswordLen)
	}
	if cfg.AllowedOrigin == "*" {
		return fmt.Errorf("CORS allowed origin must not be '*' with bearer-token auth; set a concrete origin via OMNIDNS_ALLOWED_ORIGIN")
	}
	if cfg.SessionTTL <= 0 {
		return fmt.Errorf("session TTL must be positive")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func envStr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
