# Plan: Parameterize Initial Debt in Kernel Belief Creation

**Scope:** Make `initialDebt` an explicit parameter to `EnterBelief` and `EnsureBelief`. The kernel no longer implicitly stamps `FullDebt`. Existing callers pass `kernel.FullDebt` explicitly.

**Principle:** Domain-agnostic kernel. Caller/policy owns vocabulary. Kernel stores opaque obligations.

---

## Pre-inspection summary

| Question | Answer |
|----------|--------|
| `EnterBelief` call sites | 39 (1 kernel impl, 1 service passthrough, 1 API handler, 1 CLI, 2 wizard, 1 demo, 1 example, 32 test) |
| `EnsureBelief` call sites | 18 (1 kernel impl, 2 app code, 15 test) |
| Options pattern in kernel? | None. All methods are plain positional args. |
| REST API exposes initial debt? | No. `EnterBeliefRequest` has `scenario_id`, `claim`, `claim_type` only. |
| MCP belief creation tool? | No. Beliefs created via pipeline or REST only. |
| DDL DEFAULT used by? | Only `EnsureBelief` (INSERT omits debt column). `EnterBelief` always passes debt explicitly. |
| Tests depending on 6-item init? | B-01 (`debtLen==6`), B-17 (encoding), B-23 (DDL drift), plus many retire-all-6 setup helpers. |

---

## Design

### Kernel API change

Both methods gain `initialDebt []string` as a **required** parameter:

```go
// Before:
EnterBelief(ctx, scenarioID, claim, ct) (string, error)
EnsureBelief(ctx, scenarioID, claim, ct) (string, error)

// After:
EnterBelief(ctx, scenarioID, claim, ct, initialDebt []string) (string, error)
EnsureBelief(ctx, scenarioID, claim, ct, initialDebt []string) (string, error)
```

- `EnterBelief`: passes `initialDebt` to SQL as `$4::STRING[]`
- `EnsureBelief`: passes `initialDebt` to SQL; INSERT includes debt column when provided

### SQL changes

**`sqlEnterBelief`** — no change needed (already accepts `$4::STRING[]`).

**`sqlEnsureBelief`** — modify INSERT to include debt column:

```sql
-- Before:
INSERT INTO belief (scenario_id, claim, claim_type)
SELECT $1::UUID, $2::STRING, $3::STRING

-- After:
INSERT INTO belief (scenario_id, claim, claim_type, debt)
SELECT $1::UUID, $2::STRING, $3::STRING, $4::STRING[]
```

The CTE structure is preserved. Existing beliefs are returned unchanged. New beliefs get the caller-supplied debt.

### DDL DEFAULT

**Keep the DDL DEFAULT.** It serves as defense-in-depth for raw SQL inserts (tests, proof harnesses). The kernel path no longer relies on it, but removing it would require a migration and break existing proof scripts. Not worth the churn.

### `FullDebt` location

**Keep `FullDebt` in `kernel/kernel.go`.** Moving it would create unnecessary churn across 50+ references. It is documented as a deployment/application convenience value, not a kernel semantic. The kernel creation functions no longer depend on it.

---

## Implementation steps

### Step 1: Kernel contract and implementation

**Files:** `kernel/contract.go`, `kernel/kernel.go`

1. Update `Contract` interface: add `initialDebt []string` to both methods
2. Update `Store.EnterBelief`: accept `initialDebt`, pass to SQL
3. Update `Store.EnsureBelief`: accept `initialDebt`, pass to SQL
4. Update `sqlEnsureBelief`: add `$4::STRING[]` to INSERT column list

### Step 2: Service layer

**File:** `service/ledger/ledger.go`

Update `EnterBelief` passthrough to accept and forward `initialDebt`.

### Step 3: API handler

**File:** `api/belief.go`

Update `handleEnterBelief` to pass `kernel.FullDebt` to `ledger.EnterBelief`. The REST API continues to create beliefs with the standard deployment vocabulary. No `debt` field added to `EnterBeliefRequest`.

### Step 4: Wizard, demo, CLI, example callers

**Files:**
- `internal/wizard/seed.go` (2 calls) — pass `kernel.FullDebt`
- `demo/cloud/init/main.go` (1 call) — pass `kernel.FullDebt`
- `cmd/operator-review/main.go` (1 call) — pass `kernel.FullDebt`
- `examples/github/executor/main.go` (1 call) — pass `kernel.FullDebt`

### Step 5: Application code (EnsureBelief callers)

**Files:**
- `internal/belief/belief.go` (1 call) — pass `kernel.FullDebt`
- `internal/agentjacking/ingest.go` (1 call) — pass `kernel.FullDebt`

### Step 6: Test code — all EnterBelief and EnsureBelief call sites

Update all 32 `EnterBelief` test calls and 15 `EnsureBelief` test calls to pass `kernel.FullDebt`.

**Key test files:**
- `kernel/kernel_test.go` (15 EnterBelief, 5 EnsureBelief)
- `kernel/operator_review_test.go` (5 EnterBelief)
- `kernel/example_test.go` (1 EnterBelief)
- `kernel/authority_test.go` (1 EnterBelief)
- `service/ledger/ledger_test.go` (1 EnterBelief)
- `service/authority/authority_integration_test.go` (1 EnterBelief)
- `api/helpers_test.go` (1 EnterBelief)
- `api/authorization_test.go` (5 EnsureBelief)
- `cmd/solvent-mcp/tools_cs_test.go` (1 EnsureBelief)
- `cmd/solvent-mcp/tools_agentjacking_test.go` (1 EnsureBelief)
- `cmd/solvent-mcp/tools_authority_test.go` (1 EnsureBelief)
- `adapter/github/executor_test.go` (1 EnterBelief)
- `internal/demoseed/demoseed_test.go` (1 EnterBelief)
- `internal/corpus/corpus_test.go` (1 EnterBelief)
- `internal/view/explain_test.go` (4 EnterBelief)
- `internal/pipeline/pipeline_test.go` (1 EnsureBelief)
- `internal/intent/intent_test.go` (2 EnsureBelief)

### Step 7: New tests

Add to `kernel/kernel_test.go`:

**A. Arbitrary vocabulary accepted:**
Create a belief with `["proof_check", "counterexample_search", "applicability_review"]`. Verify exact values stored. No kernel validation rejects them.

**B. Empty initial debt:**
Create a belief with `[]`. Verify creation succeeds, debt is empty. Verify promotion invariant: if debt is empty AND final_truth is false, promotion should succeed (no debt block).

**C. EnsureBelief preserves existing debt:**
Call `EnsureBelief` twice with different `initialDebt` values on the same claim. Verify the second call does NOT overwrite the first call's debt.

**D. Kernel does not validate debt names:**
Create a belief with arbitrary strings, retire one, verify it was removed. Prove the kernel treats debt as opaque identifiers.

### Step 8: Documentation update

Update the relevant architecture documentation to state:
- Debt vocabulary is policy/domain-owned
- `FullDebt` is an existing deployment vocabulary, not a universal kernel vocabulary
- The kernel stores generic obligation identifiers and enforces lifecycle

### Step 9: Full verification

Run all verification commands.

---

## Files changed (complete list)

| File | Reason |
|------|--------|
| `kernel/contract.go` | Add `initialDebt []string` to interface |
| `kernel/kernel.go` | Update `EnterBelief` and `EnsureBelief` signatures and implementation |
| `kernel/sql.go` | Update `sqlEnsureBelief` to include debt in INSERT |
| `service/ledger/ledger.go` | Update `EnterBelief` passthrough |
| `api/belief.go` | Pass `kernel.FullDebt` to `ledger.EnterBelief` |
| `internal/wizard/seed.go` | Pass `kernel.FullDebt` to `EnterBelief` (2 calls) |
| `demo/cloud/init/main.go` | Pass `kernel.FullDebt` to `EnterBelief` |
| `cmd/operator-review/main.go` | Pass `kernel.FullDebt` to `EnterBelief` |
| `examples/github/executor/main.go` | Pass `kernel.FullDebt` to `EnterBelief` |
| `internal/belief/belief.go` | Pass `kernel.FullDebt` to `EnsureBelief` |
| `internal/agentjacking/ingest.go` | Pass `kernel.FullDebt` to `EnsureBelief` |
| `kernel/kernel_test.go` | Update 20 call sites + add 4 new tests |
| `kernel/operator_review_test.go` | Update 5 call sites |
| `kernel/example_test.go` | Update 1 call site |
| `kernel/authority_test.go` | Update 1 call site |
| `service/ledger/ledger_test.go` | Update 1 call site |
| `service/authority/authority_integration_test.go` | Update 1 call site |
| `api/helpers_test.go` | Update 1 call site |
| `api/authorization_test.go` | Update 5 call sites |
| `cmd/solvent-mcp/tools_cs_test.go` | Update 1 call site |
| `cmd/solvent-mcp/tools_agentjacking_test.go` | Update 1 call site |
| `cmd/solvent-mcp/tools_authority_test.go` | Update 1 call site |
| `adapter/github/executor_test.go` | Update 1 call site |
| `internal/demoseed/demoseed_test.go` | Update 1 call site |
| `internal/corpus/corpus_test.go` | Update 1 call site |
| `internal/view/explain_test.go` | Update 4 call sites |
| `internal/pipeline/pipeline_test.go` | Update 1 call site |
| `internal/intent/intent_test.go` | Update 2 call sites |
| `docs/OS/adv_review10_debt_vocabulary.md` | Update verdict to GREEN |

**No schema changes. No migration. No new kernel primitives.**

---

## Acceptance criteria checklist

- [ ] `EnterBelief` no longer intrinsically stamps `FullDebt`
- [ ] Existing callers explicitly provide `kernel.FullDebt`
- [ ] A different domain can create beliefs with arbitrary debt vocabulary
- [ ] Kernel code does not validate or interpret individual debt names
- [ ] `EnsureBelief` does not overwrite existing debt
- [ ] Promotion/debt invariants unchanged
- [ ] No debt-count limit introduced
- [ ] No workflow/policy-engine/idempotency machinery added
- [ ] Full test suite green
- [ ] Race tests green
- [ ] I-7 and MCP verification green
