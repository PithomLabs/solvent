# Solvent API Security Model

## Authentication

All API requests require a Bearer token. The token maps to a single
`principal_id` via the configured key-to-principal mapping.

```
Authorization: Bearer your-api-key
```

Missing or invalid tokens return `401 missing_authorization` or `401 invalid_token`.

## Server-Derived Identity

The following fields are **never caller-controlled**. They are derived from
the authenticated principal:

| Field | Endpoint | Derivation |
|-------|----------|------------|
| `attached_by` | `POST /v1/targets/{id}/justifications` | Authenticated principal |
| `approved_by` | `POST /v1/targets/{id}/approve` | Authenticated principal |
| `revoked_by` | `POST /v1/targets/{id}/revoke` | Authenticated principal |
| `principal_id` | `POST /v1/authorizations/verify` | Authenticated principal |

This is the Phase 6.1 identity-binding rule. Callers cannot impersonate other
principals.

## Caller-Supplied Attribution: `discharged_by`

`discharged_by` in `POST /v1/discharge` is **caller-supplied**, not
server-derived. This is a known v1 limitation.

| Field | Endpoint | Source |
|-------|----------|--------|
| `discharged_by` | `POST /v1/discharge` | Caller-supplied UUID |

**Attribution identity vs. authorization:**

- **Attribution identity** — `discharged_by` records who claims to have
  discharged the obligation. It is stored in `debt_discharge` as durable
  audit metadata.
- **Authorization to perform the discharge state transition** — the handler
  requires authentication (Bearer token) but does **not** validate that the
  authenticated principal matches `discharged_by`. Any authenticated caller
  can discharge on behalf of any principal by supplying a different UUID.

**What `discharged_by` does NOT do:**

- It does not participate in kernel authorization decisions.
- It does not affect whether an action intent may be created.
- It does not affect whether a belief may be promoted.
- It is not consulted by `authorizeWithinTx`.

**Known v0 gap:** The kernel does not validate that the `discharged_by`
principal is revoked. Revocation enforcement for discharge is a service-layer
concern not yet implemented in v0. (See `TestT27_RevokedPrincipalDischargeV0Behavior`.)

The schema comment documents this explicitly:
> `discharged_by -> principal provides attribution. It is NOT cryptographic
> proof or non-repudiation.`

## Actor ID Binding

`actor_id` in `POST /v1/authorizations/action` is optional. If present, it
**must equal** the authenticated principal. Mismatch returns HTTP 403
`actor_id_mismatch`.

```json
{
  "actor_id": "must-match-your-authenticated-principal"
}
```

## Action Source Guard

`action_source` in `POST /v1/authorizations/action` must equal `"user_typed"`.
Tool output or automated sources are rejected with HTTP 403
`action_source_tool_output`.

## Tuple Construction

`POST /v1/authorizations/action` constructs the `AuthorityTuple`
server-side:

| Field | Source |
|-------|--------|
| `resource_type` | Hardcoded: `"scenario"` |
| `resource_id` | From `scenario_id` request field |
| `scope` | Constructed: `"belief:" + belief_id` |
| `action_namespace` | Hardcoded: `"solvent"` |
| `action_name` | From `action` request field |
| `consequence_type` | Hardcoded: `"execution"` |
| `consequence_parameters` | Hardcoded: `{}` |

Callers cannot inject arbitrary authority dimensions. The narrow API surface
eliminates entire classes of authority injection.

## Database Invariants

The kernel enforces transactional correctness via CockroachDB constraints:

- **`promoted_is_debt_free`** (CHECK): A belief cannot be promoted if it has
  open debt.
- **`live_requires_promoted`** (CHECK): An intent cannot exist on a
  non-promoted belief.
- **`23503` gate**: Authority cannot reference a non-existent or revoked
  principal.

These are database-enforced, not application-enforced. The API returns the
raw SQLSTATE and constraint name in error responses.

## Error Response Structure

```json
{
  "code": "validation_error",
  "message": "human-readable message",
  "retryable": false,
  "sqlstate": "23514",
  "constraint_name": "promoted_is_debt_free"
}
```

SQLSTATE codes are evidence. `23514` is a CHECK violation. `23503` is a
foreign key violation. `40001` is a serialization error (retry signal, not
a refusal).
