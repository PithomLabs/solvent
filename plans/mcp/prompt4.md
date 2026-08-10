You are implementing the fixes from Claude/Fable's final adversarial code
review of the Solvent MCP implementation.

IMPORTANT:
This is a FIX PASS, not a redesign.

The MCP architecture has been independently validated.

Do NOT modify:

- db/
- schema
- internal/kernel/
- internal/belief/
- internal/intent/
- internal/derive/
- internal/normalize/
- internal/pipeline/
- cmd/solvent/
- cmd/operator-review/
- FullDebt semantics
- RetractCascade semantics
- belief_edge behavior
- source_observed_at behavior
- existing Solvent business rules

The frozen core must remain untouched.

The review is:

    imp_review.md

Its verdict is REWORK REQUIRED, but explicitly states that the verdict is
mechanical, not architectural. The MCP trust boundary, six-tool surface,
audit provenance, fixture boundary, error mapping, transactional behavior,
and both real database refusals are already correct.

Your job is to apply ONLY the concrete fixes below, then independently
verify the repository from a clean state.

============================================================
FIX 1 — Taskfile YAML PARSE FAILURE
============================================================

File:

    Taskfile.yml

The current file does not parse because the mcp:seed echo command contains
a YAML scalar with `: `:

    echo "Seed complete. Post-seed state: 1 belief ..."

Quote the entire command or otherwise make it valid YAML.

Do not change the semantic content of the command unnecessarily.

Then verify:

    task --list

and confirm the relevant tasks resolve:

    setup
    test
    demo:track1
    demo:replay
    demo:track2
    inspect
    mcp:build
    mcp:seed

Do NOT merely run one task.

The acceptance condition is that Taskfile parsing itself succeeds.

============================================================
FIX 2 — REPAIR THE I-7 MCP BOUNDARY GATE
============================================================

File:

    Taskfile.yml

The current gate contains:

    grep ... | grep -v '^\|// \|/\*\|\*/'

This is broken because `^` matches every line, causing grep -v to discard
everything.

This must be fixed.

Requirements:

1. The gate must fail if cmd/solvent-mcp or internal/view contains an actual
   raw database write.

2. The gate must fail closed if either target directory is missing.

3. Do not introduce a filter that accidentally removes every line.

4. Prefer matching actual Go database-write calls rather than merely
   searching for SQL keywords anywhere in comments.

5. The gate should detect patterns such as:

       db.ExecContext(...)
       db.Exec(...)
       db.QueryContext(...)
       db.Query(...)

   when they contain write SQL such as:

       INSERT
       UPDATE
       DELETE
       CREATE
       DROP

6. Do not accidentally reject legitimate SELECT/read operations.

7. Do not alter the existing permitted audit read behavior.

A reasonable implementation direction is:

    for d in cmd/solvent-mcp internal/view; do
        [ -d "$d" ] || { echo "FAIL: $d missing"; exit 1; }
    done

    if grep -rnE --include='*.go' \
      '(ExecContext|Exec)\(.*(INSERT|UPDATE|DELETE|CREATE|DROP)' \
      cmd/solvent-mcp internal/view; then
        echo "FAIL: raw writes in MCP/view"
        exit 1
    else
        echo "I-7 MCP boundary: PASS"
    fi

You may improve this if you have a demonstrably more reliable solution,
but do not over-engineer it.

CRITICAL ACCEPTANCE TEST:

Do NOT trust the gate merely because it prints PASS.

Create a temporary scratch copy of the relevant source and inject:

    db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")

Then run the gate against the scratch copy.

The gate MUST FAIL.

Also test a clean copy.

The gate MUST PASS.

Also test a missing directory.

The gate MUST FAIL.

This injection test is mandatory.

============================================================
FIX 3 — PREVENT CROSS-SCENARIO BELIEF MUTATION
============================================================

File:

    cmd/solvent-mcp/tools.go

Claude found that:

    solvent_promote
    solvent_retire_debt

accept a scenario and belief_id, but the kernel methods themselves operate
on belief_id without scenario.

Currently the MCP handlers use scenario only for the audit lookup.

This means a caller can supply:

    scenario = track2
    belief_id = track1 belief

and mutate the track1 belief while calculating the audit for track2.

Worse, retire_debt can mutate the belief and then return an error because
the post-mutation snapshot is scoped to the wrong scenario.

Fix this.

Before invoking the kernel mutation, resolve/validate that the supplied
belief belongs to the supplied scenario.

Use the existing read-only projection:

    view.GetSnapshot(...)

This is an identity/ownership lookup, NOT an authorization rule.

Do NOT inspect debt/status/state to decide whether promotion or retirement
is allowed.

The database/kernel must remain the authority for business semantics.

The intended shape is approximately:

    snapshot lookup using scenario + belief_id

    if belief does not exist in that scenario:
        return a clean tool error

    otherwise:
        call the existing kernel method

Then obtain the audit using the supplied scenario.

Do this for:

    solvent_promote
    solvent_retire_debt

Do NOT add scenario parameters to kernel methods.

Do NOT modify kernel code.

Also consider whether the same identity mismatch can affect any other
mutation tool. Do not mechanically add unnecessary checks everywhere;
inspect the existing semantics first.

Acceptance test:

1. Populate track1.
2. Obtain a real track1 belief UUID.
3. Attempt:

       solvent_promote(
           scenario="track2",
           belief_id=<track1 UUID>
       )

4. Verify:
       - no mutation occurs;
       - clean tool error;
       - track1 belief remains unchanged.

5. Repeat for:

       solvent_retire_debt(
           scenario="track2",
           belief_id=<track1 UUID>,
           debt_item="needToyCheck"
       )

6. Verify:
       - no debt is retired;
       - clean error;
       - no mutation;
       - no misleading audit envelope.

============================================================
FIX 4 — DO NOT FABRICATE AUDIT ZERO ON REFUSAL
============================================================

File:

    cmd/solvent-mcp/tools.go

The refusal/error path currently does something equivalent to:

    audit, _ := pipeline.AuditIntent(context.Background(), db, scenarioID)

This is wrong.

Requirements:

1. Pass the request context through.

2. Do not discard the audit error.

3. If the audit succeeds:
       return the normal audit envelope.

4. If the audit itself fails:
       do NOT return:
           live_on_nonpromoted: 0

   Instead return an explicit representation such as:

       "audit": null

   or:

       "audit_error": ...

   while preserving the original refusal.

5. Do not convert an audit failure into a fake SAFE state.

Keep this change narrowly scoped to the MCP adapter.

============================================================
FIX 5 — README MCP DOCUMENTATION
============================================================

File:

    README.md

The review found that README.md contains no MCP documentation.

Add the MCP section required by the approved plan.

Do not turn this into a marketing essay.

The README should make it possible for a fresh judge to discover and run
the MCP demonstration.

Include:

1. What the MCP layer demonstrates.

2. The trust boundary:

       Agent
          ↓
       MCP adapter
          ↓
       Solvent kernel
          ↓
       CockroachDB
          ↓
       database constraints = final authority

3. Prerequisites.

4. Build:

       task mcp:build

5. Seed:

       task mcp:seed

6. Client configuration using .mcp.json.

7. State that the MCP server uses stdio.

8. The six tools.

9. The two important refusal demonstrations:

       promotion blocked by promoted_is_debt_free / 23514

       authorization blocked by gate / 23503

10. Brief Track 2 demonstration sequence:

       seed
       ledger
       promotion refusal
       authorization refusal
       retire debts
       promote
       authorize
       ingest
       falsify
       observe cancellation

11. Honest limitation:

       retraction is currently single-belief;
       belief_edge is not populated;
       no multi-hop cascade.

12. State clearly that the MCP layer is a thin adapter and does not contain
    Solvent business logic.

Do NOT claim VS Code, Codex, Claude Code, or another client was tested
unless you actually perform that test during this implementation pass.

============================================================
FIX 6 — .gitignore / STRAY BINARY
============================================================

Files:

    .gitignore
    repository root

There is a stray root-level:

    solvent-mcp

binary.

Remove it from the working tree.

Add appropriate ignores:

    /bin/
    /solvent-mcp

Do not commit generated binaries.

The intended binary is:

    bin/solvent-mcp

built by:

    task mcp:build

Verify:

    git status --short

does not show the generated root binary.

============================================================
FIX 7 — CORRECT imp.md RECORD
============================================================

File:

    plans/mcp/imp.md

Correct only documentation inaccuracies discovered by the review.

Specifically:

1. Correct the F-5 explanation.

The current claim that the executable-relative fixture fallback works
correctly when the binary is at bin/solvent-mcp is false.

The current fallback resolves to:

    bin/internal/derive/testdata/etcd_real

which does not exist.

Choose one clean solution:

    A. Fix the fallback to resolve relative to the repository root

or:

    B. Remove the fallback and explicitly require
       SOLVENT_FIXTURE_ROOT.

Prefer the simplest robust solution consistent with the approved plan.

If you fix the fallback, test it.

2. Correct the inaccurate AddTool description if still present.

3. Remove claims that VS Code/Codex were tested unless they are actually
   tested.

4. Do not fabricate new verification receipts.

5. Do not rewrite the report to hide findings.

============================================================
OPTIONAL FIXES
============================================================

Do NOT spend time on optional polish before the required fixes pass.

R-7 strict type validation is optional unless it can be fixed cleanly
without expanding scope.

R-9 items are optional.

If you do implement R-7, be precise:

    absent optional field
        ≠
    present field with wrong type

For example:

    belief_id = 12345

must NOT silently become:

    belief_id = ""

because that widens the query.

But do not turn this fix pass into a generalized validation framework.

============================================================
STRICT SCOPE FENCE
============================================================

Do NOT:

- redesign the MCP architecture;
- add authentication;
- add HTTP/SSE;
- add MCP resources;
- add MCP prompts;
- add sampling;
- add remote deployment;
- modify the schema;
- modify kernel semantics;
- populate belief_edge;
- implement multi-belief cascade;
- modify FullDebt;
- modify RetractCascade;
- modify existing CLI semantics;
- modify the frozen demo logic;
- add an override/force/admin path;
- create a second source of truth.

The approved architecture is already validated.

This is a mechanical correction pass.

============================================================
VERIFICATION ORDER
============================================================

After implementation, execute in this order.

1. Check git diff.

Confirm frozen core remains untouched:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

2. Validate YAML:

    task --list

3. Run baseline:

    unset FABLE_TEST_DSN && task test

Expected:

    107 PASS

4. Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

5. Run:

    task mcp:build

6. Run:

    task mcp:seed

7. Verify MCP startup.

8. Verify tools/list returns exactly six tools.

9. Verify M1 through MCP.

10. Verify M2 through MCP.

11. Verify Track 2 end-to-end through MCP.

12. Verify cross-scenario mismatch cannot mutate anything.

13. Verify audit failure cannot become fake audit=0.

14. Run:

    go build ./...
    go vet ./...
    gofmt -l cmd internal

15. Run the I-7 gate.

16. Perform the scratch injection test against the I-7 gate.

17. Verify .gitignore and git status.

18. If practical, perform ONE actual MCP client rehearsal.

If you cannot perform a real LLM client rehearsal, say so explicitly.
Do not claim one occurred.

============================================================
FINAL REPORT
============================================================

Return a concise implementation report with:

# Changes Made

List exact files and changes.

# Frozen Core

Show evidence that the frozen core remains untouched.

# Verification

Give actual command + result.

# I-7 Gate

Show:

    clean source → PASS
    injected raw write → FAIL
    missing directory → FAIL

# MCP Verification

Show:

    tools/list = 6
    M1 = 23514 / promoted_is_debt_free
    M2 = 23503 / gate

# Cross-Scenario Verification

Show that mismatched scenario/belief IDs cannot mutate state.

# Track 2

Show the actual MCP lifecycle result.

# Remaining Issues

Only genuine remaining issues.

# Scope Confirmation

Explicitly confirm:

    no schema changes
    no kernel changes
    no belief_edge changes
    no FullDebt changes
    no RetractCascade changes

============================================================
FINAL DECISION RULE
============================================================

Do NOT declare success merely because:

    go build ./...

passes.

The implementation is complete only when:

    task parses
    task test passes
    demos pass
    mcp:build works
    mcp:seed works
    I-7 gate genuinely fails on injected writes
    M1 genuinely comes from CockroachDB
    M2 genuinely comes from CockroachDB
    cross-scenario mutation is prevented
    README documents the path
    generated binaries are ignored
    frozen core remains untouched

If any of those fail, fix them before declaring completion.

Do not open another planning/review cycle unless you discover a genuine
architectural conflict with the approved MCP design.