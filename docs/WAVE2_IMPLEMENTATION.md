# WAVE2_IMPLEMENTATION.md

**Status:** Complete (reworked per adversarial review).
**Scope:** Wave 2 — Belief Derivation engine.
**Implements:** `WAVE2_IMPLEMENTATION_PLAN.md`
**Frozen:** Wave 1 (`internal/normalize`) is frozen. Wave 2 (`internal/derive`) is now frozen.

---

## 1. Files Created/Modified

| File | Lines | Purpose |
|---|---|---|
| `internal/derive/types.go` | 41 | `DerivedBelief` struct, classification constants |
| `internal/derive/derive.go` | 259 | Rule engine, 5 source type handlers, trust boundary gates |
| `internal/derive/derive_test.go` | 316 | 16 unit tests |
| `internal/derive/testdata/kev_entry.json` | 10 | Vulnerability advisory fixture |
| `internal/derive/testdata/release_with_fix.json` | 10 | Release fixture |
| `internal/derive/testdata/maintainer_fixes.json` | 10 | Maintainer fix confirmation (`is_maintainer: true`) |
| `internal/derive/testdata/maintainer_no_regression.json` | 10 | Maintainer no-regression confirmation |
| `internal/derive/testdata/maintainer_reviewed.json` | 10 | Security team review |
| `internal/derive/testdata/contradicting_evidence.json` | 10 | Contradicting evidence |
| `internal/derive/testdata/non_maintainer_comment.json` | 10 | Non-maintainer comment (F1 test) |
| `internal/derive/testdata/unmerged_pr.json` | 11 | Unmerged PR (F2 test) |

**Total:** 11 files, ~608 lines

---

## 2. Public API

```go
// Classification constants
const (
    Derived      = "derived"
    Accommodated = "accommodated"
)

// DerivedBelief — pure semantic output
type DerivedBelief struct {
    Claim              string
    Classification     string
    SupportingEvidence []normalize.NormalizedEvidence
    Contradicts        []normalize.NormalizedEvidence
}

// Derive — deterministic, pure function
func Derive(evidence normalize.NormalizedEvidence) []DerivedBelief
```

---

## 3. Rule Engine Summary

| SourceType | Rules | Output |
|---|---|---|
| `kev_entry` | Rule 1: Vulnerability advisory | `Derived` — new belief from KEV entry |
| `release` | Rule 2: Fix release | `Accommodated` (unconditional, per plan §5) |
| `maintainer_comment` | Rule 3: Maintainer confirmation, Rule 4: Contradiction | `Accommodated` (only if `is_maintainer: true`) or `Derived` (contradiction) |
| `github_issue` | Rule 4: Contradiction | `Derived` — contradicts prior belief |
| `github_pr` | Rule 2: Fix release | `Accommodated` (only if `merged: true`) |

---

## 4. Test Results

```
=== RUN   TestDerive_VulnerabilityAdvisory      --- PASS
=== RUN   TestDerive_FixRelease                 --- PASS
=== RUN   TestDerive_MaintainerConfirmation     --- PASS
=== RUN   TestDerive_Contradiction              --- PASS
=== RUN   TestDerive_Determinism                --- PASS
=== RUN   TestDerive_ClaimFormat                --- PASS
=== RUN   TestDerive_UnsupportedSourceType      --- PASS
=== RUN   TestDerive_MaintainerSecurityReview   --- PASS
=== RUN   TestDerive_MaintainerNoRegression     --- PASS
=== RUN   TestDerive_NonMaintainerComment       --- PASS
=== RUN   TestDerive_UnmergedPR                 --- PASS
=== RUN   TestDerive_MergedPR                   --- PASS
=== RUN   TestDerive_MalformedPayload           --- PASS
=== RUN   TestDerive_NilPayload                 --- PASS
=== RUN   TestDerive_ReleaseWithoutKeyword      --- PASS
=== RUN   TestDerive_NonMaintainerContradiction --- PASS
PASS
ok  	github.com/PithomLabs/solvent/internal/derive	0.004s
```

**16/16 tests pass.**

---

## 5. Verification Receipts

| Command | Exit Code | Proves |
|---|---|---|
| `go build ./...` | 0 | All packages compile |
| `go vet ./...` | 0 | No vet errors |
| `go test ./internal/derive/... -v` | 0 | Derivation is correct |
| `go test ./internal/normalize/... -v` | 0 | Wave 1 still passes |
| `go test ./internal/kernel/... -v` | 0 | Kernel still passes |

---

## 6. Rework: Adversarial Review Findings Resolved

| Finding | Severity | Resolution |
|---|---|---|
| F1: Non-maintainer fabricates "confirmed by maintainer" | P1 | `deriveFromMaintainerComment` now gates on `is_maintainer` from payload. Non-maintainer comments return `nil`. |
| F2: Unmerged PRs emit "fix available" | P1 | `deriveFromGitHubPR` now gates on `merged` from payload. Unmerged PRs return `nil`. |
| F3: Release "confirmed" path unreachable | P1 | Removed unreachable keyword branch. Releases unconditionally classified `Accommodated` per plan §5. |
| F4: Malformed payload fails silently | P2 | Added `extractBoolFromPayload` with explicit `(bool, bool)` return. Malformed/nil payloads → `nil` (explicit fail-on-invalid). |

---

## 7. What This Does NOT Do

- No database operations
- No kernel mutations (enter, retire, promote, retract)
- No debt management
- No action intents
- No clock reads
- No randomness
- No external services

All of the above are Wave 3 scope.

---

## 8. Next Step

**Wave 3:** Wire `DerivedBelief` outputs to kernel operations.
- `EnterBelief` for new vulnerability beliefs
- `AddEvidence` for strengthened beliefs
- `RetireDebt` for confirmed beliefs
- `RetractCascade` for contradicted beliefs
- Action intents for agent gating

Await Build Verification, Adversarial Review, and Technical Lead approval before Wave 3.
