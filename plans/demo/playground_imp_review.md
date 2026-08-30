**This implementation report is strong, and I would now treat the CLI playground as complete pending one final independent verification.**

The receipts are substantially better than the plan claims:

* Fresh/torn-down acceptance run is explicitly reported. 
* CRDB is pinned to the exact verified `v26.2.0`; no cluster settings are needed because of the recursive-CTE implementation. 
* Replay is genuinely meaningful: before/after **beliefs, evidence, and intents are all 3/3/1**, with failure on inequality. 
* Track 1 produces a real DB-derived `Promoted 1` and `Live intents 1`. 
* Track 2 correctly presents **single-belief** retraction and cancelled intent without pretending `belief_edge` propagation exists. 
* The hard-coded-output sweep is documented and shows the formerly hard-coded promotion count was the isolated issue. 
* The 107-test baseline and static checks pass. 
* Schema, kernel, `FullDebt`, and `belief_edge` remain frozen. 

### One thing I would *not* do yet

Don't declare it finally closed solely from this report. The report itself contains the exact **final adversarial review prompt** we asked for. 

So now is the right time to give that prompt to **Claude/Fable once**, because this is no longer a plan review—it can independently verify the actual implementation and receipts.

Ask Fable to focus particularly on:

1. Fresh `task setup` from torn-down state.
2. Actual availability/pull of `cockroachdb/cockroach:v26.2.0`.
3. `track1 → replay` without reset.
4. Actual before/after DB counts.
5. Track 2 retraction behavior.
6. No fabricated output.
7. 107-test baseline.
8. No schema/kernel changes.

**No more coding-agent iteration unless Fable finds a concrete failure.**

If Fable says **APPROVED**, I would officially close:

```text
SOLVENT CORE + ETCD + PLAYGROUND
                    ✅
```

and move to the next layer we discussed: **the MCP server as the agent-native interface**. The existing CLI/Taskfile remains the deterministic fallback and reference implementation.


## Claude

This report shows its receipts, which is what a report is for — real command transcripts, the grep table with sources, the count-equality gate, the F1 limitation stated twice. Most of it verifies. But a report's job is precisely to be read against the *plan that produced it*, and when I do that, **one deliverable silently changed the thing the previous review just fixed**, and **two claimed transcripts have internal tells that they weren't produced the way the report implies**. Ordered by consequence.

**R1 — BLOCKING regression, and it's exactly the class this whole chain exists to prevent: the report reintroduced the substrate discrepancy C2 removed, in a different file.** C2's entire point was that the judge's database byte-matches the verified substrate: local binary v26.2.0, so pin image v26.2.0. The report honors that in `setup.sh` — good — but the Files-Changed table adds a *new* change nobody reviewed: `Taskfile.yml` now sets `FABLE_TEST_DSN` to point the 107 tests at *the Docker container on 26261*. Plan3/F5 established the opposite: the tests use a `fable_test` database and **do not require the demo container**. So the report has re-pointed the verification suite at the demo substrate. That means the "107 tests pass" receipt was produced against the *container*, not against the environment the baseline was originally verified on — and if a judge runs `task test` without `task setup` having started the container, the suite now fails on a missing DSN target where before it was self-contained. This is a scope change smuggled in as a convenience: it wasn't in plan3, it contradicts F5's finding, and it weakens the one claim ("107 green") that's supposed to be environment-independent. Resolve explicitly: either revert `task test` to its self-contained `fable_test` path (F5's verified state, preferred), or if there's a real reason the tests now need a live cluster, that's a **STOP-and-report** per the escalation rule, not a table row. What's not acceptable is a reviewed baseline quietly acquiring a new runtime dependency between plan and report.

**R2 — MAJOR provenance tell: the "torn-down state" claim is contradicted by the report's own artifacts.** The summary asserts the acceptance run was performed from a fresh torn-down state — which is the exact condition my last review insisted on. But two things in the report are inconsistent with a single clean run. First, the Track 1 transcript still prints `Promoted 1` as a literal-looking value, yet the grep table says line 89 was *changed to a DB query*; if the fix was applied before the acceptance run, the transcript is consistent, but nothing in the report proves the transcript post-dates the fix rather than being carried over from the prior pass. Second, and harder: **`task inspect` shows Track 2's ledger state** (retracted v3.5.0 belief, cancelled deploy intent), not Track 1's. That's only correct if the acceptance sequence ran track1 → replay → track2 → inspect, with inspect reflecting track2 — which is a legitimate ordering. But the report presents the Track 1 block, the replay block, *and* an inspect block that can't coexist with the replay block's state (replay shows 3 beliefs/3 evidence/1 intent in scenario 1; inspect shows the scenario-2 retraction world). So the transcripts are from *different points in the sequence*, which is fine — but the report doesn't say so, and a reader could reasonably think `inspect` shows the state left by `replay`. State the exact command order the transcripts were captured at, and confirm each block is a real capture from that run rather than an assembled illustration. The anti-fabrication posture applies to the report itself: if these are representative rather than captured-in-one-run, say "representative."

**R3 — MAJOR omission: the one residual my last review flagged is silently marked resolved without its receipt.** plan3_review's single real residual was: verify the `v26.2.0` Docker tag is actually *pullable* before depending on it, because a local binary version doesn't guarantee a published tag. The report asserts the image is pinned and setup works, but shows **no `docker pull` / `docker manifest inspect` receipt** for that exact tag — the CRDB-setup section describes the container as if it started, but the acceptance summary is a claim, not a transcript, for the pull step specifically. Given R2's ambiguity about whether the run was truly fresh, this matters more, not less: a warm machine with the image already cached would pull-succeed while a judge's cold machine 404s on a nonexistent tag. Add the one-line receipt: the output of `docker manifest inspect cockroachdb/cockroach:v26.2.0` (or the pull log) proving the tag resolves on a machine that didn't already have it.

**R4 — MINOR consistency, worth one line: `go.mod` says go 1.25.0; earlier in the thread README/go.mod drift was a documented hazard.** The Prerequisites section states Go 1.25.0. That's internally fine, but the bchat audit found exactly this kind of version-string drift (README claiming one Go version, go.mod another), so confirm the Task prereq check validates against `go.mod`'s actual directive rather than a hard-coded "1.25.0" string — otherwise a future `go.mod` bump silently desyncs the check. Cheap to verify, and it's the same discipline C2 applied to the CRDB version.

Everything else holds and I'll name it so the residuals aren't mistaken for a global doubt: F1 is stated verbatim in README and report plus the dormant-by-design aside (correct and complete); the grep table is the real sweep with per-line sources, and it correctly shows only track1:89 was hard-coded while `Audit`, `Replay verified`, and the Track 2 block are DB-derived or assertion-gated (C3 discharged); the replay block shows genuine before/after count equality with an exit-1 gate, and the determinism precondition (`Derive()` pure, `ContentSHA256` timestamp-free) is cited (F3/C4 discharged); the settings-absence is stated as a property with its D-033 reason rather than a gap (C1 discharged); F6 is honored (scripts read `source_url`/`provenance_class`, not `ingested_at`); and the schema/kernel/`FullDebt`/`belief_edge` freeze is confirmed file by file. The reviewer prompt at the end is properly updated to the v26.2.0 and DSN specifics.

Net: the playground is one revert and two receipts away from done. Fix R1 (revert `task test` to its self-contained `fable_test` path, or escalate the new dependency — do not ship a baseline that quietly needs the demo container), disambiguate R2 (state the capture order and mark any illustrative block as such), and supply R3's pull receipt (prove the exact tag resolves cold). R4 is a one-line hardening. The one I'd least tolerate is R1, because "our 107-test baseline now silently depends on the demo container" is precisely the kind of unreviewed substrate change that C2 was raised to kill — and it came back in the very next artifact, which is the signal worth heeding: at this stage the risk isn't the design, it's small conveniences eroding the guarantees the design already earned.


## chatgpt


I agree with Claude's plan review. **Do not proceed to final code review yet.** There are three substantive corrections to make first, with R4 as a small hardening item.

### The most important one is R1

I agree this is a **real regression**.

The report explicitly says:

> `Taskfile.yml | Added FABLE_TEST_DSN env var ... points tests at Docker container port 26261` 

But the plan had already established that the 107 tests use the independent `fable_test` database and **do not require the demo container**. So this was an unreviewed change to the verification substrate.

**Revert it.**

`task test` should remain self-contained against the project's existing test database mechanism. The playground's Docker CRDB is for:

```text
task setup
task demo:*
task inspect
```

not for silently redefining the project's test environment.

This is especially important because the report currently claims:

> "107 tests + static checks"

while also changing what those tests run against. 

---

### R2 is also valid

The report needs to distinguish **actual captured command output** from assembled/representative output.

The Track 1 and replay outputs describe one state; `inspect` shows the later Track 2 state. That's perfectly legitimate given the intended sequence:

```text
task demo:track1
      ↓
task demo:replay
      ↓
task demo:track2
      ↓
task inspect
```

But the report needs to explicitly say that.

I'd require the coding agent to record:

```text
Acceptance command order:
1. task db:down
2. task setup
3. task test
4. task demo:track1
5. task demo:replay
6. task demo:track2
7. task inspect
```

And explicitly state:

> The transcript blocks are captured outputs from the corresponding points in this sequence, not a single database snapshot.

If they weren't actually captured from that run, label them **representative** instead of implying otherwise.

---

### R3 should definitely be closed

Claude is right that:

> `v26.2.0` being the locally verified version does not itself prove the Docker tag exists.

The implementation report currently asserts the exact image and fresh setup, but does not provide the requested tag-resolution receipt. 

Have the agent run:

```bash
docker manifest inspect cockroachdb/cockroach:v26.2.0
```

Preferably with the image absent locally, so we're proving **registry resolution**, not merely that an already-cached image exists.

---

### R4: do it, but don't overthink it

Make the Go prerequisite check derive the expected version from `go.mod` rather than hard-coding `1.25.0`.

The report currently says:

> Go (compatible with `go.mod`: go 1.25.0) 

That's fine today, but deriving it avoids another version-drift trap.

---

## Verdict

**Claude's review is correct: REWORK REQUIRED, but narrowly.**

```text
R1  Revert FABLE_TEST_DSN test redirection     🔴 MUST
R2  Document exact transcript capture order    🟠 MUST
R3  Prove v26.2.0 Docker tag resolves cold     🟠 MUST
R4  Derive Go version from go.mod               🟢 HARDENING
```

Do **not** redesign the playground.

Do **not** send it back for another plan review.

Give the coding agent these four corrections, have it rerun the fresh acceptance sequence, and produce an updated implementation report with actual receipts.

Then **Claude gets the actual code review**, which should be the final gate.

The fact that R1 slipped in after we explicitly froze the test substrate is exactly why this final adversarial pass is useful. The architecture is fine; we're now policing **implementation drift and evidence quality**.
