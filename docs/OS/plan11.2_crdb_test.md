# Plan 11.2 — CRDB Test & Adversarial Review

## 1. CockroachDB Status

| Item | Value |
|------|-------|
| Status | **PASS — verified by running** |
| How started | Docker container `solvent-crdb` (pre-existing, `docker start`) |
| Image | `cockroachdb/cockroach:v26.2.1` |
| Connection | `localhost:26260` (SQL), insecure, user `root` |
| Database state | Fresh — `task db:reset` applied all migrations 001–010 |

## 2. Targeted Test Results

| Test | Result |
|------|--------|
| CS_DEBT01–CS_DEBT11, CS_DEBT14 (12 tests) | **PASS — verified by running** |
| B-17 (arbitrary encoding) | **PASS — verified by running** |
| B-23 (empty DDL default) | **PASS — verified by running** |
| TestIntegration_RealFixtureRetiresWizardDebt | **PASS — verified by running** |
| TestW3_EnsureBelief_New (defect found + fixed) | **PASS after fix — verified by running** |

## 3. Full `task test` Result

| Metric | Value |
|--------|-------|
| Total | **504 passed, 0 failed, 23 skipped, 35 packages** |
| Result | **PASS — verified by running** |

## 4. MCP Verification Result

| Check | Result |
|-------|--------|
| 1. No universal debt enum remains | **PASS** |
| 2. Arbitrary debt identifiers accepted | **PASS** |
| 3. Unknown debt identifiers not vocabulary-rejected | **PASS** |
| 4. Empty-string debt_item rejected | **PASS** |
| 5. Existing belief lookup behavior works | **PASS** |
| 6. Existing retirement behavior works | **PASS** |
| All 31 checks | **MCP VERIFY GREEN** |

## 5. Migration Pre/Post Verification

| Case | Result |
|------|--------|
| **A: Old default → 010 → new default** | Pre-migration row: 6 items preserved. Post-migration row: empty. **PASS — verified by running** |
| **B: Fresh schema through 010** | Empty debt default. **PASS — verified by running** |
| **C: Reset/bootstrap paths** | All paths include 010. **PASS — verified by running** |

### Case A detail

1. Created `fable_mig_test` with schema through 004 only.
2. Inserted belief without specifying debt — received six-item default (`needProvenanceCheck,...,needOperatorSignoff`).
3. Applied 010.
4. Inserted new belief without specifying debt — received empty default.
5. Old belief still had six items. New belief had empty.
6. Cleaned up database.

### Case B detail

1. Created `fable_fresh_test` with full schema through 010.
2. Inserted belief without specifying debt — received empty default.

### Case C detail

- `Taskfile.yml` `db:reset`: includes `010_debt_opaque.sql` — **PASS**
- `cmd/solvent/main.go` `resolveSchemaPaths`: includes `010_debt_opaque.sql` — **PASS**
- `demo/cloud/init/main.go` `debtOpaquePath`: references `010_debt_opaque.sql` — **PASS**
- All 16 test suite `schemaPaths`: include `010_debt_opaque.sql` — **PASS**

## 6. Adversarial Review Findings

### A. Domain/Kernel Coupling

- **ZERO** `kernel.FullDebt` references in production code
- **ZERO** domain vocabulary strings in kernel package
- **ZERO** `slices.Contains` vocab guard in MCP tools
- **ZERO** enum listing debt items in MCP schema
- No hidden helper recreates the six-item default
- No duplicated wizard vocabulary unnecessarily
- **PASS**

### B. Opaque-Debt Invariant (Three-Layer)

| Layer | Behavior | Verdict |
|-------|----------|---------|
| Kernel | Accepts any `[]string` including `[""]`. No content inspection. | **PASS** |
| API | Rejects empty string `""` in `debt` array elements. | **PASS** |
| MCP | Rejects empty string `""` in `debt_item`. | **PASS** |

No hidden normalization: no sort, dedup, trim, or transform between input and storage.

### C. Wizard Single Source of Truth

- Named constants in `internal/belief/debt.go` — **PASS**
- `mapping.go` references named constants — **PASS**
- All callers use `belief.WizardDebt()` — **PASS**
- `WizardDebt()` returns a defensive copy — **PASS**
- `wizardDebt` is unexported — **PASS**
- No second six-item literal list in production Go code outside `state.go` map keys

**Pre-existing observation (not a defect from this change):** `internal/wizard/state.go:85-103` uses bare string literals in `checkPrompts`/`retrievalChecks` map keys instead of `belief.Need*` constants. This is a pre-existing SSoT inconsistency not introduced by the debt-opaque change.

### D. Package Dependencies

- `belief → wizard`: **none** (correct)
- `wizard → belief`: **allowed** (correct)
- `agentjacking → belief`: **allowed** (correct)
- No transitive cycle — **PASS**

### E. Test Quality

- B-17 tests arbitrary debt encoding (not FullDebt) — **distinct contract, PASS**
- B-23 tests empty DDL default — **distinct contract, PASS**
- CS_DEBT01–14 cover: arbitrary accepted, empty accepted, nil→empty, unknown accepted, arbitrary retirement, promotion blocked, promotion allowed, final-truth blocked, duplicate retirement, nonexistent belief, kernel accepts `""`, migration default
- No false-positive tests from reduced debt sets — **PASS**

### F. Promotion/Authority Invariants

- Promotion with remaining debt → blocked (SQLSTATE 23514) — **PASS**
- Promotion with empty debt → allowed — **PASS**
- Final-truth claims → blocked — **PASS**
- Action intent → promoted belief required (SQLSTATE 23503) — **PASS**
- Retraction cascade → intents cancelled — **PASS**
- Refusal log → unchanged — **PASS**

### G. Grandfathered Data

- Migration does not rewrite existing rows — **PASS** (Case A verified)
- Migration does not translate legacy debt — **PASS**
- Migration does not auto-retire existing debt — **PASS**
- Existing wizard beliefs remain retireable — **PASS**

### H. API/MCP Contract

- Omitted debt → empty — **PASS**
- Explicit `[]` → empty — **PASS**
- Explicit arbitrary values preserved — **PASS**
- Empty string rejected at ingress only — **PASS**
- No regex/length/count restrictions — **PASS**
- JSON schema and tool descriptions consistent with behavior — **PASS**

### I. Migration/Bootstrap Drift

- `Taskfile.yml`: includes 010 — **PASS**
- `cmd/solvent/main.go`: includes 010 — **PASS**
- `demo/cloud/init/main.go`: includes 010 — **PASS**
- All 16 test suites: include 010 — **PASS**
- No alternate initialization path silently leaves 004's old default active — **PASS**

## 7. Defects Found During Review

| ID | Defect | File | Line | Fix |
|----|--------|------|------|-----|
| D-1 | `TestW3_EnsureBelief_New` asserted `debtLen == 6` despite passing `[]string{"testDebt"}` (1 item) | `kernel/kernel_test.go` | 988 | Changed `debtLen == 6` to `debtLen == 1`. Updated expected/observed strings. |

## 8. Remaining Risks

| Risk | Severity | Notes |
|------|----------|-------|
| `state.go` map keys use string literals, not named constants | Low | Pre-existing; silent drift risk if constants renamed. Not introduced by this change. |
| `FullDebtNames()` double-calls `WizardDebt()` | Low | Pre-existing pattern; minor waste, not a defect. |
| `belief.go:57` passes raw `wizardDebt` instead of `WizardDebt()` | Low | Pre-existing; safe (same package, `EnsureBelief` copies internally). |
| M2_TRANSCRIPT.md is regenerated | None | Generated file; changes are expected when tests run with new assertions. |

## 9. Exact Git Working-Tree Status

- **47 modified files** (tracked changes)
- **19 untracked files** (new: `db/010_debt_opaque.sql`, `internal/belief/debt.go`, plan/docs files)
- **0 commits made**
- **0 tags created or moved**
- **0 files deleted**

## 10. Verdict

**The debt-opaque implementation is verified against a live CockroachDB instance. All 504 tests pass. MCP verification is green. Migration behavior is confirmed. One defect (D-1) was found and fixed during adversarial review. The repository is uncommitted for manual review and commit by the owner.**
