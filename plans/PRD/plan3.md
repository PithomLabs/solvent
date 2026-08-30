# DATA_PIPELINE_IMPLEMENTATION_PLAN.md (v3)

**Status:** Revised per `plan2_review.md`.
**Authority:** Implements `DATA_PIPELINE_SPEC.md` exactly as written.
**Architecture:** Frozen per `IMPLEMENTATION_CONTRACT.md`. No redesign.
**Scope Discipline:** Business logic first. Infrastructure second. Proof before plumbing.

---

## 1. Scope

### In Scope (Phase 1 Only)

- NormalizedEvidence model (domain-neutral transformation of raw input)
- Claim extraction engine (5 deterministic rules from spec §5)
- Belief lifecycle wiring (enter → retire debt → promote → retract)
- Intent lifecycle wiring (propose → cancel)
- Kernel Proof (proving the ledger works through the new pipeline)
- Captured JSON fixtures for test data
- Unit tests for each package

### Out of Scope (Deferred to Phase 2)

- EvidenceFeed interface + etcd/KEV feed implementation
- OpenAI embeddings (stub interface only)
- Graph package (`internal/graph`)
- Pipeline orchestrator (feed → normalize → derive → act)
- Schema migration (embedding column)
- Live GitHub API integration
- Live CISA KEV integration
- Authentication/RBAC
- Multi-region deployment
- bchat UI, graph visualization, Taskfile, demo, video
- AWS, Bedrock, MCP, playground, README, Devpost
- Architecture changes to the frozen kernel

---

## 2. Revised Dependency Graph

The review's core insight: **prove the core before building the edges.**

```
internal/normalize/
    (NormalizedEvidence — domain-neutral model)
        │
        ▼
internal/derive/
    (BeliefDerivation — deterministic belief derivation rules)
        │
        ▼
internal/belief/ + internal/intent/
    (Kernel Proof — prove the ledger works)
        │
        ▼
testdata/ (captured JSON fixtures)
    (Test data — deterministic, reproducible)
```

### Revised Wave Order

| Wave | Packages | Depends On | Purpose |
|---|---|---|---|
| **W1** | `internal/normalize` | None | Prove domain-neutral evidence model |
| **W2** | `internal/derive` | W1 | Prove belief derivation rules |
| **W3** | `internal/belief`, `internal/intent` | W2 | Kernel Proof |
| **W4** | Fixtures + unit tests | W3 | Prove determinism and correctness |

**Note:** Business logic (W1–W3) arrives before infrastructure (W4).

---

## 3. Files to Modify

### `internal/kernel/kernel_test.go`

Update `TestW3_B14_I6_NoEmbeddingColumn` (B-14) to assert the column exists and is nullable:

```go
// B-14 was: 0 embedding columns on belief (column did not exist under R2)
// B-14 is now: 1 embedding column, nullable, type = vector
var isNullable string
var udtName string
err := shared.QueryRowContext(ctx, `
    SELECT is_nullable, udt_name FROM information_schema.columns
    WHERE table_schema='public' AND table_name='belief' AND column_name='embedding'`).Scan(&isNullable, &udtName)

ok := err == nil && isNullable == "YES" && udtName == "vector"
```

**No schema migration.** The embedding column is nullable (I-6). The ledger works without it.

---

## 4. New Packages

### 4.1 `internal/normalize/normalize.go` (~300 lines)

Domain-neutral transformation of raw input. Every upstream source maps to this same structure.

**Canonical NormalizedEvidence model** (from spec §4.2):

```go
type NormalizedEvidence struct {
    ID               string    // content-addressed (SHA-256 of normalized form)
    SourceURL        string    // canonical URL to the original observation
    SourceType       string    // github_issue, github_pr, maintainer_comment, release, kev_entry
    ContentSHA256    string    // hash of normalized content (for deduplication)
    ObservedAt       time.Time // when observed upstream
    IngestedAt       time.Time // when ingested into Solvent
    ProvenanceClass  string    // external_feed (MVP only)
    Subject          string    // what this is about (e.g., "etcd v3.5.14")
    Assertion        string    // what this claims (e.g., "contains fix for CVE-2024-12345")
    Severity         string    // critical, high, medium, low, info
    DomainPayload    []byte    // JSON blob with source-specific fields
}
```

**Functions:**
- `Normalize(raw []byte, sourceType string) (NormalizedEvidence, error)` — dispatcher
- `normalizeGitHubIssue(raw []byte) (NormalizedEvidence, error)`
- `normalizeGitHubPR(raw []byte) (NormalizedEvidence, error)`
- `normalizeMaintainerComment(raw []byte) (NormalizedEvidence, error)`
- `normalizeRelease(raw []byte) (NormalizedEvidence, error)`
- `normalizeKEVEntry(raw []byte) (NormalizedEvidence, error)`

**Each normalizer:**
1. Parses JSON
2. Extracts required fields (fails if missing)
3. Computes `content_sha256` on the normalized form (deterministic)
4. Maps to `NormalizedEvidence` fields

**Normalization rules** (from spec §3):
- Timestamps → UTC, stored as `TIMESTAMPTZ`
- `repository` → `owner/name` format
- `labels` → sorted alphabetically (deterministic hashing)
- `body` → stripped of Markdown for belief derivation, raw preserved for evidence
- `version` → leading `v` stripped for comparison, stored with `v` prefix
- `content_sha256` → computed on normalized form, used for deduplication

---

### 4.2 `internal/derive/derive.go` (~350 lines)

Deterministic belief derivation rules from spec §5.

**DeriveAction type** (sum type):

```go
type DeriveAction struct {
    Type      string  // "enter", "add_evidence", "retire_debt", "promote", "retract"
    BeliefID  string  // existing belief ID (for add_evidence, retire_debt, promote, retract)
    Claim     string  // claim text (for enter)
    ClaimType string  // derived, accommodated, postulated (for enter)
    DebtItem  string  // debt item to retire (for retire_debt)
    Evidence  NormalizedEvidence // evidence to attach
}
```

**Rules implemented:**

> **Note:** The example version strings (e.g., "Fixed in v3.5.14") are illustrative fixture data, not hard-coded implementation assumptions. The implementation must be generic — pattern-matching on version strings, not comparing against specific values.

| # | Rule | Trigger | Action |
|---|---|---|---|
| 1 | CVE in KEV | CISA KEV entry for etcd version | `enter` + `add_evidence` + retire `needMap` |
| 2 | Fix released | Release metadata with fix version | `add_evidence` + retire `needMap` (or `needInvariant` if confirmed) |
| 3 | Maintainer confirms | Comment "Fixed in v3.5.14" | `add_evidence` + retire appropriate debt items |
| 4 | Contradicting evidence | Evidence contradicting promoted belief | `add_evidence` + `retract` |
| 5 | All debt retired | `RetireDebt` removes last item | `promote` |

**Debt items** (from spec §6.3):

| Item | Meaning | Retired By |
|---|---|---|
| `needMap` | Vuln-to-version mapping known | CVE advisory with affected/fixed versions |
| `needInvariant` | Behavioral invariant verified | Maintainer confirmation of fix behavior |
| `needToyCheck` | Minimal reproduction exists | Test case or reproducer confirmed |
| `needNullModel` | Baseline (no-fix) behavior characterized | Understanding of what happens without fix |
| `needObstruction` | Obstruction path identified | Root cause analysis confirmed |
| `needFaithfulnessReview` | Security review complete | Security team sign-off or maintainer assertion |

**Functions:**
- `Derive(normalized NormalizedEvidence) []DeriveAction` — returns list of actions to take

---

### 4.3 `internal/belief/belief.go` (~200 lines)

Wires belief derivation actions to kernel operations.

**Functions:**
- `ProcessActions(ctx context.Context, store *kernel.Store, scenarioID string, actions []derive.DeriveAction) error`

**Executes each DeriveAction by calling the appropriate kernel function:**
- `EnterBelief` for "enter"
- `AddEvidence` for "add_evidence"
- `RetireDebt` for "retire_debt"
- `Promote` for "promote"
- `RetractCascade` for "retract"

**Error handling:**
- `ErrPromotionBlocked` → log and continue (debt not yet fully retired)
- `ErrActionOnUnpromoted` → log and continue (intent was racing)
- Other errors → return immediately (fail loudly)

---

### 4.4 `internal/intent/intent.go` (~100 lines)

Intent lifecycle wiring.

**Functions:**
- `ProposeIntent(ctx context.Context, store *kernel.Store, scenarioID, beliefID, action string) error`
- `CancelIntent(ctx context.Context, store *kernel.Store, scenarioID, beliefID string) error`

Thin wrappers around `kernel.IntentOnPromoted` and the cancel logic.

---

## 5. New Tests

### 5.1 `internal/normalize/normalize_test.go`

Uses captured JSON fixtures in `testdata/`.

| Test | Fixture | Asserts |
|---|---|---|
| `TestNormalizeGitHubIssue` | `github_issue.json` | Correct fields, SHA-256 deterministic |
| `TestNormalizeGitHubPR` | `github_pr.json` | Correct fields, merged flag handled |
| `TestNormalizeMaintainerComment` | `maintainer_comment.json` | Author checked, keywords extracted |
| `TestNormalizeKEVEntry` | `kev_entry.json` | CVE ID valid, version parsed |
| `TestNormalizeRelease` | `release.json` | Version normalized, SHA matches |
| `TestHashDeterminism` | any | Same input → same SHA-256 across 100 runs |
| `TestNormalizeMissingField` | truncated JSON | Returns error, not silent null |

### 5.2 `internal/derive/derive_test.go`

| Test | Input | Asserts |
|---|---|---|
| `TestRule_CVEinKEV` | KEV entry for known CVE | enter + add_evidence + retire needMap |
| `TestRule_FixReleased` | Release with fix version | add_evidence + retire needMap |
| `TestRule_MaintainerConfirms` | Comment "Fixed in v3.5.14" | add_evidence + retire needMap + needInvariant |
| `TestRule_ContradictingEvidence` | Evidence contradicting promoted belief | retract |
| `TestRule_AllDebtRetired` | Evidence retiring last debt item | promote |
| `TestRule_AmbiguousEvidence` | Evidence that doesn't match any rule | no action (empty list) |

### 5.3 `internal/belief/belief_test.go`

| Test | Asserts |
|---|---|
| `TestBeliefLifecycle` | Full: enter → retire debt × 6 → promote → evidence → retract |
| `TestBeliefPromotionBlocked` | Debt not retired → ErrPromotionBlocked, belief stays entered |
| `TestBeliefRetractCascade` | Promoted belief with descendants → all retracted, intents cancelled |

### 5.4 `internal/intent/intent_test.go`

| Test | Asserts |
|---|---|
| `TestIntentLifecycle` | Propose on promoted → live; cancel → cancelled |
| `TestIntentOnUnpromoted` | Propose on entered → ErrActionOnUnpromoted |

---

## 6. CockroachDB Assumptions

| # | Assumption | Verified At | Risk |
|---|---|---|---|
| C1 | `VECTOR(1536)` type available on v26.2 | M0 (schema applied) | Low — GA in stable docs |
| C2 | `<=>` operator returns cosine distance 0..1 | CockroachDB docs §Vector | Low — documented |
| C3 | `schema_locked` must be cleared before ALTER | M0 report §3.3 | Low — documented |
| C4 | `crdb.ExecuteTx` retries serialization failures | M2 test B-18 | Retired |
| C5 | `ON UPDATE CASCADE` propagates status | M0 probe D3/D4 | Retired |
| C6 | No vector indexing on v26.2 (brute-force scan) | CockroachDB docs | Medium — O(n) acceptable at MVP scale |
| C7 | `pgx/v5` encodes Go `[]string` as `STRING[]` | M2 test B-17 | Retired |
| C8 | `content_sha256` is deterministic | Implementation guarantee | Medium — normalization must be deterministic |
| C9 | OpenAI API returns 1536-dim vectors | OpenAI docs | Deferred — not in Phase 1 |
| C10 | `<=>` on VECTOR(1536) returns cosine distance | CockroachDB docs §Vector | Deferred — not in Phase 1 |
| C11 | Brute-force vector scan acceptable at MVP scale | Acknowledged | Deferred — not in Phase 1 |

---

## 7. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Belief derivation rules need tuning | Medium | Test each rule independently; threshold is a knob |
| R2 | Upstream API changes break normalization | Medium | Fixtures captured; normalization isolated |
| R3 | Pipeline conflicts with kernel tests | Low | Pipeline uses kernel API; kernel tests untouched |
| R4 | CockroachDB VECTOR is preview | Low | Ledger works with NULL embeddings |
| R5 | Captured fixtures miss edge cases | Medium | Fixtures + synthetic edge case tests |
| R6 | Business logic assumptions wrong | Medium | W1–W3 prove correctness before W4 |
| R7 | Scope creep into infrastructure | High | Review gate after W4; no live APIs in Phase 1 |

---

## 8. Receipts That Prove Correctness

| Receipt | Proves |
|---|---|
| `go build ./...` exits 0 | All packages compile |
| `go vet ./...` exits 0 | No vet errors |
| `go test ./internal/normalize/...` exits 0 | Normalization is correct |
| `go test ./internal/derive/...` exits 0 | All 5 rules fire correctly |
| `go test ./internal/belief/...` exits 0 | Belief lifecycle works |
| `go test ./internal/intent/...` exits 0 | Intent lifecycle works |
| `go test ./internal/kernel/...` exits 0 | Kernel still passes (B-14 updated) |
| `scripts/check_i7.sh` exits 0 | No raw writes outside ExecuteTx |
| `TestHashDeterminism` passes 100 runs | Normalization is deterministic |
| E2E: fixture → normalize → derive → kernel → audit = 0 | No corruption |
| Idempotency: run fixture twice → same belief count | Pipeline is idempotent |
| Contradiction: contradicting fixture → retracted + cancelled | Cascade fires |

---

## 9. Estimated Implementation Order

| Wave | Packages | Estimated Effort | Files |
|---|---|---|---|
| **W1** | `internal/normalize` | 2 hours | 1 new + 5 fixtures |
| **W2** | `internal/derive` | 3 hours | 1 new |
| **W3** | `internal/belief`, `internal/intent` | 2 hours | 2 new |
| **W4** | Fixtures + unit tests | 1 hour | test updates |
| | **Total** | **~8 hours** | **~4 new files, 1 modified** |

---

## 10. Implementation Philosophy

From `prompt_imp.md`:

- Do not optimize.
- Do not generalize.
- Do not add abstractions.
- Do not add configuration.
- Do not introduce new interfaces unless explicitly required by the specification.
- Favor explicit code over clever code.
- Prefer readability over flexibility.
- Unknown behavior should fail loudly.

From `AGENTS.md`:

- Think like a distributed systems engineer, not an AI engineer.
- Every implementation decision: Architecture → Specification → Correctness → Simplicity.
- Never reverse this order.
- The implementation should be deterministic, testable, transactional, idempotent, explicit.
- Avoid hidden state, duplicated truth, speculative abstractions, premature optimization.

---

## 11. Decision Log

| ID | Decision | Rationale |
|---|---|---|
| D-071 | MVP implements smallest pipeline proving the ledger | Infrastructure that does not strengthen the core claim is deferred |
| D-072 | Business semantics precede transport semantics | Normalize → Derive → Kernel is the required order |
| D-073 | Live external integrations deferred to Phase 2 | Not necessary to validate the kernel |
| D-074 | OpenAI deferred; ledger works with NULL embeddings | I-6 proves this; embedding is optimization not requirement |
| D-075 | Graph package deferred; ledger is the product | Graph is projection, not the proof |
| D-076 | Fixtures replace live APIs for Phase 1 | Deterministic, reproducible, hackathon-appropriate |
