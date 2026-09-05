package api_test

import (
	"net/http"
	"testing"
)

func TestEvidence_AddAndGet(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	// Add evidence.
	body := map[string]interface{}{
		"scenario_id":      scenarioID,
		"belief_id":        beliefID,
		"provenance_class": "external_feed",
		"source_url":       "https://example.com/issue/123",
		"content_sha256":   "abc123def456",
	}
	resp := doRequest(t, ts, "POST", "/v1/evidence", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify belief has evidence via explain.
	resp = doRequest(t, ts, "GET", "/v1/beliefs/"+beliefID+"/explain?scenario_id="+scenarioID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var explain map[string]interface{}
	decodeJSON(t, resp, &explain)
}

func TestEvidence_InvalidProvenanceClass(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)
	beliefID := createTestBelief(t, db, scenarioID)

	body := map[string]interface{}{
		"scenario_id":      scenarioID,
		"belief_id":        beliefID,
		"provenance_class": "invalid_class",
		"content_sha256":   "abc123def456",
	}
	resp := doRequest(t, ts, "POST", "/v1/evidence", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestEvidence_MissingBeliefID(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	scenarioID := createTestScenario(t)

	body := map[string]interface{}{
		"scenario_id":      scenarioID,
		"provenance_class": "external_feed",
		"content_sha256":   "abc123def456",
	}
	resp := doRequest(t, ts, "POST", "/v1/evidence", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
