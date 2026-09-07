I reviewed the Phase 4D plan. It is **not ready to implement exactly as written**. The core direction is right, but there are several important state-machine issues that should be resolved before the coding agent touches the repository.

The biggest one is more serious than the plan currently acknowledges.

## 1. Do not let `RetractCascade` cancel `executing` intents

The plan currently proposes:

```text
executing → cancelled
```

when a belief is retracted. 

I would reject that.

`executing` has a very specific purpose: it means **Solvent has claimed this intent and an external side effect may already have happened**.

Once that is true, cancellation does not erase reality.

Consider:

```text
ClaimIntent
    ↓
executing
    ↓
GitHub accepts deployment
    ↓
human retracts belief
    ↓
RetractCascade
    ↓
cancelled
```

Now Solvent has erased the durable marker that an external execution may have occurred.

That's particularly dangerous because your own plan says an `executing` intent after a crash is ambiguous and must not automatically be retried. 

So the better rule is:

```text
RetractCascade:
    live       → cancelled
    executing  → executing
```

The retraction should prevent **future execution**, but it should not destroy the evidence that execution is already in progress or may already have happened.

This preserves the reconciliation invariant:

> **Once an intent enters `executing`, it remains an execution-attempt record until its external outcome is resolved.**

That is stronger and more internally consistent than the current CI-7 proposal.

---

## 2. There is a more serious bug: not every executor error should roll back to `live`

The plan currently says:

```text
executing → live
```

on executor error. 

And the service flow says:

```text
provider error
    ↓
RollbackClaim
    ↓
live
```



That is unsafe.

You already correctly recognize three different realities:

```text
definitely rejected
definitely not invoked
ambiguous
```

But the state transition proposed here treats all executor errors alike.

Imagine:

```text
ClaimIntent
    ↓
executing
    ↓
GitHub receives request
    ↓
GitHub starts workflow
    ↓
network connection dies
    ↓
Solvent receives timeout/error
```

From Solvent's perspective, that's an error.

But the external side effect may already have happened.

If you do:

```text
executing → live
```

then a retry can produce:

```text
first GitHub workflow
+
second GitHub workflow
```

That directly violates the intended safety property.

### Required distinction

You need at least:

```text
DEFINITIVE FAILURE
    executing → live
```

versus:

```text
AMBIGUOUS FAILURE
    executing → executing
```

The latter requires reconciliation before retry.

This follows directly from the plan's own crash model, which correctly says ambiguous execution must not be retried automatically. 

So the Phase 4D state machine needs to distinguish **provider rejection** from **provider outcome unknown**.

---

## 3. `ClaimIntent` happens too early relative to local failures

The proposed flow is:

```text
Authorize
→ ClaimIntent
→ reconstruct params
→ audit
→ provider
```



That creates another issue.

Suppose:

```text
ClaimIntent succeeds
    ↓
snapshot JSON is malformed
```

or:

```text
ClaimIntent succeeds
    ↓
local parameter validation fails
```

No provider call has occurred, yet the intent is now `executing`.

The plan currently doesn't clearly define how these **pre-provider failures** are handled.

You don't want every ordinary local failure to require operator reconciliation.

I would structure it as:

```text
Authorize
    ↓
prepare/validate everything that can fail locally
    ↓
ClaimIntent
    ↓
adapter invocation
    ↓
provider
```

Anything that can be deterministically established as “provider was never invoked” should happen before the durable claim whenever practical.

That makes the meaning of `executing` much cleaner:

> `executing` means the system has passed all local preparation and has committed to attempting the external side effect.

---

## 4. `ReconcileIntent` needs a much stronger trust boundary

The plan proposes:

```go
ReconcileIntent(..., outcome IntentOutcome)
```

where the caller supplies:

```text
Completed
Failed
Cancelled
```



This is potentially dangerous.

A caller saying:

> “Trust me, GitHub completed.”

must not itself become authority.

Otherwise reconciliation becomes an extremely powerful bypass:

```text
executing
    ↓
caller says Completed
    ↓
executed
```

I would make reconciliation explicitly an **operator/admin execution-control operation**, with authenticated identity and mandatory audit.

More importantly, the plan should define who is permitted to reconcile and what evidence is required.

For Phase 4D, I would strongly consider keeping reconciliation **internal/service-level only**, rather than exposing it through generic REST/MCP yet.

---

## 5. The API/MCP snapshot approach is good

This part is correct:

```text
target snapshot
    ↓
REST/MCP authorization
    ↓
kernel.Authorize
```

rather than allowing the caller to supply a replacement tuple. 

That preserves:

> **approved snapshot is the source of truth**

and fixes the functional gap identified in the Phase 4C+ review.

The only thing I'd require is that the snapshot lookup must verify the same active target/activation semantics already relied upon by the kernel. Don't introduce a new interpretation of "approved target" in API code.

---

## 6. `ClaimIntent` is justified—but keep the kernel primitive minimal

I agree with adding `ClaimIntent`.

This is one of the rare cases that passes the kernel test:

```text
durable state transition
+
atomic CAS
+
security-relevant concurrency property
```

The important part is to avoid turning the kernel into an execution state machine.

The kernel should know:

```text
live → executing
executing → executed
executing → live
```

as durable lifecycle facts.

It should **not** know:

```text
GitHub timeout
GitHub workflow status
provider reconciliation protocol
retry backoff
```

Those remain service/provider concerns.

---

# The state machine I recommend

Before implementation, I would revise the design to this:

```text
                  provider definitively rejects
                 ┌───────────────────────────────┐
                 │                               ▼
live ──Claim──> executing ────────────────> live
                  │
                  │ provider accepts
                  ▼
               executed

executing
   │
   │ ambiguous provider outcome / crash
   ▼
executing  ←── remains here until reconciliation
```

And separately:

```text
live ──RetractCascade──> cancelled
```

but **not**:

```text
executing ──RetractCascade──> cancelled
```

That is the major architectural correction.

The resulting invariant becomes:

> **Once an intent is `executing`, retraction cannot erase the execution-attempt record.**

That fits the plan's own reconciliation philosophy much better.

---

# One more important nuance

The current plan's claimed property is:

> “at most one accepted external execution request under Solvent-controlled concurrency.” 

With the executing-state CAS, that is achievable for **concurrent Solvent attempts**.

It is *not* enough to guarantee exactly one external execution across crash/recovery, which the plan correctly acknowledges. 

So keep this distinction extremely explicit:

```text
Concurrency safety:
    Solvent prevents two live claims from simultaneously executing.

Crash ambiguity:
    Solvent cannot know whether an external request succeeded after losing
    the provider response.

Therefore:
    ambiguous intent is frozen until reconciliation.
```

That is a very defensible Phase 4D design.

## Decision to send the planning agent

```text
Revise the Phase 4D plan before implementation.

1. DO NOT cancel `executing` intents in RetractCascade.

   Keep:
       live → cancelled

   But preserve:
       executing → executing

   Retraction may prevent future execution, but must not erase the durable
   record that an external execution attempt is in progress or may already
   have happened.

2. Distinguish DEFINITIVE provider failure from AMBIGUOUS provider failure.

   Definitive rejection:
       executing → live

   Ambiguous timeout/lost response/network uncertainty:
       executing → executing

   Ambiguous execution must require reconciliation before retry.

3. Ensure all deterministic local preparation that can fail is performed
   before ClaimIntent where practical. Once ClaimIntent succeeds, `executing`
   should mean Solvent has passed local preparation and committed to the
   external execution attempt.

4. Strengthen ReconcileIntent trust semantics.

   Do not allow an arbitrary caller to declare an intent Completed/Failed.
   Define it as privileged execution reconciliation with authenticated,
   auditable operator/service authority. Prefer keeping it internal to
   service/operations for Phase 4D rather than exposing a generic public
   REST/MCP mutation.

5. Keep API/MCP consequence parameters sourced from the approved target
   snapshot. Do not add caller-supplied consequence parameters.

6. Keep ClaimIntent as a minimal kernel primitive. Do not add provider-specific
   semantics to the kernel.

7. Update the state-transition table, failure matrix, acceptance tests,
   adversarial tests, and security invariants to reflect these decisions.

8. STOP after updating the plan. Do not implement code yet.
```

I would make those changes **before implementation**. The current plan is otherwise on the right track, especially the durable `executing` state and atomic CAS approach. But the `executing → cancelled` and “all executor errors → live” rules could undermine the very duplicate-execution protection Phase 4D is supposed to provide.
