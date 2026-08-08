# PLAN3_CODE_REVIEW.md — Wave 1 (`internal/normalize`)

**Status:** REWORK REQUIRED (targeted, small change set)
**Scope:** Wave 1 only — `internal/normalize`, its fixtures, and its tests.
Reviewed per `plan3_imp.md` §2, incorporating the technical-lead amendments in
`plan3_imp_review.md` (renamed Pass 3, added Pass 4, Scope Violations, traceability,
Technical Certification, standard verdict vocabulary).
**Reviewer posture:** adversarial. Every function not explicitly challenged is certified.

---

## 0. Pre-Flight Receipts (independently re-run)

| Receipt | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./internal/normalize/... -v` | 8/8 PASS |
| k/R-core untouched | Wave 1 has no kernel/database interaction |

Key findings were validated empirically (temporary in-package probes, removed after
use) rather than asserted from reading:

```
same input, two calls → content_sha256 equal:   true
same input, two calls → IngestedAt equal:        false  (time.Now() each call)
unmerged PR → ObservedAt equal across calls:     false  (time.Now() fabricated)
KEV key-order reorder → same hash:               true   (canonicalization works)
body "…still has slow follow-ups…" → severity:   "low"  (substring false-positive)
missing issue_number → accepted → URL /issues/0
two issues, different numbers, same text → IDENTICAL content_sha256
```

---

## Pass 1 — Distributed Systems Engineer

### Attack surface examined

- Error handling and wrapping in all five normalizers.
- Purity / idempotency: is `Normalize` a pure function of `(raw, sourceType)`?
- Silent failure paths (`computeSHA256`, `json.Marshal`).
- Nil/zero handling: `issue_number == 0`, empty `state`, empty `labels`, empty body.
- Panic safety: slicing, map access, regex compilation.
- Concurrency: mutable package globals, concurrent calls.

### Findings

**B1 (P0) — `IngestedAt` is `time.Now()` inside a function documented as pure and
deterministic.** `types.go:4-6` states *"identical input always produces identical
output and identical content_sha256"*. `normalize.go:72` sets
`IngestedAt: time.Now().UTC()` on every call. Same fixture, two calls → different
`IngestedAt`. The dedup hash is stable (verified), so today's ledger impact is
limited — but the documented determinism contract is false, and it defeats the whole
point of a content-addressed pipeline. **Fix:** the caller must own `IngestedAt` (or
it is derived from the feed's `fetched_at`); `Normalize` must not read the clock.
If passed in, the function becomes pure again.

**B2 (P1) — `ObservedAt` is fabricated on the fallback paths.**
`normalizeGitHubPR` (when `merged=false` or `merged_at` empty) and `normalizeKEVEntry`
(when `dateAdded` empty) set `ObservedAt = time.Now()`. Spec §4.2 defines `observed_at`
as *"when this was observed upstream"* — the ingest clock is not upstream time. An
unmerged PR rows claims to have been observed "now"; KEV entries without `dateAdded`
claim fabricated observation times. Contract and prompt philosophy: *"Unknown
behavior should fail loudly."* Fabricated timestamps are the opposite. These paths
should return an error (or leave `ObservedAt` zero and let the ledger decide) — never
substitute wall-clock for upstream truth.

**B3 (P2) — silent failure in `computeSHA256`.**
`computeSHA256` discards the `json.Marshal` error and returns `""`. With today's
inputs the marshal cannot fail, so it is unreachable — but a future injectable field
in a `hashable` would quietly emit an empty `ContentSHA256` into a `NOT NULL` schema
column (`evidence.content_sha256`). Returning a plain `string` makes the failure
unrepresentable; the hash should be computed eagerly inside each normalizer where
errors can propagate.

**B4 (P2) — dead code in `stripMarkdown`.**
`normalize.go:409-410` replaces `` ` `` (single backtick) before replacing ``` ```
(triple). The triple-backtick pass is unreachable — every backtick is already gone —
so code-block handling is accidental, not designed (probe: a fenced block collapses to
its inner text with markers removed, which the "known limitations" section claims is
handled). Either make code-block stripping explicit or remove the dead line; the
"limitation" note should describe what actually happens.

### Pass 1 Verdict

**Status:** CONDITIONALLY APPROVED (B1, B2 must land before freeze)

**Blockers:** (B1, B2)
**Nits:** (B3, B4)
**Certified Correct:** dispatcher switch is exhaustive over the five source types with
. consistent default error; `parseTimestamp` is fail-fast with a wrapped error; the
CVE regex compiles once at package level; no index-out-of-range possible in any strip
loop; no mutable package state is written during a call.

---

## Pass 2 — Data Engineer

### Attack surface examined

- Determinism and canonicalization: field order, whitespace, optional-field order.
- Missing-field behavior against spec §3 required flags.
- Hash stability across observation identity.
- Fixture coverage; version handling; semver edge cases.

### Findings

**Req1 (P1) — required-field validation is inconsistent with spec §3 and silently
accepts zero values that corrupt output URLs.**
Spec §3.1 marks `event_type`, `issue_number`, `repository`, `title`, `labels`,
`state`, `created_at` required for issues; §3.2 marks `pr_number`, `state`, `merged`,
`base_branch` required for PRs. The implementation validates a hand-picked subset:
- `githubIssue`: validates `Repository`, `Title`, `CreatedAt`; ignores missing
  `event_type`, zero `issue_number`, zero-length `labels`, empty `state`.
- `githubPR`: validates `Repository`, `Title`; ignores zero `pr_number`, empty `state`.
- `maintainerComment`: validates `Repository`, `Author`, `Body`, `CreatedAt`; ignores
  zero `comment_id` / `issue_or_pr_number`.
- `release`: required fields validated.
- `kevEntry`: validates CVE format, vendor/product/description/reference; **does not
  enforce `versionAffected`**, which then surfaces as an empty `Subject`.

Proven empirically: missing `issue_number` is accepted and produces a fabricated
`SourceURL = https://…/issues/0`; empty `state` is accepted. **Fix:** guard every field
the spec marks required, especially the ones that seed the URL or the hash.

**Req2 (P1) — observation identity is excluded from the content hash, so distinct
observations are conflated.**
The `github_issue` hashable is `{Source, Title, Body, State, Labels}`; it excludes the
issue number and `event_type`. Two distinct issues (different `issue_number`, different
URLs, identical text) produce the **same contenthash** (proven). Spec §4.3's ledger
dedup rule is *"two evidence rows with the same content_sha256 for one belief_id are
duplicates."* Real feeds routinely file duplicate reports (same title/body re-filed),
and a malicious feed can forge copies at will. If Wave 2/3 dedups on
`content_sha256` alone, a genuine distinct observation is silently collapsed. The
identity of an observation is `event_type + issue/pr number + repository + created_at`
per spec §3. **Fix:** include the identity fields in the hash input; the hash stays
deterministic and 64 hex chars.

**Req3 (P1) — `DomainPayload` is not the "unmodified source payload" the spec §4.3
demands.** Spec maps `domain_payload` → `evidence.snapshot` = "JSON, unmodified source
payload". The code builds a filtered map per source that drops fields: `state`,
`labels`, `created_at` (issue); `merged_at`, `state` (PR); comments drop `created_at`;
release omits several fields; KEV drops `source_url`. Spec §4.1 audit contract says raw
payload is "never parsed or modified after ingestion". A filtered reconstruction is a
projection, not a snapshot, so the audit trail loses fields the feed really sent.
**Fix:** store the raw input bytes in `DomainPayload`; keep parsed fields derived
directly from it downstream.

**Req4 (P2) — no concrete SHA is pinned.** Every test asserts `ContentSHA256 != ""`
but never a specific value. A silent change to the hash input would change every dedup
key without any test failing. The technical-lead review (`normalize_imp.md` N1/N2) made
`TestCanonicalizationDeterminism` and a negative fixture **required** at the Wave-1
gate; neither exists. **Fix:** add the canonicalization test over re-ordered JSON, a
negative fixture, and pin at least one golden hash.

### Certifications

**Certified OK:** JSON field-order canonicalization holds (struct re-marshaling is
field-order deterministic; proven by the key-order probe); labels are sorted before
hashing — sound canonicalization instinct; timestamps are normalized to UTC at parse;
`normalizeVersion` only prefixes `v` and never corrupts; keyword extraction is
substring-deterministic; unknown-source dispatch errors loudly.

### Pass 2 Verdict

**Status:** REWORK REQUIRED (Req1, Req2, Req3 must land)

**Blockers:** (Req1, Req2, Req3)
**Nits:** (severity classifier — see Pass 3)
**Certified Correct:** canonicalization; RFC3339 parse; label sort; UTC normalization.

---

## Pass 3 — Adversarial Systems Engineer

(Focus: malformed input, replay, poisoning, trust boundaries, DoS insofar as it exits
Wave 1.)

### Findings

**S1 (P1) — substring-based severity classifier misclassifies ordinary text.**
`classifySeverity` uses `strings.Contains` on the lower-cased title+body. The English
density of small substrings ("low" inside "follow", "slow", "allow", "below"; "high"
inside "highly") means ordinary prose gets forced into a bucket that then colors the
graph. Proven: the probe body, *"fixed but still has slow follow-ups"*, produced
`SeverityLow`. This manufactures a false incident signal from well-meaning text.
**Fix:** word-boundary matching (`\b`) against a severity corpus, or classify
elsewhere.

**S2 (P2) — `KnownMaintainers` is an exported, mutable map.**
Consumers can mutate it, and concurrent writers would race. It backs the
`is_maintainer` marker that Wave 2 uses as a trust decision, so it must be immutable.
**Fix:** an unexported read-only helper or an internal package constant that never
exposes the map value.

**S3 (P2) — upstream `SourceURL` for KEV entries is unvalidated.**
The KEV `reference` field is passed straight into `evidence.source_url`. No
scheme/format check; a hostile feed could emit `file:` or `javascript:` URLs that later
reach the graph/demo UI. **Fix:** validate `https?://` at normalize time (the URL is
derived today, but this one is trust).

**S4 (P2) — replay/temporal semantics are unstated.** Replayed old evidence of a
duplicate identity (or against a fresh identity after Req1 is fixed) can be
re-accepted; the pipeline has no order/anti-replay thinking yet. This is acceptable
today because nothing downstream is stateful, but it must be a shaped decision once
Wave 2 writes rows. Recorded as an explicit follow-on.

**Documentation / fixtures:**
- No negative fixture file exists; `TestNormalizeMissingField` checks only one source
  and does not cover every required field.
- `NORMALIZE_IMPLEMENTATION.md` cites `wave1_plan.md` (line 3), which does not exist in
  the repo — receipts must cite only real artifacts (traceability line).

### Pass 3 Verdict

**Status:** APPROVED WITH NITS — no bypass found in the Wave-1 surface; security
properties are correctly deferred to Wave 2/3. The severity classifier and identity
questions intersect with data correctness and are carried into the synthesis.

---

## Pass 4 — CockroachDB Engineer

**Status:** N/A / VERIFIED CLEAN — `internal/normalize` contains zero database surface:
no SQL, no driver, no transactions, no coupling to `db/001_schema.sql`. The purity
bugs in Pass 1 are pure-Go concerns, not DB concerns. No PostgreSQL/SQLSTATE / `pgx`
assumption exists here. Content hashes are 64-char lowercase hex STRINGs —
compatible with the schema's `evidence.content_sha256`, and no `embedding` column is
set in this package (correct for I-6).

**Forward flag for Wave 2/3:** whether observation identity is part of the content
hash (Req4 line) determines whether `evidence` dedup is scoped soundly. No risk today.

**Traceability note:** all of Req1–Req3, B1–B2, and S1 map to
`DATA_PIPELINE_SPEC.md` §3.1–3.5, §4.1–4.3 and to milestones M4 (ingestor) inputs.

---

## Scope Check

- Reviewed only `internal/normalize/**` and its own tests and fixtures.
- Did NOT review `internal/kernel` (out of Wave-1 scope), schema DDL, or plan docs.
- Did not propose architecture changes, new tables, or new sources.
- Note: only Wave 1 currently exists; the wider review in `plan3_imp.md` awaits the
  future `derive`/`belief`/`intent` rolls and is deferred, not author-skipped.

### Scope Violations Found

None. Nothing in `internal/normalize/` strays into demo, UI, graph, feed, or ledger
packages. The package is correctly placed at the "domain-agnostic but
domain-shaped bridge" boundary the spec assigns to it.

---

## Cross-Pass Synthesis

### Aggregate Verdict

**Status: REWORK REQUIRED** (most restrictive pass is Pass 2, plus Pass 1 blockers).

- Pass 1 — CONDITIONALLY APPROVED (B1, B2)
- Pass 2 — REWORK REQUIRED (Req1, Req2, Req3)
- Pass 3 — APPROVED WITH NITS
- Pass 4 — CLEAN

### Specification Traceability

| ID | Spec ref (authority) | Implementation file | Function | Implied invariant violated / note |
|---|---|---|---|---|
| B1 | SPEC §4.1-4.2 (deterministic ingestion) | `internal/normalize/normalize.go` | `normalizeGitHubIssue` / all normalizers (`IngestedAt`) | Processor purity: same input needs identical output |
| B2 | SPEC §4.2 "observed_at = observed upstream" | `normalize.go` | `normalizeGitHubPR`, `normalizeKEVEntry` (ObservedAt fallback) | Auditability: never fabricate timestamps |
| Req1 | SPEC §3.1–3.5 (required flags) | `normalize.go` | all normalizer entry points | Determinism/correctness: no `/issues/0` artifacts |
| Req2 | SPEC §4.3 (dedup on content identity) | `normalize.go` | content hash builders | Duplicate vs distinct observation precision |
| Req3 | SPEC §4.3 `evidence.snapshot` | `normalize.go` | `DomainPayload` per source | Audit: unmodified raw payload required |
| Req4 | `normalize_imp.md` N1/N2 (gate) | `normalize_test.go` | `TestHashDeterminism` | Canonicalization + negative-fixture coverage |
| S1 | SPEC §2 `severity` display | `normalize.go:classifySeverity` | classified severities | Derived display accuracy |

### Must-Fix Before Freeze (P0/P1)

1. Remove `time.Now()` from `Normalize`, owner `IngestedAt` at the caller (B1).
2. Do not fabricate `ObservedAt`; error or zero on missing upstream time (B2).
3. Enforce all spec-required fields per source (Req1).
4. Include observation identity in the content hash (Req2).
5. Store the unmodified raw payload in `DomainPayload` (Req3).
6. Add `TestCanonicalizationDeterminism`, a negative fixture, and a pinned golden hash
   (Req4, required by the technical lead review).

### Should-Fix (non-blocking)

1. Word-boundary severity matching (S1).
2. Make `KnownMaintainers` immutable (S2).
3. Validate KEV `reference` to `https?://` (S3).
4. Write the anti-replay/state expectations before Wave 2 begins (S4).

### Decision Log

- **DR-1:** Keep the public API (one `Normalize`); the fixes are internal to the five
  normalizers or add one purity parameter — not new interfaces.
- **DR-2:** Keep severity as derived metadata, not a stored ledger column; the
  classifier is fixed, the model is not.
- **DR-3:** Land Req1/Req2/Req3 in Wave 1 — they change the shape of what will be
  written by Wave 2/3; postponing makes retro-fitting expensive.
- **DR-4:** No replay/temporal gate in Wave 1; document it as an explicit Wave-2
  forward contract (`plan3_imp_review` S4 recording).
- **DR-5:** Aggregate blockers belt the rework list; do not start the pipeline rolls
  until the technical lead accepts this review and the change set lands.

---

## Authorization

**This review does not yet authorize Wave 1 freeze.** It authorizes the change set in
"Must-Fix" above. After the change set lands, re-run the receipts:

```
go build ./...
go vet ./...
go test ./internal/normalize/... -v
```

Then the freeze can proceed and Wave 2 (`normalize_imp` review → `derive` etc.) may
only begin after the technical lead accepts this review (workflow:
Adversarial Review → Technical Lead acceptance → Wave Freeze as specified in
`prompt_imp.md`).

---

## Technical Certification

I certify that I challenged, on the existing code and with live probes where the
behavior mattered:

- ✅ determinism / purity — `IngestedAt`, `ObservedAt`, canonicalization across key
  order
- ✅ idempotency and identity — duplicate collision under `content_sha256`
- ✅ correctness of required fields and URLs
- ✅ trust boundaries — upstream text → generated URL/labels, `is_maintainer` map
- ✅ panic safety, silent-failure audit (`computeSHA256`, `json.Marshal`)
- ✅ concurrency surface — package globals, mutable exported map
- ✅ scope discipline — no scope violations; frozen kernel untouched

**Remaining unchecked:**
- Wave 2/3 logic packages (not present in the repo yet)
- live cluster behavior of `content_sha256` in a synchronous ledger
- any demo/UI/graph behavior

**Adversarial reviewer:** Solvent technical reviewer
**Date:** 2026-08-08