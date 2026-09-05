# Phase 4A — Canonical Solvent API Contract

> **Status:** Design document for independent adversarial review
>
> **Scope:** API contract design only. No code changes, no schema changes, no kernel modifications.
>
> **Source of truth:** The Solvent repository at commit HEAD. All claims verified against actual code.

---

## 1. Executive Decision

Phase 4A defines the canonical public Solvent API contract before any UI, SDK, or client implementation.

The API becomes the language-neutral product boundary. MCP, A2A, Web UI, future SDKs, and any foreign-language client converge on the same service semantics. No protocol gets its own authorization engine.

**What Phase 4A IS:**
- A complete API resource model, operation set, and error contract
- A security-first design with explicit forbidden surface
- A reference for adversarial review before implementation
- An implementation plan for Phase 4B

**What Phase 4A IS NOT:**
- Implementation code
- A schema migration
- A UI design
- A compliance subsystem
- A production executor
- An enterprise IAM system

**Governing constraint:** The kernel remains the smallest trusted authority core. The API exposes kernel semantics without leaking implementation details.

---

## 2. Repository Reconnaissance

### What exists (shipped)

| Layer | Status | Notes |
|-------|--------|-------|
| Kernel package | Complete | 17 Contract methods, AuthorityTuple, AuthorizeResult, 24 named SQL statements |
| 4 frozen ledger tables | Complete | belief, belief_edge, evidence, action_intent |
| 7 authority tables | Complete | principal, authority_target, target_snapshot, target_activation, target_revocation, justification, debt_discharge |
| 4 service tables | Complete | workflow_token (DEAD), policy_tool, policy_actor, audit_activity |
| 3 corpus/wizard tables | Complete | corpus_issue, belief_corpus_citation, refusal_log |
| MCP server | Complete | 16 tools, stdio transport, fail-closed validation |
| Service layer | Complete | authority, audit, policy, evidence, executor |
| Authority lifecycle | Complete | CreatePrincipal through RevokeTarget, full test coverage |
| Evidence pipeline | Complete | normalize → derive → belief.Process → intent.Audit |
| Demo web app | Complete | Read-only HTML views, hardcoded Track 2 scenario |
| Demo wizard | Complete | Interactive HTTP API under /demo, cookie-based sessions |
| Integration tests | Complete | 35 test files, kernel authority tests, MCP handler tests, agentjacking defense tests |
| Formal model | Complete | Lean 4, zero sorry/admit |

### What is explicitly deferred

- No HTTP REST API (only MCP stdio + wizard HTTP + demo HTML)
- No OpenAPI/schema files
- No repository/DAO abstraction (direct SQL everywhere)
- No authentication/RBAC
- No real executor implementations (registry is empty)
- No workflow_token usage (table exists, zero code references)
- No multi-tenancy
- No UI beyond the embedded demo wizard
- No SDKs

### Architectural properties relevant to API design

1. The kernel is the **sole write boundary** — I-7 enforced by grep script
2. `internal/view` and `cmd/solvent-mcp` are **enforced read-only**
3. All writes go through `crdb.ExecuteTx` for serialization retry
4. SQLSTATE classification uses `interface{ SQLState() string }` — no driver import in kernel
5. `AuthorityTuple` is the core authorization model (8 fields)
6. `service/authority.Service.PrepareForAction()` is the ONE production authorization path
7. `service/authority.Service.ExecuteAction()` is the ONE production execution path (not yet wired to real executors)

---

## 3. Current Domain Inventory

### 3.1 Core Ledger Entities

| Entity | Purpose | Authoritative Source | Owner | Read Path | Write Path | Security Sensitivity | Current API Exposure | Recommended API Exposure |
|--------|---------|---------------------|-------|-----------|------------|---------------------|---------------------|------------------------|
| **Belief** | A claim about the world with lifecycle state | `belief` table | kernel | `view.GetSnapshot`, `view.ExplainSnapshot` | `kernel.EnterBelief`, `kernel.Promote`, `kernel.RetractCascade`, `kernel.RetireDebt`, `kernel.EnsureBelief` | High — status controls authority | MCP `solvent_ledger`, `solvent_explain`; wizard `/api/state`; demo `/beliefs` | Public read, restricted write |
| **Belief Edge** | Parent-child relationship between beliefs | `belief_edge` table | wizard (via pipeline) | `view.GetSnapshot` (join) | `internal/pipeline` | Medium — drives cascade traversal | Indirectly via `solvent_ledger` | Read-only public |
| **Evidence** | Attributed fact supporting a belief | `evidence` table | kernel | `view.GetSnapshot` (join) | `kernel.AddEvidence` | Medium — provenance matters | MCP `solvent_ledger` (include_evidence); wizard `/api/state` | Public read, restricted write |
| **Action Intent** | A live intent to take a real-world action | `action_intent` table | kernel | `view.GetSnapshot` (join) | `kernel.IntentOnPromoted` | **Critical** — represents pending consequential action | MCP `solvent_ledger`; wizard `/api/state`; demo `/intents` | Public read, restricted write |
| **Corpus Issue** | External institutional memory (GitHub issues) | `corpus_issue` table | ingestion code | `corpus.Search` (ANN) | `corpus.Ingest` | Low — external reference | Indirectly via `solvent_ledger` | Internal only (demo infrastructure) |
| **Belief-Corpus Citation** | Retrieval provenance link | `belief_corpus_citation` table | wizard | direct query | `corpus.Cite` / `corpus.Uncite` | Low — advisory | Indirectly via `solvent_ledger` | Internal only (demo infrastructure) |
| **Refusal Log** | Application record of DB refusals | `refusal_log` table | wizard | direct query | `wizard.logRefusal` | Low — audit artifact | Wizard `/api/state` | Internal only (demo infrastructure) |

### 3.2 Authority Entities

| Entity | Purpose | Authoritative Source | Owner | Read Path | Write Path | Security Sensitivity | Current API Exposure | Recommended API Exposure |
|--------|---------|---------------------|-------|-----------|------------|---------------------|---------------------|------------------------|
| **Principal** | Stable actor identity record | `principal` table | kernel | kernel authority lifecycle | `kernel.CreatePrincipal`, `kernel.RevokePrincipal` | High — identity boundary | MCP `solvent_create_principal`, `solvent_revoke_principal` | Public read, restricted write |
| **Authority Target** | Mutable proposal for authority grant | `authority_target` table | kernel | kernel authority lifecycle | `kernel.CreateTarget`, `kernel.RequestAuthorization` | **Critical** — proposal becomes authority | MCP `solvent_create_target`, `solvent_request_authorization` | Public read (own targets), restricted write |
| **Target Snapshot** | Immutable approved authority representation | `target_snapshot` table | kernel | `kernel.Authorize` (read-only) | `kernel.Approve` (INSERT-only) | **Critical** — sole execution-time authority | Indirectly via `solvent_authorize` | Internal (derived from approval) |
| **Target Activation** | Authority-granting fact (once-ever) | `target_activation` table | kernel | `kernel.Authorize` (read-only) | `kernel.Approve` (INSERT-only) | **Critical** — the grant | Indirectly via `solvent_authorize` | Internal (derived from approval) |
| **Target Revocation** | Append-only revocation fact | `target_revocation` table | kernel | `kernel.Authorize` (read-only) | `kernel.RevokeTarget` (INSERT-only) | **Critical** — permanent revocation | MCP `solvent_revoke_target` | Public read, restricted write |
| **Justification** | Proposal-time belief-to-target link | `justification` table | kernel | `kernel.Approve` (read) | `kernel.AttachJustification` (INSERT-only) | Medium — proposal integrity | MCP `solvent_attach_justification` | Public read, restricted write |
| **Debt Discharge** | Attribution record for debt retirement | `debt_discharge` table | kernel | direct query | `kernel.Discharge` (INSERT-only) | Medium — replay protection | MCP `solvent_discharge` | Public read, restricted write |

### 3.3 Service Entities

| Entity | Purpose | Authoritative Source | Owner | Read Path | Write Path | Security Sensitivity | Current API Exposure | Recommended API Exposure |
|--------|---------|---------------------|-------|-----------|------------|---------------------|---------------------|------------------------|
| **Audit Activity** | Append-only activity ledger | `audit_activity` table | service/audit | `service/audit.GetActivities` | `service/audit.Log`, `service/audit.LogRefusal` | Medium — audit trail | Indirectly via demo `/audit` | Public read (scoped), no client write |
| **Policy Tool** | Tool registry for constraint evaluation | `policy_tool` table | service/policy | `service/policy.GetTool` | `service/policy.RegisterTool` | Low — advisory | None | Internal (admin only) |
| **Policy Actor** | Actor registry for constraint evaluation | `policy_actor` table | service/policy | `service/policy.GetActor` | `service/policy.RegisterActor`, `DeactivateActor` | Low — advisory | None | Internal (admin only) |
| **Workflow Token** | DEAD TABLE | `workflow_token` table | None | None | None | None — dead | None | **Do not expose** |

> **Note:** "Recommended API Exposure" in the tables above describes the intended Phase 4B API contract and is contingent on the authentication and access-control design in §6 being implemented; it does not claim that these restrictions are enforced by the current v0 repository.

### 3.4 Entity Classification

**FACTS** (observable, immutable):
- Belief (current status, debt)
- Evidence (provenance, content hash)
- Action Intent (state)
- Target Snapshot (approved authority content)
- Target Activation (grant fact)
- Target Revocation (revocation fact)
- Debt Discharge (attribution record)
- Audit Activity (event record)

**PROJECTIONS** (derived, recomputable):
- Quality scores (from evidence)
- Explain projections (from belief + intent + debt state)
- Policy constraints (from tool + actor registries)
- Ledger summaries (counts, aggregates)

**ADVISORY INFORMATION** (informational, non-authoritative):
- Corpus citations (retrieval provenance)
- Refusal logs (application-level audit)
- Invariant receipts (check results)

**AUTHORITY** (controls what agents may do):
- Target Snapshot + Target Activation (the only authority)
- Justification links (proposal-time, not authority itself)
- Belief promotion status (gate, not authority)

---

## 4. Proposed Public Resource Model

### 4.1 Design Principles

- Smallest coherent public model
- Stable domain semantics, not kernel implementation details
- Every resource maps to an existing table or a derived projection
- No new authority-core tables for API convenience
- No kernel changes required

### 4.2 Public Resources

| Resource | Meaning | Lifecycle | Authoritative Source | Identifier | Immutable Fields | Mutable Fields | Relationships | Authoritative or Derived | Agents Create | Agents Read | Agents Update | Humans Create | Humans Read | Humans Update | Crosses Authority Boundary | Requires AuthN | Requires AuthZ | Mutation Needs Idempotency | Audit Requirements |
|----------|---------|-----------|---------------------|------------|-----------------|---------------|---------------|-------------------------|--------------|------------|--------------|-------------|------------|-------------|--------------------------|---------------|---------------|--------------------------|-------------------|
| **Belief** | A claim with lifecycle state | entered → promoted → retracted | `belief` table | `belief_id` (UUID) | id, scenario_id, claim, claim_type | status, debt, final_truth | edges, evidence, intents, justifications | Authoritative | Yes (enter) | Yes | No (status via Promote/Retract only) | Yes | Yes | No | Yes (status gates authority) | Yes | No (read) | No (Promote/Retract are separate) | Entry, promotion, retraction |
| **Evidence** | Attributed fact supporting a belief | append-only | `evidence` table | `evidence_id` (UUID) | id, scenario_id, belief_id, provenance_class, content_sha256, ingested_at | source_url, snapshot, source_observed_at | belief | Authoritative | Yes (ingest) | Yes | No | Yes | Yes | No | No | Yes | No | No (append-only) | Ingestion |
| **Action Intent** | Live intent to take a real-world action | live → cancelled → executed | `action_intent` table | `intent_id` (UUID) | id, scenario_id, belief_id, belief_status, action | state | belief, authority target | Authoritative | Yes (via AuthorizeAction) | Yes | No (state via Cancel only) | No (MCP only) | Yes | No | Yes (live intents gate execution) | Yes | Yes (authority check) | No (created with authority) | Creation, cancellation |
| **Principal** | Stable actor identity | active → revoked | `principal` table | `principal_id` (UUID) | principal_id, principal_type, issuer, created_at | revoked_at | targets, justifications, discharges | Authoritative | Yes | Yes | No (Revoke is separate) | Yes | Yes | No | Yes (identity boundary) | Yes | Yes (admin) | No (not idempotent v0) | Creation, revocation |
| **Authority Target** | Proposal for authority grant | proposed → requested → approved → revoked | `authority_target` table | `target_id` (UUID) | target_id, principal_id, resource_type, resource_id, scope, action_namespace, action_name, consequence_type, consequence_parameters, created_by, created_at | requested_by, requested_at, pinned_request_hash | justifications, snapshot, activation, revocation | Authoritative | Yes | Yes (own) | No (lifecycle via separate operations) | Yes | Yes | No | **Yes** — proposal becomes authority | Yes | Yes (create requires principal) | No (not idempotent v0) | Creation, request, approval, revocation |
| **Authority** | Approved authority (snapshot + activation) | active → revoked | `target_snapshot` + `target_activation` | `target_id` (UUID) | snapshot_id, target_id, all tuple fields, justification_set, approver, approved_at, snapshot_hash | None (immutable) | target, justifications, revocation | Authoritative | No (created by Approve) | Yes | No (immutable) | No | Yes | No | **Yes** — this IS the authority | Yes | Yes (read: verify; write: Approve) | No (append-only) | Approval |
| **Revocation** | Permanent authority revocation | append-only | `target_revocation` | `target_id` (UUID) | target_id, revoked_at, revoked_by, reason | None (immutable) | target, authority | Authoritative | No | Yes | No (immutable) | No | Yes | No | **Yes** — kills authority | Yes | Yes (admin) | No (append-only) | Revocation |
| **Justification** | Proposal-time belief-to-target link | append-only | `justification` | `justification_id` (UUID) | justification_id, target_id, belief_id, belief_status, attached_by, attached_at | None (immutable) | target, belief | Authoritative (proposal-time) | Yes | Yes | No (immutable) | Yes | Yes | No | Yes (links belief to authority) | Yes | Yes (target must exist) | Yes (ON CONFLICT DO NOTHING) | Attachment |
| **Debt Discharge** | Attribution record for debt retirement | append-only | `debt_discharge` | `discharge_id` (UUID) | discharge_id, belief_id, obligation_key, instrument_ref, discharged_by, accepted_at | None (immutable) | belief, principal | Authoritative | Yes | Yes | No (immutable) | Yes | Yes | No | No | Yes | Yes (belief must exist) | Yes (UNIQUE constraint) | Discharge |
| **Activity** | Append-only audit event | append-only | `audit_activity` | `activity_id` (UUID) | id, scenario_id, type, created_at | actor_id, subject_id, details, sqlstate, constraint_name, refusal | scenario | Derived (from operations) | No (system only) | Yes (scoped) | No (immutable) | No | Yes (scoped) | No | No | Yes | No (read-only) | No (append-only) | System-generated |

### 4.3 Resources Explicitly NOT Exposed

| Entity | Why Not |
|--------|---------|
| `target_snapshot` directly | Exposed through `Authority` resource; the snapshot IS the authority content |
| `target_activation` directly | Exposed through `Authority` resource; activation is the grant fact |
| `belief_edge` directly | Exposed through `Belief` relationships; internal graph structure |
| `workflow_token` | DEAD TABLE. Zero code references. Do not expose. |
| `policy_tool` | Internal admin. Not part of the public authority model. |
| `policy_actor` | Internal admin. Not part of the public authority model. |
| `refusal_log` | Demo wizard artifact. Not part of the canonical API. |
| `belief_corpus_citation` | Demo retrieval provenance. Internal only. |
| `corpus_issue` | Demo external memory. Internal only. |

---

## 5. Canonical Operations

### 5.1 Operation Inventory

| # | Operation | HTTP Method | Endpoint | Creates Authority | Revokes Authority | Causes External Execution | Read-Only | Reversible | Irreversible |
|---|-----------|-------------|----------|-------------------|-------------------|--------------------------|-----------|------------|--------------|
| 1 | Enter Belief | POST | `/v1/beliefs` | No | No | No | No | Yes (retract) | No |
| 2 | Get Belief | GET | `/v1/beliefs/{id}` | No | No | No | Yes | — | — |
| 3 | List Beliefs | GET | `/v1/beliefs` | No | No | No | Yes | — | — |
| 4 | Add Evidence | POST | `/v1/evidence` | No | No | No | No | No | No |
| 5 | Get Evidence | GET | `/v1/evidence/{id}` | No | No | No | Yes | — | — |
| 6 | List Evidence for Belief | GET | `/v1/beliefs/{id}/evidence` | No | No | No | Yes | — | — |
| 7 | Retire Debt | POST | `/v1/beliefs/{id}/debt/retire` | No | No | No | No | No | No |
| 8 | Promote Belief | POST | `/v1/beliefs/{id}/promote` | No | No | No | No | No | No |
| 9 | Retract Belief | POST | `/v1/beliefs/{id}/retract` | No | No | No | No | No | No |
| 10 | Create Principal | POST | `/v1/principals` | No | No | No | No | Yes (revoke) | No |
| 11 | Get Principal | GET | `/v1/principals/{id}` | No | No | No | Yes | — | — |
| 12 | List Principals | GET | `/v1/principals` | No | No | No | Yes | — | — |
| 13 | Revoke Principal | POST | `/v1/principals/{id}/revoke` | No | No | No | No | No | No |
| 14 | Create Target | POST | `/v1/targets` | No | No | No | No | No | No |
| 15 | Get Target | GET | `/v1/targets/{id}` | No | No | No | Yes | — | — |
| 16 | List Targets | GET | `/v1/targets` | No | No | No | Yes | — | — |
| 17 | Attach Justification | POST | `/v1/targets/{id}/justifications` | No | No | No | No | No | No |
| 18 | Request Authorization | POST | `/v1/targets/{id}/request` | No | No | No | No | No | No |
| 19 | Approve Target | POST | `/v1/targets/{id}/approve` | **Yes** | No | No | No | No | **Yes** |
| 20 | Authorize (verify) | POST | `/v1/authorizations/verify` | No | No | No | Yes | — | — |
| 21 | Authorize Action | POST | `/v1/authorizations/action` | No | No | No | No | No | No |
| 22 | Revoke Target | POST | `/v1/targets/{id}/revoke` | No | **Yes** | No | No | No | **Yes** |
| 23 | Discharge Debt | POST | `/v1/discharge` | No | No | No | No | No | No |
| 24 | Get Activity | GET | `/v1/activity` | No | No | No | Yes | — | — |
| 25 | Explain Belief | GET | `/v1/beliefs/{id}/explain` | No | No | No | Yes | — | — |
| 26 | Get Ledger Summary | GET | `/v1/ledger` | No | No | No | Yes | — | — |

### 5.2 Operation Details

#### POST `/v1/beliefs` — Enter Belief

**Purpose:** Create a new belief with a claim and claim type.

**Request:**
```json
{
  "scenario_id": "uuid",
  "claim": "string",
  "claim_type": "derived|accommodated|postulated"
}
```

**Response:**
```json
{
  "belief_id": "uuid",
  "claim": "string",
  "claim_type": "derived",
  "status": "entered",
  "debt": ["needProvenanceCheck", "needContradictionSweep", "needBlastRadius", "needRollbackPlan", "needVersionPin", "needOperatorSignoff"],
  "created_at": "2026-01-01T00:00:00Z"
}
```

**Authentication:** Required (API key or bearer token).
**Authorization:** Caller must be authenticated; no Solvent authority required.
**Actor type:** AGENT or HUMAN.
**Idempotency:** Not idempotent in v0 (no unique constraint on claim). Client should handle duplicate creation.
**Kernel interaction:** `kernel.EnterBelief` or `kernel.EnsureBelief`.
**Audit:** `ActivityBeliefEntered`.
**Failure modes:** Malformed UUID → 400. Empty claim → 400. Unknown claim_type → DB CHECK violation → 422 with SQLSTATE 23514.

---

#### GET `/v1/beliefs/{id}` — Get Belief

**Purpose:** Read a single belief by ID.

**Request:** Path parameter `id` (UUID).

**Response:**
```json
{
  "belief_id": "uuid",
  "scenario_id": "uuid",
  "claim": "string",
  "claim_type": "derived",
  "status": "promoted",
  "debt": [],
  "final_truth": false,
  "evidence_count": 3,
  "intent_count": 1
}
```

**Authentication:** Required.
**Authorization:** Read-only. No Solvent authority required.
**Kernel interaction:** Direct read from `belief` table.
**Failure modes:** Not found → 404.

---

#### GET `/v1/beliefs` — List Beliefs

**Purpose:** List beliefs in a scenario with optional filtering.

**Query parameters:** `scenario_id` (required), `status` (optional filter), `claim_type` (optional filter), `limit` (default 100), `offset` (default 0).

**Response:**
```json
{
  "beliefs": [...],
  "total": 42,
  "limit": 100,
  "offset": 0
}
```

---

#### POST `/v1/evidence` — Add Evidence

**Purpose:** Record evidence supporting a belief.

**Request:**
```json
{
  "scenario_id": "uuid",
  "belief_id": "uuid",
  "provenance_class": "external_feed|reproducible_artifact|live_scan|operator_asserted",
  "source_url": "string (optional)",
  "content_sha256": "string"
}
```

**Response:**
```json
{
  "evidence_id": "uuid",
  "belief_id": "uuid",
  "provenance_class": "external_feed",
  "ingested_at": "2026-01-01T00:00:00Z"
}
```

**Authentication:** Required.
**Authorization:** Caller must be authenticated. No Solvent authority required for evidence submission.
**Idempotency:** Not idempotent in v0. Duplicate evidence creates duplicate rows.
**Kernel interaction:** `kernel.AddEvidence`.
**Audit:** `ActivityEvidenceAdded`.

---

#### POST `/v1/beliefs/{id}/debt/retire` — Retire Debt

**Purpose:** Record that one review obligation on a belief has been discharged.

**Request:**
```json
{
  "debt_item": "needProvenanceCheck|needContradictionSweep|needBlastRadius|needRollbackPlan|needVersionPin|needOperatorSignoff"
}
```

**Response:**
```json
{
  "belief_id": "uuid",
  "debt": ["remaining", "items"]
}
```

**Authentication:** Required.
**Authorization:** Caller must be authenticated. No Solvent authority required.
**Idempotency:** Idempotent (array_remove on absent item is a no-op).
**Kernel interaction:** `kernel.RetireDebt`.
**Audit:** `ActivityDebtRetired`.

---

#### POST `/v1/beliefs/{id}/promote` — Promote Belief

**Purpose:** Attempt to promote a belief to authorized status.

**Request:** Empty body `{}`.

**Response (success):**
```json
{
  "belief_id": "uuid",
  "status": "promoted"
}
```

**Response (refusal — the product):**
```json
{
  "ok": false,
  "statement": "promote",
  "sqlstate": "23514",
  "constraint": "promoted_is_debt_free",
  "detail": "promotion blocked: open debt or final-truth language"
}
```

**Authentication:** Required.
**Authorization:** No Solvent authority required. The database CHECK constraint is the gate.
**Idempotency:** Idempotent if already promoted (UPDATE on same status is a no-op).
**Kernel interaction:** `kernel.Promote`.
**Audit:** `ActivityBeliefPromoted` on success; `LogRefusal` on refusal.
**Failure modes:** Open debt → SQLSTATE 23514. Final truth → SQLSTATE 23514. Not found → 404.

**Design note:** The refusal IS the product. Return HTTP 200 with the Verdict structure, not HTTP 4xx. This preserves the database refusal as a first-class API response.

---

#### POST `/v1/beliefs/{id}/retract` — Retract Belief

**Purpose:** Retract a belief, cancelling all dependent live intents.

**Request:**
```json
{
  "reason": "string (optional)"
}
```

**Response:**
```json
{
  "belief_id": "uuid",
  "retracted": 1,
  "intents_cancelled": 2,
  "descendants_retracted": 3
}
```

**Authentication:** Required.
**Authorization:** No Solvent authority required.
**Kernel interaction:** `kernel.RetractCascade`.
**Audit:** `ActivityBeliefRetracted`, `ActivityIntentCancelled`.

---

#### POST `/v1/principals` — Create Principal

**Purpose:** Create a new principal (identity record).

**Request:**
```json
{
  "principal_type": "human|agent|workload|service",
  "issuer": "string"
}
```

**Response:**
```json
{
  "principal_id": "uuid",
  "principal_type": "human",
  "issuer": "string",
  "created_at": "2026-01-01T00:00:00Z"
}
```

**Authentication:** Required.
**Authorization:** Admin operation. Requires privileged access.
**Idempotency:** Not idempotent in v0 (no unique constraint on type+issuer).
**Kernel interaction:** `kernel.CreatePrincipal`.

---

#### POST `/v1/principals/{id}/revoke` — Revoke Principal

**Purpose:** Revoke a principal. Existing FK references remain valid.

**Request:** Empty body `{}`.

**Response:**
```json
{
  "principal_id": "uuid",
  "revoked": true
}
```

**Authentication:** Required.
**Authorization:** Admin operation.
**Idempotency:** Idempotent (UPDATE WHERE revoked_at IS NULL).
**Kernel interaction:** `kernel.RevokePrincipal`.

---

#### POST `/v1/targets` — Create Authority Target

**Purpose:** Create an authority target proposal. No authority is granted until Approve.

**Request:**
```json
{
  "principal_id": "uuid",
  "resource_type": "string",
  "resource_id": "string",
  "scope": "string",
  "action_namespace": "string",
  "action_name": "string",
  "consequence_type": "string",
  "consequence_parameters": {},
  "created_by": "uuid"
}
```

**Response:**
```json
{
  "target_id": "uuid",
  "principal_id": "uuid",
  "resource_type": "scenario",
  "resource_id": "uuid",
  "scope": "belief:uuid",
  "action_namespace": "solvent",
  "action_name": "deploy",
  "consequence_type": "execution",
  "consequence_parameters": {},
  "created_by": "uuid",
  "state": "proposed",
  "created_at": "2026-01-01T00:00:00Z"
}
```

**Authentication:** Required.
**Authorization:** Caller must be authenticated. No Solvent authority required for proposal creation.
**Idempotency:** Not idempotent in v0.
**Kernel interaction:** `kernel.CreateTarget`.
**CHECK constraints enforced:** All six text fields must be non-empty (resource_type, resource_id, scope, action_namespace, action_name, consequence_type).

---

#### POST `/v1/targets/{id}/justifications` — Attach Justification

**Purpose:** Attach a promoted belief as justification for an authority target.

**Request:**
```json
{
  "belief_id": "uuid",
  "belief_status": "promoted",
  "attached_by": "uuid"
}
```

**Response:**
```json
{
  "target_id": "uuid",
  "belief_id": "uuid",
  "belief_status": "promoted",
  "attached": true
}
```

**Authentication:** Required.
**Authorization:** Target must exist. Target must not be activated. Belief must exist with the stated status.
**Idempotency:** Idempotent (ON CONFLICT DO NOTHING).
**Kernel interaction:** `kernel.AttachJustification`.
**Failure modes:** Target not found → `ErrTargetNotFound`. Already activated → `ErrAlreadyActivated`. Belief not found → FK violation.

---

#### POST `/v1/targets/{id}/request` — Request Authorization

**Purpose:** Pin the current proposal and justification set for approval.

**Request:**
```json
{
  "requested_by": "uuid"
}
```

**Response:**
```json
{
  "target_id": "uuid",
  "requested": true
}
```

**Authentication:** Required.
**Authorization:** Target must exist. Must not be activated. Must not be revoked.
**Kernel interaction:** `kernel.RequestAuthorization`.
**Failure modes:** Target not found → `ErrTargetNotFound`. Already activated → `ErrAlreadyActivated`. Already revoked → `ErrAlreadyRevoked`.

---

#### POST `/v1/targets/{id}/approve` — Approve Target

**Purpose:** Approve an authority target. Atomically creates an immutable snapshot and activation. This is the ONLY operation that creates authority.

**Request:**
```json
{
  "approved_by": "uuid"
}
```

**Response:**
```json
{
  "target_id": "uuid",
  "approved": true,
  "approved_by": "uuid"
}
```

**Authentication:** Required. Must be called from a trusted administrative surface.
**Authorization:** The approver principal must exist and not be revoked. Pin fields must be populated. Hash must match. All justifications must reference promoted beliefs.
**Creates authority:** YES. This is the sole authority-creating operation.
**Irreversible:** YES. `target_activation.UNIQUE(target_id)` prevents re-approval.
**Kernel interaction:** `kernel.Approve`.
**Failure modes:**
- Target not found → `ErrTargetNotFound`
- Pin not populated → `ErrAuthorizationMissing`
- Hash mismatch → `ErrApprovalPinMismatch`
- No justifications → `ErrInvalidProposal`
- Belief not promoted → `ErrBeliefNotPromoted`
- Approver revoked → `ErrRevokedPrincipal`
- Already activated → UNIQUE violation (SQLSTATE 23505)

---

#### POST `/v1/authorizations/verify` — Verify Authorization

**Purpose:** Read-only verification: check if an execution-time tuple matches the approved authority.

**Request:**
```json
{
  "target_id": "uuid",
  "principal_id": "uuid",
  "resource_type": "string",
  "resource_id": "string",
  "scope": "string",
  "action_namespace": "string",
  "action_name": "string",
  "consequence_type": "string",
  "consequence_parameters": {}
}
```

**Response (allowed):**
```json
{
  "target_id": "uuid",
  "allowed": true,
  "reason": ""
}
```

**Response (denied):**
```json
{
  "target_id": "uuid",
  "allowed": false,
  "reason": "principal mismatch"
}
```

**Authentication:** Required.
**Authorization:** Read-only operation. No Solvent authority required for verification.
**Kernel interaction:** `kernel.Authorize` (read-only, zero writes).
**Audit:** `ActivityAuthorizationGranted` or `ActivityAuthorizationDenied`.

---

#### POST `/v1/authorizations/action` — Authorize Action

**Purpose:** Record a live intent to take a real-world action, citing a belief as its warrant, after verifying current authority.

**Request:**
```json
{
  "scenario_id": "uuid",
  "belief_id": "uuid",
  "action": "string",
  "action_source": "user_typed",
  "target_id": "uuid",
  "actor_id": "uuid"
}
```

**Response:**
```json
{
  "belief_id": "uuid",
  "intent_id": "uuid",
  "intent_state": "live",
  "action": "string",
  "authority": {
    "allowed": true,
    "reason": ""
  }
}
```

**Authentication:** Required.
**Authorization:** **Crosses authority boundary.** `action_source` must be `"user_typed"` (hard拒绝 `"tool_output"`). `target_id` and `actor_id` are required. `service/authority.PrepareForAction` calls `kernel.Authorize`. Only if allowed does `kernel.IntentOnPromoted` create the intent.
**Creates authority:** No. Verifies existing authority.
**Kernel interaction:** `service/authority.PrepareForAction` → `kernel.Authorize` → `kernel.IntentOnPromoted`.
**Failure modes:**
- `action_source = "tool_output"` → hard拒绝 before DB access
- Missing target_id/actor_id → fail-closed before authority check
- Authority denied → refusal with reason
- Belief not promoted → SQLSTATE 23503 (gate)
- Missing scenario/belief → 400/404

**Security critical:** This is the operation where an agent requests permission to act. The authority check happens here, not at execution time.

---

#### POST `/v1/targets/{id}/revoke` — Revoke Target

**Purpose:** Revoke an authority target. Inserts an immutable revocation fact.

**Request:**
```json
{
  "revoked_by": "uuid",
  "reason": "string"
}
```

**Response:**
```json
{
  "target_id": "uuid",
  "revoked": true
}
```

**Authentication:** Required.
**Authorization:** Admin operation. Target must be activated. Must not already be revoked.
**Creates authority:** No.
**Revokes authority:** YES. Permanent and irreversible.
**Kernel interaction:** `kernel.RevokeTarget`.
**Failure modes:** Target not activated → `ErrTargetNotActivated`. Already revoked → `ErrAlreadyRevoked`.

---

#### POST `/v1/discharge` — Discharge Debt

**Purpose:** Record that a belief's debt obligation has been discharged by a principal.

**Request:**
```json
{
  "belief_id": "uuid",
  "obligation_key": "string",
  "instrument_ref": "string",
  "discharged_by": "uuid"
}
```

**Response:**
```json
{
  "belief_id": "uuid",
  "obligation_key": "string",
  "instrument_ref": "string",
  "discharged": true
}
```

**Authentication:** Required.
**Authorization:** Belief must exist. Principal must exist (FK check).
**Idempotency:** Per-belief replay protection via UNIQUE(belief_id, obligation_key, instrument_ref). Duplicate → `ErrDuplicateDischarge`.
**Kernel interaction:** `kernel.Discharge`.

---

#### GET `/v1/activity` — Get Activity

**Purpose:** Retrieve the append-only activity/audit log for a scenario.

**Query parameters:** `scenario_id` (required), `type` (optional filter), `limit` (default 100), `offset` (default 0).

**Response:**
```json
{
  "activities": [
    {
      "id": "uuid",
      "scenario_id": "uuid",
      "type": "authorization_granted",
      "actor_id": "uuid",
      "subject_id": "uuid",
      "details": {},
      "sqlstate": "23503",
      "constraint_name": "gate",
      "refusal": false,
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "total": 42
}
```

**Authentication:** Required.
**Authorization:** Read-only. Scoped to the caller's scenario.
**Kernel interaction:** `service/audit.GetActivities` (read-only).

---

#### GET `/v1/beliefs/{id}/explain` — Explain Belief

**Purpose:** Explain whether a belief is promotable or authorizable. Read-only projection.

**Response:**
```json
{
  "belief_id": "uuid",
  "claim": "string",
  "status": "entered",
  "can_promote": false,
  "can_authorize": false,
  "remaining_debt": ["needProvenanceCheck"],
  "live_intents": 0,
  "evidence_count": 2,
  "summary": "Belief has 1 remaining debt item and cannot be promoted."
}
```

**Authentication:** Required.
**Authorization:** Read-only. No Solvent authority required.
**Kernel interaction:** Derived from belief + evidence + intent state.

---

#### GET `/v1/ledger` — Get Ledger Summary

**Purpose:** Read the current ledger state: beliefs, evidence, intents, audit counts.

**Query parameters:** `scenario_id` (required).

**Response:**
```json
{
  "scenario_id": "uuid",
  "belief_count": 5,
  "evidence_count": 12,
  "promoted_count": 2,
  "live_intent_count": 1,
  "retracted_count": 0,
  "live_on_nonpromoted": 0
}
```

**Authentication:** Required.
**Authorization:** Read-only. `live_on_nonpromoted` must always be 0 (invariant I-5).

---

## 6. Authentication and Actor Model

### 6.1 Core Principle

```
ACTOR TYPE  !=  IDENTITY  !=  AUTHENTICATION
```

`actor_type=HUMAN` in a request body is not proof of identity. It is caller-declared metadata.

### 6.2 Actor Types

| Type | Meaning | API Usage | Trust Level |
|------|---------|-----------|-------------|
| `human` | A human operator | Attribution in principal, target creation, approval | Caller assertion (not proof) |
| `agent` | An autonomous agent | Attribution in intent creation, evidence submission | Caller assertion (not proof) |
| `workload` | A CI/CD pipeline or automated system | Attribution in target creation | Caller assertion (not proof) |
| `service` | A Solvent service component | Internal use | System-established |

### 6.3 Authentication Boundary

Authentication belongs at the API/service boundary, not in the kernel. The API layer must provide an `AuthenticatedPrincipal` abstraction that supports:

- **API keys** — simple key-based auth for v0
- **OAuth/OIDC** — token-based auth for human users
- **Service identity** — mTLS or workload identity for service-to-service
- **Future attestation** — cryptographic proof of agent identity

The kernel never sees authentication tokens. The API layer translates authentication into an `AuthenticatedPrincipal` and passes it as an attribution field.

### 6.4 What the API Accepts as Trustworthy

| Signal | Trustworthy? | Why |
|--------|-------------|-----|
| Bearer token / API key | Yes (at authentication boundary) | Proves the caller authenticated |
| `actor_type` in request body | **No** | Caller assertion, not proof |
| `actor_id` in request body | **No** | Caller assertion, not proof |
| `action_source` in request body | **No** (caller assertion) | Cannot prove human actually typed it |
| Server-established principal | Yes | Set by the API layer from authentication |
| `approved_by` in Approve request | **Partial** | Caller asserts; kernel verifies the principal is not revoked |

### 6.5 Authentication Abstraction

```go
type AuthenticatedPrincipal struct {
    PrincipalID string    // server-established, not caller-supplied
    PrincipalType string  // derived from authentication mechanism
    AuthMethod  string    // "api_key", "oidc", "mtls", etc.
    AuthTime    time.Time // when authentication occurred
}
```

The `PrincipalID` is set by the API layer from the authentication mechanism, NOT from the request body. The request body's `actor_id` is a separate attribution field that identifies who the caller claims to be acting for.

### 6.6 Impersonation Handling

The API does not support impersonation. The `AuthenticatedPrincipal` is established once at the request boundary and cannot be overridden by the caller. If a caller needs to act on behalf of another principal, the service layer must explicitly support delegation (not in v0).

### 6.7 Service-to-Service Calls

Service-to-service calls use service identity (mTLS or workload identity). The authenticated principal type is `service`. The `actor_id` attribution field identifies the downstream service.

### 6.8 Agent Calls

Agent calls use the same authentication mechanism as human calls (API key or OAuth). The `actor_type` attribution is `agent`. The kernel does not distinguish human from agent callers — the authority model is identity-based, not role-based.

### 6.9 Administrative Calls

Administrative operations (Approve, RevokeTarget, CreatePrincipal, RevokePrincipal) require privileged access. The API layer enforces this through role-based access control at the authentication boundary, not through the kernel.

---

## 7. Authorization Semantics

### 7.1 Five Layers — Do Not Collapse

| Layer | What It Controls | Enforced By | Can Deny | Can Grant |
|-------|-----------------|-------------|----------|-----------|
| **Authentication** | Is the caller who they claim to be? | API boundary (token, key, OIDC) | Yes | No |
| **API Access Control** | Is the authenticated caller allowed to call this endpoint? | API/service layer (role, capability) | Yes | No |
| **Solvent Authority** | Is the execution-time tuple backed by current approved authority? | `kernel.Authorize` | **Yes** | **Yes** |
| **Policy** | Does the tool/actor/class satisfy policy constraints? | `service/policy.EvaluateConstraints` | Yes (veto only) | No |
| **Execution Authorization** | Is the executor allowed to run for this principal/target? | Future: `service/authority.ExecuteAction` | Yes | No |

### 7.2 The Authorization Flow

```
API request arrives
    ↓
Authentication boundary
    ↓ establish AuthenticatedPrincipal
API access control
    ↓ is caller allowed to call this endpoint?
Policy evaluation (optional)
    ↓ can this actor use this tool at this stage?
Construct AuthorityTuple
    ↓ from request context + hardcoded fields
Re-read current state from DB
    ↓ kernel.Authorize(ctx, targetID, tuple)
    ↓ field-by-field match against live snapshot
Allow or Deny
    ↓
Audit log
```

### 7.3 Never

```
API
    ↓
cached approval
    ↓
execute
```

```
API
    ↓
caller says approved=true
    ↓
execute
```

```
API
    ↓
policy ALLOW
    ↓
execute (without kernel.Authorize)
```

### 7.4 Policy as Veto, Not Grant

`service/policy.EvaluateConstraints` returns `PolicyConstraints` with `RequiresAuthority: true` (always). Policy can set `RequiresAuthority: false` when it hard-denies (tool not found, actor not found, actor deactivated, class exceeded). In that case, calling `kernel.Authorize` is pointless — policy already blocks.

But policy **cannot** set `Allowed: true` and bypass `kernel.Authorize`. The kernel is always the final oracle.

---

## 8. Forbidden API Surface

### Explicitly Prohibited Endpoints

| Forbidden Pattern | Why It Is Forbidden |
|-------------------|---------------------|
| `/execute-without-authority` | Bypasses `kernel.Authorize`. Violates "current authority wins." |
| `/set-authorized` | Caller-controlled authority state. Violates "kernel is the final authority oracle." |
| `/approve-with-agent` | Agent-initiated approval. Violates "exact action/target binding is mandatory." |
| `/update-authority-state` | Mutable authority. Violates "append-only authority lifecycle." |
| `/force-execution` | Bypasses authorization. Violates "no production execution path may bypass kernel.Authorize." |
| `/override-authorization` | Administrative bypass. Violates "database constraints are the final authority boundary." |
| `/set-actor` | Caller-controlled actor identity. Violates "actor identity is server-established." |
| `/impersonate` | Identity spoofing. Violates "authentication belongs at the API boundary." |
| `/approve-and-execute` | Combines approval and execution. Violates "authorize != execute." |
| `/execute-as-approved` | Fakes approval state. Violates "current authority wins." |
| `/trust-tool-output` | Treats tool output as authority. Violates "retrieval is not authority." |
| `/cache-authorization` | Caches authority decisions. Violates "current-state revalidation." |

### Forbidden Request Patterns

| Forbidden Pattern | Why |
|-------------------|-----|
| Caller supplies `approved=true` in request | Authority state is DB-derived, not caller-supplied |
| Caller supplies `authority_state` in request | Same as above |
| Caller supplies `debt=[]` to force promotion | Debt state is DB-derived |
| Caller supplies `status=promoted` in create request | Status transitions go through Promote, not set |
| Caller supplies `executor` in execution request | Executor is registry-resolved, not caller-selected |
| Caller supplies `evidence_verification=verified` | Verification is advisory, not authority |

### Design Rule

The API must make dangerous semantics structurally difficult or impossible to express. Do not rely on documentation alone. If a forbidden operation can be accidentally constructed by a valid request shape, the schema is wrong.

---

## 9. Request/Response Conventions

### 9.1 IDs

- All IDs are UUIDs (v4, generated by CockroachDB `gen_random_uuid()`)
- Format in JSON: `"uuid": "550e8400-e29b-41d4-a716-446655440000"`
- Format in URLs: path parameter `{id}`
- Never expose auto-increment integers

### 9.2 Timestamps

- Format: ISO 8601 / RFC 3339 with timezone: `"2026-01-01T00:00:00Z"`
- Always UTC
- Field naming: `*_at` suffix (e.g., `created_at`, `approved_at`, `revoked_at`)

### 9.3 Version Fields

- No version field on individual resources in v0
- API version is in the URL prefix: `/v1/`
- Additive evolution: new optional fields, new endpoints
- Breaking changes require `/v2/`

### 9.4 Enums

- Lowercase strings matching database values: `"entered"`, `"promoted"`, `"retracted"`
- Or explicit string literals in the API schema
- No integer enum mappings

### 9.5 Nullable Fields

- Use JSON `null` or omit the field
- Never use empty string `""` for "not present"
- Arrays: use `[]` instead of `null` for empty collections

### 9.6 Pagination

```
GET /v1/beliefs?scenario_id=uuid&limit=20&offset=0
```

Response includes:
```json
{
  "items": [...],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

### 9.7 Filtering

- Query parameters for simple filters: `?status=promoted&type=external_feed`
- No complex filter expressions in v0
- Scenario scoping is mandatory for all reads

### 9.8 Sorting

- Default: `created_at DESC` (newest first)
- Allow `?sort=created_at&order=asc` for specific endpoints
- No arbitrary sort fields in v0

### 9.9 Field Naming

- Snake_case for all JSON fields: `belief_id`, `scenario_id`, `content_sha256`
- No camelCase in API responses
- Consistent with database column naming

### 9.10 Metadata / Provenance

- Evidence carries `provenance_class` and `content_sha256`
- Authority carries `approved_at`, `approver_principal_id`, `snapshot_hash`
- All mutations are attributed via `*_by` fields (UUID references to principals)

### 9.11 Correlation / Request IDs

- Every response includes `X-Request-ID` header
- Client may supply `X-Request-ID` in request for tracing
- Used for audit correlation

### 9.12 Idempotency Keys

- Not required in v0
- Future: `Idempotency-Key` header for POST operations
- Client should handle duplicate creation responses

### 9.13 Optimistic Concurrency

- Not required in v0
- CockroachDB serializable transactions handle concurrent mutations
- Future: `If-Match` / ETag for conflict detection on mutable resources

---

## 10. Error Contract

### 10.1 Error Response Structure

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": {
      "sqlstate": "23503",
      "constraint": "gate",
      "field": "belief_id"
    },
    "retryable": false,
    "request_id": "uuid"
  }
}
```

### 10.2 Error Code Taxonomy

| HTTP Status | Error Code | Meaning | Safe to Expose | Agent-Actionable |
|-------------|-----------|---------|----------------|-----------------|
| 400 | `malformed_request` | Request body is invalid JSON or missing required fields | Yes | Fix request |
| 400 | `invalid_uuid` | A UUID field contains a non-UUID value | Yes | Fix identifier |
| 401 | `authentication_required` | No valid credentials provided | Yes | Authenticate |
| 401 | `authentication_invalid` | Credentials are invalid or expired | Yes | Re-authenticate |
| 403 | `access_denied` | Authenticated caller lacks permission for this endpoint | Yes | Check permissions |
| 404 | `not_found` | Resource does not exist | Yes | Verify resource ID |
| 409 | `conflict` | Resource state conflict (e.g., already activated) | Yes | Check current state |
| 422 | `invariant_violation` | Database constraint refused the operation | Yes | See details |
| 422 | `authority_denied` | `kernel.Authorize` returned Allowed=false | Yes | See reason |
| 422 | `policy_denied` | Policy constraints blocked the operation | Yes | See reasons |
| 500 | `internal_error` | Unexpected server error | No | Retry |
| 503 | `service_unavailable` | Service is temporarily unavailable | Yes | Retry with backoff |

### 10.3 SQLSTATE Surfacing

When a database constraint refusal occurs, the API surfaces:

```json
{
  "error": {
    "code": "invariant_violation",
    "message": "promotion blocked: open debt or final-truth language",
    "details": {
      "sqlstate": "23514",
      "constraint": "promoted_is_debt_free"
    }
  }
}
```

SQLSTATE codes and constraint names are engine output and must be surfaced verbatim. Do not replace them with generic UI prose.

### 10.4 Authorization Denial Structure

```json
{
  "error": {
    "code": "authority_denied",
    "message": "authority denied: principal mismatch",
    "details": {
      "target_id": "uuid",
      "expected_principal": "uuid",
      "provided_principal": "different-uuid"
    }
  }
}
```

The denial must explain:
- WHAT was requested
- WHY it was denied
- WHICH authoritative fact caused the denial

### 10.5 Retry Semantics

| Error Code | Retryable | Strategy |
|-----------|-----------|----------|
| `malformed_request` | No | Fix request |
| `authentication_required` | No | Authenticate first |
| `access_denied` | No | Check permissions |
| `not_found` | No | Verify resource |
| `conflict` | No | Check current state |
| `invariant_violation` | No | Read-only refusal |
| `authority_denied` | No | Re-read authority, fix tuple |
| `40001` (serialization) | **Yes** | Auto-retry in crdb.ExecuteTx |
| `internal_error` | Yes (with backoff) | Exponential backoff |
| `service_unavailable` | Yes (with backoff) | Exponential backoff |

### 10.6 What is NOT Exposed

- Internal error messages from Go panics
- Stack traces
- Database connection details
- Internal SQL queries
- Kernel implementation details
- Schema migration state

---

## 11. Idempotency and Concurrency

### 11.1 Idempotency by Operation

| Operation | Idempotent? | Mechanism |
|-----------|------------|-----------|
| Enter Belief | No | No unique constraint on claim (v0 limitation) |
| Add Evidence | No | No uniqueness on evidence (v0 limitation) |
| Retire Debt | Yes | `array_remove` on absent item is no-op |
| Promote Belief | Yes | UPDATE on same status is no-op |
| Retract Belief | Yes | Retracted belief stays retracted |
| Create Principal | No | No unique constraint on type+issuer (v0 limitation) |
| Revoke Principal | Yes | `WHERE revoked_at IS NULL` guard |
| Create Target | No | No unique constraint on target dimensions (v0 limitation) |
| Attach Justification | Yes | `ON CONFLICT DO NOTHING` |
| Request Authorization | Partial | Re-pinning recomputes hash; idempotent if nothing changed |
| Approve Target | No | UNIQUE(target_id) prevents second approval |
| Verify Authorization | Yes | Read-only, no side effects |
| Authorize Action | Partial | Authority check is idempotent; intent creation is not |
| Revoke Target | Yes | PK prevents duplicate; `ErrAlreadyRevoked` |
| Discharge Debt | Yes | UNIQUE(belief_id, obligation_key, instrument_ref) |

### 11.2 Duplicate Request Handling

For non-idempotent operations, the API returns the original error (e.g., `ErrDuplicateDischarge`, UNIQUE violation). The client should not retry blindly.

### 11.3 Concurrent Approval

Two concurrent `Approve` calls on the same target:
- CockroachDB serializable transaction ensures exactly one succeeds
- The second receives UNIQUE violation (SQLSTATE 23505)
- This is correct behavior, not a bug

### 11.4 Concurrent Justification Attachment + Approval

`AttachJustification` and `Approve` both lock the `authority_target` row with `SELECT ... FOR UPDATE`. This serializes the race between attaching a justification and approving.

### 11.5 DB Atomicity vs External Side Effects

**DB atomicity:** All writes within `crdb.ExecuteTx` are serializable. If any statement fails, the entire transaction rolls back.

**DB + external side effects:** There is NO atomic guarantee between DB state and external provider side effects. The existing TOCTOU limitation applies:

> Solvent may establish valid CURRENT authorization immediately before an external executor invocation, but it does not claim an atomic DB + provider transaction.

### 11.6 TOCTOU Limitation

The `ExecuteAction` path re-validates authority at execution time (`PrepareForAction` is called immediately before executor invocation). Between the authority check and the executor invocation, the authority could theoretically be revoked. This is an accepted v0 limitation documented in the codebase.

In v0, the executor registry is empty, so this window has no practical consequence. When real executors are introduced, the TOCTOU window must be explicitly documented as a deployment constraint.

---

## 12. Versioning

### 12.1 Strategy

- URL prefix versioning: `/v1/`
- Simplest durable approach
- No header negotiation in v0

### 12.2 Additive Evolution

- New optional fields in responses: non-breaking
- New query parameters: non-breaking
- New endpoints: non-breaking
- New enum values in requests: breaking (require `/v2/`)

### 12.3 Breaking Changes

- Removing fields from responses
- Renaming fields
- Changing field types
- Changing enum values
- Changing required/optional semantics

### 12.4 Deprecation

- Deprecated fields: include in response with `deprecated: true` annotation
- Deprecated endpoints: return `Sunset` header with date
- Minimum deprecation period: 6 months

### 12.5 MCP Schema Evolution

MCP tool schemas are defined inline in Go code. Schema evolution follows the same additive rules. Breaking changes to MCP tool inputs require a new tool version.

### 12.6 A2A Compatibility

A2A protocol compatibility is a separate concern from REST API versioning. A2A mappings are adapters, not versioned contracts.

### 12.7 SDK Compatibility

SDKs are generated from OpenAPI. SDK versioning is independent of API versioning. SDK v1.x supports API `/v1/`.

---

## 13. MCP Mapping

### 13.1 Design Principle

MCP is an adapter into Solvent. It must map to the same service semantics as REST. No MCP tool may create an alternate authorization path.

### 13.2 Operation Mapping

| REST Operation | MCP Tool | Service Operation | Kernel Interaction |
|---------------|----------|-------------------|-------------------|
| `POST /v1/beliefs` | `solvent_ledger` (read) | `view.GetSnapshot` | Read-only |
| `POST /v1/beliefs/{id}/promote` | `solvent_promote` | `kernel.Promote` | DB CHECK gate |
| `POST /v1/beliefs/{id}/debt/retire` | `solvent_retire_debt` | `kernel.RetireDebt` | array_remove |
| `POST /v1/beliefs/{id}/retract` | `solvent_falsify` | `kernel.RetractCascade` | Cancel+retract |
| `POST /v1/authorizations/action` | `solvent_authorize_action` | `authSvc.PrepareForAction` → `kernel.IntentOnPromoted` | kernel.Authorize + intent |
| `POST /v1/principals` | `solvent_create_principal` | `kernel.CreatePrincipal` | INSERT |
| `POST /v1/principals/{id}/revoke` | `solvent_revoke_principal` | `kernel.RevokePrincipal` | UPDATE |
| `POST /v1/targets` | `solvent_create_target` | `kernel.CreateTarget` | INSERT |
| `POST /v1/targets/{id}/justifications` | `solvent_attach_justification` | `kernel.AttachJustification` | INSERT |
| `POST /v1/targets/{id}/request` | `solvent_request_authorization` | `kernel.RequestAuthorization` | UPDATE |
| `POST /v1/targets/{id}/approve` | `solvent_approve` | `kernel.Approve` | INSERT snapshot+activation |
| `POST /v1/authorizations/verify` | `solvent_authorize` | `kernel.Authorize` | Read-only verification |
| `POST /v1/targets/{id}/revoke` | `solvent_revoke_target` | `kernel.RevokeTarget` | INSERT revocation |
| `POST /v1/discharge` | `solvent_discharge` | `kernel.Discharge` | INSERT+UPDATE |
| `GET /v1/beliefs/{id}/explain` | `solvent_explain` | `view.ExplainSnapshot` | Read-only |
| `GET /v1/activity` | (none in v0) | `audit.GetActivities` | Read-only |

### 13.3 MCP-Specific Enforcement

The MCP layer adds one enforcement beyond the kernel:

**`action_source` gate:** `solvent_authorize_action` refuses `"tool_output"` before any database access. This is an MCP-layer enforcement — the kernel has no concept of `action_source`. The refusal uses `errorResult` (no audit envelope), making the DB-free path observable.

### 13.4 MCP Limitations

- No HTTP transport (stdio only in v0)
- No authentication (trusted administrative surface)
- No pagination (all results returned)
- No filtering (scenario-scoped)
- Scenario names are hardcoded enum

---

## 14. A2A Mapping

### 14.1 Current Status

No A2A implementation exists in the repository. This section defines the intended mapping for when A2A is implemented.

### 14.2 Design Principle

A2A must map to the same service semantics as REST and MCP. No A2A agent may create an alternate authorization path.

### 14.3 Intended Mapping

| REST Operation | A2A Representation | Service Operation | Kernel Interaction |
|---------------|--------------------|--------------------|-------------------|
| `POST /v1/authorizations/action` | Agent task with authority verification | `authSvc.PrepareForAction` | kernel.Authorize |
| `POST /v1/beliefs` | Agent task for belief creation | `kernel.EnterBelief` | INSERT |
| `POST /v1/evidence` | Agent task for evidence submission | `kernel.AddEvidence` | INSERT |
| `GET /v1/beliefs/{id}/explain` | Agent query for belief explanation | `view.ExplainSnapshot` | Read-only |

### 14.4 Security Constraints

- A2A agents must authenticate at the API boundary
- A2A agent tasks that cross the authority boundary must go through `kernel.Authorize`
- A2A agent tasks must not bypass the `action_source` gate
- A2A must not introduce workflow tokens, approval queues, or alternate authority semantics

---

## 15. Web UI Boundary

### 15.1 Design Principle

The Web UI is a client of the canonical API. It does not define the API. Removing the Web UI must not weaken the MCP/A2A/API capabilities or authority model.

### 15.2 UI Requirements vs API Requirements

| UI Need | Where It Belongs | Why |
|---------|-----------------|-----|
| Dashboard with belief counts | API: `GET /v1/ledger` | Read model, not authority |
| Belief detail page | API: `GET /v1/beliefs/{id}` + `GET /v1/beliefs/{id}/evidence` | Read model |
| Promotion button | API: `POST /v1/beliefs/{id}/promote` | Delegates to kernel |
| Authority approval UI | API: `POST /v1/targets/{id}/approve` | Admin operation |
| Activity timeline | API: `GET /v1/activity` | Audit log |
| Search/filter | API query parameters | Read model |
| Real-time updates | WebSocket/SSE (future) | Presentation layer |
| User authentication | API authentication boundary | Not in kernel |

### 15.3 What the UI Must NOT Do

- Set authority state directly
- Override kernel authorization decisions
- Cache authorization decisions for reuse
- Provide an alternate approval path
- Trust tool output as authority

### 15.4 UI Replaceability

The current demo web app (`demo/cloud/web/`) and wizard (`internal/wizard/`) are legacy/demo layers. They must eventually migrate to consuming the canonical API. The canonical API must not be shaped to match their current implementation.

---

## 16. SDK Strategy

### 16.1 Current State

No SDKs exist. No OpenAPI specification exists.

### 16.2 Sequence

```
Canonical API contract (Phase 4A)
    ↓
OpenAPI specification (Phase 4C)
    ↓
Go SDK (thin wrapper, if demand justifies)
    ↓
Python SDK (thin wrapper, if demand justifies)
    ↓
TypeScript SDK (thin wrapper, if demand justifies)
```

### 16.3 Design Rules

- SDKs are thin façades over the canonical API
- SDKs must never become architectural dependencies
- The canonical API is the source of truth; SDKs are convenience
- No SDK for every language — only where demand justifies
- SDKs are generated from OpenAPI, not hand-written

### 16.4 Justification for Immediate SDKs

None. The canonical API contract must be stable before any SDK is generated. Premature SDK generation creates maintenance burden without user demand.

---

## 17. OpenAPI Structure

### 17.1 Proposed Structure

```yaml
openapi: "3.1.0"
info:
  title: Solvent API
  version: "1.0.0"
  description: Canonical Solvent authority ledger API

tags:
  - name: beliefs
    description: Belief lifecycle operations
  - name: evidence
    description: Evidence submission and retrieval
  - name: authority
    description: Authority target lifecycle (propose, justify, approve, revoke)
  - name: authorization
    description: Authorization verification and action intent creation
  - name: principals
    description: Principal identity management
  - name: discharge
    description: Debt discharge operations
  - name: activity
    description: Audit activity log
  - name: ledger
    description: Ledger summary and projections
```

### 17.2 Resource/Operation Mapping

Each tag groups related operations. Every operation has:
- Request schema
- Response schema
- Error responses (400, 401, 403, 404, 409, 422, 500)
- Security requirement (bearer token)
- Idempotency annotation

### 17.3 Schema Conventions

- All IDs: `format: uuid`
- All timestamps: `format: date-time`
- Enums: `type: string` with `enum` array
- Nullable fields: `nullable: true` or `oneOf`
- Arrays: `type: array` with `items` schema

### 17.4 Internal-Only Operations

Operations that should remain internal (not public OpenAPI):
- `kernel.Promote` (exposed via `POST /v1/beliefs/{id}/promote`)
- `kernel.RetireDebt` (exposed via `POST /v1/beliefs/{id}/debt/retire`)
- `kernel.RetractCascade` (exposed via `POST /v1/beliefs/{id}/retract`)
- Policy tool/actor management (admin only)

---

## 18. Audit Contract

### 18.1 Activity Event Shape

Every API mutation produces an `ActivityEntry`:

```json
{
  "id": "uuid",
  "scenario_id": "uuid",
  "type": "authorization_granted",
  "actor_id": "uuid",
  "subject_id": "uuid",
  "details": {
    "target_id": "uuid",
    "action": "deploy"
  },
  "sqlstate": "23503",
  "constraint_name": "gate",
  "refusal": false,
  "created_at": "2026-01-01T00:00:00Z"
}
```

### 18.2 Activity Types

| Type | When | Refusal? |
|------|------|----------|
| `belief_entered` | New belief created | No |
| `belief_promoted` | Belief promoted | No |
| `belief_retracted` | Belief retracted | No |
| `evidence_added` | Evidence recorded | No |
| `debt_retired` | Debt item retired | No |
| `intent_created` | Live action intent created | No |
| `intent_cancelled` | Intent cancelled (retraction) | No |
| `authorization_checked` | `kernel.Authorize` called | No |
| `authorization_granted` | `kernel.Authorize` returned Allowed=true | No |
| `authorization_denied` | `kernel.Authorize` returned Allowed=false | Yes |
| `adapter_invoked` | Executor invoked | No |
| `executor_completed` | Executor succeeded | No |
| `executor_failed` | Executor failed | Yes |
| `executor_denied` | Executor blocked by authorization | Yes |

### 18.3 SQLSTATE Surfacing in Audit

When a database constraint refusal occurs, the audit entry captures:
- `sqlstate`: The 5-character SQLSTATE code (e.g., `"23503"`)
- `constraint_name`: The CHECK constraint name (e.g., `"gate"`)
- `refusal`: `true`

These are engine output and must be surfaced verbatim.

### 18.4 Audit Is Append-Only

`service/audit.Log` performs only INSERT. No UPDATE or DELETE methods exist. The audit trail is permanent.

### 18.5 Compliance Readiness

The audit event shape supports future compliance evidence without creating a compliance subsystem now. Every mutation is attributed, timestamped, and classified. The `sqlstate` and `constraint_name` fields provide machine-readable refusal evidence.

---

## 19. Future Execution Boundary

### 19.1 Current State

No production consequential executor exists. The executor registry is instantiated but empty. `ExecuteAction` is designed and tested but not wired to production.

### 19.2 Intended Future Flow

```
API request
    ↓
ExecutionService
    ↓ current-state revalidation
    ↓ kernel.Authorize
    ↓
Executor (from registry, NOT caller-supplied)
    ↓
External provider
```

### 19.3 What the API Must Prevent

The API design must prevent a future implementer from naturally creating:

```
API → provider
```

or:

```
API → cached approval → provider
```

### 19.4 Design Safeguards

1. **Executor is registry-resolved, not caller-selected.** The `tool_name` parameter selects from a fixed registry. The caller cannot inject an arbitrary executor.

2. **Authority is re-validated at execution time.** `ExecuteAction` calls `PrepareForAction` as its first step. No cached authority is reused.

3. **The executor receives opaque params, not authority state.** The `ActionFunc` signature is `func(ctx, params) (output, error)`. The executor does not receive the `AuthorityTuple`, approval state, or any authority-derivable data.

4. **Execution is logged.** Every executor invocation produces `ActivityAdapterInvoked`, `ActivityExecutorCompleted`, or `ActivityExecutorFailed` audit entries.

### 19.5 Arbitrary Executor Selection

Allowing callers to specify which executor runs would be dangerous. It would permit:
- Execution against unauthorized providers
- Execution with elevated privileges
- Execution bypassing policy constraints

The registry is server-side and trusted. The caller selects the `tool_name` from a fixed vocabulary, not the executor implementation.

### 19.6 DO NOT Implement Now

Phase 4A designs the boundary. Phase 4B+ implements real executors. The API contract must not assume execution exists, but must not accidentally make safe future execution impossible.

---

## 20. Database Impact

### 20.1 API Resources Served from Existing Data

| API Resource | Source Table(s) | New Queries Needed? |
|-------------|----------------|-------------------|
| Belief | `belief` | Yes (list with pagination) |
| Evidence | `evidence` | Yes (list by belief) |
| Action Intent | `action_intent` | Yes (list by scenario) |
| Principal | `principal` | Yes (list) |
| Authority Target | `authority_target` | Yes (list with state) |
| Authority (Snapshot + Activation) | `target_snapshot`, `target_activation` | Yes (join for authority state) |
| Revocation | `target_revocation` | Yes (join for revocation state) |
| Justification | `justification` | Yes (list by target) |
| Debt Discharge | `debt_discharge` | Yes (list by belief) |
| Activity | `audit_activity` | Yes (list with filtering) |

### 20.2 No Authority-Core Schema Changes

Phase 4A does NOT add any new tables to the authority core. All API resources are served from existing tables through new read queries or derived projections.

### 20.3 Read Model Additions

The API may need a lightweight read model layer (materialized views or application-level projections) for:
- Belief summaries (evidence count, intent count)
- Target state derivation (proposed/requested/approved/revoked)
- Ledger summaries (counts, aggregates)

These are product/read-model concerns, not authority-core concerns. They can be implemented as:
- Application-level queries joining existing tables
- Future materialized views if performance requires

### 20.4 No Migrations Required for Phase 4A

The 7 existing migrations cover all required tables. No new migrations are needed for the API contract design.

### 20.5 Future Migrations (if needed in Phase 4B+)

| Potential Migration | Why | Authority-Core? | Derivable? | Deferrable? |
|--------------------|-----|-----------------|------------|-------------|
| Read model tables | API pagination/performance | No (product) | Yes (from existing tables) | Yes |
| Authentication table | API key storage | No (service) | No | Yes (v0 can use env/config) |
| Rate limiting table | API throttling | No (service) | No | Yes |

---

## 21. Integration Model

### 21.1 Design Principle

External integrations (GitHub, CI/CD, deployment systems) consume the canonical API. Provider-specific semantics stay outside the API.

### 21.2 Integration Architecture

```
External System (GitHub, CI/CD, etc.)
    ↓
Integration Adapter (provider-specific)
    ↓
Canonical Solvent API
    ↓
Service Layer
    ↓
Kernel
```

### 21.3 GitHub Integration Example

The existing `adapter/github/` package parses webhooks and normalizes events. In the canonical API model:

- GitHub webhook → adapter normalizes to Solvent evidence format → `POST /v1/evidence`
- GitHub issue state → adapter creates belief → `POST /v1/beliefs`
- GitHub PR merge → adapter retires debt → `POST /v1/beliefs/{id}/debt/retire`

The adapter handles GitHub-specific auth, retry, and fallback. The API sees only Solvent domain objects.

### 21.4 Provider-Semantic Isolation

The API must not contain:
- GitHub-specific field names
- CI/CD-specific workflow states
- Deployment-specific action types

These belong in the adapter layer. The API operates on Solvent domain concepts only.

---

## 22. Reference Workflows

### 22.1 Evidence Submission

```
Caller (Agent/Human)
    → POST /v1/evidence (scenario_id, belief_id, provenance_class, content_sha256)
    → API: authenticate, validate request
    → kernel.AddEvidence (INSERT into evidence)
    → audit.Log (ActivityEvidenceAdded)
    → Response: { evidence_id, belief_id, ingested_at }
```

Trust boundary: Authentication at API boundary. Evidence content is caller-supplied; provenance is attribution, not verification.

### 22.2 Human Approval / Authority Creation

```
Admin (Human)
    → POST /v1/targets (create proposal)
    → kernel.CreateTarget (INSERT into authority_target)
    → POST /v1/targets/{id}/justifications (attach promoted beliefs)
    → kernel.AttachJustification (INSERT, ON CONFLICT DO NOTHING)
    → POST /v1/targets/{id}/request (pin proposal hash)
    → kernel.RequestAuthorization (UPDATE with hash pin)
    → POST /v1/targets/{id}/approve (approve)
    → kernel.Approve (SERIALIZABLE: INSERT snapshot + INSERT activation)
    → audit.Log (ActivityAuthorizationGranted)
    → Response: { target_id, approved: true }
```

Trust boundary: `Approve` requires hash-pin verification. The approver principal must exist and not be revoked. All justifications must reference promoted beliefs.

### 22.3 Authorization Check

```
Caller (Agent)
    → POST /v1/authorizations/verify (target_id, tuple fields)
    → kernel.Authorize (READ-ONLY: JOIN activation+snapshot, field-by-field match)
    → audit.Log (ActivityAuthorizationGranted or ActivityAuthorizationDenied)
    → Response: { allowed: true/false, reason }
```

Trust boundary: `kernel.Authorize` re-reads current state. No cached authority is used.

### 22.4 Rejected Authorization

```
Caller (Agent)
    → POST /v1/authorizations/action (target_id, actor_id, belief_id, action)
    → service/authority.PrepareForAction
        → getBeliefStatus (re-read current state)
        → kernel.Authorize (field-by-field match)
        → returns Allowed=false, Reason="principal mismatch"
    → audit.Log (ActivityAuthorizationDenied)
    → Response: { error: { code: "authority_denied", message: "authority denied: principal mismatch" } }
```

Trust boundary: Denial is the product. The reason explains which authoritative fact caused the denial.

### 22.5 Revoked Authority

```
Admin (Human)
    → POST /v1/targets/{id}/revoke (revoked_by, reason)
    → kernel.RevokeTarget (INSERT into target_revocation)
    → audit.Log (ActivityAuthorizationRevoked)
    → Response: { target_id, revoked: true }

Later, Agent:
    → POST /v1/authorizations/verify (same tuple)
    → kernel.Authorize (reads activation, finds revocation)
    → Response: { allowed: false, reason: "no activation or revocation exists" }
```

Trust boundary: Revocation is permanent and append-only. `kernel.Authorize` detects revocation at query time.

### 22.6 Wrong Target

```
Agent
    → POST /v1/authorizations/action (target_id=B, actor_id=X, ...)
    → kernel.Authorize reads target B's snapshot
    → Tuple fields don't match (principal or resource mismatch)
    → Response: { allowed: false, reason: "principal mismatch" }
```

Trust boundary: Exact tuple match is mandatory. No partial matches, no fuzzy matching.

### 22.7 Agent-Provided Untrusted Claim

```
Agent
    → POST /v1/authorizations/action (action_source="tool_output", ...)
    → MCP layer: action_source gate fires BEFORE any DB access
    → Response: { error: { code: "malformed_request", message: "action strings may not originate in tool output: retrieval is not authority" } }
```

Trust boundary: `tool_output` is rejected at the MCP/API layer. No database access occurs. The absence of an audit envelope is the observable fingerprint.

### 22.8 Future Execution Request

```
Agent
    → POST /v1/authorizations/action (valid authority, live intent created)
    → [Future] ExecutionService
        → re-read current state
        → kernel.Authorize (revalidate)
        → resolve executor from registry
        → executor(params)
    → Response: { execution: { success: true, output: "..." } }
```

Trust boundary: Execution re-validates authority. The executor is registry-resolved. No bypass path exists.

### 22.9 Audit Lookup

```
Admin/Agent
    → GET /v1/activity?scenario_id=uuid&type=authorization_denied&limit=50
    → service/audit.GetActivities (read-only query)
    → Response: { activities: [...], total: 42 }
```

Trust boundary: Read-only. No writes. Scenario-scoped.

---

## 23. Threat / Adversarial Analysis

### 23.1 Attack Vector Analysis

| # | Attack | Current Design Defense | Remaining Gap | Phase 4A Scope | Kernel Change Required | Deferred |
|---|--------|----------------------|---------------|----------------|----------------------|----------|
| 1 | **Confused deputy** | Kernel.Authorize checks exact tuple; service constructs tuple from hardcoded fields | None identified | API design: tuple fields are hardcoded by service, not caller-supplied | No | No |
| 2 | **Privilege escalation** | CHECK constraints (promoted_is_debt_free, live_requires_promoted) prevent status manipulation | None identified | API does not expose status-setting endpoints | No | No |
| 3 | **Caller-controlled authority** | Authority state is DB-derived; caller cannot set approved=true | None identified | API has no endpoint to set authority state directly | No | No |
| 4 | **Stale authorization** | kernel.Authorize re-reads current state at call time | TOCTOU window between Authorize and Execute (accepted v0 limitation) | API documents TOCTOU limitation | No | Yes (TOCTOU) |
| 5 | **Replay** | UNIQUE constraints prevent duplicate activation, discharge | CreatePrincipal/CreateTarget not idempotent (v0 limitation) | API documents non-idempotent operations | No | Yes (v0 limitation) |
| 6 | **Duplicate approval** | UNIQUE(target_id) on target_activation | None identified | Approve is structurally once-ever | No | No |
| 7 | **Wrong-target execution** | Exact tuple match includes resource_id | None identified | Authorize checks all 8 tuple fields | No | No |
| 8 | **Wrong-actor execution** | Exact tuple match includes principal_id | None identified | Authorize checks all 8 tuple fields | No | No |
| 9 | **Forged approval** | Hash pin mechanism (SHA-256 over proposal+justifications) | None identified | Approve recomputes hash and compares to pinned hash | No | No |
| 10 | **Forged evidence** | Evidence is attributed, not verified; promotion requires debt-free | Evidence quality is advisory, not authority | API surfaces evidence as fact, not authority | No | No |
| 11 | **Agent claims as authority** | action_source gate rejects tool_output; actor_type is caller assertion | action_source is self-declared, not cryptographically proven | API acknowledges limitation | No | Yes (future attestation) |
| 12 | **Cached authorization** | No cache exists; kernel.Authorize is always fresh | None identified | API has no caching layer | No | No |
| 13 | **UI-controlled auth** | UI is a client; it cannot set authority state | None identified | API does not expose authority-state-setting endpoints | No | No |
| 14 | **MCP bypass** | MCP maps to same service semantics; no alternate auth path | action_source is MCP-layer only (not in kernel) | API documents MCP enforcement | No | No |
| 15 | **A2A bypass** | A2A must map to same service semantics (design constraint) | Not yet implemented | API design prevents alternate paths | No | Yes (A2A impl) |
| 16 | **REST bypass** | REST maps to same service semantics | None identified | Canonical API is the single surface | No | No |
| 17 | **Arbitrary executor selection** | Executor is registry-resolved, not caller-selected | Registry is empty in v0 | API does not expose executor selection | No | Yes (real executors) |
| 18 | **Hidden provider side effect** | ExecuteAction logs adapter invocation + executor result | No real executors in v0 | API documents execution boundary | No | Yes (real executors) |
| 19 | **Auth/execution confusion** | Authorize is read-only; ExecuteAction calls PrepareForAction | None identified | API separates verify from action | No | No |
| 20 | **Schema ambiguity** | CHECK constraints enforce non-empty strings; enums restrict values | None identified | API request validation mirrors DB constraints | No | No |
| 21 | **Type confusion** | UUID format validation; JSON validation on consequence_parameters | None identified | API validates types at boundary | No | No |
| 22 | **Insecure default** | All defaults are safe (debt array, entered status, false final_truth) | None identified | API does not override defaults | No | No |
| 23 | **Missing auth boundary** | All mutating endpoints require authentication | Authentication not yet implemented (v0 uses trusted surface) | API design requires authentication | No | Yes (auth impl) |
| 24 | **Accidental admin bypass** | Approve, RevokeTarget, CreatePrincipal are admin-only | Admin enforcement at API layer, not kernel | API role-based access control | No | Yes (auth impl) |

### 23.2 Accepted V0 Gaps

| Gap | Risk | Mitigation |
|-----|------|-----------|
| CreatePrincipal not idempotent | Duplicate principals possible | Deploy as trusted surface; no public create |
| CreateTarget not idempotent | Duplicate targets possible | Deploy as trusted surface |
| TOCTOU between Authorize and Execute | Authority could be revoked after check | Executor registry is empty in v0; documented limitation |
| action_source is self-declared | Agent can claim user_typed falsely | Database gate is the real boundary; action_source catches honest misuse |
| Revoked principal can Discharge (T-27) | FK-valid but revocation not enforced | v0 limitation; documented |
| Retract-repromote can revive justification (T-23) | Old justification may become valid again | v0 limitation; documented as ACCEPTED SECURITY GAP |

---

## 24. Test Matrix

### 24.1 API Contract Tests

| Test Category | What to Test | Priority |
|--------------|-------------|----------|
| **Request validation** | Missing required fields, malformed UUIDs, invalid enums, empty strings | High |
| **Response schema** | Every response matches OpenAPI schema | High |
| **Idempotency** | Idempotent operations return same result on retry | High |
| **SQLSTATE preservation** | Every DB refusal surfaces correct SQLSTATE and constraint name | High |
| **Error contract** | Every error response matches error schema | High |

### 24.2 Authentication Tests

| Test | What to Test |
|------|-------------|
| Missing credentials | 401 response |
| Invalid credentials | 401 response |
| Expired credentials | 401 response |
| Valid credentials | 200 response |
| Different auth methods | API key, OAuth, service identity |

### 24.3 Authorization Tests

| Test | What to Test |
|------|-------------|
| Read without authority | Succeeds (read-only operations) |
| Write without authority | Succeeds for evidence/belief; fails for authority operations |
| Verify with exact tuple | Allowed=true |
| Verify with wrong principal | Allowed=false, reason="principal mismatch" |
| Verify with wrong resource | Allowed=false |
| Verify with wrong scope | Allowed=false |
| Verify with wrong action | Allowed=false |
| Verify with wrong consequence | Allowed=false |
| Verify on revoked target | Allowed=false |
| Verify on unactivated target | Allowed=false |
| Approve without pin | ErrAuthorizationMissing |
| Approve with hash mismatch | ErrApprovalPinMismatch |
| Approve without justifications | ErrInvalidProposal |
| Approve with unpromoted justification | ErrBeliefNotPromoted |
| Approve with revoked approver | ErrRevokedPrincipal |
| Second approve | UNIQUE violation |

### 24.4 Actor/Identity Tests

| Test | What to Test |
|------|-------------|
| Agent cannot set actor_type | Actor type is server-established |
| Caller cannot set approved=true | Authority state is DB-derived |
| Caller cannot set status=promoted | Status goes through Promote |
| Caller cannot select executor | Executor is registry-resolved |

### 24.5 Negative Tests

| Test | What to Test |
|------|-------------|
| tool_output rejection | Hard拒绝 before DB access |
| Missing target_id | Fail-closed before authority check |
| Missing actor_id | Fail-closed before authority check |
| Malformed target_id | Fail-closed before authority check |
| Unknown scenario | 404 |
| Unknown belief | 404 |
| Unknown principal | 404 |
| Unknown target | ErrTargetNotFound |

### 24.6 Idempotency Tests

| Test | What to Test |
|------|-------------|
| Retire debt (absent item) | No-op, same result |
| Promote (already promoted) | No-op, same result |
| Retract (already retracted) | No-op, same result |
| Revoke principal (already revoked) | No-op, same result |
| Revoke target (already revoked) | ErrAlreadyRevoked |
| Attach justification (duplicate) | ON CONFLICT DO NOTHING |
| Discharge (duplicate) | ErrDuplicateDischarge |

### 24.7 Concurrency Tests

| Test | What to Test |
|------|-------------|
| Concurrent Approve | Exactly 1 activation |
| Concurrent Approve + AttachJustification | FOR UPDATE serializes |
| Concurrent Discharge | Exactly 1 succeeds |
| Concurrent CreatePrincipal (same type+issuer) | Both succeed (v0 limitation) |

### 24.8 Stale-State Tests

| Test | What to Test |
|------|-------------|
| Authorize after retraction | Allowed=false (FK CASCADE) |
| Authorize after revocation | Allowed=false |
| Execute after Prepare (state changed) | Allowed=false |

### 24.9 MCP Parity Tests

| Test | What to Test |
|------|-------------|
| Every MCP tool maps to canonical API operation | Equivalence |
| MCP action_source gate | Rejects tool_output |
| MCP SQLSTATE preservation | Surfaces in error response |
| MCP fail-closed behavior | All required fields validated |

### 24.10 A2A Parity Tests

Deferred until A2A implementation.

### 24.11 Audit Tests

| Test | What to Test |
|------|-------------|
| Every mutation produces audit entry | Activity logged |
| Refusal produces audit entry with SQLSTATE | LogRefusal called |
| GetActivities returns correct entries | Read-only, scoped |
| Audit is append-only | No UPDATE/DELETE possible |

### 24.12 Regression Tests

| Test | Property Proved |
|------|----------------|
| Caller cannot set authority directly | Authority state is DB-derived |
| Caller cannot bypass kernel.Authorize | No execution path skips authorization |
| UI state cannot authorize | UI is a client |
| MCP cannot create alternate auth path | Same service semantics |
| Agent claims cannot become authority | action_source gate + kernel.Authorize |
| Stale authority is rejected | kernel.Authorize re-reads current state |
| Revoked authority is rejected | Revocation detected at query time |
| Wrong target is rejected | Exact tuple match |
| Wrong actor is rejected | Exact tuple match |
| Malformed identifiers fail safely | Fail-closed before DB access |
| Advisory confidence cannot create authority | Quality scores are read-only projections |

---

## 25. Implementation Plan

### Step 1: Freeze Domain/API Vocabulary

Define canonical resource names, field names, enum values, and error codes.

**Files/packages affected:** New file `api/vocabulary.go` or similar.
**New interfaces:** None.
**New structs:** API-level resource types (Belief, Evidence, etc.).
**Migrations:** None.
**Tests:** Vocabulary consistency tests.
**Security implications:** None.
**Rollback:** Delete vocabulary file.

### Step 2: Define Canonical Schemas

Define request/response JSON schemas for all 26 operations.

**Files/packages affected:** New package `api/schema/` or OpenAPI YAML.
**New structs:** Request/Response types for each operation.
**Migrations:** None.
**Tests:** Schema validation tests.
**Security implications:** None.
**Rollback:** Delete schema files.

### Step 3: Define Service Interfaces

Define Go interfaces that the API layer depends on (replacing direct kernel access).

**Files/packages affected:** New package `api/service/`.
**New interfaces:** `BeliefReader`, `BeliefWriter`, `AuthorityService`, `AuditReader`, etc.
**Migrations:** None.
**Tests:** Interface compliance tests.
**Security implications:** Service interfaces must not expose kernel internals.
**Rollback:** Delete interface definitions.

### Step 4: Define Authentication Boundary

Implement `AuthenticatedPrincipal` abstraction and authentication middleware.

**Files/packages affected:** New package `api/auth/`.
**New structs:** `AuthenticatedPrincipal`, `AuthService`.
**Migrations:** None (v0 uses env/config for API keys).
**Tests:** Authentication tests (missing, invalid, valid).
**Security implications:** Authentication must be required for all mutating endpoints.
**Rollback:** Disable authentication middleware.

### Step 5: Define API Handlers/Adapters

Implement HTTP handlers that map requests to service calls.

**Files/packages affected:** New package `api/handler/` or `api/handler.go`.
**New functions:** One handler per operation.
**Migrations:** None.
**Tests:** Handler tests with mock services.
**Security implications:** Handlers must validate request types, call correct service methods, never trust caller-supplied authority state.
**Rollback:** Disable API routes.

### Step 6: Implement Error Model

Map kernel errors to canonical API error responses.

**Files/packages affected:** New file `api/errors.go`.
**New structs:** `APIError`, error code constants.
**Migrations:** None.
**Tests:** Error mapping tests for every sentinel error.
**Security implications:** Internal errors must not leak. SQLSTATEs must be surfaced verbatim.
**Rollback:** Delete error mapping.

### Step 7: Add OpenAPI Contract

Generate OpenAPI specification from implemented schemas.

**Files/packages affected:** New file `api/openapi.yaml`.
**Migrations:** None.
**Tests:** OpenAPI validation against implemented endpoints.
**Security implications:** None.
**Rollback:** Delete OpenAPI file.

### Step 8: Add MCP Mapping/Parity

Verify MCP tools produce equivalent results to REST endpoints.

**Files/packages affected:** `cmd/solvent-mcp/tools.go` (tests only).
**Migrations:** None.
**Tests:** MCP parity tests comparing REST and MCP responses.
**Security implications:** MCP must not create alternate authorization paths.
**Rollback:** N/A (tests only).

### Step 9: Add A2A Mapping/Parity

Define A2A adapter mapping (design only, no implementation).

**Files/packages affected:** Documentation only.
**Migrations:** None.
**Tests:** None (design phase).
**Security implications:** A2A must map to same service semantics.
**Rollback:** N/A.

### Step 10: Add Contract/Integration Tests

Full integration tests against live CockroachDB.

**Files/packages affected:** New test files in `api/`.
**Migrations:** None.
**Tests:** All tests from §24 test matrix.
**Security implications:** Tests prove security properties.
**Rollback:** N/A.

### Step 11: Adversarial Review

Independent review of the complete API contract.

**Files/packages affected:** None (review only).
**Migrations:** None.
**Tests:** Review findings become test cases.
**Security implications:** Review must approve before Phase 4B.
**Rollback:** N/A.

### Step 12: Proceed to Optional UI Work

Only after adversarial review approval.

**Files/packages affected:** `demo/cloud/web/` and `internal/wizard/` (migration to API).
**Migrations:** None.
**Tests:** UI tests against canonical API.
**Security implications:** UI must consume API, not kernel directly.
**Rollback:** N/A.

---

## 26. Deferred Decisions

| Decision | Why Deferred | Impact on Phase 4A |
|----------|-------------|-------------------|
| Authentication mechanism (API key vs OIDC) | v0 can use trusted surface; auth is deployment concern | API design requires authentication but does not specify mechanism |
| Rate limiting strategy | No users yet; premature | API design does not preclude rate limiting |
| Pagination cursor vs offset | Offset is simpler; cursor is more performant at scale | API design uses offset; cursor is additive evolution |
| WebSocket/SSE for real-time updates | UI is deferred | API design does not preclude real-time |
| Materialized views for read performance | Premature without load data | API design uses application-level queries |
| Multi-tenancy model | v0 is single-tenant | API design scopes all operations to scenario_id |
| A2A protocol version | A2A spec is evolving | API design sketches mapping without commitment |
| SDK language priority | No demand data | API design generates OpenAPI; SDKs follow |
| Compliance schema extensions | Phase 5, not Phase 4A | Audit event shape supports future compliance |
| Workflow token removal from schema | Dead table; removal is separate concern | API design does not reference workflow_token |

---

## 27. Acceptance Criteria

### Final Self-Check

```
[KERNEL]
Does Phase 4A require any kernel change?
NO. All API resources are served from existing tables through new read queries.

[AUTHORITY]
Is there exactly one source of truth for authority?
YES. kernel.Authorize is the sole authority oracle. target_snapshot + target_activation
are the sole authority facts. No cache, no alternate path.

[API]
Can every consequential API operation explain exactly how current authority is checked?
YES. POST /v1/authorizations/action calls service/authority.PrepareForAction which calls
kernel.Authorize with fresh state. POST /v1/authorizations/verify calls kernel.Authorize
read-only. No operation bypasses this path.

[MCP]
Can MCP do anything REST cannot?
NO. MCP tools map to the same service semantics. The only MCP-specific enforcement is the
action_source gate, which is a subset of what REST enforces (REST requires authentication).

[A2A]
Can A2A do anything REST cannot?
NO. A2A is designed as an adapter into the same service semantics. No implementation exists
yet, but the design constraint is explicit.

[UI]
Can removing the UI leave the security model unchanged?
YES. The UI is a client of the canonical API. It does not define or influence the authority model.

[EXECUTION]
Can a future executor be inserted without bypassing kernel.Authorize?
YES. ExecuteAction calls PrepareForAction (which calls kernel.Authorize) as its first step.
The executor is registry-resolved, not caller-selected.

[AGENT SAFETY]
Can untrusted agent/tool output become authority?
NO. action_source="tool_output" is rejected before DB access. kernel.Authorize checks
exact tuple match. Advisory confidence scores are read-only projections.

[IDENTITY]
Can a caller simply claim to be HUMAN or an authorized principal?
YES, they can CLAIM it (actor_type is caller assertion). But the claim does not grant
authority. Authority requires kernel.Authorize with an exact tuple match against an
approved snapshot. The authentication boundary establishes server-verified identity.

[REPLAY]
Are retries and duplicate mutations safe?
PARTIALLY. Idempotent operations (RetireDebt, Promote, Retract, RevokePrincipal, RevokeTarget,
AttachJustification, Discharge) are safe. Non-idempotent operations (CreatePrincipal, CreateTarget,
EnterBelief, AddEvidence) may create duplicates under retry. This is a documented v0 limitation.

[TOCTOU]
Are DB/external-side-effect limits honestly documented?
YES. The TOCTOU window between kernel.Authorize and executor invocation is documented.
The executor registry is empty in v0, making the window non-consequential. When real
executors are introduced, the TOCTOU limitation must be explicitly documented.

[SCHEMA]
Did Phase 4A unnecessarily grow the authority-core schema?
NO. Phase 4A adds zero tables, zero columns, zero constraints. All API resources are served
from existing tables.

[COMPLEXITY]
Is every proposed API concept justified?
YES. Every resource maps to an existing table. Every operation maps to an existing kernel or
service method. No new abstractions are introduced for API aesthetics.

[REFERENCE CONTAMINATION]
Did any proposed resource, operation, token, workflow, approval model,
or authority abstraction originate from DealForge/AegisFlow?
NO. The resource model is derived entirely from Solvent's existing domain. DealForge/AegisFlow
patterns were studied for ergonomics only (resource naming, error typing, append-only audit).
No workflow tokens, approval queues, FSM guards, or alternate authority semantics were imported.

[CURRENT-vs-FUTURE]
Does the proposed API accidentally expose capabilities that do not currently exist,
particularly consequential execution?
NO. The API design documents the future execution boundary (§19) but does not expose
an /execute endpoint. ExecuteAction is designed and tested but not wired to production.
The API contract does not assume execution exists.
```

---

## 28. Final Architecture Rules

These rules are carried forward from the background phase and AGENTS.md, reinforced by the Phase 4A design:

1. **Retrieval is not authority.** The API does not treat evidence, corpus citations, or quality scores as authority.

2. **The kernel is the smallest trusted authority core.** The API does not enlarge the kernel. All new code lives in the API/service layer.

3. **Authority truth remains in the kernel and database invariants.** The API does not cache, recompute, or override authority decisions.

4. **`kernel.Authorize` is the final authority oracle.** Every consequential API operation explains how current authority is checked through kernel.Authorize.

5. **`kernel.Approve` is the sole authority-creating semantic operation.** No API endpoint creates authority except POST /v1/targets/{id}/approve.

6. **`kernel.RevokeTarget` is append-only revocation.** No API endpoint un-revokes authority.

7. **Exact action/target binding is mandatory.** The AuthorityTuple is constructed by the service, not the caller. Tuple fields are hardcoded, not caller-supplied.

8. **No second authorization engine.** The API, MCP adapter, A2A adapter, and Web UI map to the same service semantics. No protocol gets its own authorization engine.

9. **Authentication belongs at the API boundary.** The kernel does not perform authentication. The API layer establishes AuthenticatedPrincipal from tokens/keys.

10. **The Web UI is optional and replaceable.** Removing the Web UI does not weaken MCP/A2A/API capabilities or the authority model.

11. **The API is language-neutral.** JSON over HTTP. No Go-specific types. No CockroachDB-specific details.

12. **The negative space is explicit.** Forbidden endpoints and patterns are documented. The API makes dangerous semantics structurally difficult.

13. **Complexity must be earned.** Every API concept is justified by the existing domain. No new abstractions for aesthetics.

14. **Database correctness takes precedence over API convenience.** If an API design conflicts with a DB invariant, the DB invariant wins.

15. **The API contract is the product.** The UI, SDKs, MCP, and A2A are clients of the API. The API is not shaped to match any client's implementation.

---

## READY FOR ADVERSARIAL REVIEW
