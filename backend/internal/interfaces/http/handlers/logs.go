package handlers

import (
	"net/http"
	"strconv"
)

// GetStatus returns query statistics, cache metrics, and uptime.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.db.GetStats()
	stats.CacheSize = h.resolver.CacheSize()
	stats.CacheHits = h.resolver.CacheHits()
	stats.CacheMisses = h.resolver.CacheMisses()
	stats.UptimeSeconds = h.resolver.UptimeSeconds()
	respond(w, http.StatusOK, stats)
}

// GetLogs returns recent query logs, optionally filtered by limit/action/domain.
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	action := r.URL.Query().Get("action")
	domain := r.URL.Query().Get("domain")
	respond(w, http.StatusOK, h.db.GetLogsFiltered(limit, action, domain))
}

// ClearLogs deletes all query logs.
func (h *Handler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	h.db.ClearLogs()
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
