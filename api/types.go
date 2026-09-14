package api

import (
	"encoding/json"
	"time"
)

// --- Belief types ---

// EnterBeliefRequest is the request body for POST /v1/beliefs.
type EnterBeliefRequest struct {
	ScenarioID string   `json:"scenario_id"`
	Claim      string   `json:"claim"`
	ClaimType  string   `json:"claim_type"`
	Debt       []string `json:"debt,omitempty"`
}

// BeliefResponse is a belief representation in API responses.
type BeliefResponse struct {
	BeliefID      string    `json:"belief_id"`
	ScenarioID    string    `json:"scenario_id"`
	Claim         string    `json:"claim"`
	ClaimType     string    `json:"claim_type"`
	Status        string    `json:"status"`
	Debt          []string  `json:"debt"`
	FinalTruth    bool      `json:"final_truth"`
	EvidenceCount int       `json:"evidence_count"`
	IntentCount   int       `json:"intent_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// BeliefListResponse is a paginated list of beliefs.
type BeliefListResponse struct {
	Beliefs []BeliefResponse `json:"beliefs"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// RetireDebtRequest is the request body for POST /v1/beliefs/{id}/debt/retire.
type RetireDebtRequest struct {
	DebtItem string `json:"debt_item"`
}

// Verdict is the refusal response for Promote (HTTP 200 with refusal body).
type Verdict struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Gate   string `json:"gate"`
}

// --- Evidence types ---

// AddEvidenceRequest is the request body for POST /v1/evidence.
type AddEvidenceRequest struct {
	ScenarioID      string `json:"scenario_id"`
	BeliefID        string `json:"belief_id"`
	ProvenanceClass string `json:"provenance_class"`
	SourceURL       string `json:"source_url"`
	ContentSHA256   string `json:"content_sha256"`
}

// EvidenceResponse is an evidence representation in API responses.
type EvidenceResponse struct {
	EvidenceID      string    `json:"evidence_id"`
	BeliefID        string    `json:"belief_id"`
	ProvenanceClass string    `json:"provenance_class"`
	SourceURL       string    `json:"source_url"`
	ContentSHA256   string    `json:"content_sha256"`
	IngestedAt      time.Time `json:"ingested_at"`
}

// --- Principal types ---

// CreatePrincipalRequest is the request body for POST /v1/principals.
type CreatePrincipalRequest struct {
	PrincipalType string `json:"principal_type"`
	Issuer        string `json:"issuer"`
}

// PrincipalResponse is a principal representation in API responses.
type PrincipalResponse struct {
	PrincipalID   string     `json:"principal_id"`
	PrincipalType string     `json:"principal_type"`
	Issuer        string     `json:"issuer"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PrincipalListResponse is a paginated list of principals.
type PrincipalListResponse struct {
	Principals []PrincipalResponse `json:"principals"`
	Total      int                 `json:"total"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

// --- Target types ---

// CreateTargetRequest is the request body for POST /v1/targets.
type CreateTargetRequest struct {
	PrincipalID           string          `json:"principal_id"`
	ResourceType          string          `json:"resource_type"`
	ResourceID            string          `json:"resource_id"`
	Scope                 string          `json:"scope"`
	ActionNamespace       string          `json:"action_namespace"`
	ActionName            string          `json:"action_name"`
	ConsequenceType       string          `json:"consequence_type"`
	ConsequenceParameters json.RawMessage `json:"consequence_parameters"`
	CreatedBy             string          `json:"created_by"`
}

// TargetResponse is a target representation in API responses.
type TargetResponse struct {
	TargetID              string          `json:"target_id"`
	PrincipalID           string          `json:"principal_id"`
	ResourceType          string          `json:"resource_type"`
	ResourceID            string          `json:"resource_id"`
	Scope                 string          `json:"scope"`
	ActionNamespace       string          `json:"action_namespace"`
	ActionName            string          `json:"action_name"`
	ConsequenceType       string          `json:"consequence_type"`
	ConsequenceParameters json.RawMessage `json:"consequence_parameters"`
	CreatedBy             string          `json:"created_by"`
	State                 string          `json:"state"`
	CreatedAt             time.Time       `json:"created_at"`
	RequestedAt           *time.Time      `json:"requested_at,omitempty"`
	RequestedBy           *string         `json:"requested_by,omitempty"`
}

// TargetListResponse is a paginated list of targets.
type TargetListResponse struct {
	Targets []TargetResponse `json:"targets"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// AttachJustificationRequest is the request body for POST /v1/targets/{id}/justifications.
type AttachJustificationRequest struct {
	InstrumentRef string `json:"instrument_ref"`
}

// ApproveTargetRequest is the request body for POST /v1/targets/{id}/approve.
type ApproveTargetRequest struct {
	ApprovalPin string `json:"approval_pin"`
}

// RevokeTargetRequest is the request body for POST /v1/targets/{id}/revoke.
type RevokeTargetRequest struct {
	Reason string `json:"reason"`
}

// --- Authorization types ---

// VerifyAuthRequest is the request body for POST /v1/authorizations/verify.
type VerifyAuthRequest struct {
	TargetID              string          `json:"target_id"`
	ResourceType          string          `json:"resource_type"`
	ResourceID            string          `json:"resource_id"`
	Scope                 string          `json:"scope"`
	ActionNamespace       string          `json:"action_namespace"`
	ActionName            string          `json:"action_name"`
	ConsequenceType       string          `json:"consequence_type"`
	ConsequenceParameters json.RawMessage `json:"consequence_parameters"`
}

// AuthResult is the response for authorization verification.
type AuthResult struct {
	TargetID string `json:"target_id"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
}

// AuthorizeActionRequest is the request body for POST /v1/authorizations/action.
type AuthorizeActionRequest struct {
	ScenarioID   string `json:"scenario_id"`
	BeliefID     string `json:"belief_id"`
	Action       string `json:"action"`
	ActionSource string `json:"action_source"`
	TargetID     string `json:"target_id"`
	ActorID      string `json:"actor_id"`
}

// AuthorizeActionResult is the response for authorize-action.
type AuthorizeActionResult struct {
	BeliefID    string     `json:"belief_id"`
	IntentID    string     `json:"intent_id,omitempty"`
	IntentState string     `json:"intent_state"`
	Action      string     `json:"action"`
	Authority   AuthResult `json:"authority"`
}

// --- Execution types ---

// ExecuteActionRequest is the request body for POST /v1/authorizations/execute.
// The authenticated principal is derived from the API key, not from the request body.
// consequence_parameters are read from the approved snapshot, not from the caller.
type ExecuteActionRequest struct {
	ScenarioID      string `json:"scenario_id"`
	BeliefID        string `json:"belief_id"`
	Action          string `json:"action"`
	TargetID        string `json:"target_id"`
	IntentID        string `json:"intent_id"`
	ConsequenceType string `json:"consequence_type"`
}

// ExecuteActionResult is the response for execute-action.
type ExecuteActionResult struct {
	BeliefID    string    `json:"belief_id"`
	IntentID    string    `json:"intent_id,omitempty"`
	IntentState string    `json:"intent_state,omitempty"`
	Action      string    `json:"action"`
	Allowed     bool      `json:"allowed"`
	Success     bool      `json:"success"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	ExecutedAt  time.Time `json:"executed_at"`
}

// --- Discharge types ---

// DischargeRequest is the request body for POST /v1/discharge.
type DischargeRequest struct {
	ScenarioID    string `json:"scenario_id"`
	BeliefID      string `json:"belief_id"`
	ObligationKey string `json:"obligation_key"`
	InstrumentRef string `json:"instrument_ref"`
	DischargedBy  string `json:"discharged_by"`
}

// DischargeResult is the response for discharge.
type DischargeResult struct {
	BeliefID      string `json:"belief_id"`
	ObligationKey string `json:"obligation_key"`
	DischargedBy  string `json:"discharged_by"`
}

// --- Activity types ---

// ActivityResponse is a paginated list of audit activities.
type ActivityResponse struct {
	Activities []ActivityEntry `json:"activities"`
	Total      int             `json:"total"`
}

// ActivityEntry is an audit activity representation.
type ActivityEntry struct {
	ID             string                 `json:"id"`
	ScenarioID     string                 `json:"scenario_id"`
	Type           string                 `json:"type"`
	ActorID        string                 `json:"actor_id"`
	SubjectID      string                 `json:"subject_id"`
	Details        map[string]interface{} `json:"details"`
	SQLState       string                 `json:"sqlstate,omitempty"`
	ConstraintName string                 `json:"constraint_name,omitempty"`
	Refusal        bool                   `json:"refusal"`
	CreatedAt      time.Time              `json:"created_at"`
}

// --- Ledger types ---

// LedgerSummaryResponse is the summary for GET /v1/ledger.
type LedgerSummaryResponse struct {
	ScenarioID        string `json:"scenario_id"`
	BeliefCount       int    `json:"belief_count"`
	EvidenceCount     int    `json:"evidence_count"`
	PromotedCount     int    `json:"promoted_count"`
	LiveIntentCount   int    `json:"live_intent_count"`
	RetractedCount    int    `json:"retracted_count"`
	LiveOnNonPromoted int    `json:"live_on_nonpromoted"`
}

// --- Error type ---

// APIError is the canonical error response.
type APIError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
	RequestID string                 `json:"request_id,omitempty"`
}
