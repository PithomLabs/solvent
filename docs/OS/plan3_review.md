Yes. **Do the adversarial review now, before Phase 4 or Phase 6.**

The implementation report is encouraging, but there are several places where the implementation may already be drifting from the locked architecture. The status report alone is not enough to declare Phases 1–3 secure.

## My adversarial read

### 1. The biggest red flag: the "4-table kernel"

The agent reports:

> `Architecture inspected: 4-table kernel, 7 write sites`

But the locked plan defines the authority model in terms of:

`principal`, `authority_target`, `target_snapshot`, `target_activation`, `target_revocation`, `justification`, `debt_discharge`. 

That is not necessarily a contradiction—the seven objects could map onto four physical tables—but it needs to be proven.

The adversarial question is:

> **Did Phase 1 accidentally move security state out of the kernel or reinterpret kernel concepts through service-layer tables?**

We need to establish exactly which physical tables contain authoritative state and which are merely product state.

**Require an explicit table classification:**

```text
AUTHORITY CORE
    authoritative?
    security-critical?
    kernel-owned?
    DB invariant?

PRODUCT LAYER
    non-authoritative?
    projection/workflow/audit?
```

Anything ambiguous is a finding.

---

### 2. `007_service_tables.sql` needs scrutiny

The plan explicitly said:

> no default workflow migration; product persistence only when genuinely required. 

The implementation has:

```text
db/007_service_tables.sql
    workflow_token
    policy_tool
    policy_actor
    audit_activity
```

This may be perfectly valid—but the agent needs to demonstrate that these are **strictly product-layer records** and cannot become shadow authority.

The dangerous one is:

```text
workflow_token
```

because the locked architecture explicitly says:

> **TOKEN ≠ AUTHORITY**. 

We should inspect whether the token table contains, directly or indirectly:

```text
authority_id
snapshot_id
approval status
debt
evidence state
authorization result
```

Even storing such fields harmlessly can become dangerous if the service later treats them as truth.

---

### 3. The workflow implementation appears materially different from the approved model

The report says:

```text
pending → prepared → executing → completed/failed
```

The approved architecture says:

```text
INVESTIGATING
→ EVIDENCE_REVIEW
→ HUMAN_REVIEW
→ APPROVED
→ AUTHORIZATION_READY
→ EXECUTION
→ COMPLETED

REJECTED
CANCELLED
```

The difference is significant.

It may be a deliberate MVP simplification, but we cannot assume that.

The adversarial question:

> **Has workflow been reduced so aggressively that human review, policy enforcement, or authority establishment have effectively collapsed into `prepared`?**

That would be architectural drift.

More importantly, the plan says workflow **must not replace belief lifecycle, authority lifecycle, or action_intent lifecycle**. 

We should verify that `prepared` isn't functioning as:

```text
"we think this is approved"
```

instead of merely:

```text
"the workflow is ready to perform preparation/revalidation"
```

---

### 4. Evidence scoring is a potential security-smell

The report says:

> `Quality scoring (provenance diversity, reproducibility, grading A-F)`

The plan permits risk/confidence scoring only as an **advisory product-layer projection** and explicitly says it can never create or substitute for authority. 

So the adversarial test is:

```text
score = A
authority = absent
→ MUST DENY
```

and:

```text
score = F
authority = valid
→ may still authorize, subject to policy
```

If `A-F` feeds promotion, authorization, execution, or an implicit "trusted evidence" gate, we have accidentally created a second authority mechanism.

---

### 5. The policy layer is another potential second kernel

The report says:

> `class-based authorization, ... actor registry, tool registry`

This is exactly where a service-layer policy engine can quietly become a second security kernel.

The plan permits policy at the service layer, but the kernel remains the source of authority truth. 

We need to test whether policy is doing:

```text
policy says allowed
→ action proceeds
```

when it should be:

```text
policy permits this actor/action/stage
        +
current Solvent authority exists
        +
exact target/action binding matches
        ↓
allowed
```

Policy may constrain authority.

Policy may not manufacture authority.

---

### 6. "Preparation re-validates belief state" is not enough

The report says:

> `service/authority – Preparation boundary (re-validates belief state at execute time)`

The approved boundary requires re-reading:

```text
belief
evidence
debt
contradictions
current authority
policy
actor/action/target relationship
```

at consequential-action time. 

The phrase **"re-validates belief state"** worries me because it may indicate only belief is reread.

The adversarial test must ask:

> What exact database reads occur immediately before authorization?

If current authority is not re-read there, the architecture has failed its central stale-state defense.

---

### 7. GitHub adapter scope is broad enough to create hidden authority coupling

The implementation says the GitHub adapter handles:

```text
push
PR
CI status
deployment
issue
comment events
```

That's fine as an adapter, but it dramatically increases the attack surface.

We need to verify:

```text
GitHub event
→ normalized evidence
→ belief/evidence
```

and never:

```text
GitHub event
→ implicit approval
→ authorization
```

Likewise:

```text
GitHub says deployment succeeded
```

must never become:

```text
Solvent says deployment was authorized
```

The adapter must remain semantically external. The plan explicitly requires this separation. 

---

### 8. "17 new unit tests" tells us almost nothing yet

This is probably the most important testing problem.

The report highlights:

> `17 new unit tests`

But the locked plan requires adversarial tests at:

```text
KERNEL
SERVICE
ADAPTER
END-TO-END
```

including:

```text
fake approval
no authority
open debt
wrong target
revoked authority
stale state
stale token
manipulated token
browser state
executor authority creation
provider authority creation
```

and specifically:

```text
valid signed stale token
+
revoked current authority
=
DENIED
```



So I would **not accept "17 tests" as evidence of security correctness**.

We need the actual adversarial test matrix and results.

---

# The review I would run now

Do **not** ask the coding agent to continue implementation.

Give it this instead:

```text
STOP IMPLEMENTATION.

Do not begin Phase 4 or Phase 6.

Perform an adversarial security and architecture review of the
current repository against the approved Solvent Consolidated
Implementation Plan.

The objective is NOT to prove that the implementation works.

The objective is to find ways in which the implementation violates,
weakens, duplicates, bypasses, or silently changes the locked
authority model.

Do not modify code unless a tiny diagnostic test is necessary.
Prefer inspection, tracing, and adversarial tests.

==================================================
1. KERNEL BOUNDARY
==================================================

Identify every kernel package, authority-core table, authority
write path, and authority read path.

Produce a table:

component | kernel/product | authoritative? | security-critical?
         | DB invariant? | reason

Verify that the following remain the single authority truth:

principal
authority_target
target_snapshot
target_activation
target_revocation
justification
debt_discharge

Identify every service-layer location that could independently
create, represent, cache, infer, or substitute for authority.

FLAG any second authority engine.

==================================================
2. DATABASE AUDIT
==================================================

Inspect every migration added by the current implementation.

For each new table/column:

table | purpose | product or authority core | authoritative? | danger

Pay particular attention to:

workflow_token
policy_tool
policy_actor
audit_activity

Prove that workflow_token cannot become authority.

Search for fields such as:

authority_id
authority_ref
snapshot_id
approval
approval_status
debt
evidence_status
authorization
authorized
target_snapshot

Determine whether any of these are stored in workflow state
and subsequently trusted.

Confirm whether 007_service_tables.sql is genuinely product-layer
persistence and not disguised authority persistence.

==================================================
3. TOKEN ATTACK
==================================================

Trace the entire workflow-token lifecycle:

creation
signing
parsing
validation
transition
storage
retrieval
execution

Prove:

TOKEN != AUTHORITY

Verify that token contents do not provide authoritative state.

Attempt these attacks:

1. valid token + revoked authority
2. valid token + changed target
3. valid token + changed action
4. valid token + changed policy
5. valid token + changed actor
6. valid token + stale approval
7. forged token
8. tampered token
9. expired token
10. invalid stage transition

Most important:

valid signed stale token
+
revoked current authority
=
DENIED

If not, STOP and report CRITICAL.

==================================================
4. CURRENT-STATE AUTHORIZATION
==================================================

Trace the exact code path from:

execute request
→ preparation
→ authorization
→ executor

List every database read occurring immediately before the
authorization decision.

Verify that the path re-reads current:

belief
evidence
debt
contradictions
authority
policy
actor/action/target relationship

Identify every cache, token, browser value, workflow object, or
provider response that could bypass current database state.

==================================================
5. ACTOR / AUTHENTICATION ATTACK
==================================================

Trace actor handling.

Attempt:

actor=HUMAN in request body

without trusted authenticated principal context.

Verify that this cannot cross a human-only operation.

Verify:

ActorType != Identity != Authentication

Find every place where actor classification is trusted.

==================================================
6. POLICY ATTACK
==================================================

Determine whether PolicyService merely constrains operations or
whether it has accidentally become a second authority engine.

Attempt:

policy=ALLOW
authority=ABSENT

Expected:
DENIED

Attempt:

policy=DENY
authority=VALID

Expected:
DENIED

Attempt:

policy=ALLOW
authority=VALID
wrong target

Expected:
DENIED

Policy must never manufacture authority.

==================================================
7. EVIDENCE ATTACK
==================================================

Trace evidence quality scoring.

Attempt:

score=A
authority=ABSENT

Expected:
DENIED

Attempt:

score=F
authority=VALID

Expected:
policy-dependent authorization, not automatic denial merely due
to score.

Determine whether scoring can influence:

promotion
authority creation
authorization
execution

If scoring can independently authorize an action, report CRITICAL.

==================================================
8. WORKFLOW DRIFT
==================================================

Compare the implemented workflow:

pending
prepared
executing
completed
failed

against the approved conceptual workflow:

INVESTIGATING
EVIDENCE_REVIEW
HUMAN_REVIEW
APPROVED
AUTHORIZATION_READY
EXECUTION
COMPLETED
REJECTED
CANCELLED

Determine whether states were intentionally simplified or whether
important security semantics were accidentally collapsed.

Verify workflow state cannot substitute for:

belief lifecycle
authority lifecycle
action_intent lifecycle

==================================================
9. EXECUTOR ATTACK
==================================================

Trace ExecutionService → Executor → GitHub/provider.

Attempt to determine whether executor code can:

approve
promote
revoke
create authority
create authority-bearing evidence

Expected:
executor can do NONE of these.

Verify:

authorization success != execution success

Test:

valid authorization + provider failure

Expected:
authorization remains a successful fact,
execution is separately recorded as failed.

==================================================
10. EXTERNAL PROVIDER ATTACK
==================================================

Assume GitHub is malicious or compromised.

Attempt:

provider claims approval
provider claims authorization
provider returns malformed payload
provider returns contradictory payload
provider reports successful execution when it actually failed

Verify external provider output cannot directly become Solvent authority.

==================================================
11. ADAPTER BOUNDARY
==================================================

Inspect GitHub adapter.

Verify all GitHub semantics remain outside the kernel.

Verify normalization produces generic Solvent evidence rather than
provider-specific authority semantics.

Flag any kernel dependency on GitHub, webhook, PR, deployment,
issue, or comment concepts.

==================================================
12. AUDIT INTEGRITY
==================================================

Trace audit/activity recording.

Verify separate records exist for:

Solvent authorization
adapter invocation
provider response
executor result

Verify:

DENIED authorization
→ no external execution

AUTHORIZATION ALLOWED + EXECUTION FAILURE
→ both facts remain visible and distinct.

==================================================
13. SECURITY TEST COVERAGE
==================================================

Produce a matrix:

attack | existing test | test location | expected | actual

At minimum:

agent cannot invoke human-only operation
agent cannot approve itself
fake approval denied
no authority denied
open debt denied
wrong target denied
revoked authority denied
stale state denied
stale workflow token denied
manipulated token denied
browser state cannot create authority
executor cannot create authority
provider cannot create authority

==================================================
14. KERNEL GROWTH
==================================================

Compare the repository before and after implementation.

Identify:

new kernel files
modified kernel files
new kernel functions
new authority-core schema
new DB invariants

For every change ask:

Can adapter solve it?
Can service solve it?
Can policy solve it?
Can configuration solve it?

If yes, report the kernel change as unnecessary.

If no, explain the new durable security fact and atomic transition.

==================================================
15. FINDINGS CLASSIFICATION
==================================================

Classify findings:

CRITICAL
High probability of authority bypass, confused deputy,
stale-state bypass, token bypass, or second authority engine.

HIGH
Significant architecture/security weakness requiring correction.

MEDIUM
Meaningful deviation or maintainability/security risk.

LOW
Minor inconsistency or documentation issue.

For each finding provide:

ID
severity
location
attack scenario
why current implementation permits it
expected architecture
recommended correction
whether code change is required

==================================================
16. FINAL GATE
==================================================

End with exactly one of:

GO
No critical/high security findings. Safe to proceed to Phase 4/6.

HOLD
One or more critical/high findings require remediation before
continuing.

Do not recommend proceeding merely because tests pass.

Security correctness takes precedence over schedule.
```

## My recommendation

**HOLD Phase 4 and Phase 6 until that review passes.**

In particular, I would treat these as the four highest-priority questions:

```text
1. Is workflow_token truly non-authoritative?

2. Does the final authorization path re-read CURRENT authority,
   target, action, actor and policy?

3. Has PolicyService remained policy, rather than becoming a
   second authorization engine?

4. Has the new product-layer database schema remained completely
   separate from authority-core truth?
```

The implementation is far enough along that **this is exactly the right moment to attack it**. Finding architectural flaws now is much cheaper than discovering them after the UI and demo layer have cemented the wrong abstractions. The approved plan explicitly makes current authoritative state, token separation, and the absence of competing authority engines non-negotiable.  

