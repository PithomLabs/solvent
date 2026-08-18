/-
  Solvent: A Transactional Belief Ledger for Autonomous Agents
  Formal Model in Lean 4 — Types

  This module defines the abstract types modeling the core entities of Solvent:
  beliefs, action intents, edges, debt, and the ledger state.
-/

namespace Solvent

/-- Status of a belief in the ledger. Corresponds to `belief.status CHECK (...)`. -/
inductive BeliefStatus
  | entered
  | promoted
  | retracted
  deriving DecidableEq, Repr

/-- State of an action intent. Corresponds to `action_intent.state CHECK (...)`. -/
inductive IntentState
  | live
  | cancelled
  | executed
  deriving DecidableEq, Repr

/-- An un-discharged debt item blocking belief promotion. Corresponds to `belief.debt`. -/
structure DebtItem where
  id : Nat
  deriving DecidableEq, Repr

abbrev BeliefId := Nat
abbrev IntentId := Nat

/-- Kinds of edges in the belief graph. Corresponds to `belief_edge.kind`. -/
inductive EdgeKind
  | derives
  | contradicts
  deriving DecidableEq, Repr

/-- A belief entity. Corresponds to the `belief` table. -/
structure Belief where
  status : BeliefStatus
  debt : List DebtItem
  finalTruth : Bool
  deriving DecidableEq, Repr

/-- An action intent entity. Corresponds to the `action_intent` table. -/
structure ActionIntent where
  state : IntentState
  beliefId : BeliefId
  beliefStatus : BeliefStatus
  deriving DecidableEq, Repr

/-- The complete ledger state. -/
structure Ledger where
  beliefs : BeliefId → Option Belief
  intents : IntentId → Option ActionIntent
  edges : List (BeliefId × BeliefId × EdgeKind)

end Solvent
