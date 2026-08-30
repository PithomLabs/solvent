# NORMALIZE_REWORK.md

**Wave 1 Rework Receipt.** Applies Technical Lead approved changes from `PLAN3_CODE_REVIEW.md`.

---

## Files Modified

| File | Lines Before | Lines After | Changes |
|---|---|---|---|
| `internal/normalize/normalize.go` | 478 | 516 | B1, B2, Req1, S1, S3 |
| `internal/normalize/types.go` | 64 | 69 | S2 |
| `internal/normalize/normalize_test.go` | 170 | 211 | Req4, B1 assertion fix |

## Files Created

| File | Purpose |
|---|---|
| `internal/normalize/testdata/github_issue_missing_fields.json` | Negative fixture |
| `internal/normalize/testdata/github_issue_reordered.json` | Canonicalization fixture |

---

## Change Summary

### B1 (P0) — Remove `time.Now()` from `IngestedAt`

Removed `IngestedAt: time.Now().UTC()` from all 5 normalizers. The `NormalizedEvidence.IngestedAt` field remains in the struct but is left as zero value. The caller (Wave 2/3) owns ingestion time.

**Lines changed:** 5 locations in `normalize.go`

### B2 (P0) — Remove fabricated `ObservedAt`

- `normalizeGitHubPR`: When `merged=false`, `ObservedAt` is now zero (not `time.Now()`). When `merged=true` but `merged_at` empty, returns error.
- `normalizeKEVEntry`: When `dateAdded` empty, returns error (not `time.Now()`).

**Lines changed:** 2 fallback paths in `normalize.go`

### Req1 (P1) — Enforce all spec-required fields

Added validation for all fields marked required in `DATA_PIPELINE_SPEC.md` §3:

| Source | New Checks |
|---|---|
| `githubIssue` | `event_type`, `issue_number` (>0), `labels` (non-nil), `state` |
| `githubPR` | `event_type`, `pr_number` (>0), `state`, `base_branch` |
| `maintainerComment` | `comment_id` (>0), `issue_or_pr_number` (>0) |
| `kevEntry` | `versionAffected` |

### Req4 (P1) — Strengthen determinism testing

- `TestCanonicalizationDeterminism`: Same JSON with reordered fields → same hash
- `TestPinnedGoldenHash`: Asserts exact SHA-256 for github_issue.json fixture
- `TestNormalizeNegativeMissingFields`: Negative fixture with missing required fields
- `goldenGitHubIssueSHA256` constant pinned to `3a4d7367...`

### S1 (Should Fix) — Word-boundary severity matching

Replaced `strings.Contains` with `regexp.MatchString` using `\b` word boundaries:
- `\b(critical|rce|remote code execution)\b`
- `\b(high|exploit|actively exploited)\b`
- `\b(medium|moderate)\b`
- `\b(low|minor)\b`

Eliminates false positives like "follow" triggering "low" or "highly" triggering "high".

### S2 (Should Fix) — Immutable maintainer registry

Changed `KnownMaintainers` (exported mutable map) to `knownMaintainers` (unexported). Added `IsMaintainer(name string) bool` function as the public API.

### S3 (Should Fix) — Validate KEV URLs

Added `urlPattern` regex (`^https?://`) to validate `reference` field in KEV entries. Non-HTTP(S) URLs now return an error.

---

## Technical Lead Findings Resolved

| ID | Finding | Status |
|---|---|---|
| B1 | `IngestedAt` is `time.Now()` — violates purity contract | ✅ Resolved |
| B2 | Fabricated `ObservedAt` on fallback paths | ✅ Resolved |
| Req1 | Required-field validation inconsistent with spec §3 | ✅ Resolved |
| Req4 | No canonicalization test, negative fixture, or pinned hash | ✅ Resolved |
| S1 | Substring severity matching misclassifies ordinary text | ✅ Resolved |
| S2 | Mutable exported maintainer map | ✅ Resolved |
| S3 | KEV `reference` URL unvalidated | ✅ Resolved |

## Deferred (per Technical Lead)

| ID | Finding | Reason |
|---|---|---|
| Req2 | Include observation identity in `content_sha256` | Design decision — defer to Technical Lead after Wave 2 |
| Req3 | Store raw payload instead of normalized payload | Specification clarification needed first |

---

## Test Additions

| Test | Type | Asserts |
|---|---|---|
| `TestCanonicalizationDeterminism` | New | Reordered JSON → same hash |
| `TestPinnedGoldenHash` | New | Exact SHA-256 matches golden constant |
| `TestNormalizeNegativeMissingFields` | New | Missing required fields → error |

---

## Updated Receipts

```
go build ./...                              → exit 0
go vet ./...                                → exit 0
go test ./internal/normalize/... -v         → 11/11 PASS
go test ./...                               → all packages pass
```

**11 tests pass** (up from 8).

---

## Known Limitations (unchanged)

1. **Maintainer list is placeholder.** `knownMaintainers` contains 11 hardcoded GitHub usernames.
2. **Markdown stripping is basic.** Does not handle links, images, or code blocks beyond triple-backtick removal.
3. **Timestamp parsing has limited formats.** Non-standard timestamps will fail.
4. **No deduplication at this layer.** Dedup via `content_sha256` is the responsibility of the caller.
