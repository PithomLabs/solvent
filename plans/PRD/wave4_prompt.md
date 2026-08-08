You are now preparing Wave 4.

Wave 1 — Normalization: FROZEN.
Wave 2 — Belief Derivation: FROZEN.
Wave 3 — Transactional Belief Ledger / Intent Gate: FROZEN.

Do NOT modify Waves 1–3 unless a critical integration defect is discovered.

Before writing any code, produce:

WAVE4_IMPLEMENTATION_PLAN.md

First read:

- AGENTS.md
- IMPLEMENTATION_CONTRACT.md
- DATA_PIPELINE_SPEC.md
- WAVE2_IMPLEMENTATION.md
- WAVE3_IMPLEMENTATION.md
- WAVE3_REWORK.md
- relevant existing test/fixture documentation
- the current repository state

IMPORTANT ROADMAP CONTEXT

Our North Star is the minimum viable winning hackathon demo.

The core innovation is:

    real evidence
        ↓
    deterministic normalization
        ↓
    belief derivation
        ↓
    transactional belief ledger
        ↓
    action-intent gate

The demo must make CockroachDB's role unmistakable.

We have already deliberately avoided:

- multiple agents
- multiple feeds
- large datasets
- complex graphs
- sophisticated RAG
- UI-first development
- unnecessary infrastructure
- speculative architecture

Wave 4 should therefore be the smallest integration layer that turns the frozen kernel into a credible end-to-end MVP.

DO NOT assume that "Wave 4" means building the whole demo.

Your plan must explicitly determine the minimum integration required to connect:

    one realistic evidence feed
        ↓
    Wave 1
        ↓
    Wave 2
        ↓
    Wave 3
        ↓
    actionable / non-actionable belief

Focus on the existing etcd-oriented evidence story and the current repository.

The final system must remain usable for demonstrating the general transactional-belief pattern beyond cybersecurity, but do not implement multi-domain support merely for that claim.

WAVE 4 PLAN MUST COVER

1. Exact objective
2. Exact end-to-end user/system flow
3. Input data source
4. Fixture strategy
5. Realistic evidence format
6. Normalization integration
7. Derivation integration
8. Ledger integration
9. Intent-gate integration
10. What is real vs simulated
11. Required database/schema changes, if any
12. Required CockroachDB functionality
13. Package/file layout
14. Public APIs
15. Error handling
16. Idempotency/replay behavior
17. Integration-test strategy
18. Demo-critical acceptance criteria
19. Explicit non-scope
20. Estimated implementation effort
21. Risks that could threaten the August 18 deadline
22. Receipts proving the MVP works

CRITICAL ARCHITECTURAL RULES

- Kernel owns correctness.
- Higher layers translate into kernel operations.
- Do not move business logic back into the kernel.
- Do not weaken database invariants.
- Do not introduce another orchestration layer unnecessarily.
- Do not add a table merely for convenience.
- Do not add infrastructure unless the end-to-end MVP genuinely requires it.

COCKROACHDB

Where database behavior is involved, consult the CockroachDB documentation MCP before making assumptions, especially for:

- transaction behavior
- SERIALIZABLE semantics
- retry behavior
- constraints
- vector functionality if actually required
- test/development setup

Do not add CockroachDB features merely because they are available.

HACKATHON DISCIPLINE

For every proposed component, explicitly answer:

"Does this make the end-to-end demo stronger?"

If not, put it in OUT OF SCOPE.

Do NOT plan yet:

- bchat integration
- polished UI
- graph visualization
- AWS deployment
- video
- Devpost
- Taskfile polish

Those come after the minimum end-to-end system is proven.

DELIVERABLE

Produce only:

WAVE4_IMPLEMENTATION_PLAN.md

Do not implement anything.

Stop and wait for Technical Lead review.