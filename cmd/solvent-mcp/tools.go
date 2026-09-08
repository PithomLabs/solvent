package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/view"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// handleSolventLedger reads the current ledger for a scenario.
func handleSolventLedger(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scenario, _ := args["scenario"].(string)
	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	beliefIDRaw, hasBelief := args["belief_id"]
	beliefID, ok := beliefIDRaw.(string)
	if hasBelief && (!ok || beliefID == "") {
		return errorResult(fmt.Errorf("belief_id must be a non-empty string when provided")), nil
	}
	includeEvidence, _ := args["include_evidence"].(bool)

	opts := view.SnapshotOpts{
		BeliefID:        beliefID,
		IncludeEvidence: includeEvidence,
	}

	snap, err := view.GetSnapshot(ctx, db, scenarioID, opts)
	if err != nil {
		return errorResult(err), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}
	snap.AuditLiveOnNonPromoted = audit

	return jsonResult(snap), nil
}

// handleSolventIngestEvidence processes pinned evidence fixtures for a scenario.
func handleSolventIngestEvidence(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scenario, _ := args["scenario"].(string)
	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	fixtureDir := filepath.Join(fixtureRoot, scenario)

	results, err := pipeline.Run(ctx, db, scenarioID, fixtureDir)
	if err != nil {
		return errorResult(err), nil
	}

	type resultRow struct {
		Claim          string `json:"claim"`
		Classification string `json:"classification"`
		BeliefID       string `json:"belief_id"`
		DebtItems      int    `json:"debt_items"`
		Contradiction  bool   `json:"contradiction"`
	}

	var rows []resultRow
	for _, r := range results {
		claim := ""
		classification := ""
		if len(r.Beliefs) > 0 {
			claim = r.Beliefs[0].Claim
			classification = r.Beliefs[0].Classification
		}
		rows = append(rows, resultRow{
			Claim:          claim,
			Classification: classification,
			BeliefID:       r.BeliefID,
			DebtItems:      len(r.DebtItems),
			Contradiction:  r.Contradiction,
		})
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"results": rows,
	}, audit), nil
}

// handleSolventRetireDebt records that one review obligation has been discharged.
func handleSolventRetireDebt(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	item, _ := args["debt_item"].(string)
	scenario, _ := args["scenario"].(string)

	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	// An unrecognised debt item is refused here, not by the schema.
	//
	// Two reasons this has to be a handler check. This SDK's low-level AddTool states
	// that validating arguments against the input schema is the caller's job, so the
	// enum advertises the vocabulary but enforces nothing. And RetireDebt is
	// array_remove: retiring an item that is not present changes no array, still matches
	// the row, and returns success. Together those mean a typo or a stale vocabulary
	// would report "retired" and then fail one step later at promote time as
	// 23514 promoted_is_debt_free -- nowhere near the actual mistake.
	//
	// Refusing here keeps the error at the call that was wrong. Note this is not a
	// business rule: it rejects items the database never issues. Retiring a real item
	// that this belief has already discharged stays a no-op, as the kernel defines it.
	if !slices.Contains(kernel.FullDebt, item) {
		return errorResult(fmt.Errorf("unknown debt_item: %q (valid: %s)",
			item, strings.Join(kernel.FullDebt, ", "))), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.RetireDebt(ctx, scenarioID, beliefID, item); err != nil {
		return errorResult(err), nil
	}

	snap, err = view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil {
		return errorResult(err), nil
	}

	var debt []string
	if len(snap.Beliefs) > 0 {
		debt = snap.Beliefs[0].Debt
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id": beliefID,
		"debt":      debt,
	}, audit), nil
}

// handleSolventPromote attempts to promote a belief. The database refuses
// while the belief carries open debt (SQLSTATE 23514).
func handleSolventPromote(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)

	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.Promote(ctx, scenarioID, beliefID); err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id": beliefID,
		"status":    "promoted",
	}, audit), nil
}

// handleSolventAuthorizeAction records a live intent to act on a belief.
// The database refuses unless the belief is currently promoted (SQLSTATE 23503).
//
// MCP trust boundary: this is a stdio-based local process. The actor_id comes
// from the tool arguments, not from authenticated credentials. Authority is
// enforced by the target/snapshot approval workflow, not by caller identity.
func handleSolventAuthorizeAction(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	actionSource, _ := args["action_source"].(string)
	switch actionSource {
	case "tool_output":
		return errorResult(fmt.Errorf("action strings may not originate in tool output: retrieval is not authority")), nil
	case "user_typed":
		// valid — continue to the database path below
	default:
		return errorResult(fmt.Errorf("action_source is required and must be \"user_typed\" or \"tool_output\"")), nil
	}

	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)
	action, _ := args["action"].(string)
	targetID, _ := args["target_id"].(string)
	actorID, _ := args["actor_id"].(string)

	if targetID == "" || actorID == "" {
		return errorResult(fmt.Errorf("target_id and actor_id are required for authority verification")), nil
	}

	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	// Read the target's approved snapshot consequence_parameters.
	// Falls back to empty params when target is not activated — let authority deny.
	var snapParams []byte
	err = db.QueryRowContext(ctx, `
		SELECT ts.consequence_parameters
		FROM target_activation ta
		JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
		WHERE ta.target_id = $1::UUID`, targetID).Scan(&snapParams)
	if err != nil {
		snapParams = []byte("{}")
	}

	// Atomic authorization + intent creation. Authority evaluation and intent
	// creation occur in one SERIALIZABLE transaction — no race window between
	// verification and creation. This replaces the previous two-step path.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           actorID,
		ResourceType:          "scenario",
		ResourceID:            scenarioID,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            action,
		ConsequenceType:       "execution",
		ConsequenceParameters: snapParams,
	}

	decision, err := ledgerSvc.AuthorizeAndCreateIntent(ctx, scenarioID, beliefID, action, targetID, actorID, tuple)
	if err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	if !decision.Allowed {
		errMap := map[string]interface{}{
			"error":   true,
			"message": fmt.Sprintf("authority denied: %s", decision.Reason),
		}
		return envelopeErrorResult(ctx, db, errMap, scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id":    beliefID,
		"intent_state": decision.IntentState,
		"action":       action,
	}, audit), nil
}

// handleSolventFalsify retracts a belief and cancels its dependent live intent
// in one transaction. Single-belief retraction — no graph propagation.
func handleSolventFalsify(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)

	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	retracted, err := st.RetractCascade(ctx, scenarioID, beliefID)
	if err != nil {
		return errorResult(err), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	verdict := "PASS"
	if retracted == 0 {
		verdict = "NO-OP"
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id": beliefID,
		"retracted": retracted,
		"verdict":   verdict,
	}, audit), nil
}

// handleSolventExplain explains whether beliefs in a scenario are promotable
// or authorizable. It is strictly read-only: it derives its answer from the
// same SELECT-backed ledger/view used by solvent_ledger, introduces no new
// source of truth, and never invents SQLSTATEs. Real engine errors are only
// reported when actually emitted; predictions are labeled as predicted_*.
func handleSolventExplain(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scenario, _ := args["scenario"].(string)
	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	beliefIDRaw, hasBelief := args["belief_id"]
	beliefID, _ := beliefIDRaw.(string)
	if hasBelief && beliefID != "" {
		// Normalize empty string vs not provided.
	} else if hasBelief && beliefID == "" {
		return errorResult(fmt.Errorf("belief_id must be a non-empty string when provided")), nil
	} else {
		beliefID = ""
	}
	includeEvidence, _ := args["include_evidence"].(bool)

	opts := view.SnapshotOpts{
		BeliefID:        beliefID,
		IncludeEvidence: includeEvidence,
	}

	snap, err := view.GetSnapshot(ctx, db, scenarioID, opts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
		}
		return errorResult(err), nil
	}
	// If a specific belief was requested but GetSnapshot returned zero rows,
	// surface the same cross-scenario guard as the other belief-scoped tools.
	if beliefID != "" && len(snap.Beliefs) == 0 {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}
	if beliefID != "" && len(snap.Beliefs) == 1 && snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}
	snap.AuditLiveOnNonPromoted = audit

	explained := view.ExplainSnapshot(scenario, scenarioID, snap)
	return jsonResult(explained), nil
}

// --- authority lifecycle handlers ---

func handleSolventCreatePrincipal(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	principalType, _ := args["principal_type"].(string)
	if principalType == "" {
		return errorResult(fmt.Errorf("principal_type is required")), nil
	}
	issuer, _ := args["issuer"].(string)
	if issuer == "" {
		return errorResult(fmt.Errorf("issuer is required")), nil
	}

	st := kernel.New(db)
	id, err := st.CreatePrincipal(ctx, principalType, issuer)
	if err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"principal_id":   id,
		"principal_type": principalType,
		"issuer":         issuer,
	}), nil
}

func handleSolventRevokePrincipal(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	principalID, ok := args["principal_id"].(string)
	if !ok || principalID == "" {
		return errorResult(fmt.Errorf("principal_id is required")), nil
	}

	st := kernel.New(db)
	if err := st.RevokePrincipal(ctx, principalID); err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"principal_id": principalID,
		"revoked":      true,
	}), nil
}

func handleSolventCreateTarget(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	principalID, _ := args["principal_id"].(string)
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	scope, _ := args["scope"].(string)
	actionNS, _ := args["action_namespace"].(string)
	actionName, _ := args["action_name"].(string)
	consequenceType, _ := args["consequence_type"].(string)
	consequenceParams, _ := args["consequence_parameters"].(string)
	createdBy, _ := args["created_by"].(string)

	if principalID == "" || resourceType == "" || resourceID == "" || scope == "" || actionNS == "" || actionName == "" || consequenceType == "" || createdBy == "" {
		return errorResult(fmt.Errorf("all fields are required")), nil
	}

	params := []byte(consequenceParams)
	if consequenceParams != "" && !json.Valid(params) {
		return errorResult(fmt.Errorf("consequence_parameters must be valid JSON")), nil
	}

	st := kernel.New(db)
	id, err := st.CreateTarget(ctx, principalID, resourceType, resourceID, scope, actionNS, actionName, consequenceType, params, createdBy)
	if err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id":              id,
		"principal_id":           principalID,
		"resource_type":          resourceType,
		"resource_id":            resourceID,
		"scope":                  scope,
		"action_namespace":       actionNS,
		"action_name":            actionName,
		"consequence_type":       consequenceType,
		"consequence_parameters": consequenceParams,
	}), nil
}

func handleSolventAttachJustification(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	targetID, _ := args["target_id"].(string)
	beliefID, _ := args["belief_id"].(string)
	beliefStatus, _ := args["belief_status"].(string)
	attachedBy, _ := args["attached_by"].(string)

	if targetID == "" || beliefID == "" || beliefStatus == "" || attachedBy == "" {
		return errorResult(fmt.Errorf("all fields are required (target_id, belief_id, belief_status, attached_by)")), nil
	}

	st := kernel.New(db)
	if err := st.AttachJustification(ctx, targetID, beliefID, beliefStatus, attachedBy); err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id":     targetID,
		"belief_id":     beliefID,
		"belief_status": beliefStatus,
		"attached":      true,
	}), nil
}

func handleSolventRequestAuthorization(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	targetID, _ := args["target_id"].(string)
	requestedBy, _ := args["requested_by"].(string)

	if targetID == "" || requestedBy == "" {
		return errorResult(fmt.Errorf("target_id and requested_by are required")), nil
	}

	st := kernel.New(db)
	if err := st.RequestAuthorization(ctx, targetID, requestedBy); err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id": targetID,
		"requested": true,
	}), nil
}

func handleSolventApprove(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	targetID, _ := args["target_id"].(string)
	approvedBy, _ := args["approved_by"].(string)

	if targetID == "" || approvedBy == "" {
		return errorResult(fmt.Errorf("target_id and approved_by are required")), nil
	}

	st := kernel.New(db)
	if err := st.Approve(ctx, targetID, approvedBy); err != nil {
		return toolErrorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id":   targetID,
		"approved":    true,
		"approved_by": approvedBy,
	}), nil
}

func handleSolventAuthorize(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	targetID, _ := args["target_id"].(string)
	if targetID == "" {
		return errorResult(fmt.Errorf("target_id is required")), nil
	}

	cpStr, _ := args["consequence_parameters"].(string)
	var cp []byte
	if cpStr != "" {
		cp = []byte(cpStr)
		if !json.Valid(cp) {
			return errorResult(fmt.Errorf("consequence_parameters must be valid JSON")), nil
		}
	}

	tuple := kernel.AuthorityTuple{
		PrincipalID:           fmt.Sprint(args["principal_id"]),
		ResourceType:          fmt.Sprint(args["resource_type"]),
		ResourceID:            fmt.Sprint(args["resource_id"]),
		Scope:                 fmt.Sprint(args["scope"]),
		ActionNamespace:       fmt.Sprint(args["action_namespace"]),
		ActionName:            fmt.Sprint(args["action_name"]),
		ConsequenceType:       fmt.Sprint(args["consequence_type"]),
		ConsequenceParameters: cp,
	}

	for _, f := range []struct {
		name, val string
	}{
		{"principal_id", tuple.PrincipalID},
		{"resource_type", tuple.ResourceType},
		{"resource_id", tuple.ResourceID},
		{"scope", tuple.Scope},
		{"action_namespace", tuple.ActionNamespace},
		{"action_name", tuple.ActionName},
		{"consequence_type", tuple.ConsequenceType},
	} {
		if f.val == "" || f.val == "<nil>" {
			return errorResult(fmt.Errorf("%s is required", f.name)), nil
		}
	}

	st := kernel.New(db)
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		return toolErrorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id": targetID,
		"allowed":   result.Allowed,
		"reason":    result.Reason,
	}), nil
}

func handleSolventRevokeTarget(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	targetID, _ := args["target_id"].(string)
	revokedBy, _ := args["revoked_by"].(string)
	reason, _ := args["reason"].(string)

	if targetID == "" || revokedBy == "" || reason == "" {
		return errorResult(fmt.Errorf("target_id, revoked_by, and reason are required")), nil
	}

	st := kernel.New(db)
	if err := st.RevokeTarget(ctx, targetID, revokedBy, reason); err != nil {
		return toolErrorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"target_id": targetID,
		"revoked":   true,
	}), nil
}

func handleSolventDischarge(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, _ := args["belief_id"].(string)
	obligationKey, _ := args["obligation_key"].(string)
	instrumentRef, _ := args["instrument_ref"].(string)
	dischargedBy, _ := args["discharged_by"].(string)
	scenario, _ := args["scenario"].(string)

	if beliefID == "" || obligationKey == "" || instrumentRef == "" || dischargedBy == "" {
		return errorResult(fmt.Errorf("all fields are required (belief_id, obligation_key, instrument_ref, discharged_by)")), nil
	}

	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.Discharge(ctx, scenarioID, beliefID, obligationKey, instrumentRef, dischargedBy); err != nil {
		return toolErrorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"belief_id":      beliefID,
		"obligation_key": obligationKey,
		"instrument_ref": instrumentRef,
		"discharged":     true,
	}), nil
}

// --- execution handlers ---

// handleSolventExecute claims a live intent, invokes the configured executor
// with the approved snapshot parameters, and records the outcome.
//
// Security invariants:
//   - The authenticated principal is the MCP server process (trusted local surface).
//   - Snapshot consequence_parameters are read from the database, not from the caller.
//   - The executor is resolved from the internal registry, not caller-supplied.
//   - intent_id is the authoritative execution identity.
func handleSolventExecute(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if authSvc == nil {
		return errorResult(fmt.Errorf("execution service not configured")), nil
	}

	scenario, _ := args["scenario"].(string)
	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	action, _ := args["action"].(string)
	targetID, _ := args["target_id"].(string)
	intentID, _ := args["intent_id"].(string)

	if targetID == "" || intentID == "" {
		return errorResult(fmt.Errorf("target_id and intent_id are required")), nil
	}
	if action == "" {
		return errorResult(fmt.Errorf("action is required")), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	// Read the target's approved snapshot consequence_parameters.
	var snapParams []byte
	err = db.QueryRowContext(ctx, `
		SELECT ts.consequence_parameters
		FROM target_activation ta
		JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
		WHERE ta.target_id = $1::UUID`, targetID).Scan(&snapParams)
	if err != nil {
		snapParams = []byte("{}")
	}

	// The MCP server is a trusted local process. actor_id is attribution input.
	actorID, _ := args["actor_id"].(string)
	if actorID == "" {
		actorID = "mcp-agent"
	}

	result, err := authSvc.ExecuteAction(ctx, scenarioID, beliefID, action, targetID, actorID,
		intentID, map[string]interface{}{}, "execution", snapParams)
	if err != nil {
		return errorResult(err), nil
	}

	auditCount, auditErr := pipeline.AuditIntent(ctx, db, scenarioID)
	if auditErr != nil {
		return errorResult(auditErr), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id":   beliefID,
		"intent_id":   intentID,
		"action":      action,
		"allowed":     result.Allowed,
		"success":     result.Success,
		"output":      result.Output,
		"error":       result.Error,
		"reason":      result.Reason,
		"executed_at": result.ExecutedAt,
	}, auditCount), nil
}

// handleSolventActivity reads audit activity entries for a scenario.
// Enforces the same scenario-scoped access semantics as GET /v1/activity.
func handleSolventActivity(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scenario, _ := args["scenario"].(string)
	scenarioID, ok := lookupScenario(scenario)
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: %s)", scenario, strings.Join(scenarioNames(), ", "))), nil
	}

	if auditSvc == nil {
		return errorResult(fmt.Errorf("audit service not available")), nil
	}

	var activityType *audit.ActivityType
	if t, _ := args["type"].(string); t != "" {
		at := audit.ActivityType(t)
		activityType = &at
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	entries, err := auditSvc.GetActivities(ctx, scenarioID, activityType, limit)
	if err != nil {
		return errorResult(err), nil
	}

	return jsonResult(map[string]interface{}{
		"activities": entries,
		"total":      len(entries),
	}), nil
}

// --- helpers ---

func jsonResult(v interface{}) *mcp.CallToolResult {
	b, _ := json.Marshal(v)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

func errorResult(err error) *mcp.CallToolResult {
	b, _ := json.Marshal(toolError(err))
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

// toolErrorResult maps a kernel error to a structured MCP error result,
// preserving SQLSTATE and constraint name when available.
func toolErrorResult(err error) *mcp.CallToolResult {
	return errorResult(err)
}

func envelopeResult(db *sql.DB, result interface{}, audit int) *mcp.CallToolResult {
	envelope := map[string]interface{}{
		"result": result,
		"audit":  map[string]interface{}{"live_on_nonpromoted": audit},
	}
	b, _ := json.Marshal(envelope)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

func envelopeErrorResult(ctx context.Context, db *sql.DB, errResult map[string]interface{}, scenarioID string) *mcp.CallToolResult {
	audit, auditErr := pipeline.AuditIntent(ctx, db, scenarioID)
	envelope := map[string]interface{}{
		"result": errResult,
	}
	if auditErr != nil {
		envelope["audit"] = nil
		envelope["audit_error"] = auditErr.Error()
	} else {
		envelope["audit"] = map[string]interface{}{"live_on_nonpromoted": audit}
	}
	b, _ := json.Marshal(envelope)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}
