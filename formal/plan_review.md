I reviewed the plan. **I would approve it with one important rework to the proof design before implementation.**

The overall scope is excellent: isolated `formal/lean/`, Mathlib, schema vocabulary, function-based ledger, `retractCascade` only, simple recursive descent, and explicit preservation/cascade theorems. 

### The important problem

This line is the one I would change:

> `retractCascade` ... **Return `some l'` only when the resulting ledger is valid**. 

That risks making the headline preservation theorem nearly **tautological**.

You would effectively have:

```lean
retractCascade l bid =
  if ValidLedger candidate then some candidate else none
```

and then prove:

```lean
ValidLedger l →
retractCascade l bid = some l' →
ValidLedger l'
```

Of course that is true, because the function itself checked `ValidLedger`.

That is not the formal proof we actually want.

### What I want instead

The transition should refuse based on the **actual domain preconditions**, not by asking whether its output satisfies the theorem we're trying to prove.

For example:

```lean
retractCascade :
  Ledger → BeliefId → Option Ledger
```

should:

1. find the target and descendants;
2. retract the beliefs;
3. cancel affected live intents;
4. update their `beliefStatus`;
5. return `some` when the operation's **semantic preconditions** are satisfied.

Then Lean proves independently:

```lean
theorem retractCascade_preserves_validity :
  ValidLedger l →
  retractCascade l bid = some l' →
  ValidLedger l'
```

Now the theorem is genuinely meaningful.

The same principle applies to `promote`, `authorizeIntent`, and `cancelIntent`.

---

### Second nit: `cascade_update_preserves_gate`

The current theorem is:

```lean
theorem cascade_update_preserves_gate :
  ValidLedger l →
  cascadeStatusUpdate l bid newStatus = some l' →
  gate l'
```



That's useful, but I'd make the **local cascade behavior** explicit before proving the global invariant.

Something like:

```lean
theorem cascade_retraction_updates_dependent_intent :
  ...
  → intent.beliefStatus = .retracted
```

followed by:

```lean
theorem cascade_retraction_cannot_leave_live_intent :
  ...
  → intent.state ≠ .live
```

Then `gate` and `live_requires_promoted` become consequences.

That gives you a much clearer mathematical narrative:

```text
status changes
      ↓
FK-style status propagation
      ↓
live intent becomes incompatible
      ↓
authority invariant forces refusal
```

That mirrors the actual D3/D4 experiment much better.

---

### Third nit: toolchain pinning

The plan says:

```text
Lean 4.33.0 already installed
```

but then proposes resolving the exact compatible version from **Mathlib main**.  

For a GitHub submission, don't pin against a moving `main`.

Have the agent determine the exact Mathlib revision/version compatible with the installed Lean toolchain, pin both, and make `lake build` reproducible.

The goal should be:

```text
repo clone
    ↓
elan toolchain install
    ↓
lake build
    ↓
same proof
```

not "whatever Mathlib main happens to accept today."

---

### Fourth nit: the theorem set is excellent

I especially like these two:

```lean
live_intent_implies_promoted
no_live_intent_on_retracted_belief
```

because they turn the Solvent thesis into something mathematically legible:

> A live action cannot exist without the currently promoted belief underneath it.

The plan correctly derives the second from `live_requires_promoted` rather than pretending it is a separate primitive invariant. 

That's exactly the kind of thing a technically sophisticated judge can appreciate.

---

### Final recommendation to the coding agent

Give it this before approving:

```text
Proceed, with one proof-design correction:

DO NOT make transitions return `none` merely because their candidate
output fails `ValidLedger`.

The preservation theorems must be non-tautological.

Each transition should refuse only according to its semantic/domain
preconditions, and Lean must independently prove:

    ValidLedger l →
    transition l x = some l' →
    ValidLedger l'

In particular, retractCascade must not contain a final
`if ValidLedger candidate then some candidate else none` gate.

For the cascade proof, explicitly model:
1. belief status changes;
2. affected intent.beliefStatus follows the change;
3. a live intent on a retracted belief is impossible.

Then prove the global gate/live invariants from those properties.

Also pin an exact Lean + Mathlib-compatible toolchain revision rather
than depending on a moving Mathlib main branch.

Everything else in the approved plan remains unchanged.
```

**Verdict: GREEN WITH THIS NIT.** The formalization is worth adding. The repository already has empirical CockroachDB evidence; adding a non-tautological Lean proof gives you a second, independent form of evidence for the core state-machine semantics. The current plan is close, but I would not let the agent implement the `ValidLedger`-as-a-postcondition shortcut.
