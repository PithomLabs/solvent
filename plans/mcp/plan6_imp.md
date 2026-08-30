# Plan6 Hardening Pass — Implementation Summary

**Date:** 2026-08-11
**Status:** IMPLEMENTED
**Source:** `plans/mcp/plan6.md`, `plans/mcp/plan6_rework_prompt.md`

## What changed

### F-1 — I-7 Gate (Taskfile.yml)

Replaced single-line grep with two-stage gate:

- **Stage 1:** Rejects write/transaction entry points (`Exec`, `ExecContext`, `Prepare`, `PrepareContext`, `Begin`, `BeginTx`) in `cmd/solvent-mcp` and `internal/view`.
- **Stage 2:** Rejects write SQL text (`INSERT|UPDATE|DELETE|CREATE|DROP` followed by whitespace) in `cmd/solvent-mcp` and `internal/view`.

Legitimate `QueryRowContext` SELECTs in `internal/view/view.go` pass because they contain `SELECT`, not write SQL.

### F-2 — Strict belief_id in promote/retire_debt (tools.go)

Changed `beliefID, _ := args["belief_id"].(string)` to strict type assertion. Both `handleSolventPromote` and `handleSolventRetireDebt` now:

1. Reject missing/non-string/empty belief_id with a clean argument error.
2. Assert identity after snapshot lookup: `len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID`.

No business-rule inspection. Kernel/database remains authoritative.

### F-3 — Strict belief_id in ledger (tools.go)

Changed `handleSolventLedger` to distinguish "absent" from "present but wrong type":

- No `belief_id` → normal scenario-wide ledger.
- `belief_id` present but not a string or empty → argument error.

### F-4 — Exact test count (plan5_imp.md)

Changed "107+ tests passing" → "107 PASS, 0 FAIL, 0 SKIP".

## Verification

| Check | Result |
|---|---|
| `task --list` | 11 tasks, clean YAML |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l cmd internal` | no output |
| I-7 clean repository | PASS |
| I-7 8 adversarial cases (6 injections + 2 missing dirs) | all FAIL (correctly) |
| I-7 clean source after scratch tests | PASS |
| `task test` | 107 PASS, 0 FAIL, 0 SKIP |
| Frozen core diff | CLEAN |
| `git status` | no stray binary |
