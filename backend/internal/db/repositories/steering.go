package repositories

import (
	"context"

	"github.com/sohidul/dns-server/internal/db"
	steeringdomain "github.com/sohidul/dns-server/internal/domain/steering"
)

// Steering persists steering rules.
type Steering struct {
	db *db.DB
}

// NewSteering builds a steering repository over the given database.
func NewSteering(database *db.DB) *Steering {
	return &Steering{db: database}
}

// List returns all steering rules in priority order, reconstituting validated
// aggregates from persisted values.
func (r *Steering) List(ctx context.Context) ([]steeringdomain.Rule, error) {
	rows, err := r.db.Conn().QueryContext(ctx, "SELECT name, condition_type, condition_value, action_type, action_target, priority, enabled FROM steering_rules ORDER BY priority ASC, id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]steeringdomain.Rule, 0)
	for rows.Next() {
		var name, ct, cv, at, target string
		var priority, enabled int
		if err := rows.Scan(&name, &ct, &cv, &at, &target, &priority, &enabled); err != nil {
			return nil, err
		}
		rule, err := steeringdomain.New(name, ct, cv, at, target, priority, enabled != 0)
		if err != nil {
			continue // skip rows that no longer satisfy current validation
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Add inserts a rule and returns its new id.
func (r *Steering) Add(ctx context.Context, rule steeringdomain.Rule) (int64, error) {
	enabled := 0
	if rule.Enabled() {
		enabled = 1
	}
	res, err := r.db.Conn().ExecContext(
		ctx,
		"INSERT INTO steering_rules (name, condition_type, condition_value, action_type, action_target, priority, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)",
		rule.Name(), rule.ConditionType(), rule.ConditionValue(), rule.ActionType(), rule.ActionTarget(), rule.Priority(), enabled,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetEnabled toggles a rule's enabled flag.
func (r *Steering) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := r.db.Conn().ExecContext(ctx, "UPDATE steering_rules SET enabled = ? WHERE id = ?", e, id)
	return err
}

// Delete removes a rule by id.
func (r *Steering) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Conn().ExecContext(ctx, "DELETE FROM steering_rules WHERE id = ?", id)
	return err
}
