# M2 Test Transcript

Captured `go test -v` output. Two runs: the second is contract §7 M2's
second pass (A2), which under R2 also proves the suite is repeatable.

## Run 1

```
=== Wave 0 === resetting behavioral test database
    dsn:      postgresql://root@localhost:26260/fable_test?sslmode=disable
    database: fable_test  (DROP + CREATE + apply ../../db/001_schema.sql)
=== RUN   TestW0_SchemaPresent

=== Wave 0 ===
    kernel_test.go:36: W0 PASS — 4 tables present
--- PASS: TestW0_SchemaPresent (0.01s)
=== RUN   TestW1_B01_EnterBelief

=== Wave 1 ===
    kernel_test.go:66: B-01 PASS — id parseable=true, status="entered", final_truth=false, debt items=6
--- PASS: TestW1_B01_EnterBelief (0.01s)
=== RUN   TestW1_B17_DebtEncoding
    kernel_test.go:93: B-17 PASS — stored debt == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B17_DebtEncoding (0.00s)
=== RUN   TestW1_B23_DebtDefaultDrift
    kernel_test.go:122: B-23 PASS — DDL default == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B23_DebtDefaultDrift (0.00s)
=== RUN   TestW1_B02_AddEvidence
    kernel_test.go:157: B-02 PASS — 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true
--- PASS: TestW1_B02_AddEvidence (0.01s)
=== RUN   TestW1_B03_RetireDebt
    kernel_test.go:185: B-03 PASS — debt == needMap,needInvariant,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B03_RetireDebt (0.00s)
=== RUN   TestW1_B04_RetireDebtIdempotent
    kernel_test.go:214: B-04 PASS — second call err=<nil>; debt unchanged=true
--- PASS: TestW1_B04_RetireDebtIdempotent (0.01s)
=== RUN   TestW1_B05_Promote
    kernel_test.go:239: B-05 PASS — err=<nil>; status="promoted"
--- PASS: TestW1_B05_Promote (0.02s)
=== RUN   TestW1_B09_I1_PromoteWithDebt
    kernel_test.go:270: B-09 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B09_I1_PromoteWithDebt (0.00s)
=== RUN   TestW1_B10_I2_PromoteFinalTruth
    kernel_test.go:307: B-10 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B10_I2_PromoteFinalTruth (0.00s)
=== RUN   TestW1_B18_RetryClassification
    kernel_test.go:372: B-18 PASS — injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2)
--- PASS: TestW1_B18_RetryClassification (0.03s)
=== RUN   TestW1_B06_IntentOnPromoted
    kernel_test.go:427: B-06 PASS — err=<nil>; state="live", belief_status="promoted"
--- PASS: TestW1_B06_IntentOnPromoted (0.02s)
=== RUN   TestW1_B11_I3_IntentOnUnpromoted
    kernel_test.go:460: B-11 PASS — err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0
--- PASS: TestW1_B11_I3_IntentOnUnpromoted (0.00s)
=== RUN   TestW1_B12_I4_RetractSkippingCancel
    kernel_test.go:498: B-12 PASS — sqlstate="23514"; constraint="live_requires_promoted"; status="promoted"
--- PASS: TestW1_B12_I4_RetractSkippingCancel (0.03s)
=== RUN   TestW2_B07_RetractCascade

=== Wave 2 ===
    kernel_test.go:536: B-07 PASS — returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted"
--- PASS: TestW2_B07_RetractCascade (0.07s)
=== RUN   TestW2_B20_CascadeChain
    kernel_test.go:562: B-20 PASS — returned 3; unrelated belief="promoted"
--- PASS: TestW2_B20_CascadeChain (0.07s)
=== RUN   TestW2_B22_RowsAffectedSemantics
    kernel_test.go:595: B-22 PASS — returned 2; 3 intents cancelled
--- PASS: TestW2_B22_RowsAffectedSemantics (0.05s)
=== RUN   TestW2_B19_CrossScenarioIsolation
    kernel_test.go:621: B-19 PASS — returned 2; scenario-B belief="promoted"
--- PASS: TestW2_B19_CrossScenarioIsolation (0.05s)
=== RUN   TestW2_B24_B16_BlockedCascadeIsAtomic
    kernel_test.go:663: B-24 PASS — returned 0; sqlstate="23514"; constraint="live_requires_promoted"
    kernel_test.go:691: B-16 PASS — root="promoted" child="promoted"; live intents A=1 B=1; rows changed=0 — rollback verified
--- PASS: TestW2_B24_B16_BlockedCascadeIsAtomic (0.06s)
=== RUN   TestW2_B08_AuditAfterCascade
    kernel_test.go:717: B-08 PASS — returned 0 (err=<nil>)
--- PASS: TestW2_B08_AuditAfterCascade (0.06s)
=== RUN   TestW2_B13_I5_GlobalAudit
    kernel_test.go:739: B-13 PASS — 0 live-on-non-promoted row(s)
--- PASS: TestW2_B13_I5_GlobalAudit (0.00s)
=== RUN   TestW3_B21_CycleTerminates

=== Wave 3 ===
    kernel_test.go:771: B-21 PASS — returned 2 in 5 ms (deadline 30000 ms)
--- PASS: TestW3_B21_CycleTerminates (0.04s)
=== RUN   TestW3_B14_I6_NoEmbeddingColumn
    kernel_test.go:793: B-14 PASS — 0 embedding column(s) on belief
--- PASS: TestW3_B14_I6_NoEmbeddingColumn (0.02s)
=== RUN   TestW3_B15_I7_StaticGate
    kernel_test.go:808: B-15 PASS — NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md
--- PASS: TestW3_B15_I7_StaticGate (0.00s)
PASS
ok  	github.com/PithomLabs/solvent/internal/kernel	1.557s
```

## Run 2

```
=== Wave 0 === resetting behavioral test database
    dsn:      postgresql://root@localhost:26260/fable_test?sslmode=disable
    database: fable_test  (DROP + CREATE + apply ../../db/001_schema.sql)
=== RUN   TestW0_SchemaPresent

=== Wave 0 ===
    kernel_test.go:36: W0 PASS — 4 tables present
--- PASS: TestW0_SchemaPresent (0.01s)
=== RUN   TestW1_B01_EnterBelief

=== Wave 1 ===
    kernel_test.go:66: B-01 PASS — id parseable=true, status="entered", final_truth=false, debt items=6
--- PASS: TestW1_B01_EnterBelief (0.01s)
=== RUN   TestW1_B17_DebtEncoding
    kernel_test.go:93: B-17 PASS — stored debt == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B17_DebtEncoding (0.00s)
=== RUN   TestW1_B23_DebtDefaultDrift
    kernel_test.go:122: B-23 PASS — DDL default == needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B23_DebtDefaultDrift (0.00s)
=== RUN   TestW1_B02_AddEvidence
    kernel_test.go:157: B-02 PASS — 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true
--- PASS: TestW1_B02_AddEvidence (0.01s)
=== RUN   TestW1_B03_RetireDebt
    kernel_test.go:185: B-03 PASS — debt == needMap,needInvariant,needNullModel,needObstruction,needFaithfulnessReview
--- PASS: TestW1_B03_RetireDebt (0.01s)
=== RUN   TestW1_B04_RetireDebtIdempotent
    kernel_test.go:214: B-04 PASS — second call err=<nil>; debt unchanged=true
--- PASS: TestW1_B04_RetireDebtIdempotent (0.01s)
=== RUN   TestW1_B05_Promote
    kernel_test.go:239: B-05 PASS — err=<nil>; status="promoted"
--- PASS: TestW1_B05_Promote (0.02s)
=== RUN   TestW1_B09_I1_PromoteWithDebt
    kernel_test.go:270: B-09 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B09_I1_PromoteWithDebt (0.00s)
=== RUN   TestW1_B10_I2_PromoteFinalTruth
    kernel_test.go:307: B-10 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B10_I2_PromoteFinalTruth (0.00s)
=== RUN   TestW1_B18_RetryClassification
    kernel_test.go:372: B-18 PASS — injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2)
--- PASS: TestW1_B18_RetryClassification (0.03s)
=== RUN   TestW1_B06_IntentOnPromoted
    kernel_test.go:427: B-06 PASS — err=<nil>; state="live", belief_status="promoted"
--- PASS: TestW1_B06_IntentOnPromoted (0.02s)
=== RUN   TestW1_B11_I3_IntentOnUnpromoted
    kernel_test.go:460: B-11 PASS — err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0
--- PASS: TestW1_B11_I3_IntentOnUnpromoted (0.00s)
=== RUN   TestW1_B12_I4_RetractSkippingCancel
    kernel_test.go:498: B-12 PASS — sqlstate="23514"; constraint="live_requires_promoted"; status="promoted"
--- PASS: TestW1_B12_I4_RetractSkippingCancel (0.03s)
=== RUN   TestW2_B07_RetractCascade

=== Wave 2 ===
    kernel_test.go:536: B-07 PASS — returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted"
--- PASS: TestW2_B07_RetractCascade (0.07s)
=== RUN   TestW2_B20_CascadeChain
    kernel_test.go:562: B-20 PASS — returned 3; unrelated belief="promoted"
--- PASS: TestW2_B20_CascadeChain (0.09s)
=== RUN   TestW2_B22_RowsAffectedSemantics
    kernel_test.go:595: B-22 PASS — returned 2; 3 intents cancelled
--- PASS: TestW2_B22_RowsAffectedSemantics (0.06s)
=== RUN   TestW2_B19_CrossScenarioIsolation
    kernel_test.go:621: B-19 PASS — returned 2; scenario-B belief="promoted"
--- PASS: TestW2_B19_CrossScenarioIsolation (0.07s)
=== RUN   TestW2_B24_B16_BlockedCascadeIsAtomic
    kernel_test.go:663: B-24 PASS — returned 0; sqlstate="23514"; constraint="live_requires_promoted"
    kernel_test.go:691: B-16 PASS — root="promoted" child="promoted"; live intents A=1 B=1; rows changed=0 — rollback verified
--- PASS: TestW2_B24_B16_BlockedCascadeIsAtomic (0.07s)
=== RUN   TestW2_B08_AuditAfterCascade
    kernel_test.go:717: B-08 PASS — returned 0 (err=<nil>)
--- PASS: TestW2_B08_AuditAfterCascade (0.07s)
=== RUN   TestW2_B13_I5_GlobalAudit
    kernel_test.go:739: B-13 PASS — 0 live-on-non-promoted row(s)
--- PASS: TestW2_B13_I5_GlobalAudit (0.00s)
=== RUN   TestW3_B21_CycleTerminates

=== Wave 3 ===
    kernel_test.go:771: B-21 PASS — returned 2 in 5 ms (deadline 30000 ms)
--- PASS: TestW3_B21_CycleTerminates (0.05s)
=== RUN   TestW3_B14_I6_NoEmbeddingColumn
    kernel_test.go:793: B-14 PASS — 0 embedding column(s) on belief
--- PASS: TestW3_B14_I6_NoEmbeddingColumn (0.02s)
=== RUN   TestW3_B15_I7_StaticGate
    kernel_test.go:808: B-15 PASS — NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md
--- PASS: TestW3_B15_I7_StaticGate (0.00s)
PASS
ok  	github.com/PithomLabs/solvent/internal/kernel	1.682s
```
