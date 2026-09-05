This second adversarial review is **correct to HOLD**, but it exposes a **scope problem in our gate**, not another authority flaw.

The reviewer confirmed that the actual security boundary is sound: `kernel.Authorize` remains the authority oracle, exact tuple binding and revocation work, policy cannot manufacture authority, and the live integration tests are strong. The problem is that **there is currently no real consequential execution capability to protect**.  

So I would **not** respond by inventing a fake production executor. That would violate the complexity-earned rule.

## The important distinction

We currently have:

```text
evidence
   ↓
belief
   ↓
promotion
   ↓
authority
   ↓
intent
```

but not:

```text
intent
   ↓
real external execution
```

The review confirms exactly that. There are zero production `ExecuteAction` callers and no production executor registrations. 

Therefore F-1:

> "No production execution path exists"

is a **product-scope fact**, not evidence that an existing production execution path is insecure.

The dangerous alternative would have been:

```text
real execution exists
+
execution bypasses authority
```

That was the original critical defect, and we fixed it.

We now have:

```text
no real execution capability
+
a tested future execution boundary
```

Those are very different situations.

---

# I would NOT wire a fake production executor

The reviewer says one option is:

> Wire `ExecuteAction` into at least one production execution path. 

I would reject that option.

We already explicitly decided that Solvent should **not invent fake production capabilities** merely to satisfy the architecture. That is still the right decision.

Likewise, I would **not wire `service/workflow` into execution** just because the reviewer wants every planned component to have a production caller.

The current architecture explicitly treats workflow as continuity-only. The implementation review should respect actual product scope rather than turning every future abstraction into mandatory v0 infrastructure.

---

# But we do need to clean up the dead-code problem

The reviewer is right about one thing:

> a "security boundary" that is not reachable is not a production security boundary. 

So we should stop describing `ExecuteAction` and `service/workflow` as current production security boundaries.

I would change the architecture status to:

```text
CURRENT:

Kernel authority
    ↓
service authorization / preparation
    ↓
intent creation

FUTURE:

intent
    ↓
ExecuteAction
    ↓
kernel.Authorize
    ↓
Executor
    ↓
external side effect
```

The future path is **designed and tested**, but not yet a production capability.

That is honest.

---

# The real issue is our GO definition

This is where I think we should correct the plan.

We previously defined GO as:

> every consequential production execution path uses `ExecuteAction`.

That makes sense **once consequential production execution exists**.

But when there is **zero consequential production execution**, the requirement becomes impossible to satisfy without inventing functionality.

That creates a bad incentive.

### Replace it with:

```text
CURRENT PRODUCT SECURITY GATE

GO requires:

1. Every existing consequential production execution path is
   authority-gated.

2. If no consequential production execution capability exists,
   this is explicitly documented as CURRENTLY DEFERRED.

3. Any future consequential execution implementation MUST use:

      ExecuteAction
          ↓
      current-state revalidation
          ↓
      kernel.Authorize
          ↓
      Executor

4. No production executor exists unless there is a real external
   capability to execute.

5. No dead code is presented as an active production security boundary.

6. The tested future execution boundary passes the adversarial suite.

7. No existing production bypass exists.

8. No critical/high authority defects remain.

Otherwise HOLD.
```

That is a much more intellectually honest gate.

---

# F-2 needs the same treatment

The review calls the entire `service/workflow` package dead code. 

I agree it is unused, but I don't think that means we should suddenly wire it into Phase 4.

Instead:

> **Workflow is future product infrastructure, not current security infrastructure.**

We have two good choices:

### Preferred

Remove the unused workflow implementation from the current MVP and retain only architecture documentation describing the future boundary.

That gives us:

```text
LESS CODE
LESS FALSE CONFIDENCE
LESS MAINTENANCE
CLEARER CURRENT SCOPE
```

### Alternative

Keep it in the repository but explicitly mark it:

```text
FUTURE / NOT MVP
NOT A SECURITY BOUNDARY
NOT ON PRODUCTION EXECUTION PATH
```

I prefer removal unless Phase 4 actually needs workflow continuity.

---

# F-3 and F-4 should be downgraded

The reviewer identifies:

```text
operator-review CLI → IntentOnPromoted directly
MCP authorize_action → PrepareForAction conditionally
```

as medium findings. 

These deserve cleanup, but **they are not reasons to invent an execution architecture**.

The stronger policy for this stage should be:

```text
Consequential EXECUTION
→ must use ExecuteAction

Intent creation
→ may have a narrower path, but must not execute anything
```

For the MCP tool specifically, I would still fix the conditional behavior if `target_id` / `actor_id` are part of the intended request contract. A request that cannot establish the tuple should fail rather than silently create an intent that bypasses preparation.

That gives us a cleaner invariant:

```text
incomplete authorization context
→ reject request
```

rather than:

```text
incomplete authorization context
→ silently skip authority preparation
```

---

# What I would do next

**Do not proceed to Phase 4 yet.**

First make a small scope correction:

```text
CURRENT MVP
    = authority + evidence + intent + audit + operational review

FUTURE EXECUTION
    = ExecuteAction + real Executor + real external integration
```

Then perform one final **scope-aware adversarial review** to verify:

```text
existing production consequential actions
→ all authority-gated

no existing execution bypass
→ confirmed

no dead code presented as active security boundary
→ confirmed

future execution path
→ canonical and tested

kernel remains unchanged
→ confirmed
```

After that, Phase 4 can legitimately start.

---

## Give the coding agent this prompt

```text
SOLVENT — SCOPE CORRECTION + FINAL SECURITY GATE

The latest adversarial review is correct that the current repository
contains NO real consequential production execution capability.

Do NOT invent a fake production executor.
Do NOT add a fake deployment system.
Do NOT create a fake external side effect merely to satisfy a gate.

The current product is:

    evidence
      ↓
    belief
      ↓
    promotion
      ↓
    authority
      ↓
    intent

There is currently no real:

    intent
      ↓
    external consequential execution

The ExecuteAction boundary is a FUTURE execution boundary that has
already been proven with a test-only recording executor.

==================================================
1. CORRECT THE SECURITY GATE SEMANTICS
==================================================

The requirement:

    "every consequential production execution path uses ExecuteAction"

applies when consequential production execution exists.

If no consequential production execution capability exists, that
condition is satisfied by explicitly documenting:

    "No consequential production execution capability currently exists.
     Future consequential execution MUST use ExecuteAction."

Do NOT create fake production execution to satisfy the condition.

The current GO criteria are:

- every EXISTING consequential production path is authority-gated
- no existing production bypass exists
- no real consequential production execution currently exists
  unless explicitly documented
- future execution boundary is canonical and tested
- kernel.Authorize remains final authority oracle
- no second authority engine exists
- no critical/high authority defects remain

==================================================
2. CORRECT CURRENT/FUTURE ARCHITECTURE DOCUMENTATION
==================================================

Document the distinction:

CURRENT:

    evidence
      ↓
    belief
      ↓
    authority
      ↓
    intent

FUTURE:

    intent
      ↓
    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    Executor
      ↓
    external provider

Do not represent ExecuteAction as an active production execution
boundary when no production caller exists.

Represent it as the canonical FUTURE execution boundary.

==================================================
3. CLEAN DEAD SECURITY BOUNDARIES
==================================================

Review:

    service/workflow
    ExecuteAction
    service/executor

For each determine:

    CURRENT PRODUCTION
    FUTURE / NOT MVP
    TEST-ONLY
    REMOVE

Do not wire future components merely to eliminate dead-code findings.

If a component is retained:

    explicitly label it FUTURE / NOT MVP

and ensure documentation does not represent it as a current
production security control.

If service/workflow has no current product requirement, prefer removing
unused implementation rather than adding artificial production wiring.

==================================================
4. PRESERVE EXECUTER BOUNDARY
==================================================

If ExecuteAction remains:

    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    internally resolved Executor

must remain the canonical future execution path.

Do NOT add:

    caller-supplied executorFn
    fake production executor
    fake deployment provider

==================================================
5. MCP INTENT CREATION
==================================================

Review handleSolventAuthorizeAction.

If target_id and actor_id are required to construct the intended
authorization tuple, missing values should cause the request to fail
closed rather than silently skipping PrepareForAction.

Do not allow:

    incomplete authorization context
      ↓
    silently create intent

The operation must either:

    perform the intended preparation/verification

or:

    reject due to insufficient authorization context.

Intent creation must remain distinct from execution.

==================================================
6. OPERATOR-REVIEW CLI
==================================================

Review cmd/operator-review.

Determine whether it is:

    trusted administrative tooling
    or
    part of the production application security boundary.

If it is administrative tooling, document the trust boundary clearly.

Do not create unnecessary production architecture just to make the
CLI use every service.

If it creates a consequential external execution in the future, it MUST
use the canonical ExecuteAction path.

==================================================
7. AUDIT CURRENT SCOPE HONESTLY
==================================================

Do not claim execution audit events for execution that does not exist.

Current audit may record:

    evidence
    refusal
    promotion
    authority
    intent

Future execution audit will record:

    authorization
    executor invocation
    provider response
    executor result

Do not fabricate execution events.

==================================================
8. FINAL ADVERSARIAL REVIEW
==================================================

After the scope cleanup, perform a FINAL targeted adversarial review.

Search for:

    all direct external side effects
    all executor invocation sites
    all consequential provider calls
    all direct IntentOnPromoted calls
    all direct kernel.Store calls
    all ExecuteAction callers
    all kernel.Authorize callers

Classify:

    production consequential
    production read-only
    administrative CLI
    test-only
    future
    dead
    bypass

The crucial question is:

    Does any EXISTING production consequential external action
    occur without current kernel authority?

If no real consequential production action exists:

    explicitly state that fact.

Do NOT convert absence of execution into a reason to invent execution.

==================================================
9. TEST THE FUTURE EXECUTION BOUNDARY
==================================================

Keep the live CockroachDB tests.

Verify:

    no authority
      → DENIED
      → executor not called

    valid authority
      → ALLOWED
      → recording executor called

    revocation after preparation
      → DENIED
      → executor not called

    target mutation
      → DENIED
      → executor not called

    action mutation
      → DENIED
      → executor not called

These prove the future execution boundary.

==================================================
10. FINAL VERDICT
==================================================

End with exactly one:

GO

only if:

- no existing consequential production execution bypass exists
- current authority checks are correct
- tested future ExecuteAction boundary is correct
- kernel.Authorize remains final authority oracle
- policy cannot manufacture authority
- workflow cannot manufacture authority
- tokens cannot manufacture authority
- actor spoofing fails
- no dead code is falsely represented as an active security boundary
- no unexplained kernel/schema growth exists
- current absence of production execution is explicitly documented

OR:

HOLD

if:

- an existing production consequential action bypasses authority
- a real consequential execution path exists and bypasses ExecuteAction
- the future boundary is incorrect
- a critical/high security defect remains

Do NOT mark HOLD merely because the product does not yet have a real
external executor.

Do NOT invent capabilities to make the architecture look complete.

==================================================
11. PHASE 4 GATE
==================================================

If the final review returns GO:

    freeze the security baseline

then proceed to:

    PHASE 4 — WEB UI

Phase 4 must not create a parallel authorization mechanism.

The UI must consume the established authority/service APIs and expose:

    current evidence
    current authority
    exact actor
    exact action
    exact target
    allow/deny reason
    audit/refusal history

Security remains in the service/kernel path, not the browser.
```

### My recommendation

I would **not chase this HOLD by adding production execution**.

That would be architecture driven by the review instead of product reality.

The right correction is to make the plan explicitly say:

> **There is no production consequential execution yet. That is a product-scope limitation, not a security failure. When execution is introduced, ExecuteAction is the mandatory boundary.**

Then clean up or clearly label the dead workflow/executor code and run the final targeted review once more.

That gives us a much stronger foundation for Phase 4 than manufacturing a fake execution system simply to satisfy a checkbox.
