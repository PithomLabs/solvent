Use this as the fresh-review prompt:

````text
# Fresh Independent Adversarial Review — Solvent HEAD

You are performing a fresh, independent adversarial security and architecture review of the CURRENT repository HEAD.

Do NOT implement fixes.
Do NOT modify source code, tests, plans, configuration, or documentation.
Do NOT merely review planning documents and infer repository state.
Run real commands against the actual repository and inspect the actual current source.
Treat the repository, executable behavior, tests, and command output as the primary evidence.

## Review context

HEAD is:

    5933f44 ✨ phase 4e

Plan 8 and Plan 8.1 contain the historical evidence reconciliation and subsequent hardening work.

Important dispositions already established:

- F-02: CLOSED
  Cross-scenario validation occurs before mutation in `handleSolventRetireDebt` and `handleSolventPromote`.
  Raw source evidence and git-history evidence are recorded in Plan 8.

- F-04: CLOSED
  The current I-7 MCP boundary gate is functional.
  The older inert-grep finding applied to an earlier implementation.

- F-05: CLOSED
  The refusal/error path goes through `envelopeErrorResult`, which performs the audit.

- F-07: CLOSED
  Current Taskfile parses successfully.

- F-06: OPEN / LOW / ACCEPTED
  MCP argument handling uses interface/type assertions that can silently produce zero values for wrong runtime types, but downstream validation mitigates immediate consequences.

- F-01: ACCEPTED ARCHITECTURAL LIMITATION
  MCP is intentionally a trusted local stdio administrative surface.
  `actor_id` is attribution, not authentication.
  External mechanisms such as SSH forwarding, container forwarding, or IDE forwarding can bridge stdio without the process knowing.
  This is an operational/deployment trust assumption, not something claimed to be solved by an in-process check.

- `MCP_TRANSPORT` hardening:
  The binary defaults to stdio and rejects unsupported configured transport values.
  This is FUTURE-TRANSPORT HARDENING.
  It is NOT considered closure of F-01.

Do not re-litigate the historical findings merely for completeness.
However, they remain challengeable if your inspection finds concrete contradictory evidence.
Any NEW or DISTINCT issue in the same code remains fully in scope.

## Primary objective

Try to BREAK the system.

Assume that an attacker, malicious agent, confused agent, compromised integration, malformed provider response, stale authorization, incorrect caller, or incorrectly deployed component is actively trying to cause an unauthorized consequential action.

Look for defects that allow:

- unauthorized authority creation
- unauthorized authorization
- bypass of actor restrictions
- confused-deputy behavior
- cross-scenario or cross-tenant access
- action/target mismatch
- stale or revoked authority to remain usable
- replay or double execution
- concurrent execution races
- TOCTOU exploitation
- executor substitution
- provider-response ambiguity mishandling
- audit falsification or audit gaps
- MCP trust-boundary bypass
- mutation through supposedly read-only paths
- malformed or attacker-controlled input to alter security decisions
- failure-open behavior
- error-path inconsistencies
- security invariants enforced only at the UI/service layer but bypassable elsewhere
- DB isolation/invariant failures
- dangerous assumptions introduced by Phase 4D/4E
- architectural drift that undermines the kernel/service/executor trust model

## Required inspection

Start by establishing actual repository state:

```bash
git status --short
git rev-parse HEAD
git log --oneline -12
find . -maxdepth 3 -type f | sort
````

Then inspect the authority-critical paths and surrounding code, including at minimum:

```text
kernel/
service/authority/
service/executor/
service/policy/
service/audit/
internal/view/
cmd/solvent-mcp/
adapter/
api/
DB schema/migrations
scripts/
examples/github/
```

Trace end-to-end flows rather than reviewing functions in isolation:

```text
principal/authentication
    ↓
request / MCP tool
    ↓
policy / action validation
    ↓
belief / target lookup
    ↓
authority verification
    ↓
intent lifecycle
    ↓
claim / execution
    ↓
executor selection
    ↓
provider call
    ↓
provider outcome
    ↓
intent completion / reconciliation
    ↓
audit
```

## Security properties to challenge

Explicitly test or reason from code about:

### 1. Authority correctness

Verify that:

* `Approve` cannot create authority for the wrong scenario/target/action.
* `Authorize` cannot authorize stale, revoked, mismatched, or nonexistent authority.
* exact action/target/consequence binding is preserved.
* caller-supplied execution parameters cannot override frozen authoritative state.
* actor type, actor identity, and authentication are not confused.
* MCP input cannot manufacture authority merely by claiming to be HUMAN or another principal.

### 2. Scenario / isolation boundaries

Try cross-scenario combinations for every relevant read and write path.

Do not limit this to the historical R-3 handlers.

Check:

* beliefs
* debt
* targets
* activations
* revocations
* intents
* audit
* activity/read models
* executor state
* MCP tools
* REST endpoints

Look for authorization or mutation paths where scenario identity is checked too late, omitted, inferred from mutable state, or checked inconsistently.

### 3. Intent lifecycle and races

Review:

```text
live → executing → executed
executing → live     (definitive rejection)
executing → executing (ambiguous; reconciliation required)
live → cancelled
```

Verify:

* exactly one concurrent claim can win
* cancelled intents cannot later execute
* executed intents cannot execute again
* ambiguous provider outcomes do not incorrectly become success or failure
* reconciliation is fail-safe
* retries cannot bypass authorization or intent state
* authorization and claim sequencing does not create an exploitable security transition

Pay special attention to the known accepted TOCTOU windows:

```text
Authorize → ClaimIntent
ClaimIntent → external provider call
```

Do not merely restate that these windows exist.
Determine whether either window creates a new exploitable security property beyond the already accepted design.

### 4. Executor integrity

Verify:

* callers cannot choose an arbitrary executor through request data
* action names map to fixed trusted executors
* executor code cannot approve/promote/revoke/create authority
* execution parameters come from authoritative/frozen state
* provider responses cannot fabricate successful execution
* provider ambiguity is fail-safe
* provider failures cannot accidentally transition to executed

### 5. MCP boundary

Review:

* every MCP tool
* all caller-controlled arguments
* all read/write paths
* actor attribution
* scenario isolation
* error handling
* audit behavior
* direct DB access
* transport startup behavior

Treat the intended deployment assumption as known, but ask whether the implementation accidentally violates it in some OTHER way.

The question is NOT:
"Can the process detect SSH stdio tunneling?"

That is already accepted as an architectural limitation.

The question IS:
"Given the trusted-local-stdio assumption, does the implementation faithfully enforce the intended boundary everywhere it can?"

### 6. Database invariants

Inspect schema and SQL for whether security-critical invariants are enforced structurally.

Look for:

* missing uniqueness constraints
* race-prone check-then-write logic
* scenario IDs omitted from queries
* updates/deletes insufficiently scoped
* state transitions that can be bypassed
* mutable authoritative state that should be frozen
* transaction boundaries that permit inconsistent intermediate states

Prefer DB invariants where the invariant is truly durable and security-critical.

### 7. Audit integrity

Check:

* authorization events
* execution events
* provider outcomes
* reconciliation
* refusal/error paths
* whether audit failures are silently discarded
* whether audit can claim success when execution is ambiguous/failed
* whether audit records are scoped to the correct scenario/intent/action

Distinguish:
`Authorize != Execute`
and
`Provider accepted != Execution durably completed`
where applicable.

### 8. Error paths / fail-closed behavior

For every security-sensitive operation, inspect:

* nil state
* missing records
* malformed input
* wrong types
* DB errors
* serialization errors
* provider errors
* context cancellation
* partial failures
* audit failures

Look specifically for paths where an error causes the system to proceed with an empty/default value.

### 9. API/MCP consistency

Compare REST and MCP surfaces.

Look for:

* different authorization semantics
* different scenario checks
* different actor handling
* inconsistent validation
* one surface exposing a capability that another properly protects
* undocumented privileged endpoints
* dead or legacy paths still reachable

### 10. Kernel boundary / architectural drift

Re-evaluate the project's core principle:

> Keep the kernel as the smallest trusted authority core.

Look for:

* provider-specific semantics leaking into the kernel
* executor-specific behavior inside the kernel
* policy that belongs in service being embedded in the kernel
* duplicate authority logic outside the kernel
* security-critical state transitions happening outside the kernel without justification
* convenience abstractions that accidentally weaken the authority model

Do not recommend new kernel primitives unless you can demonstrate that the security property genuinely cannot be expressed outside the kernel.

## Evidence requirements

For every finding:

1. Show the exact file and line range.
2. Quote only the minimum relevant code.
3. Explain the concrete attack or failure sequence.
4. Reproduce it with a test/command where practical.
5. Distinguish observed behavior from inferred risk.
6. State whether the issue exists at CURRENT HEAD.
7. Check git history when useful to determine whether it is current, stale, or already fixed.

Do not count:

* style issues
* speculative future problems
* documentation preferences
* theoretical concerns with no plausible security consequence

unless they materially affect the trust model.

## Independent-review discipline

Do not anchor on Plan 8 or Plan 8.1.
Do not assume their conclusions are correct merely because they contain raw evidence.
Use them as context, then inspect HEAD yourself.

At the same time, do not mechanically repeat historical findings just because they appear in previous reviews.

The goal is not to maximize the number of findings.
The goal is to identify every CURRENT material defect and reject false/stale claims.

## Verification commands

Run as much of the following as the repository permits:

```bash
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -race -count=1 -p 1 ./...
task --list
task test
bash scripts/check_i7.sh
bash scripts/mcp_verify.sh
```

Also run focused tests for any suspicious path you discover.

If CockroachDB-dependent tests cannot run because the DB is unavailable, explicitly distinguish:

* code/test failure
* test-environment failure
* unavailable evidence

Do NOT call an unavailable test PASS.

## Required final report

Return exactly this structure:

# Fresh Adversarial Review

## Verdict

One of:

* GO
* GO WITH ACCEPTED LOW FINDINGS
* NO-GO

Give a one-paragraph justification.

## Critical Findings

Only Critical severity findings.

For each:

* ID
* Severity
* Location
* Attack / failure sequence
* Evidence
* Reproduction
* Impact
* Recommended direction

## High Findings

Same structure.

## Medium Findings

Same structure.

## Low Findings

Same structure, but do not inflate minor quality issues into security findings.

## Accepted Architectural Limitations

Explicitly list accepted assumptions that are NOT defects, including F-01 if still appropriate.

## Historical Finding Reconciliation

Briefly state whether F-02/F-04/F-05/F-07 remain consistent with current HEAD.
Do not re-derive them unless contradictory evidence was found.

## Verification Results

Include actual command results and clearly separate:

* PASS
* FAIL
* BLOCKED BY ENVIRONMENT
* NOT RUN

## New Findings

Explicitly state whether the review discovered any issue not already covered by the historical reviews.

## Overall Assessment

State whether the current HEAD is safe to advance to the next phase and what, if anything, must happen first.

Do not implement any fixes.

```

One change from the earlier version is deliberate: **the reviewer is not forbidden from re-checking F-02/F-04/F-05/F-07, but is told not to waste effort re-deriving them absent contradictory evidence.** That preserves the anti-anchoring discipline you described while keeping the review focused.
```
