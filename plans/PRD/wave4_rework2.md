## Technical Lead Review

**Verdict: APPROVED WITH NITS — proceed to implementation.**

The revision resolves all three blockers cleanly:

* Intent replay is now explicitly idempotent. 
* Contradictions have an explicit in-memory/result path with **no ledger mutation**, rather than pretending they were processed. 
* Determinism now uses fixed scenario IDs and compares only deterministic fields. 

### One nit I want fixed during implementation

`ProposeIfNew` still has a **check-then-insert race**:

```text
SELECT count = 0
        ↓
intent.Propose()
```

Two concurrent callers could both observe zero and create duplicate intents.

For the MVP's sequential replay this is fine, but because this is a **transactional belief ledger**, don't present this as a concurrency-safe idempotency guarantee.

**Implementation requirement:** keep the current MVP design, but document that `ProposeIfNew` is **replay-idempotent under sequential execution**, not a database-enforced concurrent uniqueness guarantee. Do not add a schema change now.

### Strategic judgment

I am happy with the fixture decision. Wave 4 is deliberately proving the **pipeline**, not yet solving the final real-data problem. The plan explicitly keeps live GitHub/CISA integration out of scope. 

That's correct. **Do not expand Wave 4.**

---

## Send this to Claude Code

```text
Wave 4 implementation plan is APPROVED WITH ONE NIT.

Implement the approved Wave 4 plan exactly.

Before coding, read the final WAVE4_IMPLEMENTATION_PLAN.md and the frozen
Wave 1/2/3 implementation receipts.

Implement ONLY:

- internal/pipeline/pipeline.go
- internal/pipeline/pipeline_test.go
- cmd/solvent/main.go

Requirements:

1. Wire:

   normalize
      ↓
   derive
      ↓
   belief.Process
      ↓
   ProposeIfNew
      ↓
   intent.Audit

2. Preserve all frozen Wave 1–3 code and architecture.

3. Implement contradiction handling exactly as specified:

   contradiction
      ↓
   Result{Contradiction:true, BeliefID:""}
      ↓
   warning
      ↓
   NO ledger mutation

4. Implement replay-idempotent intent proposal using the approved
   ProposeIfNew behavior.

IMPORTANT CONCURRENCY CAVEAT:

The approved MVP uses:

   SELECT existing live intent
      ↓
   intent.Propose()

This is sufficient for sequential replay, but is NOT a database-enforced
concurrent uniqueness guarantee.

Do NOT add schema changes or invent a new kernel API.

Document in WAVE4_IMPLEMENTATION.md:

"Intent proposal is replay-idempotent under sequential execution;
concurrent duplicate-intent prevention is deferred."

5. Use the fixed scenario IDs and deterministic transcript rules from the
   approved plan.

6. Do NOT add:

- live GitHub/CISA APIs
- polling
- bchat
- UI
- graph
- AWS
- embeddings
- new tables
- schema changes
- action execution
- multi-domain support

7. Tests must cover:

- full end-to-end fixture pipeline
- promotion after all debt is retired
- partial debt blocks promotion
- contradiction produces no ledger mutation
- deterministic results
- replay does not create duplicate live intents

8. Verify:

go build ./...
go vet ./...
gofmt -l ...
go test ./...

Run the integration tests against the real CockroachDB test environment.

Produce:

WAVE4_IMPLEMENTATION.md

Include:

- implementation summary
- files changed
- tests
- receipts
- deterministic replay result
- known concurrency limitation
- known limitations

STOP after implementation.

Do not begin bchat, real-data integration, Taskfile, demo UI,
AWS integration, or presentation work.

Wait for Technical Lead review.
```
