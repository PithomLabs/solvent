# Exact Authority Binding — Implementation Plan (Revised)

## 1. Current Authority Model

### Database Objects (7 tables)

| Table | Role | Mutability |
|-------|------|-----------|
| `principal` | Identity (human/agent/workload/service) | Mutable (revoked_at) |
| `authority_target` | Proposal (5-tuple + params + justification refs) | Mutable until approval |
| `target_snapshot` | **Immutable approved authority content** | Append-only |
| `target_activation` | Links target → snapshot (one-ever via UNIQUE(target_id)) | Append-only |
| `target_revocation` | Revocation fact (append-only) | Append-only |
| `justification` | Links beliefs to targets (ON UPDATE CASCADE) | Mutable via cascade |
| `debt_discharge` | Debt retirement records | Append-only |

### Approved Authority Identity (frozen in `target_snapshot`)

```
snapshot = {
    principal_id,          -- who is authorized
    resource_type,         -- "scenario"
    resource_id,           -- scenario UUID
    scope,                 -- "belief:<belief_uuid>"
    action_namespace,      -- "solvent"
    action_name,           -- e.g. "deploy"
    consequence_type,      -- "execution"
    consequence_parameters -- JSONB: {repo, workflow, ref}
}
```

### Current Intent Identity (in `action_intent`)

```
intent = {
    scenario_id,
    belief_id,
    action,
    state
}
```

**Missing from intent**: `target_id`, `snapshot_id`.

### Authority Resolution (`authorizeWithinTx`)

Read-only verification:
1. Resolve `target_activation` → `target_snapshot` via composite FK
2. Check no `target_revocation` exists
3. Compare presented `AuthorityTuple` field-by-field against snapshot (8 fields)
4. Verify each justification's belief is currently `promoted`
5. Return `Allowed` + `ConsequenceParameters` (from snapshot)

### Current `AuthorizeResult`

```go
type AuthorizeResult struct {
    Allowed               bool
    Reason                string
    IntentState           string
    ConsequenceParameters []byte  // from snapshot
}
```

**Missing from result**: `SnapshotID`.

---

## 2. Current Security Gap

The confused-deputy class:

```
1. Create target T1 for (belief B1, action "deploy", target T1, params P1)
2. Approve T1 → snapshot S1 frozen
3. AuthorizeAndCreateIntent(T1, tuple) → creates intent I1 = (scenario, B1, "deploy")
4. Revoke T1
5. Create target T2 for (belief B1, action "deploy", target T2, params P2)
6. Approve T2 → snapshot S2 frozen
7. ExecuteAction(I1, target=T2, params from S2) → Authorize resolves T2 → tuple matches S2 → ALLOWED
```

The intent I1 was created against T1's authority but is executed against T2's authority. The intent stores no record of which target was authorized.

**Why this matters**: The approved consequential action is bound to a specific target. An intent should only be executable against the exact authority that created it.

**What's NOT a gap**:
- `Authorize` already compares the full 8-field tuple against the snapshot — action, target, and params are all checked at authorization time
- `ClaimIntent` CAS prevents concurrent consumption
- `ConsequenceParameters` come from the snapshot, not the caller
- The executor is selected by a hardcoded action→executor map

**The actual missing invariant**: `action_intent` does not record which target/snapshot was approved, so execution cannot prove the intent was created against the same authority it's executing against.

---

## 3. Exact Authority Invariant

> An intent may only be claimed for execution against the exact target and exact snapshot
> that were approved when the intent was created.

Formally:

```
ClaimIntent(intent_id, target_id, snapshot_id)
    ⟹
    intent.target_id == target_id
    ∧ intent.snapshot_id == snapshot_id
    ∧ target_activation.target_id == target_id
    ∧ target_activation.snapshot_id == snapshot_id
    ∧ NOT EXISTS (target_revocation WHERE target_id = intent.target_id)
```

The authority identity is:

```
Authority = (target_id, snapshot_id)
```

Where:
- `target_id` identifies which target was approved
- `snapshot_id` identifies the exact frozen snapshot content

The intent durably stores this binding. Execution proves it is still the same thing.

---

## 4. Authority Tuple

The approved authority is conceptually:

```
Approved Authority =
(
    target_id,            -- which target
    snapshot_id,          -- which frozen snapshot
    target_snapshot.*     -- the 8-field content (already compared by Authorize)
)
```

`belief` is NOT part of the durable authority identity. Belief is the warrant used to CREATE authority (via justification). Once approved, authority lives in the snapshot. The justification beliefs must remain promoted (checked by `Authorize`), but the belief is not the authority itself.

```
belief ≠ authority
```

---

## 5. Kernel Growth Gate ADR

### Option A — Service-only binding

Service code compares approved target/snapshot against requested target/snapshot at execution time.

| Criterion | Assessment |
|-----------|-----------|
| Security strength | Medium — depends on all service callers correctly comparing |
| Confused-deputy resistance | Partial — service logic can be bypassed or forgotten |
| Atomicity | No DB-level guarantee — TOCTOU between compare and execute |
| Replay resistance | No durable record of what was approved for this intent |
| TOCTOU exposure | Yes — target could change between compare and claim |
| Schema impact | None |
| Kernel size impact | None |
| API compatibility | Full |
| Migration cost | None |
| Whether invariant is durable | **No** — service logic is not a durable invariant |

### Option B — Kernel/DB-enforced exact authority binding

Store `target_id` + `snapshot_id` in `action_intent`. Kernel `ClaimIntent` verifies binding atomically.

| Criterion | Assessment |
|-----------|-----------|
| Security strength | High — DB-enforced, cannot be bypassed |
| Confused-deputy resistance | Full — mismatched target/snapshot causes SQL denial |
| Atomicity | CockroachDB transaction — intent claim is atomic with binding check |
| Replay resistance | Intent can only be claimed against its own target/snapshot |
| TOCTOU exposure | None — binding is in the same row as the intent |
| Schema impact | 2 new nullable columns + composite FK on `action_intent` |
| Kernel size impact | Minimal — extends existing `ClaimIntent` WHERE clause |
| API compatibility | Internal — no external API changes |
| Migration cost | One ALTER TABLE |
| Whether invariant is durable | **Yes** — enforced by CockroachDB |

### Option C — Hybrid

Store the binding durably (Option B) AND have service layers retain validation for error quality.

| Criterion | Assessment |
|-----------|-----------|
| Security strength | Highest — defense in depth |
| Confused-deputy resistance | Full — DB + service both check |
| Atomicity | Same as Option B |
| TOCTOU exposure | None |
| Schema impact | Same as Option B |
| Kernel size impact | Same as Option B |
| API compatibility | Full |
| Migration cost | Same as Option B |
| Whether invariant is durable | **Yes** — kernel/DB enforced, service validates for error quality |

### Comparison

| Factor | A | B | C |
|--------|---|---|---|
| Durable invariant | No | Yes | Yes |
| Confused-deputy | Partial | Full | Full |
| Kernel growth | None | Minimal | Minimal |
| Error quality | Good | Good | Best |
| Complexity | Low | Low | Low |

### Decision

**Option C — Hybrid**. The kernel/DB enforces the durable invariant. Service/API layers retain validation for error quality and defense-in-depth. This matches the existing pattern (e.g., `view.GetSnapshot` guards at REST layer + kernel `SELECT EXISTS` guard).

### Gate Result

```
GATE: PASS — kernel growth justified
```

The new durable security fact (intent→target→snapshot binding) genuinely cannot be safely expressed outside the kernel. The binding must survive service restarts, retries, and concurrent access. Only CockroachDB can enforce this atomically.

---

## 6. Target Binding

**Bind to `(target_id, snapshot_id)`** as a composite pair.

Rationale:
- `target_id` alone only proves that the target exists. It does not prove which snapshot content was approved.
- `snapshot_id` alone would be an unconstrained UUID — it could reference a snapshot belonging to a different target.
- The security invariant is the **pair** `(target_id, snapshot_id)`. The pair must therefore be database-enforced, not merely guaranteed by the Go code path.
- This matches the existing schema pattern used by `target_activation`, which has `FOREIGN KEY (target_id, snapshot_id) REFERENCES target_snapshot(target_id, snapshot_id)`.
- `target_activation` already enforces `UNIQUE(target_id)` — one activation ever per target. The composite FK on `action_intent` mirrors this pattern.

If the target's mutable representation changes later, the exact object approved is identified by `(target_id, snapshot_id)` — the activation+snapshot pair that was frozen at `Approve` time.

---

## 7. Action Binding

Action is currently:
- Free text stored in `action_intent.action` and `target_snapshot.action_name`
- `Authorize` compares `tuple.ActionName` against `snapActionName` (exact string match)
- `ClaimIntent` already binds `(belief_id, action)` — Plan 9.0 remediation

**The binding is already exact for action**. A mismatched action causes:
- `Authorize` to return `"action_name mismatch"`
- `ClaimIntent` WHERE clause to match zero rows (belief_id + action check)

No additional action-binding work needed beyond what Plan 9.0 already implemented.

---

## 8. Consequence-Parameter Binding

Current state:
- `target_snapshot.consequence_parameters` stores the frozen approved JSONB
- `Authorize` returns `ConsequenceParameters` from the snapshot
- `ExecuteAction` unmarshals snapshot params into `execParams` (repo, workflow, ref)
- Caller-supplied `params` is always `map[string]interface{}{}` — ignored

**The binding is already exact for consequence parameters**. The executor receives ONLY snapshot params, never caller params.

**However**: `action_intent` does not store the snapshot's consequence parameters. If the snapshot is somehow replaced (impossible due to UNIQUE(target_id) on activation), the intent would not know.

**Decision**: Store `snapshot_id` in `action_intent`. The snapshot_id identifies the exact consequence parameters. No need to duplicate the JSONB in the intent row.

---

## 9. Execution Path

### Field-Lineage Table

| Field | Request source | Authoritative source | Compared where? |
|-------|---------------|---------------------|----------------|
| scenario_id | Request body / MCP args | Caller | `ClaimIntent` WHERE, `Authorize` ResourceID |
| belief_id | Request body / MCP args | Caller | `ClaimIntent` WHERE, `gate` FK |
| action | Request body / MCP args | Caller | `ClaimIntent` WHERE, `Authorize` ActionName |
| target_id | Request body / MCP args | Caller → kernel checks against snapshot | `ClaimIntent` WHERE (new), `Authorize` via activation FK |
| snapshot_id | **DB snapshot** (read at entry, passed through) | `target_snapshot.snapshot_id` via activation | `ClaimIntent` WHERE (new) |
| intent_id | Request body / MCP args | Created by `AuthorizeAndCreateIntent` | `ClaimIntent` WHERE |
| actor_id | **Server-derived** (AuthFromContext / "mcp-agent") | Authenticated principal | `Authorize` PrincipalID |
| consequence_type | Defaults to "execution" | Entry point | `Authorize` ConsequenceType |
| consequence_parameters | **DB snapshot** (read at entry, passed through) | `target_snapshot.consequence_parameters` | `Authorize` jsonEqual, executor receives |
| executor | **Hardcoded map** (action → executor name) | `actionExecutorMap` | Service layer |

### The Gap in the Path

```
AuthorizeAndCreateIntent:
    1. LOCK target (FOR UPDATE)
    2. authorizeWithinTx → reads snapshot, compares tuple → ALLOWED
    3. createIntentWithinTx → INSERT (scenario_id, belief_id, action)  ← MISSING: target_id, snapshot_id
    4. Commit

ExecuteAction:
    1. Read snapshot params from DB
    2. PrepareForAction → kernel.Authorize → compares tuple against snapshot → ALLOWED
    3. ClaimIntent → CAS WHERE (id, scenario_id, belief_id, action)  ← MISSING: target_id, snapshot_id check
    4. Execute provider with snapshot params
```

After fix:

```
AuthorizeAndCreateIntent:
    1. LOCK target (FOR UPDATE)
    2. authorizeWithinTx → reads snapshot, compares tuple → ALLOWED, returns snapshot_id
    3. createIntentWithinTx → INSERT (scenario_id, belief_id, action, target_id, snapshot_id)
    4. Commit

ExecuteAction:
    1. Read snapshot params + snapshot_id from DB
    2. PrepareForAction → kernel.Authorize → compares tuple → ALLOWED
    3. ClaimIntent → CAS WHERE (id, scenario_id, belief_id, action, target_id, snapshot_id)
    4. Execute provider with snapshot params
```

---

## 10. Caller Inventory

### Kernel callers that need signature changes

| Caller | Current call | New call |
|--------|-------------|----------|
| `kernel/authority.go:470` `AuthorizeAndCreateIntent` | `(ctx, targetID, tuple, scenarioID, beliefID, action)` | No change — targetID already passed |
| `kernel/authority.go:534` `ClaimIntent` | `(ctx, scenarioID, intentID, beliefID, action)` | `(ctx, scenarioID, intentID, beliefID, action, targetID, snapshotID)` |
| `kernel/kernel.go` `createIntentWithinTx` | `(ctx, tx, scenarioID, beliefID, action)` | `(ctx, tx, scenarioID, beliefID, action, targetID, snapshotID)` |

### Service callers that need updating

| File | Function | Change |
|------|----------|--------|
| `service/authority/authority.go:309` | `ExecuteAction` → `ClaimIntent` | Pass `targetID`, `snapshotID` |
| `service/authority/authority.go:172` | `PrepareForAction` → `kernel.Authorize` | No change — Authorize already has targetID |
| `service/ledger/ledger.go:141` | `AuthorizeAndCreateIntent` | No change — passthrough to kernel |

### API callers that need updating

| File | Function | Change |
|------|----------|--------|
| `api/authorization.go:114-118` | Read snapshot params | Also read `snapshot_id` |
| `api/authorization.go:225-229` | Read snapshot params (execute) | Also read `snapshot_id` |
| `api/errors.go` | `writeKernelError` | Add case for `ErrDuplicateIntent` → 409 |

### MCP callers that need updating

| File | Function | Change |
|------|----------|--------|
| `cmd/solvent-mcp/tools.go:248-252` | Read snapshot params (authorize) | Also read `snapshot_id` |
| `cmd/solvent-mcp/tools.go:690-694` | Read snapshot params (execute) | Also read `snapshot_id` |

### Test callers that need updating

| File | Approximate count |
|------|------------------|
| `kernel/authority_test.go` | ~30 `ClaimIntent` calls |
| `kernel/kernel_test.go` | ~10 `ClaimIntent` calls |
| `service/authority/authority_integration_test.go` | ~1 `ClaimIntent` call |
| `adapter/github/executor_test.go` | ~8 `ClaimIntent` calls |

### Callers that do NOT need changes

- `kernel/authority.go` `CompleteIntent`, `RollbackClaim`, `CancelIntent` — operate on already-claimed intents
- `kernel/kernel.go` `IntentOnPromoted` — creates intents without authority binding (pre-approval path)
- `internal/wizard/` — calls `s.Authorize` (service-level) for display purposes
- `cmd/operator-review/` — comment only, no live calls

---

## 11. Schema / DB Design

### New migration: `db/009_exact_authority_binding.sql`

```sql
-- Exact authority binding: bind intent to the exact target and snapshot
-- that were approved when the intent was created.
--
-- This closes the confused-deputy class: an intent created against target T1
-- cannot be executed against target T2.

-- Step 1: Add columns (nullable for pre-existing unbound intents).
ALTER TABLE action_intent
  ADD COLUMN target_id   UUID,
  ADD COLUMN snapshot_id UUID;

-- Step 2: Add composite FK enforcing (target_id, snapshot_id) correspondence.
-- target_id alone only proves the target exists. snapshot_id alone would be an
-- unconstrained UUID. The security invariant is the pair, so the pair must be
-- database-enforced — not merely guaranteed by the Go code path. This mirrors
-- the existing pattern used by target_activation.
-- ON DELETE CASCADE is defense-in-depth: target_snapshot is append-only, but
-- the cascade prevents orphaned intent rows if the FK is ever violated.
ALTER TABLE action_intent
  ADD CONSTRAINT intent_authority_binding_fk
  FOREIGN KEY (target_id, snapshot_id)
  REFERENCES target_snapshot(target_id, snapshot_id)
  ON DELETE CASCADE;

-- Step 3: Unique index — one live intent per exact authority instance.
--
-- Authority = (target_id, snapshot_id). A target is unique to one
-- (scenario, belief, action) tuple via the target schema, so the two-column
-- index enforces the stronger invariant: at most one live intent per authority
-- instance. The IS NOT NULL guard excludes pre-approval (NULL, NULL) intents.
--
-- This is a uniqueness constraint, not an idempotency-key mechanism.
-- A retry after an ambiguous timeout may encounter an already-existing
-- equivalent live intent. Full idempotency semantics are intentionally
-- out of scope for this change.
CREATE UNIQUE INDEX IF NOT EXISTS live_intent_per_snapshot
  ON action_intent (target_id, snapshot_id)
  WHERE state = 'live' AND target_id IS NOT NULL AND snapshot_id IS NOT NULL;
```

**Failure behavior**: Existing intents without `target_id`/`snapshot_id` will have NULL values. The `ClaimIntent` WHERE clause will fail for NULL values (NULL != UUID). The composite FK permits NULLs (PostgreSQL/CockroachDB NULL semantics: NULL != NULL passes FK check). This is correct — pre-authority intents should not be claimable against any authority.

**Why the composite FK is required**:
- `target_id` alone only proves that the target exists.
- `snapshot_id` alone would be an unconstrained UUID.
- The security invariant is the pair `(target_id, snapshot_id)`.
- The pair must therefore be database-enforced, not merely guaranteed by the Go code path.
- This matches the existing schema pattern used by `target_activation`.

**Rollback**:
```sql
DROP INDEX IF EXISTS live_intent_per_snapshot;
ALTER TABLE action_intent DROP CONSTRAINT IF EXISTS intent_authority_binding_fk;
ALTER TABLE action_intent DROP COLUMN IF EXISTS target_id;
ALTER TABLE action_intent DROP COLUMN IF EXISTS snapshot_id;
```

---

## 12. Error Semantics

### Error classification table

| Error | Kernel | Service | REST | MCP | Security decision? |
|-------|--------|---------|------|-----|-------------------|
| Target not found | `ErrTargetNotFound` | 404 | 404 | error | No — resource not found |
| Intent-target mismatch | `ErrIntentNotLive` (advisory: target_id mismatch) | 400 | 400 | error | No — intent/authority mismatch |
| Snapshot mismatch | `ErrIntentNotLive` (advisory: snapshot_id mismatch) | 400 | 400 | error | No — intent/authority mismatch |
| Target revoked | Denial via `Allowed: false` | Allowed=false | Allowed=false | isError | **Yes** — authorization denial |
| Intent not live | `ErrIntentNotLive` | 400 | 400 | error | No — state machine violation |
| Duplicate equivalent intent | `ErrDuplicateIntent` | 409 | 409 Conflict | conflict error | **No** — not an authorization denial |
| Intent not found | `ErrIntentNotLive` (advisory: no such intent) | 400 | 400 | error | No — resource not found |

### Authorization denial vs. duplicate-intent conflict

The security decision must be distinguished from a duplicate-intent conflict:

- **403 / denial** (`Allowed: false`): The caller/action is not authorized. This is a security decision.
- **409 / conflict** (`ErrDuplicateIntent`): Authorization is not being denied; an equivalent live intent already exists for the exact authority. This is a state conflict, not a security refusal.

Callers must not interpret 409/conflict as a security refusal. Where safely available, the existing intent ID should be surfaced so a retrying caller can recover deterministically. Do not introduce full idempotency-key semantics.

### New sentinel errors

```go
ErrDuplicateIntent = errors.New("duplicate equivalent live intent")
```

### Detection and mapping

**At creation time** (`createIntentWithinTx`): The INSERT into `action_intent` hits the `live_intent_per_snapshot` index when a live intent with the same `(scenario_id, belief_id, action, target_id, snapshot_id)` already exists. CockroachDB returns SQLSTATE `23505` (unique violation). The kernel wraps this:

```go
return wrapIf(sqlStateUniqueViolation, ErrDuplicateIntent, err)
```

This follows the established pattern used by `ErrDuplicateDischarge` (`kernel/authority.go`).

**At claim time** (`ClaimIntent`): The CAS UPDATE returns zero rows affected. This can mean:
1. Intent does not exist
2. Intent exists but is not in `live` state
3. Intent exists but tuple does not match (belief_id, action, target_id, or snapshot_id mismatch)

The CAS remains atomic and authoritative. After zero rows affected, an advisory SELECT classifies the failure for diagnostic purposes. The advisory read does NOT become the security decision — the CAS is the decision. The advisory SELECT returns:

- Intent not found → `ErrIntentNotLive` with diagnostic detail "no such intent"
- Intent found but not live → `ErrIntentNotLive` with diagnostic detail "state=<current>"
- Intent found, live, but tuple mismatch → `ErrIntentNotLive` with diagnostic detail "mismatch on <field>"

The kernel returns `ErrIntentNotLive` in all cases (preserving the existing contract). The service layer may add diagnostic detail in the error response but must not change the security decision based on the advisory read.

**API layer** (`api/errors.go`): Add case for `ErrDuplicateIntent`:
```go
case errors.Is(err, kernel.ErrDuplicateIntent):
    status = http.StatusConflict
    code = "duplicate_intent"
    message = err.Error()
```

**MCP layer**: `ErrDuplicateIntent` maps to `errorResult(err)` with `IsError: true`. The error message includes "duplicate equivalent live intent" — not a permission-denied semantic.

### ClaimIntent CAS preserved

Do NOT change the atomic ClaimIntent CAS into a non-atomic pre-check. The claim remains authoritative. The advisory diagnostic read after zero rows affected is for error classification only — it does not gate the security decision.

---

## 13. Audit Semantics

Current audit already records:
- `ActivityAuthorizationGranted` / `ActivityAuthorizationDenied` — from `PrepareForAction`
- `ActivityAdapterInvoked` — from `ExecuteAction` step 6
- `ActivityExecutorDenied` — when `Allowed: false`

**No changes needed** to the audit model. The exact-binding check happens inside `ClaimIntent`, which is called within `ExecuteAction`. If it fails, `ExecuteAction` returns an error, and the audit entry already captures the denial.

A duplicate-intent conflict (409) is NOT an authorization denial and must not produce an `ActivityAuthorizationDenied` audit entry. It is a state conflict, not a security event.

Existing principle preserved:

```
Authorize ≠ Execute
```

A failed `ClaimIntent` does not produce a false "authorized" event.

---

## 14. Confused-Deputy Test Matrix

### Test: Exact match → ALLOW

```
Approved: (belief B1, action "deploy", target T1, snapshot S1, params {repo: org/svc, ref: main})
Requested: (belief B1, action "deploy", target T1, snapshot S1)
→ ALLOW, executor invoked with snapshot params
```

### Test: Wrong target → DENY

```
Approved: (belief B1, action "deploy", target T1, snapshot S1)
Created intent I1 against T1
Approved: (belief B1, action "deploy", target T2, snapshot S2)
Execute I1 with target T2
→ DENY: intent_target mismatch, no executor invocation
```

### Test: Wrong snapshot → DENY

```
Approved: (belief B1, action "deploy", target T1, snapshot S1)
Created intent I1 against T1/S1
Target T1 somehow has a different snapshot (impossible due to UNIQUE(target_id), but test defensively)
Execute I1 with target T1, snapshot S2
→ DENY: intent_snapshot mismatch
```

### Test: Wrong action → DENY (already covered by Plan 9.0)

```
Approved: (belief B1, action "deploy", target T1)
Created intent I1 for action "deploy"
Execute I1 with action "rollback"
→ DENY: ClaimIntent WHERE clause mismatches action
```

### Test: Wrong belief → DENY (already covered by Plan 9.0)

```
Approved: (belief B1, action "deploy", target T1)
Created intent I1 for belief B1
Execute I1 with belief B2
→ DENY: ClaimIntent WHERE clause mismatches belief_id
```

### Test: Revoked target → DENY

```
Approved: (belief B1, action "deploy", target T1, snapshot S1)
Created intent I1 against T1
Revoke T1
Execute I1 with target T1
→ DENY: Authorize returns Allowed=false (revocation check)
```

### Test: Stale authority → DENY

```
Approved: (belief B1, action "deploy", target T1, snapshot S1)
Created intent I1 against T1
Retract B1 → belief status changes → justification ON UPDATE CASCADE → Authorize denies
Execute I1
→ DENY: Authorize returns Allowed=false (belief not promoted)
```

### Test: Same action + different target → DENY

```
Approved: (belief B1, action "deploy", target T1, params P1)
Approved: (belief B1, action "deploy", target T2, params P2)
Created intent I1 against T1
Execute I1 with target T2
→ DENY: intent_target mismatch
```

### Test: Same action + same target + different params → DENY

```
This is impossible in the current schema: UNIQUE(target_id) on activation means
one snapshot per target. Different params require a different target.
But test defensively by verifying snapshot_id check.
```

### Test: Duplicate equivalent intent → CONFLICT (not denial)

```
Approved: (belief B1, action "deploy", target T1, snapshot S1)
AuthorizeAndCreateIntent → intent I1 created
AuthorizeAndCreateIntent again with same authority tuple
→ 409 Conflict: ErrDuplicateIntent
→ No second live intent created
→ No authorization denial audit event
```

### Test: Composite FK rejects mismatched pair → DB ENFORCED

```
INSERT INTO action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
VALUES (..., T1, S2)  -- where S2 belongs to T2, not T1
→ FK violation: intent_authority_binding_fk
→ No row inserted
```

This verifies that the invariant is enforced by CockroachDB, not merely by application code.

---

## 15. Regression Requirements

All existing invariants must be preserved:

- `promoted_is_debt_free` — CHECK constraint, unchanged
- `gate` — composite FK on action_intent, unchanged
- `live_requires_promoted` — CHECK constraint, unchanged
- Scenario isolation — NEW-02 fix, unchanged
- Intent ownership — NEW-01 fix, unchanged
- ClaimIntent CAS — CI-4, extended with target_id + snapshot_id
- AuthorizeAndCreateIntent atomicity — unchanged
- RevokeTarget behavior — unchanged
- Target snapshot immutability — unchanged
- Fixed executor mapping — unchanged
- Provider ambiguity handling — unchanged
- ReconcileIntent audit ordering — unchanged
- MCP no-direct-write invariant — unchanged
- TOKEN ≠ AUTHORITY — unchanged

New invariant added:
- **Exact authority binding**: intent.target_id + intent.snapshot_id match the approved target/snapshot at claim time, enforced by composite FK and ClaimIntent CAS

---

## 16. Files Expected to Change

### Kernel

| File | Change |
|------|--------|
| `kernel/sql.go` | `sqlClaimIntent` adds `target_id` + `snapshot_id` to WHERE; `sqlIntentOnPromoted` adds `target_id` + `snapshot_id` to INSERT; `sqlAuthorizeResolve` adds `ts.snapshot_id` to SELECT |
| `kernel/authority.go` | `AuthorizeResult` adds `SnapshotID`; `authorizeWithinTx` scans + returns `snapshot_id`; `ClaimIntent` signature adds `targetID, snapshotID`; `AuthorizeAndCreateIntent` passes `targetID, snapshotID` to `createIntentWithinTx`; error classification after zero-rows CAS |
| `kernel/kernel.go` | `createIntentWithinTx` signature adds `targetID, snapshotID`, INSERT includes them; wraps 23505 as `ErrDuplicateIntent` |
| `kernel/contract.go` | `ClaimIntent` signature update |
| `kernel/errors.go` | Add `ErrDuplicateIntent` |

### Service

| File | Change |
|------|--------|
| `service/authority/authority.go` | `ExecuteAction` reads `snapshotID` from `AuthorizeResult`, passes to `ClaimIntent` |
| `service/ledger/ledger.go` | No change — passthrough |

### API

| File | Change |
|------|--------|
| `api/authorization.go` | Authorize handler: also SELECT `ts.snapshot_id`; Execute handler: also SELECT `ts.snapshot_id` |
| `api/errors.go` | Add `ErrDuplicateIntent` → 409 Conflict mapping |

### MCP

| File | Change |
|------|--------|
| `cmd/solvent-mcp/tools.go` | Authorize tool: also SELECT `ts.snapshot_id`; Execute tool: also SELECT `ts.snapshot_id` |

### Database

| File | Change |
|------|--------|
| `db/009_exact_authority_binding.sql` | New migration: ADD `target_id`, `snapshot_id` columns; ADD composite FK `(target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)`; ADD unique index `live_intent_per_snapshot` |

### Tests

| File | Change |
|------|--------|
| `kernel/kernel_test.go` | All `ClaimIntent` calls add `targetID, snapshotID`; new confused-deputy regression tests (wrong target, wrong snapshot, exact match, duplicate intent, composite FK rejection) |
| `kernel/authority_test.go` | `AuthorizeAndCreateIntent` callers may need updates; new tests |
| `kernel/authority_dogfood_test.go` | Possible updates for `ClaimIntent` signature |
| `service/authority/authority_integration_test.go` | `ClaimIntent` call updates; new integration tests |
| `adapter/github/executor_test.go` | All `ClaimIntent` calls add `targetID, snapshotID` |

### Documentation

| File | Change |
|------|--------|
| `IMPLEMENTATION_CONTRACT.md` | Update frozen schema if needed |

---

## 17. Implementation Sequence

1. **Verify immutability**: Confirm no code path UPDATEs `target_id` or `snapshot_id` on `action_intent` after INSERT (they don't exist yet, so this is trivially true)
2. **Add `SnapshotID` to `AuthorizeResult`** and `sqlAuthorizeResolve` SELECT
3. **Update `authorizeWithinTx`** to scan and return `snapshot_id`
4. **Update `AuthorizeAndCreateIntent`** to pass `targetID, snapshotID` to `createIntentWithinTx`
5. **Update `createIntentWithinTx`** to INSERT `target_id, snapshot_id`; wrap 23505 as `ErrDuplicateIntent`
6. **Update `sqlClaimIntent`** to include `target_id, snapshot_id` in WHERE
7. **Update `ClaimIntent`** signature to accept `targetID, snapshotID`; add advisory diagnostic after zero-rows CAS
8. **Update `Contract` interface** for `ClaimIntent`
9. **Update `kernel/errors.go`** — add `ErrDuplicateIntent`
10. **Update `service/authority/authority.go`** `ExecuteAction` to pass `targetID, snapshotID` from `AuthorizeResult` to `ClaimIntent`
11. **Update REST handlers** to also SELECT `ts.snapshot_id` from snapshot join
12. **Update MCP handlers** to also SELECT `ts.snapshot_id` from snapshot join
13. **Update `api/errors.go`** — add `ErrDuplicateIntent` → 409 mapping
14. **Write migration** `db/009_exact_authority_binding.sql`
15. **Update all test callers** of `ClaimIntent` (kernel, service, adapter tests)
16. **Add confused-deputy regression tests** (wrong target, wrong snapshot, exact match, duplicate intent, composite FK rejection)
17. **Update B-24 test** if needed
18. **Run full verification suite**
19. **Fresh adversarial review**
20. **Freeze kernel**

---

## 18. Verification Plan

```bash
# After migration
task db:reset

# Build and vet
gofmt -l cmd internal kernel
go build ./...
go vet ./...

# Full test suite
go test -count=1 -p 1 ./...

# Race detector
go test -race -count=1 -p 1 ./...

# Taskfile checks (includes i7, MCP boundary, sentry containment, mcp_verify.sh)
task test

# Manual verification
bash scripts/check_i7.sh
bash scripts/mcp_verify.sh
```

### Tests that prove exact authority binding

| Test | Proves |
|------|--------|
| `TestCS_EXACT_ExactMatch` | Valid intent + correct target + correct snapshot → ALLOW |
| `TestCS_EXACT_WrongTarget` | Valid intent + wrong target → DENY, no executor |
| `TestCS_EXACT_WrongSnapshot` | Valid intent + wrong snapshot → DENY, no executor |
| `TestCS_EXACT_RevokedTarget` | Valid intent + revoked target → DENY via Authorize |
| `TestCS_EXACT_StaleAuthority` | Valid intent + retracted belief → DENY via Authorize |
| `TestCS_EXACT_WrongAction` | Already covered by Plan 9.0 (NEW-01) |
| `TestCS_EXACT_WrongBelief` | Already covered by Plan 9.0 (NEW-01) |

### Tests that prove duplicate-equivalent-intent conflict

| Test | Proves |
|------|--------|
| `TestCS_EXACT_DuplicateIntent` | Second creation with same authority tuple → `ErrDuplicateIntent` (not authorization denial) |
| `TestCS_EXACT_DuplicateIntentREST` | REST duplicate → HTTP 409 Conflict |
| `TestCS_EXACT_DuplicateIntentMCP` | MCP duplicate → `isError` conflict error (not permission-denied) |

### Tests that prove composite FK enforcement

| Test | Proves |
|------|--------|
| `TestCS_EXACT_CompositeFKRejectsMismatch` | `target_id=T1, snapshot_id=S2` (where S2 belongs to T2) → FK violation, no row inserted |
| `TestCS_EXACT_CompositeFKRejectsNonexistent` | `target_id=T1, snapshot_id=<nonexistent>` → FK violation, no row inserted |

These verify that the invariant is enforced by CockroachDB, not merely by application code.

---

## 19. Kernel Freeze Decision

After exact authority binding is implemented and verified:

```
KERNEL FREEZE
```

Rationale:
- **Belief lifecycle**: Complete (enter, promote, retract, retire debt, discharge)
- **Authority lifecycle**: Complete (create target, attach justification, request, approve, authorize, revoke)
- **Intent lifecycle**: Complete (create, claim, complete, rollback, cancel)
- **Execution path**: Complete (authorize → claim → execute → complete/rollback)
- **Scenario isolation**: Enforced (NEW-02)
- **Intent ownership**: Enforced (NEW-01)
- **Evidence isolation**: Enforced (NEW-03)
- **Exact authority binding**: Enforced (this plan)
- **No additional kernel primitives planned** unless a future security review identifies a genuinely new atomic security invariant that cannot be expressed outside the existing kernel

The duplicate-intent conflict semantics introduced in this plan do not constitute a new kernel primitive or a new kernel capability; they are error semantics around the existing intent uniqueness invariant.

The kernel is intentionally small. The remaining gaps (promotion-epoch identity, idempotency under retry, non-DB audit) are service-level concerns that do not require kernel growth.

---

## 20. Acceptance Criteria

Every criterion must be objectively testable:

1. **Exact match**: Intent created for (belief B1, action "deploy", target T1, snapshot S1) is claimable only with (target T1, snapshot S1)
2. **Wrong target**: Intent for T1 cannot be claimed with T2 → `ErrIntentNotLive`
3. **Wrong snapshot**: Intent for S1 cannot be claimed with S2 → `ErrIntentNotLive`
4. **Wrong action**: Intent for "deploy" cannot be claimed with "rollback" → `ErrIntentNotLive` (Plan 9.0)
5. **Wrong belief**: Intent for B1 cannot be claimed with B2 → `ErrIntentNotLive` (Plan 9.0)
6. **Revoked target**: Intent for revoked target → `Allowed: false` from Authorize
7. **Stale authority**: Intent when justification belief retracted → `Allowed: false` from Authorize
8. **No executor invocation**: All DENY cases produce zero external side effects
9. **No false audit**: DENY cases do not produce false "authorized" or "executed" audit events
10. **Composite FK**: `action_intent` cannot store a `target_id`/`snapshot_id` pair that does not correspond to a real target snapshot
11. **Duplicate conflict**: Creating an equivalent live intent returns a defined conflict (`ErrDuplicateIntent`), not an authorization denial
12. **REST conflict semantics**: Duplicate-equivalent intent maps to HTTP 409 Conflict
13. **MCP conflict semantics**: Duplicate-equivalent intent is represented as a conflict error (`isError`), not a permission denial
14. **No accidental idempotency**: No idempotency-key/state machinery is introduced by this change
15. **Schema migration**: `task db:reset` applies all 9 migrations successfully
16. **All existing tests pass**: `task test` green
17. **Race-free**: `go test -race` green
18. **MCP boundary**: `mcp_verify.sh` green
19. **I-7 invariant**: `check_i7.sh` green (21 ExecuteTx sites)
20. **Kernel freeze**: No additional kernel primitives or schema changes planned
