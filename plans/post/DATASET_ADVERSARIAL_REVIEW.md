# 1. Executive Verdict

**REWORK REQUIRED**

The dataset *selection* is the strongest part of this research and should be kept almost
intact. The *implementation specification* must not be handed to a coding agent.

The report's specification was written against an imagined Solvent, not the frozen one. Three
findings are independently fatal:

1. **Nothing in the proposed dataset can ever be promoted.** `belief.debt` is a `TEXT[]` of six
   fixed EBP items and `promoted_is_debt_free` is a hard schema CHECK (`db/001_schema.sql:20-23`).
   The evidence classes the report proposes retire at most **two** of the six
   (`internal/belief/mapping.go:15-31`). Four debts remain open forever, `Promote` returns
   `ErrPromotionBlocked`, no intent can be created, and Scenarios A, C and D all die at step 2.
   The report never mentions the six-item debt vocabulary; it invents two domain debts that do
   not exist.
2. **The proposed `belief_edge` row is doubly illegal, and no code path writes edges at all.**
   `parent_belief_id = 'ev_postmortem_v35'` is an evidence ID pointed at
   `belief_edge.parent_id UUID REFERENCES belief(id)` (23503), and `edge_type = 'INVALIDATES'`
   is outside `CHECK (kind IN ('derives','contradicts'))` (23514). The kernel contract
   (`internal/kernel/contract.go:16-25`) has **no edge-writing function**; edges exist only in
   tests and `proof/`.
3. **The fixture as specified is silently ingested as nothing.** `pipeline.Run` derives source
   type from *filename* against a closed map and `continue`s on any unknown name
   (`internal/pipeline/pipeline.go:43-54, 312-315`). `etcd_demo_fixture.json` is not in that
   map, so the CLI would process zero records and print `VERDICT: PASS`. A green transcript
   that proves nothing is worse than a failure.

Add to that: the report's status vocabulary (`DERIVED`, `PENDING`, `LEGAL`) does not exist; its
"PENDING → LEGAL intent gate" **inverts** Solvent's actual gate (an intent on an unpromoted
belief is *physically refused* by the composite FK, it does not sit pending); its five
acceptance-criteria CLI commands do not exist; and `ev_rel_v350` — required by Scenario D — is
absent from the fixture and is not derivable by the existing derivation engine even if added.

None of this requires a fifth table, a schema change, embeddings, or new infrastructure. The
dataset can be made correct with fixture and wording corrections plus one disclosed, additive
extension to frozen Wave 1/2. But the specification must be rewritten before handoff.

# 2. What the Research Got Right

These parts are genuinely good and should survive the rework:

- **Source selection.** GHSA-q8m4-xhhv-38mg / CVE-2026-33413 gives a clean, dated,
  single-project `affected → fixed` pair with exact semver bounds (`>=3.5.0, <3.5.28`). The
  official maintainer-authored `Documentation/postmortems/v3.5-data-inconsistency.md` is an
  unusually strong second story: it is about distributed-state correctness, it is historical
  and stable, and it is authored by the people with standing to say it. Both are primary
  sources. (The prior review round independently corroborated CVE-2026-33413 and the v3.5.0 →
  v3.5.3 postmortem facts against NVD and the 3.5 changelog — `plans/post/research_data_review.md:3`.)
- **Rejecting live APIs.** "Authoritative source → pinned snapshot → offline deterministic
  Solvent" is exactly right, and correctly reasoned (network non-determinism, rate limits).
- **Rejecting vectors/RAG, Kafka, the full NVD corpus, and a fifth table.** All four align with
  the repository's own hard boundaries (`README.md`, I-6 and "no new tables").
- **Labeling the synthetic record as synthetic.** Wrong conclusion (see §3 BLOCKER-7), right
  instinct. Many reports would have smuggled it in unlabeled.
- **Spotting the semver prefix hazard.** A real risk. The report got the direction backwards
  (`internal/normalize/normalize.go:489-494` *adds* a `v`, it does not strip), but it was right
  that version-string joins are where this dataset breaks.
- **Under-15KB, checked-in, deterministically-ordered fixture.** The right shape of answer.
- **The generalization section** (cloud failover, escrow clawback) is a clean demonstration that
  the kernel is domain-agnostic, which is the repo's own claim.

# 3. BLOCKERS

### BLOCKER-1 — The dataset cannot promote any belief: the debt vocabulary is invented

- **Finding.** The report defines "Remediation Fix Availability Debt" retired by release
  evidence, and "Vendor Confirmation Debt" retired by the postmortem. Neither exists. Debt is a
  fixed six-item list.
- **Evidence.** `db/001_schema.sql:12-17` defaults `debt` to
  `{needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`;
  `internal/kernel/kernel.go:28-31` (`FullDebt`) writes byte-identical values.
  `promoted_is_debt_free` (`db/001_schema.sql:20-23`) refuses `status='promoted'` unless
  `array_length(debt,1)=0`. Retirement is table-driven in `internal/belief/mapping.go:15-31`:
  `release` → `{needMap, needInvariant}`; `kev_entry` → `{needMap}`. Only
  `maintainer_comment` rules can retire `needToyCheck`, `needNullModel`, `needObstruction`,
  `needFaithfulnessReview`. The report's dataset contains **zero** maintainer-comment-class
  records.
- **Why it matters.** Best case the proposed dataset retires `{needMap, needInvariant}`. Four
  debts stay open, `Promote` is refused with 23514 → `ErrPromotionBlocked`
  (`internal/kernel/kernel.go:92-97`), `belief.Process` swallows that as `nil`
  (`internal/belief/belief.go:88-96`), no intent is proposed
  (`internal/pipeline/pipeline.go:352-360`), and Scenario A's stages 2–3, all of Scenario C, and
  Scenario D's promoted precondition are unreachable. The existing frozen fixture set is
  calibrated to exactly cover all six (`internal/derive/testdata/`: `kev_entry` +
  `release_with_fix` + `maintainer_fixes` + `maintainer_no_regression` + `maintainer_reviewed`)
  — the report replaced a calibrated set with an under-powered one.
- **Minimum correction.** Restate both proposed debts in the real vocabulary, and pick one of:
  (a) add **real** maintainer-confirmation artifacts for the 3.5.28 fix covering
  tested/confirmed, no-regression, and security-review; or (b) recentre Scenario A on the
  **refusal** — promotion blocked with four open debts, intent refused by the FK with raw
  SQLSTATE 23503 — which is Solvent's actual thesis and needs no extra evidence; or (c) retire
  the residual debts through one explicitly `operator_asserted` review record (a provenance
  class the schema already has, `db/001_schema.sql:41-42`). Recommended: **(b) then (c)**, in
  that order, in the same demo.

### BLOCKER-2 — `belief_edge` mapping is illegal, and no kernel API writes edges

- **Finding.** Spec: `belief_edge` row `edge_retract_v350` with
  `parent_belief_id = 'ev_postmortem_v35'`, `child_belief_id = 'bel_v350_stable'`,
  `edge_type = 'INVALIDATES'`. Narrative also asserts `SUPPORTS` and `CONTRADICTS` edges.
- **Evidence.** `db/001_schema.sql:26-34`: both `parent_id` and `child_id` are
  `UUID NOT NULL REFERENCES belief(id)`; `kind TEXT NOT NULL DEFAULT 'derives'
  CHECK (kind IN ('derives','contradicts'))`. There is no `SUPPORTS` and no `INVALIDATES`.
  Edges are **belief→belief only** — evidence never appears in `belief_edge`; evidence attaches
  to a belief through `evidence.belief_id` (`db/001_schema.sql:38`).
  `internal/kernel/contract.go:16-25` lists all eight kernel functions; none writes an edge.
  Edges are inserted only by raw SQL in tests (`internal/kernel/kernel_test.go:945`),
  `internal/m0/gate.go:84`, and `proof/02_lifecycle_and_invariants.sql:15`.
- **Why it matters.** Three distinct failures. (i) An evidence ID in `parent_id` is a 23503 FK
  violation — this is a real defect, **not** a naming issue: the value is used as a foreign key,
  not as documentation. (ii) `'INVALIDATES'` is a 23514 CHECK violation. (iii) Even with legal
  values there is no supported way to write the row, so a coding agent would either add a kernel
  function (unfreezing the Wave-3 contract) or hand-write SQL outside `crdb.ExecuteTx`,
  violating **I-7**.
  A fourth trap: `descendantsCTE` (`internal/kernel/sql.go:66-75`) does **not** filter on
  `kind`. If the agent writes `kind='contradicts'` edges as the narrative implies,
  `RetractCascade` will traverse them and retract the *contradicting* belief along with its
  target — semantically inverted.
- **Minimum correction.** Delete the `INVALIDATES`/`SUPPORTS` edge specification entirely.
  Invalidation is expressed as: postmortem evidence attached to the target belief via
  `AddEvidence`, followed by `RetractCascade(scenarioID, targetBeliefID)`. The minimal
  Scenario D needs **zero** edges — the cascade root is included in the descendant set
  (`internal/kernel/sql.go:68-70`), so an intent held directly on the root is cancelled. If a
  multi-generation cascade is wanted, say so explicitly and cite that it currently requires
  test-style raw inserts (M2 cases B-07/B-20/B-22).

### BLOCKER-3 — Status and intent-state vocabularies do not exist; the intent gate is inverted

- **Finding.** Spec uses `belief.status ∈ {DERIVED, PROMOTED, RETRACTED}` and
  `action_intent.status ∈ {PENDING, LEGAL, EXECUTED, CANCELLED}`, with the narrative "action
  intent `act_block_v3527` initialized as PENDING while the belief is DERIVED, then transitions
  to LEGAL on promotion".
- **Evidence.** `db/001_schema.sql:9-10`: `status ∈ {'entered','promoted','retracted'}`,
  lowercase, "there is no fourth status" (`README.md`). `db/001_schema.sql:56-57`:
  `state ∈ {'live','cancelled','executed'}` — there is **no** `PENDING` and no `LEGAL`.
  `db/001_schema.sql:59` `live_requires_promoted` plus the composite FK at
  `db/001_schema.sql:66-67` mean an intent row cannot be inserted at all unless its belief is
  currently `promoted`; `kernel.IntentOnPromoted` names that refusal `ErrActionOnUnpromoted`
  (`internal/kernel/kernel.go:104-109`, `internal/kernel/errors.go:16-17`).
- **Why it matters.** The report's central mechanic is a state machine Solvent does not have.
  Solvent does not park an intent in `PENDING` and later bless it — it **refuses the row's
  existence** until the belief has earned promotion. A coding agent implementing
  `PENDING → LEGAL` would either invent a status (schema edit, forbidden) or add
  application-level gating in Go, which `README.md` names as the specific thing agents get wrong
  ("Do not re-implement these as application-level guards in Go").
- **Minimum correction.** Rewrite all status/state literals to the schema's lowercase sets.
  Replace the "gate transition" narrative with the real one: *creating* the intent is refused
  (23503, raw error shown) while debt is open; once debt is empty and the belief is promoted, the
  same call succeeds and inserts `state='live'`.

### BLOCKER-4 — Evidence table mapping is wrong in shape, in typing, and in write order

- **Finding.** Spec maps `evidence.id ← evidence_id` (`'ev_ghsa_q8m4_xhhv_38mg'`),
  `evidence_type ← source_type`, `raw_payload ← stringified raw_payload`,
  `content_hash ← SHA256(...)`, `provenance.pub_date ← published_at`, and describes stage 1 as
  "write raw payloads and hashes to the evidence table", with belief derivation as stage 2.
- **Evidence.** `db/001_schema.sql:36-47`. `id` is `UUID PRIMARY KEY DEFAULT gen_random_uuid()`
  — the text IDs are not insertable. There is no `evidence_type`, no `raw_payload`, no
  `source_id`, no `publication_date`, no `content_hash`, and no `is_synthetic`. The real columns
  are `provenance_class` (CHECK-constrained to four values), `source_url`, `snapshot JSONB`,
  `content_sha256`, `source_observed_at`, `ingested_at`. Critically, `belief_id UUID NOT NULL
  REFERENCES belief(id)` — **evidence cannot exist before its belief**, which inverts the
  report's stage order. And `sqlAddEvidence` (`internal/kernel/sql.go:17-20`) writes only
  `scenario_id, belief_id, provenance_class, source_url, content_sha256`: **`snapshot` and
  `source_observed_at` are never populated by any code path** (verified: no Go file writes
  `snapshot`).
- **Why it matters.** Every column in the report's mapping table is either renamed or absent, so
  the table is not merely imprecise — it is unusable. More importantly, the report's provenance
  tier-1 promise ("raw payloads … pinned in the evidence table … historical timeline anchoring")
  is not deliverable: raw payloads and publication dates never reach the database. Making them
  reach it means changing `AddEvidence`'s signature, which breaks `kernel.Contract`
  (`internal/kernel/contract.go:18`) — i.e. an **architectural change**, unacceptable this phase.
- **Minimum correction.** Rewrite the mapping table against the real columns. State explicitly
  that raw payloads, publication dates and retrieval dates live in the **checked-in fixture
  files and manifest**, not in the ledger; the ledger holds `source_url` + `content_sha256` as
  the provenance handle. Drop all text `evidence_id`/`belief_id`/`act_*` values as database keys
  — they may survive only as fixture-local labels, and the spec must say so.

### BLOCKER-5 — Ingestion path incompatibility: the fixture is silently skipped

- **Finding.** Spec: one array file `etcd_demo_fixture.json` with an envelope
  (`{evidence_id, source_url, source_id, source_type, retrieval_date, publication_date, raw_payload}`)
  and four source types `GITHUB_ADVISORY`, `RELEASE_METADATA`, `POSTMORTEM`, `SYNTHETIC_OVERRIDE`.
- **Evidence.** `normalize.Normalize` (`internal/normalize/normalize.go:17-32`) accepts exactly
  five source types — `github_issue`, `github_pr`, `maintainer_comment`, `release`, `kev_entry`
  — and errors on anything else. `pipeline.Run` reads a *directory*, maps **filename →
  source type** through a closed table, and `continue`s on unmapped names
  (`internal/pipeline/pipeline.go:43-54, 312-315`). Each normalizer unmarshals the file's
  **top-level** object, so an envelope with nested `raw_payload` yields zero-valued fields;
  `normalizeRelease` additionally requires `tag`, `commit_sha`, `released_at`
  (`internal/normalize/normalize.go:291-302`) — the fixture supplies only `version` and
  `is_prerelease`.
- **Why it matters.** With the filename as specified, the CLI processes zero records, produces
  zero results, and prints `VERDICT: PASS` (`cmd/solvent/main.go:112, 144-148`). Acceptance test
  1 ("parses and writes all 5 evidence records") would be reported green having written nothing.
  If the filename *were* mapped, every record would hard-fail normalization instead. Neither
  outcome is a demo.
- **Minimum correction.** One JSON object per file, each filename registered in
  `sourceTypeMap`, each file shaped to its normalizer's real field set (for a release:
  `version, tag, commit_sha, released_at, prerelease, assets`). Keep `manifest.json`,
  `ATTRIBUTION.md` and `raw/` **outside** the ingested directory rather than relying on the
  skip behaviour. Note that registering new filenames edits frozen Wave 4 — see BLOCKER-8.

### BLOCKER-6 — Contradiction semantics, including the "freeze", do not exist

- **Finding.** Scenario C step 3–4: Solvent "writes a CONTRADICTS edge into `belief_edge`" and
  "the presence of an active, unresolved CONTRADICTS edge automatically freezes belief promotion
  rules", preventing the intent from reaching `EXECUTED`.
- **Evidence.** `derive` emits a `Contradicts` field only for `github_issue` /
  `maintainer_comment` matching `reproduces|still vulnerable`
  (`internal/derive/derive.go:98-107, 146-159`). `belief.Process` handles it by emitting
  `slog.Warn` and returning `nil` — "automatic retraction deferred", **no DB write**
  (`internal/belief/belief.go:39-48`). `pipeline` does the same and records
  `Contradiction: true` with `ledger_mutation: none`
  (`internal/pipeline/pipeline.go:86-98, 362-380`; `cmd/solvent/main.go:119-124`). No code
  writes `kind='contradicts'`; no test exercises it; there is no promotion-freeze mechanism
  anywhere.
- **Why it matters.** Scenario C as written is fabricated behaviour attributed to an existing
  system. Its only real observable is a log line — which the frozen
  `internal/derive/testdata/contradicting_evidence.json` already produces.
- **Minimum correction.** Either delete Scenario C, or restate it truthfully as "contradiction
  is detected, logged with provenance, and deliberately produces no ledger mutation; automatic
  retraction is out of scope." Do not claim a freeze.

### BLOCKER-7 — The synthetic override record violates the repository's absolute provenance rule

- **Finding.** `ev_synth_vendor_override`, `source_url: https://synthetic.local/advisories/OVERRIDE-2026-01`,
  `is_synthetic: true`, presented as acceptable because it is labeled.
- **Evidence.** `README.md`, Hard boundaries: *"**No fabricated evidence — ever.** No invented
  IDs, advisories, or URLs. `external_feed` rows must have a resolving `source_url` and a
  matching `content_sha256`; CI enforces this. In a system whose entire pitch is provenance,
  fabricated evidence is self-refuting."* `synthetic.local` does not resolve. There is no
  `is_synthetic` column (`db/001_schema.sql:36-47`), and every normalizer hardcodes
  `ProvenanceExternalFeed` (`internal/normalize/normalize.go:86, 172, 249, 333, 418`), so an
  ingested synthetic record would be **labeled `external_feed`** — the exact mislabeling the
  rule forbids.
- **Why it matters.** This is the one finding where the report's own framing ("synthetic is
  acceptable in principle") is overruled by a standing repository rule, and it is the finding
  most likely to be fatal in front of a judge. Combined with BLOCKER-6, the record also buys
  nothing: it exercises no ledger behaviour.
- **Minimum correction.** **Remove `ev_synth_vendor_override` from the dataset.** If an
  operator-override demo is ever wanted, it must enter through
  `AddEvidence(..., provenance_class='operator_asserted', ...)` with a real internal reference
  and no invented URL — a path the schema already supports and the pipeline does not currently
  use.

### BLOCKER-8 — Scope: the spec silently requires unfreezing Waves 1–4

- **Finding.** The report opens by promising integration "without expanding the frozen core
  architecture" and lists "Schema Redesign (Adding 5th Table)" as rejected — then specifies four
  source types and a fixture format that no frozen package can consume.
- **Evidence.** Honouring the spec requires edits to `internal/normalize` (new normalizer +
  constant), `internal/derive` (new case), `internal/belief/mapping.go` (new debt rules), and
  `internal/pipeline` (new `sourceTypeMap` entries). `README.md`: *"Frozen waves must not be
  modified. Waves 1–4 (normalize, derive, belief/intent/kernel, pipeline/CLI) are frozen. New
  work requires Technical Lead approval"* and *"No new tables, agents, or evidence feeds."*
- **Why it matters.** The work itself is small and additive — no schema change, no fifth table,
  no new infrastructure — so this is **SMALL FIX / EXTENSION**, not architectural. But the
  report presents it as zero-change, which means the coding agent would breach a hard boundary
  without anyone having decided to. Undisclosed scope is the defect here.
- **Minimum correction.** State the unfreeze explicitly and scope it: add source types
  `github_advisory` and `postmortem` to `normalize` + `derive` + `DebtMapping` +
  `sourceTypeMap`, ~4 files, no schema change, requiring Technical Lead sign-off. The
  alternative — reshaping the GHSA record into the existing `kev_entry` shape to avoid any code
  change — is **rejected**: it would label a GitHub advisory as a CISA KEV entry, which is a
  provenance falsehood and no cheaper than the honest extension.

### BLOCKER-9 — `ev_rel_v350` is absent, and `bel_v350_stable` is not derivable even if added

- **Finding.** Scenario D requires `ev_rel_v350` → `bel_v350_stable` → `act_deploy_v350`.
  `plans/post/etcd_demo_fixture.json` contains five records — GHSA, v3.5.27, v3.5.28, postmortem,
  synthetic — and **no v3.5.0 release record**, even though the "Exact Records to Capture" table
  lists `REL-ETCD-v3.5.0` (2021-06-16) and the Executive Recommendation claims "three release
  metadata records".
- **Consequence, not just inconsistency.** Two layers:
  (i) The fixture and the narrative are two different datasets; Acceptance Criteria 1 checks for
  "all 5 evidence records", which the *fixture* satisfies, so the mismatch would pass CI and
  surface only when Scenario D is demonstrated live.
  (ii) **Adding the record does not fix it.** `deriveFromRelease`
  (`internal/derive/derive.go:67-87`) unconditionally emits exactly one claim,
  `"fix available for <subject>"`, classified `accommodated`. It cannot produce "v3.5.0 is
  General Availability and suitable for production deployments". No derivation rule in the repo
  produces a stability or deployment-suitability claim from release metadata.
- **Minimum correction.** Add the real v3.5.0 release record (it is authoritative, tiny, and
  makes the story natural) **and** stop trying to derive the baseline belief from it. Enter the
  baseline as a **`postulated`** belief via `kernel.EnterBelief` — a claim type the schema
  already has (`db/001_schema.sql:8`) and that `derive` never emits — with the v3.5.0 release
  record attached as supporting evidence. This is a demo-layer call into the existing kernel API:
  no schema change, no kernel change. Simplifying Scenario D to avoid v3.5.0 is *not*
  recommended; retraction is the report's strongest narrative and it needs a baseline.

# 4. IMPORTANT FINDINGS

### IMPORTANT-1 — `bel_v350_stable` is stronger than its source; it is decision invalidation, not falsification

- **Finding.** The claim "etcd v3.5.0 is General Availability and suitable for production
  deployments" conjoins a historical fact (v3.5.0 *was* released as GA — permanently true) with
  a judgment (suitability). The postmortem falsifies only the second, and even then carefully:
  the maintainers documented a consistency-index defect under crash conditions while noting they
  were not aware of users reporting production data corruption.
- **Why it matters.** Retracting the conjunction reads as Solvent asserting that a true
  historical fact became false, which invites exactly the objection the demo cannot afford. It
  also misuses Solvent's own model, where retraction means *un-promotion of an actionable claim*,
  preserved for audit (`README.md`: "Debt does not kill").
- **Answering the prompt's question directly: this is a deployment decision becoming invalid, not
  a historical factual belief being proven false.** Framed that way it is fully defensible and is
  a better demonstration than falsification.
- **Minimum correction.** Claim text: **"etcd v3.5.0 is approved for production deployment
  (decision as of 2021-06-16)"**, `claim_type = 'postulated'`. The postmortem is then
  invalidating evidence for a decision, and the GA fact is never a retraction target. Do not use
  the alternative "v3.5.0 has a documented data-consistency defect" as the retracted belief — the
  postmortem *supports* that claim, so retracting it would be backwards.

### IMPORTANT-2 — Scenario D's chronology is inverted

- **Finding.** The postmortem's publication date is 2022-04-20 and its `fixed_version` is 3.5.3;
  v3.5.27 is 2026-02-10 and v3.5.28 is 2026-03-20. The narrative presents the postmortem as
  invalidating evidence "arriving later".
- **Why it matters.** In publication time the "new fact" is four years older than the rest of the
  dataset, and the defect it documents was fixed 24 patch releases before v3.5.27. An evaluator
  reading the fixture's `publication_date` fields sees this immediately.
- **Minimum correction.** Run Scenario D in its **own `scenario_id`**, framed as an as-of-2022
  historical replay (the kernel is scenario-scoped by design, `internal/kernel/sql.go:66-75`,
  D-032), and label it as such in the narrative. Do not interleave it with the 2026 CVE track in
  one timeline.

### IMPORTANT-3 — `bel_v3527_vulnerable` / `bel_v3528_patches_cve`: subject/predicate/object do not exist, and one is seeded illegally

- **Finding.** The spec maps beliefs to `subject`/`predicate`/`object` columns
  (`HAS_VULNERABILITY`, `PROVIDES_FIX_FOR`, `EXHIBITS_FLAW`) and seeds
  `bel_v3528_patches_cve` with `status='PROMOTED'`.
- **Evidence.** `belief` has a single `claim TEXT` column (`db/001_schema.sql:5`) — no triple.
  The real derived strings are `"<subject> is vulnerable to CVE-…"`
  (`internal/derive/derive.go:56`) and `"fix available for <subject>"`
  (`internal/derive/derive.go:83`). Seeding `status='promoted'` while `debt` holds its default
  six items is refused by `promoted_is_debt_free` (23514).
- **Version scope is, however, sound:** the advisory range `>=3.5.0, <3.5.28` genuinely places
  3.5.27 in scope and 3.5.28 out of it, so both beliefs are factually supported by the source.
  The defect is representational, not evidential.
- **Minimum correction.** Replace the triple mapping with the exact claim strings the existing
  derivation produces, and never specify a seeded `promoted` status — promotion is reached only
  by retiring debt.

### IMPORTANT-4 — `content_sha256` is not a hash of the raw payload

- **Finding.** The report states `content_hash = SHA-256 over the pre-normalized raw JSON
  payload` and proposes pinning "global cryptographic hashes" in `manifest.json`.
- **Evidence.** Each normalizer hashes a **per-source-type normalized subset struct**, not the
  raw payload — e.g. release hashes `{version, commit_sha, prerelease}`
  (`internal/normalize/normalize.go:339-345`); KEV hashes
  `{cve_id, product, version_affected, version_fixed}` (`:422-428`). Dedup keys on
  `(belief_id, content_sha256)` (`internal/belief/belief.go:103-112`).
- **Why it matters.** A manifest pinning raw-payload digests as *expected database values* would
  never match, and a coding agent asked to reconcile them might "fix" the normalizers — breaking
  frozen Wave 1 and the replay guarantee.
- **Minimum correction.** `manifest.json` pins digests of the **fixture files** for supply-chain
  integrity, explicitly labeled as file digests. State that `evidence.content_sha256` is derived
  by `normalize` from a per-source-type subset and is not a fixture-authored value.

### IMPORTANT-5 — "Explicit BEGIN … COMMIT blocks" contradicts I-7

- **Finding.** Risks section: "the demonstration CLI runner must execute scenario mutations
  sequentially within explicit BEGIN TRANSACTION … COMMIT blocks."
- **Evidence.** **I-7**: every kernel write goes through `crdb.ExecuteTx`, which runs the 40001
  retry/savepoint protocol (`internal/kernel/kernel.go:56, 71, 81, 93, 106, 126`; `README.md`:
  "A write outside it is a bug"). `scripts/check_i7.sh` exists to police this.
- **Why it matters.** Hand-rolled transaction blocks at the CLI edge would bypass the retry
  protocol the report is trying to protect and would fail the I-7 check.
- **Minimum correction.** Delete the recommendation. Serialization-conflict handling is already
  solved by `crdb.ExecuteTx`; sequential execution is already how `pipeline.Run` works.

### IMPORTANT-6 — The dataset is not the minimum defensible set: simultaneously under- and over-specified

- **Missing:** the v3.5.0 release record (BLOCKER-9); debt-retiring confirmation evidence
  (BLOCKER-1). Without the latter the dataset is not merely small — it is **inert**.
- **Unnecessary:** `ev_synth_vendor_override` (BLOCKER-6 + BLOCKER-7) — it exercises no ledger
  behaviour and breaches a standing rule.
- **Required only by an over-complicated scenario:** the `INVALIDATES` edge, and any
  multi-generation cascade. The minimal Scenario D needs zero edges (BLOCKER-2).
- **Redundant but correctly justified:** the report's own exclusions of GHSA-2xhq-gv6c-p224 and
  GHSA-x35m-3gp4-4fh5 are sound and should stand.
- **Minimum correction.** See §7.

### IMPORTANT-7 — The acceptance-criteria CLI does not exist

- **Finding.** All five acceptance tests invoke `solvent ingest --fixture … --offline`,
  `solvent promote --belief …`, `solvent retract --evidence ev_postmortem_v35`,
  `solvent audit --intent …`.
- **Evidence.** `cmd/solvent/main.go:30-37` defines the entire surface: `--dsn`, `--schema`,
  `--scenario`, `--reset`, `--fixtures`, `--out`, `--falsify`. There are **no subcommands**.
  `--falsify` takes a **belief UUID** and requires `--scenario` (`:48-53`), and calls
  `RetractCascade` directly (`:188`) — retraction is **operator-triggered, never triggered by
  ingesting evidence**. Promotion is not a command; it is attempted automatically inside
  `belief.Process` (`internal/belief/belief.go:88`). There is no audit-by-intent query;
  `AuditLiveOnNonPromoted` returns a scenario-wide count (`internal/kernel/kernel.go:154-158`).
- **Why it matters.** Five of five acceptance tests are unrunnable as written, so "implementation
  ready" cannot be true.
- **Minimum correction.** Rewrite the acceptance table against the real flags. Also note two
  real behaviours the spec must respect: `RetractCascade` retracts only rows already
  `status='promoted'` (`internal/kernel/sql.go:87-91`), so a non-promoted root yields
  `VERDICT: NO-OP` (`cmd/solvent/main.go:207-208`); and `--offline` is unnecessary — the module
  imports no HTTP client anywhere (verified across all Go files), so offline operation is already
  true by construction. Do not add a flag to assert an existing property.

### IMPORTANT-8 — The semver hazard is real but stated backwards

- **Finding.** "Normalization logic must explicitly strip string prefixes and parse all versions
  into standard semver tuples prior to database insertion."
- **Evidence.** `normalizeVersion` (`internal/normalize/normalize.go:489-494`) **adds** a leading
  `v` when absent; it never strips, and nothing parses semver tuples. The advisory range
  `>=3.5.0, <3.5.28` is never evaluated by any code — no semver comparison exists in the
  repository. Version scope reaches the ledger only as free text inside `claim`, via
  `Subject` (`internal/normalize/normalize.go:409`).
- **Why it matters.** Answering the prompt's question directly: **Solvent does not perform semver
  interpretation, and does not "understand v3.5.27 → v3.5.28 remediation".** The v3.5.27/v3.5.28
  relationship is a *narrative* the fixture's claim strings assert, not a computation. Adding
  range evaluation would be a new derivation capability — classify as SMALL FIX / EXTENSION if
  genuinely wanted, but it is **not needed** for the demo and should be cut.
- **Minimum correction.** State that versions normalize to the `v`-prefixed string form, that no
  range evaluation occurs, and pin the exact `versionAffected` / `Subject` strings so claim text
  is deterministic across records.

# 5. MINOR FINDINGS / NITS

- **MINOR — citation hygiene.** Reference [1] is `research_prompt.md`, the report's own input.
  Nearly every claim *about Solvent's behaviour* cites [1], i.e. is unsourced. Claims about the
  repository must cite the repository.
- **MINOR — licensing.** The report does distinguish license fact from attribution requirement,
  but not from conservative recommendation, and states "Fully Permissible" as a conclusion.
  The GHSA CC-BY-4.0 claim is well-founded but is cited to a search-query URL
  (`github.com/advisories?query=…`); cite `github/advisory-database`'s `LICENSE.md` instead.
  Apache-2.0 for etcd docs and release metadata is sound; add that redistribution retains the
  copyright/NOTICE. Nothing here is blocking — proceed, with `ATTRIBUTION.md` as planned and
  "Fully Permissible" softened to "assessed permissible; not legal advice."
- **MINOR — arithmetic.** "two security and reliability records, three release records, and five
  corresponding normalized evidence items" does not reconcile with any reading of a 5-record
  fixture. Fix once the corrected set is settled.
- **MINOR — date pinning.** v3.5.0's date appears as 2021-06-16; the release-record dates for
  v3.5.27/v3.5.28 are midnight placeholders (`00:00:00Z`) while the GHSA carries a real
  timestamp. Pin all dates from the actual release JSON.
- **MINOR — advisory identifier verification.** CVE-2026-33413 and the 3.5.28 fix were
  corroborated in the prior review round against NVD and CHANGELOG-3.5. The GHSA record itself
  should still be captured byte-for-byte from the advisory API into `raw/` so
  `summary`/`severity`/range strings are pinned rather than paraphrased.
- **NIT** — `is_prerelease` in the fixture vs `prerelease` in the normalizer's struct tag
  (`internal/normalize/normalize.go:282`); resolved by reshaping the fixture anyway.
- **NIT** — `README.md` forbids internal vocabulary (`derived`/`accommodated`/`postulated`) on
  user-facing surfaces; the narrative should not print claim types to evaluators.

# 6. Scenario-by-Scenario Verification

| Scenario | Research Claim | Repository Supports? | Real Evidence Valid? | Synthetic Evidence? | Verdict |
|---|---|---|---|---|---|
| **A. Normal Promotion** | Advisory → belief `DERIVED` + intent `PENDING`; v3.5.28 release retires Fix Availability Debt; atomic promotion flips intent `PENDING`→`LEGAL` | **No.** Mechanism exists (`EnsureBelief`→`AddEvidence`→`RetireDebt`→`Promote`→`IntentOnPromoted`) but three specifics are wrong: debt is 6 fixed EBP items and a release retires only 2 (BLOCKER-1); statuses are lowercase `entered/promoted/retracted` (BLOCKER-3); a `PENDING` intent on an unpromoted belief is refused by the composite FK, not parked (BLOCKER-3) | **Yes** — GHSA range `>=3.5.0,<3.5.28` genuinely covers 3.5.27; v3.5.28 genuinely is the fix | No | **REWORK** — dataset cannot promote; recentre on the refusal, then close debt with an `operator_asserted` review record |
| **B. Replay** | Re-ingest → SHA-256 match → suppress duplicate inserts, beliefs, edges; counts unchanged | **Partly.** Idempotence is real: `EnsureBelief` dedups on `(scenario_id, claim)` (`internal/kernel/sql.go:39-53`), `evidenceExists` dedups on `(belief_id, content_sha256)`, `ProposeIfNew` dedups live intents. But the hash is over a normalized subset, not the raw payload (IMPORTANT-4), no edges are ever written so "no duplicate edges" is vacuous, and the specified fixture is skipped entirely so replay would compare zero to zero (BLOCKER-5) | Yes | No | **REWORK** — real capability, but the fixture must actually be ingested for the test to mean anything |
| **C. Contradiction** | Synthetic vendor override → `CONTRADICTS` edge in `belief_edge` → promotion freeze blocks intent reaching `EXECUTED` | **No.** Contradiction produces `slog.Warn` and explicitly **no ledger mutation** (`internal/belief/belief.go:39-48`; `internal/pipeline/pipeline.go:86-98`). No code writes `kind='contradicts'`; no freeze mechanism exists (BLOCKER-6) | n/a | **Yes** — and it breaches `README.md`'s absolute no-fabricated-evidence rule; `synthetic.local` does not resolve and would be mislabeled `external_feed` (BLOCKER-7) | **DROP** — unnecessary; if kept at all, restate truthfully as detect-and-log with no mutation |
| **D. Falsification / Retraction** | `ev_rel_v350` → `bel_v350_stable` promoted → `act_deploy_v350` LEGAL → postmortem → `INVALIDATES` edge → RETRACTED → cascade → CANCELLED | **Partly — the kernel half is genuinely strong.** `RetractCascade` is one `ExecuteTx`, cancel-strictly-before-retract (I-8), scenario-scoped, cycle-safe, with the composite FK's `ON UPDATE CASCADE` enforcing ordering structurally (`internal/kernel/kernel.go:111-147`); tested by B-07/B-12/B-20/B-22/B-24/B-08. But: no `INVALIDATES` edge type and evidence IDs cannot be edge parents (BLOCKER-2); retraction is triggered by `--falsify <beliefUUID>`, never by ingesting evidence (IMPORTANT-7); root must already be `promoted`; `ev_rel_v350` is absent and `bel_v350_stable` is not derivable (BLOCKER-9) | **Yes, with a scope correction** — the postmortem supports invalidating a *deployment decision*, not falsifying the GA fact (IMPORTANT-1); and its 2022 date inverts the "arrives later" framing (IMPORTANT-2) | No | **REWORK — achievable, and worth it.** With a `postulated` baseline belief, no edges, its own `scenario_id`, and `--falsify`, this runs on 100% real evidence with **no kernel change** |

# 7. Dataset Correction

Final minimum record set. Two tracks, two `scenario_id`s. No fifth table, no schema change.

**Track 1 — CVE-2026-33413 lifecycle (Scenarios A + B)**

| # | Record | Real/Synthetic | Source | Purpose | Required? |
|---|---|---|---|---|---|
| 1 | GHSA-q8m4-xhhv-38mg / CVE-2026-33413 advisory | **Real** | `github.com/advisories/GHSA-q8m4-xhhv-38mg` (raw advisory JSON pinned in `raw/`) | Derives the vulnerability claim for v3.5.27; retires `needMap` | **Required** |
| 2 | etcd v3.5.28 release metadata | **Real** | `github.com/etcd-io/etcd/releases/tag/v3.5.28` | Fix-availability evidence; retires `needMap`, `needInvariant` | **Required** |
| 3 | etcd v3.5.27 release metadata | **Real** | `github.com/etcd-io/etcd/releases/tag/v3.5.27` | Pins the affected deployment target as a dated fact | Optional (narrative clarity) |
| 4 | Operator security-review record | **Operator-asserted, clearly labeled** | Internal reference; `provenance_class='operator_asserted'`; **no invented URL** | Retires `needToyCheck`, `needNullModel`, `needObstruction`, `needFaithfulnessReview` so promotion can occur at all | **Required** unless real maintainer-confirmation artifacts for the 3.5.28 fix are found, in which case use those instead |
| — | ~~`ev_synth_vendor_override`~~ | ~~Synthetic~~ | ~~`synthetic.local`~~ | **Removed** — exercises no ledger behaviour; breaches the no-fabricated-evidence rule | **Removed** |

Track 1 demonstrates, in order: ingestion with provenance → **promotion refused, four debts
open** → **intent refused by the composite FK, raw SQLSTATE 23503 shown** → operator review
closes the remaining debt → promotion succeeds → intent inserts `live` → replay is a no-op. The
refusal is the demo, and it costs nothing to show.

**Track 2 — v3.5.0 deployment-decision retraction (Scenario D), separate `scenario_id`, as-of 2022-04**

| # | Record | Real/Synthetic | Source | Purpose | Required? |
|---|---|---|---|---|---|
| 5 | etcd v3.5.0 release metadata | **Real** | `github.com/etcd-io/etcd/releases/tag/v3.5.0` (2021-06) | Supporting evidence for the baseline deployment decision | **Required** |
| 6 | v3.5-data-inconsistency postmortem | **Real** | `etcd-io/etcd/Documentation/postmortems/v3.5-data-inconsistency.md` (2022-04-20) | Invalidating evidence: affects 3.5.0–3.5.2, fixed in 3.5.3 | **Required** |

Plus one **belief**, not a record: `claim_type='postulated'`, claim *"etcd v3.5.0 is approved for
production deployment (decision as of 2021-06-16)"*, entered via `kernel.EnterBelief`, promoted,
carrying a `live` intent; then `--falsify <thatBeliefUUID> --scenario <track2UUID>` cancels the
intent and retracts the belief in one transaction, with the ledger preserving all of it.

**Total: 5 required + 1 optional real records, one operator-asserted record, zero synthetic
records, zero edges, zero schema changes.** Six records where the report had five — and unlike
the report's five, these actually run.

# 8. Semantic Corrections

Exact changes; everything else in the report stands.

1. **Statuses.** `DERIVED` → `entered`; `PROMOTED` → `promoted`; `RETRACTED` → `retracted`.
   Delete `PENDING` and `LEGAL` entirely; intent states are `live` / `cancelled` / `executed`.
2. **The gate narrative.** Replace "intent initialized PENDING, transitions to LEGAL" with:
   *"creating the intent is refused by the composite foreign key (SQLSTATE 23503) while the
   belief is unpromoted; once debt is empty and the belief is promoted, the same call inserts
   `state='live'`."*
3. **Debt.** Delete "Fix Availability Debt" and "Vendor Confirmation Debt" as entities. Restate
   as: *"release evidence retires `needMap` and `needInvariant`; the remaining
   `needToyCheck`, `needNullModel`, `needObstruction`, `needFaithfulnessReview` require
   confirmation-class evidence; promotion requires all six retired
   (`promoted_is_debt_free`)."*
4. **`bel_v350_stable`.** Rename to `bel_v350_deploy_approved`. Claim text: **"etcd v3.5.0 is
   approved for production deployment (decision as of 2021-06-16)"**, `claim_type='postulated'`.
   Narrative: *"a deployment decision becomes invalid"*, never *"a historical fact is proven
   false."* Drop "General Availability" from the retraction target.
5. **`bel_v3527_vulnerable` / `bel_v3528_patches_cve`.** Delete the
   subject/predicate/object/claim_type mapping. Use the exact strings the frozen derivation
   emits: `"<product> <affected-range> is vulnerable to CVE-2026-33413"` and
   `"fix available for etcd-io/etcd v3.5.28"`. Remove the seeded `status='PROMOTED'`.
6. **Edges.** Delete the whole `belief_edge` mapping including `edge_retract_v350`. Replace
   with: *"invalidation is recorded as postmortem evidence attached to the target belief,
   followed by `RetractCascade` on that belief; `belief_edge` connects belief→belief only, with
   `kind ∈ {derives, contradicts}`, and no kernel function writes it."*
7. **Contradiction.** Delete "automatically freezes belief promotion rules" and the `CONTRADICTS`
   edge write. Replace with: *"contradiction is detected, logged with source provenance, and
   deliberately produces no ledger mutation."*
8. **Evidence mapping.** Rewrite against the real columns: `provenance_class`, `source_url`,
   `content_sha256`, `belief_id`, `scenario_id` (+ `snapshot`, `source_observed_at`, which no
   code path populates). Delete `evidence_type`, `raw_payload`, `source_id`,
   `publication_date`, `content_hash`, `is_synthetic`. State that IDs are database-generated
   UUIDs and that `ev_*` / `bel_*` / `act_*` are fixture-local labels only. State that evidence
   requires an existing belief (`belief_id NOT NULL`), so the write order is
   normalize → derive → `EnsureBelief` → `AddEvidence`, not evidence-first.
9. **Hashes.** `manifest.json` pins **fixture-file** digests, labeled as such;
   `evidence.content_sha256` is derived by `normalize` from a per-source-type subset and is not
   fixture-authored.
10. **Transactions.** Delete the explicit `BEGIN … COMMIT` recommendation (violates I-7);
    `crdb.ExecuteTx` already owns retry.
11. **Semver.** Replace "strip prefixes and parse semver tuples" with *"versions normalize to
    the `v`-prefixed string form; Solvent performs no range evaluation, so the
    v3.5.27 → v3.5.28 relationship is asserted by pinned claim text, not computed."*
12. **Offline.** Delete `--offline`; the module imports no HTTP client, so offline execution is
    already structural.
13. **Acceptance criteria.** Rewrite all five against the real CLI: `--scenario`, `--reset`,
    `--fixtures`, `--out`, `--falsify <beliefUUID>`. Note that `--falsify` on a non-promoted
    root yields `VERDICT: NO-OP`.

# 9. Implementation Readiness

**Can the research report now be handed to the coding agent? NO.**

Its dataset *selection* is ready. Its *implementation specification* would produce a
non-compiling, non-inserting, or silently-empty integration, and in two places would push the
agent across the repository's hard boundaries without anyone having decided to.

Required before handoff:

1. Rewrite the four-table mapping against `db/001_schema.sql` — statuses, intent states,
   column names, UUID keys, `provenance_class` (BLOCKERS 3, 4).
2. Delete the `belief_edge` mapping; express invalidation as evidence + `RetractCascade`
   (BLOCKER-2).
3. Restate debt in the six-item EBP vocabulary and add the evidence needed to actually reach
   zero debt — or recentre Scenario A on the refusal (BLOCKER-1).
4. Reshape the fixture: one object per file, real normalizer field sets, filenames registered in
   `sourceTypeMap`; manifest/attribution/raw outside the ingest directory (BLOCKER-5).
5. Add the real v3.5.0 release record and make the Track-2 baseline a `postulated` belief entered
   via `EnterBelief` (BLOCKER-9).
6. Remove `ev_synth_vendor_override`; drop or truthfully restate Scenario C (BLOCKERS 6, 7).
7. Disclose the Wave 1/2/4 extension (`github_advisory`, `postmortem` source types) as an
   explicit, Technical-Lead-approved, additive change — no schema edit (BLOCKER-8).
8. Rewrite the acceptance criteria against the real CLI flags (IMPORTANT-7).
9. Apply the wording corrections in §8, especially items 4 and 7.

Once those nine are folded in, the dataset is implementation-ready: real data, correct semantics,
frozen kernel, deterministic fixture. No item requires a fifth table, schema redesign,
embeddings, vector search, RAG, live APIs, Kafka, streaming, multi-agent infrastructure, or a new
domain implementation.

# 10. Coding-Agent Handoff Delta

Fold these into the coding-agent prompt verbatim:

1. **The schema is authoritative and frozen.** Read `db/001_schema.sql` before writing anything.
   Where the research report and the schema disagree, the schema wins. Do not edit that file.
2. **Statuses are lowercase and closed:** belief `entered|promoted|retracted`; intent
   `live|cancelled|executed`. There is no `DERIVED`, `PENDING`, or `LEGAL`.
3. **Do not write `belief_edge`.** Not for `SUPPORTS`, not for `INVALIDATES`, not at all. It is
   belief→belief with `kind ∈ {derives, contradicts}`, and no kernel function writes it.
   Invalidation = attach evidence, then `RetractCascade` on the target belief.
4. **Debt is the six `FullDebt` items.** Promotion requires all six retired. Retirement is
   table-driven in `internal/belief/mapping.go` — do not invent debt names.
5. **Intents cannot precede promotion.** `IntentOnPromoted` on an unpromoted belief returns
   `ErrActionOnUnpromoted` (23503). Show that refusal as a demo step; do not engineer around it.
6. **Contradictions produce no ledger mutation.** Detect, `slog.Warn`, return. Do not add a
   promotion freeze.
7. **Fixtures: one JSON object per file**, top-level fields matching the target normalizer, each
   filename registered in `pipeline.sourceTypeMap`. Verify a non-zero result count in the
   transcript — an unregistered filename is skipped silently and still prints `VERDICT: PASS`.
8. **No fabricated evidence.** No `synthetic.local`, no invented IDs or URLs. Anything not from
   a resolving primary source must be `provenance_class='operator_asserted'` and labeled.
9. **Track 2 baseline is `postulated`**, entered via `kernel.EnterBelief`, in its **own
   `scenario_id`**, framed as an as-of-2022 replay. Its claim is a *deployment-approval
   decision*, never "v3.5.0 was not GA".
10. **All writes through `crdb.ExecuteTx`** (I-7). No hand-rolled `BEGIN … COMMIT`, no raw
    `db.Exec` writes. Run `scripts/check_i7.sh`.
11. **Adding `github_advisory` / `postmortem` source types touches frozen Waves 1, 2 and 4.**
    Additive only: new normalizer + constant, new `derive` case, new `DebtMapping` entries, new
    `sourceTypeMap` entries. No schema change, no new table. Requires Technical Lead sign-off
    before you start. If it is not granted, stop and report rather than reshaping a GHSA record
    into `kev_entry`.
12. **Verify with the real CLI:** `go build ./... && go vet ./...`, then
    `solvent --scenario <uuid> --reset --fixtures <dir> --out <path>` for Tracks 1–2, then
    `solvent --falsify <beliefUUID> --scenario <track2uuid>`. Every run must end
    `audit_live_on_nonpromoted: 0`. A `NO-OP` verdict from `--falsify` means the root was not
    promoted — fix the debt path, not the cascade.
