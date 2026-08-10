# FINAL ETCD IMPLEMENTATION REVIEW

**Reviewed commit:** `51cddf6` ("implementation phase closed")
**Spec:** `plans/post/etcd_dataspec2.md`
**Implementation report:** `plans/post/etcd_dataspec2_imp.md`
**Prior review:** `plans/post/DATASET_ADVERSARIAL_REVIEW.md`
**Method:** static inspection + live execution against CockroachDB (scratch DB `solvent_review` on `:26257`; test suite on `:26260/fable_test`) + authoritative source verification via the GitHub REST API
**Date:** 2026-08-10

---

# 1. Verdict

**REWORK REQUIRED**

Scoped precisely: **the implementation architecture is APPROVED.** Every mechanism the spec
asked for works, and I verified each one against live database state rather than CLI output. The
rework is confined to the **contents of five JSON fixtures** and **one missing CLI path**. No
schema change, no kernel change, no design change, no re-opened research.

---

# 2. Executive Assessment

The engineering is good. The frozen surfaces are genuinely untouched — `db/001_schema.sql`,
`internal/kernel/{kernel,sql,contract}.go`, `internal/belief/belief.go` and `internal/intent/`
are byte-identical to `fca8b86`. The two new source types are closed switch cases in both
`normalize` and `derive`. Phase 3.5 was removed exactly as specified, so the pipeline no longer
creates intents and replay is idempotent. `cmd/operator-review` calls only kernel functions — no
raw SQL, no bypass. Track 1's debt arithmetic matched the spec's prediction digit for digit
(5/4/4 debts), promotion was refused three times, `--action` gating is correct, `RetractCascade`
cancels-then-retracts and leaves `audit_live_on_nonpromoted = 0`, and scenario isolation held
under a deliberate cross-scenario attack. 89 tests pass, build/vet/gofmt/I-7 all clean.

What fails is the data, and it fails on the one axis this project cannot afford. Solvent's entire
pitch is provenance; `README.md` states the rule without qualification — *"No fabricated evidence
— ever. No invented IDs, advisories, or URLs. `external_feed` rows must have a resolving
`source_url`…"* — and spec §12 restates it as an adversarial acceptance criterion. Four of the
five fixtures violate it:

- The GHSA fixture carries the **real advisory ID** paired with the **summary and date of an
  entirely different, older CVE** — text lifted from the repo's own CVE-2024-24790 demo fixtures.
  The real advisory is "etcd: Authorization bypasses in multiple APIs"; the fixture says "Denial
  of service … multipart MIME parsing".
- The postmortem fixture's `source_url` returns **HTTP 404** (verified), its `fixed_version`
  (`v3.5.12`) appears nowhere in the real document, its date is off by eight months, and its
  `summary` describes a **different failure mode** than the one the postmortem documents.
- All three release fixtures carry hand-typed placeholder `commit_sha` values
  (`a1b2c3d4e5f6…`, `f6e5d4c3b2a1…`, `1a2b3c4d5e6f…`), and `commit_sha` is an **input to
  `content_sha256`** — so the provenance anchor of every release evidence row in the ledger is a
  hash over invented data. Release dates are also wrong by weeks to months.

Separately, **Track 2 cannot be run from this repository at all.** `kernel.EnterBelief` and
`kernel.AddEvidence` have zero callers in `cmd/`, and the combined flag surface of both binaries
offers no way to enter a `postulated` belief or attach evidence to one. Spec §5 Steps 1–2 have no
committed mechanism, yet the report marks both PASS.

I want to be fair to the prompt's instruction not to create a review loop, and I agree the
deadline matters. This is not a design objection and not polish — a judge who clicks the
postmortem link gets a 404, and a judge who compares the advisory summary to the advisory finds
the wrong vulnerability. The fix is small and mostly mechanical: correct five files with the real
values (supplied verbatim in §4), add one small entry path for Track 2, and add tests for the new
data path. Everything else stands as approved.

One constructive note: the **real** postmortem is materially *better* for the demo than the
invented one. Its "What went wrong" section states outright that *"etcd v3.5 was recommended for
production without enough insight on the production adoption"* — authentic, maintainer-authored
support for exactly the deployment-decision-invalidation story Track 2 tells. And the real
v3.5.28 release (2026-03-20T17:41:08Z) shipped **three hours before** the advisory went public
(2026-03-20T20:48:14Z) — a sharper fix-then-disclose beat than the fixture's invented three-month
gap.

---

# 3. Specification Conformance

| Area | Status | Evidence |
|------|--------|----------|
| Track 1 | **PASS (mechanics) / FAIL (data)** | Live run produced exactly the spec §4 prediction: 3 beliefs, 3 evidence rows, debts 5/4/4, all `entered`, 0 intents, audit 0. Claim strings trace correctly through `normalizeGitHubAdvisory` → `deriveFromGitHubAdvisory`. Data fails per F-1, F-3. |
| Track 1 replay | **PASS** | Re-run with no `--reset`: belief/evidence/intent counts `3/3/1` before and after, identical belief UUIDs. `EnsureBelief` dedups on `(scenario_id, claim)`; `evidenceExists` dedups on `(belief_id, content_sha256)`; pipeline never calls `IntentOnPromoted`. |
| Track 2 | **FAIL** | Pipeline half works (2 beliefs in scenario 2, postmortem carries all 6 debts per spec §9, no leakage into Track 1). But spec §5 Steps 1–2 — enter the `postulated` baseline, `AddEvidence` the v3.5.0 release — have **no implementation** (F-4). Data fails per F-2, F-3. |
| operator-review | **PASS** | `cmd/operator-review/main.go` retires each debt via `kernel.RetireDebt`, then `kernel.Promote`. Verified live: 5 debts retired, `status=promoted`, `debt` empty. Kernel calls only — no `db.Exec`, no raw SQL. |
| `--action` | **PASS** | Optional. Without it: promotion succeeded, `action_intent` count = **0**. With it: exactly **1** row, `state=live`, `belief_status=promoted`, correct `scenario_id`. `Promote` strictly precedes `IntentOnPromoted` (`main.go:87-97`); promotion failure calls `fail()` → `os.Exit(1)` before any intent. Corroborated by OR-1..OR-4. |
| `--falsify` | **PASS** | Uses `kernel.RetractCascade` unchanged; no alternative transaction mechanism. Verified live: belief `promoted`→`retracted`, dependent intent `live`→`cancelled`, `belief_status` propagated by `ON UPDATE CASCADE`, audit 0 in both scenarios, Track 1 untouched. Wrong-scenario falsify is a clean `NO-OP` (`retracted 0`). |
| provenance | **FAIL** | `manifest.json` digests match all 5 fixtures exactly (recomputed with `sha256sum`) — the manifest mechanism is correct and is properly *not* confused with `evidence.content_sha256`. But the **content being hashed is fabricated** (F-1, F-2, F-3), one `source_url` 404s, and `ATTRIBUTION.md` repeats the broken URL (F-11). |
| invariants | **PASS** | `git diff --stat fca8b86 HEAD` empty for `db/`, `kernel.go`, `sql.go`, `contract.go`, `belief.go`, `intent/`. Zero `belief_edge` writes (`belief_edge` row count = 0 after full run). Six-item `FullDebt` untouched. `scripts/check_i7.sh`: 7 ExecuteTx sites, 0 raw writes, 1 permitted pool read. Status/state vocabularies unchanged. |
| tests | **FAIL** | 89 tests pass, 0 fail — but **zero** of them exercise the new data path. No test file references `etcd_real`, `github_advisory`, `postmortem`, `normalizeGitHubAdvisory`, or `deriveFromGitHubAdvisory` (F-5). |

**Spec defects encountered (not implementation faults, listed so they are not mis-assigned):**
spec §2 asserts a `UNIQUE(belief_id, content_sha256)` on `evidence` that does not exist — dedup is
the Go-side `evidenceExists` check with an accepted TOCTOU window; spec §11's expected JSON
transcript has never existed (the CLI emits a text transcript, and the implementation correctly
left frozen Wave 4 alone); spec §4's claim example names `CVE-2026-33413` while §8 specifies the
GHSA ID — the implementation follows §8, which is the defensible reading.

---

# 4. Findings

## F-1 — Advisory fixture pairs a real ID with a different vulnerability's content

- **Severity:** BLOCKER
- **Location:** `internal/derive/testdata/etcd_real/track1/ghsa_advisory.json:2-5`
- **Evidence:** The fixture declares `ghsa_id: "GHSA-q8m4-xhhv-38mg"` with
  `summary: "Etcd: Denial of service vulnerability in etcd through multipart MIME parsing"`,
  `severity: "critical"`, `published_at: "2026-06-05T00:00:00Z"`.
  Authoritative values from `GET https://api.github.com/advisories/GHSA-q8m4-xhhv-38mg`:

  | field | real | fixture |
  |---|---|---|
  | `summary` | `etcd: Authorization bypasses in multiple APIs` | Denial of service … multipart MIME parsing |
  | `severity` | `high` | `critical` |
  | `published_at` | `2026-03-20T20:48:14Z` | `2026-06-05T00:00:00Z` |
  | `cve_id` | `CVE-2026-33413` | (absent) |
  | package | ecosystem `go`, name `go.etcd.io/etcd/v3` | ecosystem `Go`, name `go.etcd.io/etcd` |

  The substituted text is not arbitrary — it is the repo's own **CVE-2024-24790** demo content.
  `grep -rn "multipart MIME"` returns 11 hits across `internal/normalize/testdata/kev_entry.json`,
  `github_issue.json`, `github_pr.json`, `internal/derive/testdata/maintainer_fixes.json` and
  others, all belonging to the older synthetic scenario. The fixture's `published_at` month/day
  (`06-05`) matches `kev_entry.json`'s `dateAdded: "2024-06-05"`. The repo's own prior researched
  artifact `plans/post/etcd_demo_fixture.json:10-11` already recorded the correct pair
  (`CVE-2026-33413` / "etcd: Authorization bypasses in multiple APIs").
- **Why it matters:** Violates `README.md`'s absolute no-fabricated-evidence rule and spec §12's
  "All claims are honest representations of the underlying evidence". The fabrication is
  load-bearing for the demo: the belief the judge reads —
  `Go/go.etcd.io/etcd >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg` — is anchored to a
  `source_url` whose page describes a different vulnerability entirely. In a provenance system,
  that is self-refuting.
- **Required correction:** Replace `summary` with `etcd: Authorization bypasses in multiple APIs`,
  `severity` with `high`, `published_at` with `2026-03-20T20:48:14Z`, and the package with
  ecosystem `go` / name `go.etcd.io/etcd/v3`. Recompute `manifest.json`. (The
  `>=3.5.0, <3.5.28` range is a fair semver rendering of the real
  `>= 3.5.0-alpha.0, <= 3.5.27, first_patched 3.5.28` — keep it.)

## F-2 — Postmortem fixture is substantially invented and its `source_url` 404s

- **Severity:** BLOCKER
- **Location:** `internal/derive/testdata/etcd_real/track2/postmortem_v35.json:2-7`
- **Evidence:** Verified against
  `https://raw.githubusercontent.com/etcd-io/etcd/main/Documentation/postmortems/v3.5-data-inconsistency.md`:

  | field | real | fixture |
  |---|---|---|
  | date | `2022-04-20` (document header) | `2022-12-06T00:00:00Z` |
  | root cause | "Code refactor in v3.5.0 resulted in **consistent index** not being saved atomically. Independent crash could lead to committed transactions not reflected on all members." | "incorrect **revision counting** in the apply pipeline. A **learner member receiving a snapshot** could compute a different revision than the leader" |
  | `fixed_version` | not stated as a single version; v3.5.0's crash bug fixed in v3.5.1, main fix ~2 weeks later (commonly cited v3.5.3, per `plans/post/etcd_demo_fixture.json:50`) | `v3.5.12` — appears **nowhere** in the document |
  | affected | v3.5.0 onward (introduced by the v3.5.0 refactor); prior artifact records 3.5.0/3.5.1/3.5.2 | `v3.5.x` (overbroad) |

  URL resolution, measured:
  ```
  HTTP 200  .../postmortems/v3.5-data-inconsistency.md   (correct spelling)
  HTTP 404  .../postmortems/v35-data-inconsistency.md    (fixture's spelling)
  ```
  The correct spelling is already present in the repo at
  `plans/post/etcd_demo_fixture.json:43`; the fixture and `ATTRIBUTION.md:29` both use the
  corrupted variant. The 404 URL reaches the ledger — I confirmed the `evidence.source_url` row
  after the Track 2 run.
- **Why it matters:** A non-resolving `source_url` on an `external_feed` row is the single
  explicit prohibition in `README.md`, and spec §12 lists "No fabricated URLs in fixtures". The
  invented summary describes a different failure mode, so the fixture would mislead anyone who
  reads it as documentation. Track 2's entire invalidating-evidence basis rests on this record.
- **Required correction:** Fix `source_url` to `…/postmortems/v3.5-data-inconsistency.md`
  (in the fixture **and** `ATTRIBUTION.md`); set `published_at` to `2022-04-20T00:00:00Z`;
  set `affected_versions` to the real range (`v3.5.0–v3.5.2`); set `fixed_version` to `v3.5.3`
  or drop the field's claim to what the document supports; replace `summary` with the
  document's own wording ("Code refactor in v3.5.0 resulted in consistent index not being saved
  atomically…"). Recompute `manifest.json`.

## F-3 — Fabricated `commit_sha` values feed `evidence.content_sha256`

- **Severity:** BLOCKER
- **Location:** `internal/derive/testdata/etcd_real/track1/release_v3527.json:4`,
  `track1/release_v3528.json:4`, `track2/release_v350.json:4`
- **Evidence:** The three fixtures use hand-typed placeholder patterns —
  `a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2`, `f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5`,
  `1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b`. Real values from
  `GET /repos/etcd-io/etcd/git/ref/tags/<tag>`:

  | tag | real tag SHA | real `published_at` | fixture `released_at` |
  |---|---|---|---|
  | v3.5.0 | `43b9aa5f9569412d60097cdd07eab7910fd282ab` | `2021-06-15T22:29:48Z` | `2021-06-16T12:00:00Z` |
  | v3.5.27 | `ee7ceab4e0ee88901cb46460579d1c22ee5ddb41` | `2026-02-13T18:33:16Z` | `2026-03-15T12:00:00Z` |
  | v3.5.28 | `8189e841fbff21101eb58ed97948ecc907690538` | `2026-03-20T17:41:08Z` | `2026-06-10T12:00:00Z` |

  `commit_sha` is a hash input: `internal/normalize/normalize.go:339-345` hashes
  `{version, commit_sha, prerelease}` into `ContentSHA256`, which `kernel.AddEvidence` writes to
  `evidence.content_sha256`.
- **Why it matters:** The provenance anchor of every release evidence row in the ledger is a
  SHA-256 over invented data. `README.md` requires `external_feed` rows to have "a resolving
  `source_url` **and a matching `content_sha256`**"; a hash over a fake commit matches nothing
  upstream, so the ledger's cryptographic claim is unverifiable by construction. This is the one
  fabrication that propagates all the way into the database rather than staying in the fixture.
- **Required correction:** Substitute the three real tag SHAs and the three real `published_at`
  timestamps above, then recompute `manifest.json`. (Bonus: the corrected dates give a stronger
  narrative — v3.5.28 shipped three hours before the advisory was published, same day.)

## F-4 — Track 2 has no committed execution path

- **Severity:** BLOCKER
- **Location:** `cmd/` (absence); spec §5 Steps 1–2, §11 Track 2 Step 2
- **Evidence:** Spec §5 requires
  `kernel.EnterBelief(scenarioID2, "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)", postulated)`
  followed by `kernel.AddEvidence(...)`. Measured:
  ```
  $ grep -rn "EnterBelief\|AddEvidence" --include=*.go cmd/
  NONE in cmd/
  ```
  Flag surfaces: `solvent` exposes `-dsn -falsify -fixtures -out -reset -scenario -schema`;
  `operator-review` exposes `-action -belief -debt -dsn -scenario`. Neither can create a belief
  with `claim_type='postulated'`, and neither can attach evidence to a chosen belief. The only
  `postulated` beliefs in the repo are created by tests (`internal/kernel/kernel_test.go:444`)
  and raw-SQL probes (`internal/m0/gate.go:43,78`). Spec §11 Step 2 defers to a "demo script"
  that does not exist in the repository.
- **Why it matters:** Reproducibility failure on the headline scenario. Track 2 is the retraction
  story — the demo's strongest beat — and it cannot be reproduced from a clean checkout by any
  documented command. To verify `RetractCascade` at all I had to substitute the pipeline-created
  v3.5.0 release belief as a stand-in baseline (§5). It also means the report's "Enter baseline
  postulated belief — PASS" and "Attach evidence — PASS" describe an unrecorded ad-hoc action.
- **Required correction:** Add the missing operator entry path — smallest form is two flags on
  `cmd/operator-review` (or a sibling `cmd/operator-decision`): `--enter-claim <text>
  --claim-type postulated` returning the new belief UUID, and `--evidence-url <url>
  --evidence-sha <sha>` calling `kernel.AddEvidence`. Both are existing kernel functions; no
  kernel or schema change. Then document the exact Track 2 command sequence.

## F-5 — Zero test coverage for every newly introduced data-path behavior

- **Severity:** IMPORTANT
- **Location:** `internal/normalize/normalize_test.go`, `internal/derive/derive_test.go`
  (both unmodified: `git diff --stat fca8b86 HEAD -- '*_test.go'` lists only
  `internal/kernel/operator_review_test.go` and a 4-line `pipeline_test.go` edit)
- **Evidence:**
  ```
  $ grep -rn "etcd_real|GitHubAdvisory|SourcePostmortem|github_advisory|postmortem" --include=*_test.go .
  NONE
  ```
  Untested: `normalizeGitHubAdvisory`, `normalizePostmortem`, `deriveFromGitHubAdvisory`,
  `deriveFromPostmortem`, the `github_advisory` `DebtMapping` entry, the five new
  `sourceTypeMap` entries, and real-fixture processing end-to-end. The four new tests
  (OR-1..OR-4) exercise `EnterBelief`/`RetireDebt`/`Promote`/`IntentOnPromoted` — all
  pre-existing kernel functions — with synthetic claims.
- **Why it matters:** ~205 lines of new parsing and derivation logic ship with no automated
  guard. F-1/F-2/F-3 are exactly the class of defect a fixture-processing test would have caught,
  and F-6 below is a live bug in untested code. Spec §12's technical criteria are satisfied only
  because they count *existing* tests.
- **Required correction:** Add table-driven tests for both normalizers (valid input, each
  required-field-missing case) mirroring the existing `normalize_test.go` style; add derive tests
  asserting the exact claim strings; add one pipeline test that runs `track1/` and `track2/` and
  asserts belief count, claim text, debt sets, and scenario separation.

## F-6 — Advisory's declared severity is silently discarded; `critical`/`high` becomes `info`

- **Severity:** IMPORTANT
- **Location:** `internal/normalize/normalize.go:508` (`Severity: classifySeverity(g.Summary, "")`)
- **Evidence:** Spec §7 requires "Severity: mapped from advisory severity
  (critical/high/medium/low/info)". The implementation ignores `g.Severity` for the
  `NormalizedEvidence.Severity` field — it appears only in the hash input and `DomainPayload` —
  and instead re-derives severity from the summary text. `classifySeverity`
  (`normalize.go:521-535`) matches `\b(critical|rce|remote code execution)\b`, then
  `\b(high|exploit|actively exploited)\b`, etc. Neither the fixture's summary nor the real
  summary ("etcd: Authorization bypasses in multiple APIs") contains any of those tokens, so the
  result is `info` — for an advisory the source rates **high**.
- **Why it matters:** A structured, authoritative field is thrown away in favour of keyword
  guessing on prose, silently downgrading a high-severity advisory to informational. It is a
  direct spec violation, and it survives F-1's correction (the real summary also scores `info`).
  Severity is not persisted to `evidence`, so ledger impact is nil — but it is wrong in the
  transcript and in any future consumer.
- **Required correction:** Map `g.Severity` through the existing `Severity*` constants with a
  small normalizing switch, falling back to `classifySeverity` only when the field is absent.

## F-7 — Transcript reports `intent: live` for beliefs that have no intent

- **Severity:** IMPORTANT
- **Location:** `cmd/solvent/main.go:273-281` (`intentState`), enabled by the Phase 3.5 removal in
  `internal/pipeline/pipeline.go`
- **Evidence:** `intentState` returns `r.IntentState` if set, else infers `"live"` from
  `r.Promoted`. Removing Phase 3.5 means `Result.IntentState` is now **never** assigned by
  `Run()`, so the inference is the only live path. Demonstrated on the scratch database — belief
  `55e1d474` promoted via `operator-review` **without** `--action`:
  ```
  ground truth:  55e1d474 | promoted | live_intents = 0
  transcript:    belief_id: 55e1d474 … promoted: true … intent:  live
  ```
  The transcript asserts a live intent that does not exist. In the intended demo path
  (`--action` used) the claim happens to be true by coincidence, not by verification.
- **Why it matters:** Judge-facing false output in the artifact the demo presents as the
  ledger's receipt, and a reporting regression introduced by this change — before Phase 3.5's
  removal, `IntentState` was set only when an intent had actually been created. It also
  undermines the transcript's evidentiary value precisely where the demo claims rigor.
- **Required correction:** Either query `action_intent` in `queryBelief` and populate
  `Result.IntentState` from the row, or delete the `r.Promoted` fallback so the field prints
  empty when unknown. Also update the now-inaccurate package doc comments naming
  `ProposeIfNew` in the pipeline chain (`pipeline.go:3`, `cmd/solvent/main.go:5`).

## F-8 — `operator-review` never checks `--scenario` against `--belief`, and the mismatch bricks falsification

- **Severity:** IMPORTANT
- **Location:** `cmd/operator-review/main.go:80-97`
- **Evidence:** `kernel.RetireDebt` and `kernel.Promote` are keyed on belief ID only —
  `WHERE id = $1`, no scenario predicate (`internal/kernel/sql.go:22-28`) — while
  `IntentOnPromoted` writes whatever `--scenario` was passed. Reproduced live: a belief in
  scenario `…0001` with `--scenario …0002`:
  ```
  promoted: ok
  intent: live (action="deploy etcd v3.5.28 (typo scenario)")
  audit_live_on_nonpromoted: 0
  VERDICT: PASS                      <-- no warning of any kind

  belief 890f7345 | intent_scenario …0002 | belief_scenario …0001 | live

  $ solvent --falsify 890f7345 --scenario …0001
  retract cascade: ERROR: failed to satisfy CHECK constraint
    ((state != 'live') OR (belief_status = 'promoted')) (SQLSTATE 23514)
  EXIT=1
  ```
  `RetractCascade`'s cancel step filters `scenario_id = $2`, so it misses the mis-scoped intent;
  the retract step's `ON UPDATE CASCADE` then propagates `retracted` into that intent row and
  `live_requires_promoted` correctly detonates.
- **Why it matters:** A single-character typo in a judge-facing demo script produces a
  `VERDICT: PASS` with `audit = 0`, then permanently blocks retraction of that belief — the
  demo's headline capability — recoverable only by manual SQL. Note the schema defended itself
  perfectly: no invariant was lost, the transaction refused. This is a robustness defect in new
  code, not an invariant regression.
- **Required correction:** Before acting, `SELECT scenario_id FROM belief WHERE id = $1` and exit
  non-zero if it differs from `--scenario`. Read-only pre-check; no kernel change.

## F-9 — Implementation report's test counts are wrong

- **Severity:** MINOR
- **Location:** `plans/post/etcd_dataspec2_imp.md:187-195`
- **Evidence:** Report claims 78 total (normalize 9, derive 16, pipeline 9, belief 17, intent 3,
  kernel 24). Measured `go test -count=1 -p 1 -v ./...`: normalize **12**, derive 16, pipeline 9,
  belief **18**, intent 3, kernel **31** — **89** total, 0 failures. This reconciles exactly with
  spec §12's "All 85 existing tests remain green" plus the 4 new OR cases.
- **Why it matters:** The report understates coverage while §5 shows the real gap is *what* is
  covered, not how much. Understated-but-passing is harmless to correctness; it is a traceability
  defect in a document used as evidence.
- **Required correction:** Restate as 89 (85 existing + 4 new).

## F-10 — Spec §11's Track 2 procedure destroys Track 1

- **Severity:** MINOR
- **Location:** `plans/post/etcd_dataspec2.md:616` (Track 2 Step 1 uses `--reset`)
- **Evidence:** `--reset` runs `DROP DATABASE … CASCADE` then `CREATE`
  (`cmd/solvent/main.go:243-247`). Running the documented Track 1 sequence then the documented
  Track 2 sequence wipes Track 1's beliefs, intents, and the operator-review promotion. I ran
  Track 2 **without** `--reset` to preserve Track 1 and confirm scenario coexistence (3 beliefs
  in `…0001`, 2 in `…0002`, no leakage).
- **Why it matters:** The two-scenario separation exists so both tracks can coexist in one
  database; the documented procedure defeats it and would leave a live demo with an empty Track 1.
- **Required correction:** Drop `--reset` from Track 2 Step 1 in the spec/demo procedure (use it
  only on the first command of the session).

## F-11 — `ATTRIBUTION.md` repeats the 404 URL and states GHSA licensing vaguely

- **Severity:** MINOR
- **Location:** `internal/derive/testdata/etcd_real/ATTRIBUTION.md:29`, `:8`
- **Evidence:** Line 29 carries the same non-resolving
  `…/postmortems/v35-data-inconsistency.md`. Line 8 says "Data is provided under the GitHub
  Advisory Database terms"; the `github/advisory-database` repository states **CC-BY-4.0** in its
  `LICENSE.md`, which carries a concrete attribution obligation.
- **Why it matters:** Review criterion §9 requires attribution files be accurate; a licensing
  notice that names no license does not discharge an attribution requirement.
- **Required correction:** Fix the URL; cite CC-BY-4.0 for GHSA data with a link to that
  `LICENSE.md`. Keep Apache-2.0 for the etcd material.

## F-12 — `manifest.json` inside `etcd_real/` makes a wrong `--fixtures` path silently green

- **Severity:** MINOR
- **Location:** `internal/derive/testdata/etcd_real/manifest.json`;
  `internal/pipeline/pipeline.go:296-315`
- **Evidence:** `Run()` collects `*.json`, skips subdirectories, and `continue`s on any filename
  absent from `sourceTypeMap`. Passing `--fixtures internal/derive/testdata/etcd_real` therefore
  processes zero records and the CLI prints `VERDICT: PASS`. Spec §6 correctly places
  manifest/attribution outside the track directories, so the intended paths are fine — but the
  failure mode is silent.
- **Why it matters:** A mistyped fixture path during a live demo yields a green transcript that
  proves nothing. This is the same silent-skip trap flagged as BLOCKER-5 in
  `DATASET_ADVERSARIAL_REVIEW.md`; it is now narrowed but not closed.
- **Required correction:** Have `Run()` return an error (or the CLI report `VERDICT: FAIL`) when a
  fixture directory yields zero processed records.

## F-13 — Dead code left by the Phase 3.5 removal

- **Severity:** NIT
- **Location:** `internal/pipeline/pipeline.go:37` (`Result.IntentState`), `:267-281`
  (`ProposeIfNew`)
- **Evidence:** Neither is reachable from production code after the removal; `ProposeIfNew`'s only
  callers are `pipeline_test.go:226,401,411,421`.
- **Why it matters:** Harmless, but `IntentState` is the field whose emptiness causes F-7.
- **Required correction:** Resolve alongside F-7 — keep `IntentState` and populate it, or remove
  both. Do not delay the demo for this.

## F-14 — `deriveFromPostmortem` drops the spec's version detail

- **Severity:** NIT
- **Location:** `internal/derive/derive.go:224-238`; spec §8
- **Evidence:** Spec §8 specifies
  `"<subject> has documented data inconsistency affecting <affected_versions>, fixed in <fixed_version>"`;
  the implementation emits `"<subject> has documented data inconsistency"`. The report discloses
  this at `etcd_dataspec2_imp.md:50`.
- **Why it matters:** Nothing depends on the claim text except `EnsureBelief` dedup, so the
  behavioral contract holds. Openly disclosed. Once F-2 supplies real version data, the fuller
  string would read better in the demo.
- **Required correction:** Optional — align with spec §8 after F-2, or amend the spec.

## F-15 — `check_i7.sh` cannot validate the new code its acceptance criterion implies

- **Severity:** NIT
- **Location:** `scripts/check_i7.sh:13` (`PKG="${I7_PKG:-internal/kernel}"`)
- **Evidence:** The gate greps `internal/kernel` only. Spec §12 lists it as the check for "no raw
  `db.Exec` writes at CLI level", which it does not cover. I verified `cmd/operator-review`
  manually: kernel calls only, no `db.Exec`, no `tx.Exec`.
- **Required correction:** Optionally add a `cmd/`-scoped grep for `db.Exec|tx.Exec`. The property
  currently holds.

---

# 5. End-to-End Verification

All runs against scratch database `solvent_review` on `:26257`, schema `db/001_schema.sql`,
Track 1 = `…0001`, Track 2 = `…0002`. Every state assertion below was read from the tables with
`cockroach sql`, not taken from CLI output. Scratch DB dropped afterwards.

### Track 1 — PASS (mechanics)

`solvent --scenario …0001 --reset --fixtures …/etcd_real/track1` → `results: 3`, `VERDICT: PASS`.
Database state:

| belief | claim | claim_type | status | debts |
|---|---|---|---|---|
| `06e9d99a` | `Go/go.etcd.io/etcd >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg` | `derived` | `entered` | 5 |
| `55e1d474` | `fix available for etcd-io/etcd v3.5.27` | `accommodated` | `entered` | 4 |
| `890f7345` | `fix available for etcd-io/etcd v3.5.28` | `accommodated` | `entered` | 4 |

Belief A's residual debt was exactly
`needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview` — the spec §4
prediction, item for item. 3 evidence rows, all `external_feed`, correct `source_url`s.
`action_intent` = **0**; `belief_edge` = **0**. All three promotions refused. Audit = 0.

Observed (expected, frozen-kernel behavior): `evidence.snapshot` and
`evidence.source_observed_at` are both `NULL` — `sqlAddEvidence` writes neither, so fixture
`published_at`/`released_at` values never reach the ledger. Raw payloads and dates live only in
the checked-in fixtures, as `DATASET_ADVERSARIAL_REVIEW.md` §IMPORTANT-4 required.

### Track 1 replay — PASS

Re-ran without `--reset`. Counts `beliefs/evidence/intents` = **3/3/1 before and after**, same
belief UUIDs, same claim strings. `EnsureBelief` dedups on `(scenario_id, claim)`;
`evidenceExists` on `(belief_id, content_sha256)`; the pipeline creates no intents. No duplicate
rows, no orphans, no transaction or invariant violation.

### Track 2 — FAIL (not reproducible)

`solvent --scenario …0002 --fixtures …/etcd_real/track2` → `results: 2`, `VERDICT: PASS`:
`fix available for etcd-io/etcd v3.5.0` (4 debts) and `v3.5.x has documented data inconsistency`
(**6 debts** — correct, `postmortem` retires none per spec §9).

Scenario separation is clean:

| scenario | beliefs |
|---|---|
| `…0001` | 3 (advisory + two releases) |
| `…0002` | 2 (v3.5.0 release + postmortem) |

The postmortem belief does **not** leak into Track 1. But spec §5's `postulated` baseline belief
could not be created — no binary or flag can do it (F-4). To verify the retraction mechanics I
substituted the pipeline-created v3.5.0 belief (`44c9bb60`) as a stand-in baseline. **The spec's
Track 2 as written was not executed, because it cannot be.**

### Falsification — PASS

With `44c9bb60` promoted and holding a `live` intent:

- **Wrong scenario** (`--falsify 44c9bb60 --scenario …0001`): `cascade: retracted 0 belief(s)`,
  `VERDICT: NO-OP`; belief still `promoted`, intent still `live`. Scenario scoping holds.
- **Correct scenario** (`--scenario …0002`): `cascade: retracted 1 belief(s)`, audit 0,
  `VERDICT: PASS`. Post-state: belief `retracted`, dependent intent `cancelled`, and
  `action_intent.belief_status` propagated to `retracted` by the composite FK's
  `ON UPDATE CASCADE`. Track 1's three beliefs and its live intent were untouched.

Uses `kernel.RetractCascade` unchanged — cancel strictly before retract, one `ExecuteTx`. No
alternative transaction mechanism, no direct SQL, no `belief_edge` involvement.

### Intent creation — PASS

- Without `--action`: 5 debts retired, `promoted: ok`, `action_intent` count = **0**. Confirms
  promotion does not imply an intent.
- With `--action "deploy etcd v3.5.28"`: exactly **one** row — `state=live`,
  `belief_status=promoted`, `scenario_id=…0001`, `action` verbatim. Created via
  `kernel.IntentOnPromoted`; no SQL bypass.
- Promotion strictly precedes intent creation, and a failed promotion exits before the intent call
  (`main.go:87-89`). OR-3/OR-4 corroborate at the kernel level: unpromoted → `23503`, and
  debt-blocked promotion → `23514` then `23503`.
- Caveat F-8: `--scenario` is not checked against `--belief`, so a mismatch creates a
  cross-scenario intent while still printing `VERDICT: PASS`.

### Audit invariant — PASS

`audit_live_on_nonpromoted = 0` after every single operation: initial pipeline, both
operator-review invocations, replay, Track 2 pipeline, wrong-scenario falsify, and real falsify —
measured per scenario. Even in the deliberately corrupted F-8 state the invariant was never
violated: the schema refused the offending transaction rather than admitting a live intent on a
non-promoted belief.

---

# 6. Regression Verification

| Check | Result |
|---|---|
| `go build ./...` | **PASS** (exit 0) |
| `go vet ./...` | **PASS** (exit 0) |
| `gofmt -l cmd internal` | **PASS** (empty) |
| `go test -count=1 -p 1 ./...` | **PASS** — all 6 test packages `ok`, 0 failures. Per-package `--- PASS` counts: normalize 12, derive 16, pipeline 9, belief 18, intent 3, kernel 31 = **89**. (Report claims 78 — see F-9.) |
| `scripts/check_i7.sh` | **PASS** — 7 `ExecuteTx` write sites, 0 raw writes, 1 permitted pool read; regenerated `docs/M1_I7.md` byte-identically. |

**No regressions found.** The change is additive:

- `db/001_schema.sql`, `internal/kernel/{kernel,sql,contract}.go`, `internal/belief/belief.go`,
  `internal/intent/` — `git diff --stat fca8b86 HEAD` empty for all.
- `normalize` / `derive`: additions only — two constants, two structs+normalizers, two derive
  functions, two switch cases each. No existing branch altered.
- `belief/mapping.go`: one new `DebtMapping` key. Existing rules untouched; six-item `FullDebt`
  unchanged.
- `pipeline.go`: five `sourceTypeMap` additions plus the specified Phase 3.5 removal. The one
  test expectation change (`pipeline_test.go:137-141`, 1 live intent → 0) is the correct
  consequence, not a weakened assertion.
- No `belief_edge` writes anywhere in new code; row count 0 after a full two-track run.
- `cmd/operator-review` uses only kernel functions.

Working tree note: `docs/M2_TRANSCRIPT.md` is modified-but-uncommitted (timestamp and
`elapsed_ms` churn from regenerating the suite; it was already dirty before this review). It does
contain OR-1..OR-4, all `PASS`. `docs/M1_I7.md` regenerated identically. I modified no tracked
file and committed nothing.

---

# 7. Implementation Report Accuracy

`plans/post/etcd_dataspec2_imp.md` is **accurate on mechanics and inaccurate on evidence**.

Correct and confirmed: the file-change inventory; the two new source types and their constants;
the fixture layout; the five `sourceTypeMap` entries; Phase 3.5's removal; the `--action`
behavior; every static check (build/vet/gofmt/I-7); the Track 1 debt progression 5/4/4; "all three
promotions refused"; replay producing no duplicates; schema and `kernel.go` unchanged;
`belief_edge` rows = 0; and — verified independently — that the manifest digests match the
fixtures.

Materially misleading claims:

1. **§7 "Manifest SHA256 digests match files — PASS" and §4 "5 real fixture files"** — the digests
   do match, but the content is not real (F-1, F-2, F-3). The report never states that fixture
   values were invented, and the reader of a spec whose §12 demands "No fabricated URLs in
   fixtures" would reasonably conclude the opposite. §7's "All source URLs present in evidence —
   PASS" is true of *presence* and silent on the fact that one of them 404s.
2. **§7 Track 2 "Enter baseline postulated belief — PASS" and "Attach evidence — PASS"** — no
   committed mechanism exists for either (F-4). Whatever was executed is not in the repository,
   so these rows are unreproducible and read as capabilities the implementation has.
3. **§9 "Total 78 ALL PASS"** — actual is 89 (F-9). Understated, and the per-package figures for
   normalize, belief, and kernel are all wrong.
4. **§7/§9 test rows generally** — listing "internal/normalize 9 PASS" and "internal/derive 16
   PASS" next to the new source types implies the new code is covered. No test references it
   (F-5). Stronger than what the implementation proves.
5. **§3 `github_advisory` "Assertion / Claim"** — accurate as written, but the report does not
   flag that `severity` is dropped and re-derived from prose (F-6), nor that the resulting
   severity is `info`.

The report does deserve credit for openly disclosing the `deriveFromPostmortem` claim-string
simplification (F-14) rather than papering over it.

---

# 8. Final Recommendation

**REWORK REQUIRED — implementation architecture is APPROVED.**

Do not re-open design, the spec, or the research. Nothing below touches the schema, the kernel,
the eight kernel functions, `belief_edge`, the debt vocabulary, or the frozen wave boundaries. The
required work is data correction plus one small additive CLI path.

**Blocking (must fix before the public demo):**

1. **`track1/ghsa_advisory.json`** — set `summary` = `etcd: Authorization bypasses in multiple APIs`,
   `severity` = `high`, `published_at` = `2026-03-20T20:48:14Z`, package ecosystem = `go`,
   name = `go.etcd.io/etcd/v3`.
2. **`track2/postmortem_v35.json`** — `source_url` → `…/postmortems/v3.5-data-inconsistency.md`
   (currently 404); `published_at` → `2022-04-20T00:00:00Z`; `affected_versions` → `v3.5.0–v3.5.2`;
   `fixed_version` → `v3.5.3`; `summary` → the document's own wording ("Code refactor in v3.5.0
   resulted in consistent index not being saved atomically…").
3. **Three release fixtures** — substitute the real tag SHAs and dates:
   v3.5.0 `43b9aa5f9569412d60097cdd07eab7910fd282ab` / `2021-06-15T22:29:48Z`;
   v3.5.27 `ee7ceab4e0ee88901cb46460579d1c22ee5ddb41` / `2026-02-13T18:33:16Z`;
   v3.5.28 `8189e841fbff21101eb58ed97948ecc907690538` / `2026-03-20T17:41:08Z`.
4. **Recompute `manifest.json`** after 1–3, and fix the postmortem URL in `ATTRIBUTION.md`.
5. **Add the Track 2 entry path** — two flags on `cmd/operator-review` (or a sibling command)
   wrapping the existing `kernel.EnterBelief` (with `--claim-type postulated`) and
   `kernel.AddEvidence`, then document the exact Track 2 command sequence.

**Strongly recommended before the demo (cheap, and each is judge-visible):**

6. Fix the `intent: live` false report (F-7) — populate `Result.IntentState` from
   `action_intent`, or drop the `r.Promoted` fallback.
7. Add the `--scenario`/`--belief` consistency pre-check to `operator-review` (F-8) — one
   `SELECT`; prevents a typo from permanently blocking retraction mid-demo.
8. Map the advisory's declared `severity` instead of guessing from prose (F-6).
9. Add tests for the two normalizers, the two derive functions, and one real-fixture pipeline run
   (F-5) — this is what would have caught findings 1–3.

**Defer (must not delay the demo):** F-9 through F-15 — report test counts, the spec's Track 2
`--reset`, the GHSA license citation, zero-record `VERDICT: PASS`, dead `ProposeIfNew` /
`IntentState`, the postmortem claim string, and `check_i7.sh`'s scope.

Once items 1–5 land, re-verification is fast and mostly mechanical: recompute the manifest,
re-run `go test -count=1 -p 1 ./...`, then the two-track sequence from §5 (Track 1 → operator
review with `--action` → replay → Track 2 → `--falsify`), confirming `audit_live_on_nonpromoted = 0`
throughout. On that basis the core is ready to freeze and the effort can move to the
judge-friendly playground, Taskfile, and bchat demo.

Worth saying plainly: correcting the data makes the demo *better*, not merely compliant. The real
postmortem states that *"etcd v3.5 was recommended for production without enough insight on the
production adoption"* — maintainer-authored support for precisely the deployment-decision
invalidation Track 2 dramatizes. And the real v3.5.28 release preceded the advisory's publication
by three hours on the same day, which is a far sharper fix-then-disclose beat than the invented
three-month gap. The strongest version of this demo is the true one.
