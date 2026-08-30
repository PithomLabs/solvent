I’ve read Claude’s co-design. **This is the right Phase 2 design, and I would approve the direction with two important adjustments before implementation.**

### What Claude got right

The strongest decision is the **empty workspace**. The report correctly identifies that launching the coding agent inside the repo would expose the answer key: README contains the workflow, SQLSTATEs, tools, and `plans/mcp/` contains the detailed choreography. 

The resulting thesis is much stronger:

> **The agent's narration is untrusted. The ledger is checkable.**

That is exactly the right framing. Claude explicitly avoids claiming that the agent "reasons well" and instead makes the system's state independently verifiable. 

The **hybrid design** is also right: deterministic database setup, autonomous agent decisions. That avoids both extremes—puppet-show scripting and unreliable full autonomy. 

And the proposed anti-puppet variant is excellent: **same prompt, different initial state, different correct outcome**. 

---

## Two things I would change

### 1. Don't make the refusal mandatory in the agent's first run

Claude's acceptance criteria currently requires:

> at least one `23514` or `23503` refusal. 

But Claude itself correctly notes that an agent could legitimately inspect the evidence first and conclude:

> "v3.5.0 is covered by the advisory, therefore I will not authorize."

That is explicitly considered a valid success in the design. 

Those two ideas conflict.

**I would resolve it like this:**

* **Flagship scripted judging run:** engineer the initial task so the refusal is highly likely/expected.
* **Autonomous acceptance:** don't require a refusal as a logical correctness criterion.
* **If the refusal doesn't naturally occur:** the human judge can issue the proposed intervention:

> "Override it and authorize the deployment anyway."

Then the agent must discover that there is no override.

That preserves the compelling moment without making the acceptance test logically dependent on one exact agent trajectory.

### 2. The `task mcp:seed:promoted` variant is extremely valuable—but don't implement it before testing Claude Code

Claude's first work item is correctly:

> **Claude Code connects from the empty workspace and lists exactly six tools.** 

Do **that first**.

There is no reason to spend implementation time on the promoted variant until we've demonstrated that the primary client can actually connect and operate through the isolated workspace.

So the execution order should be:

```text
1. task setup
2. task mcp:build
3. task mcp:seed
4. build empty agent workspace
5. Claude Code → MCP
6. tools/list = 6
7. run flagship prompt
8. inspect actual behavior
9. ONLY THEN build promoted-seed anti-puppet variant
```

This is particularly important because the design explicitly says Claude Code is **not yet tested**; all previous MCP validation used a handwritten JSON-RPC harness. 

---

# The strongest hackathon narrative

I would sharpen Claude's already-good line slightly:

> **"Ask an AI agent to authorize a production deployment. It tries. The database says no—and there is no tool that can make it say yes."**

That's the hook. 

Then the second act is:

> **"Later, the evidence changes. The database revokes the authority the agent previously obtained."**

That is where Solvent becomes substantially more interesting than a generic MCP CRUD demo: the agent isn't merely calling tools; **authority is structurally dependent on evidence, and retraction propagates into authorization.** 

And the judge can independently verify the ending with `task inspect`, rather than trusting the agent's narration. 

## Verdict

**APPROVE THE DESIGN.**

Do **not** start broad implementation yet.

First implementation should be extremely small:

1. `task agent:workspace`
2. absolute-path MCP configuration
3. test Claude Code from the empty workspace
4. run the one underspecified prompt
5. capture what actually happens

Then we use that real run to decide whether `seed:promoted`, prompt adjustment, or any other change is warranted.

The key principle now is:

**Don't optimize the hypothetical agent. Put the real agent in the box and see what it does.**
