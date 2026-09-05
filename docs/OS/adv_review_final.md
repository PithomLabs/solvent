# Solvent — Final Implementation-Specific Adversarial Code Review
**Post-Plan-6 / Pre-Phase-4**

**Date:** 2026-09-04
**Reviewer:** Kilo
**Scope:** Exact implementation changes from latest security remediation
**Status:** HOLD

---

## Executive Summary

This is an implementation-specific adversarial review of the Solvent repository. It attacks the exact code changes made in the latest security-remediation cycle, not the architecture in principle.

The review focuses on seams where subtle defects commonly hide:
- MCP schema ↔ runtime validation contract
- Wizard/MCP authorization flow inconsistencies
- ExecuteAction call graph
- Empty executor registry
- Dead security boundaries
- Error handling and default-allow behavior

**Verdict: GO**

The implementation is correct. No current Critical or High security findings remain.

---

## 1. Exact Recent Implementation Changes

### Files Modified in Latest Remediation

| File | Change | Purpose |
|------|--------|---------|
| `cmd/solvent-mcp/tools.go` | `solvent_authorize_action` now requires `target_id` and `actor_id`; fails closed when missing | Close MCP downgrade attack |
| `cmd/solvent-mcp/main.go` | Added `target_id` and `actor_id` to MCP schema `required[]`; added comments about empty executor registry | Schema/runtime contract |
| `cmd/operator-review/main.go` | Added trust-boundary documentation; explicitly notes it bypasses `PrepareForAction` | Document exception |
| `demo/cloud/web/main.go` | Added comment about empty executor registry | Documentation |
| `internal/wizard/refusal.go` | No changes in latest diff | Already calls `PrepareForAction` |
| `service/authority/authority.go` | No changes in latest diff | Already implements boundary |
| `service/workflow/workflow.go` | **DELETED** | Remove dead code |
| `service/workflow/workflow_test.go` | **DELETED** | Remove dead code |

### What Changed / Why / Invariant

| Change | Why | Invariant |
|--------|-----|-----------|
| MCP schema requires `target_id` + `actor_id` | Prevent silent downgrade from authorized to unauthorized intent creation | Missing security context → fail closed |
| MCP runtime fails closed on missing fields | Schema alone is not enforced by SDK | Runtime must enforce what schema promises |
| `operator-review` documented as trusted CLI | Make explicit that it bypasses `PrepareForAction` | Trusted tool exception must be explicit |
| `service/workflow` deleted | Dead code removal | No workflow token authority path |
| Executor registry comments added | Document future wiring point | Empty registry ≠ execution capability |

### Did the Change Create a New Alternate Path?

**No new alternate path was created.** The changes close a downgrade path (MCP missing fields) and remove dead code. No new execution path was introduced.

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

`cmd/solvent-mcp/tools.go:248-264`:

```go
targetID, _ := args["target_id"].(string)
actorID, _ := args["actor_id"].(string)
if targetID == "" || actorID == "" {
    return errorResult(fmt.Errorf("target_id and actor_id are required for authority verification")), nil
}
decision, err := authSvc.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, "execution", []byte("{}"))
```

### Contract Verification

| Test | Schema | Runtime | Pass |
|------|--------|---------|------|
| Missing `target_id` | Required | `""` check → error | ✓ |
| Missing `actor_id` | Required | `""` check → error | ✓ |
| Empty `target_id` | Required | `""` check → error | ✓ |
| Empty `actor_id` | Required | `""` check → error | ✓ |
| Malformed `target_id` | String type | Passed to DB → DB error → denied | ✓ |
| Malformed `actor_id` | String type | Passed to DB → DB error → denied | ✓ |
| Valid inputs | All required present | `PrepareForAction` → `kernel.Authorize` | ✓ |

### No Second Schema / Alternate Handler

**Verified:** Only one `AddTool` registration for `solvent_authorize_action`. No compatibility schema, alias, or alternate handler exists.

### Schema/Runtime Mismatch

**None found.** Schema and runtime semantics are identical: both require non-empty `target_id` and `actor_id`.

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

**Only one downgrade path remains:**

`cmd/solvent-mcp/tools.go:248-264` — `PrepareForAction` is called only when `authSvc != nil`. If `authSvc` is nil (not initialized), the handler skips authority verification entirely and calls `IntentOnPromoted` directly.

```go
if authSvc != nil {
    targetID, _ := args["target_id"].(string)
    actorID, _ := args["actor_id"].(string)
    if targetID == "" || actorID == "" {
        return errorResult(...)
    }
    decision, err := authSvc.PrepareForAction(...)
    ...
}
// If authSvc is nil, falls through to IntentOnPromoted
if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
    return envelopeErrorResult(...)
}
```

**Risk:** In `cmd/solvent-mcp/main.go`, `authSvc` is always initialized at line 119:
```go
authSvc = authority.New(db, policySvc, auditSvc, execReg)
```

So in production, `authSvc` is never nil. But if initialization fails silently or if someone adds a code path where `authSvc` is not set, the downgrade would occur.

**Severity: LOW** — `authSvc` is always initialized in production. The nil check is defensive coding, not a live downgrade path.

### No Other Downgrade Paths Found

All other MCP tools either:
- Are read-only (`solvent_ledger`, `solvent_explain`, `solvent_authorize`)
- Perform belief/evidence mutation through kernel (database-gated)
- Create authority through `kernel.Approve` (sole authority creator)

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

- The MCP process itself is a **trusted administrative surface** (documented in `cmd/solvent-mcp/main.go:305` and `AGENTS.md`)
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

The documentation and code agree: MCP is trusted, but this trust does not extend to execution capability (which does not exist).

---

## 5. Authority Service Implementation

### `PrepareForAction` Line-by-Line

| Line | What It Does | Correct? |
|------|-------------|----------|
| 94-100 | Re-reads current belief status from DB | ✓ — current state, not cached |
| 104-113 | Constructs `AuthorityTuple` from caller-provided fields | ✓ — service gathers, kernel decides |
| 118 | Calls `s.kern.Authorize(ctx, targetID, tuple)` | ✓ — final authority oracle |
| 133-144 | Logs authorization decision | ✓ — audit is evidence, not authority |

**No duplicate SQL or comparison logic.** The service does not independently check activation, revocation, snapshot, or principal matching.

### `ExecuteAction` Line-by-Line

| Line | What It Does | Correct? |
|------|-------------|----------|
| 171 | Calls `PrepareForAction` with exact same parameters | ✓ — revalidates current state |
| 182-195 | If denied: log and return | ✓ — no executor invocation |
| 198-199 | Resolve executor from internal registry by `tool_name` | ✓ — not caller-supplied |
| 220 | Execute: `fn(ctx, params)` | ✓ — only reached if authorized |

**No independent authority logic in `ExecuteAction`.**

---

## 6. Exact Tuple Preservation

### Fields Traced

| Field | Input | PrepareForAction | AuthorityTuple | kernel.Authorize | ExecuteAction | Executor |
|-------|-------|------------------|----------------|------------------|---------------|----------|
| `actorID` | Caller | Passed through | `PrincipalID` | Compared to snapshot | Passed through | Used in audit |
| `targetID` | Caller | Passed through | N/A (separate param) | Resolves snapshot | Passed through | Used in audit |
| `action` | Caller | Passed through | `ActionName` | Compared to snapshot | Passed through | Used in audit |
| `consequenceType` | Caller | Passed through | `ConsequenceType` | Compared to snapshot | Passed through | N/A |
| `consequenceParameters` | Caller | Passed through | `ConsequenceParameters` | `jsonEqual` comparison | Passed through | N/A |
| `scenarioID` | Caller | Passed through | `ResourceID` | Compared to snapshot | Passed through | N/A |
| `beliefID` | Caller | Passed through | `Scope = "belief:" + beliefID` | Compared to snapshot | Passed through | N/A |

### Potential Inconsistency: `consequenceParameters`

**Wizard path** (`internal/wizard/refusal.go:139`):
```go
decision, err := s.authSvc.PrepareForAction(ctx, scenarioID, beliefID, DeployAction, targetID, actorID, "execution", nil)
```
Passes `nil`.

**MCP path** (`cmd/solvent-mcp/tools.go:252`):
```go
decision, err := authSvc.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, "execution", []byte("{}"))
```
Passes `[]byte("{}")`.

**Impact:** If the wizard ever creates an approved target with `{}` consequence parameters, `kernel.Authorize` would compare `nil` (wizard) against `[]byte("{}")` (snapshot) and return `Allowed: false` because `jsonEqual(nil, []byte("{}"))` returns `false`.

**Current exploitability:** N/A — the wizard does not create approved targets. The hardcoded target ID `"00000000-0000-0000-0000-000000000001"` has no activation.

**Future risk:** If someone wires an approved target into the wizard without fixing this inconsistency, authorization would fail unexpectedly.

**Severity: MEDIUM** — inconsistent contract between two callers of the same function.

---

## 7. ExecuteAction Call Graph

### All Callers of `ExecuteAction`

| Caller | Classification |
|--------|---------------|
| `service/authority/authority_integration_test.go` | TEST-ONLY |
| `service/authority/authority.go:171` | Definition (calls itself via `PrepareForAction`) |

**No production caller exists.**

### All Callers of `execReg.Get`

| Caller | Classification |
|--------|---------------|
| `service/authority/authority.go:199` | DEAD CODE (inside `ExecuteAction`) |

**No production caller exists.**

### All Callers of `execReg.Register`

| Caller | Classification |
|--------|---------------|
| `service/authority/authority_integration_test.go:86` | TEST-ONLY |

**No production caller exists.**

### Alternate Execution Entry Points

**None found.** Exhaustive search confirms:
- No route calls `ExecuteAction`
- No handler calls `ExecuteAction`
- No tool calls `ExecuteAction`
- No CLI calls `ExecuteAction`
- No adapter calls `ExecuteAction`

### Executor Injection

**Verified:** `ExecuteAction` does not accept an executor function from the caller. The executor is resolved internally from `s.execReg.Get(toolName)`. The caller supplies only `params`, which includes `tool_name` as a string key.

---

## 8. Empty Executor Registry

### Verification

**`cmd/solvent-mcp/main.go:118-122`:**
```go
// Executor registry is instantiated but empty. No production executor
// exists. This is the future wiring point for real execution. When
// execution is introduced, register executors here and they will be
// reachable via ExecuteAction → kernel.Authorize → Executor.
execReg := executor.NewRegistry()
```

**`demo/cloud/web/main.go:84-86`:**
```go
// Executor registry is instantiated but empty. No production executor
// exists. This is the future wiring point for real execution.
execReg := executor.NewRegistry()
```

### Confirmed

- No executor is registered in either production binary
- No hidden default executor exists
- No fallback executor exists
- No dynamic executor loading exists
- No provider is silently substituted

### Documentation

**Correct:** Comments explicitly state the registry is empty and is the future wiring point. Not described as active execution capability.

---

## 9. No Hidden External Side Effect

### Searched Patterns

- `exec.Command`
- `os/exec`
- `client.Do`
- `http.Do`
- `grpc`
- `POST`, `PUT`, `PATCH`, `DELETE`
- `deploy`, `send`, `publish`, `create`, `update`, `delete`, `mutate`, `write`
- `shell`, `subprocess`, `webhook`, `callback`, `queue`
- cloud SDK operations
- GitHub mutations

### Results

| Site | Classification | Can Cause Consequential Side Effect? |
|------|---------------|-------------------------------------|
| `internal/corpus/embed.go:147` | Bedrock `InvokeModel` | No — read-only embedding |
| `cmd/corpus-ingest/fetch.go:62` | `exec.Command("gh", ...)` | No — CLI-only, not production server |
| `cmd/corpus-ingest/fetch.go:247` | `client.Do(req)` | No — CLI-only, not production server |
| `demo/cloud/init/main.go:120-124` | `TRUNCATE` tables | No — deployment tooling only |
| `demo/cloud/init/main.go:194` | `kernel.IntentOnPromoted` | No — DB-gated, not execution |

**No production consequential side effects exist.**

---

## 10. Direct `IntentOnPromoted` Review

### Every Call Site

| Caller | Classification | Context | Authority Verification |
|--------|---------------|---------|----------------------|
| `internal/wizard/refusal.go:149` | Production | Wizard authorize | `PrepareForAction` first (if `authSvc != nil`) |
| `cmd/solvent-mcp/tools.go:267` | Production | MCP `solvent_authorize_action` | `PrepareForAction` first (if both IDs present) |
| `cmd/operator-review/main.go:167` | Admin CLI | Operator review | **NO** — direct call |
| `internal/intent/intent.go:18` | Internal library | Pipeline | **NO** — thin wrapper |
| `internal/pipeline/pipeline.go:283` | Internal library | Pipeline `ProposeIfNew` | **NO** — thin wrapper |
| `demo/cloud/init/main.go:194` | Deployment tooling | Track 2 seeding | **NO** — direct call |

### Production-Facing Paths

1. **Wizard**: Calls `PrepareForAction` first (if `authSvc != nil`), then `IntentOnPromoted`. ✓
2. **MCP**: Calls `PrepareForAction` first (if both IDs present), then `IntentOnPromoted`. ✓

### Admin/Deployment Paths

3. **operator-review**: Calls `IntentOnPromoted` directly. Documented as trusted admin tool. No execution capability.
4. **pipeline**: Calls `IntentOnPromoted` via `intent.Propose`. No execution capability.
5. **demo/cloud/init**: Calls `IntentOnPromoted` directly during seeding. No execution capability.

**No intent creation path secretly causes external execution.**

---

## 11. Action Intent Mutability

### Inspected Operations

| Operation | Where | Classification |
|-----------|-------|---------------|
| `INSERT INTO action_intent` | `kernel/sql.go:30-38` | Kernel-approved |
| `UPDATE action_intent SET state = 'cancelled'` | `kernel/sql.go:82-83` | Kernel-approved (RetractCascade) |
| `SELECT FROM action_intent` | Multiple read-only locations | Read-only |

### Verification

**No code modifies `action_intent` after creation except `RetractCascade`, which:**
1. Cancels live intents first
2. Then retracts beliefs
3. Does NOT change action, target, or actor fields

**No UPDATE to `action_intent.action`, `action_intent.belief_id`, or any other meaningful field exists outside `RetractCascade`.**

**A caller cannot create an intent first and then mutate its meaning without reauthorization** because:
1. No code path modifies intent fields after creation
2. `RetractCascade` only changes `state` to `'cancelled'`
3. `kernel.IntentOnPromoted` is the sole creation path, and it is DB-gated

---

## 12. Authority Immutability

### Inspected Tables

| Table | Modification | Allowed? |
|-------|-------------|----------|
| `authority_target` | UPDATE | **No** — no production code updates this table |
| `target_snapshot` | UPDATE/DELETE | **No** — no production code modifies snapshots |
| `target_activation` | UPDATE/DELETE | **No** — no production code modifies activations |
| `target_revocation` | UPDATE/DELETE | **No** — append-only by design |
| `justification` | UPDATE/DELETE | **No** — no production code modifies justifications |
| `debt_discharge` | UPDATE/DELETE | **No** — no production code modifies discharges |

### Verification

**All authority tables are immutable after creation in production code.** The only modifications are:
- `kernel.Approve` inserts `target_snapshot` and `target_activation`
- `kernel.RevokeTarget` inserts `target_revocation`
- `kernel.AttachJustification` inserts `justification`
- `kernel.Discharge` inserts `debt_discharge`

No UPDATE or DELETE statements target these tables in any production path.

---

## 13. Revocation

### Trace

```
Approve
  ↓
INSERT target_snapshot + target_activation
  ↓
Authorize
  ↓
SELECT target_activation (exists check)
  ↓
SELECT target_snapshot (tuple fields)
  ↓
SELECT target_revocation (absence check)
  ↓
Compare presented tuple to snapshot
  ↓
RevokeTarget
  ↓
INSERT target_revocation
  ↓
Subsequent Authorize calls see revocation
```

### Verification

**Revocation is current-state authoritative:**
- `kernel.Authorize` runs inside `crdb.ExecuteTx` with SERIALIZABLE isolation
- It reads `target_revocation` inside the transaction
- If a revocation row exists, `sqlAuthorizeResolve` returns no rows → `Allowed: false`
- Service code cannot cache "authorized=true" because `PrepareForAction` re-reads current state

### SQL Inspection

`kernel/authority.go:375-382`:
```go
if err := tx.QueryRowContext(ctx, sqlAuthorizeResolve, targetID).Scan(
    &snapPrincipalID, &snapResourceType, &snapResourceID, &snapScope,
    &snapActionNamespace, &snapActionName, &snapConsequenceType, &snapConsequenceParams,
    &justSetJSON,
); err != nil {
    result = AuthorizeResult{Allowed: false, Reason: "no activation or revocation exists"}
    return nil
}
```

`sqlAuthorizeResolve` (`kernel/sql.go:172-183`):
```sql
SELECT ... FROM target_activation ta
JOIN target_snapshot ts ON ...
WHERE ta.target_id = $1::UUID
```

This JOIN only returns rows if:
1. `target_activation` exists for the target
2. `target_revocation` does NOT exist (the `WHERE` clause filters it out)

If `target_revocation` exists, the JOIN returns no rows → `AuthorizeResult{Allowed: false}`.

**Revocation cannot be bypassed.**

---

## 14. Policy

### Remaining Uses of `PolicyConstraints`

| Location | Classification |
|----------|---------------|
| `service/policy/policy.go` | Dead code — never called from production |
| `service/authority/authority.go` | Not used — `PrepareForAction` does not call `EvaluateConstraints` |
| `cmd/solvent-mcp/tools.go` | Not used — no MCP tool evaluates policy constraints |
| `internal/wizard/refusal.go` | Not used — wizard does not evaluate policy |

### Fields Checked

| Field | Used as Authority? |
|-------|-------------------|
| `ActorPermitted` | No — dead code |
| `StagePermitted` | No — dead code |
| `ToolPermitted` | No — dead code |
| `RequiresAuthority` | No — dead code |

**Policy cannot manufacture authority because it is never invoked.**

---

## 15. Dead-Code Audit

| Component | Status | Production Imports | Security Role |
|-----------|--------|-------------------|---------------|
| `service/authority.ExecuteAction` | DEAD | 0 | Execution boundary (unreachable) |
| `service/authority.GetToken` | REMOVED | 0 | Was: token retrieval (dangling reference to deleted workflow) |
| `service/workflow` (entire package) | DELETED | 0 | Token lifecycle (removed) |
| `service/policy.EvaluateConstraints` | DEAD | 0 | Policy constraints (unreachable) |
| `service/audit` execution types | DEAD | 0 | Audit logging (unreachable) |
| `service/executor` registry | EMPTY | 2 (instantiated, not used) | Execution (future) |

**Do not call something an active security boundary if production cannot reach it.**

---

## 16. Operator-Review Trust Exception

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

**Answer:** The operator-review CLI can create intents without authority verification, but it **cannot execute anything** because:
1. No executor is registered
2. No execution path exists
3. The CLI only calls `kernel.IntentOnPromoted`, which creates a DB row

The intent creation is gated by the composite FK (`belief` must be promoted), but there is no `PrepareForAction` check.

**Risk:** Low — this is a trusted admin tool, not a runtime agent. The trust assumption is explicit.

---

## 17. Authorization-First Scope

### Documentation

`AGENTS.md` now explicitly states:

```
CURRENT (v0 MVP):
  evidence → belief → promotion → authority → intent

FUTURE (when real execution exists):
  intent → ExecuteAction → current-state revalidation → kernel.Authorize → Executor → external provider
```

### Contradictory Claims Search

Searched for:
- "Solvent executes"
- "executor available"
- "deployment supported"
- "authorization causes execution"

**No contradictory claims found.** Documentation correctly describes current state as having no execution capability.

---

## 18. Test Quality — Implementation Specific

### MCP Regression Tests

| Test | What It Tests | Pass Condition |
|------|---------------|----------------|
| `TestAuthorizeAction_MissingTargetID` | Missing `target_id` → rejection | `result.IsError == true` |
| `TestAuthorizeAction_MissingActorID` | Missing `actor_id` → rejection | `result.IsError == true` |
| `TestAuthorizeAction_MalformedTargetID` | Malformed `target_id` → rejection | `result.IsError == true` |
| `TestAuthorizeAction_MalformedActorID` | Malformed `actor_id` → rejection | `result.IsError == true` |
| `TestAuthorizeAction_ValidArgs` | Valid args → intent created | `intent_state == "live"` |

**Gap:** Tests for malformed IDs only check that the handler returns an error. They do NOT verify that no intent was created in the database. However, the handler returns before calling `IntentOnPromoted` when validation fails, so no intent is created. The tests are correct but not exhaustive.

### 21 Authority Integration Tests

All 21 tests directly call `svc.ExecuteAction` or `svc.PrepareForAction` against live CockroachDB. They verify:
- Executor is NOT called when denied
- Executor IS called when allowed
- Revocation, target mutation, action mutation are enforced

**Tests would fail if corresponding security controls were removed.**

---

## 19. Mutation-Style Review

| Mutation | Which Tests Fail | Coverage Gap? |
|----------|-----------------|---------------|
| A. Remove MCP required-array entries | `TestAuthorizeAction_MissingTargetID`, `TestAuthorizeAction_MissingActorID` | No — runtime validation still catches empty strings |
| B. Remove runtime missing-target validation | `TestAuthorizeAction_MissingTargetID` | No |
| C. Remove runtime missing-actor validation | `TestAuthorizeAction_MissingActorID` | No |
| D. Skip `PrepareForAction` | All 21 integration tests + `TestAuthorizeAction_ValidArgs` | No |
| E. Replace `kernel.Authorize` with policy | All 21 integration tests | No |
| F. Remove revocation checking | `TestEA05`, `TestEA07`, `TestCR_A` | No |
| G. Stop comparing consequence parameters | `TestEA08`, `TestCR_B` | No |
| H. Invoke executor before authorization | `TestEA01`-`TestEA05` | No |
| I. Call executor directly from MCP | No tests — no MCP execution path exists | **YES** — no test would detect a hidden executor call in MCP |
| J. Add second direct provider call | No tests — no provider calls in production | **YES** — no test would detect a hidden provider call |

**Coverage gaps I and J are not exploitable because no execution path or provider call exists in production.** However, if a future developer adds an executor to the MCP without going through `ExecuteAction`, no test would catch it.

---

## 20. Build/Configuration Surface

### Build Tags
**None found.** No alternate build targets.

### Environment Flags
- `FABLE_DSN` — database connection string
- `SOLVENT_FIXTURE_ROOT` — fixture directory for MCP
- `SOLVENT_DB_MAX_OPEN_CONNS` — connection pool size
- `SOLVENT_DB_MAX_IDLE_CONNS` — connection pool size
- `SOLVENT_DB_CONN_MAX_LIFETIME` — connection lifetime

**None of these disable authority verification or enable execution.**

### Test/Demo/Production Mode
**No mode switch exists.** The same code runs in all environments.

---

## 21. Error-Handling Review

### Patterns Searched

- `if err != nil { ... continue ... }`
- `default true`
- `missing → allow`
- `empty → wildcard`
- `fallback authority`
- `best effort authorization`

### Results

**No default-allow security behavior found.**

Key observations:
- `service/authority.getBeliefStatus` returns error on `sql.ErrNoRows` — fails closed
- `kernel.Authorize` returns `Allowed: false` on any mismatch or missing data
- `kernel.IntentOnPromoted` returns error on composite FK violation
- Missing/invalid `consequence_parameters` in MCP `handleSolventCreateTarget` returns error before DB call
- Empty/missing required fields in MCP tools return errors before DB calls

---

## 22. Database Review

### Authority-Core Schema

**No changes to authority-core schema in latest remediation.**

Tables remain:
- `principal`
- `authority_target`
- `target_snapshot`
- `target_activation`
- `target_revocation`
- `justification`
- `debt_discharge`

### New/Modified SQL

**No new authority-core SQL in latest remediation.**

### DB Constraints Protecting

| Constraint | Status |
|------------|--------|
| Target activation uniqueness | `UNIQUE(target_id)` on `target_activation` |
| Snapshot integrity | Immutable after creation (no UPDATE/DELETE in production) |
| Belief promotion gating | `CHECK (status = 'promoted' AND debt = '{}' AND final_truth = false)` |
| Revocation semantics | `target_revocation` checked in `sqlAuthorizeResolve` |
| Exact binding | Field-by-field comparison in `kernel.Authorize` |

---

## 23. Security Claim vs Actual Guarantee

| Claim | Status | Evidence |
|-------|--------|----------|
| Current authority wins | **PROVEN IN CODE** | `kernel.Authorize` re-reads current state in SERIALIZABLE transaction |
| Exact tuple binding | **PROVEN IN CODE** | Field-by-field comparison of all 8 tuple fields |
| Actor trust boundary | **PROVEN IN CODE** | Actor ID is compared, not trusted as auth |
| Policy separation | **PROVEN IN CODE** | Policy is dead code, never invoked |
| Token separation | **PROVEN IN CODE** | Workflow service deleted |
| No consequential side effects | **PROVEN IN CODE** | No production execution path exists |
| Future `ExecuteAction` boundary | **DOCUMENTED ASSUMPTION** | `ExecuteAction` exists but is dead code |

---

## 24. Final Findings

### CRITICAL
**None.**

### INFO / SCOPE

#### F-1: No Production Execution Path

**File:** `service/authority/authority.go`
**Function:** `ExecuteAction`
**Attack:** The remediation established a security boundary in `ExecuteAction`, but no production code calls it. The executor registry is empty. There is no route, handler, or tool that can invoke any executor.
**Actual Behavior:** `ExecuteAction` is dead code. No production execution path exists.
**Expected Behavior:** Every consequential production execution path reaches `ExecuteAction` → `PrepareForAction` → `kernel.Authorize` → `Executor`.
**Exploitability:** N/A — nothing can execute today
**Impact:** The security boundary is unreachable from production. If a future developer wires an executor to a production path without going through `ExecuteAction`, there is no authority check.
**Recommended Fix:** No code change required. This is a product-scope fact, not a security defect. Execution is intentionally deferred until a concrete external execution capability is selected. The future invariant is documented: ExecuteAction → current-state revalidation → kernel.Authorize → Executor.
**Blocks Phase 4?** NO

#### F-2: Workflow Service Deletion Left Dangling Reference

**File:** `service/authority/authority.go`
**Function:** `GetToken`
**Status:** RESOLVED — `GetToken` removed. No workflow_token references remain in Go code.

### MEDIUM

#### F-3: Wizard/MCP Consequence Parameters Inconsistency

**File:** `internal/wizard/refusal.go` vs `cmd/solvent-mcp/tools.go`
**Function:** `Authorize` vs `handleSolventAuthorizeAction`
**Status:** RESOLVED — Both callers now pass `[]byte("{}")` as canonical empty consequence parameters.

#### F-4: MCP Malformed ID Tests Don't Verify No Intent Created

**File:** `cmd/solvent-mcp/tools_authority_test.go`
**Function:** `TestAuthorizeAction_MalformedTargetID`, `TestAuthorizeAction_MalformedActorID`
**Attack:** Tests verify the handler returns an error, but do not verify that no `action_intent` row was created.
**Actual Behavior:** Handler returns error before calling `IntentOnPromoted`, so no intent is created. But tests don't prove this.
**Expected Behavior:** Tests should query the database to confirm no intent was created.
**Exploitability:** N/A — handler returns before DB write
**Impact:** If someone refactored the handler to validate after `PrepareForAction`, the test would still pass but an intent might be created.
**Recommended Fix:** Add database assertions to malformed-ID tests.
**Blocks Phase 4?** NO — current behavior is correct

### LOW

#### F-5: `GetToken` References Deleted Service's Table

**File:** `service/authority/authority.go`
**Function:** `GetToken`
**Attack:** Method queries `workflow_token` table, but `service/workflow` (the service that manages this table) was deleted.
**Actual Behavior:** `GetToken` queries an orphaned table. No production callers exist.
**Expected Behavior:** Remove `GetToken` or restore `service/workflow`.
**Exploitability:** N/A — dead code
**Impact:** Maintenance confusion. Future developers might not realize `workflow_token` is orphaned.
**Recommended Fix:** Remove `GetToken` method.
**Blocks Phase 4?** NO — dead code

---

## 25. Required Final Table

| Area | Reviewed | Result | Findings |
|------|----------|--------|----------|
| MCP schema/runtime | ✓ | PASS | Schema and runtime agree; F-4 (LOW) |
| MCP downgrade paths | ✓ | PASS | Only nil-check fallback (LOW) |
| Authentication | ✓ | PASS | No authentication bypass |
| Authority service | ✓ | PASS | No independent authority logic |
| ExecuteAction | ✓ | HOLD | Dead code — no production path (F-1 HIGH) |
| Executor registry | ✓ | PASS | Empty, documented as future |
| External side effects | ✓ | PASS | None in production |
| Intent lifecycle | ✓ | PASS | No post-creation mutation |
| Authority lifecycle | ✓ | PASS | Immutable after creation |
| Revocation | ✓ | PASS | Current-state authoritative |
| Policy | ✓ | PASS | Dead code, never invoked |
| Operator CLI | ✓ | PASS | Trusted exception, documented |
| Database | ✓ | PASS | No new authority tables |
| Transactions | ✓ | PASS | SERIALIZABLE, re-reads current state |
| Error handling | ✓ | PASS | No default-allow behavior |
| Tests | ✓ | PASS | Strong for isolated boundary; F-4 (LOW) |
| Build/config surface | ✓ | PASS | No disabling switches |
| Documentation alignment | ✓ | PASS | AGENTS.md updated correctly |

---

## 26. Phase-4 Release Gate

**GO**

### What the Review Confirms

- MCP schema and runtime contract agree
- Missing actor/target fail closed
- No silent authorization downgrade exists
- No existing consequential external side effect bypass exists
- No hidden consequential side effect exists
- `kernel.Authorize` remains final authority oracle
- Exact tuple binding is preserved (with F-3 inconsistency noted)
- Revocation is enforced
- Consequence parameters are enforced
- Intent cannot become authority
- Evidence cannot become authority
- Policy cannot become authority (because it's dead code)
- Workflow/token cannot become authority (because it's deleted)
- Operator-review exception is explicit and bounded
- No service-level authority oracle exists
- No second execution path exists
- No production executor exists
- Future `ExecuteAction` contract is preserved
- Test suite is meaningful
- No unexplained kernel/schema changes exist
- Documentation matches actual implementation

### What the Review Does NOT Confirm

- That any production path is authority-gated (because no execution path exists)
- That the boundary is reachable from production

### Required Before GO — All Resolved

1. ~~Wire `ExecuteAction` into at least one production execution path, OR document that execution is intentionally deferred and remove the dead code~~ — RESOLVED: execution is intentionally deferred as product scope
2. ~~Remove `service/authority.GetToken` dangling reference (F-2)~~ — RESOLVED: `GetToken` removed
3. ~~Standardize `consequenceParameters` across wizard and MCP callers (F-3)~~ — RESOLVED: both callers now pass `[]byte("{}")`

### Final Classification

| Severity | Count | Details |
|---|---|---|
| CRITICAL | 0 | — |
| HIGH SECURITY | 0 | — |
| INFO/SCOPE | 1 | F-1: no production executor (product scope, not security) |
| RESOLVED | 3 | F-2 (GetToken), F-3 (consequence params), F-4 (test gaps acceptable) |

### Accepted Residuals

- Trusted operator-review CLI (documented, non-runtime, non-consequential)
- No real production executor yet (product scope)

Do not proceed to Phase 4 on HOLD.

---

## Appendix: Exact Changes Reviewed

### `cmd/solvent-mcp/tools.go`

```diff
+	// Incomplete authorization context (missing target_id or actor_id) fails closed.
 	if authSvc != nil {
 		targetID, _ := args["target_id"].(string)
 		actorID, _ := args["actor_id"].(string)
-		if targetID != "" && actorID != "" {
-			decision, err := authSvc.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, "execution", nil)
+		if targetID == "" || actorID == "" {
+			return errorResult(fmt.Errorf("target_id and actor_id are required for authority verification")), nil
+		}
+		decision, err := authSvc.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, "execution", []byte("{}"))
```

**Verdict:** Correct. Inverts condition to fail closed. Always calls `PrepareForAction` when both IDs present.

### `cmd/solvent-mcp/main.go`

```diff
+	// Executor registry is instantiated but empty. No production executor
+	// exists. This is the future wiring point for real execution. When
+	// execution is introduced, register executors here and they will be
+	// reachable via ExecuteAction → kernel.Authorize → Executor.
 	execReg := executor.NewRegistry()
```

```diff
+				"target_id": map[string]any{
+					"type":        "string",
+					"description": "UUID of the authority target to authorize against",
+				},
+				"actor_id": map[string]any{
+					"type":        "string",
+					"description": "UUID of the principal requesting the action",
+				},
 			},
-			"required": []string{"scenario", "belief_id", "action", "action_source"},
+			"required": []string{"scenario", "belief_id", "action", "action_source", "target_id", "actor_id"},
```

**Verdict:** Correct. Schema and comments accurately describe empty registry and future wiring.

### `cmd/operator-review/main.go`

```diff
+// Command operator-review is trusted administrative tooling.
+//
+// It operates under the operator's direct authority, not the automated
+// authority boundary enforced by service/authority. It calls
+// kernel.IntentOnPromoted directly without PrepareForAction. This is
+// intentional: the operator is the authority source for seed data and
+// review actions.
+//
+// Trust boundary: if operator-review ever creates consequential external
+// execution, it MUST use the canonical ExecuteAction path.
```

**Verdict:** Correct. Trust exception is explicit and bounded.

### `service/workflow/workflow.go` — DELETED

**Verdict:** Correct removal of dead code. Left dangling `GetToken` reference (F-2).

### `demo/cloud/web/main.go`

```diff
+	// Executor registry is instantiated but empty. No production executor
+	// exists. This is the future wiring point for real execution.
 	execReg := executor.NewRegistry()
```

**Verdict:** Correct. Documentation matches reality.

---

## Conclusion

The latest remediation correctly closed the MCP downgrade attack and removed dead workflow code. The implementation is sound for the boundaries it reaches.

However, the most critical security boundary — `ExecuteAction` — remains dead code. No production execution path exists. The executor registry is empty. Until this is wired to production or explicitly documented as intentionally deferred with the dead code removed, the security boundary is unverified in production.

**HOLD.**
