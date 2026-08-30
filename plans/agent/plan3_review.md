I’ve read **Revision 3**. At a high level, this is now the right plan for the phase, and importantly it keeps the phase narrow.

### What this phase is actually doing

The workflow is:

```text
1. Deterministically seed CockroachDB
        ↓
2. Build Solvent MCP
        ↓
3. Create an EMPTY agent workspace
        ↓
4. Give Claude Code ONLY the six Solvent MCP tools
        ↓
5. Give it ONE high-level objective:
   "Determine whether we should deploy etcd v3.5.0"
        ↓
6. Let the real agent discover the memory/workflow
        ↓
7. CockroachDB enforces what the agent is allowed to do
        ↓
8. Agent observes the resulting state
        ↓
9. Judge independently runs `task inspect`
```

The critical point is that **we are not scripting the agent**. The expected 11-step sequence is explicitly illustrative, and a correct refusal to authorize is also a successful outcome. 

### Why this phase matters

It converts what we've already proven mechanically into the actual hackathon proposition:

> **An ordinary AI agent can use CockroachDB-backed persistent memory to make and attempt operational decisions, while the database—not the LLM—remains the authority.**

The plan is especially strong on this distinction:

> **The agent's narration is untrusted. The ledger is checkable.** 

That maps directly onto the hackathon's **Agentic Memory Design** and **Technical Implementation** criteria: CockroachDB isn't merely storing agent history; its persistent transactional state determines what actions are possible. 

### The actual demo story

Ideally the judge sees:

**Act 1**

> "Agent, determine whether we should deploy etcd v3.5.0."

Agent discovers the state → attempts something → **CockroachDB refuses**.

**Act 2**

If it gets to authorization, later evidence arrives → belief becomes invalid → **authorization is revoked by the database**, without the agent explicitly cancelling it. 

That's the differentiator.

A normal MCP demo says:

> "The agent can CRUD a database."

Solvent says:

> **"The agent can ask the database for authority, but the database decides whether that authority exists—and can revoke it when its evidence becomes false."**

### And the plan correctly stops there

The most important line in Revision 3 is:

> **Implement Stage 1 only. Everything after it should be decided by what the agent actually does.** 

I agree completely.

Do **not** build the promoted-seed variant yet. Do **not** optimize the prompt. Do **not** add VS Code. Do **not** expand the MCP surface.

First answer the empirical question:

> **What does Claude Code actually do when dropped into an empty workspace with only Solvent's six tools?**

That transcript is now the most valuable artifact in the entire phase.

### One thing I would watch closely

The plan claims that `--tools ""` + `--strict-mcp-config` + empty cwd gives the agent exactly six tools and no file access. It does at least recognize that this must be **verified rather than assumed**: Step 1a explicitly stops if a built-in tool appears. 

So I would **not review this plan further before implementation**.

**Verdict: approve Revision 3 → implement Stage 1 only → inspect the real transcript.**

That is the correct next move.
