# Phase 4C+ — Adversarial Security / Architecture Code Review

**Reviewer:** Kilo Code (independent, did not implement Phase 4C+)
**Date:** 2026-09-06
**Status:** NO-GO

---

## 1. VERDICT

**NO-GO**

The implementation is architecturally incomplete and does not satisfy the approved Phase 4C+ contract. While the kernel and service layer changes are structurally sound, three critical deliverables are missing or disconnected:

1. The adapter-level test suite (`adapter/github/executor_test.go`) does not exist
2. The GitHub executor is not registered in the production MCP server (`cmd/solvent-mcp/main.go`)
3. The `FakeGitHubProvider` is dead code — never referenced by any test file

Without the adapter tests, there is no adversarial proof that the executor boundary, parameter binding, or provider contract hold in practice. Without executor registration, the first real external executor is unreachable. These are not cosmetic gaps — they block the phase from delivering its stated objective: "prove that Solvent can control a real external side effect through its authorization kernel."

---

## 2. EXECUTIVE ASSESSMENT

The Phase 4C+ implementation plan (`docs/OS/plan_4c_plus.2.md`) and the approved test target (`docs/OS/phase4c_plus_test_target.md`) define a precise, deterministic, adversarial acceptance contract. The implementation satisfies the architectural contract at the kernel and service layer:

- `CompleteIntent` is correctly placed in the `Contract` interface and is only called after provider acceptance
- Executor selection is derived from the authorized action via a fixed mapping (`actionExecutorMap`), not from caller-supplied `params["tool_name"]`
- Execution parameters are reconstructed from `decision.ConsequenceParameters` (the approved snapshot), ignoring caller-supplied values
- The exact `intentID` is verified against the database before execution
- There is exactly one database read between T2 and T3 (intent state verification)
- `CompleteIntent` failure does not overwrite `result.Success = true`
- Audit semantics correctly distinguish authorization, invocation, provider acceptance, provider failure, and intent completion failure

However, the implementation fails to deliver the acceptance contract in three material ways:

| Missing Deliverable | Contract Requirement | Status |
|---------------------|---------------------|--------|
| `adapter/github/executor_test.go` | TestExec01–38, TestAT01–16 | **ABSENT** |
| Executor registration in `cmd/solvent-mcp/main.go` | Step 5: "Register executor in main.go" | **NOT DONE** |
| `examples/github/executor_example.go` | Step 5: runnable example | **ABSENT** |

Additionally, the `FakeGitHubProvider` exists in `adapter/github/fake_provider.go` but is never imported or used by any test file. This means the deterministic provider test harness — which the test target identifies as the foundation for proving parameter binding, executor isolation, and failure semantics — is entirely disconnected from the test suite.

The existing integration tests in `service/authority/authority_integration_test.go` use `executor.RecordingFunc` (a generic test double) rather than the real `FakeGitHubProvider`. They verify authorization denial and basic success paths but do not exercise the GitHub-specific executor boundary, parameter reconstruction, or provider contract.

---

## 3. FINDINGS

### F-1 (CRITICAL): Adapter test suite does not exist

**File:** `adapter/github/executor_test.go` (MISSING)
**Section:** Test target §12, §15

**Issue:** The test target document specifies 38 test functions (TestExec01–38) and 16 adversarial tests (TestAT01–16) that must reside in `adapter/github/executor_test.go`. This file does not exist in the repository. `go test ./adapter/github/...` runs only the 4 webhook-processing tests in `github_test.go`; zero executor tests execute.

**Why it matters:** The entire adversarial proof for Phase 4C+ depends on these tests. Without them:
- Parameter binding at the executor level is unproven
- Executor selection security is untested against the real GitHub executor
- Window B characterization is unverified
- `CompleteIntent` call provenance is unverified
- Audit semantics for the execution path are unverified
- Provider failure/ambiguity handling is unverified

**Remediation:** Implement `adapter/github/executor_test.go` with all tests specified in the test target. The tests must use `FakeGitHubProvider` to verify exact parameter reception, provider call count, acceptance/rejection behavior, and synchronization.

---

### F-2 (CRITICAL): GitHub executor not registered in production

**File:** `cmd/solvent-mcp/main.go`
**Line:** 126

**Issue:** The executor registry is instantiated but empty:
```go
execReg := executor.NewRegistry()
```
No call to `github.RegisterExecutor(execReg, provider)` exists anywhere in the production entry points. The `github` package's `RegisterExecutor` function (in `adapter/github/executor.go:45`) is never called.

**Why it matters:** Phase 4C+'s stated objective is to "add Solvent's first real external executor." If the executor is not registered, `ExecuteAction` will always fail with `"no executor registered for action: deploy"` when it encounters a `deploy` action. The external side effect cannot occur. The phase delivers no consequential execution capability.

**Remediation:** Register the GitHub executor in `cmd/solvent-mcp/main.go` (gated on `GITHUB_TOKEN` as specified in the plan). Also verify registration in any other production entry point that uses `ExecuteAction`.

---

### F-3 (CRITICAL): FakeGitHubProvider is dead code

**File:** `adapter/github/fake_provider.go`
**Issue:** `FakeGitHubProvider` is defined but never imported, instantiated, or called by any test file in the repository. `grep` confirms zero references outside the `adapter/github` package.

**Why it matters:** The test target document identifies `FakeGitHubProvider` as the "deterministic provider test harness" that proves:
- Whether the executor was called
- Exact repo/workflow/ref/inputs received
- Number of provider calls
- Accepted/rejected behavior
- Returned run reference
- Controllable errors
- Ambiguous/lost-response behavior

If the fake provider is never used, none of these properties are proven by any test that actually runs.

**Remediation:** Ensure `adapter/github/executor_test.go` imports and uses `FakeGitHubProvider` for all executor-level tests.

---

### F-4 (HIGH): TestExec36 (executor-level parameter binding) does not exist

**File:** `adapter/github/executor_test.go` (MISSING)
**Section:** Test target §12.2

**Issue:** The test target specifies TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams as the critical proof that "caller-supplied execution parameters are ignored" and "the snapshot is the sole source of truth." This test does not exist.

**Why it matters:** The existing integration tests (TestP01_ValidAuthExecutes, TestEA03_WrongTarget, etc.) use `RecordingFunc`, which records the `params` map it receives but does not verify that the values came from the approved snapshot rather than the caller. A buggy implementation that passes caller-supplied params directly to the executor would pass all existing tests when the caller happens to supply matching values.

**Remediation:** Implement TestExec36 with a custom executor that records the exact parameter map it receives. The test must verify:
- Variant a: caller supplies matching + extra fields → executor receives ONLY approved fields
- Variant b: caller supplies conflicting values → executor receives snapshot values, NOT caller values
- Variant c: caller supplies arbitrary extra provider-looking fields → those fields are stripped

---

### F-5 (HIGH): TestExec35 (SI-9 — provider success does not create authority) does not exist

**File:** `adapter/github/executor_test.go` (MISSING)
**Section:** Test target §12.7

**Issue:** SI-9 states "Provider success does not retroactively create authority." The test target specifies TestExec35 to prove this by capturing pre-execution authority state and verifying byte-for-byte equality afterward. This test does not exist.

**Why it matters:** Without this test, there is no proof that provider acceptance does not trigger side effects in the authority tables (e.g., creating a new target, activating a target, promoting a belief). A buggy executor that calls kernel methods after provider success would pass all existing tests because no test inspects authority state after execution.

**Remediation:** Implement TestExec35. The test must:
1. Capture pre-execution state of target, activation, target_snapshot, belief, and principal rows
2. Execute with provider accepting
3. Re-query all captured rows and assert byte-for-byte equality (except intent state transition)
4. The test must fail if any new authority row is created or any existing row is modified

---

### F-6 (HIGH): TestExec38 (F-18 — provider success, Solvent error) does not exist

**File:** `adapter/github/executor_test.go` (MISSING)
**Section:** Test target §12.4

**Issue:** F-18 describes a distinct failure mode: "Provider succeeds but Solvent receives error — Side effect happened. Solvent reports failure." The test target specifies TestExec38 to prove this case is handled correctly (execution result truthful, intent stays live, audit records the discrepancy). This test does not exist.

**Why it matters:** Without this test, the implementation could conflate provider acceptance with provider completion, or incorrectly report success when Solvent receives an error after the provider actually accepted. The distinction between "provider accepted" and "Solvent received confirmation" is critical for audit integrity.

**Remediation:** Implement TestExec38 using `FakeGitHubProvider.SetLostResponse(true)` or an equivalent mechanism. Verify:
- Provider was called exactly 1 time
- `result.Success == false`
- `result.Error` indicates Solvent-side error
- Intent state remains `live`
- Audit contains `executor_failed` with the error

---

### F-7 (HIGH): Adversarial tests (TestAT01–16) do not exist

**File:** `adapter/github/executor_test.go` (MISSING)
**Section:** Test target §12.8

**Issue:** The test target specifies 16 adversarial tests that remove or bypass specific security controls and verify the relevant test fails. None of these tests exist.

**Why it matters:** Adversarial tests are the only proof that security controls are actually enforced. For example:
- TestAT05 (substitute arbitrary executor) would fail if `params["tool_name"]` were used for executor selection
- TestAT16 (executor uses caller params) would fail if the executor read from caller params instead of snapshot
- TestAT01 (remove authorization check) would fail if the executor were called without authorization

Without these tests, the security controls are documented but unproven.

**Remediation:** Implement all 16 adversarial tests specified in the test target.

---

### F-8 (MEDIUM): `examples/github/executor_example.go` does not exist

**File:** `examples/github/executor_example.go` (MISSING)
**Section:** Plan §22, §23

**Issue:** The plan specifies `examples/github/executor_example.go` as a runnable example demonstrating the GitHub executor integration. This file does not exist. The `examples/github/` directory contains `integration.go` (which calls the REST API for belief/evidence operations) and a compiled binary `github` (8.6MB), but no executor example.

**Why it matters:** The example serves as documentation for how the executor is wired and used. Its absence makes it harder for operators to understand the execution path.

**Remediation:** Create `examples/github/executor_example.go` demonstrating the executor wiring and a simple execution flow.

---

### F-9 (LOW): Compiled binary committed to repository

**File:** `examples/github/github` (8.6MB ELF binary)
**Issue:** A compiled Linux binary is present in the repository. Binaries should not be committed to version control.

**Why it matters:** Binaries in the repository bloat the repo, may contain platform-specific code, and violate standard Go project conventions.

**Remediation:** Remove the binary from the repository and add it to `.gitignore`.

---

## 4. MANDATORY QUESTIONS

### 1. Is the snapshot consequence_parameters truly the sole source of provider execution parameters?

**YES — at the code level.** `service/authority/authority.go` lines 245–257 reconstruct `execParams` exclusively from `decision.ConsequenceParameters`, which came from `kernel.Authorize`. The caller-supplied `params` map is never consulted for `repo`, `workflow`, or `ref`.

**BUT UNPROVEN.** TestExec36, which is supposed to prove this at the executor level, does not exist. The existing integration tests use `RecordingFunc`, which records whatever `params` it receives but does not verify the values came from the snapshot.

### 2. Can any caller-controlled field influence which GitHub repository, workflow, or ref is actually invoked?

**NO — at the code level.** The executor receives only `snapParams["repo"]`, `snapParams["workflow"]`, and `snapParams["ref"]`. Caller-supplied values in `params` are ignored.

**BUT UNPROVEN.** No test verifies that a malicious caller cannot influence the provider parameters.

### 3. Can any caller-controlled field select an arbitrary executor?

**NO — at the code level.** `resolveExecutor(action)` maps the authorized action to a fixed executor name. `params["tool_name"]` is never consulted for executor selection.

**BUT UNPROVEN.** TestAT05 (substitute arbitrary executor) does not exist.

### 4. Is exact intentID preserved all the way through CompleteIntent?

**YES.** `ExecuteAction` receives `intentID` as a parameter and passes it directly to `CompleteIntent`. The SQL statement verifies `id = $1::UUID AND scenario_id = $2::UUID`. No `LIMIT 1` or arbitrary selection exists.

### 5. Can an intent other than the requested intent accidentally be executed or marked executed?

**NO — at the code level.** The intent verification query includes `id = $1::UUID`, ensuring only the exact intent is checked. `CompleteIntent` also includes `id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`.

### 6. Does revocation before final authorization reliably deny execution?

**YES — at the code level.** `PrepareForAction` re-reads current state by calling `kernel.Authorize`, which reads `target_activation`, `target_snapshot`, and checks for `target_revocation`. If revocation occurred before T2, the re-read detects it and returns `Allowed=false`.

**BUT PARTIALLY PROVEN.** TestCR_A_RevokedAfterPrepare tests this scenario, but only for the `PrepareForAction` → `ExecuteAction` split, not for the concurrent revocation race during `AuthorizeAndCreateIntent`.

### 7. Is the Window B race correctly characterized, with no false atomicity claim?

**YES — at the documentation level.** The test target explicitly states that Window B exists, is bounded by a local function call, and is a documented v1 limitation. It does not claim atomic DB authorization + provider execution.

**BUT UNTESTED.** TestExec15B does not exist.

### 8. Are there any additional database reads between T2 and T3 beyond the one intent-state verification read?

**NO — at the code level.** Between `PrepareForAction` (T2) and `fn(ctx, execParams)` (T3), the only database operation is the intent state query (lines 229–243). All parameter reconstruction happens in-memory from `decision.ConsequenceParameters`.

### 9. Can concurrent ExecuteAction calls cause duplicate external side effects?

**YES — this is a known v1 limitation.** The intent state check (`state = 'live'`) prevents sequential duplicates, but concurrent calls can both pass the check before either transitions the state to `executed`. The test target documents this as a known limitation.

### 10. What happens if the provider accepts execution but CompleteIntent fails?

**CORRECTLY HANDLED.** `service/authority/authority.go` lines 298–315:
- `result.Success` remains `true` (provider DID accept)
- Audit logs `ActivityIntentCompletionFailed`
- The execution result is truthful
- This matches architectural requirement #10

### 11. What happens if the provider definitely fails but the service receives an error?

**CORRECTLY HANDLED.** Lines 275–291: `execErr != nil` → `result.Success = false`, audit logs `ActivityExecutorFailed`, intent stays `live`.

### 12. What happens if the provider response is ambiguous/lost?

**PARTIALLY HANDLED.** The `FakeGitHubProvider` has a `lostResponse` mode that returns an error after recording the call. The service treats this as `executor_failed`. However, without TestExec38, the exact audit trail and intent state behavior are unverified.

### 13. Can executor code itself alter Solvent authority state?

**NO — at the code level.** The executor receives only `execParams` (repo, workflow, ref). It has no database access, no kernel store reference, and no authority methods. The `ActionFunc` signature returns `(string, error)` — it cannot mutate Solvent state.

### 14. Has any GitHub-specific semantics leaked into kernel authority semantics?

**NO.** The kernel stores `consequence_parameters` as opaque JSONB. It performs semantic JSON comparison via `jsonEqual`. The kernel does not know what `repo`, `workflow`, or `ref` mean. All GitHub-specific logic lives in `adapter/github/`.

### 15. Has any new code accidentally created a second authorization engine?

**NO.** `service/authority/authority.go` delegates to `kernel.Authorize` for the authoritative allow/deny determination. The service does not perform independent authority checks. The `actionExecutorMap` is an executor routing table, not an authorization engine.

### 16. Can MCP/REST/API callers bypass the intended execution path?

**NO — at the code level.** `CompleteIntent` is not exposed through any REST or MCP handler. The only production call path is `service/authority.ExecuteAction`. However, TestExec30 (structural verification) does not exist.

### 17. Can caller-controlled actor identity or actor type be used as authority proof?

**NO.** `actorID` is a parameter to `PrepareForAction` and `ExecuteAction`. It is included in the `AuthorityTuple` and verified by `kernel.Authorize` against the target's `principal_id`. A spoofed actorID would cause a principal mismatch and denial.

### 18. Are audit events semantically correct?

**YES — at the code level.** The audit service defines distinct activity types:
- `ActivityAuthorizationGranted` / `ActivityAuthorizationDenied`
- `ActivityExecutorDenied`
- `ActivityAdapterInvoked`
- `ActivityExecutorCompleted`
- `ActivityExecutorFailed`
- `ActivityIntentCompletionFailed`

Each is logged at the appropriate point in `ExecuteAction`. However, without TestExec31–34, the ordering and completeness of audit entries are unverified.

### 19. Does the implementation preserve Authorize != Execute?

**YES.** `PrepareForAction` calls `kernel.Authorize` (read-only). `ExecuteAction` calls the executor. They are separate operations with separate audit entries. Authorization results are not cached or reused.

### 20. Does the implementation preserve TOKEN != AUTHORITY?

**YES.** The `action_intent` row is created by `AuthorizeAndCreateIntent` based on kernel authority evaluation. The executor does not create or modify intents. Provider output cannot create authority. This is verified by existing tests TestCB01_ExecutorCannotCreateAuthority and TestCB02_ProviderOutputNotAuthority.

---

## 5. SECURITY INVARIANT VERIFICATION

| Invariant | Status | Evidence |
|-----------|--------|----------|
| SI-1 | **PASS (code) / UNTESTED** | `ExecuteAction` calls `PrepareForAction` first; executor only invoked if `Allowed=true`. But no test verifies this at the executor level. |
| SI-2 | **PASS** | Executor has no authority methods. `CompleteIntent` is only called from `ExecuteAction` after provider acceptance. |
| SI-3 | **PASS** | Executor cannot revoke. No revocation methods exposed. |
| SI-4 | **PASS** | If `PrepareForAction` returns `Allowed=false`, executor is never invoked. |
| SI-5 | **PASS (code) / PARTIALLY PROVEN** | `PrepareForAction` re-reads current state. TestCR_A proves revocation between prepare and execute is caught. But concurrent revocation during `AuthorizeAndCreateIntent` is not tested. |
| SI-6 | **PASS (code) / UNTESTED** | `resolveExecutor(action)` uses fixed mapping. But TestAT05 does not exist. |
| SI-7 | **PASS** | Wrong target/action/actor causes `kernel.Authorize` to deny. Verified by TestEA03, TestEA04, TestCB04. |
| SI-8 | **PASS** | Authorization and execution are separate operations with separate audit entries. |
| SI-9 | **PASS (code) / UNTESTED** | Executor receives no kernel store reference. But TestExec35 does not exist. |
| SI-10 | **PASS** | Provider error → `result.Success = false`. TestEA covers denial paths. |
| SI-11 | **PASS (code) / UNTESTED** | Audit entries are distinct types. But TestExec31–34 do not exist. |
| SI-12 | **PASS** | Provider rejection → intent stays `live`. `result.Success = false`. |
| SI-13 | **PASS** | Authorization alone does not trigger execution. |
| SI-14 | **PASS** | Executor only invoked after `Allowed=true`. |
| SI-15 | **PASS** | Revoked target → `kernel.Authorize` denies. Verified by TestEA05, TestCR_A. |
| SI-16 | **PASS (code) / UNTESTED** | Params reconstructed from `decision.ConsequenceParameters`. But no test verifies caller params are ignored. |
| SI-17 | **PASS (code) / UNTESTED** | Same as SI-16. |
| SI-18 | **PASS (code) / UNTESTED** | Executor receives only `execParams` (repo, workflow, ref). But no test verifies extra caller fields are stripped. |

---

## 6. ARCHITECTURAL BOUNDARY REVIEW

| Boundary | Status | Evidence |
|----------|--------|----------|
| Kernel authority oracle | **PRESERVED** | `kernel.Authorize` is the sole authority determination. Service does not reimplement it. |
| No GitHub semantics in kernel | **PRESERVED** | Kernel stores `consequence_parameters` as opaque JSONB. No repo/workflow/ref knowledge. |
| Executor in extension plane | **PARTIAL** | Executor code exists in `adapter/github/` but is NOT registered in production. |
| CompleteIntent as sole new kernel primitive | **PRESERVED** | Only new kernel method. Call provenance is strictly limited. |
| No second authorization engine | **PRESERVED** | Service delegates to kernel. `actionExecutorMap` is routing, not authorization. |
| Authorize != Execute | **PRESERVED** | Separate operations, separate audit entries. |
| TOKEN != AUTHORITY | **PRESERVED** | Provider output cannot create authority. Executor cannot create authority. |

---

## 7. TOCTOU ANALYSIS

### Window A (revocation before final check)
**Status: MITIGATED (code) / PARTIALLY PROVEN**

The existing tests TestCR_A, TestEA08 prove that revocation between `PrepareForAction` and `ExecuteAction` is caught. The `kernel.Authorize` re-read sees the revocation.

However, these tests do not cover concurrent revocation during `AuthorizeAndCreateIntent` (which uses `FOR UPDATE` locking). The locking mechanism should serialize correctly, but there is no test for the concurrent case.

### Window B (revocation after final check, before provider call)
**Status: DOCUMENTED / UNTESTED**

The code correctly implements the local function-call boundary: after `kernel.Authorize` succeeds and the intent is verified live, the executor is invoked immediately with no intervening I/O. The test target acknowledges Window B as a documented v1 limitation.

However, TestExec15B does not exist, so the characterization of this race is unverified.

### Window C (GitHub state changes after provider call)
**Status: DOCUMENTED**

Outside Solvent's control. Not tested, which is correct.

---

## 8. EXECUTOR / PROVIDER TRUST-BOUNDARY ANALYSIS

### Executor isolation
**Status: CORRECT (code) / UNTESTED**

The executor:
- Receives only `execParams` (repo, workflow, ref)
- Has no database access
- Has no kernel store reference
- Returns `(string, error)` — cannot mutate Solvent state
- Is resolved from `actionExecutorMap`, not caller params

But the executor boundary is untested because `adapter/github/executor_test.go` does not exist.

### Provider contract
**Status: CORRECT (code) / UNTESTED**

`FakeGitHubProvider` correctly implements the `GitHubProvider` interface with:
- Call recording
- Deterministic acceptance/rejection
- Lost-response simulation
- Controllable errors
- Synchronization via `WaitForCalls`

But `FakeGitHubProvider` is never used in any test.

### Credential handling
**Status: CORRECT (design)**

The plan specifies `GITHUB_TOKEN` env var, injected at construction, never logged. The real provider implementation does not exist yet, so this is design-only.

---

## 9. AUDIT SEMANTICS ANALYSIS

| Event | Audit Type | Status |
|-------|-----------|--------|
| Authorization granted | `ActivityAuthorizationGranted` | Implemented |
| Authorization denied | `ActivityAuthorizationDenied` | Implemented |
| Executor denied | `ActivityExecutorDenied` | Implemented |
| Adapter invoked | `ActivityAdapterInvoked` | Implemented |
| Provider accepted | `ActivityExecutorCompleted` | Implemented |
| Provider rejected | `ActivityExecutorFailed` | Implemented |
| CompleteIntent failed | `ActivityIntentCompletionFailed` | Implemented |

**Gap:** No test verifies audit entry ordering (TestExec31), denied execution audit entries (TestExec32), provider failure audit entries (TestExec33), or audit failure after provider success (TestExec34).

---

## 10. FINDINGS TABLE

| ID | Severity | Area | Finding |
|----|----------|------|---------|
| F-1 | CRITICAL | Tests | `adapter/github/executor_test.go` does not exist |
| F-2 | CRITICAL | Wiring | GitHub executor not registered in `cmd/solvent-mcp/main.go` |
| F-3 | CRITICAL | Tests | `FakeGitHubProvider` is dead code — never used in tests |
| F-4 | HIGH | Parameter binding | TestExec36 (executor-level parameter proof) missing |
| F-5 | HIGH | Security invariant | TestExec35 (SI-9 — provider success doesn't create authority) missing |
| F-6 | HIGH | Failure semantics | TestExec38 (F-18 — provider success, Solvent error) missing |
| F-7 | HIGH | Adversarial | All adversarial tests (TestAT01–16) missing |
| F-8 | MEDIUM | Documentation | `examples/github/executor_example.go` missing |
| F-9 | LOW | Repository hygiene | 8.6MB compiled binary in `examples/github/` |

---

## 11. FINAL RECOMMENDATION

**NO-GO — Phase 4C+ implementation is incomplete.**

The kernel and service layer changes are structurally sound and match the approved architectural contract. However, the implementation fails to deliver the acceptance contract in three critical ways:

1. **No adapter tests exist.** The test target document specifies 54 tests (38 execution + 16 adversarial) in `adapter/github/executor_test.go`. This file does not exist. Without these tests, the security guarantees of the executor boundary, parameter binding, and provider contract are unproven.

2. **The executor is unreachable.** The GitHub executor exists in `adapter/github/executor.go` but is not registered in `cmd/solvent-mcp/main.go`. The executor registry is empty. Phase 4C+ cannot deliver its stated objective — "prove that Solvent can control a real external side effect" — if the executor is never invoked.

3. **The fake provider is disconnected.** `FakeGitHubProvider` exists but is never used. The deterministic test harness that the test target identifies as the foundation for proving security properties is dead code.

**Before Phase 4C+ can receive GO:**

1. Implement `adapter/github/executor_test.go` with all TestExec01–38 and TestAT01–16
2. Register the GitHub executor in `cmd/solvent-mcp/main.go`
3. Ensure `FakeGitHubProvider` is used by all adapter-level tests
4. Create `examples/github/executor_example.go`
5. Remove the committed binary from `examples/github/`
6. Re-run the full test suite and verify all new tests pass

The current implementation is a partial delivery that satisfies the kernel contract but does not deliver the executable acceptance contract. OpenCode must complete the missing test and wiring work before this phase can be approved.
