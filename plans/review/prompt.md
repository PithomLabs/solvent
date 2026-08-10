# FINAL ADVERSARIAL CODE REVIEW — SOLVENT JUDGE PLAYGROUND

You are performing the FINAL INDEPENDENT ADVERSARIAL CODE REVIEW of the
actual current Solvent repository.

Repository:

    https://github.com/PithomLabs/solvent

This is NOT a plan review.
This is NOT a review of an implementation report alone.

You must inspect the actual repository, actual code, actual scripts, and
actual runtime behavior.

The playground implementation has already gone through multiple planning
and adversarial-review cycles. Your job now is to determine whether the
CURRENT CODE actually deserves to be frozen and shipped as the
judge-facing hackathon playground.

============================================================
CURRENT VERIFIED INTENT
============================================================

The intended final playground is:

    clone repository
        ↓
    task setup
        ↓
    disposable CockroachDB v26.2.0
        ↓
    task demo:track1
        ↓
    task demo:replay
        ↓
    task demo:track2
        ↓
    task inspect

The playground demonstrates:

TRACK 1:

    real etcd evidence
        ↓
    beliefs
        ↓
    debt
        ↓
    operator review
        ↓
    promotion
        ↓
    live action intent

TRACK 1 REPLAY:

    same evidence again
        ↓
    no duplicate beliefs
    no duplicate evidence
    no duplicate intents

TRACK 2:

    historical/postulated deployment belief
        ↓
    review
        ↓
    promotion
        ↓
    live intent
        ↓
    postmortem evidence
        ↓
    falsification
        ↓
    single-belief retraction
        ↓
    dependent live intent cancelled

IMPORTANT:

The current MVP intentionally does NOT demonstrate multi-belief
belief_edge propagation.

The honest claim is:

    single-belief retraction + cancellation of its dependent live intent

Do NOT treat the absence of belief_edge propagation as a defect.

============================================================
AUTHORITATIVE MATERIALS
============================================================

Inspect these files in the repository where present:

    plans/demo/PLAYGROUND_TASK.md
    plans/demo/plan3.md
    plans/demo/plan3_review.md
    plans/demo/plan4.md
    plans/post/playground_implementation_report.md
    README.md

But:

    THE ACTUAL CURRENT CODE IS AUTHORITATIVE.

Do not approve something merely because the implementation report says it
was done.

The report is evidence to compare against the repository and runtime.

============================================================
REVIEW OBJECTIVE
============================================================

Answer one question:

    "Does the current Solvent repository actually implement a
     reproducible, honest, judge-ready playground without regressing
     the verified Solvent core?"

You must independently verify the claims.

Do not manufacture findings merely to keep the review cycle alive.

If the implementation is correct, say APPROVED.

============================================================
1. INSPECT THE ACTUAL DIFF
============================================================

Identify all changes associated with the playground.

Pay particular attention to:

    Taskfile.yml
    scripts/demo/*
    scripts/demo/config.env
    scripts/demo/setup.sh
    scripts/demo/track1.sh
    scripts/demo/replay.sh
    scripts/demo/track2.sh
    scripts/demo/inspect.sh
    README.md
    cmd/solvent/main.go
    playground implementation report

Verify that the final plan4 cosmetic cleanup did not accidentally modify
anything outside its intended scope.

Specifically inspect the `--out` diagnostic change in:

    cmd/solvent/main.go

Verify that:

    fmt.Printf(...)
        →
    fmt.Fprintf(os.Stderr, ...)

or its final equivalent:

- suppresses the capture-harness diagnostic from normal judge-facing
  output
- does NOT suppress real errors
- does NOT change exit codes
- does NOT alter transcript contents
- does NOT alter database behavior

============================================================
2. FRESH-MACHINE SETUP
============================================================

Actually verify the judge path as far as the environment permits.

The acceptance condition is:

    Docker available
    Go available
    Task available
    CockroachDB NOT preinstalled
    no pre-existing Solvent CRDB container
    no cached CRDB image if a cold-pull claim is being made

Verify:

    task setup

does all required work.

Specifically:

- prerequisite checks
- Docker availability
- Go version derived from go.mod
- Task availability
- CRDB image resolution
- CRDB v26.2.0
- container startup
- readiness
- database initialization
- schema application
- build

Do not accept "compatible version" reasoning.

The intended pinned version is:

    cockroachdb/cockroach:v26.2.0

Verify that the tag actually resolves.

If possible, remove the local image before testing so success cannot be
explained by a cached image.

============================================================
3. COCKROACHDB ENVIRONMENT
============================================================

Inspect the actual container configuration.

Verify:

- host SQL port = 26260
- HTTP port = 8081
- container SQL port = 26257
- container HTTP port = 8080
- database name = fable
- container name is consistent
- schema is applied from the repository's actual schema

Verify that the exact CRDB version is the version used by the verified
baseline.

Do NOT require non-default cluster settings if the current implementation
does not need them.

Specifically inspect RetractCascade.

Confirm the current kernel uses recursive CTEs rather than temporary
tables.

The absence of `SET CLUSTER SETTING` should be treated as a deliberate
property if that is what the code establishes.

============================================================
4. TEST ISOLATION
============================================================

This is critical.

Inspect:

    internal/testdb/testdb.go

and the Taskfile.

Verify that:

    task test

does NOT silently depend on the playground's Docker container.

Verify that no hidden:

    FABLE_TEST_DSN

override is being injected.

Run, where possible:

    unset FABLE_TEST_DSN && task test

with the playground container stopped if the repository's existing test
infrastructure permits it.

The test suite should retain its previously verified isolation.

Do NOT approve merely because:

    task setup
    ↓
    task test

works.

The question is whether `task test` itself acquired an unnecessary runtime
dependency on the playground container.

============================================================
5. TRACK 1
============================================================

Run:

    task demo:track1

Verify the actual database state.

Expected semantic result:

    3 beliefs
    3 evidence
    1 promoted belief
    1 live intent
    audit_live_on_nonpromoted = 0

Verify that the displayed counts and statuses are DB-derived.

Search the scripts for hard-coded values such as:

    1
    SAFE
    LIVE
    PROMOTED
    PASS

A status may appear as a literal label, but the success condition must
come from an actual DB query/assertion.

Do NOT accept a success message that is printed regardless of database
state.

============================================================
6. REPLAY — MOST IMPORTANT
============================================================

Run:

    task demo:track1
    task demo:replay

Do NOT reset between these commands.

Verify that `demo:replay` itself does NOT reset, truncate, recreate, or
reinitialize the database.

Capture:

    beliefs before
    evidence before
    intents before

Then replay.

Capture:

    beliefs after
    evidence after
    intents after

Require:

    before == after

for all three.

Verify that the script exits non-zero if the equality fails.

Inspect the actual deduplication mechanisms.

Expected existing mechanisms:

    EnsureBelief
        → find-or-create on (scenario_id, claim)

    evidenceExists
        → dedup on (belief_id, content_sha256)

Verify that the claim strings are deterministic.

Inspect:

    Derive()
    ContentSHA256

Confirm no runtime timestamp/randomness is incorporated into the keys.

Do not accept "replay is deterministic" merely because the database was
reset first.

============================================================
7. TRACK 2
============================================================

Run:

    task demo:track2

Verify the complete actual flow:

    postulated baseline belief
        ↓
    evidence attached
        ↓
    debts retired
        ↓
    promotion
        ↓
    live intent
        ↓
    postmortem evidence
        ↓
    falsification
        ↓
    belief RETRACTED
        ↓
    live intent CANCELLED
        ↓
    audit_live_on_nonpromoted = 0

Verify the actual database state rather than trusting stdout alone.

Verify that the falsification is scoped to the intended belief/scenario.

============================================================
8. F1 HONESTY
============================================================

Inspect:

    README.md
    demo output
    implementation report

Verify they NEVER claim that the current demo demonstrates:

    multi-belief propagation
    graph-wide cascade
    belief_edge propagation

The correct claim is:

    single-belief retraction and cancellation of its dependent live intent

Verify that the deliberate `belief_edge` deferral is documented.

Do not request implementation of belief_edge for this review.

============================================================
9. INSPECTION COMMAND
============================================================

Run:

    task inspect

Verify that it reports actual current database state.

It should expose meaningful:

    beliefs
    evidence
    intents
    audit state

Verify that it does not contain hard-coded demo results.

Run it after Track 2 and confirm that showing Track 2 state is expected
because Track 2 resets the database and runs last.

============================================================
10. NO HIDDEN NETWORK DEPENDENCY
============================================================

Inspect all demo scripts.

Verify the demos do NOT make live calls to:

    GitHub
    NVD
    OSV
    etcd APIs
    arbitrary external services

The pinned repository fixtures must be the canonical demo inputs.

Network access should only be necessary for:

    initial Docker image pull

and normal development operations, not for reconstructing the etcd
scenario.

============================================================
11. PROVENANCE / REAL DATA
============================================================

Inspect the actual etcd fixtures.

Verify the demo is really based on the pinned real records.

Verify:

- source URLs
- fixture hashes
- manifest
- attribution
- scenario separation

Verify the Track 1 and Track 2 URLs correspond to the intended etcd
release/advisory/postmortem material.

Do not accept fabricated or placeholder external evidence.

============================================================
12. HARDCODED OUTPUT SWEEP
============================================================

Repeat the final audit yourself.

Inspect:

    scripts/demo/track1.sh
    scripts/demo/replay.sh
    scripts/demo/track2.sh
    scripts/demo/inspect.sh

Search for:

    SAFE
    VIOLATION
    PASS
    LIVE
    CANCELLED
    RETRACTED
    PROMOTED
    counts
    "Replay verified"

Determine for every displayed result whether it is:

    DB-derived
    assertion-gated
    legitimate static explanatory text

The previous audit found the original hard-coded:

    Promoted 1

and replaced it with a DB query.

Verify that this remains fixed.

============================================================
13. CAPTURE-HARNESS CLEANUP
============================================================

Verify the final plan4 change.

Run:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Confirm normal judge-facing output does NOT contain:

    Transcript written to /dev/null

Inspect stderr behavior.

Ensure moving the message to stderr did NOT:

- hide real errors
- alter exit codes
- suppress useful diagnostics
- alter transcript generation

Also run:

    grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md

and inspect any remaining occurrences.

============================================================
14. TIME / DETERMINISM
============================================================

Verify that meaningful source timestamps such as:

    source_observed_at

remain tied to the real fixture.

Verify that:

    ingested_at

or other runtime-generated timestamps are not being used as deterministic
deduplication keys or replay comparison values.

============================================================
15. EXISTING CORE REGRESSION
============================================================

Verify:

    db/001_schema.sql
    internal/kernel/kernel.go
    internal/kernel/sql.go
    FullDebt
    belief_edge

remain unchanged in semantics.

No new tables.

No new belief_edge population.

No kernel bypass.

No direct database writes that circumvent existing kernel transaction
discipline.

============================================================
16. FULL TESTS / STATIC CHECKS
============================================================

Run:

    unset FABLE_TEST_DSN && task test

Verify:

    107 tests pass
    go build passes
    go vet passes
    gofmt passes
    scripts/check_i7.sh passes

Do not rely on the implementation report's claim.

Inspect actual output.

============================================================
17. README ACCURACY
============================================================

Compare README.md with actual behavior.

Verify:

- ports are correct
- prerequisites are correct
- setup instructions work
- Track 1 description is accurate
- replay description is accurate
- Track 2 description is accurate
- F1 limitation is accurately stated
- no command is documented that does not exist
- no command is omitted that is necessary for the advertised quick start

The README must not overclaim what Solvent demonstrates.

============================================================
18. SCOPE CONTROL
============================================================

Confirm that the playground did NOT introduce:

- new schema
- kernel redesign
- new belief semantics
- new debt semantics
- belief_edge implementation
- RAG
- embeddings
- vector DB
- Kafka
- Kubernetes
- web UI
- bchat
- MCP
- live data fetching
- second database technology

This is still the CLI/Taskfile reference playground.

============================================================
19. IMPLEMENTATION REPORT VS ACTUAL CODE
============================================================

Read:

    plans/post/playground_implementation_report.md

Compare every major claim against:

- actual files
- actual code
- actual command behavior
- actual database state

Pay particular attention to:

- fresh-machine claim
- cold-pull claim
- CRDB version
- test isolation
- replay
- Track 1
- Track 2
- inspect
- hard-coded output audit
- plan4 cleanup

If a report claim cannot be independently established, say so.

Do not treat a report as proof.

============================================================
20. REPORT TRANSCRIPT HONESTY
============================================================

Determine whether the command transcripts in the report correspond to
the claimed execution order.

Expected order:

    task db:down
    task setup
    task test
    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

It is acceptable that `inspect` shows Track 2 state because Track 2 ran
last.

If transcripts contain capture-harness artifacts, determine whether they
are harmless and whether the final judge-facing output is clean.

============================================================
SEVERITY
============================================================

Use:

BLOCKER
IMPORTANT
MINOR
NIT

BLOCKER:
A real correctness failure, reproducibility failure, security/integrity
problem, misleading demo behavior, or regression.

IMPORTANT:
Material issue that should be fixed before the hackathon demo.

MINOR:
Non-blocking correctness/documentation issue.

NIT:
Optional polish.

Do NOT inflate severity.

In particular:

- Do not call the lack of belief_edge propagation a defect.
- Do not demand new architecture.
- Do not reopen the research phase.
- Do not invent hypothetical failures without evidence.

============================================================
OUTPUT
============================================================

Produce:

    FINAL_PLAYGROUND_CODE_REVIEW.md

Use exactly:

# 1. Verdict

Choose:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

# 2. Executive Assessment

State whether the actual implementation matches the intended playground
contract.

# 3. Verification Matrix

| Area | Result | Evidence |
|------|--------|----------|
| Fresh setup | | |
| CRDB v26.2.0 | | |
| Test isolation | | |
| Track 1 | | |
| Replay | | |
| Track 2 | | |
| Inspect | | |
| Provenance | | |
| Output honesty | | |
| Plan4 cleanup | | |
| Core invariants | | |
| Tests/static checks | | |
| README | | |

# 4. Findings

For every finding:

    ID
    Severity
    File/location
    Evidence
    Why it matters
    Required correction

If none:

    No material findings.

# 5. End-to-End Receipts

Report actual results for:

    task setup
    task test
    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

# 6. Regression Check

Report:

    schema
    kernel
    FullDebt
    belief_edge
    ExecuteTx discipline
    existing tests

# 7. Report Accuracy

State whether:

    playground_implementation_report.md

accurately describes the actual implementation.

# 8. Final Recommendation

If APPROVED:

    Freeze the CLI/Taskfile playground.

If APPROVED WITH MINOR CORRECTIONS:

    List only the corrections and state explicitly that the architecture
    and implementation are approved.

If REWORK REQUIRED:

    List only concrete blockers.

============================================================
IMPORTANT FINAL RULE
============================================================

This is the FINAL CODE REVIEW.

Do not create another review loop for cosmetic preferences.

If the implementation is correct and the remaining issues are genuinely
minor, APPROVE it.

If you find a real blocker, demonstrate it with actual repository/code/
runtime evidence.

The standard is:

    Does the actual repository deliver the promised,
    reproducible, honest judge-facing Solvent playground?

If yes:

    APPROVED.

After approval, the Solvent CLI playground is considered frozen.

The next development phase is separate:

    Solvent MCP server
        ↓
    Claude / Codex / VS Code / OpenCode / other MCP clients

Do NOT review or implement that phase now.