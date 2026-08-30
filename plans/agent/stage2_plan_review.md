I reviewed the Stage 2 design closely. **The direction is correct, but I would make two substantive corrections before implementation.** The plan is otherwise unusually disciplined.

### 1. Fix the claim that Run B's "correct action" is to deploy

The plan says:

> "In Run A the correct answer is 'do not deploy' ... In Run B ... the correct answer requires acting on the ledger." 

That's true only if "acting" means **reconciling/retracting the stale authorization**, not deploying.

The promoted state deliberately says:

* deployment was approved;
* intent is live;
* then the agent ingests evidence saying v3.5.0 is vulnerable. 

So the correct Run B behavior is actually:

```text
existing authority
      ↓
new contradictory evidence
      ↓
recognize authority is stale
      ↓
falsify/retract
      ↓
intent cancelled
```

**Do not frame Run B as "same prompt → opposite obligation → act/deploy."**

The stronger and more accurate story is:

> **Same prompt, different memory state → different required state transition.**

Run A: don't create authority.

Run B: **reconcile existing authority when new evidence invalidates it.**

That is actually a better Solvent story.

---

### 2. Be careful with the promised 23503 refusal

The plan correctly discovers something important: **you cannot pre-seed a genuinely promoted belief and simultaneously make `authorize_action` fail**. Promotion itself establishes the condition required by the FK. 

The proposed route is:

```text
promoted
   ↓
agent falsifies
   ↓
belief retracted + intent cancelled
   ↓
agent subsequently authorizes that belief
   ↓
23503
```

That's technically coherent. 

But **don't make that the expected Stage 2 narrative**.

It's entirely possible the agent:

1. sees the promoted belief;
2. sees the contradictory advisory;
3. calls `falsify`;
4. observes cancellation;
5. stops.

That would be an excellent run, but **no 23503 occurs**.

The plan mostly acknowledges this, but its hackathon framing still gives the impression that the refusal is the climax. I'd explicitly separate:

**Primary experiment**

> Does the real agent recognize and reconcile contradictory persistent authority?

**Secondary enforcement demonstration**

> If the agent subsequently attempts to act on the retracted authority, does CockroachDB reject it with 23503?

That prevents us from result-shopping for a constraint violation.

---

## One thing I especially like

The correction at the beginning is important:

> **No real agent has ever triggered a Solvent constraint.** 

That materially changes the motivation for Stage 2. We're not polishing an already-demonstrated feature. We're testing the one major gap in the empirical story.

And the decision to put the promoted belief in **track1** is sound. The prior agent repeatedly discovered track1, while track2 was opaque to it. 

I would keep that.

---

## One more thing I'd require during implementation

Before running the real agent, do exactly what the plan says in R-5:

```text
seed promoted state
→ ingest_evidence(track1)
→ inspect ledger
```

and verify that:

* the promoted belief remains distinct;
* no claim collision occurs;
* the live intent remains attached to the promoted belief;
* the new advisory beliefs appear;
* `audit = 0`.

The plan correctly identifies this as something that must be **empirically verified rather than assumed**. 

That is important because we're deliberately putting a historical Track 2-style belief into Track 1.

---

# Verdict

**APPROVE WITH MINOR CORRECTIONS.**

I would make these wording changes before handing it to the coding agent:

1. Replace **"Run B correct = act"** with **"Run B correct = reconcile the standing authority in light of new evidence."**
2. Make **23503 a secondary/optional enforcement observation**, not the expected Stage 2 climax.
3. Preserve the existing rule: **do not force the agent to call `authorize_action` or `falsify`.**
4. Keep R-5 as a mandatory empirical preflight before the actual agent runs.

Then implement exactly the proposed **one Taskfile task**. The scope is excellent: no Go, no SQL, no MCP changes, no kernel changes. 

Most importantly, I would **not send this through another full design-review cycle after those wording fixes**. The architecture has converged. The next useful information comes from actually running Stage 2.
