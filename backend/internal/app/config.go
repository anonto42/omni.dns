package app

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DNSPort     int
	DNSAddr     string
	HTTPPort    int
	DBPath      string
	BlockNX     bool
	CacheSize   int
	UpstreamDNS string
	UpstreamTLS bool
	StaticDir   string
	LogPrune    time.Duration
	LogFormat   string
	LogLevel    string
	AdminEmail  string
	AdminPass   string
}

func LoadConfig() Config {
	cfg := Config{}

	flag.IntVar(&cfg.DNSPort, "dns-port", 53, "DNS server port")
	flag.StringVar(&cfg.DNSAddr, "dns-addr", "", "DNS bind address (default: all interfaces)")
	flag.IntVar(&cfg.HTTPPort, "http-port", 8080, "HTTP API port")
	flag.StringVar(&cfg.DBPath, "db", "data/dns.db", "SQLite database path")
	flag.BoolVar(&cfg.BlockNX, "block-nxdomain", false, "Return NXDOMAIN for blocked domains")
	flag.IntVar(&cfg.CacheSize, "cache-size", 1000, "DNS cache size")
	flag.StringVar(&cfg.UpstreamDNS, "upstream", "1.1.1.1:853", "Upstream DNS server (host:port)")
	flag.BoolVar(&cfg.UpstreamTLS, "upstream-tls", true, "Use DNS-over-TLS for upstream queries")
	flag.StringVar(&cfg.StaticDir, "static", "", "Directory with static files to serve at /")
	flag.DurationVar(&cfg.LogPrune, "log-prune", 0, "Auto-prune logs older than this (e.g. 72h)")
	flag.StringVar(&cfg.LogFormat, "log-format", "text", "Log format: text or json")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.StringVar(&cfg.AdminEmail, "admin-email", "", "Initial admin email (or OMNIDNS_ADMIN_EMAIL)")
	flag.StringVar(&cfg.AdminPass, "admin-password", "", "Initial admin password (or OMNIDNS_ADMIN_PASSWORD)")
	flag.Parse()

	cfg.AdminEmail = strings.TrimSpace(firstNonEmpty(cfg.AdminEmail, os.Getenv("OMNIDNS_ADMIN_EMAIL"), "admin@omnidns.local"))
	cfg.AdminPass = firstNonEmpty(cfg.AdminPass, os.Getenv("OMNIDNS_ADMIN_PASSWORD"))

	return cfg
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.AdminPass) == "" {
		return fmt.Errorf("initial admin password is required; set OMNIDNS_ADMIN_PASSWORD or --admin-password")
	}
	if len(cfg.AdminPass) < 8 {
		return fmt.Errorf("initial admin password must be at least 8 characters")
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
