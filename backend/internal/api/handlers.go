package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	blocklistapp "github.com/sohidul/dns-server/internal/application/blocklist"
	recordsapp "github.com/sohidul/dns-server/internal/application/records"
	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/dns"
	blocklistdomain "github.com/sohidul/dns-server/internal/domain/blocklist"
	recordsdomain "github.com/sohidul/dns-server/internal/domain/records"
	"github.com/sohidul/dns-server/internal/models"
)

const maxBodyBytes = 1 << 16

type Handler struct {
	db        *db.DB
	dns       *dns.Handler
	records   *recordsapp.Service
	blocklist *blocklistapp.Service
}

func NewHandler(database *db.DB, dnsHandler *dns.Handler) *Handler {
	notifier := db.NewNotificationRepository(database)
	return &Handler{
		db:        database,
		dns:       dnsHandler,
		records:   recordsapp.NewService(db.NewRecordsRepository(database), notifier),
		blocklist: blocklistapp.NewService(db.NewBlocklistRepository(database), notifier),
	}
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

func (h *Handler) notify(notifType, title, message string) {
	if err := h.db.AddNotification(notifType, title, message); err != nil {
		slog.Error("failed to add notification", "type", notifType, "title", title, "error", err)
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
	records, err := h.records.List(r.Context())
	if err != nil {
		slog.Error("list records failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to retrieve records"})
		return
	}
	out := make(map[string]string, len(records))
	for _, rec := range records {
		out[rec.Domain] = rec.IP
	}
	respond(w, 200, out)
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

	if err := h.records.Add(r.Context(), body.Domain, body.IP); err != nil {
		if errors.Is(err, recordsdomain.ErrInvalidDomain) {
			respond(w, 400, map[string]string{"error": "invalid domain name"})
			return
		}
		if errors.Is(err, recordsdomain.ErrInvalidIP) {
			respond(w, 400, map[string]string{"error": "invalid IP address"})
			return
		}
		slog.Error("add record failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to add record"})
		return
	}

	respond(w, 200, map[string]bool{"ok": true})
}

func respondRecordError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, recordsdomain.ErrInvalidDomain) {
		respond(w, 400, map[string]string{"error": "invalid domain name"})
		return true
	}
	if errors.Is(err, recordsdomain.ErrInvalidIP) {
		respond(w, 400, map[string]string{"error": "invalid IP address"})
		return true
	}
	return false
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

	if err := h.records.Delete(r.Context(), body.Domain); err != nil {
		if respondRecordError(w, err) {
			return
		}
		slog.Error("delete record failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to delete record"})
		return
	}

	respond(w, 200, map[string]bool{"ok": true})
}

// GetBlocklist godoc
// @Summary      Get blocklist
// @Description  Returns all domains in the blocklist
// @Produce      json
// @Success      200  {array}  models.BlockedDomain
// @Router       /api/blocklist [get]
func (h *Handler) GetBlocklist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.blocklist.List(r.Context())
	if err != nil {
		slog.Error("list blocklist failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to retrieve blocklist"})
		return
	}
	out := make([]models.BlockedDomain, 0, len(entries))
	for _, e := range entries {
		out = append(out, models.BlockedDomain{
			Domain:   e.Domain,
			AddedAt:  e.AddedAt,
			Wildcard: e.Wildcard,
		})
	}
	respond(w, 200, out)
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

	if err := h.blocklist.Add(r.Context(), body.Domain, body.Wildcard); err != nil {
		if errors.Is(err, blocklistdomain.ErrInvalidDomain) {
			respond(w, 400, map[string]string{"error": "invalid domain name"})
			return
		}
		slog.Error("add to blocklist failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to add to blocklist"})
		return
	}

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
	h.notify("success", "Settings Saved", "Upstream DNS and blocking behaviors updated successfully.")

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

	if err := h.blocklist.Remove(r.Context(), body.Domain); err != nil {
		if errors.Is(err, blocklistdomain.ErrInvalidDomain) {
			respond(w, 400, map[string]string{"error": "invalid domain name"})
			return
		}
		slog.Error("remove from blocklist failed", "error", err)
		respond(w, 500, map[string]string{"error": "failed to remove from blocklist"})
		return
	}

	respond(w, 200, map[string]bool{"ok": true})
}

// GetLogsFiltered godoc
// @Summary      Get query logs
// @Description  Returns recent DNS query logs, optionally filtered by ?limit=N&action=blocked&domain=example
// @Produce      json
// @Success      200  {array}  models.QueryLog
// @Router       /api/logs [get]
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
	h.notify("info", "Password Changed", "Administrator account password updated successfully.")
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
	h.notify("success", "Steering Rule Added", fmt.Sprintf("Added rule \"%s\" successfully.", body.Name))
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
	h.notify("info", fmt.Sprintf("Steering Rule %s", titleCase(status)), fmt.Sprintf("Traffic steering rule ID %d has been %s.", body.ID, status))
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
	h.notify("info", "Steering Rule Deleted", fmt.Sprintf("Traffic steering rule ID %d has been deleted.", body.ID))
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

	h.notify("success", "Profile Updated", fmt.Sprintf("Account details updated: Name='%s', Email='%s'.", body.Name, body.Email))
	respond(w, 200, map[string]bool{"ok": true})
}

func titleCase(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
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
