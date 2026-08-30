# 06_README_BLUEPRINT.md

**Objective:** The actual README's structure and above-the-fold content,
synthesizing 01, 03, 04, 05.

**Audience:** Whoever writes the final README.

---

## Structure

### 1. Hero Section (above the fold)

```markdown
# Solvent

Memory isn't remembering. Memory is refusing to act on what is no longer true.

A transactional belief ledger for autonomous agents on CockroachDB.

[Run the demo →](HOSTED_DEMO_URL)   [View the receipts →](docs/)
```

Three lines. Two links. Nothing else.

### 2. The 60-Second Version

For the judge who never runs anything:

```markdown
## The 60-Second Version

Two identical databases. One real safety rule.

An agent tries to act on information the system already knows is dead.

Left lane: the action fires. A human gets paged over nothing.

Right lane: the database refuses. The constraint is named `gate`.

**AUDIT = 1** vs **AUDIT = 0.**

Nobody wrote code to catch this. The schema already knew.

![The 1-vs-0 reveal](static/reveal.png)
```

Include a static image or GIF of the AUDIT reveal. This is the single most
important visual in the entire submission.

### 3. Quickstart

```markdown
## Quickstart

```bash
git clone https://github.com/PithomLabs/solvent.git
cd solvent
task wow
```

That's it. The `task wow` command starts the database, runs the demo, and
verifies the invariants.
```

### 4. How It Works

```markdown
## How It Works

Solvent stores **beliefs**, not facts.

1. A belief enters carrying epistemic obligations ("debt").
2. Evidence arrives from the real world.
3. Debt is retired as evidence satisfies each obligation.
4. The belief is promoted — the agent may now act.
5. New evidence arrives that contradicts the belief.
6. The action is cancelled. Automatically. Transactionally. By the schema.

In this demo, the evidence feed is CISA's Known Exploited Vulnerabilities
catalog. But the kernel is domain-agnostic — the same mechanism works in
finance, medicine, logistics, or anywhere an autonomous agent must not act on
stale information.
```

### 5. The Receipts

```markdown
## The Receipts

Every claim in this project is backed by a receipt from a live CockroachDB
cluster.

- **M0** — Schema applied, constraints verified. [Receipt →](docs/M0_TRANSCRIPT.md)
- **M1** — Kernel compiles, zero raw writes. [Receipt →](docs/M1_I7.md)
- **M2** — 25/25 invariant tests pass. [Receipt →](docs/M2_TRANSCRIPT.md)

The schema is the source of truth. The tests are the evidence. The receipts
are the proof.
```

### 6. Architecture

```markdown
## Architecture

Frozen. Four tables. Two agents. One evidence feed.

- `belief` — claims carrying evidence, debt, and promotion state
- `belief_edge` — dependency graph between beliefs
- `evidence` — factual observations from upstream sources
- `action_intent` — actions gated by the database

The composite foreign key on `action_intent` — `(belief_id, belief_status)
REFERENCES belief(id, status) ON UPDATE CASCADE` — is the gate. When a
belief is retracted, the cascade propagates the new status into every intent
row, and the CHECK constraint refuses any live intent on a non-promoted
belief. This is not application logic. This is schema.
```

### 7. Engineering Philosophy

```markdown
## Engineering Philosophy

> "Should this responsibility belong to the database instead?"

- Correctness lives in schema and transactions, not application code.
- Every write goes through `crdb.ExecuteTx` for serialization-failure retry.
- The kernel is driver-agnostic — errors are classified by SQLSTATE, never
  by substring matching.
- Invariants are proven, not asserted. Receipts are real, not fabricated.
```

### 8. Getting Involved

```markdown
## Getting Involved

- Read the [Implementation Contract](IMPLEMENTATION_CONTRACT.md) — the frozen
  architecture.
- Read the [Data Pipeline Spec](plans/PRD/DATA_PIPELINE_SPEC.md) — the domain
  model.
- Browse the [Plans](plans/) — the full decision trail.
```

### 9. License

```markdown
## License

Solvent — Transactional Decision Memory for Autonomous Agents.
```

---

## Acceptance Criteria

- [ ] Hero section is skimmable in under 15 seconds.
- [ ] Every claim in the README traces to either a receipt in `docs/` or a
  section in the Phase 1 docs.
- [ ] Zero internal vocabulary (`accommodated`, `postulated`, `derived`).
- [ ] The 60-second version works as a standalone pitch (judge reads nothing
  else).
- [ ] The static image of the AUDIT reveal is included.
