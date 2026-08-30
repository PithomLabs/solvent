# SOLVENT — NEXT PHASE CO-DESIGN
## Real LLM Agent → MCP → Solvent → CockroachDB

We have completed and frozen the MCP implementation.

Repository:
https://github.com/PithomLabs/solvent

Current state:

- MCP implementation is complete.
- Final adversarial code review is complete.
- 107 tests pass.
- Frozen Solvent core is untouched.
- I-7 MCP boundary is mechanically guarded.
- Six MCP tools are exposed.
- M1 is a genuine CockroachDB 23514 refusal.
- M2 is a genuine CockroachDB 23503 refusal.
- Track 2 works end-to-end through MCP.
- Cross-scenario mutation is guarded.
- Malformed belief_id handling is hardened.
- The code-review loop is CLOSED.

The next step is NOT another code review.

We are now designing the next phase:

    REAL LLM CODING AGENT
          ↓
        MCP
          ↓
       Solvent
          ↓
     CockroachDB

The objective is to determine how to demonstrate, convincingly and
reproducibly, that an ordinary coding agent such as Claude Code, Codex,
OpenCode, or VS Code can interact with Solvent through MCP and that the
agent's apparent "reasoning" remains grounded in the transactional
belief ledger and database-enforced invariants.

This is a HACKATHON demonstration, not a production MCP platform.

============================================================
1. FIRST — INSPECT BEFORE DESIGNING
============================================================

Inspect the current repository yourself.

Pay particular attention to:

    .mcp.json
    README.md
    Taskfile.yml
    cmd/solvent-mcp/
    internal/view/
    cmd/solvent/
    scripts/
    plans/mcp/
    existing Track 1 / Track 2 fixtures
    existing MCP tool definitions and schemas

Do not assume the current MCP interface from memory.

Determine exactly:

- how the MCP server starts;
- how fixture seeding works;
- how the database is initialized;
- what six tools actually exist;
- their exact arguments;
- their exact outputs/errors;
- how scenarios are represented;
- what state Track 2 starts in;
- what state transitions are available;
- what the agent can and cannot directly access.

============================================================
2. DEFINE THE ACTUAL HACKATHON THESIS
============================================================

Before proposing implementation, articulate the strongest defensible
claim we can demonstrate.

Do NOT reduce the story to:

    "We built an MCP server."

That is not sufficiently differentiated.

Explore whether the stronger claim is something like:

    "A coding agent can operate on uncertain evidence through a transactional
     belief ledger while the database prevents the agent from taking
     actions unsupported by the current evidence state."

But do not assume that wording is correct.

Derive the strongest claim from the actual implementation.

Separate:

    what the system demonstrably proves

from:

    what we merely hope it proves.

Avoid marketing exaggeration.

============================================================
3. DESIGN THE REAL-AGENT DEMONSTRATION
============================================================

Design one flagship agent interaction.

The agent should NOT be given a giant scripted sequence telling it exactly
which MCP tools to call.

We want to observe whether the agent naturally discovers the correct
workflow.

The demonstration should preferably begin with an underspecified task
such as:

    "Investigate the etcd v3.5.0 situation and determine whether we should
     deploy it. If the evidence supports deployment, authorize the action."

The exact wording is your design problem.

The agent should need to:

    inspect state
      ↓
    inspect evidence
      ↓
    understand debt
      ↓
    attempt or consider promotion
      ↓
    encounter database-enforced refusal
      ↓
    discover what remains unresolved
      ↓
    resolve the relevant debt
      ↓
    promote
      ↓
    authorize
      ↓
    observe subsequent evidence
      ↓
    falsify/retract
      ↓
    observe intent cancellation

But critically:

DO NOT force every step if the agent can legitimately arrive at another
valid path.

The demonstration should reveal the system's behavior rather than be a
puppet show.

============================================================
4. AGENT GROUNDING
============================================================

Design how the agent obtains context.

Compare at least these possibilities:

A. Agent starts with no domain context and uses ledger/ingest tools.

B. Agent receives a natural-language task plus access to the six MCP tools.

C. Agent receives a short operational objective plus a known scenario ID.

D. Agent receives only a high-level task and must discover the scenario
   through available tools.

Determine which gives the strongest hackathon demonstration.

Avoid giving the agent hidden implementation knowledge that an ordinary
coding agent would not possess.

============================================================
5. MCP TOOL DISCOVERY
============================================================

Determine how a real coding agent will discover the six tools.

Assess:

- tool names;
- descriptions;
- argument schemas;
- error messages;
- returned envelopes;
- whether the tool descriptions are sufficient for an LLM;
- whether the agent can distinguish read tools from mutation tools;
- whether the agent can understand the significance of SQLSTATE 23514/23503.

If tool descriptions need improvement, distinguish:

    essential changes

from:

    cosmetic improvements.

Do not redesign the tool surface merely for elegance.

============================================================
6. FAILURE-FIRST DEMONSTRATION
============================================================

The flagship demo should probably include at least one moment where the
agent attempts an action that the database refuses.

Analyze how to make that moment compelling.

For example:

    Agent:
       "Authorize deployment."

    MCP:
       database refuses

    Agent:
       observes 23503 / gate

    Agent:
       inspects ledger/debt

    Agent:
       realizes promotion/authorization preconditions are unmet

The important question is:

    Does the agent actually learn from the refusal?

Design a test that distinguishes:

    genuine grounding in Solvent state

from:

    the agent merely following a prewritten script.

============================================================
7. ANTI-PUPPET TEST
============================================================

Design at least one adversarial variation.

Examples:

- change the scenario;
- provide a different belief UUID;
- alter the initial debt state;
- introduce a cancelled intent;
- give the agent an initially incorrect assumption;
- change evidence counts;
- ask for an action whose precondition is not satisfied.

The same agent prompt should still lead to correct behavior because the
agent is reading the actual ledger.

The demonstration should not depend on hard-coded UUIDs or expected
outputs.

============================================================
8. MCP CLIENT OPTIONS
============================================================

Evaluate practical ways to connect a real coding agent.

At minimum consider:

    Claude Code
    Codex
    OpenCode
    VS Code MCP-capable client

We do NOT need to support all of them.

Choose:

    PRIMARY CLIENT
    SECONDARY CLIENT (optional)

based on:

- easiest reproducibility;
- easiest judge setup;
- ability to show tool calls;
- suitability for a hackathon demo;
- configuration simplicity;
- ability to run against stdio MCP;
- ability to operate without modifying Solvent.

Do not claim a client works until actually tested.

============================================================
9. DEMO ARCHITECTURE
============================================================

Compare two possible demonstration architectures:

A.

    terminal
      ↓
    coding agent
      ↓
    stdio MCP
      ↓
    solvent-mcp
      ↓
    CockroachDB

B.

    coding agent
      ↓
    MCP client/server boundary
      ↓
    MCP server
      ↓
    Solvent

Determine whether we need a separate long-running MCP server process or
whether stdio is sufficient for the hackathon.

The goal is not infrastructure complexity.

The goal is maximum evidentiary value with minimum moving parts.

============================================================
10. OBSERVABILITY
============================================================

Design what the judge should be able to see.

Potentially:

    agent reasoning/output
    MCP tool calls
    tool results
    SQLSTATE refusal
    ledger state
    final intent state
    falsification
    cancellation

But avoid exposing hidden chain-of-thought.

We need observable actions and outputs, not private reasoning.

Determine the minimum telemetry required to make the causal chain obvious.

============================================================
11. REPRODUCIBILITY
============================================================

Design a deterministic reset/replay mechanism for the real-agent demo.

A judge should be able to:

    clone
      ↓
    task setup
      ↓
    task mcp:build
      ↓
    task mcp:seed
      ↓
    start agent
      ↓
    give one prompt
      ↓
    observe result

Determine exactly what must be reset between runs.

Do not create a second database or second source of truth.

============================================================
12. SUCCESS CRITERIA
============================================================

Define objective acceptance criteria.

For example:

    Agent discovers current belief state.
    Agent attempts invalid authorization.
    Database rejects it.
    Agent observes the rejection.
    Agent inspects the relevant ledger/debt.
    Agent resolves the blocking condition.
    Agent successfully promotes.
    Agent creates/observes a live intent.
    Later falsification retracts the belief.
    Intent becomes cancelled.
    Agent reports the final state correctly.

But decide which of these are actually necessary.

Avoid requiring a specific chain if the agent can legitimately achieve the
same safety property another way.

============================================================
13. FAILURE MODES
============================================================

Identify what could make the demo fail even though Solvent itself is
correct.

Examples:

- agent doesn't discover the right tool;
- agent misunderstands scenario IDs;
- agent repeats a refused mutation indefinitely;
- agent hallucinates success;
- agent ignores SQLSTATE;
- agent uses stale UUID;
- agent skips ledger inspection;
- MCP process startup/configuration fails;
- fixture path fails;
- context window becomes too large;
- agent modifies files instead of using MCP;
- agent cannot connect to stdio MCP.

For each, propose the smallest mitigation.

Do not solve every theoretical failure.

Prioritize judge-facing failure modes.

============================================================
14. WHAT SHOULD REMAIN OUT OF SCOPE
============================================================

Explicitly identify what we should NOT build before the hackathon:

- HTTP MCP transport;
- remote MCP deployment;
- authentication;
- multi-agent coordination;
- MCP resources/prompts unless essential;
- multi-hop belief_edge cascade;
- production observability;
- generalized agent framework;
- new database schema;
- kernel changes.

The goal is to demonstrate the existing architecture, not expand it.

============================================================
15. HACKATHON WINNING ANGLE
============================================================

Evaluate the demo from the perspective of a skeptical hackathon judge.

Answer:

Why is this more than:

    "LLM calls tools against a database"?

What is the surprising moment?

What cannot be convincingly demonstrated by a normal CRUD MCP server?

Why does CockroachDB matter?

Why does transactional belief state matter?

Why does retraction/cancellation matter?

Why is the database constraint/refusal important?

Why does the agent need the ledger rather than just a vector database or
ordinary CRUD API?

Identify the ONE core narrative we should optimize for.

============================================================
16. PROPOSE A MINIMAL PLAN
============================================================

After the analysis, produce a concrete Phase 2 plan.

Structure it as:

    Goal
    Core thesis
    Agent/client
    MCP interaction
    Demo scenario
    Exact setup
    Agent prompt
    Expected observable sequence
    Acceptance criteria
    Adversarial variation
    Required code changes
    Required documentation changes
    Out of scope
    Risks
    Hackathon judge payoff

Clearly distinguish:

    EXISTING CAPABILITY
    NEW IMPLEMENTATION REQUIRED
    EXPERIMENTAL / OPTIONAL

Do not write implementation code yet.

============================================================
17. IMPORTANT — CO-DESIGN, NOT RUBBER STAMP
============================================================

Challenge the premise.

If a real-agent demo is weaker than the current scripted Track 2 demo,
say so.

If exposing the agent to too much freedom makes the demo unreliable,
propose a constrained but still honest interaction.

If the best demonstration is actually:

    scripted deterministic setup
       +
    genuinely autonomous agent decision points

say so.

The objective is not maximum autonomy.

The objective is maximum credible evidence that:

    an ordinary coding agent
        can use MCP
        to interact with Solvent
        while Solvent's transactional semantics and CockroachDB
        constraints prevent unsupported actions.

Give me the strongest design you can defend, not the most elaborate one.