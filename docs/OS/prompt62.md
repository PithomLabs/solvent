RUN THE PENDING DATABASE TESTS AND PERFORM A FINAL ADVERSARIAL CODE REVIEW.

IMPORTANT:
- Do NOT commit any code.
- Do NOT create or move git tags.
- Do NOT modify git history.
- I will perform the commit and freeze/tag myself.
- You may modify code only if the adversarial review finds a real defect required to satisfy the approved implementation plan. Any such modification must be reported explicitly.
- Do not make cosmetic refactors.

The approved implementation is the Solvent debt-opaque change from Plan 11.1.

==================================================
PHASE 1 — DATABASE AVAILABILITY
==================================================

First inspect the repository's existing CockroachDB startup/test conventions.

Determine whether CockroachDB is:
- already available;
- expected through docker/compose;
- expected through a Taskfile command;
- expected through another local development mechanism.

Use the repository's existing mechanism.

Do not invent a new database setup unless the existing mechanism is absent.

Once CockroachDB is running and reachable, verify connectivity before running the full suite.

Report:
- how CockroachDB was started/found;
- connection endpoint;
- whether the database is clean/fresh or reused;
- any relevant environment variables used.

Do not expose secrets.

==================================================
PHASE 2 — RUN TARGETED TESTS FIRST
==================================================

Run the debt-focused tests first.

At minimum exercise:

Kernel:
    CS_DEBT01
    CS_DEBT02
    CS_DEBT03
    CS_DEBT04
    CS_DEBT05
    CS_DEBT06
    CS_DEBT07
    CS_DEBT08
    CS_DEBT09
    CS_DEBT10
    CS_DEBT11

API:
    CS_DEBT12

MCP:
    CS_DEBT13

Migration:
    CS_DEBT14

Also run:
    B-17
    B-23

and the renamed:
    TestIntegration_RealFixtureRetiresWizardDebt

Do not assume names or locations if the implementation uses different test naming; locate the actual tests.

==================================================
PHASE 3 — RUN FULL TEST SUITE
==================================================

Run:

    task test

If Taskfile conventions differ, use the repository's canonical full-test command.

The full test suite must execute against a real CockroachDB instance.

Do NOT convert database tests to mocks just to make them pass.

Record:
- total result;
- failures;
- package;
- exact failing test;
- whether failure is infrastructure, migration, SQL semantics, or Go logic.

==================================================
PHASE 4 — RUN MCP VERIFICATION
==================================================

Run:

    ./scripts/mcp_verify.sh

or the repository's correct executable invocation if permissions/path differ.

Verify specifically:

1. No universal debt enum remains.
2. Arbitrary debt identifiers are accepted.
3. Unknown debt identifiers are not vocabulary-rejected.
4. Empty-string debt_item is rejected.
5. Existing belief lookup behavior still works.
6. Existing retirement behavior still works.

Report every failure separately.

==================================================
PHASE 5 — MIGRATION ADVERSARIAL VALIDATION
==================================================

Do not merely rely on the test suite.

Explicitly verify the migration behavior.

CASE A — existing database with old default:

1. Apply schema through 004.
2. Insert a belief without specifying debt.
3. Confirm it receives the historical six-item default.
4. Apply 010.
5. Insert a new belief without specifying debt.
6. Confirm new debt is empty.
7. Confirm the old belief still has the historical six-item debt.

CASE B — fresh schema:

1. Apply complete schema chain through 010.
2. Insert a belief without specifying debt.
3. Confirm debt is empty.

CASE C — reset/bootstrap paths:

Verify the repository's actual:
- test reset;
- cloud init;
- CLI reset;
- Taskfile db:reset

all produce the empty default after migration 010.

Do not mutate production or remote environments.

Use local/test databases only.

==================================================
PHASE 6 — ADVERSARIAL CODE REVIEW
==================================================

After tests pass, review the actual diff as an adversarial senior
architect, not as the original implementer.

Focus on the following attack surfaces.

--------------------------------------------------
A. DOMAIN/KERNEL COUPLING
--------------------------------------------------

Search for:

    kernel.FullDebt
    needProvenanceCheck
    needContradictionSweep
    needBlastRadius
    needRollbackPlan
    needVersionPin
    needOperatorSignoff

Verify:
- no kernel defaulting remains;
- no generic Solvent vocabulary validation remains;
- no MCP enum remains;
- no hidden helper recreates the six-item default;
- no duplicated wizard vocabulary remains unnecessarily.

Flag any accidental reintroduction of coupling.

--------------------------------------------------
B. OPAQUE-DEBT INVARIANT
--------------------------------------------------

Verify the kernel accepts arbitrary values including:

    "foo"
    "completelyUnknownDebt"
    ""

The kernel must not inspect semantic contents.

Verify boundaries behave differently:

    API:
        "" rejected

    MCP:
        "" rejected

    kernel:
        "" accepted

This three-layer distinction must be preserved.

--------------------------------------------------
C. WIZARD SINGLE SOURCE OF TRUTH
--------------------------------------------------

Verify:
- named constants exist;
- mapping.go references those constants;
- wizard consumers use WizardDebt();
- WizardDebt() returns a defensive copy;
- no caller mutates the package-level slice;
- no second six-item literal list exists.

Verify that operator-review, demo, and agentjacking reuse the same domain source where appropriate.

--------------------------------------------------
D. PACKAGE DEPENDENCIES
--------------------------------------------------

Run a package/build-level check for import cycles.

Verify:
    belief -> wizard = no
    wizard -> belief = allowed
    agentjacking -> belief = allowed
    no transitive cycle introduced

--------------------------------------------------
E. TEST QUALITY
--------------------------------------------------

Inspect the replacement of kernel.FullDebt in tests.

Ensure tests that merely need non-empty debt use simple arbitrary test debt.

Ensure tests that actually depend on wizard semantics retain the full wizard vocabulary.

Look for false-positive tests where a reduced debt set accidentally makes a
test easier and therefore weaker.

Pay particular attention to:
- discharge completeness;
- promotion-after-final-retirement;
- wizard lifecycle;
- integration fixtures;
- debt count assertions.

Verify B-17 and B-23 test distinct useful contracts.

If they are duplicates, report that rather than creating another redundant test.

--------------------------------------------------
F. PROMOTION / AUTHORITY INVARIANTS
--------------------------------------------------

Verify unchanged behavior for:
- promotion with remaining debt -> blocked;
- promotion with empty debt -> allowed when otherwise eligible;
- final-truth claims -> blocked;
- action_intent -> promoted belief required;
- retraction cascade -> intents cancelled before beliefs retract;
- refusal_log -> unchanged behavior.

Do not accept a passing test suite as sufficient if the implementation
structure obviously weakens the invariant.

--------------------------------------------------
G. GRANDFA-ATHERED DATA
--------------------------------------------------

Verify migration does not:
- rewrite existing rows;
- translate legacy debt;
- auto-retire existing debt.

Verify existing wizard beliefs remain retireable.

--------------------------------------------------
H. API/MCP CONTRACT
--------------------------------------------------

Verify:
- omitted debt -> empty;
- explicit [] -> empty;
- explicit arbitrary values preserved;
- no hidden normalization;
- no regex/length/count restrictions;
- empty string rejected only at ingress.

Check JSON schema and tool descriptions for consistency with actual behavior.

--------------------------------------------------
I. MIGRATION / BOOTSTRAP DRIFT
--------------------------------------------------

Search for every schema loader/path.

Verify 010 is not missing from:
- tests;
- CLI;
- cloud initialization;
- Taskfile/reset;
- any other discovered bootstrap path.

Check that no alternate initialization path silently leaves 004's old default active.

--------------------------------------------------
PHASE 7 — GIT DIFF REVIEW
--------------------------------------------------

Run:

    git status --short
    git diff --stat
    git diff

Do not commit anything.

Review for:
- accidental unrelated changes;
- generated files;
- debug statements;
- temporary test code;
- stale comments mentioning FullDebt;
- stale documentation;
- test names containing obsolete concepts;
- migration mistakes;
- formatting noise.

Also search for:

    FullDebt
    "full debt"
    obsolete enum descriptions
    old MCP vocabulary expectations

in all relevant source/docs/test files.

==================================================
PHASE 8 — ONLY FIX REAL DEFECTS
==================================================

If the adversarial review discovers a genuine implementation defect:

1. Explain the defect.
2. Explain why it violates the approved architecture/contract.
3. Make the smallest necessary fix.
4. Re-run affected tests.
5. Re-run full test suite if the change can affect unrelated behavior.
6. Re-run MCP verification if relevant.
7. Re-run the adversarial check.

Do NOT fix:
- cosmetic issues;
- unrelated technical debt;
- architectural ideas outside this change;
- speculative future improvements.

==================================================
PHASE 9 — FINAL REPORT
==================================================

Do NOT commit.

Do NOT tag.

Return a final implementation report with:

1. CockroachDB status
2. Targeted test results
3. Full `task test` result
4. MCP verification result
5. Migration pre/post verification
6. Bootstrap-path verification
7. Adversarial review findings
8. Any defects fixed during review
9. Remaining risks, if any
10. Exact git working-tree status

The final status must clearly distinguish:

    PASS — verified by running
    PASS — verified by static inspection
    NOT VERIFIED — infrastructure limitation
    DEFECT FOUND — fixed
    DEFECT FOUND — unresolved

Most importantly:

Do NOT say "tests will pass" based on compilation.

Only claim a database-backed behavior passes after it has actually been
executed against CockroachDB.

Leave the repository uncommitted for manual review and commit by the owner.