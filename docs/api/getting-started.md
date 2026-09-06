# Solvent API Getting Started

## Prerequisites

- Solvent API running locally (`task setup && task db:up`)
- An API key configured in `scripts/demo/config.env`

## Base URL

```
http://localhost:8080
```

## Authentication

All requests require a Bearer token:

```
Authorization: Bearer your-api-key
```

The token maps to a single `principal_id`. The authenticated principal is the
effective identity for all server-derived fields (`attached_by`, `approved_by`,
`revoked_by`, `principal_id` in verify).

## Minimal Flow

```bash
# 1. Create a principal
curl -X POST http://localhost:8080/v1/principals \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"principal_type":"service","issuer":"my-agent"}'

# 2. Enter a belief
curl -X POST http://localhost:8080/v1/beliefs \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"scenario_id":"00000000-0000-0000-0000-000000000001","claim":"etcd v3.5.0 is safe","claim_type":"derived"}'

# 3. Promote the belief
curl -X POST "http://localhost:8080/v1/beliefs/{belief_id}/promote?scenario_id=00000000-0000-0000-0000-000000000001" \
  -H "Authorization: Bearer $API_KEY"

# 4. Create a target
curl -X POST http://localhost:8080/v1/targets \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"principal_id":"...","resource_type":"scenario","resource_id":"...","scope":"belief:...","action_namespace":"solvent","action_name":"deploy","consequence_type":"execution","created_by":"..."}'

# 5. Authorize an action
curl -X POST http://localhost:8080/v1/authorizations/action \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"scenario_id":"...","belief_id":"...","action":"deploy etcd v3.5.0","action_source":"user_typed","target_id":"..."}'
```

## Key Concepts

- **Beliefs** carry claims about the world. They have debt, can be promoted
  when debt is retired, and can be retracted.
- **Targets** are authorization requests with tuple fields. They go through
  propose → request → approve → verify lifecycle.
- **Authority** is gated by CockroachDB invariants. The database — not the
  API — determines whether an action is allowed.
- **Execution** is the boundary where Solvent meets the outside world.
  Phase 4C documents the executor contract; execution is not yet implemented.

## Error Handling

All errors return a canonical `APIError`:

```json
{
  "code": "validation_error",
  "message": "invalid field: scenario_id",
  "retryable": false,
  "sqlstate": "23514",
  "constraint_name": "promoted_is_debt_free"
}
```

SQLSTATE codes are evidence — they indicate which database constraint was
violated. `23514` is a CHECK violation; `23503` is a foreign key violation.
