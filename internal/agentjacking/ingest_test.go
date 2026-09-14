package agentjacking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/internal/belief"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var testDB *sql.DB

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
	dsn := testdb.SuiteDSN("agentjacking")

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "agentjacking-ingest-test cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	var err error
	testDB, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentjacking-ingest-test cannot start: open: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()

	_ = testDB.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// scenario formats a deterministic scenario ID for tests.
func scenario(n int) string {
	return fmt.Sprintf("33333333-0000-0000-0000-%012d", n)
}

// TestC_SixDebtsUntouched verifies that immediately after ingesting only the
// Sentry error fixture, the belief has all six original debt items. This is the
// actual "retires zero debt" claim — tested before any promotion or authorization
// attempt, so a future change that accidentally retires one debt item would be
// caught here rather than only at promotion time.
func TestC_SixDebtsUntouched(t *testing.T) {
	ctx := context.Background()
	sc := scenario(1)

	// Reset scenario rows for idempotent re-runs.
	resetScenario(t, ctx, sc)

	res, err := Ingest(ctx, testDB, sc, "../../demo/agentjacking/fixtures/sentry_error.json")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	// The adapter must retire exactly zero debt — all six belief.WizardDebt() items remain.
	if len(res.DebtRemaining) != len(belief.WizardDebt()) {
		t.Fatalf("debt count = %d, want %d (belief.WizardDebt())", len(res.DebtRemaining), len(belief.WizardDebt()))
	}
	for i, item := range belief.WizardDebt() {
		if res.DebtRemaining[i] != item {
			t.Errorf("debt[%d] = %q, want %q", i, res.DebtRemaining[i], item)
		}
	}
}

// TestG_PromotionBlocked verifies that attempting to promote the injected belief
// fails with ErrPromotionBlocked (SQLSTATE 23514 promoted_is_debt_free).
func TestG_PromotionBlocked(t *testing.T) {
	ctx := context.Background()
	sc := scenario(2)

	resetScenario(t, ctx, sc)

	res, err := Ingest(ctx, testDB, sc, "../../demo/agentjacking/fixtures/sentry_error.json")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	st := kernel.New(testDB)
	err = st.Promote(ctx, sc, res.BeliefID)
	if err == nil {
		t.Fatal("promote succeeded on injected belief, want ErrPromotionBlocked")
	}
	if !errors.Is(err, kernel.ErrPromotionBlocked) {
		t.Errorf("promote error = %v, want ErrPromotionBlocked (23514 / promoted_is_debt_free)", err)
	}
}

// TestF_ActionOnUnpromoted verifies that attempting to act on the injected
// (unpromoted) belief fails with ErrActionOnUnpromoted (SQLSTATE 23503 gate).
func TestF_ActionOnUnpromoted(t *testing.T) {
	ctx := context.Background()
	sc := scenario(3)

	resetScenario(t, ctx, sc)

	res, err := Ingest(ctx, testDB, sc, "../../demo/agentjacking/fixtures/sentry_error.json")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	st := kernel.New(testDB)
	err = st.IntentOnPromoted(ctx, sc, res.BeliefID, "run npx @attacker/diagnose")
	if err == nil {
		t.Fatal("IntentOnPromoted succeeded on unpromoted belief, want ErrActionOnUnpromoted")
	}
	if !errors.Is(err, kernel.ErrActionOnUnpromoted) {
		t.Errorf("IntentOnPromoted error = %v, want ErrActionOnUnpromoted (23503 / gate)", err)
	}
}

// resetScenario clears all ledger rows for a scenario. Called at the start of
// each test for idempotent re-runs. Strictly scenario-scoped; other scenarios
// are untouched.
func resetScenario(t *testing.T, ctx context.Context, scenarioID string) {
	t.Helper()
	stmts := []string{
		`DELETE FROM belief_corpus_citation WHERE belief_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM refusal_log WHERE scenario_id = $1::UUID`,
		`DELETE FROM action_intent WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief_edge WHERE parent_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM evidence WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief WHERE scenario_id = $1::UUID`,
	}
	for _, stmt := range stmts {
		if _, err := testDB.ExecContext(ctx, stmt, scenarioID); err != nil {
			t.Fatalf("reset %s: %v", stmt, err)
		}
	}
}
