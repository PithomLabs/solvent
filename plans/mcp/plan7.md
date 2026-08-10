# Plan7 — Final MCP Hardening Fixes

**Date:** 2026-08-11
**Status:** READY FOR IMPLEMENTATION
**Source:** `plans/mcp/plan7_prompt.md`, `plans/mcp/plan6_adv_review.md`
**Scope:** Mechanical corrections only. No architectural changes. Frozen core untouched.

---

## A-1 — `solvent_authorize_action` guard

**File:** `cmd/solvent-mcp/tools.go` (line 184)

**Current:** Lax `beliefID, _ := args["belief_id"].(string)` with no ownership guard before `IntentOnPromoted`.

**Fix:** Apply the identical 6-line pattern already proven in `handleSolventPromote` and `handleSolventRetireDebt`:

```go
beliefID, ok := args["belief_id"].(string)
if !ok || beliefID == "" {
    return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
}
// ... after scenarioID resolution, before the kernel call:
snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

Identity-only check. No debt/status/intent inspection. M2 still originates in CockroachDB FK.

---

## A-2 — `solvent_falsify` guard + NO-OP verdict

**File:** `cmd/solvent-mcp/tools.go` (line 213)

**Current:** Same lax belief_id pattern. Hardcoded `"verdict": "PASS"` regardless of retracted count.

**Fix:** Add strict belief_id + identity guard (same as A-1). After `RetractCascade`:

```go
verdict := "PASS"
if retracted == 0 {
    verdict = "NO-OP"
}
```

Mirror frozen CLI logic (`cmd/solvent/main.go:202-208`).

---

## A-3 — I-7 Stage 2 case-insensitive regex

**File:** `Taskfile.yml` (Stage 2 grep)

**Current:** `grep -rnE --include='*.go' '(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]'` — case-sensitive, so lowercase SQL evades it.

**Fix:** Case-insensitive, SQL-shaped pattern:

```sh
if grep -rniE --include='*.go' '(INSERT[[:space:]]+INTO|UPDATE[[:space:]]+[a-z_]+[[:space:]]+SET|DELETE[[:space:]]+FROM|CREATE[[:space:]]+(TABLE|INDEX|DATABASE)|DROP[[:space:]]+(TABLE|INDEX|DATABASE))' cmd/solvent-mcp internal/view; then
  echo "FAIL: write SQL text in MCP/view"
  exit 1
fi
```

Must not false-FAIL on `SELECT` statements in `internal/view/view.go`.

---

## A-4 — Receipt wording

**File:** `plans/mcp/plan6_imp.md`

Change "I-7 6 adversarial injections" → "I-7 5 adversarial injections" (plan6.md table has 5 malicious + 1 clean).

---

## Adversarial Tests

### A-1/A-2 tests

| # | Test | Expected |
|---|---|---|
| 1 | cross-scenario authorize → error + zero mutation | PASS |
| 2 | cross-scenario falsify → error + zero mutation | PASS |
| 3 | missing belief_id → argument error | PASS |
| 4 | numeric belief_id → argument error | PASS |
| 5 | falsify with retracted == 0 → verdict "NO-OP" | PASS |
| 6 | valid same-scenario authorize → works | PASS |
| 7 | valid same-scenario falsify → works | PASS |

### A-3 I-7 tests

| # | Injection | Expected |
|---|---|---|
| 1 | Clean repository | PASS |
| 2 | `db.ExecContext(ctx, "UPDATE ...")` | FAIL |
| 3 | `tx.ExecContext(ctx, "UPDATE ...")` | FAIL |
| 4 | gofmt-wrapped `db.ExecContext(ctx, ...)` | FAIL |
| 5 | `db.QueryRowContext(ctx, "UPDATE ... RETURNING id")` | FAIL |
| 6 | const-hoisted SQL + `db.ExecContext` | FAIL |
| 7 | lowercase `db.QueryRowContext(ctx, "update belief set ...")` | FAIL |
| 8 | missing `cmd/solvent-mcp` | FAIL |
| 9 | missing `internal/view` | FAIL |

---

## Files Changed

| File | Change |
|---|---|
| `cmd/solvent-mcp/tools.go` | A-1 (authorize_action guard) + A-2 (falsify guard + verdict) |
| `Taskfile.yml` | A-3 (Stage 2 case-insensitive regex) |
| `plans/mcp/plan6_imp.md` | A-4 (wording correction) |

---

## Verification

| Step | Expected |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l cmd internal` | no output |
| `task --list` | 11 tasks |
| 9 I-7 scratch tests | all malicious → FAIL, clean → PASS |
| `task test` | 107 PASS, 0 FAIL, 0 SKIP |
| frozen core diff | no output |
| `git status` | no stray binary |

---

## Frozen Core

Untouched. No new files. No schema changes. No kernel changes.
