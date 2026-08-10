## prompt


now before we proceed to designing the mcp phase, I want you to know that I will have Claude co-design this phase for balance and another POV, having said that, I want you to give the prompt for Claude for the rationale for this phase as background context, what are the advantages, challenges and limitations that we face in order to win this hackathon



# SOLVENT — MCP PHASE CO-DESIGN: STRATEGIC BACKGROUND

You are now co-designing the next phase of Solvent with another AI
architect.

This is NOT yet an implementation request.

Your job in this turn is to understand the strategic rationale, advantages,
risks, constraints, and hackathon-winning objective before proposing an MCP
architecture.

Repository:

    https://github.com/PithomLabs/solvent

============================================================
CURRENT STATE — IMPORTANT
============================================================

The Solvent core and CLI/Taskfile playground are now effectively frozen.

The project has gone through multiple rounds of implementation and
adversarial review.

The current verified playground provides:

    clone
      ↓
    task setup
      ↓
    disposable CockroachDB
      ↓
    Track 1
      ↓
    replay/idempotence
      ↓
    Track 2
      ↓
    inspect

The core behavior demonstrated is:

TRACK 1

    real etcd evidence
        ↓
    typed beliefs
        ↓
    debt/review
        ↓
    promotion
        ↓
    live action intent

REPLAY

    same evidence again
        ↓
    deterministic deduplication
        ↓
    no duplicate semantic ledger state

TRACK 2

    historical/postulated belief
        ↓
    review
        ↓
    promotion
        ↓
    live intent
        ↓
    new postmortem evidence
        ↓
    falsification
        ↓
    belief retracted
        ↓
    dependent live intent cancelled

IMPORTANT CURRENT LIMITATION:

The MVP does NOT demonstrate belief_edge-based multi-belief propagation.

The honest claim is:

    single-belief retraction + cancellation of its dependent live intent

Do not redesign this during MCP co-design.

============================================================
WHY WE ARE CONSIDERING MCP
============================================================

The next phase is to expose Solvent through an MCP server so that ordinary
MCP-capable coding/agent clients can interact with the existing Solvent
ledger.

Potential clients include:

    Claude
    Codex
    VS Code
    OpenCode
    other MCP-capable agents

The conceptual architecture is:

                 MCP CLIENTS
       ┌──────────┬──────────┬──────────┐
       │ Claude   │ Codex    │ VS Code  │
       │ OpenCode │          │          │
       └────┬─────┴────┬─────┴────┬─────┘
            │          │          │
            └──────────┼──────────┘
                       │ MCP
                       ▼
              ┌─────────────────┐
              │ Solvent MCP     │
              │ Server          │
              ├─────────────────┤
              │ beliefs         │
              │ evidence        │
              │ debt/review     │
              │ promotion       │
              │ intents         │
              │ replay          │
              │ falsification   │
              └────────┬────────┘
                       │
                       ▼
                 Solvent core
                       │
                       ▼
                  CockroachDB

This architecture is NOT yet approved.

We want to determine whether it is actually the right next move.

============================================================
THE STRATEGIC QUESTION
============================================================

The question is NOT:

    "Can we build an MCP server?"

Of course we can.

The question is:

    "Does an MCP interface make Solvent substantially more compelling
     to hackathon judges, and if so, what is the smallest MCP experience
     that demonstrates the unique value of Solvent?"

We have limited time.

We do NOT want:

    technology for technology's sake
    a generic CRUD MCP server
    a second UI
    a new application architecture
    a second implementation of Solvent semantics
    unnecessary agent orchestration
    a large frontend
    RAG/vector database work
    unnecessary infrastructure

The MCP phase must sit ON TOP of the existing frozen Solvent core.

============================================================
WHY MCP COULD BE A STRONG ADVANTAGE
============================================================

Explore these hypotheses rather than assuming they are true.

### 1. Agent-native interaction

The core Solvent idea is about maintaining transactional belief state
rather than relying on ephemeral agent context.

MCP could let an actual AI coding agent interact with that state directly.

Instead of:

    "Here is a demo application."

we could demonstrate:

    "Here is infrastructure an AI agent can use to maintain and act on
     durable, auditable beliefs."

That may make the project's purpose much more obvious.

### 2. No custom UI required

A judge already using Claude/Codex/VS Code/OpenCode could interact with
Solvent through their existing agent.

Potential experience:

    Judge:
        "What do we currently believe about etcd?"

    Agent:
        queries Solvent

    Judge:
        "What evidence supports that belief?"

    Agent:
        retrieves evidence

    Judge:
        "Is there unresolved debt?"

    Agent:
        retrieves debt

    Judge:
        "Promote it."

    Agent:
        invokes the appropriate Solvent operation

    Judge:
        "Now process the postmortem. What happens?"

    Agent:
        invokes falsification

    Solvent:
        retracts belief
        cancels dependent intent

The judge watches the underlying transactional state change.

Assess whether this is genuinely compelling or merely flashy.

### 3. Client interoperability

A major potential story is:

    Claude ─┐
    Codex ──┤
    VS Code ├── MCP → Solvent
    OpenCode┘

The client is replaceable.

The belief/decision state is not trapped inside one model's context window.

Assess whether this interoperability is actually valuable enough to
highlight in a hackathon demo.

### 4. Separation of intelligence from state/invariants

Potential architectural story:

    Agent = reasoning / proposal / interaction
    Solvent = durable belief state + transactional invariants

This could be stronger than simply saying:

    "We added MCP."

Assess how to communicate this distinction.

============================================================
WHY MCP COULD ALSO HURT US
============================================================

Be adversarial.

### 1. Scope explosion

An MCP server can easily become:

    authentication
    transport
    tool design
    agent prompting
    tool discovery
    error handling
    client-specific configuration
    deployment
    documentation

We have a hackathon deadline.

Identify the minimum viable MCP surface.

### 2. Generic MCP problem

A judge could reasonably ask:

    "Why isn't this just another MCP server exposing CRUD?"

We need a strong answer.

The MCP tools must expose Solvent's domain semantics rather than raw SQL.

For example, potentially:

    promote belief
    falsify belief
    inspect evidence
    inspect debt
    inspect intent

rather than:

    execute SQL
    insert belief
    update intent

Assess this distinction carefully.

### 3. Agent hallucination / misuse

The agent is probabilistic.

Solvent is supposed to provide transactional guarantees.

We need to understand the boundary:

    What can the agent decide?
    What must Solvent enforce?
    What should be impossible through MCP?
    What operations require explicit human/operator approval?

Do not assume MCP makes an agent safe.

### 4. Demo fragility

A judge may have:

    Claude but not Codex
    Codex but not Claude
    VS Code but no configured MCP
    no local Docker
    different MCP configuration
    different model behavior

The existing CLI playground must remain the fallback.

Evaluate how much MCP setup is acceptable.

### 5. MCP could obscure the actual innovation

If the judge spends the entire demo watching:

    "agent calls tool"

they may miss:

    belief
    evidence
    debt
    promotion
    intent
    retraction

The MCP layer must amplify Solvent's semantics rather than become the
story itself.

============================================================
HACKATHON OBJECTIVE
============================================================

We are not trying to build a production platform in this phase.

The objective is to maximize:

    judge comprehension
    technical distinctiveness
    credibility
    demo memorability
    implementation reliability

under a hard deadline.

The strongest demo should probably be something like:

    Agent asks Solvent about a real operational/security decision
        ↓
    Solvent returns evidence-backed belief state
        ↓
    agent discovers debt / uncertainty
        ↓
    operator-approved promotion
        ↓
    action intent becomes live
        ↓
    new evidence arrives
        ↓
    agent asks what changed
        ↓
    Solvent retracts stale belief
        ↓
    live action is cancelled
        ↓
    agent explains the causal chain

But this is only a hypothesis.

Challenge it.

============================================================
WHAT IS ALREADY SOLVED
============================================================

Do NOT spend MCP-phase design effort on:

- real etcd research
- fixture authenticity
- CockroachDB schema redesign
- replay semantics
- belief_edge
- debt model redesign
- kernel redesign
- custom frontend
- RAG
- embeddings
- vector database
- Kubernetes
- Kafka

Those are outside this phase.

The current CLI/Taskfile playground is the deterministic reference
implementation and fallback.

============================================================
KNOWN TECHNICAL CONSTRAINTS
============================================================

The current system has:

- Go implementation
- CockroachDB
- transactional kernel
- beliefs
- evidence
- debt
- promotion
- action intents
- falsification/retraction
- real etcd fixtures
- deterministic replay

The MCP layer should NOT bypass the existing kernel.

For example, an MCP client should NOT be able to do:

    UPDATE action_intent SET state='cancelled'

Instead, it should invoke the legitimate domain operation that causes
Solvent to enforce the correct invariants.

The MCP server should therefore be an adapter over existing semantics,
not a parallel business-logic implementation.

============================================================
IMPORTANT KNOWN LIMITATION
============================================================

The current evidence persistence path does not yet thread:

    Normalized.ObservedAt
        ↓
    evidence.source_observed_at

The fixtures contain real source dates and normalization parses them, but
the frozen kernel does not currently persist that value.

This is a known deferred capability.

Do NOT "fix" it during this design exercise.

Consider whether and when that matters to the MCP experience.

============================================================
CO-DESIGN QUESTIONS
============================================================

Before proposing implementation, answer these strategically:

1. Is MCP actually the highest-value next move for winning the hackathon?

2. What is the single most compelling MCP demo?

3. What should the MCP server expose?

4. What should it explicitly NOT expose?

5. Which Solvent invariants must remain entirely server/kernel-controlled?

6. What should require operator/human approval?

7. How do we prevent the MCP server from becoming generic CRUD?

8. What is the smallest viable tool surface?

9. What is the minimum viable client configuration?

10. Should we optimize for ONE excellent client demonstration or
    interoperability across several clients?

11. How do we preserve the CLI playground as a zero-risk fallback?

12. What failure modes could make the MCP demo embarrassing or fragile?

13. What should happen if the agent makes a bad request?

14. What should the MCP tool responses look like so the agent can explain
    Solvent's state accurately?

15. What is the strongest 3–5 minute judge narrative?

16. What should we deliberately NOT build before the deadline?

17. What would make a judge say:

       "This is more than an MCP wrapper."

18. What would make a skeptical judge say:

       "This is just CRUD exposed through MCP."

19. How can the demo visibly prove that Solvent, rather than the model,
    enforces the important invariants?

20. What is the minimum implementation that creates a credible
    "agent-native transactional belief ledger" demonstration?

============================================================
IMPORTANT DESIGN PRINCIPLE
============================================================

Treat this as a hackathon optimization problem.

For every proposed feature ask:

    Does this increase judge-visible value enough to justify its
    implementation and demo risk?

Prefer:

    one excellent MCP workflow

over:

    ten mediocre tools.

Prefer:

    existing Solvent semantics exposed cleanly

over:

    new MCP-specific business logic.

Prefer:

    deterministic CLI fallback

over:

    MCP becoming a single point of demo failure.

============================================================
YOUR OUTPUT
============================================================

Do NOT write implementation code yet.

Produce a strategic co-design review with:

# 1. Strategic Verdict

Should we build the MCP phase?

Why?

# 2. Winning Argument

What is the strongest reason MCP makes Solvent more compelling?

# 3. Biggest Risks

Rank the top risks by:

    impact × probability

# 4. Recommended MCP Boundary

What belongs:

    Agent
    MCP server
    Solvent kernel
    CockroachDB

# 5. Minimum Tool Surface

Propose the smallest useful set of MCP tools.

For every tool explain:

    purpose
    why it exists
    what it cannot do
    invariant it relies on

# 6. Killer Demo

Design the strongest 3–5 minute judge-facing interaction.

Include the actual conversational sequence between:

    judge
    agent
    MCP
    Solvent

# 7. Anti-CRUD Test

Explain why this is NOT merely CRUD over MCP.

# 8. Failure/Demo Resilience

Explain what happens if:

    MCP unavailable
    client unavailable
    agent makes invalid request
    database unavailable
    tool call fails
    evidence is contradictory

# 9. What NOT To Build

Be ruthless.

List features that should be rejected before the deadline.

# 10. Implementation Phases

Propose:

    Phase A — minimum MCP
    Phase B — demo polish
    Phase C — optional stretch

Only include Phase C if it is genuinely worthwhile.

# 11. Hackathon Winning Assessment

Score the MCP phase from 1–10 on:

    novelty
    judge comprehension
    technical depth
    demo memorability
    implementation risk
    reliability

Then give an overall recommendation.

Do not assume MCP is automatically good.

If you believe the MCP phase is the wrong move, say so clearly and explain
what should replace it.

============================================================
FINAL INSTRUCTION
============================================================

You are the SECOND ARCHITECT here.

Do not merely validate the existing direction.

Challenge assumptions, identify where the MCP idea could become
technology theater, and propose a design that maximizes Solvent's chance
of winning the hackathon under the remaining deadline.

The existing Solvent core is frozen.

Your job is to help decide what the smallest, strongest agent-native layer
on top of that core should be.