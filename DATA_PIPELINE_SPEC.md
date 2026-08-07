# DATA_PIPELINE_SPEC.md

**Status:** Authoritative specification for the etcd evidence pipeline.
**Authority:** This document is the single source of truth for what enters the system,
how it is transformed, and what leaves each stage. Implementation follows this spec.
**Scope:** Domain model only. No Go, no SQL, no API calls, no UI.

---

## 1. Domain Overview

An etcd cluster is infrastructure-critical. Running an unpatched, actively-exploited
version is an operational risk. Running an untested upgrade is also an operational risk.

The agent's job is to answer one question:

> **Is my current etcd version safe, and if not, what should I do about it?**

The agent cannot answer this from a single fact. It needs:

- What vulnerabilities exist?
- Which are actively exploited?
- Has the vendor released a fix?
- Is the fix deployed in my fleet?
- What is the operational cost of upgrading?

Each of these is a belief. Beliefs carry evidence. Evidence arrives asynchronously.
The agent acts on promoted beliefs — and must stop acting the instant a belief is
falsified.

This is the scenario Solvent demonstrates: an autonomous agent whose actions are
gated by a database-enforced belief ledger, not by prompt engineering or application
convention.

---

## 2. IPO Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         INPUT                                   │
│                                                                 │
│  GitHub Issues    GitHub PRs    Maintainer     Release     CISA │
│  (opened/closed)  (merged)      Comments       Metadata    KEV  │
│  Labels changed   Reverted      ("fixed in     Version     CVE  │
│  Milestones       Reviews       v3.5.14")      SHA              │
│                  requested                                    │
└───────────────────────┬─────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                        PROCESS                                  │
│                                                                 │
│  Stage 1: Normalize ──── Raw API objects → domain-neutral events│
│  Stage 2: Hash ───────── content_sha256 for dedup              │
│  Stage 3: Classify ──── provenance, timestamp, source           │
│  Stage 4: Extract ───── evidence → belief lifecycle rules       │
│  Stage 5: Resolve ───── create / strengthen / weaken / retract  │
│  Stage 6: Gate ──────── "may I act?" → database answers         │
│  Stage 7: Audit ─────── everything traceable                    │
│                                                                 │
└───────────────────────┬─────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                        OUTPUT                                   │
│                                                                 │
│  Beliefs          Evidence Chains     Action Intents            │
│  (promoted,       (provenance,       (live, cancelled,         │
│   retracted)       source trail)      executed)                 │
│                                                                 │
│  Audit Receipts   Graph State        Confidence (derived)      │
│  (machine-        (dependency        (never stored,            │
│   verifiable)      topology)          display-only)             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Input Model

### 3.1 GitHub Issue Events

| Field | Required | Type | Notes |
|---|---|---|---|
| `event_type` | yes | string | `opened`, `closed`, `reopened`, `labeled`, `unlabeled`, `milestoned` |
| `issue_number` | yes | int | Stable identifier within the repository |
| `repository` | yes | string | `etcd-io/etcd` (or other tracked repo) |
| `title` | yes | string | Issue title; may contain CVE IDs |
| `body` | yes | string | Full markdown body; may contain version references, links |
| `labels` | yes | string[] | Current labels at event time |
| `state` | yes | string | `open` or `closed` |
| `created_at` | yes | timestamp | When the event occurred (GitHub's `updated_at` for label changes) |
| `author` | no | string | GitHub username |
| `milestone` | no | string | Current milestone name |

**Normalization rules:**
- Timestamps converted to UTC, stored as `TIMESTAMPTZ`.
- `repository` normalized to `owner/name` format.
- `labels` sorted alphabetically for deterministic hashing.
- `body` stripped of Markdown formatting for claim extraction, raw preserved for evidence.

### 3.2 GitHub Pull Request Events

| Field | Required | Type | Notes |
|---|---|---|---|
| `event_type` | yes | string | `opened`, `closed`, `merged`, `review_requested`, `review_submitted` |
| `pr_number` | yes | int | Stable identifier |
| `repository` | yes | string | `owner/name` |
| `title` | yes | string | PR title |
| `body` | no | string | Description; may contain fix references |
| `state` | yes | string | `open` or `closed` |
| `merged` | yes | bool | Whether the PR was merged (vs. closed without merge) |
| `merged_at` | no | timestamp | When merged; null if not merged |
| `base_branch` | yes | string | Target branch (e.g., `main`, `release-3.5`) |
| `head_sha` | no | string | Commit SHA at merge time |

**Normalization rules:**
- PRs that are closed without merge are recorded but carry弱 evidence (may indicate rejection).
- Merged PRs carry strong evidence when linked to an issue with a CVE label.
- `head_sha` used to verify the release artifact matches the merged code.

### 3.3 Maintainer Comments

| Field | Required | Type | Notes |
|---|---|---|---|
| `comment_id` | yes | int | Stable identifier |
| `issue_or_pr_number` | yes | int | Parent issue or PR |
| `repository` | yes | string | `owner/name` |
| `author` | yes | string | GitHub username; must be a recognized maintainer for strong weight |
| `body` | yes | string | Full comment text |
| `created_at` | yes | timestamp | When the comment was posted |

**Normalization rules:**
- Author checked against a known-maintainer list (hardcoded for MVP).
- Comments containing version strings (e.g., `v3.5.14`, `v3.4.35`) are flagged.
- Comments containing "fixed", "reproduces", "regression", "needs investigation" are classified by keyword.
- Temporal context: a comment saying "Fixed in v3.5.14" followed by "Actually reproduces again" weakens the first.

### 3.4 Release Metadata

| Field | Required | Type | Notes |
|---|---|---|---|
| `version` | yes | string | Semantic version (e.g., `v3.5.14`) |
| `tag` | yes | string | Git tag name |
| `commit_sha` | yes | string | Commit SHA the tag points to |
| `released_at` | yes | timestamp | When the release was published |
| `prerelease` | yes | bool | Whether this is a pre-release |
| `assets` | no | string[] | Download URLs of release artifacts |

**Normalization rules:**
- Version strings normalized: leading `v` stripped for comparison, stored with `v` prefix.
- Pre-release versions carry弱 evidence for promotion but may carry strong evidence for debt retirement (the fix exists, even if not yet stable).
- `commit_sha` used to verify PR merge matches release artifact.

### 3.5 External References (CISA KEV / CVE)

| Field | Required | Type | Notes |
|---|---|---|---|
| `cve_id` | yes | string | CVE identifier (e.g., `CVE-2024-12345`) |
| `vendor` | yes | string | `etcd` |
| `product` | yes | string | `etcd` |
| `version_affected` | yes | string | Version range or specific version |
| `version_fixed` | no | string | Version that contains the fix |
| `kev_date` | no | timestamp | Date added to CISA KEV (null if not in KEV) |
| `description` | yes | string | Brief description of the vulnerability |
| `source_url` | yes | string | URL to the advisory |

**Normalization rules:**
- CVE IDs validated against pattern `CVE-\d{4}-\d{4,}`.
- `version_affected` parsed into a comparison-friendly format (semver tuples).
- `kev_date` being non-null is the strongest indicator of active exploitation.
- Multiple sources for the same CVE are merged by `cve_id`; `content_sha256` deduplicates identical snapshots.

---

## 4. Canonical Evidence Model

### 4.1 Raw Input

The unmodified data from the upstream source. Stored as JSON in the evidence
table's snapshot column. Never parsed or modified after ingestion — this is the
audit trail.

Fields:
- **source** — identifier for the upstream system (e.g., `github`, `cisa_kev`)
- **event_type** — what happened (e.g., `issue.closed`, `pr.merged`)
- **payload** — unmodified upstream JSON
- **fetched_at** — when this was pulled from the source

### 4.2 Normalized Evidence

The domain-neutral transformation of a raw event. This is what the claim extraction
stage operates on. Every upstream source maps to this same structure.

Fields:
- **id** — content-addressed (SHA-256 of normalized form), used as primary key
- **source_url** — canonical URL to the original observation
- **source_type** — the upstream category (e.g., `github_issue`, `github_pr`,
  `maintainer_comment`, `release`, `kev_entry`)
- **content_sha256** — hash of the normalized content (for deduplication)
- **observed_at** — when this was observed upstream
- **ingested_at** — when this entered Solvent
- **provenance_class** — one of: `external_feed`, `reproducible_artifact`,
  `live_scan`, `operator_asserted` (MVP uses `external_feed` only)
- **subject** — what this is about (e.g., `etcd v3.5.14`)
- **assertion** — what this claims (e.g., `contains fix for CVE-2024-12345`)
- **severity** — one of: `critical`, `high`, `medium`, `low`, `info`
- **confidence** — derived strength signal (never stored in the ledger)
- **domain_payload** — JSON blob with source-specific fields (opaque to kernel)

### 4.3 Ledger Evidence

The normalized evidence written to the evidence table. One row per unique observation.

Field mapping (normalized → ledger):
- **id** → `evidence.id` (UUID, primary key)
- **scenario_id** → `evidence.scenario_id` (the evaluation context)
- **belief_id** → `evidence.belief_id` (the belief this evidence supports/contradicts)
- **provenance_class** → `evidence.provenance_class` (`external_feed` for MVP)
- **source_url** → `evidence.source_url` (canonical upstream URL)
- **domain_payload** → `evidence.snapshot` (JSON, unmodified source payload)
- **content_sha256** → `evidence.content_sha256` (deduplication hash, NOT NULL)
- **observed_at** → `evidence.source_observed_at` (when observed upstream)
- **ingested_at** → `evidence.ingested_at` (when ingested into Solvent)

**Deduplication:** Two evidence rows with the same `content_sha256` for the same
`belief_id` are duplicates. The ingestor must check before insert. If a duplicate is
found, the existing row is not modified — evidence is append-only.

---

## 5. Claim Extraction

Claim extraction is the transformation from evidence to belief lifecycle actions.
It is a deterministic rule engine, not an LLM. The rules are domain-specific;
the mechanism is domain-agnostic.

### 5.1 Rule: CVE in KEV → Enter Belief

**Trigger:** A new entry appears in CISA KEV for an etcd version.

**Action:**
1. Check if a belief with claim matching `"etcd {version} is vulnerable to {cve_id}"` already exists in the scenario.
2. If not: `EnterBelief` with claim type `derived`, debt = `FullDebt`.
3. `AddEvidence` with the KEV entry as evidence.
4. Retire debt items as evidence permits (see 5.3).

### 5.2 Rule: Fix Released → Strengthen Belief

**Trigger:** A release metadata event shows a version containing a fix for a known CVE.

**Action:**
1. Find the belief for the vulnerable version.
2. `AddEvidence` with the release metadata.
3. If the fix version is ≥ the vulnerable version, retire `needMap` (the mapping is known).
4. If the fix has been tested by the community (maintainer comments confirm), retire `needInvariant` and `needToyCheck`.

### 5.3 Rule: Maintainer Confirms → Retire Debt

**Trigger:** A maintainer comment confirms a fix is available and tested.

**Action:**
1. Match the comment to the relevant belief by CVE ID or version reference.
2. `RetireDebt` for the appropriate item:
   - "Fixed in v3.5.14" → retire `needMap`
   - "Tested in staging" → retire `needInvariant`, `needToyCheck`
   - "Confirmed no regression" → retire `needNullModel`, `needObstruction`
   - "Reviewed by security team" → retire `needFaithfulnessReview`

### 5.4 Rule: Contradicting Evidence → Weaken or Retract

**Trigger:** A maintainer comment says "Actually reproduces again" or a new CVE is filed
against the supposedly-fixed version.

**Action:**
1. If the belief is still `entered`: add evidence that contradicts the prior evidence. No state change yet — the belief stays `entered` with whatever debt remains.
2. If the belief was `promoted`: `RetractCascade` from this belief. This is the demo path — falsifying evidence drives a live cascade.

### 5.5 Rule: All Debt Retired → Ready for Promotion

**Trigger:** `RetireDebt` removes the last item from `belief.debt`.

**Action:**
The belief is now promotable. The Claim Agent calls `Promote`. If the schema's
`promoted_is_debt_free` CHECK accepts, the belief moves to `promoted` and the
Security Agent may act on it.

### 5.6 Duplicate Detection

**Trigger:** A new belief's claim is semantically similar to an existing belief in the
same scenario.

**Action:**
1. Compute cosine distance between the new claim's embedding and existing entered beliefs' embeddings.
2. If distance < threshold (0.15 for MVP): the new evidence is attached to the existing belief via `AddEvidence`, not a new `EnterBelief`.
3. If distance ≥ threshold or no embedding available: insert as a new belief.

This is the only place embeddings participate in belief semantics. The ledger is
fully functional with every embedding NULL — dedup degrades to exact-match on
`content_sha256` only.

---

## 6. Belief Lifecycle

### 6.1 States

| State | Meaning | Allowed Actions |
|---|---|---|
| `entered` | Claim has arrived at the door. Carries full starting debt. Unpromoted. | Add evidence, retire debt, promote (if debt-free). |
| `promoted` | Debt is retired. Agent may act on this belief. | Add evidence (may trigger retraction), receive intents. |
| `retracted` | Falsifying evidence arrived. Belief is dead. | None — this is a terminal state. |

### 6.2 Transitions

```
                    ┌──────────────────────────────────┐
                    │                                  │
                    ▼                                  │
              ┌──────────┐   all debt retired   ┌──────────────┐
              │ ENTERED  │ ────────────────────► │  PROMOTED    │
              └──────────┘   + Promote()         └──────────────┘
                    │                                  │
                    │  falsifying evidence             │  falsifying evidence
                    │  (rare: direct retract)          │  + RetractCascade()
                    │                                  │
                    ▼                                  ▼
              ┌──────────┐                     ┌──────────────┐
              │RETRACTED │◄────────────────────│  RETRACTED   │
              └──────────┘  RetractCascade()   └──────────────┘
```

### 6.3 Debt Items

The six starting debt items represent epistemic obligations that must be retired
before promotion:

| Item | Meaning | Retired By |
|---|---|---|
| `needMap` | The vulnerability-to-version mapping is known | CVE advisory with affected/fixed versions |
| `needInvariant` | The behavioral invariant is verified | Maintainer confirmation of fix behavior |
| `needToyCheck` | A minimal reproduction exists | Test case or reproducer confirmed |
| `needNullModel` | The baseline (no-fix) behavior is characterized | Understanding of what happens without the fix |
| `needObstruction` | The obstruction path is identified | Root cause analysis confirmed |
| `needFaithfulnessReview` | Security review is complete | Security team sign-off or maintainer assertion |

### 6.4 Entry Conditions

A belief enters the system when:
1. New evidence arrives that makes a claim relevant.
2. No existing belief in the same scenario covers the same claim (dedup check).
3. The claim is typed as `derived` (from evidence), `accommodated` (merged from existing), or `postulated` (hypothesized).

A belief is **never** entered at `promoted` — all beliefs enter at `entered` with full
debt. Promotion is earned, not assumed.

---

## 7. Intent Lifecycle

### 7.1 States

| State | Meaning | Allowed Transitions |
|---|---|---|
| `live` | Agent has proposed an action on a promoted belief. | → `cancelled` (belief retracted), → `executed` (action completed). |
| `cancelled` | The belief this intent rode on has been retracted. Terminal. | None. |
| `executed` | The action was carried out. Terminal (MVP does not model post-execution). | None. |

### 7.2 Transitions

```
                 ┌────────────────────────────────┐
                 │                                │
                 ▼                                │
           ┌──────────┐  belief retracted   ┌──────────────┐
           │   LIVE   │ ──────────────────► │  CANCELLED   │
           └──────────┘  RetractCascade()   └──────────────┘
                 │
                 │  action completed
                 ▼
           ┌──────────┐
           │ EXECUTED │
           └──────────┘
```

### 7.3 The Gate

A `live` intent **cannot** exist unless its belief is currently `promoted`. This is
enforced by:
1. The composite FK: `(belief_id, belief_status) REFERENCES belief(id, status) ON UPDATE CASCADE`.
2. The CHECK constraint: `live_requires_promoted: state <> 'live' OR belief_status = 'promoted'`.

When a belief is retracted, `ON UPDATE CASCADE` propagates the new status into the
intent row, which then violates `live_requires_promoted` unless the intent was already
cancelled. This is I-4, proven on CockroachDB at M0.

### 7.4 Action Types (MVP)

| Action | Trigger | Description |
|---|---|---|
| `notify_oncall` | Belief promoted with CVE in KEV | Page the on-call engineer |
| `defer_patch` | Belief promoted with fix available | Schedule patch deployment |
| `page_incident_comriminator` | Belief promoted, actively exploited | Escalate to incident response |
| `stand_down` | Belief retracted | Cancel the previous escalation |

---

## 8. Graph Semantics

### 8.1 Nodes

| Node Type | Table | Meaning |
|---|---|---|
| **Evidence** | `evidence` | A factual observation from an upstream source. |
| **Belief** | `belief` | A claim about the world, carrying evidence and debt. |
| **Action Intent** | `action_intent` | An action the agent proposes or has fired. |

### 8.2 Edges

| Edge Type | Table | Meaning |
|---|---|---|
| **Supports** | `evidence.belief_id` (FK) | This evidence supports this belief. |
| **Derives** | `belief_edge` (`kind='derives'`) | This belief depends on that belief. |
| **Contradicts** | `belief_edge` (`kind='contradicts'`) | This belief contradicts that belief. |
| **Governs** | `action_intent.belief_id` (FK) | This intent is governed by this belief. |

### 8.3 Traversal

The dependency graph is traversed via `belief_edge` edges. The cascade
(`RetractCascade`) follows `derives` edges transitively using a recursive CTE.
Traversal is scoped to `scenario_id` — edges leaving the scenario are not followed.

### 8.4 Status Propagation

When a belief is retracted:
1. All `live` intents on the belief and its transitive descendants are cancelled.
2. The belief and its transitive descendants are set to `retracted`.
3. `ON UPDATE CASCADE` propagates the new status into intent rows.
4. The whole operation is one transaction (I-8).

---

## 9. Outputs

### 9.1 Beliefs

| Output | Consumer | Description |
|---|---|---|
| Promoted beliefs | Security Agent | May be acted on. Carries evidence chain. |
| Retracted beliefs | Audit log, demo UI | Was true, now dead. Carries the falsifying evidence. |
| Entered beliefs | Internal | Pending debt retirement. Not yet actionable. |

### 9.2 Evidence Chains

| Output | Consumer | Description |
|---|---|---|
| Full provenance chain | Audit log, demo UI | Evidence → Belief → Action trace. Every action traceable to its evidence. |
| Per-belief evidence count | Demo UI | Strength indicator (more evidence = stronger belief). |

### 9.3 Action Intents

| Output | Consumer | Description |
|---|---|---|
| Live intents | Security Agent, demo UI | Currently active actions. |
| Cancelled intents | Audit log | Actions that were stopped by a retraction. |
| Audit count | Demo UI, verification | `AuditLiveOnNonPromoted` — must be 0 in committed state. |

### 9.4 Receipts

| Output | Consumer | Description |
|---|---|---|
| SQLSTATE errors | Verification scripts, demo UI | Raw database refusals. Never paraphrased. |
| Cascade receipts | Demo UI | Number of beliefs retracted, intents cancelled. |
| Promotion receipts | Audit log | Timestamp of promotion, debt state at promotion. |

### 9.5 Graph State

| Output | Consumer | Description |
|---|---|---|
| Node statuses | Demo UI | Color-coded by state (green/gray/red). |
| Edge statuses | Demo UI | Live/cancelled per intent. |
| Dependency depth | Audit | How deep the belief chain goes. |

---

## 10. Domain Independence

### etcd-Specific (EvidenceFeed implementations)

These concepts belong to the etcd domain and must never appear in the kernel:

- CVE IDs, CISA KEV catalog
- etcd version strings, release tags
- GitHub issue/PR event types
- Maintainer identity and comment semantics
- "vulnerable", "patched", "exploited"
- `notify_oncall`, `defer_patch`, `page_incident_comminator`
- Version comparison logic (semver)
- The specific six debt items (though the mechanism of debt is generic)

### Kernel-Generic (internal/kernel)

These concepts belong to the transactional belief ledger and are domain-agnostic:

- Belief, evidence, action_intent (the three tables + belief_edge)
- Belief lifecycle: entered → promoted → retracted
- Intent lifecycle: live → cancelled/executed
- Debt retirement and promotion gating
- Cascade: transitive descendant retraction in one transaction
- Scenario scoping
- Composite FK with ON UPDATE CASCADE
- Audit: `AuditLiveOnNonPromoted`
- Serialization-failure retry (database-driven, not application-managed)

### Bridge (evidence pipeline)

These concepts mediate between domain and kernel:

- Normalized evidence model (domain-neutral transformation of raw input)
- Evidence feed contract (implemented per domain, kernel-agnostic)
- Claim extraction rules (domain-specific, kernel-agnostic)
- Duplicate detection (embedding similarity or content hash)

---

## 11. Future EvidenceFeeds

The kernel never names a domain. Any new evidence source implements the
`EvidenceFeed` interface and produces `NormalizedEvidence` objects.

### GitHub Security Advisories

```
Source:     GitHub Advisory Database
Events:     advisory published, advisory updated, advisory withdrawn
Normalization: map to CVE ID, affected versions, fixed versions
Claims:     "etcd {version} is vulnerable to {GHSA-ID}" or "{CVE-ID}"
Debt retirement: same rules as KEV
```

### NVD (National Vulnerability Database)

```
Source:     NVD API 2.0
Events:     CVE published, CVE updated (CVSS score change)
Normalization: map to CVE ID, CVSS vectors, affected CPEs
Claims:     same as KEV, with CVSS severity as confidence signal
Debt retirement: CVSS score ≥ 9.0 retires needFaithfulnessReview automatically
```

### Package Registries (npm, PyPI, Go modules)

```
Source:     registry advisory feeds (e.g., GitHub Dependabot, OSV)
Events:     vulnerability published, patch available
Normalization: map to package name + version, CVE if assigned
Claims:     "{package} {version} is vulnerable to {CVE}"
Unique:     version ranges are more complex than single-version CVEs
```

### Logistics / Supply Chain

```
Source:     shipping APIs, warehouse management systems
Events:     shipment delayed, customs hold, temperature excursion
Normalization: map to shipment ID, expected vs actual arrival
Claims:     "shipment {ID} will arrive late" or "cargo temperature exceeded threshold"
Debt retirement: carrier confirmation retires needMap
```

### Internal Scan Results

```
Source:     vulnerability scanners (Trivy, Grype, Snyk)
Events:     scan completed, new finding, finding resolved
Normalization: map to image tag + CVE, affected package
Claims:     "image {tag} contains {CVE} in {package}"
Unique:     same CVE may appear across multiple images — different beliefs per image
```

---

## Constraints (from prompt.md)

- No implementation.
- No SQL.
- No Go.
- No API calls.
- No UI.
- No demo.
- No architecture redesign.
- Keep the kernel completely domain-independent.
- Favor explicit domain semantics over implementation details.
