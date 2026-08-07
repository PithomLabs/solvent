# DATA_PIPELINE_IMPLEMENTATION_PLAN.md

**Status:** Ready for review.
**Authority:** Implements `DATA_PIPELINE_SPEC.md` exactly as written.
**Architecture:** Frozen per `IMPLEMENTATION_CONTRACT.md`. No redesign.

---

## 1. Scope

### In Scope

- EvidenceFeed interface + etcd/KEV feed implementation
- Normalization pipeline (GitHub issues/PRs, maintainer comments, releases, KEV entries)
- Canonical evidence model (RawEvent → NormalizedEvidence → ledger row)
- Claim extraction engine (5 deterministic rules from spec §5)
- Belief lifecycle wiring (enter → retire debt → promote → retract)
- Intent lifecycle wiring (propose → cancel)
- Graph semantics (belief_edge creation, derives/contradicts edges)
- Embedding dedup (cosine distance via CockroachDB `<=>` operator, OpenAI text-embedding-3-small)
- Pipeline orchestrator (evidence → normalized → claims → belief actions)
- Audit outputs (AuditLiveOnNonPromoted wrapper)
- Schema migration (add embedding column)

### Out of Scope

- bchat UI, graph visualization, Taskfile, demo, video
- AWS, Bedrock, MCP, playground, README, Devpost
- Architecture changes to the frozen kernel
- Second evidence feed
- Agent sophistication (agents stay thin)
- Authentication/RBAC
- Multi-region deployment
- Stored confidence scores (confidence is derived, never stored)
- A fourth belief status

---

## 2. Dependency Graph

```
db/001_schema.sql (migration: add embedding)
    │
    ▼
internal/feed/          (EvidenceFeed interface)
    │
    ├─► internal/normalize/   (RawEvent → NormalizedEvidence)
    │       │
    │       ▼
    │   internal/embed/       (embedding generation + dedup query)
    │       │
    │       ▼
    │   internal/claim/       (NormalizedEvidence → belief actions)
    │       │
    │       ├─► internal/belief/  (wires claim rules → kernel ops)
    │       │
    │       └─► internal/intent/  (wires intent rules → kernel ops)
    │
    └─► internal/graph/       (belief_edge management)
            │
            ▼
        internal/pipeline/    (orchestrator: feed → normalize → claim → act)
```

### Wave Order

| Wave | Packages | Depends On |
|---|---|---|
| W0 | Schema migration (embedding column) | None |
| W1 | `internal/feed`, `internal/normalize` | W0 |
| W2 | `internal/embed` | W0 |
| W3 | `internal/claim` | W1 |
| W4 | `internal/belief`, `internal/intent`, `internal/graph` | W2, W3 |
| W5 | `internal/pipeline`, `internal/feed/etcd` | W4 |
| W6 | Integration tests + receipts | W5 |

---

## 3. Files to Modify

### `db/001_schema.sql`

Add `embedding` column to the `belief` table, after `final_truth` and before the CHECK constraint:

```sql
-- I-6: nullable; ledger fully functional with every embedding NULL.
-- 1536 dims = OpenAI text-embedding-3-small.
embedding   VECTOR(1536) NULL,
```

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

### Live cluster migration (`fable`)

```sql
ALTER TABLE belief SET (schema_locked = false);
ALTER TABLE belief ADD COLUMN embedding VECTOR(1536);
ALTER TABLE belief SET (schema_locked = true);
```

---

## 4. New Packages

### 4.1 `internal/feed/feed.go` (~40 lines)

The EvidenceFeed interface. Domain-agnostic. Every upstream source implements this.

```go
package feed

import "context"

// Event is the raw, unmodified data from an upstream source.
type Event struct {
    Source    string // "github", "cisa_kev", etc.
    EventType string // "issue.closed", "pr.merged", etc.
    Payload   []byte // unmodified upstream JSON
    FetchedAt time.Time
}

// EvidenceFeed produces raw events from an upstream source.
type EvidenceFeed interface {
    // Fetch returns all events since lastFetch. Idempotent.
    Fetch(ctx context.Context, lastFetch time.Time) ([]Event, error)
    // Name returns the feed identifier (e.g., "github_etcd", "cisa_kev").
    Name() string
}
```

### 4.2 `internal/feed/etcd/etcd.go` (~200 lines)

etcd/KEV feed implementation. Fetches from CISA KEV API and GitHub API.

Responsibilities:
- CISA KEV: `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json`
- GitHub: `https://api.github.com/repos/etcd-io/etcd/issues` and `/pulls`
- Rate limiting (GitHub: 5000 req/hr authenticated)
- Pagination (fetch all since timestamp)
- Returns `[]feed.Event` with raw JSON payloads

Configuration:
- `ETCD_GITHUB_TOKEN` — GitHub personal access token (optional, raises rate limit)
- `ETCD_FEED_INTERVAL` — polling interval (default: 5 minutes)

### 4.3 `internal/normalize/normalize.go` (~300 lines)

Transforms raw events into the canonical NormalizedEvidence model.

Functions:
- `Normalize(event feed.Event) (NormalizedEvidence, error)` — dispatcher
- `normalizeGitHubIssue(payload []byte) (NormalizedEvidence, error)`
- `normalizeGitHubPR(payload []byte) (NormalizedEvidence, error)`
- `normalizeMaintainerComment(payload []byte) (NormalizedEvidence, error)`
- `normalizeRelease(payload []byte) (NormalizedEvidence, error)`
- `normalizeKEVEntry(payload []byte) (NormalizedEvidence, error)`

Each normalizer:
1. Parses JSON
2. Extracts required fields (fails if missing)
3. Computes `content_sha256` on the normalized form (deterministic)
4. Maps to `NormalizedEvidence` fields

### 4.4 `internal/embed/embed.go` (~150 lines)

Embedding generation and deduplication.

Functions:
- `GenerateEmbedding(ctx context.Context, text string) ([]float32, error)` — calls OpenAI API
- `FindSimilar(ctx context.Context, db *sql.DB, scenarioID string, embedding []float32, threshold float64) (string, error)` — queries CockroachDB using `<=>` operator
- `Dedup(ctx context.Context, db *sql.DB, scenarioID string, claim string, embedding []float32) (existingBeliefID string, isDuplicate bool, err error)` — combines generation + lookup

CockroachDB query for dedup:
```sql
SELECT id FROM belief
WHERE scenario_id = $1::UUID
  AND embedding IS NOT NULL
  AND status <> 'retracted'
ORDER BY embedding <=> $2::VECTOR
LIMIT 1
```

If the result has cosine distance < threshold (0.15), it's a duplicate.

### 4.5 `internal/claim/claim.go` (~350 lines)

Deterministic claim extraction rules from spec §5.

Functions:
- `Extract(normalized NormalizedEvidence) []ClaimAction` — returns a list of actions to take

ClaimAction is a sum type:
```go
type ClaimAction struct {
    Type       string // "enter", "add_evidence", "retire_debt", "promote", "retract"
    BeliefID   string // existing belief ID (for add_evidence, retire_debt, promote, retract)
    Claim      string // claim text (for enter)
    ClaimType  string // derived, accommodated, postulated (for enter)
    DebtItem   string // debt item to retire (for retire_debt)
    Evidence   NormalizedEvidence // evidence to attach
}
```

Rules implemented:
1. **CVE in KEV** → `enter` + `add_evidence` + retire `needMap`
2. **Fix released** → `add_evidence` + retire `needMap` (or `needInvariant` if confirmed)
3. **Maintainer confirms** → `add_evidence` + retire appropriate debt items
4. **Contradicting evidence** → `add_evidence` + `retract` (if promoted)
5. **All debt retired** → `promote`

### 4.6 `internal/belief/belief.go` (~200 lines)

Wires claim actions to kernel operations.

Functions:
- `ProcessActions(ctx context.Context, store *kernel.Store, scenarioID string, actions []claim.ClaimAction) error`

Executes each ClaimAction by calling the appropriate kernel function:
- `EnterBelief` for "enter"
- `AddEvidence` for "add_evidence"
- `RetireDebt` for "retire_debt"
- `Promote` for "promote"
- `RetractCascade` for "retract"

Error handling:
- `ErrPromotionBlocked` → log and continue (debt not yet fully retired)
- `ErrActionOnUnpromoted` → log and continue (intent was racing)
- Other errors → return immediately (fail loudly)

### 4.7 `internal/intent/intent.go` (~100 lines)

Intent lifecycle wiring.

Functions:
- `ProposeIntent(ctx context.Context, store *kernel.Store, scenarioID, beliefID, action string) error`
- `CancelIntent(ctx context.Context, store *kernel.Store, scenarioID, beliefID string) error`

Thin wrappers around `kernel.IntentOnPromoted` and the cancel logic.

### 4.8 `internal/graph/graph.go` (~150 lines)

Belief edge management.

Functions:
- `AddEdge(ctx context.Context, db *sql.DB, parentID, childID, kind string) error`
- `GetDescendants(ctx context.Context, db *sql.DB, scenarioID, rootID string) ([]string, error)`
- `GetChildren(ctx context.Context, db *sql.DB, beliefID string) ([]string, error)`

Edge kinds: `derives`, `contradicts` (from schema CHECK).

### 4.9 `internal/pipeline/pipeline.go` (~400 lines)

The orchestrator. Wires everything together.

Functions:
- `Run(ctx context.Context, config Config) error` — one full pipeline cycle
- `RunOnce(ctx context.Context, config Config) error` — single evidence item

Config:
```go
type Config struct {
    DB          *sql.DB
    Feed        feed.EvidenceFeed
    ScenarioID  string
    EmbeddingFn func(ctx context.Context, text string) ([]float32, error) // injected
}
```

Pipeline stages:
1. `feed.Fetch()` → raw events
2. `normalize.Normalize()` → normalized evidence
3. `embed.Dedup()` → check for duplicates
4. `claim.Extract()` → claim actions
5. `belief.ProcessActions()` → execute against kernel
6. `graph.AddEdge()` → create dependency edges
7. `kernel.AuditLiveOnNonPromoted()` → verify invariant I-5

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

### 5.2 `internal/embed/embed_test.go`

| Test | Asserts |
|---|---|
| `TestDedupExactMatch` | Same claim → dedup detected, returns existing belief ID |
| `TestDedupDifferentClaims` | Different claims → no dedup |
| `TestDedupNullEmbedding` | NULL embeddings → falls back to content hash |
| `TestDedupThreshold` | Claims just above/below 0.15 threshold → correct classification |
| `TestGenerateEmbedding` | Returns 1536-dim vector (requires OPENAI_API_KEY) |

### 5.3 `internal/claim/claim_test.go`

| Test | Input | Asserts |
|---|---|---|
| `TestRule_CVEinKEV` | KEV entry for known CVE | enter + add_evidence + retire needMap |
| `TestRule_FixReleased` | Release with fix version | add_evidence + retire needMap |
| `TestRule_MaintainerConfirms` | Comment "Fixed in v3.5.14" | add_evidence + retire needMap + needInvariant |
| `TestRule_ContradictingEvidence` | Evidence contradicting promoted belief | retract |
| `TestRule_AllDebtRetired` | Evidence retiring last debt item | promote |
| `TestRule_AmbiguousEvidence` | Evidence that doesn't match any rule | no action (empty list) |

### 5.4 `internal/belief/belief_test.go`

| Test | Asserts |
|---|---|
| `TestBeliefLifecycle` | Full: enter → retire debt × 6 → promote → evidence → retract |
| `TestBeliefPromotionBlocked` | Debt not retired → ErrPromotionBlocked, belief stays entered |
| `TestBeliefRetractCascade` | Promoted belief with descendants → all retracted, intents cancelled |

### 5.5 `internal/intent/intent_test.go`

| Test | Asserts |
|---|---|
| `TestIntentLifecycle` | Propose on promoted → live; cancel → cancelled |
| `TestIntentOnUnpromoted` | Propose on entered → ErrActionOnUnpromoted |

### 5.6 `internal/graph/graph_test.go`

| Test | Asserts |
|---|---|
| `TestEdgeCreation` | Creates derives edge, queryable |
| `TestEdgeContradicts` | Creates contradicts edge |
| `TestGetDescendants` | 3-node chain → returns all descendants |
| `TestCyclicGraph` | 2-node cycle → terminates, returns 2 |

### 5.7 `internal/pipeline/pipeline_test.go`

| Test | Asserts |
|---|---|
| `TestPipelineEndToEnd` | Feed → normalize → claim → act → audit = 0 |
| `TestPipelineIdempotency` | Run twice → same belief count, same audit |
| `TestPipelineContradiction` | Contradicting evidence → retracted + cancelled |
| `TestPipelineDuplicateEvidence` | Same evidence twice → 1 belief, 2 evidence rows |

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
| C9 | OpenAI API returns 1536-dim vectors | OpenAI docs | Medium — API key required |
| C10 | `<=>` on VECTOR(1536) returns cosine distance | CockroachDB docs §Vector | Low |
| C11 | Brute-force vector scan acceptable at MVP scale | Acknowledged | Low — hundreds of beliefs |

---

## 7. Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | OpenAI API key not set | High | Pipeline returns clear error: "OPENAI_API_KEY not set" |
| R2 | OpenAI rate limits during bulk ingestion | Medium | Batch embeddings, retry with backoff |
| R3 | Claim extraction rules need tuning | Medium | Test each rule independently; threshold is a knob |
| R4 | Upstream API changes break normalization | Medium | Fixtures captured; normalization isolated |
| R5 | Pipeline conflicts with kernel tests | Low | Pipeline uses kernel API; kernel tests untouched |
| R6 | CockroachDB VECTOR is preview | Low | Ledger works with NULL embeddings |
| R7 | Captured fixtures miss edge cases | Medium | Fixtures + synthetic edge case tests |
| R8 | Embedding dedup threshold (0.15) is arbitrary | Medium | Configurable; start conservative |
| R9 | Pipeline produces beliefs that violate invariants | Low | Kernel enforces invariants; pipeline cannot bypass |
| R10 | Race between pipeline and manual kernel ops | Low | Scenario scoping isolates |

---

## 8. Receipts That Prove Correctness

| Receipt | Proves |
|---|---|
| `go build ./...` exits 0 | All packages compile |
| `go vet ./...` exits 0 | No vet errors |
| `go test ./internal/normalize/...` exits 0 | Normalization is correct |
| `go test ./internal/embed/...` exits 0 | Dedup works |
| `go test ./internal/claim/...` exits 0 | All 5 rules fire correctly |
| `go test ./internal/belief/...` exits 0 | Belief lifecycle works |
| `go test ./internal/intent/...` exits 0 | Intent lifecycle works |
| `go test ./internal/graph/...` exits 0 | Graph semantics work |
| `go test ./internal/pipeline/...` exits 0 | E2E pipeline works |
| `go test ./internal/kernel/...` exits 0 | Kernel still passes (B-14 updated) |
| `scripts/check_i7.sh` exits 0 | No raw writes outside ExecuteTx |
| E2E: feed → pipeline → audit = 0 | No corruption |
| Idempotency: run twice → same audit | Pipeline is idempotent |
| Dedup: same claim twice → 1 belief, 2 evidence | Dedup works |
| Contradiction: contradicting evidence → retracted + cancelled | Cascade fires |

---

## 9. Estimated Implementation Order

| Wave | Packages | Estimated Effort | Files |
|---|---|---|---|
| **W0** | Schema migration | 30 min | 2 modified |
| **W1** | `internal/feed`, `internal/normalize` | 2 hours | 3 new + 4 fixtures |
| **W2** | `internal/embed` | 1 hour | 1 new |
| **W3** | `internal/claim` | 3 hours | 1 new |
| **W4** | `internal/belief`, `internal/intent`, `internal/graph` | 3 hours | 3 new |
| **W5** | `internal/pipeline`, `internal/feed/etcd` | 3 hours | 2 new |
| **W6** | Integration tests + receipts | 2 hours | test updates |
| | **Total** | **~15 hours** | **~20 new files, 2 modified** |

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
