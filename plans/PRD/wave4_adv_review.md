## Technical Lead Review

**Verdict: REWORK REQUIRED.**

This is a **good adversarial review**, and I agree with the three P1 findings. The reviewer also did exactly what we asked: it tested the actual CLI path and found a defect that ordinary unit tests missed. 

### P1 — F1: contradiction warning disappears in the actual demo path

**Accept.**

`ProcessEvidence()` warns, but `Run()`—which the CLI actually uses—doesn't. That means D-6 is currently false even though the underlying pipeline function works. 

This is exactly the kind of bug adversarial review should catch.

### P1 — F2: transcript loses source provenance

**Accept.**

The plan explicitly promised deterministic `source_url`, yet `Run()` leaves `Result.Normalized` empty. 

Fix it. Provenance is important to the demo because judges should be able to see **where the belief came from**.

### P1 — F4: unsafe claim aggregation

**Strongly accept.**

This is the most important finding.

The current implementation can do:

```text
CVE-A / etcd
       +
CVE-B / redis
       ↓
one belief about CVE-A
```

and then potentially let evidence for CVE-B contribute to CVE-A's debt retirement. The reviewer empirically demonstrated exactly that. 

This isn't merely future-proofing. It's a **semantic correctness defect**.

The minimal fix—group CVE-bearing claims separately—is exactly what I want. No generalized multi-domain abstraction.

---

### P2/P3 — Accept, but don't let them expand scope

* **F3:** fix the incorrect embedding documentation; **do not add embeddings**. 
* **P4:** correct the test count from 81 → 80. 
* **P5:** sort merged claims for deterministic ordering. Cheap and worthwhile. 

### Important judgment

I would **not** add any new adversarial review after these fixes.

These are now tightly bounded Wave 4 corrections. Then:

> **verification → Wave 4 freeze → stop kernel engineering.**

We need to start spending our remaining time on the actual judge experience.

---

## Prompt for Claude Code

```text
Implement the Technical Lead-approved Wave 4 rework.

Verdict: REWORK REQUIRED.

Do NOT redesign Wave 4.
Do NOT modify Waves 1–3.
Do NOT add schema changes.
Do NOT add embeddings.
Do NOT begin bchat, real-data integration, graph, Taskfile, AWS, or UI.

Fix ONLY the following findings from WAVE4_ADVERSARIAL_REVIEW.md.

============================================================
F1 — CONTRADICTION WARNING IN Run() — P1
============================================================

The CLI uses pipeline.Run(), but Run() currently returns contradiction
results without emitting the required slog.Warn.

Fix the Run() contradiction path so the demo-facing CLI produces:

- Contradiction=true
- slog.Warn
- source_url populated
- no ledger mutation

Reuse the same warning helper used by ProcessEvidence if practical,
so the two paths cannot drift.

Add/update a test proving the Run() path emits the warning.

Do NOT implement contradiction persistence or automatic retraction.

============================================================
F2 — TRANSCRIPT SOURCE PROVENANCE — P1
============================================================

Run() currently returns Results whose Normalized field is zero-valued.

Populate Result.Normalized (or use an equally small dedicated provenance
field) so the CLI transcript contains the actual:

- source type
- source_url

from the evidence that produced the result.

The approved deterministic transcript contract requires source_url.

Add/update the relevant test.

============================================================
F4 — SAFE CVE CLAIM AGGREGATION — P1
============================================================

Fix aggregateDeriveResults.

Current behavior incorrectly merges every non-contradiction claim into
the first CVE-bearing claim.

Example of the bug:

    etcd / CVE-A
    redis / CVE-B

must NEVER become:

    etcd / CVE-A
      + evidence for CVE-B

Minimal fix only:

- Treat each distinct CVE-bearing claim as its own primary claim.
- Only merge supporting evidence into a primary claim when it belongs to
  the same CVE.
- Preserve the existing etcd MVP behavior.
- Preserve deterministic ordering.
- Do not introduce a generic multi-domain aggregation framework.
- Do not move this responsibility into the kernel.

Add a regression test with two different CVEs/products proving they remain
two independent DerivedBeliefs.

Also ensure evidence for CVE-B cannot retire debt belonging to CVE-A.

============================================================
F3 — EMBEDDING DOCUMENTATION — P2
============================================================

Correct the inaccurate Wave 4 documentation.

The repository schema does NOT contain an embedding column.

State explicitly:

"The schema has no embedding column by design (I-6). The ledger operates
correctly with zero vectors."

Do NOT add an embedding column.

============================================================
P4 — TEST COUNT — P2
============================================================

Correct WAVE4_IMPLEMENTATION.md:

- kernel tests = 27
- total tests = 80

Do not change code merely to manufacture another test.

============================================================
P5 — DETERMINISTIC MAP ORDER — P3
============================================================

Make aggregateDeriveResults deterministic when multiple merged claims
exist.

Sort the resulting claims by a stable key before returning them.

Add a small regression assertion if appropriate.

============================================================
DO NOT CHANGE
============================================================

Do NOT modify:

- Wave 1
- Wave 2
- Wave 3
- schema
- kernel
- intent model
- CockroachDB transaction design
- concurrent intent limitation
- contradiction architecture
- fixture strategy
- real-data integration
- bchat
- graph
- UI
- Taskfile

============================================================
VERIFICATION
============================================================

Run:

go build ./...
go vet ./...
gofmt -l internal/pipeline cmd/solvent
go test ./internal/kernel/ -count=1
go test ./internal/belief/ -count=1
go test ./internal/intent/ -count=1
go test ./internal/derive/ -count=1
go test ./internal/normalize/ -count=1
go test ./internal/pipeline/ -count=1

Then verify the CLI path against CockroachDB.

Specifically verify:

1. normal fixtures → promoted belief
2. audit = 0
3. exactly one live intent
4. replay → still one live intent
5. contradiction → visible warning + source_url
6. contradiction → zero ledger mutation
7. two different CVEs → two independent claims/beliefs
8. transcript provenance is populated
9. deterministic result ordering

Update:

WAVE4_IMPLEMENTATION.md

Create:

WAVE4_REWORK.md

Document:

- F1 resolved
- F2 resolved
- F4 resolved
- F3/P4/P5 resolved
- test results
- CLI receipt
- remaining known limitations

STOP.

Do not begin any post-Wave-4 work.

Wait for Technical Verification.
```

This is the **last technical rework I expect before freezing Wave 4**. The adversarial review has already confirmed the deeper architecture—kernel ownership, transactional invariants, replay behavior, and frozen-wave boundaries are sound. 

After verification, we should resist the temptation to keep improving the pipeline. **The next fight is the demo.**
