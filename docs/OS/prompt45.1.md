I reviewed `plan7(2).md`. The plan is **good and close to implementation-ready**, but I would make several corrections before implementation. The core direction—wire the already-proven `ExecuteAction` into REST/MCP, expose activity, fix reconciliation audit, and finish the GitHub end-to-end workflow—is exactly the right next step. 

There are, however, **four things I would fix first**.

## 1. The biggest security requirement: authenticated principal binding must be explicit

The REST execution contract says:

> `actor_id` derived from authenticated principal. 

Good—but this needs to be treated as a **hard acceptance invariant**, not merely implementation detail.

The new endpoint accepts:

```json
{
  "scenario_id": "...",
  "belief_id": "...",
  "action": "deploy",
  "target_id": "...",
  "intent_id": "...",
  "consequence_type": "execution"
}
```

There is no `principal_id`, which is good. But the handler must obtain the authenticated principal from the existing authentication boundary and pass **that identity** into `ExecuteAction`.

The test suite should explicitly attack:

```text
authenticated principal A
+
intent belonging to principal B
→ DENIED
```

Otherwise you could accidentally create a new API path where the old service-level security works only because the direct caller already supplies the correct principal.

Make this a named acceptance test.

---

## 2. `solvent_activity` needs an authorization decision

The plan adds:

> `solvent_activity` — read audit activity entries for a scenario. 

That is sensible, but this is potentially a new information-disclosure boundary.

The plan currently says:

```text
scenario (optional: type)
```

without defining who may read another scenario's audit trail.

Even in the current single-scenario-per-execution model, I would require the tool to preserve the same authenticated/trusted boundary as the REST activity endpoint.

At minimum the plan should state:

> `solvent_activity` cannot bypass the authorization already governing `GET /v1/activity`; it must enforce the same principal/scenario access semantics.

Don't build a second audit-access policy.

---

## 3. The “production hardening” token-prefix validation is too specific

The plan proposes:

```text
GITHUB_TOKEN must be non-empty and start with ghp_ or github_pat_
```



I would change that.

The application does not need to become a GitHub-token-format validator. Token formats can change, and GitHub supports multiple credential mechanisms. A prefix check adds coupling without strengthening Solvent's authority model.

Use:

```text
GITHUB_TOKEN present
    → construct provider

GITHUB_TOKEN absent
    → do not register provider
```

Then let GitHub reject an invalid credential.

That is simpler and more future-proof.

---

## 4. There is a small internal contradiction in the plan

Section 6 says:

> **No changes to `authority.Service`**. 

But Section 8 explicitly modifies:

```text
service/authority/authority.go
```

to add reconciliation audit logging. 

Obviously the intended meaning is:

> **No changes to `ExecuteAction` or its execution semantics.**

Fix that wording so the implementation agent doesn't treat the later audit change as violating the earlier frozen-service statement.

---

# One more thing I would tighten: the execution API contract

The endpoint currently accepts:

```text
scenario_id
belief_id
action
target_id
intent_id
consequence_type
```

That is workable, but it creates several caller-controlled identifiers which must all remain mutually consistent.

The service already has the important defense: exact intent binding plus kernel authorization. The plan should make the security contract explicit:

```text
intent_id is authoritative identity for the execution attempt
+
all supplied scenario/belief/action/target fields must match the
authoritative execution context
+
authenticated principal comes from the server
+
consequence_parameters never come from the caller
```

Otherwise a future developer may simplify the handler and accidentally trust one of the request fields.

I would make this an adversarial test:

```text
valid intent
+
different target_id
→ DENIED

valid intent
+
different belief_id
→ DENIED

valid intent
+
different action
→ DENIED

valid intent
+
different scenario_id
→ DENIED
```

That is more valuable than merely testing the happy path.

---

# The GitHub flagship workflow is exactly the right demo

This part is strong.

The planned story:

```text id="h6e5a6"
Evidence
  ↓
Agent belief
  ↓
Debt discharged
  ↓
Promotion
  ↓
Target + frozen GitHub parameters
  ↓
Human approval
  ↓
Authorization
  ↓
Intent
  ↓
Execute
  ↓
GitHub workflow_dispatch
  ↓
Audit
```

makes the central Solvent thesis visible in one coherent narrative. 

And the three adversarial demonstrations are particularly good:

```text
caller changes repo/workflow/ref
        → snapshot wins

caller changes executor
        → fixed action→executor wins

authority revoked before execution
        → kernel denies
```



That is much more compelling than a generic CRUD demo.

---

# One thing I would change in the demo language

The plan says:

> “Agent gathers evidence from GitHub issues” and uses an etcd claim. 

Be careful not to make the demo accidentally depend on the external issue being genuinely trustworthy or on the truth of “etcd v3.5.x is safe to deploy.”

For the flagship demonstration, the content is illustrative. The security property is:

> **Even if the agent's evidence/reasoning is wrong or manipulated, it cannot substitute its own authority or execution parameters.**

That should be the narrative focus.

---

# My recommendation

I would **approve the architecture with those four corrections**.

Tell the planning agent:

```text id="5f53tt"
PHASE 4E — FINAL PLAN CORRECTIONS BEFORE IMPLEMENTATION

Make these changes only. Do not redesign the architecture.

1. AUTHENTICATED PRINCIPAL BINDING

Make this a hard security invariant for the new REST execution endpoint and
MCP execution tool.

The authenticated principal must come from the existing authentication
boundary, never from caller input.

Add acceptance/adversarial tests proving:

    principal A + intent belonging to principal B → DENIED

Also test mismatched scenario/belief/action/target identifiers against an
otherwise valid intent.

The endpoint must not trust caller-supplied identity.

2. MCP ACTIVITY ACCESS

Define `solvent_activity` as using the same access semantics as
GET /v1/activity.

It must not create a second authorization policy for audit access or permit
cross-scenario disclosure merely because MCP is local.

3. GITHUB TOKEN VALIDATION

Do NOT validate GITHUB_TOKEN by requiring ghp_ or github_pat_ prefixes.

Use only:

    token present → register provider
    token absent  → provider not registered

Do not over-couple Solvent to GitHub token formatting.

4. SERVICE SECTION WORDING

Replace:

    "No changes to authority.Service"

with:

    "No changes to ExecuteAction semantics or execution authority model."

Reconciliation audit logging in authority.Service remains an approved Phase
4E change.

5. EXECUTION REQUEST SECURITY CONTRACT

Explicitly state:

    intent_id identifies the intended execution attempt.

    scenario_id, belief_id, action, target_id supplied by the caller must
    remain consistent with that intent and are checked by the existing
    authority/execution path.

    authenticated principal is server-derived.

    consequence_parameters never come from the caller.

Add adversarial tests for:
    wrong scenario
    wrong belief
    wrong action
    wrong target
    wrong principal

6. DEMO NARRATIVE

Clarify that evidence/belief contents are illustrative and that the security
demonstration is specifically that agent-provided evidence and reasoning do
not become authority or allow execution-parameter substitution.

Do not change anything else.

After updating the plan, STOP.
Do not implement code yet.
```

After these corrections, I would **lock Phase 4E and implement it**. The existing architecture is already doing the difficult part; Phase 4E's job should be to make the proven execution path actually usable by an agent and demonstrable through the canonical API surfaces, without creating another security layer. 
