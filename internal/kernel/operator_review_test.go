package kernel_test

import (
	"context"
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
