# M2 Test Transcript

Captured `go test -v` output. Two runs: the second is contract §7 M2's
second pass (A2), which under R2 also proves the suite is repeatable.

## Run 1

```
=== Wave 0 === resetting behavioral test database
    dsn:      postgresql://root@localhost:26260/fable_test?sslmode=disable
    database: fable_test  (DROP + CREATE + apply ../db/001_schema.sql, ../db/002_corpus.sql, ../db/003_wizard.sql, ../db/004_debt_vocabulary.sql)
=== RUN   TestW0_SchemaPresent

=== Wave 0 ===
    kernel_test.go:36: W0 PASS — 4 tables present
--- PASS: TestW0_SchemaPresent (0.02s)
=== RUN   TestW1_B01_EnterBelief

=== Wave 1 ===
    kernel_test.go:66: B-01 PASS — id parseable=true, status="entered", final_truth=false, debt items=6
--- PASS: TestW1_B01_EnterBelief (0.01s)
=== RUN   TestW1_B17_DebtEncoding
    kernel_test.go:93: B-17 PASS — stored debt == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B17_DebtEncoding (0.00s)
=== RUN   TestW1_B23_DebtDefaultDrift
    kernel_test.go:122: B-23 PASS — DDL default == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B23_DebtDefaultDrift (0.00s)
=== RUN   TestW1_B02_AddEvidence
    kernel_test.go:157: B-02 PASS — 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true
--- PASS: TestW1_B02_AddEvidence (0.01s)
=== RUN   TestW1_B03_RetireDebt
    kernel_test.go:185: B-03 PASS — debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B03_RetireDebt (0.01s)
=== RUN   TestW1_B04_RetireDebtIdempotent
    kernel_test.go:214: B-04 PASS — second call err=<nil>; debt unchanged=true
--- PASS: TestW1_B04_RetireDebtIdempotent (0.01s)
=== RUN   TestW1_B05_Promote
    kernel_test.go:239: B-05 PASS — err=<nil>; status="promoted"
--- PASS: TestW1_B05_Promote (0.02s)
=== RUN   TestW1_B09_I1_PromoteWithDebt
    kernel_test.go:270: B-09 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B09_I1_PromoteWithDebt (0.01s)
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
--- PASS: TestW2_B19_CrossScenarioIsolation (0.08s)
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
    kernel_test.go:771: B-21 PASS — returned 2 in 6 ms (deadline 30000 ms)
--- PASS: TestW3_B21_CycleTerminates (0.05s)
=== RUN   TestW3_B14_I6_NoEmbeddingColumn
    kernel_test.go:793: B-14 PASS — 0 embedding column(s) on belief
--- PASS: TestW3_B14_I6_NoEmbeddingColumn (0.02s)
=== RUN   TestW3_B15_I7_StaticGate
    kernel_test.go:808: B-15 PASS — NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md
--- PASS: TestW3_B15_I7_StaticGate (0.00s)
=== RUN   TestW3_EnsureBelief_New
    kernel_test.go:836: W3-Ensure-New PASS — id parseable=true, status="entered", debt items=6
--- PASS: TestW3_EnsureBelief_New (0.01s)
=== RUN   TestW3_EnsureBelief_Existing
    kernel_test.go:865: W3-Ensure-Existing PASS — id1=f6c56762-bdf0-4242-9af0-870420ba5a9b, id2=f6c56762-bdf0-4242-9af0-870420ba5a9b, same=true; count=1
--- PASS: TestW3_EnsureBelief_Existing (0.01s)
=== RUN   TestW3_EnsureBelief_DifferentScenario
    kernel_test.go:889: W3-Ensure-DiffScenario PASS — idA=8c65b1f5-25cb-4b77-b721-b0600bb29e5d, idB=e2b56f7d-fb26-4d8c-9e33-97be9e9d31e6, different=true
--- PASS: TestW3_EnsureBelief_DifferentScenario (0.01s)
=== RUN   TestOperatorReview_PromoteWithoutAction

=== Wave operator-review ===
    operator_review_test.go:42: OR-1 PASS — audit = 0
--- PASS: TestOperatorReview_PromoteWithoutAction (0.02s)
=== RUN   TestOperatorReview_PromoteWithAction
    operator_review_test.go:97: OR-2 PASS — audit = 0, intent_count = 1
--- PASS: TestOperatorReview_PromoteWithAction (0.03s)
=== RUN   TestOperatorReview_UnpromotedBelief
    operator_review_test.go:131: OR-3 PASS — 23503
--- PASS: TestOperatorReview_UnpromotedBelief (0.00s)
=== RUN   TestOperatorReview_FailedPromotion
    operator_review_test.go:184: OR-4 PASS — promote=23514, intent=23503
--- PASS: TestOperatorReview_FailedPromotion (0.01s)
=== RUN   TestOperatorReview_ScenarioGuard_RejectsMismatch
    operator_review_test.go:258: OR-5 PASS — mismatch=true, debt_changed=false, status=entered, intents=0
--- PASS: TestOperatorReview_ScenarioGuard_RejectsMismatch (0.01s)
=== RUN   Example_lifecycle
--- PASS: Example_lifecycle (0.05s)
PASS
ok  	github.com/PithomLabs/solvent/kernel	3.032s
```

## Run 2

```
=== Wave 0 === resetting behavioral test database
    dsn:      postgresql://root@localhost:26260/fable_test?sslmode=disable
    database: fable_test  (DROP + CREATE + apply ../db/001_schema.sql, ../db/002_corpus.sql, ../db/003_wizard.sql, ../db/004_debt_vocabulary.sql)
=== RUN   TestW0_SchemaPresent

=== Wave 0 ===
    kernel_test.go:36: W0 PASS — 4 tables present
--- PASS: TestW0_SchemaPresent (0.01s)
=== RUN   TestW1_B01_EnterBelief

=== Wave 1 ===
    kernel_test.go:66: B-01 PASS — id parseable=true, status="entered", final_truth=false, debt items=6
--- PASS: TestW1_B01_EnterBelief (0.01s)
=== RUN   TestW1_B17_DebtEncoding
    kernel_test.go:93: B-17 PASS — stored debt == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B17_DebtEncoding (0.00s)
=== RUN   TestW1_B23_DebtDefaultDrift
    kernel_test.go:122: B-23 PASS — DDL default == needProvenanceCheck,needContradictionSweep,needBlastRadius,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B23_DebtDefaultDrift (0.00s)
=== RUN   TestW1_B02_AddEvidence
    kernel_test.go:157: B-02 PASS — 1 evidence row(s), sha="deadbeefcafe"; status "entered"→"entered", debt unchanged=true
--- PASS: TestW1_B02_AddEvidence (0.01s)
=== RUN   TestW1_B03_RetireDebt
    kernel_test.go:185: B-03 PASS — debt == needProvenanceCheck,needContradictionSweep,needRollbackPlan,needVersionPin,needOperatorSignoff
--- PASS: TestW1_B03_RetireDebt (0.01s)
=== RUN   TestW1_B04_RetireDebtIdempotent
    kernel_test.go:214: B-04 PASS — second call err=<nil>; debt unchanged=true
--- PASS: TestW1_B04_RetireDebtIdempotent (0.01s)
=== RUN   TestW1_B05_Promote
    kernel_test.go:239: B-05 PASS — err=<nil>; status="promoted"
--- PASS: TestW1_B05_Promote (0.03s)
=== RUN   TestW1_B09_I1_PromoteWithDebt
    kernel_test.go:270: B-09 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B09_I1_PromoteWithDebt (0.01s)
=== RUN   TestW1_B10_I2_PromoteFinalTruth
    kernel_test.go:307: B-10 PASS — errors.Is=true; errors.As=true; sqlstate="23514"; constraint="promoted_is_debt_free"; status="entered"
--- PASS: TestW1_B10_I2_PromoteFinalTruth (0.00s)
=== RUN   TestW1_B18_RetryClassification
    kernel_test.go:372: B-18 PASS — injection=true, control txn sqlstate="40001"; Promote err=<nil>; status="promoted"; ErrPromotionBlocked=false; retry_count=unknown (crdb_internal restricted on v26.2, SQLSTATE 42501; not unlocked — see N2)
--- PASS: TestW1_B18_RetryClassification (0.04s)
=== RUN   TestW1_B06_IntentOnPromoted
    kernel_test.go:427: B-06 PASS — err=<nil>; state="live", belief_status="promoted"
--- PASS: TestW1_B06_IntentOnPromoted (0.03s)
=== RUN   TestW1_B11_I3_IntentOnUnpromoted
    kernel_test.go:460: B-11 PASS — err sentinel=true; sqlstate="23503"; constraint="gate"; intent rows=0
--- PASS: TestW1_B11_I3_IntentOnUnpromoted (0.01s)
=== RUN   TestW1_B12_I4_RetractSkippingCancel
    kernel_test.go:498: B-12 PASS — sqlstate="23514"; constraint="live_requires_promoted"; status="promoted"
--- PASS: TestW1_B12_I4_RetractSkippingCancel (0.03s)
=== RUN   TestW2_B07_RetractCascade

=== Wave 2 ===
    kernel_test.go:536: B-07 PASS — returned 2; root="retracted" child="retracted"; intent="cancelled"/"retracted"
--- PASS: TestW2_B07_RetractCascade (0.08s)
=== RUN   TestW2_B20_CascadeChain
    kernel_test.go:562: B-20 PASS — returned 3; unrelated belief="promoted"
--- PASS: TestW2_B20_CascadeChain (0.10s)
=== RUN   TestW2_B22_RowsAffectedSemantics
    kernel_test.go:595: B-22 PASS — returned 2; 3 intents cancelled
--- PASS: TestW2_B22_RowsAffectedSemantics (0.07s)
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
    kernel_test.go:771: B-21 PASS — returned 2 in 6 ms (deadline 30000 ms)
--- PASS: TestW3_B21_CycleTerminates (0.06s)
=== RUN   TestW3_B14_I6_NoEmbeddingColumn
    kernel_test.go:793: B-14 PASS — 0 embedding column(s) on belief
--- PASS: TestW3_B14_I6_NoEmbeddingColumn (0.02s)
=== RUN   TestW3_B15_I7_StaticGate
    kernel_test.go:808: B-15 PASS — NOT runtime-executable; asserted by scripts/check_i7.sh, run by scripts/m2_accept.sh before this suite — see docs/M1_I7.md
--- PASS: TestW3_B15_I7_StaticGate (0.00s)
=== RUN   TestW3_EnsureBelief_New
    kernel_test.go:836: W3-Ensure-New PASS — id parseable=true, status="entered", debt items=6
--- PASS: TestW3_EnsureBelief_New (0.01s)
=== RUN   TestW3_EnsureBelief_Existing
    kernel_test.go:865: W3-Ensure-Existing PASS — id1=75dab20f-d962-4f15-9923-62e6cebc4efd, id2=75dab20f-d962-4f15-9923-62e6cebc4efd, same=true; count=1
--- PASS: TestW3_EnsureBelief_Existing (0.01s)
=== RUN   TestW3_EnsureBelief_DifferentScenario
    kernel_test.go:889: W3-Ensure-DiffScenario PASS — idA=1eccf4ee-ade7-4a3e-92b3-02dce7ff0cba, idB=c81cdfb7-4828-48b9-a1b3-59bd296c09c0, different=true
--- PASS: TestW3_EnsureBelief_DifferentScenario (0.00s)
=== RUN   TestOperatorReview_PromoteWithoutAction

=== Wave operator-review ===
    operator_review_test.go:42: OR-1 PASS — audit = 0
--- PASS: TestOperatorReview_PromoteWithoutAction (0.02s)
=== RUN   TestOperatorReview_PromoteWithAction
    operator_review_test.go:97: OR-2 PASS — audit = 0, intent_count = 1
--- PASS: TestOperatorReview_PromoteWithAction (0.03s)
=== RUN   TestOperatorReview_UnpromotedBelief
    operator_review_test.go:131: OR-3 PASS — 23503
--- PASS: TestOperatorReview_UnpromotedBelief (0.00s)
=== RUN   TestOperatorReview_FailedPromotion
    operator_review_test.go:184: OR-4 PASS — promote=23514, intent=23503
--- PASS: TestOperatorReview_FailedPromotion (0.01s)
=== RUN   TestOperatorReview_ScenarioGuard_RejectsMismatch
    operator_review_test.go:258: OR-5 PASS — mismatch=true, debt_changed=false, status=entered, intents=0
--- PASS: TestOperatorReview_ScenarioGuard_RejectsMismatch (0.01s)
=== RUN   Example_lifecycle
--- PASS: Example_lifecycle (0.06s)
PASS
ok  	github.com/PithomLabs/solvent/kernel	3.283s
```
