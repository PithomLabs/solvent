# Phase 4C Final Independent Adversarial Review

**Reviewer:** Kilo Code (independent, did not implement Phase 4C)
**Date:** 2026-09-06
**Status:** FINAL

---

# 1. VERDICT

**GO**

Phase 4C is ready to freeze. All seven findings from the prior review have been remediated. The acceptance gates pass. No kernel changes, no schema changes, no new services, no production executor, no API behavior changes were introduced. The extension plane is correctly established above the frozen kernel.

---

# 2. EXECUTIVE ASSESSMENT

Phase 4C successfully established:

```
frozen kernel (unchanged)
    ↓
stable canonical API (OpenAPI 3.1 + decision record)
    ↓
extension plane (Python client, GitHub integration, executor docs)
```

The resulting API is safe and sufficiently unambiguous for external consumption. The critical security invariants are preserved:

- Authenticated-principal binding for REST API identity fields
- Server-side AuthorityTuple construction in authorize-action
- actor_id mismatch rejection in REST
- action_source guard rejecting tool output
- Empty executor registry (no production execution path)
- Forbidden endpoints absent from route table

The MCP trust boundary is explicitly documented as a trusted local administrative surface with caller-declared attribution, and the documentation correctly states this model must not be generalized to remote transports.

---

# 3. CRITICAL / HIGH FINDINGS

**None.**

---

# 4. MEDIUM / LOW / INFO FINDINGS

**None.**

All prior findings (F-1 through F-7) have been remediated. No new findings were discovered.

---

# 5. F-1 THROUGH F-7 RECHECK

| Finding | Status | Evidence |
|---------|--------|----------|
| F-1 Spectral config failure | **PASS** | `task lint:openapi` exits 0. Invalid `paths-kebab-case` and `security-defined` rules removed from `.spectral.yaml`. |
| F-2 `discharged_by` documentation gap | **PASS** | `docs/api/security.md` now has explicit "Caller-Supplied Attribution: `discharged_by`" section documenting it as caller-supplied, not server-derived, with known v0 gap noted. |
| F-3 MCP actor_id divergence | **PASS** | `docs/api/extensions.md` section 3 now documents MCP as trusted local administrative surface, explains stdio trust boundary, and explicitly states divergence from REST Phase 6.1 binding. |
| F-4 `VerifyAuthRequest.PrincipalID` dead field | **PASS** | `PrincipalID` removed from `api/types.go` `VerifyAuthRequest` struct. No wire-level behavior change (field was already ignored by handler). |
| F-5 Forbidden endpoints missing from OpenAPI info | **PASS** | `docs/openapi/solvent.yaml` info.description now lists all 12 forbidden endpoint patterns. |
| F-6 OpenAPI tag casing warnings | **PASS** | Global `tags:` section added to `solvent.yaml` with capitalized names (`Beliefs`, `Evidence`, etc.) matching operation tags. |
| F-7 Python port documentation mismatch | **PASS** | `examples/python/README.md` documents configurable `SOLVENT_URL` and shows override for port 8081. |

---

# 6. CONTRACT RECONCILIATION

| Mismatch | Status | Evidence |
|----------|--------|----------|
| M-1 AttachJustification | **PASS** | Decision record documents query-param `belief_id` + body `instrument_ref`. OpenAPI matches. Handler matches. |
| M-2 ApproveTarget | **PASS** | Decision record documents body `approval_pin` only; `approved_by` server-derived. OpenAPI matches. Handler matches. |
| M-3 RevokeTarget | **PASS** | Decision record documents body `reason` only; `revoked_by` server-derived. OpenAPI matches. Handler matches. |
| M-4 VerifyAuth | **PASS** | Decision record documents 8 tuple fields only; `principal_id` from auth. OpenAPI matches. Handler uses `principal.PrincipalID` from `AuthFromContext`. |
| M-5 AuthorizeAction actor_id | **PASS** | Decision record documents optional `actor_id` with 403 mismatch. OpenAPI matches. Handler enforces `actor_id_mismatch` → 403. |
| M-6 AuthorizeAction tuple | **PASS** | Decision record documents hardcoded server-side tuple. OpenAPI matches. Handler constructs `AuthorityTuple` with hardcoded dimensions. |

---

# 7. VERIFIED ARCHITECTURAL INVARIANTS

| Invariant | Status | Evidence |
|-----------|--------|----------|
| Kernel unchanged | **PASS** | `git diff HEAD -- kernel/` shows only test formatting changes |
| API unchanged | **PASS** | `git diff HEAD -- api/*.go` shows only F-4 field removal (cosmetic) and test formatting |
| Schema unchanged | **PASS** | `db/` directory unchanged |
| No new services | **PASS** | `service/` diff is empty |
| One authority engine | **PASS** | Only `kernel.Authorize` and `kernel.AuthorizeAndCreateIntent` exist |
| Authenticated principal binding | **PASS** | REST API: `attached_by`, `approved_by`, `revoked_by`, `principal_id` all from `AuthFromContext` |
| actor_id security semantics | **PASS** | REST: `actor_id_mismatch` → 403. MCP: caller-declared attribution, documented as trusted surface. |
| Server-side protected tuple construction | **PASS** | `authorization.go` lines 111-120: hardcoded dimensions from request fields |
| REST/MCP convergence | **PASS** | Both paths reach `kernel.AuthorizeAndCreateIntent` for authorization decisions |
| No forbidden endpoint | **PASS** | Zero forbidden patterns in `api/api.go` route table |
| No production executor | **PASS** | `cmd/solvent-mcp/main.go:126` — empty `executor.NewRegistry()`. No `execReg.Register(...)` in production code. |
| No execution bypass | **PASS** | `service/authority.ExecuteAction` not wired to API. Executor registry empty. |
| Evidence ≠ authority | **PASS** | Evidence ingestion (`POST /v1/evidence`) is separate from authority creation (`Approve`) |
| Policy ≠ authority | **PASS** | `service/policy` is advisory only; kernel is final authority oracle |
| Intent ≠ execution | **PASS** | `AuthorizeAndCreateIntent` creates intent; `ExecuteAction` not wired to API |
| Authorization ≠ execution | **PASS** | Authorization returns `AuthResult`; execution boundary is future |
| Extension plane above kernel | **PASS** | Python client, GitHub integration, MCP all call API/service layer; no kernel imports in examples |

---

# 8. TEST EVIDENCE

## Build and Vet
```
$ go build ./...
(no output)

$ go vet ./...
(no output)
```

## Go Tests (isolated)
```
$ go test -count=1 -p 1 ./...
ok  	github.com/PithomLabs/solvent/adapter/github	0.004s
ok  	github.com/PithomLabs/solvent/api	25.956s
ok  	github.com/PithomLabs/solvent/api/openapi	0.025s
...
ok  	github.com/PithomLabs/solvent/service/executor	0.002s
```

## Go Tests (parallel)
```
$ go test -count=1 ./...
ok  	github.com/PithomLabs/solvent/adapter/github	0.005s
ok  	github.com/PithomLabs/solvent/api	17.755s
ok  	github.com/PithomLabs/solvent/api/openapi	0.051s
...
ok  	github.com/PithomLabs/solvent/service/executor	0.024s
```

## OpenAPI Contract Test
```
$ task test:openapi
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

## Spectral Lint
```
$ task lint:openapi
(21 warnings, 0 errors)
Exit code: 0
```

Warnings are pre-existing `operation-description` warnings (missing `description` field where only `summary` is present). Not errors. Task passes.

## Python Syntax
```
$ python3 -m py_compile examples/python/client.py
$ python3 -m py_compile examples/python/basic_authorization.py
Python syntax OK
```

## Python Live Flow
```
$ SOLVENT_URL=http://localhost:8083 SOLVENT_API_KEY=test-key python3 basic_authorization.py
1. Created principal: c5a72e4b-d53d-4e73-9be5-c32fe248823c
2. Entered belief: 04427520-3be5-4df5-9609-a93974f9c210
3. Retired 6 debts
4. Belief status: promoted
5. Created target: 2f1bdeea-fb93-4101-924f-47eabacc3c68
6. Attached justification
8. Requested authorization
9. Verify result: allowed=False
10. Action authorized: allowed=False
    Reason: no activation or revocation exists
Exit code: 0
```

The `allowed=False` results are correct: target not approved (approval step intentionally skipped due to known v1 limitation).

## GitHub Example Build
```
$ go build ./examples/github/
(no output)
```

---

# 9. DOCUMENTATION / EXAMPLE CONSISTENCY

| Document | Consistent with Code? | Notes |
|----------|----------------------|-------|
| `phase4c_api_decisions.md` | **Yes** | All M-1 through M-6 decisions faithful to current implementation |
| `solvent.yaml` | **Yes** | 26 routes match implementation. Security-sensitive schemas match decision record. Forbidden endpoints listed in info description. |
| `getting-started.md` | **Yes** | Minimal flow matches OpenAPI. Server-derived identity fields correctly listed. |
| `security.md` | **Yes** | Server-derived identity table correct. `discharged_by` documented as caller-supplied. Actor ID binding documented. Tuple construction documented. |
| `extensions.md` | **Yes** | MCP trust boundary explicitly documented. Executor contract documented. Extension architecture consistent. |
| Python client | **Yes** | Handwritten thin wrapper. No authorization logic. Matches OpenAPI shapes. |
| GitHub integration | **Yes** | Calls REST API only. No kernel imports. No authority logic. Stops at approval boundary. |

No contradictions found between documents.

---

# 10. RESIDUAL RISKS

| Risk | Severity | Pre-existing? | Mitigation |
|------|----------|---------------|------------|
| `discharged_by` caller-supplied without principal validation | Medium | Yes (Phase 4B) | Documented in security.md as known v1 limitation |
| MCP `actor_id` caller-declared without Phase 6.1 binding | Low | Yes (by design) | Documented in extensions.md as trusted local surface; must not generalize to remote transports |
| Approval pin not exposed in `TargetResponse` | Medium | Yes (Phase 4B) | Documented in decision record and Python README |
| Spec-in-sync test does not deep-validate security schema internals | Low | Yes (Phase 4C) | Test checks schema references exist but not internal field constraints. Handlers enforce constraints. |
| Spectral warnings (21 `operation-description`) | Info | Pre-existing | Warnings only, not errors. Spec uses `summary` instead of `description` for many operations. |

---

# 11. FINAL PHASE 4C DISPOSITION

**READY TO FREEZE**

All acceptance gates pass:
- `go build ./...` ✅
- `go vet ./...` ✅
- `go test -count=1 -p 1 ./...` ✅
- `go test -count=1 ./...` ✅
- `task lint:openapi` ✅ (exit 0)
- `task test:openapi` ✅
- Python reference flow executes against live API ✅
- GitHub example compiles ✅
- No kernel changes ✅
- No schema changes ✅
- No new services ✅
- No production executor ✅

All seven prior findings (F-1 through F-7) have been remediated. No new findings were discovered during independent review.

The canonical API decision record, OpenAPI spec, security documentation, extension documentation, Python reference client, and GitHub reference integration form a coherent and consistent external contract for the frozen Solvent authorization foundation.
