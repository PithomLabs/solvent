# Plan 11.2 — Final: Make `debt` Opaque to Solvent

## 1. Executive Decision

Remove the hardcoded six-item deployment-review debt vocabulary from the Solvent
kernel. The database default becomes an empty array. Callers/domain adapters
supply their own opaque debt identifiers.

**This change restores documented doctrine** (writeup §4.3: "debt opaque
vocabulary... belongs to the application, policy, or deployment") rather than
expanding kernel scope. The implementation had drifted from the design.

**Architectural invariant:**

    DEBT IS OPAQUE TO SOLVENT.

Solvent stores debt, retires debt, and gates promotion on empty-vs-non-empty.
Solvent never enumerates, validates, normalizes, interprets, or rejects debt
strings based on content.

**Safety posture change:** Before this change, no belief could reach promotion
without discharging six Solvent-injected obligations. After this change, the
database default is empty and domain adapters must explicitly attach their
starting debt. This is intentional kernel decoupling, compensated by mandatory
adapter-level debt attachment.

---

## 2. Actual Repository Findings

### Package dependency graph (BLOCKING CHECK — resolved)

| Edge | Exists? |
|------|---------|
| `internal/belief` → `internal/wizard` | NO |
| `internal/wizard` → `internal/belief` | NO |
| Any package imports both? | NO (`internal/pipeline` imports `belief` only; `demo/cloud/web` imports `wizard` only) |

**No cycle exists.** The vocabulary can live in `internal/belief/` (where
`mapping.go` and `belief.go` already consume it) and `internal/wizard` can
import it without creating a cycle.

### mapping.go relationship to kernel.FullDebt (resolved)

Branch B: `mapping.go` independently hardcodes the same six strings. It does NOT
import `kernel`. Both `belief.go` (line 57, starting debt) and `mapping.go`
(lines 15-34, retirement rules) use the same six strings through separate code
paths with no shared source of truth.

**Resolution:** Single `wizardDebt` constant with named constants in
`internal/belief/debt.go`. Both `belief.go` and `mapping.go` reference it.

### Schema bootstrap path

| Path | 001 | 002-003 | 004 | 005-009 | 010 (new) |
|------|-----|---------|-----|---------|-----------|
| Test suites (14 files) | YES | YES | YES | YES | ADD |
| Cloud init, cold | YES | YES | YES | YES | ADD |
| Cloud init, warm | NO | YES | YES | YES | ADD |
| CLI `resolveSchemaPaths` | YES | YES | YES | NO | ADD |

**001's debt default is always overwritten by 004.** 010 after 004 achieves
empty default for all paths. 001 is NOT modified.

### Existing unknown-debt rejection tests

- **Go test suite:** NONE. The MCP handler's vocabulary guard (`tools.go:131`)
  is untested by `go test`.
- **Shell script only:** `scripts/mcp_verify.sh` (lines 120-153) tests enum
  assertion, retired-name rejection, and bogus-item rejection.

### RetireDebt behavior (confirmed)

- Missing belief → `ErrBeliefNotFound` (test: CS-1)
- Missing item → silent no-op, returns nil (tests: B-04, CS-2)
- `array_remove` on `["foo","foo"]` removes ALL matching occurrences

### Freeze convention

Previous freeze: `7602699` (no annotated tag). Two tags exist:
`backup-before-secret-cleanup`, `v0.0.0-alpha`.

### EBP Coordinator

No coordinator code exists in this repository. The requirement is a mandatory
specification for the external Coordinator component.

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

**Layer responsibilities:**

| Layer | Responsible for | Prohibited from |
|-------|----------------|-----------------|
| Solvent kernel | Store debt, retire opaque IDs, gate promotion on empty | Enumerate, validate, normalize, interpret debt strings |
| API/MCP adapter | Structural validation (reject `""`), optional field mapping | Vocabulary validation, length/regex rules |
| Wizard domain | Own `wizardDebt`, own `DebtMapping`, validate own vocabulary | Rely on kernel for vocabulary |
| EBP Coordinator (external) | Attach `ebpInitialDebt` during packet compilation | Rely on Solvent defaults |

---

## 4. Schema / Migration Changes

### 4a. `db/001_schema.sql` — NO CHANGE

001's debt default is overwritten by 004 in every supported path. Editing 001 is
cosmetic. Leave unchanged.

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

Existing rows retain their historical debt arrays. Migration must NOT rewrite
rows, translate debt names, or auto-retire items. Old wizard beliefs remain
operable because the wizard domain still recognizes the same identifiers.

### 4d. Schema path updates

Add `"../db/010_debt_opaque.sql"` (with correct relative prefix) to all 14
test suite `schemaPaths` definitions. Add cloud init constant. Add to
`resolveSchemaPaths` hardcoded layer list. Add to `Taskfile.yml` `db:reset`.

---

## 5. API / MCP Changes

### 5a. API — add optional debt field

`api/types.go` `EnterBeliefRequest`:

```go
type EnterBeliefRequest struct {
    ScenarioID string   `json:"scenario_id"`
    Claim      string   `json:"claim"`
    ClaimType  string   `json:"claim_type"`
    Debt       []string `json:"debt,omitempty"`
}
```

`api/belief.go` `handleEnterBelief` (line 33):

```go
debt := req.Debt
if debt == nil {
    debt = []string{}
}
id, err := s.ledger.EnterBelief(r.Context(), req.ScenarioID, req.Claim,
    kernel.ClaimType(req.ClaimType), debt)
```

### 5b. Structural validation (API boundary)

Reject empty-string debt items (`""`) at the API boundary:

```go
for _, d := range debt {
    if d == "" {
        writeValidationError(w, "debt", "empty string is not a valid debt identifier", "")
        return
    }
}
```

### 5c. MCP handler — remove vocabulary validation, add empty-string guard

`cmd/solvent-mcp/tools.go`: Remove the `slices.Contains` guard (lines 128-134).
Add empty-string rejection:

```go
if item == "" {
    return errorResult(fmt.Errorf("empty debt_item is not a valid identifier")), nil
}
```

### 5d. MCP schema — remove enum

`cmd/solvent-mcp/main.go` line 240: Remove `"enum": kernel.FullDebt`. Update
description to explain that callers discover valid values by inspecting the
belief's current debt.

---

## 6. Wizard Vocabulary Consolidation

### 6a. New file: `internal/belief/debt.go`

Single source of truth for the deployment-review vocabulary:

```go
package belief

// Named constants for the deployment-review debt vocabulary.
// These are domain-specific obligation identifiers, not Solvent kernel concepts.
// Both belief creation (starting debt) and evidence mapping (retirement rules)
// reference this single source of truth.
const (
    NeedProvenanceCheck    = "needProvenanceCheck"
    NeedContradictionSweep = "needContradictionSweep"
    NeedBlastRadius        = "needBlastRadius"
    NeedRollbackPlan       = "needRollbackPlan"
    NeedVersionPin         = "needVersionPin"
    NeedOperatorSignoff    = "needOperatorSignoff"
)

// WizardDebt returns the deployment-review starting debt vocabulary.
// Returns a defensive copy to prevent mutation of the backing slice.
func WizardDebt() []string {
    return []string{
        NeedProvenanceCheck,
        NeedContradictionSweep,
        NeedBlastRadius,
        NeedRollbackPlan,
        NeedVersionPin,
        NeedOperatorSignoff,
    }
}

// wizardDebt is the internal copy used by belief.go and mapping.go.
var wizardDebt = WizardDebt()
```

### 6b. Update `internal/belief/belief.go` line 57

Replace `kernel.FullDebt` with `wizardDebt`:

```go
beliefID, err := st.EnsureBelief(ctx, scenarioID, b.Claim, ct, wizardDebt)
```

### 6c. Update `internal/belief/mapping.go`

Replace all six hardcoded string literals with named constants:

```go
var DebtMapping = map[string][]DebtRule{
    "kev_entry": {
        {Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{NeedProvenanceCheck}},
    },
    "release": {
        {Match: regexp.MustCompile(`(?i)release`), Items: []string{NeedProvenanceCheck, NeedContradictionSweep}},
    },
    "maintainer_comment": {
        {Match: regexp.MustCompile(`(?i)\b(fixed|fix released|patch available)\b`), Items: []string{NeedProvenanceCheck, NeedContradictionSweep}},
        {Match: regexp.MustCompile(`(?i)\b(tested|confirmed)\b`), Items: []string{NeedBlastRadius, NeedRollbackPlan}},
        {Match: regexp.MustCompile(`(?i)\bno regression\b`), Items: []string{NeedRollbackPlan, NeedVersionPin}},
        {Match: regexp.MustCompile(`(?i)\b(security review|reviewed by)\b`), Items: []string{NeedOperatorSignoff}},
    },
    "github_pr": {
        {Match: regexp.MustCompile(`(?i)\bfix\b`), Items: []string{NeedProvenanceCheck, NeedContradictionSweep}},
    },
    "github_advisory": {
        {Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{NeedProvenanceCheck}},
    },
}
```

### 6d. Update wizard package

`internal/wizard/seed.go`: Import `belief` package, replace `kernel.FullDebt`
with `belief.WizardDebt()`.

`internal/wizard/discharge.go` lines 216-221: Replace `FullDebtNames()`:

```go
func FullDebtNames() []string {
    return belief.WizardDebt()
}
```

`internal/wizard/state.go`: Replace `kernel.FullDebt` references with
`belief.WizardDebt()`.

### 6e. Other callers

`demo/cloud/init/main.go`: Import `belief.WizardDebt()` (same domain).

`cmd/operator-review/main.go`: Import `belief.WizardDebt()` (same domain).

`examples/github/executor/main.go`: Define a local example debt slice (isolated
example, should not import internal packages).

`internal/agentjacking/ingest.go`: Import `belief.WizardDebt()` (same demo
domain).

### 6f. Remove kernel.FullDebt

Delete from `kernel/kernel.go` lines 11-34. Remove compile-time assertion
`_ []string = FullDebt` from `kernel/contract.go` line 46.

---

## 7. EBP Coordinator Integration Contract

**No Coordinator code exists in this repository.** This is a mandatory external
specification.

### 7a. Mandatory translation rule

The Coordinator's packet compilation step MUST produce:

```
compiledDebt = ebpInitialDebt ∪ acceptedPacketDebt
```

For every EBP belief compiled from a research packet, the compiled canonical
belief MUST contain `ebpInitialDebt` even when the agent packet omits debt.

Agent-supplied debt may add obligations but may NOT remove required EBP debt.

### 7b. Required negative regression test (external)

```
input:   valid belief packet, agent supplies no debt
output:  compiled canonical belief contains ebpInitialDebt
```

This test belongs to the Coordinator, not Solvent. It prevents fail-open
research ingestion.

### 7c. Safety responsibility map

| Domain | Owner | Enforcement point |
|--------|-------|-------------------|
| Wizard | Wizard domain code | `belief.WizardDebt()` at creation |
| EBP | Coordinator | `ebpInitialDebt` at packet compilation |
| Operator review | CLI tool | Local debt slice at belief creation |
| Demo | Cloud init | Local debt slice at belief creation |
| Other domains | Their own adapter | Must explicitly attach starting debt |

Solvent provides no policy default. The database empty default is intentional.

---

## 8. Test Classification and Changes

### 8a. Classification of kernel.FullDebt test occurrences

**Category A (53 occurrences):** Test only needs any non-empty debt. Replace
with `[]string{"testDebt"}` or similar simple value.

**Category B (34 occurrences):** Test depends on the six-item wizard lifecycle.
Use `belief.WizardDebt()`.

### 8b. Category B test files (require wizard vocabulary)

| File | Tests | Reason |
|------|-------|--------|
| `kernel/kernel_test.go` | B01, B17, B23 | Debt encoding, DDL drift, exact length |
| `kernel/example_test.go` | Example_lifecycle | Full lifecycle with specific items |
| `internal/view/explain_test.go` | AskBeforePromotion, PromotionAfterDebtDischarge, ReassessmentFalsification | Full lifecycle, debt count checks |
| `internal/wizard/flow_test.go` | W07, W12, W15 | Full wizard lifecycle, operatorArtifacts mapping |
| `internal/wizard/http_test.go` | HTTP flow | Discharges all six via API |
| `internal/wizard/seed_test.go` | SeedVerify | Checks debt length |
| `internal/wizard/ledger_isolation_test.go` | Lifecycle | Full wizard lifecycle |
| `internal/wizard/falsify_test.go` | atFalsify, HTTP flow | Discharges all six |
| `internal/agentjacking/ingest_test.go` | SixDebtsUntouched | Checks debt length and items |

### 8c. Rewrite B-17 and B-23

**New B-17** (`TestW1_B17_DebtEncoding`): Proves arbitrary caller-supplied debt
encodes into `STRING[]` correctly. Create belief with `["foo", "bar", "baz"]`,
read back, verify element-for-element.

**New B-23** (`TestW1_B23_DebtDefaultIsEmpty`): Proves DDL default is empty
array. Raw INSERT without debt column, verify stored debt is `{}`.

### 8d. New regression tests

**Kernel tests** (in `kernel/kernel_test.go`):

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
| `TestCS_DEBT09_DuplicateDebtRetirement` | Create `["foo", "foo"]`, retire `"foo"`, verify `[]` |
| `TestCS_DEBT10_RetireNonexistentBelief` | RetireDebt against nonexistent belief → error |
| `TestCS_DEBT11_KernelAcceptsEmptyString` | Kernel accepts `[""]` as opaque string |

**API boundary test** (in `api/belief_test.go`):

| Test | Purpose |
|------|---------|
| `TestCS_DEBT12_APIShouldRejectEmptyString` | `POST /v1/beliefs` with `debt: [""]` → 400 |

**MCP boundary test** (in `cmd/solvent-mcp/` test files):

| Test | Purpose |
|------|---------|
| `TestCS_DEBT13_MCPShouldRejectEmptyString` | `solvent_retire_debt` with `debt_item: ""` → error |

**Migration verification test** (in `kernel/kernel_test.go`):

| Test | Purpose |
|------|---------|
| `TestCS_DEBT14_MigrationSupersedesOldDefault` | INSERT without debt after 010 → empty |

### 8e. Rename TestIntegration_RealFixtureRetiresFullDebt

Rename to `TestIntegration_RealFixtureRetiresWizardDebt` to reflect actual
semantics after `FullDebt` is removed.

### 8f. Update mcp_verify.sh

- Remove enum check (lines 121-126)
- Remove retired-name rejection test (line 142-144)
- Remove bogus-item rejection test (lines 147-148)
- Update test 4c to verify belief lookup without vocabulary guard
- Add empty-string rejection test
- Update description check (lines 127-130)

### 8g. Existing tests that must pass unchanged

- `TestW1_B03_RetireDebt` — retirement mechanism
- `TestW1_B04_RetireDebtIdempotent` — missing item no-op
- `TestW1_B05_Promote` — promotion gate
- `TestW1_B06_IntentOnPromoted` — action_intent gate
- `TestW1_B11_I3_IntentOnUnpromoted` — I-3 invariant
- `TestW2_B07_RetractCascade` — cascade semantics
- `TestCS1_RetireDebt_CrossScenario` — ErrBeliefNotFound
- `TestCS2_RetireDebt_SameScenario_Idempotent` — idempotency
- `TestDA02_RetireArbitraryDebt` — already proves arbitrary debt works

---

## 9. Migration Verification

### Pre/post state assertion

For an existing database with 004 already applied:

```
BEFORE 010:
    INSERT belief (scenario_id, claim, claim_type) VALUES (...)
    → verify debt = ARRAY['needProvenanceCheck',...]

APPLY 010

AFTER 010:
    INSERT belief (scenario_id, claim, claim_type) VALUES (...)
    → verify debt = ARRAY[]::TEXT[]

EXISTING ROW:
    → verify debt UNCHANGED (still has old six items)
```

### Bootstrap path verification

| Path | Verify |
|------|--------|
| Test suite `Reset` | 010 applied, empty default active |
| Cloud init, cold | 010 applied, empty default active |
| Cloud init, warm | 010 applied, empty default active |
| CLI `resolveSchemaPaths` | 010 included in layer list |
| `Taskfile.yml` `db:reset` | 010 applied |

---

## 10. Documentation / Taxonomy Guidance

### 10a. Kernel documentation

Update `kernel/doc.go` or package comment:

> Solvent treats debt identifiers as opaque obligation IDs. Debt semantics are
> defined by the caller/domain. Solvent only enforces whether debt is empty or
> non-empty for promotion.

### 10b. Functional taxonomy (external, non-normative)

Create guidance in `docs/patterns/` or Conductor docs:

| Role | Question |
|------|----------|
| Provenance | Where did this come from? |
| Consistency | Does it contradict what we hold? |
| Scope | Under what conditions does it hold? |
| Verification | Has anything independent checked it? |
| Impact | What does this affect if wrong? |
| Recovery | If invalidated, what then? |

Frame explicitly as guidance for domain vocabulary design. NOT a universal
literal vocabulary. NOT mandatory Solvent values. NOT enforced by code.

### 10c. Do NOT create

- `ebp:*` constants in Solvent
- Universal default debt vocabulary
- Solvent-level taxonomy enforcement

---

## 11. Decision Record

Write to `docs/OS/` as a named decision document.

**Lead with:** This change restores documented doctrine (writeup §4.3) rather
than expanding kernel scope — the implementation had drifted from the design.

**Record:**
- Why kernel was reopened (hardcoded vocabulary violates domain-agnostic boundary)
- Evidence from repository inspection (mapping.go duplication, API forced default)
- Why no universal vocabulary was introduced (would recreate coupling)
- Wizard vocabulary ownership (`internal/belief/debt.go`, single source)
- EBP Coordinator ownership (mandatory `ebpInitialDebt`, external spec)
- Grandfathering decision (existing rows untouched)
- API/MCP boundary semantics (structural validation only)
- Migration behavior (010 supersedes 004 default)
- Safety posture change and compensation
- Tests (CS_DEBT01-14, rewritten B-17/B-23)
- Final frozen commit/hash
- Supersession of `7602699`

---

## 12. Re-freeze Procedure

### Sequence

1. Implement all changes
2. Run targeted tests (new debt tests)
3. Run full suite (`task test`)
4. Verify migration/bootstrap paths
5. Repository-wide vocabulary sweep (grep all six strings)
6. Write decision record
7. Commit
8. Annotated tag on freeze commit (e.g., `v0-freeze-debt-opaque`)
9. Update freeze-baseline references in documentation
10. Record supersession of `7602699`

---

## 13. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Domain adapter forgets starting debt | Mandatory adapter specification; Coordinator enforces for EBP |
| Typo becomes zombie debt | Domain adapter validates own vocabulary before EnterBelief |
| Schema path 010 missing from test suite | `go test` fails immediately on schema application |
| MCP client caches old enum | Tool description updated to explain discovery via belief |
| `belief` → `wizard` cycle | Verified: no cycle exists |
| `mapping.go` drift from `wizardDebt` | Both files in same package, reference same constants |

---

## 14. Exact Files Expected to Change

### Schema/migration (2 files)
| File | Change | Why |
|------|--------|-----|
| `db/010_debt_opaque.sql` | NEW | Idempotent: SET DEFAULT empty array |
| `Taskfile.yml` | Add 010 to db:reset | Bootstrap completeness |

### Kernel (2 files)
| File | Change | Why |
|------|--------|-----|
| `kernel/kernel.go` | Delete `FullDebt` (lines 11-34) | Remove hardcoded vocabulary |
| `kernel/contract.go` | Remove `_ []string = FullDebt` (line 46) | Compile-time assertion for deleted var |

### Belief wiring layer (3 files)
| File | Change | Why |
|------|--------|-----|
| `internal/belief/debt.go` | NEW | Single source: named constants + `wizardDebt` |
| `internal/belief/belief.go` | `kernel.FullDebt` → `wizardDebt` (line 57) | Domain-owned starting debt |
| `internal/belief/mapping.go` | Hardcoded strings → named constants | Eliminate duplicate vocabulary |

### API (2 files)
| File | Change | Why |
|------|--------|-----|
| `api/types.go` | Add `Debt []string` to `EnterBeliefRequest` | Caller-supplied debt |
| `api/belief.go` | Use `req.Debt`, add `""` rejection (line 33) | Pass caller debt, structural validation |

### MCP (2 files)
| File | Change | Why |
|------|--------|-----|
| `cmd/solvent-mcp/tools.go` | Remove `slices.Contains` guard, add `""` rejection | Accept opaque debt, reject malformed |
| `cmd/solvent-mcp/main.go` | Remove `enum`, update description (lines 236-241) | No universal vocabulary |

### Wizard (3 files)
| File | Change | Why |
|------|--------|-----|
| `internal/wizard/seed.go` | Import `belief`, use `belief.WizardDebt()` | Domain-owned vocabulary |
| `internal/wizard/discharge.go` | `FullDebtNames()` → `belief.WizardDebt()` | Single source |
| `internal/wizard/state.go` | `kernel.FullDebt` → `belief.WizardDebt()` | Domain-owned vocabulary |

### Other callers (3 files)
| File | Change | Why |
|------|--------|-----|
| `demo/cloud/init/main.go` | Import `belief.WizardDebt()`, add 010 | Same domain |
| `cmd/operator-review/main.go` | Import `belief.WizardDebt()` | Same domain |
| `examples/github/executor/main.go` | Local example debt slice | Isolated example |
| `internal/agentjacking/ingest.go` | Import `belief.WizardDebt()` | Same demo domain |

### Schema path lists (14 test files + CLI)
All 14 `schemaPaths` definitions + `cmd/solvent/main.go` `resolveSchemaPaths`
+ `demo/cloud/init/main.go` constants.

### Test files (~15+ files)
Category A: replace with simple test debt. Category B: use `belief.WizardDebt()`.
Rewrite B-17/B-23. Add CS_DEBT01-14. Rename RealFixtureRetiresFullDebt.

### Verification script (1 file)
| File | Change | Why |
|------|--------|-----|
| `scripts/mcp_verify.sh` | Remove enum/rejection tests, add `""` test | Matches new behavior |

---

## 15. Acceptance Criteria

1. `task test` passes — full suite including new regression tests
2. No kernel-level debt vocabulary — grep for all six strings shows only
   wizard/domain/example/test usage, never kernel validation or defaulting
3. Empty default verified — fresh database gives new beliefs empty debt
4. Arbitrary debt accepted — CS_DEBT01/04 pass
5. Kernel accepts opaque `""` — CS_DEBT11 passes
6. API rejects `""` — CS_DEBT12 passes
7. MCP rejects `""` — CS_DEBT13 passes
8. Promotion gate unchanged — CS_DEBT06/07/08 pass
9. Action intent gate unchanged — existing tests pass
10. Retraction cascade unchanged — existing tests pass
11. Refusal logging unchanged — existing tests pass
12. Migration verified — 010 applied in all bootstrap paths, pre/post assertion
13. Decision record written — doctrine cited, alternatives recorded
14. New freeze tagged — annotated tag, supersession of `7602699` recorded
15. `mcp_verify.sh` updated — matches new behavior
16. No new universal vocabulary introduced — no `ebp:*` constants, no "sane
    defaults", no shared taxonomy enforcement in Solvent
17. Package dependency clean — no cycles introduced
18. `mapping.go` uses named constants — no hardcoded string literals remain
