# WAVE3_IMP.md

**Status:** Complete. Awaiting adversarial review.
**Scope:** Wave 3 — Wiring DerivedBelief outputs into the Transactional Belief Ledger.
**Implements:** `WAVE3_IMPLEMENTATION_PLAN.md`, `wave3_plan_rework.md`, `wave3_qa.md`.
**Frozen:** Wave 1 (`internal/normalize`) frozen. Wave 2 (`internal/derive`) frozen. Wave 3 (`internal/belief`, `internal/intent`) now frozen pending review.

---

## 1. Files Created/Modified

### Wiring layer (new)

| File | Lines | Purpose |
|---|---|---|
| `internal/belief/mapping.go` | 46 | Compile-time evidence→debt mapping table |
| `internal/belief/belief.go` | 102 | `Process()` — thin translation layer |
| `internal/belief/belief_test.go` | 310 | 7 unit tests |
| `internal/belief/integration_test.go` | 165 | 4 integration tests |
| `internal/intent/intent.go` | 26 | `Propose()`, `Audit()` wrappers |
| `internal/intent/intent_test.go` | 114 | 3 unit tests |

### Kernel additions (modified)

| File | Changes | Lines added |
|---|---|---|
| `internal/kernel/kernel.go` | `EnsureBelief` function | +17 |
| `internal/kernel/sql.go` | `sqlEnsureBelief` CTE, `SQLCatalog` entry | +15 |
| `internal/kernel/kernel_test.go` | 3 EnsureBelief tests | +60 |

### Documentation (new)

| File | Purpose |
|---|---|
| `WAVE3_IMPLEMENTATION.md` | Implementation receipts |
| `WAVE3_IMP.md` | This document |

**Total:** 9 files created, 3 files modified, ~755 lines added.

---

## 2. Architecture: What Wave 3 Is and Isn't

Wave 3 is a **thin translation layer**. It converts semantic objects (`derive.DerivedBelief`) into kernel operations. It does not own correctness, business logic, or orchestration.

```
derive.DerivedBelief
        │
        ▼
  belief.Process()        ← thin translator
        │
        ├── EnsureBelief   ← kernel (CTE, single tx)
        ├── AddEvidence    ← kernel (per evidence item)
        ├── RetireDebt     ← kernel (per debt item)
        └── Promote        ← kernel (may fail: debt not retired)
```

**What Process() owns:**
- Translating `derive.Classification` → `kernel.ClaimType`
- Calling `DebtItemsForEvidence()` to map evidence → debt items
- Calling `evidenceExists()` to deduplicate before `AddEvidence`
- Absorbing `ErrPromotionBlocked` as non-error

**What Process() does NOT own:**
- Business logic (which beliefs are true)
- Retry orchestration (crdb.ExecuteTx handles that)
- Debt lifecycle management (kernel + schema own that)
- Contradiction resolution (caller responsibility, deferred)
- Claim deduplication by similarity (Claim Agent responsibility)

---

## 3. Design Decisions

| # | Decision | Alternative Rejected | Justification |
|---|---|---|---|
| D1 | `EnsureBelief` as CTE find-or-create in kernel | Separate SELECT + INSERT in wiring layer | B2: kernel owns uniqueness semantics; single transaction eliminates TOCTOU |
| D2 | `Process()` as thin translator | Orchestration with business logic | B1: kernel owns correctness; wiring layer translates only |
| D3 | `evidenceExists()` TOCTOU accepted for MVP | Move dedup into kernel via new SQL | Schema has no unique constraint on `(belief_id, content_sha256)`; adding one would modify frozen DDL |
| D4 | Exact claim string equality for belief dedup | Vector similarity dedup | MVP scope; similarity dedup is Claim Agent responsibility, not wiring layer |
| D5 | `ErrPromotionBlocked → nil` absorbed | Return error to caller | Debt-not-fully-retired is expected intermediate state, not an error condition |
| D6 | Contradictions logged, not acted on | Full retraction pipeline | MVP scope; retraction target resolution requires caller-provided belief ID mapping |
| D7 | Sequential kernel transactions (4 steps) | Single combined transaction | Each step is independently retryable; intermediate states are visible but not harmful under SERIALIZABLE |

---

## 4. Public API

```go
// belief.Process translates a DerivedBelief into kernel operations.
// ErrPromotionBlocked is absorbed (debt not yet fully retired — expected).
// All other errors are returned immediately.
func Process(ctx context.Context, db *sql.DB, scenarioID string, b derive.DerivedBelief) error

// belief.DebtItemsForEvidence returns the debt items that the given evidence retires.
// Returns nil if no rule matches the source type or assertion pattern.
func DebtItemsForEvidence(sourceType, assertion string) []string

// intent.Propose records a live action intent on a promoted belief.
// Returns ErrActionOnUnpromoted if the belief is not currently promoted.
func Propose(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error

// intent.Audit returns the count of live intents whose belief is not promoted.
// Must always return 0 in committed state (invariant I-5).
func Audit(ctx context.Context, db *sql.DB, scenarioID string) (int, error)
```

---

## 5. Process() Walkthrough

```
Process(ctx, db, scenarioID, derivedBelief)
│
├─ IF claim == "" → return error (validation)
│
├─ IF contradicts non-empty → log, return nil (MVP: no retraction)
│
├─ Translate classification → kernel.ClaimType
│   "accommodated" → kernel.Accommodated
│   anything else  → kernel.Derived
│
├─ Step 1: EnsureBelief(scenarioID, claim, ct) → beliefID
│   CTE: SELECT existing OR INSERT new (single transaction)
│
├─ Step 2: FOR each SupportingEvidence:
│   ├─ IF evidenceExists(beliefID, sha256) → skip (dedup)
│   └─ AddEvidence(scenarioID, beliefID, provenance, url, sha256)
│
├─ Step 3: FOR each SupportingEvidence:
│   └─ FOR each DebtItemsForEvidence(sourceType, assertion):
│       └─ RetireDebt(beliefID, item)
│
└─ Step 4: Promote(beliefID)
    ├─ success → return nil (belief promoted)
    └─ ErrPromotionBlocked → return nil (debt not retired, expected)
    └─ other error → return error (fail loudly)
```

Error model: every kernel error propagates except `ErrPromotionBlocked`, which is
the schema refusing promotion because debt remains. This is the normal case for
partial evidence — the belief stays `entered` until enough evidence arrives.

---

## 6. Evidence→Debt Mapping

The mapping in `internal/belief/mapping.go` is the only place domain knowledge
enters the wiring layer. It is a compile-time constant — no configuration, no
runtime mutation.

| Source Type | Assertion Pattern (regex) | Debt Items Retired |
|---|---|---|
| `kev_entry` | `vulnerable to` | needMap |
| `release` | `release` | needMap, needInvariant |
| `maintainer_comment` | `fixed\|fix released\|patch available` | needMap, needInvariant |
| `maintainer_comment` | `tested\|confirmed` | needToyCheck, needNullModel |
| `maintainer_comment` | `no regression` | needNullModel, needObstruction |
| `maintainer_comment` | `security review\|reviewed by` | needFaithfulnessReview |
| `github_pr` | `fix` | needMap, needInvariant |

Full retirement requires all6 items. In practice this means a belief needs
evidence from at least: 1 KEV/release/PR entry + 3 maintainer comments
(confirmation + testing + no-regression + security review).

---

## 7. Kernel Changes: EnsureBelief

`EnsureBelief` is the only kernel function added in Wave 3. It performs
find-or-create in a single transaction via CTE:

```sql
WITH existing AS (
    SELECT id FROM belief
    WHERE scenario_id = $1::UUID AND claim = $2::STRING
    LIMIT 1
),
inserted AS (
    INSERT INTO belief (scenario_id, claim, claim_type)
    SELECT $1::UUID, $2::STRING, $3::STRING
    WHERE NOT EXISTS (SELECT 1 FROM existing)
    RETURNING id
)
SELECT id FROM existing
UNION ALL
SELECT id FROM inserted
```

The CTE ensures:
- **Atomicity**: find-or-create is one round trip, one transaction
- **No TOCTOU**: the INSERT is conditional on the SELECT finding nothing
- **Idempotency**: calling twice returns the same ID
- **Scenario isolation**: the WHERE clause scopes to `scenario_id`

Added to `SQLCatalog()` for verification support.

---

## 8. Test Strategy

### Unit tests — `internal/belief/belief_test.go` (7 tests)

| Test | What It Proves |
|---|---|
| `TestProcess_EnterNewBelief` | New belief created, 1 evidence row attached |
| `TestProcess_ExistingBelief` | Same claim reuses belief, adds 2nd evidence |
| `TestProcess_Promotion` | 6 evidence items retire all debt → promoted |
| `TestProcess_PromotionBlocked` | 1 evidence item leaves debt → stays entered |
| `TestProcess_Contradiction` | Contradiction does not enter belief |
| `TestProcess_MalformedBelief` | Empty claim rejected |
| `TestProcess_Determinism` | 100 replays → 1 belief, 1 evidence (idempotent) |

### Unit tests — `internal/intent/intent_test.go` (3 tests)

| Test | What It Proves |
|---|---|
| `TestPropose_OnPromotedBelief` | Intent on promoted belief succeeds |
| `TestPropose_OnUnpromotedBelief` | Intent on unpromoted belief fails (FK gate) |
| `TestAudit_NoLiveIntentsOnNonPromoted` | Audit returns 0 on clean state |

### Integration tests — `internal/belief/integration_test.go` (4 tests)

| Test | What It Proves |
|---|---|
| `TestIntegration_ProcessToIntent` | Full pipeline: evidence → belief → promote → propose → audit |
| `TestIntegration_PartialDebtBlocksPromotion` | Incomplete debt blocks promotion, blocks intent |
| `TestIntegration_MultiScenarioIsolation` | Two scenarios are independent |
| `TestIntegration_DeterminismAcrossReplays` | 100 replays → 1 belief, 1 evidence |

### Kernel tests — `internal/kernel/kernel_test.go` (3 new tests)

| Test | What It Proves |
|---|---|
| `W3-EnsureBelief_New` | EnsureBelief creates new belief with 6 debt items |
| `W3-EnsureBelief_Existing` | EnsureBelief returns same ID for duplicate claim |
| `W3-EnsureBelief_DifferentScenario` | Different scenarios produce different belief IDs |

---

## 9. Known Limitations & Deferred Work

### L1 — Contradictions not acted on

`Process()` logs contradictions but does not call `RetractCascade`. The
retraction target (which belief to retract) must be resolved by the caller
via claim matching or belief ID lookup. This is MVP scope limitation.

**Risk:** Contradicted beliefs remain promoted and actionable.
**Mitigation:** Manual review in demo. Full resolution requires belief
graph traversal (belief_edge table).

### L2 — Evidence dedup TOCTOU

`evidenceExists()` performs a SELECT before `AddEvidence` INSERT. Under
SERIALIZABLE isolation, a concurrent transaction inserting the same evidence
would cause a serialization failure and retry. The TOCTOU window exists but
is bounded by CockroachDB's serialization mechanism.

**Accepted per:** `wave3_qa.md` decision, Option A refined.

### L3 — Schema embedding column not added

`db/001_schema.sql` does not yet have `embedding VECTOR(1536) NULL`.
B-14 test not updated. Deferred to post-Wave 3.

### L4 — Sequential transactions, intermediate visibility

Each kernel step (EnsureBelief, AddEvidence, RetireDebt, Promote) runs in
its own transaction. An observer can see intermediate states:
- Belief entered, no evidence yet
- Evidence attached, debt not yet retired
- Debt retired, not yet promoted

Under SERIALIZABLE isolation these intermediate states are not visible to
concurrent transactions performing our protocol. Single-session observers
can see them. This is acceptable for the MVP demo.

**B3 from rework:** Partially addressed. Full justification requires
formal visibility analysis deferred to M3.

---

## 10. Verification Receipts

| Command | Exit Code | Proves |
|---|---|---|
| `go build ./...` | 0 | All packages compile |
| `go vet ./...` | 0 | No vet errors |
| `gofmt -l internal/belief internal/intent internal/kernel` | (empty) | Formatting clean |
| `go test ./internal/kernel/ -count=1` | 0 | 28/28 kernel tests pass |
| `go test ./internal/belief/ -count=1` | 0 | 11/11 belief tests pass |
| `go test ./internal/intent/ -count=1` | 0 | 3/3 intent tests pass |
| `go test ./internal/derive/ -count=1` | 0 | 16/16 derive tests pass (Wave 2 intact) |
| `go test ./internal/normalize/ -count=1` | 0 | 11/11 normalize tests pass (Wave 1 intact) |

**Total: 69 tests, all PASS.**

---

## 11. Adversarial Code Review Prompt

### Scope

Review all files in:
- `internal/belief/` (mapping.go, belief.go, belief_test.go, integration_test.go)
- `internal/intent/` (intent.go, intent_test.go)
- Kernel additions: `internal/kernel/kernel.go` (EnsureBelief), `internal/kernel/sql.go` (sqlEnsureBelief), `internal/kernel/kernel_test.go` (3 new tests)

### Authority

- `AGENTS.md` — architectural principles, development rules
- `IMPLEMENTATION_CONTRACT.md` — frozen schema, invariant definitions
- `WAVE3_IMPLEMENTATION_PLAN.md` — approved implementation plan
- `wave3_plan_rework.md` — Technical Lead rework findings (B1, B2, B3)
- `wave3_qa.md` — dedup ownership decision

### Review Passes

#### Pass 1 — Distributed Systems

| Question | Expected |
|---|---|
| Is `Process()` stateless? (no global mutable state, no clock reads, no randomness) | Yes |
| Are sequential kernel transactions safe under SERIALIZABLE? | Yes — each step retries via crdb.ExecuteTx |
| Is the TOCTOU in `evidenceExists()` bounded? | Yes — serialization failure retries the check |
| Can concurrent `Process()` calls on the same scenario corrupt state? | No — EnsureBelief CTE + evidence dedup + schema constraints |
| Does `EnsureBelief` CTE guarantee exactly-once belief creation? | Yes — UNION ALL of existing/inserted, scenario-scoped |

#### Pass 2 — Data Engineering

| Question | Expected |
|---|---|
| Does every evidence item correctly map to debt items via the compile-time table? | Verify mapping.go against6 debt items |
| Can evidence be lost? (e.g., dedup skip when evidence is actually new) | No — content_sha256 is content-addressed |
| Can debt be over-retired? (retiring an already-retired item) | No — array_remove on absent item is no-op |
| Does the claim dedup by exact string equality miss near-duplicates? | Yes, accepted — similarity dedup is Claim Agent scope |
| Is `DebtItemsForEvidence` deterministic? | Yes — regex matching on string input |

#### Pass 3 — Adversarial Systems

| Question | Expected |
|---|---|
| Can empty/malformed `DerivedBelief` bypass validation? | No — `Process()` rejects empty claim |
| Can evidence dedup be exploited to suppress new evidence? | No — content_sha256 is hash of normalized content |
| Can contradictions be weaponized to retract arbitrary beliefs? | No — contradiction path does not call RetractCascade |
| Can `ErrPromotionBlocked` be confused with success? | No — it IS expected success (debt not retired yet) |
| Can `intent.Propose` be called on a retracted belief? | No — composite FK enforces `belief_status='promoted'` |

#### Pass 4 — Spec Conformance

| Requirement | Implementation | Verdict |
|---|---|---|
| B1: Process() is thin translation | 102 lines, no business logic, each step is direct kernel call | Verify |
| B2: Kernel owns find-or-create | EnsureBelief CTE in kernel, not wiring layer | Verify |
| B3: Sequential transactions justified | Intermediate states visible but not harmful under SERIALIZABLE | Verify |
| C2: Evidence→debt is compile-time table | mapping.go: DebtMapping var, no runtime mutation | Verify |
| I-5: Audit always returns 0 in committed state | intent.Audit wraps kernel.AuditLiveOnNonPromoted | Verify |
| I-3: Intent requires promoted belief | intent.Propose wraps kernel.IntentOnPromoted (FK gate) | Verify |

### Empirical Probes

Run these probes and verify expected outcomes:

1. `Process()` with empty claim → **must error**
2. `Process()` with duplicate evidence (same sha256) → **must deduplicate (1 evidence row)**
3. `Process()` with1 kev_entry evidence → **must NOT promote (debt incomplete)**
4. `Process()` with6 evidence items covering all debt → **must promote**
5. `intent.Propose()` on unpromoted belief → **must fail (ErrActionOnUnpromoted)**
6. `EnsureBelief()` with same claim twice, same scenario → **must return same ID**
7. `EnsureBelief()` with same claim, different scenario → **must return different IDs**
8. 100 replays of `Process()` with same evidence → **1 belief, 1 evidence row**
9. `Process()` with contradiction → **no belief entered**
10. `intent.Audit()` on clean scenario → **0**

### Verdict Format

```
PASS / REWORK REQUIRED / BLOCKED

Findings:
| # | Severity | File:Line | Description | Resolution |
|---|---|---|---|---|

Receipts:
go build ./...                  → exit ?
go vet ./...                    → exit ?
go test ./internal/belief/...   → ?/? PASS
go test ./internal/intent/...   → ?/? PASS
go test ./internal/kernel/...   → ?/? PASS
```

**Decision is STOP on any P1 finding.** Do not redesign. Do not weaken the invariant. Do not adapt the test to the observed behavior.
