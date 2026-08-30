# Post-Fable Remediation Verification — Solvent

**Reviewer:** Claude (Fable 5) — the author of the original adversarial review (F-1 … F-23)
**Date:** 2026-08-10
**Scope:** Did the remediation described in `plans/post/fix.md` actually fix what it claims?
**Method:** Full dynamic re-verification against a fresh single-node CockroachDB v26.2.0
(`scripts/m0_up.sh`, port 26260). Every code claim was read at `file:line`; every "this passes"
claim was re-executed. **No tracked file was left modified by this review** — receipts that the
gates and the test suite overwrite were snapshotted before the run and restored afterward;
`git status --short` at the end was byte-identical to the baseline taken at the start.
Nothing was built into the working tree (the CLI was built to an off-tree path).

No new requirements were introduced. Waves 1–4 were not reopened. No architectural change is
recommended anywhere in this document.

---

## Verdict summary

The **code** side of the remediation is real. Every mechanical defect I flagged in the kernel,
pipeline, normalizer, CLI, gate script, and repository hygiene is genuinely fixed, and the
acceptance machinery that was red now runs green end-to-end — I executed all of it. The
falsification path is not a decorative flag: it reaches `kernel.RetractCascade`, it is
scenario-scoped, and I watched it move a promoted belief to `retracted` while its live intent
became `cancelled` with the I-5 audit still 0.

The **receipt and documentation** side is not finished, and one item moved backwards. `fix.md`
claims all receipts were "regenerated … from one current build/test run"; two of them were not,
and I can prove the M2 pair still comes from different runs. `fix.md` claims the README
contradictions are fixed; at least four survive, including the very class of claim F-3 was
about ("seven functions"), and the remediation introduced a new false count — the tree now runs
**85** tests while README, `AGENTS.md`, `PUBLIC_REPO_AUDIT.md`, and `fix.md`'s own verification
table all say 84.

Because two items the remediation explicitly claimed as fixed (F-5, F-23) are still partly
broken, and because the remediation itself introduced a fresh untrue count, the verdict is
**REWORK REQUIRED** — but the remaining list is small, entirely documentation/receipt-level, and
none of it touches code behavior.

---

## Findings table

| Finding | Status | Evidence | Remaining Issue |
|---------|--------|----------|-----------------|
| **F-1** I-7 gate expects 6 | **FIXED** | `scripts/check_i7.sh:14` → `EXPECT_TX=7` with `EnsureBelief` in the comment. Executed: `I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read`, exit 0. `internal/kernel/kernel.go` is **absent** from `git diff HEAD --stat` — no code was removed to satisfy the count. All four gate sections (raw writes, single pool read, tx count, `CREATE TEMP`) still active; drift detection intact because the count is still asserted with `-ne`. Regenerated `docs/M1_I7.md` is byte-identical to the committed one. | none |
| **F-2** gofmt non-empty | **FIXED** | `gofmt -l cmd internal` → empty, exit 0 (run standalone and inside both gates, over the whole `cmd`+`internal` trees, not just changed files). The `derive.go`/`types.go` diffs are pure `var`-block alignment; no token changed. README gained the gofmt line (`README.md:146`). | none |
| **F-3** `EnsureBelief` missing from Contract | **PARTIALLY FIXED** | `internal/kernel/contract.go:24` declares `EnsureBelief(context.Context, string, string, ClaimType) (string, error)` — identical to `Store`'s signature (`kernel.go:166`); `var _ Contract = (*Store)(nil)` still active at `contract.go:28` and the module builds. `IMPLEMENTATION_CONTRACT.md` §4 amended with the eighth function. | Stale "seven functions" claims survive in three places, one of them judge-facing: `README.md:59-62` still lists the kernel's public surface as seven functions with no `EnsureBelief`; `docs/M1_REPORT.md:33` "all seven §4 functions"; `docs/M2_REPORT.md:74` same. |
| **F-4** `RetractCascade` unreachable | **FIXED (as scoped)** | Full path traced and executed — see §3. `cmd/solvent/main.go:36,48-53` → `runFalsify` (`:168`) → `kernel.RetractCascade` (`:188`). Live result: belief `a63c00fc…` `promoted → retracted`, its intent `live → cancelled`, direct SQL audit `live_on_nonpromoted = 0`. Scenario scoping enforced by `descendantsCTE` (`sql.go:66-75`) and confirmed adversarially. | Two truthfulness notes, not mechanism failures: a no-op falsify still prints `VERDICT: PASS` and exits 0 (N-5), and nothing in shipped code writes `belief_edge`, so the CLI cascade set is always a singleton (N-6). |
| **F-5 / F-8** stale receipts | **PARTIALLY FIXED** | Verified reproducible by re-running the machinery: `docs/M1_PREPARE.md` **9/9** (my run differs only in `generated_at` and three `elapsed_ms` cells); `docs/M1_I7.md` byte-identical; `docs/M1_BUILD.md` byte-identical; `docs/M0_TRANSCRIPT.md` byte-identical below the volatile fence; `M2_TRANSCRIPT.md` case content identical apart from UUIDs/timings. `cmd/m1verify/main.go:48` now says "expected: 9 statements prepared". **F-8 fully fixed**: `kernel_test.go:811` reads "asserted statically: 7 ExecuteTx write sites, 0 raw writes", and the gate it defers to now genuinely passes, and `m2_accept.sh` genuinely reaches the suite (exit 0) — B-15's statement is now true rather than vacuous. | (a) `docs/M2_REPORT.md:10,16` still says "**25/25** cases pass" / "25 rows" against a 28-row transcript — untouched by the remediation. (b) The committed M2 pair is **still from two different runs** (proof in §2). (c) `docs/M0_TRANSCRIPT.md` was regenerated with `--no-timings`, silently deleting the `elapsed_ms` column from a frozen receipt (N-9). |
| **F-6** CLI configuration | **FIXED** | Tested, not read — see §4. No DSN → `no DSN: pass --dsn or set FABLE_DSN`, exit 1. `FABLE_DSN` honored (reset banner printed `fable_demo`). `--dsn` overrides `FABLE_DSN`. `--fixtures` works from `/tmp`; without it from `/tmp` → `fixture directory not found: internal/derive/testdata`, exit 1. Nothing silently reached `fable_test`. | Two sharp edges in the new code, both low: a trailing slash on `--fixtures` defeats the schema fallback (`main.go:73`), and `runFalsify` never applies that fallback at all — in both cases `cliReset` has **already dropped the database** before the failure (N-4). |
| **F-7** `DebtItems` empty strings | **FIXED (production); test claim overstated** | `internal/pipeline/pipeline.go:389` selects `debt::STRING` and `:395` populates via `parsePGArray` (`:400`). I confirmed the literal the parser consumes is real: `{needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}`. No `make([]string, debtLen)` placeholder survives anywhere. | The new "no element is empty" assertion (`pipeline_test.go:216-220`) runs against a **duplicated copy** of the parser inside the test file (`pipeline_test.go:687-700`), not against production `Result.DebtItems`; the only assertion on the real field is still a length check (`:87`). The field is fixed; the test credited with proving it does not cover it (N-7). |
| **F-9** `evidenceExists` swallows errors | **FIXED** | `internal/belief/belief.go:103` now returns `(bool, error)`; caller wraps at `:64-67`. Exercised the error path live: dropped the `evidence` table, ran the pipeline → `pipeline: belief.Process: check evidence exists: ERROR: relation "evidence" does not exist (SQLSTATE 42P01)`, exit 1. A transient failure can no longer degrade into the duplicate-insert path; only a successful `count = 0` reads as absence. | none |
| **F-13** normalizer hardcodes etcd | **FIXED** | `internal/normalize/normalize.go:15` `githubAssetPattern`, applied at `:314`; URL and subject derived from payload (`:319-326`). `TestNormalizeRelease_NonEtcd` passes and asserts neither URL nor subject contains "etcd". The etcd fixture still yields `etcd-io/etcd v3.5.15` — from its own asset URL, not a constant. `ContentSHA256` is unaffected (it hashes version/commit/prerelease only), so the pinned golden hash and all downstream receipts are untouched — confirmed by the CLI transcript reproducing byte-for-byte. | An assets-less release falls back to `https://github.com/releases/tag/<tag>` (`:324`), a non-resolving URL, which `README.md:105-107` forbids for `external_feed` rows (N-8). Low: no shipped fixture takes that branch. |
| **F-21** tracked binary | **FIXED** | `solvent` staged as deleted (`D  solvent`, 13,665,796 bytes removed); `.gitignore` gained `/solvent` under an explicit "Extensionless Linux binaries" comment. No replacement binary is tracked — I built to an off-tree path and `git status` stayed clean. | History still carries the blob (inherent; noted in the original review, not a remediation failure). |
| **F-22** stale `files/` + `files.zip` | **FIXED** | `files/001_schema.sql`, `files/02_lifecycle_and_invariants.sql`, `files/IMPLEMENTATION_CONTRACT.md`, `files/cells.sh`, `files/go.mod`, and the boobytrap `files/kernel.go` all staged deleted, plus `files.zip`. Verified the authoritative originals survive: `db/001_schema.sql`, `IMPLEMENTATION_CONTRACT.md`, `proof/` — and the schema still applies cleanly (M0 B1 PASS, 6 statements). | Root-level planning `.md` duplication (original F-22's second half) is untouched; two new untracked root files exist (`FABLE_REMEDIATION.md`, `review_fable.md`). Not blocking — see §4. |
| **F-23** README contradictions | **PARTIALLY FIXED** | Fixed and verified: four tables named correctly (`README.md:53-54`, matching the live `B3` probe); the false vector claim replaced with "**No embedding column**" (`:91-93`); gofmt added to "Before you commit" (`:146`); `-falsify` documented (`:125-126`). | Four surviving contradictions plus one newly introduced — N-1, N-2, N-3 and the `:34`/`:148`/`:72` embedding-and-vector leftovers. Detail in §6. |

Deferred by agreement, verified **not regressed**: F-10 (§5), F-11, F-12, F-14, F-15, F-16, F-17,
F-18, F-19, F-20 — all still present exactly as originally described, none aggravated.

---

## 1. Remediation verification

The nine code-level items (F-1, F-2, F-3-code, F-6, F-7, F-9, F-13, F-21, F-22) are genuinely
fixed, and the fixes are the minimal ones. Three checks specifically guard against a fix that
merely satisfies its own gate:

- **F-1 was not achieved by deleting code.** `git diff HEAD --stat` touches 29 files;
  `internal/kernel/kernel.go` is not among them. The kernel still has seven `crdb.ExecuteTx`
  sites and eight §4 functions; only the expected count moved.
- **F-2 was not achieved by narrowing the check.** Both gates still run `gofmt -l cmd internal`
  over the whole trees and still hard-block on any output; I ran it standalone as well.
- **F-3's compile-time guarantee is live, not documented.** `var _ Contract = (*Store)(nil)`
  remains at `contract.go:28`, and the interface now has all eight methods, so a signature drift
  fails `go build` — which I ran (exit 0).

Two items are only partly done, and in both cases the *claim* in `fix.md` is what is wrong
rather than the code:

- **F-5**: "Regenerated all receipts through the actual acceptance machinery from one current
  build/test run" is not true of `docs/M2_TRANSCRIPT.md` + `docs/M2_TEST.md` (§2), and
  `docs/M2_REPORT.md` was never touched at all, so the 25/25-vs-28/28 contradiction I originally
  filed under F-5 is still on disk.
- **F-23**: the two lines named in `fix.md` were fixed; the rest of the file was not re-read
  against the code, which is the same method failure `PUBLIC_REPO_AUDIT.md` committed
  (verify-the-listed-claims rather than diff-the-whole-file).

---

## 2. Regression verification

Everything below was executed in this review, in this order, against a fresh cluster.

```
$ ./scripts/m0_up.sh
starting single-node cluster on localhost:26260 (http 8081)
CockroachDB CCL v26.2.0 (x86_64-pc-linux-gnu, built 2026/04/21 18:36:57, go1.25.5)

$ go build ./...                       exit=0
$ go vet ./...                         exit=0
$ gofmt -l cmd internal                (no output) exit=0
$ ./scripts/check_isolation.sh          E2 PASS: internal/kernel imports nothing from internal/m0
$ ./scripts/check_i7.sh                 I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read (audit)

$ go test -count=1 -p 1 ./...
ok  internal/belief 3.171s   ok internal/derive 0.003s   ok internal/intent 2.849s
ok  internal/kernel 4.326s   ok internal/normalize 0.010s   ok internal/pipeline 3.597s
exit=0

$ go run ./cmd/m0verify --reset --apply --no-timings --out <off-tree>   # ×2
**M0 GREEN** — 16/16 probes passed.   (both runs)
diff below volatile fence, run1 vs run2:            BYTE-IDENTICAL      # E1 holds
diff below fence, committed docs/M0_TRANSCRIPT.md:  BYTE-IDENTICAL      # receipt reproduces

$ ./scripts/m1_accept.sh
E2 PASS · build 0, vet 0, gofmt clean · I-7 PASS 7 sites · PREPARE sweep green — 9/9
M1 gate green.                                                          exit=0

$ ./scripts/m2_accept.sh
E2 PASS · build 0, vet 0, gofmt clean · I-7 PASS · run 1 PASS · run 2 PASS
M2 gate green.                                                          exit=0
docs/M2_TRANSCRIPT.md: **GREEN — 28/28 cases passed**, 28 rows
docs/M2_TEST.md:       27 top-level PASS per run, twice
```

Against `fix.md`'s verification table, every line reproduces **except the test total**:

| Claim in `fix.md` | Verified | Actual |
|---|---|---|
| build / vet / gofmt clean | ✅ | exit 0, exit 0, empty |
| all 6 test packages ok | ✅ | ok ×6 |
| E2 PASS | ✅ | PASS |
| I-7 PASS — 7 sites | ✅ | 7 |
| `m1_accept.sh` exit 0 | ✅ | exit 0 (prerequisite: the schema must already be applied — run `m0verify --apply` first; the script does not apply it and fails at PREPARE with 42P01 on a bare database. Pre-existing, not a remediation defect.) |
| `m2_accept.sh` exit 0 — 28/28 twice | ✅ | exit 0, 28/28, twice |
| M0 16/16 GREEN | ✅ | 16/16, E1 byte-identical |
| PREPARE 9/9 | ✅ | 9/9 |
| CLI `VERDICT: PASS` | ✅ | PASS, audit 0 |
| `-falsify` PASS — 1 belief retracted | ✅ | 1 retracted, intent cancelled, audit 0 |
| CLI from another directory | ✅ | works with `--fixtures` + DSN |
| Replay determinism | ✅ | byte-identical, twice |
| no tracked binary / `files/` / `files.zip` | ✅ | all gone |
| **"Total tests: 84, all passing"** | ❌ | **85 pass** — see N-1 |

**Proof that the committed M2 receipt pair is still from two different runs.** In a genuine
`m2_accept.sh` pass, `M2_TRANSCRIPT.md` is written by run 2, so its B-21 observation must equal
run 2's. My run:

```
docs/M2_TRANSCRIPT.md   returned 2 in 14 ms
docs/M2_TEST.md  run 1  returned 2 in 16 ms
docs/M2_TEST.md  run 2  returned 2 in 14 ms      ← matches the transcript, as it must
```

The committed remediation receipts:

```
docs/M2_TRANSCRIPT.md   returned 2 in  7 ms
docs/M2_TEST.md  run 1  returned 2 in  5 ms
docs/M2_TEST.md  run 2  returned 2 in  6 ms      ← does NOT match the transcript
```

So the committed `M2_TRANSCRIPT.md` was overwritten by a *later, separate* `go test` after
`m2_accept.sh` finished — almost certainly the `go test -count=1 -p 1 ./...` line in `fix.md`'s
own verification table. This is F-10's hazard already realized in shipped evidence, and it is
why the F-5 sub-finding "captured from different builds" survives. The content is not falsified
(both are 28/28 green; case text is identical apart from UUIDs and timings) — but the pair is
not the single-pass artifact `fix.md` says it is.

No regression was found anywhere: 6/6 packages green, M0 16/16 with E1 determinism intact, M2
28/28 twice, PREPARE 9/9, I-7 = 7, both CLI modes PASS, replay byte-identical.

---

## 3. Falsification CLI verification

Path traced through code and then executed against a live cluster. Existence of the flag was
not accepted as evidence at any step.

**Call path.** `main.go:36` declares `-falsify`; `:40-45` resolves the DSN (flag → `FABLE_DSN`
→ hard error); `:48-54` short-circuits into `runFalsify` **before** any fixture, schema, or
pipeline work, with `:49-51` refusing to proceed without `--scenario`; `runFalsify` (`:168`)
opens the pool, pings, constructs `kernel.New(db)`, and calls
`st.RetractCascade(ctx, scenarioID, beliefID)` at `:188` — the same function the kernel suite
proves (B-07/16/19/20/22/24), unchanged. Scoping is structural, not advisory: `descendantsCTE`
(`sql.go:66-75`) anchors on `b.id = $1 AND b.scenario_id = $2` and re-joins every hop back to
`belief` in the same scenario, and both cascade halves carry `AND scenario_id = $2`
(`sql.go:81-91`). Cancel precedes retract inside one `crdb.ExecuteTx` (`kernel.go:126-142`).

**Live run** (`FABLE_DSN` → scratch database `fable_demo`, never `fable_test`):

```
$ solvent -reset --scenario 55555555-0000-0000-0000-000000000001
=== Solvent CLI === resetting database
    database: fable_demo  (DROP + CREATE + apply db/001_schema.sql)
WARN pipeline: contradiction received, no ledger mutation source_url=…issuecomment-220123459
results: 2 · belief a63c00fc-… · promoted: true · debt_items: 0 · intent: live
audit_live_on_nonpromoted: 0 · VERDICT: PASS

$ (SQL) SELECT id,status FROM belief;  SELECT id,state FROM action_intent;
a63c00fc-… | promoted        a3b674c9-… | live
```

Then the falsification, with the pipeline flags deliberately also present to test exclusivity:

```
$ solvent -falsify a63c00fc-… --scenario 55555555-…-0001 --fixtures internal/derive/testdata
FALSIFY
belief: a63c00fc-f20f-467c-ae2b-e9e79cc7d131
cascade: retracted 1 belief(s)
audit_live_on_nonpromoted: 0
VERDICT: PASS                                    exit=0

$ (SQL) belief a63c00fc-… | retracted     intent a3b674c9-… | cancelled
$ (SQL) SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id
        WHERE a.state='live' AND b.status<>'promoted';          → 0
$ (SQL) SELECT count(*) FROM evidence;                          → 5 (unchanged)
```

- **The receipt is truthful for the real case**: 1 retracted matches the database exactly; the
  intent is `cancelled`, not deleted; the audit is 0 both as printed and as an independent query.
- **Cancel-precedes-retract is proven by the schema, not by the print**: had the CLI retracted
  first, the composite FK's `ON UPDATE CASCADE` would have propagated `retracted` into the live
  intent and `live_requires_promoted` (23514) would have aborted the transaction — which is
  exactly what M2 case B-12 demonstrates in the same run.
- **Mutual exclusivity is real, not documentary**: with `--fixtures` and `--scenario` both
  supplied, the evidence count stayed at 5 and no pipeline transcript was produced. The pipeline
  never ran.
- **Transactional**: single `crdb.ExecuteTx`; rollback-on-block is proven by B-16 in the same
  green suite.

**Adversarial cases — all fail safely:**

| Case | Result |
|---|---|
| `-falsify <id>` with no `--scenario` | `--falsify requires --scenario to scope the cascade`, exit 1 |
| `-falsify <valid id>` with a **different** scenario | `retracted 0 belief(s)`; direct SQL after: belief still `promoted`, intent still `live`, evidence 5 — **the other scenario's rows were not touched**. Knowing an ID is not enough. |
| `-falsify not-a-uuid` | exit 1 — `retract cascade: ERROR: … could not parse "not-a-uuid" as type uuid (SQLSTATE 22P02)`. Raw SQLSTATE surfaced, nothing mutated. |
| `-falsify` a well-formed nonexistent UUID | `retracted 0 belief(s)`, nothing mutated |
| `-falsify` an already-retracted belief | `retracted 0 belief(s)` — idempotent (`status='promoted'` guard at `sql.go:89`) |

Two honest limitations, both reported as new findings rather than F-4 failures:

- **N-5**: every no-op above still prints `VERDICT: PASS` and exits 0. The verdict is computed
  from the audit alone (`main.go:205-209`), so "I falsified nothing" and "I falsified the belief"
  are indistinguishable by exit code on a judge's screen. The `retracted 0 belief(s)` line is
  literally truthful; the verdict line is not informative.
- **N-6**: no shipped code path writes `belief_edge` — the only writers are
  `internal/kernel/kernel_test.go` and the M0 probe harness (`internal/m0/gate.go:84`). I
  confirmed `SELECT count(*) FROM belief_edge` = **0** after a full pipeline run. The CLI can
  therefore only ever demonstrate a one-belief retract plus its own intent cancel; the
  multi-generation cascade remains test-only (B-07/20/22 prove it). `README.md:125-126` says
  "atomically retract a belief **and its cascade**", which overstates what the binary can show.

Judged against what the original review actually asked for — "even a `-falsify <belief-id>` flag
that calls `RetractCascade` and re-prints the audit would put the headline capability on stage" —
this is delivered, verified, and safe. The capability is genuinely judge-playable.

---

## 4. Public repository verification

- **Binary hygiene** — `solvent` untracked and ignored; building it produces no `git status`
  entry. Confirmed by building to an off-tree path and re-checking status.
- **Stale trees** — `files/` and `files.zip` gone, including the pre-`EnsureBelief` `kernel.go`
  draft. Authoritative sources intact and functional: the frozen DDL still applies (M0 B1),
  `IMPLEMENTATION_CONTRACT.md` is present and now records eight functions, `proof/` untouched.
- **README vs reality** — read in full (162 lines) against the live schema, CLI, and contracts.
  Corrected claims verified; surviving ones listed in §6. The four tables at `:53-54` match the
  live `B3` probe exactly; there is no vector/embedding column claim in the architecture section
  any more; gofmt is in the pre-commit list; `-falsify` is documented (though not runnably).
- **`PUBLIC_REPO_AUDIT.md` is now stale in a new way.** Its §5 certifies "84 tests" with a
  per-package table listing `normalize` at 11 (`:90-101`) and declares README and `AGENTS.md`
  "Correct — 84 tests". All three are now wrong by one. The audit was not re-run with the
  diff-the-whole-file method the original F-23 asked for, so the document that missed README
  lines 91 and 100 still stands as the repo's self-certification.
- **New untracked root files** — `FABLE_REMEDIATION.md` (a near-duplicate of `plans/post/fix.md`)
  and `review_fable.md` (a duplicate of `plans/post/review_fable.md`). Untracked, so they harm
  nothing today; committing them would recreate the root-duplication pattern F-22 was about.

---

## 5. DEFERRED FINDING — F-10

**Still present, and it has already damaged a shipped receipt.**

`internal/kernel/suite_test.go:21` hardcodes `transcriptPath = "../../docs/M2_TRANSCRIPT.md"`
and `:64` writes it unconditionally at suite teardown. There is no flag, no environment
override, and no freeze check. I reproduced the side effect: a plain `go test -count=1 -p 1 ./...`
left `docs/M2_TRANSCRIPT.md` modified (diff confined to `generated_at`, per-case `elapsed_ms`,
and generated UUIDs; content 28/28 PASS).

**Does it create an immediate public-repository correctness problem? Yes — a small but real one.**
Not because the transcript is false (it is not; it reproduces), but because:

1. It is why the committed `M2_TRANSCRIPT.md` and `M2_TEST.md` disagree today (§2). The "one
   current build/test run" claim in `fix.md` is untrue *as a direct consequence of F-10*, so
   F-10 is no longer purely latent.
2. `README.md:147` instructs every contributor to run the suite before committing, which dirties
   a tracked evidence artifact by design. A reviewer who reverts that "spurious" change and a
   reviewer who commits it produce different repositories from the same green run.

This is not a failure of the current remediation and I do not count it against the verdict.
The cheapest containment consistent with the project's own idiom is ordering discipline —
regenerate the M2 pair last, via `m2_accept.sh`, and commit that pair together — rather than any
code change.

**F-11, F-12, F-14, F-15, F-16, F-17, F-18, F-19, F-20 — verified unchanged, no regressions:**
`_ = text` still at `pipeline.go:177,210`; `computeSHA256` still returns `""` on marshal failure
(`normalize.go:448`); still no `UNIQUE (scenario_id, claim)` (`db/001_schema.sql:23` has only
`belief_id_status_key`); A3's declared statement still differs from what executes
(`internal/m0/env.go:46`); issue `Assertion` still title-only (`normalize.go:88,174`);
`TestPropose_OnUnpromotedBelief` still asserts only `err == nil` (`intent_test.go:106-108`);
`AcquireResetLock` still spins without timeout (`testdb.go:110-116`); `FullDebt` still an
exported mutable slice (`kernel.go:28`); dead code still at `normalize.go:459-460` and
`derive.go:16,131`.

---

## 6. NEW findings introduced or missed by the remediation

**N-1. The test count is now wrong in four documents — including the remediation's own receipt.**
The tree runs **85** tests, not 84. Verified by execution:
`internal/normalize` 12 · `internal/derive` 16 · `internal/kernel` 27 · `internal/belief` 18 ·
`internal/intent` 3 · `internal/pipeline` 9 = 85 top-level `--- PASS`, zero subtests.
The F-13 fix added `TestNormalizeRelease_NonEtcd`, taking Wave 1 from 11 to 12, and no count was
updated: `README.md:135` (Wave 1 = 11), `README.md:140,147` ("84 tests"), `AGENTS.md:90`,
`PUBLIC_REPO_AUDIT.md:90-101`, and `fix.md`'s own "Total tests: **84**, all passing". This is a
truthfulness regression created by the remediation, in exactly the class of defect it was
convened to remove. Fix: change 11→12 and 84→85 in those five places.

**N-2. `README.md:59-62` still describes the kernel as seven functions.** The public-surface
bullet lists `New`, `EnterBelief`, `AddEvidence`, `RetireDebt`, `Promote`, `IntentOnPromoted`,
`RetractCascade`, `AuditLiveOnNonPromoted` — no `EnsureBelief`. F-3 amended the machine-checked
contract and the supreme document but not the human-facing one, which is where a judge looks.

**N-3. The README's CLI instructions no longer work as written.** `README.md:123-126` says
"Run it with `--scenario <UUID> --reset`" and "Use `-falsify <belief-id> --scenario <UUID>`".
Both now exit 1 with `no DSN: pass --dsn or set FABLE_DSN` — a direct consequence of the F-6
fix, which correctly removed the `fable_test` default without updating the instructions that
depended on it. Fix: show `FABLE_DSN=…` (or `--dsn`) in both examples, and mention `--fixtures`
for non-repo-root use.

**N-4. `cliReset` drops the database before it knows it can apply the schema.** `main.go:216-248`
runs `DROP DATABASE` + `CREATE DATABASE` and only then resolves and reads the DDL. Two reachable
inputs hit that window, both verified live:
`--fixtures <path>/testdata/` (trailing slash) defeats the `filepath.Dir` fallback at `:73`, and
`-falsify --reset` from any non-repo-root CWD never gets a fallback at all (`:172` passes
`*schema` verbatim). Both printed
`reset: read db/001_schema.sql: … no such file or directory`, exit 1 — **after** the database had
been dropped and recreated empty. Low severity (`--reset` is explicitly destructive), but the
failure is silent about what it destroyed.

**N-5. A no-op falsification reports `VERDICT: PASS` and exit 0.** See §3. Wrong scenario,
nonexistent ID, and already-retracted belief all print PASS. Cheapest honest fix: make the
verdict line distinguish `retracted 0` (e.g. `VERDICT: NO-OP`), leaving the audit check as-is.

**N-6. Nothing outside tests writes `belief_edge`, so the CLI cascade is always a singleton.**
See §3. Either soften `README.md:125-126` ("retract a belief and cancel its live intents
atomically") or note explicitly that the multi-generation cascade is evidenced by M2 cases
B-07/B-20/B-22 rather than by the CLI.

**N-7. `parsePGArray` is duplicated, and the F-7 test exercises the copy.** Production at
`pipeline.go:400`, a second identical function at `pipeline_test.go:687-700`. The new
"no element is empty string" assertion runs against the test's copy, so production
`Result.DebtItems` could regress to placeholders with the suite still green. Separately, the
parser splits on `,` with no quote handling — correct for the six fixed debt identifiers, and I
am **not** recommending a general parser; it is worth one comment saying why the simple split is
sufficient.

**N-8. The release normalizer's assets-less fallback emits a non-resolving URL.**
`normalize.go:324` yields `https://github.com/releases/tag/<tag>` when no asset matches. No
shipped fixture reaches it, but `README.md:105-107` requires `external_feed` rows to carry a
resolving `source_url`. Returning an error, or leaving `SourceURL` empty, would be more
consistent with the rest of the package's validation style.

**N-9. `docs/M0_TRANSCRIPT.md` lost a column.** It was regenerated with `--no-timings`, so the
committed frozen receipt no longer has `elapsed_ms` for its 16 probes (the pre-remediation
version did). Determinism is unaffected — that is exactly what the flag is for, and E1 verified
byte-identical below the fence — but the archived receipt is now less informative than the one it
replaced. Regenerating without `--no-timings` for the committed copy, and using `--no-timings`
only for the two-run determinism diff, restores it.

---

## Rework list (complete, and all documentation/receipt-level)

1. `84` → `85` and Wave 1 `11` → `12` in `README.md:135,140,147`, `AGENTS.md:90`,
   `PUBLIC_REPO_AUDIT.md:90-101`. **(N-1 — the one item I consider genuinely required)**
2. Add `EnsureBelief` to the kernel surface list in `README.md:59-62`. **(N-2 / F-3)**
3. Regenerate `docs/M2_REPORT.md`'s 25/25 → 28/28 and "25 rows" → "28 rows"; fix
   `docs/M1_REPORT.md:15` (8/8 → 9/9) and `:33` ("seven" → eight). **(F-5 / F-3)**
4. Re-run `m2_accept.sh` **last** and commit `M2_TRANSCRIPT.md` + `M2_TEST.md` together so the
   pair matches. **(F-5 / F-10)**
5. Put the DSN into the README's two CLI examples. **(N-3)**
6. Soften "and its cascade" or point at B-07/B-20/B-22. **(N-6)**
7. Optional, in the same pass: `VERDICT: NO-OP` on `retracted 0` (N-5); README `:34`, `:72`,
   `:148` still speak of embedding columns and vector-similarity dedup, contradicting the new
   `:91-93` (F-23 leftovers); regenerate `M0_TRANSCRIPT.md` with timings (N-9).

Nothing in this list requires touching kernel, pipeline, normalize, derive, belief, intent, or
the schema. Items N-4, N-7, N-8 are recorded for adjudication, not requested as changes.

---

## Final verdict

**REWORK REQUIRED**

Justification, held to the standard the prompt sets — an actual remediation item still broken, or
a regression introduced:

- **F-5 is still broken in part**, and the specific claim "regenerated all receipts … from one
  current build/test run" is provably untrue: `docs/M2_REPORT.md` still says 25/25 against a
  28-row transcript, and the committed M2 transcript/test pair still comes from two different
  runs (B-21: 7 ms vs 6 ms, where a genuine pass matches exactly, as my own run demonstrates).
- **F-23 is still broken in part**: the judge-facing README still describes the kernel as seven
  functions — the exact defect F-3 existed to remove — and still contradicts itself on embeddings
  and vector dedup.
- **A new untruth was introduced**: the tree runs 85 tests and four documents, including
  `fix.md`'s own verification table, say 84.

What this verdict is *not* saying: the ledger, the gates, and the new capability are sound. I
re-executed the entire acceptance machinery and it is green — build, vet, gofmt, E2, I-7 = 7,
M0 16/16 with byte-identical determinism, PREPARE 9/9, M2 28/28 twice, 85/85 tests, CLI PASS with
byte-identical replay, and a falsification path that really does move a promoted belief to
`retracted`, cancel its live intent in the same transaction, keep the I-5 audit at 0, and refuse
to cross a scenario boundary. Solvent has crossed most of the way from *"working core with broken
scaffolding"* to *"verified core with truthful contracts, receipts, and a judge-playable
falsification path"*: the core is verified, the contracts are truthful, the falsification path is
playable. What is not yet true is **"truthful receipts"** — three numbers and one function list
short of it. That is an afternoon of documentation work, not a rework of the remediation.

---

## Appendix — environment and cleanup

Cluster: single-node CockroachDB CCL v26.2.0 on `localhost:26260` (HTTP 8081), started with
`scripts/m0_up.sh`, stopped with `scripts/m0_down.sh` after the review.
Scratch databases `fable_demo`, `fable_demo2`, `fable_demo3`, `fable_demo4` created for CLI
testing and dropped; `fable_test` and `fable` were used only by the project's own machinery.
`docs/*.md` were checksummed before the run and restored afterward (`sha256sum -c` clean);
`git status --short` after the review diffs empty against the baseline taken before it.
