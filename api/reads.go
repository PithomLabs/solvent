package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ReadBelief returns a single belief with evidence and intent counts.
func ReadBelief(ctx context.Context, db *sql.DB, scenarioID, beliefID string) (*BeliefResponse, error) {
	var b BeliefResponse
	var debtRaw string
	err := db.QueryRowContext(ctx,
		`SELECT id, scenario_id, claim, claim_type, status, debt::STRING, final_truth
		 FROM belief WHERE scenario_id=$1::UUID AND id=$2::UUID`,
		scenarioID, beliefID).Scan(
		&b.BeliefID, &b.ScenarioID, &b.Claim, &b.ClaimType, &b.Status, &debtRaw, &b.FinalTruth)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("belief %s not found in scenario %s", beliefID, scenarioID)
	}
	if err != nil {
		return nil, err
	}
	b.Debt = parsePGArray(debtRaw)

	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE belief_id=$1::UUID`, beliefID).Scan(&b.EvidenceCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE belief_id=$1::UUID`, beliefID).Scan(&b.IntentCount)

	return &b, nil
}

// ListBeliefs returns beliefs with optional status/claimType filtering and pagination.
func ListBeliefs(ctx context.Context, db *sql.DB, scenarioID, status, claimType string, limit, offset int) (*BeliefListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	where := "scenario_id=$1::UUID"
	args := []interface{}{scenarioID}
	argN := 2

	if status != "" {
		where += fmt.Sprintf(" AND status=$%d::STRING", argN)
		args = append(args, status)
		argN++
	}
	if claimType != "" {
		where += fmt.Sprintf(" AND claim_type=$%d::STRING", argN)
		args = append(args, claimType)
		argN++
	}

	// Count total.
	var total int
	countQuery := fmt.Sprintf("SELECT count(*) FROM belief WHERE %s", where)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Fetch page.
	query := fmt.Sprintf(
		`SELECT id, scenario_id, claim, claim_type, status, debt::STRING, final_truth
		 FROM belief WHERE %s ORDER BY claim LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &BeliefListResponse{
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}
	for rows.Next() {
		var b BeliefResponse
		var debtRaw string
		if err := rows.Scan(&b.BeliefID, &b.ScenarioID, &b.Claim, &b.ClaimType, &b.Status, &debtRaw, &b.FinalTruth); err != nil {
			return nil, err
		}
		b.Debt = parsePGArray(debtRaw)
		resp.Beliefs = append(resp.Beliefs, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if resp.Beliefs == nil {
		resp.Beliefs = []BeliefResponse{}
	}
	return resp, nil
}

// ReadEvidence returns a single evidence row.
func ReadEvidence(ctx context.Context, db *sql.DB, evidenceID string) (*EvidenceResponse, error) {
	var e EvidenceResponse
	err := db.QueryRowContext(ctx,
		`SELECT id, belief_id, provenance_class, source_url, content_sha256, ingested_at
		 FROM evidence WHERE id=$1::UUID`,
		evidenceID).Scan(
		&e.EvidenceID, &e.BeliefID, &e.ProvenanceClass, &e.SourceURL, &e.ContentSHA256, &e.IngestedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("evidence %s not found", evidenceID)
	}
	return &e, err
}

// ListEvidenceForBelief returns all evidence for a belief.
func ListEvidenceForBelief(ctx context.Context, db *sql.DB, scenarioID, beliefID string) ([]EvidenceResponse, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, belief_id, provenance_class, source_url, content_sha256, ingested_at
		 FROM evidence WHERE scenario_id=$1::UUID AND belief_id=$2::UUID ORDER BY ingested_at`,
		scenarioID, beliefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EvidenceResponse
	for rows.Next() {
		var e EvidenceResponse
		if err := rows.Scan(&e.EvidenceID, &e.BeliefID, &e.ProvenanceClass, &e.SourceURL, &e.ContentSHA256, &e.IngestedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []EvidenceResponse{}
	}
	return out, nil
}

// ReadPrincipal returns a single principal.
func ReadPrincipal(ctx context.Context, db *sql.DB, principalID string) (*PrincipalResponse, error) {
	var p PrincipalResponse
	var revokedAt sql.NullTime
	err := db.QueryRowContext(ctx,
		`SELECT principal_id, principal_type, issuer, revoked_at, created_at
		 FROM principal WHERE principal_id=$1::UUID`,
		principalID).Scan(
		&p.PrincipalID, &p.PrincipalType, &p.Issuer, &revokedAt, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("principal %s not found", principalID)
	}
	if err != nil {
		return nil, err
	}
	if revokedAt.Valid {
		p.RevokedAt = &revokedAt.Time
	}
	return &p, nil
}

// ListPrincipals returns principals with pagination.
func ListPrincipals(ctx context.Context, db *sql.DB, limit, offset int) (*PrincipalListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM principal`).Scan(&total)

	rows, err := db.QueryContext(ctx,
		`SELECT principal_id, principal_type, issuer, revoked_at, created_at
		 FROM principal ORDER BY created_at LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &PrincipalListResponse{
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}
	for rows.Next() {
		var p PrincipalResponse
		var revokedAt sql.NullTime
		if err := rows.Scan(&p.PrincipalID, &p.PrincipalType, &p.Issuer, &revokedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		if revokedAt.Valid {
			p.RevokedAt = &revokedAt.Time
		}
		resp.Principals = append(resp.Principals, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if resp.Principals == nil {
		resp.Principals = []PrincipalResponse{}
	}
	return resp, nil
}

// ReadTarget returns a target with derived state.
func ReadTarget(ctx context.Context, db *sql.DB, targetID string) (*TargetResponse, error) {
	var t TargetResponse
	var requestedBy sql.NullString
	var requestedAt sql.NullTime
	var revokedAt sql.NullTime

	err := db.QueryRowContext(ctx,
		`SELECT t.target_id, t.principal_id, t.resource_type, t.resource_id, t.scope,
		        t.action_namespace, t.action_name, t.consequence_type, t.consequence_parameters,
		        t.created_by, t.created_at,
		        t.requested_by, t.requested_at,
		        r.revoked_at
		 FROM authority_target t
		 LEFT JOIN target_revocation r ON r.target_id = t.target_id
		 WHERE t.target_id=$1::UUID`,
		targetID).Scan(
		&t.TargetID, &t.PrincipalID, &t.ResourceType, &t.ResourceID, &t.Scope,
		&t.ActionNamespace, &t.ActionName, &t.ConsequenceType, &t.ConsequenceParameters,
		&t.CreatedBy, &t.CreatedAt,
		&requestedBy, &requestedAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("target %s not found", targetID)
	}
	if err != nil {
		return nil, err
	}

	// Derive state.
	switch {
	case revokedAt.Valid:
		t.State = "revoked"
	case requestedAt.Valid:
		t.State = "requested"
	default:
		t.State = "proposed"
	}
	if requestedBy.Valid {
		t.RequestedBy = &requestedBy.String
	}
	if requestedAt.Valid {
		t.RequestedAt = &requestedAt.Time
	}

	return &t, nil
}

// ListTargets returns targets with pagination.
func ListTargets(ctx context.Context, db *sql.DB, limit, offset int) (*TargetListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM authority_target`).Scan(&total)

	rows, err := db.QueryContext(ctx,
		`SELECT t.target_id, t.principal_id, t.resource_type, t.resource_id, t.scope,
		        t.action_namespace, t.action_name, t.consequence_type, t.consequence_parameters,
		        t.created_by, t.created_at,
		        t.requested_by, t.requested_at,
		        r.revoked_at
		 FROM authority_target t
		 LEFT JOIN target_revocation r ON r.target_id = t.target_id
		 ORDER BY t.created_at LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &TargetListResponse{
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}
	for rows.Next() {
		var t TargetResponse
		var requestedBy sql.NullString
		var requestedAt sql.NullTime
		var revokedAt sql.NullTime
		if err := rows.Scan(
			&t.TargetID, &t.PrincipalID, &t.ResourceType, &t.ResourceID, &t.Scope,
			&t.ActionNamespace, &t.ActionName, &t.ConsequenceType, &t.ConsequenceParameters,
			&t.CreatedBy, &t.CreatedAt,
			&requestedBy, &requestedAt, &revokedAt); err != nil {
			return nil, err
		}
		switch {
		case revokedAt.Valid:
			t.State = "revoked"
		case requestedAt.Valid:
			t.State = "requested"
		default:
			t.State = "proposed"
		}
		if requestedBy.Valid {
			t.RequestedBy = &requestedBy.String
		}
		if requestedAt.Valid {
			t.RequestedAt = &requestedAt.Time
		}
		resp.Targets = append(resp.Targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if resp.Targets == nil {
		resp.Targets = []TargetResponse{}
	}
	return resp, nil
}

// ReadActivity returns activities from the audit service.
func ReadActivity(ctx context.Context, db *sql.DB, scenarioID, activityType string, limit, offset int) (*ActivityResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	where := "scenario_id=$1::UUID"
	args := []interface{}{scenarioID}
	argN := 2

	if activityType != "" {
		where += fmt.Sprintf(" AND type=$%d::STRING", argN)
		args = append(args, activityType)
		argN++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT count(*) FROM audit_activity WHERE %s", where)
	_ = db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query := fmt.Sprintf(
		`SELECT id, scenario_id, type, actor_id, subject_id, details, sqlstate, constraint_name, refusal, created_at
		 FROM audit_activity WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &ActivityResponse{
		Total: total,
	}
	for rows.Next() {
		var a ActivityEntry
		var detailsRaw []byte
		var sqlState, constraintName sql.NullString
		if err := rows.Scan(
			&a.ID, &a.ScenarioID, &a.Type, &a.ActorID, &a.SubjectID,
			&detailsRaw, &sqlState, &constraintName, &a.Refusal, &a.CreatedAt); err != nil {
			return nil, err
		}
		if detailsRaw != nil {
			_ = json.Unmarshal(detailsRaw, &a.Details)
		}
		if sqlState.Valid {
			a.SQLState = sqlState.String
		}
		if constraintName.Valid {
			a.ConstraintName = constraintName.String
		}
		resp.Activities = append(resp.Activities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if resp.Activities == nil {
		resp.Activities = []ActivityEntry{}
	}
	return resp, nil
}

// ReadLedgerSummary returns 6 aggregate counts for a scenario.
func ReadLedgerSummary(ctx context.Context, db *sql.DB, scenarioID string) (*LedgerSummaryResponse, error) {
	resp := &LedgerSummaryResponse{ScenarioID: scenarioID}

	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, scenarioID).Scan(&resp.BeliefCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID`, scenarioID).Scan(&resp.EvidenceCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND status='promoted'`, scenarioID).Scan(&resp.PromotedCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='live'`, scenarioID).Scan(&resp.LiveIntentCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND status='retracted'`, scenarioID).Scan(&resp.RetractedCount)
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent a JOIN belief b ON b.id = a.belief_id
		 WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id=$1::UUID`,
		scenarioID).Scan(&resp.LiveOnNonPromoted)

	return resp, nil
}

// BeliefExplainResponse mirrors view.BeliefExplain for the API.
type BeliefExplainResponse struct {
	BeliefID                     string   `json:"belief_id"`
	Claim                        string   `json:"claim"`
	ClaimType                    string   `json:"claim_type"`
	Status                       string   `json:"status"`
	RemainingDebt                []string `json:"remaining_debt"`
	FinalTruth                   bool     `json:"final_truth"`
	IsPromoted                   bool     `json:"is_promoted"`
	IsRetracted                  bool     `json:"is_retracted"`
	CanPromote                   bool     `json:"can_promote"`
	PromotionBlockedReason       string   `json:"promotion_blocked_reason,omitempty"`
	PredictedPromotionSQLState   string   `json:"predicted_promotion_sqlstate,omitempty"`
	PredictedPromotionConstraint string   `json:"predicted_promotion_constraint,omitempty"`
	CanAuthorize                 bool     `json:"can_authorize"`
	AuthorizationBlockedReason   string   `json:"authorization_blocked_reason,omitempty"`
	PredictedAuthSQLState        string   `json:"predicted_authorization_sqlstate,omitempty"`
	PredictedAuthConstraint      string   `json:"predicted_authorization_constraint,omitempty"`
	LiveIntents                  []struct {
		BeliefID string `json:"belief_id"`
		Action   string `json:"action"`
		State    string `json:"state"`
	} `json:"live_intents"`
	EvidenceCount int    `json:"evidence_count"`
	HumanSummary  string `json:"human_summary"`
}

// ReadBeliefExplain returns the explain projection for a belief.
func ReadBeliefExplain(ctx context.Context, db *sql.DB, scenarioID, beliefID string) (*BeliefExplainResponse, error) {
	// Delegate to the view package for explain logic.
	// For now, read the belief and construct a minimal explain.
	b, err := ReadBelief(ctx, db, scenarioID, beliefID)
	if err != nil {
		return nil, err
	}

	resp := &BeliefExplainResponse{
		BeliefID:      b.BeliefID,
		Claim:         b.Claim,
		ClaimType:     b.ClaimType,
		Status:        b.Status,
		RemainingDebt: b.Debt,
		FinalTruth:    b.FinalTruth,
		IsPromoted:    b.Status == "promoted",
		IsRetracted:   b.Status == "retracted",
		EvidenceCount: b.EvidenceCount,
	}

	// CanPromote logic mirrors the DB CHECK promoted_is_debt_free.
	switch {
	case b.Status == "retracted":
		resp.CanPromote = false
		resp.PromotionBlockedReason = "belief is retracted"
	case b.Status == "promoted":
		resp.CanPromote = false
		resp.PromotionBlockedReason = "already promoted"
	case b.FinalTruth:
		resp.CanPromote = false
		resp.PromotionBlockedReason = "blocked by final_truth=true"
		resp.PredictedPromotionSQLState = "23514"
		resp.PredictedPromotionConstraint = "promoted_is_debt_free"
	case len(b.Debt) > 0:
		resp.CanPromote = false
		resp.PromotionBlockedReason = fmt.Sprintf("%d unresolved obligation(s) remain", len(b.Debt))
		resp.PredictedPromotionSQLState = "23514"
		resp.PredictedPromotionConstraint = "promoted_is_debt_free"
	default:
		resp.CanPromote = true
	}

	// CanAuthorize logic mirrors the composite FK gate.
	switch {
	case b.Status == "promoted" && len(b.Debt) == 0 && !b.FinalTruth:
		resp.CanAuthorize = true
	case b.Status == "retracted":
		resp.CanAuthorize = false
		resp.AuthorizationBlockedReason = "belief is retracted"
		resp.PredictedAuthSQLState = "23503"
		resp.PredictedAuthConstraint = "gate"
	default:
		resp.CanAuthorize = false
		resp.AuthorizationBlockedReason = fmt.Sprintf("belief is not promoted (status=%q)", b.Status)
		resp.PredictedAuthSQLState = "23503"
		resp.PredictedAuthConstraint = "gate"
	}

	// Live intents.
	rows, err := db.QueryContext(ctx,
		`SELECT belief_id, action, state FROM action_intent WHERE belief_id=$1::UUID AND state='live'`,
		beliefID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var li struct {
				BeliefID string `json:"belief_id"`
				Action   string `json:"action"`
				State    string `json:"state"`
			}
			if err := rows.Scan(&li.BeliefID, &li.Action, &li.State); err == nil {
				resp.LiveIntents = append(resp.LiveIntents, li)
			}
		}
	}
	if resp.LiveIntents == nil {
		resp.LiveIntents = []struct {
			BeliefID string `json:"belief_id"`
			Action   string `json:"action"`
			State    string `json:"state"`
		}{}
	}

	// Build human summary.
	resp.HumanSummary = buildExplainSummary(resp)

	return resp, nil
}

func buildExplainSummary(be *BeliefExplainResponse) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Belief %q", be.Claim))

	switch be.Status {
	case "entered":
		if len(be.RemainingDebt) == 0 && !be.FinalTruth {
			parts = append(parts, "is entered and ready to promote")
		} else {
			parts = append(parts, fmt.Sprintf("is entered with %d unresolved obligation(s)", len(be.RemainingDebt)))
		}
	case "promoted":
		parts = append(parts, "is promoted")
	case "retracted":
		parts = append(parts, "is retracted")
	}

	if be.CanPromote {
		parts = append(parts, "promotion would succeed")
	} else if be.PromotionBlockedReason != "" && be.Status != "promoted" && be.Status != "retracted" {
		parts = append(parts, fmt.Sprintf("promotion blocked: %s", be.PromotionBlockedReason))
	}

	if be.CanAuthorize {
		parts = append(parts, "authorization would succeed")
	} else if be.AuthorizationBlockedReason != "" {
		parts = append(parts, fmt.Sprintf("authorization blocked: %s", be.AuthorizationBlockedReason))
	}

	return strings.Join(parts, ". ") + "."
}

// parsePGArray parses a PostgreSQL text array literal like {a,b,c} into a string slice.
func parsePGArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return []string{}
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}
