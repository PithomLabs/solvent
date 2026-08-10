============================================================
SOLVENT REPOSITORY — REQUIRED CONTEXT
============================================================

The current public Solvent repository is:

https://github.com/PithomLabs/solvent

You MUST inspect this repository before beginning the research.

Use it to understand:

- the actual current architecture
- the four-table schema
- existing normalization types
- existing derivation semantics
- evidence/debt concepts
- current etcd fixtures
- CLI behavior
- RetractCascade behavior
- existing provenance fields
- current acceptance/invariant expectations
- what the coding agent will actually have to integrate with

Treat the repository as the authoritative source for:

    "What Solvent currently supports."

Do NOT assume that a capability exists merely because the research brief
describes it.

Likewise, do NOT modify or redesign Solvent as part of this research.

For external-world facts about etcd, CVEs, releases, advisories,
licenses, dates, and source material, independently verify those facts
against authoritative primary sources.

Your research should therefore distinguish:

    SOLVENT REPOSITORY FACT
        from
    EXTERNAL-WORLD FACT
        from
    YOUR INFERENCE / RECOMMENDATION

If the repository and this research brief disagree about the current
implementation, inspect the repository and report the discrepancy rather
than silently resolving it.


```text
# DEEP RESEARCH BRIEF — SOLVENT REAL-WORLD EVIDENCE DATASET

You are acting as an independent senior research analyst and data-engineering
researcher.

Do NOT write code.

Do NOT redesign Solvent.

Do NOT propose a large cybersecurity data platform.

Your job is to determine the smallest, strongest, most defensible
REAL-WORLD evidence dataset that should be integrated into the existing
Solvent project for its hackathon demo.

The final deadline is August 18, 2026, so optimize aggressively for:

- credibility
- simplicity
- deterministic replay
- provenance
- judge comprehension
- implementation speed

============================================================
CONTEXT: WHAT SOLVENT IS
============================================================

Solvent is a Transactional Belief Ledger built around CockroachDB.

Its core idea is:

    evidence
        ↓
    normalization
        ↓
    belief derivation
        ↓
    transactional belief ledger
        ↓
    promotion gate
        ↓
    action intent
        ↓
    audit / falsification

The central thesis is:

    "The ledger is the product. The demo is the proof."

And conceptually:

    "Memory is not remembering.
     Memory is refusing to act on what is no longer true."

The core implementation is already built and frozen.

Do NOT redesign it.

The current verified architecture has four database tables:

    belief
    belief_edge
    evidence
    action_intent

The existing pipeline already supports:

- evidence normalization
- deterministic belief derivation
- debt accumulation/retirement
- transactional promotion
- action-intent gating
- replay/idempotency behavior
- contradiction detection
- transactional RetractCascade
- audit invariants
- a CLI falsification path

The current core is deliberately small.

============================================================
CURRENT DEMO DOMAIN
============================================================

The current concrete demonstration domain is etcd security/release
evidence.

However, the architectural thesis is NOT:

    "Solvent is an etcd security tool."

The thesis is:

    "Transactional belief management can be applied to many domains."

etcd is simply the first concrete evidence domain because it provides a
compact, understandable lifecycle involving:

- a real software project
- releases
- vulnerabilities/advisories
- affected/fixed versions
- evidence that can strengthen or invalidate a belief
- potential remediation decisions

We want to demonstrate the general pattern without building a generic
multi-domain platform.

============================================================
CURRENT DATA PROBLEM
============================================================

The current repository uses a deterministic local fixture representing
an etcd vulnerability/evidence lifecycle.

It is useful for testing the pipeline, but it is not yet the strongest
possible hackathon demo because some of the evidence is simulated.

We now want to replace or augment that fixture with a SMALL PINNED SNAPSHOT
OF REAL-WORLD EVIDENCE.

CRITICAL:

We do NOT want a live API dependency.

The final demo should work offline after checkout and should produce the
same deterministic results every time.

Desired model:

    authoritative real-world sources
            ↓
       research/verification
            ↓
       pinned snapshot
            ↓
       deterministic local fixture
            ↓
          Solvent

============================================================
PRIMARY RESEARCH QUESTION
============================================================

What is the SMALLEST authoritative, real-world, redistributable or safely
referenceable etcd evidence dataset that can demonstrate a complete
Solvent lifecycle:

    evidence
       ↓
    belief
       ↓
    debt reduction
       ↓
    promotion
       ↓
    action intent
       ↓
    contradictory/falsifying evidence
       ↓
    transactional retraction
       ↓
    audit = safe

The dataset should be small enough that a judge can understand the entire
story rather than seeing an enormous cybersecurity dataset.

============================================================
RESEARCH OBJECTIVES
============================================================

Investigate authoritative sources for etcd, especially:

- official etcd GitHub repository/releases
- official etcd security advisories
- GitHub Security Advisories where authoritative/relevant
- CVE/NVD records
- OSV records
- CISA KEV if relevant
- release notes/changelogs
- official documentation
- other primary sources only where justified

Prioritize PRIMARY/AUTHORITATIVE sources.

Do not automatically combine every source.

Determine which sources provide the cleanest evidence chain.

============================================================
QUESTION 1 — WHAT REAL DATA SHOULD WE USE?
============================================================

Identify candidate real-world etcd records.

For each candidate provide:

- source
- URL
- stable identifier
- publication/release date
- exact relevant fields
- why it matters to the Solvent story
- whether it is authoritative
- whether it can be redistributed
- whether it can be pinned into a repository
- whether it requires attribution
- whether it is stable enough for deterministic replay

Do not recommend records merely because they are interesting.

Every record must have a role in the lifecycle.

============================================================
QUESTION 2 — MINIMUM DATASET
============================================================

Find the smallest useful dataset.

Prefer something approximately like:

- 1–3 vulnerabilities/advisories
- a few affected/fixed versions
- a few release records
- a small number of supporting evidence records

Do NOT assume those numbers are correct.

Derive the minimum from the actual evidence.

Explain why each selected record is necessary.

Also explicitly identify records that were considered but rejected and why.

============================================================
QUESTION 3 — CAN THE DATA SUPPORT FOUR DEMO STORIES?
============================================================

The final dataset should ideally support these four scenarios.

### A. NORMAL PROMOTION

Real evidence:

    ↓

belief formed

    ↓

required evidence/debt satisfied

    ↓

belief promoted

    ↓

action intent becomes legal

### B. REPLAY

Same real evidence replayed:

    ↓

same semantic result

    ↓

no duplicate belief/action

    ↓

deterministic output

### C. CONTRADICTION

New evidence conflicts with an existing belief:

    ↓

contradiction detected

    ↓

unsafe mutation/action prevented

### D. FALSIFICATION / RETRACTION

A promoted belief is subsequently invalidated:

    ↓

RetractCascade

    ↓

dependent live intent cancelled/retracted

    ↓

audit remains safe

For each scenario, identify the EXACT real records that would be used.

If the real-world data cannot naturally support one of these scenarios,
say so explicitly.

Do NOT manufacture a false contradiction merely to make the demo work.

If a synthetic fixture is necessary for one scenario, clearly label it
as synthetic and explain why.

============================================================
QUESTION 4 — MAP REAL DATA INTO SOLVENT
============================================================

Produce a precise mapping:

    REAL SOURCE
        ↓
    RAW RECORD
        ↓
    NORMALIZED EVIDENCE
        ↓
    DERIVED BELIEF
        ↓
    DEBT ITEM
        ↓
    PROMOTION
        ↓
    INTENT

For every selected real record identify:

- source
- source URL
- source identifier
- evidence type
- relevant fields
- normalization fields
- derived claim
- affected entity
- debt implication
- whether it supports or contradicts another claim

Do NOT invent new Solvent data types unless the existing model genuinely
cannot represent the evidence.

If the existing model cannot represent something important, flag it as a
gap rather than silently redesigning the model.

============================================================
QUESTION 5 — PROVENANCE
============================================================

Determine the minimum provenance we should preserve.

At minimum investigate:

- source URL
- source identifier
- source type
- retrieval date
- publication date if available
- content hash if appropriate
- version/release identifier
- original source attribution

Distinguish:

    required for Solvent
    useful for the demo
    unnecessary metadata

Keep the fixture small.

============================================================
QUESTION 6 — SNAPSHOT DESIGN
============================================================

Recommend how the real-world records should be pinned.

The final demo should not depend on:

- live Internet
- live APIs
- external service availability
- changing upstream data

Determine whether the best approach is:

- checked-in JSON
- JSONL
- normalized fixture
- raw source snapshot + derived fixture
- another simple format

Recommend the smallest reproducible approach.

We want a future judge to be able to inspect:

    "This is the real source."

then:

    "This is the exact snapshot Solvent consumed."

============================================================
QUESTION 7 — SOURCE RELIABILITY
============================================================

For every proposed source distinguish:

    PRIMARY
    SECONDARY
    DERIVED

Prefer primary sources.

Cross-check important facts across independent authoritative sources.

Do not treat search snippets or third-party summaries as evidence when
the primary source is available.

If sources disagree:

- report the disagreement
- identify the authoritative interpretation
- explain which record should enter the fixture

============================================================
QUESTION 8 — LEGAL / REDISTRIBUTION
============================================================

Investigate whether the proposed snapshot can safely be committed to a
public GitHub repository.

For each source determine:

- license
- terms relevant to redistribution
- attribution requirements
- whether storing the relevant excerpt/record is permissible
- whether storing only metadata + URL is safer
- whether the source is public but still unsuitable for copying verbatim

Do not give legal certainty where the source terms are ambiguous.

Clearly distinguish:

    verified licensing fact
    interpretation
    conservative recommendation

============================================================
QUESTION 9 — GENERALIZATION
============================================================

The demo should communicate that the abstraction is not inherently
cybersecurity-specific.

Without implementing another domain, identify 2–3 analogous domains
where the SAME evidence → belief → transactional gate pattern could apply.

Examples might include:

- infrastructure operations
- compliance
- financial reconciliation
- software release management

But do not force these examples.

The point is to determine whether the etcd dataset demonstrates a genuinely
general principle rather than a one-off cybersecurity workflow.

============================================================
QUESTION 10 — DEMO NARRATIVE
============================================================

Design the shortest judge-comprehensible story.

Target:

    under 2 minutes to understand

The judge should be able to see:

    1. real evidence arrives
    2. Solvent forms a belief
    3. evidence/debt accumulates
    4. transactionally promoted belief permits action
    5. new evidence invalidates it
    6. transactional retraction prevents stale action

Describe exactly what the judge sees and what they can play with.

Do NOT design UI yet.

============================================================
IMPORTANT CONSTRAINTS
============================================================

Do NOT recommend:

- huge datasets
- embeddings
- RAG
- vector databases
- live polling
- complex ETL infrastructure
- Kafka
- multiple agents
- Kubernetes deployment
- AWS infrastructure
- a generic cybersecurity platform
- multi-domain implementation
- schema redesign
- new database tables

The current Solvent kernel is FROZEN.

The purpose of this research is to feed the existing system with better
evidence, not to make the system bigger.

============================================================
OUTPUT
============================================================

Produce a detailed research report:

    REAL_EVIDENCE_RESEARCH.md

Use this structure:

1. Executive Recommendation

2. Canonical Demo Narrative

3. Source Inventory

4. Recommended Minimum Dataset

5. Exact Records to Capture

6. Evidence Provenance

7. Raw Record → Solvent Evidence Mapping

8. Evidence → Belief Mapping

9. Evidence → Debt Mapping

10. Normal Path

11. Replay Path

12. Contradiction Path

13. Falsification / Retraction Path

14. Snapshot / Fixture Recommendation

15. Licensing / Redistribution Assessment

16. Why etcd Is a Good Demonstration Domain

17. Generalization Beyond Cybersecurity

18. Rejected Alternatives

19. Risks and Unknowns

20. Exact Implementation Specification for Coding Agent

21. Acceptance Criteria

22. Source List

============================================================
EVIDENCE DISCIPLINE
============================================================

For every important factual claim:

- cite the source
- provide the exact URL
- identify whether it is primary/secondary
- distinguish fact from inference

Do NOT fabricate URLs, CVEs, release dates, licenses, or advisory details.

When uncertain, explicitly say:

    "Not verified."

Do not fill the gap from model memory.

============================================================
FINAL DELIVERABLE REQUIREMENT
============================================================

The final section, "Exact Implementation Specification for Coding Agent",
must be sufficiently concrete that another coding agent can implement the
dataset integration without having to redo the research.

It should specify:

- exact files/records to obtain
- exact fields to preserve
- exact fixture structure
- exact provenance
- exact normalization expectations
- exact derived beliefs
- exact debt expectations
- exact demo scenarios
- exact acceptance tests

But do NOT write the implementation code.

The research is successful only if it leaves us with:

    REAL DATA
       +
    STRONG PROVENANCE
       +
    MINIMAL FIXTURE
       +
    DETERMINISTIC REPLAY
       +
    COMPLETE DEMO STORY
       +
    NO ARCHITECTURAL EXPANSION

The objective is not to find the most data.

The objective is to find the smallest amount of REAL evidence that makes
Solvent's central idea undeniable.
```
