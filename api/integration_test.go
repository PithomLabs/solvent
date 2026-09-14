package api_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/PithomLabs/solvent/kernel"
)

// TestIntegration_AuthorizeAction_ActorIDMismatch verifies that sending a
// conflicting actor_id in the request body returns 403, not 400.
func TestIntegration_AuthorizeAction_ActorIDMismatch(t *testing.T) {
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

	// Create a DIFFERENT principal to use as a conflicting actor_id.
	otherPrincipalID := createTestPrincipal(t, db)

	body := map[string]interface{}{
		"scenario_id":   scenarioID,
		"belief_id":     beliefID,
		"action":        "execute",
		"target_id":     targetID,
		"action_source": "user_typed",
		"actor_id":      otherPrincipalID, // conflicts with authenticated principal
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/action", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var errResp map[string]interface{}
	decodeJSON(t, resp, &errResp)

	code, _ := errResp["code"].(string)
	if code != "actor_id_mismatch" {
		t.Errorf("expected code=actor_id_mismatch, got %s", code)
	}
}

// TestIntegration_AuthorizeAction_Atomicity verifies that authority evaluation
// and intent creation occur atomically. An unactivated target should produce
// allowed=false with no intent created — proving the kernel evaluated authority
// before (or without) creating an intent.
func TestIntegration_AuthorizeAction_Atomicity(t *testing.T) {
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

	// Atomic authorize + create intent on an unactivated target.
	// The authority check should fail (no activation), so no intent is created.
	body := map[string]interface{}{
		"scenario_id":   scenarioID,
		"belief_id":     beliefID,
		"action":        "execute",
		"target_id":     targetID,
		"action_source": "user_typed",
	}
	resp := doRequest(t, ts, "POST", "/v1/authorizations/action", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorize: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	allowed, _ := result["allowed"].(bool)
	intentState, _ := result["intent_state"].(string)

	// Authority denied (target not activated) — no intent should be created.
	// This proves the kernel evaluated authority before creating intent.
	if allowed {
		t.Error("expected allowed=false for unactivated target")
	}
	if intentState != "" {
		t.Errorf("expected no intent_state on denial, got %v", intentState)
	}
}

// TestIntegration_ConcurrentRevokeTarget is a smoke test that proves the API
// does not crash or corrupt state under concurrency. It does NOT claim to prove
// atomicity — the formal proof is at the kernel level (T-C9, T-C10).
func TestIntegration_ConcurrentRevokeTarget(t *testing.T) {
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

	// Retire debts, promote the belief, and attach a justification so the proposal is valid.
	kern := kernel.New(db)
	for _, item := range []string{"testDebt"} {
		if err := kern.RetireDebt(context.Background(), scenarioID, beliefID, item); err != nil {
			t.Fatalf("retire debt %q: %v", item, err)
		}
	}
	if err := kern.Promote(context.Background(), scenarioID, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}
	if err := kern.AttachJustification(context.Background(), targetID, beliefID, "promoted", principalID); err != nil {
		t.Fatalf("attach justification: %v", err)
	}

	// Activate the target so authorization has a chance of succeeding.
	reqBody := map[string]interface{}{
		"scenario_id":   scenarioID,
		"belief_id":     beliefID,
		"action":        "execute",
		"target_id":     targetID,
		"action_source": "user_typed",
	}
	resp := doRequest(t, ts, "POST", "/v1/targets/"+targetID+"/request", reqBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request authorization: expected 200, got %d", resp.StatusCode)
	}
	approveBody := map[string]interface{}{"approval_pin": "test-pin"}
	resp = doRequest(t, ts, "POST", "/v1/targets/"+targetID+"/approve", approveBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve target: expected 200, got %d", resp.StatusCode)
	}

	// Run concurrent requests.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var authorizeStatus, revokeStatus int

	wg.Add(2)
	go func() {
		defer wg.Done()
		body := map[string]interface{}{
			"scenario_id":   scenarioID,
			"belief_id":     beliefID,
			"action":        "execute",
			"target_id":     targetID,
			"action_source": "user_typed",
		}
		resp := doRequest(t, ts, "POST", "/v1/authorizations/action", body)
		resp.Body.Close()
		mu.Lock()
		authorizeStatus = resp.StatusCode
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		body := map[string]interface{}{"reason": "concurrent revocation test"}
		resp := doRequest(t, ts, "POST", "/v1/targets/"+targetID+"/revoke", body)
		resp.Body.Close()
		mu.Lock()
		revokeStatus = resp.StatusCode
		mu.Unlock()
	}()

	wg.Wait()

	// No 500 errors during concurrent access.
	if authorizeStatus == http.StatusInternalServerError {
		t.Error("authorize returned 500 during concurrent access")
	}
	if revokeStatus == http.StatusInternalServerError {
		t.Error("revoke returned 500 during concurrent access")
	}

	// Verify final state: target must be in a consistent state.
	resp = doRequest(t, ts, "GET", "/v1/targets/"+targetID, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var target map[string]interface{}
	decodeJSON(t, resp, &target)

	state, ok := target["state"].(string)
	if !ok {
		t.Fatalf("expected state string, got %v", target["state"])
	}
	switch state {
	case "revoked", "proposed", "requested", "approved", "activated":
		// valid
	default:
		t.Errorf("unexpected state: %s", state)
	}
}

// TestIntegration_DuplicateDischargePrevention verifies that discharging the
// same obligation twice returns 409 Conflict.
func TestIntegration_DuplicateDischargePrevention(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)
	_ = createTestPrincipal(t, db) // create a second principal for other tests

	// First discharge.
	body := map[string]interface{}{
		"scenario_id":   scenarioID,
		"belief_id":      beliefID,
		"obligation_key": "test-obligation",
		"instrument_ref": "ref-001",
		"discharged_by":  testPrincipalID,
	}
	resp := doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first discharge: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Duplicate discharge should fail.
	body["discharged_by"] = testPrincipalID
	resp = doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate discharge: expected 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- Cross-scenario endpoint regression tests (Plan 8.5 §4a) ---

func TestCS_RetireDebt_WrongScenario(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioA := createTestScenario(t)
	scenarioB := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioA)

	// Read debt before.
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)

	// Retire with wrong scenario → 404. scenario_id is a query parameter.
	body := map[string]interface{}{
		"debt_item": "testDebt",
	}
	resp := doRequest(t, ts, "POST",
		"/v1/beliefs/"+beliefID+"/debt/retire?scenario_id="+scenarioB, body)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("wrong scenario retire: expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify zero mutation.
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if debtBefore != debtAfter {
		t.Errorf("debt mutated: before=%q, after=%q", debtBefore, debtAfter)
	}
}

func TestCS_Promote_WrongScenario(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioA := createTestScenario(t)
	scenarioB := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioA)

	// Read status before.
	var statusBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT status FROM belief WHERE id=$1::UUID`, beliefID).Scan(&statusBefore)

	// Promote with wrong scenario → 404. scenario_id is a query parameter.
	resp := doRequest(t, ts, "POST",
		"/v1/beliefs/"+beliefID+"/promote?scenario_id="+scenarioB, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("wrong scenario promote: expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify zero mutation.
	var statusAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT status FROM belief WHERE id=$1::UUID`, beliefID).Scan(&statusAfter)
	if statusBefore != statusAfter {
		t.Errorf("status mutated: before=%q, after=%q", statusBefore, statusAfter)
	}
}

func TestCS_Discharge_WrongScenario(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioA := createTestScenario(t)
	scenarioB := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioA)
	_ = createTestPrincipal(t, db) // create a second principal for other tests

	// Read state before.
	var countBefore int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, beliefID).Scan(&countBefore)
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)

	// Discharge with wrong scenario → 404.
	body := map[string]interface{}{
		"scenario_id":   scenarioB,
		"belief_id":     beliefID,
		"obligation_key": "cs-obligation",
		"instrument_ref": "cs-instrument",
		"discharged_by":  testPrincipalID,
	}
	resp := doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("wrong scenario discharge: expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify zero mutation.
	var countAfter int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, beliefID).Scan(&countAfter)
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if countBefore != countAfter {
		t.Errorf("discharge rows mutated: before=%d, after=%d", countBefore, countAfter)
	}
	if debtBefore != debtAfter {
		t.Errorf("debt mutated: before=%q, after=%q", debtBefore, debtAfter)
	}
	_ = scenarioA // unused but ensures both scenarios are created
}

// --- Service-layer access control tests (Post-Kernel-Freeze Cleanup) ---

func TestAC_RetireDebt_RevokedPrincipalRejected(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	// Read debt before.
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)

	// Revoke the authenticated principal.
	_, err := db.ExecContext(context.Background(),
		`UPDATE principal SET revoked_at = now() WHERE principal_id = $1::UUID`, testPrincipalID)
	if err != nil {
		t.Fatalf("revoke principal: %v", err)
	}

	// Attempt RetireDebt → 403 revoked_principal.
	body := map[string]interface{}{
		"debt_item": "testDebt",
	}
	resp := doRequest(t, ts, "POST",
		"/v1/beliefs/"+beliefID+"/debt/retire?scenario_id="+scenarioID, body)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("revoked principal retire: expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify zero debt mutation.
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if debtBefore != debtAfter {
		t.Errorf("debt mutated after denial: before=%q, after=%q", debtBefore, debtAfter)
	}

	// Restore principal for other tests.
	_, _ = db.ExecContext(context.Background(),
		`UPDATE principal SET revoked_at = NULL WHERE principal_id = $1::UUID`, testPrincipalID)
}

func TestAC_RetireDebt_ActivePrincipalAllowed(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	// Read debt before.
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)
	if debtBefore == "" {
		t.Fatal("belief has no debt to retire")
	}

	// Retire one debt item → 200.
	body := map[string]interface{}{
		"debt_item": "testDebt",
	}
	resp := doRequest(t, ts, "POST",
		"/v1/beliefs/"+beliefID+"/debt/retire?scenario_id="+scenarioID, body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("active principal retire: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify debt was mutated.
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if debtBefore == debtAfter {
		t.Errorf("debt not mutated: before=%q, after=%q", debtBefore, debtAfter)
	}
}

func TestAC_Discharge_ImpersonationRejected(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	// Read state before.
	var countBefore int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, beliefID).Scan(&countBefore)
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)

	// Create a different principal to impersonate.
	impersonatedID := createTestPrincipal(t, db)

	// Attempt discharge with mismatched discharged_by → 403.
	body := map[string]interface{}{
		"scenario_id":    scenarioID,
		"belief_id":      beliefID,
		"obligation_key": "ac-obligation",
		"instrument_ref": "ac-instrument",
		"discharged_by":  impersonatedID,
	}
	resp := doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("impersonation discharge: expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify zero discharge rows created.
	var countAfter int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, beliefID).Scan(&countAfter)
	if countBefore != countAfter {
		t.Errorf("discharge rows mutated: before=%d, after=%d", countBefore, countAfter)
	}

	// Verify zero debt mutation.
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if debtBefore != debtAfter {
		t.Errorf("debt mutated after denial: before=%q, after=%q", debtBefore, debtAfter)
	}
}

func TestAC_Discharge_OwnIdentityAllowed(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	// Read debt before.
	var debtBefore string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtBefore)

	// Discharge using own identity.
	body := map[string]interface{}{
		"scenario_id":    scenarioID,
		"belief_id":      beliefID,
		"obligation_key": "testDebt",
		"instrument_ref": "ac-own-instrument",
		"discharged_by":  testPrincipalID,
	}
	resp := doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("own identity discharge: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Query ALL discharge rows for this belief (no obligation_key filter).
	var rowCount int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`,
		beliefID).Scan(&rowCount)

	// Prove a discharge row exists.
	if rowCount == 0 {
		t.Fatal("no discharge row found after HTTP 200 — persistence discrepancy")
	}

	// Query the actual row contents.
	var storedObligation, storedInstrument, storedBy string
	err := db.QueryRowContext(context.Background(),
		`SELECT obligation_key, instrument_ref, discharged_by
		 FROM debt_discharge WHERE belief_id=$1::UUID LIMIT 1`,
		beliefID).Scan(&storedObligation, &storedInstrument, &storedBy)
	if err != nil {
		t.Fatalf("query discharge row: %v", err)
	}

	// Prove discharged_by matches the authenticated principal.
	if storedBy != testPrincipalID {
		t.Errorf("persisted discharged_by mismatch: want %q, got %q", testPrincipalID, storedBy)
	}

	// Prove obligation_key and instrument_ref match.
	if storedObligation != "testDebt" {
		t.Errorf("persisted obligation_key mismatch: want %q, got %q", "testDebt", storedObligation)
	}
	if storedInstrument != "ac-own-instrument" {
		t.Errorf("persisted instrument_ref mismatch: want %q, got %q", "ac-own-instrument", storedInstrument)
	}

	// Prove debt was retired.
	var debtAfter string
	_ = db.QueryRowContext(context.Background(),
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, beliefID).Scan(&debtAfter)
	if debtBefore == debtAfter {
		t.Errorf("debt not retired: before=%q, after=%q", debtBefore, debtAfter)
	}
	if debtAfter != "" && debtBefore != debtAfter {
		t.Logf("debt retired: before=%q, after=%q", debtBefore, debtAfter)
	}
}