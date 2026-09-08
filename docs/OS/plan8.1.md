# Plan 8.1 — Verification Pass + F-01 Hardening

**Date:** 2026-09-08
**Repository:** HEAD `5933f44 ✨ phase 4e`
**Predecessor:** Plan 8 (evidence reconciliation)

---

## Purpose

Plan 8 established that findings F-02/F-04/F-05/F-07 from the cumulative review are stale for the current HEAD. This plan:

1. Records raw source evidence to permanently close those findings
2. Hardens F-01 (MCP trust boundary) to match the project's security discipline
3. Documents F-06 as accepted LOW
4. Runs the full verification suite
5. Prepares for a fresh independent adversarial review

---

## Part 1: Raw Evidence Closure for F-02/F-04/F-05/F-07

### 1a. F-02 — Cross-scenario guard ordering

Preserve the exact source excerpts that prove validate-before-mutate ordering.

**File:** `cmd/solvent-mcp/tools.go`

Capture `handleSolventRetireDebt` (lines 104-166) and `handleSolventPromote` (lines 168-202). The critical lines are:

```
handleSolventRetireDebt:
  line 136: snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
  line 137: if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
  line 138:     return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
  line 139: }
  line 141: st := kernel.New(db)
  line 142: if err := st.RetireDebt(ctx, beliefID, item); err != nil {

handleSolventPromote:
  line 182: snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
  line 183: if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
  line 184:     return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
  line 185: }
  line 187: st := kernel.New(db)
  line 188: if err := st.Promote(ctx, beliefID); err != nil {
```

Also preserve git history:

```bash
git log --oneline --follow -- cmd/solvent-mcp/tools.go
```

And initial commit state:

```bash
git show 64d0530:cmd/solvent-mcp/tools.go | grep -n -B2 -A5 'Cross-scenario'
```

**Closure statement:** The cross-scenario guard has been in the correct order (validate → mutate) since the initial commit `64d0530`. There was never a regression. F-02 is CLOSED.

### 1b. F-04 — I-7 gate functionality

Preserve the Taskfile I-7 gate (lines 258-272) and the `task --list` output.

```bash
task --list
```

The Taskfile gate has two stages:
- Stage 1: rejects `Exec/ExecContext/Prepare/PrepareContext/Begin/BeginTx`
- Stage 2: rejects write SQL patterns (INSERT/UPDATE/DELETE/CREATE/DROP)

**Closure statement:** The I-7 gate is functional with proper regex. The inert-gate claim (R-2) refers to an older version. F-04 is CLOSED.

### 1c. F-05 — Audit error on refusal path

Preserve the handler and the `envelopeErrorResult` function:

```go
// tools.go:190
if err := st.Promote(ctx, beliefID); err != nil {
    return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
}

// tools.go:786-801
func envelopeErrorResult(ctx context.Context, db *sql.DB, errResult map[string]interface{}, scenarioID string) *mcp.CallToolResult {
    audit, auditErr := pipeline.AuditIntent(ctx, db, scenarioID)
    // ... audit is called and included in response
}
```

**Closure statement:** `envelopeErrorResult` performs the audit on the error path. F-05 is CLOSED.

### 1d. F-07 — Taskfile syntax

Preserve `task --list` output showing all 28 tasks listed.

**Closure statement:** Taskfile parses correctly. F-07 is CLOSED.

---

## Part 2: F-01 Hardening — MCP Transport Invariant

### Current state

`cmd/solvent-mcp/main.go:637`:
```go
if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
```

The binary is hardcoded to `StdioTransport`. No environment variable, flag, or configuration exists to switch to SSE, HTTP, or any network transport. The MCP SDK offers `SSEServerTransport` and `StreamableServerTransport`, but they are never instantiated.

### Design decision

The reviewers converge on: **the MCP binary has no network-serving code path unless an explicitly enabled transport mode is selected.** The implementation should make accidental exposure difficult rather than merely documenting the intended deployment model.

I propose: add a `MCP_TRANSPORT` environment variable check. Default is `stdio`. Any other value causes immediate startup failure.

```go
// 9. Validate transport mode.
transport := os.Getenv("MCP_TRANSPORT")
if transport == "" {
    transport = "stdio"
}
if transport != "stdio" {
    log.Error("unsupported MCP transport",
        "transport", transport,
        "supported", "stdio",
        "note", "MCP is a trusted local administrative surface; network transports are not supported")
    fmt.Fprintf(os.Stderr,
        "unsupported MCP transport %q: only stdio is supported\n"+
        "MCP is a trusted local surface; network exposure transfers responsibility to the deployment boundary\n",
        transport)
    os.Exit(1)
}

// 10. Run on stdio.
if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
```

### Why this approach

| Criterion | Assessment |
|-----------|-----------|
| Consistent with project security discipline | Yes — runtime check, fail-closed, same pattern as `action_source` gate and `actor_id` mismatch rejection |
| Does not create false failures | Yes — no file-descriptor probing, no pipe checks that break under container runtimes |
| Explicit invariant | Yes — `MCP_TRANSPORT` env var documents the contract; unsupported values are rejected |
| Minimal code | ~8 lines in `main.go` |
| No architectural change | Yes — binary remains stdio-only; the check prevents accidental misconfiguration |
| Does not add authentication to MCP | Correct — `actor_id` remains attribution, not authentication |

### What this does NOT do

- Does not add authentication to MCP (reviewers explicitly rejected this)
- Does not probe file descriptors (too implementation-specific)
- Does not add SSE/HTTP transport (the SDK supports it, but it's out of scope)
- Does not change the trust model (MCP is still a trusted local surface)

---

## Part 3: F-06 Disposition

F-06 (typed argument widening) is confirmed as real: Go type assertions on `interface{}` yield zero values for wrong types. The downstream guards (`slices.Contains(FullDebt, item)`, empty-string checks) mitigate immediate consequences.

**Disposition:** F-06 = OPEN / LOW / non-blocking. Accept as-is. No code change in this pass.

---

## Part 4: Plan 8 Wording Corrections

Update `.opencode/plans/plan8.md`:

### 4a. Remove implementation from reconciliation

Delete Item 1 ("Disposition F-01 — DEFER") which recommends adding a comment to `main.go`. That recommendation belongs in Plan 8.1, not in the reconciliation document.

### 4b. Rigorous final verdict wording

Replace:

> "The system at HEAD `5933f44` has no P0 or P1 defects identified by the cumulative review."

With:

> "No current P0/P1 defect from the cumulative review has been reproduced at HEAD `5933f44`."

This is more precise because F-01 remains an architectural trust-boundary concern and F-06 remains a real low-severity issue — neither is a false finding, they're just not P0/P1.

---

## Part 5: Verification Suite

After implementing Part 2 (F-01 hardening) and Part 4 (plan8 wording):

```bash
gofmt -w cmd/solvent-mcp/main.go
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -race -count=1 -p 1 ./internal/derive ./internal/normalize ./service/executor ./service/policy ./api/openapi
task test
bash scripts/check_i7.sh
bash scripts/mcp_verify.sh
```

---

## Part 6: Fresh Independent Adversarial Review

After all changes pass verification, request a fresh independent adversarial review of HEAD with:

- Plan 8 as context (evidence reconciliation record)
- Plan 8.1 as implemented changes
- Explicit instruction: "Do not re-litigate F-02/F-04/F-05/F-07 — these are closed with raw evidence in Plan 8. Focus on new or undetected findings."

---

## Implementation Sequence

1. Update `docs/OS/plan8.md` — remove Item 1, fix final verdict wording
2. Edit `cmd/solvent-mcp/main.go` — add `MCP_TRANSPORT` check before `server.Run`
3. Run verification suite (Part 5)
4. If all pass, declare GREEN pending fresh review
5. If any fail, diagnose and fix before proceeding

---

## Files Modified

| File | Change |
|------|--------|
| `.opencode/plans/plan8.md` | Remove Item 1, fix final verdict wording |
| `cmd/solvent-mcp/main.go` | Add `MCP_TRANSPORT` env var check (~8 lines) |

---

## Acceptance Criteria

- [ ] `task --list` succeeds
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` succeeds
- [ ] `go test -count=1 -p 1 ./...` succeeds (or DB-dependent tests skipped gracefully)
- [ ] `task test` succeeds (or DB-dependent parts skip gracefully)
- [ ] `bash scripts/check_i7.sh` succeeds
- [ ] `bash scripts/mcp_verify.sh` succeeds
- [ ] `MCP_TRANSPORT=sse ./bin/solvent-mcp` exits with error message
- [ ] `MCP_TRANSPORT=stdio ./bin/solvent-mcp` (or unset) proceeds normally
- [ ] Plan 8 final verdict uses rigorous wording
- [ ] Plan 8 Item 1 (comment recommendation) removed
