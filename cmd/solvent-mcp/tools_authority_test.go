package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/authority"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/ledger"
	"github.com/PithomLabs/solvent/service/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var sharedDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.SuiteDSN("mcp")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	schemaPaths := []string{
		"../../db/001_schema.sql",
		"../../db/002_corpus.sql",
		"../../db/003_wizard.sql",
		"../../db/004_debt_vocabulary.sql",
		"../../db/005_authority_mvp.sql",
		"../../db/006_authority_justification_cascade.sql",
		"../../db/007_service_tables.sql",
		"../../db/008_executing_state.sql",
		"../../db/009_exact_authority_binding.sql",
	}
	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "solvent-mcp cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	var err error
	sharedDB, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "solvent-mcp cannot start: open: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	pol := policy.New(sharedDB)
	aud := audit.New(sharedDB)
	reg := executor.NewRegistry()
	authSvc = authority.New(sharedDB, pol, aud, reg)
	auditSvc = aud
	ledgerSvc = ledger.New(sharedDB, aud)

	code := m.Run()

	_ = sharedDB.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// promoteTestBelief creates a belief, retires all debt, and promotes it.
func promoteTestBelief(t *testing.T, ctx context.Context, st *kernel.Store, sc, claim string) string {
	t.Helper()
	bid, err := st.EnsureBelief(ctx, sc, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("setup (ensure belief): %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, sc, bid, item)
	}
	if err := st.Promote(ctx, sc, bid); err != nil {
		t.Fatalf("setup (promote belief): %v", err)
	}
	return bid
}

// MT-1: CreateTarget maps all fields correctly.
func TestMCPHandler_CreateTarget(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	pid, err := st.CreatePrincipal(ctx, "agent", "test-issuer")
	if err != nil {
		t.Fatal(err)
	}

	args := map[string]interface{}{
		"principal_id":           pid,
		"resource_type":          "service",
		"resource_id":            "svc-1",
		"scope":                  "deploy",
		"action_namespace":       "compute",
		"action_name":            "restart",
		"consequence_type":       "downtime",
		"consequence_parameters": `{"env":"prod"}`,
		"created_by":             pid,
	}

	result, err := handleSolventCreateTarget(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned MCP error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["target_id"] == nil || resp["target_id"] == "" {
		t.Error("target_id not returned")
	}
	if resp["resource_type"] != "service" {
		t.Errorf("resource_type = %v, want service", resp["resource_type"])
	}
	if resp["scope"] != "deploy" {
		t.Errorf("scope = %v, want deploy", resp["scope"])
	}
}

// MT-2: Approve maps approved_by correctly, returns success.
func TestMCPHandler_Approve(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	pid, err := st.CreatePrincipal(ctx, "agent", "test-issuer")
	if err != nil {
		t.Fatal(err)
	}
	bid := promoteTestBelief(t, ctx, st, "00000000-0000-0000-0000-000000000001", "test belief")

	tid, _ := st.CreateTarget(ctx, pid, "service", "svc-1", "deploy", "compute", "restart", "downtime", []byte(`{"env":"prod"}`), pid)
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)

	args := map[string]interface{}{
		"target_id":   tid,
		"approved_by": pid,
	}

	result, err := handleSolventApprove(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned MCP error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)

	if resp["approved"] != true {
		t.Errorf("approved = %v, want true", resp["approved"])
	}
	if resp["target_id"] != tid {
		t.Errorf("target_id = %v, want %s", resp["target_id"], tid)
	}
}

// MT-3: Authorize maps tuple correctly, returns Allowed/Reason.
func TestMCPHandler_Authorize(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	pid, _ := st.CreatePrincipal(ctx, "agent", "test-issuer")
	bid := promoteTestBelief(t, ctx, st, "00000000-0000-0000-0000-000000000001", "test belief")

	tid, _ := st.CreateTarget(ctx, pid, "service", "svc-1", "deploy", "compute", "restart", "downtime", []byte(`{"env":"prod"}`), pid)
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)
	_ = st.Approve(ctx, tid, pid)

	args := map[string]interface{}{
		"target_id":              tid,
		"principal_id":           pid,
		"resource_type":          "service",
		"resource_id":            "svc-1",
		"scope":                  "deploy",
		"action_namespace":       "compute",
		"action_name":            "restart",
		"consequence_type":       "downtime",
		"consequence_parameters": `{"env":"prod"}`,
	}

	result, err := handleSolventAuthorize(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned MCP error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)

	if resp["allowed"] != true {
		t.Errorf("allowed = %v, reason = %v", resp["allowed"], resp["reason"])
	}
}

// MT-4: Kernel denial becomes MCP denial result (not infrastructure error).
func TestMCPHandler_KernelDenial(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	pid, _ := st.CreatePrincipal(ctx, "agent", "test-issuer")
	bid := promoteTestBelief(t, ctx, st, "00000000-0000-0000-0000-000000000001", "test belief")

	tid, _ := st.CreateTarget(ctx, pid, "service", "svc-1", "deploy", "compute", "restart", "downtime", []byte(`{"env":"prod"}`), pid)
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)
	_ = st.Approve(ctx, tid, pid)

	// Revoke principal, then try to approve another target (should fail).
	_ = st.RevokePrincipal(ctx, pid)

	tid2, _ := st.CreateTarget(ctx, pid, "service", "svc-2", "deploy", "compute", "restart", "downtime", []byte(`{"env":"prod"}`), pid)
	_ = st.AttachJustification(ctx, tid2, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid2, pid)

	args := map[string]interface{}{
		"target_id":   tid2,
		"approved_by": pid,
	}

	result, err := handleSolventApprove(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned infrastructure error: %v", err)
	}

	// Should be an MCP error (IsError=true) but not an infrastructure error.
	if !result.IsError {
		t.Fatal("expected MCP error for revoked approver, got success")
	}

	// The response should contain the kernel error, not a handler error.
	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)
	if resp["error"] != true {
		t.Error("expected error=true in response")
	}
}

// MT-5: Kernel infrastructure error becomes tool error (not panic).
func TestMCPHandler_KernelInfraError(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	// Try to approve a non-existent target.
	args := map[string]interface{}{
		"target_id":   "00000000-0000-0000-0000-999999999999",
		"approved_by": "00000000-0000-0000-0000-999999999999",
	}

	result, err := handleSolventApprove(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error (should return MCP error): %v", err)
	}

	if !result.IsError {
		t.Fatal("expected MCP error for non-existent target")
	}

	// Should contain the kernel error message.
	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)
	if resp["error"] != true {
		t.Error("expected error=true in response")
	}
}

// MT-6: RequestAuthorization on activated target returns ErrAlreadyActivated.
func TestMCPHandler_RequestAuthorizationOnActivated(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	pid, err := st.CreatePrincipal(ctx, "agent", "test-issuer")
	if err != nil {
		t.Fatal(err)
	}
	bid := promoteTestBelief(t, ctx, st, "00000000-0000-0000-0000-000000000001", "test belief for MT-6")

	tid, _ := st.CreateTarget(ctx, pid, "service", "svc-1", "deploy", "compute", "restart", "downtime", []byte(`{"env":"prod"}`), pid)
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)
	_ = st.Approve(ctx, tid, pid)

	args := map[string]interface{}{
		"target_id":    tid,
		"requested_by": pid,
	}

	result, err := handleSolventRequestAuthorization(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected MCP error for activated target")
	}

	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)
	if resp["error"] != true {
		t.Error("expected error=true in response")
	}
}

// setupAuthorityState creates the minimum authority state needed for
// handleSolventAuthorizeAction: a principal, a promoted belief, and an
// approved target with justification linking the belief.
func setupAuthorityState(t *testing.T, ctx context.Context, st *kernel.Store, scenarioID string) (actorID, targetID, beliefID string) {
	t.Helper()

	pid, err := st.CreatePrincipal(ctx, "agent", "test-authorize-action")
	if err != nil {
		t.Fatalf("setup (create principal): %v", err)
	}
	actorID = pid

	bid := promoteTestBelief(t, ctx, st, scenarioID, "test belief for authorize action")
	beliefID = bid

	// Create target matching PrepareForAction's tuple construction:
	//   ResourceType = "scenario", ResourceID = scenarioID,
	//   Scope = "belief:"+beliefID, ActionNamespace = "solvent",
	//   ActionName = "deploy etcd v3.5.28", ConsequenceType = "execution"
	tid, err := st.CreateTarget(ctx, pid, "scenario", scenarioID, "belief:"+bid, "solvent", "deploy etcd v3.5.28", "execution", []byte("{}"), pid)
	if err != nil {
		t.Fatalf("setup (create target): %v", err)
	}
	targetID = tid

	if err := st.AttachJustification(ctx, tid, bid, "promoted", pid); err != nil {
		t.Fatalf("setup (attach justification): %v", err)
	}
	if err := st.RequestAuthorization(ctx, tid, pid); err != nil {
		t.Fatalf("setup (request authorization): %v", err)
	}
	if err := st.Approve(ctx, tid, pid); err != nil {
		t.Fatalf("setup (approve): %v", err)
	}

	return actorID, targetID, beliefID
}

// initAuthSvc sets the package-level authSvc global for tests that need
// authority verification. Must be called before the handler under test.
func initAuthSvc(t *testing.T, db *sql.DB) {
	t.Helper()
	pol := policy.New(db)
	aud := audit.New(db)
	reg := executor.NewRegistry()
	authSvc = authority.New(db, pol, aud, reg)
	t.Cleanup(func() { authSvc = nil })
}

// TestAuthorizeAction_MissingTargetID proves that omitting target_id
// rejects the request before any intent is created.
func TestAuthorizeAction_MissingTargetID(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)
	initAuthSvc(t, db)

	scenarioID := "00000000-0000-0000-0000-000000000001"
	actorID, _, beliefID := setupAuthorityState(t, ctx, st, scenarioID)

	args := map[string]interface{}{
		"scenario":      "track1",
		"belief_id":     beliefID,
		"action":        "deploy etcd v3.5.28",
		"action_source": "user_typed",
		"actor_id":      actorID,
		// target_id intentionally omitted
	}

	result, err := handleSolventAuthorizeAction(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected rejection when target_id is missing")
	}

	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)
	if resp["error"] != true {
		t.Errorf("expected error=true, got: %v", resp)
	}
}

// TestAuthorizeAction_MissingActorID proves that omitting actor_id
// rejects the request before any intent is created.
func TestAuthorizeAction_MissingActorID(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)
	initAuthSvc(t, db)

	scenarioID := "00000000-0000-0000-0000-000000000001"
	_, targetID, beliefID := setupAuthorityState(t, ctx, st, scenarioID)

	args := map[string]interface{}{
		"scenario":      "track1",
		"belief_id":     beliefID,
		"action":        "deploy etcd v3.5.28",
		"action_source": "user_typed",
		"target_id":     targetID,
		// actor_id intentionally omitted
	}

	result, err := handleSolventAuthorizeAction(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected rejection when actor_id is missing")
	}

	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp)
	if resp["error"] != true {
		t.Errorf("expected error=true, got: %v", resp)
	}
}

// TestAuthorizeAction_MalformedTargetID proves that a non-UUID target_id
// is rejected before any intent is created.
func TestAuthorizeAction_MalformedTargetID(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)
	initAuthSvc(t, db)

	scenarioID := "00000000-0000-0000-0000-000000000001"
	actorID, _, beliefID := setupAuthorityState(t, ctx, st, scenarioID)

	args := map[string]interface{}{
		"scenario":      "track1",
		"belief_id":     beliefID,
		"action":        "deploy etcd v3.5.28",
		"action_source": "user_typed",
		"target_id":     "not-a-valid-uuid",
		"actor_id":      actorID,
	}

	result, err := handleSolventAuthorizeAction(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected rejection for malformed target_id")
	}
}

// TestAuthorizeAction_MalformedActorID proves that a non-UUID actor_id
// is rejected before any intent is created.
func TestAuthorizeAction_MalformedActorID(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)
	initAuthSvc(t, db)

	scenarioID := "00000000-0000-0000-0000-000000000001"
	_, targetID, beliefID := setupAuthorityState(t, ctx, st, scenarioID)

	args := map[string]interface{}{
		"scenario":      "track1",
		"belief_id":     beliefID,
		"action":        "deploy etcd v3.5.28",
		"action_source": "user_typed",
		"target_id":     targetID,
		"actor_id":      "not-a-valid-uuid",
	}

	result, err := handleSolventAuthorizeAction(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected rejection for malformed actor_id")
	}
}

// TestAuthorizeAction_ValidArgs proves that valid target_id + actor_id
// preserves existing authorization behavior.
func TestAuthorizeAction_ValidArgs(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)
	initAuthSvc(t, db)

	scenarioID := "00000000-0000-0000-0000-000000000001"
	actorID, targetID, beliefID := setupAuthorityState(t, ctx, st, scenarioID)

	args := map[string]interface{}{
		"scenario":      "track1",
		"belief_id":     beliefID,
		"action":        "deploy etcd v3.5.28",
		"action_source": "user_typed",
		"target_id":     targetID,
		"actor_id":      actorID,
	}

	result, err := handleSolventAuthorizeAction(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned MCP error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Unwrap envelope: {"result": {...}, "audit": {...}}
	inner, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result envelope, got: %v", resp)
	}
	if inner["intent_state"] != "live" {
		t.Errorf("intent_state = %v, want live", inner["intent_state"])
	}
	if inner["belief_id"] != beliefID {
		t.Errorf("belief_id = %v, want %s", inner["belief_id"], beliefID)
	}
}

// --- Execution Tool Tests ---

// TestMCPHandler_Execute_RequiresAuthSvc: verify error when authSvc is nil.
func TestMCPHandler_Execute_RequiresAuthSvc(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	// Ensure authSvc is nil.
	authSvc = nil

	args := map[string]interface{}{
		"scenario":  "track1",
		"belief_id": "00000000-0000-0000-0000-000000000099",
		"action":    "deploy",
		"target_id": "00000000-0000-0000-0000-000000000098",
		"intent_id": "00000000-0000-0000-0000-000000000097",
	}

	result, err := handleSolventExecute(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error when authSvc is nil")
	}
}

// TestMCPHandler_Execute_InvalidScenario: verify error for unknown scenario.
func TestMCPHandler_Execute_InvalidScenario(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	// Ensure authSvc is set.
	if authSvc == nil {
		t.Skip("authority service not initialized")
	}

	args := map[string]interface{}{
		"scenario":  "nonexistent",
		"belief_id": "00000000-0000-0000-0000-000000000099",
		"action":    "deploy",
		"target_id": "00000000-0000-0000-0000-000000000098",
		"intent_id": "00000000-0000-0000-0000-000000000097",
	}

	result, err := handleSolventExecute(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error for unknown scenario")
	}
}

// TestMCPHandler_Execute_MissingRequiredFields: verify error for missing fields.
func TestMCPHandler_Execute_MissingRequiredFields(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	if authSvc == nil {
		t.Skip("authority service not initialized")
	}

	// Missing belief_id.
	args := map[string]interface{}{
		"scenario":  "track1",
		"action":    "deploy",
		"target_id": "00000000-0000-0000-0000-000000000098",
		"intent_id": "00000000-0000-0000-0000-000000000097",
	}
	result, err := handleSolventExecute(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error for missing belief_id")
	}
}

// TestMCPHandler_Execute_InvalidBelief: verify error for non-existent belief in scenario.
func TestMCPHandler_Execute_InvalidBelief(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	if authSvc == nil {
		t.Skip("authority service not initialized")
	}

	args := map[string]interface{}{
		"scenario":  "track1",
		"belief_id": "00000000-0000-0000-0000-000000000099",
		"action":    "deploy",
		"target_id": "00000000-0000-0000-0000-000000000098",
		"intent_id": "00000000-0000-0000-0000-000000000097",
	}
	result, err := handleSolventExecute(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error for non-existent belief")
	}
}

// TestMCPHandler_Activity_RequiresAuditSvc: verify error when auditSvc is nil.
func TestMCPHandler_Activity_RequiresAuditSvc(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	// Save and restore.
	origAudit := auditSvc
	defer func() { auditSvc = origAudit }()
	auditSvc = nil

	args := map[string]interface{}{
		"scenario": "track1",
	}
	result, err := handleSolventActivity(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error when auditSvc is nil")
	}
}

// TestMCPHandler_Activity_InvalidScenario: verify error for unknown scenario.
func TestMCPHandler_Activity_InvalidScenario(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	if auditSvc == nil {
		t.Skip("audit service not initialized")
	}

	args := map[string]interface{}{
		"scenario": "nonexistent",
	}
	result, err := handleSolventActivity(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error for unknown scenario")
	}
}

// TestMCPHandler_Activity_Success: verify activity entries are returned.
func TestMCPHandler_Activity_Success(t *testing.T) {
	db := sharedDB
	ctx := context.Background()

	if auditSvc == nil {
		t.Skip("audit service not initialized")
	}

	args := map[string]interface{}{
		"scenario": "track1",
		"limit":    float64(10),
	}
	result, err := handleSolventActivity(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned MCP error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	activities, ok := resp["activities"].([]interface{})
	if !ok {
		t.Fatalf("expected activities array, got: %v", resp)
	}
	t.Logf("activity count: %d", len(activities))
}
