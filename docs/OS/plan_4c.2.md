# Phase 4C Remediation Plan (plan_4c.2)

**Objective:** Close all findings from the authoritative adversarial review (`docs/OS/phase_4c_adv_review.md`), then hand the corrected tree back to the independent reviewer for fresh GO/REQUEST CHANGES.

**Constraints:** No kernel changes, no authority model changes, no new services, no migrations, no executor changes, no test weakening.

---

## F-1 (BLOCKER): Fix Spectral lint config

**File:** `docs/openapi/.spectral.yaml`

**Action:** Remove `paths-kebab-case` and `security-defined` rules that do not exist in installed Spectral `@stoplight/spectral-cli@6.16.3`. Replace with corrected minimal ruleset:

```yaml
extends:
  - spectral:oas
rules:
  operation-operationId: warn
  operation-tags: warn
  tag-description: off
  no-eval-in-markdown: off
  oas3-valid-media-example: off
  info-contact: off
  info-license: off
```

**Verification:** `task lint:openapi` exits 0.

---

## F-2 (REQUIRED ANALYSIS): `discharged_by` audit attribution

**Code trace result:** `discharged_by` is pure audit attribution.

- `api/discharge.go:28-34` — accepts from JSON body, passes to kernel
- `kernel/authority.go:552-564` — inserts into `debt_discharge` table, retires debt
- `kernel/sql.go:187-193` — `INSERT INTO debt_discharge` + `UPDATE belief SET debt = array_remove(...)`
- `db/005_authority_mvp.sql:215-248` — `debt_discharge` table: `discharged_by UUID NOT NULL REFERENCES principal(principal_id)`, schema comment at line 237: "discharged_by -> principal provides attribution. It is NOT cryptographic proof or non-repudiation."
- `authorizeWithinTx` never reads `debt_discharge` — the field does not affect authorization decisions
- `Approve` at `kernel/authority.go:322-331` explicitly validates revocation, but `Discharge` does not — this is documented as an accepted v0 gap in `kernel/authority_test.go:877-902` (test T-27)
- Uniqueness constraint is on `(belief_id, obligation_key, instrument_ref)`, not `discharged_by` — two callers can't double-discharge, regardless of who claims it

**Action:** Update `docs/api/security.md` to add `discharged_by` to the "Server-Derived Identity" table as a known v1 limitation (caller-supplied, not server-derived, pure attribution, no authorization impact). Document that revocation enforcement for discharge is a service-layer concern not yet implemented in v0.

**Files to modify:** `docs/api/security.md`

---

## F-3 (REQUIRED DECISION): MCP `actor_id` divergence

**Code trace result:** MCP is a trusted local administrative surface.

- Transport: stdio only (`cmd/solvent-mcp/main.go:564` — `&mcp.StdioTransport{}`)
- No authentication middleware anywhere in `cmd/solvent-mcp/`
- MCP has full write access: 10+ mutating tools including `solvent_authorize_action`, `solvent_approve`, `solvent_create_principal`, `solvent_revoke_target`
- Explicit trust boundary comments: `tools.go:206-208` ("MCP trust boundary: this is a stdio-based local process. The actor_id comes from the tool arguments, not from authenticated credentials."), `main.go:449` ("Only call from a trusted administrative surface"), `main.go:322` ("the v0 MCP server must be deployed as a trusted administrative surface")
- MCP is spawned locally by an MCP host (Claude Desktop, VS Code) — the operator is the local user
- REST API has `AuthMiddleware` (`api/auth.go`), `AuthFromContext`, `actor_id_mismatch` rejection — MCP has none of these

**Decision:** MCP is a trusted local administrative surface, equivalent to a CLI. The `actor_id` is an attribution input, not an authenticated credential. The authority engine still enforces authorization via `authorizeWithinTx` (same kernel call as REST), so an MCP caller cannot authorize actions they don't have authority for — but the attribution of *who* performed the action is caller-declared.

This is an explicit architectural choice, not an accidental leftover. The REST API's Phase 6.1 binding exists because it serves untrusted network callers. The MCP server's stdio transport provides the trust boundary.

**Action:** Update `docs/api/extensions.md` section 3 ("MCP Integration") to:
1. Correct current inaccuracies (says "read-only" and "six tools" — both false; MCP has 16 tools, 10+ mutating)
2. Document the trust model: MCP is a trusted local administrative surface, stdio-only, no authentication, caller-declared `actor_id`
3. Explain why MCP diverges from REST Phase 6.1 binding: the stdio transport is the trust boundary
4. Document that `actor_id` in MCP is attribution, not authentication, and the authority engine still enforces authorization via target/snapshot approval workflow

**Files to modify:** `docs/api/extensions.md`

---

## F-4 (RECOMMENDED): `VerifyAuthRequest.PrincipalID` dead field

**Code trace:**
- `api/types.go:158` — `PrincipalID string` field present in struct
- `api/authorization.go:31` — handler uses `principal.PrincipalID` from `AuthFromContext`, never reads `req.PrincipalID`
- OpenAPI spec correctly omits `principal_id` from `VerifyAuthRequest` schema

**Decision:** Remove `PrincipalID` from `VerifyAuthRequest` struct.

Rationale:
- The API is frozen — removing an unused Go field is not an HTTP API change (no JSON wire change since it was already ignored)
- Keeping the field is misleading — developers using Go types as reference would include a field the server silently ignores
- The OpenAPI spec is the canonical contract and it correctly omits the field — the Go type should match
- No external consumers exist yet (pre-release)
- Adding a comment is less clean than removing the dead field

**Action:** Remove `PrincipalID string` from `VerifyAuthRequest` in `api/types.go`.

**Files to modify:** `api/types.go`

**Verification:** `go build ./...`, `go vet ./...`, `go test -count=1 -p 1 ./api/...`

---

## F-5 (REQUIRED): Forbidden endpoint list in OpenAPI info description

**Action:** Add the 12 forbidden endpoint patterns to the `info.description` field of `docs/openapi/solvent.yaml`:

```
Forbidden endpoints (Phase 4A §8): The following patterns are
structurally prohibited and will never be added:
/execute-without-authority, /set-authorized, /approve-with-agent,
/update-authority-state, /force-execution, /override-authorization,
/set-actor, /impersonate, /approve-and-execute, /execute-as-approved,
/trust-tool-output, /cache-authorization.
```

**Files to modify:** `docs/openapi/solvent.yaml` (info.description block, lines 5-15)

---

## F-6 (RECOMMENDED): OpenAPI tag casing consistency

**Current state:** Operations use `[Beliefs]`, `[Evidence]`, etc. (capitalized). No global `tags:` section exists. This produces `operation-tag-defined` Spectral warnings.

**Action:** Add global `tags:` section before `paths:` with capitalized names matching operations:

```yaml
tags:
  - name: Beliefs
    description: Belief lifecycle operations
  - name: Evidence
    description: Evidence submission and retrieval
  - name: Principals
    description: Principal identity management
  - name: Targets
    description: Authority target lifecycle
  - name: Authorizations
    description: Authorization verification and action authorization
  - name: Discharge
    description: Debt discharge and retirement
  - name: Activity
    description: Audit activity log
  - name: Ledger
    description: Ledger summary and status
```

**Files to modify:** `docs/openapi/solvent.yaml` (insert before `paths:`)

**Verification:** `task lint:openapi` warnings reduced by 20+.

---

## F-7 (OPTIONAL): Python example port mismatch

**Current state:** Demo config uses `SOLVENT_HTTP_PORT=8081`. Code default is `:8080`. Python examples use `http://localhost:8080`. The Python examples are correct relative to the code default. The demo config is the outlier.

**Action:** Add a note in `examples/python/README.md` that the port is configurable via `SOLVENT_URL` environment variable and defaults to 8080.

**Files to modify:** `examples/python/README.md`

---

## Execution Order

1. F-1 (Spectral config) — unblocks `task lint:openapi`
2. F-5 (forbidden endpoints in OpenAPI) — spec change
3. F-6 (global tags) — spec change, combines with F-5
4. F-2 (security.md update) — documentation
5. F-3 (extensions.md MCP section rewrite) — documentation
6. F-4 (remove dead Go field) — code change
7. F-7 (port doc) — documentation

## Verification

Run all acceptance gates:

```bash
task lint:openapi
task test:openapi
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
go test -count=1 ./...
```

## Hand-off

After all fixes pass verification, hand the corrected tree to the **same independent adversarial reviewer** (separately launched, not the implementation agent). The reviewer must independently re-check F-1 through F-7 and return fresh GO/REQUEST CHANGES.
