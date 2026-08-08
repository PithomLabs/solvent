# DATA_PIPELINE_SPEC_REVIEW_PROMPT.md

**Purpose:** This prompt instructs an adversarial reviewer to produce a structured
review of `DATA_PIPELINE_SPEC.md`. The reviewer executes three sequential passes,
each from a different role. The output is the review document itself.

**Scope:** `DATA_PIPELINE_SPEC.md` only. Do not review kernel code, schema DDL,
Phase 1 docs, or any other artifact.

**Posture:** You are adversarial. Your job is to find problems, not to praise.
Every section you don't challenge is a section you've implicitly certified as
correct. Be explicit about what you certified and what you didn't.

---

## Instructions

1. Read `DATA_PIPELINE_SPEC.md` in full.
2. Read `IMPLEMENTATION_CONTRACT.md` in full — this is the frozen architecture
   contract. The spec must not contradict it.
3. Execute Pass 1 (Distributed Systems Engineer).
4. Execute Pass 2 (Data Engineer).
5. Execute Pass 3 (Security Engineer).
6. Execute Cross-Pass Synthesis.
7. Write the review document following the format below.

---

## Pass 1: Distributed Systems Engineer

You are a senior distributed systems engineer who has spent a decade debugging
production systems where "the application was supposed to handle this."

### Attack Vectors

For each section of the spec, ask:

1. **State machine completeness:** Can a belief or intent reach a state that has
   no defined outgoing transitions? Are there states where the system must act
   but the spec doesn't say who acts?

2. **Race conditions:** The spec describes a pipeline. What happens when two
   evidence items arrive for the same belief simultaneously? Is the pipeline
   sequential or concurrent? What serialization guarantees exist?

3. **Partial failure:** If the pipeline crashes mid-claim-extraction (evidence
   added but belief state not updated), what is the system's state? Is this
   recoverable?

4. **Invariant gaps:** The spec defines 5 rules in §5. Are there evidence
   combinations that produce a belief state not covered by any rule? What
   about evidence that neither strengthens nor weakens — does it hang forever?

5. **Termination:** The cascade follows `derives` edges transitively. Does
   the spec define cycle handling? What if `contradicts` edges form a cycle
   with `derives` edges?

6. **Debt semantics:** What if evidence retires a debt item, then new evidence
   arrives that re-opens the obligation? Can debt increase after retirement?
   The spec doesn't say.

7. **Promotion timing:** When exactly does promotion happen? Is it immediate
   after the last debt item is retired? Is there a window where the belief is
   "promotable but not yet promoted"? What can happen in that window?

8. **The gate's assumptions:** The composite FK assumes `belief_status` on
   `action_intent` matches `belief.status`. What if they drift (e.g., through
   a raw SQL path the kernel doesn't control)?

### Questions to Force

- Can a belief enter `promoted` state with non-empty debt through any path
  other than `Promote()`?
- What happens if `RetractCascade` is called on a belief that was never promoted?
- Can a `cancelled` intent be un-cancelled if new evidence reverses the retraction?
- What is the maximum depth of the dependency graph? Is there a practical limit?
- If two beliefs contradict each other, and both have live intents, what happens
  when one is retracted?

### Verdict Format

```
### Pass 1 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list — must be fixed before implementation)

**Nits:** (numbered list — non-blocking improvements)

**Certified Correct:** (list of sections you challenged and found sound)
```

---

## Pass 2: Data Engineer

You are a senior data engineer who has built ingestion pipelines that process
millions of events per day and have seen every form of upstream data corruption.

### Attack Vectors

For each input source in §3, ask:

1. **Schema drift:** What if GitHub changes its API response format? What if a
   new field appears that the normalization rules don't account for? Is the
   pipeline fail-closed or fail-open?

2. **Missing fields:** The spec marks fields as "required" and "optional." What
   happens when a required field is missing from upstream? Does the pipeline
   reject the event, or silently proceed with a null?

3. **Normalization correctness:** The spec says timestamps are "converted to
   UTC." What if the upstream timestamp has no timezone info? What if it's
   ambiguous (e.g., "2024-01-15 12:00:00")?

4. **Hash stability:** `content_sha256` is used for dedup. What if the same
   logical event produces different JSON (e.g., different field ordering)?
   Does the normalization produce deterministic hashes?

5. **Upstream idempotency:** Can the same upstream event be delivered twice?
   Is the pipeline idempotent? What if `content_sha256` collides (even
   theoretically)?

6. **Version parsing:** The spec says `version_affected` is "parsed into a
   comparison-friendly format (semver tuples)." What if the version string
   doesn't parse? What about pre-release versions, build metadata, or
   non-semver strings like `3.5.x`?

7. **Maintainer identification:** The spec says "author checked against a
   known-maintainer list (hardcoded for maintainers)." What if a non-maintainer
   posts a comment that looks like a maintainer confirmation? Is this
   a trust boundary?

8. **Embedding dedup:** The 0.15 cosine distance threshold is arbitrary. What
   is the false-positive rate? What is the false-negative rate? How is the
   threshold validated?

### Questions to Force

- What happens when the CISA KEV feed returns a CVE that doesn't match any
  known etcd version?
- What if a GitHub issue is reopened after being closed — does the pipeline
  create a new evidence item or update the existing one?
- Can the same CVE appear in both KEV and GitHub advisories with different
  version ranges? How is this reconciled?
- What if `content_sha256` is computed on the raw JSON but the JSON includes
  timestamps that change on every fetch?
- Is the normalization deterministic across different runs of the pipeline?

### Verdict Format

```
### Pass 2 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list)

**Nits:** (numbered list)

**Certified Correct:** (list of sections you challenged and found sound)
```

---

## Pass 3: Security Engineer

You are a security engineer who models threats against autonomous systems.
You assume the upstream feeds are untrusted and that an attacker's goal is to
make the agent act on a false belief or fail to act on a true one.

### Attack Vectors

1. **Belief poisoning:** An attacker controls an upstream feed. What can they
   inject to make the agent promote a false belief? Can they retire debt items
   without legitimate evidence?

2. **Evidence fabrication:** The spec says "no fabricated CVE IDs." But the
   pipeline trusts upstream data. What if a malicious feed returns a real CVE
   ID with fake version ranges? The pipeline would ingest it as legitimate
   evidence.

3. **Denial of service:** Can an attacker flood the pipeline with evidence
   items to prevent real beliefs from being promoted? Is there rate limiting?

4. **Schema injection:** The input model includes free-text fields (issue
   body, comment body, description). Can these contain SQL injection, prompt
   injection, or other attack payloads? Does the pipeline sanitize them?

5. **Trust boundary:** The spec says "author checked against a known-maintainer
   list." What if an attacker compromises a maintainer's GitHub account? Is
   the maintainer list the only trust boundary?

6. **Embedding manipulation:** Can an attacker craft a claim whose embedding
   is close enough to an existing belief to trigger the dedup threshold,
   effectively merging attacker-controlled evidence into a legitimate belief?

7. **Contradiction injection:** Can an attacker inject evidence that contradicts
   a promoted belief, causing a cascade that cancels live actions? This is the
   "stand-down attack" — the opposite of belief poisoning.

8. **Replay attacks:** Can old evidence be replayed to re-retire debt items
   that were legitimately retired? Is there temporal validation?

### Questions to Force

- What prevents an attacker from creating a fake GitHub issue with a fake CVE
  ID that passes the CVE pattern validator?
- If the pipeline is fail-open on malformed input, can an attacker bypass
  claim extraction entirely by sending events that don't match any rule?
- Can an attacker control the `content_sha256` hash by carefully constructing
  the upstream payload?
- Is there a maximum evidence count per belief? Can an attacker exhaust
  storage by attaching millions of evidence items to one belief?

### Verdict Format

```
### Pass 3 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list)

**Nits:** (numbered list)

**Certified Correct:** (list of sections you challenged and found sound)
```

---

## Cross-Pass Synthesis

After all three passes, synthesize:

### Aggregate Verdict

The overall status is the most restrictive of the three pass verdicts.
If any pass returns BLOCKED, the aggregate is BLOCKED.

### Must-Fix Before Implementation

Combine all blockers from all three passes. Deduplicate. Prioritize by
severity: anything that could produce corrupt state or enable adversarial
manipulation is P0; anything that could cause pipeline failure under
normal operation is P1; everything else is P2.

### Should-Fix (Non-Blocking)

Combine all nits from all three passes. These are improvements that would
strengthen the spec but don't block implementation.

### Design Questions (Escalate to Architect)

Questions that require a design decision, not just a spec fix. These are
escalated to the architect for ruling. Format each as:

```
DQ-N: [question]
Options: [option A] / [option B] / [option C]
Recommendation: [which option and why]
```

---

## Scope Check

Confirm that the review stayed within bounds:
- Reviewed only `DATA_PIPELINE_SPEC.md`
- Did not review kernel code, schema DDL, or Phase 1 docs
- Did not propose architecture changes
- Did not add new tables, agents, or evidence feeds beyond what the spec defines

---

## Authorization

The review authorizes (or does not authorize) implementation of the data
pipeline spec. If BLOCKED, the review must name the specific blocker that
prevents authorization and the minimum fix required to unblock.

---

## Decision Log

Record every decision made during the review, with the reasoning:

```
- DR-N: [decision] — [reasoning]
```

---

## Output

Write the review to `plans/PRD/DATA_PIPELINE_SPEC_REVIEW.md`.
