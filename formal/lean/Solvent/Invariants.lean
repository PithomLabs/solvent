/-
  Solvent: A Transactional Belief Ledger for Autonomous Agents
  Formal Model in Lean 4 — Invariants

  This module defines the three database-enforced authority invariants
  and the composite validity predicate `ValidLedger`.

  These correspond directly to SQL constraints:
  - `promoted_is_debt_free`  CHECK (status != 'promoted' OR (debt = '{}' AND NOT final_truth))
  - `gate`                   FOREIGN KEY (belief_id, belief_status)
                               REFERENCES belief(id, status) ON UPDATE CASCADE
  - `live_requires_promoted` CHECK (state != 'live' OR belief_status = 'promoted')
-/

import Solvent.Types

namespace Solvent

/--
  Invariant 1: A promoted belief must be debt-free and not marked as final truth.
  Corresponds to SQL constraint `promoted_is_debt_free`.
-/
def promoted_is_debt_free (l : Ledger) : Prop :=
  ∀ (bid : BeliefId) (b : Belief),
    l.beliefs bid = some b →
    b.status = BeliefStatus.promoted →
    b.debt = [] ∧ b.finalTruth = false

/--
  Invariant 2: For every existing intent whose referenced belief exists in the ledger,
  the intent's cached `beliefStatus` matches the belief's current actual `status`.
  Corresponds to the composite foreign key with `ON UPDATE CASCADE` (`gate`).
-/
def gate (l : Ledger) : Prop :=
  ∀ (iid : IntentId) (intent : ActionIntent) (b : Belief),
    l.intents iid = some intent →
    l.beliefs intent.beliefId = some b →
    intent.beliefStatus = b.status

/--
  Invariant 3: Every live intent must reference a belief whose current status
  is `promoted`, and whose cached `beliefStatus` is also `promoted`.
  Corresponds to SQL constraint `live_requires_promoted`.
-/
def live_requires_promoted (l : Ledger) : Prop :=
  ∀ (iid : IntentId) (intent : ActionIntent),
    l.intents iid = some intent →
    intent.state = IntentState.live →
    intent.beliefStatus = BeliefStatus.promoted ∧
    ∃ (b : Belief), l.beliefs intent.beliefId = some b ∧ b.status = BeliefStatus.promoted

/--
  Composite validity of the ledger state: all three invariants hold simultaneously.
-/
def ValidLedger (l : Ledger) : Prop :=
  promoted_is_debt_free l ∧ gate l ∧ live_requires_promoted l

end Solvent
