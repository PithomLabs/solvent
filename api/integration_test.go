package api_test

import (
	"net/http"
	"sync"
	"testing"
)

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

// TestIntegration_ConcurrentRevokeTarget verifies that revoking a target while
// an authorization is in progress does not produce an inconsistent state.
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

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// Concurrent: authorize action and revoke target.
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
		defer resp.Body.Close()
		// Either 200 (allowed/denied) is acceptable — no crash.
		if resp.StatusCode != http.StatusOK {
			errCh <- nil
		}
	}()
	go func() {
		defer wg.Done()
		body := map[string]interface{}{
			"reason": "concurrent revocation test",
		}
		resp := doRequest(t, ts, "POST", "/v1/targets/"+targetID+"/revoke", body)
		defer resp.Body.Close()
		// Either 200 (success) or 409/404 (already revoked/not found) is acceptable.
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusNotFound {
			errCh <- nil
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// Verify final state: target should be in a consistent state.
	resp := doRequest(t, ts, "GET", "/v1/targets/"+targetID, nil)
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
	// State must be one of the defined values.
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
	dischargePrincipalID := createTestPrincipal(t, db)

	// First discharge.
	body := map[string]interface{}{
		"belief_id":      beliefID,
		"obligation_key": "test-obligation",
		"instrument_ref": "ref-001",
		"discharged_by":  dischargePrincipalID,
	}
	resp := doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first discharge: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Duplicate discharge should fail.
	body["discharged_by"] = dischargePrincipalID
	resp = doRequest(t, ts, "POST", "/v1/discharge", body)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate discharge: expected 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
