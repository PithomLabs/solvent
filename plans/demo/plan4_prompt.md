Send this to the coding agent:

```text
# FINAL COSMETIC CLEANUP — REMOVE CAPTURE-HARNESS NOISE FROM JUDGE OUTPUT

Repository:
https://github.com/PithomLabs/solvent

The Solvent judge playground implementation is now substantively complete
and verified.

DO NOT reopen the architecture.
DO NOT modify the Solvent kernel.
DO NOT modify the schema.
DO NOT modify the etcd data pipeline.
DO NOT modify replay semantics.
DO NOT modify test infrastructure.
DO NOT redesign the Taskfile.
DO NOT add MCP yet.
DO NOT add new features.

This is one narrowly scoped final presentation cleanup before the final
independent code review.

============================================================
CONTEXT
============================================================

The latest playground implementation report has verified:

- fresh/torn-down Docker acceptance path
- cold pull of cockroachdb/cockroach:v26.2.0
- no non-default CRDB settings required
- task test is self-contained
- 107 tests pass
- Track 1 works
- Track 1 replay is genuine idempotence
- Track 2 works
- single-belief retraction + dependent intent cancellation works
- all displayed success values are DB-derived/assertion-gated
- schema/kernel/FullDebt/belief_edge remain frozen

The remaining issue identified in the final plan review is purely
judge-facing output noise.

The demo transcripts currently contain lines such as:

    Transcript written to /dev/null

These appear in the output because the demo scripts use a `script(1)`
capture/harness mechanism.

This is not a correctness problem.

It is undesirable in judge-facing output because it looks like an
implementation diagnostic and creates confusion:

    "Why is the demo writing a transcript to /dev/null?"

We want the actual demo output to communicate Solvent's behavior, not the
capture mechanism used to collect receipts.

============================================================
TASK
============================================================

Remove/suppress the:

    Transcript written to /dev/null

message(s)

from the judge-facing demo output.

The underlying capture/verification behavior MUST NOT be weakened or
removed merely to hide the message.

In particular:

- do not remove the actual execution
- do not remove assertions
- do not replace real command execution with canned output
- do not redirect the entire demo output to /dev/null
- do not suppress database errors
- do not suppress non-zero exits
- do not alter the actual pipeline
- do not change replay behavior
- do not change Track 1/Track 2 behavior

Only suppress the specific capture-harness diagnostic.

============================================================
1. INSPECT BEFORE MODIFYING
============================================================

Inspect:

- scripts/demo/track1.sh
- scripts/demo/replay.sh
- scripts/demo/track2.sh
- scripts/demo/inspect.sh
- scripts/demo/setup.sh
- scripts/demo/config.env
- Taskfile.yml

Determine exactly where:

    "Transcript written to /dev/null"

comes from.

Do not guess.

Determine whether it is:

- emitted by `script(1)`
- emitted by a shell wrapper
- emitted by a helper
- emitted by another command

Then suppress only that diagnostic.

============================================================
2. PRESERVE CAPTURE/EXECUTION SEMANTICS
============================================================

If the current pattern is something equivalent to:

    script -q -c "..." /dev/null

do NOT simply remove the command if that command is necessary for
the existing execution/PTY behavior.

Instead, use the smallest safe mechanism to suppress the specific
diagnostic while preserving:

- command execution
- stdout
- stderr where needed
- exit status
- `set -e` behavior
- database writes
- assertions
- transcript capture behavior if it is actually needed

If suppressing the message requires changing the capture method, prove
that the replacement preserves the command's exit code and output.

Do not introduce a new dependency.

============================================================
3. JUDGE-FACING OUTPUT REQUIREMENT
============================================================

After the change, these commands should not display:

    Transcript written to /dev/null

or equivalent capture-harness noise:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

The output should instead read naturally as a Solvent demo.

For example:

    --- Processing evidence ---

    --- Ledger state after pipeline ---

    ...

Do NOT over-polish the output in this task.

This is only removal of capture-harness noise.

============================================================
4. VERIFY THAT NO REAL ERRORS ARE HIDDEN
============================================================

This is critical.

Intentionally cause/verify at least one failure path if practical, or
inspect the shell execution semantics carefully, to ensure that the
suppression does NOT hide:

- command failures
- SQL failures
- failed assertions
- non-zero exit codes
- database connection errors

The demo must still fail loudly when an underlying operation fails.

In particular verify that replay's existing assertion remains intact:

    before counts == after counts

and that a mismatch still causes exit code 1.

Do not weaken this just to make output cleaner.

============================================================
5. RUN THE EXISTING ACCEPTANCE PATH
============================================================

After the cleanup, run the relevant demos using the existing verified
environment.

At minimum:

    task demo:track1
    task demo:replay
    task demo:track2
    task inspect

Do NOT reset between Track 1 and replay.

The required sequence is:

    task demo:track1
        ↓
    task demo:replay
        ↓
    task demo:track2
        ↓
    task inspect

Track 2 resets the database as designed.

Confirm:

### Track 1

- 3 beliefs
- 3 evidence
- 1 promoted belief
- 1 live intent
- audit = SAFE

### Replay

- before = 3 beliefs / 3 evidence / 1 intent
- after = 3 beliefs / 3 evidence / 1 intent
- replay assertion passes
- no reset occurs

### Track 2

- baseline belief promoted
- live intent created
- postmortem processed
- baseline belief retracted
- its live intent cancelled
- audit = SAFE

### Inspect

Shows Track 2 final state because Track 2 ran last.

============================================================
6. RUN TESTS
============================================================

Run:

    unset FABLE_TEST_DSN && task test

Confirm:

    107 tests pass
    go build passes
    go vet passes
    gofmt check passes
    check_i7.sh passes

Do not change the test environment.

`task test` must remain independent of the playground container setup
according to the verified test infrastructure.

============================================================
7. CHECK FOR UNINTENDED CHANGES
============================================================

After implementation, inspect the diff.

The expected change should be extremely small and confined to the demo
capture/output mechanism.

If you find yourself modifying:

- internal/kernel/*
- internal/belief/*
- internal/derive/*
- internal/normalize/*
- internal/pipeline/*
- internal/intent/*
- db/001_schema.sql
- etcd fixtures
- Taskfile semantics
- test infrastructure

STOP.

Those changes are outside this task.

============================================================
8. UPDATE IMPLEMENTATION REPORT
============================================================

Update:

    plans/post/playground_implementation_report.md

Add a brief note under the relevant implementation/verification section:

    Capture-harness diagnostic cleanup:
    Removed the "Transcript written to /dev/null" diagnostic from
    judge-facing demo output while preserving the underlying command
    execution, assertions, exit-code behavior, and verification path.

Do not rewrite the report unnecessarily.

Do not change previously verified claims except where the new command
outputs require updating.

If new command output is included, make clear that it is actual captured
output from the final run.

============================================================
9. FINAL OUTPUT AUDIT
============================================================

Run:

    grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md

The judge-facing scripts should contain no occurrence that would be
printed during normal execution.

Also inspect the output manually for accidental capture-harness noise.

Do NOT blindly grep-and-delete arbitrary `/dev/null` usage.

Only the diagnostic should disappear; legitimate redirections may remain.

============================================================
10. FINAL DELIVERABLE
============================================================

Report:

1. Where the diagnostic originated.
2. Exactly how it was suppressed.
3. Why command execution/exit-code behavior is preserved.
4. Actual Track 1 result.
5. Actual replay result.
6. Actual Track 2 result.
7. Actual inspect result.
8. Test result.
9. Exact files changed.
10. Confirmation that schema/kernel/test infrastructure were untouched.

Do NOT commit.
Do NOT publish.
Do NOT implement MCP.
Do NOT start another review.

============================================================
DEFINITION OF DONE
============================================================

This task is DONE when:

    "Transcript written to /dev/null"
        ↓
    absent from normal judge-facing demo output

while:

    real execution
    real database state
    real assertions
    real exit codes
    real replay idempotence
    real Track 1
    real Track 2
    107 tests

remain unchanged and passing.

This is the final cosmetic cleanup before the independent adversarial
CODE REVIEW.

After this task, STOP.
```
