package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PithomLabs/solvent/internal/view"
	"github.com/PithomLabs/solvent/kernel"
)

// handleEnterBelief handles POST /v1/beliefs.
func (s *Server) handleEnterBelief(w http.ResponseWriter, r *http.Request) {
	var req EnterBeliefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	if err := validateNonEmpty(req.ScenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.Claim, "claim"); err != nil {
		writeValidationError(w, "claim", err.Error(), "")
		return
	}
	if err := validateEnum(req.ClaimType, "claim_type", []string{"derived", "accommodated", "postulated"}); err != nil {
		writeValidationError(w, "claim_type", err.Error(), "")
		return
	}

	id, err := s.ledger.EnterBelief(r.Context(), req.ScenarioID, req.Claim, kernel.ClaimType(req.ClaimType), kernel.FullDebt)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	// Read back the created belief.
	b, err := ReadBelief(r.Context(), s.db, req.ScenarioID, id)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	s.auditLog(r.Context(), req.ScenarioID, "belief_entered", AuthFromContext(r.Context()), id, nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

// handleGetBelief handles GET /v1/beliefs/{id}.
func (s *Server) handleGetBelief(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(id, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	b, err := ReadBelief(r.Context(), s.db, scenarioID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// handleListBeliefs handles GET /v1/beliefs.
func (s *Server) handleListBeliefs(w http.ResponseWriter, r *http.Request) {
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}
	status := r.URL.Query().Get("status")
	claimType := r.URL.Query().Get("claim_type")
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	resp, err := ListBeliefs(r.Context(), s.db, scenarioID, status, claimType, limit, offset)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRetireDebt handles POST /v1/beliefs/{id}/debt/retire.
func (s *Server) handleRetireDebt(w http.ResponseWriter, r *http.Request) {
	beliefID := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(beliefID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	var req RetireDebtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	if err := validateNonEmpty(req.DebtItem, "debt_item"); err != nil {
		writeValidationError(w, "debt_item", err.Error(), "")
		return
	}

	// Cross-scenario guard: belief must belong to the claimed scenario.
	if _, err := view.GetSnapshot(r.Context(), s.db, scenarioID, view.SnapshotOpts{BeliefID: beliefID}); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
		return
	}

	if err := s.ledger.RetireDebt(r.Context(), scenarioID, beliefID, req.DebtItem); err != nil {
		writeKernelError(w, err, "")
		return
	}

	// Read updated belief.
	b, err := ReadBelief(r.Context(), s.db, scenarioID, beliefID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	s.auditLog(r.Context(), scenarioID, "debt_retired", AuthFromContext(r.Context()), beliefID, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// handlePromoteBelief handles POST /v1/beliefs/{id}/promote.
func (s *Server) handlePromoteBelief(w http.ResponseWriter, r *http.Request) {
	beliefID := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(beliefID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	// Cross-scenario guard: belief must belong to the claimed scenario.
	if _, err := view.GetSnapshot(r.Context(), s.db, scenarioID, view.SnapshotOpts{BeliefID: beliefID}); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
		return
	}

	err := s.ledger.Promote(r.Context(), scenarioID, beliefID)
	if err != nil {
		if isPromotionBlocked(err) {
			// Verdict: HTTP 200 with refusal body.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Verdict{
				Type:   "refusal",
				Reason: err.Error(),
				Gate:   "promoted_is_debt_free",
			})
			return
		}
		writeKernelError(w, err, "")
		return
	}

	b, err := ReadBelief(r.Context(), s.db, scenarioID, beliefID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	s.auditLog(r.Context(), scenarioID, "belief_promoted", AuthFromContext(r.Context()), beliefID, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// handleRetractBelief handles POST /v1/beliefs/{id}/retract.
func (s *Server) handleRetractBelief(w http.ResponseWriter, r *http.Request) {
	beliefID := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(beliefID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	_, err := s.ledger.RetractCascade(r.Context(), scenarioID, beliefID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	b, err := ReadBelief(r.Context(), s.db, scenarioID, beliefID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	s.auditLog(r.Context(), scenarioID, "belief_retracted", AuthFromContext(r.Context()), beliefID, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// handleExplainBelief handles GET /v1/beliefs/{id}/explain.
func (s *Server) handleExplainBelief(w http.ResponseWriter, r *http.Request) {
	beliefID := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(beliefID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	resp, err := ReadBeliefExplain(r.Context(), s.db, scenarioID, beliefID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleListEvidenceForBelief handles GET /v1/beliefs/{id}/evidence.
func (s *Server) handleListEvidenceForBelief(w http.ResponseWriter, r *http.Request) {
	beliefID := r.PathValue("id")
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(beliefID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	resp, err := ListEvidenceForBelief(r.Context(), s.db, scenarioID, beliefID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func isPromotionBlocked(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "promotion blocked") ||
		strings.Contains(err.Error(), "23514"))
}
