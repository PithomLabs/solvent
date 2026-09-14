# Plan: Post-Kernel-Freeze Service Authorization Cleanup

**Scope:** Close the REST RetireDebt/Discharge authorization gap. Service/API layer only. No kernel changes.

---

## Current state (evidence)

### Authentication
- `AuthMiddleware` validates Bearer tokens against a static `keyToPrincipal` map
- Sets `*AuthenticatedPrincipal{PrincipalID, PrincipalType, AuthMethod, AuthTime}` on context
- All routes go through this middleware — no unauthenticated requests reach handlers

### Authorization enforcement (existing pattern)
Only 3 handlers enforce principal identity:
- `handleAuthorizeAction` — derives `effectiveActor` from API key, rejects `actor_id` mismatch with HTTP 403
- `handleExecuteAction` — derives principal from API key
- `handleVerifyAuthorization` — uses principal in authority tuple

### RetireDebt (the gap)
- `handleRetireDebt` (`api/belief.go:98`) uses `AuthFromContext` only for audit logging
- No authorization gate: any authenticated principal can retire debt on any belief
- Cross-scenario guard exists (via `view.GetSnapshot`) but no identity check

### Discharge (the gap)
- `handleDischarge` (`api/discharge.go`) does NOT call `AuthFromContext` at all
- `discharged_by` is caller-supplied from request body, not verified against authenticated principal
- An impersonator can attribute a discharge to any principal

### Belief ownership
- The `belief` table has NO `created_by`, `owner`, or `principal_id` column
- Beliefs belong to scenarios, not principals
- There is no ownership structure to restrict RetireDebt by belief owner

### Nil initialDebt — verified behavior at HEAD

| Input | Go code | SQL received | Column value |
|-------|---------|-------------|--------------|
| `nil` | `debt = []string{}` (nil guard at `kernel.go:65`) | `$4::STRING[]` = empty array | `ARRAY[]::STRING[]` |
| `[]string{}` | `debt = []string{}` (no-op) | `$4::STRING[]` = empty array | `ARRAY[]::STRING[]` |
| DDL DEFAULT | (never reached — INSERT always includes debt column) | N/A | N/A |

All callers pass `kernel.FullDebt` explicitly. Nil never occurs in practice. Behavior is deliberate and correct. No change needed.

---

## Design: Narrowest defensible service-layer access control

### RetireDebt access control

**Constraint:** Beliefs have no owner. Any authenticated principal could legitimately need to retire debt on any belief (e.g., operator reviewing beliefs created by others).

**Narrowest defensible rule:**
1. Verify `AuthFromContext` returns non-nil (defense-in-depth, though middleware guarantees this)
2. Verify the principal exists in the `principal` table and is not revoked
3. If either check fails, reject with appropriate HTTP status before any debt mutation

**Why this is sufficient:** The principal table is the existing identity registry. Revoked principals should not perform any writes. This matches the existing `ErrRevokedPrincipal` → HTTP 403 pattern already in the error mapping.

**TOCTOU race — documented and accepted:**

The kernel's `RetireDebt` opens its own `crdb.ExecuteTx` internally (`kernel/kernel.go:109`). The service layer does not have access to `crdb.ExecuteTx` (it is a kernel-internal dependency from `github.com/cockroachdb/cockroach-go/v2/crdb`). There is no way to perform a principal-active check inside the kernel's transaction without modifying the kernel.

Therefore the principal-active check is a **best-effort liveness pre-check**, not a transactionally atomic authorization gate. The residual race is:

```
Thread A: verifyPrincipalActive("P1") → OK
Thread B: RevokePrincipal("P1") → committed
Thread A: kernel.RetireDebt(...) → succeeds (kernel does not check principal)
```

**Mitigations:**
- Revocation takes effect immediately for all new requests (middleware validates on every request)
- The race window is tiny (between the pre-check and the kernel's transaction begin)
- The principal table is the identity registry; revocation is the correct control for compromised keys
- This is the standard pattern in systems where the kernel does not enforce actor identity

**What we intentionally do NOT do:**
- Do NOT restrict by belief owner (beliefs have no owner)
- Do NOT add RBAC or policy checks
- Do NOT add new kernel semantics
- Do NOT claim the pre-check is transactionally atomic (it is not)

### Discharge access control

**Rule (following the `actor_id_mismatch` pattern):**
1. Extract authenticated principal via `AuthFromContext`
2. If `discharged_by` in the request body differs from the authenticated `PrincipalID`, reject with HTTP 403 and code `discharged_by_mismatch`
3. Use the authenticated principal as the effective `discharged_by`, ignoring the caller-supplied value

**Why:** The `discharged_by` column is `$4::UUID` in the kernel SQL — it records who performed the discharge. Letting callers attribute discharges to arbitrary principals is an impersonation vector. The authenticated identity is the only trustworthy source.

**HTTP error semantics:** HTTP 403 Forbidden with code `discharged_by_mismatch`, matching the existing `actor_id_mismatch` convention.

---

## Implementation steps

### Step 1: Add principal existence/revocation check helper

**File:** `api/auth.go`

Add a helper function that verifies the authenticated principal exists and is not revoked:

```go
func (s *Server) verifyPrincipalActive(ctx context.Context, principalID string) error {
    // Query principal table: exists + not revoked
    // Return nil if active, specific error if not found or revoked
}
```

This reuses the existing `principal` table and `ErrRevokedPrincipal` sentinel.

### Step 2: Add access control to handleRetireDebt

**File:** `api/belief.go`

Before the cross-scenario guard, add:
1. Extract `AuthFromContext`
2. Call `verifyPrincipalActive(ctx, principal.PrincipalID)`
3. On failure: return HTTP 403 with appropriate code

Document that this is best-effort liveness validation with a residual TOCTOU race.

### Step 3: Add access control to handleDischarge

**File:** `api/discharge.go`

Before the cross-scenario guard, add:
1. Extract `AuthFromContext`
2. Call `verifyPrincipalActive(ctx, principal.PrincipalID)`
3. Validate `req.DischargedBy == principal.PrincipalID`
4. On mismatch: return HTTP 403 with code `discharged_by_mismatch`
5. Override `req.DischargedBy` with `principal.PrincipalID` (defense-in-depth)

### Step 4: Add error code for discharged_by_mismatch

**File:** `api/errors.go`

Add `discharged_by_mismatch` as a recognized error code (HTTP 403), following the `actor_id_mismatch` pattern.

### Step 5: Tests for RetireDebt access control

**File:** `api/integration_test.go`

1. **Happy path:** Authenticated authorized principal retires debt → 200, debt mutated
2. **Revoked principal:** Create principal, revoke it, attempt RetireDebt → 403, zero debt mutation
3. **Cross-scenario (existing):** Already tested, ensure no regression
4. **No mutation on denial:** Verify belief debt unchanged after 403

### Step 6: Tests for Discharge access control

**File:** `api/integration_test.go`

1. **Happy path:** Authenticated caller uses own identity as `discharged_by` → 200
2. **Impersonation rejected:** Caller supplies different `discharged_by` → 403, zero discharge rows, zero debt mutation
3. **Effective actor override:** Verify the stored `discharged_by` matches the authenticated principal, not the caller-supplied value
4. **Cross-scenario (existing):** Already tested, ensure no regression
5. **No mutation on denial:** Verify zero new `debt_discharge` rows and belief debt unchanged

### Step 7: Nil initialDebt — document only

**File:** `kernel/kernel.go` (comment only)

Add a comment to the nil guard documenting the verified semantics:
- `nil` → treated as empty `[]string{}` → belief created with no debt items
- DDL DEFAULT is never reached (INSERT always includes debt column)
- All supported callers pass `kernel.FullDebt` explicitly
- This is deliberate; the nil guard prevents NULL constraint violations

No code change needed.

### Step 8: Documentation update

**File:** `docs/OS/adv_review10_debt_vocabulary.md`

Update the trust boundary section to note:
- RetireDebt and Discharge now verify principal identity at the service boundary
- `discharged_by` is authenticated/validated, not caller-trusted
- This is a service-layer security control, not a kernel change
- The RetireDebt check is best-effort liveness validation (document TOCTOU)

### Step 9: Full verification

Run all verification commands.

---

## Files changed

| File | Reason |
|------|--------|
| `api/auth.go` | Add `verifyPrincipalActive` helper |
| `api/belief.go` | Add access control gate to `handleRetireDebt` |
| `api/discharge.go` | Add access control gate + `discharged_by` validation to `handleDischarge` |
| `api/errors.go` | Add `discharged_by_mismatch` error code |
| `api/integration_test.go` | Tests for RetireDebt/Discharge access control |
| `docs/OS/adv_review10_debt_vocabulary.md` | Update trust boundary documentation |
| `kernel/kernel.go` | Comment-only: document nil initialDebt semantics |

**No schema changes. No migration. No new kernel primitives. No new tables.**

---

## Kernel-freeze protection

| Check | Expected |
|-------|----------|
| Kernel primitives added | 0 |
| Kernel invariants changed | 0 |
| Schema changes | 0 |
| Migration changes | 0 |
| Kernel signatures changed | 0 |
| Kernel SQL changed | 0 |

---

## Acceptance criteria

- [ ] RetireDebt: revoked principal rejected with 403, zero debt mutation
- [ ] RetireDebt: active principal allowed, debt mutated
- [ ] RetireDebt: TOCTOU race documented as best-effort liveness validation
- [ ] Discharge: impersonation rejected with 403, zero discharge rows, zero debt mutation
- [ ] Discharge: own identity allowed, stored `discharged_by` matches authenticated principal
- [ ] No audit events recorded for rejected requests (no false "debt_retired" or "discharged" events)
- [ ] Existing tests pass (cross-scenario, duplicate discharge, kernel debt lifecycle)
- [ ] No kernel changes
- [ ] No schema changes
- [ ] Full test suite green
- [ ] Race tests green
- [ ] I-7 and MCP verification green
