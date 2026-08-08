# M0 Report — Cluster + Schema Up

Work Package 02. Companion to `docs/M0_TRANSCRIPT.md`, which holds the receipts.
This document is the argument; the transcript is the log.

---

## 1. Verdict

**M0 is GREEN.** All 16 probes pass (A1–A4, B1–B4, C1–C3, D1–D5), plus the two
acceptance-level checks E1 (determinism) and E2 (harness isolation). No blocker.
**M1 is not started, per D-021.**

| Deliverable (`prompt_pkg02.md`) | Status |
|---|---|
| Schema applied | ✅ `db/001_schema.sql`, 6 statements, zero errors (B1) |
| Verification transcript | ✅ `docs/M0_TRANSCRIPT.md` |
| SQLSTATE receipts | ✅ 23514 ×3, 23503 ×1, all from `*pgconn.PgError` |
| CockroachDB version | ✅ **CockroachDB CCL v26.2.0** (x86_64-pc-linux-gnu, built 2026/04/21, go1.25.5) |
| READ COMMITTED confirmed | ✅ setting `true` (A2); a real transaction reports `read committed` (A3) |
| Blocker report | None. Four non-blocking divergences and two carried findings, below. |

---

## 2. What M0 retired

Contract §9 named one risk above all others:

> *"The composite FK + `ON UPDATE CASCADE` + CHECK interaction behaves differently on the
> target CockroachDB version than §5 states (this is the one interaction most likely to
> differ from the Postgres proof; verify at M0 and report before proceeding)."*

**The question M0 asked:** when a belief's `status` changes from `promoted` to `retracted`
while a `live` intent still references it, does CockroachDB propagate the new status into the
child row through `ON UPDATE CASCADE` *and then re-evaluate the child's CHECK* — so that the
wrong ordering is refused by the schema rather than by developer discipline?

The CockroachDB documentation states that `ON UPDATE CASCADE` against a composite `UNIQUE`
target is supported. It does **not** state whether child CHECK constraints are re-evaluated on
cascade-modified rows. That gap is why this was measured, not assumed (D-018).

**The answer: yes, identically to PostgreSQL 16.**

- **D3** — with a live intent outstanding, `UPDATE belief SET status='retracted'` was refused
  with **SQLSTATE 23514**, constraint **`live_requires_promoted`** (name carried in the
  driver's structured `ConstraintName` field). The cascade fired, the child CHECK was
  re-evaluated, and the write was rejected. Invariant **I-4 holds on CockroachDB.**
- **D4** — the correct ordering (cancel live intents, then retract, in **one** transaction)
  committed; 2 beliefs were retracted; and the surviving cancelled intent reads
  `state='cancelled'`, `belief_status='retracted'` — proving the cascade propagated rather
  than the row merely surviving untouched. This is the Postgres proof's "BUG-B FIX PASS",
  reproduced. Invariant **I-8's ordering requirement is enforced by the schema.**
- **D1** — an intent citing a never-promoted belief was refused with **SQLSTATE 23503**,
  constraint **`gate`**. Invariant **I-3 holds.**
- **D5 / independent check** — `live`-on-non-promoted count is **0**. Invariant **I-5 holds.**

Corroborated outside the harness with `cockroach sql`: intent `…d0` (the D1 attempt) does not
exist, `…d1` is `cancelled`/`retracted`, beliefs `…0a` and `…0b` are `retracted`, `…c1`/`…c2`
remain `entered` (promotion refused), `…c3` is `promoted`.

**What this licenses.** The architecture's central claim — *the gate is the database's, not the
application's* — is true on the target engine. M1 may proceed on the assumption that the
schema enforces I-1 through I-5, and that the kernel's job is routing and ordering, not
gatekeeping. No invariant was weakened to reach this result.

**What M0 did not test,** and must not be read as covering: concurrent behavior. M0 ran
single-session probes. The cascade-vs-new-intent race, and whether `READ COMMITTED` permits
the corruption that `SERIALIZABLE` and the hardened FK refuse, is **M3's** experiment. A3
establishes only the precondition — that a `READ COMMITTED` transaction genuinely runs at
`READ COMMITTED` and is not silently upgraded, which M3's cell 1 depends on.

---

## 3. Divergences from the PostgreSQL proof

Four, none blocking. Behavior is identical in every case; the differences are in surfaces.

### 3.1 CHECK violation messages do not name the constraint — affects M5

PostgreSQL 16 (`../plans/files/TRANSCRIPT.md`):

```
new row for relation "belief" violates check constraint "promoted_is_debt_free"
```

CockroachDB v26.2 (transcript §C1):

```
ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING)
  OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth)))
  (SQLSTATE 23514)
```

The constraint *name* is absent from the message text; CockroachDB prints the expression
instead. The name is still available, in the structured `ConstraintName` field of
`*pgconn.PgError` — which is how probes C1, C2 and D3 identify it.

**Why it matters.** M5's acceptance criterion is: *"an action attempt against an unpromoted
belief returns the raw FK error (23503) with no LLM narration substituted for it."* That
specific criterion is **safe** — FK errors *do* name the constraint on CockroachDB
(`violates foreign key constraint "gate"`, transcript §D1), so the M5 cell reads cleanly on
camera. The exposure is any demo surface that shows a **CHECK** refusal (I-1, I-2, I-4): the
raw CockroachDB text is an expression dump that neither names the invariant it defends nor
reads as evidence to a viewer.

**Decision required:** is rendering `pgErr.ConstraintName` alongside the raw message
"narration substituted for the error"? I read it as not — it is a database-supplied field, not
prose, and it restores exactly the information PostgreSQL put in the message. But M5's wording
is strict and this is the architect's call, not the implementer's. **Not acted on in M0.**

### 3.2 Auto-generated CHECK constraint names differ

Inline column CHECKs are named by the engine. PostgreSQL would produce
`belief_claim_type_check`; CockroachDB produced `check_claim_type`, `check_status`,
`check_state`, `check_provenance_class`, `check_parent_id_child_id`.

Every **explicitly named** constraint survived byte-exact: `promoted_is_debt_free`,
`live_requires_promoted`, `gate`, `belief_id_status_key`, and the partial index `live_intents`
(B2). **Why it matters:** nothing in the contract or kernel references the auto-named ones, so
the impact today is zero. It becomes a hazard only if later code matches on a constraint name
it did not choose. Recorded so that it is a known fact rather than a future surprise.

### 3.3 All four tables carry `schema_locked = true`

A CockroachDB v26.2 default, visible in every `SHOW CREATE TABLE` (B2). **Why it matters:** it
is not an M0 issue, but any future `ALTER TABLE` must first clear the lock. That directly
touches finding R2 below — if the architect rules that `belief.embedding` should exist, adding
it is a two-step operation on this engine, not one.

### 3.4 Type aliasing

`TEXT` → `STRING`, `TEXT[]` → `STRING[]`, `BOOLEAN` → `BOOL` in the stored DDL. Cosmetic;
CockroachDB aliases these to identical semantics. The `debt` array default survived intact,
including all six starting debt items (B2). No action.

---

## 4. Carried findings (escalated, not fixed — contract §9)

### R2 — `belief.embedding` is specified but does not exist

Contract §3 states *"`belief.embedding` is nullable; the ledger is fully functional with every
embedding NULL"*, and I-6 speaks of *"every `embedding` column NULL"*. The frozen
`db/001_schema.sql:3` states the opposite in a comment — *"I-6: no embedding column"* — and
defines no such column. Confirmed empirically: `SHOW CREATE TABLE belief` (B2) has no
`embedding`.

Under the contract's own authority clause (*"Where this file and `db/001_schema.sql` disagree,
STOP and report"*) this is a reportable disagreement. **It blocked no M0 criterion**, so M0
completed rather than halting.

**Why it matters.** I-6 is currently satisfied *vacuously* — "all invariant tests pass with
every embedding NULL" is trivially true when the column does not exist, which is not the
property the invariant was written to assert. More concretely, **M4 requires "merge duplicate
claims via vector similarity before insert."** There is nowhere to put a vector. M4 cannot be
implemented against this schema as frozen.

**Decision required before M4** (and, given §3.3, cheaper to decide before M1 writes against
the table): either add `embedding VECTOR(1536)` nullable to `belief`, or amend contract §3 and
I-6 to drop the column and re-specify how M4 achieves merge-not-fork. **Not acted on in M0.**

### R3 — `CREATE TEMP TABLE` in `RetractCascade` will fail on CockroachDB

`internal/kernel/kernel.go:115` (pre-supplied M1 material) builds its descendant set with
`CREATE TEMP TABLE _desc ON COMMIT DROP`. Verified on this cluster:

```
SHOW experimental_enable_temp_tables;   -->  off
CREATE TEMP TABLE _probe_tmp (id INT);
ERROR: temporary tables are only supported experimentally
SQLSTATE: XCEXF
HINT: You can enable temporary tables by running `SET experimental_enable_temp_tables = 'on'`.
```

**Why it matters.** `RetractCascade` is the single most important function in the system — it
is invariant I-8 and the live cascade the M6 demo shows on camera. As written it fails at
runtime on the target engine, with a SQLSTATE (`XCEXF`) that is neither 23514 nor 23503 and
would not be recognized by the kernel's error classification. This is an M1 defect, discovered
at M0 rather than on demo day. It is not fixed here: M0 writes no kernel code.

For the record, the cascade is expressible without a temp table — this harness's D4 computes
the same transitive descendant set with an inline `WITH RECURSIVE` CTE attached to each
`UPDATE`, and it commits correctly (transcript §D4). That is an observation about
feasibility, not a proposed redesign; the fix is M1's to make and the architect's to approve.

---

## 5. Receipts index

All in `docs/M0_TRANSCRIPT.md`; nothing is duplicated here.

| Claim | Receipt |
|---|---|
| Cluster version | A1 |
| READ COMMITTED available / not upgraded | A2, A3 |
| Schema applies with zero errors | B1 |
| Constraints preserved as written | B2 (full `SHOW CREATE TABLE`) |
| Exactly four tables | B3 |
| Cascade did not rewrite catalog metadata | B4 |
| Promote-with-debt refused, 23514 | C1 |
| Promote final-truth refused, 23514 | C2 |
| Promote debt-free succeeds | C3 |
| Intent on unpromoted refused, 23503 `gate` | D1 |
| Intent on promoted accepted | D2 |
| **Retract-with-live-intent refused, 23514 `live_requires_promoted`** | **D3** |
| **Cancel-then-retract commits; cascade propagates** | **D4** |
| Audit = 0 | D5 |
| Two runs byte-identical below the volatile fence | E1 (`scripts/m0_accept.sh`) |
| `internal/kernel` imports nothing from `internal/m0` | E2 (`scripts/check_isolation.sh`) |

Reproduce end to end:

```bash
./scripts/m0_up.sh        # single-node v26.2.0 on 26260, database `fable`
./scripts/m0_accept.sh    # E2 → build → A–D transcript → E1
```

---

## 6. Stop

M0's acceptance criteria are met. Per D-021 and `prompt_pkg02.md`, work stops here and awaits
architectural review. **M1 has not been started.** Two decisions are pending and both belong
to the architect: R2 (the embedding column) and §3.1 (whether `ConstraintName` may be
displayed beside a raw CHECK error in the M5 demo path).
