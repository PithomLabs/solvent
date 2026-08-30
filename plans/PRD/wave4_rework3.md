## Technical Lead Review

**Verdict: REWORK REQUIRED — but this is a straightforward implementation pass.**

The important thing is that this file confirms **the fixes have not actually been applied yet**. The coding agent documented the six findings rather than implementing them. 

I agree with all six.

### Required

1. **F1 — contradiction warning in `Run()`**
   Must emit the warning on the actual CLI path, including `source_url`. 

2. **F2 — populate provenance**
   `Result.Normalized` cannot remain zero-valued if the CLI is supposed to show the source. 

3. **F4 — fix CVE aggregation**
   This is the most important correctness fix. Different CVEs must never be merged into one belief. 

4. **F3 — documentation**
   Correct the embedding statement; **do not add the column**. 

5. **P4 — test count**
   Correct 81 → 80. 

6. **P5 — deterministic ordering**
   Sort the resulting claims rather than relying on Go map iteration. 

### Additional finding: remove redundant intent proposal

**Accept.**

If `Run()` already performs `ProposeIfNew`, `main.go` doing it again is unnecessary. It isn't dangerous because the operation is replay-idempotent, but it makes the architecture less clean. 

I'd remove it **now**, since we're already touching `main.go`.

---

## One correction to the rework document

The phrase:

> "all six rework items ... were identified ... but the code fixes were never applied"

is important. 

So **do not ask for another review yet**. The next action is simply implementation.

After implementation:

```text
Wave 4 rework
      ↓
Build/test verification
      ↓
Technical Lead review
      ↓
Wave 4 FROZEN
```

I do **not** want another full adversarial cycle unless the implementation introduces something unexpected.

---

## Prompt for Claude Code

```text
Implement the Wave 4 rework now.

WAVE4_REWORK.md is a rework specification, not merely a report.

The six findings documented there MUST be fixed.

Do not redesign Wave 4.
Do not modify Waves 1–3.
Do not add schema changes.
Do not add embeddings.
Do not begin bchat, real-data integration, graph, Taskfile, AWS, UI, or
any other post-Wave-4 work.

============================================================
F1 — CONTRADICTION WARNING IN Run()
============================================================

Fix pipeline.Run() so the contradiction path emits slog.Warn.

The warning MUST include:

- source_url
- source_type

The CLI path must visibly produce the contradiction warning.

Reuse a shared warning helper if practical so ProcessEvidence and Run()
cannot diverge.

Add/update a test proving the Run() path emits the warning.

Preserve:

- Contradiction=true
- no belief mutation
- no evidence mutation

Do not implement automatic retraction.

============================================================
F2 — RESULT PROVENANCE
============================================================

Fix Run() so Result contains the actual normalized provenance.

The CLI transcript must show:

- source type
- source_url

Use the normalized evidence produced during Phase 1.

Do not fabricate provenance.

Add/update the appropriate test.

============================================================
F4 — SAFE CVE AGGREGATION
============================================================

Fix aggregateDeriveResults.

Different CVE-bearing claims MUST remain independent.

Example:

    etcd / CVE-A
    redis / CVE-B

must produce separate DerivedBelief results.

Do NOT merge CVE-B evidence into the CVE-A belief.

Only merge non-CVE supporting evidence when it belongs to the same CVE
primary.

Use the minimal CVE-aware approach described in WAVE4_REWORK.md.

Do not create a generic multi-domain aggregation framework.

Do not move aggregation into the kernel.

Add a regression test with at least two different CVEs/products.

Verify that evidence from one CVE cannot contribute to debt retirement
for another CVE.

============================================================
F3 — EMBEDDING DOCUMENTATION
============================================================

Correct documentation so it consistently states:

"The schema has no embedding column by design (I-6). The ledger operates
correctly with zero vectors."

Do NOT modify the schema.

Do NOT add embeddings.

============================================================
P4 — TEST COUNTS
============================================================

Correct all documentation receipts:

- kernel: 27
- belief: 18
- derive: 16
- normalize: 11
- intent: 3
- pipeline: current actual count after tests
- current baseline total: 80 before new regression tests

Do not manipulate tests merely to preserve a number.

After adding tests, report the actual total.

============================================================
P5 — DETERMINISTIC ORDER
============================================================

Sort merged DerivedBeliefs by a stable key before returning them.

Do not depend on Go map iteration order.

Add a regression assertion if appropriate.

============================================================
ADDITIONAL CLEANUP — REDUNDANT INTENT PROPOSAL
============================================================

Remove the redundant intent-proposal loop from cmd/solvent/main.go.

Run() already performs ProposeIfNew and records IntentState.

The CLI should consume that result rather than proposing the same intent
again.

Do NOT change ProposeIfNew itself.

============================================================
PRESERVE
============================================================

The following are intentionally unchanged:

- Wave 1
- Wave 2
- Wave 3
- CockroachDB schema
- kernel invariants
- intent FK gate
- sequential replay idempotency
- documented concurrent intent limitation
- contradiction-no-mutation behavior
- local fixture strategy

============================================================
TESTS
============================================================

Add/update tests for:

1. Run() contradiction warning
2. Run() provenance/source_url
3. two independent CVE claims
4. deterministic ordering with multiple claims
5. full existing end-to-end flow
6. replay idempotency
7. contradiction produces zero ledger mutation

Run:

go build ./...
go vet ./...
gofmt -l internal/pipeline cmd/solvent

go test ./internal/kernel/    -count=1
go test ./internal/belief/    -count=1
go test ./internal/intent/    -count=1
go test ./internal/derive/    -count=1
go test ./internal/normalize/ -count=1
go test ./internal/pipeline/  -count=1

Then run the CLI against CockroachDB and capture the actual transcript.

============================================================
DOCUMENTATION
============================================================

Update:

WAVE4_IMPLEMENTATION.md

Create/update:

WAVE4_REWORK.md

Document:

- each finding fixed
- files changed
- tests added
- actual test counts
- CLI receipt
- deterministic replay receipt
- two-CVE aggregation receipt
- remaining known concurrency limitation

STOP after implementation and verification.

Do NOT begin post-Wave-4 work.

Wait for Technical Lead verification.
```

**We're very close now.** This is no longer architectural exploration; it's cleanup of the final integration layer. Once this passes verification, I want to **freeze Wave 4 and stop touching the kernel**. The remaining effort should go toward the judge-facing experience and the real pinned evidence feed.
