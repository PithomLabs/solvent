# 08_SUBMISSION_STRATEGY.md

**Objective:** Package everything for the actual Devpost submission fields.

**Audience:** Whoever fills out the submission form.

---

## Hackathon Requirements Mapping

| Requirement | Source Artifact | Status |
|---|---|---|
| **Repository URL** | GitHub repo: `github.com/PithomLabs/solvent` | Ready |
| **Demo App URL** | Hosted web demo (Journey A) | **Needs build** — Phase 2 |
| **Video (< 3 minutes)** | Recorded demo following 03_DEMO_STORYBOARD.md | **Needs recording** — Phase 3 |
| **Write-up** | Synthesized from 01, 03, 06 | **Needs drafting** — Phase 3 |

---

## Judging Criteria Coverage

| Criterion | Weight | How We Cover It | Artifact |
|---|---|---|---|
| **Technical Implementation** | High | Composite FK + ON UPDATE CASCADE + CHECK on CockroachDB v26.2. Schema-enforced invariants, not application logic. | `IMPLEMENTATION_CONTRACT.md`, `docs/M0-M2` |
| **Production Readiness** | High | What happens when things go wrong: the cascade-vs-intent race. Concurrency proven, not asserted. | `docs/M2_TRANSCRIPT.md`, M3 demo |
| **Creativity & Originality** | Medium | Side-by-side corruption-vs-refusal is a demo mechanic judges haven't seen. Split-screen live race. | 03_DEMO_STORYBOARD.md |
| **Real-World Impact** | Medium | "A human got paged over nothing" is a cost every judge has felt. Domain-agnostic kernel works in any vertical. | 01_PRODUCT_VISION.md, DATA_PIPELINE_SPEC.md |
| **Use of CockroachDB** | High | Serializable isolation, composite FKs, ON UPDATE CASCADE, `crdb.ExecuteTx` retry. Not just "uses a database" — the database is the product. | `internal/kernel/`, `docs/M0_TRANSCRIPT.md` |

---

## Open Decisions (must resolve before submission)

### 1. Public Name

**Status:** OPEN

"Solvent" is the working name (it is the Go module path:
`github.com/PithomLabs/solvent`). The public-facing name for the submission
has not been decided.

**Decision required:** Product owner / team vote.

**Timeline:** Before video recording.

**Impact:** Affects README, end card, submission title, demo URL.

### 2. Storyline CVE

**Status:** OPEN (deliberately deferred per project practice)

The demo uses a real CVE from CISA's KEV catalog as the falsifying evidence
trigger. The exact CVE has not been chosen — the project established early
(`fable_plan.md`: "Pick the storyline CVE at record time") that this would be
selected when the evidence feed is wired, not during planning.

**Requirements for the chosen CVE:**
- Must be real (anti-fabrication constraint in `IMPLEMENTATION_CONTRACT.md`)
- Must have a fix version (so the stand-down path works)
- Must be in CISA KEV (so the escalation path is believable)
- Must affect etcd (the demo's domain)
- Must be recent enough that judges might recognize it

**Decision required:** Select CVE at record time (M4/M5).

**Timeline:** Before video recording.

**Impact:** Affects demo narrative, evidence feed wiring, narrator script.

---

## Submission Form Fields

### Title

> Solvent: Transactional Decision Memory for Autonomous Agents on CockroachDB

(Subject to public-name decision)

### Tagline

> Memory isn't remembering. Memory is refusing to act on what is no longer true.

### Description (draft)

Solvent is a transactional belief ledger for autonomous agents. It proves that
an AI agent can be wrong, and the database catches it anyway — using a rule the
agent never had to remember.

The core mechanism: a composite foreign key with ON UPDATE CASCADE that enforces
cancel-before-retract at the schema level. When new evidence falsifies a belief,
every dependent action is cancelled in one atomic transaction. No application
code. No prompt engineering. Just database-enforced correctness.

**What we built:**
- A domain-agnostic kernel (7 functions, zero raw writes) on CockroachDB
- Schema-enforced invariants (I-1 through I-8) proven against a live cluster
- A split-screen demo: the same race, two schemas, two outcomes — AUDIT = 1 vs 0
- A data pipeline specification for autonomous agent memory

**Why CockroachDB:**
The correctness properties — transactional isolation, composite foreign keys,
ON UPDATE CASCADE — are not features we use. They are the product. This is not
a prototype that happens to use a database. The schema IS the safety mechanism.

### Links

| Link | URL | Description |
|---|---|---|
| Code | `github.com/PithomLabs/solvent` | The repository |
| Demo | (hosted URL — Phase 2) | Interactive split-screen demo |
| Video | (YouTube/Loom — Phase 3) | 3-minute demo recording |
| Docs | `github.com/PithomLabs/solvent/tree/main/docs` | M0-M2 receipts |

---

## Deliverables Checklist

| Deliverable | Owner | Status | Blocks |
|---|---|---|---|
| Repository pushed to GitHub | — | ☐ | Submission |
| Hosted demo URL | Phase 2 | ☐ | Submission |
| Video recording | Phase 3 | ☐ | Submission |
| Write-up | Phase 3 | ☐ | Submission |
| Public name decided | Product owner | ☐ | Video, README, submission |
| Storyline CVE selected | M4/M5 | ☐ | Demo wiring, video |

---

## Acceptance Criteria

- [ ] Every mandatory submission field has a named source artifact.
- [ ] Both open decisions (public name, storyline CVE) are listed explicitly,
  not buried.
- [ ] The judging-criteria coverage table shows no gaps.
- [ ] The submission form can be filled out using only artifacts in this
  repository.
