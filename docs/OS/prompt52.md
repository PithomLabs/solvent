Use this prompt. It is intentionally **implementation-focused**: fix the three substantive current vulnerabilities, clean the two LOW verification defects, add regression coverage, and stop. Do not start Plan 8.6 design work yet.

````text id="6mrm6k"
# Security Remediation Pass — NEW-01 / NEW-02 / NEW-03 + Verification Cleanup

We have completed a fresh adversarial review of the current repository HEAD.

The review returned NO-GO with three substantive current security/integrity findings:

- NEW-01 — HIGH: ExecuteAction permits same-scenario intent identity substitution
- NEW-02 — HIGH: IntentOnPromoted permits cross-scenario intent creation, breaking RetractCascade
- NEW-03 — MEDIUM: AddEvidence permits cross-scenario evidence writes

It also identified two LOW verification defects:

- NEW-04 — `kernel/kernel_test.go` is not gofmt-clean, causing `task test` to fail
- NEW-05 — `scripts/mcp_verify.sh` default DSN points at an unmigrated database

The prior Plan 8.4 scenario-binding remediation is correct for RetireDebt/Promote/Discharge.
Do NOT undo or weaken it.

This is an IMPLEMENTATION pass.

## Hard constraints

- Fix the current findings.
- Do NOT begin Plan 8.6 design work.
- Do NOT perform unrelated refactors.
- Do NOT redesign the authority architecture.
- Do NOT introduce a second authorization engine.
- Keep the kernel as the smallest trusted authority core.
- Preserve all existing authority, intent, executor, audit, and scenario invariants.
- Run real commands against the actual repository.
- Add regression tests that reproduce the reported failures and prove they are closed.
- Do NOT declare success from compilation alone.
- If a kernel API/schema change is necessary, make the smallest justified change consistent with the existing kernel invariant model.

Before modifying anything, establish:

```bash
git status --short
git rev-parse HEAD
git log --oneline -10
git diff --stat
````

---

# 1. NEW-01 — EXECUTEACTION INTENT OWNERSHIP

## Vulnerability

Current behavior permits:

```text
Belief A + action "deploy"
+
live intent I_B belonging to Belief B / action "rollback"
+
same scenario
        ↓
ExecuteAction(A, "deploy", intentID=I_B)
        ↓
Authorize A succeeds
ClaimIntent(I_B) succeeds
executor runs deploy for A
I_B becomes executed
```

The current non-empty `intentID` precondition is insufficient.

The authoritative execution identity must remain bound to:

```text
(scenario_id, belief_id, action)
```

## Required invariant

An intent can be claimed for execution only when:

```text
intent.scenario_id == scenarioID
intent.belief_id   == beliefID
intent.action      == action
intent.state       == live
```

This must be enforced atomically at the intent claim transition.

## Preferred implementation

Update `kernel.ClaimIntent` so the CAS predicate itself binds the full tuple.

Conceptually:

```sql
UPDATE action_intent
SET state = 'executing'
WHERE id = $1::UUID
  AND scenario_id = $2::UUID
  AND belief_id = $3::UUID
  AND action = $4::STRING
  AND state = 'live'
```

Do NOT perform a non-atomic:

```text
SELECT intent
        ↓
compare fields
        ↓
UPDATE state
```

when the comparison can instead be part of the atomic CAS.

Update the kernel contract and all callers accordingly.

`ExecuteAction` must pass the actual:

```text
scenarioID
beliefID
action
intentID
```

to `ClaimIntent`.

## Error semantics

A mismatch must fail closed.

Do not reveal unnecessary cross-scenario information.

It is acceptable for an invalid tuple to return the existing generic claim failure/error semantics rather than distinguishing:

* nonexistent intent
* wrong scenario
* wrong belief
* wrong action

unless the current API contract already distinguishes them.

## Regression tests

Add tests for ALL of:

```text
1. Correct scenario + correct belief + correct action + live intent
   → claim succeeds

2. Same scenario + WRONG belief
   → claim fails
   → state remains live
   → executor is NOT invoked

3. Same scenario + WRONG action
   → claim fails
   → state remains live
   → executor is NOT invoked

4. Wrong scenario
   → claim fails
   → state remains live
   → executor is NOT invoked

5. ExecuteAction with mismatched intent
   → error / not allowed
   → external executor not invoked
   → intent not consumed
```

Most importantly reproduce the exact adversarial case from NEW-01:

```text
Belief A: approved for deploy
Belief A: no live intent

Belief B:
  live intent
  action = rollback

ExecuteAction(
  belief=A,
  action=deploy,
  intentID=beliefBIntent
)

Expected:
  rejected
  beliefA unchanged
  beliefB intent remains live
  executor NOT called
```

Do not merely test `ClaimIntent` in isolation. At least one integration test must exercise the actual `ExecuteAction` path.

---

# 2. NEW-02 — INTENTONPROMOTED SCENARIO ISOLATION

## Vulnerability

Current `IntentOnPromoted` can create:

```text
action_intent:
  scenario_id = scenario B
  belief_id   = belief A
```

when belief A belongs to scenario A.

Because the current relational constraints do not enforce the scenario/belief pair, this can create a cross-scenario intent.

Later:

```text
RetractCascade(scenario A, belief A)
```

does not cancel the scenario-B intent, and the surviving live intent can violate:

```text
live_requires_promoted
```

when the belief is retracted.

Observed result:

```text
SQLSTATE 23514
RetractCascade rolls back
belief cannot be retracted
```

## Required invariant

For every `action_intent`:

```text
action_intent.scenario_id
AND
action_intent.belief_id
```

must refer to the SAME scenario.

An intent may never be created for a belief owned by another scenario.

## Required implementation

In `createIntentWithinTx` / `IntentOnPromoted`:

Before inserting the intent, verify inside the SAME transaction:

```sql
SELECT EXISTS(
  SELECT 1
  FROM belief
  WHERE id = $1::UUID
    AND scenario_id = $2::UUID
)
```

If false:

```text
return ErrBeliefNotFound
```

or the repository's equivalent scoped/not-found error.

The validation MUST occur inside the same `crdb.ExecuteTx` transaction as the INSERT.

Do not validate outside the transaction and then insert later.

Preserve existing "must be promoted" behavior.

Do not remove or weaken the existing FK/check constraints.

## Consider schema-level protection

Inspect whether the current schema can cheaply enforce the invariant structurally with an appropriate composite uniqueness/FK constraint.

Do NOT automatically add a schema migration.

The project has a high bar for schema/kernel growth.

For this remediation, prefer the minimal transaction-scoped invariant enforcement unless the existing schema already has the natural composite key structure and adding the constraint is clearly safe and justified.

If you discover that a schema-level constraint is clearly the safer minimal solution, document why before applying it.

## Regression tests

Reproduce the exact exploit:

```text
scenario A owns belief A

attempt:
IntentOnPromoted(
    scenarioB,
    beliefA,
    action
)

Expected:
  error
  zero action_intent rows created
```

Then prove the downstream invariant:

```text
scenario A owns promoted belief A
cross-scenario intent creation attempt fails

RetractCascade(scenarioA, beliefA)
→ succeeds
→ belief becomes retracted
→ no SQLSTATE 23514
```

Also retain a normal positive test:

```text
IntentOnPromoted(scenarioA, beliefA, action)
→ succeeds
→ intent created correctly
```

If there is any possibility of an inconsistent legacy intent row already existing, inspect how migration/upgrade behavior handles it. Do not invent cleanup behavior without evidence; report it separately if discovered.

---

# 3. NEW-03 — ADD EVIDENCE SCENARIO ISOLATION

## Vulnerability

`POST /v1/evidence` currently allows:

```text
scenario B
belief belonging to scenario A
```

and successfully inserts:

```text
evidence.scenario_id = B
evidence.belief_id   = A
```

This violates the same scenario-bound object invariant.

## Required invariant

Every evidence row must satisfy:

```text
evidence.scenario_id == belief.scenario_id
```

## Required implementation

Apply the same architectural pattern used for the other repaired mutation paths.

At minimum:

* validate `scenario_id`
* validate the belief belongs to that scenario
* prevent insertion when it does not

Prefer enforcement in the kernel transaction boundary because the kernel already receives both:

```text
scenarioID
beliefID
```

The REST handler may also retain a handler-level guard for clear error semantics / defense-in-depth.

Do not create a second evidence-specific authorization system.

## Regression tests

Add:

```text
REST:
  scenario A + belief A
  → evidence creation succeeds

REST:
  scenario B + belief A
  → rejected
  → HTTP error
  → zero evidence rows created

Kernel:
  AddEvidence(scenarioB, beliefA, ...)
  → rejected
  → zero evidence rows created
```

Verify that the rejected request cannot leave partial rows.

Also verify existing legitimate evidence-ingestion behavior is unchanged.

---

# 4. NEW-04 — FORMAT REGRESSION

Run:

```bash
gofmt -l cmd internal kernel
```

Fix formatting in `kernel/kernel_test.go` and any other file reported by the command.

Do not make unrelated formatting changes across the repository.

Then verify:

```bash
task test
```

---

# 5. NEW-05 — MCP VERIFICATION HARNESS

Inspect:

```text
scripts/mcp_verify.sh
Taskfile.yml
DB migration layout
```

Current reported problem:

```text
scripts/mcp_verify.sh
    ↓
default DSN
    ↓
fable
    ↓
database lacks migrations 005-008
```

while the authority-migrated test DB passes all 34 MCP assertions.

Determine the intended test database configuration from the repository itself.

Fix the smallest configuration/setup issue necessary so that:

```bash
bash scripts/mcp_verify.sh
```

works from the documented/default test environment after:

```bash
task db:reset
```

Do NOT silently point the script at an arbitrary personal database.

If the intended architecture is that `task db:reset` must apply migrations 001-008, correct that.
If the intended architecture is a dedicated MCP test DSN, correct the harness accordingly.

Use the repository's established conventions.

Then verify:

```bash
task db:reset
bash scripts/mcp_verify.sh
```

returns success.

---

# 6. SEARCH FOR RELATED SCENARIO-PAIRING GAPS

Because NEW-02 and NEW-03 reveal a broader invariant class, perform a targeted repository audit before stopping.

Search for all code where both:

```text
scenario_id
```

and one of:

```text
belief_id
target_id
intent_id
activation_id
revocation_id
```

are accepted or written.

Examples:

```bash
grep -Rnw --include='*.go' \
  -e 'scenarioID' \
  -e 'ScenarioID' \
  -e 'scenario_id' .
```

Then inspect mutation paths involving:

```text
belief
evidence
action_intent
target
target_activation
target_revocation
debt_discharge
```

The goal is NOT to launch a broad refactor.

The goal is to determine whether NEW-02/NEW-03 are isolated omissions or instances of another immediately exploitable scenario-pairing defect.

If another CURRENT security-relevant gap is found, report it and fix it only if it is clearly within the same invariant class and can be corrected without architectural redesign.

Do NOT silently expand scope indefinitely.

---

# 7. AUDIT THE EXECUTEACTION CONTRACT END-TO-END

After NEW-01 is fixed, inspect:

```text
PrepareForAction
Authorize
ClaimIntent
executor invocation
CompleteIntent
ReconcileIntent
```

Confirm the execution tuple remains coherent throughout:

```text
scenarioID
beliefID
action
targetID
intentID
```

Specifically verify:

```text
intentID cannot be substituted
beliefID cannot be substituted
action cannot be substituted
target remains bound to authoritative snapshot
```

Do not introduce caller-provided execution parameters into the authority source of truth.

---

# 8. PRESERVE EXISTING INVARIANTS

Do NOT break:

```text
kernel.Authorize remains authority oracle
TOKEN != AUTHORITY
Authorize != Execute
snapshot parameters remain authoritative
fixed action → executor mapping
ClaimIntent remains atomic CAS
ambiguous provider outcome remains executing
CompleteIntent source-state guard
REST authenticated principal
MCP trusted-local stdio model
GitHub semantics outside kernel
no second authorization engine
```

Do not revive removed workflow-token infrastructure.

Do not add general-purpose tenant isolation.

Do not redesign MCP authentication.

---

# 9. TEST REQUIREMENTS

Add or update tests so the following matrix is executable:

## Scenario isolation

```text
               Same scenario       Wrong scenario
RetireDebt        PASS                 REJECT
Promote           PASS                 REJECT
Discharge         PASS                 REJECT
AddEvidence       PASS                 REJECT
IntentOnPromoted  PASS                 REJECT
```

For each rejected case:

```text
error
zero durable mutation
no misleading success audit
```

## Intent ownership

```text
scenario + belief + action all match
    → claim succeeds

scenario matches, belief differs
    → claim rejected

scenario matches, action differs
    → claim rejected

scenario differs
    → claim rejected
```

## Retract safety

```text
cross-scenario intent creation attempt
    → rejected
    → no foreign intent exists

retract victim belief
    → succeeds
    → no 23514
```

## Evidence

```text
same-scenario evidence
    → succeeds

cross-scenario evidence
    → rejected
    → no row
```

## Discharge

Preserve the existing tests proving:

```text
cross-scenario discharge
    → zero writes

same-scenario discharge
    → atomic success
```

---

# 10. VERIFICATION

Run all relevant checks:

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

Also run focused tests for:

```text
NEW-01
NEW-02
NEW-03
```

If practical, run the exact standalone exploit reproductions from the adversarial review again and demonstrate that they now fail.

Do not report "PASS" for anything that did not actually run.

---

# 11. FINAL SECURITY CHECK

After implementation and tests, inspect the final diff:

```bash
git diff --stat
git diff
git status --short
```

Check for:

* accidental unrelated changes
* weakened checks
* test-only bypasses
* hidden default changes
* incorrect error semantics
* duplicated authorization logic
* caller paths that still omit scenario context
* stale comments contradicting implementation

---

# 12. REQUIRED FINAL REPORT

Return:

## Changes Made

List every modified file and the security purpose.

## NEW-01

* Root cause
* Fix
* Tests
* Exact exploit reproduction result

## NEW-02

* Root cause
* Fix
* Tests
* RetractCascade reproduction result

## NEW-03

* Root cause
* Fix
* REST + kernel test results

## NEW-04

* Formatting fix
* `task test` result

## NEW-05

* Harness/config fix
* `mcp_verify.sh` result

## Related Scenario-Pair Audit

State whether additional current scenario-pairing gaps were found.

If found, list them explicitly.
If none, say so.

## Verification Results

Use:

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

## Security Disposition

State explicitly:

```text
NEW-01: CLOSED / OPEN
NEW-02: CLOSED / OPEN
NEW-03: CLOSED / OPEN
NEW-04: CLOSED / OPEN
NEW-05: CLOSED / OPEN
F-01: ACCEPTED ARCHITECTURAL LIMITATION
F-06: OPEN / LOW / ACCEPTED
```

## Remaining Issues

List anything genuinely unresolved.

Do NOT invent issues just to make the report look thorough.

---

## STOP CONDITION

Stop after:

1. NEW-01 is fixed and behaviorally verified.
2. NEW-02 is fixed and the RetractCascade exploit is behaviorally verified closed.
3. NEW-03 is fixed and REST + kernel behavior is verified.
4. NEW-04 and NEW-05 are fixed.
5. Full verification is run.
6. Final diff is inspected.

Do NOT proceed into Plan 8.6.
Do NOT perform another architectural redesign.

```

One important detail in this prompt is that **NEW-01 is fixed at the `ClaimIntent` CAS boundary**, not by adding another pre-check in `ExecuteAction`. That keeps the intent ownership invariant atomic and puts the security property at the state transition that actually consumes the intent. The adversarial review's reproduction establishes that this is the missing binding. :contentReference[oaicite:0]{index=0} :contentReference[oaicite:1]{index=1}

Likewise, NEW-02 and NEW-03 are treated as instances of the broader `(scenario_id, object_id)` integrity invariant rather than isolated endpoint bugs. The review's CockroachDB reproductions support that framing. :contentReference[oaicite:2]{index=2} :contentReference[oaicite:3]{index=3}
```
