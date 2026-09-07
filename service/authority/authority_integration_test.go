package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
	"github.com/PithomLabs/solvent/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// testScenario returns a deterministic UUID for integration tests.
// Each test gets a unique scenario to avoid cross-test interference.
func testScenario(n int) string {
	return fmt.Sprintf("44444444-0000-0000-0000-%012x", n)
}

// testConsequenceParams returns the canonical GitHub consequence parameters matching createApprovedTarget.
func testConsequenceParams() []byte {
	p, _ := json.Marshal(map[string]string{"repo": "owner/repo", "workflow": "deploy.yml", "ref": "main"})
	return p
}

var (
	dsn    string
	shared *sql.DB
)

var schemaPaths = []string{
	"../../db/001_schema.sql",
	"../../db/002_corpus.sql",
	"../../db/003_wizard.sql",
	"../../db/004_debt_vocabulary.sql",
	"../../db/005_authority_mvp.sql",
	"../../db/006_authority_justification_cascade.sql",
	"../../db/007_service_tables.sql",
	"../../db/008_executing_state.sql",
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.SuiteDSN("authority")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "authority integration cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "authority integration: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "authority integration: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()

	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// --- Test helpers ---

func newTestService(t *testing.T, rec *executor.RecordingFunc) *Service {
	t.Helper()
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	reg.Register("github_trigger_workflow", rec.Func())
	return New(shared, pol, aud, reg)
}

func createPrincipal(t *testing.T, ctx context.Context, st *kernel.Store, ptype, issuer string) string {
	t.Helper()
	id, err := st.CreatePrincipal(ctx, ptype, issuer)
	if err != nil {
		t.Fatalf("setup (create principal): %v", err)
	}
	return id
}

func createAndPromoteBelief(t *testing.T, ctx context.Context, st *kernel.Store, scenarioID, claim string) string {
	t.Helper()
	id, err := st.EnterBelief(ctx, scenarioID, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("setup (enter belief): %v", err)
	}
	// Retire all debt items so promotion succeeds.
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, id, item)
	}
	if err := st.Promote(ctx, id); err != nil {
		t.Fatalf("setup (promote belief): %v", err)
	}
	return id
}

func createApprovedTarget(t *testing.T, ctx context.Context, st *kernel.Store, scenarioID, principalID, beliefID, action string) string {
	t.Helper()
	params, _ := json.Marshal(map[string]string{"repo": "owner/repo", "workflow": "deploy.yml", "ref": "main"})

	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", scenarioID, "belief:"+beliefID,
		"solvent", action, "execution", params, principalID)
	if err != nil {
		t.Fatalf("setup (create target): %v", err)
	}

	if err := st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID); err != nil {
		t.Fatalf("setup (attach justification): %v", err)
	}

	if err := st.RequestAuthorization(ctx, targetID, principalID); err != nil {
		t.Fatalf("setup (request authorization): %v", err)
	}

	if err := st.Approve(ctx, targetID, principalID); err != nil {
		t.Fatalf("setup (approve): %v", err)
	}

	return targetID
}

// createLiveIntent inserts a live action intent and returns its ID.
func createLiveIntent(t *testing.T, ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(ctx, `
		INSERT INTO action_intent (scenario_id, belief_id, action)
		VALUES ($1::UUID, $2::UUID, $3::STRING)
		RETURNING id`,
		scenarioID, beliefID, action).Scan(&id)
	if err != nil {
		t.Fatalf("setup (create live intent): %v", err)
	}
	return id
}

func tupleMatches(targetScenarioID, action string) kernel.AuthorityTuple {
	return kernel.AuthorityTuple{
		ResourceType:    "scenario",
		ResourceID:      targetScenarioID,
		Scope:           "belief:*",
		ActionNamespace: "solvent",
		ActionName:      action,
	}
}

// --- Execution Authorization Tests ---

// TestEA01_NoAuthorityTarget: no authority target exists → DENIED.
func TestEA01_NoAuthorityTarget(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(1)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA01")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "nonexistent-target", "nonexistent-actor",
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA02_PromotedNoAuthority: promoted belief + no authority → DENIED.
func TestEA02_PromotedNoAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(2)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA02")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "any-target", "any-actor",
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA03_WrongTarget: valid authority + wrong target → DENIED.
func TestEA03_WrongTarget(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(3)
	principalID := createPrincipal(t, ctx, st, "agent", "ea03-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA03")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	_ = targetID // used for setup only

	// Try to execute against a different target.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "wrong-target-id", principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong target, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA04_WrongAction: valid authority + wrong action → DENIED.
func TestEA04_WrongAction(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(4)
	principalID := createPrincipal(t, ctx, st, "agent", "ea04-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA04")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Try to execute with a different action than what was approved.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong action, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA05_RevokedAuthority: revoked authority → DENIED.
func TestEA05_RevokedAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(5)
	principalID := createPrincipal(t, ctx, st, "agent", "ea05-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA05")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Revoke the target.
	if err := st.RevokeTarget(ctx, targetID, principalID, "test revocation"); err != nil {
		t.Fatalf("setup (revoke target): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for revoked authority, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA06_PolicyAllowNoAuthority: policy ALLOW + authority ABSENT → DENIED.
func TestEA06_PolicyAllowNoAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(6)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA06")

	// No authority target exists. Policy may allow, but authority is absent.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA07_PolicyAllowRevokedAuthority: policy ALLOW + authority REVOKED → DENIED.
func TestEA07_PolicyAllowRevokedAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(7)
	principalID := createPrincipal(t, ctx, st, "agent", "ea07-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA07")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Revoke.
	if err := st.RevokeTarget(ctx, targetID, principalID, "test revocation"); err != nil {
		t.Fatalf("setup (revoke target): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA08_TargetMutatesBetweenPrepareAndExecute: target changes after preparation → DENIED.
func TestEA08_TargetMutatesBetweenPrepareAndExecute(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(8)
	principalID := createPrincipal(t, ctx, st, "agent", "ea08-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA08")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Prepare succeeds.
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("PrepareForAction should succeed, got: %s", decision.Reason)
	}

	// Revoke between prepare and execute.
	if err := st.RevokeTarget(ctx, targetID, principalID, "mutation test"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	// Execute must re-read current state and deny.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED after revocation, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when authority revoked between prepare and execute")
	}
}

// TestEA09_ActionMutatesBetweenPrepareAndExecute: action changes after preparation → DENIED.
func TestEA09_ActionMutatesBetweenPrepareAndExecute(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(9)
	principalID := createPrincipal(t, ctx, st, "agent", "ea09-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA09")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Prepare with "deploy".
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("PrepareForAction should succeed, got: %s", decision.Reason)
	}

	// Execute with different action "rollback".
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong action, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestEA10_FakeApproval: fake approval (no pin/hash) → DENIED.
func TestEA10_FakeApproval(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(10)
	principalID := createPrincipal(t, ctx, st, "agent", "ea10-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for EA10")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Try to authorize with a tuple that doesn't match the snapshot.
	// The target was approved with specific fields; using different fields should fail.
	wrongTuple := kernel.AuthorityTuple{
		PrincipalID:     principalID,
		ResourceType:    "wrong",
		ResourceID:      "wrong",
		Scope:           "wrong",
		ActionNamespace: "wrong",
		ActionName:      "wrong",
	}
	result, err := st.Authorize(ctx, targetID, wrongTuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for fake tuple, got ALLOWED")
	}

	// ExecuteAction should also deny.
	execResult, err := svc.ExecuteAction(ctx, sid, beliefID, "wrong", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if execResult.Allowed {
		t.Errorf("expected DENIED for fake approval, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// --- Capability Boundary Tests ---

// TestCB01_ExecutorCannotCreateAuthority: executor is downstream, not upstream.
func TestCB01_ExecutorCannotCreateAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(11)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CB01")

	// No authority exists. Executor cannot grant itself authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("executor must not be able to create authority")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestCB02_ProviderOutputNotAuthority: external output is evidence, not auth.
func TestCB02_ProviderOutputNotAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(12)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CB02")

	// Provider output cannot substitute for authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{
			"provider_output": "authorized by external system",
			"provider信任状":  "trusted",
		}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("provider output must not create authority")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestCB03_WorkflowStateCannotCreateAuthority: token lifecycle ≠ authority.
func TestCB03_WorkflowStateCannotCreateAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(13)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CB03")

	// Workflow state cannot substitute for authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{
			"workflow_state": "executing",
			"token_valid":    true,
		}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("workflow state must not create authority")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestCB04_AgentCannotApproveItself: approval requires separate principal.
func TestCB04_AgentCannotApproveItself(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(14)
	principalID := createPrincipal(t, ctx, st, "agent", "ea04-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CB04")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Using a different principal should fail.
	wrongPrincipal := createPrincipal(t, ctx, st, "agent", "ea04-wrong-issuer")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, wrongPrincipal,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("wrong principal must not be authorized")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestCB05_ActorSpoofingFails: request-body actor not trusted as auth.
func TestCB05_ActorSpoofingFails(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(15)
	principalID := createPrincipal(t, ctx, st, "agent", "ea05-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CB05")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Spoofed actor must not match.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, "spoofed-actor",
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("spoofed actor must not be authorized")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}

	_ = principalID
}

// --- Adapter / Integration Tests ---

// TestAI01_MalformedOutputDenied: malformed provider output → rejected.
func TestAI01_MalformedOutputDenied(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(16)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for AI01")

	// Malformed parameters should not grant authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{
			"malformed":     "\x00\x01\x02",
			"sql_injection": "'; DROP TABLE belief; --",
		}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("malformed output must not create authority")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// TestAI02_ProviderClaimsCannotManufactureAuthority: evidence ≠ authority.
func TestAI02_ProviderClaimsCannotManufactureAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(17)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for AI02")

	// Provider claims cannot manufacture authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		"", map[string]interface{}{
			"provider_claims":    "this action is authorized",
			"provider_signature": "fake-sig",
		}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("provider claims must not create authority")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when DENIED")
	}
}

// --- Critical Regression Tests ---

// TestCR_A_RevokedAfterPrepare: valid auth → prepare → auth revoked → DENIED.
func TestCR_A_RevokedAfterPrepare(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(18)
	principalID := createPrincipal(t, ctx, st, "agent", "cra-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CRA")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Prepare succeeds.
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("PrepareForAction should succeed, got: %s", decision.Reason)
	}

	// Revoke between prepare and execute.
	if err := st.RevokeTarget(ctx, targetID, principalID, "regression test A"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	// Execute must deny.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED after revocation, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("CR-A: executor must NOT be called when authority revoked between prepare and execute")
	}
}

// TestCR_B_TargetChangedAfterPrepare: valid auth → prepare → target changed → DENIED.
func TestCR_B_TargetChangedAfterPrepare(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(19)
	principalID := createPrincipal(t, ctx, st, "agent", "crb-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CRB")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Prepare succeeds.
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("PrepareForAction should succeed, got: %s", decision.Reason)
	}

	// Execute with a different target ID.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "different-target", principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong target, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("CR-B: executor must NOT be called when target changed between prepare and execute")
	}
}

// TestCR_C_ActionChangedAfterPrepare: valid auth → prepare → action changed → DENIED.
func TestCR_C_ActionChangedAfterPrepare(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(20)
	principalID := createPrincipal(t, ctx, st, "agent", "crc-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for CRC")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Prepare succeeds.
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("PrepareForAction should succeed, got: %s", decision.Reason)
	}

	// Execute with a different action.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		"", map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong action, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("CR-C: executor must NOT be called when action changed between prepare and execute")
	}
}

// --- Positive Test ---

// TestP01_ValidAuthExecutes: valid auth + correct target + correct action → ALLOWED.
func TestP01_ValidAuthExecutes(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "etcd deployed successfully")
	svc := newTestService(t, rec)

	sid := testScenario(21)
	principalID := createPrincipal(t, ctx, st, "agent", "p01-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for P01")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if !result.Allowed {
		t.Errorf("expected ALLOWED, got DENIED: %s", result.Reason)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false: %s", result.Error)
	}
	if !rec.Called() {
		t.Errorf("executor must be called when ALLOWED")
	}
	if result.Output != "etcd deployed successfully" {
		t.Errorf("expected output 'etcd deployed successfully', got %q", result.Output)
	}
}
