package ledger_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/ledger"
)

var (
	dsn  string
	db   *sql.DB
	svc  *ledger.Service
 kern *kernel.Store
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
	dsn = testdb.SuiteDSN("ledger")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "ledger tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	db, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ledger tests cannot start: open: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ledger tests cannot start: ping: %v\n", err)
		db.Close()
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	kern = kernel.New(db)
	aud := audit.New(db)
	svc = ledger.New(db, aud)

	code := m.Run()

	db.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

func createTestPrincipal(t *testing.T, ctx context.Context) string {
	t.Helper()
	id, err := kern.CreatePrincipal(ctx, "agent", "test-issuer")
	if err != nil {
		t.Fatalf("create principal: %v", err)
	}
	return id
}

func createTestTarget(t *testing.T, ctx context.Context, principalID string) string {
	t.Helper()
	id, err := kern.CreateTarget(ctx, principalID,
		"service", "svc-1", "deploy",
		"compute", "restart", "downtime", []byte(`{"env":"prod"}`), principalID)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	return id
}

func createTestBelief(t *testing.T, ctx context.Context, scenarioID string) string {
	t.Helper()
	id, err := kern.EnterBelief(ctx, scenarioID, "test belief for ledger", kernel.Derived)
	if err != nil {
		t.Fatalf("create belief: %v", err)
	}
	return id
}

func TestLedger_AuthorizeAndCreateIntent_Allowed(t *testing.T) {
	ctx := context.Background()
	scenarioID := fmt.Sprintf("00000000-0000-0000-0000-%012x", time.Now().UnixNano()%0xFFFFFFFFFFFF)

	principalID := createTestPrincipal(t, ctx)
	targetID := createTestTarget(t, ctx, principalID)
	beliefID := createTestBelief(t, ctx, scenarioID)

	// Retire all debts so the belief can be promoted.
	for _, item := range kernel.FullDebt {
		if err := kern.RetireDebt(ctx, beliefID, item); err != nil {
			t.Fatalf("retire debt %q: %v", item, err)
		}
	}
	if err := kern.Promote(ctx, beliefID); err != nil {
		t.Fatalf("promote belief: %v", err)
	}

	// Full authority chain: attach justification, request, approve.
	if err := kern.AttachJustification(ctx, targetID, beliefID, "promoted", principalID); err != nil {
		t.Fatalf("attach justification: %v", err)
	}

	// Full authority chain: attach justification, request, approve.
	if err := kern.AttachJustification(ctx, targetID, beliefID, "promoted", principalID); err != nil {
		t.Fatalf("attach justification: %v", err)
	}
	if err := kern.RequestAuthorization(ctx, targetID, principalID); err != nil {
		t.Fatalf("request authorization: %v", err)
	}
	if err := kern.Approve(ctx, targetID, principalID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "service",
		ResourceID:            "svc-1",
		Scope:                 "deploy",
		ActionNamespace:       "compute",
		ActionName:            "restart",
		ConsequenceType:       "downtime",
		ConsequenceParameters: []byte(`{"env":"prod"}`),
	}

	decision, err := svc.AuthorizeAndCreateIntent(ctx, scenarioID, beliefID, "deploy test",
		targetID, principalID, tuple)
	if err != nil {
		t.Fatalf("AuthorizeAndCreateIntent: %v", err)
	}

	if !decision.Allowed {
		t.Errorf("expected allowed=true, got allowed=false reason=%q", decision.Reason)
	}
	if decision.IntentState != "live" {
		t.Errorf("expected intent_state=live, got %q", decision.IntentState)
	}

	// Verify a live intent exists in the database.
	var cnt int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND belief_id = $2::UUID AND state = 'live'`,
		scenarioID, beliefID).Scan(&cnt); err != nil {
		t.Fatalf("query intent count: %v", err)
	}
	if cnt != 1 {
		t.Errorf("expected 1 live intent, got %d", cnt)
	}
}

func TestLedger_AuthorizeAndCreateIntent_Denied(t *testing.T) {
	ctx := context.Background()
	scenarioID := fmt.Sprintf("00000000-0000-0000-0000-%012x", time.Now().UnixNano()%0xFFFFFFFFFFFF)

	principalID := createTestPrincipal(t, ctx)
	targetID := createTestTarget(t, ctx, principalID)
	beliefID := createTestBelief(t, ctx, scenarioID)

	// Do NOT activate the target — authority should fail.

	tuple := kernel.AuthorityTuple{
		PrincipalID:           principalID,
		ResourceType:          "service",
		ResourceID:            "svc-1",
		Scope:                 "deploy",
		ActionNamespace:       "compute",
		ActionName:            "restart",
		ConsequenceType:       "downtime",
		ConsequenceParameters: []byte(`{"env":"prod"}`),
	}

	decision, err := svc.AuthorizeAndCreateIntent(ctx, scenarioID, beliefID, "deploy test",
		targetID, principalID, tuple)
	if err != nil {
		t.Fatalf("AuthorizeAndCreateIntent: %v", err)
	}

	if decision.Allowed {
		t.Error("expected allowed=false for unactivated target")
	}
	if decision.Reason == "" {
		t.Error("expected non-empty reason for denial")
	}

	// Verify no intent was created.
	var cnt int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND belief_id = $2::UUID`,
		scenarioID, beliefID).Scan(&cnt); err != nil {
		t.Fatalf("query intent count: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected 0 intents, got %d", cnt)
	}
}
