# WAVE4_IMPLEMENTATION_PLAN.md

**Status:** Draft — awaiting Technical Lead review.
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
intent.Propose(ctx, db, scenarioID, beliefID, action)  ← Wave 3 (frozen)
  │
  ▼
intent.Audit(ctx, db, scenarioID)                       ← Wave 3 (frozen)
  │
  ▼
Transcript (stdout) — receipts proving pipeline correctness
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
7. `contradicting_evidence.json` → triggers contradiction path (slog.Warn, retraction deferred)
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
    Normalized  normalize.NormalizedEvidence
    Beliefs     []derive.DerivedBelief
    BeliefID    string // empty if contradiction or no belief emitted
    Promoted    bool
    DebtItems   []string // remaining debt after processing
    IntentState string   // "live", "cancelled", or "" (no intent proposed)
}

// ProcessEvidence chains normalize → derive → belief.Process for one raw evidence item.
// Returns the result for each derived belief. Contradictions produce a Result with
// BeliefID="" and Beliefs containing the contradiction.
func ProcessEvidence(ctx context.Context, db *sql.DB, scenarioID string, raw []byte, sourceType string) ([]Result, error)

// Run processes a directory of evidence fixtures in deterministic order.
// Returns results in processing order.
func Run(ctx context.Context, db *sql.DB, scenarioID string, fixtureDir string) ([]Result, error)
```

### `cmd/solvent`

```
Usage: solvent [flags]
  -dsn string       CockroachDB connection string (env: FABLE_DSN)
  -schema string    Path to frozen DDL (default: db/001_schema.sql)
  -scenario string  Scenario ID (default: auto-generated UUID)
  -reset            Drop and recreate the database first
  -out string       Transcript output path (default: stdout)
```

---

## 10. Error Handling

Following the established pattern (kernel owns errors, higher layers propagate or classify):

- `normalize.Normalize` returns `(NormalizedEvidence, error)` → pipeline wraps with source context
- `derive.Derive` returns `[]DerivedBelief` (never errors; returns nil for unknown source types)
- `belief.Process` returns `error` → `ErrPromotionBlocked` is acceptable (belief not yet promotable), other errors propagate
- `intent.Propose` returns `error` → `ErrActionOnUnpromoted` is expected for non-promoted beliefs, other errors propagate
- `intent.Audit` returns `(int, error)` → error propagates

The `cmd/solvent` entrypoint prints the first non-acceptable error and exits with code 1.

---

## 11. Idempotency / Replay Behavior

All pipeline stages are idempotent:

- **normalize:** Pure function, deterministic, no side effects
- **derive:** Pure function, deterministic, no side effects
- **belief.Process:** `EnsureBelief` is find-or-create (CTE), `AddEvidence` deduplicates by `content_sha256`, `RetireDebt` is `array_remove` (idempotent), `Promote` is a no-op if already promoted
- **intent.Propose:** Creates a new intent row each call (not idempotent — the pipeline should only call this once per promoted belief)

The integration test replays the full pipeline 3 times against the same scenario and asserts identical database state after each replay (except intent count, which grows by 1 per replay). The pipeline should check for existing intents before proposing new ones.

---

## 12. Integration Test Strategy

### Test: `TestEndToEnd_ProcessRealFixtures`

```
1. Reset test database, apply schema
2. Generate scenario ID
3. Process all fixtures through pipeline
4. Assert:
   a. Exactly 1 belief exists with claim containing "CVE-2024-24790"
   b. Belief status is "promoted"
   c. Belief debt is empty (all 6 items retired)
   d. Evidence count ≥ 5 (kev_entry + release + 3 maintainer comments)
   e. AuditLiveOnNonPromoted = 0
   f. At least 1 live intent exists
5. Replay full pipeline
6. Assert identical state (idempotency)
7. Assert AuditLiveOnNonPromoted still = 0
```

### Test: `TestEndToEnd_PartialDebtBlocksPromotion`

```
1. Process only kev_entry fixture
2. Assert belief status is "entered" (debt not fully retired)
3. Assert intent.Propose returns ErrActionOnUnpromoted
4. Assert AuditLiveOnNonPromoted = 0
```

### Test: `TestEndToEnd_DeterministicOutput`

```
1. Process fixtures twice, capture Results
2. Assert results are byte-identical (determinism)
```

---

## 13. Demo-Critical Acceptance Criteria

| # | Criterion | How verified |
|---|---|---|
| D-1 | `cmd/solvent` compiles and runs | `go build ./cmd/solvent && ./solvent -dsn $FABLE_DSN -reset` |
| D-2 | Pipeline processes all fixtures without error | Exit code 0, no stderr errors |
| D-3 | At least 1 belief is promoted | Transcript shows `status=promoted` |
| D-4 | AuditLiveOnNonPromoted = 0 | Transcript shows `audit=0` |
| D-5 | At least 1 intent is live | Transcript shows `intent=live` |
| D-6 | Contradiction fixture produces slog.Warn | Transcript logs warning with source_url |
| D-7 | Full pipeline is deterministic | Run twice, diff transcript — byte-identical |
| D-8 | Integration tests pass | `go test ./internal/pipeline/ -v` |

---

## 14. Explicit Non-Scope

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

## 15. Estimated Implementation Effort

| File | Lines (est.) | Complexity |
|---|---|---|
| `internal/pipeline/pipeline.go` | 50–70 | Low — sequential chain of existing functions |
| `internal/pipeline/pipeline_test.go` | 100–140 | Medium — 3 test cases, DB assertions |
| `cmd/solvent/main.go` | 80–120 | Low — flag parsing, pipeline call, transcript rendering |
| **Total** | 230–330 | Low |

Estimated implementation time: **2–3 hours** for a developer familiar with the codebase.

---

## 16. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Fixture ordering may not retire all 6 debt items | Belief stays `entered`, never promoted | Verify fixture assertions match DebtMapping rules before coding; add a 6th maintainer comment if needed |
| `intent.Propose` called multiple times on replay | Multiple live intents on same belief | Check for existing live intent before proposing |
| Test database lock contention | Integration tests block on shared `fable_test` | Use existing `AcquireResetLock`/`ReleaseResetLock` pattern |
| `go test ./...` parallel race | Intermittent test failures | Run serially; document in README |

---

## 17. Receipts Proving the MVP Works

After implementation, the following receipts must exist:

1. **`go build ./cmd/solvent`** — exit code 0
2. **`go vet ./...`** — exit code 0
3. **`go test ./internal/pipeline/ -v`** — all 3 tests pass
4. **`./solvent -dsn $FABLE_DSN -reset`** — transcript showing promoted belief, audit=0, intent=live
5. **Replay idempotency** — second run produces identical transcript (except intent count)
6. **Contradiction path** — transcript includes slog.Warn for `contradicting_evidence.json`

---

## 18. Implementation Sequence

1. Write `internal/pipeline/pipeline.go` (ProcessEvidence, Run)
2. Write `internal/pipeline/pipeline_test.go` (3 tests)
3. Run tests, verify green
4. Write `cmd/solvent/main.go` (orchestrator + transcript)
5. Build and run against live cluster
6. Produce receipts
7. Stop — await Technical Lead review
