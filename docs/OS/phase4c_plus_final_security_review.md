# Phase 4C+ — Final Independent Adversarial Security / Architecture Review

**Reviewer:** Kilo Code (independent, did not implement Phase 4C+)
**Date:** 2026-09-06
**Status:** NO-GO

---

## 1. VERDICT

**NO-GO**

The implementation has made significant progress since the previous NO-GO review. All nine prior findings (F-1 through F-9) are now CLOSED. The kernel and service layer correctly implement the approved architectural contract. However, two MEDIUM-severity gaps remain in the acceptance test suite that prevent the implementation from genuinely satisfying the approved Phase 4C+ test target.

---

## 2. EXECUTIVE ASSESSMENT

### Repository State
```
master
64884f5 ✨ phase 4c
```

Working tree contains:
- New: `adapter/github/executor.go`, `executor_test.go`, `fake_provider.go`, `http_provider.go`, `provider.go`
- Modified: `cmd/solvent-mcp/main.go`, `kernel/authority.go`, `kernel/contract.go`, `kernel/sql.go`, `service/authority/authority.go`, `service/audit/audit.go`
- New example: `examples/github/executor/main.go`
- Removed: `examples/github/github` binary

### Verification Results
```
go build ./...       PASS
go vet ./...         PASS
go test -count=1 ./... PASS (all packages)
go test -race ./...  PASS (all packages)
go test ./adapter/github/... 55/55 PASS
```

### Previous NO-GO Findings — Closure Status

| ID | Severity | Finding | Status | Evidence |
|----|----------|---------|--------|----------|
| F-1 | CRITICAL | Missing adapter executor test suite | **CLOSED** | `adapter/github/executor_test.go` exists with 55 test functions (TestExec01–38, TestAT01–16) |
| F-2 | CRITICAL | GitHub executor not registered in production | **CLOSED** | `cmd/solvent-mcp/main.go:128` — `github.RegisterExecutor(execReg, provider)` gated on `GITHUB_TOKEN` |
| F-3 | CRITICAL | FakeGitHubProvider unused | **CLOSED** | `FakeGitHubProvider` instantiated in 30+ test functions, call recording, parameter capture, and synchronization verified |
| F-4 | HIGH | Missing TestExec36 | **CLOSED** | TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams exists and verifies executor receives only snapshot values |
| F-5 | HIGH | Missing TestExec35 | **CLOSED** | TestExec35_ProviderSuccessDoesNotCreateAuthority exists with byte-for-byte authority state comparison |
| F-6 | HIGH | Missing TestExec38 | **CLOSED** | TestExec38_ProviderSuccessSolventReceivesError exists and verifies F-18 semantics |
| F-7 | HIGH | Missing adversarial tests | **CLOSED** | TestAT01–16 all exist and exercise adversarial controls |
| F-8 | MEDIUM | Missing GitHub executor example | **CLOSED** | `examples/github/executor/main.go` exists and demonstrates full execution flow |
| F-9 | LOW | Committed 8.6MB binary | **CLOSED** | `examples/github/github` deleted, no longer tracked |

### Remaining Gaps

| ID | Severity | Finding |
|----|----------|---------|
| N-1 | MEDIUM | TestExec34 is a placeholder — does not simulate audit write failure |
| N-2 | MEDIUM | TestExec15B does not actually characterize Window B — runs normal execution without revocation race |
| N-3 | LOW | HTTP provider has no explicit timeout on `http.Client` |
| N-4 | LOW | TestExec30 does not verify REST/MCP cannot call `CompleteIntent` |

---

## 3. PREVIOUS FINDING CLOSURE DETAILS

### F-1: Adapter test suite does not exist → CLOSED

**Evidence:** `adapter/github/executor_test.go` (75KB) contains 55 test functions:
- TestExec01–38 (execution path tests)
- TestAT01–16 (adversarial tests)

All discovered by `go test ./adapter/github/...` and all pass.

### F-2: GitHub executor not registered → CLOSED

**Evidence:** `cmd/solvent-mcp/main.go:126-132`:
```go
if token := os.Getenv("GITHUB_TOKEN"); token != "" {
    provider := github.NewHTTPProvider(token)
    github.RegisterExecutor(execReg, provider)
    log.Info("github executor registered", "action", "deploy")
} else {
    log.Info("github executor not registered (GITHUB_TOKEN not set)")
}
```

Registration is gated on `GITHUB_TOKEN`, no hardcoded credentials, no credential logging.

### F-3: FakeGitHubProvider unused → CLOSED

**Evidence:** `grep -R "FakeGitHubProvider"` shows 30+ instantiations in `executor_test.go`. The fake provider is used for:
- Call recording (`Calls()`, `CallCount()`, `LastCall()`)
- Parameter capture (`WorkflowCall` struct)
- Acceptance/rejection (`SetAccept()`)
- Error injection (`SetErr()`)
- Lost-response simulation (`SetLostResponse()`)
- Synchronization (`WaitForCalls()`)

### F-4 through F-7: Missing tests → CLOSED

All specified tests (TestExec35, TestExec36, TestExec38, TestAT01–16) exist in `executor_test.go` and pass.

### F-8: Missing example → CLOSED

`examples/github/executor/main.go` demonstrates the full execution flow: create principal → enter belief → promote → create target → approve → create intent → execute.

### F-9: Committed binary → CLOSED

`git ls-files examples/github/github` returns empty. Binary is removed from working tree and git index.

---

## 4. NEW FINDINGS

### N-1 (MEDIUM): TestExec34 is a placeholder

**File:** `adapter/github/executor_test.go:1474-1480`
**Section:** Test target §12.7

**Issue:** TestExec34_AuditWriteFailureAfterProviderSuccess does not simulate an audit write failure. It logs two messages and returns:
```go
func TestExec34_AuditWriteFailureAfterProviderSuccess(t *testing.T) {
    t.Log("Audit gap is a known operational limitation...")
    t.Log("Full testing requires injecting audit write failure, which is deferred to Phase 4D.")
}
```

**Why it matters:** The test target explicitly requires:
- "Simulate audit write failure after provider acceptance"
- "Execution result remains truthful (`result.Success == true`)"
- "Audit entry may be missing (gap), but the execution did happen"

The current test verifies none of these. It does not inject an audit failure, does not execute any code path, and does not assert anything about execution results or audit gaps.

**Remediation:** Either implement the test with an injectable audit service that can simulate write failures, or move the test to Phase 4D with explicit documentation that the current acceptance contract does not require it.

---

### N-2 (MEDIUM): TestExec15B does not actually characterize Window B

**File:** `adapter/github/executor_test.go:729-760`
**Section:** Test target §12.3

**Issue:** TestExec15B_RevocationAfterCheck_DocumentedRace claims to characterize the Window B race but does not perform any revocation between T2 and T3. The test runs a normal `ExecuteAction` with no intervening revocation and logs the result:
```go
// In v1, there is no hook between T2 and T3. We characterize the behavior
// by calling ExecuteAction normally (T2 passes), then verifying the result.
// A revocation arriving in Window B would result in Allowed=true.
result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
    intentID, map[string]interface{}{}, "execution", testConsequenceParams())
```

**Why it matters:** The test target explicitly states:
> "Test requirement: Revocation after T2, before T3 → accepted as documented v1 race. Test must characterize the behavior, not falsely require prevention."

The current test does not characterize the behavior — it runs a normal execution and logs that Window B exists. It does not force the race, does not demonstrate the boundary, and provides no adversarial value. A test that always passes regardless of the implementation provides no evidence about the claimed limitation.

**Remediation:** Either:
1. Implement a deterministic synchronization mechanism to force revocation between T2 and T3, or
2. Reclassify the test as a documentation-only note and remove it from the gating test suite

---

### N-3 (LOW): HTTP provider has no explicit timeout

**File:** `adapter/github/http_provider.go:25`
**Section:** Part VIII

**Issue:** The HTTP client is instantiated with no timeout:
```go
httpClient: &http.Client{},
```

While the request uses `http.NewRequestWithContext(ctx, ...)` which respects context cancellation, there is no explicit `http.Client.Timeout`. A network stall or slow GitHub API response could cause the executor to block indefinitely if the caller does not cancel the context.

**Remediation:** Add a configurable timeout to the HTTP client, e.g.:
```go
httpClient: &http.Client{Timeout: 30 * time.Second},
```

---

### N-4 (LOW): TestExec30 does not verify REST/MCP cannot call CompleteIntent

**File:** `adapter/github/executor_test.go:1226-1262`
**Section:** Test target §12.6

**Issue:** TestExec30 verifies that `CompleteIntent` exists on the `Contract` interface and is callable from the kernel. However, it does not verify that no REST or MCP handler can call it. The test states:
> "No REST handler calls CompleteIntent (verified by grep)."

But no grep or AST analysis is performed. The test relies on manual code review.

**Remediation:** Add a structural verification that scans `cmd/solvent-mcp/main.go` and `api/` for any call to `CompleteIntent` and fails if found.

---

## 5. SECURITY INVARIANT MATRIX

| Invariant | Status | Evidence |
|-----------|--------|----------|
| SI-1 | **PASS** | `ExecuteAction` calls `PrepareForAction` first; executor only invoked if `Allowed=true`. Verified by TestExec02, TestExec03, TestAT01. |
| SI-2 | **PASS** | Executor has no authority methods. `CompleteIntent` only called from `ExecuteAction` after provider acceptance. |
| SI-3 | **PASS** | Executor cannot revoke. No revocation methods exposed. |
| SI-4 | **PASS** | If `PrepareForAction` returns `Allowed=false`, executor is never invoked. |
| SI-5 | **PASS** | `PrepareForAction` re-reads current state. TestExec08, TestExec15A, TestCR_A prove revocation is caught. |
| SI-6 | **PASS** | `resolveExecutor(action)` uses fixed mapping. `params["tool_name"]` is never consulted. Verified by TestExec09, TestAT05. |
| SI-7 | **PASS** | Wrong target/action/actor causes `kernel.Authorize` to deny. Verified by TestExec04, TestExec05, TestExec06. |
| SI-8 | **PASS** | Authorization and execution are separate operations with separate audit entries. |
| SI-9 | **PASS** | TestExec35 captures pre-execution authority state and verifies byte-for-byte equality afterward. |
| SI-10 | **PASS** | Provider error → `result.Success = false`. Verified by TestExec20, TestExec21, TestAT07. |
| SI-11 | **PARTIAL** | Audit entries are distinct types. TestExec31 verifies ordering. But TestExec34 is a placeholder. |
| SI-12 | **PASS** | Provider rejection → intent stays `live`. `result.Success = false`. Verified by TestExec28. |
| SI-13 | **PASS** | Authorization alone does not trigger execution. |
| SI-14 | **PASS** | Executor only invoked after `Allowed=true`. |
| SI-15 | **PASS** | Revoked target → `kernel.Authorize` denies. Verified by TestExec03, TestExec08, TestExec15A. |
| SI-16 | **PASS** | Params reconstructed from `decision.ConsequenceParameters`. TestExec36 proves caller params are ignored. |
| SI-17 | **PASS** | Same as SI-16. |
| SI-18 | **PASS** | Executor receives only `execParams` (repo, workflow, ref). TestExec36 verifies extra caller fields are stripped. |

---

## 6. TEST VERIFICATION

### Build and Vet
```
$ go build ./...
(no output)

$ go vet ./...
(no output)
```

### Unit Tests
```
$ go test -count=1 ./...
ok  	github.com/PithomLabs/solvent/adapter/github	24.181s
ok  	github.com/PithomLabs/solvent/api	19.518s
...
ok  	github.com/PithomLabs/solvent/service/authority	19.257s
```

### Race Detector
```
$ go test -race ./...
ok  	github.com/PithomLabs/solvent/adapter/github	18.150s
ok  	github.com/PithomLabs/solvent/api	11.557s
...
ok  	github.com/PithomLabs/solvent/service/authority	13.967s
```

All packages pass with race detector enabled.

### Adapter/Github Test Count
```
$ go test -v ./adapter/github/... | grep "^--- PASS"
55 PASS lines (including subtests)
```

Test functions discovered:
- TestExec01–38 (38 tests)
- TestAT01–16 (16 tests)
- TestProcessPush, TestProcessPR, TestProcessCIStatus, TestParseWebhookPush, TestTrimRef (5 existing tests)

Total: 55 test functions, all passing.

---

## 7. ADVERSARIAL TEST QUALITY ASSESSMENT

### Strong Tests

**TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams**
- Uses a custom `recordingExecutor` that captures the exact parameter map
- Passes conflicting caller params: `{"repo": "org/caller", "workflow": "caller.yml", "ref": "dev"}`
- Verifies executor receives ONLY `{"repo": "org/approved", "workflow": "deploy.yml", "ref": "main"}`
- This test would FAIL if the implementation used caller params instead of snapshot

**TestAT05_SubstituteArbitraryExecutor**
- Registers a `malicious_executor` alongside the correct one
- Passes `{"tool_name": "malicious_executor"}` in caller params
- Verifies `malicious_executor` is NEVER called
- Verifies correct executor IS called
- This test would FAIL if `params["tool_name"]` were used for executor selection

**TestAT16_ExecutorUsesCallerParamsFails**
- Registers a `recordingExecutor` that captures exact params
- Passes conflicting caller params
- Verifies executor receives snapshot values, not caller values
- This test would FAIL if executor read from caller params

**TestExec35_ProviderSuccessDoesNotCreateAuthority**
- Captures pre-execution state of target, activation, belief
- Executes with provider accepting
- Re-queries and asserts byte-for-byte equality
- Verifies only intent state changes to `executed`
- This test would FAIL if provider acceptance triggered authority mutation

### Weak Tests

**TestExec15B_RevocationAfterCheck_DocumentedRace**
- Claims to characterize Window B but performs no revocation between T2 and T3
- Runs a normal `ExecuteAction` and logs the result
- Always passes regardless of implementation
- Provides no adversarial value

**TestExec34_AuditWriteFailureAfterProviderSuccess**
- Placeholder that logs two messages
- Does not simulate audit failure
- Does not assert anything about execution results or audit gaps

**TestExec30_CompleteIntentNotCallableFromPublicPath**
- Verifies `CompleteIntent` exists on `Contract` interface and is callable
- Does not verify no REST/MCP handler calls it
- Relies on "code review" for the public-path verification

---

## 8. REAL HTTP PROVIDER REVIEW

### File: `adapter/github/http_provider.go`

**Credential handling:**
- Token stored in `HTTPProvider.token` (unexported)
- Token passed via `Authorization: Bearer` header
- Token never logged
- Token injected at construction, not from params
- **PASS**

**Request construction:**
- URL: `https://api.github.com/repos/{owner}/{repo}/actions/workflows/{workflow}/dispatches`
- `strings.TrimPrefix(repo, "/")` prevents path traversal
- Request body: `{"ref": ref, "inputs": inputs}`
- Headers: `Accept`, `Authorization`, `X-GitHub-Api-Version`, `Content-Type`
- **PASS**

**Response handling:**
- 204 No Content → success with empty RunID
- 201 Created → parses `workflow_run.id` from response body
- Other status codes → error
- **PASS**

**Issues:**

1. **No explicit HTTP timeout** (LOW)
   - `httpClient: &http.Client{}` has zero timeout
   - Context cancellation is respected via `http.NewRequestWithContext`
   - But a network stall without context cancellation would block indefinitely
   - **Recommendation:** Add `Timeout: 30 * time.Second` or similar

2. **No redirect policy** (INFO)
   - Default Go behavior follows redirects
   - For a GitHub API client, following redirects is acceptable
   - No security issue

3. **Error message includes response body** (INFO)
   - Line 103: `fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))`
   - Response body may contain sensitive information from GitHub
   - This error is returned to the executor and logged in audit
   - **Recommendation:** Consider truncating or sanitizing response body in error messages

---

## 9. TOCTOU / CONCURRENCY ANALYSIS

### Window A (revocation before final check)
**Status: MITIGATED**

- `PrepareForAction` calls `kernel.Authorize` which re-reads current state
- If revocation occurred before T2, the re-read detects it
- Verified by TestExec08, TestExec15A, TestCR_A

### Window B (revocation after final check, before provider call)
**Status: DOCUMENTED / UNTESTED**

- Code correctly implements local function-call boundary
- No atomic DB+provider coordination claimed
- BUT: TestExec15B does not actually test the race
- The test runs normal execution without any revocation

### Concurrent duplicate execution
**Status: DOCUMENTED LIMITATION**

- Intent state check (`state = 'live'`) prevents sequential duplicates
- Concurrent calls can both pass check before either transitions to `executed`
- TestExec26 documents this as known v1 limitation
- Race detector passes, indicating no data races in test code

### Race detector results
```
$ go test -race ./...
All packages PASS
```

No data races detected in any package.

---

## 10. ARCHITECTURAL BOUNDARY REVIEW

| Boundary | Status | Evidence |
|----------|--------|----------|
| Kernel authority oracle | **PRESERVED** | `kernel.Authorize` is sole authority determination |
| No GitHub semantics in kernel | **PRESERVED** | Kernel stores `consequence_parameters` as opaque JSONB |
| Executor in extension plane | **PRESERVED** | Executor code in `adapter/github/`, registered in `cmd/solvent-mcp/main.go` |
| CompleteIntent as sole new kernel primitive | **PRESERVED** | Only new kernel method; call provenance strictly limited |
| No second authorization engine | **PRESERVED** | Service delegates to kernel; `actionExecutorMap` is routing |
| Authorize != Execute | **PRESERVED** | Separate operations, separate audit entries |
| TOKEN != AUTHORITY | **PRESERVED** | Provider output cannot create authority |

---

## 11. FINDINGS TABLE

| ID | Severity | Area | Finding |
|----|----------|------|---------|
| N-1 | MEDIUM | Tests | TestExec34 is a placeholder — does not simulate audit write failure |
| N-2 | MEDIUM | Tests | TestExec15B does not actually characterize Window B — runs normal execution without revocation race |
| N-3 | LOW | Provider | HTTP provider has no explicit timeout on `http.Client` |
| N-4 | LOW | Tests | TestExec30 does not verify REST/MCP cannot call `CompleteIntent` |

---

## 12. FINAL ANSWER

**"Can OpenCode now implement Phase 4C+ from this test target without inventing missing security semantics?"**

**MOSTLY YES, but two MEDIUM gaps remain.**

The implementation is substantially complete and the kernel/service layer correctly implements the approved architectural contract. All nine previous NO-GO findings are closed. The security invariants are preserved:

- `CompleteIntent` is correctly placed in the `Contract` interface
- Executor selection is derived from authorized action via fixed mapping
- Execution parameters are reconstructed from approved snapshot
- Exact `intentID` is preserved through `CompleteIntent`
- Provider acceptance vs. completion is correctly distinguished
- Audit semantics correctly distinguish authorization, invocation, provider result, and intent completion failure

However, the acceptance test suite has two MEDIUM-severity gaps:

1. **TestExec34** is a placeholder that does not test the audit-write-failure scenario it claims to test
2. **TestExec15B** claims to characterize Window B but runs a normal execution without any revocation race

These gaps mean the test suite does not fully satisfy the approved acceptance contract. OpenCode would need to either:
- Implement the missing test logic, or
- Explicitly document these as Phase 4D items removed from the gating criteria

**Recommendation:** Return NO-GO and require remediation of N-1 and N-2 before Phase 4C+ can be declared COMPLETE.

---

**VERDICT: NO-GO**

Phase 4C+ implementation is architecturally sound but the acceptance test suite has two MEDIUM-severity gaps (N-1, N-2) that prevent it from genuinely satisfying the approved test target.
