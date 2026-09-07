I reviewed the revised Phase 4D plan. This version is **much stronger and is close to implementation-ready**, but I would make **three final corrections before letting the coding agent implement it**.

The good news is that the two major conceptual problems from the previous version are now fixed:

* `executing` survives retraction rather than being cancelled. 
* Definitive provider rejection rolls back to `live`, while ambiguous failure remains `executing`. 

That is the right model.

## 1. `ClaimIntent` + pre-claim read needs tightening

The plan now has:

```text
PrepareForAction
→ Resolve executor
→ Verify exact intent is live
→ reconstruct params
→ validate params
→ ClaimIntent
→ provider
```

The pre-claim `SELECT state ...` is explicitly described as informational. 

That's fine, but the plan should explicitly state:

> **The pre-claim read has no authority and no correctness significance. `ClaimIntent` is the only gate that decides whether execution ownership was acquired.**

Otherwise someone could accidentally make the read control behavior later.

I'd actually prefer:

```text
Authorize
→ resolve executor
→ reconstruct/validate parameters
→ ClaimIntent CAS
→ provider
```

The pre-read is unnecessary unless it provides a useful user-facing error distinction. The CAS already tells you whether the intent is live.

This is especially important because Phase 4D's new invariant is about **atomic ownership**, and the authoritative operation is the CAS. 

## 2. Provider error classification is still underspecified

This is the most important remaining design gap.

The plan says:

```text
4xx except 408 → definitive
408/5xx/network → ambiguous
```

and says:

> "The adapter/provider layer provides classification hints ... The service layer maps these to kernel transitions." 

But the actual contract for those hints isn't defined.

You need a concrete abstraction, something like:

```go
type ProviderError interface {
    error
    Outcome() ProviderOutcome
}

type ProviderOutcome int

const (
    ProviderRejected ProviderOutcome = iota
    ProviderAmbiguous
)
```

or an equivalent typed/sentinel design.

Otherwise the service ends up inspecting GitHub-specific errors such as HTTP status codes, which starts leaking provider semantics upward.

The architectural rule should be:

```text
GitHub adapter
    ↓
classifies provider outcome
    ↓
generic provider outcome
    ↓
service chooses kernel transition
```

not:

```text
service
    ↓
"if GitHub status == 408..."
```

The plan correctly says provider-specific semantics belong outside the kernel; make the service/provider interface precise enough to preserve that. 

## 3. `ReconcileIntent` needs an atomic/authorization contract

The plan says `ReconcileIntent` is privileged and requires an authenticated `operatorID`, which is good. It also says the operation itself **trusts the caller's external verification**. 

But the state-transition contract needs to be more explicit.

For example:

```text
executing → executed
executing → live
executing → cancelled
```

must only be possible from `executing`.

And reconciliation should not silently turn an arbitrary current state into the requested outcome.

You want:

```text
ReconcileIntent(
    scenarioID,
    intentID,
    outcome,
    operatorID,
)
```

to fail if the intent is not currently `executing`.

Otherwise an operator could accidentally use reconciliation as a generic lifecycle override.

The kernel transitions should remain authoritative:

```text
executing → executed
executing → live
```

with predicates enforcing the source state.

The plan already has the right basic idea; it just needs the contract stated explicitly.

---

# One thing I strongly approve

The revised `RetractCascade` decision is now correct:

```text
live       → cancelled
executing  → executing
```

This preserves the execution-attempt marker even after authority retraction. 

That is much safer than the earlier proposal to cancel `executing`.

The new invariant:

> **Once an intent enters `executing`, retraction cannot erase the execution-attempt record.**

is exactly the kind of durable fact Phase 4D needs. 

---

# The API/MCP change is also right

Reading consequence parameters from the approved target snapshot instead of accepting caller-supplied parameters is the correct decision. 

That preserves:

```text
Approved snapshot
        ↓
authorization
        ↓
execution
```

rather than:

```text
Approved snapshot
        ↓
caller rewrites tuple
        ↓
authorization
```

However, make sure the API's snapshot read is not treated as a replacement for `kernel.Authorize`. It is merely supplying the exact tuple data to the existing authority check.

---

# I would also slightly tighten the Phase 4D objective

The current wording:

> "at most one accepted external execution request under Solvent-controlled concurrency" 

is good, but keep the boundary explicit:

```text
At most one provider attempt can be claimed concurrently
for a given Solvent intent.

This does not guarantee exactly one external side effect
across crashes or ambiguous provider outcomes.
```

The plan already explains that distinction well. 

---

## Verdict

**Plan: GO after three small revisions.**

Do not implement yet. Give the coding agent this final instruction:

```text
Revise the Phase 4D plan once more before implementation.

1. Make ClaimIntent the sole authoritative ownership gate.
   The pre-claim intent-state read is informational only. Prefer removing it
   unless it has a concrete user-facing purpose.

2. Define a concrete generic provider-outcome classification contract.
   The service must NOT inspect GitHub-specific HTTP semantics directly.
   The adapter should classify errors into at least:
       DEFINITIVE_REJECTION
       AMBIGUOUS
   and the service maps those generic outcomes to:
       executing -> live
       executing -> executing

3. Strengthen ReconcileIntent semantics:
   - only operates on an intent currently in executing state
   - source-state predicate must be enforced atomically
   - outcome transitions must be explicit:
       executing -> executed
       executing -> live
       executing -> cancelled
   - operator identity is authenticated/audited
   - do not expose through generic REST/MCP in Phase 4D

4. Keep the rest of the plan unchanged:
   - live -> executing via ClaimIntent CAS
   - executing survives RetractCascade
   - definitive rejection -> live
   - ambiguous outcome -> executing
   - no automatic retry for ambiguous state
   - snapshot remains sole consequence-parameter source
   - configurable HTTP timeout
   - no provider-specific semantics in kernel
   - no new generic orchestration framework

5. Update the state machine, transition table, failure matrix, interfaces,
   acceptance tests, and security invariants to reflect these clarifications.

STOP after revising the plan. Do not implement code.
```

After those corrections, I would consider **Phase 4D architecture locked and ready for implementation**. The revised plan is now consistent with the core Solvent philosophy: add durable state only where it creates a real security/concurrency invariant, while keeping GitHub/provider semantics outside the kernel. 
