# Plan6 — Final MCP Hardening Pass

**Date:** 2026-08-11
**Status:** READY FOR IMPLEMENTATION
**Source:** `plans/mcp/plan6_prompt.md`, `plans/mcp/plan5_review.md`
**Scope:** Mechanical corrections only. No architectural changes. Frozen core untouched.

---

## F-1 — I-7 Gate Hardening

**File:** `Taskfile.yml` (lines 82–86)

**Current:** Single grep catches `Exec/ExecContext` + SQL keyword on the same physical line. Misses gofmt-wrapped calls, `QueryRowContext` writes, const-hoisted SQL (the kernel's own idiom).

**Fix:** Replace with two-stage gate:

```sh
# Stage 1: Reject write/transaction entry points in MCP/view
if grep -rnE --include='*.go' '\.(Exec|ExecContext|Prepare|PrepareContext|Begin|BeginTx)\(' cmd/solvent-mcp internal/view; then
  echo "FAIL: direct write/transaction call in MCP/view"
  exit 1
fi
# Stage 2: Reject write SQL text in MCP/view
if grep -rnE --include='*.go' '(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]+(INTO|TABLE|FROM)' cmd/solvent-mcp internal/view; then
  echo "FAIL: write SQL text in MCP/view"
  exit 1
fi
```

**Invariant:** MCP/view may read. MCP/view may call the Solvent kernel. MCP/view may NOT directly write or open write transactions.

### Adversarial Scratch Tests (8 cases)

All tests on scratch copies under `/tmp/opencode/i7_test/`. Repository must be byte-for-byte unchanged afterward.

| # | Injected form | Expected |
|---|---|---|
| 1 | Clean repository | PASS |
| 2 | `db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")` | FAIL |
| 3 | `tx.ExecContext(ctx, "UPDATE ...")` | FAIL |
| 4 | `db.ExecContext(ctx, "UPDATE ...")` (gofmt-wrapped) | FAIL |
| 5 | `db.QueryRowContext(ctx, "UPDATE ... RETURNING id")` | FAIL |
| 6 | `const sqlBackdoor = "UPDATE ..."` + `db.ExecContext(ctx, sqlBackdoor, id)` | FAIL |
| 7 | Missing `cmd/solvent-mcp` | FAIL |
| 8 | Missing `internal/view` | FAIL |

After scratch tests, verify real repository is unchanged, then run gate on real source → PASS.

---

## F-2 — Strict belief_id Validation in promote/retire_debt

**File:** `cmd/solvent-mcp/tools.go`

**Current:** `beliefID, _ := args["belief_id"].(string)` — silently converts wrong type to `""`. Ownership guard passes on empty string because `GetSnapshot` unfiltered branch returns all beliefs, and `len(...) == 0` is false.

**Fix in both `handleSolventPromote` and `handleSolventRetireDebt`:**

```go
beliefID, ok := args["belief_id"].(string)
if !ok || beliefID == "" {
    return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
}
```

Then after snapshot lookup, assert identity rather than non-emptiness:

```go
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

**Tests (via MCP or manual verification):**
1. Valid same-scenario UUID → mutation works
2. Cross-scenario UUID → clean error, zero mutation
3. Missing belief_id → clean argument error
4. Numeric belief_id → clean argument error
5. Boolean belief_id → clean argument error
6. Object/array belief_id → clean argument error
7. Empty string → clean argument error

---

## F-3 — Strict belief_id Validation in ledger

**File:** `cmd/solvent-mcp/tools.go`

**Current:** Same `beliefID, _ := args["belief_id"].(string)` — silently widens to full scenario when `belief_id` is present but wrong type.

**Fix:** Distinguish "absent" from "present but wrong type":

```go
beliefIDRaw, hasBelief := args["belief_id"]
beliefID, ok := beliefIDRaw.(string)
if hasBelief && (!ok || beliefID == "") {
    return errorResult(fmt.Errorf("belief_id must be a non-empty string when provided")), nil
}
```

**Behavior:**
- `{"scenario":"track1"}` → normal scenario-wide ledger
- `{"scenario":"track1","belief_id":"valid UUID"}` → one-belief ledger
- `{"scenario":"track1","belief_id":12345}` → argument error
- `{"scenario":"track1","belief_id":true}` → argument error
- `{"scenario":"track1","belief_id":{}}` → argument error
- `{"scenario":"track1","belief_id":""}` → argument error

Do not alter underlying view semantics.

---

## F-4 — Exact Test Count

**File:** `plans/mcp/plan5_imp.md`

Change: "107+ tests passing" → "107 PASS, 0 FAIL, 0 SKIP"

---

## Verification

| Step | Expected |
|---|---|
| `task --list` | 11 tasks, clean YAML |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l cmd internal` | no output |
| Frozen core diff | no output |
| 8 adversarial scratch tests | all malicious → FAIL, clean → PASS |
| `task test` | 107 tests, 0 failures |
| `git status --short` | no stray binary, no untracked files |

## Files Changed

| File | Change |
|---|---|
| `Taskfile.yml` | I-7 gate (lines 82–86): two-stage grep |
| `cmd/solvent-mcp/tools.go` | F-2 (promote, retire_debt) + F-3 (ledger) |
| `plans/mcp/plan5_imp.md` | F-4 (test count wording) |

## Frozen Core

Untouched. No new files. No new tools. No schema changes.

## Final State

All six fixes from plan5 + four hardening fixes from plan6 applied. I-7 gate catches all tested write forms. Input validation rejects malformed belief_id at the adapter boundary. Test count is exact.
