# Solvent Python Reference Client

Handwritten reference client for the Solvent Authorization Kernel API. This
is NOT an SDK — it demonstrates the minimum surface needed to interact with
the canonical v1 HTTP API.

## Files

- `client.py` — Reference client with methods for each API resource
- `basic_authorization.py` — Authorization lifecycle demonstration
  (approval step skipped — see Known Limitations below)

## Usage

```python
from client import SolventClient

client = SolventClient("http://localhost:8080", "your-api-key")
```

The API URL is configurable. The `basic_authorization.py` example reads it
from the `SOLVENT_URL` environment variable (defaults to
`http://localhost:8080`). Override with:

```bash
SOLVENT_URL=http://localhost:8081 SOLVENT_API_KEY=your-key python basic_authorization.py
```

# Create a principal
principal = client.create_principal("service", "my-agent")

# Enter a belief
belief = client.enter_belief(scenario_id, "etcd v3.5.0 is safe", "derived")

# Promote the belief
client.promote_belief(belief["belief_id"], scenario_id)

# Create a target
target = client.create_target(
    principal_id=principal["principal_id"],
    resource_type="scenario",
    resource_id=scenario_id,
    scope=f"belief:{belief['belief_id']}",
    action_namespace="solvent",
    action_name="deploy",
    consequence_type="execution",
    created_by=principal["principal_id"],
)

# Authorize an action
result = client.authorize_action(
    scenario_id=scenario_id,
    belief_id=belief["belief_id"],
    action="deploy etcd v3.5.0",
    target_id=target["target_id"],
)
```

## Security Notes

- `actor_id`, when supplied, must equal the authenticated principal; the API
  derives the effective principal from authentication (Phase 6.1).
- `action_source` must equal `"user_typed"`. Tool output is rejected with 403.
- `principal_id`, `approved_by`, `revoked_by` are all server-derived from the
  authenticated principal. Callers cannot impersonate.

## Known Limitations

**Approval pin not exposed by API.** The `TargetResponse` does not include
`pinned_request_hash`. The caller cannot complete the approve step
(`POST /v1/targets/{id}/approve`) without knowing the hash. This is a
known v1 API limitation, not introduced by Phase 4C. The approval
credential must be obtained through a separately secured channel.

The `basic_authorization.py` example demonstrates the consumer-visible
lifecycle up to the frozen approval boundary.
