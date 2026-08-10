This review **changes the verdict back to REWORK REQUIRED**, and I agree with it.

The important distinction is that **the architecture is approved; the implementation is not yet demo-ready**. Fable independently verified the mechanics against CockroachDB and found the actual remaining problems are mostly data integrity plus one missing Track 2 execution path. 

### The must-fix list

I would send the coding agent **exactly these five items**, in this order:

1. **Fix all five real fixtures**

   * correct GHSA/CVE metadata
   * correct postmortem URL/content/date/version claims
   * replace all three fake release SHAs/dates with the authoritative values
   * regenerate `manifest.json`
   * fix `ATTRIBUTION.md` URL/license information 

2. **Add the missing Track 2 entry path**

   * enter the `postulated` baseline belief
   * attach the v3.5.0 evidence
   * use existing `kernel.EnterBelief` / `kernel.AddEvidence`
   * no schema/kernel changes 

3. **Fix the false `intent: live` transcript**

   * don't infer intent merely because a belief is promoted
   * query actual `action_intent` state or leave it unknown 

4. **Add the scenario/belief consistency check**

   * `operator-review --scenario X --belief Y` must verify that belief Y actually belongs to X
   * otherwise fail before creating an intent
   * Fable demonstrated this can currently create a cross-scenario intent and later break falsification. 

5. **Fix advisory severity mapping**

   * use the authoritative `g.Severity`
   * don't re-derive it from prose. 

### Strongly recommend #6

Add tests for the new etcd normalizers/derivers and at least one real-fixture pipeline test. Fable found **zero automated tests exercising the newly introduced data path**, which is exactly how the fabricated fixture data escaped detection. 

I would do this. It's cheap and directly protects the thing we're about to show judges.

### What we should NOT do

Do **not** reopen:

* schema
* kernel
* debt model
* belief-edge semantics
* overall architecture
* research
* dataset selection
* bchat
* demo design

Fable explicitly says the architecture is approved and that the required work is data correction + one small additive CLI path. 

So our state is now:

```text
Solvent architecture        ✅ APPROVED
Core mechanics              ✅ APPROVED
etcd integration            ⚠️ fix
Real-world provenance       ❌ fix
Track 2 reproducibility     ❌ fix
Judge-facing transcript     ⚠️ fix
```

After those fixes, **one final mechanical re-verification** and then we freeze the core and move to Taskfile/playground/bchat.

And frankly, Fable found something valuable here: the **real data makes the demo better**. The actual postmortem directly supports the deployment-decision invalidation story, and the real v3.5.28 release preceded the advisory publication by only about three hours. 

**So I would absolutely fix this rather than simplify the demo.**
