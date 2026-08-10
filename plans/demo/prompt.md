# TASK — BUILD THE SOLVENT JUDGE PLAYGROUND: TASKFILE + REPRODUCIBLE DEMO

You are continuing work in the existing Solvent repository:

https://github.com/PithomLabs/solvent

The core transactional belief ledger and real-world etcd integration are
COMPLETE and VERIFIED.

Current verified state:

- real etcd fixtures are integrated
- Track 1 E2E passes
- Track 1 replay passes
- Track 2 E2E passes
- operator-review --action works
- --falsify / RetractCascade works
- scenario guard works
- CockroachDB E2E verified
- schema unchanged
- kernel.go unchanged
- belief_edge remains unused
- manifest hashes verified
- 107 tests pass

DO NOT reopen or redesign that implementation.

The goal of this task is now:

    MAKE SOLVENT EASY FOR A JUDGE TO CLONE, START, RUN, INSPECT,
    REPLAY, AND UNDERSTAND.

This is a playground/packaging task.

============================================================
PRIMARY USER EXPERIENCE
============================================================

A judge should ideally be able to do:

    git clone ...
    cd solvent
    task setup

and then:

    task demo:track1
    task demo:replay
    task demo:track2

without manually installing CockroachDB.

The judge should be able to inspect what happened with simple commands.

The experience should communicate:

    Evidence
       ↓
    Belief
       ↓
    Debt / review
       ↓
    Promotion
       ↓
    Intent

and:

    New evidence
       ↓
    Previously authorized belief invalidated
       ↓
    RetractCascade
       ↓
    Intent cancelled

Do not turn this into a generic framework.

============================================================
1. FIRST — INSPECT THE EXISTING REPOSITORY
============================================================

Before changing anything, inspect:

- existing Taskfile, if present
- README.md
- AGENTS.md
- existing scripts
- current CLI commands/help
- CockroachDB setup instructions
- existing Docker/container configuration
- existing fixture directories
- current etcd demo commands
- current test commands
- current schema initialization mechanism

Reuse existing mechanisms wherever possible.

Do NOT invent duplicate setup paths.

============================================================
2. TASKFILE
============================================================

Create or improve the repository Taskfile.

The Taskfile should provide a small, obvious command surface.

At minimum:

    task setup
    task db:up
    task db:down
    task db:reset

    task demo:track1
    task demo:replay
    task demo:track2

    task inspect
    task test

Use names that are intuitive to a first-time contributor.

============================================================
3. COCKROACHDB SETUP
============================================================

IMPORTANT:

Assume the judge may NOT have CockroachDB installed.

Prefer a Docker-based CockroachDB container using a PINNED version
compatible with the version already verified by the project.

Do NOT silently install system packages.

Do NOT modify the host unnecessarily.

`task setup` should:

1. check required prerequisites
2. detect whether Docker is available
3. start the pinned CockroachDB container using the repository's
   reproducible configuration
4. wait until CockroachDB is ready
5. initialize/apply the existing Solvent schema
6. verify the database connection
7. run a minimal sanity check
8. print the next commands for the judge

If Docker is unavailable:

    fail clearly

and tell the user exactly what prerequisite is missing.

Do NOT invent an alternative database implementation.

============================================================
4. DATABASE COMMANDS
============================================================

Implement:

    task db:up

Starts the pinned local CockroachDB instance.

    task db:down

Stops/removes the project-local database container safely.

    task db:reset

Resets the local demo database to a known empty state and reapplies the
existing Solvent schema.

Do NOT modify the schema.

Do NOT create a second schema.

Do NOT introduce SQL dumps as the canonical demo dataset.

The canonical demo inputs remain the pinned etcd fixtures already in:

    internal/derive/testdata/etcd_real/

The database is reconstructed state.

============================================================
5. TRACK 1 DEMO
============================================================

Implement:

    task demo:track1

It should reset the database first so the result is deterministic.

Then execute the EXISTING Track 1 workflow.

Do not create a new application path.

The intended story is:

    GHSA/CVE evidence
          ↓
    etcd v3.5.27
          ↓
    etcd v3.5.28
          ↓
    beliefs/evidence/debt
          ↓
    operator review
          ↓
    promotion
          ↓
    live action intent

The command should produce concise, readable output.

At the end show enough information for a judge to understand:

    beliefs created
    evidence processed
    debt/review
    promoted belief
    live intent
    audit result

Do not dump the entire database.

============================================================
6. TRACK 1 REPLAY
============================================================

Implement:

    task demo:replay

This must demonstrate deterministic replay/idempotence using the existing
pipeline semantics.

Do not claim intent creation itself is part of ingestion replay.

Show:

    first run
      ↓
    resulting ledger state
      ↓
    replay
      ↓
    no duplicate beliefs/evidence/intents
      ↓
    same semantic result

Use actual database inspection to establish the result.

The output should explicitly say something like:

    Replay verified: no duplicate ledger state created.

Do not fabricate this message unless the underlying checks actually pass.

============================================================
7. TRACK 2 DEMO
============================================================

Implement:

    task demo:track2

Reset the database first.

Execute the existing two-stage Track 2 workflow:

STEP 1:

    enter postulated historical deployment belief
    attach real v3.5.0 release evidence

STEP 2:

    operator review
    retire required debts
    promote belief
    create live intent

STEP 3:

    ingest real v3.5 data-inconsistency postmortem
    falsify the target belief
    RetractCascade
    verify belief = retracted
    verify intent = cancelled
    verify audit invariant

The final output should make the causal sequence obvious.

============================================================
8. INSPECTION COMMAND
============================================================

Implement:

    task inspect

This should provide a concise human-readable snapshot of the current
ledger.

Show at minimum:

    BELIEFS
      ID
      scenario
      status
      claim

    EVIDENCE
      ID
      belief
      source URL/type

    INTENTS
      ID
      belief
      action
      state

    AUDIT
      live intents on non-promoted beliefs

Do not expose raw CockroachDB internals unless useful.

Do not create a separate UI.

Use the existing CLI/database access patterns where possible.

============================================================
9. TEST COMMAND
============================================================

Implement:

    task test

Run the project's complete supported verification suite.

At minimum:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...
    gofmt -l cmd internal
    scripts/check_i7.sh

If the repository already has a canonical test command, reuse it.

The Taskfile must not hide failures.

============================================================
10. README — 5 MINUTE PLAYGROUND
============================================================

Update README.md to provide a very short judge-facing path.

The first-time experience should roughly be:

    # Solvent

    One-sentence explanation of the Transactional Belief Ledger.

    ## Quick Start

        task setup

    ## Watch Track 1

        task demo:track1

    Explain in a few bullets:

    evidence → belief → debt → promotion → intent

    ## Replay It

        task demo:replay

    Explain what the replay demonstrates.

    ## Watch Retraction

        task demo:track2

    Explain:

    belief → authorized action → new evidence → RetractCascade → cancelled intent

    ## Inspect the Ledger

        task inspect

    Then provide links/sections for engineers who want to go deeper.

Do NOT write a giant README.

The first five minutes matter more than exhaustive documentation.

============================================================
11. JUDGE EXPERIENCE
============================================================

The output must be understandable without reading Go code.

Prefer concise sections such as:

    === SOLVENT / TRACK 1 ===

    Evidence      3
    Beliefs       3
    Promoted      1
    Live intents  1
    Audit         SAFE

and:

    === SOLVENT / TRACK 2 ===

    Decision      PROMOTED
    Action        LIVE
    New evidence  POSTMORTEM
    Retraction    EXECUTED
    Belief        RETRACTED
    Action        CANCELLED
    Audit         SAFE

Use actual values from the database.

Do not hard-code results merely to make the presentation look good.

============================================================
12. DETERMINISM
============================================================

Every demo command must be reproducible.

Avoid:

- current timestamps in semantic output
- random IDs in displayed comparisons where avoidable
- network calls
- live GitHub APIs
- NVD/OSV APIs
- external services
- mutable external dependencies

The real etcd fixture files are the canonical inputs.

Do not download the real data during demo execution.

============================================================
13. SCOPE BOUNDARY
============================================================

DO NOT:

- modify db/001_schema.sql
- modify kernel semantics
- modify FullDebt
- introduce new tables
- introduce belief_edge relationships
- add RAG
- add embeddings
- add vector databases
- add Kafka
- add Kubernetes
- add a new web application
- add bchat yet
- replace the existing CLI
- introduce a second database technology
- create a SQL dump as the canonical dataset
- redesign the etcd pipeline

If something appears necessary, STOP and report it rather than expanding
scope.

============================================================
14. BCHAT
============================================================

Do NOT implement bchat integration in this task.

The CLI playground must work independently first.

bchat will later sit on top of the already-verified Solvent workflows.

============================================================
15. ACCEPTANCE CRITERIA
============================================================

The task is complete only if:

### Setup

    task setup

works on a clean machine with Docker but without CockroachDB installed.

It must:

- start pinned CockroachDB
- initialize schema
- verify readiness

### Track 1

    task demo:track1

produces a real successful Track 1 run.

### Replay

    task demo:replay

proves no duplicate semantic ledger state.

### Track 2

    task demo:track2

proves:

    postulated belief
      → promotion
      → live intent
      → postmortem
      → RetractCascade
      → retracted belief
      → cancelled intent

### Inspection

    task inspect

shows actual current database state.

### Tests

    task test

passes.

### Existing correctness

Existing 107-test baseline must remain green unless a test is legitimately
updated because of a directly related change.

### Core

No schema/kernel architectural changes.

============================================================
16. DELIVERABLES
============================================================

Produce:

    Taskfile.yml
    updated README.md

and only the minimal supporting scripts/configuration required.

If scripts are necessary, keep them small and explain why each exists.

Also produce:

    plans/post/playground_implementation_report.md

containing:

1. Files changed
2. Commands added
3. CockroachDB setup mechanism
4. Track 1 demo result
5. Replay result
6. Track 2 demo result
7. Inspection output/result
8. Test results
9. Any prerequisite requirements
10. Confirmation that schema/kernel remain unchanged
11. Any remaining limitations

============================================================
17. FINAL ADVERSARIAL REVIEW PROMPT
============================================================

At the end of the implementation report, include a ready-to-copy prompt
for a separate adversarial reviewer to verify:

- fresh-machine setup
- Docker/CockroachDB reproducibility
- Taskfile correctness
- Track 1
- replay
- Track 2
- inspect output
- README accuracy
- no hidden network dependency
- no hard-coded success output
- no schema/kernel changes
- existing 107-test baseline

============================================================
EXECUTION RULE
============================================================

Implement this directly in the CURRENT LOCAL SOLVENT REPOSITORY.

Do NOT publish.
Do NOT commit unless explicitly asked.

First inspect the repository and existing commands.

Then implement the smallest possible playground.

After implementation, run the complete acceptance sequence.

If anything fails, fix it.

Do not compensate for failures by weakening tests or hiding errors.

The objective is:

    CLONE
      ↓
    TASK SETUP
      ↓
    TASK DEMO
      ↓
    UNDERSTAND SOLVENT
      ↓
    INSPECT / PLAY
      ↓
    DEEP DIVE IF INTERESTED

The core ledger is finished.

Now make it easy for judges to experience it.