# SOLVENT — FINAL ADVERSARIAL CODE REVIEW OF MCP IMPLEMENTATION

You are performing an independent, adversarial CODE REVIEW of the actual
Solvent codebase.


The MCP implementation has now been completed.

The implementation notes are attached/provided as:

    imp.md

IMPORTANT:

    imp.md is UNTRUSTED IMPLEMENTATION CLAIMS.

Do not accept its claims merely because they are documented there.

Your job is to inspect the ACTUAL repository, execute the relevant commands,
inspect the actual code, and determine whether the implementation really
satisfies the approved MCP specification.

The approved design/specification is:

    plans/mcp/plan3.md
    plans/mcp/plan4.1.md
    plans/mcp/plan4.1_eval.md

Read those documents as well.

============================================================
1. REVIEW POSTURE
============================================================

Be adversarial.

Do NOT rubber-stamp the implementation.

Do NOT assume:

    "the implementation notes say it works"

means it works.

Do NOT treat successful compilation as proof of correctness.

Do NOT merely inspect diffs.

You must combine:

    source inspection
    static analysis
    repository inspection
    actual execution
    database-state verification
    MCP protocol/client verification

The key question is:

> Does the shipped MCP layer actually provide the approved Solvent
> agent-native transactional-gate demonstration without weakening,
> bypassing, or duplicating the frozen Solvent semantics?

Look specifically for defects that could embarrass us during a live
hackathon demonstration.

============================================================
2. IMPORTANT HISTORY
============================================================

The project has already gone through extensive adversarial review.

The core Solvent playground is frozen.

Previous reviews caught:

- stale receipts;
- hard-coded displayed values;
- replay/idempotence assumptions;
- test-environment drift;
- Docker version mismatch;
- demo-state sequencing errors;
- stale report transcripts;
- Track 2 header rendering;
- incorrectly labeled intent counts;
- source_observed_at wiring limitations.

Therefore:

    distrust documentation;
    distrust transcripts;
    distrust "verified" claims;
    verify the current implementation.

The most important lesson from the previous phase is:

> A internally consistent transcript is NOT evidence that the current
> code produced it.

Run the current code.

============================================================
3. FROZEN-CORE REGRESSION
============================================================

First establish whether the frozen core remains untouched.

Inspect:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

Run an appropriate git diff against HEAD/current baseline.

Verify that MCP implementation has NOT modified:

    schema
    kernel semantics
    FullDebt
    RetractCascade
    belief_edge
    source_observed_at persistence
    existing CLI semantics

If any frozen-core change exists:

    report it prominently.

Do not excuse it merely because tests pass.

============================================================
4. BASELINE REGRESSION
============================================================

Actually run:

    unset FABLE_TEST_DSN && task test

Verify the expected:

    107 tests pass

Then run the existing demos:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Verify they still behave as before.

Do not rely on imp.md's statement that they pass.

If the test count differs from 107, explain exactly why.

============================================================
5. BUILD / STATIC QUALITY
============================================================

Run:

    go build ./...
    go vet ./...
    gofmt -l cmd internal

Verify:

    no build failures
    no vet failures
    no formatting violations

Inspect:

    go.mod
    go.sum

Verify the MCP SDK dependency is the intended dependency and that the
kernel did not acquire an MCP dependency.

============================================================
6. MCP SERVER STARTUP
============================================================

Inspect:

    cmd/solvent-mcp/main.go

Verify:

1. FABLE_DSN is obtained correctly.
2. SOLVENT_FIXTURE_ROOT is obtained correctly.
3. fixture directories are validated.
4. DB is opened using the existing testdb mechanism.
5. pgx stdlib driver is registered.
6. DB ping occurs.
7. MCP server is created correctly.
8. exactly six tools are registered.
9. stdio transport is used.
10. there is no hidden HTTP/SSE server.
11. startup fails clearly when prerequisites are missing.

Actually run:

    task mcp:build

Then launch the compiled binary in an appropriate controlled environment.

Do not use the implementation notes as proof.

============================================================
7. .MCP.JSON
============================================================

Inspect:

    .mcp.json

Verify:

- it launches the CURRENT compiled binary;
- it does not use stale `go run` behavior;
- no machine-specific absolute paths;
- no secrets;
- FABLE_DSN is supplied;
- SOLVENT_FIXTURE_ROOT is supplied;
- the configuration actually corresponds to the current executable.

Pay particular attention to the interaction between:

    bin/solvent-mcp

and:

    executable-relative fixture fallback.

Actually test the configuration if practical.

============================================================
8. TOOL COUNT AND SURFACE
============================================================

Verify there are exactly six domain tools:

    solvent_ledger
    solvent_ingest_evidence
    solvent_retire_debt
    solvent_promote
    solvent_authorize_action
    solvent_falsify

Search for unauthorized additional tools.

Especially look for accidental:

    create_belief
    enter_belief
    raw_sql
    force
    override
    cancel_intent

No additional escape hatch should exist.

============================================================
9. F-1 — SCENARIO / AUDIT CORRECTION
============================================================

Verify the final schemas of:

    solvent_promote
    solvent_retire_debt

Both must require:

    scenario
    belief_id

and retire_debt additionally requires:

    debt_item

Verify that the scenario is used to call:

    pipeline.AuditIntent(...)

but is NOT used to reproduce Solvent business rules.

Do not accept a hidden query such as:

    belief → scenario inference

if the specification requires explicit scenario.

============================================================
10. AUDIT PROVENANCE
============================================================

This is critical.

For EVERY state-changing tool:

    solvent_ingest_evidence
    solvent_retire_debt
    solvent_promote
    solvent_authorize_action
    solvent_falsify

verify that:

    audit.live_on_nonpromoted

comes from:

    pipeline.AuditIntent

and nowhere else.

Search the MCP code for:

    SELECT
    COUNT
    action_intent
    live
    promoted
    nonpromoted

to determine whether the MCP layer has accidentally implemented a second
audit calculation.

The MCP layer must NOT become a second source of truth.

============================================================
11. MCP BUSINESS-LOGIC BYPASS REVIEW
============================================================

This is one of the most important sections.

Read every handler in:

    cmd/solvent-mcp/tools.go

Look specifically for semantic logic such as:

    if belief.Status != "promoted"
    if len(debt) > 0
    if intent.State != "live"
    if belief.Status == ...
    if debt contains ...

The MCP layer may perform:

    input validation
    type validation
    scenario resolution
    formatting
    error mapping

It must NOT reproduce Solvent authorization semantics.

The intended pattern is:

    unmarshal
        ↓
    kernel/pipeline call
        ↓
    format

If the MCP server can decide that an operation is forbidden BEFORE the
kernel/database sees the operation, report it.

============================================================
12. M1 — REAL DATABASE REFUSAL
============================================================

This must be independently demonstrated.

Use:

    task mcp:seed

Then connect through a REAL MCP client.

Read:

    solvent_ledger(track2)

Obtain the actual baseline UUID from the tool result.

Then request:

    solvent_promote

with:

    scenario = track2
    belief_id = actual UUID

Expected:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

CRITICAL:

Determine whether this refusal actually came from CockroachDB.

Do not accept:

    MCP pre-validation
    hard-coded error
    manually generated refusal
    tool-description-driven refusal

as equivalent.

Trace the call path:

    MCP
      ↓
    kernel
      ↓
    DB
      ↓
    PgError
      ↓
    MCP error mapping

This is the centerpiece of the hackathon demonstration.

============================================================
13. M2 — REAL DATABASE REFUSAL
============================================================

Using the same baseline before promotion, request:

    solvent_authorize_action

with:

    scenario = track2
    belief_id = actual UUID
    action = "deploy etcd v3.5.0"

Expected:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Again verify the actual origin.

The MCP layer must NOT have decided:

    "belief is not promoted, therefore refuse"

It must call:

    kernel.Store.IntentOnPromoted

and receive the database refusal.

============================================================
14. ERROR MAPPING REVIEW
============================================================

Inspect:

    cmd/solvent-mcp/errors.go

Verify:

    errors.As(..., *pgconn.PgError)

is used.

Verify the wrapped PgError survives the kernel.

Verify:

    SQLSTATE
    constraint
    sentinel

are preserved.

Verify database refusals are returned as:

    MCP tool result
    IsError = true

rather than protocol-level failure.

Try both:

    23514
    23503

in the real demo.

============================================================
15. F-3 — PGX DRIVER
============================================================

Verify:

    _ "github.com/jackc/pgx/v5/stdlib"

exists at the MCP executable boundary.

Verify it is actually necessary and correctly positioned.

Do not merely accept the implementation note.

============================================================
16. READ-ONLY VIEW
============================================================

Inspect:

    internal/view/view.go

Verify it contains only read operations.

Search specifically for:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

Do not rely only on the Taskfile grep.

Read the code.

Verify the view has no business-rule decisions.

Verify debt parsing is correct for actual PostgreSQL output.

Verify:

    scenario filtering
    belief_id filtering
    include_evidence

actually work.

============================================================
17. I-7 GATE ADVERSARIAL REVIEW
============================================================

Inspect the actual Taskfile gate.

The implementation claims:

    --include='*.go'
    comment exclusion
    explicit if/else

Verify all three.

Then test the gate under failure conditions.

Temporarily or conceptually test:

    missing cmd/solvent-mcp
    missing internal/view
    a deliberately inserted raw-write pattern

without committing those changes.

Determine whether the gate can:

    falsely PASS
    falsely FAIL
    silently ignore grep errors

The gate must fail closed.

Also determine whether the comment-filtering logic itself can accidentally
hide a real write.

Do not accept "the grep currently passes" as sufficient.

============================================================
18. FIXTURE SECURITY / DETERMINISM
============================================================

Inspect:

    solvent_ingest_evidence

Verify the MCP client can only select:

    track1
    track2

Verify it cannot provide:

    arbitrary filesystem path
    arbitrary fixture directory
    arbitrary filename
    arbitrary URL

Verify server-side mapping is authoritative.

Try malformed inputs.

Expected behavior should be a clean tool error, not:

    filesystem traversal
    arbitrary file access
    arbitrary ingestion
    panic

============================================================
19. SEED TASK
============================================================

Inspect:

    task mcp:seed

Verify:

    dotenv:
      - scripts/demo/config.env

is actually supported by the Taskfile version in use.

Run:

    task mcp:seed

from an environment where the expected environment is NOT already
accidentally inherited, if practical.

Verify:

    DB reset
    schema present
    Track 2 baseline entered
    exactly 1 belief
    exactly 1 evidence
    exactly 6 debts
    0 intents
    audit 0

Verify:

    no promotion
    no intent

Also inspect whether the command is actually passing:

    $SOLVENT_DSN

to operator-review.

============================================================
20. SEED → INGEST — CRITICAL SEMANTIC TEST
============================================================

This was previously identified as a potential danger.

Actually perform:

    seed
      ↓
    promote/authorize through MCP
      ↓
    ingest Track 2 through MCP

Then inspect the resulting state.

Verify:

    baseline UUID remains unchanged
    baseline is not duplicated
    baseline remains promoted
    baseline debt is empty

Verify the expected additional beliefs.

Verify:

    3 beliefs
    3 evidence
    3 distinct content_sha256
    1 live intent
    audit 0
    belief_edge 0

Do NOT infer correctness from the CLI Track 2 demo.

This must be tested against the MCP sequence itself.

============================================================
21. FULL FALSIFICATION PAYOFF
============================================================

After the live intent exists:

1. Call solvent_ledger.
2. Obtain the CURRENT baseline UUID.
3. Call solvent_falsify using that UUID.
4. Read ledger again.

Verify:

    baseline → retracted
    baseline intent → cancelled
    other beliefs remain entered
    audit = 0
    belief_edge = 0

Verify the intent cancellation happened because of the existing kernel
RetractCascade semantics, not MCP code.

This is the second centerpiece of the demo.

============================================================
22. TRANSACTIONALITY
============================================================

Inspect the actual kernel call path for:

    Promote
    IntentOnPromoted
    RetractCascade

Verify MCP has not introduced multiple independent writes where one
transaction was expected.

Especially verify that:

    falsify
        →
    belief retraction
        +
    dependent intent cancellation

remains one transactional domain operation.

The MCP layer must not emulate this as:

    UPDATE belief
    then
    UPDATE intent

============================================================
23. GROUNDING / AGENT BEHAVIOR
============================================================

Inspect the shipped tool descriptions.

Verify they instruct the agent to:

    read ledger before asserting state;
    reread after mutation;
    obtain UUID immediately before falsification;
    not pre-check database-enforced gates.

Then perform a real MCP-client rehearsal.

Look for:

    invented counts
    invented UUIDs
    stale UUIDs
    invented SQLSTATE
    invented constraint names
    status assertions not supported by the latest relevant tool result

The tool descriptions must not accidentally encourage the agent to
simulate Solvent instead of invoking it.

============================================================
24. TOOL SCHEMA ADVERSARIAL REVIEW
============================================================

Inspect the actual JSON schemas presented to the MCP client.

Verify:

    required fields
    enum restrictions
    UUID handling
    optional fields
    boolean handling
    malformed input behavior

Do not assume Go struct types guarantee MCP schema correctness.

Ask:

    Can an agent accidentally omit scenario?
    Can it pass arbitrary scenario text?
    Can it pass arbitrary paths?
    Can it omit belief_id?
    Can it manufacture an action that bypasses the intended tool semantics?

============================================================
25. MCP PROTOCOL / CLIENT COMPATIBILITY
============================================================

Use at least one real MCP client.

Verify:

    initialization
    tool discovery
    schema discovery
    tool invocation
    tool errors
    structured result parsing

The implementation notes mention:

    VS Code
    Codex

Do not accept those as verified unless actually tested.

At minimum verify ONE real client.

Report exactly which client was tested.

============================================================
26. CURRENT RECEIPTS ONLY
============================================================

The attached `imp.md` claims:

    build clean
    I-7 pass
    frozen core unchanged

and gives a verification table.

Treat ALL of those as claims until independently reproduced.

Do not copy them into your verdict.

If your fresh run disagrees with imp.md:

    report the discrepancy explicitly.

Do not "reconcile" the evidence by assuming the document is newer.

============================================================
27. README / DOCUMENTATION AUDIT
============================================================

Inspect the README and implementation notes for overclaims.

Verify it does NOT claim:

    multi-hop belief graph propagation
    belief_edge cascade
    bitemporal source_observed_at persistence
    production authentication
    remote MCP deployment

Verify it accurately states:

    single-belief retraction
    dependent live-intent cancellation
    local stdio
    no authentication
    frozen kernel

Also verify all commands documented actually work.

============================================================
28. SECURITY / ROBUSTNESS REVIEW
============================================================

Look for:

- arbitrary file access;
- path traversal;
- arbitrary fixture ingestion;
- SQL injection;
- raw SQL exposure;
- panic on malformed MCP arguments;
- nil handling;
- database connection leaks;
- rows not closed;
- transaction misuse;
- context cancellation ignored;
- secrets accidentally exposed;
- unbounded input;
- unsafe error exposure.

Do not inflate theoretical risks.

Only report issues that materially apply to this local hackathon implementation.

============================================================
29. PERFORMANCE / COMPLEXITY REVIEW
============================================================

Do not demand production-scale optimization.

But inspect for obvious mistakes such as:

    N+1 queries in ledger snapshot
    repeated full scans
    unnecessary database connections
    leaked rows
    duplicate pipeline execution
    state cached in MCP

The MCP server must not introduce a second persistent state store.

============================================================
30. HACKATHON DEMO REVIEW
============================================================

Evaluate the implementation from a skeptical judge's perspective.

Can a judge actually observe:

    agent asks to promote
        ↓
    DB says NO

and:

    agent asks to authorize
        ↓
    DB says NO

and later:

    promotion
        ↓
    live intent
        ↓
    postmortem
        ↓
    retraction
        ↓
    cancellation

without hidden manual intervention?

Identify any point where the demo can fail because of:

    setup
    environment
    MCP client
    stale state
    seed ordering
    UUID handling
    tool schema
    error formatting
    fixture path
    database state

Be especially alert for "works in the developer's session" behavior.

============================================================
31. DO NOT MANUFACTURE BLOCKERS
============================================================

Be adversarial, but be fair.

Do NOT downgrade the implementation because:

    "I would architect it differently"

if the current architecture satisfies the approved design.

Do NOT invent production requirements such as:

    distributed auth
    remote hosting
    TLS
    multi-user tenancy

unless they affect the actual hackathon acceptance criteria.

Conversely, do NOT waive a real bug because:

    "it's only a demo."

The standard is:

    Does it faithfully implement the approved design and survive
    a real judge-facing demonstration?

============================================================
32. REQUIRED FINDING FORMAT
============================================================

For every finding use:

    ID:
    Severity:
        BLOCKING / HIGH / MEDIUM / LOW / NIT

    Title:

    Evidence:
        exact file:line references

    Reproduction:
        exact command or interaction

    Why it matters:

    Required fix:

Do not merely say:

    "could be improved."

A finding must be actionable and evidenced.

============================================================
33. REQUIRED FINAL REPORT
============================================================

Structure your final review as:

# 1. Verdict

Choose exactly one:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

Do not manufacture a blocker.

# 2. Executive Summary

Maximum 10 bullets.

# 3. Findings

Ordered by severity and consequence.

# 4. Frozen-Core Verification

Actual evidence.

# 5. MCP Boundary Verification

Actual evidence.

# 6. M1 Verification

Actual live receipt.

# 7. M2 Verification

Actual live receipt.

# 8. Track 2 End-to-End Verification

Actual state before/after:

    seed
    refusal
    promotion
    authorization
    ingest
    falsification

# 9. Test / Build Results

Actual results.

# 10. Receipt Integrity

Explicitly compare:

    imp.md claims
    current repository
    your fresh execution

State whether any receipts were stale or misleading.

# 11. Hackathon Risk

Rank the remaining risks:

    Critical
    High
    Medium
    Low

# 12. Exact Fix List

Only the changes actually required.

# 13. Scope Decision

Explicitly state:

    implement fixes
    or
    rework

Do not recommend another planning cycle unless a genuine architectural
problem has been discovered.

============================================================
34. MOST IMPORTANT QUESTION
============================================================

At the end answer this directly:

> If I put the current repository on a fresh machine with CockroachDB,
> build the MCP server, connect one real MCP client, and follow the Track 2
> demo, will the system genuinely demonstrate that an AI agent cannot
> override Solvent's transactional safety gates?

If YES:

    explain why with receipts.

If NO:

    identify exactly where it breaks.

============================================================
35. FINAL REVIEW PRINCIPLE
============================================================

The implementation notes say:

    "The agent is not trusted. MCP is not trusted. The tool description is
     not trusted. The database is the final authority."

Your review must determine whether that statement is TRUE in the actual
implementation.

Do not review the intention.

Review the executable reality.

Run it.
Break it.
Inspect it.
Then decide.