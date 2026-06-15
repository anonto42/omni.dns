package handlers

import (
	"net/http"
	"strings"
)

// GetSettings returns all persisted settings.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, h.db.GetSettings())
}

// SaveSettings persists settings and applies upstream/blocking changes to the
// running resolver immediately, without a restart.
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if !decodeJSON(w, r, &body) {
		return
	}
	h.db.SaveSettings(body)
	h.notify("success", "Settings Saved", "Upstream DNS and blocking behaviors updated successfully.")

	if addr, ok := body["upstream_dns"]; ok && addr != "" {
		tls := strings.HasSuffix(addr, ":853")
		if !strings.Contains(addr, ":") {
			addr += ":853"
			tls = true
		}
		h.resolver.SetPrimaryUpstream(addr, tls)
	}
	if v, ok := body["block_nxdomain"]; ok {
		h.resolver.SetBlockNXDOMAIN(v == "true")
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
