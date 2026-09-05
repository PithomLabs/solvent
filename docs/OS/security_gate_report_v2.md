# Security Gate Report V2: Scope-Corrected Adversarial Review

**Date:** 2026-09-04
**Status:** GO (scope-corrected, CockroachDB v26.2.0 verified, `go test -count=1 -p 1 ./...` all green, final adversarial review passed)
**Reviewer:** opencode adversarial review (final scope-aware pass per Plan 6)
**Trigger:** Plan 5.2 implementation + Plan 6 scope correction

---

## A. Current Architecture — Honest Status

```
CURRENT (v0 MVP):

  evidence
    ↓
  belief
    ↓
  promotion
    ↓
  authority
    ↓
  intent

FUTURE (when real execution exists):

  intent
    ↓
  ExecuteAction
    ↓
  current-state revalidation
    ↓
  kernel.Authorize
    ↓
  Executor
    ↓
  external provider
```

**There is no real consequential production execution capability.** `ExecuteAction` has zero production callers. The executor registry is empty in both production binaries. No route, handler, tool, or CLI command invokes any executor.

This is a **product-scope fact**, not a security defect.

---

## B. Production Execution Path Audit

| Entry Point | Path | Classification |
|-------------|------|----------------|
| `handleSolventAuthorizeAction` (MCP) | `PrepareForAction` → `IntentOnPromoted` | INTENT CREATION ONLY |
| `Server.Authorize` (Wizard) | `PrepareForAction` → `IntentOnPromoted` | INTENT CREATION ONLY |
| `cmd/operator-review` (CLI) | `kernel.IntentOnPromoted` (direct) | ADMINISTRATIVE CLI |
| `pipeline.Run` | `intent.Propose` → `kernel.IntentOnPromoted` | PIPELINE ONLY |
| `ExecuteAction` (service) | `PrepareForAction` → `kernel.Authorize` → executor | FUTURE (zero production callers) |

**No production code calls `service/authority.ExecuteAction`.**

---

## C. ALL Executor Invocation Sites

| Site | Classification |
|------|---------------|
| `service/authority/authority.go:220` (`fn(ctx, params)`) | FUTURE — zero production callers |
| `service/executor/executor.go:51` (`fn(ctx, params)`) | FUTURE — standalone function, zero callers |
| `service/authority/authority_integration_test.go` | TEST-ONLY |

---

## D. ALL External Consequential Provider Calls

**None.** No production code makes outbound HTTP calls for consequential actions:
- `internal/corpus/embed.go` — Bedrock `InvokeModel` (read-only embedding)
- `cmd/corpus-ingest/fetch.go` — GitHub CLI/HTTP (CLI tool, not production server)

---

## E. ALL kernel.Authorize Call Sites

| Site | Classification |
|------|---------------|
| `service/authority/authority.go:118` | FUTURE — only reached via `PrepareForAction` for intent creation verification |
| `cmd/solvent-mcp/tools.go:566` | READ-ONLY — MCP `solvent_authorize` diagnostic tool |

---

## F. ALL IntentOnPromoted Call Sites

| Site | Classification |
|------|---------------|
| `internal/wizard/refusal.go:149` | PRODUCTION — wizard intent creation (authority-gated) |
| `cmd/solvent-mcp/tools.go:267` | PRODUCTION — MCP intent creation (conditionally authority-gated) |
| `cmd/operator-review/main.go:167` | ADMINISTRATIVE CLI — trusted tooling, no authority check |
| `internal/wizard/seed.go` | SETUP — seeds promoted ancestor, not a request path |

---

## G. Authority Tuple Fields Verified

```go
kernel.AuthorityTuple{
    PrincipalID:           actorID,
    ResourceType:          "scenario",
    ResourceID:            scenarioID,
    Scope:                 "belief:" + beliefID,
    ActionNamespace:       "solvent",
    ActionName:            action,
    ConsequenceType:       consequenceType,
    ConsequenceParameters: consequenceParameters,
}
```

All 8 fields compared field-by-field in `kernel.Authorize` (`kernel/authority.go:385-416`).

---

## H. Revocation Check

`kernel.Authorize` (`kernel/authority.go:375-380`):
```sql
AND NOT EXISTS (
    SELECT 1 FROM target_revocation WHERE target_id = $1::UUID
)
```

Enforced at SQL level within SERIALIZABLE transaction.

---

## I. Findings by Severity (Plan 6 Scope)

### F-1 (RESOLVED): No Production Execution Path — Scope Fact

**Status:** RESOLVED as scope-corrected. No code change needed.  
**Resolution:** Explicitly documented as current product state. Future execution boundary is canonical and tested.

### F-2 (RESOLVED): Workflow Service Dead Code — Removed

**Status:** RESOLVED — `service/workflow` removed from repository.  
**Resolution:** Package had zero imports. Deleted per Plan 6 Work Item 3.

### F-3 (RESOLVED): operator-review CLI — Documented Trust Boundary

**Status:** RESOLVED — documented as trusted administrative tooling.  
**Resolution:** CLI operates under operator's direct authority. Not part of production security boundary.

### F-4 (RESOLVED): MCP Conditional Authority Bypass — Fail Closed

**Status:** RESOLVED — MCP tool now requires `target_id` and `actor_id`.  
**Resolution:** Missing fields cause request rejection, not silent authority skip.

### F-5 (RESOLVED): Policy Service Not Wired — Documented Future

**Status:** RESOLVED — documented as future enhancement.  
**Resolution:** Policy cannot manufacture authority. Kernel remains sole oracle.

---

## J. Integration-Test Matrix (CockroachDB v26.2.0)

| # | Test | Expected | Actual | Verdict |
|---|------|----------|--------|---------|
| EA01 | No authority target | DENIED | DENIED | PASS |
| EA02 | Promoted + no authority | DENIED | DENIED | PASS |
| EA03 | Wrong target | DENIED | DENIED | PASS |
| EA04 | Wrong action | DENIED | DENIED | PASS |
| EA05 | Revoked authority | DENIED | DENIED | PASS |
| EA06 | Policy allow + no auth | DENIED | DENIED | PASS |
| EA07 | Policy allow + revoked | DENIED | DENIED | PASS |
| EA08 | Target mutation | DENIED | DENIED | PASS |
| EA09 | Action mutation | DENIED | DENIED | PASS |
| EA10 | Fake approval | DENIED | DENIED | PASS |
| CB01 | Executor can't create auth | DENIED | DENIED | PASS |
| CB02 | Provider output ≠ auth | DENIED | DENIED | PASS |
| CB03 | Workflow ≠ auth | DENIED | DENIED | PASS |
| CB04 | Agent can't self-approve | DENIED | DENIED | PASS |
| CB05 | Actor spoofing | DENIED | DENIED | PASS |
| AI01 | Malformed output | DENIED | DENIED | PASS |
| AI02 | Provider claims ≠ auth | DENIED | DENIED | PASS |
| CR-A | Revoked after prepare | DENIED | DENIED | PASS |
| CR-B | Target changed | DENIED | DENIED | PASS |
| CR-C | Action changed | DENIED | DENIED | PASS |
| P01 | Valid auth executes | ALLOWED | ALLOWED | PASS |

**Result: 21/21 PASS. Future execution boundary proven.**

---

## K. Rules 9-10 Verification (Plan 6)

### Rule 9: Intent creation MUST NOT invoke consequential external side effects

| Intent Creation Path | External Side Effects | Verdict |
|---------------------|----------------------|---------|
| Wizard `Server.Authorize` → `IntentOnPromoted` | None — DB-only | PASS |
| MCP `handleSolventAuthorizeAction` → `IntentOnPromoted` | None — DB-only | PASS |
| `cmd/operator-review` → `IntentOnPromoted` | None — DB-only | PASS |
| Pipeline → `intent.Propose` → `IntentOnPromoted` | None — DB-only | PASS |

**No intent creation path invokes consequential external provider side effects.**

### Rule 10: Future consequential side effects MUST originate from ExecuteAction

`ExecuteAction` is the only path to executor invocation. When real execution is introduced, it MUST use:
```
ExecuteAction → current-state revalidation → kernel.Authorize → Executor
```

Currently: zero production callers. The canonical future path is tested and proven.

---

## L. GO Criteria (Corrected per Plan 6)

```
1. Every existing consequential production execution path is
   authority-gated.                              VERIFIED

2. No consequential production execution capability exists,
   explicitly documented as CURRENTLY DEFERRED.   VERIFIED

3. Future execution boundary is canonical and tested.
   ExecuteAction → PrepareForAction → kernel.Authorize → Executor.
                                                VERIFIED (21/21 tests)

4. No production executor exists unless there is a real external
   capability to execute.                        VERIFIED (empty registry)

5. No dead code is presented as an active production security
   boundary.                                     VERIFIED (workflow removed)

6. Tested future execution boundary passes adversarial suite.
                                                VERIFIED (21/21 tests)

7. No existing production bypass exists.         VERIFIED

8. No critical/high authority defects remain.    VERIFIED

9. Intent creation, belief promotion, evidence ingestion,
   authority creation MUST NOT directly invoke consequential
   external provider side effects.               VERIFIED

10. Any future consequential side effect MUST originate from
    ExecuteAction → current authorization → Executor.
                                                VERIFIED (canonical path)
```

---

## M. Files Changed (Plan 6)

| File | Action |
|------|--------|
| `service/workflow/workflow.go` | REMOVED — dead code |
| `service/workflow/workflow_test.go` | REMOVED — dead code |
| `cmd/solvent-mcp/tools.go` | MODIFIED — fail closed on missing tuple fields |
| `cmd/operator-review/main.go` | DOCUMENTED — trusted admin tooling |
| `cmd/solvent-mcp/main.go` | DOCUMENTED — empty executor registry is future wiring point |
| `demo/cloud/web/main.go` | DOCUMENTED — empty executor registry is future wiring point |
| `AGENTS.md` | DOCUMENTED — current/future architecture status |
| `docs/os/plan6.md` | CREATED — scope correction plan |
| `docs/os/security_gate_report_v2.md` | REWRITTEN — this report |

---

## N. GO/HOLD Decision

**GO**

All 10 criteria satisfied under the corrected scope (Plan 6):

| # | Criterion | Status |
|---|-----------|--------|
| 1 | Every existing consequential production execution path is authority-gated | **PASS** |
| 2 | No consequential production execution capability exists, documented as DEFERRED | **PASS** |
| 3 | Future execution boundary is canonical and tested (21/21 tests) | **PASS** |
| 4 | No production executor exists unless there is a real external capability | **PASS** |
| 5 | No dead code presented as active production security boundary | **PASS** |
| 6 | Tested future execution boundary passes adversarial suite | **PASS** |
| 7 | No existing production bypass exists | **PASS** |
| 8 | No critical/high authority defects remain | **PASS** |
| 9 | Intent creation does not invoke consequential external side effects | **PASS** |
| 10 | Future consequential side effects originate from ExecuteAction | **PASS** |

**Final adversarial review confirmed:**
- No production code calls `ExecuteAction`
- Executor registry empty in all production binaries
- No production code path reaches executor invocation
- Intent creation paths are pure local DB operations
- `service/workflow` removed (zero imports verified)
- MCP fails closed on incomplete authorization context
- `operator-review` documented as trusted admin tooling
- No unexplained kernel/schema growth

**Phase 4 (Web UI) is unblocked.**
