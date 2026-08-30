# M1 Report — Kernel Package Compiles

Work Package 03. Companion to `docs/M1_BUILD.md`, `docs/M1_I7.md` and
`docs/M1_PREPARE.md`, which hold the receipts. This document is the argument.

---

## 1. Verdict

**M1 is GREEN.** `./scripts/m1_accept.sh` exits 0.

| Deliverable (`plans/M1/plan1_review.md`) | Status |
|---|---|
| `M1_REPORT.md` | ✅ this document |
| PREPARE transcript | ✅ `docs/M1_PREPARE.md` — **9/9** statements prepared, none skipped |
| Build transcript | ✅ `docs/M1_BUILD.md` — `go build ./...` exit **0** |
| Vet transcript | ✅ `docs/M1_BUILD.md` — `go vet ./...` exit **0**; `gofmt -l` empty |
| I-7 report | ✅ `docs/M1_I7.md` — **7** `ExecuteTx` write sites, **0** raw writes |
| Remaining blockers | **None new.** Two carried items in §5. |

All **eight** contract §4 functions are implemented. There is no stub (D-034).

---

## 2. What M1 delivers

### The kernel

`internal/kernel` is now the complete Go expression of contract §4:

| File | Role |
|---|---|
| `kernel.go` | `Store`, `New`, and all eight §4 functions |
| `sql.go` | Every statement the kernel runs — one source of truth |
| `errors.go` | The two sentinels, SQLSTATE classification, `wrapIf` |
| `contract.go` | `Contract` interface + compile-time assertions |

The pre-supplied draft is fully superseded. It was never a frozen input — contract §1 names
only `IMPLEMENTATION_CONTRACT.md` and `db/001_schema.sql` — and M0 had already proved one of
its statements could not run on the target engine.

### Signatures are compile-enforced, not reviewed

`contract.go` transcribes §4 as a Go interface and asserts `var _ Contract = (*Store)(nil)`.
A signature that drifts from the contract now fails `go build`. This is the mechanical form of
§7 M1's *"Every §4 function exists with the stated signature"* — the criterion is satisfied by
the compiler, not by someone reading a table.

### I-7 is enforced by a gate, not a convention

`scripts/check_i7.sh` refuses the build unless: zero raw `s.db.Exec`/`s.db.Query` calls; exactly
one pool-level read, and it must belong to `AuditLiveOnNonPromoted`; exactly seven
`crdb.ExecuteTx` sites; and zero `CREATE TEMP TABLE` (D-033).

**The gate was tested against three deliberate violations**, not merely observed to pass — a
raw pool write, a write path with its `ExecuteTx` removed, and a reintroduced
`CREATE TEMP TABLE`. All three were caught. A check that has never failed is not yet evidence.

### The rulings, as implemented

- **D-032 — scenario-scoped cascade.** `belief_edge` carries no `scenario_id`, so scoping the
  *traversal* is not a WHERE clause: each hop joins the child back to `belief` and requires
  `cb.scenario_id = $2`. Both UPDATEs carry `scenario_id = $2` as well. Traversal and updates
  are scoped, exactly as ruled.
- **D-033 — `WITH RECURSIVE` canonical.** The temp table is gone; the cascade uses the form M0
  probe D4 proved. `CREATE TEMP TABLE` count in the package: 0, enforced by the gate.
- **I-8 — ordering.** Cancel (`kernel.go:128`) textually precedes retract (`kernel.go:132`)
  inside a single `crdb.ExecuteTx` closure opened at `kernel.go:126`. One transaction, cancel
  first.
- **N3.** `retracted` is the belief UPDATE's `RowsAffected` — belief rows retracted, never
  cancelled intents. Stated in the doc comment.

---

## 3. PREPARE validation (D-035, D-039)

All 9 catalog statements prepare cleanly against the schema-applied database, and are
deallocated immediately. Nothing was executed; no transaction was opened; no row was written.

**What this proves.** Preparation runs CockroachDB's optimizer build, so it resolves relation
names, column names, and placeholder types. A typo'd column, a wrong arity, or a placeholder
the planner cannot type would have failed here.

**The negative control matters more than the pass.** A sweep that has never rejected anything
proves nothing about its own sensitivity, so the mechanism was exercised directly:

```sql
PREPARE neg_control AS UPDATE belief SET status='promoted' WHERE no_such_column = $1::UUID;
ERROR: column "no_such_column" does not exist
SQLSTATE: 42703
```

`PREPARE` does reject. The sweep is real.

**What this does not prove — and must not be read as proving.** Constraint violations, row
counts, ordering, and every other semantic. 23514 and 23503 arise only at execution.
**The PREPARE sweep is a SQL compile check, not an invariant check.** M2 remains the
behavioral gate, and a green sweep must never be reported as "the kernel works".

### One assumption partially strengthened

C15 (pgx encoding a Go `[]string` as `STRING[]`) was carried into M1 as an Assumption.
Preparation now pins the *expected* type from the database's side:

```
name | parameter_types
pt   | {uuid,text,text,text[]}
```

`$4` is `text[]`, which is what `EnterBelief` must supply. This narrows the risk to the driver's
encoding step alone; it does not close it, because nothing was executed. C15 remains open and
is M2's first test.

---

## 4. Deviations from the plan

**None.** Revision 3 was implemented as written. Two mechanical affordances were added while
building, neither changing behavior:

1. `check_i7.sh` accepts `I7_PKG`/`I7_OUT` overrides so the gate could be run against doctored
   copies for the negative controls above. Defaults are unchanged.
2. The I-7 report's write-site table names the statement constant at each site
   (`sqlRetractCascadeCancel`, …) rather than a truncated source line. The first version
   truncated at the first comma and was unreadable.

---

## 5. Carried findings (unchanged, non-blocking)

- **R2 — `belief.embedding`.** Contract §3 and the frozen DDL still disagree. Contract §4
  declares no embedding-bearing API, so M1 was not blocked. Deferred to M4 per standing
  instruction. **Note for whoever rules on it:** M0 recorded that all four tables carry
  `schema_locked = true`, so adding the column is a two-step `ALTER` on this engine.
- **M0 §3.1 — CHECK messages omit the constraint name.** Still awaiting the M5 ruling on
  whether `ConstraintName` may be displayed beside a raw error. M1 kept the kernel
  driver-agnostic (classification via `interface{ SQLState() string }`, no pgx import), so
  `ConstraintName` is not reachable from a kernel error. If M5 requires it, that is a small
  deliberate change at the `cmd/` edge — not in the kernel.

No new conflict with `IMPLEMENTATION_CONTRACT.md`, the M0 findings, or observed CockroachDB
behavior was found.

---

## 6. Risks still open at M1's close

These are M1's own risks, carried into M2. M0's retired risks are not repeated.

| # | Risk | Discharged by |
|---|---|---|
| M1-R1 | PREPARE validates structure, not semantics | M2 — the invariant suite |
| M1-R2 | `[]string` → `text[]` encoding (narrowed in §3, not closed) | M2's first `EnterBelief` test; fallback `'{…}'::TEXT[]` pre-decided |
| M1-R3 | `FullDebt` (Go) vs the `debt` DEFAULT (DDL) can drift | M2 should assert a belief inserted without an explicit list equals `FullDebt` |
| M1-R4 | A foreign-scenario `live` intent can block a scoped cascade — it fails 23514 and rolls back | Consequence of D-032. A **refusal, not corruption**; M2 should assert the failure mode explicitly |
| M1-R5 | The traversal joins `belief` per hop; wrong predicates would silently mis-collect descendants, and that count is the return value | M2 must assert the descendant set, **including a belief in another scenario that must not be retracted** |
| M1-R6 | 23514 → `ErrPromotionBlocked` assumes `promoted_is_debt_free` is the only CHECK reachable from the promote statement | Holds against the frozen schema; revisit only on a re-freeze |
| M1-R7 | The `%w` wrap must stay below the SQLSTATE check or 40001 would be masked from retry | Structural: `wrapIf` takes the expected code as a parameter |
| M1-R8 | `SQLCatalog()` widens the exported surface beyond §4 | Marked *"Verification support. Not part of the public kernel API."* in source (N1); withdrawable after M2 |

---

## 7. Reproduce

```bash
./scripts/m0_up.sh          # single-node v26.2.0 on 26260, database `fable`
./scripts/m1_accept.sh      # isolation → build → vet → gofmt → I-7 → PREPARE sweep
```

---

## 8. Stop

M1's acceptance criteria are met. Per D-041 and the review's closing instruction, work stops
here and awaits architectural review. **M2 has not been started.**
