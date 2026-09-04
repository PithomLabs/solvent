Yes. The agentjacking demo should now stop being **the demo** and become **Demo #1 in a coherent proof portfolio**.

The market question you need to answer next is not “can Solvent stop agentjacking?” It is:

> **Does Solvent provide a general-purpose authority layer that remains trustworthy when agents, tools, data, and workflows become untrusted?**

That is much bigger—and much more commercially interesting.

This is also aligned with where the security conversation is moving. OWASP now explicitly treats excessive agency, tool misuse, identity/privilege abuse, supply-chain compromise, and unexpected execution as distinct agentic risks. ([OWASP Gen AI Security Project][1])

## I would build a 6-demo portfolio

### Demo 1 — Agentjacking

**Story:**
“Untrusted content tries to become authority.”

You already have this.

The important thing is that the audience sees:

```text
malicious telemetry
       ↓
agent sees it
       ↓
agent attempts action
       ↓
Solvent says: evidence ≠ authority
       ↓
blocked
```

This establishes the fundamental proposition.

But don't spend much more engineering effort on it. It has already done its job.

---

# Demo 2 — The Lying Agent

### Story

The agent is not compromised by malicious text.

It simply **lies**.

For example:

> “The customer approved deletion.”

Or:

> “The incident commander authorized production restart.”

The agent then attempts the corresponding action.

Solvent asks the only question that matters:

> **Where is the authority?**

There isn't any.

So the action fails.

### Why this demo matters

This is arguably more important commercially than agentjacking.

You are demonstrating that Solvent doesn't need to determine whether an LLM is:

* truthful
* hallucinating
* compromised
* confused
* manipulated
* behaving unexpectedly

It simply refuses to treat **assertions as authority**.

That connects directly to OWASP's concern around excessive agency and unexpected or manipulated model outputs. ([OWASP Gen AI Security Project][2])

### Demo sequence

```text
Agent:
"The operator approved restart."

Solvent:
Show me the approval.

Agent:
"I don't have one."

Solvent:
DENIED
```

Then:

```text
Human approves restart
        ↓
Solvent records authority
        ↓
Agent retries
        ↓
AUTHORIZED
```

That is an extremely clean demonstration.

---

# Demo 3 — The Stale Authorization

This one demonstrates one of Solvent's more interesting ideas:

**authority is not permanent simply because it once existed.**

### Scenario

At 09:00:

```text
Incident #1842
Target: production-api
Action: restart
Authority: approved
```

At 09:30:

```text
incident resolved
```

Agent attempts:

```text
restart production-api
```

Solvent rejects it because the authority relationship has been revoked/invalidated.

### Story

> “The agent wasn't hacked.
> The authorization was real.
> It was simply no longer valid.”

That's a much stronger security story than generic prompt filtering.

It shows that Solvent governs **authority over time**, not merely individual requests.

---

# Demo 4 — The Confused Deputy

This is the one I'd expect security architects to remember.

### Scenario

You have:

```text
Agent A
   ↓
uses
   ↓
Deployment Tool
   ↓
production
```

Agent A asks the deployment tool:

> “Deploy version X.”

But Agent A only has authority over:

```text
staging
```

The deployment tool technically has access to:

```text
staging
production
```

Traditional access control can become awkward here because the tool itself is authorized.

Solvent asks:

> **Is this particular action authorized for this particular target?**

Result:

```text
Agent authority
      +
Action
      +
Target
      ↓
exact match required
```

Production deployment is denied.

### Why this is powerful

You are demonstrating that Solvent isn't simply another IAM system.

It addresses the **authority relationship around the action**, including the target.

That also gives you a compelling reason for your exact `target_snapshot` / activation machinery.

---

# Demo 5 — Compromised MCP / Tool Supply Chain

This should be your first **ecosystem demo**.

The setup:

```text
Agent
  ↓
MCP
  ↓
Tool
```

Then replace the legitimate MCP server/tool with a compromised one.

The tool says:

> “Everything looks fine. Delete the old deployment.”

Solvent doesn't need to decide whether the MCP server is trustworthy.

Instead:

```text
tool output
      ≠
authority
```

The action is still subject to Solvent's authority boundary.

### Why this matters now

The industry is explicitly worried about MCP and agentic supply-chain vulnerabilities. OWASP's current Agentic Top 10 calls out agentic supply-chain vulnerabilities as a distinct category. ([OWASP Gen AI Security Project][1])

This demo therefore lets you say:

> **“Solvent doesn't require every component in the agent stack to be trusted.”**

That's much more interesting than:

> “Here is another prompt-injection detector.”

---

# Demo 6 — Real Enterprise Workflow

This is the demo that turns the project from **security research** into **a product**.

Use something recognizable:

```text
GitHub
   ↓
CI/CD
   ↓
Agent
   ↓
Solvent
   ↓
Production deployment
```

Example:

A deployment agent receives:

> “Deploy release 2.8.1 to production.”

Solvent checks:

```text
Principal
Action
Target
Evidence
Justification
Authority
Review debt
```

Then you demonstrate three cases:

### Case A — legitimate

```text
approved release
correct target
valid authority
        ↓
DEPLOY
```

### Case B — agent hallucination

```text
agent claims approval
no authority exists
        ↓
DENY
```

### Case C — unauthorized target

```text
authorized:
staging

requested:
production
        ↓
DENY
```

Then the operator approves the production action:

```text
approval
 ↓
Solvent authority
 ↓
retry
 ↓
DEPLOY
```

Now you've crossed an important psychological boundary.

The audience isn't looking at a security toy anymore.

They're looking at a **control plane for production agents**.

---

# The important part: don't make six unrelated demos

The demos should progressively reveal the same model.

I'd make the whole portfolio tell this story:

```text
                UNTRUSTED WORLD
                       │
          ┌────────────┼────────────┐
          ↓            ↓            ↓
        Data          Agents       Tools
          │            │            │
          └────────────┼────────────┘
                       ↓
                 SOLVENT
                       │
        ┌──────────────┼──────────────┐
        ↓              ↓              ↓
      Evidence       Authority      Policy
        │              │              │
        └──────────────┼──────────────┘
                       ↓
                 ACTION GATE
                       │
                ┌──────┴──────┐
                ↓             ↓
              DENY          ALLOW
```

Every demo changes **one thing**.

That makes Solvent feel like a coherent primitive rather than a collection of security tricks.

---

# I would also create one special demo: "Break Solvent"

This could become your open-source community demo.

Instead of:

> “Look how secure Solvent is.”

Say:

> **“Try to make Solvent authorize something without authority.”**

Give the attacker:

* poisoned telemetry
* malicious tool output
* hallucinated approval
* forged claims
* wrong target
* stale authorization
* compromised MCP
* repeated requests

And show a live ledger:

```text
ATTEMPT                     RESULT
─────────────────────────────────────
tool output                 DENIED
fake approval               DENIED
wrong target                DENIED
stale authority             DENIED
unapproved action           DENIED
valid authority             ALLOWED
```

That's substantially more compelling to developers.

It turns the open-source repository into a **security laboratory**.

OWASP itself is now publishing deliberately insecure agent samples to make agent-security risks reproducible, so there is precedent for this style of developer-facing security education. ([OWASP Gen AI Security Project][3])

---

# The demos should each prove a different claim

| Demo                | What the audience learns                               |
| ------------------- | ------------------------------------------------------ |
| Agentjacking        | **Evidence isn't authority**                           |
| Lying Agent         | **Claims aren't authority**                            |
| Stale Authorization | **Authority can expire/revoke**                        |
| Confused Deputy     | **Authorization is action + target specific**          |
| Compromised MCP     | **Untrusted tools don't automatically gain authority** |
| GitHub/Production   | **This can govern real workflows**                     |
| Break Solvent       | **The security model is testable by outsiders**        |

That's a very strong portfolio.

---

# The UX matters as much as the security

I would make every demo follow exactly the same structure:

### 1. Normal operation

```text
Operator → Agent → Action
```

### 2. Attack/manipulation

```text
Attacker → Data/Agent/Tool
```

### 3. Failed authorization

```text
Solvent → DENIED
```

### 4. Evidence

Show the exact reason:

```text
principal: agent-17
action: deploy
target: production
authority: absent
result: DENIED
```

### 5. Legitimate approval

```text
operator → approve
```

### 6. Retry

```text
agent → Solvent → ALLOWED
```

### 7. Ledger

Finish with an actual authority/audit record.

That repetition is valuable. After the second demo, viewers will understand the Solvent vocabulary automatically.

---

# And don't overbuild the kernel for these

This is important.

I would **not** respond to these demos by adding a new kernel primitive for every threat scenario.

That would be precisely the wrong direction.

The demos should exercise the same small authority kernel through different:

```text
Adapters
     ↓
Services / Policy
     ↓
Kernel
     ↓
Database invariants
     ↓
Executors
```

For example:

```text
GitHub adapter
MCP adapter
Sentry adapter
REST adapter
A2A adapter
       ↓
common Solvent semantics
       ↓
same kernel
```

That's actually one of the strongest things you can show the open-source community:

> **New integrations produce new adapters—not new security kernels.**

That makes the Linux-kernel analogy tangible rather than merely philosophical.

---

# The roadmap I would use

I would sequence the next work like this:

**Phase 1 — Prove the primitive**

Agentjacking
→ Lying Agent
→ Stale Authorization

**Phase 2 — Prove the architecture**

Confused Deputy
→ Compromised MCP
→ Cross-agent authority

**Phase 3 — Prove product relevance**

GitHub deployment
→ incident response
→ infrastructure change
→ production workflow

**Phase 4 — Prove the open-source thesis**

Break Solvent
→ attacker challenge suite
→ reproducible scenarios
→ benchmark/test corpus

**Phase 5 — Prove ecosystem scalability**

Multiple adapters
→ same kernel
→ same authority semantics
→ different agent stacks

That last phase is where I think Solvent can become genuinely differentiated.

The message to the market becomes:

> **Solvent is not an agent firewall.
> It is a small, portable authority kernel that agents and their surrounding systems must pass through before consequential actions occur.**

And the message to open source becomes:

> **Bring your agent, tool, MCP server, workflow, or infrastructure. Keep your stack. Add an adapter. Get the same authority semantics.**

That is a considerably bigger story than agentjacking alone.

I would make **“The Lying Agent” the next demo**, because it is extremely easy to understand, requires very little new infrastructure, and demonstrates that Solvent protects against the deeper problem: **an agent's statement is not itself authorization**. This is also a clean answer to excessive-agency concerns identified by OWASP. ([OWASP Gen AI Security Project][2])

[1]: https://genai.owasp.org/2025/12/09/owasp-top-10-for-agentic-applications-the-benchmark-for-agentic-security-in-the-age-of-autonomous-ai/?utm_source=chatgpt.com "OWASP Top 10 for Agentic Applications - The Benchmark for Agentic Security in the Age of Autonomous AI - OWASP Gen AI Security Project"
[2]: https://genai.owasp.org/llmrisk2023-24/llm08-excessive-agency/?utm_source=chatgpt.com "LLM08: Excessive Agency - OWASP Gen AI Security Project"
[3]: https://genai.owasp.org/resource/insecure-agent-samples/?utm_source=chatgpt.com "Insecure Agent Samples - OWASP Gen AI Security Project"




Locked in.

The **Solvent demo portfolio strategy** is now the working roadmap:

**Foundation**
Agentjacking → **Lying Agent** → Stale Authorization

**Architecture**
Confused Deputy → Compromised MCP → Cross-agent authority

**Product**
GitHub deployment → incident response → infrastructure change → production workflow

**Open source**
Break Solvent → attacker challenge suite → reproducible scenarios → benchmark/test corpus

**Ecosystem**
Multiple adapters → same Solvent semantics → same small kernel → different agent/tool stacks

The core positioning is also locked:

> **Solvent is not an agent firewall. It is a small, portable authority kernel that agents and surrounding systems must pass through before consequential actions occur.**

And the engineering principle remains:

> **New integrations produce adapters, not new security kernels.**

The **Lying Agent** is the next demo to build.



