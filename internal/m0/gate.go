// Verification-only. This package is M0's executable specification, not production
// infrastructure. It must never be imported by internal/kernel (D-023) and is frozen
// at the close of M0 (O1).
package m0

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Fixed fixture identifiers (N4). No gen_random_uuid() anywhere in this harness:
// a transcript that changes between runs is not a receipt.
const (
	scenarioID = "11111111-1111-1111-1111-111111111111"

	beliefDebted     = "00000000-0000-0000-0000-0000000000c1" // C1: open debt
	beliefFinalTruth = "00000000-0000-0000-0000-0000000000c2" // C2: final_truth = true
	beliefClean      = "00000000-0000-0000-0000-0000000000c3" // C3: debt-free, promotable

	beliefRoot    = "00000000-0000-0000-0000-00000000000a" // D: cascade root
	beliefChild   = "00000000-0000-0000-0000-00000000000b" // D: descendant
	beliefEntered = "00000000-0000-0000-0000-00000000000e" // D1: never promoted

	intentRefused = "00000000-0000-0000-0000-0000000000d0" // D1: must not exist
	intentLive    = "00000000-0000-0000-0000-0000000000d1" // D2..D4: the live intent
)

// RunGate executes the C- and D-series and returns once every probe is recorded.
// It does not stop on failure: a complete transcript is worth more than an early exit.
func RunGate(ctx context.Context, r *Runner) {
	runCheckSeries(ctx, r)
	runGateSeries(ctx, r)
}

// --- C: the promotion CHECK, contract §7 M0 read literally ---

func runCheckSeries(ctx context.Context, r *Runner) {
	ok := setup(ctx, r, "C0", "C-series fixtures", fmt.Sprintf(`
		INSERT INTO belief (id, scenario_id, claim, claim_type, status, debt, final_truth) VALUES
		  ('%s', '%s', 'claim carrying its full starting debt',   'accommodated', 'entered', DEFAULT, false),
		  ('%s', '%s', 'claim asserted as settled and final',     'postulated',   'entered', '{}',    true),
		  ('%s', '%s', 'claim whose debt has been fully retired', 'derived',      'entered', '{}',    false)`,
		beliefDebted, scenarioID, beliefFinalTruth, scenarioID, beliefClean, scenarioID))
	if !ok {
		return
	}

	r.ExpectFailure(ctx,
		"C1",
		"A promoted-with-open-debt UPDATE fails with SQLSTATE 23514 (contract §7 M0; invariant I-1)",
		"23514", "promoted_is_debt_free",
		fmt.Sprintf("UPDATE belief SET status = 'promoted' WHERE id = '%s'", beliefDebted),
	)

	r.ExpectFailure(ctx,
		"C2",
		"Promoting a final-truth claim fails with SQLSTATE 23514 (invariant I-2)",
		"23514", "promoted_is_debt_free",
		fmt.Sprintf("UPDATE belief SET status = 'promoted' WHERE id = '%s'", beliefFinalTruth),
	)

	r.ExpectOK(ctx,
		"C3",
		"A promoted-debt-free UPDATE succeeds (contract §7 M0)",
		fmt.Sprintf("UPDATE belief SET status = 'promoted' WHERE id = '%s'", beliefClean),
	)
}

// --- D: composite FK + ON UPDATE CASCADE + CHECK. The §9 high-risk interaction. ---

func runGateSeries(ctx context.Context, r *Runner) {
	ok := setup(ctx, r, "D0", "D-series fixtures", fmt.Sprintf(`
		INSERT INTO belief (id, scenario_id, claim, claim_type, status, debt) VALUES
		  ('%s', '%s', 'root: exposure signal absent for asset A', 'accommodated', 'promoted', '{}'),
		  ('%s', '%s', 'child: asset A triage may be deferred',    'derived',      'promoted', '{}'),
		  ('%s', '%s', 'unrelated claim still at the door',        'postulated',   'entered',  '{}')`,
		beliefRoot, scenarioID, beliefChild, scenarioID, beliefEntered, scenarioID))
	if !ok {
		return
	}
	if !setup(ctx, r, "D0", "D-series dependency edge", fmt.Sprintf(
		"INSERT INTO belief_edge (parent_id, child_id) VALUES ('%s', '%s')", beliefRoot, beliefChild)) {
		return
	}

	r.ExpectFailure(ctx,
		"D1",
		"An action intent citing a non-promoted belief is refused by the composite FK (invariant I-3)",
		"23503", "gate",
		fmt.Sprintf(
			"INSERT INTO action_intent (id, scenario_id, belief_id, action) VALUES ('%s', '%s', '%s', 'defer_patch')",
			intentRefused, scenarioID, beliefEntered),
	)

	r.ExpectOK(ctx,
		"D2",
		"An action intent citing a promoted belief is accepted",
		fmt.Sprintf(
			"INSERT INTO action_intent (id, scenario_id, belief_id, action) VALUES ('%s', '%s', '%s', 'defer_patch')",
			intentLive, scenarioID, beliefChild),
	)

	// D3 is the measurement contract §9 demanded. PostgreSQL 16 refused this with
	// 23514/live_requires_promoted: ON UPDATE CASCADE propagated the new status into
	// the intent row, and the child CHECK then detonated. Whether CockroachDB
	// re-evaluates the child CHECK on a cascade-modified row is undocumented.
	r.ExpectFailure(ctx,
		"D3",
		"Retracting a belief that still carries a live intent is refused (invariant I-4; the §9 high-risk interaction)",
		"23514", "live_requires_promoted",
		fmt.Sprintf("UPDATE belief SET status = 'retracted' WHERE id = '%s'", beliefChild),
	)

	runD4(ctx, r)

	r.ExpectValue(ctx,
		"D5",
		"AuditLiveOnNonPromoted returns 0 in committed state (invariant I-5)",
		"0",
		`SELECT count(*)::STRING FROM action_intent a JOIN belief b ON b.id = a.belief_id
		 WHERE a.state = 'live' AND b.status <> 'promoted'`,
	)
}

// runD4 performs the correct cascade in ONE transaction: compute the transitive
// descendant set, cancel live intents first, then retract. It must commit, and the
// surviving cancelled intent must carry the cascaded belief_status.
func runD4(ctx context.Context, r *Runner) {
	const descendants = `WITH RECURSIVE d(id) AS (
		SELECT $1::UUID
		UNION
		SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
	)`

	var (
		capturedErr  error
		stage        string
		gotState     string
		gotStatus    string
		retractedRow int64
	)

	start := time.Now()
	func() {
		tx, err := r.DB().BeginTx(ctx, nil)
		if err != nil {
			stage, capturedErr = "BEGIN", err
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if _, err := tx.ExecContext(ctx, descendants+`
			UPDATE action_intent SET state = 'cancelled'
			WHERE state = 'live' AND belief_id IN (SELECT id FROM d)`, beliefRoot); err != nil {
			stage, capturedErr = "cancel live intents", err
			return
		}

		res, err := tx.ExecContext(ctx, descendants+`
			UPDATE belief SET status = 'retracted'
			WHERE status = 'promoted' AND id IN (SELECT id FROM d)`, beliefRoot)
		if err != nil {
			stage, capturedErr = "retract descendants", err
			return
		}
		retractedRow, _ = res.RowsAffected()

		if err := tx.QueryRowContext(ctx,
			"SELECT state, belief_status FROM action_intent WHERE id = $1", intentLive,
		).Scan(&gotState, &gotStatus); err != nil {
			stage, capturedErr = "read back the cancelled intent", err
			return
		}

		if err := tx.Commit(); err != nil {
			stage, capturedErr = "COMMIT", err
			return
		}
		committed = true
	}()
	elapsed := time.Since(start)

	p := Probe{
		ID: "D4",
		Criterion: "Cancel-then-retract commits in ONE transaction, and ON UPDATE CASCADE " +
			"propagates the new status into the surviving cancelled intent (invariant I-8)",
		Statement:  "BEGIN; WITH RECURSIVE d(...) UPDATE action_intent SET state='cancelled' ...; WITH RECURSIVE d(...) UPDATE belief SET status='retracted' ...; COMMIT",
		Expected:   "commits; 2 beliefs retracted; intent reads state='cancelled', belief_status='retracted'",
		SQLState:   sqlStateOf(capturedErr),
		Constraint: constraintOf(capturedErr),
		RawError:   rawOf(capturedErr),
		Elapsed:    elapsed,
	}

	switch {
	case capturedErr != nil:
		p.Observed = fmt.Sprintf("FAILED at %q with SQLSTATE %s", stage, p.SQLState)
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	case retractedRow != 2:
		p.Observed = fmt.Sprintf("committed, but retracted %d beliefs (expected 2)", retractedRow)
	case gotState != "cancelled" || gotStatus != "retracted":
		p.Observed = fmt.Sprintf("committed, but intent reads state=%q belief_status=%q — cascade did not propagate", gotState, gotStatus)
	default:
		p.Observed = "committed; 2 beliefs retracted; intent reads state='cancelled', belief_status='retracted'"
		p.Pass = true
	}
	r.Record(p)
}

// setup applies fixture rows. On success it records nothing — fixtures are not
// acceptance criteria. On failure it records a probe, because a broken fixture
// invalidates every probe downstream of it.
func setup(ctx context.Context, r *Runner, id, what, stmt string) bool {
	start := time.Now()
	_, err := r.DB().ExecContext(ctx, stmt)
	if err == nil {
		return true
	}
	r.Record(Probe{
		ID:         id,
		Criterion:  "Fixture setup: " + what,
		Statement:  collapse(stmt),
		Expected:   "fixtures insert cleanly",
		Observed:   fmt.Sprintf("FAILED with SQLSTATE %s — downstream probes were not run", sqlStateOf(err)),
		SQLState:   sqlStateOf(err),
		Constraint: constraintOf(err),
		RawError:   rawOf(err),
		Elapsed:    time.Since(start),
		Detail:     "raw driver error:\n\n    " + strings.ReplaceAll(rawOf(err), " / ", "\n    "),
	})
	return false
}
