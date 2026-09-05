# Solvent — Targeted Post-Remediation Adversarial Code Review

**Date:** 2026-09-04
**Reviewer:** Kilo
**Scope:** Current repository state after security remediation, before Phase 4
**Status:** HOLD

---

## 1. Review Objective

Attempt to break these invariants:

```
Every consequential production execution path
    ->
ExecuteAction
    ->
PrepareForAction
    ->
current policy constraints
    ->
kernel.Authorize
    ->
Executor
```

And:

```
No successful CURRENT kernel.Authorize
for the EXACT actor/action/target tuple
    ->
no executor invocation.
```

---

## 2. Actual Production Consequential Execution Call Graph

**FINDING: There is no production consequential execution path.**

After exhaustive search of every handler, route, tool, CLI command, and adapter:

| Entry Point | Execution Path | Classification |
|-------------|---------------|----------------|
| `internal/wizard/http.go` routes | `/api/authorize` -> `s.Authorize` -> `PrepareForAction` -> `kernel.IntentOnPromoted` | INTENT CREATION ONLY |
| `cmd/solvent-mcp/tools.go` | `handleSolventAuthorizeAction` -> `PrepareForAction` -> `kernel.IntentOnPromoted` | INTENT CREATION ONLY |
| `cmd/operator-review/main.go` | `runReviewMode` -> `kernel.IntentOnPromoted` | INTENT CREATION ONLY (CLI) |
| `cmd/solvent/main.go` | `pipeline.Run` -> `intent.Propose` -> `kernel.IntentOnPromoted` | PIPELINE ONLY |
| `demo/cloud/web/main.go` | No execution routes registered | DEAD END |
| `cmd/solvent-mcp/main.go` | No execution tools registered | DEAD END |

**No production code calls `service/authority.ExecuteAction`.**

The executor registry is instantiated in two production binaries:
- `cmd/solvent-mcp/main.go:118` -- `execReg := executor.NewRegistry()` -- **empty, no executors registered**
- `demo/cloud/web/main.go:84` -- `execReg := executor.NewRegistry()` -- **empty, no executors registered**

No production code calls `execReg.Register(...)`.

The only callers of `ExecuteAction` are in `service/authority/authority_integration_test.go` (test code only).

---

## 3. Every Executor Invocation Site

| Site | Classification |
|------|---------------|
| `service/authority/authority.go:220` (`fn(ctx, params)`) | AUTHORITY-GATED but **DEAD** -- never reached from production |
| `service/executor/executor.go:51` (`fn(ctx, params)`) | **DEAD** -- standalone function, never called from production |
| `service/authority/authority_integration_test.go` | TEST-ONLY |

---

## 4. Every External Consequential Provider Call

| Call | Classification |
|------|---------------|
| `internal/corpus/embed.go:147` (`bedrockruntime.InvokeModel`) | READ-ONLY -- embedding generation for ANN search, not execution |
| `cmd/corpus-ingest/fetch.go:62` (`exec.Command("gh", ...)`) | CLI-ONLY -- not part of production server |
| `cmd/corpus-ingest/fetch.go:247` (`client.Do(req)`) | CLI-ONLY -- not part of production server |

**No production consequential provider calls exist.**

---

## 5. Every Current `kernel.Authorize` Call Site

| Site | Classification |
|------|---------------|
| `service/authority/authority.go:118` | AUTHORITY-GATED but only reached via `PrepareForAction` for **intent creation verification**, not execution |
| `cmd/solvent-mcp/tools.go:566` | READ-ONLY -- MCP `solvent_authorize` tool, explicit verification-only |

---

## 6. Attack #1 -- Execution Bypass

**Result: No bypass found because no execution path exists.**

After tracing every route, handler, tool, and CLI command:
- No production path invokes any executor
- No production path calls `ExecuteAction`
- The executor registry is empty in both production binaries

**This is not a bypass -- it is a gap.** The security boundary exists in code but is unreachable from any production execution path.

---

## 7. Attack #2 -- `kernel.Authorize` Bypass

**Result: No bypass found.**

No service code independently decides authority. `PrepareForAction` delegates exclusively to `kernel.Authorize`.

The only places `kernel.Authorize` is called outside tests:
1. `service/authority/authority.go:118` -- via `PrepareForAction`
2. `cmd/solvent-mcp/tools.go:566` -- read-only verification tool `solvent_authorize`

No service-level authority oracle exists.

---

## 8. Attack #3 -- Exact Tuple Binding

**Result: Verified in code, but untestable in production because no execution path exists.**

In `service/authority/authority.go`:
- `ExecuteAction` receives `scenarioID, beliefID, action, targetID, actorID, params, consequenceType, consequenceParameters`
- It passes the **exact same values** to `PrepareForAction`
- `PrepareForAction` constructs `AuthorityTuple` from those exact values and calls `kernel.Authorize`
- If authorized, `ExecuteAction` resolves the executor internally from `execReg.Get(toolName)` where `toolName` comes from `params["tool_name"]`

The tuple binding is preserved in the code. However, since `ExecuteAction` is never called from production, this binding is untested in any production path.

---

## 9. Attack #4 -- Execution-Time Revalidation

**Result: N/A -- no execution path exists.**

`ExecuteAction` calls `PrepareForAction` at execution time, which re-reads current state. But since no production code calls `ExecuteAction`, this revalidation is never exercised in production.

---

## 10. Attack #5 -- Revocation Race / Stale Authority

**Result: Verified in tests, untested in production.**

Tests `TestEA08_TargetMutatesBetweenPrepareAndExecute` and `TestCR_A_RevokedAfterPrepare` verify that revocation between `PrepareForAction` and `ExecuteAction` is detected. But since no production code calls `ExecuteAction`, this is never exercised in production.

---

## 11. Attack #6 -- Target/Action Mutation

**Result: Verified in tests, untested in production.**

Tests `TestEA03_WrongTarget`, `TestEA04_WrongAction`, `TestCR_B_TargetChangedAfterPrepare`, and `TestCR_C_ActionChangedAfterPrepare` verify exact tuple binding. But since no production code calls `ExecuteAction`, this is never exercised in production.

---

## 12. Attack #7 -- Policy as Second Authority Engine

**Result: No bypass found, but policy is dead code in production.**

`service/policy.EvaluateConstraints` is never called from any production execution path. The policy service exists but does not constrain anything because nothing executes.

No path exists where `policy ALLOW + authority ABSENT = EXECUTED` because there is no execution path.

---

## 13. Attack #8 -- Workflow Token

**Result: `service/workflow` is dead code.**

`service/workflow` is not imported anywhere in production code. Its token lifecycle management (`CreateToken`, `Prepare`, `Execute`, `Complete`, `Fail`, `Expire`) is unreachable.

No production path uses workflow tokens for authority.

---

## 14. Attack #9 -- Actor/Authentication Spoofing

**Result: No bypass found in production paths.**

Production paths that call `PrepareForAction`:
1. **Wizard** (`internal/wizard/refusal.go:139`): Uses hardcoded `actorID := "00000000-0000-0000-0000-000000000001"` and `targetID := "00000000-0000-0000-0000-000000000001"`. Not user-controlled.
2. **MCP** (`cmd/solvent-mcp/tools.go:249-263`): Uses `actorID` and `targetID` from request args, but only when both are non-empty. Still intent creation only.

Neither path leads to execution.

Test `TestCB05_ActorSpoofingFails` verifies that a spoofed actor ID does not match the approved principal.

---

## 15. Attack #10 -- Executor Subversion

**Result: Not applicable -- no executors are registered in production.**

The executor registry is empty in both production binaries. Even if `ExecuteAction` were called, there would be no executor to invoke.

The tests register a `test_action` executor and verify that:
- Executors cannot create authority (`TestCB01`)
- Executors cannot approve targets
- Executors cannot mutate belief state
- Executors cannot manufacture authority-bearing evidence

---

## 16. Attack #11 -- Direct Provider Bypass

**Result: No bypass found.**

No production server path makes external provider calls. The only external calls are:
- `internal/corpus/embed.go`: Bedrock `InvokeModel` for embedding (read-only)
- `cmd/corpus-ingest/fetch.go`: GitHub CLI and HTTP client (CLI tool, not production server)

No provider output can reach an executor because no executor is reachable.

---

## 17. Attack #12 -- Intent Creation vs Execution

**Result: Verified. Intent creation != execution.**

Production paths:
- Wizard `/api/authorize` -> `s.Authorize` -> `PrepareForAction` (verification) + `kernel.IntentOnPromoted` (intent creation)
- MCP `solvent_authorize_action` -> `PrepareForAction` (verification) + `kernel.IntentOnPromoted` (intent creation)

Neither path calls `ExecuteAction`. Intent creation is clearly separated from execution.

However, the `cmd/operator-review/main.go` CLI calls `kernel.IntentOnPromoted` directly without `PrepareForAction`. This is a CLI tool, not a production server, but it skips the authority verification layer.

---

## 18. Attack #13 -- Audit Semantics

**Result: Audit types are defined but never emitted in production.**

`service/audit/audit.go` defines these execution-related activity types:
- `ActivityAdapterInvoked`
- `ActivityProviderResponded`
- `ActivityExecutorCompleted`
- `ActivityExecutorFailed`
- `ActivityExecutorDenied`

These are only emitted from `service/authority.ExecuteAction`, which is never called from production.

Production audit entries come from:
- Wizard `refuse` and `promote` paths
- MCP tool handlers
- Pipeline

None of these emit execution-related audit entries.

---

## 19. Attack #14 -- Dead Security Boundaries

**Result: Multiple dead security boundaries found.**

| Component | Status | Production Imports |
|-----------|--------|-------------------|
| `service/authority.ExecuteAction` | DEAD | 0 |
| `service/authority.GetToken` | DEAD | 0 |
| `service/workflow` (entire package) | DEAD | 0 |
| `service/policy.EvaluateConstraints` | DEAD | 0 |
| `service/audit` execution types | DEAD | 0 |

---

## 20. Attack #15 -- DB/Transaction Semantics for TOCTOU

**Result: Verified in code, untested in production.**

`kernel.Authorize` runs inside `crdb.ExecuteTx` with SERIALIZABLE isolation. It reads:
1. `target_activation` (existence check)
2. `target_snapshot` (tuple fields)
3. `target_revocation` (absence check)
4. `belief` table for each justification's current status

`PrepareForAction` calls `kernel.Authorize` and `ExecuteAction` calls `PrepareForAction` again. This means execution-time authorization re-reads all current state inside a fresh transaction.

But since no production code calls `ExecuteAction`, this is never exercised in production.

---

## 21. Attack #16 -- Test Quality

**Result: Tests are strong for the boundary they test, but they test isolated unit-level code, not production paths.**

The 21 integration tests in `service/authority/authority_integration_test.go` directly call `svc.ExecuteAction` and `svc.PrepareForAction` against live CockroachDB. They verify:

| Test | What it proves |
|------|---------------|
| `TestEA01_NoAuthorityTarget` | No target -> DENIED, executor not called |
| `TestEA02_PromotedNoAuthority` | No authority -> DENIED, executor not called |
| `TestEA03_WrongTarget` | Wrong target -> DENIED, executor not called |
| `TestEA04_WrongAction` | Wrong action -> DENIED, executor not called |
| `TestEA05_RevokedAuthority` | Revoked -> DENIED, executor not called |
| `TestEA06_PolicyAllowNoAuthority` | Policy ALLOW + no authority -> DENIED |
| `TestEA07_PolicyAllowRevokedAuthority` | Policy ALLOW + revoked -> DENIED |
| `TestEA08_TargetMutatesBetweenPrepareAndExecute` | Revocation between prepare and execute -> DENIED |
| `TestEA09_ActionMutatesBetweenPrepareAndExecute` | Action change between prepare and execute -> DENIED |
| `TestEA10_FakeApproval` | Wrong tuple -> DENIED |
| `TestCB01_ExecutorCannotCreateAuthority` | Executor cannot grant itself authority |
| `TestCB02_ProviderOutputNotAuthority` | Provider output != authority |
| `TestCB03_WorkflowStateCannotCreateAuthority` | Workflow state != authority |
| `TestCB04_AgentCannotApproveItself` | Wrong principal -> DENIED |
| `TestCB05_ActorSpoofingFails` | Spoofed actor -> DENIED |
| `TestAI01_MalformedOutputDenied` | Malformed params != authority |
| `TestAI02_ProviderClaimsCannotManufactureAuthority` | Provider claims != authority |
| `TestCR_A_RevokedAfterPrepare` | Revocation after prepare -> DENIED |
| `TestCR_B_TargetChangedAfterPrepare` | Target change after prepare -> DENIED |
| `TestCR_C_ActionChangedAfterPrepare` | Action change after prepare -> DENIED |
| `TestP01_ValidAuthExecutes` | Valid auth -> executor called |

**Mutation testing mindset:**

| Security Control Removed | Tests That Would Fail |
|--------------------------|----------------------|
| `kernel.Authorize` removed | ALL 21 tests |
| Revocation checking removed | `TestEA05`, `TestEA07`, `TestCR_A` |
| Target matching removed | `TestEA03`, `TestCR_B` |
| Action matching removed | `TestEA04`, `TestCR_C` |
| `ExecuteAction` bypassed preparation | `TestEA08`, `TestEA09`, `TestCR_A`, `TestCR_B`, `TestCR_C` |
| Executor invoked before authorization | `TestEA01`, `TestEA02`, `TestEA03`, `TestEA04`, `TestEA05` |

The tests are strong for the isolated boundary.

---

## 22. Kernel Preservation

**Result: No kernel modifications.**

The kernel (`kernel/`) has not been modified. Authority semantics remain:
- `kernel.Authorize` = authority oracle (read-only)
- `kernel.Approve` = authority creation
- `kernel.RevokeTarget` = revocation
- `kernel.IntentOnPromoted` = intent creation (database-gated)

No service-level replacement exists.

---

## 23. Findings by Severity

### F-1 (HIGH): No Production Execution Path

**File:** `service/authority/authority.go`
**Function:** `ExecuteAction`
**Attack Scenario:** The remediation established a security boundary in `ExecuteAction`, but no production code calls it. The executor registry is empty. There is no route, handler, or tool that can invoke any executor.
**Why It Works:** Exhaustive search of all production entry points (wizard routes, MCP tools, CLI commands, adapters) reveals zero callers of `ExecuteAction`.
**Expected Architecture:** Every consequential production execution path reaches `ExecuteAction` -> `PrepareForAction` -> `kernel.Authorize` -> `Executor`.
**Actual Architecture:** `ExecuteAction` is dead code. No production execution path exists.
**Recommended Fix:** Wire `ExecuteAction` into production paths before Phase 4, or document that execution is intentionally deferred and the boundary is currently unguarded.
**Code Change Required:** Yes -- add production execution paths that call `ExecuteAction`, or remove the dead code and accept that the system currently has no execution capability.

### F-2 (HIGH): Workflow Service Is Dead Code

**File:** `service/workflow/workflow.go`
**Function:** All
**Attack Scenario:** The workflow service implements token lifecycle management, but it is not imported anywhere in production code. Its security boundary (token state machine) is unreachable.
**Why It Works:** Grep for `service/workflow` imports in production code returns zero results.
**Expected Architecture:** Workflow tokens gate execution through the state machine.
**Actual Architecture:** Workflow tokens are never created or used in production.
**Recommended Fix:** Either wire workflow tokens into the execution path or remove the dead code.
**Code Change Required:** Yes.

### F-3 (MEDIUM): `operator-review` CLI Bypasses `PrepareForAction`

**File:** `cmd/operator-review/main.go`
**Function:** `runReviewMode`
**Attack Scenario:** The CLI calls `kernel.IntentOnPromoted` directly without `PrepareForAction` authority verification. This is a CLI tool (not production server), but it skips the authority layer.
**Why It Works:** Line 167: `st.IntentOnPromoted(ctx, scenarioID, beliefID, action)` is called directly.
**Expected Architecture:** All intent creation should go through `PrepareForAction` for authority verification.
**Actual Architecture:** The CLI bypasses the authority service.
**Recommended Fix:** Call `authSvc.PrepareForAction` before `IntentOnPromoted` in the CLI, or document that the CLI is a trusted administrative tool.
**Code Change Required:** No -- this is a CLI tool, not a production server. The database still enforces belief promotion.

### F-4 (MEDIUM): MCP `solvent_authorize_action` Conditionally Skips `PrepareForAction`

**File:** `cmd/solvent-mcp/tools.go`
**Function:** `handleSolventAuthorizeAction`
**Attack Scenario:** When `target_id` or `actor_id` are missing from the request, `PrepareForAction` is skipped. The MCP tool creates an intent without authority verification.
**Why It Works:** Lines 248-264: `if targetID != "" && actorID != ""` guards the `PrepareForAction` call.
**Expected Architecture:** Every intent creation should be authority-verified.
**Actual Architecture:** Intent creation without authority verification when target/actor are missing.
**Recommended Fix:** Either require `target_id` and `actor_id` for all intent creation, or document that missing fields mean "no authority verification."
**Code Change Required:** No -- this is intent creation, not execution. The database still enforces belief promotion.

### F-5 (LOW): Policy Service Is Not Wired Into Production

**File:** `service/policy/policy.go`
**Function:** `EvaluateConstraints`
**Attack Scenario:** Policy constraints are never evaluated in any production execution path because no execution path exists.
**Why It Works:** `EvaluateConstraints` is never called from any production handler, route, or tool.
**Expected Architecture:** Policy constrains authority before execution.
**Actual Architecture:** Policy exists but is never invoked.
**Recommended Fix:** Wire `EvaluateConstraints` into `PrepareForAction` or `ExecuteAction` when an execution path is added.
**Code Change Required:** No -- policy is correct but unused.

---

## 24. Final Verdict

**HOLD**

### Reasons:

1. **F-1 (HIGH): No production execution path exists.** The security boundary is dead code. `ExecuteAction` is never called from production. The executor registry is empty. There is no route, handler, or tool that can invoke any executor.

2. **F-2 (HIGH): Workflow service is dead code.** Not imported anywhere in production.

3. **Dead security boundaries remain.** The most critical security boundary (`ExecuteAction`) is unreachable from production.

### What the review confirms:

- The code-level security boundary is **correct** and **well-tested**
- No bypass exists because there is no execution path to bypass
- `kernel.Authorize` is the final authority oracle where it is called
- Exact tuple binding is preserved in the code
- Revocation is enforced in the code
- Policy cannot manufacture authority
- Workflow cannot manufacture authority (because it's dead code)
- Actor spoofing fails in the tests
- No direct provider bypass exists
- Executor cannot create authority
- Denied paths cannot reach executor (in tests)
- Live integration tests remain valid

### What the review does NOT confirm:

- That any production path is authority-gated (because no production execution path exists)
- That the boundary is reachable from production
- That denied paths cannot reach the executor in production (because no path reaches the executor at all)

### Required before GO:

1. Wire `ExecuteAction` into at least one production execution path, OR
2. Document that execution is intentionally deferred and the boundary is currently unguarded, AND
3. Remove or document dead security boundaries (`service/workflow`, unused `ExecuteAction`)

Do not proceed to Phase 4 on HOLD.
