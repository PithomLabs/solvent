package kernel_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/PithomLabs/solvent/kernel"
)

// exampleScenario is unique to this file. The suite's other scenarios occupy
// 11111111.. through 66666666.. (plus 10000000-..-0001 and 23000000-..-0001), and
// operator_review_test.go already shares the kernel suite's namespace; a fresh one
// keeps the example's audit counts its own.
const exampleScenario = "77777777-7777-7777-7777-777777777777"

// Example_lifecycle walks the complete ledger lifecycle against a live database:
//
//	evidence → belief → debt → promotion → intent → falsification → retraction → cancellation
//
// The two refusals in the middle are the point. A belief carrying debt cannot be
// promoted, and an action cannot cite a belief that is not promoted — neither is
// decided by this Go code. The database refuses, and the kernel only names the
// refusal. Everything printed below is read back from CockroachDB, never asserted
// from memory.
func Example_lifecycle() {
	ctx := context.Background()
	st := kernel.New(shared)

	// --- evidence → belief ----------------------------------------------------
	// A claim enters free: status 'entered', carrying the full starting debt.
	// Ideas are never gated at the door; only actions are.
	beliefID, err := st.EnterBelief(ctx, exampleScenario,
		"etcd v3.5.0 is approved for production deployment", kernel.Postulated)
	if err != nil {
		fmt.Println("EnterBelief:", err)
		return
	}
	if err := st.AddEvidence(ctx, exampleScenario, beliefID,
		"external_feed",
		"https://github.com/etcd-io/etcd/releases/tag/v3.5.0",
		"f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1",
	); err != nil {
		fmt.Println("AddEvidence:", err)
		return
	}
	fmt.Printf("entered:   status=%s debt=%d evidence=%d\n",
		exampleStatus(beliefID), exampleDebt(beliefID), exampleEvidence(beliefID))

	// --- the debt gate (I-1) --------------------------------------------------
	// Refused by CHECK promoted_is_debt_free. No debt is inspected in Go.
	err = st.Promote(ctx, beliefID)
	fmt.Printf("promote:   %s sqlstate=%s constraint=%s\n",
		exampleSentinel(err, kernel.ErrPromotionBlocked, "ErrPromotionBlocked"),
		sqlStateOf(err), constraintOf(err))

	// --- the action gate (I-3) ------------------------------------------------
	// Refused by the composite FK "gate". The row is not rejected by a rule; with
	// the belief unpromoted it is unrepresentable.
	err = st.IntentOnPromoted(ctx, exampleScenario, beliefID, "deploy etcd v3.5.0")
	fmt.Printf("intent:    %s sqlstate=%s constraint=%s\n",
		exampleSentinel(err, kernel.ErrActionOnUnpromoted, "ErrActionOnUnpromoted"),
		sqlStateOf(err), constraintOf(err))

	// --- debt → promotion -----------------------------------------------------
	// Review discharges each obligation. Nothing about the claim changed; only its
	// epistemic standing did.
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, beliefID, item); err != nil {
			fmt.Println("RetireDebt:", err)
			return
		}
	}
	if err := st.Promote(ctx, beliefID); err != nil {
		fmt.Println("Promote:", err)
		return
	}
	fmt.Printf("promoted:  status=%s debt=%d\n",
		exampleStatus(beliefID), exampleDebt(beliefID))

	// --- intent ---------------------------------------------------------------
	// The identical statement that was refused above now succeeds.
	if err := st.IntentOnPromoted(ctx, exampleScenario, beliefID, "deploy etcd v3.5.0"); err != nil {
		fmt.Println("IntentOnPromoted:", err)
		return
	}
	fmt.Printf("authorized: intent=%s\n", exampleIntent(beliefID))

	// --- falsification → retraction → cancellation ----------------------------
	// New evidence contradicts the belief. One transaction: live intents are
	// cancelled FIRST, then the belief and its descendants are retracted (I-8).
	retracted, err := st.RetractCascade(ctx, exampleScenario, beliefID)
	if err != nil {
		fmt.Println("RetractCascade:", err)
		return
	}
	fmt.Printf("retracted: beliefs=%d status=%s intent=%s\n",
		retracted, exampleStatus(beliefID), exampleIntent(beliefID))

	// --- audit (I-5) ----------------------------------------------------------
	// Live intents resting on non-promoted beliefs. Must be 0 in every committed
	// state — the dependent action did not survive the belief it rested on.
	n, err := st.AuditLiveOnNonPromoted(ctx, exampleScenario)
	if err != nil {
		fmt.Println("Audit:", err)
		return
	}
	fmt.Printf("audit:     live_on_nonpromoted=%d\n", n)

	// Output:
	// entered:   status=entered debt=6 evidence=1
	// promote:   ErrPromotionBlocked sqlstate=23514 constraint=promoted_is_debt_free
	// intent:    ErrActionOnUnpromoted sqlstate=23503 constraint=gate
	// promoted:  status=promoted debt=0
	// authorized: intent=live
	// retracted: beliefs=1 status=retracted intent=cancelled
	// audit:     live_on_nonpromoted=0
}

// --- read-back helpers --------------------------------------------------------
//
// The example prints only what it reads back from the database. UUIDs and
// timestamps are never printed, so the output is deterministic.

func exampleSentinel(err error, want error, name string) string {
	if errors.Is(err, want) {
		return name
	}
	if err == nil {
		return "UNEXPECTEDLY ALLOWED"
	}
	return "unexpected error: " + err.Error()
}

func exampleStatus(beliefID string) string {
	var s string
	if err := shared.QueryRow(
		`SELECT status FROM belief WHERE id = $1::UUID`, beliefID).Scan(&s); err != nil {
		return "?"
	}
	return s
}

func exampleDebt(beliefID string) int {
	var n int
	if err := shared.QueryRow(
		`SELECT coalesce(array_length(debt,1),0) FROM belief WHERE id = $1::UUID`,
		beliefID).Scan(&n); err != nil {
		return -1
	}
	return n
}

func exampleEvidence(beliefID string) int {
	var n int
	if err := shared.QueryRow(
		`SELECT count(*) FROM evidence WHERE belief_id = $1::UUID`, beliefID).Scan(&n); err != nil {
		return -1
	}
	return n
}

func exampleIntent(beliefID string) string {
	var s string
	if err := shared.QueryRow(
		`SELECT state FROM action_intent WHERE belief_id = $1::UUID`, beliefID).Scan(&s); err != nil {
		return "none"
	}
	return s
}
