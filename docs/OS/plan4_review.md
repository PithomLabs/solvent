I reviewed **Plan 4** carefully. It is much better than the original implementation direction, and it correctly responds to the adversarial findings. The central diagnosis is right: the kernel is healthy; the real defect is that the production execution path bypasses the authority boundary. 

However, **I would not give Plan 4 an unconditional GO yet**. There are a few architectural corrections I would make before handing it back to the coding agent.

## 1. The biggest problem: `IntentOnPromoted` is still in the proposed execution path

Plan 4 says:

```text
handleSolventAuthorizeAction
  → service.PrepareForAction(...)
  → kernel.IntentOnPromoted (if authority allows)
```



This is the most important thing I would change.

The approved architecture says:

```text
prepare
→ current authority verification
→ executor
```

But Plan 4 introduces:

```text
prepare
→ authority check
→ IntentOnPromoted
```

That is not necessarily wrong **if `IntentOnPromoted` is only creating an intent and execution happens later through the validated executor boundary**.

But the plan currently does not establish that clearly enough.

The adversarial review found that the current production path is:

```text
handler → kernel.Store → DB
```

and that the service layer is dead code. 

So the remediation must be explicit about the distinction between:

```text
REQUEST / INTENT CREATION
```

and:

```text
CONSEQUENTIAL EXECUTION
```

Otherwise the agent may "fix" `Authorize` while leaving the actual execution mechanism architecturally ambiguous.

### I would change Step 3 to:

```text
handleSolventAuthorizeAction
    ↓
trusted actor context
    ↓
PrepareForAction
    ↓
current policy constraints
    ↓
kernel.Authorize
    ↓
create/record action intent if appropriate
```

Then separately:

```text
ExecuteAction
    ↓
re-read current authoritative state
    ↓
kernel.Authorize
    ↓
ExecutionService
    ↓
Executor
    ↓
external system
```

**Most importantly: authorization at intent creation must not be treated as authorization at execution time.**

The final execution boundary must independently revalidate.

---

# 2. `ExecuteAction` must become the actual critical path, not just a new function

Plan 4 says:

> Create `service/authority.ExecuteAction` — the ONE path for consequential actions. 

Good.

But then the remediation needs to make an even stronger requirement:

> **There must be exactly one production consequential-execution path.**

Right now the report says there are three independent production paths:

```text
MCP
Wizard
Pipeline
```

all calling `kernel.Store` directly. 

So the acceptance criterion should not merely be:

```text
service/authority imported somewhere
```

It should be:

```text
NO production consequential execution path may reach an external
executor/provider without passing through ExecuteAction.
```

That is a much stronger and more useful gate.

---

# 3. Do not make `handleSolventPromote` pass through authority verification

This line deserves correction:

> `cmd/solvent-mcp/tools.go` — Wire `handleSolventAuthorizeAction` **and `handleSolventPromote`** through authority verification. 

I would **not do that automatically**.

Promotion and authorization are different lifecycle operations.

The architecture explicitly preserves:

```text
belief lifecycle
authority lifecycle
action_intent lifecycle
```

as distinct concepts. 

Promotion establishes the state of a belief.

Authority establishes permission for a consequential action.

Therefore:

```text
Promote(belief)
```

should not require an already-existing authority merely because the later execution requires one.

Otherwise you risk introducing a circular dependency:

```text
belief must be promoted
→ authority needed
→ authority may require promoted justification/belief
→ ...
```

The actual fix is:

```text
PROMOTE
    → existing promotion/debt semantics

AUTHORIZE/EXECUTE
    → current authority semantics
```

So I would remove `handleSolventPromote` from the authority-verification requirement unless repository inspection proves that `Promote` itself represents a consequential external action—which the current architecture says it does not.

---

# 4. Policy is correctly identified as dangerous, but the fix should be stronger

Plan 4 correctly says:

> Policy must be demoted from boolean `allowed` to constraint-only. 

That is exactly right.

I would make the required contract explicit.

Instead of:

```go
AuthorizeToolCall(...) (bool, ...)
```

prefer something conceptually like:

```go
type PolicyDecision struct {
    Allowed            bool
    Constraints        []Constraint
    RequiresHuman      bool
    RequiresAuthority  bool
    RequiredStage      WorkflowState
    Reasons            []string
}
```

But even that can still be misleading because `Allowed` sounds like final authorization.

Better:

```go
type PolicyConstraints struct {
    ActorPermitted     bool
    StagePermitted     bool
    ToolPermitted      bool
    RequiresHuman      bool
    RequiresAuthority  bool
    Reasons            []string
}
```

Then:

```text
Policy constraints
        +
kernel.Authorize
        =
final authorization decision
```

That makes it structurally harder for the service layer to become the authority engine again.

---

# 5. The actor/authentication fix is currently underspecified

Plan 4 correctly states:

```text
action_source = "user_typed"
```

must not be trusted as proof of a human. 

But it says:

> Authentication must be derived from the trusted MCP/HTTP boundary.

Good principle, but the coding agent needs a concrete boundary.

Otherwise it may simply replace:

```text
action_source = "user_typed"
```

with another client-supplied field.

I would require the remediation to identify:

```text
authenticated principal
        ↓
trusted request context
        ↓
actor classification
        ↓
service
```

And explicitly prohibit:

```text
request.body.actor
request.body.actor_type
request.body.action_source
```

from being treated as authentication.

This is particularly important because the adversarial review already demonstrated that `action_source` is client-declared. 

---

# 6. The stale-token test wording is now correct

This part of Plan 4 is excellent:

```text
Opaque DB-backed workflow token
≠
self-authenticating authority token
```

and:

```text
valid stale workflow token
+
revoked current authority
=
DENIED
```



I agree with the decision **not to add HMAC merely to satisfy the review**.

The real security property is current authoritative state, not cryptographic ceremony.

The only change I would make is to ensure the test actually goes through the **real production execution path**, rather than testing a service method in isolation.

---

# 7. Workflow clarification is good, but I would preserve the conceptual state model

The clarification that:

```text
pending → prepared → executing → completed/failed
```

may remain an implementation representation is reasonable. 

But there is a dangerous sentence:

> `"prepared" must mean only that preparation/revalidation succeeded.`

Good—but the agent should also prove that `prepared` does **not imply**:

```text
human approved
authority exists
execution permitted
```

That distinction belongs in tests, not merely documentation.

I would require:

```text
prepared + no authority → execution DENIED
prepared + revoked authority → execution DENIED
prepared + wrong target → execution DENIED
```

---

# 8. Audit is missing from the remediation's actual file plan

This is an important omission.

The adversarial review found:

> production paths do not record authorization, adapter, provider, or executor outcomes in a unified audit trail. 

Plan 4 says:

```text
ExecuteAction
→ log result
```



but the "Files to modify" section does not explicitly include the audit service.

That should be corrected.

At minimum:

```text
service/audit/...
```

must be included in the remediation scope, because otherwise we risk fixing authorization while leaving the production audit trail fragmented.

---

# 9. The test list is good, but one test is missing from the explicit 16

The explicit matrix has:

```text
wrong target
wrong action
revocation
stale token
fake approval
actor spoofing
policy allow + absent authority
policy allow + revoked authority
...
```



I would add a dedicated test for:

```text
VALID AUTHORITY + EXECUTION-TIME TARGET MUTATION
→ DENIED
```

because that is the confused-deputy defense in its strongest form.

Likewise:

```text
VALID AUTHORITY
+
ACTION CHANGES AFTER PREPARATION
→ DENIED
```

That demonstrates revalidation rather than merely initial validation.

---

# 10. The final gate needs one stronger criterion

Current Plan 4 says GO requires:

> production execution uses current authority verification. 

I would sharpen it to:

> **Every production consequential execution path is mechanically incapable of reaching the executor without a successful current-authority verification.**

That difference matters.

"Uses verification" can be satisfied by inserting a check somewhere.

"Cannot reach executor without verification" establishes an actual security boundary.

---

# Verdict on Plan 4

### Architecture: **Very good**

### Diagnosis: **Correct**

### Kernel strategy: **Correct**

### Token strategy: **Correct**

### Policy correction: **Correct direction**

### Workflow correction: **Mostly correct**

### Main remaining issue: **the actual execution path is not specified tightly enough**

### Secondary issue: **Promote should not automatically be coupled to authority**

### Secondary issue: **audit remediation needs to be explicitly included**

### Overall:

> **HOLD — but only for refinement of Plan 4, not for another architectural redesign.**

This is importantly different from the previous HOLD.

The adversarial review found the actual critical flaw: the kernel was sound but the service boundary was bypassed.  Plan 4 correctly attacks that flaw. Now we need to make sure the remediation itself cannot produce another half-connected security architecture.

## I would send the coding agent this correction

```text
PLAN 4 REFINEMENT — SECURITY CRITICAL

Before implementation, incorporate these corrections.

1. ONE REAL EXECUTION PATH

There must be exactly one production path for consequential
external execution:

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

No production MCP, wizard, or pipeline path may call an executor
or external consequential operation directly.

A service being imported somewhere is not sufficient.

The security invariant is:

    no successful current kernel authorization
    → executor cannot be reached

2. SEPARATE INTENT CREATION FROM EXECUTION

Intent creation and consequential execution are different.

It is acceptable for an authorize/request handler to create or
record an action intent, but that MUST NOT constitute final
execution authorization.

Execution must independently pass through ExecuteAction.

Therefore:

    request/intent creation
        ↓
    current authorization decision as appropriate

and later:

    ExecuteAction
        ↓
    current authority re-read
        ↓
    kernel.Authorize
        ↓
    executor

Never rely on an earlier authorization result.

3. DO NOT COUPLE PROMOTION TO AUTHORITY

Do not automatically route Promote through authority verification.

Promotion belongs to the belief lifecycle.

Authority belongs to the authority lifecycle.

Preserve:

    belief promotion
    ≠
    action authorization

Only consequential operations require the authority gate.

4. POLICY IS CONSTRAINTS, NOT AUTHORITY

Refactor policy so it cannot function as a final authorization
engine.

Do not allow:

    policy ALLOW
    + authority ABSENT
    = execution

The final decision must require both:

    policy constraints satisfied
    +
    kernel.Authorize(current authority)

Prefer a type/API that makes policy constraints explicit rather
than returning a semantically misleading final boolean "allowed".

5. AUTHENTICATION MUST BE TRUSTED

Do not treat these as authentication:

    actor in request body
    actor_type in request body
    action_source in request body
    user_typed in request body

Identify the trusted authentication boundary and derive the
authenticated principal from it.

Then derive actor classification from trusted context.

Document exactly where this occurs.

6. AUDIT IS PART OF THIS REMEDIATION

Wire the real production path to the activity/audit service.

Record separately:

    Solvent authorization
    adapter invocation
    provider response
    executor result

Preserve:

    authorization succeeded + execution failed

as different from:

    authorization denied + execution never occurred

7. PREPARATION MUST RE-READ CURRENT STATE

Immediately before consequential execution, re-read:

    current belief
    current evidence
    current debt
    contradictions
    current authority
    target snapshot
    activation
    revocation
    current policy
    actor
    action
    target

Do not rely on prepared state, cached values, browser state, or
workflow token state.

8. CONFUSED-DEPUTY REGRESSIONS

Add explicit tests:

    valid authority + wrong target = DENIED
    valid authority + wrong action = DENIED

    valid authority validated earlier
    + target changes before execution
    = DENIED

    valid authority validated earlier
    + action changes before execution
    = DENIED

9. WORKFLOW SEMANTICS

The internal:

    pending → prepared → executing → completed/failed

representation may remain.

But prove through tests that:

    prepared != approved
    prepared != authorized
    prepared != authority exists

Required tests:

    prepared + no authority = DENIED
    prepared + revoked authority = DENIED
    prepared + wrong target = DENIED

10. DEAD-CODE RULE

After remediation:

    service/authority
    service/workflow
    service/executor

must either be part of the real production path or be removed.

No security boundary may exist only as dead code.

11. FINAL GO REQUIREMENT

Do not declare GO merely because the tests pass.

GO requires:

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

If any of these are false:

    HOLD

Security correctness takes precedence over schedule.
```

**This is the version I would lock in.**

Once this remediation produces **GO**, *then* Phase 4 becomes the right next step. Phase 6 should follow after the security boundary is proven, because the demos should showcase the real production path rather than create another parallel path. The current Plan 4 already correctly keeps Phase 4/6 blocked until GO. 
