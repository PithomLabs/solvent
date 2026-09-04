I reviewed **Plan 4.1** as the remediation plan against the adversarial findings. It is substantially stronger than Plan 4 and fixes the major architectural ambiguities I identified. The plan now explicitly establishes one real execution path, separates intent creation from execution, keeps promotion independent from authority, demotes policy to constraints, and requires the production path—not isolated service tests—to pass the adversarial suite.  

I would still make **three corrections before locking it**.

## 1. `ExecuteAction(... executorFn func(...))` is still too permissive

This is the biggest remaining issue.

Plan 4.1 defines:

```go
func (s *Service) ExecuteAction(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    executorFn func(ctx context.Context) (string, error),
)
```



That technically permits a caller to supply an arbitrary execution function.

That undermines the architectural invariant:

```text
ExecuteAction
    ↓
current authorization
    ↓
ExecutionService
    ↓
Executor
    ↓
provider
```

because the actual API becomes:

```text
caller
  ↓
ExecuteAction
  ↓
arbitrary function supplied by caller
```

The executor abstraction becomes advisory rather than authoritative.

### Fix

`ExecuteAction` should resolve the executor internally from the trusted `ExecutionService`/executor registry.

Conceptually:

```go
ExecuteAction(
    ctx,
    scenarioID,
    beliefID,
    action,
    targetID,
    actorID,
    params,
) → ExecutionResult
```

Then internally:

```text
ExecuteAction
    ↓
PrepareForAction
    ↓
kernel.Authorize
    ↓
ExecutionService.Execute(...)
    ↓
registered Executor
    ↓
provider adapter
```

The caller must **never inject the executor function**.

This is an important security-boundary hardening.

---

## 2. Be careful not to create an authority checker inside `service/authority`

The plan says:

> Re-read current authority state → build `AuthorityTuple` → `kernel.Authorize`. 

That is directionally correct, but we need a hard distinction between:

```text
service gathers contextual inputs
```

and:

```text
service independently determines whether authority exists
```

The kernel is already the authority source of truth.

The service must **not** implement something like:

```go
SELECT target_activation...
SELECT target_revocation...
if active && !revoked && targetMatches {
    allowed = true
}
```

and then call that "preparation."

That would recreate the exact second-authority-engine problem we just found.

The preferred flow is:

```text
service
    ↓
construct exact AuthorityTuple from request/current context
    ↓
kernel.Authorize(tuple)
    ↓
kernel performs authoritative verification
```

The service may retrieve supporting context needed to construct the tuple, but **the final authority determination belongs to the kernel**.

The plan already says the kernel is healthy and should remain unchanged. 

I would make this explicit in Plan 4.1.

---

## 3. The authentication requirement is architecturally correct but operationally underspecified

The plan says:

```text
authenticated principal from trusted request context
        ↓
actor classification
        ↓
service
```



Good.

But there is a practical danger: the repository may not actually have authenticated MCP/HTTP principals yet.

The agent must **not fabricate an authentication layer merely to make the test pass**.

The safe rule should be:

```text
No trusted authenticated principal
    ↓
no human-authorized consequential operation
    ↓
DENIED / unavailable
```

Do not replace client-declared:

```text
user_typed
HUMAN
actor_id
```

with another client-declared field and call that authentication.

If real authentication infrastructure does not exist yet, the remediation should identify the boundary and **fail closed**, rather than inventing fake authentication.

---

# One more subtle issue: intent creation

Plan 4.1 now says:

```text
handleSolventAuthorizeAction
    ↓
policy constraints
    ↓
PrepareForAction
    ↓
kernel.IntentOnPromoted
```



This is acceptable **only if `IntentOnPromoted` is clearly an intent/request operation and does not itself execute anything**.

The critical invariant should be:

```text
intent creation ≠ execution
```

and:

```text
authorization decision at request time
    ≠
authorization decision at execution time
```

The later:

```text
ExecuteAction
    ↓
re-read current authority
    ↓
kernel.Authorize
```

must remain mandatory even when an earlier request passed authorization.

Plan 4.1 does state this, which is good. 

---

# The test suite is now in much better shape

The addition of:

```text
valid authority + target mutates before execution → DENIED
valid authority + action mutates before execution → DENIED
```

is exactly what was missing. 

And the explicit requirement that tests exercise the **actual production execution path** is critical. 

The strongest regression remains:

```text
valid stale workflow token
+
revoked current authority
=
DENIED
```



That's the test I would treat as the symbolic centerpiece of the remediation.

---

# My verdict

### Plan 4.1: **HOLD — minor refinement required**

Not because the architecture is wrong.

Quite the opposite: **the architecture is now correct enough that we should tighten the implementation contract rather than redesign it.**

The remaining corrections are:

```text
1. ExecuteAction must NOT accept caller-supplied executorFn.

2. service/authority must never independently implement authority
   determination; kernel.Authorize remains the final authority oracle.

3. Missing authentication must fail closed rather than be simulated.

4. Intent creation must remain explicitly distinct from execution.
```

Everything else in the refined plan is aligned well with the adversarial review.

## I would append this exact amendment to Plan 4.1

```text
## 14. Final Implementation Hardening Rules

### Rule 1 — Executor Cannot Be Caller-Injected

ExecuteAction MUST NOT accept an arbitrary executor function from
the caller.

Forbidden:

    ExecuteAction(..., executorFn func(...))

Required conceptual shape:

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

The caller may supply action parameters, but never the execution
implementation.

The executor must be selected internally from the trusted
ExecutionService/registry.

### Rule 2 — Kernel Is the Final Authority Oracle

The service layer may gather current context and construct the
AuthorityTuple required by the kernel.

The service layer MUST NOT independently determine whether authority
exists.

Forbidden:

    service SQL/query logic
        ↓
    service decides authority=true
        ↓
    executor

Required:

    service gathers tuple/context
        ↓
    kernel.Authorize(tuple)
        ↓
    authoritative decision

Do not implement a second authority checker in service/authority.

### Rule 3 — Authentication Fails Closed

Do not fabricate authentication.

Never treat request-body values such as:

    actor
    actor_type
    action_source
    user_typed
    HUMAN

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

    current-state preparation
        ↓
    kernel.Authorize
        ↓
    executor

Earlier authorization results, workflow state, cached values, and
tokens must never be reused as execution authority.

### Rule 5 — Production Path Invariant

The following invariant must be mechanically true:

    No successful current kernel.Authorize
        → no executor invocation.

No MCP handler, wizard handler, pipeline, workflow token, policy
decision, or external provider may bypass this invariant.

### Rule 6 — GO Test

Before declaring GO, prove through an end-to-end test that:

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

Also prove:

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

And:

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

With those amendments, **I would lock Plan 4.1 and let the coding agent implement it**.

The plan already has the correct final gate: no Phase 4 or Phase 6 until every consequential execution path uses current authority verification, no executor is reachable without successful `kernel.Authorize`, policy cannot create authority, stale authority is rejected, and the adversarial tests pass. 

**So: do not proceed to UI/demo yet. Refine 4.1 with the four rules above, then implement the remediation.**
