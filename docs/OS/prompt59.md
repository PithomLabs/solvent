You are implementing a narrowly scoped Solvent kernel fix.

OBJECTIVE

Fix Solvent's hardcoded, deployment-specific debt vocabulary while preserving the existing epistemic/authority semantics and schema design.

The architectural invariant is:

    DEBT IS OPAQUE TO SOLVENT.

Solvent must know only that:
- a belief may have zero or more debt items;
- debt items are opaque identifiers supplied by the caller;
- debt may be retired one item at a time;
- promotion is forbidden while any debt remains;
- promotion is forbidden for final-truth claims;
- all existing promotion, intent, retraction, and refusal gates remain unchanged.

The domain/application layer defines the meaning of debt identifiers.

DO NOT redesign Solvent.
DO NOT add a debt table.
DO NOT add a debt vocabulary table.
DO NOT introduce EBP/BM-IST/domain concepts into Solvent.
DO NOT modify Conductor.
DO NOT create a third research-state database.
DO NOT change the semantics of promotion, action_intent, retraction cascade, or refusal logging.

--------------------------------------------------
CURRENT PROBLEM
--------------------------------------------------

The current belief schema hardcodes a six-item default debt vocabulary:

    needProvenanceCheck
    needContradictionSweep
    needBlastRadius
    needRollbackPlan
    needVersionPin
    needOperatorSignoff

Those are deployment/domain-specific obligations and therefore violate Solvent's domain-agnostic boundary.

The required replacement is:

1. `belief.debt` remains `TEXT[]`.
2. Its default becomes an empty array.
3. `EnterBelief` accepts caller-supplied debt values.
4. Solvent treats debt strings as opaque.
5. Existing `RetireDebt` behavior remains generic.
6. Promotion still requires an empty debt array.
7. Existing database gates remain authoritative.

Desired schema semantics:

    debt TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]

Use the syntax that is actually compatible with the repository's CockroachDB/Postgres setup.

--------------------------------------------------
STEP 1 — INSPECT BEFORE EDITING
--------------------------------------------------

First inspect the repository and determine:

- current Solvent commit/version;
- current schema migrations;
- exact `belief` schema;
- exact `EnterBelief` implementation and signature;
- all callers of `EnterBelief`;
- exact `RetireDebt` implementation;
- exact `Promote` implementation;
- `action_intent` implementation and tests;
- `RetractCascade` implementation and tests;
- refusal-log implementation and tests;
- existing migration numbering;
- repository conventions for migrations and database tests.

Do not assume filenames, migration numbers, or function signatures until inspected.

Produce a concise implementation plan internally before editing.

--------------------------------------------------
STEP 2 — DATABASE CHANGE
--------------------------------------------------

Modify the `belief.debt` definition so that new beliefs default to no debt:

    debt TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]

Do not change:
- `belief.id`
- `scenario_id`
- `claim`
- `claim_type`
- `status`
- `final_truth`
- promotion CHECK constraint
- unique constraints
- `belief_edge`
- `evidence`
- `action_intent`

Prefer a proper migration if the repository uses immutable migrations and the current database may already exist.

The migration must be safe and minimal.

Do not create:
- `debt`
- `debt_type`
- `debt_definition`
- `debt_vocabulary`
- any equivalent table.

--------------------------------------------------
STEP 3 — ENTER BELIEF API
--------------------------------------------------

Change `EnterBelief` so debt is supplied by the caller.

Conceptually:

    EnterBelief(ctx, scenarioID, claim, claimType, debt)

where `debt` is `[]string` or the repository's idiomatic equivalent.

Requirements:

- arbitrary strings must be accepted;
- empty debt must be accepted;
- nil/empty input must result in zero debt;
- Solvent must not validate debt names against a vocabulary;
- Solvent must not interpret debt strings;
- preserve all existing validation unrelated to debt;
- preserve generated canonical IDs;
- preserve existing return behavior unless a change is strictly required.

Do not silently invent a default debt vocabulary elsewhere in the Go code.

Search the entire repository for the old six debt names and eliminate any Solvent-level defaulting associated with them.

--------------------------------------------------
STEP 4 — RETIRE DEBT
--------------------------------------------------

Keep `RetireDebt` generic.

It should continue to:
- remove the requested opaque string from the array;
- be idempotent when the item is already absent, according to current behavior;
- avoid introducing vocabulary validation.

Do not redesign this API unless the current implementation makes arbitrary debt impossible.

--------------------------------------------------
STEP 5 — PRESERVE PROMOTION GATE
--------------------------------------------------

The promotion rule must remain:

    status = entered
    AND debt is empty
    AND final_truth = false

Promotion must fail when even one arbitrary debt item remains.

Promotion must succeed when debt is empty and all other existing requirements are satisfied.

Do not move this authority into application code.

The database remains authoritative.

--------------------------------------------------
STEP 6 — PRESERVE CONSEQUENTIAL AUTHORITY SEMANTICS
--------------------------------------------------

Do not change the following behavior:

`action_intent`
- live action intents require a promoted belief;
- existing foreign-key / status protections remain intact.

`RetractCascade`
- preserve transactional behavior;
- cancel live intents before retracting affected beliefs;
- preserve descendant handling;
- preserve existing failure/refusal semantics.

`refusal_log`
- preserve the existing refusal logging mechanism;
- do not add debt-specific refusal categories merely because debt is now generic.

This change is about debt vocabulary, not authority semantics.

--------------------------------------------------
STEP 7 — TESTS
--------------------------------------------------

Add focused regression coverage.

At minimum test:

1. Arbitrary debt accepted

Create a belief with:

    ["needNullModel", "foo", "bar"]

It must succeed.

2. Empty debt accepted

Create a belief with:

    []

It must succeed.

3. Nil/no-debt input

Where idiomatic for the current API, verify nil produces empty debt rather than a hardcoded default.

4. Unknown debt accepted

Use a deliberately meaningless value such as:

    "completelyUnknownDebt"

It must be accepted.

5. Arbitrary debt retirement

Create:

    ["foo", "bar"]

Retire `"foo"`.

Verify only `"bar"` remains.

6. Promotion blocked with arbitrary debt

Create:

    ["foo"]

Attempt promotion.

Verify promotion is refused.

7. Promotion succeeds when debt is empty

Create with:

    []

Promote.

Verify promotion succeeds exactly as before.

8. Final-truth prohibition unchanged

Verify a final-truth claim still cannot be promoted.

9. Action-intent gate unchanged

Verify live action intent still requires a promoted belief.

10. Retraction cascade unchanged

Verify the existing cascade semantics, especially:

    cancel live intents first
    then retract beliefs

11. Refusal logging unchanged

Existing refusal behavior must still be recorded.

12. Existing behavior regression

Run the full existing Solvent test suite and ensure unrelated behavior is unchanged.

Prefer testing at the database/service boundary where the repository currently tests these invariants.

--------------------------------------------------
STEP 8 — SEARCH FOR DOMAIN COUPLING
--------------------------------------------------

After implementation, search the Solvent repository for all occurrences of:

    needProvenanceCheck
    needContradictionSweep
    needBlastRadius
    needRollbackPlan
    needVersionPin
    needOperatorSignoff

There must be no Solvent-level code that automatically inserts or requires those names.

Also search for any newly introduced semantic checks such as:

    allowed debt types
    known debt
    supported debt
    valid debt
    debt vocabulary
    debt definition

There must be none.

--------------------------------------------------
STEP 9 — VERIFY ARCHITECTURAL BOUNDARY
--------------------------------------------------

The resulting architecture must be:

    Domain / EBP / Coordinator
            |
            | supplies opaque debt IDs
            v
        Solvent
            |
            | only knows empty vs non-empty
            v
       Promotion Gate

Example domain-specific debt may be:

    needMap
    needInvariant
    needToyCheck
    needNullModel
    needObstruction
    needFaithfulnessReview

Those names are NOT Solvent concepts.

Do not put them into Solvent schema, constants, enums, validators, comments that imply semantic ownership, or business logic.

--------------------------------------------------
STEP 10 — DOCUMENTATION
--------------------------------------------------

Update only documentation that is directly affected by the API/schema change.

The documentation should clearly state:

    Solvent treats debt identifiers as opaque obligation IDs.
    Debt semantics are defined by the caller/domain.
    Solvent only enforces whether debt is empty or non-empty for promotion.

Do not add broad EBP documentation to the Solvent repository.

--------------------------------------------------
IMPLEMENTATION DISCIPLINE
--------------------------------------------------

This is a kernel-minimization change.

Before adding code, ask:

    Is this required to make debt domain-agnostic?

If not, do not add it.

Do not refactor unrelated code.
Do not rename unrelated APIs.
Do not modernize unrelated tests.
Do not change formatting across unrelated files.
Do not alter Conductor.
Do not introduce new abstractions unless required by the existing implementation.

--------------------------------------------------
DELIVERABLE
--------------------------------------------------

Implement the change completely.

Then report:

1. files changed;
2. migration added/modified;
3. exact `EnterBelief` API change;
4. tests added;
5. full test result;
6. confirmation that the old hardcoded debt vocabulary is no longer enforced by Solvent;
7. confirmation that promotion, action_intent, retraction cascade, and refusal logging semantics remain unchanged.

If the repository's current implementation differs from the assumptions above, adapt to the actual code while preserving the architectural requirements. Do not invent a new design merely because the existing implementation is organized differently.