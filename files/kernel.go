// Package kernel is the transactional belief ledger. Every statement here is a
// transcription of the pure-SQL proof (db/001_schema.sql + the invariant/cascade
// proof), not new design. Invariants I-1..I-5 are enforced by the schema; this
// package's job is to route every write through crdb.ExecuteTx so serialization
// failures (40001) retry correctly, and to keep the cascade in ONE transaction.
package kernel

import (
	"context"
	"database/sql"
	"errors"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
)

// Full starting debt — the EBP kernel's fullDebt, verbatim.
var FullDebt = []string{
	"needMap", "needInvariant", "needToyCheck",
	"needNullModel", "needObstruction", "needFaithfulnessReview",
}

type ClaimType string

const (
	Derived      ClaimType = "derived"
	Accommodated ClaimType = "accommodated"
	Postulated   ClaimType = "postulated"
)

// Store wraps a *sql.DB. The DB must point at CockroachDB (or, for the proof,
// Postgres) with the schema from db/001_schema.sql already applied.
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

// EnterBelief adds a claim at the door: alive, fully in debt, unpromoted.
// "Ideas enter free" — no gate here by design.
func (s *Store) EnterBelief(ctx context.Context, scenarioID, claim string, ct ClaimType) (string, error) {
	var id string
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO belief (scenario_id, claim, claim_type, debt)
			VALUES ($1, $2, $3, $4)
			RETURNING id`,
			scenarioID, claim, string(ct), FullDebt,
		).Scan(&id)
	})
	return id, err
}

// AddEvidence records evidence for a belief. provenanceClass must be one of the
// four schema-enforced classes; contentSHA256 is required (no fabricated rows).
func (s *Store) AddEvidence(ctx context.Context, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO evidence
			  (scenario_id, belief_id, provenance_class, source_url, content_sha256)
			VALUES ($1, $2, $3, $4, $5)`,
			scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)
		return err
	})
}

// RetireDebt removes one debt item. One useful move retires one debt.
func (s *Store) RetireDebt(ctx context.Context, beliefID, item string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE belief SET debt = array_remove(debt, $2) WHERE id = $1`,
			beliefID, item)
		return err
	})
}

// Promote moves a belief to the throne. The schema CHECK (promoted_is_debt_free)
// makes this fail if any debt remains or final-truth language is present — the
// gate is not enforced here, it is enforced by the database. We surface that.
var ErrPromotionBlocked = errors.New("promotion blocked: open debt or final-truth language")

func (s *Store) Promote(ctx context.Context, beliefID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE belief SET status = 'promoted' WHERE id = $1`, beliefID)
		if isCheckViolation(err) {
			return ErrPromotionBlocked
		}
		return err
	})
}

// IntentOnPromoted records intent to act. The composite FK (belief_id,'promoted')
// physically refuses this unless the belief is currently promoted (I-3).
var ErrActionOnUnpromoted = errors.New("action refused: belief is not promoted")

func (s *Store) IntentOnPromoted(ctx context.Context, scenarioID, beliefID, action string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO action_intent (scenario_id, belief_id, action)
			VALUES ($1, $2, $3)`, scenarioID, beliefID, action)
		if isFKViolation(err) {
			return ErrActionOnUnpromoted
		}
		return err
	})
}

// RetractCascade is the whole thesis in one serializable transaction: walk the
// dependency graph from rootID, cancel live intents on every descendant FIRST
// (so the composite FK's ON UPDATE CASCADE + live_requires_promoted CHECK cannot
// strand a live intent), then un-promote the descendants. Cancel-before-retract
// is enforced by the schema; running it in one tx under serializable is what
// makes the concurrent case (cascade vs. new intent) refuse instead of corrupt.
func (s *Store) RetractCascade(ctx context.Context, scenarioID, rootID string) (retracted int, err error) {
	err = crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `
			CREATE TEMP TABLE _desc ON COMMIT DROP AS
			WITH RECURSIVE d(id) AS (
			  SELECT $1::uuid
			  UNION
			  SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
			) SELECT id FROM d`, rootID); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `
			UPDATE action_intent SET state = 'cancelled'
			WHERE state = 'live' AND belief_id IN (SELECT id FROM _desc)`); e != nil {
			return e
		}
		res, e := tx.ExecContext(ctx, `
			UPDATE belief SET status = 'retracted'
			WHERE status = 'promoted' AND id IN (SELECT id FROM _desc)`)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		retracted = int(n)
		return nil
	})
	return retracted, err
}

// AuditLiveOnNonPromoted is invariant I-5 as a query: the count must always be 0.
func (s *Store) AuditLiveOnNonPromoted(ctx context.Context, scenarioID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM action_intent a JOIN belief b ON b.id = a.belief_id
		WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = $1`,
		scenarioID).Scan(&n)
	return n, err
}

// --- error classification: pgx SQLSTATE, not brittle string matching (F-19) ---

type sqlStater interface{ SQLState() string }

func sqlState(err error) string {
	var s sqlStater
	if errors.As(err, &s) {
		return s.SQLState()
	}
	return ""
}
func isCheckViolation(err error) bool { return sqlState(err) == "23514" }
func isFKViolation(err error) bool    { return sqlState(err) == "23503" }
