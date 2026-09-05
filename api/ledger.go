package api

import (
	"encoding/json"
	"net/http"
)

// handleGetLedger handles GET /v1/ledger.
func (s *Server) handleGetLedger(w http.ResponseWriter, r *http.Request) {
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}

	resp, err := ReadLedgerSummary(r.Context(), s.db, scenarioID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
