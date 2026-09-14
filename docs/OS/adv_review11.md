# ADVERSARIAL CODE REVIEW — DEBT-OPAQUE CHANGE

## A. EXECUTIVE VERDICT

**APPROVE WITH NON-BLOCKING FINDINGS**

The core invariant `DEBT IS OPAQUE TO SOLVENT` is maintained. The kernel no longer owns vocabulary, the HTTP API and MCP boundaries enforce the empty/non-empty contract correctly, and `010_debt_opaque.sql` makes the database default empty in all normal bootstrap paths. No blocker was found.

---

## B. FINDINGS TABLE

| ID | Severity | File / Line | Finding | Why It Matters | Recommended Fix |
|---|---|---|---|---|---|
| F-1 | **MEDIUM** | `demo/cloud/init/main.go:136` | `resetAndSeed` omits `db/010_debt_opaque.sql` from its `ApplySchema` call. It applies `corpusSchemaPath`, `wizardSchemaPath`, and `debtSchemaPath` (`004`) but not `debtOpaquePath` (`010`). | After a verification-failure reset, `004` runs `ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY['needProvenanceCheck', …]`, reverting the column default back to the old six-item debt. The database schema is left in an inconsistent state. Any future caller that relies on the DB default (raw SQL insert, new helper, example) would receive the old vocabulary instead of the intended empty array. | Add `debtOpaquePath` to the `ApplySchema` call in `resetAndSeed`, or document why omitting it is intentional and safe. |
| F-2 | **MEDIUM** | `internal/wizard/state.go:85-103` | `checkPrompts` and `retrievalChecks` maps, plus `ContradictionCheck` constant, use bare string literals (`"needProvenanceCheck"`, etc.) instead of the named constants exported from `internal/belief/debt.go`. | If the wizard vocabulary changes in `debt.go`, these literals drift silently. The “single source of truth” contract is violated inside the wizard package itself — the domain owner. | Replace bare literals with `belief.NeedProvenanceCheck`, `belief.NeedContradictionSweep`, etc. |
| F-3 | **LOW** | `db/001_schema.sql:19-28` | The CREATE TABLE still embeds the old six-item `ARRAY[...]` default and the comment asserts “This literal and `kernel.FullDebt` are the two encodings of one fact.” | `kernel.FullDebt` no longer exists. The comment is now false. The embedded default is overridden by `010` at bootstrap, but anyone reading `001` is misled about where the canonical default lives. | Update the comment to reflect that `010_debt_opaque.sql` sets the canonical default, or move the empty default into `001` and drop `010`’s ALTER. |
| F-4 | **LOW** | `db/004_debt_vocabulary.sql` | The migration still runs `ALTER TABLE … SET DEFAULT ARRAY['needProvenanceCheck', …]`. | In the current bootstrap order `004` is always followed by `010`, so the file is now a no-op that re-states a retired default. It obscures the migration history and could confuse a future operator who sees two competing defaults. | Either delete the file (and document that `010` supersedes it) or rewrite it as a true no-op with a header explaining it is retired. |
| F-5 | **LOW** | `internal/wizard/discharge.go:216-218` | `FullDebtNames()` is a leftover exported name from the `kernel.FullDebt` era. | External callers (templates, tests) still import this symbol. The name no longer corresponds to anything in the kernel and invites grep-searches that return false positives. | Rename to `WizardDebtNames()` and add a `// Deprecated:` alias if backward compatibility matters. |
| F-6 | **LOW** | `internal/belief/belief.go:57` | `Process` hardcodes `wizardDebt` (the deployment-review vocabulary) in a package whose doc comment claims “no business logic, no orchestration.” | The package is named generically (`belief`) but silently manufactures domain policy. An independent domain importing this package would receive wizard debt without realizing it. | Either rename the package to signal it is wizard-domain, or make the starting debt a parameter of `Process`. |
| F-7 | **LOW** | `internal/wizard/seed.go:113,125`; `internal/wizard/state.go:118` | Comments state the six items are “the DDL default” or “Full debt, untouched — the six items the DDL default issued.” | The DDL default now issues `ARRAY[]::TEXT[]`. The six items come from `belief.WizardDebt()`, not the database. The comment describes a world that no longer exists. | Update comments to say the wizard explicitly supplies `belief.WizardDebt()`; the DDL default is empty. |

---

## C. INVARIANT VERIFICATION

| Invariant | Status | Evidence |
|---|---|---|
| **Debt opaque** | ✅ PASS | `kernel/kernel.go` no longer references any vocabulary. `EnterBelief` and `EnsureBelief` accept `[]string` without inspection. `RetireDebt` is `array_remove` with no vocabulary check. |
| **API boundary** | ✅ PASS | `api/belief.go:33-42` rejects `[""]`; `nil`/omitted/`[]` normalize to `[]string{}`. No other REST endpoint creates beliefs. |
| **MCP boundary** | ✅ PASS | `cmd/solvent-mcp/tools.go:117-122` rejects empty `debt_item`. No MCP tool creates beliefs directly. |
| **Wizard vocabulary ownership** | ⚠️ PASS WITH WARNING | Vocabulary is defined in `internal/belief/debt.go` and consumed via `belief.WizardDebt()`. Wizard callers (`seed.go`, `discharge.go`, `state.go`, `demo/cloud/init/main.go`, `cmd/operator-review/main.go`) all use it. **Exception:** `wizard/state.go` and `wizard/discharge.go` still contain bare string-literal debt identifiers (F-2). |
| **Promotion** | ✅ PASS | `kernel/kernel.go:106-120` delegates to `sqlPromote`; `promoted_is_debt_free` CHECK is the gate. Empty debt → promotion may proceed; non-empty debt → 23514 refusal. `final_truth` is also gated (CS_DEBT08). |
| **Action intent** | ✅ PASS | `kernel/kernel.go:128-153` and `sqlIntentOnPromoted` use composite FK `(belief_id, belief_status) -> belief(id, status)` plus `live_requires_promoted` CHECK. |
| **Retraction** | ✅ PASS | `kernel/kernel.go:185-208` cancels live intents first, then retracts descendants. |
| **Migration** | ⚠️ PASS WITH WARNING | All normal bootstrap paths (`cmd/solvent/main.go`, every test suite `schemaPaths`, `demo/cloud/init/main.go` cold-start path) apply `010_debt_opaque.sql` AFTER `004`. **Exception:** `demo/cloud/init/main.go:136` `resetAndSeed` omits `010` (F-1). |
| **Grandfathering** | ✅ PASS | No code path rewrites existing `belief.debt` arrays. Old wizard beliefs retain their historical six-item debt; the kernel reads them opaquely. |

---

## D. SCOPE REVIEW

The intended change scope is:
- Remove `kernel.FullDebt`
- Expose caller-supplied debt
- Move wizard vocabulary to `internal/belief/debt.go`
- Update migrations (`010`)
- Preserve authority semantics

No unrelated refactors, debug statements, or generated artifacts were found in the diff. The `docs/M2_TRANSCRIPT.md` change is a regenerated test transcript (timestamps and per-case elapsed times), not a behavioral change.

One minor scope observation: `internal/belief/belief.go` was modified to use `wizardDebt` inside `Process`. This is a behavioral change — `Process` now always injects wizard debt regardless of caller domain. It is related to the migration (the old code injected `kernel.FullDebt`, which was the same six strings), but it reinforces the observation in F-6 that this package is not actually domain-agnostic.

---

## E. TEST-QUALITY ASSESSMENT

The tests were migrated from `kernel.FullDebt` to either `[]string{"testDebt"}` or `belief.WizardDebt()`. The new `CS_DEBT` regression tests (DEBT-01 through DEBT-14) explicitly exercise the opaque-debt contract:

- Arbitrary non-empty debt accepted (DEBT-01)
- Explicit empty debt accepted (DEBT-02)
- Nil debt normalizes to empty (DEBT-03)
- Unknown vocabulary accepted (DEBT-04)
- Arbitrary debt can be retired (DEBT-05)
- Promotion blocked while arbitrary debt remains (DEBT-06)
- Promotion succeeds with empty debt (DEBT-07)
- `final_truth` still blocks promotion (DEBT-08)
- Duplicate debt retirement removes all occurrences (DEBT-09)
- Nonexistent belief → `ErrBeliefNotFound` (DEBT-10)
- Empty-string debt accepted by kernel (DEBT-11) — opaque to kernel, rejected at API/MCP boundary
- Migration supersedes old default (DEBT-14)

**Tests still prove the intended invariants.** The cardinality checks that previously asserted `debtLen == 6` were correctly updated to assert `debtLen == 1` (for single-item test debt) or `debtLen == 0` (for empty debt). The old `B-23` drift test was repurposed to verify the DDL default is empty.

One soft spot: many existing kernel tests now use `[]string{"testDebt"}` (one item) instead of six. The multi-item scenarios are covered by DEBT-01, DEBT-05, and DEBT-09, but the everyday kernel paths (EnterBelief, RetireDebt, Promote) are exercised with single-item debt. This is acceptable because the kernel treats the array opaquely, but it is a coverage reduction compared to the pre-change suite.

---

## F. EXACT BLOCKERS BEFORE COMMIT

**None.** No blocker was identified. The two MEDIUM findings (F-1, F-2) should be addressed but do not prevent commit if the owner accepts the residual risk:

- **F-1 residual risk:** The demo explicitly passes `belief.WizardDebt()` to every `EnterBelief` and `EnsureBelief` call, so the wrong default in `resetAndSeed` does not affect the demo’s own behavior. The risk is to future callers that might rely on the DB default.
- **F-2 residual risk:** The wizard is the domain owner of its vocabulary. Bare literals inside the wizard package are a maintenance hazard, not a semantic breach — the kernel still sees only opaque strings.

---

## G. EXPLICIT STATEMENTS

> **No files modified during this review.**

> **No commit created.**

> **No tag created.**
