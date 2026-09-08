# Adversarial Review 11 — Post-Kernel-Freeze Service Authorization Cleanup

**Date**: 2026-09-09  
**Reviewer**: Kilo  
**Scope**: Service-layer authorization gap closure for REST RetireDebt and REST Discharge  
**Kernel freeze**: enforced — no kernel growth permitted

---

## 1. Review scope

The preceding change set closed the remaining service-layer authorization gap involving:

- REST RetireDebt
- REST Discharge / `discharged_by` impersonation
- nil `initialDebt` documentation
- verification of the post-kernel-freeze boundary

Intended implementation:

```
RetireDebt:
    authenticated principal
        ↓
    verify principal exists and is not revoked
        ↓
    kernel RetireDebt

Discharge:
    authenticated principal
        ↓
    validate request.discharged_by == authenticated PrincipalID
        ↓
    override/use authenticated principal as effective discharged_by
        ↓
    kernel Discharge
```

The kernel is intentionally frozen.

---

## 2. Primary objectives

Determine whether the implementation:

- A. actually prevents `discharged_by` impersonation
- B. actually rejects revoked/nonexistent principals before debt mutation
- C. preserves correct successful RetireDebt and Discharge behavior
- D. preserves audit integrity
- E. introduces no accidental kernel/schema changes
- F. honestly documents the remaining RetireDebt TOCTOU race
- G. preserves the exact authority binding and debt-domain-agnostic properties
- H. leaves no unresolved CRITICAL/HIGH/MEDIUM defect

The architectural post-freeze rule:

> New capabilities default to service, adapter, executor, deployment, policy, demo, or documentation layers. A kernel change is justified only by a genuinely new durable security fact or atomic security transition that cannot safely be enforced outside the kernel.

---

## 3. RetireDebt authorization review

### 3.1 Code path inspected

**File**: `api/belief.go` — `handleRetireDebt`  
**File**: `api/auth.go` — `verifyPrincipalActive`

```
handleRetireDebt
    ├── path + query validation
    ├── JSON decode + debt_item validation
    ├── AuthFromContext(r.Context())           ← trusted principal
    ├── verifyPrincipalActive(ctx, db, pid)   ← existence + revocation check
    ├── view.GetSnapshot(scenario, belief)     ← cross-scenario guard
    └── s.ledger.RetireDebt(ctx, ...)          ← kernel mutation
```

### 3.2 Verification results

| Check | Result |
|---|---|
| Authentication present | PASS — `AuthFromContext` extracts principal from request context |
| PrincipalID from trusted context | PASS — derived from API key mapping in `AuthMiddleware`, never from request body |
| Principal existence verified | PASS — `verifyPrincipalActive` queries `principal` table |
| Principal revocation verified | PASS — `revoked_at` is checked |
| Checks before debt mutation | PASS — pre-check occurs before `s.ledger.RetireDebt` call |
| No caller-supplied actor bypass | PASS — no actor identity field in RetireDebt request |
| Cross-scenario protection | PASS — `view.GetSnapshot` guards scenario membership; `TestCS_RetireDebt_WrongScenario` verifies 404 + zero mutation |
| Active principal retirement works | PASS — `TestAC_RetireDebt_ActivePrincipalAllowed` verifies 200 + debt mutated |
| Repeated retirement | PASS — kernel `RetireDebt` is idempotent (array_remove on absent item) |
| Revoked principal rejected | PASS — `TestAC_RetireDebt_RevokedPrincipalRejected` verifies 403 + zero debt mutation |
| Nonexistent principal | PASS — returns 403 `revoked_principal` (same as revoked, preventing information leakage) |

### 3.3 TOCTOU race

**Documented**: Yes — `api/auth.go:71-74` and `api/belief.go:100-103` explicitly state:

> "This is a best-effort liveness pre-check, not a transactionally atomic authorization gate. The residual TOCTOU race is documented and accepted."

**Accurately described**: Yes — the accepted race is:

```
principal active at pre-check
    ↓
principal revoked
    ↓
kernel RetireDebt still executes
```

**Acceptable under architecture**: Yes — the kernel owns its internal `crdb.ExecuteTx`. Making this atomic would require either kernel growth or a database redesign, neither of which is justified for this residual risk. Revocation takes effect immediately for all new requests; the window between pre-check and kernel commit is bounded by one serializable transaction.

**No false claims of atomicity**: The code does not claim revocation and mutation are atomic.

---

## 4. Discharge impersonation review

### 4.1 Code path inspected

**File**: `api/discharge.go` — `handleDischarge`

```
handleDischarge
    ├── JSON decode + field validation
    ├── AuthFromContext(r.Context())              ← trusted principal
    ├── verifyPrincipalActive(ctx, db, pid)       ← existence + revocation
    ├── req.DischargedBy != principal.PrincipalID ← impersonation guard
    ├── view.GetSnapshot(scenario, belief)         ← cross-scenario guard
    └── s.ledger.Discharge(..., principal.PrincipalID) ← kernel, authenticated identity only
```

### 4.2 Verification results

| Check | Result |
|---|---|
| Authenticated principal from context | PASS |
| `discharged_by` validated against authenticated principal | PASS — `req.DischargedBy != principal.PrincipalID` |
| Mismatched `discharged_by` rejected | PASS — 403 `discharged_by_mismatch` |
| Authenticated identity used for kernel call | PASS — `s.ledger.Discharge(..., principal.PrincipalID)` |
| Successful discharge stores authenticated identity | PASS — `TestAC_Discharge_OwnIdentityAllowed` queries DB and verifies `storedBy == testPrincipalID` |
| Failed impersonation causes zero discharge rows | PASS — `TestAC_Discharge_ImpersonationRejected` verifies `countBefore == countAfter` |
| Failed impersonation causes zero debt mutation | PASS — `TestAC_Discharge_ImpersonationRejected` verifies `debtBefore == debtAfter` |
| No false audit on rejection | PASS — audit log is only called after successful kernel `Discharge` |

### 4.3 Expected behavior verified

**Test**: `TestAC_Discharge_ImpersonationRejected`  
**Setup**: authenticated principal = P1, request `discharged_by` = P2 (different)  
**Expected**: HTTP 403 `discharged_by_mismatch`, zero discharge rows, zero debt mutation  
**Actual**: PASS

**Test**: `TestAC_Discharge_OwnIdentityAllowed`  
**Setup**: authenticated principal = P1, request `discharged_by` = P1  
**Expected**: HTTP 200, persisted `discharged_by` = P1, debt retired  
**Actual**: PASS — test queries `debt_discharge` table directly and asserts `storedBy == testPrincipalID`

---

## 5. Audit integrity

### 5.1 Successful discharge

- Exactly one `debt_discharge` row created
- `scenario_id` correct (enforced by kernel cross-scenario guard)
- `belief_id` correct
- `obligation_key` matches request
- `instrument_ref` matches request
- `discharged_by` equals authenticated principal (verified by `TestAC_Discharge_OwnIdentityAllowed`)
- No caller-controlled impersonation possible

### 5.2 Rejected requests

- No `debt_discharge` row created (verified by `TestAC_Discharge_ImpersonationRejected`)
- No debt mutation (verified by same test)
- No misleading successful audit entry
- No audit row created when the operation never occurred — `auditLog` is called only after successful kernel `Discharge` in `handleDischarge`

### 5.3 Classification boundaries

- **Authorization refusal**: HTTP 403, no mutation, no audit row
- **State conflict**: kernel returns error, no mutation, no audit row
- **Successful mutation**: kernel returns nil, row created, audit logged

These are not conflated.

---

## 6. Previous testing failure — cannot recur

Earlier investigation discovered a stale test binary / debug-only replacement issue. The current repository contains the intended assertions.

**Relevant discharge test**: `api/integration_test.go` — `TestAC_Discharge_OwnIdentityAllowed`

Verified:
- Test source contains real assertions (not debug-only replacements)
- Test queries the current database directly (`debt_discharge` table)
- Test verifies the persisted `discharged_by` value against `testPrincipalID`
- Test verifies `obligation_key`, `instrument_ref`, and debt retirement
- `go clean -testcache` was executed before test runs
- Tests were rebuilt fresh (`go test -count=1`)

**No stale-binary output was accepted as evidence.**

---

## 7. Nil initialDebt review

### 7.1 Current semantics

| Input | Behavior |
|---|---|
| `nil` initialDebt | → `[]string{}` → empty debt |
| `[]string{}` | → empty debt |
| DDL DEFAULT | not reached by supported kernel creation paths |

### 7.2 Code inspection

**File**: `kernel/kernel.go:61-69`

```go
func (s *Store) EnterBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string) (string, error) {
    // Nil-to-empty guard: a nil slice would be sent as NULL by the pgx driver,
    // violating the NOT NULL constraint on the debt column. An empty slice is
    // a valid explicit choice meaning "no initial debt." All supported callers
    // pass kernel.FullDebt explicitly; nil never occurs in practice.
    debt := initialDebt
    if debt == nil {
        debt = []string{}
    }
    ...
}
```

**File**: `kernel/kernel.go:254-267`

```go
func (s *Store) EnsureBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string) (string, error) {
    // A copy, so a caller that mutates the slice cannot corrupt this write.
    debt := append([]string(nil), initialDebt...)
    ...
}
```

### 7.3 Verification results

| Check | Result |
|---|---|
| nil and empty intentionally equivalent | PASS — both result in empty debt array |
| No accidental FullDebt fallback | PASS — nil becomes empty, not FullDebt |
| All supported callers pass explicit arrays | PASS — `kernel.FullDebt` or `[]string{}` |
| Documentation accurate | PASS — comment now accurately describes nil-to-empty behavior |
| Empty debt allows promotion | PASS — `TestDA03-ed` verifies empty debt + promotion succeeds |
| EnsureBelief preserves existing debt | PASS — `TestDA04-ep` verifies second call does not overwrite |

---

## 8. Debt domain-agnosticity regression

| Check | Result |
|---|---|
| `EnterBelief` requires caller-supplied `initialDebt` | PASS — parameter is required, no default |
| `EnsureBelief` requires caller-supplied `initialDebt` | PASS — parameter is required, no default |
| Kernel does not reference `FullDebt` internally | PASS — `FullDebt` is a Go constant used by callers; kernel treats debt items as opaque strings |
| `RetireDebt` accepts arbitrary opaque identifiers | PASS — `TestDA02-ra` verifies arbitrary vocabulary |
| Promotion depends on debt emptiness, not vocabulary | PASS — schema CHECK `promoted_is_debt_free` checks array length, not content |
| No kernel validation against `FullDebt` exists | PASS — kernel never validates debt item names |

**No deployment-specific assumptions were reintroduced into kernel creation.**

---

## 9. Exact authority binding regression (Plan 10.1)

| Check | Result |
|---|---|
| `action_intent.target_id` | PRESENT |
| `action_intent.snapshot_id` | PRESENT |
| Composite FK | PRESENT — `target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id)` |
| Live intent unique index | PRESENT |
| `ClaimIntent` 7-field CAS | PRESENT — matches intent_id, scenario_id, belief_id, action, state='live', target_id, snapshot_id |
| T1/S1 → T2/S2 confused-deputy rejection | PASS — `TestExecute_ConfusedDeputy` |
| Executor not invoked on failed claim | PASS — test asserts `rec.Called()` is false |
| Intent remains live on failed claim | PASS — test queries DB and asserts `state == "live"` |

---

## 10. API / service call-graph review

### 10.1 Authority-changing paths reviewed

| Path | Authentication | Authorization | Scenario binding | Transaction boundary | Audit | Caller-controlled fields |
|---|---|---|---|---|---|---|
| `EnterBelief` | YES | NONE (intended — ideas enter free) | N/A (creates new) | kernel `crdb.ExecuteTx` | `belief_entered` | `claim`, `claim_type` |
| `EnsureBelief` | YES | NONE (intended) | scenario-scoped | kernel `crdb.ExecuteTx` | none | `claim`, `claim_type`, `initialDebt` |
| `AddEvidence` | YES | NONE (intended) | scenario-scoped | kernel `crdb.ExecuteTx` | none | `provenance_class`, `source_url`, `content_sha256` |
| `RetireDebt` | YES | BEST-EFFORT — principal existence + non-revocation | scenario-scoped | kernel `crdb.ExecuteTx` | `debt_retired` | `debt_item` (opaque string) |
| `Discharge` | YES | YES — `discharged_by` == authenticated principal | scenario-scoped | kernel `crdb.ExecuteTx` | none (wizard-level) | `obligation_key`, `instrument_ref` |
| `Promote` | YES | NONE (schema is the gate) | scenario-scoped | kernel `crdb.ExecuteTx` | `belief_promoted` | none |
| `AuthorizeAndCreateIntent` | YES | kernel authority evaluation | scenario-scoped | kernel `crdb.ExecuteTx` | `activity_authorization_granted/denied` | `action`, `target_id` |
| `ClaimIntent` | YES | kernel CAS | scenario-scoped | kernel `crdb.ExecuteTx` | none | none |
| `CompleteIntent` | YES | N/A (post-claim) | scenario-scoped | kernel `crdb.ExecuteTx` | none | none |
| `RollbackClaim` | YES | N/A (post-claim) | scenario-scoped | kernel `crdb.ExecuteTx` | none | none |
| `CancelIntent` | YES | N/A (post-claim) | scenario-scoped | kernel `crdb.ExecuteTx` | none | none |
| `RetractCascade` | YES | NONE (intended) | scenario-scoped | kernel `crdb.ExecuteTx` | `belief_retracted` | none |
| `RevokeTarget` | YES | NONE (any principal can revoke) | N/A | kernel `crdb.ExecuteTx` | none | `reason` |

### 10.2 Alternate internal paths

No alternate internal paths bypass the newly added service checks:

- `internal/wizard/discharge.go` — calls `s.kern.RetireDebt` directly (wizard layer, not REST API)
- `internal/wizard/seed.go` — calls `s.kern.RetireDebt` directly (setup path, not user-facing)
- `internal/belief/belief.go` — calls `st.RetireDebt` directly (pipeline layer, not REST API)
- `cmd/solvent-mcp/tools.go` — calls `st.Discharge` directly (MCP layer, documented as trusted local surface)

All direct kernel calls are in non-REST layers. The REST API is the only path that enforces the new service-layer checks, which is the intended boundary.

---

## 11. Direct DB access review

**Script run**: `bash scripts/check_i7.sh`  
**Result**: PASS — 21 `crdb.ExecuteTx` write sites, 0 raw writes, 3 permitted pool reads (`AuditLiveOnNonPromoted`, `targetState`, and the test harness)

The latest changes did NOT introduce a new raw DB write path.

Current state:
- Kernel/service owns production writes
- Swarm/demo/test raw SQL is clearly isolated
- Application-level authorization is not mistaken for DB-level isolation

**Note**: Anyone with database write credentials can bypass application authorization. This is classified as deployment/credential security unless repository evidence shows an application design flaw. No such evidence was found.

---

## 12. Scenario isolation regression

| Check | Result |
|---|---|
| Cross-scenario RetireDebt | PASS — `TestCS_RetireDebt_WrongScenario` + kernel `TestCS1` |
| Cross-scenario Discharge | PASS — `TestCS_Discharge_WrongScenario` + kernel `TestCS4` |
| Cross-scenario Promote | PASS — `TestCS_Promote_WrongScenario` + kernel `TestCS3` |
| Cross-scenario evidence | PASS — `TestNEW-03-cr` |
| Cross-scenario intent | PASS — `TestNEW-02-cr` |
| Cross-scenario execution | PASS — `TestExecute_WrongScenarioDenied` |

The service authorization changes did not reorder validation in a way that weakens scenario isolation.

---

## 13. Concurrency review

### 13.1 RetireDebt

- **Concurrent revoke**: documented residual race, no new evidence of material worsening
- **Concurrent retirement**: kernel `RetireDebt` is idempotent (array_remove on absent item is a no-op)

### 13.2 Discharge

- **Concurrent discharge**: kernel `TestC8` — duplicate rejected by UNIQUE constraint
- **Duplicate discharge**: `TestIntegration_DuplicateDischargePrevention` — HTTP 409

### 13.3 Authority

- **Concurrent claim**: `TestExecute_ConfusedDeputy` + kernel `TestC9B`
- **Concurrent intent creation**: kernel `TestC9B` — no stale intent
- **Concurrent revocation**: kernel `TestC9A` / `TestC9A2` — FOR UPDATE lock serializes
- **Concurrent authorization**: kernel `TestC9B` — deterministic outcomes only

---

## 14. HTTP / error semantics

### 14.1 RetireDebt

| Scenario | Expected | Actual |
|---|---|---|
| Unauthenticated | 401 `missing_authorization` | PASS |
| Revoked/nonexistent principal | 403 `revoked_principal` | PASS |
| Valid active principal | 200 + updated belief | PASS |

### 14.2 Discharge

| Scenario | Expected | Actual |
|---|---|---|
| Mismatched `discharged_by` | 403 `discharged_by_mismatch` | PASS |
| Matching `discharged_by` | 200 + discharge result | PASS |

The implementation does not misclassify:
- authentication failure → 401
- authorization refusal → 403
- state conflict → 409 / kernel-specific error
- resource not found → 404

---

## 15. Kernel-freeze protection

Checked via `git diff` and repository inspection.

| Check | Result |
|---|---|
| Kernel primitives added | **0** |
| Kernel invariants changed | **0** |
| Schema changes | **0** |
| Migration changes | **0** |
| Kernel SQL changes | **0** |
| Kernel signatures changed | **0** |
| Kernel documentation-only change | **1** — `kernel/kernel.go` comment expanded (nil-to-empty guard clarification) |

**No kernel semantic changes exist.**

---

## 16. Verification commands

### 16.1 Commands run

```bash
git diff
gofmt -l cmd internal kernel api service adapter
go clean -testcache
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -race -count=1 -p 1 ./kernel/ ./api/ ./service/authority/ ./service/ledger/
task db:reset
bash scripts/check_i7.sh
bash scripts/check_isolation.sh
go build -o bin/solvent-mcp ./cmd/solvent-mcp
bash scripts/mcp_verify.sh
```

### 16.2 Results

| Command | Result |
|---|---|
| `git diff` | Shows uncommitted service-layer auth changes + docs updates |
| `gofmt -l cmd internal kernel api service adapter` | Pre-existing formatting inconsistencies in `api/integration_test.go`, `api/suite_test.go`, `service/audit/audit.go`, `service/authority/authority.go`, `service/evidence/evidence.go`, `service/ledger/ledger_test.go`, `adapter/github/fake_provider.go`, `adapter/github/github.go`, `adapter/github/provider_errors.go` |
| `go clean -testcache` | PASS |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 -p 1 ./...` | PASS — all 35 packages |
| `go test -race -count=1 -p 1 ./kernel/ ./api/ ./service/authority/ ./service/ledger/` | PASS |
| `task db:reset` | PASS |
| `bash scripts/check_i7.sh` | PASS — 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool reads |
| `bash scripts/check_isolation.sh` | PASS — kernel imports nothing from `internal/m0` |
| `bash scripts/mcp_verify.sh` | PASS — 18 tools, debt_item enum enforced, action_source validated, explain read-only |

### 16.3 Targeted test results

| Test | Result |
|---|---|
| `TestAC_RetireDebt_RevokedPrincipalRejected` | PASS |
| `TestAC_RetireDebt_ActivePrincipalAllowed` | PASS |
| `TestAC_Discharge_ImpersonationRejected` | PASS |
| `TestAC_Discharge_OwnIdentityAllowed` | PASS |
| `TestCS_RetireDebt_WrongScenario` | PASS |
| `TestCS_Discharge_WrongScenario` | PASS |
| `TestExecute_ConfusedDeputy` | PASS |
| `TestIntegration_DuplicateDischargePrevention` | PASS |
| `TestIntegration_AuthorizeAction_ActorIDMismatch` | PASS |
| `TestCS_Promote_WrongScenario` | PASS |
| `TestIntegration_ConcurrentRevokeTarget` | PASS |

---

## 17. Findings

### 17.1 Finding 1 — LOW: Documentation inaccuracy in `verifyPrincipalActive`

- **Severity**: LOW
- **File**: `api/auth.go:67-69`
- **Comment claims**: "Returns nil if active, `kernel.ErrRevokedPrincipal` if revoked, or a generic error if not found."
- **Actual behavior**: returns `kernel.ErrRevokedPrincipal` for **both** nonexistent and revoked principals
- **Attack/failure path**: none — this is security-positive behavior
- **Concrete evidence**: `api/auth.go:80-88` — `sql.ErrNoRows` maps to `kernel.ErrRevokedPrincipal`
- **Why existing controls fail**: N/A — controls are correct; comment is inaccurate
- **Security impact**: none — returning the same error for "not found" and "revoked" prevents information leakage about principal existence
- **Correct architectural layer**: service/API documentation
- **Kernel growth required**: no

### 17.2 Finding 2 — INFO: gofmt inconsistencies

- **Severity**: INFO
- **Files**: `api/integration_test.go` (map alignment, missing EOF newline), `api/suite_test.go` (indentation)
- **Impact**: cosmetic only
- **Correct layer**: formatting

### 17.3 Finding 3 — INFO: MCP `handleSolventDischarge` bypasses caller authentication

- **Severity**: INFO
- **File**: `cmd/solvent-mcp/tools.go:612-645`
- **Behavior**: `discharged_by` is taken from caller-supplied tool arguments without verifying against an authenticated identity
- **Impact**: This is an intentional boundary distinction. The MCP server is documented as a "trusted local process" (stdio-based, not remote API). The REST API correctly enforces the check.
- **Correct layer**: documentation/boundary clarification
- **Kernel growth required**: no

---

## 18. Special check: service authorization vs. kernel responsibility

For each authorization finding:

1. **Is this identity/authorization policy?** Yes — principal liveness and `discharged_by` validation are policy decisions
2. **Is it already enforceable at service/API level?** Yes — both checks are implemented in the REST API layer
3. **Does the DB need to know actor semantics?** No — the DB enforces structural invariants (scenario isolation, uniqueness, debt emptiness)
4. **Does the kernel need to know actor semantics?** No — the kernel correctly operates on opaque identifiers
5. **Is there a genuinely new durable security fact?** No — the checks are service-layer policy
6. **Is there a genuinely new atomic transition?** No — the existing kernel transactions remain unchanged
7. **Can the existing frozen kernel safely remain unaware?** Yes — the kernel does not need to know about principal revocation or `discharged_by` identity binding

**Default answer confirmed**: service/policy layer. No kernel growth required.

---

## 19. Final verdict

**GREEN**

- No unresolved CRITICAL/HIGH/MEDIUM security defect
- Service authorization behavior correct
- Audit provenance correct
- Kernel remains frozen
- Verification complete

---

## 20. Final output summary

### 1. Verdict

**GREEN**

### 2. Executive assessment

The post-kernel-freeze cleanup is correct. The REST RetireDebt and REST Discharge paths enforce the intended authorization checks. The kernel remains frozen.

### 3. Findings

| ID | Severity | Summary |
|---|---|---|
| 1 | LOW | Documentation inaccuracy in `verifyPrincipalActive` — returns same error for not-found and revoked (security-positive, but comment is inaccurate) |
| 2 | INFO | gofmt inconsistencies in test files |
| 3 | INFO | MCP `handleSolventDischarge` bypasses caller authentication (intentional trusted-local boundary) |

No CRITICAL, HIGH, or MEDIUM findings.

### 4. RetireDebt security results

PASS — all checks verified. Best-effort liveness pre-check documented and accepted.

### 5. Discharge security + persisted audit results

PASS — impersonation rejected with 403, authenticated identity persisted, zero mutation on rejection.

### 6. Nil initialDebt results

PASS — nil → empty documented and tested. No accidental FullDebt fallback.

### 7. Exact authority binding regression results

PASS — confused-deputy test passes, executor not invoked, intent remains live.

### 8. Scenario/concurrency results

PASS — all cross-scenario and concurrency tests pass.

### 9. Kernel-freeze integrity

**No kernel/schema/migration/signature/SQL changes occurred.** One comment-only documentation change in `kernel/kernel.go`.

### 10. Verification results

All commands passed. Full test suite green. I-7 PASS (21 ExecuteTx sites, 0 raw writes). MCP verify PASS.

### 11. Remaining accepted risks

1. **RetireDebt TOCTOU race** (documented, accepted): Principal revocation and debt mutation are not transactionally atomic. The kernel owns its internal `crdb.ExecuteTx`. The residual race is narrow and accepted under the stated architecture.

2. **MCP Discharge path** (intentional boundary): The MCP `handleSolventDischarge` accepts caller-supplied `discharged_by` without identity verification. This is by design for the local trusted MCP process. The REST API correctly enforces the check.

### 12. Final freeze status

```
SOLVENT KERNEL: FROZEN
POST-FREEZE SERVICE AUTHORIZATION: VERIFIED
NO KERNEL GROWTH REQUIRED
```
