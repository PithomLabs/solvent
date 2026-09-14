# Plan 11 — Revised: Make `debt` Opaque to Solvent

## 1. Executive Decision

**Kernel scope change:** Remove the hardcoded six-item deployment-review debt
vocabulary from the Solvent kernel. The database default becomes an empty array.
Callers/domain adapters supply their own opaque debt identifiers.

**Architectural invariant:**

    DEBT IS OPAQUE TO SOLVENT.

Solvent stores debt, retires debt, and gates promotion on empty-vs-non-empty.
Solvent never enumerates, validates, normalizes, or interprets debt strings.

**Safety posture change:** Before this change, no belief could reach promotion
without discharging six Solvent-injected obligations. After this change, the
database default is empty and domain adapters must explicitly attach their
starting debt. This is intentional kernel decoupling, compensated by mandatory
adapter-level debt attachment.

**This change restores documented doctrine** (writeup §4.3: "debt opaque
vocabulary... belongs to the application, policy, or deployment") rather than
expanding kernel scope. The implementation had drifted from the design.

---

## 2. Actual Repository Findings

### mapping.go relationship to kernel.FullDebt

**Branch found: B — independent declarations of the same six strings.**

`internal/belief/mapping.go` does NOT import `kernel` or reference
`kernel.FullDebt`. It independently hardcodes the same six debt identifiers as
string literals inside `DebtRule.Items` slices. The two copies are byte-identical
but maintained independently.

`internal/belief/belief.go` (line 57) calls `EnsureBelief` with
`kernel.FullDebt` for starting debt, then (line 79) calls `DebtItemsForEvidence`
from `mapping.go` for retirement. Both consume the same six strings but through
separate code paths.

**Resolution:** Define one domain-owned constant (`wizardDebt`) in a new file
`internal/belief/debt.go`. Both `belief.go` and `mapping.go` reference it.
Eliminates two independent copies and the `kernel.FullDebt` dependency.

### Schema bootstrap path

| Path | 001 | 002 | 003 | 004 | 005 | 006 | 007 | 008 | 009 |
|------|-----|-----|-----|-----|-----|-----|-----|-----|-----|
| Test suites (14 files) | YES | YES | YES | YES | YES | YES | YES | YES | YES |
| Cloud init, cold | YES | YES | YES | YES | YES | YES | YES | YES | NO |
| Cloud init, warm | NO | YES | YES | YES | YES | YES | YES | YES | NO |
| CLI `resolveSchemaPaths` | YES | YES | YES | YES | NO | NO | NO | NO | NO |

**Key finding:** 001's debt default is ALWAYS overwritten by 004 in every
supported path. Editing 001 is cosmetic, not load-bearing. Adding 010 after 004
achieves the empty default for all paths.

`resolveSchemaPaths` in `cmd/solvent/main.go` has a hardcoded list of 3 layers
(002, 003, 004) — it does NOT auto-discover files and must be updated for 010.

### Existing unknown-debt rejection tests

**Go test suite: NONE.** No `go test` tests assert rejection of unknown debt
items. The MCP handler's vocabulary guard (`tools.go:131`) is effectively untested
by Go tests.

**Shell script only:** `scripts/mcp_verify.sh` (lines 120-153) tests:
- `needMap` → REFUSED (retired name)
- `totallyBogus` → REFUSED (unknown item)
- `needBlastRadius` → passes guard, fails on belief lookup
- `debt_item` enum must equal `FULL_DEBT` (lines 121-126)

These tests will need updating when the MCP guard is removed.

### RetireDebt behavior

- **Missing belief:** Returns `kernel.ErrBeliefNotFound` (test: CS-1, line 633)
- **Missing item:** Silent no-op, returns nil (test: B-04, line 195; CS-2, line 664)
- **Already tested with arbitrary vocabulary:** Test DA-02 (line 1583) proves
  `RetireDebt` works with `["proof_check", "counterexample_search", "applicability_review"]`

### Freeze/tag convention

- Previous freeze: `7602699` (commit message: "solvent kernel freeze")
- Existing tags: `backup-before-secret-cleanup`, `v0.0.0-alpha`
- No annotated tag on `7602699`
- No automated freeze mechanism — manual commit + hash recording

### EBP Coordinator

**No coordinator code exists in this repository.** The `ebpInitialDebt`
requirement is a mandatory specification for an external component (the
Coordinator), not a code change in Solvent. The plan records this as a
coordination requirement with a negative regression test specification.

---

## 3. Ownership / Boundary Model

```
    Wizard Domain                         EBP Coordinator
    ─────────────                         ───────────────
    wizardDebt (6 items)                  ebpInitialDebt (6 items)
    DebtMapping (retirement rules)        Packet compilation
    Domain validation                     Mandatory debt attachment
            │                                     │
            │  []string opaque                    │  []string opaque
            └──────────────┬──────────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │   SOLVENT   │
                    │             │
                    │ debt TEXT[]  │
                    │ opaque IDs   │
                    │ empty default│
                    │ promotion    │
                    │   gate       │
                    └─────────────┘
```

- **Solvent kernel:** stores, retires, gates. No vocabulary.
- **API/Application boundary:** structural validation (reject `""`), optional
  debt field maps to empty `[]string`. No vocabulary validation.
- **Wizard domain:** owns `wizardDebt` and `DebtMapping` from one source of
  truth. May validate its own vocabulary.
- **EBP Coordinator (external):** MUST attach `ebpInitialDebt` during packet
  compilation. This is a mandatory specification, not Solvent code.

---

## 4. Schema / Migration Changes

### 4a. `db/001_schema.sql` — NO CHANGE

The debt default in 001 is overwritten by 004 in every supported path. Editing
001 is cosmetic and creates a misleading impression that 001 alone controls the
default. Leave it unchanged.

### 4b. New file: `db/010_debt_opaque.sql`

```sql
-- DEBT IS OPAQUE TO SOLVENT.
--
-- The domain/application layer defines the meaning of debt identifiers.
-- Solvent only enforces empty-vs-non-empty for promotion.
--
-- This migration supersedes 004's SET DEFAULT for new inserts.
-- Existing rows retain their historical debt values.
--
-- Idempotent. Applied on every container start.

ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];
```

### 4c. Legacy / grandfathered beliefs

Existing database rows retain their historical debt arrays. This migration must
NOT rewrite old beliefs, translate old debt names, or auto-retire historical
obligations. Old wizard beliefs remain retireable because `wizardDebt` contains
the same six identifiers.

### 4d. Schema path updates

Add `"../db/010_debt_opaque.sql"` (or `"../../db/..."` depending on relative
path) to all 14 test suite `schemaPaths` definitions.

Add the cloud init constant and wire it into both `ApplySchema` calls in
`demo/cloud/init/main.go`.

Add `"010_debt_opaque.sql"` to the hardcoded layer list in
`cmd/solvent/main.go` `resolveSchemaPaths` (line 277).

Add to `Taskfile.yml` `db:reset` task.

---

## 5. EnterBelief / API Changes

### 5a. Kernel — no signature change

`EnterBelief` already accepts `initialDebt []string`. The nil-to-empty
normalization already exists (kernel.go:58-60). No change to the kernel.

### 5b. API — add optional debt field

`api/types.go` `EnterBeliefRequest`:

```go
type EnterBeliefRequest struct {
    ScenarioID string   `json:"scenario_id"`
    Claim      string   `json:"claim"`
    ClaimType  string   `json:"claim_type"`
    Debt       []string `json:"debt,omitempty"`
}
```

`api/belief.go` `handleEnterBelief` (line 33): Replace `kernel.FullDebt` with:

```go
debt := req.Debt
if debt == nil {
    debt = []string{}
}
id, err := s.ledger.EnterBelief(r.Context(), req.ScenarioID, req.Claim,
    kernel.ClaimType(req.ClaimType), debt)
```

### 5c. Structural input validation (API boundary)

Reject empty-string debt items (`""`) at the API boundary. This is structural
validation, not vocabulary validation:

```go
for _, d := range debt {
    if d == "" {
        writeValidationError(w, "debt", "empty string is not a valid debt identifier", "")
        return
    }
}
```

### 5d. MCP handler — remove vocabulary validation

`cmd/solvent-mcp/tools.go` lines 128-134: Remove the `slices.Contains`
guard entirely. The handler passes the opaque string directly to
`kernel.RetireDebt`.

### 5e. MCP schema — remove enum

`cmd/solvent-mcp/main.go` line 240: Remove `"enum": kernel.FullDebt`. Update
the description (line 241) to explain that callers should inspect the belief's
current debt to discover valid values.

---

## 6. Wizard Vocabulary Migration

### 6a. Create `internal/belief/debt.go` — single source of truth

```go
package belief

// wizardDebt is the wizard/deployment-review workflow's starting debt
// vocabulary. These are domain semantics, not Solvent kernel concepts.
// Both belief creation (starting debt) and evidence mapping (retirement
// rules) reference this single constant.
var wizardDebt = []string{
    "needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
    "needRollbackPlan", "needVersionPin", "needOperatorSignoff",
}
```

### 6b. Update `internal/belief/belief.go` line 57

Replace `kernel.FullDebt` with `wizardDebt`:

```go
beliefID, err := st.EnsureBelief(ctx, scenarioID, b.Claim, ct, wizardDebt)
```

Remove the `kernel` import if no longer needed (it's still needed for
`kernel.New`, `kernel.Derived`, `kernel.Accommodated`, `kernel.ErrPromotionBlocked`).

### 6c. Update `internal/belief/mapping.go`

Replace all six hardcoded string literals with references to `wizardDebt`:

```go
var DebtMapping = map[string][]DebtRule{
    "kev_entry": {
        {Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{wizardDebt[0]}},
    },
    "release": {
        {Match: regexp.MustCompile(`(?i)release`), Items: []string{wizardDebt[0], wizardDebt[1]}},
    },
    // ... etc.
}
```

**Alternative (cleaner):** Keep the string literals in `mapping.go` but add a
comment stating they must match `wizardDebt`. The key architectural point is
that `kernel.FullDebt` is eliminated as the shared source, and both files live
in the same `belief` package so they can reference the same constant.

### 6d. Update `internal/wizard/seed.go`

Remove the `kernel.FullDebt` references (lines 102, 114, 126, 153-154). The
wizard's `Seed` function creates beliefs through the kernel directly — it should
import `belief.wizardDebt` or define its own local copy.

Since `wizardDebt` lives in `internal/belief/` (same package as `mapping.go`),
the wizard package can import it:

```go
import "github.com/PithomLabs/solvent/internal/belief"
```

Or the wizard defines its own local constant. The cleanest option: move
`wizardDebt` to a shared location accessible by both packages, or keep it in
`internal/belief/` and have the wizard import it.

**Decision:** Keep `wizardDebt` in `internal/belief/debt.go`. The wizard
package imports `belief.wizardDebt`. This eliminates all copies except one.

### 6e. Update `internal/wizard/discharge.go` lines 216-221

Replace `FullDebtNames()` with a function that returns a copy of
`belief.wizardDebt`:

```go
func FullDebtNames() []string {
    out := make([]string, len(belief.WizardDebt()))
    copy(out, belief.WizardDebt())
    return out
}
```

(Export the accessor if needed, or make the wizard define its own local.)

### 6f. Update `internal/wizard/state.go`

Replace `kernel.FullDebt` references (lines 83, 131, 137, 347-348) with the
wizard's own debt list.

### 6g. Update `demo/cloud/init/main.go` line 170

Define a local `demoDebt` slice or import from the belief package:

```go
demoDebt := []string{
    "needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
    "needRollbackPlan", "needVersionPin", "needOperatorSignoff",
}
```

### 6h. Update `cmd/operator-review/main.go` line 119

Define a local debt slice for operator review.

### 6i. Update `examples/github/executor/main.go` line 82

Define a local example debt slice.

---

## 7. EBP Coordinator Integration

**The EBP Coordinator does not exist in this repository.** The requirement is
a mandatory specification for the external Coordinator component:

### 7a. Mandatory translation rule

The Coordinator's packet compilation step MUST union `ebpInitialDebt` into
every belief it creates. When a work-agent packet omits debt, the compiled
Solvent belief MUST contain the EBP starting set.

### 7b. Required negative regression test (external)

When the Coordinator is implemented, it must include:

```
input:  valid belief packet, no debts supplied by agent
output: compiled Solvent belief contains ebpInitialDebt
```

This prevents fail-open research ingestion.

### 7c. Additional debt merging

When the agent packet proposes additional debt beyond `ebpInitialDebt`, the
Coordinator MUST preserve/merge both sets — never silently delete required
EBP initial debt.

### 7d. This plan's scope

This plan does not implement Coordinator code. It records the architectural
requirement. The Coordinator spec must reference this plan's §7 as a mandatory
debt-attachment rule.

---

## 8. Test Changes

### 8a. Remove `kernel.FullDebt` constant

Delete from `kernel/kernel.go` lines 11-34. Remove the compile-time assertion
`_ []string = FullDebt` from `kernel/contract.go` line 46.

### 8b. Rewrite tests B-17 and B-23

**New B-17** (`TestW1_B17_DebtEncoding`): Proves arbitrary caller-supplied
debt encodes into `STRING[]` correctly. Create belief with `["foo", "bar", "baz"]`,
read back, verify element-for-element.

**New B-23** (`TestW1_B23_DebtDefaultDrift`): Proves DDL default is empty array.
Raw INSERT without debt column, verify stored debt is `{}`.

### 8c. New regression tests (kernel_test.go)

| Test | Purpose |
|------|---------|
| `TestCS_DEBT01_ArbitraryDebtAccepted` | Create with `["needNullModel", "foo", "bar"]` — must succeed |
| `TestCS_DEBT02_EmptyDebtAccepted` | Create with `[]` — stored debt is empty |
| `TestCS_DEBT03_NilDebtDefaultsEmpty` | Create without debt — stored debt is `{}` |
| `TestCS_DEBT04_UnknownDebtAccepted` | Create with `["completelyUnknownDebt"]` — must succeed |
| `TestCS_DEBT05_ArbitraryDebtRetirement` | Create `["foo", "bar"]`, retire `"foo"`, verify `["bar"]` |
| `TestCS_DEBT06_PromotionBlockedWithArbitraryDebt` | Create `["foo"]`, promote → 23514 |
| `TestCS_DEBT07_PromotionSucceedsEmptyDebt` | Create `[]`, promote → success |
| `TestCS_DEBT08_FinalTruthStillBlocked` | Final-truth claim cannot be promoted |
| `TestCS_DEBT09_DDLDefaultIsEmpty` | Raw INSERT without debt → stored as `{}` |
| `TestCS_DEBT10_EmptyStringRejected` | Empty-string debt item rejected at API boundary |
| `TestCS_DEBT11_DuplicateDebtBehavior` | `["foo", "foo"]` — retire removes both (array_remove semantics) |
| `TestCS_DEBT12_RetireNonexistentBelief` | RetireDebt against nonexistent belief → error |

### 8d. Update test callers of kernel.FullDebt

All ~100 occurrences of `kernel.FullDebt` in test files must be replaced with
explicit inline slices. Most tests use `kernel.FullDebt` as convenient starting
debt — replace with `[]string{"needBlastRadius"}` or similar simple test debt.

### 8e. Update mcp_verify.sh

- Remove enum check (lines 121-126)
- Remove "unknown debt_item" rejection tests (lines 141-148)
- Update test 4c to verify the belief lookup path without the guard
- Update description check (lines 127-130)

### 8f. Existing tests that must pass unchanged

- `TestW1_B03_RetireDebt` — RetireDebt semantics
- `TestW1_B04_RetireDebtIdempotent` — missing item no-op
- `TestW1_B05_Promote` — promotion gate
- `TestW1_B06_IntentOnPromoted` — action_intent gate
- `TestW1_B11_I3_IntentOnUnpromoted` — I-3 invariant
- `TestW2_B07_RetractCascade` — cascade semantics
- `TestCS1_RetireDebt_CrossScenario` — ErrBeliefNotFound
- `TestCS2_RetireDebt_SameScenario_Idempotent` — idempotency
- `TestDA02_RetireArbitraryDebt` — already proves arbitrary debt works
- `TestIntegration_RealFixtureRetiresFullDebt` — full pipeline (needs wizardDebt)

---

## 9. Migration / Bootstrap Verification

### Verification steps

1. Fresh database: Apply 001-010 in order. Verify `belief.debt` default is
   `ARRAY[]::TEXT[]` (not the six-item vocabulary).
2. Existing database with 004 applied: Apply only 010. Verify new inserts get
   empty default. Verify existing rows retain historical debt.
3. Cloud init warm start: Verify 010 is applied on container restart.
4. Test suite reset: Verify all 14 test suites apply 010 and get empty default.
5. CLI `--reset`: Verify `resolveSchemaPaths` includes 010.

### What to NOT do

- Do NOT rewrite existing belief rows
- Do NOT translate old debt names
- Do NOT auto-retire historical obligations
- Do NOT modify 001_schema.sql

---

## 10. Documentation / Taxonomy Guidance

### 10a. Kernel documentation

Update `kernel/doc.go` or the kernel package comment to state:

> Solvent treats debt identifiers as opaque obligation IDs. Debt semantics are
> defined by the caller/domain. Solvent only enforces whether debt is empty or
> non-empty for promotion.

### 10b. Functional taxonomy (external, non-normative)

Create guidance documentation OUTSIDE the kernel (in Conductor docs or
`docs/patterns/`) describing the recurring functional debt roles:

| Role | Question |
|------|----------|
| Provenance | Where did this come from? |
| Consistency | Does it contradict what we hold? |
| Scope | Under what conditions does it hold? |
| Verification | Has anything independent checked it? |
| Impact | What does this affect if wrong? |
| Recovery | If invalidated, what then? |

Frame explicitly as:
- Conceptual guidance for domain vocabulary design
- NOT a universal literal vocabulary
- NOT mandatory Solvent debt values
- NOT kernel policy

### 10c. Do NOT create

- `ebp:need_provenance` or similar namespaced constants
- Universal default debt vocabulary
- Solvent-level debt taxonomy enforcement

---

## 11. Decision Record

Record in `docs/OS/` as a named decision document:

**Lead with:** This change restores documented doctrine (writeup §4.3) rather
than expanding kernel scope — the implementation had drifted from the design.

**Record:**
- Why the kernel was reopened (hardcoded vocabulary violates domain-agnostic boundary)
- Alternatives considered (universal vocabulary, namespace convention, required field)
- Why no universal vocabulary was introduced (would recreate coupling)
- Wizard vocabulary ownership (single source in `internal/belief/debt.go`)
- EBP Coordinator obligation (mandatory `ebpInitialDebt`, external spec)
- Grandfathering decision (existing rows untouched)
- Tests (CS_DEBT01-12, rewritten B-17/B-23)
- Safety posture change (fail-closed → fail-open, compensated by adapter obligation)
- Final frozen commit/hash
- Supersession of prior freeze (`7602699`)

---

## 12. Re-freeze Procedure

### Sequence

1. Implement all changes
2. Run targeted tests (new debt tests)
3. Run full suite (`task test`)
4. Verify migration/bootstrap paths
5. Repository-wide vocabulary sweep (grep for all six strings)
6. Write decision record
7. Commit
8. Tag the commit as the new freeze (annotated tag, e.g., `v0-freeze-debt-opaque`)
9. Update freeze-baseline references in documentation
10. Record supersession of `7602699`

### Freeze tagging convention

Use an annotated git tag on the freeze commit. The tag message should reference:
- The commit hash
- The architectural rationale
- The superseded freeze (`7602699`)

---

## 13. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Domain adapter forgets to attach starting debt | Mandatory adapter-level specification; Coordinator validator enforces for EBP |
| Typo in debt string becomes silent zombie debt | Domain adapter validates its own vocabulary before calling EnterBelief |
| Schema path 010 missing from a test suite | `go test` fails immediately on schema application |
| MCP client caches old enum schema | Tool description updated to explain discovery via belief inspection |
| Old wizard beliefs incompatible with new vocabulary | Same six identifiers preserved in wizard domain layer |

---

## 14. Exact Files Expected to Change

### Schema/migration (2 files)
| File | Change | Why |
|------|--------|-----|
| `db/010_debt_opaque.sql` | NEW | Idempotent migration: SET DEFAULT empty array |
| `Taskfile.yml` | Add 010 to db:reset | Bootstrap completeness |

### Kernel (2 files)
| File | Change | Why |
|------|--------|-----|
| `kernel/kernel.go` | Delete `FullDebt` (lines 11-34) | Remove hardcoded vocabulary |
| `kernel/contract.go` | Remove `_ []string = FullDebt` (line 46) | Compile-time assertion for deleted var |

### Belief wiring layer (3 files)
| File | Change | Why |
|------|--------|-----|
| `internal/belief/debt.go` | NEW | Single source of truth: `wizardDebt` |
| `internal/belief/belief.go` | Replace `kernel.FullDebt` → `wizardDebt` (line 57) | Domain-owned starting debt |
| `internal/belief/mapping.go` | Replace hardcoded strings → `wizardDebt` refs | Eliminate duplicate vocabulary |

### API (2 files)
| File | Change | Why |
|------|--------|-----|
| `api/types.go` | Add `Debt []string` field to `EnterBeliefRequest` | Caller-supplied debt |
| `api/belief.go` | Use `req.Debt` instead of `kernel.FullDebt` (line 33) | Pass caller debt through |

### MCP (2 files)
| File | Change | Why |
|------|--------|-----|
| `cmd/solvent-mcp/tools.go` | Remove `slices.Contains` guard (lines 128-134) | Solvent accepts arbitrary debt |
| `cmd/solvent-mcp/main.go` | Remove `enum` from schema, update description (lines 236-241) | No universal vocabulary |

### Wizard (3 files)
| File | Change | Why |
|------|--------|-----|
| `internal/wizard/seed.go` | Import `belief.wizardDebt`, replace `kernel.FullDebt` | Domain-owned vocabulary |
| `internal/wizard/discharge.go` | Replace `FullDebtNames()` to use `belief.WizardDebt()` | Single source |
| `internal/wizard/state.go` | Replace `kernel.FullDebt` references | Domain-owned vocabulary |

### Other callers (3 files)
| File | Change | Why |
|------|--------|-----|
| `demo/cloud/init/main.go` | Define local `demoDebt`, add 010 to schema paths | Domain-specific debt |
| `cmd/operator-review/main.go` | Define local debt slice | Domain-specific debt |
| `examples/github/executor/main.go` | Define local example debt | Domain-specific debt |

### Schema path lists (14 test files + CLI)
All 14 `schemaPaths` definitions + `cmd/solvent/main.go` `resolveSchemaPaths`
+ `demo/cloud/init/main.go` constants.

### Test files (~15+ files)
Replace `kernel.FullDebt` with explicit debt slices. Rewrite B-17/B-23. Add
CS_DEBT01-12.

### Verification script (1 file)
| File | Change | Why |
|------|--------|-----|
| `scripts/mcp_verify.sh` | Remove enum check, remove rejection tests, update discovery | Matches new behavior |

---

## 15. Acceptance Criteria

1. **`task test` passes** — full suite including new regression tests
2. **No kernel-level debt vocabulary** — grep for all six strings shows only
   wizard/domain/example/test usage, never kernel validation or defaulting
3. **Empty default verified** — fresh database gives new beliefs empty debt
4. **Arbitrary debt accepted** — CS_DEBT01/04 pass
5. **Promotion gate unchanged** — CS_DEBT06/07/08 pass
6. **Action intent gate unchanged** — existing tests pass
7. **Retraction cascade unchanged** — existing tests pass
8. **Refusal logging unchanged** — existing tests pass
9. **Migration verified** — 010 applied in all bootstrap paths
10. **Decision record written** — doctrine cited, alternatives recorded
11. **New freeze tagged** — annotated tag, supersession of `7602699` recorded
12. **mcp_verify.sh updated** — matches new behavior
13. **No new universal vocabulary introduced** — no `ebp:*` constants, no
    "sane defaults", no shared taxonomy enforcement in Solvent
