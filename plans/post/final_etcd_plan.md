# Final etcd Implementation Plan

**Source:** `FINAL_ETCD_IMPLEMENTATION_REVIEW.md` + `final_etcd_imp_review_prompt.md`
**Date:** 2026-08-10
**Scope:** Data correction + one small additive CLI path. No schema, kernel, debt model, belief-edge, or architecture changes.

---

## Fix 1: Correct all five fixture files

### F-1 — `track1/ghsa_advisory.json`

| field | current (wrong) | corrected |
|---|---|---|
| `summary` | `"Etcd: Denial of service vulnerability in etcd through multipart MIME parsing"` | `"etcd: Authorization bypasses in multiple APIs"` |
| `severity` | `"critical"` | `"high"` |
| `published_at` | `"2026-06-05T00:00:00Z"` | `"2026-03-20T20:48:14Z"` |
| `affected[0].package.ecosystem` | `"Go"` | `"go"` |
| `affected[0].package.name` | `"go.etcd.io/etcd"` | `"go.etcd.io/etcd/v3"` |

The `>=3.5.0, <3.5.28` range is a fair semver rendering of the real advisory — keep it.

### F-2 — `track2/postmortem_v35.json`

| field | current (wrong) | corrected |
|---|---|---|
| `source_url` | `…/postmortems/v35-data-inconsistency.md` (404) | `…/postmortems/v3.5-data-inconsistency.md` (200) |
| `published_at` | `"2022-12-06T00:00:00Z"` | `"2022-04-20T00:00:00Z"` |
| `affected_versions` | `"v3.5.x"` | `"v3.5.0–v3.5.2"` |
| `fixed_version` | `"v3.5.12"` | `"v3.5.3"` |
| `summary` | invented failure mode | `"Code refactor in v3.5.0 resulted in consistent index not being saved atomically. An independent crash could lead to committed transactions not being reflected on all members."` |

### F-3 — Three release fixtures

| fixture | `commit_sha` (current → corrected) | `released_at` (current → corrected) |
|---|---|---|
| `track2/release_v350.json` | `1a2b3c4d5e6f…` → `43b9aa5f9569412d60097cdd07eab7910fd282ab` | `2021-06-16T12:00:00Z` → `2021-06-15T22:29:48Z` |
| `track1/release_v3527.json` | `a1b2c3d4e5f6…` → `ee7ceab4e0ee88901cb46460579d1c22ee5ddb41` | `2026-03-15T12:00:00Z` → `2026-02-13T18:33:16Z` |
| `track1/release_v3528.json` | `f6e5d4c3b2a1…` → `8189e841fbff21101eb58ed97948ecc907690538` | `2026-06-10T12:00:00Z` → `2026-03-20T17:41:08Z` |

`commit_sha` feeds `content_sha256` via `normalize.go:339-345` — correcting it fixes the provenance anchor.

### Post-fix

- Recompute `manifest.json` SHA256 digests for all 5 files
- Fix `ATTRIBUTION.md` line 29: add missing dot (`v35` → `v3.5`)
- Fix `ATTRIBUTION.md` line 8: cite CC-BY-4.0 for GHSA data with link to `LICENSE.md`

---

## Fix 2: Add Track 2 entry path (F-4) + scenario/belief check (F-8)

### New flags on `cmd/operator-review/main.go`

```
--enter-claim <text>       enter a new belief (requires --claim-type)
--claim-type <type>        claim type for --enter-claim (postulated/derived/accommodated)
--evidence-url <url>       attach evidence to the belief (requires --evidence-sha)
--evidence-sha <sha>       content SHA256 for --evidence-url (64 hex chars, validated)
```

### Validation rules

- `--enter-claim` requires `--claim-type`; `--claim-type` requires `--enter-claim`
- `--evidence-url` requires `--evidence-sha` and vice versa; both require `--enter-claim`
- `--evidence-sha` must be exactly 64 hex characters (`^[0-9a-f]{64}$`)
- `--evidence-url` must be non-empty
- When `--enter-claim` is used, `--claim-type` must be `postulated` (the only Track 2 entry path)
- `--belief` is mutually exclusive with `--enter-claim` (two distinct modes)

### Behavior — two-command Track 2 sequence

**Command 1 — Enter baseline belief + attach evidence:**
```
operator-review --scenario <UUID> \
  --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" \
  --claim-type postulated \
  --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0" \
  --evidence-sha <64-hex-sha>
```
→ calls `kernel.EnterBelief`, prints `BELIEF_ID=<uuid>`
→ calls `kernel.AddEvidence`, prints `evidence: attached`
→ no debt retirement, no promotion, no intent

**Command 2 — Operator review on the entered belief:**
```
operator-review --scenario <UUID> --belief <BELIEF_ID> \
  --debt needMap --debt needInvariant --debt needToyCheck \
  --debt needNullModel --debt needObstruction --debt needFaithfulnessReview \
  --action "deploy etcd v3.5.0"
```
→ retires 6 debts, promotes, creates intent

This separation is conceptually cleaner: command 1 represents the historical decision being recorded; command 2 represents the human review/authorization. Easier for judges to follow.

### Scenario/belief consistency pre-check (F-8)

Before retiring debts, promoting, or creating intent (command 2 path):
```sql
SELECT scenario_id FROM belief WHERE id = $1
```
Exit non-zero if it differs from `--scenario`. Read-only pre-check; no kernel change.

This prevents a single-character typo from creating a cross-scenario intent that permanently blocks falsification.

---

## Fix 3: Fix false `intent: live` transcript (F-7)

### Change in `cmd/solvent/main.go:273-281`

Delete the `r.Promoted` fallback. The function becomes:

```go
func intentState(r pipeline.Result) string {
    return r.IntentState
}
```

When no intent was created, the field prints empty — which is correct.

### Update stale doc comments

- `internal/pipeline/pipeline.go:3`: remove reference to `ProposeIfNew`
- `cmd/solvent/main.go:5`: remove reference to `ProposeIfNew`

---

## Fix 4: Fix advisory severity mapping (F-6)

### Change in `internal/normalize/normalize.go:535`

Replace:
```go
Severity: classifySeverity(g.Summary, ""),
```

With a normalizing switch:
```go
Severity: normalizeAdvisorySeverity(g.Severity, g.Summary),
```

Where `normalizeAdvisorySeverity` maps the authoritative `g.Severity` field through existing constants:
```go
func normalizeAdvisorySeverity(raw, summary string) string {
    switch strings.ToLower(raw) {
    case "critical":
        return SeverityCritical
    case "high":
        return SeverityHigh
    case "medium":
        return SeverityMedium
    case "low":
        return SeverityLow
    case "info":
        return SeverityInfo
    default:
        return classifySeverity(summary, "")
    }
}
```

Falls back to `classifySeverity` only when the field is absent or unrecognized.

---

## Fix 5: Add tests for new data paths (F-5)

### `internal/normalize/normalize_test.go`

Table-driven tests for both new normalizers:
- `TestNormalizeGitHubAdvisory` — valid input, asserts correct subject/assertion/severity/source_url
- `TestNormalizeGitHubAdvisory_MissingRequiredFields` — each required field absent
- `TestNormalizePostmortem` — valid input, asserts correct subject/assertion/source_url
- `TestNormalizePostmortem_MissingRequiredFields` — each required field absent

### `internal/derive/derive_test.go`

- `TestDeriveFromGitHubAdvisory` — asserts exact claim string from real advisory data
- `TestDeriveFromPostmortem` — asserts exact claim string

### `internal/pipeline/pipeline_test.go`

- `TestEndToEnd_RealFixtureTracks` — runs `track1/` and `track2/` against a real DB, asserts:
  - Track 1: 3 beliefs, correct claim text, correct debt sets (5/4/4), scenario separation
  - Track 2: 2 beliefs, postmortem has 6 debts, no leakage into Track 1

### `internal/kernel/operator_review_test.go`

- `TestOperatorReview_ScenarioGuard_RejectsMismatch` — pass a belief from scenario A with `--scenario B`, assert:
  - non-zero exit
  - no debt mutation
  - no promotion
  - no intent created

---

## Execution order

1. Fixtures (F-1, F-2, F-3) + manifest + ATTRIBUTION — data-only, no code
2. Track 2 entry path + scenario/belief check (F-4, F-8) — `cmd/operator-review/main.go`
3. Transcript fix (F-7) — `cmd/solvent/main.go` + doc comments
4. Severity mapping (F-6) — `internal/normalize/normalize.go`
5. Tests (F-5) — four test files
6. Recompute manifest, run full suite, re-run E2E verification

---

## Verification after all fixes

1. `go build ./...`
2. `go vet ./...`
3. `gofmt -l cmd internal`
4. `scripts/check_i7.sh`
5. `go test -count=1 -p 1 ./...` — all pass
6. Track 1 E2E: `solvent --reset --fixtures …/track1` → verify 3 beliefs, debts 5/4/4, audit 0
7. Operator review: `operator-review --action "deploy etcd v3.5.28" --debt …` → promoted, intent live
8. Track 1 replay: re-run → no duplicates
9. Track 2 entry: `operator-review --enter-claim "…" --claim-type postulated --evidence-url "…" --evidence-sha "…"` → BELIEF_ID printed
10. Track 2 promote + intent: `operator-review --belief <BELIEF_ID> --action "deploy etcd v3.5.0" --debt …`
11. Track 2 falsify: `solvent --falsify … --scenario …0002` → retracted, cancelled, audit 0
12. Scenario guard: `operator-review --scenario A --belief <belief-from-B>` → non-zero exit, no mutations
13. Manifest SHA256 digests recomputed and matching
14. No `belief_edge` rows, schema unchanged, kernel unchanged
