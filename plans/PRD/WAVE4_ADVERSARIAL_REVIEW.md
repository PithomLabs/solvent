# WAVE4 ADVERSARIAL REVIEW

**Date:** 2026-08-08
**Scope:** `internal/pipeline/` (pipeline.go, pipeline_test.go), `cmd/solvent/main.go`.
**Authority:** `AGENTS.md`, `IMPLEMENTATION_CONTRACT.md`, `DATA_PIPELINE_SPEC.md`, approved `WAVE4_IMPLEMENTATION_PLAN.md` (§1–§19), `WAVE4_IMPLEMENTATION.md` (receipt), Tech Lead `wave4_adv_review_prompt.md` (N1–N3 + 7 focus areas + classify P0/P1/P2 + exactly one verdict).
**Method:** distributed-systems / data-engineering / adversarial / conformance passes, static review of all Wave 4 code against the approved plan, receipts, and the frozen Waves 1–3 contracts + empirical probes against the live CockroachDB v26.2.0 test cluster (probe file removed after the review).
**Also read for context:** `wave4_prompt.md`, `wave4_rework.md` (TL P0/P1), `wave4_rework2.md` (final approval).

---

## Verdict: REWORK REQUIRED

Three P1 contract violations (F1, F2, F4). All fixes are small and contained to Wave 4 files: nothing in Waves 1–3 needs to change, no schema change, no new table, no features added. Everything the Tech Lead marked "deliberately deferred" stays deferred and is correctly documented.

**Base assessments (all receipts green, proof below):**
- End-to-end chain works against real CockroachDB: kev → promoted belief, audit = 0, one live intent, replay idempotent.
- Frozen waves untouched: kernel/belief/intent/derive/normalize files unchanged by Wave 4.
- Decision honesty is real: `ProposeIfNew` concurrency race is genuinely documented (N3) and the code does not overclaim — verified empirically (20/20 duplicate intents under concurrency — the race exists exactly as documented).
- Determinism: fixed scenario IDs, sorted fixture processing, deterministic claim texts.
- Schema/doc issue (N2) resolved below without adding embeddings.
- No business logic leaked from pipeline into the kernel.

---

## Critical Findings

### F1 [P1] — Contradiction path is silent in the CLI (`Run`), violating D-6

**Location:** `pipeline.Run` Phase 4 — `internal/pipeline/pipeline.go:289-295`; `cmd/solvent/main.go:96-101`.
**Spec violated:** approved plan §2 contradiction flow and demo acceptance criterion **D-6**: "Contradiction fixture produces `slog.Warn` … transcript shows warning with `source_url`."
**Empirical proof:** `Run()` on `contradicting_evidence.json` returns `Contradiction=true`, but **zero** log lines are emitted (probe: `contradiction results=1, log lines with 'contradiction': 0`). `ProcessEvidence` does warn (pipeline.go:85-96), but `Run` — the path `cmd/solvent` actually uses, per plan §14 D-1/D-2 — appends the contradiction result **without warning and without `source_url`**. The transcript's contradiction block prints `source: ""`, `source_url: ""`.

For the demo, the contradiction fixture is the anti-FUD receipt; it must visibly log the contradiction with its URL, exactly as planned. Right now the *demo-facing* path silently drops it.

**Fix:** in `Run` Phase 4, take over the `ProcessEvidence` behavior:
```go
for _, c := range contradictions {
    for _, ev := range c.Contradicts {
        slog.Warn("pipeline: contradiction received, no ledger mutation",
            "source_url", ev.SourceURL, "source_type", ev.SourceType)
    }
    allResults = append(allResults, Result{Contradiction: true, Beliefs: []derive.DerivedBelief{c}})
}
```
Optionally refactor so `Run` reuses the same warn helper as `ProcessEvidence` so the two paths can't drift again.

### F2 [P1] — `Result.Normalized` empty in `Run`, transcript source fields always blank

**Location:** `pipeline.Run` result construction, `internal/pipeline/pipeline.go:270-275`; `cmd/solvent/main.go:93-95,101`.
`Run` builds `Result{BeliefID, Promoted, DebtItems, Beliefs}` — `Normalized` is left zero-valued. The CLI transcript renders `source:`/`source_url:` from `r.Normalized`, i.e. always empty in the demo path. Plan §13 lists source type and `source_url` as deterministic transcript fields the CLI must render.

**Fix:** set `r.Normalized` from each merged belief's first supporting evidence around the loop at pipeline.go:260-275 (the merged `derive.DerivedBelief` retains `SupportingEvidence` with the real source), so the transcript shows the true source. Or populate a dedicated `Result` field. ~3 lines.

---

### F4 [P1] — Evidence aggregation can merge semantically independent claims (N1)

**Location:** `aggregateDeriveResults`, `internal/pipeline/pipeline.go:123-189`.
**Spec violated:** N1 — "verify that this cannot accidentally merge semantically independent claims … Determine whether merging all non-contradiction evidence into the first CVE-bearing claim can incorrectly merge semantically independent claims."

**Empirical proof:** two CVE-bearing claims — `"etcd is vulnerable to CVE-2024-24790"` and `"redis is vulnerable to CVE-2024-00001"` — produce:

```
PROBE-MERGE merged=1 contra=0
PROBE-MERGE claim="etcd is vulnerable to CVE-2024-24790" … evidence=2
  evidence="CVE-2024-24790 etcd vulnerable to DoS"
  evidence="CVE-2024-00001 redis vulnerable to RCE"
```

The second claim is **not** its own belief: it is silently merged into the first CVE claim, its evidence attached to a belief about a *different product and different vulnerability*. The merge key is "claim != primaryClaim" — **not** "same CVE" nor "same product". The current fixture set has exactly one CVE-bearing claim, so the current corpus is safe — explicitly the situation N1 protects ("if it is safe for the current fixture set, leave it alone"). BUT the mechanism is wrong: it would corrupt the ledger the moment a second KEV/release or a second product appears — and, worse, a second CVE-bearing claim's evidence then counts toward *debt retirement of a belief about a different vulnerability* (belief.Process retires debt from the merged evidence list).

**Not a multi-domain abstraction request** — this is a 4-line conditional:

```go
if cvePattern.MatchString(d.Claim) {
    // CVE-bearing claims are their own primaries; never merge a
    // different-CVE claim into an existing primary.
    if entry, ok := claims[d.Claim]; ok { entry.evidence = append(...) } else { claims[d.Claim] = &claimEntry{...} }
    continue
}
if primaryClaim != "" { /* existing merge */ }
```
or equivalently: only merge claims whose evidence references the same CVE as the primary (match `cvePattern` on `d.Claim` and on the primary before merging).

---

### F3 [P2] — `WAVE4_IMPLEMENTATION_PLAN` §6 contradicts the repository (N2 resolved)

- Schema (`db/001_schema.sql:2,6`): **no embedding column exists**. The file's own comment says: "I-6: no embedding column. The ledger is meaningful with zero vectors".
- Plan §6 states: "the `belief.embedding` column is nullable and not used" — **false**; there is no such column.
- Receipt §7.5: "`embedding VECTOR(1536) NULL` not yet in `db/001_schema.sql`" — **accurate**.

**Resolution (N2):** the receipt is right; plan §6's statement is wrong. Correct the plan §6 wording (or a note in WAVE4_IMPLEMENTATION.md) to: "The schema has no embedding column by design (I-6). The ledger operates correctly with zero vectors." **Do not add the column** — explicitly forbidden by N2.

---

### P4 [P2] — Test count discrepancy: 80 pass, not 81

Receipt §4 claims "All 76+5 … 81 tests, all PASS" with kernel=28. Actual: kernel=27, belief=18, intent=3, derive=16, normalize=11, pipeline=5 → **80**. The kernel suite runs 27 tests, not 28. Fix the receipt numbers.

---

### P5 [P3] — Result-order determinism depends on Go map iteration order

`aggregateDeriveResults` builds `merged` by iterating over a `map[string]claimEntry` (pipeline.go:179-186). Map iteration order in Go is unspecified; today there is exactly one merged claim, so the order holds — but the determinism of `TestEndToEnd_DeterministicResults` (plan §12) rides on a single-key map and could flake if the corpus ever yields a second claim. Sort `merged` by claim before return (deterministic and ~2 lines). Not exercised today; a cheap future-proof.

---

## Focus-area verdicts (from the review prompt)

1. **Evidence aggregation** — current fixtures correct; mechanism unsafe for a second CVE (F4) — must use the minimal CVE-aware branch above.
2. **Intent idempotency** — sequential replay idempotency **verified** (tests + probe: 3 proposals → 1 intent; full replay keeps 1 intent); concurrent prevention genuinely absent but **documented verbatim** per N3. PASS as scoped.
3. **Contradiction** — `ProcessEvidence` path: correct (warn, no belief, no evidence, no ledger mutation — verified by `TestEndToEnd_ContradictionNoLedgerMutation`: 0 beliefs, 0 evidence rows). `Run`/CLI path: **fail** (P1, see F1). The "no contradictory evidence persisted" property holds in both paths.
4. **End-to-end invariants** — partial debt blocks promote: verified (status `entered`, `ErrActionOnUnpromoted`); promoted belief gets live intent: verified (FK enforces); no live intent on non-promoted: verified; replay no duplicate live intents: verified; audit=0: verified. All 5 hold, each with a test.
5. **Determinism** — scenario IDs fixed; fixture order sorted; claims deterministic. Verified: two scenario runs produce identical result order and counts. Sensitivities: F2 (transcript source fields blank) and F5 (map-iteration order) — both covered by recommended fixes.
6. **Schema/documentation** — F3 resolution above; no other code/schema inconsistency found (composite FK, `live_requires_promoted`, `promoted_is_debt_free` all match `db/001_schema.sql`).
7. **Architecture** — no business logic in the kernel; pipeline is translation/orchestration; the recommended aggregation change remains a pipeline-local concern per N1. PASS.

---

## Receipts

```
go build ./...                              → exit 0
go vet ./...                                → exit 0
gofmt -l internal/pipeline cmd/solvent        → (empty)
go test ./internal/kernel/    -count=1      → ok  (27 tests)
go test ./internal/belief/    -count=1      → ok  (18 tests)
go test ./internal/intent/    -count=1      → ok  (3 tests)
go test ./internal/derive/    -count=1      → ok  (16 tests)
go test ./internal/normalize/ -count=1      → ok  (11 tests)
go test ./internal/pipeline/  -count=1      → ok  (5 tests)
TOTAL: 80/80 pass (serial; suites were run serially, see P4)

CockroachDB binary: v26.2.0 (build 2026/04/21) — matches receipt.
```

Empirical probes (temporary, since removed) ran against `fable_test` on :26260 and their outputs are quoted inline. All received after deletions: re-ran `go build`, `go vet`, and `go test ./internal/pipeline -count=1` to confirm the probe removal left no residue — green.

---

## Verdict

**REWORK REQUIRED** (P1s: F1 contradiction-silence in `Run`, F2 blank transcript sources, F4 unsafe merge; P2s: F3 doc fix, P4 count; P3: F5 map order).

After the stated minimal fixes (all in `internal/pipeline` + one doc line + one receipt line), re-run the receipts; the MVP then matches the Tech Lead's "APPROVED WITH NITS" expectations.

— End of Wave 4 adversarial review —