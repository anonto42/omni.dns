package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/sohidul/esp32-dns-server/internal/db"
	"github.com/sohidul/esp32-dns-server/internal/dns"
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

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.db.GetStats()
	stats.CacheSize = h.dns.CacheSize()
	stats.CacheHits = h.dns.CacheHits()
	stats.CacheMisses = h.dns.CacheMisses()
	stats.UptimeSeconds = h.dns.UptimeSeconds()
	respond(w, 200, stats)
}

func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetLogs(100))
}

func (h *Handler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	h.db.ClearLogs()
	respond(w, 200, map[string]bool{"ok": true})
}

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

func (h *Handler) AddRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body struct {
		Domain string `json:"domain"`
		IP     string `json:"ip"`
	}
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

func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body struct {
		Domain string `json:"domain"`
	}
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

func (h *Handler) GetBlocklist(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetBlocklist())
}

func (h *Handler) AddToBlocklist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body struct {
		Domain   string `json:"domain"`
		Wildcard bool   `json:"wildcard"`
	}
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

func (h *Handler) RemoveFromBlocklist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var body struct {
		Domain string `json:"domain"`
	}
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
