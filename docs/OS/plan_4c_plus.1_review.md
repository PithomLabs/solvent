This revision is **substantially better**. The two critical gaps have been addressed in the plan itself:

* Provider-specific execution parameters are now explicitly bound to `target_snapshot.consequence_parameters`, rather than being caller-controlled.  
* The test matrix now contains explicit parameter-substitution tests for repo/workflow/ref. 
* The plan now honestly acknowledges that re-authorization does **not** create atomicity with GitHub and that a T1→T2 window remains. 
* Concurrent duplicate execution is explicitly classified as a known v1 limitation rather than pretending it is solved. 
* The GitHub dispatch contract now uses `return_run_details=true` and explicitly defines the provider reference. 
* Real GitHub tests are explicitly opt-in and normal `go test ./...` must never cause real side effects. 

However, I would make **one important change before Step 0 goes to Kilo Code**.

## The remaining conceptual problem: `CompleteIntent` is still conflating provider acceptance with execution completion

The plan defines:

```text
PROVIDER_ACCEPTED
    ↓
EXECUTED
```

and says:

> `live → executed` only on provider acceptance. 

But the state name **`EXECUTED`** semantically implies that the external action actually executed. Yet the same plan explicitly says:

> provider acceptance does not prove provider completion. 

and:

> Solvent does not guarantee that GitHub completed the workflow. 

So:

```text
GitHub accepted workflow_dispatch
        ↓
action_intent.state = executed
```

creates a semantic contradiction.

What the kernel can honestly know at that moment is something closer to:

```text
EXECUTION_ACCEPTED
```

not:

```text
EXECUTED
```

This matters because `action_intent.state` is kernel-owned durable state. Calling that state `executed` risks making downstream systems believe a fact Solvent does not actually possess.

### I would not add another kernel state yet

That could become another unnecessary kernel expansion.

Instead, before implementation, make an explicit decision about what `executed` **means**.

There are two clean choices:

**Option A — redefine `executed`:**

> `executed` means “the external provider accepted the execution request.”

Then document that it does **not** mean provider workflow completion.

**Option B — don't transition to `executed` in 4C+:**

Keep the intent `live` and record provider acceptance in the audit/evidence layer, deferring the state transition until a later phase has authoritative completion/status information.

I lean toward **Option A for this phase**, because the existing schema already has `executed`, and the plan deliberately avoids schema changes. But this needs to be an explicit semantic decision, not left implicit.

---

## Second issue: the T1→T2 model is now honest, but the test target still labels T-16/T-21 too strongly

The plan says:

> "Revocation caught by re-authorization." 

That's fine for:

```text
revocation happens before final authorization check
```

but not for:

```text
revocation happens after final authorization check
before executor invocation
```

The plan itself acknowledges this window exists. 

So the test needs two explicitly different cases:

```text
T-A:
revocation before final check
→ must deny

T-B:
revocation after final check, before executor call
→ behavior is an accepted v1 race / must be characterized
```

Otherwise the test suite will quietly encode the false belief that `PrepareForAction` closes the entire interval.

This is especially important because the project's Phase 6.4 history taught us to distinguish **actual serialization** from **a fresh read**.

---

## The parameter-binding design is now on the right track

This is the strongest improvement.

The plan now says:

```text
Target creation
    consequence_parameters
        ↓
Approve
    snapshot freezes them
        ↓
Execution
    read snapshot
        ↓
construct tuple with them
        ↓
Authorize
        ↓
executor receives approved values
```



That is exactly the kind of extension mechanism we want.

The kernel remains domain-agnostic:

> It does not know what `repo`, `workflow`, or `ref` mean. 

Yet the exact external operation is still bound to the approved target.

That preserves:

```text
generic authority kernel
+
provider-specific consequence parameters
```

without putting GitHub semantics inside the kernel.

---

## `CompleteIntent` still needs one more security constraint

The plan now says:

> `CompleteIntent` is callable only by the trusted execution service path and only a provider-accepted execution may cause that call. 

Good.

But "only the trusted execution service path" needs to be made concrete in the implementation plan.

Otherwise you are creating:

```text
CompleteIntent(intentID)
```

as a generic kernel API whose caller provenance isn't encoded.

The plan should specify something like:

```text
CompleteIntent is not a general-purpose service operation.
Only ExecuteAction may invoke it after:
    current authorization
    approved-parameter binding
    executor invocation
    provider acceptance
```

And test that no API/MCP/client path can call it directly.

The existing plan already says it is not exposed through API/MCP; make the **internal call graph** equally explicit.

---

# My disposition

```text
Test engineering first             ✅
Parameter binding                  ✅
Parameter substitution tests       ✅
TOCTOU honesty                     ✅
GitHub API semantics               ✅
Real side-effect gating            ✅
Executor-selection security        ✅
Credential isolation               ✅
Kilo pre-implementation review     ✅

Intent "executed" semantics        ⚠ MUST clarify
T1→T2 test distinction             ⚠ MUST sharpen
CompleteIntent call provenance     ⚠ MUST make concrete
```

### Therefore

**GO WITH CHANGES — do not let implementation begin yet.**

Send OpenCode this:

```text
Revise the Phase 4C+ plan before Step 0 is submitted to Kilo Code.

1. Resolve the semantic meaning of action_intent.state = 'executed'.

The plan currently says:
    provider acceptance → state = executed

while also correctly stating:
    provider acceptance ≠ provider completion.

Choose and explicitly document one interpretation.

Preferred:
    "executed" means "the external provider accepted the execution request",
    not "the provider completed the workflow."

Make this definition explicit everywhere:
- state model
- correctness invariants
- CompleteIntent justification
- audit semantics
- acceptance criteria
- docs.

Do not add another kernel state unless genuinely necessary.

2. Sharpen the TOCTOU test target.

Distinguish:
    A. authority revoked BEFORE the final authorization check
       → MUST deny and MUST make zero provider calls

from:
    B. authority revoked AFTER the final authorization check but BEFORE
       executor invocation
       → this remains an explicit v1 race window / limitation unless the
         plan discovers a legitimate atomic mechanism.

Do not claim re-authorization closes B.

Add an explicit test case and expected disposition for B.

3. Make CompleteIntent's internal call provenance concrete.

Document:
    ExecuteAction
      → successful current authorization
      → approved-parameter reconstruction
      → executor invocation
      → provider acceptance
      → CompleteIntent

State that CompleteIntent is not a general-purpose internal convenience
method and is not reachable through REST, MCP, or caller-controlled input.

Add a test/verification requirement showing no public or caller-controlled
path can invoke CompleteIntent directly.

Do not otherwise expand the kernel.

Then re-submit the revised test-engineering target to Kilo Code.

Kilo Code must review the target before production implementation begins.
```

Once those three points are settled, **the plan is in good shape for Kilo Code's Step 0 review**. The most important part is that the test target now genuinely captures the dangerous boundary: the real external action must be the **exact provider operation bound to the approved target**, not merely an arbitrary `deploy` action carrying caller-selected parameters. 


## claude

Both fixes landed, one fully, one partially with a reasoning gap worth being precise about.

**Finding #1 (parameter binding) is resolved correctly, not just documented around.** `consequence_parameters` was already a real JSONB field compared via `jsonEqual` inside `kernel.Authorize` — this revision correctly identifies that the existing mechanism already carries the binding this needed, and the fix is to make the executor read the *approved snapshot's* `consequence_parameters` as canonical rather than trusting fresh caller-supplied values. That's a real structural fix, not a policy statement, and SI-16/17/18 plus AT-13/14/15 (substitute repo/workflow/ref, expect zero provider calls) are exactly the right tests for it. Good.

**Finding #2 (stale-authority race) is only half-addressed, and the new justification answers a different question than the one I raised.** §10 now argues concurrent execution isn't a concern because "the execution entry point is the trusted local MCP surface (single process, sequential execution)." But the race I described was never about two executions colliding with each other — it's about a *revocation*, arriving from an entirely independent surface, landing between one execution's re-check and its executor invocation. `RevokeTarget` is reachable via the REST API, which this same project's own Phase 6.4 work established is a fully concurrent, independent surface — nothing about MCP processing its own requests sequentially serializes it against a revoke arriving through REST at the same moment. "Sequential MCP execution" closes off *duplicate-execution* races (a real concern, correctly deferred to Phase 4D), but it doesn't touch the *revoke-races-against-one-in-flight-execution* concern at all, because the revoke was never coming from the MCP surface in the first place.

That said, I think there's a defensible version of "accept this as a v1 limitation" available here — just not the one currently written. The actual window in this design (a single re-check immediately followed by an in-process function call, no intervening database round-trip) is genuinely much narrower than the original Phase 6.4 vulnerability, where the gap spanned two entirely separate transactions. That narrowness is a real, correct argument for treating this as an acceptably small residual risk, the same way T2→T3 is honestly accepted elsewhere in this document. I'd rewrite this section to say that directly — "the window is bounded by a single local function call with no I/O, not by execution being sequential" — rather than leaning on a "sequential, single process" claim that doesn't actually describe the thing that could go wrong.

**One thing worth flagging as a genuine strength, since I went and checked it rather than assuming:** R-9's claim that `return_run_details=true` returns the workflow run ID directly from the dispatch call, avoiding a separate polling step, is accurate — GitHub shipped exactly this in February 2026, replacing the old 204-No-Content-with-no-run-ID behavior that used to force exactly the kind of ambiguous "did it actually dispatch" polling this plan's failure model (F-14, F-19) is designed around. Good that this got verified against the real API rather than assumed.

**Net:** parameter binding is closed, ship it. Rewrite §10's justification to name the actual reason the T1→T2 window is acceptable (no intervening I/O) rather than an argument (sequential MCP processing) that doesn't address the cross-surface revoke case — that's a paragraph-level fix, not a design change, and I'd clear this for the test-engineering-gate step once it's made.


## chatgpt

Agreed. That is the correct disposition.

The only remaining change is to rewrite §10 so it explicitly distinguishes the two concurrency questions:

> **T1→T2 revocation race.** The v1 execution path performs a final authorization check immediately before invoking the executor. The residual race is that a revocation from an independent surface, such as REST, can occur after that check and before the executor call. This window is bounded by the local in-process function-call boundary and contains no intervening I/O or database round-trip; it is therefore materially narrower than the previously identified multi-transaction stale-authority race. The v1 design accepts this residual TOCTOU window as a documented limitation. This does **not** rely on MCP request serialization, which only constrains concurrent executions entering through MCP and does not serialize REST revocations against an in-flight execution.

That wording makes the security claim precise without pretending the race has been eliminated.

I would also make the test target reflect that distinction explicitly:

* **Revocation before T2:** authorization must fail and the provider must receive zero calls.
* **Revocation after T2 but before provider acceptance:** accepted as the documented v1 race; test should characterize, not falsely require, prevention.

With that paragraph corrected, the parameter-binding finding is closed and the test-engineering specification is ready for the Kilo gate.
