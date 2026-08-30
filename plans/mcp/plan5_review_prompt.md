# SOLVENT — ADVERSARIAL CODE REVIEW OF PLAN 5 FIX PASS

You are performing the final adversarial code review of the ACTUAL
Solvent repository after implementation of plan5.

The implementation summary is:

    plans/mcp/plan5_imp.md

Treat that document as UNTRUSTED CLAIMS.

Do not review the summary as if it were evidence.

Inspect the actual repository, source code, git diff, Taskfile, MCP
implementation, and execute the relevant commands yourself.

The purpose of this review is to determine whether the seven fixes from
plan5 were actually implemented correctly and whether they introduced
any regression.

This is a CODE REVIEW, not a planning review.

Do not redesign the MCP architecture.

Do not reopen already-settled architecture unless you find an actual
implementation contradiction.

============================================================
1. ORIGINAL FINDINGS BEING FIXED
============================================================

The previous adversarial review identified:

    R-1  Taskfile YAML parse failure
    R-2  I-7 gate effectively dead
    R-3  cross-scenario mutation
    R-4  fabricated audit=0 on audit failure
    R-5  missing README MCP documentation
    R-6  stray generated binary
    R-7  inaccurate implementation record

The plan5 implementation claims all seven are fixed.

Verify each independently.

============================================================
2. IMPORTANT REVIEW PRINCIPLE
============================================================

Do not trust:

    plan5_imp.md
    its verification table
    "PASS"
    "CLEAN"
    "107+ tests"

unless independently reproduced.

Previous phases discovered stale receipts.

Therefore:

    implementation claims are hypotheses;
    repository state is evidence;
    execution results are stronger evidence.

If a claim cannot be reproduced, report the discrepancy.

============================================================
3. GIT / FROZEN CORE
============================================================

First inspect:

    git status
    git diff
    git diff --stat

Verify these remain untouched:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

Also verify:

    FullDebt
    RetractCascade
    belief_edge
    source_observed_at

were not changed.

If any frozen-core file changed:

    report it prominently.

Do not waive it merely because tests pass.

============================================================
4. FIX 1 — TASKFILE YAML
============================================================

Inspect Taskfile.yml.

Run:

    task --list

Verify the file parses successfully.

Verify the MCP tasks actually exist:

    mcp:build
    mcp:seed

Do not merely accept "11 tasks" as proof.

Run the relevant tasks far enough to verify that their recipes themselves
are executable.

Pay particular attention to the modified echo command.

============================================================
5. FIX 2 — I-7 GATE
============================================================

This is a HIGH-PRIORITY REVIEW.

The previous I-7 gate was broken because:

    grep -v '^...'

discarded all lines.

The implementation claims the gate is now fixed.

Do NOT accept:

    "I-7 gate: PASS"

as sufficient evidence.

Inspect the actual Taskfile implementation.

Verify:

1. cmd/solvent-mcp must exist.
2. internal/view must exist.
3. Clean source must PASS.
4. A real raw write must FAIL.
5. A missing directory must FAIL.
6. grep errors cannot silently produce PASS.
7. The pattern actually searches Go source.
8. The pattern cannot be trivially bypassed by the implementation's
   actual write forms.

Perform the adversarial injection test yourself.

Create a temporary copy or temporary modification containing something
equivalent to:

    db.ExecContext(ctx,
        "UPDATE belief SET status='promoted' WHERE id=$1")

Run the SAME gate logic against it.

Expected:

    FAIL

Then restore the repository exactly.

Also test:

    missing cmd/solvent-mcp
    missing internal/view

Expected:

    FAIL

Then verify clean repository:

    PASS

This finding is NOT discharged until all three cases are demonstrated.

============================================================
6. I-7 — CHECK FOR FALSE SECURITY
============================================================

Do not stop at the exact injection string from the implementation notes.

Inspect the actual MCP code for all database access patterns.

Search for:

    Exec(
    ExecContext(
    Query(
    QueryContext(
    Prepare(
    Begin(
    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

Determine whether the I-7 gate would actually catch a raw write that
the MCP implementation could plausibly introduce.

Also verify that legitimate reads remain allowed.

If the gate is intentionally only a lightweight mechanical guard rather
than a complete static analyzer, that's acceptable — but it must actually
catch the class of write it claims to prohibit.

============================================================
7. FIX 3 — CROSS-SCENARIO MUTATION
============================================================

Inspect:

    cmd/solvent-mcp/tools.go

Specifically:

    handleSolventPromote
    handleSolventRetireDebt

Verify they perform a scenario/belief ownership lookup BEFORE mutation.

Expected conceptual behavior:

    scenario + belief_id
        ↓
    read-only lookup
        ↓
    belief belongs to scenario?
        ↓
    YES → kernel mutation
    NO  → clean error, NO mutation

The lookup must NOT inspect:

    debt
    status
    intent state

to decide authorization.

It is only identity/ownership validation.

Test this LIVE.

Create/populate a track1 belief.

Then attempt:

    solvent_promote(
        scenario="track2",
        belief_id=<track1 UUID>
    )

Verify:

    clean error
    no mutation

Then test:

    solvent_retire_debt(
        scenario="track2",
        belief_id=<track1 UUID>,
        debt_item=<valid item>
    )

Verify:

    clean error
    no mutation
    track1 debt unchanged

Also verify the valid same-scenario mutation still works.

============================================================
8. CHECK OTHER MUTATING TOOLS
============================================================

Inspect:

    solvent_authorize_action
    solvent_falsify

Verify they remain correctly scenario-scoped through their existing
kernel APIs.

Do not demand unnecessary duplicate checks if the kernel operation
already takes scenarioID and enforces the correct scope.

============================================================
9. FIX 4 — AUDIT ERROR PROPAGATION
============================================================

Inspect:

    envelopeErrorResult

Verify:

- it receives the request context;
- it calls AuditIntent with that context;
- it does NOT use context.Background();
- it does NOT discard audit errors;
- it does NOT fabricate audit=0.

If audit succeeds:

    audit.live_on_nonpromoted

must be present.

If audit itself fails:

    audit must not falsely say 0.

An explicit null/error representation is acceptable.

Test or construct a realistic audit failure if practical.

At minimum verify the code path.

============================================================
10. CHECK ALL envelopeErrorResult CALLERS
============================================================

The implementation summary says only:

    handleSolventPromote
    handleSolventAuthorizeAction

were updated.

Verify every caller in the actual repository.

Search for:

    envelopeErrorResult(

Ensure:

    every caller passes ctx.

No stale caller should silently use the old signature or a background
context.

============================================================
11. FIX 5 — README
============================================================

Inspect README.md.

Verify there is now an MCP section.

It must accurately document:

    trust boundary
    prerequisites
    task mcp:build
    task mcp:seed
    .mcp.json
    six tools
    M1 / 23514
    M2 / 23503
    Track 2 sequence
    single-belief retraction limitation
    belief_edge limitation
    MCP as thin adapter

Check every documented command against the actual repository.

Do not accept documentation that merely describes an intended workflow.

Do NOT allow claims that:

    VS Code
    Codex
    Claude Code

were tested unless the current repository review actually tests them.

============================================================
12. FIX 6 — GITIGNORE
============================================================

Verify:

    /bin/
    /solvent-mcp

are present in .gitignore.

Verify the stray root binary is gone.

Run:

    git status --short

Confirm no generated binary is tracked or left untracked.

Build:

    task mcp:build

Then verify:

    bin/solvent-mcp

is ignored as intended.

============================================================
13. FIX 7 — imp.md RECORD
============================================================

Inspect:

    plans/mcp/imp.md

Verify:

- stale VS Code/Codex claim removed;
- AddTool signature corrected;
- fixture fallback explanation corrected;
- no fabricated receipts introduced.

Important:

The implementation summary says the fallback resolves to a nonexistent
bin/internal/... path and normal operation requires SOLVENT_FIXTURE_ROOT.

Verify this against the actual code.

Do not modify main.go merely to clean up documentation unless the code
itself is incorrect.

============================================================
14. BUILD
============================================================

Run:

    go build ./...
    go vet ./...
    gofmt -l cmd internal

Do not accept "PASS" in the report without actual output.

============================================================
15. TEST COUNT
============================================================

The implementation summary says:

    "107+ tests passing"

This is insufficiently precise.

Run:

    unset FABLE_TEST_DSN && task test

Record the EXACT result.

Determine:

    exact number of tests
    packages
    failures, if any

Verify the existing test suite remains self-contained.

It must not silently depend on the MCP container or MCP server.

============================================================
16. FROZEN DEMOS
============================================================

Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Verify all pass.

Verify:

    belief_edge = 0

Verify no MCP changes altered existing CLI/demo semantics.

============================================================
17. MCP BUILD / SEED
============================================================

Run:

    task mcp:build

Then:

    task mcp:seed

Verify the actual resulting state.

Do not accept the expected state without checking the DB.

Expected:

    1 Track 2 baseline belief
    1 evidence
    6 debts
    0 intents

and the baseline is:

    entered
    postulated

============================================================
18. MCP PROTOCOL
============================================================

Use an actual MCP client if available.

Verify:

    initialization
    tools/list
    exactly six tools
    tool schemas
    tool invocation
    IsError handling

Do not merely inspect Go registration.

============================================================
19. M1 — DATABASE REFUSAL
============================================================

Actually perform:

    solvent_promote(
        scenario="track2",
        belief_id=<actual baseline UUID>
    )

Expected:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

Verify the error originates from:

    CockroachDB
      ↓
    PgError
      ↓
    kernel
      ↓
    MCP error mapping

and is not MCP pre-validation.

============================================================
20. M2 — DATABASE REFUSAL
============================================================

Actually perform:

    solvent_authorize_action(
        scenario="track2",
        belief_id=<actual baseline UUID>,
        action="deploy etcd v3.5.0"
    )

Expected:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Again verify it is a genuine database refusal.

============================================================
21. FULL TRACK 2 LIFECYCLE
============================================================

Actually run:

    seed
      ↓
    promotion refusal
      ↓
    authorization refusal
      ↓
    retire all debts
      ↓
    promotion
      ↓
    authorization
      ↓
    live intent
      ↓
    ingest
      ↓
    ledger
      ↓
    falsify
      ↓
    ledger

Verify:

    baseline UUID stable
    baseline promoted before falsification
    intent live
    evidence count correct
    falsification works
    belief retracted
    intent cancelled
    audit correct
    belief_edge = 0

Do not use stale transcripts.

============================================================
22. RECEIPT INTEGRITY
============================================================

Compare:

    plan5_imp.md

against your actual execution.

For every claimed verification:

    task --list
    build
    vet
    gofmt
    I-7
    frozen core
    tests

determine whether you actually reproduced it.

If the implementation summary says:

    "I-7 PASS"

but your adversarial injection test shows:

    PASS

on a raw write:

then the implementation summary is false and this is a finding.

If it says:

    "107+"

but exact execution gives:

    109

that is not necessarily a code defect, but the report is imprecise.

Distinguish:

    code defect
    documentation defect
    stale receipt
    harmless wording

============================================================
23. HACKATHON JUDGE PERSPECTIVE
============================================================

Evaluate the implementation as if you are a skeptical judge with a fresh
clone.

Can they:

    task setup
    task mcp:build
    task mcp:seed
    connect an MCP client
    invoke six tools
    see the real 23514 refusal
    see the real 23503 refusal
    promote after debt clearance
    authorize an intent
    ingest Track 2
    falsify the belief
    observe cancellation

without developer-only state?

Look for:

    setup fragility
    stale paths
    missing environment variables
    task failures
    misleading README
    stale transcripts
    generated artifacts
    client configuration failures

============================================================
24. DO NOT MANUFACTURE BLOCKERS
============================================================

Be adversarial but evidence-driven.

Do not recommend architectural rework because you personally prefer
another design.

Do not demand production infrastructure outside the hackathon scope.

But do not waive an actual judge-facing failure because:

    "it's just a demo."

============================================================
25. REQUIRED FINDING FORMAT
============================================================

For each finding:

    ID:
    Severity:
        BLOCKING / HIGH / MEDIUM / LOW / NIT

    Title:

    Evidence:
        exact file:line

    Reproduction:
        exact command or MCP interaction

    Impact:

    Required fix:

Order findings by consequence.

============================================================
26. FINAL VERDICT
============================================================

Choose exactly one:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

Do not manufacture a blocker.

"REWORK REQUIRED" should mean actual implementation changes are necessary,
not merely documentation polish.

============================================================
27. FINAL REPORT
============================================================

Return:

# Verdict

# Executive Summary

# Findings

# Frozen Core Verification

# I-7 Adversarial Verification

Include:

    clean → PASS
    injected write → FAIL
    missing directory → FAIL

# MCP Verification

Include:

    six tools
    M1
    M2
    Track 2 lifecycle

# Test / Build Results

Include exact test count.

# Receipt Integrity

Explicitly state whether plan5_imp.md's claims match current reality.

# Hackathon Judge Risk

# Required Fixes

# Final Scope Assessment

Answer explicitly:

> Is the Solvent MCP implementation now safe to hand to the final
> fresh-machine acceptance test?

============================================================
FINAL PRINCIPLE
============================================================

The previous review established that the architecture works.

This review is about whether the FIX PASS actually closed the holes.

Do not review intentions.

Do not review the summary.

Review the current executable reality.

Especially attack:

    Taskfile
    I-7 gate
    cross-scenario isolation
    audit error handling
    fresh setup
    exact test count
    real MCP protocol
    real database refusals

Run it.
Break it.
Verify it.
Then decide.