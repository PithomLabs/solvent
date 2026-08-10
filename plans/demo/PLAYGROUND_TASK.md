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

    Evidence → Belief → Debt / review → Promotion → Intent

and:

    New evidence → previously authorized belief invalidated → RetractCascade → Intent cancelled

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

Create or improve the repository Taskfile with a small, obvious surface.

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

Use names intuitive to a first-time contributor.

============================================================
3. COCKROACHDB SETUP
============================================================

Assume the judge may NOT have CockroachDB installed.

Prefer a Docker-based CockroachDB container using a PINNED version
compatible with the version already verified by the project (see F2 — pin
the EXACT verified version, not merely a compatible one).

Do NOT silently install system packages. Do NOT modify the host unnecessarily.

`task setup` must, IN THIS ORDER:

1. check required prerequisites (Docker + Go + Task — see F4)
2. detect whether Docker is available
3. start the pinned CockroachDB container using the repository's
   reproducible configuration
4. wait until CockroachDB is ready
5. APPLY every required non-default cluster/session setting (see F2). This
   step is separate from container flags: `SET CLUSTER SETTING` is applied
   after the node is ready, before the schema. Do not fold it into image
   flags and assume it took.
6. initialize/apply the existing Solvent schema
7. verify the database connection
8. run a minimal sanity check
9. print the next commands for the judge

If Docker is unavailable: fail clearly and name the missing prerequisite.
Do NOT invent an alternative database implementation.

============================================================
4. DATABASE COMMANDS
============================================================

    task db:up      — start the pinned local CockroachDB instance
    task db:down    — stop/remove the project-local container safely
    task db:reset   — reset to a known empty state and reapply the existing
                      schema (and re-apply required cluster/session settings
                      per F2, in the same order as setup)

Do NOT modify the schema. Do NOT create a second schema.
Do NOT introduce SQL dumps as the canonical demo dataset.

The canonical demo inputs remain the pinned etcd fixtures already in:

    internal/derive/testdata/etcd_real/

The database is reconstructed state.

============================================================
5. TRACK 1 DEMO
============================================================

    task demo:track1

Reset the database first so the result is deterministic, then execute the
EXISTING Track 1 workflow. Do not create a new application path.

Story:

    GHSA/CVE evidence → etcd v3.5.27 → etcd v3.5.28 →
    beliefs/evidence/debt → operator review → promotion → live action intent

Concise, readable output. At the end show enough for a judge to understand:
beliefs created, evidence processed, debt/review, promoted belief, live
intent, audit result. Do not dump the entire database.

============================================================
6. TRACK 1 REPLAY
============================================================

    task demo:replay

Demonstrates deterministic idempotence using the existing pipeline semantics.
Do not claim intent creation itself is part of ingestion replay.

SEQUENCING (resolves the reset ambiguity — see F3):
`demo:replay` ASSUMES `demo:track1` ran immediately prior and operates on
that populated database. It MUST NOT reset. Running any resetting demo
(e.g. `demo:track2`) between `demo:track1` and `demo:replay` invalidates the
comparison; the README and the command's own preamble must state this.

Show:

    first run → resulting ledger state → replay → no duplicate
    beliefs/evidence/intents → same semantic result

Use actual database inspection to establish the result. Output should say:

    Replay verified: no duplicate ledger state created.

Do not fabricate this message unless the underlying checks actually pass
(see F3 — if the pipeline does not truly dedup, STOP and report).

============================================================
7. TRACK 2 DEMO
============================================================

    task demo:track2

Reset the database first. Execute the existing two-stage Track 2 workflow.

STEP 1: enter postulated historical deployment belief; attach real v3.5.0
        release evidence.
STEP 2: operator review; retire required debts; promote belief; create live
        intent.
STEP 3: ingest real v3.5 data-inconsistency postmortem; falsify the target
        belief; RetractCascade; verify belief = retracted; verify intent =
        cancelled; verify audit invariant.

The final output should make the causal sequence obvious. See F1: the
narrative is single-belief retraction + cancellation of ITS live intent, not
downstream/graph propagation.

============================================================
8. INSPECTION COMMAND
============================================================

    task inspect

Concise human-readable snapshot of the current ledger. At minimum:

    BELIEFS   id, scenario, status, claim
    EVIDENCE  id, belief, source URL/type
    INTENTS   id, belief, action, state
    AUDIT     live intents on non-promoted beliefs

Do not expose raw CockroachDB internals unless useful. Do not create a
separate UI. Use existing CLI/database access patterns where possible.

============================================================
9. TEST COMMAND
============================================================

    task test

Run the project's complete supported verification suite. At minimum:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...
    gofmt -l cmd internal
    scripts/check_i7.sh   (only if it exists — see F5)

If the repo already has a canonical test command, reuse it. If any tests
require CockroachDB, ensure the DB is running or establish the dependency
(see F5). The Taskfile must not hide failures.

============================================================
10. README — 5 MINUTE PLAYGROUND
============================================================

Update README.md to a very short judge-facing path: one-sentence
explanation of the Transactional Belief Ledger; Quick Start (`task setup`);
Watch Track 1 (`task demo:track1`) with evidence→belief→debt→promotion→intent
bullets; Replay It (`task demo:replay`) with what replay demonstrates AND the
"track1 immediately prior, no reset between" note; Watch Retraction
(`task demo:track2`) with belief→authorized action→new evidence→RetractCascade
→cancelled intent (single-belief, per F1); Inspect (`task inspect`); then
deeper links for engineers. Do NOT write a giant README. The first five
minutes matter more than exhaustive documentation.

============================================================
11. JUDGE EXPERIENCE
============================================================

Output must be understandable without reading Go. Prefer concise sections:

    === SOLVENT / TRACK 1 ===
    Evidence      3
    Beliefs       3
    Promoted      1
    Live intents  1
    Audit         SAFE

    === SOLVENT / TRACK 2 ===
    Decision      PROMOTED
    Action        LIVE
    New evidence  POSTMORTEM
    Retraction    EXECUTED
    Belief        RETRACTED
    Action        CANCELLED
    Audit         SAFE

Use actual values from the database. Do not hard-code results to make the
presentation look good (see final review point 10).

============================================================
12. DETERMINISM
============================================================

Every demo command must be reproducible. Avoid: current timestamps in
semantic output, random IDs in displayed comparisons where avoidable,
network calls, live GitHub APIs, NVD/OSV APIs, external services, mutable
external dependencies.

The real etcd fixture files are the canonical inputs. Do not download the
real data during demo execution. See F6 for the source_observed_at vs
ingested_at distinction.

============================================================
13. SCOPE BOUNDARY
============================================================

DO NOT: modify db/001_schema.sql; modify kernel semantics; modify FullDebt;
introduce new tables; introduce belief_edge relationships; add RAG; add
embeddings; add vector databases; add Kafka; add Kubernetes; add a new web
application; add bchat yet; replace the existing CLI; introduce a second
database technology; create a SQL dump as the canonical dataset; redesign
the etcd pipeline.

If something appears necessary, STOP and report it rather than expanding scope.

============================================================
14. BCHAT
============================================================

Do NOT implement bchat integration in this task. The CLI playground must
work independently first. bchat will later sit on top of the verified
Solvent workflows.

============================================================
15. ACCEPTANCE CRITERIA
============================================================

Complete only if:

- `task setup` works on a clean machine with Docker but WITHOUT CockroachDB
  installed: starts pinned CRDB, applies required settings, initializes
  schema, verifies readiness.
- `task demo:track1` produces a real successful Track 1 run.
- `task demo:replay`, run immediately after `demo:track1` and WITHOUT reset,
  proves no duplicate semantic ledger state via a real dedup mechanism.
- `task demo:track2` proves: postulated belief → promotion → live intent →
  postmortem → RetractCascade → retracted belief → cancelled intent
  (single-belief, per F1).
- `task inspect` shows actual current database state.
- `task test` passes.
- Existing 107-test baseline remains green unless a test is legitimately
  updated because of a directly related change.
- No schema/kernel architectural changes.

============================================================
16. DELIVERABLES
============================================================

Produce: `Taskfile.yml`, updated `README.md`, and only the minimal
supporting scripts/configuration required (keep scripts small; explain why
each exists).

Also produce `plans/post/playground_implementation_report.md` containing:

1. Files changed
2. Commands added
3. CockroachDB setup mechanism (exact pinned version + every non-default
   cluster/session setting encoded, per F2)
4. Track 1 demo result
5. Replay result (including the dedup mechanism identified, per F3)
6. Track 2 demo result
7. Inspection output/result
8. Test results
9. Any prerequisite requirements (Docker + Go + Task, per F4)
10. Confirmation that schema/kernel remain unchanged — or, if inspection
    shows kernel.go is NOT unchanged, report the discrepancy honestly
11. Remaining limitations — MUST include the F1 statement verbatim, AND note
    that belief_edge-based multi-belief propagation is a deliberate future
    deferral (a single real dependency edge is what would make RetractCascade
    propagate); it is dormant by design, not a defect.

============================================================
CRITICAL PLAYGROUND CORRECTIONS — MUST VERIFY
============================================================

F1 — RETRACTIONCASCADE HONESTY

The current Solvent implementation has belief_edge intentionally unused.
Therefore do NOT describe Track 2 as demonstrating multi-node/downstream
cascade propagation. The judge-facing narrative and output must say:

    belief retracted → its live intent cancelled

Do NOT claim: downstream beliefs retracted; graph propagation; cascading
through dependent beliefs.

Before implementation, inspect RetractCascade and confirm its actual behavior.

The playground report and README must explicitly state this limitation:

    "The current MVP demonstrates single-belief retraction and cancellation
     of its dependent live intent. belief_edge-based multi-belief propagation
     is intentionally outside the current demo."

Do NOT add belief_edge relationships to fix this.

------------------------------------------------------------

F2 — REPRODUCIBLE COCKROACHDB ENVIRONMENT

Do NOT merely choose a CockroachDB image that is "compatible." Inspect the
repository and existing verification environment and determine the EXACT
CockroachDB version used for the verified E2E workflow. Pin that exact image.

Also inspect the existing Solvent code and scripts for EVERY non-default
CockroachDB cluster/session setting required by the verified workflows. In
particular, inspect RetractCascade for temporary-table usage. If CREATE TEMP
TABLE or another feature requires a non-default CockroachDB setting, the
Docker setup MUST configure that setting explicitly (applied after readiness,
before schema — see §3 step 5 and §4 db:reset).

The fresh judge container must reproduce the verified environment. Do NOT
assume image + schema = sufficient. Verify: exact CRDB version, required
cluster settings, required session settings, isolation assumptions, database
initialization, readiness.

If the current kernel requires a setting, configure it in the disposable
local demo environment rather than modifying kernel semantics. If inspection
reveals that kernel.go is NOT actually unchanged, report the discrepancy
rather than silently claiming it is unchanged.

------------------------------------------------------------

F3 — REPLAY MUST BE REAL IDEMPOTENCE

`task demo:replay` MUST NOT reset the database. Intended sequence:

    task demo:track1 → existing populated database → task demo:replay →
    run Track 1 ingestion AGAIN → compare before/after counts →
    assert zero duplicate semantic ledger state

Do NOT reset, truncate, or recreate the database during replay.

Before implementing the replay assertion, inspect the actual pipeline and
identify the mechanism that makes replay idempotent. Determine the actual
deduplication key/logic (examples: content_sha256, source_url, semantic claim
key, another existing pipeline key). Do NOT invent a dedup mechanism.

If the existing pipeline does NOT actually deduplicate replay, STOP and report
that as a blocker rather than printing "Replay verified: no duplicate ledger
state created." Replay must prove idempotence, not deterministic
reconstruction after a reset.

------------------------------------------------------------

F4 — PREREQUISITES

`task setup` must verify Docker, Go (compatible with go.mod), and Task, and
fail clearly naming the missing prerequisite. Do not silently install system
packages.

------------------------------------------------------------

F5 — TEST ENVIRONMENT

Inspect whether any of the 107 tests require CockroachDB. If DB-dependent
tests exist, `task test` must ensure the required database is running or
clearly establish the dependency. Also verify that `scripts/check_i7.sh`
actually exists before relying on it. Do not assume repository files exist.

------------------------------------------------------------

F6 — DETERMINISTIC TIME SEMANTICS

Distinguish source_observed_at (meaningful fixed timestamp from the real
fixture) from ingested_at (runtime/database timestamp). Keep meaningful
source/world timestamps in the demo. Do NOT use runtime wall-clock timestamps
when comparing deterministic demo results. Do NOT claim complete output
determinism if runtime-generated fields are being displayed.

============================================================
FINAL ADVERSARIAL REVIEW PROMPT (ready to copy)
============================================================

Review the Solvent judge playground on a FRESH machine with Docker but
without CockroachDB installed. Verify, and STOP-and-report any that cannot be
established from the repository rather than accepting a claim:

1. Fresh-machine `task setup` works: prerequisites checked (Docker + Go +
   Task), pinned CRDB starts, required non-default cluster/session settings
   applied (especially temporary-table support), schema initialized,
   readiness verified.
2. The CockroachDB image EXACTLY matches the verified version — not merely a
   compatible one.
3. Every non-default CRDB setting the verified workflow needs is encoded in
   the disposable demo environment; grep the config/scripts to confirm.
4. `task demo:track1` produces a real run; the summary values are read from
   the database, not printed constants.
5. `task demo:replay` runs WITHOUT reset, immediately after track1, and its
   idempotence is backed by an ACTUAL existing dedup key (name it); confirm
   no truncate/recreate hides in the replay path.
6. `task demo:track2` shows single-belief retraction + cancelled live intent;
   README and output NEVER imply multi-belief/graph propagation (F1).
7. `task inspect` reflects actual current database state.
8. source_observed_at remains meaningful in output while ingested_at is
   excluded from deterministic comparisons (F6).
9. No hidden network dependency: no live GitHub/NVD/OSV/external calls during
   any demo; fixtures are the only inputs.
10. Grep ALL demo scripts and output paths for hard-coded success strings
    (Audit: SAFE, Intent: LIVE, "Replay verified", RETRACTED, CANCELLED). Each
    must be printed only AFTER the corresponding check passes against the DB.
11. No schema/kernel changes: db/001_schema.sql, kernel.go, and FullDebt
    unchanged; no new tables; belief_edge still unused.
12. Existing 107-test baseline green (`task test`), with any DB-dependent
    tests given a reproducible environment.

If any item cannot be verified from the existing repository, STOP and report
the blocker instead of inventing behavior.

============================================================
EXECUTION RULE
============================================================

Implement this directly in the CURRENT LOCAL SOLVENT REPOSITORY. Do NOT
publish. Do NOT commit unless explicitly asked.

First inspect the repository and existing commands. Then implement the
smallest possible playground. After implementation, run the complete
acceptance sequence. If anything fails, fix it. Do not compensate for
failures by weakening tests or hiding errors.

Objective:

    CLONE → TASK SETUP → TASK DEMO → UNDERSTAND SOLVENT →
    INSPECT / PLAY → DEEP DIVE IF INTERESTED

The core ledger is finished. Now make it easy for judges to experience it.



## chatgpt

Yes. **This merged prompt is ready to hand to the coding agent.** The two seams are correctly resolved, and the F1 limitation is now explicitly recorded as a deliberate deferral rather than an accidental gap. 

I would make **one final tiny correction before sending it**:

* At the top it still says **“Track 1 replay passes”** as part of the verified baseline. 
* But F3 correctly requires the agent to **re-establish that replay is genuine idempotence rather than reset-based determinism**. 

That's not fatal, because the prompt later tells the agent to STOP if the existing pipeline lacks real deduplication. But I'd change the top-level claim to:

> `Track 1 replay has been previously reported as passing; this task must independently verify that the replay is genuine idempotence.`

That keeps the prompt epistemically clean.

**Otherwise: send it. No more Claude review of the prompt.**

The agent now has the correct hard gates:

```text
Fresh machine
   ↓
Docker + Go + Task
   ↓
exact verified CRDB version
   ↓
required cluster settings
   ↓
schema
   ↓
Track 1
   ↓
REAL replay/idempotence
   ↓
Track 2
   ↓
single-belief retraction + intent cancellation
   ↓
inspection
   ↓
107-test baseline
```

And critically, if any premise is false, it is instructed to **STOP rather than paper over it**. 

That's exactly the discipline we want at this stage.
