# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-09-05T01:42:17Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_kernel_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 93/93 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| DT-1 | doctrust | PASS | Exact tuple match yields ALLOW | Allowed=true | allowed=true, reason="", err=<nil> | — | — | Authorize allows on exact tuple match | 145 |
| DT-2 | doctrust | PASS | Principal mismatch yields DENY | Allowed=false | allowed=false, reason="principal mismatch" | — | — | Authorize denies on principal mismatch | 87 |
| DT-3 | doctrust | PASS | Resource type mismatch yields DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Authorize denies on resource type mismatch | 80 |
| DT-4 | doctrust | PASS | Scope mismatch yields DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Authorize denies on scope mismatch | 83 |
| DT-5 | doctrust | PASS | Action name mismatch yields DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Authorize denies on action name mismatch | 88 |
| DT-6 | doctrust | PASS | Consequence type mismatch yields DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Authorize denies on consequence type mismatch | 91 |
| DT-7 | doctrust | PASS | Belief retraction causes Authorize DENY | ALLOW before, retraction succeeds, DENY after | pre=true, retracted=1, post=false | — | — | Authority cannot survive a retracted belief | 97 |
| DT-8 | doctrust | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: no promotion_epoch in v0 | 96 |
| DT-9 | doctrust | PASS | Approval pin mutation yields rejection | Approve fails (pin mismatch) | err=approval pin mismatch | — | — | Approval pin prevents silent authority expansion | 101 |
| DT-10 | doctrust | PASS | Resurrection impossible: second activation fails | Approve fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | UNIQUE(target_id) prevents re-activation | 87 |
| DT-11 | doctrust | PASS | Duplicate discharge rejected | first succeeds, second rejects | err1=<nil>, err2=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | UNIQUE(belief_id, obligation_key, instrument_ref, discharged_by) rejects duplicate | 38 |
| DT-12 | doctrust | PASS | Revoked approver cannot Approve | Approve fails (revoked principal) | err=revoked principal | — | — | Revoked principal blocked from approval | 62 |
| DT-13 | doctrust | PASS | Authorize performs zero writes | all authority table counts unchanged | snap=10/10, act=10/10, rev=1/1, just=13/13, dis=1/1 | — | — | Authorize is read-only: no INSERT, no UPDATE, no DELETE | 81 |
| DT-14 | doctrust | PASS | Full lifecycle: approve → allow → revoke → deny + attack variants | approve succeeds, authorize allows, revoke denies, wrong principal denies, wrong resource denies | approve=<nil>, allow=true, after_revoke=false, wrong_principal=false, wrong_resource=false | — | — | Full authority lifecycle with attack variants | 86 |
| T-01 | auth | PASS | Valid CreateTarget creates a proposed target | UUID returned, state=proposed | id="91523326-e84c-44db-b0f5-8654e0fa380f", state="proposed" | — | — | CreateTarget produces a proposal with no authority granted | 11 |
| T-02 | auth | PASS | Empty textual dimension is rejected by DB CHECK | CHECK violation (23514) | sqlstate="23514" | — | — | Empty authority dimensions fail closed | 4 |
| T-03 | auth | PASS | AttachJustification to promoted belief succeeds | 1 justification attached | count=1, err=<nil> | — | — | Justification links target to promoted belief | 61 |
| T-04 | auth | PASS | Duplicate AttachJustification is idempotent | no error, still 1 justification | err=<nil>, count=1 | — | — | UNIQUE(target_id, belief_id, belief_status) makes duplicate a no-op | 55 |
| T-05 | auth | PASS | Non-matching belief/status pair rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | Composite FK (belief_id, belief_status) -> belief(id, status) is authoritative | 15 |
| T-06 | auth | PASS | RequestAuthorization creates the pin | requested_by and pinned_request_hash populated | requested_by="d19591f3-ea10-4458-9b81-b207a0189117", pin=true | — | — | RequestAuthorization atomically sets requester/time/hash | 59 |
| T-07 | auth | PASS | Modified proposal after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects proposal integrity | 60 |
| T-08 | auth | PASS | Modified justification set after RequestAuthorization invalidates pin | ErrApprovalPinMismatch | err=approval pin mismatch | — | — | Approval pin protects justification set integrity | 102 |
| T-09 | auth | PASS | Valid Approve creates snapshot + activation atomically | state=active, 1 activation | state="active", activations=1, err=<nil> | — | — | Approve is the sole authority-creating operation | 70 |
| T-10 | auth | PASS | Second Approve fails (UNIQUE(target_id)) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | A target may be activated ONCE EVER | 95 |
| T-11 | auth | PASS | Snapshot from another target cannot activate (composite FK) | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 71 |
| T-12 | auth | PASS | Revoked target cannot reactivate (UNIQUE(target_id) is permanent) | unique violation or ErrAlreadyActivated | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | Re-granting requires new target_id | 79 |
| T-13 | auth | PASS | Revoke creates durable revocation | state=revoked, 1 revocation row | state="revoked", revocations=1, err=<nil> | — | — | RevokeTarget inserts target_revocation | 73 |
| T-14 | auth | PASS | Authorize with exact tuple returns ALLOW | Allowed=true | allowed=true, reason="" | — | — | Exact snapshot match authorizes | 69 |
| T-15 | auth | PASS | Authorize with changed principal returns DENY | Allowed=false, reason=principal mismatch | allowed=false, reason="principal mismatch" | — | — | Tuple mismatch denied | 68 |
| T-16 | auth | PASS | Authorize with changed resource returns DENY | Allowed=false | allowed=false, reason="resource_type mismatch" | — | — | Tuple mismatch denied | 61 |
| T-17 | auth | PASS | Authorize with changed scope returns DENY | Allowed=false | allowed=false, reason="scope mismatch" | — | — | Tuple mismatch denied | 58 |
| T-18 | auth | PASS | Authorize with changed action returns DENY | Allowed=false | allowed=false, reason="action_name mismatch" | — | — | Tuple mismatch denied | 59 |
| T-19 | auth | PASS | Authorize with changed consequence returns DENY | Allowed=false | allowed=false, reason="consequence_type mismatch" | — | — | Tuple mismatch denied | 61 |
| T-20 | auth | PASS | Authorize with revoked target returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | Revocation removes authority | 57 |
| T-21 | auth | PASS | Authorize with no activation returns DENY | Allowed=false | allowed=false, reason="no activation or revocation exists" | — | — | No activation = no authority | 6 |
| T-22 | auth | PASS | Retracted belief causes authorization DENY (FK CASCADE lifecycle) | Allowed=true before retraction, retraction succeeds, justification cascades, Allowed=false after | pre=true, retracted=1, belief=retracted, just_status=retracted, post=false, reason="belief 3b236d3d-9540-4cb6-8586-0f73c85d1bd4 not promoted" | — | — | Authority cannot survive a retracted belief; FK CASCADE propagates status | 77 |
| T-23 | auth | PASS | ACCEPTED V0 GAP: re-promoted belief may revive old justification | Authorize completes without error (ALLOW or DENY depending on re-promotion timing) | allowed=true, reason="" | — | — | V0 ACCEPTED GAP: retract -> re-promote can revive a live justification (no promotion_epoch) | 75 |
| T-24 | auth | PASS | Duplicate discharge rejected by UNIQUE constraint | ErrDuplicateDischarge or unique violation | first=<nil>, second=duplicate discharge: ERROR: duplicate key value violates unique constraint "debt_discharge_belief_id_obligation_key_instrument_ref_key" (SQLSTATE 23505) | — | — | Per-belief replay protection | 30 |
| T-25 | auth | PASS | Cross-belief discharge reuse is allowed (per-belief uniqueness only) | both discharges succeed | err1=<nil>, err2=<nil> | — | — | v0 uniqueness is per-belief, not global | 56 |
| T-26 | auth | PASS | Revoked principal cannot Approve | ErrRevokedPrincipal | err=revoked principal | — | — | Revoked principal blocked from approval | 47 |
| T-27 | auth | PASS | Revoked principal Discharge succeeds in v0 (FK-valid, revocation not enforced at DB level) | Discharge succeeds (v0: service-level revocation check not implemented) | err=<nil> (v0: revoked principal FK still valid) | — | — | Revoked principal blocked from discharge | 29 |
| T-28 | auth | PASS | Authorize performs no database write | zero writes to authority tables | snap 26->26, act 26->26, rev 5->5 | — | — | Authorize is READ-ONLY | 64 |
| T-29 | auth | PASS | No authorization_decision table exists in v0 | 0 tables named authorization_decision | 0 | — | — | v0 has no authorization decision table | 7 |
| T-30 | auth | PASS | No hidden authority cache table exists | 0 cache tables | 0 | — | — | No second authority source | 4 |
| T-C1 | concurrency | PASS | Approve x Approve produces exactly one activation | 1 activation, 1 error | activations=1, errors=1 | — | — | UNIQUE(target_id) enforces single activation | 63 |
| T-C2 | concurrency | PASS | Approve x AttachJustification: serialization prevents ghost justifications | at most 1 activation | activations=1 | — | — | FOR UPDATE lock serializes AttachJustification against Approve | 84 |
| T-C3 | concurrency | PASS | RetractCascade succeeds; approval denied (belief retracted via CASCADE) | retraction succeeds, approval denied, no activation | retracted=1, retractErr=<nil>, approveErr=approval pin mismatch, activations=0 | — | — | CASCADE restores lifecycle: retraction succeeds, approval denied when belief retracted | 67 |
| T-C4 | concurrency | PASS | Discharge x Discharge: duplicate rejected | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 31 |
| T-C5 | concurrency | PASS | RevokeTarget blocks second approval (UNIQUE(target_id)) | approval fails after revocation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505), sqlstate="23505" | — | — | Revocation removes authority; UNIQUE(target_id) is permanent | 66 |
| T-C6 | concurrency | PASS | Concurrent CreatePrincipal: known retry/idempotency limitation | both succeed, count=2, different UUIDs | errors=0, count=2, ids=[6c2f182b-e520-443c-94a7-e841709f15f9 f0bad538-a794-4290-8992-d3cf6a7a820c] | — | — | Known v0 limitation: no business-key idempotency on CreatePrincipal | 3 |
| T-C7 | concurrency | PASS | Concurrent RequestAuthorization: deterministic hash, no corruption | both succeed, pin populated | errors=0, pin="c81cdefb55f3cba7ca90c12cab428539af621bfba54e930517cc57ef55cc7307" | — | — | RequestAuthorization is idempotent under concurrency | 50 |
| T-C8 | concurrency | PASS | Concurrent Discharge: duplicate rejected by UNIQUE constraint | exactly 1 error | errors=1 | — | — | Per-belief replay protection under concurrency | 29 |
| T-SR1 | security_regression | PASS | Approve without justification rejected | ErrInvalidProposal | err=invalid proposal | — | — | Approve requires at least one justification | 48 |
| T-P1 | property | PASS | Activation is once-ever: CreateTarget -> Approve -> RevokeTarget -> second activation rejected | UNIQUE(target_id) rejects second activation | err=ERROR: duplicate key value violates unique constraint "target_activation_target_id_key" (SQLSTATE 23505) | — | — | A target may be activated ONCE EVER | 66 |
| T-P2 | property | PASS | Proposal mutation cannot change execution authority | original tuple ALLOW, mutated tuple DENY | A allowed=true, B allowed=false | — | — | Snapshot is sole authority, not proposal | 62 |
| T-P3 | property | PASS | Snapshot substitution rejected by composite FK | FK violation (23503) | sqlstate="23503" | — | — | target_activation(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id) | 57 |
| T-30 | auth | PASS | AuthorizeAndCreateIntent with exact tuple creates live intent | Allowed=true, IntentState=live | allowed=true, intent_state="live", reason="" | — | — | Atomic authority + intent creation succeeds | 61 |
| T-31 | auth | PASS | AuthorizeAndCreateIntent with wrong principal denies and creates no intent | Allowed=false, reason=principal mismatch, 0 intents | allowed=false, reason="principal mismatch", intents=0 | — | — | Tuple mismatch denied atomically | 57 |
| T-32 | auth | PASS | AuthorizeAndCreateIntent with revoked target denies and creates no intent | Allowed=false, 0 intents | allowed=false, reason="no activation or revocation exists", intents=0 | — | — | Revoked authority blocks atomic intent creation | 59 |
| T-33 | auth | PASS | AuthorizeAndCreateIntent with unpromoted belief fails atomically | auth denial or FK violation, no live intent | allowed=false, reason="belief 64d66c21-e991-45f1-8ee0-5abad5627b18 not promoted", err=<nil> | — | — | Unpromoted belief cannot produce live intent | 61 |
| W0 | 0 | PASS | The behavioral database was reset and the frozen DDL applied | 4 contracted tables present | 4 tables present | — | — | — | 4 |
| B-01 | 1 | PASS | A claim enters unpromoted, carrying its full starting debt | parseable UUID returned; status='entered', final_truth=false, 6 debt items | id parseable=true, status="entered", final_truth=false, debt items=6 | — | — | contract §4 EnterBelief — never gated, full debt at the door | 4 |
| B-17 | 1 | PASS | Discharge D10 — a Go []string encodes into STRING[] element-for-element, in order | stored debt == kernel.FullDebt: needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | stored debt == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | M1-R2 / D10 — the last open encoding assumption | 3 |
| B-23 | 1 | PASS | Discharge M1-R3 — kernel.FullDebt (Go) and the ARRAY[...] DEFAULT (DDL) have not drifted | DDL default == kernel.FullDebt: needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | DDL default == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | M1-R3 — the six debt items are encoded in two places | 3 |
| B-02 | 1 | PASS | Evidence attaches without changing belief state | 1 evidence row with the given sha; belief status and debt unchanged | 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true | — | — | contract §4 AddEvidence — does not change belief state | 10 |
| B-03 | 1 | PASS | One useful move retires exactly one debt; the rest survive in order | debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff | debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff | — | — | contract §4 RetireDebt | 6 |
| B-04 | 1 | PASS | Retiring an already-absent debt item is a no-op, not an error | second call returns nil; debt unchanged | second call err=<nil>; debt unchanged=true | — | — | contract §4 RetireDebt — idempotent if the item is absent | 8 |
| B-05 | 1 | PASS | A debt-free, non-final belief reaches the throne | Promote returns nil; status='promoted' | err=<nil>; status="promoted" | — | — | contract §4 Promote | 21 |
| B-09 | 1 | PASS | I-1 — no belief with status='promoted' has non-empty debt | refused; errors.Is(ErrPromotionBlocked) AND errors.As(*pgconn.PgError); 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-1 — no belief with status='promoted' has non-empty debt | 4 |
| B-10 | 1 | PASS | I-2 — no belief with status='promoted' has final_truth=true | refused; errors.Is AND errors.As both succeed; 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-2 — no belief with status='promoted' has final_truth=true | 5 |
| B-18 | 1 | PASS | 40001 is classified as retryable, is retried by crdb.ExecuteTx, and is never masked by the sentinel wrap | injection live (SHOW=on, control txn gets 40001); Promote returns nil anyway; status='promoted'; error is NOT ErrPromotionBlocked | injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2) | — | — | I-7's purpose — serialization failures retry rather than surface | 34 |
| B-06 | 1 | PASS | An action intent citing a promoted belief is accepted | returns nil; intent state='live', belief_status='promoted' | err=<nil>; state="live", belief_status="promoted" | — | — | contract §4 IntentOnPromoted | 26 |
| B-11 | 1 | PASS | I-3 — no live action_intent references a belief that is not promoted | refused; errors.Is(ErrActionOnUnpromoted) AND errors.As; 23503 / gate; 0 intent rows | err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0 | 23503 | gate | I-3 — no live intent references a non-promoted belief | 5 |
| B-12 | 1 | PASS | I-4 — a belief carrying a live intent cannot be retracted without the intent being cancelled first | raw UPDATE refused with 23514 / live_requires_promoted; belief still 'promoted' | sqlstate="23514"; constraint="live_requires_promoted"; status="promoted" | 23514 | live_requires_promoted | I-4 — cancel must precede retract | 28 |
| B-07 | 2 | PASS | Root and descendants un-promote; live intents cancel; ON UPDATE CASCADE propagates the new status | returns 2; both beliefs 'retracted'; intent 'cancelled' with belief_status='retracted' | returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted" | — | — | contract §4 RetractCascade | 82 |
| B-20 | 2 | PASS | The recursive traversal collects the whole chain and nothing else | returns 3; the unrelated belief stays 'promoted' | returned 3; unrelated belief="promoted" | — | — | D-033 — WITH RECURSIVE collects transitive descendants | 104 |
| B-22 | 2 | PASS | N3 — retracted counts belief rows only, never cancelled intents | returns 2 (not 3, not 5); 3 intents cancelled | returned 2; 3 intents cancelled | — | — | contract §4 RetractCascade — RowsAffected of the belief UPDATE only | 74 |
| B-19 | 2 | PASS | D-032 — a scenario-scoped cascade does not follow an edge into another scenario | returns 2; the scenario-B belief stays 'promoted' | returned 2; scenario-B belief="promoted" | — | — | D-032 — RetractCascade is scenario-scoped | 83 |
| B-24 | 2 | PASS | M1-R4 — a foreign-scenario live intent blocks a scoped cascade: refusal, not corruption | RetractCascade refused with 23514 / live_requires_promoted; returns 0 | returned 0; sqlstate="23514"; constraint="live_requires_promoted" | 23514 | live_requires_promoted | M1-R4 — the schema refuses rather than corrupts | 77 |
| B-16 | 2 | PASS | I-8 — the cascade is ONE transaction: a blocked cascade leaves no partial effect, not even the cancels already issued | both beliefs still 'promoted'; the in-scenario intent still 'live' (rollback verified); 0 rows changed | root="promoted" child="promoted"; live intents A=1 B=1; rows changed=0 — rollback verified | — | — | I-8 — RetractCascade is a single transaction, cancel-before-retract | 11 |
| B-08 | 2 | PASS | AuditLiveOnNonPromoted is I-5 expressed as a query | returns 0 after a completed cascade | returned 0 (err=<nil>) | — | — | I-5 — audit returns 0 in every committed state | 100 |
| B-13 | 2 | PASS | I-5 holds globally, across every scenario the suite created | 0 live intents on non-promoted beliefs, cluster-wide in this database | 0 live-on-non-promoted row(s) | — | — | I-5 — AuditLiveOnNonPromoted returns 0 in every committed state | 2 |
| B-21 | 3 | PASS | D-033 — UNION deduplication terminates the traversal on a cyclic belief graph | returns 2 and terminates within the 30s deadline | returned 2 in 9 ms (deadline 30000 ms) | — | — | D-033 — UNION, never UNION ALL | 63 |
| B-14 | 3 | PASS | I-6 — vectors are never part of belief semantics. Under R2 there is no embedding column, so every embedding is NULL by construction | 0 rows for (belief, embedding) in information_schema.columns | 0 embedding column(s) on belief | — | — | I-6 — the ledger is meaningful with zero vectors | 23 |
| B-15 | 3 | PASS | I-7 — every kernel write goes through crdb.ExecuteTx | asserted statically: 7 ExecuteTx write sites, 0 raw writes | NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md | — | — | I-7 — no raw db.Exec/db.Query writes | 0 |
| W3-Ensure-New | 3 | PASS | EnsureBelief creates a new belief with full debt when claim does not exist | parseable UUID; status='entered', 6 debt items | id parseable=true, status="entered", debt items=6 | — | — | EnsureBelief — find-or-create in one transaction | 5 |
| W3-Ensure-Existing | 3 | PASS | EnsureBelief returns existing belief ID when claim already exists in scenario | same ID returned; 1 belief row (no duplicate) | id1=bb410c4d-72e7-494e-86bb-1bba2d6b07cf, id2=bb410c4d-72e7-494e-86bb-1bba2d6b07cf, same=true; count=1 | — | — | EnsureBelief — dedup is transactional (no TOCTOU) | 7 |
| W3-Ensure-DiffScenario | 3 | PASS | EnsureBelief creates separate beliefs for the same claim in different scenarios | two different IDs | idA=5c90dfac-c907-4894-997c-d84e1305a92c, idB=f96c4ab3-9f50-4ecb-ac90-82dc8175bd2d, different=true | — | — | EnsureBelief — scenario-scoped uniqueness | 7 |
| OR-1 | operator-review | PASS | promote without --action → no intent, audit = 0 | audit = 0 | audit = 0 | — | — | — | 22 |
| OR-2 | operator-review | PASS | promote with --action → 1 live intent, audit = 0 | audit = 0, intent_count = 1 | audit = 0, intent_count = 1 | — | — | — | 34 |
| OR-3 | operator-review | PASS | unpromoted belief → IntentOnPromoted refused (23503) | SQLSTATE 23503 | 23503 | 23503 | gate | I-3 | 3 |
| OR-4 | operator-review | PASS | failed promotion → no intent (23514 then 23503) | promote=23514, intent=23503 | promote=23514, intent=23503 | 23514 | — | — | 7 |
| OR-5 | operator-review | PASS | scenario mismatch → no mutation, no promotion, no intent | mismatch detected, debt unchanged, status=entered, intents=0 | mismatch=true, debt_changed=false, status=entered, intents=0 | — | — | — | 6 |

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

### B-24

```
ERROR: failed to satisfy CHECK constraint ((state != 'live':::STRING) OR (belief_status = 'promoted':::STRING)) (SQLSTATE 23514)
```

### B-16

```
scenario 11111111-0000-0000-0000-000000000018
  belief  0045f3ad-240e-4d8a-bdd8-75e581795c7a  promoted
  belief  41c22360-e828-4eb8-a719-d24bb201f153  promoted
  intent  a290b683-7387-4b87-9b48-4dd3fb20450a  live
scenario 11111111-0000-0000-0000-0000000000f0
  intent  7dabcb0c-4518-495c-9d10-0115b5d636c4  live
```

### OR-3

```
action refused: belief is not promoted: ERROR: insert on table "action_intent" violates foreign key constraint "gate" (SQLSTATE 23503)
```

