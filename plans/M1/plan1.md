# M1 Implementation Plan — Kernel Package Compiles — Revision 2

Revision 1 was **❌ REWORK REQUIRED** (`plans/M1/plan0_review.md`): one architectural blocker
(B1), two implementation blockers (B2, B3), plus the ruling on E-1. To be written into the
repo as `plans/M1/plan1.md`.

## What Revision 2 changes

| Ref | Finding | Change |
|---|---|---|
| **D-032** | `RetractCascade` **is scenario-scoped**; `scenarioID` is semantically meaningful, not symmetry | Both the recursive traversal *and* both UPDATEs are scoped to the scenario. §3 carries the SQL shape. |
| **D-033** | Temp-table approach retired; `WITH RECURSIVE` adopted as canonical | The draft's `CREATE TEMP TABLE` is superseded. |
| **B1 / D-034** | **Stubbing rejected.** "Do not carry intentional incompleteness into the codebase." | The stub and `ErrCascadeUnresolved` are **removed entirely**. M1 implements all seven §4 functions. §1's "6 of 7" caveat is withdrawn. |
| **B2 / D-035** | "No database contact" is an overreaction; require `Prepare()` if it can detect malformed SQL | **Researched and adopted** — `PREPARE` does validate. New `cmd/m1verify` runs a prepare sweep. This reverses my earlier "strict build+vet" decision; the reversal is deliberate and the evidence is in §4. |
| **B3 / D-036** | `contractV1` is version-in-the-type-name | Renamed to **`Contract`**. Of the two names offered, `Contract` is chosen over `KernelContract` because the package is already `kernel` — `kernel.Contract` reads clean, `kernel.KernelContract` stutters. |

---

# 1. Objective

M1 makes package `internal/kernel` the complete, compiling Go expression of contract §4
against the frozen schema, on the driver stack M0 pinned.

**All seven §4 functions are implemented.** Every write path routes through `crdb.ExecuteTx`
so serialization failures (40001) retry; error classification reads pgx SQLSTATE codes, never
error text. `go build ./...` and `go vet ./...` exit 0.

M1 adds two guarantees beyond "it compiles":

1. **Signatures are compile-enforced** — a wrong signature fails the build, not a review.
2. **Every kernel statement is PREPAREd against the live schema** — syntax, relation names,
   column names and placeholder types are validated before M2. Nothing is executed; no row is
   written.

M1 still asserts nothing about *behavior*. Proving the invariants is M2's job.

---

# 2. Scope

## In scope

| Artifact | Purpose |
|---|---|
| `internal/kernel/kernel.go` | `Store`, `New`, and all seven §4 functions |
| `internal/kernel/sql.go` | The statement catalog — one source of truth, used by the functions *and* the verifier |
| `internal/kernel/errors.go` | `ErrPromotionBlocked`, `ErrActionOnUnpromoted`, SQLSTATE classification |
| `internal/kernel/contract.go` | Compile-time assertion that every §4 signature exists exactly as written |
| `cmd/m1verify/main.go` | PREPARE sweep (B2). Thin: connect, prepare each statement, deallocate, report |
| `scripts/check_i7.sh` | Mechanical I-7 gate: zero raw write calls outside `crdb.ExecuteTx` |
| `scripts/m1_accept.sh` | Gate: isolation → build → vet → gofmt → I-7 → PREPARE sweep |
| `docs/M1_REPORT.md` | Closing report |

Also in scope: exported `ClaimType` with its three constants, and the exported full
starting-debt list (contract §4, "Exported value required").

**One deliberate addition beyond §4's list:** `kernel.SQLCatalog() []NamedSQL`, returning the
kernel's own statements sorted by name. It exists so `cmd/m1verify` can prepare *the actual
statements the kernel runs* rather than a hand-copied duplicate that would drift. It is
additive — it changes no §4 signature — and is documented in-code as verification support.
Flagged here rather than slipped in.

## Explicitly out of scope

- **Tests of any kind** — no `_test.go` files. The invariant suite is M2.
- **Execution.** The verifier PREPAREs and DEALLOCATEs. It runs no `INSERT`, `UPDATE`, `SELECT`
  or `EXECUTE`, and writes no row.
- Concurrency and isolation experiments (M3); evidence ingestor and Claim Agent (M4);
  Security Agent and demo path (M5); graph UI (M6); Bedrock; Lambda.
- **The embedding column (R2).** Contract §4 declares no embedding-bearing API, so M1 proceeds
  without it. Remains deferred to M4 per standing instruction.
- Modifying `db/001_schema.sql` (frozen) or `internal/m0/**` (frozen at M0 close, per O1).

---

# 3. API Surface

Package `internal/kernel`. Signatures and behavioral contracts only.

```go
var FullDebt = []string{
    "needMap", "needInvariant", "needToyCheck",
    "needNullModel", "needObstruction", "needFaithfulnessReview",
}

type ClaimType string
const (
    Derived      ClaimType = "derived"
    Accommodated ClaimType = "accommodated"
    Postulated   ClaimType = "postulated"
)

var ErrPromotionBlocked   = errors.New("promotion blocked: open debt or final-truth language")
var ErrActionOnUnpromoted = errors.New("action refused: belief is not promoted")

type Store struct{ /* unexported */ }
func New(db *sql.DB) *Store
```

| Function | Behavioral contract |
|---|---|
| `New(db *sql.DB) *Store` | Wraps an open pool pointed at a database with `db/001_schema.sql` applied. Does not ping, migrate, or validate. |
| `EnterBelief(ctx, scenarioID, claim string, ct ClaimType) (id string, err error)` | Inserts at the door: `status='entered'`, `debt` = the full starting debt, `final_truth` at its default. Never gated. Returns the new id via `RETURNING id`. |
| `AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error` | Inserts one evidence row. `contentSHA256` is required — the column is `NOT NULL` and the kernel substitutes no placeholder. Does not change belief state. |
| `RetireDebt(ctx, beliefID, item string) error` | Removes one debt item via `array_remove`. Idempotent when absent — zero rows changed is success. |
| `Promote(ctx, beliefID string) error` | Sets `status='promoted'`. On SQLSTATE **23514** returns an error satisfying `errors.Is(err, ErrPromotionBlocked)`. The gate is the database's. |
| `IntentOnPromoted(ctx, scenarioID, beliefID, action string) error` | Inserts a `live` intent. On SQLSTATE **23503** returns an error satisfying `errors.Is(err, ErrActionOnUnpromoted)`. |
| `RetractCascade(ctx, scenarioID, rootID string) (retracted int, err error)` | **Scenario-scoped (D-032).** In ONE transaction: compute the transitive descendants of `rootID` within `scenarioID`; set all `live` intents on those beliefs to `cancelled`; then set those beliefs from `promoted` to `retracted`. Cancel strictly precedes retract (I-8). Returns the count of beliefs retracted. |
| `AuditLiveOnNonPromoted(ctx, scenarioID string) (int, error)` | Invariant I-5 as a query, scoped to the scenario. Read path — may use `QueryRowContext` directly; I-7 governs writes. |

## `RetractCascade` — shape (D-032 + D-033)

`belief_edge` carries **no `scenario_id` column**, so scoping the traversal means constraining
each hop through `belief`. Both statements carry the same CTE:

```sql
WITH RECURSIVE d(id) AS (
    SELECT b.id FROM belief b WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
  UNION
    SELECT e.child_id
      FROM belief_edge e
      JOIN d          ON e.parent_id = d.id
      JOIN belief cb  ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
)
UPDATE action_intent SET state = 'cancelled'
 WHERE state = 'live' AND scenario_id = $2::UUID AND belief_id IN (SELECT id FROM d);
-- then, same CTE:
UPDATE belief SET status = 'retracted'
 WHERE status = 'promoted' AND scenario_id = $2::UUID AND id IN (SELECT id FROM d);
```

`UNION`, never `UNION ALL`. The docs warn that a recursive subquery must eventually return no
rows; `belief_edge`'s CHECK forbids only self-loops (`parent_id <> child_id`), not longer
cycles, so **`UNION`'s deduplication is what guarantees termination** on a cyclic belief graph.
This is load-bearing, not stylistic.

## Error semantics

Sentinels **wrap** rather than replace the driver error:

```go
if sqlState(err) == "23514" {
    return fmt.Errorf("%w: %w", ErrPromotionBlocked, err)
}
return err   // 40001 and everything else pass through untouched
```

Both `errors.Is(err, ErrPromotionBlocked)` and `errors.As(err, &pgErr)` then succeed — §4's
sentinel contract is met, and M5 can still surface the raw SQLSTATE and constraint name. The
wrap happens **only after** the code is confirmed, so a retryable 40001 is never masked from
`crdb`'s retry logic.

**The kernel imports no driver.** Classification goes through `interface{ SQLState() string }`,
which `*pgconn.PgError` satisfies. Accepted consequence: `ConstraintName` is a *field*, not a
method, so no interface reaches it without importing pgx — M1 classifies on SQLSTATE alone,
which is what §4 requires. See M1-R6.

## Signature assertion (B3)

```go
type Contract interface {
    EnterBelief(context.Context, string, string, ClaimType) (string, error)
    AddEvidence(context.Context, string, string, string, string, string) error
    RetireDebt(context.Context, string, string) error
    Promote(context.Context, string) error
    IntentOnPromoted(context.Context, string, string, string) error
    RetractCascade(context.Context, string, string) (int, error)
    AuditLiveOnNonPromoted(context.Context, string) (int, error)
}
var _ Contract = (*Store)(nil)
var _ func(*sql.DB) *Store = New
```

---

# 4. CockroachDB considerations

| # | Behavior M1 depends on | Status | Evidence |
|---|---|---|---|
| C1 | `crdb.ExecuteTx` issues `SAVEPOINT cockroach_restart` / `RELEASE`, retrying only on 40001 | **Verified** | Docs: retry savepoints supported in v26.2, `RELEASE` acts as commit. Source: `crdb/common.go` `ExecuteInTx`, `crdb/tx.go` `errIsRetryable` |
| C2 | A non-retryable error returned from the closure propagates **unwrapped** | **Verified** | `ExecuteInTx` returns `err` verbatim when `!errIsRetryable(err)`. This is what makes `errors.Is` on the sentinel work. |
| C3 | `crdb` detects 40001 via `errors.As` on `interface{ SQLState() string }` | **Verified** | `crdb/tx.go` `errCode`/`errWithSQLState` |
| C4 | `array_remove(STRING[], STRING)` supported, immutable | **Verified** | Docs: array functions table; worked `UPDATE … SET col=array_remove(col,'x')` example |
| C5 | Promotion with open debt / final truth fails **23514** on `promoted_is_debt_free` | **Verified** | M0 probes C1, C2 |
| C6 | Intent citing a non-promoted belief fails **23503** on `gate` | **Verified** | M0 probe D1 |
| C7 | `*pgconn.PgError` carries a populated `ConstraintName` and satisfies `SQLState()` | **Verified** | M0 transcript: "(structured field)" on C1/C2/D1/D3 |
| C8 | `pgx/v5/stdlib` passes arbitrary Go types through `database/sql` to pgx | **Verified** | `stdlib/sql.go:548` — `CheckNamedValue` returns `nil` for everything |
| C9 | A `WITH RECURSIVE` CTE may prefix an `UPDATE` and compute transitive descendants | **Verified** | M0 probe D4 committed using exactly this form |
| C10 | `UNION` (not `UNION ALL`) terminates the traversal on a cyclic graph | **Verified** | Docs: "The `UNION` variant deduplicates rows"; "recursive subqueries must eventually return no results" |
| **C11** | **`PREPARE` validates without executing**, resolving relations, columns and placeholder types | **Verified** | CRDB issue tracker, its own repro sessions: `PREPARE … AS … FROM stock` → `(42P01) relation "stock" does not exist`; `PREPARE p1(INT) AS SELECT $1:::DECIMAL` → `42804`; `PREPARE … = $1` → `42P18 could not determine data type of placeholder`. Stack traces show `conn_executor_prepare.go → populatePrepared → prepareUsingOptimizer → optbuilder.Build` — preparation runs the full optimizer build. |
| **C12** | `PREPARE` accepts all five kernel write statements | **Verified** | All are `INSERT`/`UPDATE` (incl. CTE-prefixed `UPDATE`), which is precisely the statement class `PREPARE` accepts. |
| **C13** | What `PREPARE` does **not** catch | **Verified — stated for honesty** | Constraint violations, row counts, ordering, and all semantics. 23514/23503 arise only at execution. M1's sweep is a *SQL compile check*, not an invariant check. |
| C14 | The recursive term may join a second table (`belief cb`) to scope by scenario | **Assumption → discharged at M1** | Standard SQL and not prohibited by the docs; M0 D4 already joined `belief_edge e JOIN d`. The scenario join is new — and the PREPARE sweep validates it structurally at M1 rather than M2. |
| C15 | pgx encodes a Go `[]string` as `STRING[]` for `debt` | **Assumption** | C8 proves it is offered to pgx; correct encoding is unproven without execution, and `PREPARE` reports only the *expected* parameter type. Discharged by M2's first `EnterBelief` test. Pre-decided fallback: pass `'{…}'::TEXT[]`. |
| C16 | `RETURNING id` returns the generated UUID | **Assumption** | Preparable and near-certain; the returned *value* is unexercised until M2. |
| C17 | Concurrency, contention, isolation-level behavior | **Out of scope** | M3 |
| C18 | Vector/embedding types | **Out of scope** | M4; R2 deferred |

**On B2's "if CockroachDB cannot validate without execution, state that explicitly":** it can.
The evidence is C11–C13. Preparation is a real type-check against the live catalog, it is
cheap (one connection, ~8 statements, no transaction), and it moves an entire class of failure
— typo'd column, wrong arity, uninferable placeholder — from M2 back to M1. C13 records its
ceiling honestly so the sweep is not mistaken for the invariant suite.

---

# 5. Implementation order

Every step leaves the tree buildable. Baseline is green: `go vet ./...` exits 0 today.

| Step | Work | Buildable after? |
|---|---|---|
| **S1** | `errors.go`: the two sentinels, `sqlState(error) string`, `wrapIf(code, sentinel, err)` | ✅ |
| **S2** | `kernel.go` skeleton: package doc, `ClaimType` + constants, `FullDebt`, `Store`, `New` | ✅ |
| **S3** | `sql.go`: named statement consts + `NamedSQL` + `SQLCatalog()` (sorted) | ✅ |
| **S4** | `EnterBelief` — `ExecuteTx` + `INSERT … RETURNING id` | ✅ |
| **S5** | `AddEvidence` — `ExecuteTx` + `INSERT` | ✅ |
| **S6** | `RetireDebt` — `ExecuteTx` + `array_remove` | ✅ |
| **S7** | `Promote` — `ExecuteTx` + 23514 wrap | ✅ |
| **S8** | `IntentOnPromoted` — `ExecuteTx` + 23503 wrap | ✅ |
| **S9** | `AuditLiveOnNonPromoted` — read path, no `ExecuteTx` | ✅ |
| **S10** | **`RetractCascade`** — one `ExecuteTx`, scenario-scoped CTE, cancel-then-retract, `RowsAffected` | ✅ |
| **S11** | `contract.go` — `Contract` interface + assertions. **A wrong signature fails the build here.** | ✅ |
| **S12** | `cmd/m1verify` — PREPARE sweep over `SQLCatalog()` | ✅ |
| **S13** | `scripts/check_i7.sh` — static I-7 gate | ✅ |
| **S14** | `scripts/m1_accept.sh` — full gate | ✅ |
| **S15** | `docs/M1_REPORT.md` | ✅ |

**S12's mechanism.** For each catalog entry, on one connection against a schema-applied
database:

```sql
PREPARE m1_<name> AS <statement>;
DEALLOCATE m1_<name>;
```

Report per statement: name, prepared yes/no, SQLSTATE and message on failure. Non-zero exit on
the first failure, printing criterion / observed / expected in the M0 house style. No
transaction is opened; nothing is executed.

**S13's mechanism.** I-7 is a *grep* criterion in contract §7, so it gets a real grep: flag any
`db.Exec`/`db.Query` write in `internal/kernel`, allow `QueryRowContext` only on the audit read
path, and require every `INSERT`/`UPDATE`/`DELETE` in the package to sit inside an `ExecuteTx`
closure.

---

# 6. Acceptance criteria

| Step | Measurable criterion |
|---|---|
| S1 | `go build ./internal/kernel` = 0; both sentinels exported |
| S2 | `FullDebt` has exactly 6 items, string-equal and in the same order as the `ARRAY[...]` default in `db/001_schema.sql`. The three `ClaimType` constants match `check_claim_type` exactly. |
| S3 | `SQLCatalog()` returns one entry per kernel statement (expected 8: 5 writes, 1 read, 2 cascade), sorted by name, deterministic across calls |
| S4–S9 | After each step `go build ./...` and `go vet ./...` = 0 |
| S7 / S8 | Wrap occurs only after the SQLSTATE is confirmed; a 40001 returns the original error unwrapped (code inspection — no test file in M1) |
| S9 | `AuditLiveOnNonPromoted` performs no write and is the only function not using `ExecuteTx` |
| **S10** | One `ExecuteTx` closure; the cancel statement textually precedes the retract statement; both carry `scenario_id = $2`; the CTE uses `UNION`; `retracted` comes from the belief UPDATE's `RowsAffected`. **No `CREATE TEMP TABLE` anywhere in the package** (grep = 0 hits, per D-033). |
| S11 | The package compiles **with** `contract.go` present — the mechanical form of §7 M1's "Every §4 function exists with the stated signature" |
| **S12** | `cmd/m1verify` prepares **all 8** statements with zero failures against a schema-applied database. Any `42P01`/`42703`/`42P18`/`42804` fails M1. |
| S13 | `./scripts/check_i7.sh` = 0, reporting 6 `ExecuteTx`-wrapped write sites (Enter, AddEvidence, RetireDebt, Promote, IntentOnPromoted, RetractCascade) and 0 raw writes |
| S14 | `./scripts/m1_accept.sh` = 0 end to end |
| **M1 overall** | `go build ./...` = 0; `go vet ./...` = 0; `gofmt -l cmd internal` empty; PREPARE sweep green; `./scripts/m1_accept.sh` = 0; `docs/M1_REPORT.md` present |

Explicitly **not** an M1 criterion: that any statement produces correct results, that any
invariant holds, or that any row is written. Those are M2.

---

# 7. Risks introduced by M1

M0's retired risks are not repeated. R2 (embedding) is a carried finding, not an M1 risk. The
Revision-1 stub risk is **withdrawn** — D-034 removed the stub.

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| **M1-R1** | `PREPARE` validates syntax and names but **not semantics** (C13). A statement that prepares cleanly can still violate a constraint, update the wrong rows, or return the wrong count. | Medium | Stated in the report so a green sweep is never read as "the kernel works". M2 remains the behavioral gate. |
| **M1-R2** | `[]string` → `STRING[]` encoding is C15, an Assumption that PREPARE cannot discharge. `EnterBelief` is the first call in every flow, so a failure blocks all of M2 at once. | Medium | Discharge as M2's first test. Fallback pre-decided: `'{…}'::TEXT[]`. |
| **M1-R3** | `FullDebt` (Go) and the `debt` DEFAULT (DDL) encode the same six items in two places and can drift silently. | Medium | S2 requires byte-equality at authoring time; M2 should assert a belief inserted *without* an explicit list equals `FullDebt`. |
| **M1-R4** | **Scenario-scoped cascade can be blocked by a foreign-scenario intent.** Under D-032 the cancel skips a `live` intent in another scenario pointing at a belief being retracted; the belief UPDATE then fails 23514 on `live_requires_promoted` and the whole transaction rolls back. | Medium | This is a *refusal, not corruption* — the schema still holds the line, and the caller sees a loud error. Recorded as a direct consequence of D-032. If cross-scenario edges are ever created, this surfaces immediately rather than silently. |
| **M1-R5** | Recursive traversal now joins `belief` twice per hop (C14). Wrong join predicates would silently under- or over-collect descendants — and the count is `RetractCascade`'s return value. | Medium | PREPARE catches structural errors at M1; M2 must assert the descendant set explicitly, including a belief in a *different* scenario that must not be retracted. |
| **M1-R6** | 23514 → `ErrPromotionBlocked` assumes `promoted_is_debt_free` is the only CHECK reachable from `UPDATE belief SET status='promoted'`. True against the frozen schema (`check_status` is satisfied by the literal; `check_claim_type` is untouched). | Low | Recorded; the schema is frozen. Revisit only on a re-freeze — e.g. if R2 adds a column. |
| **M1-R7** | The multi-`%w` wrap sits above the driver error. A refactor that moved it before the SQLSTATE check would mask 40001 and silently stop `crdb` retrying. | Low | The helper takes the expected code as a parameter, so ordering is structural, not conventional. |
| **M1-R8** | `SQLCatalog()` widens the kernel's exported surface beyond §4 for verification's sake. | Low | Single source of truth — the same consts back the functions, so it cannot drift. Documented in-code as verification support. Withdrawable after M2 without touching any §4 signature. |
| **M1-R9** | The PREPARE sweep needs a running cluster, so `m1_accept.sh` is no longer purely static. | Low | `scripts/m0_up.sh` already provides the cluster. The sweep is skippable with an explicit flag for a DB-less environment, but **skipping is reported loudly** and does not count as green. |

---

# 8. Escalations

## E-1 — RESOLVED

Ruled in `plans/M1/plan0_review.md`:

- **D-032** — `RetractCascade` is scenario-scoped; both traversal and updates are scoped.
  Implemented per §3.
- **D-033** — the temp-table approach is retired; `WITH RECURSIVE` as demonstrated by M0 D4 is
  the canonical implementation. The draft `internal/kernel/kernel.go:115` is superseded, and
  S10's acceptance greps for zero `CREATE TEMP TABLE` occurrences.

No stub. M1 implements the real function (D-034).

## E-2 — Carried, non-blocking

- **R2 (`belief.embedding`)** — contract §3 and the frozen DDL still disagree. Contract §4
  declares no embedding-bearing API, so M1 is not blocked; deferred to M4 per standing
  instruction. Restated only so it is not lost.
- **M0 §3.1 (CHECK messages omit the constraint name)** — still awaiting the M5 ruling on
  whether `ConstraintName` may be displayed beside a raw error. M1's decision to keep the
  kernel driver-agnostic means `ConstraintName` is not reachable from kernel errors; if M5
  requires it, that is a small deliberate change at the `cmd/` edge, not in the kernel.

No new conflict with `IMPLEMENTATION_CONTRACT.md`, the M0 findings, or observed CockroachDB
behavior was found.

---

# Verification

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
./scripts/m0_up.sh          # cluster on 26260, database `fable` (schema applied)
./scripts/m1_accept.sh      # isolation → build → vet → gofmt → I-7 → PREPARE sweep
echo "exit=$?"              # 0 == every M1 criterion green
```

Then confirm by reading, not by trusting the exit code:

- `internal/kernel/contract.go` compiles — that *is* the signature proof.
- `./scripts/check_i7.sh` reports 6 `ExecuteTx`-wrapped write sites and 0 raw writes.
- `RetractCascade` contains `UNION` (not `UNION ALL`), carries `scenario_id = $2` on both
  UPDATEs, cancels before it retracts, and contains no `CREATE TEMP TABLE`.
- The sweep output lists all 8 statements prepared, none skipped.

Stop at M1's close. Do not begin M2.
