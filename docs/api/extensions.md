# Solvent API Extension Architecture

## Current State

Phase 4C establishes the canonical v1 HTTP API. The kernel is frozen. The API
is frozen. Extensions are possible without modifying either.

## Extension Points

### 1. Evidence Adapters

Evidence sources are `adapter/github/` implementations. Changing the source
does not require kernel changes.

```text
GitHub event → adapter/github → NormalizedEvidence
  → POST /v1/evidence
  → kernel ingests into belief
```

Future adapters follow the same pattern: translate external events into
`NormalizedEvidence` and call the API.

### 2. Executor Boundary

The executor boundary is designed but not yet implemented. The contract:

```text
ExecuteAction
  → current-state revalidation
  → kernel.Authorize
  → Executor (registry-resolved)
  → external provider
```

Phase 4C documents the contract. Phase 4C+ introduces the first real
consequential executor.

### 3. MCP Integration

The MCP server (`cmd/solvent-mcp/`) speaks JSON-RPC over stdio. It exposes
sixteen tools that mirror and extend the REST API surface, including
read-write operations (ingest evidence, promote beliefs, create principals,
approve targets, authorize actions, discharge debt, retract beliefs).

**Trust model:** The MCP server is a **trusted local administrative
surface**, equivalent to a CLI. It is stdio-only (no network listener, no
port binding). The MCP host (Claude Desktop, VS Code, etc.) spawns the
server as a local subprocess. The operator is the local user.

**`actor_id` semantics:** The `actor_id` in `solvent_authorize_action` is a
**caller-declared attribution input**, not an authenticated credential. There
is no authentication middleware, no Bearer token validation, and no
`actor_id_mismatch` rejection in the MCP path. The authority engine still
enforces authorization via the target/snapshot approval workflow — an MCP
caller cannot authorize actions they don't have authority for — but the
attribution of *who* performed the action is caller-declared.

**Divergence from REST:** The REST API derives the effective principal from
authenticated credentials and rejects conflicting `actor_id` values (Phase
6.1 identity binding). The MCP server does not. This is an explicit
architectural choice: the REST API serves untrusted network callers; the MCP
server's stdio transport provides the trust boundary.

**Forward-looking constraint:** This trust model applies to the current
stdio-only MCP server. It **MUST NOT** be generalized to future remote MCP
transports, agent-to-agent (A2A) protocols, agent-runtime integrations, or
any untrusted integration boundary. Any future remote or network-exposed
integration MUST implement authenticated-principal binding equivalent to the
REST API's Phase 6.1 model.

## What Phase 4C Does NOT Add

- No new kernel primitives
- No new schema migrations
- No new services
- No production executor
- No SDK
- No workflow engine
- No policy DSL

## Adding a New Evidence Adapter

1. Implement the adapter in `adapter/<source>/`
2. Translate external events into `NormalizedEvidence`
3. Call `POST /v1/evidence` with the normalized data
4. The kernel handles ingestion, belief creation, and evidence linking

No kernel changes required.

## Adding a New Executor

1. Implement the `ActionFunc` interface from `service/executor/`
2. Register the executor in the registry
3. Call `ExecuteAction` → `kernel.Authorize` → executor

No kernel changes required. The executor registry is empty by design.
