Use this as the coding-agent prompt:

```text
You are updating the repository's AGENTS.md so it accurately reflects the CURRENT Solvent architecture after the kernel freeze.

This is a DOCUMENTATION SYNCHRONIZATION TASK ONLY.

Do NOT redesign the system.
Do NOT modify kernel behavior.
Do NOT modify schema or migrations.
Do NOT modify APIs, services, executors, adapters, MCP, tests, or scripts merely to make the documentation fit.
Do NOT reopen the kernel freeze.

The repository itself is the source of truth for current implementation details.

==================================================
OBJECTIVE
==================================================

Update the root AGENTS.md so that it becomes the canonical operating guide for future coding agents working on the frozen Solvent architecture.

The current AGENTS.md is materially stale. It still describes an earlier v0 state in which:

- execution was future-only,
- the executor registry was empty,
- the architecture was primarily framed as memory → belief → authority,
- the data model was described as only a small set of core tables,
- exact target/snapshot authority binding was not documented,
- service-layer authorization boundaries were not documented,
- the kernel-freeze growth rule was not expressed in its final form,
- AUTHORIZE != EXECUTE was not explicit,
- authentication / identity / actor type / authorization distinctions were incomplete,
- the adapter/service/executor/deployment extension model was incomplete.

Your job is to reconcile the document with the repository as it exists NOW.

==================================================
PHASE 1 — REPOSITORY AUDIT
==================================================

Before editing AGENTS.md, inspect the repository thoroughly enough to establish the actual current architecture.

At minimum inspect:

1. Repository structure
   - root directories
   - cmd/
   - api/
   - service/
   - kernel/
   - adapter/
   - internal/
   - demos/
   - scripts/
   - migrations/schema
   - Taskfile/configuration

2. Kernel
   - public methods
   - authority lifecycle
   - belief lifecycle
   - debt lifecycle
   - target/snapshot/activation/revocation model
   - action intent lifecycle
   - authorization
   - ClaimIntent / CompleteIntent / RollbackClaim / CancelIntent
   - scenario isolation
   - transaction boundaries
   - exact authority binding
   - database invariants

3. Database/schema
   - actual tables
   - constraints
   - foreign keys
   - unique indexes
   - CHECK constraints
   - state transitions
   - any exact `(target_id, snapshot_id)` relationships
   - confirm what is actually kernel data versus service/product/audit data

4. Service layer
   - authority execution path
   - policy checks
   - RetireDebt
   - Discharge
   - ExecuteAction
   - scenario guards
   - authentication assumptions
   - actor/principal handling

5. Executors/adapters
   - GitHub executor/provider
   - executor selection
   - provider outcome handling
   - what the executor is and is NOT allowed to do
   - any other real executor/provider integrations

6. API
   - REST execution endpoint
   - authorization endpoint(s)
   - authentication boundary
   - caller-controlled identity fields
   - error semantics

7. MCP
   - tools exposed
   - authentication/deployment boundary
   - local trusted stdio assumption
   - any distinction between local trusted MCP and remotely exposed MCP

8. Tests
   - exact authority/confused-deputy tests
   - scenario-isolation tests
   - concurrency/race tests
   - service authorization tests
   - executor tests
   - audit tests
   - MCP verification
   - I-7/raw-write checks

9. Documentation/scripts
   - existing architecture docs
   - freeze rules
   - Taskfile
   - verification scripts
   - any documentation that contradicts AGENTS.md

Do not infer implementation from the old AGENTS.md when repository evidence is available.

==================================================
PHASE 2 — PRODUCE AN INTERNAL DELTA CHECK
==================================================

Before editing, identify:

A. Claims in AGENTS.md that are obsolete.
B. Claims that remain correct.
C. New architectural facts that must be added.
D. Statements that cannot be verified from the repository and therefore should NOT be asserted as facts.
E. Any discrepancy serious enough to indicate a possible code regression.

Do not fix code to resolve documentation discrepancies.

If you discover a genuine implementation inconsistency that cannot safely be treated as documentation drift, STOP before making architectural changes and report it.

==================================================
PHASE 3 — REWRITE AGENTS.md
==================================================

Rewrite AGENTS.md into a concise but technically authoritative guide for coding agents.

Keep the strongest useful material from the current file, but remove obsolete claims.

The document should be organized approximately around these concepts. You may improve the organization if the repository suggests a better structure.

--------------------------------------------------
1. PROJECT
--------------------------------------------------

Describe Solvent as the current product, not merely as a "transactional belief ledger."

The important framing is:

Solvent is a small, durable authority layer/checkpoint between autonomous systems and consequential actions.

Preserve the distinction between:
- belief
- evidence
- intent
- authority
- execution

Make clear that Solvent is NOT merely an agent memory system.

--------------------------------------------------
2. CORE THESIS
--------------------------------------------------

Make the following central:

> Retrieval is not authority.

Also establish:

> Evidence is not authority.
> Agent claims are not authority.
> Capability is not authority.
> Intent is not authority.
> Authorization is distinct from execution.

Use the strongest wording supported by the implementation.

--------------------------------------------------
3. KERNEL FREEZE
--------------------------------------------------

This must be prominent.

Document the final rule:

> New capabilities default to service, adapter, executor, deployment, policy, demo, or documentation layers.

And:

> Kernel changes require a genuinely new durable security fact or atomic security transition that cannot safely be expressed outside the existing kernel.

Make explicit:

> Grow the ecosystem, not the kernel.

Explain that future contributors must NOT add kernel primitives simply because a feature is "security related."

--------------------------------------------------
4. ARCHITECTURE / RESPONSIBILITY BOUNDARIES
--------------------------------------------------

Document the current extension decision tree:

External product/protocol
    → Adapter

Policy/orchestration/composition
    → Service / Policy

Execution/infrastructure
    → Executor / Deployment

Customer-specific behavior
    → Policy / Configuration / Data

Reporting/UI/analytics
    → Product / Read Model / Service

Impossible state
    → DB invariant

New atomic security primitive
    → Kernel only if unavoidable

Everything else
    → Don't add it

Explain that provider-specific and domain-specific semantics should not be moved into the generic kernel.

--------------------------------------------------
5. AUTHORITY MODEL
--------------------------------------------------

Document the actual lifecycle supported by the repository.

Include the concept that authority must bind to the exact consequence, including the exact target/snapshot identity.

Document the confused-deputy protection.

Do NOT overstate anything that the repository does not actually enforce.

--------------------------------------------------
6. BELIEFS / EVIDENCE / DEBT
--------------------------------------------------

Retain the useful belief-ledger concepts from the existing AGENTS.md.

Document:

- evidence is attributable
- debt is an explicit unresolved obligation
- debt vocabulary is opaque/domain-specific
- kernel should not hardcode deployment-specific debt names
- promotion is structurally gated by debt state
- agent reasoning does not itself create authority

Do not reintroduce deployment-specific vocabulary into kernel guidance.

--------------------------------------------------
7. EXACT AUTHORITY BINDING
--------------------------------------------------

This is a critical addition.

Document that an action intent is tied to the exact authority instance represented by target + snapshot.

Explain why this exists:

- prevents confused deputy behavior
- prevents an approval for one target/state from being reused for another
- database constraints reinforce the relationship
- ClaimIntent must preserve exact identity

Do not invent constraint names unless verified from the repository.

--------------------------------------------------
8. AUTHORIZE != EXECUTE
--------------------------------------------------

Document the current real execution path.

Use the repository's actual flow, which should conceptually look like:

Prepare
  → current state / intent handling
  → Authorize
  → Claim intent
  → fixed executor/provider
  → provider outcome
  → Complete / Rollback / Reconciliation as applicable

Verify exact ordering against the code before documenting it.

Explicitly state:

Authorization does not prove that the external side effect happened.

The executor consumes authorization.
The executor must not mint authority.

--------------------------------------------------
9. EXECUTOR / ADAPTER RULES
--------------------------------------------------

Document the actual GitHub executor integration if still present.

Make the boundary explicit:

- adapters translate external systems into Solvent concepts
- executors perform already-authorized consequences
- executors do not approve, promote, revoke, or create authority
- provider-specific errors/outcomes remain provider concerns
- caller input must not freely select arbitrary executors unless the repository actually supports that

Verify all of this before writing it.

--------------------------------------------------
10. ACTOR / AUTHENTICATION / IDENTITY
--------------------------------------------------

Document the distinction:

Actor type != identity != authentication != authorization.

Examples:

HUMAN / AGENT / SYSTEM are actor categories.

Authentication happens at the deployment/API boundary.

Do not trust caller-supplied actor identity in request bodies.

Where the repository derives principal identity from authenticated request context, document that.

--------------------------------------------------
11. TRUST BOUNDARIES
--------------------------------------------------

Document the major trust vectors:

1. direct kernel calls
2. alternate authority-mutating APIs
3. lower-level service paths bypassing policy
4. direct DB access
5. deployment/configuration exposure

Explain that a policy is only meaningful if the relevant consequential mutation paths cannot bypass it.

Do not claim application authorization is equivalent to DB credential isolation.

--------------------------------------------------
12. MCP
--------------------------------------------------

Document the actual current MCP boundary.

Distinguish:

local trusted stdio deployment
from
remote/public MCP exposure.

Do not imply that local MCP caller fields constitute strong authentication unless the repository actually provides it.

Make the deployment boundary explicit.

--------------------------------------------------
13. AUDIT
--------------------------------------------------

Document:

- audit events should correspond to actual events
- rejected authorization attempts must not be represented as successful actions
- distinguish authorization, provider invocation, provider outcome, and execution result
- audit is observability/evidence of behavior, not a substitute for authority enforcement

Do not overstate atomicity if audit is outside the kernel transaction.

--------------------------------------------------
14. RACE / CONCURRENCY RULES
--------------------------------------------------

Document only races verified in code/tests.

Important concepts likely include:

- claim before external side effect
- concurrent intent handling
- concurrent revocation
- provider-side TOCTOU limitations
- service-layer RetireDebt liveness pre-check is best-effort and NOT transactionally atomic

Use precise language.

Do not hide accepted residual risk.

--------------------------------------------------
15. DATABASE RULES
--------------------------------------------------

Retain the current strong position:

CockroachDB is an active enforcement layer, not passive storage.

Document real invariants only.

Do not state "where application code can enforce it, schema always wins" as an absolute if the current implementation contradicts that; instead express the actual architecture carefully:

Use DB invariants for structural security facts that the schema can safely enforce.

Preserve distinction between:
- DB-enforced invariant
- kernel transactional logic
- service policy
- external-provider behavior

--------------------------------------------------
16. SCENARIO ISOLATION
--------------------------------------------------

Document the actual scenario-binding rules verified by current tests.

Cross-scenario operations must fail closed.

Do not rely on caller-supplied IDs where the current service/kernel derives or validates them.

--------------------------------------------------
17. DEVELOPMENT RULES
--------------------------------------------------

Preserve the strongest existing principles:

- think like a distributed systems engineer
- explicit invariants
- deterministic behavior
- transactional correctness
- minimal architecture
- unknowns become receipts
- no invented metrics
- no prompt-only guarantees
- don't duplicate truth
- don't weaken the ledger for demo convenience

Add:

- verify current repository behavior before documenting it
- prefer service/policy/adapters/executors over kernel growth
- do not reopen frozen architecture casually
- do not claim guarantees stronger than the actual transaction/deployment boundary

--------------------------------------------------
18. VERIFICATION
--------------------------------------------------

Document the repository's real verification commands.

Prefer commands derived from the repo rather than invented ones.

Do NOT hardcode stale package counts.

Use the repository's current:
- test
- vet
- race
- build
- DB reset
- I-7
- isolation
- MCP verification
commands where applicable.

The existing rule to use `task test` for current suite status should remain if that target still exists.

--------------------------------------------------
19. REMAINING ACCEPTED BOUNDARIES
--------------------------------------------------

Only include risks verified from the current repository.

Likely examples, ONLY if still true:

- RetireDebt principal liveness pre-check is best effort rather than atomic with the kernel transaction.
- local MCP is a trusted deployment boundary rather than a fully authenticated remote service.

Do not convert LOW/INFO observations into fake guarantees.

==================================================
IMPORTANT DOCUMENTATION RULES
==================================================

1. Repository truth wins over the old AGENTS.md.

2. Do not preserve obsolete statements simply because they sound architectural.

3. Do not add speculative future architecture as if it already exists.

4. Clearly distinguish:
   - current implementation
   - architectural principle
   - future extension point

5. Do not hardcode test counts.

6. Do not invent constraint names, table counts, API endpoints, executor behavior, or security guarantees.

7. Preserve exact terminology already established in the repository where possible.

8. Do not describe the database as doing work it does not actually do.
   In particular, do not describe recursive belief traversal as a DB cascade unless the repository really implements it that way.

9. Do not weaken or remove useful retrieval-integrity rules from the existing AGENTS.md unless the repository proves they are obsolete.

10. Do not mention this prompt or the coding-agent process inside AGENTS.md.

==================================================
PHASE 4 — VERIFY THE UPDATED DOCUMENT
==================================================

After editing AGENTS.md:

1. Re-read the entire AGENTS.md.
2. Cross-check every implementation-specific claim against the repository.
3. Ensure no obsolete "future-only execution" language remains.
4. Ensure no claim says executor support is empty if a real executor exists.
5. Ensure exact authority binding is documented.
6. Ensure the kernel-freeze rule is prominent and unambiguous.
7. Ensure service/policy responsibilities are not incorrectly pushed into the kernel.
8. Ensure trusted-local MCP boundary is documented accurately.
9. Ensure no stale architecture diagrams remain.
10. Ensure formatting is clean.

Do NOT change production code.

==================================================
FINAL VERIFICATION / REPORT
==================================================

Run the minimum relevant repository verification needed to confirm that the documentation update did not alter code behavior.

At minimum:
- git diff -- AGENTS.md
- git status --short
- any repository-native documentation/format check if available

Do not run an enormous unrelated test matrix merely for the documentation task unless necessary.

Final response should contain:

1. A concise summary of what changed in AGENTS.md.
2. The major stale statements removed or corrected.
3. The major current architectural facts added.
4. Confirmation that no kernel/schema/code behavior was changed.
5. Verification commands run and results.
6. Any repository discrepancy discovered that should be reviewed separately.

Do not claim "fully verified" unless the actual repository evidence supports it.

==================================================
SUCCESS CRITERION
==================================================

When finished, a new coding agent reading AGENTS.md should understand:

- what Solvent is NOW,
- what the frozen kernel is responsible for,
- what the kernel is explicitly NOT responsible for,
- how authority is established,
- why exact target/snapshot binding matters,
- why AUTHORIZE != EXECUTE,
- how adapters/services/policies/executors/deployments extend the system,
- where authentication and policy live,
- what the MCP trust boundary is,
- what the important database invariants are,
- what residual risks are consciously accepted,
- and, above all:

> GROW THE ECOSYSTEM, NOT THE KERNEL.

The final AGENTS.md must describe the CURRENT repository, not an earlier Solvent milestone.
```
