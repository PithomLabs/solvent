package api

import (
	"encoding/json"
	"net/http"
)

// handleCreateTarget handles POST /v1/targets.
func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var req CreateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	if err := validateUUID(req.PrincipalID, "principal_id"); err != nil {
		writeValidationError(w, "principal_id", err.Error(), "")
		return
	}
	for _, f := range []struct{ val, name string }{
		{req.ResourceType, "resource_type"},
		{req.Scope, "scope"},
		{req.ActionNamespace, "action_namespace"},
		{req.ActionName, "action_name"},
		{req.ConsequenceType, "consequence_type"},
	} {
		if err := validateNonEmpty(f.val, f.name); err != nil {
			writeValidationError(w, f.name, err.Error(), "")
			return
		}
	}
	if len(req.ConsequenceParameters) > 0 {
		if err := validateJSON(req.ConsequenceParameters, "consequence_parameters"); err != nil {
			writeValidationError(w, "consequence_parameters", err.Error(), "")
			return
		}
	}

	id, err := s.ledger.CreateTarget(r.Context(), req.PrincipalID, req.ResourceType, req.ResourceID,
		req.Scope, req.ActionNamespace, req.ActionName, req.ConsequenceType,
		req.ConsequenceParameters, req.CreatedBy)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, id)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

// handleGetTarget handles GET /v1/targets/{id}.
func (s *Server) handleGetTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateUUID(id, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleListTargets handles GET /v1/targets.
func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	resp, err := ListTargets(r.Context(), s.db, limit, offset)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAttachJustification handles POST /v1/targets/{id}/justifications.
func (s *Server) handleAttachJustification(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if err := validateUUID(targetID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	var req AttachJustificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	if err := validateNonEmpty(req.InstrumentRef, "instrument_ref"); err != nil {
		writeValidationError(w, "instrument_ref", err.Error(), "")
		return
	}

	// Get belief_id from query param.
	beliefID := r.URL.Query().Get("belief_id")
	if err := validateUUID(beliefID, "belief_id"); err != nil {
		writeValidationError(w, "belief_id", err.Error(), "")
		return
	}

	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
		return
	}

	if err := s.ledger.AttachJustification(r.Context(), targetID, beliefID, "promoted",
		principal.PrincipalID); err != nil {
		writeKernelError(w, err, "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, targetID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleRequestAuthorization handles POST /v1/targets/{id}/request.
func (s *Server) handleRequestAuthorization(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if err := validateUUID(targetID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
		return
	}
	if err := s.ledger.RequestAuthorization(r.Context(), targetID, principal.PrincipalID); err != nil {
		writeKernelError(w, err, "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, targetID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleApproveTarget handles POST /v1/targets/{id}/approve.
func (s *Server) handleApproveTarget(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if err := validateUUID(targetID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	var req ApproveTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	if err := validateNonEmpty(req.ApprovalPin, "approval_pin"); err != nil {
		writeValidationError(w, "approval_pin", err.Error(), "")
		return
	}

	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
		return
	}
	if err := s.ledger.Approve(r.Context(), targetID, principal.PrincipalID); err != nil {
		writeKernelError(w, err, "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, targetID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleRevokeTarget handles POST /v1/targets/{id}/revoke.
func (s *Server) handleRevokeTarget(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if err := validateUUID(targetID, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	var req RevokeTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	principal := AuthFromContext(r.Context())
	if principal == nil {
		writeError(w, http.StatusUnauthorized, "missing_principal", "Authentication required", nil)
		return
	}
	if err := s.ledger.RevokeTarget(r.Context(), targetID, principal.PrincipalID, req.Reason); err != nil {
		writeKernelError(w, err, "")
		return
	}

	t, err := ReadTarget(r.Context(), s.db, targetID)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
