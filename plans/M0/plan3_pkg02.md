# M0 — Cluster + Schema Up (Work Package 02) — Revision 3

Revision 2 was **✅ APPROVED, no blocking issues** (`plan2_pkg02_review.md`, D-022…D-026:
*"Proceed with implementation. Implement Revision 2 exactly as written. Do not optimize. Do
not simplify. Do not expand."*).

Revision 3 changes **nothing** about the probes, the architecture, or the scope. It folds in
the three review items that carry implementation consequences — D-024 (determinism promoted
to an acceptance criterion), D-023 (isolation made mechanical), O2 (report is an
architectural artifact, not a log) — and O1's protection note. Everything else below is
Revision 2 verbatim. To be written into the repo as `go/plan3_pkg02.md`.

## What Revision 3 adds

| Ref | Finding | Implementation consequence |
|---|---|---|
| **D-024** | *"Transcript determinism is now an acceptance criterion rather than a convenience."* | The two-run diff stops being a suggested command and becomes **gated probe E1**. Non-determinism fails M0 with a non-zero exit, exactly like a wrong SQLSTATE. Requires a machine-strippable volatile header (below). |
| **D-023** | *"`internal/m0` … must remain isolated from the kernel."* | Add a **mechanical** guard, not just a sentence: `scripts/check_isolation.sh` greps `internal/kernel` for any `internal/m0` import and fails the run. Prose can't be enforced; a check can. |
| **O2** | *"Treat `M0_REPORT.md` as an architectural artifact, not a log file. It should explain **why** a divergence matters."* | The report gets a fixed why-first structure (below) rather than being a dump of probe rows. The transcript is the log; the report is the argument. |
| **O1** | *"The harness is becoming an executable specification. Protect it. Do not let it evolve into production infrastructure."* | Every `internal/m0` file carries a header declaring it verification-only and frozen at M0; the isolation check enforces the boundary. |

D-025 (B2/B4 metadata diff) was already in Revision 2 and is unchanged. No probe was added,
removed, or reworded except the new E-series.

---

## Context

`IMPLEMENTATION_CONTRACT.md` is frozen. `prompt_pkg02.md` authorizes **Milestone M0 only**:
bring up CockroachDB, apply `db/001_schema.sql`, verify every M0 acceptance criterion. No
kernel code, no M1.

M0 exists to retire one architectural risk, not to write software. The invariant design was
proven on **PostgreSQL 16** (`proof/02_lifecycle_and_invariants.sql`; transcript at
`../plans/files/TRANSCRIPT.md`). Contract §9 names the composite FK + `ON UPDATE CASCADE` +
CHECK interaction as *"the one interaction most likely to differ from the Postgres proof;
verify at M0 and report before proceeding."* Everything below produces that receipt on the
real target version, in raw SQLSTATE codes rather than prose.

## Confirmed inputs and environment

| Fact | Value |
|---|---|
| Frozen DDL | `go/db/001_schema.sql` — present, 4 tables, matches contract §3 |
| CockroachDB binary | `~/.local/bin/cockroach` **v26.2.0** |
| Port 26257 / 8080 | **occupied** by the unrelated `bchat-crdb` container — must not be touched |
| Go | 1.25.7; `cockroach-go/v2` and `jackc/pgx` already in the module cache; proxy reachable |
| Given module file | `files/go.mod` → `module github.com/PithomLabs/solvent`, `go 1.24`, `cockroach-go/v2 v2.4.3` |
| Pre-existing | `internal/kernel/kernel.go` (M1 material — **read-only this milestone**) |

Confirmed via the CockroachDB docs MCP (D-018: documented behavior is authoritative;
undocumented behavior must be measured):
- `ON UPDATE CASCADE` on a composite FK referencing a `UNIQUE` constraint is supported and
  documented. **The docs do not state** whether the child's CHECK is re-evaluated on
  cascade-modified rows — D3/D4 measure it.
- `sql.txn.read_committed_isolation.enabled` defaults to `true` from v24.1 on, so v26.2 needs
  no cluster-setting change. M0 records the receipt anyway.

**Cluster target (assumption, on the record):** fresh local single-node on **port 26260 /
HTTP 8081**, database `fable`, from the installed v26.2.0 binary. Local because M2/M3 need
cheap drop-and-recreate cycles and a deterministic CI harness; 26260 because 26257 is taken.
Re-running against the CockroachDB Cloud cluster in `../.env` is a one-flag follow-up if a
managed-cluster receipt is wanted.

## Scope boundary

Build: cluster bringup, schema application, the M0 verification harness, two documents.
Do **not** build: any kernel function, any agent, any feed, any UI, any test file that
anticipates M2. Do not edit `internal/kernel/kernel.go` or `db/001_schema.sql`.

## Deliverables

```
go/
  go.mod                       # promoted from files/go.mod; + jackc/pgx/v5
  go.sum
  cmd/m0verify/main.go         # N1: thin — flags, wiring, exit code. Target < 120 lines.
  internal/m0/probe.go         #   Probe struct, Runner, per-probe timing (N3), SQLSTATE capture
  internal/m0/env.go           #   A-series: version, isolation settings
  internal/m0/schema.go        #   B-series: apply DDL, SHOW CREATE snapshots
  internal/m0/gate.go          #   C- and D-series: the CHECK and FK/CASCADE probes
  internal/m0/transcript.go    #   N4: deterministic rendering, volatile-header fencing
  scripts/m0_up.sh             # start node on 26260, create/reset database `fable`
  scripts/m0_down.sh           # stop node; data dir removable
  scripts/m0_accept.sh         # D-024: full gate — isolation check, build, run, E1 determinism
  scripts/check_isolation.sh   # D-023: mechanical kernel↮harness import guard
  docs/M0_TRANSCRIPT.md        # generated receipts (the log)
  docs/M0_REPORT.md            # the architectural argument (O2)
  plan3_pkg02.md               # this plan
```

`internal/m0` is a harness package, not kernel code — it holds no belief-ledger semantics and
is frozen once M1 lands. Nothing in it may be imported by `internal/kernel`; per **D-023**
that rule is enforced by `scripts/check_isolation.sh`, not by good intentions. Per **O1**,
each file opens with:

```go
// Verification-only. This package is M0's executable specification, not production
// infrastructure. It must never be imported by internal/kernel (D-023) and is frozen
// at the close of M0 (O1).
```

## The probe table

Every probe records: **id, criterion, statement, expected, observed, SQLSTATE, constraint
name, raw driver message verbatim, elapsed_ms** (N3). Each runs in its own transaction;
expected-failure probes roll back so nothing leaks forward.

**A — environment receipts**
- A1 `SELECT version()` → CockroachDB v26.2.x
- A2 `SHOW CLUSTER SETTING sql.txn.read_committed_isolation.enabled` → `true`
- A3 `BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED; SHOW transaction_isolation`
  → `read committed`. The real confirmation: it proves the transaction is **not** silently
  upgraded to SERIALIZABLE, which M3's cell 1 depends on.
- A4 `SHOW default_transaction_isolation` → `serializable` (records the default)

**B — schema application** (contract §7 M0: *"applies with zero errors"*)
- B1 Apply `db/001_schema.sql` statement-by-statement; any error aborts M0 immediately.
- B2 `SHOW CREATE TABLE` for all four tables, **sorted by name**, captured verbatim. The
  load-bearing receipt that CockroachDB preserved: the `gate` FK with `ON UPDATE CASCADE`,
  `belief_id_status_key` UNIQUE, `promoted_is_debt_free`, `live_requires_promoted`, the
  `ARRAY[...]` default on `debt TEXT[]`, and the partial index `live_intents`.
- B3 Exactly four tables exist (contract §2).
- B4 **(N2 / D-025)** Re-run B2 after the D-series and diff against the B2 snapshot. Expected:
  byte-identical. Any drift means CockroachDB rewrote metadata during cascade — a reportable
  event under D-020, printed as a unified diff.

**C — contract §7 M0 criteria, literally**
- C1 promoted-with-open-debt UPDATE → **23514**, constraint `promoted_is_debt_free`
- C2 `final_truth=true` + empty debt, then promote → **23514** (same constraint, I-2 arm)
- C3 debt-free, `final_truth=false`, promote → **succeeds**

**D — the §9 high-risk interaction** (the reason M0 exists; risk R1)
- D1 INSERT `action_intent` citing an `entered` belief → **23503**, constraint `gate`
- D2 INSERT `action_intent` citing a `promoted` belief → **succeeds**
- D3 With a live intent outstanding, `UPDATE belief SET status='retracted'` → **must be
  refused**. Postgres gave `23514` / `live_requires_promoted` via cascade. Record whatever
  CockroachDB actually returns. If it *succeeds*, M0 has found the blocker and stops there.
- D4 Correct order, one transaction: cancel the intent, then retract. Must commit, and the
  surviving cancelled row must read `belief_status='retracted'` — proving `ON UPDATE CASCADE`
  propagated. (The Postgres proof calls this "BUG-B FIX PASS".)
- D5 Audit `live` intents on non-promoted beliefs → **0**.

D3 and D4 are the whole point: D3 shows the schema refuses the wrong order, D4 shows it
permits the right one. Divergence from the Postgres transcript is a reportable architectural
event (D-020), not an implementation bug to be patched.

**E — acceptance-level checks (new in Revision 3)**
- E1 **(D-024) Determinism.** Two full `--reset --apply --no-timings` runs against a fresh
  database must produce transcripts that are byte-identical below the volatile header.
  A non-empty diff **fails M0** and is emitted in full. This is now a criterion, not a
  convenience — a transcript that cannot be reproduced is not a receipt.
- E2 **(D-023) Harness isolation.** `internal/kernel` must contain zero imports of
  `internal/m0`. A hit fails the run.

## Go harness design

`cmd/m0verify` is the "connection smoke test" M0 permits, doing double duty as the acceptance
runner. It pins the driver stack M1 inherits.

- `database/sql` + **`pgx/v5/stdlib`** (D-019: sole approved driver for the MVP), because
  `internal/kernel/kernel.go:153` classifies errors through an `interface{ SQLState() string }`.
  `*pgconn.PgError` implements that method; `lib/pq`'s `*pq.Error` does **not**. The harness
  proves this end-to-end by asserting `errors.As(err, &pgErr)` yields `"23514"` on C1 —
  SQLSTATE, never substring matching.
- No `crdb.ExecuteTx` in the probes. That wrapper retries and swallows; M0 needs the raw first
  error. `ExecuteTx` is a kernel-write requirement (I-7), and M0 writes no kernel.
- Flags: `--dsn`, `--apply`, `--reset` (drop+recreate `fable`), `--out`, `--no-timings`.
- Exit non-zero on the first failed criterion, printing exactly the three lines
  `prompt_pkg02.md` demands: **criterion / observed / expected**.

E1 and E2 live in `scripts/m0_accept.sh`, **not** inside the binary — running the verifier
twice and diffing is orchestration, and putting it in `main.go` would undo N1. The script is
the M0 gate; the binary is one step inside it.

Build `./cmd/m0verify` specifically, not `./...`, so an unrelated compile problem in the
pre-supplied `internal/kernel` cannot masquerade as an M0 failure.

### Determinism (N4, now gated by D-024)

The transcript must diff cleanly across runs:
- **Fixed fixture UUIDs**, in the style of the Postgres proof
  (`00000000-0000-0000-0000-00000000000a`, …). No `gen_random_uuid()` anywhere in the harness.
- **Sorted** object listings — tables, constraints, indexes by name.
- **All wall-clock values confined to a fenced volatile header** — `generated_at`, cluster
  build tag, host — delimited by literal `<!-- volatile:begin -->` / `<!-- volatile:end -->`
  markers. Revision 2 stripped the header with `tail -n +3`; that is line-count-fragile, and
  now that E1 gates the milestone the strip must be exact. Everything below the closing marker
  is required to be reproducible.
- `elapsed_ms` lives in its own column and is suppressed entirely by `--no-timings`, giving a
  byte-stable artifact while keeping N3's diagnostic signal in normal runs.
- Fixed-width table formatting; no map iteration order anywhere in rendering.

## `docs/M0_REPORT.md` structure (O2)

The transcript is the log. The report is the argument, and it is written to be read by the
architect, in this order:

1. **Verdict** — M0 green, or blocked, in one line.
2. **What was retired** — the §9 risk, stated as the question M0 asked and the answer it got:
   does CockroachDB v26.2 enforce cancel-before-retract through composite FK + `ON UPDATE
   CASCADE` + CHECK the way PostgreSQL 16 did? Cite D3/D4 receipts.
3. **Divergences from the PostgreSQL proof** — for each, *why it matters*: which invariant
   (I-1…I-8) it touches, which milestone it threatens, and what the architect must decide.
   An entry that only says "CockroachDB returned X instead of Y" is incomplete.
4. **Carried findings** — R2 (embedding) and R3 (temp tables), each with the same
   why-it-matters treatment and an explicit "not fixed in M0, per §9".
5. **Receipts index** — pointers into `M0_TRANSCRIPT.md`, no data duplicated.

If M0 is fully green with no divergence, §3 says so explicitly and states what that licenses:
M1 may proceed on the assumption that the schema is the gate.

## §9 findings to report, not fix

Recorded in `docs/M0_REPORT.md` and left alone — fixing either would be redesign.

1. **`belief.embedding` disagreement (R2).** Contract §3 says *"`belief.embedding` is
   nullable"* and I-6 references *"every `embedding` column NULL"*, but `db/001_schema.sql:3`
   states *"I-6: no embedding column"* and defines none. Per the authority clause this is a
   STOP-and-report item — but it blocks **no M0 criterion**, so M0 completes and the note goes
   to the architect for an M1/M4 ruling (M4 requires "merge duplicates via vector similarity",
   which will need a decision). Do not fix during M0.
2. **`CREATE TEMP TABLE` in `RetractCascade` (R3).** `internal/kernel/kernel.go:115` uses a
   temp table; CockroachDB gates temp tables behind `experimental_enable_temp_tables`.
   Observation only — M1's problem, flagged early so it is not discovered on demo day.

Anything else that surfaces gets the same treatment: named, quantified, escalated, never
worked around.

## Verification

The whole gate is one command (D-024 makes E1/E2 part of it, not optional extras):

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
./scripts/m0_up.sh          # single-node on 26260, fresh `fable` database
./scripts/m0_accept.sh      # E2 isolation → build → run → E1 determinism
echo "exit=$?"              # 0 == every M0 criterion green, including E1/E2
```

`m0_accept.sh` runs, in order: `check_isolation.sh`; `go build ./cmd/m0verify`; the verifier
with `--reset --apply --out docs/M0_TRANSCRIPT.md`; then two `--no-timings` runs into a temp
dir, stripped at the `<!-- volatile:end -->` marker and diffed. Any step non-zero stops the
script and prints criterion / observed / expected.

Then confirm by reading, not by trusting the exit code:
- `docs/M0_TRANSCRIPT.md` shows `23514` on C1/C2, `23503` on D1, a refusal on D3, a commit
  plus `belief_status='retracted'` on D4, `0` on D5, and **no diff** on B4.
- `SHOW CREATE TABLE action_intent` in the transcript contains `ON UPDATE CASCADE`.
- Cross-check the D-series line-by-line against `../plans/files/TRANSCRIPT.md`.
- `docs/M0_REPORT.md` §3 explains *why* each divergence matters, or states plainly that there
  were none.

Independent spot-check, outside the harness:

```bash
cockroach sql --insecure --host=localhost:26260 -d fable \
  -e "SHOW CREATE TABLE action_intent;" \
  -e "SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id
      WHERE a.state='live' AND b.status<>'promoted';"
```

## Stop condition (D-021)

When every M0 criterion is green — A, B, C, D, **and E** — **stop**. Return:
`M0_REPORT.md`, `M0_TRANSCRIPT.md`, SQLSTATE receipts, CockroachDB version, READ COMMITTED
verification, blocker report if any. Await architectural review. **Do not begin M1.**

If implementation discovers behavior different from this plan: stop, produce receipts,
escalate. Do not redesign.
