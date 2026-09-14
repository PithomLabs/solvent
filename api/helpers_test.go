package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PithomLabs/solvent/api"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/authority"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDSN points at the per-package test database.
var testDSN = testdb.SuiteDSN("api")

// testDB opens a test database connection. Returns nil if CockroachDB is unavailable.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Skipf("CockroachDB unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("CockroachDB unavailable: %v", err)
	}
	return db
}

// newTestServer creates a new API server with audit logging for tests.
func newTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	auditSvc := audit.New(db)
	server := api.NewServer(db, auditSvc)
	keyMap := map[string]string{
		"test-api-key-12345": testPrincipalID,
	}
	handler := api.AuthMiddleware(keyMap, server.Handler())
	return httptest.NewServer(handler)
}

// newTestServerWithAuth creates a new API server with authority service for execution tests.
func newTestServerWithAuth(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	auditSvc := audit.New(db)
	policySvc := policy.New(db)
	execReg := executor.NewRegistry()
	authSvc := authority.New(db, policySvc, auditSvc, execReg)
	server := api.NewServer(db, auditSvc, api.WithAuthorityService(authSvc))
	keyMap := map[string]string{
		"test-api-key-12345": testPrincipalID,
	}
	handler := api.AuthMiddleware(keyMap, server.Handler())
	return httptest.NewServer(handler)
}

// newTestServerWithRecordingExecutor creates a new API server with authority service
// and a recording executor. Returns the test server and the recording function so
// tests can assert on executor invocation.
func newTestServerWithRecordingExecutor(t *testing.T, db *sql.DB) (*httptest.Server, *executor.RecordingFunc) {
	t.Helper()
	auditSvc := audit.New(db)
	policySvc := policy.New(db)
	execReg := executor.NewRegistry()
	rec := executor.NewRecordingFunc("test_action", "test output")
	execReg.Register("github_trigger_workflow", rec.Func())
	authSvc := authority.New(db, policySvc, auditSvc, execReg)
	server := api.NewServer(db, auditSvc, api.WithAuthorityService(authSvc))
	keyMap := map[string]string{
		"test-api-key-12345": testPrincipalID,
	}
	handler := api.AuthMiddleware(keyMap, server.Handler())
	return httptest.NewServer(handler), rec
}

// validAuth returns a valid Authorization header.
func validAuth() string {
	return "Bearer test-api-key-12345"
}

// doRequest makes an HTTP request and returns the response.
func doRequest(t *testing.T, ts *httptest.Server, method, path string, body interface{}) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.URL+path, reqBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", validAuth())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// decodeJSON decodes the response body into the target.
func decodeJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// createTestScenario returns a unique scenario UUID. Uses nano time to avoid
// collisions across test binary invocations.
func createTestScenario(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("00000000-0000-0000-0000-%012x", time.Now().UnixNano()%0xFFFFFFFFFFFF)
}

// createTestPrincipal creates a test principal and returns its ID.
func createTestPrincipal(t *testing.T, db *sql.DB) string {
	t.Helper()
	id, err := kernel.New(db).CreatePrincipal(context.Background(), "service", "test-issuer")
	if err != nil {
		t.Fatalf("create principal: %v", err)
	}
	return id
}

// createTestTarget creates a test target and returns its ID.
func createTestTarget(t *testing.T, db *sql.DB, principalID string) string {
	t.Helper()
	id, err := kernel.New(db).CreateTarget(context.Background(), principalID,
		"scenario", "test-scenario", "global", "solvent", "execute",
		"execution", []byte("{}"), principalID)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	return id
}

// createTestBelief creates a test belief and returns its ID.
func createTestBelief(t *testing.T, db *sql.DB, scenarioID string) string {
	t.Helper()
	id, err := kernel.New(db).EnterBelief(context.Background(), scenarioID,
		"Test belief for API tests", kernel.Derived, []string{"testDebt"})
	if err != nil {
		t.Fatalf("create belief: %v", err)
	}
	return id
}
