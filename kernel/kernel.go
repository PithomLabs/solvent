// The package comment lives in doc.go.
package kernel

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
)

// FullDebt is the full starting debt every belief carries at the door.
//
// These six items are byte-identical, and in the same order, to the ARRAY[...]
// default on belief.debt in db/001_schema.sql. The duplication is deliberate — the
// Go constant is what EnterBelief writes — but it means the two can drift, so any
// change to one must change the other. Tests B-17 and B-23 exist to catch that drift
// behaviourally: they insert a belief and compare what the database actually stored
// against this slice.
//
// # The vocabulary changed in Phase 5, on purpose
//
// These names were inherited physics-proof scaffolding (needMap, needInvariant,
// needToyCheck, needNullModel, needObstruction, needFaithfulnessReview). They are now
// the deployment-review obligations the demo is actually about. Same count, same
// order, same mechanism — only the strings moved, so every invariant and every
// cardinality check is unaffected.
//
// Existing rows were deliberately NOT rewritten. A belief that entered the ledger
// under the old vocabulary keeps the debt it was issued; the change applies to what
// the database hands out from here on. See db/004_debt_vocabulary.sql.
var FullDebt = []string{
	"needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
	"needRollbackPlan", "needVersionPin", "needOperatorSignoff",
}

// ClaimType is the typed provenance of a claim. The three values are exactly the
// schema's check_claim_type set.
type ClaimType string

const (
	Derived      ClaimType = "derived"
	Accommodated ClaimType = "accommodated"
	Postulated   ClaimType = "postulated"
)

// Store wraps an open pool pointed at a database with db/001_schema.sql applied.
type Store struct{ db *sql.DB }

// New wraps db. It does not ping, migrate, or validate the schema.
func New(db *sql.DB) *Store { return &Store{db: db} }

// EnterBelief inserts a claim at the door: status 'entered', carrying the full
// starting debt, unpromoted. Ideas enter free — this path is never gated.
func (s *Store) EnterBelief(ctx context.Context, scenarioID, claim string, ct ClaimType) (string, error) {
	// A copy, so a caller that mutates FullDebt cannot corrupt this write.
	debt := append([]string(nil), FullDebt...)

	var id string
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, sqlEnterBelief,
			scenarioID, claim, string(ct), debt,
		).Scan(&id)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// AddEvidence records one evidence row for a belief. contentSHA256 is required —
// the column is NOT NULL and this function substitutes no placeholder for a hash it
// was not given. Belief state is unchanged.
func (s *Store) AddEvidence(ctx context.Context, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlAddEvidence,
			scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)
		return err
	})
}

// RetireDebt removes one debt item. It is idempotent: array_remove on an absent item
// changes nothing, and affecting zero rows is success, not an error.
func (s *Store) RetireDebt(ctx context.Context, beliefID, item string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlRetireDebt, beliefID, item)
		return err
	})
}

// Promote moves a belief to promoted.
//
// The gate is the schema's promoted_is_debt_free CHECK, not this function's: no
// debt is inspected here. When the database refuses with 23514 the refusal is named
// ErrPromotionBlocked and the driver error is preserved underneath it.
func (s *Store) Promote(ctx context.Context, beliefID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlPromote, beliefID)
		return wrapIf(sqlStateCheckViolation, ErrPromotionBlocked, err)
	})
}

// IntentOnPromoted records intent to act on a belief.
//
// The composite FK (belief_id, 'promoted') -> belief(id, status) physically refuses
// this unless the belief is currently promoted (I-3). A 23503 refusal is named
// ErrActionOnUnpromoted with the driver error preserved underneath.
func (s *Store) IntentOnPromoted(ctx context.Context, scenarioID, beliefID, action string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlIntentOnPromoted, scenarioID, beliefID, action)
		return wrapIf(sqlStateFKViolation, ErrActionOnUnpromoted, err)
	})
}

// RetractCascade un-promotes a belief and everything derived from it, in ONE
// transaction.
//
// Order is mandatory (I-8): live intents on the descendant set are cancelled FIRST,
// then the descendants are retracted. Reversing it does not corrupt anything — the
// composite FK's ON UPDATE CASCADE propagates the new status into each intent row and
// live_requires_promoted then refuses the write — but it does fail, which is the
// schema enforcing the ordering rather than this code remembering it.
//
// Scenario-scoped per D-032: traversal and both updates are confined to scenarioID.
//
// retracted is the RowsAffected of the belief update only. It counts belief rows
// retracted, never cancelled intents.
func (s *Store) RetractCascade(ctx context.Context, scenarioID, rootID string) (int, error) {
	var retracted int
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Cancel first.
		if _, err := tx.ExecContext(ctx, sqlRetractCascadeCancel, rootID, scenarioID); err != nil {
			return err
		}
		// Then retract.
		res, err := tx.ExecContext(ctx, sqlRetractCascadeRetract, rootID, scenarioID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		retracted = int(n)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return retracted, nil
}

// AuditLiveOnNonPromoted is invariant I-5 expressed as a query: the count of live
// intents whose belief is not promoted, scoped to one scenario. It must be 0 in
// every committed state.
//
// This is the one read path, so it does not use crdb.ExecuteTx — I-7 governs writes.
func (s *Store) AuditLiveOnNonPromoted(ctx context.Context, scenarioID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, sqlAuditLiveOnNonPromoted, scenarioID).Scan(&n)
	return n, err
}

// EnsureBelief returns the ID of a belief with the given claim in the scenario.
// If no such belief exists, it creates one with the given claim type, full starting
// debt, and status='entered'.
//
// The find-or-create is a single transaction — no TOCTOU boundary.
// The caller does not need to know whether the belief was newly created.
func (s *Store) EnsureBelief(ctx context.Context, scenarioID, claim string, ct ClaimType) (string, error) {
	var id string
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, sqlEnsureBelief,
			scenarioID, claim, string(ct),
		).Scan(&id)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}
