This review is useful, and I would **not ignore the HOLD**—but I also would **not respond by inventing a production executor**.

There is a more important issue now:

> **The repository state described by this independent review does not match the Plan 6 completion state you showed earlier.**

The review still finds:

* `service/workflow` present and dead.
* `service/policy` dead.
* `ExecuteAction` dead.
* MCP `solvent_authorize_action` still conditionally skips `PrepareForAction`.
* `operator-review` still bypasses `PrepareForAction`.

Those are directly contrary to parts of the Plan 6 work you said had been completed, especially the workflow removal and MCP fail-closed change. The review explicitly reports `service/workflow` still exists and is not imported, and that the MCP conditional skip remains.  

So **before Phase 4, stop and reconcile the actual repository state**.

## What the review proves

The good news is substantial.

The fresh audit found:

```text
No hidden consequential production side effects
No alternate authority creator
kernel.Authorize remains the authority oracle
Exact tuple binding preserved
Revocation enforced
No direct provider bypass
No default-allow behavior
No build-target bypass
21 integration tests remain meaningful
```

Those findings are supported by the review's sections on side effects, authority creation, tuple binding, transactions, error handling, and test quality.    

So I do **not** see a newly discovered authority bypass here.

The current HOLD is mainly:

```text
current scope/documentation/code state inconsistent
+
dead code remains
+
MCP fail-closed fix apparently did not land
```

That is different.

---

# The two findings I would fix immediately

## F-2: `service/workflow` must actually be gone

Plan 6 explicitly called for deleting:

```text
service/workflow/workflow.go
service/workflow/workflow_test.go
```

Yet the fresh review still finds the package and calls it dead. 

That means one of two things happened:

1. the cleanup was not actually committed/applied, or
2. the review ran against a different repository state.

Either way, resolve it **before anything else**.

Do not wire workflow into execution.

Delete it if it has no current requirement.

---

## F-4: MCP conditional authority skip is still present

This is more important than F-3.

The review says:

```text
target_id missing OR actor_id missing
→ PrepareForAction skipped
→ intent created
```



That contradicts the Plan 6 requirement that incomplete authorization context must fail closed.

This should be fixed exactly as previously specified:

```text
missing target_id
    OR
missing actor_id
        ↓
REJECT
```

No silent bypass.

Even though this only affects intent creation and not current external execution, it is still a bad security/API semantic because it creates two different paths into the same intent operation.

---

# F-1 should be reclassified

The reviewer continues to call:

> no production execution path

a HIGH finding. 

Under the corrected Plan 6 scope, I would **not classify that as a security vulnerability**.

The correct statement is:

```text
No production consequential execution capability exists.
Therefore there is currently no production execution boundary to bypass.
```

The future invariant remains:

```text
future execution
→ ExecuteAction
→ current revalidation
→ kernel.Authorize
→ Executor
```

That is already how the architecture is documented. 

So F-1 becomes:

> **Scope/documentation issue, not a security blocker.**

But we should not use that conclusion to excuse dead code.

---

# F-3 is acceptable with explicit trust-boundary documentation

The `operator-review` CLI bypass is real:

```text
operator-review
→ kernel.IntentOnPromoted
```

instead of:

```text
operator-review
→ PrepareForAction
```

The review correctly classifies it as a trusted administrative CLI and gives it MEDIUM rather than HIGH. 

I would keep this as an explicit exception:

```text
trusted administrative CLI
≠
runtime security boundary
```

But the documentation needs to make clear that it must never perform future consequential external execution outside `ExecuteAction`.

---

# F-5 is fine

`service/policy` being unused is acceptable **provided we stop describing it as an active security layer**.

The review correctly says it currently cannot weaken execution because there is no execution path and it is dead. 

I'd either:

```text
remove service/policy
```

or:

```text
mark clearly FUTURE / NOT MVP
```

Given our complexity-earned philosophy, I lean toward removal unless Phase 4 actually needs it.

---

# The real next step

Do **not** go to Phase 4 yet.

Do this instead:

```text
1. Verify repository/commit being reviewed
2. Confirm whether service/workflow really exists
3. Fix/remove service/workflow
4. Fix MCP conditional skip
5. Re-run:
      go test -count=1 -p 1 ./...
      go build ./...
      go vet ./...
6. Run one final independent targeted review
7. Reclassify F-1 as scope, not security, if no execution exists
8. Require:
      0 Critical
      0 High
      0 unresolved security bypasses
9. Then Phase 4
```

The fact that the independent reviewer found the **MCP skip still present** even though your earlier Plan 6 completion report claimed it was fixed is the thing I would investigate first. 

## Give the coding agent this next

```text
SOLVENT — RECONCILE POST-REVIEW STATE BEFORE PHASE 4

STOP. Do not begin Phase 4.

The independent targeted adversarial review found discrepancies
between the expected Plan 6 state and the actual repository state.

Do NOT add a fake production executor.
Do NOT wire workflow into execution.
Do NOT weaken authority checks.

First reconcile the repository.

==================================================
1. VERIFY THE REVIEWED REPOSITORY STATE
==================================================

Confirm:

    git status
    git rev-parse HEAD
    git diff
    git log -1 --stat

Confirm that the repository being reviewed is the same state in
which Plan 6 was supposedly completed.

If the review was performed against an older commit/state, say so
explicitly and repeat the review against the current HEAD after
verification.

==================================================
2. REMOVE service/workflow IF STILL PRESENT
==================================================

Plan 6 intended:

    service/workflow/workflow.go
    service/workflow/workflow_test.go

to be removed because there is no current product requirement.

If those files still exist and remain unused:

    DELETE THEM.

Do NOT wire them into execution merely to eliminate the finding.

Confirm no production/test imports remain.

==================================================
3. FIX MCP CONDITIONAL AUTHORITY SKIP
==================================================

Inspect:

    cmd/solvent-mcp/tools.go
    handleSolventAuthorizeAction

Required:

    target_id missing OR actor_id missing
        →
    reject request

Forbidden:

    target_id missing OR actor_id missing
        →
    skip PrepareForAction
        →
    create intent

Intent creation must use a single explicit contract.

If target_id and actor_id are required to construct the intended
authorization context, require them.

==================================================
4. REVIEW operator-review TRUST BOUNDARY
==================================================

Keep operator-review as trusted administrative tooling if that is
the intended architecture.

Document:

    trusted administrative CLI
    direct operator authority
    not runtime execution security boundary

It MUST NOT perform future consequential external execution directly.

Any future consequential external operation must use:

    ExecuteAction
        →
    current revalidation
        →
    kernel.Authorize
        →
    Executor

==================================================
5. REVIEW service/policy
==================================================

Determine whether service/policy has a current MVP requirement.

If not:

    remove it

or explicitly mark it:

    FUTURE / NOT MVP

Do not represent it as an active production security layer.

Do not wire unused policy merely to eliminate a review finding.

==================================================
6. PRESERVE CURRENT/FUTURE ARCHITECTURE
==================================================

CURRENT:

    evidence
      ↓
    belief
      ↓
    promotion
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

Do NOT create production execution merely to satisfy the review.

==================================================
7. ADD RULE TO SECURITY DOCUMENTATION
==================================================

Explicitly state:

    No consequential production execution capability currently exists.

    Therefore no current production execution path can bypass authority.

    ExecuteAction is the canonical future execution boundary.

    Any future consequential side effect MUST originate from:

        ExecuteAction
        →
        current authorization
        →
        Executor

Also preserve:

    intent creation
    belief promotion
    evidence ingestion
    authority creation

MUST NOT directly invoke consequential external side effects.

==================================================
8. RUN FRESH VERIFICATION
==================================================

With the corrected repository:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...

No cached tests.

==================================================
9. RUN FINAL SCOPE-AWARE ADVERSARIAL REVIEW
==================================================

Perform a NEW independent review against CURRENT HEAD.

Search for:

    all external side effects
    all provider calls
    all executor calls
    all ExecuteAction callers
    all direct IntentOnPromoted calls
    all direct kernel.Store calls
    all kernel.Authorize calls
    all authority creation sites
    all workflow/token remnants
    all policy authority decisions

Confirm:

    no production consequential execution exists

and:

    no hidden consequential side effect exists

and:

    no second authority engine exists

and:

    no dead security component is represented as active security
    infrastructure

==================================================
10. FINDING CLASSIFICATION

Do NOT call "no production execution capability" a security
vulnerability when no consequential production action exists.

Classify it as:

    PRODUCT SCOPE / DOCUMENTATION

unless an actual consequential execution path exists.

Security blockers are:

    CRITICAL
    HIGH

Only actual current security defects should receive these severities.

==================================================
11. FINAL GO GATE

GO only if:

    no CRITICAL findings
    no HIGH security findings
    no existing consequential production bypass
    no hidden consequential side effects
    kernel.Authorize remains authority oracle
    authority creation remains controlled
    exact tuple binding remains enforced
    revocation remains effective
    policy cannot manufacture authority
    workflow cannot manufacture authority
    intent cannot manufacture authority
    evidence cannot manufacture authority
    providers cannot manufacture authority
    MCP incomplete authorization context fails closed
    operator-review trust boundary is documented
    no dead security boundary is presented as active
    future execution boundary remains ExecuteAction
    tests/build/vet all pass

Otherwise:

    HOLD

Do not proceed to Phase 4 on HOLD.

==================================================
FINAL REPORT
==================================================

Report:

    repository HEAD reviewed
    files removed
    files changed
    test results
    build result
    vet result
    current vs future architecture
    remaining risks
    all findings and severity
    final GO/HOLD

Do not rely on the previous security report.
Review CURRENT HEAD.
```

The independent review itself found **no critical authority bypass and no hidden consequential production side effect**, which is encouraging.  

So I would treat this as a **reconciliation pass**, not another architectural redesign.

The one thing I would absolutely not tolerate is declaring another GO while the repository still contradicts the documented Plan 6 state. **Current HEAD must match the security report.**



## claude

Round 2 correctly resolves round 1's core findings — `kernel.Authorize` is now genuinely the sole final oracle, checking all eight fields, revocation-checked, called from a real `PrepareForAction`, and policy/workflow are cleanly disconnected rather than dangerously wired in as pseudo-authority. That part of the trajectory is right. But I don't think HOLD-because-`ExecuteAction`-is-dead-code is actually the most important finding in this document, and the review's own §13 surfaces something more urgent that it then waves away too quickly.

**This is the one that matters: `handleSolventAuthorizeAction` silently downgrades to the old, weaker gate whenever `target_id`/`actor_id` are omitted — and the agent controls whether they're omitted.**

§13 states this plainly and then dismisses it in the same breath: *"skips `PrepareForAction` when `targetID` or `actorID` are missing... This is intent creation, not execution. The database still enforces belief promotion via the composite FK."* That's true, but it's describing exactly the *old* gate — belief-promoted-therefore-live-intent-allowed — which is the mechanism the entire v0 authority-target model (`target_snapshot`/`target_activation`/exact-tuple binding) was built specifically to supersede, because it's the mechanism vulnerable to the confused-deputy case (Scenario D, several rounds back: a promoted belief cited for an action it was never approved for). If an agent can reach the stronger check or the weaker one *by choosing whether to include two optional parameters in its own tool call*, that's not a dead-code gap sitting safely unreachable — it's a live, agent-triggerable downgrade path on the one surface in this whole review that actually is reachable in production. Unlike `ExecuteAction`, this isn't hypothetically dangerous once wired up someday; `solvent_authorize_action` is callable today. This deserves to be the headline finding, not a footnote inside the MCP attack table, and it's inconsistent with the rigor the rest of this document applies everywhere else — the same document correctly refuses to accept "policy is probably fine" or "workflow tokens are probably fine" on the strength of an assurance, and then accepts exactly that kind of assurance here.

**Fix, and it's cheap:** make `target_id` and `actor_id` required parameters on `solvent_authorize_action`, full stop — reject the call before any DB read if either is missing, the same fail-closed pattern already used for `action_source`. If there's a real reason a caller sometimes can't supply them, that needs to be a named, deliberate, documented exception with its own justification, not a silent conditional skip discovered by an adversarial reviewer.

**Second, smaller point, same shape of problem:** the operator-review CLI bypassing `PrepareForAction` gets the same one-line dismissal — *"documented as a trusted admin tool."* This project has spent five review rounds refusing to accept "trusted" as a substitute for "structurally enforced" everywhere else in this codebase. I'd want the same question asked here that's been asked of every other actor: what happens if a legitimate operator runs a stale `operator-review` session against a belief whose authority target was revoked five minutes ago? Right now the answer is "the intent gets created anyway, because this path doesn't check `target_revocation` at all." Low blast radius, since it's a local admin tool — but "low blast radius" was never the bar this project held anything else to.

**Third, a framing note on the verdict itself, not a new defect:** "Required Before GO" offers wiring `ExecuteAction` and documenting deferral as equally acceptable options. I don't think they are. If Phase 4/6 is supposed to demonstrate a real authorize-through-execute flow — which is what every planning document in this thread has assumed Phase 4/6 means — then "document that it's deferred" quietly changes what Phase 4/6 delivers rather than satisfying it. That's a legitimate call to make, but it should be made explicitly, by you, as a scope decision, not left as an either/or a reviewer offered in passing.

**Given the stated urgency, here's the concrete path, not another open-ended cycle:** fix the `solvent_authorize_action` required-parameters gap now — it's small and it's the one live issue. Decide, in one sentence, whether Phase 4/6 needs a real executor or ships as authorization-only for this milestone, and write that decision down wherever the phase scope lives. Leave `operator-review` as a named, accepted residual — low-risk, worth a comment in the code saying why, not worth blocking on. That closes this out in an afternoon, not another review round.


## PROMPT

I agree with the critique. The **live MCP downgrade path is the most important remaining issue**, and the previous review gave it too little weight.

`solvent_authorize_action` is reachable today, and the caller controls whether `target_id` and `actor_id` are present. That means the stronger authority-target path is optional at the MCP boundary. The fact that the downstream operation is currently only intent creation reduces blast radius, but it does **not** make the trust-boundary downgrade acceptable. The safer contract is simply:

```text
missing target_id OR missing actor_id
    → reject immediately
```

The operator-review issue is real too, but I agree with your prioritization: **accept it as a named residual trusted-admin exception rather than block on it**. The important thing is to document exactly why it is trusted and that it must never become a consequential execution path.

For the phase-scope question, I would make the explicit decision:

> **Phase 4/6 for this milestone are authorization-first and ship without a real production executor; real consequential execution is deferred until a concrete external execution capability is chosen.**

That preserves the complexity-earned principle instead of inventing a fake deployment system. The future execution contract remains `ExecuteAction → current revalidation → kernel.Authorize → Executor`.

## Send this to the coding agent

```text
SOLVENT — FINAL PRE-PHASE-4 SECURITY FIX

The independent adversarial review identified one LIVE production issue
that must be fixed before Phase 4:

MCP `solvent_authorize_action` allows the caller to omit `target_id`
and/or `actor_id`, which silently downgrades to the older belief-only
intent gate.

This is not acceptable.

==================================================
1. FIX MCP REQUIRED PARAMETERS
==================================================

In:

    cmd/solvent-mcp/tools.go
    handleSolventAuthorizeAction

Make these parameters mandatory:

    target_id
    actor_id

Required behavior:

    target_id missing
        →
    reject immediately

    actor_id missing
        →
    reject immediately

    target_id == empty
        →
    reject

    actor_id == empty
        →
    reject

Do NOT silently skip PrepareForAction.

Do NOT fall back to:

    promoted belief
        →
    IntentOnPromoted

when authorization context is incomplete.

The MCP operation must have one security contract, not two.

Preferred ordering:

    validate required fields
        ↓
    obtain trusted actor context
        ↓
    policy/constraint processing as applicable
        ↓
    PrepareForAction
        ↓
    kernel authority semantics
        ↓
    intent creation

No DB write should occur when required authorization parameters
are missing.

Add or update a regression test proving:

    missing target_id → rejected
    missing actor_id  → rejected

and proving that no intent is created.

==================================================
2. REVIEW OPERATOR-REVIEW AS ACCEPTED RESIDUAL
==================================================

Keep `cmd/operator-review` as trusted administrative tooling.

Do NOT force it through production runtime authorization merely to
eliminate a review finding.

Add an explicit code comment/documentation stating:

- this is trusted administrative tooling
- it operates under direct operator authority
- it is not an autonomous/runtime execution boundary
- it may create intents directly through kernel APIs
- it MUST NOT perform consequential external execution directly
- any future consequential external execution must use:

      ExecuteAction
        ↓
      current-state revalidation
        ↓
      kernel.Authorize
        ↓
      Executor

This is an accepted residual trust boundary, not an accidental
security mechanism.

==================================================
3. VERIFY NO OTHER OPTIONAL AUTHORIZATION DOWNGRADES
==================================================

Search all MCP and HTTP handlers for patterns such as:

    if field != "" {
        perform security check
    }

or any conditional path where missing security context causes a
stronger authorization check to be skipped.

Search for:

    actor_id
    target_id
    action_source
    authority
    authorization
    PrepareForAction
    kernel.Authorize

For every consequential or intent-authorizing operation, determine:

    required
    optional
    default behavior

Any missing security context that silently falls back to a weaker
gate must be fixed.

==================================================
4. PHASE 4 / PHASE 6 SCOPE DECISION
==================================================

Record this explicit product decision in the phase-scope documentation:

    Phase 4 and Phase 6 for this milestone are authorization-first
    and ship without a real production consequential executor.

    Real consequential execution is deferred until a concrete
    external execution capability is selected.

The future execution contract remains:

    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    Executor
      ↓
    external provider

Do NOT create a fake deployment system or fake production side effect
to satisfy the phase.

The existing recording executor remains test-only.

==================================================
5. TESTING
==================================================

Run:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...

Also verify specifically:

    missing target_id
        → request rejected
        → no intent created

    missing actor_id
        → request rejected
        → no intent created

    valid target_id + actor_id
        → existing authorization behavior preserved

==================================================
6. FINAL TARGETED REVIEW
==================================================

After the fix, perform one final targeted read-only review of:

    cmd/solvent-mcp/tools.go
    all MCP tools
    operator-review
    all IntentOnPromoted call sites
    all PrepareForAction call sites
    all kernel.Authorize call sites
    all external side-effect sites

Focus specifically on:

    optional security parameters
    silent downgrade paths
    direct intent creation
    trusted admin exceptions
    hidden consequential side effects

Do not reopen previously settled architecture unless this review
finds a concrete contradiction.

==================================================
7. FINAL CLASSIFICATION
==================================================

Expected result after fix:

CRITICAL: 0
HIGH:     0

Accepted residual:

    operator-review trusted administrative CLI

Document it as:

    ACCEPTED TRUST BOUNDARY — NON-RUNTIME, NON-CONSEQUENTIAL

Do not classify "no production executor exists" as a security flaw.

==================================================
8. FINAL GO GATE
==================================================

GO only if:

- MCP missing target_id fails closed
- MCP missing actor_id fails closed
- no silent authorization downgrade exists
- no existing consequential production side effect bypass exists
- no hidden consequential side effect exists
- kernel.Authorize remains the final authority oracle
- exact tuple binding remains enforced
- revocation remains effective
- policy cannot manufacture authority
- workflow cannot manufacture authority
- intent cannot manufacture authority
- operator-review is explicitly documented as trusted admin tooling
- Phase 4/6 authorization-first scope is explicitly documented
- tests/build/vet pass
- no critical/high security findings remain

Otherwise:

HOLD

Do not begin Phase 4 on HOLD.
```

The key change in our reasoning is that we're no longer using **"execution doesn't exist"** to excuse a reachable weaker authorization path. The MCP surface is live, so its contract must be fail-closed now.

After this fix and the final targeted review, I would be comfortable moving to Phase 4 under the explicit **authorization-first** milestone scope.
