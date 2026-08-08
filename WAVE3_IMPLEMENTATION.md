# Wave 3 Implementation Receipts

## Summary

Wave 3 wires DerivedBelief outputs into the Transactional Belief Ledger.
All 14 new tests pass on CockroachDB v26.2.0. Build clean, vet clean, gofmt clean.

---

## New Files

### internal/belief/mapping.go

Compile-time evidence→debt mapping table. Domain-agnostic structure,
etcd-specific entries for the MVP.

- `DebtRule` struct: `Match *regexp.Regexp`, `Items []string`
- `DebtMapping`: map of source type → rules
- `DebtItemsForEvidence(sourceType, assertion)`: returns matched debt items

### internal/belief/belief.go

Thin translation layer. `Process()` translates `derive.DerivedBelief` into
kernel operations. No business logic, no orchestration.

Steps:
1. Validate claim non-empty
2. `EnsureBelief` → beliefID
3. `AddEvidence` for each supporting evidence (deduplicated by content_sha256)
4. `RetireDebt` for each debt item the evidence covers
5. `Promote` (may fail with ErrPromotionBlocked — acceptable)

Contradictions bypass steps 2–4 (known limitation for MVP).

Evidence deduplication: `evidenceExists()` checks if evidence with the same
`content_sha256` already exists for the belief before inserting. TOCTOU window
accepted for MVP per wave3_qa.md decision.

### internal/belief/belief_test.go

7 unit tests:

| Test | Purpose | Result |
|------|---------|--------|
| TestProcess_EnterNewBelief | New belief enters with 1 evidence row | PASS |
| TestProcess_ExistingBelief | Same claim deduplicates belief, adds 2nd evidence | PASS |
| TestProcess_Promotion | 6 evidence items retire all debt → promoted | PASS |
| TestProcess_PromotionBlocked | 1 evidence item leaves debt → stays entered | PASS |
| TestProcess_Contradiction | Contradiction does not enter belief | PASS |
| TestProcess_MalformedBelief | Empty claim rejected | PASS |
| TestProcess_Determinism | 100 replays → 1 belief, 1 evidence | PASS |

### internal/belief/integration_test.go

4 integration tests:

| Test | Purpose | Result |
|------|---------|--------|
| TestIntegration_ProcessToIntent | Full pipeline: derive → belief → promote → propose → audit | PASS |
| TestIntegration_PartialDebtBlocksPromotion | Incomplete debt blocks promotion, blocks intent | PASS |
| TestIntegration_MultiScenarioIsolation | Two scenarios independent | PASS |
| TestIntegration_DeterminismAcrossReplays | 100 replays → 1 belief, 1 evidence | PASS |

### internal/intent/intent.go

Thin wrappers around kernel operations:

- `Propose(ctx, db, scenarioID, beliefID, action)`: wraps `IntentOnPromoted`
- `Audit(ctx, db, scenarioID)`: wraps `AuditLiveOnNonPromoted`

### internal/intent/intent_test.go

3 unit tests:

| Test | Purpose | Result |
|------|---------|--------|
| TestPropose_OnPromotedBelief | Intent on promoted belief succeeds | PASS |
| TestPropose_OnUnpromotedBelief | Intent on unpromoted belief fails | PASS |
| TestAudit_NoLiveIntentsOnNonPromoted | Audit returns 0 on clean state | PASS |

---

## Kernel Changes

### internal/kernel/kernel.go

Added `EnsureBelief(ctx, scenarioID, claim, ct) (string, error)`:
- Wraps `crdb.ExecuteTx`
- Find-or-create in single transaction (no TOCTOU)
- Returns existing belief ID if claim already exists in scenario

### internal/kernel/sql.go

Added `sqlEnsureBelief` CTE:
```sql
WITH existing AS (
    SELECT id FROM belief
    WHERE scenario_id = $1::UUID AND claim = $2::STRING
    LIMIT 1
),
inserted AS (
    INSERT INTO belief (scenario_id, claim, claim_type)
    SELECT $1::UUID, $2::STRING, $3::STRING
    WHERE NOT EXISTS (SELECT 1 FROM existing)
    RETURNING id
)
SELECT id FROM existing
UNION ALL
SELECT id FROM inserted
```

Added to `SQLCatalog()`.

### internal/kernel/kernel_test.go

3 new tests:

| Test | Purpose | Result |
|------|---------|--------|
| W3-Ensure-New | EnsureBelief creates new belief | PASS |
| W3-Ensure-Existing | EnsureBelief returns same ID for duplicate | PASS |
| W3-Ensure-DiffScenario | Different scenarios → different beliefs | PASS |

---

## Test Results

```
ok  internal/kernel   1.837s  (28 tests)
ok  internal/belief   1.848s  (11 tests)
ok  internal/intent   1.113s  (3 tests)
ok  internal/derive   0.002s  (16 tests)
ok  internal/normalize 0.011s (11 tests)
```

Total: 69 tests, all PASS.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `gofmt -l internal/belief internal/intent internal/kernel` — clean

## Known Limitations

1. **Contradictions**: MVP logs contradictions but does not resolve the retraction
   target. The actual retraction target must be resolved by the caller.

2. **Evidence dedup TOCTOU**: `evidenceExists()` has a TOCTOU window between check
   and insert. Accepted for MVP per wave3_qa.md decision.

3. **Schema**: `embedding VECTOR(1536) NULL` column not yet added to `db/001_schema.sql`.
   B-14 test not updated. Deferred.
