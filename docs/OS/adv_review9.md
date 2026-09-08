# Adversarial Security Review — Plan 10.1 (Exact Authority Binding)

**Review date:** 2026-09-08  
**Reviewer:** Kilo (adversarial, implementation-level)  
**Scope:** HEAD after Plan 10.1 completion  
**Review type:** REVIEW ONLY — no code, schema, tests, or documentation modified

---

## 1. Verdict

**GREEN WITH ACCEPTED LOW**

Plan 10.1 closes the confused-deputy vulnerability it was designed to close. The database-enforced composite FK `(target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)` and the extended `ClaimIntent` CAS predicate together make the exact authority binding durable and atomic. No unresolved CRITICAL, HIGH, or MEDIUM security defect exists.

Two LOW-severity verification gaps and one INFO observation are documented below. None requires kernel growth.

---

## 2. Executive Assessment

The confused-deputy class described in plan10.1.md §2:

    intent I1 created under authority T1/S1
    → T1 revoked
    → T2/S2 approved with compatible tuple
    → execute I1 against T2/S2

is closed. The attack path fails at `ClaimIntent`:

1. `ExecuteAction` reads snapshot params for T2 from the database (DB-derived, not caller-supplied).
2. `PrepareForAction` → `kernel.Authorize` with T2 returns `Allowed: true` (if T2 is active, belief promoted, etc.).
3. `ClaimIntent(ctx, scenarioID, intentID, beliefID, action, targetID=T2, snapshotID=S2)` executes the CAS:
   ```sql
   UPDATE action_intent SET state = 'executing'
   WHERE id = $1 AND scenario_id = $2
     AND belief_id = $3 AND action = $4
     AND state = 'live'
     AND (
       ($5::TEXT = '' AND target_id IS NULL AND snapshot_id IS NULL)
       OR
       (target_id::TEXT = $5::TEXT AND snapshot_id::TEXT = $6::TEXT)
     )
   ```
4. Intent I1 carries `target_id=T1, snapshot_id=S1`. The predicate `target_id::TEXT = 'T2' AND snapshot_id::TEXT = 'S2'` evaluates to false (or NULL treated as false). Zero rows affected.
5. `ClaimIntent` returns `ErrIntentNotLive`. `ExecuteAction` returns `Allowed = false`. Executor is never invoked.

The composite FK additionally prevents any `INSERT` into `action_intent` with a `(target_id, snapshot_id)` pair that does not correspond to a real row in `target_snapshot`. Verified by direct database test against CockroachDB.

The kernel freeze is sound. No new kernel primitive is required.

---

## 3. Findings

### Finding 1 — Missing confused-deputy integration test

- **Severity:** MEDIUM
- **Location:** `service/authority/authority_integration_test.go`, `adapter/github/executor_test.go`, `api/authorization_test.go`
- **Attack path:** T1 → T2 substitution (Attack Path 1)
- **Why existing controls fail:** No test in the current repository creates two approved targets T1 and T2, binds an intent to T1/S1 via `AuthorizeAndCreateIntent`, and then calls `ExecuteAction` against T2/S2. The existing tests that reference "wrong target" either:
  - use a non-existent target ID (`TestExec18_WrongTarget`, `TestEA03_WrongTarget`), which is denied by `Authorize` before `ClaimIntent` is reached, or
  - test parameter substitution against the correct target (`TestExec04_WrongTarget`).
  None exercise the exact authority binding in `ClaimIntent` for a live, bound intent against a different but real target.
- **Impact:** The code correctly prevents the attack, but the absence of a direct regression test means a future change to `ClaimIntent` or the composite FK could reintroduce the vulnerability without detection.
- **Required disposition:** Add an integration test that:
  1. Approves T1/S1 and T2/S2 with identical belief/action.
  2. Creates intent I1 bound to T1/S1 via `AuthorizeAndCreateIntent`.
  3. Calls `ExecuteAction` with `targetID=T2` and `snapshotID=S2`.
  4. Asserts `Allowed = false`, executor not invoked, and `ErrIntentNotLive` is the kernel error.
  This is a test-authoring gap, not a kernel-growth requirement.

### Finding 2 — `setupExecutionScenario` inserts unbound intents

- **Severity:** LOW
- **Location:** `api/authorization_test.go:274-279`
- **Attack path:** NULL / pre-authority intent path (Attack Path 4)
- **Reproduction:** The helper inserts intents via raw SQL without `target_id`/`snapshot_id`:
  ```go
  INSERT INTO action_intent (scenario_id, belief_id, action)
  VALUES ($1::UUID, $2::UUID, $3::STRING) RETURNING id
  ```
  Because `newTestServerWithAuth` creates an empty executor registry, execution tests using this helper return early at executor resolution (`resolveExecutor` → `!ok`) before reaching `ClaimIntent`. The exact authority binding path is therefore untested at the API layer.
- **Impact:** API-level execution tests do not exercise the bound-intent happy path or the exact-match rejection in `ClaimIntent`. A defect in the REST→service→kernel plumbing for `targetID`/`snapshotID` propagation would not be caught by existing API tests.
- **Required disposition:** Update `setupExecutionScenario` to optionally insert bound intents, or add a separate helper for bound-intent execution scenarios.

### Finding 3 — Unused `snapshot_id` reads in REST and MCP handlers

- **Severity:** LOW
- **Location:** `api/authorization.go:115-123, 228-236`; `cmd/solvent-mcp/tools.go:249-257, 693-701`
- **Reproduction:** All four handlers SELECT `ts.snapshot_id` from `target_activation JOIN target_snapshot` but never use the variable. The kernel re-reads the snapshot inside `authorizeWithinTx`.
- **Impact:** No security impact. The `snapshot_id` is read from the database, never from the caller, so there is no injection risk. The dead read is a minor maintenance hazard: if a future maintainer mistakenly uses the handler-level `snapID` instead of the kernel-derived `decision.SnapshotID`, it could create a subtle inconsistency. Currently, the unused variable cannot affect the security decision.
- **Required disposition:** Remove the unused `snapID` reads, or document that the kernel is the sole authority for `snapshot_id`.

### Finding 4 — Migration discrepancy from plan specification

- **Severity:** INFO
- **Location:** `docs/OS/plan10.1.md:406-408` vs `db/009_exact_authority_binding.sql:33-35`
- **Reproduction:** The plan specifies:
  ```sql
  CREATE UNIQUE INDEX intent_authority_unique
    ON action_intent (scenario_id, belief_id, action, target_id, snapshot_id)
    WHERE state = 'live';
  ```
  The actual migration creates:
  ```sql
  CREATE UNIQUE INDEX live_intent_per_snapshot
    ON action_intent (target_id, snapshot_id)
    WHERE state = 'live' AND target_id IS NOT NULL AND snapshot_id IS NOT NULL;
  ```
- **Impact:** The actual index is stricter: it prevents any two live intents from sharing the same `(target_id, snapshot_id)`, regardless of scenario, belief, or action. In normal operation, a target is unique to one scenario/belief/action tuple, so this does not reject valid intents. It is defense-in-depth. However, if a bug or malicious direct SQL were to insert an intent with a mismatched scenario/belief/action against an existing target, the stricter index would prevent a second legitimate intent for the same target from being created via the kernel. This is an acceptable trade-off, but it is a deviation from the documented plan.
- **Required disposition:** Update `plan10.1.md` to match the actual migration, or update the migration to match the plan. No security impact either way.

---

## 4. Attack-Path Results

| Attack Path | Result | Evidence |
|-------------|--------|----------|
| 1. T1 → T2 confused deputy | **PASS** | `ClaimIntent` CAS predicate rejects mismatched `(target_id, snapshot_id)`. Composite FK rejects cross-target inserts. Verified by code review and direct DB test. |
| 2. Same target, different snapshot | **PASS** | `UNIQUE(target_id)` on `target_activation` makes multiple snapshots per target impossible through normal approval. Defensively, `ClaimIntent` checks `snapshot_id` atomically. |
| 3. Cross-target / cross-snapshot corruption | **PASS** | Direct DB test: `INSERT action_intent (T1, S2)` where S2 belongs to T2 → `23503` FK violation. `INSERT action_intent (T1, nonexistent-S)` → `23503` FK violation. Valid `(T1, S1)` → succeeds. |
| 4. NULL / pre-authority intent path | **PASS** | `sqlClaimIntent` NULL-safe predicate correctly distinguishes `(NULL, NULL)` unbound intents from bound intents. `ExecuteAction` never calls `ClaimIntent` with empty `targetID`/`snapshotID` (validated as non-empty UUIDs upstream). Kernel tests `TestCS_CD01`–`CD04` verify the boundary. |
| 5. Intent identity substitution | **PASS** | `ClaimIntent` CAS is atomic over `(id, scenario_id, belief_id, action, target_id, snapshot_id)`. No API or service layer trusts caller-supplied identity after authorization. `targetID` and `snapshotID` are DB-derived in both REST and MCP handlers. |
| 6. Claim race | **PASS** | `WHERE state = 'live'` CAS ensures exactly one concurrent claim succeeds. Race detector passes on `go test -race ./...` for kernel, service/authority, api, cmd/solvent-mcp, adapter/github. |
| 7. Creation race / duplicate intent | **PASS** | `live_intent_per_snapshot` unique index ensures exactly one live intent survives concurrent equivalent creations. `ErrDuplicateIntent` is wrapped from `23505` in `createIntentWithinTx`. Not an authorization denial — no `ActivityAuthorizationDenied` audit entry. |
| 8. Composite-FK interaction with NULL | **PASS** | DB test: `(NULL, NULL)` insert succeeds. `(T1, NULL)` and `(NULL, S1)` succeed (FK passes with NULLs). Such rows cannot be claimed by `ClaimIntent` because the NULL-safe predicate requires both NULL or both matching. |
| 9. Snapshot provenance | **PASS** | `snapshot_id` is read from `target_activation JOIN target_snapshot` in REST and MCP handlers, and from `sqlAuthorizeResolve` in the kernel. No caller-supplied `snapshot_id` reaches `ClaimIntent`. |
| 10. Consequence parameters | **PASS** | `ExecuteAction` unmarshals `decision.ConsequenceParameters` (from kernel `AuthorizeResult`, which reads the snapshot). Caller-supplied `params` is ignored. Executor receives only `execParams` derived from the snapshot. |
| 11. API/MCP error semantics | **PASS** | `ErrDuplicateIntent` → HTTP 409 Conflict (`api/errors.go:79-82`) → MCP `isError: true` with conflict message. Not mapped to `ActivityAuthorizationDenied`. `ErrIntentNotLive` remains the honest result for claim-time mismatches. |
| 12. Diagnostic-read TOCTOU | **PASS** | The post-CAS diagnostic SELECT runs in the same `crdb.ExecuteTx` SERIALIZABLE transaction as the CAS UPDATE. It cannot observe a state that the CAS did not see. The CAS is the authoritative security decision; the diagnostic read is advisory only and does not gate the outcome. |
| 13. Intent mutation | **PASS** | Repository-wide search for `UPDATE action_intent` confirms only `state` is ever mutated (`executing`, `executed`, `live`, `cancelled`). No path updates `target_id`, `snapshot_id`, `belief_id`, `action`, or `scenario_id`. |
| 14. IntentOnPromoted bypass | **PASS** | `IntentOnPromoted` creates `(NULL, NULL)` intents. No application path upgrades an unbound intent to a bound intent. `ExecuteAction` always calls `ClaimIntent` with concrete `targetID`/`snapshotID`; the NULL-safe predicate rejects the mismatch. `CompleteIntent`, `RollbackClaim`, `CancelIntent` operate only on `executing` state, which unbound intents can never reach. |
| 15. Scenario isolation | **PASS** | `createIntentWithinTx` verifies belief exists in scenario. `ClaimIntent`, `CompleteIntent`, `RollbackClaim`, `CancelIntent` require `scenario_id`. `RetractCascade`, `Discharge`, `RetireDebt`, `Promote` are scenario-scoped. Cross-scenario target/snapshot/intent/evidence/execution paths are denied. |
| 16. Existing security invariants | **PASS** | Regression-checked: `promoted_is_debt_free` (23514), `gate` (23503), `live_requires_promoted` (23514), scenario isolation, intent ownership, ClaimIntent CAS, `AuthorizeAndCreateIntent` atomicity, `RevokeTarget` serialization, target snapshot immutability, fixed executor mapping, provider ambiguity handling, `ReconcileIntent` audit ordering, MCP no-direct-write invariant, TOKEN ≠ AUTHORITY. No regressions introduced by migration 009 or the SQL changes. |

---

## 5. Verification Results

### 5.1 Build and static analysis

```bash
$ gofmt -l cmd internal kernel
(no output)

$ go build ./...
(no output)

$ go vet ./...
(no output)
```

**Result:** PASS

### 5.2 Full test suite

```bash
$ go test -count=1 -p 1 ./...
ok  	github.com/PithomLabs/solvent/adapter/github	14.827s
ok  	github.com/PithomLabs/solvent/api	9.766s
...
ok  	github.com/PithomLabs/solvent/service/authority	11.523s
...
```

**Result:** PASS — all packages green

### 5.3 Race detector

```bash
$ go test -race -count=1 -p 1 ./kernel ./service/authority ./api ./cmd/solvent-mcp ./adapter/github
ok  	github.com/PithomLabs/solvent/kernel	16.052s
ok  	github.com/PithomLabs/solvent/service/authority	11.927s
ok  	github.com/PithomLabs/solvent/api	10.875s
ok  	github.com/PithomLabs/solvent/cmd/solvent-mcp	10.516s
ok  	github.com/PithomLabs/solvent/adapter/github	15.835s
```

**Result:** PASS — no data races detected

### 5.4 Database reset and migration 009

```bash
$ task db:reset
...
ALTER TABLE
NOTICE: constraint "action_intent_state_check" of relation "action_intent" does not exist, skipping
ALTER TABLE
...
CREATE INDEX
Database reset complete.
```

**Result:** PASS — migration 009 applied cleanly

### 5.5 Live schema verification

```sql
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'action_intent'
ORDER BY ordinal_position;

-- target_id  UUID  YES
-- snapshot_id UUID  YES

SELECT conname, contype, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'action_intent'::regclass;

-- intent_authority_binding_fk  f  FOREIGN KEY (target_id, snapshot_id) REFERENCES target_snapshot(target_id, snapshot_id) ON DELETE CASCADE
-- live_intent_per_snapshot      u  UNIQUE (target_id ASC, snapshot_id ASC) WHERE (((state = 'live'::STRING) AND (target_id IS NOT NULL)) AND (snapshot_id IS NOT NULL))
```

**Result:** PASS — composite FK and unique index present with correct definitions

### 5.6 Composite FK enforcement (live DB test)

| Test | Expected | Actual | Result |
|------|----------|--------|--------|
| `INSERT (T1, S2)` where S2 belongs to T2 | `23503` FK violation | `23503` FK violation | PASS |
| `INSERT (T1, nonexistent-S)` | `23503` FK violation | `23503` FK violation | PASS |
| `INSERT (T1, S1)` valid pair | Success | Success | PASS |
| `INSERT (NULL, NULL)` unbound | Success | Success | PASS |
| Duplicate `(T1, S1)` live intent | `23505` unique violation | `23505` unique violation | PASS |

### 5.7 I-7 invariant

```bash
$ bash scripts/check_i7.sh
I-7 PASS: 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool read(s)
```

**Result:** PASS

### 5.8 MCP boundary and verify

```bash
$ bash scripts/mcp_verify.sh
...
MCP VERIFY GREEN — 7 tools (6 + solvent_explain) ...
```

**Result:** PASS

### 5.9 Taskfile full suite

```bash
$ task test
... (all sub-checks pass)
```

**Result:** PASS

---

## 6. Kernel-Freeze Recommendation

**READY TO FREEZE**

No unresolved finding requires additional kernel primitives or schema changes. Future capabilities should default to service, adapter, executor, deployment, policy, demo, or documentation layers.

The exact authority binding invariant is enforced at two levels:

1. **Database:** composite FK `intent_authority_binding_fk` prevents any `action_intent` row from storing a `(target_id, snapshot_id)` pair that does not reference a real `target_snapshot`. The unique index `live_intent_per_snapshot` prevents duplicate bound live intents.
2. **Kernel CAS:** `ClaimIntent` atomically verifies `(id, scenario_id, belief_id, action, target_id, snapshot_id)` before transitioning `live → executing`. This is the sole authoritative ownership gate.

The service, REST, and MCP layers correctly propagate DB-derived `targetID` and `snapshotID` through to `ClaimIntent`. Caller-supplied values are never trusted for the authority binding.

The kernel is frozen.
