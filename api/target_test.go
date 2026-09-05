package api_test

import (
	"net/http"
	"testing"
)

func TestTarget_CreateAndGet(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	principalID := createTestPrincipal(t, db)

	body := map[string]interface{}{
		"principal_id":           principalID,
		"resource_type":          "scenario",
		"resource_id":            "test-scenario",
		"scope":                  "belief:test-belief",
		"action_namespace":       "solvent",
		"action_name":            "execute",
		"consequence_type":       "execution",
		"consequence_parameters": map[string]interface{}{},
		"created_by":             principalID,
	}
	resp := doRequest(t, ts, "POST", "/v1/targets", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	decodeJSON(t, resp, &created)

	targetID, ok := created["target_id"].(string)
	if !ok || targetID == "" {
		t.Fatalf("expected target_id, got %v", created["target_id"])
	}

	// Get target.
	resp = doRequest(t, ts, "GET", "/v1/targets/"+targetID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var fetched map[string]interface{}
	decodeJSON(t, resp, &fetched)

	if fetched["resource_type"] != "scenario" {
		t.Errorf("expected resource_type scenario, got %v", fetched["resource_type"])
	}
}

func TestTarget_List(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	principalID := createTestPrincipal(t, db)

	for i := 0; i < 2; i++ {
		body := map[string]interface{}{
			"principal_id":           principalID,
			"resource_type":          "scenario",
			"resource_id":            "test-scenario",
			"scope":                  "belief:test-belief",
			"action_namespace":       "solvent",
			"action_name":            "execute",
			"consequence_type":       "execution",
			"consequence_parameters": map[string]interface{}{},
			"created_by":             principalID,
		}
		resp := doRequest(t, ts, "POST", "/v1/targets", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp := doRequest(t, ts, "GET", "/v1/targets", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list map[string]interface{}
	decodeJSON(t, resp, &list)

	targets, ok := list["targets"].([]interface{})
	if !ok || len(targets) < 2 {
		t.Errorf("expected at least 2 targets, got %v", list["targets"])
	}
}
