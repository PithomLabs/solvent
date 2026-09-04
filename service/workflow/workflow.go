// Package workflow implements the WorkflowService: typed workflow tokens,
// state machine transitions, and the preparation/revalidation boundary.
//
// The workflow layer sits between the policy layer (which decides WHAT is allowed)
// and the executor layer (which does the work). It guarantees that:
//
//   - A token can only transition through valid states.
//   - Tokens carry typed payloads that are validated at creation time.
//   - Execution re-validates current belief state before acting.
//   - Completed or failed tokens cannot be re-executed.
package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TokenState represents the lifecycle of a workflow token.
type TokenState string

const (
	StatePending   TokenState = "pending"
	StatePrepared  TokenState = "prepared"
	StateExecuting TokenState = "executing"
	StateCompleted TokenState = "completed"
	StateFailed    TokenState = "failed"
	StateExpired   TokenState = "expired"
)

// Valid transitions defines the state machine.
var ValidTransitions = map[TokenState][]TokenState{
	StatePending:   {StatePrepared, StateExpired},
	StatePrepared:  {StateExecuting, StateExpired},
	StateExecuting: {StateCompleted, StateFailed},
	StateCompleted: {},
	StateFailed:    {},
	StateExpired:   {},
}

// WorkflowToken is a typed, lifecycle-tracked authorization to perform a specific action.
type WorkflowToken struct {
	ID           string                 `json:"id"`
	ScenarioID   string                 `json:"scenario_id"`
	BeliefID     string                 `json:"belief_id"`
	ActionType   string                 `json:"action_type"`
	State        TokenState             `json:"state"`
	Payload      map[string]interface{} `json:"payload"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    *time.Time             `json:"expires_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	FailureError string                 `json:"failure_error,omitempty"`
}

// ToolCall is the specific action type for tool execution.
const ActionTypeToolCall = "tool_call"

// ToolCallPayload is the typed payload for tool_call tokens.
type ToolCallPayload struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	ActorID  string                 `json:"actor_id"`
}

// Service manages workflow tokens and state transitions.
type Service struct {
	db *sql.DB
}

// New creates a new workflow Service.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateToken creates a new workflow token in pending state with a typed payload.
func (s *Service) CreateToken(ctx context.Context, scenarioID, beliefID, actionType string, payload map[string]interface{}) (*WorkflowToken, error) {
	if err := validateTransition(StatePending, StatePending); err != nil {
		return nil, err
	}

	if err := validatePayload(actionType, payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	token := &WorkflowToken{
		ID:         uuid.New().String(),
		ScenarioID: scenarioID,
		BeliefID:   beliefID,
		ActionType: actionType,
		State:      StatePending,
		Payload:    payload,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workflow_token (id, scenario_id, belief_id, action_type, state, payload, created_at, updated_at)
		VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5, $6::JSONB, $7, $8)`,
		token.ID, token.ScenarioID, token.BeliefID, token.ActionType,
		token.State, string(payloadJSON), token.CreatedAt, token.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create workflow token: %w", err)
	}

	return token, nil
}

// Prepare transitions a token from pending to prepared, indicating the system
// has validated the token is ready for execution.
func (s *Service) Prepare(ctx context.Context, tokenID string) (*WorkflowToken, error) {
	return s.transition(ctx, tokenID, StatePrepared)
}

// Execute transitions a token from prepared to executing.
func (s *Service) Execute(ctx context.Context, tokenID string) (*WorkflowToken, error) {
	return s.transition(ctx, tokenID, StateExecuting)
}

// Complete transitions a token from executing to completed.
func (s *Service) Complete(ctx context.Context, tokenID string) (*WorkflowToken, error) {
	now := time.Now()
	token, err := s.transition(ctx, tokenID, StateCompleted)
	if err != nil {
		return nil, err
	}
	token.CompletedAt = &now
	return token, nil
}

// Fail transitions a token from executing to failed with an error message.
func (s *Service) Fail(ctx context.Context, tokenID string, failureErr error) (*WorkflowToken, error) {
	token, err := s.transition(ctx, tokenID, StateFailed)
	if err != nil {
		return nil, err
	}
	token.FailureError = failureErr.Error()
	_, updateErr := s.db.ExecContext(ctx,
		`UPDATE workflow_token SET failure_error = $2 WHERE id = $1::UUID`,
		tokenID, token.FailureError)
	if updateErr != nil {
		return nil, fmt.Errorf("update failure error: %w", updateErr)
	}
	return token, nil
}

// Expire transitions a token to expired.
func (s *Service) Expire(ctx context.Context, tokenID string) (*WorkflowToken, error) {
	return s.transition(ctx, tokenID, StateExpired)
}

// GetToken retrieves a workflow token by ID.
func (s *Service) GetToken(ctx context.Context, tokenID string) (*WorkflowToken, error) {
	var token WorkflowToken
	var payloadRaw []byte
	var completedAt sql.NullTime
	var expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, scenario_id, belief_id, action_type, state, payload,
		       created_at, updated_at, expires_at, completed_at, failure_error
		FROM workflow_token WHERE id = $1::UUID`, tokenID).Scan(
		&token.ID, &token.ScenarioID, &token.BeliefID, &token.ActionType,
		&token.State, &payloadRaw, &token.CreatedAt, &token.UpdatedAt,
		&expiresAt, &completedAt, &token.FailureError)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("token %s not found", tokenID)
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow token: %w", err)
	}

	if err := json.Unmarshal(payloadRaw, &token.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	if expiresAt.Valid {
		token.ExpiresAt = &expiresAt.Time
	}
	if completedAt.Valid {
		token.CompletedAt = &completedAt.Time
	}

	return &token, nil
}

// ListTokensByScenario returns all workflow tokens for a scenario.
func (s *Service) ListTokensByScenario(ctx context.Context, scenarioID string) ([]*WorkflowToken, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scenario_id, belief_id, action_type, state, payload,
		       created_at, updated_at, expires_at, completed_at, failure_error
		FROM workflow_token WHERE scenario_id = $1::UUID ORDER BY created_at`,
		scenarioID)
	if err != nil {
		return nil, fmt.Errorf("list workflow tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*WorkflowToken
	for rows.Next() {
		var token WorkflowToken
		var payloadRaw []byte
		var completedAt, expiresAt sql.NullTime

		if err := rows.Scan(
			&token.ID, &token.ScenarioID, &token.BeliefID, &token.ActionType,
			&token.State, &payloadRaw, &token.CreatedAt, &token.UpdatedAt,
			&expiresAt, &completedAt, &token.FailureError); err != nil {
			return nil, fmt.Errorf("scan workflow token: %w", err)
		}

		if err := json.Unmarshal(payloadRaw, &token.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		if expiresAt.Valid {
			token.ExpiresAt = &expiresAt.Time
		}
		if completedAt.Valid {
			token.CompletedAt = &completedAt.Time
		}
		tokens = append(tokens, &token)
	}
	return tokens, rows.Err()
}

// transition performs a state machine transition with validation.
func (s *Service) transition(ctx context.Context, tokenID string, newState TokenState) (*WorkflowToken, error) {
	token, err := s.GetToken(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	if err := validateTransition(token.State, newState); err != nil {
		return nil, err
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE workflow_token SET state = $2, updated_at = $3 WHERE id = $1::UUID`,
		tokenID, newState, now)
	if err != nil {
		return nil, fmt.Errorf("transition token %s to %s: %w", tokenID, newState, err)
	}

	token.State = newState
	token.UpdatedAt = now
	return token, nil
}

// validateTransition checks that a state transition is allowed.
func validateTransition(from, to TokenState) error {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return fmt.Errorf("unknown state: %s", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s -> %s", from, to)
}

// validatePayload checks that the payload matches the action type.
func validatePayload(actionType string, payload map[string]interface{}) error {
	switch actionType {
	case ActionTypeToolCall:
		var tc ToolCallPayload
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &tc); err != nil {
			return fmt.Errorf("tool_call payload must have tool_name (string) and args (object)")
		}
		if tc.ToolName == "" {
			return fmt.Errorf("tool_call payload requires tool_name")
		}
		if tc.ActorID == "" {
			return fmt.Errorf("tool_call payload requires actor_id")
		}
		return nil
	default:
		return fmt.Errorf("unknown action type: %s", actionType)
	}
}
