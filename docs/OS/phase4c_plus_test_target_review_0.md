# Phase 4C+ Test Engineering Gate — Independent Adversarial Review

**Reviewer:** Kilo Code (independent, did not implement Phase 4C+)
**Date:** 2026-09-06
**Review target:** `docs/OS/phase4c_plus_test_target.md`
**Status:** REQUEST CHANGES

---

## 1. VERDICT

**REQUEST CHANGES**

The test target is structurally strong in many areas — the TOCTOU model is honest, the fake provider design is deterministic, and the adversarial tests are well-conceived. However, there are material contradictions with the design basis, gaps in invariant coverage, and insufficient enforcement of the parameter-binding security property. These must be resolved before OpenCode can implement against this specification without inventing missing security semantics.

---

## 2. EXECUTIVE RATIONALE

The document correctly:
- Defines `executed` as provider acceptance, not completion
- Distinguishes Window A (must deny) from Window B (documented v1 race)
- Provides a deterministic fake provider with call recording
- Includes adversarial tests for most major security controls
- Maps invariants to tests in a traceability matrix

But it:
- Contains a direct contradiction with the implementation plan regarding `CompleteIntent` placement
- Leaves SI-9 untested
- Does not prove the executor reconstructs parameters from the approved snapshot
- Allows caller-supplied matching parameters to become a second source of truth
- Has a non-deterministic Window B test with no concrete assertion

These gaps mean OpenCode would have to infer missing security semantics during implementation, which violates the "test engineering first" principle.

---

## 3. FINDINGS

### F-1 (CRITICAL): TestExec30 contradicts the implementation plan on CompleteIntent placement

**Section:** Test target §12.6 (TestExec30), Plan §21 (CompleteIntent call provenance)

**Problem:** The test target states:
> `CompleteIntent` is NOT in the `Contract` interface (or if it is, it is documented as internal-only).

But the implementation plan (Section 21, "Caller constraint") explicitly states:
> Add `CompleteIntent` to `Contract` interface

And the package/file map (Section 22) lists `kernel/contract.go` as:
> Add `CompleteIntent` to `Contract` interface | Prod | Medium

The test target's ambiguity ("NOT in the Contract interface (or if it is...)") directly contradicts the plan's explicit instruction to add it to the Contract interface.

**Why it matters:** OpenCode cannot implement both the test target and the plan. If CompleteIntent is in the Contract interface, TestExec30's assertion #1 fails. If it's not in the Contract interface, the plan's Step 3 is wrong.

**Remediation:** The test target must unambiguously state whether CompleteIntent is in the Contract interface. If the plan is authoritative, update TestExec30 to assert: "CompleteIntent IS in the Contract interface, and the test verifies no public/caller-controlled path invokes it through that interface."

---

### F-2 (CRITICAL): SI-9 has no test — provider success cannot retroactively create authority

**Section:** Test target §3 (SI-9), §13 (Traceability Matrix)

**Problem:** SI-9 states:
> Provider success does not retroactively create authority

The traceability matrix maps SI-9 to:
> TestExec28 — Provider rejection does not create authority

But TestExec28 tests the OPPOSITE case (provider rejection). There is no test that verifies provider success does NOT create authority. If a buggy implementation granted authority after provider success, no existing test would fail.

**Why it matters:** This is a security invariant with zero test coverage. A test that only verifies the negative case (rejection doesn't create authority) does not prove the positive case (success doesn't create authority).

**Remediation:** Add a test that:
1. Executes with valid authority, provider accepts
2. Verifies that no new authority target, belief promotion, or principal grant was created as a side effect of provider success
3. The test must fail if provider acceptance triggers any authority-granting side effect

---

### F-3 (CRITICAL): Parameter binding is not proven at the executor level

**Section:** Test target §12.2 (TestExec10-13), §13 (Traceability Matrix)

**Problem:** TestExec10-13 verify that parameter substitution is rejected at the AUTHORIZATION level (`kernel.Authorize` returns `Allowed=false`). But they do not verify that when authorization succeeds, the executor reconstructs parameters from the approved snapshot.

The test target states:
> The service layer reads the approved snapshot's `consequence_parameters`. Passes to executor as canonical values.

But the tests do not verify this. TestExec14 verifies that matching caller params produce correct provider calls, but does not verify that the executor received ONLY approved values and not caller-supplied values.

**Why it matters:** The security property is:
```
approved target → target_snapshot.consequence_parameters → execution-time reconstruction → executor → provider
```

If the implementation passes both approved values AND caller-supplied values to the executor, and the executor happens to use the approved ones when they match, TestExec14 passes but the security property is violated. The executor has a "second source of truth."

**Remediation:** Add a test where:
1. Caller supplies params that PARTIALLY match approved values but include an extra field
2. Authorization succeeds (because the core fields match)
3. The test verifies the provider received ONLY the approved fields, NOT the extra caller-supplied field
4. Or, the test verifies that caller-supplied params that CONTRADICT approved values are ignored even when the kernel-level check passes

Alternatively, TestExec14 should be strengthened to verify the executor received the approved snapshot values by inspecting the fake provider's `Calls()` and asserting the exact struct fields match the snapshot, not just the caller's input.

---

### F-4 (HIGH): TestExec15B (Window B) has no deterministic assertion

**Section:** Test target §12.3 (TestExec15B)

**Problem:** The test specification says:
> - Assertions:
>   - `result.Allowed == true` (check already passed)
>   - Provider MAY be called (0 or 1 calls).
>   - This is a documented v1 race. Test MUST characterize, not prevent.

A test whose expected outcome is "MAY be called" is non-deterministic. It will pass regardless of whether the provider is called or not. Such a test cannot prove any security property — it merely documents the intended behavior in prose.

**Why it matters:** The review explicitly asks for deterministic tests. A test that passes in both the secure and insecure case provides no adversarial value.

**Remediation:** The test must specify a concrete observable that characterizes the race. For example:
- If the implementation adds a re-check between T2 and T3, the test must verify that re-check exists and is deterministic
- OR, the test must be removed from the "must pass" acceptance criteria and moved to a characterization document
- The test specification must state: "This test documents the accepted race; it is not a security gate"

---

### F-5 (HIGH): Caller-supplied matching parameters can become a second source of truth

**Section:** Test target §12.2 (TestExec14), §13 (Traceability Matrix)

**Problem:** TestExec14 verifies that when caller params MATCH approved values, execution succeeds and the provider receives those values. But it does not verify that the values came from the approved snapshot, not from the caller.

If the implementation uses caller-supplied params as the authoritative source when they match the snapshot, TestExec14 passes. The security property (SI-16, SI-17, SI-18) is violated because the caller's input is now a valid execution parameter path.

**Why it matters:** The architecture explicitly states:
> Caller-supplied execution parameters are ignored by the executor.

But TestExec14 does not prove this. It only proves that matching values produce correct results.

**Remediation:** TestExec14 must be split into two tests:
1. Test that matching caller params produce correct provider calls (current TestExec14)
2. Test that the executor receives values from the approved snapshot, not from caller params — for example, by having the caller supply params that match the snapshot but the test verifies the executor used the snapshot's copy (not the caller's reference)

---

### F-6 (MEDIUM): TestExec10-13 do not verify executor receives approved values

**Section:** Test target §12.2 (TestExec10-13)

**Problem:** These tests verify that substitution is rejected at the authorization level. But they do not verify that the executor reconstructs parameters from the approved snapshot.

If the implementation's executor reads from caller-supplied params instead of the snapshot, and the kernel.Authorize check happens to reject the mismatch, the tests pass but the executor is still reading from the wrong source.

**Why it matters:** The security model requires the snapshot to be the sole source of truth. Tests that only verify the authorization gate do not prove the executor's parameter source.

**Remediation:** Add a complementary test where authorization succeeds but the executor's parameter source is verified to be the snapshot, not caller input.

---

### F-7 (MEDIUM): F-18 (provider success but Solvent receives error) not tested

**Section:** Test target §5 (Failure Model), §12.4 (Provider Failure Tests)

**Problem:** F-18 states:
> Provider succeeds but Solvent receives error — Side effect happened. Solvent reports failure.

But there is no test for this scenario. TestExec23 tests "provider accepts but response lost" (F-15), but F-18 is a different case: the provider succeeded AND Solvent received an error response.

**Why it matters:** The failure model lists this as a distinct ambiguous outcome, but no test proves the system handles it correctly.

**Remediation:** Add a test that simulates the provider succeeding but Solvent receiving an error, and verifies:
- Execution result reports failure (truthful)
- The side effect happened (provider was called)
- Audit records the discrepancy

---

### F-8 (LOW): Fake provider lost-response simulation not specified

**Section:** Test target §6 (FakeGitHubProvider), §12.4 (TestExec23)

**Problem:** TestExec23 says:
> Fake provider configured to accept but then simulate a response loss (e.g., close connection before reading response).

But the fake provider interface in Section 6 does not include a mechanism for simulating lost responses. The struct has `accept`, `err`, `delay`, but no `lostResponse` field or method.

**Why it matters:** The test cannot be implemented without extending the fake provider interface.

**Remediation:** Either add a `lostResponse` field to `FakeGitHubProvider` or specify an alternative deterministic mechanism (e.g., return a special error that indicates response loss).

---

### F-9 (LOW): TestExec15B synchronization mechanism underspecified

**Section:** Test target §12.3 (TestExec15B)

**Problem:** The test says:
> Between T2 and T3: revocation goroutine revokes target.

But T2 and T3 are in the same goroutine with no I/O. The test needs a way to inject the revocation between the final authorization check and the executor invocation. The test target mentions channels for synchronization in other tests but does not specify the exact mechanism for TestExec15B.

**Why it matters:** Without a specified synchronization mechanism, OpenCode cannot implement a deterministic test.

**Remediation:** Specify the exact synchronization primitive (e.g., a channel that the execution goroutine signals after T2 and before T3, which the revocation goroutine waits on).

---

## 4. EXPLICIT DETERMINATIONS

### Parameter Binding
**NOT adequately tested.** Tests prove authorization rejects substitutions, but do not prove the executor reconstructs parameters from the approved snapshot. The executor's parameter source is not verified.

### Executor Selection
**Adequately tested.** TestExec09 and TestAT05 directly prove that caller-supplied `tool_name` is ignored and the correct executor is derived from the authorized action.

### Window A TOCTOU (revocation before final check)
**Adequately tested.** TestExec08 and TestExec15A require zero provider calls when revocation occurs before the final authorization re-read.

### Window B TOCTOU (revocation after final check, before provider call)
**NOT honestly modeled.** TestExec15B has no deterministic assertion. The test will pass regardless of outcome, providing no adversarial value. The acceptance rationale is correct (local function-call boundary), but the test does not characterize the behavior.

### CompleteIntent Semantics
**Mostly unambiguous.** `executed` is correctly defined as provider acceptance. Tests verify intent stays `live` on rejection and ambiguous outcomes. However, TestExec30 contradicts the plan on Contract interface placement.

### Provider Acceptance vs Completion
**Correctly distinguished.** The test target maintains the four-fact model and does not conflate acceptance with completion.

### Audit Semantics
**Adequately tested.** Tests verify audit entry ordering and distinct types. TestExec34 verifies audit failure does not falsify execution results.

### Deterministic Fake-Provider Design
**Mostly sufficient.** The fake provider supports call recording, deterministic synchronization, and controllable behavior. Lost-response simulation is missing from the interface.

### Invariant-to-Test Traceability
**Incomplete.** SI-9 has no test. Some tests verify only the authorization-level check, not the executor-level behavior.

---

## 5. TRACEABILITY GAPS

| Invariant | Mapped Tests | Gap |
|-----------|--------------|-----|
| SI-9 | TestExec28 (tests opposite case) | No test proves provider success does NOT create authority |
| SI-16 | TestExec10-13, TestAT13-15 | Tests verify auth rejects; do not verify executor uses snapshot |
| SI-17 | TestExec10-13, TestAT13-15 | Tests verify auth rejects; do not verify executor ignores caller params |
| SI-18 | TestExec10-14 | Tests verify auth rejects; do not verify executor receives ONLY approved params |

---

## 6. FINAL STATEMENT

**Is this test target strong enough to serve as the acceptance contract?**

**NO — not in its current state.**

The test target contains:
- 1 critical contradiction with the implementation plan (CompleteIntent placement)
- 1 critical invariant with zero test coverage (SI-9)
- 1 critical security property not proven at the required level (parameter binding at executor level)
- 1 non-deterministic test that provides no adversarial value (TestExec15B)
- 2 medium gaps in test completeness (F-6, F-7)

OpenCode cannot implement against this specification without:
1. Resolving the CompleteIntent contradiction
2. Inventing a test for SI-9
3. Determining how to prove executor-level parameter binding
4. Deciding what to do with the non-deterministic Window B test

These are not minor wording issues. They are material security and consistency gaps that would force OpenCode to make unauthorized design decisions during implementation.

**The test target must be revised to address F-1 through F-7 before implementation may begin.**

---

## 7. REMEDIATION PRIORITY

1. **CRITICAL — Must fix before GO:**
   - F-1: Resolve CompleteIntent placement contradiction
   - F-2: Add test for SI-9
   - F-3: Add executor-level parameter binding test

2. **HIGH — Must fix before GO:**
   - F-4: Make TestExec15B deterministic or remove from acceptance criteria
   - F-5: Strengthen TestExec14 to prove snapshot is sole source

3. **MEDIUM — Should fix before GO:**
   - F-6: Add test verifying executor receives approved values
   - F-7: Add test for F-18 (provider success, Solvent error)

4. **LOW — Can fix during implementation:**
   - F-8: Add lostResponse to fake provider
   - F-9: Specify TestExec15B synchronization mechanism

---

**This review is complete. Phase 4C+ Step 0 cannot proceed until the test target document is revised to address the CRITICAL and HIGH findings.**
