# Phase 4B Adversarial Security Review

## 1. VERDICT

**GO WITH CONDITIONS**

The Phase 4B production implementation preserves the Solvent authority architecture. No critical or high-severity production vulnerabilities were found. The kernel is the sole authority oracle, `AuthorizeAndCreateIntent` is atomic, REST and MCP converge on the same path, and the Phase 6.1 identity binding is correctly implemented. Two conditions must be satisfied before freeze: a missing required test must be added, and a nil-pointer defense-in-depth gap in four API handlers must be closed.

---

## 2. EXECUTIVE ASSESSMENT

The Phase 4B security model holds in production code. The canonical architecture — `Client/Protocol → Service Layer → Small Trusted Kernel → CockroachDB` — is realized exactly as designed. There is exactly one authority implementation (`authorizeWithinTx`), one authority-creating operation (`kernel.Approve`), one revocation path (`kernel.RevokeTarget`), and one atomic authorize+intent primitive (`kernel.AuthorizeAndCreateIntent`). The service layer (`service/ledger`) delegates to the kernel; it does not reimplement authority semantics, does not bypass the kernel, and does not become a second authority engine. Both REST and MCP handlers route through the same `service/ledger.AuthorizeAndCreateIntent` method, so there is no MCP-specific bypass.

The Phase 6.1 identity fix correctly eliminates the `actor_id` override vulnerability. `api/auth.go` now maps API keys to real `PrincipalID` values via `keyToPrincipal`, and both `handleVerifyAuthorization` and `handleAuthorizeAction` derive `AuthorityTuple.PrincipalID` from `AuthenticatedPrincipal.PrincipalID`, not from request body fields. A conflicting `req.ActorID` is rejected with HTTP 403 `actor_id_mismatch`.

The atomicity guarantee in `kernel.AuthorizeAndCreateIntent` is sound. The method runs `authorizeWithinTx` (read-only) and `createIntentWithinTx` (intent insert) inside a single `crdb.ExecuteTx` closure. Under CockroachDB SERIALIZABLE isolation, a concurrent `target_revocation` insert creates a read/write conflict on the shared transaction's read set, forcing a 40001 retry. On retry, `authorizeWithinTx` re-reads current state and either denies or allows fresh. The closure is idempotent: `authorizeWithinTx` is pure-read, and `createIntentWithinTx` is an INSERT that either commits or rolls back atomically with the authority check. No partial commit is possible.

However, the verification suite has gaps. The Phase 6.1 plan explicitly required a `TestIntegration_AuthorizeAction_ActorIDMismatch` test; it was never added. The existing `TestIntegration_ConcurrentRevokeTarget` does not exercise the actual race: it creates an unactivated target, so the authority check denies before intent creation regardless of concurrency. Four API handlers have a nil-pointer defense-in-depth gap. The `service/ledger` package has no tests. These are conditions that must be closed before the implementation is considered fully verified.

---

## 3. CRITICAL / HIGH FINDINGS

### Finding H-1: Missing Required Actor-ID Mismatch Test

**ID:** H-1
**Severity:** HIGH
**Classification:** INTRODUCED BY PHASE 6.1 TEST-INFRASTRUCTURE ONLY
**File + line:** `api/integration_test.go` (entire file — no `TestIntegration_AuthorizeAction_ActorIDMismatch`)
**Vulnerability:** The Phase 6.1 plan (`docs/OS/plan6.1.md`, Step 10) explicitly required adding `TestIntegration_AuthorizeAction_ActorIDMismatch` to prove that a request body `actor_id` conflicting with the authenticated principal is rejected with HTTP 403. This test does not exist. The production code at `api/authorization.go:104-109` correctly implements the rejection, but without the test, the fix is unverified by automated regression. A future change could silently remove or weaken the rejection without the test suite catching it.
**Concrete exploit/concurrency scenario:** Not applicable — the production code is correct. The gap is verification-only.
**Why existing controls do not prevent it:** The test was specified in the approved plan but was never implemented.
**Required remediation:** Add `TestIntegration_AuthorizeAction_ActorIDMismatch` to `api/integration_test.go`. The test must send a request with `actor_id` set to a UUID that differs from the authenticated test principal (`testPrincipalID`) and assert HTTP 403 with `actor_id_mismatch`.
**Whether remediation requires kernel/schema/API redesign:** No. Test-only fix.

---

## 4. MEDIUM / LOW / INFO FINDINGS

### Finding M-1: Nil Pointer Dereference Risk in Four API Handlers

**ID:** M-1
**Severity:** MEDIUM
**Classification:** INTRODUCED BY PHASE 4B
**File + line:** `api/target.go:117`, `api/target.go:141`, `api/target.go:175`, `api/target.go:205`
**Vulnerability:** Four handlers dereference `AuthFromContext(r.Context())` without a nil check:
- `handleAttachJustification` line 117: `AuthFromContext(r.Context()).PrincipalID`
- `handleRequestAuthorization` line 141: `principal.PrincipalID` (after assignment, but nil check missing)
- `handleApproveTarget` line 175: `principal.PrincipalID`
- `handleRevokeTarget` line 205: `principal.PrincipalID`

In normal operation, `AuthMiddleware` sets the principal for every authenticated request and returns 401 without calling `next` for unauthenticated requests. However, if the middleware is bypassed, misconfigured, or if a future refactor introduces a code path that calls these handlers without going through the middleware, a nil `*AuthenticatedPrincipal` will cause a panic. The `handleVerifyAuthorization` and `handleAuthorizeAction` handlers correctly perform nil checks; these four do not.
**Concrete exploit/concurrency scenario:** An attacker who discovers a way to bypass `AuthMiddleware` (e.g., through a future route registration error) can trigger a panic rather than a clean 401/403 response, causing a denial of service.
**Why existing controls do not prevent it:** The auth middleware provides structural protection, but defense-in-depth nil checks are absent.
**Required remediation:** Add nil checks matching the pattern in `handleVerifyAuthorization`:
```go
principal := AuthFromContext(r.Context())
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal", "No authenticated principal", nil)
    return
}
```
**Whether remediation requires kernel/schema/API redesign:** No. API handler fix only.

---

### Finding M-2: `service/ledger` Has Zero Test Coverage

**ID:** M-2
**Severity:** MEDIUM
**Classification:** INTRODUCED BY PHASE 4B
**File + line:** `service/ledger/ledger.go` (no corresponding `*_test.go` file exists)
**Vulnerability:** The `service/ledger` package is the architectural boundary between API handlers and the kernel. It contains `AuthorizeAndCreateIntent`, the most security-sensitive method in the codebase. The package has no tests. If a future change introduces a bug in the service layer — for example, by duplicating authority logic, bypassing the kernel, or constructing the tuple incorrectly — the test suite will not catch it.
**Concrete exploit/concurrency scenario:** Not applicable — no vulnerability exists today, but the absence of tests means the service layer's correctness is unverified.
**Why existing controls do not prevent it:** The kernel has comprehensive tests (T-01 through T-C8), and the API integration tests exercise the full stack. But the service layer's delegation logic is itself untested.
**Required remediation:** Add `service/ledger/ledger_test.go` with tests for `AuthorizeAndCreateIntent` that verify: (1) allowed authority produces a live intent, (2) denied authority produces no intent, (3) the service delegates to the kernel's atomic primitive (no SQL duplication).
**Whether remediation requires kernel/schema/API redesign:** No. Test-only addition.

---

### Finding M-3: `TestIntegration_ConcurrentRevokeTarget` Does Not Test the Actual Race

**ID:** M-3
**Severity:** MEDIUM
**Classification:** INTRODUCED BY PHASE 4B
**File + line:** `api/integration_test.go:59-137`
**Vulnerability:** The test creates an unactivated target (`createTestTarget` does not call `Approve`). An unactivated target always fails authority evaluation, so no intent is ever created regardless of concurrency. The test fires concurrent `authorize-action` and `revoke-target` requests, but the authority check denies before intent creation, meaning the test does not exercise the `AuthorizeAndCreateIntent` race window at all. The test verifies only that the server does not crash under concurrent requests — not that the atomic primitive prevents stale-authority intent creation.
**Concrete exploit/concurrency scenario:** If `AuthorizeAndCreateIntent` were replaced with the old two-step `PrepareForAction` + `IntentOnPromoted`, this test would still pass, because the target is unactivated and the authority check fails in the first step.
**Why existing controls do not prevent it:** The kernel's T-C1 through T-C5 concurrency tests cover the actual race at the kernel level. But the API integration test that claims to cover the end-to-end race does not.
**Required remediation:** Modify the test to: (1) create and activate a target (`Approve`), (2) fire concurrent `authorize-action` and `revoke-target`, (3) verify that no live intent exists on the target after the race resolves. This requires querying `action_intent` post-race.
**Whether remediation requires kernel/schema/API redesign:** No. Test modification only.

---

### Finding M-4: Wizard Still Uses the Old Two-Step Authorization Pattern (PRE-EXISTING)

**ID:** M-4
**Severity:** MEDIUM
**Classification:** PRE-EXISTING
**File + line:** `internal/wizard/refusal.go:133-149`
**Vulnerability:** The wizard's `Authorize` method calls `s.authSvc.PrepareForAction(...)` and then `s.kern.IntentOnPromoted(...)` in separate transactions. This is the exact check-then-write race that Phase 4B fixed in REST and MCP. Between the two calls, a concurrent `RevokeTarget` can commit, creating a live intent on superseded authority. The wizard is legacy/demo code and was explicitly excluded from Phase 4B scope per the plan ("Do not delete working demo functionality"), but the vulnerability remains reachable through the wizard's `/api/state` HTTP endpoint.
**Concrete exploit/concurrency scenario:** An operator uses the wizard to authorize an action. Concurrently, an administrator revokes the target. The wizard's `PrepareForAction` reads the active target, then `RevokeTarget` commits, then `IntentOnPromoted` inserts a live intent on the now-revoked target.
**Why existing controls do not prevent it:** The wizard does not use `kernel.AuthorizeAndCreateIntent`. The old two-step path has no atomicity guarantee.
**Required remediation:** Migrate the wizard's `Authorize` method to use `kernel.AuthorizeAndCreateIntent` (or `service/ledger.AuthorizeAndCreateIntent`). This is explicitly deferred by the Phase 4B plan but should be tracked as a follow-up.
**Whether remediation requires kernel/schema/API design:** No. The kernel primitive already exists; the wizard must adopt it.

---

### Finding L-1: `cmd/operator-review` Calls `IntentOnPromoted` Directly (PRE-EXISTING)

**ID:** L-1
**Severity:** LOW
**Classification:** PRE-EXISTING
**File + line:** `cmd/operator-review/main.go:178`
**Vulnerability:** The operator-review CLI calls `st.IntentOnPromoted(ctx, scenarioID, beliefID, action)` directly, without `PrepareForAction` or `kernel.Authorize`. This is documented as intentional ("operates under the operator's direct authority"), but it means the CLI can create live intents on beliefs without verifying current authority.
**Concrete exploit/concurrency scenario:** Not applicable — this is a trusted administrative CLI tool, not a network-facing service. The trust boundary is the operator's shell.
**Why existing controls do not prevent it:** The tool is intentionally unconstrained.
**Required remediation:** None. Documented trusted-tool pattern.
**Whether remediation requires kernel/schema/API redesign:** No.

---

### Finding L-2: `handleCreateTarget` Accepts Caller-Supplied `principal_id` Without Validation

**ID:** L-2
**Severity:** LOW
**Classification:** INTRODUCED BY PHASE 4B
**File + line:** `api/target.go:39`
**Vulnerability:** `handleCreateTarget` passes `req.PrincipalID` directly to `s.ledger.CreateTarget` without validating that the authenticated caller is authorized to create targets for that principal. The Phase 4A contract classifies `principal_id` as a caller assertion, not proof, and v0 does not enforce caller-to-principal authorization for target creation. This is by design but means any authenticated caller can create targets for any principal.
**Concrete exploit/concurrency scenario:** An authenticated caller with principal A can create an authority target for principal B. This does not grant authority (only `Approve` does), but it pollutes the target namespace.
**Why existing controls do not prevent it:** v0 has no RBAC on target creation. The Phase 4A contract explicitly defers this.
**Required remediation:** None for v0. Future work should add API-level access control.
**Whether remediation requires kernel/schema/API redesign:** Future API-layer access control, not kernel.

---

## 5. VERIFIED INVARIANTS

The following invariants were proven from code and tests:

- **kernel is sole authority engine** — `authorizeWithinTx` is the single implementation of authority semantics, used by both `Authorize` and `AuthorizeAndCreateIntent`. No service/API/MCP path independently evaluates authority. (`kernel/authority.go:366-430`, `service/ledger/ledger.go:129-141`)
- **Authorize is read-only** — `authorizeWithinTx` performs zero writes. `TestT28_AuthorizePure` confirms row counts in `target_snapshot`, `target_activation`, and `target_revocation` are unchanged after `Authorize`. (`kernel/authority.go:432-448`, `kernel/authority_test.go:906-944`)
- **Approve creates authority** — `kernel.Approve` atomically inserts `target_snapshot` and `target_activation` in one `crdb.ExecuteTx`. `TestT09_ValidApprove` confirms state=active with 1 activation. (`kernel/authority.go:249-357`, `kernel/authority_test.go:332-357`)
- **RevokeTarget revokes authority append-only** — `kernel.RevokeTarget` inserts into `target_revocation`. `TestT13_Revoke` confirms state=revoked with 1 revocation row. Second approval fails with `ErrAlreadyActivated` (UNIQUE(target_id)). (`kernel/authority.go:500-526`, `kernel/authority_test.go:464-490, 434-461`)
- **AuthorizeAndCreateIntent is atomic** — Both `authorizeWithinTx` and `createIntentWithinTx` execute inside a single `crdb.ExecuteTx` closure. The existing `Authorize` and `IntentOnPromoted` methods delegate to these helpers, preserving backward compatibility. (`kernel/authority.go:466-496`, `kernel/kernel.go:109-132`)
- **stale revocation cannot produce a committed live intent** — `sqlAuthorizeResolve` reads `target_activation`, `target_snapshot`, and `NOT EXISTS (SELECT 1 FROM target_revocation)`. Under SERIALIZABLE, a concurrent `target_revocation` insert conflicts with the in-flight transaction's read set, forcing 40001 retry. On retry, `authorizeWithinTx` re-reads and denies. `TestTC1_ApproveXApprove` and `TestTC5_RevokeXApprove` prove single-activation semantics. (`kernel/sql.go:172-181`, `kernel/authority_test.go:988-1187`)
- **authenticated principal controls API authority identity** — `AuthMiddleware` maps Bearer tokens to `PrincipalID` via `keyToPrincipal`. Both `handleVerifyAuthorization` and `handleAuthorizeAction` use `principal.PrincipalID`, ignoring `req.PrincipalID` from the request body. (`api/auth.go:26-55`, `api/authorization.go:23-39, 96-120`)
- **actor_id cannot override authenticated identity** — `handleAuthorizeAction` rejects `req.ActorID` that conflicts with `effectiveActor` with HTTP 403 `actor_id_mismatch`. (`api/authorization.go:104-109`)
- **REST and MCP converge on the same authority path** — REST: `handleAuthorizeAction` → `service/ledger.AuthorizeAndCreateIntent` → `kernel.AuthorizeAndCreateIntent`. MCP: `handleSolventAuthorizeAction` → `ledgerSvc.AuthorizeAndCreateIntent` → `kernel.AuthorizeAndCreateIntent`. Both paths are identical after the service boundary. (`api/authorization.go:122`, `cmd/solvent-mcp/tools.go:258`)
- **no workflow token is authority** — Zero `workflow_token` references in Go code. The table exists but is dead.
- **no production executor bypass exists** — `service/executor/executor.go` exists but the registry is empty. No API or MCP endpoint invokes an executor. `cmd/solvent-mcp/main.go:126` explicitly documents "Executor registry is instantiated but empty."
- **no hidden consequential execution path exists** — No `/execute` endpoint, no `ExecuteAction` invocation in REST or MCP handlers, no caller-supplied executor selection.
- **no second authority engine** — `authorizeWithinTx` is the single implementation. The service layer delegates; the API handlers delegate. No duplicate SQL, no alternate tuple comparison, no cached decisions.

---

## 6. FAILED / UNPROVEN INVARIANTS

- **tests exercise the intended security paths** — PARTIALLY FAILED.
  - The actor-id mismatch rejection is implemented but has no test (`TestIntegration_AuthorizeAction_ActorIDMismatch` is missing).
  - `TestIntegration_ConcurrentRevokeTarget` claims to test the authorize+revoke race but uses an unactivated target, so it never exercises the race window.
  - `service/ledger` has zero tests, so the service-layer delegation is unverified.

---

## 7. TEST EVIDENCE

**Fresh runs (not cached):**

```
go build ./...                        # PASS — no output
go vet ./...                          # PASS — no output
go test -count=1 -p 1 ./...           # PASS — all packages green
go test -count=1 ./...                # PASS — all packages green (parallel)
```

**Critical security tests (fresh, -v):**

```
go test -count=1 -v -run TestIntegration_AuthorizeAction_Atomicity ./api/...
# PASS — 0.04s

go test -count=1 -v -run TestIntegration_ConcurrentRevokeTarget ./api/...
# PASS — 0.02s (weak test — see Finding M-3)

go test -count=1 -v -run TestAJ_UnpromotedBelief_AuthorityDenied ./cmd/solvent-mcp/...
# PASS — 0.05s

go test -count=1 -v -run TestAuthorizeAction_ValidArgs ./cmd/solvent-mcp/...
# PASS — 0.08s

go test -count=1 -v -run TestAJ_ToolOutputRefused ./cmd/solvent-mcp/...
# PASS — 0.00s

go test -count=1 -v -run TestAJ_NilDBPanicsOnDBPath ./cmd/solvent-mcp/...
# PASS — 0.00s

go test -count=1 -v -run 'TestT14_AuthorizeExactTuple|TestT20_AuthorizeRevokedTarget|TestTC1_ApproveXApprove' ./kernel/...
# PASS — all three
```

**Test gap confirmed:**
```
grep -rn "TestIntegration_AuthorizeAction_ActorIDMismatch" .
# No results — test does not exist
```

---

## 8. PHASE 4B DISPOSITION

**READY WITH SPECIFIC FIXES**

The Phase 4B production implementation is architecturally sound and preserves the Solvent authority invariant. No redesign is needed. Before freeze, the following specific fixes must be applied:

1. **Add `TestIntegration_AuthorizeAction_ActorIDMismatch`** to `api/integration_test.go` (Finding H-1).
2. **Add nil checks** in `api/target.go` handlers: `handleAttachJustification`, `handleRequestAuthorization`, `handleApproveTarget`, `handleRevokeTarget` (Finding M-1).
3. **Strengthen `TestIntegration_ConcurrentRevokeTarget`** to use an activated target and verify no live intent is created post-race (Finding M-3).
4. **Add `service/ledger/ledger_test.go`** to cover the service layer's delegation to `kernel.AuthorizeAndCreateIntent` (Finding M-2).

Items M-4 (wizard two-step pattern) and L-1 (operator-review direct `IntentOnPromoted`) are pre-existing and explicitly out of Phase 4B scope. They should be tracked as follow-up work but do not block freeze.

The Phase 6.1/6.2 test-infrastructure changes (per-package `SuiteDSN`, `api/suite_test.go`, MCP `ledgerSvc` initialization) are correct and do not weaken Phase 4B security semantics. The per-package database isolation is a legitimate hardening improvement.

---

## APPENDIX: Architecture Call Graph (Consequential Paths)

**REST authorize-action:**
```
POST /v1/authorizations/action
  → api.AuthMiddleware (authenticates, sets AuthenticatedPrincipal)
  → api.handleAuthorizeAction
  → api.validateUUID / validateNonEmpty / action_source gate
  → AuthFromContext → principal.PrincipalID (effectiveActor)
  → actor_id mismatch check (403 if conflicting)
  → service/ledger.AuthorizeAndCreateIntent
  → kernel.AuthorizeAndCreateIntent (ONE crdb.ExecuteTx)
    → authorizeWithinTx (READ target_activation + target_snapshot + NOT EXISTS target_revocation + tuple comparison + justification belief status checks)
    → createIntentWithinTx (INSERT action_intent)
  → audit log
  → JSON response
```

**MCP authorize-action:**
```
solvent_authorize_action tool
  → cmd/solvent-mcp/toolHandler
  → handleSolventAuthorizeAction
  → action_source gate (tool_output → immediate refusal)
  → cross-scenario guard (view.GetSnapshot)
  → ledgerSvc.AuthorizeAndCreateIntent
  → kernel.AuthorizeAndCreateIntent (SAME atomic primitive as REST)
  → envelopeResult with audit
```

**No alternate path** reaches `action_intent` insertion without passing through `kernel.AuthorizeAndCreateIntent` or `kernel.IntentOnPromoted`. The wizard and operator-review tools use the older two-step path, but they are pre-existing trusted-admin/legacy paths, not the canonical API surface.

---

**Review completed.** All production code paths verified. Test suite green. Findings documented above.
