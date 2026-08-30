# AGENTS.md

Think like a distributed systems engineer, not an LLM engineer.

## Project

**Solvent** is a transactional belief ledger for autonomous agents.

The objective is not another agent or chatbot. It is a **database-enforced memory system** where
autonomous agents cannot act on beliefs that are no longer true. The demo exists only to make the
ledger understandable.

---

# Core thesis

**Retrieval is not authority.**

```
Memory / Retrieval  →  Belief  →  Authority  →  CockroachDB invariant
```

Retrieval can be wrong. Judgment can be wrong. Authority must be structurally constrained.

Traditional agent memory stores facts. Solvent stores **beliefs**: they carry evidence, evolve as new
evidence arrives, may be promoted or retracted, and govern whether an agent may act. The
database — not the LLM — determines whether an action is allowed.

> Memory is not remembering.
>
> Memory is refusing to act on what is no longer true.

This principle takes precedence over convenience or agent autonomy.

---

# Architectural rules

- **Evidence must be attributable.** Every discharge names what justified it. A receipt that cannot
  be traced to a specific row is decoration.
- **Belief status controls authority.** An action intent may only cite a belief that is promoted
  *right now*.
- **Authority cannot survive a retracted or non-promoted belief.** This is the invariant the whole
  system exists to hold.
- **Database constraints are the final authority boundary.** Where a guarantee can live in schema or
  in application code, it lives in schema. The application cannot forget a `CHECK`.
- **Recursive belief traversal is application logic, not a DB cascade.** The walk over `belief_edge`
  is a `WITH RECURSIVE` CTE in Go. CockroachDB does not traverse the graph; it enforces that the
  traversal cannot finish having left a live intent behind. Never write that the database cascades
  through the belief graph.

The architecture is frozen. Do not redesign.

The ledger is four tables — `belief`, `belief_edge`, `evidence`, `action_intent`. The corpus and
wizard layer adds `corpus_issue`, `belief_corpus_citation`, and `refusal_log`. No further tables
without explicit approval. The kernel remains domain-agnostic: evidence sources are `EvidenceFeed`
implementations, and changing the source must not require kernel changes.

---

# Retrieval rules

- **Never fabricate vectors.** No synthetic embeddings, no fallback path that invents a result when a
  real one is unavailable.
- **Never substitute prettier retrieval numbers.** The measured distance is the number, including
  when it is unflattering.
- **Preserve the actual ANN distance.** Do not recompute it at render time; a receipt that drifts
  from the row it cites is worse than no receipt.
- **Distinguish query-relative distance from belief-relative contradiction distance.** A `considered`
  citation is measured against the query that surfaced it. A `contradicts` citation is measured
  against the belief's own claim. They are not interchangeable.
- **Do not introduce relevance thresholds without explicit evidence.** No arbitrary cutoffs, no
  silent reranking.
- **One citation per retrieval debt.** Two independent obligations may not be discharged by the same
  evidence, and each receipt must name the citation that actually paid it.

---

# Demo rules

Canonical decision: **"Is etcd v3.5.x safe to deploy?"**

Measured values — preserve exactly, never round differently or substitute:

| Value | Meaning |
|---|---|
| `7,239` | ingested etcd issues, PRs excluded; the measured corpus, never GitHub's live counter |
| `#19220 · 0.372424` | nearest hit for the deployment question |
| `#12987 · 0.387913`, `#12788 · 0.401092` | the rest of the deployment top-3 |
| `#14139 · 0.199509` | the integrity question's top hit |
| `#13766 · 0.594920` | the canonical falsifier, `relation: contradicts` |
| rank `573 / 7,239` | **an offline measurement, not application UI state** — the app does not render a rank |

Query text is byte-sensitive: `Is etcd v3.5.x safe to deploy?` yields `0.372424`; a lowercase `is`
yields a different vector and a different distance.

**SQLSTATEs are evidence.** `23503 · gate`, `23514 · promoted_is_debt_free`,
`23514 · live_requires_promoted` are engine output and must be surfaced verbatim, never replaced by
generic UI prose. An application-raised refusal carries no constraint name, deliberately — claiming
one would be inventing engine output.

**`40001` is a retry signal, not a refusal.** Under `crdb.ExecuteTx` the transaction retries and then
refuses on fresh state. The accurate chain is detect → retry → refusal on fresh state.

---

# Submission rules

- No unsupported claims. No invented metrics.
- Verify live behaviour before changing narration. If the deployed UI and the script disagree, stop
  and report the difference rather than working around it.
- Preserve the distinction between application logic and DB-enforced invariants.
- Do not claim a tool as verified on the strength of a configured endpoint.
- Do not cite a transcript that does not exist. Where evidence is code rather than a recorded run,
  cite the code.
- Run `task test` for the current suite result. Do not hardcode test counts into documentation —
  they go stale silently.

---

# Engineering philosophy

Prefer explicit invariants, receipts, deterministic behaviour, transactional correctness, minimal
architecture.

Avoid hidden state, prompt-only guarantees, duplicated truth, speculative abstractions, unnecessary
orchestration.

Agents do not decide truth. They observe evidence, propose beliefs, and propose actions. The ledger
determines whether those actions remain valid.

CockroachDB is not passive storage. It enforces transactional consistency, promotion rules, action
gating, atomic retraction, and auditability.

---

# Development rules

- Preserve the frozen architecture. Do not silently redesign behaviour.
- Unknowns become receipts.
- Database correctness takes precedence over application convenience.
- If a design decision benefits the demo but weakens the ledger, reject it. The ledger is the
  product; the demo is the proof.

When in doubt, ask:

> "Should this responsibility belong to the database instead?"
