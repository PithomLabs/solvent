I consolidated the new reviews with my prior review. The architectural conclusion remains unchanged, but there are several concrete plan defects that should be fixed before implementation. The strongest new finding is the **CS_DEBT10 layer contradiction**: the plan currently places an API-boundary rejection test in `kernel_test.go`, which would violate the central opaque-debt invariant if implemented literally. 

## Consolidated final disposition

### Must fix before implementation

**1. Resolve the `belief` ↔ `wizard` package dependency before coding.**

The plan moves wizard vocabulary into `internal/belief/debt.go` and has `internal/wizard` consume it. That is correct only if the package graph permits it. The coding plan must inspect imports first.

Desired outcome:

```text
belief
  owns vocabulary + mapping

wizard
  imports belief
```

If `belief` already imports `wizard`, use a tiny shared domain package instead:

```text
internal/domain/debt.go
```

Do not proceed with a package cycle and do not duplicate the six strings again. This is a genuine blocking implementation check. 

**2. Use named debt constants, not `wizardDebt[0]`, `[1]`, etc.**

This is unanimously worth fixing.

Use:

```go
const (
    NeedProvenanceCheck    = "needProvenanceCheck"
    NeedContradictionSweep = "needContradictionSweep"
    NeedBlastRadius        = "needBlastRadius"
    NeedRollbackPlan       = "needRollbackPlan"
    NeedVersionPin         = "needVersionPin"
    NeedOperatorSignoff    = "needOperatorSignoff"
)
```

Then compose:

```go
var wizardDebt = []string{
    NeedProvenanceCheck,
    NeedContradictionSweep,
    NeedBlastRadius,
    NeedRollbackPlan,
    NeedVersionPin,
    NeedOperatorSignoff,
}
```

`DebtMapping` references the named constants. This preserves one source of truth without positional coupling. 

**3. Delete the `mapping.go` alternative.**

There must be no "keep literals + comment" option. The repository inspection already proved there are two independently maintained copies. The whole purpose of this change is to eliminate that drift. 

**4. Correct the empty-string test boundary.**

This is important.

The rule is:

```text
API/MCP adapter:
    "" -> reject

Solvent kernel:
    "" -> accept as opaque string
```

Therefore:

```text
CS_DEBT10
```

must be an API/MCP boundary test, **not** a kernel test.

Add a complementary kernel regression test proving arbitrary opaque strings, including `""`, are accepted by the kernel. That directly protects the architectural invariant rather than accidentally reversing it. 

**5. Classify every `kernel.FullDebt` test occurrence before replacing it.**

Do not blindly replace ~100 references with `[]string{"testDebt"}`.

There are two classes:

```text
A. Any non-empty debt is sufficient
   -> use simple test debt

B. Test depends on the actual six-item lifecycle
   -> use the domain-owned wizard vocabulary
```

The second class includes discharge, retirement-order, promotion-after-last-debt, and full-pipeline tests. The new plan must explicitly classify these before changing them. 

**6. Add MCP-side empty-string validation.**

The plan currently removes the MCP vocabulary guard but only adds empty-string validation to `api/belief.go`.

That creates:

```text
HTTP API -> "" rejected
MCP      -> "" reaches kernel
```

Either validate `""` in the MCP handler too, or explicitly establish that the MCP contract intentionally permits it.

I recommend **validate it in MCP**, because the empty string is malformed structural input, not legitimate domain vocabulary. 

**7. Strengthen migration verification with a pre-state assertion.**

For an existing database with migration 004 already applied:

```text
before 010:
    insert -> old six-item default

apply 010

after 010:
    insert -> {}

existing row:
    unchanged
```

That demonstrates 010 is actually superseding the old default rather than merely testing the final state. 

**8. Make the EBP compensating control mechanically testable.**

The external Coordinator must guarantee:

```text
packet has no debt
        ↓
Coordinator adds ebpInitialDebt
        ↓
canonical belief is NOT debt-free
```

The integration test belongs to the Coordinator, not this Solvent repository, but the Solvent decision record should explicitly name the enforcement owner.

The same record should identify the policy owner for Wizard, operator-review, demo, and examples. 

**9. Remove the redundant B-23 / CS_DEBT09 duplication unless they test different layers.**

They currently both prove the default is empty.

Keep one canonical DDL-default test. Preserve the other only if it tests a distinct bootstrap/API path. Otherwise it adds maintenance without new evidence.

**10. Rename `TestIntegration_RealFixtureRetiresFullDebt`.**

The old `FullDebt` concept is being removed. Rename the test to describe what it actually proves, e.g.:

```text
TestIntegration_RealFixtureRetiresWizardDebt
```

or another name grounded in its actual semantics. This also keeps the repository-wide vocabulary sweep meaningful. 

### Keep as approved

These remain correct:

* `001_schema.sql` unchanged.
* Additive `010_debt_opaque.sql`.
* Existing rows grandfathered unchanged.
* Optional `Debt []string`.
* Empty default at database level.
* Arbitrary opaque debt accepted by the kernel.
* Domain-owned wizard vocabulary.
* No universal `ebp:*` vocabulary.
* Functional taxonomy remains documentation/guidance only.
* Promotion gate unchanged.
* `action_intent`, retraction cascade, refusal logging unchanged.
* Annotated re-freeze and explicit supersession of `7602699`.
* Full repository vocabulary sweep.

The core boundary matrix is still exactly right: domain owns meaning, ingress handles structural validity, and Solvent owns storage plus promotion enforcement. 

---

# Prompt for the coding agent

```text
REVise Plan 11.1 into the final implementation-ready plan.

Do NOT implement code yet.

Consolidate the existing Plan 11.1, the prior architecture review, and the latest review findings. Resolve every contradiction and remove every obsolete alternative.

The final plan must be internally consistent, repository-grounded, and ready for implementation without leaving design choices to the coding agent.

==================================================
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
