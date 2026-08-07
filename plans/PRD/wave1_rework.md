# WAVE1_REWORK.md — Comprehensive Documentation

**Status:** Complete. All Technical Lead findings resolved.
**Authority:** Implements `PLAN3_CODE_REVIEW.md` changeset per `PLAN3_CODE_prompt.md`.
**Scope:** Wave 1 only — `internal/normalize`, fixtures, and tests.

---

## 1. Executive Summary

Wave 1 (`internal/normalize`) was implemented and reworked per the Technical Lead's accepted findings. The rework targeted 7 specific changes — 2 P0 blockers, 2 P1 requirements, and 3 should-fix improvements — without altering the package's public API, architecture, or scope.

**Key outcomes:**
- `Normalize()` is now a pure function — no clock reads, no fabricated timestamps
- All spec-required fields are enforced — no silent `/issues/0` artifacts
- Determinism testing is strengthened — canonicalization, pinned golden hash, negative fixtures
- Severity classification uses word-boundary matching — no false positives from substring matches
- Maintainer registry is immutable — no exported mutable map
- KEV URLs are validated — no `file:` or `javascript:` injection vectors

---

## 2. Review Chain

```
Wave 1 Implementation
    │
    ▼
PLAN3_CODE_REVIEW.md (adversarial review — 4 passes)
    │
    ▼
plan3_review.md (Technical Lead acceptance — accepts some, defers some)
    │
    ▼
PLAN3_CODE_prompt.md (implementation instructions — 7 changes)
    │
    ▼
Wave 1 Rework (this document)
    │
    ▼
Wave 1 Freeze (pending)
```

### Traceability Matrix

| Review Finding | Technical Lead Verdict | Change ID | Status |
|---|---|---|---|
| B1: `IngestedAt` is `time.Now()` | Accepted (P0) | B1 | ✅ Resolved |
| B2: Fabricated `ObservedAt` | Accepted (P0) | B2 | ✅ Resolved |
| Req1: Required-field validation | Accepted (P1) | Req1 | ✅ Resolved |
| Req2: Observation identity in hash | Demoted to design decision | — | Deferred |
| Req3: Raw payload storage | Demoted to spec clarification | — | Deferred |
| Req4: Canonicalization tests | Accepted | Req4 | ✅ Resolved |
| S1: Severity classifier | Accepted (should fix) | S1 | ✅ Resolved |
| S2: Mutable maintainer map | Accepted (should fix) | S2 | ✅ Resolved |
| S3: KEV URL validation | Accepted (should fix) | S3 | ✅ Resolved |

---

## 3. Change Catalog

### B1 (P0) — Remove `time.Now()` from `IngestedAt`

**What:** The `IngestedAt` field in `NormalizedEvidence` was set to `time.Now().UTC()` inside every normalizer. This violated the documented determinism contract: *"identical input always produces identical output."*

**Why:** `IngestedAt` represents when the evidence entered Solvent — not when it was observed upstream. The normalization layer does not own ingestion time. The caller (Wave 2/3 feed layer) does.

**Where:** 5 locations in `normalize.go` (lines 72, 145, 216, 284, 365 in the original).

**Before:**
```go
norm := NormalizedEvidence{
    // ...
    IngestedAt: time.Now().UTC(),  // ← violates purity
    // ...
}
```

**After:**
```go
norm := NormalizedEvidence{
    // ...
    // IngestedAt left as zero value — caller sets it
    // ...
}
```

**Behavioral change:** `NormalizedEvidence.IngestedAt` is now always `time.Time{}` (zero) after normalization. The caller must set it.

---

### B2 (P0) — Remove Fabricated `ObservedAt`

**What:** Two normalizers fabricated `ObservedAt = time.Now().UTC()` when upstream observation time was unavailable:
- `normalizeGitHubPR` when `merged=false` or `merged_at` empty
- `normalizeKEVEntry` when `dateAdded` empty

**Why:** Spec §4.2 defines `observed_at` as *"when this was observed upstream."* The ingest clock is not upstream time. Fabricated timestamps violate the audit contract: *"Unknown behavior should fail loudly."*

**Where:** 2 fallback paths in `normalize.go`.

**Before (PR):**
```go
if g.Merged && g.MergedAt != "" {
    observedAt, err = parseTimestamp(g.MergedAt)
} else {
    observedAt = time.Now().UTC()  // ← fabricated
}
```

**After (PR):**
```go
if g.Merged && g.MergedAt != "" {
    observedAt, err = parseTimestamp(g.MergedAt)
} else if !g.Merged {
    observedAt = time.Time{}  // zero — unmerged PRs have no observation time
} else {
    return NormalizedEvidence{}, fmt.Errorf("normalize github pr: merged_at is required for merged PRs")
}
```

**Before (KEV):**
```go
if k.KEVDate != "" {
    observedAt, err = parseTimestamp(k.KEVDate)
} else {
    observedAt = time.Now().UTC()  // ← fabricated
}
```

**After (KEV):**
```go
if k.KEVDate == "" {
    return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: dateAdded is required")
}
observedAt, err := parseTimestamp(k.KEVDate)
```

**Behavioral change:** Missing upstream timestamps now produce errors (KEV) or zero values (unmerged PRs) instead of fabricated wall-clock times.

---

### Req1 (P1) — Enforce All Spec-Required Fields

**What:** Required-field validation was inconsistent with `DATA_PIPELINE_SPEC.md` §3. Some fields marked "required" in the spec were silently accepted as zero values, producing corrupted output (e.g., `SourceURL = ".../issues/0"`).

**Why:** Spec §3 marks specific fields as required for each source type. The implementation validated a hand-picked subset. Missing fields corrupted URLs, hashes, and downstream dedup.

**Where:** All 5 normalizers in `normalize.go`.

**Validation matrix:**

| Source | Field | Spec Required | Before | After |
|---|---|---|---|---|
| `githubIssue` | `event_type` | yes | ❌ | ✅ |
| `githubIssue` | `issue_number` | yes | ❌ (>0) | ✅ (>0) |
| `githubIssue` | `labels` | yes | ❌ | ✅ (non-nil) |
| `githubIssue` | `state` | yes | ❌ | ✅ |
| `githubPR` | `event_type` | yes | ❌ | ✅ |
| `githubPR` | `pr_number` | yes | ❌ (>0) | ✅ (>0) |
| `githubPR` | `state` | yes | ❌ | ✅ |
| `githubPR` | `base_branch` | yes | ❌ | ✅ |
| `maintainerComment` | `comment_id` | yes | ❌ (>0) | ✅ (>0) |
| `maintainerComment` | `issue_or_pr_number` | yes | ❌ (>0) | ✅ (>0) |
| `kevEntry` | `versionAffected` | yes | ❌ | ✅ |

**Behavioral change:** Missing required fields now return errors. No malformed normalized objects are produced.

---

### Req4 (P1) — Strengthen Determinism Testing

**What:** Tests asserted `ContentSHA256 != ""` but never a specific value. A silent change to hash inputs would go undetected. No canonicalization test or negative fixture existed.

**Why:** The Technical Lead review required: reordered JSON fixture, negative fixture, pinned golden hash. These catch silent regressions in the hashing logic.

**Where:** `normalize_test.go` (3 new tests) + 2 new fixtures.

**New tests:**

| Test | Purpose |
|---|---|
| `TestCanonicalizationDeterminism` | Same JSON with reordered fields → same hash |
| `TestPinnedGoldenHash` | Exact SHA-256 matches `goldenGitHubIssueSHA256` constant |
| `TestNormalizeNegativeMissingFields` | Negative fixture with missing required fields → error |

**Golden hash:** `3a4d736728742d74a7341d7c534b043234c2d7076f00916aeb7f8e26cddfcc5e`

**Behavioral change:** Test coverage increased from 8 to 11 tests. Hash regressions are now caught.

---

### S1 (Should Fix) — Word-Boundary Severity Matching

**What:** `classifySeverity` used `strings.Contains` on lowercased text. Small substrings ("low" inside "follow", "high" inside "highly") produced false severity classifications.

**Why:** The severity classifier feeds the graph visualization and demo. False positives from ordinary prose manufacture incident signals.

**Where:** `classifySeverity` function in `normalize.go`.

**Before:**
```go
case strings.Contains(combined, "low") || strings.Contains(combined, "minor"):
    return SeverityLow
```

**After:**
```go
var severityLow = regexp.MustCompile(`\b(low|minor)\b`)

case severityLow.MatchString(combined):
    return SeverityLow
```

**Behavioral change:** "follow" no longer triggers `SeverityLow`. "highly" no longer triggers `SeverityHigh`. Only whole-word matches classify severity.

---

### S2 (Should Fix) — Immutable Maintainer Registry

**What:** `KnownMaintainers` was an exported mutable `map[string]bool`. Consumers could mutate it, and concurrent writers would race.

**Why:** The maintainer list backs the `is_maintainer` trust decision in Wave 2. An exported mutable map is a correctness hazard.

**Where:** `types.go`.

**Before:**
```go
var KnownMaintainers = map[string]bool{
    "hexfusion": true,
    // ...
}
```

**After:**
```go
var knownMaintainers = map[string]bool{  // unexported
    "hexfusion": true,
    // ...
}

func IsMaintainer(name string) bool {  // read-only accessor
    return knownMaintainers[name]
}
```

**Behavioral change:** `KnownMaintainers` is no longer accessible. `IsMaintainer(name)` is the public API.

---

### S3 (Should Fix) — Validate KEV URLs

**What:** The KEV `reference` field was passed straight into `evidence.source_url` without scheme validation. A hostile feed could emit `file:` or `javascript:` URLs.

**Why:** The URL reaches the graph/demo UI. Unvalidated URLs are an injection vector.

**Where:** `normalizeKEVEntry` in `normalize.go`.

**After:**
```go
var urlPattern = regexp.MustCompile(`^https?://`)

// In normalizeKEVEntry:
if !urlPattern.MatchString(k.SourceURL) {
    return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: reference must be an HTTP(S) URL")
}
```

**Behavioral change:** Non-HTTP(S) URLs in KEV entries now return errors.

---

## 4. Purity Contract

Before the rework, `Normalize()` was documented as pure but was not:

> *"identical input always produces identical output and identical content_sha256"*
> — `types.go` line 4–6

This was false. Same fixture, two calls → different `IngestedAt`. The dedup hash was stable (verified), but the documented contract was broken.

After the rework, `Normalize()` is genuinely pure:

| Property | Before | After |
|---|---|---|
| Same input → same `ContentSHA256` | ✅ | ✅ |
| Same input → same `IngestedAt` | ❌ (`time.Now()`) | ✅ (zero) |
| Same input → same `ObservedAt` | ❌ (fabricated fallback) | ✅ (error or zero) |
| Same input → same output | ❌ | ✅ |
| Clock reads | 5+ per call | 0 per call |
| Side effects | 1 (clock) | 0 |

---

## 5. Required-Field Enforcement

The rework enforces every field marked "required" in `DATA_PIPELINE_SPEC.md` §3.1–3.5.

**GitHub Issue (§3.1):**
- `event_type` — must be non-empty
- `issue_number` — must be > 0
- `repository` — must be non-empty
- `title` — must be non-empty
- `labels` — must be non-nil (empty slice is OK)
- `state` — must be non-empty
- `created_at` — must be non-empty and parseable

**GitHub PR (§3.2):**
- `event_type` — must be non-empty
- `pr_number` — must be > 0
- `repository` — must be non-empty
- `title` — must be non-empty
- `state` — must be non-empty
- `base_branch` — must be non-empty
- `merged_at` — required if `merged=true`

**Maintainer Comment (§3.3):**
- `comment_id` — must be > 0
- `issue_or_pr_number` — must be > 0
- `repository` — must be non-empty
- `author` — must be non-empty
- `body` — must be non-empty
- `created_at` — must be non-empty and parseable

**Release (§3.4):**
- `version` — must be non-empty
- `tag` — must be non-empty
- `commit_sha` — must be non-empty
- `released_at` — must be non-empty and parseable

**KEV Entry (§3.5):**
- `cveID` — must match `CVE-\d{4}-\d{4,}`
- `vendor` — must be non-empty
- `product` — must be non-empty
- `versionAffected` — must be non-empty
- `shortDescription` — must be non-empty
- `reference` — must be non-empty and `https?://`
- `dateAdded` — must be non-empty and parseable

---

## 6. Determinism & Testing

### Golden Hash Pinning

The golden hash `3a4d736728742d74a7341d7c534b043234c2d7076f00916aeb7f8e26cddfcc5e` is the SHA-256 of the normalized `github_issue.json` fixture. Any change to:
- the hash input fields (`source`, `title`, `body`, `state`, `labels`)
- the JSON marshaling order
- the `computeSHA256` function

will break `TestPinnedGoldenHash`, catching silent regressions.

### Canonicalization Test

`TestCanonicalizationDeterminism` loads two versions of the same GitHub issue:
- `github_issue.json` — original field order
- `github_issue_reordered.json` — same data, different JSON key order

Both must produce identical `ContentSHA256`. This proves that JSON key ordering does not affect the hash.

### Negative Fixture

`github_issue_missing_fields.json` contains a GitHub issue with only `event_type`, `repository`, and `title` — missing `issue_number`, `labels`, `state`, and `created_at`. `TestNormalizeNegativeMissingFields` asserts that normalization fails with an error.

---

## 7. Severity Classifier

### Before (substring matching)

```go
strings.Contains(combined, "low")    // "follow" → true (false positive)
strings.Contains(combined, "high")   // "highly" → true (false positive)
```

### After (word-boundary matching)

```go
severityLow.MatchString(combined)    // `\b(low|minor)\b` — "follow" → false
severityHigh.MatchString(combined)   // `\b(high|exploit|actively exploited)\b` — "highly" → false
```

### Classification table

| Pattern | Matches | Does Not Match |
|---|---|---|
| `\b(critical\|rce\|remote code execution)\b` | "critical vulnerability", "rce in parser" | "critically important" |
| `\b(high\|exploit\|actively exploited)\b` | "high severity", "actively exploited" | "highly available" |
| `\b(medium\|moderate)\b` | "medium severity", "moderate risk" | "intermediate" |
| `\b(low\|minor)\b` | "low severity", "minor issue" | "follow-up", "allowlist" |

---

## 8. Trust Boundary Improvements

### S2: Immutable Maintainer Registry

The maintainer list is a trust boundary — Wave 2 uses it to decide whether a comment carries maintainer authority. Making it immutable prevents:
- Accidental mutation from test code
- Race conditions from concurrent normalization
- API surface area for external manipulation

The `IsMaintainer(name string) bool` function provides read-only access without exposing the underlying map.

### S3: KEV URL Validation

KEV entries carry a `reference` URL that becomes `evidence.source_url`. Without validation, a hostile CISA KEV feed could inject:
- `file:///etc/passwd` — local file access
- `javascript:alert(1)` — XSS in demo UI
- `data:text/html,...` — content injection

The `^https?://` regex ensures only HTTP(S) URLs pass normalization.

---

## 9. Deferred Decisions

### Req2: Observation Identity in `content_sha256`

**Question:** Does `content_sha256` identify **content** or **observations**?

**Technical Lead decision:** Demoted to design decision. The current hash represents semantic content — identical text produces identical hash. Changing this would alter ledger dedup semantics. Decision deferred to after Wave 2.

**Impact on Wave 2:** Wave 2 must understand that `content_sha256` deduplicates on semantic content, not observation identity. Two distinct issues with identical text will produce the same hash.

### Req3: Raw vs. Canonical Payload

**Question:** Should `DomainPayload` store the raw input bytes or the canonicalized form?

**Technical Lead decision:** Demoted to specification clarification. The spec maps `domain_payload` → `evidence.snapshot` = "JSON, unmodified source payload." The current implementation stores a canonicalized projection. Changing this requires a spec update first.

**Impact on Wave 2:** Wave 2 receives canonicalized `DomainPayload`. If the spec is later clarified to require raw bytes, the normalization layer must be updated.

---

## 10. Impact on Wave 2

### What Wave 2 Can Assume

1. **`Normalize()` is pure.** Same input → same output, always. No clock reads.
2. **`IngestedAt` is zero.** Wave 2 must set it (e.g., from `feed.Event.FetchedAt`).
3. **`ObservedAt` is either parsed from upstream or zero.** Never fabricated.
4. **All required fields are validated.** No malformed objects reach Wave 2.
5. **`ContentSHA256` is deterministic.** Same semantic content → same hash.
6. **`IsMaintainer(name)` is the trust API.** Do not access the underlying map.

### IngestedAt Contract

```
Normalize(raw, sourceType) → NormalizedEvidence{IngestedAt: time.Time{}}
                                      │
                                      ▼
                              Caller sets IngestedAt
                              (e.g., from feed.Event.FetchedAt)
```

### ObservedAt Contract

| Source | ObservedAt |
|---|---|
| GitHub Issue | Parsed from `created_at` |
| GitHub PR (merged) | Parsed from `merged_at` |
| GitHub PR (unmerged) | Zero (no upstream observation time) |
| Maintainer Comment | Parsed from `created_at` |
| Release | Parsed from `released_at` |
| KEV Entry | Parsed from `dateAdded` |

---

## 11. Test Matrix

| # | Test | Type | Input | Asserts |
|---|---|---|---|---|
| 1 | `TestNormalizeGitHubIssue` | Positive | `github_issue.json` | Fields, URL, hash, zero IngestedAt |
| 2 | `TestNormalizeGitHubPR` | Positive | `github_pr.json` | Fields, URL, hash |
| 3 | `TestNormalizeMaintainerComment` | Positive | `maintainer_comment.json` | Fields, URL, hash |
| 4 | `TestNormalizeRelease` | Positive | `release.json` | Fields, subject, hash |
| 5 | `TestNormalizeKEVEntry` | Positive | `kev_entry.json` | Fields, severity, hash |
| 6 | `TestHashDeterminism` | Determinism | `github_issue.json` | 100 iterations, same hash |
| 7 | `TestCanonicalizationDeterminism` | Canonicalization | `github_issue.json` + `github_issue_reordered.json` | Same hash despite different JSON key order |
| 8 | `TestPinnedGoldenHash` | Golden hash | `github_issue.json` | Exact SHA-256 match |
| 9 | `TestNormalizeMissingField` | Negative | Truncated JSON | Error returned |
| 10 | `TestNormalizeInvalidSource` | Negative | Unknown source type | Error returned |
| 11 | `TestNormalizeNegativeMissingFields` | Negative | `github_issue_missing_fields.json` | Error returned |

**Golden hash value:** `3a4d736728742d74a7341d7c534b043234c2d7076f00916aeb7f8e26cddfcc5e`

---

## 12. Receipts

```
$ go build ./...
(exit 0)

$ go vet ./...
(exit 0)

$ go test ./internal/normalize/... -v
=== RUN   TestNormalizeGitHubIssue
--- PASS: TestNormalizeGitHubIssue (0.00s)
=== RUN   TestNormalizeGitHubPR
--- PASS: TestNormalizeGitHubPR (0.00s)
=== RUN   TestNormalizeMaintainerComment
--- PASS: TestNormalizeMaintainerComment (0.00s)
=== RUN   TestNormalizeRelease
--- PASS: TestNormalizeRelease (0.00s)
=== RUN   TestNormalizeKEVEntry
--- PASS: TestNormalizeKEVEntry (0.00s)
=== RUN   TestHashDeterminism
--- PASS: TestHashDeterminism (0.01s)
=== RUN   TestCanonicalizationDeterminism
--- PASS: TestCanonicalizationDeterminism (0.00s)
=== RUN   TestPinnedGoldenHash
--- PASS: TestPinnedGoldenHash (0.00s)
=== RUN   TestNormalizeMissingField
--- PASS: TestNormalizeMissingField (0.00s)
=== RUN   TestNormalizeInvalidSource
--- PASS: TestNormalizeInvalidSource (0.00s)
=== RUN   TestNormalizeNegativeMissingFields
--- PASS: TestNormalizeNegativeMissingFields (0.00s)
PASS
ok  	github.com/PithomLabs/solvent/internal/normalize	0.010s

$ go test ./...
ok  	github.com/PithomLabs/solvent/internal/kernel	(cached)
ok  	github.com/PithomLabs/solvent/internal/normalize	0.010s
```

### Line Counts

| File | Before Rework | After Rework | Delta |
|---|---|---|---|
| `normalize.go` | 478 | 516 | +38 |
| `normalize_test.go` | 170 | 211 | +41 |
| `types.go` | 64 | 69 | +5 |
| **Total** | **712** | **796** | **+84** |

### Test Counts

| Metric | Before Rework | After Rework |
|---|---|---|
| Tests | 8 | 11 |
| Fixtures | 5 | 7 |
| Golden hashes | 0 | 1 |
