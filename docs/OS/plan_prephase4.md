# Plan: Phase 4A — Canonical Solvent API Contract

## Source Prompt

`docs/OS/prompt12.md` — 28-section design/reconnaissance task producing a reviewable architectural contract.

## Context

`docs/OS/background_phase4.md` — API-first roadmap: 4A (Contract) → 4B (Implementation) → 4C (OpenAPI + References) → 4D (Optional UI) → 4E (Adversarial Review).

## Locked Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Deliverable format | Full 28-section Markdown document | Adversarial review requires complete contract |
| DealForge/AegisFlow | Use existing repo analyses only, no external fetch | Prevents architectural contamination |
| Baseline verification | Thorough — verify every material security claim | False premise poisons entire design |
| Output location | `docs/OS/phase4a_api_contract.md` | Fits existing `docs/OS` structure |
| Wizard HTTP API | Design new canonical contract; wizard is legacy/demo | Wizard is not the permanent architecture |

## Deliverable

Single Markdown file: `docs/OS/phase4a_api_contract.md`

No code changes. No schema changes. No kernel modifications.

---

## Phase 1: Deep Reconnaissance (parallel agents)

Launch 8 parallel explore agents to read every material package:

| Agent | Scope | Files | Purpose |
|-------|-------|-------|---------|
| A | Kernel | `kernel/kernel.go`, `kernel/authority.go`, `kernel/contract.go`, `kernel/errors.go`, `kernel/sql.go` | Contract interface, AuthorityTuple, errors, SQLSTATE, all 16 methods |
| B | Service authority + executor | `service/authority/*.go`, `service/executor/*.go` | PrepareForAction, ExecuteAction, executor registry (empty), execution boundary |
| C | MCP implementation | `cmd/solvent-mcp/tools.go`, `cmd/solvent-mcp/errors.go`, `cmd/solvent-mcp/main.go` | Tool schemas, auth mapping, SQLSTATE preservation, fail-closed |
| D | Existing HTTP | `internal/wizard/http.go`, `demo/cloud/web/handlers.go`, `demo/cloud/web/main.go` | Wizard API patterns, demo web handlers, what's reusable vs legacy |
| E | Schema + SQL | `db/005_authority_mvp.sql`, `db/007_service_tables.sql`, `db/001_schema.sql`, `kernel/sql.go` | Authority schema, audit tables, all SQL statements |
| F | Service audit/policy/evidence | `service/audit/*.go`, `service/policy/*.go`, `service/evidence/*.go` | Activity model, policy constraints, evidence quality projections |
| G | Reference analyses | `docs/OS/analysis_dealforge.md`, `docs/OS/analysis_aegisflow.md` | API/resource modeling, request/response, naming, and integration ergonomics only. Do NOT import DealForge/AegisFlow approval, workflow, token, authorization, or authority semantics. Explicitly reject any reference pattern that conflicts with: evidence != authority, workflow != authority, authorize != execute, current authority wins, kernel is the final authority oracle |
| H | Security tests | `kernel/authority_test.go`, `kernel/authority_dogfood_test.go`, `cmd/solvent-mcp/tools_authority_test.go`, `cmd/solvent-mcp/tools_agentjacking_test.go` | Security test coverage, failure modes, edge cases |

Each agent returns a structured report. **No document authoring begins until all 8 reports are collected.**

The parallel agents are read-only reconnaissance agents.

They must not:
- modify source files
- modify documentation
- modify schemas
- run migrations
- generate implementation code
- create competing API designs independently

They return observations and evidence only.

---

## Phase 2: Baseline Verification

For each material security claim in `prompt12.md` §2, verify against actual code:

| # | Claim | Verification Target | Method |
|---|-------|---------------------|--------|
| 1 | `solvent_authorize_action` requires actor/target | `cmd/solvent-mcp/tools.go` | Read handler, confirm required fields, confirm fail-closed on missing |
| 2 | Missing required auth inputs fail closed | MCP error mapping + tools.go | Confirm no conditional skip of authorization |
| 3 | Malformed identifiers fail safely | MCP tests + kernel errors | Confirm `ErrInvalidProposal`, FK errors, SQLSTATE surfacing |
| 4 | No production consequential executor | `service/executor/` | Confirm instantiated but empty registry |
| 5 | `kernel.Authorize` is current-state exact tuple | `kernel/authority.go` | Read authorize SQL, confirm re-read + field-by-field match |
| 6 | `kernel.Approve` creates authority | `kernel/authority.go` | Confirm snapshot + activation creation |
| 7 | `kernel.RevokeTarget` is append-only | `kernel/authority.go` | Confirm no UPDATE/DELETE on revocation |
| 8 | Workflow removed/dead | `service/` directory, `go.mod` | Confirm no workflow package, no workflow token reads |
| 9 | Stale token paths absent | grep for `GetToken`, `workflow_token` reads | Confirm no production code reads the table |
| 10 | No Store-based auth bypass | kernel boundary, I-7 guard script | Confirm all writes through `crdb.ExecuteTx` |
| 11 | No stale workflow/token authority semantics | Search for `GetToken`, `workflow_token`, approval tokens, cached authorization, client-carried authority/approval state, or equivalent semantic paths | Inspect callers and data flow; do not rely only on symbol/table-name searches |

**Output:** For each claim: PASS, FAIL (with `BASELINE DRIFT` + actual behavior + impact assessment), or UNVERIFIABLE.

If security-relevant drift materially affects design → document ends with `NOT READY FOR ADVERSARIAL REVIEW`.

---

## Phase 3: Document Authoring

Write `docs/OS/phase4a_api_contract.md` with these 28 sections:

| § | Section | Content | Primary Source |
|---|---------|---------|----------------|
| 1 | Executive Decision | Scope, intent, constraints, what Phase 4A is and isn't | prompt12.md §3 |
| 2 | Repository Reconnaissance | What exists, what's deferred, codebase inventory | Phase 1 reports |
| 3 | Current Domain Inventory | Every entity: resource, purpose, authoritative source, owner, read/write path, security sensitivity, current API exposure, recommended API exposure | kernel + schema + service |
| 4 | Proposed Public Resource Model | Smallest coherent public model; for each: meaning, lifecycle, identifier, immutable/mutable fields, relationships, auth requirements, audit requirements | Derived from §3 |
| 5 | Canonical Operations | Every operation: HTTP method, endpoint, request/response schema, auth requirements, actor type, idempotency, audit, kernel interaction, failure modes, retry semantics | Derived from kernel contract + MCP tools |
| 6 | Authentication / Actor Model | HUMAN/AGENT/SYSTEM, auth abstraction (API keys, OAuth/OIDC, service identity), what is caller assertion vs server-established | Architecture docs + MCP |
| 7 | Authorization Semantics | 5-layer distinction: Authentication ≠ API Access Control ≠ Solvent Authority ≠ Policy ≠ Execution Authorization | kernel.Authorize + service/authority |
| 8 | Forbidden API Surface | Negative space: every dangerous endpoint pattern, why each is forbidden | All attack vectors |
| 9 | Request/Response Conventions | IDs, timestamps, versions, enums, nullable, pagination, filtering, sorting, field naming, metadata, provenance, correlation IDs, idempotency keys | New design (language-neutral JSON) |
| 10 | Error Contract | Canonical error taxonomy: malformed, auth failure, access denied, invalid actor/target, stale state, revoked/missing authority, tuple mismatch, policy denial, conflict, invariant violation, external failure, ISE | kernel/errors.go + SQLSTATE |
| 11 | Idempotency / Concurrency | Duplicate requests, retries, concurrent approval, DB atomicity vs external side effects, TOCTOU limitation | DB limits |
| 12 | Versioning | `/v1` prefix strategy, additive evolution, breaking changes, deprecation | Simplest durable approach |
| 13 | MCP Mapping | For each operation: REST → MCP representation → Service → Kernel | cmd/solvent-mcp/ |
| 14 | A2A Mapping | For each operation: REST → A2A representation → Service → Kernel | Future sketch |
| 15 | Web UI Boundary | UI as client, not authority; which UI needs belong in API vs read model vs client-only | Architecture principle |
| 16 | SDK Strategy | OpenAPI first, thin SDKs later as justified; no SDK as architectural dependency | Background phase4 |
| 17 | OpenAPI Structure | Tags, resources, operations, schemas, errors, security, pagination, examples | New design |
| 18 | Audit Contract | Activity event shape: request received, auth result, decision, authority creation/revocation, adapter invocation, provider result, failure | service/audit/ |
| 19 | Future Execution Boundary | ExecutionService sketch: API → revalidation → kernel.Authorize → Executor → provider; prevent bypass paths | service/authority + executor |
| 20 | Database Impact | Which API resources served from existing data; no authority-core schema growth; for any proposed migration: why, authority-core or read-model, derivable, deferrable | All existing tables |
| 21 | Integration Model | GitHub/CI/CD as integration adapter; provider-specific semantics stay outside API | adapter/github/ |
| 22 | Reference Workflows | 9 canonical flows: evidence submission, human approval, auth check, rejected auth, revoked authority, wrong target, agent untrusted claim, future execution, audit lookup | kernel + service |
| 23 | Threat / Adversarial Analysis | 25+ attack vectors: confused deputy, privilege escalation, caller-controlled authority, stale auth, replay, duplicate approval, wrong target/actor, forged approval/evidence, agent claims as authority, cached auth, UI-controlled auth, MCP/A2A bypass, arbitrary executor, schema ambiguity, type confusion, missing auth boundary | §8 + §6 combined |
| 24 | Test Matrix | Contract, auth, actor, negative, idempotency, concurrency, stale-state, revocation, wrong-target, wrong-actor, MCP parity, A2A parity, audit, error-contract, serialization + regression tests | Existing tests + gaps |
| 25 | Implementation Plan | 12-step staged plan with files/packages, new interfaces, structs, migrations, tests, security implications, rollback strategy | Existing structure |
| 26 | Deferred Decisions | Open questions requiring decisions before or during 4B | Identified during design |
| 27 | Acceptance Criteria | Checklist: KERNEL, AUTHORITY, API, MCP, A2A, UI, EXECUTION, AGENT SAFETY, IDENTITY, REPLAY, TOCTOU, SCHEMA, COMPLEXITY, REFERENCE CONTAMINATION, CURRENT-vs-FUTURE | prompt12.md §28 |
| 28 | Final Architecture Rules | Hard constraints carried forward | Background + AGENTS.md |

---

## Phase 4: Internal Adversarial Pre-Review

This is a design-author self-review only. It does NOT substitute for the independent adversarial review that follows Phase 4A.

Before finalizing, run adversarial self-check from `prompt12.md` §23:

| Attack Vector | Analysis Per Attack |
|---------------|---------------------|
| Confused deputy | Can a service role be tricked into acting outside its scope? |
| Privilege escalation | Can a low-privilege operation create high-privilege state? |
| Caller-controlled authority | Can the caller set `approved=true` or `authority_state`? |
| Stale authorization | Can a retry use cached/outdated authority? |
| Replay | Can a valid request be replayed to produce duplicate authority? |
| Duplicate approval | Can `Approve` be called twice on the same target? |
| Wrong-target execution | Can authority for target A be used on target B? |
| Wrong-actor execution | Can principal X use authority granted to principal Y? |
| Forged approval | Can a client construct a valid `target_snapshot`? |
| Forged evidence | Can fabricated evidence reach `kernel.Promote`? |
| Agent claims as authority | Can `actor_type=AGENT` or `action_source=tool_output` become authority? |
| Cached authorization | Can a stale `AuthorizeResult` be reused? |
| UI-controlled auth | Can the web UI set authority state independently? |
| MCP bypass | Can MCP create an alternate authorization path? |
| A2A bypass | Can A2A create an alternate authorization path? |
| REST bypass | Can direct REST calls bypass service-level checks? |
| Arbitrary executor selection | Can the caller choose which executor runs? |
| Hidden provider side effect | Can a mutation produce unannounced external effects? |
| Auth/execution confusion | Can `Authorize` be mistaken for `Execute`? |
| Schema ambiguity | Can nullable/optional fields create implicit authority? |
| Type confusion | Can string/number/enum mismatch bypass checks? |
| Insecure default | Do defaults fail-open? |
| Missing auth boundary | Is there an unauthenticated path to mutations? |
| Accidental admin bypass | Can admin operations be reached without admin auth? |

For each: **Attack → Current defense → Remaining gap → Phase 4A scope → Kernel change required → Deferred?**

---

## Phase 5: Final Output

1. Run 15-item checklist from `prompt12.md` §28 (including REFERENCE CONTAMINATION and CURRENT-vs-FUTURE)
2. Append `READY FOR ADVERSARIAL REVIEW` or `NOT READY FOR ADVERSARIAL REVIEW`
3. Write to `docs/OS/phase4a_api_contract.md`

### Final Readiness Gate (§28)

```
[KERNEL]
Does Phase 4A require any kernel change?
If yes, why is it unavoidable?

[AUTHORITY]
Is there exactly one source of truth for authority?

[API]
Can every consequential API operation explain exactly how current authority is checked?

[MCP]
Can MCP do anything REST cannot?

[A2A]
Can A2A do anything REST cannot?

[UI]
Can removing the UI leave the security model unchanged?

[EXECUTION]
Can a future executor be inserted without bypassing kernel.Authorize?

[AGENT SAFETY]
Can untrusted agent/tool output become authority?

[IDENTITY]
Can a caller simply claim to be HUMAN or an authorized principal?

[REPLAY]
Are retries and duplicate mutations safe?

[TOCTOU]
Are DB/external-side-effect limits honestly documented?

[SCHEMA]
Did Phase 4A unnecessarily grow the authority-core schema?

[COMPLEXITY]
Is every proposed API concept justified?

[REFERENCE CONTAMINATION]
Did any proposed resource, operation, token, workflow, approval model,
or authority abstraction originate from DealForge/AegisFlow?

If yes, demonstrate compatibility with Solvent's governing principles.
If compatibility cannot be demonstrated, remove it.

[CURRENT-vs-FUTURE]
Does the proposed API accidentally expose capabilities that do not currently exist,
particularly consequential execution?

If yes, mark them explicitly FUTURE and ensure no Phase 4B implementation path
can interpret the design as permission to create an unsafe bypass.
```

---

## Execution Order

```
Phase 1 (8 parallel explore agents)
        ↓
Phase 2 (baseline verification — sequential, thorough)
        ↓
Phase 3 (document authoring — sequential sections)
        ↓
Phase 4 (internal adversarial pre-review)
        ↓
Phase 5 (final checklist + output)
```

---

## Constraints

- **DO NOT** modify any source code
- **DO NOT** modify any schema/migration
- **DO NOT** modify the kernel
- **DO NOT** fetch external repositories
- **DO NOT** silently fix discrepancies during reconnaissance
- **DO NOT** import DealForge/AegisFlow conceptual authority, workflow, approval, or token semantics into the Solvent API contract
- **DO** record `BASELINE DRIFT` for any contradicted claim
- **DO** conclude `NOT READY FOR ADVERSARIAL REVIEW` if security-relevant drift is found
- **DO** keep the document complete — no collapsed/executive-summary sections

---

## Post-Phase 4A Sequence (for reference)

```
Phase 4A (this plan)  →  Adversarial Review
                              ↓
                        Phase 4B (Service/API Implementation)
                              ↓
                        Phase 4C (OpenAPI + Reference Integrations)
                              ↓
                        Phase 4D (Optional Web UI)
                              ↓
                        Phase 4E (API/UI Adversarial Review)
```
