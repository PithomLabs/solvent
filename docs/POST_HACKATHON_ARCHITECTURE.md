# Post-Hackathon Architecture

**Status:** Future architecture specification derived from first-order, second-order, and final architecture reviews.
**Scope:** What Solvent should become after the hackathon if its thesis is taken to its logical conclusion.
**Not:** An implementation plan for the hackathon. No code, no migrations, no commits.

---

## Purpose

This document specifies the post-hackathon evolution of Solvent's architecture. It merges only conclusions that survived adversarial validation against the actual schema, code, and CockroachDB v26.2.0 behavior. Every proposal is explicitly tagged with its implementation tier.

The thesis that drives every decision:

> **Retrieval can be wrong.**
> **Judgment can be wrong.**
> **Authority cannot silently outlive what justifies it.**

---

## Tier Classification Key

| Tag | Meaning |
|---|---|
| **CURRENT / VERIFIED** | Exists, tested, shipped. Read from code and schema. |
| **POST-HACKATHON TARGET** | Designed, validated, not implemented. Ready for development after hackathon. |
| **RESEARCH / LONG-TERM** | Conceptual. Not designed. Requires further investigation. |

---

# 1. CURRENT / VERIFIED

## 1.1 Schema

Seven tables in two layers. The four frozen ledger tables are the product; the corpus/wizard tables are the demo infrastructure.

```text
┌─────────────────────────────────────────────────────────────┐
│                    FROZEN LEDGER (001_schema.sql)            │
│                                                             │
│  belief ──────── belief_edge ──────── belief                │
│    │              (derives,                                 │
│    │               contradicts)                             │
│    │                                                        │
│    ├──── evidence                                           │
│    │                                                        │
│    └──── action_intent ──── FK(belief_id, belief_status)    │
│                              REFERENCES belief(id, status)   │
│                              ON UPDATE CASCADE               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                 CORPUS / WIZARD (002, 003, 004)              │
│                                                             │
│  corpus_issue ──── belief_corpus_citation                   │
│                        (relation: considered | contradicts)  │
│                                                             │
│  refusal_log                                                │
└─────────────────────────────────────────────────────────────┘
```

**Frozen tables:** `belief`, `belief_edge`, `evidence`, `action_intent`
**Corpus tables:** `corpus_issue`, `belief_corpus_citation`, `refusal_log`

## 1.2 Invariants

| ID | Invariant | Enforcement | SQLSTATE |
|---|---|---|---|
| I-1 | Promoted belief has empty debt and `final_truth=false` | Schema CHECK (`promoted_is_debt_free`) | 23514 |
| I-2 | Live intent references a promoted belief | Schema CHECK (`live_requires_promoted`) | 23514 |
| I-3 | Intent creation requires promoted belief | Composite FK (`gate`) with ON UPDATE CASCADE | 23503 |
| I-4 | Belief retraction cascades to intent status | ON UPDATE CASCADE + CHECK re-evaluation | 23514 |
| I-5 | `AuditLiveOnNonPromoted` returns 0 | Query (application-level assertion) | — |
| I-6 | No embedding column on ledger tables | Schema (absence) | — |
| I-7 | Every kernel write through `crdb.ExecuteTx` | Application (I-7 guard script) | — |
| I-8 | `RetractCascade` cancel-before-retract ordering | Application (single transaction) | — |

**Key property measured, not assumed:** CockroachDB re-evaluated CHECK constraints on rows modified by ON UPDATE CASCADE. Verified at M0 against v26.2.0 (`docs/M0_REPORT.md:42-47`).

## 1.3 Kernel API

Eight functions, all in `kernel/kernel.go`. Seven write sites through `crdb.ExecuteTx`, one read site.

| Function | Write/Read | I-7 |
|---|---|---|
| `EnterBelief` | Write | ✓ |
| `AddEvidence` | Write | ✓ |
| `RetireDebt` | Write | ✓ |
| `Promote` | Write | ✓ |
| `IntentOnPromoted` | Write | ✓ |
| `RetractCascade` | Write (2 statements) | ✓ |
| `EnsureBelief` | Write | ✓ |
| `AuditLiveOnNonPromoted` | Read | — |

## 1.4 Lean Model

Eleven theorems in `formal/lean/`, zero `sorry`, zero `admit`, zero unchecked `axiom`.

**Transition preservation:**
- `promote_preserves_validity`
- `cancelIntent_preserves_validity`
- `authorizeIntent_preserves_validity`
- `retractCascade_preserves_validity`

**Authority impossibility (the Solvent thesis):**
- `live_intent_implies_promoted`
- `no_live_intent_on_retracted_belief`

**Cascade properties:**
- `promotion_updates_dependent_intent`
- `cascade_retraction_updates_dependent_intent`
- `cascade_retraction_cannot_leave_live_intent`
- `cascade_update_preserves_gate`
- `cascade_retraction_blocks_live_intent`

**Scope:** Lean proves abstract state-machine preservation. CockroachDB proves real engine refusal empirically. This is not a refinement proof.

## 1.5 Contradiction Chain (Current State)

The pipeline detects contradictions but discards them:

```text
normalize          →  no contradiction signal
    │
derive             →  DerivedBelief.Contradicts = [NormalizedEvidence]
    │                  knows: "etcd v3.5.14 is contradicted"
    │                  does NOT know: which belief UUID
    │
belief.Process     →  slog.Warn("no ledger mutation"), return nil
    │                  THE GAP: contradiction discarded
    │
pipeline           →  Result.Contradiction = true, no DB write
    │
kernel             →  RetractCascade exists but is never called
    │
SQL                →  belief_edge.kind supports 'contradicts' but no code files edges
```

**Where meaning is lost:** `belief.Process:39-48`. The system detects the contradiction, has the tool to act on it, but the wiring layer discards it.

## 1.6 What Current Solvent Cannot Do

| Gap | Impact |
|---|---|
| Cannot identify which belief a contradiction targets | Contradictions detected and discarded |
| Cannot record contradictions durably | Belief graph has no contradiction edges |
| Cannot invalidate authority when evidence ages | Live intent can outlive stale evidence |
| Cannot distinguish debt types | All six items discharge identically |
| Cannot attribute intents to agents | Single-agent demo only |

---

# 2. POST-HACKATHON TARGET

## 2.1 Contradiction Target Identification

**Problem:** The derive stage operates on evidence in isolation. It produces `DerivedBelief.Contradicts` containing `NormalizedEvidence` — NOT a belief UUID. The system knows "etcd v3.5.14 is contradicted" but does not know which specific belief row is contradicted.

**Why `belief_edge` cannot help:** Both FKs reference `belief(id)`. An evidence→belief contradiction would violate the FK on `parent_id`.

**Solution:** The pipeline orchestrator (not the derive function) performs a subject-based lookup after derive:

```text
derive stage
    │
    │  DerivedBelief{ Claim: "prior belief about etcd v3.5.14 is contradicted",
    │                  Contradicts: [evidence] }
    │
    ▼
pipeline orchestrator
    │
    │  SELECT id FROM belief
    │  WHERE scenario_id = $1::UUID
    │    AND claim LIKE '%' || 'etcd v3.5.14' || '%'
    │    AND status <> 'retracted'
    │
    │  → belief UUID (or no match)
    │
    ▼
FileContradiction(scenarioID, beliefID, evidenceID)
```

**Why the derive stage must remain pure:** It is a deterministic function of evidence. Adding belief-table access would break its purity guarantee. Belief-target identification is an orchestration concern, not a derivation concern.

**Demo approach:** Subject-based text lookup. Fragile but feasible for the single-domain demo.

**Production approach (RESEARCH):** Semantic belief identity — belief fingerprinting or embedding-based matching.

## 2.2 Evidence→Belief Contradiction Relation

**New table (additive, not modifying 001_schema.sql):**

```sql
CREATE TABLE evidence_contradiction (
    evidence_id UUID NOT NULL REFERENCES evidence(id),
    belief_id   UUID NOT NULL REFERENCES belief(id),
    filed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (evidence_id, belief_id)
);
```

**Why not `belief_edge`:** `belief_edge` FKs reference `belief(id)` only. Using it for evidence→belief would violate the FK constraint. `belief_edge` remains belief-to-belief only.

**Invariant impact:** None on I-1 through I-8. The new table has its own FKs that do not interact with the existing CHECK constraints.

**New invariant (additive):**

```
I-9: Every evidence_contradiction row references a valid evidence and belief.
     (Enforced by FK constraints on the new table.)
```

## 2.3 Contradiction Resolution Protocol

Three phases, matching Solvent's existing philosophy: evidence proposes, the database enforces, the operator decides.

**Phase 1: Record (durable)**
- Pipeline orchestrator identifies target belief (§2.1)
- `FileContradiction` adds `needContradictionResolution` debt to the belief
- `evidence_contradiction` row records the evidence→belief link
- Belief remains in its current status; authority is NOT affected

**Phase 2: Block promotion (structural)**
- The new debt item prevents re-promotion of a retracted belief
- If the belief is already promoted, the debt does NOT immediately retract it
- The contradiction is recorded but authority is not yet affected

**Phase 3: Resolve (operator action)**
- Operator reviews the contradiction and either:
  - Discharges the debt (false alarm → belief stays promoted)
  - Retracts the belief (valid contradiction → cascade cancels intents)

**Why NOT automatic retraction:**
1. A single contradictory comment might not warrant retracting a promoted belief
2. The contradiction target is identified by subject text, not belief ID — false positives are possible
3. Automatic retraction of a belief with live intents would trigger cascade cancellation without human review

**New kernel function:**

```go
func (s *Store) FileContradiction(ctx context.Context, scenarioID, beliefID, evidenceID string) error
```

Single transaction: add debt (array_append, idempotent) + insert evidence_contradiction row.

## 2.4 Temporal Authority

### The Failure Sequence

```text
T+0:00  belief promoted (debt discharged)
T+0:01  action_intent becomes live (FK satisfied)
T+0:02  evidence ages out (validity_horizon expires)
        NO DATABASE WRITE OCCURS
T+0:03  system reads action_intent.state = 'live'
        nothing distinguishes this from T+0:01
```

At T+0:03, the database says `state = 'live'`. No CHECK fires because no row was modified. The intent is structurally live but the evidence justifying it is stale.

### Why `now()` in CHECK Is Not Continuous

```sql
CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now())
```

| Event | CHECK fires? | Result |
|---|---|---|
| INSERT with stale expires_at | YES | Refused ✓ |
| UPDATE to stale expires_at | YES | Refused ✓ |
| Time passes, no write | NO | Intent remains live ✗ |

CockroachDB evaluates `now()` at **transaction time** when the CHECK is evaluated (on INSERT/UPDATE). It does NOT provide continuous evaluation. This is a fundamental DBMS property, not a CockroachDB limitation.

### Four-Layer Defense-in-Depth

No single mechanism provides "live means currently authorized." All four layers are required.

**Layer 1: CHECK constraint (safety net)**
```sql
ALTER TABLE action_intent ADD CONSTRAINT intent_not_stale
    CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now());
```
Catches bad writes. Prevents new stale intents from entering.

**Layer 2: Lazy expiration (write trigger)**
```go
func (s *Store) EnsureFreshness(ctx context.Context, scenarioID string) (int, error) {
    // UPDATE action_intent SET state = 'expired'
    // WHERE state = 'live' AND expires_at IS NOT NULL AND expires_at <= now()
    // Called before every intent-read path.
}
```
The CHECK fires on the UPDATE. Any code path that encounters a stale intent must call this first.

**Layer 3: Periodic expiry worker (belt-and-suspenders)**
```go
func (s *Store) ExpireStaleIntents(ctx context.Context) (int, error) {
    // Same SQL, scoped to all scenarios.
    // Runs on a ticker (e.g., every 60 seconds).
    // Catches intents that slip through lazy expiration.
}
```

**Layer 4: Authorization-time check (defense in depth)**
```go
func (s *Store) IntentOnPromoted(ctx context.Context, scenarioID, beliefID, action string) error {
    // Before creating intent, verify belief is not stale:
    // SELECT validity_horizon FROM belief WHERE id = $1::UUID
    // If validity_horizon IS NOT NULL AND validity_horizon <= now(), refuse.
}
```

### Schema Additions

```sql
-- On belief:
ALTER TABLE belief ADD COLUMN validity_horizon TIMESTAMPTZ;
-- NULL = no expiry (default for existing rows)

-- On action_intent:
ALTER TABLE action_intent ADD COLUMN expires_at TIMESTAMPTZ;
-- NULL = no expiry
-- Non-NULL = intent must be cancelled/renewed before this time

-- New CHECK on action_intent:
ALTER TABLE action_intent ADD CONSTRAINT intent_not_stale
    CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now());
```

**Why both columns:** The belief's `validity_horizon` is the source of truth. The intent's `expires_at` is a denormalized copy that makes the CHECK self-contained — it does not need to JOIN belief at check time. When `validity_horizon` changes, the intent's `expires_at` is updated in the same transaction.

### Invariant Impact

| Existing Invariant | Impact |
|---|---|
| I-1: `promoted_is_debt_free` | UNCHANGED — validity_horizon does not affect promotion CHECK |
| I-2: `live_requires_promoted` | UNCHANGED — intent_not_stale is a new CHECK, not a modification |
| I-3: `gate` FK | UNCHANGED — expires_at is not part of the FK |
| I-4: `belief_id_status_key` | UNCHANGED |

The new CHECK `intent_not_stale` is **additive**. An intent that was live before the migration remains live after — unless its expires_at has passed, in which case the first `EnsureFreshness` call cancels it.

## 2.5 Evidence Quality

**New column on evidence:**

```sql
ALTER TABLE evidence ADD COLUMN evidence_quality TEXT NOT NULL DEFAULT 'attested'
    CHECK (evidence_quality IN ('deterministic', 'attested', 'degraded'));
```

| Quality | Meaning | Example |
|---|---|---|
| `deterministic` | Reproducible, machine-verifiable | CVE scan, release binary hash |
| `attested` | Human-reported, trust-dependent | Maintainer comment, operator assertion |
| `degraded` | Source unavailable, snapshot only | Archived page, cached response |

**Invariant impact:** None. This is advisory metadata on the evidence table. No existing CHECK or FK references it.

## 2.6 Typed Debt

### Why Not Per-Belief Metadata

The proposed `debt_metadata` companion table creates a two-source-of-truth problem:

| Source | What It Says | CHECK Enforces? |
|---|---|---|
| `belief.debt TEXT[]` | "needProvenanceCheck is outstanding" | YES — `promoted_is_debt_free` |
| `debt_metadata` | "needProvenanceCheck is deterministic, needs 2 sources" | NO — advisory only |

If metadata says discharged but `TEXT[]` still contains it, promotion is blocked. If `TEXT[]` says discharged but metadata says needs 2 sources, the kernel might promote prematurely.

### Static Configuration Table (Preferred)

```sql
CREATE TABLE debt_config (
    item        TEXT PRIMARY KEY,
    debt_type   TEXT NOT NULL DEFAULT 'attested'
                CHECK (debt_type IN ('deterministic', 'attested', 'quorum')),
    min_sources INT NOT NULL DEFAULT 1
);
```

**Why this works:**
- `debt TEXT[]` remains the sole source of truth for CHECK enforcement
- `debt_config` is static configuration (like `FullDebt`), not per-row state
- The kernel consults `debt_config` when evaluating whether a discharge is sufficient
- No two-source-of-truth problem: `debt_config` is policy, `debt TEXT[]` is state

**What typed debt enables:**

| Current | With Typed Debt |
|---|---|
| "needProvenanceCheck" discharged by any evidence | Discharged by 2 independent deterministic sources |
| "needOperatorSignoff" discharged by operator typing anything | Discharged by operator attestation with identity recorded |
| "needContradictionSweep" discharged by one citation | Requires scanning all contradicting citations |

## 2.7 Agent Identity and Risk Tier

**New columns on action_intent:**

```sql
ALTER TABLE action_intent ADD COLUMN principal UUID;
-- NULL = anonymous (demo default)
-- Non-NULL = agent identity

ALTER TABLE action_intent ADD COLUMN risk_tier TEXT NOT NULL DEFAULT 'standard'
    CHECK (risk_tier IN ('standard', 'elevated', 'critical'));
```

| Tier | Requirements |
|---|---|
| `standard` | Promoted belief + discharged debt |
| `elevated` | Promoted belief + discharged debt + deterministic validation |
| `critical` | Promoted belief + discharged debt + deterministic + human approval + fresh evidence |

**Invariant impact:** None. These are advisory columns. No existing CHECK or FK references them.

---

# 3. RESEARCH / LONG-TERM

## 3.1 Semantic Belief Identity

The subject-based text lookup (§2.1) is fragile. Production use requires:

- **Belief fingerprinting:** Hash of normalized claim + subject + claim_type. Stable across re-entry.
- **Embedding-based matching:** Vector similarity between contradiction claim and existing beliefs. Requires an embedding column on belief (currently prohibited by I-6).
- **Ontological mapping:** Domain-specific entity resolution. Requires a knowledge graph.

**Open question:** Does adding an embedding column to belief violate I-6? I-6 says "vectors are never part of belief semantics." An embedding used only for lookup (not for promotion or authority) might be acceptable as a derived index, similar to `corpus_issue.embedding`.

## 3.2 Continuous Temporal Enforcement

No DBMS provides continuous CHECK evaluation. Research directions:

- **PostgreSQL triggers:** `BEFORE UPDATE` triggers can enforce temporal constraints. CockroachDB does not support triggers.
- **Event-driven architecture:** A change-data-capture pipeline that detects stale intents and writes cancellations. Requires infrastructure beyond the demo.
- **Materialized views with refresh:** A view that filters stale intents, refreshed on a timer. The underlying rows still say `state='live'`.
- **Hybrid approach (current recommendation):** CHECK + lazy expiration + periodic worker + authorization-time check. The strongest achievable guarantee without DBMS-level continuous evaluation.

## 3.3 Cross-Domain Evidence

Multiple evidence feeds require:

- Per-domain debt vocabularies (current `FullDebt` is etcd-specific)
- Cross-domain contradiction detection (evidence from feed A contradicts belief from feed B)
- Domain-specific provenance classes (current four values are etcd-specific)

**Dependency:** Typed debt (§2.6) must be implemented first.

## 3.4 Multi-Agent Authority Scoping

Multiple agents creating intents on the same belief require:

- Per-agent belief visibility (agent A can only see beliefs in its domain)
- Conflicting intents from different agents (agent B wants to deploy, agent C wants to wait)
- Authority delegation (agent A delegates to agent B)

**Dependency:** Agent identity (§2.7) must be implemented first.

## 3.5 Lean Temporal Model

**New concept in Types.lean:**
- `Option Time` on belief (validity_horizon)
- `Option Time` on intent (expires_at)

**New invariant in Invariants.lean:**
```lean
def intent_not_stale (l : Ledger) : Prop :=
  ∀ (iid : IntentId) (intent : ActionIntent),
    l.intents iid = some intent →
    intent.state = IntentState.live →
    intent.expiresAt = none ∨
    ∃ (now : Time), intent.expiresAt some t → t > now
```

**New theorem in Preservation.lean:**
- `ensureFreshness_preserves_validity` — proves that cancelling stale intents preserves ValidLedger

**New theorem (conceptual):**
- `stale_authority_cannot_persist` — proves that a live intent cannot remain live past its expires_at in a valid ledger

**Status:** Conceptual sketch only. Actual formalization deferred.

---

# 4. DATABASE EVOLUTION

## 4.1 Migration Strategy

All changes in new migration files. Never modify `db/001_schema.sql`.

| Migration | Content | Tier |
|---|---|---|
| `005_contradiction.sql` | `evidence_contradiction` table | POST-HACKATHON |
| `006_temporal.sql` | `validity_horizon`, `expires_at`, `intent_not_stale` CHECK | POST-HACKATHON |
| `007_evidence_quality.sql` | `evidence_quality` column | POST-HACKATHON |
| `008_debt_config.sql` | `debt_config` table | POST-HACKATHON |
| `009_agent_identity.sql` | `principal`, `risk_tier` columns | POST-HACKATHON |

**Rule:** Additive columns, additive tables, additive CHECKs. No existing column changes. No existing table modifications.

## 4.2 Invariant Preservation

| Feature | I-1 | I-2 | I-3 | I-4 | I-5 | I-6 | I-7 | I-8 |
|---|---|---|---|---|---|---|---|---|
| Contradiction relation | — | — | — | — | — | — | +1 site | — |
| Temporal authority | — | — | — | — | — | — | +2 sites | — |
| Evidence quality | — | — | — | — | — | — | — | — |
| Typed debt | — | — | — | — | — | — | — | — |
| Agent identity | — | — | — | — | — | — | — | — |

All existing invariants UNCHANGED. New invariants are additive (I-9 for contradiction relation).

## 4.3 Verification Strategy

| Layer | What to Verify | How |
|---|---|---|
| Schema | New CHECKs, FKs, tables | `SHOW CREATE TABLE`, CHECK re-evaluation tests |
| Kernel | New functions, I-7 count | Table-driven tests, `check_i7.sh` update |
| Lean | New theorems for new transitions | `lake build`, grep for sorry/admit |
| Integration | Full pipeline with contradiction + staleness | End-to-end test against live cluster |
| Deployment | Migration applies cleanly on existing data | `task test` on fresh + migrated databases |

---

# 5. GO KERNEL EVOLUTION

## 5.1 New Functions

```go
// FileContradiction records a contradiction and adds resolution debt.
// Single transaction: add debt + insert evidence_contradiction.
func (s *Store) FileContradiction(ctx context.Context, scenarioID, beliefID, evidenceID string) error

// EnsureFreshness cancels stale intents. Called before any intent-read path.
func (s *Store) EnsureFreshness(ctx context.Context, scenarioID string) (int, error)

// ExpireStaleIntents cancels stale intents across all scenarios.
// Runs on a ticker (e.g., every 60 seconds).
func (s *Store) ExpireStaleIntents(ctx context.Context) (int, error)

// SetBeliefHorizon sets or updates the validity_horizon on a belief.
func (s *Store) SetBeliefHorizon(ctx context.Context, beliefID string, horizon time.Time) error
```

## 5.2 Modified Functions

```go
// IntentOnPromoted: copy validity_horizon → expires_at when creating intent.
// No signature change. Internal SQL change only.

// RetractCascade: also nullify expires_at on cancelled intents.
// No signature change. Internal SQL change only.
```

## 5.3 I-7 Impact

| State | Write Sites | Function Count |
|---|---|---|
| Current | 7 | 8 functions |
| Post-hackathon | 10 | 12 functions |

New sites: `FileContradiction`, `EnsureFreshness`, `ExpireStaleIntents`. The `check_i7.sh` guard requires updating.

---

# 6. ARCHITECTURES REJECTED

| Architecture | Why Rejected |
|---|---|
| **Event sourcing** | The ledger IS the event log. Every row is an event. Adding a separate event store duplicates truth. |
| **Separate authority grant table** | `action_intent` + composite FK already IS the authority object. A separate table adds complexity without structural benefit. |
| **Automatic retraction on contradiction** | Too dangerous without human review. A single contradictory comment might not warrant retracting a promoted belief with live intents. |
| **Confidence scores as columns** | Derived display, never stored. Confidence is a function of evidence, not a property of the belief. Storing it creates a second source of truth. |
| **Per-belief debt metadata** | Two-source-of-truth problem. `debt_config` (static) is preferable to `debt_metadata` (per-row). |
| **P0 proposal (IsStale kernel check only)** | Not structurally enforced. A check that depends on every code path calling it is a procedural commitment, not a schema constraint. |
| **Lean redesign for temporal model** | Conceptual sketch only. Defer formalization until the production schema stabilizes. |
| **`belief_edge` for evidence→belief contradictions** | FK violation. Both `parent_id` and `child_id` reference `belief(id)`. Evidence IDs are not belief IDs. |
| **Subject-text matching in derive stage** | Breaks derive purity. The derive function must remain a deterministic function of evidence. Belief-target identification is an orchestration concern. |

---

# 7. ANSWER

**What should Solvent become after the hackathon if its thesis is taken to its logical conclusion?**

Solvent should become a system where:

1. **Contradictions are durable.** When evidence contradicts a belief, the contradiction is recorded in the belief graph with a link to the specific evidence and belief involved. The contradiction adds a resolution debt that blocks re-promotion. The operator decides whether to retract.

2. **Authority expires.** Every belief carries a temporal bound. Every intent carries a denormalized copy of that bound. A CHECK constraint catches bad writes. A lazy-expiration kernel function catches stale reads. A periodic worker catches missed expirations. An authorization-time check catches staleness at creation. Together, these four layers make live operationally mean ‘currently authorized,’ subject to the freshness enforcement protocol"

3. **Debt is typed.** A static configuration table defines discharge rules: deterministic validation requires machine verification, attested debt requires human sign-off, quorum debt requires multiple independent sources. The `TEXT[]` remains authoritative; the configuration table is policy.

4. **Agents are identified.** Every intent carries the identity of the agent that created it. Every intent carries a risk tier that determines the debt threshold required for authorization.

5. **The database remains the final authority.** Every guarantee is enforceable by schema constraints, not by application discipline. The application proposes; the database decides.

The core thesis survives intact:

> **Retrieval can be wrong.**
> **Judgment can be wrong.**
> **Authority cannot silently outlive what justifies it.**

---

*This document was produced by merging conclusions from three adversarial reviews (findings.md, second_order_review.md, findings2.md) against the actual schema, code, and CockroachDB v26.2.0 behavior. No files were modified. No implementation was produced.*
