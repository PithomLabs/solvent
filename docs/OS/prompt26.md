Kilo did exactly what we wanted at Gate 1, but it exposed an important procedural issue: **OpenCode has not yet created the actual test-target file.**

Kilo's verdict is **REQUEST CHANGES / BLOCKED**, specifically because `docs/OS/phase4c_plus_test_target.md` does not exist. It also says the plan itself is not a sufficient substitute for the test target because the required executable detail—exact test functions/assertions, invariant→test traceability, deterministic TOCTOU synchronization, parameter substitution assertions, etc.—has not yet been extracted into that document. 

So the correct next step is **not another Kilo prompt**. Send this to **OpenCode**:

```text
Phase 4C+ Step 0 — Create the Test Engineering Target

Kilo Code has completed the first independent gate review.

Verdict: REQUEST CHANGES / BLOCKED.

The blocker is procedural and explicit:
`docs/OS/phase4c_plus_test_target.md` does not yet exist.

Do NOT implement any production code yet.

Your task now is ONLY to create:

    docs/OS/phase4c_plus_test_target.md

This document must be the executable acceptance contract for Phase 4C+ implementation.

Base it on the approved current Phase 4C+ implementation plan and preserve its existing architectural decisions. Do not redesign Phase 4C+.

The test target must explicitly contain, at minimum:

1. Scope and behavioral objective

2. Canonical state model
   - authorization
   - invocation
   - provider acceptance
   - provider completion
   - `action_intent.state = executed` means provider accepted the request, NOT workflow completion

3. Security invariants
   - SI-1 through SI-18 from the implementation plan

4. Correctness invariants
   - CI-1 through CI-6

5. Failure model
   - every applicable failure mode from the plan, including ambiguous provider outcomes

6. Deterministic fake-provider contract
   - exact interface/types needed by tests
   - recorded calls
   - exact received repo/workflow/ref/inputs
   - acceptance/rejection
   - returned run reference
   - controlled errors
   - synchronization primitives
   - no sleep-based race testing

7. Exact test specifications
   For each test provide:
   - test function name
   - setup
   - inputs
   - synchronization/interleaving
   - exact assertions
   - expected provider call count
   - expected side effect
   - security/correctness property being proved

8. Parameter-binding tests
   These are CRITICAL.
   Explicitly specify tests for:
   - substitute repo
   - substitute workflow
   - substitute ref
   - substitute any provider parameter
   - caller-supplied matching parameters
   For every substitution test:
   - exact substitution mechanism
   - expected authorization/result
   - fake provider Calls() must be inspected
   - provider call count MUST be zero for rejected substitutions

9. Executor-selection tests
   Explicitly prove that caller-controlled `tool_name` cannot select an arbitrary registered executor.
   The test must fail if executor selection is again derived from caller input.

10. TOCTOU tests
    Precisely distinguish:

    A. Revocation BEFORE final authorization check:
       - must deny
       - zero provider calls

    B. Revocation AFTER final authorization check but BEFORE executor invocation:
       - characterize the documented v1 race
       - provider call MAY occur
       - do NOT claim prevention

    The test specification MUST explain that Window B exists across an independent REST revoke and that its acceptance rationale is the narrow local boundary:
    successful final authorization check → immediate local executor invocation,
    with no intervening I/O/database round-trip.

    Do NOT justify Window B using sequential MCP processing.

11. CompleteIntent tests
    Prove:
    - provider acceptance causes live → executed
    - provider rejection does not cause executed
    - ambiguous provider outcome does not falsely establish acceptance
    - no public/caller-controlled path invokes CompleteIntent
    - call provenance is the trusted execution path only

12. Audit tests
    Specify exact audit entries and ordering:
    authorization → invocation → provider result
    Also specify truthful behavior when audit writing fails after provider acceptance.

13. Duplicate execution
    Distinguish:
    - sequential duplicate prevention
    - concurrent duplicate execution as an explicitly documented v1 limitation

14. Traceability matrix

Use exactly this conceptual structure:

    Invariant | Test Function | Exact Assertion | What Breaks If Removed

Every material security/correctness invariant must map to one or more concrete tests.

15. Adversarial testing
    Include tests demonstrating that removing/bypassing each major security control causes the relevant test to fail.

16. Acceptance criteria
    Define the exact conditions required for the implementation gate to pass.

IMPORTANT PROCESS RULES:

- Documentation only. No production implementation.
- Do not modify kernel/service/adapter behavior.
- Do not add schema changes.
- Do not create a new service.
- Do not generalize the executor architecture.
- Do not add async polling, retries, multiple providers, RBAC, or remote execution APIs.
- Do not soften the known TOCTOU limitation.
- Do not replace provider acceptance with provider completion.
- Do not merely copy the plan sections. Turn them into an executable test specification with concrete test names, assertions, synchronization, and traceability.

After creating the file:

1. Verify the document exists.
2. Check it against every requirement above.
3. Report exactly what was created.
4. STOP.

Do not implement Step 1 or any production code.

The next action after you stop is to hand the CURRENT repository to Kilo Code for the independent Test Engineering Gate review.
```

This is precisely what Kilo requested: the missing file plus concrete test-level detail and invariant traceability. 

So the pipeline is now clean:

**OpenCode creates test target → Kilo Gate 1 reviews it → only after GO does OpenCode implement.**
