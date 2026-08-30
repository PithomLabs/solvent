Wave 2 implementation plan is approved.

Implement Wave 2 exactly as approved.

Do not redesign the architecture.

Implement ONLY:

- internal/derive

Create:

- types.go
- derive.go
- derive_test.go
- testdata fixtures

Requirements

- Consume normalize.NormalizedEvidence only.
- Produce deterministic DerivedBelief values.
- No database.
- No kernel mutations.
- No debt management.
- No action intents.
- No CockroachDB code.
- No graph.
- No external services.
- No clock reads.
- No randomness.

Maintain strict separation:

Wave 2 derives beliefs.

Wave 3 mutates beliefs.

Add comprehensive unit tests.

Run:

- gofmt
- go build ./...
- go vet ./...
- go test ./...
- go test ./internal/derive/... -v

Deliverables

1. Wave 2 implementation.
2. WAVE2_IMPLEMENTATION.md containing:

- Files created
- Public API
- Rules implemented
- Test summary
- Specification traceability
- Known limitations
- Receipts

Stop after implementation.

Do not begin Wave 3.

Wait for Build Verification, Adversarial Review, and Technical Lead review.