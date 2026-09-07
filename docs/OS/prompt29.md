Phase 4C+ — Gate 1 Final Re-Review

You are Kilo Code, the independent adversarial reviewer for Solvent Phase 4C+.

This is the SECOND PASS of Gate 1 after the previous REQUEST CHANGES verdict.

Review the CURRENT repository state and the revised:

    docs/OS/phase4c_plus_test_target.md

Do NOT implement code.
Do NOT modify the repository.
Do NOT assume the previous findings were correctly fixed.
Independently verify each remediation against the actual current document.

Previous Kilo review identified:
- F-1 CompleteIntent / Contract contradiction
- F-2 missing SI-9 coverage
- F-3 parameter binding not proven at executor level
- F-4 nondeterministic Window B test
- F-5 caller params could remain a second source of truth
- F-6 incomplete executor-level parameter verification
- F-7 missing F-18 test
- F-8 missing lost-response fake-provider mechanism
- F-9 underspecified Window B synchronization

The revised test target is intended to resolve these findings.

REVIEW OBJECTIVE

Determine whether:

    docs/OS/phase4c_plus_test_target.md

is now a sufficiently precise, internally consistent, deterministic,
adversarial, implementation-ready acceptance contract such that OpenCode
can implement Phase 4C+ without inventing missing security semantics.

CRITICAL REVIEW AREAS

1. CompleteIntent

Verify that the document now consistently states:
- CompleteIntent IS in the Contract interface.
- It remains a kernel-owned state transition.
- REST cannot invoke it.
- MCP cannot invoke it.
- caller-controlled input cannot invoke it.
- the only production call path is the trusted execution path.
- it occurs only after provider acceptance.

Check for contradictions anywhere in the document.

2. SI-9

Verify that a dedicated success-path test proves:
- provider acceptance does not create authority
- existing authority rows remain unchanged
- no new target/activation/principal/belief is created
- the expected intent transition is the only relevant state change

Do not accept count-only checking.
Verify that the test actually compares state strongly enough to catch mutation with unchanged counts.

3. Parameter binding — CRITICAL

This is the most important review area.

Verify the document now distinguishes two different tests:

A. Kernel binding tests:
   - deliberately construct a wrong AuthorityTuple
   - prove kernel.Authorize rejects the mismatched tuple

B. Execution-path binding tests:
   - caller-supplied execution parameters are ignored
   - service reconstructs execution parameters from approved snapshot
   - executor receives only approved snapshot values
   - provider receives only approved values
   - conflicting caller parameters do NOT become the authorization source
   - extra caller fields do not reach executor/provider

Specifically inspect TestExec10-13 versus TestExec36.

There must be no contradiction between:
- "kernel rejects a deliberately wrong tuple"
and
- "caller parameters are ignored during ExecuteAction."

4. Window B TOCTOU

Verify that the revised TestExec15B:
- is explicitly a characterization test, not a security gate
- uses deterministic synchronization
- forces:
      T2 completed
      → revoke through independent DB connection
      → T3 executor invocation
- demonstrates the accepted race boundary
- does NOT claim prevention
- does NOT rely on sequential MCP processing
- clearly distinguishes Window A from Window B

Check that the test has an actual deterministic observable result and is not a disguised "0 or 1 calls" nondeterministic test.

5. Failure model

Verify F-18 now has explicit test coverage and remains distinct from:
- response lost
- timeout
- provider rejection

Check whether the document's fake provider semantics actually support the stated failure modes.

6. Fake provider

Verify the fake provider contract is complete enough to implement:
- call recording
- exact parameter inspection
- acceptance/rejection
- errors
- deterministic synchronization
- lost response
- provider acceptance followed by Solvent-visible error, where required

No sleep-based race testing.

7. Traceability

Verify every SI/CI invariant maps to concrete tests with:
- exact assertion
- observable effect
- meaningful failure mode

Pay particular attention to:
- SI-9
- SI-16
- SI-17
- SI-18
- CI-5
- CI-6

8. Adversarial tests

For each major security property, ask:

"If the relevant security control were removed or replaced with caller-controlled behavior, would the specified test actually fail?"

Pay special attention to:
- arbitrary executor selection
- caller-controlled execution parameters
- snapshot bypass
- authorization bypass
- false execution success
- false provider acceptance
- authority mutation after provider success

9. Acceptance criteria

Verify that the document clearly distinguishes:
- GATING SECURITY TESTS
- CHARACTERIZATION TESTS

Window B and concurrent duplicate execution must not accidentally be represented as stronger security guarantees than intended.

10. Architectural scope

Ensure nothing in the revised test target silently requires:
- new services
- schema changes
- generic executor framework
- multiple providers
- async polling
- retry orchestration
- RBAC
- multi-tenancy
- remote execution API
- additional kernel primitives

FINAL DELIVERABLE

Produce:

    docs/OS/phase4c_plus_test_target_review.md

Overwrite/update the previous review report with this second-pass review.

The report must contain:

1. VERDICT:
   - GO
   - REQUEST CHANGES

2. Executive rationale.

3. Finding-by-finding disposition:
   - F-1 through F-9
   - Mark each:
       RESOLVED
       PARTIALLY RESOLVED
       NOT RESOLVED
   - Explain the evidence.

4. Any NEW findings discovered during this review.

5. Explicit determinations for:
   - CompleteIntent consistency
   - SI-9 coverage
   - executor-level parameter binding
   - caller parameter source-of-truth isolation
   - Window A
   - Window B
   - provider failure/ambiguity
   - audit semantics
   - invariant traceability
   - architectural scope

6. Final answer to this exact question:

"Can OpenCode now implement Phase 4C+ from this test target without inventing missing security semantics?"

GATE RULE

Return GO only when the document is genuinely implementation-ready.

Do not approve because the previous findings were cosmetically addressed.
Reject anything that still leaves OpenCode to choose between competing security semantics.

If the target is ready, issue:

    GO — TEST ENGINEERING GATE PASSED

and explicitly authorize progression to Step 1 implementation.

If not ready, issue:

    REQUEST CHANGES — TEST ENGINEERING GATE NOT PASSED

with precise remediation.

This is a review-only task. Do not modify production code.