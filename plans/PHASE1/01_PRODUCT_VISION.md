# 01_PRODUCT_VISION.md

**Objective:** State what Solvent is, in one paragraph a judge could repeat back,
before any UX detail.

**Audience:** Judges skimming before they click anything; every other doc's author
checking they haven't drifted.

---

## The Thesis

Memory is not remembering. Memory is refusing to act on what is no longer true.

## What Solvent Is

Solvent is a **transactional belief ledger** for autonomous agents. It gives
agents a database-enforced memory system where beliefs carry evidence, evolve as
new evidence arrives, and govern whether an agent may act. When new evidence
falsifies a belief, every dependent decision is automatically cancelled — in one
atomic transaction, enforced by the database, not by prompt engineering.

## The Core Loop

1. **Belief enters** the system carrying epistemic obligations ("debt").
2. **Evidence accumulates** from real-world sources.
3. **Debt is retired** as evidence satisfies each obligation.
4. **Belief is promoted** when all debt is retired — the agent may now act.
5. **New evidence arrives** that contradicts the belief.
6. **Action is cancelled** — automatically, transactionally, by the schema.

## Why CockroachDB

This is not a prototype that happens to use a database. The correctness
properties — transactional isolation, composite foreign keys with ON UPDATE
CASCADE, CHECK constraints that reject invalid state transitions — are the
product. An operational project you assemble and pray over on Postgres is a
system you build and verify on CockroachDB.

## The Demo Instance

The hackathon demo uses a cybersecurity scenario: an autonomous agent tracking
CVE vulnerabilities in etcd. But the kernel is domain-agnostic. The same
mechanism works in finance, medicine, logistics, legal, or customer support —
anywhere an autonomous agent must not act on stale information.

## The One Sentence

If a judge reads nothing else:

> **Solvent proves that an AI agent can be wrong, and the database catches it
> anyway — using a rule the agent never had to remember.**

## Open Item

The public name for the product is still open. "Solvent" is the working name.
The public-facing name will be decided before submission.

---

## Acceptance Criteria

- [ ] A reader with zero context can restate the thesis in one sentence after
  reading this doc alone.
- [ ] Contains zero implementation nouns (no table names, no function names, no
  SQL).
- [ ] Zero internal vocabulary (`accommodated`, `postulated`, `derived`).
