# 05_REPOSITORY_UX.md

**Objective:** What a judge sees the instant the repo opens, before reading a
word of prose.

**Audience:** Whoever restructures the repo before submission.

---

## Above the Fold (README hero)

The first thing a judge sees:

```
# Solvent

Memory isn't remembering. Memory is refusing to act on what is no longer true.

A transactional belief ledger for autonomous agents on CockroachDB.

[Run the demo →](link-to-hosted-demo)   [View the receipts →](docs/)
```

Nothing else above the fold. No badges, no feature lists, no installation
instructions. Three lines of text and two links.

---

## Directory Layout at a Glance

```
solvent/
├── cmd/                  # Entry points (demo, verify)
├── db/                   # Frozen schema (001_schema.sql)
├── docs/                 # Receipts: M0, M1, M2 reports and transcripts
├── internal/
│   ├── kernel/           # The transactional belief ledger (7 functions)
│   ├── testdb/           # Test harness (cluster management)
│   └── m0/               # M0 verification (frozen)
├── plans/                # The paper trail
│   ├── PRD/              # Requirements and data pipeline spec
│   ├── PHASE1/           # Product definition (this set of docs)
│   ├── M0/               # M0 plans and reviews
│   ├── M1/               # M1 plans and reviews
│   ├── M2/               # M2 plans and reviews
│   └── M3demo/           # M3 demo narrative
├── proof/                # SQL proof harness (PostgreSQL original)
├── scripts/              # Acceptance scripts (m0_accept, m1_accept, etc.)
├── AGENTS.md             # Engineering philosophy
└── IMPLEMENTATION_CONTRACT.md  # Frozen architecture contract
```

### What Each Directory Means (judge-facing language)

| Directory | What it is | Why it matters |
|---|---|---|
| `cmd/` | The programs you can run | Entry points for the demo and verification |
| `db/` | The database schema | The frozen source of truth — 4 tables, 66 lines of SQL |
| `docs/` | The receipts | Real test output from a real CockroachDB cluster |
| `internal/kernel/` | The engine | 7 functions, zero raw writes, every write through `crdb.ExecuteTx` |
| `plans/` | The thinking | How we got from requirements to implementation |
| `proof/` | The original proof | PostgreSQL verification that preceded the CockroachDB build |
| `scripts/` | The gates | Automated checks that run before every milestone |

---

## Proof Navigation

A judge should be able to find the invariant receipts within two clicks from
the README:

```
README
  └─ "View the receipts →"
       └─ docs/
            ├─ M0_REPORT.md      (cluster + schema proven)
            ├─ M0_TRANSCRIPT.md   (raw SQLSTATE receipts)
            ├─ M1_REPORT.md      (kernel compiles, I-7 enforced)
            ├─ M1_BUILD.md       (build + vet transcripts)
            ├─ M1_I7.md          (write-site audit)
            ├─ M2_REPORT.md      (25/25 invariant tests pass)
            ├─ M2_TRANSCRIPT.md  (behavioral receipts per case)
            └─ M2_TEST.md        (raw test output)
```

### Key Receipts (what to look for)

| Receipt | Where | What it proves |
|---|---|---|
| SQLSTATE 23514 × 3 | M0_TRANSCRIPT.md §C1, §C2, §D3 | Schema CHECK constraints fire correctly |
| SQLSTATE 23503 | M0_TRANSCRIPT.md §D1 | Composite FK gate refuses invalid intents |
| "cancel-then-retract commits" | M0_TRANSCRIPT.md §D4 | ON UPDATE CASCADE propagates correctly |
| 25/25 PASS | M2_TRANSCRIPT.md | All invariants hold on a live CockroachDB cluster |
| "rollback verified" | M2_TRANSCRIPT.md §B-16 | The cascade is ONE transaction (I-8) |

---

## The Paper Trail

The `plans/` directory is not documentation — it is the audit trail of every
decision made during the project:

```
plans/
├── pre/              # Early ideation (case studies, guidelines)
├── PRD/              # Requirements: DATA_PIPELINE_SPEC.md
├── PHASE1/           # Product definition (this set)
├── M0/               # "Can the schema actually enforce this on CockroachDB?"
├── M1/               # "Can the kernel compile against it?"
├── M2/               # "Does the kernel actually behave correctly?"
└── M3demo/           # "How do we show this to judges in 3 minutes?"
```

Each plan directory contains:
- The prompt that started the work
- The implementation plan
- The review (with verdict and decision log)
- The report (with receipts)

This is not overhead. This is the evidence that every design decision was
reviewed, approved, and verified.

---

## Acceptance Criteria

- [ ] A judge can find the invariant receipts (M0-M2) within two clicks from
  the README.
- [ ] The directory-layout description contains no package or file-level
  implementation detail beyond folder purpose.
- [ ] The README hero section is skimmable in under 15 seconds.
- [ ] Every claim in the README traces to either a receipt in `docs/` or a
  section in the Phase 1 docs.
