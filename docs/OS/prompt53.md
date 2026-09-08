Use this as the post-remediation adversarial review prompt:

````text
# Fresh Adversarial Review — Post-NEW-01/02/03 Remediation

Review the CURRENT repository HEAD after the remediation of:

- NEW-01 — `ClaimIntent` intent ownership
- NEW-02 — `IntentOnPromoted` cross-scenario isolation
- NEW-03 — `AddEvidence` cross-scenario isolation
- NEW-04 — formatting regression
- NEW-05 — MCP verification database/migration mismatch

This is a FRESH, INDEPENDENT adversarial review.

Do NOT implement fixes.
Do NOT modify source code, tests, configuration, documentation, or plans.
Run real commands against the actual repository.
Inspect the current source tree and actual database-backed behavior.
Do not assume the remediation is correct because tests pass.
Do not assume the previous adversarial review was complete.

The objective is to BREAK the current implementation and determine whether any material security, integrity, isolation, authority, concurrency, audit, or architectural defect remains.

---

# 1. Establish the Exact Review Target

Run first:

```bash
git status --short
git rev-parse HEAD
git log --oneline -15
git diff --stat
git diff
````

Record whether the remediation is:

* committed
* uncommitted
* mixed

Do not confuse the previous reviewed commit with the actual source currently under review.

---

# 2. Verification Baseline

Run the complete verification suite before drawing conclusions:

```bash
gofmt -l cmd internal kernel
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -race -count=1 -p 1 ./...
task --list
task db:reset
task test
bash scripts/check_i7.sh
bash scripts/mcp_verify.sh
```

Clearly classify every result as:

```text
PASS
FAIL
BLOCKED BY ENVIRONMENT
NOT RUN
```

Do not call a blocked check PASS.

Verify that `task db:reset` actually provisions all required authority migrations and that `mcp_verify.sh` is operating against the intended database.

---

# 3. Primary Objective: Attack the New Invariants

The remediation introduced these intended invariants:

```text
1. RetireDebt:
   (scenarioID, beliefID) must match

2. Promote:
   (scenarioID, beliefID) must match

3. Discharge:
   (scenarioID, beliefID) must match atomically

4. AddEvidence:
   (scenarioID, beliefID) must match

5. Intent creation:
   (scenarioID, beliefID) must match

6. Intent claim:
   (scenarioID, intentID, beliefID, action) must match

7. Reconciliation:
   successful state transition precedes successful-completion audit
```

Do not merely verify happy paths.

Try to violate each invariant through every accessible caller.

---

# 4. NEW-01 — Intent Ownership

Inspect:

```text
kernel/sql.go
kernel/authority.go
kernel/contract.go
service/authority/authority.go
```

Verify the atomic claim predicate includes:

```text
intent ID
scenario ID
belief ID
action
state = live
```

The critical SQL should conceptually be:

```sql
UPDATE action_intent
SET state = 'executing'
WHERE id = $1
  AND scenario_id = $2
  AND belief_id = $3
  AND action = $4
  AND state = 'live'
```

Do not accept a service-layer pre-check as equivalent to the atomic CAS.

## Attack matrix

Test:

```text
correct scenario + correct belief + correct action
→ succeeds

correct scenario + wrong belief
→ rejected

correct scenario + wrong action
→ rejected

wrong scenario + correct belief/action
→ rejected

intent from another belief
→ rejected

intent from another action
→ rejected
```

For every rejection prove:

```text
intent remains live
executor not called
no external action occurs
```

Reproduce the exact previous exploit:

```text
Belief A:
  authorized for deploy
  no live deploy intent

Belief B:
  live rollback intent

ExecuteAction(
  A,
  deploy,
  intentB
)
```

Expected:

```text
rejected
intentB remains live
executor not invoked
```

---

# 5. IMPORTANT: Verify Intent Tuple Immutability

The claim-time security model assumes these fields cannot change after intent creation:

```text
action_intent.scenario_id
action_intent.belief_id
action_intent.action
```

Perform a repository-wide search:

```bash
grep -Rnw --include='*.go' \
  -e 'UPDATE action_intent' \
  -e 'scenario_id' \
  -e 'belief_id' \
  -e 'action' \
  kernel service api cmd internal
```

Determine whether any reachable code can mutate those three fields after creation.

If they are immutable, establish that from actual schema/code.

If they are mutable, attack the sequence:

```text
claim validated tuple
    ↓
intent tuple changes
    ↓
CompleteIntent / RollbackClaim / CancelIntent
```

This is a critical prerequisite for NEW-01 closure.

---

# 6. NEW-02 — Intent Creation Scenario Isolation

Inspect:

```text
createIntentWithinTx
IntentOnPromoted
AuthorizeAndCreateIntent
sqlIntentOnPromoted
action_intent constraints
```

Verify that the scenario/belief ownership check occurs:

```text
inside the same transaction
before INSERT
```

Attack:

```text
scenario B + belief owned by A
```

through BOTH:

```text
IntentOnPromoted
AuthorizeAndCreateIntent
```

Expected:

```text
error
zero action_intent rows
```

Do not test only `IntentOnPromoted`.

---

# 7. RetractCascade DoS Regression

Reproduce the original NEW-02 exploit attempt:

```text
scenario A owns promoted belief A

attempt to create intent:
scenario B + belief A

then:
RetractCascade(scenario A, belief A)
```

Expected:

```text
cross-scenario intent creation rejected
RetractCascade succeeds
belief becomes retracted
no SQLSTATE 23514
```

Then inspect the full `RetractCascade` implementation for any other way a foreign-scenario intent could remain attached to the victim belief.

Do not assume the patched creation path is the only possible source of malformed state.

---

# 8. NEW-03 — Evidence Isolation

Inspect:

```text
kernel.AddEvidence
sqlAddEvidence
api/evidence.go
service/ledger/
```

Verify:

```text
same scenario + belief
→ success

wrong scenario + belief
→ rejected
→ zero evidence rows
```

Test both:

```text
kernel
REST /v1/evidence
```

Also search for other callers of `AddEvidence`.

Every caller must preserve:

```text
scenarioID ↔ beliefID
```

Do not assume REST is the only entry point.

---

# 9. Search for the BROADER Scenario-Pairing Class

NEW-02 and NEW-03 demonstrated that scenario/object relationship invariants can be omitted from seemingly unrelated mutation paths.

Do a systematic inventory.

Search every mutation involving:

```text
scenario_id
belief_id
target_id
intent_id
activation_id
revocation_id
```

Inspect all code that creates or mutates:

```text
belief
evidence
action_intent
target
target_activation
target_revocation
debt_discharge
```

For each relationship ask:

```text
Does this object identity belong to the supplied scenario?
Where is that invariant enforced?
Can a caller bypass it?
Is enforcement atomic?
```

This is a targeted invariant audit, NOT permission for an unrelated redesign.

Report any new CURRENT exploit.

---

# 10. RetireDebt and Promote

Re-test the Plan 8.4 changes.

For `RetireDebt` verify:

```text
same scenario + item present
→ success

same scenario + item already absent
→ idempotent success

wrong scenario
→ error

nonexistent belief
→ error
```

Pay attention to the implementation's existence-check semantics and transaction boundary.

For `Promote`:

```text
same scenario
→ success where policy/state permits

wrong scenario
→ rejected

nonexistent belief
→ rejected
```

Ensure zero-row/empty-result behavior cannot silently become success.

---

# 11. Discharge Atomicity

Inspect the complete transaction.

Expected logical sequence:

```text
verify scenario/belief
    ↓
insert discharge record
    ↓
update belief debt
    ↓
commit
```

Attack:

```text
wrong scenario
duplicate discharge
wrong obligation
already-retired obligation
DB error
context cancellation
transaction retry
concurrent discharge
```

Prove:

```text
invalid discharge
→ no discharge record
→ no debt mutation

valid discharge
→ exactly one discharge record
→ intended debt removed
```

The old CS-6 forced rollback harness was flaky and was not a release blocker by itself.

Do not turn "CS-6 was not reproduced" into a finding unless you discover an actual atomicity flaw in the implementation.

---

# 12. ExecuteAction End-to-End

Trace:

```text
ExecuteAction
  ↓
PrepareForAction
  ↓
Authorize
  ↓
ClaimIntent
  ↓
executor
  ↓
CompleteIntent
```

Verify these fields remain coherent throughout:

```text
scenarioID
beliefID
action
targetID
intentID
```

Attack combinations where one field belongs to another object.

Especially test:

```text
valid authority for A/deploy
+
intent for B/rollback
```

and:

```text
valid authority for A/deploy
+
intent for A/rollback
```

No executor call should occur unless the entire tuple matches.

---

# 13. AuthorizeAndCreateIntent

This is a particularly important path because it combines authorization and intent creation.

Verify:

```text
scenario/belief mismatch
→ rejected

target/belief mismatch
→ rejected

action mismatch
→ rejected

correct tuple
→ intent created
```

Review the transaction and lock ordering.

Look for:

```text
authorization approved
intent creation using different object identity
```

or:

```text
scenario checked in one transaction
intent inserted in another
```

---

# 14. Executor Integrity

Verify callers cannot choose an arbitrary executor.

Search:

```bash
grep -Rnw --include='*.go' \
  -e 'executor' \
  -e 'provider' \
  -e 'workflow_dispatch' \
  service executor adapter api cmd
```

Check:

```text
action
→ fixed executor mapping
```

not:

```text
caller-supplied executor
→ execution
```

Also verify provider responses cannot elevate authority or force an invalid intent transition.

---

# 15. Audit Truthfulness

Inspect:

```text
ReconcileIntent
CompleteIntent
ActivityReconciliationCompleted
ActivityReconciliationFailed
```

Attack:

```text
state transition succeeds
→ completed audit

state transition fails
→ no false completed audit
→ truthful failure event
→ original error preserved
```

Also inspect audit failure behavior.

Ask:

> Can a failed audit operation cause the system to report a successful state transition when it did not occur?

Verify that `Authorize != Execute` remains visible in the audit model.

---

# 16. MCP Boundary

Re-review all MCP tools.

Verify:

```text
scenario handling
belief handling
actor attribution
action_source
mutation boundaries
audit paths
error paths
```

Test cross-scenario mutations through:

```text
retire_debt
promote
discharge
evidence if exposed
intent creation if exposed
```

Treat F-01 exactly as previously established:

```text
ACCEPTED ARCHITECTURAL LIMITATION
```

Do not count external stdin/stdout tunneling as a new code defect.

Do verify that the implementation itself has no accidental network transport.

Verify:

```text
MCP_TRANSPORT=sse
→ fail closed

MCP_TRANSPORT=http
→ fail closed

unset
→ stdio
```

---

# 17. REST Boundary

Review all REST mutation endpoints, not only evidence.

Search:

```bash
grep -Rnw --include='*.go' \
  -e 'http.MethodPost' \
  -e 'http.MethodPut' \
  -e 'http.MethodPatch' \
  -e 'http.MethodDelete' \
  api
```

For each mutation determine:

```text authenticated principal
scenario context
object identity
ownership check
kernel mutation
audit
```

Look specifically for another endpoint with the same pattern as NEW-03:

```text
caller supplies scenario_id
caller supplies object_id
handler assumes pairing is valid
```

---

# 18. Database / Schema Integrity

Inspect the schema and migrations.

Look for whether these relationships are structurally enforceable:

```text
action_intent.scenario_id ↔ belief.scenario_id
evidence.scenario_id ↔ belief.scenario_id
debt_discharge ↔ belief
```

Do not automatically demand composite foreign keys.

Determine whether the application-level checks are sufficient and atomic for the current architecture.

Report only concrete security or integrity consequences.

---

# 19. Concurrency and Race Analysis

Run:

```bash
go test -race -count=1 -p 1 ./...
```

Then specifically inspect concurrent behavior for:

```text
two ClaimIntent callers
ClaimIntent + ExecuteAction
AuthorizeAndCreateIntent + RevokeTarget
RetireDebt concurrent calls
Discharge concurrent calls
IntentOnPromoted concurrent calls
```

Look for duplicate execution, inconsistent state, or a race that bypasses the security invariant.

Distinguish normal CockroachDB transaction retry behavior from actual application-level bugs.

---

# 20. Error Semantics / Fail Closed

Search security-sensitive code for patterns like:

```text
err ignored
zero value on type assertion
RowsAffected ignored
empty ID accepted
nil decision treated as allowed
default enum treated as safe
audit failure ignored
```

Examples:

```bash
grep -Rnw --include='*.go' \
  -e ', _ :=' \
  -e 'RowsAffected' \
  -e 'if err != nil' \
  kernel service api cmd internal
```

Do not flag every occurrence mechanically.

Inspect each one in a security-sensitive path.

---

# 21. Historical Findings

Treat these as established context:

```text
F-02 CLOSED
F-04 CLOSED
F-05 CLOSED
F-07 CLOSED
F-06 OPEN / LOW / ACCEPTED
F-01 ACCEPTED ARCHITECTURAL LIMITATION
```

Do not reopen them without current contradictory evidence.

The purpose of this review is to discover CURRENT defects beyond the already-remediated findings.

---

# 22. Avoid False Positives

Do NOT report:

* style preferences
* hypothetical future problems
* lack of a test when the behavior is otherwise directly established
* accepted architecture merely because it differs from another design
* CS-6 merely because the harness remains unimplemented
* duplicated validation that is deliberately defense-in-depth
* the existence of caller-supplied scenario IDs by itself

A finding must have:

```text
current code
+
plausible attack/failure sequence
+
material security/integrity consequence
```

---

# 23. Required Finding Classification

For every finding classify as one of:

```text
NEW
REGRESSION
INCOMPLETE REMEDIATION
MISSED EXISTING ISSUE
ACCEPTED LIMITATION
STALE / FALSE
```

Do not call something a REGRESSION unless previously-correct behavior actually regressed.

---

# 24. Evidence Requirements

Every material finding must include:

```text
ID
Severity
Title
Location
Exact relevant code
Attack/failure sequence
Observed evidence
Reproduction command/test
Impact
Why existing controls do not stop it
Recommended remediation direction
```

For exploit claims, reproduce them against the actual current repository whenever practical.

Do not rely on stale reproduction logs from previous reviews.

---

# 25. Required Final Report

Return exactly:

# Fresh Adversarial Review — Post-Remediation

## Verdict

Choose exactly one:

* GREEN
* GREEN WITH ACCEPTED LOW FINDINGS
* CONDITIONAL GO
* NO-GO

Give one concise paragraph explaining why.

## Critical Findings

None if none exist.

## High Findings

None if none exist.

## Medium Findings

None if none exist.

## Low Findings

Include F-06 and any genuinely current LOW findings.

## Accepted Architectural Limitations

Explicitly include F-01 and any other deliberate design choices that are not defects.

## Historical Finding Status

| Finding | Status | Evidence |
| ------- | ------ | -------- |
| F-02    |        |          |
| F-04    |        |          |
| F-05    |        |          |
| F-07    |        |          |
| F-06    |        |          |
| F-01    |        |          |

## NEW-01 Verification

State whether intent ownership is now actually enforced.

Include:

```text
scenario match
belief match
action match
state match
immutability verification
```

## NEW-02 Verification

State whether:

```text
IntentOnPromoted
AuthorizeAndCreateIntent
RetractCascade
```

are safe against the previous exploit.

## NEW-03 Verification

State whether kernel + REST evidence insertion is scenario-bound.

## New Findings

Explicitly state:

```text
No new findings
```

or enumerate them.

## Verification Results

| Check          | Result |
| -------------- | ------ |
| gofmt          |        |
| go build       |        |
| go vet         |        |
| full go test   |        |
| full race test |        |
| task --list    |        |
| task db:reset  |        |
| task test      |        |
| check_i7.sh    |        |
| mcp_verify.sh  |        |

## Security Invariant Assessment

Explicitly assess:

```text
Scenario isolation
Intent ownership
Authority integrity
Action/target binding
Intent creation
Evidence integrity
Discharge atomicity
Executor integrity
Audit truthfulness
MCP boundary
DB invariants
Concurrency / TOCTOU
```

Use:

```text
PASS
FAIL
PARTIAL
ACCEPTED
```

with a short evidence statement for each.

## Final Assessment

State:

1. whether the current HEAD is ready to advance;
2. any mandatory remaining remediation;
3. any accepted LOW findings;
4. any verification limitations.

Do NOT implement fixes.

Do NOT modify the repository.

The review is successful only if it either demonstrates the current implementation is sound or produces concrete, reproducible evidence of what remains broken.

```
```
