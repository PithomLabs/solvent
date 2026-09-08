package api

import (
	"encoding/json"
	"net/http"

	"github.com/PithomLabs/solvent/internal/view"
)

// handleAddEvidence handles POST /v1/evidence.
func (s *Server) handleAddEvidence(w http.ResponseWriter, r *http.Request) {
	var req AddEvidenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	if err := validateNonEmpty(req.ScenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}
	if err := validateUUID(req.BeliefID, "belief_id"); err != nil {
		writeValidationError(w, "belief_id", err.Error(), "")
		return
	}
	if err := validateEnum(req.ProvenanceClass, "provenance_class", []string{
		"external_feed", "reproducible_artifact", "live_scan", "operator_asserted",
	}); err != nil {
		writeValidationError(w, "provenance_class", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.ContentSHA256, "content_sha256"); err != nil {
		writeValidationError(w, "content_sha256", err.Error(), "")
		return
	}

	// Cross-scenario guard: belief must belong to the claimed scenario.
	if _, err := view.GetSnapshot(r.Context(), s.db, req.ScenarioID, view.SnapshotOpts{BeliefID: req.BeliefID}); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "belief not found in scenario", nil)
		return
	}

	if err := s.ledger.AddEvidence(r.Context(), req.ScenarioID, req.BeliefID,
		req.ProvenanceClass, req.SourceURL, req.ContentSHA256); err != nil {
		writeKernelError(w, err, "")
		return
	}

	s.auditLog(r.Context(), req.ScenarioID, "evidence_added", AuthFromContext(r.Context()), req.BeliefID, nil)

	// v0: no evidence_id returned.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(EvidenceResponse{
		BeliefID:        req.BeliefID,
		ProvenanceClass: req.ProvenanceClass,
		SourceURL:       req.SourceURL,
		ContentSHA256:   req.ContentSHA256,
	})
}

// handleGetEvidence handles GET /v1/evidence/{id}.
func (s *Server) handleGetEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateUUID(id, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	e, err := ReadEvidence(r.Context(), s.db, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e)
}
