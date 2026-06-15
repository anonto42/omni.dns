// Package steering holds the business rules for DNS traffic steering: the Rule
// aggregate plus validation of conditions and actions.
package steering

import (
	"errors"
	"strings"
)

var (
	ErrEmptyName        = errors.New("steering rule name is required")
	ErrInvalidCondition = errors.New("invalid steering condition type")
	ErrInvalidAction    = errors.New("invalid steering action type")
	ErrMissingTarget    = errors.New("steering action requires a target")
)

// Recognized condition and action types.
var (
	conditionTypes = map[string]bool{
		"Domain": true, "Client IP": true, "Query Type": true, "Time Range": true,
	}
	actionTypes = map[string]bool{
		"Forward": true, "Block": true, "Redirect": true,
	}
)

// Rule is the aggregate root for a steering rule. A constructed Rule has passed
// validation of its condition and action.
type Rule struct {
	name           string
	conditionType  string
	conditionValue string
	actionType     string
	actionTarget   string
	priority       int
	enabled        bool
}

// New validates and builds a Rule from raw input.
func New(name, conditionType, conditionValue, actionType, actionTarget string, priority int, enabled bool) (Rule, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Rule{}, ErrEmptyName
	}
	if !conditionTypes[conditionType] {
		return Rule{}, ErrInvalidCondition
	}
	if !actionTypes[actionType] {
		return Rule{}, ErrInvalidAction
	}
	// Forward and Redirect both need a target; Block does not.
	if actionType != "Block" && strings.TrimSpace(actionTarget) == "" {
		return Rule{}, ErrMissingTarget
	}
	return Rule{
		name:           name,
		conditionType:  conditionType,
		conditionValue: conditionValue,
		actionType:     actionType,
		actionTarget:   actionTarget,
		priority:       priority,
		enabled:        enabled,
	}, nil
}

func (r Rule) Name() string           { return r.name }
func (r Rule) ConditionType() string  { return r.conditionType }
func (r Rule) ConditionValue() string { return r.conditionValue }
func (r Rule) ActionType() string     { return r.actionType }
func (r Rule) ActionTarget() string   { return r.actionTarget }
func (r Rule) Priority() int          { return r.priority }
func (r Rule) Enabled() bool          { return r.enabled }
