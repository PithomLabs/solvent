# NORMALIZE_IMPLEMENTATION.md

**Wave 1 Receipt.** Implements `internal/normalize` per `plan3.md` and `wave1_plan.md`.

---

## Files Created

| File | Lines | Purpose |
|---|---|---|
| `internal/normalize/types.go` | 64 | NormalizedEvidence struct, constants, maintainer list |
| `internal/normalize/normalize.go` | 478 | Normalization logic (5 normalizers + helpers) |
| `internal/normalize/normalize_test.go` | 170 | Unit tests (8 tests) |
| `internal/normalize/testdata/github_issue.json` | 30 | GitHub issue fixture |
| `internal/normalize/testdata/github_pr.json` | 22 | GitHub PR fixture |
| `internal/normalize/testdata/maintainer_comment.json` | 12 | Maintainer comment fixture |
| `internal/normalize/testdata/release.json` | 14 | Release metadata fixture |
| `internal/normalize/testdata/kev_entry.json` | 12 | CISA KEV entry fixture |

**Total:** 8 files, 712 lines of Go code

---

## Public Types

```go
type NormalizedEvidence struct {
    ID              string
    SourceURL       string
    SourceType      string
    ContentSHA256   string
    ObservedAt      time.Time
    IngestedAt      time.Time
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
```

Dispatches raw JSON to the appropriate normalizer. Returns error for unknown source type.

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
| `TestNormalizeMissingField` | PASS |
| `TestNormalizeInvalidSource` | PASS |

**8/8 tests pass.**

---

## Deviations from Spec

None. Implementation follows `DATA_PIPELINE_SPEC.md` §3 and §4 exactly.

---

## Known Limitations

1. **Maintainer list is placeholder.** `KnownMaintainers` contains 11 hardcoded GitHub usernames. Real implementation would load from config.

2. **Markdown stripping is basic.** `stripMarkdown` removes bold/italic markers and heading prefixes. Does not handle links, images, or code blocks beyond triple-backtick removal.

3. **Severity classification is keyword-based.** `classifySeverity` uses simple string matching. May misclassify unusual descriptions.

4. **Timestamp parsing has limited formats.** `parseTimestamp` supports RFC3339, ISO 8601, and a few common formats. Non-standard timestamps will fail.

5. **No deduplication at this layer.** Dedup via `content_sha256` is the responsibility of the caller (Wave 2/3).

---

## CockroachDB Assumptions

None. This package is pure Go — no database interaction.

---

## Receipts

```
go build ./...                         → exit 0
go vet ./...                           → exit 0
go test ./internal/normalize/... -v    → 8/8 PASS
go test ./...                          → all packages pass
```
