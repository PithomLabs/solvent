package api

import (
	"encoding/json"
	"net/http"
)

// handleCreatePrincipal handles POST /v1/principals.
func (s *Server) handleCreatePrincipal(w http.ResponseWriter, r *http.Request) {
	var req CreatePrincipalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}

	if err := validateNonEmpty(req.PrincipalType, "principal_type"); err != nil {
		writeValidationError(w, "principal_type", err.Error(), "")
		return
	}
	if err := validateNonEmpty(req.Issuer, "issuer"); err != nil {
		writeValidationError(w, "issuer", err.Error(), "")
		return
	}

	id, err := s.ledger.CreatePrincipal(r.Context(), req.PrincipalType, req.Issuer)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	p, err := ReadPrincipal(r.Context(), s.db, id)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// handleGetPrincipal handles GET /v1/principals/{id}.
func (s *Server) handleGetPrincipal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateUUID(id, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	p, err := ReadPrincipal(r.Context(), s.db, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// handleListPrincipals handles GET /v1/principals.
func (s *Server) handleListPrincipals(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	resp, err := ListPrincipals(r.Context(), s.db, limit, offset)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRevokePrincipal handles POST /v1/principals/{id}/revoke.
func (s *Server) handleRevokePrincipal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateUUID(id, "id"); err != nil {
		writeValidationError(w, "id", err.Error(), "")
		return
	}

	if err := s.ledger.RevokePrincipal(r.Context(), id); err != nil {
		writeKernelError(w, err, "")
		return
	}

	p, err := ReadPrincipal(r.Context(), s.db, id)
	if err != nil {
		writeKernelError(w, err, "")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}
