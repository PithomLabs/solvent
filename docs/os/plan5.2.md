# Plan 5.2: Final Security Remediation + Adversarial Gate

**Status:** LOCKED — awaiting implementation instruction.

**Trigger:** Previous security-hardening remediation wired intent creation through
authority verification. Execution path (`ExecuteAction`) defined but has zero
production callers. Executor registry empty. No real consequential external
execution capability exists in the codebase. "deploy" is an action string, not
a deployment system.

**Scope:** Prove the security boundary using a test-only recording executor.
Do not invent fake production capabilities.

**Amendments incorporated:** Three corrections from adversarial review:
(1) workflow token participation explicitly resolved, (2) workflow removed from
execution path, (3) test suite classified into three categories.

---

## 1. Current State (Verified)

| Component | Status |
|-----------|--------|
| `kernel.Authorize` | Healthy, read-only, field-by-field match against snapshot |
| `kernel.Approve` | Healthy, sole authority-creating operation |
| `kernel.RevokeTarget` | Healthy, append-only |
| `service/authority` | On production path. `PrepareForAction` called by MCP + Wizard at intent creation. `ExecuteAction` defined but zero callers. |
| `service/policy` | Injected into authority service but never called. Read-only constraint layer. |
| `service/audit` | Wired. Authorization + execution lifecycle events defined. |
| `service/executor` | Registry created empty. No `Register()` calls in production code. |
| `service/workflow` | Exists as workflow token lifecycle. **Not part of the execution path.** Continuity only. |
| MCP intent creation | Wired through `PrepareForAction` → `kernel.IntentOnPromoted` |
| MCP execution | No `ExecuteAction` caller exists |
| Wizard intent creation | Wired through `PrepareForAction` → `kernel.IntentOnPromoted` |
| Wizard execution | No `ExecuteAction` caller exists |
| Pipeline | Creates beliefs/intents, does not authorize or execute |

## 2. Workflow Token Resolution

**Decision: Workflow tokens are continuity-only, not part of the authority boundary.**

```
workflow token
  ↓
workflow continuity (state machine progression)

ExecuteAction
  ↓
current authoritative state
  ↓
kernel.Authorize
  ↓
executor
```

Token validity MUST NOT be required for authority unless the actual workflow
contract requires it. In the current architecture, authority is determined
solely by `kernel.Authorize` against the current snapshot. A workflow token
tracks execution lifecycle; it does not establish or deny authority.

This means:
- An expired workflow token does not cause `ExecuteAction` to deny.
- A stale workflow token does not cause `ExecuteAction` to deny.
- Token lifecycle is managed separately by `WorkflowService`.
- `ExecuteAction` does not receive or resolve a workflow token.
- Tests for expired/stale tokens are workflow-state tests, not authority tests.

If a future product requirement needs tokens at execution time, `ExecuteAction`
MUST receive or resolve the token explicitly and validate it before execution.
That is a future architectural change, not part of this plan.

## 3. WorkflowService Wiring

**WorkflowService is not part of the canonical execution path.**

The authority service does not invoke `WorkflowService`. `ExecuteAction` does
not transition workflow state. The workflow token lifecycle exists as a
standalone state machine for future use.

This is the correct state for the current product:
- No real executor exists.
- No workflow-driven execution exists.
- The workflow token schema is present for future use.
- Leaving `service/workflow` as a standalone module (not wired into execution)
  is honest rather than deceptive.

Do not allow another "wired-looking but semantically disconnected" boundary.
That was the original failure mode.

After implementation, this must be mechanically discoverable:

```
MCP / Wizard / future execution entry
        ↓
ExecuteAction
        ↓
PrepareForAction
        ↓
kernel.Authorize
        ↓
Executor (test-only recording executor for now)
```

`workflow` does not appear in this path because it does not contribute to it.

## 4. The Correct Interpretation

```
REAL CURRENT PRODUCT

request
  ↓
intent creation
  ↓
[no real external execution exists yet]
```

For security testing only:

```
integration test
  ↓
ExecuteAction
  ↓
PrepareForAction
  ↓
kernel.Authorize
  ↓
Recording/Test Executor
```

The test executor does nothing except record:

```
called
action
target
actor
parameters
```

This proves the security boundary without pretending Solvent currently has a
real deployment backend.

## 5. Architectural Rules (Preserved)

- `ExecuteAction` remains the canonical future production execution path.
- No caller-supplied `executorFn`. Executor resolved from internal registry.
- `kernel.Authorize` remains the final authority oracle.
- Policy is a constraint layer, not a second authority engine.
- Workflow tokens are continuity-only, not authority.
- Intent creation ≠ execution. Earlier authorization is never reused.
- Authentication fails closed. Request-body values never trusted as auth.
- Audit distinguishes authorization from execution.

## 6. Work Item 1: Test-Only Recording Executor + Integration Tests

### 6.1 Recording Executor

Create `service/executor/recording_test.go` with a spy/recording executor
that captures:

- `Called bool`
- `Action string`
- `TargetID string`
- `ActorID string`
- `Params map[string]interface{}`
- `Output string`
- `Err error`

The executor is test-only. It is never registered in production code.

### 6.2 Integration Test Setup

Create `service/authority/authority_integration_test.go` with:

- Database-connected test helper using real CockroachDB
- Schema migration application (001–007)
- Test data seeding: principal, target, activation, snapshot, promoted belief
- Recording executor registered in the authority service

### 6.3 Test Matrix — Execution Authorization Tests

These tests exercise the `ExecuteAction` path and verify that unauthorized
actions cannot reach the execution boundary.

| # | Scenario | Expected | Executor Called? |
|---|----------|----------|-----------------|
| 1 | No authority target exists | DENIED | No |
| 2 | Promoted belief + no authority | DENIED | No |
| 3 | Valid authority + wrong target | DENIED | No |
| 4 | Valid authority + wrong action | DENIED | No |
| 5 | Revoked authority | DENIED | No |
| 6 | Policy ALLOW + authority ABSENT | DENIED | No |
| 7 | Policy ALLOW + authority REVOKED | DENIED | No |
| 8 | Target mutates between prepare and execute | DENIED | No |
| 9 | Action mutates between prepare and execute | DENIED | No |
| 10 | Fake approval (no pin/hash) | DENIED | No |

### 6.4 Test Matrix — Capability Boundary Tests

These tests prove that non-authority components cannot manufacture authority.

| # | Scenario | Expected | Notes |
|---|----------|----------|-------|
| 11 | Executor cannot create authority | DENIED | Executor is downstream, not upstream |
| 12 | Provider output cannot create authority | DENIED | External output is evidence, not auth |
| 13 | Workflow state cannot create authority | DENIED | Token lifecycle ≠ authority |
| 14 | Agent cannot approve itself | DENIED | Approval requires separate principal |
| 15 | Actor spoofing fails | DENIED | Request-body actor not trusted as auth |

### 6.5 Test Matrix — Adapter / Integration Tests

These tests verify edge cases at the boundary.

| # | Scenario | Expected | Notes |
|---|----------|----------|-------|
| 16 | Malformed provider output | DENIED / rejected | Input validation |
| 17 | Provider claims cannot manufacture authority | DENIED | Evidence ≠ authority |

### 6.6 Critical Regression Tests (3)

These tests prove the TOCTOU boundary: a previous valid authorization does
not protect against stale state.

| Test | Flow | Assert |
|------|------|--------|
| A | Valid auth → prepare → auth revoked → ExecuteAction → DENIED | executor.called == false |
| B | Valid auth → prepare → target changed → ExecuteAction → DENIED | executor.called == false |
| C | Valid auth → prepare → action changed → ExecuteAction → DENIED | executor.called == false |

### 6.7 Positive Test (1)

| Test | Flow | Assert |
|------|------|--------|
| P | Valid auth + correct target + correct action → ExecuteAction → ALLOWED | executor.called == true, executor.output captured |

## 7. Work Item 2: Adversarial Code Review

After Work Item 1 is complete and tests pass, perform a fresh adversarial
review of the entire repository.

### 7.1 Review Scope

- kernel
- authority-core schema
- service layer
- policy
- workflow
- workflow_token
- MCP
- wizard
- pipeline
- adapter
- executor
- audit
- integration tests
- production entry points

### 7.2 Attack Vectors (18)

1. Any path around `ExecuteAction`
2. Any path around `kernel.Authorize`
3. Any way policy can create authority
4. Any way workflow state can create authority
5. Any way tokens can substitute for authority
6. Any stale-state execution
7. Any wrong-target execution
8. Any wrong-action execution
9. Any actor spoofing
10. Any client-controlled authentication
11. Any provider-created authority
12. Any executor-created authority
13. Any hidden direct provider execution path
14. Any audit path that falsely conflates authorization with execution
15. Any second authority engine
16. Any new kernel/schema expansion not justified by the plan
17. Any dead security boundary
18. Any test that passes without exercising the real security boundary

## 8. Work Item 3: Security Gate Report

Generate the final security gate report with sections A–R:

A. Current execution-path diagram
B. Previously bypassing paths and whether each is fixed
C. List of ALL consequential-action entry points
D. List of ALL executor invocation sites
E. List of ALL external consequential provider calls
F. Exact location of current kernel.Authorize call
G. Exact authority tuple fields checked
H. Exact revocation check
I. Exact actor/authentication boundary
J. Exact policy constraint boundary
K. Exact workflow-token role (continuity only, not authority)
L. Authority-related DB reads immediately before execution
M. Audit event sequence
N. Integration-test matrix with ACTUAL results
O. All files changed
P. All migrations changed/added
Q. Kernel changes (or: "No kernel changes")
R. Remaining risks

## 9. Sequencing

```
Work Item 1 (Recording Executor + Integration Tests)
    ↓
Work Item 2 (Adversarial Code Review)
    ↓
Work Item 3 (Security Gate Report)
    ↓
GO / HOLD
    ↓
only then Phase 4
```

## 10. Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `service/executor/recording_test.go` | Create | Test-only recording executor |
| `service/authority/authority_integration_test.go` | Create | 17+ database-connected integration tests |
| `docs/os/security_gate_report_v2.md` | Create | Final adversarial review output |
| `docs/os/plan4.2.md` | Update | Status to GO (if gate passes) |

## 11. Key Invariants to Prove

1. No successful CURRENT `kernel.Authorize` for the EXACT actor/action/target tuple → no executor invocation
2. Current authority is re-read immediately before execution
3. Exact target/action binding is verified
4. Revocation is checked
5. Policy cannot manufacture authority
6. Workflow tokens cannot manufacture authority
7. Actor spoofing fails
8. Authentication is trusted or human-only actions fail closed
9. Provider output cannot create authority
10. Executor cannot create authority
11. Stale authority is rejected
12. Wrong target/action is rejected
13. Denied paths prove executor was not called
14. Database-connected integration tests pass
15. Audit distinguishes authorization from execution
16. No second authority engine exists
17. No production bypass exists
18. Service boundaries are actually on the critical path

## 12. What This Plan Does NOT Do

- Does not create fake production executors
- Does not invent deployment systems
- Does not add HMAC/JWT authentication
- Does not create identity providers
- Does not introduce execution-effect verification machinery
- Does not claim atomic external transaction coordination
- Does not wire workflow into execution path (workflow is continuity-only)

## 13. TOCTOU Boundary (Documented)

Do NOT claim that Solvent can atomically coordinate its database
authorization transaction with an external provider side effect.

The guaranteed property for this MVP:

    successful CURRENT kernel authorization
    for the EXACT actor/action/target tuple
    immediately precedes executor invocation

There may be an unavoidable distributed race between authorization
and an external provider side effect.

Do not simulate an atomic external transaction.
Do not introduce execution-effect verification machinery.

Document this as a known limitation.

## 14. Final Security Gate

After implementation and adversarial review, end with exactly one:

**GO**

only if ALL of the following are true:

- every consequential production execution path uses ExecuteAction
- no executor is reachable without successful CURRENT kernel.Authorize for the EXACT tuple
- current authority is re-read immediately before execution
- exact target/action binding is verified
- revocation is checked
- policy cannot manufacture authority
- workflow tokens cannot manufacture authority
- actor spoofing fails
- authentication is trusted or human-only actions fail closed
- provider output cannot create authority
- executor cannot create authority
- stale authority is rejected
- wrong target/action is rejected
- target/action mutation tests pass
- denied paths prove executor was not called
- database-connected integration tests pass
- audit distinguishes authorization from execution
- no second authority engine exists
- no production bypass exists
- service boundaries are actually on the critical path
- no unexplained kernel/schema growth exists
- workflow is not part of the execution path (continuity-only)

Otherwise:

**HOLD**

Do not proceed to Phase 4 or Phase 6.

Security correctness takes precedence over schedule.
