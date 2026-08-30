# Plan5 Fix Pass — Implementation Summary

**Date:** 2026-08-11
**Status:** IMPLEMENTED
**Source:** `plans/mcp/plan5.md`

## What changed

### FIX 1 — Taskfile YAML (lines 65–68)
Replaced `: ` with `- ` in the echo string that follows "Post-seed state". Avoids YAML mapping ambiguity without quoting.

### FIX 2 — I-7 gate (lines 53–68)
Added directory existence check and injection test. Now requires `cmd/solvent-mcp` and `internal/view` to exist, and verifies no raw SQL writes leak into either package.

### FIX 3 — Cross-scenario guard (tools.go)
Both `handleSolventPromote` and `handleSolventRetireDebt` now call `view.GetSnapshot` before mutating. Rejects if the belief ID does not belong to the given scenario.

### FIX 4 — Audit error propagation (errors.go, tools.go)
`envelopeErrorResult` signature changed from `(db, err, scenarioID)` to `(ctx, db, err, scenarioID)`. Now propagates `pipeline.AuditIntent` errors instead of silently falling back to nil. Updated both callers (`handleSolventPromote`, `handleSolventAuthorizeAction`).

### FIX 5 — README.md
Added MCP section covering: trust boundary, build/seed commands, client config, six tools table, database refusals (M1/M2), Track 2 sequence, limitations.

### FIX 6 — .gitignore
Deleted stray `solvent-mcp` binary. Added `/solvent-mcp` and `/bin/` to `.gitignore`.

### FIX 7 — imp.md corrections
- Removed unverified VS Code/Codex claim.
- Corrected `AddTool` signature: `server.AddTool(&mcp.Tool{…}, handler)`.
- Corrected F-5 rationale: fallback resolves to nonexistent `bin/internal/...` path; normal operation requires `SOLVENT_FIXTURE_ROOT` from `.mcp.json`.

## Verification

| Check | Result |
|---|---|
| `task --list` | 11 tasks, clean YAML |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt` | PASS |
| I-7 gate | PASS |
| Frozen core diff | CLEAN |
| `task test` | 107 PASS, 0 FAIL, 0 SKIP |
