/-
  Solvent: A Transactional Belief Ledger for Autonomous Agents
  Formal Model in Lean 4 — Transitions

  This module defines the four core state-machine transitions of Solvent:
  - `promote`: Promote an entered belief to promoted (with FK cascade to dependent intents).
  - `cancelIntent`: Cancel a live intent.
  - `authorizeIntent`: Authorize a live intent on a currently promoted belief.
  - `retractCascade`: Transitively retract a belief and its derives-descendants,
                     cancelling live intents and cascading retracted status.

  IMPORTANT: Transitions refuse (return `none`) SOLELY on domain preconditions.
  They do NOT check `ValidLedger` on their candidate output.
-/

import Solvent.Types
import Solvent.Invariants

namespace Solvent

/--
  Compute the direct `derives` children of a given belief ID from the edge list.
-/
def derivesChildren (edges : List (BeliefId × BeliefId × EdgeKind)) (b : BeliefId) : List BeliefId :=
  edges.filterMap (fun (src, dst, k) =>
    if src == b && k == EdgeKind.derives then some dst else none)

/--
  Compute the reachable `derives` descendants with bounded recursion.
-/
def reachableDerives (edges : List (BeliefId × BeliefId × EdgeKind))
                     (fuel : Nat) (visited : List BeliefId) (queue : List BeliefId) : List BeliefId :=
  match fuel, queue with
  | 0, _ => visited
  | _, [] => visited
  | fuel + 1, q :: qs =>
    if q ∈ visited then
      reachableDerives edges fuel visited qs
    else
      let children := derivesChildren edges q
      reachableDerives edges fuel (q :: visited) (children ++ qs)

/--
  Set of beliefs affected by cascading retraction from `root` (root + derives descendants).
-/
def affectedBeliefs (l : Ledger) (root : BeliefId) : List BeliefId :=
  reachableDerives l.edges (l.edges.length + 1) [] [root]

/--
  Transition 1: `promote`
  Promote an `entered` belief to `promoted` if it has no debt and is not marked as finalTruth.
  Models the composite FK `ON UPDATE CASCADE` by updating `beliefStatus := promoted`
  for all dependent intents referencing `bid`.
-/
def promote (l : Ledger) (bid : BeliefId) : Option Ledger :=
  match l.beliefs bid with
  | some b =>
    if b.status == BeliefStatus.entered && b.debt == [] && !b.finalTruth then
      let newBeliefs : BeliefId → Option Belief :=
        fun id =>
          if id == bid then
            some { b with status := BeliefStatus.promoted }
          else
            l.beliefs id
      let newIntents : IntentId → Option ActionIntent :=
        fun iid =>
          match l.intents iid with
          | some intent =>
            if intent.beliefId == bid then
              some { intent with beliefStatus := BeliefStatus.promoted }
            else
              some intent
          | none => none
      some { l with beliefs := newBeliefs, intents := newIntents }
    else
      none
  | none => none

/--
  Transition 2: `cancelIntent`
  Cancel an existing live intent.
-/
def cancelIntent (l : Ledger) (iid : IntentId) : Option Ledger :=
  match l.intents iid with
  | some intent =>
    if intent.state == IntentState.live then
      let newIntents : IntentId → Option ActionIntent :=
        fun id =>
          if id == iid then
            some { intent with state := IntentState.cancelled }
          else
            l.intents id
      some { l with intents := newIntents }
    else
      none
  | none => none

/--
  Transition 3: `authorizeIntent`
  Authorize a live intent on a currently promoted belief.
-/
def authorizeIntent (l : Ledger) (iid : IntentId) : Option Ledger :=
  match l.intents iid with
  | some intent =>
    match l.beliefs intent.beliefId with
    | some b =>
      if b.status == BeliefStatus.promoted then
        let newIntents : IntentId → Option ActionIntent :=
          fun id =>
            if id == iid then
              some { intent with state := IntentState.live, beliefStatus := BeliefStatus.promoted }
            else
              l.intents id
        some { l with intents := newIntents }
      else
        none
    | none => none
  | none => none

/--
  Transition 4: `retractCascade`
  Transitively retracts a belief and its derives descendants:
  1. Identifies affected beliefs (`root` + transitive `derives` children).
  2. Cancels all live intents referencing affected beliefs and updates their `beliefStatus := retracted`.
  3. Updates all affected beliefs to `status := retracted`.
  Safe execution ordering (cancelling live intents before retracting) ensures invariants are preserved.
-/
def retractCascade (l : Ledger) (bid : BeliefId) : Option Ledger :=
  match l.beliefs bid with
  | some _ =>
    let aff := affectedBeliefs l bid
    let newBeliefs : BeliefId → Option Belief :=
      fun id =>
        match l.beliefs id with
        | some b =>
          if id ∈ aff then
            some { b with status := BeliefStatus.retracted }
          else
            some b
        | none => none
    let newIntents : IntentId → Option ActionIntent :=
      fun iid =>
        match l.intents iid with
        | some intent =>
          if intent.beliefId ∈ aff then
            let newState :=
              if intent.state == IntentState.live then
                IntentState.cancelled
              else
                intent.state
            some { intent with state := newState, beliefStatus := BeliefStatus.retracted }
          else
            some intent
        | none => none
    some { l with beliefs := newBeliefs, intents := newIntents }
  | none => none

end Solvent
