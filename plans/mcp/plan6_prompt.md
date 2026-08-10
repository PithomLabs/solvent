# SOLVENT — FINAL MCP HARDENING PASS

You are implementing the remaining minor corrections identified by the
independent adversarial review:

    plans/mcp/plan5_review.md

Repository:

    https://github.com/PithomLabs/solvent

The MCP architecture has already been independently validated.

Claude/Fable's review verdict is:

    APPROVED WITH MINOR CORRECTIONS

The reviewer independently verified:

- Taskfile parses;
- all 11 tasks resolve;
- exactly 107 tests pass;
- frozen core is untouched;
- MCP exposes exactly six tools;
- M1 genuinely reaches CockroachDB and returns 23514;
- M2 genuinely reaches CockroachDB and returns 23503;
- cross-scenario mutation is blocked;
- audit errors no longer fabricate audit=0;
- Track 2 works end-to-end through MCP;
- the database performs the retraction → intent cancellation transaction.

DO NOT redesign anything.

This is the final hardening pass before the fresh-machine / real-agent
acceptance test.

============================================================
SCOPE
============================================================

Implement ONLY:

    F-1  I-7 gate hardening
    F-2  strict belief_id validation in promote/retire_debt
    F-3  strict belief_id validation in ledger
    F-4  exact test count in plan5_imp.md

Do NOT implement F-5 unless it is completely trivial and does not expand
scope.

Do NOT modify the frozen Solvent core.

Absolutely do not modify:

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

    FullDebt
    RetractCascade
    belief_edge
    source_observed_at

Do not add:

    new MCP tools
    new database tables
    new schema behavior
    new authorization logic
    raw SQL writes
    MCP-side replicas of Solvent business rules

============================================================
F-1 — HARDEN THE I-7 MCP BOUNDARY GATE
============================================================

File:

    Taskfile.yml

Current gate:

    grep -rnE --include='*.go' \
      '(ExecContext|Exec)\(.*(INSERT|UPDATE|DELETE|CREATE|DROP)' \
      cmd/solvent-mcp internal/view

Claude demonstrated that this misses:

1. gofmt-wrapped calls:

       db.ExecContext(ctx,
           "UPDATE ...")

2. query-path writes:

       db.QueryRowContext(ctx, "UPDATE ... RETURNING id")

3. const-hoisted SQL:

       const sqlBackdoor = `UPDATE ...`
       db.ExecContext(ctx, sqlBackdoor, id)

The third case is particularly important because it mirrors the coding
style already used by the kernel.

Fix the gate so it actually protects the MCP/view boundary.

Preferred approach:

Because cmd/solvent-mcp and internal/view are NOT supposed to perform
direct database writes at all, use a conservative gate that rejects
direct write/transaction entry points in those directories.

For example, a gate may reject:

    Exec
    ExecContext
    Prepare
    PrepareContext
    Begin
    BeginTx

in:

    cmd/solvent-mcp
    internal/view

and separately detect write SQL text if useful.

Do not blindly copy the example if you can produce a cleaner implementation.

The important invariant is:

    MCP/view may read.
    MCP/view may call the Solvent kernel.
    MCP/view may NOT directly write or open write transactions.

The gate must:

- PASS on current clean source;
- FAIL on direct Exec/ExecContext writes;
- FAIL on transaction-opening write paths;
- FAIL on the tested QueryRowContext write;
- FAIL on const-hoisted write SQL;
- FAIL if either required directory is missing;
- not silently pass on grep errors;
- continue to allow legitimate SELECT/read code.

============================================================
MANDATORY I-7 ADVERSARIAL TESTS
============================================================

Do not just run the clean gate.

Create temporary scratch copies or temporary test fixtures so the
repository itself remains unchanged.

Test all of these:

TEST 1 — current clean repository

    Expected: PASS

TEST 2 — one-line ExecContext write

    db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")

    Expected: FAIL

TEST 3 — transaction write entry point

    tx.ExecContext(ctx, "UPDATE ...")

    Expected: FAIL

TEST 4 — gofmt-wrapped call

    db.ExecContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1")

    Expected: FAIL

TEST 5 — QueryRowContext write

    db.QueryRowContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1 RETURNING id")

    Expected: FAIL

TEST 6 — const-hoisted SQL

    const sqlBackdoor = `UPDATE belief SET status='promoted' WHERE id=$1`

    db.ExecContext(ctx, sqlBackdoor, id)

    Expected: FAIL

TEST 7 — missing cmd/solvent-mcp

    Expected: FAIL

TEST 8 — missing internal/view

    Expected: FAIL

After all scratch tests, verify the actual repository is byte-for-byte
unchanged except for the intended Taskfile modification.

Run the gate on the real repository again:

    Expected: PASS

============================================================
F-2 — STRICT belief_id VALIDATION
============================================================

File:

    cmd/solvent-mcp/tools.go

Affected handlers:

    solvent_promote
    solvent_retire_debt

Current problem:

    beliefID, _ := args["belief_id"].(string)

This silently turns:

    missing belief_id
    numeric belief_id
    boolean belief_id
    object belief_id

into:

    ""

The ownership guard then uses an empty ID and can fall through to an
unfiltered snapshot.

CockroachDB eventually rejects the mutation, but the MCP adapter should
reject malformed input explicitly.

Implement strict validation:

    beliefID, ok := args["belief_id"].(string)

Reject when:

    !ok
    OR
    beliefID == ""

Return a clean tool-level argument error.

Then perform the existing scenario ownership lookup.

Do NOT add business-rule checks here.

The lookup should establish only:

    "Does this belief ID belong to this scenario?"

Then verify identity explicitly.

Do not merely check:

    len(snap.Beliefs) > 0

Require the requested belief itself to be present.

Conceptually:

    if !ok || beliefID == "" {
        return argument error
    }

    snapshot := GetSnapshot(... scenarioID, beliefID ...)

    if error
       OR len(snapshot.Beliefs) != 1
       OR snapshot.Beliefs[0].ID != beliefID {
        return "belief not found in scenario"
    }

Then call the existing kernel mutation.

Do NOT:

    inspect debt here
    inspect promotion status here
    inspect intent state here
    reproduce kernel authorization rules

The kernel/database remains authoritative.

Test:

1. valid same-scenario UUID → mutation works
2. cross-scenario UUID → clean error, zero mutation
3. missing belief_id → clean argument error
4. numeric belief_id → clean argument error
5. boolean belief_id → clean argument error
6. object/array belief_id → clean argument error
7. empty string → clean argument error

============================================================
F-3 — STRICT belief_id VALIDATION IN solvent_ledger
============================================================

File:

    cmd/solvent-mcp/tools.go

Current behavior:

    beliefID, _ := args["belief_id"].(string)

This means:

    solvent_ledger({"scenario":"track1","belief_id":12345})

silently becomes an unfiltered scenario query.

That is dangerous for an agent-facing interface because the agent may
believe it asked for one belief while actually receiving all beliefs.

Fix this.

Distinguish:

    field absent
    field present but wrong type

Expected behavior:

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

    {"scenario":"track1","belief_id":""}

        → argument error

Do not silently convert invalid values to "".

Do not alter the underlying view semantics.

============================================================
F-4 — EXACT TEST COUNT
============================================================

File:

    plans/mcp/plan5_imp.md

Change:

    "107+ tests passing"

to the exact verified result:

    107 tests passing

Use the actual verification wording from the adversarial review:

    107 PASS, 0 FAIL, 0 SKIP

Do not fabricate a new number.

Do not add claims about tests that were not actually run.

============================================================
NO OTHER CHANGES
============================================================

Do not:

- remove the fixture fallback merely for cosmetic reasons;
- modify main.go;
- add MCP unit-test infrastructure;
- add client integrations;
- change .mcp.json;
- change database schema;
- change kernel behavior;
- change CLI behavior;
- add authentication;
- add HTTP/SSE;
- add MCP resources/prompts;
- implement multi-belief cascade.

The purpose of this pass is to close the three small validation/gate
issues and correct the test-count wording.

============================================================
VERIFICATION
============================================================

After implementation:

1. Inspect diff.

Verify ONLY expected files changed.

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

3. Run:

    task --list

Expected:

    11 tasks

4. Run:

    unset FABLE_TEST_DSN && task test

Expected:

    107 tests
    0 failures

5. Run:

    go build ./...
    go vet ./...
    gofmt -l cmd internal

Expected:

    all clean

6. Run the I-7 gate.

Expected:

    clean source → PASS

7. Run ALL eight adversarial I-7 scratch tests above.

Expected:

    all malicious cases → FAIL
    missing directories → FAIL
    clean source → PASS

8. Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Expected:

    all pass
    belief_edge = 0

9. Run:

    task mcp:build
    task mcp:seed

10. Perform MCP validation:

    - exactly six tools
    - M1 = 23514 / promoted_is_debt_free
    - M2 = 23503 / gate
    - Track 2 lifecycle still works

11. Explicitly test malformed belief_id values through MCP.

12. Explicitly test cross-scenario mutation again.

13. Verify:

    git status --short

No generated root binary.

============================================================
FINAL REPORT
============================================================

Return:

# Changes Made

Exact files and changes.

# I-7 Gate

Show results for all eight adversarial cases.

# Input Validation

Show malformed belief_id results.

# Regression

Show:

    107 tests
    build
    vet
    formatting
    four demos
    MCP build
    MCP seed

# Frozen Core

Show that it remains untouched.

# Remaining Issues

Only genuine remaining issues.

# Verdict

State one:

    READY FOR FRESH-MACHINE ACCEPTANCE

or

    NOT READY

Do NOT call it READY if any mandatory adversarial test fails.

Do not recommend another planning/review cycle unless an actual
architectural problem is discovered.

This is the final mechanical hardening pass before attaching a real
LLM coding agent to the MCP server.