package handlers

import (
	"net/http"
	"strings"

	"github.com/sohidul/dns-server/internal/interfaces/http/middleware"
	"github.com/sohidul/dns-server/internal/shared/models"
)

// Login validates credentials and returns a session token.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !h.db.VerifyUser(body.Email, body.Password) {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	token, err := h.db.CreateSession(body.Email)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}
	respond(w, http.StatusOK, map[string]string{"token": token})
}

// Logout deletes the caller's session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != "" {
		h.db.DeleteSession(token)
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// ChangePassword updates the admin password after verifying the current one.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var body models.ChangePasswordRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	email, _ := middleware.UserFromContext(r.Context())
	if !h.db.VerifyUser(email, body.CurrentPassword) {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}
	if len(body.NewPassword) < 8 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
		return
	}
	if err := h.db.ChangePassword(email, body.NewPassword); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to change password"})
		return
	}
	h.notify("info", "Password Changed", "Administrator account password updated successfully.")
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// GetProfile returns the authenticated user's email and display name.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	email, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	name, err := h.db.GetProfile(email)
	if err != nil {
		name = "Administrator"
	}
	respond(w, http.StatusOK, map[string]string{"email": email, "name": name})
}

// UpdateProfile changes the user's display name and email.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	oldEmail, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Name == "" || body.Email == "" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "name and email cannot be empty"})
		return
	}
	if !strings.Contains(body.Email, "@") {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid email address"})
		return
	}
	if err := h.db.UpdateProfile(oldEmail, body.Email, body.Name); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.notify("success", "Profile Updated", "Account details updated successfully.")
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
