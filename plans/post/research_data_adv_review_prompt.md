# ADVERSARIAL REVIEW — SOLVENT REAL-WORLD EVIDENCE DATASET RESEARCH

You are acting as the final adversarial technical reviewer for the proposed
real-world evidence dataset for Solvent.

This is NOT a coding task.

Do NOT implement anything.
Do NOT redesign Solvent.
Do NOT expand the architecture.
Do NOT propose additional datasets unless required to resolve a concrete
defect.

Your job is to determine whether the research report is actually
implementation-ready against the CURRENT FROZEN SOLVENT REPOSITORY.

============================================================
INPUTS
============================================================

1. Solvent repository:

   https://github.com/PithomLabs/solvent

2. Attached research report:

   "Solvent Evidence Dataset Research.pdf"

The research report was produced by Gemini Deep Research.

You MUST read the entire research report.

You MUST inspect the current Solvent repository before reaching conclusions.

Treat:

    the repository
        =
    authoritative for what Solvent currently implements

Treat:

    the research report
        =
    a proposed real-world data/integration specification

Do NOT assume that claims in the research report about Solvent behavior
are true merely because the report states them.

============================================================
REVIEW QUESTION
============================================================

The central question is:

    "Can the proposed real-world etcd dataset be implemented faithfully
     against the CURRENT Solvent architecture without changing the frozen
     kernel semantics?"

We are specifically deciding whether the report can now be handed to the
coding agent.

Your verdict must be one of:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

============================================================
CRITICAL REVIEW AREAS
============================================================

### 1. DATASET COHERENCE

The report claims a minimum dataset consisting of:

- GHSA-q8m4-xhhv-38mg / CVE-2026-33413
- etcd v3.5.0
- etcd v3.5.27
- etcd v3.5.28
- v3.5-data-inconsistency postmortem
- one explicitly synthetic contradiction record

But inspect the report carefully.

Determine whether the actual proposed fixture contains every record required
by every scenario it describes.

In particular:

    Scenario D refers to ev_rel_v350

but inspect whether the concrete JSON fixture actually contains it.

If there is a mismatch:

- identify it precisely
- determine whether v3.5.0 must be added
- determine whether the scenario can instead be simplified
- recommend the smallest correction

Do not merely note the inconsistency; determine its consequence.

============================================================
2. REAL-WORLD FACT → SOLVENT BELIEF VALIDITY
============================================================

Audit whether every important proposed belief is actually supported by
the cited real-world evidence.

Pay particular attention to:

    bel_v3527_vulnerable

    bel_v3528_patches_cve

    bel_v350_stable

Ask:

- Is the belief wording actually supported by the source?
- Is the predicate semantically justified?
- Is the object correct?
- Is the version scope correct?
- Is the claim stronger than the source warrants?

In particular, examine the proposed claim:

    "etcd v3.5.0 is General Availability and suitable for production
     deployments"

and the use of the v3.5 data-inconsistency postmortem as invalidating
evidence.

Determine whether this is genuinely a defensible falsification/retraction
scenario or whether the report is overstating what the postmortem proves.

If the claim should instead represent:

    a deployment decision becoming invalid
    rather than
    a historical factual belief being proven false

say so explicitly.

Recommend the minimum semantic correction.

============================================================
3. ACTUAL SOLVENT SCHEMA / EDGE SEMANTICS

Inspect the repository's actual schema and implementation.

Verify:

    belief
    belief_edge
    evidence
    action_intent

and especially:

    belief_edge

Determine exactly:

- what IDs belief_edge references
- whether edges connect belief→belief, evidence→belief, or something else
- valid edge types
- directionality
- foreign-key expectations
- how SUPPORTS / CONTRADICTS / INVALIDATES are actually represented
- whether the research report's proposed edge mappings are legal

Pay particular attention to the report's proposed mapping:

    parent_belief_id = 'ev_postmortem_v35'

if that value is actually an evidence ID.

Determine whether this is a real defect or merely a naming/documentation
issue.

Do not guess.

============================================================
4. NORMALIZATION / DERIVATION COMPATIBILITY

Inspect the existing normalization and derivation code.

For every proposed evidence record determine:

    Can existing Solvent normalization consume this record as-is?

    Can existing derivation produce the proposed belief?

    Does the existing code recognize the proposed evidence_type?

    Does it understand the proposed payload fields?

    Does it already perform the required semver interpretation?

    Does it already understand v3.5.27 → v3.5.28 remediation?

If the report assumes new behavior, classify it:

    EXISTING CAPABILITY
    SMALL FIX / EXTENSION
    ARCHITECTURAL CHANGE

Architectural change is unacceptable for this phase.

============================================================
5. DEBT / PROMOTION SEMANTICS

Audit the proposed:

    Fix Availability Debt

and:

    Vendor Confirmation Debt

against actual Solvent debt semantics.

Determine:

- how debt is currently represented
- what creates it
- what retires it
- whether release evidence can actually retire it
- whether the proposed promotion sequence is valid
- whether the action intent can actually become LEGAL under existing rules

Do not accept conceptual descriptions unless the repository supports them.

============================================================
6. RETRACTION / RETRACTCASCADE

Inspect the actual RetractCascade implementation and tests.

Verify whether the proposed scenario:

    promoted belief
        ↓
    invalidating evidence
        ↓
    INVALIDATES relationship
        ↓
    RETRACTED belief
        ↓
    dependent belief(s)
        ↓
    CANCELLED action intent

actually matches current implementation semantics.

Determine whether the postmortem can be used with the existing falsification
CLI/path without new kernel behavior.

This is a key acceptance criterion.

============================================================
7. CONTRADICTION SCENARIO

The report explicitly labels the contradiction record synthetic.

That is acceptable in principle.

Determine whether:

    ev_synth_vendor_override

actually exercises the CURRENT contradiction semantics.

Verify:

- its target belief
- its claim structure
- how CONTRADICTS is derived
- what the current system does when contradiction exists
- whether the claimed "freeze" behavior actually exists

Do not allow the synthetic record to silently masquerade as real-world
evidence.

Determine whether its inclusion is:

    necessary
    useful
    or unnecessary

and recommend the smallest correct treatment.

============================================================
8. MINIMUM DATASET

Challenge the claim that this is the minimum defensible dataset.

Do NOT optimize for fewer records at all costs.

Instead ask:

    What is the smallest dataset that preserves the strongest demo narrative
    without distorting real-world facts?

Identify:

- unnecessary records
- missing records
- redundant records
- records required only because of an unnecessarily complicated scenario

The final recommendation must prioritize:

    factual integrity
    architectural compatibility
    demo clarity
    deterministic replay

over raw dataset size.

============================================================
9. PROVENANCE / FIXTURE DESIGN

Audit:

    manifest.json
    ATTRIBUTION.md
    raw snapshots
    etcd_demo_fixture.json
    hashes
    retrieval timestamps

against the actual repository conventions.

Determine whether the proposed fixture structure is compatible with the
existing ingestion path.

Do not approve a fixture format that requires an unnecessary ingestion
rewrite.

============================================================
10. LICENSING CLAIMS

Review the report's licensing conclusions conservatively.

Do not provide a legal opinion.

Determine whether the report distinguishes:

    verified license fact
    attribution requirement
    conservative repository recommendation

Pay particular attention to the assertion that GHSA data is CC-BY-4.0
and that the selected etcd materials may be redistributed exactly as
proposed.

If anything is insufficiently supported, flag it.

Do NOT derail the implementation unless licensing is genuinely blocking.

============================================================
11. INTERNAL CONSISTENCY

Search the research report for contradictions between:

- Executive Recommendation
- Canonical Demo Narrative
- Exact Records
- Evidence Mapping
- Belief Mapping
- Debt Mapping
- Scenario A
- Scenario B
- Scenario C
- Scenario D
- Snapshot specification
- Exact Implementation Specification
- Acceptance Criteria

The implementation specification must agree with the narrative.

Identify every material contradiction.

============================================================
12. SCOPE CONTROL

The final implementation MUST NOT require:

- a fifth database table
- schema redesign
- embeddings
- vector search
- RAG
- live NVD/OSV APIs
- Kafka
- streaming infrastructure
- multi-agent infrastructure
- generic cybersecurity platform
- new domain implementation

The Solvent kernel is frozen.

Flag anything in the report that implicitly violates this.

============================================================
DECISION FRAMEWORK
============================================================

For every finding classify it as:

    BLOCKER
    IMPORTANT
    MINOR
    NIT

A BLOCKER means:

- implementation would be incorrect,
- the dataset misrepresents real-world evidence,
- the proposed semantics contradict the existing kernel,
- or implementation would require architectural changes.

IMPORTANT means:

- should be fixed before implementation,
- but does not invalidate the overall dataset choice.

MINOR means:

- documentation/fixture cleanup,
- low implementation risk.

NIT means:

- optional polish that should NOT delay implementation.

============================================================
OUTPUT
============================================================

Produce:

    DATASET_ADVERSARIAL_REVIEW.md

Use this exact structure:

# 1. Executive Verdict

One of:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

Then give a concise rationale.

# 2. What the Research Got Right

Identify the strongest parts of the dataset selection and research.

# 3. BLOCKERS

For every blocker:

- finding
- evidence from repository/report
- why it matters
- minimum correction

# 4. IMPORTANT FINDINGS

Same structure.

# 5. MINOR FINDINGS / NITS

Keep this short.

# 6. Scenario-by-Scenario Verification

Table:

| Scenario | Research Claim | Repository Supports? | Real Evidence Valid? | Synthetic Evidence? | Verdict |
|---|---|---|---|---|---|

Cover:

    A. Normal Promotion
    B. Replay
    C. Contradiction
    D. Falsification / Retraction

# 7. Dataset Correction

Provide the exact final minimum record set.

For every record state:

- real/synthetic
- source
- purpose
- required/optional

# 8. Semantic Corrections

State the exact wording or mapping that should change.

Do not rewrite the whole research report.

# 9. Implementation Readiness

Answer explicitly:

    Can the research report now be handed to the coding agent?

If NO:

    list only the changes required before handoff.

If YES:

    state why.

# 10. Coding-Agent Handoff Delta

If corrections are required, provide a short exact list of changes that
must be incorporated into the coding-agent implementation prompt.

============================================================
FINAL CONSTRAINT
============================================================

This review is NOT an invitation to redesign Solvent.

If the dataset can be made correct with a small fixture/documentation
correction, prefer that over architectural changes.

If the research report contains an incorrect assumption about Solvent,
correct the assumption rather than modifying Solvent to accommodate it.

The objective is to reach:

    REAL DATA
        +
    CORRECT SEMANTICS
        +
    FROZEN KERNEL
        +
    DETERMINISTIC FIXTURE
        +
    IMPLEMENTATION-READY SPEC

with the smallest possible amount of rework.

Do not write code.

Do not commit anything.

Do not modify the repository.