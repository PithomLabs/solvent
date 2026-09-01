package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var sharedDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.DSN()
	schemaPaths := []string{
		"../../db/001_schema.sql",
		"../../db/002_corpus.sql",
		"../../db/003_wizard.sql",
		"../../db/004_debt_vocabulary.sql",
		"../../db/005_authority_mvp.sql",
		"../../db/006_authority_justification_cascade.sql",
	}
	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		panic("reset: " + err.Error())
	}
	var err error
	sharedDB, err = testdb.Open(dsn)
	if err != nil {
		panic("open: " + err.Error())
	}
	defer sharedDB.Close()
	os.Exit(m.Run())
}

// promoteTestBelief creates a belief, retires all debt, and promotes it.
func promoteTestBelief(t *testing.T, ctx context.Context, st *kernel.Store, sc, claim string) string {
	t.Helper()
	bid, err := st.EnsureBelief(ctx, sc, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("setup (ensure belief): %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, bid, item)
	}
	if err := st.Promote(ctx, bid); err != nil {
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
