You are performing the Wave 2 Adversarial Code Review.

Wave 2 implementation is complete.

This is NOT a design review.

The architecture is frozen.

The implementation plan is approved.

Your job is to determine whether the implementation faithfully realizes the approved design.

Do NOT redesign the system.

Do NOT propose future features.

Do NOT expand scope.

Review ONLY:

- internal/derive
- associated tests
- WAVE2_IMPLEMENTATION.md

Reference:

- AGENTS.md
- IMPLEMENTATION_CONTRACT.md
- DATA_PIPELINE_SPEC.md
- WAVE2_IMPLEMENTATION_PLAN.md

Perform four independent review passes.

======================================================================
PASS 1 — Distributed Systems Engineer
======================================================================

Review:

- determinism
- purity
- idempotency
- hidden state
- concurrency assumptions
- architectural drift
- specification conformance

======================================================================
PASS 2 — Data Engineering
======================================================================

Review:

- derivation correctness
- rule completeness
- provenance preservation
- contradiction handling
- evidence mapping
- classification correctness

======================================================================
PASS 3 — Adversarial Systems Engineer
======================================================================

Attempt to break the implementation.

Look for:

- malformed evidence
- unsupported source types
- ambiguous rules
- contradictory evidence
- replay behavior
- invalid assumptions
- panic conditions
- silent failures

======================================================================
PASS 4 — CockroachDB Engineer
======================================================================

Although this package contains no database code, verify:

- no leakage of persistence concerns
- no transaction assumptions
- no kernel responsibilities
- clean boundary for Wave 3

======================================================================
Cross-Pass Synthesis
======================================================================

Produce one unified report.

Classify findings as:

P0 — Must Fix
P1 — Should Fix
P2 — Nice to Have

For every finding include:

- Specification section violated
- Implementation location
- Why it matters
- Recommended fix

Do NOT redesign the architecture.

Do NOT reopen deferred Technical Lead decisions.

======================================================================
Verification
======================================================================

Verify:

- go build ./...
- go vet ./...
- go test ./...
- Wave 1 still passes
- Wave 2 tests pass
- No Wave 3 functionality exists

======================================================================
Verdict
======================================================================

Output exactly one:

- REWORK REQUIRED
- APPROVED WITH NITS
- APPROVED

If APPROVED or APPROVED WITH NITS, explicitly state whether Wave 2 is ready to freeze.

Produce:

WAVE2_ADVERSARIAL_REVIEW.md