# Adversarial Code Review — Solvent (frozen codebase)

**Reviewer:** Claude (Fable 5), adversarial review against the specs in `docs/` and `plans/`
**Date:** 2026-08-10
**Scope:** Does the frozen code at this commit live up to the expectations its own specifications set?
**Method:** Full dynamic verification — every "this passes" claim was re-executed where possible, against a fresh single-node CockroachDB v26.2.0 started with `scripts/m0_up.sh`. No tracked file was modified by this review (one file regenerated as a test side effect was restored; see F-10).

---

## Verdict

**The ledger is real. The gates around it are red.**

The core promise — invariants I-1..I-8 enforced by schema constraints and a composite foreign key, never by application code — is genuinely delivered and was re-proven live during this review: all 84 tests pass, all 16 M0 probes pass with byte-identical determinism, the 28-case M2 behavioral suite is green twice consecutively, the PREPARE sweep is green, and the Wave-4 demo CLI reproduces its ratified receipt including byte-identical replay. Every ratified rework item that touches code (Wave 1 B1/B2/S1–S3, Wave 2 F1–F4, Wave 3 P1–P3, Wave 4 F1/F2/F4/P5 + CLI cleanup) was verified implemented.

But the project's own acceptance machinery no longer passes: `scripts/check_i7.sh` fails (hardcoded count of 6 `ExecuteTx` sites vs. 7 actual after `EnsureBelief`), `gofmt -l` is non-empty (4 files), and therefore **both `m1_accept.sh` and `m2_accept.sh` exit 1 today** — while the frozen reports in `docs/` state they exit 0. The kernel's compile-time contract check no longer describes the kernel's API. Several frozen receipts are provably stale against the frozen code. The headline capability — the atomic retract cascade — is exercised only by kernel tests; no shipped binary can demonstrate it. And the repo carries a 13.7 MB compiled binary plus a stale draft of the kernel as tracked files.

Nothing found here indicates the *behavior* diverges from the invariants. What has decayed is the correspondence between the code and the evidence trail that is this project's stated identity ("opinions are not receipts").

---

## Methodology

Normative baseline (in precedence order, per `IMPLEMENTATION_CONTRACT.md`'s "this file wins"):

1. `IMPLEMENTATION_CONTRACT.md` (§4 kernel API, §5 invariants I-1..I-8, §8 non-goals) + frozen DDL `db/001_schema.sql`
2. Milestone specs and reports: `plans/M0/plan3_pkg02.md`, `plans/M1/plan2.md`, `plans/M2/M2_IMPLEMENTATION_PLAN.md`, `docs/M0_*`, `docs/M1_*`, `docs/M2_*`
3. Wave specs and ratified reworks: `DATA_PIPELINE_SPEC.md`, `plans/PRD/plan3.md`, `WAVE2/3/4_IMPLEMENTATION_PLAN.md`, `plans/PRD/wave1_rework.md`, `WAVE2_REWORK.md`, `WAVE3_REWORK.md`, `WAVE4_REWORK.md`, and the approval rulings
4. `plans/M3demo/plan_M3demo.md`, `plans/PHASE1/*`, `AGENTS.md`, `README.md`, `PUBLIC_REPO_AUDIT.md`

Deviations already accepted by the Tech Lead are **not** re-flagged; they are listed in the appendix so this review can't be accused of grading against superseded text.

Dynamic checks executed (receipts in the appendix):

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...` | exit 0, exit 0 |
| `gofmt -l cmd internal` | **4 files** (non-empty) |
| `scripts/check_isolation.sh` | PASS |
| `scripts/check_i7.sh` | **FAIL** — "7 crdb.ExecuteTx call(s) / expected 6" |
| `scripts/m2_accept.sh` | **exit 1** at the gofmt step (verbatim receipt below) |
| `go test -count=1 -p 1 ./...` (live cluster, `fable_test`) | **all 6 test packages `ok`** |
| `cmd/m1verify` PREPARE sweep (output redirected off-tree) | **GREEN — 9/9** (frozen doc says 8/8) |
| `cmd/m0verify --reset --apply --no-timings` × 2 (off-tree) | **16/16 GREEN**, byte-identical below the volatile fence (E1 holds) |
| `cmd/solvent -reset -scenario …` then replay without reset | matches the ratified WAVE4_REWORK receipt; **replay byte-identical**; contradiction `slog.Warn` emitted; `audit_live_on_nonpromoted: 0`; `VERDICT: PASS` |

`m1_accept.sh` was **not** executed verbatim because it overwrites the frozen receipt `docs/M1_BUILD.md` *before* running its checks (`scripts/m1_accept.sh:39`); its outcome is fully determined by its components, all run individually above — it blocks at the gofmt step, same as `m2_accept.sh`.

---

## What holds (verified, with credit where due)

- **The schema is the strongest artifact in the repo and does what the contract says.** `db/001_schema.sql` enforces I-1/I-2 (`promoted_is_debt_free`, with the correct `coalesce(array_length…)` NULL handling), I-3 (`live_requires_promoted` + composite FK `gate`), and I-4 (`ON UPDATE CASCADE` re-triggering the child CHECK). Four tables exactly; no embedding column, per the ratified R2/F3 position.
- **The kernel honors its hardest promises.** All writes route through `crdb.ExecuteTx` (7 sites); `AuditLiveOnNonPromoted` is the single permitted pool read (`internal/kernel/kernel.go:156`); `errors.go`'s `wrapIf` takes the SQLSTATE as a parameter so classification-before-wrapping is structural and a retryable 40001 can never be masked (M1-R7 discharged by construction); the package imports no driver; `descendantsCTE` uses `UNION` (not `UNION ALL`), which B-21 proves terminates a real 2-cycle; cancel-precedes-retract inside one transaction (`kernel.go:126-135`), and B-16 proves a blocked cascade rolls back even the cancels already issued.
- **B-18 deserves specific mention:** it proves retry-injection is actually live via a control transaction on a DSN-scoped separate pool before asserting `Promote` succeeds through it — rigor most test suites skip.
- **The M0/M2 verification machinery reproduces today.** Fresh cluster → 16/16 probes, E1 determinism byte-identical, M2 suite 28/28 twice, `testdb.Reset`'s `_test`-suffix guard genuinely refuses non-test databases and prints the redacted DSN first.
- **README's test claim is accurate.** 88 `func Test*` minus 4 `TestMain` = **84 tests, all passing**, matching `README.md:137` and the per-wave table (11/16/48/9).
- **The ratified reworks are implemented, not just recorded.** Wave 1: zero clock reads (`IngestedAt` left zero and asserted so), no fabricated `ObservedAt`, word-boundary severity regexes (`normalize.go:495-500`), unexported `knownMaintainers` + `IsMaintainer` accessor, pinned golden hash `3a4d…` present (`normalize_test.go:17`). Wave 2: maintainer gate with contradiction bypass (`derive.go:91-113`), merged-PR gate (`derive.go:163-171`), unconditional `Accommodated` releases (F3, `derive.go:77-86`), `(bool, bool)` payload extraction (F4, `derive.go:224-245`). Wave 3: `ErrPromotionBlocked → nil` with `ctx.Err()` checked first (`belief.go:84-91`), contradiction `slog.Warn` + no ledger mutation (`belief.go:39-48`), union-in-table-order debt mapping (`mapping.go:36-54`). Wave 4: `Run()` Phase-4 warnings (F1, `pipeline.go:362-369`), `Result.Normalized` populated for all results (F2), sorted merged claims (P5, `pipeline.go:230-234`), the redundant CLI intent loop removed, `ProposeIfNew` replay-idempotent — demonstrated by a byte-identical replay transcript.

---

## Findings

### Blocking — the acceptance gates are red while the frozen reports say green

**F-1. `scripts/check_i7.sh` hardcodes `EXPECT_TX=6`; the kernel has 7 `crdb.ExecuteTx` sites. The I-7 gate fails today.**
- Spec: `IMPLEMENTATION_CONTRACT.md` §7 M1 ("Grep confirms zero raw write calls outside `crdb.ExecuteTx`"); `docs/M2_REPORT.md` §1 ("`./scripts/m2_accept.sh` exits 0"); `docs/M1_REPORT.md` ("`./scripts/m1_accept.sh` exits 0").
- Code: `scripts/check_i7.sh:14` (`EXPECT_TX=6   # Enter, AddEvidence, RetireDebt, Promote, IntentOnPromoted, RetractCascade`); the seventh site is `EnsureBelief` at `internal/kernel/kernel.go:168`, added in Wave 3 with approval, without updating the gate.
- Evidence (executed): `I-7 FAIL / observed: 7 crdb.ExecuteTx call(s) / expected: 6`, exit 1.
- Consequence: `m1_accept.sh` and `m2_accept.sh` both abort. The M1/M2 "gate green" claims in `docs/` are true of a past commit but false of the frozen one. Note the irony the gate was built to prevent: its own negative-control test (three deliberate violations, `docs/M1_REPORT.md`) proved it catches *additions of raw writes* — it also catches legitimate additions, and Wave 3 shipped without re-running it.

**F-2. `gofmt -l cmd internal` is non-empty; both acceptance gates hard-block on it.**
- Spec: `scripts/m1_accept.sh:88-94` and `scripts/m2_accept.sh:25-32` treat any output as BLOCKED; `docs/M1_BUILD.md` records it empty.
- Evidence (executed): `internal/derive/derive.go`, `internal/derive/derive_test.go`, `internal/normalize/normalize.go`, `internal/normalize/types.go`. Verbatim `m2_accept.sh` run: `M2 BLOCKED / criterion: gofmt -l cmd internal is empty`, exit 1 — before the suite even runs.
- Aggravator: `README.md:140-146` ("Before you commit") lists build, vet, and tests — but omits the gofmt check its own gates enforce, which is plausibly how the drift shipped.

**F-3. The kernel's compile-time contract no longer describes the kernel, and the supreme contract document was never amended.**
- Spec: `internal/kernel/contract.go:8-13` — "a signature that drifts from §4 fails the build rather than a review"; `docs/M1_review.md` D-043 — "Kernel API is now considered frozen"; `plans/PRD/wave3_qa.md` approved exactly one addition, `EnsureBelief`.
- Code: `Contract` (`contract.go:16-24`) lists seven methods; `*Store` has eight. `EnsureBelief` was never added, so the advertised drift-detection guarantee silently excludes the newest method. `IMPLEMENTATION_CONTRACT.md` §4 — the document that "wins" over every other — still defines seven functions and was never updated to record the approved eighth; `docs/M1_REPORT.md` §1 ("All seven contract §4 functions… there is no stub") and `plans/PHASE1/05_REPOSITORY_UX.md` ("7 functions, zero raw writes") are likewise stale.
- Why this is blocking rather than cosmetic: this project's differentiator is that its claims are mechanically enforced. The one mechanical enforcement of the API surface was allowed to rot at exactly the moment the surface changed.

### High

**F-4. `RetractCascade` — the headline capability — is unreachable from any shipped binary.**
- Spec: `README.md:115-119` — the demo "must prove exactly three things… the **cascade** (new evidence falsifies a belief and downstream intents cancel atomically)"; `plans/PHASE1/01_PRODUCT_VISION.md` (the entire thesis); `plans/M3demo/plan_M3demo.md` (the reveal).
- Code: the only callers of `kernel.RetractCascade` outside `internal/kernel/kernel_test.go` are — none. `belief.Process` deliberately defers it (W3-P2, waived); `pipeline.Run` and `cmd/solvent` never call it; the M3demo race harness does not exist (only the PostgreSQL-era `proof/harness/cells.sh`).
- What is and isn't waived: W3-P2 legitimately waived *automatic retraction inside `Process`*. Nothing waived the absence of *any* production path. The Wave-4 CLI transcript can show the gate and the audit, but the falsify-and-cascade beat — the product's reason to exist — currently lives only in test code. The cascade itself is excellent and proven (B-07/16/19/20/22/24); it just has no stage.

**F-5. Frozen verification receipts are stale against the frozen code — re-execution contradicts four of them.**
- `docs/M1_PREPARE.md` says "**8/8** statements prepared." Re-run today (output off-tree): "**GREEN — 9/9** statements prepared, none skipped" — `SQLCatalog()` (`internal/kernel/sql.go:111-125`) now returns 9 entries including `ensure_belief`.
- `docs/M1_I7.md` says "PASS — 6 `crdb.ExecuteTx` write sites" with a line-number table that predates `EnsureBelief`. The script that generates it currently fails (F-1), so this receipt cannot be regenerated as-is.
- `docs/M2_REPORT.md` says "**25/25 cases pass**" and lists the transcript at "25 rows"; `docs/M2_TRANSCRIPT.md` says "**GREEN — 28/28**" and has 28 rows (the three `W3-Ensure-*` cases). The report predates Wave 3's additions to the suite it reports on.
- `docs/M2_TEST.md` (the captured `go test -v`, both runs) contains **no** `EnsureBelief` test at all, and its B-21 timing (5 ms) differs from the transcript's (6 ms) — `M2_TEST.md` and `M2_TRANSCRIPT.md` were captured from different builds, despite `m2_accept.sh` writing both in one pass.
- Also stale: `cmd/m1verify/main.go:48` — the `--skip` diagnostic hardcodes "expected: 8 statements prepared" while the success path computes 9 dynamically.
- Mitigating: the *current* behavior is green (verified — suite 28/28 twice, PREPARE 9/9, M0 16/16 with E1 determinism). The receipts are stale, not falsified; but a project whose premise is "receipts, not opinions" is now shipping receipts that its own tooling would refuse to reproduce.

**F-6. `cmd/solvent` contradicts its own spec on configuration, and its default target is the test database.**
- Spec: `WAVE4_IMPLEMENTATION_PLAN.md` §"cmd/solvent" — `-dsn string  CockroachDB connection string (env: FABLE_DSN)`.
- Code (`cmd/solvent/main.go:28,35-37,42`):
  - The default comes from `testdb.DSN()`, which reads **`FABLE_TEST_DSN`** (`internal/testdb/testdb.go:29-34`) — setting `FABLE_DSN`, as the plan and the error message instruct, silently does nothing.
  - The error branch `fail("no DSN: pass --dsn or set FABLE_DSN")` is unreachable: `testdb.DSN()` always returns at least `DefaultDSN`.
  - `DefaultDSN` is `…/fable_test` — the demo binary defaults to the behavioral test suite's throwaway database (observed live: running the CLI with `-reset` printed the `fable_test` reset banner and dropped it).
  - The fixture directory is hardcoded as a CWD-relative path (`filepath.Join("internal","derive","testdata")`) — the demo only works when invoked from the repo root; `go install` or running the (committed!) `./solvent` binary from anywhere else fails on startup.
- The transcript itself matches the ratified receipt exactly (verified, including `belief_id` presence, which `WAVE4_REWORK.md`'s receipt sanctions), so this is a packaging/config defect, not a behavioral one — but it's a defect in the one artifact judges are told to run.

### Medium

**F-7. `Result.DebtItems` contains `debtLen` empty strings — the field's content is a lie; only its length is real.**
- Spec: `WAVE4_IMPLEMENTATION_PLAN.md` defines `DebtItems []string`; the struct comment says "remaining debt after processing" (`pipeline.go:35`).
- Code: `internal/pipeline/pipeline.go:112` and `:345` — `make([]string, debtLen)`. `cmd/solvent/main.go:102` prints `len(r.DebtItems)`, so the transcript is correct by accident; any future consumer that reads the elements gets `["", "", …]`.

**F-8. Test case B-15 passes unconditionally and its text is stale.**
- Code: `internal/kernel/kernel_test.go:806-815` — `rec.check(t, true, …)` with `Expected: "asserted statically: 6 ExecuteTx write sites"`.
- The honesty is real (its `Observed` says "NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite") — but that statement is currently **false twice over**: the script it defers to fails (F-1), and `m2_accept.sh` never reaches the suite. The transcript still records B-15 as PASS in a 28/28 green run. A vacuous case whose stated backing gate is red is worse than no case: it manufactures the appearance of I-7 coverage.

**F-9. `evidenceExists` swallows the query error — a transient DB failure silently degrades into a duplicate-insert path.**
- Code: `internal/belief/belief.go:99-105` — `_ = db.QueryRowContext(…).Scan(&count); return count > 0`.
- The dedup TOCTOU window is explicitly waived (`WAVE3_IMPLEMENTATION_PLAN.md` §18 R5); the *discarded error* is not part of that waiver and contradicts Wave 3's own error-handling contract ("Other errors → returned immediately (fail loudly)", §4).

**F-10. `go test ./internal/kernel` unconditionally overwrites the frozen receipt `docs/M2_TRANSCRIPT.md`.**
- Observed live: running the suite for this review left `docs/M2_TRANSCRIPT.md` modified (regenerated with new timestamps/timings; content otherwise identical — 28/28 PASS. The file was restored from git afterward; the captured diff shows only `generated_at` and per-case `elapsed_ms` drift).
- Consequence: any contributor who runs the tests dirties a frozen evidence artifact; conversely, the "frozen" receipt is whatever the last test run happened to write. Evidence artifacts that are also build outputs need either a redirect flag or a freeze that the suite respects.

**F-11. `aggregateDeriveResults` is deterministic today only by structural accident, and its CVE merging has a gap.**
- Spec: `WAVE4_REWORK.md` F4 (CVE-aware aggregation; "evidence from one CVE must never contribute to another CVE's debt retirement") and P5 (sorted output, no reliance on map iteration order).
- Code (`internal/pipeline/pipeline.go:133-250`): the final output is sorted (P5 met, `:230-234`), but the primary-CVE lookup iterates a Go map and `break`s on first match (`:173-179`, `:206-211`). Currently at most one entry can carry the primary CVE (same-CVE claims are merged into it on arrival), so the scan is deterministic — but the determinism depends on that non-obvious invariant, in the exact function the rework singled out for map-order bugs. Meanwhile two *different-text* claims sharing a *non-primary* CVE are keyed separately (`:192-200`) and never merged — evidence attaching to "A is vulnerable to CVE-X" will not reach "B is vulnerable to CVE-X". The shipped fixtures can't trigger either path (verified: single-CVE corpus, kev fixture sorts first), and the two-CVE regression test required by F4 exists and passes; the gap is latent, not live. Two dead `_ = text` statements (`:176`, `:209`) mark where the code was reworked.

**F-12. `computeSHA256` silently returns `""` on marshal failure — in the provenance-integrity function — and the schema would accept it.**
- Code: `internal/normalize/normalize.go:426-433`; `db/001_schema.sql:44` has `content_sha256 TEXT NOT NULL` but no length/format CHECK, so the empty string inserts cleanly.
- Practically unreachable today (all hashed fields are strings/bools), but this is the single function whose entire purpose is content integrity, in a system whose pitch is provenance; it should be structurally incapable of producing an empty identity. Related: `AGENTS.md`/`README.md:104-105` claim "CI enforces" resolving `source_url` + matching `content_sha256` — no such CI exists in the repo.

**F-13. The release normalizer hardcodes etcd — a domain leak the spec's own layering forbids.**
- Spec: `DATA_PIPELINE_SPEC.md` §"internal/normalize" — the package is the *domain-neutral* transformation layer; `db/001_schema.sql:4` states the feed-agnostic principle for the schema.
- Code: `internal/normalize/normalize.go:311` (`https://github.com/etcd-io/etcd/releases/tag/%s`) and `:315` (`"etcd %s"`) are applied to **every** release regardless of input — any non-etcd release gets an etcd URL and subject. The kev/issue/PR/comment normalizers correctly derive these from payload fields; only `release` cheats. (The hardcoded `knownMaintainers` list is different: it is labelled a placeholder and is the sanctioned MVP trust boundary.)

**F-14. `EnsureBelief`'s "no TOCTOU" holds only under SERIALIZABLE, and nothing structural backs it.**
- Code: `internal/kernel/sql.go:39-53` — CTE find-or-create with no `ON CONFLICT`; the schema has no `UNIQUE (scenario_id, claim)`.
- Adjudication status: `WAVE3_REWORK.md` proved 24 concurrent goroutines produce one row *under the default isolation* and accepted the absence of a unique index. Under READ COMMITTED — which this project deliberately enables (`internal/m0/env.go` probes it; M3 is designed around it) — two concurrent `EnsureBelief` calls can both observe absence and both insert. Not a today-bug; a loaded gun for M3, worth a one-line comment on the function at minimum.

**F-15. M0 probe A3's receipt records a statement that never executed.**
- Code: `internal/m0/env.go:46` declares `stmt = "BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED; SHOW transaction_isolation"`, but the code runs `BeginTx` + `SET TRANSACTION ISOLATION LEVEL READ COMMITTED` + `SHOW` (`:52-61`). The `Statement` column of the A3 row in `docs/M0_TRANSCRIPT.md` is therefore SQL that was not run. Semantically equivalent, and the observed value is real — but in a transcript whose header is "every row is a receipt," one receipt is a paraphrase.

### Low

**F-16. Contradictions from GitHub issues can only ever match the title.** `normalize.go:87` sets `Assertion = g.Title`; `derive.go:147` matches `reproducesPattern` against `Assertion`. An issue whose *body* says "still vulnerable" derives nothing. The Wave-2 waivers covered the bare-`regression` keyword and `is_maintainer` on issues — not title-only scope. No fixture exercises the issue path at all.

**F-17. `TestPropose_OnUnpromotedBelief` asserts only `err != nil`** (`internal/intent/intent_test.go:106-109`), never `errors.Is(err, kernel.ErrActionOnUnpromoted)` — the sentinel is the entire point of that path, and `pipeline.IsActionOnUnpromoted` exists precisely to consume it. Several integration tests likewise discard read errors (`_ = shared.QueryRowContext…`), converting query failures into misleading zero-value assertions.

**F-18. `testdb.AcquireResetLock` spins forever** (`internal/testdb/testdb.go:110-120`): no timeout, no staleness check, no PID, 50 ms sleep. One `SIGKILL`ed test run leaves `/tmp/fable_test.reset.lock` behind and every subsequent `go test` hangs silently. `WAVE3_REWORK.md` documented the lock as test-infra; the unbounded, diagnostic-free hang is a distinct sharp edge. (This review checked for and would have removed a stale lock before running the suite; none was present.)

**F-19. `kernel.FullDebt` is an exported mutable slice.** `EnterBelief` copies it defensively (`kernel.go:53`), but a caller can mutate the package-level value before any copy is taken, corrupting every subsequent write. The contract requires an "exported value"; an accessor returning a copy, or an array, would honor it safely.

**F-20. Dead code.** `stripMarkdown` strips ``` ` ``` then ``` ``` ``` (`normalize.go:440-441` — the second is unreachable); `regressionKeywords` (`derive.go:16`) duplicates `noRegressionPattern` (`derive.go:15`), both checked with `||` at `derive.go:131`; `_ = text` twice in `pipeline.go` (F-11).

### Hygiene

**F-21. A 13.7 MB unstripped compiled binary (`solvent`) is tracked in git** (committed in `c54a881`), permanently in history. Root cause: `.gitignore` covers `*.exe`/`*.test`/`*.out` but not extensionless Linux binaries — exactly what `go build ./cmd/solvent` produces.

**F-22. The `files/` directory and `files.zip` triplicate tree content, including a boobytrap.** `files/001_schema.sql`, `files/02_lifecycle_and_invariants.sql`, `files/cells.sh`, `files/IMPLEMENTATION_CONTRACT.md` are byte-identical duplicates; `files/go.mod` is a stub; **`files/kernel.go` is a stale pre-`EnsureBelief` draft of the kernel** with no marker saying it is superseded — a plausible-looking wrong kernel for any future reader or tool that greps the repo. `files.zip` re-archives the same. Roughly 20 planning `.md` files at repo root duplicate `plans/PRD/` copies (with at least one meaningful divergence: root `WAVE4_IMPLEMENTATION_PLAN.md` is the revised edition; the `plans/PRD/` copy is older).

**F-23. The README contradicts the schema, the ratified F3 ruling, and itself — and `PUBLIC_REPO_AUDIT.md` certified it anyway.**
- `README.md:91`: "**Vector column is nullable and used for merge-not-fork**" — there is no vector column; `db/001_schema.sql:3` says "I-6: no embedding column"; test B-14 asserts its absence; `WAVE4_REWORK.md` F3 required documentation to state exactly the opposite (the F3 fix was applied to `WAVE4_IMPLEMENTATION.md:163` but README line 91 was never corrected).
- `README.md:100`: "deliberately **three-tables**-two-agents" vs. line 53 of the same file (correctly, four tables) and the contract §2 ("Exactly four tables").
- `PUBLIC_REPO_AUDIT.md` (commit `51d8082`, "public repo audit") checked line 53, declared "README.md: Already correct," and concluded "All documentation claims match the actual repository state" — it missed both lines 91 and 100, which were present in the audited commit (verified against `git show HEAD:README.md`). The audit's method (verify the specific claims someone thought to list) is exactly the failure mode this codebase's schema-enforced design exists to avoid.
- Smaller: the file's first line is literally `# README.md`; lines 2–4 address AI agents, and the judge-facing README specified by `plans/PHASE1/06_README_BLUEPRINT.md` (hero, 60-second version, `task wow`, receipts links) does not exist — the latter is acknowledged as unbuilt in `plans/PHASE1/08_SUBMISSION_STRATEGY.md` and is listed under waivers, but shipping an agent-orientation doc *as* `README.md` in a repo audited "public-ready" is a decision worth revisiting.

### Documentation-integrity notes (docs vs. docs, no code change implied)

- `docs/M0_review.md` D-027 **approves** displaying `ConstraintName` beside raw CHECK errors, yet `docs/M1_REPORT.md` §5 and `docs/M2_REPORT.md` §6 still call the question "open"/"awaiting the M5 ruling." The later reports never absorbed the earlier decision.
- I-6 is satisfied vacuously and the docs say so themselves (`docs/M0_REPORT.md` §4: "not the property the invariant was written to assert") — B-14 records PASS against an invariant whose subject does not exist. Sanctioned by D-030/F3; noted because the contract §3 text ("`belief.embedding` is nullable") was never amended and still contradicts the DDL it declares frozen.
- The M0 report's "16 probes" figure is **correct** — the frozen transcript table has 16 rows and a fresh run reproduces 16/16 (an earlier-draft discrepancy hypothesis was checked and discarded during this review).

---

## Explicitly not flagged (already adjudicated — appendix of waivers honored)

Contract §8 non-goals (extra tables/agents/feeds, stored confidence, fourth status, auth, second visualization, etc.); no embedding column and vector-similarity dedup deferred to Phase 2 (D-030, D-074, WAVE4 F3 — B-14 asserts absence by design); Wave 1 Req2 (semantic-content hashing) and Req3 (canonicalized `DomainPayload`); Wave 2's bare-`regression` non-trigger, `deriveFromGitHubIssue` skipping `is_maintainer`, and silent-nil `extractKeywordsFromPayload`; Wave 3's evidence-dedup TOCTOU, deferred automatic retraction in `Process` (W3-P2), and the knowingly-wrong aspirational comment at `internal/belief/belief.go:26` (recorded in `PUBLIC_REPO_AUDIT.md` §12 as frozen); Wave 4's `ProposeIfNew` concurrency deferral ("I would not touch it now"), fixtures-only ingestion, no graph/Taskfile/live-feed/AWS; `ConstraintName` unreachable from the driver-free kernel (M1-R6); the M3 isolation experiment absent from the Go suite (M2-R7); `snapshot`/`source_observed_at` columns never written — downstream of the Req3 waiver and the deferred M4 ingestor, noted only as a reminder that `NormalizedEvidence.DomainPayload/ObservedAt/Severity/Subject` are currently computed, tested, and discarded at the ledger boundary.

---

## Recommended remediation order (smallest changes that make the repo's claims true again)

1. `gofmt -w` the four files; add gofmt to README's "Before you commit" (unblocks both gates' first step). *(F-2)*
2. `scripts/check_i7.sh`: `EXPECT_TX=7` with `EnsureBelief` added to the comment; re-run `m1_accept.sh`/`m2_accept.sh` to regenerate `docs/M1_I7.md`, `docs/M1_BUILD.md`, `docs/M1_PREPARE.md` (9/9), `docs/M2_TEST.md`/`M2_TRANSCRIPT.md` from one build; update B-15's `Expected` text and `cmd/m1verify`'s `--skip` message. *(F-1, F-5, F-8)*
3. Add `EnsureBelief` to `kernel.Contract`; amend `IMPLEMENTATION_CONTRACT.md` §4 with the wave3_qa-approved eighth signature. *(F-3)*
4. Fix `cmd/solvent`: honor `FABLE_DSN`, make the fixture dir a flag with the current path as default, stop defaulting to `fable_test`. *(F-6)*
5. Populate `Result.DebtItems` with the actual items (one extra column in `queryBelief`). *(F-7)*
6. Return the error from `evidenceExists`. *(F-9)*
7. `git rm --cached solvent files.zip && git rm -r files/`; add extensionless-binary pattern (e.g. `/solvent`) to `.gitignore`. *(F-21, F-22)*
8. Fix README lines 91 and 100; re-run the public-repo audit with a diff-the-whole-file method. *(F-23)*
9. Give the demo a cascade: even a `-falsify <belief-id>` flag on `cmd/solvent` that calls `RetractCascade` and re-prints the audit would put the headline capability on stage. *(F-4)*

---

## Appendix — dynamic evidence log

All commands run 2026-08-10 from the repo root; cluster: single-node CockroachDB v26.2.0 on `localhost:26260` started via `scripts/m0_up.sh` (torn down after the review).

```
$ gofmt -l cmd internal
internal/derive/derive.go
internal/derive/derive_test.go
internal/normalize/normalize.go
internal/normalize/types.go

$ ./scripts/check_i7.sh ; echo exit=$?
I-7 FAIL
  criterion: every §4 write function routes through crdb.ExecuteTx
  observed:  7 crdb.ExecuteTx call(s)
  expected:  6
exit=1

$ ./scripts/m2_accept.sh ; echo exit=$?
== E2: harness isolation ==
E2 PASS: internal/kernel imports nothing from internal/m0
== build + vet + gofmt ==
M2 BLOCKED
  criterion: gofmt -l cmd internal is empty
  observed:  internal/derive/derive.go … internal/normalize/types.go
  expected:  no output
exit=1

$ go test -count=1 -p 1 ./...
ok  github.com/PithomLabs/solvent/internal/belief    1.489s
ok  github.com/PithomLabs/solvent/internal/derive    0.002s
ok  github.com/PithomLabs/solvent/internal/intent    1.176s
ok  github.com/PithomLabs/solvent/internal/kernel    1.762s
ok  github.com/PithomLabs/solvent/internal/normalize 0.009s
ok  github.com/PithomLabs/solvent/internal/pipeline  1.453s
# side effect: docs/M2_TRANSCRIPT.md regenerated (28/28 PASS, timings drift only) — restored via git checkout

$ go run ./cmd/m1verify --dsn 'postgresql://root@localhost:26260/fable?...' --out <off-tree>
PREPARE sweep green — 9/9 statements prepared.        # frozen docs/M1_PREPARE.md says 8/8

$ go run ./cmd/m0verify --reset --apply --no-timings --out <off-tree>   # run twice
**M0 GREEN** — 16/16 probes passed.
diff below-fence run1 vs run2: byte-identical          # E1 holds

$ go run ./cmd/solvent -reset -scenario 55555555-…-0009 ; # then replay without -reset
WARN pipeline: contradiction received, no ledger mutation source_url=https://github.com/etcd-io/etcd/issues/17234#… 
results: 2 · promoted: true · debt_items: 0 · intent: live
audit_live_on_nonpromoted: 0 · VERDICT: PASS
replay transcript: byte-identical to first run
```
