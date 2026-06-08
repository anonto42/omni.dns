package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/dns"
	"github.com/sohidul/dns-server/internal/models"
)

const (
	maxDomainLen = 253
	maxBodyBytes = 1 << 16
)

type Handler struct {
	db  *db.DB
	dns *dns.Handler
}

func NewHandler(database *db.DB, dnsHandler *dns.Handler) *Handler {
	return &Handler{db: database, dns: dnsHandler}
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// GetStatus godoc
// @Summary      Get server status and stats
// @Description  Returns query statistics, cache info, and uptime
// @Produce      json
// @Success      200  {object}  models.Stats
// @Router       /api/status [get]
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.db.GetStats()
	stats.CacheSize = h.dns.CacheSize()
	stats.CacheHits = h.dns.CacheHits()
	stats.CacheMisses = h.dns.CacheMisses()
	stats.UptimeSeconds = h.dns.UptimeSeconds()
	respond(w, 200, stats)
}

// Health godoc
// @Summary      Health check
// @Description  Returns the service status
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]string{"status": "ok"})
}

// GetLogs godoc
// @Summary      Get query logs
// @Description  Returns the most recent DNS query logs
// @Produce      json
// @Success      200  {array}  models.QueryLog
// @Router       /api/logs [get]
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetLogs(100))
}

// ClearLogs godoc
// @Summary      Clear query logs
// @Description  Deletes all DNS query logs from the database
// @Produce      json
// @Success      200  {object}  map[string]bool
// @Router       /api/logs [delete]
func (h *Handler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	h.db.ClearLogs()
	respond(w, 200, map[string]bool{"ok": true})
}

// GetRecords godoc
// @Summary      Get custom DNS records
// @Description  Returns all manually configured DNS records
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /api/records [get]
func (h *Handler) GetRecords(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetCustomRecords())
}

func validDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > maxDomainLen {
		return false
	}
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func validIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// AddRecord godoc
// @Summary      Add custom DNS record
// @Description  Creates a new custom DNS entry
// @Accept       json
// @Produce      json
// @Param        record  body      models.AddRecordRequest  true  "Domain and IP"
// @Success      200     {object}  map[string]bool
// @Failure      400     {object}  map[string]string
// @Router       /api/records [post]
func (h *Handler) AddRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body models.AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	body.Domain = strings.ToLower(strings.TrimSpace(body.Domain))
	body.IP = strings.TrimSpace(body.IP)

	if !validDomain(body.Domain) {
		respond(w, 400, map[string]string{"error": "invalid domain name"})
		return
	}
	if !validIP(body.IP) {
		respond(w, 400, map[string]string{"error": "invalid IP address"})
		return
	}

	h.db.AddCustomRecord(body.Domain, body.IP)
	respond(w, 200, map[string]bool{"ok": true})
}

// DeleteRecord godoc
// @Summary      Delete custom DNS record
// @Description  Removes a custom DNS entry
// @Accept       json
// @Produce      json
// @Param        record  body      models.DeleteRecordRequest  true  "Domain name"
// @Success      200     {object}  map[string]bool
// @Failure      400     {object}  map[string]string
// @Router       /api/records [delete]
func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body models.DeleteRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	body.Domain = strings.ToLower(strings.TrimSpace(body.Domain))

	if !validDomain(body.Domain) {
		respond(w, 400, map[string]string{"error": "invalid domain name"})
		return
	}

	h.db.DeleteCustomRecord(body.Domain)
	respond(w, 200, map[string]bool{"ok": true})
}

// GetBlocklist godoc
// @Summary      Get blocklist
// @Description  Returns all domains in the blocklist
// @Produce      json
// @Success      200  {array}  models.BlockedDomain
// @Router       /api/blocklist [get]
func (h *Handler) GetBlocklist(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetBlocklist())
}

// AddToBlocklist godoc
// @Summary      Add to blocklist
// @Description  Blocks a new domain
// @Accept       json
// @Produce      json
// @Param        block  body      models.AddBlockRequest  true  "Domain and Wildcard flag"
// @Success      200    {object}  map[string]bool
// @Failure      400    {object}  map[string]string
// @Router       /api/blocklist [post]
func (h *Handler) AddToBlocklist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body models.AddBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	body.Domain = strings.ToLower(strings.TrimSpace(body.Domain))

	if !validDomain(body.Domain) {
		respond(w, 400, map[string]string{"error": "invalid domain name"})
		return
	}

	h.db.AddToBlocklist(body.Domain, body.Wildcard)
	respond(w, 200, map[string]bool{"ok": true})
}

// RemoveFromBlocklist godoc
// @Summary      Remove from blocklist
// @Description  Unblocks a domain
// @Accept       json
// @Produce      json
// @Param        domain  body      models.RemoveBlockRequest  true  "Domain name"
// @Success      200     {object}  map[string]bool
// @Failure      400     {object}  map[string]string
// @Router       /api/blocklist [delete]
func (h *Handler) RemoveFromBlocklist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body models.RemoveBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	body.Domain = strings.ToLower(strings.TrimSpace(body.Domain))

	if !validDomain(body.Domain) {
		respond(w, 400, map[string]string{"error": "invalid domain name"})
		return
	}

	h.db.RemoveFromBlocklist(body.Domain)
	respond(w, 200, map[string]bool{"ok": true})
}
