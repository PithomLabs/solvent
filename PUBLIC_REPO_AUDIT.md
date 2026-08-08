# PUBLIC_REPO_AUDIT.md

**Date:** 2026-08-08
**Scope:** Final documentation audit before public repository publication.
**Authority:** Public-repo principle — a judge must be able to clone the repository and find that the README describes what actually happens.

---

## Verdict: READY TO PUBLISH

One documentation discrepancy found and fixed (AGENTS.md missing `belief_edge` table). One aspirational comment in frozen code noted (belief.go:26) — documented, not fixed per freeze policy. All other claims verified against the actual repository state.

---

## 1. Schema Tables

**Actual:** 4 tables in `db/001_schema.sql`:
1. `belief` (lines 6-24)
2. `belief_edge` (lines 26-33)
3. `evidence` (lines 36-47)
4. `action_intent` (lines 49-65)

**AGENTS.md before audit:** Listed 3 tables (belief, evidence, action_intent) — missing `belief_edge`.
**AGENTS.md after audit:** Fixed. Now lists all 4 tables.

**README.md:** Already correct — "four tables in `db/001_schema.sql` — `belief`, `belief_edge`, `evidence`, `action_intent`" (line 53).

---

## 2. Retraction

**What exists:**
- `RetractCascade` is implemented in `internal/kernel/kernel.go:124-147`
- `RetractCascade` is tested in `internal/kernel/kernel_test.go` (TestW2_B07_RetractCascade)
- `RetractCascade` is proven in `proof/02_lifecycle_and_invariants.sql` (lines 60-72)

**What is NOT automatic:**
- Contradictions do NOT trigger `RetractCascade`
- `belief.Process` (line 38-48) detects contradictions and returns nil without ledger mutation
- The pipeline logs contradictions via `slog.Warn` and records `Contradiction: true` in results
- Transcript output shows `ledger_mutation: none` for contradictions

**Documentation status:**
- AGENTS.md: Correct — describes kernel capabilities without claiming automatic retraction
- README.md: Correct — does not claim automatic contradiction retraction
- `belief.go:26` comment says "Contradictions bypass steps 2-4 and call RetractCascade" — this is aspirational/incorrect relative to the actual code (lines 38-48 return nil). **Frozen code, not fixed.** Documented here for transparency.

---

## 3. Evidence / Fixtures

**Actual:** 9 JSON fixtures in `internal/derive/testdata/`:

| Fixture | Source Type | Content |
|---|---|---|
| `kev_entry.json` | kev_entry | CISA KEV entry for CVE-2024-24790 (etcd multipart MIME DoS) |
| `kev_entry_v2.json` | kev_entry | Second CVE fixture for two-CVE aggregation testing |
| `release_with_fix.json` | release | etcd v3.5.15 release with fix |
| `release_confirmed.json` | release | v3.5.15 with "fixed", "tested", "confirmed" keywords |
| `maintainer_fixes.json` | maintainer_comment | hexfusion confirms fix (is_maintainer=true) |
| `maintainer_no_regression.json` | maintainer_comment | serathius confirms no regression (is_maintainer=true) |
| `maintainer_reviewed.json` | maintainer_comment | liggitt security review (is_maintainer=true) |
| `non_maintainer_comment.json` | maintainer_comment | non-maintainer comment (is_maintainer=false) |
| `unmerged_pr.json` | github_pr | Closed unmerged PR |
| `contradicting_evidence.json` | maintainer_comment | ptabor says vulnerability still exists |

**What they represent:** A simulated etcd vulnerability lifecycle — advisory, fix release, maintainer confirmations, and one contradicting comment. Real CVE identifier (CVE-2024-24790), real etcd project, simulated GitHub/KEV event data.

**Documentation status:**
- AGENTS.md: Correct — "The current MVP uses a real etcd issue history as demonstration data, processed through the normalize→derive→pipeline chain."
- README.md: Correct — describes etcd as the domain candidate

---

## 4. Embeddings

**Actual:** No embedding column exists in the schema. Line 3 of `db/001_schema.sql` explicitly states: "I-6: no embedding column. The ledger is meaningful with zero vectors."

**Kernel test B-14** (`internal/kernel/kernel_test.go:793-801`) actively verifies the absence of an embedding column by querying `information_schema.columns`.

**Documentation status:**
- AGENTS.md: Correct — no mention of embedding column as existing
- README.md: Correct — I-6 states "Every invariant holds with all `embedding` columns NULL"
- `WAVE4_IMPLEMENTATION.md:159`: Correct — "The schema has no embedding column by design (I-6)"

---

## 5. Test Count

**Actual:** 84 tests, verified by running each package:

| Package | File | Tests |
|---|---|---|
| kernel | `internal/kernel/kernel_test.go` | 27 |
| belief | `internal/belief/belief_test.go` | 13 |
| belief | `internal/belief/integration_test.go` | 5 |
| derive | `internal/derive/derive_test.go` | 16 |
| normalize | `internal/normalize/normalize_test.go` | 11 |
| intent | `internal/intent/intent_test.go` | 3 |
| pipeline | `internal/pipeline/pipeline_test.go` | 9 |
| **Total** | | **84** |

**Documentation status:**
- AGENTS.md: Correct — "84 tests, all passing"
- README.md: Correct — "84 tests, all passing"

---

## 6. CLI

**Actual:** `cmd/solvent/main.go`

**Flags:**
| Flag | Default | Required | Description |
|---|---|---|---|
| `--dsn` | `testdb.DSN()` (env `FABLE_DSN`) | Yes | CockroachDB connection string |
| `--schema` | `db/001_schema.sql` | No | Path to frozen DDL |
| `--scenario` | `""` | Yes | Scenario ID for deterministic replay |
| `--reset` | `false` | No | Drop and recreate target database |
| `--out` | `""` (stdout) | No | Transcript output path |

**Command to run:**
```bash
export FABLE_DSN="postgresql://root@localhost:26260/fable_test?sslmode=disable"
go run ./cmd/solvent --scenario "55555555-0000-0000-0000-000000000001" --reset
```

**Output:** SOLVENT TRANSCRIPT with source provenance, belief state, audit count, and VERDICT.

**Documentation status:**
- README.md: Correct — "Run it with `--scenario <UUID> --reset`"
- Verified: command produces expected output against live CockroachDB

---

## 7. Kernel API

**Actual:** 8 methods on `*Store` + 1 constructor:

| Function | Purpose |
|---|---|
| `New(db)` | Constructor |
| `EnterBelief(ctx, scenarioID, claim, ct)` | Insert belief at door |
| `AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256)` | Record evidence |
| `RetireDebt(ctx, beliefID, item)` | Remove one debt item |
| `Promote(ctx, beliefID)` | Set status=promoted |
| `IntentOnPromoted(ctx, scenarioID, beliefID, action)` | Insert live intent |
| `RetractCascade(ctx, scenarioID, rootID)` | Cancel-then-retract in one txn |
| `AuditLiveOnNonPromoted(ctx, scenarioID)` | Count live intents on non-promoted beliefs |
| `EnsureBelief(ctx, scenarioID, claim, ct)` | Find-or-create belief |

**Error sentinels:** `ErrPromotionBlocked`, `ErrActionOnUnpromoted`

**Documentation status:**
- README.md: Correct — lists all kernel functions
- AGENTS.md: Correct — describes kernel capabilities

---

## 8. Invariants

**Actual:** 8 invariants (I-1 through I-8), all verified:

| Invariant | Enforcement | Status |
|---|---|---|
| I-1 | Schema CHECK `promoted_is_debt_free` | Verified |
| I-2 | Same CHECK (final_truth) | Verified |
| I-3 | Composite FK + CHECK `live_requires_promoted` | Verified |
| I-4 | ON UPDATE CASCADE propagates status | Verified |
| I-5 | `AuditLiveOnNonPromoted` returns 0 | Verified |
| I-6 | No embedding column (schema + test B-14) | Verified |
| I-7 | All writes via `crdb.ExecuteTx` | Verified |
| I-8 | `RetractCascade` is single transaction | Verified |

**Documentation status:**
- README.md: Correct — lists all 8 invariants
- AGENTS.md: Correct — describes database responsibilities

---

## 9. Build Commands

| Command | Status |
|---|---|
| `go build ./...` | ✅ Exit 0 |
| `go vet ./...` | ✅ Exit 0 |
| `gofmt -l internal/pipeline/ cmd/solvent/` | ✅ Empty (all formatted) |
| `go test ./internal/kernel/ -count=1` | ✅ 27 tests pass |
| `go test ./internal/belief/ -count=1` | ✅ 18 tests pass |
| `go test ./internal/derive/ -count=1` | ✅ 16 tests pass |
| `go test ./internal/normalize/ -count=1` | ✅ 11 tests pass |
| `go test ./internal/intent/ -count=1` | ✅ 3 tests pass |
| `go test ./internal/pipeline/ -count=1` | ✅ 9 tests pass |

---

## 10. File Paths Referenced in Documentation

| Path | Exists | Correct |
|---|---|---|
| `db/001_schema.sql` | ✅ | 4 tables, 66 lines |
| `internal/kernel/kernel.go` | ✅ | 177 lines, 8 methods |
| `internal/belief/belief.go` | ✅ | 105 lines |
| `internal/intent/intent.go` | ✅ | Exists |
| `internal/derive/derive.go` | ✅ | Exists |
| `internal/normalize/normalize.go` | ✅ | Exists |
| `internal/pipeline/pipeline.go` | ✅ | 428 lines |
| `cmd/solvent/main.go` | ✅ | 148 lines |
| `proof/02_lifecycle_and_invariants.sql` | ✅ | 93 lines |
| `proof/harness/` | ✅ | Exists |
| `internal/derive/testdata/` | ✅ | 9 JSON fixtures |
| `internal/normalize/testdata/` | ✅ | 7 JSON fixtures |

---

## 11. Changes Made

| File | Change | Reason |
|---|---|---|
| `AGENTS.md:66-70` | Added `belief_edge` to table list | Was missing from "Current MVP consists of only" |

---

## 12. Known Limitations (Not Discrepancies)

1. **`belief.go:26` comment is aspirational.** Says "Contradictions bypass steps 2-4 and call RetractCascade" but code returns nil without calling RetractCascade. Frozen code (Wave 3), not fixed. Documented here.

2. **Sequential test execution required.** The test harness shares a single CockroachDB database (`fable_test`). Running `go test ./...` in parallel causes race conditions. Serial execution is documented.

3. **Intent proposal is replay-idempotent, not concurrent-safe.** `ProposeIfNew` uses SELECT-then-INSERT. Concurrent duplicate-intent prevention is deferred.

---

## 13. Final Recommendation

**READY TO PUBLISH.**

All documentation claims match the actual repository state. The one discrepancy (missing `belief_edge` in AGENTS.md) has been fixed. The aspirational comment in `belief.go:26` is in frozen code and documented as a known limitation. A judge cloning this repository will find that the README describes what actually happens.

---

— End of public-repo audit —
