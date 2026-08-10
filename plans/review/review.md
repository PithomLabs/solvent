# Final Adversarial Code Review — Solvent Judge Playground

**Reviewed:** working tree at `HEAD = 9c0bd4a`, `github.com/PithomLabs/solvent`
**Prompt:** `plans/review/prompt.md`
**Date:** 2026-08-11
**Method:** independent inspection + full end-to-end execution on this machine. Every claim below is
backed by a captured command output or a cited `file:line`. Nothing is accepted from
`plans/post/playground_implementation_report.md` on its own authority.

---

# 1. Verdict

**APPROVED WITH MINOR CORRECTIONS**

No blockers. The playground is reproducible, its numbers are DB-derived, its evidence is real, and
the verified Solvent core is untouched. One judge-facing output defect (F-1) should be fixed before
the demo; the rest are documentation and labeling corrections.

---

# 2. Executive Assessment

The actual implementation matches the intended playground contract.

`task setup → task demo:track1 → task demo:replay → task demo:track2 → task inspect` was executed in
that order against a **freshly created container** and produced exactly the promised semantics:

- **Track 1** landed 3 beliefs / 3 evidence / 1 promoted / 1 **live** intent, `audit_live_on_nonpromoted = 0`.
  I re-queried every one of those numbers directly against CockroachDB and they match what the script
  printed. The counts come from `SELECT count(*)` queries, not from constants.
- **Replay** is genuinely idempotent, and it is stronger than the report claims. I snapshotted
  beliefs, evidence and intents **at row level including primary keys, `content_sha256`, and
  `ingested_at`** before and after the replay. All three tables came back **byte-identical**, which
  proves rows were *found*, not re-inserted and not overwritten. `replay.sh` contains no reset,
  truncate, drop, or re-init path, and its equality check is a real gate that exits 1 on mismatch.
- **Track 2** performed the whole arc: postulated baseline belief → evidence → six debts retired →
  promotion → live intent → postmortem evidence → falsification → belief `retracted` → dependent
  intent `cancelled`, with `audit_live_on_nonpromoted = 0` both scenario-scoped and cluster-wide.
  `cascade: retracted 1 belief(s)` — an honest single-belief count, exactly the claim the project makes.
- **`task inspect`** is 100% DB-derived with no baked-in demo results.

The core is provably unregressed: `git diff HEAD -- db/ internal/ cmd/ proof/` is **empty**. Not one
line of schema, kernel, or CLI source changed for the playground. `belief_edge` has 0 rows.
`unset FABLE_TEST_DSN && task test` passes with **107 passing tests, 0 failures, 0 skips**, plus
`go build`, `go vet`, `gofmt`, and `check_i7.sh` (7 `ExecuteTx` write sites, 0 raw writes).

Provenance is real, not placeholder. All five fixtures hash-match `manifest.json`, and I fetched
`GHSA-q8m4-xhhv-38mg` from GitHub live: it exists, is titled *"etcd: Authorization bypasses in
multiple APIs"*, High severity, published 2026-03-20, `go.etcd.io/etcd/v3`, fixed in 3.5.28 — matching
the fixture field for field.

The absence of `belief_edge` propagation is treated here as the deliberate, documented MVP boundary
it is, not a defect.

---

# 3. Verification Matrix

| Area | Result | Evidence |
|------|--------|----------|
| Fresh setup | **PASS** | `docker rm solvent-crdb` → `task setup` exit 0; container recreated from `cockroachdb/cockroach:v26.2.0`, readiness poll, `fable` created, `db/001_schema.sql` applied (4 tables present), binaries built |
| CRDB v26.2.0 | **PASS** (cold pull: partial) | `docker manifest inspect cockroachdb/cockroach:v26.2.0` resolved live from the registry (OCI index, amd64 digest `sha256:30a70bfd…`); container reports `CockroachDB CCL v26.2.0 … built 2026/04/21`; ports `26260→26257`, `8081→8080`. Byte-level re-download **not** re-performed — see F-6 |
| Test isolation | **PASS** | `internal/testdb/testdb.go:22` `DefaultDSN` is byte-identical at `HEAD` (last touched `021b36f`, 2026-08-08); no `FABLE_TEST_DSN` in `Taskfile.yml`; tests target `fable_test`, demos target `fable`; `git diff HEAD -- internal/` empty |
| Track 1 | **PASS** | `task demo:track1` exit 0; independent SQL: beliefs=3, evidence=3, promoted=1, intents_total=1, **intents_live=1**, audit=0, derived beliefs=1 |
| Replay | **PASS** | `task demo:replay` exit 0, no reset between runs; my own row-level snapshots `diff`-identical for beliefs, evidence (incl. `content_sha256`, `ingested_at`) and intents; belief IDs `e74d38c4…`, `2b7c9cd8…`, `a3d1f44c…` preserved across both runs |
| Track 2 | **PASS** | `task demo:track2` exit 0; SQL: postulated belief `retracted`, its intent `cancelled`, `belief_edge` = 0 rows, `retracted` count = 1, scenario-scoped audit = 0, global audit = 0 |
| Inspect | **PASS** | `task inspect` exit 0; output matches the post-Track-2 DB row for row; every field from a `SELECT`, no constants |
| Provenance | **PASS** | All 5 fixture SHA-256 match `manifest.json`; `track2.sh:12` `V350_SHA` matches `track2/release_v350.json`; GHSA-q8m4-xhhv-38mg verified live against GitHub; v3.5.0 commit `43b9aa5f…`/2021-06-15 and the v3.5.0–v3.5.2 data-inconsistency postmortem match public etcd history |
| Output honesty | **PASS with F-1, F-4** | Every `SAFE`/`PASS`/`Replay verified` string sits inside a branch gated on a DB-derived value (`inspect.sh:63`, `track1.sh:97`, `track2.sh:101`, `replay.sh:68`); `Promoted` is `SELECT count(*) … status='promoted'` (`track1.sh:86-90`) — the old hard-coded `1` is gone and stays gone |
| Plan4 cleanup | **PASS (different mechanism than plan4 specified)** | `grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md Taskfile.yml` → no matches; none of the 4 captured demo outputs contain it. Achieved by dropping `--out` entirely, **not** by the `Fprintf(os.Stderr, …)` edit plan4 recommended — `cmd/solvent/main.go:158` is still `fmt.Printf`. See F-3 |
| Core invariants | **PASS** | `git diff --stat HEAD -- db/ internal/ cmd/ proof/` empty; 4 tables only; `FullDebt` unchanged; `belief_edge` 0 rows; `check_i7.sh` PASS; no `SET CLUSTER SETTING` in any shipped file; `RetractCascade` uses `WITH RECURSIVE` (`internal/kernel/sql.go:67`) |
| Tests/static checks | **PASS** | `unset FABLE_TEST_DSN && task test` exit 0; `go test -v` count: **107 PASS, 0 FAIL, 0 SKIP**; build/vet/gofmt/check_i7 all green |
| README | **PASS with F-2** | Ports 26260/8081 correct; all documented commands exist; F1 limitation stated at `README.md:39`; 107-test claim verified. Prerequisites list omits Task — see F-2 |

---

# 4. Findings

## F-1 — Track 2 renders the TSV header row as a phantom belief

- **Severity:** IMPORTANT
- **Location:** `scripts/demo/track2.sh:57`
- **Evidence:** Actual captured output of `task demo:track2`:

  ```
  --- State before falsification ---
    Belief:  claim
    Status:  status
    Action:  action
    Intent:  intent_state
    Belief:  etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
    Status:  promoted
    Action:  deploy etcd v3.5.0
    Intent:  live
  ```

  The query is `--format=tsv` and returns exactly two lines — a header and one data row — confirmed
  with `cat -A`:

  ```
  claim^Istatus^Iaction^Iintent_state$
  etcd v3.5.0 is approved for production deployment…^Ipromoted^Ideploy etcd v3.5.0^Ilive$
  ```

  `tail -n2` takes *the last two lines*, i.e. header **and** row. Every other display block in the
  demo scripts correctly uses `tail -n +2` (`track1.sh:39`, `track2.sh:86`, `inspect.sh:17/33/47`).
- **Why it matters:** This is deterministic — it happens on every run — and it lands in the
  climactic "state before falsification" panel of the flagship Track 2 demo. A judge sees what looks
  like a second, malformed belief with status `status`. It undermines the credibility of a ledger
  demo whose entire pitch is that the displayed state is the real state.
- **Required correction:** `tail -n2` → `tail -n +2` on `scripts/demo/track2.sh:57`.

## F-2 — README omits Task from the prerequisites its own quick start requires

- **Severity:** MINOR
- **Location:** `README.md:7` ("Requires Docker and Go") vs `README.md:12` (`task setup`)
- **Evidence:** `scripts/demo/setup.sh:32-36` hard-fails with exit 1 when `task` is absent. Every
  documented entry point in the README — `task setup`, `task demo:track1`, `task demo:replay`,
  `task demo:track2`, `task inspect`, `task test` — is a Task invocation. The implementation report
  itself lists three prerequisites (Docker, Go, **Task**) at its line 335-337; the README lists two.
- **Why it matters:** §17 requires that nothing necessary for the advertised quick start is omitted.
  A judge following the README on a clean machine cannot execute step one.
- **Required correction:** `README.md:7` → "Requires Docker, Go, and [Task](https://taskfile.dev/installation/)."

## F-3 — The plan4 cleanup shipped a different mechanism than plan4 specified

- **Severity:** MINOR (informational; the goal is fully met)
- **Location:** `plans/demo/plan4.md:20` vs `cmd/solvent/main.go:158`
- **Evidence:** plan4 Option A prescribed `fmt.Printf` → `fmt.Fprintf(os.Stderr, …)` at
  `main.go:158`. That line is **unchanged** — still `fmt.Printf("Transcript written to %s\n", *out)`.
  The goal was instead achieved by removing `--out /dev/null` from the demo scripts entirely:
  `grep -Rn -- "--out" scripts/demo/` returns nothing, and the artifact is absent from all four
  captured demo outputs.
- **Why it matters:** I am required to verify the change described in §1, and it is not the change
  that was made. The substitution is, however, **better** on every axis §1 asks about: `cmd/solvent`
  is byte-identical to `HEAD`, so there is provably zero risk of hidden error suppression, altered
  exit codes, changed transcript contents, or database side effects — verified by
  `git diff HEAD -- cmd/` being empty, and by all four demos exiting 0 with full transcripts on
  stdout. Nothing to fix in the code.
- **Required correction:** Reconcile `plans/demo/plan4.md` (or a one-line note in the report) so the
  record states the shipped mechanism. Do **not** now apply the `Fprintf` edit — it is unnecessary
  and would reopen a frozen file.

## F-4 — `track1.sh` labels a total intent count as "Live intents"

- **Severity:** MINOR
- **Location:** `scripts/demo/track1.sh:74-77` (query) → `:96` (label)
- **Evidence:** The query is `SELECT count(*) FROM action_intent WHERE scenario_id = '…0001'` with
  **no `state` filter**; line 96 prints it as `Live intents`. Today the value is accidentally correct
  — I confirmed `intents_total = 1` and `intents_live = 1` — so nothing is currently misreported.
  Note the audit line immediately below it *does* filter on `a.state = 'live'` (`track1.sh:81-83`),
  so the two lines disagree about what they measure.
- **Why it matters:** The label is a claim about state. The moment scenario 1 ever holds a
  `cancelled` intent, the demo would overstate live authority — which is precisely the property
  Solvent exists to police.
- **Required correction:** Add `AND state = 'live'` to the `INTENT_COUNT` query, or relabel to `Intents`.

## F-5 — The report's transcripts are not faithful captures of the shipped scripts

- **Severity:** MINOR
- **Location:** `plans/post/playground_implementation_report.md:119, 122, 180, 205, 231` and `:224-228`
- **Evidence:** Two independent contradictions, in opposite directions:
  1. The transcripts contain `Transcript written to /dev/null` five times. The shipped scripts pass
     no `--out` flag at all, so current code **cannot** emit that line — and my captured runs do not.
     These blocks predate the cleanup the same report announces at its line 14.
  2. The Track 2 transcript at lines 224-228 shows a *clean* "State before falsification" block with
     no header row. As F-1 establishes, the shipped `track2.sh:57` emits the header deterministically.
     Current code **cannot** produce the report's clean block either.
- **Why it matters:** §20 asks whether the transcripts correspond to the claimed execution. They
  correspond to an earlier script revision. The *conclusions* the report draws are all independently
  true — I verified every one of them — but the pasted evidence is stale and would mislead anyone
  who diffs it against a live run.
- **Required correction:** Re-capture the four demo transcripts from the current scripts (after F-1
  is fixed) and replace the stale blocks.

## F-6 — The cold-pull claim is not independently re-established here

- **Severity:** MINOR (scope limitation of this review, not a code defect)
- **Location:** `plans/post/playground_implementation_report.md:12, 95, 97`
- **Evidence:** The report claims an acceptance run "from a torn-down state (fresh Docker container
  with cold image pull)" including `docker rmi cockroachdb/cockroach:v26.2.0`. By explicit decision
  for this review, I removed the **container** but retained the cached image, so `task setup` created
  the container from cache. What I *did* establish: `docker manifest inspect` performed a live
  registry round-trip and the pinned tag resolves to a real multi-arch OCI index
  (amd64 `sha256:30a70bfd1bfb…`), and the local image digest
  `sha256:235dd3c1a064…` runs and self-reports `CockroachDB CCL v26.2.0`.
- **Why it matters:** §2 asks not to accept a success that could be explained by a cached image. The
  tag-resolution half is proven; the byte-level download half is not re-proven by me.
- **Required correction:** None to the code. State the distinction if the receipt is cited to judges;
  re-run with `docker rmi` if a hard cold-pull receipt is wanted.

## F-7 — `source_observed_at` is always NULL, contradicting the report's checklist item 8

- **Severity:** MINOR
- **Location:** `internal/kernel/sql.go:18-20` vs `plans/post/playground_implementation_report.md:389`
- **Evidence:** Report item 8 asserts "`source_observed_at` remains meaningful in output while
  `ingested_at` is excluded from deterministic comparisons." Actual Track 1 evidence rows:

  ```
  content_sha256                     source_observed_at   ingested_at
  5686382eb6af707c…                  NULL                 2026-08-10 18:40:08.585328+00
  bbdccca2632853eb…                  NULL                 2026-08-10 18:40:08.556146+00
  ed0283a2a2604944…                  NULL                 2026-08-10 18:40:08.573559+00
  ```

  `sqlAddEvidence` inserts only `(scenario_id, belief_id, provenance_class, source_url,
  content_sha256)` — the column is never written, and `AddEvidence`'s signature
  (`internal/kernel/kernel.go:70`) has no parameter for it. The fixtures *do* carry the real dates
  (`released_at: "2026-03-20T17:41:08Z"`, `"2021-06-15T22:29:48Z"`) and `normalize` *does* parse them
  into `Normalized.ObservedAt` (`internal/normalize/types.go:17`) — the value is simply dropped at
  the kernel boundary.
- **Why it matters:** The report overclaims. Note this is **not** a playground regression: the kernel
  is byte-identical to `HEAD`, so this is a pre-existing property, and fixing it would mean editing a
  frozen kernel — out of scope. Also note the *load-bearing* half of §14 is fully satisfied:
  `ingested_at` is not a dedup key (dedup is on `content_sha256`, `internal/belief/belief.go:101-111`)
  and not a replay comparison value — my row-level diff showed `ingested_at` **unchanged** across the
  replay, which is itself the proof that no row was rewritten.
- **Required correction:** Amend report item 8 to say `source_observed_at` is currently unpopulated
  (a known kernel limitation), rather than "meaningful in output". No code change.

## F-8 — Both tracks run the pipeline twice and print the transcript twice

- **Severity:** NIT
- **Location:** `scripts/demo/track1.sh:17-28`, `scripts/demo/track2.sh:20-23` + `:67-70`
- **Evidence:** The `--reset` invocation also passes `--scenario` and `--fixtures`, so it resets *and*
  ingests. The section headed `--- Processing evidence ---` (track1) and
  `--- Step 3: Ingest postmortem + falsify ---` (track2) therefore re-runs an already-complete
  ingest. Captured output shows the identical 3-result transcript printed twice in Track 1 and the
  identical 2-result transcript twice in Track 2.
- **Why it matters:** Harmless and arguably a bonus idempotence demonstration, but it doubles the
  scroll and the second run's label promises work that already happened.
- **Required correction:** Optional — drop `--fixtures/--scenario` from the reset invocation, or
  relabel. Not required for approval.

## F-9 — Cosmetic output noise

- **Severity:** NIT
- **Location:** `scripts/demo/setup.sh:83-88`; `scripts/demo/track2.sh:88`
- **Evidence:** `setup.sh` redirects only stdout to `/dev/null`, so CockroachDB schema `NOTICE:`
  lines break the `Initializing database 'fable'...` progress line mid-sentence. Separately,
  `track2.sh:88`'s `[ -n "$intent" ]` guard does not catch CockroachDB's literal `NULL` string, so
  the final panel prints `intent: NULL` for beliefs that have no intent.
- **Required correction:** Optional — `2>/dev/null` on the schema apply; `[ -n "$intent" ] && [ "$intent" != "NULL" ]`.

---

# 5. End-to-End Receipts

Executed in the prompt's §20 order on this machine. Only `solvent-crdb` was touched; other
containers were left running.

| Command | Exit | Result |
|---|---|---|
| `docker manifest inspect cockroachdb/cockroach:v26.2.0` | 0 | Live registry resolution; OCI image index, amd64 digest `sha256:30a70bfd1bfb509d26e598f1999ab7f6693080d4c63f54ff76b0d9aa61a59094` |
| `docker stop solvent-crdb && docker rm solvent-crdb` | 0 | Container removed; `bchat-crdb`, `bchat-postgres`, `crdb3-*` untouched |
| `task setup` | 0 | `WARNING: Go 1.25.0 required (per go.mod), found 1.25.7` (non-fatal, correct behavior) → prerequisites OK → container created → `CockroachDB: READY` → `fable` + schema → binaries built. `docker inspect`: `26257/tcp→26260`, `8080/tcp→8081`, cmd `start-single-node --insecure --accept-sql-without-tls`. SQL reports `CockroachDB CCL v26.2.0`; tables = `action_intent, belief, belief_edge, evidence` |
| `unset FABLE_TEST_DSN && task test` **(container stopped)** | 1 | Fails with `dial tcp 127.0.0.1:26260: connect: connection refused`. **Expected and pre-existing** — `testdb.go:22` has pinned 26260 since commit `021b36f`. The suite targets `fable_test`, a different database from the demos' `fable`; the M2-R1 `_test` suffix guard prevents the suite from ever resetting `fable` |
| `unset FABLE_TEST_DSN && task test` **(container running)** | 0 | `belief`, `derive`, `intent`, `kernel`, `normalize`, `pipeline` all `ok`. Verbose count: **107 PASS / 0 FAIL / 0 SKIP**. `go build`, `go vet`, `gofmt` clean. `I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read (audit)` |
| `task demo:track1` | 0 | 3 results; final panel `Evidence 3 / Beliefs 3 / Promoted 1 / Live intents 1 / Audit SAFE`. Independent SQL confirms `beliefs=3, evidence=3, promoted=1, intents_total=1, intents_live=1, audit=0, derived=1` |
| `task demo:replay` | 0 | Before `3/3/1` → after `3/3/1`; `Replay verified: no duplicate ledger state created.` My row-level snapshots: beliefs, evidence, intents all **IDENTICAL row-for-row including IDs, `content_sha256`, and `ingested_at`**. Belief IDs preserved from Track 1. Result 3 correctly now shows `promoted: true, debt_items: 0`, reflecting the operator review |
| `task demo:track2` | 0 | `promoted` → `intent: live` → `cascade: retracted 1 belief(s)` → `[retracted] … intent: cancelled` → `Audit SAFE`. Independent SQL: postulated belief `retracted`, intent `cancelled` on a `retracted` belief, scenario audit 0, global audit 0, `belief_edge` 0 rows, total retracted = 1. Contains the F-1 header artifact |
| `task inspect` | 0 | 3 beliefs / 3 evidence / 1 intent / `Live intents on non-promoted beliefs: 0 (SAFE)`. Matches the post-Track-2 DB exactly. Showing Track 2 state is correct — Track 2 reset the database and ran last |
| `grep -Rni "Transcript written to /dev/null" scripts/demo/ README.md Taskfile.yml` | 1 | No matches. Also absent from all four captured demo outputs |
| Fixture hash check (5 files vs `manifest.json`) | — | 5/5 MATCH; `track2.sh:12` `V350_SHA` matches |
| `git status --short` after the full run | — | Unchanged from the start: `M README.md`, `M docs/M2_TRANSCRIPT.md`, plus the untracked playground paths. No source file was modified by this review |

**Network isolation:** `grep` for `net/http`/`http.Client`/`net.Dial` across all non-test Go sources
returns **nothing**; `grep` for `curl|wget|nc|fetch` across `scripts/demo/*.sh` returns **nothing**.
The only URLs in the demo scripts are three install hints in `setup.sh:15/21/34` and the
`--evidence-url` **literal string** at `track2.sh:32`, which is recorded as provenance metadata and
never dereferenced. Both tracks reconstruct the etcd scenario purely from pinned on-disk fixtures.
Network is required only for the initial Docker image pull.

---

# 6. Regression Check

| Item | Status | Evidence |
|---|---|---|
| `db/001_schema.sql` | **UNCHANGED** | `git diff HEAD -- db/` empty. Exactly 4 `CREATE TABLE` statements: `belief`, `belief_edge`, `evidence`, `action_intent`. No new tables |
| `internal/kernel/kernel.go` | **UNCHANGED** | `git diff HEAD -- internal/` empty |
| `internal/kernel/sql.go` | **UNCHANGED** | Same. `RetractCascade` still uses `WITH RECURSIVE d(id)` (`sql.go:67`), `UNION` not `UNION ALL` (`sql.go:62`), scenario-scoped per D-032 |
| `FullDebt` | **UNCHANGED** | `internal/kernel/kernel.go:28`; test B-23 confirms it has not drifted from the DDL `ARRAY[…]` default |
| `belief_edge` | **UNPOPULATED** | `SELECT count(*) FROM belief_edge` = **0** after both tracks. No script or non-test Go source inserts into it |
| `ExecuteTx` discipline (I-7) | **INTACT** | `scripts/check_i7.sh` PASS: 7 `crdb.ExecuteTx` write sites, 0 raw `s.db.Exec/Query` writes, 1 permitted pool read (the I-5 audit). D-033 gate: 0 `CREATE TEMP` occurrences |
| Kernel bypass by demo scripts | **NONE** | All demo mutations go through `cmd/solvent` / `cmd/operator-review`. Every `docker exec … cockroach sql` in the scripts is a `SELECT`. `db:reset` in `Taskfile.yml:22-27` is DDL-only and is not invoked by any demo |
| Cluster settings | **NONE REQUIRED** | No `SET CLUSTER SETTING` in any shipped file. Deliberate property of the recursive-CTE design (D-033), confirmed by `check_i7.sh` enforcement — correctly treated as a property, not a gap |
| Existing tests | **GREEN** | 107 PASS / 0 FAIL / 0 SKIP |
| Out-of-scope tech (RAG, embeddings, vector DB, Kafka, K8s, web UI, bchat, MCP, live fetching, second DB) | **ABSENT** | Repo-wide grep finds only pre-existing *negative* assertions — test B-14 and the `db/001_schema.sql:3` comment proving I-6 (no embedding column) |
| `docs/M2_TRANSCRIPT.md` (modified) | **BENIGN** | Diff is regenerated timings and `generated_at` only; every row still `PASS`. A test-run artifact, not a scope violation |

---

# 7. Report Accuracy

`plans/post/playground_implementation_report.md` **accurately describes the actual implementation in
substance.** Every material claim I could test held up under independent verification:

| Report claim | Independently established? |
|---|---|
| CRDB pinned to v26.2.0 | **Yes** — tag resolves live; container self-reports v26.2.0 |
| No `SET CLUSTER SETTING` required (recursive-CTE / D-033) | **Yes** |
| `task test` self-contained, no `FABLE_TEST_DSN` override | **Yes** — and the port alignment is the *right* fix: `testdb.go:22` was never modified, the container was aligned to it |
| Track 1 values are DB-derived, not constants | **Yes** — including the previously hard-coded `Promoted 1`, now `SELECT count(*)` at `track1.sh:86-90` |
| Replay is genuine, no reset hidden in the path | **Yes — and stronger than claimed.** Row-level identity, not just count identity |
| Dedup via `EnsureBelief` CTE + `evidenceExists(content_sha256)`; `Derive()` pure, no timestamps in keys | **Yes** |
| Track 2 = single-belief retraction + cancelled dependent intent | **Yes** — `retracted 1 belief(s)`, `belief_edge` still empty |
| `inspect` reflects real current state | **Yes** |
| No hidden network dependency | **Yes** |
| Schema/kernel/`FullDebt` unchanged, `belief_edge` unused, no new tables | **Yes** — `git diff HEAD` is empty across `db/`, `internal/`, `cmd/`, `proof/` |
| 107 tests green | **Yes** — exactly 107 |
| Capture-harness diagnostic removed from judge-facing output | **Yes** |
| Acceptance run included a cold image pull (`docker rmi`) | **Not re-established here** — see F-6 |
| Pasted command transcripts correspond to the shipped scripts | **No** — see F-5 |
| `source_observed_at` remains meaningful in output (item 8) | **No** — it is NULL; see F-7 |

So: the report's *conclusions* are sound and I confirmed them from the repository and runtime. Its
*pasted evidence* is stale, and two secondary claims (cold pull, `source_observed_at`) overreach.
The gap is in the receipts, not in the implementation.

---

# 8. Final Recommendation

**The architecture and implementation are APPROVED.** The playground is reproducible, honest, and
judge-ready, and the verified Solvent core is provably unregressed. No blocker was found, and none
was manufactured.

Apply these corrections, then freeze the CLI/Taskfile playground:

1. **F-1 (before the demo)** — `scripts/demo/track2.sh:57`: `tail -n2` → `tail -n +2`.
2. **F-2** — `README.md:7`: add Task to the prerequisites.
3. **F-4** — `scripts/demo/track1.sh:74-77`: add `AND state = 'live'`, or relabel line 96 to `Intents`.
4. **F-5** — re-capture the four demo transcripts in the report from the current scripts (do this
   *after* F-1, so the fixed output is what gets recorded).
5. **F-3, F-6, F-7** — record-keeping only: note in `plans/demo/plan4.md` / the report that the
   cleanup shipped by removing `--out` rather than by editing `main.go`; scope the cold-pull receipt
   to what was actually re-run; and correct report item 8 to say `source_observed_at` is currently
   unpopulated.

F-8 and F-9 are optional polish and explicitly **not** conditions of approval.

Do not open another review cycle for these. They are four one-line edits and three documentation
corrections; none touches the schema, the kernel, `cmd/`, or any invariant. No new architecture is
requested, the research phase stays closed, and the absence of `belief_edge` propagation remains the
deliberate, correctly documented MVP boundary — not a defect.

The next phase (Solvent MCP server) is out of scope and was not reviewed.
