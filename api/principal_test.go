package api_test

import (
	"net/http"
	"testing"
)

func TestPrincipal_CreateAndGet(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	// Create principal.
	body := map[string]interface{}{
		"principal_type": "service",
		"issuer":         "test-api-issuer",
	}
	resp := doRequest(t, ts, "POST", "/v1/principals", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	decodeJSON(t, resp, &created)

	principalID, ok := created["principal_id"].(string)
	if !ok || principalID == "" {
		t.Fatalf("expected principal_id, got %v", created["principal_id"])
	}

	// Get principal.
	resp = doRequest(t, ts, "GET", "/v1/principals/"+principalID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var fetched map[string]interface{}
	decodeJSON(t, resp, &fetched)

	if fetched["issuer"] != "test-api-issuer" {
		t.Errorf("expected issuer, got %v", fetched["issuer"])
	}
}

func TestPrincipal_List(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	// Create two principals.
	for i := 0; i < 2; i++ {
		body := map[string]interface{}{
			"principal_type": "service",
			"issuer":         "test-issuer",
		}
		resp := doRequest(t, ts, "POST", "/v1/principals", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	// List principals.
	resp := doRequest(t, ts, "GET", "/v1/principals", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list map[string]interface{}
	decodeJSON(t, resp, &list)

	principals, ok := list["principals"].([]interface{})
	if !ok || len(principals) < 2 {
		t.Errorf("expected at least 2 principals, got %v", list["principals"])
	}
}

func TestPrincipal_Revoke(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	// Create principal.
	body := map[string]interface{}{
		"principal_type": "service",
		"issuer":         "test-revocable",
	}
	resp := doRequest(t, ts, "POST", "/v1/principals", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	decodeJSON(t, resp, &created)
	principalID, ok := created["principal_id"].(string)
	if !ok || principalID == "" {
		t.Fatalf("expected principal_id, got %v", created["principal_id"])
	}

	// Revoke principal.
	resp = doRequest(t, ts, "POST", "/v1/principals/"+principalID+"/revoke", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var revoked map[string]interface{}
	decodeJSON(t, resp, &revoked)

	if revoked["revoked_at"] == nil {
		t.Error("expected revoked_at to be set")
	}
}
