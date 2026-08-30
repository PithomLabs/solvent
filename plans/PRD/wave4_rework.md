## Technical Lead Review

**Verdict: REWORK REQUIRED**

The plan is admirably small and is aligned with the MVP, but there are **three issues I would fix before implementation**. 

### P0 — Replay/idempotency contradiction

The plan simultaneously says:

* the full pipeline is idempotent,
* `intent.Propose` is **not** idempotent,
* replay grows intent count,
* yet the acceptance criterion calls for identical state/transcript.  

Don't hand-wave this.

For the MVP, make **intent proposal idempotent at the pipeline level**: before proposing, detect an existing live intent for that belief/action and don't create another.

No schema change needed.

### P0 — Contradiction result is internally inconsistent

The plan says:

> `ProcessEvidence` returns a Result with `BeliefID=""` and `Beliefs` containing the contradiction.

But Wave 3's `belief.Process` currently logs the contradiction and returns without persisting it. So the pipeline needs to explicitly preserve the `DerivedBelief` in its **in-memory Result** rather than implying that Wave 3 processed it. 

Define clearly:

```text
contradiction
    ↓
derive result
    ↓
pipeline Result contains contradiction
    ↓
slog warning
    ↓
NO ledger mutation
```

Don't invent persistence for it.

### P1 — "byte-identical deterministic transcript" needs tightening

The CLI defaults `scenario` to an auto-generated UUID. If the transcript contains scenario IDs, belief IDs, timestamps, etc., two runs cannot simply be expected to produce byte-identical output. 

Make the deterministic test use an **explicit fixed scenario ID** and ensure the transcript contains only deterministic fields.

Don't compare database-generated UUIDs or timestamps.

---

## One strategic concern

The plan deliberately says:

> **Evidence feed = simulated local JSON.** 

That's fine **for Wave 4 as the kernel integration milestone**.

But this is important: **this cannot be the final hackathon demo artifact.**

Our eventual demo needs the "middle ground" we've discussed:

```text
real-world source
      ↓
pinned / curated real snapshot
      ↓
local deterministic replay
      ↓
Solvent
```

That gives us **real evidence without making the demo dependent on a live external API**.

So don't add live API integration to Wave 4. Keep this plan small. We'll add a real-data snapshot after the pipeline works.

---

# Everything else

The scope is excellent:

* no schema changes
* three new files
* existing frozen waves reused
* real CockroachDB
* real transactional invariants
* no bchat/UI yet
* no AWS yet
* no graph yet.  

That's exactly the discipline we want.

The end-to-end acceptance criteria are also pointed in the right direction: **promoted belief + zero illegal live intents + contradiction signal**. 

---

# Decision

```text
REWORK REQUIRED

P0
1. Make pipeline intent proposal idempotent.
2. Clarify contradiction handling/result semantics.

P1
3. Make deterministic transcript criteria actually deterministic.

Do NOT expand Wave 4 to live APIs, bchat, graph, AWS, or UI.
```

### Prompt to Claude Code

```text
Revise WAVE4_IMPLEMENTATION_PLAN.md according to the Technical Lead review.

Do NOT implement yet.

Keep the existing Wave 4 scope and architecture.

Required changes:

1. INTENT IDEMPOTENCY — P0

The plan currently says the pipeline is idempotent while intent.Propose
creates a new intent on every call.

Change the Wave 4 pipeline design so replaying the same evidence does NOT
create duplicate live intents.

No schema changes.

Use existing database/kernel capabilities to detect an existing live
intent for the same belief/action before proposing a new one.

Define the exact behavior in the plan and add an integration-test assertion.

2. CONTRADICTION SEMANTICS — P0

Clarify ProcessEvidence behavior for DerivedBelief.Contradicts.

The frozen Wave 3 behavior is:

- contradiction detected
- warning emitted
- automatic retraction deferred
- no belief/evidence mutation

Therefore Wave 4 must preserve the contradiction in the returned in-memory
Result for transcript/testing purposes, while making clear that it was NOT
persisted as a belief.

Explicitly document:

contradiction
→ Result contains contradiction
→ warning emitted
→ no ledger mutation

Do not add a contradiction table.
Do not add belief_edge.
Do not implement automatic retraction.

3. DETERMINISTIC TRANSCRIPT — P1

The CLI currently defaults scenario ID to an auto-generated UUID.

Revise the deterministic acceptance test so it uses an explicit fixed
scenario ID.

Ensure the transcript contains only deterministic values.

Do not claim byte-identical output if it contains generated IDs,
timestamps, or other nondeterministic fields.

Define exactly which fields are compared.

4. REAL DATA — DO NOT EXPAND WAVE 4

Keep the current local fixtures for this implementation wave.

Do NOT add:

- live GitHub API
- live CISA/KEV API
- polling
- network dependencies

We will later replace/augment the fixtures with a pinned snapshot of real
world evidence for the final hackathon demo.

5. PRESERVE MVP SCOPE

Do not add:

- bchat
- graph
- AWS deployment
- UI
- embeddings
- multi-domain support
- action execution
- additional tables
- schema changes

Update:

WAVE4_IMPLEMENTATION_PLAN.md

Include revised:

- APIs
- replay semantics
- contradiction semantics
- deterministic transcript criteria
- integration tests
- acceptance criteria

Stop after revising the plan.

Do not write implementation code.
Wait for Technical Lead review.
```
