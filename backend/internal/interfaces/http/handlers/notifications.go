package handlers

import (
	"net/http"

	"github.com/sohidul/dns-server/internal/shared/models"
)

// GetNotifications returns all notifications, newest first.
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	notifs, err := h.db.GetNotifications()
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to retrieve notifications"})
		return
	}
	if notifs == nil {
		notifs = []models.Notification{}
	}
	respond(w, http.StatusOK, notifs)
}

// MarkNotificationsRead marks one notification (or all) as read.
func (h *Handler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var body models.ManageNotificationRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	var err error
	if body.All {
		err = h.db.MarkAllNotificationsRead()
	} else {
		err = h.db.MarkNotificationRead(body.ID)
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to update notifications"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// DeleteNotifications deletes one notification (or all).
func (h *Handler) DeleteNotifications(w http.ResponseWriter, r *http.Request) {
	var body models.ManageNotificationRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	var err error
	if body.All {
		err = h.db.ClearAllNotifications()
	} else {
		err = h.db.DeleteNotification(body.ID)
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete notifications"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
