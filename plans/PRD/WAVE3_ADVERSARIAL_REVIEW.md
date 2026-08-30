# WAVE3 ADVERSARIAL REVIEW

**Date:** 2026-08-08
**Scope:** `internal/belief/` (belief.go, mapping.go, belief_test.go, integration_test.go), `internal/intent/` (intent.go, intent_test.go), kernel additions (EnsureBelief in kernel.go/sql.go + 3 tests).
**Authority used:** `AGENTS.md`, `IMPLEMENTATION_CONTRACT.md`, `wave3_prompt.md`, `WAVE3_IMPLEMENTATION_PLAN.md`, `wave3_plan_rework.md` (B1/B2/B3), `wave3_qa.md` (Option A refined → EnsureBelief), `wave3_imp.md` (receipt), `wave3_imp_review.md` (N1–N3).
**Method:** distributed-systems / data-engineering / adversarial / conformance passes with empirical probes (temporary in-package tests; all removed).

---

## Verdict: REWORK REQUIRED

One true P0, two P1s, all three caused by the same two faults: `Process` reports success when it did no work it claims to have done, and the evidence→debt mapping computes the wrong retirement for real-world input. Architecture itself is intact; fixes are contained to the Wave 3 files.

**Base assessments (verify with receipts below, green):**
- Kernel additions are additive and correct; `EnsureBelief` CTE eliminates TOCTOU — empirically proven safe under 24 concurrent goroutines (1 row, 1 ID, 0 errors).
- Pure Go, deterministic, kernel-owned correctness preserved; B1/B2/B3 resolutions honored.
- W3-Ensure-* tests exist; intent gates via composite FK are schema-enforced and tested.
- Evidence dedup TOCTOU accepted per M2/qa — confirmed documented and bounded.

---

## Critical Findings

### W3-P1 — `Process` swallows every Promote error; serializes DB failures as success [P0]

**File:** `internal/belief/belief.go:82-89`

```go
if err := st.Promote(ctx, beliefID); err != nil {
    if ctx.Err() != nil {
        return ctx.Err()
    }
    // ErrPromotionBlocked means debt is not fully retired.
    return nil   // ← everything else is silently dropped
}
```

The doc comment (claim 26-28) — and the plan/receipt — promise: "ErrPromotionBlocked → nil; **Other errors → returned immediately (fail loudly)**". The implementation never consults `ErrPromotionBlocked`; `belief.go` does not import `errors` at all. `kernel.Promote` maps 23514 → `ErrPromotionBlocked`, but every other failure class (deadlock 40P01, exhausted-retry 40001, connection drop, statement timeout, schema drift) is a real error that Promote returns and Process erases.

**Consequence:** the most safety-critical step (the actionability gate) can fail and the caller — every caller — is told "success". The ledger's core promise is "agents cannot act on beliefs that are no longer true"; here a belief may silently remain `entered` forever and a downstream orchestrator will assume the pipeline advanced. This is the exact "silent failure" class the contract bans.

**Required change (contained, vs errors.Is):**

```go
if err := st.Promote(ctx, beliefID); err != nil {
    if ctx.Err() != nil { return ctx.Err() }
    if errors.Is(err, kernel.ErrPromotionBlocked) { return nil }
    return fmt.Errorf("belief.Process: promote %s: %w", beliefID, err)
}
```

Add `TestProcess_PromoteHardFailure`: promote is forced to a non-23514 error (e.g., expect the belief to fail loudly). The existing `B-18` harness offers `inject_retry_errors_enabled` for a retried-failure variant.

---

### W3 — P2 [Contradiction path is a silent no-op; trace erased] [P1]

`internal/belief/belief.go:36-46` — for `b.Contradicts` non-empty, the code is:

```go
for _, c := range b.Contradicts { _ = c }
return nil
```

Empirically: `beliefs=0, evidence_rows=0, err=nil` — **nothing** is persisted or logged (the package imports no logging at all), and the implementation returns success.

- The receipt/`wave3_imp_review` N3 says: "Contradictions detected and surfaced; automatic retraction deferred". Here there is **no detection signal and no surfacing**: the contradicting evidence (the only record that a prior belief is false) is discarded at the wire. The ledger's auditability promise is a core selling point; the retracting evidence is not salvageable later.
- N1 (tech lead): "acceptable for MVP, but call it out **clearly**". The code does not disclose at all.

**Required change (contained):** at minimum, log at `slog.Warn` ("contradiction received, automatic retraction deferred — evidence dropped: %s", SourceURL of each Contradicts item) before `return nil`, and add a unit test asserting the log is emitted. Better (still MVP-scaling): persist a `contradicts`-kind `belief_edge` … blocked? No: `belief_edge` requires both FK sides; there is no target resolution. So logging + returning a documented no-op is the bounded correct fix here. Alternatives are explicitly out of scope (target resolution is a caller responsibility per plan).

---

### W3 — P3 [Evidence→debt mapping miscomputes on real input (first-rule-wins ordering)] [P1]

`internal/belief/mapping.go:40-45` — `DebtItemsForEvidence` returns the FIRST matching rule's items only, and **the first match is decided by pattern order, not semantics**:

| Evidence body (validation) | Rules matched (in table order) | Items RETIRED (actually) | Items plan intends |
|---|---|---|---|
| `"Confirmed no regression. All existing tests pass…"` | R1: none; R2 `tested\|confirmed` ✓; R3 `no regression` ✓ | `[needToyCheck needNullModel]` (R2) | `[needNullModel needObstruction]` |
| `"Reviewed by security team … the fix properly addresses…"` | R2: `confirmed` – no; R4 `reviewed by` ✓ | `[needFaithfulnessReview]` | same ✓ (ok here) |

**Concretely:** the Wave-2 fixture `maintainer_no_regression.json` contains **"Confirmed no regression…"** — "Confirmed" matches rule 2 (`tested|confirmed`) **first**, so `needObstruction` is only retired when a comment contains "no regression" but NOT "confirmed"; real-world maintainer comments nearly always pair them. Net effect in the demo pipeline: **`needObstruction` (and thus `needFaithfulnessReview`) can be unreachable on the honest fixture set** — full formation, the kernel's headline behavior, is unreachable exactly on the inputs the demo carries. The integration tests pass only because their hand-crafted assertions each hit a single rule ("security review completed", "no regression observed"), never mirror real fixture bodies.

**Resist temptation to reorder rules** (silently redesigns); correct fix is one of:
- **Bounded:** have each rule carry `Items` and **union** the items of all matched rules into one `[]string` (retirement becomes set semantics; `RetireDebt` is idempotent via `array_remove`), so a body that matches both `confirmed` and `no regression` retires both sets. Still deterministic: iterate rules in table order.
- Or add one integration test that pushes the real Wave-2 fixture bodies through mapping and asserts the promoted belief can reach full formation — making the fixture corpus actually able to fully retire the six-item debt instead of hand-crafted single-rule statements.

---

### W3-P4 — Verified strengths (record for the receipt)

- `EnsureBelief` concurrency: 24 goroutines → exactly 1 row, 1 unique ID, no errors. No unique index on (scenario_id, claim) is needed because CRDB SERIALIZABLE makes the CTE sound. Add a permanent test (`TestEnsureBelief_Concurrent`) so this stays proven.
- Classification mapping: `accommodated → kernel.Accommodated`, everything else → `kernel.Derived` — matches plan §6.
- Evidence dedup TOCTOU: accepted per the qa decision; documented, bounded, does not fail this review.

---

## Receipts (run serially, public across suites)

```
go build ./...                  → 0
go vet ./...                    → 0
gofmt -l internal/belief internal/intent internal/kernel → empty
go test ./internal/kernel/      → ok  (27 tests incl. W3-Ensure-New/Existing/DiffScenario)
go test ./internal/belief/      → ok  (11 tests)
go test ./internal/intent/      → ok  (3 tests)
go test ./internal/derive/      → ok  (16)
go test ./internal/normalize/   → ok  (11)
```

Caveat on running in parallel: belief/intent/kernel each DROP+CREATE the shared `fable_test` DB; `go test ./...` (parallel) has exhibited a race → "database 42P04 already exists" errors, and a forceloaded suite leaves `/tmp/fable_test.reset.lock` behind, blocking the next run **indefinitely without diagnostics** (the lock loop has no timeout, message, or removal-on-exit path — I hit this during review). Fix promptly: either serialize suite runs in CI or use one lock per DSN with a watchdog; at minimum log "waiting for lock", note owner PID, and treat a maliciously-kept lock as removal-after-`ps` guidance can't be automated — document in README.

---

## Reference of implemented receipt duties, passed

- B1 — `Process` is a thin translation layer: yes (each step is a direct kernel call; no business logic). Except containment bug W3-P1 (error absorbed).
- B2 — kernel owns find-or-create: yes; CTE, no TOCTOU, empirically proven.
- B3 — visibility model: documented in `wave3_imp.md` §9 L4 + justified.
- C2 — evidence→debt table is compile-time: yes (`mapping.go`), single table, no runtime mutation.
- I-3/I-5 — composite FK gate and `AuditLiveOnNonPromoted` verified in intent/kernel tests and integration scenes.

---

## Decision

**REWORK REQUIRED until all three land:**

1. `belief.Process` must `errors.Is` Promote's error against `ErrPromotionBlocked` and return every other error up (W3-P1, fail-loudly contract, P0 class).
2. The contradiction path must at minimum log a warning before returning (W3-P2, P1) so the dropped evidence is at least surfaced.
3. The evidence→debt mapping must retire the **union** of all matched rules (or the real fixture bodies must be able to fully retire the six debt items), fixing W3-P3.

After resolution, re-run the receipts above; then Wave 3 matches what the tech lead approved ("with nits").

— End of Wave 3 adversarial review —