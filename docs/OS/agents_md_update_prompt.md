CORE ARCHITECTURAL DECISION
==================================================

The invariant is:

    DEBT IS OPAQUE TO SOLVENT.

Solvent:
- stores debt as TEXT[];
- treats debt identifiers as opaque;
- knows only empty vs non-empty;
- permits arbitrary non-empty debt strings;
- allows retirement of opaque identifiers;
- gates promotion on debt being empty;
- does NOT enumerate, validate, normalize, lowercase, trim, or interpret debt identifiers.

The domain/application layer owns debt vocabulary and its semantic validation.

Do NOT replace the removed six-item Solvent vocabulary with any new universal vocabulary.

Do NOT introduce:
- ebp:* defaults;
- generic Solvent debt defaults;
- a universal debt enum;
- a debt table;
- a debt vocabulary table;
- a Solvent taxonomy;
- namespace enforcement;
- arbitrary length/count restrictions in Solvent.

The database default is intentionally:

    ARRAY[]::TEXT[]

==================================================
1. RESOLVE PACKAGE DEPENDENCY FIRST
==================================================

Before finalizing the plan, inspect the actual Go import graph.

The intended design is:

    belief package
        owns wizard debt vocabulary
        owns DebtMapping

    wizard package
        consumes belief-owned vocabulary

However, verify whether internal/belief imports internal/wizard.

If:

    belief -> wizard

already exists, then wizard -> belief would create a cycle.

If no cycle exists:
    keep vocabulary in internal/belief/debt.go.

If a cycle would exist:
    create the smallest appropriate shared domain package,
    e.g. internal/domain/debt.go,
    containing only the wizard vocabulary constants.

Do NOT duplicate the vocabulary merely to avoid the package-cycle issue.

The final plan MUST record which branch actually exists in the repository.

==================================================
2. SINGLE SOURCE OF TRUTH FOR WIZARD VOCABULARY
==================================================

The repository inspection established that:
- mapping.go has its own six literals;
- belief creation separately used kernel.FullDebt;
- both contained the same six strings.

The final design must eliminate that duplication.

Use named constants, NOT positional slice indexes.

Preferred structure:

    const (
        NeedProvenanceCheck    = "needProvenanceCheck"
        NeedContradictionSweep = "needContradictionSweep"
        NeedBlastRadius        = "needBlastRadius"
        NeedRollbackPlan       = "needRollbackPlan"
        NeedVersionPin         = "needVersionPin"
        NeedOperatorSignoff    = "needOperatorSignoff"
    )

and:

    var wizardDebt = []string{
        NeedProvenanceCheck,
        NeedContradictionSweep,
        NeedBlastRadius,
        NeedRollbackPlan,
        NeedVersionPin,
        NeedOperatorSignoff,
    }

DebtMapping must reference the named constants directly.

Do NOT use:

    wizardDebt[0]
    wizardDebt[1]

etc.

Do NOT retain the alternative of duplicated literals with comments.

That alternative is rejected.

==================================================
3. WIZARD API / ACCESS
==================================================

If wizard is a different package from the vocabulary owner:

- expose a safe accessor such as:

    func WizardDebt() []string

- return a defensive copy;
- never expose the mutable backing slice.

Do not leave conditional language such as:
"export accessor if needed"
or
"wizard can define its own copy."

The final plan must make exactly one decision.

==================================================
4. SOLVENT SCHEMA
==================================================

Do not modify historical 001_schema.sql if inspection confirms it is always superseded by 004.

Add the additive migration:

    db/010_debt_opaque.sql

with:

    ALTER TABLE belief
      ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];

Preserve existing rows unchanged.

Do NOT rewrite, translate, or retire historical debt.

==================================================
5. MIGRATION VERIFICATION
==================================================

Document and test all bootstrap paths actually found in the repository.

For an existing database where 004 has already been applied:

BEFORE 010:
    insert a belief without debt
    verify the old six-item default appears

APPLY 010

AFTER 010:
    insert another belief without debt
    verify debt is empty

Also verify:
    existing pre-010 row remains unchanged.

For fresh installation:
    apply complete migration chain
    verify new default is empty.

Verify:
- test schema loaders;
- cloud cold/warm initialization;
- CLI reset;
- Taskfile reset.

==================================================
6. EnterBelief API CONTRACT
==================================================

Keep debt optional in JSON.

Example:

    {
      "scenario_id": "...",
      "claim": "...",
      "claim_type": "derived"
    }

means:

    debt = []

Explicit [] has identical semantics.

Explicit debt values are passed through unchanged.

Do NOT make debt a required JSON field.

The database empty default remains valid.

==================================================
7. STRUCTURAL VALIDATION BOUNDARY
==================================================

Empty string is malformed input.

Therefore:

HTTP/API:
    "" -> reject

MCP:
    "" -> reject

Solvent kernel:
    "" -> ACCEPT

This distinction is mandatory.

Solvent itself MUST NOT reject empty-string debt because doing so would make Solvent semantically inspect opaque debt values.

Add:
1. API boundary test proving "" is rejected.
2. MCP boundary test proving "" is rejected.
3. Kernel test proving arbitrary opaque strings, including "",
   are accepted.

Do not place the empty-string rejection test in kernel_test.go.

==================================================
8. REMOVE VOCABULARY VALIDATION
==================================================

Remove the MCP guard based on kernel.FullDebt / slices.Contains.

Remove the MCP enum exposing kernel.FullDebt.

The resulting MCP schema must not enumerate universal valid debt strings.

Update tool description so callers know they can inspect the target belief's current debt values.

However:

MCP must still reject malformed "" input.

Semantic vocabulary validation belongs to the owning domain, not Solvent.

==================================================
9. RETIRE DEBT CONTRACT
==================================================

Preserve existing semantics confirmed by repository inspection:

- nonexistent belief -> ErrBeliefNotFound
- nonexistent debt item on existing belief -> idempotent no-op
- duplicate debt values -> preserve actual array_remove behavior

For duplicates:

    ["foo", "foo"]

retiring "foo" should result in:

    []

Test the actual resulting array state, not only nil error.

==================================================
10. UPDATE EXISTING TESTS INTELLIGENTLY
==================================================

Do NOT blindly replace all ~100 kernel.FullDebt references.

First classify every occurrence.

Category A:
    test only needs any non-empty debt

Use a simple value such as:

    []string{"testDebt"}

Category B:
    test depends on the semantic six-item wizard lifecycle

Examples:
- discharge-completeness tests
- retire-each-item tests
- promotion only after the last wizard debt
- integration tests tied to the wizard workflow

Use the domain-owned wizard vocabulary through the approved accessor/API.

Perform this classification BEFORE editing tests.

Preserve the semantic strength of the existing tests.

==================================================
11. REWRITE OBSOLETE TESTS
==================================================

Replace old B-17/B-23 behavior tests with current contract tests.

Do NOT preserve obsolete tests as commented code.

Canonical tests should cover:

- arbitrary debt accepted;
- omitted debt => empty;
- explicit empty debt;
- unknown debt accepted;
- arbitrary debt retired;
- promotion blocked with remaining debt;
- promotion succeeds with no debt;
- final-truth restriction unchanged;
- duplicate debt behavior;
- missing debt item is a no-op;
- missing belief remains an error;
- kernel accepts opaque empty-string value;
- API rejects empty-string value;
- MCP rejects empty-string value;
- DDL default is empty.

Avoid duplicate tests.

If B-23 and CS_DEBT09 test exactly the same DDL property,
keep one canonical test.

Only keep both if inspection proves they validate different layers/paths.

Rename:

    TestIntegration_RealFixtureRetiresFullDebt

to a name that reflects actual semantics, such as wizard debt,
rather than the deleted FullDebt kernel constant.

==================================================
12. EBP COORDINATOR
==================================================

There is no Coordinator implementation in this repository.

Therefore do NOT pretend to implement it here.

The Solvent plan must record a mandatory external integration contract:

    EBP Coordinator owns ebpInitialDebt.

For every EBP belief compiled from a research packet:

    compiledDebt =
        ebpInitialDebt
        UNION
        accepted additional packet debt

Agent-supplied debt may add obligations but may not remove required EBP debt.

Critical negative regression test belongs to the Coordinator:

    input:
        valid belief packet
        agent supplies no debt

    output:
        compiled canonical belief contains ebpInitialDebt

This test must be specified as an external integration acceptance gate.

It is NOT a Solvent test.

The Coordinator is responsible for enforcing this because it is the layer that knows what EBP requires.

==================================================
13. OTHER DOMAIN CALLERS
==================================================

Inspect:
- demo/cloud/init
- operator-review
- github executor
- other existing callers

For each caller determine:

A. Is it genuinely using the wizard/deployment-review vocabulary?
    -> reuse the shared domain-owned wizard vocabulary.

B. Is it a genuinely independent domain?
    -> define its own domain-owned debt vocabulary.

Do NOT create unnecessary duplicate copies of the same vocabulary.

Do NOT create a global shared Solvent vocabulary.

Document who owns and validates each caller's policy.

==================================================
14. FUNCTIONAL TAXONOMY
==================================================

Preserve the useful conceptual guidance outside Solvent.

The guidance may discuss recurring functional roles such as:

    provenance
    consistency
    scope
    verification
    impact
    recovery
    human authority when policy requires it

But explicitly state:

- this is guidance;
- not a universal literal vocabulary;
- not Solvent policy;
- not a required set;
- not an enum;
- not enforced by code.

Do NOT add ebp:* constants to Solvent.

Place the guidance outside the kernel repository or in an explicitly non-normative documentation area.

==================================================
15. GRAND-FATHERING
==================================================

Existing beliefs retain their historical debt arrays.

The migration must not:
- rewrite rows;
- translate historical debt;
- auto-retire historical items.

Old wizard beliefs must remain operable because the wizard domain still recognizes the historical identifiers.

==================================================
16. DECISION RECORD
==================================================

The decision record must lead with:

This change restores documented doctrine that debt vocabulary belongs to
the application, policy, or deployment. It does not expand Solvent's
kernel scope.

Record:

- why kernel was reopened;
- evidence from repository inspection;
- prior six-item coupling;
- why no universal replacement vocabulary was introduced;
- wizard ownership;
- EBP Coordinator ownership;
- grandfathering;
- API/MCP boundary semantics;
- migration behavior;
- integration safety compensation;
- final tests;
- final frozen commit;
- supersession of 7602699.

Also explicitly record the safety responsibility map:

    Wizard:
        wizard domain validates and attaches wizard debt

    EBP:
        Coordinator validates/attaches ebpInitialDebt

    Other domain callers:
        their own application code owns and enforces their policy

Solvent:
    provides no policy default.

==================================================
17. RE-FREEZE
==================================================

Final sequence:

    implement
    -> targeted tests
    -> full suite
    -> migration/bootstrap verification
    -> vocabulary sweep
    -> decision record
    -> commit
    -> annotated tag on exact freeze commit
    -> update baseline references
    -> record supersession of 7602699

Do not leave the freeze as floating HEAD.

==================================================
18. REPOSITORY-WIDE VOCABULARY SWEEP
==================================================

Search all six old identifiers.

Classify every remaining occurrence.

Expected:
- wizard/domain code
- legitimate independent domain code
- appropriate fixtures/tests/examples

Unexpected:
- Solvent kernel
- Solvent default
- generic Solvent validator
- universal MCP enum

The sweep must be part of acceptance.

==================================================
19. FINAL PLAN OUTPUT
==================================================

Produce a revised plan with:

1. Executive decision
2. Actual repository findings
3. Package dependency resolution
4. Ownership/boundary model
5. Schema/migration changes
6. API/MCP changes
7. Wizard vocabulary consolidation
8. Coordinator integration contract
9. Test classification and changes
10. Migration verification
11. Documentation/taxonomy guidance
12. Decision record
13. Re-freeze
14. Risks and mitigations
15. Exact files expected to change
16. Acceptance criteria

Do not present alternatives that have already been rejected.

Do not leave "if needed", "or", or other unresolved architectural choices
where repository inspection can settle them.

The plan must explicitly distinguish:

    kernel semantic behavior
    API/MCP structural validation
    domain vocabulary validation
    Coordinator policy enforcement

The final design must make Solvent smaller and more domain-agnostic,
not replace one hardcoded vocabulary with another.
```

This incorporates the net-valid findings from the latest review while retaining the earlier corrections. In particular, the **empty-string distinction, package-cycle check, semantic classification of `FullDebt` test callers, pre/post migration verification, domain-specific safety ownership, and removal of leftover alternatives** are now explicit implementation gates rather than suggestions. 

The resulting plan should be the last planning revision before implementation.
||||||| base
=======
Use this as the coding-agent prompt:

```text
You are updating the repository's AGENTS.md so it accurately reflects the CURRENT Solvent architecture after the kernel freeze.

This is a DOCUMENTATION SYNCHRONIZATION TASK ONLY.

Do NOT redesign the system.
Do NOT modify kernel behavior.
Do NOT modify schema or migrations.
Do NOT modify APIs, services, executors, adapters, MCP, tests, or scripts merely to make the documentation fit.
Do NOT reopen the kernel freeze.

The repository itself is the source of truth for current implementation details.

==================================================
OBJECTIVE
==================================================

Update the root AGENTS.md so that it becomes the canonical operating guide for future coding agents working on the frozen Solvent architecture.

The current AGENTS.md is materially stale. It still describes an earlier v0 state in which:

- execution was future-only,
- the executor registry was empty,
- the architecture was primarily framed as memory → belief → authority,
- the data model was described as only a small set of core tables,
- exact target/snapshot authority binding was not documented,
- service-layer authorization boundaries were not documented,
- the kernel-freeze growth rule was not expressed in its final form,
- AUTHORIZE != EXECUTE was not explicit,
- authentication / identity / actor type / authorization distinctions were incomplete,
- the adapter/service/executor/deployment extension model was incomplete.

Your job is to reconcile the document with the repository as it exists NOW.

==================================================
PHASE 1 — REPOSITORY AUDIT
==================================================

Before editing AGENTS.md, inspect the repository thoroughly enough to establish the actual current architecture.

At minimum inspect:

1. Repository structure
   - root directories
   - cmd/
   - api/
   - service/
   - kernel/
   - adapter/
   - internal/
   - demos/
   - scripts/
   - migrations/schema
   - Taskfile/configuration

2. Kernel
   - public methods
   - authority lifecycle
   - belief lifecycle
   - debt lifecycle
   - target/snapshot/activation/revocation model
   - action intent lifecycle
   - authorization
   - ClaimIntent / CompleteIntent / RollbackClaim / CancelIntent
   - scenario isolation
   - transaction boundaries
   - exact authority binding
   - database invariants

3. Database/schema
   - actual tables
   - constraints
   - foreign keys
   - unique indexes
   - CHECK constraints
   - state transitions
   - any exact `(target_id, snapshot_id)` relationships
   - confirm what is actually kernel data versus service/product/audit data

4. Service layer
   - authority execution path
   - policy checks
   - RetireDebt
   - Discharge
   - ExecuteAction
   - scenario guards
   - authentication assumptions
   - actor/principal handling

5. Executors/adapters
   - GitHub executor/provider
   - executor selection
   - provider outcome handling
   - what the executor is and is NOT allowed to do
   - any other real executor/provider integrations

6. API
   - REST execution endpoint
   - authorization endpoint(s)
   - authentication boundary
   - caller-controlled identity fields
   - error semantics

7. MCP
   - tools exposed
   - authentication/deployment boundary
   - local trusted stdio assumption
   - any distinction between local trusted MCP and remotely exposed MCP

8. Tests
   - exact authority/confused-deputy tests
   - scenario-isolation tests
   - concurrency/race tests
   - service authorization tests
   - executor tests
   - audit tests
   - MCP verification
   - I-7/raw-write checks

9. Documentation/scripts
   - existing architecture docs
   - freeze rules
   - Taskfile
   - verification scripts
   - any documentation that contradicts AGENTS.md

Do not infer implementation from the old AGENTS.md when repository evidence is available.

==================================================
PHASE 2 — PRODUCE AN INTERNAL DELTA CHECK
==================================================

Before editing, identify:

A. Claims in AGENTS.md that are obsolete.
B. Claims that remain correct.
C. New architectural facts that must be added.
D. Statements that cannot be verified from the repository and therefore should NOT be asserted as facts.
E. Any discrepancy serious enough to indicate a possible code regression.

Do not fix code to resolve documentation discrepancies.

If you discover a genuine implementation inconsistency that cannot safely be treated as documentation drift, STOP before making architectural changes and report it.

==================================================
PHASE 3 — REWRITE AGENTS.md
==================================================

Rewrite AGENTS.md into a concise but technically authoritative guide for coding agents.

Keep the strongest useful material from the current file, but remove obsolete claims.

The document should be organized approximately around these concepts. You may improve the organization if the repository suggests a better structure.

--------------------------------------------------
1. PROJECT
--------------------------------------------------

Describe Solvent as the current product, not merely as a "transactional belief ledger."

The important framing is:

Solvent is a small, durable authority layer/checkpoint between autonomous systems and consequential actions.

Preserve the distinction between:
- belief
- evidence
- intent
- authority
- execution

Make clear that Solvent is NOT merely an agent memory system.

--------------------------------------------------
2. CORE THESIS
--------------------------------------------------

Make the following central:

> Retrieval is not authority.

Also establish:

> Evidence is not authority.
> Agent claims are not authority.
> Capability is not authority.
> Intent is not authority.
> Authorization is distinct from execution.

Use the strongest wording supported by the implementation.

--------------------------------------------------
3. KERNEL FREEZE
--------------------------------------------------

This must be prominent.

Document the final rule:

> New capabilities default to service, adapter, executor, deployment, policy, demo, or documentation layers.

And:

> Kernel changes require a genuinely new durable security fact or atomic security transition that cannot safely be expressed outside the existing kernel.

Make explicit:

> Grow the ecosystem, not the kernel.

Explain that future contributors must NOT add kernel primitives simply because a feature is "security related."

--------------------------------------------------
4. ARCHITECTURE / RESPONSIBILITY BOUNDARIES
--------------------------------------------------

Document the current extension decision tree:

External product/protocol
    → Adapter

Policy/orchestration/composition
    → Service / Policy

Execution/infrastructure
    → Executor / Deployment

Customer-specific behavior
    → Policy / Configuration / Data

Reporting/UI/analytics
    → Product / Read Model / Service

Impossible state
    → DB invariant

New atomic security primitive
    → Kernel only if unavoidable

Everything else
    → Don't add it

Explain that provider-specific and domain-specific semantics should not be moved into the generic kernel.

--------------------------------------------------
5. AUTHORITY MODEL
--------------------------------------------------

Document the actual lifecycle supported by the repository.

Include the concept that authority must bind to the exact consequence, including the exact target/snapshot identity.

Document the confused-deputy protection.

Do NOT overstate anything that the repository does not actually enforce.

--------------------------------------------------
6. BELIEFS / EVIDENCE / DEBT
--------------------------------------------------

Retain the useful belief-ledger concepts from the existing AGENTS.md.

Document:

- evidence is attributable
- debt is an explicit unresolved obligation
- debt vocabulary is opaque/domain-specific
- kernel should not hardcode deployment-specific debt names
- promotion is structurally gated by debt state
- agent reasoning does not itself create authority

Do not reintroduce deployment-specific vocabulary into kernel guidance.

--------------------------------------------------
7. EXACT AUTHORITY BINDING
--------------------------------------------------

This is a critical addition.

Document that an action intent is tied to the exact authority instance represented by target + snapshot.

Explain why this exists:

- prevents confused deputy behavior
- prevents an approval for one target/state from being reused for another
- database constraints reinforce the relationship
- ClaimIntent must preserve exact identity

Do not invent constraint names unless verified from the repository.

--------------------------------------------------
8. AUTHORIZE != EXECUTE
--------------------------------------------------

Document the current real execution path.

Use the repository's actual flow, which should conceptually look like:

Prepare
  → current state / intent handling
  → Authorize
  → Claim intent
  → fixed executor/provider
  → provider outcome
  → Complete / Rollback / Reconciliation as applicable

Verify exact ordering against the code before documenting it.

Explicitly state:

Authorization does not prove that the external side effect happened.

The executor consumes authorization.
The executor must not mint authority.

--------------------------------------------------
9. EXECUTOR / ADAPTER RULES
--------------------------------------------------

Document the actual GitHub executor integration if still present.

Make the boundary explicit:

- adapters translate external systems into Solvent concepts
- executors perform already-authorized consequences
- executors do not approve, promote, revoke, or create authority
- provider-specific errors/outcomes remain provider concerns
- caller input must not freely select arbitrary executors unless the repository actually supports that

Verify all of this before writing it.

--------------------------------------------------
10. ACTOR / AUTHENTICATION / IDENTITY
--------------------------------------------------

Document the distinction:

Actor type != identity != authentication != authorization.

Examples:

HUMAN / AGENT / SYSTEM are actor categories.

Authentication happens at the deployment/API boundary.

Do not trust caller-supplied actor identity in request bodies.

Where the repository derives principal identity from authenticated request context, document that.

--------------------------------------------------
11. TRUST BOUNDARIES
--------------------------------------------------

Document the major trust vectors:

1. direct kernel calls
2. alternate authority-mutating APIs
3. lower-level service paths bypassing policy
4. direct DB access
5. deployment/configuration exposure

Explain that a policy is only meaningful if the relevant consequential mutation paths cannot bypass it.

Do not claim application authorization is equivalent to DB credential isolation.

--------------------------------------------------
12. MCP
--------------------------------------------------

Document the actual current MCP boundary.

Distinguish:

local trusted stdio deployment
from
remote/public MCP exposure.

Do not imply that local MCP caller fields constitute strong authentication unless the repository actually provides it.

Make the deployment boundary explicit.

--------------------------------------------------
13. AUDIT
--------------------------------------------------

Document:

- audit events should correspond to actual events
- rejected authorization attempts must not be represented as successful actions
- distinguish authorization, provider invocation, provider outcome, and execution result
- audit is observability/evidence of behavior, not a substitute for authority enforcement

Do not overstate atomicity if audit is outside the kernel transaction.

--------------------------------------------------
14. RACE / CONCURRENCY RULES
--------------------------------------------------

Document only races verified in code/tests.

Important concepts likely include:

- claim before external side effect
- concurrent intent handling
- concurrent revocation
- provider-side TOCTOU limitations
- service-layer RetireDebt liveness pre-check is best-effort and NOT transactionally atomic

Use precise language.

Do not hide accepted residual risk.

--------------------------------------------------
15. DATABASE RULES
--------------------------------------------------

Retain the current strong position:

CockroachDB is an active enforcement layer, not passive storage.

Document real invariants only.

Do not state "where application code can enforce it, schema always wins" as an absolute if the current implementation contradicts that; instead express the actual architecture carefully:

Use DB invariants for structural security facts that the schema can safely enforce.

Preserve distinction between:
- DB-enforced invariant
- kernel transactional logic
- service policy
- external-provider behavior

--------------------------------------------------
16. SCENARIO ISOLATION
--------------------------------------------------

Document the actual scenario-binding rules verified by current tests.

Cross-scenario operations must fail closed.

Do not rely on caller-supplied IDs where the current service/kernel derives or validates them.

--------------------------------------------------
17. DEVELOPMENT RULES
--------------------------------------------------

Preserve the strongest existing principles:

- think like a distributed systems engineer
- explicit invariants
- deterministic behavior
- transactional correctness
- minimal architecture
- unknowns become receipts
- no invented metrics
- no prompt-only guarantees
- don't duplicate truth
- don't weaken the ledger for demo convenience

Add:

- verify current repository behavior before documenting it
- prefer service/policy/adapters/executors over kernel growth
- do not reopen frozen architecture casually
- do not claim guarantees stronger than the actual transaction/deployment boundary

--------------------------------------------------
18. VERIFICATION
--------------------------------------------------

Document the repository's real verification commands.

Prefer commands derived from the repo rather than invented ones.

Do NOT hardcode stale package counts.

Use the repository's current:
- test
- vet
- race
- build
- DB reset
- I-7
- isolation
- MCP verification
commands where applicable.

The existing rule to use `task test` for current suite status should remain if that target still exists.

--------------------------------------------------
19. REMAINING ACCEPTED BOUNDARIES
--------------------------------------------------

Only include risks verified from the current repository.

Likely examples, ONLY if still true:

- RetireDebt principal liveness pre-check is best effort rather than atomic with the kernel transaction.
- local MCP is a trusted deployment boundary rather than a fully authenticated remote service.

Do not convert LOW/INFO observations into fake guarantees.

==================================================
IMPORTANT DOCUMENTATION RULES
==================================================

1. Repository truth wins over the old AGENTS.md.

2. Do not preserve obsolete statements simply because they sound architectural.

3. Do not add speculative future architecture as if it already exists.

4. Clearly distinguish:
   - current implementation
   - architectural principle
   - future extension point

5. Do not hardcode test counts.

6. Do not invent constraint names, table counts, API endpoints, executor behavior, or security guarantees.

7. Preserve exact terminology already established in the repository where possible.

8. Do not describe the database as doing work it does not actually do.
   In particular, do not describe recursive belief traversal as a DB cascade unless the repository really implements it that way.

9. Do not weaken or remove useful retrieval-integrity rules from the existing AGENTS.md unless the repository proves they are obsolete.

10. Do not mention this prompt or the coding-agent process inside AGENTS.md.

==================================================
PHASE 4 — VERIFY THE UPDATED DOCUMENT
==================================================

After editing AGENTS.md:

1. Re-read the entire AGENTS.md.
2. Cross-check every implementation-specific claim against the repository.
3. Ensure no obsolete "future-only execution" language remains.
4. Ensure no claim says executor support is empty if a real executor exists.
5. Ensure exact authority binding is documented.
6. Ensure the kernel-freeze rule is prominent and unambiguous.
7. Ensure service/policy responsibilities are not incorrectly pushed into the kernel.
8. Ensure trusted-local MCP boundary is documented accurately.
9. Ensure no stale architecture diagrams remain.
10. Ensure formatting is clean.

Do NOT change production code.

==================================================
FINAL VERIFICATION / REPORT
==================================================

Run the minimum relevant repository verification needed to confirm that the documentation update did not alter code behavior.

At minimum:
- git diff -- AGENTS.md
- git status --short
- any repository-native documentation/format check if available

Do not run an enormous unrelated test matrix merely for the documentation task unless necessary.

Final response should contain:

1. A concise summary of what changed in AGENTS.md.
2. The major stale statements removed or corrected.
3. The major current architectural facts added.
4. Confirmation that no kernel/schema/code behavior was changed.
5. Verification commands run and results.
6. Any repository discrepancy discovered that should be reviewed separately.

Do not claim "fully verified" unless the actual repository evidence supports it.

==================================================
SUCCESS CRITERION
==================================================

When finished, a new coding agent reading AGENTS.md should understand:

- what Solvent is NOW,
- what the frozen kernel is responsible for,
- what the kernel is explicitly NOT responsible for,
- how authority is established,
- why exact target/snapshot binding matters,
- why AUTHORIZE != EXECUTE,
- how adapters/services/policies/executors/deployments extend the system,
- where authentication and policy live,
- what the MCP trust boundary is,
- what the important database invariants are,
- what residual risks are consciously accepted,
- and, above all:

> GROW THE ECOSYSTEM, NOT THE KERNEL.

The final AGENTS.md must describe the CURRENT repository, not an earlier Solvent milestone.
```
