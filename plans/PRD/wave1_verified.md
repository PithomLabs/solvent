# WAVE1_VERIFIED.md

**Status:** VERIFIED
**Scope:** Wave 1 only — `internal/normalize`, fixtures, and tests.
**Purpose:** Technical verification that the approved findings from `PLAN3_CODE_REVIEW.md`
were resolved in the Wave 1 rework (`wave1_rework.md`). Not an adversarial review, not a
new architecture review, not a redesign.

---

## 1. Verdict

```
VERIFIED
```

All seven approved findings are resolved in the current implementation. Deferred design
decisions (Req2, Req3) were not reopened.

---

## 2. Finding-by-Finding Verification

| Finding | Requirement | Verification Method | Result |
|---|---|---|---|
| **B1** | `Normalize()` is pure; no clock reads | `grep` for `time.Now()` in package = 0 matches. Independent probe: same input normalized twice → `IngestedAt` zero both times, `ObservedAt` identical, no wall-clock dependency. | ✅ |
| **B2** | No fabricated `ObservedAt` | Probe: unmerged PR → `ObservedAt` zero; merged PR missing `merged_at` → error; KEV missing `dateAdded` → error. Code inspection: fallback paths removed (`normalize.go:152-163`, `:382-388`). | ✅ |
| **Req1** | All spec-required fields validated | Probe rejected 8 malformed inputs: missing `event_type`, zero `issue_number`, nil `labels`, empty `state`, missing `base_branch`, zero `comment_id`, missing `versionAffected`, non-HTTP(S) KEV URL. All five normalizers enforce spec §3 required fields. | ✅ |
| **Req4** | Canonicalization tests, golden hash, negative fixtures | `TestCanonicalizationDeterminism` + `github_issue_reordered.json`; `TestPinnedGoldenHash` pinned to `3a4d736728742d74a7341d7c534b043234c2d7076f00916aeb7f8e26cddfcc5e`; `github_issue_missing_fields.json` + `TestNormalizeNegativeMissingFields`. 11/11 tests pass. | ✅ |
| **S1** | Word-boundary severity matching | `classifySeverity` uses `\b(critical|rce|remote code execution)\b`, `\b(high|exploit|actively exploited)\b`, `\b(medium|moderate)\b`, `\b(low|minor)\b`. Probe: "still has slow follow-ups" → not Low; "highly available" → not High; "low severity"/"critical severity" classify correctly. | ✅ |
| **S2** | Immutable maintainer registry | `KnownMaintainers` exported symbol removed (probe compile error proves removal); `IsMaintainer(name)` is the sole read-only accessor. | ✅ |
| **S3** | HTTP(S)-only KEV URLs | `urlPattern = ^https?://` enforced in `normalizeKEVEntry`; probe: `javascript:alert(1)` rejected with error. | ✅ |

---

## 3. Receipts (independently re-run)

```
go build ./...     → exit 0
go vet ./...       → exit 0
go test ./...      → all pass (kernel + normalize)
go test ./internal/normalize/... -v → 11/11 PASS
```

| Test | Result |
|---|---|
| TestNormalizeGitHubIssue | PASS |
| TestNormalizeGitHubPR | PASS |
| TestNormalizeMaintainerComment | PASS |
| TestNormalizeRelease | PASS |
| TestNormalizeKEVEntry | PASS |
| TestHashDeterminism (100 iterations) | PASS |
| TestCanonicalizationDeterminism | PASS |
| TestPinnedGoldenHash | PASS |
| TestNormalizeMissingField | PASS |
| TestNormalizeInvalidSource | PASS |
| TestNormalizeNegativeMissingFields | PASS |

---

## 4. Verification Method

- Read the reworked `normalize.go`, `types.go`, `normalize_test.go`, and all 7 fixtures.
- Grepped the package for residual clock reads and exported mutable symbols.
- Ran temporary in-package verification tests (removed after; the code left in the repo
  is untouched and re-verified green via `go build`/`go vet`/`go test ./...`).

## 5. Deferred (per tech-lead decision, not re-examined)

- Req2 — observation identity in `content_sha256` (design decision; Wave 2 must assume
  hash is semantic-content-based).
- Req3 — raw vs canonical `DomainPayload` (spec clarification; Wave 2 receives the
  canonicalized form).

---

Date: 2026-08-08
Reviewer role: Technical Verification Review