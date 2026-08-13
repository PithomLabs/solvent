# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-08-13T19:31:37Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 33/33 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| W0 | 0 | PASS | The behavioral database was reset and the frozen DDL applied | 4 contracted tables present | 4 tables present | — | — | — | 10 |
| B-01 | 1 | PASS | A claim enters unpromoted, carrying its full starting debt | parseable UUID returned; status='entered', final_truth=false, 6 debt items | id parseable=true, status="entered", final_truth=false, debt items=6 | — | — | contract §4 EnterBelief — never gated, full debt at the door | 5 |
| B-17 | 1 | PASS | Discharge D10 — a Go []string encodes into STRING[] element-for-element, in order | stored debt == kernel.FullDebt: needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview | stored debt == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview | — | — | M1-R2 / D10 — the last open encoding assumption | 3 |
| B-23 | 1 | PASS | Discharge M1-R3 — kernel.FullDebt (Go) and the ARRAY[...] DEFAULT (DDL) have not drifted | DDL default == kernel.FullDebt: needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview | DDL default == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview | — | — | M1-R3 — the six debt items are encoded in two places | 2 |
| B-02 | 1 | PASS | Evidence attaches without changing belief state | 1 evidence row with the given sha; belief status and debt unchanged | 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true | — | — | contract §4 AddEvidence — does not change belief state | 8 |
| B-03 | 1 | PASS | One useful move retires exactly one debt; the rest survive in order | debt == needMap,needInvariant,needNullModel,needObstruction,needFaithfulnessReview | debt == needMap,needInvariant,needNullModel,needObstruction,needFaithfulnessReview | — | — | contract §4 RetireDebt | 4 |
| B-04 | 1 | PASS | Retiring an already-absent debt item is a no-op, not an error | second call returns nil; debt unchanged | second call err=<nil>; debt unchanged=true | — | — | contract §4 RetireDebt — idempotent if the item is absent | 6 |
| B-05 | 1 | PASS | A debt-free, non-final belief reaches the throne | Promote returns nil; status='promoted' | err=<nil>; status="promoted" | — | — | contract §4 Promote | 18 |
| B-09 | 1 | PASS | I-1 — no belief with status='promoted' has non-empty debt | refused; errors.Is(ErrPromotionBlocked) AND errors.As(*pgconn.PgError); 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-1 — no belief with status='promoted' has non-empty debt | 3 |
| B-10 | 1 | PASS | I-2 — no belief with status='promoted' has final_truth=true | refused; errors.Is AND errors.As both succeed; 23514 / promoted_is_debt_free; belief still 'entered' | errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered" | 23514 | promoted_is_debt_free | I-2 — no belief with status='promoted' has final_truth=true | 3 |
| B-18 | 1 | PASS | 40001 is classified as retryable, is retried by crdb.ExecuteTx, and is never masked by the sentinel wrap | injection live (SHOW=on, control txn gets 40001); Promote returns nil anyway; status='promoted'; error is NOT ErrPromotionBlocked | injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2) | — | — | I-7's purpose — serialization failures retry rather than surface | 29 |
| B-06 | 1 | PASS | An action intent citing a promoted belief is accepted | returns nil; intent state='live', belief_status='promoted' | err=<nil>; state="live", belief_status="promoted" | — | — | contract §4 IntentOnPromoted | 21 |
| B-11 | 1 | PASS | I-3 — no live action_intent references a belief that is not promoted | refused; errors.Is(ErrActionOnUnpromoted) AND errors.As; 23503 / gate; 0 intent rows | err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0 | 23503 | gate | I-3 — no live intent references a non-promoted belief | 3 |
| B-12 | 1 | PASS | I-4 — a belief carrying a live intent cannot be retracted without the intent being cancelled first | raw UPDATE refused with 23514 / live_requires_promoted; belief still 'promoted' | sqlstate="23514"; constraint="live_requires_promoted"; status="promoted" | 23514 | live_requires_promoted | I-4 — cancel must precede retract | 26 |
| B-07 | 2 | PASS | Root and descendants un-promote; live intents cancel; ON UPDATE CASCADE propagates the new status | returns 2; both beliefs 'retracted'; intent 'cancelled' with belief_status='retracted' | returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted" | — | — | contract §4 RetractCascade | 68 |
| B-20 | 2 | PASS | The recursive traversal collects the whole chain and nothing else | returns 3; the unrelated belief stays 'promoted' | returned 3; unrelated belief="promoted" | — | — | D-033 — WITH RECURSIVE collects transitive descendants | 92 |
| B-22 | 2 | PASS | N3 — retracted counts belief rows only, never cancelled intents | returns 2 (not 3, not 5); 3 intents cancelled | returned 2; 3 intents cancelled | — | — | contract §4 RetractCascade — RowsAffected of the belief UPDATE only | 61 |
| B-19 | 2 | PASS | D-032 — a scenario-scoped cascade does not follow an edge into another scenario | returns 2; the scenario-B belief stays 'promoted' | returned 2; scenario-B belief="promoted" | — | — | D-032 — RetractCascade is scenario-scoped | 64 |
| B-24 | 2 | PASS | M1-R4 — a foreign-scenario live intent blocks a scoped cascade: refusal, not corruption | RetractCascade refused with 23514 / live_requires_promoted; returns 0 | returned 0; sqlstate="23514"; constraint="live_requires_promoted" | 23514 | live_requires_promoted | M1-R4 — the schema refuses rather than corrupts | 57 |
| B-16 | 2 | PASS | I-8 — the cascade is ONE transaction: a blocked cascade leaves no partial effect, not even the cancels already issued | both beliefs still 'promoted'; the in-scenario intent still 'live' (rollback verified); 0 rows changed | root="promoted" child="promoted"; live intents A=1 B=1; rows changed=0 — rollback verified | — | — | I-8 — RetractCascade is a single transaction, cancel-before-retract | 9 |
| B-08 | 2 | PASS | AuditLiveOnNonPromoted is I-5 expressed as a query | returns 0 after a completed cascade | returned 0 (err=<nil>) | — | — | I-5 — audit returns 0 in every committed state | 61 |
| B-13 | 2 | PASS | I-5 holds globally, across every scenario the suite created | 0 live intents on non-promoted beliefs, cluster-wide in this database | 0 live-on-non-promoted row(s) | — | — | I-5 — AuditLiveOnNonPromoted returns 0 in every committed state | 1 |
| B-21 | 3 | PASS | D-033 — UNION deduplication terminates the traversal on a cyclic belief graph | returns 2 and terminates within the 30s deadline | returned 2 in 4 ms (deadline 30000 ms) | — | — | D-033 — UNION, never UNION ALL | 44 |
| B-14 | 3 | PASS | I-6 — vectors are never part of belief semantics. Under R2 there is no embedding column, so every embedding is NULL by construction | 0 rows for (belief, embedding) in information_schema.columns | 0 embedding column(s) on belief | — | — | I-6 — the ledger is meaningful with zero vectors | 14 |
| B-15 | 3 | PASS | I-7 — every kernel write goes through crdb.ExecuteTx | asserted statically: 7 ExecuteTx write sites, 0 raw writes | NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md | — | — | I-7 — no raw db.Exec/db.Query writes | 0 |
| W3-Ensure-New | 3 | PASS | EnsureBelief creates a new belief with full debt when claim does not exist | parseable UUID; status='entered', 6 debt items | id parseable=true, status="entered", debt items=6 | — | — | EnsureBelief — find-or-create in one transaction | 4 |
| W3-Ensure-Existing | 3 | PASS | EnsureBelief returns existing belief ID when claim already exists in scenario | same ID returned; 1 belief row (no duplicate) | id1=2d435dd9-d788-4550-918e-3b05cc9639f6, id2=2d435dd9-d788-4550-918e-3b05cc9639f6, same=true; count=1 | — | — | EnsureBelief — dedup is transactional (no TOCTOU) | 4 |
| W3-Ensure-DiffScenario | 3 | PASS | EnsureBelief creates separate beliefs for the same claim in different scenarios | two different IDs | idA=96e6b334-067a-4ae4-a792-aae41baad18a, idB=fd18305e-18b9-42e3-8098-4f7390d067b6, different=true | — | — | EnsureBelief — scenario-scoped uniqueness | 4 |
| OR-1 | operator-review | PASS | promote without --action → no intent, audit = 0 | audit = 0 | audit = 0 | — | — | — | 17 |
| OR-2 | operator-review | PASS | promote with --action → 1 live intent, audit = 0 | audit = 0, intent_count = 1 | audit = 0, intent_count = 1 | — | — | — | 28 |
| OR-3 | operator-review | PASS | unpromoted belief → IntentOnPromoted refused (23503) | SQLSTATE 23503 | 23503 | 23503 | gate | I-3 | 2 |
| OR-4 | operator-review | PASS | failed promotion → no intent (23514 then 23503) | promote=23514, intent=23503 | promote=23514, intent=23503 | 23514 | — | — | 6 |
| OR-5 | operator-review | PASS | scenario mismatch → no mutation, no promotion, no intent | mismatch detected, debt unchanged, status=entered, intents=0 | mismatch=true, debt_changed=false, status=entered, intents=0 | — | — | — | 6 |

## Receipts

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
  belief  34e98f8c-33cf-46f9-ac8e-d299068d9903  promoted
  belief  56bb27fe-5527-4df4-abec-eba61af0a5ea  promoted
  intent  7f758b7b-c19f-4756-9a0b-cbde4b18dd5a  live
scenario 11111111-0000-0000-0000-0000000000f0
  intent  c09bcd38-9f97-4969-80e2-596b61f7ab61  live
```

### OR-3

```
action refused: belief is not promoted: ERROR: insert on table "action_intent" violates foreign key constraint "gate" (SQLSTATE 23503)
```

