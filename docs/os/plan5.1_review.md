I reviewed **Plan 5.1** against the entire sequence so far, including the original plan, adversarial review, Plan 4, Plan 4.1, and Plan 4.2.

This is **very close**, and the strategic decision is correct: do not invent a fake production deployment capability; prove the security boundary with a test-only recording executor. The plan explicitly captures that scope. 

However, I would make **three important corrections before locking it**.

# 1. The workflow-token story is still underspecified

This is the most important remaining issue.

Plan 5.1 says:

```text
PrepareForAction
→ kernel.Authorize
→ Recording/Test Executor
```

and later requires:

```text
stale token + revoked authority → DENIED
expired token → DENIED
```

But the proposed:

```go
ExecuteAction(
    ctx,
    scenarioID,
    beliefID,
    action,
    targetID,
    actorID,
    params,
)
```

does **not contain a workflow-token identifier**. 

And `PrepareForAction` also does not take a token.

That creates an ambiguity:

> **How can `ExecuteAction` reject an expired or invalid workflow token if the execution boundary does not receive or resolve one?**

The plan must answer this explicitly.

There are two legitimate designs:

### A. Token is required for workflow-driven execution

Then make it explicit:

```go
ExecuteAction(
    ctx,
    workflowTokenID,
    action,
    targetID,
    params,
)
```

and the service resolves the workflow, validates the token, then independently validates current authority.

### B. Token is purely optional continuity metadata

Then remove token validity from the core execution security contract.

In that design:

```text
workflow token
    ↓
workflow continuity only

ExecuteAction
    ↓
current authoritative state
    ↓
kernel.Authorize
```

An expired workflow token may prevent workflow progression, but it is not what establishes or denies authority.

Given our locked architecture, **I prefer B unless the actual product workflow genuinely requires tokens at execution time.**

The important thing is not to have a test called `TestExpiredTokenDenied` when the execution API does not even consume the token.

---

# 2. `workflow` is still described as wired but not actually part of execution

Plan 5.1 says:

> `service/workflow` is wired but not invoked by any execution path. 

Yet `ExecuteAction` says:

```text
transition workflow token to executing
```



That is inconsistent.

Either:

```text
ExecuteAction
    ↓
WorkflowService
    ↓
token → executing
```

is actually implemented,

or workflow is not part of the execution path yet.

Do not allow another "wired-looking but semantically disconnected" boundary. That was the original failure mode.

I would require:

> Every dependency shown in the canonical execution path must have a real production caller, except the test executor itself.

So after implementation, this should be mechanically discoverable:

```text
MCP / Wizard / future execution entry
        ↓
ExecuteAction
        ↓
PrepareForAction
        ↓
kernel.Authorize
        ↓
ExecutionService
        ↓
Executor
```

Workflow should only appear if it actually contributes to that path.

---

# 3. The "18 denial tests" need better classification

The test matrix is good overall. 

But some entries aren't really `ExecuteAction` denial tests:

```text
executor cannot create authority
provider output cannot create authority
workflow state cannot create authority
```

Those are architectural properties, not necessarily execution-path tests.

I would split the test suite into:

### Execution authorization tests

```text
no authority
wrong target
wrong action
revoked authority
stale authority
policy allow + absent authority
policy allow + revoked authority
target mutation
action mutation
stale token if token participates
expired token if token participates
```

### Capability-boundary tests

```text
executor cannot create authority
provider cannot create authority
workflow cannot create authority
agent cannot approve itself
actor spoofing fails
```

### Integration/adapter tests

```text
malformed provider output
external provider cannot manufacture authority
```

That gives the final security report much better evidentiary value.

---

# One thing Plan 5.1 gets exactly right

This decision is excellent:

> **Do not create fake production executors.** 

And the TOCTOU language is appropriately honest:

```text
successful current kernel authorization
    →
immediately precedes executor invocation
```

without pretending that Solvent can atomically transact with an external provider. 

That is the correct security claim for this stage.

---

# I would also make one small wording change to the core invariant

Plan 5.1 currently says:

```text
No successful kernel.Authorize → no executor invocation
```



Make it stronger:

```text
No successful CURRENT kernel.Authorize for the EXACT actor/action/target tuple
    →
no executor invocation.
```

The word **CURRENT** and the exact tuple are important because stale authorization is the central threat being tested.

---

# Final verdict

### **Plan 5.1: APPROVE WITH MINOR REFINEMENT**

The architecture is now solid enough that I would **not send it through another major redesign**.

Make these three corrections:

```text
1. Explicitly define whether workflow_token participates in execution.
   Do not test token expiry/staleness through an API that doesn't consume
   or resolve the token.

2. Resolve the workflow contradiction:
   either actually use WorkflowService in ExecuteAction or keep workflow
   outside the execution path.

3. Split the test suite into:
   execution authorization,
   capability-boundary,
   and adapter/integration tests.
```

Then lock it.

## The exact amendment I would append

```text id="1z7u8c"
## Final Contract Clarifications

### A. Workflow Token Participation

Explicitly define whether workflow tokens participate in consequential
execution.

If workflow tokens are continuity-only:

    token
      ↓
    workflow continuity

    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    executor

Token validity MUST NOT be required for authority unless the actual
workflow contract requires it.

If a token is required by the execution workflow, ExecuteAction MUST
receive or resolve the token explicitly and validate it before execution.

Do not define tests for expired/stale tokens unless the execution path
actually consumes or resolves the token.

### B. WorkflowService Wiring

Any service named in the canonical execution path must have a real
production caller.

If ExecuteAction transitions workflow state, it must invoke the actual
WorkflowService.

If workflow state is not required for execution, remove it from the
execution path rather than leaving a nominally "wired" but unused
security boundary.

No dead security boundary is acceptable.

### C. Security-Test Classification

Separate tests into:

1. Execution Authorization
   - no authority
   - wrong target
   - wrong action
   - revoked authority
   - stale authority
   - target mutation
   - action mutation
   - policy allow + authority absent
   - policy allow + authority revoked
   - stale/expired token where token participates

2. Capability Boundaries
   - executor cannot create authority
   - provider cannot create authority
   - workflow cannot create authority
   - agent cannot approve itself
   - actor spoofing fails

3. Adapter / Integration
   - malformed provider output
   - provider claims cannot create authority

The final report must distinguish these categories.

### D. Strengthened Execution Invariant

The required invariant is:

    no successful CURRENT kernel.Authorize
    for the EXACT actor/action/target tuple
    →
    no executor invocation.

A previous authorization decision, prepared state, workflow state,
token, policy result, or browser state must never substitute for the
current kernel authorization decision.
```

Once those are incorporated, **I would lock Plan 5.1 and tell the coding agent to implement it**.

And importantly, after it produces the recording-executor integration tests and adversarial review, **that becomes the real GO/HOLD gate** before Phase 4. The plan already establishes that sequencing correctly.  
