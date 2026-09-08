Perform a focused pre-kernel-freeze architectural review of the CURRENT repository.

REVIEW ONLY.
Do not modify code, schema, tests, documentation, or configuration.

This review has TWO related objectives:

1. Determine whether Solvent's debt vocabulary is genuinely domain-agnostic.
2. Determine what the actual debt representation is and whether it requires any finite technical resource limit.

Do not assume from documentation or prior discussion that debt is stored as an array. Verify the actual implementation, schema, serialization, lifecycle, and API paths first.

The architectural goal is:

> The Solvent kernel must be domain-agnostic.
> It should enforce generic debt/obligation lifecycle and the invariant that unresolved
> debt prevents promotion, without encoding the meaning of individual debt items.

Also:

> A technical resource bound, if genuinely required for safety, must not become a
> semantic/domain limit disguised as a kernel rule.

## 1. Trace the actual debt representation

Determine exactly how debt is represented today.

Inspect:

- Go types
- database schema
- SQL queries
- serialization/deserialization
- REST API
- MCP tools
- service layer
- kernel layer
- tests
- fixtures
- documentation

Determine which of these is actually true:

A. JSON/JSONB array stored directly on a row
B. SQL array
C. normalized debt/obligation rows
D. another representation
E. hybrid representation

Show the actual type/schema.

For example, determine whether it is conceptually:

    []string
    []Debt
    JSONB array
    SQL array
    debt rows keyed to belief
    map/set
    opaque obligation identifiers

Do not infer the answer from terminology.

## 2. Identify the debt vocabulary

Find the exact source of the debt vocabulary.

Search the entire repository for:

- hardcoded debt item names
- constants
- enums
- switch statements
- maps from debt → meaning
- default debt lists
- SQL CHECK constraints
- JSON schema/OpenAPI enums
- MCP argument restrictions
- test fixtures
- documentation defining a canonical debt list

For every vocabulary item found, classify it as:

- kernel semantic vocabulary
- kernel representation only
- service/policy vocabulary
- adapter vocabulary
- demo/example vocabulary
- test fixture only
- documentation/example only

The critical question is:

> Does the kernel understand the meaning of the debt item, or does it merely store/manage an opaque obligation identifier and enforce its lifecycle?

## 3. Determine whether debt vocabulary is hardcoded into the kernel

Look specifically for code equivalent to:

    case "specific_debt_name":
        ...

or:

    defaultDebts = []string{...}

or:

    CHECK (debt_type IN (...))

or any other construct where the kernel itself knows a fixed domain vocabulary.

A finding is architectural coupling only when the kernel must know what an obligation means.

A generic kernel may safely store:

    debt_key = "counterparty_review"

or:

    obligation_id = "<opaque identifier>"

provided the kernel does not attach domain semantics to that identifier.

## 4. Verify the actual promotion invariant

Determine exactly what the kernel and database enforce.

The preferred invariant is conceptually:

    unresolved_debt > 0
        → promotion denied

    unresolved_debt == 0
        → debt does not block promotion

The kernel should not need to know:

    "debt A means X"
    "debt B means Y"
    "a normal workflow has 3 debts"
    "a security workflow has 6 debts"

unless there is a demonstrated security reason.

Document the actual SQL/constraint implementation.

## 5. Investigate whether a technical debt-count limit is necessary

Do NOT assume an upper bound is required.

Determine whether the actual representation creates a resource/security concern such as:

- unbounded JSON payload
- unbounded SQL array
- transaction amplification
- excessive row count
- expensive serialization/deserialization
- pathological memory consumption
- database row-size limits
- API request amplification
- CPU cost proportional to debt count
- attack through arbitrarily large debt collections

Separate:

### Semantic limit

Example:

    "A belief may contain at most 10 debts."

This is a domain/workflow rule and should NOT be placed in the kernel merely for discipline.

### Technical resource limit

Example:

    "The kernel/API will reject an input that attempts to create an
     excessively large debt collection."

This may be justified as resource protection.

## 6. Determine where a technical limit belongs

If a finite technical limit is required, determine the correct enforcement layer.

Consider:

A. kernel
B. database
C. service/API boundary
D. transport/request limit
E. no explicit limit required

Use this principle:

> Put resource limits at the lowest layer that genuinely needs them for safety,
> but do not turn a resource limit into domain semantics.

Prefer a service/API/request boundary when that sufficiently protects the system.

A kernel-level limit should require an explicit security/resource argument.

Do NOT choose an arbitrary number such as 6, 10, 20, or 100 merely because it "sounds reasonable."

If a technical ceiling is justified, determine an evidence-based value from:

- actual schema constraints
- actual database limits
- actual payload characteristics
- measured cost
- existing request limits
- threat model
- implementation complexity

If there is insufficient evidence to justify a number, say so.

Do not invent precision.

## 7. Explicitly investigate the "100 smells like leakage" concern

Evaluate this architectural proposition:

> A kernel constant such as `MaxDebts = 100` can be suspicious if the number is intended to
> represent workflow discipline rather than resource safety.

Determine whether the current codebase has evidence that a numeric ceiling would be:

- a legitimate technical guardrail
- an arbitrary product/domain assumption
- unnecessary because another layer already has an adequate bound
- necessary only at the input/transport layer

Do not reject a limit merely because it is 100.
Do not accept a limit merely because it is finite.

The question is:

> What security/resource property does the number enforce?

If the answer is "good workflows should not have that many debts," it does not belong in the kernel.

## 8. Test domain portability conceptually

Determine whether the same kernel can support radically different vocabularies without kernel modification.

Use examples only as conceptual probes:

Finance:
    counterparty_review
    sanctions_check
    limit_check

Healthcare:
    consent_review
    clinical_validation
    privacy_check

Security:
    source_validation
    exploitability_review
    impact_review

Software deployment:
    artifact_verified
    change_review
    rollback_plan

The kernel should care only that unresolved obligations remain.

It should not need to understand those names.

## 9. Inspect API/MCP contracts for accidental vocabulary freezing

Determine whether REST or MCP currently exposes debt as:

- free/opaque identifiers
- a fixed enum
- a fixed list
- domain-provided values

Determine whether the public contract accidentally turns today's vocabulary into Solvent's permanent vocabulary.

If so, classify whether that is:

- real architectural coupling
- merely an example/fixture
- a documentation issue
- a service-layer concern

## 10. Inspect tests for hidden domain assumptions

Search for tests that assume:

- exact debt names
- exact number of debt items
- exact debt ordering
- fixed initialization
- a particular domain meaning

Classify each test as:

- kernel invariant test
- generic lifecycle test
- service policy test
- domain fixture

The kernel test suite should ideally prove generic behavior such as:

    unresolved debt → promotion blocked
    all debt resolved → promotion allowed

rather than proving a particular industry vocabulary.

## 11. Kernel Growth Gate

If fixing domain coupling requires a kernel change, determine whether that change truly belongs in the kernel.

Apply the established rule:

> Kernel changes require a genuinely new durable security fact or atomic security
> transition that cannot be safely expressed outside the kernel.

Do NOT expand the kernel merely to:

- support more debt names
- support domain configuration
- add policy engines
- add scoring
- add workflow abstractions
- add generic orchestration
- make the API more convenient
- impose product-style discipline

If the existing representation is already generic, prefer leaving the kernel unchanged.

If a technical resource bound is needed but can safely live at the service/API boundary, keep it there.

## 12. Important distinction

Do NOT conflate:

    domain-agnostic debt vocabulary
with:
    unlimited debt cardinality

A generic system can have:

    opaque obligation identifiers
    + finite technical resource limits

without becoming domain-specific.

Likewise, a system can have:

    no explicit debt-count limit

if the underlying representation and surrounding resource controls make that safe.

The review must establish which case the actual repository supports.

## 13. Required output

### A. Verdict

Choose exactly one:

    GREEN
    GREEN WITH MINOR CLEANUP
    NO-GO

Definitions:

GREEN:
- debt vocabulary is domain-agnostic
- representation is suitable
- no kernel semantic coupling exists
- no additional kernel change is required for this issue

GREEN WITH MINOR CLEANUP:
- kernel is already domain-agnostic
- only tests/docs/examples/API presentation imply a fixed vocabulary
- cleanup does not require kernel redesign

NO-GO:
- kernel contains real domain-specific debt semantics
- or the representation creates an unresolved security/resource issue
- and correction is required before kernel freeze

## 14. Required evidence

Provide:

### 1. Actual debt representation

Exact:

    type/schema/table
    fields
    storage format
    lifecycle

### 2. Actual vocabulary inventory

For every discovered debt item:

    name
    location
    layer
    semantic or fixture-only

### 3. Hardcoding assessment

For every hardcoded dependency:

    file
    symbol/line
    layer
    architectural significance

### 4. Resource-bound assessment

Answer:

    Is the debt collection actually unbounded?

    If yes, what concrete attack/resource problem results?

    Is an explicit finite limit technically required?

    If yes, which layer should enforce it?

    What evidence supports the limit?

    If no evidence supports a numeric ceiling, say so explicitly.

### 5. Domain portability test

Answer explicitly:

> Could finance, healthcare, security, deployment, and unrelated applications use
> the same Solvent kernel with different debt vocabularies without modifying kernel code?

### 6. Kernel-freeze recommendation

If GREEN or GREEN WITH MINOR CLEANUP, state:

> The debt model is sufficiently domain-agnostic and sufficiently resource-safe to proceed to kernel freeze.

If NO-GO:

- identify the exact defect
- identify the minimum correction
- state whether kernel growth is actually required

## 15. Hard constraints

- Review only.
- No code changes.
- No schema changes.
- No arbitrary numeric limit.
- No assumption that debt is an array.
- No domain-specific debt semantics in the kernel.
- No workflow abstraction.
- No policy engine.
- No idempotency work.
- No unrelated kernel expansion.
- Do not turn "discipline" into a kernel constant.

The objective is not to make debt unlimited.
The objective is to make the kernel generic while protecting actual technical resources where necessary.

Use the CURRENT repository as the source of truth.