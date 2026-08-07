You are implementing Wave 3 of Solvent.

Before planning, read completely:

- AGENTS.md
- IMPLEMENTATION_CONTRACT.md
- DATA_PIPELINE_SPEC.md
- WAVE2*.md files

The architecture is frozen.

Wave 1 and Wave 2 are frozen.

Do NOT redesign the architecture.

----------------------------------------------------------------------
Project Context
----------------------------------------------------------------------

Solvent is a Transactional Belief Ledger.

It is NOT an agent framework.

It is NOT a chatbot.

It is NOT a RAG system.

The core innovation is that autonomous agents cannot act on stale beliefs because the database enforces the belief lifecycle.

Everything else (bchat, UI, demo, graph, AWS) exists only to demonstrate this capability.

----------------------------------------------------------------------
Current Roadmap
----------------------------------------------------------------------

Phase 0
✅ Architecture
✅ Specifications
✅ Engineering workflow

Phase 1 (Kernel)

✅ Wave 1
Normalization

Reality
    ↓
NormalizedEvidence

✅ Wave 2
Belief Derivation

NormalizedEvidence
    ↓
DerivedBelief

▶ Wave 3 (THIS WORK)

DerivedBelief
    ↓
Transactional Belief Ledger

belief
evidence
action_intent

This is the most important implementation phase.

This is where derived beliefs become transactional database objects.

----------------------------------------------------------------------
Wave 3 Mission
----------------------------------------------------------------------

Wire the semantic outputs of Wave 2 into the Transactional Belief Ledger.

Wave 3 owns:

- belief lifecycle
- evidence attachment
- debt tracking
- promotion rules
- retraction rules
- action gating
- transactional invariants

Wave 3 does NOT own:

- normalization
- belief derivation
- GitHub
- KEV
- polling
- embeddings
- vector search
- UI
- bchat
- graph visualization
- AWS
- demo

----------------------------------------------------------------------
Primary Goal
----------------------------------------------------------------------

Implement the minimum kernel necessary to prove the thesis:

"A belief may become actionable only while its transactional invariants remain true."

The implementation should make it impossible (or transactionally rejected) for stale beliefs to remain actionable.

----------------------------------------------------------------------
Engineering Principles
----------------------------------------------------------------------

Database before application.

Transactions before prompts.

Invariants before convenience.

Explicit correctness before abstraction.

Prefer schema constraints and transactions over application logic whenever practical.

----------------------------------------------------------------------
Your Task
----------------------------------------------------------------------

Do NOT implement Wave 3.

First produce:

WAVE3_IMPLEMENTATION_PLAN.md

The plan should include:

1. Scope
2. Explicit non-scope
3. Package/file layout
4. Public API
5. Kernel responsibilities
6. Belief lifecycle
7. Evidence lifecycle
8. Action intent lifecycle
9. Transaction boundaries
10. CockroachDB responsibilities
11. Invariants enforced by database
12. Invariants enforced by Go
13. Retry strategy (CockroachDB SERIALIZABLE)
14. Failure modes
15. Unit test strategy
16. Integration test strategy
17. Specification traceability
18. Risks
19. Receipts proving correctness
20. Estimated implementation order

----------------------------------------------------------------------
CockroachDB
----------------------------------------------------------------------

This is the first phase where CockroachDB becomes a first-class participant.

Consult CockroachDB documentation (using MCP where appropriate) for:

- SERIALIZABLE transaction semantics
- retry handling
- SQLSTATE 40001
- transaction boundaries
- foreign keys
- CHECK constraints
- recursive CTE support
- best practices for transaction retry loops

Do not assume PostgreSQL behavior.

----------------------------------------------------------------------
Reminder
----------------------------------------------------------------------

We are optimizing for the hackathon MVP.

Do not build infrastructure.

Do not build integrations.

Do not build the demo.

Build the kernel.

The demo, bchat integration, and visualization will simply expose the kernel after it is complete.

Stop after producing:

WAVE3_IMPLEMENTATION_PLAN.md

Do not write implementation code.
Wait for Technical Lead review.