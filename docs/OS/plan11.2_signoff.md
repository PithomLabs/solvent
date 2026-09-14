The reviews converge: **Plan 11.1 is approved for execution**. No further architectural redesign is warranted. The remaining items are implementation notes, and I would fold them into the execution prompt rather than reopen the plan. The reviewers specifically confirm the kernel/domain boundary, the three-layer empty-string behavior, named constants, test classification, migration assertions, and re-freeze protocol. 

There is one wording correction worth carrying into the execution instructions: operator-review should **reuse `belief.WizardDebt()`**, not be described as owning a separate local debt slice. 

Use this as the final coding-agent prompt:

```text id="final-execution-prompt"
IMPLEMENT PLAN 11.1 — MAKE DEBT OPAQUE TO SOLVENT

The architecture and implementation plan are approved.

Proceed directly to implementation.

Do NOT reopen the architecture.
Do NOT replace this design with another abstraction.
Do NOT introduce a universal debt vocabulary.
Do NOT add unrelated refactors.

The governing invariant is:

    DEBT IS OPAQUE TO SOLVENT.

==================================================
1. FINAL ARCHITECTURE
==================================================

Solvent kernel:
- stores debt as TEXT[];
- empty database default;
- accepts arbitrary opaque debt identifiers;
- retires opaque identifiers;
- promotion requires debt array empty;
- does NOT enumerate, validate, normalize, lowercase, trim,
  interpret, or otherwise impose semantic rules on debt values.

API/MCP boundary:
- debt field is optional;
- omitted debt becomes []string{};
- malformed empty-string identifiers ("") are rejected;
- vocabulary membership is NOT validated here;
- no regex, length, count, or domain-specific whitelist.

Wizard domain:
- owns the six deployment-review debt identifiers;
- owns DebtMapping;
- is the single source of truth;
- must use named constants rather than positional slice indexes;
- must not depend on kernel.FullDebt.

EBP Coordinator:
- external to this repository;
- owns ebpInitialDebt;
- MUST attach EBP starting debt during packet compilation;
- packet-provided debt may add obligations but cannot remove required EBP debt.

Other applications/domains:
- explicitly own and attach their own starting debt;
- do not rely on Solvent's database default.

==================================================
2. BEFORE EDITING — VERIFY REAL CODE
==================================================

Inspect the repository once before making changes.

Confirm:
- package graph remains cycle-free;
- exact current FullDebt references;
- exact migration/bootstrap paths;
- exact API and MCP handlers;
- exact tests using FullDebt;
- exact current RetireDebt semantics.

The package graph was already verified as cycle-free:
    belief -> wizard = NO
    wizard -> belief = NO

Still confirm this remains true after planned imports are introduced.

If an unexpected cycle appears:
- stop and resolve with the smallest domain-sharing mechanism;
- do not duplicate the wizard vocabulary.

==================================================
3. DATABASE / MIGRATION
==================================================

Do not modify db/001_schema.sql.

Add:

    db/010_debt_opaque.sql

containing the repository-compatible equivalent of:

    ALTER TABLE belief
      ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];

Preserve all existing rows.

Do not rewrite historical debt.
Do not translate historical debt.
Do not auto-retire historical debt.

Register 010 in every actual schema path:
- 14 test schema loaders;
- cloud cold initialization;
- cloud warm initialization;
- CLI resolveSchemaPaths;
- Taskfile db:reset;
- any other bootstrap path discovered during inspection.

For the migration verification, explicitly prove:

BEFORE 010, with 004 already applied:
    new insert gets old six-item default.

AFTER 010:
    new insert gets empty array.

Existing pre-010 rows:
    remain unchanged.

==================================================
4. REMOVE KERNEL FULLDEBT
==================================================

Delete kernel.FullDebt and its compile-time assertion.

Remove all kernel-level assumptions that the six deployment-review
strings are universally valid.

Search the kernel after implementation to ensure none remain.

==================================================
5. WIZARD VOCABULARY — SINGLE SOURCE OF TRUTH
==================================================

Create/finish the domain-owned vocabulary in the actual package selected
by the approved plan: internal/belief/debt.go.

Use named constants:

    NeedProvenanceCheck
    NeedContradictionSweep
    NeedBlastRadius
    NeedRollbackPlan
    NeedVersionPin
    NeedOperatorSignoff

Compose the canonical wizardDebt slice from those constants.

Do NOT use:

    wizardDebt[0]
    wizardDebt[1]
    ...

Expose:

    WizardDebt() []string

as a defensive-copy accessor for external packages.

Preferred shape:

    var wizardDebt = []string{
        NeedProvenanceCheck,
        ...
    }

    func WizardDebt() []string {
        return append([]string(nil), wizardDebt...)
    }

Do not create a second independently maintained vocabulary list.

Add a comment documenting that external callers must use WizardDebt()
rather than mutating the package-level slice.

==================================================
6. MAPPING.GO
==================================================

Replace all six hardcoded string literals in mapping.go with the named
domain constants.

Do NOT retain a second literal list.
Do NOT retain the rejected "keep literals + comment" alternative.

The result must have exactly one domain-owned vocabulary.

==================================================
7. WIZARD / OTHER CALLERS
==================================================

Update:
- internal/wizard/seed.go
- internal/wizard/discharge.go
- internal/wizard/state.go

to use belief.WizardDebt() as appropriate.

Update:
- demo/cloud/init/main.go
- cmd/operator-review/main.go
- internal/agentjacking/ingest.go

to reuse belief.WizardDebt() when they are using the same wizard/deployment
domain vocabulary.

Do NOT describe those as owning independent local copies.

For:
- examples/github/executor/main.go

keep a local example debt value if it is intentionally isolated from
internal application packages.

For every other caller discovered:
- determine whether it is the same wizard domain;
- reuse the wizard source when it is;
- define a genuinely independent domain vocabulary only when it is not.

==================================================
8. API
==================================================

Add to EnterBeliefRequest:

    Debt []string `json:"debt,omitempty"`

Omitted debt:
    []string{}

Explicit empty array:
    []

Explicit arbitrary values:
    preserved unchanged.

At api/belief.go:
- normalize nil to empty slice;
- reject any debt item equal to "";
- do not validate vocabulary.

Do not make Debt a required JSON field.

==================================================
9. MCP
==================================================

Remove the kernel.FullDebt/slices.Contains vocabulary guard.

Remove the debt enum from the MCP schema.

Add structural validation:

    debt_item == "" -> error

Pass all other debt identifiers directly to the ledger.

Update tool description so users know valid retirement identifiers can
be discovered by inspecting the belief's current debt.

Update scripts/mcp_verify.sh:
- remove enum assertions;
- remove unknown/retired-name rejection assumptions;
- preserve belief-lookup behavior;
- add empty-string rejection coverage.

Edit the shell script by pattern rather than relying on stale line numbers.

==================================================
10. RETIREMENT SEMANTICS
==================================================

Preserve:

    nonexistent belief
        -> ErrBeliefNotFound

    existing belief + nonexistent debt item
        -> idempotent no-op

    duplicate debt:
        ["foo","foo"]
        retire("foo")
        -> []

Use the actual array_remove behavior.

Do not redesign RetireDebt.

==================================================
11. TEST CLASSIFICATION
==================================================

Do NOT blindly replace every FullDebt occurrence with testDebt.

Classify every test occurrence before modifying it.

Category A:
- test only requires a non-empty debt value;
- replace with a simple value such as ["testDebt"].

Category B:
- test depends on the six-item wizard lifecycle;
- use belief.WizardDebt().

Pay particular attention to:
- discharge completeness;
- retire-each-item flows;
- promotion-after-final-retirement;
- wizard lifecycle;
- tests asserting debt count/content;
- integration fixtures.

Preserve semantic coverage.

==================================================
12. NEW REGRESSION TESTS
==================================================

Implement the approved tests with correct layer placement.

Kernel-level:

CS_DEBT01
    arbitrary debt accepted

CS_DEBT02
    explicit empty debt accepted

CS_DEBT03
    nil debt results in empty storage

CS_DEBT04
    unknown debt accepted

CS_DEBT05
    arbitrary debt retirement

CS_DEBT06
    promotion blocked while arbitrary debt remains

CS_DEBT07
    promotion succeeds with empty debt

CS_DEBT08
    final-truth prohibition remains unchanged
    only add as new coverage if it is not already adequately tested

CS_DEBT09
    duplicate debt retirement removes all matching entries

CS_DEBT10
    nonexistent belief still errors

CS_DEBT11
    kernel accepts [""] as opaque debt

API boundary:

CS_DEBT12
    API rejects [""] with HTTP 400

MCP boundary:

CS_DEBT13
    MCP rejects debt_item == ""

Migration:

CS_DEBT14
    migration 010 supersedes 004's old default

Do not duplicate B-23 and CS_DEBT14 if they are proven to exercise exactly
the same path. Consolidate duplicate coverage where appropriate.

B-17:
    arbitrary slice encoding test.

B-23:
    DDL empty-default test, unless CS_DEBT14 already covers the identical
    path; avoid redundant tests.

Rename:
    TestIntegration_RealFixtureRetiresFullDebt

to:

    TestIntegration_RealFixtureRetiresWizardDebt

or an equivalent name reflecting actual semantics.

==================================================
13. THREE-LAYER EMPTY-STRING INVARIANT
==================================================

The final behavior MUST be:

    Solvent kernel:
        accepts ""

    HTTP API:
        rejects ""

    MCP:
        rejects ""

This is intentional.

It proves:
- Solvent remains domain-agnostic;
- adapters reject malformed structural input.

Do not accidentally add empty-string rejection to the kernel.

==================================================
14. EBP COORDINATOR CONTRACT
==================================================

There is no Coordinator code in this repository.

Do not implement one here.

Preserve the external integration specification:

    compiledDebt =
        ebpInitialDebt UNION acceptedPacketDebt

Every EBP belief compiled from a packet must receive ebpInitialDebt,
even if the agent packet supplies no debt.

Agents may propose additional debt.
Agents cannot remove required EBP starting debt.

The Coordinator must have a negative regression test:

    input:
        valid packet, no debt

    output:
        compiled belief contains ebpInitialDebt

This is an external integration acceptance gate, not a Solvent test.

Do not hardcode ebpInitialDebt into Solvent.

Do not assume a universal literal count in Solvent.

==================================================
15. DOCUMENTATION
==================================================

Update kernel documentation to say:

    Solvent treats debt identifiers as opaque obligation IDs.
    Debt semantics are defined by the caller/domain.
    Solvent only enforces whether debt is empty or non-empty for promotion.

Keep the functional taxonomy outside the kernel and explicitly
non-normative.

It may describe roles such as:
- provenance
- consistency
- scope
- verification
- impact
- recovery

But it must NOT become:
- an enum;
- a default;
- a Solvent schema;
- an enforcement rule.

==================================================
16. DECISION RECORD
==================================================

Create/update the decision record in docs/OS/.

Lead with:

    This change restores documented doctrine: debt vocabulary belongs
    to the application, policy, or deployment. It does not expand the
    Solvent kernel.

Record:
- why the kernel was reopened;
- repository evidence;
- removal of FullDebt;
- wizard ownership;
- EBP Coordinator ownership;
- adapter boundary rules;
- grandfathering;
- migration behavior;
- safety responsibility by domain;
- tests;
- final commit;
- supersession of 7602699.

Also document:

    Wizard:
        belief.WizardDebt()

    EBP:
        Coordinator-owned ebpInitialDebt

    Operator review:
        same shared wizard vocabulary where applicable

    Demo:
        same shared wizard vocabulary where applicable

    Independent domains:
        own their own policy.

==================================================
17. VERIFICATION
==================================================

Run:

    targeted tests
    -> full task test
    -> migration/bootstrap verification
    -> repository-wide vocabulary sweep

The vocabulary sweep should establish:

Allowed:
- internal/belief/debt.go
- wizard/domain usage
- legitimate demo/example usage
- test fixtures
- decision documentation

Forbidden:
- kernel defaults
- kernel validators
- generic Solvent enum/whitelist
- MCP universal vocabulary.

Do not treat legitimate test literals or the decision record as failures.

==================================================
18. ACCEPTANCE CRITERIA
==================================================

Do not declare complete until all are true:

1. Full test suite passes.
2. kernel.FullDebt is removed.
3. No kernel-level debt vocabulary remains.
4. Database default is empty after 010.
5. Existing rows remain unchanged.
6. Arbitrary debt is accepted by Solvent.
7. Kernel accepts opaque empty-string values.
8. API rejects empty-string values.
9. MCP rejects empty-string values.
10. Promotion gate is unchanged.
11. Action-intent gate is unchanged.
12. Retraction cascade is unchanged.
13. Refusal logging is unchanged.
14. All bootstrap paths apply 010.
15. Wizard vocabulary has one source of truth.
16. mapping.go uses named constants.
17. No package cycle introduced.
18. Old semantic tests retain their strength.
19. Decision record is written.
20. Repository-wide vocabulary sweep passes.
21. New freeze is tagged.
22. 7602699 is explicitly recorded as superseded.
23. No universal replacement vocabulary was introduced.

==================================================
19. RE-FREEZE
==================================================

Final sequence:

    implementation
    -> targeted tests
    -> full suite
    -> migration/bootstrap verification
    -> repository-wide vocabulary sweep
    -> decision record
    -> commit
    -> annotated tag on exact freeze commit
    -> baseline references updated
    -> supersession of 7602699 recorded

Use the established repository convention discovered during inspection.

Do not leave the freeze as floating HEAD.

==================================================
20. IMPLEMENTATION DISCIPLINE
==================================================

This is a narrow kernel contraction.

Do NOT:
- redesign Solvent;
- change Conductor;
- add a debt table;
- add a universal debt enum;
- add EBP knowledge to Solvent;
- add semantic reasoning;
- add autonomous promotion;
- add automatic debt retirement;
- introduce unrelated refactors;
- change unrelated APIs.

When implementation encounters ambiguity:
- resolve from the approved plan and actual repository evidence;
- preserve the domain/kernel boundary;
- choose the smallest change;
- do not invent a new abstraction.

Proceed with implementation now.
```

The remaining review notes are implementation hygiene rather than architecture blockers: add `agentjacking` to the dependency check, document the defensive-copy invariant, classify CS_DEBT08 correctly as existing versus new coverage, use pattern-based edits in `mcp_verify.sh`, and make the vocabulary-sweep acceptance condition explicit. 

At this point the correct move is **implementation, tests, evidence, and re-freeze—not another design cycle**.
