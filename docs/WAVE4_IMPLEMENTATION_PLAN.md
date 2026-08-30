# WAVE4_IMPLEMENTATION_PLAN.md

**Status:** Revised — addresses Technical Lead review (P0: intent idempotency, P0: contradiction semantics, P1: deterministic transcript).
**Frozen waves:** Wave 1 (normalize), Wave 2 (derive), Wave 3 (belief, intent, kernel). No modifications.
**Schema:** `db/001_schema.sql` unchanged. No new tables, no new columns.

---

## 1. Objective

Build the **minimum integration layer** that connects one realistic evidence feed through Waves 1→2→3 to produce actionable / non-actionable beliefs in a live CockroachDB database.

The output is a single `cmd/solvent` command and one integration test proving the full pipeline works end-to-end.

---

## 2. End-to-End Flow

```
Evidence fixtures (testdata/*.json)
  │
  ▼
normalize.Normalize(raw, sourceType)          ← Wave 1 (frozen)
  │
  ▼
derive.Derive(normalizedEvidence)              ← Wave 2 (frozen)
  │
  ▼
belief.Process(ctx, db, scenarioID, derived)   ← Wave 3 (frozen)
  │  ├── EnsureBelief (find-or-create)
  │  ├── AddEvidence (deduplicated)
  │  ├── RetireDebt (mapping rules)
  │  └── Promote (may block)
  │
  ▼
pipeline.ProposeIfNew(ctx, db, ...)            ← Wave 4 (new — idempotent intent)
  │  ├── SELECT count(*) WHERE belief_id=X AND action=Y AND state='live'
  │  └── intent.Propose (only if count = 0)
  │
  ▼
intent.Audit(ctx, db, scenarioID)              ← Wave 3 (frozen)
  │
  ▼
Transcript (stdout) — receipts proving pipeline correctness
```

Contradictions bypass the kernel entirely:

```
contradiction evidence
  │
  ▼
derive.Derive → DerivedBelief with non-empty Contradicts
  │
  ▼
pipeline captures in-memory Result{Contradiction: true, BeliefID: ""}
  │
  ▼
slog.Warn with source_url
  │
  ▼
NO ledger mutation (belief.Process returns nil, no DB write)
```

---

## 3. Input Data Source

Existing test fixtures in two directories:

| Directory | Fixtures | Source types |
|---|---|---|
| `internal/normalize/testdata/` | 7 JSON files | github_issue, github_pr, maintainer_comment, release, kev_entry |
| `internal/derive/testdata/` | 9 JSON files | kev_entry, release, maintainer_comment (multiple variants), github_pr |

The pipeline reads **both** directories. Evidence from `normalize/testdata/` proves normalization works; evidence from `derive/testdata/` proves derivation works with richer payloads (e.g., maintainer keywords, contradiction signals).

---

## 4. Fixture Strategy

**No new fixtures.** All evidence is existing test data. This is deliberate:

- Fixtures are already validated by Waves 1–2 tests
- No fabrication risk (no invented CVE IDs, URLs, or timestamps)
- Deterministic: same input always produces same output
- Replay-safe: `belief.Process` is idempotent (EnsureBelief dedup, content_sha256 dedup)

Fixture processing order matters for debt retirement. The pipeline processes fixtures in a fixed deterministic order:

1. `kev_entry.json` → enters belief with `needMap` retired
2. `release_with_fix.json` → adds evidence, retires `needMap` + `needInvariant`
3. `maintainer_fixes.json` → retires `needMap` + `needInvariant`
4. `maintainer_no_regression.json` → retires `needToyCheck` + `needNullModel` + `needObstruction`
5. `maintainer_reviewed.json` → retires `needFaithfulnessReview`
6. `release_confirmed.json` → adds evidence (already deduplicated)
7. `contradicting_evidence.json` → triggers contradiction path (slog.Warn, no ledger mutation)
8. `non_maintainer_comment.json` → no belief emitted (non-maintainer, filtered by derive)
9. `unmerged_pr.json` → no belief emitted (unmerged, filtered by derive)

After steps 1–5, all 6 debt items are retired → belief is promotable.

---

## 5. What Is Real vs Simulated

| Aspect | Status |
|---|---|
| CockroachDB connection | **Real** — live cluster via DSN |
| Schema application | **Real** — `db/001_schema.sql` applied by test harness |
| Normalization | **Real** — Wave 1 code paths execute |
| Derivation | **Real** — Wave 2 code paths execute |
| Belief lifecycle | **Real** — kernel operations hit the database |
| Debt retirement | **Real** — mapping rules fire, debt array shrinks |
| Promotion gate | **Real** — `promoted_is_debt_free` CHECK enforced by CockroachDB |
| Intent gate | **Real** — composite FK enforced by CockroachDB |
| Audit | **Real** — `AuditLiveOnNonPromoted` query returns 0 |
| Evidence feed | **Simulated** — local JSON files, not a live GitHub/CISA API |
| Action execution | **Simulated** — intent is proposed but not "executed" (MVP does not model post-execution) |

---

## 6. Required Schema Changes

**None.** The frozen schema supports all Wave 4 operations. The `belief.embedding` column is nullable and not used; the ledger functions correctly with NULL embeddings (invariant I-6).

---

## 7. Required CockroachDB Functionality

All already proven at M0/M1/M2:

- `SERIALIZABLE` isolation (default, used by `crdb.ExecuteTx`)
- `TEXT[]` array operations (`array_remove`)
- Composite FK with `ON UPDATE CASCADE`
- CHECK constraints (`promoted_is_debt_free`, `live_requires_promoted`)
- CTE queries (`sqlEnsureBelief`, `descendantsCTE`)

No new CockroachDB features required.

---

## 8. Package / File Layout

```
cmd/solvent/main.go              — CLI entrypoint (orchestrator)
internal/pipeline/pipeline.go    — normalize→derive→process chain
internal/pipeline/pipeline_test.go — end-to-end integration test
```

Three new files. No modifications to existing files.

---

## 9. Public APIs

### `internal/pipeline`

```go
// Result captures the outcome of processing one evidence item through the pipeline.
type Result struct {
    Normalized   normalize.NormalizedEvidence
    Beliefs      []derive.DerivedBelief
    BeliefID     string // empty if contradiction or no belief emitted
    Promoted     bool
    DebtItems    []string // remaining debt after processing
    IntentState  string   // "live", "cancelled", or "" (no intent proposed)
    Contradiction bool    // true if this result is a contradiction (no ledger mutation)
}

// ProcessEvidence chains normalize → derive → belief.Process for one raw evidence item.
//
// Normal path:
//   normalize → derive → belief.Process (EnsureBelief, AddEvidence, RetireDebt, Promote)
//   → Result with BeliefID set, Contradiction=false
//
// Contradiction path (DerivedBelief.Contradicts non-empty):
//   → Result with BeliefID="", Contradiction=true
//   → slog.Warn emitted with source_url
//   → NO ledger mutation (belief.Process returns nil without DB writes)
//
// The caller should NOT call ProposeIfNew for contradiction results.
func ProcessEvidence(ctx context.Context, db *sql.DB, scenarioID string, raw []byte, sourceType string) ([]Result, error)

// ProposeIfNew records a live action intent on a promoted belief only if no live intent
// already exists for the same belief/action pair. Idempotent: replaying the same evidence
// does not create duplicate intents.
//
// Check: SELECT count(*) FROM action_intent
//   WHERE scenario_id=$1 AND belief_id=$2 AND action=$3 AND state='live'
// If count > 0: skip, return nil.
// If count = 0: call intent.Propose.
func ProposeIfNew(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error

// Run processes a directory of evidence fixtures in deterministic order.
// Returns results in processing order.
func Run(ctx context.Context, db *sql.DB, scenarioID string, fixtureDir string) ([]Result, error)
```

### `cmd/solvent`

```
Usage: solvent [flags]
  -dsn string       CockroachDB connection string (env: FABLE_DSN)
  -schema string    Path to frozen DDL (default: db/001_schema.sql)
  -scenario string  Scenario ID (required for deterministic replay)
  -reset            Drop and recreate the database first
  -out string       Transcript output path (default: stdout)
```

---

## 10. Error Handling

Following the established pattern (kernel owns errors, higher layers propagate or classify):

- `normalize.Normalize` returns `(NormalizedEvidence, error)` → pipeline wraps with source context
- `derive.Derive` returns `[]DerivedBelief` (never errors; returns nil for unknown source types)
- `belief.Process` returns `error` → `ErrPromotionBlocked` is acceptable (belief not yet promotable), other errors propagate
- `ProposeIfNew` returns `error` → `ErrActionOnUnpromoted` is expected for non-promoted beliefs, other errors propagate
- `intent.Audit` returns `(int, error)` → error propagates

The `cmd/solvent` entrypoint prints the first non-acceptable error and exits with code 1.

---

## 11. Idempotency / Replay Behavior

**All pipeline stages are idempotent, including intent proposal.**

- **normalize:** Pure function, deterministic, no side effects
- **derive:** Pure function, deterministic, no side effects
- **belief.Process:** `EnsureBelief` is find-or-create (CTE), `AddEvidence` deduplicates by `content_sha256`, `RetireDebt` is `array_remove` (idempotent), `Promote` is a no-op if already promoted
- **ProposeIfNew:** Checks for existing live intent before proposing. If a live intent already exists for the same belief/action pair, skips the INSERT. Replaying the same evidence does not create duplicate intents.

The integration test replays the full pipeline 3 times against the same scenario and asserts **identical database state** after each replay — including identical intent count.

Contradictions are also idempotent: `belief.Process` logs `slog.Warn` and returns `nil` without any DB write. Replaying a contradiction produces the same log warning and the same empty database state.

---

## 12. Integration Test Strategy

### Test: `TestEndToEnd_ProcessRealFixtures`

Uses fixed scenario ID: `"55555555-0000-0000-0000-000000000001"`

```
1. Reset test database, apply schema
2. Process all fixtures through pipeline (deterministic order)
3. Assert deterministic fields:
   a. Exactly 1 belief exists with claim containing "CVE-2024-24790"
   b. Belief status is "promoted"
   c. Belief debt length is 0 (all 6 items retired)
   d. Evidence count ≥ 5 (kev_entry + release + 3 maintainer comments)
   e. AuditLiveOnNonPromoted = 0
   f. Exactly 1 live intent exists
4. Record: belief_status, debt_len, evidence_count, audit_count, intent_count
5. Replay full pipeline (same scenario, same fixtures)
6. Assert all recorded values are identical (idempotency)
7. Assert AuditLiveOnNonPromoted still = 0
```

### Test: `TestEndToEnd_PartialDebtBlocksPromotion`

Uses fixed scenario ID: `"55555555-0000-0000-0000-000000000002"`

```
1. Process only kev_entry fixture
2. Assert belief status is "entered" (debt not fully retired)
3. Assert ProposeIfNew returns ErrActionOnUnpromoted
4. Assert AuditLiveOnNonPromoted = 0
```

### Test: `TestEndToEnd_ContradictionNoLedgerMutation`

Uses fixed scenario ID: `"55555555-0000-0000-0000-000000000003"`

```
1. Process contradicting_evidence.json fixture
2. Assert Result.Contradiction == true
3. Assert Result.BeliefID == ""
4. Assert 0 beliefs exist in database for this scenario
5. Assert 0 evidence rows exist for this scenario
6. Assert AuditLiveOnNonPromoted = 0
```

### Test: `TestEndToEnd_DeterministicResults`

Uses fixed scenario ID: `"55555555-0000-0000-0000-000000000004"`

```
1. Process fixtures twice into separate scenarios
2. Compare deterministic fields only (not UUIDs or timestamps):
   - Claim text
   - Belief status
   - Debt item count
   - Evidence count
   - Audit count
   - Intent count
   - Contradiction flag
3. Assert all fields match
```

---

## 13. Deterministic Transcript Criteria

The CLI transcript contains only deterministic fields:

| Field | Deterministic? | Notes |
|---|---|---|
| claim text | Yes | Derived from fixture content |
| belief status | Yes | `entered` or `promoted` |
| debt item count | Yes | 0 or 6 |
| evidence count | Yes | Deterministic fixture processing |
| audit count | Yes | Always 0 |
| intent state | Yes | `live` or `cancelled` |
| contradiction flag | Yes | `true` or `false` |
| source_url | Yes | Fixture content |
| scenario_id | **No** — must be provided explicitly | CLI requires `-scenario` flag |
| belief UUID | **No** — generated by database | Not included in transcript |
| evidence UUID | **No** — generated by database | Not included in transcript |
| content_sha256 | **No** — hash of normalized content | Not included in transcript |
| timestamps | **No** — `now()` at insert time | Not included in transcript |

The deterministic test (§12 Test 4) compares only the deterministic fields listed above. The CLI transcript renders only these fields.

---

## 14. Demo-Critical Acceptance Criteria

| # | Criterion | How verified |
|---|---|---|
| D-1 | `cmd/solvent` compiles and runs | `go build ./cmd/solvent && ./solvent -dsn $FABLE_DSN -scenario X -reset` |
| D-2 | Pipeline processes all fixtures without error | Exit code 0, no stderr errors |
| D-3 | At least 1 belief is promoted | Transcript shows `status=promoted` |
| D-4 | AuditLiveOnNonPromoted = 0 | Transcript shows `audit=0` |
| D-5 | At least 1 intent is live | Transcript shows `intent=live` |
| D-6 | Contradiction fixture produces slog.Warn | Transcript logs warning with source_url |
| D-7 | Replay produces identical deterministic state | Run twice with same -scenario, compare deterministic fields |
| D-8 | Integration tests pass | `go test ./internal/pipeline/ -v` |

---

## 15. Explicit Non-Scope

The following are **not** part of Wave 4. They come after the minimum end-to-end system is proven:

- bchat integration
- Polished UI / web dashboard
- Graph visualization
- AWS deployment
- Video recording
- Devpost submission
- Taskfile polish
- Live GitHub/CISA API integration
- Multi-domain evidence feeds
- Embedding-based dedup (I-6: NULL embeddings are fine)
- Action execution (intent proposes but does not "do")
- Multiple scenarios or multi-tenant support

---

## 16. Estimated Implementation Effort

| File | Lines (est.) | Complexity |
|---|---|---|
| `internal/pipeline/pipeline.go` | 70–90 | Low — sequential chain + ProposeIfNew helper |
| `internal/pipeline/pipeline_test.go` | 140–180 | Medium — 4 test cases, DB assertions, fixed scenario IDs |
| `cmd/solvent/main.go` | 80–120 | Low — flag parsing, pipeline call, transcript rendering |
| **Total** | 290–390 | Low |

Estimated implementation time: **2–3 hours** for a developer familiar with the codebase.

---

## 17. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Fixture ordering may not retire all 6 debt items | Belief stays `entered`, never promoted | Verify fixture assertions match DebtMapping rules before coding; add a 6th maintainer comment if needed |
| Test database lock contention | Integration tests block on shared `fable_test` | Use existing `AcquireResetLock`/`ReleaseResetLock` pattern |
| `go test ./...` parallel race | Intermittent test failures | Run serially; document in README |

---

## 18. Receipts Proving the MVP Works

After implementation, the following receipts must exist:

1. **`go build ./cmd/solvent`** — exit code 0
2. **`go vet ./...`** — exit code 0
3. **`go test ./internal/pipeline/ -v`** — all 4 tests pass
4. **`./solvent -dsn $FABLE_DSN -scenario X -reset`** — transcript showing promoted belief, audit=0, intent=live
5. **Replay idempotency** — second run with same scenario produces identical deterministic state
6. **Contradiction path** — transcript includes slog.Warn for `contradicting_evidence.json`, 0 beliefs in DB for that scenario

---

## 19. Implementation Sequence

1. Write `internal/pipeline/pipeline.go` (ProcessEvidence, ProposeIfNew, Run)
2. Write `internal/pipeline/pipeline_test.go` (4 tests with fixed scenario IDs)
3. Run tests, verify green
4. Write `cmd/solvent/main.go` (orchestrator + transcript)
5. Build and run against live cluster
6. Produce receipts
7. Stop — await Technical Lead review
