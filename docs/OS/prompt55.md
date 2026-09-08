Perform a fresh, implementation-level adversarial security review of the CURRENT repository after completion of Plan 10.1 (Exact Authority Binding).

This is a REVIEW ONLY.

Do not modify code, schema, tests, documentation, or configuration.
Do not propose implementation work until you have established whether a real defect exists.
Do not rely on the implementation summary alone. The repository at HEAD and actual command results are authoritative.

## Review objective

Determine whether Plan 10.1 actually closes the confused-deputy vulnerability it was designed to close:

    intent created under authority T1/S1
        ↓
    later execution presented with authority T2/S2
        ↓
    execution must be impossible

The final security invariant should be:

    an action_intent can be claimed only against
    the exact (target_id, snapshot_id) authority
    captured when that intent was created.

The database must enforce the durable relationship, and the atomic ClaimIntent CAS must enforce the execution-time binding.

## Architectural boundary

Treat the current kernel freeze decision as intentional.

The kernel is now considered frozen after Plan 10.1 unless this review discovers a genuinely new kernel-level security defect.

Do NOT recommend kernel expansion merely for:
- convenience
- cleaner APIs
- better error messages
- product features
- swarm support
- harness support
- idempotency
- remote MCP
- authentication
- observability
- reporting
- packaging
- deployment convenience

Any finding must first be classified as:
- existing defect
- regression introduced by Plan 10.1
- genuinely new security invariant
- documentation/verification weakness
- acceptable architectural limitation

Only a genuinely new atomic/durable security invariant that cannot be enforced outside the current kernel justifies reopening the Kernel Growth Gate.

## Primary attack paths

Attempt to break ALL of the following.

### 1. T1 → T2 confused deputy

Construct or reason through:

1. Approve T1/S1.
2. Create intent I1 against T1/S1.
3. Revoke T1.
4. Approve T2/S2 with the same belief/action or otherwise compatible tuple.
5. Attempt to execute I1 using T2/S2.

Expected:

- authorization does not accidentally attach I1 to T2/S2
- ClaimIntent cannot consume I1 under T2/S2
- executor is never invoked
- no false executed/authorized audit event appears

Prefer an actual repository regression test or a direct executable reproduction where feasible.

### 2. Same target, different snapshot

Attempt:

    intent → T1/S1
    execute → T1/S2

Even though the current schema makes multiple snapshots for one target impossible through normal activation, verify both:
- application-level rejection
- DB-level protection

Do not dismiss this merely because normal code paths make it impossible.

### 3. Cross-target / cross-snapshot corruption

Attempt to insert:

    target_id = T1
    snapshot_id = S2

where S2 belongs to T2.

Also attempt:

    target_id = valid T1
    snapshot_id = nonexistent UUID

Expected:

- composite FK rejects both
- no invalid action_intent row survives

Verify the actual migration and actual database behavior.

### 4. NULL / pre-authority intent path

Verify the NULL-safe ClaimIntent implementation carefully.

There are now two conceptually different paths:

A. `IntentOnPromoted`
   - target_id/snapshot_id may be NULL

B. `AuthorizeAndCreateIntent`
   - target_id/snapshot_id must be concrete

Verify:

- IntentOnPromoted still behaves correctly
- NULL target/snapshot does not accidentally become a wildcard
- an unbound intent cannot be claimed as an authority-bound intent
- a bound intent cannot be claimed using NULL values
- SQL three-valued logic does not create an authorization bypass

Do not merely inspect the SQL. Exercise the behavior where practical.

### 5. Intent identity substitution

Attempt every substitution dimension:

    intent A + belief B
    intent A + action X
    intent A + target T
    intent A + snapshot S
    intent A + scenario C

Try to execute an intent using mismatched values.

Expected:

- atomic ClaimIntent prevents the mismatch
- no executor invocation
- no state transition to executing

Pay particular attention to whether any API/service layer trusts caller-supplied identity after an earlier authorization step.

### 6. Claim race

Verify that two concurrent callers cannot both claim the same intent.

Expected:

- exactly one transition live → executing
- at most one external executor invocation path

Use the real database and race detector where applicable.

### 7. Creation race / duplicate intent

Two concurrent calls create the same:

    scenario
    belief
    action
    target
    snapshot

Expected:

- exactly one live intent survives
- exactly one successful creation
- the other receives `ErrDuplicateIntent`
- duplicate conflict is NOT treated as authorization denial
- no false `ActivityAuthorizationDenied`

Verify that `23505` handling is correctly scoped and cannot hide unrelated unique violations.

### 8. Composite-FK interaction with NULL

Because the migration intentionally allows nullable legacy intent fields, verify that:

- `(NULL, NULL)` legacy rows can exist where intended
- `(NULL, concrete snapshot)` and `(concrete target, NULL)` behave correctly
- such rows remain permanently unclaimable by the authority-bound ClaimIntent path
- the composite FK does not accidentally create a wildcard relationship

### 9. Snapshot provenance

Trace `snapshot_id` from:

    target_activation
        → target_snapshot
        → AuthorizeResult
        → ExecuteAction
        → ClaimIntent

Verify there is no point where caller-supplied snapshot identity can replace DB-derived snapshot identity.

Pay special attention to REST and MCP handlers.

### 10. Consequence parameters

Verify that executor parameters still come exclusively from the approved snapshot.

Attempt:

- correct target + malicious caller parameters
- correct intent + different params
- T1 intent + T2 parameters

Expected:

- caller parameters cannot redirect execution
- executor receives only approved snapshot parameters

Do not confuse consequence-parameter binding with exact authority binding; review both independently.

### 11. API/MCP error semantics

Verify the distinction between:

    authorization denial
    vs.
    intent/state conflict

Specifically:

- duplicate equivalent intent → 409 REST
- duplicate equivalent intent → MCP conflict error
- not an authorization denial
- no `ActivityAuthorizationDenied` generated for duplicate conflict

Also verify that `ErrIntentNotLive` remains the honest result for claim-time mismatches and that diagnostic classification cannot alter the security decision.

### 12. Diagnostic-read TOCTOU

Inspect the new post-CAS diagnostic SELECT.

Prove that:

- the CAS is still the authoritative security decision
- the diagnostic read cannot turn a failed claim into a successful claim
- a concurrent state change cannot cause the diagnostic result to be used as authorization
- diagnostic messaging does not create a security bypass

### 13. Intent mutation

Search the entire repository for any UPDATE path capable of changing:

    action_intent.target_id
    action_intent.snapshot_id
    action_intent.belief_id
    action_intent.action
    action_intent.scenario_id

after creation.

The intended security model depends on these identity fields remaining immutable.

If the repository permits any mutation, determine whether it creates a real exploit.

### 14. IntentOnPromoted bypass

Review `IntentOnPromoted` extremely carefully.

This path intentionally creates pre-approval intents with NULL authority binding.

Determine whether such an intent can later become executable through:

- a later authority approval
- a service-layer lookup
- NULL comparison behavior
- manually supplied target/snapshot values
- REST/MCP execution paths

Expected: there must be no path that upgrades an unbound intent into an authority-bound intent implicitly.

### 15. Scenario isolation

Re-test Plan 9.0's scenario isolation fixes while reviewing Plan 10.1.

Attempt:

- cross-scenario target
- cross-scenario snapshot
- cross-scenario intent
- cross-scenario evidence
- cross-scenario execution

Do not assume previous fixes remain correct simply because this change was narrower.

### 16. Existing security invariants

Regression-check:

- `promoted_is_debt_free`
- `gate`
- `live_requires_promoted`
- scenario isolation
- intent ownership
- ClaimIntent CAS
- AuthorizeAndCreateIntent atomicity
- RevokeTarget
- target snapshot immutability
- fixed executor mapping
- provider ambiguity handling
- ReconcileIntent audit ordering
- MCP no-direct-write invariant
- TOKEN != AUTHORITY

Look specifically for regressions introduced by the new migration and SQL changes.

## Repository inspection requirements

Before reaching a conclusion:

1. Inspect the actual current implementation.
2. Search all callers of:
   - `ClaimIntent`
   - `AuthorizeAndCreateIntent`
   - `IntentOnPromoted`
   - `ExecuteAction`
3. Search all SQL touching `action_intent`.
4. Inspect migration 009 exactly as applied.
5. Verify the actual composite FK in the live database.
6. Verify the actual unique index.
7. Inspect REST and MCP snapshot-ID retrieval.
8. Inspect API/MCP error mappings.
9. Inspect audit paths around authorization, claim, conflict, and execution.
10. Inspect git diff/history around the Plan 10.1 implementation where useful to distinguish new defects from pre-existing behavior.

Do not accept comments or documentation as proof where executable evidence can be obtained.

## Required verification

Run, where the repository/environment permits:

    gofmt -l cmd internal kernel
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Also run:

    task db:reset

and verify all migrations, including 009, actually apply.

For DB-backed security findings, prefer actual CockroachDB-backed tests over mocks.

## Severity standard

Use:

- CRITICAL: direct authority bypass with consequential execution possible
- HIGH: exploitable integrity/security defect that can cause unauthorized consequential action or durable corruption
- MEDIUM: meaningful security/control defect requiring specific conditions, but not direct arbitrary authority bypass
- LOW: limited weakness, verification/documentation gap, or defense-in-depth issue
- INFO: observation with no actionable security impact

Do not inflate severity.

Do not downgrade a real exploit merely because it requires a malicious caller.

## Important review discipline

Do not assume that all intended Plan 10.1 properties are correct simply because:
- tests pass
- the implementation summary says they pass
- the plan says they should pass

Conversely, do not manufacture findings merely because some theoretical edge case is conceivable.

Every substantive finding must contain:

1. exact code/schema location
2. attack or failure path
3. why existing controls fail
4. concrete reproduction/evidence where feasible
5. security impact
6. severity
7. whether kernel growth is actually required

For every candidate finding, attempt to falsify it before reporting it.

## Final disposition

Conclude with exactly one:

    GREEN
    GREEN WITH ACCEPTED LOW
    NO-GO

Use NO-GO only for real unresolved CRITICAL/HIGH/MEDIUM security defects.

If GREEN or GREEN WITH ACCEPTED LOW, explicitly state whether the evidence is sufficient to proceed to the formal kernel freeze.

If any issue is found that would reopen the kernel, explicitly explain why the existing service/adapter/DB boundaries cannot safely enforce it and whether a new Kernel Growth Gate is actually required.

## Final output structure

### 1. Verdict

One of GREEN / GREEN WITH ACCEPTED LOW / NO-GO.

### 2. Executive assessment

Brief assessment of whether Plan 10.1 actually closes the confused-deputy gap.

### 3. Findings

For each real finding:

- ID
- Severity
- Location
- Reproduction/evidence
- Impact
- Required disposition

Do not include speculative findings in this section.

### 4. Attack-path results

Report PASS/FAIL for:

- T1 → T2 substitution
- T1/S1 → T1/S2
- composite FK corruption
- NULL/unbound intent bypass
- belief/action/target/snapshot/scenario substitution
- concurrent claim
- concurrent duplicate creation
- parameter substitution
- REST/MCP conflict semantics
- diagnostic-read race
- IntentOnPromoted bypass
- scenario isolation
- invariant regressions

### 5. Verification results

Report the actual commands and outcomes.

Do not say "tests pass" without stating what was actually executed.

### 6. Kernel-freeze recommendation

State one of:

    READY TO FREEZE

or

    NOT READY TO FREEZE

If READY TO FREEZE, explicitly confirm:

> No unresolved finding requires additional kernel primitives or schema changes. Future capabilities should default to service, adapter, executor, deployment, policy, demo, or documentation layers.

Remember: this is a fresh adversarial review of the implementation, not a restatement of Plan 10.1.