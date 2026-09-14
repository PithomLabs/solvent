PERFORM A FINAL ADVERSARIAL CODE REVIEW OF THE CURRENT UNCOMMITTED SOLVENT DEBT-OPAQUE CHANGE.

IMPORTANT:
- Do NOT modify any files.
- Do NOT commit.
- Do NOT create or move tags.
- Do NOT “clean up” unrelated code.
- Do NOT assume the prior implementation report is correct.
- Treat the current working tree as potentially defective.
- Your job is to try to break the implementation and find real defects before the owner commits it.

The implementation has already passed:
- go build ./...
- go vet ./...
- task test: 504 passed, 0 failed
- MCP verification: all checks green
- live CockroachDB migration verification
- adversarial review that fixed one regression (D-1)

Do NOT simply repeat those results. Independently review the actual code and current diff.

==================================================
1. ESTABLISH REVIEW BASELINE
==================================================

Run:

    git status --short
    git diff --stat
    git diff
    git diff --check

Confirm there are:
- no accidental deletions;
- no debug statements;
- no temporary instrumentation;
- no unrelated refactors;
- no generated artifacts accidentally added;
- no test-only hacks weakening production behavior.

Do not modify anything.

==================================================
2. REVIEW THE ARCHITECTURAL INVARIANT
==================================================

The central invariant is:

    DEBT IS OPAQUE TO SOLVENT.

Attack this claim.

Verify the Solvent kernel:
- never enumerates debt identifiers;
- never validates vocabulary;
- never normalizes case;
- never trims whitespace;
- never sorts debt;
- never deduplicates debt;
- never interprets semantic meaning;
- never injects the old six-item default;
- only cares whether the debt array is empty or non-empty for promotion.

Search production code for:

    FullDebt
    NeedProvenanceCheck
    NeedContradictionSweep
    NeedBlastRadius
    NeedRollbackPlan
    NeedVersionPin
    NeedOperatorSignoff

Classify every occurrence.

Flag any occurrence in the kernel that reintroduces semantic coupling.

==================================================
3. ATTACK THE THREE-LAYER CONTRACT
==================================================

Required behavior:

    Kernel:
        [""] is accepted as opaque debt

    HTTP API:
        [""] is rejected

    MCP:
        debt_item == "" is rejected

Test/inspect actual implementation paths, not just existing tests.

Look for bypasses:
- another API endpoint;
- another service method;
- direct database insertion path;
- another MCP tool;
- another handler;
- alternate JSON decoding path.

Determine whether malformed debt can bypass boundary validation and reach places where it should not.

Do NOT conclude "safe" merely because CS_DEBT11/12/13 exist.

==================================================
4. ATTACK THE EMPTY DEFAULT
==================================================

Verify every new-belief path.

Look for any path that can create a belief:
- API
- MCP
- internal packages
- wizard
- demo
- operator review
- examples
- tests
- direct SQL
- helper/factory methods

Determine:
- which paths intentionally attach domain debt;
- which paths rely on empty database default;
- whether any production path accidentally creates an immediately promotable belief when it should attach obligations.

The database default must be empty.

The application/domain layer must own policy.

Do NOT "fix" this by introducing a universal default.

==================================================
5. ATTACK WIZARD SINGLE SOURCE OF TRUTH
==================================================

Inspect internal/belief/debt.go and every caller.

Verify:
- exactly one vocabulary definition;
- named constants used;
- no positional indexing;
- WizardDebt() returns a defensive copy;
- package-level backing slice cannot be mutated externally;
- mapping.go references named constants;
- wizard consumers use WizardDebt() where crossing package boundaries;
- no duplicate six-item slice exists elsewhere.

Pay special attention to:

    internal/wizard/state.go

The current review identified pre-existing bare string literals in map keys.

Determine whether any of them are actually debt identifiers and therefore violate
the new single-source-of-truth contract, versus merely being unrelated map keys.

Do NOT automatically refactor pre-existing unrelated literals.

==================================================
6. ATTACK TEST MIGRATION
==================================================

Review every replacement of kernel.FullDebt in tests.

Classify:
A. Test needs only non-empty debt
B. Test depends on exact wizard vocabulary/lifecycle

Look specifically for:
- tests that became weaker because six debts became one;
- tests that still assume debt length six when they should not;
- tests that accidentally use wizard vocabulary for generic kernel behavior;
- tests that pass because the implementation is wrong but the test no longer exercises the old invariant;
- tests that assert only errors instead of resulting database state.

Look at:
- promotion-after-final-debt;
- discharge completeness;
- integration lifecycle;
- wizard HTTP flows;
- mapping tests.

The goal is not merely "all tests pass."
The goal is "the tests still prove the intended invariants."

==================================================
7. ATTACK MIGRATION 010
==================================================

Inspect:

    db/010_debt_opaque.sql

Verify:
- correct database syntax;
- correct table/column;
- idempotency consistent with actual bootstrap behavior;
- no accidental row rewrite;
- no unintended constraint changes.

Inspect every schema path.

Try to find any path where:
- 010 is omitted;
- 004 remains authoritative;
- migrations are applied out of order;
- tests and production use different schema sequences.

Pay attention to:
- Taskfile;
- CLI resolveSchemaPaths;
- cloud init;
- test fixtures;
- reset helpers;
- direct schema-loading tests.

Do not trust the migration test alone.

==================================================
8. ATTACK GRANDFATHERING
==================================================

Verify an existing belief created before 010:
- retains historical debt;
- remains readable;
- remains retireable;
- does not get implicitly reinterpreted;
- does not get translated into the new domain vocabulary.

Verify that old wizard beliefs continue to use the identifiers the wizard understands.

==================================================
9. ATTACK PROMOTION / AUTHORITY INVARIANTS
==================================================

Independently inspect these paths:

    Promote
    IntentOnPromoted
    RetractCascade
    refusal_log
    AuditLiveOnNonPromoted

Verify no accidental semantic changes.

Specifically attack:

    debt non-empty -> promotion blocked
    debt empty -> promotion may proceed if other gates pass
    final_truth -> promotion remains blocked
    live action intent -> promoted belief required
    retraction -> live intents cancelled before beliefs retract

Look for subtle bypasses introduced by changing how debt is created.

==================================================
10. ATTACK RETIREDEBT SEMANTICS
==================================================

Verify:
- nonexistent belief -> ErrBeliefNotFound;
- existing belief + missing debt item -> idempotent no-op;
- duplicate values -> array_remove semantics remain unchanged;
- arbitrary strings can be retired;
- no vocabulary validation remains anywhere below the owning domain.

Inspect transaction boundaries and SQL carefully.

Do not rely exclusively on tests.

==================================================
11. ATTACK API CONTRACT
==================================================

Review:

    EnterBeliefRequest

Required semantics:

    debt omitted -> []
    debt explicitly [] -> []
    debt arbitrary -> preserved
    debt [""] -> rejected at API boundary

Check:
- JSON tags;
- nil handling;
- unexpected mutation;
- any alternate decoding path;
- error status/code consistency;
- whether API silently supplies wizard debt where it should not.

The generic API must remain domain-agnostic.

==================================================
12. ATTACK MCP CONTRACT
==================================================

Verify:
- no debt enum;
- no hidden FullDebt-derived schema;
- arbitrary identifier accepted;
- empty-string rejected;
- description matches actual behavior;
- belief lookup remains correct;
- retirement semantics remain correct.

Look for stale assumptions in:
- tool descriptions;
- examples;
- shell verification;
- JSON schema;
- help text.

==================================================
13. ATTACK CALLER OWNERSHIP
==================================================

Inspect every production caller of belief creation.

For each classify:

    Wizard domain
    EBP/external coordinator
    independent domain
    generic infrastructure
    example/test

Verify:
- Wizard callers explicitly attach wizard debt.
- Generic infrastructure does NOT silently manufacture policy.
- Independent domains own their own starting debt.
- No caller unexpectedly relies on Solvent empty default where policy requires starting obligations.

Because the EBP Coordinator is external to this repository:
- verify only the integration contract/documentation here;
- do not invent Coordinator implementation;
- ensure the Solvent code does not accidentally assume EBP vocabulary.

==================================================
14. ATTACK "OPAQUE" SEMANTICS THROUGH HELPERS
==================================================

Search for functions that appear harmless but could secretly recreate vocabulary semantics:

- validation helpers;
- factory functions;
- constructors;
- test helpers;
- schema helpers;
- MCP helpers;
- error classification;
- debt formatting;
- serialization/deserialization code.

A helper that contains the six-item list is still a hardcoded vocabulary,
even if kernel.go no longer does.

==================================================
15. ATTACK DOCUMENTATION DRIFT
==================================================

Search for stale text such as:

    "FullDebt"
    "six debts"
    "all six"
    "valid debt items"
    "enum"
    "known debt"
    "supported debt"

Classify each occurrence:
- obsolete;
- legitimate wizard-domain documentation;
- historical decision record;
- test fixture.

Flag stale generic Solvent wording.

Do NOT rewrite historical decision records merely to hide history.

==================================================
16. ATTACK DATABASE DEFAULT + APPLICATION DEFAULT INTERACTION
==================================================

Look for accidental double defaults such as:

    database = []
    API = []
    internal factory = wizardDebt

Determine whether each is intentional.

The correct model is:

    Solvent generic path -> empty unless caller supplies debt
    Wizard path -> explicit wizard debt
    EBP Coordinator -> explicit EBP debt
    independent domain -> explicit domain debt

Flag any production path where a generic layer silently substitutes policy.

==================================================
17. ADVERSARIAL CHANGE-SCOPE REVIEW
==================================================

The intended change is narrow:
- remove kernel vocabulary;
- expose caller debt;
- move wizard vocabulary to domain;
- update migrations;
- preserve authority semantics.

Identify any changed behavior that is not necessary for that objective.

For each suspicious change:
- file;
- exact behavior;
- why it is unrelated;
- whether it should be reverted.

Do NOT make the change yourself.

==================================================
18. MANUAL DIFF REVIEW OF HIGH-RISK FILES
==================================================

Read actual diffs carefully for at least:

    kernel/kernel.go
    kernel/contract.go
    api/types.go
    api/belief.go
    cmd/solvent-mcp/tools.go
    cmd/solvent-mcp/main.go
    internal/belief/debt.go
    internal/belief/belief.go
    internal/belief/mapping.go
    internal/wizard/seed.go
    internal/wizard/discharge.go
    internal/wizard/state.go
    db/010_debt_opaque.sql
    Taskfile.yml
    cmd/solvent/main.go
    demo/cloud/init/main.go

Also inspect any other production file with material FullDebt changes.

==================================================
19. DO NOT FIX CODE
==================================================

This is a review-only pass.

If a defect is found:
- do not edit it;
- record it;
- assign severity:
    BLOCKER
    HIGH
    MEDIUM
    LOW
- explain exact location;
- explain why it matters;
- give minimal recommended fix.

Do not create a commit.

Do not create a tag.

==================================================
20. FINAL REVIEW REPORT
==================================================

Return:

A. Executive verdict:
    APPROVE
    APPROVE WITH NON-BLOCKING FINDINGS
    BLOCK — FIX REQUIRED

B. Findings table:

    ID
    Severity
    File/line
    Finding
    Why it matters
    Recommended fix

C. Invariant verification:
    debt opaque
    API/MCP boundary
    wizard vocabulary ownership
    promotion
    action intent
    retraction
    migration
    grandfathering

D. Scope review:
    unrelated changes, if any

E. Test-quality assessment:
    whether tests still prove the intended behavior

F. Exact blockers before commit, if any.

G. Explicit statement:
    "No files modified during this review."
    "No commit created."
    "No tag created."

Do not give a clean approval merely because the prior test suite passed.

Your job is to find something the implementer may have missed.
Only report issues that are supported by actual repository evidence.