// Package authority implements the AuthorityService: the ONE production path
// for consequential external execution.
//
// The service gathers context and constructs the AuthorityTuple, then delegates
// the authoritative allow/deny determination to kernel.Authorize. The service
// MUST NOT independently determine whether authority exists.
//
// Security invariants:
//   - No successful current kernel.Authorize → no executor invocation.
//   - The executor is resolved internally from the registry, never caller-supplied.
//   - Intent creation ≠ execution. Earlier authorization results are never reused.
//   - The kernel is the final authority oracle. No second authority engine.
package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
)

// AuthorizationDecision is the outcome of PrepareForAction.
type AuthorizationDecision struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason,omitempty"`
	BeliefID     string `json:"belief_id"`
	BeliefStatus string `json:"belief_status"`
	TargetID     string `json:"target_id"`
	Action       string `json:"action"`
	CheckedAt    time.Time `json:"checked_at"`
}

// ExecutionResult records the outcome of an action execution.
type ExecutionResult struct {
	TokenID    string    `json:"token_id"`
	Allowed    bool      `json:"allowed"`
	Success    bool      `json:"success"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	ExecutedAt time.Time `json:"executed_at"`
}

// Service manages the authority boundary. It is the ONE production path for
// consequential external execution.
type Service struct {
	db       *sql.DB
	kern     *kernel.Store
	policy   *policy.Service
	audit    *audit.Service
	execReg  *executor.Registry
}

// New creates a new authority Service.
func New(db *sql.DB, pol *policy.Service, aud *audit.Service, reg *executor.Registry) *Service {
	return &Service{
		db:      db,
		kern:    kernel.New(db),
		policy:  pol,
		audit:   aud,
		execReg: reg,
	}
}

// PrepareForAction gathers current context and delegates to kernel.Authorize.
//
// The service retrieves data required to construct the AuthorityTuple, but
// MUST NOT independently evaluate whether authority is valid. The authoritative
// allow/deny determination comes from kernel.Authorize.
//
// This function re-reads CURRENT state at call time. It does not use cached
// authority, workflow token contents, browser state, or earlier authorization
// results.
func (s *Service) PrepareForAction(
	ctx context.Context,
	scenarioID, beliefID, action, targetID, actorID string,
	consequenceType string,
	consequenceParameters []byte,
) (*AuthorizationDecision, error) {
	decision := &AuthorizationDecision{
		BeliefID:  beliefID,
		TargetID:  targetID,
		Action:    action,
		CheckedAt: time.Now(),
	}

	// 1. Re-read current belief state.
	beliefStatus, err := s.getBeliefStatus(ctx, scenarioID, beliefID)
	if err != nil {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("belief read failed: %v", err)
		return decision, nil
	}
	decision.BeliefStatus = beliefStatus

	// 2. Construct the AuthorityTuple from context.
	//    The service gathers the tuple; kernel.Authorize decides.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           actorID,
		ResourceType:          "scenario",
		ResourceID:            scenarioID,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            action,
		ConsequenceType:       consequenceType,
		ConsequenceParameters: consequenceParameters,
	}

	// 3. Call kernel.Authorize — the final authority oracle.
	//    The service does NOT independently check activation, revocation,
	//    snapshot, or exact-match logic. That is the kernel's job.
	result, err := s.kern.Authorize(ctx, targetID, tuple)
	if err != nil {
		decision.Allowed = false
		decision.Reason = fmt.Sprintf("kernel authorize error: %v", err)
		return decision, nil
	}

	decision.Allowed = result.Allowed
	decision.Reason = result.Reason

	// 4. Log the authorization check.
	logType := audit.ActivityAuthorizationGranted
	if !result.Allowed {
		logType = audit.ActivityAuthorizationDenied
	}
	s.audit.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       logType,
		ActorID:    actorID,
		SubjectID:  beliefID,
		Details: map[string]interface{}{
			"target_id": targetID,
			"action":    action,
			"allowed":   result.Allowed,
			"reason":    result.Reason,
		},
	})

	return decision, nil
}

// ExecuteAction is the ONE production execution path.
//
// The caller supplies action parameters but NEVER the execution implementation.
// The executor is resolved internally from the trusted registry.
//
// Security invariants enforced:
//   - PrepareForAction re-reads current state (no cached authority).
//   - kernel.Authorize is called immediately before execution.
//   - The executor is from the internal registry, not caller-supplied.
//   - Authorization and execution outcomes are logged separately.
func (s *Service) ExecuteAction(
	ctx context.Context,
	scenarioID, beliefID, action, targetID, actorID string,
	params map[string]interface{},
	consequenceType string,
	consequenceParameters []byte,
) (*ExecutionResult, error) {
	result := &ExecutionResult{
		ExecutedAt: time.Now(),
	}

	// 1. PrepareForAction — re-reads current state, calls kernel.Authorize.
	decision, err := s.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, consequenceType, consequenceParameters)
	if err != nil {
		result.Allowed = false
		result.Error = err.Error()
		return result, nil
	}

	result.Allowed = decision.Allowed
	result.Reason = decision.Reason

	// 2. If not allowed: refuse, log refusal, return.
	if !decision.Allowed {
		s.audit.Log(ctx, &audit.ActivityEntry{
			ScenarioID: scenarioID,
			Type:       audit.ActivityExecutorDenied,
			ActorID:    actorID,
			SubjectID:  beliefID,
			Details: map[string]interface{}{
				"target_id": targetID,
				"action":    action,
				"reason":    decision.Reason,
			},
		})
		return result, nil
	}

	// 3. Resolve executor from internal registry (NOT caller-supplied).
	toolName, _ := params["tool_name"].(string)
	fn, ok := s.execReg.Get(toolName)
	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("executor not registered for tool: %s", toolName)
		return result, nil
	}

	// 4. Log authorization granted before execution.
	s.audit.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       audit.ActivityAdapterInvoked,
		ActorID:    actorID,
		SubjectID:  beliefID,
		Details: map[string]interface{}{
			"target_id": targetID,
			"action":    action,
			"tool":      toolName,
		},
	})

	// 5. Execute.
	output, execErr := fn(ctx, params)

	if execErr != nil {
		result.Success = false
		result.Error = execErr.Error()
		s.audit.Log(ctx, &audit.ActivityEntry{
			ScenarioID: scenarioID,
			Type:       audit.ActivityExecutorFailed,
			ActorID:    actorID,
			SubjectID:  beliefID,
			Details: map[string]interface{}{
				"target_id": targetID,
				"action":    action,
				"error":     execErr.Error(),
			},
		})
	} else {
		result.Success = true
		result.Output = output
		s.audit.Log(ctx, &audit.ActivityEntry{
			ScenarioID: scenarioID,
			Type:       audit.ActivityExecutorCompleted,
			ActorID:    actorID,
			SubjectID:  beliefID,
			Details: map[string]interface{}{
				"target_id": targetID,
				"action":    action,
			},
		})
	}

	return result, nil
}

// getBeliefStatus reads the current belief status from the database.
// This is context gathering, NOT authority determination.
func (s *Service) getBeliefStatus(ctx context.Context, scenarioID, beliefID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT status FROM belief
		WHERE scenario_id = $1::UUID AND id = $2::UUID`,
		scenarioID, beliefID).Scan(&status)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("belief %s not found in scenario %s", beliefID, scenarioID)
	}
	if err != nil {
		return "", fmt.Errorf("query belief: %w", err)
	}
	return status, nil
}

// GetToken retrieves a workflow token by ID (exposed for external callers).
func (s *Service) GetToken(ctx context.Context, tokenID string) (map[string]interface{}, error) {
	var payload []byte
	var state string
	err := s.db.QueryRowContext(ctx, `
		SELECT state, payload FROM workflow_token WHERE id = $1::UUID`, tokenID).Scan(&state, &payload)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	var p map[string]interface{}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	p["state"] = state
	return p, nil
}
