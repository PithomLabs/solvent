package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PithomLabs/solvent/kernel"
)

// writeError writes a canonical API error response.
func writeError(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{
		Code:    code,
		Message: message,
		Details: details,
	})
}

// writeKernelError maps kernel sentinel errors to HTTP status + canonical code.
func writeKernelError(w http.ResponseWriter, err error, requestID string) {
	if err == nil {
		return
	}

	var status int
	var code string
	var message string
	var details map[string]interface{}
	var retryable bool

	switch {
	case errors.Is(err, kernel.ErrInvalidProposal):
		status = http.StatusUnprocessableEntity
		code = "invalid_proposal"
		message = err.Error()
	case errors.Is(err, kernel.ErrApprovalPinMismatch):
		status = http.StatusConflict
		code = "approval_pin_mismatch"
		message = err.Error()
	case errors.Is(err, kernel.ErrAuthorizationMissing):
		status = http.StatusNotFound
		code = "authorization_missing"
		message = err.Error()
	case errors.Is(err, kernel.ErrAlreadyActivated):
		status = http.StatusConflict
		code = "already_activated"
		message = err.Error()
	case errors.Is(err, kernel.ErrAlreadyRevoked):
		status = http.StatusConflict
		code = "already_revoked"
		message = err.Error()
	case errors.Is(err, kernel.ErrTargetNotFound):
		status = http.StatusNotFound
		code = "target_not_found"
		message = err.Error()
	case errors.Is(err, kernel.ErrTargetNotActivated):
		status = http.StatusConflict
		code = "target_not_activated"
		message = err.Error()
	case errors.Is(err, kernel.ErrBeliefNotPromoted):
		status = http.StatusConflict
		code = "belief_not_promoted"
		message = err.Error()
	case errors.Is(err, kernel.ErrRevokedPrincipal):
		status = http.StatusForbidden
		code = "revoked_principal"
		message = err.Error()
	case errors.Is(err, kernel.ErrDuplicateDischarge):
		status = http.StatusConflict
		code = "duplicate_discharge"
		message = err.Error()
	case errors.Is(err, kernel.ErrActionOnUnpromoted):
		status = http.StatusConflict
		code = "action_on_unpromoted"
		message = err.Error()
	case errors.Is(err, kernel.ErrDuplicateIntent):
		status = http.StatusConflict
		code = "duplicate_intent"
		message = err.Error()
	default:
		status = http.StatusInternalServerError
		code = "internal_error"
		message = "An internal error occurred"
		retryable = true
	}

	// Extract SQLSTATE from the error chain if present.
	sqlState := extractSQLState(err)
	if sqlState != "" {
		if details == nil {
			details = make(map[string]interface{})
		}
		details["sqlstate"] = sqlState
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{
		Code:      code,
		Message:   message,
		Details:   details,
		Retryable: retryable,
		RequestID: requestID,
	})
}

// writeValidationError writes a 422 validation error for a specific field.
func writeValidationError(w http.ResponseWriter, field, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(APIError{
		Code:      "validation_error",
		Message:   message,
		Details:   map[string]interface{}{"field": field},
		RequestID: requestID,
	})
}

// extractSQLState attempts to extract a PostgreSQL SQLSTATE from the error chain.
func extractSQLState(err error) string {
	s := err.Error()
	for _, prefix := range []string{"SQLSTATE ", "sqlstate "} {
		for i := 0; i <= len(s)-len(prefix); i++ {
			if s[i:i+len(prefix)] == prefix {
				start := i + len(prefix)
				end := start
				for end < len(s) && s[end] != ' ' && s[end] != ',' && s[end] != ')' {
					end++
				}
				if end-start == 5 {
					return s[start:end]
				}
			}
		}
	}
	return ""
}
