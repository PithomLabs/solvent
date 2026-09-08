package view_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/internal/view"
	"github.com/PithomLabs/solvent/kernel"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	dsnExplain    string
	sharedExplain *sql.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsnExplain = testdb.SuiteDSN("view")
	name, _ := testdb.DBNameFromDSN(dsnExplain)
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
	if err := testdb.Reset(ctx, dsnExplain, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "view explain tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	var err error
	sharedExplain, err = testdb.Open(dsnExplain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "view explain tests cannot start: open: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := sharedExplain.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "view explain tests cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	code := m.Run()
	_ = sharedExplain.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

func scenarioExplain(n int) string {
	return fmt.Sprintf("eeee0000-0000-0000-0000-%012x", n)
}

// TestExplain_AskBeforePromotion — TEST A from prompt2.md:6
// belief entered, debt open, Ask reports unresolved, Authorize is refused (predicted).
func TestExplain_AskBeforePromotion(t *testing.T) {
	ctx := context.Background()
	sc := scenarioExplain(1)
	st := kernel.New(sharedExplain)

	beliefID, err := st.EnterBelief(ctx, sc, "explain test: ask before promotion", kernel.Derived)
	if err != nil {
		t.Fatalf("EnterBelief: %v", err)
	}

	snap, err := view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{IncludeEvidence: false})
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	var auditTmp int
	_ = sharedExplain.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id WHERE a.state='live' AND b.status<>'promoted' AND a.scenario_id=$1::UUID`, sc).Scan(&auditTmp)
	snap.AuditLiveOnNonPromoted = auditTmp
	// Use ExplainSnapshot directly
	exp := view.ExplainSnapshot("track1", sc, snap)
	if len(exp.Beliefs) != 1 {
		t.Fatalf("expected 1 belief explained, got %d", len(exp.Beliefs))
	}
	be := exp.Beliefs[0]
	if be.BeliefID != beliefID {
		t.Errorf("belief_id mismatch")
	}
	if be.CanPromote {
		t.Error("expected CanPromote=false when debt remains")
	}
	if be.PredictedPromotionSQLState != "23514" {
		t.Errorf("expected predicted 23514 for promotion, got %q", be.PredictedPromotionSQLState)
	}
	if be.PredictedPromotionConstraint != "promoted_is_debt_free" {
		t.Errorf("expected promoted_is_debt_free, got %q", be.PredictedPromotionConstraint)
	}
	if be.CanAuthorize {
		t.Error("expected CanAuthorize=false when not promoted")
	}
	if be.PredictedAuthSQLState != "23503" {
		t.Errorf("expected predicted 23503 for authorize, got %q", be.PredictedAuthSQLState)
	}
	if be.PredictedAuthConstraint != "gate" {
		t.Errorf("expected gate, got %q", be.PredictedAuthConstraint)
	}
	if len(be.RemainingDebt) != 6 {
		t.Errorf("expected 6 remaining debt, got %d", len(be.RemainingDebt))
	}
	if be.IsPromoted || be.IsRetracted {
		t.Error("should be neither promoted nor retracted")
	}
	if be.HumanSummary == "" {
		t.Error("HumanSummary empty")
	}

	// Verify real DB still refuses
	if err := st.Promote(ctx, sc, beliefID); err == nil {
		t.Error("expected Promote to be refused with 23514, got nil")
	}
	if err := st.IntentOnPromoted(ctx, sc, beliefID, "deploy"); err == nil {
		t.Error("expected IntentOnPromoted to be refused with 23503, got nil")
	}
}

// TestExplain_PromotionAfterDebtDischarge — TEST B
func TestExplain_PromotionAfterDebtDischarge(t *testing.T) {
	ctx := context.Background()
	sc := scenarioExplain(2)
	st := kernel.New(sharedExplain)

	beliefID, err := st.EnterBelief(ctx, sc, "explain test: promotion after discharge", kernel.Derived)
	if err != nil {
		t.Fatalf("EnterBelief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, sc, beliefID, item); err != nil {
			t.Fatalf("RetireDebt %s: %v", item, err)
		}
	}
	// Before promote, explain should say can_promote true
	snap, _ := view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	snap.AuditLiveOnNonPromoted = 0
	exp := view.ExplainSnapshot("track1", sc, snap)
	if len(exp.Beliefs) != 1 {
		t.Fatalf("expected 1 belief")
	}
	if !exp.Beliefs[0].CanPromote {
		t.Errorf("expected CanPromote=true after debt empty, got false: %q", exp.Beliefs[0].PromotionBlockedReason)
	}

	if err := st.Promote(ctx, sc, beliefID); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	snap, _ = view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	exp = view.ExplainSnapshot("track1", sc, snap)
	be := exp.Beliefs[0]
	if !be.IsPromoted {
		t.Error("expected IsPromoted true")
	}
	if be.CanPromote {
		t.Error("CanPromote should be false when already promoted")
	}
	if !be.CanAuthorize {
		t.Errorf("expected CanAuthorize true when promoted, blocked: %q", be.AuthorizationBlockedReason)
	}

	if err := st.IntentOnPromoted(ctx, sc, beliefID, "deploy v1"); err != nil {
		t.Fatalf("IntentOnPromoted after promote: %v", err)
	}
	snap, _ = view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	exp = view.ExplainSnapshot("track1", sc, snap)
	be = exp.Beliefs[0]
	if len(be.LiveIntents) != 1 {
		t.Errorf("expected 1 live intent, got %d", len(be.LiveIntents))
	}
	if be.LiveIntents[0].Action != "deploy v1" {
		t.Errorf("unexpected action %q", be.LiveIntents[0].Action)
	}
}

// TestExplain_ReassessmentFalsification — TEST C
func TestExplain_ReassessmentFalsification(t *testing.T) {
	ctx := context.Background()
	sc := scenarioExplain(3)
	st := kernel.New(sharedExplain)

	beliefID, err := st.EnterBelief(ctx, sc, "explain test: reassessment", kernel.Derived)
	if err != nil {
		t.Fatalf("EnterBelief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, sc, beliefID, item)
	}
	if err := st.Promote(ctx, sc, beliefID); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if err := st.IntentOnPromoted(ctx, sc, beliefID, "deploy"); err != nil {
		t.Fatalf("IntentOnPromoted: %v", err)
	}

	// Before falsify, explain shows live
	snap, _ := view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	exp := view.ExplainSnapshot("track1", sc, snap)
	if len(exp.Beliefs[0].LiveIntents) != 1 {
		t.Fatalf("expected live before falsify")
	}
	if exp.AuditLiveOnNonPromoted != 0 {
		t.Errorf("audit should be 0 before falsify")
	}

	// Falsify
	if _, err := st.RetractCascade(ctx, sc, beliefID); err != nil {
		t.Fatalf("RetractCascade: %v", err)
	}
	snap, _ = view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	// Need audit
	var audit int
	_ = sharedExplain.QueryRowContext(ctx, `SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id WHERE a.state='live' AND b.status<>'promoted' AND a.scenario_id=$1::UUID`, sc).Scan(&audit)
	snap.AuditLiveOnNonPromoted = audit
	exp = view.ExplainSnapshot("track1", sc, snap)
	be := exp.Beliefs[0]
	if !be.IsRetracted {
		t.Error("expected IsRetracted true after falsify")
	}
	if be.CanAuthorize {
		t.Error("CanAuthorize should be false after retraction")
	}
	if len(be.LiveIntents) != 0 {
		t.Errorf("expected 0 live intents after falsify, got %d", len(be.LiveIntents))
	}
	if exp.AuditLiveOnNonPromoted != 0 {
		t.Errorf("audit should be 0 after cascade, got %d", exp.AuditLiveOnNonPromoted)
	}
	if be.HumanSummary == "" || len(be.HumanSummary) < 20 {
		t.Error("HumanSummary too short after retraction")
	}
}

// TestExplain_ReadOnly — explain must not mutate DB
func TestExplain_ReadOnly(t *testing.T) {
	ctx := context.Background()
	sc := scenarioExplain(4)
	st := kernel.New(sharedExplain)
	beliefID, _ := st.EnterBelief(ctx, sc, "read-only test", kernel.Derived)

	beforeCount := countBeliefs(t, ctx, sc)
	snap, _ := view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{BeliefID: beliefID, IncludeEvidence: true})
	_ = view.ExplainSnapshot("track1", sc, snap)
	_ = view.ExplainSnapshot("track1", sc, snap)
	afterCount := countBeliefs(t, ctx, sc)
	if beforeCount != afterCount {
		t.Errorf("Explain mutated DB: before %d after %d", beforeCount, afterCount)
	}
}

// TestExplain_InvalidBeliefID — handler-level guard is tested via view direct, but we check empty case
func TestExplain_EmptyScenario(t *testing.T) {
	ctx := context.Background()
	sc := scenarioExplain(99)
	snap, err := view.GetSnapshot(ctx, sharedExplain, sc, view.SnapshotOpts{})
	if err != nil {
		t.Fatalf("GetSnapshot empty: %v", err)
	}
	exp := view.ExplainSnapshot("track1", sc, snap)
	if len(exp.Beliefs) != 0 {
		t.Errorf("expected 0 beliefs for empty scenario, got %d", len(exp.Beliefs))
	}
	if exp.GlobalSummary == "" {
		t.Error("GlobalSummary empty for empty scenario")
	}
}

func countBeliefs(t *testing.T, ctx context.Context, sc string) int {
	t.Helper()
	var n int
	if err := sharedExplain.QueryRowContext(ctx, `SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&n); err != nil {
		t.Fatalf("countBeliefs: %v", err)
	}
	return n
}
