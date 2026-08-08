# WAVE3_IMPLEMENTATION_PLAN.md (Revised)

**Status:** Revised per Technical Lead review (`wave3_plan_rework.md`, `wave3_qa.md`).
**Scope:** Wave 3 — Belief wiring into the Transactional Belief Ledger.
**Implements:** `DATA_PIPELINE_SPEC.md` §5, §6, §7 and `IMPLEMENTATION_CONTRACT.md` §4.
**Frozen:** Wave 1 (`internal/normalize`) and Wave 2 (`internal/derive`) are frozen.

---

## 1. Scope

### In Scope

- `kernel.EnsureBelief` — additive kernel function (find-or-enter in one transaction)
- `internal/belief` package — thin translation layer: DerivedBelief → kernel calls
- `internal/intent` package — propose/cancel action intents on promoted beliefs
- Evidence-to-debt retirement mapping (compile-time table)
- Unit tests for both packages
- Integration test proving the full pipeline: raw JSON → normalize → derive → belief → kernel → audit

### Out of Scope

- Normalization (Wave 1, frozen)
- Belief derivation (Wave 2, frozen)
- Kernel modifications beyond `EnsureBelief` (M0–M2 frozen)
- Schema migration (embedding column, deferred)
- EvidenceFeed / polling / GitHub API / KEV API
- Embeddings / vector search
- Security Agent orchestration (demo phase)
- Graph visualization
- bchat / UI / demo / AWS
- Multi-agent concurrency (M3 isolation experiment)

---

## 2. Explicit Non-Scope

Do NOT build:

- A third agent or evidence feed
- Embedding computation or vector similarity dedup
- The EvidenceFeed interface
- Pipeline orchestrator (polling loop)
- Action execution (only propose/cancel intents)
- Stored confidence scores (derived display only)
- Schema changes to `db/001_schema.sql`
- Any modification to `internal/normalize` or `internal/derive`
- Any kernel modification beyond `EnsureBelief`

---

## 3. Package/File Layout

```
internal/
├── kernel/
│   ├── kernel.go             + EnsureBelief (additive, one function)
│   ├── sql.go                + sqlEnsureBelief (additive, one statement)
│   └── kernel_test.go        + TestEnsureBelief (additive, one test)
├── belief/
│   ├── belief.go             Thin translation: DerivedBelief → kernel calls
│   ├── mapping.go            Compile-time evidence→debt mapping table
│   └── belief_test.go        Unit tests for belief wiring
├── intent/
│   ├── intent.go             Propose/cancel action intents
│   └── intent_test.go        Unit tests for intent wiring
├── derive/                    (frozen, Wave 2)
├── normalize/                 (frozen, Wave 1)
└── testdb/                    (test infrastructure, frozen)
```

**Total new:** ~6 files, ~500 lines estimated.

---

## 4. Public API

### `internal/kernel` (additive)

```go
// EnsureBelief returns the ID of a belief with the given claim in the scenario.
// If no such belief exists, it creates one with the given claim type, full starting
// debt, and status='entered'.
//
// The find-or-create is a single transaction — no TOCTOU boundary.
// The caller does not need to know whether the belief was newly created.
func (s *Store) EnsureBelief(ctx context.Context, scenarioID, claim string, ct ClaimType) (string, error)
```

### `internal/belief`

```go
package belief

// Process translates a single DerivedBelief into kernel operations.
// It is a thin translation layer — no business logic, no orchestration.
//
// Each step is a direct kernel call:
//   1. EnsureBelief → beliefID
//   2. AddEvidence for each SupportingEvidence item
//   3. RetireDebt for each debt item the evidence covers
//   4. Promote (may fail with ErrPromotionBlocked — acceptable)
//
// Contradictions bypass steps 2-4 and call RetractCascade.
//
// Error handling:
//   - ErrPromotionBlocked → nil (debt not yet fully retired)
//   - Other errors → returned immediately (fail loudly)
func Process(ctx context.Context, db *sql.DB, scenarioID string, b derive.DerivedBelief) error
```

### `internal/intent`

```go
package intent

// Propose records a live action intent on a promoted belief.
// Returns ErrActionOnUnpromoted if the belief is not currently promoted.
func Propose(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error

// Audit returns the count of live intents whose belief is not promoted.
// Must always return 0 in committed state (invariant I-5).
func Audit(ctx context.Context, db *sql.DB, scenarioID string) (int, error)
```

---

## 5. Kernel Responsibilities

The kernel (`internal/kernel`) provides the transactional primitives:

| Kernel Function | Wave 3 Usage |
|---|---|
| `EnsureBelief` | **New.** Find-or-enter belief in one transaction. Returns belief ID. |
| `AddEvidence` | Attach each `SupportingEvidence` item to the belief |
| `RetireDebt` | Retire debt items covered by the evidence |
| `Promote` | Promote when all debt retired (schema CHECK enforces) |
| `RetractCascade` | Retract belief and descendants on contradiction |
| `IntentOnPromoted` | Record agent action intent |
| `AuditLiveOnNonPromoted` | Verify invariant I-5 |

Wave 3 adds one function (`EnsureBelief`) to the kernel. All other kernel functions are used as-is from M0–M2.

---

## 6. Belief Lifecycle

### Entry

1. `Process` receives a `DerivedBelief`.
2. Call `kernel.EnsureBelief(scenarioID, claim, claimType)` → returns `beliefID`.
3. The kernel handles find-or-create inside one transaction.

### Classification Mapping

| DerivedBelief.Classification | kernel.ClaimType |
|---|---|
| `"derived"` | `kernel.Derived` |
| `"accommodated"` | `kernel.Accommodated` |

### Evidence Attachment

For each item in `DerivedBelief.SupportingEvidence`:
- Call `kernel.AddEvidence(scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)`

Evidence deduplication is by `content_sha256`. The wiring layer performs a read before each `AddEvidence` call to check for duplicates. This creates a narrow TOCTOU window, which is accepted for the MVP because:
- The harm is duplicate evidence rows, not data corruption
- Evidence is append-only — duplicates are harmless
- The single-agent MVP has no concurrent evidence attachment for the same belief
- The kernel's `EnsureBelief` already eliminates the more critical belief dedup TOCTOU

### Debt Retirement

After attaching evidence, retire the debt items the evidence covers using the compile-time mapping table (see §7).

### Promotion

After retiring debt, call `kernel.Promote(beliefID)`. If the schema rejects (debt not fully retired), `ErrPromotionBlocked` is returned — the wiring layer returns nil, and the belief remains `entered` with remaining debt.

### Contradiction

If `DerivedBelief.Contradicts` is non-empty:
1. Do NOT attach evidence or retire debt.
2. Call `kernel.RetractCascade(scenarioID, contradictedBeliefID)`.
3. Return nil.

---

## 7. Evidence Lifecycle

### Attachment

Each item in `DerivedBelief.SupportingEvidence` is attached via `AddEvidence`:

```
AddEvidence(ctx, scenarioID, beliefID,
    evidence.ProvenanceClass,
    evidence.SourceURL,
    evidence.ContentSHA256)
```

### Deduplication

Before each `AddEvidence` call, check if evidence with the same `ContentSHA256` already exists for this belief. If so, skip.

**Dedup query:**
```sql
SELECT count(*) FROM evidence
WHERE belief_id = $1::UUID AND content_sha256 = $2::STRING
```

This read is outside the kernel transaction (TOCTOU accepted — see §6).

### Debt Retirement Mapping

The mapping from evidence to debt items is a compile-time table in `internal/belief/mapping.go`:

```go
// DebtMapping maps evidence source types to the debt items they retire.
// The mechanism is domain-agnostic; the entries are etcd-specific for the MVP.
// This table is the only place domain knowledge enters the wiring layer.
var DebtMapping = map[string][]DebtRule{
    "kev_entry": {
        {Match: "vulnerable to", Items: []string{"needMap"}},
    },
    "release": {
        {Match: "release", Items: []string{"needMap", "needInvariant"}},
    },
    "maintainer_comment": {
        {Match: "fixed|fix released|patch available", Items: []string{"needMap", "needInvariant"}},
        {Match: "tested|confirmed", Items: []string{"needToyCheck", "needNullModel"}},
        {Match: "no regression", Items: []string{"needNullModel", "needObstruction"}},
        {Match: "security review|reviewed by", Items: []string{"needFaithfulnessReview"}},
    },
    "github_pr": {
        {Match: "fix", Items: []string{"needMap", "needInvariant"}},
    },
}

type DebtRule struct {
    Match string   // regex pattern matched against evidence.Assertion
    Items []string // debt items retired by this evidence
}
```

**Retirement is idempotent:** `RetireDebt` on an absent item is a no-op (kernel §4).

### Contradictions

When `DerivedBelief.Contradicts` is non-empty, the evidence contradicts a prior belief. The wiring layer calls `RetractCascade` instead of retiring debt.

---

## 8. Action Intent Lifecycle

### Propose

`intent.Propose` wraps `kernel.IntentOnPromoted`:

```go
func Propose(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error {
    st := kernel.New(db)
    return st.IntentOnPromoted(ctx, scenarioID, beliefID, action)
}
```

### Cancel

Intent cancellation is handled automatically by `RetractCascade` — the kernel cancels live intents before retracting beliefs (I-8). No explicit cancel function is needed in the wiring layer.

### Audit

`intent.Audit` wraps `kernel.AuditLiveOnNonPromoted`:

```go
func Audit(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
    st := kernel.New(db)
    return st.AuditLiveOnNonPromoted(ctx, scenarioID)
}
```

---

## 9. Transaction Boundaries

Each kernel call runs inside `crdb.ExecuteTx` (SERIALIZABLE isolation). The wiring layer makes sequential kernel calls — each is its own transaction.

### Visibility Model

An observer can see intermediate states between kernel calls:

| Step | Observable State |
|---|---|
| After `EnsureBelief` | Belief exists, `status='entered'`, full debt, no evidence |
| After `AddEvidence` | Evidence attached, debt unchanged |
| After `RetireDebt` | Debt partially retired |
| After `Promote` (success) | Belief `status='promoted'`, debt empty |
| After `Promote` (blocked) | Belief `status='entered'`, debt partially retired |

**This is acceptable because:**
1. No agent acts until promotion — the Security Agent only reads `status='promoted'`
2. Promotion requires all debt retired — the schema enforces this
3. An `entered` belief with partial debt is a belief-in-progress, not a correctness violation
4. The single-agent MVP has no concurrent writers for the same belief

### Why not one big transaction?

The kernel's design assumes one operation per `crdb.ExecuteTx`. Wrapping multiple kernel calls in a single application transaction would bypass the kernel's retry logic and violate I-7.

---

## 10. CockroachDB Responsibilities

| Responsibility | Implementation |
|---|---|
| SERIALIZABLE isolation | Default; `crdb.ExecuteTx` manages retry |
| Serialization failure retry | `crdb.ExecuteTx` handles 40001 internally |
| Foreign key enforcement | Composite FK on `action_intent` (I-3) |
| CHECK constraints | `promoted_is_debt_free` (I-1, I-2), `live_requires_promoted` (I-3) |
| ON UPDATE CASCADE | Propagates belief status to intent rows (I-4) |
| Recursive CTE | `RetractCascade` transitive descendant traversal (I-8) |
| ARRAY operations | `array_remove` for debt retirement |

---

## 11. Invariants Enforced by Database

These invariants are enforced by `db/001_schema.sql` CHECK constraints and foreign keys. The wiring layer does NOT duplicate them in application code.

| Invariant | Schema Mechanism | Wave 3 Impact |
|---|---|---|
| **I-1** No promoted belief has non-empty debt | `promoted_is_debt_free` CHECK | `Promote` may fail — wiring layer handles gracefully |
| **I-2** No promoted belief has `final_truth=true` | `promoted_is_debt_free` CHECK | Not triggered in MVP (no final_truth writes) |
| **I-3** No live intent on non-promoted belief | Composite FK + `live_requires_promoted` CHECK | `IntentOnPromoted` may fail — wiring layer returns error |
| **I-4** Live intent blocks retraction | `ON UPDATE CASCADE` + `live_requires_promoted` | `RetractCascade` cancels first (I-8) |
| **I-5** Audit returns 0 | Query assertion | `intent.Audit` wraps this |

---

## 12. Invariants Enforced by Go

| Invariant | Enforcement |
|---|---|
| **I-6** Embeddings are never required | No embedding code in wiring layer |
| **I-7** All writes through `crdb.ExecuteTx` | Kernel enforces; wiring layer uses kernel API only |
| **I-8** `RetractCascade` is one transaction | Kernel enforces; wiring layer calls kernel API |
| Belief dedup by exact claim | `EnsureBelief` — kernel transaction, no TOCTOU |
| Evidence dedup by content_sha256 | Wiring layer read + skip (TOCTOU accepted for MVP) |
| Evidence-to-debt mapping is deterministic | Compile-time table in `mapping.go` |
| `Process` is translation, not orchestration | Each step is a direct kernel call |

---

## 13. Retry Strategy (CockroachDB SERIALIZABLE)

**Mechanism:** `crdb.ExecuteTx` implements the CockroachDB-recommended retry loop:

1. Begin transaction with `SAVEPOINT cockroach_restart`
2. Execute statements
3. `RELEASE SAVEPOINT cockroach_restart`
4. On 40001: rollback to savepoint, retry with exponential backoff
5. After max retries: return error

**Wave 3 impact:** None. The kernel already handles retry. The wiring layer makes sequential kernel calls, each retried independently.

**Verification:** Test B-18 (already passing) proves retry works under injected 40001 errors.

---

## 14. Failure Modes

| Failure | Cause | Handling |
|---|---|---|
| `ErrPromotionBlocked` | Debt not fully retired | Return nil — belief stays `entered` |
| `ErrActionOnUnpromoted` | Belief not promoted | Return error to caller |
| Duplicate evidence | Same content_sha256 for same belief | Skip — evidence is append-only |
| Unknown claim for contradiction | Belief not found | Return error (fail loudly) |
| Cascade blocked by foreign-scenario intent | I-4 violation | Kernel returns 23514 — transaction rolls back atomically |
| Serialization failure | Concurrent writes | `crdb.ExecuteTx` retries automatically |

---

## 15. Unit Test Strategy

### `internal/kernel/kernel_test.go` (additive)

| Test | Asserts |
|---|---|
| `TestEnsureBelief_New` | New claim → belief created with full debt, status='entered' |
| `TestEnsureBelief_Existing` | Same claim → same belief ID returned, no duplicate |
| `TestEnsureBelief_DifferentScenario` | Same claim, different scenario → different belief ID |

### `internal/belief/belief_test.go`

| Test | Asserts |
|---|---|
| `TestProcess_EnterNewBelief` | New claim → EnsureBelief + AddEvidence + RetireDebt called |
| `TestProcess_ExistingBelief` | Same claim → same belief ID, evidence attached |
| `TestProcess_Promotion` | All debt retired → belief promoted |
| `TestProcess_PromotionBlocked` | Debt remaining → ErrPromotionBlocked handled gracefully |
| `TestProcess_Contradiction` | Contradicts non-empty → RetractCascade called |
| `TestProcess_MalformedBelief` | Empty claim → error returned |
| `TestProcess_Determinism` | Same input → same kernel state (100 iterations) |

### `internal/intent/intent_test.go`

| Test | Asserts |
|---|---|
| `TestPropose_LiveIntent` | Promoted belief → intent state 'live' |
| `TestPropose_Unpromoted` | Entered belief → ErrActionOnUnpromoted |
| `TestAudit_Zero` | No live-on-non-promoted → returns 0 |

---

## 16. Integration Test Strategy

### `internal/belief/integration_test.go`

Full pipeline test: raw JSON → normalize → derive → belief → kernel → audit.

| Test | Flow | Asserts |
|---|---|---|
| `TestPipeline_VulnerabilityToAction` | KEV → normalize → derive → ensure → evidence → retire → promote → intent → audit=0 | Full lifecycle |
| `TestPipeline_ContradictionCascade` | KEV → promote → intent → contradiction → retracted + cancelled → audit=0 | Cascade works |
| `TestPipeline_MultipleEvidence` | KEV + release + maintainer → all evidence attached → all debt retired → promoted | Evidence accumulation |
| `TestPipeline_AuditHolds` | After every operation → AuditLiveOnNonPromoted = 0 | I-5 always true |

---

## 17. Specification Traceability

| Wave 3 Output | Spec Section | Kernel Function |
|---|---|---|
| Ensure belief exists | §5.1, §6.4 | `EnsureBelief` |
| Attach supporting evidence | §4.3, §5.1 | `AddEvidence` |
| Retire debt (KEV) | §5.1, §6.3 | `RetireDebt` |
| Retire debt (fix release) | §5.2, §6.3 | `RetireDebt` |
| Retire debt (maintainer) | §5.3, §6.3 | `RetireDebt` |
| Promote when debt-free | §5.5, §6.1 | `Promote` |
| Retract on contradiction | §5.4, §6.2 | `RetractCascade` |
| Propose intent | §7.3 | `IntentOnPromoted` |
| Audit invariant | §5, I-5 | `AuditLiveOnNonPromoted` |

---

## 18. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Debt retirement mapping may be incomplete for edge cases | Medium | Test each evidence type independently; mapping is extensible |
| R2 | Exact claim match dedup may miss semantically similar claims | Low | Acceptable for MVP; embeddings deferred to Phase 2 |
| R3 | `ErrPromotionBlocked` on every new belief (debt never fully retired) | Medium | Map evidence to debt items correctly; test full lifecycle |
| R4 | Foreign-scenario intent blocks cascade (B-24 behavior) | Low | Kernel handles atomically; integration test covers this |
| R5 | Evidence dedup TOCTOU (accepted) | Low | Harm is duplicate rows, not corruption; single-agent MVP |

---

## 19. Receipts That Prove Correctness

| Receipt | Proves |
|---|---|
| `go build ./...` exits 0 | All packages compile |
| `go vet ./...` exits 0 | No vet errors |
| `go test ./internal/kernel/... -v` exits 0 | Kernel + EnsureBelief correct |
| `go test ./internal/belief/... -v` exits 0 | Belief wiring is correct |
| `go test ./internal/intent/... -v` exits 0 | Intent wiring is correct |
| `go test ./internal/derive/... -v` exits 0 | Wave 2 still passes |
| `go test ./internal/normalize/... -v` exits 0 | Wave 1 still passes |
| `TestEnsureBelief_Existing` | Dedup is transactional (no TOCTOU) |
| `TestPipeline_VulnerabilityToAction` | Full lifecycle: ensure → evidence → retire → promote → intent → audit=0 |
| `TestPipeline_ContradictionCascade` | Contradiction triggers cascade; intents cancelled; audit=0 |
| `TestPipeline_AuditHolds` | I-5 holds after every operation |
| `TestProcess_Determinism` | Same input → same kernel state (100 iterations) |
| E2E: raw JSON → normalize → derive → kernel → audit=0 | No corruption across the full pipeline |

---

## 20. Estimated Implementation Order

| Step | Task | Est. Time |
|---|---|---|
| 1 | Add `EnsureBelief` to kernel (kernel.go + sql.go + test) | 1 hour |
| 2 | Create `internal/belief/mapping.go` — compile-time debt mapping | 30 min |
| 3 | Create `internal/belief/belief.go` — thin `Process` translation | 1 hour |
| 4 | Create `internal/intent/intent.go` — `Propose`, `Audit` | 30 min |
| 5 | Create `internal/belief/belief_test.go` — 7 unit tests | 1 hour |
| 6 | Create `internal/intent/intent_test.go` — 3 unit tests | 30 min |
| 7 | Create `internal/belief/integration_test.go` — 4 integration tests | 1 hour |
| 8 | Run verification: build, vet, test | 15 min |
| 9 | Write `WAVE3_IMPLEMENTATION.md` — receipts | 15 min |
| | **Total** | **~6 hours** |
