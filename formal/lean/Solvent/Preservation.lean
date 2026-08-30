/-
  Solvent: A Transactional Belief Ledger for Autonomous Agents
  Formal Model in Lean 4 — Preservation & Authority Theorems

  This module contains machine-checked proofs that:
  1. The four valid transitions preserve `ValidLedger` (non-tautological preservation).
  2. Authority Impossibility Theorems (Solvent thesis: live authority cannot exist without
     a promoted belief).
  3. Layered cascade theorems connecting local FK status propagation and liveness cancellation
     to global invariant preservation.

  DISTINCTION OF RESPONSIBILITIES:
  - Lean 4 proves that the SAFE state transitions preserve ValidLedger.
  - CockroachDB (D3/D4 experiment) empirically proves that UNSAFE transition orderings
    are structurally refused by engine constraints (23503 gate, 23514 live_requires_promoted).
-/

import Solvent.Types
import Solvent.Invariants
import Solvent.Transitions

namespace Solvent

/- ==============================================================================
   PART 1: AUTHORITY IMPOSSIBILITY THEOREMS (SOLVENT THESIS)
   ============================================================================== -/

/--
  Theorem 1: Any live intent in a valid ledger must reference a belief whose
  current status is `promoted`.
-/
theorem live_intent_implies_promoted (l : Ledger) (iid : IntentId) (intent : ActionIntent) :
    ValidLedger l →
    l.intents iid = some intent →
    intent.state = IntentState.live →
    ∃ (b : Belief), l.beliefs intent.beliefId = some b ∧ b.status = BeliefStatus.promoted := by
  intro hval hintent hlive
  have hlive_req := hval.2.2 iid intent hintent hlive
  exact hlive_req.2

/--
  Theorem 2: Impossibility of live authority on a retracted belief.
  In a valid ledger, no live intent can reference a belief that is retracted.
-/
theorem no_live_intent_on_retracted_belief (l : Ledger) (iid : IntentId) (intent : ActionIntent) (b : Belief) :
    ValidLedger l →
    l.intents iid = some intent →
    intent.state = IntentState.live →
    l.beliefs intent.beliefId = some b →
    b.status = BeliefStatus.retracted →
    False := by
  intro hval hintent hlive hbelief hretracted
  have ⟨b_prom, hb_prom_look, hb_prom_status⟩ := live_intent_implies_promoted l iid intent hval hintent hlive
  rw [hbelief] at hb_prom_look
  cases hb_prom_look
  rw [hretracted] at hb_prom_status
  contradiction

/- ==============================================================================
   PART 2: LOCAL CASCADE & PROMOTION THEOREMS
   ============================================================================== -/

/--
  Theorem 3: Promotion cascades `beliefStatus := promoted` to all dependent intents.
  Models composite FK `ON UPDATE CASCADE`.
-/
theorem promotion_updates_dependent_intent (l : Ledger) (bid : BeliefId) (l' : Ledger)
    (iid : IntentId) (intent' : ActionIntent) :
    ValidLedger l →
    promote l bid = some l' →
    l'.intents iid = some intent' →
    intent'.beliefId = bid →
    intent'.beliefStatus = BeliefStatus.promoted := by
  intro _ hprom hintent' hmatch
  unfold promote at hprom
  split at hprom
  · next b hb =>
    split at hprom
    · next hcond =>
      injection hprom with hl'
      subst hl'
      dsimp only at hintent'
      split at hintent'
      · next intent hintent =>
        split at hintent'
        · next hbid =>
          injection hintent' with hieq
          subst hieq
          rfl
        · next hbid =>
          injection hintent' with hieq
          subst hieq
          have hsame : (intent.beliefId == bid) = true := by
            rw [hmatch]
            exact beq_self_eq_true bid
          rw [hsame] at hbid
          contradiction
      · contradiction
    · contradiction
  · contradiction

/--
  Theorem 4: Cascading retraction updates all dependent intents on affected beliefs
  to have `beliefStatus := retracted`.
-/
theorem cascade_retraction_updates_dependent_intent (l : Ledger) (bid : BeliefId) (l' : Ledger)
    (iid : IntentId) (intent' : ActionIntent) (bid' : BeliefId) :
    ValidLedger l →
    retractCascade l bid = some l' →
    l'.intents iid = some intent' →
    intent'.beliefId = bid' →
    bid' ∈ affectedBeliefs l bid →
    intent'.beliefStatus = BeliefStatus.retracted := by
  intro _ hcasc hintent' hmatch haff
  unfold retractCascade at hcasc
  split at hcasc
  · next b hb =>
    injection hcasc with hl'
    subst hl'
    dsimp only at hintent'
    split at hintent'
    · next intent hintent =>
      split at hintent'
      · next haff_intent =>
        injection hintent' with hieq
        subst hieq
        rfl
      · next hnaff_intent =>
        injection hintent' with hieq
        subst hieq
        have haff_orig : intent.beliefId ∈ affectedBeliefs l bid := by
          rw [hmatch]
          exact haff
        contradiction
    · contradiction
  · contradiction

/--
  Theorem 5: Cascading retraction ensures no dependent intent on an affected belief
  remains live.
-/
theorem cascade_retraction_cannot_leave_live_intent (l : Ledger) (bid : BeliefId) (l' : Ledger)
    (iid : IntentId) (intent' : ActionIntent) (bid' : BeliefId) :
    ValidLedger l →
    retractCascade l bid = some l' →
    l'.intents iid = some intent' →
    intent'.beliefId = bid' →
    bid' ∈ affectedBeliefs l bid →
    intent'.state ≠ IntentState.live := by
  intro _ hcasc hintent' hmatch haff
  unfold retractCascade at hcasc
  split at hcasc
  · next b hb =>
    injection hcasc with hl'
    subst hl'
    dsimp only at hintent'
    split at hintent'
    · next intent hintent =>
      split at hintent'
      · next haff_intent =>
        injection hintent' with hieq
        subst hieq
        split
        · intro hlive; contradiction
        · next hnlive =>
          intro hlive
          have heq : (intent.state == IntentState.live) = true := by
            rw [hlive]
            rfl
          rw [heq] at hnlive
          contradiction
      · next hnaff_intent =>
        injection hintent' with hieq
        subst hieq
        have haff_orig : intent.beliefId ∈ affectedBeliefs l bid := by
          rw [hmatch]
          exact haff
        contradiction
    · contradiction
  · contradiction

/- ==============================================================================
   PART 3: PRESERVATION THEOREMS (NON-TAUTOLOGICAL)
   ============================================================================== -/

/--
  Theorem 6: `promote` preserves ledger validity.
-/
theorem promote_preserves_validity (l : Ledger) (bid : BeliefId) (l' : Ledger) :
    ValidLedger l →
    promote l bid = some l' →
    ValidLedger l' := by
  intro ⟨hpdf, hgate, hlrp⟩ hprom
  unfold promote at hprom
  split at hprom
  · next b hb =>
    split at hprom
    · next hcond =>
      injection hprom with hl'
      subst hl'
      have h1 : (b.status == BeliefStatus.entered) = true := by
        simp only [Bool.and_eq_true] at hcond
        exact hcond.1.1
      have h2 : (b.debt == []) = true := by
        simp only [Bool.and_eq_true] at hcond
        exact hcond.1.2
      have h3 : (!b.finalTruth) = true := by
        simp only [Bool.and_eq_true] at hcond
        exact hcond.2
      have hb_status : b.status = BeliefStatus.entered := beq_iff_eq.mp h1
      have hb_debt : b.debt = [] := beq_iff_eq.mp h2
      have hb_not_final : b.finalTruth = false := by
        cases hft : b.finalTruth
        · rfl
        · rw [hft] at h3; contradiction

      refine ⟨?_, ?_, ?_⟩
      -- 1. promoted_is_debt_free
      · intro b_id b' hb' hstatus'
        dsimp only at hb'
        split at hb'
        · next heq =>
          injection hb' with hbeq
          subst hbeq
          exact ⟨hb_debt, hb_not_final⟩
        · next hneq =>
          exact hpdf b_id b' hb' hstatus'

      -- 2. gate
      · intro iid intent' b' hintent' hb'
        dsimp only at hintent' hb'
        split at hintent'
        · next intent hintent =>
          split at hintent'
          · next hbid_eq =>
            injection hintent' with hieq
            subst hieq
            have hbideq_nat : intent.beliefId = bid := beq_iff_eq.mp hbid_eq
            split at hb'
            · next hbid'_eq =>
              injection hb' with hbeq
              subst hbeq
              rfl
            · next hbid'_neq =>
              have hbideq_nat' : intent.beliefId ≠ bid := by
                intro h; rw [h, beq_self_eq_true] at hbid'_neq; contradiction
              contradiction
          · next hbid_neq =>
            injection hintent' with hieq
            subst hieq
            split at hb'
            · next hbid'_eq =>
              have hbideq_nat : intent.beliefId = bid := beq_iff_eq.mp hbid'_eq
              rw [hbideq_nat, beq_self_eq_true] at hbid_neq
              contradiction
            · next hbid'_neq =>
              exact hgate iid intent b' hintent hb'
        · contradiction

      -- 3. live_requires_promoted
      · intro iid intent' hintent' hlive'
        dsimp only at hintent'
        split at hintent'
        · next intent hintent =>
          split at hintent'
          · next hbid_eq =>
            injection hintent' with hieq
            subst hieq
            refine ⟨rfl, ?_⟩
            have hbid_nat : intent.beliefId = bid := beq_iff_eq.mp hbid_eq
            have hlive_orig : intent.state = IntentState.live := hlive'
            have ⟨_b_old, hb_old_look, hb_old_status⟩ := (hlrp iid intent hintent hlive_orig).2
            refine ⟨{ b with status := BeliefStatus.promoted }, ?_, rfl⟩
            dsimp only
            have heq_bool : (intent.beliefId == bid) = true := by simp [hbid_nat]
            rw [heq_bool]
            rfl
          · next hbid_neq =>
            injection hintent' with hieq
            subst hieq
            have hlive_orig : intent.state = IntentState.live := hlive'
            have ⟨hold_status, ⟨b_old, hb_old_look, hb_old_status⟩⟩ := hlrp iid intent hintent hlive_orig
            refine ⟨hold_status, ?_⟩
            refine ⟨b_old, ?_, hb_old_status⟩
            dsimp only
            split
            · next heq =>
              have heq_nat : intent.beliefId = bid := beq_iff_eq.mp heq
              rw [heq_nat, beq_self_eq_true] at hbid_neq
              contradiction
            · next _ =>
              exact hb_old_look
        · contradiction
    · contradiction
  · contradiction

/--
  Theorem 7: `cancelIntent` preserves ledger validity.
-/
theorem cancelIntent_preserves_validity (l : Ledger) (iid : IntentId) (l' : Ledger) :
    ValidLedger l →
    cancelIntent l iid = some l' →
    ValidLedger l' := by
  intro ⟨hpdf, hgate, hlrp⟩ hcan
  unfold cancelIntent at hcan
  split at hcan
  · next intent hintent =>
    split at hcan
    · next hlive =>
      injection hcan with hl'
      subst hl'
      refine ⟨?_, ?_, ?_⟩
      -- 1. promoted_is_debt_free
      · intro bid b hb hstatus
        dsimp only at hb
        exact hpdf bid b hb hstatus
      -- 2. gate
      · intro i_id intent' b hintent' hb
        dsimp only at hintent'
        split at hintent'
        · next heq =>
          injection hintent' with hieq
          subst hieq
          have heq_nat : i_id = iid := beq_iff_eq.mp heq
          have hintent_orig : l.intents i_id = some intent := by
            rw [heq_nat]
            exact hintent
          exact hgate i_id intent b hintent_orig hb
        · next hneq =>
          exact hgate i_id intent' b hintent' hb
      -- 3. live_requires_promoted
      · intro i_id intent' hintent' hlive'
        dsimp only at hintent'
        split at hintent'
        · next heq =>
          injection hintent' with hieq
          subst hieq
          contradiction
        · next hneq =>
          exact hlrp i_id intent' hintent' hlive'
    · contradiction
  · contradiction

/--
  Theorem 8: `authorizeIntent` preserves ledger validity.
-/
theorem authorizeIntent_preserves_validity (l : Ledger) (iid : IntentId) (l' : Ledger) :
    ValidLedger l →
    authorizeIntent l iid = some l' →
    ValidLedger l' := by
  intro ⟨hpdf, hgate, hlrp⟩ hauth
  unfold authorizeIntent at hauth
  split at hauth
  · next intent hintent =>
    split at hauth
    · next b hb =>
      split at hauth
      · next hstatus =>
        injection hauth with hl'
        subst hl'
        have hb_status : b.status = BeliefStatus.promoted := beq_iff_eq.mp hstatus
        refine ⟨?_, ?_, ?_⟩
        -- 1. promoted_is_debt_free
        · intro bid b' hb' hprom'
          dsimp only at hb'
          exact hpdf bid b' hb' hprom'
        -- 2. gate
        · intro i_id intent' b' hintent' hb'
          dsimp only at hintent'
          split at hintent'
          · next heq =>
            injection hintent' with hieq
            subst hieq
            dsimp only at hb'
            rw [hb] at hb'
            cases hb'
            rw [hb_status]
          · next hneq =>
            exact hgate i_id intent' b' hintent' hb'
        -- 3. live_requires_promoted
        · intro i_id intent' hintent' hlive'
          dsimp only at hintent'
          split at hintent'
          · next heq =>
            injection hintent' with hieq
            subst hieq
            refine ⟨rfl, ?_⟩
            exact ⟨b, hb, hb_status⟩
          · next hneq =>
            exact hlrp i_id intent' hintent' hlive'
      · contradiction
    · contradiction
  · contradiction

/--
  Theorem 9: `retractCascade` preserves ledger validity.
-/
theorem retractCascade_preserves_validity (l : Ledger) (bid : BeliefId) (l' : Ledger) :
    ValidLedger l →
    retractCascade l bid = some l' →
    ValidLedger l' := by
  intro ⟨hpdf, hgate, hlrp⟩ hcasc
  unfold retractCascade at hcasc
  split at hcasc
  · next b hb =>
    injection hcasc with hl'
    subst hl'
    refine ⟨?_, ?_, ?_⟩
    -- 1. promoted_is_debt_free
    · intro b_id b' hb' hprom'
      dsimp only at hb'
      split at hb'
      · next b_orig hlook =>
        split at hb'
        · next haff =>
          injection hb' with hbeq
          subst hbeq
          contradiction
        · next hnaff =>
          injection hb' with hbeq
          subst hbeq
          exact hpdf b_id b_orig hlook hprom'
      · contradiction

    -- 2. gate
    · intro iid intent' b' hintent' hb'
      dsimp only at hintent' hb'
      split at hintent'
      · next intent hintent =>
        split at hintent'
        · next haff_i =>
          injection hintent' with hieq
          subst hieq
          split at hb'
          · next b_orig hlook =>
            split at hb'
            · next haff_b =>
              injection hb' with hbeq
              subst hbeq
              rfl
            · next hnaff_b =>
              injection hb' with hbeq
              subst hbeq
              rfl
          · contradiction
        · next hnaff_i =>
          injection hintent' with hieq
          subst hieq
          split at hb'
          · next b_orig hlook =>
            split at hb'
            · next haff_b =>
              have hnaff_i' : intent.beliefId ∉ affectedBeliefs l bid := hnaff_i
              contradiction
            · next hnaff_b =>
              injection hb' with hbeq
              subst hbeq
              exact hgate iid intent b_orig hintent hlook
          · contradiction
      · contradiction

    -- 3. live_requires_promoted
    · intro iid intent' hintent' hlive'
      dsimp only at hintent'
      split at hintent'
      · next intent hintent =>
        split at hintent'
        · next haff_i =>
          injection hintent' with hieq
          subst hieq
          split at hlive'
          · contradiction
          · next hnlive =>
            have hnlive' : intent.state ≠ IntentState.live := by
              intro h
              have heq : (intent.state == IntentState.live) = true := by simp [h]
              contradiction
            contradiction
        · next hnaff_i =>
          injection hintent' with hieq
          subst hieq
          have hlive_orig : intent.state = IntentState.live := hlive'
          have ⟨hold_status, ⟨b_old, hb_old_look, hb_old_status⟩⟩ := hlrp iid intent hintent hlive_orig
          refine ⟨hold_status, ?_⟩
          refine ⟨b_old, ?_, hb_old_status⟩
          dsimp only
          rw [hb_old_look]
          dsimp only
          split
          · next haff_b =>
            have hsame_id : intent.beliefId ∈ affectedBeliefs l bid := haff_b
            contradiction
          · next _ =>
            rfl
      · contradiction
  · contradiction

/--
  Theorem 10: Status update during cascade preserves the gate invariant.
-/
theorem cascade_update_preserves_gate (l : Ledger) (bid : BeliefId) (l' : Ledger) :
    ValidLedger l →
    retractCascade l bid = some l' →
    gate l' := by
  intro hval hcasc
  exact (retractCascade_preserves_validity l bid l' hval hcasc).2.1

/--
  Theorem 11: Retraction via cascade structurally blocks live intents from existing
  on retracted beliefs.
-/
theorem cascade_retraction_blocks_live_intent (l : Ledger) (bid : BeliefId) (l' : Ledger)
    (bid' : BeliefId) (b' : Belief) (iid : IntentId) (intent' : ActionIntent) :
    ValidLedger l →
    retractCascade l bid = some l' →
    l'.beliefs bid' = some b' →
    b'.status = BeliefStatus.retracted →
    l'.intents iid = some intent' →
    intent'.beliefId = bid' →
    intent'.state ≠ IntentState.live := by
  intro hval hcasc hb' hret hint' hmatch
  have hval' := retractCascade_preserves_validity l bid l' hval hcasc
  intro hlive
  rw [← hmatch] at hb'
  exact no_live_intent_on_retracted_belief l' iid intent' b' hval' hint' hlive hb' hret

end Solvent
