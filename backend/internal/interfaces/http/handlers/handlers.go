// Package handlers implements the REST API, split by resource. A single Handler
// holds the shared dependencies; each resource's endpoints live in their own
// file.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	db "github.com/sohidul/dns-server/internal/infrastructure/persistence"
	blocklistapp "github.com/sohidul/dns-server/internal/modules/blocklist/application"
	recordsapp "github.com/sohidul/dns-server/internal/modules/records/application"
	steeringapp "github.com/sohidul/dns-server/internal/modules/steering/application"
)

const maxBodyBytes = 1 << 16

// Resolver is the subset of the DNS resolver the API needs: runtime controls
// and metrics for the status endpoint.
type Resolver interface {
	SetPrimaryUpstream(addr string, tls bool)
	SetBlockNXDOMAIN(v bool)
	CacheSize() int
	CacheHits() int64
	CacheMisses() int64
	UptimeSeconds() float64
}

// Handler bundles the dependencies shared by all resource handlers.
type Handler struct {
	db        *db.DB
	resolver  Resolver
	records   *recordsapp.Service
	blocklist *blocklistapp.Service
	steering  *steeringapp.Service
}

// New builds the API handler from the database, resolver, and application
// services.
func New(database *db.DB, resolver Resolver, records *recordsapp.Service, blocklist *blocklistapp.Service, steering *steeringapp.Service) *Handler {
	return &Handler{
		db:        database,
		resolver:  resolver,
		records:   records,
		blocklist: blocklist,
		steering:  steering,
	}
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encode json response failed", "error", err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func (h *Handler) notify(notifType, title, message string) {
	if err := h.db.AddNotification(notifType, title, message); err != nil {
		slog.Error("add notification failed", "type", notifType, "title", title, "error", err)
	}
}

// Health reports service liveness.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
