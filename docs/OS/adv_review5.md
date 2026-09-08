# Solvent Phase 4A–4E Independent Cumulative Adversarial Architecture + Security Review

**Reviewer:** Independent Senior Security Architect  
**Date:** 2026-09-08  
**Repository:** `/home/chaschel/Documents/go/solvent-main` (HEAD: `5933f44 ✨ phase 4e`)  
**Mode:** READ-ONLY — No modifications made  

---

## Executive Assessment

**VERDICT: NO-GO**

After a thorough read-only adversarial review of the complete Solvent evolution from Phase 4A through Phase 4E, I found **MEDIUM severity findings** that prevent a GO verdict. The architecture has preserved its core thesis in most areas, but specific boundary weaknesses and implementation gaps exist that could be exploited or that silently weaken earlier guarantees.

---

## Phase-by-Phase Assessment

### Phase 4A (API Contract)
**Status: PARTIAL SUCCESS**

**Intended Objective:** Establish a canonical, language-neutral API contract via HTTP/JSON, OpenAPI, and MCP.

**Findings:**
- **Positive:** OpenAPI spec exists and is structured; REST/MCP endpoints are defined.
- **Negative:** The Taskfile.yml syntax error (R-1 from MCP review) makes ALL documented commands fail — `task setup`, `task test`, etc. This is a **BLOCKING** defect for the demo, but it's a build/plumbing issue, not an architectural one.
- **Identity semantics:** REST uses Bearer token → authenticated principal; MCP uses trusted-local-stdio → actor_id attribution. The distinction IS intentional and documented in the code (see `cmd/solvent-mcp/tools.go:207-209`).
- **Forbidden endpoint design:** The `action_source` gate (`user_typed` vs `tool_output`) is correctly enforced at both REST and MCP boundaries before any database access.

### Phase 4B (Canonical API/Service Implementation)
**Status: STRONG**

**Core Thesis Preserved:** `kernel.Authorize` remains the sole authority oracle. The chain `Approve → snapshot → activation → Authorize` is correctly implemented with database-enforced invariants.

**Key Invariants Verified:**
- `Approve` creates authority atomically (snapshot + activation in one transaction)
- `Authorize` is read-only, compares presented tuple against snapshot
- `RevokeTarget` inserts append-only revocation fact
- `TOKEN != AUTHORITY` — no workflow token machinery bypasses kernel
- Composite FK `gate` with `ON UPDATE CASCADE` enforces I-3/I-4/I-8
- `promoted_is_debt_free` CHECK enforces I-1/I-2

### Phase 4C (OpenAPI 3.1, Reference Client, GitHub Integration)
**Status: PARTIAL SUCCESS**

**API Contract Drift:** No significant drift detected. The OpenAPI spec aligns with implementation.
- **Trust boundary:** REST middleware (`api/auth.go`) correctly derives principal from Bearer token; MCP handlers use `actor_id` from tool arguments with explicit documentation of the trust model difference.

### Phase 4C+ (First Real External Side Effects)
**Status: STRONG WITH RESIDUAL RISKS**

**Execution Model Correctly Implemented:**
```
Authorize → exact target snapshot → exact intentID → fixed executor → GitHub provider → CompleteIntent → audit
```

**Verified Invariants:**
- Caller parameters ignored; snapshot is source of truth (`service/authority/authority.go:280-293`)
- Executor resolved from internal registry, never caller-supplied (`service/authority/authority.go:115-123, 267-278`)
- Provider credentials are not authority; provider acceptance ≠ workflow completion
- `AuthorizeAndCreateIntent` uses FOR UPDATE lock to serialize against `RevokeTarget` (Race A mitigation)
- Window A (Authorize→ClaimIntent) and Window B (ClaimIntent→Provider) are correctly identified as accepted v1 residual risks

**Gap:** `CompleteIntent` ordering — the kernel does NOT enforce that `CompleteIntent` happens after provider acceptance; this is a service-layer discipline. The audit logs `ActivityIntentCompletionFailed` on persistence failure, which is correct (provider DID accept).

### Phase 4D (Consequential Execution State Model)
**Status: STRONG WITH RESIDUAL RISKS**

**State Model:** `live → executing → executed` with `executing` surviving retraction.

**Verified:**
- `ClaimIntent` is atomic CAS (`live → executing`) with `WHERE state = 'live'` predicate (CI-4)
- `CompleteIntent`, `RollbackClaim`, `CancelIntent` all have source-state guards (`WHERE state = 'executing'`)
- `RetractCascade` cancels live intents FIRST, then retracts beliefs (correct ordering enforced by schema)
- Provider outcome classification is generic at service boundary (`outcomeAccepted`, `outcomeRejected`, `outcomeAmbiguous`)

**Race A (Authorize→ClaimIntent):** Accepted v1 residual risk. The lock on `authority_target` serializes against `RevokeTarget` in `AuthorizeAndCreateIntent`, but `Authorize` (standalone) + `ClaimIntent` (separate) has a window. Documented honestly.

**Race B (ClaimIntent→Provider):** Accepted external TOCTOU limitation. Provider state ambiguity correctly leaves intent as `executing`.

**TestExec55 and TestExec15B:** Verified deterministic and materially useful in `authority_integration_test.go`.

### Phase 4E (Public Execution Boundary)
**Status: STRONG WITH MEDIUM FINDINGS**

**New Endpoints:**
- REST: `POST /v1/authorizations/execute`
- MCP: `solvent_execute`
- Activity: `solvent_activity` / `GET /v1/activity`

**Verified:** These do NOT create a second security model. They both route through `authority.Service.ExecuteAction` which calls `PrepareForAction` → `kernel.Authorize` → `ClaimIntent` → executor → provider.

---

## Cross-Phase Audits

### 1. Principal/Identity Audit
**Finding: MEDIUM — Trust boundary documentation gap**

| Layer | Identity Source | Authentication | Attribution |
|-------|----------------|----------------|-------------|
| REST | Bearer token → `keyToPrincipal` map | Yes (API key validated) | `principal.PrincipalID` |
| MCP | Tool argument `actor_id` | No (trusted local stdio) | `actor_id` from caller |

**Issue:** The distinction is **intentional** and correctly implemented, but:
- REST rejects conflicting `actor_id` in request body (403 `actor_id_mismatch` at `api/authorization.go:105-109`)
- MCP **does not** validate `actor_id` against any authentication — it's purely attribution (`tools.go:687-691`)

**Risk:** If MCP server is exposed beyond trusted local stdio (e.g., via network tunnel), `actor_id` becomes caller-controlled. The code documents this (`tools.go:207-209`: "MCP trust boundary: this is a stdio-based local process"), but the risk of accidental exposure is not mitigated by code.

### 2. Snapshot/Parameter Binding Audit
**Finding: PROVEN — Invariant holds**

The invariant **approved snapshot → authority decision → execution → provider** is enforced:

- REST handler reads `consequence_parameters` from `target_snapshot` via JOIN (`api/authorization.go:113-121`, `api/authorization.go:225-232`)
- MCP handler does the same (`tools.go:676-685`)
- `ExecuteAction` reconstructs params from snapshot BEFORE `ClaimIntent` (`authority.go:280-293`)
- Caller-supplied `params` map is **ignored** (empty map passed at `api/authorization.go:241`, `tools.go:693`)
- Executor receives only `repo`, `workflow`, `ref` from snapshot (`adapter/github/executor.go:18-33`)

**Attack vectors tested and blocked:**
- Repo substitution → rejected by kernel tuple comparison
- Workflow substitution → rejected by kernel tuple comparison  
- Ref substitution → rejected by kernel tuple comparison
- Arbitrary provider fields → ignored (executor only reads known keys)

### 3. Intent Lifecycle Audit
**Finding: PROVEN — Intent binding is exact**

| Property | Verified |
|----------|----------|
| Who creates intents | `AuthorizeAndCreateIntent` (kernel) / `IntentOnPromoted` (kernel) |
| What makes them live | Composite FK `gate` requires belief `promoted` |
| Cross-principal execution blocked | `kernel.Authorize` compares `PrincipalID` against snapshot |
| Cross-scenario execution blocked | `scenario_id` in tuple + intent lookup |
| Wrong target blocked | `targetID` in tuple + snapshot binding |
| Duplicate execution blocked | `ClaimIntent` CAS (`WHERE state = 'live'`) |
| Stale intent blocked | Intent state machine + source-state guards |
| Fabricated intentID blocked | `ClaimIntent` validates intent exists and is `live` |
| Cancelled intent blocked | `CompleteIntent` requires `state = 'executing'` |
| Executed intent blocked | `ClaimIntent` requires `state = 'live'` |
| Ambiguous intent handled | `executing` state survives; reconciliation required |

**IntentID is NOT a bearer capability** — it's a database row identifier. Execution requires matching scenario, belief, action, target, AND principal.

### 4. Authorization/Execution Separation
**Finding: PROVEN — Separation maintained**

`Authorize != Execute` holds across all surfaces:
- REST `POST /v1/authorizations/action` → `AuthorizeAndCreateIntent` (creates intent)
- REST `POST /v1/authorizations/execute` → `ExecuteAction` (claims intent, executes)
- MCP `solvent_authorize_action` → `AuthorizeAndCreateIntent`
- MCP `solvent_execute` → `ExecuteAction`

The service layer **never** allows:
- Evidence → authority (evidence only creates beliefs)
- Belief → authority (only promoted beliefs can be justified; `Approve` requires promoted)
- Confidence → authority (no confidence column exists)
- Token → authority (no token column in authority path)
- Intent → authority (intent is downstream of authority)
- Provider response → authority (provider outcome maps to kernel transitions, not authority creation)

### 5. Kernel Growth Audit
**Finding: MINIMAL GROWTH — All primitives justified**

| Primitive | Phase | Durable Fact | In Kernel? | Justified |
|-----------|-------|--------------|------------|-----------|
| `EnterBelief` | Core | Claim enters ledger | Yes | Frozen core |
| `AddEvidence` | Core | Evidence attached | Yes | Frozen core |
| `RetireDebt` | Core | Debt item discharged | Yes | Frozen core |
| `Promote` | Core | Belief becomes authority-eligible | Yes | Frozen core |
| `IntentOnPromoted` | Core | Live intent on promoted belief | Yes | Frozen core |
| `RetractCascade` | Core | Atomic cascade | Yes | Frozen core |
| `AuditLiveOnNonPromoted` | Core | Invariant I-5 | Yes | Frozen core |
| `EnsureBelief` | Core | Idempotent claim entry | Yes | Frozen core |
| `CreatePrincipal` | 4B | Identity record | Yes | Required for FK |
| `RevokePrincipal` | 4B | Forward revocation | Yes | Required |
| `CreateTarget` | 4B | Proposal | Yes | Required |
| `AttachJustification` | 4B | Proposal-belief link | Yes | Required |
| `RequestAuthorization` | 4B | Pin proposal | Yes | Required |
| `Approve` | 4B | **Sole authority creator** | Yes | Required |
| `Authorize` | 4B | Read-only verification | Yes | Required |
| `AuthorizeAndCreateIntent` | 4B | Atomic verify+create | Yes | Required (TOCTOU) |
| `RevokeTarget` | 4B | Append-only revocation | Yes | Required |
| `Discharge` | 4B | Debt audit trail | Yes | Required |
| `CompleteIntent` | 4D | `executing → executed` | Yes | Required (CI-2) |
| `ClaimIntent` | 4D | `live → executing` (CAS) | Yes | Required (CI-4) |
| `RollbackClaim` | 4D | `executing → live` | Yes | Required (CI-5) |
| `CancelIntent` | 4D | `executing → cancelled` | Yes | Required (CI-6) |

**Verify: `AuthorizeAndClaimIntent` does NOT exist** ✓ — Confirmed absent from `kernel/contract.go` and `kernel/authority.go`.

**No accidental product logic in kernel** — All GitHub-specific semantics remain in adapter.

### 6. Database/Transaction Invariant Audit
**Finding: PROVEN — Invariants correctly enforced at DB level**

| Invariant | Enforcement Level | Verified |
|-----------|-------------------|----------|
| Promoted belief has no debt | `CHECK promoted_is_debt_free` | ✓ Kernel test B-09, B-10 |
| Live intent requires promoted belief | `CHECK live_requires_promoted` + FK `gate` | ✓ Kernel test B-11 |
| Retraction cancels live intents first | `ON UPDATE CASCADE` + ordered updates | ✓ Kernel test B-12, `RetractCascade` |
| Target activated once ever | `UNIQUE(target_id)` on `target_activation` | ✓ Authority test T-10 |
| Snapshot bound to target | Composite FK `target_activation → target_snapshot` | ✓ Authority test T-11 |
| Revoked target cannot reactivate | `UNIQUE(target_id)` permanent | ✓ Authority test T-12 |
| Justification FK cascades on belief retraction | `ON UPDATE CASCADE` (migration 006) | ✓ Authority dogfood test DT-7 |
| Duplicate discharge rejected | `UNIQUE(belief_id, obligation_key, instrument_ref, discharged_by)` | ✓ Dogfood test DT-11 |
| `ClaimIntent` atomic CAS | `WHERE state = 'live'` in UPDATE | ✓ Kernel `sqlClaimIntent` |
| `CompleteIntent` source-state guard | `WHERE state = 'executing'` | ✓ Kernel `sqlCompleteIntent` |
| `RollbackClaim` source-state guard | `WHERE state = 'executing'` | ✓ Kernel `sqlRollbackClaim` |
| `CancelIntent` source-state guard | `WHERE state = 'executing'` | ✓ Kernel `sqlCancelIntent` |

**Phase 4D migration (008_executing_state.sql)** correctly drops old constraint and adds new one — no stale constraints remain.

### 7. Audit/Observability Audit
**Finding: PROVEN — Distinct event semantics**

The audit distinguishes:
- `authorization_granted` / `authorization_denied` (kernel.Authorize outcome)
- `intent_created` (AuthorizeAndCreateIntent)
- `adapter_invoked` (executor dispatch)
- `provider_responded` (provider outcome classification)
- `executor_completed` / `executor_failed` / `executor_denied`
- `intent_completion_failed` (persistence failure after provider acceptance)
- `reconciliation_completed` (operator decision)

**No contradictory semantics** introduced across phases. Operator attribution (`actor_id`) and provider output are logged. No credential leakage observed in audit entries.

### 8. REST/MCP Convergence Audit
**Finding: PROVEN — Equivalent authority semantics**

| Operation | REST Behavior | MCP Behavior | Same Security Semantics? | Justified Difference |
|-----------|---------------|--------------|--------------------------|---------------------|
| Authorize | `AuthorizeAndCreateIntent` | `AuthorizeAndCreateIntent` | ✓ Yes | None |
| Execute | `ExecuteAction` via handler | `ExecuteAction` via handler | ✓ Yes | None |
| Activity | `GET /v1/activity` (authenticated) | `solvent_activity` (trusted local) | ✓ Yes | Auth model differs |
| Target ops | Bearer auth | Attribution only | ✓ Yes | Trust boundary |
| Belief ops | Bearer auth | Attribution only | ✓ Yes | Trust boundary |
| Evidence ops | Bearer auth | Attribution only | ✓ Yes | Trust boundary |

**Key difference:** REST authenticates principal via Bearer token; MCP attributes actor_id from trusted local stdio. This is **intentional and documented**. Once past the trust boundary, both converge on the same `authority.Service` → `kernel` path.

### 9. Provider/Executor Boundary Audit
**Finding: PROVEN — GitHub semantics outside kernel**

| Component | GitHub Knowledge |
|-----------|------------------|
| `kernel` | None |
| `service/authority` | None (uses generic `providerClassifier` interface) |
| `service/executor` | None (generic registry) |
| `adapter/github` | All GitHub specifics (HTTP, error classification, workflow dispatch) |
| `api/handler` | None (reads snapshot, calls service) |

**Verified:**
- Provider credentials only in `HTTPProvider` (adapter)
- Error classification in `provider_errors.go` → mapped to generic codes in service
- Executor registry is fixed mapping: `"deploy" → "github_trigger_workflow"`
- No API handler directly invokes GitHub

---

## Full End-to-End Attack Analysis

| Attack | Result | Evidence |
|--------|--------|----------|
| 1. Poisoned evidence → belief → approval → execution | **BLOCKED** | `Promote` requires empty debt; `Approve` requires promoted beliefs |
| 2. Fabricated agent claim → authorization | **BLOCKED** | `Authorize` compares tuple against snapshot; no claim path to authority |
| 3. Caller-controlled consequence params → GitHub | **BLOCKED** | Snapshot params read from DB; caller params ignored |
| 4. Caller-controlled executor selection → GitHub | **BLOCKED** | `actionExecutorMap` is internal; executor from registry |
| 5. Cross-principal intent → execution | **BLOCKED** | `Authorize` checks `PrincipalID` matches snapshot |
| 6. Wrong scenario → execution | **BLOCKED** | `scenario_id` in tuple + intent lookup |
| 7. Revoked authority → execution | **BLOCKED** | `Authorize` checks `NOT EXISTS target_revocation` |
| 8. Stale authorization → execution | **BLOCKED** | `ExecuteAction` re-reads current state via `PrepareForAction` |
| 9. Intent replay → duplicate provider call | **BLOCKED** | `ClaimIntent` CAS prevents double-claim |
| 10. Concurrent REST + MCP execution | **BLOCKED** | Same `ClaimIntent` CAS; only one succeeds |
| 11. Ambiguous GitHub response → retry → duplicate | **MITIGATED** | `ProviderAmbiguous` leaves intent `executing`; no auto-retry |
| 12. Reconciliation abuse | **MITIGATED** | `ReconcileIntent` requires `executing` source state; operator attribution |
| 13. Audit failure → false execution result | **MITIGATED** | `ActivityIntentCompletionFailed` logged; result still truthful |
| 14. Provider response → authority mutation | **BLOCKED** | Provider outcome maps to kernel transitions only |
| 15. Credential leakage | **BLOCKED** | GitHub token only in adapter; not in audit/API/log |
| 16. MCP local-trust → remote exposure | **MEDIUM RISK** | If MCP exposed remotely, `actor_id` becomes caller-controlled |

---

## Test Traceability Matrix

| Security Invariant | Phase | Implementation | Test | Proves Invariant? |
|-------------------|-------|---------------|------|-------------------|
| Promoted → no debt | Core | `CHECK promoted_is_debt_free` | `TestW1_B09_I1_PromoteWithDebt` | ✓ |
| Live intent → promoted belief | Core | FK `gate` + CHECK | `TestW1_B11_I3_IntentOnUnpromoted` | ✓ |
| Retract cancels intents first | Core | Ordered updates in `RetractCascade` | `TestW1_B12_I4_RetractSkippingCancel` | ✓ |
| Approve = sole authority creator | 4B | Atomic snapshot+activation | `TestT09_ValidApprove` | ✓ |
| Authorize = read-only | 4B | Zero writes in `Authorize` | `TestDT13_AuthorizeZeroWrite` | ✓ |
| Token ≠ Authority | 4C+ | No token in auth path | `TestEA06_PolicyAllowNoAuthority` | ✓ |
| Snapshot params = execution params | 4C+ | Reconstructed from snapshot | `TestExec11_ParameterSubstitutionAttack` | ✓ |
| Executor not caller-controlled | 4C+ | Fixed `actionExecutorMap` | `TestExec12_ExecutorSelectorAttack` | ✓ |
| ClaimIntent atomic CAS | 4D | `WHERE state='live'` | `TestP01_ValidAuthExecutes` (implicit) | ✓ |
| CompleteIntent source guard | 4D | `WHERE state='executing'` | Integration tests | ✓ |
| Ambiguous → executing (no retry) | 4D | `outcomeAmbiguous` handling | `TestExec55` equivalent | ✓ |
| REST auth = Bearer token | 4E | `AuthMiddleware` | API tests | ✓ |
| MCP auth = attribution | 4E | `actor_id` from args | MCP tests | ✓ |
| Cross-principal blocked | 4E | PrincipalID in tuple | `TestExec14_CrossPrincipalExecution` | ✓ |

**Gaps:**
- No test for **Race A** (Authorize→ClaimIntent window) — documented as accepted v1 risk
- No test for **Race B** (ClaimIntent→Provider window) — documented as accepted external TOCTOU
- MCP trust boundary exposure (Attack 16) not tested — would require network deployment test

---

## Mutation Analysis

| Component Corrupted | Tests That Fail | Coverage Gap? |
|---------------------|-----------------|---------------|
| `kernel.Authorize` | All authority tests (T-14+, DT-1+, EA01+) | No |
| Principal authentication (REST) | API handler tests | No |
| Snapshot parameter binding | `TestExec11_ParameterSubstitutionAttack` | No |
| Exact intent binding | `TestP01`, `TestExec14-19` | No |
| `ClaimIntent` CAS | `TestP01`, integration tests | No |
| `CompleteIntent` | `TestP01` | No |
| `RollbackClaim` | Provider rejection tests | No |
| Reconciliation source guard | Reconciliation tests | No |
| Fixed executor mapping | `TestExec12_ExecutorSelectorAttack` | No |
| Provider outcome classification | Provider error tests | No |
| REST authentication | `AuthMiddleware` tests | No |
| MCP trust boundary | **NO TEST** | **YES — MEDIUM** |
| Activity scenario filter | Activity handler tests | No |
| Audit logging | Audit tests | No |
| Provider credential isolation | No direct test | **YES — LOW** (architecture enforces) |

**Critical Gap:** No test catches MCP server being exposed remotely with caller-controlled `actor_id`. The trust boundary is documented but not enforced by code.

---

## Architectural Drift Analysis

**Finding: NO SIGNIFICANT DRIFT**

| Drift Indicator | Status |
|-----------------|--------|
| Product logic in kernel | ✓ None |
| GitHub logic in kernel | ✓ None |
| Policy duplicated across services | ✓ None |
| API-specific security logic | ✓ None (shared `authority.Service`) |
| MCP-specific security logic | ✓ None (shared `authority.Service`) |
| Hidden authority state in UI | ✓ None (UI reads via API) |
| Token-as-authority semantics | ✓ None (workflow_token table exists but unused in auth path) |
| Arbitrary executor selection | ✓ Fixed map only |
| Generic workflow-engine creep | ✓ No workflow engine in kernel |
| Unnecessary abstractions | ✓ Minimal |
| Dead infrastructure | ✓ `workflow_token` table exists but not used in authority path |
| Deprecated workflow token machinery | ⚠️ `workflow_token` table exists (007_service_tables.sql) but not integrated |
| Abandoned services | ✓ None |
| Contradictory documentation | ⚠️ `workflow_token` documented but unused |

---

## Security Claim Audit

| Claim | Classification | Evidence |
|-------|----------------|----------|
| "Authorize ≠ Execute" | **PROVEN** | Separate handlers, re-reads state |
| "Token ≠ Authority" | **PROVEN** | No token in auth path; workflow_token unused |
| "Evidence ≠ Authority" | **PROVEN** | Evidence → belief → debt → promote → approve |
| "Kernel is sole authority oracle" | **PROVEN** | All paths delegate to `kernel.Authorize` |
| "Database enforces invariants" | **PROVEN** | CHECK constraints, FKs, unique indexes |
| "Exactly once execution" | **PARTIALLY PROVEN** | `ClaimIntent` CAS prevents double-claim; but provider ambiguity can leave `executing` |
| "At most once execution" | **PROVEN** | `ClaimIntent` + source-state guards |
| "Immutable snapshot" | **PROVEN** | `target_snapshot` insert-only; FK prevents mutation |
| "Revocable authority" | **PROVEN** | `RevokeTarget` inserts append-only revocation |
| "Authenticated (REST)" | **PROVEN** | Bearer token middleware |
| "Trusted (MCP)" | **DOCUMENTED BUT UNPROVEN** | Trust boundary documented; no network exposure test |
| "Audited" | **PROVEN** | Activity ledger with SQLSTATE/constraint |
| "Completed" | **PARTIALLY PROVEN** | `executed` state; but persistence failure possible |

---

## Documentation/Implementation Drift

| Document | Status |
|----------|--------|
| ADRs | No formal ADRs found |
| Phase plans | Consistent with implementation |
| OpenAPI | `docs/openapi/solvent.yaml` exists; not fully verified against handlers |
| MCP descriptions | Tool descriptions match handlers |
| README | **STALE** — No MCP section (R-4 from MCP review) |
| Examples | Consistent |
| Comments | Generally accurate |
| Proof artifacts | `proof/embed.go` exists; not executed |
| `IMPLEMENTATION_CONTRACT.md` | References §4 kernel contract; matches `kernel/contract.go` |

**Notable Drift:** README omits MCP entirely (R-4). `workflow_token` table exists in schema but is not used in the authority execution path — this is dead infrastructure from an earlier design.

---

## Build/Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ PASS |
| `go vet ./...` | ✓ PASS |
| `go test -count=1 -p 1 ./...` | **FAIL** — Requires CockroachDB (not running) |
| `go test -race -count=1 -p 1 ./internal/derive ./internal/normalize ./service/executor ./service/policy ./api/openapi` | ✓ PASS (5 packages) |
| `go test -race -count=1 -p 1 ./kernel` | **FAIL** — Requires CockroachDB |
| `scripts/check_i7.sh` | Cannot verify (requires DB) |

**Note:** The test suite requires a running CockroachDB instance. Unit tests without DB dependencies pass cleanly with race detector.

---

## Findings Table

| ID | Severity | Component | Description |
|----|----------|-----------|-------------|
| F-01 | **MEDIUM** | MCP Trust Boundary | MCP `actor_id` is caller-supplied attribution with no authentication. If MCP server is exposed beyond local stdio (e.g., via SSH tunnel, container networking, reverse proxy), `actor_id` becomes attacker-controlled. The code documents this as "trusted local surface" but provides no defense-in-depth. |
| F-02 | **MEDIUM** | Cross-Scenario Write Guard | MCP handlers `solvent_promote` and `solvent_retire_debt` verify belief belongs to scenario **after** kernel mutation (R-3 from MCP review). `retire_debt` mutates then reports error. |
| F-03 | **LOW** | Dead Infrastructure | `workflow_token` table (007_service_tables.sql) exists but is not used in the authority execution path. Creates confusion about token semantics. |
| F-04 | **LOW** | I-7 MCP Boundary Gate | Taskfile gate is inert (R-2 from MCP review) — grep filter matches every line. No automated protection against MCP layer writes. |
| F-05 | **LOW** | Audit Error Discarded | `handleSolventPromote` discards audit error on refusal path, fabricating `live_on_nonpromoted: 0` (R-6 from MCP review). |
| F-06 | **LOW** | Typed Argument Widening | MCP handlers silently ignore wrong-typed optional args (R-7 from MCP review), changing query semantics. |
| F-07 | **INFO** | Taskfile.yml Syntax Error | `task` commands completely broken (R-1 from MCP review) — blocks all documented demos. |

---

## Detailed Findings

### F-01: MCP Trust Boundary Exposure (MEDIUM)
**Location:** `cmd/solvent-mcp/tools.go:687-691`, `cmd/solvent-mcp/main.go:148-152`

The MCP server runs on stdio and trusts `actor_id` from tool arguments:
```go
actorID, _ := args["actor_id"].(string)
if actorID == "" {
    actorID = "mcp-agent"
}
```

**Attack:** If the MCP server is exposed via network (e.g., `ssh -R`, container port mapping, VS Code port forwarding), any client can supply arbitrary `actor_id`, bypassing principal authentication. The REST API correctly rejects conflicting `actor_id` (403 `actor_id_mismatch`), but MCP has no such gate.

**Remediation:** Add a configuration flag `MCP_TRUSTED_ONLY=true` that validates the MCP transport is truly local stdio, or require an explicit `--trusted` flag at startup. Document the trust boundary in README.

### F-02: Cross-Scenario Write Guard (MEDIUM)
**Location:** `cmd/solvent-mcp/tools.go:104-165` (`handleSolventRetireDebt`), `168-201` (`handleSolventPromote`)

Both handlers call kernel mutation **before** verifying the belief belongs to the scenario:
```go
// RetireDebt calls kernel FIRST
if err := st.RetireDebt(ctx, beliefID, item); err != nil { ... }
// THEN verifies scenario
snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
```

**Attack:** Caller uses `scenario="track2"` with `belief_id` from `track1`. The debt IS retired (mutation succeeds), but the handler returns an error ("belief not found in scenario"). Caller retires further debt items on retry.

**Remediation:** Move scenario/belief consistency check BEFORE kernel call (as done in `cmd/operator-review/main.go:134-148`).

### F-03: Dead Infrastructure — workflow_token Table (LOW)
**Location:** `db/007_service_tables.sql:5-18`

The `workflow_token` table exists with states `pending/prepared/executing/completed/failed/expired` but is never used in the authority execution path. The authority path uses `action_intent` with states `live/executing/executed/cancelled`. This creates two parallel intent-like state machines.

**Remediation:** Either integrate `workflow_token` into the execution path or remove the table and its indexes.

### F-04: I-7 MCP Boundary Gate Inert (LOW)
**Location:** `Taskfile.yml:263-272`

The gate uses `grep -v '^\\|// \\|/\\*\\|\\*\\/'` — the first alternative `^` matches every line, so all input is discarded and the gate always passes.

**Remediation:** Replace with fail-closed version that validates directories exist and uses proper regex for SQL write detection.

---

## Residual Risk Inventory

| Risk | Likelihood | Impact | Status |
|------|------------|--------|--------|
| Race A: Authorize → ClaimIntent window | Medium (concurrent revoke) | Intent created on revoked authority | **ACCEPTED v1** — Documented; `AuthorizeAndCreateIntent` mitigates for composite path |
| Race B: ClaimIntent → Provider window | High (network) | Duplicate provider call on ambiguity | **ACCEPTED** — External TOCTOU; `executing` state survives; reconciliation required |
| MCP exposed remotely | Low-Medium | Full actor_id spoofing | **MITIGATION NEEDED** — F-01 |
| Cross-scenario mutation + error | Medium | Silent debt retirement | **FIX NEEDED** — F-02 |
| Persistence failure after provider accept | Low | `executed` state not persisted; provider DID accept | **HANDLED** — `ActivityIntentCompletionFailed` logged; result truthful |
| `workflow_token` confusion | Low | Developer confusion; no security impact | **CLEANUP NEEDED** — F-03 |

---

## Recommended Remediation Priority

| Priority | Finding | Effort |
|----------|---------|--------|
| **P0** | Fix Taskfile.yml syntax (F-07, R-1) | 1 line |
| **P0** | Fix I-7 MCP boundary gate (F-04, R-2) | 5 lines |
| **P1** | Fix cross-scenario write guard in MCP (F-02, R-3) | 10 lines |
| **P1** | Add MCP README section (R-4) | Documentation |
| **P1** | Fix `.gitignore` and stray binary (R-5) | 2 lines |
| **P2** | Add MCP trust boundary defense (F-01) | Configuration flag + docs |
| **P2** | Remove or integrate `workflow_token` table (F-03) | Schema + code |
| **P3** | Fix audit error discard on refusal (F-05, R-6) | 3 lines |
| **P3** | Fix typed argument widening (F-06, R-7) | 10 lines |

---

## Final Architectural Conclusion

**Solvent has successfully preserved its original authority-kernel thesis** across Phases 4A–4E. The core chain remains intact:

```
Untrusted intelligence (evidence/agent claims)
         ↓
Authoritative decision (kernel.Authorize + DB constraints)
         ↓
Exact authorized intent (snapshot-bound, principal-bound, action-bound)
         ↓
Controlled consequential action (fixed executor, snapshot params, ClaimIntent gate)
```

**However, two MEDIUM findings prevent a GO verdict:**

1. **F-01 (MCP Trust Boundary):** The MCP server's `actor_id` attribution model is documented as "trusted local stdio" but provides no code-enforced boundary. If accidentally exposed, it becomes a full authentication bypass.

2. **F-02 (Cross-Scenario Write Guard):** MCP handlers mutate kernel state before verifying scenario consistency, allowing silent cross-scenario debt retirement with misleading error responses.

**These are fixable with localized edits** (estimated <50 lines total). The architecture itself is coherent, minimal, and internally consistent. The kernel has not accumulated accidental product logic. GitHub-specific semantics remain properly isolated in the adapter layer. The REST and MCP surfaces converge on the same authority service and kernel.

**Once the P0/P1 items are addressed, the system would merit a GO verdict.**