## prompt

wait, just to clarify, tell me in a high level where are we going with this phase? I mean how does the workflow goes? I just want to remind you what the hackathon is looking for per attached


Yes. Looking back at the **actual hackathon brief**, I would frame this phase a little differently. The MCP agent demo is **not the product by itself**. It is the final proof that Solvent's CockroachDB-backed memory actually changes how an agent operates.

The hackathon explicitly asks for an **agentic application using CockroachDB as persistent memory**, with the agent storing, retrieving, and acting on memory, and it requires **at least two CockroachDB tools plus at least one AWS service**. 

## Where we are going

Think of the whole project as:

```text
                    ┌─────────────────────┐
                    │    REAL AI AGENT    │
                    │  Claude Code/etc.   │
                    └──────────┬──────────┘
                               │
                               │ MCP
                               ▼
                    ┌─────────────────────┐
                    │   SOLVENT MEMORY    │
                    │                     │
                    │ beliefs              │
                    │ evidence             │
                    │ debt                 │
                    │ authorization        │
                    │ retraction           │
                    └──────────┬──────────┘
                               │
                               │ transactional SQL
                               ▼
                    ┌─────────────────────┐
                    │    COCKROACHDB      │
                    │ persistent memory   │
                    │ + constraints       │
                    └─────────────────────┘
```

The current phase is specifically about putting the **real agent on top of the system**.

### Before this phase

We proved that Solvent itself works:

```text
real evidence
   ↓
belief
   ↓
debt
   ↓
promotion
   ↓
authorization
   ↓
new evidence
   ↓
falsification
   ↓
authorization cancelled
```

We also proved that the MCP server exposes this safely and that CockroachDB itself enforces the important boundaries.

So we already have the **memory engine**.

### This phase

Now we ask:

> **Can an ordinary AI coding agent actually use that memory to operate?**

Not:

> Can Claude follow our 11-step script?

Instead:

```text
Human gives agent objective
          ↓
Agent discovers Solvent tools
          ↓
Agent reads persistent memory
          ↓
Agent decides what to do
          ↓
Solvent accepts OR CockroachDB refuses
          ↓
Agent observes result
          ↓
Agent adapts
          ↓
Memory changes
          ↓
Agent's authority changes with the memory
```

That's why the empty workspace matters.

The agent **doesn't get the Solvent README or our choreography**. It only gets the MCP interface. Claude's revised plan explicitly makes this the Stage 1 experiment. 

---

# What the judge should experience

The ideal 3-minute story is roughly:

### Act 1 — Give an agent a real operational problem

> "Determine whether we should deploy etcd v3.5.0 to production."

The agent doesn't know our workflow.

It discovers the Solvent memory.

```text
agent
 ↓
ledger
 ↓
"there is a belief about etcd 3.5.0"
 ↓
evidence / debt
```

### Act 2 — Memory constrains action

The agent tries to authorize or otherwise advance the deployment.

CockroachDB says:

**NO.**

Not because we programmed:

```go
if debt > 0 {
    return "no"
}
```

but because the **database's transactional model refuses the state transition**.

That's the important moment.

Claude's plan correctly identifies the thesis as:

> **"The agent's narration is untrusted. The ledger is checkable."** 

### Act 3 — Memory changes

Eventually the agent can reach legitimate authority.

Then new evidence arrives.

The previous belief is falsified.

And the authorization disappears **as a consequence of the memory state**, rather than because an agent remembered to clean it up.

That's the second dramatic moment:

```text
AUTHORIZED
     │
     │ new evidence
     ▼
FALSIFIED
     │
     ▼
AUTHORIZATION REVOKED
```

Claude's design calls this the two-act narrative: refusal, then revocation. 

---

# Why this matters for *this* hackathon

This is where I would keep us disciplined.

The hackathon judging criteria aren't simply "does your MCP work?"

They specifically ask whether:

### 1. CockroachDB is meaningful agentic memory

They want CockroachDB to be more than toy queries—state, context, transactional data, etc. 

**Solvent's answer:**

> The agent's operational authority is persistent memory encoded as transactional state.

That's much stronger than:

> "We stored chat history in CockroachDB."

### 2. The agent actually uses the memory

The agent needs to **retrieve and act on persistent state**, which is exactly what this phase tests.

### 3. CockroachDB's properties matter

This is crucial.

If we could replace CockroachDB with:

```text
SQLite + some application if-statements
```

and nothing interesting changed, we're not exploiting the hackathon platform deeply enough.

Our intended story is:

```text
Agent
 ↓
Solvent
 ↓
CockroachDB constraints / transactions
 ↓
"you cannot do this"
```

and later:

```text
new evidence
 ↓
transactional retraction
 ↓
authorization disappears
```

That's why the database is part of the **behavior**, not merely storage.

### 4. AWS still has to be integrated

This is the big thing I don't want us to lose sight of.

The brief explicitly requires **at least one AWS service**. 

And at least **two CockroachDB tools** are required. 

So **this MCP phase does not finish the hackathon project**.

It proves the agent ↔ Solvent interaction.

We still need to make the final architecture satisfy the submission requirements, likely something along the lines of:

```text
                     AI AGENT
                        │
                 ┌──────┴──────┐
                 │             │
                MCP       AWS service
                 │             │
                 ▼             ▼
             Solvent       e.g. Bedrock/
                 │          Lambda/S3
                 │
                 ▼
             CockroachDB
          persistent memory
```

The exact AWS/CockroachDB second-tool choice should be decided **after** we see what the real-agent experiment tells us, rather than prematurely adding infrastructure.

---

# Therefore, our immediate workflow is very simple

We're here:

```text
[✓] Solvent core
[✓] deterministic etcd dataset
[✓] Track 1 / Track 2
[✓] CockroachDB enforcement
[✓] MCP server
[✓] MCP adversarial review
[✓] MCP hardening
        │
        ▼
     WE ARE HERE
        │
        ▼
[ ] REAL CLAUDE CODE AGENT
[ ] EMPTY WORKSPACE
[ ] MCP ONLY
[ ] OBSERVE WHAT IT ACTUALLY DOES
        │
        ▼
[ ] decide anti-puppet variant
[ ] decide final demo choreography
        │
        ▼
[ ] integrate required AWS service
[ ] ensure 2+ CockroachDB tools
        │
        ▼
[ ] final 3-minute demo
[ ] README / architecture
[ ] submission
```

The hackathon deadline shown in your brief is **August 19, 2026 at 5:00 AM GMT+8**, so we should resist turning this into another months-long architecture exercise. 

## The most important distinction

**Solvent is the agent's memory substrate.**

**MCP is the interface into that memory.**

**The real-agent experiment proves the memory is actually useful to an agent.**

**CockroachDB is what makes the memory transactional and authoritative.**

**AWS + the second CockroachDB capability complete the hackathon requirements.**

And the final video needs to make all of that visible in **under three minutes**. 

So I would **not let us get distracted by making the agent demo elaborate**. The immediate goal is simply:

> **Put Claude Code in the box, give it the objective, give it only Solvent MCP, and see whether the memory model actually guides its behavior.**

Then we design the final hackathon presentation around what we learn.



