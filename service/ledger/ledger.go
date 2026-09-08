// Package ledger implements the API-specific service layer. It sits between
// HTTP handlers and the kernel, providing:
//
//   - product/domain orchestration
//   - effective-actor resolution
//   - audit coordination
//   - transactional authority + intent creation via the kernel's atomic primitive
//
// The service does NOT implement Solvent authority semantics itself.
// All authority evaluation is delegated to the kernel.
package ledger

import (
	"context"
	"database/sql"
	"time"

	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
)

// AuthorizationDecision is the outcome of an authority + intent operation.
type AuthorizationDecision struct {
	Allowed     bool      `json:"allowed"`
	Reason      string    `json:"reason,omitempty"`
	IntentState string    `json:"intent_state,omitempty"`
	BeliefID    string    `json:"belief_id"`
	TargetID    string    `json:"target_id"`
	Action      string    `json:"action"`
	CheckedAt   time.Time `json:"checked_at"`
}

// Service manages API-specific orchestration. It is NOT an authority source.
// All authority evaluation is delegated to the kernel.
type Service struct {
	db    *sql.DB
	kern  *kernel.Store
	audit *audit.Service
}

// New creates a new ledger Service.
func New(db *sql.DB, aud *audit.Service) *Service {
	return &Service{
		db:    db,
		kern:  kernel.New(db),
		audit: aud,
	}
}

// --- Passthrough operations (kernel method is self-contained) ---

// EnterBelief delegates to kernel.EnterBelief.
func (s *Service) EnterBelief(ctx context.Context, scenarioID, claim string, ct kernel.ClaimType, initialDebt []string) (string, error) {
	return s.kern.EnterBelief(ctx, scenarioID, claim, ct, initialDebt)
}

// AddEvidence delegates to kernel.AddEvidence.
func (s *Service) AddEvidence(ctx context.Context, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error {
	return s.kern.AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)
}

// RetireDebt delegates to kernel.RetireDebt.
func (s *Service) RetireDebt(ctx context.Context, scenarioID, beliefID, item string) error {
	return s.kern.RetireDebt(ctx, scenarioID, beliefID, item)
}

// Promote delegates to kernel.Promote.
func (s *Service) Promote(ctx context.Context, scenarioID, beliefID string) error {
	return s.kern.Promote(ctx, scenarioID, beliefID)
}

// RetractCascade delegates to kernel.RetractCascade.
func (s *Service) RetractCascade(ctx context.Context, scenarioID, rootID string) (int, error) {
	return s.kern.RetractCascade(ctx, scenarioID, rootID)
}

// CreatePrincipal delegates to kernel.CreatePrincipal.
func (s *Service) CreatePrincipal(ctx context.Context, principalType, issuer string) (string, error) {
	return s.kern.CreatePrincipal(ctx, principalType, issuer)
}

// RevokePrincipal delegates to kernel.RevokePrincipal.
func (s *Service) RevokePrincipal(ctx context.Context, principalID string) error {
	return s.kern.RevokePrincipal(ctx, principalID)
}

// CreateTarget delegates to kernel.CreateTarget.
func (s *Service) CreateTarget(ctx context.Context, principalID, resourceType, resourceID, scope, actionNamespace, actionName, consequenceType string, consequenceParameters []byte, createdBy string) (string, error) {
	return s.kern.CreateTarget(ctx, principalID, resourceType, resourceID, scope, actionNamespace, actionName, consequenceType, consequenceParameters, createdBy)
}

// AttachJustification delegates to kernel.AttachJustification.
func (s *Service) AttachJustification(ctx context.Context, targetID, beliefID, beliefStatus, attachedBy string) error {
	return s.kern.AttachJustification(ctx, targetID, beliefID, beliefStatus, attachedBy)
}

// RequestAuthorization delegates to kernel.RequestAuthorization.
func (s *Service) RequestAuthorization(ctx context.Context, targetID, requestedBy string) error {
	return s.kern.RequestAuthorization(ctx, targetID, requestedBy)
}

// Approve delegates to kernel.Approve.
func (s *Service) Approve(ctx context.Context, targetID, approverPrincipalID string) error {
	return s.kern.Approve(ctx, targetID, approverPrincipalID)
}

// RevokeTarget delegates to kernel.RevokeTarget.
func (s *Service) RevokeTarget(ctx context.Context, targetID, revokedBy, reason string) error {
	return s.kern.RevokeTarget(ctx, targetID, revokedBy, reason)
}

// Discharge delegates to kernel.Discharge.
func (s *Service) Discharge(ctx context.Context, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy string) error {
	return s.kern.Discharge(ctx, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy)
}

// --- Read-only operations ---

// VerifyAuthority delegates to kernel.Authorize (read-only, own transaction).
func (s *Service) VerifyAuthority(ctx context.Context, targetID string, tuple kernel.AuthorityTuple) (kernel.AuthorizeResult, error) {
	return s.kern.Authorize(ctx, targetID, tuple)
}

// --- Transactional operation (atomic authority + intent) ---

// AuthorizeAndCreateIntent evaluates current authority and creates a live
// action intent inside ONE SERIALIZABLE transaction via the kernel's atomic
// primitive. The kernel is the sole authority oracle.
func (s *Service) AuthorizeAndCreateIntent(
	ctx context.Context,
	scenarioID, beliefID, action, targetID, actorID string,
	tuple kernel.AuthorityTuple,
) (*AuthorizationDecision, error) {
	decision := &AuthorizationDecision{
		BeliefID:  beliefID,
		TargetID:  targetID,
		Action:    action,
		CheckedAt: time.Now(),
	}

	result, err := s.kern.AuthorizeAndCreateIntent(ctx, targetID, tuple, scenarioID, beliefID, action)
	if err != nil {
		return nil, err
	}

	decision.Allowed = result.Allowed
	decision.Reason = result.Reason
	decision.IntentState = result.IntentState

	// Log the authorization decision.
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
