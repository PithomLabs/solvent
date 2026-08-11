This is the result we needed. **Run 2 is the canonical Stage 1 evidence, and I would now move to Stage 2.** 

### What Run 2 proves

The most important thing is **reproducibility across a genuinely clean terminal**:

* Track 1 was selected again.
* Ingest happened first again.
* The agent read the resulting ledger.
* It independently concluded that v3.5.0 should not be deployed.
* It did not manufacture authority by retiring debt.
* Final state remained safe: **4 beliefs, 4 evidence, 0 intents, audit 0**. 

That makes the behavior substantially more credible than Run 1.

And the clean-shell provenance is now explicit: no `CLAUDE_*` variables, stdin closed, and the run was performed outside the nested Claude session. 

### The interesting result

The agent consistently chooses the **safe path without needing CockroachDB to slap its hand**.

Run 1:

```text
agent → attempts authorization → CockroachDB refuses
```

Run 2:

```text
agent → understands vulnerability → doesn't attempt authorization
```

Both arrive at:

```text
NO UNSAFE AUTHORIZATION
```

That is actually useful. It shows the database constraint is a **last line of defense**, not something the agent must repeatedly collide with.

### But there's one documentation error to correct

The Run 2 report says:

> "Authorization refusal: Reproduced"

while immediately explaining that Run 2 **never attempted authorization**. 

I'd change that wording to:

> **Authorization avoidance: reproduced. Database refusal: not exercised in Run 2.**

Also, the report says the agent's summary accurately reflects the **3 derived beliefs**, but there are actually **two accommodated + one derived** beliefs.  That's a minor terminology correction.

These are report issues, not implementation problems.

---

# Now I would move to Stage 2

And I think Run 2 gives us the answer about **which Stage 2 experiment to run**.

Don't try to force the current task to produce a refusal. The agent has demonstrated that it can legitimately answer "no" before touching the gate.

Instead, create the **promoted/authority-present variant**:

```text
                  SAME AGENT
                     │
              SAME HIGH-LEVEL
                  PROMPT
                     │
          ┌──────────┴──────────┐
          ▼                     ▼
     BASELINE STATE        PROMOTED STATE
          │                     │
          ▼                     ▼
     "Don't deploy"       "Investigate/act"
                                │
                                ▼
                         attempts authorization
                                │
                                ▼
                         CockroachDB says NO
```

That gives us the experiment we originally wanted:

> **Same agent. Same MCP. Same task. Different persistent memory → different behavior.**

That's much stronger than simply scripting a refusal.

### And then we have the real two-act story

**Act 1 — epistemic restraint**

Agent discovers that v3.5.0 is vulnerable and refuses to authorize.

**Act 2 — transactional authority**

Give it a state where an apparently promotable/authorized path exists, then challenge it. The database becomes the final authority when the agent attempts an action that violates the transactional invariant.

Then eventually:

```text
new evidence
     ↓
belief falsified
     ↓
intent cancelled
```

That is the part that makes Solvent more than an ordinary "AI + database" application.

---

## I would NOT do these things yet

* Don't add more MCP tools.
* Don't change the core.
* Don't rewrite the prompt to force authorization.
* Don't try to eliminate the agent's occasional narration errors.
* Don't run another Claude review of Stage 1.
* Don't build VS Code/OpenCode support yet.

The Stage 1 experiment has answered its question.

**Stage 1 conclusion:**

> A real Claude Code agent, isolated from the repository, can discover and use Solvent's MCP interface to inspect persistent evidence and independently avoid an unsafe deployment. The behavior reproduces across clean runs.

That's a legitimate result.

Now the next experiment should test the complementary property:

> **Can the same agent operate when the memory state says authority is potentially available—and does CockroachDB still prevent an unsupported transition?**

That is the highest-value next move.

**So I would ask Claude to design Stage 2 around the promoted-state variant, not ask the coding agent to implement anything yet.**
