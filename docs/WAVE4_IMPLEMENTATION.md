# WAVE4_IMPLEMENTATION.md

**Status:** Complete. All tests pass on CockroachDB v26.2.0.
**Scope:** Wave 4 — Minimum integration layer connecting evidence fixtures through Waves 1→2→3.
**Frozen waves:** Wave 1 (normalize), Wave 2 (derive), Wave 3 (belief, intent, kernel). No modifications.
**Schema:** `db/001_schema.sql` unchanged.

---

## 1. Files Created

| File | Lines | Purpose |
|---|---|---|
| `internal/pipeline/pipeline.go` | 428 | normalize→derive→belief.Process→ProposeIfNew chain with safe CVE aggregation |
| `internal/pipeline/pipeline_test.go` | 679 | 9 end-to-end integration tests |
| `cmd/solvent/main.go` | 148 | CLI orchestrator with deterministic transcript |

**Total:** 3 files, 1255 lines. No modifications to frozen Waves 1–3.

---

## 2. Implementation Summary

### Pipeline Architecture

```
Phase 1: Derive (no DB writes)
  normalize.Normalize(raw, sourceType) → NormalizedEvidence
  derive.Derive(norm) → []DerivedBelief

Phase 2: Aggregate
  Merge all non-contradiction evidence into the primary vulnerability claim
  (first CVE-bearing claim). Contradictions collected separately.

Phase 3: Ledger
  belief.Process(ctx, db, scenarioID, mergedBelief)
    → EnsureBelief (find-or-create)
    → AddEvidence (deduplicated by content_sha256)
    → RetireDebt (mapping rules)
    → Promote (may block)

Phase 3.5: Intent
  ProposeIfNew(ctx, db, scenarioID, beliefID, "upgrade")
    → SELECT count(*) WHERE belief_id=X AND action=Y AND state='live'
    → intent.Propose (only if count = 0)

Phase 4: Contradictions
  Result{Contradiction: true, BeliefID: ""}
  slog.Warn with source_url
  NO ledger mutation
```

### Evidence Aggregation

The derive layer produces different claims for different evidence types:
- `kev_entry` → "etcd vX is vulnerable to CVE-XXXX"
- `release` → "fix available for etcd vX"
- `maintainer_comment` → "fix for etcd-io/etcd confirmed by maintainer"
- etc.

The pipeline's `aggregateDeriveResults` merges non-CVE supporting evidence into the first CVE-bearing claim (the vulnerability belief). Each distinct CVE-bearing claim remains independent — different CVEs produce separate beliefs. Contradictions are collected separately. This ensures correct belief isolation while consolidating supporting evidence, matching the plan's expected output.

### Intent Idempotency

`ProposeIfNew` checks for an existing live intent before proposing:

```sql
SELECT count(*) FROM action_intent
WHERE scenario_id=$1 AND belief_id=$2 AND action=$3 AND state='live'
```

If count > 0, skips the INSERT. Replaying the same evidence does not create duplicate intents.

---

## 3. Tests

| Test | Scenario ID | Purpose | Result |
|---|---|---|---|
| `TestEndToEnd_ProcessRealFixtures` | `55555555-0000-0000-0000-000000000001` | Full pipeline: promote, audit=0, intent=live, idempotent replay | PASS |
| `TestEndToEnd_PartialDebtBlocksPromotion` | `55555555-0000-0000-0000-000000000002` | Partial debt → entered, intent blocked | PASS |
| `TestEndToEnd_ContradictionNoLedgerMutation` | `55555555-0000-0000-0000-000000000003` | Contradiction → 0 beliefs, 0 evidence | PASS |
| `TestEndToEnd_DeterministicResults` | `55555555-0000-0000-0000-000000000004` / `...0005` | Deterministic fields identical across scenarios | PASS |
| `TestEndToEnd_ProposeIfNew_Idempotent` | `55555555-0000-0000-0000-000000000006` | 3x propose → 1 intent (idempotent) | PASS |
| `TestEndToEnd_RunContradictionWarning` | `55555555-0000-0000-0000-000000000007` | Run() emits slog.Warn with source_url for contradictions | PASS |
| `TestEndToEnd_TwoCVEIndependent` | `55555555-0000-0000-0000-000000000008` | Two different CVEs produce independent beliefs | PASS |
| `TestEndToEnd_ResultProvenance` | `55555555-0000-0000-0000-000000000009` | Run() populates Result.Normalized with source info | PASS |
| `TestEndToEnd_DeterministicOrderWithMultipleClaims` | `...000A` / `...000B` | Result order deterministic with multiple claims | PASS |

**9/9 tests pass.**

---

## 4. Verification Receipts

| Command | Exit Code | Proves |
|---|---|---|
| `go build ./...` | 0 | All packages compile |
| `go vet ./...` | 0 | No vet errors |
| `gofmt -l internal/pipeline/ cmd/solvent/` | (empty) | All files formatted |
| `go test ./internal/pipeline/ -count=1` | 0 | 9/9 pipeline tests pass |
| `go test ./internal/... -count=1` (serial) | 0 | All 84 tests pass across all packages |

### Full Test Results

```
ok  internal/belief     ~1.9s   (18 tests)
ok  internal/derive     ~0.0s   (16 tests)
ok  internal/intent     ~1.2s   (3 tests)
ok  internal/kernel     ~1.7s   (27 tests)
ok  internal/normalize  ~0.0s   (11 tests)
ok  internal/pipeline   ~1.4s   (9 tests)
```

**Total: 84 tests, all PASS.**

---

## 5. Deterministic Replay Result

Replaying the full pipeline against the same scenario produces identical deterministic state:

| Field | Run 1 | Run 2 | Match |
|---|---|---|---|
| Belief count | 1 | 1 | ✓ |
| Belief status | promoted | promoted | ✓ |
| Debt items | 0 | 0 | ✓ |
| Evidence count | 5 | 5 | ✓ |
| Audit count | 0 | 0 | ✓ |
| Intent count | 1 | 1 | ✓ |

---

## 6. Known Concurrency Limitation

**Intent proposal is replay-idempotent under sequential execution; concurrent duplicate-intent prevention is deferred.**

The `ProposeIfNew` function uses a SELECT-then-INSERT pattern:

```sql
-- Check
SELECT count(*) FROM action_intent
WHERE scenario_id=$1 AND belief_id=$2 AND action=$3 AND state='live'

-- Insert (only if count = 0)
INSERT INTO action_intent (scenario_id, belief_id, action) VALUES ($1, $2, $3)
```

This is NOT a database-enforced concurrent uniqueness guarantee. Under concurrent execution, two goroutines could both see count=0 and both insert, creating duplicate live intents. This is acceptable for the MVP's sequential demo path. Concurrent duplicate-intent prevention would require a unique index or serializable transaction, which is deferred.

---

## 7. Known Limitations

1. **Evidence aggregation is CVE-aware.** The `aggregateDeriveResults` function merges non-CVE supporting evidence into the first CVE-bearing claim while keeping distinct CVE-bearing claims independent. A multi-domain deployment would need a different aggregation strategy.

2. **No live evidence feed.** The pipeline reads local JSON fixtures, not a live GitHub/CISA API. A real-data snapshot will be added after the pipeline works.

3. **No graph visualization.** The dependency graph (Evidence → Belief → Action) is not rendered. Deferred.

4. **No bchat integration.** Deferred.

5. **No embedding column by design (I-6).** The schema has no embedding column. The ledger operates correctly with zero vectors. B-14 test not updated.

---

## 8. Non-Scope (Not Implemented)

Per the approved plan §15:

- bchat integration
- Polished UI / web dashboard
- Graph visualization
- AWS deployment
- Video recording / Devpost
- Live GitHub/CISA API integration
- Multi-domain evidence feeds
- Embedding-based dedup
- Action execution
- Multiple scenarios / multi-tenant support
