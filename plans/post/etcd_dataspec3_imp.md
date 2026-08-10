# etcd Real-Data Implementation Report

**Spec:** `plans/post/etcd_dataspec2.md`
**Date:** 2026-08-10
**Status:** Implemented + E2E verified against CockroachDB v26.2.0

---

## 1. Summary

Two new source types (`github_advisory`, `postmortem`) added to the normalize→derive→mapping chain. Five real fixture files created across two track directories. Pipeline Phase 3.5 (intent proposal) removed per spec. Standalone `cmd/operator-review` script created (Option B) with `--action` flag for intent creation.

## 2. Files Changed

| File | Lines | Change |
|---|---|---|
| `internal/normalize/types.go:33-34` | +2 | Added `SourceGitHubAdvisory`, `SourcePostmortem` constants |
| `internal/normalize/normalize.go:17-37` | +2 cases | Added switch cases for new source types |
| `internal/normalize/normalize.go:444-598` | +155 | Added `githubAdvisory` struct, `normalizeGitHubAdvisory`, `postmortemRecord` struct, `normalizePostmortem` |
| `internal/derive/derive.go:24-41` | +2 cases | Added switch cases for new source types |
| `internal/derive/derive.go:189-238` | +50 | Added `deriveFromGitHubAdvisory`, `deriveFromPostmortem` |
| `internal/belief/mapping.go:28-31` | +4 | Added `github_advisory` DebtMapping entry |
| `internal/pipeline/pipeline.go:53-59` | +6 | Added 5 fixture filenames to `sourceTypeMap` |
| `internal/pipeline/pipeline.go` | -9 | Removed Phase 3.5 (intent proposal loop) |
| `internal/pipeline/pipeline_test.go` | ~2 | Updated assertion: intentCount 1→0 (Phase 3.5 removed) |
| `cmd/operator-review/main.go` | +89 | New standalone operator review script |
| `internal/kernel/operator_review_test.go` | ~80 | New file: 4 tests (OR-1 through OR-4) |
| `internal/derive/testdata/etcd_real/track1/ghsa_advisory.json` | +20 | GHSA advisory fixture |
| `internal/derive/testdata/etcd_real/track1/release_v3527.json` | +12 | v3.5.27 release fixture |
| `internal/derive/testdata/etcd_real/track1/release_v3528.json` | +12 | v3.5.28 release fixture |
| `internal/derive/testdata/etcd_real/track2/release_v350.json` | +12 | v3.5.0 release fixture |
| `internal/derive/testdata/etcd_real/track2/postmortem_v35.json` | +14 | Postmortem fixture |
| `internal/derive/testdata/etcd_real/manifest.json` | +11 | SHA256 digests |
| `internal/derive/testdata/etcd_real/ATTRIBUTION.md` | +38 | Source licenses |

## 3. New Source Types

### github_advisory

- **Constant:** `normalize.SourceGitHubAdvisory = "github_advisory"`
- **Normalizer:** `normalizeGitHubAdvisory` — parses GHSA JSON, extracts affected package/range
- **Assertion:** `"vulnerable to <ghsa_id>"`
- **Claim:** `"<ecosystem>/<name> >=<introduced>, <<fixed> is vulnerable to <ghsa_id>"`
- **Classification:** `derived`
- **DebtMapping:** retires `needMap`

### postmortem

- **Constant:** `normalize.SourcePostmortem = "postmortem"`
- **Normalizer:** `normalizePostmortem` — parses postmortem JSON
- **Assertion:** `"<affected_versions> has documented data inconsistency"`
- **Claim:** `"<affected_versions> has documented data inconsistency"`
- **Classification:** `derived`
- **DebtMapping:** no debt retirement (invalidating evidence)

## 4. Fixture Layout

```
internal/derive/testdata/etcd_real/
  track1/
    ghsa_advisory.json       github_advisory   GHSA-q8m4-xhhv-38mg
    release_v3527.json       release           etcd v3.5.27
    release_v3528.json       release           etcd v3.5.28
  track2/
    release_v350.json        release           etcd v3.5.0
    postmortem_v35.json      postmortem        v3.5 data inconsistency
  manifest.json              SHA256 digests for all 5 fixtures
  ATTRIBUTION.md             source licenses (Apache 2.0, GHSA terms)
```

## 5. Pipeline Changes

### sourceTypeMap additions

5 new entries added to `internal/pipeline/pipeline.go:53-59`:
```go
"ghsa_advisory.json":   normalize.SourceGitHubAdvisory,
"release_v3527.json":   normalize.SourceRelease,
"release_v3528.json":   normalize.SourceRelease,
"release_v350.json":    normalize.SourceRelease,
"postmortem_v35.json":  normalize.SourcePostmortem,
```

### Phase 3.5 removed

The intent proposal loop (`ProposeIfNew` for promoted beliefs) was removed from `pipeline.Run()`. The pipeline now:
1. Phase 1: derive all fixture evidence (no DB writes)
2. Phase 2: aggregate evidence by claim
3. Phase 3: process aggregated beliefs through the ledger
4. Phase 4: record contradictions

Intent creation is now an **explicit demo action** via `cmd/operator-review`.

## 6. Operator Review Script

`cmd/operator-review/main.go` — Option B per spec Section 15.

**Flags:**
- `--dsn` — CockroachDB DSN (or `FABLE_DSN` env)
- `--scenario` — scenario UUID
- `--belief` — belief UUID to promote
- `--debt` — debt item to retire (repeatable)
- `--action` — optional; if set, creates a `live` intent on the promoted belief

**Behavior:**
1. Retires each specified debt via `kernel.RetireDebt`
2. Promotes the belief via `kernel.Promote`
3. If `--action` is set, creates a live intent via `kernel.IntentOnPromoted`
4. Audits via `kernel.AuditLiveOnNonPromoted`
5. Prints VERDICT: PASS/FAIL

## 7. Verification

### Static Checks

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l cmd internal` | PASS (empty) |
| `scripts/check_i7.sh` | PASS (7 ExecuteTx sites) |

### Track 1 E2E — Real Vulnerability Disclosure

| Step | Result |
|---|---|
| Pipeline processes 3 fixtures | PASS — 3 beliefs, 3 evidence |
| Vulnerability belief: 5 debts | PASS `{needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}` |
| v3.5.27 release: 4 debts | PASS `{needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}` |
| v3.5.28 release: 4 debts | PASS `{needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}` |
| Promotions refused (all entered) | PASS |
| Audit = 0 | PASS |
| Operator review: retire 5 debts | PASS |
| Promote vulnerability belief | PASS — status = `promoted`, debt = `{}` |
| Intent created | PASS — `deploy etcd v3.5.28`, state = `live` |
| Audit = 0 after intent | PASS |
| Replay: no duplicate beliefs/evidence/intents | PASS — same 3 beliefs, 3 evidence, 1 intent |

### Track 2 E2E — Historical Retraction

| Step | Result |
|---|---|
| Pipeline processes 2 fixtures | PASS — 2 beliefs (release + postmortem) |
| Enter baseline postulated belief | PASS — status = `entered`, 6 debts |
| Attach evidence | PASS |
| Retire 6 debts | PASS |
| Promote baseline belief | PASS — status = `promoted`, debt = `{}` |
| Create intent | PASS — `deploy etcd v3.5.0`, state = `live` |
| `--falsify` baseline belief | PASS — `retracted 1 belief(s)`, audit = 0 |
| Post-falsify: belief = `retracted` | PASS |
| Post-falsify: intent = `cancelled` | PASS |

### Structural Verification

| Check | Result |
|---|---|
| Schema unchanged (4 tables) | PASS |
| `kernel.go` unchanged | PASS (0 diff) |
| `001_schema.sql` unchanged | PASS (0 diff) |
| `belief_edge` rows = 0 | PASS |
| Manifest SHA256 digests match files | PASS |
| All source URLs present in evidence | PASS |
| Source type constants correct | PASS |
| DebtMapping `github_advisory` entry | PASS |
| `sourceTypeMap` has all 5 entries | PASS |

## 8. Spec Traceability

| Spec Section | Implementation |
|---|---|
| §3 Final Dataset | 5 fixtures in `track1/` and `track2/` |
| §4 Track 1 Design | 3 fixtures process through normalize→derive→belief.Process |
| §5 Track 2 Design | Postmortem fixture + kernel-entered baseline belief |
| §6 Fixture Layout | `etcd_real/track1/` and `etcd_real/track2/` subdirectories |
| §7 Normalization | `normalizeGitHubAdvisory`, `normalizePostmortem` |
| §8 Derivation | `deriveFromGitHubAdvisory`, `deriveFromPostmortem` |
| §9 Debt/Promotion | `github_advisory` entry in DebtMapping |
| §10 Code Changes | 8 files modified, 2 new files, 5 fixture files |
| §11 CLI/Demo | `--fixtures .../track1` or `.../track2` |
| §12 Acceptance | build/vet/fmt/i7 all pass; all tests pass including DB |
| §15 Operator Review | `cmd/operator-review/main.go` (Option B, with `--action`) |

## 9. Test Inventory

### Actual counts (verified via `go test -count=1 -v`)

| Package | `func Test` count | Notes |
|---|---|---|
| `internal/normalize` | 12 | Unchanged from HEAD |
| `internal/derive` | 16 | Unchanged from HEAD |
| `internal/kernel` | 31 | +4 new operator review tests |
| `internal/belief` | 19 (14+5 in 2 files) | Unchanged from HEAD |
| `internal/intent` | 3 | Unchanged from HEAD |
| `internal/pipeline` | 9 | Unchanged from HEAD |
| **Total** | **90** | **All pass** |

### Delta from HEAD

The previous verified Solvent state (AGENTS.md) reported **85 tests**. The actual HEAD count was **86** (AGENTS.md was slightly off — likely an approximation or excluded `TestMain` functions from some packages). The current count is **90** — a net increase of **+4**, all accounted for by the 4 new operator review tests in `internal/kernel/operator_review_test.go`.

No tests were deleted, renamed, merged, made unreachable, or accidentally omitted. The only existing test file modified was `internal/pipeline/pipeline_test.go` — a single assertion change (`intentCount != 1` → `intentCount != 0`) to reflect that Phase 3.5 was removed. No test function was added or removed from that file.
