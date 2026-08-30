# ADR-0001: Authority, Warrant, and Attestation Model

## Status

**Accepted for post-hackathon architecture; not implemented in current hackathon build.**

This record captures the post-hackathon decisions that survived adversarial review of the Solvent strategy corpus (`review_claude.md`, `review_chatgpt.md`, `review2_chatgpt.md`, `review2_claude.md` in `/v2`, consolidated as the Solvent_DocTrust adversarial review). No decision in this ADR modifies the current hackathon implementation. Where this ADR and the deployed code or schema disagree on what is current, the code and schema win.

The hackathon implementation is frozen per `AGENTS.md:52-57` and `IMPLEMENTATION_CONTRACT.md:2-8`.

---

## Context

### What Solvent implements today

Solvent is a transactional belief ledger. Four frozen tables carry the authority semantics (`db/001_schema.sql:6-76`):

- `belief` (`id, scenario_id, claim, claim_type, status, debt TEXT[], final_truth`) — a working premise with open review obligations
- `belief_edge` (`parent_id, child_id, kind`) — belief-to-belief derivation graph, not traversed by the database
- `evidence` (`scenario_id, belief_id, provenance_class, source_url, snapshot, content_sha256`) — attributable record of what was observed
- `action_intent` (`scenario_id, belief_id, belief_status, action, state`) — a live intent to take a real-world action citing a belief as its warrant

Three corpus/wizard tables layer retrieval and audit (`corpus_issue, belief_corpus_citation, refusal_log` — `db/003_wizard.sql:1-61`) but carry no authority semantics.

The lifecycle is `evidence → belief → debt → promotion → action_intent` (`AGENTS.md:15-27`, `README.md:28-33`, `kernel/kernel.go:52-161`). Three schema-level mechanisms carry the entire authority guarantee (`README.md:122-137`, `db/001_schema.sql:30-75`):

| Invariant | Constraint | SQLSTATE | Enforcement |
|---|---|---|---|
| `promoted_is_debt_free` | `CHECK (status <> 'promoted' OR (debt='{}' AND NOT final_truth))` on `belief` | `23514` | CockroachDB |
| `gate` | `FOREIGN KEY (belief_id, belief_status) REFERENCES belief(id,status) ON UPDATE CASCADE` on `action_intent` | `23503` | CockroachDB |
| `live_requires_promoted` | `CHECK (state <> 'live' OR belief_status='promoted')` on `action_intent` | `23514` | CockroachDB (re-evaluated on rows touched by `ON UPDATE CASCADE`, measured at `docs/M0_REPORT.md:42-47`) |

Supporting invariants `I-5` through `I-8` (`IMPLEMENTATION_CONTRACT.md:5`, `docs/POST_HACKATHON_ARCHITECTURE.md:1.2`) govern `AuditLiveOnNonPromoted` (=0 in every committed state), absence of vectors on ledger tables (`I-6`), all writes through `crdb.ExecuteTx` (`I-7`, `kernel/kernel.go:59` + `scripts/check_i7.sh`), and `RetractCascade` single-transaction cancel-before-retract ordering (`I-8`, `kernel/kernel.go:127-149`, `kernel/sql.go:66-92`).

The kernel (`kernel/kernel.go:1`, `kernel/contract.go:1`) exposes eight functions — `EnterBelief, AddEvidence, RetireDebt, Promote, IntentOnPromoted, RetractCascade, EnsureBelief, AuditLiveOnNonPromoted` — all writes through `crdb.ExecuteTx`. The only current external surface is a local stdio MCP server (`cmd/solvent-mcp/main.go:1`, `tools.go:1`) with six tools: `solvent_ledger, solvent_ingest_evidence, solvent_retire_debt, solvent_promote, solvent_authorize_action, solvent_falsify`. The debt vocabulary is six `TEXT[]` items stored in `belief.debt` and mirrored in `kernel.FullDebt` (`kernel/kernel.go:31`): `needProvenanceCheck, needContradictionSweep, needBlastRadius, needRollbackPlan, needVersionPin, needOperatorSignoff` — `promoted_is_debt_free` checks cardinality (`array_length(debt,1)=0`), never element values, so the vocabulary can evolve without rebuilding constraints (`db/004_debt_vocabulary.sql:1-47`).

The deployed demo asks `Is etcd v3.5.x safe to deploy?` against 7,239 ingested etcd issues. `AGENTS.md:82-93` and `README.md:80-106` measured values are preserved verbatim. Formal model `formal/lean/` proves abstract state-machine preservation (`promote_preserves_validity`, `retractCascade_preserves_validity`, `live_intent_implies_promoted`, `no_live_intent_on_retracted_belief`, etc.) with zero `sorry` — not a refinement proof of the Go/CockroachDB implementation (`README.md:167-201`).

### What the adversarial review found

`review_claude.md` Parts I-II (Findings 1-14) and `review2_chatgpt.md` / `review2_claude.md` converge on four load-bearing gaps *around* this kernel, not in it:

1.  **Debt retirement is the weak path (Finding 4, CRITICAL).** The `gate` is unforgeable, but `Promote` only requires `debt='{}'`, and `RetireDebt` (`kernel/kernel.go:83`, `kernel/sql.go:22-24` `array_remove`) is idempotent — retiring an absent item is a silent no-op — while `solvent_retire_debt` (`cmd/solvent-mcp/tools.go:102-164`) is invoked by the **agent** after presenting a plain-language question to a human in the same chat surface. A convincingly framed question, an alert-fatigued reviewer, or a compromised elicitation channel can retire debt as easily as a legitimate review. DocTrust's separate, Ed25519-signed human-consent channel is strictly stronger than Solvent's current agent-mediated path (`review_claude.md:83-85`).

2.  **Warrant portability is unresolved (Finding 6, HIGH).** `product.md` / `strategy.md` want a portable Warrant `any executor can carry, transmit, cache, verify, and present` — but never choose between (a) signed self-contained token (PKI/rotation/revocation/expiry liability) and (b) live central verification (every gated action has a hard external dependency on Solvent). These have opposite operational consequences (`review_claude.md:91-97`).

3.  **Self-referential TCB (Finding 5, HIGH).** Once Solvent is `the layer every consequential action routes through`, its own evidence-ingestion endpoint becomes the highest-value target in the stack — compromising it forges the authorization record every downstream executor trusts. The GhostJacking lesson `the tool trusted most becomes the way in` applies with more force to Solvent than to any vendor it defends (`review_claude.md:87-89`).

4.  **Roadmap assumes a team that does not exist (Findings 1-2, CRITICAL; Finding 3, HIGH).** The pre-review strategy specified Cloud + Edge + Core + MCP + A2A + Connector SDK + multi-tenancy + five identity standards + risk tiers + shadow mode + OEM + standards work on a solo-founder base, and `multi-tenancy first-class from day one` directly violates the frozen-kernel rule (`AGENTS.md:55`, `review_claude.md:70-74`).

This ADR is the subtraction that survived: eight narrow decisions, one frozen kernel, everything else deferred until customer evidence justifies it.

---

## Problem

### The strong gate

The `gate` + `promoted_is_debt_free` + `live_requires_promoted` triad genuinely separates belief from authority. `kernel/kernel.go:107-111` `IntentOnPromoted` refuses `23503` unless `belief(status='promoted')` *right now* via a composite FK, not application logic. `kernel/kernel.go:127-149` `RetractCascade` cancels intents before retracting — and even if reversed, `ON UPDATE CASCADE` propagates the new status into `action_intent.belief_status` and `live_requires_promoted` detonates, refusing the transaction. The M3 control experiment (`/proof`, `README.md:145-164`) shows a naive schema at `READ COMMITTED` committing `AUDIT=1` with correct application logic, while the hardened schema refuses `23503 · gate` at the same isolation.

### The weaker debt-retirement path

That strength collapses if the thing that empties `debt` is weak. Today it is:

```text
human answer (plain language, in the agent's chat surface)
  → agent interprets the answer
  → agent calls solvent_retire_debt(scenario, belief_id, debt_item)     // cmd/solvent-mcp/tools.go:102
  → kernel.RetireDebt array_remove                                       // kernel/kernel.go:83, kernel/sql.go:22
  → if debt='{}' then Promote succeeds (23514 would have blocked)       // kernel/kernel.go:95
  → IntentOnPromoted now succeeds via gate
```

No cryptographic binding exists between `the human actually verified this for this exact belief and this exact obligation` and `the row now says the item is gone`. Validity is checked in the MCP handler (`tools.go:129-132` rejects unknown items, but retiring an already-discharged real item stays a no-op per kernel contract), not by an independent channel or signature. The agent that benefits from promotion is the same agent that reports the review outcome.

This is not a flaw in the `CHECK`/`FK` design — it is a missing attestation primitive on the path that satisfies the `CHECK`.

### The availability consequence of centralized verification

If warrant validity is centrally verified (Decision 1), then by definition `no valid warrant → no consequential action` and `Solvent unreachable → no consequential authorization`. The elegant `Warrant as portable JSON` diagram hides a real production choice: fail open (an outage silently disables the authorization layer, worse than not having one) or fail closed (a Solvent outage takes down every downstream HIGH_IMPACT action across every customer — a genuine availability and contractual commitment for a solo-founder project). This is not a later problem; it is the direct consequence of the warrant model and belongs in the same paragraph (`review2_claude.md:7-13`).

A second confusion remains: `READ`/`DRAFT` and bounded low-risk work should not have identical availability requirements to `HIGH_IMPACT_EXECUTE`. Treating all operations as fail-closed is operationally absurd; treating all as fail-open defeats the product thesis.

### The future self-referential threat model

Solvent today has no production-scale external Evidence Gateway — the evidence surface is fixtures and local ingestion (`internal/pipeline/`, `internal/derive/testdata/`). Once warrants are externally relied upon, that ingestion path *becomes* authorization-critical. Evidence → Belief → Warrant → Action means forged evidence, tenant cross-talk, replayed attestations, compromised connectors, or poisoned provenance can forge authority transitively. The adversarial review correctly flags this as a later-stage but *non-optional* hardening surface (`review_claude.md:87-89`). Documenting the deferral explicitly is the honest posture for a pre-revenue system; pretending the surface does not exist is not.

---

## Decision

All eight decisions are **Accepted** for post-hackathon evolution and **not implemented** in the current hackathon build. Where a code signature or schema change is implied, it is future-only — illustrative JSON and semantic contracts below, no migration, no handler change.

### Decision 1 — Central warrant verification

The future Warrant model uses **Solvent as the authoritative verification point**. Do not design a self-contained bearer-token authority model as the primary model.

**Why central:**

- **Revocation and reassessment remain centrally controllable.** A self-contained token that cannot be revoked before `expires_at` is a liability precisely in the `reassessment` scenario the product exists for — new evidence contradicts the belief mid-flight, the warrant must die now, not at expiry. Central verification makes revocation a ledger write, not a key-distribution problem.
- **Solvent remains an authorization service rather than becoming a PKI platform.** Self-contained tokens force Solvent to own key rotation, distribution, revocation lists, clock-skew handling, and offline verification across every integrated system — a different company than the transactional belief ledger.
- **A Warrant is a portable protocol representation, but its authoritative validity is checked by Solvent.** Portability means `any executor can carry, transmit, cache, and present a Warrant reference` — verification means `the executor (or Solvent itself on the executor's behalf) checks that reference against the current ledger state before acting`. Prose in a warrant does not confer authority; the ledger does.

**Explicit unresolved consequence:**

> **Every consequential authorization depends on Solvent reachability.**

This is a deliberate, stated availability cost. Solvent becomes part of the availability path. See `Failure / Availability Semantics`.

A signed, self-contained credential may be added later for offline or cross-domain verification, but as a *secondary* mode with explicit revocation semantics, never as the primary authority model.

### Decision 2 — Fail closed for consequential actions

Semantic rule:

```text
Solvent unavailable
  → consequential authorization = DENY / REVIEW
```

An executor that cannot verify a warrant must not treat the absence of a denial as an allowance. `No answer` is not `ALLOW`.

Do **not** claim every operation must fail closed. Distinguish by consequence:

| Class | Example | Solvent-unreachable behavior |
|---|---|---|
| `READ / DRAFT` | search corpus, retrieve issues, draft analysis, generate report | **May continue** per policy — no consequential external effect |
| `LOW_RISK bounded action` | open ticket, restart named service inside fixed condition set | **Policy-dependent** — may continue with bounded blast radius if explicitly configured |
| `HIGH_IMPACT action` | `deploy`, `update_dns`, `issue_refund`, `place_order`, `alter privileges`, `reroute production traffic` | **Solvent reachable + valid warrant required** — `DENY` or `REVIEW` when unreachable |

The current hackathon build does not implement this rule — it has no risk-tier column and no offline handling beyond CockroachDB's own transactional errors (`23503`, `23514`, `40001`). The rule is the post-hackathon contract for any hosted Solvent.

High availability, regional redundancy, local decision caches *only where safe*, and disaster recovery are **future production engineering work, not implemented today**. They mitigate the cost of the central-verification choice; they do not change the semantic rule.

### Decision 3 — Debt retirement requires independent attestation

This is the most important correctness strengthening in this ADR.

**Current weak path (conceptually, not a judgment on current operators):**

```text
human
  → agent interprets the answer (same context window that will benefit from promotion)
  → solvent_retire_debt(scenario, belief_id, debt_item)   // cmd/solvent-mcp/tools.go:102, kernel/kernel.go:83
```

A typo or stale vocabulary is caught at `tools.go:129-132`; a legitimate `debt_item` that is already absent is a silent no-op (kernel contract: idempotent `array_remove`). But there is no binding between the human principal and the retired item.

**Future model:**

```text
human
  → independent attestation (out-of-band channel from the requesting agent)
  → cryptographic/integrity verification (kernel verifies before array_remove)
  → debt discharge (array_remove debt_item from belief.debt)
  → promotion (CHECK promoted_is_debt_free enforces debt='{}')
```

**Semantic requirements for an attestation** (each field load-bearing):

| Field | Meaning | Why |
|---|---|---|
| `attestation_id` | unique id of this attestation | auditability, replay detection |
| `belief_id` | exact belief this attestation applies to | prevents cross-belief reuse |
| `debt_item` | exact obligation being discharged | prevents `needProvenanceCheck` attestation retiring `needOperatorSignoff` |
| `attested_by` | principal (human, verified out-of-band) | attributable authority, not agent identity |
| `issued_at` | timestamp of attestation | freshness, ordering |
| `expires_at` | optional, when this attestation lapses | temporal scoping where appropriate |
| `channel` | independent channel from requesting agent's session | prevents the requesting agent manufacturing its own approval |
| `integrity proof / signature` | Ed25519 or equivalent over `(belief_id, debt_item, attested_by, issued_at)` | integrity, non-repudiation, forgery resistance |

The attestation must be:

- **Bound to the exact belief** (`belief_id` is part of the signed payload, not just a wrapper field).
- **Bound to the exact debt item** — one citation per retrieval debt (`AGENTS.md:74`), one attestation per debt item. Two obligations may not be discharged by the same evidence or the same attestation.
- **Attributable to a principal** — a named human or verified system principal, not `agent:claims-review`.
- **Integrity protected** — signature or equivalent covering the binding above, verified by the kernel before `RetireDebt` mutates the row.
- **Produced through an independent channel from the requesting agent** — the channel field must not be the same MCP session that requested the review. The requesting agent's client may render the elicitation, but the response must be independently verified.

Explicit rule:

> **The agent may request the review. The agent may not manufacture or assert the resulting attestation.**

Discussion of mechanism: **Ed25519 is the initial implementation candidate** because DocTrust already demonstrates that pattern (separate human-consent channel, Ed25519-signed before the audit is sealed — `review_claude.md:83-85`). This ADR does **not** make Ed25519 a permanent protocol requirement. The semantic contract is `attestation = principal + exact object + exact obligation + timestamp + integrity proof + independent channel`; the signature algorithm is an implementation detail of the first concrete build. See `Attestation Model` for the illustrative object and `Warrant Model` for the illustrative warrant.

### Attestation key-management boundary

Decision 1 rejects self-contained bearer-token PKI for Warrant verification because that would make Solvent responsible for executor-side key distribution, rotation, revocation, offline verification, clock-skew handling, and related PKI operations across every integrated system.

Decision 3 necessarily introduces a smaller trust-management problem because Solvent must verify an attestation `integrity_proof` against a known principal key before discharging debt.

This is the same CLASS of problem, but substantially smaller in scope and blast radius: human attestors and debt-discharge events rather than every downstream executor and every action warrant.

The existence of this smaller PKI problem must be explicit. Central Warrant verification bounds key management; it does not eliminate it.

> Decision 3 does not eliminate PKI/key-management concerns; it bounds them to the attestation boundary rather than distributing warrant keys to every executor.

Required design questions before production attestation:

- principal enrollment
- key binding
- key storage/protection
- key rotation
- key revocation
- principal departure
- suspected compromise
- auditability
- replay protection

No final implementation is specified for these items here.

MCP Elicitation provides a structured human interaction/channel. It does not by itself create a cryptographic signature. Standard MCP elicitation responses are structured text returned over the session; they are not signed assertions and must not be treated as integrity proofs. Therefore the future architecture is conceptually:

```text
Human → Client-rendered attestation interaction → Attestation/signing mechanism → Solvent verification → Debt discharge
```

The exact signing UX is unresolved.

Future design must decide whether the attestation mechanism uses:

- application-managed signing keys;
- enterprise identity-backed signing;
- WebAuthn/passkey-backed user presence/approval;
- another equivalent integrity mechanism.

No choice is made here. This ADR does not claim standard MCP clients currently sign elicitation responses.

This is a **future design, not current implementation.** Today `RetireDebt` (`kernel/kernel.go:83`) accepts `(beliefID, item)` and `solvent_retire_debt` accepts `(scenario, belief_id, debt_item)` with no attestation — the future hardens this path to require a verified attestation before the `array_remove` occurs. No signature verification, no attestation table, and no handler change are implied to exist now.

### Decision 4 — Separate semantic truth from protocol representation

Clarify the product vocabulary:

```text
Warrant is NOT a new replacement for action_intent today.
```

- **CURRENT:** `action_intent` is the existing structural authority mechanism. A `live` intent citing `(belief_id, belief_status='promoted')` via `gate` (`db/001_schema.sql:74`), guarded by `live_requires_promoted` (`db/001_schema.sql:69`), **is** authority. No separate `authority` or `warrant` table exists (`kernel/contract.go:1`, `kernel/kernel.go:1`, `db/001_schema.sql:1`). Authority is a *relationship* between a live intent and a currently promoted belief, enforced by schema, not a separate row.

- **FUTURE:** A Warrant is the **interoperable representation** of that authority for MCP / A2A / REST / gRPC / SDK consumers that do not share CockroachDB. It is a JSON protocol object (see `Warrant Model`) carrying `warrant_id, principal, action, resource, belief, status, issued_at, expires_at, decision` that can be carried across systems, presented to an executor, and centrally verified against the ledger.

Do not claim Warrant exists in the current codebase. The migration path is representation-first: today's `IntentOnPromoted` + `gate` becomes tomorrow's `Authorize → Warrant` flow where the database still decides.

### Decision 5 — Risk-tiered availability semantics

Future Solvent should classify actions by consequence/risk, for example:

```text
READ
DRAFT
LOW_RISK_EXECUTE
HIGH_IMPACT_EXECUTE
```

The exact taxonomy remains future design — `strategy.md:21` / `docs/POST_HACKATHON_ARCHITECTURE.md:2.7` `risk_tier` is illustrative, not accepted as `ALTER TABLE action_intent ADD COLUMN risk_tier` in this ADR. The taxonomy will be policy-driven (potentially per-tenant `debt_config`-style configuration) and domain-agnostic (a hospital's `second_opinion` vs. a law firm's `conflict_check` both map to typed debt, not to a new invariant).

Principle:

> **The more consequential the action, the stronger the requirement for a live Solvent decision.**

Do not implement this now. No `risk_tier` column, no policy engine, and no shadow-mode handling are implied to exist in the hackathon build.

### Decision 6 — Solvent's own evidence gateway becomes part of its TCB

Explicitly record the unresolved Finding 5 (`review_claude.md:87-89`):

> Once external systems rely on Solvent warrants, Solvent's own evidence ingestion path becomes authorization-critical. The GhostJacking lesson — *the tool trusted most becomes the way in* — applies with more force to Solvent than to any vendor it defends, because compromising the ingestion path forges the authorization record every downstream executor trusts.

Therefore **future production Solvent must adversarially review**, before any external reliance:

- evidence ingestion (public feeds, webhooks, connector credentials)
- tenant isolation (no cross-tenant warrant confusion — `review_claude.md:70-74` — RFC 8707 Resource Indicators if applicable, or scenario/tenant scoping equivalent)
- provenance (where did this evidence come from, what was observed, what did it affect)
- replay (old attestations re-presented, stale evidence re-ingested)
- poisoning (attacker-controlled content that triggers or blocks promotion)
- forged attestations (manufactured `attested_by` or signatures)
- compromised connectors (a connector's own breach becomes Solvent's breach)
- connector credentials (rotation, scoping, blast radius)
- blast radius (one compromised feed does not forge authority for unrelated beliefs)

For the hackathon: **OUT OF SCOPE.** Reason: the current demo does not yet expose the production-scale external Evidence Gateway. The surface is fixtures and a local pipeline (`internal/derive/testdata/`, `internal/pipeline/`); there is no hosted ingestion endpoint to harden. This deferral is deliberate and documented, not silent omission. Absence of a threat model for this surface before production would be a blocking finding.

See `Deferred Work`. This decision does not create a new table or ingestion service.

### Decision 7 — Preserve the solo-founder scope discipline

Record the deliberately reduced near-term product scope:

```text
Connect
Ask
Authorize
Reassess
```

Over **one MCP interface** (`cmd/solvent-mcp/main.go:87` — stdio, six tools), **one reference agent** (the existing claim/ingestor + security-agent split, or a single Claude/Cursor client for the hosted surface), and **one generic evidence path** (the current `EvidenceFeed` seam, replaceable without kernel changes — `AGENTS.md:56`).

Everything else is **later unless customer evidence justifies expansion**:

- A2A extension
- Connector SDK
- multi-tenancy (`Organization → Workspace → Beliefs` — `strategy.md:19` — requires a deliberate, reviewed schema change, not a day-one checkbox, given `AGENTS.md:52-55` freeze)
- OEM / Solvent Core
- Solvent Edge (self-hosted/private)
- standards work (IETF / A2A extension proposal)
- many deep vendor integrations (Cloudflare / Datadog / Sentry deep dives beyond one or two lighthouse proofs)
- sophisticated risk taxonomy (beyond the four-tier sketch in Decision 5)

This is a **sequencing decision, not a rejection** of those future capabilities. The roadmap cut is what `review_claude.md:66` (`The fix is subtraction, not more strategy`) and `review2_chatgpt.md:46` correctly identified as the highest-leverage change for a solo founder: ship `Connect → Ask → Authorize → Reassess` exceptionally well before adding surface area. Quantity of integrations is not the moat; the invariant is.

### Decision 8 — DocTrust relationship

State accurately:

> **DocTrust and Solvent currently have convergent architecture. DocTrust is NOT currently a runtime dependency of Solvent. Solvent is NOT currently a runtime dependency of DocTrust. Future integration is planned.**

DocTrust is a Go-based compliance-document system with its own `EvidenceProvider` / Check / Scenario / Ruleset architecture and its own human-review, Ed25519-signed consent channel — built independently of Solvent's kernel, not calling into it. The structural resemblance (`evidence → belief-like state → gate → human authority → audit`) is real and worth pointing to, but `DocTrust builds upon Solvent` currently means convergent design, not shared code or shared runtime. If an investor or judge asks where DocTrust calls `kernel.RetireDebt` or `solvent_authorize_action`, the honest answer today is: nowhere.

Do not use `builds upon` or imply shared code unless the repository proves otherwise (it does not). The correct pitch language, consistent with `review_claude.md:119-122` and `review2_chatgpt.md:58-60`, is `parallel, convergent architecture, with integration on the roadmap`. A credible post-hackathon integration — DocTrust's Ruleset engine issuing findings as Solvent evidence and getting its promotion decision from Solvent's gate — is a good idea but explicitly **not** under either hackathon's deadline pressure.

---

## Current vs Future

| Capability | Current Solvent | Post-Hackathon Decision (Accepted, not implemented) | Future / Research |
|---|---|---|---|
| **Evidence** | `evidence` table (`db/001_schema.sql:47`) with `provenance_class IN (external_feed, reproducible_artifact, live_scan, operator_asserted)`, fixtures in `internal/derive/testdata/` | Evidence ingestion path becomes part of TCB; attributable, provenance-preserving, replay/poisoning-reviewed before production (Decision 6) | Evidence Gateway: pluggable `EvidenceFeed` (`AGENTS.md:56`) productized as SaaS gateway with connector credentials, tenant isolation, content-hash and provenance envelope (`docs/POST_HACKATHON_ARCHITECTURE.md:2.5`) |
| **Belief** | `belief` (`id, claim, claim_type, status, debt TEXT[], final_truth`) + `belief_edge` graph, `WITH RECURSIVE` CTE traversal in Go (`kernel/sql.go:66`) | Belief lifecycle unchanged; attestation-bound debt retirement hardens the transition to `debt='{}'` (Decision 3) | Semantic belief identity: fingerprinting / embedding-based contradiction matching (`docs/POST_HACKATHON_ARCHITECTURE.md:3.1`) |
| **Debt** | Six items `needProvenanceCheck, needContradictionSweep, needBlastRadius, needRollbackPlan, needVersionPin, needOperatorSignoff` — `TEXT[]` `FullDebt` (`kernel/kernel.go:31`, `db/001_schema.sql:25`, `db/004_debt_vocabulary.sql:47`); `promoted_is_debt_free` checks `array_length(debt,1)=0` only | Debt retirement requires independent, integrity-protected attestation bound to exact `belief_id + debt_item` (Decision 3); agent may request, may not manufacture | Typed/configurable debt: per-tenant `debt_config` (`debt_type IN (deterministic, attested, quorum)`, `min_sources`) as policy, `debt TEXT[]` remains sole CHECK truth (`docs/POST_HACKATHON_ARCHITECTURE.md:2.6`); `belief.validity_horizon` |
| **Promotion** | `kernel.Promote` (`kernel/kernel.go:95`) → `UPDATE belief SET status='promoted'`; `ErrPromotionBlocked` on `23514` `promoted_is_debt_free` | Promotion gated by attested discharge; Ed25519 is initial implementation candidate, not protocol requirement (Decision 3) | No semantic change; promotion remains `CHECK`-enforced |
| **Action Intent** | `action_intent` + `gate` FK + `live_requires_promoted` (`db/001_schema.sql:60-76`); `kernel.IntentOnPromoted` (`kernel/kernel.go:107`, `23503`) | Risk-tiered classification of actions by consequence (Decision 5) shapes availability and policy, not the FK itself | Temporal authority: `action_intent.expires_at` copy of `belief.validity_horizon`, `intent_not_stale CHECK`, lazy expiration `EnsureFreshness` + periodic worker + authorization-time check (`docs/POST_HACKATHON_ARCHITECTURE.md:2.4`); `principal`/`risk_tier` columns (`2.7`) |
| **Warrant** | **No Warrant object.** Structural authority is `live` intent + `promoted` belief + `gate` (`AGENTS.md:42`, `README.md:28`) | **Warrant = portable interoperable authority representation** of that relationship (Decision 4); verification is central by Solvent; availability cost stated (Decision 1) | Warrant protocol stabilized; optional signed secondary mode with revocation semantics for offline/cross-domain verification (Decision 1) |
| **Attestation** | **No Attestation object.** `solvent_retire_debt(scenario, belief_id, debt_item)` + `kernel.RetireDebt` idempotent `array_remove` (`cmd/solvent-mcp/tools.go:102`, `kernel/kernel.go:83`) | **Attestation = independent, attributable, integrity-protected discharge** with 8 fields, bound to exact belief+item, independent channel (Decision 3) | Production attestation verification in kernel; replay protection; channel binding to MCP elicitation or out-of-band flow |
| **Reassessment** | Explicit invalidation: `RetractCascade` (`kernel/kernel.go:127`) cancels intents then retracts in one transaction; contradiction detection exists in `derive`/`belief.Process` but is discarded (`belief/belief.go:38-48` logs and returns nil) | Central reassessment: new evidence or contradiction can revoke/reassess warrants centrally (Decision 1 + 6) | Continuous reassessment: `evidence_contradiction` relation + `FileContradiction` adding `needContradictionResolution` debt (`docs/POST_HACKATHON_ARCHITECTURE.md:2.1-2.3`), temporal staleness as contradiction-like trigger (`2.4`), richer propagation |
| **Identity** | No identity column; single-agent demo; scenario-scoped (`track1`/`track2` fixed UUIDs, `cmd/solvent-mcp/main.go:31`) | Solvent's ingestion gateway must adversarially review tenant isolation, provenance, connector credentials, blast radius before production (Decision 6) | Pluggable identity (OIDC/OAuth/SAML/workload identities) consumed, not owned by Solvent; `principal UUID` on `action_intent` as defined in `docs/POST_HACKATHON_ARCHITECTURE.md:2.7` |
| **Risk tiers** | None | Risk-tiered availability semantics: `READ, DRAFT, LOW_RISK_EXECUTE, HIGH_IMPACT_EXECUTE` — exact taxonomy deferred (Decision 5) | Policy-driven risk tiers per tenant; `risk_tier` advisory column is one possible implementation, not the decision itself |
| **MCP** | Local stdio `solvent-mcp` (`cmd/solvent-mcp/main.go:87`), six tools, thin adapter, no hosting | Single MCP interface retained for `Connect → Ask → Authorize → Reassess`; MCP Elicitation as the human-approval UX where the client's native surface renders the prompt (Decision 7 + `claude.md:14`) | Remote/hosted MCP SaaS; auth hardening (`RFC 8707` Resource Indicators) for tenant isolation (`claude.md:16`); interceptor mode |
| **A2A** | **Not implemented** | Deferred — sequencing decision, not rejection (Decision 7) | Warrant-aware A2A extension (`solvent.warrant` reference in Agent Card handoff), independent verification by receiving agent, standards-track proposal |
| **External evidence gateway** | Out of scope; fixtures only | **Out of scope** for hackathon, explicitly recorded (Decision 6) | Production gateway with adversarial review of ingestion, provenance, replay, poisoning, forged attestations, compromised connectors, credentials, blast radius |

---

## Security Properties

The future design is intended to guarantee (each is a decision, not a current claim):

1.  **Agent output is not authority.** Reading, drafting, correlating, and proposing are allowed; a consequential action requires a `live` intent on a currently `promoted` belief. Today via `gate` (`23503`); in the future via a valid, centrally verified Warrant (`README.md:122-132`).

2.  **Debt discharge is intended to become independently attributable and integrity-protected; the exact principal/key lifecycle remains a production design requirement.** In the future, an obligation is discharged only by a verified attestation bound to `belief_id + debt_item + attested_by + issued_at + channel + integrity proof` (Decision 3). The request channel and the attestation channel are independent — a compromised or persuaded agent session cannot self-certify.

3.  **Consequential actions fail closed if authority cannot be verified.** `Solvent unreachable → DENY/REVIEW` for `HIGH_IMPACT` actions (Decision 2). `READ`/`DRAFT` may continue per policy. This is the stated cost of Decision 1.

4.  **Warrants can be revoked/reassessed centrally.** A belief retraction (`RetractCascade`) or new contradicting evidence invalidates dependent warrants immediately and centrally — no key-revocation list propagation delay, no token lifetime to expire. Portable representation, central decision.

5.  **Contradictions and new evidence can affect downstream authority.** A recorded contradiction that resolves to a retraction cascades through `belief_edge` descendants and cancels dependent `live` intents in one transaction (`kernel/kernel.go:127`, `kernel/sql.go:81-92`). Future: durable `evidence_contradiction` + resolution debt (`docs/POST_HACKATHON_ARCHITECTURE.md:2.1-2.3`).

6.  **Solvent itself becomes subject to the same authority discipline it imposes.** Once warrants are externally relied upon, Solvent's own ingestion path is authorization-critical (Decision 6) — provenance is preserved (`evidence.content_sha256, provenance_class, source_url, ingested_at`), and forged attestation, replay, and poisoning are treated as authorization bypasses, not data-quality issues.

---

## Failure / Availability Semantics

### The centralized-verification tradeoff

Decision 1 chooses a hard truth over a comfortable diagram: a system that says `Solvent is the authority boundary` cannot, without contradicting itself, authorize a consequential action when the authority boundary is unavailable and unverifiable.

Therefore this ADR is honest about the price:

> **Every consequential authorization depends on Solvent reachability.**

An executor that cannot reach Solvent and cannot independently verify a warrant is in the `no valid warrant` state. The product does not hide that Solvent becomes part of the availability path. Customers should not discover this in an outage postmortem.

### Fail-closed behavior

```text
Solvent unavailable
  → HIGH_IMPACT action = DENY or REVIEW
  → LOW_RISK / bounded action = policy-dependent (may be ALLOW per explicit config)
  → READ / DRAFT = may continue per policy
```

`HIGH_IMPACT` (`deploy`, `update_dns`, `issue_refund`, `place_order`, `alter privileges`, `reroute production traffic`) **requires** `Solvent reachable + warrant valid (promoted belief + valid attestation + no unresolved contradiction + policy pass)` at authorization time. If any conjunct is unknown because Solvent is unreachable, the decision is the safe one — `DENY` or `REVIEW` — not `ALLOW with a note`.

`LOW_RISK` and `READ`/`DRAFT` are deliberately not forced closed — an agent that cannot search or draft during a transient outage is operationally useless, and those operations have no consequential external effect to gate. The risk taxonomy (Decision 5) exists precisely to avoid the false choice between `everything open` and `everything down`.

### What this does not promise today

High availability, regional redundancy, local decision caches only where safe, and disaster recovery are **future production engineering work, not implemented today** (Decision 2). The deployed demo runs on CockroachDB Cloud Serverless `v26.2.5, multi-region, primary region aws-us-west-2` with a single App Runner service (`README.md:46-52`), not a fleet with failover. A `cache` that serves `ALLOW` from stale state without verification would be a fail-open bug with better branding; any cache must be bounded, verifiable, and explicitly scoped to safe operation classes.

`40001 RETRY_SERIALIZABLE` remains a retry signal, not a refusal — under `crdb.ExecuteTx` the transaction retries and then refuses on fresh state (`AGENTS.md:103`, `README.md:159-161`, `kernel/kernel.go:59`). Availability failures must not be conflated with serialization retries.

---

## Attestation Model

**Illustrative future protocol representation — not a current schema object.** No `attestation` table exists today. No attestation signature is verified by the current kernel. This object is the proposed semantic contract for the post-hackathon hardening of `RetireDebt`.

```json
{
  "attestation_id": "att_01h8x7q2e9a3b4c5d6e7f8g9h0",
  "belief_id": "b_8f21c4a0-1234-4abc-9def-0123456789ab",
  "debt_item": "needOperatorSignoff",
  "attested_by": "human:alice@example.com",
  "issued_at": "2026-08-31T04:00:00Z",
  "expires_at": "2026-09-01T04:00:00Z",
  "channel": "mcp_elicitation:claude_desktop:session_9f3a",
  "integrity_proof": {
    "alg": "Ed25519",
    "kid": "alice_2026_08",
    "value": "base64url(ed25519_signature_over_canonical_payload)"
  }
}
```

Canonical payload for `integrity_proof.value` (initial candidate; algorithm is not a permanent protocol requirement — Decision 3):

```text
attestation_id || belief_id || debt_item || attested_by || issued_at || channel
```

Semantic contract, restated for implementers:

- **One attestation per debt item.** A single attestation does not discharge multiple items; two independent obligations require two independent attestations (cf. `AGENTS.md:74` `One citation per retrieval debt` — same principle for attestations).
- **Binding is in the signed payload.** `belief_id` and `debt_item` are inside the integrity proof, not just adjacent JSON fields.
- **Principal is attributable.** `attested_by` is a principal verified through an independent channel, not the requesting agent's identity.
- **Channel is independent.** `channel` must not be the same MCP session that requested the review. The requesting agent may trigger the review request; it may not produce or relay the attestation as its own assertion.
- **`expires_at` is optional.** When absent, the attestation is valid until explicitly revoked or superseded; when present, it scopes temporal authority (see Deferred Work: temporal/freshness authority).
- **Verification precedes discharge.** The kernel verifies `integrity_proof` against a known key for `attested_by` *before* executing `array_remove(debt, debt_item)`. Verification failure leaves `debt` unchanged and promotion still blocked at `23514`.

The Ed25519 choice is the **initial implementation candidate** because DocTrust already demonstrates this pattern; the ADR-level guarantee is `integrity protected + attributable + independent channel`, not `necessarily Ed25519 forever`.

---

## Warrant Model

**Illustrative future protocol representation — not a current schema object.** No `warrant` table exists today. The current authority object is `action_intent` + `belief(id,status)` via `gate` (`db/001_schema.sql:74`). This object is the proposed portable, verifiable representation of that relationship for MCP/A2A/REST/gRPC/SDK consumers.

```json
{
  "warrant_id": "w_91f2c4a0-1234-4abc-9def-0123456789ab",
  "principal": "agent:claims-review",
  "action": "deploy",
  "resource": "prod/payment-api",
  "belief": {
    "belief_id": "b_8f21c4a0-1234-4abc-9def-0123456789ab",
    "claim": "etcd v3.5.14 is safe to deploy",
    "status": "promoted"
  },
  "status": "authorized",
  "issued_at": "2026-08-31T04:00:00Z",
  "expires_at": "2026-08-31T18:00:00Z",
  "decision": "ALLOW",
  "reason": null,
  "policy": "claims.v3",
  "evidence_summary": {
    "evidence_count": 8,
    "attestations": 6,
    "citations": [
      { "corpus_issue": "#19220", "distance": 0.372424, "relation": "considered" },
      { "corpus_issue": "#14139", "distance": 0.199509, "relation": "considered" }
    ]
  }
}
```

`DENY` / `REVIEW` variants carry machine-readable reasons (`review2_chatgpt.md:7-13`):

```json
{ "decision": "DENY", "reason": "belief_not_promoted", "constraint": "gate", "sqlstate": "23503" }
{ "decision": "DENY", "reason": "debt_open", "constraint": "promoted_is_debt_free", "sqlstate": "23514" }
{ "decision": "DENY", "reason": "solvent_unreachable", "constraint": null, "detail": "no valid warrant could be verified" }
{ "decision": "REVIEW", "reason": "unresolved_contradiction", "detail": "needContradictionResolution outstanding" }
```

Contract notes:

- **Warrant is not authority; it is evidence of authority.** Possessing the JSON does not grant permission. An executor must verify it (`Solvent reachable → verify belief still promoted + debt still empty + no intervening retraction + no expiry + policy pass`) before acting. This preserves revocation semantics: a retracted belief kills every warrant that cited it, centrally and immediately.
- **Central verification is the primary model** (Decision 1). A signed secondary mode may be added for offline/cross-domain scenarios with explicit revocation — not as the primary.
- **SQLSTATEs are evidence** (`AGENTS.md:97-98`): `23503 · gate`, `23514 · promoted_is_debt_free`, `23514 · live_requires_promoted` remain the engine's own words, surfaced verbatim, never replaced by generic UI prose. An application-raised `DENY` for `solvent_unreachable` carries **no** constraint name, deliberately — claiming one would be inventing engine output.
- **Evidence distances are measured** (`AGENTS.md:65-70`): `0.372424`, `0.199509`, `0.594920` are preserved exactly where shown, not re-rounded or substituted.
- No `warrant` table, no new FK, no `risk_tier` column, and no signing key set are implied to exist now.

---

## Trust Boundaries

Left-to-right diagrams, not down-arrow chains.

### Authority boundary

```text
Agent → MCP/A2A/API → Solvent (ledger + policy) → Identity/Policy (OIDC/OAuth/OPA/Cedar/Cerbos/external) → Executor (SaaS/Cloud/Workflow/EHR/DB)
```

- **Agent → MCP/A2A/API:** Agent proposes. Any agent (Claude, GPT, Gemini, Cursor, custom) via any northbound interface that speaks the warrant protocol.
- **MCP/A2A/API → Solvent:** Solvent evaluates. Evidence provenance, belief status, debt, attestations, and revocation are checked. MCP is the northbound interface today; A2A/REST/gRPC are future northbound interfaces speaking the same warrant semantics.
- **Solvent → Identity/Policy:** Solvent does not own identity. `Is this principal allowed to attempt this?` is answered by external identity/policy; `Is this particular action justified by a valid warrant right now?` is answered by Solvent. `OAuth/IAM + Solvent = Authorization for agentic action` (`product.md:261-268`).
- **Identity/Policy → Executor:** Executor acts only on `ALLOW` with a currently verified warrant. Legacy systems become `Action Interceptors` (`strategy.md:14`) without rewriting the agent.

`READ`/`DRAFT` may traverse this boundary without a warrant per policy (Decision 2); `HIGH_IMPACT_EXECUTE` must not.

### Attestation boundary

```text
Human → Client-rendered attestation interaction → Attestation/signing mechanism → Solvent (verifies integrity proof) → Debt Discharge (array_remove) → Promotion (CHECK) → Warrant (verifiable)
```

MCP Elicitation (`claude.md:14`) provides a structured human interaction/channel — the client's native surface renders the prompt — but does not by itself create a cryptographic signature. The attestation/signing mechanism is a separate step from the interaction; the exact signing UX is unresolved. The channel carrying the human response is independent from the requesting agent's session, and that independence is what prevents the agent from paraphrasing a plain-language answer into authority. Standard MCP elicitation responses are structured text over the session, not signed assertions.

- **Human → Client-rendered attestation interaction:** Human responds through the client's native UI, independent from the requesting agent's session, rather than through the agent's context window.
- **Attestation/signing mechanism → Solvent:** Solvent verifies `integrity_proof` against a known principal key *before* mutating `belief.debt`. Verification failure → no discharge → `promoted_is_debt_free` still blocks.
- **Solvent → Debt Discharge → Promotion → Warrant:** Attestation is the *only* strong path to empty `debt`; promotion remains the single `CHECK`-gated step to `promoted`; warrants are issued only after promotion.

Both diagrams share one rule: **the agent is never the authority.** The agent proposes, cites, and presents; the database or the attestation channel decides.

---

## What remains unchanged

Explicitly:

- **Four frozen ledger tables remain the current foundation** — `belief, belief_edge, evidence, action_intent` (`db/001_schema.sql:1-76`, `AGENTS.md:59`). No new tables.
- **Existing SQL invariants remain** — `promoted_is_debt_free` (23514), `gate` (23503), `live_requires_promoted` (23514), `ON UPDATE CASCADE` re-evaluation behavior as measured, `I-5 AuditLiveOnNonPromoted=0`, `I-6 no embedding column on ledger`, `I-7 crdb.ExecuteTx on all writes`, `I-8 cancel-before-retract` — additive changes only in `POST_HACKATHON_ARCHITECTURE.md:4` (`005_*`, `006_*`, etc.), none in this ADR's current tier.
- **Current Lean model remains** — 11 theorems in `formal/lean/` (`live_intent_implies_promoted`, `no_live_intent_on_retracted_belief`, `cascade_retraction_cannot_leave_live_intent`, etc.), zero `sorry` (`README.md:169-196`).
- **Current MCP semantics remain** — six tools `solvent_ledger, solvent_ingest_evidence, solvent_retire_debt, solvent_promote, solvent_authorize_action, solvent_falsify` with their current signatures (`cmd/solvent-mcp/main.go:88-223`, `tools.go:1-327`), thin-adapter `unmarshal → kernel call → format`, scenario-scoped `track1`/`track2`.
- **No attestation key-management infrastructure exists in the current implementation.**
- **No production redesign is implied by this ADR.** This is a decision record. The deployed demo at `AWS App Runner / CockroachDB Cloud Serverless v26.2.5` (`README.md:46-54`), vector ANN on `corpus_issue.embedding VECTOR(1024)`, and the three demo beats (`ASK → DISCHARGE → FALSIFY` with refusal trail `23503 → 23514 → 23514`) are unchanged.

---

## Deferred Work

The following are **intentionally deferred** — Sequencing, not rejection — until customer evidence justifies expansion or until the current `Connect → Ask → Authorize → Reassess` surface is proven with one lighthouse integration:

- **Production evidence gateway security review** — full adversarial review of ingestion, tenant isolation, provenance, replay, poisoning, forged attestations, compromised connectors, connector credentials, blast radius (Decision 6). Required before any external system relies on warrants.
- **Multi-tenancy** — `Organization → Workspace → Agents/Beliefs/Evidence/Policies/Authorities` with tenant-scoped ledger. Directly conflicts with the frozen-kernel rule (`AGENTS.md:52-57`); requires a deliberate, reviewed schema change (at minimum a tenant column threaded through `belief, evidence, action_intent` and a rework of what `scenario` means — `review_claude.md:70-74`).
- **A2A integration** — warrant-aware A2A extension (`solvent.warrant` reference in Agent Card handoff), independent verification by receiving agent, standards-track proposal parallel to Open Agent Passport (`claude.md:18`).
- **Remote MCP** — hosted MCP server as managed service (the `Cloud Managed MCP Server` in `README.md:267-269` is currently configured but unverified at `401`).
- **Connector ecosystem** — Connector SDK with `Evidence Source → Authority Check → Action Executor → Change Callback` contract, making the 100th integration easier than the first three (`product.md:327-362`).
- **OEM / Solvent Core / Solvent Edge** — embeddable ledger for platform vendors (ServiceNow, Salesforce, agent platforms) and private-deployment variants (`product.md:590-616`).
- **Temporal / freshness authority** — `belief.validity_horizon` + `action_intent.expires_at` + `intent_not_stale CHECK` + lazy expiration + periodic worker + authorization-time check (`docs/POST_HACKATHON_ARCHITECTURE.md:2.4`, `3.2`). Time passing alone triggers no write today — a fundamental DBMS property (`docs/POST_HACKATHON_ARCHITECTURE.md:2.4`).
- **Richer contradiction propagation** — durable `evidence_contradiction` relation, `FileContradiction` debt addition (`docs/POST_HACKATHON_ARCHITECTURE.md:2.1-2.3`), subject-based lookup fragility acknowledged (`POST_HACKATHON_ARCHITECTURE.md:3.1`).
- **Typed debt** — `debt_config` static configuration (`debt_type IN (deterministic, attested, quorum)`, `min_sources`) as policy, `debt TEXT[]` remains sole CHECK truth (`docs/POST_HACKATHON_ARCHITECTURE.md:2.6`); per-domain vocabularies (`needSecondOpinion` vs `needConflictCheck`).

### Attestation key lifecycle

This is required before production use of signed human attestations. It is intentionally deferred because the current hackathon build has no attestation object or attestation key infrastructure.

- Who enrolls an attestor?
- How is a key bound to the principal?
- Where is the private key held?
- How is rotation handled?
- How is revocation propagated?
- What happens when the principal leaves?
- What happens after suspected compromise?
- How is a revoked attestation prevented from being replayed?

Also deferred: evidence-quality advisory column, agent identity `principal UUID`, richer provenance, standards work beyond a working protocol, shadow-mode `observe → recommend → enforce` rollout tooling, and any pricing/licensing model.

---

## Rationale

The principle that survives every review in `review_claude.md:1` is narrower and more powerful than any feature list:

> **Agents may reason.**
> **Agents may propose.**
> **Authority must be earned.**
> **Authority must be independently verifiable.**
> **Authority must be revocable when its justification changes.**
> **Consequential authority must fail closed when its authority boundary cannot be verified.**

Everything else in this ADR exists to make that primitive true in code rather than in prose:

- `Memory / Retrieval → Belief → Authority → CockroachDB invariant` (`AGENTS.md:7`) — not `retrieval → answer → action in one context window` (`README.md:35-39`, `review_claude.md:60`).
- `Evidence → Belief → (Attested) Debt → Promotion → (Warrant →) Action → Reassessment` — the future kernel keeps `evidence → belief → debt → promotion → action_intent` intact and adds attestation and warrant as *representations* around the same invariants.
- `The database decides` (`README.md:111-114`) — the agent and the MCP adapter propose; the kernel owns transaction discipline; CockroachDB owns the invariants (`SOLVENT_ENGINEERING_GUIDE.md:6`).

This ADR is the **formal architectural bridge** between the current Solvent kernel and the larger product vision. It deliberately strengthens the single weakest transition — human judgment to debt discharge — to the same standard as the already-strong `gate`, rather than adding surface area. The current four-table ledger and its schema-enforced invariants are not a toy subset of the eventual architecture; they are the mechanism the eventual architecture is built to preserve.

---

*Evidence: this ADR cites `AGENTS.md`, `README.md`, `SOLVENT_ENGINEERING_GUIDE.md`, `db/001_schema.sql`, `db/003_wizard.sql`, `db/004_debt_vocabulary.sql`, `kernel/kernel.go`, `kernel/sql.go`, `kernel/contract.go`, `cmd/solvent-mcp/main.go`, `cmd/solvent-mcp/tools.go`, `docs/POST_HACKATHON_ARCHITECTURE.md`, `formal/lean/`, and the review corpus `review_claude.md` + `review_chatgpt.md` + `review2_chatgpt.md` + `review2_claude.md`. No code was modified. No table was added. No invariant was weakened.*
