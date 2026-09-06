# Phase 4C Adversarial Review — Independent Report

**Reviewer role:** Independent security reviewer. Did not implement Phase 4C. All findings are rediscovered from direct repository inspection.

**Date:** 2026-09-06

---

## 1. Verdict

**READY WITH SPECIFIC FIXES**

Phase 4C is functionally complete and the core deliverables are correct, but one explicit acceptance gate fails (`task lint:openapi`). There are also three documentation/contract gaps that should be closed before freeze. None of the findings require kernel changes, schema migrations, or API behavior modifications.

---

## 2. Findings

### F-1 (BLOCKER): Spectral lint fails on invalid rule references

**Location:** `docs/openapi/.spectral.yaml`

**Evidence:**
```
Error #1: Cannot extend non-existing rule: "paths-kebab-case"
```

Both `paths-kebab-case` and `security-defined` are not valid rules in the installed Spectral CLI version (`@stoplight/spectral-cli@6.16.3`). This causes `task lint:openapi` to exit with status 2.

**Impact:** The explicit acceptance gate `[OPENAPI] Spectral lint passes on docs/openapi/solvent.yaml` fails. CI contract validation is broken.

**Fix:** Remove the two invalid rules from `.spectral.yaml`. The built-in `spectral:oas` ruleset already provides `operation-operationId`, `operation-tags`, and `operation-description` checks. A corrected minimal ruleset:
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

---

### F-2 (SECURITY — PRE-EXISTING, UNDOCUMENTED): `discharged_by` is caller-supplied without principal validation

**Location:** `api/discharge.go` lines 28–34, `api/types.go` line 201, `docs/api/security.md`

**Code path:**
```go
// api/discharge.go
if err := validateNonEmpty(req.DischargedBy, "discharged_by"); err != nil {
    writeValidationError(w, "discharged_by", err.Error(), "")
    return
}
if err := s.ledger.Discharge(r.Context(), req.BeliefID, req.ObligationKey,
    req.InstrumentRef, req.DischargedBy); err != nil {
```

The handler accepts `discharged_by` from the JSON body and passes it directly to `kernel.Discharge`, which inserts it into `debt_discharge.discharged_by` without checking it against the authenticated principal.

**Contrast with other identity fields:**
- `attached_by` → derived from `AuthFromContext` (target.go line 122)
- `approved_by` → derived from `AuthFromContext` (target.go line 189)
- `revoked_by` → derived from `AuthFromContext` (target.go line 223)
- `principal_id` in verify → derived from `AuthFromContext` (authorization.go line 31)

**Documentation gap:** `docs/api/security.md` "Server-Derived Identity" table omits `discharged_by`. The OpenAPI spec documents it as caller-supplied, which is faithful to the implementation, but the security docs should call it out explicitly.

**Impact:** Any authenticated caller can discharge obligations on behalf of any principal by supplying a different `discharged_by` UUID. This is a pre-existing Phase 4B behavior that Phase 4C faithfully documents, but the security docs should call it out explicitly.

---

### F-3 (SECURITY — SEMANTIC DIVERGENCE): MCP `actor_id` lacks Phase 6.1 binding

**Location:** `cmd/solvent-mcp/tools.go` lines 209–258, `docs/api/extensions.md`

**REST API behavior (authorization.go lines 104–109):**
```go
if req.ActorID != "" && req.ActorID != effectiveActor {
    writeError(w, http.StatusForbidden, "actor_id_mismatch", ...)
    return
}
```

**MCP behavior (tools.go lines 227–228, 247–248):**
```go
actorID, _ := args["actor_id"].(string)
// ...
tuple := kernel.AuthorityTuple{
    PrincipalID: actorID,  // used directly, no binding check
```

The MCP tool requires `actor_id` as a mandatory argument and uses it directly in the `AuthorityTuple`. There is no `actor_id_mismatch` rejection in the MCP path. The MCP comment acknowledges this is a "trusted administrative surface," but the divergence from the REST API's Phase 6.1 binding is not documented in `docs/api/extensions.md`.

**Impact:** REST and MCP produce different input-validation outcomes for the same kernel call. The authority outcomes (allowed/denied) are identical because both paths reach `kernel.AuthorizeAndCreateIntent`, but the attribution and audit actor_id can differ.

---

### F-4 (CONTRACT — PRE-EXISTING): `VerifyAuthRequest.PrincipalID` is a dead field

**Location:** `api/types.go` line 158, `api/authorization.go` line 31

**Evidence:**
```go
// api/types.go
type VerifyAuthRequest struct {
    TargetID              string `json:"target_id"`
    PrincipalID           string `json:"principal_id"`   // <-- present in type
    ResourceType          string `json:"resource_type"`
    ...
}

// api/authorization.go — handler ignores req.PrincipalID
tuple := kernel.AuthorityTuple{
    PrincipalID: principal.PrincipalID,  // <-- from auth context, not body
    ...
}
```

The Go type accepts `principal_id` in the JSON body, but the handler discards it and uses the authenticated principal. The OpenAPI spec correctly omits `principal_id` from `VerifyAuthRequest`. This creates a type-level contract mismatch: a developer using the Go types as a reference client would include `principal_id`, which the server silently ignores.

**Impact:** Low. The server is secure (ignores the field), but the Go type is misleading and contradicts the canonical OpenAPI contract.

---

### F-5 (DOCUMENTATION): Forbidden endpoints list missing from OpenAPI spec info description

**Location:** `docs/openapi/solvent.yaml` lines 1–19, `docs/OS/plan_4c.md` §7.5

**Plan requirement:**
> Document the forbidden surface in the spec's `description` field at the info level.

**Actual spec:**
```yaml
info:
  title: Solvent Authorization Kernel API
  version: "1.0.0"
  description: |
    Transactional belief ledger for autonomous agents...
```

The 12 forbidden endpoint patterns (`/execute-without-authority`, `/set-authorized`, etc.) are not listed. The spec also does not contain any of these paths, which is the critical invariant, but the plan explicitly required the info-level documentation.

---

### F-6 (MINOR): OpenAPI tag casing mismatch causes Spectral warnings

**Location:** `docs/openapi/solvent.yaml` tags section vs. operation `tags` fields

**Global tags:**
```yaml
tags:
  - name: beliefs        # lowercase
  - name: evidence       # lowercase
```

**Operation tags:**
```yaml
/v1/beliefs/post:
  tags: [Beliefs]        # capitalized — mismatch
```

This produces 20+ `operation-tag-defined` warnings from the built-in `spectral:oas` ruleset. These are warnings, not errors, but they reduce the signal value of Spectral output.

---

### F-7 (MINOR): Python client default URL mismatch

**Location:** `examples/python/client.py` line 22, `scripts/demo/config.env`

The Python client defaults to `http://localhost:8080`, but the Solvent API default port per `cmd/solvent-api/main.go` is `:8080` while the demo config uses `SOLVENT_HTTP_PORT=8081`. This is a documentation mismatch, not a code bug — the client URL is configurable.

---

## 3. Verified Invariants

| Invariant | Status | Evidence |
|-----------|--------|----------|
| No kernel files functionally modified | ✅ PASS | `git diff HEAD -- kernel/` shows only test formatting changes |
| No new service boundaries introduced | ✅ PASS | `service/` diff is empty |
| No new schema migrations added | ✅ PASS | `db/` directory unchanged |
| Executor registry is empty (no production execution path) | ✅ PASS | `service/executor/executor.go` — empty `NewRegistry()`; `cmd/solvent-mcp/main.go` line 126 |
| All 26 canonical routes exist in both implementation and spec | ✅ PASS | `api/api.go` routes match `solvent.yaml` paths exactly |
| No forbidden endpoints exposed | ✅ PASS | Zero matches for forbidden patterns in `api/api.go` |
| REST API enforces Phase 6.1 actor binding | ✅ PASS | `authorization.go` lines 104–109: `actor_id_mismatch` → 403 |
| REST API rejects non-user_typed action_source | ✅ PASS | `authorization.go` lines 71–76: `action_source_tool_output` → 403 |
| AuthorityTuple constructed server-side in authorize-action | ✅ PASS | `authorization.go` lines 111–120: hardcoded dimensions from request fields |
| Server-derived identity fields not caller-controlled | ✅ PASS | `attached_by`, `approved_by`, `revoked_by`, `principal_id` all from `AuthFromContext` |
| Spec-in-sync test passes | ✅ PASS | 7/7 tests pass in `api/openapi/openapi_test.go` |
| Python reference flow executes against live API | ✅ PASS | Exit code 0; flow completes through authorize_action |
| GitHub example compiles | ✅ PASS | `go build .` in `examples/github/` succeeds |
| `go build ./...` | ✅ PASS | Clean build |
| `go vet ./...` | ✅ PASS | Clean |
| `go test -count=1 -p 1 ./...` | ✅ PASS | All packages pass |
| `go test -count=1 ./...` | ✅ PASS | All packages pass |
| `task test:openapi` | ✅ PASS | `go test -count=1 -v ./api/openapi/...` passes |

---

## 4. Test Evidence

### Build and Unit Tests
```
ok  	github.com/PithomLabs/solvent/adapter/github	0.005s
ok  	github.com/PithomLabs/solvent/api	25.502s
ok  	github.com/PithomLabs/solvent/api/openapi	0.026s
...
ok  	github.com/PithomLabs/solvent/service/executor	0.002s
```

### OpenAPI Contract Test
```
=== RUN   TestOpenAPISpec_CanonicalRoutes
--- PASS: TestOpenAPISpec_CanonicalRoutes (0.00s)
=== RUN   TestOpenAPISpec_NoExtraPaths
--- PASS: TestOpenAPISpec_NoExtraPaths (0.00s)
=== RUN   TestOpenAPISpec_MutatingEndpointsRequireBearerAuth
--- PASS: TestOpenAPISpec_MutatingEndpointsRequireBearerAuth (0.00s)
=== RUN   TestOpenAPISpec_SecurityDecision_VerifyAuthNoPrincipalID
--- PASS: TestOpenAPISpec_SecurityDecision_VerifyAuthNoPrincipalID (0.00s)
=== RUN   TestOpenAPISpec_SecurityDecision_AuthorizeActionHasOptionalActorID
--- PASS: TestOpenAPISpec_SecurityDecision_AuthorizeActionHasOptionalActorID (0.00s)
=== RUN   TestOpenAPISpec_BearerAuthSchemeDefined
--- PASS: TestOpenAPISpec_BearerAuthSchemeDefined (0.00s)
=== RUN   TestOpenAPISpec_GlobalSecurity
--- PASS: TestOpenAPISpec_GlobalSecurity (0.01s)
PASS
```

### Python Reference Flow (live API)
```
1. Created principal: ccc2f95d-1e3b-4bbb-a2b5-d782067f3102
2. Entered belief: a6a2c0d1-5e41-4c9e-a0c4-59adeb4dec40
3. Retired 6 debts
4. Belief status: promoted
5. Created target: cd7631de-1bef-45b4-a789-7917aec2642e
6. Attached justification
8. Requested authorization
9. Verify result: allowed=False
10. Action authorized: allowed=False
    Reason: no activation or revocation exists
Exit code: 0
```

The `allowed=False` results are correct: the target was not approved (approval step is intentionally skipped in the reference flow due to the known v1 limitation that the approval pin is not exposed in `TargetResponse`).

### Spectral Lint (FAILURE)
```
Error #1: Cannot extend non-existing rule: "paths-kebab-case"
Error #2: Cannot extend non-existing rule: "security-defined"
task: Failed to run task "lint:openapi": exit status 2
```

---

## 5. Residual Risks

| Risk | Severity | Pre-existing? | Mitigation |
|------|----------|---------------|------------|
| `discharged_by` caller-supplied without principal validation | Medium | Yes (Phase 4B) | Document in security model as known v1 limitation; fix in future API version |
| MCP/REST semantic divergence on `actor_id` binding | Low | Yes (by design) | Document divergence explicitly in `extensions.md` |
| `VerifyAuthRequest.PrincipalID` dead field in Go types | Low | Yes (Phase 4B) | Remove field or add comment marking it ignored |
| Approval pin not exposed in `TargetResponse` | Medium | Yes (Phase 4B) | Documented in decision record and Python README; fix requires API redesign |
| Spectral lint failure blocks CI | High | No (Phase 4C) | Remove invalid rules from `.spectral.yaml` |

---

## 6. Final Phase 4C Disposition

### READY WITH SPECIFIC FIXES

**Required before freeze:**

1. **Fix `.spectral.yaml`** — Remove `paths-kebab-case` and `security-defined`. These rules do not exist in the installed Spectral CLI version and cause `task lint:openapi` to fail. This is the only acceptance gate failure.

2. **Add forbidden endpoints to OpenAPI spec info description** — The plan §7.5 requires:
   > Document the forbidden surface in the spec's `description` field at the info level.
   
   The current `solvent.yaml` info description does not list the 12 forbidden endpoint patterns.

3. **Document MCP/REST actor_id divergence** — Add a note in `docs/api/extensions.md` explaining that the MCP `solvent_authorize_action` tool requires caller-supplied `actor_id` without Phase 6.1 authenticated-principal binding, because MCP is a local stdio trusted surface. This makes the semantic divergence explicit.

4. **Document `discharged_by` in security model** — Add `discharged_by` to the `docs/api/security.md` table, either as a caller-supplied field or as a known limitation requiring future server-derivation.

**Recommended (non-blocking):**

5. Fix OpenAPI tag casing — Change global tags to `Beliefs`, `Evidence`, etc. (matching operation tags), or change operation tags to lowercase. This eliminates 20+ Spectral warnings.
6. Consider removing `PrincipalID` from `api/types.go` `VerifyAuthRequest` struct, or add a comment that the field is ignored by the handler.

### What is correct and should not be changed

- The canonical API decision record (`docs/OS/phase4c_api_decisions.md`) is complete and correctly resolves all M-1 through M-6 mismatches.
- The OpenAPI spec accurately documents the frozen v1 API surface, including all security-hardened request/response shapes.
- The spec-in-sync test (`api/openapi/openapi_test.go`) correctly verifies route presence, method correctness, security scheme definition, and references the key security-sensitive schemas.
- The Python reference client (`examples/python/`) is a handwritten thin wrapper with zero authorization logic. It executes the full reference flow correctly against a live API.
- The GitHub integration (`examples/github/`) compiles and correctly demonstrates the evidence → API → authority lifecycle without embedding authorization logic.
- The executor documentation in `docs/api/extensions.md` correctly describes the empty registry, the `ActionFunc` contract, and the security boundaries.
- No kernel files were functionally modified. No new service boundaries were introduced. No schema migrations were added.
- All 26 documented routes have matching implementation handlers. No forbidden endpoints are present.
- The REST API correctly enforces all Phase 6.1 bindings: server-derived identity, actor_id mismatch rejection, and action_source guard.
