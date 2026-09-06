# Phase 4C: Canonical API Decision Record

This document is the sole input to OpenAPI spec writing. It records every material reconciliation decision (M-1 through M-6) between the Phase 4A contract, the Phase 4B frozen implementation, and the canonical v1 HTTP API established by this reconciliation.

**Governing constraint:** OpenAPI describes the canonical v1 HTTP API established by Phase 4C reconciliation. Phase 4A remains the original semantic source except where an explicitly documented security decision superseded it.

**Resolution hierarchy:**
1. Security hardening superseding Phase 4A → canonical v1
2. Phase 4A where not superseded
3. Phase 4B implementation as documentation target
4. OpenAPI describes canonical v1

---

## M-1: AttachJustificationRequest

**Phase 4A contract (§5.2):** Request body contains `belief_id`, `belief_status`, `attached_by`.

**Phase 4B implementation:** Request body contains `instrument_ref`. `belief_id` comes from query parameter `?belief_id=uuid`. `belief_status` is hardcoded to `"promoted"`. `attached_by` is the authenticated principal.

**Classification:** Contract ambiguity + security hardening.

**Resolution:** Keep the exact Phase 4B API. The query-parameter `belief_id` is the current frozen behavior. The OpenAPI spec documents it faithfully. No implementation change.

**Canonical v1 shape:**

```
POST /v1/targets/{target_id}/justifications?belief_id=uuid

Request body:
{
  "instrument_ref": "string (required)"
}

Response: TargetResponse (200)
```

**Security note:** `attached_by` is derived from the authenticated principal. `belief_status` is hardcoded server-side.

---

## M-2: ApproveTargetRequest

**Phase 4A contract (§5.2):** Request body contains `approved_by: "uuid"`.

**Phase 4B implementation:** Request body contains `approval_pin: "string"`. `approved_by` is the authenticated principal.

**Classification:** Security hardening. The approver identity must be server-derived, not caller-supplied.

**Resolution:** Keep implementation. The `approval_pin` is the caller-supplied hash verification. The `approved_by` is server-derived. The OpenAPI spec documents this correctly.

**Canonical v1 shape:**

```
POST /v1/targets/{target_id}/approve

Request body:
{
  "approval_pin": "string (required)"
}

Response: TargetResponse (200)
  - approved_by is server-derived from authenticated principal
```

**Security note:** `approved_by` is never caller-controlled. This is a first-class authorization invariant.

---

## M-3: RevokeTargetRequest

**Phase 4A contract (§5.2):** Request body contains `revoked_by: "uuid"`, `reason: "string"`.

**Phase 4B implementation:** Request body contains `reason` only. `revoked_by` is the authenticated principal.

**Classification:** Security hardening. Same pattern as M-2.

**Resolution:** Keep implementation. `revoked_by` is server-derived.

**Canonical v1 shape:**

```
POST /v1/targets/{target_id}/revoke

Request body:
{
  "reason": "string (required)"
}

Response: TargetResponse (200)
  - revoked_by is server-derived from authenticated principal
```

---

## M-4: VerifyAuthRequest

**Phase 4A contract (§5.2):** Request body contains `principal_id` and all tuple fields.

**Phase 4B implementation:** `principal_id` is derived from authenticated principal. Body contains tuple fields only.

**Classification:** Security hardening. The principal performing verification must be the authenticated identity.

**Resolution:** Keep implementation. `principal_id` is not in the request body; it is the authenticated principal.

**Canonical v1 shape:**

```
POST /v1/authorizations/verify

Request body (8 tuple fields only):
{
  "target_id": "uuid",
  "resource_type": "string",
  "resource_id": "string",
  "scope": "string",
  "action_namespace": "string",
  "action_name": "string",
  "consequence_type": "string",
  "consequence_parameters": {}
}

Response: AuthResult (200)
```

**Security note:** `principal_id` is always the authenticated principal. Callers cannot impersonate.

---

## M-5: AuthorizeActionRequest — actor_id

**Phase 4A contract (§5.2):** `actor_id` is caller-supplied in the request body.

**Phase 4B implementation:** `actor_id` is checked against the authenticated principal. Mismatch → HTTP 403 `actor_id_mismatch`. The effective principal is the authenticated identity.

**Classification:** Security hardening — Phase 6.1 actor binding.

**Resolution:** Keep implementation. `actor_id` remains in the request body as an optional field for audit attribution, but if provided and it conflicts with the authenticated principal, the request is rejected.

**Canonical v1 shape:**

```
POST /v1/authorizations/action

Request body:
{
  "scenario_id": "uuid (required)",
  "belief_id": "uuid (required)",
  "action": "string (required)",
  "action_source": "string (required, must be 'user_typed')",
  "target_id": "uuid (required)",
  "actor_id": "string (optional — if present, must match authenticated principal)"
}

Response: AuthorizeActionResult (200)
  - authority.allowed: bool
  - authority.reason: string
  - intent_state: string (if allowed)
  - belief_id: string
  - intent_id: string (if allowed)
  - action: string
```

**Security notes:**
- `actor_id`, when supplied, must equal the authenticated principal; the API derives the effective principal from authentication (Phase 6.1).
- `action_source` must equal `"user_typed"`. Tool output (`action_source` ≠ `"user_typed"`) is rejected with HTTP 403 `action_source_tool_output`.
- The AuthorityTuple is constructed server-side. Callers cannot inject arbitrary authority dimensions.

---

## M-6: handleAuthorizeAction — hardcoded tuple fields

**Phase 4A contract (§5.2):** The request body includes `resource_type`, `scope`, `action_namespace`, `action_name`, `consequence_type`, `consequence_parameters`.

**Phase 4B implementation:** The handler hardcodes `resource_type="scenario"`, `resource_id=scenario_id`, `scope="belief:"+belief_id`, `action_namespace="solvent"`, `action_name=action`, `consequence_type="execution"`, `consequence_parameters={}`.

**Classification:** Security hardening — the narrower API is intentional.

**Resolution:** This is the canonical v1 API shape. The broader Phase 4A tuple-input shape is superseded for this endpoint by the Phase 6.1 security decision. The AuthorityTuple is constructed server-side from a small number of request fields. Callers cannot inject arbitrary authority dimensions.

**Canonical v1 shape:**

```
POST /v1/authorizations/action

Server-constructed AuthorityTuple:
  ResourceType = "scenario"           (hardcoded)
  ResourceID   = scenario_id          (from request body)
  Scope        = "belief:" + belief_id (from request body)
  ActionNamespace = "solvent"         (hardcoded)
  ActionName   = action               (from request body)
  ConsequenceType = "execution"       (hardcoded)
  ConsequenceParameters = {}          (hardcoded)

Caller-supplied fields:
  scenario_id, belief_id, action, action_source, target_id, actor_id
```

**Security rationale:** Narrowing the API to a small set of request fields that map to a known tuple shape eliminates entire classes of authority injection. The kernel invariant ("authority cannot survive a retracted or non-promoted belief") is enforced by the database, not by the API surface.

---

## Non-Reconciled Endpoints

The following endpoints have no Phase 4A/4B mismatch. OpenAPI documents them faithfully:

| Endpoint | Notes |
|----------|-------|
| `POST /v1/beliefs` | `scenario_id`, `claim`, `claim_type` — no mismatch |
| `GET /v1/beliefs/{id}` | Query params: `scenario_id` |
| `GET /v1/beliefs` | Query params: `scenario_id`, `status`, `claim_type`, `limit`, `offset` |
| `POST /v1/beliefs/{id}/debt/retire` | `debt_item` in body |
| `POST /v1/beliefs/{id}/promote` | Query param: `scenario_id` |
| `POST /v1/beliefs/{id}/retract` | Query param: `scenario_id` |
| `GET /v1/beliefs/{id}/explain` | Query param: `scenario_id` |
| `GET /v1/beliefs/{id}/evidence` | Query param: `scenario_id` |
| `POST /v1/evidence` | Full body; no mismatch |
| `GET /v1/evidence/{id}` | Path param only |
| `POST /v1/principals` | `principal_type`, `issuer` — no mismatch |
| `GET /v1/principals/{id}` | Path param only |
| `GET /v1/principals` | Query params: `limit`, `offset` |
| `POST /v1/principals/{id}/revoke` | Path param only |
| `POST /v1/targets` | Full body; no mismatch |
| `GET /v1/targets/{id}` | Path param only |
| `GET /v1/targets` | Query params: `limit`, `offset` |
| `POST /v1/targets/{id}/request` | Path param only |
| `POST /v1/discharge` | Full body; no mismatch |
| `GET /v1/activity` | Query params: `scenario_id`, `type`, `limit`, `offset` |
| `GET /v1/ledger` | Query param: `scenario_id` |

---

## Authorization Flow Summary

```
1. Create principal
   POST /v1/principals → PrincipalResponse

2. Create target (tuples)
   POST /v1/targets → TargetResponse

3. Attach justification
   POST /v1/targets/{id}/justifications?belief_id=uuid → TargetResponse

4. Request authorization
   POST /v1/targets/{id}/request → TargetResponse

5. Approve target
   POST /v1/targets/{id}/approve → TargetResponse

6. Verify authority
   POST /v1/authorizations/verify → AuthResult

7. Authorize action
   POST /v1/authorizations/action → AuthorizeActionResult
```

---

## Known v1 API Limitation: Approval Pin Not Exposed

**Affected endpoint:** `POST /v1/targets/{id}/request`

**Behavior:** `RequestAuthorization` computes a SHA-256 hash over the proposal tuple + justifications and stores it in `authority_target.pinned_request_hash`. The `TargetResponse` does not include this value.

**Consequence:** The caller cannot complete the approve step (`POST /v1/targets/{id}/approve`) without knowing the hash. The hash must be obtained through a separately secured channel.

**Status:** Known v1 API limitation, not introduced by Phase 4C. Resolution belongs to a future API version when the approval mechanism is redesigned.

---

## Extension Points (Phase 4C+, design only)

```
Future executor:
  ExecuteAction
    → current-state revalidation
    → kernel.Authorize
    → Executor (registry-resolved)
    → external provider
```

The executor boundary is not implemented in Phase 4C. Phase 4C stops at documentation of the contract.
