package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	steeringapp "github.com/sohidul/dns-server/internal/modules/steering/application"
	steeringdomain "github.com/sohidul/dns-server/internal/modules/steering/domain"
	"github.com/sohidul/dns-server/internal/shared/models"
)

// GetSteeringRules returns all steering rules.
func (h *Handler) GetSteeringRules(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, h.db.GetSteeringRules())
}

// AddSteeringRule creates a new steering rule.
func (h *Handler) AddSteeringRule(w http.ResponseWriter, r *http.Request) {
	var body models.AddSteeringRuleRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	id, err := h.steering.Add(r.Context(), steeringapp.RuleDTO{
		Name:           body.Name,
		ConditionType:  body.ConditionType,
		ConditionValue: body.ConditionValue,
		ActionType:     body.ActionType,
		ActionTarget:   body.ActionTarget,
		Priority:       body.Priority,
		Enabled:        body.Enabled,
	})
	if err != nil {
		if writeSteeringError(w, err) {
			return
		}
		slog.Error("add steering rule failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to add rule"})
		return
	}
	respond(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// UpdateSteeringRule toggles a steering rule's enabled flag.
func (h *Handler) UpdateSteeringRule(w http.ResponseWriter, r *http.Request) {
	var body models.UpdateSteeringRuleRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.steering.SetEnabled(r.Context(), body.ID, body.Enabled); err != nil {
		slog.Error("update steering rule failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to update rule"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

// DeleteSteeringRule removes a steering rule.
func (h *Handler) DeleteSteeringRule(w http.ResponseWriter, r *http.Request) {
	var body models.DeleteSteeringRuleRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.steering.Delete(r.Context(), body.ID); err != nil {
		slog.Error("delete steering rule failed", "error", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete rule"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeSteeringError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, steeringdomain.ErrEmptyName):
		respond(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
	case errors.Is(err, steeringdomain.ErrInvalidCondition):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid condition type"})
	case errors.Is(err, steeringdomain.ErrInvalidAction):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid action type"})
	case errors.Is(err, steeringdomain.ErrMissingTarget):
		respond(w, http.StatusBadRequest, map[string]string{"error": "action requires a target"})
	default:
		return false
	}
	return true
}
