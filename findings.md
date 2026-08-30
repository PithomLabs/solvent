# Solvent Adversarial Design Review

**Source:** VVAH analysis (`vvah2.md`) + deep inspection of the Solvent codebase on disk.
**Status:** Read-only review. No files modified. No implementation proposed.
**Scope:** Full codebase — schema, kernel, Lean model, corpus, wizard, MCP, pipeline, tests, documentation.

---

## Executive Verdict

**STRENGTHEN SELECTIVELY**

Solvent already solves the hardest problems VVAH exposes: the database, not the LLM, decides whether authority is legal; a belief that carries debt cannot be promoted; an intent cannot survive its belief's retraction. These are not prose commitments — they are CHECK constraints, composite foreign keys, and ON UPDATE CASCADE behaviors that no code path can bypass.

The VVAH analysis is valuable but was produced without filesystem access to Solvent. It mischaracterizes several things Solvent already handles (evidence provenance classes, citation relations, refusal logging) and proposes features that would dilute the frozen architecture. The genuinely useful insights are narrower than the analysis suggests: typed evidence quality, staleness/freshness, and multi-agent authority scoping. None requires redesign.

The recommended evolution is **four targeted additions**, not a rearchitecture.

---

## Verified Current Architecture

### What Actually Exists (Read From Code)

**7 tables in two layers:**

| Layer | Tables | Source |
|---|---|---|
| Frozen ledger | `belief`, `belief_edge`, `evidence`, `action_intent` | `db/001_schema.sql` |
| Corpus/wizard | `corpus_issue`, `belief_corpus_citation`, `refusal_log` | `db/002_corpus.sql`, `db/003_wizard.sql` |

**Kernel API (8 functions, all in `kernel/kernel.go`):**
- `EnterBelief`, `AddEvidence`, `RetireDebt`, `Promote`, `IntentOnPromoted`, `RetractCascade`, `AuditLiveOnNonPromoted`, `EnsureBelief`

**Schema invariants (enforced by CockroachDB, not Go):**
- I-1: `promoted_is_debt_free` CHECK — promoted beliefs must have empty debt and `final_truth=false`
- I-2: `live_requires_promoted` CHECK — live intents must reference promoted beliefs
- I-3: composite FK `gate` with `ON UPDATE CASCADE` — retraction cascades into intent rows
- I-4: `belief_id_status_key UNIQUE` — FK target for the cascade

**Lean formal model (`formal/lean/`):**
- 11 theorems, zero `sorry`/`admit`/`unchecked axiom`
- Proves: `promote_preserves_validity`, `cancelIntent_preserves_validity`, `authorizeIntent_preserves_validity`, `retractCascade_preserves_validity`
- Proves: `live_intent_implies_promoted`, `no_live_intent_on_retracted_belief`
- Proves cascade theorems: `promotion_updates_dependent_intent`, `cascade_retraction_updates_dependent_intent`, `cascade_retraction_cannot_leave_live_intent`, `cascade_update_preserves_gate`, `cascade_retraction_blocks_live_intent`

**Evidence provenance model (already exists):**
- `evidence.provenance_class` ∈ {`external_feed`, `reproducible_artifact`, `live_scan`, `operator_asserted`}
- `belief_corpus_citation.relation` ∈ {`considered`, `contradicts`}
- `evidence.content_sha256` NOT NULL — hash-pinned
- `corpus_issue.content_sha256` NOT NULL — idempotent ingestion

**Retrieval-provenance (already exists):**
- `belief_corpus_citation` records: `belief_id`, `corpus_id`, `distance`, `query_text`, `retrieved_at`, `relation`
- The query text and measured distance are on the record — an auditor can re-run the search

**Refusal logging (already exists):**
- `refusal_log` records: `statement`, `sqlstate`, `constraint_name`, `detail`, `logged_at`

**Debt model:**
- `TEXT[]` array on belief, 6 items: `needProvenanceCheck`, `needContradictionSweep`, `needBlastRadius`, `needRollbackPlan`, `needVersionPin`, `needOperatorSignoff`
- Boolean: an item is present or absent. No types, no quorum, no cardinality.

---

## What the VVAH Analysis Gets Right

**1. "Findings are triage candidates, not confirmed vulnerabilities" → Solvent's debt/promotion gate**
VVAH's core gap is that "human review is required" is prose, not a schema constraint. Solvent's `CHECK (status <> 'promoted' OR debt = '{}')` is the structural version of exactly this: a claim cannot become actionable until its debt is cleared. This is genuinely stronger than VVAH's model.

**2. "Three human gates are organizational commitments, not structural constraints" → composite FK gate**
VVAH's article describes three human gates (before run, at patch review, before merge) that are procedural. Solvent's `gate` FK makes "action on non-promoted belief" literally unrepresentable. This is a real architectural advantage.

**3. "GhostJacking is retrieval becoming authority" → Solvent's core thesis**
The DEF CON attack chain (agent reads payload from log → rewrites DNS) is exactly the failure mode Solvent exists to prevent. An agent's read can produce a wrong belief but cannot turn that belief directly into a credentialed action without passing through the promotion gate.

**4. MTTA's ambiguity → Solvent's immutable ledger**
"One metric, three definitions" is a deployment tracking problem. Solvent's model of immutable, timestamped belief-state transitions (entered → promoted → retracted) with cancellation timestamps makes lifecycle metrics derivable from ledger rows rather than self-reported status.

**5. Voting disabled by default → schema-level fail-closed**
VVAH's FP defense silently collapses to single-pass. Solvent's equivalent: if you cannot discharge the debt, you cannot promote, period. There is no "weaker mode" that silently passes.

---

## What the VVAH Analysis Gets Wrong About Solvent

**1. "Solvent doesn't distinguish normal/incomplete/degraded/failed evidence"**
Wrong. `evidence.provenance_class` already distinguishes four evidence origins. `belief_corpus_citation.relation` distinguishes "considered during discharge" from "introduced as contradiction." The refusal log records every database refusal with its SQLSTATE and constraint name. The evidence model is more differentiated than VVAH assumes.

**2. "Solvent doesn't record retrieval provenance"**
Wrong. `belief_corpus_citation` stores the exact query text, measured distance, and retrieval timestamp for every citation. This is the auditable retrieval receipt VVAH asks for — it already exists.

**3. "Solvent can't distinguish 'agent failed to find it' from 'corpus didn't contain it'"**
Partially wrong. The retrieval receipt shows what was returned. The corpus is a known, countable set (7,239 issues). The measured rank of the falsifier (573/7,239) is an offline measurement. An auditor can distinguish these — the information is on the record.

**4. "Solvent needs evidence redaction"**
Misleading. The evidence table stores `source_url`, `content_sha256`, and `provenance_class` — not raw text. The `snapshot` JSONB column is nullable and currently only used by the corpus layer for issue metadata, not for raw payloads containing secrets. The `refusal_log` stores SQLSTATE and constraint names, not evidence content. There is no path where credentials or PII enter the ledger.

**5. "Solvent needs typed/quorum debt"**
This is a genuine insight, but VVAH presents it as a gap when it is better understood as a deliberate simplification. The six debt items are demo-scoped deployment-review obligations, not a general debt framework. The mechanism (array removal) is correct for the current scope. Typed debt is a future evolution, not a current weakness.

**6. "Solvent needs authority as a first-class object"**
Authority is already a first-class concept — it is what the schema enforces. `action_intent` IS the authority object: it names the belief, carries the action, and is gated by the composite FK. Adding an intermediate "authority grant" table would duplicate what the CHECK + FK already do.

---

## Current Weaknesses

Ranked by severity:

| # | Weakness | Severity | Classification |
|---|---|---|---|
| W-1 | Debt is flat `TEXT[]` — no typing, no cardinality, no independence requirement | Medium | DOCUMENTED-ONLY |
| W-2 | No staleness/freshness model — beliefs never expire or require revalidation | Medium | UNVERIFIED |
| W-3 | No multi-agent authority scoping — `scenario_id` isolates but doesn't scope authority per agent | Low | DOCUMENTED-ONLY |
| W-4 | `belief_edge` has no writer in kernel — edges are demo scaffolding only | Low | DOCUMENTED-ONLY |
| W-5 | Contradiction path in `belief.Process` is a no-op (logs warning, no ledger mutation) | Low | CODE-ENFORCED |
| W-6 | Lean model proves abstract state machine, not Go/CockroachDB refinement | Low | FORMALLY-PROVED (abstract) |
| W-7 | No `ON DELETE` behavior on `evidence.belief_id` FK — cascade semantics implicit | Low | UNVERIFIED |
| W-8 | `corpus_issue.embedding` nullable — intermediate state, not a gap | None | DOCUMENTED-ONLY |

---

## Recommended Architecture Evolution

### The VVAH-Suggested Insights Worth Taking

```text
Evidence
   │
   ▼
Provenance Quality (NEW: evidence_quality column)
   │
   ▼
Belief
   │
   ▼
Typed Debt (FUTURE: debt as structured items, not flat array)
   │
   ▼
Staleness Guard (NEW: validity_horizon on belief)
   │
   ▼
Promotion
   │
   ▼
Scoped Authority (FUTURE: agent_id on action_intent)
   │
   ▼
Action Intent
   │
   ▼
Execution / Retraction
```

**But only the first two are justified by the actual review.** The rest are future evolutions.

### What NOT to Add

| VVAH Suggestion | Verdict | Reason |
|---|---|---|
| Event sourcing / append-only event log | REJECT | The ledger IS the event log. Adding a parallel event table duplicates truth. |
| Confidence scores as columns | REJECT | Confidence is derived display, never a column (AGENTS.md §8 non-goal). |
| Authority grant as separate table | REJECT | `action_intent` + composite FK already IS the authority object. |
| Quorum voting on debt | REJECT for now | The six debt items are demo-scoped. Typed debt is a future evolution. |
| Multi-model orchestration | REJECT | Solvent is domain-agnostic. Embedding provider is a config concern, not an architecture concern. |

---

## Priority Matrix

| Priority | Problem | Proposed Change | Why | Scope | Risk |
|---|---|---|---|---|---|
| P0 | No staleness model | Add `validity_horizon` TIMESTAMPTZ to belief | A belief whose evidence is 6 months old should not authorize a deploy without revalidation. This is the single most important post-demo extension. | Schema + kernel + wizard | Low — additive, no invariant change |
| P1 | Flat debt has no quality signal | Add `evidence_quality` TEXT column to evidence table | Distinguish `deterministic` (build passed, hash verified) from `attested` (LLM judgment, human review). Promotion logic can then require at least N deterministic discharges. | Schema + kernel | Low — additive column, existing logic unchanged |
| P2 | No multi-agent authority scoping | Add `agent_id` UUID to action_intent | Two agents forming competing beliefs need separate authority scopes. The current `scenario_id` isolation is a demo convenience, not a production boundary. | Schema + kernel | Medium — FK changes, existing tests need update |
| P3 | Typed debt model | Restructure debt from `TEXT[]` to a `debt_item` table | Enable per-item type, cardinality, independence requirement, discharge proof. This is the natural next abstraction. | Schema rewrite (ledger tables) | HIGH — frozen architecture change, requires architect approval |

---

## Database Evolution

### P0: Staleness (Validated Before Implementation)

```sql
ALTER TABLE belief ADD COLUMN validity_horizon TIMESTAMPTZ;
-- NULL means "no expiry" (default for existing rows)
-- Non-NULL means "must be revalidated before this time"
```

No CHECK constraint change. No invariant change. The kernel adds one filter: `WHERE validity_horizon IS NULL OR validity_horizon > now()` when considering promotion eligibility. This can be a VIEW or a kernel-side check — the schema does not need to enforce it.

### P1: Evidence Quality

```sql
ALTER TABLE evidence ADD COLUMN evidence_quality TEXT NOT NULL DEFAULT 'attested'
  CHECK (evidence_quality IN ('deterministic', 'attested', 'degraded'));
```

`deterministic`: build passed, hash verified, source record matched, database constraint passed.
`attested`: LLM judgment, human review, model-generated summary.
`degraded`: partial result, tool failure, uncertain output.

This is additive. Existing `belief.Process` logic is unchanged. The quality signal is available for promotion logic to consult.

---

## Go Kernel Evolution

### P0: Staleness Check

Add to `kernel/kernel.go`:
```go
func (s *Store) IsStale(ctx context.Context, beliefID string) (bool, error)
```

Returns true if `validity_horizon IS NOT NULL AND validity_horizon <= now()`. The wizard and pipeline can call this before promotion or intent creation.

### P1: Evidence Quality

No kernel change needed. The `evidence_quality` column is set by the caller (pipeline/wizard) and stored by `AddEvidence`. The kernel does not interpret it — that is the application's job.

---

## Lean Evolution

The current Lean model proves preservation properties for the four safe transitions. The VVAH analysis suggests additional theorems:

| Proposed Theorem | Value | Recommendation |
|---|---|---|
| `stale_authority_cannot_become_live` | Medium | DEFER — requires adding time to the abstract model, which is a significant scope expansion |
| `deterministic_debt_required_for_high_risk` | Low | DEFER — typed debt is P3, too early to formalize |
| `retraction_invalidates_all_dependent_authority` | Already proved | `cascade_retraction_cannot_leave_live_intent` + `no_live_intent_on_retracted_belief` already cover this |
| `degraded_evidence_cannot_satisfy_deterministic_debt` | Medium | DEFER — depends on P1 evidence quality |

**Recommendation:** The current 11 theorems are sufficient for the hackathon. Post-hackathon, the highest-value addition is `stale_authority_cannot_become_live` once P0 is implemented.

---

## Verification Strategy

| Change | Unit Tests | Integration Tests | Adversarial Tests | DB Proofs | Lean Proofs |
|---|---|---|---|---|---|
| P0 (staleness) | `IsStale` returns correct bool | Promotion with expired horizon is refused | Promote → time passes → intent creation | CHECK unaffected | DEFERRED |
| P1 (evidence quality) | `evidence_quality` stored correctly | Quality column populated by pipeline | Deterministic evidence required for high-risk action | CHECK constraint holds | DEFERRED |
| P2 (agent scoping) | Intent scoped to agent | Cross-agent intent refused | Agent A retracts, agent B's intent survives | FK + CHECK unaffected | DEFERRED |
| P3 (typed debt) | Debt table CRUD | Promotion with typed debt | Quorum discharge | NEW CHECK constraints | NEW theorems |

---

## Hackathon Recommendation

**Do not change anything before submission.**

The submission is technically strong. The three schema invariants are demonstrated, the Lean model is complete, the control experiment is measured, and the demo path is verified.

Specifically:
- Do NOT add staleness (P0) — it is important but does not improve the demo
- Do NOT add evidence quality (P1) — it adds complexity without judge-visible value
- Do NOT add agent scoping (P2) — the single-agent demo is the right scope
- Do NOT add typed debt (P3) — this is a post-hackathon evolution

The current architecture is the right shape for the hackathon. The VVAH insights validate the direction; they do not require pre-submission changes.

---

## Post-Hackathon Roadmap

**Phase 1 (Weeks 1-2): Staleness**
- Add `validity_horizon` to belief
- Implement `IsStale` in kernel
- Update wizard to show staleness status
- Add staleness test to invariant suite

**Phase 2 (Weeks 3-4): Evidence Quality**
- Add `evidence_quality` to evidence table
- Update pipeline to set quality based on source type
- Update promotion logic to optionally require deterministic discharges
- Add quality-aware tests

**Phase 3 (Month 2): Multi-Agent Scoping**
- Add `agent_id` to action_intent
- Implement cross-agent authority rules
- Update scenario isolation to include agent scoping
- Add concurrency tests

**Phase 4 (Month 3+): Typed Debt**
- Evaluate whether to restructure debt from `TEXT[]` to a table
- If yes, design the debt_item schema with type, cardinality, independence
- This is the most architecturally significant change — requires full review

---

## New Insights

**1. Evidence quality is not confidence.**
VVAH's precision/recall is a pipeline metric. Solvent's evidence quality is a property of individual evidence rows. A finding can have high confidence (the LLM is sure) but low quality (it's attested, not deterministic). These are orthogonal dimensions. Architectural consequence: `evidence_quality` and confidence are separate concepts; never conflate them.

**2. The refusal log is already an event history.**
VVAH suggests an append-only event log for lifecycle metrics. Solvent already has `refusal_log` — every database refusal is recorded with SQLSTATE, constraint name, and timestamp. Lifecycle timestamps can be derived from the existing ledger transitions (entered, promoted, retracted) plus refusal log entries. No new event table needed.

**3. Retrieval provenance is already auditable.**
`belief_corpus_citation` stores query text, distance, and timestamp. An auditor can re-run the exact search and compare results. This is the "retrieval receipt" VVAH asks for — it already exists at `db/002_corpus.sql:76-83`.

**4. The TOCTOU window in belief.Process is accepted, not a bug.**
`evidenceExists` checks before `AddEvidence` inserts — there is a theoretical race. This is documented as accepted for MVP (`wave3_qa.md`). The uniqueness constraint is on `(belief_id, content_sha256)` which does not exist in the schema — the guard is application-level. This is honest about its limitation rather than pretending it is safe.

**5. The Lean model's abstraction gap is intentional, not a weakness.**
The Lean model proves properties of an abstract state machine. CockroachDB proves the real engine's refusal behavior empirically. This is a deliberate separation of concerns: Lean proves the design is correct; the database proves the implementation matches the design. A refinement proof would be valuable but is a long-term research project, not a gap to close now.

**6. Solvent's domain-agnosticism is stronger than VVAH assumes.**
VVAH discusses per-stage model orchestration. Solvent's kernel has no concept of models, stages, or pipelines. The `EvidenceFeed` interface is the only extension point, and changing the evidence source requires no kernel changes. This is the right level of abstraction for a belief ledger.

**7. The wizard's "no handler-side precondition" rule is load-bearing.**
`internal/wizard/wizard.go:11-17` explicitly states that no handler should check debt before calling Promote. If a handler ever grows `if len(debt) > 0 { return }`, the demo stops demonstrating anything — the refusal would be the application's opinion rather than the schema's guarantee. This rule must survive any evolution.

---

## Final Target Model

```text
Evidence (with provenance quality)
   │
   ▼
Belief (with validity horizon)
   │
   ▼
Typed Debt (with cardinality and independence)
   │
   ▼
Promotion (with staleness check)
   │
   ▼
Scoped Authority (per agent, per risk tier)
   │
   ▼
Action Intent
   │
   ▼
Execution / Retraction / Expiry
```

The core stays the same: retrieval proposes, belief carries debt, promotion gates authority, the database enforces. The additions are quality signals on evidence, time bounds on beliefs, and scope on authority — all additive, all preserving the frozen invariant structure.

Solvent's thesis is correct and its implementation is strong. The VVAH analysis confirms the direction. The right next step is not redesign — it is the measured, selective evolution described above.
