You are now implementing PHASE 4B — SOLVENT SERVICE/API IMPLEMENTATION.

The Phase 4A API contract has completed independent adversarial review and is
approved.

AUTHORITATIVE SPECIFICATION:

docs/OS/phase4a_api_contract.md

Do not redesign the API.

Do not reinterpret the API contract.

Implement the approved contract faithfully.

==================================================
1. HARD ARCHITECTURAL RULE
==================================================

The canonical architecture is:

Client / Protocol
    ↓
API boundary
    ↓
Service layer
    ↓
Solvent kernel
    ↓
CockroachDB

The kernel remains the smallest trusted authority core.

`kernel.Authorize` remains the final authority oracle.

`kernel.Approve` remains the authority-creation semantic boundary.

`kernel.RevokeTarget` remains the revocation semantic boundary.

Do not create any second authorization engine in:
- HTTP handlers
- service packages
- MCP
- A2A
- Web UI
- repositories
- middleware

==================================================
2. FIRST STEP — IMPLEMENTATION GAP ANALYSIS
==================================================

Before editing code:

1. Read the entire approved Phase 4A contract.
2. Inspect the current repository.
3. Map every Phase 4A operation to:
   - existing service
   - existing kernel capability
   - existing repository/storage path
   - existing MCP equivalent
   - missing implementation
4. Produce a concise implementation gap table.

For each API operation identify:

Operation
Current implementation
Missing component
Kernel interaction
Database impact
Security boundary
Tests required

Do NOT begin implementation until this mapping is complete.

==================================================
3. IMPLEMENT ONLY THE APPROVED CONTRACT
==================================================

Implement the smallest set of packages/interfaces necessary to establish
the canonical API.

Likely areas include:

- API request/response types
- authentication context abstraction
- actor/effective-principal resolution
- service interfaces
- service implementations
- canonical error mapping
- HTTP handlers/router
- audit integration
- request validation
- idempotency handling where Phase 4A requires it
- API integration tests

Do not invent additional resources or operations.

Do not add convenience endpoints merely because they are easy.

==================================================
4. AUTHENTICATION / ACTOR BINDING
==================================================

Preserve the Phase 4A distinction:

ACTOR TYPE != IDENTITY != AUTHENTICATION

Never treat:

actor_id
actor_type
created_by
approved_by
requested_by

as proof of identity.

The implementation must establish the authenticated principal at the API
boundary and derive the effective actor from trusted server-side context
according to the Phase 4A contract.

A caller must not be able to select another principal merely by changing
an actor identifier in JSON.

Do not implement enterprise IAM infrastructure.

Implement the abstraction specified by Phase 4A.

==================================================
5. AUTHORIZATION
==================================================

For every consequential operation:

API authentication
    ↓
API access control
    ↓
service validation/context construction
    ↓
current-state re-read
    ↓
kernel.Authorize
    ↓
allow / deny

Never trust:

- cached authorization
- browser state
- approval booleans
- caller-supplied authority state
- agent claims
- tool output
- stale snapshots
- previously returned authorization results

Do not create any service-level substitute for `kernel.Authorize`.

==================================================
6. AUTHORITY CREATION
==================================================

Only the approved API operation may invoke the authority-creating semantics
represented by `kernel.Approve`.

The client must not be able to construct authority state directly.

The API must not expose operations such as:

/set-authorized
/force-approval
/update-authority-state
/approve-with-agent
/execute-without-authority
/override-authorization
/approve-and-execute

or equivalent variants.

==================================================
7. EXECUTION
==================================================

There is currently NO production consequential executor.

Do not invent one.

Do not add provider side effects.

Do not create a production `/execute` endpoint merely because the API contract
describes the future boundary.

Preserve the future architecture:

API
 → ExecutionService
 → current-state revalidation
 → kernel.Authorize
 → Executor
 → provider

Any implementation work related to that boundary must remain explicitly
future/test-only unless Phase 4A identifies otherwise.

==================================================
8. MCP / A2A
==================================================

MCP and A2A are adapters.

They must converge on the same service semantics as the canonical API.

Do not duplicate authorization logic.

Where an existing MCP operation already exists:

- preserve compatible semantics
- route through the canonical service boundary where appropriate
- do not create a second security implementation

A2A may remain a future adapter where Phase 4A specifies it as future.

==================================================
9. EXISTING WIZARD API
==================================================

`internal/wizard/http.go` and `/demo/api/*` are legacy/demo transport.

Do not make them the canonical API.

Where practical, migrate/reuse them through the new service boundary.

Do not allow wizard-specific semantics to become the domain model.

Do not delete functioning demo behavior merely for architectural cleanliness.

==================================================
10. ERROR CONTRACT
==================================================

Implement the canonical Phase 4A error taxonomy.

Errors must be:

- stable
- machine-readable
- language-neutral
- safe
- deterministic

Do not expose unnecessary internal implementation details.

Preserve SQLSTATE internally/audit-side only where the Phase 4A contract
requires it.

Do not couple the public semantic error model to CockroachDB internals.

==================================================
11. AUDIT
==================================================

Implement the approved audit/activity contract.

Keep distinct:

request
authentication
authorization decision
authority creation
revocation
adapter invocation
provider result
future execution

Preserve:

Authorize != Execute

Do not make the audit system a second security state store.

==================================================
12. DATABASE
==================================================

Prefer existing schema.

Do not modify authority-core schema unless Phase 4A explicitly requires it.

Do not introduce migrations merely for convenience.

Any new persistence must be clearly classified as:

- authority-core
- product/read-model
- audit/activity
- derived/cache

and justified against the Phase 4A contract.

==================================================
13. IDEMPOTENCY / CONCURRENCY
==================================================

Implement only the semantics specified by Phase 4A.

Verify:

- retry safety
- duplicate mutations
- concurrent approval
- duplicate revocation
- stale state
- wrong actor
- wrong target

Do not claim atomicity between Solvent DB state and future external provider
effects.

==================================================
14. TEST REQUIREMENTS
==================================================

Add contract and integration tests proving:

- authenticated principal is correctly established
- caller cannot impersonate another principal
- actor type alone is not trusted
- caller cannot set authority state directly
- kernel.Authorize is used for consequential authorization
- stale authority is rejected
- revoked authority is rejected
- wrong target is rejected
- wrong actor is rejected
- tool output cannot become authority
- malformed IDs fail safely
- MCP cannot bypass canonical service semantics
- API errors match the contract
- audit events are emitted correctly
- retries do not create unintended duplicate authority
- no production external side effect was introduced

Where existing security tests already cover a property, preserve them and
avoid redundant parallel mechanisms.

==================================================
15. IMPLEMENTATION ORDER
==================================================

Implement in small reviewed steps:

1. API/domain types
2. authentication context abstraction
3. actor/effective-principal binding
4. service interfaces
5. service implementations
6. canonical error model
7. HTTP/API transport
8. audit integration
9. idempotency/concurrency controls
10. MCP/service convergence
11. contract/integration/security tests
12. full repository verification

After each substantial step:

- run targeted tests
- inspect the diff
- confirm no kernel semantics changed

==================================================
16. ABSOLUTE DO-NOT-BUILD LIST
==================================================

Do NOT:

- expand the kernel for API convenience
- add workflow tokens
- reintroduce workflow authority semantics
- create a second authorization engine
- add production external execution
- add compliance/GRC infrastructure
- add UI
- add SDKs for every language
- add enterprise IAM systems
- make MCP authoritative over REST
- make A2A authoritative over REST
- make UI state authoritative
- expose raw DB tables as the public API
- trust caller-supplied actor identity
- trust agent approval claims
- trust tool output as authority
- create fake functionality for demos
- silently alter the approved Phase 4A contract

==================================================
17. VALIDATION GATE
==================================================

Before declaring Phase 4B complete, run:

go test -count=1 -p 1 ./...
go build ./...
go vet ./...

Also run targeted API/security integration tests.

Verify:

- no authority-core schema growth without explicit justification
- no new kernel primitive
- no production executor
- no workflow-token path
- no direct authorization bypass
- no alternate protocol authorization engine
- no actor impersonation path
- no hidden external side effect

Produce a final implementation report containing:

1. Implemented operations
2. Files changed
3. New interfaces/types
4. Database changes, if any
5. Security controls
6. Tests added
7. Existing tests preserved
8. Deferred items
9. Known limitations
10. Verification results

The final report must explicitly state:

- whether the kernel changed
- whether the authority schema changed
- whether production execution exists
- whether authentication is now enforced
- whether REST/MCP/A2A converge on the same service semantics

Do not proceed to Phase 4C work.

Finish with:

PHASE 4B IMPLEMENTATION COMPLETE

or

PHASE 4B NOT COMPLETE