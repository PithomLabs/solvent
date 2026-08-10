package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/view"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// handleSolventLedger reads the current ledger for a scenario.
func handleSolventLedger(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	scenario, _ := args["scenario"].(string)
	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
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
	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
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

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.RetireDebt(ctx, beliefID, item); err != nil {
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

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.Promote(ctx, beliefID); err != nil {
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
func handleSolventAuthorizeAction(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)
	action, _ := args["action"].(string)

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id":    beliefID,
		"intent_state": "live",
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

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
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
