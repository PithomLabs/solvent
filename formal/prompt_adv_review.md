Use this prompt. It tells the coding agent to be **adversarial rather than congratulatory**, and specifically tests whether the Lean model proves anything meaningful or merely proves properties of an oversimplified model.

```text
Perform an ADVERSARIAL REVIEW of the newly implemented Lean 4 formal model
under:

    formal/lean/

This is a review ONLY.

DO NOT modify any files.
DO NOT commit anything.
DO NOT rewrite proofs.
DO NOT add theorems.
DO NOT "fix" findings.

Your job is to try to BREAK the formalization and determine whether its
claims are actually meaningful, sound, and faithful to Solvent.

The formal model is intended to provide a machine-checked proof of the
ABSTRACT Solvent belief/authority state machine.

IMPORTANT CLAIM BOUNDARY:

Lean proves:
    abstract state-machine preservation properties

CockroachDB empirically proves:
    real-engine refusal behavior

The Lean model is NOT claimed to be:
    - a refinement proof of Go
    - a formal verification of CockroachDB
    - a proof that retrieval is correct
    - a proof that Solvent makes an agent correct

Review specifically against that boundary.

==================================================
1. READ THE IMPLEMENTATION
==================================================

Read all of:

    formal/lean/lean-toolchain
    formal/lean/lakefile.lean
    formal/lean/README.md
    formal/lean/Solvent.lean
    formal/lean/Solvent/Types.lean
    formal/lean/Solvent/Invariants.lean
    formal/lean/Solvent/Transitions.lean
    formal/lean/Solvent/Preservation.lean
    formal/lean/Solvent/Examples.lean

Also inspect the production counterparts:

    db/001_schema.sql
    internal/kernel/
    docs/M0_REPORT.md
    docs/M2_TRANSCRIPT.md
    AGENTS.md
    README.md

Do not review Lean in isolation.

==================================================
2. MOST IMPORTANT QUESTION:
IS THE FORMAL MODEL ACTUALLY NON-TAUTOLOGICAL?
==================================================

Try to find any way the proofs succeed because:
- transitions secretly enforce their own postconditions;
- invariants are stronger/weaker than the production semantics in a way
  that makes preservation trivial;
- the model excludes the states or transitions that make the theorem
  interesting;
- Option-returning transitions refuse exactly the states that would break
  the theorem;
- helper functions silently discard problematic states.

Inspect every transition:

    promote
    cancelIntent
    authorizeIntent
    retractCascade

Verify that refusal happens only because of genuine domain preconditions,
NOT because `ValidLedger` is consulted directly or indirectly.

Look for hidden forms of the same issue:
- candidate filtering
- helper predicates equivalent to `ValidLedger`
- impossible branches
- constructors that cannot represent invalid states
- transformations that silently normalize invalid state

If you find any such issue, classify it as HIGH severity.

==================================================
3. STATE-SPACE AUDIT
==================================================

Ask:

    Can the Lean model represent every important state relevant to the
    theorem?

Try constructing adversarial states such as:

- promoted belief with debt
- promoted belief with finalTruth = true
- live intent on entered belief
- live intent on retracted belief
- cancelled intent on entered belief
- executed intent on retracted belief
- intent referencing missing belief
- intent whose beliefStatus is stale
- multiple intents referencing the same belief
- multiple beliefs with arbitrary IDs
- unrelated beliefs plus unrelated intents
- dependency chains with multiple levels
- contradictory edges
- duplicate edges
- cycles in derives edges
- descendant sets containing duplicate IDs

Determine whether the model permits these states and whether the invariants
and transitions behave as intended.

Pay special attention to whether `gate` and `live_requires_promoted`
actually capture the same semantic boundary as the SQL schema.

==================================================
4. GATE SEMANTICS
==================================================

Compare Lean `gate` with the production composite FK:

    FOREIGN KEY (belief_id, belief_status)
      REFERENCES belief(id, status)
      ON UPDATE CASCADE

Verify that Lean models BOTH:

1. the relationship between an intent and its referenced belief;
2. the mirrored status that is propagated by the cascade.

Try to find a mismatch involving:
- missing beliefs
- stale intent.beliefStatus
- promotion
- retraction
- cancellation
- executed intents

Ask whether the Lean model permits a state that the SQL FK would reject,
or rejects one the SQL would permit.

==================================================
5. PROMOTION SEMANTICS
==================================================

Audit:

    promote
    promotion_updates_dependent_intent
    promote_preserves_validity

Verify that promotion:
- changes the belief status;
- propagates status to dependent intents;
- preserves gate;
- preserves live_requires_promoted;
- preserves promoted_is_debt_free.

Look for a subtle mismatch where dependent intents are updated in Lean
but the actual Go/SQL path behaves differently.

==================================================
6. RETRACTION / CASCADE SEMANTICS
==================================================

This is the most important part.

Audit:

    retractCascade
    cascade_retraction_updates_dependent_intent
    cascade_retraction_cannot_leave_live_intent
    cascade_update_preserves_gate
    cascade_retraction_blocks_live_intent
    retractCascade_preserves_validity

Verify the exact ordering:

    cancel live intents
        ↓
    update mirrored intent status
        ↓
    retract beliefs

Try adversarial cases involving:
- one dependent belief
- multiple descendants
- multiple intents
- multiple intents on the same belief
- cancelled intents
- executed intents
- unrelated intents
- unrelated beliefs
- multiple descendant levels
- shared descendants
- cycles
- duplicate edges

Determine whether the simple recursive descendant implementation is
semantically sufficient for the theorem actually claimed.

IMPORTANT:
The Lean model intentionally does NOT reproduce the exact SQL recursive
CTE/cycle-deduplication implementation.

Therefore determine whether the abstraction is still sound for the
theorems stated.

If the theorem silently assumes an acyclic graph while the type allows
cycles, flag that.

==================================================
7. SEARCH FOR VACUOUS / TRIVIAL THEOREMS
==================================================

For every major theorem, ask:

    Could this theorem be true only because the model cannot represent
    the interesting counterexample?

Audit at least:

    promote_preserves_validity
    cancelIntent_preserves_validity
    authorizeIntent_preserves_validity
    retractCascade_preserves_validity
    live_intent_implies_promoted
    no_live_intent_on_retracted_belief
    promotion_updates_dependent_intent
    cascade_retraction_updates_dependent_intent
    cascade_retraction_cannot_leave_live_intent
    cascade_update_preserves_gate
    cascade_retraction_blocks_live_intent

Identify anything that is technically true but materially weaker than the
project's README/DevPost language.

==================================================
8. REPRESENTATION / MODEL FIDELITY
==================================================

Audit these choices:

    beliefs : BeliefId → Option Belief
    intents : IntentId → Option ActionIntent
    edges   : List (...)

Ask whether:
- duplicate IDs are impossible or merely represented by function lookup;
- edge duplicates affect semantics;
- `DebtItem` is expressive enough;
- `Belief.finalTruth : Bool` correctly models the production meaning;
- `BeliefStatus` and `IntentState` match the actual SQL states;
- the Lean model accidentally permits states SQL CHECK constraints prohibit;
- the Lean model accidentally excludes states SQL permits.

Compare each type directly against the schema.

==================================================
9. PRODUCTION CORRESPONDENCE
==================================================

Compare every Lean transition against the real Go kernel operation.

Build a table:

    Lean transition
    Go operation
    SQL operation(s)
    semantic correspondence
    mismatch

Pay particular attention to:

    promote
    authorizeIntent
    cancelIntent
    retractCascade

Do NOT demand implementation identity.

The question is whether the abstraction being proved is the abstraction
the production system claims to implement.

Flag every meaningful semantic gap.

==================================================
10. COCKROACHDB CLAIM BOUNDARY
==================================================

Verify that the Lean documentation does NOT accidentally claim:

    "Lean proves CockroachDB is correct"

or:

    "Lean formally verifies the Go implementation"

or:

    "Lean proves the database refusal behavior"

The correct division is:

    Lean:
    abstract safe-transition preservation

    CockroachDB:
    empirical proof that unsafe ordering is refused

Flag any wording that exceeds that.

==================================================
11. AXIOM / TRUST AUDIT
==================================================

Verify:

    no `sorry`
    no `admit`
    no unchecked `axiom`

But also inspect imports and theorem dependencies.

Look for:
- imported axioms
- noncomputable shortcuts
- unsafe declarations
- assumptions hidden in helper lemmas
- theorem statements whose premises already encode the conclusion

Do not merely grep for strings.

Determine whether the final proof actually rests only on the intended Lean
logic + Mathlib.

==================================================
12. TEST THE FORMAL MODEL ADVERSARIALLY
==================================================

Where practical, add temporary LOCAL test snippets only in /tmp or another
temporary location, not inside the repository.

Try to:
- construct counterexample ledgers;
- evaluate transition functions;
- derive contradictions from purportedly invalid states;
- test edge cases around Function.update;
- test descendant computation on cycles/duplicates;
- test stale beliefStatus states;
- test multiple intents per belief.

Do not modify repository files.

==================================================
13. COMPARE AGAINST THE PROJECT'S CLAIMS
==================================================

Read the current README and Engineering Guide.

For each formalization claim, label:

    EXACTLY SUPPORTED
    SUPPORTED BUT NARROWER
    OVERSTATED
    UNSUPPORTED

Pay special attention to phrases such as:
- "formal verification"
- "machine-checked proof"
- "authority cannot outlive its belief"
- "database-enforced"
- "safe transition"
- "general state machine"

==================================================
14. FINAL REPORT
==================================================

Return ONLY:

# VERDICT

GREEN
GREEN WITH NITS
RED

# EXECUTIVE ASSESSMENT

2–5 paragraphs.

Be adversarial.
Do not praise the formalization merely because it compiles.

# CRITICAL FINDINGS

For each finding:

    Severity: CRITICAL / HIGH / MEDIUM / LOW
    Finding:
    Evidence:
    Why it matters:
    Recommended action:

Only include real findings.

# TAUTOLOGY / VACUITY AUDIT

For each major theorem:
    meaningful / potentially vacuous / vacuous

Brief reason.

# MODEL-FIDELITY AUDIT

Table:

| Area | Lean model | Production | Assessment |

Cover:
- states
- invariants
- promotion
- authorization
- cancellation
- retraction
- cascade
- graph traversal
- status propagation

# THEOREM AUDIT

| Theorem | Sound? | Non-vacuous? | Faithful? | Notes |

# TRUST / AXIOM AUDIT

State whether:
- sorry = 0
- admit = 0
- axiom = 0
- hidden assumptions exist

# CLAIM AUDIT

List any README/Engineering Guide claim that is stronger than what the
Lean model actually proves.

# RECOMMENDATION

Choose exactly one:

    SHIP AS-IS
    SHIP WITH NITS
    REWORK

If NITS or REWORK, identify the minimum changes needed.

IMPORTANT:
This is an adversarial review.
Do not rewrite anything.
Do not modify files.
Do not commit.
Do not create a "better" version.
The goal is to determine whether the current formalization deserves to be
presented to technical hackathon judges as machine-checked evidence.
```
