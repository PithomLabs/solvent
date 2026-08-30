// Package belief wires DerivedBelief outputs into the Transactional Belief Ledger.
//
// It is a thin translation layer — no business logic, no orchestration.
// Each step is a direct kernel call. The kernel owns correctness.
package belief

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/kernel"
)

// Process translates a single DerivedBelief into kernel operations.
//
// Each step is a direct kernel call:
//  1. EnsureBelief → beliefID
//  2. AddEvidence for each SupportingEvidence item (deduplicated by content_sha256)
//  3. RetireDebt for each debt item the evidence covers
//  4. Promote (may fail with ErrPromotionBlocked — acceptable)
//
// Contradictions bypass steps 2-4 and call RetractCascade.
//
// Error handling:
//   - ErrPromotionBlocked → nil (debt not yet fully retired)
//   - Other errors → returned immediately (fail loudly)
func Process(ctx context.Context, db *sql.DB, scenarioID string, b derive.DerivedBelief) error {
	if b.Claim == "" {
		return fmt.Errorf("belief.Process: claim must not be empty")
	}

	st := kernel.New(db)

	// Contradictions: retraction path.
	if len(b.Contradicts) > 0 {
		for _, c := range b.Contradicts {
			slog.Warn("belief.Process: contradiction received, automatic retraction deferred",
				"source_url", c.SourceURL,
				"source_type", c.SourceType,
				"claim", b.Claim,
			)
		}
		return nil
	}

	// Translation: DerivedBelief → kernel.ClaimType
	ct := kernel.Derived
	if b.Classification == "accommodated" {
		ct = kernel.Accommodated
	}

	// Step 1: Ensure belief exists.
	beliefID, err := st.EnsureBelief(ctx, scenarioID, b.Claim, ct)
	if err != nil {
		return err
	}

	// Step 2: Attach supporting evidence (deduplicated by content_sha256).
	for _, e := range b.SupportingEvidence {
		exists, err := evidenceExists(ctx, db, beliefID, e.ContentSHA256)
		if err != nil {
			return fmt.Errorf("belief.Process: check evidence exists: %w", err)
		}
		if exists {
			continue
		}
		if err := st.AddEvidence(ctx, scenarioID, beliefID,
			e.ProvenanceClass, e.SourceURL, e.ContentSHA256); err != nil {
			return err
		}
	}

	// Step 3: Retire debt items the evidence covers.
	for _, e := range b.SupportingEvidence {
		items := DebtItemsForEvidence(e.SourceType, e.Assertion)
		for _, item := range items {
			if err := st.RetireDebt(ctx, beliefID, item); err != nil {
				return err
			}
		}
	}

	// Step 4: Promote (may fail with ErrPromotionBlocked — acceptable).
	if err := st.Promote(ctx, beliefID); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, kernel.ErrPromotionBlocked) {
			return nil
		}
		return fmt.Errorf("belief.Process: promote %s: %w", beliefID, err)
	}

	return nil
}

// evidenceExists checks if evidence with the given content_sha256 already exists
// for this belief. TOCTOU window accepted for MVP (see wave3_qa.md).
func evidenceExists(ctx context.Context, db *sql.DB, beliefID, contentSHA256 string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE belief_id=$1::UUID AND content_sha256=$2`,
		beliefID, contentSHA256).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
