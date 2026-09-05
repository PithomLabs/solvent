# Plan 4: Security-Hardening Remediation

**Status:** HOLD — Phase 4 and Phase 6 blocked until this produces GO.

**Trigger:** Adversarial architecture review found critical/high defects in the
execution path. The kernel is healthy. The service authority boundary is dead code.

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

## 3. What Needs to Change

### Files to modify

1. `service/authority/authority.go` — Refactor to use real kernel
   `AuthorityTuple` + `kernel.Authorize`, not the simplified version.
2. `cmd/solvent-mcp/tools.go` — Wire `handleSolventAuthorizeAction` and
   `handleSolventPromote` through authority verification.
3. `internal/wizard/refusal.go` — Wire `Server.Authorize` through authority
   verification (demo-specific, lighter touch).
4. New: `service/authority/authority_test.go` — Adversarial test suite.

### What NOT to change

- Kernel code (healthy)
- `db/001_schema.sql` through `db/006_*.sql` (frozen)
- Internal packages (`belief`, `intent`, `pipeline`, `derive`, `normalize`) —
  these are evidence ingestion, not action execution

## 4. Remediation Steps

### Step 1: Refactor `service/authority` to use real kernel types

Replace the simplified `viewBelief` / `PreparedAction` with the kernel's actual
`AuthorityTuple` and `Authorize` method. The service becomes a thin orchestration
layer:

```
PrepareForAction(scenarioID, beliefID, action, targetID, actorID)
  → kernel.Authorize(AuthorityTuple{...})
  → return AuthorizationDecision
```

### Step 2: Define the production execution entry point

Create `service/authority.ExecuteAction` — the ONE path for consequential
actions:

```
ExecuteAction(ctx, scenarioID, action, targetID, actorID)
  → re-read belief state
  → re-read authority state
  → kernel.Authorize
  → if allowed: execute via adapter
  → log result
```

### Step 3: Wire MCP path

Modify `handleSolventAuthorizeAction` to call
`service/authority.PrepareForAction` before `kernel.IntentOnPromoted`. The MCP
handler becomes:

```
handleSolventAuthorizeAction
  → validate inputs
  → service.PrepareForAction(beliefID, action, targetID, actorID)
  → kernel.IntentOnPromoted (if authority allows)
```

### Step 4: Wire Wizard path

The wizard is demo-specific. Its `Server.Authorize` should call
`service/authority.PrepareForAction` but with demo-appropriate defaults (the
demo has one target, one actor).

### Step 5: Adversarial test suite

16 mandatory tests covering the attack matrix.

### Step 6: Verify no dead code remains

After wiring, confirm `service/authority` is imported by production paths.

## 5. Required Security Properties

```
1.  promoted belief + policy allow + no authority     → DENIED
2.  valid authority + wrong target                    → DENIED
3.  valid authority + wrong action                    → DENIED
4.  revoked authority                                 → DENIED
5.  stale token + revoked authority                   → DENIED
6.  fake approval                                     → DENIED
7.  agent asserts HUMAN                               → DENIED
8.  workflow state creates authority                  → DENIED
9.  executor creates authority                        → DENIED
10. external provider creates authority               → DENIED
11. malformed provider output authorizes              → DENIED
12. expired token                                     → DENIED
13. invalid workflow transition                       → DENIED
14. policy ALLOW + authority ABSENT                   → DENIED
15. policy ALLOW + authority VALID + wrong target     → DENIED
16. policy ALLOW + authority REVOKED                  → DENIED
```

## 6. Adversarial Test Matrix

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

Most important regression:

```
valid stale workflow token
+
revoked current authority
=
DENIED
```

## 7. Token Security (Clarification)

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

## 8. Actor / Authentication Boundary

Do not trust:

```
action_source = "user_typed"
```

as proof of human interaction. Authentication must be derived from the
trusted MCP/HTTP boundary. Do not allow a malicious caller to assert HUMAN
merely by changing request data.

Preserve:

```
ActorType ≠ Identity ≠ Authentication
```

Do not build an identity provider.

## 9. Workflow Semantics (Clarification)

The current workflow is:

```
pending → prepared → executing → completed/failed
```

This can remain as an internal implementation state while preserving the
conceptual security boundaries:

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

"prepared" must mean only that preparation/revalidation succeeded.

## 10. Required Output Before Completion

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

## 11. Final Security Gate

Do NOT continue to Phase 4 or Phase 6 automatically.

End with exactly one:

**GO**

Meaning:
- no critical/high security findings remain
- production execution uses current authority verification
- adversarial tests pass
- service boundaries are actually wired
- no second authority engine exists

OR

**HOLD**

Meaning:
- any critical/high finding remains
- authority verification is incomplete
- production path still bypasses the service/security boundary
- mandatory adversarial tests fail

Security correctness takes precedence over schedule.

## 12. Kernel Change Policy

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
