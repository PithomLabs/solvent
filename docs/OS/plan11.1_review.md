## chatgpt

I reviewed **Plan 11.1** against the consolidated decisions and the actual findings recorded in the plan. It is very close, but I would **not hand it to the implementation agent unchanged**. There are a few concrete inconsistencies worth fixing first.

The architectural direction is correct: empty Solvent default, caller-owned vocabulary, wizard vocabulary consolidated in the domain layer, and EBP debt attachment owned by the external Coordinator. 

## Required corrections

### 1. Fix the `wizardDebt` visibility contradiction

This is the clearest implementation bug in the plan.

The plan defines:

```go
var wizardDebt = []string{ ... }
```

in package `belief`, but later says:

```go
import "github.com/PithomLabs/solvent/internal/belief"
belief.wizardDebt
```

from package `wizard`. An unexported identifier cannot be referenced from another package.

The plan later introduces `WizardDebt()`, but the implementation sequence is contradictory.  

**Fix:** make the domain API explicit:

```go
var wizardDebt = []string{
    "needProvenanceCheck",
    "needContradictionSweep",
    "needBlastRadius",
    "needRollbackPlan",
    "needVersionPin",
    "needOperatorSignoff",
}

func WizardDebt() []string {
    return append([]string(nil), wizardDebt...)
}
```

Then:

* `belief.go` may use `wizardDebt` directly because it is in package `belief`.
* `mapping.go` may use `wizardDebt` directly.
* `wizard` package uses `belief.WizardDebt()`.

Do **not** expose the mutable slice itself.

### 2. Do not use `wizardDebt[0]`, `[1]`, etc. in `mapping.go`

This is unnecessarily fragile:

```go
Items: []string{wizardDebt[0], wizardDebt[1]}
```

It turns semantic identifiers into positional semantics. Somebody removing/reordering the slice later could silently change the meaning of the mapping.

The cleaner domain representation is named constants:

```go
const (
    debtProvenanceCheck    = "needProvenanceCheck"
    debtContradictionSweep = "needContradictionSweep"
    debtBlastRadius        = "needBlastRadius"
    debtRollbackPlan       = "needRollbackPlan"
    debtVersionPin         = "needVersionPin"
    debtOperatorSignoff    = "needOperatorSignoff"
)

var wizardDebt = []string{
    debtProvenanceCheck,
    debtContradictionSweep,
    debtBlastRadius,
    debtRollbackPlan,
    debtVersionPin,
    debtOperatorSignoff,
}
```

Then `DebtMapping` refers to the named constants.

This keeps exactly one vocabulary definition while preserving semantic readability.

### 3. Separate Solvent API structural validation from Wizard validation more carefully

The plan says:

> API boundary: structural validation (reject `""`)

That is reasonable. But there are really two boundaries:

```text
generic Solvent API
        |
        | structural validity
        v
Solvent

Wizard/EBP adapter
        |
        | vocabulary validity
        v
Solvent
```

The API should reject an empty identifier because `""` is malformed, not because Solvent understands debt semantics.

For Wizard:

```text
needProvenanceChck
```

must be rejected by the Wizard domain before calling Solvent.

That distinction is important and should be stated explicitly in the plan rather than grouping both under "API/Application boundary." The plan otherwise gets this architectural distinction right. 

### 4. Make the EBP requirement a separate acceptance gate

The plan correctly says the Coordinator does not exist in this repository. 

But acceptance criterion #1 is:

```text
task test passes
```

and none of the acceptance criteria requires the external Coordinator negative test.

That could allow the Solvent PR to be declared complete while the most important compensating safety requirement remains merely documented.

Add:

```text
External integration acceptance:
The Coordinator specification contains and passes the required
"packet omits debt -> compiled belief receives ebpInitialDebt"
regression test before the integrated research workflow is considered complete.
```

Keep it explicitly **out of the Solvent implementation PR**.

### 5. Clarify the source of `ebpInitialDebt`

The plan says:

> `ebpInitialDebt (6 items)`

but does not actually define those six in the plan.

Given the architecture we've settled on, that's acceptable **only because they belong to the external Coordinator**. But the Coordinator specification must name the actual EBP six rather than treating `ebpInitialDebt` as an abstract placeholder.

Otherwise an implementer could interpret it as "whatever debt the packet supplied."

The rule should be:

```text
EBP initial debt = Coordinator-owned EBP vocabulary
agent packet debt = additional proposal/input
compiled debt = EBP initial debt ∪ accepted packet debt
```

The EBP vocabulary itself should remain outside Solvent.

### 6. `mapping.go` consolidation should be implemented through named domain identifiers

The actual repository finding is good: `mapping.go` contains an independent copy of the six strings. Consolidating that is exactly the right move. 

But the "Alternative (cleaner)" in §6c should be removed.

The plan should no longer offer:

> keep the string literals in `mapping.go` but add a comment

That would knowingly preserve two sources of truth, precisely what the repository inspection discovered.

Make the single-source design mandatory.

### 7. Reconsider the demo/operator/example duplication

The plan says:

```text
demoDebt
operator-review local debt
github executor local debt
```

This is architecturally valid because those callers own their own policy. But the review should distinguish:

```text
same workflow vocabulary
        -> shared domain constant

different workflow vocabulary
        -> local vocabulary
```

Do not create copies merely because those files currently consume the same six identifiers.

For each of the three callers, the implementation agent should inspect whether the six-item set is genuinely their own vocabulary or merely inherited from Solvent. If it is the same wizard/deployment-review workflow, reuse the wizard/domain source. If it is genuinely independent, keep a local vocabulary.

That prevents "remove shared coupling" from turning into "create five copies."

### 8. B-23 and CS_DEBT09 are redundant

The plan has:

```text
B-23_DebtDefaultDrift
CS_DEBT09_DDLDefaultIsEmpty
```

both testing essentially the same database-default property. 

You don't need both unless the repository convention makes one specifically a historical compatibility test.

I would keep:

```text
CS_DEBT09_DDLDefaultIsEmpty
```

as the canonical regression and make B-23 either:

* replaced by that test, or
* explicitly a higher-level API/default test if it exercises a different path.

Otherwise the test suite is adding ceremony rather than coverage.

### 9. "Fail-closed → fail-open" needs more precise wording

The plan calls the safety posture:

> fail-closed → fail-open

That is rhetorically understandable, but technically it risks implying that Solvent itself was a security control whose purpose was to force six universal obligations.

The more precise statement is:

```text
Old behavior:
Solvent implicitly supplied one deployment-specific debt policy.

New behavior:
Solvent supplies no policy; the caller must supply it.

Safety responsibility moves from implicit kernel policy
to explicit application policy.
```

The plan already says this correctly in §1; use that wording in the decision record rather than relying on "fail-open." 

## What I would leave unchanged

The following parts are solid and should stay:

* `001_schema.sql` remains untouched because `004` overwrites its default on every supported path. 
* Additive `010_debt_opaque.sql`.
* Existing rows are grandfathered unchanged. 
* Optional API debt field.
* Arbitrary debt accepted.
* Empty-string rejection at the edge.
* Missing belief remains an error while missing debt item remains an idempotent no-op. 
* Removal of the MCP enum.
* Removal of `kernel.FullDebt`.
* Preservation of promotion/action-intent/retraction/refusal semantics.
* External, non-normative functional taxonomy rather than a new universal vocabulary. 
* Annotated-tag re-freeze and explicit supersession of `7602699`. 

## Final verdict

**Plan 11.1 is architecturally approved after these corrections.**

The most important implementation shape is now:

```text
                 APPLICATION / DOMAIN
          ┌─────────────────────────────┐
          │ Wizard vocabulary           │
          │ EBP vocabulary              │
          │ Domain validation           │
          │ DebtMapping                 │
          └──────────────┬──────────────┘
                         │
                    []string debt
                         │
                         ▼
                  ┌──────────────┐
                  │   SOLVENT    │
                  │              │
                  │ debt TEXT[]  │
                  │ opaque       │
                  │ empty/nonempty
                  │ promotion   │
                  └──────────────┘
```

The kernel gets **smaller**, not smarter.

The four changes I would make to Plan 11.1 before implementation are:

```text
1. Fix WizardDebt visibility/API.
2. Replace positional wizardDebt indexing with named constants.
3. Make the external EBP integration test an explicit integration acceptance gate.
4. Remove redundant B-23/CS_DEBT09 coverage unless they exercise distinct paths.
```

Everything else in the plan is directionally sound and consistent with the reviewed architecture.



## gemini

The revised Plan 11 cleanly resolves the debt vocabulary decoupling while maintaining strict boundaries between domain responsibilities and kernel mechanisms.

**Domain vs. Kernel Boundary Matrix**

| Layer | Component | Ownership & Responsibilities | Prohibited Behaviors |
| --- | --- | --- | --- |
| **Domain** | `internal/belief/debt.go` | Single source of truth for `wizardDebt` (6 items); used by `EnsureBelief` and `DebtMapping`. | Importing or referencing deleted `kernel.FullDebt`. |
| **Domain** | External EBP Coordinator | Explicitly attaches `ebpInitialDebt` during packet ingestion; preserves proposed agent debt. | Relying on Solvent to supply default research obligations. |
| **Ingress** | `api/belief.go` & `cmd/solvent-mcp` | Accepts optional `Debt []string` (nil defaults to `{}`); rejects empty string (`""`) elements. | Enforcing regex, length, or vocabulary white-lists on debt identifiers. |
| **Kernel** | `kernel/kernel.go` & Postgres DDL | Stores opaque `TEXT[]` strings; defaults to `ARRAY[]::TEXT[]`; gates promotion on empty array. | Normalizing, lowercasing, enumerating, or inspecting debt string contents. |

---

**Implementation Matrix by Repository Module**

**Database & Migration**

* `db/010_debt_opaque.sql`: Additive migration setting `ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];`.
* `cmd/solvent/main.go`, `demo/cloud/init/main.go`, `Taskfile.yml`: Register `010_debt_opaque.sql` across schema paths and reset tasks.

**Kernel Core**

* `kernel/kernel.go`: Remove `FullDebt` declaration and associated documentation comments.
* `kernel/contract.go`: Remove compile-time variable assertion `_ []string = FullDebt`.

**Belief & Wizard Domain**

* `internal/belief/debt.go`: Define package-level `wizardDebt` containing the 6 deployment-review identifiers.
* `internal/belief/belief.go` & `internal/belief/mapping.go`: Route `EnsureBelief` and `DebtMapping` directly to `wizardDebt`.
* `internal/wizard/seed.go`, `discharge.go`, `state.go`: Update calls to reference `belief.WizardDebt()`.

**API & Tooling Boundary**

* `api/types.go` & `api/belief.go`: Expose `Debt []string` in `EnterBeliefRequest`; validate `d != ""` per item.
* `cmd/solvent-mcp/tools.go` & `main.go`: Delete `slices.Contains` validation block and remove `"enum"` property from JSON schema.
* `scripts/mcp_verify.sh`: Remove enum assertion and unknown debt rejection checks.

---

**Verification and Freeze Protocol**

* **Regression Suite:** Add tests `TestCS_DEBT01` through `TestCS_DEBT12` to `kernel/kernel_test.go` covering arbitrary strings, empty arrays, duplicate array-removal semantics, and API empty-string rejection.
* **Refactored Drift Tests:** Replace B-17 with `TestW1_B17_DebtEncoding` (arbitrary slice encoding) and B-23 with `TestW1_B23_DebtDefaultDrift` (verifying DB DDL defaults to `{}`).
* **Repository Vocabulary Sweep:** Verify via codebase search that the 6 deployment strings appear exclusively inside `internal/belief/`, wizard domain files, examples, and test fixtures.
* **Baseline Freeze:** Record supersession of commit `7602699` in `docs/OS/`, commit changes, and issue an annotated tag (e.g., `v0-freeze-debt-opaque`).


## Z

Plan 11–Revised review. Verdict: **approvable — this is the most execution-ready artifact in the project's history, and every prior finding landed with evidence rather than assertion.** The repository findings section is the difference: branch B confirmed by inspection, the bootstrap path traced (001 never load-bearing — M2 resolved with the *correct* answer, leave it untouched), the unknown-debt rejection tests located where I predicted but couldn't verify (shell script, not Go), retire semantics confirmed by existing test names (B-04 idempotent no-op, DA-02 arbitrary vocabulary — meaning the kernel was already generic and only the constant was hardcoded, which strengthens the doctrine-restoration framing), and the freeze-tag convention found deficient (no annotated tag on `7602699` — acknowledged and fixed going forward). The four residual items from my last review all landed: coordinator obligation as spec-with-negative-test (§7), single-source `wizardDebt` (§6), annotated-tag re-freeze with supersession (§12), doctrine-first decision record (§11). The floor survived as guidance-not-code (§10b), the Gemini rejections held, and the degenerate cases got tests (CS_DEBT10–12). What remains is the residue a final pass should catch — small, concrete, and none architecture-class.

## High

**H1 — §6d/§6e contains an internal package-cycle risk the plan hasn't checked.** `internal/wizard/discharge.go` replaces `FullDebtNames()` with a function referencing `belief.WizardDebt()` — while §6c's `mapping.go` (package `belief`) and §6a's `debt.go` (package `belief`) own the constant. Check the actual import direction: does the `wizard` package already import `belief`, and does `belief` import `wizard` anywhere (seed flows, state types)? If any file in `internal/belief/` imports `internal/wizard`, the §6d decision ("wizard imports belief") creates a cycle and the build fails at compile time — an annoyingly late discovery for something checkable in thirty seconds now. The plan itself wobbles here ("import `belief.wizardDebt` **or** define its own local copy" / "export the accessor **if needed**") — resolve the conditional before coding, and if a cycle exists, the fallback is a tiny shared package (`internal/domain/debt.go`) rather than the two-copy drift being avoided.

**H2 — §8d's "~100 occurrences, replace with explicit inline slices" hides contract-meaning drift.** Tests currently using `kernel.FullDebt` aren't interchangeable: some use it as *arbitrary starting debt* (any non-empty works — inline `"testDebt"` fine), but others assert *specific* semantics — retirement-order tests, discharge-completeness flows, tests that retire items by name and expect promotion to unlock only at the last one. A blind find-replace to `[]string{"testDebt"}` breaks the latter class or, worse, passes vacuously (one-item debt retires once, test green, coverage gone). The sweep must classify each call site: *any-nonempty sufficient* → simple slice; *semantic debt-set required* → preserve the full six via `wizardDebt`/`belief.WizardDebt()`. Estimate the split by grepping test names for retire/promote/discharge patterns before rewriting; expect roughly a 70/30 split, and the 30% are the ones that carry regression weight.

## Medium

**M1 — CS_DEBT10 tests the API boundary but the MCP path bypasses it.** The empty-string rejection lives in `api/belief.go` (§5c); `tools.go`'s retired guard also covered this transitively (the enum couldn't contain `""`). After removal, an MCP `debt_item: ""` reaches the kernel unvalidated. Either replicate the structural check in the MCP handler (three lines, same location as the removed guard) or add CS_DEBT13 documenting kernel-level acceptance of `""` as the deliberate contract. Pick one; the current plan has a hole where a rule used to be.

**M2 — §9's verification step 2 needs a pre-state assertion.** "Existing DB with 004 applied: apply only 010, verify new inserts get empty default" — add *before* applying 010: insert one row, confirm it gets the six-item default, then apply 010, insert again, confirm empty. Without the before-assertion, a wrong-supersession assumption (e.g., 004's default interpreted at table-creation only) produces a false pass.

**M3 — The decision record should also record the *fail-open* boundary condition, not just the posture change.** §11 lists "safety posture change (fail-closed → fail-open, compensated by adapter obligation)" — one row deeper: *the compensation is enforced where?* Answer: coordinator packet-validator (EBP), wizard seed path (wizard), and — explicitly unnamed anywhere — **operator-review, demo, and examples now have nobody enforcing their obligation except their own code**. One sentence each in the record naming who enforces what per domain. That's the document someone consults in a year when a new adapter inherits the empty default.

**M4 — `belief.go` import cleanup note is correct but verify against §14's file list.** §6b says the `kernel` import survives (New, claim types, errors) — good; just ensure §14's "kernel (2 files)" table doesn't tempt a future editor into thinking `internal/belief/` is kernel-untouched. It's wiring-layer, correctly listed separately. No action if the table stays as-is — this is a reminder, not a change.

## Low

`TestIntegration_RealFixtureRetiresFullDebt` (§8f) — rename it; "FullDebt" dies with the constant, and the name will misdirect the next grep-sweep. §6c's "alternative (cleaner)" (literals + comment) is *less* clean — the consolidation already chose the constant reference; strike the alternative from the plan so the coding agent doesn't exercise unrequested judgment. CS_DEBT11's expectation ("retire removes both") should assert the *array state* post-retire (`[]`), not just the nil error, or the test passes under either semantic.

## The closing observation

Worth recording in the decision record itself, because it's the project-level point: **the repository findings in §2 retroactively justify the entire review process.** Branch B (two byte-identical copies, independently maintained) is textbook drift already happened; the untested MCP guard is a security control with zero coverage; the missing freeze tag is process debt sitting in the baseline. None of that was visible from the architecture documents — only from inspecting the actual repository. The seven rounds of specification review produced the doctrine; the doctrine then correctly licensed this contraction; and the repo inspection found the implementation had drifted from both. That's the full governance loop — spec, freeze, doctrine, amendment, evidence — working end to end, and it should be cited as precedent in §11: *kernel changes are judged against the kernel's own frozen principles, and this one passed because it made the kernel smaller.*

Execute with H1/H2 resolved pre-merge. After this lands and re-freezes, the kernel is cleaner than the day it was frozen — and the coordinator spec inherits a debt-attachment rule with a negative test waiting for it. That's the sequence working.


## claude

This revision is substantially stronger than the synthesis it's based on — §2's "Actual Repository Findings" resolved every open empirical question with real evidence (branch B confirmed for `mapping.go` with exact line citations, the bootstrap-path table proving `001`'s edit would be cosmetic, the freeze-tag convention actually audited rather than assumed). That's the right way to close out a "verify before implementing" instruction. Three things worth fixing before this goes to a coding agent, though, and they're all the same species of issue: places where an earlier round's open question got *partially* resolved and the unresolved half got left in the document as if it were still live.

## CS_DEBT10 tests the wrong layer, and if implemented as placed, it would assert behavior that contradicts this plan's own architecture

§8c puts `TestCS_DEBT10_EmptyStringRejected` under the heading "New regression tests (**kernel_test.go**)," alongside the other eleven kernel-level tests. But §5c is explicit that empty-string rejection is an **API-boundary** check, not a kernel one — and the plan's central thesis, restated in §10c, is that Solvent "never enumerates, validates, normalizes, or interprets debt strings." That means `kernel.EnterBelief` called directly with `[""]` *must succeed* — rejecting it at the kernel level would itself be a violation of the opaque-debt invariant this whole plan exists to establish. As currently placed, CS_DEBT10 either gets implemented against `kernel.EnterBelief` and asserts a rejection that shouldn't happen (a real architecture bug, not just a misplaced test), or someone has to notice the mismatch during implementation and relocate it — better to fix it now than let a coding agent discover the contradiction mid-task. Two fixes needed: move CS_DEBT10 to wherever `api/belief.go`'s handler gets tested, and add a *new*, genuinely kernel-level test proving the opposite — that `kernel.EnterBelief` accepts an empty string without complaint, since that's the actual invariant being protected. Right now there's no test anywhere confirming the kernel doesn't validate; there's only a misplaced test that would assert it does.

## `mapping.go`'s "Alternative (cleaner)" directly reopens the item marked blocking last round

§6c presents the fix (replace hardcoded literals with `wizardDebt` references) and then, in the same section, offers an "**Alternative (cleaner):** Keep the string literals in `mapping.go` but add a comment stating they must match `wizardDebt`." That alternative is exactly the two-independently-maintained-copies problem the previous round flagged as **ACCEPT — blocking** and this round's §2 investigation specifically confirmed as real ("Branch B — independent declarations of the same six strings... maintained independently"). A comment promising two things stay in sync is not a fix for that — it's a restatement of the exact failure mode already observed in the repository. §14's file-change table asserts the decision was actually made ("Replace hardcoded strings → `wizardDebt` refs"), so this reads like leftover brainstorming that should have been deleted once §14 was written, not a live option. I'd delete the "Alternative (cleaner)" paragraph outright rather than leave it sitting next to the real decision — a coding agent skimming for the easier path could reasonably take it, and "cleaner" is doing a lot of unearned persuasive work in that sentence for an option that reintroduces a bug this plan spent a whole finding closing.

## The proposed `mapping.go` fix itself has a fragility worth naming before it ships

Even the *correct* option — `Items: []string{wizardDebt[0]}`, `wizardDebt[1]`, etc. — references the shared vocabulary by **positional index** rather than by name. That's a real, if smaller, risk: if anyone ever reorders `wizardDebt`, inserts a new item in the middle, or removes one, every numeric index in `mapping.go` silently starts pointing at a different semantic item, with no compiler error and no obvious test failure unless something specifically asserts the mapping's content rather than just its shape. Given this whole plan is about eliminating implicit, easy-to-break coupling in favor of explicit structural relationships, I'd replace the bare slice with named constants (`NeedProvenanceCheck = "needProvenanceCheck"`, etc.) composed into `wizardDebt`, so `mapping.go` can reference `belief.NeedProvenanceCheck` directly instead of `wizardDebt[0]`. Same single-source-of-truth property, but the dependency becomes readable and reorder-safe instead of positional.

One smaller echo of the same pattern: §6e's discharge.go update still says "Export the accessor if needed, or make the wizard define its own local" — but §6d already made this decision explicitly ("the wizard package imports `belief.wizardDebt`"). Worth trimming that sentence down to match the decision rather than re-presenting it as open.

**Everything else holds up well** — the risk table's mitigations are concrete rather than aspirational (a schema-path omission failing `go test` immediately is a real, verifiable safety net, not just a hope), the grandfathering language is unchanged and correct, and adding an actual annotated tag for this freeze (where the prior one, per §2, had none) is a genuine process improvement worth keeping, not just precedent-following. Fix the three items above and this is ready to implement.

