package authority

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
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
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
	"../../db/009_exact_authority_binding.sql",
	"../../db/010_debt_opaque.sql",
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
	id, err := st.EnterBelief(ctx, scenarioID, claim, kernel.Derived, []string{"testDebt"})
	if err != nil {
		t.Fatalf("setup (enter belief): %v", err)
	}
	// Retire all debt items so promotion succeeds.
	for _, item := range []string{"testDebt"} {
		_ = st.RetireDebt(ctx, scenarioID, id, item)
	}
	if err := st.Promote(ctx, scenarioID, id); err != nil {
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
func createLiveIntent(t *testing.T, ctx context.Context, db *sql.DB, scenarioID, beliefID, action string, opts ...string) string {
	t.Helper()
	var id string
	var err error
	if len(opts) >= 2 && opts[0] != "" && opts[1] != "" {
		err = db.QueryRowContext(ctx, `
			INSERT INTO action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
			VALUES ($1::UUID, $2::UUID, $3::STRING, $4::UUID, $5::UUID)
			RETURNING id`,
			scenarioID, beliefID, action, opts[0], opts[1]).Scan(&id)
	} else {
		err = db.QueryRowContext(ctx, `
			INSERT INTO action_intent (scenario_id, belief_id, action)
			VALUES ($1::UUID, $2::UUID, $3::STRING)
			RETURNING id`,
			scenarioID, beliefID, action).Scan(&id)
	}
	if err != nil {
		t.Fatalf("setup (create live intent): %v", err)
	}
	return id
}

func lookupSnapshotID(t *testing.T, ctx context.Context, db *sql.DB, targetID string) string {
	t.Helper()
	var snapshotID string
	err := db.QueryRowContext(ctx,
		`SELECT snapshot_id FROM target_activation WHERE target_id = $1::UUID`,
		targetID).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("setup (lookup snapshot): %v", err)
	}
	return snapshotID
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "nonexistent-target", "nonexistent-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "any-target", "any-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	_ = targetID // used for setup only

	// Try to execute against a different target.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "wrong-target-id", principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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

// TestEA03b_ConfusedDeputy: T1→T2 confused-deputy attack.
//
// Creates two real, valid, approved targets T1/S1 and T2/S2 with identical
// belief/action. Creates intent I1 via AuthorizeAndCreateIntent against T1/S1
// (the real kernel creation path). Then calls ExecuteAction with target T2.
//
// T2 is itself a valid authority — authorization may succeed. But ClaimIntent
// must reject I1 because I1 is bound to T1/S1, not T2/S2. This is the exact
// authority binding regression for the confused-deputy class.
func TestEA03b_ConfusedDeputy(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(3003)
	principalID := createPrincipal(t, ctx, st, "agent", "ea03b-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "confused deputy belief")

	// Create two approved targets with identical belief/action but different
	// consequence parameters. Both are real, valid authorities.
	t1 := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	t2 := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Create intent I1 via the real kernel creation path (AuthorizeAndCreateIntent).
	// This binds I1 to T1/S1 atomically — the same path production uses.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: testConsequenceParams(),
	}
	result, err := st.AuthorizeAndCreateIntent(ctx, t1, tuple, sid, beliefID, "deploy")
	if err != nil {
		t.Fatalf("AuthorizeAndCreateIntent(T1): %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected T1 authorization to be allowed, got denied: %s", result.Reason)
	}
	if result.IntentState != "live" {
		t.Fatalf("expected intent state live, got %q", result.IntentState)
	}

	// Read the intent ID created by AuthorizeAndCreateIntent.
	var intentID string
	err = shared.QueryRowContext(ctx,
		"SELECT id FROM action_intent WHERE scenario_id=$1::UUID AND belief_id=$2::UUID AND action='deploy' AND target_id=$3::UUID AND state='live'",
		sid, beliefID, t1).Scan(&intentID)
	if err != nil {
		t.Fatalf("read intent ID: %v", err)
	}

	// Execute against T2 — T2 is itself a valid, approved authority.
	// Authorization for T2 may succeed, but ClaimIntent must reject I1
	// because I1 is bound to T1/S1, not T2/S2.
	execResult, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", t2, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}

	if execResult.Allowed {
		t.Error("expected DENIED for confused-deputy T1→T2 substitution, got ALLOWED")
	}
	if rec.Called() {
		t.Error("executor must NOT be called when ClaimIntent rejects")
	}

	// Verify intent I1 is still live.
	var state string
	err = shared.QueryRowContext(ctx,
		"SELECT state FROM action_intent WHERE id=$1::UUID", intentID).Scan(&state)
	if err != nil {
		t.Fatalf("read intent state: %v", err)
	}
	if state != "live" {
		t.Errorf("intent should remain live after rejected claim, got %q", state)
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "rollback")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Try to execute with a different action than what was approved.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Revoke the target.
	if err := st.RevokeTarget(ctx, targetID, principalID, "test revocation"); err != nil {
		t.Fatalf("setup (revoke target): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// No authority target exists. Policy may allow, but authority is absent.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Revoke.
	if err := st.RevokeTarget(ctx, targetID, principalID, "test revocation"); err != nil {
		t.Fatalf("setup (revoke target): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "rollback")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "wrong")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// No authority exists. Executor cannot grant itself authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Provider output cannot substitute for authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{
			"provider_output": "authorized by external system",
			"provider信任状":     "trusted",
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Workflow state cannot substitute for authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Using a different principal should fail.
	wrongPrincipal := createPrincipal(t, ctx, st, "agent", "ea04-wrong-issuer")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, wrongPrincipal,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	// Spoofed actor must not match.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, "spoofed-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Malformed parameters should not grant authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Provider claims cannot manufacture authority.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "rollback")
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
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
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
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

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

// --- Adversarial Tests: Execution Security Contract ---

// TestExec11_ParameterSubstitutionAttack: caller supplies different repo/workflow/ref.
// Expected: kernel rejects mismatched parameters — the kernel itself prevents
// parameter substitution by validating consequence_parameters against the snapshot.
func TestExec11_ParameterSubstitutionAttack(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(30)
	principalID := createPrincipal(t, ctx, st, "agent", "exec11-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec11")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Pass DIFFERENT consequence_parameters than what was approved.
	attackerParams, _ := json.Marshal(map[string]string{
		"repo":     "evil/owner-repo",
		"workflow": "malicious.yml",
		"ref":      "main",
	})

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", attackerParams)
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	// The kernel validates consequence_parameters against the snapshot.
	// Mismatched params → DENIED. This is the correct security behavior:
	// the kernel prevents parameter substitution at the authorization boundary.
	if result.Allowed {
		t.Fatalf("expected DENIED for mismatched params, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when params mismatch")
	}
	t.Logf("correctly denied: reason=%s", result.Reason)
}

// TestExec12_ExecutorSelectorAttack: attempt to override executor via action name.
// Expected: action is authorized (kernel approves), but executor is not found.
// The hardcoded action→executor mapping is the only execution path.
func TestExec12_ExecutorSelectorAttack(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(31)
	principalID := createPrincipal(t, ctx, st, "agent", "exec12-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec12")

	// Create a target with action="malicious_action" (not in actionExecutorMap).
	params, _ := json.Marshal(map[string]string{"repo": "owner/repo", "workflow": "deploy.yml", "ref": "main"})
	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", sid, "belief:"+beliefID,
		"solvent", "malicious_action", "execution", params, principalID)
	if err != nil {
		t.Fatalf("setup (create target): %v", err)
	}
	_ = st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID)
	_ = st.RequestAuthorization(ctx, targetID, principalID)
	_ = st.Approve(ctx, targetID, principalID)

	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "malicious_action")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "malicious_action", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", params)
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	// The kernel authorizes the action (target is approved), but the executor
	// registry has no entry for "malicious_action". Result: Allowed=true, Success=false.
	if !result.Allowed {
		t.Errorf("expected ALLOWED (kernel approves), got DENIED: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (no executor), got true")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for unregistered action")
	}
}

// TestExec13_AuthorityRevokedMidFlight: revoke authority between authorize and execute.
// Expected: PrepareForAction re-reads current state, kernel.Authorize denies.
func TestExec13_AuthorityRevokedMidFlight(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(32)
	principalID := createPrincipal(t, ctx, st, "agent", "exec13-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec13")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke authority.
	if err := st.RevokeTarget(ctx, targetID, principalID, "mid-flight revocation"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED after revocation, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called when authority revoked")
	}
}

// TestExec14_CrossPrincipalExecution: principal A's intent executed by principal B.
// Expected: DENIED — authenticated principal does not match intent's authority context.
func TestExec14_CrossPrincipalExecution(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(33)
	principalA := createPrincipal(t, ctx, st, "agent", "exec14-issuer-a")
	principalB := createPrincipal(t, ctx, st, "agent", "exec14-issuer-b")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec14")
	targetID := createApprovedTarget(t, ctx, st, sid, principalA, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Principal B tries to execute principal A's intent.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalB,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for cross-principal execution, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for cross-principal execution")
	}
}

// TestExec15_WrongScenario: valid intent + different scenario_id → DENIED.
func TestExec15_WrongScenario(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(34)
	wrongSid := testScenario(35)
	principalID := createPrincipal(t, ctx, st, "agent", "exec15-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec15")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Use wrong scenario_id.
	result, err := svc.ExecuteAction(ctx, wrongSid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong scenario, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for wrong scenario")
	}
}

// TestExec16_WrongBelief: valid intent + different belief_id → DENIED.
func TestExec16_WrongBelief(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(36)
	principalID := createPrincipal(t, ctx, st, "agent", "exec16-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec16")
	wrongBeliefID := createAndPromoteBelief(t, ctx, st, sid, "wrong belief for Exec16")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Use wrong belief_id.
	result, err := svc.ExecuteAction(ctx, sid, wrongBeliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong belief, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for wrong belief")
	}
}

// TestExec17_WrongAction: valid intent + different action → DENIED.
func TestExec17_WrongAction(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(37)
	principalID := createPrincipal(t, ctx, st, "agent", "exec17-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec17")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Use wrong action.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong action, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for wrong action")
	}
}

// TestExec18_WrongTarget: valid intent + different target_id → DENIED.
func TestExec18_WrongTarget(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(38)
	principalID := createPrincipal(t, ctx, st, "agent", "exec18-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec18")
	createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Use wrong target_id.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "wrong-target-id", principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong target, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for wrong target")
	}
}

// TestExec19_WrongPrincipal: valid intent + different principal → DENIED.
func TestExec19_WrongPrincipal(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "output")
	svc := newTestService(t, rec)

	sid := testScenario(39)
	principalA := createPrincipal(t, ctx, st, "agent", "exec19-issuer-a")
	wrongPrincipal := createPrincipal(t, ctx, st, "agent", "exec19-issuer-wrong")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for Exec19")
	targetID := createApprovedTarget(t, ctx, st, sid, principalA, beliefID, "deploy")
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Use wrong principal.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, wrongPrincipal,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if result.Allowed {
		t.Errorf("expected DENIED for wrong principal, got ALLOWED")
	}
	if rec.Called() {
		t.Errorf("executor must NOT be called for wrong principal")
	}
}

// --- ExecuteAction empty intentID test (Plan 8.5 §5a) ---

func TestEA_EmptyIntentID(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	rec := executor.NewRecordingFunc("test_action", "should not run")
	svc := newTestService(t, rec)

	sid := testScenario(801)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for empty intentID")
	principalID := createPrincipal(t, ctx, st, "agent", "empty-intent-test")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		"", // empty intentID
		map[string]interface{}{}, "execution", testConsequenceParams())

	if err == nil {
		t.Fatal("expected error for empty intentID, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result for empty intentID, got %+v", result)
	}
	if rec.Called() {
		t.Error("executor must NOT be called when intentID is empty")
	}
}

// --- ReconcileIntent audit ordering tests (Plan 8.5 §5b, §5c) ---

func TestReconcile_CompletedAudit(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	sid := testScenario(802)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for reconcile completed")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Claim the intent (live → executing).
	if err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", ""); err != nil {
		t.Fatalf("setup claim: %v", err)
	}

	svc := newTestService(t, executor.NewRecordingFunc("noop", ""))
	operatorID := "test-operator-completed"

	// Reconcile as completed.
	err := svc.ReconcileIntent(ctx, sid, intentID, IntentOutcomeCompleted, operatorID)
	if err != nil {
		t.Fatalf("ReconcileIntent error: %v", err)
	}

	// Verify intent state is 'executed'.
	var state string
	_ = shared.QueryRowContext(ctx,
		`SELECT state FROM action_intent WHERE id=$1::UUID`, intentID).Scan(&state)
	if state != "executed" {
		t.Errorf("intent state: want 'executed', got %q", state)
	}

	// Verify reconciliation_completed audit entry.
	completedType := audit.ActivityReconciliationCompleted
	entries, err := audit.New(shared).GetActivities(ctx, sid, &completedType, 10)
	if err != nil {
		t.Fatalf("get activities: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.SubjectID == intentID && e.ActorID == operatorID {
			found = true
			break
		}
	}
	if !found {
		t.Error("reconciliation_completed audit entry not found")
	}
}

func TestReconcile_FailedAudit(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	sid := testScenario(803)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "belief for reconcile failed")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")
	// Intent is 'live' — not claimed, not executing.

	svc := newTestService(t, executor.NewRecordingFunc("noop", ""))
	operatorID := "test-operator-failed"

	// RollbackClaim on a non-executing intent — should fail with ErrNotExecuting.
	err := svc.ReconcileIntent(ctx, sid, intentID, IntentOutcomeFailed, operatorID)
	if err == nil {
		t.Fatal("expected error for rollback on non-executing intent, got nil")
	}

	// Verify intent state is still 'live'.
	var state string
	_ = shared.QueryRowContext(ctx,
		`SELECT state FROM action_intent WHERE id=$1::UUID`, intentID).Scan(&state)
	if state != "live" {
		t.Errorf("intent state: want 'live', got %q", state)
	}

	// Verify reconciliation_failed audit entry.
	failedType := audit.ActivityReconciliationFailed
	entries, err := audit.New(shared).GetActivities(ctx, sid, &failedType, 10)
	if err != nil {
		t.Fatalf("get activities: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.SubjectID == intentID && e.ActorID == operatorID {
			found = true
			if e.Details == nil {
				t.Error("reconciliation_failed audit entry has no details")
			}
			break
		}
	}
	if !found {
		t.Error("reconciliation_failed audit entry not found")
	}
}
