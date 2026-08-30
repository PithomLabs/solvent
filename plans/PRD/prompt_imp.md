You are implementing the Transactional Belief Ledger.

The architecture is frozen.

The data pipeline specification is frozen.

Your job is NOT to redesign the system.

Your job is to faithfully implement the specification.

----------------------------------------------------------------------
Context
----------------------------------------------------------------------

The Transactional Belief Ledger is the primary product.

The demo, bchat integration, UI, graph visualization, and video are secondary.

The implementation must optimize for correctness, determinism, transactional semantics, and architectural fidelity.

Think like a distributed systems engineer, not an AI engineer.

----------------------------------------------------------------------

Read these documents completely before planning.

Required

- IMPLEMENTATION_CONTRACT.md (authoritative architecture)
- DATA_PIPELINE_SPEC.md (authoritative domain specification)
- AGENTS.md (engineering philosophy)

Read all of them completely before writing the implementation plan.

If any documents appear inconsistent:

STOP.

Do not guess.

Report the inconsistency.

----------------------------------------------------------------------

Mission
----------------------------------------------------------------------

Implement the DATA_PIPELINE_SPEC exactly as written.

The implementation should transform upstream evidence into transactional beliefs.

The pipeline ends when an action intent is either:

- accepted
- rejected
- cancelled

according to the Transactional Belief Ledger.

----------------------------------------------------------------------

Implementation Philosophy
----------------------------------------------------------------------

Do not optimize.

Do not generalize.

Do not add abstractions.

Do not add configuration.

Do not introduce new interfaces unless explicitly required by the specification.

Favor explicit code over clever code.

Prefer readability over flexibility.

Unknown behavior should fail loudly.

----------------------------------------------------------------------

Implementation Boundaries
----------------------------------------------------------------------

Implement ONLY the data pipeline.

Specifically:

- EvidenceFeed ingestion
- normalization
- canonical evidence
- belief derivation
- belief lifecycle
- intent lifecycle
- graph semantics
- audit outputs

Do NOT implement

- bchat UI
- graph visualization
- Taskfile
- demo
- video
- AWS
- Bedrock
- MCP integration
- playground
- README
- Devpost
- architecture changes

----------------------------------------------------------------------

Planning
----------------------------------------------------------------------

Before writing code,

produce

DATA_PIPELINE_IMPLEMENTATION_PLAN.md

The plan must include

1. Scope

Exactly what will be implemented.

Exactly what will not.

2. Dependency graph

Implementation order.

3. Files to modify

4. New packages

5. New tests

6. CockroachDB assumptions

7. Risks

8. Receipts that prove correctness

9. Estimated implementation order

Do not write code yet.

----------------------------------------------------------------------

CockroachDB Requirements
----------------------------------------------------------------------

Use CockroachDB documentation (via MCP where appropriate) whenever implementation depends on CockroachDB behavior.

Do not assume PostgreSQL behavior.

Explicitly identify every place where CockroachDB semantics matter.

Examples

- transactions
- retries
- SQLSTATE
- constraints
- foreign keys
- recursive queries
- vector index behavior

----------------------------------------------------------------------

Implementation Rules
----------------------------------------------------------------------

Every implementation decision must satisfy

Architecture

↓

Specification

↓

Correctness

↓

Simplicity

Never reverse this order.

----------------------------------------------------------------------

Implementation Quality
----------------------------------------------------------------------

The implementation should be

- deterministic
- testable
- transactional
- idempotent
- explicit

Avoid

- hidden state
- duplicated truth
- speculative abstractions
- premature optimization

----------------------------------------------------------------------

Deliverables
----------------------------------------------------------------------

Phase 1

DATA_PIPELINE_IMPLEMENTATION_PLAN.md

Nothing else.

Do not begin implementation.

----------------------------------------------------------------------

Future Workflow
----------------------------------------------------------------------

After the implementation plan is approved:

Implementation

↓

Correctness Review

↓

Adversarial Review

↓

Architecture Conformance Review

↓

Technical Lead Review

↓

Freeze

Do not skip any stage.