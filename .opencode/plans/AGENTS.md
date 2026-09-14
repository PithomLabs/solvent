# AGENTS.md

Think like a distributed systems engineer, not an LLM engineer.

## 1. PROJECT

**Solvent** is a small, durable authority layer between autonomous systems and consequential actions.

It is not merely an agent memory system. It is a checkpoint that determines whether an autonomous system may perform a consequential action based on evidence, belief, policy, and exact authority binding. The database — not the caller — decides whether an action is allowed.

The five core concepts are distinct:

- **Belief**: a claim with evidence, debt, and lifecycle status
- **Evidence**: attributable provenance supporting or contradicting a belief
- **Authority**: a durable, approved relationship binding a principal to a specific consequence
- **Intent**: a recorded plan to act on a belief under specific authority
- **Execution**: the actual invocation of a real-world consequence through an executor

---

## 2. CORE THESIS

**Retrieval is not authority.**

```
Evidence / Retrieval
        ↓
      Belief
        ↓
    Authority
        ↓
      Intent
        ↓
    Execution
```

Retrieval can be wrong. Judgment can be wrong. Authority must be structurally constrained.

Additional principles:

- **Authorization is distinct from execution.** Authorization permits an intended consequence. It does not prove that the external consequence occurred.
- **Agent claims are not authority.** An agent asserting something is true does not make it true.
- **Intent is not authority.** Recording intent to act does not grant permission to act.
- **Evidence is not authority.** Evidence supports beliefs; beliefs support authority; authority enables intent; execution consumes authority.

The architecture enforces that agents do not decide truth. They observe evidence, propose beliefs, and propose actions. The ledger determines whether those actions remain valid.

---

## 3. KERNEL FREEZE

The kernel is frozen. Do not redesign.

> **New capabilities default to service, adapter, executor, deployment, policy, demo, or documentation layers.**

> **Kernel changes require a genuinely new durable security fact or atomic security transition that cannot safely be expressed outside the existing kernel.**

> **Grow the ecosystem, not the kernel.**

The kernel is frozen because the current authority model is sufficient; future domain semantics, provider integrations, governance policy, and execution capabilities must normally be implemented outside the kernel.

Future contributors must NOT add kernel primitives simply because a feature is "security related." The post-freeze authorization work required no kernel primitives, invariant, schema, migration, signature, or kernel SQL changes.

If a new capability can be implemented in the service, adapter, executor, policy, or deployment layers, it MUST be implemented there.

---

## 4. ARCHITECTURE / RESPONSIBILITY BOUNDARIES

The extension decision tree:

| Need | Layer |
|------|-------|
| External product/protocol integration | Adapter |
| Policy/orchestration/composition | Service / Policy |
| Execution/infrastructure | Executor / Deployment |
| Customer-specific behavior | Policy / Configuration / Data |
| Reporting/UI/analytics | Product / Read Model / Service |
| Impossible state | DB invariant |
| New atomic security primitive | Kernel only if unavoidable |

Provider-specific and domain-specific semantics should not be moved into the generic kernel.

The current repository structure:

- `kernel/` — frozen authority and belief primitives
- `service/` — authority, ledger, policy, audit, executor services
- `adapter/` — external system integrations (e.g., GitHub)
- `api/` — REST API with authentication
- `cmd/solvent-mcp/` — MCP stdio server (trusted local boundary)
- `db/` — schema migrations; frozen kernel schema and later service/product schema are kept distinct

---

## 5. AUTHORITY MODEL

The durable authority relationship:

```
principal
   ↓
authority target
   ↓
approved snapshot
   ↓
target activation
   ↓
action intent
   ↓
authorization
   ↓
claim
   ↓
execution
```

Justification and evidence support the approval decision rather than being a mandatory sequential lifecycle step. The key operations:

- **CreatePrincipal** — establish actor identity
- **CreateTarget** — propose an authority relationship (no authority granted)
- **AttachJustification** — link beliefs supporting the target
- **RequestAuthorization** — atomically pin the proposal and justification set
- **Approve** — create immutable snapshot and activation (sole authority-creating operation)
- **Authorize** — read-only verification of current authority against a presented tuple
- **AuthorizeAndCreateIntent** — atomic authority evaluation + intent creation in one SERIALIZABLE transaction
- **ClaimIntent** — atomic CAS live→executing (sole ownership gate for execution)
- **CompleteIntent / RollbackClaim / CancelIntent** — execution outcome recording
- **RevokeTarget** — insert immutable revocation fact (no unrevoke in v0)

The confused-deputy protection ensures an approval for one target/state cannot be reused for another. This is enforced by exact authority binding (section 7).

---

## 6. BELIEFS / EVIDENCE / DEBT

**Beliefs** are claims that carry evidence, evolve as new evidence arrives, may be promoted or retracted, and govern whether an agent may act.

**Evidence** is attributable. Every discharge names what justified it. A receipt that cannot be traced to a specific row is decoration.

**Debt** is an explicit unresolved obligation. Debt vocabulary is opaque and domain-specific. The kernel does not validate debt item names. Current deployments may define convenience vocabularies such as `belief.WizardDebt()`, but those are application/deployment concerns.

Promotion is structurally gated by debt state: the schema's `promoted_is_debt_free` CHECK prevents promotion while debt remains.

Agent reasoning does not itself create authority. Authority requires explicit approval through the authority lifecycle.

### Retrieval and Evidence Integrity

Evidence entering Solvent must remain truthful and attributable:

- **Never fabricate retrieval results.** No synthetic embeddings, no fallback path that invents a result when a real one is unavailable.
- **Preserve measured retrieval values.** The measured distance is the number, including when it is unflattering. Do not recompute it at render time.
- **Preserve attribution and provenance.** A receipt that drifts from the row it cites is worse than no receipt.
- **Distinguish evidence relationships.** A `considered` citation is measured against the query that surfaced it. A `contradicts` citation is measured against the belief's own claim. They are not interchangeable.
- **Do not introduce arbitrary relevance thresholds.** No cutoffs, no silent reranking.

---

## 7. EXACT AUTHORITY BINDING

An action intent is tied to the **exact authority instance** represented by the composite `(target_id, snapshot_id)` pair.

```
action_intent.(target_id, snapshot_id)
    → target_snapshot(target_id, snapshot_id)
```

This exists because:

- It prevents confused deputy behavior: an approval for target A cannot be used for target B
- It prevents an approval for one target/state from being reused for another
- Database constraints reinforce the relationship: composite FK + unique index on `(target_id, snapshot_id) WHERE state = 'live'`
- `ClaimIntent` must preserve exact identity: the CAS predicate matches on `(belief_id, action, target_id, snapshot_id)`

The `IntentOnPromoted` path (pre-approval) creates intents with NULL `target_id` and `snapshot_id`. Only `AuthorizeAndCreateIntent` sets these fields.

---

## 8. AUTHORIZE != EXECUTE

The real execution path:

```
PrepareForAction (re-reads current state)
  → kernel.Authorize (read-only authority evaluation)
  → resolve executor from internal registry
  → reconstruct execution params from snapshot (BEFORE claim)
  → ClaimIntent (atomic CAS live→executing)
  → execute provider
  → CompleteIntent or RollbackClaim based on outcome
```

Core rules:

- **Authorization permits an intended consequence.**
- **It does not prove that the external consequence occurred.**
- **Provider acceptance is not workflow completion.**
- The executor is resolved from the internal registry, NOT by the caller.
- Snapshot `consequence_parameters` are read from the database, not from the caller.
- Earlier authorization results are never reused.
- The kernel is the authoritative authority evaluator for Solvent. Do not introduce a second competing authority engine.

---

## 9. EXECUTOR / ADAPTER RULES

**Adapters** translate external systems into Solvent concepts. The current adapter is `adapter/github`, which implements the `GitHubProvider` interface for triggering GitHub Actions workflows.

**Executors** perform already-authorized consequences. The current executor is `github_trigger_workflow`, registered when `GITHUB_TOKEN` is configured.

The boundary:

- Adapters translate external systems into Solvent concepts
- Executors perform already-authorized consequences
- Executors do NOT approve, promote, revoke, or create authority
- Provider-specific errors/outcomes remain provider concerns
- Caller input must NOT freely select arbitrary executors — the executor is resolved internally from the registry by action name

Provider outcome handling:

- `ProviderAccepted` (code 0) — provider accepted, treat as success
- `ProviderRejected` (code 1) — definitive rejection, rollback claim, allow retry
- `ProviderAmbiguous` (code 2) — unknown outcome, leave intent as executing, no retry

---

## 10. ACTOR / AUTHENTICATION / IDENTITY

The distinction:

- **Actor type** (HUMAN / AGENT / SYSTEM / workload / service) — category of the principal
- **Identity** (principal_id) — the specific actor
- **Authentication** — happens at the deployment/API boundary
- **Authorization** — kernel determines whether the identified actor may perform the action

The API derives principal identity from the authenticated Bearer token, mapped via static configuration (`keyToPrincipal`). Do not trust caller-supplied `actor_id` in request bodies — if it conflicts with the authenticated principal, the request is rejected with 403 `actor_id_mismatch`.

MCP uses trusted local stdio with no strong authentication. The `actor_id` in MCP tool arguments is an attribution input, not authentication proof.

---

## 11. TRUST BOUNDARIES

The major trust vectors:

1. **Direct kernel calls** — internal service paths
2. **REST API** — authenticated via Bearer tokens mapped to principals
3. **MCP stdio** — trusted local administrative surface
4. **Direct DB access** — bypasses all policy and kernel logic
5. **Deployment/configuration exposure** — DSN, tokens, environment variables

A policy is only meaningful if the relevant consequential mutation paths cannot bypass it. Application authorization is not equivalent to DB credential isolation.

The MCP server is a trusted local process. It runs on stdio only. Network transports are not supported; selecting one fails closed to prevent accidental trust-model changes.

---

## 12. MCP

MCP currently operates as a trusted local stdio deployment boundary. It is not equivalent to a remotely authenticated API. Do not treat caller-supplied identity fields as strong authentication. Remote exposure requires an explicit authentication/deployment boundary.

The MCP server exposes these tools (verify against repository at implementation time):

**Read tools:**
- `solvent_ledger` — read current ledger state for a scenario
- `solvent_explain` — explain promotion/authorization readiness (read-only)
- `solvent_activity` — read audit activity entries

**Mutation tools:**
- `solvent_retire_debt` — retire a debt item from a belief
- `solvent_promote` — promote a belief (database refuses if debt remains)
- `solvent_falsify` — retract a belief and cancel dependent live intent
- `solvent_discharge` — record debt discharge with attribution
- `solvent_ingest_evidence` — process evidence fixtures through the pipeline

**Authority lifecycle tools:**
- `solvent_create_principal` — create a new principal
- `solvent_revoke_principal` — revoke a principal
- `solvent_create_target` — create an authority target proposal
- `solvent_attach_justification` — attach a belief as justification
- `solvent_request_authorization` — pin proposal and justification set
- `solvent_approve` — approve a target (sole authority-creating operation)
- `solvent_authorize` — read-only authority verification
- `solvent_revoke_target` — revoke an authority target

**Execution tools:**
- `solvent_authorize_action` — atomic authority + intent creation
- `solvent_execute` — claim intent, invoke executor, record outcome

---

## 13. AUDIT

Audit events correspond to actual events. The audit distinguishes:

- **Authorization decisions** — `authorization_granted`, `authorization_denied`
- **Adapter invocations** — `adapter_invoked`
- **Executor outcomes** — `executor_completed`, `executor_failed`
- **Reconciliation** — `reconciliation_completed`, `reconciliation_failed`

Rejected authorization attempts must not be represented as successful actions. The audit log is append-only and preserves the distinction between:

- Authorization succeeded + execution failed
- Authorization denied + execution never occurred

Audit is observability/evidence of behavior, not a substitute for authority enforcement.

---

## 14. RACE / CONCURRENCY RULES

Verified race conditions from the repository:

- **Claim before external side effect** — `ClaimIntent` (atomic CAS live→executing) occurs before the provider invocation, ensuring at most one concurrent claim succeeds per intent
- **Concurrent revocation** — `RevokeTarget` and `AuthorizeAndCreateIntent` serialize via `SELECT ... FOR UPDATE` on `authority_target`, preventing live intents on superseded authority
- **Provider-side TOCTOU** — provider acceptance does not mean workflow completion; the provider's external state may diverge
- **RetireDebt liveness pre-check** — best-effort and NOT transactionally atomic with the kernel transaction; revocation takes effect immediately for all new requests

---

## 15. DATABASE RULES

CockroachDB is an active enforcement layer, not passive storage.

- **DB-enforced invariants** — structural security facts that the schema can safely enforce (e.g., `promoted_is_debt_free`, `gate` composite FK, `live_requires_promoted`)
- **Kernel transactional logic** — atomic operations within `crdb.ExecuteTx` (e.g., `RetractCascade` cancel-before-retract ordering)
- **Service policy** — orchestration and composition decisions
- **External-provider behavior** — provider-specific outcomes

Recursive belief traversal is application logic, not a DB cascade. The walk over `belief_edge` is a `WITH RECURSIVE` CTE in Go. CockroachDB does not traverse the graph; it enforces that the traversal cannot finish having left a live intent behind.

Use DB invariants for structural security facts that the schema can safely enforce. The application cannot forget a `CHECK`.

---

## 16. SCENARIO ISOLATION

Every operation is scoped by a `scenario_id` the caller supplies. Scenarios are an isolation convention enforced by query predicate, not by a foreign key — there is no scenario table.

Cross-scenario operations must fail closed. The kernel derives or validates scenario IDs rather than trusting caller-supplied IDs where the current implementation validates them.

`RetractCascade` will not traverse out of its scenario (D-032).

---

## 17. DEVELOPMENT RULES

- Think like a distributed systems engineer.
- Preserve explicit invariants, receipts, deterministic behavior, transactional correctness, minimal architecture.
- Avoid hidden state, prompt-only guarantees, duplicated truth, speculative abstractions, unnecessary orchestration.
- Unknowns become receipts.
- Database correctness takes precedence over application convenience.
- If a design decision benefits the demo but weakens the ledger, reject it. The ledger is the product; the demo is the proof.
- Verify current repository behavior before documenting it.
- Prefer service/policy/adapters/executors over kernel growth.
- Do not reopen frozen architecture casually.
- Do not claim guarantees stronger than the actual transaction/deployment boundary.
- Do not hardcode test counts — they go stale silently.

When in doubt, ask:

> "Should this responsibility belong to the database instead?"

---

## 18. VERIFICATION

Repository verification commands:

- `task test` — full verification suite (tests, build, vet, I-7, isolation, MCP)
- `task db:reset` — drop and recreate schema
- `task mcp:verify` — MCP tool verification over stdio
- `scripts/check_i7.sh` — I-7 invariant check (no raw writes outside transactions)
- `go test -count=1 -p 1 ./...` — run all tests
- `go vet ./...` — static analysis
- `gofmt -l cmd internal kernel` — formatting check

Do not hardcode test counts. Use `task test` for current suite status.

---

## 19. REMAINING ACCEPTED BOUNDARIES

Verified risks consciously accepted in the current repository:

- **RetireDebt principal liveness pre-check** — best-effort rather than atomic with the kernel transaction; the kernel owns its internal `crdb.ExecuteTx`, so this check cannot be made atomic with downstream kernel mutations
- **Local MCP trusted boundary** — MCP is a trusted local deployment boundary, not a fully authenticated remote service; caller-supplied identity fields are attribution inputs, not authentication proof
- **Provider acceptance ≠ workflow completion** — provider acceptance does not prove the external workflow completed successfully
- **v0 idempotency limitations** — `EnterBelief`, `CreateTarget`, `IntentOnPromoted` are not idempotent under client-timeout retries; fixing requires schema changes to the frozen tables
- **Justification revival** — v0 has no promotion-epoch identity; retract then re-promote can revive a live justification because v0 does not claim identity of the original promotion occurrence
