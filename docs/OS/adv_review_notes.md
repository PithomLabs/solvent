Yes. This adversarial review confirms that **stopping before Phase 4/6 was exactly the right decision**.

The key finding is severe: the kernel itself is healthy, but **the production execution path bypasses the kernel's authority verification entirely**. The review shows that an agent can reach execution with a promoted belief and policy approval despite having no valid authority target, approval, revocation check, or exact action/target binding. 

## What I would do now

Do **not** start Phase 4 or Phase 6.

The remediation should be treated as a **security-hardening phase between Phase 3 and Phase 4**.

The highest-priority issue is:

```text
CURRENT:

agent
  ↓
promoted belief
  ↓
policy allows
  ↓
intent / execution
  ↓
external action


REQUIRED:

agent
  ↓
workflow
  ↓
PrepareForAction
  ↓
current belief/evidence/debt/contradictions
  ↓
current policy
  ↓
current actor/action/target relationship
  ↓
kernel.Authorize
  ↓
current authority
  ↓
exact target/action binding
  ↓
revocation check
  ↓
executor
  ↓
external action
```

The review confirms that the service authority package exists but is **dead code**, while wizard and MCP still call the kernel directly without the necessary authority verification path.  

### One important correction to the review

I would **not automatically accept F-8/F-9's claim that lack of token signing is itself a major security vulnerability**.

The current `workflow_token` is a server-side database row, not a JWT-like self-contained authorization token. The critical invariant is that the token must not become authority and must not bypass current kernel state. The review correctly proves the more important failure: **the system doesn't re-read authority at all**. 

So I would clarify the architecture as:

```text
Opaque DB-backed workflow token
    ≠
self-authenticating authority token
```

Signing can be useful later if tokens become portable/self-contained, but **do not add cryptography merely to satisfy a checklist**. That would violate the "complexity must be earned" rule.

The real mandatory security test should therefore be phrased:

```text
valid stale workflow token
+
revoked current authority
=
DENIED
```

rather than requiring signing unless the implementation actually adopts signed tokens.

## The actual remediation prompt

Give the coding agent this now:

```text
STOP.

Phase 4 and Phase 6 remain BLOCKED.

The adversarial architecture review found critical/high defects in
the execution path. Do not proceed to UI or demo work.

Your task is REMEDIATION ONLY.

Do not broaden scope.
Do not redesign the kernel.
Do not add unrelated abstractions.
Do not add compliance functionality.

==================================================
1. PRIMARY SECURITY FIX
==================================================

Wire the real production execution path through the authority
verification boundary.

The following paths must no longer permit execution merely because
a belief is promoted and policy allows:

- wizard execution path
- MCP execution path
- any service execution path

Every consequential action must pass through:

    PrepareForAction(...)
        →
    current policy validation
        →
    current Solvent authority verification
        →
    executor

Use the existing kernel authority semantics.

Do not create a second authority engine.

==================================================
2. IMPLEMENT PrepareForAction
==================================================

Implement:

    PrepareForAction(
        scenarioID,
        beliefID,
        action,
        target,
        actor
    ) → PreparationResult

The preparation boundary must re-read CURRENT state.

It must evaluate:

- current belief
- current evidence
- current debt
- contradictions
- current authority
- current target snapshot
- current activation
- current revocation state
- current policy
- actor/action/target relationship

Do not use cached authority.
Do not use workflow token contents as authoritative state.
Do not trust browser state.
Do not trust agent-provided approval.

==================================================
3. RESTORE KERNEL AUTHORIZATION AS THE AUTHORITY GATE
==================================================

The final consequential authorization decision must ultimately
depend on the existing Solvent authority semantics.

Verify:

- authority exists
- authority is currently active
- authority has not been revoked
- target snapshot matches
- action matches
- actor/principal relationship matches
- required promoted justification exists
- all existing kernel gates remain enforced

Use the existing kernel.Authorize semantics where applicable.

Do NOT implement a parallel SQL-based authority checker in the
service layer unless the existing kernel API is genuinely
insufficient.

If the kernel API is insufficient, STOP and produce an ADR before
changing the kernel.

==================================================
4. POLICY MUST NOT BECOME AUTHORITY
==================================================

Refactor policy evaluation so policy is a constraint, not the final
authority decision.

This must fail:

    policy = ALLOW
    authority = ABSENT
    → DENIED

This must fail:

    policy = ALLOW
    authority = VALID
    target = WRONG
    → DENIED

This must fail:

    policy = ALLOW
    authority = REVOKED
    → DENIED

This may succeed:

    policy = ALLOW
    authority = VALID
    exact target/action match
    → ALLOWED

Policy may constrain authority.

Policy may never manufacture authority.

==================================================
5. EXECUTOR BOUNDARY
==================================================

Create ONE production execution path:

    ExecutionService
        ↓
    Executor
        ↓
    provider adapter

Executor receives a CURRENT VALIDATED authorization decision.

Executor must not:

- approve
- promote
- revoke
- create authority
- manufacture evidence-based authority

The executor only executes already-authorized actions.

==================================================
6. WORKFLOW SEMANTICS
==================================================

The current workflow is:

    pending → prepared → executing → completed/failed

Determine whether this can remain as an internal implementation
state while preserving the conceptual security boundaries:

    INVESTIGATING
    EVIDENCE_REVIEW
    HUMAN_REVIEW
    APPROVED
    AUTHORIZATION_READY
    EXECUTION
    COMPLETED
    REJECTED
    CANCELLED

Do not allow "prepared" to mean:

    approved
    authorized
    authority exists

"prepared" must mean only that preparation/revalidation succeeded.

If the simplified state model is retained, document the mapping and
prove that it does not collapse human review or authority semantics.

==================================================
7. TOKEN SECURITY
==================================================

Keep workflow tokens non-authoritative.

Do not add authority fields to workflow_token.

Do not use token state as proof of current authority.

The required security property is:

    valid stale workflow token
    +
    revoked current authority
    =
    DENIED

Also test:

    valid token + changed target = DENIED
    valid token + changed action = DENIED
    valid token + changed actor = DENIED
    valid token + changed policy = policy re-evaluated
    expired token = DENIED where expiry is configured
    invalid transition = DENIED
    manipulated token/state = DENIED where applicable

Do not add HMAC/JWT signing solely to satisfy the test.

If tokens remain opaque DB-backed identifiers, document that the
security property comes from server-side state and authenticated
access, not token self-authentication.

==================================================
8. ACTOR / AUTHENTICATION
==================================================

Do not trust:

    action_source = "user_typed"

as proof of human interaction.

Authentication must be derived from the trusted MCP/HTTP boundary.

Do not allow a malicious caller to assert HUMAN merely by changing
request data.

Preserve:

    ActorType != Identity != Authentication

Do not build an identity provider.

==================================================
9. WIZARD PATH
==================================================

The wizard's Authorize/Execute path must no longer create an
action that can execute without current authority.

Trace:

    UI request
      →
    service
      →
    preparation
      →
    kernel authorization
      →
    executor

The UI must never directly determine authorization.

A button labeled "Authorize" cannot itself imply authority.

==================================================
10. MCP PATH
==================================================

Trace:

    MCP request
      →
    trusted actor context
      →
    policy
      →
    PrepareForAction
      →
    kernel.Authorize
      →
    executor

Do not trust client-supplied provenance fields as authentication.

==================================================
11. AUDIT PATH
==================================================

Once execution is correctly gated, wire production activity logging.

Record distinct events for:

    Solvent authorization
    adapter invocation
    provider response
    executor result

Preserve:

    authorization succeeded + execution failed

as distinct from:

    authorization denied + execution never happened

Do not replace/refactor kernel authority records into activity logs.

==================================================
12. ADVERSARIAL TEST SUITE
==================================================

Write real tests.

Do not merely inspect code.

Mandatory tests:

1. no authority → DENIED
2. promoted belief + policy allow + no authority → DENIED
3. valid authority + wrong target → DENIED
4. valid authority + wrong action → DENIED
5. revoked authority → DENIED
6. stale authority → DENIED
7. stale workflow token + revoked authority → DENIED
8. fake approval → DENIED
9. agent attempts human-only operation → DENIED
10. actor spoofing HUMAN → DENIED
11. browser/workflow state cannot create authority
12. executor cannot create authority
13. external provider cannot create authority
14. malformed provider output does not authorize
15. expired workflow token → DENIED where applicable
16. invalid workflow transition → DENIED

Most important regression:

    valid stale workflow token
    +
    revoked current authority
    =
    DENIED

==================================================
13. TEST AT ALL FOUR LEVELS
==================================================

KERNEL
SERVICE
ADAPTER
END-TO-END

The end-to-end tests must exercise the ACTUAL production execution
path.

Do not test only isolated service methods.

==================================================
14. DEAD CODE
==================================================

After wiring the production path, inspect:

    service/authority
    service/workflow
    service/executor

Remove dead implementations or clearly establish their production
ownership.

There must not be a situation where a security boundary exists in
code but the real application bypasses it.

==================================================
15. DO NOT MODIFY THE KERNEL UNLESS NECESSARY
==================================================

Kernel is currently considered healthy.

Do not modify kernel code merely to make service integration easier.

If a kernel modification becomes necessary:

STOP.

Create an ADR containing:

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

Default decision remains:

    REJECT KERNEL CHANGE

==================================================
16. REQUIRED OUTPUT
==================================================

Before saying remediation is complete, provide:

A. Execution-path diagram before/after

B. List of all production consequential-action entry points

C. Exact location where current authority is verified

D. Exact fields used for action/target binding

E. Exact revocation check

F. Exact actor/authentication boundary

G. List of authority-related DB reads at execution time

H. List of code paths that can create authority

I. List of code paths that can execute actions

J. Adversarial test matrix:

attack | expected | actual | test location

K. All files changed

L. All migrations changed/added

M. Any kernel changes
   If none, explicitly state:
   "No kernel changes."

N. Remaining risks

==================================================
17. FINAL SECURITY GATE
==================================================

Do NOT continue to Phase 4 or Phase 6 automatically.

End with exactly one:

GO

Meaning:
- no critical/high security findings remain
- production execution uses current authority verification
- adversarial tests pass
- service boundaries are actually wired
- no second authority engine exists

OR

HOLD

Meaning:
- any critical/high finding remains
- authority verification is incomplete
- production path still bypasses the service/security boundary
- mandatory adversarial tests fail

Security correctness takes precedence over schedule.
```

### One further architectural point

The review exposes something important beyond the individual bugs:

**The problem was not that the kernel was too weak. The problem was that the product had successfully built a stronger-looking service architecture without actually putting it on the critical path.**

That is exactly the kind of failure the Solvent philosophy is supposed to prevent.

The review explicitly confirms the kernel and authority schema are clean, while the service boundary is effectively bypassed. 

So the next milestone should not be "finish Phase 2."

It should be:

> **Make the real execution path pass through the architecture we already designed, then attack that path until it survives.**

Only after that produces **GO** should Phase 4 begin.





