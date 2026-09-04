// Package audit implements the AuditService: activity ledger, SQLSTATE evidence
// surfacing, and invariant verification.
//
// The audit layer is the evidence trail. Every mutation produces an entry;
// every invariant check produces a receipt. The audit log is append-only.
//
// The audit distinguishes:
//   - authorization decisions (granted/denied)
//   - adapter invocations
//   - executor outcomes (completed/failed/denied)
//
// This preserves:
//   authorization succeeded + execution failed
// as distinct from:
//   authorization denied + execution never occurred
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ActivityType classifies an audit entry.
type ActivityType string

const (
	ActivityBeliefEntered     ActivityType = "belief_entered"
	ActivityBeliefPromoted    ActivityType = "belief_promoted"
	ActivityBeliefRetracted   ActivityType = "belief_retracted"
	ActivityEvidenceAdded     ActivityType = "evidence_added"
	ActivityDebtRetired       ActivityType = "debt_retired"
	ActivityIntentCreated     ActivityType = "intent_created"
	ActivityIntentCancelled   ActivityType = "intent_cancelled"
	ActivityToolAuthorized    ActivityType = "tool_authorized"
	ActivityToolDenied        ActivityType = "tool_denied"
	ActivityWorkflowCreated   ActivityType = "workflow_created"
	ActivityWorkflowCompleted ActivityType = "workflow_completed"
	ActivityWorkflowFailed    ActivityType = "workflow_failed"
	ActivityInvariantChecked  ActivityType = "invariant_checked"

	// Authorization lifecycle.
	ActivityAuthorizationChecked  ActivityType = "authorization_checked"
	ActivityAuthorizationGranted  ActivityType = "authorization_granted"
	ActivityAuthorizationDenied   ActivityType = "authorization_denied"

	// Execution lifecycle.
	ActivityAdapterInvoked   ActivityType = "adapter_invoked"
	ActivityProviderResponded ActivityType = "provider_responded"
	ActivityExecutorCompleted ActivityType = "executor_completed"
	ActivityExecutorFailed    ActivityType = "executor_failed"
	ActivityExecutorDenied    ActivityType = "executor_denied"
)

// ActivityEntry is one row in the activity ledger.
type ActivityEntry struct {
	ID          string                 `json:"id"`
	ScenarioID  string                 `json:"scenario_id"`
	Type        ActivityType           `json:"type"`
	ActorID     string                 `json:"actor_id,omitempty"`
	SubjectID   string                 `json:"subject_id,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	SQLState    string                 `json:"sqlstate,omitempty"`
	Constraint  string                 `json:"constraint,omitempty"`
	Refusal     bool                   `json:"refusal"`
	CreatedAt   time.Time              `json:"created_at"`
}

// InvariantReceipt records the result of an invariant check.
type InvariantReceipt struct {
	Name      string    `json:"name"`
	Passed    bool      `json:"passed"`
	Count     int       `json:"count"`
	CheckedAt time.Time `json:"checked_at"`
}

// Service manages the activity ledger.
type Service struct {
	db *sql.DB
}

// New creates a new audit Service.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// Log records an activity entry.
func (s *Service) Log(ctx context.Context, entry *ActivityEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	detailsJSON, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshal details: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO audit_activity (id, scenario_id, type, actor_id, subject_id, details, sqlstate, constraint_name, refusal, created_at)
		VALUES ($1::UUID, $2::UUID, $3, $4, $5, $6::JSONB, $7, $8, $9, $10)`,
		entry.ID, entry.ScenarioID, entry.Type, entry.ActorID, entry.SubjectID,
		string(detailsJSON), entry.SQLState, entry.Constraint, entry.Refusal, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}

	return nil
}

// LogRefusal logs a database refusal with its SQLSTATE and constraint name.
func (s *Service) LogRefusal(ctx context.Context, scenarioID string, activityType ActivityType, subjectID, sqlstate, constraintName string, details map[string]interface{}) error {
	return s.Log(ctx, &ActivityEntry{
		ScenarioID: scenarioID,
		Type:       activityType,
		SubjectID:  subjectID,
		Details:    details,
		SQLState:   sqlstate,
		Constraint: constraintName,
		Refusal:    true,
	})
}

// GetActivities returns audit entries for a scenario, optionally filtered by type.
func (s *Service) GetActivities(ctx context.Context, scenarioID string, activityType *ActivityType, limit int) ([]*ActivityEntry, error) {
	var rows *sql.Rows
	var err error

	if activityType != nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, scenario_id, type, actor_id, subject_id, details, sqlstate, constraint_name, refusal, created_at
			FROM audit_activity WHERE scenario_id = $1::UUID AND type = $2
			ORDER BY created_at DESC LIMIT $3`,
			scenarioID, *activityType, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, scenario_id, type, actor_id, subject_id, details, sqlstate, constraint_name, refusal, created_at
			FROM audit_activity WHERE scenario_id = $1::UUID
			ORDER BY created_at DESC LIMIT $2`,
			scenarioID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("get activities: %w", err)
	}
	defer rows.Close()

	var entries []*ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		var detailsRaw []byte
		var actorID, subjectID sql.NullString
		var sqlState, constraintName sql.NullString

		if err := rows.Scan(&e.ID, &e.ScenarioID, &e.Type, &actorID, &subjectID,
			&detailsRaw, &sqlState, &constraintName, &e.Refusal, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}

		if actorID.Valid {
			e.ActorID = actorID.String
		}
		if subjectID.Valid {
			e.SubjectID = subjectID.String
		}
		if sqlState.Valid {
			e.SQLState = sqlState.String
		}
		if constraintName.Valid {
			e.Constraint = constraintName.String
		}
		if detailsRaw != nil {
			if err := json.Unmarshal(detailsRaw, &e.Details); err != nil {
				return nil, fmt.Errorf("unmarshal details: %w", err)
			}
		}

		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// VerifyInvariant checks an invariant and records a receipt.
func (s *Service) VerifyInvariant(ctx context.Context, name string, checkFn func(ctx context.Context) (int, error)) (*InvariantReceipt, error) {
	count, err := checkFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("check invariant %s: %w", name, err)
	}

	receipt := &InvariantReceipt{
		Name:      name,
		Passed:    count == 0,
		Count:     count,
		CheckedAt: time.Now(),
	}

	return receipt, nil
}
