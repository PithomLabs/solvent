Yes. **Fix the minor corrections before MCP.** They are tiny, directly improve the judge experience, and—most importantly—give us a clean final state to freeze rather than carrying known defects into the next layer.

Send the coding agent this:

```text
# FINAL PLAYGROUND POLISH — CLOSE THE REVIEW FINDINGS BEFORE MCP

Repository:
https://github.com/PithomLabs/solvent

The final independent adversarial code review is complete.

Verdict:

    APPROVED WITH MINOR CORRECTIONS

The Solvent core and playground architecture are approved and MUST remain
frozen.

Do NOT redesign anything.
Do NOT reopen the core.
Do NOT modify the schema.
Do NOT modify kernel semantics.
Do NOT modify replay/deduplication.
Do NOT implement MCP.
Do NOT add new features.

This is the final small cleanup pass before we freeze the CLI/Taskfile
playground and begin the MCP phase.

============================================================
APPROVED CORE — DO NOT TOUCH
============================================================

The independent reviewer verified:

- fresh Docker setup
- CRDB v26.2.0
- real etcd fixtures
- Track 1
- genuine replay idempotence
- Track 2
- single-belief retraction + intent cancellation
- 107 tests
- core/schema invariants
- belief_edge remains intentionally unused
- no material architecture defect

Do not disturb any of that.

In particular, DO NOT modify:

    db/001_schema.sql
    internal/kernel/*
    internal/derive/*
    internal/normalize/*
    internal/pipeline/*
    internal/intent/*
    cmd/solvent/main.go

The previous plan4 idea of changing stdout/stderr in cmd/solvent/main.go
is explicitly REJECTED.

The current correct solution for the `/dev/null` diagnostic was removing
the unnecessary `--out /dev/null` usage from the demo scripts.

Do NOT reintroduce a cmd/ change.

============================================================
FIX F-1 — TRACK 2 HEADER BUG
============================================================

File:

    scripts/demo/track2.sh

Current bug:

    tail -n2

is being used on TSV output containing a header row.

This causes the header itself to appear as a phantom belief, producing
output such as:

    Belief: claim
    Status: status
    Action: action
    Intent: intent_state

This is a real judge-facing defect.

Change the logic so it skips the header correctly.

Use:

    tail -n +2

or an equivalent minimal fix.

DO NOT alter the underlying query or database behavior.

After fixing, verify that the Track 2 "state before falsification" panel
shows only actual ledger rows.

============================================================
FIX F-4 — TRACK 1 LIVE INTENT COUNT
============================================================

File:

    scripts/demo/track1.sh

The displayed value is labelled:

    Live intents

but the current query counts intents without explicitly filtering state.

Fix the query so it actually counts live intents.

Use the existing schema/state representation.

The semantic requirement is:

    displayed "Live intents"
        =
    COUNT(intents WHERE state = 'live')

Do NOT rename the display to merely "Intents".

The point of the Track 1 panel is specifically to demonstrate that
promotion produced LIVE authority.

Do not change intent semantics.

============================================================
FIX F-9 — TRACK 2 NULL INTENT DISPLAY
============================================================

File:

    scripts/demo/track2.sh

Inspect the final Track 2 output.

If the intent lookup returns NULL because the intent has been cancelled
or no longer exists in the particular query path, do not print misleading
raw output such as:

    intent: NULL

Add the smallest safe guard so the panel remains human-readable.

IMPORTANT:

Do NOT hide a real invariant failure.

The guard should distinguish:

    legitimate absence / NULL state

from:

    unexpected database/query failure.

If the query itself fails, the script must still fail.

The goal is only to prevent a legitimate NULL result from looking like a
broken demo.

============================================================
FIX F-2 — README PREREQUISITES
============================================================

File:

    README.md

The quick-start path begins with:

    task setup

and the setup checks for Task.

Make the prerequisites explicitly state that the judge needs:

    Docker
    Go
    Task

Do not add unnecessary dependencies.

Ensure the documented Go requirement is derived/consistent with the
repository's actual go.mod requirement.

Keep the README concise.

============================================================
F-3 — DOCUMENT THE ACTUAL CAPTURE CLEANUP
============================================================

Do NOT modify code for F-3.

The final implementation correctly removed the unnecessary:

    --out /dev/null

from the demo scripts.

That eliminated the:

    Transcript written to /dev/null

diagnostic.

Do NOT apply the previously proposed:

    fmt.Printf → fmt.Fprintf(os.Stderr)

change to cmd/solvent/main.go.

The CLI is already correctly frozen.

Only reconcile the implementation report/plan wording so it accurately
describes what actually happened.

Use wording equivalent to:

    The capture diagnostic was eliminated by removing the unnecessary
    --out /dev/null argument from the demo scripts. No change to
    cmd/solvent/main.go was required.

Do not claim that stderr redirection was implemented.

============================================================
F-6 — REPORT PROVENANCE HONESTY
============================================================

Update the implementation report so the CRDB verification claim exactly
matches what was actually verified.

The exact tag:

    cockroachdb/cockroach:v26.2.0

was independently confirmed to resolve from the registry.

If the final review did not perform a second cold byte-download because
the image was cached, do not claim that it did.

Distinguish:

    registry/tag resolution verified

from:

    cold image download independently reproduced

Do not weaken the actual setup claim; just make the receipt precise.

============================================================
F-7 — DOCUMENT THE ObservedAt WIRING GAP
============================================================

This is documentation only.

DO NOT modify the kernel.

The current architecture has:

    real source date
        ↓
    fixture
        ↓
    normalization parses ObservedAt
        ↓
    kernel persistence currently does NOT thread it through
        ↓
    evidence.source_observed_at remains NULL

The existing schema already has the source_observed_at field.

Do NOT invent a new schema change.

Update the implementation report's limitations section to state clearly:

    Normalization currently parses ObservedAt from the real fixtures,
    but the current frozen kernel does not yet thread ObservedAt through
    the evidence persistence boundary, so source_observed_at remains
    unpopulated. Wiring this world-time value through the persistence
    boundary is deferred to the MCP phase.

Use the repository's actual terminology.

This is a known deferred capability, NOT a reason to modify the frozen
kernel now.

============================================================
F-5 — RECAPTURE THE REPORT RECEIPTS
============================================================

THIS MUST HAPPEN AFTER F-1, F-4, AND F-9 ARE FIXED.

The previous implementation report contains stale transcript material
from an earlier revision.

Do not edit the transcripts manually to make them look correct.

Actually rerun the current implementation and capture fresh output.

Use this exact sequence:

    task db:down

    task setup

    unset FABLE_TEST_DSN && task test

    task demo:track1

    task demo:replay

    task demo:track2

    task inspect

Do NOT reset between Track 1 and replay.

Track 2 may reset the database as designed.

The resulting report should explicitly identify the capture order.

The final inspect output should therefore show Track 2's final state.

That is expected.

============================================================
VERIFY TRACK 1
============================================================

Confirm actual output/state:

    3 beliefs
    3 evidence
    1 promoted belief
    1 live intent
    audit safe

The "Live intents" value must now come from:

    state = 'live'

Do not hard-code the value.

============================================================
VERIFY REPLAY
============================================================

Confirm:

    before:
        beliefs = 3
        evidence = 3
        intents = 1

    after:
        beliefs = 3
        evidence = 3
        intents = 1

The existing equality assertion must remain intact.

Do not reset before replay.

============================================================
VERIFY TRACK 2
============================================================

Confirm the "state before falsification" panel contains ONLY real rows.

There must be no phantom:

    claim / status / action / intent_state

header row.

Confirm:

    belief → promoted
    intent → live
    postmortem → processed
    belief → retracted
    intent → cancelled
    audit → safe

The output must remain honest about this being:

    single-belief retraction + cancellation of its dependent intent

Do not claim belief_edge propagation.

============================================================
VERIFY TESTS
============================================================

Run:

    unset FABLE_TEST_DSN && task test

Confirm:

    107 tests pass
    go build passes
    go vet passes
    gofmt passes
    scripts/check_i7.sh passes

Do not introduce any test-environment change.

============================================================
FINAL OUTPUT AUDIT
============================================================

Inspect:

    scripts/demo/track1.sh
    scripts/demo/replay.sh
    scripts/demo/track2.sh
    scripts/demo/inspect.sh

Verify:

- no hard-coded success counts
- no hard-coded status pretending to be DB state
- "Live intents" actually means live
- no phantom header row
- no misleading NULL display
- no "Transcript written to /dev/null"
- no swallowed errors
- no changed exit-code behavior

Run:

    grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md

and inspect any remaining matches.

============================================================
SCOPE CHECK
============================================================

The final diff MUST remain limited to:

    scripts/demo/*
    README.md
    plans/post/playground_implementation_report.md

or similarly narrow playground/documentation files if genuinely required.

There must be NO changes to:

    db/
    internal/
    cmd/
    proof/
    etcd fixture semantics

If you discover that one of these areas must change to fix a finding:

    STOP.

Do not make the change.
Report it instead.

============================================================
IMPLEMENTATION REPORT
============================================================

Update:

    plans/post/playground_implementation_report.md

Make it accurately reflect the FINAL implementation.

It must include:

1. Final files changed
2. Fresh acceptance command order
3. Fresh Track 1 receipt
4. Fresh replay receipt
5. Fresh Track 2 receipt
6. Fresh inspect receipt
7. 107-test receipt
8. CRDB v26.2.0 verification
9. F1 single-belief limitation
10. ObservedAt wiring limitation
11. Capture-diagnostic cleanup mechanism
12. Confirmation that schema/kernel/cmd core remain frozen

Do not leave stale transcript blocks from the previous implementation.

Do not manufacture output.

============================================================
FINAL STOP CONDITION
============================================================

After completing the fixes and fresh verification:

STOP.

Do NOT implement MCP.

Do NOT start another review.

Do NOT redesign the playground.

Do NOT add new features.

The next phase will be:

    SOLVENT MCP SERVER
        ↓
    Claude / Codex / VS Code / OpenCode
        ↓
    existing frozen Solvent core

The purpose of THIS task is to leave the CLI/Taskfile playground in a
clean, honest, reproducible, judge-ready state before that next phase.

Final response must summarize:

    - fixes made
    - tests run
    - fresh demo results
    - files changed
    - confirmation that core/schema/kernel/cmd remain untouched

Do not commit.
Do not push.
```
