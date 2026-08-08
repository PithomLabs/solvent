## Technical Lead Review

**Verdict: REWORK REQUIRED**

This is a **high-quality adversarial review**, and I agree with its three substantive findings. The important part is that the architecture remains intact; these are contained implementation defects. 

### P0 — Must fix

**1. `Process()` swallows real `Promote` failures.**

This is a genuine safety bug. The implementation promises “only `ErrPromotionBlocked` is absorbed,” but currently returns `nil` for every promotion error. 

**Fix:** `errors.Is(err, kernel.ErrPromotionBlocked)` → `nil`; everything else → return/wrap the error.

### P1 — Must fix

**2. Contradiction evidence is silently discarded.**

The review is right: calling this "surfaced" is inaccurate when the code literally discards it and returns success. 

For the MVP, **do not build retraction now**. Just emit a structured warning and test it. That preserves the declared scope while making the limitation observable.

**3. Evidence→debt mapping uses first-match semantics.**

This is the most interesting finding because it threatens the **actual demo path**. A realistic `"Confirmed no regression"` comment matches both rules, but only the first rule's debt is retired. 

I accept the reviewer's preferred fix:

> **Union all matching rule outputs.**

That is deterministic, simple, and semantically correct because `RetireDebt` is already idempotent.

Do **not** merely reorder the rules.

---

### One additional finding

The shared test database race is real, but **I would not make it a Wave 3 code blocker**. 

Record it as an engineering/CI issue and fix the test harness separately. We don't want to expand the Wave 3 kernel scope because the reviewer discovered parallel-test infrastructure debt.

---

# Decision

```text
REWORK REQUIRED

P0:
- Fix Promote error swallowing.

P1:
- Surface contradiction evidence.
- Union all matching evidence→debt rules.

Deferred:
- Test DB parallelism/lock robustness.
```

## Prompt for Claude Code

```text
Implement the Technical Lead-approved Wave 3 rework.

Do NOT redesign Wave 3.
Do NOT begin Wave 4.
Do NOT implement automatic contradiction retraction.

The adversarial review found three accepted implementation defects.

1. FIX PROMOTE ERROR HANDLING — P0

In internal/belief/belief.go:

Process() must:

- absorb ONLY kernel.ErrPromotionBlocked
- propagate every other Promote error
- preserve context cancellation behavior

Use errors.Is(err, kernel.ErrPromotionBlocked).

Required behavior:

ErrPromotionBlocked → nil

Any other error → return/wrap error

Add:

TestProcess_PromoteHardFailure

Prove a non-23514 Promote failure is returned to the caller.

2. SURFACE CONTRADICTIONS — P1

The current contradiction path silently discards b.Contradicts.

Keep automatic retraction deferred.

When b.Contradicts is non-empty:

- emit a structured slog warning
- identify the contradiction/evidence sufficiently for diagnosis
- return nil because automatic retraction remains intentionally deferred

Add a test proving the warning is emitted.

Do NOT add belief_edge.
Do NOT add graph traversal.
Do NOT implement retraction.

3. FIX EVIDENCE→DEBT MAPPING — P1

DebtItemsForEvidence currently returns only the first matching rule.

Change it to return the UNION of debt items from ALL matching rules.

Requirements:

- deterministic ordering
- no duplicate debt items
- preserve the existing compile-time mapping table
- do not reorder rules merely to hide the defect
- RetireDebt remains idempotent

Add/modify tests using realistic Wave 2 fixture text, especially:

"Confirmed no regression. All existing tests pass…"

Prove that ALL applicable debt items are returned.

Also add an integration test proving realistic fixture evidence can collectively retire the required debt and reach promotion.

4. DO NOT CHANGE

Do not modify:

- Wave 1
- Wave 2
- schema
- EnsureBelief architecture
- intent FK gate
- automatic retraction design
- bchat
- demo
- AWS
- embeddings
- graph

5. TEST HARNESS ISSUE

The adversarial review found a shared `fable_test` database race when test suites execute in parallel.

Do NOT redesign the kernel for this.

If the fix is trivial and isolated to test infrastructure, document it separately.

Otherwise record it as a known test-harness limitation and do not expand Wave 3 scope.

6. VERIFICATION

Run serially:

go build ./...
go vet ./...
gofmt -l internal/belief internal/intent internal/kernel

go test ./internal/kernel/ -count=1
go test ./internal/belief/ -count=1
go test ./internal/intent/ -count=1
go test ./internal/derive/ -count=1
go test ./internal/normalize/ -count=1

Then run the relevant Wave 3 integration tests.

Update:

WAVE3_IMPLEMENTATION.md

Create:

WAVE3_REWORK.md

Document:

- W3-P1 resolved
- W3-P2 resolved
- W3-P3 resolved
- tests added
- verification receipts
- remaining test-harness limitation

STOP.

Do not begin Wave 4.

Wait for Technical Verification before Wave 3 is frozen.
```

This is a **contained rework**, not a reason to reopen the architecture. The adversarial review actually confirms the most important thing: `EnsureBelief`, the intent gate, CockroachDB ownership, and the overall kernel boundary survived hostile testing. 
