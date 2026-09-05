package api_test

import (
	"net/http"
	"testing"
)

func TestBelief_EnterAndGet(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)

	// Enter belief.
	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"claim":       "etcd v3.5.x is safe to deploy",
		"claim_type":  "derived",
	}
	resp := doRequest(t, ts, "POST", "/v1/beliefs", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	decodeJSON(t, resp, &created)

	beliefID, ok := created["belief_id"].(string)
	if !ok || beliefID == "" {
		t.Fatalf("expected belief_id, got %v", created["belief_id"])
	}

	// Get belief.
	resp = doRequest(t, ts, "GET", "/v1/beliefs/"+beliefID+"?scenario_id="+scenarioID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var fetched map[string]interface{}
	decodeJSON(t, resp, &fetched)

	if fetched["claim"] != "etcd v3.5.x is safe to deploy" {
		t.Errorf("expected claim, got %v", fetched["claim"])
	}
}

func TestBelief_List(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)

	// Enter two beliefs.
	for _, claim := range []string{"claim 1", "claim 2"} {
		body := map[string]interface{}{
			"scenario_id": scenarioID,
			"claim":       claim,
			"claim_type":  "derived",
		}
		resp := doRequest(t, ts, "POST", "/v1/beliefs", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	// List beliefs.
	resp := doRequest(t, ts, "GET", "/v1/beliefs?scenario_id="+scenarioID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list map[string]interface{}
	decodeJSON(t, resp, &list)

	beliefs, ok := list["beliefs"].([]interface{})
	if !ok || len(beliefs) != 2 {
		t.Errorf("expected 2 beliefs, got %v", list["beliefs"])
	}
}

func TestBelief_InvalidClaimType(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)

	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"claim":       "test claim",
		"claim_type":  "invalid",
	}
	resp := doRequest(t, ts, "POST", "/v1/beliefs", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestBelief_MissingClaim(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)

	body := map[string]interface{}{
		"scenario_id": scenarioID,
		"claim_type":  "derived",
	}
	resp := doRequest(t, ts, "POST", "/v1/beliefs", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
