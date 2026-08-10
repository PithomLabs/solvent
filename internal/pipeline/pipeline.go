// Package pipeline is the minimum integration layer for Wave 4.
//
// It chains normalize → derive → belief.Process → ProposeIfNew → intent.Audit
// into a single pass over evidence fixtures. The kernel owns correctness;
// this package is a thin orchestrator.
//
// Intent proposal is replay-idempotent under sequential execution;
// concurrent duplicate-intent prevention is deferred.
package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PithomLabs/solvent/internal/belief"
	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/internal/intent"
	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/normalize"
)

// Result captures the outcome of processing one evidence item through the pipeline.
type Result struct {
	Normalized    normalize.NormalizedEvidence
	Beliefs       []derive.DerivedBelief
	BeliefID      string // empty if contradiction or no belief emitted
	Promoted      bool
	DebtItems     []string // remaining debt after processing
	IntentState   string   // "live", "cancelled", or "" (no intent proposed)
	Contradiction bool     // true if this result is a contradiction (no ledger mutation)
}

// sourceTypeMap maps fixture filenames to their source types.
// The filename-to-source-type mapping is deterministic and compile-time.
var sourceTypeMap = map[string]string{
	"kev_entry.json":                normalize.SourceKEVEntry,
	"kev_entry_v2.json":             normalize.SourceKEVEntry,
	"release_with_fix.json":         normalize.SourceRelease,
	"maintainer_fixes.json":         normalize.SourceMaintainerComment,
	"maintainer_no_regression.json": normalize.SourceMaintainerComment,
	"maintainer_reviewed.json":      normalize.SourceMaintainerComment,
	"release_confirmed.json":        normalize.SourceRelease,
	"contradicting_evidence.json":   normalize.SourceMaintainerComment,
	"non_maintainer_comment.json":   normalize.SourceMaintainerComment,
	"unmerged_pr.json":              normalize.SourceGitHubPR,
}

// ProcessEvidence chains normalize → derive → belief.Process for one raw evidence item.
//
// Normal path:
//
//	normalize → derive → belief.Process (EnsureBelief, AddEvidence, RetireDebt, Promote)
//	→ Result with BeliefID set, Contradiction=false
//
// Contradiction path (DerivedBelief.Contradicts non-empty):
//
//	→ Result with BeliefID="", Contradiction=true
//	→ slog.Warn emitted with source_url
//	→ NO ledger mutation (belief.Process returns nil without DB writes)
func ProcessEvidence(ctx context.Context, db *sql.DB, scenarioID string, raw []byte, sourceType string) ([]Result, error) {
	norm, err := normalize.Normalize(raw, sourceType)
	if err != nil {
		return nil, fmt.Errorf("pipeline: normalize: %w", err)
	}

	derived := derive.Derive(norm)
	if len(derived) == 0 {
		return nil, nil
	}

	var results []Result
	for _, d := range derived {
		r := Result{
			Normalized: norm,
			Beliefs:    []derive.DerivedBelief{d},
		}

		// Contradiction path: no ledger mutation.
		if len(d.Contradicts) > 0 {
			r.Contradiction = true
			for _, c := range d.Contradicts {
				slog.Warn("pipeline: contradiction received, no ledger mutation",
					"source_url", c.SourceURL,
					"source_type", c.SourceType,
					"claim", d.Claim,
				)
			}
			results = append(results, r)
			continue
		}

		// Normal path: belief.Process → kernel operations.
		if err := belief.Process(ctx, db, scenarioID, d); err != nil {
			return nil, fmt.Errorf("pipeline: belief.Process: %w", err)
		}

		// Query the belief ID and state.
		beliefID, status, debtItems, err := queryBelief(ctx, db, scenarioID, d.Claim)
		if err != nil {
			return nil, fmt.Errorf("pipeline: query belief: %w", err)
		}

		r.BeliefID = beliefID
		r.Promoted = status == "promoted"
		r.DebtItems = debtItems
		results = append(results, r)
	}

	return results, nil
}

// mergedClaim pairs a DerivedBelief with the normalized provenance of its first
// supporting evidence item. Used to populate Result.Normalized in Run().
type mergedClaim struct {
	belief derive.DerivedBelief
	norm   normalize.NormalizedEvidence
}

// aggregateDeriveResults collects all fixture derive outputs and merges supporting
// evidence from derived beliefs that reference the same vulnerability into a single
// DerivedBelief. Each distinct CVE-bearing claim remains independent — different
// CVEs produce separate beliefs. Non-CVE evidence is merged into the first CVE-bearing
// claim that matches its CVE identifier. Contradictions are collected separately.
// Returns the merged beliefs (with provenance), sorted by claim for determinism,
// and the contradictions.
func aggregateDeriveResults(allFixtureResults [][]derive.DerivedBelief) ([]mergedClaim, []derive.DerivedBelief) {
	cvePattern := regexp.MustCompile(`CVE-\d{4}-\d{4,}`)

	type claimEntry struct {
		classification string
		evidence       []normalize.NormalizedEvidence
		norm           normalize.NormalizedEvidence // provenance of first supporting evidence
	}
	claims := make(map[string]*claimEntry)
	var contradictions []derive.DerivedBelief

	// Track the first CVE seen — it becomes the primary for non-CVE evidence.
	var primaryCVE string

	for _, fixtureResults := range allFixtureResults {
		for _, d := range fixtureResults {
			if len(d.Contradicts) > 0 {
				contradictions = append(contradictions, d)
				continue
			}
			if d.Claim == "" {
				continue
			}

			// Extract CVE identifier from the claim, if any.
			cve := ""
			if m := cvePattern.FindString(d.Claim); m != "" {
				cve = m
			}

			// CVE-bearing claims are their own primaries.
			if cve != "" {
				if primaryCVE == "" {
					primaryCVE = cve
				}
				// Same CVE as primary: merge evidence into the primary entry.
				if cve == primaryCVE {
					primaryClaim := d.Claim
					// Find the existing primary entry — it may have different claim text.
					var primaryEntry *claimEntry
					for text, entry := range claims {
						if entryCVE := cvePattern.FindString(text); entryCVE == primaryCVE {
							primaryEntry = entry
							_ = text
							break
						}
					}
					if primaryEntry != nil {
						primaryEntry.evidence = append(primaryEntry.evidence, d.SupportingEvidence...)
					} else {
						claims[primaryClaim] = &claimEntry{
							classification: d.Classification,
							evidence:       d.SupportingEvidence,
							norm:           firstNorm(d),
						}
					}
					continue
				}
				// Different CVE: independent claim.
				if entry, ok := claims[d.Claim]; ok {
					entry.evidence = append(entry.evidence, d.SupportingEvidence...)
				} else {
					claims[d.Claim] = &claimEntry{
						classification: d.Classification,
						evidence:       d.SupportingEvidence,
						norm:           firstNorm(d),
					}
				}
				continue
			}

			// Non-CVE evidence: merge into the primary CVE claim if one exists.
			if primaryCVE != "" {
				for text, entry := range claims {
					if entryCVE := cvePattern.FindString(text); entryCVE == primaryCVE {
						entry.evidence = append(entry.evidence, d.SupportingEvidence...)
						_ = text
						break
					}
				}
				continue
			}

			// No CVE seen yet: store normally.
			if entry, ok := claims[d.Claim]; ok {
				entry.evidence = append(entry.evidence, d.SupportingEvidence...)
			} else {
				claims[d.Claim] = &claimEntry{
					classification: d.Classification,
					evidence:       d.SupportingEvidence,
					norm:           firstNorm(d),
				}
			}
		}
	}

	// Build sorted results for determinism (P5).
	var sortedClaims []string
	for claim := range claims {
		sortedClaims = append(sortedClaims, claim)
	}
	sort.Strings(sortedClaims)

	var merged []mergedClaim
	for _, claim := range sortedClaims {
		entry := claims[claim]
		merged = append(merged, mergedClaim{
			belief: derive.DerivedBelief{
				Claim:              claim,
				Classification:     entry.classification,
				SupportingEvidence: entry.evidence,
			},
			norm: entry.norm,
		})
	}

	return merged, contradictions
}

// firstNorm returns the NormalizedEvidence from a DerivedBelief's first
// supporting evidence item, or a zero value if none exists.
func firstNorm(d derive.DerivedBelief) normalize.NormalizedEvidence {
	if len(d.SupportingEvidence) > 0 {
		return d.SupportingEvidence[0]
	}
	return normalize.NormalizedEvidence{}
}

// ProposeIfNew records a live action intent on a promoted belief only if no live intent
// already exists for the same belief/action pair. Idempotent: replaying the same evidence
// does not create duplicate intents.
//
// Intent proposal is replay-idempotent under sequential execution;
// concurrent duplicate-intent prevention is deferred.
func ProposeIfNew(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent
		 WHERE scenario_id=$1::UUID AND belief_id=$2::UUID AND action=$3::STRING AND state='live'`,
		scenarioID, beliefID, action).Scan(&count)
	if err != nil {
		return fmt.Errorf("pipeline: check existing intent: %w", err)
	}
	if count > 0 {
		return nil
	}
	return intent.Propose(ctx, db, scenarioID, beliefID, action)
}

// Run processes a directory of evidence fixtures in deterministic order.
// Returns results in processing order.
func Run(ctx context.Context, db *sql.DB, scenarioID string, fixtureDir string) ([]Result, error) {
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		return nil, fmt.Errorf("pipeline: read fixture dir: %w", err)
	}

	// Deterministic order: sort by filename.
	var filenames []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		filenames = append(filenames, e.Name())
	}
	sort.Strings(filenames)

	// Phase 1: derive all fixture evidence (no DB writes).
	var allFixtureResults [][]derive.DerivedBelief
	for _, name := range filenames {
		raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			return nil, fmt.Errorf("pipeline: read fixture %s: %w", name, err)
		}

		sourceType, ok := sourceTypeMap[name]
		if !ok {
			continue
		}

		norm, err := normalize.Normalize(raw, sourceType)
		if err != nil {
			return nil, fmt.Errorf("pipeline: normalize %s: %w", name, err)
		}

		allFixtureResults = append(allFixtureResults, derive.Derive(norm))
	}

	// Phase 2: aggregate evidence by claim.
	merged, contradictions := aggregateDeriveResults(allFixtureResults)

	// Phase 3: process aggregated beliefs through the ledger.
	var allResults []Result

	for _, mc := range merged {
		d := mc.belief
		if err := belief.Process(ctx, db, scenarioID, d); err != nil {
			return nil, fmt.Errorf("pipeline: belief.Process: %w", err)
		}

		beliefID, status, debtItems, err := queryBelief(ctx, db, scenarioID, d.Claim)
		if err != nil {
			return nil, fmt.Errorf("pipeline: query belief: %w", err)
		}

		allResults = append(allResults, Result{
			Normalized: mc.norm,
			BeliefID:   beliefID,
			Promoted:   status == "promoted",
			DebtItems:  debtItems,
			Beliefs:    []derive.DerivedBelief{d},
		})
	}

	// Phase 3.5: propose intents for promoted beliefs.
	for i := range allResults {
		r := &allResults[i]
		if r.Promoted && r.BeliefID != "" {
			if err := ProposeIfNew(ctx, db, scenarioID, r.BeliefID, "upgrade"); err != nil {
				return nil, fmt.Errorf("pipeline: propose intent: %w", err)
			}
			r.IntentState = "live"
		}
	}

	// Phase 4: record contradictions with warning (F1). No ledger mutation.
	for _, c := range contradictions {
		for _, ev := range c.Contradicts {
			slog.Warn("pipeline: contradiction received, no ledger mutation",
				"source_url", ev.SourceURL,
				"source_type", ev.SourceType,
				"claim", c.Claim,
			)
		}
		contradictionNorm := firstNorm(c)
		if contradictionNorm.SourceType == "" && len(c.Contradicts) > 0 {
			contradictionNorm = c.Contradicts[0]
		}
		allResults = append(allResults, Result{
			Normalized:    contradictionNorm,
			Contradiction: true,
			Beliefs:       []derive.DerivedBelief{c},
		})
	}

	return allResults, nil
}

// queryBelief returns the belief ID, status, and remaining debt items for a claim.
func queryBelief(ctx context.Context, db *sql.DB, scenarioID, claim string) (id, status string, debt []string, err error) {
	var debtRaw string
	err = db.QueryRowContext(ctx,
		`SELECT id, status, debt::STRING
		 FROM belief WHERE scenario_id=$1::UUID AND claim=$2`,
		scenarioID, claim).Scan(&id, &status, &debtRaw)
	if err != nil {
		return "", "", nil, err
	}
	debt = parsePGArray(debtRaw)
	return id, status, debt, nil
}

// parsePGArray parses a PostgreSQL text array literal like {a,b,c} into a string slice.
func parsePGArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return []string{}
	}
	// Strip outer braces.
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// AuditIntent returns the audit count for the scenario.
func AuditIntent(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
	return intent.Audit(ctx, db, scenarioID)
}

// QueryBeliefCount returns the number of beliefs in the scenario.
func QueryBeliefCount(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, scenarioID).Scan(&count)
	return count, err
}

// QueryEvidenceCount returns the number of evidence rows in the scenario.
func QueryEvidenceCount(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID`, scenarioID).Scan(&count)
	return count, err
}

// QueryIntentCount returns the number of live intents in the scenario.
func QueryIntentCount(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='live'`, scenarioID).Scan(&count)
	return count, err
}

// IsActionOnUnpromoted reports whether err is ErrActionOnUnpromoted.
func IsActionOnUnpromoted(err error) bool {
	return errors.Is(err, kernel.ErrActionOnUnpromoted)
}
