# Solvent — Engineering Guide

> **The model can propose. Memory can retrieve. Judgment can be wrong. Authority cannot silently outlive the belief it depends on.**

This document is the technical companion to the Solvent demo.

The video is deliberately a whirlwind tour: a decision is refused, evidence is retrieved, authority is granted, a later contradiction is introduced, an unsafe retraction is blocked, a safe cascade commits, and a control experiment shows what happens when the database boundary is removed.

This document slows that story down just enough for an engineer or judge to understand **what the repository is doing, where each invariant lives, and why CockroachDB is the important part of the design**.

---

## 1. What Solvent is

Solvent is a persistent belief-and-authority layer for an agent.

The system separates four things that are easy to blur together in ordinary retrieval-driven applications:

```text
Evidence
   │
   ▼
Belief
   │
   ▼
Authority
   │
   ▼
Action
```

Retrieval answers:

> What evidence can I find?

A belief answers:

> What do I currently accept as a working premise?

Authority answers:

> What am I actually allowed to do because of that premise?

Solvent's key design decision is that **retrieval is not authority**.

The agent may retrieve the wrong thing.
The agent may make the wrong judgment.
The database must still prevent an invalid authority state from surviving silently.

---

## 2. The concrete decision in the demo

The demo asks:

> **Is etcd v3.5.x safe to deploy?**

A fresh belief begins with six obligations.

Until those obligations are discharged, the belief is only a claim.

The first attempted authorization fails with:

```text
23503 · gate
```

The attempted promotion fails with:

```text
23514 · promoted_is_debt_free
```

A third attempt tries to discharge the contradiction sweep without a citation and fails with:

```text
23514
no citation for needContradictionSweep — select evidence in Search first
```

These are not UI validations pretending to be safety. They are database results surfaced through the application.

The deployed capture shows the first refusal explicitly as a foreign-key failure on `action_intent`. The refusal advances the wizard from ASK to DISCHARGE, and the scenario cannot simply continue as though nothing happened. fileciteturn219file0L39-L49

---

## 3. The memory layer

Solvent stores the agent's memory in CockroachDB rather than splitting semantic retrieval from transactional state.

The deployed system uses:

- CockroachDB Cloud Serverless
- a native vector index over `VECTOR(1024)`
- Amazon Bedrock `amazon.titan-embed-text-v2:0`
- a corpus of 7,239 etcd issues
- AWS App Runner + ECR for the public service
- `ccloud` as a fail-closed deployment preflight
- a self-built stdio MCP server exposing six domain tools

The repository's verified tool claims are deliberately narrower than the full architecture:

| CockroachDB capability | Status |
|---|---|
| Distributed Vector Indexing | verified |
| ccloud CLI | verified |

The vector-index evidence is a CockroachDB-native ANN index used by the live retrieval path; the project does **not** claim that it measured index sharding or distribution. fileciteturn219file5L313-L334

---

## 4. Retrieval is intentionally allowed to be incomplete

The demo uses two different searches against the same corpus and model.

### Deployment query

The deployment-oriented query surfaces plausible upgrade material, including:

```text
#19220 · 0.372424
```

### Integrity query

A more targeted integrity query surfaces:

```text
#14139 · 0.199509
```

The important observation is not that one query is "good" and the other is "bad."

It is that **changing the question changes what memory surfaces**.

That is the empirical reason Solvent refuses to equate "retrieved" with "authoritative."

The demo then goes one step further: a contradictory issue, `#13766`, is introduced explicitly because the deployment search did not surface it. Its measured distance from the belief claim is `0.594920`, and its offline measured rank is 573 / 7,239.

That is not hidden or replaced with a nicer retrieval result.

---

## 5. Evidence becomes an explicit receipt

When a retrieval result is actually used to discharge a review obligation, Solvent records the relationship.

The demo shows two retrieval-backed obligations with explicit receipts:

```text
needProvenanceCheck
  → #19220 · 0.372424

needContradictionSweep
  → #14139 · 0.199509
```

The remaining obligations are explicit operator artifacts rather than pretending that every obligation is a vector search result.

This distinction matters because a receipt should answer:

> **What did we actually rely on?**

not merely:

> **Did something appear on screen?**

The production demo also makes the absence of a citation a first-class refusal rather than silently treating a missing search result as success. fileciteturn219file0L48-L49

---

## 6. Where the real safety boundary lives

The MCP layer and Go application are intentionally not the final authority.

The design boundary is:

```text
Agent
  │
  ▼
MCP server
  │
  ▼
Solvent kernel / transaction code
  │
  ▼
CockroachDB
```

The agent does not get to decide that a belief is promotable or an intent is safe.

The MCP server is deliberately thin: validate input, call one kernel function, format the result. It does not implement belief semantics or SQL policy checks.

The kernel owns transaction discipline and reports the database result.

CockroachDB is the final invariant boundary.

The repository explicitly defines this division of responsibility: the agent owns reasoning, the MCP layer owns translation, the kernel owns transaction discipline, and CockroachDB owns the invariants. fileciteturn219file8L592-L623

---

## 7. The three database invariants that drive the demo

### I-1 — A promoted belief has no open debt

A `CHECK` constraint, `promoted_is_debt_free`, blocks promotion while review obligations remain open.

Observed demo failure:

```text
23514 · promoted_is_debt_free
```

This is why the first promotion attempt cannot simply "override" the review process.

### I-3 — A live intent must refer to a promoted belief

A composite foreign key named `gate` makes an action intent referentially impossible against a non-promoted belief.

Observed demo failure:

```text
23503 · gate
```

This is the action boundary.

### I-4 — Cancellation must precede retraction

The `live_requires_promoted` check is re-evaluated when a belief status changes.

Observed unsafe-retraction failure:

```text
23514 · live_requires_promoted
```

The important consequence is that a belief cannot be retracted while a live authorization still depends on it.

The architecture documentation is explicit that this is a database invariant, not an application convention. fileciteturn219file5L276-L303

---

## 8. The safe cascade

The belief graph is not walked recursively by CockroachDB.

That traversal is application logic implemented with a recursive SQL CTE in Go.

CockroachDB enforces the **final authority invariant**.

The safe sequence is:

```text
cancel intent
    ↓
retract belief
    ↓
cascade updates descendant state
    ↓
CHECK / FK constraints re-evaluate
    ↓
COMMIT
```

The successful demo state is:

```text
ancestor belief      retracted
descendant belief    retracted
action intent        cancelled
live_on_nonpromoted  0
```

The application decides what graph nodes to traverse. The database decides whether the resulting state is legal.

That separation is intentional. fileciteturn219file5L278-L290

---

## 9. Why the refusal trail matters

Every refusal is retained as part of the audit trail.

The demo therefore leaves behind a chronology showing:

```text
authorize   → 23503 · gate
promote     → 23514 · promoted_is_debt_free
discharge   → 23514
retract     → 23514 · live_requires_promoted
```

This matters because a system that only reports its final state can hide how it got there.

Solvent keeps both:

1. **what happened**, and
2. **why the database refused it**.

The SQLSTATE and constraint name are treated as product evidence rather than translated away into generic UI language. fileciteturn219file5L280-L286

---

## 10. The control experiment: why schema-level enforcement matters

The `/proof` page runs the same application race three ways.

| Schema | Isolation | Result |
|---|---|---|
| naive | READ COMMITTED | no error, both transactions commit, **AUDIT = 1** |
| naive | SERIALIZABLE | `40001 RETRY_SERIALIZABLE` |
| hardened | READ COMMITTED | `23503 · gate` |

The `40001` result is a retry signal, not a refusal.

The revealing cell is the first one:

> Correct application logic + weak schema → silently invalid committed state.

Nothing in the application code had to be "wrong" for the final state to be wrong.

The hardened schema instead causes the authority boundary to reject the same unsafe write at READ COMMITTED.

The control was run with local CockroachDB v26.2.0 for the first two cells and deployed v26.2.5 for the hardened cell; the project does not collapse those into one identical environment. fileciteturn219file5L292-L303

---

## 11. The three most important code paths

A fast codebase tour can therefore follow three paths rather than reading every package.

### Path A — retrieval

```text
query
  ↓
Amazon Titan embedding
  ↓
CockroachDB vector index
  ↓
ranked corpus issues
  ↓
evidence citation
```

The important thing to inspect is where an ANN result becomes a durable evidence receipt.

### Path B — authority

```text
belief
  ↓
debt
  ↓
promotion
  ↓
action intent
```

Inspect the SQL around:

- `promoted_is_debt_free`
- `gate`
- `live_requires_promoted`

Then look at the corresponding kernel calls and refusal classification.

### Path C — falsification

```text
contradictory evidence
  ↓
belief retraction
  ↓
dependent-intent handling
  ↓
transaction
  ↓
database constraint re-check
```

The key implementation detail is that recursive traversal is performed in Go/SQL application logic, while the final legality of the resulting authority state belongs to CockroachDB. fileciteturn219file8L598-L622

---

## 12. Why the demo is deliberately not "AI decides correctly"

The demo never claims that retrieval will always find the decisive fact.

It demonstrates the opposite.

The deployment query does not surface the later falsifier.

The judgment can still be wrong after evidence is recorded.

The system therefore does not try to make the model omniscient.

Instead, it makes the consequences of a wrong judgment:

- explicit,
- attributable,
- transactional,
- auditable,
- and bounded by database invariants.

That is the central design claim.

---

## 13. Reading the repository efficiently

A judge or engineer doing a fast repository tour should read in this order:

```text
README.md
    ↓
demo / video
    ↓
schema / lifecycle SQL
    ↓
belief + intent kernel
    ↓
MCP boundary
    ↓
vector retrieval
    ↓
proof/
```

Start with the README to understand the thesis and the three experiments.

Then inspect the schema and lifecycle SQL before reading much Go. The SQL is the source of truth for the invariants the Go implementation is supposed to reproduce. The repository explicitly calls out the pure-SQL proof as the authority for those semantics. fileciteturn219file9L661-L667

After that, trace one operation from MCP → kernel → SQL → SQLSTATE.

That gives a much faster understanding than reading the repository package-by-package.

---

## 14. The design in one diagram

```text
                    AGENT
                      │
             reasoning / proposal
                      │
                      ▼
                 MCP SERVER
             thin translation layer
                      │
                      ▼
               SOLVENT KERNEL
          transactions + traversal
                      │
             ┌────────┴────────┐
             │                 │
             ▼                 ▼
      VECTOR RETRIEVAL     BELIEF / INTENT
             │                 │
             └────────┬────────┘
                      ▼
                COCKROACHDB
        persistent memory + invariants
                      │
       ┌──────────────┼──────────────┐
       ▼              ▼              ▼
   23503 gate     23514 debt     23514 live_requires_promoted
```

The key architectural idea is that the database is not merely where memory happens to live.

**It is where the final authority boundary is enforced.**

---

## 15. What this project is actually demonstrating

Solvent is not primarily a vector-search demo.

The vector search is important because it exposes the retrieval problem.

The actual demonstration is:

```text
retrieval can be incomplete
        ↓
belief can therefore be wrong
        ↓
authority must not be implicit
        ↓
authority becomes a database-constrained state
        ↓
retraction cannot silently leave live authority behind
```

The control experiment is the proof of why the last step matters:

> **Correct application logic is not enough if the database permits an invalid committed state.**

That is the reason the memory layer and the safety boundary are the same CockroachDB system.

---

## 16. Verified limits and non-claims

To keep the repository technically honest:

- The demo does **not** claim perfect retrieval.
- Rank 573 is an offline measured rank, not a UI field.
- The project does **not** claim measured vector-index sharding/distribution.
- `40001` is described as a retry signal, not a refusal.
- Belief-graph traversal is application logic; CockroachDB does not recursively traverse the graph.
- Distributed Vector Indexing and ccloud CLI are the two verified CockroachDB tool claims.
- The composite FK and CHECK constraints are the core memory/authority architecture, not a separate rubric claim.
- The first two control cells and hardened cell were run against explicitly documented CockroachDB versions.

This discipline is intentional: the strongest claims in the repository are the ones the demo and database can actually prove.

---

## 17. If you remember only three things

**1. Retrieval is not authority.**

Finding something relevant does not make it safe to act.

**2. CockroachDB is the final boundary.**

The agent and application can propose and traverse; the database decides whether the committed authority state is legal.

**3. Cell 1 is the reason this matters.**

Correct application logic can still silently commit the wrong state when the schema is weak.

Solvent exists to make that failure impossible to ignore — and, where possible, impossible to commit.
