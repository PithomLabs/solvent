package kernel

import "sort"

// Every statement the kernel runs, in one place. The functions in kernel.go use
// these constants directly, so SQLCatalog cannot drift from what actually executes.
//
// Placeholders carry explicit casts. CockroachDB resolves placeholder types at
// PREPARE time and refuses a statement it cannot type (42P18), so the casts are
// what let cmd/m1verify prepare these without executing them.
const (
	sqlEnterBelief = `
		INSERT INTO belief (scenario_id, claim, claim_type, debt)
		VALUES ($1::UUID, $2::STRING, $3::STRING, $4::STRING[])
		RETURNING id`

	sqlAddEvidence = `
		INSERT INTO evidence
		  (scenario_id, belief_id, provenance_class, source_url, content_sha256)
		VALUES ($1::UUID, $2::UUID, $3::STRING, $4::STRING, $5::STRING)`

	sqlRetireDebt = `
		UPDATE belief SET debt = array_remove(debt, $2::STRING)
		WHERE id = $1::UUID`

	sqlPromote = `
		UPDATE belief SET status = 'promoted'
		WHERE id = $1::UUID`

	sqlIntentOnPromoted = `
		INSERT INTO action_intent (scenario_id, belief_id, action)
		VALUES ($1::UUID, $2::UUID, $3::STRING)`

	sqlAuditLiveOnNonPromoted = `
		SELECT count(*) FROM action_intent a
		JOIN belief b ON b.id = a.belief_id
		WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = $1::UUID`

	sqlEnsureBelief = `
		WITH existing AS (
			SELECT id FROM belief
			WHERE scenario_id = $1::UUID AND claim = $2::STRING
			LIMIT 1
		),
		inserted AS (
			INSERT INTO belief (scenario_id, claim, claim_type)
			SELECT $1::UUID, $2::STRING, $3::STRING
			WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING id
		)
		SELECT id FROM existing
		UNION ALL
		SELECT id FROM inserted`
)

// descendantsCTE computes the transitive descendants of $1 within scenario $2.
//
// Scenario-scoped per D-032: belief_edge carries no scenario_id, so each hop is
// constrained by joining the child back to belief. An edge that leaves the scenario
// is not followed.
//
// UNION, never UNION ALL. belief_edge's CHECK forbids only self-loops
// (parent_id <> child_id), not longer cycles, so the deduplication UNION performs is
// what guarantees this recursion terminates on a cyclic belief graph. This is
// load-bearing, not stylistic.
const descendantsCTE = `
		WITH RECURSIVE d(id) AS (
		    SELECT b.id FROM belief b
		     WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
		  UNION
		    SELECT e.child_id
		      FROM belief_edge e
		      JOIN d         ON e.parent_id = d.id
		      JOIN belief cb ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
		)`

// The two halves of the cascade. Cancel strictly precedes retract (I-8): the
// composite FK's ON UPDATE CASCADE propagates the new status into every intent row,
// and live_requires_promoted then detonates unless the intent was already cancelled.
const (
	sqlRetractCascadeCancel = descendantsCTE + `
		UPDATE action_intent SET state = 'cancelled'
		 WHERE state = 'live'
		   AND scenario_id = $2::UUID
		   AND belief_id IN (SELECT id FROM d)`

	sqlRetractCascadeRetract = descendantsCTE + `
		UPDATE belief SET status = 'retracted'
		 WHERE status = 'promoted'
		   AND scenario_id = $2::UUID
		   AND id IN (SELECT id FROM d)`
)

// NamedSQL pairs a statement with a stable name.
//
// Verification support.
// Not part of the public kernel API.
type NamedSQL struct {
	Name string
	SQL  string
}

// SQLCatalog returns the kernel's own statements, sorted by name.
//
// Verification support.
// Not part of the public kernel API.
//
// It exists so cmd/m1verify can PREPARE the statements the kernel actually runs
// rather than a hand-copied second set that would drift. It changes no contract §4
// signature and may be withdrawn after M2 without touching one.
func SQLCatalog() []NamedSQL {
	out := []NamedSQL{
		{"add_evidence", sqlAddEvidence},
		{"audit_live_on_nonpromoted", sqlAuditLiveOnNonPromoted},
		{"enter_belief", sqlEnterBelief},
		{"ensure_belief", sqlEnsureBelief},
		{"intent_on_promoted", sqlIntentOnPromoted},
		{"promote", sqlPromote},
		{"retire_debt", sqlRetireDebt},
		{"retract_cascade_cancel", sqlRetractCascadeCancel},
		{"retract_cascade_retract", sqlRetractCascadeRetract},
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
