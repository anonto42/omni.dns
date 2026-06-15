// Package steering is the application service coordinating steering-rule
// persistence with user notifications.
package steering

import (
	"context"
	"fmt"

	steeringdomain "github.com/sohidul/dns-server/internal/domain/steering"
)

// RuleDTO is the application-layer representation of a steering rule.
type RuleDTO struct {
	ID             int64
	Name           string
	ConditionType  string
	ConditionValue string
	ActionType     string
	ActionTarget   string
	Priority       int
	Enabled        bool
}

// Service orchestrates steering-rule use cases.
type Service struct {
	repo     steeringdomain.Repository
	notifier steeringdomain.Notifier
}

// NewService builds a steering service.
func NewService(repo steeringdomain.Repository, notifier steeringdomain.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

// List returns all steering rules.
func (s *Service) List(ctx context.Context) ([]RuleDTO, error) {
	rules, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, RuleDTO{
			Name:           r.Name(),
			ConditionType:  r.ConditionType(),
			ConditionValue: r.ConditionValue(),
			ActionType:     r.ActionType(),
			ActionTarget:   r.ActionTarget(),
			Priority:       r.Priority(),
			Enabled:        r.Enabled(),
		})
	}
	return out, nil
}

// Add validates and persists a new steering rule, returning its id.
func (s *Service) Add(ctx context.Context, in RuleDTO) (int64, error) {
	rule, err := steeringdomain.New(in.Name, in.ConditionType, in.ConditionValue, in.ActionType, in.ActionTarget, in.Priority, in.Enabled)
	if err != nil {
		return 0, err
	}
	id, err := s.repo.Add(ctx, rule)
	if err != nil {
		return 0, err
	}
	_ = s.notifier.Notify(ctx, "success", "Steering Rule Added", fmt.Sprintf("Added rule %q successfully.", rule.Name()))
	return id, nil
}

// SetEnabled toggles a rule's enabled state.
func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	if err := s.repo.SetEnabled(ctx, id, enabled); err != nil {
		return err
	}
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	_ = s.notifier.Notify(ctx, "info", "Steering Rule "+title(status), fmt.Sprintf("Traffic steering rule ID %d has been %s.", id, status))
	return nil
}

// Delete removes a rule by id.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.notifier.Notify(ctx, "info", "Steering Rule Deleted", fmt.Sprintf("Traffic steering rule ID %d has been deleted.", id))
	return nil
}

func title(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
