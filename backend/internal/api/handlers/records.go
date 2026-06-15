package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/sohidul/dns-server/internal/db/models"
	recordsdomain "github.com/sohidul/dns-server/internal/domain/records"
)

// GetRecords returns all custom DNS records as a domain->IP map.
func (h *Handler) GetRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.records.List(r.Context())
	if err != nil {
		slog.Error("list records failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve records"})
		return
	}
	out := make(map[string]string, len(records))
	for _, rec := range records {
		out[rec.Domain] = rec.IP
	}
	respond(w, http.StatusOK, out)
}

// AddRecord creates a custom DNS record (A or AAAA, inferred from the IP).
func (h *Handler) AddRecord(w http.ResponseWriter, r *http.Request) {
	var body models.AddRecordRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.records.Add(r.Context(), body.Domain, body.IP); err != nil {
		if writeRecordError(w, err) {
			return
		}
		slog.Error("add record failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to add record"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// DeleteRecord removes a custom DNS record.
func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	var body models.DeleteRecordRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.records.Delete(r.Context(), body.Domain); err != nil {
		if writeRecordError(w, err) {
			return
		}
		slog.Error("delete record failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete record"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// writeRecordError maps domain validation errors to 400 responses.
func writeRecordError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, recordsdomain.ErrInvalidDomain):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid domain name"})
		return true
	case errors.Is(err, recordsdomain.ErrInvalidIP):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid IP address"})
		return true
	}
	return false
}
