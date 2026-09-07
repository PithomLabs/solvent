Now that **Phase 4C+ is independently GO**, I would make the next milestone **Phase 4D: Idempotent Consequential Execution + API Completion**.

The key is not to pile on features. Phase 4C proved the authority-to-real-side-effect path. Phase 4D should solve the most important known operational weakness: **the gap between an external side effect and Solvent's ability to know whether it has already happened**, especially around concurrent execution and post-provider persistence failure. The final review explicitly identifies concurrent duplicate execution and `CompleteIntent` persistence failure as accepted v1 limitations. 

Use this prompt:

```text
PHASE 4D — IDEATION / IMPLEMENTATION PLAN ONLY
Idempotent Consequential Execution + API Completion

You are now planning the next Solvent milestone after Phase 4C+ received
independent adversarial GO.

IMPORTANT:

Phase 4C+ is COMPLETE.

Do not reopen settled Phase 4C architecture unless you discover a concrete
security contradiction.

The current authority architecture is trusted and must remain the foundation.

==================================================
PHASE 4C+ BASELINE
==================================================

The following are now established and should be treated as fixed unless a real
security defect requires otherwise:

- kernel.Authorize is the sole authority oracle
- TOKEN != AUTHORITY
- Authorize != Execute
- approved snapshot consequence_parameters are the sole provider execution source
- caller execution parameters are ignored
- caller tool_name/executor selectors cannot choose arbitrary executors
- executor selection is fixed from authorized action
- exact intentID is propagated through execution
- CompleteIntent is the only added kernel lifecycle primitive
- CompleteIntent occurs only after provider acceptance
- CI-2 is authoritative:
      executor nil error => result.Success == true
- provider acceptance does not mean workflow completion
- Window A revocation is denied
- Window B remains a documented v1 TOCTOU limitation
- GitHub semantics remain in adapter/github
- executor cannot mutate authority state
- no second authorization engine exists
- no public CompleteIntent endpoint exists
- no unnecessary kernel primitives
- no unnecessary schema semantics
- kernel remains the smallest trusted authority core

Independent review result:

    GO

Do not redesign these properties.

==================================================
WHY PHASE 4D EXISTS
==================================================

The final Phase 4C+ review explicitly accepted these as v1 limitations:

1. Concurrent duplicate execution is possible.

2. CompleteIntent persistence failure after provider acceptance can leave the
   intent live, allowing a later duplicate external execution.

3. REST/MCP authorization currently supplies empty `{}` consequence parameters,
   meaning those surfaces cannot authorize targets whose approved snapshots
   contain GitHub-specific consequence parameters.

4. HTTP provider has no explicit timeout.

5. SetTestHook is still an exported test seam and should be cleaned up if
   appropriate.

Phase 4D should address the operationally important problems, but do so with the
smallest architecture that solves them.

==================================================
PRIMARY PHASE 4D OBJECTIVE
==================================================

Design a robust idempotent consequential-execution model.

Desired property:

    one logical authorized intent
        =>
    at most one accepted external execution request

while remaining honest about the inability to atomically coordinate a database
transaction with an external provider.

Do NOT claim impossible distributed atomicity.

The design must explicitly reason about:

    Solvent state
        +
    provider state
        +
    retries
        +
    crashes
        +
    timeouts
        +
    ambiguous provider responses
        +
    concurrent callers

==================================================
QUESTIONS THE PLAN MUST ANSWER
==================================================

1. How do we prevent two concurrent ExecuteAction calls from both reaching the
   provider?

2. How do we prevent a second provider call after:
       provider accepted
       CompleteIntent failed
       first caller crashed?

3. What state should represent:
       prepared
       executing
       provider accepted
       completed
       ambiguous
   ?

4. Can an execution lease / compare-and-swap state transition solve concurrency
   without putting provider semantics into the kernel?

5. Which parts belong in:
       kernel
       service
       executor
       adapter
       database invariant

6. Does idempotency require a new kernel primitive?

If yes, prove why it cannot be safely expressed in the service layer.

Do NOT add a kernel primitive merely because it is convenient.

7. Does idempotency require schema changes?

If yes, identify the smallest possible schema change and justify its durable
security meaning.

8. How does retry work after:
       provider error
       provider timeout
       lost response
       Solvent persistence failure

9. How do we distinguish:
       definitely not executed
       definitely accepted
       unknown / ambiguous

10. How does GitHub's workflow_dispatch behavior affect our idempotency design?

Do not assume the external provider offers exactly-once semantics.

==================================================
REQUIRED FAILURE MATRIX
==================================================

Build a complete state/failure matrix covering at least:

A. two simultaneous ExecuteAction calls

B. provider rejects

C. provider times out before acceptance is known

D. provider response is lost after the provider accepted

E. provider accepts and CompleteIntent succeeds

F. provider accepts and CompleteIntent fails

G. process crashes after provider acceptance but before Solvent persistence

H. process crashes before provider invocation

I. revoke before final authorization

J. revoke during Window B

K. retry after ambiguous result

L. retry after known provider acceptance

For each case specify:

- Solvent state before
- provider state known/unknown
- permitted retry behavior
- whether another provider invocation is allowed
- final user-visible result
- audit semantics

==================================================
CONCURRENCY DESIGN
==================================================

Investigate at least these approaches:

- SQL compare-and-swap
- row locking
- execution lease
- unique idempotency key
- explicit execution state
- provider-side idempotency where available

Compare them against:

- correctness
- race resistance
- crash recovery
- ambiguity handling
- kernel complexity
- schema complexity
- operational simplicity

Select the smallest design that materially improves the known v1 limitation.

Do not build a generic distributed workflow engine.

==================================================
KERNEL BOUNDARY
==================================================

Be extremely conservative.

Ask:

Is idempotency an authority fact?

Or is it execution coordination?

Preferred principle:

    authority semantics → kernel
    execution coordination → service/executor layer
    provider semantics → adapter

Only move something into the kernel if there is a durable security fact or
atomic transition that cannot be safely represented outside it.

==================================================
API / MCP CONSEQUENCE PARAMETERS
==================================================

Separately design how REST and MCP authorization should accept the same
consequence parameters that the approved target snapshot contains.

Current problem:

    REST/MCP authorization request
        →
    ConsequenceParameters = {}

Therefore GitHub targets containing:

    repo
    workflow
    ref

cannot currently be authorized through those surfaces.

Design the smallest coherent API change so that:

    target creation
        →
    approval
        →
    REST/MCP authorization
        →
    execution

all refer to the same approved consequence tuple.

Requirements:

- no second authority engine
- no bypass of kernel.Authorize
- exact tuple semantics preserved
- server-side identity binding preserved
- no caller-controlled execution parameter substitution
- API/MCP remain service interfaces over the same authority semantics

Do not implement the change yet.
Produce the contract first.

==================================================
HTTP PROVIDER HARDENING
==================================================

Evaluate whether Phase 4D should add:

    explicit HTTP timeout

Prefer a small deterministic timeout configuration.

Do not turn this into a generalized networking framework.

Also evaluate whether GitHub error response bodies should be truncated/sanitized
before being propagated into audit-visible errors.

These are lower priority than idempotency, but include them in the plan if
they are cheap and clearly justified.

==================================================
TEST STRATEGY
==================================================

Design the Phase 4D acceptance suite before implementation.

Tests must cover:

- concurrent duplicate execution
- compare-and-swap/lease correctness
- crash/retry semantics
- provider acceptance + persistence failure
- ambiguous provider response
- safe retry rules
- exact intent binding
- revocation interactions
- audit semantics
- API/MCP consequence parameters
- GitHub real provider behavior

Adversarial tests should attempt to:

- execute the same intent concurrently
- reuse an already executing intent
- retry after ambiguous response
- bypass idempotency through another API path
- alter provider parameters on retry
- substitute another intentID
- execute after revocation
- cause two executors to race
- make provider output become authority

Tests must prove security properties, not merely exercise code paths.

==================================================
DEFERRED / OUT OF SCOPE
==================================================

Do NOT add:

- generic workflow engine
- distributed transaction system
- exactly-once external execution claims
- generic orchestration platform
- arbitrary retries without state semantics
- Kubernetes executor
- AWS executor
- multi-tenancy
- RBAC/IAM
- enterprise compliance platform
- UI redesign
- agent memory/state platform
- generic event-sourcing framework

Those are separate concerns.

==================================================
DELIVERABLE
==================================================

DO NOT MODIFY PRODUCTION CODE YET.

Produce a detailed Phase 4D implementation plan.

The plan must contain:

1. Objective

2. Current v1 failure model

3. Proposed idempotency model

4. Exact execution state machine

5. Concurrency control mechanism

6. Crash recovery model

7. Ambiguous provider-result model

8. Retry rules

9. Kernel/service/executor/provider responsibility split

10. Required schema changes, if any

11. Required kernel changes, if any

12. API/MCP consequence-parameter contract

13. HTTP provider hardening

14. Acceptance tests

15. Adversarial tests

16. Migration/backward compatibility considerations

17. Observability/audit implications

18. Security invariants added by Phase 4D

19. Explicit non-goals

20. Risk analysis

==================================================
ARCHITECTURAL DECISION RULE
==================================================

Before proposing any new kernel primitive, service, schema table, or state:

ask:

    Is this a durable security fact?
    Is this an atomic transition?
    Can it be expressed safely outside the kernel?

If it can be expressed outside the kernel, keep it outside the kernel.

The objective is:

    improve consequential execution reliability
    without turning Solvent into a giant orchestration system.

==================================================
FINAL PLAN QUALITY BAR
==================================================

The final plan must be implementable by a coding agent without inventing
security semantics.

It must explicitly state:

- what changes
- what does not change
- why each change belongs where it does
- exact state transitions
- exact concurrency guarantees
- exact limitations

Do not claim "exactly once" unless the design can actually establish it.

Prefer precise language such as:

    "at most one provider invocation under Solvent-controlled concurrency"

or:

    "provider invocation may be ambiguous after crash; retry is prohibited
     until external state is reconciled"

over unjustified exactly-once claims.

STOP after producing the plan.
```

I would make **idempotency the centerpiece**, rather than immediately expanding Solvent into more integrations. The final review has effectively given us the roadmap: the authority model is working; now the next engineering question is how to make a consequential action **safe to retry and safe under concurrency** without corrupting the simplicity of the kernel. 

And I would treat the REST/MCP consequence-parameter issue as the second track of the same phase, because otherwise the GitHub capability is proven internally but awkward through the actual agent-facing interfaces. 
