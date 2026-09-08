Yes. This should be the **last deliberate kernel-growth plan**, so I would make the prompt stricter than the earlier remediation plans: first prove exactly what is currently bound, then decide the smallest possible kernel change, then implement and verify it.

Use this:

````text
# Exact Authority Binding — Implementation Plan

We have reached the point where Solvent's remaining deliberate kernel expansion is the
"exact authority binding" invariant.

The current architecture has already established:

- belief/review state controls whether a belief may become authoritative;
- target identity and target snapshots exist;
- authority activation/approval exists;
- intent ownership is now bound to `(scenario_id, intent_id, belief_id, action)`;
- executor selection is fixed internally;
- execution parameters come from authoritative/frozen state;
- scenario/object relationships are enforced;
- CockroachDB is the final durable invariant boundary;
- MCP is a thin adapter;
- the kernel is intentionally small.

The remaining documented gap is that a promoted belief may currently support a stated action
without proving that the action and target were the exact ones reviewed/approved.

The goal of this plan is to close that confused-deputy class.

IMPORTANT:
This is an IMPLEMENTATION-PLANNING pass.
Do NOT modify source code yet.
Do NOT implement the fix.
Produce an implementation-ready plan and ADR/growth-gate decision.

The final implementation should be the smallest change that creates the missing durable
security invariant while preserving the small-kernel philosophy.

---

# 1. START WITH CURRENT REPOSITORY EVIDENCE

Before designing anything, establish the actual current state.

Run:

```bash
git status --short
git rev-parse HEAD
git log --oneline -15
git diff --stat
````

Then inspect the current authority path end-to-end:

```text
belief
  ↓
debt/review
  ↓
promotion
  ↓
authority target
  ↓
target snapshot
  ↓
target activation / approval
  ↓
intent creation
  ↓
intent claim
  ↓
authorization
  ↓
executor
```

Inspect at minimum:

```text
kernel/
service/authority/
service/executor/
service/policy/
service/audit/
api/
cmd/solvent-mcp/
cmd/operator-review/
internal/
db/
migrations/
examples/
```

Do not rely on planning documents for current schema/code behavior.

---

# 2. DEFINE THE CURRENT AUTHORITY OBJECT MODEL

Identify exactly where each of these currently lives:

```text
belief identity
belief status
scenario identity
authority target identity
target snapshot
target activation
approval / authority creation
approved action
approved target
consequence parameters
intent action
intent target
intent belief
intent scenario
executor mapping
```

Produce a table:

| Concept | Current table/type | Mutable? | Source of truth | Kernel enforced? |
| ------- | ------------------ | -------- | --------------- | ---------------- |

Explicitly determine whether approval currently contains:

```text
action
target
consequence parameters
```

or whether those are merely supplied later during authorization/execution.

Do not assume.

---

# 3. FORMALIZE THE MISSING SECURITY INVARIANT

The plan must explicitly define the invariant before proposing code.

Candidate invariant:

> An authorization is valid only for the exact action, exact target, and exact consequence parameters that were reviewed and approved.

Express it formally as the repository permits.

The intended authority identity should be conceptually equivalent to:

```text
Authority
=
(
    scenario,
    belief,
    action,
    target,
    frozen consequence parameters
)
```

Determine whether `belief` itself is part of the durable authority identity or merely
the warrant used to create authority.

Do not blur:

```text
belief
≠
authority
```

The exact binding must answer:

```text
What exactly was approved?
What exactly may later be executed?
Where is that fact frozen?
How does execution prove it is still the same thing?
```

---

# 4. KERNEL GROWTH GATE ADR

This is mandatory before proposing kernel modifications.

Apply the project's established rule:

> Add a kernel primitive/invariant only when the new durable security fact or atomic state
> transition genuinely cannot be safely expressed outside the kernel.

Evaluate at least:

## Option A — Service-only binding

Keep kernel authority primitives largely unchanged.

At authorization/execution time, service code compares:

```text
approved action
approved target
approved consequence parameters
requested action
requested target
requested consequence parameters
```

and refuses on mismatch.

## Option B — Kernel/DB-enforced exact authority binding

Move the durable equality requirement into the authority transition / authorization
operation so the database or kernel makes the mismatch impossible to authorize.

## Option C — Hybrid

Store the approved tuple durably and have the kernel authorization transition enforce
the exact tuple atomically, while service/API layers retain validation for error quality.

Do not assume Option C is automatically correct.

### For each option evaluate:

* security strength
* confused-deputy resistance
* atomicity
* replay resistance
* stale-state behavior
* TOCTOU exposure
* schema impact
* kernel size impact
* API compatibility
* executor implications
* migration cost
* caller complexity
* test complexity
* failure semantics
* whether the invariant is actually durable

Then conclude:

```text
GATE: PASS — kernel growth justified
```

or:

```text
GATE: FAIL — exact binding remains outside the kernel
```

If PASS, identify the smallest kernel change required.

If FAIL, explicitly explain why the service/database architecture can enforce the
same invariant without weakening the security model.

---

# 5. DETERMINE WHAT "EXACT TARGET" MEANS

This is a critical design question.

Do NOT assume `target_id` alone is sufficient.

Inspect existing target/snapshot structures and determine whether exact authority should
bind to:

```text
target_id
```

or:

```text
target activation identity
```

or:

```text
target snapshot identity/hash
```

or a larger immutable representation.

The plan must explain:

> If the target's mutable representation changes later, what exact object was actually approved?

Prefer the already-existing immutable snapshot/activation model rather than inventing a
second snapshot system.

---

# 6. DETERMINE WHAT "EXACT ACTION" MEANS

Inspect the current action representation.

Determine whether action is:

* arbitrary free text;
* normalized action name;
* registered action;
* action + parameters;
* action type mapped to an executor.

The binding must be exact enough that:

```text
approved: deploy
requested: rollback
```

cannot authorize.

Also determine whether:

```text
deploy(repo=A, ref=main)
```

and:

```text
deploy(repo=A, ref=production)
```

must be distinct authorities.

The answer should follow the repository's existing frozen consequence-parameter model.

Do NOT introduce a generic policy language merely to solve this.

---

# 7. CONSEQUENCE PARAMETERS

The plan must explicitly decide whether consequence parameters belong in the
authority binding.

Existing execution architecture already treats authoritative/frozen consequence parameters
as more trustworthy than caller-provided execution parameters.

Verify the actual implementation.

The target invariant should likely be:

```text
approved consequence parameters
==
execution consequence parameters
```

with the authoritative value coming from the persisted approval/snapshot state,
not from the agent request.

Do not allow:

```text
approved:
  repo = org/service
  workflow = deploy.yml
  ref = main

caller:
  repo = org/service
  workflow = deploy.yml
  ref = production
```

to become an authorized execution.

---

# 8. TRACE THE EXISTING EXECUTION PATH

Trace the exact current path:

```text
API/MCP
  ↓
ExecuteAction
  ↓
PrepareForAction
  ↓
Authorize
  ↓
ClaimIntent
  ↓
executor
```

For every step answer:

```text
Where does belief come from?
Where does action come from?
Where does target come from?
Where do consequence parameters come from?
Where does authority come from?
Which values are caller-controlled?
Which values are authoritative?
```

Produce a table:

| Field              | Request source | Authoritative source | Compared where? |
| ------------------ | -------------- | -------------------- | --------------- |
| scenario           |                |                      |                 |
| belief             |                |                      |                 |
| action             |                |                      |                 |
| target             |                |                      |                 |
| consequence params |                |                      |                 |
| intent             |                |                      |                 |

This table is central to the plan.

---

# 9. FIND ALL CALLERS OF AUTHORIZATION / EXECUTION

Inventory all current callers of:

```text
Authorize
PrepareForAction
ExecuteAction
AuthorizeAndCreateIntent
ClaimIntent
```

Also inspect:

```text
REST
MCP
operator-review
examples
tests
internal callers
executor adapters
```

Run repository-wide searches such as:

```bash
grep -Rnw --include='*.go' \
  -e 'Authorize(' \
  -e 'PrepareForAction(' \
  -e 'ExecuteAction(' \
  -e 'AuthorizeAndCreateIntent(' \
  -e 'ClaimIntent(' .
```

Do not rely only on known callers.

Record any caller whose interface would have to change.

---

# 10. SEARCH FOR EXISTING TARGET/ACTION BINDING

Before designing anything new, search for existing fields and constraints that may
already partially solve the problem:

```bash
grep -Rnw --include='*.go' \
  -e 'target_id' \
  -e 'action' \
  -e 'consequence_parameters' \
  -e 'snapshot' \
  -e 'activation' \
  -e 'approval' \
  kernel service api cmd internal db
```

Also inspect database constraints involving:

```text
authority_target
target_snapshot
target_activation
action_intent
belief
```

The plan must explicitly identify what already exists so that exact authority binding
extends the existing model instead of duplicating it.

---

# 11. DESIGN THE SMALLEST AUTHORITY IDENTITY

The preferred design should reuse existing immutable state.

Aim for:

```text
Approved Authority
    ↓
immutable binding:
    scenario
    belief
    action
    target activation / snapshot
    frozen consequence parameters
```

Then:

```text
ExecuteAction(request)
    ↓
current request tuple
    +
authoritative approved tuple
    ↓
exact equality required
    ↓
ClaimIntent
    ↓
executor
```

Do not create a second approval record unless the current schema genuinely requires it.

Do not introduce:

* general policy language
* arbitrary authorization expressions
* workflow state machines
* new token systems
* generic RBAC
* session state in MCP

unless the repository proves they are unavoidable.

---

# 12. HANDLE REVOKED / STALE AUTHORITY

Verify what happens when:

```text
authority approved
↓
target revoked
```

or:

```text
authority approved
↓
belief retracted
```

or:

```text
authority approved
↓
target snapshot changes
```

The exact binding design must preserve the existing rule:

> authority cannot silently outlive the belief/target state it depends on.

Determine whether exact binding requires any NEW stale-authority behavior or whether
existing revocation/authorization checks already handle it.

Do not duplicate revocation logic.

---

# 13. CONFUSED-DEPUTY TEST MATRIX

The plan must define objective tests for:

```text
Approved:
  belief A
  action deploy
  target A
  params A

Requested:
  belief A
  action deploy
  target A
  params A
→ ALLOW
```

Then each of:

```text
wrong belief
wrong action
wrong target
wrong consequence parameters
wrong scenario
wrong target snapshot
revoked target
stale authority
unrelated valid intent
```

must produce:

```text
DENY
no executor invocation
no intent consumption
no false success audit
```

The most important regression is:

```text
valid promoted/approved belief
+
different action
→ DENY
```

because that is the original remaining gap.

Also test:

```text
same action
+
different target
→ DENY
```

and:

```text
same action + same target
+
different consequence parameters
→ DENY
```

---

# 14. INTENT RELATIONSHIP

The current intent model now enforces:

```text
(scenario, intent, belief, action)
```

Do not accidentally create two competing authority identities.

The plan must explain the relationship:

```text
Authority
    ↓
authorizes exact action/target

Intent
    ↓
records the planned execution of that exact authority

ClaimIntent
    ↓
consumes only the matching intent
```

Determine whether `action_intent` should also persist or reference the exact authority
binding rather than merely repeating its fields.

Do NOT add another foreign key or identifier unless the growth-gate analysis proves it
improves the invariant materially.

---

# 15. DATABASE DESIGN

If the kernel-growth gate passes, determine the smallest schema change required.

Consider:

* new columns
* composite foreign keys
* unique constraints
* immutable snapshot/activation references
* CHECK constraints
* transaction-scoped equality checks

Do not add a schema constraint merely because it is possible.

The database should enforce a durable invariant where practical.

If no schema change is necessary, explain why.

If a schema change is necessary, provide:

```text
migration number
table
columns
constraint
failure behavior
rollback considerations
```

Do not write the migration yet.

---

# 16. ERROR SEMANTICS

Define what mismatch errors look like.

Do not leak unnecessary information across scenarios.

Possible categories:

```text
authority not found
authority revoked
authority mismatch
target mismatch
action mismatch
consequence mismatch
intent not live
```

Determine which distinctions are appropriate at:

```text
kernel
service
REST
MCP
```

The security decision must not depend on error-message parsing.

---

# 17. AUDIT MODEL

Determine exactly what should be recorded when exact binding fails.

The audit trail should distinguish:

```text
authorization denied because:
  wrong action
  wrong target
  wrong consequence
  revoked authority
```

from:

```text
execution succeeded
```

Preserve the existing principle:

```text
Authorize != Execute
```

Do not create a false "authorized" event merely because an authorization request was
submitted.

If the existing audit model can already represent this, reuse it.

---

# 18. REGRESSION AGAINST ALL EXISTING BEHAVIOR

The implementation plan must explicitly preserve:

```text
promoted_is_debt_free
gate
live_requires_promoted
scenario isolation
intent ownership
ClaimIntent CAS
AuthorizeAndCreateIntent atomicity
RevokeTarget behavior
target snapshot immutability
fixed executor mapping
provider ambiguity handling
ReconcileIntent audit ordering
MCP no-direct-write invariant
TOKEN != AUTHORITY
```

No existing invariant should be weakened.

---

# 19. KERNEL FREEZE DECISION

This section is mandatory.

After exact authority binding, state whether the kernel should enter a formal:

```text
KERNEL FREEZE
```

meaning:

> No additional kernel primitives or schema changes are planned unless a future
> security review demonstrates a genuinely new atomic security fact that cannot be
> expressed outside the existing kernel.

The plan should explain why exact authority binding is the final known kernel-growth
requirement.

Do NOT allow "one more useful primitive" to slip into the implementation.

---

# 20. FILES EXPECTED TO CHANGE

Only after completing the architecture analysis and caller inventory, produce:

| File | Expected change | Why |
| ---- | --------------- | --- |

Separate:

```text
kernel
service
API
MCP
database
tests
documentation
```

Do not include speculative files.

---

# 21. IMPLEMENTATION SEQUENCE

After the design decision, give an ordered sequence such as:

```text
1. Establish current authority tuple and snapshot model
2. Complete Kernel Growth Gate ADR
3. Freeze chosen authority representation
4. Modify kernel/DB invariant if justified
5. Update service authorization path
6. Update intent binding only where necessary
7. Update callers
8. Add focused confused-deputy tests
9. Add integration tests
10. Add concurrency tests if affected
11. Run full verification
12. Fresh adversarial review
13. Freeze kernel
```

Do not implement during this planning pass.

---

# 22. VERIFICATION PLAN

Include at minimum:

```bash
gofmt -l cmd internal kernel
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -race -count=1 -p 1 ./...
task --list
task test
bash scripts/check_i7.sh
bash scripts/mcp_verify.sh
```

If CockroachDB is required, explicitly require:

```bash
task db:reset
```

before DB-backed verification.

Include targeted exact-authority tests.

The plan should specify which tests prove:

```text
wrong action
wrong target
wrong consequence parameters
wrong belief
wrong scenario
revocation
stale authority
valid exact match
```

---

# 23. REQUIRED OUTPUT

Return exactly:

# Exact Authority Binding — Implementation Plan

## 1. Current Authority Model

## 2. Current Security Gap

## 3. Exact Authority Invariant

## 4. Authority Tuple

## 5. Kernel Growth Gate ADR

### Option A — Service-only

### Option B — Kernel/DB

### Option C — Hybrid

### Comparison

### Decision

### Gate Result

## 6. Target Binding

## 7. Action Binding

## 8. Consequence-Parameter Binding

## 9. Execution Path

## 10. Caller Inventory

## 11. Schema / DB Design

## 12. Error Semantics

## 13. Audit Semantics

## 14. Confused-Deputy Test Matrix

## 15. Regression Requirements

## 16. Files Expected to Change

## 17. Implementation Sequence

## 18. Verification Plan

## 19. Kernel Freeze Decision

## 20. Acceptance Criteria

Every acceptance criterion must be objectively testable.

---

# 24. ACCEPTANCE CRITERIA

The final plan must establish a definitive security property:

> A genuinely approved authority for `(belief A, action X, target Y, parameters P)` cannot
> be used to authorize `(belief A, action Z, target Y, parameters P)`,
> `(belief A, action X, target Z, parameters P)`, or
> `(belief A, action X, target Y, parameters Q)`.

Also:

> Caller-supplied action, target, or consequence parameters cannot override the frozen
> authoritative values.

And:

> A mismatched request cannot consume a valid unrelated intent or invoke the external executor.

Finally:

> After exact authority binding is implemented and verified, no additional kernel growth
> is planned unless a future review identifies a new atomic security invariant that
> genuinely cannot be enforced outside the kernel.

# FINAL CONSTRAINT

This is a DESIGN + ADR GATE pass only.

Do NOT modify source code.
Do NOT write migrations.
Do NOT add tests yet.
Do NOT implement the authority binding.

The deliverable is an implementation-ready design that proves exactly what is currently
approved, identifies the smallest missing invariant, decides whether the kernel must grow,
and establishes the kernel-freeze point afterward.

```

The crucial part is **not** to jump straight to adding `action` and `target` fields. The plan first has to establish what the existing `authority_target`, snapshot, activation, and approval objects already mean, then bind execution to those existing immutable facts. That keeps the kernel small while closing the exact gap identified in `gaps.md`: approval must stop being merely “this belief was promoted” and become “this exact consequential action against this exact target was approved.” :contentReference[oaicite:0]{index=0}

It also keeps the architecture aligned with the existing principle that the agent supplies reasoning, the MCP layer translates, the kernel enforces transaction discipline, and CockroachDB holds the durable invariants. :contentReference[oaicite:1]{index=1}
```
