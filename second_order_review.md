# Second-Order Solvent Design Review

**Source:** Findings from first-order adversarial review + deep re-inspection of contradiction chain, temporal authority, typed debt, and authority scope.
**Status:** Read-only review. No files modified. No implementation proposed.
**Scope:** Temporal validity, contradiction propagation (full chain), typed debt, authority model.

---

## Core Finding

Solvent's central guarantee is:

> **Authority cannot silently outlive the belief/evidence it depends on.**

This guarantee currently holds through two mechanisms:
1. **Debt gates promotion.** A belief with outstanding debt cannot become promoted. (CHECK constraint.)
2. **Retraction cascades to authority.** When a belief is retracted, live intents on it are cancelled via `ON UPDATE CASCADE` + `live_requires_promoted`. (Composite FK.)

Both mechanisms require a **database write** to fire. The first-order review proposed a `validity_horizon` column with a kernel-side `IsStale()` check. That proposal has a structural flaw: it only catches staleness at the moment of a new write. An already-live intent whose belief becomes stale between writes remains structurally "live" in the database with no triggering event to invalidate it.

This is **not** the same class of guarantee as retraction cascade. RetractCascade guarantees that a write to the belief status propagates to intents within the same transaction. Staleness has no such write — time passes, nothing fires.

The correct design must answer: **what does "live" mean when the evidence that justified the belief is no longer current?**

---

## Temporal Authority Analysis

### The Failure Sequence

```text
T+0:00  belief "etcd v3.5.x safe to deploy" promoted
        (debt discharged, final_truth=false)
T+0:01  action_intent "deploy etcd v3.5.28" becomes live
        (FK satisfied: belief is promoted)
T+0:02  validity_horizon expires (evidence is 90 days old)
        NO DATABASE WRITE OCCURS
T+0:03  system reads action_intent.state = 'live'
        nothing distinguishes this from T+0:01
```

At T+0:03, the database says `state = 'live'`. No CHECK constraint fires because no row was modified. No `ON UPDATE CASCADE` fires because no parent row changed. The intent is structurally live, but the evidence justifying the belief it cites is stale.

**This is the gap.** The current Solvent model has no mechanism to invalidate authority without a write.

### Approach Comparison

| # | Approach | How It Works | Does `state=live` still mean authorized? | Verdict |
|---|---|---|---|---|
| A | **Execution-time freshness check** | Check `now() > validity_horizon` at the moment an intent is *executed* (not just read) | Only if "live" is redefined as "live AND fresh" — but the database row still says `state='live'` | PARTIAL — catches execution, not representation |
| B | **Authorization-time freshness check** | Check `now() > validity_horizon` when creating intent via `IntentOnPromoted` | Only at creation — staleness after creation is invisible | WEAK — same gap as P0 proposal |
| C | **Explicit expiry state transition** | Add `expired` to `action_intent.state` CHECK; a background process or write sets `state='expired'` when `now() > validity_horizon` | YES — if the transition fires. But requires a write, which requires something to trigger it | STRONGEST — but requires a write trigger |
| D | **Authority lease** | Intent carries its own `expires_at`; CHECK adds `state <> 'live' OR expires_at > now()` | YES — but `now()` is transaction-stable in CRDB; the CHECK only fires on row modification | MODERATE — same write-dependency as C |
| E | **Temporal predicates/views** | Filtered view: `CREATE VIEW live_authority AS SELECT * FROM action_intent WHERE state='live' AND (belief_id NOT IN (SELECT id FROM belief WHERE validity_horizon IS NOT NULL AND validity_horizon <= now()))` | The VIEW filters correctly, but the underlying row still says `state='live'` | WEAK — representation gap remains |
| F | **Database-enforced (CHECK with now())** | `CHECK (state <> 'live' OR validity_horizon IS NULL OR validity_horizon > now())` | CRDB evaluates `now()` at transaction time, not continuously. This CHECK fires only when the row is modified. | MODERATE — better than B, same as D |
| G | **Hybrid Go + CRDB** | Go detects stale beliefs, writes cancellation to intents; CRDB CHECK catches any gap | YES — if the Go detection is reliable and covers all paths | STRONG — but introduces application-side obligation |

### Analysis

**Approach C (explicit expiry state transition) is the only one that preserves Solvent's structural guarantee.** The key insight is:

> The guarantee must be representable as a database state, not as an application-side check.

If `state = 'live'` means "currently authorized," then a belief becoming stale must cause a **write** that changes `state` from `'live'` to `'expired'` (or `'cancelled'`). This write can be:
- Triggered by a background process (polling for stale beliefs)
- Triggered by any read path that detects staleness (lazy expiration)
- Triggered by `ON UPDATE CASCADE` if `validity_horizon` is on `belief` and a CHECK on `action_intent` references it

**The cleanest approach is D/F hybrid:**
1. Add `validity_horizon TIMESTAMPTZ` to `belief`
2. Add `expires_at TIMESTAMPTZ` to `action_intent` (copied from belief at intent creation)
3. Add CHECK: `state <> 'live' OR expires_at IS NULL OR expires_at > now()`
4. A read path that detects `expires_at IS NOT NULL AND expires_at <= now()` writes `state = 'expired'`

This creates a **self-healing authority boundary**: any code path that encounters a stale intent must update it before the transaction commits, and the CHECK ensures no stale intent survives.

### Why NOT the P0 Proposal

The first-order review's P0 (kernel `IsStale()` check) fails because:

```text
IsStale(belief) → true
    ↓
who writes the intent to cancelled?
    ↓
if nobody calls IsStale, the intent stays live
```

The check is correct but **not structurally enforced**. It depends on every code path that reads an intent to also check staleness. That is the same category of guarantee as "human review is required" — a procedural commitment, not a schema constraint.

### Recommended Temporal Model

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

**Why both columns?** The belief's `validity_horizon` is the source of truth. The intent's `expires_at` is a denormalized copy that makes the CHECK self-contained — it does not need to JOIN belief at check time. When `validity_horizon` changes (e.g., new evidence resets the clock), the intent's `expires_at` is updated in the same transaction.

**Write trigger:** A lazy-expiration path in the kernel:

```go
func (s *Store) EnsureFreshness(ctx context.Context, scenarioID string) (int, error) {
    // Cancel any live intent whose expires_at has passed.
    // This is the write that fires the CHECK and makes staleness structural.
    var cancelled int
    err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        res, err := tx.ExecContext(ctx, `
            UPDATE action_intent SET state = 'expired'
            WHERE state = 'live' AND scenario_id = $1::UUID
              AND expires_at IS NOT NULL AND expires_at <= now()`, scenarioID)
        if err != nil { return err }
        n, err := res.RowsAffected()
        if err != nil { return err }
        cancelled = int(n)
        return nil
    })
    return cancelled, err
}
```

This is called before any intent-read path. The CHECK is the structural guarantee; `EnsureFreshness` is the write that makes it fire. Together they close the gap.

---

## Contradiction Propagation Analysis

### Full Chain Trace

**Stage 1: normalize**
- Input: raw JSON fixture
- Output: `NormalizedEvidence` with `SourceType`, `Assertion`, `ContentSHA256`
- Contradiction signal: **none at this stage.** Normalization is a structural transformation. It does not know whether the evidence contradicts anything.

**Stage 2: derive**
- Input: `NormalizedEvidence`
- Output: `[]DerivedBelief`
- Contradiction signal: `DerivedBelief.Contradicts` field (non-empty `[]NormalizedEvidence`)
- Where contradictions originate:
  - `deriveFromMaintainerComment` (`derive.go:102`): `reproducesPattern` match → `Claim: "prior belief about X is contradicted"`, `Contradicts: [evidence]`
  - `deriveFromGitHubIssue` (`derive.go:151`): `reproducesPattern` match → same pattern
- Key observation: the contradiction carries a **Claim** text (`"prior belief about X is contradicted"`) AND the **Contradicts** evidence, but does NOT identify WHICH belief is contradicted. The claim text is a loose string match, not a belief ID.

**Stage 3: belief.Process**
- Input: `DerivedBelief` with non-empty `Contradicts`
- Current behavior (`belief.go:39-48`):
  ```go
  if len(b.Contradicts) > 0 {
      for _, c := range b.Contradicts {
          slog.Warn("belief.Process: contradiction received, no ledger mutation",
              "source_url", c.SourceURL,
              "source_type", c.SourceType,
              "claim", b.Claim,
          )
      }
      return nil
  }
  ```
- **The contradiction is logged and discarded.** No belief is created, no edge is filed, no debt is added, no retraction is triggered. The evidence that contradicts a prior belief is lost from the durable ledger.

**Stage 4: pipeline**
- `pipeline.ProcessEvidence` (`pipeline.go:90-100`): same pattern — logs warning, sets `Result.Contradiction = true`, continues
- `pipeline.Run` (`pipeline.go:354-372`): collects contradictions, logs warnings, appends to results — **no ledger mutation**
- The pipeline's contradiction handling is identical to `belief.Process`: log and discard.

**Stage 5: kernel**
- The kernel has `RetractCascade` which IS the correct retraction mechanism
- But nothing in the pipeline calls it for contradictions
- The kernel is correct; the gap is in the wiring layer

**Stage 6: SQL**
- `belief_edge` has a `contradicts` kind, but no code files contradicts edges
- The demo's falsification path uses `belief_corpus_citation` with `relation = 'contradicts'` — but that is the wizard's manual path, not the automated pipeline

### Where the Contradiction's Semantic Meaning Is Lost

The contradiction's meaning is lost at `belief.Process:39-48`. The `derive` stage correctly identifies that evidence contradicts a prior belief and carries both the claim text and the evidence. But `belief.Process` logs a warning and returns nil — the durable ledger never learns that a contradiction exists.

This is exactly the **epistemic-to-authority discontinuity** that `prompt2.md` identifies. The system knows a contradiction exists (it detected it) but does not represent it in the belief graph (it discarded it).

### What SHOULD Happen

The answer is NOT "always automatically retract." Automatic retraction is dangerous because:
1. A single contradictory comment might not warrant retracting a promoted belief
2. The contradiction target is identified by claim text, not belief ID — false positives are possible
3. Automatic retraction of a belief with live intents would trigger cascade cancellation without human review

The correct design is a **three-phase contradiction protocol:**

**Phase 1: Record the contradiction (durable)**
- File a `contradicts` edge in `belief_edge` between the contradicted belief and the contradicting evidence
- Add a `needContradictionResolution` debt item to the contradicted belief
- This makes the contradiction visible in the belief graph without changing authority

**Phase 2: Block promotion (structural)**
- The new debt item prevents re-promotion of a retracted belief
- If the belief is already promoted, the debt does not immediately retract it — the contradiction is recorded but authority is not yet affected

**Phase 3: Resolve (operator action)**
- The operator reviews the contradiction and either:
  - Discharges the debt (contradiction was false alarm → belief stays promoted)
  - Retracts the belief (contradiction is valid → cascade cancels intents)

This matches Solvent's existing philosophy: **evidence proposes, the database enforces, the operator decides.** The contradiction is evidence, not authority. Recording it is mandatory; acting on it is an operator decision.

### Minimal Durable State for Contradictions

```sql
-- belief_edge already supports 'contradicts' kind:
-- kind TEXT NOT NULL DEFAULT 'derives' CHECK (kind IN ('derives','contradicts'))
-- No schema change needed.

-- New debt item for contradiction resolution:
-- Added to FullDebt and belief.debt DEFAULT:
'needContradictionResolution'
```

The kernel change is one function:

```go
func (s *Store) FileContradiction(ctx context.Context, scenarioID, beliefID, contradictingEvidenceID, claimText string) error
```

This files a `contradicts` edge and adds `needContradictionResolution` debt in one transaction. The belief remains in its current status. Authority is not affected until the operator acts.

---

## Typed Debt Analysis

### Current Model

```sql
debt TEXT[] NOT NULL DEFAULT ARRAY['needProvenanceCheck', ...]
```

Boolean: present = outstanding, absent = discharged. No types, no cardinality, no independence.

### The Smallest Useful Generalization

Do NOT jump to a new table. The smallest model that represents the six dimensions is:

```sql
-- Extend the TEXT[] with a structured encoding:
-- Each debt item is "name:meta" where meta is optional JSON
-- Example: "needProvenanceCheck:{\"type\":\"deterministic\",\"min_sources\":2}"
```

This is ugly and wrong. **Do not do this.**

The correct smallest generalization is a **companion table** that annotates debt items without changing the core `TEXT[]` mechanism:

```sql
CREATE TABLE debt_metadata (
    belief_id   UUID NOT NULL REFERENCES belief(id),
    item        TEXT NOT NULL,
    debt_type   TEXT NOT NULL DEFAULT 'attested'
                CHECK (debt_type IN ('deterministic', 'attested', 'quorum')),
    min_sources INT NOT NULL DEFAULT 1,
    PRIMARY KEY (belief_id, item)
);
```

**Why this works:**
- The core `debt TEXT[]` and `promoted_is_debt_free` CHECK remain unchanged
- `debt_metadata` is advisory — the kernel can consult it when deciding whether a discharge is sufficient
- `debt_type` distinguishes deterministic validation from human attestation
- `min_sources` enables quorum requirements (e.g., "need 2 independent confirmations")
- The CHECK on `debt_type` is in the metadata table, not the belief table — no frozen architecture change

**But this is P3 for a reason.** The current six debt items work correctly for the demo. Typed debt is a future evolution that becomes important when Solvent handles more than one evidence domain.

### What Typed Debt Enables

| Current | With Typed Debt |
|---|---|
| "needProvenanceCheck" discharged by any evidence | "needProvenanceCheck" discharged by 2 independent deterministic sources |
| "needOperatorSignoff" discharged by operator typing anything | "needOperatorSignoff" discharged by operator attestation with identity recorded |
| "needContradictionSweep" discharged by one citation | "needContradictionSweep" requires scanning all contradicting citations |

The conceptual separation is:

```text
DebtItem
  ├── name           (what obligation)
  ├── type           (deterministic / attested / quorum)
  ├── min_sources    (how many independent sources needed)
  ├── freshness      (how long a discharge remains valid)
  └── discharge_proof (what evidence actually paid it)
```

---

## Authority Model Analysis

### Current: `action_intent` IS the Authority Object

```sql
action_intent (
    id            UUID PRIMARY KEY,
    scenario_id   UUID NOT NULL,
    belief_id     UUID NOT NULL,
    belief_status TEXT NOT NULL DEFAULT 'promoted',
    action        TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'live' CHECK (...),
    expires_at    TIMESTAMPTZ,       -- proposed
    CONSTRAINT gate FOREIGN KEY (belief_id, belief_status)
      REFERENCES belief(id, status) ON UPDATE CASCADE
)
```

This already provides:
- **Identity:** UUID primary key
- **Belief binding:** belief_id + belief_status (FK)
- **Action:** text field
- **State machine:** live → cancelled / executed / expired
- **Temporal bounds:** expires_at (proposed)

### What It Does NOT Provide

| Missing | Why It Matters | Recommendation |
|---|---|---|
| **principal** | Who created this intent? Two agents might create intents on the same belief with different actions. Currently indistinguishable. | ADD for multi-agent. Not needed for single-agent demo. |
| **scope** | What systems does this intent affect? "deploy etcd" vs "deploy to staging" vs "deploy to production." Currently one action string. | DEFER — action text can encode scope for now. |
| **risk tier** | Should different actions require different debt thresholds? "deploy to production" vs "open a PR" have very different consequences. | DEFER — natural extension of typed debt. |
| **validity interval** | Covered by expires_at. | INCLUDE in temporal model. |

### Verdict

`action_intent` is sufficient for the current single-agent demo. For multi-agent production use, it needs `principal` (UUID referencing an agent identity). Risk tier and scope are downstream of typed debt — they become meaningful when debt items carry type information.

The recommended evolution is:

```sql
ALTER TABLE action_intent ADD COLUMN principal UUID;
-- NULL = anonymous (demo default)
-- Non-NULL = agent identity

ALTER TABLE action_intent ADD COLUMN risk_tier TEXT NOT NULL DEFAULT 'standard'
    CHECK (risk_tier IN ('standard', 'elevated', 'critical'));
-- standard: promoted belief + discharged debt
-- elevated: promoted belief + discharged debt + deterministic validation
-- critical: promoted belief + discharged debt + deterministic + human approval + fresh evidence
```

But this is P2, not P0. The current system is correct for its scope.

---

## Recommended Semantic Model

```text
Evidence
  ├── provenance_class     (existing: external_feed, reproducible_artifact, ...)
  ├── evidence_quality     (NEW: deterministic, attested, degraded)
  └── freshness            (derived from source_observed_at)
         │
         ▼
Belief
  ├── debt                 (existing: TEXT[])
  ├── debt_metadata        (NEW: per-item type, cardinality, freshness)
  ├── validity_horizon     (NEW: when this belief needs revalidation)
  └── edges                (existing: derives, contradicts)
         │
         ▼
Contradiction Protocol     (NEW: not automatic retraction)
  ├── file contradicts edge
  ├── add needContradictionResolution debt
  └── operator reviews and resolves
         │
         ▼
Promotion
  ├── debt = {}            (existing CHECK)
  ├── validity_horizon OK  (NEW: not expired)
  └── risk tier met        (FUTURE: typed debt satisfaction)
         │
         ▼
Authority
  ├── principal            (FUTURE: agent identity)
  ├── expires_at           (NEW: from belief.validity_horizon)
  ├── risk_tier            (FUTURE: action risk classification)
  └── state machine        (existing: live → cancelled/executed/expired)
         │
         ▼
Action Intent
  ├── gate FK              (existing: belief_id + belief_status)
  ├── intent_not_stale     (NEW CHECK: state <> 'live' OR expires_at IS NULL OR expires_at > now())
  └── EnsureFreshness      (NEW kernel: lazy-expire stale intents)
```

The critical insight: **temporal authority requires a write.** The CHECK `intent_not_stale` is the structural guarantee; `EnsureFreshness` is the write that makes it fire. Together they are the same class of guarantee as retraction cascade — the database refuses to let stale authority survive.

---

## Database Consequences

### Immediate (P0 + Contradiction Protocol)

```sql
-- 1. Temporal validity
ALTER TABLE belief ADD COLUMN validity_horizon TIMESTAMPTZ;
ALTER TABLE action_intent ADD COLUMN expires_at TIMESTAMPTZ;
ALTER TABLE action_intent ADD CONSTRAINT intent_not_stale
    CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now());

-- 2. Contradiction debt
-- No schema change. Add 'needContradictionResolution' to FullDebt
-- and belief.debt DEFAULT.
```

### Deferred (P1+)

```sql
-- Evidence quality (P1)
ALTER TABLE evidence ADD COLUMN evidence_quality TEXT NOT NULL DEFAULT 'attested'
    CHECK (evidence_quality IN ('deterministic', 'attested', 'degraded'));

-- Debt metadata (P3)
CREATE TABLE debt_metadata (...);

-- Agent identity (P2)
ALTER TABLE action_intent ADD COLUMN principal UUID;
ALTER TABLE action_intent ADD COLUMN risk_tier TEXT NOT NULL DEFAULT 'standard'
    CHECK (risk_tier IN ('standard', 'elevated', 'critical'));
```

### Invariant Impact

| Existing Invariant | Impact |
|---|---|
| I-1: `promoted_is_debt_free` | UNCHANGED — validity_horizon does not affect promotion CHECK |
| I-2: `live_requires_promoted` | UNCHANGED — intent_not_stale is a new CHECK, not a modification |
| I-3: `gate` FK | UNCHANGED — expires_at is not part of the FK |
| I-4: `belief_id_status_key` | UNCHANGED |

The new CHECK `intent_not_stale` is **additive**. It does not modify any existing constraint. An intent that was live before the migration remains live after — unless its expires_at has passed, in which case the first `EnsureFreshness` call cancels it.

---

## Go Consequences

### New Kernel Functions

```go
// EnsureFreshness cancels stale intents. Called before any intent-read path.
func (s *Store) EnsureFreshness(ctx context.Context, scenarioID string) (int, error)

// FileContradiction records a contradiction and adds resolution debt.
func (s *Store) FileContradiction(ctx context.Context, scenarioID, beliefID, evidenceID, claimText string) error

// SetBeliefHorizon sets or updates the validity_horizon on a belief.
func (s *Store) SetBeliefHorizon(ctx context.Context, beliefID string, horizon time.Time) error
```

### Modified Functions

```go
// IntentOnPromoted: copy validity_horizon → expires_at when creating intent.
// No signature change. Internal SQL change only.

// RetractCascade: also update expires_at to NULL on cancelled intents.
// No signature change. Internal SQL change only.
```

### New SQL Statements

```go
const sqlEnsureFreshness = `
    UPDATE action_intent SET state = 'expired'
    WHERE state = 'live' AND scenario_id = $1::UUID
      AND expires_at IS NOT NULL AND expires_at <= now()`

const sqlFileContradiction = `
    WITH RECURSIVE d(id) AS (
        SELECT b.id FROM belief b
        WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
      UNION
        SELECT e.child_id FROM belief_edge e
        JOIN d ON e.parent_id = d.id
        JOIN belief cb ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
    )
    INSERT INTO belief_edge (parent_id, child_id, kind)
    SELECT $3::UUID, id, 'contradicts' FROM d
    ON CONFLICT (parent_id, child_id) DO NOTHING`

const sqlSetBeliefHorizon = `
    UPDATE belief SET validity_horizon = $2::TIMESTAMPTZ
    WHERE id = $1::UUID`
```

### I-7 Impact

The kernel currently has exactly 7 `crdb.ExecuteTx` write sites. Adding `EnsureFreshness`, `FileContradiction`, and `SetBeliefHorizon` brings it to 10. The `scripts/check_i7.sh` guard would need updating — but this is post-hackathon.

---

## Lean Consequences

The Lean model proves properties of an abstract state machine with four transitions and three invariants. Adding temporal validity means:

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

This is a conceptual sketch, not a redesign of Types.lean. The actual formalization is deferred.

---

## What Should Remain Frozen

| Component | Status | Reason |
|---|---|---|
| `db/001_schema.sql` | FROZEN | The four core tables and their CHECK/FK constraints are the product. Additive columns (validity_horizon, expires_at) go in a new migration file, not 001. |
| `kernel/kernel.go` function signatures | FROZEN | New functions are additive. Existing signatures do not change. |
| `kernel.FullDebt` | FROZEN for now | Adding `needContradictionResolution` is a vocabulary amendment, not an architecture change. Authorised by the same process as Phase 5's vocabulary change. |
| `kernel/contract.go` Contract interface | FROZEN | New methods are additive. The interface does not need to change until post-hackathon. |
| Lean model | FROZEN for now | Conceptual additions only. No actual Lean code changes. |
| `IMPLEMENTATION_CONTRACT.md` | FROZEN | Post-hackathon revision. |
| `AGENTS.md` architecture rules | FROZEN | These rules are correct and should survive the evolution. |

---

## Post-Hackathon Priority Order

| Priority | Evolution | Why This Order |
|---|---|---|
| **P0** | **Contradiction protocol** — file contradicts edges, add resolution debt, no automatic retraction | Closes the epistemic-to-authority gap. Requires no schema changes to frozen tables. Enables the belief graph to represent contradictions durably. |
| **P1** | **Temporal authority** — validity_horizon, expires_at, intent_not_stale CHECK, EnsureFreshness | Extends the central guarantee to cover time. Requires additive schema changes. |
| **P2** | **Evidence quality** — evidence_quality column, deterministic/attested/degraded | Enriches the evidence model. Valuable once typed debt exists. |
| **P3** | **Typed debt** — debt_metadata table, per-item type/cardinality/freshness | Turns demo-specific obligations into a general epistemic mechanism. Foundation for risk-tiered authority. |
| **P4** | **Agent identity** — principal on action_intent, risk_tier | Natural evolution for multi-agent/consequential actions. |

### Why Contradiction Before Temporal

Contradiction protocol is P0 because:
1. It closes a gap that exists **right now** — contradictory evidence is detected and discarded
2. It requires no schema changes to frozen tables — only a new edge type usage and a new debt item
3. It strengthens the core thesis immediately — "evidence changes belief → authority must change"
4. It is the natural precursor to typed debt — contradiction resolution is a debt type

Temporal authority is P1 because:
1. It requires additive schema changes (new columns, new CHECK)
2. It requires a new kernel function (EnsureFreshness) that changes the I-7 write count
3. It is architecturally clean but lower urgency — the demo does not need staleness

---

## What This Review Does NOT Recommend

1. **Event sourcing** — the ledger IS the event log
2. **Separate authority grant table** — action_intent + composite FK already IS the authority object
3. **Automatic retraction on contradiction** — too dangerous without human review
4. **Confidence scores as columns** — derived display, never stored
5. **Lean redesign for temporal model** — conceptual sketch only, defer formalization
6. **Any changes before hackathon submission** — the current system is strong

---

*This review was produced by tracing the actual code paths in the Solvent codebase. Every claim is grounded in specific files and line numbers. No files were modified.*
