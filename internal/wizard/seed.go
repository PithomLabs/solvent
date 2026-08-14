package wizard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/PithomLabs/solvent/internal/demoseed"
	"github.com/PithomLabs/solvent/kernel"
)

// The wizard's own claims, and the action the judge will try to authorise.
//
// # Why this is a PRE-falsification state, and why that matters
//
// Track 2's seed ends retracted and cancelled — a terminal state, correct for
// replaying a historical reversal and useless as a starting point, because there is
// nothing left to decide. The wizard seeds the moment BEFORE the reversal: a standing
// assumption already promoted, and a fresh claim that depends on it and has not
// earned anything yet.
//
// # Which belief the judge drives
//
// The DESCENDANT. This is the single easiest thing to get wrong and the failure is
// silent. The judge promotes and authorises the descendant, so the live intent hangs
// off the descendant; screen 3 then retracts the ANCESTOR, the traversal crosses the
// edge, and the cascade collides with the judge's own intent — 23514 ·
// live_requires_promoted, the Tier 1 refusal the Act 6 probe measured on both
// clusters.
//
// Point the intent at the ancestor instead and everything still "works": you get a
// refusal, but it is Tier 2 — the belief that directly carries the intent — and the
// graph traversal is doing no work at all. The demo would be claiming a cascade it
// never performed.
const (
	AncestorClaim   = "etcd v3.5.x has no known data-integrity defects"
	DescendantClaim = "upgrading production etcd to v3.5.x is safe"
	DeployAction    = "deploy etcd v3.5.28"
	EdgeKind        = "derives"
)

// The ancestor's provenance.
//
// The seeded ancestor is a decision somebody already made, so it is recorded as
// exactly that: operator-asserted, with an internal reference rather than an invented
// URL. Dressing it up as a retrieval result would be the one dishonest row in the
// demo, and it would undercut the contrast screens 1 and 2 exist to draw — that NEW
// authority has to be earned with recorded evidence, while this prior decision simply
// predates the ledger.
const (
	seedEvidenceRef  = "solvent:seed/prior-decision"
	seedEvidenceNote = "Seeded prior decision: at the time of seeding, no data-integrity defect " +
		"was known for the etcd v3.5.x line. Operator-asserted; not retrieved."
)

// SeedState is the shape SeedVerify measures. Every field is checked, because
// kernel.Promote will happily succeed against a UUID that does not exist.
type SeedState struct {
	AncestorID     string
	DescendantID   string
	AncestorStatus string
	DescStatus     string
	AncestorDebt   int
	DescDebt       int
	Edges          int
	Intents        int
	AncestorEvid   int
}

// NewScenario mints a fresh scenario id.
//
// Taken from the database rather than a Go UUID library: gen_random_uuid() is already
// what every table's primary key uses, and the module has no uuid dependency worth
// adding for one call.
func (s *Server) NewScenario(ctx context.Context) (string, error) {
	var id string
	if err := s.db.QueryRowContext(ctx, `SELECT gen_random_uuid()::STRING`).Scan(&id); err != nil {
		return "", fmt.Errorf("wizard: new scenario: %w", err)
	}
	return id, nil
}

// Seed writes the pre-falsification state into an empty scenario.
//
// Refuses a scenario that already holds beliefs rather than adding to it: seeding
// twice would produce two ancestors and an ambiguous graph, and the ambiguity would
// surface as a confusing screen rather than an error.
func (s *Server) Seed(ctx context.Context, scenarioID string) error {
	var existing int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, scenarioID).Scan(&existing); err != nil {
		return fmt.Errorf("wizard: seed precheck: %w", err)
	}
	if existing != 0 {
		return fmt.Errorf("wizard: scenario %s already holds %d belief(s); refusing to seed over it",
			scenarioID, existing)
	}

	// 1. The standing assumption, with its operator-asserted provenance.
	ancestorID, err := s.kern.EnterBelief(ctx, scenarioID, AncestorClaim, kernel.Postulated)
	if err != nil {
		return fmt.Errorf("wizard: enter ancestor: %w", err)
	}
	sum := sha256.Sum256([]byte(seedEvidenceNote))
	if err := s.kern.AddEvidence(ctx, scenarioID, ancestorID,
		"operator_asserted", seedEvidenceRef, hex.EncodeToString(sum[:])); err != nil {
		return fmt.Errorf("wizard: attach ancestor evidence: %w", err)
	}

	// 2. The claim the judge will drive. Full debt, untouched — the six items the DDL
	//    default issued are exactly what screen 1 is refused on.
	if _, err := s.kern.EnterBelief(ctx, scenarioID, DescendantClaim, kernel.Derived); err != nil {
		return fmt.Errorf("wizard: enter descendant: %w", err)
	}

	// 3. The edge that makes screen 3's cascade real rather than narrated.
	if _, err := demoseed.FileEdge(ctx, s.db, scenarioID, AncestorClaim, DescendantClaim, EdgeKind); err != nil {
		return fmt.Errorf("wizard: file edge: %w", err)
	}

	// 4. Promote the ancestor only. It is a decision already taken, so it carries no
	//    outstanding debt; the descendant keeps all six, which is what screen 1 refuses
	//    on and screen 2 discharges.
	for _, d := range kernel.FullDebt {
		if err := s.kern.RetireDebt(ctx, ancestorID, d); err != nil {
			return fmt.Errorf("wizard: retire %s on ancestor: %w", d, err)
		}
	}
	if err := s.kern.Promote(ctx, ancestorID); err != nil {
		return fmt.Errorf("wizard: promote ancestor: %w", err)
	}

	// 5. No action_intent. The judge creates the only one, on screen 2, by authorising
	//    the descendant — which is what makes the audit number non-vacuous later.

	// kernel.Promote runs a bare UPDATE with no RowsAffected check, so it reports
	// success for a belief id that does not exist. Verify rather than assume.
	st, err := s.SeedVerify(ctx, scenarioID)
	if err != nil {
		return err
	}
	if st.AncestorStatus != "promoted" {
		return fmt.Errorf("wizard: seed verify: ancestor status = %q, want promoted", st.AncestorStatus)
	}
	if st.DescStatus != "entered" {
		return fmt.Errorf("wizard: seed verify: descendant status = %q, want entered", st.DescStatus)
	}
	if st.AncestorDebt != 0 {
		return fmt.Errorf("wizard: seed verify: ancestor debt = %d, want 0", st.AncestorDebt)
	}
	if st.DescDebt != len(kernel.FullDebt) {
		return fmt.Errorf("wizard: seed verify: descendant debt = %d, want %d", st.DescDebt, len(kernel.FullDebt))
	}
	if st.Edges != 1 {
		return fmt.Errorf("wizard: seed verify: edges = %d, want 1", st.Edges)
	}
	if st.Intents != 0 {
		return fmt.Errorf("wizard: seed verify: intents = %d, want 0", st.Intents)
	}
	if st.AncestorEvid != 1 {
		return fmt.Errorf("wizard: seed verify: ancestor evidence rows = %d, want 1", st.AncestorEvid)
	}
	return nil
}

// SeedVerify measures the seeded topology. It reports what it found and leaves the
// judging to the caller.
func (s *Server) SeedVerify(ctx context.Context, scenarioID string) (SeedState, error) {
	var st SeedState

	err := s.db.QueryRowContext(ctx, `
		SELECT id::STRING, status, coalesce(array_length(debt, 1), 0)
		  FROM belief WHERE scenario_id = $1::UUID AND claim = $2`,
		scenarioID, AncestorClaim).Scan(&st.AncestorID, &st.AncestorStatus, &st.AncestorDebt)
	if err == sql.ErrNoRows {
		return st, fmt.Errorf("wizard: seed verify: ancestor %q not found in scenario %s", AncestorClaim, scenarioID)
	}
	if err != nil {
		return st, fmt.Errorf("wizard: seed verify: read ancestor: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT id::STRING, status, coalesce(array_length(debt, 1), 0)
		  FROM belief WHERE scenario_id = $1::UUID AND claim = $2`,
		scenarioID, DescendantClaim).Scan(&st.DescendantID, &st.DescStatus, &st.DescDebt)
	if err == sql.ErrNoRows {
		return st, fmt.Errorf("wizard: seed verify: descendant %q not found in scenario %s", DescendantClaim, scenarioID)
	}
	if err != nil {
		return st, fmt.Errorf("wizard: seed verify: read descendant: %w", err)
	}

	// belief_edge carries no scenario_id (D-032), so it is reached through the parent.
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM belief_edge e
		  JOIN belief p ON p.id = e.parent_id
		 WHERE p.scenario_id = $1::UUID`, scenarioID).Scan(&st.Edges); err != nil {
		return st, fmt.Errorf("wizard: seed verify: count edges: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID`, scenarioID).Scan(&st.Intents); err != nil {
		return st, fmt.Errorf("wizard: seed verify: count intents: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID AND belief_id = $2::UUID`,
		scenarioID, st.AncestorID).Scan(&st.AncestorEvid); err != nil {
		return st, fmt.Errorf("wizard: seed verify: count ancestor evidence: %w", err)
	}
	return st, nil
}
