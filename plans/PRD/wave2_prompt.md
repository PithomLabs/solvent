The verification is exactly what I wanted.

**Wave 1 is now frozen.** The verifier stayed disciplined, verified only the approved findings, confirmed all seven fixes, reran the receipts, and did not reopen deferred design decisions. 

Now we move to **Wave 2: Belief Derivation**.

---

```text
Implement Wave 2 only.

Wave 1 is frozen.

Do NOT modify `internal/normalize`.

The architecture and DATA_PIPELINE_SPEC.md remain frozen.

Mission

Implement the Belief Derivation engine.

Scope

Create:

- internal/derive

Implement:

- deterministic belief derivation from NormalizedEvidence
- rule-based (no LLM)
- belief classification
    - derived
    - accommodated
    - postulated
- belief lifecycle transitions
- debt generation
- provenance mapping
- dependency extraction (logical only; no graph package)
- comprehensive unit tests
- fixtures required by Wave 2

Do NOT implement:

- CockroachDB persistence
- kernel transaction logic
- action intents
- graph visualization
- recursive graph traversal
- GitHub API
- KEV downloader
- embeddings
- vector search
- bchat integration
- UI
- Taskfile
- demo
- AWS

Requirements

- Consume only `normalize.NormalizedEvidence`.
- Derivation must be deterministic.
- Same evidence must always produce identical beliefs.
- No clock reads.
- No randomness.
- No external services.
- Fail loudly on invalid evidence.
- Keep business rules explicit and easily reviewable.
- Do not hardcode etcd-specific logic beyond the approved fixture rules.

Before writing code, produce:

WAVE2_IMPLEMENTATION_PLAN.md

Include:

1. Scope
2. Files to create
3. Package layout
4. Public API
5. Rule engine design
6. Belief derivation rules
7. Debt generation rules
8. Claim classification rules
9. Specification traceability
10. CockroachDB assumptions (if any)
11. Risks
12. Receipts that will prove correctness

Do not write implementation yet.

Stop after producing the implementation plan.

Wait for Technical Lead review before coding.
```
