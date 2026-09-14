## chatgpt

Plan 11.2 is materially stronger and now resolves the major architecture issues. The package graph, `mapping.go` ownership, MCP/API boundary, kernel-vs-adapter empty-string semantics, test classification, migration pre/post check, and re-freeze process are all now explicit. 

I would make **three final corrections before implementation**.

### 1. `wizardDebt` should not be a duplicated slice

The plan has:

```go
func WizardDebt() []string {
    return []string{
        NeedProvenanceCheck,
        ...
    }
}

var wizardDebt = WizardDebt()
```

This creates a second representation of the same vocabulary, even though the strings themselves come from named constants. It is not severe, but it is unnecessary.

Prefer:

```go
var wizardDebt = []string{
    NeedProvenanceCheck,
    NeedContradictionSweep,
    NeedBlastRadius,
    NeedRollbackPlan,
    NeedVersionPin,
    NeedOperatorSignoff,
}

func WizardDebt() []string {
    return append([]string(nil), wizardDebt...)
}
```

That gives:

```text
one vocabulary definition
one canonical slice
defensive copy for external callers
```

The plan's current structure is otherwise correct. 

### 2. The test classification needs one correction

The plan says Category B includes:

```text
kernel/kernel_test.go | B01, B17, B23
```

and says these require the wizard vocabulary. But the new B-17 explicitly tests arbitrary caller-supplied debt:

```text
["foo", "bar", "baz"]
```

So B-17 should be Category A, not Category B.

B-23 is the DDL default test and does not inherently require wizard vocabulary either.

The classification should therefore be based on **what the test actually asserts**, not which file it lives in. The plan's 53/34 split is useful, but the specific examples should be verified once more before implementation. 

### 3. The `ebpInitialDebt` count should not be frozen as "6 items" in the generic plan

The boundary diagram says:

```text
ebpInitialDebt (6 items)
```

That is only safe if the current EBP Coordinator specification explicitly defines six initial debts at the time this plan is implemented.

Architecturally, the stronger contract is:

```text
ebpInitialDebt = Coordinator-owned mandatory EBP starting set
```

The important invariant is that the Coordinator attaches the required set; Solvent does not know how many items it contains.

This avoids accidentally freezing the number six as another hidden cross-layer assumption.

### Everything else

I would now consider these settled:

* `001_schema.sql` remains untouched. 
* `010` becomes the authoritative default migration.
* Existing beliefs are grandfathered.
* `Debt []string` remains optional.
* Empty string is rejected at API/MCP but accepted by kernel.
* MCP loses the vocabulary enum.
* Wizard owns its six-item vocabulary.
* `mapping.go` uses named constants rather than literals.
* EBP enforcement belongs to the external Coordinator.
* The functional taxonomy remains non-normative.
* `FullDebt` disappears from the kernel.
* Existing authority/lifecycle semantics remain unchanged.
* The new freeze is explicitly tagged.

The plan is now **implementation-ready after those three small edits**. The core acceptance criteria are also correctly aligned with the architecture, particularly the explicit proof that the kernel accepts opaque `""` while API/MCP reject it. 

I would make those edits and then move directly to implementation; another architecture review cycle is no longer adding meaningful structural value.



## gemini

Plan 11.1 establishes a clean separation between domain vocabulary ownership and kernel mechanisms. Decoupling the six-item deployment review set from the kernel restores the architectural invariant that debt identifiers remain strictly opaque to Solvent.

**Component Boundary Matrix**

| Component | Responsible For | Prohibited From |
| --- | --- | --- |
| **Solvent Kernel** | Storing `debt TEXT[]`, idempotent retirement, and empty-vs-non-empty promotion gating.

 | Enumerating, validating, normalizing, or interpreting string contents.

 |
| **API / MCP Ingress** | Structural validation (rejecting `""` items) and default empty array mapping for omitted inputs.

 | Enforcing regex, length limits, or vocabulary white-lists.

 |
| **Wizard Domain** | Defining `wizardDebt` (6 items) in `internal/belief/debt.go` and driving `DebtMapping`.

 | Importing or depending on kernel-level debt constants.

 |
| **EBP Coordinator** | Explicitly attaching `ebpInitialDebt` during external packet compilation.

 | Expecting Solvent to supply research obligation defaults.

 |

**Execution & Repository Mapping**

* **Database Migration:** `db/010_debt_opaque.sql` sets `ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];` without altering existing historical rows. Registered across all 14 schema path definitions, CLI `resolveSchemaPaths`, and cloud initialization.


* **Kernel Cleanup:** `kernel.FullDebt` and its contract assertions are fully removed from `kernel/kernel.go` and `kernel/contract.go`.


* **Domain Consolidation:** `internal/belief/debt.go` serves as the sole source of truth for named constants `NeedProvenanceCheck`, `NeedContradictionSweep`, `NeedBlastRadius`, `NeedRollbackPlan`, `NeedVersionPin`, and `NeedOperatorSignoff`.


* **Boundary Validation:** Structural guards in `api/belief.go` and `cmd/solvent-mcp/tools.go` reject empty strings (`""`) with HTTP 400 or tool errors while passing valid arbitrary strings directly to the ledger.



**Verification & Re-Freeze Protocol**

* **Regression Coverage:** Implement tests `TestCS_DEBT01` through `TestCS_DEBT14` covering arbitrary debt acceptance, empty array defaults, array-remove semantics for duplicate entries, and structural empty-string rejections.


* **Drift Tests:** Replace legacy B-17/B-23 tests with arbitrary slice encoding (`TestW1_B17_DebtEncoding`) and database DDL empty default checks (`TestW1_B23_DebtDefaultIsEmpty`).


* **Final Tagging:** Execute a repository-wide vocabulary sweep, verify full suite passing (`task test`), document supersession of commit `7602699` in `docs/OS/`, and issue an annotated tag (`v0-freeze-debt-opaque`).


## Z

Plan 11.1 review — final pass. Verdict: **approved for execution.** Every finding from the previous round landed, most with repository-verified evidence rather than assertion: the package-cycle check (H1) was run and resolved with a dependency table; the test classification (H2) was done with real counts (53/34) and named files; the MCP empty-string hole (M1) got both a boundary guard and its test (CS_DEBT13); the pre/post migration assertion (M2) is in §9 with the before-state explicitly asserted; the per-domain enforcement map (M3) is the §7c table — which is the exact document a future adapter author needs; the RealFixture rename landed (8e); the unneeded alternative was struck (6c now uses named constants only); and CS_DEBT11 flipped to the deliberate-contract form (kernel accepts `""`, boundaries reject) with the API/MCP/kernel split made coherent — that's the right three-layer answer to the degenerate case, each layer's behavior now specified and tested. The disposition table between planning and repository findings is the strongest section: this plan *knows things about the repo* that the architecture documents never did, and says where each came from.

Residual items, all minor and none blocking — they're notes for the executing agent, not gates:

1. **`internal/agentjacking/ingest.go` appears in §14 but not in §2's dependency table.** The cycle check asserted edges for belief↔wizard and the consumers; if `agentjacking` imports `belief` (it will, per §6e), confirm it doesn't also create any transitive edge back. Thirty-second check during implementation; expected fine, but the §2 table should gain the row if it's checked.

2. **`WizardDebt()` returns a fresh slice per call (§6a) while `wizardDebt` package var caches one copy — §6b uses the unexported `wizardDebt`.** Fine as written, but the plan should note the *invariant*: callers must treat returned slices as read-only (defensive copy exists precisely so mutation can't corrupt the shared cache — but the cached `wizardDebt` var is itself shared, so any future code that appends to it corrupts every later caller). One comment in `debt.go` stating "do not mutate the package-level slice; always via `WizardDebt()` for external use" closes it.

3. **CS_DEBT08's "final-truth claim cannot be promoted" predates this change — verify it exists today before counting it as new coverage.** If it's an existing test, list it under §8g (must-pass-unchanged) rather than §8d (new); the distinction matters for the decision record's claim about what this change added versus preserved.

4. **`mcp_verify.sh` line numbers shift after the §8f deletions** — the subsequent edits in that file should be made bottom-up or by pattern, not by the cited line numbers, or the fourth edit lands on the wrong block. Classic shell-script edit hazard; one sentence in the plan.

5. **Acceptance criterion 2's grep sweep should include the test files' *new* literals.** After replacement, `"needProvenanceCheck"` will appear legitimately in `debt.go`, wizard tests, and possibly the decision record — the sweep's pass condition should be phrased as "no occurrences outside `internal/belief/debt.go`, wizard/demo/example usage, and test fixtures" so the criterion is checkable rather than aspirational.

None of these change the plan's structure. Execute as written with those five notes in hand.

## What this plan represents — worth one paragraph in the record itself

This is the first artifact in the entire project where **the repository contradicted the documents, the documents were found to have drifted from their own doctrine, and the change licensed was a contraction** — kernel scope reduced, vocabulary returned to the layer the doctrine always said owned it, the safety compensation specified and tested rather than assumed. The governance loop ran completely: doctrine (§4.3) → drift discovered by inspection → amendment judged against the doctrine → evidence (tests, sweeps, migration assertions) → re-freeze with supersession. Seven months of review process existed to make exactly this cycle trustworthy, and this plan is what it looks like when it works: the kernel comes out *smaller* than the day it was frozen, which per its own §2 is the only direction it was ever supposed to grow.

Land it, tag it, write the record with the doctrine citation leading. Then the coordinator spec inherits a debt-attachment rule (§7) with a negative test already specified — and the research POC proceeds on a kernel that is now, finally, exactly as domain-agnostic as the architecture always claimed it was.



## claude

All three findings are resolved cleanly, and in two places the fix goes a bit further than what I asked for rather than just meeting it.

**CS_DEBT10's misplacement is fully corrected, and the three-way split is better than the two-way split I suggested.** `TestCS_DEBT11_KernelAcceptsEmptyString` now proves the kernel *accepts* an opaque empty string — the actual missing test, confirming the invariant rather than contradicting it — while `TestCS_DEBT12` (API, 400) and `TestCS_DEBT13` (MCP, error) each test rejection at the layer that's actually supposed to reject it. That's a cleaner mapping than my original two-test suggestion, since API and MCP are separate boundaries with separate handler code, and testing them separately means a bug in one doesn't hide behind the other passing.

**The "Alternative (cleaner)" paragraph is gone**, and `mapping.go`'s update in §6c now shows only the named-constants version, consistent with §14's file table. No reopened path back to the two-copy problem.

**The positional-index fragility is fixed, and `WizardDebt()` returning a defensive copy is a good, unprompted addition.** Named constants (`NeedProvenanceCheck`, etc.) mean `mapping.go` now reads as self-documenting rather than requiring a reader to cross-reference index numbers against the slice definition, and returning a fresh copy from `WizardDebt()` rather than exposing the backing slice directly closes off a mutation risk nobody asked about but is the right instinct given how central "single source of truth, never silently diverges" is to this whole plan.

One small, non-blocking wording inconsistency worth a one-line fix before this ships: §7c's responsibility table describes operator-review as owning "**Local debt slice** at belief creation," while §6e and §14 both correctly specify it should **import** `belief.WizardDebt()` — the same shared source, not an independent copy. The actual instructions (§6e, §14) are right and consistent with each other; only §7c's summary prose uses looser language that could read as contradicting them. Worth tightening that one cell so a reader skimming just the responsibility table doesn't get a different impression than the implementation section gives.

That aside, §2's investigation additions (the package-dependency-cycle check, confirming `belief`↔`wizard` has no import cycle before committing to that architecture) is exactly the kind of thing that should be verified before, not discovered during, implementation. This is ready to build.