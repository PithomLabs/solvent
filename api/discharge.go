package api

import (
	"encoding/json"
	"net/http"

	"github.com/PithomLabs/solvent/internal/view"
)

// handleDischarge handles POST /v1/discharge.
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

	// Cross-scenario guard: belief must belong to the claimed scenario.
	if _, err := view.GetSnapshot(r.Context(), s.db, req.ScenarioID, view.SnapshotOpts{BeliefID: req.BeliefID}); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
		return
	}

	if err := s.ledger.Discharge(r.Context(), req.ScenarioID, req.BeliefID, req.ObligationKey,
		req.InstrumentRef, req.DischargedBy); err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DischargeResult{
		BeliefID:      req.BeliefID,
		ObligationKey: req.ObligationKey,
		DischargedBy:  req.DischargedBy,
	})
}
