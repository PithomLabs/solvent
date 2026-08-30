# M1 Implementation Plan — Kernel Package Compiles

Supersedes the M0 plan (M0 closed green; see `docs/M0_REPORT.md`). To be written into the
repo as `plans/M1/plan.md`.

**Authority note.** `internal/kernel/kernel.go` exists in the tree as a pre-supplied draft. It
is **not** a frozen input — contract §1 names only `IMPLEMENTATION_CONTRACT.md` and
`db/001_schema.sql`. Where the draft and contract §4 differ, §4 wins. The draft is a starting
point, and M0 already proved one of its statements cannot run on CockroachDB (R3).

---

# 1. Objective

M1 makes package `internal/kernel` the complete, compiling Go expression of contract §4
against the frozen schema, on the driver stack M0 pinned.

Concretely: every §4 function exists with its stated signature; every write path routes
through `crdb.ExecuteTx` so serialization failures (40001) retry; error classification reads
pgx SQLSTATE codes, never error text. `go build ./...` and `go vet ./...` exit 0.

M1 asserts nothing about runtime behavior. Proving the invariants is M2's job. The one
guarantee M1 adds beyond "it compiles" is that the **signatures** are compile-enforced, not
eyeballed.

**M1 will not fully satisfy contract §6's "Implement all of §4."** Per the ruling recorded in
§8, `RetractCascade` is declared with its exact signature but left unimplemented pending
architectural review. M1 therefore closes as **6 of 7 functions implemented, 1 declared and
blocked.** That is stated up front rather than buried.

---

# 2. Scope

## In scope

| Artifact | Purpose |
|---|---|
| `internal/kernel/kernel.go` | `Store`, `New`, and the six implementable §4 functions |
| `internal/kernel/errors.go` | `ErrPromotionBlocked`, `ErrActionOnUnpromoted`, SQLSTATE classification |
| `internal/kernel/contract.go` | Compile-time assertions that every §4 signature exists exactly as written |
| `scripts/check_i7.sh` | Mechanical I-7 gate: zero raw write calls outside `crdb.ExecuteTx` |
| `scripts/m1_accept.sh` | Static gate: isolation → build → vet → I-7 → (signatures via build) |
| `docs/M1_REPORT.md` | Closing report, including the §8 escalation |

Also in scope: the exported `ClaimType` type with its three constants, and the exported full
starting-debt list (contract §4, "Exported value required").

## Explicitly out of scope

- **`RetractCascade`'s body** — declared only; blocked on the §8 ruling.
- **Any database contact.** M1 runs no statement, opens no connection, inserts no row.
  `scripts/m1_accept.sh` is static analysis only. *(Decided: strict build+vet reading. A
  PREPARE-based SQL validator was considered and rejected as scope drift; the cost is
  recorded as risk M1-R4.)*
- **Tests of any kind** — no `_test.go` files. The invariant suite is M2.
- Concurrency and isolation experiments (M3); evidence ingestor and Claim Agent (M4);
  Security Agent and demo path (M5); graph UI (M6); Bedrock; Lambda.
- **The embedding column (R2).** Contract §4 declares no embedding-bearing API, so M1
  proceeds without it. Remains deferred to M4 per standing instruction.
- Modifying `db/001_schema.sql` (frozen) or `internal/m0/**` (frozen at M0 close, per O1).

---

# 3. API Surface

Package `internal/kernel`. Signatures and behavioral contracts only.

```go
// Exported values
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

// Exported sentinels
var ErrPromotionBlocked    = errors.New("promotion blocked: open debt or final-truth language")
var ErrActionOnUnpromoted  = errors.New("action refused: belief is not promoted")

type Store struct{ /* unexported */ }

func New(db *sql.DB) *Store
```

| Function | Behavioral contract |
|---|---|
| `New(db *sql.DB) *Store` | Wraps an open pool pointed at a database with `db/001_schema.sql` applied. Does not ping, migrate, or validate. |
| `EnterBelief(ctx, scenarioID, claim string, ct ClaimType) (id string, err error)` | Inserts at the door: `status='entered'`, `debt` = the full starting debt, `final_truth` left at its default. Never gated. Returns the new id via `RETURNING id`. |
| `AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) error` | Inserts one evidence row. `contentSHA256` is required — the column is `NOT NULL` and the kernel does not substitute a placeholder. Does not change belief state. |
| `RetireDebt(ctx, beliefID, item string) error` | Removes one debt item via `array_remove`. Idempotent when the item is absent (zero rows changed is success, not an error). |
| `Promote(ctx, beliefID string) error` | Sets `status='promoted'`. On SQLSTATE **23514** returns an error satisfying `errors.Is(err, ErrPromotionBlocked)`. The gate is the database's; this function only names the refusal. |
| `IntentOnPromoted(ctx, scenarioID, beliefID, action string) error` | Inserts a `live` intent. On SQLSTATE **23503** returns an error satisfying `errors.Is(err, ErrActionOnUnpromoted)`. |
| `AuditLiveOnNonPromoted(ctx, scenarioID string) (int, error)` | Invariant I-5 as a query: count of `live` intents whose belief is not `promoted`, scoped to the scenario. Read path — may use `QueryRowContext` directly (I-7 governs writes). |
| `RetractCascade(ctx, scenarioID, rootID string) (retracted int, err error)` | **Declared, not implemented in M1.** Returns `(0, ErrCascadeUnresolved)` where `ErrCascadeUnresolved` documents the pending ruling. See §8. |

## Error semantics (decided)

Sentinels **wrap** rather than replace the driver error:

```go
if sqlState(err) == "23514" {
    return fmt.Errorf("%w: %w", ErrPromotionBlocked, err)
}
return err   // 40001 and everything else pass through untouched
```

Both `errors.Is(err, ErrPromotionBlocked)` and `errors.As(err, &pgErr)` then succeed — §4's
sentinel contract is met, and M5 can still surface the raw SQLSTATE and constraint name. The
wrap happens **only after** the code is confirmed, so a retryable 40001 never gets masked from
`crdb`'s retry logic. Multi-`%w` requires Go ≥1.20; the module is on 1.25.

**The kernel imports no driver.** Classification goes through
`interface{ SQLState() string }`, which `*pgconn.PgError` satisfies. The driver is chosen at
the edge (`cmd/`), not in the kernel. Consequence, accepted: `ConstraintName` is a *field* on
`pgconn.PgError`, not a method, so no interface can reach it without importing pgx — M1
classifies on SQLSTATE alone, which is what §4 requires. See risk M1-R6 for why that is
unambiguous against this schema.

---

# 4. CockroachDB considerations

| # | Behavior M1 depends on | Status | Evidence |
|---|---|---|---|
| C1 | `crdb.ExecuteTx` issues `SAVEPOINT cockroach_restart` / `RELEASE SAVEPOINT`, and retries only on 40001 | **Verified** | Docs: retry savepoints supported in v26.2, `RELEASE` acts as commit. Source: `crdb/common.go` `ExecuteInTx`, `crdb/tx.go` `errIsRetryable` |
| C2 | A non-retryable error returned from the `ExecuteTx` closure propagates to the caller **unwrapped** | **Verified** | `ExecuteInTx` returns `err` verbatim when `!errIsRetryable(err)`; deferred `Rollback` releases the conn. This is what makes `errors.Is` on the sentinel work. |
| C3 | `crdb` detects 40001 via `errors.As` on `interface{ SQLState() string }` | **Verified** | `crdb/tx.go` `errCode`/`errWithSQLState`. Confirms the multi-`%w` wrap must not sit above a 40001 — it does not, by construction. |
| C4 | `array_remove(STRING[], STRING)` is supported and immutable | **Verified** | Docs: array functions table; worked `UPDATE … SET col=array_remove(col,'x')` example |
| C5 | Promotion with open debt / final truth fails with **23514** on `promoted_is_debt_free` | **Verified** | M0 probes C1, C2 |
| C6 | Insert of an intent citing a non-promoted belief fails with **23503** on `gate` | **Verified** | M0 probe D1 |
| C7 | `*pgconn.PgError` carries a populated `ConstraintName` and satisfies `SQLState()` | **Verified** | M0 transcript: "(structured field)" on C1/C2/D1/D3 |
| C8 | `pgx/v5/stdlib` passes arbitrary Go types through `database/sql` to pgx's type map | **Verified** | `stdlib/sql.go:548` — `CheckNamedValue` returns `nil` for everything |
| C9 | pgx encodes a Go `[]string` as `STRING[]` for the `debt` column | **Assumption** | C8 proves it is *offered* to pgx; that pgx maps it to `text[]` is unproven here because M1 makes no DB contact. Discharged by M2's first `EnterBelief` test. Fallback if it fails: pass a `'{a,b,c}'::TEXT[]` literal. |
| C10 | `RETURNING id` inside a transaction returns the generated UUID | **Assumption** | Standard and near-certain; unexercised in M0 and not exercised in M1. Discharged at M2. |
| C11 | `CREATE TEMP TABLE` (draft `RetractCascade`) is refused | **Verified — will not be used** | M0: `SQLSTATE XCEXF`, `experimental_enable_temp_tables = off`. Moot for M1 given the stub; part of the §8 ruling. |
| C12 | A `WITH RECURSIVE` CTE may prefix an `UPDATE`, computing transitive descendants | **Verified** | M0 probe D4 committed using exactly this form. Relevant only once §8 is ruled. |
| C13 | Concurrency, contention, isolation-level behavior | **Out of scope** | M3. |
| C14 | Vector/embedding types | **Out of scope** | M4; R2 deferred. |

---

# 5. Implementation order

Every step leaves the tree buildable. Baseline is already green: `go vet ./...` exits 0 today.

| Step | Work | Buildable after? |
|---|---|---|
| **S1** | `errors.go`: the two sentinels, `ErrCascadeUnresolved`, `sqlState(error) string` via the `SQLState()` interface, and `wrapIf(code, sentinel, err)` | ✅ (compiles standalone) |
| **S2** | `kernel.go` skeleton: package doc, `ClaimType` + constants, `FullDebt`, `Store`, `New` | ✅ |
| **S3** | `EnterBelief` — `ExecuteTx` + `INSERT … RETURNING id` | ✅ |
| **S4** | `AddEvidence` — `ExecuteTx` + `INSERT` | ✅ |
| **S5** | `RetireDebt` — `ExecuteTx` + `array_remove` | ✅ |
| **S6** | `Promote` — `ExecuteTx` + 23514 wrap | ✅ |
| **S7** | `IntentOnPromoted` — `ExecuteTx` + 23503 wrap | ✅ |
| **S8** | `AuditLiveOnNonPromoted` — read path, no `ExecuteTx` | ✅ |
| **S9** | `RetractCascade` — signature + doc comment citing §8; body returns `(0, ErrCascadeUnresolved)` | ✅ |
| **S10** | `contract.go` — compile-time signature assertions. **A wrong signature fails the build here.** | ✅ (or fails loudly, which is the point) |
| **S11** | `scripts/check_i7.sh` — static I-7 gate | ✅ |
| **S12** | `scripts/m1_accept.sh` — static gate chaining isolation, build, vet, I-7 | ✅ |
| **S13** | `docs/M1_REPORT.md` — closing report and escalation | ✅ |

S10's mechanism, so "signatures exist" is enforced rather than reviewed:

```go
type contractV1 interface {
    EnterBelief(context.Context, string, string, ClaimType) (string, error)
    AddEvidence(context.Context, string, string, string, string, string) error
    RetireDebt(context.Context, string, string) error
    Promote(context.Context, string) error
    IntentOnPromoted(context.Context, string, string, string) error
    RetractCascade(context.Context, string, string) (int, error)
    AuditLiveOnNonPromoted(context.Context, string) (int, error)
}
var _ contractV1 = (*Store)(nil)
var _ func(*sql.DB) *Store = New
```

S11's mechanism — I-7 is a *grep* criterion in contract §7, so it gets a real grep. The check
flags any `db.Exec`/`db.Query` write call in `internal/kernel`, allowing `QueryRowContext` only
on the audit read path, and requires every `INSERT`/`UPDATE`/`DELETE` literal in the package to
sit inside an `ExecuteTx` closure.

---

# 6. Acceptance criteria

| Step | Measurable criterion |
|---|---|
| S1 | `go build ./internal/kernel` exits 0. `ErrPromotionBlocked`, `ErrActionOnUnpromoted` exported. |
| S2 | `FullDebt` has exactly 6 items, string-equal to the `db/001_schema.sql` `ARRAY[...]` default, in the same order. Three `ClaimType` constants match the schema's `check_claim_type` values exactly. |
| S3–S8 | After each step `go build ./...` and `go vet ./...` exit 0. Each function's SQL names only columns present in `db/001_schema.sql` (verified by reading the DDL, not by executing). |
| S6 | `Promote` wraps only on 23514; a synthetic 40001 path returns the original error unwrapped (verified by code inspection — no test file in M1). |
| S7 | `IntentOnPromoted` wraps only on 23503, same condition. |
| S8 | `AuditLiveOnNonPromoted` performs no write and is the only function not using `ExecuteTx`. |
| S9 | `RetractCascade` exists with signature `(context.Context, string, string) (int, error)` and returns a non-nil error that `errors.Is`-matches `ErrCascadeUnresolved`. It performs no database call. |
| S10 | The package compiles **with** `contract.go` present. This is the mechanical form of contract §7 M1's "Every §4 function exists with the stated signature." |
| S11 | `./scripts/check_i7.sh` exits 0 and reports the count of `ExecuteTx`-wrapped write sites (expected: 5 — Enter, AddEvidence, RetireDebt, Promote, IntentOnPromoted). |
| S12 | `./scripts/m1_accept.sh` exits 0 end to end. |
| **M1 overall** | `go build ./...` = 0; `go vet ./...` = 0; `gofmt -l cmd internal` empty; `./scripts/m1_accept.sh` = 0; `docs/M1_REPORT.md` present with the §8 escalation. |

Explicitly **not** an M1 criterion: that any statement executes correctly, that any invariant
holds, or that any row is written. Those are M2.

---

# 7. Risks introduced by M1

M0's retired risks are not repeated. R2 (embedding) and R3 (temp tables) are carried findings,
not new M1 risks — R3 is folded into §8.

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| **M1-R1** | **No statement is ever executed in M1** (accepted consequence of the strict build+vet decision). A typo'd column name or wrong placeholder arity ships as "green". | Medium | M2 exercises every function; failures there are M1 defects, not M2 discoveries. Mitigated in-step by reading `db/001_schema.sql` alongside each statement. |
| **M1-R2** | `[]string` → `STRING[]` encoding is C9, an Assumption. `EnterBelief` is the *first* call in every future flow, so a failure here blocks all of M2 at once. | Medium | Discharge it as M2's very first test. Fallback pre-decided: `'{…}'::TEXT[]` literal. |
| **M1-R3** | `FullDebt` (Go) and the `debt` `ARRAY[...]` DEFAULT (DDL) encode the same six items in two places and can drift silently. | Medium | S2 requires byte-equality against the DDL at authoring time; M2 should assert a belief inserted *without* an explicit debt list equals `FullDebt`. |
| **M1-R4** | `FullDebt` is an exported mutable slice; a caller can append to or reorder it and corrupt every later `EnterBelief`. | Low | Contract §4 asks for an exported *value*, so the var stays. Pass a defensive copy into the SQL call. |
| **M1-R5** | **`RetractCascade` is stubbed**, so I-8 has no implementation. M2's cascade tests and M6's on-camera cascade both depend on it. This is a schedule dependency on the §8 ruling, not a code defect. | **High** | Escalated in §8 before any code is written. Everything else in M1 proceeds meanwhile. |
| **M1-R6** | Mapping 23514 → `ErrPromotionBlocked` assumes `promoted_is_debt_free` is the only CHECK reachable from `UPDATE belief SET status='promoted'`. True against the frozen schema (`check_status` is satisfied by the literal; `check_claim_type` is untouched). A future CHECK on `belief` would silently widen the mapping. | Low | Recorded here; the schema is frozen. Revisit only if the schema is re-frozen — e.g. if R2 adds a column. |
| **M1-R7** | The multi-`%w` wrap sits *above* the driver error. If a future refactor moves the wrap before the SQLSTATE check, a 40001 would be masked and `crdb` would stop retrying — silently losing I-7's whole purpose. | Low | Wrap strictly after classification; the helper takes the expected code as a parameter so the ordering is structural, not a convention. |

---

# 8. Escalations

## E-1 — `RetractCascade` is blocked. STOP.

**M1 will not implement `RetractCascade`.** It is declared with its §4 signature and returns
`ErrCascadeUnresolved`. Two distinct questions must be ruled on together, because both change
the same statements.

### E-1a — Ambiguity: does `scenarioID` scope the cascade? *(contract §9, "more than one reasonable implementation with different observable behavior")*

Contract §4 gives the signature `RetractCascade(ctx, scenarioID, rootID string)` but describes
the work as *"compute the transitive descendant set of `rootID` over `belief_edge`"* — with no
mention of the scenario. `belief_edge` has **no `scenario_id` column** (confirmed in
`db/001_schema.sql` and in M0's B2 snapshot). The pre-supplied draft accepts the parameter and
never uses it.

Two readings, both defensible:

1. **Scenario-scoped** — add `AND scenario_id = $2` to both UPDATEs. The parameter earns its
   place; a cross-scenario edge cannot un-promote a neighbouring scenario's beliefs.
2. **Global walk** — the parameter is carried for signature symmetry only. Literal reading of
   §4's wording.

Behavior is **identical** while every edge stays within one scenario, and **differs** the
moment one does not. Nothing in the schema prevents a cross-scenario edge. Since
`RetractCascade` is the function that un-promotes decisions, a wrong reading here silently
retracts beliefs outside the caller's scenario — which is precisely the class of error the
contract exists to prevent. Not a decision the implementer should make quietly.

### E-1b — Confirmed defect: the draft's descendant-set mechanism cannot run

The draft builds its descendant set with `CREATE TEMP TABLE _desc ON COMMIT DROP`
(`internal/kernel/kernel.go:115`). M0 verified this is refused on the target cluster:

```
SHOW experimental_enable_temp_tables;   -->  off
CREATE TEMP TABLE _probe_tmp (id INT);
ERROR: temporary tables are only supported experimentally   (SQLSTATE XCEXF)
```

`XCEXF` is neither 23514 nor 23503 and would not be recognized by the kernel's classification.
M0 also demonstrated a working alternative: probe D4 computed the identical transitive
descendant set with an inline `WITH RECURSIVE` CTE attached to each `UPDATE`, in one
transaction, and committed — including the cancel-before-retract ordering I-8 requires. That
is a *feasibility observation from a measured receipt*, not a proposed redesign.

### What is requested

One ruling covering both: (a) scenario-scoped or global; (b) approval to use the
`WITH RECURSIVE` CTE form proven by M0 D4 in place of the temp table. On receipt, implementing
`RetractCascade` is a single small step — the surrounding kernel, sentinels, and I-7 plumbing
are all delivered by M1.

## E-2 — Carried, non-blocking

- **R2 (`belief.embedding`)** — contract §3 and the frozen DDL still disagree. Contract §4
  declares no embedding-bearing API, so **M1 is not blocked**, and per standing instruction it
  stays deferred to M4. Restated only so it is not lost.
- **M0 §3.1 (CHECK messages omit the constraint name)** — still awaiting the M5 ruling on
  whether `ConstraintName` may be displayed beside a raw error. M1's decision to keep the
  kernel driver-agnostic means `ConstraintName` is *not* currently reachable from kernel
  errors; if the M5 ruling requires surfacing it, that is a small, deliberate change at the
  `cmd/` edge, not in the kernel.

No other conflict with `IMPLEMENTATION_CONTRACT.md`, the M0 findings, or observed CockroachDB
behavior was found.

---

# Verification

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
./scripts/m1_accept.sh      # isolation → build ./... → vet ./... → gofmt → I-7 grep
echo "exit=$?"              # 0 == every M1 criterion green
```

No cluster is required — M1 makes no database contact. Then confirm by reading:

- `internal/kernel/contract.go` compiles, which *is* the signature proof.
- `./scripts/check_i7.sh` reports 5 `ExecuteTx`-wrapped write sites and 0 raw writes.
- `RetractCascade` returns `ErrCascadeUnresolved` and contains no SQL.
- `docs/M1_REPORT.md` §8 reproduces escalation E-1 for the architect.

Stop at M1's close. Do not begin M2.
