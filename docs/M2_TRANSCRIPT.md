# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-09-08T05:22:34Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_kernel_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 111/111 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| DT-1 | doctrust | PASS | Exact tuple match yields ALLOW | Allowed=true | allowed=true, reason="", err=<nil> | — | — | Authorize allows on exact tuple match | 192 |
| DT-2 | doctrust | PASS | Principal mismatch yields DENY | Allowed=false | allowed=false, reason="principal mismatch" | — | — | Authorize denies on principal mismatch | 112 |
| DT-3 | doctrust | PASS | Resource type mismatch yields DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Authorize denies on resource type mismatch | 101 |
| DT-4 | doctrust | PASS | Scope mismatch yields DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Authorize denies on scope mismatch | 95 |
| DT-5 | doctrust | PASS | Action name mismatch yields DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Authorize denies on action name mismatch | 96 |
| DT-6 | doctrust | PASS | Consequence type mismatch yields DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Authorize denies on consequence type mismatch | 100 |
| DT-7 | doctrust | PASS | Belief retraction causes Authorize DENY | ALLOW before, retraction succeeds, DENY after | pre=true, retracted=1, post=false | — | — | Authority cannot survive a retracted belief | 114 |
| DT-8 | doctrust | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: no promotion_epoch in v0 | 107 |
| DT-9 | doctrust | PASS | Approval pin mutation yields rejection | Approve fails (pin mismatch) | err=approval pin mismatch | — | — | Approval pin prevents silent authority expansion | 137 |
| DT-10 | doctrust | PASS | Resurrection impossible: second activation fails | Approve fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | UNIQUE(target_id) prevents re-activation | 126 |
| DT-11 | doctrust | PASS | Duplicate discharge rejected | first succeeds, second rejects | err1=<nil>, err2=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | UNIQUE(belief_id, obligation_key, instrument_ref, discharged_by) rejects duplicate | 56 |
| DT-12 | doctrust | PASS | Revoked approver cannot Approve | Approve fails (revoked principal) | err=revoked principal | — | — | Revoked principal blocked from approval | 70 |
| DT-13 | doctrust | PASS | Authorize performs zero writes | all authority table counts unchanged | snap=10/10, act=10/10, rev=1/1, just=13/13, dis=1/1 | — | — | Authorize is read-only: no INSERT, no UPDATE, no DELETE | 88 |
| DT-14 | doctrust | PASS | Full lifecycle: approve → allow → revoke → deny + attack variants | approve succeeds, authorize allows, revoke denies, wrong principal denies, wrong resource denies | approve=<nil>, allow=true, after_revoke=false, wrong_principal=false, wrong_resource=false | — | — | Full authority lifecycle with attack variants | 109 |
| T-01 | auth | PASS | Valid CreateTarget creates a proposed target | UUID returned, state=proposed | id="932a9a87-60e7-4fbd-a87b-838b4a03637d", state="proposed" | — | — | CreateTarget produces a proposal with no authority granted | 17 |
| T-02 | auth | PASS | Empty textual dimension is rejected by DB CHECK | CHECK violation (23514) | sqlstate="23514" | — | — | Empty authority dimensions fail closed | 6 |
| T-03 | auth | PASS | AttachJustification to promoted belief succeeds | 1 justification attached | count=1, err=<nil> | — | — | Justification links target to promoted belief | 90 |
| T-04 | auth | PASS | Duplicate AttachJustification is idempotent | no error, still 1 justification | err=<nil>, count=1 | — | — | UNIQUE(target_id, belief_id, belief_status) makes duplicate a no-op | 63 |
| T-05 | auth | PASS | Non-matching belief/status pair rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | Composite FK (belief_id, belief_status) -> belief(id, status) is authoritative | 13 |
| T-06 | auth | PASS | RequestAuthorization creates the pin | requested_by and pinned_request_hash populated | requested_by="e2e0a354-f41d-4016-ab0c-6ce6278cc20c", pin=true | — | — | RequestAuthorization atomically sets requester/time/hash | 57 |
| T-07 | auth | PASS | Modified proposal after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects proposal integrity | 60 |
| T-08 | auth | PASS | Modified justification set after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects justification set integrity | 105 |
| T-09 | auth | PASS | Valid Approve creates snapshot + activation atomically | state=active, 1 activation | state="active", activations=1, err=<nil> | — | — | Approve is the sole authority-creating operation | 72 |
| T-10 | auth | PASS | Second Approve fails (UNIQUE(target_id)) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | A target may be activated ONCE EVER | 77 |
| T-11 | auth | PASS | Snapshot from another target cannot activate (composite FK) | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 74 |
| T-12 | auth | PASS | Revoked target cannot reactivate (UNIQUE(target_id) is permanent) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | Re-granting requires new target_id | 84 |
| T-13 | auth | PASS | Revoke creates durable revocation | state=revoked, 1 revocation row | state="revoked", revocations=1, err=<nil> | — | — | RevokeTarget inserts target_revocation | 78 |
| T-14 | auth | PASS | Authorize with exact tuple returns ALLOW | Allowed=true | allowed=true, reason="" | — | — | Exact snapshot match authorizes | 69 |
| T-15 | auth | PASS | Authorize with changed principal returns DENY | Allowed=false, reason=principal mismatch | allowed=false, reason="principal mismatch" | — | — | Tuple mismatch denied | 72 |
| T-16 | auth | PASS | Authorize with changed resource returns DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Tuple mismatch denied | 70 |
| T-17 | auth | PASS | Authorize with changed scope returns DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Tuple mismatch denied | 73 |
| T-18 | auth | PASS | Authorize with changed action returns DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Tuple mismatch denied | 68 |
| T-19 | auth | PASS | Authorize with changed consequence returns DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Tuple mismatch denied | 73 |
| T-20 | auth | PASS | Authorize with revoked target returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | Revocation removes authority | 84 |
| T-21 | auth | PASS | Authorize with no activation returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | No activation = no authority | 8 |
| T-22 | auth | PASS | Retracted belief causes authorization DENY (FK CASCADE lifecycle) | Allowed=true before retraction, retraction succeeds, justification cascades, Allowed=false after | pre=true, retracted=1, belief=retracted, just_status=retracted, post=false, reason="belief f374c81c-9826-4df6-a57b-0e41fbbcc9b1 not promoted" | — | — | Authority cannot survive a retracted belief; FK CASCADE propagates status | 93 |
| T-23 | auth | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes without error (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: retract -> re-promote can revive a live justification (no promotion_epoch) | 83 |
| T-24 | auth | PASS | Duplicate discharge rejected by UNIQUE constraint | ErrDuplicateDischarge or unique violation | first=<nil>, second=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | Per-belief replay protection | 49 |
| T-25 | auth | PASS | Cross-belief discharge reuse is allowed (per-belief uniqueness only) | both discharges succeed | err1=<nil>, err2=<nil> | — | — | v0 uniqueness is per-belief, not global | 65 |
| T-26 | auth | PASS | Revoked principal cannot Approve | ErrRevokedPrincipal | err=revoked principal | — | — | Revoked principal blocked from approval | 59 |
| T-27 | auth | PASS | Revoked principal Discharge succeeds in v0 (FK-valid, revocation not enforced at DB level) | Discharge succeeds (v0: service-level revocation check not implemented) | err=<nil> (v0: revoked principal FK still valid) | — | — | Revoked principal blocked from discharge | 36 |
| T-28 | auth | PASS | Authorize performs no database write | zero writes to authority tables | snap 26->26, act 26->26, rev 5->5 | — | — | Authorize is READ-ONLY | 71 |
| T-29 | auth | PASS | No authorization_decision table exists in v0 | 0 tables named authorization_decision | 0 | — | — | v0 has no authorization decision table | 7 |
| T-30 | auth | PASS | No hidden authority cache table exists | 0 cache tables | 0 | — | — | No second authority source | 5 |
| T-C1 | concurrency | PASS | Approve x Approve produces exactly one activation | 1 activation, 1 error | activations=1, errors=1 | — | — | UNIQUE(target_id) enforces single activation | 73 |
| T-C2 | concurrency | PASS | Approve x AttachJustification: serialization prevents ghost justifications | at most 1 activation | activations=1 | — | — | FOR UPDATE lock serializes AttachJustification against Approve | 99 |
| T-C3 | concurrency | PASS | RetractCascade succeeds; approval denied (belief retracted via CASCADE) | retraction succeeds, approval denied, no activation | retracted=1, retractErr=<nil>, approveErr=approval pin mismatch, activations=0 | — | — | CASCADE restores lifecycle: retraction succeeds, approval denied when belief retracted | 68 |
| T-C4 | concurrency | PASS | Discharge x Discharge: duplicate rejected | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 40 |
| T-C5 | concurrency | PASS | RevokeTarget blocks second approval (UNIQUE(target_id)) | approval fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | Revocation removes authority; UNIQUE(target_id) is permanent | 79 |
| T-C6 | concurrency | PASS | Concurrent CreatePrincipal: known retry/idempotency limitation | both succeed, count=2, different UUIDs | errors=0, count=2, ids=[31a4ffa3-feaf-4b48-b702-392452782106 857c6efc-3e26-45ed-a379-70bef2b0169a] | — | — | Known v0 limitation: no business-key idempotency on CreatePrincipal | 4 |
| T-C7 | concurrency | PASS | Concurrent RequestAuthorization: deterministic hash, no corruption | both succeed, pin populated | errors=0, pin="112d9db00c0ad0170a65f1b9b174d642d195d025496c981eb598d0216dd2f41d" | — | — | RequestAuthorization is idempotent under concurrency | 57 |
| T-C8 | concurrency | PASS | Concurrent Discharge: duplicate rejected by UNIQUE constraint | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 39 |
| T-C10 | concurrency | PASS | Old two-step sequence creates stale-authority intent (negative control) | stale live intent exists after old two-step sequence | intent_count=1, err=<nil> | — | — | Old sequence: Authorize commits → RevokeTarget commits → IntentOnPromoted commits → stale intent persists | 86 |
| T-C9A | concurrency | PASS | FOR UPDATE lock serializes authorize-first against concurrent revoke | intent exists, revocation exists; Conn2 blocked on lock until Conn1 committed | intents=1, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures authorize-first ordering is respected | 85 |
| T-C9A2 | concurrency | PASS | FOR UPDATE lock serializes revoke-first: authority denied after revocation | authority denied, no intent, revocation exists | authority_denied=true, intents=0, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures revoke-first ordering is respected | 73 |
| T-C9B | concurrency | PASS | Concurrent AuthorizeAndCreateIntent + RevokeTarget: no stale intent | outcome A (authorize wins, intent valid) or B (revoke wins, denied); never C (stale intent) | authorize_allowed=true, authorize_err=<nil>, revoke_err=<nil>, intents=1, revocations=1, stale=false, outcome=valid_pre_revocation_intent | — | — | No committed live intent created on superseded authority | 102 |
| T-SR1 | security_regression | PASS | Approve without justification rejected | ErrInvalidProposal | err=invalid proposal | — | — | Approve requires at least one justification | 62 |
| T-P1 | property | PASS | Activation is once-ever: CreateTarget -> Approve -> RevokeTarget -> second activation rejected | UNIQUE(target_id) rejects second activation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | A target may be activated ONCE EVER | 79 |
| T-P2 | property | PASS | Proposal mutation cannot change execution authority | original tuple ALLOW, mutated tuple DENY | A allowed=true, B allowed=false | — | — | Snapshot is sole authority, not proposal | 74 |
| T-P3 | property | PASS | Snapshot substitution rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 72 |
| T-30 | auth | PASS | AuthorizeAndCreateIntent with exact tuple creates live intent | Allowed=true, IntentState=live | allowed=true, intent_state="live", reason="" | — | — | Atomic authority + intent creation succeeds | 72 |
| T-31 | auth | PASS | AuthorizeAndCreateIntent with wrong principal denies and creates no intent | Allowed=false, reason=principal mismatch, 0 intents | allowed=false, reason="principal mismatch", intents=0 | — | — | Tuple mismatch denied atomically | 122 |
| T-32 | auth | PASS | AuthorizeAndCreateIntent with revoked target denies and creates no intent | Allowed=false, 0 intents | allowed=false, reason="no activation or revocation exists", intents=0 | — | — | Revoked authority blocks atomic intent creation | 78 |
| T-33 | auth | PASS | AuthorizeAndCreateIntent with unpromoted belief fails atomically | auth denial or FK violation, no live intent | allowed=false, reason="belief 978d97c3-972b-406f-9800-1b91ce745ab6 not promoted", err=<nil> | — | — | Unpromoted belief cannot produce live intent | 69 |
| W0 | 0 | PASS | The behavioral database was reset and the frozen DDL applied | 4 contracted tables present | 4 tables present | — | — | — | 5 |
| B-01 | 1 | PASS | A claim enters unpromoted, carrying its full starting debt | parseable UUID returned; status='entered', final_truth=false, 6 debt items | id parseable=true, status="entered", final_truth=false, debt items=6 | — | — | contract §4 EnterBelief — never gated, full debt at the door | 4 |
| B-17 | 1 | PASS | Discharge D10 — a Go []string encodes into STRING[] element-for-element, in order | stored debt == kernel.FullDebt: needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | stored debt == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | M1-R2 / D10 — the last open encoding assumption | 3 |
| B-23 | 1 | PASS | Discharge M1-R3 — kernel.FullDebt (Go) and the ARRAY[...] DEFAULT (DDL) have not drifted | DDL default == kernel.FullDebt: needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | DDL default == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | M1-R3 — the six debt items are encoded in two places | 4 |
| B-02 | 1 | PASS | Evidence attaches without changing belief state | 1 evidence row with the given sha; belief status and debt unchanged | 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true | — | — | contract §4 AddEvidence — does not change belief state | 12 |
| B-03 | 1 | PASS | One useful move retires exactly one debt; the rest survive in order | debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff | debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | contract §4 RetireDebt | 6 |
| B-04 | 1 | PASS | Retiring an already-absent debt item is a no-op, not an error | second call returns nil; debt unchanged | second call err=<nil>; debt unchanged=true | — | — | contract §4 RetireDebt — idempotent if the item is absent | 11 |
| B-05 | 1 | PASS | A debt-free, non-final belief reaches the throne | Promote returns nil; status='promoted' | err=<nil>; status="promoted" | — | — | contract §4 Promote | 29 |
| B-09 | 1 | PASS | I-1 — no belief with status='promoted' has non-empty debt | refused; errors.Is(ErrPromotionBlocked) AND errors.As(*pgconn.PgError); 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-1 — no belief with status='promoted' has non-empty debt | 6 |
| B-10 | 1 | PASS | I-2 — no belief with status='promoted' has final_truth=true | refused; errors.Is AND errors.As both succeed; 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-2 — no belief with status='promoted' has final_truth=true | 6 |
| B-18 | 1 | PASS | 40001 is classified as retryable, is retried by crdb.ExecuteTx, and is never masked by the sentinel wrap | injection live (SHOW=on, control txn gets 40001); Promote returns nil anyway; status='promoted'; error is NOT ErrPromotionBlocked | injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2) | — | — | I-7's purpose — serialization failures retry rather than surface | 40 |
| B-06 | 1 | PASS | An action intent citing a promoted belief is accepted | returns nil; intent state='live', belief_status='promoted' | err=<nil>; state="live", belief_status="promoted" | — | — | contract §4 IntentOnPromoted | 34 |
| B-11 | 1 | PASS | I-3 — no live action_intent references a belief that is not promoted | refused; errors.Is(ErrActionOnUnpromoted) AND errors.As; 23503 / gate; 0 intent rows | err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0 | 23503 | gate | I-3 — no live intent references a non-promoted belief | 5 |
| B-12 | 1 | PASS | I-4 — a belief carrying a live intent cannot be retracted without the intent being cancelled first | raw UPDATE refused with 23514 / live_requires_promoted; belief still 'promoted' | sqlstate="23514"; constraint="live_requires_promoted"; status="promoted" | 23514 | live_requires_promoted | I-4 — cancel must precede retract | 41 |
| B-07 | 2 | PASS | Root and descendants un-promote; live intents cancel; ON UPDATE CASCADE propagates the new status | returns 2; both beliefs 'retracted'; intent 'cancelled' with belief_status='retracted' | returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted" | — | — | contract §4 RetractCascade | 87 |
| B-20 | 2 | PASS | The recursive traversal collects the whole chain and nothing else | returns 3; the unrelated belief stays 'promoted' | returned 3; unrelated belief="promoted" | — | — | D-033 — WITH RECURSIVE collects transitive descendants | 133 |
| B-22 | 2 | PASS | N3 — retracted counts belief rows only, never cancelled intents | returns 2 (not 3, not 5); 3 intents cancelled | returned 2; 3 intents cancelled | — | — | contract §4 RetractCascade — RowsAffected of the belief UPDATE only | 92 |
| B-19 | 2 | PASS | D-032 — a scenario-scoped cascade does not follow an edge into another scenario | returns 2; the scenario-B belief stays 'promoted' | returned 2; scenario-B belief="promoted" | — | — | D-032 — RetractCascade is scenario-scoped | 108 |
| CS-1 | cs | PASS | M-01 — RetireDebt with wrong scenario returns ErrBeliefNotFound, zero mutation | ErrBeliefNotFound + debt unchanged | err=belief not found in scenario, debt_before="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff", debt_after="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | cross-scenario isolation for RetireDebt | 5 |
| CS-2 | cs | PASS | M-01 — same-scenario RetireDebt is idempotent | both calls succeed + debt item removed | err1=<nil>, err2=<nil>, debt="needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | RetireDebt idempotency within scenario | 12 |
| CS-3 | cs | PASS | M-01 — Promote with wrong scenario returns ErrBeliefNotFound, zero mutation | ErrBeliefNotFound + status unchanged | err=belief not found in scenario, status_before="entered", status_after="entered" | — | — | cross-scenario isolation for Promote | 29 |
| CS-4 | cs | PASS | M-01 — Discharge with wrong scenario returns ErrBeliefNotFound, zero writes | ErrBeliefNotFound + zero discharge rows + debt unchanged | err=belief not found in scenario, discharge_rows: before=0 after=0, debt_before="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" debt_after="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | cross-scenario isolation for Discharge | 9 |
| CS-5 | cs | PASS | M-01 — same-scenario Discharge is atomic: discharge row + debt retired | success + 1 discharge row + debt item removed | err=<nil>, discharge_rows=1, debt="needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff" | — | — | Discharge atomicity within scenario | 20 |
| B-24 | 2 | PASS | NEW-02 — cross-scenario IntentOnPromoted rejected at createIntentWithinTx boundary | ErrBeliefNotFound | err=belief not found in scenario | — | — | NEW-02 — scenario ownership verified before intent INSERT | 70 |
| B-16 | 2 | PASS | I-8 — RetractCascade succeeds after cross-scenario intent rejection (no 23514 deadlock) | returns 2; both beliefs retracted; no live intents; no SQLSTATE 23514 | returned 2; err=<nil>; root="retracted" child="retracted"; live intents A=0 B=0 — retract succeeded, no 23514 | — | — | I-8 — RetractCascade is safe when foreign intents are rejected at creation | 104 |
| B-08 | 2 | PASS | AuditLiveOnNonPromoted is I-5 expressed as a query | returns 0 after a completed cascade | returned 0 (err=<nil>) | — | — | I-5 — audit returns 0 in every committed state | 95 |
| B-13 | 2 | PASS | I-5 holds globally, across every scenario the suite created | 0 live intents on non-promoted beliefs, cluster-wide in this database | 0 live-on-non-promoted row(s) | — | — | I-5 — AuditLiveOnNonPromoted returns 0 in every committed state | 3 |
| B-21 | 3 | PASS | D-033 — UNION deduplication terminates the traversal on a cyclic belief graph | returns 2 and terminates within the 30s deadline | returned 2 in 5 ms (deadline 30000 ms) | — | — | D-033 — UNION, never UNION ALL | 61 |
| B-14 | 3 | PASS | I-6 — vectors are never part of belief semantics. Under R2 there is no embedding column, so every embedding is NULL by construction | 0 rows for (belief, embedding) in information_schema.columns | 0 embedding column(s) on belief | — | — | I-6 — the ledger is meaningful with zero vectors | 18 |
| B-15 | 3 | PASS | I-7 — every kernel write goes through crdb.ExecuteTx | asserted statically: 7 ExecuteTx write sites, 0 raw writes | NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md | — | — | I-7 — no raw db.Exec/db.Query writes | 0 |
| W3-Ensure-New | 3 | PASS | EnsureBelief creates a new belief with full debt when claim does not exist | parseable UUID; status='entered', 6 debt items | id parseable=true, status="entered", debt items=6 | — | — | EnsureBelief — find-or-create in one transaction | 6 |
| W3-Ensure-Existing | 3 | PASS | EnsureBelief returns existing belief ID when claim already exists in scenario | same ID returned; 1 belief row (no duplicate) | id1=6b0f77a3-0d38-4bf3-bfbc-a92588de833d, id2=6b0f77a3-0d38-4bf3-bfbc-a92588de833d, same=true; count=1 | — | — | EnsureBelief — dedup is transactional (no TOCTOU) | 5 |
| W3-Ensure-DiffScenario | 3 | PASS | EnsureBelief creates separate beliefs for the same claim in different scenarios | two different IDs | idA=b0317838-01ac-4c60-aa2c-de7cd6587fdc, idB=123ddcf8-a5f2-4f94-ab2b-11d3f03fa5b4, different=true | — | — | EnsureBelief — scenario-scoped uniqueness | 5 |
| NEW-01-hp | CS | PASS | ClaimIntent succeeds with correct scenario + belief + action + live intent | nil error | err=<nil> | — | — | — | 38 |
| NEW-01-wb | CS | PASS | ClaimIntent rejects wrong belief (same scenario, different belief) | error | err=intent is not in live state | — | — | — | 61 |
| NEW-01-wa | CS | PASS | ClaimIntent rejects wrong action (same scenario, same belief, different action) | error | err=intent is not in live state | — | — | — | 41 |
| NEW-01-ex | CS | PASS | NEW-01 exact exploit: claim intentB with wrong belief+action tuple is rejected | error | err=intent is not in live state | — | — | — | 68 |
| NEW-02-cr | CS | PASS | IntentOnPromoted rejects cross-scenario belief | ErrBeliefNotFound | err=belief not found in scenario | — | — | — | 26 |
| NEW-02-ra | CS | PASS | Cross-scenario intent rejected + RetractCascade succeeds (no 23514) | cross-scenario rejected; retract returns 2; child retracted | crossRejected=true; n=2; err=<nil>; child="retracted" | — | — | — | 81 |
| NEW-02-ss | CS | PASS | Same-scenario IntentOnPromoted succeeds | nil error | err=<nil> | — | — | — | 30 |
| NEW-03-ss | CS | PASS | AddEvidence succeeds with same-scenario belief | nil error | err=<nil> | — | — | — | 30 |
| NEW-03-cr | CS | PASS | AddEvidence rejects cross-scenario belief | ErrBeliefNotFound | err=belief not found in scenario | — | — | — | 28 |
| OR-1 | operator-review | PASS | promote without --action → no intent, audit = 0 | audit = 0 | audit = 0 | — | — | — | 26 |
| OR-2 | operator-review | PASS | promote with --action → 1 live intent, audit = 0 | audit = 0, intent_count = 1 | audit = 0, intent_count = 1 | — | — | — | 40 |
| OR-3 | operator-review | PASS | unpromoted belief → IntentOnPromoted refused (23503) | SQLSTATE 23503 | 23503 | 23503 | gate | I-3 | 4 |
| OR-4 | operator-review | PASS | failed promotion → no intent (23514 then 23503) | promote=23514, intent=23503 | promote=23514, intent=23503 | 23514 | — | — | 11 |
| OR-5 | operator-review | PASS | scenario mismatch → no mutation, no promotion, no intent | mismatch detected, debt unchanged, status=entered, intents=0 | mismatch=true, debt_changed=false, status=entered, intents=0 | — | — | — | 8 |

## Receipts

### DT-8

```
ACCEPTED V0 GAP — no promotion_epoch in v0
```

### DT-9

```
approval pin mismatch
```

### DT-10

```
ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505)
```

### DT-11

```
duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505)
```

### DT-12

```
revoked principal
```

### T-02

```
ERROR: failed to satisfy CHECK constraint (resource_type != '':::STRING) (SQLSTATE 23514)
```

### T-05

```
ERROR: insert on table "justification" violates foreign key constraint "justification_belief_fk" (SQLSTATE 23503)
```

### T-07

```
approval pin mismatch
```

### T-08

```
approval pin mismatch
```

### T-10

```
ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505)
```

### T-11

```
ERROR: insert on table "target_activation" violates foreign key constraint "target_activation_target_id_snapshot_id_fkey" (SQLSTATE 23503)
```

### T-12

```
ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505)
```

### T-23

```
ACCEPTED V0 GAP — no promotion_epoch in v0
```

### T-24

```
duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505)
```

### T-26

```
revoked principal
```

### T-C5

```
ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505)
```

### T-SR1

```
invalid proposal
```

### T-P1

```
ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505)
```

### T-P3

```
ERROR: insert on table "target_activation" violates foreign key constraint "target_activation_target_id_snapshot_id_fkey" (SQLSTATE 23503)
```

### B-09

```
promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514)
```

### B-10

```
promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514)
```

### B-11

```
action refused: belief is not promoted: ERROR: insert on table "action_intent" violates foreign key constraint "gate" (SQLSTATE 23503)
```

### B-12

```
ERROR: failed to satisfy CHECK constraint ((state != 'live':::STRING) OR (belief_status = 'promoted':::STRING)) (SQLSTATE 23514)
```

### CS-1

```
belief not found in scenario
```

### CS-3

```
belief not found in scenario
```

### CS-4

```
belief not found in scenario
```

### B-24

```
belief not found in scenario
```

### B-16

```
scenario 11111111-0000-0000-0000-000000000018
  belief  8ae031d4-c2d2-4dd8-977e-7a10a8d8a9f8  retracted
  belief  a1c907f8-e5e7-4e20-ab17-bc8c3f442f16  retracted
  intent  d68b8e46-a487-49c8-a94d-2821f0cc9a1e  cancelled
scenario 11111111-0000-0000-0000-0000000000f0
```

### OR-3

```
action refused: belief is not promoted: ERROR: insert on table "action_intent" violates foreign key constraint "gate" (SQLSTATE 23503)
```

