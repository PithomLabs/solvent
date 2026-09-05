package api

import (
	"encoding/json"
	"net/http"

	"github.com/PithomLabs/solvent/kernel"
)

// handleVerifyAuthorization handles POST /v1/authorizations/verify.
func (s *Server) handleVerifyAuthorization(w http.ResponseWriter, r *http.Request) {
	var req VerifyAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	if err := validateUUID(req.TargetID, "target_id"); err != nil {
		writeValidationError(w, "target_id", err.Error(), "")
		return
	}

	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal",
			"No authenticated principal", nil)
		return
	}

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principal.PrincipalID,
		ResourceType:          req.ResourceType,
		ResourceID:            req.ResourceID,
		Scope:                 req.Scope,
		ActionNamespace:       req.ActionNamespace,
		ActionName:            req.ActionName,
		ConsequenceType:       req.ConsequenceType,
		ConsequenceParameters: req.ConsequenceParameters,
	}

	result, err := s.ledger.VerifyAuthority(r.Context(), req.TargetID, tuple)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResult{
		TargetID: req.TargetID,
		Allowed:  result.Allowed,
		Reason:   result.Reason,
	})
}

// handleAuthorizeAction handles POST /v1/authorizations/action.
//
// This is the most security-sensitive handler. It uses the kernel's atomic
// AuthorizeAndCreateIntent primitive to ensure authority evaluation and intent
// creation occur in one SERIALIZABLE transaction.
//
// The effective principal is derived from the authenticated API credential,
// not from the request body. A caller-supplied actor_id that conflicts with
// the authenticated principal is rejected with 403 actor_id_mismatch.
func (s *Server) handleAuthorizeAction(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	// Gate: action_source must be user_typed — reject tool output.
	if req.ActionSource != "user_typed" {
		writeError(w, http.StatusForbidden, "action_source_tool_output",
			"action strings may not originate in tool output: retrieval is not authority", nil)
		return
	}

	if err := validateUUID(req.ScenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}
	if err := validateUUID(req.BeliefID, "belief_id"); err != nil {
		writeValidationError(w, "belief_id", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.Action, "action"); err != nil {
		writeValidationError(w, "action", err.Error(), "")
		return
	}
	if err := validateUUID(req.TargetID, "target_id"); err != nil {
		writeValidationError(w, "target_id", err.Error(), "")
		return
	}

	// The effective principal is the authenticated identity, not the request body.
	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal",
			"No authenticated principal", nil)
		return
	}
	effectiveActor := principal.PrincipalID

	// Reject conflicting actor_id — fail loud, not silent.
	if req.ActorID != "" && req.ActorID != effectiveActor {
		writeError(w, http.StatusForbidden, "actor_id_mismatch",
			"actor_id does not match authenticated principal", nil)
		return
	}

	tuple := kernel.AuthorityTuple{
		PrincipalID:           effectiveActor,
		ResourceType:          "scenario",
		ResourceID:            req.ScenarioID,
		Scope:                 "belief:" + req.BeliefID,
		ActionNamespace:       "solvent",
		ActionName:            req.Action,
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte("{}"),
	}

	decision, err := s.ledger.AuthorizeAndCreateIntent(r.Context(),
		req.ScenarioID, req.BeliefID, req.Action, req.TargetID, effectiveActor, tuple)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	if !decision.Allowed {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AuthorizeActionResult{
			BeliefID: req.BeliefID,
			Action:   req.Action,
			Authority: AuthResult{
				TargetID: req.TargetID,
				Allowed:  false,
				Reason:   decision.Reason,
			},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthorizeActionResult{
		BeliefID:    req.BeliefID,
		IntentState: decision.IntentState,
		Action:      req.Action,
		Authority: AuthResult{
			TargetID: req.TargetID,
			Allowed:  true,
		},
	})
}
