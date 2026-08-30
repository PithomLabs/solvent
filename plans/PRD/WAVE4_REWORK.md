# WAVE4_REWORK.md

**Status:** Open — rework items documented but not applied to code.
**Scope:** `internal/pipeline/pipeline.go`, `internal/pipeline/pipeline_test.go`, `cmd/solvent/main.go`, documentation.
**Authority:** `WAVE4_ADVERSARIAL_REVIEW.md` (F1–F4, P4, P5), TL rework reviews (`wave4_rework.md`, `wave4_rework2.md`).
**Method:** Code audit against adversarial review findings; verified against live CockroachDB v26.2.0.

---

## Verdict: REWORK REQUIRED

Six rework items were identified by the adversarial review and documented. The code fixes were **never implemented**. The current `pipeline.go`, `pipeline_test.go`, and `main.go` still contain the unfixed versions. All findings are contained to Wave 4 files — no schema changes, no frozen wave modifications.

---

## Finding Status

### F1 [P1] — Contradiction path is silent in `Run()` — no `slog.Warn`

**Spec violated:** D-6 — "Contradiction fixture produces `slog.Warn` … transcript shows warning with `source_url`."
**Location:** `internal/pipeline/pipeline.go:289-295` (Phase 4 of `Run`).
**Current code:**
```go
// Phase 4: record contradictions (no ledger mutation).
for _, c := range contradictions {
    allResults = append(allResults, Result{
        Contradiction: true,
        Beliefs:       []derive.DerivedBelief{c},
    })
}
```
**Problem:** Zero log lines emitted. The `ProcessEvidence` path (line 85-96) does warn, but `Run()` — the path `cmd/solvent` actually uses — does not. The demo-facing contradiction receipt is silent.
**Fix:** In `Run` Phase 4, emit `slog.Warn` with `source_url` and `source_type` before appending the result. Consider refactoring so both `ProcessEvidence` and `Run` reuse the same warn helper.

---

### F2 [P1] — `Result.Normalized` empty in `Run()`, transcript source fields always blank

**Spec violated:** Plan §13 — source type and `source_url` are deterministic transcript fields.
**Location:** `internal/pipeline/pipeline.go:270-275` (result construction in `Run`).
**Current code:**
```go
allResults = append(allResults, Result{
    BeliefID:  beliefID,
    Promoted:  status == "promoted",
    DebtItems: make([]string, debtLen),
    Beliefs:   []derive.DerivedBelief{d},
})
```
**Problem:** `Normalized` is left zero-valued. `cmd/solvent/main.go:94-95` renders `r.Normalized.SourceType` and `r.Normalized.SourceURL` — both always empty in the demo path.
**Fix:** Set `Normalized` from the `norm` produced in Phase 1. The `norm` variable is scoped inside the Phase 1 loop; either carry it through the aggregation or store it on the merged `DerivedBelief`. ~3 lines.

---

### F4 [P1] — Evidence aggregation merges semantically independent CVE claims

**Spec violated:** N1 — "verify that this cannot accidentally merge semantically independent claims."
**Location:** `internal/pipeline/pipeline.go:147-165` (`aggregateDeriveResults`).
**Current code:**
```go
if primaryClaim != "" && d.Claim != primaryClaim {
    // merge into primary — no CVE check
}
```
**Problem:** A second CVE-bearing claim (e.g., "redis is vulnerable to CVE-2024-00001") is silently merged into the first CVE claim (e.g., "etcd is vulnerable to CVE-2024-24790"), its evidence attached to a belief about a different product and vulnerability. The current single-CVE fixture set masks this.
**Fix:** CVE-bearing claims are their own primaries; never merge a different-CVE claim into an existing primary:
```go
if cvePattern.MatchString(d.Claim) {
    // CVE-bearing claims are their own primaries
    if entry, ok := claims[d.Claim]; ok {
        entry.evidence = append(entry.evidence, d.SupportingEvidence...)
    } else {
        claims[d.Claim] = &claimEntry{
            classification: d.Classification,
            evidence:       d.SupportingEvidence,
        }
    }
    continue
}
if primaryClaim != "" {
    // merge non-CVE evidence into primary
}
```

---

### F3 [P2] — Documentation inconsistency on embedding column

**Problem:** Some doc references mention `belief.embedding` as if it exists. The schema (`db/001_schema.sql`) has no embedding column by design (I-6).
**Location:** `WAVE4_IMPLEMENTATION.md:159` — says `embedding VECTOR(1536) NULL` not yet in schema. This is accurate but inconsistent with other references.
**Fix:** Ensure all documentation says: "The schema has no embedding column by design (I-6). The ledger operates correctly with zero vectors." No code change.

---

### P4 [P2] — Test count discrepancy: 80 pass, not 81

**Receipt claims:** kernel=28, pipeline=7, total=81.
**Actual counts:**
| Package | Tests |
|---|---|
| `internal/kernel` | 27 |
| `internal/belief` | 18 |
| `internal/derive` | 16 |
| `internal/normalize` | 11 |
| `internal/intent` | 3 |
| `internal/pipeline` | 5 |
| **Total** | **80** |

**Fix:** Update all receipt tables in `WAVE4_IMPLEMENTATION.md` and any other docs to reflect actual counts.

---

### P5 [P3] — Result-order determinism depends on Go map iteration order

**Location:** `internal/pipeline/pipeline.go:179-186` (`aggregateDeriveResults`).
**Current code:**
```go
for claim, entry := range claims {
    merged = append(merged, derive.DerivedBelief{...})
}
```
**Problem:** Map iteration order in Go is unspecified. Currently one merged claim so order holds, but `TestEndToEnd_DeterministicResults` would flake with two claims.
**Fix:** Sort `merged` by claim before return (~2 lines).

---

## Additional Finding (not in adversarial review)

### main.go redundant intent proposal

**Location:** `cmd/solvent/main.go:70-77`.
**Current code:**
```go
// Propose intents for promoted beliefs.
for _, r := range results {
    if r.Contradiction || r.BeliefID == "" || !r.Promoted {
        continue
    }
    if err := pipeline.ProposeIfNew(ctx, db, *scenario, r.BeliefID, "upgrade"); err != nil {
        fail(fmt.Sprintf("propose intent: %v", err))
    }
}
```
**Problem:** `Run()` Phase 3.5 already proposes intents and sets `r.IntentState = "live"`. This loop re-proposes — redundant. `ProposeIfNew` is idempotent (SELECT-then-INSERT), so it's harmless but wasteful.
**Recommendation:** Remove the redundant loop from `main.go`. The transcript already reads `r.IntentState` from `Run()`.

---

## Receipt After Rework

Once fixes are applied, re-run:
```
go build ./...                              → exit 0
go vet ./...                                → exit 0
gofmt -l internal/pipeline cmd/solvent      → (empty)
go test ./internal/kernel/    -count=1      → ok  (27 tests)
go test ./internal/belief/    -count=1      → ok  (18 tests)
go test ./internal/intent/    -count=1      → ok  (3 tests)
go test ./internal/derive/    -count=1      → ok  (16 tests)
go test ./internal/normalize/ -count=1      → ok  (11 tests)
go test ./internal/pipeline/  -count=1      → ok  (5+ tests)
TOTAL: 80+ tests, all PASS (serial execution required)
```

---

## Message to Reviewer

All six rework items (F1, F2, F4, F3, P4, P5) were identified by the adversarial review and documented here, but the code fixes were never applied to `pipeline.go`, `pipeline_test.go`, or `main.go`. The fixes are small and contained entirely to Wave 4 files — no schema changes, no frozen wave modifications. After applying, all receipts must be re-verified. The pipeline's core correctness properties (promotion gating, contradiction-no-mutation, replay-idempotency, audit=0) hold in the current code; these findings affect the CLI-facing observable behavior (logging, transcript, aggregation safety), not the kernel invariants.

---

— End of Wave 4 rework receipts —
