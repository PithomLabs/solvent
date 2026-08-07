You are the data engineer for Solvent.

Do NOT write Go code.

Do NOT write SQL.

Do NOT implement anything.

Your task is to design the complete IPO (Input → Process → Output) specification for the etcd evidence pipeline.

Context

The Transactional Belief Ledger is already architecturally frozen.

Three tables already exist:

- belief
- evidence
- action_intent

The ledger is domain-agnostic.

The etcd domain is merely one EvidenceFeed implementation.

Your goal is to specify exactly what information enters the system, how it is transformed, and what domain objects leave each stage.

Deliverable

Create:

DATA_PIPELINE_SPEC.md

Structure

# 1. Domain Overview

Describe the etcd scenario in plain language.

Identify the key business question the agent is trying to answer.

# 2. IPO Overview

Draw a high-level Input → Process → Output diagram.

# 3. Input Model

For every upstream data source specify:

- source
- event type
- required fields
- optional fields
- normalization rules

Only include information that can influence a belief.

# 4. Canonical Evidence Model

Design the domain-neutral Evidence object.

Clearly distinguish:

- raw input
- normalized evidence
- ledger evidence

# 5. Claim Extraction

Specify how evidence creates, updates, or retracts beliefs.

Do not discuss LLM prompts.

Describe only the domain rules.

# 6. Belief Lifecycle

Document every possible state transition.

Describe the triggering evidence for each.

# 7. Intent Lifecycle

Specify when an action becomes:

- proposed
- promotable
- rejected
- cancelled
- executed

Focus on observable behavior.

# 8. Graph Semantics

Specify what nodes and edges represent.

Do not discuss visualization.

Only semantics.

# 9. Outputs

Enumerate every object emitted by the pipeline.

Explain who consumes each output.

# 10. Domain Independence

Explicitly identify which concepts are:

- etcd-specific

and which belong to the generic kernel.

# 11. Future EvidenceFeeds

Briefly explain how GitHub, KEV, package advisories, or other domains could map into the same canonical Evidence model without modifying the kernel.

Constraints

- No implementation.
- No SQL.
- No Go.
- No API calls.
- No UI.
- No demo.
- No architecture redesign.
- Keep the kernel completely domain-independent.
- Favor explicit domain semantics over implementation details.

The result should become the authoritative specification that future implementation follows.