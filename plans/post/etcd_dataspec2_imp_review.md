I would **not call this fully done yet**. The implementation report is good, but there are **two things that need verification before we declare the implementation complete**.

### 1. Biggest issue: DB-dependent tests were skipped

The report says:

> `DB-dependent tests | SKIP (CockroachDB not running)` 

That's important because the core behavior we're adding is **database behavior**:

* debt retirement
* promotion refusal/success
* intent FK gating
* RetractCascade
* replay/deduplication
* audit invariant

So `go build`, `go vet`, formatting, and unit tests are not enough.

**Next action:** start CockroachDB and run the full test suite, then exercise both etcd tracks end-to-end.

---

### 2. The implementation report says "all checks passing" when DB checks actually weren't run

The heading says:

> **Status: Implemented — all checks passing** 

but later explicitly says DB-dependent tests were skipped. 

That's a documentation nit, but more importantly it tells us **the implementation has not yet earned the "done" label**.

Change the status mentally to:

> **Implemented — static/unit checks passing; DB integration verification pending.**

---

## What I would do next

**Do not do another design review. Do not send it back to Fable yet.**

Have the **same coding agent** perform the actual integration verification now.

Give it this:

```text
The implementation of plans/post/etcd_dataspec2.md is complete according
to etcd_dataspec2_imp.md.

Before making ANY further code changes, perform the missing database-level
verification.

The implementation report explicitly says:

    DB-dependent tests: SKIP (CockroachDB not running)

Therefore the implementation is NOT yet considered verified.

TASK:

1. Start the project's expected local CockroachDB environment using the
   existing repository-supported mechanism. Do not invent a new setup.

2. Run the COMPLETE test suite, including all DB-dependent tests.

3. Run:
   go build ./...
   go vet ./...
   gofmt -l cmd internal
   scripts/check_i7.sh
   go test ./...

4. Exercise Track 1 end-to-end using the exact fixture directory from
   plans/post/etcd_dataspec2.md:

   internal/derive/testdata/etcd_real/track1

   Verify:
   - 3 records processed
   - 3 beliefs
   - 3 evidence records
   - correct debt retirement
   - all initial promotions refused
   - operator-review retires the five remaining debts on the vulnerability
     belief
   - promotion succeeds
   - explicit intent creation succeeds
   - audit_live_on_nonpromoted = 0

5. Exercise Track 1 replay.

   Re-run the pipeline against the same scenario and verify:
   - zero new belief rows
   - zero new evidence rows
   - no duplicate intent
   - deterministic transcript/result
   - no invariant violation

6. Exercise Track 2 end-to-end:

   internal/derive/testdata/etcd_real/track2

   Verify:
   - baseline postulated belief is entered correctly
   - v3.5.0 evidence is attached correctly
   - debts are retired
   - belief promotes
   - live intent exists
   - postmortem evidence is ingested
   - --falsify invokes RetractCascade
   - intent becomes cancelled
   - belief becomes retracted
   - retracted count is correct
   - audit_live_on_nonpromoted = 0

7. Inspect the actual database state after each stage. Do not rely only
   on CLI output.

8. Verify the five fixture records against their manifest SHA256 hashes.

9. Verify all source URLs are exactly the intended real sources.

10. Verify no schema changes occurred.

11. Verify internal/kernel/kernel.go remains unchanged.

12. Verify pipeline no longer creates intents.

13. Verify there are no belief_edge writes introduced.

14. If anything fails, diagnose the smallest root cause and fix ONLY what
    is required by etcd_dataspec2.md. Do not redesign the architecture.

15. If everything passes, update:
      etcd_dataspec2_imp.md
   so its verification status accurately says that DB integration tests
   and both end-to-end tracks have been verified.

16. Produce a concise implementation verification report containing:
   - commands run
   - tests passed/failed
   - Track 1 results
   - Track 1 replay results
   - Track 2 results
   - database-level invariant results
   - any code changes made
   - final verdict

Do NOT publish anything.
Do NOT commit anything.
Do NOT start demo/UI work yet.

The goal of this task is:

    "prove the implemented real-data Solvent path works against CockroachDB"

not merely:

    "prove the code compiles."
```

### Why this is the right next move

We're now here:

```text
Research                    ✅
Adversarial research       ✅
Implementation spec        ✅
Implementation             ✅
Static/unit verification   ✅
DB integration verification ← HERE
Demo/playground             ⏸
```

The **DB verification is the gate**. Once that passes, I would consider the real-data integration technically closed and move immediately to the demo/playground layer rather than doing another round of architectural polishing.

The implementation itself appears to track the approved spec closely: the two new source types, five fixtures, separate tracks, operator-review mechanism, and removal of pipeline intent proposal are all reported as implemented. 



## chatgpt

**Option 1 — Extend `operator-review` with an optional `--action` flag.**

That's the cleanest MVP and matches our demo philosophy:

```text
pipeline
  ↓
belief + evidence
  ↓
operator-review
  ├─ retire remaining debt
  ├─ promote belief
  └─ --action → IntentOnPromoted
                  ↓
               LIVE intent
```

Why:

* **One clean demo step** rather than making judges remember a second CLI command.
* Uses the existing `IntentOnPromoted` kernel API rather than SQL.
* Keeps intent creation explicitly **outside the pipeline**, as we deliberately decided.
* Preserves the important invariant: intent is created **only after successful promotion**.
* Makes the operator-review tool represent the complete human decision/action boundary.
* `--action` remains optional, so the tool can still demonstrate promotion without creating an intent.

I would **not** use SQL. That would bypass the kernel and violate the project's write-path discipline.

### Prompt to coding agent

```text
Extend cmd/operator-review with an OPTIONAL --action flag.

Current behavior:
    retire specified debts
    → promote belief
    → audit

Required behavior when --action is supplied:
    retire debts
    → promote belief
    → call kernel.IntentOnPromoted
    → verify/create the resulting live intent
    → audit

Requirements:

1. Do NOT modify kernel.IntentOnPromoted.
2. Do NOT move intent creation back into pipeline.Run().
3. Do NOT use SQL/db.Exec to create intents.
4. Intent creation MUST happen only after successful promotion.
5. --action must be optional; without it, preserve current behavior.
6. Define a minimal action representation consistent with the existing
   IntentOnPromoted API. Do not invent a new action model.
7. If IntentOnPromoted fails, return a non-zero exit and do not print
   VERDICT: PASS.
8. Print the resulting intent ID/state when --action succeeds.
9. Add focused tests for:
      - promotion without --action → no intent
      - promotion with --action → exactly one live intent
      - unpromoted belief → no intent
      - failed promotion → no intent
10. Verify audit_live_on_nonpromoted remains 0.
11. Run:
      gofmt -l cmd internal
      go build ./...
      go vet ./...
      go test ./...
      scripts/check_i7.sh

Then exercise the real Track 1 fixture end-to-end and verify the resulting
database state.

Do NOT make any other architectural changes.
Do NOT modify the schema.
Do NOT reintroduce intent creation into the pipeline.
Do NOT commit or publish.

Update the implementation report with the new operator-review behavior
and verification results.
```

**This is the right choice.** It gives the judge one explicit action:

> **"Review the evidence → approve the belief → authorize the action."**

That is actually a stronger demonstration of the Transactional Belief Ledger than hiding intent creation inside the ingestion pipeline.
