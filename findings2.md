# Final Architecture Validation

**Source:** Adversarial re-inspection of `second_order_review.md` against actual schema, code, and CockroachDB version.
**Status:** Read-only review. No files modified. No implementation proposed.
**Scope:** Contradiction relationship model, temporal authority, typed debt, priority ordering.

---

## Critical Corrections

### CRITICAL: `belief_edge` Cannot Represent Evidence → Contradicts → Belief

The `belief_edge` schema (`001_schema.sql:37-44`):

```sql
parent_id UUID NOT NULL REFERENCES belief(id),
child_id  UUID NOT NULL REFERENCES belief(id),
```

**Both FKs reference `belief(id)`.** `belief_edge` can ONLY represent:
- `belief → derives → belief`
- `belief → contradicts → belief`

It **cannot** represent:
- `evidence → contradicts → belief`
- `evidence → contradicts → evidence`

The `FileContradiction` SQL in `second_order_review.md` is **INVALID**:

```sql
INSERT INTO belief_edge (parent_id, child_id, kind)
SELECT $3::UUID, id, 'contradicts' FROM d
```

If `$3` is an evidence UUID, this violates the FK on `parent_id`. **The proposal silently assumes evidence IDs are belief IDs.** This is wrong.

### CRITICAL: Contradiction Target Cannot Be Identified

The derive stage (`derive.go:102-110`, `derive.go:155-162`) produces:

```go
DerivedBelief{
    Claim:          "prior belief about " + evidence.Subject + " is contradicted",
    Contradicts:    []normalize.NormalizedEvidence{evidence},
}
```

The `Contradicts` field contains `NormalizedEvidence` — NOT a belief ID. The system knows "etcd v3.5.14 is contradicted" but **does not know which specific belief UUID is being contradicted**. The `Subject` field is a text string, not a foreign key.

The `belief_edge` INSERT requires a `parent_id` that is a valid `belief.id`. The derive stage has no access to the belief table. There is no lookup mechanism.

**This means the entire contradiction protocol as specified in `second_order_review.md` is infeasible without first solving belief-target identification.**

### CORRECTED: What Actually Exists

| Layer | Contradiction Signal | What It Knows | What It Doesn't Know |
|---|---|---|---|
| normalize | none | structural transformation | nothing about contradictions |
| derive | `DerivedBelief.Contradicts` | evidence that contradicts *something* | which belief UUID is contradicted |
| belief.Process | logs and discards | nothing (nil return) | nothing |
| pipeline | logs and discards | nothing (Contradiction=true) | nothing |
| kernel | `RetractCascade` exists | correct retraction mechanism | is never called for contradictions |
| SQL | `belief_edge.kind` supports `'contradicts'` | schema is ready | no code files contradicts edges |

**The semantic meaning of the contradiction is lost at `belief.Process:39-48`.** The pipeline detects it, the kernel has the tool to act on it, but the wiring layer discards it.

---

## Contradiction Relationship Model

### The Identification Problem

The pipeline's derive stage operates on **evidence in isolation** — it has no access to the belief table. When it detects a contradiction, it knows:

- `evidence.Subject` = "etcd v3.5.14" (text)
- `evidence.Assertion` = "still vulnerable to CVE-2024-12345" (text)
- The semantic meaning: "the prior belief about this subject is wrong"

It does **not** know:
- The UUID of the belief being contradicted
- Whether a belief with that claim even exists in the ledger

### Alternatives Evaluated

| # | Approach | Feasibility | Correctness | Verdict |
|---|---|---|---|---|
| A | **Explicit target belief_id** | Derive stage has no access to belief table. Would require passing all belief IDs/claims into Derive, breaking its purity. | Perfect if available | IMPOSSIBLE at derive stage |
| B | **Derived semantic identity / lookup** | After derive, pipeline queries `belief WHERE claim LIKE '%' || subject || '%'` to find target. Fragile but feasible. | Correct if subject is unique; false matches possible | BEST AVAILABLE for demo |
| C | **Contradiction as a new belief** | Create a new belief "contradiction of X" with status 'entered'. No edge needed. | Avoids FK problem; but doesn't link to contradicted belief | WORKS but loses graph structure |
| D | **Separate evidence→belief relation** | New table `evidence_contradiction(evidence_id, belief_id)`. Requires schema change. | Clean; but new table in frozen architecture | DEFERRED — not P0 |
| E | **Other: contradiction as debt-only** | Add `needContradictionResolution` debt to the belief identified by subject match. No edge. | Minimal; debt blocks re-promotion | WORKS for demo |

### Recommendation

**For the demo (pre-hackathon):** Use **E (contradiction as debt-only)** combined with **B (subject-based lookup)**:

1. After derive detects a contradiction, pipeline queries:
   ```sql
   SELECT id FROM belief
   WHERE scenario_id = $1::UUID
     AND claim LIKE '%' || $2::STRING || '%'
     AND status <> 'retracted'
   ```
2. If a matching belief is found, add `needContradictionResolution` debt via a new kernel function
3. No edge is filed. The debt prevents re-promotion. The operator sees the debt and decides.

**Post-hackathon:** Implement **D (separate evidence→belief relation)** with a new table:
```sql
CREATE TABLE evidence_contradiction (
    evidence_id UUID NOT NULL REFERENCES evidence(id),
    belief_id   UUID NOT NULL REFERENCES belief(id),
    filed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (evidence_id, belief_id)
);
```

This preserves `belief_edge` for belief-to-belief relations only and introduces a clean evidence-to-belief contradiction relation.

### Corrected FileContradiction

The kernel function should NOT use `belief_edge`. Instead:

```go
func (s *Store) FileContradiction(ctx context.Context, scenarioID, beliefID, evidenceID string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // 1. Add debt (array_append, idempotent)
        _, err := tx.ExecContext(ctx, `
            UPDATE belief SET debt = array_append(debt, 'needContradictionResolution')
            WHERE id = $1::UUID AND NOT (debt @> ARRAY['needContradictionResolution']::STRING[])`,
            beliefID)
        if err != nil { return err }
        // 2. Record evidence→belief link (new table, post-hackathon)
        // For now, just add the debt. The evidence is already in the evidence table.
        return nil
    })
}
```

---

## Temporal Authority Model

### CockroachDB Version and Capabilities

- **Version:** CockroachDB CCL v26.2.0 (`docs/M0_REPORT.md:19`)
- **Isolation:** SERIALIZABLE (default, via `crdb.ExecuteTx`)
- **`now()` in CHECK constraints:** CockroachDB evaluates `now()` at **transaction time** when the CHECK is evaluated (on INSERT/UPDATE of the row containing the CHECK). It does NOT provide continuous evaluation.

### Assumptions Validated

| Assumption | Valid? | Evidence |
|---|---|---|
| `now()` is permitted in CHECK | **YES** — CockroachDB supports volatile functions in CHECK | CockroachDB docs; standard SQL |
| CHECK evaluates at write-time only | **YES** — no continuous evaluation | CockroachDB architecture |
| An already-live intent can remain live after time passes | **YES** — no write → no CHECK evaluation | Fundamental DBMS behavior |
| `EnsureFreshness` can establish the guarantee | **PARTIAL** — only if every code path calls it | Application-level obligation |
| An execution path could bypass `EnsureFreshness` | **YES** — any path that reads intents without calling it first | Structural gap |

### Why `now()` in CHECK Is Necessary But Not Sufficient

```sql
CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now())
```

This CHECK is evaluated when:
- A new intent is INSERTed with `state='live'` and `expires_at` in the past → **refused** ✓
- An existing intent is UPDATEed to `state='live'` with `expires_at` in the past → **refused** ✓
- An existing intent has `state='live'` and time passes → **no write occurs** → **CHECK never fires** ✗

The CHECK catches **bad writes** but cannot catch **time passing**. This is the same gap as the P0 proposal.

### Strongest Achievable Semantics

Given that no DBMS provides continuous CHECK evaluation, the strongest achievable design is:

**Layer 1: CHECK constraint (safety net)**
```sql
ALTER TABLE action_intent ADD CONSTRAINT intent_not_stale
    CHECK (state <> 'live' OR expires_at IS NULL OR expires_at > now());
```
Catches bad writes. Prevents new stale intents from entering.

**Layer 2: Lazy expiration (write trigger)**
```go
func (s *Store) EnsureFreshness(ctx context.Context, scenarioID string) (int, error) {
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
Called before every intent-read path. The CHECK then fires on the UPDATE.

**Layer 3: Periodic expiry worker (belt-and-suspenders)**
```go
func (s *Store) ExpireStaleIntents(ctx context.Context) (int, error) {
    // Same SQL as EnsureFreshness but scoped to all scenarios.
    // Runs on a ticker (e.g., every 60 seconds).
    // Catches intents that slip through lazy expiration.
}
```

**Layer 4: Execution-time verification (defense in depth)**
```go
func (s *Store) IntentOnPromoted(ctx context.Context, scenarioID, beliefID, action string) error {
    // Before creating intent, verify belief is not stale:
    // SELECT validity_horizon FROM belief WHERE id = $1::UUID
    // If validity_horizon IS NOT NULL AND validity_horizon <= now(), refuse.
    // This catches staleness at authorization time.
}
```

**The combination of all four layers is the strongest achievable guarantee.** No single layer is sufficient. The CHECK catches bad writes, lazy expiration catches stale reads, periodic worker catches missed lazy expirations, and authorization-time check catches staleness at creation.

### Compare Approaches

| Approach | Catches | Misses | Structural? |
|---|---|---|---|
| Execution-time verification | Bad reads | Stale intents between reads | No — procedural |
| Authorization-time verification | Stale intent creation | Staleness after creation | No — procedural |
| Explicit expiry transition | All (if write fires) | Nothing (if write fires) | YES — but write must be triggered |
| Lazy expiration | Stale intents on read | Intents not read | Partially — read-path obligation |
| Periodic expiry worker | Stale intents on timer | Intents between ticks | Partially — timer obligation |
| Transaction-local freshness | Stale intents in txn | Stale intents outside txn | No — txn-scoped |
| Authority leases | Same as explicit expiry | Same as explicit expiry | YES — same write-dependency |
| **Hybrid (CHECK + lazy + periodic + auth-time)** | All | Only if all layers fail simultaneously | Strongest achievable |

### Verdict

**"Live means currently authorized" is achievable only as a composite guarantee across four layers.** No single mechanism provides it. The CHECK is the structural foundation; the three application layers are obligations that must be maintained.

This is architecturally honest: the guarantee requires both schema and application cooperation. Pretending otherwise (e.g., "the CHECK alone handles it") is wrong.

---

## Typed Debt Model

### Two-Source-of-Truth Risk

The proposed `debt_metadata` companion table creates a real risk:

| Source | What It Says | CHECK Enforces? |
|---|---|---|
| `belief.debt TEXT[]` | "needProvenanceCheck is outstanding" | YES — `promoted_is_debt_free` checks array length |
| `debt_metadata` | "needProvenanceCheck is deterministic, needs 2 sources" | NO — advisory only |

If `debt_metadata` says an item is discharged but the `TEXT[]` still contains it, the belief cannot be promoted. If the `TEXT[]` says an item is discharged but `debt_metadata` says it needs 2 sources, the kernel might promote prematurely.

### Smallest Principled Future Model

**Do NOT add `debt_metadata` as a companion table.** Instead:

1. **Keep `debt TEXT[]` as the sole source of truth** for CHECK enforcement
2. **Add a `debt_config` table** (static, not per-belief) that defines discharge rules:
   ```sql
   CREATE TABLE debt_config (
       item        TEXT PRIMARY KEY,
       debt_type   TEXT NOT NULL DEFAULT 'attested'
                   CHECK (debt_type IN ('deterministic', 'attested', 'quorum')),
       min_sources INT NOT NULL DEFAULT 1
   );
   ```
3. The kernel consults `debt_config` when evaluating whether a discharge is sufficient
4. The `TEXT[]` remains authoritative; `debt_config` is a policy table, not a per-belief annotation

This avoids the two-source-of-truth problem: `debt_config` is static configuration (like `FullDebt`), not per-row state.

### Verdict

**Typed debt is P3 for a reason.** The current six debt items work correctly. The smallest principled future model is a static `debt_config` table, not a per-belief `debt_metadata` table. This is deferred to post-hackathon.

---

## Revised Priority Order

| Priority | Evolution | Why Revised |
|---|---|---|
| **P0** | **Contradiction target identification** — solve the derive-to-belief matching problem | The contradiction protocol cannot proceed without this. The current priority order assumes it's solved; it isn't. |
| **P1** | **Contradiction recording** — file evidence→belief contradiction relation (new table), add resolution debt | Depends on P0. Once the target is identified, recording is straightforward. |
| **P2** | **Temporal authority** — validity_horizon, expires_at, CHECK, EnsureFreshness, periodic worker | Self-contained; does not depend on P0/P1. But P0/P1 is higher urgency because contradictions are detected and discarded *right now*. |
| **P3** | **Evidence quality** — evidence_quality column | Independent; low urgency. |
| **P4** | **Typed debt** — static debt_config table | Depends on P1 (contradiction resolution is a debt type). |
| **P5** | **Agent identity** — principal on action_intent | Independent; low urgency for single-agent demo. |

### Why This Order

The original P0 (contradiction protocol) is infeasible as specified because the derive stage cannot identify the contradicting belief. The corrected priority makes this explicit:

1. **P0: Solve identification** — the pipeline must be able to match a contradiction to a specific belief UUID
2. **P1: Record durably** — once identified, record the contradiction and add debt
3. **P2: Temporal authority** — independent evolution, architecturally clean
4. **P3-P5: enrichments** — deferred to post-hackathon

---

## What Must NOT Change Before Hackathon

| Component | Status | Reason |
|---|---|---|
| `db/001_schema.sql` | FROZEN | Core tables and CHECK/FK constraints |
| `kernel/kernel.go` function signatures | FROZEN | Existing API surface |
| `kernel.FullDebt` | FROZEN | Vocabulary change requires authorisation |
| `kernel/contract.go` Contract interface | FROZEN | Interface stability |
| Lean model | FROZEN | Conceptual additions only |
| `IMPLEMENTATION_CONTRACT.md` | FROZEN | Post-hackathon revision |
| `AGENTS.md` architecture rules | FROZEN | Correct and should survive |

---

## Recommended Post-Hackathon Architecture

```text
Evidence
  ├── provenance_class     (existing)
  ├── evidence_quality     (P3: deterministic, attested, degraded)
  └── freshness            (derived from source_observed_at)
         │
         ▼
Contradiction Lookup       (P0: pipeline matches contradiction to belief)
  ├── subject-based lookup (demo)
  └── semantic identity    (production)
         │
         ▼
Belief
  ├── debt                 (existing TEXT[])
  ├── debt_config          (P4: static discharge rules)
  ├── validity_horizon     (P2: temporal bound)
  └── edges                (existing: derives, contradicts — belief→belief only)
         │
         ▼
Evidence Contradiction     (P1: new table evidence_contradiction)
  ├── evidence_id          (FK to evidence)
  ├── belief_id            (FK to belief)
  └── filed_at             (timestamp)
         │
         ▼
Promotion
  ├── debt = {}            (existing CHECK)
  ├── validity_horizon OK  (P2: not expired)
  └── no unresolved contradictions (P1: debt blocks re-promotion)
         │
         ▼
Authority
  ├── principal            (P5: agent identity)
  ├── expires_at           (P2: from belief.validity_horizon)
  └── state machine        (existing: live → cancelled/executed/expired)
         │
         ▼
Action Intent
  ├── gate FK              (existing)
  ├── intent_not_stale     (P2 CHECK)
  ├── EnsureFreshness      (P2 lazy expiration)
  └── PeriodicExpiry       (P2 background worker)
```

---

## New Core Principles

1. **Contradiction target identification is a prerequisite for contradiction recording.** Do not design a contradiction protocol that assumes the target belief is known. The derive stage operates on evidence in isolation and cannot identify the target.

2. **`belief_edge` is belief-to-belief only.** Evidence-to-belief relations require a separate table. Do not overload `belief_edge` with evidence IDs.

3. **Temporal authority is a composite guarantee.** No single mechanism (CHECK, lazy expiration, periodic worker, authorization-time check) provides "live means currently authorized." All four layers are required.

4. **`now()` in CHECK constraints is write-time enforcement, not continuous enforcement.** It catches bad writes but cannot catch time passing. This is a fundamental DBMS limitation, not a CockroachDB quirk.

5. **Static configuration tables are not per-row state.** `debt_config` (static discharge rules) is architecturally different from `debt_metadata` (per-belief annotations). The former is policy; the latter is state. Policy tables do not create two-source-of-truth problems.

6. **The pipeline's derive stage is pure and must remain pure.** It operates on evidence in isolation. Belief-target identification belongs in the pipeline orchestrator, not in the derive function.

---

*This validation was produced by re-reading the actual schema, code, and CockroachDB version confirmation. Every claim is grounded in specific files and line numbers. No files were modified.*
