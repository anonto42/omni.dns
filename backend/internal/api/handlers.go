package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
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
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
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
	// Allow wildcard prefix (*.example.com) — strip it before validating the rest.
	d := domain
	if strings.HasPrefix(d, "*.") {
		d = d[2:]
	}
	if len(d) == 0 {
		return false
	}
	for _, c := range d {
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
	h.db.AddNotification("success", "DNS Record Created", fmt.Sprintf("Custom DNS record created for %s pointing to %s.", body.Domain, body.IP))
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
// func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
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
	h.db.AddNotification("info", "DNS Record Deleted", fmt.Sprintf("Custom DNS record for %s has been deleted.", body.Domain))
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
	h.db.AddNotification("warning", "Domain Blocked", fmt.Sprintf("Domain %s added to local blocklist (Wildcard: %t).", body.Domain, body.Wildcard))
	respond(w, 200, map[string]bool{"ok": true})
}

// Login godoc
// @Summary      Authenticate
// @Description  Validates credentials and returns a session token
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /api/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if !h.db.VerifyUser(body.Email, body.Password) {
		respond(w, 401, map[string]string{"error": "invalid email or password"})
		return
	}
	token, err := h.db.CreateSession(body.Email)
	if err != nil {
		respond(w, 500, map[string]string{"error": "failed to create session"})
		return
	}
	respond(w, 200, map[string]string{"token": token})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetSettings())
}

func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	h.db.SaveSettings(body)
	h.db.AddNotification("success", "Settings Saved", "Upstream DNS and blocking behaviors updated successfully.")

	// Apply changes immediately without restart.
	if addr, ok := body["upstream_dns"]; ok && addr != "" {
		tls := strings.HasSuffix(addr, ":853")
		if !strings.Contains(addr, ":") {
			addr = addr + ":853"
			tls = true
		}
		h.dns.SetPrimaryUpstream(addr, tls)
	}
	if v, ok := body["block_nxdomain"]; ok {
		h.dns.SetBlockNXDOMAIN(v == "true")
	}

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
	h.db.AddNotification("success", "Domain Unblocked", fmt.Sprintf("Domain %s removed from local blocklist.", body.Domain))
	respond(w, 200, map[string]bool{"ok": true})
}

// GetLogs with optional ?limit=N&action=blocked&domain=example query params
func (h *Handler) GetLogsFiltered(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	action := r.URL.Query().Get("action")
	domain := r.URL.Query().Get("domain")
	respond(w, 200, h.db.GetLogsFiltered(limit, action, domain))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != "" {
		h.db.DeleteSession(token)
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	email, _ := r.Context().Value(userKey).(string)
	if !h.db.VerifyUser(email, body.CurrentPassword) {
		respond(w, 401, map[string]string{"error": "current password is incorrect"})
		return
	}
	if len(body.NewPassword) < 8 {
		respond(w, 400, map[string]string{"error": "new password must be at least 8 characters"})
		return
	}
	if err := h.db.ChangePassword(email, body.NewPassword); err != nil {
		slog.Error("change password failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to change password"})
		return
	}
	h.db.AddNotification("info", "Password Changed", "Administrator account password updated successfully.")
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) GetSteeringRules(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, h.db.GetSteeringRules())
}

func (h *Handler) AddSteeringRule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.AddSteeringRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		respond(w, 400, map[string]string{"error": "name is required"})
		return
	}
	id, err := h.db.AddSteeringRule(body)
	if err != nil {
		slog.Error("add steering rule failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to add rule"})
		return
	}
	h.db.AddNotification("success", "Steering Rule Added", fmt.Sprintf("Added rule \"%s\" successfully.", body.Name))
	respond(w, 200, map[string]any{"ok": true, "id": id})
}

func (h *Handler) UpdateSteeringRule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.UpdateSteeringRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	h.db.UpdateSteeringRuleEnabled(body.ID, body.Enabled)
	status := "disabled"
	if body.Enabled {
		status = "enabled"
	}
	h.db.AddNotification("info", fmt.Sprintf("Steering Rule %s", strings.Title(status)), fmt.Sprintf("Traffic steering rule ID %d has been %s.", body.ID, status))
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) DeleteSteeringRule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.DeleteSteeringRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	h.db.DeleteSteeringRule(body.ID)
	h.db.AddNotification("info", "Steering Rule Deleted", fmt.Sprintf("Traffic steering rule ID %d has been deleted.", body.ID))
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	email, ok := r.Context().Value(userKey).(string)
	if !ok {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	name, err := h.db.GetProfile(email)
	if err != nil {
		name = "Administrator" // fallback
	}
	respond(w, 200, map[string]string{
		"email": email,
		"name":  name,
	})
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	oldEmail, ok := r.Context().Value(userKey).(string)
	if !ok {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))

	if body.Name == "" || body.Email == "" {
		respond(w, 400, map[string]string{"error": "name and email cannot be empty"})
		return
	}

	if !strings.Contains(body.Email, "@") {
		respond(w, 400, map[string]string{"error": "invalid email address"})
		return
	}

	err := h.db.UpdateProfile(oldEmail, body.Email, body.Name)
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}

	h.db.AddNotification("success", "Profile Updated", fmt.Sprintf("Account details updated: Name='%s', Email='%s'.", body.Name, body.Email))
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	notifs, err := h.db.GetNotifications()
	if err != nil {
		respond(w, 500, map[string]string{"error": "failed to retrieve notifications"})
		return
	}
	if notifs == nil {
		notifs = []models.Notification{}
	}
	respond(w, 200, notifs)
}

func (h *Handler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.ManageNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	var err error
	if body.All {
		err = h.db.MarkAllNotificationsRead()
	} else {
		err = h.db.MarkNotificationRead(body.ID)
	}
	if err != nil {
		respond(w, 500, map[string]string{"error": "failed to update notifications"})
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (h *Handler) DeleteNotifications(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body models.ManageNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	var err error
	if body.All {
		err = h.db.ClearAllNotifications()
	} else {
		err = h.db.DeleteNotification(body.ID)
	}
	if err != nil {
		respond(w, 500, map[string]string{"error": "failed to delete notifications"})
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
