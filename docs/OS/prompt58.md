Perform a fresh, implementation-level adversarial security and architecture review of the CURRENT Solvent repository at HEAD.

This review is AFTER:
- Plan 10.1 Exact Authority Binding
- Plan 10.1 remediation
- Debt-domain-agnostic correction / initial-debt parameterization

The current implementation is intended to be the basis for the formal Solvent kernel freeze.

REVIEW ONLY.
Do not modify code, schema, migrations, tests, documentation, or configuration.

The repository at HEAD and actual command results are authoritative.
Do not rely on implementation summaries or previous review conclusions without verifying them.

============================================================
1. PRIMARY OBJECTIVE
============================================================

Determine whether the current Solvent kernel is genuinely ready to freeze.

Specifically verify that:

A. Exact authority binding is actually closed.
B. The debt mechanism is domain-agnostic.
C. Initial debt vocabulary is caller/policy-owned rather than kernel-owned.
D. No unintended kernel semantics were introduced by the debt parameterization.
E. Existing kernel invariants remain intact.
F. No unresolved CRITICAL/HIGH/MEDIUM defect requires kernel growth.
G. Any remaining issues belong outside the kernel.

The kernel-freeze rule is:

> After freeze, new capabilities default to service, adapter, executor,
> deployment, policy, demo, or documentation layers.

A kernel change is justified only if a fresh review demonstrates:

> a genuinely new durable security fact or atomic security transition
> that cannot safely be enforced outside the kernel.

Do not reopen the kernel for convenience, cleanliness, product features,
domain semantics, or hypothetical future requirements.

============================================================
2. KERNEL FREEZE BASELINE
============================================================

Treat these as deliberate architectural properties to verify:

Generic kernel responsibilities:
- belief identity
- belief lifecycle
- generic debt/obligation lifecycle
- promotion gate
- evidence association
- structural belief relationships
- scenario isolation
- authority lifecycle
- exact authority binding
- intent lifecycle
- atomic state transitions
- database-enforced invariants

Explicitly OUTSIDE kernel:
- debt vocabulary meaning
- physics semantics
- semantic applicability
- mathematical reasoning
- policy interpretation
- human-vs-agent business rules
- swarm orchestration
- authentication/identity systems beyond existing kernel assumptions
- domain-specific review logic
- policy engines
- workflow orchestration
- idempotency infrastructure
- remote MCP features
- UI/product features

============================================================
3. EXACT AUTHORITY BINDING REVIEW
============================================================

Re-verify the complete confused-deputy defense.

Attempt:

1. Approve T1/S1.
2. Create intent I1 against T1/S1.
3. Approve a different valid target T2/S2 with compatible action/belief.
4. Attempt to execute I1 against T2/S2.

Verify:

- authorization behavior
- ClaimIntent behavior
- intent state
- executor invocation
- audit behavior

Expected:

- I1 cannot be claimed against T2/S2.
- executor is never invoked.
- intent does not enter executing state.
- no false executed/authorized event exists.

Verify the exact atomic predicate used by ClaimIntent.

It must bind the intent to the complete intended identity, including:
- intent_id
- scenario_id
- belief_id
- action
- target_id
- snapshot_id
- live state

Verify that the binding cannot be bypassed through:
- REST
- MCP
- service methods
- alternate kernel callers
- test helpers
- direct SQL paths available to the application

============================================================
4. DEBT DOMAIN-AGNOSTICITY REVIEW
============================================================

Inspect the CURRENT implementation.

Verify that:

- EnterBelief accepts caller-supplied initial debt.
- EnsureBelief accepts caller-supplied initial debt.
- The kernel does not silently inject FullDebt.
- The kernel does not validate debt names against FullDebt.
- RetireDebt treats debt identifiers generically.
- Promotion only depends on unresolved debt/cardinality, not vocabulary meaning.
- No switch/case or semantic map over named debt items exists in the kernel.
- No kernel CHECK constraint enumerates domain debt names.

Search the entire repository for:
- FullDebt
- individual debt names
- debt enums
- debt switches
- debt maps
- debt defaults
- array validation
- debt vocabulary checks

Classify each occurrence as:
- kernel mechanism
- application/policy vocabulary
- API/MCP validation
- fixture/test
- documentation
- compatibility fallback

The desired state is:

    Domain/policy
        ↓
    supplies initialDebt
        ↓
    Solvent kernel
        ↓
    stores/manages opaque obligations

The kernel must not require a particular industry's vocabulary.

============================================================
5. FULLDEBT SPECIFICALLY
============================================================

Verify whether FullDebt remains only a convenience/default vocabulary.

Determine:

- where FullDebt is defined
- which callers use it
- whether any kernel creation path still depends on it
- whether any database default silently makes it mandatory
- whether replacing FullDebt with another vocabulary requires kernel modification

Important distinction:

    FullDebt may exist in the repository.

That is NOT itself a failure.

The question is:

    Does the kernel's semantics depend on FullDebt?

Expected:

    No.

============================================================
6. ENSUREBELIEF SEMANTICS
============================================================

Inspect EnsureBelief carefully.

Verify:

- new beliefs use caller-supplied initialDebt
- existing beliefs are not silently overwritten with new debt
- find-or-create semantics remain intact
- retries do not unintentionally mutate an existing belief's debt
- no hidden fallback silently replaces caller-supplied debt

Test:

    EnsureBelief(..., debtA)
    EnsureBelief(..., debtB)

Expected existing belief:
    remains debtA

============================================================
7. DDL DEFAULT REVIEW
============================================================

Inspect all current DDL defaults and migrations involving belief.debt.

Determine:

- whether FullDebt remains a DDL default
- which code paths actually rely on it
- whether raw SQL can create a belief with deployment-specific debt
- whether supported kernel/service paths depend on that default

Distinguish:

    compatibility/defense-in-depth default
from:
    kernel domain semantics

A DDL default containing FullDebt is acceptable only if:
- it is not required by the supported domain-agnostic kernel creation path,
- its behavior is documented,
- it does not undermine the claim that initial debt is caller/policy-owned.

Do not remove it merely for aesthetic consistency.

============================================================
8. DEBT RESOURCE-BOUND REVIEW
============================================================

Verify the actual current representation.

Determine:
- TEXT[]
- JSON/JSONB
- normalized rows
- other

Verify actual mutation semantics.

The previous investigation found:
- initial set is bounded in the current implementation,
- mutation is subtractive,
- array elements are removed rather than appended.

Re-verify this at HEAD.

Determine whether parameterizing initialDebt introduced a new unbounded input path.

Ask:

    Can an untrusted caller now create arbitrarily large debt arrays?

If yes:
- determine whether this is a real security/resource issue
- determine the correct enforcement layer

Do NOT invent a number merely for discipline.

A limit belongs in the kernel only if:
- there is a concrete resource/security reason,
- and a lower layer cannot safely enforce it.

Otherwise keep it at service/API/input boundaries.

============================================================
9. NULL / EMPTY DEBT SEMANTICS
============================================================

Review:
- nil initialDebt
- empty initialDebt
- non-empty initialDebt

Determine exact behavior.

Verify that:
- nil does not accidentally mean "use FullDebt" unless deliberately documented
- empty does not create malformed state
- promotion semantics remain correct
- SQL encoding is safe
- existing callers are not silently changing behavior

Pay special attention to the difference between:
- nil slice
- empty slice
- NULL SQL value
- empty SQL array

Do not assume they are equivalent.

============================================================
10. PROMOTION INVARIANT
============================================================

Verify that the parameterization did not weaken:

    unresolved debt → promotion blocked

and:

    empty debt → debt itself no longer blocks promotion

Verify the actual DB constraint.

Do not allow a caller-supplied initialDebt of [] to bypass any other independent promotion requirements.

Confirm that promotion still relies on the existing database invariant rather than caller identity.

============================================================
11. RETIREDEBT / DISCHARGE
============================================================

Inspect the actual implementation.

Verify:

- arbitrary obligation identifiers can be stored/retired
- kernel does not attach meaning to them
- retiring a nonexistent obligation behaves consistently
- discharge remains distinct from retirement where intended
- existing audit behavior remains intact
- no domain-specific human semantics leaked into kernel

Also review the newly identified service-level issue:

- Is REST RetireDebt authorized appropriately?
- Is discharged_by tied to an authenticated principal?
- Can one authenticated principal mutate another principal's debt state?

Classify this correctly.

Do NOT call a service authorization defect a kernel defect unless it actually requires a kernel invariant.

============================================================
12. ACTOR / AUTHORIZATION BOUNDARY
============================================================

Re-check all authority-changing entry points:

- EnterBelief
- RetireDebt
- Discharge
- Promote
- AuthorizeAndCreateIntent
- ClaimIntent
- RevokeTarget
- any other authority-mutating operation

For each, determine:

1. Who can invoke it?
2. Is the caller authenticated?
3. Is the caller authorized?
4. Can another path bypass the intended policy layer?
5. Can direct DB access bypass it?

Keep these categories separate:

- I-7/static analysis
- runtime authentication
- runtime authorization
- database credential isolation
- deployment/configuration controls

Do not treat I-7 as runtime access control.

============================================================
13. POLICY BYPASS REVIEW
============================================================

Even though the kernel is frozen, check whether any higher-level policy
can currently be bypassed by calling the kernel directly.

Specifically inspect whether the repository exposes:
- direct kernel calls
- alternate internal service calls
- command-line administrative paths
- test-only paths accidentally included in production
- direct SQL write access

Do not invent a capability-token system merely because one could exist.

Only report an actual bypass or concrete architectural exposure.

============================================================
14. ACTION INTENT CREATION
============================================================

Inspect every Action Intent creation path.

Verify:
- action intent is created with correct scenario
- belief belongs to scenario
- action is coherent
- authority binding is captured
- target/snapshot provenance is correct
- caller cannot substitute authority identity after authorization
- pre-approval intent paths cannot be accidentally upgraded

Pay particular attention to IntentOnPromoted.

Verify that:
- intentionally unbound intents remain unbound
- NULL target/snapshot never acts as a wildcard
- an unbound intent cannot become authority-bound without an explicit approved creation path

============================================================
15. SCENARIO ISOLATION
============================================================

Re-run the previously fixed cross-scenario attack paths.

Attempt:
- cross-scenario belief
- cross-scenario evidence
- cross-scenario intent
- cross-scenario target
- cross-scenario snapshot
- cross-scenario debt mutation
- cross-scenario promotion
- cross-scenario execution

Verify database and kernel protections.

============================================================
16. SQL / DATABASE INTEGRITY
============================================================

Inspect all SQL touching:
- belief
- debt
- action_intent
- authority_target
- target_snapshot
- target_activation
- target_revocation
- justification
- debt_discharge

Look for:
- raw writes outside the kernel
- missing transaction boundaries
- missing scenario predicates
- incomplete foreign keys
- nullable/wildcard behavior
- bypasses through alternate SQL
- incorrect ON DELETE/UPDATE behavior

Verify I-7 still passes.

============================================================
17. RACE CONDITIONS
============================================================

Verify:
- concurrent ClaimIntent
- concurrent creation of equivalent bound intents
- concurrent debt retirement
- concurrent promotion
- concurrent retract/reliance paths
- any newly affected race introduced by initialDebt parameterization

Use the real CockroachDB-backed tests where practical.

Do not infer race safety solely from Go synchronization.

============================================================
18. PUBLIC API / MCP REVIEW
============================================================

Check whether the parameterization accidentally changes:
- REST contracts
- MCP contracts
- OpenAPI
- MCP tool schemas
- validation semantics

Determine whether callers can now submit domain-specific debt vocabulary through an interface that was intended to remain deployment-specific.

It is acceptable for internal/application callers to supply different initial debt.

Do not expose new public functionality unless it is already architecturally required.

============================================================
19. DOCUMENTATION / ARCHITECTURAL CONSISTENCY
============================================================

Check that documentation now says:

- debt vocabulary is policy/domain-owned
- FullDebt is an application/deployment vocabulary
- kernel stores generic obligation identifiers
- initial debt is caller-supplied
- promotion depends on unresolved debt, not debt names

Check that no document still implies:
- six debts are universal
- FullDebt is a kernel semantic
- debt count is a conceptual workflow limit

Also verify the kernel-freeze documentation accurately describes:
- what remains frozen
- what layers remain extensible
- what conditions would reopen the Kernel Growth Gate

============================================================
20. REQUIRED COMMANDS
============================================================

Run, where available:

    gofmt -l cmd internal kernel api service adapter
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task db:reset
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Also run targeted DB-backed tests for:
- exact authority binding
- arbitrary debt vocabulary
- empty debt
- EnsureBelief behavior
- cross-scenario isolation
- concurrent claims
- concurrent duplicate creation
- NULL/nil/empty debt behavior

Report actual command output/results.

============================================================
21. FINDING DISCIPLINE
============================================================

Do not report hypothetical concerns as findings.

For every substantive finding provide:

- ID
- severity
- exact file/symbol/line
- attack/failure path
- why existing controls fail
- concrete evidence/reproduction
- security impact
- correct layer
- whether kernel growth is actually required

Before reporting a finding, attempt to falsify it.

Classify findings as:

CRITICAL
HIGH
MEDIUM
LOW
INFO

Do not inflate severity.

A service/API authorization weakness is not a kernel defect merely because
the kernel contains the underlying operation.

A documentation inconsistency is not a security defect unless it creates
a realistic bypass or unsafe operator behavior.

============================================================
22. SPECIAL KERNEL-GROWTH TEST
============================================================

For every candidate kernel change, explicitly ask:

1. What durable security fact is missing?
2. What atomic transition is missing?
3. Why can service/policy logic not safely enforce it?
4. Why can the database already not enforce it?
5. Why would adding it to the kernel be safer than keeping it outside?
6. What attack becomes possible without it?

If these questions cannot be answered concretely:

    DO NOT recommend kernel growth.

============================================================
23. FINAL VERDICT
============================================================

Conclude with exactly one:

    GREEN
    GREEN WITH ACCEPTED LOW
    NO-GO

Use:

GREEN
- no unresolved CRITICAL/HIGH/MEDIUM defect
- debt model genuinely domain-agnostic
- exact authority binding intact
- no kernel expansion required

GREEN WITH ACCEPTED LOW
- only minor non-blocking issues remain
- no kernel expansion required

NO-GO
- real unresolved security/integrity defect exists
- or kernel domain coupling remains
- or a genuinely new atomic/durable invariant has been demonstrated

============================================================
24. FINAL OUTPUT STRUCTURE
============================================================

### 1. Verdict

### 2. Executive assessment

State whether the current Solvent kernel is genuinely ready to freeze.

### 3. Findings

Only real findings.

### 4. Exact authority binding results

Explicit PASS/FAIL.

### 5. Debt-domain-agnosticism results

Explicitly answer:

> Can a second domain use the same Solvent kernel with a different debt
> vocabulary without modifying kernel semantics?

### 6. Resource-bound results

Explicitly answer:

> Is a debt-count limit required?

### 7. Trust-boundary results

Report actual status of:
- kernel entry
- alternate mutation paths
- policy bypass
- DB writes
- deployment exposure

### 8. Verification results

List actual commands and actual outcomes.

### 9. Kernel-freeze decision

If ready:

    READY TO FREEZE

and state:

> No unresolved finding requires additional kernel primitives or schema changes.
> Future capabilities default to service, adapter, executor, deployment, policy,
> demo, or documentation layers.

If not ready:
- explain exactly what blocks freeze
- identify the minimum corrective layer
- state whether Kernel Growth Gate is actually required

This must be a fresh adversarial review of the CURRENT implementation.
Do not simply restate previous review conclusions.