Use this as the coding-agent prompt. I would make the **grounding-rule wording correction part of the implementation instructions**, while otherwise telling the agent to follow Plan 3 exactly and not reopen design decisions.

```text
# SOLVENT — IMPLEMENT PLAN 3: MCP SERVER PHASE A

You are the coding agent implementing the attached:

    plan3(5).md

This document is the FINAL implementation specification for the Solvent
MCP Phase A.

Repository:

    https://github.com/PithomLabs/solvent

READ THE ENTIRE PLAN 3 BEFORE TOUCHING CODE.

Do not redesign the plan.
Do not perform another planning exercise.
Do not broaden scope.

The purpose of this task is to IMPLEMENT the approved specification and
produce the evidence required by its 18 acceptance criteria.

============================================================
0. NON-NEGOTIABLE STATUS
============================================================

Plan 3 is approved for implementation.

The Solvent core and CLI playground are FROZEN.

The following directories/files must not be modified:

    db/
    internal/kernel/
    internal/belief/
    internal/intent/
    internal/derive/
    internal/normalize/
    internal/pipeline/
    cmd/solvent/
    cmd/operator-review/

Do not modify kernel semantics.
Do not modify schema.
Do not populate belief_edge.
Do not modify RetractCascade.
Do not add source_observed_at persistence.
Do not refactor the CLI to share code with MCP.

The MCP server is an ADAPTER over the existing Solvent semantics.

Its architecture is:

    Agent
       ↓
    MCP server
       ↓
    existing Solvent kernel/pipeline/view
       ↓
    CockroachDB

The MCP layer is NOT a second implementation of Solvent.

============================================================
1. PRIMARY OBJECTIVE
============================================================

Build the Phase A stdio MCP server described by Plan 3.

The central demo must make these two events observable:

M1:

    Agent attempts to promote a belief carrying open debt
        ↓
    CockroachDB rejects it
        ↓
    SQLSTATE 23514
        ↓
    constraint promoted_is_debt_free

M2:

    Agent attempts to authorize an action citing a non-promoted belief
        ↓
    CockroachDB rejects it
        ↓
    SQLSTATE 23503
        ↓
    constraint gate

Then demonstrate:

    debt cleared
        ↓
    promotion succeeds
        ↓
    deployment intent becomes live
        ↓
    real postmortem evidence is ingested
        ↓
    original belief is falsified
        ↓
    belief becomes retracted
        ↓
    dependent live intent becomes cancelled
        ↓
    audit remains 0

The closing conceptual proof is:

    The model did not cancel the deployment.
    The database did.

============================================================
2. FIRST STEP — INSPECT, DO NOT ASSUME
============================================================

Before implementing anything:

1. Read plan3(5).md completely.
2. Inspect the current repository state.
3. Verify the referenced functions, files, schemas, Taskfile,
   scripts, and package APIs actually exist as described.
4. Inspect the existing MCP-related dependencies, if any.
5. Inspect the current Go version and go.mod.
6. Inspect the current Taskfile.
7. Inspect:
       internal/kernel/
       internal/pipeline/
       cmd/operator-review/
       scripts/check_i7.sh
       scripts/demo/
       internal/derive/testdata/etcd_real/
8. Confirm the frozen-core state before editing.

If a plan assertion is materially false, STOP and report the discrepancy
rather than silently redesigning around it.

Do not "fix" unrelated pre-existing issues.

============================================================
3. MCP IMPLEMENTATION BOUNDARY
============================================================

Create the new files specified by Plan 3:

    cmd/solvent-mcp/main.go
    cmd/solvent-mcp/tools.go
    cmd/solvent-mcp/errors.go
    internal/view/view.go
    .mcp.json

Modify only the approved packaging/integration surfaces:

    go.mod
    go.sum
    Taskfile.yml
    README.md

No other files should change unless absolutely required by the MCP SDK
dependency or Go tooling, and if so report the reason explicitly.

Expected dependency:

    github.com/modelcontextprotocol/go-sdk

Use the official Go SDK described in Plan 3.

stdio transport only.

Do NOT add:

    HTTP
    SSE
    authentication
    authorization layer
    session state
    caching
    remote hosting
    MCP resources
    MCP prompts
    MCP sampling
    streaming/progress
    Docker packaging
    client compatibility matrix

============================================================
4. TRUST BOUNDARY — CRITICAL
============================================================

The MCP server must remain a thin adapter.

Handlers should conceptually be:

    unmarshal
        ↓
    existing kernel/pipeline/view call
        ↓
    format result

There must be ZERO MCP business logic deciding whether an operation is
allowed.

Specifically:

DO NOT write logic such as:

    if belief.Status != "promoted" { ... }
    if len(belief.Debt) > 0 { ... }
    if intent.State != "live" { ... }

to determine whether an operation should proceed.

The handler must call the existing domain operation and let the database
decide.

Especially:

    solvent_promote

MUST NOT pre-check debt.

Call:

    kernel.Store.Promote(...)

and report the actual database refusal.

Likewise:

    solvent_authorize_action

MUST NOT pre-check promotion status.

Call:

    kernel.Store.IntentOnPromoted(...)

and report the actual database refusal.

The database is the authority.

============================================================
5. SIX MCP TOOLS — IMPLEMENT EXACTLY
============================================================

Implement these six tools and no additional domain tools.

------------------------------------------------------------
5.1 solvent_ledger
------------------------------------------------------------

Input:

    {
      "scenario": "track1|track2",
      "belief_id": "uuid (optional)",
      "include_evidence": "boolean (optional, default false)"
    }

Read-only.

Use the new read-only view layer plus:

    pipeline.AuditIntent(ctx, db, scenarioID)

Return:

    beliefs:
        id
        claim
        claim_type
        status
        debt

    evidence when requested:
        belief_id
        source_url
        provenance_class
        content_sha256

    intents:
        belief_id
        action
        state

    audit_live_on_nonpromoted

Do not independently calculate the audit count.

No writes.

------------------------------------------------------------
5.2 solvent_ingest_evidence
------------------------------------------------------------

Input:

    {
      "scenario": "track1|track2"
    }

Resolve the scenario to the server-owned fixture directory.

Call:

    pipeline.Run(ctx, db, scenarioID, fixtureDir)

Do not reimplement:

    fixture reading
    normalization
    derivation
    aggregation
    evidence insertion
    deduplication

Do not accept:

    arbitrary path
    filename
    URL
    free-text evidence

Return the pipeline result plus the standard audit envelope.

------------------------------------------------------------
5.3 solvent_retire_debt
------------------------------------------------------------

Input:

    {
      "belief_id": "uuid",
      "debt_item": "string"
    }

Call:

    kernel.Store.RetireDebt(...)

Use the existing FullDebt semantics.

Do not invent new debt types.

Do not add bulk-clear.

An already absent debt item remains a no-op as specified.

Return resulting debt plus audit.

------------------------------------------------------------
5.4 solvent_promote
------------------------------------------------------------

Input:

    {
      "belief_id": "uuid"
    }

Call:

    kernel.Store.Promote(...)

DO NOT pre-check debt.

On success:

    status = promoted

On refusal, preserve:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

The error must be returned as an MCP TOOL RESULT with IsError=true,
not as a protocol-level failure.

The model must receive the actual refusal.

------------------------------------------------------------
5.5 solvent_authorize_action
------------------------------------------------------------

Input:

    {
      "scenario": "track1|track2",
      "belief_id": "uuid",
      "action": "string"
    }

Call:

    kernel.Store.IntentOnPromoted(...)

DO NOT pre-check promotion status.

On refusal preserve:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Again, return this as an MCP tool result.

------------------------------------------------------------
5.6 solvent_falsify
------------------------------------------------------------

Input:

    {
      "scenario": "track1|track2",
      "belief_id": "uuid"
    }

Call:

    kernel.Store.RetractCascade(...)

Do not implement another retraction algorithm.

Return the actual result.

The demo is explicitly:

    single-belief retraction
    +
    cancellation of that belief's dependent live intent

Do NOT describe this as:

    belief graph propagation
    multi-hop cascade
    belief_edge propagation

belief_edge must remain empty.

============================================================
6. ERROR HANDLING
============================================================

Use the existing error architecture.

Inspect:

    internal/kernel/errors.go

Use errors.As to reach:

    *pgconn.PgError

Preserve:

    err.Error()
    SQLSTATE
    ConstraintName

For example:

    promotion blocked...
    23514
    promoted_is_debt_free

and:

    action refused...
    23503
    gate

Do not substring-match SQL errors.

Do not lose the underlying PgError through MCP wrapping.

Return database refusals as:

    IsError: true

tool results.

Non-database errors:

    IsError: true
    concise human-readable message
    no stack traces
    no silent success

============================================================
7. RESPONSE ENVELOPE
============================================================

For the five state-changing tools:

    ingest_evidence
    retire_debt
    promote
    authorize_action
    falsify

use:

    {
      "result": { ... },
      "audit": {
        "live_on_nonpromoted": 0
      }
    }

including error results.

The ledger tool embeds the audit value in its snapshot as specified.

CRITICAL:

The MCP layer must NEVER independently calculate:

    live_on_nonpromoted

Always obtain it from:

    pipeline.AuditIntent(...)

No count query in cmd/solvent-mcp or internal/view for this value.

============================================================
8. FIXTURE RESOLUTION
============================================================

The only scenario input is:

    track1
    track2

No path parameter.

No filename parameter.

No directory parameter.

No URL parameter.

Resolve internally:

    track1 → UUID 00000000-0000-0000-0000-000000000001
    track2 → UUID 00000000-0000-0000-0000-000000000002

and:

    fixtureRoot/track1
    fixtureRoot/track2

Fixture root comes from:

    SOLVENT_FIXTURE_ROOT

with the specified executable-relative fallback.

At startup:

    stat track1
    stat track2

If either is missing:

    exit non-zero
    mention SOLVENT_FIXTURE_ROOT

Never defer this failure until a tool call.

Call pipeline.Run AS-IS.

Do not substitute ProcessEvidence.

Do not use go:embed as a shortcut.

============================================================
9. INTERNAL VIEW
============================================================

Implement:

    internal/view/view.go

as a read-only projection layer.

It may SELECT.

It must not:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

It must not contain business decisions about whether a transition is
allowed.

The view exists only to present the current ledger state.

============================================================
10. I-7 / ANTI-BYPASS GATE
============================================================

Extend the existing I-7 check in Taskfile.

The existing:

    scripts/check_i7.sh

is parameterized by:

    I7_PKG

Use that mechanism.

Add a check ensuring:

    cmd/solvent-mcp
    internal/view

contain no raw DB writes.

Reads in internal/view are allowed.

The check should reject raw:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

writes in the MCP/view implementation.

Also implement the explicit acceptance check:

    grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" \
      cmd/solvent-mcp internal/view

with no matches.

If implementation needs a legitimate SELECT, that is fine.

============================================================
11. DATABASE CONNECTION
============================================================

Use:

    FABLE_DSN

consistent with the existing project connection model.

Use:

    testdb.Open(dsn)

Ping on startup.

If the DB cannot be reached:

    fail fast
    clearly mention task setup

Do not introduce another DSN convention.

Do not make the MCP server maintain its own DB state.

============================================================
12. SEED TASK
============================================================

Implement:

    task mcp:seed

according to Plan 3.

It must:

1. Reset the fable DB.
2. Reapply the frozen schema using the existing task machinery.
3. Enter exactly the Track 2 historical belief using the existing
   operator-review CLI.

Use:

    scenario:
      00000000-0000-0000-0000-000000000002

Claim:

    etcd v3.5.0 is approved for production deployment
    (decision as of 2021-06-16)

Claim type:

    postulated

Evidence URL:

    https://github.com/etcd-io/etcd/releases/tag/v3.5.0

Evidence SHA:

    f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1

The seed MUST NOT promote the belief.

The seed MUST NOT create an intent.

Expected post-seed state:

    1 belief
    postulated
    entered
    6 debts
    1 evidence
    0 intents
    audit 0

Do not implement a new MCP belief-creation tool.

============================================================
13. SEED → INGEST MUST BE VERIFIED
============================================================

This is one of the most important acceptance checks.

The MCP sequence differs from the frozen CLI sequence.

MCP:

    seed baseline
        ↓
    promote
        ↓
    authorize
        ↓
    ingest Track 2
        ↓
    falsify baseline

The existing CLI track2 does the ingestion first.

Do not assume these produce equivalent state.

VERIFY it.

After:

    solvent_ingest_evidence(track2)

against the already-seeded/promoted/authorized DB:

verify:

    3 beliefs

    baseline:
        same UUID as the seeded baseline
        postulated
        promoted
        debt empty

    accommodated belief:
        entered
        4 debts

    derived belief:
        entered
        6 debts

    3 evidence rows

    3 distinct content_sha256 values

    1 live intent attached to baseline

    audit 0

    belief_edge 0

Then:

    solvent_falsify(baseline UUID)

must produce:

    baseline → retracted
    baseline intent → cancelled
    other two beliefs remain entered
    retracted = 1
    audit = 0
    belief_edge = 0

Do not merely assert these conditions in documentation.

Actually run and verify them.

============================================================
14. GROUNDING RULE — ONE SMALL CLARIFICATION
============================================================

Plan 3 says every state assertion must come from the "immediately
preceding tool result."

That wording is unnecessarily strict relative to the actual tool
contracts.

Implement the intended anti-memory rule as follows:

> Every state, count, status, or identifier the agent asserts must be
> grounded in the most recent tool result that contains that fact.
> After a mutation, the agent must use solvent_ledger to re-read any
> state it intends to assert rather than relying on an earlier result.

This is a documentation/spec clarification only.

DO NOT add duplicated state to every mutation response merely to satisfy
the old wording.

Prefer a clean pattern such as:

    mutation
        ↓
    ledger read
        ↓
    agent assertion

when the agent needs to state resulting state.

The critical properties remain:

- no stale UUIDs;
- no invented counts;
- no remembered statuses;
- no invented SQLSTATE;
- no state claims unsupported by tool results.

============================================================
15. TOOL DESCRIPTIONS
============================================================

Use the descriptions from Plan 3 essentially verbatim.

Especially preserve these concepts:

solvent_promote:

    The database decides whether promotion is permitted.
    Do not pre-check debt.
    Attempt the operation and report the result.

solvent_authorize_action:

    The database decides whether the belief is promoted.
    Do not pre-check.

solvent_falsify:

    Single-belief retraction plus cancellation of its dependent live intent.
    No belief graph propagation.

solvent_ledger:

    The only source of truth for current state.
    Read before making claims about counts/status/identifiers.

solvent_ingest_evidence:

    Full existing pipeline.
    Deterministic/idempotent.

solvent_retire_debt:

    Existing six debt items only.

Do not weaken these descriptions for convenience.

============================================================
16. .MCP.JSON
============================================================

Create the project-scoped MCP configuration described by Plan 3.

It must:

- launch the built solvent MCP executable;
- pass FABLE_DSN;
- pass SOLVENT_FIXTURE_ROOT;
- use stdio;
- not contain secrets;
- not expose arbitrary fixture paths to tool callers.

Inspect the repository's actual layout and Taskfile before deciding exact
paths.

Do not hard-code machine-specific absolute paths.

============================================================
17. TASKFILE
============================================================

Add:

    mcp:build
    mcp:seed

and the required I-7 gate extension.

Follow existing Taskfile conventions.

Do not make ordinary:

    task test

depend on a running MCP server.

Do not make the 107-test baseline depend on the MCP demo container.

Preserve the previously verified self-contained test behavior:

    unset FABLE_TEST_DSN && task test

must remain valid.

============================================================
18. README
============================================================

Add the MCP section specified by Plan 3.

The README must explain:

1. What MCP adds.
2. The trust boundary.
3. The six tools.
4. How to build.
5. How to seed.
6. How to connect a local MCP client.
7. The Track 2 judge conversation.
8. The two database refusals.
9. Promotion → live intent → falsification → cancellation.
10. The honest limitation:

    single-belief retraction and cancellation of its dependent live intent

Never claim:

    multi-hop propagation
    belief graph cascade

Also state that:

    source_observed_at

is not currently persisted by the frozen kernel and is deferred.

Keep the CLI playground as the primary fallback.

============================================================
19. ACCEPTANCE TESTING
============================================================

Do not stop after compilation.

Run the full acceptance sequence.

------------------------------------------------------------
A. Core freeze
------------------------------------------------------------

Verify:

    git diff --stat HEAD -- \
      db/ \
      internal/kernel \
      internal/belief \
      internal/intent \
      internal/derive \
      internal/normalize \
      internal/pipeline \
      cmd/solvent \
      cmd/operator-review

is empty.

Then:

    unset FABLE_TEST_DSN && task test

Must remain:

    107 tests pass
    0 failures

Also:

    go build
    go vet
    gofmt
    check_i7

must pass.

Then verify frozen demos:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

all pass with unchanged behavior.

------------------------------------------------------------
B. MCP boundary
------------------------------------------------------------

Verify:

    no raw DB writes in cmd/solvent-mcp
    no raw DB writes in internal/view

Verify no semantic conditional logic in MCP based on:

    debt
    belief status
    intent state

Verify audit values all originate from:

    pipeline.AuditIntent

------------------------------------------------------------
C. Fixture safety
------------------------------------------------------------

Verify no MCP tool schema contains:

    path
    directory
    filename
    URL

for fixture selection.

Verify:

    scenario = track1 | track2

only.

Start with invalid/missing:

    SOLVENT_FIXTURE_ROOT

and verify the server fails fast.

------------------------------------------------------------
D. M1
------------------------------------------------------------

Run:

    task mcp:seed

Connect an MCP client.

Call:

    solvent_ledger(track2)

Then:

    solvent_promote(baseline UUID)

Verify live:

    IsError = true
    SQLSTATE = 23514
    constraint = promoted_is_debt_free

This must be a real DB refusal.

------------------------------------------------------------
E. M2
------------------------------------------------------------

On the same non-promoted baseline:

    solvent_authorize_action(
        track2,
        baseline UUID,
        "deploy etcd v3.5.0"
    )

Verify:

    IsError = true
    SQLSTATE = 23503
    constraint = gate

Again, this must be the actual DB refusal.

------------------------------------------------------------
F. Payoff
------------------------------------------------------------

Retire all six debts.

Promote.

Verify through ledger.

Authorize:

    deploy etcd v3.5.0

Verify:

    belief promoted
    intent live
    audit 0

Then:

    solvent_ingest_evidence(track2)

Verify the exact §7.2 state.

Then:

    solvent_ledger(track2)

read the baseline UUID again.

Then:

    solvent_falsify(track2, baseline UUID)

Verify:

    baseline retracted
    baseline intent cancelled
    other two beliefs remain entered
    audit 0
    belief_edge 0

------------------------------------------------------------
G. Grounding rehearsal
------------------------------------------------------------

Perform at least one full end-to-end rehearsal through a real MCP client.

Capture the transcript.

Verify:

- six-debt statement came from ledger;
- UUIDs passed to mutations came from ledger;
- status claims after mutation came from the most recent tool result
  containing those statuses, preferably a fresh ledger read;
- SQLSTATE/constraint claims came from actual refusal results;
- no number or status was invented from the fixture or from this plan.

============================================================
20. IMPORTANT: DO NOT FAKE RECEIPTS
============================================================

This repository has already gone through multiple adversarial reviews
where stale transcripts were discovered.

Do NOT reuse old transcripts.

Do NOT manufacture example output and present it as execution evidence.

Do NOT copy expected output into a report and call it a receipt.

After implementation:

    run the commands
    capture actual output
    verify actual state
    record actual receipts

If something cannot be executed, say so.

============================================================
21. FINAL VERIFICATION REPORT
============================================================

After implementation, produce a concise implementation report containing:

1. Files changed.
2. Frozen-core diff verification.
3. Dependency added.
4. MCP architecture implemented.
5. Six tools implemented.
6. I-7 gate result.
7. Seed result.
8. M1 actual receipt.
9. M2 actual receipt.
10. Seed → ingest verification.
11. Final falsification/cancellation receipt.
12. Full MCP rehearsal transcript.
13. Test/build/vet/gofmt results.
14. Any deviations from Plan 3.
15. Any unresolved issues.

For every claimed runtime result, provide actual command/output evidence.

Do not claim success merely because the code compiles.

============================================================
22. STOP CONDITIONS
============================================================

STOP and report rather than improvising if:

- frozen-core files need modification;
- kernel semantics need modification;
- schema needs modification;
- belief_edge needs population;
- source_observed_at needs kernel changes;
- existing exported APIs are incompatible with the plan;
- the MCP SDK API differs materially from the specified design;
- the Track 2 seed → ingest state differs materially from §7.2;
- M1 cannot produce the real 23514 refusal;
- M2 cannot produce the real 23503 refusal;
- the MCP layer requires semantic business logic to function;
- the 107-test baseline regresses;
- frozen demos regress.

Do not "solve" a STOP condition by weakening the acceptance criterion.

============================================================
23. FINAL SCOPE FENCE
============================================================

DO NOT build:

- belief_edge propagation
- multi-hop cascade
- create_belief MCP tool
- enter_belief MCP tool
- raw SQL tool
- direct intent cancellation
- force/override/admin
- arbitrary evidence URLs
- arbitrary fixture paths
- HTTP
- SSE
- authentication
- remote deployment
- caching
- MCP sessions
- MCP resources
- MCP prompts
- MCP sampling
- streaming
- frontend
- RAG
- embeddings
- vector DB
- client compatibility matrix
- source_observed_at wiring
- kernel changes
- schema changes
- CLI refactor

The MCP phase is an adapter layer over the frozen Solvent core.

============================================================
24. DEFINITION OF DONE
============================================================

You are DONE only when:

    107 existing tests remain green
    +
    frozen core remains byte-for-byte unchanged
    +
    MCP builds
    +
    I-7 MCP boundary check passes
    +
    six tools work
    +
    fixture boundary is enforced
    +
    M1 is observed against real CockroachDB
    +
    M2 is observed against real CockroachDB
    +
    Track 2 seed → ingest state is verified
    +
    promotion → live intent works
    +
    falsification → cancellation works
    +
    belief_edge remains 0
    +
    at least one real MCP-client rehearsal succeeds
    +
    transcript is captured from the CURRENT implementation
    +
    README explains the honest scope
    +
    no unapproved scope has been added

The most important thing to preserve is the central proof:

    The agent can request.
    The MCP server can forward.
    The kernel can execute.
    But CockroachDB decides.

Implement Plan 3 now.
```
