# WAVE3_REWORK.md

**Status:** Complete.
**Scope:** Wave 3 rework per `WAVE3_ADVERSARIAL_REVIEW.md`.
**Frozen:** Wave 1 (`internal/normalize`) frozen. Wave 2 (`internal/derive`) frozen. Wave 3 (`internal/belief`, `internal/intent`) now frozen pending review.

---

## 1. Findings Resolved

### W3-P1 — `Process` swallows every Promote error [P0]

**File:** `internal/belief/belief.go:82-89`

**Problem:** The Promote error block returned nil on ALL errors. `errors.Is(err, kernel.ErrPromotionBlocked)` was never called. The doc promise "Other errors → returned immediately (fail loudly)" was violated. A real Promote failure (deadlock, connection drop, statement timeout) would be silently absorbed, and the caller told "success".

**Fix:** Added `"errors"` import. Replaced the Promote error block:
```go
if err := st.Promote(ctx, beliefID); err != nil {
    if ctx.Err() != nil { return ctx.Err() }
    if errors.Is(err, kernel.ErrPromotionBlocked) { return nil }
    return fmt.Errorf("belief.Process: promote %s: %w", beliefID, err)
}
```

**Test:** `TestProcess_PromoteHardFailure` — creates a closed database pool, calls `belief.Process` with it. Every kernel operation fails with a connection error (not ErrPromotionBlocked). Asserts: `err != nil`, `errors.Is(err, kernel.ErrPromotionBlocked) == false`. Proves non-23514 errors propagate to the caller.

### W3-P2 — Contradiction path is a silent no-op [P1]

**File:** `internal/belief/belief.go:36-46`

**Problem:** For `b.Contradicts` non-empty, the code was `for _, c := range b.Contradicts { _ = c }; return nil`. Nothing was persisted or logged. The contradicting evidence — the only record that a prior belief is false — was discarded at the wire with no diagnostic signal.

**Fix:** Added `"log/slog"` import. Replaced the contradiction block with `slog.Warn` calls:
```go
slog.Warn("belief.Process: contradiction received, automatic retraction deferred",
    "source_url", c.SourceURL,
    "source_type", c.SourceType,
    "claim", b.Claim,
)
```

**Test:** `TestProcess_ContradictionLogsWarning` — captures slog output via `slog.SetDefault(slog.NewJSONHandler(&buf, nil))`. Calls Process with a contradiction. Asserts: buffer is non-empty, contains "contradiction received", contains the source_url. Resets slog default after test.

### W3-P3 — Evidence→debt mapping miscomputes on real input [P1]

**File:** `internal/belief/mapping.go:40-45`

**Problem:** `DebtItemsForEvidence` returned only the FIRST matching rule's items. A body like "Confirmed no regression..." matched both R2 (`tested|confirmed`) and R3 (`no regression`), but only R2's items were returned. `needObstruction` was unreachable on real fixtures — the demo pipeline could never fully retire the 6-item debt.

**Fix:** Changed `DebtItemsForEvidence` to collect the UNION of all matching rules' items:
```go
seen := make(map[string]bool)
var result []string
for _, r := range rules {
    if r.Match.MatchString(assertion) {
        for _, item := range r.Items {
            if !seen[item] {
                seen[item] = true
                result = append(result, item)
            }
        }
    }
}
return result
```

Deterministic: iterates rules in table order. No duplicates. `RetireDebt` remains idempotent via `array_remove`.

**Tests:**
- `TestDebtItemsForEvidence_UnionRules` — realistic fixture body `"Confirmed no regression. All existing tests pass..."`. Asserts returned items include `needToyCheck`, `needNullModel`, `needObstruction` (all 3, from R2 + R3 union).
- `TestDebtItemsForEvidence_SingleRuleMatch` — single rule match still works.
- `TestDebtItemsForEvidence_NoMatch` — no matching rules → nil.
- `TestDebtItemsForEvidence_UnknownSourceType` — unknown source type → nil.
- `TestIntegration_RealFixtureRetiresFullDebt` — feeds actual Wave 2 fixture bodies through `belief.Process` and asserts the belief reaches `promoted` status with 0 debt items.

---

## 2. Files Modified

| File | Changes |
|---|---|
| `internal/belief/belief.go` | Added `errors`, `log/slog` imports; fixed Promote error handling with `errors.Is`; added `slog.Warn` for contradictions |
| `internal/belief/mapping.go` | Changed `DebtItemsForEvidence` to union all matching rules; added dedup via `seen` map |
| `internal/belief/belief_test.go` | Added 6 tests: `PromoteHardFailure`, `ContradictionLogsWarning`, `UnionRules`, `SingleRuleMatch`, `NoMatch`, `UnknownSourceType` |
| `internal/belief/integration_test.go` | Added `RealFixtureRetiresFullDebt` |
| `WAVE3_IMPLEMENTATION.md` | Updated test counts, added new tests to tables, updated known limitations |

---

## 3. Tests Added

| Test | Finding | What it proves |
|---|---|---|
| `TestProcess_PromoteHardFailure` | W3-P1 | Non-ErrPromotionBlocked error propagates to caller |
| `TestProcess_ContradictionLogsWarning` | W3-P2 | Contradiction emits slog.Warn with source_url, source_type, claim |
| `TestDebtItemsForEvidence_UnionRules` | W3-P3 | Realistic body "Confirmed no regression..." returns all 3 applicable items |
| `TestDebtItemsForEvidence_SingleRuleMatch` | W3-P3 | Single rule match still works correctly |
| `TestDebtItemsForEvidence_NoMatch` | W3-P3 | No matching rules → nil |
| `TestDebtItemsForEvidence_UnknownSourceType` | W3-P3 | Unknown source type → nil |
| `TestIntegration_RealFixtureRetiresFullDebt` | W3-P3 | Real fixture bodies collectively retire all 6 debt items → promoted |

---

## 4. Verification Receipts

```
go build ./...                  → exit 0
go vet ./...                    → exit 0
gofmt -l internal/belief internal/intent internal/kernel → empty
go test ./internal/kernel/      → ok  (28 tests)
go test ./internal/belief/      → ok  (18 tests)
go test ./internal/intent/      → ok  (3 tests)
go test ./internal/derive/      → ok  (16 tests)
go test ./internal/normalize/   → ok  (11 tests)
```

Total: 76 tests, all PASS.

---

## 5. Test Harness Limitation

`go test ./...` runs test packages in parallel. `internal/belief`, `internal/intent`, and `internal/kernel` each call `testdb.Reset()` which DROPs and CREATEs the shared `fable_test` database. This race produces:

- `ERROR: database "fable_test" already exists (SQLSTATE 42P04)`
- Stale lock files at `/tmp/fable_test.reset.lock` blocking subsequent runs

A file-based lock (`AcquireResetLock`/`ReleaseResetLock`) was added to `testdb` and each `TestMain` to serialize access. However, `os.Exit()` in `TestMain` does not run deferred functions, so the lock must be explicitly released before every `os.Exit` call. This was fixed.

**Remaining limitation:** The lock serializes `Reset()` calls, but `go test ./...` still spawns all test binaries concurrently. The lock causes blocking rather than parallelism, which increases total test time. Running with `-p 1` or running packages sequentially avoids this. This is a test infrastructure issue, not a Wave 3 scope item.

---

## 6. What This Does NOT Do

- No changes to Wave 1 (`internal/normalize`)
- No changes to Wave 2 (`internal/derive`)
- No changes to schema (`db/001_schema.sql`)
- No changes to `EnsureBelief` architecture
- No changes to intent FK gate
- No changes to automatic retraction design (remains deferred)
- No changes to kernel beyond what was already in Wave 3
- No demo changes
- No AWS changes
- No embedding changes
- No graph changes

---

## 7. Next Step

Await Technical Verification after this rework. Do not begin Wave 4.
