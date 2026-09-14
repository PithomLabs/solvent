PERFORM A FINAL ADVERSARIAL DIFF REVIEW OF THE CURRENT UNCOMMITTED DEBT-OPAQUE CHANGE.

IMPORTANT:
- Review only. DO NOT modify files.
- DO NOT commit.
- DO NOT create or move tags.
- Do NOT make cleanup changes.
- Do NOT re-run the entire architecture/planning exercise.
- Treat the current working tree and diff as potentially defective.
- The implementation has already passed:
    go build ./...
    go vet ./...
    targeted tests
    task test: 504 passed, 0 failed
    ./scripts/mcp_verify.sh: green
- Your purpose now is to find anything the implementation report or tests failed to catch.

Current known deferred finding:
    internal/m0/gate.go:42 contains SQL fixture claim text
    "claim carrying its full starting debt"
    This is outside corrective-pass scope because internal/m0 is a standalone
    verification tool with no _test.go files and is not executed by task test.

Do NOT automatically reopen that finding. Determine whether the current diff
creates any NEW issue related to it.

==================================================
1. REVIEW THE ACTUAL DIFF
==================================================

Run:

    git status --short
    git diff --stat
    git diff --check
    git diff

Review every changed production file, test, migration, and documentation file.

Do not rely on the previous summary.

Look for:
- accidental changes;
- omitted required changes;
- unrelated refactors;
- stale comments;
- weakened tests;
- test-only behavior leaking into production;
- generated artifacts;
- temporary/debug code;
- changes outside the approved scope.

==================================================
2. VERIFY THE FINAL CONTRACT
==================================================

The central invariant remains:

    DEBT IS OPAQUE TO SOLVENT.

Confirm the final diff preserves:

Solvent:
    stores debt;
    accepts arbitrary opaque strings;
    does not enumerate vocabulary;
    does not normalize;
    does not inspect semantics;
    promotion only cares about empty/non-empty debt.

API/MCP:
    omitted debt -> empty;
    explicit [] -> empty;
    arbitrary debt -> preserved;
    empty string -> rejected at ingress.

Wizard:
    owns its six-item vocabulary;
    uses one source of truth;
    uses named constants;
    no duplicate literals.

Do not infer correctness from test names alone. Inspect actual changed code.

==================================================
3. ATTACK THE F-1 FIX
==================================================

Inspect:

    demo/cloud/init/main.go

Verify resetAndSeed now applies:

    004
    then 010

and that 010 is actually passed to the same ApplySchema execution path.

Confirm that 005–008 were intentionally NOT added and that this is consistent
with resetAndSeed's actual table usage.

Specifically verify:
- resetAndSeed only requires the four core tables;
- seed operations do not touch tables from 005–008;
- 010 still runs after 004;
- no reset path can leave the old six-item default active.

Do NOT recommend adding 005–008 merely for symmetry.

==================================================
4. ATTACK THE F-2 FIX
==================================================

Inspect:

    internal/wizard/state.go

Verify every debt identifier used there now references the named constants,
not string literals.

Confirm:
- checkPrompts uses belief.Need* constants;
- retrievalChecks uses belief.Need* constants;
- ContradictionCheck does not introduce another vocabulary;
- no new duplicate debt strings were introduced elsewhere.

Also inspect the repository for the six identifiers and classify every
remaining production occurrence.

Expected ownership:
- internal/belief/debt.go;
- legitimate consumers;
- legitimate historical/docs/test fixtures.

Flag any new duplicate source of truth.

==================================================
5. ATTACK F-7 COMMENT FIXES
==================================================

Search for stale claims such as:

    "DDL default"
    "FullDebt"
    "full starting debt"
    "six items the DDL default issued"
    "full debt"

Classify each occurrence:
- valid historical documentation;
- valid domain documentation;
- stale production/test comment;
- fixture text;
- generated artifact;
- standalone unrelated tool.

Do not require removal of legitimate historical references.

But flag any changed file whose comments now contradict actual behavior.

==================================================
6. ATTACK MIGRATION ORDER
==================================================

Inspect all schema paths touched by the corrective pass.

Confirm:

    004 -> 010

is preserved everywhere relevant.

Verify:
- no reset path applies 010 before 004;
- no path omits 010 after applying 004;
- no code path accidentally edits historical migrations;
- existing rows are not rewritten.

Do not propose modifications to 001/004 unless the diff has actually made
their semantics inconsistent in a way that affects a supported path.

==================================================
7. ATTACK THE SINGLE-SOURCE-OF-TRUTH CLAIM
==================================================

Inspect:

    internal/belief/debt.go
    internal/belief/mapping.go
    internal/wizard/*
    demo/cloud/init/*
    cmd/operator-review/*
    internal/agentjacking/*

Verify:
- named constants are the only six-item definitions;
- WizardDebt() is a defensive copy;
- no caller mutates the internal backing slice;
- no positional indexing;
- no independent six-item slice has appeared.

Search for all six literals individually.

Report any production duplicate.

==================================================
8. ATTACK TEST QUALITY
==================================================

Do not simply state "504 tests pass."

Review the changed tests for regressions in test strength.

Specifically inspect:
- tests converted from kernel.FullDebt;
- tests now using []string{"testDebt"};
- tests using belief.WizardDebt();
- B-17;
- B-23;
- CS_DEBT01-14;
- TestW3_EnsureBelief_New;
- wizard lifecycle tests;
- integration tests.

Look for:
- cardinality assertions accidentally weakened;
- tests that now use one debt where multiple debts are semantically required;
- tests that pass vacuously;
- tests that only assert errors but not resulting state;
- tests that accidentally depend on domain vocabulary when they should test
  generic kernel behavior.

The existing multi-item coverage includes arbitrary acceptance,
retirement, and duplicate removal. Verify those tests actually exercise it.

==================================================
9. ATTACK PROMOTION/AUTHORITY SEMANTICS
==================================================

Inspect the diff for accidental changes to:

    Promote
    IntentOnPromoted
    RetractCascade
    refusal_log
    final_truth
    promoted_is_debt_free

Verify:
- remaining debt blocks promotion;
- empty debt does not bypass other gates;
- final_truth restriction unchanged;
- action_intent remains tied to promoted belief;
- retraction ordering unchanged;
- refusal behavior unchanged.

Do not infer this solely from passing tests; inspect relevant SQL/code diffs.

==================================================
10. ATTACK API/MCP BOUNDARIES
==================================================

Verify actual changed code:

API:
    nil -> []
    omitted -> []
    [] -> []
    arbitrary -> preserve
    [""] -> reject

MCP:
    arbitrary -> accept
    unknown -> accept
    "" -> reject
    no enum
    no vocabulary guard

Look for alternative handlers or helpers that bypass these boundary rules.

==================================================
11. ATTACK HISTORICAL COMPATIBILITY
==================================================

Confirm:
- migrations 001 and 004 were not modified;
- existing historical beliefs remain untouched;
- old wizard beliefs remain retireable;
- migration 010 changes only the future default.

Inspect whether any changed test/helper accidentally assumes historical beliefs
have the new empty debt.

==================================================
12. ATTACK SCOPE
==================================================

Compare actual changed files to intended scope.

Expected categories:
- 010 migration / schema registration;
- kernel FullDebt removal;
- debt domain source;
- wizard/domain callers;
- API/MCP;
- tests;
- directly affected docs/comments.

Flag anything that appears unrelated.

In particular, scrutinize:
- generated files;
- docs changed only because timestamps changed;
- unrelated business logic;
- package refactors;
- API changes unrelated to debt.

==================================================
13. ATTACK THE KNOWN DEFERRED M0 FINDING
==================================================

Search:

    internal/m0/gate.go

The existing claim text:

    "claim carrying its full starting debt"

is known to be stale relative to the new semantics.

Do NOT automatically require fixing it in this change.

Instead determine:

A. Is it touched by the current diff?
B. Is it now referenced by any changed production/test path?
C. Does the current change make the fixture actively misleading to a
   supported workflow?
D. Does changing it now require broadening scope?

Only report it as a blocker if the current diff creates a new dependency that
makes the stale fixture materially dangerous.

Otherwise keep it explicitly deferred.

==================================================
14. ADVERSARIAL SEARCHES
==================================================

Run searches for:

    kernel.FullDebt
    FullDebt
    needProvenanceCheck
    needContradictionSweep
    needBlastRadius
    needRollbackPlan
    needVersionPin
    needOperatorSignoff
    "DDL default"
    "full starting debt"
    "full debt"

For every match, classify:
- kernel;
- domain;
- test;
- docs;
- historical record;
- fixture;
- unrelated tool.

Do not treat every textual match as a defect.

==================================================
15. FINAL VERDICT
==================================================

Return exactly:

A. VERDICT
    APPROVE
    APPROVE WITH NON-BLOCKING FINDINGS
    BLOCK — FIX REQUIRED

B. FINDINGS TABLE

Columns:
    ID
    Severity
    File/line
    Finding
    Evidence
    Recommended action

Severity:
    BLOCKER
    HIGH
    MEDIUM
    LOW

C. INVARIANT STATUS
    debt opaque
    migration ordering
    wizard SSoT
    API boundary
    MCP boundary
    promotion
    action intent
    retraction
    grandfathering

D. DIFF/SCOPE STATUS
    unrelated changes?
    stale comments?
    weakened tests?
    omitted required change?

E. KNOWN DEFERRED M0 FINDING
    confirm whether it remains correctly deferred.

F. FINAL COMMIT READINESS
    READY
    NOT READY

==================================================
16. HARD CONSTRAINTS
==================================================

Do NOT:
- modify files;
- commit;
- tag;
- rewrite history;
- perform unrelated cleanup;
- reopen architecture.

The repository must remain exactly as found.

End with explicit statements:

    No files modified.
    No commit created.
    No tag created.