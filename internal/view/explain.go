// Package view — explain is a read-only projection that answers "why" for the
// four-verb product slice: Connect → Ask → Authorize → Reassess.
// It is strictly derived from the same SELECT-backed Snapshot used by
// solvent_ledger. It introduces no new source of truth, no writes, and no
// SQLSTATE invention.
//
// The explain output preserves engine evidence verbatim when a real DB
// operation has already produced it (23503 gate, 23514 promoted_is_debt_free,
// 23514 live_requires_promoted). When predicting what *would* fail before an
// attempted write, the result labels the SQLSTATE as a predicted value
// (predicted_*), not an observed engine error.
package view

import (
	"fmt"
	"strings"
)

// BeliefExplain is a per-belief human- and machine-readable explanation
// derived from a Snapshot. Every field is computed from the same SELECT rows
// that solvent_ledger already returns.
type BeliefExplain struct {
	BeliefID                     string   `json:"belief_id"`
	Claim                        string   `json:"claim"`
	ClaimType                    string   `json:"claim_type"`
	Status                       string   `json:"status"`
	RemainingDebt                []string `json:"remaining_debt"`
	FinalTruth                   bool     `json:"final_truth"`
	IsPromoted                   bool     `json:"is_promoted"`
	IsRetracted                  bool     `json:"is_retracted"`
	CanPromote                   bool     `json:"can_promote"`
	PromotionBlockedReason       string   `json:"promotion_blocked_reason,omitempty"`
	PredictedPromotionSQLState   string   `json:"predicted_promotion_sqlstate,omitempty"`
	PredictedPromotionConstraint string   `json:"predicted_promotion_constraint,omitempty"`
	CanAuthorize                 bool     `json:"can_authorize"`
	AuthorizationBlockedReason   string   `json:"authorization_blocked_reason,omitempty"`
	PredictedAuthSQLState        string   `json:"predicted_authorization_sqlstate,omitempty"`
	PredictedAuthConstraint      string   `json:"predicted_authorization_constraint,omitempty"`
	LiveIntents                  []Intent `json:"live_intents"`
	EvidenceCount                int      `json:"evidence_count"`
	HumanSummary                 string   `json:"human_summary"`
}

// ExplainResult is the top-level response for solvent_explain.
type ExplainResult struct {
	Scenario               string          `json:"scenario"`
	ScenarioID             string          `json:"scenario_id"`
	AuditLiveOnNonPromoted int             `json:"audit_live_on_nonpromoted"`
	Beliefs                []BeliefExplain `json:"beliefs"`
	GlobalSummary          string          `json:"global_summary"`
}

// ExplainSnapshot derives a human-readable and machine-readable explanation
// from a Snapshot. It performs no DB writes and introduces no new state.
func ExplainSnapshot(scenario string, scenarioID string, snap *Snapshot) *ExplainResult {
	res := &ExplainResult{
		Scenario:               scenario,
		ScenarioID:             scenarioID,
		AuditLiveOnNonPromoted: snap.AuditLiveOnNonPromoted,
	}

	// Index counts and intents by belief.
	evidenceByBelief := make(map[string]int)
	for _, e := range snap.Evidence {
		evidenceByBelief[e.BeliefID]++
	}
	intentsByBelief := make(map[string][]Intent)
	for _, it := range snap.Intents {
		intentsByBelief[it.BeliefID] = append(intentsByBelief[it.BeliefID], it)
	}

	for _, b := range snap.Beliefs {
		be := explainBelief(b, intentsByBelief[b.ID], evidenceByBelief[b.ID])
		res.Beliefs = append(res.Beliefs, be)
	}

	res.GlobalSummary = buildGlobalSummary(res)
	return res
}

func explainBelief(b Belief, intents []Intent, evidenceCount int) BeliefExplain {
	be := BeliefExplain{
		BeliefID:      b.ID,
		Claim:         b.Claim,
		ClaimType:     b.ClaimType,
		Status:        b.Status,
		RemainingDebt: b.Debt,
		FinalTruth:    b.FinalTruth,
		EvidenceCount: evidenceCount,
		IsPromoted:    b.Status == "promoted",
		IsRetracted:   b.Status == "retracted",
	}
	if be.RemainingDebt == nil {
		be.RemainingDebt = []string{}
	}

	// Filter live intents for this belief.
	var live []Intent
	for _, it := range intents {
		if it.State == "live" {
			live = append(live, it)
		}
	}
	if live == nil {
		live = []Intent{}
	}
	be.LiveIntents = live

	// CanPromote logic mirrors the DB CHECK promoted_is_debt_free.
	switch {
	case b.Status == "retracted":
		be.CanPromote = false
		be.PromotionBlockedReason = "belief is retracted — retracted beliefs cannot be promoted"
	case b.Status == "promoted":
		be.CanPromote = false
		be.PromotionBlockedReason = "already promoted"
	case b.FinalTruth:
		be.CanPromote = false
		be.PromotionBlockedReason = "blocked by final_truth=true — would violate promoted_is_debt_free"
		be.PredictedPromotionSQLState = "23514"
		be.PredictedPromotionConstraint = "promoted_is_debt_free"
	case len(b.Debt) > 0:
		be.CanPromote = false
		be.PromotionBlockedReason = fmt.Sprintf("blocked: %d unresolved obligation(s) remain: %s — predicted to violate promoted_is_debt_free", len(b.Debt), strings.Join(b.Debt, ", "))
		be.PredictedPromotionSQLState = "23514"
		be.PredictedPromotionConstraint = "promoted_is_debt_free"
	case b.Status == "entered":
		be.CanPromote = true
	default:
		be.CanPromote = false
		be.PromotionBlockedReason = fmt.Sprintf("blocked: unexpected status %q", b.Status)
	}

	// CanAuthorize logic mirrors the composite FK gate + live_requires_promoted.
	switch {
	case b.Status == "promoted" && len(b.Debt) == 0 && !b.FinalTruth:
		be.CanAuthorize = true
	case b.Status == "retracted":
		be.CanAuthorize = false
		be.AuthorizationBlockedReason = "belief is retracted — live intents cannot survive retraction (live_requires_promoted)"
		be.PredictedAuthSQLState = "23503"
		be.PredictedAuthConstraint = "gate"
	case b.Status != "promoted":
		be.CanAuthorize = false
		// Distinguish entered-with-debt vs entered-debt-free but not yet promoted vs other.
		if len(b.Debt) > 0 {
			be.AuthorizationBlockedReason = fmt.Sprintf("belief is not promoted (status=%q, %d debt remaining) — predicted to be refused by gate", b.Status, len(b.Debt))
		} else {
			be.AuthorizationBlockedReason = fmt.Sprintf("belief is not promoted (status=%q) — predicted to be refused by gate", b.Status)
		}
		be.PredictedAuthSQLState = "23503"
		be.PredictedAuthConstraint = "gate"
	case b.FinalTruth:
		be.CanAuthorize = false
		be.AuthorizationBlockedReason = "belief carries final_truth — cannot be promoted, therefore cannot authorize (promoted_is_debt_free)"
		be.PredictedAuthSQLState = "23503"
		be.PredictedAuthConstraint = "gate"
	default:
		be.CanAuthorize = false
		be.AuthorizationBlockedReason = "not promotable — predicted gate refusal"
		be.PredictedAuthSQLState = "23503"
		be.PredictedAuthConstraint = "gate"
	}

	be.HumanSummary = buildBeliefSummary(be)

	return be
}

func buildBeliefSummary(be BeliefExplain) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Belief %q", be.Claim))

	switch be.Status {
	case "entered":
		if len(be.RemainingDebt) == 0 && !be.FinalTruth {
			parts = append(parts, "is entered and has no remaining debt — ready to promote")
		} else if be.FinalTruth {
			parts = append(parts, "is blocked by final_truth=true")
		} else {
			parts = append(parts, fmt.Sprintf("is entered with %d unresolved obligation(s): %s", len(be.RemainingDebt), strings.Join(be.RemainingDebt, ", ")))
		}
	case "promoted":
		if len(be.LiveIntents) > 0 {
			actions := make([]string, 0, len(be.LiveIntents))
			for _, it := range be.LiveIntents {
				actions = append(actions, it.Action)
			}
			parts = append(parts, fmt.Sprintf("is promoted with %d live intent(s): %s", len(be.LiveIntents), strings.Join(actions, ", ")))
		} else {
			parts = append(parts, "is promoted — no live intents")
		}
	case "retracted":
		parts = append(parts, "is retracted")
		if len(be.LiveIntents) > 0 {
			parts = append(parts, fmt.Sprintf("but still has %d live intent(s) — would violate live_requires_promoted (23514) and must be cancelled first", len(be.LiveIntents)))
		} else {
			parts = append(parts, "and has no live intents (audit clean)")
		}
	}

	if be.EvidenceCount > 0 {
		parts = append(parts, fmt.Sprintf("supported by %d evidence row(s)", be.EvidenceCount))
	} else {
		parts = append(parts, "no evidence rows attached in this view (pass include_evidence=true)")
	}

	if be.CanPromote {
		parts = append(parts, "promotion would succeed (predicted)")
	} else if be.PromotionBlockedReason != "" && be.Status != "promoted" && be.Status != "retracted" {
		parts = append(parts, fmt.Sprintf("promotion predicted to be refused (%s)", be.PromotionBlockedReason))
	}

	if be.CanAuthorize {
		parts = append(parts, "authorization would succeed — belief is currently promoted")
	} else if be.AuthorizationBlockedReason != "" {
		parts = append(parts, fmt.Sprintf("authorization predicted to be refused (%s)", be.AuthorizationBlockedReason))
	}

	return strings.Join(parts, ". ") + "."
}

func buildGlobalSummary(res *ExplainResult) string {
	if len(res.Beliefs) == 0 {
		return fmt.Sprintf("Scenario %q has no beliefs. Audit live_on_nonpromoted=%d.", res.Scenario, res.AuditLiveOnNonPromoted)
	}
	promoted := 0
	retracted := 0
	liveIntents := 0
	for _, be := range res.Beliefs {
		if be.IsPromoted {
			promoted++
		}
		if be.IsRetracted {
			retracted++
		}
		liveIntents += len(be.LiveIntents)
	}
	status := fmt.Sprintf("Scenario %q: %d belief(s), %d promoted, %d retracted, %d live intent(s), audit_live_on_nonpromoted=%d", res.Scenario, len(res.Beliefs), promoted, retracted, liveIntents, res.AuditLiveOnNonPromoted)
	if res.AuditLiveOnNonPromoted != 0 {
		status += " — INVARIANT VIOLATION: live intents exist on non-promoted beliefs (should be 0)"
	} else {
		status += " — audit clean"
	}
	return status + "."
}
