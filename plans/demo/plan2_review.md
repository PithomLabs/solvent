I’d mark this **Approved with one important nit — do not rework the plan**.

The plan2 analysis is solid and correctly resolves the major concerns:

* Replay is **genuine idempotence**, backed by `EnsureBelief` + `evidenceExists`, not reset-based determinism. 
* No non-default CockroachDB settings are actually required; the earlier temporary-table concern is resolved by the actual code inspection. 
* F1 is correctly characterized as **single-belief retraction + cancellation of its intent**, with `belief_edge` deliberately deferred. 
* The remaining implementation work is genuinely small: Task prerequisite, truthful promoted count, README corrections, limitation, and report. 

### One correction before implementation

**G6 should be tightened.**

The original requirement was to pin the **exact CRDB version used by the verified E2E workflow**. Plan2 says:

> local binary = v26.2.0, Docker = v26.2.1, and v26.2.1 is "acceptable." 

That's slightly inconsistent with our explicit F2 requirement.

I would simply say:

> **Pin the Docker image to v26.2.0, matching the verified local environment, unless the agent first reruns and verifies the complete E2E suite against v26.2.1.**

Don't introduce a new verification cycle merely to justify 26.2.1. **v26.2.0 is the safer choice.**

### Then proceed

The implementation sequence is appropriately minimal:

```text
fix setup
   ↓
fix truthful Track 1 output
   ↓
fix README
   ↓
write implementation report
   ↓
task test
   ↓
task setup
   ↓
track1 → replay → track2 → inspect
```

That exact sequence is already laid out in the plan. 

**Verdict: APPROVED WITH ONE NIT → implement.**

And importantly, **this does not reopen the Solvent core.** The plan explicitly keeps schema, kernel, belief-edge, bchat, web UI, and network behavior out of scope. 

Once this pass succeeds, I would consider the CLI playground **done** and then we can make the MCP-server idea a deliberate next-layer decision rather than mixing it into this task.


## claude

This plan is the correct shape — it inspected before proposing, mapped findings to real file:line evidence, and confirmed the three claims I most wanted verified (F1 single-node, F3 real dedup, F5 test DB isolation) against actual code. Approve the direction. But it contains **one internal contradiction that is a latent scope breach**, **one downgraded severity that will bite during judging**, and **two honesty gaps in its own verification**. Ordered by consequence.

**C1 — BLOCKING contradiction: F2's finding refutes G6/G7, and the plan doesn't reconcile them.** The plan says in the F2 analysis that `check_i7.sh` enforces *no `CREATE TEMP TABLE` in the kernel* and the kernel uses `WITH RECURSIVE` instead — therefore no non-default settings are needed. If that's true, it is **very good news**: my F2 worry (temp-tables gated behind a cluster setting killing the Track 2 climax on a fresh container) is *moot by design*, and the merged prompt's §3-step-5 becomes a no-op. But the plan simultaneously lists G7 ("setup does not apply non-default cluster/session settings") as a gap to note and keeps §3-step-5 language alive. Both cannot be load-bearing. Resolve explicitly: if inspection confirms zero non-default settings, then G7 is **not a gap** — it's the correct state — and the report must *affirmatively state* "no `SET CLUSTER SETTING` required; the recursive-CTE design is exactly why," rather than listing it as a deficiency. The danger of leaving it as a limp "should note" is that a later reader re-adds a setting step to "fix" G7 and reintroduces exactly the fragility we designed out. Turn the absence into a documented, deliberate property. (This also retroactively confirms the AGENTS.md/proof lineage drifted from the current kernel — the proof used temp tables; the kernel now uses recursive CTEs to satisfy check_i7/D-033. That's fine and better, but the report should note the proof artifacts are now behind the kernel on this point.)

**C2 — MISCLASSIFIED severity: G6 is not "High-then-shrugged," it's the one reproducibility fact that must be pinned exactly, per F2's own instruction.** The plan flags the v26.2.1-image-vs-v26.2.0-verified-binary gap as High, then dissolves it to "acceptable, backward-compatible, note it." That reasoning is plausible but it is *precisely the "compatible is good enough" move F2 forbids*. The instruction was pin the **exact verified version**, not a compatible one — and the verified environment is the local v26.2.0 binary the 107 tests ran against. Two honest resolutions, and the plan must pick one, not narrate past it: either (a) pin `cockroachdb/cockroach:v26.2.0` so the container byte-matches the verification substrate — the cheaper, F2-compliant choice — or (b) if there's a real reason to prefer v26.2.1, then **re-run the 107-test baseline and both E2E tracks against v26.2.1** and record *that* as the new verified version, making the image and the verification substrate agree. What's not acceptable is shipping an image one patch ahead of anything that was ever verified and calling the delta a footnote. Recommend (a): it's one character, it eliminates the discrepancy, and "the judge's container is the exact build we verified" is a cleaner claim than any compatibility argument. This is a STOP-and-reconcile item, not a note.

**C3 — MAJOR honesty gap: G8's fix removes one hard-coded value but the plan never audited for the rest.** Finding a hard-coded `"Promoted 1"` in track1.sh is exactly the class of defect the whole anti-fabrication posture exists to catch — good catch. But fixing that single line while the reviewer checklist (point 10) demands *grepping all demo output paths for every success string* means the plan is patching the instance it stumbled on rather than clearing the class. Before "done," grep track1/track2/inspect/replay for every displayed constant — `SAFE`, `LIVE`, `RETRACTED`, `CANCELLED`, `PROMOTED`, every count — and confirm each is DB-derived. The plan should state the grep was run and list what it found, not fix one line and move on. If `"Promoted 1"` was hard-coded, the prior is that its neighbors (`Audit SAFE`, the Track 2 summary block) may be too, and those are the highest-stakes lines in the demo. Treating G8 as "Low, one-line fix" understates that it's evidence of a pattern that needs a sweep.

**C4 — MINOR but load-bearing: the plan verified dedup logic exists; it did not verify dedup *works end-to-end under replay*, which is a different claim.** F3's analysis reads `EnsureBelief`'s find-or-create CTE and `evidenceExists`'s guard and concludes genuine idempotence — correct at the code level. But "the CTE is written correctly" and "replaying Track 1 against a populated DB produces byte-identical counts" are two claims, and only the second is what the judge sees. The plan's execution order does run the replay (step 7), but it should make the *idempotence assertion itself* the pass/fail gate — capture before/after counts for beliefs, evidence, *and* intents, and require equality — rather than treating the run as a smoke test. One subtlety the plan glossed: `EnsureBelief` dedups on `(scenario_id, claim)` and `evidenceExists` on `(belief_id, content_sha256)` — so idempotence holds only if the replay presents the *same claim strings and same content hashes*. If any pipeline step derives a claim string with a runtime-varying component, dedup silently misses and replay forks. Confirm the claim keys are themselves deterministic across runs; that's the actual precondition for F3, and it wasn't checked.

The rest is right and I won't pad it: G1/F4 (add the Task check) is correct and complete; F5's finding that tests use `fable_test` and don't need the demo container is exactly the isolation that makes `task test` reproducible, and confirming `check_i7.sh` exists closes my F5 doubt; F6 is verified correctly (scripts read `source_url`/`provenance_class`, not `ingested_at`); and G2/G3 (the port drift, 26260 vs 26261) is a real doc bug worth fixing even at Low, because a judge who pastes the wrong port hits a failure on their first command and that first impression is the whole game. The plan also implicitly settles a naming echo worth noting: the DB is `fable`, tests use `fable_test`, the repo is `solvent` — "Fable" survives as an internal codename while "Solvent" is the public name, which matches where we landed.

Net: the plan is executable after four changes — reconcile C1 (absence-of-settings is a documented property, not a gap), resolve C2 (pin v26.2.0, or re-verify against v26.2.1 — pick one, don't footnote), sweep C3 (grep all output constants, not just the one found), and harden C4 (make replay's count-equality the assertion, and confirm claim keys are deterministic). Items 1–7 of the execution order are otherwise sound. The one I'd least tolerate slipping is C2, because "we shipped judges a build we never actually tested" is the single most avoidable reproducibility failure on the list, and it's a one-character fix.


## final review

I agree with this adversarial assessment. **It upgrades the plan from “approved with nit” to “approved after these four corrections.”** The four points are substantive and all can be resolved without reopening architecture.

### Required changes before implementation

1. **C1 — Remove G7 as a gap.**

   * Explicitly state that **no non-default CockroachDB settings are required**.
   * Explain that the current recursive-CTE `RetractCascade` is why.
   * Do not add a pointless cluster-setting step.
   * Record that the older D-033/check artifact lineage used to discuss temp tables is now behind the current kernel.
     The plan's own inspection supports this. 

2. **C2 — Pin CRDB to `v26.2.0`.**

   * Do **not** accept v26.2.1 merely because it is compatible.
   * Match the Docker image to the environment in which the 107-test/E2E verification was actually performed.
   * This is the cleanest, cheapest resolution.

3. **C3 — Sweep all demo output for hard-coded claims.**

   * Grep `track1.sh`, `track2.sh`, `replay.sh`, and `inspect.sh`.
   * Check every count and every status: `SAFE`, `LIVE`, `PROMOTED`, `RETRACTED`, `CANCELLED`, etc.
   * Every displayed result must come from an actual DB query/assertion.
   * Don't just fix the discovered `"Promoted 1"` instance.

4. **C4 — Make replay verification an actual gate.**

   * Run Track 1.
   * Capture **belief, evidence, and intent counts**.
   * Replay without reset.
   * Capture them again.
   * Require exact equality.
   * Also verify the claim strings are deterministic, because the dedup keys are based on `(scenario_id, claim)` and `(belief_id, content_sha256)`. The plan already establishes those mechanisms. 

### Revised verdict

**REWORK THE PLAN, NOT THE ARCHITECTURE.**

After those four edits:

```text
C1  no CRDB settings required       → documented property
C2  CRDB v26.2.0                    → exact verified environment
C3  all demo output audited         → no fabricated presentation
C4  replay equality asserted        → genuine E2E idempotence
```

Then **implementation proceeds immediately**.

I would not send this back to Claude again after those corrections. The adversarial review has already done exactly what we wanted: it found the remaining reproducibility and honesty risks before they reached the judge-facing layer.

And I agree with the reviewer's priority: **C2 is the one I would absolutely not compromise on.** The plan explicitly says the Docker image is v26.2.1 while the verified local binary is v26.2.0.  Pinning v26.2.0 eliminates the ambiguity entirely.
