# SOLVENT — FINAL MCP PHASE A IMPLEMENTATION HANDOFF

Repository:
    https://github.com/PithomLabs/solvent

Authoritative plan:
    plan4.1.md

Independent evaluation:
    plan4.1_eval.md

You are the coding agent implementing the approved Solvent MCP Phase A.

READ BOTH DOCUMENTS COMPLETELY BEFORE EDITING:

    plan4.1.md
    plan4.1_eval.md

The independent evaluation was performed against the live repository and
identified two blocking contradictions and four small implementation/documentation
corrections.

You must implement plan4.1 WITH ALL SIX CORRECTIONS BELOW.

This is an implementation task, not another design exercise.

DO NOT reopen the MCP architecture.

DO NOT redesign the six-tool surface.

DO NOT modify the frozen Solvent core.

============================================================
1. AUTHORITATIVE DESIGN
============================================================

The approved architecture is:

    MCP-capable AI agent
             |
             v
       Solvent MCP
             |
             v
      existing Solvent
       kernel/pipeline
             |
             v
       CockroachDB

The MCP server is a THIN ADAPTER.

The database remains the final authority.

The MCP layer must NOT reproduce Solvent's business rules.

The central hackathon demonstration is:

    Agent requests promotion
        ->
    MCP forwards request
        ->
    CockroachDB refuses because debt remains

    Agent requests authorization
        ->
    MCP forwards request
        ->
    CockroachDB refuses because belief is not promoted

    Agent clears legitimate debt
        ->
    promotion succeeds
        ->
    deployment intent becomes LIVE

    Real Track 2 postmortem arrives
        ->
    agent invokes falsification
        ->
    belief becomes RETRACTED
        ->
    dependent intent becomes CANCELLED

The conceptual proof is:

    The agent can request.
    MCP can forward.
    Solvent can execute.
    CockroachDB decides.

============================================================
2. READ/VERIFY BEFORE EDITING
============================================================

Before touching code:

1. Read all of plan4.1.md.
2. Read all of plan4.1_eval.md.
3. Inspect the live repository.
4. Verify the APIs referenced by the plans.

At minimum inspect:

    go.mod
    Taskfile.yml
    README.md

    internal/kernel/
    internal/kernel/kernel.go
    internal/kernel/errors.go
    internal/pipeline/
    internal/testdb/
    cmd/operator-review/
    scripts/check_i7.sh
    scripts/demo/
    internal/derive/testdata/etcd_real/

Verify the actual signatures of:

    kernel.Store.RetireDebt
    kernel.Store.Promote
    kernel.Store.IntentOnPromoted
    kernel.Store.RetractCascade

    pipeline.Run
    pipeline.AuditIntent

    testdb.Open

Do not assume the plan's pseudocode exactly matches the SDK or repository APIs.

If a concrete SDK/API detail differs, adapt the implementation to the actual
API while preserving the approved architecture and semantics.

============================================================
3. FROZEN CORE — ABSOLUTE
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

No changes to:

    schema
    constraints
    kernel semantics
    FullDebt
    RetractCascade
    belief_edge
    source_observed_at
    existing CLI behavior

Do not refactor the CLI to share implementation with MCP.

If implementation appears to require a frozen-core change:

    STOP
    report the exact incompatibility
    DO NOT work around it by weakening the specification.

============================================================
4. APPROVED CHANGE SURFACES
============================================================

Create:

    cmd/solvent-mcp/main.go
    cmd/solvent-mcp/tools.go
    cmd/solvent-mcp/errors.go
    internal/view/view.go
    .mcp.json

Modify only:

    go.mod
    go.sum
    Taskfile.yml
    README.md

Do not introduce unrelated changes.

============================================================
5. CORRECTION F-1 — ADD SCENARIO TO PROMOTE/RETIRE_DEBT
============================================================

THIS IS A REQUIRED PLAN CORRECTION.

The original plan required every state-changing tool to return:

    audit.live_on_nonpromoted

sourced exclusively from:

    pipeline.AuditIntent(ctx, db, scenarioID)

But:

    kernel.Store.Promote(...)
    kernel.Store.RetireDebt(...)

do not accept scenarioID, and the original tool schemas did not provide it.

Therefore ADD `scenario` to both tool schemas.

Final schemas:

    solvent_promote:
    {
      "scenario": "track1|track2",
      "belief_id": "uuid"
    }

    solvent_retire_debt:
    {
      "scenario": "track1|track2",
      "belief_id": "uuid",
      "debt_item": "string"
    }

The scenario is used for the audit call.

The scenario MUST NOT be used to pre-check or implement promotion/debt
semantics.

The domain operation remains:

    Promote(beliefID)

or:

    RetireDebt(beliefID, item)

Then:

    pipeline.AuditIntent(...scenarioID...)

This preserves the clean boundary.

DO NOT infer scenario by querying the belief from MCP.

DO NOT add a separate audit query.

============================================================
6. SIX MCP TOOLS — EXACTLY SIX
============================================================

Implement exactly:

    solvent_ledger
    solvent_ingest_evidence
    solvent_retire_debt
    solvent_promote
    solvent_authorize_action
    solvent_falsify

DO NOT add:

    create_belief
    enter_belief
    force
    override
    raw_sql
    cancel_intent
    arbitrary fixture path
    arbitrary evidence URL

============================================================
7. solvent_ledger
============================================================

Arguments:

    scenario: track1|track2
    belief_id: optional UUID
    include_evidence: optional bool

Use:

    internal/view

plus:

    pipeline.AuditIntent

The tool reports current state.

It must NOT decide whether a transition is allowed.

Return:

    beliefs
    evidence when requested
    intents
    audit_live_on_nonpromoted

All state shown must come from the database.

============================================================
8. solvent_ingest_evidence
============================================================

Arguments:

    scenario: track1|track2

Map only:

    track1
        ->
    00000000-0000-0000-0000-000000000001

    track2
        ->
    00000000-0000-0000-0000-000000000002

Resolve fixture directories server-side.

Do not accept:

    arbitrary filesystem path
    filename
    URL
    fixture root

from the MCP client.

Call the existing:

    pipeline.Run(...)

Do not reimplement ingestion, normalization, derivation, deduplication,
or aggregation.

============================================================
9. solvent_retire_debt
============================================================

Arguments:

    scenario
    belief_id
    debt_item

Call:

    kernel.Store.RetireDebt(...)

Valid debt items remain exactly the six existing FullDebt items.

Do not add debt types.

Do not add bulk clearing.

Then obtain audit using the supplied scenario.

============================================================
10. solvent_promote
============================================================

Arguments:

    scenario
    belief_id

Call:

    kernel.Store.Promote(...)

DO NOT inspect debt before calling Promote.

DO NOT reproduce:

    promoted_is_debt_free

inside MCP.

The database must generate the refusal.

For Track 2 before debt clearance, the actual result must be:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

Then obtain audit using the supplied scenario.

============================================================
11. solvent_authorize_action
============================================================

Arguments:

    scenario
    belief_id
    action

Call:

    kernel.Store.IntentOnPromoted(...)

DO NOT pre-check belief status.

For a non-promoted belief, the actual result must be:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Return the database refusal as an MCP tool result with:

    IsError = true

============================================================
12. solvent_falsify
============================================================

Arguments:

    scenario
    belief_id

Call:

    kernel.Store.RetractCascade(...)

Do not implement a second cascade algorithm.

The honest semantic scope remains:

    single-belief retraction
    +
    cancellation of that belief's dependent live intent

Do NOT claim:

    multi-hop propagation
    belief_edge propagation

Keep:

    belief_edge = 0

============================================================
13. ERROR HANDLING
============================================================

Implement MCP error mapping in:

    cmd/solvent-mcp/errors.go

Use:

    errors.As(err, &pgErr)

with:

    *pgconn.PgError

The existing kernel wraps errors with %w.

Preserve:

    err.Error()
    SQLSTATE
    ConstraintName

For database errors return a tool result containing:

    error
    sentinel
    sqlstate
    constraint

and:

    IsError = true

Do NOT substring-match SQL errors.

Do NOT turn database refusals into generic MCP protocol failures.

============================================================
14. PGX DRIVER — CORRECTION F-3
============================================================

`internal/testdb.Open()` calls:

    sql.Open("pgx", dsn)

Therefore `cmd/solvent-mcp/main.go` MUST register the driver at the
command boundary.

Include:

    import _ "github.com/jackc/pgx/v5/stdlib"

Do not add the driver import to the frozen kernel.

This matches the repository's existing executable pattern.

============================================================
15. INTERNAL VIEW
============================================================

Create:

    internal/view/view.go

It is READ ONLY.

SELECT is allowed.

It must contain no:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

It must not implement domain rules.

It must only project current ledger state.

Do not calculate:

    live_on_nonpromoted

inside the view.

That value comes from:

    pipeline.AuditIntent

============================================================
16. AUDIT ENVELOPE
============================================================

Every state-changing tool must return:

    {
      "result": {...},
      "audit": {
        "live_on_nonpromoted": N
      }
    }

including database refusal results.

Audit MUST come from:

    pipeline.AuditIntent(...)

The only reason promote/retire_debt can now do this is because their tool
schemas include `scenario`.

Do not create a second audit implementation.

============================================================
17. STDIO SERVER
============================================================

Create:

    cmd/solvent-mcp/main.go

Use the actual MCP Go SDK API available in go.mod.

stdio only.

Startup sequence:

1. Read FABLE_DSN.
2. Read SOLVENT_FIXTURE_ROOT.
3. Resolve the approved fixture-root fallback if retained.
4. Verify:
       track1 fixture
       track2 fixture
5. Open DB.
6. Ping DB.
7. Register exactly six tools.
8. Run stdio transport.

No:

    HTTP
    SSE
    authentication
    sessions
    caching
    remote deployment
    MCP resources
    MCP prompts
    sampling
    streaming

============================================================
18. FIXTURE ROOT / .MCP.JSON — CORRECTION F-5
============================================================

Do NOT use:

    go run ./cmd/solvent-mcp

in the shipped `.mcp.json`.

The approved configuration must launch the built executable:

    bin/solvent-mcp

because plan4.1's executable-relative fixture fallback is meaningful only
when running the actual built executable.

`task mcp:build` must create:

    bin/solvent-mcp

The `.mcp.json` must invoke that executable.

It must provide:

    FABLE_DSN
    SOLVENT_FIXTURE_ROOT

Do not use machine-specific absolute paths.

Do not put secrets into `.mcp.json`.

If you determine that the executable-relative fallback is unnecessary,
do not silently remove it; report the discrepancy before changing that
design.

============================================================
19. TASKFILE — CORRECTION F-2
============================================================

Implement:

    task mcp:build
    task mcp:seed

`task mcp:seed` must work from the repository's clean-machine setup.

The existing Taskfile declares:

    scripts/demo/config.env

but does not source it.

Therefore ensure the MCP seed task loads the existing configuration.

Preferred solution:

    dotenv:
      - scripts/demo/config.env

Use the repository's existing configuration rather than hardcoding another
DSN.

Then:

    task mcp:seed

must be able to invoke:

    go run ./cmd/operator-review ...

with a valid DSN.

Do not modify the operator-review CLI.

Do not create another configuration source.

============================================================
20. MCP SEED
============================================================

`task mcp:seed` must:

1. Reset the fable DB.
2. Reapply the existing schema through existing task machinery.
3. Seed exactly the Track 2 historical baseline using operator-review.
4. Not promote it.
5. Not create an intent.

Use the exact Track 2 scenario UUID, claim, claim type, evidence URL,
and SHA specified in plan4.1.

Expected seed state:

    1 belief
    postulated
    entered
    6 debts
    1 evidence
    0 intents
    audit 0

Do not create an MCP belief-creation tool.

============================================================
21. I-7 GATE — CORRECTION F-4
============================================================

The MCP boundary gate must be robust.

It must:

1. Fail if `cmd/solvent-mcp` does not exist.
2. Fail if `internal/view` does not exist.
3. Search only Go source files.
4. Avoid false-positive matches from ordinary prose/comments where
   practical.
5. Fail on raw DB-write keywords.
6. Never silently PASS because grep returned exit code 2.

Use an explicit shell structure.

For example, conceptually:

    test -d cmd/solvent-mcp
    test -d internal/view

    if grep ...; then
        echo "FAIL: raw writes in MCP/view"
        exit 1
    else
        echo "I-7 MCP boundary: PASS"
    fi

Use appropriate:

    --include='*.go'

and/or word boundaries / focused matching.

Inspect the repository's existing I-7 mechanism and make the smallest
robust extension.

Do not make a gate that falsely passes when directories are missing.

============================================================
22. NO MCP BUSINESS LOGIC
============================================================

After implementation inspect:

    cmd/solvent-mcp/tools.go

There must be no MCP-level decision-making based on:

    debt
    belief status
    intent state

The MCP layer may perform:

    input validation
    scenario resolution
    error mapping
    response formatting

It must not reproduce Solvent transition rules.

============================================================
23. TRACK 2 SEED → INGEST
============================================================

This MUST be empirically verified.

The MCP flow is:

    seed baseline
        ↓
    attempt promote
        ↓
    attempt authorize
        ↓
    clear debt
        ↓
    promote
        ↓
    authorize
        ↓
    ingest Track 2
        ↓
    reread baseline UUID
        ↓
    falsify baseline

After Track 2 ingestion verify:

    3 beliefs

    baseline:
        same UUID
        postulated
        promoted
        debt empty

    accommodated:
        entered
        4 debts

    derived:
        entered
        6 debts

    3 evidence rows

    3 distinct content_sha256 values

    1 live intent on baseline

    audit = 0

    belief_edge = 0

Do not assume the result because the CLI Track 2 demo behaves correctly.

Actually verify it through MCP.

============================================================
24. GROUNDING RULE
============================================================

Use this corrected grounding rule:

> Every state, count, status, or identifier the agent asserts must be
> grounded in the most recent tool result that contains that fact.
> After a mutation, the agent should use solvent_ledger to re-read any
> state it intends to assert rather than relying on an earlier result.

Do NOT force every mutation response to duplicate the full ledger.

The important rule is:

    no invented state
    no invented count
    no stale UUID
    no invented SQLSTATE
    no invented constraint

For important post-mutation assertions, prefer:

    mutation
       ↓
    solvent_ledger
       ↓
    assertion

============================================================
25. FULL MCP DEMO
============================================================

Use one Track 2 scenario only.

Narrative:

    historical 2021 belief
        ↓
    six unresolved debts
        ↓
    agent attempts promotion
        ↓
    DB refuses: 23514
        ↓
    agent attempts authorization
        ↓
    DB refuses: 23503
        ↓
    retire six debts
        ↓
    promote
        ↓
    authorize deployment
        ↓
    live intent
        ↓
    ingest real Track 2 evidence
        ↓
    falsify original belief
        ↓
    belief retracted
        ↓
    intent cancelled
        ↓
    audit 0

Do not introduce Track 1 into the MCP narrative.

Do not create a new belief through MCP.

============================================================
26. BUILD / TEST
============================================================

Run:

    gofmt
    go build ./...
    go vet ./...
    task mcp:build

Then:

    unset FABLE_TEST_DSN && task test

The existing 107-test baseline must remain green.

The test suite must NOT depend on the MCP server or demo container.

============================================================
27. FROZEN DEMO REGRESSION
============================================================

Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

All must continue to pass.

Verify:

    belief_edge = 0

Verify frozen-core directories remain unchanged.

Use git diff to prove this.

============================================================
28. MCP CLIENT REHEARSAL
============================================================

Use at least one real MCP-capable client.

Do not merely unit-test the handlers.

The client must actually communicate with:

    bin/solvent-mcp

over stdio.

Perform the complete Track 2 narrative.

Capture the transcript from the CURRENT implementation.

Do not reuse any prior transcript.

Do not manufacture output.

The transcript must visibly demonstrate:

    23514 promoted_is_debt_free
    23503 gate
    successful promotion
    live intent
    real postmortem ingestion
    retraction
    cancellation
    audit 0

============================================================
29. README
============================================================

Update README with:

- MCP purpose;
- trust boundary;
- six tools;
- build;
- seed;
- client configuration;
- Track 2 demonstration;
- two database refusals;
- promotion;
- live intent;
- falsification;
- cancellation;
- audit result;
- honest limitations.

Do NOT claim:

    belief_edge propagation
    multi-hop cascade
    bitemporal persistence

State that source_observed_at is currently not threaded through the frozen
kernel and is deferred.

Keep CLI fallback documentation.

============================================================
30. CORRECTION F-6
============================================================

Correct stale documentation references discovered by the independent review.

In particular:

- update the FullDebt line reference to the actual current line;
- revise the conditional grep instruction so it is clearly described as
  a review aid rather than a precise mechanical gate if it remains.

Do not introduce unrelated documentation churn.

============================================================
31. FINAL RECEIPT REQUIREMENTS
============================================================

Produce an implementation report after actually executing the work.

Include:

1. Exact files changed.
2. Frozen-core diff result.
3. MCP SDK version.
4. Build result.
5. Vet result.
6. Gofmt result.
7. 107-test result.
8. Frozen demo results.
9. I-7 result.
10. mcp:seed result.
11. M1 actual refusal receipt.
12. M2 actual refusal receipt.
13. Promotion/live-intent result.
14. Track 2 ingest result.
15. UUID stability result.
16. Falsification/cancellation result.
17. Audit result.
18. belief_edge result.
19. Real MCP-client transcript.
20. README result.
21. Deviations, if any.

Every runtime claim must have an actual receipt.

============================================================
32. RECEIPT HONESTY
============================================================

Previous phases discovered stale transcripts.

Therefore:

DO NOT:

- reuse old output;
- paste expected output as actual;
- claim a command ran when it did not;
- claim a fresh environment without actually using one;
- claim an MCP client rehearsal without connecting a client.

If something cannot be run:

    say NOT RUN

and explain why.

Do not manufacture evidence.

============================================================
33. STOP CONDITIONS
============================================================

STOP and report if:

- frozen-core modification appears necessary;
- schema modification appears necessary;
- kernel modification appears necessary;
- belief_edge modification appears necessary;
- source_observed_at wiring appears necessary;
- MCP SDK API materially conflicts with the design;
- seed → ingest produces materially different semantics;
- M1 does not produce real 23514;
- M2 does not produce real 23503;
- MCP requires duplicated Solvent business logic;
- 107 tests regress;
- frozen demos regress.

Do not solve a STOP condition by weakening the acceptance criteria.

============================================================
34. SCOPE FENCE
============================================================

DO NOT build:

    belief_edge propagation
    multi-hop cascade
    create_belief MCP tool
    enter_belief MCP tool
    raw SQL MCP tool
    force
    override
    direct intent cancellation
    arbitrary fixture paths
    arbitrary evidence URLs
    HTTP
    SSE
    authentication
    remote deployment
    frontend
    RAG
    embeddings
    vector DB
    MCP resources
    MCP prompts
    MCP sampling
    streaming
    caching
    sessions
    client compatibility matrix
    source_observed_at kernel wiring
    schema changes
    kernel changes
    CLI refactor

============================================================
35. DEFINITION OF DONE
============================================================

Do not declare completion until ALL are true:

    [ ] plan4.1 implemented
    [ ] F-1 corrected: scenario added to promote/retire_debt
    [ ] F-2 corrected: mcp:seed loads config.env
    [ ] F-3 corrected: pgx driver registered
    [ ] F-4 corrected: I-7 gate cannot silently PASS
    [ ] F-5 corrected: .mcp.json launches bin/solvent-mcp
    [ ] F-6 corrected: stale references fixed
    [ ] six MCP tools work
    [ ] no raw DB writes in MCP/view
    [ ] no MCP business-rule duplication
    [ ] fixture boundary enforced
    [ ] M1 real 23514
    [ ] M2 real 23503
    [ ] debt retirement works
    [ ] promotion works
    [ ] intent becomes live
    [ ] Track 2 ingestion works
    [ ] baseline UUID remains stable
    [ ] falsification works
    [ ] intent becomes cancelled
    [ ] audit = 0
    [ ] belief_edge = 0
    [ ] 107 existing tests remain green
    [ ] frozen demos remain green
    [ ] frozen core remains unchanged
    [ ] real MCP-client rehearsal succeeds
    [ ] current transcript captured
    [ ] README updated
    [ ] no scope creep

============================================================
FINAL PRINCIPLE
============================================================

Do not optimize for the MCP server merely "working."

Optimize for the architectural proof:

    Agent
       ↓
    asks for something
       ↓
    MCP forwards it
       ↓
    Solvent executes the existing operation
       ↓
    CockroachDB enforces the invariant
       ↓
    MCP reports the actual decision

The agent is NOT the authority.

The MCP server is NOT the authority.

The database is.

Implement the approved plan now, incorporating all six corrections above.
Do not reopen the design.