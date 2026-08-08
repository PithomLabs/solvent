# AGENTS.md

Think like a distributed systems engineer, not an LLM engineer.

## Project

**Solvent** is a transactional belief ledger for autonomous agents.

The primary objective is **not** to build another AI agent or chatbot.

The objective is to build a **database-enforced memory system** where autonomous agents cannot act on beliefs that are no longer true.

The demo is only a presentation of this capability.

---

# Core Thesis

Traditional agent memory stores facts.

Solvent stores **beliefs**.

Beliefs:

- carry evidence,
- evolve as new evidence arrives,
- may be promoted or retracted,
- govern whether an agent may act.

The database—not the LLM—determines whether an action is allowed.

---

# Problem Statement

Modern AI agents often continue acting on stale information.

Prompt engineering cannot reliably prevent this.

Application code alone cannot reliably enforce correctness under concurrency.

Solvent applies distributed systems principles to agent memory.

Beliefs become transactional objects.

Actions become database-gated.

---

# Primary Design Principle

> Memory is not remembering.
>
> Memory is refusing to act on what is no longer true.

This principle takes precedence over convenience or agent autonomy.

---

# Architectural Principles

The architecture is frozen.

Do not redesign.

Current MVP consists of only:

- belief
- belief_edge
- evidence
- action_intent

No additional tables unless explicitly approved.

The kernel remains domain-agnostic.

---

# Implementation Status

Waves 1–4 are frozen. Each wave was implemented, reworked, adversarially reviewed, and verified against a live CockroachDB v26.2.0 cluster.

| Wave | Scope | Status | Tests |
|---|---|---|---|
| 1 — Normalize | `internal/normalize` | Frozen | 11 |
| 2 — Derive | `internal/derive` | Frozen | 16 |
| 3 — Kernel | `internal/belief`, `internal/intent`, kernel additions | Frozen | 48 |
| 4 — Pipeline | `internal/pipeline`, `cmd/solvent` | Frozen | 9 |

**Total: 84 tests, all passing.**

Frozen waves must not be modified. New work requires Technical Lead approval.

---

# Evidence

The kernel never depends on a specific domain.

Cybersecurity, etcd, GitHub, package advisories, logistics, etc. are merely **EvidenceFeed** implementations.

The current MVP uses a real etcd issue history as demonstration data, processed through the normalize→derive→pipeline chain.

Changing the evidence source must not require kernel changes.

---

# Agent Responsibilities

Agents do not decide truth.

Agents:

- observe evidence,
- propose beliefs,
- propose actions.

The Transactional Belief Ledger determines whether those actions remain valid.

---

# Database Responsibilities

CockroachDB is not passive storage.

CockroachDB enforces:

- transactional consistency,
- promotion rules,
- action gating,
- atomic retraction,
- auditability.

Whenever possible, correctness belongs in schema and transactions rather than application code.

---

# Engineering Philosophy

Prefer:

- explicit invariants
- receipts
- deterministic behavior
- transactional correctness
- minimal architecture

Avoid:

- hidden state
- prompt-only guarantees
- duplicated truth
- speculative abstractions
- unnecessary orchestration

---

# Scope

This repository builds the **Transactional Belief Ledger**.

The demonstration application exists solely to make the ledger understandable.

If a design decision benefits the demo but weakens the ledger, reject it.

If a design decision strengthens the ledger but simplifies the demo, prefer the ledger.

The ledger is the product.

The demo is the proof.

---

# Development Rules

- Preserve the frozen architecture.
- Frozen waves must not be modified. New work requires Technical Lead approval.
- Do not silently redesign behavior.
- Unknowns become receipts.
- Implementation follows approved specifications.
- Database correctness takes precedence over application convenience.

When in doubt, ask:

> "Should this responsibility belong to the database instead?"