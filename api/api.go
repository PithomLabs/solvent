package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/ledger"
)

// Server is the canonical REST API server.
type Server struct {
	db       *sql.DB
	ledger   *ledger.Service
	auditSvc *audit.Service
}

// NewServer creates a new API server.
func NewServer(db *sql.DB, auditSvc *audit.Service) *Server {
	ls := ledger.New(db, auditSvc)
	return &Server{
		db:       db,
		ledger:   ls,
		auditSvc: auditSvc,
	}
}

// Handler returns the configured HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Belief routes.
	mux.HandleFunc("POST /v1/beliefs", s.handleEnterBelief)
	mux.HandleFunc("GET /v1/beliefs/{id}", s.handleGetBelief)
	mux.HandleFunc("GET /v1/beliefs", s.handleListBeliefs)
	mux.HandleFunc("POST /v1/beliefs/{id}/debt/retire", s.handleRetireDebt)
	mux.HandleFunc("POST /v1/beliefs/{id}/promote", s.handlePromoteBelief)
	mux.HandleFunc("POST /v1/beliefs/{id}/retract", s.handleRetractBelief)
	mux.HandleFunc("GET /v1/beliefs/{id}/explain", s.handleExplainBelief)
	mux.HandleFunc("GET /v1/beliefs/{id}/evidence", s.handleListEvidenceForBelief)

	// Evidence routes.
	mux.HandleFunc("POST /v1/evidence", s.handleAddEvidence)
	mux.HandleFunc("GET /v1/evidence/{id}", s.handleGetEvidence)

	// Principal routes.
	mux.HandleFunc("POST /v1/principals", s.handleCreatePrincipal)
	mux.HandleFunc("GET /v1/principals/{id}", s.handleGetPrincipal)
	mux.HandleFunc("GET /v1/principals", s.handleListPrincipals)
	mux.HandleFunc("POST /v1/principals/{id}/revoke", s.handleRevokePrincipal)

	// Target routes.
	mux.HandleFunc("POST /v1/targets", s.handleCreateTarget)
	mux.HandleFunc("GET /v1/targets/{id}", s.handleGetTarget)
	mux.HandleFunc("GET /v1/targets", s.handleListTargets)
	mux.HandleFunc("POST /v1/targets/{id}/justifications", s.handleAttachJustification)
	mux.HandleFunc("POST /v1/targets/{id}/request", s.handleRequestAuthorization)
	mux.HandleFunc("POST /v1/targets/{id}/approve", s.handleApproveTarget)
	mux.HandleFunc("POST /v1/targets/{id}/revoke", s.handleRevokeTarget)

	// Authorization routes.
	mux.HandleFunc("POST /v1/authorizations/verify", s.handleVerifyAuthorization)
	mux.HandleFunc("POST /v1/authorizations/action", s.handleAuthorizeAction)

	// Discharge route.
	mux.HandleFunc("POST /v1/discharge", s.handleDischarge)

	// Activity route.
	mux.HandleFunc("GET /v1/activity", s.handleGetActivity)

	// Ledger route.
	mux.HandleFunc("GET /v1/ledger", s.handleGetLedger)

	return mux
}

// auditLog is a helper for handlers to emit audit entries.
func (s *Server) auditLog(ctx context.Context, scenarioID, activityType string, principal *AuthenticatedPrincipal, subjectID string, details map[string]interface{}) {
	if s.auditSvc == nil {
		return
	}
	actorID := ""
	if principal != nil {
		actorID = principal.PrincipalID
	}
	s.auditSvc.Log(ctx, &audit.ActivityEntry{
		ScenarioID: scenarioID,
		Type:       audit.ActivityType(activityType),
		ActorID:    actorID,
		SubjectID:  subjectID,
		Details:    details,
	})
}

// queryInt parses an integer query parameter with a default value.
func queryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}
