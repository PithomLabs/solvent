package api

import (
	"encoding/json"
	"net/http"

	"github.com/PithomLabs/solvent/internal/view"
)

// handleDischarge handles POST /v1/discharge.
//
// Access control: the authenticated principal must exist and not be revoked.
// The discharged_by field is validated against the authenticated principal —
// caller-supplied impersonation is rejected with 403 discharged_by_mismatch.
func (s *Server) handleDischarge(w http.ResponseWriter, r *http.Request) {
	var req DischargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
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
	if err := validateNonEmpty(req.ObligationKey, "obligation_key"); err != nil {
		writeValidationError(w, "obligation_key", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.InstrumentRef, "instrument_ref"); err != nil {
		writeValidationError(w, "instrument_ref", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.DischargedBy, "discharged_by"); err != nil {
		writeValidationError(w, "discharged_by", err.Error(), "")
		return
	}

	// Access control: verify authenticated principal is active.
	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal",
			"No authenticated principal", nil)
		return
	}
	if err := verifyPrincipalActive(r.Context(), s.db, principal.PrincipalID); err != nil {
		writeKernelError(w, err, "")
		return
	}

	// Reject impersonation: discharged_by must match the authenticated principal.
	if req.DischargedBy != principal.PrincipalID {
		writeError(w, http.StatusForbidden, "discharged_by_mismatch",
			"discharged_by does not match authenticated principal", nil)
		return
	}

	// Cross-scenario guard: belief must belong to the claimed scenario.
	if _, err := view.GetSnapshot(r.Context(), s.db, req.ScenarioID, view.SnapshotOpts{BeliefID: req.BeliefID}); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
		return
	}

	if err := s.ledger.Discharge(r.Context(), req.ScenarioID, req.BeliefID, req.ObligationKey,
		req.InstrumentRef, principal.PrincipalID); err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DischargeResult{
		BeliefID:      req.BeliefID,
		ObligationKey: req.ObligationKey,
		DischargedBy:  principal.PrincipalID,
	})
}
