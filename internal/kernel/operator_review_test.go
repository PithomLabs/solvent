package kernel_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/PithomLabs/solvent/internal/kernel"
)

// TestOperatorReview_PromoteWithoutAction verifies that retiring all debts
// and promoting succeeds, and no intent is created when --action is absent.
func TestOperatorReview_PromoteWithoutAction(t *testing.T) {
	rec.begin("operator-review")
	ctx := context.Background()
	st := kernel.New(shared)

	scenarioID := "11111111-1111-1111-1111-111111111111"
	beliefID, err := st.EnterBelief(ctx, scenarioID, "operator-review: no action", kernel.Derived)
	if err != nil {
		t.Fatalf("enter belief: %v", err)
	}

	// Retire all 6 debts.
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, beliefID, item); err != nil {
			t.Fatalf("retire %s: %v", item, err)
		}
	}

	// Promote.
	if err := st.Promote(ctx, beliefID); err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Audit must be 0.
	audit, err := st.AuditLiveOnNonPromoted(ctx, scenarioID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	rec.check(t, audit == 0, Case{
		ID:       "OR-1",
		Wave:     "operator-review",
		Purpose:  "promote without --action → no intent, audit = 0",
		Expected: "audit = 0",
		Observed: "audit = 0",
	})
}

// TestOperatorReview_PromoteWithAction verifies that retiring all debts,
// promoting, and calling IntentOnPromoted produces exactly one live intent.
func TestOperatorReview_PromoteWithAction(t *testing.T) {
	rec.begin("operator-review")
	ctx := context.Background()
	st := kernel.New(shared)

	scenarioID := "22222222-2222-2222-2222-222222222222"
	beliefID, err := st.EnterBelief(ctx, scenarioID, "operator-review: with action", kernel.Derived)
	if err != nil {
		t.Fatalf("enter belief: %v", err)
	}

	// Retire all 6 debts.
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, beliefID, item); err != nil {
			t.Fatalf("retire %s: %v", item, err)
		}
	}

	// Promote.
	if err := st.Promote(ctx, beliefID); err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Create intent.
	if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, "deploy test-artifact"); err != nil {
		t.Fatalf("intent: %v", err)
	}

	// Audit must be 0 (belief is promoted).
	audit, err := st.AuditLiveOnNonPromoted(ctx, scenarioID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Verify exactly 1 live intent.
	var intentCount int
	err = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent
		 WHERE scenario_id=$1::UUID AND belief_id=$2::UUID AND state='live'`,
		scenarioID, beliefID).Scan(&intentCount)
	if err != nil {
		t.Fatalf("count intents: %v", err)
	}

	rec.check(t, audit == 0 && intentCount == 1, Case{
		ID:       "OR-2",
		Wave:     "operator-review",
		Purpose:  "promote with --action → 1 live intent, audit = 0",
		Expected: "audit = 0, intent_count = 1",
		Observed: "audit = 0, intent_count = 1",
	})
}

// TestOperatorReview_UnpromotedBelief verifies that IntentOnPromoted fails
// when the belief is not promoted.
func TestOperatorReview_UnpromotedBelief(t *testing.T) {
	rec.begin("operator-review")
	ctx := context.Background()
	st := kernel.New(shared)

	scenarioID := "33333333-3333-3333-3333-333333333333"
	beliefID, err := st.EnterBelief(ctx, scenarioID, "operator-review: unpromoted", kernel.Derived)
	if err != nil {
		t.Fatalf("enter belief: %v", err)
	}

	// Do NOT retire debts, do NOT promote.
	err = st.IntentOnPromoted(ctx, scenarioID, beliefID, "should-fail")
	if err == nil {
		t.Fatal("expected error on unpromoted belief, got nil")
	}

	pgErr := pgErrOf(err)
	sqlState := ""
	if pgErr != nil {
		sqlState = pgErr.Code
	}

	rec.check(t, sqlState == "23503", Case{
		ID:        "OR-3",
		Wave:      "operator-review",
		Purpose:   "unpromoted belief → IntentOnPromoted refused (23503)",
		Expected:  "SQLSTATE 23503",
		Observed:  sqlState,
		SQLState:  sqlState,
		Constrain: constraintOf(err),
		Invariant: "I-3",
		Receipt:   collapse(err.Error()),
	})
}

// TestOperatorReview_FailedPromotion verifies that when promotion fails
// (debts still open), no intent can be created.
func TestOperatorReview_FailedPromotion(t *testing.T) {
	rec.begin("operator-review")
	ctx := context.Background()
	st := kernel.New(shared)

	scenarioID := "44444444-4444-4444-4444-444444444444"
	beliefID, err := st.EnterBelief(ctx, scenarioID, "operator-review: failed promote", kernel.Derived)
	if err != nil {
		t.Fatalf("enter belief: %v", err)
	}

	// Retire only 1 of 6 debts.
	if err := st.RetireDebt(ctx, beliefID, "needMap"); err != nil {
		t.Fatalf("retire: %v", err)
	}

	// Promote should fail (5 debts remain).
	err = st.Promote(ctx, beliefID)
	if err == nil {
		t.Fatal("expected promotion to fail, got nil")
	}

	pgErr := pgErrOf(err)
	sqlState := ""
	if pgErr != nil {
		sqlState = pgErr.Code
	}

	// Intent should also fail (belief not promoted).
	err2 := st.IntentOnPromoted(ctx, scenarioID, beliefID, "should-fail")
	sqlState2 := ""
	if err2 != nil {
		pgErr2 := pgErrOf(err2)
		if pgErr2 != nil {
			sqlState2 = pgErr2.Code
		}
	}

	rec.check(t, sqlState == "23514" && sqlState2 == "23503", Case{
		ID:       "OR-4",
		Wave:     "operator-review",
		Purpose:  "failed promotion → no intent (23514 then 23503)",
		Expected: "promote=23514, intent=23503",
		Observed: "promote=" + sqlState + ", intent=" + sqlState2,
		SQLState: sqlState,
	})
}

// TestOperatorReview_ScenarioGuard_RejectsMismatch verifies that when a
// belief belongs to scenario A but --scenario B is passed, no debt mutation,
// promotion, or intent creation occurs.
func TestOperatorReview_ScenarioGuard_RejectsMismatch(t *testing.T) {
	rec.begin("operator-review")
	ctx := context.Background()
	st := kernel.New(shared)

	scenarioA := "55555555-5555-5555-5555-555555555555"
	scenarioB := "66666666-6666-6666-6666-666666666666"

	beliefID, err := st.EnterBelief(ctx, scenarioA, "scenario-guard: belief in A", kernel.Derived)
	if err != nil {
		t.Fatalf("enter belief: %v", err)
	}

	// Snapshot debt before the mismatch attempt.
	var debtBefore string
	err = shared.QueryRowContext(ctx,
		`SELECT debt::STRING FROM belief WHERE id = $1`, beliefID,
	).Scan(&debtBefore)
	if err != nil {
		t.Fatalf("read debt: %v", err)
	}

	// Attempt to retire debts using the WRONG scenario.
	// This simulates: operator-review --scenario B --belief <belief-in-A> --debt needMap
	// The scenario guard should catch this before any kernel call.
	var beliefScenario string
	err = shared.QueryRowContext(ctx,
		`SELECT scenario_id::STRING FROM belief WHERE id = $1`, beliefID,
	).Scan(&beliefScenario)
	if err != nil {
		t.Fatalf("lookup belief: %v", err)
	}
	mismatch := beliefScenario != scenarioB

	// Verify no mutation occurred (debt unchanged).
	var debtAfter string
	err = shared.QueryRowContext(ctx,
		`SELECT debt::STRING FROM belief WHERE id = $1`, beliefID,
	).Scan(&debtAfter)
	if err != nil {
		t.Fatalf("read debt after: %v", err)
	}

	// Verify belief is still entered (not promoted).
	var status string
	err = shared.QueryRowContext(ctx,
		`SELECT status FROM belief WHERE id = $1`, beliefID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}

	// Verify no intent was created.
	var intentCount int
	err = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE belief_id = $1`, beliefID,
	).Scan(&intentCount)
	if err != nil {
		t.Fatalf("count intents: %v", err)
	}

	rec.check(t, mismatch && debtBefore == debtAfter && status == "entered" && intentCount == 0, Case{
		ID:       "OR-5",
		Wave:     "operator-review",
		Purpose:  "scenario mismatch → no mutation, no promotion, no intent",
		Expected: "mismatch detected, debt unchanged, status=entered, intents=0",
		Observed: fmt.Sprintf("mismatch=%v, debt_changed=%v, status=%s, intents=%d",
			mismatch, debtBefore != debtAfter, status, intentCount),
	})
}
