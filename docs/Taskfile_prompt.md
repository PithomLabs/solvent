# TASK — VERIFY `task demo` IS ACCURATE AND JUDGE-SAFE (Solvent)

You are working in the existing Solvent repository. The ledger core and the CLI
playground are COMPLETE and VERIFIED (107 tests green, schema/kernel frozen). Do
NOT redesign anything. This is a **verification** task with a small, bounded set
of fixes permitted only where a check fails.

## Prime directive

Confirm each claim below by INSPECTING the real script and RUNNING the real
command, then pasting the actual output. Never infer a script's behaviour from
`Taskfile.yml` — open the script. A finding (something that does not hold) is a
SUCCESS of this task, not a failure: report it precisely with file:line and
captured output. Do NOT weaken a check, hide an error, or edit a frozen file to
make a check pass. If a fix is needed and it is outside the explicitly-permitted
list in §7, STOP and report instead of expanding scope.

Frozen (never edit): `db/001_schema.sql`, `internal/kernel/**`, `cmd/**`,
`FullDebt`, and do not populate `belief_edge`.

---

## Background / rationale (why each check exists)

Solvent is a transactional belief ledger; the judge demo tells its story through a
Taskfile. A reviewer verified the *individual* subtasks (`track1`, `replay`,
`track2`, `inspect`) end-to-end. But the aggregate one-command path — `task demo` —
and the interaction between subtasks were reviewed only from the Taskfile, not by
running the composite. Three specific risks were identified from the Taskfile alone
and could not be closed without the scripts. This task closes them.

- **R-A (idempotence integrity).** The project's replay claim is a strong one:
  re-ingesting the SAME evidence into an ALREADY-POPULATED ledger creates zero new
  rows (idempotence), which is stronger than "reset then reproduce" (determinism).
  This only holds if `demo:replay` runs on `track1`'s populated state with NO reset
  between them. But `track1.sh` resets the DB itself (for deterministic output), and
  the `demo` task also issues its own `db:reset`. So the composite ordering
  (`db:reset → track1 → replay`) may or may not actually exercise idempotence,
  depending on what `track1.sh` and `replay.sh` each do internally. If replay is
  really running after a reset, the "no duplicates" message proves determinism, not
  idempotence — a weaker claim than advertised. We must know which.

- **R-B (destructive-reset safety).** Every reset is `DROP DATABASE IF EXISTS fable
  CASCADE`. `task demo` fires it twice, and several other tasks reset too. This is
  safe ONLY because it is scoped to the throwaway `solvent-crdb` container. The
  moment a DSN points elsewhere, or an unrelated `fable` database exists in that
  container, `CASCADE` deletes it with no confirmation. For a judge running once on
  a fresh container this is fine; as a repeatable command it is a loaded gun. We
  want the reset tasks to announce their target before dropping, so no one is
  surprised.

- **R-C (setup idempotence + coupling).** `task demo` has `deps: [setup]`, and Task
  `deps` run every invocation. If `setup.sh` is not idempotent (e.g. `docker run` a
  fresh container rather than `docker start` an existing one, with no `status:`
  guard), a second `task demo` may fail on "container name already in use" or
  silently recreate state. Also, coupling "run the demo" to "mutate my Docker host
  every time" is the wrong contract for a repeatable command.

- **R-D (judge-facing cleanliness).** The reviewer found two output defects in
  `track2.sh`: F-1, a TSV **header row rendered as a phantom belief** (`tail -n2`
  instead of `tail -n +2`) in the climactic "state before falsification" panel; and
  F-9, `intent: NULL` printed for beliefs with no intent. If those fixes did not
  land in the `track2.sh` this Taskfile calls, `task demo` will display them to a
  judge. The orchestration cannot fix a script bug — the script must.

The goal is a clear-cut, honest demo: one that either runs cleanly as `task demo`,
or whose correct judge path is documented explicitly because the aggregate has a
caveat. Both outcomes are acceptable; a demo that *looks* clean while quietly
proving the weaker claim is not.

---

## §1 — Inventory first (no changes)

Open and read, then state their reset/ingest behaviour in one line each, quoting the
decisive lines with file:line:
`scripts/demo/setup.sh`, `scripts/demo/track1.sh`, `scripts/demo/replay.sh`,
`scripts/demo/track2.sh`, `scripts/demo/inspect.sh`, `scripts/demo/config.env`.

For each of `track1.sh` and `replay.sh` answer explicitly:
- Does it reset/drop/recreate the DB itself? (quote the line, or state "no reset")
- Does it run `cmd/solvent`/`cmd/operator-review`, or raw `cockroach sql`?
- What exactly does it INSERT vs SELECT?

## §2 — R-A: does `demo` actually test idempotence?

Trace the `demo` task's real effect by running each step and querying row counts
between steps. Capture `SELECT count(*)` for belief, evidence, action_intent
(scenario-scoped) at these points:

1. after the leading `db:reset`
2. after `demo:track1`
3. after `demo:replay`

Then answer, with the captured numbers:
- Did `demo:replay` run on `track1`'s populated state, or on a reset DB?
- Are counts after replay EQUAL to counts after track1 (idempotence), or did replay
  repopulate from empty (determinism only)?
- Independently, run the reviewer's verified path — `task demo:track1` then
  `task demo:replay` as two separate commands with NO reset between — and capture the
  same counts. Do the two paths agree?

If the aggregate `demo` task does NOT exercise idempotence (because a reset lands
between track1 and replay), that is finding **F-A**: report it and recommend the fix
in §7 (do not silently rewrite the sequence beyond §7's bound).

## §3 — R-C: is `setup` idempotent and safe to run as a dep?

- Read `setup.sh`: does it `docker run` (create) or `docker start` (reuse)? Is there
  a guard for an already-running container?
- Run `task setup` twice in a row from a state where the container already exists.
  Capture both exits and outputs. Does the second run succeed cleanly, fail, or
  recreate state?
- Report whether `demo`'s `deps: [setup]` is safe to re-invoke. If not, that is
  finding **F-C**.

## §4 — R-D: are the judge-facing outputs clean?

Run `task demo:track2` and paste the FULL captured output. Specifically verify:
- The "state before falsification" panel shows exactly ONE belief, not a phantom
  header-row belief with `status: status` (F-1). Quote `track2.sh`'s relevant
  `tail` line with file:line.
- No line prints `intent: NULL` (F-9). Quote the guard line.
If either defect is present, report **F-D** and apply the §7-permitted one-line fix.

## §5 — R-B: do resets announce their target?

- Confirm every reset path (`db:reset` in `Taskfile.yml`, and any reset inside a
  script) is scoped to the `solvent-crdb` container / the demo `fable` DB, and that
  the test suite's `fable_test` DB is never reachable by a demo reset (quote the
  `_test` separation).
- Note whether any reset echoes the DSN/database it is about to DROP before dropping.
  If not, that is finding **F-B** (low severity) — recommend the §7 echo addition.

## §6 — Regression guard (must stay true)

Confirm and paste evidence:
- `git diff --stat HEAD -- db/ internal/ cmd/` is EMPTY after all your runs.
- `SELECT count(*) FROM belief_edge` = 0 after the full sequence.
- `unset FABLE_TEST_DSN && task test` — capture the pass/fail and the test count.
  (Note: this needs the container running; if it fails only on a stopped container,
  say so — that is expected, not a regression.)
- `task inspect` output matches the actual post-Track-2 DB rows (spot-check one
  belief, one evidence, one intent by ID).

## §7 — Permitted fixes (and ONLY these)

Apply a fix ONLY for a check that failed, and ONLY from this list. Anything else →
STOP and report.

- **F-D / F-1:** in `scripts/demo/track2.sh`, `tail -n2` → `tail -n +2` on the
  offending display line. One character.
- **F-D / F-9:** in `scripts/demo/track2.sh`, strengthen the intent guard to also
  reject the literal string `NULL`, e.g. `[ -n "$intent" ] && [ "$intent" != "NULL" ]`.
- **F-B:** add an `echo` to the reset path stating the DSN/DB about to be dropped,
  before the `DROP`. Presentation only; no behavioural change.
- **F-A:** if the aggregate `demo` task does not exercise idempotence, do the MINIMAL
  correction that makes the sequence honest — either (a) reorder so `replay`
  immediately follows `track1` with no intervening reset AND confirm `replay.sh`
  itself does not reset, or (b) if the scripts' own resets make a clean composite
  impossible, leave `demo` as-is and instead document in README that the idempotence
  claim is demonstrated by running `demo:track1` then `demo:replay` as separate
  steps. Pick whichever is truthful with the LEAST change. Do NOT edit script
  internals beyond what (a) strictly requires.
- **F-C:** if `setup` is not idempotent as a dep, the MINIMAL fix is to change
  `demo`'s dependency from `setup` to `db:up` (with a one-line note that `setup`
  is the one-time bootstrap), NOT to rewrite `setup.sh`.

Every fix must be one that does not touch a frozen path (§ top). If a needed fix
would touch `db/`, `internal/`, or `cmd/`, STOP and report — it is out of scope.

## §8 — Deliverable

Write `plans/post/demo_task_verification.md` containing:

1. The §1 inventory (one line per script + the decisive quotes).
2. F-A result: the captured counts at all three points, both the aggregate path and
   the two-command path, and the verdict — does `task demo` prove idempotence or only
   determinism?
3. F-C result: two consecutive `task setup` runs, exits, verdict.
4. F-D result: full `task demo:track2` output; F-1 and F-9 present or absent.
5. F-B result: reset scoping confirmation; whether targets are announced.
6. §6 regression evidence (git diff empty, belief_edge 0, test count, inspect match).
7. Fixes applied (from §7 only), each with the one-line diff and the failed check it
   addresses. If none were needed, say so.
8. FINAL VERDICT, one of:
   - "`task demo` is accurate and judge-safe as-is" — with the evidence, or
   - "`task demo` is judge-safe after the listed §7 fixes" — with before/after, or
   - "`task demo` should not be the judge path; the verified path is the explicit
     sequence, now documented in README" — with the reason.
9. The exact command sequence you recommend a judge run, copy-pasteable, with a
   one-line note on whether to prefer `task demo` or the explicit subtask sequence.

## Execution rule

Inspect, then run, then report. Real scripts, real commands, real output. Do not
compensate for a failing check by weakening it. Do not commit or publish. If any
required fix falls outside §7, STOP and report the blocker rather than expanding
scope.
