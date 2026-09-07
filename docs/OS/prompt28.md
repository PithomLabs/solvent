Yes — this is the **real Gate 1 Kilo review**, and the result is useful. Kilo is correctly blocking the handoff to implementation, but several findings are straightforward specification fixes rather than architectural redesign.

The important thing is **do not send this back to OpenCode to implement yet**. Send it back to OpenCode to **revise only the test-target document**, then rerun Kilo Gate 1.

### What Kilo found

The three issues that genuinely must be fixed before GO are:

**F-1 — CompleteIntent contradiction.**
The test target currently says CompleteIntent may not be in `Contract`, while the implementation plan explicitly requires adding it to `kernel/contract.go`. Kilo correctly identifies that OpenCode otherwise has to choose which document to violate. 

This is easy to resolve: **keep `CompleteIntent` in the Contract interface**, as the implementation plan specifies, and make the test assert that although the method exists on the kernel contract, **no REST/MCP/public path exposes or invokes it**. 

**F-2 — SI-9 is genuinely uncovered.**
The existing test proves provider rejection doesn't create authority, but SI-9 concerns provider **success** not retroactively creating authority. Kilo is right that those are different properties. 

Add a success-path test that snapshots the relevant authority state before execution and verifies that successful GitHub acceptance creates **no new authority grant/promotion/activation**.

**F-3/F-5 — parameter binding needs to prove source, not merely equality.**
This is the biggest substantive issue. Kilo correctly spotted that:

```text
caller params == approved values
```

does **not** prove:

```text
approved snapshot → executor
```

The implementation could still be using caller input and merely happen to receive matching values. 

The cleanest fix is to make the test explicitly prove that the **service constructs the executor parameter map from the approved snapshot**, while caller input is ignored.

### Window B

Kilo also confirms what we already suspected: the architectural rationale is correct, but the current test is not useful because “0 or 1 calls” makes it pass either way. 

I would **not redesign the execution model** to eliminate Window B at this stage. Instead, revise the test specification so Window B is explicitly a **characterization test / non-gating test**, with deterministic synchronization showing:

```text
T2 completed
    ↓
revocation occurs on independent connection
    ↓
T3 executor invocation
```

The acceptance criteria should not treat the test as a security proof. Kilo itself explicitly accepts that as the appropriate direction. 

### One correction to Kilo's F-6/F-7 classification

Kilo calls F-6 and F-7 “medium,” but the actual substance is already covered by the same parameter-binding and failure-model work. They are worth fixing, but they do **not** require an architectural change.

So I would now have OpenCode make a **test-target-only revision**, then hand it back to Kilo.

Use this prompt:

```text
Phase 4C+ — Gate 1 Remediation Only

Kilo Code has reviewed:

    docs/OS/phase4c_plus_test_target.md

Verdict: REQUEST CHANGES.

DO NOT implement production code.

Your task is to revise ONLY the test-engineering specification so that Kilo can perform Gate 1 again.

Primary review report:
    docs/OS/phase4c_plus_test_target_review.md

Address every CRITICAL and HIGH finding explicitly.

Required changes:

1. COMPLETEINTENT / CONTRACT INTERFACE

Resolve the contradiction identified by Kilo F-1.

The implementation plan is authoritative here:
- CompleteIntent SHALL be added to the kernel Contract interface.
- It remains internal-only in terms of exposure.
- No REST endpoint invokes it.
- No MCP tool invokes it.
- No caller-controlled input can select or invoke it.

Update TestExec30 accordingly.

The test MUST NOT assert that CompleteIntent is absent from Contract.

Instead prove:
- CompleteIntent exists on Contract as required by the implementation design.
- No REST handler calls it.
- No MCP tool calls it.
- The only production call site is the trusted execution path.
- That call occurs only after provider acceptance.

Do not redesign this.

2. SI-9 — PROVIDER SUCCESS MUST NOT CREATE AUTHORITY

Add a dedicated test proving:

    valid authority
      → provider accepts
      → execution succeeds
      → NO new authority is created

The test must snapshot relevant authority state before execution and verify that provider success does not:
- create a new target
- create a new activation
- promote a belief
- grant authority to another principal
- otherwise mutate authority state as a consequence of provider success

Do not rely on provider rejection for this property.

Add the test to the traceability matrix for SI-9.

3. PARAMETER BINDING — PROVE THE SOURCE OF TRUTH

Strengthen the existing parameter-binding tests.

The security property is NOT merely:

    caller params == approved params

The property is:

    approved target snapshot
        ↓
    execution-time reconstruction
        ↓
    executor
        ↓
    provider

Caller-supplied execution parameters must NOT be a second source of truth.

Add explicit test coverage proving this.

At minimum add a test such as:

    TestExec15_ExecutorUsesApprovedSnapshotParams

Setup:
- approved snapshot contains repo/workflow/ref A
- caller supplies parameters that are either:
  a) matching values but independently constructed, and/or
  b) conflicting values that must be ignored/rejected

The test must observe the actual values received by the executor/provider.

The test must prove:
- provider receives the approved snapshot values
- provider does not receive caller-only values
- caller cannot inject an additional provider parameter
- caller values do not become authoritative merely because they match

Prefer validating the actual executor parameter map where practical, not only final provider output.

Update SI-16, SI-17, SI-18 traceability accordingly.

4. STRENGTHEN TestExec14

The existing matching-parameter test is insufficient by itself.

Keep it if useful, but explicitly state that it proves compatibility, not source-of-truth.

Add a complementary test whose failure mode would be:

    implementation uses caller parameters as authoritative
    → test FAILS

This must be visible in the traceability table.

5. TOCTOU WINDOW B

Do NOT redesign away Window B.

Keep the existing architectural statement that the race exists.

Replace the current nondeterministic acceptance condition:

    provider MAY be called (0 or 1)

with a deterministic characterization specification.

Use explicit synchronization around the execution path, conceptually:

    execution reaches T2 and signals channel
        ↓
    independent revocation occurs through separate DB connection
        ↓
    test releases execution into T3
        ↓
    executor invocation is observed

The test is a characterization test, NOT a security gate.

State explicitly:

- Window A is a security gate: revocation before T2 MUST deny.
- Window B is a documented v1 limitation.
- The implementation must not claim atomic DB authorization + provider execution.
- The test exists to prove the race boundary is understood and characterized, not eliminated.

Do NOT use MCP serialization as the justification.

6. F-18

Add explicit test coverage for:
- provider side effect occurs
- Solvent receives/represents an error
- execution result remains truthful
- audit records the discrepancy appropriately

Make the distinction from response-loss/ambiguous outcome explicit.

7. LOST RESPONSE

Extend the fake-provider specification with an explicit deterministic mechanism for simulating:
- provider accepts
- response is lost

Do not leave this implicit.

8. TRACEABILITY

Update the invariant matrix so every material invariant maps to a concrete test and exact observable assertion.

Especially verify:
- SI-9
- SI-16
- SI-17
- SI-18
- CI-4

For each relevant test include:

    Invariant
    Test Function
    Exact Assertion
    What breaks if the security property is removed

9. ACCEPTANCE CRITERIA

Make the distinction explicit:

GATING SECURITY TESTS:
- must have deterministic pass/fail expectations

CHARACTERIZATION TESTS:
- document known v1 limitations
- are not used to claim a stronger security guarantee

Window B belongs in the second category.

10. SCOPE

This remains documentation-only.

DO NOT:
- modify production code
- add kernel behavior
- add services
- change schema
- add async execution
- add retries
- add multiple providers
- add remote execution API
- eliminate Window B by architectural redesign

After revising the document:

1. Verify only the test-target documentation changed.
2. Re-check every Kilo finding F-1 through F-9.
3. Update the test target's internal numbering and traceability consistently.
4. STOP.

Do not implement any tests or production code.

The revised repository will be handed back to Kilo Code for Gate 1 re-review.
```

That is the right loop now:

**Kilo REQUEST CHANGES → OpenCode revises the test specification only → Kilo re-reviews → GO → OpenCode implements.**

And notably, Kilo itself says the test target is close: it specifically describes the TOCTOU model, fake provider, executor selection, `executed` semantics, failure model, kernel-growth discipline, and architectural boundaries as correct. 

So this is **not a restart**. It is a focused Gate 1 remediation pass.
