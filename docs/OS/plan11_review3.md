## gemini

This disposition draws a pristine line between **Vocabulary (Domain/Application)** and **Enforcement (Kernel)**. Stripping vocabulary knowledge out of Solvent while mandating that domain adapters (Wizard, EBP Coordinator) explicitly inject their own initial obligations completely solves the coupling problem without creating fail-open vulnerabilities in production workflows.

**Responsibility & Ownership Boundary**

| Component | Responsible For | Strictly Prohibited From |
| --- | --- | --- |
| **Solvent Kernel** | Storing `debt TEXT[]`, enforcing `promoted_is_debt_free` gate (empty vs. non-empty), idempotent retirement. | Enumerating, validating, normalizing, or interpreting debt string values. |
| **API Ingress** | Structural validation (rejecting `""` items), default empty slice mapping for omitted JSON fields. | Imposing arbitrary length/regex limits or normalizing string casing. |
| **Wizard Domain** | Owning `wizardDebt` array and `DebtMapping` as the single application source of truth. | Importing or expecting `kernel.FullDebt`. |
| **EBP Coordinator** | Explicitly injecting `ebpInitialDebt` onto candidate beliefs compiled from ingested packets. | Relying on Solvent to supply default research obligations. |

---

**Edge-Case Handling Contract**

* **Empty String (`""`):** Rejected at API/Adapter validation as a malformed identifier.
* **Duplicates (`["foo", "foo"]`):** Structurally permitted in storage; `RetireDebt` removes all instances atomically via `array_remove`.
* **Retiring Non-Existent Debt:** Treated as an idempotent no-op (preserves current return semantics).
* **Grandfathered State:** Existing database rows retain their historical debt arrays without mutation or re-interpretation.

---

**Final Directive for the Coding Agent**

```text
Do not replace the removed six-item Solvent vocabulary with any new universal or default vocabulary. Solvent's database default is an empty array. Domain adapters own their debt vocabularies and must explicitly attach their starting debt when creating beliefs. For this repository, consolidate and preserve the wizard's six deployment-review identifiers inside the wizard/domain layer, ensuring DebtMapping consumes that same domain-owned vocabulary rather than kernel.FullDebt. For the research Coordinator, define and attach ebpInitialDebt explicitly during packet compilation. Solvent must never validate, normalize, enumerate, or interpret debt identifiers.

```


## Z

Disposition verification, then the residual findings. This consolidation is dispositionally sound — the accept/reject table correctly separates the recurring coupling risk (universal vocabulary) from the legitimate structural findings, and the Gemini rejections are all correct: length caps, trimming, case normalization, and required-field enforcement would each re-import domain assumptions into a layer the doctrine explicitly insulates. The H2-not-API-required correction is exactly right: omission→empty is the correct Solvent contract, and the safety property migrates to *adapter obligation* — mandatory `ebpInitialDebt` at coordinator translation, not mandatory field at API. The single-source-of-truth move for the wizard vocabulary (domain layer owning both `wizardDebt` and `DebtMapping`) closes the two-copy drift properly. The degenerate-case semantics proposed (reject empty-string at application boundary; array_remove semantics for duplicates; idempotent no-op for missing retirement) match the current implementation's actual behavior — no invented semantics, which is the right call. The floor-as-guidance-not-code disposition is the correct reading of my proposal, and "the different domains repeatedly encounter similar *questions*" preserves its value without recreating the coupling. Net: this is approvable, and the final instruction paragraph is exactly the right shape for a coding-agent prompt.

Four residual items, ordered by when they'll bite:

## 1. The coordinator's `ebpInitialDebt` obligation needs its enforcement mechanism named (this is the one that reopens if skipped)

The consolidation makes coordinator debt-attachment mandatory in prose — but prose was exactly what failed for seven rounds (role cards, "never fabricate," advisor classification). The mandatory translation rule needs to be *checkable*: **validation belongs in the coordinator's packet validator, not the agent**. Specifically: a work-agent packet may *propose* debts, but the coordinator's compile step MUST union the EBP initial set into every belief it creates — and `ebpInitialDebt` missing from a compiled belief is a coordinator defect, testable with one negative test (packet with no debts in → belief with EBP six out). One test makes the obligation mechanical; without it, the first rushed adapter implementation silently drops it, and you've rebuilt the fail-open posture the consolidation explicitly documents as intentional-but-compensated. The compensation must be enforced somewhere, and the coordinator validator is that somewhere.

## 2. `mapping.go`'s disposition needs one verification step before merge

The consolidation says "if `mapping.go` currently derives from `kernel.FullDebt`, move the definition to the wizard/domain layer" — conditional. Before the coding agent touches anything, the conditional resolves by inspection (M4's grep answers it in thirty seconds), but the plan should state the expected outcome for both branches: if shared → single-source move as described; if independent → *verify equivalence* (the two copies match today) and record that they're now intentionally domain-owned. Either way the decision record gets one sentence. Unresolved conditionals in kernel-touching plans are how silent drift gets its start.

## 3. The re-freeze sequence has one ordering subtlety worth pinning

The acceptance chain ends "new commit → new frozen hash → Test H / baseline references updated." Pin the intermediate: **tag the re-freeze commit itself** (e.g., the freeze commit is its own annotated tag, as `7602699` was), so the baseline is a *named* object rather than a floating HEAD reference — and update the freeze-baseline references in one commit that *also* notes the supersession in its message. Otherwise the next "what's frozen?" archaeology session finds a hash with no context. Thirty seconds of discipline, permanent payoff.

## 4. The decision record's doctrine citation should be the headline, not a footnote

When the record is written, lead with: *this change restores documented doctrine (writeup §4.3: "debt opaque vocabulary... belongs to the application, policy, or deployment") rather than expanding kernel scope — the implementation had drifted from the design.* That framing matters beyond this change: it establishes the precedent that kernel changes are judged against the kernel's own frozen principles, and it's the cleanest possible demonstration that the freeze discipline has teeth in both directions — it blocks unjustified growth *and* licenses justified contraction. This is the first kernel amendment in the project's history; its record is the template for the next one.

With those four folded in — coordinator-side enforcement test, mapping.go branch resolution, freeze-tag sequencing, doctrine-first record — Plan 11 as consolidated is ready for the coding agent. The vocabulary question that started this thread is now fully settled at every layer: storage and structural enforcement in the kernel, meaning and validation in the domain, guidance (not law) in the floor document, and the whole thing documented by a record that shows the governance system working exactly as designed — including on the kernel itself.


## claude

This is a strong synthesis — the through-line that ties nearly every rejection together ("this would just move the hardcoded coupling to a different location") is the correct test to apply, and I want to be specific about where it's applied well versus where I think it needs one more turn of the screw.

**The `mapping.go` disposition is right, but "if it currently derives from `kernel.FullDebt`" is still conditional, and this is the one item that was marked blocking.** The final instruction correctly resolves the *design* question — single source of truth in the wizard/domain layer, `DebtMapping` consumes it, `kernel.FullDebt` disappears entirely — but it still hasn't been confirmed which of the two actual states the file is currently in. That's not a synthesis problem, it's just the one open empirical fact nobody's checked yet, and since this table marked it "ACCEPT — blocking" earlier, I'd want it actually looked at (`grep -n "FullDebt\|kernel\." internal/wizard/mapping.go` or wherever it lives) before the coding agent starts, not discovered mid-implementation.

**The H2 reframing is the sharpest correction in this document, and it's correct for a reason worth stating explicitly: it relocates the failure to the layer that can actually be held accountable for it.** Making the Solvent API field required would have caught a missing debt list with a generic 400 error that says nothing about *why* debt matters for this belief. Making Coordinator translation mandatory means a missing EBP debt list fails inside the one component that actually knows what "this research claim needs a provenance check" means — the error, if any, comes from the layer with the context to make it meaningful. That's the same logic as pushing the typo-check for `wizardDebt` down to the wizard rather than the kernel in my last turn, applied one level higher in the stack. Worth stating as the reason, not just the rule, because "Coordinator must supply initial debt" is going to look like an arbitrary extra step to whoever implements it unless the reasoning travels with the instruction.

**One place I think the taxonomy discussion overcorrected relative to what I actually proposed, and it's worth restoring one piece.** The final instruction to the coding agent doesn't mention the functional-taxonomy guidance at all — it says to preserve the wizard's six names and define `ebpInitialDebt`, but the cross-domain table (provenance / consistency / impact / reproducibility / reversibility / human authority) that made the case for *why* those aren't arbitrary seems to have been dropped from what actually ships, even though the document's own "Universal floor" section explicitly keeps it as guidance. If that guidance is going in Conductor's docs as planned, I'd make sure it's not lost between this synthesis and the actual doc that gets written — the value of that table wasn't the six words, it was showing three unrelated domains independently landing on the same six *questions* with completely different literal answers. That's the evidence for "this is a real pattern, not a coincidence," and it's worth carrying forward explicitly rather than trusting it survives by reference.

**The degenerate-case semantics are all correct, and the reasoning for retire-on-nonexistent is worth double-checking against one thing:** "idempotent no-op, preserving current semantics" — is this actually consistent with how `array_remove` behaves when the debt array legitimately doesn't contain the target string versus when the *belief itself* doesn't exist? Those need to stay distinguishable (a `RetireDebt` call against a nonexistent belief should still error; only a nonexistent *item within an existing belief's debt array* should no-op). I'd assume this is already how it works given "preserving current implementation" is the stated intent, but it's cheap to make explicit as a fourth degenerate-case test alongside the three already listed, given how central "identify the belief unambiguously before mutating" has been to every other fix in this thread.

**Everything else is right as written, and I'd call out the empty-string decision specifically as well-placed:** rejecting `""` at the API/application boundary rather than the kernel is exactly consistent with "identity belongs to the owning application" — an empty string isn't a vocabulary question, it's a malformed-input question, and malformed input gets caught at the edge regardless of what domain owns the vocabulary behind it. That's a clean distinction the Gemini review's rejected suggestions (max length, case normalization) blurred by treating all string hygiene as one category; this document correctly separates "structural validity of an identifier" from "domain meaning of an identifier," and only the former belongs anywhere near Solvent.

**Net:** confirm the actual state of `mapping.go` before implementation starts, make sure the functional-taxonomy table survives into whatever doc actually ships (not just this synthesis), add the fourth degenerate case distinguishing missing-belief from missing-item, and this is ready to hand to the coding agent as written.