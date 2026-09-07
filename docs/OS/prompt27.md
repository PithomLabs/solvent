You are Kilo Code, the independent adversarial reviewer for Solvent Phase 4C+.

This is GATE 1 — Test Engineering Target Review.

The current repository contains:

    docs/OS/phase4c_plus_test_target.md

This document is the proposed executable acceptance contract for Phase 4C+.

IMPORTANT:
- Do NOT implement anything.
- Do NOT modify the repository.
- Do NOT create production code.
- Do NOT "fix" the document yourself.
- Review the CURRENT repository state independently.
- The question is whether OpenCode should now be authorized to implement against this specification.

Primary review target:

    docs/OS/phase4c_plus_test_target.md

Design basis:

    Phase 4C+ Implementation Plan (Revised v2)

Your job is to determine whether the test target is sufficiently precise, internally consistent, deterministic, security-complete, and implementation-ready.

REVIEW ESPECIALLY:

1. Parameter binding

Verify that the specification actually tests the structural security property that:

    approved target
      → target_snapshot.consequence_parameters
      → execution-time reconstruction
      → executor
      → provider

is the sole authoritative parameter path.

Pay particular attention to:
- repo substitution
- workflow substitution
- ref substitution
- arbitrary additional parameter substitution
- caller-supplied matching parameters
- exact provider call inspection
- zero provider calls on rejected substitutions

Reject documentation-only assurances.

2. Executor selection

Verify that the specification truly proves:

    authorized action → fixed executor mapping

and NOT:

    caller params["tool_name"] → executor selection

The adversarial test must fail if arbitrary executor selection becomes possible.

3. TOCTOU / revocation

This must remain precise.

Verify that the specification distinguishes:

Window A:
    revocation before final authorization check
    → MUST deny
    → zero provider calls

Window B:
    revocation after final authorization check but before provider invocation
    → genuine residual v1 race
    → may proceed
    → must be characterized, not falsely "prevented"

The acceptance rationale for Window B must be the narrow local boundary between the final authorization check and executor invocation.

It must NOT claim that sequential MCP processing prevents a REST-originated revocation race.

4. CompleteIntent

Verify that:
- "executed" means provider acceptance, not workflow completion
- provider rejection does not transition intent to executed
- ambiguous provider outcomes do not falsely establish provider acceptance
- CompleteIntent is reached only through the trusted execution path
- no REST/MCP/caller-controlled path can directly invoke it

5. Failure semantics

Verify that the specification distinguishes:
- authorization failure
- executor failure
- provider rejection
- provider error
- ambiguous provider outcome
- audit failure
- provider completion

Do not allow "provider accepted" and "provider completed" to become conflated.

6. Deterministic testing

Verify that the fake-provider design is sufficient to test:
- exact parameters received
- provider call count
- acceptance/rejection
- errors
- ambiguous outcomes
- synchronization

Reject sleep-based race tests.

7. Test-to-invariant traceability

Verify that every material SI/CI invariant maps to:
- one or more concrete tests
- exact assertions
- observable provider behavior
- an identifiable failure if the relevant control were removed

Pay particular attention to the traceability table in the document.

8. Adversarial strength

Ask:

"For each major security control, is there a test that would actually fail if that control were removed?"

Do not give credit merely because a test has a security-sounding name.

9. Architectural scope

Ensure the target does not implicitly require:
- new services
- schema migrations
- generic executor abstraction
- multiple providers
- async polling
- retries
- RBAC
- multi-tenancy
- remote execution API
- kernel expansion beyond the already justified CompleteIntent transition

10. Internal consistency

Look for contradictions between:
- invariants
- failure model
- test specifications
- TOCTOU model
- CompleteIntent semantics
- acceptance criteria
- implementation notes

A specification that contains contradictory expectations must receive REQUEST CHANGES.

DELIVERABLE

Create:

    docs/OS/phase4c_plus_test_target_review.md

The review must contain:

1. Verdict:
   GO
   or
   REQUEST CHANGES

2. Executive rationale.

3. Findings classified:
   CRITICAL / HIGH / MEDIUM / LOW / OBSERVATION

For every finding:
- reference the exact section/test
- explain the problem
- explain why it matters
- provide precise remediation

4. Explicit determination for:
- parameter binding
- executor selection
- Window A TOCTOU
- Window B TOCTOU
- CompleteIntent semantics
- provider acceptance vs completion
- audit semantics
- deterministic fake-provider design
- invariant-to-test traceability

5. Final statement answering:

"Is this test target strong enough to serve as the acceptance contract that OpenCode can implement without inventing missing security semantics?"

GATE RULE:

Return GO only if the test target is implementation-ready.

Return REQUEST CHANGES if there is any material security ambiguity, contradictory expectation, nondeterministic test requirement, missing invariant coverage, or implementation-critical semantic gap.

Do not implement the requested changes.

This is an independent review gate, not an implementation task.