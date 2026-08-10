# etcd Real-Data Implementation Specification

**Decision Date:** 2026-08-10
**Status:** DRAFT — pending operator-review mechanism decision (Section 15)
**Frozen Architecture:** Waves 1–4 — no schema changes, no new kernel functions, no new tables

---

## 1. Executive Decision

Two-track real-world etcd story, all records genuine:

- **Track 1 — CVE lifecycle**: GHSA-q8m4-xhhv-38mg advisory → vulnerability belief → release fix → debt retirement → promotion refusal → operator review → promotion → intent
- **Track 2 — historical retraction**: etcd v3.5.0 deployment decision (postulated 2021-06-16) → live intent → 2022 postmortem → `--falsify` → RetractCascade → cancelled intent

### What is being built

- Two new source types (`github_advisory`, `postmortem`) added to the normalize→derive→mapping chain
- Five real fixture files + one manifest + one attribution file
- Track 1: promotion lifecycle with full debt retirement
- Track 2: postulated belief with retraction lifecycle

### What is NOT being built

- Fifth table or schema change
- Embeddings / vector search
- Live GitHub API calls or CVE feeds
- New kernel functions
- `belief_edge` writes (invalidation via evidence + RetractCascade only)
- Multi-agent orchestration
- Kafka / streaming
- RAG pipeline

---

## 2. Repository Reality

### Schema (frozen)

`db/001_schema.sql` — 4 tables:

| Table | Key columns | Constraints |
|---|---|---|
| `belief` | id, scenario_id, claim, claim_type, status, debt, final_truth | `status IN (entered, promoted, retracted)`, `claim_type IN (derived, accommodated, postulated)`, `promoted_is_debt_free` CHECK |
| `evidence` | id, scenario_id, belief_id, provenance_class, source_url, content_sha256 | `provenance_class IN (external_feed, reproducible_artifact, live_scan, operator_asserted)`, UNIQUE(belief_id, content_sha256) |
| `action_intent` | id, scenario_id, belief_id, belief_status, action, state | `state IN (live, cancelled, executed)`, `live_requires_promoted` CHECK, composite FK with ON UPDATE CASCADE |
| `belief_edge` | parent_id, child_id, kind | `kind IN (derives, contradicts)`, self-loop CHECK — **not used in MVP** |

`belief.debt` = 6 fixed items (db/001_schema.sql:14-16):
`needMap`, `needInvariant`, `needToyCheck`, `needNullModel`, `needObstruction`, `needFaithfulnessReview`

### Normalizers (frozen)

`internal/normalize/normalize.go:17-32` — closed switch, 5 source types:
`github_issue`, `github_pr`, `maintainer_comment`, `release`, `kev_entry`

`internal/normalize/types.go` — `NormalizedEvidence` struct:
Subject, Assertion, SourceType, SourceURL, Severity, ProvenanceClass, ContentSHA256, ObservedAt, DomainPayload

Source type constants (`normalize/types.go:27-33`):
```go
SourceGitHubIssue       = "github_issue"
SourceGitHubPR          = "github_pr"
SourceMaintainerComment = "maintainer_comment"
SourceRelease           = "release"
SourceKEVEntry          = "kev_entry"
```

### Derivation (frozen)

`internal/derive/derive.go:24-38` — exact claim strings per source type:

| Source type | Claim string | Classification | Assertion checked by derive |
|---|---|---|---|
| `kev_entry` | `"<subject> is vulnerable to <cves>"` | `derived` | `"vulnerable to"` (vulnPattern) |
| `release` | `"fix available for <subject>"` | `accommodated` | `"release"` (string contains) |
| `maintainer_comment` | varies by keyword priority | `accommodated` | body text (keyword extraction) |
| `github_issue` | contradiction only | `derived` | `"reproduces\|still vulnerable"` |
| `github_pr` | `"fix available for <subject>"` when merged | `accommodated` | `"fix"` (string contains) |

### Debt mapping (frozen)

`internal/belief/mapping.go:15-31` — table-driven:

| Source type | Match pattern | Debts retired |
|---|---|---|
| `kev_entry` | `(?i)vulnerable to` | `needMap` |
| `release` | `(?i)release` | `needMap`, `needInvariant` |
| `maintainer_comment` | `(?i)\b(fixed\|fix released\|patch available)\b` | `needMap`, `needInvariant` |
| `maintainer_comment` | `(?i)\b(tested\|confirmed)\b` | `needToyCheck`, `needNullModel` |
| `maintainer_comment` | `(?i)\bno regression\b` | `needNullModel`, `needObstruction` |
| `maintainer_comment` | `(?i)\b(security review\|reviewed by)\b` | `needFaithfulnessReview` |
| `github_pr` | `(?i)\bfix\b` | `needMap`, `needInvariant` |

`DebtItemsForEvidence(sourceType, assertion)` returns the **union** of all matched rules' items.

### Pipeline (frozen)

`internal/pipeline/pipeline.go:43-54`:
- `sourceTypeMap` — closed map of filename → source type
- `Run()` — reads directory, sorts `.json` files, processes each
- `ProcessEvidence` — normalize → derive → belief.Process
- `ProposeIfNew` — enters new beliefs via kernel.EnterBelief
- Hardcoded `provenance_class: "external_feed"` for all evidence

### Kernel (frozen)

`internal/kernel/kernel.go` — 8 functions:

| Function | SQL | Behavior |
|---|---|---|
| `EnterBelief` | INSERT belief | Full starting debt, status='entered' |
| `AddEvidence` | INSERT evidence | UNIQUE(belief_id, content_sha256) guard |
| `RetireDebt` | UPDATE belief SET debt = array_remove(debt, $2) | Idempotent, zero-row affected = success |
| `Promote` | UPDATE belief SET status='promoted' | CHECK enforced; 23514 → ErrPromotionBlocked |
| `IntentOnPromoted` | INSERT action_intent | FK enforced; 23503 → ErrActionOnUnpromoted |
| `RetractCascade` | One ExecuteTx: cancel before retract (I-8) | Scenario-scoped, returns retracted count |
| `EnsureBelief` | WITH existing/inserted UPSERT | Deduplicates by (scenario_id, claim) |
| `AuditLiveOnNonPromoted` | SELECT count(*) | Must = 0 in every committed state |

### CLI (frozen)

`cmd/solvent/main.go:30-37` — flags:
`--dsn`, `--schema`, `--scenario`, `--reset`, `--fixtures`, `--out`, `--falsify`

### EnsureBelief deduplication

`internal/kernel/sql.go:39-53` — deduplicates by `(scenario_id, claim)`. Two different source types producing the same claim string share one belief. Two source types producing different claim strings create separate beliefs.

---

## 3. Final Dataset

5 real records + 1 operator-asserted mechanism + 1 postulated belief. Zero synthetic records.

| # | File | Real / Operator | Source Type | Purpose | Track |
|---|---|---|---|---|---|
| 1 | `ghsa_advisory.json` | Real | `github_advisory` (NEW) | Derives vulnerability claim; retires `needMap` | 1 |
| 2 | `release_v3527.json` | Real | `release` (existing) | Pins affected deployment target as dated fact | 1 |
| 3 | `release_v3528.json` | Real | `release` (existing) | Fix-availability evidence; retires `needMap`, `needInvariant` | 1 |
| 4 | Operator review | Operator-asserted | Demo script (kernel calls) | Retires remaining debts for vulnerability belief | 1 |
| 5 | `release_v350.json` | Real | `release` (existing) | Supporting evidence for baseline deployment decision | 2 |
| 6 | `postmortem_v35.json` | Real | `postmortem` (NEW) | Invalidating evidence for deployment decision | 2 |
| — | Belief entry | Not a record | `kernel.EnterBelief` | "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" | 2 |

### Record provenance

| Record | Source | URL |
|---|---|---|
| GHSA-q8m4-xhhv-38mg | GitHub Security Advisory | `https://github.com/advisories/GHSA-q8m4-xhhv-38mg` |
| etcd v3.5.27 release | etcd releases | `https://github.com/etcd-io/etcd/releases/tag/v3.5.27` |
| etcd v3.5.28 release | etcd releases | `https://github.com/etcd-io/etcd/releases/tag/v3.5.28` |
| Operator review | Internal demo | `operator-asserted://review` (no fabricated URL) |
| etcd v3.5.0 release | etcd releases | `https://github.com/etcd-io/etcd/releases/tag/v3.5.0` |
| v3.5-data-inconsistency postmortem | etcd community | `https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/...` |

---

## 4. Track 1 — CVE Lifecycle

### Record processing order

Records are processed in directory sort order (alphabetical by filename):
1. `ghsa_advisory.json` — normalize(`github_advisory`) → derive → vulnerability claim
2. `release_v3527.json` — normalize(`release`) → derive → fix-available claim (v3.5.27)
3. `release_v3528.json` — normalize(`release`) → derive → fix-available claim (v3.5.28)

### Belief lifecycle

**Step 1 — GHSA advisory processed:**
- `normalizeGitHubAdvisory` → Subject: `"etcd-io/etcd >=3.5.0, <3.5.28"`, Assertion: `"vulnerable to CVE-2026-33413"`
- `deriveFromGitHubAdvisory` → Claim: `"etcd-io/etcd >=3.5.0, <3.5.28 is vulnerable to CVE-2026-33413"`, Classification: `derived`
- `belief.Process`: `EnsureBelief(scenarioID, claim, derived)` → belief_id_A
- `AddEvidence(scenarioID, belief_id_A, "external_feed", source_url, content_sha256)` → debt_A: `{needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `DebtItemsForEvidence("github_advisory", "vulnerable to CVE-2026-33413")` → `["needMap"]` (via new DebtMapping rule)
- `RetireDebt(belief_id_A, "needMap")` → debt_A: `{needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `Promote(belief_id_A)` → **REFUSED** — 5 debts open (SQLSTATE 23514 → ErrPromotionBlocked → swallowed)

**Step 2 — v3.5.27 release processed:**
- `normalizeRelease` → Subject: `"etcd-io/etcd v3.5.27"`, Assertion: `"release v3.5.27 published"`
- `deriveFromRelease` → Claim: `"fix available for etcd-io/etcd v3.5.27"`, Classification: `accommodated`
- `belief.Process`: `EnsureBelief(scenarioID, claim, accommodated)` → belief_id_B
- `AddEvidence` → debt_B: `{needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `DebtItemsForEvidence("release", "release v3.5.27 published")` → `["needMap", "needInvariant"]`
- `RetireDebt(belief_id_B, "needMap")` → debt_B: `{needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `RetireDebt(belief_id_B, "needInvariant")` → debt_B: `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `Promote(belief_id_B)` → **REFUSED** — 4 debts open

**Step 3 — v3.5.28 release processed:**
- Claim: `"fix available for etcd-io/etcd v3.5.28"`, Classification: `accommodated`
- `belief.Process`: `EnsureBelief` → belief_id_C
- `AddEvidence` → debt_C: `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` (needMap + needInvariant retired)
- `Promote(belief_id_C)` → **REFUSED** — 4 debts open

**State after pipeline:**

| Belief | Claim | Debts remaining | Status |
|---|---|---|---|
| A | `"... is vulnerable to CVE-2026-33413"` | `{needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` | entered |
| B | `"fix available for ... v3.5.27"` | `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` | entered |
| C | `"fix available for ... v3.5.28"` | `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` | entered |

All three promotion attempts fail with SQLSTATE 23514 (ErrPromotionBlocked).

**Step 4 — Operator review (retires remaining debts on belief A):**
- `RetireDebt(belief_id_A, "needInvariant")` → debt_A: `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`
- `RetireDebt(belief_id_A, "needToyCheck")` → debt_A: `{needNullModel, needObstruction, needFaithfulnessReview}`
- `RetireDebt(belief_id_A, "needNullModel")` → debt_A: `{needObstruction, needFaithfulnessReview}`
- `RetireDebt(belief_id_A, "needObstruction")` → debt_A: `{needFaithfulnessReview}`
- `RetireDebt(belief_id_A, "needFaithfulnessReview")` → debt_A: `{}`

**Step 5 — Promotion:**
- `Promote(belief_id_A)` → SUCCESS — debt = {}, CHECK passes
- belief A: status = 'promoted', promoted_is_debt_free = true

**Step 6 — Intent:**
- `IntentOnPromoted(scenarioID, belief_id_A, "deploy etcd v3.5.28")` → SUCCESS
- Intent state = 'live'

**Step 7 — Replay (determinism check):**
- Re-run pipeline → `EnsureBelief` dedups, `AddEvidence` dedups (UNIQUE hash) → zero new rows
- `DebtItemsForEvidence` idempotent, `RetireDebt` idempotent (array_remove on absent item)
- `Promote` idempotent (already promoted)
- `IntentOnPromoted` creates duplicate intent — **replay must be idempotent at pipeline level**
- Transcript output identical to first run → `VERDICT: PASS`

### Demo beats

1. Pipeline runs, 3 beliefs entered, 6 evidence records ingested
2. All three `Promote` attempts fail → show refusal (primary demo beat)
3. Operator review retires remaining debts on vulnerability belief → show retirement
4. `Promote` succeeds on vulnerability belief → show promotion
5. `IntentOnPromoted` succeeds → show intent
6. Replay → zero new rows → show determinism

---

## 5. Track 2 — Historical Retraction

### Baseline belief

The deployment decision for etcd v3.5.0 is entered as a `postulated` belief via `kernel.EnterBelief`. This is NOT derived from evidence — the decision itself is the starting point.

### Lifecycle

**Step 1 — Baseline belief entered:**
- `kernel.EnterBelief(scenarioID2, "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)", postulated)` → belief_id_D
- debt_D: `{needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}`

**Step 2 — Supporting evidence attached:**
- `kernel.AddEvidence(scenarioID2, belief_id_D, "external_feed", "https://github.com/etcd-io/etcd/releases/tag/v3.5.0", content_sha256_v350)`

**Step 3 — Debts retired:**
- `RetireDebt` for all 6 items (the baseline decision is fully vetted by the demo operator)

**Step 4 — Promotion:**
- `Promote(belief_id_D)` → SUCCESS — debt = {}

**Step 5 — Intent:**
- `IntentOnPromoted(scenarioID2, belief_id_D, "deploy etcd v3.5.0")` → SUCCESS
- Intent state = 'live'

**Step 6 — Postmortem ingested (via normal pipeline):**
- Pipeline processes `postmortem_v35.json` in Track 2 scenario
- `normalizePostmortem` → derives invalidating evidence claim
- `belief.Process` → belief for the postmortem is entered

**Step 7 — Retraction via --falsify:**
- `solvent --falsify <belief_id_D> --scenario <track2UUID>`
- `RetractCascade(scenarioID2, belief_id_D)`
  - Cancel: intent state → 'cancelled'
  - Retract: belief status → 'retracted'
- Return: `retracted = 1, audit_live_on_nonpromoted = 0`
- `VERDICT: PASS`

### Demo beats

1. Baseline belief entered, promoted, intent created — show the decision is live
2. Postmortem ingested — show new evidence arrives
3. `--falsify` triggers RetractCascade — show transactional retraction
4. Audit shows 0 — show invariant I-5 holds after retraction

---

## 6. Fixture Layout

### Directory structure

```
internal/derive/testdata/etcd_real/
  ghsa_advisory.json            # github_advisory (NEW source type)
  release_v3527.json            # release (existing)
  release_v3528.json            # release (existing)
  release_v350.json             # release (existing — Track 2 supporting)
  postmortem_v35.json           # postmortem (NEW source type)
  maintainer_review.json        # maintainer_comment (raw reference, not processed by pipeline)
  manifest.json                 # file digests for supply-chain integrity
  ATTRIBUTION.md                # licensing notices for all records
```

### Fixture rules

- Each `.json` file: one JSON object, top-level fields matching the target normalizer's struct
- Filenames must be registered in `sourceTypeMap` (`pipeline/pipeline.go`)
- `manifest.json`: `{"files": {"ghsa_advisory.json": "sha256:...", ...}}` — computed after all fixtures written
- `ATTRIBUTION.md`: licensing and source notices for every record
- Manifest and attribution are OUTSIDE the ingested directory — pipeline reads only `.json` files from the fixtures path

### File sizes

Target: each fixture under 2KB. Total fixture directory under 15KB.

---

## 7. Normalization Specification

### github_advisory (NEW)

**Input JSON shape:**
```json
{
  "ghsa_id": "GHSA-q8m4-xhhv-38mg",
  "summary": "Etcd: Denial of service vulnerability in etcd through multipart MIME parsing",
  "severity": "critical",
  "published_at": "2026-06-05T00:00:00Z",
  "affected": [
    {
      "package": {"ecosystem": "Go", "name": "go.etcd.io/etcd"},
      "ranges": [
        {
          "type": "SEMVER",
          "events": [
            {"introduced": "3.5.0"},
            {"fixed": "3.5.28"}
          ]
        }
      ]
    }
  ],
  "references": ["https://github.com/advisories/GHSA-q8m4-xhhv-38mg"]
}
```

**Go struct:**
```go
type githubAdvisory struct {
    GHSAID      string            `json:"ghsa_id"`
    Summary     string            `json:"summary"`
    Severity    string            `json:"severity"`
    PublishedAt string            `json:"published_at"`
    Affected    []ghsaAffected    `json:"affected"`
    References  []string          `json:"references"`
}

type ghsaAffected struct {
    Package ghsaPackage   `json:"package"`
    Ranges  []ghsaRange   `json:"ranges"`
}

type ghsaPackage struct {
    Ecosystem string `json:"ecosystem"`
    Name      string `json:"name"`
}

type ghsaRange struct {
    Type   string       `json:"type"`
    Events []ghsaEvent  `json:"events"`
}

type ghsaEvent struct {
    Introduced string `json:"introduced"`
    Fixed      string `json:"fixed"`
}
```

**Required fields:** `ghsa_id`, `summary`, `severity`, `published_at`, `affected`
**Optional fields:** `references`

**Normalized output:**
- Subject: `"<ecosystem>/<name> >=<introduced>, <<fixed>"`
- Assertion: `"vulnerable to <ghsa_id_as_cve>"` (or `"vulnerable to <ghsa_id>"` if no CVE)
- Severity: mapped from advisory severity (critical/high/medium/low/info)
- ProvenanceClass: `external_feed`
- ContentHash: `SHA256({ghsa_id, severity, summary, affected_ranges})`

### postmortem (NEW)

**Input JSON shape:**
```json
{
  "title": "etcd v3.5 Data Inconsistency Postmortem",
  "affected_versions": "v3.5.x",
  "fixed_version": "v3.5.12",
  "summary": "Data inconsistency between v3.5.x and v3.4.x due to incorrect revision counting...",
  "published_at": "2022-12-06T00:00:00Z",
  "source_url": "https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/..."
}
```

**Go struct:**
```go
type postmortemRecord struct {
    Title            string `json:"title"`
    AffectedVersions string `json:"affected_versions"`
    FixedVersion     string `json:"fixed_version"`
    Summary          string `json:"summary"`
    PublishedAt      string `json:"published_at"`
    SourceURL        string `json:"source_url"`
}
```

**Required fields:** `title`, `affected_versions`, `fixed_version`, `summary`, `published_at`, `source_url`
**Optional fields:** (none)

**Normalized output:**
- Subject: `"<affected_versions>"`
- Assertion: `"<subject> has documented data inconsistency"`
- Severity: `critical`
- ProvenanceClass: `external_feed`
- ContentHash: `SHA256({title, affected_versions, fixed_version, summary})`

### Existing source types (no changes)

| Source type | Assertion produced | Subject produced |
|---|---|---|
| `release` | `"release <version> published"` | `"<repo> <version>"` |
| `kev_entry` | `"vulnerable to <cveID>"` | `"<product> <versionAffected>"` |
| `maintainer_comment` | body text (cleaned) | `"<repository>"` |
| `github_issue` | title text | `"<repository>"` |
| `github_pr` | title text | `"<repository>"` |

---

## 8. Derivation Specification

### github_advisory (NEW)

**deriveFromGitHubAdvisory function:**
- Check: assertion must contain `"vulnerable to"` (vulnPattern)
- Claim: `"<subject> is vulnerable to <ghsa_id>"` where subject = normalized affected range
- Classification: `derived`

**Claim examples:**
- `"etcd-io/etcd >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg"`

### postmortem (NEW)

**deriveFromPostmortem function:**
- No precondition check (postmortem always produces a valid claim)
- Claim: `"<subject> has documented data inconsistency affecting <affected_versions>, fixed in <fixed_version>"`
- Classification: `derived`

**Claim examples:**
- `"v3.5.x has documented data inconsistency affecting v3.5.x, fixed in v3.5.12"`

### Existing derive functions (no changes)

| Source type | Claim produced | Classification |
|---|---|---|
| `kev_entry` | `"<subject> is vulnerable to <cves>"` | `derived` |
| `release` | `"fix available for <subject>"` | `accommodated` |
| `maintainer_comment` | varies by keyword priority | `accommodated` |
| `github_issue` | contradiction only | `derived` |
| `github_pr` | `"fix available for <subject>"` when merged | `accommodated` |

---

## 9. Debt / Promotion Specification

### DebtMapping additions (NEW)

```go
"github_advisory": {
    {Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{"needMap"}},
},
"postmortem": {
    // No debt retirement — postmortem is invalidating evidence, not confirming
},
```

### Debt lifecycle — Track 1

| Step | Belief A (vulnerability) | Belief B (fix v3.5.27) | Belief C (fix v3.5.28) |
|---|---|---|---|
| After GHSA advisory | `{needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` | — | — |
| After v3.5.27 release | same | `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` | — |
| After v3.5.28 release | same | same | `{needToyCheck, needNullModel, needObstruction, needFaithfulnessReview}` |
| After operator review | `{}` | (unchanged) | (unchanged) |
| After promote | PROMOTED | (unchanged) | (unchanged) |

### Promotion rules

- `Promote` → UPDATE belief SET status='promoted' WHERE id=$1
- CHECK `promoted_is_debt_free`: `status <> 'promoted' OR (coalesce(array_length(debt,1),0) = 0 AND NOT final_truth)`
- SQLSTATE 23514 → ErrPromotionBlocked
- `belief.Process` swallows ErrPromotionBlocked (returns nil)

### Intent gating

- `IntentOnPromoted` → INSERT action_intent with composite FK (belief_id, 'promoted') → belief(id, status)
- SQLSTATE 23503 → ErrActionOnUnpromoted
- Intent state = 'live'

---

## 10. Required Code Changes

### EXISTING CODE REUSE (frozen, no changes)

- `internal/normalize/normalize.go` — add 2 new cases to switch
- `internal/derive/derive.go` — add 2 new derive functions
- `internal/belief/mapping.go` — add `github_advisory` entry
- `internal/pipeline/pipeline.go` — add entries to `sourceTypeMap`

### ADDITIVE EXTENSION (requires Technical Lead approval)

| File | Change | Lines (est.) |
|---|---|---|
| `internal/normalize/types.go` | Add `SourceGitHubAdvisory = "github_advisory"`, `SourcePostmortem = "postmortem"` constants | ~4 |
| `internal/normalize/normalize.go` | Add `githubAdvisory` struct, `normalizeGitHubAdvisory` function, `postmortemRecord` struct, `normalizePostmortem` function, 2 cases to switch | ~120 |
| `internal/derive/derive.go` | Add `deriveFromGitHubAdvisory`, `deriveFromPostmortem` functions, 2 cases to switch | ~50 |
| `internal/belief/mapping.go` | Add `"github_advisory"` entry to `DebtMapping` | ~5 |
| `internal/pipeline/pipeline.go` | Add 5 entries to `sourceTypeMap` | ~8 |
| `cmd/operator-review/main.go` | Standalone operator review script (see Section 15) | ~60 |

**Total estimated new code:** ~250 lines across 6 files.

### FORBIDDEN / NOT NEEDED

- No schema changes (`db/001_schema.sql` frozen)
- No new tables
- No belief_edge writes
- No new kernel functions (`internal/kernel/kernel.go` frozen)
- No BEGIN/COMMIT blocks at CLI level
- No raw db.Exec writes at CLI level
- No embeddings/vector search
- No live API calls
- No RAG
- No Kafka/streaming

---

## 11. CLI / Demo Procedure

### Track 1 — CVE lifecycle

```bash
# Set DSN
export FABLE_DSN="postgresql://root@localhost:26257/solvent?sslmode=disable"

# Scenario 1: CVE lifecycle
TRACK1=$(solvent --dsn "$FABLE_DSN" --scenario 00000000-0000-0000-0000-000000000001 --reset \
  --fixtures internal/derive/testdata/etcd_real --out /tmp/track1_transcript.json)

# Verify: 3 beliefs entered, 6 evidence records, 3 promotion attempts REFUSED
# Transcript shows SQLSTATE 23514 for each promotion attempt

# Operator review: retire remaining debts on vulnerability belief
operator-review --dsn "$FABLE_DSN" --scenario 00000000-0000-0000-0000-000000000001 \
  --belief <vulnerability_belief_id> \
  --debt needInvariant --debt needToyCheck --debt needNullModel \
  --debt needObstruction --debt needFaithfulnessReview

# Re-run: promotion + intent
solvent --dsn "$FABLE_DSN" --scenario 00000000-0000-0000-0000-000000000001 \
  --fixtures internal/derive/testdata/etcd_real --out /tmp/track1_rerun.json

# Replay: same output → VERDICT: PASS
```

### Track 2 — Historical retraction

```bash
# Scenario 2: historical retraction (as-of 2022)
TRACK2=$(solvent --dsn "$FABLE_DSN" --scenario 00000000-0000-0000-0000-000000000002 --reset \
  --fixtures internal/derive/testdata/etcd_real --out /tmp/track2_transcript.json)

# Demo script: Enter baseline belief, attach evidence, retire debts, promote, create intent
# (handled by demo script or direct kernel calls)

# Falsify: retraction
solvent --dsn "$FABLE_DSN" --falsify <baseline_belief_id> --scenario 00000000-0000-0000-0000-000000000002
# → retracted 1 belief(s), audit_live_on_nonpromoted: 0, VERDICT: PASS
```

### Expected transcript output (Track 1)

```json
{
  "scenario_id": "00000000-0000-0000-0000-000000000001",
  "records_processed": 3,
  "beliefs_entered": 3,
  "evidence_attached": 3,
  "debts_retired": 3,
  "promotions_attempted": 3,
  "promotions_succeeded": 0,
  "promotions_refused": 3,
  "intents_created": 0,
  "verdict": "REFUSED"
}
```

---

## 12. Acceptance Criteria

### Functional

- [ ] Pipeline processes all 3 fixtures without error
- [ ] 3 beliefs entered with correct claim strings
- [ ] 3 evidence records attached with correct provenance
- [ ] Debt retirement matches DebtMapping rules (3 items retired: 1 needMap + 2 needMap+needInvariant)
- [ ] All 3 promotion attempts fail with SQLSTATE 23514 (ErrPromotionBlocked)
- [ ] Operator review retires remaining debts on vulnerability belief
- [ ] Promotion succeeds after operator review
- [ ] Intent created on promoted belief
- [ ] Replay produces zero new rows (determinism)
- [ ] Track 2: `--falsify` retracts 1 belief, cancels intent
- [ ] `audit_live_on_nonpromoted = 0` after every operation

### Technical

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l cmd internal` returns empty
- [ ] `scripts/check_i7.sh` passes (7 ExecuteTx sites)
- [ ] All 85 existing tests remain green
- [ ] New source types are closed in normalize switch and derive switch
- [ ] `sourceTypeMap` includes all 5 new fixture filenames
- [ ] `manifest.json` computed after all fixtures written
- [ ] `ATTRIBUTION.md` present with licensing notices

### Adversarial

- [ ] No fabricated URLs in fixtures (all real source URLs)
- [ ] No schema changes
- [ ] No new kernel functions
- [ ] No belief_edge writes
- [ ] `operator_asserted` provenance used only for demo script kernel calls, not pipeline-processed
- [ ] All claims are honest representations of the underlying evidence

---

## 13. Scope / Deadline Check

### NOT being built

- Fifth table
- Schema change
- Embeddings / vector search
- Live API calls
- RAG pipeline
- Kafka / streaming
- Multi-agent infrastructure
- New domain implementations beyond etcd

### Remains MVP

- Two tracks, two scenarios
- 5 real records + 1 operator-asserted mechanism + 1 postulated belief
- Zero fabricated evidence
- Zero edges
- Zero schema changes

### Estimated complexity

- ~6 files changed (normalize, derive, mapping, pipeline, operator-review, types)
- 2 new normalizer functions
- 2 new derive functions
- 1 new DebtMapping entry
- ~250 lines of new code
- Risk: operator-asserted debt retirement is a manual step outside the pipeline

---

## 14. Adversarial Review Prompt

```
You are an adversarial coding-review agent. Your job is to compare an
implementation against THIS design document and the frozen repository contracts.

READ these files first:
1. plans/post/etcd_real_data_implementation_spec.md (this document)
2. IMPLEMENTATION_CONTRACT.md
3. db/001_schema.sql
4. README.md
5. AGENTS.md

Then verify:

1. Schema unchanged: db/001_schema.sql must be byte-identical
2. Kernel unchanged: internal/kernel/kernel.go must have exactly 8 functions
3. No belief_edge writes: grep for "belief_edge" in all new code — zero matches
4. No raw SQL at CLI: cmd/ code uses kernel functions, not db.Exec
5. Source types closed: normalize switch and derive switch include all new types
6. DebtMapping correct: github_advisory retires needMap, postmortem retires nothing
7. Fixture format matches normalizer structs: every JSON field maps to a Go struct field
8. Claim strings match derive functions: trace normalize → derive → claim exactly
9. Debt retirement traceable: for each belief, list debts before/after each evidence
10. Replay idempotent: pipeline re-run produces zero new rows
11. RetractCascade order: cancel before retract (I-8)
12. Audit = 0 after every operation
13. No fabricated URLs: all source_url values are real
14. Provenance honest: operator_asserted only outside pipeline

Return a pass/fail verdict for each check, with file:line references for failures.
```

---

## 15. Open Question — Operator Review Mechanism

After the pipeline runs and promotion is refused with open debts, the remaining debts must be retired via direct kernel calls. How should this be implemented in the demo?

### Options

| Option | Description | Complexity | Honesty |
|---|---|---|---|
| **A** | New CLI subcommand `solvent review --scenario <uuid> --debt <item>` that calls `kernel.RetireDebt` | Medium — adds a new flag/subcommand | Honest — clear operator action label |
| **B** | Standalone Go script `cmd/operator-review/main.go` that opens DB, calls `kernel.RetireDebt` N times | Low — one new file, ~60 lines | Honest — explicitly an operator tool |
| **C** | Extend `--falsify` to also support `--retire-debt` for manual debt retirement | Low — adds one flag | Slightly overloaded semantics |
| **D** | Document the SQL directly in the demo procedure (raw `UPDATE` with receipt) | Lowest — no new code | Raw SQL violates "no raw db.Exec writes at CLI level" |

### Recommendation

**Option B** — a standalone operator review script. It keeps the kernel contract clean, doesn't overload existing CLI semantics, and the demo script can invoke it as a subprocess.

### Option B implementation sketch

```go
// cmd/operator-review/main.go
package main

import (
    "context"
    "database/sql"
    "flag"
    "fmt"
    "os"

    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/PithomLabs/solvent/internal/kernel"
)

func main() {
    dsn := flag.String("dsn", "", "CockroachDB DSN")
    scenarioID := flag.String("scenario", "", "Scenario UUID")
    beliefID := flag.String("belief", "", "Belief UUID")
    debts := flag.StringSlice("debt", nil, "Debt items to retire")
    flag.Parse()

    db, err := sql.Open("pgx", *dsn)
    if err != nil { fmt.Fprintf(os.Stderr, "open: %v\n", err); os.Exit(1) }
    defer db.Close()

    st := kernel.New(db)
    ctx := context.Background()

    for _, item := range *debts {
        if err := st.RetireDebt(ctx, *beliefID, item); err != nil {
            fmt.Fprintf(os.Stderr, "retire %s: %v\n", item, err)
            os.Exit(1)
        }
        fmt.Printf("retired: %s\n", item)
    }

    // Attempt promote
    if err := st.Promote(ctx, *beliefID); err != nil {
        fmt.Fprintf(os.Stderr, "promote: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("promoted: ok")
}
```

**Awaiting user decision on which option to implement before writing the file.**
