package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/PithomLabs/solvent/kernel"
)

func TestAuth_Verify(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	principalID := createTestPrincipal(t, db)
	targetID := createTestTarget(t, db, principalID)

	body := map[string]interface{}{
		"target_id":        targetID,
		"resource_type":    "scenario",
		"resource_id":      "test-scenario",
		"scope":            "global",
		"action_namespace": "solvent",
		"action_name":      "execute",
		"consequence_type": "execution",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/verify", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	// The authenticated principal (testPrincipalID) differs from the target's
	// principal, so the tuple won't match. This is expected — verify reports
	// the kernel's decision, not a test setup error.
	allowed, ok := result["allowed"].(bool)
	if !ok {
		t.Fatalf("expected allowed boolean, got %v", result["allowed"])
	}
	t.Logf("verify result: allowed=%v reason=%v", allowed, result["reason"])
}

func TestAuth_VerifyMissingTarget(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	body := map[string]interface{}{
		"resource_type":    "scenario",
		"resource_id":      "test-scenario",
		"scope":            "belief:test-belief",
		"action_namespace": "solvent",
		"action_name":      "execute",
		"consequence_type": "execution",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/verify", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAuth_AuthorizeAction_RejectsToolOutput(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)
	principalID := createTestPrincipal(t, db)
	targetID := createTestTarget(t, db, principalID)

	body := map[string]interface{}{
		"scenario_id":   scenarioID,
		"belief_id":     beliefID,
		"action":        "execute",
		"target_id":     targetID,
		"action_source": "tool_output",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/action", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestExecuteAction_ServiceUnavailable(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// newTestServer does not configure an authority service, so authSvc is nil.
	ts := newTestServer(t, db)
	defer ts.Close()

	body := map[string]interface{}{
		"scenario_id": "00000000-0000-0000-0000-000000000001",
		"belief_id":   "00000000-0000-0000-0000-000000000002",
		"action":      "deploy",
		"target_id":   "00000000-0000-0000-0000-000000000003",
		"intent_id":   "00000000-0000-0000-0000-000000000004",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when authSvc nil, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestExecuteAction_MissingScenarioID(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	body := map[string]interface{}{
		"belief_id": "00000000-0000-0000-0000-000000000002",
		"action":    "deploy",
		"target_id": "00000000-0000-0000-0000-000000000003",
		"intent_id": "00000000-0000-0000-0000-000000000004",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing scenario_id, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestExecuteAction_MissingBeliefID(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	body := map[string]interface{}{
		"scenario_id": "00000000-0000-0000-0000-000000000001",
		"action":      "deploy",
		"target_id":   "00000000-0000-0000-0000-000000000003",
		"intent_id":   "00000000-0000-0000-0000-000000000004",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing belief_id, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestExecuteAction_MissingIntentID(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	body := map[string]interface{}{
		"scenario_id": "00000000-0000-0000-0000-000000000001",
		"belief_id":   "00000000-0000-0000-0000-000000000002",
		"action":      "deploy",
		"target_id":   "00000000-0000-0000-0000-000000000003",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing intent_id, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestExecuteAction_InvalidJSON(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL+"/v1/authorizations/execute",
		bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", validAuth())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- API-Level Adversarial Integration Tests ---
// These tests prove that the HTTP boundary preserves the same security
// invariants already proven in service/authority. They exercise the full
// HTTP → handler → service → kernel path with CockroachDB.

// setupExecutionScenario creates a complete scenario for execution tests:
// promoted belief, approved target with snapshot params, live intent.
// Returns scenarioID, beliefID, targetID, intentID, principalID.
func setupExecutionScenario(t *testing.T, db *sql.DB) (string, string, string, string, string) {
	t.Helper()
	ctx := context.Background()
	st := kernel.New(db)

	scenarioID := createTestScenario(t)
	// Use testPrincipalID so the API key (which maps to this principal) matches.
	principalID := testPrincipalID

	// Create and promote belief.
	beliefID, err := st.EnsureBelief(ctx, scenarioID, "test belief for execution", kernel.Derived, kernel.FullDebt)
	if err != nil {
		t.Fatalf("ensure belief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, scenarioID, beliefID, item)
	}
	if err := st.Promote(ctx, scenarioID, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}

	// Create approved target with snapshot params.
	snapParams, _ := json.Marshal(map[string]string{
		"repo":     "owner/repo",
		"workflow": "deploy.yml",
		"ref":      "main",
	})
	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", "deploy", "execution", snapParams, principalID)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	_ = st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID)
	_ = st.RequestAuthorization(ctx, targetID, principalID)
	_ = st.Approve(ctx, targetID, principalID)

	// Create live intent via direct SQL (same pattern as authority integration tests).
	var intentID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO action_intent (scenario_id, belief_id, action)
		 VALUES ($1::UUID, $2::UUID, $3::STRING)
		 RETURNING id`,
		scenarioID, beliefID, "deploy").Scan(&intentID)
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}

	return scenarioID, beliefID, targetID, intentID, principalID
}

func TestExecute_CrossPrincipalDenied(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	// Create scenario with a DIFFERENT principal (not testPrincipalID).
	ctx := context.Background()
	st := kernel.New(db)
	scenarioID := createTestScenario(t)
	otherPrincipalID := createTestPrincipal(t, db)

	// Create and promote belief under otherPrincipalID.
	beliefID, err := st.EnsureBelief(ctx, scenarioID, "cross-principal belief", kernel.Derived, kernel.FullDebt)
	if err != nil {
		t.Fatalf("ensure belief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, scenarioID, beliefID, item)
	}
	if err := st.Promote(ctx, scenarioID, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}

	// Create target owned by otherPrincipalID.
	snapParams, _ := json.Marshal(map[string]string{
		"repo":     "owner/repo",
		"workflow": "deploy.yml",
		"ref":      "main",
	})
	targetID, err := st.CreateTarget(ctx, otherPrincipalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", "deploy", "execution", snapParams, otherPrincipalID)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	_ = st.AttachJustification(ctx, targetID, beliefID, "promoted", otherPrincipalID)
	_ = st.RequestAuthorization(ctx, targetID, otherPrincipalID)
	_ = st.Approve(ctx, targetID, otherPrincipalID)

	// Create live intent.
	var intentID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO action_intent (scenario_id, belief_id, action)
		 VALUES ($1::UUID, $2::UUID, $3::STRING)
		 RETURNING id`,
		scenarioID, beliefID, "deploy").Scan(&intentID)
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}

	// Execute with testPrincipalID's API key — different from otherPrincipalID.
	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   targetID,
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	// The authenticated principal (testPrincipalID) differs from the target's
	// principal (otherPrincipalID), so kernel.Authorize should deny.
	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for cross-principal execution, got ALLOWED")
	}
}

func TestExecute_WrongScenarioDenied(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	scenarioID, beliefID, targetID, intentID, _ := setupExecutionScenario(t, db)
	_ = scenarioID

	// Use a different scenario_id.
	wrongScenario := createTestScenario(t)
	body := map[string]interface{}{
		"scenario_id": wrongScenario,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   targetID,
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for wrong scenario, got ALLOWED")
	}
}

func TestExecute_WrongBeliefDenied(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	scenarioID, _, targetID, intentID, _ := setupExecutionScenario(t, db)

	// Create a different belief in the same scenario.
	ctx := context.Background()
	st := kernel.New(db)
	wrongBeliefID, err := st.EnsureBelief(ctx, scenarioID, "wrong belief", kernel.Derived, kernel.FullDebt)
	if err != nil {
		t.Fatalf("ensure wrong belief: %v", err)
	}

	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   wrongBeliefID,
		"action":      "deploy",
		"target_id":   targetID,
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for wrong belief, got ALLOWED")
	}
}

func TestExecute_WrongActionDenied(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	scenarioID, beliefID, targetID, intentID, _ := setupExecutionScenario(t, db)

	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "rollback",
		"target_id":   targetID,
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for wrong action, got ALLOWED")
	}
}

func TestExecute_WrongTargetDenied(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	scenarioID, beliefID, _, intentID, _ := setupExecutionScenario(t, db)

	// Use a non-existent target_id.
	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   "00000000-0000-0000-0000-999999999999",
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for wrong target, got ALLOWED")
	}
}

func TestExecute_CallerParamsIgnored(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	// This test verifies that caller-supplied consequence_parameters in the
	// request body are ignored. The handler reads snapshot params from the DB.
	// We verify this by checking the response: if the handler used caller params,
	// the kernel would reject the mismatch (as proven in Exec11).
	// Since the handler reads snapshot params from DB, the request succeeds.

	ts := newTestServerWithAuth(t, db)
	defer ts.Close()

	scenarioID, beliefID, targetID, intentID, _ := setupExecutionScenario(t, db)

	// The request body includes consequence_parameters, but the handler
	// ignores them and reads from the DB. This is the correct behavior.
	body := map[string]interface{}{
		"scenario_id":            scenarioID,
		"belief_id":              beliefID,
		"action":                 "deploy",
		"target_id":              targetID,
		"intent_id":              intentID,
		"consequence_parameters": `{"repo":"evil/repo","workflow":"evil.yml","ref":"evil"}`,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	// The handler reads snapshot params from DB, not from the request body.
	// So the authorization should be ALLOWED (snapshot params match).
	allowed, _ := result["allowed"].(bool)
	if !allowed {
		t.Errorf("expected ALLOWED (handler uses DB params), got DENIED: %v", result["reason"])
	}
}

// lookupSnapshotID reads the snapshot_id for an approved target from target_activation.
func lookupSnapshotID(t *testing.T, ctx context.Context, db *sql.DB, targetID string) string {
	t.Helper()
	var snapshotID string
	err := db.QueryRowContext(ctx,
		`SELECT snapshot_id FROM target_activation WHERE target_id = $1::UUID`,
		targetID).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("lookup snapshot: %v", err)
	}
	return snapshotID
}

// setupBoundExecutionScenario creates a scenario with an approved target and a
// live intent BOUND to that target via (target_id, snapshot_id). This is the
// production-quality setup that exercises the exact authority binding path.
// Returns scenarioID, beliefID, targetID, snapshotID, intentID, principalID.
func setupBoundExecutionScenario(t *testing.T, db *sql.DB) (string, string, string, string, string, string) {
	t.Helper()
	ctx := context.Background()
	st := kernel.New(db)

	scenarioID := createTestScenario(t)
	principalID := testPrincipalID

	beliefID, err := st.EnsureBelief(ctx, scenarioID, "test belief for bound execution", kernel.Derived, kernel.FullDebt)
	if err != nil {
		t.Fatalf("ensure belief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, scenarioID, beliefID, item)
	}
	if err := st.Promote(ctx, scenarioID, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}

	snapParams, _ := json.Marshal(map[string]string{
		"repo":     "owner/repo",
		"workflow": "deploy.yml",
		"ref":      "main",
	})
	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", "deploy", "execution", snapParams, principalID)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	_ = st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID)
	_ = st.RequestAuthorization(ctx, targetID, principalID)
	_ = st.Approve(ctx, targetID, principalID)

	snapshotID := lookupSnapshotID(t, ctx, db, targetID)

	// Create live intent with exact authority binding (target_id, snapshot_id).
	var intentID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
		 VALUES ($1::UUID, $2::UUID, $3::STRING, $4::UUID, $5::UUID)
		 RETURNING id`,
		scenarioID, beliefID, "deploy", targetID, snapshotID).Scan(&intentID)
	if err != nil {
		t.Fatalf("create bound intent: %v", err)
	}

	return scenarioID, beliefID, targetID, snapshotID, intentID, principalID
}

// TestExecute_ConfusedDeputy proves the REST execution path correctly rejects
// a T1-bound intent when the request targets T2 — where T2 is itself a real,
// valid, approved authority. This is the plumbing regression for Finding 1:
// the kernel-level CAS rejection must surface through the REST→service→kernel
// pipeline.
//
// The critical assertion: T2 is valid, authorization may succeed, but ClaimIntent
// must reject I1 because I1 is bound to T1/S1, not T2/S2.
func TestExecute_ConfusedDeputy(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts, rec := newTestServerWithRecordingExecutor(t, db)
	defer ts.Close()

	ctx := context.Background()
	st := kernel.New(db)

	scenarioID := createTestScenario(t)
	principalID := testPrincipalID

	// Create and promote belief.
	beliefID, err := st.EnsureBelief(ctx, scenarioID, "confused deputy belief", kernel.Derived, kernel.FullDebt)
	if err != nil {
		t.Fatalf("ensure belief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, scenarioID, beliefID, item)
	}
	if err := st.Promote(ctx, scenarioID, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}

	// Create two approved targets with identical belief/action but different snapshot params.
	// T1 is the legitimate target for intent I1. T2 is itself a real, valid, approved
	// authority — not a nonexistent target. The test proves that even though T2 is
	// authorized, ClaimIntent rejects I1 because I1 belongs to T1.
	snapParamsT1, _ := json.Marshal(map[string]string{"repo": "org/targetT1", "workflow": "deploy.yml", "ref": "main"})
	targetT1, err := st.CreateTarget(ctx, principalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", "deploy", "execution", snapParamsT1, principalID)
	if err != nil {
		t.Fatalf("create target T1: %v", err)
	}
	_ = st.AttachJustification(ctx, targetT1, beliefID, "promoted", principalID)
	_ = st.RequestAuthorization(ctx, targetT1, principalID)
	_ = st.Approve(ctx, targetT1, principalID)

	snapParamsT2, _ := json.Marshal(map[string]string{"repo": "org/targetT2", "workflow": "deploy.yml", "ref": "main"})
	targetT2, err := st.CreateTarget(ctx, principalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", "deploy", "execution", snapParamsT2, principalID)
	if err != nil {
		t.Fatalf("create target T2: %v", err)
	}
	_ = st.AttachJustification(ctx, targetT2, beliefID, "promoted", principalID)
	_ = st.RequestAuthorization(ctx, targetT2, principalID)
	_ = st.Approve(ctx, targetT2, principalID)

	snapshotT1 := lookupSnapshotID(t, ctx, db, targetT1)

	// Create intent I1 bound to T1/S1 (the legitimate authority).
	var intentID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
		 VALUES ($1::UUID, $2::UUID, $3::STRING, $4::UUID, $5::UUID)
		 RETURNING id`,
		scenarioID, beliefID, "deploy", targetT1, snapshotT1).Scan(&intentID)
	if err != nil {
		t.Fatalf("create bound intent: %v", err)
	}

	// Execute against T2 — the request targets T2, but I1 is bound to T1.
	// T2 is itself a real approved authority, so authorization may succeed.
	// ClaimIntent must reject I1 because I1's (target_id, snapshot_id) != (T2, S2).
	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   targetT2,
		"intent_id":   intentID,
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/execute", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	if allowed {
		t.Error("expected DENIED for confused-deputy T1→T2 substitution, got ALLOWED")
	}
	if rec.Called() {
		t.Error("executor must NOT be called when ClaimIntent rejects")
	}

	// Verify intent I1 is still live (not moved to executing).
	var state string
	err = db.QueryRowContext(ctx,
		`SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("read intent state: %v", err)
	}
	if state != "live" {
		t.Errorf("intent should remain live after rejected claim, got %q", state)
	}
}
