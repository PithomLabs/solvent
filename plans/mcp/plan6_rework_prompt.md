# SOLVENT — PLAN 6 FINAL MCP HARDENING IMPLEMENTATION

Repository:
    https://github.com/PithomLabs/solvent

Authoritative implementation plan:
    plans/mcp/plan6.md

Previous adversarial review:
    plans/mcp/plan5_review.md

You are the coding agent implementing the FINAL MCP hardening pass.

READ:

    plans/mcp/plan6.md
    plans/mcp/plan5_review.md

before making changes.

Plan 6 is intentionally mechanical. The MCP architecture has already
converged and has been independently validated.

DO NOT redesign the architecture.

DO NOT reopen the MCP tool surface.

DO NOT modify the frozen Solvent core.

There is ONE correction to the supplied Plan 6 that you MUST incorporate:
the proposed I-7 Stage 2 regex does not actually detect ordinary
`UPDATE belief SET ...` SQL or guarantee detection of
`QueryRowContext(... UPDATE ... RETURNING ...)`.

The implementation below supersedes that specific regex while preserving
Plan 6's intended invariant.

============================================================
1. SCOPE
============================================================

Implement only:

    F-1  I-7 gate hardening
    F-2  strict belief_id validation in promote/retire_debt
    F-3  strict belief_id validation in ledger
    F-4  exact test count in plan5_imp.md

Do not introduce unrelated changes.

The expected changed files are:

    Taskfile.yml
    cmd/solvent-mcp/tools.go
    plans/mcp/plan5_imp.md

No new files are required.

============================================================
2. FROZEN CORE — ABSOLUTE
============================================================

DO NOT MODIFY:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

Do not modify:

    schema
    FullDebt
    RetractCascade
    belief_edge
    source_observed_at
    existing CLI semantics

The MCP layer must remain a thin adapter.

If you believe a frozen-core change is required:

    STOP
    report the incompatibility
    DO NOT work around it by modifying the core.

============================================================
3. F-1 — I-7 MCP BOUNDARY GATE
============================================================

File:

    Taskfile.yml

The purpose of I-7 is:

    MCP/view may read.
    MCP/view may call the existing Solvent kernel/pipeline.
    MCP/view may NOT directly perform database writes or open
    write-capable database transactions.

The existing gate was too narrow.

It missed:

    gofmt-wrapped calls
    QueryRowContext writes
    const-hoisted SQL

The supplied Plan 6 Stage 2 regex is also insufficient because:

    UPDATE belief SET ...

does not match:

    (INSERT|UPDATE|DELETE|CREATE|DROP)
    [space]
    (INTO|TABLE|FROM)

Therefore DO NOT implement that regex literally.

------------------------------------------------------------
3A. Stage 1 — Direct DB API tripwire
------------------------------------------------------------

Implement a conservative gate that rejects direct DB APIs in:

    cmd/solvent-mcp
    internal/view

At minimum detect these write-capable/direct database APIs:

    Exec
    ExecContext
    Query
    QueryContext
    QueryRow
    QueryRowContext
    Prepare
    PrepareContext
    Begin
    BeginTx

The exact regex may vary, but it MUST detect method calls despite
formatting/whitespace.

For example, this must be caught:

    db.ExecContext(ctx,
        "UPDATE belief SET ...")

and:

    db.QueryRowContext(ctx,
        "UPDATE belief SET ... RETURNING id")

and:

    tx.ExecContext(ctx, ...)

and:

    db.ExecContext(ctx, sqlBackdoor, id)

Do not depend on the SQL string appearing on the same physical line as
the method call.

The gate should fail closed if either target directory is missing.

------------------------------------------------------------
3B. Stage 2 — SQL write-text tripwire
------------------------------------------------------------

Keep a supplementary SQL-text check.

It must recognize actual write statements including:

    INSERT INTO ...
    UPDATE ... SET ...
    DELETE FROM ...
    CREATE TABLE ...
    DROP TABLE ...

Do NOT use the flawed pattern:

    (INSERT|UPDATE|DELETE|CREATE|DROP)
    [[:space:]]+
    (INTO|TABLE|FROM)

because it misses:

    UPDATE ... SET

The SQL-text tripwire is supplementary.

It does NOT need to be a full SQL parser.

Its purpose is to catch obvious write SQL accidentally introduced into
MCP/view source.

Avoid making ordinary prose/comments the sole source of false positives
where practical, but do not weaken the gate so much that real write SQL
passes.

------------------------------------------------------------
3C. Required I-7 adversarial tests
------------------------------------------------------------

You MUST actually test the gate using temporary scratch copies.

Do NOT leave test injections in the repository.

Use a scratch directory such as:

    /tmp/opencode/i7_test/

Test ALL of these:

TEST 1 — clean repository

    Expected: PASS

TEST 2 — direct ExecContext

    db.ExecContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1")

    Expected: FAIL

TEST 3 — transaction ExecContext

    tx.ExecContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1")

    Expected: FAIL

TEST 4 — gofmt-wrapped ExecContext

    db.ExecContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1")

    with the SQL on a different physical line from the call if necessary.

    Expected: FAIL

TEST 5 — QueryRowContext write

    db.QueryRowContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1 RETURNING id")

    Expected: FAIL

TEST 6 — const-hoisted SQL

    const sqlBackdoor =
        `UPDATE belief SET status='promoted' WHERE id=$1`

    db.ExecContext(ctx, sqlBackdoor, id)

    Expected: FAIL

TEST 7 — missing cmd/solvent-mcp

    Expected: FAIL

TEST 8 — missing internal/view

    Expected: FAIL

IMPORTANT:

Test 5 MUST fail.

Test 6 MUST fail.

These are specifically included because the previous gate missed them.

After all scratch tests:

    verify the actual repository is unchanged.

Then run the gate against the real source:

    Expected: PASS

Do not report "I-7 PASS" until both positive and negative tests have
actually been executed.

============================================================
4. F-2 — STRICT belief_id VALIDATION
============================================================

File:

    cmd/solvent-mcp/tools.go

Affected handlers:

    handleSolventPromote
    handleSolventRetireDebt

The existing pattern:

    beliefID, _ := args["belief_id"].(string)

is unsafe because malformed values become:

    ""

Implement strict validation:

    beliefID, ok := args["belief_id"].(string)
    if !ok || beliefID == "" {
        return errorResult(
            fmt.Errorf("belief_id is required and must be a string"),
        ), nil
    }

Then perform the existing scenario ownership lookup.

After the snapshot lookup, assert actual identity.

Use the equivalent of:

    if err != nil ||
       len(snap.Beliefs) != 1 ||
       snap.Beliefs[0].ID != beliefID {
        return errorResult(
            fmt.Errorf(
                "belief %s not found in scenario %s",
                beliefID,
                scenario,
            ),
        ), nil
    }

IMPORTANT:

This is ONLY an identity/scope check.

Do NOT inspect:

    debt
    promotion status
    intent state

to decide whether the operation is allowed.

The existing kernel/database remains the authority.

Do NOT add scenario handling to the frozen kernel.

------------------------------------------------------------
4A. Required F-2 tests
------------------------------------------------------------

Verify:

1. valid same-scenario UUID
       → mutation works

2. cross-scenario UUID
       → clean error
       → zero mutation

3. missing belief_id
       → clean argument error

4. numeric belief_id
       → clean argument error

5. boolean belief_id
       → clean argument error

6. object/array belief_id
       → clean argument error

7. empty string
       → clean argument error

For the cross-scenario case, verify the target belief's state/debt is
unchanged after the rejected request.

============================================================
5. F-3 — STRICT belief_id VALIDATION IN solvent_ledger
============================================================

File:

    cmd/solvent-mcp/tools.go

Affected handler:

    handleSolventLedger

The existing code silently converts a present but invalid belief_id into:

    ""

which widens the query to the entire scenario.

Distinguish:

    field absent

from:

    field present but wrong type

Implement the equivalent of:

    beliefIDRaw, hasBelief := args["belief_id"]

    beliefID, ok := beliefIDRaw.(string)

    if hasBelief && (!ok || beliefID == "") {
        return errorResult(
            fmt.Errorf(
                "belief_id must be a non-empty string when provided",
            ),
        ), nil
    }

Behavior MUST be:

    {"scenario":"track1"}
        → normal scenario-wide ledger

    {"scenario":"track1","belief_id":"valid UUID"}
        → one-belief ledger

    {"scenario":"track1","belief_id":12345}
        → argument error

    {"scenario":"track1","belief_id":true}
        → argument error

    {"scenario":"track1","belief_id":{}}
        → argument error

    {"scenario":"track1","belief_id":[]}
        → argument error

    {"scenario":"track1","belief_id":""}
        → argument error

Do not alter the underlying view semantics.

============================================================
6. F-4 — EXACT TEST COUNT
============================================================

File:

    plans/mcp/plan5_imp.md

Replace:

    107+ tests passing

with:

    107 PASS, 0 FAIL, 0 SKIP

Do not invent a different number.

The exact test result MUST subsequently be verified with:

    unset FABLE_TEST_DSN && task test

============================================================
7. NO OTHER CHANGES
============================================================

Do NOT:

- redesign MCP;
- add tools;
- add MCP resources;
- add MCP prompts;
- add authentication;
- add HTTP/SSE;
- add client integrations;
- modify .mcp.json;
- modify main.go;
- modify database schema;
- modify kernel code;
- modify CLI code;
- modify FullDebt;
- modify RetractCascade;
- populate belief_edge;
- implement multi-belief cascade;
- add raw SQL tools;
- add force/override mechanisms;
- add MCP-side business rules.

Do not implement the optional F-5 cleanup unless absolutely necessary
for the requested fixes.

============================================================
8. VERIFICATION ORDER
============================================================

After implementation execute:

1. Inspect diff.

2. Verify frozen core:

    git diff --stat HEAD -- \
      db/ \
      internal/kernel/ \
      internal/belief/ \
      internal/intent/ \
      internal/derive/ \
      internal/normalize/ \
      internal/pipeline/ \
      cmd/solvent/ \
      cmd/operator-review/

Expected:

    no output

3. Verify Taskfile:

    task --list

Expected:

    clean parse
    11 tasks

4. Run:

    go build ./...

5. Run:

    go vet ./...

6. Run:

    gofmt -l cmd internal

Expected:

    no output

7. Run I-7 adversarial scratch tests.

Expected:

    clean → PASS
    tests 2–6 → FAIL
    tests 7–8 → FAIL

8. Run I-7 against actual clean repository.

Expected:

    PASS

9. Run:

    unset FABLE_TEST_DSN && task test

Expected:

    107 PASS
    0 FAIL
    0 SKIP

10. Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Expected:

    all pass
    belief_edge = 0

11. Run:

    task mcp:build
    task mcp:seed

12. Verify MCP tool count remains exactly six.

13. Verify:

    M1 = 23514 / promoted_is_debt_free
    M2 = 23503 / gate

14. Verify Track 2 lifecycle remains functional.

15. Test malformed belief_id values through MCP.

16. Test cross-scenario mutation through MCP.

17. Verify:

    git status --short

No stray generated binary.
No accidental files.

============================================================
9. RECEIPT HONESTY
============================================================

Do not reuse old transcripts.

Do not copy expected output as actual output.

For every verification claim, provide the actual command and result.

If a test cannot be run:

    say NOT RUN

Do not fabricate a PASS.

In particular, provide actual receipts for:

    I-7 clean PASS
    I-7 malicious FAIL cases
    exact 107-test result
    cross-scenario protection
    malformed belief_id handling

============================================================
10. FINAL REPORT
============================================================

Return:

# Changes Made

Exact files changed and what changed.

# I-7 Verification

Table:

    Case | Expected | Actual | PASS/FAIL

Include all 8 cases.

# Input Validation

Show:

    malformed ledger belief_id
    malformed promote belief_id
    malformed retire_debt belief_id
    cross-scenario attempts

# Regression

Include exact results for:

    task --list
    go build ./...
    go vet ./...
    gofmt
    task test
    demo:track1
    demo:replay
    demo:track2
    inspect
    mcp:build
    mcp:seed

# Frozen Core

Explicitly confirm no frozen-core changes.

# Remaining Issues

Only genuine remaining issues.

# Final Verdict

Choose exactly one:

    READY FOR FRESH-MACHINE / REAL-AGENT ACCEPTANCE

or:

    NOT READY

Do not recommend another planning cycle unless an actual architectural
conflict is discovered.

============================================================
FINAL PRINCIPLE
============================================================

The architecture is already approved.

This is the last mechanical hardening pass.

The critical invariant is:

    Agent
      ↓
    MCP adapter
      ↓
    existing Solvent semantics
      ↓
    CockroachDB

The MCP adapter must not become a second authority.

The I-7 gate must make that boundary mechanically visible.

The agent-facing input boundary must reject malformed identifiers rather
than silently widening queries.

After these fixes and their adversarial verification, stop the
implementation/review loop and proceed to the final fresh-machine test
with a real LLM coding agent.