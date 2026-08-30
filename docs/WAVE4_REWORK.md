# WAVE4_REWORK.md

**Status:** Complete — all rework items applied and verified.
**Scope:** `internal/pipeline/pipeline.go`, `internal/pipeline/pipeline_test.go`, `cmd/solvent/main.go`, documentation.
**Authority:** `WAVE4_ADVERSARIAL_REVIEW.md` (F1–F4, P4, P5), TL rework reviews (`wave4_rework.md`, `wave4_rework2.md`).
**Method:** Code audit against adversarial review findings; verified against live CockroachDB v26.2.0.

---

## Verdict: ALL REWORK ITEMS RESOLVED

Six rework items plus one additional cleanup were identified by the adversarial review. All fixes have been applied to Wave 4 files. No schema changes, no frozen wave modifications.

---

## Finding Status

### F1 [P1] — Contradiction path is silent in `Run()` — FIXED

**Fix applied:** `Run()` Phase 4 now emits `slog.Warn` with `source_url` and `source_type` for each contradiction before appending the result.
**Verification:** `TestEndToEnd_RunContradictionWarning` captures slog output and confirms the warning is emitted with correct source info.

### F2 [P1] — `Result.Normalized` empty in `Run()` — FIXED

**Fix applied:** `Run()` now populates `Result.Normalized` from the `mergedClaim.norm` (first supporting evidence's NormalizedEvidence). The `aggregateDeriveResults` function was modified to return `[]mergedClaim` which pairs each DerivedBelief with its provenance.
**Verification:** `TestEndToEnd_ResultProvenance` confirms non-empty `SourceType` and `SourceURL` on all results including contradictions.

### F4 [P1] — Evidence aggregation merges independent CVE claims — FIXED

**Fix applied:** `aggregateDeriveResults` now uses CVE-aware aggregation. Each distinct CVE-bearing claim is treated as its own primary. Different CVEs produce separate beliefs. Non-CVE evidence is merged into the first CVE-bearing claim that matches its CVE identifier.
**Verification:** `TestEndToEnd_TwoCVEIndependent` creates two different CVE fixtures, runs the pipeline, and confirms 2 independent beliefs with 2 evidence rows (1 per CVE, no cross-contamination).

### F3 [P2] — Documentation inconsistency on embedding column — FIXED

**Fix applied:** `WAVE4_IMPLEMENTATION.md` now states: "The schema has no embedding column by design (I-6). The ledger operates correctly with zero vectors."

### P4 [P2] — Test count discrepancy — FIXED

**Fix applied:** All receipt tables updated to reflect actual counts.

### P5 [P3] — Result-order determinism — FIXED

**Fix applied:** `aggregateDeriveResults` now sorts merged claims by claim text before return. Map iteration order no longer affects result ordering.
**Verification:** `TestEndToEnd_DeterministicOrderWithMultipleClaims` runs two scenarios with two CVEs and confirms identical result order.

### Additional — Redundant intent proposal in main.go — FIXED

**Fix applied:** Removed the redundant intent-proposal loop from `cmd/solvent/main.go`. `Run()` already performs `ProposeIfNew` and records `IntentState`. The CLI consumes that result directly.

---

## Files Changed

| File | Change |
|---|---|
| `internal/pipeline/pipeline.go` | F1 (slog.Warn), F2 (Result.Normalized), F4 (CVE-aware aggregation), P5 (sorted output), `mergedClaim` type, `firstNorm` helper, `kev_entry_v2.json` source mapping |
| `internal/pipeline/pipeline_test.go` | Added 4 new tests: `RunContradictionWarning`, `TwoCVEIndependent`, `ResultProvenance`, `DeterministicOrderWithMultipleClaims` |
| `cmd/solvent/main.go` | Removed redundant intent-proposal loop |
| `WAVE4_IMPLEMENTATION.md` | Updated test counts (84 total), fixed embedding docs, updated aggregation description |
| `WAVE4_REWORK.md` | This file — rework receipts |

---

## Tests Added

| Test | Finding | Purpose |
|---|---|---|
| `TestEndToEnd_RunContradictionWarning` | F1 | Verifies Run() emits slog.Warn with source_url for contradictions |
| `TestEndToEnd_TwoCVEIndependent` | F4 | Two different CVEs produce independent beliefs, no cross-contamination |
| `TestEndToEnd_ResultProvenance` | F2 | Run() populates Result.Normalized with source info on all results |
| `TestEndToEnd_DeterministicOrderWithMultipleClaims` | P5 | Result order deterministic with multiple claims across runs |

---

## Actual Test Counts

| Package | Tests |
|---|---|
| `internal/kernel` | 27 |
| `internal/belief` | 18 |
| `internal/derive` | 16 |
| `internal/normalize` | 11 |
| `internal/intent` | 3 |
| `internal/pipeline` | 9 |
| **Total** | **84** |

---

## Verification Receipts

```
go build ./...                              → exit 0
go vet ./...                                → exit 0
gofmt -l internal/pipeline/ cmd/solvent/    → (empty)

go test ./internal/kernel/    -count=1      → ok  (27 tests)
go test ./internal/belief/    -count=1      → ok  (18 tests)
go test ./internal/intent/    -count=1      → ok  (3 tests)
go test ./internal/derive/    -count=1      → ok  (16 tests)
go test ./internal/normalize/ -count=1      → ok  (11 tests)
go test ./internal/pipeline/  -count=1      → ok  (9 tests)

TOTAL: 84 tests, all PASS (serial execution required)
```

---

## CLI Transcript Receipt

```
$ go run ./cmd/solvent/ --scenario "55555555-0000-0000-0000-000000000001" --reset

2026/08/08 11:25:40 WARN pipeline: contradiction received, no ledger mutation
  source_url=https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123459
  source_type=maintainer_comment
  claim="prior belief about etcd-io/etcd is contradicted"

SOLVENT TRANSCRIPT
==================

scenario: 55555555-0000-0000-0000-000000000001
results:  2

--- result 1 ---
  source:       kev_entry
  source_url:   https://nvd.nist.gov/vuln/detail/CVE-2024-24790
  claim:        etcd v3.4.x, v3.5.x before v3.5.15 is vulnerable to CVE-2024-24790
  classification: derived
  belief_id:    d81775aa-13e4-4e35-a846-f41f453c5dc5
  promoted:     true
  debt_items:   0
  intent:       live

--- result 2 ---
  source:       maintainer_comment
  source_url:   https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123459
  contradiction: true
  ledger_mutation: none

--- audit ---
audit_live_on_nonpromoted: 0

VERDICT: PASS
```

---

## Deterministic Replay Receipt

Replaying with the same scenario produces identical state:

| Field | Run 1 | Run 2 | Match |
|---|---|---|---|
| Belief count | 1 | 1 | ✓ |
| Belief status | promoted | promoted | ✓ |
| Debt items | 0 | 0 | ✓ |
| Evidence count | 5 | 5 | ✓ |
| Audit count | 0 | 0 | ✓ |
| Intent count | 1 | 1 | ✓ |
| Belief ID | d81775aa-... | d81775aa-... | ✓ |

---

## Two-CVE Aggregation Receipt

`TestEndToEnd_TwoCVEIndependent` with two different CVE fixtures:

| Field | CVE-2024-24790 (etcd) | CVE-2024-0001 (redis) |
|---|---|---|
| Belief created | ✓ | ✓ |
| Evidence rows | 1 | 1 |
| Cross-contamination | none | none |
| Total beliefs | 2 | — |
| Total evidence | 2 | — |

---

## Known Concurrency Limitation (unchanged)

**Intent proposal is replay-idempotent under sequential execution; concurrent duplicate-intent prevention is deferred.**

The `ProposeIfNew` function uses a SELECT-then-INSERT pattern. This is NOT a database-enforced concurrent uniqueness guarantee. Concurrent duplicate-intent prevention would require a unique index or serializable transaction, which is deferred.

---

— End of Wave 4 rework receipts —
