# Plan: prompt4 Fix Pass — plan5.md

**Date:** 2026-08-11
**Status:** READY FOR IMPLEMENTATION
**Source:** `plans/mcp/prompt4.md`, `plans/mcp/imp_review.md`
**Scope:** Mechanical corrections only. No architectural changes. Frozen core untouched.

---

## Fix 1 — Taskfile YAML parse failure

**File:** `Taskfile.yml:67`
**Severity:** BLOCKING

**Problem:** Unquoted scalar contains `: ` after "Post-seed state", YAML parses it as a mapping.

```yaml
- echo "Seed complete. Post-seed state: 1 belief (postulated, entered, 6 debts), 1 evidence, 0 intents, audit 0."
```

**Fix:** Replace `: ` with `- ` (avoids YAML mapping ambiguity without adding quoting complexity):

```yaml
- echo "Seed complete. Post-seed state - 1 belief (postulated, entered, 6 debts), 1 evidence, 0 intents, audit 0."
```

**Verify:** `task --list` — all 8 tasks must resolve.

---

## Fix 2 — Repair the I-7 MCP boundary gate

**File:** `Taskfile.yml:78-84`
**Severity:** HIGH

**Problem:** `grep -v '^\|// \|/\*\|\*\/'` — `^` matches every line, so `grep -v` discards everything. Gate always passes.

**Fix:** Replace with fail-closed version that:
1. Asserts both directories exist before grepping
2. Greps for actual Go database-write call patterns (`ExecContext|Exec` + write SQL)
3. Does not filter on comments (not needed — current sources have no such comments)

```yaml
# I-7 MCP boundary — no raw writes in cmd/solvent-mcp or internal/view
- |
  for d in cmd/solvent-mcp internal/view; do
    [ -d "$d" ] || { echo "FAIL: $d missing"; exit 1; }
  done
  if grep -rnE --include='*.go' '(ExecContext|Exec)\(.*(INSERT|UPDATE|DELETE|CREATE|DROP)' cmd/solvent-mcp internal/view; then
    echo "FAIL: raw writes in MCP/view"
    exit 1
  else
    echo "I-7 MCP boundary: PASS"
  fi
```

**Verify (mandatory injection test):**
1. Create scratch copy, inject `db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")` → gate MUST FAIL
2. Clean copy → gate MUST PASS
3. Missing directory → gate MUST FAIL

---

## Fix 3 — Prevent cross-scenario belief mutation

**File:** `cmd/solvent-mcp/tools.go`
**Severity:** MEDIUM

**Problem:** `solvent_promote` and `solvent_retire_debt` accept scenario + belief_id but the kernel methods operate on belief_id without scenario. A caller can mutate a track1 belief while labeling it as track2.

**Fix:** Before invoking the kernel mutation, resolve/validate that the supplied belief belongs to the supplied scenario using `view.GetSnapshot`. This is an identity/ownership lookup, NOT an authorization rule.

Apply to:
- `handleSolventPromote` — before `st.Promote(ctx, beliefID)`
- `handleSolventRetireDebt` — before `st.RetireDebt(ctx, beliefID, item)`

Pattern:
```go
snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
if err != nil || len(snap.Beliefs) == 0 {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

Do NOT add scenario to kernel methods. Do NOT inspect debt/status/state.

Also check `solvent_authorize_action` and `solvent_falsify` — `IntentOnPromoted` takes scenarioID so it already scopes correctly. `RetractCascade` takes scenarioID so it already scopes correctly. Only `promote` and `retire_debt` are affected.

**Verify:** Attempt `solvent_promote(scenario="track2", belief_id=<track1 UUID>)` → clean error, no mutation, track1 belief unchanged.

---

## Fix 4 — Do not fabricate audit zero on refusal

**File:** `cmd/solvent-mcp/tools.go:246-257`
**Severity:** LOW

**Problem:** `envelopeErrorResult` uses `context.Background()` and discards the audit error (`audit, _ := ...`), fabricating `live_on_nonpromoted: 0` on the refusal path.

**Fix:** Change `envelopeErrorResult` to accept `ctx` and propagate the audit error:

```go
func envelopeErrorResult(ctx context.Context, db *sql.DB, errResult map[string]interface{}, scenarioID string) *mcp.CallToolResult {
    audit, auditErr := pipeline.AuditIntent(ctx, db, scenarioID)
    envelope := map[string]interface{}{
        "result": errResult,
    }
    if auditErr != nil {
        envelope["audit"] = nil
        envelope["audit_error"] = auditErr.Error()
    } else {
        envelope["audit"] = map[string]interface{}{"live_on_nonpromoted": audit}
    }
    b, _ := json.Marshal(envelope)
    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
    }
}
```

Update all callers of `envelopeErrorResult` to pass `ctx`.

---

## Fix 5 — README MCP documentation

**File:** `README.md`
**Severity:** MEDIUM

**Fix:** Add MCP section after the CLI quick start (after "Run Tests"). Content per plan3 §12.2 / plan4.1 Phase 12:

1. What MCP demonstrates (database as final authority)
2. Trust boundary diagram
3. Prerequisites (Docker, Go, Task, CockroachDB running)
4. Build (`task mcp:build`)
5. Seed (`task mcp:seed`)
6. Client configuration (`.mcp.json`)
7. Six tools with brief descriptions
8. Two refusal demonstrations (M1: 23514, M2: 23503)
9. Track 2 sequence overview
10. Honest limitation (single-belief retraction, belief_edge empty)
11. Note that MCP is a thin adapter with no business logic

Do NOT claim VS Code/Codex/Claude Code was tested.

---

## Fix 6 — .gitignore / stray binary

**Files:** `.gitignore`, repository root
**Severity:** MEDIUM

**Fix:**
1. Delete `solvent-mcp` at repository root (16 MB stray binary)
2. Add to `.gitignore`:
   ```
   /bin/
   /solvent-mcp
   ```

**Verify:** `git status --short` does not show the generated root binary.

---

## Fix 7 — Correct imp.md record

**File:** `plans/mcp/imp.md`
**Severity:** LOW

**Fixes:**
1. Correct F-5 rationale: executable-relative fallback does NOT work when binary is at `bin/solvent-mcp` — it resolves to `bin/internal/derive/testdata/etcd_real` which doesn't exist. Remove the fallback and require `SOLVENT_FIXTURE_ROOT`.
2. Fix AddTool description: `server.AddTool(&mcp.Tool{…}, handler)`, not `server.AddTool(server, toolDef, handler)`.
3. Remove unverified VS Code/Codex claims.
4. Do not fabricate new verification receipts.

---

## Optional Fixes (only if clean)

### R-7 — Strict type validation (optional)
Distinguish "absent optional field" from "present field with wrong type" in type assertions. Return validation error for wrong types. Do not widen queries silently.

### R-9 — Nil slices (optional)
`Beliefs`/`Intents` are nil slices when empty → marshal to `null` not `[]`. Initialize with empty slices in `GetSnapshot`.

### R-9 — Unused db parameter
`envelopeResult(db *sql.DB, …)` never uses `db`. Remove the parameter.

---

## Verification Order (from prompt4.md)

1. `git diff --stat HEAD -- db/ internal/kernel ...` → frozen core untouched
2. `task --list` → all 8 tasks resolve
3. `unset FABLE_TEST_DSN && task test` → 107 PASS
4. `task demo:track1 && task demo:replay && task demo:track2 && task inspect` → all exit 0
5. `task mcp:build` → binary at `bin/solvent-mcp`
6. `task mcp:seed` → seeds correctly
7. MCP startup → server starts on stdio
8. `tools/list` → exactly 6 tools
9. M1 through MCP → SQLSTATE 23514 / promoted_is_debt_free
10. M2 through MCP → SQLSTATE 23503 / gate
11. Track 2 end-to-end through MCP
12. Cross-scenario mismatch → clean error, no mutation
13. Audit failure → `"audit": null` not fabricated zero
14. `go build ./... && go vet ./... && gofmt -l cmd internal` → clean
15. I-7 gate → clean source PASS, injected write FAIL, missing dir FAIL
16. `.gitignore` and `git status` → no stray binary
17. One MCP client rehearsal (if practical)

---

## Files Modified

| File | Change |
|---|---|
| `Taskfile.yml` | Quote echo (FIX 1), fix I-7 gate (FIX 2) |
| `cmd/solvent-mcp/tools.go` | Cross-scenario guard (FIX 3), audit error handling (FIX 4) |
| `README.md` | Add MCP section (FIX 5) |
| `.gitignore` | Add `/bin/`, `/solvent-mcp` (FIX 6) |
| `plans/mcp/imp.md` | Correct F-5, AddTool, remove unverified claims (FIX 7) |

## Files NOT Modified

- `db/` — frozen
- `internal/kernel/` — frozen
- `internal/belief/` — frozen
- `internal/intent/` — frozen
- `internal/derive/` — frozen
- `internal/normalize/` — frozen
- `internal/pipeline/` — frozen
- `cmd/solvent/` — frozen
- `cmd/operator-review/` — frozen
- `cmd/solvent-mcp/main.go` — no changes needed (fixture fallback behavior is safe — fails closed)
