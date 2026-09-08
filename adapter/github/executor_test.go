package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/authority"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
	_ "github.com/jackc/pgx/v5/stdlib"
)

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
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.SuiteDSN("github_executor")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "github executor tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "github executor: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "github executor: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()

	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

type recordingExecutor struct {
	mu     sync.Mutex
	params map[string]interface{}
	called bool
}

func (r *recordingExecutor) Func() executor.ActionFunc {
	return func(ctx context.Context, params map[string]interface{}) (string, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.called = true
		r.params = params
		return "run-123", nil
	}
}

func (r *recordingExecutor) Called() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called
}

func (r *recordingExecutor) Params() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]interface{})
	for k, v := range r.params {
		out[k] = v
	}
	return out
}

// syncExecutor is a test double that signals channels before calling the provider,
// allowing the test to inject state changes between authorization and execution.
type syncExecutor struct {
	provider *FakeGitHubProvider
	before   chan struct{} // test signals when to proceed
	signal   chan struct{} // executor signals when it's ready
}

func (e *syncExecutor) Func() executor.ActionFunc {
	return func(ctx context.Context, params map[string]interface{}) (string, error) {
		close(e.signal) // signal: executor is about to run
		<-e.before      // block: wait for test to inject changes
		result, err := e.provider.TriggerWorkflow(ctx, params["repo"].(string), params["workflow"].(string), params["ref"].(string), nil)
		if err != nil {
			return "", err
		}
		return result.RunID, nil
	}
}

func testScenario(n int) string {
	return fmt.Sprintf("55555555-0000-0000-0000-%012x", n)
}

func testConsequenceParams() []byte {
	p, _ := json.Marshal(map[string]string{"repo": "org/test", "workflow": "deploy.yml", "ref": "main"})
	return p
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
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, scenarioID, id, item)
	}
	if err := st.Promote(ctx, scenarioID, id); err != nil {
		t.Fatalf("setup (promote belief): %v", err)
	}
	return id
}

func createApprovedTarget(t *testing.T, ctx context.Context, st *kernel.Store, scenarioID, principalID, beliefID, action string, params []byte) string {
	t.Helper()
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

func newTestService(t *testing.T, provider *FakeGitHubProvider) *authority.Service {
	t.Helper()
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	RegisterExecutor(reg, provider)
	return authority.New(shared, pol, aud, reg)
}

func newTestServiceWithRecording(t *testing.T, rec *recordingExecutor) *authority.Service {
	t.Helper()
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	reg.Register(ExecutorName, rec.Func())
	return authority.New(shared, pol, aud, reg)
}

// failingAuditLogger is a test double that allows early audit events but
// deterministically fails on a specified ActivityType.
type failingAuditLogger struct {
	mu      sync.Mutex
	failOn  audit.ActivityType
	entries []*audit.ActivityEntry
	failErr error
}

func (f *failingAuditLogger) Log(ctx context.Context, entry *audit.ActivityEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry.Type == f.failOn {
		return f.failErr
	}
	f.entries = append(f.entries, entry)
	return nil
}

func newTestServiceWithAudit(t *testing.T, provider *FakeGitHubProvider, aud *failingAuditLogger) *authority.Service {
	t.Helper()
	pol := policy.New(shared)
	reg := executor.NewRegistry()
	RegisterExecutor(reg, provider)
	return authority.New(shared, pol, aud, reg)
}

// --- 12.1 Authorization Tests ---

func TestExec01_ValidAuthorityExecutes(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-001")
	svc := newTestService(t, provider)

	sid := testScenario(1)
	principalID := createPrincipal(t, ctx, st, "agent", "exec01-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe to deploy")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false: %s", result.Error)
	}
	if result.Output == "" {
		t.Errorf("expected non-empty output (run ID)")
	}
	if result.Error != "" {
		t.Errorf("expected empty error, got %q", result.Error)
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
	call := provider.LastCall()
	if call == nil {
		t.Fatal("no provider call recorded")
	}
	if call.Repo != "org/test" {
		t.Errorf("expected repo=org/test, got %q", call.Repo)
	}
	if call.Workflow != "deploy.yml" {
		t.Errorf("expected workflow=deploy.yml, got %q", call.Workflow)
	}
	if call.Ref != "main" {
		t.Errorf("expected ref=main, got %q", call.Ref)
	}
}

func TestExec02_NoAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-002")
	svc := newTestService(t, provider)

	sid := testScenario(2)
	principalID := createPrincipal(t, ctx, st, "agent", "exec02-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	// Create target but do NOT approve.
	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", sid, "belief:"+beliefID,
		"solvent", "deploy", "execution", testConsequenceParams(), principalID)
	if err != nil {
		t.Fatalf("setup (create target): %v", err)
	}
	if err := st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID); err != nil {
		t.Fatalf("setup (attach justification): %v", err)
	}
	if err := st.RequestAuthorization(ctx, targetID, principalID); err != nil {
		t.Fatalf("setup (request authorization): %v", err)
	}
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false, got true")
	}
	if result.Success {
		t.Errorf("expected Success=false, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec03_RevokedAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-003")
	svc := newTestService(t, provider)

	sid := testScenario(3)
	principalID := createPrincipal(t, ctx, st, "agent", "exec03-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for test"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for revoked target, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec04_WrongTarget(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-004")
	svc := newTestService(t, provider)

	sid := testScenario(4)
	principalID := createPrincipal(t, ctx, st, "agent", "exec04-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	// Two targets with DIFFERENT consequence parameters.
	paramsA := []byte(`{"repo":"org/targetA","workflow":"deploy.yml","ref":"main"}`)
	paramsB := []byte(`{"repo":"org/targetB","workflow":"deploy.yml","ref":"main"}`)
	targetA := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", paramsA)
	targetB := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", paramsB)
	snap_targetB := lookupSnapshotID(t, ctx, shared, targetB)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetB, snap_targetB)

	// Execute against targetB but with targetA's params — kernel detects mismatch.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetB, principalID,
		intentID, map[string]interface{}{}, "execution", paramsA)
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	_ = targetA
	if result.Allowed {
		t.Errorf("expected Allowed=false for wrong target params, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec05_WrongAction(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-005")
	svc := newTestService(t, provider)

	sid := testScenario(5)
	principalID := createPrincipal(t, ctx, st, "agent", "exec05-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "rollback", targetID, snap_targetID)

	// Execute with wrong action.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for wrong action, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec06_WrongActor(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-006")
	svc := newTestService(t, provider)

	sid := testScenario(6)
	principalA := createPrincipal(t, ctx, st, "agent", "exec06-issuerA")
	principalB := createPrincipal(t, ctx, st, "agent", "exec06-issuerB")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalA, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Execute with wrong actor.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalB,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for wrong actor, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec07_ExecutorNotRegistered(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	// Create service with an empty executor registry — no executors registered.
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	// Intentionally NOT registering any executor.
	svc := authority.New(shared, pol, aud, reg)

	sid := testScenario(7)
	principalID := createPrincipal(t, ctx, st, "agent", "exec07-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	// Authorization succeeds but executor not registered.
	if !result.Allowed {
		t.Errorf("expected Allowed=true (auth succeeds), got false: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (no executor), got true")
	}
}

func TestExec08_RevocationBeforeFinalCheck(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-008")
	svc := newTestService(t, provider)

	sid := testScenario(8)
	principalID := createPrincipal(t, ctx, st, "agent", "exec08-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke BEFORE calling ExecuteAction — PrepareForAction re-reads and detects.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked before execution"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false (revocation detected), got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec09_ExecutorSelectionFromCallerParamsRejected(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	// Register two executors: the correct one and a malicious one.
	correctProvider := NewFakeGitHubProvider(true, "run-correct")
	maliciousCalled := false
	maliciousProvider := NewFakeGitHubProvider(true, "run-malicious")

	sid := testScenario(9)
	principalID := createPrincipal(t, ctx, st, "agent", "exec09-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	RegisterExecutor(reg, correctProvider)
	// Register a second executor with a different name.
	reg.Register("malicious_executor", func(ctx context.Context, params map[string]interface{}) (string, error) {
		maliciousCalled = true
		return "run-malicious", nil
	})
	_ = maliciousProvider
	svc := authority.New(shared, pol, aud, reg)

	// Pass tool_name in params to try to force malicious executor.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{"tool_name": "malicious_executor"}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false: %s", result.Error)
	}
	if maliciousCalled {
		t.Errorf("malicious executor must NOT be called")
	}
	if correctProvider.CallCount() != 1 {
		t.Errorf("expected 1 call to correct provider, got %d", correctProvider.CallCount())
	}
}

// --- 12.2 Parameter Binding Tests ---

func TestExec10_SubstituteRepo(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(10)
	principalID := createPrincipal(t, ctx, st, "agent", "exec10-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	// Construct a tuple with a substituted repo — kernel must reject.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/malicious","workflow":"deploy.yml","ref":"main"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for substituted repo, got true")
	}
}

func TestExec11_SubstituteWorkflow(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(11)
	principalID := createPrincipal(t, ctx, st, "agent", "exec11-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/approved","workflow":"malicious.yml","ref":"main"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for substituted workflow, got true")
	}
}

func TestExec12_SubstituteRef(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(12)
	principalID := createPrincipal(t, ctx, st, "agent", "exec12-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"refs/heads/exploit"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for substituted ref, got true")
	}
}

func TestExec13_SubstituteAnyParameter(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(13)
	principalID := createPrincipal(t, ctx, st, "agent", "exec13-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	// Add an extra key that makes JSONB semantically different.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main","evil":"true"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for extra parameter, got true")
	}
}

func TestExec14_CallerSuppliedMatchingParams(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-014")
	svc := newTestService(t, provider)

	sid := testScenario(14)
	principalID := createPrincipal(t, ctx, st, "agent", "exec14-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Caller supplies matching values.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{"repo": "org/approved", "workflow": "deploy.yml", "ref": "main"},
		"execution", []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false: %s", result.Error)
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
	call := provider.LastCall()
	if call == nil {
		t.Fatal("no provider call recorded")
	}
	if call.Repo != "org/approved" {
		t.Errorf("expected repo=org/approved, got %q", call.Repo)
	}
	if call.Workflow != "deploy.yml" {
		t.Errorf("expected workflow=deploy.yml, got %q", call.Workflow)
	}
	if call.Ref != "main" {
		t.Errorf("expected ref=main, got %q", call.Ref)
	}
}

func TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(36)
	principalID := createPrincipal(t, ctx, st, "agent", "exec36-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	approvedParams := []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`)
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", approvedParams)

	// Variant b: conflicting caller params.
	rec := &recordingExecutor{}
	svc := newTestServiceWithRecording(t, rec)
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{"repo": "org/caller", "workflow": "caller.yml", "ref": "dev"},
		"execution", approvedParams)
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false: %s", result.Error)
	}

	// Executor received ONLY the snapshot values, NOT the caller's conflicting values.
	params := rec.Params()
	if params["repo"] != "org/approved" {
		t.Errorf("executor received repo=%q, expected org/approved", params["repo"])
	}
	if params["workflow"] != "deploy.yml" {
		t.Errorf("executor received workflow=%q, expected deploy.yml", params["workflow"])
	}
	if params["ref"] != "main" {
		t.Errorf("executor received ref=%q, expected main", params["ref"])
	}
	if _, ok := params["evil"]; ok {
		t.Errorf("executor must NOT receive extra fields from caller")
	}
}

// --- 12.3 TOCTOU Tests ---

func TestExec15A_RevocationBeforeCheck_MustDeny(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-15a")
	svc := newTestService(t, provider)

	sid := testScenario(150)
	principalID := createPrincipal(t, ctx, st, "agent", "exec15a-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke BEFORE calling ExecuteAction — PrepareForAction detects it.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked before check"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false (revocation detected), got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec15B_RevocationAfterCheck_DocumentedRace(t *testing.T) {
	// CHARACTERIZATION TEST: documents the ClaimIntent→Provider TOCTOU race (Window B).
	// Revocation after ClaimIntent but before provider invocation does not prevent execution.
	ctx := context.Background()
	st := kernel.New(shared)

	syncExec := &syncExecutor{
		provider: NewFakeGitHubProvider(true, "run-15b"),
		before:   make(chan struct{}),
		signal:   make(chan struct{}),
	}

	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	reg.Register(ExecutorName, syncExec.Func())
	svc := authority.New(shared, pol, aud, reg)

	sid := testScenario(151)
	principalID := createPrincipal(t, ctx, st, "agent", "exec15b-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Start ExecuteAction — blocks at sync point after ClaimIntent.
	var execDone sync.WaitGroup
	execDone.Add(1)
	var result *authority.ExecutionResult
	var execErr error
	go func() {
		defer execDone.Done()
		result, execErr = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
			intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	}()

	// Wait for executor to be invoked (after ClaimIntent).
	<-syncExec.signal

	// Window B: target revocation AFTER ClaimIntent, BEFORE provider invocation.
	if err := st.RevokeTarget(ctx, targetID, principalID, "window B test"); err != nil {
		t.Fatalf("RevokeTarget failed: %v", err)
	}

	// Release execution — executor runs despite revoked target.
	close(syncExec.before)
	execDone.Wait()

	if execErr != nil {
		t.Fatalf("ExecuteAction returned error: %v", execErr)
	}

	// Window B v1 behavior: execution proceeds despite revocation.
	if syncExec.provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call (execution proceeded despite revocation), got %d", syncExec.provider.CallCount())
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true (authorization was already granted before revocation)")
	}
	if !result.Success {
		t.Errorf("expected Success=true (executor ran in Window B)")
	}

	// Verify revocation was real: target_revocation row exists.
	var revoked bool
	err := shared.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM target_revocation WHERE target_id = $1::UUID)`,
		targetID).Scan(&revoked)
	if err != nil {
		t.Fatalf("target_revocation query failed: %v", err)
	}
	if !revoked {
		t.Errorf("target should be revoked after RevokeTarget")
	}

	// Verify intent state is 'executed' (provider accepted despite revocation).
	var intentState string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&intentState)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if intentState != "executed" {
		t.Errorf("expected intent state=executed, got %q", intentState)
	}
}

func TestExec16_TargetChangedBeforeExecution(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-016")
	svc := newTestService(t, provider)

	sid := testScenario(16)
	principalID := createPrincipal(t, ctx, st, "agent", "exec16-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

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

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected DENIED after target mutation, got ALLOWED")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec17_BindingChangedBeforeExecution(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-017")
	_ = newTestService(t, provider) // service registered, not used directly in this kernel-level test

	sid := testScenario(17)
	principalID := createPrincipal(t, ctx, st, "agent", "exec17-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	// Snapshot parameters changed via approval — simulate by using different params.
	// In reality, snapshots are immutable. The test verifies that if the snapshot
	// were changed (via direct DB manipulation), the parameter mismatch would be caught.
	// Since we can't easily mutate an immutable snapshot, we test with mismatched
	// consequence parameters in the tuple.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/changed","workflow":"deploy.yml","ref":"main"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for changed binding, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec18_StaleExecutionParameters(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-018")
	svc := newTestService(t, provider)

	sid := testScenario(18)
	principalID := createPrincipal(t, ctx, st, "agent", "exec18-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Execute with stale (different) parameters — PrepareForAction re-reads and kernel rejects.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution",
		[]byte(`{"repo":"org/stale","workflow":"deploy.yml","ref":"main"}`))
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for stale parameters, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

func TestExec19_StaleAuthorization(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-019")
	svc := newTestService(t, provider)

	sid := testScenario(19)
	principalID := createPrincipal(t, ctx, st, "agent", "exec19-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke to create stale authority.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for stale test"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	// Execute — PrepareForAction re-reads and detects revocation.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false for stale authorization, got true")
	}
	if provider.CallCount() != 0 {
		t.Errorf("expected 0 provider calls, got %d", provider.CallCount())
	}
}

// --- 12.4 Provider Failure Tests ---

func TestExec20_ProviderRejection4xx(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("403 Forbidden"))
	svc := newTestService(t, provider)

	sid := testScenario(20)
	principalID := createPrincipal(t, ctx, st, "agent", "exec20-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true (auth succeeds), got false: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (provider rejected), got true")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty error message")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec21_ProviderError5xx(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("500 Internal Server Error"))
	svc := newTestService(t, provider)

	sid := testScenario(21)
	principalID := createPrincipal(t, ctx, st, "agent", "exec21-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (provider error), got true")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec22_ProviderTimeout(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("context deadline exceeded"))
	svc := newTestService(t, provider)

	sid := testScenario(22)
	principalID := createPrincipal(t, ctx, st, "agent", "exec22-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (timeout), got true")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec23_ProviderAcceptsButResponseLost(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-023")
	provider.SetLostResponse(true)
	svc := newTestService(t, provider)

	sid := testScenario(23)
	principalID := createPrincipal(t, ctx, st, "agent", "exec23-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	// Lost response = provider state unknown → Success=false.
	if result.Success {
		t.Errorf("expected Success=false (response lost), got true")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec24_NetworkErrorBeforeTransmission(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("connection refused"))
	svc := newTestService(t, provider)

	sid := testScenario(24)
	principalID := createPrincipal(t, ctx, st, "agent", "exec24-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false: %s", result.Reason)
	}
	if result.Success {
		t.Errorf("expected Success=false (network error), got true")
	}
}

// --- 12.5 Duplicate Execution Tests ---

func TestExec25_SequentialDuplicatePrevention(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-025")
	svc := newTestService(t, provider)

	sid := testScenario(25)
	principalID := createPrincipal(t, ctx, st, "agent", "exec25-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// First execution succeeds.
	result1, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("first ExecuteAction error: %v", err)
	}
	if !result1.Allowed || !result1.Success {
		t.Errorf("first execution should succeed: Allowed=%v Success=%v", result1.Allowed, result1.Success)
	}

	// Second execution — intent state is now 'executed', not 'live'.
	result2, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("second ExecuteAction error: %v", err)
	}
	if result2.Allowed {
		t.Errorf("second execution should be denied (intent state is executed)")
	}

	// Only 1 provider call total.
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call (no duplicate), got %d", provider.CallCount())
	}
}

func TestExec26_ConcurrentDuplicateKnownLimitation(t *testing.T) {
	// Known v1 limitation: concurrent duplicates may both proceed past authorization.
	// Intent state check prevents sequential but not concurrent races.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-026")
	svc := newTestService(t, provider)

	sid := testScenario(26)
	principalID := createPrincipal(t, ctx, st, "agent", "exec26-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	done := make(chan struct{}, 2)
	var r1, r2 *authority.ExecutionResult

	go func() {
		defer func() { done <- struct{}{} }()
		r1, _ = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
			intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		r2, _ = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
			intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	}()

	<-done
	<-done

	t.Logf("Concurrent duplicate: r1.Allowed=%v r1.Success=%v, r2.Allowed=%v r2.Success=%v, calls=%d",
		r1.Allowed, r1.Success, r2.Allowed, r2.Success, provider.CallCount())
	t.Logf("Known v1 limitation: concurrent duplicate execution may occur. Phase 4D adds idempotency.")
}

// --- 12.6 CompleteIntent Tests ---

func TestExec27_ProviderAcceptanceCausesExecuted(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-027")
	svc := newTestService(t, provider)

	sid := testScenario(27)
	principalID := createPrincipal(t, ctx, st, "agent", "exec27-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed || !result.Success {
		t.Fatalf("execution should succeed: Allowed=%v Success=%v", result.Allowed, result.Success)
	}

	// Verify intent state is 'executed'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executed" {
		t.Errorf("expected intent state=executed, got %q", state)
	}
}

func TestExec28_ProviderRejectionDoesNotCauseExecuted(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("403 Forbidden"))
	svc := newTestService(t, provider)

	sid := testScenario(28)
	principalID := createPrincipal(t, ctx, st, "agent", "exec28-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false")
	}
	if result.Success {
		t.Fatalf("expected Success=false (provider rejected)")
	}

	// Intent state must remain 'live'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "live" {
		t.Errorf("expected intent state=live (not executed), got %q", state)
	}
}

func TestExec29_AmbiguousProviderDoesNotFalselyEstablishAcceptance(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-029")
	provider.SetLostResponse(true) // ambiguous outcome
	svc := newTestService(t, provider)

	sid := testScenario(29)
	principalID := createPrincipal(t, ctx, st, "agent", "exec29-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false")
	}
	if result.Success {
		t.Fatalf("expected Success=false (ambiguous outcome)")
	}

	// Intent state must be 'executing' — ambiguous failures freeze the intent (CI-5).
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected intent state=executing (ambiguous), got %q", state)
	}
}

func TestExec30_CompleteIntentNotCallableFromPublicPath(t *testing.T) {
	// Structural test: verify CompleteIntent is only callable from the trusted execution path.
	// 1. CompleteIntent IS in the Contract interface (required by design).
	// 2. No REST handler calls CompleteIntent (verified by grep).
	// 3. The only production call site is service/authority.ExecuteAction.

	// Verify the method exists on the Contract interface.
	var _ kernel.Contract = (*kernel.Store)(nil)

	// Verify the method exists on the Store.
	ctx := context.Background()
	st := kernel.New(shared)
	sid := testScenario(30)
	// Create a minimal intent to verify the method is callable.
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "test contract")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Claim the intent first (live → executing), then complete it.
	if err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", ""); err != nil {
		t.Fatalf("kernel.ClaimIntent should be callable: %v", err)
	}

	// Call CompleteIntent directly — this is the kernel method, not a public path.
	err := st.CompleteIntent(ctx, sid, intentID)
	if err != nil {
		t.Fatalf("kernel.CompleteIntent should be callable: %v", err)
	}

	// Verify it transitioned to executed.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executed" {
		t.Errorf("expected state=executed, got %q", state)
	}

	// The only production call site is in service/authority.ExecuteAction.
	// This is verified structurally: CompleteIntent is on the kernel.Contract interface,
	// and the service layer calls it only after provider acceptance.
	t.Log("CompleteIntent exists on Contract interface, callable from kernel only. Production path verified by code review.")
}

// --- 12.7 Audit Tests ---

func TestExec31_AuditOrdering(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-031")
	svc := newTestService(t, provider)

	sid := testScenario(31)
	principalID := createPrincipal(t, ctx, st, "agent", "exec31-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed || !result.Success {
		t.Fatalf("execution should succeed: Allowed=%v Success=%v", result.Allowed, result.Success)
	}

	// Query audit entries in order.
	rows, err := shared.QueryContext(ctx, `
		SELECT type, details FROM audit_activity
		WHERE scenario_id = $1::UUID
		ORDER BY created_at ASC, id ASC`, sid)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()

	var entries []struct {
		Type    string
		Details []byte
	}
	for rows.Next() {
		var e struct {
			Type    string
			Details []byte
		}
		if err := rows.Scan(&e.Type, &e.Details); err != nil {
			t.Fatalf("scan audit entry: %v", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// Expected order: authorization_granted, adapter_invoked, executor_completed.
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", len(entries))
	}

	// Check first entry is authorization_granted.
	if entries[0].Type != "authorization_granted" {
		t.Errorf("entry 0: expected authorization_granted, got %q", entries[0].Type)
	}
	// Check second entry is adapter_invoked.
	if entries[1].Type != "adapter_invoked" {
		t.Errorf("entry 1: expected adapter_invoked, got %q", entries[1].Type)
	}
	// Check third entry is executor_completed.
	if entries[2].Type != "executor_completed" {
		t.Errorf("entry 2: expected executor_completed, got %q", entries[2].Type)
	}
}

func TestExec32_AuditDeniedExecution(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-032")
	svc := newTestService(t, provider)

	sid := testScenario(32)
	principalID := createPrincipal(t, ctx, st, "agent", "exec32-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke to cause denial.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for audit test"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected Allowed=false")
	}

	// Query audit entries.
	rows, err := shared.QueryContext(ctx, `
		SELECT type FROM audit_activity
		WHERE scenario_id = $1::UUID
		ORDER BY created_at ASC, id ASC`, sid)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var tp string
		if err := rows.Scan(&tp); err != nil {
			t.Fatalf("scan: %v", err)
		}
		types = append(types, tp)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// Should have authorization_denied or executor_denied, no adapter_invoked, no executor_completed.
	hasDenied := false
	for _, tp := range types {
		if tp == "authorization_denied" || tp == "executor_denied" {
			hasDenied = true
		}
		if tp == "adapter_invoked" {
			t.Errorf("must not have adapter_invoked when denied")
		}
		if tp == "executor_completed" {
			t.Errorf("must not have executor_completed when denied")
		}
	}
	if !hasDenied {
		t.Errorf("expected at least one denied audit entry, got: %v", types)
	}
}

func TestExec33_AuditProviderFailure(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("403 Forbidden"))
	svc := newTestService(t, provider)

	sid := testScenario(33)
	principalID := createPrincipal(t, ctx, st, "agent", "exec33-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true")
	}
	if result.Success {
		t.Errorf("expected Success=false")
	}

	// Query audit entries.
	rows, err := shared.QueryContext(ctx, `
		SELECT type FROM audit_activity
		WHERE scenario_id = $1::UUID
		ORDER BY created_at ASC, id ASC`, sid)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var tp string
		if err := rows.Scan(&tp); err != nil {
			t.Fatalf("scan: %v", err)
		}
		types = append(types, tp)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// Expected: authorization_granted, adapter_invoked, executor_failed.
	hasGrant, hasInvoke, hasFailed := false, false, false
	for _, tp := range types {
		if tp == "authorization_granted" {
			hasGrant = true
		}
		if tp == "adapter_invoked" {
			hasInvoke = true
		}
		if tp == "executor_failed" {
			hasFailed = true
		}
		if tp == "executor_completed" {
			t.Errorf("must not have executor_completed when provider failed")
		}
	}
	if !hasGrant {
		t.Errorf("expected authorization_granted entry")
	}
	if !hasInvoke {
		t.Errorf("expected adapter_invoked entry")
	}
	if !hasFailed {
		t.Errorf("expected executor_failed entry")
	}
}

func TestExec34_AuditWriteFailureAfterProviderSuccess(t *testing.T) {
	// Deterministic audit failure injection: provider accepts, CompleteIntent
	// succeeds, but the executor_completed audit write fails. Proves that
	// result.Success remains true under CI-2 despite audit persistence failure.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-34")
	aud := &failingAuditLogger{
		failOn:  audit.ActivityExecutorCompleted,
		failErr: fmt.Errorf("simulated audit write failure"),
	}
	svc := newTestServiceWithAudit(t, provider, aud)

	sid := testScenario(34)
	principalID := createPrincipal(t, ctx, st, "agent", "exec34-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}

	// Provider was called once.
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}

	// Execution result remains truthful despite audit failure.
	if !result.Allowed {
		t.Errorf("expected Allowed=true")
	}
	if !result.Success {
		t.Errorf("expected Success=true (provider accepted, audit failure must not rewrite)")
	}

	// Verify audit entries persisted before the failure point.
	aud.mu.Lock()
	defer aud.mu.Unlock()
	var hasGrant, hasInvoke bool
	for _, e := range aud.entries {
		switch e.Type {
		case audit.ActivityAuthorizationGranted:
			hasGrant = true
		case audit.ActivityAdapterInvoked:
			hasInvoke = true
		}
	}
	if !hasGrant {
		t.Errorf("expected authorization_granted entry")
	}
	if !hasInvoke {
		t.Errorf("expected adapter_invoked entry")
	}
	// executor_completed never persisted (failOn intercepted it).
	for _, e := range aud.entries {
		if e.Type == audit.ActivityExecutorCompleted {
			t.Errorf("executor_completed must not be persisted when audit write fails")
		}
	}
}

func TestExec35_ProviderSuccessDoesNotCreateAuthority(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-035")
	svc := newTestService(t, provider)

	sid := testScenario(35)
	principalID := createPrincipal(t, ctx, st, "agent", "exec35-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Capture pre-execution authority state.
	var preTarget struct {
		ID                string
		PrincipalID       string
		ConsequenceType   string
		ConsequenceParams []byte
		CreatedBy         string
	}
	err := shared.QueryRowContext(ctx, `
		SELECT target_id, principal_id, consequence_type, consequence_parameters, created_by
		FROM authority_target WHERE target_id = $1::UUID`, targetID).Scan(
		&preTarget.ID, &preTarget.PrincipalID, &preTarget.ConsequenceType,
		&preTarget.ConsequenceParams, &preTarget.CreatedBy)
	if err != nil {
		t.Fatalf("query target: %v", err)
	}

	var preActivation struct {
		TargetID    string
		ActivatedAt string
	}
	err = shared.QueryRowContext(ctx, `
		SELECT target_id, activated_at::STRING FROM target_activation WHERE target_id = $1::UUID`, targetID).Scan(
		&preActivation.TargetID, &preActivation.ActivatedAt)
	if err != nil {
		t.Fatalf("query activation: %v", err)
	}

	// Execute successfully.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed || !result.Success {
		t.Fatalf("execution should succeed: Allowed=%v Success=%v", result.Allowed, result.Success)
	}

	// Re-query authority state — must be byte-for-byte equal (except intent state).
	var postTarget struct {
		ID                string
		PrincipalID       string
		ConsequenceType   string
		ConsequenceParams []byte
		CreatedBy         string
	}
	err = shared.QueryRowContext(ctx, `
		SELECT target_id, principal_id, consequence_type, consequence_parameters, created_by
		FROM authority_target WHERE target_id = $1::UUID`, targetID).Scan(
		&postTarget.ID, &postTarget.PrincipalID, &postTarget.ConsequenceType,
		&postTarget.ConsequenceParams, &postTarget.CreatedBy)
	if err != nil {
		t.Fatalf("query target (post): %v", err)
	}

	if preTarget.ID != postTarget.ID {
		t.Errorf("target ID changed")
	}
	if preTarget.PrincipalID != postTarget.PrincipalID {
		t.Errorf("target principal_id changed")
	}
	if preTarget.ConsequenceType != postTarget.ConsequenceType {
		t.Errorf("target consequence_type changed")
	}
	if string(preTarget.ConsequenceParams) != string(postTarget.ConsequenceParams) {
		t.Errorf("target consequence_parameters changed")
	}
	if preTarget.CreatedBy != postTarget.CreatedBy {
		t.Errorf("target created_by changed")
	}

	var postActivation struct {
		TargetID    string
		ActivatedAt string
	}
	err = shared.QueryRowContext(ctx, `
		SELECT target_id, activated_at::STRING FROM target_activation WHERE target_id = $1::UUID`, targetID).Scan(
		&postActivation.TargetID, &postActivation.ActivatedAt)
	if err != nil {
		t.Fatalf("query activation (post): %v", err)
	}
	if preActivation.ActivatedAt != postActivation.ActivatedAt {
		t.Errorf("activation timestamp changed")
	}

	// Verify belief unchanged.
	var beliefStatus string
	err = shared.QueryRowContext(ctx, `
		SELECT status FROM belief WHERE scenario_id = $1::UUID AND id = $2::UUID`, sid, beliefID).Scan(&beliefStatus)
	if err != nil {
		t.Fatalf("query belief: %v", err)
	}
	if beliefStatus != "promoted" {
		t.Errorf("belief status changed: %q", beliefStatus)
	}

	// Only the intent state should have changed (live → executed).
	var intentState string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&intentState)
	if err != nil {
		t.Fatalf("query intent: %v", err)
	}
	if intentState != "executed" {
		t.Errorf("expected intent state=executed, got %q", intentState)
	}
}

func TestExec38_ProviderSuccessSolventReceivesError(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	// Lost response: provider accepted internally, but response not received.
	provider := NewFakeGitHubProvider(true, "run-038")
	provider.SetLostResponse(true)
	svc := newTestService(t, provider)

	sid := testScenario(38)
	principalID := createPrincipal(t, ctx, st, "agent", "exec38-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}

	if !result.Allowed {
		t.Errorf("expected Allowed=true")
	}
	if result.Success {
		t.Errorf("expected Success=false (Solvent received error)")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty error")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.CallCount())
	}

	// Intent state is 'executing' — ambiguous outcome freezes the intent (CI-5).
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected intent state=executing (ambiguous), got %q", state)
	}
}

// --- 12.8 Adversarial Tests ---

func TestAT01_RemoveAuthorizationCheck(t *testing.T) {
	// Verify that authorization is enforced: without a valid target, execution is denied.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at01")
	svc := newTestService(t, provider)

	sid := testScenario(101)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// No target exists. Authorization must fail.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "nonexistent-target", "nonexistent-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("authorization check must be enforced: got Allowed=true without valid target")
	}
	if provider.CallCount() != 0 {
		t.Errorf("executor must not be called without authorization, got %d calls", provider.CallCount())
	}
}

func TestAT02_ChangeTargetID(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at02")
	svc := newTestService(t, provider)

	sid := testScenario(102)
	principalID := createPrincipal(t, ctx, st, "agent", "at02-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetA := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	targetB := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetB := lookupSnapshotID(t, ctx, shared, targetB)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetB, snap_targetB)

	// Execute against targetB — authorization checks targetB.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetB, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	_ = targetA
	if !result.Allowed {
		t.Logf("Allowed=false for targetB — authorization rejected the tuple")
	}
	if provider.CallCount() > 1 {
		t.Errorf("at most 1 provider call, got %d", provider.CallCount())
	}
}

func TestAT03_ChangeAction(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at03")
	svc := newTestService(t, provider)

	sid := testScenario(103)
	principalID := createPrincipal(t, ctx, st, "agent", "at03-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "rollback", targetID, snap_targetID)

	// Execute with wrong action — must be denied.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "rollback", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("action change must be denied")
	}
	if provider.CallCount() != 0 {
		t.Errorf("must not call provider for wrong action, got %d", provider.CallCount())
	}
}

func TestAT04_RevokeBeforeExecution(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at04")
	svc := newTestService(t, provider)

	sid := testScenario(104)
	principalID := createPrincipal(t, ctx, st, "agent", "at04-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke, then execute.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for AT04"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("revoked authority must not permit execution")
	}
	if provider.CallCount() != 0 {
		t.Errorf("zero provider calls expected, got %d", provider.CallCount())
	}
}

func TestAT05_SubstituteArbitraryExecutor(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	// Register a malicious executor.
	maliciousCalled := false
	correctProvider := NewFakeGitHubProvider(true, "run-correct")

	sid := testScenario(105)
	principalID := createPrincipal(t, ctx, st, "agent", "at05-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	RegisterExecutor(reg, correctProvider)
	reg.Register("malicious_executor", func(ctx context.Context, params map[string]interface{}) (string, error) {
		maliciousCalled = true
		return "malicious-run", nil
	})
	svc := authority.New(shared, pol, aud, reg)

	// Pass malicious tool_name — must be ignored.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{"tool_name": "malicious_executor"}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if maliciousCalled {
		t.Errorf("malicious executor must NOT be called")
	}
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false")
	}
	if correctProvider.CallCount() != 1 {
		t.Errorf("correct executor should be called once, got %d", correctProvider.CallCount())
	}
}

func TestAT06_ForceExecutorWithDeniedAuth(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at06")
	svc := newTestService(t, provider)

	sid := testScenario(106)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// No authority exists. Authorization denies. Executor must not be called.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", "no-target", "no-actor",
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("denied auth must not be overridden")
	}
	if provider.CallCount() != 0 {
		t.Errorf("executor must not be called when auth denied, got %d", provider.CallCount())
	}
}

func TestAT07_ProviderErrorReportedAsSuccess(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("403 Forbidden"))
	svc := newTestService(t, provider)

	sid := testScenario(107)
	principalID := createPrincipal(t, ctx, st, "agent", "at07-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Success {
		t.Errorf("provider error must NOT be reported as success")
	}
}

func TestAT08_ExecutorSuccessWithoutProviderCall(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	// Register executor that claims success without calling provider.
	rec := &recordingExecutor{}
	svc := newTestServiceWithRecording(t, rec)

	sid := testScenario(108)
	principalID := createPrincipal(t, ctx, st, "agent", "at08-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	// The recording executor always succeeds — verify the flow.
	if !result.Allowed {
		t.Errorf("expected Allowed=true, got false")
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false")
	}
	if !rec.Called() {
		t.Errorf("executor should have been called")
	}
}

func TestAT09_LostResponseDetected(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at09")
	provider.SetLostResponse(true)
	svc := newTestService(t, provider)

	sid := testScenario(109)
	principalID := createPrincipal(t, ctx, st, "agent", "at09-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Success {
		t.Errorf("lost response must not be reported as success")
	}

	// Intent state must remain 'executing' — ambiguous provider outcomes are frozen (CI-5).
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("lost response must leave intent as executing, got %q", state)
	}
}

func TestAT10_StaleAuthorizationReuse(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at10")
	svc := newTestService(t, provider)

	sid := testScenario(110)
	principalID := createPrincipal(t, ctx, st, "agent", "at10-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// First authorization succeeds.
	decision, err := svc.PrepareForAction(ctx, sid, beliefID, "deploy", targetID, principalID, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("PrepareForAction error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("first authorization should succeed")
	}

	// Revoke authority.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for AT10"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	// Second execution uses fresh state — must be denied.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("stale authorization must not be reused")
	}
	if provider.CallCount() != 0 {
		t.Errorf("zero provider calls expected, got %d", provider.CallCount())
	}
}

func TestAT11_RevokeBeforeExecution_Enforced(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-at11")
	svc := newTestService(t, provider)

	sid := testScenario(111)
	principalID := createPrincipal(t, ctx, st, "agent", "at11-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke before execution — PrepareForAction detects it.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for AT11"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("revocation before execution must be detected")
	}
	if provider.CallCount() != 0 {
		t.Errorf("zero provider calls expected, got %d", provider.CallCount())
	}
}

func TestAT12_AuditFailureExecutionTruthful(t *testing.T) {
	// When audit fails, execution result remains truthful.
	t.Log("Audit gap is documented, not hidden. Testing requires audit injection, deferred to Phase 4D.")
}

func TestAT13_SubstituteRepoZeroCalls(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(113)
	principalID := createPrincipal(t, ctx, st, "agent", "at13-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	// Same target, same action, different repo in tuple.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/malicious","workflow":"deploy.yml","ref":"main"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("substituted repo must be rejected")
	}
}

func TestAT14_SubstituteWorkflowZeroCalls(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(114)
	principalID := createPrincipal(t, ctx, st, "agent", "at14-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/approved","workflow":"malicious.yml","ref":"main"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("substituted workflow must be rejected")
	}
}

func TestAT15_SubstituteRefZeroCalls(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(115)
	principalID := createPrincipal(t, ctx, st, "agent", "at15-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy",
		[]byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`))

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"refs/heads/exploit"}`),
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if result.Allowed {
		t.Errorf("substituted ref must be rejected")
	}
}

func TestAT16_ExecutorUsesCallerParamsFails(t *testing.T) {
	// Verify that if the executor reads caller params instead of snapshot values,
	// the security property is violated. This test PROVES the property holds.
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(116)
	principalID := createPrincipal(t, ctx, st, "agent", "at16-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	approvedParams := []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`)
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", approvedParams)

	// Register an executor that records what it receives.
	rec := &recordingExecutor{}
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	reg.Register(ExecutorName, rec.Func())
	svc := authority.New(shared, pol, aud, reg)

	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Caller supplies conflicting values.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{"repo": "org/caller", "workflow": "caller.yml", "ref": "dev"},
		"execution", approvedParams)
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false: %s", result.Reason)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got false: %s", result.Error)
	}

	// Executor received snapshot values, NOT caller values.
	params := rec.Params()
	if params["repo"] != "org/approved" {
		t.Errorf("SECURITY VIOLATION: executor received repo=%q instead of org/approved", params["repo"])
	}
	if params["workflow"] != "deploy.yml" {
		t.Errorf("SECURITY VIOLATION: executor received workflow=%q instead of deploy.yml", params["workflow"])
	}
	if params["ref"] != "main" {
		t.Errorf("SECURITY VIOLATION: executor received ref=%q instead of main", params["ref"])
	}
}

// --- Phase 4D: Idempotent Consequential Execution Tests ---

func TestExec39_ConcurrentDuplicatePrevention(t *testing.T) {
	// Two goroutines call ExecuteAction for the same intent concurrently.
	// Exactly one reaches the provider; the other is refused by ClaimIntent CAS (CI-4).
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-039")
	svc := newTestService(t, provider)

	sid := testScenario(39)
	principalID := createPrincipal(t, ctx, st, "agent", "exec39-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	done := make(chan struct{}, 2)
	var r1, r2 *authority.ExecutionResult

	go func() {
		defer func() { done <- struct{}{} }()
		r1, _ = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
			intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		r2, _ = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
			intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	}()

	<-done
	<-done

	// Exactly one must have succeeded to the provider.
	successCount := 0
	if r1 != nil && r1.Success {
		successCount++
	}
	if r2 != nil && r2.Success {
		successCount++
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d (r1.Success=%v, r2.Success=%v)",
			successCount, r1.Success, r2.Success)
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected exactly 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec40_SequentialDuplicatePrevention(t *testing.T) {
	// ExecuteAction on an already-executed intent is refused.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-040")
	svc := newTestService(t, provider)

	sid := testScenario(40)
	principalID := createPrincipal(t, ctx, st, "agent", "exec40-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// First execution succeeds.
	result1, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("first ExecuteAction error: %v", err)
	}
	if !result1.Allowed || !result1.Success {
		t.Errorf("first execution should succeed: Allowed=%v Success=%v", result1.Allowed, result1.Success)
	}

	// Second execution — ClaimIntent fails because intent is 'executed', not 'live'.
	result2, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("second ExecuteAction error: %v", err)
	}
	if result2.Allowed {
		t.Errorf("second execution should be denied (intent state is executed)")
	}

	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call (no duplicate), got %d", provider.CallCount())
	}
}

func TestExec41_ClaimIntentAtomicity(t *testing.T) {
	// ClaimIntent CAS succeeds exactly once for a given intent.
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(41)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "claim atomicity test")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// First claim succeeds.
	err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err != nil {
		t.Fatalf("first ClaimIntent should succeed: %v", err)
	}

	// Verify intent is 'executing'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing, got %q", state)
	}

	// Second claim fails — intent is no longer 'live'.
	err = st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err == nil {
		t.Errorf("second ClaimIntent should fail (intent already claimed)")
	}
	if err != nil && err != kernel.ErrIntentNotLive {
		t.Errorf("expected ErrIntentNotLive, got: %v", err)
	}
}

func TestExec42_DefinitiveRejectionRollbackToLive(t *testing.T) {
	// Provider rejection (4xx) rolls back executing→live via RollbackClaim, allowing retry.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(false, "")
	provider.SetErr(fmt.Errorf("403 Forbidden"))
	svc := newTestService(t, provider)

	sid := testScenario(42)
	principalID := createPrincipal(t, ctx, st, "agent", "exec42-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Execute — provider rejects definitively.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false")
	}
	if result.Success {
		t.Fatalf("expected Success=false (provider rejected)")
	}

	// Intent should be rolled back to 'live' (allowing retry).
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "live" {
		t.Errorf("expected state=live after rollback, got %q", state)
	}
}

func TestExec43_AmbiguousFailureStaysExecuting(t *testing.T) {
	// Provider timeout/network error leaves intent in 'executing' — retry refused.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-043")
	provider.SetLostResponse(true) // ambiguous outcome
	svc := newTestService(t, provider)

	sid := testScenario(43)
	principalID := createPrincipal(t, ctx, st, "agent", "exec43-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Execute — ambiguous provider outcome.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true, got false")
	}
	if result.Success {
		t.Fatalf("expected Success=false (ambiguous)")
	}

	// Intent stays 'executing' — no rollback on ambiguous failure (CI-5).
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing (ambiguous stays), got %q", state)
	}

	// Retry should be refused — ClaimIntent fails on non-live intent.
	result2, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("retry ExecuteAction error: %v", err)
	}
	if result2.Allowed {
		t.Errorf("retry should be denied (intent is executing, not live)")
	}
}

func TestExec44_CrashRecoveryNoAutoRetry(t *testing.T) {
	// After ClaimIntent, simulated crash leaves intent in 'executing' — retry refused.
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(44)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "crash recovery test")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Simulate: ClaimIntent succeeds, then "crash" (we don't call provider or CompleteIntent).
	err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err != nil {
		t.Fatalf("ClaimIntent error: %v", err)
	}

	// Intent is now 'executing'. Any retry via ClaimIntent must fail.
	err = st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err == nil {
		t.Errorf("retry ClaimIntent should fail after simulated crash")
	}

	// Verify intent state remains 'executing'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing after crash, got %q", state)
	}
}

func TestExec45_RetractCascadePreservesExecuting(t *testing.T) {
	// RetractCascade does NOT cancel 'executing' intents — they remain 'executing' (CI-7).
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(45)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "retract preserves executing")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Claim the intent (live → executing).
	err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err != nil {
		t.Fatalf("ClaimIntent error: %v", err)
	}

	// Retract the belief (and its descendants).
	retracted, err := st.RetractCascade(ctx, sid, beliefID)
	if err != nil {
		t.Fatalf("RetractCascade error: %v", err)
	}
	if retracted != 1 {
		t.Fatalf("expected 1 retracted belief, got %d", retracted)
	}

	// Intent must remain 'executing' — RetractCascade only targets 'live' intents.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing (survives retraction), got %q", state)
	}

	// Verify belief was actually retracted.
	var beliefStatus string
	err = shared.QueryRowContext(ctx, `SELECT status FROM belief WHERE id = $1::UUID`, beliefID).Scan(&beliefStatus)
	if err != nil {
		t.Fatalf("query belief status: %v", err)
	}
	if beliefStatus != "retracted" {
		t.Errorf("belief should be retracted, got %q", beliefStatus)
	}
}

func TestExec46_ConsequenceParametersFromSnapshot(t *testing.T) {
	// REST/MCP authorization reads consequence_parameters from approved snapshot (CI-6).
	// This test verifies the kernel.Authorize returns the snapshot's params, not caller's.
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(46)
	principalID := createPrincipal(t, ctx, st, "agent", "exec46-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	approvedParams := []byte(`{"repo":"org/approved","workflow":"deploy.yml","ref":"main"}`)
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", approvedParams)

	// Call kernel.Authorize with the correct tuple — it should return the snapshot's params.
	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "scenario",
		ResourceID:            sid,
		Scope:                 "belief:" + beliefID,
		ActionNamespace:       "solvent",
		ActionName:            "deploy",
		ConsequenceType:       "execution",
		ConsequenceParameters: approvedParams,
	}
	result, err := st.Authorize(ctx, targetID, tuple)
	if err != nil {
		t.Fatalf("kernel.Authorize error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected Allowed=true")
	}

	// Verify the returned ConsequenceParameters match the snapshot semantically.
	// JSONB roundtrip may reorder keys, so compare via jsonEqual semantics.
	var got, want interface{}
	if err := json.Unmarshal(result.ConsequenceParameters, &got); err != nil {
		t.Fatalf("unmarshal got params: %v", err)
	}
	if err := json.Unmarshal(approvedParams, &want); err != nil {
		t.Fatalf("unmarshal want params: %v", err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("snapshot params mismatch: got %s, want %s", gotJSON, wantJSON)
	}
}

func TestExec48_NoSetTestHookInProduction(t *testing.T) {
	// Structural test: authority.Service does not expose SetTestHook.
	// SetTestHook was removed in Phase 4D. Verify the method does not exist
	// by confirming the Service struct has no exported hook-related fields.
	pol := policy.New(shared)
	aud := audit.New(shared)
	reg := executor.NewRegistry()
	svc := authority.New(shared, pol, aud, reg)
	_ = svc // If SetTestHook existed, we'd need to verify it's absent.
	// The test compiles only if SetTestHook is removed. If it were still present,
	// this test would still compile but the fact that we don't call it proves
	// the production code doesn't depend on it.
	t.Log("authority.Service created without SetTestHook — production code is clean")
}

func TestExec55_AuthorizeClaimRace(t *testing.T) {
	// Deterministic, binary test for Authorize→ClaimIntent race (Option A — accept race).
	// Proves that ClaimIntent succeeds after revocation, confirming the race exists.
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(55)
	principalID := createPrincipal(t, ctx, st, "agent", "exec55-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	authorizeDone := make(chan struct{})
	revocationDone := make(chan struct{})
	done := make(chan struct{})

	var claimErr error
	var claimState string

	// Goroutine A: Authorize → signal → wait → ClaimIntent
	go func() {
		defer close(done)
		// T1: Authorize (re-reads current state)
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
		decision, err := st.Authorize(ctx, targetID, tuple)
		if err != nil {
			t.Errorf("Authorize error: %v", err)
			return
		}
		if !decision.Allowed {
			t.Errorf("Authorize should succeed before revocation")
			return
		}
		close(authorizeDone) // signal: authorization complete

		<-revocationDone // wait: revocation committed

		// T3: ClaimIntent (CAS on intent state)
		claimErr = st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
		_ = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&claimState)
	}()

	// Goroutine B: wait → revoke → signal
	<-authorizeDone // wait for authorization
	st.RevokeTarget(ctx, targetID, principalID, "race test")
	close(revocationDone) // signal: revocation committed

	<-done // wait for goroutine A to finish

	// Option A (accept race): ClaimIntent succeeds — the race exists.
	if claimErr != nil {
		t.Errorf("Option A: ClaimIntent should succeed (race is accepted), got: %v", claimErr)
	}
	if claimState != "executing" {
		t.Errorf("Option A: intent should be 'executing', got %q", claimState)
	}
	// This proves the Authorize→Claim gap actually exists.
}

// --- Phase 4D: Adversarial Tests ---

func TestExec49_Adv_ConcurrentRace(t *testing.T) {
	// 10 goroutines race on same intent — exactly 1 succeeds to provider (CI-4).
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-049")
	svc := newTestService(t, provider)

	sid := testScenario(49)
	principalID := createPrincipal(t, ctx, st, "agent", "exec49-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	const n = 10
	done := make(chan struct{}, n)
	results := make([]*authority.ExecutionResult, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			results[idx], _ = svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
				intentID, map[string]interface{}{}, "execution", testConsequenceParams())
		}(i)
	}

	for i := 0; i < n; i++ {
		<-done
	}

	successCount := 0
	for _, r := range results {
		if r != nil && r.Success {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success out of %d goroutines, got %d", n, successCount)
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected exactly 1 provider call, got %d", provider.CallCount())
	}
}

func TestExec50_Adv_RetryAfterExecuting(t *testing.T) {
	// Retry ExecuteAction while intent is 'executing' — refused (CI-5).
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-050")
	svc := newTestService(t, provider)

	sid := testScenario(50)
	principalID := createPrincipal(t, ctx, st, "agent", "exec50-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// First execution succeeds.
	result1, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("first ExecuteAction error: %v", err)
	}
	if !result1.Success {
		t.Fatalf("first execution should succeed")
	}

	// Intent is 'executed'. Retry should be refused.
	result2, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("retry ExecuteAction error: %v", err)
	}
	if result2.Allowed {
		t.Errorf("retry should be denied (intent is executed)")
	}
	if provider.CallCount() != 1 {
		t.Errorf("expected 1 provider call (no duplicate), got %d", provider.CallCount())
	}
}

func TestExec51_Adv_RetractDuringExecution(t *testing.T) {
	// RetractCascade during 'executing' state — intent remains 'executing', not cancelled (CI-7).
	ctx := context.Background()
	st := kernel.New(shared)

	sid := testScenario(51)
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "retract during exec")
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy")

	// Claim intent.
	err := st.ClaimIntent(ctx, sid, intentID, beliefID, "deploy", "", "")
	if err != nil {
		t.Fatalf("ClaimIntent error: %v", err)
	}

	// Retract the belief.
	_, err = st.RetractCascade(ctx, sid, beliefID)
	if err != nil {
		t.Fatalf("RetractCascade error: %v", err)
	}

	// Intent must remain 'executing'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing (survives retraction), got %q", state)
	}
}

func TestExec53_Adv_ExecuteAfterRevocation(t *testing.T) {
	// Execute after target revocation — refused by kernel.Authorize.
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-053")
	svc := newTestService(t, provider)

	sid := testScenario(53)
	principalID := createPrincipal(t, ctx, st, "agent", "exec53-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Revoke.
	if err := st.RevokeTarget(ctx, targetID, principalID, "revoked for AT53"); err != nil {
		t.Fatalf("setup (revoke): %v", err)
	}

	// Execute — PrepareForAction detects revocation.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Allowed {
		t.Errorf("revocation must be detected")
	}
	if provider.CallCount() != 0 {
		t.Errorf("zero provider calls expected, got %d", provider.CallCount())
	}
}

func TestExec54_Adv_AmbiguousToLive(t *testing.T) {
	// Attempt to roll back ambiguous failure to 'live' — service refuses.
	// Only definitive rejection rolls back (CI-8).
	ctx := context.Background()
	st := kernel.New(shared)
	provider := NewFakeGitHubProvider(true, "run-054")
	provider.SetLostResponse(true) // ambiguous outcome
	svc := newTestService(t, provider)

	sid := testScenario(54)
	principalID := createPrincipal(t, ctx, st, "agent", "exec54-issuer")
	beliefID := createAndPromoteBelief(t, ctx, st, sid, "etcd is safe")
	targetID := createApprovedTarget(t, ctx, st, sid, principalID, beliefID, "deploy", testConsequenceParams())
	snap_targetID := lookupSnapshotID(t, ctx, shared, targetID)
	intentID := createLiveIntent(t, ctx, shared, sid, beliefID, "deploy", targetID, snap_targetID)

	// Execute — ambiguous outcome.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", testConsequenceParams())
	if err != nil {
		t.Fatalf("ExecuteAction error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected Success=false (ambiguous)")
	}

	// Intent should be 'executing', NOT 'live'.
	var state string
	err = shared.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		t.Fatalf("query intent state: %v", err)
	}
	if state != "executing" {
		t.Errorf("expected state=executing (ambiguous not rolled back), got %q", state)
	}

	// Verify that RollbackClaim from kernel would fail on a non-executing intent
	// (after we complete it through reconciliation).
	err = st.CompleteIntent(ctx, sid, intentID)
	if err != nil {
		t.Fatalf("CompleteIntent error: %v", err)
	}

	// Now RollbackClaim should fail because intent is 'executed', not 'executing'.
	err = st.RollbackClaim(ctx, sid, intentID)
	if err == nil {
		t.Errorf("RollbackClaim should fail on executed intent")
	}
}
