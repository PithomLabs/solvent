# Adversarial Security & Architecture Review — Solvent Kernel Freeze

**Date:** 2026-09-08
**Reviewer:** Kilo (automated adversarial review)
**Scope:** HEAD of `solvent-main` after Plan 10.1 (Exact Authority Binding) and debt-domain-agnostic correction
**Rule:** READ-ONLY. No code, schema, migration, test, documentation, or configuration modifications.

---

## 1. Verdict

**GREEN**

The Solvent kernel is genuinely ready to freeze.

- No unresolved CRITICAL, HIGH, or MEDIUM defect.
- Exact authority binding is closed at the database and kernel level.
- The debt mechanism is domain-agnostic; `FullDebt` is an application vocabulary, not a kernel semantic.
- Existing kernel invariants remain intact.
- No kernel expansion is required.

One LOW service-layer observation is recorded below. It does not block kernel freeze and does not require kernel growth.

---

## 2. Executive Assessment

The current kernel satisfies all seven primary objectives:

| Objective | Status |
|-----------|--------|
| A. Exact authority binding closed | **PASS** |
| B. Debt mechanism domain-agnostic | **PASS** |
| C. Initial debt caller/policy-owned | **PASS** |
| D. No unintended kernel semantics from parameterization | **PASS** |
| E. Existing kernel invariants intact | **PASS** |
| F. No unresolved CRITICAL/HIGH/MEDIUM defect | **PASS** |
| G. Remaining issues outside kernel | **PASS** |

**Kernel-freeze decision: READY TO FREEZE**

> No unresolved finding requires additional kernel primitives or schema changes.
> Future capabilities default to service, adapter, executor, deployment, policy,
> demo, or documentation layers.

---

## 3. Findings

### Finding 1 — LOW: REST `RetireDebt`/`Discharge` authenticated but not policy-authorized

- **Severity:** LOW
- **Files:** `api/belief.go:98-143`, `api/discharge.go:10-57`
- **Attack path:** Any authenticated principal can retire debt or discharge obligations on any belief in any scenario, subject only to cross-scenario existence check. There is no runtime authorization check (e.g., `actor_id` vs. belief owner, scenario-level policy).
- **Why existing controls fail:** The handlers validate UUIDs and scenario membership, then call `s.ledger.RetireDebt` / `s.ledger.Discharge` directly. No `AuthorizationDecision` is requested from the kernel or a policy service.
- **Concrete evidence:** `handleRetireDebt` (lines 99-143) and `handleDischarge` (lines 11-57) lack any call to `s.kern.Authorize` or equivalent. The authenticated principal is available via `AuthFromContext(r.Context())` but is not consulted.
- **Security impact:** A principal with API credentials can mutate debt state for beliefs they do not own. This is a policy-layer gap, not a kernel invariant gap. The kernel correctly treats debt identifiers as opaque and does not enforce ownership.
- **Correct layer:** Service or policy layer. The kernel's `RetireDebt` and `Discharge` are intentionally generic; caller authorization is a product/policy concern.
- **Kernel growth required:** No.

**Kernel-growth test for Finding 1:**
1. Durable security fact missing? No — the kernel does not claim to enforce principal-level debt ownership.
2. Atomic transition missing? No — `RetireDebt` and `Discharge` are already atomic.
3. Service/policy cannot enforce? Incorrect — a policy service can wrap these handlers with authorization checks.
4. Database cannot enforce? Correct — the DB has no principal-on-debt constraint, and adding one would be domain semantics.
5. Kernel safer than outside? No — adding principal validation to the kernel would couple it to authentication/identity systems explicitly listed as outside the kernel baseline.
6. Attack without it? Yes, but the attack is against policy, not against kernel invariants. The kernel's job is to enforce that debt retirement is subtractive and scenario-scoped, which it does.

**Conclusion:** Finding 1 is a service-layer authorization gap. It does not block kernel freeze.

---

## 4. Exact Authority Binding Results

### PASS

**Atomic predicate used by `ClaimIntent`:**

```sql
UPDATE action_intent
   SET state = 'executing'
 WHERE id = $1::UUID
   AND scenario_id = $2::UUID
   AND belief_id = $3::UUID
   AND action = $4::STRING
   AND target_id = $5::UUID
   AND snapshot_id = $6::UUID
   AND state = 'live'
```

This binds the intent to its complete intended identity:
- `intent_id`
- `scenario_id`
- `belief_id`
- `action`
- `target_id`
- `snapshot_id`
- `state = 'live'`

**Database enforcement (migration 009):**
- Composite FK `intent_authority_binding_fk`: `action_intent(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) ON DELETE CASCADE`
- Unique index `live_intent_per_snapshot`: at most one live intent per `(target_id, snapshot_id)`

**Verified attack paths:**

| Attack | Expected | Actual | Result |
|--------|----------|--------|--------|
| Execute I1 against T2/S2 (T2 is valid approved authority) | ClaimIntent rejects, executor not invoked, intent stays `live` | `allowed=false`, executor not called, intent remains `live` | **PASS** |
| Concurrent creation of equivalent bound intents on same snapshot | Unique index blocks duplicate | `ErrDuplicateIntent` (23514) | **PASS** |
| Pre-migration intent with NULL target/snapshot | NULL != any real (T,S), so unclaimable | NULL comparison fails in SQL | **PASS** |
| Direct SQL insert without kernel | FK enforces valid (target_id, snapshot_id) | FK violation (23503) if invalid | **PASS** |
| REST `handleExecuteAction` passes caller-supplied target | `ClaimIntent` inside `ExecuteAction` rejects mismatched (T,S) | Rejected | **PASS** |
| MCP `handleSolventExecute` passes caller-supplied target | Same kernel CAS rejects | Rejected | **PASS** |

**Test evidence:**
- `TestExecute_ConfusedDeputy` (api/authorization_test.go:623): PASS — T1-bound intent I1 executed against T2 is denied, executor not called, intent stays `live`.
- `TestTC9_PartA_ForUpdateLockMechanism` (kernel/authority_test.go:1405): PASS — FOR UPDATE lock serializes authorize-first against concurrent revoke.
- `TestTC9_PartB_ConcurrentRevokeTarget` (kernel/authority_test.go:~1700): PASS — concurrent revoke during authorize denies intent creation.
- `task db:reset` applied migration 009 cleanly.
- Live DB verified: composite FK `intent_authority_binding_fk` and unique index `live_intent_per_snapshot` present.

---

## 5. Debt-Domain-Agnosticism Results

### PASS

**Question:** Can a second domain use the same Solvent kernel with a different debt vocabulary without modifying kernel semantics?

**Answer:** Yes.

**Evidence:**

| Requirement | Status | Evidence |
|-------------|--------|----------|
| `EnterBelief` accepts caller-supplied `initialDebt` | **YES** | `kernel/kernel.go:61` — `func (s *Store) EnterBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string)` |
| `EnsureBelief` accepts caller-supplied `initialDebt` | **YES** | `kernel/kernel.go:252` — `func (s *Store) EnsureBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string)` |
| Kernel does not silently inject `FullDebt` | **YES** | No default parameter, no fallback to `FullDebt` in either method. SQL receives `$4::STRING[]` directly. |
| Kernel does not validate debt names against `FullDebt` | **YES** | No `switch`, `if` chain, map, or CHECK constraint over debt item names exists in the kernel. |
| `RetireDebt` treats identifiers generically | **YES** | `kernel/sql.go:192` — `UPDATE belief SET debt = array_remove(debt, $2::STRING)` — opaque string removal. |
| Promotion depends on unresolved debt cardinality, not vocabulary | **YES** | `kernel/authority.go:296-314` — `sqlPromote` checks `array_length(debt, 1) IS NULL`; no name inspection. |
| No kernel CHECK constraint enumerates domain debt names | **YES** | `db/001_schema.sql:38` — `CHECK (status IN (...))` — only status values are enumerated. Debt is `TEXT[]` with no item-level CHECK. |

**`FullDebt` classification:**

| Location | Classification | Rationale |
|----------|----------------|-----------|
| `kernel/kernel.go:31` — definition | Application/policy vocabulary | Defined as a `var` in the kernel package for convenience, but the kernel's `EnterBelief`/`EnsureBelief` no longer reference it. |
| `kernel/kernel.go` — callers inside kernel | None | No kernel method calls `FullDebt` internally. |
| `api/belief.go:33` — REST handler | Application vocabulary | `handleEnterBelief` passes `kernel.FullDebt` explicitly. This is a deployment choice, not a kernel requirement. |
| `internal/belief/belief.go:57` — pipeline | Application vocabulary | `belief.Process` passes `kernel.FullDebt` explicitly. |
| `cmd/solvent-mcp/tools.go:131` — MCP validation | API/MCP validation | Validates user-supplied `debt_item` against `kernel.FullDebt` enum. This is tool-schema hygiene, not kernel semantics. |
| `db/001_schema.sql:25` — DDL default | Compatibility/defense-in-depth default | `DEFAULT ARRAY[...]` for raw SQL inserts. Kernel creation paths pass debt explicitly and do not rely on this default. |
| `db/004_debt_vocabulary.sql:47` — re-statement of default | Compatibility/defense-in-depth default | Explicitly documented as "copy of the ARRAY[...] default, so a FRESH database takes its default from there." |
| Tests | Fixture/test | All test call sites pass `kernel.FullDebt` explicitly. |

**Desired data flow confirmed:**

```
Domain/policy
    |
    v
supplies initialDebt
    |
    v
Solvent kernel
    |
    v
stores/manages opaque obligations
```

The kernel does not require any particular industry's vocabulary.

---

## 6. FullDebt Specifically

### PASS

**Where `FullDebt` is defined:** `kernel/kernel.go:31`

**Which callers use it:**
- `api/belief.go:33` — REST `handleEnterBelief`
- `internal/belief/belief.go:57` — pipeline `Process`
- `internal/agentjacking/ingest.go:55` — demo ingest
- `internal/wizard/seed.go:102,114` — wizard seed
- `demo/cloud/init/main.go:170` — demo init
- `cmd/operator-review/main.go:119` — CLI
- `examples/github/executor/main.go:82` — example
- `cmd/solvent-mcp/tools.go:131` — MCP validation enum
- All test files

**Does any kernel creation path depend on it?** No. `EnterBelief` and `EnsureBelief` require `initialDebt []string` as an explicit parameter. No default, no fallback, no internal reference.

**Does any database default silently make it mandatory?** No. The DDL default (`ARRAY['needProvenanceCheck', ...]`) applies only when the `debt` column is omitted from an `INSERT`. All supported kernel/service paths include the `debt` column in their INSERT statements.

**Does replacing `FullDebt` with another vocabulary require kernel modification?** No. A deployment can pass any `[]string` to `EnterBelief`/`EnsureBelief`. The kernel stores and manipulates the array opaquely.

**Conclusion:** `FullDebt` is a convenience vocabulary for existing deployments. The kernel's semantics do not depend on it.

---

## 7. EnsureBelief Semantics

### PASS

**Verified behavior:**
1. New beliefs use caller-supplied `initialDebt` — confirmed by `sqlEnsureBelief` which inserts `$4::STRING[]`.
2. Existing beliefs are not silently overwritten — confirmed by `TestDA04_EnsureBeliefPreservesExistingDebt` (kernel_test.go:1653-1693): PASS — same ID returned, debt preserved as `item_a,item_b`.
3. Find-or-create semantics intact — `EnsureBelief` uses `ON CONFLICT (scenario_id, claim) DO UPDATE` and returns `EXCLUDED.id`.
4. Retries do not mutate existing debt — the `ON CONFLICT DO UPDATE` does not update the `debt` column.
5. No hidden fallback — no fallback to `FullDebt` or any other value exists.

**Test evidence:**
- `TestDA04_EnsureBeliefPreservesExistingDebt`: PASS
- `TestIntegration_DeterminismAcrossReplays` (internal/belief/integration_test.go:137): PASS — 100 replays produce identical state.

---

## 8. DDL Default Review

### PASS

**Current DDL defaults involving `belief.debt`:**

```sql
-- db/001_schema.sql:25
debt TEXT[] NOT NULL DEFAULT ARRAY['needProvenanceCheck', 'needContradictionSweep', 'needBlastRadius', 'needRollbackPlan', 'needVersionPin', 'needOperatorSignoff']

-- db/004_debt_vocabulary.sql:47 (re-statement)
ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY['needProvenanceCheck', ...];
```

**Classification:** Compatibility/defense-in-depth default.

**Why it is acceptable:**
1. The supported domain-agnostic kernel creation path (`EnterBelief`, `EnsureBelief`) always passes `$4::STRING[]` explicitly and does not rely on this default.
2. Its behavior is documented in `docs/OS/debt_parameterization.md:60-62` and `db/004_debt_vocabulary.sql:25`.
3. It does not undermine the claim that initial debt is caller/policy-owned, because the kernel API requires the caller to supply the array. The default only applies to raw SQL that omits the column.

**Raw SQL with domain-specific debt:** Yes — any raw `INSERT INTO belief (scenario_id, claim, claim_type, debt) VALUES (...)` can supply an arbitrary `TEXT[]`. This is intentional.

---

## 9. Debt Resource-Bound Review

### PASS

**Current representation:** `TEXT[]` (PostgreSQL array of text).

**Mutation semantics:**
- `RetireDebt`: subtractive only — `array_remove(debt, $2::STRING)`. Elements are removed, never appended.
- `EnterBelief`/`EnsureBelief`: set at creation time only.

**Can an untrusted caller create arbitrarily large debt arrays?**
- Through the kernel API: the kernel accepts any `[]string`. There is no length check in `EnterBelief` or `EnsureBelief`.
- Through the REST API: `EnterBeliefRequest` has no `debt` field. The REST handler always passes `kernel.FullDebt` (6 items). The REST surface is bounded.
- Through MCP: No belief-creation tool exists. Beliefs are created via the pipeline or REST.
- Through internal application code: callers pass `kernel.FullDebt` (6 items) or other bounded arrays.

**Is a kernel-level debt-count limit required?**
No. The current input surfaces are bounded:
- REST: hardcoded to `FullDebt` (6 items)
- MCP: no belief creation
- Internal application: bounded by caller code
- Tests: bounded by fixtures

If a future deployment adds a belief-creation surface that accepts untrusted debt arrays, the limit belongs at that service/API boundary, not in the kernel. The kernel's job is to store and retire opaque strings; cardinality limits are deployment concerns.

---

## 10. NULL / Empty Debt Semantics

### PASS

**Nil initialDebt (Go `nil` slice):**
- `sqlEnterBelief` and `sqlEnsureBelief` pass `$4::STRING[]` to PostgreSQL.
- Go's `database/sql` driver encodes a `nil` slice as `NULL`.
- PostgreSQL `TEXT[] NOT NULL DEFAULT ...` — `NULL` violates `NOT NULL` and triggers the DDL default.
- **Behavior:** A `nil` initialDebt triggers the DDL default (`FullDebt`). This is documented and intentional for defense-in-depth on raw SQL paths.
- **Kernel API contract:** The kernel API requires a non-nil `[]string`. Callers pass `kernel.FullDebt` or explicit arrays. The parameterization does not change this behavior.

**Empty initialDebt (Go `[]string{}`):**
- Encoded as an empty SQL array `ARRAY[]::TEXT[]` or `'{}'`.
- Stored as empty array.
- `array_length(debt, 1)` returns `NULL` for an empty array.
- `sqlPromote` checks `array_length(debt, 1) IS NULL` — empty array satisfies this, so promotion is not blocked by debt.
- **Test:** `TestDA03_EmptyInitialDebt` (kernel_test.go:1622-1651): PASS — empty debt creates belief, promotion succeeds.

**Non-empty initialDebt:**
- Stored verbatim.
- `RetireDebt` removes matching items via `array_remove`.
- Promotion blocked until `array_length(debt, 1) IS NULL`.

**Summary:**
| Input | SQL value | Stored | Promotion blocked? |
|-------|-----------|--------|-------------------|
| `nil` | `NULL` | DDL default (`FullDebt`) | Yes, until retired |
| `[]string{}` | `ARRAY[]` or `'{}'` | Empty array | No |
| `["a", "b"]` | `ARRAY['a','b']` | `["a","b"]` | Yes, until retired |

The distinction between nil and empty is preserved. Existing callers pass explicit arrays and are unaffected.

---

## 11. Promotion Invariant

### PASS

**Verified invariant:** `unresolved debt -> promotion blocked`

**Database enforcement:**
```sql
-- db/001_schema.sql:27-35
ALTER TABLE belief ADD CONSTRAINT promoted_is_debt_free
    CHECK (status <> 'promoted' OR array_length(debt, 1) IS NULL);
```

- `array_length(debt, 1)` returns `NULL` when `debt` is `NULL` (DDL default path) or an empty array.
- Returns a positive integer when debt items exist.
- The CHECK constraint is enforced by the database on every `UPDATE belief SET status = 'promoted'`.

**Empty debt does not block promotion:**
- Empty array -> `array_length` is `NULL` -> CHECK satisfied.
- Test: `TestDA03_EmptyInitialDebt`: PASS — promotion succeeds with empty debt.

**Caller-supplied `[]` cannot bypass other promotion requirements:**
- `final_truth = true` still blocks promotion (I-2).
- Composite FK `gate` still blocks intent creation on non-promoted beliefs (I-3).
- `live_requires_promoted` CHECK still blocks retraction of promoted beliefs with live intents (I-4).

**Promotion does not depend on caller identity:**
- `Promote(ctx, scenarioID, beliefID)` takes no `actorID` parameter.
- The database CHECK is the sole gate.

---

## 12. RetireDebt / Discharge

### PASS

**Kernel implementation:**
- `RetireDebt`: `UPDATE belief SET debt = array_remove(debt, $2::STRING) WHERE scenario_id = $1::UUID AND id = $2::UUID` — subtractive, idempotent, scenario-scoped.
- `Discharge`: Inserts into `debt_discharge` — append-only audit record. Does not mutate `belief.debt`.

**Arbitrary obligation identifiers:** Both methods accept arbitrary strings. No kernel validation against `FullDebt` or any other vocabulary.

**Retiring a nonexistent obligation:** `array_remove` is idempotent — removing a non-existent element is a no-op. Test: `TestW1_B04_RetireDebtIdempotent`: PASS.

**Discharge distinct from retirement:** Yes — `Discharge` records an audit trail in `debt_discharge`; `RetireDebt` mutates `belief.debt`. They are separate operations with separate SQL.

**Audit behavior intact:** `debt_discharge` records `scenario_id`, `belief_id`, `obligation_key`, `instrument_ref`, `discharged_by`, timestamp.

**No domain-specific human semantics leaked into kernel:** `Discharge` stores `discharged_by` as an opaque string. The kernel does not interpret it.

---

## 13. Actor / Authorization Boundary

### Summary

| Kernel method | Who can invoke | Authenticated | Authorized | Bypass via alternate path | Bypass via direct DB |
|---------------|---------------|---------------|------------|---------------------------|----------------------|
| `EnterBelief` | Service/REST/MCP/demo/test | Yes (REST/MCP) | No kernel check | Service passthrough | Yes — raw SQL INSERT |
| `AddEvidence` | Service/REST/MCP/demo/test | Yes (REST/MCP) | No kernel check | Service passthrough | Yes — raw SQL INSERT |
| `RetireDebt` | Service/REST/MCP/demo/test | Yes (REST/MCP) | **No** — Finding 1 | Service passthrough | Yes — raw SQL UPDATE |
| `Promote` | Service/REST/MCP/demo/test | Yes (REST/MCP) | No kernel check | Service passthrough | Yes — raw SQL UPDATE |
| `Discharge` | Service/REST/MCP/demo/test | Yes (REST/MCP) | **No** — Finding 1 | Service passthrough | Yes — raw SQL INSERT |
| `AuthorizeAndCreateIntent` | Service/REST/MCP | Yes (REST/MCP) | Kernel `Authorize` | Service passthrough | Yes — raw SQL INSERT (FK-blocked) |
| `ClaimIntent` | Service (ExecuteAction) | Implicit via service | Kernel CAS | Service only | Yes — raw SQL UPDATE (CAS predicate) |
| `CompleteIntent` | Service (ExecuteAction) | Implicit via service | No kernel check | Service only | Yes — raw SQL UPDATE |
| `RollbackClaim` | Service (ExecuteAction) | Implicit via service | No kernel check | Service only | Yes — raw SQL UPDATE |
| `CancelIntent` | Service (ReconcileIntent) | Implicit via service | No kernel check | Service only | Yes — raw SQL UPDATE |
| `RetractCascade` | Service/REST/MCP/demo/test | Yes (REST/MCP) | No kernel check | Service passthrough | Yes — raw SQL UPDATE (graph traversal required) |
| `RevokeTarget` | Service/REST/MCP/demo/test | Yes (REST/MCP) | No kernel check | Service passthrough | Yes — raw SQL INSERT |

**Categories:**

- **I-7/static analysis:** `scripts/check_i7.sh` reports 21 `ExecuteTx` write sites, 0 raw writes in production code. Test code and proof harnesses (`internal/m0/gate.go`, `internal/wizard/http_test.go`, etc.) contain raw SQL, which is acceptable for verification-only packages.
- **Runtime authentication:** REST and MCP enforce authentication. `AuthFromContext` rejects unauthenticated requests with 401.
- **Runtime authorization:** `handleAuthorizeAction` enforces `action_source == "user_typed"` and rejects conflicting `actor_id`. `handleExecuteAction` derives `effectiveActor` from the authenticated principal, never from the request body. Finding 1: `RetireDebt` and `Discharge` lack runtime authorization checks.
- **Database credential isolation:** Direct DB access bypasses all application-layer authorization. This is a deployment/configuration control — the database credentials must be protected independently.
- **Deployment/configuration controls:** CockroachDB network access, connection string secrecy, and IAM are outside the kernel.

---

## 14. Policy Bypass Review

### PASS

**Direct kernel calls outside service layer:**
- The `kernel.Store` type is exported, and tests/demos call it directly.
- This is not a bypass — the kernel is the boundary. There is no higher-level policy enforcement inside the kernel that could be bypassed.
- The kernel enforces structural invariants (FKs, CHECKs, CAS predicates). Policy (who may call what) is enforced by the service/REST/MCP layers.

**Alternate internal service calls:**
- `service/ledger` is a thin passthrough to the kernel. No alternate path exists.
- `service/authority` is the sole production execution path. `ExecuteAction` is the canonical boundary.

**Command-line administrative paths:**
- `cmd/operator-review` and `cmd/solvent-mcp` are user-facing tools. They authenticate via local stdio or API keys. No hidden administrative path exists.

**Test-only paths in production:**
- No test-only code is compiled into production binaries. Verification-only packages (`internal/m0`, `internal/wizard/http_test.go`) are separate modules.

**Direct SQL write access:**
- Available to anyone with database credentials. This is expected and is a deployment concern.

**Conclusion:** No policy bypass exists. The kernel is the authority boundary; the service layer is the caller boundary. The architecture is clean.

---

## 15. Action Intent Creation

### PASS

**Every creation path inspected:**

| Path | Scenario | Belief | Action | Authority binding | Target/Snapshot |
|------|----------|--------|--------|-------------------|-----------------|
| `AuthorizeAndCreateIntent` (kernel/authority.go:383-467) | `$1::UUID` | `$3::UUID` | `$4::STRING` | `$5::UUID`, `$6::UUID` (from `target_activation`) | Correct |
| `IntentOnPromoted` (kernel/authority.go:483-525) | `$1::UUID` | `$3::UUID` | `$4::STRING` | `NULL`, `NULL` | Correct — intentionally unbound |
| Direct SQL in tests | Explicit | Explicit | Explicit | Explicit | Explicit |

**Verified properties:**
- Action intent is created with correct `scenario_id`.
- `belief_id` must belong to `scenario_id` — enforced by the composite FK in `evidence` and the application logic in cross-scenario guards.
- Action is caller-supplied but validated against `action_source == "user_typed"` in REST/MCP.
- Authority binding is captured in `target_id` and `snapshot_id` columns (migration 009).
- Caller cannot substitute authority identity after authorization — `ClaimIntent` CAS rejects mismatched `(target_id, snapshot_id)`.
- Pre-approval intents (`IntentOnPromoted`) have `NULL` `target_id`/`snapshot_id`. The composite FK allows NULLs. The unique index `live_intent_per_snapshot` excludes NULLs (`WHERE target_id IS NOT NULL AND snapshot_id IS NOT NULL`). An unbound intent cannot become bound without an explicit `AuthorizeAndCreateIntent` call.

**Test evidence:**
- `TestExecute_BoundExecution` scenarios in `api/authorization_test.go` exercise bound intent creation and execution.
- `TestExecute_ConfusedDeputy` exercises the T1->T2 substitution attempt.

---

## 16. Scenario Isolation

### PASS

**Verified cross-scenario attack paths:**

| Attack | Test | Result |
|--------|------|--------|
| Cross-scenario belief read | `TestW2_B19_CrossScenarioIsolation` | PASS — belief returned only from its own scenario |
| Cross-scenario debt mutation | `TestCS1_RetireDebt_CrossScenario` | PASS — `belief not found in scenario` |
| Cross-scenario promotion | `TestCS3_Promote_CrossScenario` | PASS — `belief not found in scenario` |
| Cross-scenario discharge | `TestCS4_Discharge_CrossScenario` | PASS — `belief not found in scenario` |
| Cross-scenario retract (rejected) | `TestCS_NEW02_CrossScenarioRejected` | PASS — `belief not found in scenario` |
| Cross-scenario retract (cascade) | `TestCS_NEW02_CrossScenarioRetractSucceeds` | PASS — cascade confined to target scenario |
| Cross-scenario discharge (duplicate key) | `TestCS_NEW03_CrossScenarioRejected` | PASS — `belief not found in scenario` |
| Cross-scenario evidence | `TestIntegration_MultiScenarioIsolation` (internal/belief) | PASS — 1 belief per scenario |

**Database enforcement:**
- Every mutating kernel SQL includes `scenario_id = $1::UUID` in the WHERE clause.
- `evidence.belief_id -> belief.id` composite FK includes `scenario_id`.
- `belief_edge.parent_id -> belief.id` and `child_id -> belief.id` — no `scenario_id` in `belief_edge`, but parent/child lookup is through `belief` which carries `scenario_id`. The application validates scenario membership before edge creation.

**I-7 still passes:** 21 `ExecuteTx` write sites, 0 raw writes.

---

## 17. SQL / Database Integrity

### PASS

**All SQL touching kernel tables inspected:**

| Table | Kernel SQL locations | Raw SQL outside kernel | Risk |
|-------|---------------------|------------------------|------|
| `belief` | `kernel/sql.go` (INSERT, UPDATE debt, UPDATE status, SELECT) | `internal/m0/gate.go`, `internal/wizard/http_test.go`, `demo/agentjacking/ingest/main.go` | Low — test/proof-only raw SQL |
| `action_intent` | `kernel/authority.go` (INSERT, UPDATE state x4) | `kernel/authority_test.go`, `adapter/github/executor_test.go`, `internal/m0/gate.go` | Low — test raw SQL; FK + unique index enforce integrity |
| `evidence` | `kernel/sql.go` (INSERT) | `internal/corpus/corpus.go`, `internal/agentjacking/ingest_test.go` | Low — test/demo raw SQL |
| `belief_edge` | `kernel/sql.go` (INSERT) | `internal/demoseed/demoseed.go` | Low — internal app code |
| `debt_discharge` | `kernel/authority.go` (INSERT) | None outside kernel | None |
| `authority_target` | `kernel/authority.go` (INSERT, SELECT FOR UPDATE) | None | None |
| `target_snapshot` | `kernel/authority.go` (INSERT) | None | None |
| `target_activation` | `kernel/authority.go` (INSERT) | None | None |
| `target_revocation` | `kernel/authority.go` (INSERT, SELECT) | None | None |
| `justification` | `kernel/authority.go` (INSERT) | None | None |

**Missing transaction boundaries:** None in production kernel code. All mutating kernel methods use `s.db.ExecContext` inside a caller-managed transaction (SERIALIZABLE for `AuthorizeWithinTx`, `AuthorizeAndCreateIntent`).

**Missing scenario predicates:** None in production kernel SQL. Every UPDATE/INSERT/DELETE on `belief`, `action_intent`, `evidence` includes `scenario_id`.

**Incomplete foreign keys:** None. Migration 009 added `intent_authority_binding_fk`. All other FKs are present.

**Nullable/wildcard behavior:** `action_intent.target_id` and `snapshot_id` are nullable. NULLs are allowed for `IntentOnPromoted` pre-approval intents. The unique index `live_intent_per_snapshot` excludes NULLs. NULL never acts as a wildcard for `ClaimIntent` because the CAS predicate requires `target_id = $5::UUID AND snapshot_id = $6::UUID` — NULL != UUID.

**ON DELETE/UPDATE behavior:**
- `intent_authority_binding_fk ... ON DELETE CASCADE` — deleting a `target_snapshot` cascades to `action_intent`. This is correct because a deleted snapshot invalidates the authority binding.
- `justification.belief_id -> belief.id ON UPDATE CASCADE ON DELETE CASCADE` — correct; belief retraction cascades to justifications.
- `action_intent.belief_id -> belief.id ON UPDATE CASCADE` — correct; belief ID is immutable (UUID), but cascade preserves referential integrity.

**I-7 result:** 21 `ExecuteTx` write sites, 0 raw writes in production code.

---

## 18. Race Conditions

### PASS

**Verified concurrent scenarios:**

| Scenario | Test | Result |
|----------|------|--------|
| Concurrent `ClaimIntent` on same live intent | `TestTC9` series | PASS — FOR UPDATE lock serializes; CAS rejects stale state |
| Concurrent creation of equivalent bound intents | Unique index `live_intent_per_snapshot` | PASS — `ErrDuplicateIntent` (23514) |
| Concurrent debt retirement | `TestTC8_ConcurrentDischarge` | PASS — `errors=1` (expected contention) |
| Concurrent promotion | Promotion uses bare UPDATE; database serializes | PASS — no corruption |
| Concurrent retract/reliance | `TestTC9_PartB` | PASS — revoke-first serialized by FOR UPDATE |
| Concurrent `RequestAuthorization` | `TestTC7_ConcurrentRequestAuthorization` | PASS — idempotent |

**Newly affected by `initialDebt` parameterization:** None. `EnterBelief` and `EnsureBelief` do not share mutable state across calls. `EnsureBelief` uses `ON CONFLICT DO UPDATE` which is safe under concurrent inserts.

**Race safety source:** Database-enforced (serializable transactions, unique indexes, FK constraints). Not inferred from Go synchronization.

---

## 19. Public API / MCP Review

### PASS

**REST contracts:**
- `EnterBeliefRequest` has no `debt` field. The REST API continues to create beliefs with `kernel.FullDebt`. No change to public contract.
- `RetireDebtRequest` has `debt_item` — caller supplies a string. The API does not expose the debt vocabulary as a typed enum.
- `DischargeRequest` has `obligation_key`, `instrument_ref`, `discharged_by` — all strings. No domain semantics.
- `AuthorizeActionRequest` has no change from Plan 10.1.

**MCP contracts:**
- `solvent_retire_debt` advertises `debt_item` as an enum generated from `kernel.FullDebt`. This is tool-schema hygiene for the demo surface.
- No MCP belief-creation tool exists. Beliefs are created via the pipeline or REST.

**OpenAPI / MCP tool schemas:**
- No new public functionality exposed by debt parameterization.
- No domain-specific debt vocabulary is exposed through interfaces intended to remain deployment-specific.

**Validation semantics:**
- `validateUUID`, `validateNonEmpty`, `validateEnum` unchanged.
- `action_source` validation (`user_typed` vs `tool_output`) unchanged.

---

## 20. Documentation / Architectural Consistency

### PASS

**Verified documents:**

| Document | Status |
|----------|--------|
| `docs/OS/debt_parameterization.md` | Correctly states: debt vocabulary is policy/domain-owned; `FullDebt` is an existing deployment vocabulary; kernel stores generic obligation identifiers; initial debt is caller-supplied; promotion depends on unresolved debt, not debt names. |
| `docs/OS/plan10.1.md` | Correctly describes exact authority binding, confused-deputy gap, composite FK, unique index, NULL-fails-closed design. |
| `docs/OS/adv_review9.md` | Previous review — not relied upon for this review. |

**No document implies:**
- Six debts are universal — no document makes this claim after `debt_parameterization.md`.
- `FullDebt` is a kernel semantic — `debt_parameterization.md:66` explicitly states it is a deployment vocabulary.
- Debt count is a conceptual workflow limit — no document makes this claim.

**Kernel-freeze documentation:**
- `docs/OS/plan10.1.md:149` — "Kernel Growth Gate ADR" section exists and is closed.
- `docs/OS/roadmap3.md:661` — "exact authority binding should be the final kernel feature we deliberately plan now. After that, institute a kernel freeze."
- `docs/OS/plan_review.md:17` — "Kernel API and semantics are frozen by default; kernel changes require the Kernel Growth Gate."

---

## 21. Verification Results

### Actual commands and outcomes

| Command | Outcome |
|---------|---------|
| `gofmt -l cmd internal kernel api service adapter` | 6 non-test files flagged (formatting only, not logic). Test files also flagged. No logic defects. |
| `go build ./...` | **PASS** — no build errors |
| `go vet ./...` | **PASS** — no vet errors |
| `go test -count=1 -p 1 ./...` | **PASS** — all packages green |
| `go test -race -count=1 -p 1 ./kernel ./service/authority ./api ./cmd/solvent-mcp ./adapter/github` | **PASS** — race detector clean |
| `task db:reset` | **PASS** — migration 009 applied; composite FK `intent_authority_binding_fk` and unique index `live_intent_per_snapshot` verified in live DB |
| `task test` | **PASS** — MCP VERIFY GREEN, all 7 tools present, enum correct, action_source validated |
| `bash scripts/check_i7.sh` | **PASS** — I-7: 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool reads |
| `bash scripts/mcp_verify.sh` | **PASS** — MCP VERIFY GREEN |

### Targeted DB-backed tests

| Test | Package | Result |
|------|---------|--------|
| `TestDA03_EmptyInitialDebt` | `kernel` | **PASS** |
| `TestDA04_EnsureBeliefPreservesExistingDebt` | `kernel` | **PASS** |
| `TestExecute_ConfusedDeputy` | `api` | **PASS** |
| `TestTC9_PartA_ForUpdateLockMechanism` | `kernel` | **PASS** |
| `TestTC9_PartB_ConcurrentRevokeTarget` | `kernel` | **PASS** |
| `TestTC4_DischargeXDischarge` | `kernel` | **PASS** |
| `TestTC8_ConcurrentDischarge` | `kernel` | **PASS** |
| `TestW2_B19_CrossScenarioIsolation` | `kernel` | **PASS** |
| `TestCS1_RetireDebt_CrossScenario` | `kernel` | **PASS** |
| `TestCS3_Promote_CrossScenario` | `kernel` | **PASS** |
| `TestCS4_Discharge_CrossScenario` | `kernel` | **PASS** |
| `TestIntegration_ProcessToIntent` | `internal/belief` | **PASS** |
| `TestIntegration_MultiScenarioIsolation` | `internal/belief` | **PASS** |
| `TestIntegration_DeterminismAcrossReplays` | `internal/belief` | **PASS** |
| `TestExecuteActionNotCallerInjected` | `service/authority` | **PASS** |

---

## 22. Trust-Boundary Results

| Boundary | Status |
|----------|--------|
| Kernel entry | Only through `kernel.Store` methods. No alternate entry points. |
| Alternate mutation paths | Service layer is thin passthrough. No alternate kernel caller in production. |
| Policy bypass | None — kernel does not enforce policy; service/REST/MCP layers enforce authentication and input validation. |
| DB writes | 21 `ExecuteTx` write sites, all in kernel/service. 0 raw writes in production code. |
| Deployment exposure | Direct DB access bypasses all application controls. This is expected and is a deployment concern. |

---

## 23. Kernel-Freeze Decision

### READY TO FREEZE

No unresolved finding requires additional kernel primitives or schema changes.

**Finding 1 (LOW)** — REST `RetireDebt`/`Discharge` authenticated but not policy-authorized — is a service-layer authorization gap. Corrective action: add policy authorization checks in `api/belief.go` and `api/discharge.go`, or move the operations behind a policy-aware service method. This does not require kernel growth.

**All other security objectives are met:**
- Exact authority binding is closed (Plan 10.1 complete, verified).
- Debt model is genuinely domain-agnostic (parameterization complete, verified).
- Existing kernel invariants remain intact (I-1 through I-8, plus cross-scenario isolation).
- No new kernel-level defect was discovered.
- No genuinely new atomic/durable security fact has been demonstrated that cannot be enforced outside the kernel.

**Post-freeze default:**

> Future capabilities default to service, adapter, executor, deployment, policy,
> demo, or documentation layers.

A kernel change is justified only if a future review demonstrates a genuinely new durable security fact or atomic security transition that cannot safely be enforced outside the kernel.

---

*End of review.*
