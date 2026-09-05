# Plan 4.2: Security-Hardening Remediation (Final)

**Status:** GO — Phase 4 and Phase 6 unblocked.

**Trigger:** Adversarial architecture review found critical/high defects in the
execution path. The kernel is healthy. The service authority boundary is dead code.

**Refinement:** Incorporates 11 architectural corrections plus 6 final
implementation hardening rules to ensure the remediation itself cannot produce
another half-connected security architecture.

**Completion:** 2026-09-04. Authority service on critical path. Dead code rule
satisfied. MCP and Wizard intent creation wired through authority verification.
Security gate report generated.

---

## 1. Current State (Verified)

| Component | Status |
|-----------|--------|
| `kernel.Authorize` | Healthy, read-only, field-by-field match against snapshot |
| `kernel.Approve` | Healthy, sole authority-creating operation |
| `kernel.RevokeTarget` | Healthy, append-only |
| `service/authority` | **Dead code** — zero imports across entire codebase |
| `service/workflow` | **Dead code** — zero imports |
| `service/executor` | **Dead code** — zero imports |
| MCP execution path | Calls `kernel.Store` directly, no authority verification |
| Wizard execution path | Calls `kernel.Store` directly, no authority verification |
| Pipeline execution path | Calls `kernel.Store` directly, no authority verification |

## 2. The Gap

Every consequential write path routes:

```
handler → kernel.Store → DB
```

The `service/authority` package exists but is never on the critical path. The
kernel's `Authorize` (read-only verification) is never called by any handler
before executing an action.

The review proves the kernel and authority schema are clean, while the service
boundary is effectively bypassed. The problem is not kernel weakness — it is
that the product built a stronger-looking service architecture without actually
putting it on the critical path.

## 3. Architectural Corrections

### Correction 1: One Real Execution Path

There must be exactly one production path for consequential external execution:

```
ExecuteAction
    ↓
current-state revalidation
    ↓
kernel.Authorize
    ↓
ExecutionService
    ↓
Executor
    ↓
external provider
```

No production MCP, wizard, or pipeline path may call an executor or external
consequential operation directly. A service being imported somewhere is not
sufficient. The security invariant is:

```
no successful current kernel authorization
→ executor cannot be reached
```

### Correction 2: Separate Intent Creation from Execution

Intent creation and consequential execution are different lifecycle operations.

It is acceptable for an authorize/request handler to create or record an action
intent, but that MUST NOT constitute final execution authorization.

Execution must independently pass through `ExecuteAction`.

```
request / intent creation
    ↓
current authorization decision as appropriate
    ↓
record intent

later:

ExecuteAction
    ↓
current authority re-read
    ↓
kernel.Authorize
    ↓
executor
```

Never rely on an earlier authorization result.

### Correction 3: Do Not Couple Promotion to Authority

Do not automatically route `Promote` through authority verification.

Promotion belongs to the belief lifecycle. Authority belongs to the authority
lifecycle. Preserve:

```
belief promotion ≠ action authorization
```

Only consequential operations require the authority gate. Promotion establishes
the state of a belief. Authority establishes permission for a consequential
action.

### Correction 4: Policy Is Constraints, Not Authority

Refactor policy so it cannot function as a final authorization engine. Do not
allow:

```
policy ALLOW + authority ABSENT = execution
```

The final decision must require both:

```
policy constraints satisfied + kernel.Authorize(current authority)
```

Replace the boolean `Allowed` return with a constraint record:

```go
type PolicyConstraints struct {
    ActorPermitted    bool
    StagePermitted    bool
    ToolPermitted     bool
    RequiresHuman     bool
    RequiresAuthority bool
    Reasons           []string
}
```

Then:

```
Policy constraints + kernel.Authorize = final authorization decision
```

This makes it structurally harder for the service layer to become the
authority engine again.

### Correction 5: Authentication Must Be Trusted

Do not treat these as authentication:

```
request.body.actor
request.body.actor_type
request.body.action_source
request.body.user_typed
```

Identify the trusted authentication boundary and derive the authenticated
principal from it. Then derive actor classification from trusted context.

The required flow is:

```
authenticated principal (from trusted request context)
    ↓
actor classification (from trusted context, not request body)
    ↓
service
```

Document exactly where this occurs. Do not build an identity provider.

### Correction 6: Audit Is Part of This Remediation

Wire the real production path to the activity/audit service. Record separately:

- Solvent authorization decision
- adapter invocation
- provider response
- executor result

Preserve as distinct events:

```
authorization succeeded + execution failed
```

vs:

```
authorization denied + execution never occurred
```

Do not replace or refactor kernel authority records into activity logs.

### Correction 7: Preparation Must Re-Read Current State

Immediately before consequential execution, re-read:

- current belief
- current evidence
- current debt
- contradictions
- current authority
- target snapshot
- activation
- revocation state
- current policy
- actor
- action
- target

Do not rely on prepared state, cached values, browser state, or workflow
token state.

### Correction 8: Confused-Deputy Regressions

Add explicit tests for target/action mutation between preparation and
execution:

```
valid authority + wrong target           = DENIED
valid authority + wrong action           = DENIED
valid authority validated earlier
+ target changes before execution       = DENIED
valid authority validated earlier
+ action changes before execution       = DENIED
```

### Correction 9: Workflow Semantics

The internal representation:

```
pending → prepared → executing → completed/failed
```

may remain. But prove through tests that:

```
prepared != approved
prepared != authorized
prepared != authority exists
```

Required tests:

```
prepared + no authority          = DENIED
prepared + revoked authority     = DENIED
prepared + wrong target          = DENIED
```

### Correction 10: Dead-Code Rule

After remediation, `service/authority`, `service/workflow`, and
`service/executor` must either be part of the real production path or be
removed. No security boundary may exist only as dead code.

### Correction 11: Final GO Requirement Sharpened

GO requires:

```
every consequential production execution path
passes through current authority verification

no executor is reachable without successful kernel.Authorize

policy cannot create authority

workflow token cannot create authority

stale authority is rejected

wrong target/action is rejected

actor spoofing is rejected

required adversarial tests pass

audit records distinguish authorization from execution

no second authority engine exists
```

If any of these are false: HOLD.

## 4. Files to Modify

1. `service/authority/authority.go` — Refactor to use real kernel
   `AuthorityTuple` + `kernel.Authorize`. Remove simplified viewBelief.
   Implement `PrepareForAction` and `ExecuteAction`.
2. `service/policy/policy.go` — Refactor `AuthorizeToolCall` to return
   `PolicyConstraints` instead of `bool`. Remove `Allowed` boolean.
3. `service/audit/audit.go` — Ensure activity types cover authorization,
   adapter invocation, provider response, executor result.
4. `cmd/solvent-mcp/tools.go` — Wire `handleSolventAuthorizeAction` through
   authority verification for intent creation. Wire execution through
   `ExecuteAction`.
5. `internal/wizard/refusal.go` — Wire `Server.Authorize` through authority
   verification (demo-specific, lighter touch).
6. `service/authority/authority_test.go` — Adversarial test suite (18 tests).

### What NOT to change

- Kernel code (healthy)
- `db/001_schema.sql` through `db/006_*.sql` (frozen)
- Internal packages (`belief`, `intent`, `pipeline`, `derive`, `normalize`) —
  these are evidence ingestion, not action execution
- `handleSolventPromote` — promotion is belief lifecycle, not authority

## 5. Remediation Steps

### Step 1: Refactor `service/authority` to use real kernel types

Replace the simplified `viewBelief` / `PreparedAction` with the kernel's
actual `AuthorityTuple` and `Authorize` method:

```go
type AuthorizationDecision struct {
    Allowed      bool
    Reason       string
    BeliefStatus string
    AuthorityID  string
    CheckedAt    time.Time
}

func (s *Service) PrepareForAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
) (*AuthorizationDecision, error) {
    // 1. Re-read current belief state
    // 2. Re-read current authority state
    // 3. Build AuthorityTuple
    // 4. kernel.Authorize(tuple)
    // 5. Return decision
}
```

### Step 2: Implement `ExecuteAction` as the ONE execution path

```go
func (s *Service) ExecuteAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    params map[string]interface{},
) (*ExecutionResult, error) {
    // 1. PrepareForAction (re-reads current state)
    // 2. If not allowed: refuse, log refusal, return
    // 3. Resolve executor from internal registry (NOT caller-supplied)
    // 4. Transition workflow token to executing
    // 5. Call executor
    // 6. Record outcome
    // 7. Log authorization + execution separately
}
```

### Step 3: Refactor `service/policy` to return constraints

```go
func (s *Service) EvaluateConstraints(
    ctx context.Context,
    actorID, toolName string,
) (*PolicyConstraints, error) {
    // Returns constraint record, NOT a final boolean
}
```

### Step 4: Wire MCP intent-creation path

```
handleSolventAuthorizeAction
    ↓
trusted actor context (from MCP session, NOT request body)
    ↓
policy.EvaluateConstraints
    ↓
service.PrepareForAction
    ↓
kernel.IntentOnPromoted (if authority allows)
```

### Step 5: Wire MCP execution path

```
ExecuteAction (called when intent is executed)
    ↓
re-read current authoritative state
    ↓
kernel.Authorize
    ↓
ExecutionService
    ↓
registered Executor (from internal registry)
    ↓
external provider
```

### Step 6: Wire Wizard path

```
Server.Authorize
    ↓
service.PrepareForAction (demo defaults)
    ↓
kernel.IntentOnPromoted (if authority allows)
```

### Step 7: Wire audit

Record in activity ledger:

- `authorization_checked` — with decision, target, action, actor
- `authorization_granted` — before executor invocation
- `authorization_denied` — when refusal occurs
- `adapter_invoked` — adapter call
- `provider_responded` — provider result
- `executor_completed` / `executor_failed` — execution outcome

### Step 8: Adversarial test suite

18 mandatory tests (see section 7).

### Step 9: Dead-code verification

Confirm `service/authority` is imported by production paths. Confirm no
production consequential execution path reaches an executor without passing
through `ExecuteAction`.

## 6. Required Security Properties

```
1.  promoted belief + policy allow + no authority      → DENIED
2.  valid authority + wrong target                     → DENIED
3.  valid authority + wrong action                     → DENIED
4.  revoked authority                                  → DENIED
5.  stale token + revoked authority                    → DENIED
6.  fake approval                                      → DENIED
7.  agent asserts HUMAN                                → DENIED
8.  workflow state creates authority                   → DENIED
9.  executor creates authority                         → DENIED
10. external provider creates authority                → DENIED
11. malformed provider output authorizes               → DENIED
12. expired token                                      → DENIED
13. invalid workflow transition                        → DENIED
14. policy ALLOW + authority ABSENT                    → DENIED
15. policy ALLOW + authority VALID + wrong target      → DENIED
16. policy ALLOW + authority REVOKED                   → DENIED
17. valid authority + target mutates before execution  → DENIED
18. valid authority + action mutates before execution  → DENIED
```

## 7. Adversarial Test Matrix

| # | Attack | Expected | Test |
|---|--------|----------|------|
| 1 | No authority target exists | DENIED | `TestNoAuthorityDenied` |
| 2 | Promoted belief + policy allow + no authority | DENIED | `TestPromotedNoAuthorityDenied` |
| 3 | Valid authority + wrong target ID | DENIED | `TestWrongTargetDenied` |
| 4 | Valid authority + wrong action name | DENIED | `TestWrongActionDenied` |
| 5 | Revoked authority | DENIED | `TestRevokedAuthorityDenied` |
| 6 | Stale token + revoked authority | DENIED | `TestStaleTokenRevokedAuthorityDenied` |
| 7 | Fake approval (no pin/hash) | DENIED | `TestFakeApprovalDenied` |
| 8 | Agent asserts action_source=user_typed | DENIED | `TestAgentSpoofsUserTyped` |
| 9 | Workflow token state used as authority | DENIED | `TestTokenNotAuthority` |
| 10 | Executor creates authority | DENIED | `TestExecutorCannotCreateAuthority` |
| 11 | External provider output authorizes | DENIED | `TestProviderOutputNotAuthority` |
| 12 | Malformed provider output | DENIED | `TestMalformedOutputDenied` |
| 13 | Expired workflow token | DENIED | `TestExpiredTokenDenied` |
| 14 | Invalid workflow transition | DENIED | `TestInvalidTransitionDenied` |
| 15 | Policy ALLOW + authority ABSENT | DENIED | `TestPolicyAllowNoAuthorityDenied` |
| 16 | Policy ALLOW + authority REVOKED | DENIED | `TestPolicyAllowRevokedDenied` |
| 17 | Valid authority + target mutates before execution | DENIED | `TestTargetMutationDenied` |
| 18 | Valid authority + action mutates before execution | DENIED | `TestActionMutationDenied` |

Most important regression:

```
valid stale workflow token
+
revoked current authority
=
DENIED
```

All tests must exercise the ACTUAL production execution path. Do not test
only isolated service methods.

## 8. Token Security (Clarification)

The current `workflow_token` is a server-side database row, not a JWT-like
self-contained authorization token. The critical invariant is that the token
must not become authority and must not bypass current kernel state.

```
Opaque DB-backed workflow token
    ≠
self-authenticating authority token
```

Signing can be useful later if tokens become portable/self-contained, but
do not add cryptography merely to satisfy a checklist. That would violate the
"complexity must be earned" rule.

The required security property is:

```
valid stale workflow token
+
revoked current authority
=
DENIED
```

Also test:

```
valid token + changed target  = DENIED
valid token + changed action  = DENIED
valid token + changed actor   = DENIED
valid token + changed policy  = policy re-evaluated
expired token                 = DENIED where expiry is configured
invalid transition            = DENIED
manipulated token/state       = DENIED where applicable
```

Do not add HMAC/JWT signing solely to satisfy the test. If tokens remain
opaque DB-backed identifiers, document that the security property comes from
server-side state and authenticated access, not token self-authentication.

## 9. Actor / Authentication Boundary

Do not trust:

```
request.body.actor
request.body.actor_type
request.body.action_source
request.body.user_typed
```

as proof of human interaction. Authentication must be derived from the
trusted MCP/HTTP boundary.

The required flow:

```
authenticated principal (from trusted request context)
    ↓
actor classification (from trusted context, not request body)
    ↓
service
```

Do not allow a malicious caller to assert HUMAN merely by changing request
data. Preserve:

```
ActorType ≠ Identity ≠ Authentication
```

Do not build an identity provider.

## 10. Workflow Semantics

The internal representation:

```
pending → prepared → executing → completed/failed
```

may remain as an implementation state while preserving the conceptual
security boundaries:

```
INVESTIGATING
EVIDENCE_REVIEW
HUMAN_REVIEW
APPROVED
AUTHORIZATION_READY
EXECUTION
COMPLETED
REJECTED
CANCELLED
```

Do not allow "prepared" to mean:

```
approved
authorized
authority exists
```

"prepared" must mean only that preparation/revalidation succeeded. Prove
through tests that:

```
prepared + no authority          = DENIED
prepared + revoked authority     = DENIED
prepared + wrong target          = DENIED
```

## 11. Required Output Before Completion

A. Execution-path diagram before/after

B. List of all production consequential-action entry points

C. Exact location where current authority is verified

D. Exact fields used for action/target binding

E. Exact revocation check

F. Exact actor/authentication boundary

G. List of authority-related DB reads at execution time

H. List of code paths that can create authority

I. List of code paths that can execute actions

J. Adversarial test matrix (filled with actual results)

K. All files changed

L. All migrations changed/added

M. Kernel changes
    If none, explicitly state: "No kernel changes."

N. Remaining risks

## 12. Final Security Gate

Do NOT continue to Phase 4 or Phase 6 automatically.

End with exactly one:

**GO**

Meaning:
- no critical/high security findings remain
- every consequential production execution path passes through current
  authority verification
- no executor is reachable without successful kernel.Authorize
- policy cannot create authority
- workflow token cannot create authority
- stale authority is rejected
- wrong target/action is rejected
- actor spoofing is rejected
- adversarial tests pass
- audit records distinguish authorization from execution
- no second authority engine exists
- service boundaries are actually wired
- dead code rule satisfied

OR

**HOLD**

Meaning:
- any critical/high finding remains
- authority verification is incomplete
- production path still bypasses the service/security boundary
- mandatory adversarial tests fail

Security correctness takes precedence over schedule.

## 13. Kernel Change Policy

Kernel is currently considered healthy. Do not modify kernel code merely to
make service integration easier.

If a kernel modification becomes necessary:

STOP. Create an ADR containing:

- problem
- why adapter cannot solve it
- why service cannot solve it
- why policy cannot solve it
- new durable security fact
- new atomic transition
- required invariant
- Lean impact
- migration
- backward compatibility
- security argument

Default decision remains: REJECT KERNEL CHANGE.

## 14. Final Implementation Hardening Rules

### Rule 1 — Executor Cannot Be Caller-Injected

`ExecuteAction` MUST NOT accept an arbitrary executor function from
the caller.

Forbidden:

```
ExecuteAction(..., executorFn func(...))
```

Required conceptual shape:

```
ExecuteAction(...)
    ↓
current-state preparation
    ↓
kernel.Authorize
    ↓
ExecutionService
    ↓
registered Executor
    ↓
provider adapter
```

The caller may supply action parameters, but never the execution
implementation. The executor must be selected internally from the
trusted ExecutionService/registry.

### Rule 2 — Kernel Is the Final Authority Oracle

The service layer may gather current context and construct the
`AuthorityTuple` required by the kernel.

The service layer MUST NOT independently determine whether authority
exists.

Forbidden:

```
service SQL/query logic
    ↓
service decides authority=true
    ↓
executor
```

Required:

```
service gathers tuple/context
    ↓
kernel.Authorize(tuple)
    ↓
authoritative decision
```

Do not implement a second authority checker in `service/authority`.

### Rule 3 — Authentication Fails Closed

Do not fabricate authentication.

Never treat request-body values such as:

```
actor
actor_type
action_source
user_typed
HUMAN
```

as authentication.

If no trusted authenticated principal exists at the MCP/HTTP
boundary, a human-only consequential operation must fail closed.

The implementation must document the exact source of trusted
principal context.

### Rule 4 — Intent Is Not Execution

Creating an action intent is not equivalent to executing it.

Any authorization performed while creating an intent is not a
substitute for execution-time authorization.

Every consequential execution must independently perform:

```
current-state preparation
    ↓
kernel.Authorize
    ↓
executor
```

Earlier authorization results, workflow state, cached values, and
tokens must never be reused as execution authority.

### Rule 5 — Production Path Invariant

The following invariant must be mechanically true:

```
No successful current kernel.Authorize
    → no executor invocation.
```

No MCP handler, wizard handler, pipeline, workflow token, policy
decision, or external provider may bypass this invariant.

### Rule 6 — GO Test

Before declaring GO, prove through an end-to-end test that:

```
valid authority
    ↓
preparation succeeds
    ↓
authority is revoked
    ↓
ExecuteAction
    ↓
current kernel.Authorize
    ↓
DENIED
    ↓
executor was NOT called.
```

Also prove:

```
valid authority
    ↓
preparation succeeds
    ↓
target changes
    ↓
ExecuteAction
    ↓
DENIED
    ↓
executor was NOT called.
```

And:

```
valid authority
    ↓
preparation succeeds
    ↓
action changes
    ↓
ExecuteAction
    ↓
DENIED
    ↓
executor was NOT called.
```
