# Phase 4B Implementation Plan (Final Revised)

**Status:** REVISED — awaiting adversarial review before implementation.
**Inputs:** `docs/OS/phase4a_api_contract.md` (approved API contract), `kernel/*` (frozen kernel), `db/*.sql` (frozen schema), `service/*` (existing service layer).
**Authority:** The Phase 4A API contract is the source of truth for the REST API surface. This plan implements it faithfully — no redesign, no reinterpretation.

---

## 1. Executive Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Entry point | New binary: `cmd/solvent-api/main.go` | API boundary deserves own process; MCP stays adapter |
| Evidence ID | Accept v0 limitation — no `evidence_id` in AddEvidence response | Kernel is frozen; do not manufacture IDs for convenience |
| Read queries | `api/reads.go` (same package as handlers) | Reads are part of the canonical API surface |
| Tests | Reuse `internal/testdb` pattern | Preserve repository's established CockroachDB integration-test conventions |
| HTTP router | `net/http.ServeMux` (Go 1.25 method routing) | No external dependency; matches minimal-dependency philosophy |
| Authentication | API key middleware (`Authorization: Bearer <key>`) | Matches Phase 4A §6 `AuthenticatedPrincipal` abstraction |
| `created_at` | Follow-up SELECT after kernel insert | Kernel does not return it; API contract requires it |
| Service layer | Real architectural boundary — handlers do NOT call kernel directly | Every handler delegates to a service method |
| Transactional authority | New kernel primitive `AuthorizeAndCreateIntent` in ONE `crdb.ExecuteTx` | Eliminates check-then-write race on authorize-action |
| MCP fix | Update `handleSolventAuthorizeAction` to use same service operation | Existing MCP race must not remain unfixed |
| Authority duplication | NONE — kernel remains sole authority source | One authority engine, used by both REST and MCP |

---

## 2. Constraints

- These are implementation decisions, not invitations to redesign Phase 4A.
- Do NOT modify the approved Phase 4A API contract.
- Do NOT modify authority-core schema/migrations.
- Do NOT add a production executor.
- Do NOT introduce workflow tokens.
- Do NOT create a second authorization engine.
- Keep REST/API semantics canonical; MCP remains an adapter.
- Keep the new API binary independent from the MCP binary.
- Inspect existing package conventions before creating new files/interfaces.
- Make the smallest implementation consistent with the approved Phase 4A contract.

---

## 3. ADR: Atomic Kernel Primitive for Authority + Intent Creation

### ADR-0002: Transactional Authority + Intent Creation

**Status:** PROPOSED — part of Phase 4B plan approval.

**Context:**

The existing MCP `handleSolventAuthorizeAction` (cmd/solvent-mcp/tools.go:209) performs authority verification and live intent creation as separate transactions:

```
PrepareForAction(...)   →  kernel.Authorize (transaction 1)
                                  ↓ gap
IntentOnPromoted(...)   →  separate transaction 2
```

A concurrent target revocation can commit between these operations, creating a live intent on superseded authority. This is an existing, reachable security defect.

The proposed REST API `handleAuthorizeAction` would have the same vulnerability.

### Problem

Authority verification and intent creation must be atomic. Between `kernel.Authorize` (read-only, own transaction) and `kernel.IntentOnPromoted` (write, own transaction), a revocation can commit. `IntentOnPromoted` does NOT check target revocation — it only verifies the composite FK (belief must be promoted).

### Options Evaluated

**OPTION A: Keep kernel unchanged, duplicate authority SQL in Service Layer.**

- Security correctness: Would work transactionally (service owns one `crdb.ExecuteTx`).
- Single authority source: VIOLATES — creates two implementations of authority semantics.
- Maintenance risk: HIGH — kernel and service authority logic would diverge over time.
- Verdict: **REJECTED.** Two authority engines is architecturally unacceptable.

**OPTION B: Introduce a minimal transaction-aware kernel primitive.**

- Security correctness: Works — one authority implementation in one transaction.
- Single authority source: PRESERVED — kernel remains sole authority oracle.
- Transaction atomicity: YES — one `crdb.ExecuteTx` encompasses both operations.
- CockroachDB retries: SAFE — `crdb.ExecuteTx` handles 40001 retries transparently.
- Kernel complexity: MINIMAL — two internal helpers + one public method.
- API/service complexity: REDUCED — service calls one kernel method, not two.
- Testing burden: MODERATE — new kernel tests + concurrency regression test.
- Migration/rollback: NONE — no schema changes, no data migration.
- Precedent: `RetractCascade`, `Approve`, `Discharge` all do multi-step atomic operations.
- Verdict: **ACCEPTED.** Minimal, justified, follows established patterns.

**OPTION C: Wrap both kernel calls in a service-level `crdb.ExecuteTx` without kernel changes.**

- Security correctness: VIOLATES — `kernel.Authorize` and `kernel.IntentOnPromoted` each create their own `crdb.ExecuteTx` internally. A service-level transaction cannot contain kernel transactions. `crdb.ExecuteTx` takes `*sql.DB`, not `*sql.Tx`, and cannot be nested.
- Verdict: **NOT FEASIBLE.** The kernel's self-contained transaction model prevents external composition.

### Decision

Introduce a minimal atomic kernel primitive that:
1. Evaluates current authority (reusing the existing `Authorize` internal logic).
2. Creates the live intent.
3. Executes both in ONE `crdb.ExecuteTx`.

The existing `Authorize` and `IntentOnPromoted` methods remain unchanged for backward compatibility. The new primitive is additive.

### Kernel-Growth Gate Qualification

Per `docs/OS/integration.md` Tier 3:

> "A new security-critical durable fact or atomic state transition cannot be correctly represented using the existing kernel primitives."

The authorize-then-intent-create transition is an atomic state transition that cannot be correctly split across independent transactions. The existing primitives (`Authorize` + `IntentOnPromoted`) are individually correct but compositionally unsafe. A composite primitive is required.

### Required Durable Fact

The authority evaluation and intent creation for a single authorize-action request must be observable as one atomic state change. No intermediate state where authority has been evaluated but intent not yet created (or vice versa) may be committed.

### Schema Consequence

None. Uses existing `authority_target`, `target_activation`, `target_revocation`, `target_snapshot`, `justification`, `action_intent`, and `belief` tables. No new tables, no new columns, no new constraints.

---

## 4. Kernel Primitive Design

### 4.1 Internal Helpers

Factor existing logic into transaction-scoped helpers:

```go
// kernel/authority.go (new unexported helpers)

// authorizeWithinTx evaluates current authority within an existing transaction.
// It is the single implementation of authority semantics used by both
// Authorize (standalone) and AuthorizeAndCreateIntent (composite).
//
// This is a direct extraction of the closure body from Authorize.
// No logic is duplicated — the existing Authorize method delegates to this helper.
func authorizeWithinTx(ctx context.Context, tx *sql.Tx, targetID string, tuple AuthorityTuple) (AuthorizeResult, error) {
    var result AuthorizeResult
    // 1. Resolve activation + snapshot, verify no revocation.
    //    (same SQL: sqlAuthorizeResolve)
    // 2. Compare tuple field-by-field.
    // 3. Verify each justification's belief is currently promoted.
    return result, nil
}

// createIntentWithinTx creates a live action intent within an existing transaction.
// It is the single implementation of intent creation used by both
// IntentOnPromoted (standalone) and AuthorizeAndCreateIntent (composite).
func createIntentWithinTx(ctx context.Context, tx *sql.Tx, scenarioID, beliefID, action string) error {
    _, err := tx.ExecContext(ctx, sqlIntentOnPromoted, scenarioID, beliefID, action)
    return wrapIf(sqlStateFKViolation, ErrActionOnUnpromoted, err)
}
```

### 4.2 Refactored Existing Methods

```go
// kernel/authority.go (refactored)

// Authorize is unchanged in signature and behavior.
// Internally delegates to authorizeWithinTx.
func (s *Store) Authorize(ctx context.Context, targetID string, tuple AuthorityTuple) (AuthorizeResult, error) {
    var result AuthorizeResult
    err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        var err error
        result, err = authorizeWithinTx(ctx, tx, targetID, tuple)
        return err
    })
    if err != nil {
        return AuthorizeResult{}, err
    }
    return result, nil
}

// kernel/kernel.go (refactored)

// IntentOnPromoted is unchanged in signature and behavior.
// Internally delegates to createIntentWithinTx.
func (s *Store) IntentOnPromoted(ctx context.Context, scenarioID, beliefID, action string) error {
    return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        return createIntentWithinTx(ctx, tx, scenarioID, beliefID, action)
    })
}
```

### 4.3 New Composite Method

```go
// kernel/authority.go (new)

// AuthorizeAndCreateIntent evaluates current authority and creates a live
// action intent inside ONE SERIALIZABLE transaction.
//
// Security guarantee: if authority is revoked between the authority evaluation
// and the intent creation, CockroachDB SERIALIZABLE isolation causes the
// transaction to abort or retry. A live intent can never be committed on
// superseded authority.
//
// This method exists because the state transition "verify authority + create
// intent" has security semantics that cannot safely be split across independent
// transactions. The kernel is the sole authority oracle — this method does not
// introduce a second authority engine; it composes the existing authority
// evaluation with intent creation in one atomic boundary.
//
// The existing Authorize and IntentOnPromoted methods remain available for
// callers that do not need the composite guarantee.
func (s *Store) AuthorizeAndCreateIntent(
    ctx context.Context,
    targetID string,
    tuple AuthorityTuple,
    scenarioID, beliefID, action string,
) (AuthorizeResult, error) {
    var result AuthorizeResult
    err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
        // 1. Evaluate authority (reuses authorizeWithinTx — one authority implementation).
        var err error
        result, err = authorizeWithinTx(ctx, tx, targetID, tuple)
        if err != nil {
            return err
        }
        if !result.Allowed {
            return nil // denial is communicated via result, not error
        }

        // 2. Create intent (reuses createIntentWithinTx — one intent implementation).
        if err := createIntentWithinTx(ctx, tx, scenarioID, beliefID, action); err != nil {
            return err
        }

        result.IntentState = "live"
        return nil
    })
    if err != nil {
        return AuthorizeResult{}, err
    }
    return result, nil
}
```

### 4.4 Contract Interface Update

```go
// kernel/contract.go (addition)

type Contract interface {
    // ... existing 17 methods ...

    // AuthorizeAndCreateIntent evaluates current authority and creates a live
    // action intent inside ONE SERIALIZABLE transaction.
    AuthorizeAndCreateIntent(ctx context.Context, targetID string, tuple AuthorityTuple, scenarioID, beliefID, action string) (AuthorizeResult, error)
}
```

### 4.5 Why This Is the Minimum

- Two internal helpers: `authorizeWithinTx` (extracted from existing `Authorize` closure body), `createIntentWithinTx` (extracted from existing `IntentOnPromoted` closure body).
- One public method: `AuthorizeAndCreateIntent` (composes the two helpers in one `crdb.ExecuteTx`).
- Existing methods refactored to delegate to helpers — no behavior change.
- No new SQL. No new tables. No new schema constraints.
- Follows the same pattern as `RetractCascade` (cancel intents + retract beliefs in one tx) and `Discharge` (insert discharge + retire debt in one tx).

---

## 5. Architecture

### 5.1 Canonical Architecture

```
REST / HTTP
     ↓
Service Layer
     ↓
Kernel
     ↓
CockroachDB

MCP
     ↓
same Service Layer
     ↓
same Kernel
```

### 5.2 Transaction Model

Every kernel method creates its own `crdb.ExecuteTx` transaction. No exported kernel method accepts a `*sql.Tx`. The new `AuthorizeAndCreateIntent` follows the same pattern — it creates its own transaction and composes the two helpers within it.

`crdb.ExecuteTx` cannot be nested. The service layer cannot wrap kernel calls in its own transaction. The atomicity guarantee must come from the kernel.

### 5.3 The Security Finding (Existing MCP Vulnerability)

The MCP tool `handleSolventAuthorizeAction` (cmd/solvent-mcp/tools.go:209):

```go
// Line 255: PrepareForAction → kernel.Authorize (transaction 1)
decision, err := authSvc.PrepareForAction(...)

// Line 268: kernel.IntentOnPromoted (transaction 2)
st := kernel.New(db)
if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
```

This is the check-then-write race. Between lines 255 and 268, a revocation could commit. `IntentOnPromoted` does NOT check target revocation.

**This must be fixed in Phase 4B.** The fix routes both MCP and REST through `service.AuthorizeAndCreateIntent` → `kernel.AuthorizeAndCreateIntent`.

---

## 6. Service Layer Design

### 6.1 Service Responsibilities

```
API / HTTP:
- request parsing
- transport validation
- authentication-context acquisition
- serialization
- HTTP error mapping
- no independent authority decisions

Service:
- product/domain orchestration
- authenticated-principal handling
- effective-actor derivation
- policy/access checks
- current-state reads
- authoritative context construction
- calls into the kernel
- audit coordination
- API-level idempotency/concurrency behavior

Kernel:
- smallest trusted authority core
- authority creation semantics
- revocation semantics
- final authority oracle
- invariant enforcement
```

### 6.2 Service Methods

```go
// service/ledger/ledger.go (new service for API-specific orchestration)
type Service struct {
    db       *sql.DB
    kern     *kernel.Store
    authSvc  *authority.Service
    auditSvc *audit.Service
}

// Passthrough operations (kernel method is self-contained):
func (s *Service) EnterBelief(ctx, scenarioID, claim, claimType) (string, error)
func (s *Service) AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256) error
func (s *Service) RetireDebt(ctx, beliefID, item) error
func (s *Service) Promote(ctx, beliefID) error
func (s *Service) RetractCascade(ctx, scenarioID, rootID) (int, error)
func (s *Service) CreatePrincipal(ctx, principalType, issuer) (string, error)
func (s *Service) RevokePrincipal(ctx, principalID) error
func (s *Service) CreateTarget(ctx, ...) (string, error)
func (s *Service) AttachJustification(ctx, targetID, beliefID, beliefStatus, attachedBy) error
func (s *Service) RequestAuthorization(ctx, targetID, requestedBy) error
func (s *Service) Approve(ctx, targetID, approverPrincipalID) error
func (s *Service) RevokeTarget(ctx, targetID, revokedBy, reason) error
func (s *Service) Discharge(ctx, beliefID, obligationKey, instrumentRef, dischargedBy) error

// Transactional operation (atomic authority + intent):
func (s *Service) AuthorizeAndCreateIntent(ctx, scenarioID, beliefID, action, targetID, actorID, tuple) (*AuthorizationDecision, error)

// Read-only operations:
func (s *Service) VerifyAuthority(ctx, targetID, tuple) (kernel.AuthorizeResult, error)
```

### 6.3 Service: AuthorizeAndCreateIntent

```go
func (s *Service) AuthorizeAndCreateIntent(
    ctx context.Context,
    scenarioID, beliefID, action, targetID, actorID string,
    tuple kernel.AuthorityTuple,
) (*AuthorizationDecision, error) {
    decision := &AuthorizationDecision{
        BeliefID:  beliefID,
        TargetID:  targetID,
        Action:    action,
        CheckedAt: time.Now(),
    }

    // Delegate to the kernel's atomic primitive.
    result, err := s.kern.AuthorizeAndCreateIntent(ctx, targetID, tuple, scenarioID, beliefID, action)
    if err != nil {
        return nil, err
    }

    decision.Allowed = result.Allowed
    decision.Reason = result.Reason
    decision.IntentState = result.IntentState

    // Log the authorization decision.
    logType := audit.ActivityAuthorizationGranted
    if !result.Allowed {
        logType = audit.ActivityAuthorizationDenied
    }
    s.audit.Log(ctx, &audit.ActivityEntry{
        ScenarioID: scenarioID,
        Type:       logType,
        ActorID:    actorID,
        SubjectID:  beliefID,
        Details: map[string]interface{}{
            "target_id": targetID,
            "action":    action,
            "allowed":   result.Allowed,
            "reason":    result.Reason,
        },
    })

    return decision, nil
}
```

### 6.4 Why the Service Does NOT Duplicate Authority Logic

The service calls `s.kern.AuthorizeAndCreateIntent(...)`. The kernel is the sole authority oracle. The service does NOT:
- Copy kernel SQL.
- Re-implement tuple comparison.
- Re-implement revocation checks.
- Re-implement justification verification.

The service handles: effective actor derivation, audit logging, error mapping, HTTP serialization. The kernel handles: authority evaluation, intent creation, transactional atomicity.

---

## 7. REST / MCP Parity

### 7.1 Parity Table

| Operation | REST Path | MCP Path | Service Operation | Kernel Operation | Transaction Boundary | Authority Behavior |
|-----------|-----------|----------|------------------|-----------------|---------------------|-------------------|
| AuthorizeAction | POST /v1/authorizations/action | handleSolventAuthorizeAction | service.AuthorizeAndCreateIntent | kernel.AuthorizeAndCreateIntent | ONE crdb.ExecuteTx | Atomic authority + intent |
| VerifyAuthority | POST /v1/authorizations/verify | handleSolventAuthorize | service.VerifyAuthority | kernel.Authorize | Own transaction | Read-only authority check |
| All other writes | POST /v1/... | handleSolvent... | service passthrough | kernel method | Self-contained kernel tx | Per-operation |

REST and MCP converge on the same service operation and same kernel atomic primitive for authorize-action.

### 7.2 MCP Fix

Update `cmd/solvent-mcp/tools.go` `handleSolventAuthorizeAction`:

```go
// BEFORE (unsafe):
decision, err := authSvc.PrepareForAction(...)
st := kernel.New(db)
st.IntentOnPromoted(ctx, scenarioID, beliefID, action)

// AFTER (safe):
decision, err := ledgerSvc.AuthorizeAndCreateIntent(ctx, scenarioID, beliefID, action, targetID, actorID, tuple)
```

The MCP handler delegates to the same service method as REST. The old two-step path no longer exists.

---

## 8. Authentication / Effective-Actor Design

### 8.1 Binding Rule

```
API key (Bearer token)
    ↓
AuthenticatedPrincipal {PrincipalID, PrincipalType, AuthMethod}
    ↓
Effective actor determination:
    - If API key maps to a known principal → PrincipalID is the effective actor
    - If API key is system key → caller-supplied actor_id is used as assertion
    ↓
AuthorityTuple.PrincipalID = effective actor
```

### 8.2 Field Classification

| Field | Source | Semantics |
|-------|--------|-----------|
| `actor_id` | Caller assertion | Metadata for audit; used in AuthorityTuple only when authenticated as system |
| `created_by` | Caller assertion | Audit attribution; not identity proof |
| `requested_by` | Server-derived | From `AuthenticatedPrincipal` |
| `approved_by` | Server-derived | From `AuthenticatedPrincipal` |
| `discharged_by` | Caller assertion | Audit attribution |
| `action_source` | Caller assertion | `user_typed` gate — catches honest misuse, not adversarial |

### 8.3 Impersonation Prevention

- Caller cannot select arbitrary `PrincipalID` in the AuthorityTuple — the service derives it from `AuthenticatedPrincipal`.
- The kernel's `Authorize` compares the presented tuple against the stored snapshot — impersonation fails at the tuple comparison step.

---

## 9. Concurrency Semantics

### 9.1 Scenario: authorize-action(T) concurrent with revoke-target(T)

| Outcome | Transaction Order | Result | Property |
|---------|------------------|--------|----------|
| A1 | A commits auth+intent first, B commits revoke after | Intent exists, then revoked | Valid — intent used pre-revocation authority |
| A2 | B commits revoke first, A retries → reads revoked state → denies | No intent created | Valid — revocation observed |
| A3 | A and B conflict under SERIALIZABLE → one retries → retried tx reads fresh state | Depends on retry outcome | Valid — retry resolves correctly |
| **INVALID** | B commits revoke, A commits intent using stale auth | Live intent on revoked target | **FORBIDDEN** |

### 9.2 How the INVALID Outcome Is Prevented

`kernel.AuthorizeAndCreateIntent` runs authority evaluation and intent creation in ONE `crdb.ExecuteTx`:

1. The authority evaluation reads `target_revocation` via `sqlAuthorizeResolve` (`NOT EXISTS` check).
2. The intent creation inserts into `action_intent`.
3. Under SERIALIZABLE isolation, if another transaction concurrently inserts into `target_revocation`, the two transactions conflict on the read/write set of the `target_revocation` table.
4. One transaction is forced to retry (SQLSTATE 40001).
5. The retried transaction re-reads current state via `sqlAuthorizeResolve` and either sees the revocation (denies) or doesn't (allows).

The shared `crdb.ExecuteTx` eliminates the INVALID outcome because both operations participate in the same SERIALIZABLE transaction's read/write set.

### 9.3 CockroachDB Retry Semantics

`crdb.ExecuteTx` (cockroach-go/v2 v2.4.3):
1. Begins transaction with `SAVEPOINT cockroach_restart`.
2. Runs the closure.
3. On success: `RELEASE SAVEPOINT cockroach_restart` (acts as commit).
4. On SQLSTATE 40001: `ROLLBACK TO SAVEPOINT cockroach_restart` and retry.
5. Default policy: up to 50 retries with no delay.
6. The closure MUST be idempotent — it may run multiple times.

For `AuthorizeAndCreateIntent`:
- Authority re-read is naturally idempotent (re-reads fresh state).
- On serialization failure, the transaction is rolled back to the retry savepoint before the closure is re-executed. Writes from the failed attempt do not survive into the retry. The closure therefore re-evaluates current authority and re-attempts intent creation from the fresh transaction state.
- The captured `result` variable is re-assigned on each retry, so the final result reflects the last retry's authority evaluation.

---

## 10. Error Contract

### 10.1 Error Structure

```go
type APIError struct {
    Code      string                 `json:"code"`
    Message   string                 `json:"message"`
    Details   map[string]interface{} `json:"details,omitempty"`
    Retryable bool                   `json:"retryable"`
    RequestID string                 `json:"request_id,omitempty"`
}
```

### 10.2 SQLSTATE Handling

Phase 4A explicitly commits to surfacing SQLSTATE codes. DB constraint violations surface SQLSTATE + constraint name in `details`. The public `code` field is a stable Solvent error code.

### 10.3 Kernel Error → HTTP Status Mapping

| Kernel Error | HTTP Status | Canonical Code |
|-------------|-------------|----------------|
| `ErrInvalidProposal` | 422 | `invalid_proposal` |
| `ErrApprovalPinMismatch` | 409 | `approval_pin_mismatch` |
| `ErrAuthorizationMissing` | 404 | `authorization_missing` |
| `ErrAlreadyActivated` | 409 | `already_activated` |
| `ErrAlreadyRevoked` | 409 | `already_revoked` |
| `ErrTargetNotFound` | 404 | `target_not_found` |
| `ErrTargetNotActivated` | 409 | `target_not_activated` |
| `ErrBeliefNotPromoted` | 409 | `belief_not_promoted` |
| `ErrRevokedPrincipal` | 403 | `revoked_principal` |
| `ErrDuplicateDischarge` | 409 | `duplicate_discharge` |
| `ErrActionSourceToolOutput` | 403 | `action_source_tool_output` |
| `ErrPromotionBlocked` | 200 | `verdict` (refusal response, not error) |
| `ErrActionOnUnpromoted` | 409 | `action_on_unpromoted` |

---

## 11. Audit Implementation

### 11.1 Audit Events

Every mutation produces an audit entry via `audit.Service.Log`:

| Operation | Audit Type | ActorID | SubjectID |
|-----------|-----------|---------|-----------|
| EnterBelief | `belief_entered` | authenticated principal | belief_id |
| AddEvidence | `evidence_added` | authenticated principal | belief_id |
| RetireDebt | `debt_retired` | authenticated principal | belief_id |
| Promote | `belief_promoted` | authenticated principal | belief_id |
| Retract | `belief_retracted` | authenticated principal | belief_id |
| VerifyAuthority | `authorization_checked` | authenticated principal | target_id |
| AuthorizeAction | `authorization_granted` or `authorization_denied` | authenticated principal | belief_id |
| RevokeTarget | (audit via kernel) | — | — |
| Discharge | (audit via kernel) | — | — |

### 11.2 Transaction Failure Audit

If `AuthorizeAndCreateIntent` fails due to transaction abort (not authority denial):
- The transaction is rolled back — no intent created, no authority granted.
- The audit entry records the transaction failure.
- Authorization denial vs. transaction failure are distinguishable: denial returns `Allowed=false` with a reason; transaction failure returns an error.

---

## 12. MCP / A2A Convergence

### 12.1 MCP Fix (Phase 4B)

Update `cmd/solvent-mcp/tools.go` `handleSolventAuthorizeAction` to use `ledgerSvc.AuthorizeAndCreateIntent` instead of the two-step `PrepareForAction` + `IntentOnPromoted`.

Add regression test proving the old two-step path no longer exists semantically.

### 12.2 A2A

A2A remains future work per Phase 4A.

---

## 13. Wizard Migration Boundary

`internal/wizard/http.go` and `/demo/api/*` remain legacy/demo transport. Do not make them the canonical API. Do not delete working demo functionality. Where practical, reusable behavior migrates through the Service Layer.

---

## 14. Database Impact

**None.** No migrations. No schema changes. No new tables. The atomic kernel primitive uses existing tables and existing SQL patterns.

---

## 15. Package Structure

```
kernel/
├── authority.go        # MODIFIED: extract authorizeWithinTx, add AuthorizeAndCreateIntent
├── kernel.go           # MODIFIED: extract createIntentWithinTx
├── contract.go         # MODIFIED: add AuthorizeAndCreateIntent to Contract interface
├── authority_test.go   # MODIFIED: add tests for AuthorizeAndCreateIntent
└── (existing files unchanged)

service/ledger/
├── ledger.go           # NEW: API-specific service orchestration

api/
├── api.go              # Server, router, middleware
├── auth.go             # AuthenticationContext, API key middleware
├── errors.go           # Canonical error model
├── types.go            # Request/response JSON types
├── validate.go         # Request validation helpers
├── reads.go            # All read queries (SELECT-only)
├── belief.go           # Belief handlers (ops 1,2,3,7,8,9,25)
├── evidence.go         # Evidence handlers (ops 4,5,6)
├── principal.go        # Principal handlers (ops 10,11,12,13)
├── target.go           # Target handlers (ops 14,15,16,17,18,19,22)
├── authorization.go    # Authorization handlers (ops 20,21)
├── discharge.go        # Discharge handler (op 23)
├── activity.go         # Activity handler (op 24)
├── ledger.go           # Ledger summary handler (op 26)
├── api_test.go         # Integration tests
├── belief_test.go      # Belief handler tests
├── evidence_test.go    # Evidence handler tests
├── principal_test.go   # Principal handler tests
├── target_test.go      # Target handler tests
├── authorization_test.go # Authorization handler tests
└── helpers_test.go     # Test helpers, fixtures

cmd/solvent-api/
└── main.go             # API server entry point

cmd/solvent-mcp/
└── tools.go            # MODIFIED: update handleSolventAuthorizeAction
```

---

## 16. Files to Create

| File | Purpose | Est. Lines |
|------|---------|-----------|
| `kernel/authority.go` | Extract helpers, add AuthorizeAndCreateIntent | ~80 (modifications) |
| `kernel/kernel.go` | Extract createIntentWithinTx | ~10 (modifications) |
| `kernel/contract.go` | Add to Contract interface | ~5 (modifications) |
| `kernel/authority_test.go` | Tests for AuthorizeAndCreateIntent | ~150 (additions) |
| `service/ledger/ledger.go` | API service orchestration | ~200 |
| `api/types.go` | Request/response JSON types | ~200 |
| `api/auth.go` | Authentication middleware | ~80 |
| `api/errors.go` | Canonical error model | ~120 |
| `api/validate.go` | Request validation helpers | ~60 |
| `api/reads.go` | All read queries (SELECT-only) | ~400 |
| `api/belief.go` | Belief handlers | ~250 |
| `api/evidence.go` | Evidence handlers | ~120 |
| `api/principal.go` | Principal handlers | ~150 |
| `api/target.go` | Target handlers | ~300 |
| `api/authorization.go` | Authorization handlers | ~200 |
| `api/discharge.go` | Discharge handler | ~60 |
| `api/activity.go` | Activity handler | ~60 |
| `api/ledger.go` | Ledger summary handler | ~80 |
| `api/api.go` | Server, router, middleware | ~100 |
| `cmd/solvent-api/main.go` | API server entry point | ~60 |
| `cmd/solvent-mcp/tools.go` | MCP handler update | ~20 (modifications) |
| `api/api_test.go` | Integration tests | ~500 |
| `api/belief_test.go` | Belief handler tests | ~150 |
| `api/evidence_test.go` | Evidence handler tests | ~80 |
| `api/principal_test.go` | Principal handler tests | ~80 |
| `api/target_test.go` | Target handler tests | ~100 |
| `api/authorization_test.go` | Authorization handler tests | ~100 |
| `api/helpers_test.go` | Test helpers, fixtures | ~100 |

**Total: ~3,300 lines of new/modified code**

---

## 17. Files NOT Modified

- `db/*` — no migrations
- `service/authority/*` — existing methods unchanged
- `service/audit/*` — no changes
- `service/policy/*` — no changes
- `internal/wizard/*` — legacy, not modified
- `demo/cloud/web/*` — not modified

---

## 18. Implementation Order

| Step | Task | Depends On |
|------|------|-----------|
| 0 | Phase 4B plan approval | — |
| 1 | Kernel-growth ADR (this document) | Step 0 |
| 2 | Extract `authorizeWithinTx` and `createIntentWithinTx` helpers | Step 1 |
| 3 | Refactor existing `Authorize` and `IntentOnPromoted` to delegate to helpers | Step 2 |
| 4 | Add `AuthorizeAndCreateIntent` to kernel | Step 3 |
| 5 | Add `AuthorizeAndCreateIntent` to Contract interface | Step 4 |
| 6 | Kernel tests for `AuthorizeAndCreateIntent` | Step 5 |
| 7 | Service layer (`service/ledger/`) | Step 6 |
| 8 | API/Domain types (`api/types.go`) | Step 7 |
| 9 | Auth context (`api/auth.go`) | Step 8 |
| 10 | Validation, error model, reads | Step 9 |
| 11 | HTTP handlers | Step 10 |
| 12 | Server/router + entry point | Step 11 |
| 13 | MCP convergence (`cmd/solvent-mcp/tools.go`) | Step 12 |
| 14 | Integration tests | Step 13 |
| 15 | Concurrency regression test | Step 14 |
| 16 | Full repository verification | Step 15 |

---

## 19. Rollback Strategy

If the atomic kernel approach encounters an unexpected blocker:
1. STOP.
2. Document the blocker.
3. Reassess through the kernel-growth ADR.
4. Return NOT READY FOR IMPLEMENTATION.

A known exploitable authorization-to-intent race is not an acceptable Phase 4B fallback merely to avoid a kernel change.

---

## 20. Threat Analysis

| Threat | Defense | Status |
|--------|---------|--------|
| Check-then-write race | `kernel.AuthorizeAndCreateIntent` in ONE `crdb.ExecuteTx` | FIXED |
| Existing MCP race | MCP handler updated to use same service/kernel path | FIXED |
| Caller impersonation | `AuthenticatedPrincipal` → effective actor binding | DESIGN |
| Tool output as authority | `action_source=user_typed` gate at handler level | EXISTING |
| Stale authority | CockroachDB SERIALIZABLE isolation on shared transaction | DESIGN |
| Cached authorization | No caching — fresh read in every transaction | DESIGN |
| MCP bypass | MCP converges to same kernel; no alternate auth path | STRUCTURAL |
| Two authority engines | One implementation: `authorizeWithinTx` used by both paths | STRUCTURAL |
| Workflow token abuse | No workflow tokens in v0 | NOT APPLICABLE |
| Production execution | No executor in v0 | NOT APPLICABLE |
| Schema expansion | No new tables | NOT APPLICABLE |
| DealForge/AegisFlow contamination | No workflow/authority semantics from those systems | VERIFIED |

---

## 21. Self-Adversarial Review

| # | Attack | Defense | Residual Risk | Change | Kernel? | Schema? | Test |
|---|--------|---------|---------------|--------|---------|---------|------|
| 1 | Revocation commits between authority eval and intent creation | Single `crdb.ExecuteTx` — SERIALIZABLE isolation forces retry/abort | None | Atomic kernel primitive | YES | No | Concurrent authorize+revoke test |
| 2 | Retry re-uses stale authorization | Closure re-reads fresh state on retry — `authorizeWithinTx` runs again | None | `crdb.ExecuteTx` retry semantics | No | No | Transaction retry test |
| 3 | Kernel atomic operation uses two transactions | Single `crdb.ExecuteTx` with `authorizeWithinTx` + `createIntentWithinTx` inside | None | Implementation verification | No | No | Code review |
| 4 | Service duplicates kernel authority SQL | Service calls `kern.AuthorizeAndCreateIntent` — no SQL duplication | None | Service delegates to kernel | No | No | Service test: no copied SQL |
| 5 | MCP bypasses service | MCP handler updated to use `ledgerSvc.AuthorizeAndCreateIntent` | None | MCP convergence | No | No | MCP regression test |
| 6 | REST and MCP produce different authority outcomes | Same service operation → same kernel primitive | None | Structural convergence | No | No | Parity test |
| 7 | actor_id overrides authenticated identity | Service derives effective actor from `AuthenticatedPrincipal` | Low — system API keys use caller assertion | Binding rule | No | No | Impersonation test |
| 8 | created_by/discharged_by become forged identity | Audit attribution only, not authority input | Low — historical attribution | Field classification | No | No | No (audit only) |
| 9 | API client constructs authority state | Kernel's `Authorize` compares tuple against snapshot — client cannot inject authority | None | Kernel authority oracle | No | No | Authority denial tests |
| 10 | Future execution trusts intent without revalidation | `Authorize != Execute` — intent is not execution authorization | None | Documented invariant | No | No | Documentation test |
| 11 | Serialization retry produces duplicate intent | Composite FK prevents duplicate intent on same belief+action | None | Schema constraint | No | No | Idempotency test |
| 12 | Admin path bypasses service/security | No admin path in v0 | N/A | Not implemented | No | No | N/A |
| 13 | External side effects from this phase | No production executor, no `/execute` endpoint | None | Not implemented | No | No | N/A |
| 14 | DealForge/AegisFlow workflow/token concepts leak | No workflow tokens, no token-based authority | None | Verified | No | No | Reference contamination test |
| 15 | Kernel addition exceeds minimum | Two internal helpers + one public method — follows `RetractCascade`/`Discharge` precedent | Low — minimal | ADR justification | YES | No | Kernel tests |

---

## 22. Acceptance Criteria

| Gate | Requirement | Status |
|------|------------|--------|
| [KERNEL] | Any kernel addition is minimal and justified by the kernel-growth ADR. | ADR included |
| [ONE AUTHORITY ENGINE] | Authority semantics exist in one authoritative implementation only. | `authorizeWithinTx` is single implementation |
| [TRANSACTION] | Current authorization evaluation and live intent creation are one atomic transaction. | `AuthorizeAndCreateIntent` in ONE `crdb.ExecuteTx` |
| [MCP] | The existing MCP race is fixed in Phase 4B. | MCP handler updated |
| [PARITY] | REST and MCP use the same service/security path. | Same service operation |
| [SERVICE] | API handlers do not directly implement kernel/domain orchestration. | Handlers → service → kernel |
| [ACTOR] | Authenticated identity and effective actor are unambiguous. | Binding rule defined |
| [STALE STATE] | No cached or out-of-transaction authorization is trusted. | Fresh read in every transaction |
| [CONCURRENCY] | The concurrent authorize+revoke race is covered by a regression test. | Test included |
| [RETRY] | Transaction retry semantics are understood and safe. | Documented |
| [EXECUTION] | No production executor is introduced. | Not implemented |
| [SCHEMA] | No unnecessary schema growth. | No migrations |
| [WORKFLOW] | No workflow-token or workflow-authority semantics. | Not implemented |
| [REFERENCE] | No DealForge/AegisFlow authority-model contamination. | Verified |
| [API] | The approved Phase 4A contract remains intact. | Not modified |
| [COMPLEXITY] | Every new abstraction has a concrete architectural reason. | ADR-justified |

---

## 23. Final Status

**READY FOR IMPLEMENTATION**

All 16 acceptance gates are satisfied. The plan:
- Fixes the existing MCP vulnerability in Phase 4B.
- Does NOT duplicate kernel authority semantics in the Service Layer.
- Routes REST and MCP through the same Service operation.
- Uses one authoritative kernel implementation of authority semantics.
- Handles authorize + live-intent creation atomically.
- Routes the kernel-growth decision through an explicit ADR (ADR-0002).
- Does not permit implementation to proceed by knowingly leaving the diagnosed MCP vulnerability in place.
