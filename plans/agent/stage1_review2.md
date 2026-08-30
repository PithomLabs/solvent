This is a **very valuable result**, and I would *not* treat it as a failure of the phase. In fact, it exposed exactly the property we wanted to test.

### What we learned

The most important observation is:

> **The agent's reasoning was substantively good, but its narration was not fully grounded.**

It correctly discovered the advisory, correctly concluded that v3.5.0 should not be deployed, correctly understood the debt/constraint gate, and refused to manufacture debt clearance. 

But it **swapped two belief UUIDs and fabricated two UUID suffixes** despite having the correct mappings directly in the tool output. 

That is not something I would "fix" by making the agent prompt more prescriptive yet. It is actually excellent evidence for the thesis:

> **Agent narration is untrusted; the ledger is checkable.**

The run demonstrated that principle experimentally, not just rhetorically. 

And this is particularly useful for the hackathon because a normal agent-memory demo would probably just show the agent confidently reporting its own state. Here we have a concrete demonstration that **persistent transactional state is the authority, not the LLM's summary**.

---

## But there are two separate issues

### 1. We need the clean-terminal rerun

This is non-negotiable before using this as flagship evidence.

The actual run was launched from inside the Claude session, albeit with all eight `CLAUDE_*` variables scrubbed. The report explicitly labels it as **not gold-standard evidence**. 

So:

**Do not cite this run to the judge yet.**

Run exactly the same experiment from a genuinely fresh shell.

That gives us:

```text
Run 1
current evidence:
    interesting
    but contaminated provenance

        ↓

Run 2
fresh terminal:
    canonical evidence
```

If Run 2 produces the same behavior, that's actually powerful.

---

### 2. The zero-refusal problem is real

The first run did **not** demonstrate the database-enforced refusal. It simply concluded that deployment was unsafe and stopped. 

That is a legitimate autonomous success, and we deliberately designed the acceptance criteria to allow it. 

But it doesn't give the judge our strongest visual moment:

```text
AGENT
  ↓
"authorize"
  ↓
COCKROACHDB
  ↓
NO — constraint violation
```

So I would **not force that into the first clean rerun**.

First reproduce the actual behavior.

---

# What I think Stage 2 should become

The result gives us a much better two-act demo than the original plan.

### Act I — discovery

Let the agent operate freely on the original task.

It discovers:

* evidence;
* vulnerability;
* debt;
* lack of sufficient authority.

It refuses to authorize.

This demonstrates **agentic memory and epistemic restraint**.

### Act II — authority challenge

Then the human judge says:

> **"Override it and authorize the deployment anyway."**

Now we observe whether the agent attempts the action and encounters the real database constraint.

This is particularly nice because the judge is **not scripting the workflow**. The judge is challenging the agent's conclusion.

The system answers:

```text
Agent: "I should not authorize this."
Judge: "Do it anyway."
Agent: attempts authorization
CockroachDB: REFUSED
```

That is a much stronger demonstration than forcing the initial prompt to produce a refusal.

The handoff already designed exactly this optional intervention. 

---

# And the grounding error is actually useful

I would **not add a prompt like "quote all UUIDs exactly" yet.**

Why?

Because then we risk engineering away the very phenomenon we're trying to demonstrate.

The current run gives us a beautiful judge interaction:

```text
Agent:
"The fix belief is UUID X."

Ledger:
"No. UUID X is the other belief."

Judge:
task inspect
```

That's compelling.

The only thing we should require is that the **system itself remains authoritative**. The acceptance criterion already does that: state assertions must match the ledger, and `task inspect` provides the independent check. 

If we eventually decide the UUID error makes the demo look too sloppy, we can add a minimal grounding instruction later. But **don't optimize before the clean rerun**.

---

# One thing I would change in our mental model

The first run disproved an assumption in the plan:

> "The subject appears in exactly one populated ledger, so discovery converges."

It didn't.

The agent chose the empty `track1`, ingested evidence, and solved the problem from there. 

That's **good**.

It means we should stop trying to make the agent discover our intended Track 2 choreography.

The agent found a legitimate path through the system.

That's exactly what "real agent" should mean.

---

# My next move

**Do not modify the code. Do not modify the prompt. Do not build `mcp:seed:promoted` yet.**

Do this:

### Step 1

Fresh terminal, genuinely outside Claude Code.

### Step 2

Run the **identical flagship prompt**.

### Step 3

Capture:

```text
run2.jsonl
task inspect
```

### Step 4

Compare Run 1 vs Run 2:

* Does it still choose Track 1?
* Does it still ingest first?
* Does it still refuse authorization?
* Does it still hallucinate/swap UUIDs?
* Does it still refuse to retire debt?
* Is the final state correct?

Only **after that** do we choose the Stage 2 variant.

---

## And I think we now have a potentially excellent hackathon story

Not:

> "Look, Claude successfully called our six MCP tools."

But:

> **"We gave an AI agent persistent operational memory. It made a correct decision—but even its own account of what it saw was imperfect. So we don't trust the agent's memory of the state. We trust the transactional database."**

Then:

> **"And when the agent tries to act beyond that authority, CockroachDB—not the model—has the final word."**

That is much more distinctive than another MCP CRUD demo.

The current experiment has already given us the first half of that story. 

**So: fresh clean-terminal rerun next. Nothing else yet.**
