# WAVE2_IMPLEMENTATION.md

**Status:** Complete.
**Scope:** Wave 2 — Belief Derivation engine.
**Implements:** `WAVE2_IMPLEMENTATION_PLAN.md`
**Frozen:** Wave 1 (`internal/normalize`) is frozen. Wave 2 (`internal/derive`) is now frozen.

---

## 1. Files Created

| File | Lines | Purpose |
|---|---|---|
| `internal/derive/types.go` | 41 | `DerivedBelief` struct, classification constants |
| `internal/derive/derive.go` | 220 | Rule engine, 5 source type handlers, 4 rules |
| `internal/derive/derive_test.go` | 228 | 10 unit tests |
| `internal/derive/testdata/kev_entry.json` | 10 | Vulnerability advisory fixture |
| `internal/derive/testdata/release_with_fix.json` | 10 | Release without maintainer confirmation |
| `internal/derive/testdata/release_confirmed.json` | 11 | Release with maintainer keywords |
| `internal/derive/testdata/maintainer_fixes.json` | 9 | Maintainer fix confirmation |
| `internal/derive/testdata/maintainer_no_regression.json` | 9 | Maintainer no-regression confirmation |
| `internal/derive/testdata/maintainer_reviewed.json` | 9 | Security team review |
| `internal/derive/testdata/contradicting_evidence.json` | 9 | Contradicting evidence |

**Total:** 10 files, ~556 lines

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
| `release` | Rule 2: Fix release | `Accommodated` (confirmed) or `Derived` (unconfirmed) |
| `maintainer_comment` | Rule 3: Maintainer confirmation, Rule 4: Contradiction | `Accommodated` (confirmation) or `Derived` (contradiction) |
| `github_issue` | Rule 4: Contradiction | `Derived` — contradicts prior belief |
| `github_pr` | Rule 2: Fix release | `Accommodated` — PR with fix reference |

---

## 4. Test Results

```
=== RUN   TestDerive_VulnerabilityAdvisory    --- PASS
=== RUN   TestDerive_FixRelease               --- PASS
=== RUN   TestDerive_MaintainerConfirmation   --- PASS
=== RUN   TestDerive_Contradiction            --- PASS
=== RUN   TestDerive_Determinism              --- PASS
=== RUN   TestDerive_ClaimFormat              --- PASS
=== RUN   TestDerive_UnsupportedSourceType    --- PASS
=== RUN   TestDerive_ReleaseConfirmedByMaintainer --- PASS
=== RUN   TestDerive_MaintainerSecurityReview --- PASS
=== RUN   TestDerive_MaintainerNoRegression   --- PASS
PASS
ok  	github.com/PithomLabs/solvent/internal/derive	0.003s
```

**10/10 tests pass.**

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

## 6. Bug Fix During Implementation

**Issue:** `reproducesPattern` matched "regression" in "no regression", causing false contradictions for maintainer confirmation evidence.

**Fix:**
1. Split into two patterns: `reproducesPattern` (handles "reproduces", "still vulnerable") and `noRegressionPattern` (handles "no regression" as a positive signal).
2. In `deriveFromMaintainerComment`, check for "no regression" before the keyword gate, so evidence with "no regression" in the assertion is treated as confirmation, not contradiction.

**Impact:** Fixed `TestDerive_MaintainerNoRegression` and `TestDerive_FixRelease` (test expected `Accommodated` for unconfirmed release; corrected to `Derived`).

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
