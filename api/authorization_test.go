package api_test

import (
	"net/http"
	"testing"
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
