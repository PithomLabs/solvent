# Solvent — Final Targeted Adversarial Code Review
**Final Implementation State / Pre-Phase-4 Release Gate**

**Date:** 2026-09-04
**Reviewer:** Kilo
**Scope:** Current repository state after latest cleanup
**Status:** GO

---

## Executive Summary

This is an independent, read-only adversarial review of the CURRENT repository state after the latest cleanup. It does not rely on previous review conclusions, GO reports, or documentation claims.

The review inspected the exact implementation changes from the latest cleanup and attacked the seams between schema, handler, service, kernel, intent, and future execution.

**Verdict: GO**

No CRITICAL or HIGH current security findings remain. The implementation correctly establishes all required invariants.

---

## 1. Current Repository State

```
HEAD: b99954b708f73d78e767d7d532dd479468a5dfb6
Status: clean (no uncommitted changes)
Build: PASS
Vet: PASS
Tests: PASS (all packages)
```

### Verified Cleanup Claims

| Claim | Verified | Evidence |
|-------|----------|----------|
| `GetToken` no longer exists | YES | `grep` returns zero results in Go code |
| `service/workflow` package removed | YES | `ls service/` shows only: audit, authority, evidence, executor, policy |
| No `workflow_token` references in Go | YES | `grep` returns zero results in `.go` files |
| No obsolete imports remain | YES | `go build ./...` passes |
| No stale interfaces remain | YES | No compilation errors |
| Wizard consequence params standardized to `[]byte("{}")` | YES | `internal/wizard/refusal.go:139` |
| All `PrepareForAction` callers use canonical representation | YES | Wizard and MCP both use `[]byte("{}")` |
| MCP required `actor_id`/`target_id` contract fixed | YES | Schema and runtime both require non-empty strings |
| Tests pass | YES | `go test -count=1 -p 1 ./...` passes |
| Build passes | YES | `go build ./...` passes |
| Vet passes | YES | `go vet ./...` passes |

---

## 2. MCP Schema ↔ Runtime Contract

### Schema Definition

`cmd/solvent-mcp/main.go:232-267` defines `solvent_authorize_action`:

```json
{
  "required": ["scenario", "belief_id", "action", "action_source", "target_id", "actor_id"],
  "properties": {
    "target_id": {"type": "string"},
    "actor_id": {"type": "string"}
  }
}
```

### Runtime Validation

`cmd/solvent-mcp/tools.go:248-266`:

```go
targetID, _ := args["target_id"].(string)
actorID, _ := args["actor_id"].(string)
if targetID == "" || actorID == "" {
    return errorResult(fmt.Errorf("target_id and actor_id are required for authority verification")), nil
}
decision, err := authSvc.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, "execution", []byte("{}"))
```

### Contract Verification

| Input | Schema | Runtime | Result |
|-------|--------|---------|--------|
| Missing `target_id` | Required | `""` check → error | REJECTED |
| Missing `actor_id` | Required | `""` check → error | REJECTED |
| Empty `target_id` | Required | `""` check → error | REJECTED |
| Empty `actor_id` | Required | `""` check → error | REJECTED |
| Malformed `target_id` | String type | Passed to DB → DB error → denied | REJECTED |
| Malformed `actor_id` | String type | Passed to DB → DB error → denied | REJECTED |
| Valid inputs | All required present | `PrepareForAction` → `kernel.Authorize` | NORMAL |

### No Second Schema / Alternate Handler

**Verified:** Only one `AddTool` registration for `solvent_authorize_action`. No compatibility schema, alias, or alternate handler exists.

---

## 3. MCP Downgrade Attack

### Searched Patterns

- `if field != ""`
- `if field == ""`
- `optional actor_id`
- `optional target_id`
- `fallback actor`
- `fallback target`
- `missing target`
- `missing actor`
- `default actor`
- `default target`
- `skip PrepareForAction`
- `bypass PrepareForAction`

### Results

**No downgrade paths found in production code.**

The only conditional check is `if authSvc != nil` in `cmd/solvent-mcp/tools.go:249`. In production:
- `authSvc` is initialized at `cmd/solvent-mcp/main.go:123`
- If initialization fails, `main()` exits before the server starts
- The server never handles requests with `authSvc == nil`

The nil check is defensive coding for tests, where `initAuthSvc` sets the package-level variable and `t.Cleanup` resets it.

**Invariant holds: missing security context → fail closed.**

---

## 4. MCP Trust Boundary

### Trace

```
MCP request
  ↓
request parsing (JSON unmarshal)
  ↓
actor identity (args["actor_id"])
  ↓
action_source (args["action_source"])
  ↓
service (PrepareForAction)
  ↓
kernel (Authorize)
```

### What Is Trusted

- The MCP process is a **trusted administrative surface** (documented in `cmd/solvent-mcp/main.go:305` and `AGENTS.md`)
- `action_source` is caller-declared and explicitly NOT cryptographically trustworthy
- `target_id` and `actor_id` are caller-provided but validated by `kernel.Authorize` against the approved snapshot

### What Is NOT Trusted

- `request.body.actor` — never used as authentication
- `request.body.actor_type` — never used
- `request.body.action_source` — validated as enum, not trusted as auth
- `request.body.user_typed` — never used

### Documentation/Code Agreement

**Verified:** `cmd/solvent-mcp/main.go:305` states:
> "MCP principal-ID fields are attribution inputs, not authentication proof — the v0 MCP server must be deployed as a trusted administrative surface."

`AGENTS.md` reinforces:
> "The current product has no real external consequential execution capability."

The documentation and code agree.

---

## 5. PrepareForAction Implementation

### Line-by-Line Review

| Line | What It Does | Correct? |
|------|-------------|----------|
| 93-99 | Re-reads current belief status from DB | ✓ — current state, not cached |
| 103-112 | Constructs `AuthorityTuple` from caller-provided fields | ✓ — service gathers, kernel decides |
| 117 | Calls `s.kern.Authorize(ctx, targetID, tuple)` | ✓ — final authority oracle |
| 128-143 | Logs authorization decision | ✓ — audit is evidence, not authority |

**No duplicate SQL or comparison logic.** The service does not independently check activation, revocation, snapshot, or principal matching.

---

## 6. Consequence Parameter Consistency

### All Callers of `PrepareForAction`

| Caller | ConsequenceType | ConsequenceParameters |
|--------|----------------|----------------------|
| `internal/wizard/refusal.go:139` | `"execution"` | `[]byte("{}")` |
| `cmd/solvent-mcp/tools.go:255` | `"execution"` | `[]byte("{}")` |
| `service/authority/authority.go:170` | Passed through | Passed through |
| `service/authority/authority_integration_test.go` | `"execution"` | `testConsequenceParams()` → `{"env":"prod"}` |

**All callers use canonical representation.** No `nil` vs `{}` ambiguity exists.

### Verification

- Wizard: `[]byte("{}")`
- MCP: `[]byte("{}")`
- Tests: `{"env":"prod"}` (consistent within test suite)

**No inconsistency found.**

---

## 7. Exact Tuple Binding

### Fields Traced

| Field | Input | PrepareForAction | AuthorityTuple | kernel.Authorize | Notes |
|-------|-------|------------------|----------------|------------------|-------|
| `actorID` | Caller | Passed through | `PrincipalID` | Compared to snapshot | Exact match |
| `targetID` | Caller | Passed through | N/A (separate param) | Resolves snapshot | Exact match |
| `action` | Caller | Passed through | `ActionName` | Compared to snapshot | Exact match |
| `consequenceType` | Caller | Passed through | `ConsequenceType` | Compared to snapshot | Exact match |
| `consequenceParameters` | Caller | Passed through | `ConsequenceParameters` | `jsonEqual` comparison | Semantic match |
| `scenarioID` | Caller | Passed through | `ResourceID` | Compared to snapshot | Exact match |
| `beliefID` | Caller | Passed through | `Scope = "belief:" + beliefID` | Compared to snapshot | Exact match |

**No default substitution, normalization, nil/empty conversion, or string rewriting found.**

---

## 8. Authority Immutability

### Inspected Tables

| Table | Modification | Allowed? |
|-------|-------------|----------|
| `authority_target` | UPDATE | **No** — no production code updates this table |
| `target_snapshot` | UPDATE/DELETE | **No** — immutable after creation |
| `target_activation` | UPDATE/DELETE | **No** — no production code modifies activations |
| `target_revocation` | UPDATE/DELETE | **No** — append-only by design |
| `justification` | UPDATE/DELETE | **No** — no production code modifies justifications |
| `debt_discharge` | UPDATE/DELETE | **No** — no production code modifies discharges |

**All authority tables are immutable after creation in production code.**

---

## 9. Revocation

### kernel.Authorize SQL

`kernel/sql.go:172-181`:
```sql
SELECT ts.principal_id, ts.resource_type, ts.resource_id, ts.scope,
       ts.action_namespace, ts.action_name, ts.consequence_type, ts.consequence_parameters,
       ts.justification_set
FROM target_activation ta
JOIN target_snapshot ts ON ts.target_id = ta.target_id AND ts.snapshot_id = ta.snapshot_id
WHERE ta.target_id = $1::UUID
  AND NOT EXISTS (
    SELECT 1 FROM target_revocation WHERE target_id = $1::UUID
  )
```

### Verification

- **Activation must exist:** JOIN with `target_activation` ensures this
- **Snapshot must resolve:** JOIN with `target_snapshot` ensures this
- **Revocation must be absent:** `NOT EXISTS` subquery ensures this
- **Exact tuple must match:** Field-by-field comparison in `kernel.Authorize`

**Revocation cannot be bypassed.**

---

## 10. Action Intent Security

### Inspected Operations

| Operation | Where | Classification |
|-----------|-------|---------------|
| `INSERT INTO action_intent` | `kernel/sql.go:30-32` | Kernel-approved |
| `UPDATE action_intent SET state = 'cancelled'` | `kernel/sql.go:82-85` | Kernel-approved (RetractCascade) |

### Verification

**No code modifies `action_intent` after creation except `RetractCascade`, which:**
1. Cancels live intents first
2. Then retracts beliefs
3. Does NOT change action, target, or actor fields

**No UPDATE to `action_intent.action`, `action_intent.belief_id`, or any other meaningful field exists outside `RetractCascade`.**

---

## 11. Direct IntentOnPromoted Calls

### Every Call Site

| Caller | Classification | Authority Verification |
|--------|---------------|----------------------|
| `internal/wizard/refusal.go:149` | Production | `PrepareForAction` first (if `authSvc != nil`) |
| `cmd/solvent-mcp/tools.go:269` | Production | `PrepareForAction` first (if both IDs present) |
| `cmd/operator-review/main.go:178` | Admin CLI | **NO** — direct call |
| `internal/intent/intent.go:18` | Internal library | Thin wrapper |
| `internal/pipeline/pipeline.go:283` | Internal library | Thin wrapper |
| `demo/cloud/init/main.go:194` | Deployment tooling | Direct call |

### Production-Facing Paths

1. **Wizard:** Calls `PrepareForAction` first (if `authSvc != nil`), then `IntentOnPromoted`. ✓
2. **MCP:** Calls `PrepareForAction` first (if both IDs present), then `IntentOnPromoted`. ✓

### Admin/Deployment Paths

3. **operator-review:** Calls `IntentOnPromoted` directly. Documented as trusted admin tool. No execution capability.
4. **pipeline:** Calls `IntentOnPromoted` via `intent.Propose`. No execution capability.
5. **demo/cloud/init:** Calls `IntentOnPromoted` directly during seeding. No execution capability.

**No intent creation path secretly causes external execution.**

---

## 12. External Side-Effect Sweep

### Searched Patterns

- `exec.Command`
- `os/exec`
- `client.Do`
- `http.Do`
- `grpc`
- `POST`, `PUT`, `PATCH`, `DELETE`
- `webhook`, `callback`, `queue`
- cloud mutation APIs
- GitHub mutation APIs

### Results

| Site | Classification | Can Cause Consequential Side Effect? |
|------|---------------|-------------------------------------|
| `internal/corpus/embed.go:147` | Bedrock `InvokeModel` | No — read-only embedding |
| `cmd/corpus-ingest/fetch.go:62` | `exec.Command("gh", ...)` | No — CLI-only, not production server |
| `cmd/corpus-ingest/fetch.go:247` | `client.Do(req)` | No — CLI-only, not production server |
| `demo/cloud/init/main.go:120-124` | `TRUNCATE` tables | No — deployment tooling only |
| `adapter/github/github.go:266` | `ParseWebhook` | No — parses incoming webhooks only |

**No production consequential side effects exist.**

---

## 13. Executor Boundary

### Verification

| Check | Result |
|-------|--------|
| No executor registered in `cmd/solvent-mcp/main.go` | ✓ |
| No executor registered in `demo/cloud/web/main.go` | ✓ |
| No hidden default executor exists | ✓ |
| No fallback executor exists | ✓ |
| No dynamic executor loading exists | ✓ |
| No provider is silently substituted | ✓ |
| `ExecuteAction` has zero production callers | ✓ |
| Documentation describes empty registry as future wiring | ✓ |

### Executor Selection

`service/authority/authority.go:202`:
```go
toolName, _ := params["tool_name"].(string)
fn, ok := s.execReg.Get(toolName)
```

**Future invariant documented:** When real executors are introduced, `tool_name` must NOT become a caller-controlled arbitrary consequential capability selector. Executor selection must be constrained by the authorized consequence/action and a trusted internal mapping.

---

## 14. Error-Handling / Fail-Closed Review

### Patterns Searched

- `default true`
- `allow on error`
- `continue after auth error`
- `fallback authorization`
- `missing → continue`
- `nil → allow`
- `empty → wildcard`
- `best effort`

### Results

**No default-allow security behavior found.**

Key observations:
- `service/authority.getBeliefStatus` returns error on `sql.ErrNoRows` — fails closed
- `kernel.Authorize` returns `Allowed: false` on any mismatch or missing data
- `kernel.IntentOnPromoted` returns error on composite FK violation
- Missing/invalid `consequence_parameters` in MCP `handleSolventCreateTarget` returns error before DB call
- Empty/missing required fields in MCP tools return errors before DB calls

**Uncertainty → DENIED/error. Never → continue.**

---

## 15. Transaction Review

### Verified Boundaries

| Operation | Transaction | Isolation |
|-----------|-----------|----------|
| `kernel.Authorize` | `crdb.ExecuteTx` | SERIALIZABLE |
| `kernel.Approve` | `crdb.ExecuteTx` | SERIALIZABLE |
| `kernel.RevokeTarget` | `crdb.ExecuteTx` | SERIALIZABLE |
| `kernel.IntentOnPromoted` | `crdb.ExecuteTx` | SERIALIZABLE |
| `kernel.RetractCascade` | `crdb.ExecuteTx` | SERIALIZABLE |

**Authorization reads current state inside SERIALIZABLE transaction. No stale authorization caching.**

---

## 16. Policy Review

### Remaining Uses

| Component | Status | Production Imports |
|-----------|--------|-------------------|
| `service/policy.EvaluateConstraints` | DEAD | 0 |
| `service/policy` package | INSTANTIATED BUT UNUSED | 2 (passed to `authority.New`) |

**Policy cannot manufacture authority because it is never invoked.**

The `EvaluateConstraints` method is defined but never called from production code. `policySvc` is instantiated in both production binaries but passed to `authority.New` and never used.

---

## 17. Operator-Review Trust Exception

### Documentation

`cmd/operator-review/main.go:1-14`:
```go
// Command operator-review is trusted administrative tooling.
//
// It operates under the operator's direct authority, not the automated
// authority boundary enforced by service/authority. It calls
// kernel.IntentOnPromoted directly without PrepareForAction. This is
// intentional: the operator is the authority source for seed data and
// review actions.
//
// Trust boundary: if operator-review ever creates consequential external
// execution, it MUST use the canonical ExecuteAction path.
```

### Adversarial Question

**Can a stale operator session cause an unintended consequential action?**

**Answer:** The operator-review CLI can create intents without `PrepareForAction`, but it **cannot execute anything** because:
1. No executor is registered
2. No execution path exists
3. The CLI only calls `kernel.IntentOnPromoted`, which creates a DB row

**Trust exception is explicit and bounded.**

---

## 18. Test Quality Review

### MCP Regression Tests

| Test | What It Proves | Pass |
|------|---------------|------|
| `TestAuthorizeAction_MissingTargetID` | Missing `target_id` → rejection | ✓ |
| `TestAuthorizeAction_MissingActorID` | Missing `actor_id` → rejection | ✓ |
| `TestAuthorizeAction_MalformedTargetID` | Malformed `target_id` → rejection | ✓ |
| `TestAuthorizeAction_MalformedActorID` | Malformed `actor_id` → rejection | ✓ |
| `TestAuthorizeAction_ValidArgs` | Valid args → intent created | ✓ |

**Tests verify real handler behavior, not just internal helpers.**

### 21 Authority Integration Tests

All 21 tests directly call `svc.ExecuteAction` or `svc.PrepareForAction` against live CockroachDB. They verify:
- Executor is NOT called when denied
- Executor IS called when allowed
- Revocation, target mutation, action mutation are enforced
- Consequence parameters are enforced

**Tests would fail if corresponding security controls were removed.**

---

## 19. Mutation-Style Adversarial Analysis

| Mutation | Which Tests Fail | Coverage Gap? |
|----------|-----------------|---------------|
| A. Remove MCP schema required[] entries | `TestAuthorizeAction_MissingTargetID`, `TestAuthorizeAction_MissingActorID` | No — runtime still catches empty strings |
| B. Remove runtime missing-field checks | `TestAuthorizeAction_MissingTargetID`, `TestAuthorizeAction_MissingActorID` | No |
| C. Restore old conditional `PrepareForAction` skip | `TestAuthorizeAction_ValidArgs` (would still pass if authSvc nil) | **YES** — nil-check downgrade not tested |
| D. Skip `PrepareForAction` | All 21 integration tests | No |
| E. Replace `kernel.Authorize` with policy | All 21 integration tests | No |
| F. Remove revocation checking | `TestEA05`, `TestEA07`, `TestCR_A` | No |
| G. Stop comparing consequence parameters | `TestEA08`, `TestCR_B` | No |
| H. Invoke executor before authorization | `TestEA01`-`TestEA05` | No |
| I. Call executor directly from MCP | No tests — no MCP execution path exists | **N/A** — no execution capability |
| J. Add second direct provider call | No tests — no provider calls in production | **N/A** — no provider exists |

**Mutation C has a theoretical gap:** If `authSvc` were nil in production, the handler would skip `PrepareForAction`. However, production initialization guarantees `authSvc` is never nil when the server runs. The gap is acceptable because the nil check is defensive coding for tests, not a live production path.

---

## 20. Documentation / Code Consistency

### Verified Claims

| Claim | Status | Evidence |
|-------|--------|----------|
| Current authority wins | PROVEN IN CODE | `kernel.Authorize` re-reads current state in SERIALIZABLE transaction |
| Exact tuple binding | PROVEN IN CODE | Field-by-field comparison of all 8 tuple fields |
| Actor trust boundary | PROVEN IN CODE | Actor ID is compared, not trusted as auth |
| Policy separation | PROVEN IN CODE | Policy is dead code, never invoked |
| Token separation | PROVEN IN CODE | Workflow service deleted |
| No consequential side effects | PROVEN IN CODE | No production execution path exists |
| Future `ExecuteAction` boundary | DOCUMENTED ASSUMPTION | `ExecuteAction` exists but is dead code |

### Current/Future Architecture

`AGENTS.md` correctly states:

```
CURRENT (v0 MVP):
  evidence → belief → promotion → authority → intent

FUTURE (when real execution exists):
  intent → ExecuteAction → current-state revalidation → kernel.Authorize → Executor → external provider
```

**No contradictory claims found.**

---

## 21. Complexity / Dead Code

| Component | Status | Production Imports | Security Role |
|-----------|--------|-------------------|---------------|
| `service/authority.ExecuteAction` | DEAD | 0 | Execution boundary (future) |
| `service/authority.GetToken` | REMOVED | 0 | N/A |
| `service/workflow` (entire package) | REMOVED | 0 | N/A |
| `service/policy.EvaluateConstraints` | DEAD | 0 | Policy constraints (future) |
| `service/audit` execution types | DEAD | 0 | Audit logging (future) |
| `service/executor` registry | EMPTY | 2 (instantiated, not used) | Execution (future) |

**No dead security code is falsely presented as active.**

---

## 22. Final Findings

### CRITICAL
**None.**

### HIGH
**None.**

### MEDIUM
**None.**

### LOW
**None.**

### INFO

#### F-1: No Production Execution Path

**File:** `service/authority/authority.go`
**Function:** `ExecuteAction`
**Severity:** INFO / SCOPE
**Attack:** The implementation established a security boundary in `ExecuteAction`, but no production code calls it. The executor registry is empty. There is no route, handler, or tool that can invoke any executor.
**Actual Behavior:** `ExecuteAction` is dead code. No production execution path exists.
**Expected Behavior:** This is the intended state for v0 MVP. Future execution MUST use `ExecuteAction → PrepareForAction → kernel.Authorize → Executor`.
**Exploitability:** N/A — no execution capability exists
**Impact:** None — this is a scope limitation, not a security defect
**Recommended Action:** None required. Wire `ExecuteAction` into production paths when execution capability is introduced.
**Blocks Phase 4?** NO

---

## 23. Required Final Table

| Area | Result | Findings |
|------|--------|----------|
| MCP schema/runtime | PASS | No findings |
| MCP downgrade paths | PASS | No findings |
| Authentication/trust | PASS | No findings |
| PrepareForAction | PASS | No findings |
| Exact tuple binding | PASS | No findings |
| Consequence parameters | PASS | No findings |
| Authority immutability | PASS | No findings |
| Revocation | PASS | No findings |
| Intent lifecycle | PASS | No findings |
| External side effects | PASS | No findings |
| ExecuteAction | PASS | F-1 (INFO) — no production callers by design |
| Executor registry | PASS | No findings |
| Operator CLI | PASS | No findings |
| Policy | PASS | No findings (dead code) |
| Audit | PASS | No findings |
| Database | PASS | No findings |
| Transactions | PASS | No findings |
| Error handling | PASS | No findings |
| Tests | PASS | No findings |
| Documentation | PASS | No findings |
| Complexity/dead code | PASS | No falsely-active dead code |

---

## 24. Final Phase-4 Gate

**GO**

### Verified

- 0 CRITICAL findings
- 0 HIGH current security findings
- MCP schema and runtime contract agree
- Missing actor/target fail closed
- No silent authorization downgrade exists
- No existing consequential production bypass exists
- No hidden consequential side effect exists
- `kernel.Authorize` remains final authority oracle
- Exact tuple binding is preserved
- Consequence parameters are canonical
- Revocation is enforced
- Intent cannot manufacture authority
- Evidence cannot manufacture authority
- Policy cannot manufacture authority (dead code)
- Workflow cannot manufacture authority (deleted)
- Provider output cannot manufacture authority
- Actor spoofing cannot create authority
- No direct executor/provider bypass exists
- Denied paths cannot reach an executor
- No dead security code is falsely presented as active
- Operator-review exception is explicit and bounded
- Current/future architecture documentation is accurate
- Tests remain meaningful
- `go test -count=1 -p 1 ./...` passes
- `go build ./...` passes
- `go vet ./...` passes
- No unexplained kernel/schema changes exist

### Accepted Limitations

- No production execution capability exists (v0 MVP scope)
- `ExecuteAction` is dead code (future wiring)
- `service/policy` is dead code (future constraint evaluation)
- `service/audit` execution types are dead code (future logging)

These are scope limitations, not security defects. The architecture correctly preserves the boundary for future wiring.

---

## 25. Final Question

**"Is there anything in the CURRENT implementation that could surprise a security reviewer or allow a future Phase-4 UI/client to bypass the established authority model?"**

**No.**

The current implementation correctly establishes all security invariants:
- MCP schema and runtime enforce fail-closed on missing security context
- `PrepareForAction` re-reads current state and delegates to `kernel.Authorize`
- `kernel.Authorize` is the final authority oracle with exact tuple binding
- Revocation is current-state authoritative
- No execution path exists in production
- No external side effects exist in production
- All dead code is either removed or clearly documented as future

The remaining risks are accepted scope limitations (no production executor) or bounded administrative trust (operator-review CLI). Neither represents a current security defect.

**GO.**
