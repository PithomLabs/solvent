package api

import (
	"encoding/json"
	"net/http"
)

// handleGetActivity handles GET /v1/activity.
func (s *Server) handleGetActivity(w http.ResponseWriter, r *http.Request) {
	scenarioID := r.URL.Query().Get("scenario_id")
	if err := validateUUID(scenarioID, "scenario_id"); err != nil {
		writeValidationError(w, "scenario_id", err.Error(), "")
		return
	}
	activityType := r.URL.Query().Get("type")
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	resp, err := ReadActivity(r.Context(), s.db, scenarioID, activityType, limit, offset)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
