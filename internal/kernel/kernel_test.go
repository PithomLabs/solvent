package kernel_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/testdb"
)

// Fixed scenario UUIDs, one per case, so cases cannot interfere and the transcript is
// reproducible. No gen_random_uuid() anywhere in the harness.
func scenario(n int) string {
	return fmt.Sprintf("11111111-0000-0000-0000-%012x", n)
}

// ---------------------------------------------------------------- Wave 0

// TestW0_SchemaPresent is the harness smoke test: the reset ran, the frozen DDL
// applied, and the four contracted tables exist.
func TestW0_SchemaPresent(t *testing.T) {
	rec.begin("0")
	ctx := context.Background()

	var n int
	err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema='public' AND table_type='BASE TABLE'
		  AND table_name IN ('belief','belief_edge','evidence','action_intent')`).Scan(&n)

	rec.check(t, err == nil && n == 4, Case{
		ID: "W0", Wave: "0",
		Purpose:  "The behavioral database was reset and the frozen DDL applied",
		Expected: "4 contracted tables present",
		Observed: fmt.Sprintf("%d tables present", n),
		Receipt:  receiptOf(err),
	})
}

// ---------------------------------------------------------------- Wave 1

// B-01 / B-17 / B-23: EnterBelief, and the two encoding questions it carries.
func TestW1_B01_EnterBelief(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(1)

	id, err := st.EnterBelief(ctx, sc, "a claim at the door", kernel.Accommodated)

	var status string
	var finalTruth bool
	var debtLen int
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT status, final_truth, coalesce(array_length(debt,1),0) FROM belief WHERE id=$1::UUID`,
			id).Scan(&status, &finalTruth, &debtLen)
	}

	ok := err == nil && looksLikeUUID(id) && status == "entered" && !finalTruth && debtLen == 6
	rec.check(t, ok, Case{
		ID: "B-01", Wave: "1",
		Purpose:   "A claim enters unpromoted, carrying its full starting debt",
		Expected:  "parseable UUID returned; status='entered', final_truth=false, 6 debt items",
		Observed:  fmt.Sprintf("id parseable=%t, status=%q, final_truth=%t, debt items=%d", looksLikeUUID(id), status, finalTruth, debtLen),
		Invariant: "contract §4 EnterBelief — never gated, full debt at the door",
		Receipt:   receiptOf(err),
	})
}

// B-17 discharges D10: pgx encoding a Go []string into STRING[]. M1 could only
// confirm the expected parameter type; nothing had ever executed.
func TestW1_B17_DebtEncoding(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(17)

	id, err := st.EnterBelief(ctx, sc, "claim whose debt encoding is under test", kernel.Derived)

	var stored string
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, id).Scan(&stored)
	}
	want := strings.Join(kernel.FullDebt, ",")

	rec.check(t, err == nil && stored == want, Case{
		ID: "B-17", Wave: "1",
		Purpose:   "Discharge D10 — a Go []string encodes into STRING[] element-for-element, in order",
		Expected:  "stored debt == kernel.FullDebt: " + want,
		Observed:  "stored debt == " + stored,
		Invariant: "M1-R2 / D10 — the last open encoding assumption",
		Receipt:   receiptOf(err),
	})
}

// B-23 discharges M1-R3: the Go constant and the DDL DEFAULT must agree.
func TestW1_B23_DebtDefaultDrift(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	sc := scenario(23)
	id := "23000000-0000-0000-0000-000000000001"

	// Raw insert WITHOUT a debt list, so the column DEFAULT supplies it.
	_, err := shared.ExecContext(ctx, `
		INSERT INTO belief (id, scenario_id, claim, claim_type)
		VALUES ($1::UUID, $2::UUID, 'claim relying on the DDL default', 'postulated')`, id, sc)

	var stored string
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, id).Scan(&stored)
	}
	want := strings.Join(kernel.FullDebt, ",")

	rec.check(t, err == nil && stored == want, Case{
		ID: "B-23", Wave: "1",
		Purpose:   "Discharge M1-R3 — kernel.FullDebt (Go) and the ARRAY[...] DEFAULT (DDL) have not drifted",
		Expected:  "DDL default == kernel.FullDebt: " + want,
		Observed:  "DDL default == " + stored,
		Invariant: "M1-R3 — the six debt items are encoded in two places",
		Receipt:   receiptOf(err),
	})
}

func TestW1_B02_AddEvidence(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(2)

	id, err := st.EnterBelief(ctx, sc, "claim awaiting evidence", kernel.Accommodated)
	beforeStatus, beforeDebt := beliefState(t, ctx, id)

	if err == nil {
		err = st.AddEvidence(ctx, sc, id, "external_feed",
			"https://example.invalid/feed/item", "deadbeefcafe")
	}

	var n int
	var sha string
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT count(*), coalesce(max(content_sha256),'') FROM evidence
			 WHERE belief_id=$1::UUID AND provenance_class='external_feed'`, id).Scan(&n, &sha)
	}
	afterStatus, afterDebt := beliefState(t, ctx, id)

	ok := err == nil && n == 1 && sha == "deadbeefcafe" &&
		beforeStatus == afterStatus && beforeDebt == afterDebt
	rec.check(t, ok, Case{
		ID: "B-02", Wave: "1",
		Purpose:   "Evidence attaches without changing belief state",
		Expected:  "1 evidence row with the given sha; belief status and debt unchanged",
		Observed:  fmt.Sprintf("%d evidence row(s), sha=%q; status %q→%q, debt unchanged=%t", n, sha, beforeStatus, afterStatus, beforeDebt == afterDebt),
		Invariant: "contract §4 AddEvidence — does not change belief state",
		Receipt:   receiptOf(err),
	})
}

func TestW1_B03_RetireDebt(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(3)

	id, err := st.EnterBelief(ctx, sc, "claim retiring one debt", kernel.Derived)
	if err == nil {
		err = st.RetireDebt(ctx, id, "needToyCheck")
	}

	var stored string
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, id).Scan(&stored)
	}
	want := strings.Join(without(kernel.FullDebt, "needToyCheck"), ",")

	rec.check(t, err == nil && stored == want, Case{
		ID: "B-03", Wave: "1",
		Purpose:   "One useful move retires exactly one debt; the rest survive in order",
		Expected:  "debt == " + want,
		Observed:  "debt == " + stored,
		Invariant: "contract §4 RetireDebt",
		Receipt:   receiptOf(err),
	})
}

func TestW1_B04_RetireDebtIdempotent(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(4)

	id, err := st.EnterBelief(ctx, sc, "claim retiring an absent debt", kernel.Derived)
	if err == nil {
		err = st.RetireDebt(ctx, id, "needToyCheck")
	}
	before := debtString(t, ctx, id)

	// Same item again: absent now, must be a no-op rather than an error.
	var secondErr error
	if err == nil {
		secondErr = st.RetireDebt(ctx, id, "needToyCheck")
	}
	after := debtString(t, ctx, id)

	rec.check(t, err == nil && secondErr == nil && before == after, Case{
		ID: "B-04", Wave: "1",
		Purpose:   "Retiring an already-absent debt item is a no-op, not an error",
		Expected:  "second call returns nil; debt unchanged",
		Observed:  fmt.Sprintf("second call err=%v; debt unchanged=%t", secondErr, before == after),
		Invariant: "contract §4 RetireDebt — idempotent if the item is absent",
		Receipt:   receiptOf(secondErr),
	})
}

func TestW1_B05_Promote(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(5)

	id, err := st.EnterBelief(ctx, sc, "claim whose debt is fully retired", kernel.Derived)
	if err == nil {
		err = retireAll(ctx, st, id)
	}
	if err == nil {
		err = st.Promote(ctx, id)
	}
	status, _ := beliefState(t, ctx, id)

	rec.check(t, err == nil && status == "promoted", Case{
		ID: "B-05", Wave: "1",
		Purpose:   "A debt-free, non-final belief reaches the throne",
		Expected:  "Promote returns nil; status='promoted'",
		Observed:  fmt.Sprintf("err=%v; status=%q", err, status),
		Invariant: "contract §4 Promote",
		Receipt:   receiptOf(err),
	})
}

// B-09 is I-1: promotion with open debt is unrepresentable.
func TestW1_B09_I1_PromoteWithDebt(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(9)

	id, setupErr := st.EnterBelief(ctx, sc, "claim still carrying debt", kernel.Accommodated)
	if setupErr != nil {
		t.Fatalf("B-09 setup: %v", setupErr)
	}

	err := st.Promote(ctx, id)
	status, _ := beliefState(t, ctx, id)

	isSentinel := errors.Is(err, kernel.ErrPromotionBlocked)
	pgErr := pgErrOf(err)
	ok := err != nil && isSentinel && pgErr != nil &&
		pgErr.Code == "23514" && pgErr.ConstraintName == "promoted_is_debt_free" &&
		status == "entered"

	rec.check(t, ok, Case{
		ID: "B-09", Wave: "1",
		Purpose:   "I-1 — no belief with status='promoted' has non-empty debt",
		Expected:  "refused; errors.Is(ErrPromotionBlocked) AND errors.As(*pgconn.PgError); 23514 / promoted_is_debt_free; belief still 'entered'",
		Observed:  describeRefusal(err, isSentinel, status),
		SQLState:  sqlStateOf(err),
		Constrain: constraintOf(err),
		Invariant: "I-1 — no belief with status='promoted' has non-empty debt",
		Receipt:   receiptOf(err),
	})
}

// B-10 is I-2: final-truth language blocks promotion.
func TestW1_B10_I2_PromoteFinalTruth(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(10)
	id := "10000000-0000-0000-0000-000000000001"

	// Debt-free but final-truth: isolates the I-2 arm of the CHECK from the I-1 arm.
	if _, err := shared.ExecContext(ctx, `
		INSERT INTO belief (id, scenario_id, claim, claim_type, debt, final_truth)
		VALUES ($1::UUID, $2::UUID, 'claim asserted as settled and final', 'postulated', '{}', true)`,
		id, sc); err != nil {
		t.Fatalf("B-10 setup: %v", err)
	}

	err := st.Promote(ctx, id)
	status, _ := beliefState(t, ctx, id)

	isSentinel := errors.Is(err, kernel.ErrPromotionBlocked)
	pgErr := pgErrOf(err)
	ok := err != nil && isSentinel && pgErr != nil &&
		pgErr.Code == "23514" && pgErr.ConstraintName == "promoted_is_debt_free" &&
		status == "entered"

	rec.check(t, ok, Case{
		ID: "B-10", Wave: "1",
		Purpose:   "I-2 — no belief with status='promoted' has final_truth=true",
		Expected:  "refused; errors.Is AND errors.As both succeed; 23514 / promoted_is_debt_free; belief still 'entered'",
		Observed:  describeRefusal(err, isSentinel, status),
		SQLState:  sqlStateOf(err),
		Constrain: constraintOf(err),
		Invariant: "I-2 — no belief with status='promoted' has final_truth=true",
		Receipt:   receiptOf(err),
	})
}

// B-18 (D-048): retry classification, promoted into Wave 1 so the highest-risk
// CockroachDB assumption is retired before the rest of the suite depends on it.
//
// inject_retry_errors_enabled makes every statement inside an explicit transaction
// return 40001; the transaction proceeds normally after the 3rd retry IF the client
// retries via SAVEPOINT cockroach_restart, which is exactly what crdb.ExecuteTx does.
// Success here therefore proves three things at once: 40001 was classified as
// retryable, ExecuteTx retried, and the sentinel wrap never masked it.
func TestW1_B18_RetryClassification(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	sc := scenario(18)

	// Seed through the ordinary pool: a debt-free belief that Promote must accept.
	seed := kernel.New(shared)
	id, err := seed.EnterBelief(ctx, sc, "claim promoted under injected retries", kernel.Derived)
	if err == nil {
		err = retireAll(ctx, seed, id)
	}
	if err != nil {
		t.Fatalf("B-18 setup: %v", err)
	}

	// Injection lives in a SEPARATE pool's DSN, never as a SET on the shared pool
	// (M2-R2). application_name lets the retry count be attributed afterwards.
	const appName = "m2_b18_retry"
	injDSN, err := testdb.WithOptions(dsn,
		map[string]string{"inject_retry_errors_enabled": "true"}, appName)
	if err != nil {
		t.Fatalf("B-18 dsn: %v", err)
	}
	injDB, err := testdb.Open(injDSN)
	if err != nil {
		t.Fatalf("B-18 open: %v", err)
	}
	defer func() { _ = injDB.Close() }()

	// CONTROL: prove the injection is actually live before concluding anything from a
	// success. Without this, a DSN option that silently failed to apply would make this
	// case pass vacuously — the test would "prove" retry handling while nothing retried.
	injectionOn, controlState := injectionControl(ctx, injDB)

	promoteErr := kernel.New(injDB).Promote(ctx, id)
	status, _ := beliefState(t, ctx, id)

	// N2: read the retry count from existing instrumentation only. No instrumentation
	// is invented; when the metric is unreachable the transcript records why.
	retries := retryCount(ctx, appName)

	wrongSentinel := errors.Is(promoteErr, kernel.ErrPromotionBlocked)
	ok := injectionOn && controlState == "40001" &&
		promoteErr == nil && status == "promoted" && !wrongSentinel

	rec.check(t, ok, Case{
		ID: "B-18", Wave: "1",
		Purpose: "40001 is classified as retryable, is retried by crdb.ExecuteTx, and is never masked by the sentinel wrap",
		Expected: "injection live (SHOW=on, control txn gets 40001); Promote returns nil anyway; " +
			"status='promoted'; error is NOT ErrPromotionBlocked",
		Observed: fmt.Sprintf("injection=%t, control txn sqlstate=%q; Promote err=%v; status=%q; ErrPromotionBlocked=%t; retry_count=%s",
			injectionOn, controlState, promoteErr, status, wrongSentinel, retries),
		SQLState:  sqlStateOf(promoteErr),
		Invariant: "I-7's purpose — serialization failures retry rather than surface",
		Receipt:   receiptOf(promoteErr),
	})
}

// injectionControl proves inject_retry_errors_enabled is genuinely in force on this
// pool: the session variable reads "on", and a plain transaction that does NOT use
// crdb.ExecuteTx receives 40001. Only then does B-18's success mean anything.
func injectionControl(ctx context.Context, db *sql.DB) (on bool, controlSQLState string) {
	var v string
	if err := db.QueryRowContext(ctx, "SHOW inject_retry_errors_enabled").Scan(&v); err != nil {
		return false, "SHOW failed: " + err.Error()
	}
	on = v == "on" || v == "true"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return on, "BeginTx failed: " + err.Error()
	}
	defer func() { _ = tx.Rollback() }()

	var one int
	err = tx.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	if err == nil {
		return on, "none — no error was injected"
	}
	return on, sqlStateOf(err)
}

func TestW1_B06_IntentOnPromoted(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(6)

	id, err := promotedBelief(ctx, st, sc, "promoted belief accepting an intent")
	if err == nil {
		err = st.IntentOnPromoted(ctx, sc, id, "defer_patch")
	}

	var state, beliefStatus string
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT state, belief_status FROM action_intent WHERE belief_id=$1::UUID`, id).
			Scan(&state, &beliefStatus)
	}

	rec.check(t, err == nil && state == "live" && beliefStatus == "promoted", Case{
		ID: "B-06", Wave: "1",
		Purpose:   "An action intent citing a promoted belief is accepted",
		Expected:  "returns nil; intent state='live', belief_status='promoted'",
		Observed:  fmt.Sprintf("err=%v; state=%q, belief_status=%q", err, state, beliefStatus),
		Invariant: "contract §4 IntentOnPromoted",
		Receipt:   receiptOf(err),
	})
}

// B-11 is I-3: the composite FK physically refuses an intent on an unpromoted belief.
func TestW1_B11_I3_IntentOnUnpromoted(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(11)

	id, setupErr := st.EnterBelief(ctx, sc, "claim that never left the door", kernel.Postulated)
	if setupErr != nil {
		t.Fatalf("B-11 setup: %v", setupErr)
	}

	err := st.IntentOnPromoted(ctx, sc, id, "defer_patch")

	var n int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE belief_id=$1::UUID`, id).Scan(&n)

	isSentinel := errors.Is(err, kernel.ErrActionOnUnpromoted)
	pgErr := pgErrOf(err)
	ok := err != nil && isSentinel && pgErr != nil &&
		pgErr.Code == "23503" && pgErr.ConstraintName == "gate" && n == 0

	rec.check(t, ok, Case{
		ID: "B-11", Wave: "1",
		Purpose:   "I-3 — no live action_intent references a belief that is not promoted",
		Expected:  "refused; errors.Is(ErrActionOnUnpromoted) AND errors.As; 23503 / gate; 0 intent rows",
		Observed:  fmt.Sprintf("err sentinel=%t; sqlstate=%q; constraint=%q; intent rows=%d", isSentinel, sqlStateOf(err), constraintOf(err), n),
		SQLState:  sqlStateOf(err),
		Constrain: constraintOf(err),
		Invariant: "I-3 — no live intent references a non-promoted belief",
		Receipt:   receiptOf(err),
	})
}

// B-12 is I-4: skipping the cancel is refused by the schema. Not reachable through
// the kernel, which always cancels first, so the test issues the raw UPDATE that a
// careless caller might.
func TestW1_B12_I4_RetractSkippingCancel(t *testing.T) {
	rec.begin("1")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(12)

	id, err := promotedBelief(ctx, st, sc, "promoted belief carrying a live intent")
	if err == nil {
		err = st.IntentOnPromoted(ctx, sc, id, "defer_patch")
	}
	if err != nil {
		t.Fatalf("B-12 setup: %v", err)
	}

	_, updErr := shared.ExecContext(ctx,
		`UPDATE belief SET status='retracted' WHERE id=$1::UUID`, id)
	status, _ := beliefState(t, ctx, id)

	pgErr := pgErrOf(updErr)
	ok := updErr != nil && pgErr != nil &&
		pgErr.Code == "23514" && pgErr.ConstraintName == "live_requires_promoted" &&
		status == "promoted"

	rec.check(t, ok, Case{
		ID: "B-12", Wave: "1",
		Purpose:   "I-4 — a belief carrying a live intent cannot be retracted without the intent being cancelled first",
		Expected:  "raw UPDATE refused with 23514 / live_requires_promoted; belief still 'promoted'",
		Observed:  fmt.Sprintf("sqlstate=%q; constraint=%q; status=%q", sqlStateOf(updErr), constraintOf(updErr), status),
		SQLState:  sqlStateOf(updErr),
		Constrain: constraintOf(updErr),
		Invariant: "I-4 — cancel must precede retract",
		Receipt:   receiptOf(updErr),
	})
}

// ---------------------------------------------------------------- Wave 2

func TestW2_B07_RetractCascade(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(7)

	root, child := chain(t, ctx, st, sc, "cascade root", "cascade child")
	if err := st.IntentOnPromoted(ctx, sc, child, "defer_patch"); err != nil {
		t.Fatalf("B-07 setup: %v", err)
	}

	n, err := st.RetractCascade(ctx, sc, root)

	rootStatus, _ := beliefState(t, ctx, root)
	childStatus, _ := beliefState(t, ctx, child)
	var intentState, intentBeliefStatus string
	_ = shared.QueryRowContext(ctx,
		`SELECT state, belief_status FROM action_intent WHERE belief_id=$1::UUID`, child).
		Scan(&intentState, &intentBeliefStatus)

	ok := err == nil && n == 2 &&
		rootStatus == "retracted" && childStatus == "retracted" &&
		intentState == "cancelled" && intentBeliefStatus == "retracted"

	rec.check(t, ok, Case{
		ID: "B-07", Wave: "2",
		Purpose:   "Root and descendants un-promote; live intents cancel; ON UPDATE CASCADE propagates the new status",
		Expected:  "returns 2; both beliefs 'retracted'; intent 'cancelled' with belief_status='retracted'",
		Observed:  fmt.Sprintf("returned %d; root=%q child=%q; intent=%q/%q", n, rootStatus, childStatus, intentState, intentBeliefStatus),
		Invariant: "contract §4 RetractCascade",
		Receipt:   receiptOf(err),
	})
}

func TestW2_B20_CascadeChain(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(20)

	c1 := mustPromoted(t, ctx, st, sc, "chain link 1")
	c2 := mustPromoted(t, ctx, st, sc, "chain link 2")
	c3 := mustPromoted(t, ctx, st, sc, "chain link 3")
	c9 := mustPromoted(t, ctx, st, sc, "unrelated belief")
	edge(t, ctx, c1, c2)
	edge(t, ctx, c2, c3)

	n, err := st.RetractCascade(ctx, sc, c1)
	unrelated, _ := beliefState(t, ctx, c9)

	rec.check(t, err == nil && n == 3 && unrelated == "promoted", Case{
		ID: "B-20", Wave: "2",
		Purpose:   "The recursive traversal collects the whole chain and nothing else",
		Expected:  "returns 3; the unrelated belief stays 'promoted'",
		Observed:  fmt.Sprintf("returned %d; unrelated belief=%q", n, unrelated),
		Invariant: "D-033 — WITH RECURSIVE collects transitive descendants",
		Receipt:   receiptOf(err),
	})
}

func TestW2_B22_RowsAffectedSemantics(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(22)

	root, child := chain(t, ctx, st, sc, "counted root", "counted child")
	// Three live intents across two beliefs: the return value must not count these.
	for _, spec := range []struct{ belief, action string }{
		{root, "defer_patch"}, {child, "defer_patch"}, {child, "notify_owner"},
	} {
		if err := st.IntentOnPromoted(ctx, sc, spec.belief, spec.action); err != nil {
			t.Fatalf("B-22 setup: %v", err)
		}
	}

	n, err := st.RetractCascade(ctx, sc, root)

	var cancelled int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='cancelled'`, sc).
		Scan(&cancelled)

	rec.check(t, err == nil && n == 2 && cancelled == 3, Case{
		ID: "B-22", Wave: "2",
		Purpose:   "N3 — retracted counts belief rows only, never cancelled intents",
		Expected:  "returns 2 (not 3, not 5); 3 intents cancelled",
		Observed:  fmt.Sprintf("returned %d; %d intents cancelled", n, cancelled),
		Invariant: "contract §4 RetractCascade — RowsAffected of the belief UPDATE only",
		Receipt:   receiptOf(err),
	})
}

// B-19 discharges D12: the scoped traversal must not follow an edge out of its scenario.
func TestW2_B19_CrossScenarioIsolation(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	scA, scB := scenario(19), scenario(190)

	a1 := mustPromoted(t, ctx, st, scA, "scenario A root")
	a2 := mustPromoted(t, ctx, st, scA, "scenario A child")
	b1 := mustPromoted(t, ctx, st, scB, "scenario B belief")
	edge(t, ctx, a1, a2)
	edge(t, ctx, a2, b1) // the cross-scenario edge

	n, err := st.RetractCascade(ctx, scA, a1)
	bStatus, _ := beliefState(t, ctx, b1)

	rec.check(t, err == nil && n == 2 && bStatus == "promoted", Case{
		ID: "B-19", Wave: "2",
		Purpose:   "D-032 — a scenario-scoped cascade does not follow an edge into another scenario",
		Expected:  "returns 2; the scenario-B belief stays 'promoted'",
		Observed:  fmt.Sprintf("returned %d; scenario-B belief=%q", n, bStatus),
		Invariant: "D-032 — RetractCascade is scenario-scoped",
		Receipt:   receiptOf(err),
	})
}

// B-24 and B-16 are ONE execution with two assertions. B-24 asserts how the cascade
// fails; B-16 asserts that nothing survived the failure.
//
// A live intent in scenario B pointing at a belief in scenario A escapes the scoped
// cancel. The retract then trips live_requires_promoted through ON UPDATE CASCADE, and
// the whole transaction must roll back — including the cancels already issued.
func TestW2_B24_B16_BlockedCascadeIsAtomic(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	scA, scB := scenario(24), scenario(240)

	a1 := mustPromoted(t, ctx, st, scA, "blocked cascade root")
	a2 := mustPromoted(t, ctx, st, scA, "blocked cascade child")
	edge(t, ctx, a1, a2)

	// In-scenario intent: the cascade would cancel this one.
	if err := st.IntentOnPromoted(ctx, scA, a2, "defer_patch"); err != nil {
		t.Fatalf("B-24 setup: %v", err)
	}
	// Foreign-scenario intent on the SAME belief: the scoped cancel cannot see it.
	if err := st.IntentOnPromoted(ctx, scB, a2, "foreign_scenario_action"); err != nil {
		t.Fatalf("B-24 setup (foreign intent): %v", err)
	}

	n, err := st.RetractCascade(ctx, scA, a1)

	// B-24: how it failed.
	pgErr := pgErrOf(err)
	failedCorrectly := err != nil && n == 0 && pgErr != nil &&
		pgErr.Code == "23514" && pgErr.ConstraintName == "live_requires_promoted"

	rec.check(t, failedCorrectly, Case{
		ID: "B-24", Wave: "2",
		Purpose:   "M1-R4 — a foreign-scenario live intent blocks a scoped cascade: refusal, not corruption",
		Expected:  "RetractCascade refused with 23514 / live_requires_promoted; returns 0",
		Observed:  fmt.Sprintf("returned %d; sqlstate=%q; constraint=%q", n, sqlStateOf(err), constraintOf(err)),
		SQLState:  sqlStateOf(err),
		Constrain: constraintOf(err),
		Invariant: "M1-R4 — the schema refuses rather than corrupts",
		Receipt:   receiptOf(err),
	})

	// B-16: nothing survived.
	rec.begin("2")
	a1Status, _ := beliefState(t, ctx, a1)
	a2Status, _ := beliefState(t, ctx, a2)

	var liveInA, liveInB int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='live'`, scA).Scan(&liveInA)
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='live'`, scB).Scan(&liveInB)

	atomic := a1Status == "promoted" && a2Status == "promoted" && liveInA == 1 && liveInB == 1
	observed := fmt.Sprintf("root=%q child=%q; live intents A=%d B=%d; rows changed=0", a1Status, a2Status, liveInA, liveInB)
	if atomic {
		observed += " — rollback verified"
	}

	rec.check(t, atomic, Case{
		ID: "B-16", Wave: "2",
		Purpose:   "I-8 — the cascade is ONE transaction: a blocked cascade leaves no partial effect, not even the cancels already issued",
		Expected:  "both beliefs still 'promoted'; the in-scenario intent still 'live' (rollback verified); 0 rows changed",
		Observed:  observed,
		Invariant: "I-8 — RetractCascade is a single transaction, cancel-before-retract",
		Receipt:   rowDump(t, ctx, scA, scB),
	})
}

func TestW2_B08_AuditAfterCascade(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(8)

	root, child := chain(t, ctx, st, sc, "audited root", "audited child")
	if err := st.IntentOnPromoted(ctx, sc, child, "defer_patch"); err != nil {
		t.Fatalf("B-08 setup: %v", err)
	}
	if _, err := st.RetractCascade(ctx, sc, root); err != nil {
		t.Fatalf("B-08 cascade: %v", err)
	}

	n, err := st.AuditLiveOnNonPromoted(ctx, sc)

	rec.check(t, err == nil && n == 0, Case{
		ID: "B-08", Wave: "2",
		Purpose:   "AuditLiveOnNonPromoted is I-5 expressed as a query",
		Expected:  "returns 0 after a completed cascade",
		Observed:  fmt.Sprintf("returned %d (err=%v)", n, err),
		Invariant: "I-5 — audit returns 0 in every committed state",
		Receipt:   receiptOf(err),
	})
}

// B-13 sweeps every scenario the suite created. AuditLiveOnNonPromoted is
// scenario-scoped by signature (M2-R4), so the global form is a raw query.
func TestW2_B13_I5_GlobalAudit(t *testing.T) {
	rec.begin("2")
	ctx := context.Background()

	var n int
	err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM action_intent a
		JOIN belief b ON b.id = a.belief_id
		WHERE a.state='live' AND b.status <> 'promoted'`).Scan(&n)

	rec.check(t, err == nil && n == 0, Case{
		ID: "B-13", Wave: "2",
		Purpose:   "I-5 holds globally, across every scenario the suite created",
		Expected:  "0 live intents on non-promoted beliefs, cluster-wide in this database",
		Observed:  fmt.Sprintf("%d live-on-non-promoted row(s)", n),
		Invariant: "I-5 — AuditLiveOnNonPromoted returns 0 in every committed state",
		Receipt:   receiptOf(err),
	})
}

// ---------------------------------------------------------------- Wave 3

// B-21 executes D-033's termination argument. UNION (not UNION ALL) deduplicates, and
// belief_edge's CHECK forbids only self-loops, so a two-node cycle is the case that
// would hang if the deduplication were not load-bearing.
func TestW3_B21_CycleTerminates(t *testing.T) {
	rec.begin("3")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st := kernel.New(shared)
	sc := scenario(21)

	d1 := mustPromoted(t, ctx, st, sc, "cycle node 1")
	d2 := mustPromoted(t, ctx, st, sc, "cycle node 2")
	edge(t, ctx, d1, d2)
	edge(t, ctx, d2, d1) // closes the cycle

	start := time.Now()
	n, err := st.RetractCascade(ctx, sc, d1)
	elapsed := time.Since(start)

	rec.check(t, err == nil && n == 2, Case{
		ID: "B-21", Wave: "3",
		Purpose:   "D-033 — UNION deduplication terminates the traversal on a cyclic belief graph",
		Expected:  "returns 2 and terminates within the 30s deadline",
		Observed:  fmt.Sprintf("returned %d in %d ms (deadline 30000 ms)", n, elapsed.Milliseconds()),
		Invariant: "D-033 — UNION, never UNION ALL",
		Receipt:   receiptOf(err),
	})
}

// B-14 is I-6 under R2: there is no embedding column, so "every embedding is NULL"
// holds by construction. The second pass required by contract §7 M2 is the suite's
// second run, driven by scripts/m2_accept.sh.
func TestW3_B14_I6_NoEmbeddingColumn(t *testing.T) {
	rec.begin("3")
	ctx := context.Background()

	var n int
	err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='belief' AND column_name='embedding'`).Scan(&n)

	rec.check(t, err == nil && n == 0, Case{
		ID: "B-14", Wave: "3",
		Purpose:   "I-6 — vectors are never part of belief semantics. Under R2 there is no embedding column, so every embedding is NULL by construction",
		Expected:  "0 rows for (belief, embedding) in information_schema.columns",
		Observed:  fmt.Sprintf("%d embedding column(s) on belief", n),
		Invariant: "I-6 — the ledger is meaningful with zero vectors",
		Receipt:   receiptOf(err),
	})
}

// B-15 records I-7's status honestly: no test can prove a negative about code paths at
// runtime. It is asserted statically by scripts/check_i7.sh, which scripts/m2_accept.sh
// runs before this suite.
func TestW3_B15_I7_StaticGate(t *testing.T) {
	rec.begin("3")
	rec.check(t, true, Case{
		ID: "B-15", Wave: "3",
		Purpose:   "I-7 — every kernel write goes through crdb.ExecuteTx",
		Expected:  "asserted statically: 6 ExecuteTx write sites, 0 raw writes",
		Observed:  "NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md",
		Invariant: "I-7 — no raw db.Exec/db.Query writes",
	})
}

// ---------------------------------------------------------------- EnsureBelief

func TestW3_EnsureBelief_New(t *testing.T) {
	rec.begin("3")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(30)

	id, err := st.EnsureBelief(ctx, sc, "claim created by EnsureBelief", kernel.Derived)

	var status string
	var debtLen int
	if err == nil {
		err = shared.QueryRowContext(ctx,
			`SELECT status, coalesce(array_length(debt,1),0) FROM belief WHERE id=$1::UUID`,
			id).Scan(&status, &debtLen)
	}

	ok := err == nil && looksLikeUUID(id) && status == "entered" && debtLen == 6
	rec.check(t, ok, Case{
		ID: "W3-Ensure-New", Wave: "3",
		Purpose:   "EnsureBelief creates a new belief with full debt when claim does not exist",
		Expected:  "parseable UUID; status='entered', 6 debt items",
		Observed:  fmt.Sprintf("id parseable=%t, status=%q, debt items=%d", looksLikeUUID(id), status, debtLen),
		Invariant: "EnsureBelief — find-or-create in one transaction",
		Receipt:   receiptOf(err),
	})
}

func TestW3_EnsureBelief_Existing(t *testing.T) {
	rec.begin("3")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := scenario(31)
	claim := "claim that already exists"

	id1, err := st.EnsureBelief(ctx, sc, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("first EnsureBelief: %v", err)
	}

	id2, err := st.EnsureBelief(ctx, sc, claim, kernel.Derived)

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND claim=$2`, sc, claim).Scan(&count)

	ok := err == nil && id1 == id2 && count == 1
	rec.check(t, ok, Case{
		ID: "W3-Ensure-Existing", Wave: "3",
		Purpose:   "EnsureBelief returns existing belief ID when claim already exists in scenario",
		Expected:  "same ID returned; 1 belief row (no duplicate)",
		Observed:  fmt.Sprintf("id1=%s, id2=%s, same=%t; count=%d", id1, id2, id1 == id2, count),
		Invariant: "EnsureBelief — dedup is transactional (no TOCTOU)",
		Receipt:   receiptOf(err),
	})
}

func TestW3_EnsureBelief_DifferentScenario(t *testing.T) {
	rec.begin("3")
	ctx := context.Background()
	st := kernel.New(shared)
	scA, scB := scenario(32), scenario(320)
	claim := "same claim in different scenarios"

	idA, err := st.EnsureBelief(ctx, scA, claim, kernel.Derived)
	if err != nil {
		t.Fatalf("scenario A: %v", err)
	}
	idB, err := st.EnsureBelief(ctx, scB, claim, kernel.Derived)

	ok := err == nil && idA != idB
	rec.check(t, ok, Case{
		ID: "W3-Ensure-DiffScenario", Wave: "3",
		Purpose:   "EnsureBelief creates separate beliefs for the same claim in different scenarios",
		Expected:  "two different IDs",
		Observed:  fmt.Sprintf("idA=%s, idB=%s, different=%t", idA, idB, idA != idB),
		Invariant: "EnsureBelief — scenario-scoped uniqueness",
		Receipt:   receiptOf(err),
	})
}

// ---------------------------------------------------------------- helpers

func retireAll(ctx context.Context, st *kernel.Store, id string) error {
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, id, item); err != nil {
			return err
		}
	}
	return nil
}

func promotedBelief(ctx context.Context, st *kernel.Store, sc, claim string) (string, error) {
	id, err := st.EnterBelief(ctx, sc, claim, kernel.Derived)
	if err != nil {
		return "", err
	}
	if err := retireAll(ctx, st, id); err != nil {
		return "", err
	}
	if err := st.Promote(ctx, id); err != nil {
		return "", err
	}
	return id, nil
}

func mustPromoted(t *testing.T, ctx context.Context, st *kernel.Store, sc, claim string) string {
	t.Helper()
	id, err := promotedBelief(ctx, st, sc, claim)
	if err != nil {
		t.Fatalf("setup (promote %q): %v", claim, err)
	}
	return id
}

// chain creates a promoted parent and a promoted child joined by an edge.
func chain(t *testing.T, ctx context.Context, st *kernel.Store, sc, rootClaim, childClaim string) (string, string) {
	t.Helper()
	root := mustPromoted(t, ctx, st, sc, rootClaim)
	child := mustPromoted(t, ctx, st, sc, childClaim)
	edge(t, ctx, root, child)
	return root, child
}

func edge(t *testing.T, ctx context.Context, parent, child string) {
	t.Helper()
	if _, err := shared.ExecContext(ctx,
		`INSERT INTO belief_edge (parent_id, child_id) VALUES ($1::UUID, $2::UUID)`,
		parent, child); err != nil {
		t.Fatalf("setup (edge %s->%s): %v", parent, child, err)
	}
}

func beliefState(t *testing.T, ctx context.Context, id string) (status, debt string) {
	t.Helper()
	err := shared.QueryRowContext(ctx,
		`SELECT status, array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, id).
		Scan(&status, &debt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("read belief %s: %v", id, err)
	}
	return status, debt
}

func debtString(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	_, debt := beliefState(t, ctx, id)
	return debt
}

// retryCount reads an existing CockroachDB metric (N2). No instrumentation is invented
// to obtain it, and the restricted interface is NOT unlocked to reach it: on v26.2
// crdb_internal requires allow_unsafe_internals, which the docs call unsupported.
// The absence of the metric is itself recorded, with the SQLSTATE that explains it.
func retryCount(ctx context.Context, appName string) string {
	var maxRetries sql.NullInt64
	err := shared.QueryRowContext(ctx, `
		SELECT max(max_retries) FROM crdb_internal.node_transaction_statistics
		WHERE application_name = $1`, appName).Scan(&maxRetries)
	if err != nil {
		if state := sqlStateOf(err); state != "" {
			return fmt.Sprintf("unknown (crdb_internal restricted on v26.2, SQLSTATE %s; not unlocked — see N2)", state)
		}
		return "unknown (metric unreachable)"
	}
	if !maxRetries.Valid {
		return "unknown (no statistics row for this application_name)"
	}
	return fmt.Sprintf("%d", maxRetries.Int64)
}

// rowDump is the receipt for a case whose failure mode is that nothing was refused:
// the row state itself is the evidence.
func rowDump(t *testing.T, ctx context.Context, scenarios ...string) string {
	t.Helper()
	var b strings.Builder
	for _, sc := range scenarios {
		b.WriteString("scenario " + sc + "\n")
		rows, err := shared.QueryContext(ctx, `
			SELECT 'belief', id::STRING, status FROM belief WHERE scenario_id=$1::UUID
			UNION ALL
			SELECT 'intent', id::STRING, state FROM action_intent WHERE scenario_id=$1::UUID
			ORDER BY 1, 2`, sc)
		if err != nil {
			b.WriteString("  <dump failed: " + err.Error() + ">\n")
			continue
		}
		for rows.Next() {
			var kind, id, state string
			if err := rows.Scan(&kind, &id, &state); err == nil {
				b.WriteString(fmt.Sprintf("  %-7s %s  %s\n", kind, id, state))
			}
		}
		_ = rows.Close()
	}
	return strings.TrimRight(b.String(), "\n")
}

func describeRefusal(err error, isSentinel bool, status string) string {
	return fmt.Sprintf("errors.Is=%t; errors.As=%t; sqlstate=%q; constraint=%q; status=%q",
		isSentinel, pgErrOf(err) != nil, sqlStateOf(err), constraintOf(err), status)
}

func receiptOf(err error) string {
	if err == nil {
		return ""
	}
	return collapse(err.Error())
}

func without(items []string, drop string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	return s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}
