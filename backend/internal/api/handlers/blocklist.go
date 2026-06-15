package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/sohidul/dns-server/internal/db/models"
	blocklistdomain "github.com/sohidul/dns-server/internal/domain/blocklist"
)

// GetBlocklist returns all blocked domains.
func (h *Handler) GetBlocklist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.blocklist.List(r.Context())
	if err != nil {
		slog.Error("list blocklist failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve blocklist"})
		return
	}
	out := make([]models.BlockedDomain, 0, len(entries))
	for _, e := range entries {
		out = append(out, models.BlockedDomain{Domain: e.Domain, AddedAt: e.AddedAt, Wildcard: e.Wildcard})
	}
	respond(w, http.StatusOK, out)
}

// AddToBlocklist blocks a domain (optionally as a wildcard).
func (h *Handler) AddToBlocklist(w http.ResponseWriter, r *http.Request) {
	var body models.AddBlockRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.blocklist.Add(r.Context(), body.Domain, body.Wildcard); err != nil {
		if errors.Is(err, blocklistdomain.ErrInvalidDomain) {
			respond(w, http.StatusBadRequest, map[string]string{"error": "invalid domain name"})
			return
		}
		slog.Error("add to blocklist failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to add to blocklist"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// RemoveFromBlocklist unblocks a domain.
func (h *Handler) RemoveFromBlocklist(w http.ResponseWriter, r *http.Request) {
	var body models.RemoveBlockRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.blocklist.Remove(r.Context(), body.Domain); err != nil {
		if errors.Is(err, blocklistdomain.ErrInvalidDomain) {
			respond(w, http.StatusBadRequest, map[string]string{"error": "invalid domain name"})
			return
		}
		slog.Error("remove from blocklist failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove from blocklist"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
