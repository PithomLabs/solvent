Yes. **Plan 4.1 incorporates the correction I requested and is now ready for implementation.**

The shell gate is fixed correctly with an explicit `if/else`, eliminating the `FAIL`/`PASS` chaining bug. 

More importantly, I don't see a new substantive design problem in this revision:

* **Frozen-core boundary:** preserved. 
* **MCP remains a thin adapter:** no semantic debt/status/intent decisions. 
* **Error preservation:** correctly uses `errors.As` to recover the underlying `PgError`. 
* **Fixture boundary:** still server-owned and scenario-enumerated. 
* **M1/M2 are real database-gate demonstrations**, not MCP pre-validation. 
* **Seed → ingest is explicitly verified**, including UUID stability and the expected three-belief state. 
* **Grounding rule is the corrected version** we agreed on. 
* **Fresh MCP-client rehearsal and current transcripts are mandatory**, which is important given the stale-receipt problem from the previous phase. 

### One thing to watch during implementation

I would **not block implementation**, but tell the coding agent to verify the exact MCP SDK API rather than blindly copying the pseudocode:

```go
mcp.NewServer(...)
mcp.AddTool(...)
mcp.StdioTransport{}
```

Plan 4.1 explicitly says to stop if the SDK API materially differs. 

That's exactly the right behavior. Don't let the agent "adapt" the architecture just because the SDK's concrete API differs slightly.

### Verdict

**APPROVED — HAND TO CODING AGENT.**

No more Claude planning review. No more plan iteration.

The next stage should be:

**Plan 4.1 → coding agent implementation → actual execution receipts → Claude/Fable adversarial review of the implementation.**

And importantly, the coding agent should **not modify `plan4.md` just because plan4.1 supersedes its bad shell snippet**. Plan 4.1 is itself the implementation plan; the source-plan correction is already documented. The implementation should focus on the actual MCP surfaces and leave the frozen Solvent core untouched.



Use this as the **implementation handoff prompt**. It tells the coding agent to treat `plan4.1.md` as authoritative, inspect the repo first, implement incrementally, and produce real receipts rather than assuming the plan is correct.

```text
# SOLVENT — IMPLEMENT PLAN 4.1: MCP PHASE A

You are the coding agent for the Solvent repository.

Repository:
    https://github.com/PithomLabs/solvent

Authoritative implementation plan:
    plans/mcp/plan4.1.md

Read the ENTIRE `plan4.1.md` before making any changes.

This is an implementation task, NOT a design task.

The MCP architecture has already been designed and adversarially reviewed.
Do not reopen the architecture, invent alternatives, or perform another
planning exercise unless a STOP CONDITION below is encountered.

============================================================
1. MISSION
============================================================

Implement `plan4.1.md` completely.

The goal is to add a thin stdio MCP adapter over the existing, frozen
Solvent core.

The final architecture must be:

    MCP-capable agent
          │
          ▼
    Solvent MCP server
          │
          ▼
    existing Solvent kernel / pipeline
          │
          ▼
    CockroachDB

The MCP server is NOT a new implementation of Solvent.

The database and existing kernel remain the authority.

The MCP layer must expose Solvent's existing semantics to an AI agent,
not create new semantics.

============================================================
2. BEFORE TOUCHING CODE — INSPECT
============================================================

First inspect:

    plans/mcp/plan4.1.md
    go.mod
    Taskfile.yml
    README.md

Then inspect all APIs referenced by the plan, especially:

    internal/kernel/
    internal/kernel/errors.go
    internal/pipeline/
    internal/testdb/
    cmd/operator-review/
    scripts/check_i7.sh
    scripts/demo/
    internal/derive/testdata/etcd_real/

Verify the exact signatures of:

    kernel.Store.Promote
    kernel.Store.IntentOnPromoted
    kernel.Store.RetireDebt
    kernel.Store.RetractCascade
    pipeline.Run
    pipeline.AuditIntent
    testdb.Open

Also inspect how:

    FABLE_DSN
    SOLVENT_DSN
    SOLVENT_FIXTURE_ROOT

are currently used.

Do not assume the plan's pseudocode is the exact API.

If the MCP SDK API differs syntactically from the plan, adapt to the SDK
while preserving the specified architecture and behavior.

If the existing Solvent APIs cannot support the plan without modifying
the frozen core, STOP and report.

============================================================
3. HARD FROZEN-CORE RULE
============================================================

These areas are frozen:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

Do NOT modify them.

Do NOT modify:

    schema
    constraints
    kernel semantics
    belief semantics
    intent semantics
    RetractCascade
    FullDebt
    belief_edge
    source_observed_at persistence

Do NOT refactor the CLI.

Do NOT modify the existing Track 1 / replay / Track 2 demo semantics.

If MCP cannot be implemented without changing one of these areas:

    STOP
    report the exact incompatibility
    do not work around it by weakening the specification

============================================================
4. ALLOWED NEW/CHANGED SURFACES
============================================================

Create:

    cmd/solvent-mcp/main.go
    cmd/solvent-mcp/tools.go
    cmd/solvent-mcp/errors.go
    internal/view/view.go
    .mcp.json

Modify:

    go.mod
    go.sum
    Taskfile.yml
    README.md

Do not modify other files unless an unavoidable tooling/dependency issue
requires it.

If an unexpected file must change, report:

    file
    reason
    exact change

before proceeding if it affects the frozen boundary.

============================================================
5. ADD MCP SDK
============================================================

Add:

    github.com/modelcontextprotocol/go-sdk

Use the current compatible API available to this repository.

After adding the dependency:

    go build ./...
    go vet ./...

must pass before proceeding.

Do not add unrelated dependencies.

The kernel must not gain an MCP dependency.

============================================================
6. IMPLEMENT ERROR MAPPING FIRST
============================================================

Create:

    cmd/solvent-mcp/errors.go

This is the only MCP-layer file that should directly reference:

    github.com/jackc/pgx/v5/pgconn

Use:

    errors.As(err, &pgErr)

to recover:

    *pgconn.PgError

The existing kernel wrapping uses `%w`, so preserve the wrapped
PostgreSQL error.

For PgError return a tool error payload containing:

    error: true
    sentinel: err.Error()
    sqlstate: pgErr.SQLState()
    constraint: pgErr.ConstraintName

For ordinary errors return:

    error: true
    message: err.Error()

Database refusals must become MCP TOOL RESULTS with:

    IsError = true

They must NOT become generic protocol failures.

Do not substring-match SQL errors.

Do not discard:

    SQLSTATE
    constraint name

============================================================
7. IMPLEMENT READ-ONLY VIEW
============================================================

Create:

    internal/view/view.go

This is a read-only projection layer for:

    solvent_ledger

Define the structures specified by plan4.1:

    Snapshot
    Belief
    Evidence
    Intent
    SnapshotOpts

Implement:

    Snapshot(ctx, db, scenarioID, opts)

It may SELECT from:

    belief
    evidence
    action_intent

It must contain ZERO:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

The view must not decide whether an operation is permitted.

It only reports current state.

Do not calculate:

    live_on_nonpromoted

inside the view.

That value must come from:

    pipeline.AuditIntent(...)

============================================================
8. IMPLEMENT SIX TOOLS — EXACTLY SIX
============================================================

Implement exactly these six domain tools:

    solvent_ledger
    solvent_ingest_evidence
    solvent_retire_debt
    solvent_promote
    solvent_authorize_action
    solvent_falsify

Do not add:

    create_belief
    enter_belief
    force
    override
    raw_sql
    cancel_intent
    arbitrary evidence
    arbitrary fixture access

============================================================
9. TOOL 1 — solvent_ledger
============================================================

Arguments:

    scenario: track1 | track2
    belief_id: optional UUID
    include_evidence: optional bool

Use:

    view.Snapshot(...)
    pipeline.AuditIntent(...)

Return:

    beliefs
    optional evidence
    intents
    audit_live_on_nonpromoted

This is read-only.

The ledger is the authoritative source for:

    current state
    counts
    statuses
    identifiers
    debt

Do not manufacture values from fixtures.

============================================================
10. TOOL 2 — solvent_ingest_evidence
============================================================

Arguments:

    scenario: track1 | track2

The scenario must resolve entirely server-side.

Map:

    track1 → scenario UUID 00000000-0000-0000-0000-000000000001
    track2 → scenario UUID 00000000-0000-0000-0000-000000000002

Resolve:

    <fixtureRoot>/track1
    <fixtureRoot>/track2

Never accept from the MCP client:

    filesystem path
    directory
    filename
    arbitrary URL
    fixture root

Call:

    pipeline.Run(...)

directly.

Do not reimplement the pipeline.

Do not substitute another pipeline function.

Return:

    pipeline results
    audit

============================================================
11. TOOL 3 — solvent_retire_debt
============================================================

Arguments:

    belief_id
    debt_item

Call:

    kernel.Store.RetireDebt(...)

Valid debt items are exactly:

    needMap
    needInvariant
    needToyCheck
    needNullModel
    needObstruction
    needFaithfulnessReview

Do not invent additional debt types.

Do not add bulk debt clearing.

Return the resulting debt state plus audit.

============================================================
12. TOOL 4 — solvent_promote
============================================================

Arguments:

    belief_id

Call:

    kernel.Store.Promote(...)

CRITICAL:

Do NOT inspect debt first.

Do NOT write:

    if len(debt) > 0 ...

Do NOT refuse the request inside MCP.

The MCP server must call the actual kernel/database operation.

For the Track 2 baseline with open debt, the expected result is:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

This is one of the central hackathon demonstrations.

The database says NO.

============================================================
13. TOOL 5 — solvent_authorize_action
============================================================

Arguments:

    scenario
    belief_id
    action

Call:

    kernel.Store.IntentOnPromoted(...)

CRITICAL:

Do NOT inspect belief status first.

Do NOT write:

    if status != promoted ...

The database must decide.

For a non-promoted belief the expected result is:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Again:

    MCP forwards request
    database refuses

============================================================
14. TOOL 6 — solvent_falsify
============================================================

Arguments:

    scenario
    belief_id

Call:

    kernel.Store.RetractCascade(...)

Do not implement a second retraction algorithm.

The current honest semantic claim is:

    single-belief retraction
    +
    cancellation of that belief's dependent live intent

Do NOT describe this as:

    belief graph propagation
    multi-hop cascade

Keep:

    belief_edge = 0

============================================================
15. ZERO MCP BUSINESS LOGIC
============================================================

Audit `cmd/solvent-mcp/tools.go` carefully.

There must be no semantic conditionals such as:

    if belief.Status != "promoted"
    if len(debt) > 0
    if intent.State != "live"

The only expected conditionals should be ordinary programming/error
handling, argument validation, scenario resolution, etc.

The MCP server should behave like:

    decode
      ↓
    invoke domain operation
      ↓
    format

not:

    decode
      ↓
    reproduce Solvent rules
      ↓
    maybe invoke domain operation

============================================================
16. AUDIT MUST COME FROM EXISTING PIPELINE
============================================================

For every state-changing tool:

    solvent_ingest_evidence
    solvent_retire_debt
    solvent_promote
    solvent_authorize_action
    solvent_falsify

return:

    {
      "result": {...},
      "audit": {
        "live_on_nonpromoted": N
      }
    }

Get N exclusively from:

    pipeline.AuditIntent(...)

Do not create a separate audit query.

Do not independently calculate it in MCP.

Do not infer it from the tool result.

============================================================
17. STDIO SERVER
============================================================

Create:

    cmd/solvent-mcp/main.go

Use the MCP SDK's actual stdio transport API.

At startup:

1. Read FABLE_DSN.
2. Read SOLVENT_FIXTURE_ROOT.
3. Resolve executable-relative fallback if required by plan.
4. Verify:
       <fixtureRoot>/track1
       <fixtureRoot>/track2
5. Fail immediately if either is missing.
6. Open DB using existing:
       testdb.Open(...)
7. Ping the DB.
8. Fail clearly if unavailable.
9. Register exactly six tools.
10. Run the stdio MCP transport.

No HTTP.

No SSE.

No authentication.

No remote server.

No session state.

No caching.

No MCP resources.

No MCP prompts.

No sampling.

============================================================
18. .MCP.JSON
============================================================

Create project-local:

    .mcp.json

It must launch:

    ./cmd/solvent-mcp

and provide:

    FABLE_DSN
    SOLVENT_FIXTURE_ROOT

Do not put secrets in the file.

Do not use machine-specific absolute paths.

Use the repository-relative configuration specified in plan4.1.

Before finalizing, test that the configuration actually launches the
current implementation.

============================================================
19. TASKFILE
============================================================

Add:

    task mcp:build

and:

    task mcp:seed

mcp:build:

    go build -o bin/solvent-mcp ./cmd/solvent-mcp

mcp:seed must:

1. reset the DB;
2. use the existing operator-review CLI;
3. create the Track 2 historical baseline;
4. not promote it;
5. not create an intent.

Use the exact scenario UUID, claim, claim type, evidence URL, and SHA
from plan4.1.

Expected seed state:

    1 belief
    postulated
    entered
    6 debts
    1 evidence
    0 intents
    audit 0

Do not implement MCP belief creation just to simplify seeding.

============================================================
20. I-7 MCP BOUNDARY GATE
============================================================

Extend the existing Taskfile test gate using the corrected shell form:

    if grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' \
      cmd/solvent-mcp internal/view; then
        echo "FAIL: raw writes in MCP/view"
        exit 1
    else
        echo "I-7 MCP boundary: PASS"
    fi

Do NOT use the broken:

    grep ... && echo FAIL && exit 1 || echo PASS

form.

The check must fail if any raw write is found.

The implementation must have zero raw writes in:

    cmd/solvent-mcp
    internal/view

SELECT is allowed.

============================================================
21. CORE TEST ISOLATION
============================================================

Preserve the existing test independence.

This command must remain valid:

    unset FABLE_TEST_DSN && task test

The 107-test baseline must NOT require:

    task setup
    MCP server
    demo container
    MCP client

Do not silently repoint tests to the MCP/demo database.

============================================================
22. BUILD AND STATIC CHECKS
============================================================

Run:

    gofmt
    go build ./...
    go vet ./...
    task mcp:build

Then:

    grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' \
      cmd/solvent-mcp internal/view

must return no matches.

Also inspect:

    cmd/solvent-mcp/tools.go

for semantic conditionals.

============================================================
23. SEED AND M1
============================================================

Start from a clean DB.

Run:

    task mcp:seed

Then use a real MCP client.

First:

    solvent_ledger(track2)

Obtain the actual baseline UUID from this result.

Then call:

    solvent_promote(baseline UUID)

Do not hard-code the UUID in the MCP interaction.

Expected actual result:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

Capture the actual output.

============================================================
24. M2
============================================================

Without changing the baseline promotion state, call:

    solvent_authorize_action(
        track2,
        baseline UUID,
        "deploy etcd v3.5.0"
    )

Expected:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Capture the actual output.

These two refusals must come from CockroachDB.

============================================================
25. FULL PAYOFF
============================================================

Use MCP to:

1. Retire all six actual debt items.
2. Promote the baseline.
3. Read ledger and verify promotion.
4. Authorize:
       deploy etcd v3.5.0
5. Read ledger and verify live intent.
6. Ingest Track 2 evidence.
7. Verify exact post-ingest state.
8. Read ledger again.
9. Obtain the current baseline UUID from the ledger.
10. Falsify that UUID.
11. Read ledger again.

Expected post-ingest:

    3 beliefs
    baseline:
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
    audit 0
    belief_edge 0

Expected post-falsification:

    baseline = retracted
    baseline intent = cancelled
    other two beliefs = entered
    retracted = 1
    audit = 0
    belief_edge = 0

============================================================
26. CRITICAL SEED → INGEST VERIFICATION
============================================================

Do not merely run the sequence and say it worked.

Verify:

    baseline UUID before ingest
        ==
    baseline UUID after ingest

Verify that ingestion does not create a duplicate baseline.

Verify:

    baseline remains the original belief
    postulated
    promoted
    debt empty

Verify the two additional expected beliefs.

Verify:

    exactly 3 evidence rows
    exactly 3 distinct hashes
    exactly 1 live intent

If actual behavior differs from plan4.1:

    STOP
    report exact state
    do not modify kernel/schema to force conformity

============================================================
27. GROUNDING DISCIPLINE
============================================================

The MCP interaction must never invent:

    UUIDs
    counts
    statuses
    SQLSTATEs
    constraint names

The agent must ground assertions in tool results.

Use the corrected rule:

> Every state, count, status, or identifier the agent asserts must be
> grounded in the most recent tool result that contains that fact.
> After a mutation, the agent should use solvent_ledger to re-read any
> state it intends to assert rather than relying on an earlier result.

Do not make every mutation response artificially duplicate the full ledger.

For important post-mutation claims, prefer:

    mutation
      ↓
    solvent_ledger
      ↓
    agent explanation

============================================================
28. REAL MCP CLIENT REHEARSAL
============================================================

A compile-only test is NOT sufficient.

Use at least one actual MCP-capable client.

The exact client can be whichever supported client is practically
available, but it must communicate with the current implementation over
stdio.

Perform the full Track 2 narrative.

The agent should experience:

    historical belief
        ↓
    open debt
        ↓
    promote request
        ↓
    DATABASE REFUSES
        ↓
    authorize request
        ↓
    DATABASE REFUSES
        ↓
    clear debts
        ↓
    promote
        ↓
    authorize
        ↓
    LIVE intent
        ↓
    ingest real postmortem
        ↓
    falsify
        ↓
    RETRACTED belief
        ↓
    CANCELLED intent

Capture the transcript from the CURRENT implementation.

Do not reuse old transcripts.

Do not manufacture transcript output.

============================================================
29. FROZEN CORE REGRESSION
============================================================

After MCP implementation, run:

    unset FABLE_TEST_DSN && task test

Require:

    107 tests pass

Then run the frozen demos:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

They must continue to pass.

Check:

    belief_edge = 0

Check that:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

remain unchanged.

Use git diff to prove this.

============================================================
30. README
============================================================

Add the approved MCP section.

It must explain:

- what MCP adds;
- the trust boundary;
- the six tools;
- build instructions;
- seed instructions;
- client configuration;
- Track 2 narrative;
- M1 refusal;
- M2 refusal;
- promotion → live intent;
- postmortem → falsification;
- retraction → cancellation;
- single-belief limitation;
- source_observed_at limitation;
- CLI fallback.

Do not overclaim.

Specifically do NOT say:

    multi-hop belief graph propagation
    full RetractionCascade across belief_edge
    bitemporal evidence persistence

The honest limitation remains:

    single-belief retraction and cancellation of its dependent live intent

============================================================
31. NO SCOPE CREEP
============================================================

Do NOT build:

    belief_edge propagation
    new kernel semantics
    schema changes
    source_observed_at wiring
    create_belief
    enter_belief
    raw SQL
    force
    override
    direct intent cancellation
    arbitrary fixture paths
    arbitrary evidence URLs
    HTTP
    SSE
    auth
    remote deployment
    frontend
    RAG
    embeddings
    vector DB
    client matrix
    MCP resources
    MCP prompts
    MCP sampling
    streaming
    caching
    session management

If you find yourself needing one of these, STOP.

============================================================
32. STOP CONDITIONS
============================================================

Immediately stop and report if:

- a frozen file must change;
- schema must change;
- kernel must change;
- belief_edge must be populated;
- source_observed_at must be wired through kernel;
- an existing API is materially incompatible;
- the MCP SDK differs materially from the architecture;
- Track 2 seed → ingest produces unexpected semantic state;
- M1 does not produce real 23514;
- M2 does not produce real 23503;
- MCP requires duplicated business logic;
- the 107-test baseline regresses;
- frozen demos regress.

Do NOT solve these by weakening acceptance criteria.

============================================================
33. FINAL RECEIPTS
============================================================

At completion, provide an implementation report.

Include:

### A. Files changed

Exact list.

### B. Frozen-core verification

Actual git diff command/result.

### C. Dependency

Exact MCP SDK version added.

### D. Build

Actual:

    go build ./...
    go vet ./...
    task mcp:build

results.

### E. Existing tests

Actual:

    unset FABLE_TEST_DSN && task test

result.

### F. Frozen demos

Actual results for:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

### G. I-7

Actual raw-write grep result and Taskfile gate result.

### H. M1

Actual MCP refusal:

    SQLSTATE
    constraint
    IsError

### I. M2

Actual MCP refusal:

    SQLSTATE
    constraint
    IsError

### J. Seed

Actual seed state.

### K. Seed → ingest

Actual:

    belief count
    evidence count
    intent count
    UUID stability
    debt states
    audit
    belief_edge

### L. Falsification

Actual:

    retracted belief
    cancelled intent
    remaining beliefs
    audit
    belief_edge

### M. Real MCP-client rehearsal

Current transcript.

### N. Deviations

If none:

    "No deviations from plan4.1."

If any:

    list each one explicitly.

============================================================
34. RECEIPT HONESTY
============================================================

This project has already caught stale transcripts in previous reviews.

Therefore:

DO NOT:

- copy old output;
- invent output;
- present expected output as actual;
- claim a fresh-machine test without performing it;
- claim an MCP-client rehearsal without actually connecting a client.

Every runtime claim must come from the current implementation.

If something cannot be run, explicitly say:

    NOT RUN

and explain why.

============================================================
35. DEFINITION OF DONE
============================================================

The implementation is complete only when ALL are true:

    [ ] 107 existing tests remain green
    [ ] frozen core unchanged
    [ ] MCP builds
    [ ] six tools implemented
    [ ] no raw writes in MCP/view
    [ ] no MCP semantic gate logic
    [ ] fixture boundary enforced
    [ ] missing fixture root fails fast
    [ ] M1 real 23514 refusal
    [ ] M2 real 23503 refusal
    [ ] promotion works after debt clearance
    [ ] live intent created
    [ ] Track 2 ingest verified
    [ ] baseline UUID stable
    [ ] falsification works
    [ ] intent cancelled
    [ ] audit 0
    [ ] belief_edge 0
    [ ] frozen demos still pass
    [ ] real MCP-client rehearsal succeeds
    [ ] current transcript captured
    [ ] README updated honestly
    [ ] no unapproved scope added

Do not declare completion until these have been actually verified.

============================================================
FINAL PRINCIPLE
============================================================

The entire MCP phase exists to make one architectural fact visible:

    The agent is not the authority.

The agent can reason.
The agent can request.
The MCP server can forward.

But:

    Solvent's kernel and CockroachDB enforce the rules.

The most important live demonstration is therefore:

    Agent: "Promote this."
    Solvent: "NO — 23514."

and later:

    Agent: "Authorize this deployment."
    Solvent: "NO — 23503."

and finally:

    Belief → RETRACTED
    Intent → CANCELLED

The implementation must make that behavior real, deterministic, auditable,
and demonstrable.

Implement `plan4.1.md` now.
```
