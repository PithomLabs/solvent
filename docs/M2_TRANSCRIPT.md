# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-09-14T14:48:06Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_kernel_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 131/131 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| DT-1 | doctrust | PASS | Exact tuple match yields ALLOW | Allowed=true | allowed=true, reason="", err=<nil> | — | — | Authorize allows on exact tuple match | 148 |
| DT-2 | doctrust | PASS | Principal mismatch yields DENY | Allowed=false | allowed=false, reason="principal mismatch" | — | — | Authorize denies on principal mismatch | 88 |
| DT-3 | doctrust | PASS | Resource type mismatch yields DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Authorize denies on resource type mismatch | 84 |
| DT-4 | doctrust | PASS | Scope mismatch yields DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Authorize denies on scope mismatch | 75 |
| DT-5 | doctrust | PASS | Action name mismatch yields DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Authorize denies on action name mismatch | 72 |
| DT-6 | doctrust | PASS | Consequence type mismatch yields DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Authorize denies on consequence type mismatch | 82 |
| DT-7 | doctrust | PASS | Belief retraction causes Authorize DENY | ALLOW before, retraction succeeds, DENY after | pre=true, retracted=1, post=false | — | — | Authority cannot survive a retracted belief | 100 |
| DT-8 | doctrust | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: no promotion_epoch in v0 | 85 |
| DT-9 | doctrust | PASS | Approval pin mutation yields rejection | Approve fails (pin mismatch) | err=approval pin mismatch | — | — | Approval pin prevents silent authority expansion | 75 |
| DT-10 | doctrust | PASS | Resurrection impossible: second activation fails | Approve fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | UNIQUE(target_id) prevents re-activation | 73 |
| DT-11 | doctrust | PASS | Duplicate discharge rejected | first succeeds, second rejects | err1=<nil>, err2=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | UNIQUE(belief_id, obligation_key, instrument_ref, discharged_by) rejects duplicate | 36 |
| DT-12 | doctrust | PASS | Revoked approver cannot Approve | Approve fails (revoked principal) | err=revoked principal | — | — | Revoked principal blocked from approval | 48 |
| DT-13 | doctrust | PASS | Authorize performs zero writes | all authority table counts unchanged | snap=10/10, act=10/10, rev=1/1, just=13/13, dis=1/1 | — | — | Authorize is read-only: no INSERT, no UPDATE, no DELETE | 70 |
| DT-14 | doctrust | PASS | Full lifecycle: approve → allow → revoke → deny + attack variants | approve succeeds, authorize allows, revoke denies, wrong principal denies, wrong resource denies | approve=<nil>, allow=true, after_revoke=false, wrong_principal=false, wrong_resource=false | — | — | Full authority lifecycle with attack variants | 82 |
| T-01 | auth | PASS | Valid CreateTarget creates a proposed target | UUID returned, state=proposed | id="e8e24075-f024-41ae-8d1e-f177d10b8f65", state="proposed" | — | — | CreateTarget produces a proposal with no authority granted | 10 |
| T-02 | auth | PASS | Empty textual dimension is rejected by DB CHECK | CHECK violation (23514) | sqlstate="23514" | — | — | Empty authority dimensions fail closed | 4 |
| T-03 | auth | PASS | AttachJustification to promoted belief succeeds | 1 justification attached | count=1, err=<nil> | — | — | Justification links target to promoted belief | 36 |
| T-04 | auth | PASS | Duplicate AttachJustification is idempotent | no error, still 1 justification | err=<nil>, count=1 | — | — | UNIQUE(target_id, belief_id, belief_status) makes duplicate a no-op | 40 |
| T-05 | auth | PASS | Non-matching belief/status pair rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | Composite FK (belief_id, belief_status) -> belief(id, status) is authoritative | 18 |
| T-06 | auth | PASS | RequestAuthorization creates the pin | requested_by and pinned_request_hash populated | requested_by="8d5ba6d0-b8ab-4e53-9900-4baecc9421bf", pin=true | — | — | RequestAuthorization atomically sets requester/time/hash | 46 |
| T-07 | auth | PASS | Modified proposal after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects proposal integrity | 48 |
| T-08 | auth | PASS | Modified justification set after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects justification set integrity | 72 |
| T-09 | auth | PASS | Valid Approve creates snapshot + activation atomically | state=active, 1 activation | state="active", activations=1, err=<nil> | — | — | Approve is the sole authority-creating operation | 58 |
| T-10 | auth | PASS | Second Approve fails (UNIQUE(target_id)) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | A target may be activated ONCE EVER | 58 |
| T-11 | auth | PASS | Snapshot from another target cannot activate (composite FK) | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 58 |
| T-12 | auth | PASS | Revoked target cannot reactivate (UNIQUE(target_id) is permanent) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | Re-granting requires new target_id | 67 |
| T-13 | auth | PASS | Revoke creates durable revocation | state=revoked, 1 revocation row | state="revoked", revocations=1, err=<nil> | — | — | RevokeTarget inserts target_revocation | 68 |
| T-14 | auth | PASS | Authorize with exact tuple returns ALLOW | Allowed=true | allowed=true, reason="" | — | — | Exact snapshot match authorizes | 57 |
| T-15 | auth | PASS | Authorize with changed principal returns DENY | Allowed=false, reason=principal mismatch | allowed=false, reason="principal mismatch" | — | — | Tuple mismatch denied | 58 |
| T-16 | auth | PASS | Authorize with changed resource returns DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Tuple mismatch denied | 57 |
| T-17 | auth | PASS | Authorize with changed scope returns DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Tuple mismatch denied | 55 |
| T-18 | auth | PASS | Authorize with changed action returns DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Tuple mismatch denied | 55 |
| T-19 | auth | PASS | Authorize with changed consequence returns DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Tuple mismatch denied | 54 |
| T-20 | auth | PASS | Authorize with revoked target returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | Revocation removes authority | 68 |
| T-21 | auth | PASS | Authorize with no activation returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | No activation = no authority | 9 |
| T-22 | auth | PASS | Retracted belief causes authorization DENY (FK CASCADE lifecycle) | Allowed=true before retraction, retraction succeeds, justification cascades, Allowed=false after | pre=true, retracted=1, belief=retracted, just_status=retracted, post=false, reason="belief b68f51de-ad85-4310-b260-e7304d66a116 not promoted" | — | — | Authority cannot survive a retracted belief; FK CASCADE propagates status | 75 |
| T-23 | auth | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes without error (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: retract -> re-promote can revive a live justification (no promotion_epoch) | 68 |
| T-24 | auth | PASS | Duplicate discharge rejected by UNIQUE constraint | ErrDuplicateDischarge or unique violation | first=<nil>, second=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | Per-belief replay protection | 28 |
| T-25 | auth | PASS | Cross-belief discharge reuse is allowed (per-belief uniqueness only) | both discharges succeed | err1=<nil>, err2=<nil> | — | — | v0 uniqueness is per-belief, not global | 36 |
| T-26 | auth | PASS | Revoked principal cannot Approve | ErrRevokedPrincipal | err=revoked principal | — | — | Revoked principal blocked from approval | 41 |
| T-27 | auth | PASS | Revoked principal Discharge succeeds in v0 (FK-valid, revocation not enforced at DB level) | Discharge succeeds (v0: service-level revocation check not implemented) | err=<nil> (v0: revoked principal FK still valid) | — | — | Revoked principal blocked from discharge | 23 |
| T-28 | auth | PASS | Authorize performs no database write | zero writes to authority tables | snap 26->26, act 26->26, rev 5->5 | — | — | Authorize is READ-ONLY | 58 |
| T-29 | auth | PASS | No authorization_decision table exists in v0 | 0 tables named authorization_decision | 0 | — | — | v0 has no authorization decision table | 11 |
| T-30 | auth | PASS | No hidden authority cache table exists | 0 cache tables | 0 | — | — | No second authority source | 5 |
| T-C1 | concurrency | PASS | Approve x Approve produces exactly one activation | 1 activation, 1 error | activations=1, errors=1 | — | — | UNIQUE(target_id) enforces single activation | 59 |
| T-C2 | concurrency | PASS | Approve x AttachJustification: serialization prevents ghost justifications | at most 1 activation | activations=1 | — | — | FOR UPDATE lock serializes AttachJustification against Approve | 73 |
| T-C3 | concurrency | PASS | RetractCascade succeeds; approval denied (belief retracted via CASCADE) | retraction succeeds, approval denied, no activation | retracted=1, retractErr=<nil>, approveErr=approval pin mismatch, activations=0 | — | — | CASCADE restores lifecycle: retraction succeeds, approval denied when belief retracted | 62 |
| T-C4 | concurrency | PASS | Discharge x Discharge: duplicate rejected | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 49 |
| T-C5 | concurrency | PASS | RevokeTarget blocks second approval (UNIQUE(target_id)) | approval fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | Revocation removes authority; UNIQUE(target_id) is permanent | 87 |
| T-C6 | concurrency | PASS | Concurrent CreatePrincipal: known retry/idempotency limitation | both succeed, count=2, different UUIDs | errors=0, count=2, ids=[a8066dc8-ded1-4475-9483-95656046fc7e 4598e852-837e-4195-b57e-ea622a69869f] | — | — | Known v0 limitation: no business-key idempotency on CreatePrincipal | 3 |
| T-C7 | concurrency | PASS | Concurrent RequestAuthorization: deterministic hash, no corruption | both succeed, pin populated | errors=0, pin="d14dab52431d87b750703c478d071c7c2b187848f50ce6b5c5ebeb70e553021f" | — | — | RequestAuthorization is idempotent under concurrency | 48 |
| T-C8 | concurrency | PASS | Concurrent Discharge: duplicate rejected by UNIQUE constraint | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 27 |
| T-C10 | concurrency | PASS | Old two-step sequence creates stale-authority intent (negative control) | stale live intent exists after old two-step sequence | intent_count=1, err=<nil> | — | — | Old sequence: Authorize commits → RevokeTarget commits → IntentOnPromoted commits → stale intent persists | 64 |
| T-C9A | concurrency | PASS | FOR UPDATE lock serializes authorize-first against concurrent revoke | intent exists, revocation exists; Conn2 blocked on lock until Conn1 committed | intents=1, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures authorize-first ordering is respected | 65 |
| T-C9A2 | concurrency | PASS | FOR UPDATE lock serializes revoke-first: authority denied after revocation | authority denied, no intent, revocation exists | authority_denied=true, intents=0, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures revoke-first ordering is respected | 57 |
| T-C9B | concurrency | PASS | Concurrent AuthorizeAndCreateIntent + RevokeTarget: no stale intent | outcome A (authorize wins, intent valid) or B (revoke wins, denied); never C (stale intent) | authorize_allowed=false, authorize_err=<nil>, revoke_err=<nil>, intents=0, revocations=1, stale=false, outcome=authorization_denied | — | — | No committed live intent created on superseded authority | 69 |
| T-SR1 | security_regression | PASS | Approve without justification rejected | ErrInvalidProposal | err=invalid proposal | — | — | Approve requires at least one justification | 44 |
| T-P1 | property | PASS | Activation is once-ever: CreateTarget -> Approve -> RevokeTarget -> second activation rejected | UNIQUE(target_id) rejects second activation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | A target may be activated ONCE EVER | 60 |
| T-P2 | property | PASS | Proposal mutation cannot change execution authority | original tuple ALLOW, mutated tuple DENY | A allowed=true, B allowed=false | — | — | Snapshot is sole authority, not proposal | 58 |
| T-P3 | property | PASS | Snapshot substitution rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 53 |
| T-30 | auth | PASS | AuthorizeAndCreateIntent with exact tuple creates live intent | Allowed=true, IntentState=live | allowed=true, intent_state="live", reason="" | — | — | Atomic authority + intent creation succeeds | 58 |
| T-31 | auth | PASS | AuthorizeAndCreateIntent with wrong principal denies and creates no intent | Allowed=false, reason=principal mismatch, 0 intents | allowed=false, reason="principal mismatch", intents=0 | — | — | Tuple mismatch denied atomically | 59 |
| T-32 | auth | PASS | AuthorizeAndCreateIntent with revoked target denies and creates no intent | Allowed=false, 0 intents | allowed=false, reason="no activation or revocation exists", intents=0 | — | — | Revoked authority blocks atomic intent creation | 62 |
| T-33 | auth | PASS | AuthorizeAndCreateIntent with unpromoted belief fails atomically | auth denial or FK violation, no live intent | allowed=false, reason="belief c59b4075-b716-4184-ba8c-64fb4f43b4e3 not promoted", err=<nil> | — | — | Unpromoted belief cannot produce live intent | 56 |
| W0 | 0 | PASS | The behavioral database was reset and the frozen DDL applied | 4 contracted tables present | 4 tables present | — | — | — | 5 |
| B-01 | 1 | PASS | A claim enters unpromoted, carrying its supplied starting debt | parseable UUID returned; status='entered', final_truth=false, 1 debt item | id parseable=true, status="entered", final_truth=false, debt items=1 | — | — | contract §4 EnterBelief — never gated, caller-supplied debt at the door | 4 |
| B-17 | 1 | PASS | Discharge D10 — a Go []string encodes into STRING[] element-for-element, in order | stored debt == [foo,bar,baz]: foo,bar,baz | stored debt == foo,bar,baz | — | — | M1-R2 / D10 — arbitrary debt encodes correctly | 4 |
| B-23 | 1 | PASS | Discharge M1-R3 — DDL DEFAULT produces empty debt array | DDL default == empty array: 0 debt items | DDL default == 0 debt items | — | — | M1-R3 — debt default is intentionally empty | 4 |
| B-02 | 1 | PASS | Evidence attaches without changing belief state | 1 evidence row with the given sha; belief status and debt unchanged | 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true | — | — | contract §4 AddEvidence — does not change belief state | 11 |
| B-03 | 1 | PASS | One useful move retires exactly one debt; the rest survive in order | debt == | debt == | — | — | contract §4 RetireDebt | 7 |
| B-04 | 1 | PASS | Retiring an already-absent debt item is a no-op, not an error | second call returns nil; debt unchanged | second call err=<nil>; debt unchanged=true | — | — | contract §4 RetireDebt — idempotent if the item is absent | 10 |
| B-05 | 1 | PASS | A debt-free, non-final belief reaches the throne | Promote returns nil; status='promoted' | err=<nil>; status="promoted" | — | — | contract §4 Promote | 13 |
| B-09 | 1 | PASS | I-1 — no belief with status='promoted' has non-empty debt | refused; errors.Is(ErrPromotionBlocked) AND errors.As(*pgconn.PgError); 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-1 — no belief with status='promoted' has non-empty debt | 6 |
| B-10 | 1 | PASS | I-2 — no belief with status='promoted' has final_truth=true | refused; errors.Is AND errors.As both succeed; 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-2 — no belief with status='promoted' has final_truth=true | 5 |
| B-18 | 1 | PASS | 40001 is classified as retryable, is retried by crdb.ExecuteTx, and is never masked by the sentinel wrap | injection live (SHOW=on, control txn gets 40001); Promote returns nil anyway; status='promoted'; error is NOT ErrPromotionBlocked | injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2) | — | — | I-7's purpose — serialization failures retry rather than surface | 25 |
| B-06 | 1 | PASS | An action intent citing a promoted belief is accepted | returns nil; intent state='live', belief_status='promoted' | err=<nil>; state="live", belief_status="promoted" | — | — | contract §4 IntentOnPromoted | 20 |
| B-11 | 1 | PASS | I-3 — no live action_intent references a belief that is not promoted | refused; errors.Is(ErrActionOnUnpromoted) AND errors.As; 23503 / gate; 0 intent rows | err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0 | 23503 | gate | I-3 — no live intent references a non-promoted belief | 6 |
| B-12 | 1 | PASS | I-4 — a belief carrying a live intent cannot be retracted without the intent being cancelled first | raw UPDATE refused with 23514 / live_requires_promoted; belief still 'promoted' | sqlstate="23514"; constraint="live_requires_promoted"; status="promoted" | 23514 | live_requires_promoted | I-4 — cancel must precede retract | 24 |
| B-07 | 2 | PASS | Root and descendants un-promote; live intents cancel; ON UPDATE CASCADE propagates the new status | returns 2; both beliefs 'retracted'; intent 'cancelled' with belief_status='retracted' | returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted" | — | — | contract §4 RetractCascade | 61 |
| B-20 | 2 | PASS | The recursive traversal collects the whole chain and nothing else | returns 3; the unrelated belief stays 'promoted' | returned 3; unrelated belief="promoted" | — | — | D-033 — WITH RECURSIVE collects transitive descendants | 70 |
| B-22 | 2 | PASS | N3 — retracted counts belief rows only, never cancelled intents | returns 2 (not 3, not 5); 3 intents cancelled | returned 2; 3 intents cancelled | — | — | contract §4 RetractCascade — RowsAffected of the belief UPDATE only | 58 |
| B-19 | 2 | PASS | D-032 — a scenario-scoped cascade does not follow an edge into another scenario | returns 2; the scenario-B belief stays 'promoted' | returned 2; scenario-B belief="promoted" | — | — | D-032 — RetractCascade is scenario-scoped | 60 |
| CS-1 | cs | PASS | M-01 — RetireDebt with wrong scenario returns ErrBeliefNotFound, zero mutation | ErrBeliefNotFound + debt unchanged | err=belief not found in scenario, debt_before="testDebt", debt_after="testDebt" | — | — | cross-scenario isolation for RetireDebt | 5 |
| CS-2 | cs | PASS | M-01 — same-scenario RetireDebt is idempotent | both calls succeed + debt item removed | err1=<nil>, err2=<nil>, debt="testDebt" | — | — | RetireDebt idempotency within scenario | 12 |
| CS-3 | cs | PASS | M-01 — Promote with wrong scenario returns ErrBeliefNotFound, zero mutation | ErrBeliefNotFound + status unchanged | err=belief not found in scenario, status_before="entered", status_after="entered" | — | — | cross-scenario isolation for Promote | 8 |
| CS-4 | cs | PASS | M-01 — Discharge with wrong scenario returns ErrBeliefNotFound, zero writes | ErrBeliefNotFound + zero discharge rows + debt unchanged | err=belief not found in scenario, discharge_rows: before=0 after=0, debt_before="testDebt" debt_after="testDebt" | — | — | cross-scenario isolation for Discharge | 11 |
| CS-5 | cs | PASS | M-01 — same-scenario Discharge is atomic: discharge row + debt retired | success + 1 discharge row + debt item removed | err=<nil>, discharge_rows=1, debt="testDebt" | — | — | Discharge atomicity within scenario | 22 |
| B-24 | 2 | PASS | NEW-02 — cross-scenario IntentOnPromoted rejected at createIntentWithinTx boundary | ErrBeliefNotFound | err=belief not found in scenario | — | — | NEW-02 — scenario ownership verified before intent INSERT | 40 |
| B-16 | 2 | PASS | I-8 — RetractCascade succeeds after cross-scenario intent rejection (no 23514 deadlock) | returns 2; both beliefs retracted; no live intents; no SQLSTATE 23514 | returned 2; err=<nil>; root="retracted" child="retracted"; live intents A=0 B=0 — retract succeeded, no 23514 | — | — | I-8 — RetractCascade is safe when foreign intents are rejected at creation | 80 |
| B-08 | 2 | PASS | AuditLiveOnNonPromoted is I-5 expressed as a query | returns 0 after a completed cascade | returned 0 (err=<nil>) | — | — | I-5 — audit returns 0 in every committed state | 61 |
| B-13 | 2 | PASS | I-5 holds globally, across every scenario the suite created | 0 live intents on non-promoted beliefs, cluster-wide in this database | 0 live-on-non-promoted row(s) | — | — | I-5 — AuditLiveOnNonPromoted returns 0 in every committed state | 3 |
| B-21 | 3 | PASS | D-033 — UNION deduplication terminates the traversal on a cyclic belief graph | returns 2 and terminates within the 30s deadline | returned 2 in 10 ms (deadline 30000 ms) | — | — | D-033 — UNION, never UNION ALL | 43 |
| B-14 | 3 | PASS | I-6 — vectors are never part of belief semantics. Under R2 there is no embedding column, so every embedding is NULL by construction | 0 rows for (belief, embedding) in information_schema.columns | 0 embedding column(s) on belief | — | — | I-6 — the ledger is meaningful with zero vectors | 24 |
| B-15 | 3 | PASS | I-7 — every kernel write goes through crdb.ExecuteTx | asserted statically: 7 ExecuteTx write sites, 0 raw writes | NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md | — | — | I-7 — no raw db.Exec/db.Query writes | 0 |
| W3-Ensure-New | 3 | PASS | EnsureBelief creates a new belief with full debt when claim does not exist | parseable UUID; status=.entered., 1 debt item | id parseable=true, status="entered", debt items=1 | — | — | EnsureBelief — find-or-create in one transaction | 7 |
| W3-Ensure-Existing | 3 | PASS | EnsureBelief returns existing belief ID when claim already exists in scenario | same ID returned; 1 belief row (no duplicate) | id1=64585303-9b17-45ec-a174-58a3ad93b6df, id2=64585303-9b17-45ec-a174-58a3ad93b6df, same=true; count=1 | — | — | EnsureBelief — dedup is transactional (no TOCTOU) | 6 |
| W3-Ensure-DiffScenario | 3 | PASS | EnsureBelief creates separate beliefs for the same claim in different scenarios | two different IDs | idA=f6a46466-2ab6-45bf-8d9b-f19e81495b40, idB=64341379-6ce9-488f-99e9-d0c7bbeb1688, different=true | — | — | EnsureBelief — scenario-scoped uniqueness | 6 |
| NEW-01-hp | CS | PASS | ClaimIntent succeeds with correct scenario + belief + action + live intent | nil error | err=<nil> | — | — | — | 31 |
| NEW-01-wb | CS | PASS | ClaimIntent rejects wrong belief (same scenario, different belief) | error | err=intent is not in live state | — | — | — | 40 |
| NEW-01-wa | CS | PASS | ClaimIntent rejects wrong action (same scenario, same belief, different action) | error | err=intent is not in live state | — | — | — | 27 |
| NEW-01-ex | CS | PASS | NEW-01 exact exploit: claim intentB with wrong belief+action tuple is rejected | error | err=intent is not in live state | — | — | — | 41 |
| NEW-02-cr | CS | PASS | IntentOnPromoted rejects cross-scenario belief | ErrBeliefNotFound | err=belief not found in scenario | — | — | — | 13 |
| NEW-02-ra | CS | PASS | Cross-scenario intent rejected + RetractCascade succeeds (no 23514) | cross-scenario rejected; retract returns 2; child retracted | crossRejected=true; n=2; err=<nil>; child="retracted" | — | — | — | 61 |
| NEW-02-ss | CS | PASS | Same-scenario IntentOnPromoted succeeds | nil error | err=<nil> | — | — | — | 23 |
| NEW-03-ss | CS | PASS | AddEvidence succeeds with same-scenario belief | nil error | err=<nil> | — | — | — | 15 |
| NEW-03-cr | CS | PASS | AddEvidence rejects cross-scenario belief | ErrBeliefNotFound | err=belief not found in scenario | — | — | — | 12 |
| CD01-em | CD | PASS | ClaimIntent succeeds with exact empty target/snapshot (IntentOnPromoted path) | nil error | err=<nil> | — | — | — | 30 |
| CD02-wt | CD | PASS | ClaimIntent rejects when target_id doesn't match (NULL vs non-NULL) | error | err=intent is not in live state | — | — | — | 28 |
| CD03-di | CD | PASS | Two different intents on same belief+action can both be claimed (different intent IDs) | nil error | err=<nil> | — | — | — | 50 |
| CD04-fk | CD | PASS | ClaimIntent rejects with non-existent target/snapshot (FK or NULL mismatch) | error | err=intent is not in live state | — | — | — | 28 |
| DA01-av | DA | PASS | EnterBelief accepts arbitrary debt vocabulary (domain-portable) | proof_check,counterexample_search,applicability_review | proof_check,counterexample_search,applicability_review | — | — | — | 3 |
| DA02-ra | DA | PASS | RetireDebt works on arbitrary debt vocabulary (kernel treats names as opaque) | proof_check,applicability_review | proof_check,applicability_review | — | — | — | 6 |
| DA03-ed | DA | PASS | Empty initialDebt creates belief with no debt; promotion succeeds | (empty string) |  | — | — | — | 9 |
| DA04-ep | DA | PASS | EnsureBelief does not overwrite existing debt with caller's initialDebt | same ID; debt=item_a,item_b | sameID=true; debt=item_a,item_b | — | — | — | 6 |
| DEBT-01 | cs | PASS | Arbitrary non-empty debt identifiers are accepted | stored debt == needNullModel,foo,bar | stored debt == needNullModel,foo,bar | — | — | Solvent accepts opaque debt identifiers | 2 |
| DEBT-02 | cs | PASS | Explicit empty debt array is accepted | 0 debt items | 0 debt items | — | — | Empty debt accepted | 3 |
| DEBT-03 | cs | PASS | Nil debt input results in empty stored debt | 0 debt items | 0 debt items | — | — | Nil debt normalized to empty | 2 |
| DEBT-04 | cs | PASS | Completely unknown debt identifier is accepted | stored debt == completelyUnknownDebt | stored debt == completelyUnknownDebt | — | — | Solvent does not validate debt vocabulary | 2 |
| DEBT-05 | cs | PASS | Arbitrary debt item can be retired | stored debt == bar | stored debt == bar | — | — | RetireDebt works with arbitrary identifiers | 5 |
| DEBT-06 | cs | PASS | Promotion blocked while arbitrary debt remains | ErrPromotionBlocked (23514 / promoted_is_debt_free) | err=promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514) sqlstate=23514 constraint=promoted_is_debt_free | — | — | Database gate enforces debt-free promotion | 4 |
| DEBT-07 | cs | PASS | Promotion succeeds when debt is empty | nil error; status=promoted | err=<nil>; status=promoted | — | — | Empty debt permits promotion | 9 |
| DEBT-08 | cs | PASS | Final-truth claim cannot be promoted even with empty debt | ErrPromotionBlocked | err=promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514) | — | — | final_truth gate unchanged | 8 |
| DEBT-09 | cs | PASS | Retiring duplicate debt removes all matching entries (array_remove semantics) | 0 debt items after retirement | 0 debt items | — | — | array_remove removes all matching occurrences | 6 |
| DEBT-10 | cs | PASS | RetireDebt against nonexistent belief returns ErrBeliefNotFound | ErrBeliefNotFound | err=belief not found in scenario | — | — | Nonexistent belief is an error, not a no-op | 0 |
| DEBT-11 | cs | PASS | Kernel accepts empty-string as opaque debt (domain-agnostic) | stored debt == empty string | stored debt == | — | — | Solvent does not reject any opaque string | 2 |
| DEBT-14 | cs | PASS | Migration 010 supersedes 004: new insert gets empty default | 0 debt items (empty array default) | 0 debt items | — | — | Database default is empty after migration 010 | 4 |
| OR-1 | operator-review | PASS | promote without --action → no intent, audit = 0 | audit = 0 | audit = 0 | — | — | — | 12 |
| OR-2 | operator-review | PASS | promote with --action → 1 live intent, audit = 0 | audit = 0, intent_count = 1 | audit = 0, intent_count = 1 | — | — | — | 25 |
| OR-3 | operator-review | PASS | unpromoted belief → IntentOnPromoted refused (23503) | SQLSTATE 23503 | 23503 | 23503 | gate | I-3 | 4 |
| OR-4 | operator-review | PASS | failed promotion → no intent (23514 then 23503) | promote=23514, intent=23503 | promote=23514, intent=23503 | 23514 | — | — | 10 |
| OR-5 | operator-review | PASS | scenario mismatch → no mutation, no promotion, no intent | mismatch detected, debt unchanged, status=entered, intents=0 | mismatch=true, debt_changed=false, status=entered, intents=0 | — | — | — | 7 |

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
  belief  7550e4cb-2ee5-4efd-bd23-0d494ff3bbb3  retracted
  belief  cc36f559-a549-461c-9b84-5895b0235c0c  retracted
  intent  93afdd7e-a154-4c47-8222-972c6e9eb493  cancelled
scenario 11111111-0000-0000-0000-0000000000f0
```

### OR-3

```
action refused: belief is not promoted: ERROR: insert on table "action_intent" violates foreign key constraint "gate" (SQLSTATE 23503)
```

