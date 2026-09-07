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
	"errors"
	"fmt"
	"time"

	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
)

// AuthorizationDecision is the outcome of PrepareForAction.
type AuthorizationDecision struct {
	Allowed               bool   `json:"allowed"`
	Reason                string `json:"reason,omitempty"`
	BeliefID              string `json:"belief_id"`
	BeliefStatus          string `json:"belief_status"`
	TargetID              string `json:"target_id"`
	Action                string `json:"action"`
	CheckedAt             time.Time `json:"checked_at"`
	ConsequenceParameters []byte   `json:"-"` // snapshot's approved params from kernel
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

// IntentOutcome classifies the operator's reconciliation decision.
type IntentOutcome int

const (
	// IntentOutcomeCompleted means the operator verified the provider accepted.
	// Transition: executing → executed (via CompleteIntent).
	IntentOutcomeCompleted IntentOutcome = iota

	// IntentOutcomeFailed means the operator verified the provider rejected.
	// Transition: executing → live (via RollbackClaim). Retry is safe.
	IntentOutcomeFailed

	// IntentOutcomeCancelled means the operator decides not to retry.
	// Transition: executing → cancelled (via CancelIntent).
	IntentOutcomeCancelled
)

// auditLogger is the minimal audit interface used by the authority service.
// *audit.Service satisfies this interface.
type auditLogger interface {
	Log(ctx context.Context, entry *audit.ActivityEntry) error
}

// providerClassifier is satisfied by adapter errors that carry a classified
// provider outcome. The adapter defines the outcome constants; the service
// maps them to kernel transitions by numeric code. This avoids importing the
// adapter package, which would create an import cycle in tests.
type providerClassifier interface {
	error
	ProviderOutcomeCode() int
}

// Provider outcome codes — must match adapter/github.ProviderOutcome values.
const (
	outcomeAccepted  = 0 // ProviderAccepted
	outcomeRejected  = 1 // ProviderRejected
	outcomeAmbiguous = 2 // ProviderAmbiguous
)

// Service manages the authority boundary. It is the ONE production path for
// consequential external execution.
type Service struct {
	db      *sql.DB
	kern    *kernel.Store
	policy  *policy.Service
	audit   auditLogger
	execReg *executor.Registry
}

// New creates a new authority Service.
func New(db *sql.DB, pol *policy.Service, aud auditLogger, reg *executor.Registry) *Service {
	return &Service{
		db:      db,
		kern:    kernel.New(db),
		policy:  pol,
		audit:   aud,
		execReg: reg,
	}
}

// actionExecutorMap maps action names to registered executor names.
// Executor selection is constrained by the authorized action, not by
// caller-supplied params.
var actionExecutorMap = map[string]string{
	"deploy": "github_trigger_workflow",
}

// resolveExecutor returns the registered executor name for the given action.
func resolveExecutor(action string) (string, bool) {
	name, ok := actionExecutorMap[action]
	return name, ok
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
	decision.ConsequenceParameters = result.ConsequenceParameters

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
// Step ordering (Option A — accept Authorize→Claim race):
//  1. PrepareForAction (kernel.Authorize)       — re-reads current state
//  2. Refuse if not allowed                     — unchanged
//  3. Resolve executor                          — unchanged
//  4. Reconstruct execution params from snapshot — BEFORE claim (can fail locally)
//  5. Validate execution params                  — BEFORE claim
//  6. ClaimIntent (atomic CAS live→executing)    — sole authority gate
//  7. Log adapter_invoked                        — unchanged
//  8. Execute provider                           — unchanged
//  9. Map provider outcome to kernel transitions  — NEW
//
// Security invariants enforced:
//   - PrepareForAction re-reads current state (no cached authority).
//   - kernel.Authorize is called immediately before execution.
//   - The executor is from the internal registry, not caller-supplied.
//   - Authorization and execution outcomes are logged separately.
//   - ClaimIntent is the sole authoritative ownership gate (CI-4).
//   - Provider outcome classification is adapter-specific (CI-9).
func (s *Service) ExecuteAction(
	ctx context.Context,
	scenarioID, beliefID, action, targetID, actorID string,
	intentID string,
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
	executorName, ok := resolveExecutor(action)
	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("no executor registered for action: %s", action)
		return result, nil
	}
	fn, ok := s.execReg.Get(executorName)
	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("executor not registered: %s", executorName)
		return result, nil
	}

	// 4. Reconstruct execution params from snapshot (already in AuthorizeResult).
	//    This happens BEFORE ClaimIntent — can fail locally without holding the claim.
	var snapParams map[string]interface{}
	if err := json.Unmarshal(decision.ConsequenceParameters, &snapParams); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("unmarshal snapshot params: %v", err)
		return result, nil
	}

	execParams := map[string]interface{}{
		"repo":     snapParams["repo"],
		"workflow": snapParams["workflow"],
		"ref":      snapParams["ref"],
	}

	// 5. Validate execution params (can fail locally).
	if execParams["repo"] == nil || execParams["workflow"] == nil || execParams["ref"] == nil {
		result.Success = false
		result.Error = "missing required execution params in snapshot"
		return result, nil
	}

	// 6. ClaimIntent — atomic CAS live→executing. Sole authority gate (CI-4).
	if intentID != "" {
		if err := s.kern.ClaimIntent(ctx, scenarioID, intentID); err != nil {
			result.Allowed = false
			result.Error = fmt.Sprintf("claim intent: %v", err)
			return result, nil
		}
	}

	// 7. Log adapter_invoked.
	s.audit.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       audit.ActivityAdapterInvoked,
		ActorID:    actorID,
		SubjectID:  beliefID,
		Details: map[string]interface{}{
			"target_id": targetID,
			"action":    action,
			"tool":      executorName,
		},
	})

	// 8. Execute.
	output, execErr := fn(ctx, execParams)

	if execErr != nil {
		// 9. Map provider outcome to kernel transitions (CI-8, CI-9).
		var pc providerClassifier
		if errors.As(execErr, &pc) {
			switch pc.ProviderOutcomeCode() {
			case outcomeAccepted:
				// Provider accepted despite error response. Treated as success.
				result.Success = true
				result.Output = output
				// Fall through to CompleteIntent below.

			case outcomeRejected:
				// Definitive rejection. Rollback claim, allow retry.
				if intentID != "" {
					_ = s.kern.RollbackClaim(ctx, scenarioID, intentID)
				}
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
				return result, nil

			case outcomeAmbiguous:
				// Unknown outcome. Leave intent as executing. No retry (CI-5).
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
				return result, nil
			}
		} else {
			// Non-provider error (e.g., executor registration issue).
			// Provider was never invoked. Treat as definitive rejection.
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
			return result, nil
		}
	} else {
		// Provider accepted (no error). Set success.
		result.Success = true
		result.Output = output
	}

	// Provider accepted. CompleteIntent (executing→executed).
	if intentID != "" {
		if err := s.kern.CompleteIntent(ctx, scenarioID, intentID); err != nil {
			// Persistence failure. Provider DID accept. Result remains truthful.
			s.audit.Log(ctx, &audit.ActivityEntry{
				ScenarioID: scenarioID,
				Type:       audit.ActivityIntentCompletionFailed,
				ActorID:    actorID,
				SubjectID:  beliefID,
				Details: map[string]interface{}{
					"target_id": targetID,
					"action":    action,
					"intent_id": intentID,
					"error":     err.Error(),
					"note":      "provider accepted but intent state not persisted",
				},
			})
			return result, nil
		}
	}

	// Persistence succeeded.
	s.audit.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       audit.ActivityExecutorCompleted,
		ActorID:    actorID,
		SubjectID:  beliefID,
		Details: map[string]interface{}{
			"target_id": targetID,
			"action":    action,
			"intent_id": intentID,
		},
	})
	return result, nil
}

// ReconcileIntent resolves an ambiguous 'executing' intent.
// This is a privileged operation requiring authenticated operator authority.
//
// Precondition: intent must currently be in 'executing' state.
// Returns ErrNotExecuting if the intent is not in 'executing' state.
//
// The caller MUST have verified the external provider state before calling.
// This method does NOT verify provider state — it trusts the caller's
// external verification, but enforces the source-state predicate atomically (CI-10).
//
// Reconciliation is an operator execution-control operation, not a public API.
func (s *Service) ReconcileIntent(ctx context.Context, scenarioID, intentID string, outcome IntentOutcome, operatorID string) error {
	// Log reconciliation before dispatching to kernel.
	s.audit.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       audit.ActivityReconciliationCompleted,
		ActorID:    operatorID,
		SubjectID:  intentID,
		Details: map[string]interface{}{
			"intent_id": intentID,
			"outcome":   outcome.String(),
		},
	})

	switch outcome {
	case IntentOutcomeCompleted:
		return s.kern.CompleteIntent(ctx, scenarioID, intentID)
	case IntentOutcomeFailed:
		return s.kern.RollbackClaim(ctx, scenarioID, intentID)
	case IntentOutcomeCancelled:
		return s.kern.CancelIntent(ctx, scenarioID, intentID)
	default:
		return fmt.Errorf("unknown outcome: %d", outcome)
	}
}

// String returns the human-readable name for an IntentOutcome.
func (o IntentOutcome) String() string {
	switch o {
	case IntentOutcomeCompleted:
		return "completed"
	case IntentOutcomeFailed:
		return "failed"
	case IntentOutcomeCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
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
