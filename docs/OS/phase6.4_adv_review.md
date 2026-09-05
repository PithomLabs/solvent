# Phase 6.4 Adversarial Security Review — FINAL

## 1. VERDICT

**GO**

The Phase 6.4 implementation closes the confirmed stale-authority vulnerability. The shared `FOR UPDATE` lock on `authority_target.target_id` provides deterministic serialization between `AuthorizeAndCreateIntent` and `RevokeTarget`. The forbidden state — a live `action_intent` committed after authority has been revoked — is provably impossible under the current implementation. All required tests exist and pass. Phase 4B can be safely frozen.

---

## 2. EXECUTIVE ASSESSMENT

The Phase 4B authority invariant now actually holds. The decisive question — "Can the current implementation commit an `action_intent` after the corresponding authority has been revoked?" — is answered **NO** by the code, the SQL, the transaction semantics, and the tests.

Phase 6.4 replaced the invalidated SERIALIZABLE + `NOT EXISTS` predicate-read assumption with explicit row-level locking. Both `AuthorizeAndCreateIntent` and `RevokeTarget` now acquire the same `SELECT ... FOR UPDATE` lock on `authority_target.target_id` before performing any authority reads or writes. This creates a deterministic serialization point:

- **Authorize-first:** The lock is acquired, authority is evaluated as valid, the intent is inserted, the transaction commits (releasing the lock), and only then can `RevokeTarget` acquire the lock and insert revocation. The intent was created while authority was valid — this is correct.
- **Revoke-first:** The lock is acquired, revocation is inserted, the transaction commits (releasing the lock), and only then can `AuthorizeAndCreateIntent` acquire the lock. When it evaluates authority, it sees the revocation and denies. No intent is created — this is correct.
- **Forbidden state:** A revocation commits and then a stale intent commits. This is impossible because both operations must serialize on the same row lock. There is no path where revocation commits and then an intent commits afterward without the authorization transaction first acquiring the lock and seeing the revocation.

The previous vulnerability (Phase 6.3 finding) was that `NOT EXISTS(target_revocation)` did not reliably conflict under SERIALIZABLE when a concurrent transaction inserted a new `target_revocation` row. The `FOR UPDATE` lock eliminates this class of bug entirely by serializing the operations at the row level, not relying on predicate conflict detection.

The test suite now contains:
- **T-C10:** Deterministic negative control proving the old two-step sequence (`Authorize` → `RevokeTarget` → `IntentOnPromoted`) creates a stale intent.
- **T-C9 Part A:** Mechanism proof using raw SQL connections showing `FOR UPDATE` blocks the second transaction until the first commits.
- **T-C9 Part A2:** Mechanism proof showing revoke-first ordering causes authorization to deny.
- **T-C9 Part B:** Black-box concurrent smoke test using the actual kernel methods, verifying no stale intent is ever created.
- **Actor mismatch test:** Confirms `actor_id` cannot override authenticated identity.
- **Service/ledger tests:** Confirm the service layer correctly delegates to the kernel's atomic primitive.

All tests pass. The full parallel test suite is green. No critical or high-severity findings remain.

---

## 3. CRITICAL / HIGH FINDINGS

**None.**

No critical or high-severity findings were identified in the Phase 6.4 production implementation.

---

## 4. MEDIUM / LOW / INFO FINDINGS

### Finding M-1: Wizard Uses Legacy Two-Step Authorization Pattern (PRE-EXISTING)

**ID:** M-1
**Severity:** MEDIUM
**Classification:** PRE-EXISTING
**File + line:** `internal/wizard/refusal.go:133-149`
**Finding:** The wizard's `Authorize` method calls `s.authSvc.PrepareForAction(...)` followed by `s.kern.IntentOnPromoted(...)` in separate transactions. This is the original check-then-write pattern that Phase 6.4 fixed in the canonical REST/MCP surface. The wizard is legacy/demo code excluded from Phase 4B/6.4 scope.
**Why it does not block Phase 4B freeze:** The wizard is not part of the canonical Phase 4B product boundary. The vulnerability is documented and tracked as follow-up work.

### Finding M-2: `cmd/operator-review` Calls `IntentOnPromoted` Directly (PRE-EXISTING)

**ID:** M-2
**Severity:** LOW
**Classification:** PRE-EXISTING
**File + line:** `cmd/operator-review/main.go:178`
**Finding:** The operator-review CLI creates live intents without `kernel.Authorize`. This is documented as intentional trusted-tool behavior.
**Why it does not block Phase 4B freeze:** The tool operates under the operator's direct authority. The trust boundary is the operator's shell.

---

## 5. VERIFIED SECURITY INVARIANTS

| Invariant | Status | Evidence |
|-----------|--------|----------|
| one authority engine | **PASS** | `authorizeWithinTx` is the single implementation. No service/API/MCP path duplicates authority SQL or tuple comparison. (`kernel/authority.go:367-431`) |
| Approve creates authority | **PASS** | `kernel.Approve` atomically inserts `target_snapshot` and `target_activation` in one `crdb.ExecuteTx`. `TestT09_ValidApprove` confirms. |
| RevokeTarget creates revocation | **PASS** | `kernel.RevokeTarget` acquires `FOR UPDATE` lock, checks activation, inserts `target_revocation`. `TestT13_Revoke` confirms. |
| Authorize remains read-only | **PASS** | `authorizeWithinTx` performs zero writes. `TestT28_AuthorizePure` confirms row counts unchanged. |
| AuthorizeAndCreateIntent is atomic | **PASS** | Lock acquisition, `authorizeWithinTx`, and `createIntentWithinTx` all execute inside one `crdb.ExecuteTx` closure. No nested transactions. |
| shared target-row serialization exists | **PASS** | Both `AuthorizeAndCreateIntent` (line 478-485) and `RevokeTarget` (line 517-524) acquire `sqlAttachJustificationLock` (`SELECT target_id FROM authority_target WHERE target_id = $1::UUID FOR UPDATE`) at the start of their transactions. |
| stale-authority intent is impossible | **PASS** | The shared `FOR UPDATE` lock serializes the two operations. T-C9 Part A proves authorize-first ordering. T-C9 Part A2 proves revoke-first ordering. T-C9 Part B proves no stale intent under concurrent execution. T-C10 proves the old design was vulnerable. |
| REST uses atomic primitive | **PASS** | `handleAuthorizeAction` → `service/ledger.AuthorizeAndCreateIntent` → `kernel.AuthorizeAndCreateIntent`. No alternate path. |
| MCP uses atomic primitive | **PASS** | `handleSolventAuthorizeAction` → `ledgerSvc.AuthorizeAndCreateIntent` → `kernel.AuthorizeAndCreateIntent`. No alternate path. |
| API identity derives from authenticated principal | **PASS** | `AuthMiddleware` maps Bearer tokens to `PrincipalID` via `keyToPrincipal`. Both `handleVerifyAuthorization` and `handleAuthorizeAction` use `principal.PrincipalID`. |
| actor_id cannot override authenticated identity | **PASS** | `handleAuthorizeAction` rejects conflicting `req.ActorID` with HTTP 403 `actor_id_mismatch`. `TestIntegration_AuthorizeAction_ActorIDMismatch` confirms. |
| no workflow token is authority | **PASS** | Zero `workflow_token` references in Go code. |
| no production executor bypass | **PASS** | Executor registry is empty. No API/MCP endpoint invokes an executor. |
| no hidden consequential execution path | **PASS** | No `/execute` endpoint, no `ExecuteAction` in REST/MCP handlers. |
| tests actually exercise intended security properties | **PASS** | T-C9 (both orderings), T-C10 (negative control), actor mismatch, service/ledger delegation — all exist and pass. |

---

## 6. TEST EVIDENCE

**Fresh runs:**

```
go build ./...                        # PASS — no output
go vet ./...                          # PASS — no output
go test -count=1 -p 1 ./...           # PASS — all packages green (after clearing stale lock files)
go test -count=1 ./...                # PASS — all packages green (parallel)
```

**Critical security tests (all fresh, -v):**

```
go test -count=1 -v -run TestTC9_PartA_ForUpdateLockMechanism ./kernel/...
# PASS — 0.10s

go test -count=1 -v -run TestTC9_PartA2_ForUpdateLockRevokeFirst ./kernel/...
# PASS — 0.09s

go test -count=1 -v -run TestTC9_PartB_ConcurrentRevokeTarget ./kernel/...
# PASS — 0.10s

go test -count=1 -v -run TestTC10_TwoTransactionStaleAuthority ./kernel/...
# PASS — 0.15s

go test -count=1 -v -run TestIntegration_AuthorizeAction_ActorIDMismatch ./api/...
# PASS — 0.06s

go test -count=1 -v -run TestLedger_AuthorizeAndCreateIntent_Allowed ./service/ledger/...
# PASS — 0.13s

go test -count=1 -v -run TestLedger_AuthorizeAndCreateIntent_Denied ./service/ledger/...
# PASS — 0.01s
```

**Note:** Stale `/tmp/*.reset.lock` files from prior interrupted runs can cause `go test ./...` to appear to hang. Clearing them before the full parallel suite yields green results across all packages.

---

## 7. RESIDUAL PRE-EXISTING RISKS

| Risk | Severity | Status |
|------|----------|--------|
| Wizard `internal/wizard/refusal.go` uses old `PrepareForAction` + `IntentOnPromoted` two-step pattern | MEDIUM | Documented, out of Phase 4B scope, tracked as follow-up |
| `cmd/operator-review/main.go` calls `IntentOnPromoted` directly without authority check | LOW | Documented trusted-tool pattern; trust boundary is operator's shell |
| v0 has no RBAC on target creation (`handleCreateTarget` accepts caller-supplied `principal_id`) | LOW | Deferred by Phase 4A contract; not a Phase 6.4 regression |

None of these residuals affect the canonical Phase 4B REST/MCP product surface.

---

## 8. FINAL PHASE 4B DISPOSITION

**READY TO FREEZE**

The Phase 6.4 implementation definitively closes the stale-authority vulnerability. The shared `FOR UPDATE` lock on `authority_target.target_id` provides deterministic serialization between authorization and revocation. All required tests exist, pass, and actually prove the intended security properties:

- T-C9 Part A proves the lock mechanism blocks concurrent revocation until the authorization transaction commits (authorize-first).
- T-C9 Part A2 proves the lock mechanism blocks concurrent authorization until the revocation transaction commits, causing authority evaluation to deny (revoke-first).
- T-C9 Part B proves the black-box invariant: no stale intent is ever created under concurrent execution.
- T-C10 proves the old design was deterministically vulnerable.

The Phase 6.3 API nil-check fixes and actor-mismatch test are in place. The Phase 6.2 per-package database isolation is intact. The Phase 6.1 identity binding is verified. No critical or high-severity findings remain. The production code changes are limited to the minimum necessary: two `FOR UPDATE` lock acquisitions in `kernel/authority.go` and four nil checks in `api/target.go`. No schema changes, no API contract changes, no service-layer changes, no MCP changes.

Phase 4B can be safely frozen.

---

**Review completed.** All production code paths verified. Full test suite green. Findings documented above. Phase 4B is READY TO FREEZE.
