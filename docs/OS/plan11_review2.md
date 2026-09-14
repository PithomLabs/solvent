I reviewed the three assessments against the architecture we have already established. There is substantial agreement, but several recommendations should be rejected because they would simply recreate the coupling we are removing.

The consolidated decision is below.

## Final disposition

| Finding                                                        | Decision                           | Rationale                                                                                                                                                                                      |
| -------------------------------------------------------------- | ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| H1 — decision record + re-freeze                               | **ACCEPT**                         | A kernel change must end with a new frozen commit/baseline.                                                                                                                                    |
| H2 — EBP coordinator must attach initial debt                  | **ACCEPT — important**             | Once Solvent stops supplying vocabulary, the application layer must explicitly supply EBP debt. Otherwise the research workflow can accidentally create immediately promotable beliefs.        |
| `mapping.go` relationship to vocabulary                        | **ACCEPT — blocking**              | Must establish whether it shares `kernel.FullDebt` or independently owns the wizard vocabulary. Do not leave two silently divergent copies.                                                    |
| M1 — find/invert old negative-validation tests                 | **ACCEPT**                         | Removing vocabulary validation changes the contract; obsolete rejection tests must be deliberately replaced.                                                                                   |
| M2 — trace `001` vs later migration                            | **ACCEPT**                         | Do not modify historical schema merely for appearance. Determine the actual bootstrap path and make the additive migration authoritative.                                                      |
| M3 — empty strings / duplicates / missing retirement semantics | **ACCEPT**                         | These are legitimate contract edges that become more important once vocabulary validation disappears.                                                                                          |
| M4 — repository-wide literal sweep                             | **ACCEPT**                         | Cheap, high-value verification.                                                                                                                                                                |
| M5 — explicit grandfathering decision                          | **ACCEPT**                         | Existing beliefs must retain their historical debt; changing the vocabulary must not implicitly mutate old state.                                                                              |
| MCP schema/discoverability                                     | **ACCEPT, small**                  | Tool descriptions should explain how callers discover the actual debt values.                                                                                                                  |
| Auto-discovery of migrations in tests                          | **ACCEPT as follow-up**            | Valid architectural cleanup, but not part of this kernel fix.                                                                                                                                  |
| Gemini: max 64 chars / max 16 debts                            | **REJECT**                         | Arbitrary kernel constraints are unnecessary and create new domain assumptions.                                                                                                                |
| Gemini: trim/lowercase debt IDs in Solvent                     | **REJECT**                         | Solvent must not normalize opaque identifiers. Identity belongs to the owning application.                                                                                                     |
| Gemini: require `debt` explicitly in API                       | **REJECT**                         | Explicit `[]` versus omitted field does not restore the safety property; the database default is intentionally empty. The domain adapter is responsible for attaching its starting vocabulary. |
| Gemini: "fail-open vulnerability"                              | **PARTIALLY ACCEPT**               | The safety posture does change, intentionally. That must be documented and compensated for at the application boundary, not reintroduced into Solvent.                                         |
| "Universal default debt vocabulary"                            | **REJECT**                         | This would merely move the hardcoded vocabulary problem upward.                                                                                                                                |
| Universal epistemic **floor/taxonomy**                         | **ACCEPT as guidance, NOT code**   | Useful architectural guidance, but it must not become a Solvent-enforced vocabulary.                                                                                                           |
| Shared `ebp:*` strings as defaults                             | **REJECT**                         | Same coupling problem in a nicer namespace.                                                                                                                                                    |
| Optional namespace convention                                  | **ACCEPT as application guidance** | Useful for domain-owned identifiers, but not a Solvent requirement.                                                                                                                            |

The key conclusion from the reviews is exactly this:

> **Do not replace the six hardcoded Solvent defaults with another six "generic" defaults.**

The correct abstraction is **domain-owned vocabulary + optional conceptual guidance**, not a new universal kernel vocabulary. This aligns with the strongest portions of the reviews: debt semantics belong to the application/domain layer, while Solvent only enforces the structural property that promotion requires zero outstanding debt. 

## One important correction to our earlier plan

We should keep the API `debt` field **optional**, as previously decided.

Omitted:

```json
{
  "claim": "...",
  "claim_type": "derived"
}
```

means:

```text
debt = []
```

Explicit:

```json
"debt": ["foo", "bar"]
```

means exactly those opaque debt identifiers.

The important distinction is:

```text
Solvent API:
    omission -> no starting debt

Research Coordinator:
    MUST supply EBP initial debt when creating an EBP belief

Wizard:
    MUST supply wizard debt when creating a wizard belief

Other applications:
    define their own policy
```

Thus H2 is not an argument for making the Solvent API field required. It is an argument for making **Coordinator translation mandatory**. The review correctly identifies that otherwise the research workflow could silently become debt-free at ingestion. 

## The wizard vocabulary

Keep the wizard's existing six names.

But consolidate their ownership.

The desirable structure is:

```text
internal/wizard/domain vocabulary
        |
        +---- wizardDebt
        |
        +---- DebtMapping
        |
        v
     Solvent
   []string opaque
```

`kernel.FullDebt` should disappear as a Solvent concept.

If `mapping.go` currently derives its vocabulary from `kernel.FullDebt`, move that shared six-item definition into the **wizard/domain layer** and have both wizard creation and `DebtMapping` reference it. That eliminates the two-copy drift identified by the review. 

This is one place where Claude's review is particularly useful: the Solvent refactor itself is straightforward, but the application-layer vocabulary still needs a single source of truth.

## The "universal floor"

I would keep this, but strictly as architectural guidance.

The useful abstraction is not:

```text
default debt =
    needProvenance
    needConsistency
    needScope
    ...
```

It is:

```text
When designing a domain's debt vocabulary, consider:

    provenance
    consistency
    scope
    verification
    impact
    recovery

Then decide which actually apply and instantiate them
using domain-specific obligations.
```

That preserves the insight that the different domains repeatedly encounter similar *questions* without pretending they share identical debt semantics. The review's distinction between a universal functional taxonomy and a universal literal vocabulary is sound. 

I would put that guidance **outside Solvent**, alongside the Coordinator/application architecture documentation.

Do not create `ebp:*` constants in Solvent.

## Degenerate cases

Add explicit tests for:

```text
""                  -> decide/document behavior
["foo", "foo"]      -> define/document behavior
retire("missing")   -> preserve existing behavior
```

My preferred semantics are:

```text
empty string:
    reject at application/API boundary as malformed identifier

duplicates:
    allowed structurally, but retirement removes all matching occurrences
    because array_remove already has that behavior

retire nonexistent:
    idempotent no-op, preserving current semantics
```

The first is structural validation, not vocabulary validation. The latter two should remain consistent with the current implementation rather than inventing new behavior.

## Security/input controls

Do **not** add arbitrary:

```text
max 64 chars
max 16 debts
lowercase everything
trim everything
regex vocabulary
```

to Solvent.

Those suggestions are reasonable generic API-hardening ideas, but they are not justified by this architectural change. They would turn Solvent from "opaque debt ledger" into "debt identifier policy engine."

A domain adapter can impose stronger constraints where appropriate.

For example:

```text
EBP:
    needMap
    needInvariant
    ...

Wizard:
    needProvenanceCheck
    ...
```

Each adapter may validate its own vocabulary.

That is the right location for catching:

```text
needProvenanceChck
```

rather than making Solvent recognize what a correct string ought to be. Claude's observation here is valid: the failure should move **to the owning layer**, not disappear. 

## Migration/re-freeze

This becomes part of the acceptance criteria.

The implementation is not complete at:

```text
go test ./...
```

It is complete at:

```text
implementation
→ tests pass
→ migration verified
→ decision record updated
→ new commit
→ new frozen hash
→ Test H / baseline references updated
```

The old `7602699` freeze cannot remain the authoritative baseline after this kernel change. The review is correct on that. 

Also explicitly record:

> Existing beliefs are grandfathered. Their stored debt arrays are not rewritten or reinterpreted by this migration.

That is important because changing the meaning of **new** debt creation must not mutate historical epistemic state.

## Revised implementation boundary

The final architecture should therefore be:

```text
                    DOMAIN / APPLICATION
            ┌──────────────────────────────┐
            │ EBP initial debt             │
            │ Wizard debt                   │
            │ Domain validation             │
            │ DebtMapping                  │
            │ Human/policy obligations     │
            └──────────────┬───────────────┘
                           │
                    []string debt
                           │
                           ▼
                    ┌─────────────┐
                    │   SOLVENT   │
                    │             │
                    │ debt TEXT[]  │
                    │ opaque IDs   │
                    │ empty/nonempty│
                    └──────┬──────┘
                           │
                 promotion gate
                           │
                           ▼
                    authority state
```

This preserves the architecture we established:

```text
CAPABILITY ≠ WORK ≠ AUTHORITY ≠ EXECUTION
```

and more specifically:

```text
DEBT MEANING  → application/domain
DEBT STORAGE  → Solvent
DEBT ENFORCEMENT FOR PROMOTION → Solvent
```

### Final instruction to the coding agent

I would now amend the previous implementation prompt with this decisive paragraph:

> **Do not replace the removed six-item Solvent vocabulary with any new universal/default vocabulary.** Solvent's default debt is empty. Domain adapters own their debt vocabularies and must explicitly attach their starting debt when creating beliefs. For this repository, preserve the wizard's six deployment-review identifiers in the wizard/domain layer and make `DebtMapping` consume that same domain-owned vocabulary rather than `kernel.FullDebt`. For the research Coordinator, define and attach `ebpInitialDebt` explicitly. Solvent must never validate, normalize, enumerate, or interpret either vocabulary.

That is the cleanest net result of all three reviews. It preserves the valid adversarial concerns without allowing them to drag the kernel back toward the exact coupling we are trying to remove.
