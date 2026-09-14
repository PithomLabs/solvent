# FINAL ADVERSARIAL DIFF REVIEW — DEBT-OPAQUE CHANGE (Corrective Pass)

## A. VERDICT

**APPROVE WITH NON-BLOCKING FINDINGS**

The corrective pass is verified. Both previously identified findings (F-1: `resetAndSeed` missing `010`; F-2: wizard bare string literals) are confirmed fixed in the current diff. No new blocker or high-severity defect was introduced. The deferred `internal/m0/gate.go` finding remains correctly outside scope: the current diff does not touch that file, and no changed production path references its stale fixture text.

---

## B. FINDINGS TABLE

| ID | Severity | File / Line | Finding | Evidence | Recommended Action |
|---|---|---|---|---|---|
| F-5 (residual) | **LOW** | `internal/wizard/discharge.go:218` | `FullDebtNames()` retains its pre-change exported name. It now delegates to `belief.WizardDebt()`, so behavior is correct, but the symbol name still reads as a kernel-concept. | Diff at `discharge.go:216-220` updated the body but not the identifier. No production caller was found outside the wizard package. | Rename to `WizardDebtNames()` in a follow-up; add a `// Deprecated:` alias if any external template imports it. Not a blocker. |
| F-7 (residual) | **LOW** | `internal/wizard/seed.go:113` | Comment still opens with “Full debt, untouched — the six items” without capitalising the domain term. | Diff updated line 114 to `belief.WizardDebt() supplies` but left line 113 ambiguous. | Re-word to “Wizard debt, untouched — the six items `belief.WizardDebt()` supplies” for clarity. Not a blocker. |

No new findings were introduced by the corrective pass.

---

## C. INVARIANT STATUS

| Invariant | Status | Evidence |
|---|---|---|
| **debt opaque** | ✅ PASS | `kernel/kernel.go`: no vocabulary references. `EnterBelief`/`EnsureBelief` accept `[]string` without inspection. `RetireDebt` is `array_remove` with no vocabulary check. |
| **migration ordering** | ✅ PASS | All production/bootstrap paths apply `010_debt_opaque.sql` **after** `004_debt_vocabulary.sql`: `cmd/solvent/main.go:279`, `demo/cloud/init/main.go:80,86,136`. All 13 test suite `schemaPaths` arrays include `010` after `004`. `demo/cloud/init/main.go:136` `resetAndSeed` now includes `debtOpaquePath` (F-1 fix confirmed). |
| **wizard SSoT** | ✅ PASS | `internal/belief/debt.go` is the single definition point. All wizard callers (`seed.go`, `discharge.go`, `state.go`, `demo/cloud/init/main.go`, `cmd/operator-review/main.go`, `examples/github/executor/main.go`, `internal/agentjacking/ingest.go`) use `belief.WizardDebt()` or named constants. `internal/wizard/state.go:85-103` now uses `belief.Need*` constants (F-2 fix confirmed). |
| **API boundary** | ✅ PASS | `api/belief.go:33-42`: `nil` → `[]`, `[]` → preserved, arbitrary → preserved, `[""]` → rejected with 400. No other REST endpoint creates beliefs. |
| **MCP boundary** | ✅ PASS | `cmd/solvent-mcp/tools.go:117-122`: empty `debt_item` rejected; arbitrary non-empty accepted. No enum, no vocabulary guard. |
| **promotion** | ✅ PASS | `kernel/kernel.go:105-119`: promotion delegates to schema CHECK `promoted_is_debt_free`. Empty debt permits promotion; non-empty debt → 23514 `ErrPromotionBlocked`. `final_truth` gate unchanged (CS_DEBT08). |
| **action intent** | ✅ PASS | `kernel/kernel.go:121-169`: composite FK `(belief_id, belief_status) → belief(id, status)` plus `live_requires_promoted` CHECK. No changes to intent creation path. |
| **retraction** | ✅ PASS | `kernel/kernel.go:184-207`: cancel-first, retract-second ordering unchanged. |
| **grandfathering** | ✅ PASS | No migration or kernel path rewrites existing `belief.debt` arrays. Old wizard beliefs retain historical six-item debt; kernel reads them opaquely. |

---

## D. DIFF/SCOPE STATUS

| Category | Assessment |
|---|---|
| **unrelated changes?** | None found. All modified files are directly related to debt-opaque: kernel signature, wizard/domain callers, API/MCP boundaries, test migration, `010` migration, MCP verify script, implementation contract, and M2 transcript regeneration. |
| **stale comments?** | Two residual LOW-severity wording issues remain (F-5, F-7 above). No comment now contradicts actual behavior. |
| **weakened tests?** | No. Cardinality assertions were correctly updated (`debtLen == 6` → `debtLen == 1` or `0`). Multi-item coverage is preserved by CS_DEBT01/05/09. B-23 was correctly repurposed to verify empty default. |
| **omitted required change?** | None. `010` is registered in every bootstrap path. `kernel.FullDebt` is fully removed from production. |

---

## E. KNOWN DEFERRED M0 FINDING

`internal/m0/gate.go:42` contains stale fixture claim text: **“claim carrying its full starting debt”**.

**Current diff impact:** None. The file is **not touched** by the current diff (confirmed by `git diff -- internal/m0/gate.go` producing no output). No changed production or test path imports or references `internal/m0`. The `internal/m0` package is explicitly frozen at M0 close and is not executed by `task test`.

**Conclusion:** The finding remains correctly **deferred**. The current change does not create a new dependency that would make the stale fixture text materially dangerous to a supported workflow.

---

## F. FINAL COMMIT READINESS

**READY**

The corrective pass has addressed the two medium-severity findings from the prior review (F-1 and F-2). No new blocker or high-severity defect was introduced. The remaining items are low-severity clean-ups that do not affect correctness.

---

## G. EXPLICIT STATEMENTS

No files modified during this review.

No commit created.

No tag created.
