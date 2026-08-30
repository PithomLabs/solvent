# M0 — Cluster + Schema Up (Work Package 02) — Revision 2

Supersedes `plan_pkg02.md`. Incorporates `plan_pkg02_review.md` (✅ APPROVED WITH NITS,
decisions D-017…D-021). To be written into the repo as `go/plan2_pkg02.md`.

## Changes from revision 1

| Ref | Nit | Disposition |
|---|---|---|
| N1 | `cmd/m0verify` has too many responsibilities; avoid a 1000-line `main.go` | **Adopted** — logic moves to `internal/m0/`, five small files; `main.go` becomes flags + wiring + exit code only |
| N2 | Capture ⟨?⟩ before and after the D-series | **Adopted, with a stated reading** — see note below |
| N3 | Record elapsed time per probe | **Adopted** — `elapsed_ms` column on every probe |
| N4 | Deterministic transcript output | **Adopted** — sorted objects, fixed fixture UUIDs, timestamps quarantined to one header line, `--no-timings` for byte-stable diffs |

All four were essentially free, so all four are in. No scope expanded: same probes, same
deliverable count, same stop condition.

> **N2 is truncated in `plan_pkg02_review.md`** (lines 109–112 read "Capture" followed by a
> blank, then "before and after D-series"). Given the stated rationale — *"if CockroachDB
> rewrites metadata after cascade operations"* — I read the missing object as
> **`SHOW CREATE TABLE`**, and implement it as B2 (before) / B4 (after) with a diff. If the
> intent was something else (`SHOW CONSTRAINTS`, `SHOW INDEXES`, `crdb_internal` catalog rows),
> say so and it is a one-line change.

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
  internal/m0/transcript.go    #   N4: deterministic rendering
  scripts/m0_up.sh             # start node on 26260, create/reset database `fable`
  scripts/m0_down.sh           # stop node; data dir removable
  docs/M0_TRANSCRIPT.md        # generated receipts
  docs/M0_REPORT.md            # pass/fail summary + §9 findings
  plan2_pkg02.md               # this plan
```

`internal/m0` is a harness package, not kernel code — it holds no belief-ledger semantics and
is deleted or ignored once M1 lands. Nothing in it may be imported by `internal/kernel`.

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
- B4 **(N2)** Re-run B2 after the D-series and diff against the B2 snapshot. Expected:
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

Build `./cmd/m0verify` specifically, not `./...`, so an unrelated compile problem in the
pre-supplied `internal/kernel` cannot masquerade as an M0 failure.

### Determinism (N4)

The transcript must diff cleanly across runs:
- **Fixed fixture UUIDs**, in the style of the Postgres proof
  (`00000000-0000-0000-0000-00000000000a`, …). No `gen_random_uuid()` anywhere in the harness.
- **Sorted** object listings — tables, constraints, indexes by name.
- **All wall-clock values confined to one header line** (`generated_at`, cluster build tag),
  so a diff of two runs touches exactly that line plus any genuine behavioral change.
- `elapsed_ms` lives in its own column and is suppressed entirely by `--no-timings`, giving a
  byte-stable artifact for CI comparison while keeping N3's signal in normal runs.
- Fixed-width table formatting; no map iteration order anywhere in rendering.

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

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
./scripts/m0_up.sh                 # single-node on 26260, fresh `fable` database
go build ./cmd/m0verify
./cmd/m0verify --dsn "$FABLE_DSN" --reset --apply --out docs/M0_TRANSCRIPT.md
echo "exit=$?"                     # 0 == every M0 criterion green
```

Then confirm by reading, not by trusting the exit code:
- `docs/M0_TRANSCRIPT.md` shows `23514` on C1/C2, `23503` on D1, a refusal on D3, a commit
  plus `belief_status='retracted'` on D4, `0` on D5, and **no diff** on B4.
- `SHOW CREATE TABLE action_intent` in the transcript contains `ON UPDATE CASCADE`.
- Cross-check the D-series line-by-line against `../plans/files/TRANSCRIPT.md`.

Determinism check (N4) — two clean runs must be byte-identical apart from the header:

```bash
./cmd/m0verify --dsn "$FABLE_DSN" --reset --apply --no-timings --out /tmp/t1.md
./cmd/m0verify --dsn "$FABLE_DSN" --reset --apply --no-timings --out /tmp/t2.md
diff <(tail -n +3 /tmp/t1.md) <(tail -n +3 /tmp/t2.md) && echo "deterministic"
```

Independent spot-check, outside the harness:

```bash
cockroach sql --insecure --host=localhost:26260 -d fable \
  -e "SHOW CREATE TABLE action_intent;" \
  -e "SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id
      WHERE a.state='live' AND b.status<>'promoted';"
```

## Stop condition (D-021)

When every M0 criterion is green, **stop**. Return: `M0_REPORT.md`, `M0_TRANSCRIPT.md`,
SQLSTATE receipts, CockroachDB version, READ COMMITTED verification, blocker report if any.
Await architectural review. **Do not begin M1.**
