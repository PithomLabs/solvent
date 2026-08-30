# FINAL RECEIPT / DOCUMENTATION REWORK AFTER FABLE VERIFICATION

Read these first:

1. review_fable.md — original Fable adversarial review
2. remediation_review.md — Fable's POST-FABLE REMEDIATION VERIFICATION
3. fix.md — the implementation summary you previously produced

The latest Fable verification is authoritative.

Its verdict is:

    REWORK REQUIRED

BUT:

    The code is verified.
    The acceptance gates are green.
    The falsification CLI is verified and safe.
    Waves 1–4 remain frozen.
    No kernel/schema/pipeline redesign is required.

Fable explicitly says the remaining work is an afternoon of
documentation/receipt work, not architectural rework.

DO NOT modify:

- kernel behavior
- schema
- belief logic
- intent logic
- pipeline semantics
- derive semantics
- normalize implementation
- Waves 1–4 architecture

DO NOT address N-4, N-7, or N-8 in this pass.

============================================================
REQUIRED FIX 1 — TEST COUNT: 84 → 85
============================================================

Fable actually executed the repository and found 85 passing tests:

    normalize 12
    derive 16
    kernel 27
    belief 18
    intent 3
    pipeline 9
    ----------------
    total 85

The F-13 regression test added one normalize test.

Update all stale counts identified by Fable:

README.md:
- Wave 1: 11 → 12
- total tests: 84 → 85
- every other stale 84/11 reference found by whole-file inspection

AGENTS.md:
- 84 → 85

PUBLIC_REPO_AUDIT.md:
- update the per-package table:
  normalize = 12
- update total = 85
- update any "84 tests" claims

fix.md:
- update its own verification table:
  "Total tests: 85, all passing"

IMPORTANT:

Search the entire repository for:

    84
    11 tests
    11
    "84 tests"

but do NOT blindly replace unrelated numbers.

Only correct references that actually describe the current test suite.

============================================================
REQUIRED FIX 2 — ENSUREBELIEF IN README KERNEL SURFACE
============================================================

Fable found README.md still describes the kernel public surface as seven
functions.

Update README.md so the public kernel surface includes all eight functions,
including:

    EnsureBelief

Use the actual signature/role from IMPLEMENTATION_CONTRACT.md.

Then search for stale claims such as:

    seven functions
    seven §4 functions
    7 functions

in public/current documentation.

Relevant stale reports identified by Fable:

    docs/M1_REPORT.md
    docs/M2_REPORT.md

Correct them too.

Do not change the implementation contract itself; it is already correct.

============================================================
REQUIRED FIX 3 — STALE M1/M2 REPORT COUNTS
============================================================

Fable found:

    docs/M2_REPORT.md

still says:

    25/25 cases
    25 rows

but the actual current M2 suite is:

    28/28
    28 rows

Change those stale claims.

Also fix:

    docs/M1_REPORT.md

where Fable found:

    8/8

and:

    "all seven §4 functions"

These must reflect:

    9/9
    eight §4 functions

Do a whole-file consistency check on both reports.

Do NOT invent new metrics.

============================================================
REQUIRED FIX 4 — REGENERATE M2 RECEIPTS CORRECTLY
============================================================

This is important.

Fable proved that the committed:

    docs/M2_TRANSCRIPT.md
    docs/M2_TEST.md

currently come from different runs.

The current M2 suite itself is healthy:

    28/28 twice

But the frozen pair does not represent one coherent run.

Fix this by:

1. Ensure the schema is applied/prepared as required.
2. Run:

       scripts/m2_accept.sh

3. Do NOT run a later plain `go test ./...` afterward, because F-10 causes
   the kernel suite to overwrite:

       docs/M2_TRANSCRIPT.md

4. The final committed pair must be the pair produced by the SAME final
   m2_accept.sh run.

5. Verify that the M2 transcript's B-21 observation matches the corresponding
   M2_TEST run 2 observation.

6. Confirm:

       M2_TRANSCRIPT.md = 28/28
       M2_TEST.md        = 28/28 / 27 top-level PASS per run

IMPORTANT:

Do not modify kernel suite behavior to solve F-10.

F-10 remains deferred.

The correct containment for this pass is:

    run m2_accept.sh LAST
    preserve its generated M2 pair
    do not run anything afterward that rewrites the transcript

============================================================
REQUIRED FIX 5 — README CLI EXAMPLES
============================================================

Fable found the F-6 configuration fix made the README examples stale.

The CLI now correctly requires a DSN.

Therefore update BOTH README CLI examples.

The documentation must show either:

    FABLE_DSN=<dsn> ...

or:

    --dsn <dsn>

Use the simplest clear example consistent with the actual CLI.

Also explain that:

    --fixtures

is needed when invoking the CLI from a working directory where the default
fixture resolution does not apply.

Do NOT restore a silent fable_test default.

The F-6 behavior is correct and must remain.

============================================================
REQUIRED FIX 6 — SOFTEN THE CLI "CASCADE" CLAIM
============================================================

Fable found an important demo/documentation distinction.

The kernel's RetractCascade is real and tested.

However, the shipped pipeline does not currently create belief_edge rows,
so the CLI's current real-data demonstration retracts:

    one belief
    +
    its live intent

rather than demonstrating a multi-generation belief_edge cascade.

Do NOT add belief_edge generation now.

Do NOT redesign the pipeline.

Simply make README.md truthful.

Change wording such as:

    "atomically retract a belief and its cascade"

to wording that accurately describes what the shipped CLI demonstrates.

For example, it may say that the CLI:

    atomically retracts a belief and cancels its live dependent intent

Then, if appropriate, point readers to the M2 B-07/B-20/B-22 cases as the
evidence that multi-generation RetractCascade itself is implemented and
tested.

The goal is:

    CLI demo = truthful
    kernel capability = still documented
    no new architecture

============================================================
OPTIONAL — N-5
============================================================

Fable recommends a small honesty improvement:

If:

    retracted 0 belief(s)

then do not print:

    VERDICT: PASS

because a no-op falsification is not actually a successful falsification.

Preferred:

    VERDICT: NO-OP

while retaining the existing audit check.

This is a tiny judge-facing correctness improvement.

Implement it only if it can be done cleanly without touching kernel
semantics.

If implemented, add/update the CLI test.

============================================================
RECOMMENDED — README EMBEDDING LEFTOVERS
============================================================

Fable also found that although the main false vector claim was corrected,
README still contains leftover references at approximately:

    :34
    :72
    :148

describing embedding columns/vector-similarity dedup.

These contradict the current design:

    NO embedding/vector column.

Remove or rewrite those references so the README is internally consistent.

Do not add an embedding system.

============================================================
OPTIONAL — M0 RECEIPT TIMINGS
============================================================

Fable found:

    docs/M0_TRANSCRIPT.md

was regenerated with:

    --no-timings

which removed the elapsed_ms column.

This is not a correctness failure.

If convenient, regenerate the committed M0 receipt WITH timings, while
using --no-timings only for the two-run determinism comparison.

Do this only if it does not complicate the final verification.

============================================================
DO NOT DO THESE
============================================================

Explicitly DO NOT fix:

N-4:
- cliReset drops the DB before schema resolution

N-7:
- duplicated parsePGArray test helper

N-8:
- assets-less release fallback URL

Also do NOT reopen:

F-10
F-11
F-12
F-14
F-15
F-16
F-17
F-18
F-19
F-20

Fable verified these are unchanged and non-regressing.

============================================================
FINAL VERIFICATION
============================================================

After the documentation/receipt changes:

1. Search for stale counts:

    rg '84|25/25|25 rows|seven.*function|seven §4' \
       README.md AGENTS.md docs/ PUBLIC_REPO_AUDIT.md fix.md

Inspect every result manually; do not blindly replace unrelated numbers.

2. Verify:

    go build ./...
    go vet ./...
    gofmt -l cmd internal
    go test -count=1 -p 1 ./...

3. Run:

    scripts/check_i7.sh
    scripts/m1_accept.sh

4. Run:

    scripts/m2_accept.sh

THIS MUST BE THE LAST COMMAND THAT CAN MODIFY
docs/M2_TRANSCRIPT.md OR docs/M2_TEST.md.

After m2_accept.sh completes:

    DO NOT run go test ./...
    DO NOT run kernel tests
    DO NOT run another command that rewrites M2 receipts.

5. Verify the final committed M2 pair corresponds to the same run.

6. Verify final test count = 85.

7. Verify README CLI examples include DSN.

8. Verify README says four tables and no embedding/vector column.

9. Verify README kernel surface includes EnsureBelief.

10. Verify M1/M2 reports use 9/9, 8 functions, and 28/28.

11. Verify git status and diff carefully.

============================================================
DELIVERABLE
============================================================

Update/create:

    POST_FABLE_REMEDIATION_FINAL.md

Summarize:

- F-3 documentation completion
- F-5/F-8 receipt completion
- F-23 README completion
- N-1 resolved
- N-2 resolved
- N-3 resolved
- N-5 if implemented
- N-6 resolved
- embedding leftovers resolved
- final test count
- final M1/M2 results
- exact final M2 receipt relationship
- deferred findings unchanged

Do NOT commit to GitHub.

Do NOT push.

STOP after the final verification.

Wait for Technical Lead review.