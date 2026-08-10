# Plan7 Hardening Fixes — Implementation Summary

**Date:** 2026-08-11
**Status:** IMPLEMENTED
**Source:** `plans/mcp/plan7.md`, `plans/mcp/plan7_review.md`

## What changed

### A-1 — `solvent_authorize_action` guard (tools.go)

Added strict belief_id + identity-only scenario ownership guard before `IntentOnPromoted`. Mirrors the pattern already proven in `handleSolventPromote` and `handleSolventRetireDebt`.

Rejects: missing belief_id, non-string belief_id, empty belief_id, cross-scenario belief_id. Identity-only check — no debt/status/intent inspection. M2 still originates in CockroachDB FK.

### A-2 — `solvent_falsify` guard + NO-OP verdict (tools.go)

Added strict belief_id + identity-only scenario ownership guard before `RetractCascade`. After the kernel call, verdict is `"NO-OP"` when `retracted == 0`, matching the frozen CLI logic.

### A-3 — I-7 Stage 2 case-insensitive regex (Taskfile.yml)

Replaced case-sensitive `(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]` with case-insensitive SQL-shaped pattern:

```
(INSERT[[:space:]]+INTO|UPDATE[[:space:]]+[a-z_]+[[:space:]]+SET|DELETE[[:space:]]+FROM|CREATE[[:space:]]+(TABLE|INDEX|DATABASE)|DROP[[:space:]]+(TABLE|INDEX|DATABASE))
```

Catches lowercase SQL on the query path. Legitimate `SELECT` statements in `internal/view/view.go` pass.

### A-4 — Receipt wording (plan6_imp.md)

Corrected "6 adversarial injections" → "8 adversarial cases (6 injections + 2 missing dirs)".

## I-7 Verification (9 cases)

| # | Case | Result |
|---|---|---|
| 1 | Clean repository | PASS |
| 2 | direct ExecContext | FAIL (Stage 1) |
| 3 | tx.ExecContext | FAIL (Stage 1) |
| 4 | gofmt-wrapped ExecContext | FAIL (Stage 1) |
| 5 | QueryRowContext write | FAIL (Stage 2) |
| 6 | const-hoisted SQL | FAIL (Stage 1) |
| 7 | lowercase QueryRowContext | FAIL (Stage 2) |
| 8 | missing cmd/solvent-mcp | FAIL |
| 9 | missing internal/view | FAIL |

## Verification

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l cmd internal` | no output |
| `task --list` | 11 tasks |
| `task test` | 107 PASS, 0 FAIL, 0 SKIP |
| Frozen core diff | CLEAN |
| `git status` | no stray binary |
