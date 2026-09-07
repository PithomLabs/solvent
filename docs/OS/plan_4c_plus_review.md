I would **not approve this plan for implementation yet**. The test-engineering structure is strong, but there are several issues that matter before we let OpenCode touch the kernel or introduce a real external side effect.

My disposition is:

> **GO WITH CHANGES**

The good news is that the plan is substantially better than a typical executor plan because it explicitly establishes the test target before implementation, defines security/correctness invariants, builds a deterministic provider oracle, and requires Kilo Code to review the target before implementation. 

## 1. The biggest unresolved issue: duplicate execution is not actually defined

The test-engineering requirement was:

> define the target rigorously and robustly.

But T-10 currently says:

> `1 or 2 calls` / `Depends`. 

That is not a sufficiently defined target.

The plan simultaneously says:

* duplicate execution is an ambiguity;
* no idempotency exists;
* a future phase will add idempotency;
* the first real executor is consequential. 

Those can coexist, but the test specification needs a precise statement of what Solvent **guarantees now**.

For example:

```text
Sequential duplicate:
    after first provider acceptance + CompleteIntent,
    second execution MUST NOT reach provider.

Concurrent duplicate:
    NOT GUARANTEED in 4C+
    and explicitly classified as a known v1 limitation.
```

Or, if concurrent duplicates must be prevented, the plan needs an actual claim/serialization mechanism.

Do not leave the test as "1 or 2 calls."

---

## 2. `CompleteIntent` needs a much harder architectural justification

This is the most important kernel question.

The plan proposes:

```text
live
  ↓
executed
```

and says this belongs in the kernel because it is an atomic security-critical transition. 

I'm not yet convinced by that justification.

`executed` is **not itself authority**. It is a statement about execution outcome. More importantly, the kernel has no direct knowledge that GitHub actually accepted anything.

The actual chain is:

```text
GitHub says accepted
       ↓
service trusts provider response
       ↓
CompleteIntent(...)
       ↓
kernel records "executed"
```

So `CompleteIntent` is really a durable recording transition based on an **external attestation supplied by the service**.

That may still be a legitimate kernel transition because `action_intent` is a kernel-owned state machine, but the plan needs to answer:

> Why must the kernel own `live → executed`, rather than the service/audit layer recording an execution result externally?

And more importantly:

> **What prevents arbitrary internal code from calling `CompleteIntent(intentID)` and manufacturing an execution fact?**

The current plan only says:

```go
UPDATE action_intent
SET state='executed'
WHERE id=$1 AND state='live'
```



That means the method as described is not cryptographically or semantically bound to provider acceptance. The security boundary depends entirely on trusted callers.

I would require the plan to explicitly establish:

```text
CompleteIntent is not an authority grant.
CompleteIntent is not exposed through API/MCP.
CompleteIntent is callable only by the trusted execution service path.
Only a provider-accepted execution may cause that call.
```

Then Kilo can decide whether the minimal kernel method is justified.

---

## 3. The GitHub API description is now factually incomplete

This needs correction before implementation.

The plan says the workflow-dispatch request is synchronous and that:

> “Provider returns a workflow run ID.” 

Current GitHub REST documentation says the workflow-dispatch endpoint can return:

* **204 No Content** when `return_run_details=false`
* **200** with `workflow_run_id`, `run_url`, and `html_url` when `return_run_details=true`. ([GitHub Docs][1])

So the real executor plan should explicitly choose:

```text
POST .../dispatches?return_run_details=true
```

and make the returned run ID the provider reference.

Otherwise the plan's claimed authoritative execution reference does not line up with the actual provider contract.

This is not cosmetic: the audit design explicitly depends on that provider reference. 

---

## 4. The actual first integration boundary must be made explicit

The executor is registered in:

```text
cmd/solvent-mcp/main.go
```

only. 

That means the first real consequential execution path appears to be:

```text
MCP
 ↓
service/authority.ExecuteAction
 ↓
GitHub executor
 ↓
GitHub
```

not:

```text
REST / generic application
 ↓
execution API
```

That is acceptable for a first integration, but the plan needs to explicitly state:

> **Phase 4C+ uses the trusted local MCP execution surface as its execution entry point. It does not yet establish a general remote execution API.**

Otherwise the project may accidentally conclude that Solvent has already become a general execution/authorization layer for arbitrary apps and agents.

This is particularly important because the current MCP identity model is deliberately a **trusted local administrative surface**, as established in Phase 4C. The plan needs to preserve that trust assumption rather than silently turning MCP into a generic remote-agent boundary.

---

## 5. The "same goroutine" claim is too strong

The plan says:

```text
T1 authorization
 ↓
T2 executor invoked (same goroutine, same process)
 ↓
T3 GitHub
```



"Same goroutine" is an implementation detail, not a security guarantee.

The meaningful invariant is:

```text
current authorization check
        ↓
trusted executor selection
        ↓
executor invocation
```

not whether Go happens to execute it on the same goroutine.

I'd remove that language from the security model.

---

## 6. The parameter-binding proof needs to be stronger

The plan correctly identifies the existing vulnerability:

```go
toolName, _ := params["tool_name"].(string)
execReg.Get(toolName)
```

and proposes deriving executor name from the authorized action instead. 

Good.

But SI-7 says:

> exact authorized action/target must be the action sent to GitHub. 

The plan currently says the executor will simply validate:

```text
repo
workflow
ref
```



That's not enough by itself.

The test target needs to establish the provenance of those values:

```text
authorized target
     ↓
execution parameters
     ↓
GitHub request
```

For example:

```text
authorized repository
    MUST determine GitHub repo

authorized ref/scope
    MUST determine GitHub ref

authorized consequence parameters
    MUST determine workflow/environment

caller-supplied arbitrary repo
    MUST NOT override authorized repo
```

Otherwise you can have a perfectly authorized `"deploy"` action that ends up deploying the **wrong repository**.

That would violate SI-7 while all the high-level authorization tests still pass.

---

## 7. The test engineering target should distinguish "provider call" from "provider effect"

The fake provider is good because it gives you a side-effect oracle. 

But some test rows still conflate:

```text provider was called
```

with:

```text provider accepted
```

and:

```text external workflow actually began
```

The plan itself correctly recognizes these as different facts. 

Make that distinction explicit in the test oracle:

```text
executor invocation
provider request received
provider accepted request
provider run created
provider run completed
```

For Phase 4C+ you may stop at:

```text provider accepted + run reference
```

but the target needs to say that clearly.

---

## 8. Failure tests need a small correction

This row is too broad:

> Executor returns error → no provider call. 

An executor can return an error **after it has called GitHub**.

So split it:

```text
executor internal validation failure
    → zero provider calls

provider 4xx/5xx
    → one provider call
    → executor_failed

network error before request transmission
    → zero or provider-unknown depending on transport semantics

response lost after request transmission
    → one provider call
    → ambiguous
```

The plan already recognizes ambiguity later, but the matrix should make the cases mutually precise.

---

## 9. Real integration testing must be isolated from ordinary `go test ./...`

The plan proposes:

```bash
GITHUB_TOKEN=... go test ... -run TestRealGitHub
```

which is reasonable, but it should explicitly ensure that the normal suite **never performs a real GitHub side effect simply because `GITHUB_TOKEN` happens to exist**.

I'd require:

```text
unit/fake-provider tests
    → normal go test ./...

real GitHub integration
    → explicit command / integration build tag / explicit opt-in
```

No accidental external mutation from a developer's environment.

---

## 10. The scope contradiction should be cleaned up

The plan says:

> "Schema migrations (except one additive column/constraint)" 

but later the actual kernel change is described as an `UPDATE` to an existing state column, with no schema change at all. 

Change the scope line to:

> **No schema migrations.**

Unless reconnaissance later proves a schema modification is actually required.

---

# What I would approve after these corrections

The overall shape is very good:

```text
STEP 0
Test target + invariants
        ↓
fake provider
        ↓
executor tests
        ↓
fix executor selection
        ↓
minimal CompleteIntent decision
        ↓
real GitHub provider
        ↓
safe integration
        ↓
full verification
        ↓
Kilo Code
```

The plan's test matrix is already far better than the normal "write an executor and add a few happy-path tests" approach. 

## My required changes before implementation

```text
1. Define duplicate-execution semantics precisely.
2. Re-justify CompleteIntent as a kernel primitive and constrain its caller.
3. Correct GitHub workflow_dispatch behavior:
   use return_run_details=true if run ID is required.
4. Explicitly state that 4C+ execution enters through the trusted MCP
   surface and is not yet a generic remote execution API.
5. Remove "same goroutine" from the security guarantee.
6. Add explicit action/target → executor-parameter provenance tests.
7. Split executor-internal failure from provider-call failure.
8. Ensure real GitHub tests are explicit opt-in and cannot run accidentally
   in the normal suite.
9. Change "schema migrations except..." to simply "no schema migrations."
```

And I would preserve one particularly important requirement from the plan:

> **Kilo Code reviews `phase4c_plus_test_target.md` before production implementation begins.** 

That is exactly the test-engineering discipline we wanted: **the target gets independently challenged before the code exists.**

Once those points are corrected, I would be comfortable approving the plan to move into **Step 0 test engineering**, with Kilo Code reviewing the target before OpenCode implements the executor.

[1]: https://docs.github.com/en/rest/actions/workflows?utm_source=chatgpt.com "REST API endpoints for workflows - GitHub Docs"



## claude

Reviewing this as a plan on its own reasoning, not against code that doesn't exist yet — a full fresh pass, since this is worth doing properly rather than just restating my earlier note.

## CRITICAL — The executor's domain-specific parameters (repo/workflow/ref) are never shown to be bound to what was actually approved

This is a bigger problem than the one I raised last time, and it comes directly from something Phase 4C itself established. M-6 in the Phase 4C contract-reconciliation table states the authorize-action handler hardcodes `consequence_parameters={}` — an empty map — as part of the tuple that `kernel.Authorize`/`AuthorizeAndCreateIntent` actually checks. That's correct and appropriate for a domain-agnostic kernel; Solvent's authority core shouldn't know what a "repo" or "workflow" is.

But it means the tuple the kernel verifies contains *nothing* about which GitHub repository, workflow, or ref is being targeted. So when §16's `NewWorkflowTriggerExecutor` reads `params["repo"]`, `params["workflow"]`, `params["ref"]` and hands them straight to `provider.TriggerWorkflow`, those values have to come from somewhere — and this document never says where, or how they're bound to what the human actually approved. Two possibilities, and the plan doesn't distinguish which one it's building:

- If those values are reconstructed server-side from something stored on the target/snapshot at approval time, that's the correct design — but nothing in §16, §18, or the file map (`adapter/github/executor.go`) describes that reconstruction, and there's no test for "params supplied at execution time differ from what's stored on the approved snapshot, and execution is refused."
- If those values are supplied by the caller alongside the authorize/execute request — which is the more natural reading of `ActionFunc`'s `params map[string]interface{}` signature and how `ExecuteAction` is described elsewhere in this document — then a target approved for "deploy to `repo-a`, workflow `deploy.yml`" could be executed against `repo-b`, workflow `wipe-everything.yml`, with the *same* `target_id` and the *same* authorized `action` string, and the kernel would authorize it, because none of that ever entered the tuple it checks.

SI-7 states the requirement plainly — *"the exact authorized action/target must be the action sent to GitHub"* — but nothing in this document demonstrates a mechanism that makes that true. AT-2 and AT-3 test for changing the *target ID* or *action* after authorization; neither tests for holding target ID and action constant while substituting the domain-specific parameters underneath them, which is the actual shape of this gap. That's a testing blind spot that follows directly from the design gap, not a separate oversight.

This needs an explicit answer before implementation starts: where do `repo`/`workflow`/`ref` get stored at target-creation/approval time, and what, concretely, checks the values handed to the executor against that stored record at execution time. If the answer is "nothing does," this is a confused-deputy hole that doesn't require winning a race to exploit — it works every time, by construction.

## CRITICAL — The execution-time re-authorization check still rests on the exact assumption Phase 6.4 disproved, and it now shows up in three places, not one

Restating this because it's still unaddressed and it's now visibly load-bearing across the whole document, not an isolated line: F-6 in the failure model, §10's TOCTOU model, and §18's authorization-to-execution boundary all independently assert that a plain re-read via `kernel.Authorize` — "SERIALIZABLE," no `FOR UPDATE` — closes the gap between authorization and executor invocation, on the reasoning that it's "the same method," "same goroutine," happening "at T2." That's the identical shape of reasoning behind the original `AuthorizeAndCreateIntent`, before Phase 6.4 found that CockroachDB doesn't reliably treat a `NOT EXISTS` predicate read as conflicting with a later insert satisfying it. The fix that actually worked was a shared row lock forcing real serialization; nothing in this document proposes the equivalent for the execution-time check, and the fact that this same unexamined assumption is repeated in three separate sections suggests it was carried forward as settled fact rather than re-examined against what this project just learned about itself one phase earlier. This is strictly worse in consequence than the intent-creation case, since the thing that could race ahead on stale authority here is a live GitHub Actions dispatch, not a database row.

## What's genuinely solid here

The test-engineering-first sequencing (spec before code, Kilo Code review before merge) is the right discipline and it's followed consistently. The failure model (F-1 through F-21) is thorough and correctly classified into Security/Correctness/Operational/Ambiguous, and it's honest about what's genuinely unresolved (F-14 through F-20 all correctly route to "requires operator reconciliation" rather than pretending a resolution exists). Credential handling in §17 is clean: token captured in a closure at construction, never in `params`, never logged, registration skipped rather than erroring when absent. `CompleteIntent`'s kernel-growth justification in §21 is well-reasoned and appropriately minimal. The four-fact distinction (authorization/invocation/provider-acceptance/provider-completion) and the T2→T3 boundary being accepted as genuinely inherent, rather than something to paper over, are both right.

## Net

Two things need real answers before this plan is ready to implement, not two things to note and proceed anyway: where the domain-specific execution parameters come from and how they're bound to the approved target, and how the execution-time re-check gets the same serialization guarantee Phase 6.4 established for intent creation. Both are design questions the plan can answer without writing code yet — this is exactly the kind of thing the test-engineering-gate step (§5) exists to force before Step 1 begins, and I'd send it back to that step rather than let it proceed into implementation with either question still open.


## chatgpt

Yes. I agree with this review. On the plan **as a design artifact**, these are two genuine blockers, not polish items.

The first is the more fundamental one: **the plan has an authorization target, but it has not yet defined an execution target.**

### 1. Domain-specific execution parameters are currently unbound

The plan says the approved action is effectively:

```text
principal
target_id
action = deploy
```

while the executor later consumes:

```text
repo
workflow
ref
```

Those are not represented in the Phase 4C `authorize-action` tuple because the Phase 4C shape intentionally uses server-derived generic dimensions and currently has empty `consequence_parameters`. 

Then the proposed executor does:

```go
repo, _ := params["repo"].(string)
workflow, _ := params["workflow"].(string)
ref, _ := params["ref"].(string)
```

and sends those values to GitHub. 

That creates exactly the hole the reviewer describes.

The plan must establish a chain like:

```text
approved target/snapshot
        ↓
canonical execution parameters
        ↓
execution-time validation
        ↓
GitHub request
```

For example:

```text
Approved target
    resource_id = org/repo-a
    scope       = branch:main
    action      = deploy
    consequence parameters
        workflow = deploy.yml
        ref      = main
        environment = production
             ↓
Execution request
             ↓
derived execution parameters
             ↓
GitHub
```

The critical invariant then becomes:

> **The executor may execute only the provider-specific parameters bound to the approved target; caller-supplied execution parameters may not redefine them.**

And the test target absolutely needs:

```text
same target_id
same action
different repo/workflow/ref
→ provider call MUST NOT occur
```

That is currently absent from T-02–T-16. 

This should be fixed **before Step 1**, exactly as the reviewer recommends.

---

### 2. The execution-time authorization claim is too strong

The reviewer is also right about the second issue, with one important refinement.

The plan currently claims:

> `PrepareForAction` re-reads current state at T2 and this “eliminates the T1→T2 TOCTOU.” 

That statement is too broad.

A second current-state authorization check can eliminate a stale **database decision that existed before the execution check**, but it does **not create an atomic boundary between the check and the provider call**.

There is still:

```text
T2:
    current authorization check succeeds
        ↓
T2.1:
    authorization state can change
        ↓
T2.2:
    executor invokes GitHub
```

The plan already correctly acknowledges that the DB cannot be atomically coordinated with GitHub. 

Therefore the real question is:

> **What guarantee do we require between the final Solvent authorization decision and the external side effect, and can the existing kernel provide it without pretending that DB locking extends into GitHub?**

That needs to be answered before implementation.

And this is precisely where the project's Phase 6.4 lesson matters: **we should not assume that another `kernel.Authorize()` read is sufficient merely because it is fresh.** The plan needs to determine whether the final execution authorization itself requires an atomic kernel transition such as:

```text
LIVE
  ↓
EXECUTION_CLAIMED
```

or some equivalent reservation/claim mechanism, rather than simply:

```text
Authorize()
  ↓
Execute()
```

Whether that belongs in the kernel is a real Kernel Growth Gate question. The current plan jumps directly to `CompleteIntent` without first proving that the pre-execution side of the boundary is sufficient. 

---

# I would send this back to Step 0

Do **not** let OpenCode proceed to implementation.

Use this instruction:

```text
RETURN TO STEP 0 — TEST ENGINEERING / TARGET DEFINITION.

The Phase 4C+ plan has two unresolved CRITICAL design gaps. Do not implement
production code until both are explicitly resolved in the test target and
architecture.

CRITICAL 1 — DOMAIN-SPECIFIC EXECUTION PARAMETER BINDING

The current plan authorizes a generic Solvent action:

    action = "deploy"

but the proposed GitHub executor consumes:

    repo
    workflow
    ref

The Phase 4C authorize-action tuple currently contains no GitHub-specific
consequence parameters, and the plan does not establish where these values
come from or how they are bound to the approved target.

You MUST resolve this before implementation.

Explicitly define:

    1. Where repo/workflow/ref are stored or derived at target creation/
       approval time.

    2. How they become the canonical execution parameters.

    3. Whether they are stored in the existing target/snapshot/intent
       representation or derived from another authoritative record.

    4. How execution-time values are compared against the approved values.

    5. How caller-supplied parameters are prevented from overriding the
       approved values.

    6. Which component owns this binding.

    7. Whether the kernel needs to know these values or whether they remain
       provider-specific data above the kernel.

The preferred architecture is:

    approved generic Solvent target
            ↓
    authoritative provider-specific execution binding
            ↓
    execution-time validation
            ↓
    GitHub executor
            ↓
    GitHub

Do NOT put GitHub-specific semantics into the kernel merely to make this
easier.

Add explicit tests:

    same target_id
    same authorized action
    substitute repo
        → zero provider calls

    same target_id
    same authorized action
    substitute workflow
        → zero provider calls

    same target_id
    same authorized action
    substitute ref
        → zero provider calls

    same target_id
    same authorized action
    substitute any other execution parameter
        → zero provider calls

These tests MUST fail if the implementation trusts caller-supplied
provider parameters.

CRITICAL 2 — EXECUTION-TIME AUTHORIZATION / TOCTOU

Remove the claim that a second plain kernel.Authorize() "eliminates the
T1→T2 TOCTOU."

That is too strong.

The plan must explicitly model:

    final authorization decision
        ↓
    executor selection
        ↓
    executor invocation
        ↓
    GitHub request

and identify the interval in which authority may change.

Use the lessons learned in Phase 6.3/6.4:

    Do not assume a fresh predicate read is sufficient simply because the
    transaction uses SERIALIZABLE.

Determine whether the execution boundary requires an atomic kernel state
transition such as a claim/reservation, or whether the existing intent
state plus current authorization can provide the required guarantee.

If a new kernel primitive is proposed, apply the Kernel Growth Gate:

    - Is it a new durable security fact?
    - Is it a new atomic security-critical state transition?
    - Is it impossible to express safely above the kernel?

Do not approve a kernel change merely because it is convenient.

If no additional kernel primitive is necessary, prove why.

Update the test target to explicitly cover:

    authority revoked immediately before execution
    authority revoked concurrently with execution start
    target changed before execution
    binding changed before execution
    repeated execution attempt
    stale execution parameters
    stale authorization

For every scenario define:

    expected authorization
    expected executor invocation
    expected provider call count
    expected external side effect
    expected audit state

CRITICAL TEST REQUIREMENT:

The test suite must distinguish:

    current authorization
from
    provider invocation
from
    provider acceptance

Do not treat "Authorize() returned true" as proof that execution was safe.

==================================================
ALSO FIX THESE PLAN ISSUES
==================================================

1. Define duplicate execution semantics precisely.
   Do not leave T-10 as "1 or 2 calls."

2. Remove "same goroutine" from the security model.

3. Split executor-internal failure from provider-call failure.

4. Make the real GitHub integration explicitly opt-in so normal `go test`
   cannot accidentally cause external side effects.

5. Change "schema migrations except one additive column/constraint" to
   "no schema migrations" unless the revised design proves one necessary.

6. Verify the actual GitHub workflow-dispatch response model before relying
   on a workflow run ID as the provider reference.

7. Preserve the distinction:

       authorization
       invocation
       provider acceptance
       provider completion

8. Do not expand ActionFunc or create a generalized executor framework
   unless the revised design proves it necessary.

==================================================
MANDATORY TEST-ENGINEERING DELIVERABLE
==================================================

Before any production implementation:

    docs/OS/phase4c_plus_test_target.md

must explicitly contain:

    target behavior
    authoritative execution binding
    canonical state model
    security invariants
    correctness invariants
    TOCTOU model
    failure model
    duplicate-execution semantics
    provider side-effect oracle
    deterministic fake-provider model
    complete test matrix

Kilo Code must review this test target BEFORE OpenCode begins production
implementation.

Do not proceed to Step 1 until Kilo Code approves the target.

==================================================
IMPLEMENTATION GATE
==================================================

The state machine is now:

    revise test target
        ↓
    Kilo Code review
        ↓
    Kilo GO
        ↓
    OpenCode implements
        ↓
    OpenCode verifies
        ↓
    Kilo independently reviews implementation
        ↓
    GO

Do not bypass the first Kilo review.

The purpose of this phase is not to make a GitHub API call.

The purpose is to prove:

    Solvent authority
        ↓
    binds the EXACT consequential operation
        ↓
    and only that operation
        ↓
    may reach the external provider

while keeping GitHub semantics outside the kernel.
```

## Why I would stop here

The reviewer has identified the exact point where **"authorization layer" could accidentally become "authorization of a label while the executor chooses the real operation."**

That is unacceptable.

The first real executor has to establish:

```text
"deploy"
+
approved repository
+
approved workflow
+
approved ref
+
approved consequence
        ↓
EXACTLY THIS EXTERNAL OPERATION
```

Only after that binding is formally defined and tested should we let OpenCode implement the executor.

The second issue is equally important because Phase 6.4 already taught us not to treat a database predicate read as stronger than it is. The execution path needs an equally rigorous guarantee rather than inheriting the phrase "re-read current state" as a security proof.

So I would **reject the current Phase 4C+ plan for implementation, return it to Step 0, have OpenCode revise the test target and execution-boundary design, then have Kilo Code review that target before any production code is written.**

That's exactly what the test-engineering gate is for.


