// Package demoseed holds the one piece of demo state that the kernel deliberately
// cannot produce: the derivation edge between two seeded beliefs.
//
// # Why this is not in the kernel
//
// `belief_edge` has no writer anywhere in production code. The kernel reads it —
// RetractCascade's WITH RECURSIVE traversal walks it (kernel/sql.go:58-72) — but
// nothing has ever populated it, which is why README.md:140-143 records retraction
// as "currently single-belief". Adding an eighth write path to the kernel is not an
// option: invariant I-7 pins the count at exactly seven crdb.ExecuteTx write sites
// and scripts/check_i7.sh fails the build on eight.
//
// That constraint is correct rather than inconvenient. Filing a derivation edge is
// not an epistemic operation — it asserts nothing about truth, debt, or authority,
// and it cannot promote or retract anything. It is a statement about how two claims
// were related when they were seeded. So it lives here, outside the kernel, in a
// package whose name makes clear it is demo scaffolding. internal/corpus took the
// same route for the same reason (see internal/corpus/corpus.go:13-22).
//
// # Why an edge exists at all
//
// Without one, the FALSIFY & AUDIT screen can only retract the belief that directly
// carries the live intent. That works, but it demonstrates the weaker of the two
// enforcement behaviours the probe measured. proof/act6_tier_probe.log shows that
// retracting an ANCESTOR is refused too: the recursive traversal reaches the
// descendant, the composite `gate` FK cascades the new status into the descendant's
// intent, and live_requires_promoted refuses with 23514.
//
// The traversal is application logic. What the database enforces is that the
// traversal cannot finish having left a live intent behind.
package demoseed

import (
	"context"
	"database/sql"
	"fmt"
)

// The two seeded claims the edge connects. Matched by text, not by UUID, because
// every seeding path generates fresh UUIDs — the local demo scripts and the cloud
// initializer produce different ids for the same claims, and the edge has to be the
// same relationship in both.
const (
	// ParentClaim is the finding: etcd's v3.5 line has a documented data-consistency
	// defect. internal/derive produces it from the postmortem fixture.
	ParentClaim = "v3.5.0–v3.5.2 has documented data inconsistency"

	// ChildClaim is the claim that depends on it. "A fix is available" is a statement
	// ABOUT the defect above: it presupposes the defect was correctly identified. If
	// the parent finding were withdrawn, this claim would lose the thing it refers to,
	// which is exactly what makes it a legitimate descendant rather than a sibling.
	ChildClaim = "fix available for etcd-io/etcd v3.5.0"

	// Kind is one of the two values belief_edge's CHECK permits ('derives',
	// 'contradicts'). This edge is a derivation, not a contradiction.
	Kind = "derives"
)

// ErrClaimsMissing reports that the seeded beliefs the edge connects were not found.
// Returned rather than ignored: silently skipping the edge would leave the Tier 1
// demo path quietly unavailable, which is the failure mode this package exists to
// prevent.
type ErrClaimsMissing struct {
	ScenarioID  string
	ParentClaim string
	ChildClaim  string
	FoundParent bool
	FoundChild  bool
}

func (e *ErrClaimsMissing) Error() string {
	return fmt.Sprintf(
		"demoseed: cannot file the edge %q --> %q in scenario %s (parent found=%t, child found=%t); "+
			"the claims it connects must exist first",
		e.ParentClaim, e.ChildClaim, e.ScenarioID, e.FoundParent, e.FoundChild)
}

// FileDerivationEdge files the single demo edge ParentClaim --derives--> ChildClaim
// and reports whether a new row was written.
//
// Idempotent: belief_edge's primary key is (parent_id, child_id), so a repeat call
// conflicts and does nothing. That matters because the demo seed is re-run freely
// and `pipeline.Run` is itself invoked twice by the Track 2 walkthrough.
//
// One statement, so the implicit transaction is the whole operation and CockroachDB
// retries it internally — the same reasoning internal/corpus documents for its
// single-statement writes.
func FileDerivationEdge(ctx context.Context, db *sql.DB, scenarioID string) (bool, error) {
	return FileEdge(ctx, db, scenarioID, ParentClaim, ChildClaim, Kind)
}

// FileEdge files one edge between two claims in the same scenario, matched by text,
// and reports whether a new row was written.
//
// This is FileDerivationEdge generalised: the Track 2 demo has one hardcoded pair,
// but the wizard seeds a different pair under a fresh scenario, and both need the
// identical semantics — text matching, idempotence, and a loud error when a claim is
// absent rather than a silent no-op.
//
// kind must be one of the two values belief_edge's CHECK permits ('derives',
// 'contradicts'); the database enforces that, so this function does not duplicate the
// check.
func FileEdge(ctx context.Context, db *sql.DB, scenarioID, parentClaim, childClaim, kind string) (bool, error) {
	const q = `
		INSERT INTO belief_edge (parent_id, child_id, kind)
		SELECT p.id, c.id, $4
		  FROM belief p, belief c
		 WHERE p.scenario_id = $1::UUID AND p.claim = $2
		   AND c.scenario_id = $1::UUID AND c.claim = $3
		ON CONFLICT (parent_id, child_id) DO NOTHING`

	res, err := db.ExecContext(ctx, q, scenarioID, parentClaim, childClaim, kind)
	if err != nil {
		return false, fmt.Errorf("demoseed: file edge %q --> %q: %w", parentClaim, childClaim, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("demoseed: rows affected: %w", err)
	}
	if n > 0 {
		return true, nil
	}

	// Zero rows is ambiguous — either the edge already existed, or one of the claims
	// is absent and the SELECT matched nothing. Those are very different situations
	// and only one of them is fine.
	present, err := EdgeExistsBetween(ctx, db, scenarioID, parentClaim, childClaim)
	if err != nil {
		return false, err
	}
	if present {
		return false, nil
	}

	var foundParent, foundChild bool
	if err := db.QueryRowContext(ctx, `
		SELECT
		  EXISTS (SELECT 1 FROM belief WHERE scenario_id = $1::UUID AND claim = $2),
		  EXISTS (SELECT 1 FROM belief WHERE scenario_id = $1::UUID AND claim = $3)`,
		scenarioID, parentClaim, childClaim).Scan(&foundParent, &foundChild); err != nil {
		return false, fmt.Errorf("demoseed: locate claims: %w", err)
	}
	return false, &ErrClaimsMissing{
		ScenarioID:  scenarioID,
		ParentClaim: parentClaim,
		ChildClaim:  childClaim,
		FoundParent: foundParent,
		FoundChild:  foundChild,
	}
}

// SQLRetractWithoutCancel retracts a belief and its descendants WITHOUT cancelling
// their live intents. It exists to be refused.
//
// # Read this before using it
//
// This statement is not a kernel operation and must never be used to actually
// retract anything. kernel.RetractCascade is the only correct path: it cancels
// strictly before retracting (I-8), so it always succeeds and can never produce the
// refusal. Anyone hunting for the Tier 1 refusal by trying to make RetractCascade
// fail will conclude the enforcement is broken. It is not — the kernel is simply
// safe by construction.
//
// What this statement is for is the FALSIFY & AUDIT screen's first button: the naive
// thing a normal application does, "retract the belief", issued as one UPDATE over
// the subtree. The composite `gate` FK propagates the new status into every intent
// row via ON UPDATE CASCADE, and live_requires_promoted then refuses with
// 23514 · live_requires_promoted. proof/act6_tier_probe.log measured exactly this on
// both the local and the deployed cluster.
//
// The recursive traversal here is application logic. What the database enforces is
// that the traversal cannot finish having left a live intent behind.
//
// Placeholders are $1 root belief id, $2 scenario id — the same shape the kernel's
// cascade statements use, so the two are directly comparable.
const SQLRetractWithoutCancel = `
		WITH RECURSIVE d(id) AS (
		    SELECT b.id FROM belief b
		     WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
		  UNION
		    SELECT e.child_id
		      FROM belief_edge e
		      JOIN d         ON e.parent_id = d.id
		      JOIN belief cb ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
		)
		UPDATE belief SET status = 'retracted'
		 WHERE status = 'promoted'
		   AND scenario_id = $2::UUID
		   AND id IN (SELECT id FROM d)`

// EdgeExists reports whether the demo edge is present in the given scenario.
//
// belief_edge carries no scenario_id of its own (D-032), so the scenario is reached
// through the parent belief — the same join inspect.sh and the audit query use.
func EdgeExists(ctx context.Context, db *sql.DB, scenarioID string) (bool, error) {
	return EdgeExistsBetween(ctx, db, scenarioID, ParentClaim, ChildClaim)
}

// EdgeExistsBetween reports whether an edge between the two named claims is present
// in the given scenario. EdgeExists is this with the Track 2 pair.
func EdgeExistsBetween(ctx context.Context, db *sql.DB, scenarioID, parentClaim, childClaim string) (bool, error) {
	var ok bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM belief_edge e
		    JOIN belief p ON p.id = e.parent_id
		    JOIN belief c ON c.id = e.child_id
		   WHERE p.scenario_id = $1::UUID AND p.claim = $2
		     AND c.scenario_id = $1::UUID AND c.claim = $3)`,
		scenarioID, parentClaim, childClaim).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("demoseed: check edge %q --> %q: %w", parentClaim, childClaim, err)
	}
	return ok, nil
}
