package api_test

import (
	"net/http"
	"testing"
)

func TestAPI_RoutesExist(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		body   interface{}
		code   int
	}{
		{"GET", "/v1/beliefs?scenario_id=00000000-0000-0000-0000-000000000000", nil, 200},
		{"POST", "/v1/beliefs", map[string]interface{}{
			"scenario_id": "00000000-0000-0000-0000-000000000000",
			"claim":       "test claim",
			"claim_type":  "derived",
		}, 201},
		{"GET", "/v1/evidence/00000000-0000-0000-0000-000000000000", nil, 404},
		{"GET", "/v1/principals", nil, 200},
		{"GET", "/v1/targets", nil, 200},
		{"POST", "/v1/authorizations/verify", nil, 400},
		{"POST", "/v1/authorizations/action", nil, 400},
		{"POST", "/v1/discharge", nil, 400},
		{"GET", "/v1/activity?scenario_id=00000000-0000-0000-0000-000000000000", nil, 200},
		{"GET", "/v1/ledger?scenario_id=00000000-0000-0000-0000-000000000000", nil, 200},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp := doRequest(t, ts, tt.method, tt.path, tt.body)
			if resp.StatusCode != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}

func TestAPI_Unauthorized(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ts := newTestServer(t, db)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/v1/beliefs?scenario_id=00000000-0000-0000-0000-000000000000", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
