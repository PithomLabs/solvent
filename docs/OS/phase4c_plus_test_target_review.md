# Phase 4C+ Test Engineering Gate — Independent Adversarial Review (Second Pass)

**Reviewer:** Kilo Code (independent, did not implement Phase 4C+)
**Date:** 2026-09-06
**Review target:** `docs/OS/phase4c_plus_test_target.md`
**Status:** GO — TEST ENGINEERING GATE PASSED

---

## 1. VERDICT

**GO — TEST ENGINEERING GATE PASSED**

The revised test target is now a sufficiently precise, internally consistent, deterministic, adversarial, and implementation-ready acceptance contract. OpenCode may proceed to Step 1 implementation against this specification without inventing missing security semantics.

---

## 2. EXECUTIVE RATIONALE

The first-pass review identified nine findings (F-1 through F-9) covering CompleteIntent placement contradiction, missing SI-9 coverage, insufficient executor-level parameter binding proof, nondeterministic Window B test, caller-parameter source-of-truth gaps, missing F-18 coverage, incomplete fake-provider contract, and underspecified Window B synchronization.

The revised document resolves all nine findings:

1. **CompleteIntent placement**: Now explicitly states CompleteIntent IS in the Contract interface (line 927), aligning with the implementation plan. TestExec30 verifies no public path invokes it.
2. **SI-9 coverage**: TestExec35_ProviderSuccessDoesNotCreateAuthority (lines 995-1029) captures pre-execution authority state and verifies byte-for-byte equality afterward, proving provider success does not retroactively create authority.
3. **Executor-level parameter binding**: TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams (lines 671-710) explicitly verifies the executor receives ONLY approved snapshot values, ignoring conflicting or extra caller-supplied parameters.
4. **Window B determinism**: TestExec15B (lines 729-768) is now a deterministic characterization test with explicit channel synchronization and concrete assertions (provider call count == 1).
5. **Caller parameter isolation**: TestExec14 explicitly disclaims proving snapshot sole-source-of-truth and defers to TestExec36 (line 667-669).
6. **F-18 coverage**: TestExec38_ProviderSuccessSolventReceivesError (lines 1031-1057) covers provider success with Solvent-visible error, distinct from response-lost and timeout scenarios.
7. **Fake provider completeness**: `lostResponse` field and `SetLostResponse` method added (lines 207-208, 231), with explicit behavioral contract (lines 235-244).
8. **Window B synchronization**: Explicit channel-based synchronization specified (lines 738-745).
9. **Traceability matrix**: All SI/CI invariants now map to concrete tests with exact assertions.

The document maintains architectural scope, preserves the four-fact model, and correctly distinguishes gating security tests from characterization tests.

---

## 3. FINDING-BY-FINDING DISPOSITION

### F-1: CompleteIntent / Contract Contradiction
**Status: RESOLVED**

**Evidence:**
- Line 927: "CompleteIntent IS in the Contract interface (required by implementation design)."
- Line 938: Assertions include "CompleteIntent exists on Contract interface."
- TestExec30 (lines 924-945) now verifies CompleteIntent is in the Contract interface and that no REST/MCP handler invokes it.

The test target now aligns with the implementation plan's explicit instruction to add CompleteIntent to the Contract interface.

---

### F-2: Missing SI-9 Coverage
**Status: RESOLVED**

**Evidence:**
- Lines 995-1029: TestExec35_ProviderSuccessDoesNotCreateAuthority
- The test captures pre-execution state of target, activation, target_snapshot, belief, and principal rows.
- After execution, it re-queries and asserts byte-for-byte equality (except intent state transition).
- Explicitly verifies: no new target, no modified target, no new activation, no re-promotion, no new principal.
- The only expected state change is the intent transition from `live` to `executed`.

This is strong state comparison, not count-only checking. The test would fail if provider success triggered any authority mutation.

---

### F-3: Parameter Binding Not Proven at Executor Level
**Status: RESOLVED**

**Evidence:**
- Lines 601-614: The document explicitly distinguishes two binding levels:
  - Kernel-level binding (TestExec10-13): proves kernel.Authorize rejects substituted tuples
  - Execution-path binding (TestExec36): proves service layer reconstructs from snapshot, executor receives only approved values
- Lines 671-710: TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams
  - Variant a: caller supplies matching + extra fields → executor receives EXACTLY approved fields, extra field NOT present
  - Variant b: caller supplies conflicting values → service IGNORES caller values, executor receives snapshot values
  - Variant c: caller supplies arbitrary extra provider-looking fields → those fields NOT present in executor's received params
- The test uses a custom executor that records the exact parameter map it receives.

The security property is now proven at the executor level, not just the authorization level.

---

### F-4: Nondeterministic Window B Test
**Status: RESOLVED**

**Evidence:**
- Lines 729-768: TestExec15B_RevocationAfterCheck_DocumentedRace
- Classification: "CHARACTERIZATION TEST — documents a known v1 limitation. NOT a security gate."
- Synchronization: Explicit channel-based forcing:
  1. Execution goroutine passes T2, signals `afterT2`
  2. Revocation goroutine waits on `afterT2`, revokes via SEPARATE DB connection
  3. Revocation goroutine signals `revoked`
  4. Execution goroutine waits on `revoked`, then invokes executor (T3)
- Assertions:
  - `result.Allowed == true` (deterministic)
  - The executor WAS invoked (deterministic — synchronization guarantees ordering)
  - Provider call count is exactly 1 (deterministic)
  - Intent state: may be `executed` or `live` (documented)
- The test is no longer a "0 or 1 calls" nondeterministic test. It has concrete deterministic observables.

---

### F-5: Caller-Supplied Matching Parameters Could Remain a Second Source of Truth
**Status: RESOLVED**

**Evidence:**
- Lines 654-669: TestExec14_CallerSuppliedMatchingParams now explicitly notes: "Does NOT prove snapshot is sole source of truth; see TestExec36."
- Lines 671-710: TestExec36 proves the executor receives only approved snapshot values.
  - Variant a: extra caller fields are stripped
  - Variant b: conflicting caller values are ignored
  - Variant c: arbitrary extra provider-looking fields are stripped

The snapshot is now proven to be the sole source of truth.

---

### F-6: Incomplete Executor-Level Parameter Verification
**Status: RESOLVED**

**Evidence:**
- TestExec36 explicitly tests the execution path, not just kernel.Authorize.
- The test verifies the executor receives only approved snapshot values through a custom executor that records exact parameter maps.
- This is covered under F-3 resolution.

---

### F-7: Missing F-18 Test
**Status: RESOLVED**

**Evidence:**
- Lines 1031-1057: TestExec38_ProviderSuccessSolventReceivesError
- Simulates: provider accepts internally, but Solvent receives error
- Assertions: provider called exactly 1, result.Success == false, intent stays live, audit contains executor_failed
- Explicitly distinguished from:
  - TestExec23 (F-15: response lost entirely — ambiguous)
  - TestExec22 (F-14: timeout — provider state unknown)
  - This test: provider succeeded definitively, Solvent received error

---

### F-8: Missing Lost-Response Fake-Provider Mechanism
**Status: RESOLVED**

**Evidence:**
- Lines 200-210: FakeGitHubProvider now has `lostResponse bool` field
- Lines 231-232: `SetLostResponse(lost bool)` method
- Lines 235-244: Lost-response behavioral contract explicitly defined:
  - When `lostResponse == true` and `accept == true`: records the call, returns error simulating response loss
  - Intent state remains `live`
  - `result.Success == false`

---

### F-9: Underspecified Window B Synchronization
**Status: RESOLVED**

**Evidence:**
- Lines 735-745: Explicit channel synchronization specified:
  1. Execution goroutine passes T2, signals `afterT2`
  2. Revocation goroutine waits on `afterT2`, revokes via SEPARATE DB connection
  3. Revocation goroutine signals `revoked`
  4. Execution goroutine waits on `revoked`, then invokes executor (T3)

The synchronization mechanism is deterministic and clearly specified.

---

## 4. NEW FINDINGS

**None.** No new material findings were discovered during this review. The document is internally consistent and all security invariants are now adequately tested.

---

## 5. EXPLICIT DETERMINATIONS

### CompleteIntent Consistency
**PASS.** CompleteIntent is consistently stated to be in the Contract interface (lines 927, 938). The call provenance is strictly limited to the trusted execution path after provider acceptance (lines 406-423). TestExec30 verifies no REST/MCP handler invokes it. No contradictions found anywhere in the document.

### SI-9 Coverage
**PASS.** TestExec35_ProviderSuccessDoesNotCreateAuthority (lines 995-1029) explicitly proves provider success does not retroactively create authority. The test captures pre-execution authority state and verifies byte-for-byte equality afterward (except intent state transition). This is strong state comparison, not count-only checking.

### Executor-Level Parameter Binding
**PASS.** TestExec36_ExecutorUsesApprovedSnapshotNotCallerParams (lines 671-710) explicitly verifies:
- Caller-supplied execution parameters are ignored
- Service reconstructs execution parameters from approved snapshot
- Executor receives only approved snapshot values
- Provider receives only approved values
- Conflicting caller parameters do NOT become the authorization source
- Extra caller fields do not reach executor/provider

### Caller Parameter Source-of-Truth Isolation
**PASS.** TestExec36 variants a, b, and c collectively prove that caller parameters are never part of the execution data path. The snapshot is the sole source of truth.

### Window A TOCTOU (Revocation Before Final Check)
**PASS.** TestExec08 and TestExec15A require zero provider calls when revocation occurs before the final authorization re-read. Deterministic synchronization via channels and CockroachDB SERIALIZABLE.

### Window B TOCTOU (Revocation After Final Check, Before Provider Call)
**PASS.** TestExec15B is now a deterministic characterization test:
- Uses explicit channel synchronization to force ordering
- Asserts provider call count == 1 (deterministic)
- Clearly classified as CHARACTERIZATION, not GATING SECURITY
- Does NOT claim prevention
- Does NOT rely on sequential MCP processing
- Acceptance rationale is the local function-call boundary

### Provider Failure / Ambiguity
**PASS.** The failure model is fully covered:
- F-14 (timeout): TestExec22
- F-15 (response lost): TestExec23
- F-18 (provider success, Solvent error): TestExec38
- All three are distinct and have separate tests with distinct assertions

### Audit Semantics
**PASS.** Tests verify:
- Audit entry ordering (TestExec31)
- Distinct entry types for denied execution (TestExec32)
- Distinct entry types for provider failure (TestExec33)
- Audit failure does not falsify execution results (TestExec34)

### Invariant Traceability
**PASS.** Every SI/CI invariant maps to concrete tests with exact assertions:
- SI-9: TestExec35 (NEW)
- SI-16, SI-17, SI-18: TestExec10-13 (kernel), TestExec36 (execution path), TestAT13-16 (adversarial)
- CI-5: TestExec14, TestExec36
- CI-6: TestExec10-13, TestExec14, TestExec36

### Architectural Scope
**PASS.** The test target does not require:
- New services
- Schema changes (beyond already-justified CompleteIntent)
- Generic executor framework
- Multiple providers
- Async polling
- Retry orchestration
- RBAC
- Multi-tenancy
- Remote execution API
- Additional kernel primitives

---

## 6. FINAL ANSWER

**"Can OpenCode now implement Phase 4C+ from this test target without inventing missing security semantics?"**

**YES.**

The test target is now implementation-ready. All nine previous findings have been resolved. The document provides:

1. **CompleteIntent consistency**: Unambiguous placement in Contract interface, verified by TestExec30.
2. **SI-9 coverage**: TestExec35 proves provider success does not create authority through strong state comparison.
3. **Executor-level parameter binding**: TestExec36 proves the executor receives only approved snapshot values, not caller params.
4. **Window B characterization**: TestExec15B is deterministic, uses explicit synchronization, and correctly characterizes the race without claiming prevention.
5. **F-18 coverage**: TestExec38 covers provider success with Solvent-visible error.
6. **Complete fake provider contract**: Includes lostResponse mechanism with explicit behavioral contract.
7. **Complete traceability**: Every SI/CI invariant maps to concrete tests with exact assertions.

OpenCode can implement against this specification without making unauthorized design decisions. The test target is a precise, adversarial, deterministic acceptance contract.

---

**GO — TEST ENGINEERING GATE PASSED**

Phase 4C+ implementation may proceed to Step 1.
