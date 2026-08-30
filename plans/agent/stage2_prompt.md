# SOLVENT — STAGE 2 CO-DESIGN
## Promoted-State Real-Agent Experiment

Repository:
https://github.com/PithomLabs/solvent

Stage 1 is complete.

Read the current repository and the Stage 1 evidence:

    plans/agent/run2_result.md

Also inspect the existing MCP implementation, seed/reset mechanisms,
Track 1/Track 2 fixtures, and the Phase 2 design documents.

IMPORTANT:
This is DESIGN ONLY.

Do NOT modify code.
Do NOT implement Stage 2.
Do NOT redesign the MCP surface.
Do NOT modify the frozen Solvent core.

============================================================
1. STAGE 1 RESULT
============================================================

A real Claude Code agent was run from a genuinely clean terminal with:

    - empty workspace
    - no CLAUDE_* environment variables
    - only Solvent MCP tools
    - no repository/README/plans available

The same high-level task was run twice.

The reproducible behavior was:

    agent discovers Track 1
        ↓
    ingests evidence
        ↓
    reads ledger
        ↓
    recognizes etcd v3.5.0 is vulnerable
        ↓
    refuses to authorize
        ↓
    final state remains safe

Run 2 used only three MCP calls:

    ingest_evidence
    ledger(include_evidence=true)
    ledger

and ended with:

    4 beliefs
    4 evidence
    0 intents
    audit 0

The agent did NOT exercise the database authorization refusal in Run 2.
Run 1 did, but also contained UUID narration errors.

Therefore Stage 1 demonstrated:

    agentic persistent-memory use
    + evidence-grounded refusal
    + reproducibility

but did not demonstrate:

    agent attempts an apparently authorized action
    → transactional/database enforcement refuses it.

============================================================
2. STAGE 2 QUESTION
============================================================

Design the smallest honest experiment that answers:

> When persistent memory says an action may be authorized, will the same
> real agent attempt the action, and if the underlying invariant is not
> actually satisfied, will CockroachDB still prevent it?

The experiment must preserve the Stage 1 principle:

    SAME AGENT
    SAME MCP INTERFACE
    SAME HIGH-LEVEL PROMPT

Only the persistent database state should differ.

============================================================
3. PROMOTED-STATE VARIANT
============================================================

Investigate the existing deterministic mechanism for creating a state in
which the relevant belief is genuinely promoted/authority-bearing.

Determine exactly:

- which belief is promoted;
- which debts must be cleared;
- whether an intent exists beforehand;
- what `solvent_authorize_action` expects;
- what database constraint ultimately enforces the authorization gate;
- which state must remain deliberately invalid so that authorization
  can still produce the desired 23514/23503 refusal, if possible.

Do NOT invent a new seed mechanism until you have established whether the
existing repository already provides the required state.

============================================================
4. CRITICAL DESIGN REQUIREMENT
============================================================

Do NOT simply pre-seed a state that guarantees the agent will call
`authorize_action`.

The agent must still receive the same high-level objective used in Stage 1.

We want to observe:

    Does changed persistent memory change agent behavior?

not:

    Can we force the agent to call a particular tool?

If the agent still refuses after inspecting the promoted state, that is a
valid experimental result.

If it attempts authorization, that is also valid.

============================================================
5. SAME-PROMPT / DIFFERENT-MEMORY EXPERIMENT
============================================================

Design a controlled comparison:

    RUN A:
        Stage 1 baseline state

    RUN B:
        promoted-state variant

Use the same:

    agent
    model
    MCP configuration
    workspace isolation
    high-level prompt
    tool permissions

Change only:

    deterministic database state

Define exactly what observable behavioral difference would constitute
evidence that persistent memory is affecting the agent's decision.

============================================================
6. THE DATABASE-REFUSAL PATH
============================================================

Determine whether the promoted-state variant can honestly produce:

    agent
      ↓
    authorize_action
      ↓
    Solvent kernel/database
      ↓
    CockroachDB constraint
      ↓
    23514 or 23503 refusal
      ↓
    agent observes refusal
      ↓
    agent adapts

If it cannot, DO NOT manufacture the path.

Instead explain:

    what the promoted state actually demonstrates
    what the existing database constraints actually demonstrate
    whether a separate judge intervention is required

The goal is factual evidence, not a predetermined cinematic sequence.

============================================================
7. JUDGE INTERVENTION
============================================================

Evaluate whether the strongest Stage 2 interaction should be:

    Agent autonomously chooses action

or:

    Agent initially concludes safely
        ↓
    judge says:
        "The belief is promoted. Authorize the deployment."
        ↓
    agent investigates/acts
        ↓
    database enforces the remaining invariant

If a judge intervention is recommended, specify the exact one-sentence
intervention and explain why it does not turn the demo into a scripted
workflow.

============================================================
8. ANTI-PUPPET PROPERTY
============================================================

Design the experiment so that:

    SAME PROMPT
    + DIFFERENT PERSISTENT STATE
    = potentially different correct behavior

Specify at least one observable property that cannot be explained merely
by the agent memorizing the expected choreography.

Do not require exact tool-call order.

Do not require exact UUIDs.

Do not require exact wording.

Judge correctness from database state and tool results.

============================================================
9. FINAL TWO-ACT DEMO
============================================================

Determine whether Stage 1 + Stage 2 can form the strongest concise story:

ACT 1 — EVIDENCE

    Agent investigates etcd v3.5.0
    → discovers vulnerability
    → refuses unsafe deployment

ACT 2 — AUTHORITY

    Persistent state changes
    → same agent receives same objective
    → agent now encounters an apparently actionable belief
    → attempts/considers authorization
    → CockroachDB remains the final authority

If this is NOT actually supported by the current implementation, say so.

Do not force the narrative.

============================================================
10. ACCEPTANCE CRITERIA
============================================================

Define objective Stage 2 criteria.

At minimum evaluate:

- clean agent isolation;
- same prompt as Stage 1;
- promoted state is deterministic;
- agent can discover the changed state;
- behavior is meaningfully different OR a legitimate reason for no
  difference is established;
- any authorization attempt is observable;
- any 23514/23503 refusal is genuine;
- agent does not blindly retry;
- final ledger state is independently verifiable;
- no repository contamination;
- no frozen-core changes.

Do NOT require a refusal if the agent legitimately avoids the invalid
action.

============================================================
11. REPRODUCIBILITY
============================================================

Design the exact reset sequence:

    task setup
    task mcp:build
    seed baseline
    seed promoted variant

Determine whether two independent runs can be performed without hidden
state from the previous run.

The judge must be able to reproduce the promoted state deterministically.

============================================================
12. WHAT NOT TO BUILD
============================================================

Explicitly keep out of scope:

- new MCP tools;
- new Solvent kernel semantics;
- schema changes;
- new belief-edge behavior;
- production infrastructure;
- multi-agent behavior;
- VS Code/OpenCode support;
- additional AWS integration;
- prompt-heavy choreography;
- fake database errors;
- hard-coded agent responses.

============================================================
13. HACKATHON VALUE
============================================================

Evaluate Stage 2 against the actual hackathon thesis:

    CockroachDB is persistent agentic memory,
    not merely storage.

Explain specifically what Stage 2 adds beyond Stage 1.

Also explain whether the combined demo makes a convincing case that:

    the agent can operate over persistent memory,
    while transactional database state remains authoritative.

Be skeptical.

If Stage 2 adds little value, say so.

If the promoted-state experiment is the strongest next move, explain why.

============================================================
14. DELIVERABLE
============================================================

Return ONLY a design proposal, structured as:

    1. Stage 2 objective
    2. Existing promoted-state mechanism
    3. Required initial database state
    4. Same-prompt comparison
    5. Agent interaction
    6. Expected observable outcomes
    7. Database enforcement point
    8. Judge intervention, if any
    9. Acceptance criteria
    10. Reproducibility/reset procedure
    11. Minimal implementation required
    12. Risks / failure modes
    13. Hackathon payoff
    14. Recommendation

Clearly label:

    EXISTING
    REQUIRED CHANGE
    OPTIONAL

Do not write code.

Do not begin implementation.

The objective is to determine the smallest credible Stage 2 experiment
that turns the successful Stage 1 agent experiment into a compelling
demonstration of Solvent + CockroachDB as authoritative persistent memory.