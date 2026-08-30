## Technical Lead Review

**Verdict: APPROVED WITH NITS — proceed to Wave 4 adversarial review.**

The implementation is substantially on target. It proves the critical end-to-end chain against real CockroachDB:

> **evidence → normalization → derivation → ledger → promotion → intent gate → audit**

and all **81 tests pass**. 

### What I like

* **Wave 1–3 remain untouched**, preserving the frozen architecture. 
* The pipeline actually reaches CockroachDB rather than mocking the ledger. 
* The most important receipts exist: promoted belief, zero illegal intents, replay stability, contradiction isolation. 
* The concurrent intent limitation is explicitly documented rather than falsely claiming stronger guarantees. 
* The implementation remains small enough to be an MVP despite the test-heavy 814-line footprint.

## Nits to carry into adversarial review

### N1 — Evidence aggregation is the biggest architectural question

`aggregateDeriveResults` is explicitly **etcd/CVE-specific**:

> merge everything into the first CVE-bearing claim. 

I accept this **for the MVP**, because it is outside the kernel and gives us the single coherent belief needed for the demo.

But the adversarial reviewer should verify that this cannot accidentally merge **semantically independent claims** merely because they happen to contain the same CVE.

If it is safe for the current fixture set, leave it alone. **Do not generalize it into a multi-domain abstraction now.**

### N2 — Embedding/schema statement needs reconciliation

The implementation says:

> "`embedding VECTOR(1536) NULL` not yet in `db/001_schema.sql`"

while the earlier Wave 4 plan stated that the existing schema already supported the nullable embedding column. 

This isn't a reason to reopen Wave 3, but the adversarial review should establish which statement is actually correct and ensure the documentation doesn't contradict the repository.

**Do not add embeddings merely to resolve this.**

### N3 — Intent race remains deliberately deferred

Correctly documented, so **not a blocker**. 

The adversarial reviewer should simply confirm that the demo never exercises concurrent intent proposal.

---

# Strategic verdict

**Wave 4 has achieved its actual purpose.**

We now have the minimum technical spine:

```text
REAL CockroachDB
      │
      ▼
 Evidence
      │
      ▼
 Normalize
      │
      ▼
 Derive
      │
      ▼
 Belief + Debt
      │
      ▼
 Promotion Gate
      │
      ▼
 Action Intent
      │
      ▼
 Audit = 0
```

That means **we should stop adding kernel functionality**.

The next adversarial review is effectively the last technical gate on the core MVP. After that, our priorities should shift sharply toward:

1. **real pinned evidence snapshot**
2. **bchat as the interaction surface**
3. **Taskfile / one-command setup**
4. **judge playground**
5. **visual graph**
6. **README + technical writeup**
7. **demo recording / Devpost**

That is how we protect the August 18 deadline.

---

## Send this to the coding agent for the next step

```text
Perform the Wave 4 Adversarial Code Review.

Wave 4 implementation is complete and has passed:

- go build ./...
- go vet ./...
- go test ./internal/... -count=1
- 81 tests total, all passing
- CockroachDB v26.2.0

Do NOT redesign Wave 4.

Do NOT add features.

Do NOT begin bchat, real-data integration, graph, Taskfile, AWS,
UI, or demo work.

Review ONLY the implemented Wave 4 integration against:

- AGENTS.md
- IMPLEMENTATION_CONTRACT.md
- DATA_PIPELINE_SPEC.md
- approved WAVE4_IMPLEMENTATION_PLAN.md
- WAVE4_IMPLEMENTATION.md
- frozen Wave 1/2/3 contracts and implementations

Focus particularly on:

1. Evidence aggregation

Review aggregateDeriveResults carefully.

Determine whether merging all non-contradiction evidence into the first
CVE-bearing claim can incorrectly merge semantically independent claims.

For the current etcd fixtures, determine whether the behavior is correct.

Do NOT demand a generalized multi-domain abstraction unless the current
implementation is actually incorrect.

2. Intent idempotency

Verify that ProposeIfNew is replay-idempotent as claimed.

Explicitly distinguish:

- sequential replay idempotency
- concurrent duplicate-intent prevention

The latter is intentionally deferred.

Do not treat the documented concurrency limitation as a defect unless
it violates the approved MVP contract.

3. Contradiction handling

Verify:

contradiction
→ Result.Contradiction=true
→ warning
→ no belief
→ no evidence
→ no ledger mutation

Ensure the implementation does not accidentally persist contradictory
evidence.

4. End-to-end invariants

Verify:

- partial debt cannot promote
- promoted belief can receive a live intent
- no live intent exists for a non-promoted belief
- replay does not create duplicate live intents
- AuditLiveOnNonPromoted remains zero

5. Determinism

Verify fixed scenario IDs and deterministic fixture ordering.

Ensure generated UUIDs and timestamps are not incorrectly treated as
deterministic outputs.

6. Schema/documentation consistency

Check the claim concerning:

embedding VECTOR(1536) NULL

against the actual schema.

If documentation is inconsistent, report it.

Do NOT add embedding functionality.

7. Architecture

Verify:

Wave 1 = normalization
Wave 2 = derivation
Wave 3 = kernel correctness
Wave 4 = integration/orchestration

Look specifically for business logic leaking into the pipeline that
belongs in the kernel.

Classify findings:

P0 — must fix
P1 — should fix
P2 — documentation/future

For each finding provide:

- exact file/location
- violated invariant/specification
- why it matters
- minimal fix

Do not reopen deliberately deferred features.

At the end provide exactly one verdict:

REWORK REQUIRED
APPROVED WITH NITS
APPROVED

Produce:

WAVE4_ADVERSARIAL_REVIEW.md

Stop there.
```
