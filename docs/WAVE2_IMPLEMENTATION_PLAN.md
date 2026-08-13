# WAVE2_IMPLEMENTATION_PLAN.md

**Status:** Revised per Technical Lead review.
**Scope:** Wave 2 only — Belief Derivation engine.
**Authority:** Implements `DATA_PIPELINE_SPEC.md` §5 and §6.
**Frozen:** Wave 1 (`internal/normalize`) is frozen. Do not modify.

---

## 1. Scope

### In Scope

- `internal/derive` package
- Deterministic belief derivation from `NormalizedEvidence`
- Rule-based engine (no LLM, no external services)
- Belief classification: `derived`, `accommodated`
- `DerivedBelief` struct (pure semantic output)
- Contradiction detection
- Provenance mapping
- Supporting evidence collection
- Comprehensive unit tests
- Testdata fixtures

### Out of Scope

- Kernel operations (enter, retire debt, promote, retract) — Wave 3
- CockroachDB persistence — Wave 3
- Action intents — Wave 3
- Debt management — Wave 3
- ClaimID grouping — Wave 3
- Graph visualization / recursive traversal
- EvidenceFeed / polling
- Embeddings / vector search
- bchat integration / UI / demo / AWS

---

## 2. Files to Create

| File | Lines (est.) | Purpose |
|---|---|---|
| `internal/derive/types.go` | ~40 | DerivedBelief struct, constants |
| `internal/derive/derive.go` | ~250 | Rule engine, 4 rules |
| `internal/derive/derive_test.go` | ~250 | Unit tests (10 tests) |
| `internal/derive/testdata/kev_entry.json` | ~15 | Vulnerability advisory trigger |
| `internal/derive/testdata/release_with_fix.json` | ~15 | Fix release trigger |
| `internal/derive/testdata/release_confirmed.json` | ~15 | Fix with maintainer confirmation |
| `internal/derive/testdata/maintainer_fixes.json` | ~12 | Maintainer confirms fix |
| `internal/derive/testdata/maintainer_no_regression.json` | ~12 | Maintainer confirms no regression |
| `internal/derive/testdata/maintainer_reviewed.json` | ~12 | Security team review |
| `internal/derive/testdata/contradicting_evidence.json` | ~12 | Contradicting evidence trigger |

**Total:** ~10 files, ~540 lines

---

## 3. Package Layout

```
internal/derive/
├── types.go              DerivedBelief struct, constants
├── derive.go             Rule engine, Derive() function, 4 rules
├── derive_test.go        Unit tests (10 tests)
└── testdata/
    ├── kev_entry.json              Vulnerability advisory trigger
    ├── release_with_fix.json       Fix release trigger
    ├── release_confirmed.json      Fix with maintainer confirmation
    ├── maintainer_fixes.json       Maintainer confirms fix
    ├── maintainer_no_regression.json  Maintainer confirms no regression
    ├── maintainer_reviewed.json    Security team review
    └── contradicting_evidence.json Contradicting evidence trigger
```

---

## 4. Public API

### Classification Constants

```go
const (
    Derived      = "derived"
    Accommodated = "accommodated"
)
```

### DerivedBelief

```go
// DerivedBelief describes a belief derived from evidence.
// Pure semantic output — no kernel operations, no debt, no IDs.
// Wave 3 decides what to do with these results.
type DerivedBelief struct {
    Claim              string
    Classification     string
    SupportingEvidence []normalize.NormalizedEvidence
    Contradicts        []normalize.NormalizedEvidence
}
```

### Derive

```go
// Derive produces derived beliefs from normalized evidence.
// Pure function — no clock reads, no randomness, no external services.
// Same evidence always produces identical beliefs.
func Derive(evidence normalize.NormalizedEvidence) []DerivedBelief
```

---

## 5. Rule Engine Design

`Derive()` dispatches on `evidence.SourceType`:

| SourceType | Rules Evaluated |
|---|---|
| `kev_entry` | Rule 1: Vulnerability advisory |
| `release` | Rule 2: Fix release |
| `maintainer_comment` | Rule 3: Maintainer confirmation, Rule 4: Contradiction |
| `github_issue` | Rule 4: Contradiction (if keywords match) |
| `github_pr` | Rule 2: Fix release (if merged and CVE references) |

Each rule produces `DerivedBelief` values with structured claims and evidence.

---

## 6. Belief Derivation Rules

### Rule 1: Vulnerability Advisory → Enter Belief

**Generic mechanism:** Evidence matches a vulnerability advisory pattern.

**etcd example:** KEV entry for etcd CVE.

**Trigger:** `evidence.SourceType == "kev_entry"` and `evidence.Assertion` contains `"vulnerable to CVE-"`.

**Output:**
```go
DerivedBelief{
    Claim:          "etcd {version} is vulnerable to {cve_id}",
    Classification: Derived,
    SupportingEvidence: []NormalizedEvidence{evidence},
}
```

**Claim format:** Structured from `evidence.Subject` and `evidence.Assertion`:
- Subject: `"etcd v3.5.x, v3.5.x before v3.5.15"` → extract version
- Assertion: `"vulnerable to CVE-2024-24790"` → extract CVE ID

### Rule 2: Fix Release → Strengthen Belief

**Generic mechanism:** Evidence matches a fix release pattern.

**etcd example:** etcd release with fix version.

**Trigger:** `evidence.SourceType == "release"` and `evidence.Assertion` contains `"release"`.

**Output:**
```go
DerivedBelief{
    Claim:          "fix available for {product} {version}",
    Classification: Accommodated,
    SupportingEvidence: []NormalizedEvidence{evidence},
}
```

**Enhancement:** If the release's domain payload contains maintainer confirmation keywords, also note this in the claim.

### Rule 3: Maintainer Confirmation → Retire Debt

**Generic mechanism:** Evidence matches a maintainer confirmation pattern.

**etcd example:** Maintainer comment "Fixed in v3.5.14".

**Trigger:** `evidence.SourceType == "maintainer_comment"` and domain payload contains `keywords`.

**Keyword → classification mapping:**

| Keywords | Output |
|---|---|
| "fixed", "fix released", "patch available" | `Accommodated` with fix confirmation |
| "tested", "confirmed" | `Accommodated` with test confirmation |
| "no regression" | `Accommodated` with regression confirmation |
| "security review", "reviewed by" | `Accommodated` with security confirmation |

**Output:**
```go
DerivedBelief{
    Claim:          "fix for {product} confirmed by maintainer",
    Classification: Accommodated,
    SupportingEvidence: []NormalizedEvidence{evidence},
}
```

### Rule 4: Contradicting Evidence → Weaken or Retract

**Generic mechanism:** Evidence contradicts a prior belief.

**etcd example:** Comment "Actually reproduces again".

**Trigger:** Evidence keywords contain "reproduces", "regression", or "still vulnerable".

**Output:**
```go
DerivedBelief{
    Claim:          "prior belief about {product} is contradicted",
    Classification: Derived,
    SupportingEvidence: []NormalizedEvidence{},
    Contradicts:    []NormalizedEvidence{evidence},
}
```

**Note:** The derive package always emits contradiction results. The belief wiring layer (Wave 3) checks belief status — if `entered`, the contradiction is informational; if `promoted`, it triggers cascade.

---

## 7. Specification Traceability

| DerivedBelief Output | Spec Section | Wave 3 Kernel Function |
|---|---|---|
| New belief (Rule 1) | §5.1, §6.4 | `EnterBelief` |
| Strengthened belief (Rule 2) | §5.2 | `AddEvidence` |
| Confirmed belief (Rule 3) | §5.3 | `RetireDebt` |
| Contradicted belief (Rule 4) | §5.4, §6.2 | `RetractCascade` |

---

## 8. CockroachDB Assumptions

None. The derive package is pure Go — no database interaction.

---

## 9. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Rules are domain-specific (etcd CVEs) but mechanism must be domain-agnostic | Medium | Rules pattern-match on evidence fields, not hardcoded values |
| R2 | Contradicting evidence rule is keyword-based | Medium | Test with multiple contradicting patterns |
| R3 | Claim format may need presentation layer | Low | Structured facts, not formatted strings |

---

## 10. Receipts That Will Prove Correctness

| Receipt | Proves |
|---|---|
| `go build ./...` exits 0 | All packages compile |
| `go vet ./...` exits 0 | No vet errors |
| `go test ./internal/derive/... -v` exits 0 | Derivation is correct |
| `go test ./internal/normalize/... -v` exits 0 | Wave 1 still passes |
| `go test ./internal/kernel/... -v` exits 0 | Kernel still passes |
| `TestDerive_VulnerabilityAdvisory` | Rule 1: KEV entry produces derived belief with evidence |
| `TestDerive_FixRelease` | Rule 2: Release produces accommodated belief |
| `TestDerive_MaintainerConfirmation` | Rule 3: Comment produces accommodated belief |
| `TestDerive_Contradiction` | Rule 4: Contradicting evidence produces contradiction |
| `TestDerive_Determinism` | Same evidence → same beliefs (100 iterations) |
| `TestDerive_ClaimFormat` | Claim is structured, not hardcoded |

---

## 11. Implementation Order

| Step | Task | Est. Time |
|---|---|---|
| 1 | Create `types.go` — DerivedBelief struct, constants | 10 min |
| 2 | Create testdata fixtures (7 files) | 20 min |
| 3 | Create `derive.go` — rule engine, 4 rules | 1.5 hours |
| 4 | Create `derive_test.go` — 10 tests | 1 hour |
| 5 | Run verification: build, vet, test | 15 min |
| | **Total** | **~3 hours** |
