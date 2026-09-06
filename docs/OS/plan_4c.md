# Phase 4C Implementation Plan

## 1. Executive Decision

Phase 4C turns Solvent's frozen kernel + verified API boundary into a consumable, language-neutral authorization surface. The kernel remains frozen. The API remains frozen. Phase 4C adds the canonical OpenAPI contract, reference integration examples, and the documentation/CI infrastructure that makes the API usable by external engineers without reading the kernel source.

**What Phase 4C IS:**

- Canonical OpenAPI 3.1 specification describing the actual public API
- Reference Python client proving language-neutrality
- Reference GitHub integration proving the extension architecture
- Executor contract documentation (design only, no implementation)
- Minimal API + security + extension documentation
- CI contract validation (lint + spec-in-sync tests)
- Explicit contract reconciliation between Phase 4A, Phase 4B, and OpenAPI

**What Phase 4C IS NOT:**

- A new kernel primitive
- A new service boundary
- A new schema migration
- A production executor
- An SDK
- A multi-language SDK suite
- A full contract-test platform
- RBAC, multi-tenancy, or enterprise IAM
- A large Web UI
- A workflow engine
- A policy DSL

**Governing constraint:** OpenAPI describes the canonical Phase 4A API. OpenAPI does not redefine authority semantics. The Phase 4A contract remains the canonical semantic source of truth for non-security mismatches; the implementation's security hardening takes precedence where it corrects a genuine vulnerability.

---

## 2. Current-State Reconnaissance

### 2.1 REST/API Implementation

| Component | Status | Location |
|-----------|--------|----------|
| HTTP server + routing | Complete | `api/api.go` — Go 1.22+ `net/http.ServeMux` with method-path routing |
| Request/response types | Complete | `api/types.go` — 19 types covering all 26 endpoints |
| Authentication middleware | Complete | `api/auth.go` — Bearer token → `AuthenticatedPrincipal` |
| Error model | Complete | `api/errors.go` — kernel error → HTTP status mapping, SQLSTATE extraction |
| Validation | Complete | `api/validate.go` — UUID, enum, non-empty, JSON validation |
| Read queries | Complete | `api/reads.go` — all SELECT-only database reads |
| Belief handlers | Complete | `api/belief.go` — 8 handlers (enter, get, list, retire debt, promote, retract, explain, list evidence) |
| Evidence handlers | Complete | `api/evidence.go` — 2 handlers (add, get) |
| Principal handlers | Complete | `api/principal.go` — 4 handlers (create, get, list, revoke) |
| Target handlers | Complete | `api/target.go` — 7 handlers (create, get, list, attach justification, request auth, approve, revoke) |
| Authorization handlers | Complete | `api/authorization.go` — 2 handlers (verify, action) |
| Discharge handler | Complete | `api/discharge.go` |
| Activity handler | Complete | `api/activity.go` |
| Ledger handler | Complete | `api/ledger.go` |
| Integration tests | Complete | `api/integration_test.go`, `api/*_test.go` |
| Entry point | Complete | `cmd/solvent-api/main.go` |

### 2.2 OpenAPI / Spec Infrastructure

| Component | Status |
|-----------|--------|
| OpenAPI spec file | **Does not exist** |
| OpenAPI generation tooling | **Does not exist** |
| Spectral linting | **Does not exist** |
| Contract tests | **Does not exist** |
| Swagger annotations in code | **Does not exist** |

### 2.3 Existing Integration/Adapter Interfaces

| Component | Status | Location |
|-----------|--------|----------|
| GitHub adapter | Complete | `adapter/github/github.go` — maps GitHub events to `NormalizedEvidence` |
| EvidenceFeed interface | Implicit | Adapter pattern exists but no formal Go interface |
| Executor registry | Complete (empty) | `service/executor/executor.go` — `ActionFunc`, `Registry`, `ExecuteAction` |
| Recording executor | Complete (test-only) | `service/executor/recording.go` |

### 2.4 Existing Service Boundaries

| Service | Status | Responsibility |
|---------|--------|---------------|
| `service/ledger` | Complete | API orchestration, effective-actor derivation, audit coordination |
| `service/authority` | Complete | `PrepareForAction`, `ExecuteAction` (not wired to API) |
| `service/audit` | Complete | Append-only activity ledger |
| `service/policy` | Complete | Tool/actor registry, constraint evaluation |
| `service/evidence` | Complete | Quality projections (read-only scoring) |
| `service/executor` | Complete (empty) | Executor registry |

### 2.5 Contract/Implementation Mismatches Identified

| # | Contract (Phase 4A) | Implementation (Phase 4B) | Classification |
|---|---------------------|--------------------------|----------------|
| 1 | `AttachJustificationRequest` body: `belief_id`, `belief_status`, `attached_by` | Body: `instrument_ref`; `belief_id` via query param; `belief_status` hardcoded "promoted"; `attached_by` from authenticated principal | Contract gap — implementation derived from Phase 6.1 security fix |
| 2 | `ApproveTargetRequest` body: `approved_by: "uuid"` | Body: `approval_pin: "string"`; `approved_by` from authenticated principal | Security hardening — principal must be server-derived |
| 3 | `RevokeTargetRequest` body: `revoked_by: "uuid"`, `reason: "string"` | Body: `reason` only; `revoked_by` from authenticated principal | Security hardening — same pattern as approve |
| 4 | `VerifyAuthRequest` body: `principal_id: "uuid"` | `principal_id` from authenticated principal (body field ignored) | Security hardening — same pattern |
| 5 | `AuthorizeActionRequest` body: `actor_id: "uuid"` (caller-supplied) | `actor_id` checked against authenticated principal (mismatch → 403); effective principal derived from auth | Security hardening — Phase 6.1 binding |
| 6 | `handleAuthorizeAction` tuple: caller supplies `resource_type`, `scope`, `action_namespace`, `action_name`, `consequence_type` | Handler hardcodes: `resource_type="scenario"`, `scope="belief:"+beliefID`, `action_namespace="solvent"`, `consequence_type="execution"`, `consequence_parameters={}` | Implementation shortcut — needs reconciliation |

**Resolution hierarchy:**
1. Security invariants and approved security fixes (Phase 6.1) take precedence
2. Phase 4A canonical API semantics
3. Current implementation
4. OpenAPI representation

Material mismatches require explicit decision before the OpenAPI spec is written.

### 2.6 Existing Documentation Structure

| Component | Status |
|-----------|--------|
| `docs/OS/` | Extensive — prompts, plans, reviews, roadmap |
| `docs/ARCHITECTURE/` | Exists |
| `docs/ADR/` | Exists |
| API documentation | **Does not exist** |
| Security model documentation | **Does not exist** (scattered across prompts/plans) |
| Extension model documentation | **Does not exist** |

### 2.7 Existing CI

| Component | Status |
|-----------|--------|
| Taskfile.yml | Primary task runner |
| `task test` | Full verification: `go test`, `go build`, `go vet`, `gofmt`, I-7 boundary check, MCP verify |
| GitHub Actions | **Does not exist** |
| Contract validation | **Does not exist** |

### 2.8 Existing Examples/Clients

| Component | Status |
|-----------|--------|
| `examples/` directory | **Does not exist** |
| Generated client SDK | **Does not exist** |
| Test helpers as client examples | `api/helpers_test.go` serves as de facto client reference |

---

## 3. Architecture

```
                        AI agents
                           │
                        MCP / A2A
                           │
apps ───────────────── REST/API ───────────────── workflows
                           │
              ┌────────────┴────────────┐
              │     OpenAPI 3.1 YAML    │
              │   (canonical contract)  │
              └────────────┬────────────┘
                           │
                  ┌────────┴────────┐
                  │  Phase 4B API   │
                  │  (frozen)       │
                  └────────┬────────┘
                           │
                  ┌────────┴────────┐
                  │  Service Layer   │
                  │  (ledger, auth,  │
                  │   audit, policy) │
                  └────────┬────────┘
                           │
              ┌────────────┴────────────┐
              │      SOLVENT KERNEL     │
              │   (frozen, Phase 4B)    │
              └────────────┬────────────┘
                           │
                     CockroachDB

Reference examples:
    examples/python/client.py          → language-neutral API consumer
    examples/github/                   → evidence + authority lifecycle

Extension plane (documented, not implemented):
    Integration adapter → Service → Kernel
    Executor adapter    → Service → Kernel → External provider
```

The OpenAPI document sits between the frozen implementation and the external world. It is the canonical external contract. Tests prevent drift between the document and the implementation.

---

## 4. Phase 4C Scope

### 4.1 Deliverables

1. **OpenAPI 3.1 specification** at `docs/openapi/solvent.yaml`
2. **Contract reconciliation** — explicit decision for every Phase 4A/Phase 4B mismatch
3. **Spectral linting config** at `docs/openapi/.spectral.yaml`
4. **Spec-in-sync Go test** at `api/openapi_test.go`
5. **Python reference client** at `examples/python/`
6. **GitHub reference integration** at `examples/github/`
7. **API getting started guide** at `docs/api/getting-started.md`
8. **Security model overview** at `docs/api/security.md`
9. **Extension model overview** at `docs/api/extensions.md`
10. **Executor contract documentation** in `docs/api/extensions.md`
11. **Taskfile additions** for `task lint:openapi` and `task test:openapi`
12. **Adversarial review** of the complete Phase 4C deliverable set

### 4.2 Acceptance gates

- `go build ./...`
- `go vet ./...`
- `go test -count=1 -p 1 ./...`
- `go test -count=1 ./...`
- Spectral lint passes on `docs/openapi/solvent.yaml`
- Spec-in-sync test passes
- Python client executes reference flow without error
- Adversarial review returns GO

---

## 5. Phase 4C Non-Goals

| Non-Goal | Why Deferred |
|----------|-------------|
| RBAC / role-based access | No users yet; premature |
| Multi-tenancy | v0 is single-tenant |
| Enterprise IAM / OAuth | API key auth is sufficient for v0 |
| Policy DSL | Existing policy service is advisory only |
| Workflow engine | Not needed for authorization layer |
| Compliance subsystem | Audit event shape supports future compliance |
| Production executor | Executor contract designed, not implemented |
| Multiple executors | One real executor at a time |
| SDKs for multiple languages | Python reference client proves language-neutrality |
| Large Web UI | UI is optional and replaceable |
| Generalized plugin platform | Extension model documented, not platformized |
| OpenAPI code generation | Hand-written spec preserves canonical semantics |
| Contract test platform | Spec-in-sync test is sufficient for Phase 4C |
| WebSocket/SSE | Not needed for authorization layer |
| Rate limiting | Premature without users |
| Idempotency keys | v0 limitation accepted |

---

## 6. API Contract Reconciliation

### 6.1 Reconciliation Method

For each mismatch:
1. Identify the exact Phase 4A contract clause
2. Identify the exact Phase 4B implementation behavior
3. Classify: implementation bug / intentional security hardening / contract ambiguity / harmless implementation detail
4. Choose the smallest correct resolution
5. Record the decision

### 6.2 Mismatch Resolutions

#### M-1: AttachJustificationRequest

**Phase 4A contract (§5.2):** Request body contains `belief_id`, `belief_status`, `attached_by`.

**Phase 4B implementation:** Request body contains `instrument_ref`. `belief_id` comes from query parameter `?belief_id=uuid`. `belief_status` is hardcoded to `"promoted"`. `attached_by` is the authenticated principal.

**Classification:** Contract ambiguity + security hardening. The Phase 4A contract intended `belief_id` as a required field but placed it in the body. The implementation moved it to a query parameter (unusual for POST) and added `instrument_ref` which is not in the contract. The authenticated-principal binding is a security improvement.

**Resolution:**
- `belief_id` → move to request body (consistent with all other endpoints). Remove from query parameter.
- `belief_status` → keep hardcoded "promoted" (the handler rejects non-promoted beliefs at the kernel level; explicit status in the request body is misleading).
- `attached_by` → keep derived from authenticated principal (security invariant).
- `instrument_ref` → keep in request body (it is the justification reference; the Phase 4A contract omitted it, which is a contract gap).

**OpenAPI decision:** Request body contains `belief_id` (required, UUID) and `instrument_ref` (required, string). No `belief_status` or `attached_by` in body. Server derives `attached_by` from authentication.

#### M-2: ApproveTargetRequest

**Phase 4A contract (§5.2):** Request body contains `approved_by: "uuid"`.

**Phase 4B implementation:** Request body contains `approval_pin: "string"`. `approved_by` is the authenticated principal.

**Classification:** Security hardening. The approver identity must be server-derived, not caller-supplied. The approval pin is a hash-based verification mechanism that the Phase 4A contract documented but placed in the wrong field.

**Resolution:** Keep implementation. The `approval_pin` is the caller-supplied hash verification. The `approved_by` is server-derived. The OpenAPI spec documents this correctly.

**OpenAPI decision:** Request body contains `approval_pin` (required, string). Response contains `target_id`, `approved`, `approved_by` (from server). No `approved_by` in request.

#### M-3: RevokeTargetRequest

**Phase 4A contract (§5.2):** Request body contains `revoked_by: "uuid"`, `reason: "string"`.

**Phase 4B implementation:** Request body contains `reason` only. `revoked_by` is the authenticated principal.

**Classification:** Security hardening. Same pattern as M-2.

**Resolution:** Keep implementation. `revoked_by` is server-derived.

**OpenAPI decision:** Request body contains `reason` (required, string). No `revoked_by` in request.

#### M-4: VerifyAuthRequest

**Phase 4A contract (§5.2):** Request body contains `principal_id` and all tuple fields.

**Phase 4B implementation:** `principal_id` is derived from authenticated principal. Body contains tuple fields only.

**Classification:** Security hardening. The principal performing verification must be the authenticated identity.

**Resolution:** Keep implementation. `principal_id` is not in the request body; it is the authenticated principal.

**OpenAPI decision:** Request body contains only the 7 tuple fields (`target_id`, `resource_type`, `resource_id`, `scope`, `action_namespace`, `action_name`, `consequence_type`, `consequence_parameters`). No `principal_id` in body.

#### M-5: AuthorizeActionRequest — actor_id

**Phase 4A contract (§5.2):** `actor_id` is caller-supplied in the request body.

**Phase 4B implementation:** `actor_id` is checked against the authenticated principal. Mismatch → HTTP 403 `actor_id_mismatch`. The effective principal is the authenticated identity.

**Classification:** Security hardening — Phase 6.1 actor binding.

**Resolution:** Keep implementation. `actor_id` remains in the request body as an optional field for audit attribution, but if provided and it conflicts with the authenticated principal, the request is rejected.

**OpenAPI decision:** `actor_id` is an optional string in the request body. Document that if present, it must match the authenticated principal or the request is rejected with 403.

#### M-6: handleAuthorizeAction — hardcoded tuple fields

**Phase 4A contract (§5.2):** The request body includes `resource_type`, `scope`, `action_namespace`, `action_name`, `consequence_type`, `consequence_parameters`.

**Phase 4B implementation:** The handler hardcodes `resource_type="scenario"`, `resource_id=scenario_id`, `scope="belief:"+belief_id`, `action_namespace="solvent"`, `action_name=action`, `consequence_type="execution"`, `consequence_parameters={}`.

**Classification:** Implementation shortcut. The handler constructs the tuple from a small number of request fields, hardcoding the dimensional mapping. This is actually more secure than the contract (callers cannot inject arbitrary tuple dimensions), but it means the API surface is narrower than the contract intended.

**Resolution:** This is the most important mismatch. Two options:

- **Option A (narrower API):** Accept `scenario_id` and `belief_id` and `action` only. Hardcode the rest. The OpenAPI spec documents the narrower surface. This is the current behavior and is more secure.
- **Option B (wider API):** Accept all tuple fields in the request body, matching Phase 4A. The handler validates them. This gives more flexibility but exposes more attack surface.

**Decision:** Option A (narrower API). The hardcoded tuple mapping is an intentional security constraint, not an accident. The API for authorize-action should accept only `scenario_id`, `belief_id` and `action` and `target_id` and `action_source`. The AuthorityTuple is constructed server-side. This matches the existing implementation and the principle that callers should not control authority dimensions.

**OpenAPI decision:** `POST /v1/authorizations/action` accepts `scenario_id`, `belief_id`, `action`, `action_source`, `target_id`, `actor_id` (optional). The tuple is constructed server-side. Document the hardcoded mapping.

### 6.3 Summary of Reconciliation

| Mismatch | Resolution | OpenAPI Impact |
|----------|-----------|----------------|
| M-1 AttachJustification | Move `belief_id` to body, keep `instrument_ref`, remove `belief_status`/`attached_by` from body | Request body: `belief_id`, `instrument_ref` |
| M-2 ApproveTarget | Keep `approval_pin` in body, `approved_by` server-derived | Request body: `approval_pin` |
| M-3 RevokeTarget | Keep `reason` in body, `revoked_by` server-derived | Request body: `reason` |
| M-4 VerifyAuth | Remove `principal_id` from body, derive from auth | Request body: 7 tuple fields only |
| M-5 AuthorizeAction actor_id | Optional field, must match auth or 403 | Request body: `actor_id` optional |
| M-6 AuthorizeAction tuple | Hardcoded server-side mapping | Request body: `scenario_id`, `belief_id`, `action`, `target_id`, `action_source`, `actor_id` (optional) |

---

## 7. OpenAPI Design

### 7.1 Specification Structure

```yaml
openapi: "3.1.0"
info:
  title: Solvent API
  version: "1.0.0"
  description: |
    Canonical Solvent authority ledger API.
    Solvent is a transactional belief ledger for autonomous agents.
    This API exposes authority lifecycle, belief management, evidence
    submission, and authorization verification.

servers:
  - url: http://localhost:8080
    description: Local development
  - url: https://{deployment}.solvent.dev
    description: Production (template)

security:
  - bearerAuth: []

tags:
  - name: beliefs
    description: Belief lifecycle operations
  - name: evidence
    description: Evidence submission and retrieval
  - name: principals
    description: Principal identity management
  - name: authority
    description: Authority target lifecycle (propose, justify, approve, revoke)
  - name: authorization
    description: Authorization verification and action intent creation
  - name: discharge
    description: Debt discharge operations
  - name: activity
    description: Audit activity log
  - name: ledger
    description: Ledger summary and projections

paths:
  /v1/beliefs: ...
  /v1/beliefs/{id}: ...
  /v1/beliefs/{id}/debt/retire: ...
  /v1/beliefs/{id}/promote: ...
  /v1/beliefs/{id}/retract: ...
  /v1/beliefs/{id}/explain: ...
  /v1/beliefs/{id}/evidence: ...
  /v1/evidence: ...
  /v1/evidence/{id}: ...
  /v1/principals: ...
  /v1/principals/{id}: ...
  /v1/principals/{id}/revoke: ...
  /v1/targets: ...
  /v1/targets/{id}: ...
  /v1/targets/{id}/justifications: ...
  /v1/targets/{id}/request: ...
  /v1/targets/{id}/approve: ...
  /v1/targets/{id}/revoke: ...
  /v1/authorizations/verify: ...
  /v1/authorizations/action: ...
  /v1/discharge: ...
  /v1/activity: ...
  /v1/ledger: ...

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: API key as bearer token

  schemas:
    Belief: ...
    BeliefList: ...
    Evidence: ...
    Principal: ...
    PrincipalList: ...
    Target: ...
    TargetList: ...
    AuthorityTuple: ...
    AuthResult: ...
    AuthorizeActionResult: ...
    ActivityEntry: ...
    ActivityList: ...
    LedgerSummary: ...
    Verdict: ...
    APIError: ...
    ErrorResponse: ...

  parameters:
    ScenarioID:
      name: scenario_id
      in: query
      required: true
      schema:
        type: string
        format: uuid
    Limit:
      name: limit
      in: query
      schema:
        type: integer
        default: 100
    Offset:
      name: offset
      in: query
      schema:
        type: integer
        default: 0
```

### 7.2 Reusable Schemas

**APIError** (canonical error response):
```yaml
APIError:
  type: object
  required: [code, message]
  properties:
    code:
      type: string
      description: Stable Solvent error code
    message:
      type: string
      description: Human-readable error description
    details:
      type: object
      description: Additional error context (sqlstate, constraint_name, field)
    retryable:
      type: boolean
      default: false
    request_id:
      type: string
      format: uuid
```

**AuthResult** (authorization verification result):
```yaml
AuthResult:
  type: object
  required: [target_id, allowed, reason]
  properties:
    target_id:
      type: string
      format: uuid
    allowed:
      type: boolean
    reason:
      type: string
```

**Verdict** (promotion refusal — HTTP 200 with refusal body):
```yaml
Verdict:
  type: object
  required: [type, reason, gate]
  properties:
    type:
      type: string
    reason:
      type: string
    gate:
      type: string
```

### 7.3 Security Scheme

```yaml
securitySchemes:
  bearerAuth:
    type: http
    scheme: bearer
    bearerFormat: API Key
    description: |
      Bearer token authentication. The token maps to a principal_id
      configured on the server. The authenticated principal is used as
      the effective actor for all authorization operations.

      The kernel never sees authentication tokens. The API layer
      translates the token into an AuthenticatedPrincipal and passes
      it as an attribution field.
```

### 7.4 Response Envelopes

All successful responses return the resource directly (no wrapper envelope). Paginated responses include `total`, `limit`, `offset` fields.

Error responses always use the `ErrorResponse` wrapper:
```yaml
ErrorResponse:
  type: object
  properties:
    error:
      $ref: '#/components/schemas/APIError'
```

### 7.5 Forbidden Endpoints (Negative Space)

The OpenAPI spec must NOT document these endpoints (they are forbidden per Phase 4A §8):

- `/execute-without-authority`
- `/set-authorized`
- `/approve-with-agent`
- `/update-authority-state`
- `/force-execution`
- `/override-authorization`
- `/set-actor`
- `/impersonate`
- `/approve-and-execute`
- `/execute-as-approved`
- `/trust-tool-output`
- `/cache-authorization`

Document the forbidden surface in the spec's `description` field at the info level.

### 7.6 Document Location

`docs/openapi/solvent.yaml` — the canonical OpenAPI 3.1 contract for the public Solvent HTTP API. Maintained intentionally and validated against the implementation; not generated from Go types.

---

## 8. Reference Integrations

### 8.1 Python Reference Client (`examples/python/`)

**Purpose:** Prove Solvent is truly language-neutral. A developer in Python should be able to integrate with Solvent without understanding the kernel internals.

**Structure:**
```
examples/python/
├── README.md
├── client.py
└── basic_authorization.py
```

**`client.py`** — thin HTTP client:
- `SolventClient(base_url, api_key)` struct
- Methods mapping 1:1 to API endpoints
- Zero authorization logic of its own
- Pure request serialization + response deserialization
- Generated from OpenAPI where practical

**`basic_authorization.py`** — reference flow:
```python
# 1. Authenticate
client = SolventClient("http://localhost:8080", "your-api-key")

# 2. Create a principal
principal = client.create_principal("agent", "github-actions")

# 3. Enter a belief
belief = client.enter_belief(scenario_id, "deploy commit abc123 is safe", "derived")

# 4. Create authority target
target = client.create_target(
    principal_id=principal.principal_id,
    resource_type="repository",
    resource_id="org/repo",
    scope="branch:main",
    action_namespace="github",
    action_name="deploy",
    consequence_type="execution",
    consequence_parameters={"environment": "production"},
    created_by=principal.principal_id,
)

# 5. Attach justification
client.attach_justification(target.target_id, belief.belief_id, "commit safety evidence")

# 6. Request authorization
client.request_authorization(target.target_id)

# 7. Approve (admin)
client.approve_target(target.target_id, approval_pin=target.approval_pin)

# 8. Verify authority
result = client.verify_authorization(target.target_id, principal.principal_id, ...)
assert result.allowed

# 9. Authorize action
action_result = client.authorize_action(
    scenario_id=scenario_id,
    belief_id=belief.belief_id,
    action="deploy",
    action_source="user_typed",
    target_id=target.target_id,
)
assert action_result.authority.allowed
```

**Design rules:**
- Zero authorization logic in the client
- Pure HTTP serialization
- No SDK abstraction that could diverge from the canonical API
- Follows the OpenAPI spec exactly

### 8.2 GitHub Reference Integration (`examples/github/`)

**Purpose:** Demonstrate both the evidence ingestion chain and the complete authority lifecycle in one bounded integration.

**Structure:**
```
examples/github/
├── README.md
└── integration.go
```

**Flow:**
```text
GitHub webhook / event
        ↓
adapter/github.ParseWebhook / Adapter.ProcessEvent
        ↓
NormalizedEvidence
        ↓
POST /v1/evidence  (via Solvent API)
        ↓
POST /v1/beliefs   (create belief from evidence)
        ↓
POST /v1/targets   (create deployment authority target)
        ↓
POST /v1/targets/{id}/justifications  (attach belief as justification)
        ↓
POST /v1/targets/{id}/request  (pin proposal)
        ↓
POST /v1/targets/{id}/approve  (human approval)
        ↓
POST /v1/authorizations/action  (agent requests deployment)
        ↓
Executor boundary  (documented, not implemented)
```

**Key properties:**
- GitHub-specific logic stays inside `adapter/github/`
- All downstream operations use canonical Solvent API concepts
- The integration calls the REST API, not the kernel directly
- The executor boundary is documented but no real execution occurs
- Demonstrates evidence ≠ authority: evidence informs, authority decides

**Scope:** One concrete scenario — "GitHub push triggers deployment authorization." The integration exercises evidence ingestion + the complete authority lifecycle, proving both planes work together.

---

## 9. Extension Mechanism Validation

### 9.1 Current Extension Seams

| Seam | Interface | Status | Location |
|------|-----------|--------|----------|
| Protocol Adapter (MCP) | MCP tools → Service | Complete | `cmd/solvent-mcp/` |
| Protocol Adapter (REST) | HTTP handlers → Service | Complete | `api/` |
| Evidence Adapter | `adapter/github/` → `NormalizedEvidence` | Complete (no formal interface) | `adapter/github/` |
| Executor | `ActionFunc` → `Registry` → `ExecuteAction` | Complete (empty registry) | `service/executor/` |
| Service Layer | Handlers → Service → Kernel | Complete | `service/ledger/`, `service/authority/` |

### 9.2 Extension Architecture Assessment

The current architecture cleanly supports:

```
Protocol Adapter (REST/MCP/A2A)
        ↓
Service Layer (existing)
        ↓
Kernel (frozen)
```

and:

```
Integration Adapter (GitHub/CI/CD/K8s)
        ↓
Solvent API (REST)
        ↓
Service Layer
        ↓
Kernel
```

and future:

```
Service Layer
        ↓
Executor Adapter (GitHub Actions/CI/CD)
        ↓
External Provider
```

### 9.3 What Is Missing for First Real Integration

1. **Formal EvidenceFeed interface** — `adapter/github/` follows the pattern but has no Go interface. A formal `EvidenceFeed` interface would make the adapter contract explicit.
2. **Executor contract documentation** — `ActionFunc` exists but its security boundary is not documented. Phase 4C documents what executors may and must not do.
3. **Integration reference** — no example exists showing how to wire an adapter to the Solvent API. `examples/github/` fills this gap.

### 9.4 What Is NOT Missing

- No new service boundary needed
- No new kernel primitive needed
- No new schema needed
- No new adapter interface needed (the existing pattern is sufficient)

---

## 10. First Real Integration

### 10.1 Preferred Candidate: GitHub Deployment Authorization

After Phase 4C, the first serious integration should prove the complete chain:

```text
untrusted / AI-generated information
        ↓
evidence
        ↓
review
        ↓
explicit authority
        ↓
exact current-state authorization
        ↓
external execution
        ↓
audit
```

### 10.2 Conceptual Flow

```text
AI agent proposes:
    deploy commit X to production

Solvent receives:
    actor (authenticated principal)
    resource (repository)
    target (branch + environment)
    action (deploy)
    consequence (production deployment)
    evidence (CI status, test results, review approval)

reviewer/policy establishes authority:
    principal approved for repository deployment
    branch:main scope
    deployment action
    evidence: CI passing + human review

Solvent authorizes:
    EXACT principal
    EXACT action
    EXACT target
    CURRENT authority

executor invokes GitHub/CI/CD provider:
    GitHub Actions workflow_dispatch or deployment API

Solvent records:
    authorization
    execution reference
    result
    audit evidence
```

### 10.3 Separation of Authorization and Execution

The plan must clearly separate:

```text
authorization
    kernel.Authorize → allowed/denied
from
    execution
    executor → external provider side effect
```

and:

```text
Solvent authority
    database facts, kernel semantics
from
    external provider semantics
    GitHub API, CI/CD specifics
```

### 10.4 Phase 4C vs. Phase 4C+ Boundary

**Phase 4C:**
- Document the executor contract
- Build the reference integration up to the executor boundary
- Prove evidence → belief → target → approval → authorization

**Phase 4C+ (next phase):**
- Implement one real GitHub executor
- Wire ExecuteAction to a real external provider
- Prove authorization → execution → audit
- Close the TOCTOU window with real execution

---

## 11. Executor Boundary

### 11.1 Current Executor Abstraction

```go
// service/executor/executor.go
type ActionFunc func(ctx context.Context, params map[string]interface{}) (string, error)

type Registry struct { actions map[string]ActionFunc }
func NewRegistry() *Registry
func (r *Registry) Register(name string, fn ActionFunc)
func (r *Registry) Get(name string) (ActionFunc, bool)
func ExecuteAction(ctx, registry, actionName, params) (string, error)
```

### 11.2 What Exists

- `ActionFunc` type — minimal, sufficient for Phase 4C
- `Registry` — maps action names to implementations
- `ExecuteAction` — dispatch function
- `RecordingFunc` — test-only recording function

### 11.3 What Is Missing (Documented, Not Implemented)

1. **Executor contract documentation** — what executors may and must not do
2. **Executor security boundary** — executors cannot approve/revoke authority
3. **Executor input contract** — what `params` contains (opaque to executor, but documented)
4. **Executor output contract** — execution reference, success/failure
5. **Executor audit handoff** — how execution results feed back into audit

### 11.4 Executor Contract (to be documented)

```
Executor MAY:
- perform the already-authorized external action
- return execution outcome/reference
- log execution results

Executor MUST NOT:
- approve authority
- create authority
- revoke authority
- reinterpret Solvent authorization
- bypass current authorization checks
- become a second policy engine
- trust cached authorization
- select itself (registry-resolved, not caller-selected)

Executor RECEIVES:
- action name (from registry)
- opaque parameters (from IntentOnPromoted context)
- context with timeout

Executor RETURNS:
- execution reference (external ID, URL, etc.)
- error (if execution fails)

Executor DOES NOT RECEIVE:
- AuthorityTuple
- approval state
- authority-derivable data
- ability to modify kernel state
```

### 11.5 Why ActionFunc Stays As-Is

The `ActionFunc` signature is sufficient for Phase 4C. A richer interface (with `AuthorizationContext`, `ExecutionResult`, `AuditHandoff`) should be designed only when the first real executor reveals concrete requirements. Premature abstraction creates surface area without evidence.

---

## 12. Service / Policy Boundary

### 12.1 Current Service Boundaries

| Service | Responsibility | Sufficient for Phase 4C? |
|---------|---------------|------------------------|
| `service/ledger` | API orchestration, effective-actor, audit | Yes |
| `service/authority` | PrepareForAction, ExecuteAction | Yes (ExecuteAction not wired) |
| `service/audit` | Append-only activity ledger | Yes |
| `service/policy` | Tool/actor registry, constraint evaluation | Yes |
| `service/evidence` | Quality projections | Yes |
| `service/executor` | Executor registry | Yes |

### 12.2 New Service Boundaries

**None proposed.** The existing boundaries are sufficient for Phase 4C.

Integration-specific orchestration (e.g., GitHub webhook → evidence → belief → target) belongs in the integration adapter / reference integration code, not in a new service.

### 12.3 Policy ≠ Authority

`service/policy.EvaluateConstraints` returns advisory output. It can veto (deny) but cannot grant authority. The kernel remains the final authority oracle. This boundary must not be weakened by Phase 4C.

---

## 13. Documentation

### 13.1 Documentation Set

| Document | Location | Purpose |
|----------|----------|---------|
| OpenAPI spec | `docs/openapi/solvent.yaml` | Canonical API contract |
| API getting started | `docs/api/getting-started.md` | How to call the API |
| Security model | `docs/api/security.md` | Authority model, authentication, five authorization layers |
| Extension model | `docs/api/extensions.md` | Adapter/executor/service boundaries, how to extend |

### 13.2 API Getting Started (`docs/api/getting-started.md`)

Contents:
- Prerequisites (CockroachDB, API key)
- Authentication (Bearer token)
- Core concepts (belief, evidence, target, authority, authorization)
- Quick start: create a belief, promote it, create a target, approve, authorize
- Error handling (error response structure, SQLSTATE surfacing)
- Links to OpenAPI spec

### 13.3 Security Model (`docs/api/security.md`)

Contents:
- Five authorization layers (authentication, API access control, Solvent authority, policy, execution authorization)
- AuthenticatedPrincipal model
- AuthorityTuple construction (server-side, not caller-controlled)
- Forbidden API surface
- SQLSTATE surfacing policy
- Evidence ≠ authority principle
- Policy ≠ authority principle
- Intent ≠ execution principle

### 13.4 Extension Model (`docs/api/extensions.md`)

Contents:
- Architecture overview (kernel → service → extension plane)
- Protocol adapters (REST, MCP, A2A)
- Integration adapters (GitHub, CI/CD, K8s)
- Executor contract
- Service/policy boundaries
- Kernel-growth gate
- How to add a new integration without changing the kernel

### 13.5 Documentation Generation

Repetitive API reference material (endpoint descriptions, schema documentation) should be generated from the canonical OpenAPI where practical. Security and extension docs are hand-written.

---

## 14. CI / Contract Verification

### 14.1 Validation Stack

```
1. Spectral → OpenAPI quality/style
2. Spec-sync Go test → contract surface drift
3. go test ./... → runtime correctness
4. adversarial review → security/semantic correctness
```

### 14.2 Spectral Configuration

`docs/openapi/.spectral.yaml`:
- Validates OpenAPI 3.1 structure
- Enforces naming conventions
- Checks for required security schemes
- Verifies error response schemas

### 14.3 Spec-in-Sync Test

`api/openapi_test.go` — lightweight Go test that verifies:
- Required canonical routes exist in the spec
- HTTP methods match
- Security is declared where required
- Key response schemas exist
- Canonical error schema exists
- Forbidden endpoints are absent from the spec

This test does NOT duplicate the entire API implementation. It verifies the contract surface, not behavioral correctness.

### 14.4 Taskfile Additions

```yaml
lint:openapi:
  desc: Lint OpenAPI specification
  cmds:
    - spectral lint docs/openapi/solvent.yaml --ruleset docs/openapi/.spectral.yaml

test:openapi:
  desc: Run OpenAPI contract validation tests
  cmds:
    - go test -count=1 -v -run TestOpenAPISpec ./api/...
```

### 14.5 What Is NOT in Phase 4C

- Full contract test suite (runtime validation against live API) — deferred to first real integration
- GitHub Actions CI — Taskfile is sufficient
- Schema validation middleware — premature

---

## 15. Security / Adversarial Review

### 15.1 Review Areas

| # | Area | What to Verify |
|---|------|---------------|
| 1 | API contract drift | OpenAPI matches implementation; no undocumented endpoints |
| 2 | Accidental kernel exposure | API does not expose kernel tables, internal types, or schema details |
| 3 | Authentication ambiguity | AuthenticatedPrincipal is clear; token → principal mapping is unambiguous |
| 4 | Caller-controlled security fields | No request field can override server-derived principal, authority state, or approval |
| 5 | Error leakage | Internal errors, stack traces, DB details are never exposed |
| 6 | Unauthorized resource access | Read operations are properly scoped; no cross-scenario reads |
| 7 | Second authorization engine | No endpoint creates authority except approve; no endpoint bypasses kernel.Authorize |
| 8 | API/MCP semantic divergence | REST and MCP produce identical authority outcomes |
| 9 | Executor bypass | No production execution path exists; executor registry is empty |
| 10 | Stale/cached authorization | No caching layer; kernel.Authorize re-reads fresh state |
| 11 | Arbitrary provider capability selection | Executor is registry-resolved; caller cannot inject executor |
| 12 | Reference-client security shortcuts | Python client has no authorization logic of its own |
| 13 | Contract reconciliation correctness | All M-1 through M-6 resolutions are correct and complete |
| 14 | OpenAPI spec completeness | Every real endpoint is documented; no invented endpoints |

### 15.2 Review Requirements

- No "looks good" review
- Require concrete code-path evidence
- Verify against actual HTTP handlers, not documentation
- Check that the spec-in-sync test actually catches drift

---

## 16. Kernel-Growth Gate

### 16.1 Evaluation

Every proposed Phase 4C change is evaluated against:

> Is this:
> - a new durable security fact?
> - an atomic security-critical state transition?
> - impossible to represent safely above the kernel?

### 16.2 Phase 4C Changes

| Change | New Durable Security Fact? | New Atomic Transition? | Impossible Above Kernel? | Decision |
|--------|--------------------------|----------------------|------------------------|----------|
| OpenAPI spec | No | No | No | Extend above kernel |
| Python client | No | No | No | Extend above kernel |
| GitHub reference integration | No | No | No | Extend above kernel |
| Executor contract docs | No | No | No | Extend above kernel |
| API docs | No | No | No | Extend above kernel |
| Spec-in-sync test | No | No | No | Extend above kernel |
| Spectral config | No | No | No | Extend above kernel |

**No kernel changes proposed.** All Phase 4C work lives in the extension plane.

---

## 17. Package / File Implementation Map

### 17.1 New Files

| File | Package | Responsibility | Production/Test/Docs | New Abstraction? |
|------|---------|---------------|---------------------|-----------------|
| `docs/openapi/solvent.yaml` | — | Canonical OpenAPI 3.1 spec | Docs | No |
| `docs/openapi/.spectral.yaml` | — | Spectral linting config | Docs/CI | No |
| `docs/api/getting-started.md` | — | API getting started guide | Docs | No |
| `docs/api/security.md` | — | Security model overview | Docs | No |
| `docs/api/extensions.md` | — | Extension model overview | Docs | No |
| `examples/python/client.py` | — | Python reference client | Docs/Example | No (thin wrapper) |
| `examples/python/basic_authorization.py` | — | Python authorization flow example | Docs/Example | No |
| `examples/python/README.md` | — | Python client documentation | Docs | No |
| `examples/github/integration.go` | — | GitHub reference integration | Docs/Example | No |
| `examples/github/README.md` | — | GitHub integration documentation | Docs | No |

### 17.2 Modified Files

| File | Change | New Abstraction? |
|------|--------|-----------------|
| `api/openapi_test.go` | New file: spec-in-sync contract validation test | No (test only) |
| `Taskfile.yml` | Add `lint:openapi` and `test:openapi` tasks | No |

### 17.3 Files NOT Modified

- `kernel/*` — frozen
- `api/*.go` (handlers, types, auth, errors) — frozen
- `service/*` — frozen
- `cmd/*` — frozen
- `db/*` — no migrations
- `adapter/github/*` — unchanged (used by reference integration, not modified)

---

## 18. Implementation Order

| Step | Task | Depends On | Estimated Scope |
|------|------|-----------|----------------|
| 1 | Contract reconciliation: classify and resolve all M-1 through M-6 mismatches | — | Decision doc (part of this plan) |
| 2 | Write `docs/openapi/solvent.yaml` — full OpenAPI 3.1 spec | Step 1 | ~800 lines YAML |
| 3 | Write `docs/openapi/.spectral.yaml` — linting config | Step 2 | ~30 lines YAML |
| 4 | Write `api/openapi_test.go` — spec-in-sync test | Step 2 | ~150 lines Go |
| 5 | Add `lint:openapi` and `test:openapi` to `Taskfile.yml` | Steps 3, 4 | ~10 lines YAML |
| 6 | Run `go test ./...` to verify no regressions | Steps 4, 5 | Verification |
| 7 | Write `examples/python/client.py` — Python reference client | Step 2 | ~200 lines Python |
| 8 | Write `examples/python/basic_authorization.py` — reference flow | Step 7 | ~100 lines Python |
| 9 | Write `examples/python/README.md` | Steps 7, 8 | ~50 lines |
| 10 | Write `examples/github/integration.go` — GitHub reference integration | Steps 2, 7 | ~200 lines Go |
| 11 | Write `examples/github/README.md` | Step 10 | ~50 lines |
| 12 | Write `docs/api/getting-started.md` | Step 2 | ~150 lines |
| 13 | Write `docs/api/security.md` | Step 2 | ~150 lines |
| 14 | Write `docs/api/extensions.md` | Steps 2, 10 | ~200 lines |
| 15 | Run full verification suite | Steps 4-14 | Verification |
| 16 | Adversarial review of complete Phase 4C deliverables | Step 15 | Review |
| 17 | Address review findings | Step 16 | Remediation |

---

## 19. Acceptance Criteria

| Gate | Requirement |
|------|------------|
| [OPENAPI] | `docs/openapi/solvent.yaml` exists, is valid OpenAPI 3.1, and passes Spectral lint |
| [SPEC-SYNC] | `api/openapi_test.go` passes and would catch route/method/security drift |
| [RECONCILE] | All M-1 through M-6 contract mismatches have explicit resolution decisions |
| [PYTHON] | `examples/python/basic_authorization.py` executes the full flow against a running API |
| [GITHUB] | `examples/github/integration.go` compiles and documents the evidence + authority lifecycle |
| [DOCS] | `docs/api/getting-started.md`, `docs/api/security.md`, `docs/api/extensions.md` exist and are accurate |
| [EXECUTOR] | Executor contract is documented in `docs/api/extensions.md` with security boundaries |
| [BUILD] | `go build ./...` passes |
| [VET] | `go vet ./...` passes |
| [TEST] | `go test -count=1 -p 1 ./...` passes |
| [NO-KERNEL] | No kernel files modified |
| [NO-SCHEMA] | No migrations added |
| [NO-NEWSERVICE] | No new service boundaries introduced |
| [TASKFILE] | `task lint:openapi` and `task test:openapi` work |
| [SECURITY] | Adversarial review returns GO |

---

## 20. Rollback Strategy

If Phase 4C encounters an unexpected blocker:

1. **OpenAPI spec is wrong:** Reconcile the mismatch, update the spec. The spec is a file, not a migration.
2. **Python client fails:** Fix the client or the spec. Neither is a production system.
3. **Contract reconciliation reveals a security issue:** Stop. Escalate to kernel team. Do not silently "fix" by weakening the spec.
4. **Spectral lint fails:** Fix the spec. This is a documentation quality gate, not a production gate.
5. **Spec-in-sync test fails:** Either the spec or the implementation is wrong. Investigate and fix.

**Rollback is always safe** because Phase 4C modifies no production code. It adds documentation, tests, and reference examples. Removing any of these files restores the pre-Phase-4C state.

---

## 21. Risks / Open Questions

| # | Risk | Mitigation |
|---|------|-----------|
| 1 | Contract reconciliation reveals deeper Phase 4A/4B divergence | Explicit case-by-case resolution; security wins |
| 2 | OpenAPI spec drifts from implementation over time | Spec-in-sync test catches drift; Spectral lint catches quality issues |
| 3 | Python client becomes outdated | Client is a reference example, not a maintained SDK; update when API changes |
| 4 | GitHub reference integration becomes stale | It is an example, not a product; update when API changes |
| 5 | Executor contract documentation constrains future design | Document current minimal contract; revisit when real executor exists |
| 6 | Adversarial review finds security issues in reconciliation | Expected and desired; findings become remediation items |
| 7 | Hardcoded tuple mapping in authorize-action is too narrow | Document the constraint; widening is additive evolution (non-breaking) |
| 8 | Spectral rules are too strict or too loose | Tune `.spectral.yaml` based on first lint run |

### Open Questions

| # | Question | Impact | Recommended Resolution |
|---|----------|--------|----------------------|
| 1 | Should `AttachJustification` use body or query param for `belief_id`? | API ergonomics | Body (consistent with other endpoints) — resolved in M-1 |
| 2 | Should `handleAuthorizeAction` accept caller-supplied tuple fields? | Security vs. flexibility | No — keep hardcoded server-side mapping — resolved in M-6 |
| 3 | Should the OpenAPI spec include the MCP tool mapping? | Spec scope | No — MCP is a separate protocol; document the mapping in `extensions.md` |
| 4 | Should examples be executable (with `go run` / `python`)? | Example quality | Yes — `basic_authorization.py` should be runnable; `integration.go` should compile |

---

## 22. Next Phase

After Phase 4C completes and the adversarial review returns GO:

```
Phase 4C   OpenAPI + reference integrations + docs
    ↓
Phase 4C+  One real GitHub executor (consequential execution)
    ↓
Phase 4D   Optional Web UI (consumes canonical API)
    ↓
Phase 4E   API/UI adversarial review
    ↓
Phase 5    Compliance / Governance (earned later)
```

The immediate next phase is **Phase 4C+ — One Real GitHub Executor**:

- Implement a real GitHub Actions / Deployment executor
- Wire `service/authority.ExecuteAction` to the executor registry
- Prove authorization → execution → audit
- Close the TOCTOU documentation gap with real execution
- Adversarial review of the execution boundary

This is the phase where Solvent crosses from "authorization layer" to "authorization + execution layer." It should only begin after Phase 4C is verified and the extension seam is proven stable.

---

## 23. Final Recommendation

Phase 4C is the bridge between the frozen kernel and the extension ecosystem. It does not add new authority semantics, new kernel primitives, or new service boundaries. It makes the existing, verified API boundary consumable by external engineers.

The key architectural contribution is:

```
SMALL TRUSTED AUTHORITY KERNEL (frozen)
            ↓
STABLE LANGUAGE-NEUTRAL API (OpenAPI 3.1)
            ↓
EXTENSION PLANE
            ↓
AGENTS / APPS / WORKFLOWS / SYSTEMS
```

This is how Solvent becomes universal without making the kernel universal. The kernel stays small. The API becomes the stable external boundary. Extensions plug in above the API.

**Phase 4C delivers:**
1. A canonical OpenAPI contract that accurately describes the real API
2. A Python reference client proving language-neutrality
3. A GitHub reference integration proving the extension architecture
4. Documentation enabling external engineers to consume and extend Solvent
5. CI contract validation preventing spec/implementation drift
6. Explicit resolution of every contract/implementation mismatch

**Phase 4C does NOT deliver:**
1. Production execution
2. New security semantics
3. SDKs
4. Enterprise features
5. New service boundaries

The plan is deliberately small, precise, and focused on making the existing foundation consumable. That is exactly what the roadmap requires at this point.

---

## How to Make Solvent Universal Without Making the Kernel Universal

The answer is expressed concretely through six layers:

```
1. KERNEL (frozen)
   Stable authority semantics. Domain-agnostic. Unaware of GitHub,
   Kubernetes, AWS, MCP, A2A, UI, compliance, or AI providers.

2. CANONICAL API (OpenAPI 3.1)
   Language-neutral HTTP/JSON surface. The single external contract.
   Not generated from Go types. Not shaped by any particular client.

3. SERVICE / POLICY COMPOSITION
   Evidence ingestion, policy evaluation, integration orchestration,
   execution coordination. Above the kernel. Does not become authority.

4. PROTOCOL ADAPTERS
   REST, MCP, A2A. Translate external protocol semantics into
   canonical Solvent operations. No alternate authorization paths.

5. INTEGRATION ADAPTERS
   GitHub, CI/CD, Kubernetes, AWS. Provider-specific logic at the
   edge. Never touches the kernel. Emits canonical Solvent types.

6. EXECUTOR ADAPTERS
   Execute authorized actions against external providers.
   Registry-resolved. Cannot approve/revoke authority.
   Cannot bypass kernel.Authorize.
```

A future GitHub integration plugs in at layers 5 and 6 without changing layers 1-4. A future Kubernetes integration plugs in at layers 5 and 6 without changing layers 1-4. A future A2A integration plugs in at layer 4 without changing layers 1-3.

The kernel stays small because every extension is expressed above it.

---

## What Is the Smallest Next Implementation

The smallest next implementation that makes Solvent more useful as an authorization layer:

```
one clear integration contract    (OpenAPI 3.1)
one strong reference integration  (GitHub evidence → authority lifecycle)
one excellent end-to-end story    (Python client proves language-neutrality)
minimal trusted core              (kernel unchanged)
maximum reuse                     (existing services, existing adapter)
```

That is Phase 4C. Not more features. Not more services. Not more abstractions. Just making the existing foundation consumable.
