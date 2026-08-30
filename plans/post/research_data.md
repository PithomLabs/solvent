Yes. **The next move is primarily research/data-engineering design, not coding.** I would hand this next phase to a stronger research-capable AI agent first, then bring the result back to the coding agent for implementation.

The distinction matters because we don't yet want to tell the coding agent *how* to obtain the data. We first need to decide **what data Solvent should consume, why those data are sufficient, and how to turn them into a deterministic evidence fixture without corrupting the abstraction.**

## What we're actually researching

The current Solvent core already accepts an evidence pipeline:

```text
evidence
   ↓
normalize
   ↓
derive
   ↓
belief/debt
   ↓
transactional ledger
   ↓
intent
```

The current evidence is deliberately simulated. The next phase asks:

> **What is the smallest set of real-world etcd evidence that can make this pipeline credible and visually compelling to a hackathon judge?**

Not:

> "How do we build a huge cybersecurity data ingestion platform?"

That distinction is critical.

---

# Research Phase: Real Evidence Design

I'd have the research AI answer **five questions**.

### 1. What real etcd evidence should we use?

We want a **small, authoritative, legally/simple-to-use snapshot**, probably involving something like:

```text
etcd release/version information
        +
CVE/vulnerability information
        +
security/advisory information
        +
resolution/remediation evidence
```

But we should not assume that combination yet.

The research agent should investigate the actual available sources and determine which ones give us the cleanest chain:

```text
"Something happened"
        ↓
"This belief became justified"
        ↓
"This debt was satisfied"
        ↓
"Action became permissible"
```

The goal is **evidence causality**, not dataset size.

---

### 2. What is the minimum dataset?

This is probably the most important question.

I would explicitly ask the research AI to find the smallest useful dataset, perhaps on the order of:

```text
1–3 vulnerabilities
2–4 releases
a handful of advisory/event records
a few pieces of supporting evidence
```

rather than hundreds or thousands of records.

We need enough data to demonstrate:

### Normal path

```text
evidence
 → belief
 → debt reduction
 → promotion
 → intent
```

### Replay

```text
same evidence
 → same result
 → no duplicate intent
```

### Contradiction

```text
contradictory evidence
 → contradiction detected
 → no unsafe mutation
```

### Falsification

```text
promoted belief
 → falsify
 → RetractCascade
 → intent cancelled
 → audit = 0
```

Those four stories are much more important than dataset volume.

---

# 3. Which real sources should become immutable snapshots?

This is where a research-capable agent is much better suited than the coding agent.

We don't want the demo depending on:

```text
Internet
   ↓
live API
   ↓
maybe unavailable
   ↓
demo fails
```

Instead:

```text
Authoritative source
       ↓
research/verification
       ↓
pinned snapshot
       ↓
Solvent fixture
       ↓
deterministic replay
```

The research agent should identify:

* authoritative source
* exact URLs
* relevant records
* retrieval date
* licenses/redistribution considerations
* stable identifiers
* fields we actually need
* what can be frozen into the repository
* what must be transformed

This gives us **real-world provenance without live-network fragility**.

---

# 4. How does real data map to our existing evidence model?

This is where we need to be particularly disciplined.

The research agent should produce a mapping like:

| Real-world fact      | Solvent evidence       |
| -------------------- | ---------------------- |
| etcd release         | release evidence       |
| CVE/advisory         | vulnerability evidence |
| fixed version        | remediation evidence   |
| subsequent release   | resolution evidence    |
| conflicting advisory | contradiction evidence |

Then:

```text
Real source field
       ↓
NormalizedEvidence
       ↓
DerivedBelief
       ↓
DebtItem
```

The research agent needs to identify **which real fields support which existing debt items**.

It should **not invent new debt semantics merely because the source contains additional information**.

That is how we prevent scope creep.

---

# 5. Does the real dataset demonstrate our "CGh-like" abstraction?

This is the deeper reason for doing the research.

Remember the principle we were trying to compress into the demo:

```text
       EVIDENCE
          │
          ▼
       BELIEF
          │
     ┌────┴────┐
     ▼         ▼
   SUPPORT   CONTRADICT
     │         │
     ▼         ▼
  ACTION     RETRACTION
   GATE       / BLOCK
```

The real dataset needs to make this **visible**, not merely technically possible.

The research agent should therefore identify:

> **one canonical story that a judge can understand in under two minutes.**

Something like:

```text
etcd vulnerability discovered
        ↓
belief: "etcd deployment is affected"
        ↓
evidence accumulates
        ↓
belief becomes promoted
        ↓
agent proposes remediation
        ↓
new evidence falsifies/invalidates belief
        ↓
transactional retraction
        ↓
remediation intent is cancelled
```

If the real-world data cannot naturally support that narrative, **we should choose different evidence**, not force the data into the story.

---

# What I would NOT ask the research agent to do

Very important.

Don't give it a giant open-ended prompt like:

> "Research cybersecurity datasets for Solvent."

That will produce a giant literature review and waste time.

Instead constrain it:

### No:

* ML research
* embeddings
* RAG
* huge datasets
* live ingestion architecture
* multiple agents
* generic cybersecurity platform
* Kubernetes + etcd + GitHub + CISA + NVD + OSV simultaneously
* production threat intelligence architecture

### Yes:

* **etcd only**
* authoritative sources
* minimum viable real dataset
* provenance
* deterministic snapshot
* mapping to existing Solvent evidence types
* four demo scenarios
* exact recommended records
* exact fields
* exact transformations
* licensing/redistribution considerations
* implementation-ready data specification

---

# The deliverable I want from the research AI

Not code.

I would ask for:

```text
REAL_EVIDENCE_RESEARCH.md
```

with exactly this structure:

```text
1. Executive recommendation

2. Canonical demo narrative

3. Authoritative sources

4. Exact records to capture

5. Minimum dataset

6. Evidence → NormalizedEvidence mapping

7. NormalizedEvidence → DerivedBelief mapping

8. Evidence → DebtItem mapping

9. Normal path

10. Replay path

11. Contradiction path

12. Falsification/retraction path

13. Provenance requirements

14. Snapshot format

15. Licensing / redistribution considerations

16. Why this dataset generalizes beyond cybersecurity

17. What NOT to implement

18. Exact implementation specification for the coding agent

19. Acceptance criteria

20. Risks / unresolved questions
```

The **last section should be sufficiently concrete that we can hand it directly to the coding agent**.

---

# The workflow from here

This is the important part:

```text
             CURRENT STATE
                  │
                  ▼
       CORE VERIFIED / FROZEN
                  │
                  ▼
       ┌────────────────────┐
       │ RESEARCH AI        │
       │                    │
       │ real data          │
       │ sources            │
       │ minimum dataset    │
       │ narrative          │
       │ mappings           │
       │ provenance         │
       └─────────┬──────────┘
                 │
                 ▼
       REAL_EVIDENCE_RESEARCH.md
                 │
                 ▼
          YOU + ME REVIEW
                 │
                 ▼
       implementation prompt
                 │
                 ▼
       ┌────────────────────┐
       │ CODING AGENT       │
       │                    │
       │ snapshot           │
       │ fixtures           │
       │ adapters           │
       │ tests              │
       └─────────┬──────────┘
                 │
                 ▼
          adversarial review
                 │
                 ▼
          VERIFIED REAL DATA
                 │
                 ▼
              bchat
                 │
                 ▼
          Taskfile/playground
                 │
                 ▼
               DEMO
```

So **yes: hand the next task to the more capable research AI.**

The coding agent's job comes *after* we know exactly what we're feeding Solvent.

And this is actually a very good moment to separate the two agents: **research AI decides what reality we want to capture; coding agent decides how to faithfully encode that decision into the existing architecture.** That prevents the coding agent from opportunistically expanding the data model just because it encounters interesting source data.
