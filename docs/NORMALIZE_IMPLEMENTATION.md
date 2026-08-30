# NORMALIZE_IMPLEMENTATION.md

**Wave 1 Receipt.** Implements `internal/normalize` per `plan3.md` and `wave1_plan.md`.
Updated per `PLAN3_CODE_REVIEW.md` rework — see `NORMALIZE_REWORK.md` for change details.

---

## Files

| File | Lines | Purpose |
|---|---|---|
| `internal/normalize/types.go` | 69 | NormalizedEvidence struct, constants, immutabler maintainer registry |
| `internal/normalize/normalize.go` | 516 | Normalization logic (5 normalizers + helpers) |
| `internal/normalize/normalize_test.go` | 211 | Unit tests (11 tests) |
| `internal/normalize/testdata/github_issue.json` | 30 | GitHub issue fixture |
| `internal/normalize/testdata/github_pr.json` | 22 | GitHub PR fixture |
| `internal/normalize/testdata/maintainer_comment.json` | 12 | Maintainer comment fixture |
| `internal/normalize/testdata/release.json` | 14 | Release metadata fixture |
| `internal/normalize/testdata/kev_entry.json` | 12 | CISA KEV entry fixture |
| `internal/normalize/testdata/github_issue_missing_fields.json` | 5 | Negative fixture |
| `internal/normalize/testdata/github_issue_reordered.json` | 17 | Canonicalization fixture |

**Total:** 10 files, 796 lines of Go code

---

## Public Types

```go
type NormalizedEvidence struct {
    ID              string
    SourceURL       string
    SourceType      string
    ContentSHA256   string
    ObservedAt      time.Time
    IngestedAt      time.Time    // caller owns; zero at normalization
    ProvenanceClass string
    Subject         string
    Assertion       string
    Severity        string
    DomainPayload   []byte
}
```

---

## Public Functions

```go
func Normalize(raw []byte, sourceType string) (NormalizedEvidence, error)
func IsMaintainer(name string) bool
```

`Normalize` dispatches raw JSON to the appropriate normalizer. Returns error for unknown source type or missing required fields. Pure function — does not read the clock.

`IsMaintainer` reports whether a GitHub username is a recognized maintainer.

---

## Public Constants

```go
SourceGitHubIssue       = "github_issue"
SourceGitHubPR          = "github_pr"
SourceMaintainerComment = "maintainer_comment"
SourceRelease           = "release"
SourceKEVEntry          = "kev_entry"

SeverityCritical = "critical"
SeverityHigh     = "high"
SeverityMedium   = "medium"
SeverityLow      = "low"
SeverityInfo     = "info"

ProvenanceExternalFeed = "external_feed"
```

---

## Test Coverage

| Test | Status |
|---|---|
| `TestNormalizeGitHubIssue` | PASS |
| `TestNormalizeGitHubPR` | PASS |
| `TestNormalizeMaintainerComment` | PASS |
| `TestNormalizeRelease` | PASS |
| `TestNormalizeKEVEntry` | PASS |
| `TestHashDeterminism` | PASS (100 iterations) |
| `TestCanonicalizationDeterminism` | PASS |
| `TestPinnedGoldenHash` | PASS |
| `TestNormalizeMissingField` | PASS |
| `TestNormalizeInvalidSource` | PASS |
| `TestNormalizeNegativeMissingFields` | PASS |

**11/11 tests pass.**

---

## Deviations from Spec

None. Implementation follows `DATA_PIPELINE_SPEC.md` §3 and §4 exactly.

---

## Known Limitations

1. **Maintainer list is placeholder.** `knownMaintainers` contains 11 hardcoded GitHub usernames. Real implementation would load from config.

2. **Markdown stripping is basic.** `stripMarkdown` removes bold/italic markers and heading prefixes. Does not handle links, images, or code blocks beyond triple-backtick removal.

3. **Timestamp parsing has limited formats.** `parseTimestamp` supports RFC3339, ISO 8601, and a few common formats. Non-standard timestamps will fail.

4. **No deduplication at this layer.** Dedup via `content_sha256` is the responsibility of the caller (Wave 2/3).

---

## CockroachDB Assumptions

None. This package is pure Go — no database interaction.

---

## Receipts

```
go build ./...                         → exit 0
go vet ./...                           → exit 0
go test ./internal/normalize/... -v    → 11/11 PASS
go test ./...                          → all packages pass
```
