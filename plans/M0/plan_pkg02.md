# M0 — Cluster + Schema Up (Work Package 02)

## Context

`IMPLEMENTATION_CONTRACT.md` is frozen. `prompt_pkg02.md` authorizes **Milestone M0 only**:
bring up CockroachDB, apply `db/001_schema.sql`, and verify every M0 acceptance criterion —
no kernel code, no M1.

M0 exists to retire one specific risk. The invariant design was proven on **PostgreSQL 16**
(`proof/02_lifecycle_and_invariants.sql`, transcript in `../plans/files/TRANSCRIPT.md`).
Contract §9 names the composite FK + `ON UPDATE CASCADE` + CHECK interaction as *"the one
interaction most likely to differ from the Postgres proof; verify at M0 and report before
proceeding."* Everything below is built to produce that receipt, on the real target version,
with raw SQLSTATE codes rather than prose.

## Confirmed inputs and environment

| Fact | Value |
|---|---|
| Frozen DDL | `go/db/001_schema.sql` — present, 4 tables, matches contract §3 |
| CockroachDB binary | `~/.local/bin/cockroach` **v26.2.0** |
| Port 26257 / 8080 | **occupied** by the unrelated `bchat-crdb` container — must not be touched |
| Go | 1.25.7; `cockroach-go/v2` and `jackc/pgx` already in the module cache; proxy reachable |
| Given module file | `files/go.mod` → `module github.com/PithomLabs/solvent`, `go 1.24`, `cockroach-go/v2 v2.4.3` |
| Pre-existing | `internal/kernel/kernel.go` (M1 material — **read-only this milestone**) |

Docs confirmed via the CockroachDB MCP:
- `ON UPDATE CASCADE` on a composite FK referencing a `UNIQUE` constraint is supported and
  documented. **What the docs do not state** is whether the child's CHECK is re-evaluated on
  cascade-modified rows — that is exactly what D3/D4 below measure empirically.
- `sql.txn.read_committed_isolation.enabled` defaults to `true` from v24.1 onward, so v26.2
  has READ COMMITTED available without a cluster-setting change. M0 still records the receipt.

**Cluster target (assumption, stated for the record):** a fresh local single-node cluster on
**port 26260 / HTTP 8081**, database `fable`, started from the installed v26.2.0 binary. Local
because M2/M3 will need cheap drop-and-recreate cycles and a deterministic CI harness; 26260
because 26257 is in use. Re-running the same script against the CockroachDB Cloud cluster in
`../.env` is a one-flag follow-up if a managed-cluster receipt is wanted — say the word.

## Scope boundary

Build: cluster bringup, schema application, an M0 verification harness, two transcripts.
Do **not** build: any kernel function, any agent, any feed, any UI, any test file that
anticipates M2. Do not edit `internal/kernel/kernel.go` or `db/001_schema.sql`.

## Deliverables

```
go/
  go.mod                    # promoted from files/go.mod; + jackc/pgx/v5
  go.sum
  cmd/m0verify/main.go      # bringup-agnostic verifier: connects, applies, probes, reports
  cmd/m0verify/probes.go    # the probe table (below)
  scripts/m0_up.sh          # start node on 26260, create/reset database `fable`
  scripts/m0_down.sh        # stop node, keep data dir removable
  docs/M0_TRANSCRIPT.md     # generated: version, settings, SHOW CREATE, every probe receipt
  docs/M0_REPORT.md         # pass/fail summary + §9 findings
  plan.md                   # this plan, copied into the repo folder as requested
```

## The probe table

Every probe records: id, criterion text, statement, expected, observed, **SQLSTATE**,
**constraint name**, and the **raw driver message verbatim**. Each runs in its own
transaction; expected-failure probes are rolled back so nothing leaks forward.

**A — environment receipts**
- A1 `SELECT version()` → CockroachDB v26.2.x
- A2 `SHOW CLUSTER SETTING sql.txn.read_committed_isolation.enabled` → `true`
- A3 `BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED; SHOW transaction_isolation`
  → `read committed`. This is the real confirmation: it proves the transaction is **not**
  silently upgraded to SERIALIZABLE, which is what M3's cell 1 depends on.
- A4 `SHOW default_transaction_isolation` → `serializable` (records the default)

**B — schema application** (contract §7 M0: *"applies with zero errors"*)
- B1 Apply `db/001_schema.sql` statement-by-statement; any error aborts M0 immediately.
- B2 `SHOW CREATE TABLE` for all four tables, captured verbatim. This is the load-bearing
  receipt that CockroachDB actually preserved: the `gate` FK with `ON UPDATE CASCADE`, the
  `belief_id_status_key` UNIQUE, `promoted_is_debt_free`, `live_requires_promoted`, the
  `ARRAY[...]` default on `debt TEXT[]`, and the partial index `live_intents`.
- B3 Exactly four tables exist (contract §2).

**C — contract §7 M0 criteria, literally**
- C1 promoted-with-open-debt UPDATE → **23514**, constraint `promoted_is_debt_free`
- C2 `final_truth=true` + empty debt, then promote → **23514** (same constraint, I-2 arm)
- C3 debt-free, `final_truth=false`, promote → **succeeds**

**D — the §9 high-risk interaction** (the reason M0 exists)
- D1 INSERT `action_intent` citing an `entered` belief → **23503**, constraint `gate`
- D2 INSERT `action_intent` citing a `promoted` belief → **succeeds**
- D3 With a live intent outstanding, `UPDATE belief SET status='retracted'` → **must be
  refused**. Postgres gave `23514` / `live_requires_promoted` via cascade. Record whatever
  CockroachDB actually returns. If it *succeeds*, M0 has found the blocker and stops here.
- D4 Correct order in one transaction: cancel the intent, then retract. Must commit, and the
  surviving cancelled row must read `belief_status='retracted'` — proving `ON UPDATE CASCADE`
  propagated. (Postgres proof calls this "BUG-B FIX PASS".)
- D5 Audit `live` intents on non-promoted beliefs → **0**.

D3 and D4 together are the whole point: D3 shows the schema refuses the wrong order, D4 shows
it permits the right one. If either diverges from the Postgres transcript, stop and report.

## Go harness design

`cmd/m0verify` is the "connection smoke test" M0 permits, doing double duty as the acceptance
runner. It deliberately pins the driver stack M1 will inherit:

- `database/sql` + **`pgx/v5/stdlib`**, because `internal/kernel/kernel.go:153` classifies
  errors through an `interface{ SQLState() string }`. `*pgconn.PgError` implements that
  method; `lib/pq`'s `*pq.Error` does **not**. Choosing pgx here is what makes the kernel's
  existing error classification work later. The harness proves it end-to-end by asserting
  `errors.As(err, &pgErr)` yields `"23514"` on probe C1 — SQLSTATE, never substring matching
  (contract §4, I-7's sibling rule).
- No `crdb.ExecuteTx` in the probes. That wrapper retries and swallows; M0 needs the raw
  first error. `ExecuteTx` is a kernel-write requirement (I-7), and M0 writes no kernel.
- Flags: `--dsn`, `--apply` (run the DDL), `--reset` (drop+recreate `fable` first),
  `--out docs/M0_TRANSCRIPT.md`.
- Exit non-zero on the first failed criterion, printing exactly the three lines
  `prompt_pkg02.md` demands: **criterion / observed / expected**.

Build with `go build ./cmd/m0verify` rather than `./...`, so an unrelated compile problem in
the pre-supplied `internal/kernel` cannot masquerade as an M0 failure.

## §9 findings to report, not fix

Both are recorded in `docs/M0_REPORT.md` and left alone — fixing either would be redesign.

1. **`belief.embedding` disagreement.** Contract §3 says *"`belief.embedding` is nullable"* and
   I-6 references *"every `embedding` column NULL"*, but `db/001_schema.sql:3` states *"I-6: no
   embedding column"* and defines none. Per the authority clause this is a STOP-and-report
   item — but it blocks **no M0 criterion**, so M0 completes and the note goes to the
   architect for an M1/M4 ruling (M4 requires "merge duplicates via vector similarity", which
   will need a decision).
2. **`CREATE TEMP TABLE` in `RetractCascade`.** `internal/kernel/kernel.go:115` uses a temp
   table; CockroachDB gates temp tables behind `experimental_enable_temp_tables`. Observation
   only — M1's problem, flagged early so it is not discovered on demo day.

Anything else that surfaces gets the same treatment: named, quantified, escalated, not
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
  plus `belief_status='retracted'` on D4, and `0` on D5.
- `SHOW CREATE TABLE action_intent` in the transcript contains `ON UPDATE CASCADE`.
- Cross-check the D-series against `../plans/files/TRANSCRIPT.md`; any divergence from the
  PostgreSQL run is the §9 report, and M0 stops rather than proceeding to M1.

Independent spot-check, outside the harness:

```bash
cockroach sql --insecure --host=localhost:26260 -d fable \
  -e "SHOW CREATE TABLE action_intent;" \
  -e "SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id
      WHERE a.state='live' AND b.status<>'promoted';"
```

## Stop condition

When every M0 criterion is green, **stop**. Report the deliverables listed in
`prompt_pkg02.md` (schema applied, transcript, SQLSTATE receipts, version, READ COMMITTED
confirmation, blocker notes) and wait for the architect before M1.
