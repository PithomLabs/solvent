# Plan 11: Make `debt` Opaque to Solvent

## Goal

Remove the hardcoded six-item debt vocabulary from the Solvent kernel. After this
change, Solvent treats `belief.debt` as a `TEXT[]` of opaque obligation identifiers.
The domain/application layer (wizard, EBP, coordinator) owns the meaning of debt
strings.

## Architectural Invariant After Change

```
Domain / Wizard / Coordinator
        |
        | supplies opaque []string debt IDs
        v
    Solvent kernel
        |
        | stores, retires, gates on empty vs non-empty
        v
   Promotion Gate (database-enforced)
```

---

## Step 1: Database Schema Changes

### 1a. `db/001_schema.sql` (line 25-27) — Change DEFAULT

**Before:**

```sql
debt TEXT[] NOT NULL DEFAULT ARRAY[
  'needProvenanceCheck','needContradictionSweep','needBlastRadius',
  'needRollbackPlan','needVersionPin','needOperatorSignoff'],
```

**After:**

```sql
debt TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
```

Only affects fresh databases. The `promoted_is_debt_free` CHECK constraint is
unchanged.

### 1b. New file: `db/010_debt_opaque.sql`

Idempotent migration for existing databases. Overrides the default set by
`004_debt_vocabulary.sql` for future inserts. Existing rows are not touched
(historical debt preserved).

```sql
-- Debt is opaque to Solvent. The domain/application layer defines the meaning
-- of debt identifiers. Solvent only enforces empty-vs-non-empty for promotion.
--
-- This migration supersedes 004's SET DEFAULT for new inserts. Existing rows
-- retain their historical debt values.
--
-- Idempotent. Applied on every container start.

ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];
```

**Do not modify `004_debt_vocabulary.sql`.** Migration `010` runs after `004`
and overrides the default. This is safe because `SET DEFAULT` only affects
future inserts.

---

## Step 2: Remove `kernel.FullDebt` from `kernel/kernel.go`

**Delete lines 11-34** (the `FullDebt` variable and its comment block).

The `EnterBelief` signature already accepts `initialDebt []string` — no
signature change needed. The nil-to-empty normalization (lines 58-60) already
exists:

```go
debt := initialDebt
if debt == nil {
    debt = []string{}
}
```

**Also update `kernel/contract.go` line 46:** Remove the compile-time assertion
`_ []string = FullDebt`.

---

## Step 3: Update All Callers of `kernel.FullDebt`

### 3a. API layer: `api/belief.go`

- Add `Debt []string` field to `EnterBeliefRequest` with
  `json:"debt,omitempty"`.
- In `handleEnterBelief`: normalize nil/omitted debt to empty slice, pass to
  `EnterBelief`.
- Do not validate debt vocabulary — Solvent accepts arbitrary strings.

### 3b. MCP layer: `cmd/solvent-mcp/tools.go` (lines 128-134)

Remove the vocabulary validation block:

```go
// DELETE:
if !slices.Contains(kernel.FullDebt, item) {
    return errorResult(fmt.Errorf("unknown debt_item: %q (valid: %s)", ...))
}
```

Solvent accepts any string. The MCP layer must not reject items.

### 3c. MCP schema: `cmd/solvent-mcp/main.go` (line 240)

Remove `"enum": kernel.FullDebt` from the `debt_item` input schema. Change
description to state debt_item is an opaque string.

Also update the `solvent_retire_debt` description (line 221) to remove
"must be one of the six items the database issued".

### 3d. Wizard: `internal/wizard/seed.go` and `internal/wizard/discharge.go`

Create a local `wizardDebt` slice in `internal/wizard/seed.go`:

```go
// wizardDebt is the wizard workflow's own starting debt vocabulary.
// These are wizard semantics, not Solvent kernel concepts.
var wizardDebt = []string{
    "needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
    "needRollbackPlan", "needVersionPin", "needOperatorSignoff",
}
```

Replace all `kernel.FullDebt` references in wizard code with `wizardDebt`.

In `internal/wizard/discharge.go` (line 216-221): Replace `FullDebtNames()`
with a function that returns `wizardDebt`.

In `internal/wizard/state.go`: Replace `kernel.FullDebt` references with the
local wizard debt list.

### 3e. Cloud init: `demo/cloud/init/main.go` (line 170)

Replace `kernel.FullDebt` with a local demo debt slice or pass empty debt.
The cloud init demo uses the deployment-review vocabulary — define a local
`demoDebt` slice.

### 3f. Operator review CLI: `cmd/operator-review/main.go` (line 119)

Replace `kernel.FullDebt` with a local slice appropriate for operator review.

### 3g. All test files (~100 occurrences)

Replace `kernel.FullDebt` with explicit inline slices. Most tests just need
*some* debt to exercise the promotion gate. Use simple test debt like
`[]string{"testDebt"}` or `[]string{"foo", "bar"}`. For tests that specifically
need multiple debt items (e.g., testing retirement), use explicit slices.

### 3h. `examples/github/executor/main.go` (line 82)

Replace `kernel.FullDebt` with a local example debt slice.

---

## Step 4: Rewrite Tests B-17 and B-23

Replace `TestW1_B17_DebtEncoding` and `TestW1_B23_DebtDefaultDrift` with new
tests:

- **New B-17**: Proves arbitrary caller-supplied debt encodes into `STRING[]`
  correctly. Create a belief with `["foo", "bar", "baz"]`, read back from DB,
  verify element-for-element.

- **New B-23**: Proves the DDL default is empty array. Raw INSERT without debt
  column, verify stored debt is `{}`.

---

## Step 5: Add New Regression Tests

Add to `kernel/kernel_test.go` using the existing naming conventions:

| Test Name | Wave | Purpose |
|---|---|---|
| `TestCS_DEBT01_ArbitraryDebtAccepted` | cs | Create with `["needNullModel", "foo", "bar"]` — must succeed |
| `TestCS_DEBT02_EmptyDebtAccepted` | cs | Create with `[]` — stored debt is empty |
| `TestCS_DEBT03_NilDebtDefaultsEmpty` | cs | Create without debt — stored debt is `{}` |
| `TestCS_DEBT04_UnknownDebtAccepted` | cs | Create with `["completelyUnknownDebt"]` — must succeed |
| `TestCS_DEBT05_ArbitraryDebtRetirement` | cs | Create `["foo", "bar"]`, retire `"foo"`, verify `["bar"]` |
| `TestCS_DEBT06_PromotionBlockedWithArbitraryDebt` | cs | Create `["foo"]`, promote → 23514/promoted_is_debt_free |
| `TestCS_DEBT07_PromotionSucceedsEmptyDebt` | cs | Create `[]`, promote → success |
| `TestCS_DEBT08_FinalTruthStillBlocked` | cs | Final-truth claim cannot be promoted (unchanged) |
| `TestCS_DEBT09_DDLDefaultIsEmpty` | cs | Raw INSERT without debt → stored as `{}` |

---

## Step 6: Add Migration to Schema Path Lists

Append `"../db/010_debt_opaque.sql"` to `schemaPaths` in all test suite files:

- `kernel/suite_test.go`
- `adapter/github/executor_test.go`
- `api/suite_test.go`
- `internal/corpus/corpus_test.go`
- `internal/wizard/seed_test.go`
- `internal/pipeline/pipeline_test.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `internal/demoseed/demoseed_test.go`
- `internal/agentjacking/ingest_test.go`
- `service/ledger/ledger_test.go`
- `internal/belief/belief_test.go`
- `service/authority/authority_integration_test.go`
- `internal/view/explain_test.go`
- `internal/intent/intent_test.go`
- `demo/cloud/init/main.go` (both `ApplySchema` calls)
- `cmd/solvent/main.go` (the `resolveSchemaPaths` function — it auto-discovers
  layer files, so adding the file may be sufficient, but verify)

Also add to `Taskfile.yml` `db:reset` task (line 35).

---

## Step 7: Documentation Update

Update only directly affected docs. Add a note to `kernel/doc.go` or the kernel
package comment stating:

> Solvent treats debt identifiers as opaque obligation IDs. Debt semantics are
> defined by the caller/domain. Solvent only enforces whether debt is empty or
> non-empty for promotion.

Do not add EBP documentation to the Solvent repository.

---

## Step 8: Full Test Run

```bash
task test
```

This runs `go test -count=1 -p 1 ./...`, `go build ./...`, `go vet ./...`,
`gofmt` check, and the I-7 static gate check.

---

## What Does NOT Change

| Component | Status |
|---|---|
| `belief` table columns (other than DEFAULT) | Frozen |
| `belief_edge`, `evidence`, `action_intent` | Frozen |
| `promoted_is_debt_free` CHECK | Frozen |
| `live_requires_promoted` CHECK | Frozen |
| `gate` FK constraint | Frozen |
| `EnterBelief` function signature | Already accepts `[]string` |
| `RetireDebt` implementation | Already generic |
| `Promote` implementation | Already schema-gated |
| `RetractCascade` implementation | Unchanged |
| `refusal_log` schema/behavior | Unchanged |
| `mapping.go` (domain layer) | Keeps its own vocabulary |
| Conductor | Unchanged |
| Authority model | Unchanged |

---

## Execution Order Summary

1. Create `db/010_debt_opaque.sql`
2. Edit `db/001_schema.sql` — change DEFAULT
3. Delete `kernel.FullDebt` from `kernel/kernel.go`, update `contract.go`
4. Update `api/belief.go` — add optional debt field to request
5. Update `cmd/solvent-mcp/tools.go` — remove vocabulary validation
6. Update `cmd/solvent-mcp/main.go` — remove enum from schema
7. Create `wizardDebt` in `internal/wizard/seed.go`, update `discharge.go`
   and `state.go`
8. Update `demo/cloud/init/main.go` — local demo debt
9. Update `cmd/operator-review/main.go` — local debt
10. Update `examples/github/executor/main.go`
11. Update all test callers of `kernel.FullDebt` (~100 occurrences)
12. Rewrite tests B-17 and B-23
13. Add new regression tests (CS_DEBT01–09)
14. Add `010` to all `schemaPaths` slices (~15 files) and Taskfile
15. Run `task test`

---

## Deliverables

1. **Files changed** — listed above in execution order
2. **Migration added** — `db/010_debt_opaque.sql`
3. **`EnterBelief` API change** — no signature change (already accepts
   `[]string`); `FullDebt` constant removed; callers pass their own debt
4. **Tests added** — 9 new regression tests + 2 rewritten drift tests
5. **Full test result** — `task test` must pass
6. **Old vocabulary removed** — no Solvent-level code inserts or requires
   `needProvenanceCheck` etc.
7. **Authority semantics preserved** — promotion, action_intent, retraction
   cascade, refusal logging all unchanged
