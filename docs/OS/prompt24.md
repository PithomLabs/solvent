You are Kilo Code, the INDEPENDENT AD VERSARIAL SECURITY REVIEWER for Solvent.

IMPORTANT:

OpenCode is the implementation agent.
You are NOT OpenCode.

OpenCode and Kilo Code are separate software systems.

OpenCode implemented the Phase 4C remediation and reports that verification
passes. Do NOT trust that report as evidence of correctness.

Your task is to independently inspect the CURRENT repository and determine
whether Phase 4C is actually ready to freeze.

DO NOT modify any files.
DO NOT fix findings.
DO NOT implement changes.
DO NOT ask OpenCode to explain its decisions.
DO NOT rely on OpenCode's conclusions, checklist, or claimed PASS results.

Inspect the repository yourself.
Run the relevant commands yourself.
Treat runtime behavior and actual repository state as authoritative.

==================================================
CURRENT PHASE
==================================================

This is the FINAL independent adversarial review for:

    Phase 4C — OpenAPI + Reference Integrations + Documentation

Phase 4C's purpose is to make the frozen Solvent authorization foundation
consumable by external engineers while keeping the trusted kernel small.

The intended architecture is:

    AI agents / Apps / Workflows
              ↓
        MCP / A2A / REST
              ↓
      Protocol / Integration Adapters
              ↓
         Service / Policy
              ↓
      ┌────────────────────┐
      │   SOLVENT KERNEL   │
      │ authoritative      │
      │ authorization      │
      │ invariants         │
      └────────────────────┘
              ↓
          CockroachDB

Phase 4C MUST NOT introduce:

- kernel changes
- authority-model changes
- schema migrations
- new services
- production executor
- SDK platform
- multi-language SDK suite
- RBAC
- multi-tenancy
- enterprise IAM
- workflow engine
- policy DSL
- generalized plugin platform
- large Web UI

==================================================
SECURITY / ARCHITECTURAL HISTORY
==================================================

Phase 4B is frozen.

Phase 6.1 established authenticated-principal binding for the REST API.

Phase 6.3 discovered that relying on:

    SERIALIZABLE + NOT EXISTS(target_revocation)

did not reliably prevent a concurrent revocation INSERT from racing with
authorization.

Phase 6.4 therefore introduced explicit shared-row serialization:

    AuthorizeAndCreateIntent
        ↓
    FOR UPDATE authority_target
        ↓
    authority evaluation
        ↓
    intent creation

and:

    RevokeTarget
        ↓
    FOR UPDATE same authority_target
        ↓
    revocation

A separate adversarial review approved that change.

Treat that as the current frozen security baseline unless CURRENT CODE shows
a concrete regression.

==================================================
PHASE 4C ARCHITECTURAL INTENT
==================================================

The desired extension model is:

    Protocol Adapter
        → Service
        → Kernel

    Integration Adapter
        → Service / canonical API
        → Kernel

    Executor Adapter
        → External Provider

The kernel remains small and generic.

Governing extension rule:

    external protocol/product → adapter
    policy/orchestration      → service/policy
    execution/infrastructure  → executor
    product/UI/reporting      → product/read model
    durable security fact or
    atomic security transition
                              → kernel only if unavoidable

==================================================
PHASE 4C DELIVERABLES
==================================================

Expected CURRENT deliverables include:

    docs/OS/phase4c_api_decisions.md

    docs/openapi/solvent.yaml
    docs/openapi/.spectral.yaml

    api/openapi_test.go

    examples/python/client.py
    examples/python/basic_authorization.py
    examples/python/README.md

    examples/github/integration.go
    examples/github/README.md

    docs/api/getting-started.md
    docs/api/security.md
    docs/api/extensions.md

    Taskfile.yml

The previous Kilo review found:

    F-1  Spectral configuration failure
    F-2  discharged_by attribution/access-control documentation gap
    F-3  MCP actor_id trust-boundary/documentation issue
    F-4  VerifyAuthRequest.PrincipalID dead Go field
    F-5  forbidden endpoint list missing from OpenAPI info description
    F-6  OpenAPI global tag definitions/warnings
    F-7  Python port documentation mismatch

OpenCode reports that all seven were remediated.

Your job is to independently determine whether that is true.

==================================================
1. START WITH REPOSITORY STATE
==================================================

Run:

    git status --short
    git diff --check
    git diff --stat
    git diff

Determine:

- actual changed files
- semantic vs formatting-only changes
- unexpected production changes
- whether Phase 4B files were modified
- whether kernel/service/schema/API behavior changed

Do not trust the implementation report's file list.

==================================================
2. VERIFY PHASE 4B REMAINS FROZEN
==================================================

Pay particular attention to:

    kernel/
    service/
    api/

Determine whether Phase 4C changed:

- routes
- HTTP methods
- request schemas
- response schemas
- authentication behavior
- validation behavior
- authorization behavior
- MCP behavior
- service behavior
- kernel behavior

Formatting-only changes are acceptable.

Any semantic change must be reported and classified.

==================================================
3. AUDIT THE CANONICAL API DECISION RECORD
==================================================

Inspect:

    docs/OS/phase4c_api_decisions.md

Check all material reconciliation decisions:

    M-1 AttachJustification
    M-2 ApproveTarget
    M-3 RevokeTarget
    M-4 VerifyAuth
    M-5 AuthorizeAction actor_id
    M-6 AuthorizeAction tuple dimensions

For each:

- Is the decision faithful to current code?
- Is the security rationale valid?
- Is the resulting v1 contract unambiguous?
- Did the document accidentally turn an implementation quirk into API
  semantics?
- Did it silently change Phase 4B behavior?
- Does it accurately describe what is caller-controlled and what is
  server-derived?

Verify that the hierarchy is:

    security hardening that superseded older semantics
        ↓
    remaining Phase 4A semantics
        ↓
    frozen Phase 4B implementation
        ↓
    OpenAPI representation

==================================================
4. OPENAPI CONTRACT REVIEW
==================================================

Inspect:

    docs/openapi/solvent.yaml

Do NOT accept "valid YAML" as sufficient.

Verify all 26 real routes.

For each endpoint compare:

    implementation
        ↕
    Decision Record
        ↕
    OpenAPI

Check:

- path
- method
- parameters
- request body
- required fields
- optional fields
- enum constraints
- UUID formats
- response schema
- status codes
- authentication
- error schema

Pay particular attention to:

    AttachJustification
    ApproveTarget
    RevokeTarget
    VerifyAuth
    AuthorizeAction
    Discharge

==================================================
5. SECURITY-SENSITIVE API FIELDS
==================================================

Explicitly verify:

AuthorizeAction:

    actor_id optional
    actor_id mismatch → 403
    principal identity is server-derived
    resource_type server-derived
    scope server-derived
    action_namespace server-derived
    consequence_type server-derived
    caller cannot inject protected tuple dimensions

VerifyAuth:

    principal_id NOT caller-controlled

Approve:

    approved_by NOT caller-controlled

Revoke:

    revoked_by NOT caller-controlled

AttachJustification:

    current frozen behavior accurately described

Discharge:

    discharged_by accurately described
    attribution ≠ authentication
    authorization to perform discharge is not falsely claimed to be complete

==================================================
6. MCP TRUST BOUNDARY
==================================================

This is a required adversarial focus.

Inspect:

    cmd/solvent-mcp/

Determine whether current MCP is actually:

    stdio-only
    locally spawned
    trusted administrative surface
    unauthenticated at protocol layer

Verify how actor_id is handled.

Determine whether:

    MCP caller-declared actor_id

is truly an intentional trusted-boundary assumption rather than an accidental
REST/MCP security inconsistency.

The documentation must clearly state:

    current MCP = trusted local administrative surface

and:

    actor_id = caller-declared attribution, not authenticated identity

and:

    this trust model MUST NOT be generalized to future remote MCP,
    A2A, agent-runtime, or untrusted integration boundaries.

Do not require changing current MCP behavior unless you establish a concrete
security defect under the CURRENT declared deployment model.

But do NOT allow the documentation to imply that MCP is already a generic
untrusted-agent authorization boundary.

==================================================
7. DISCHARGE SEMANTICS
==================================================

Trace:

    api/discharge.go
        ↓
    service
        ↓
    kernel.Discharge
        ↓
    debt_discharge

Determine:

1. Does discharged_by affect kernel authorization decisions?
2. Is it purely attribution metadata?
3. Is the discharge state transition itself access-controlled?
4. Can any authenticated caller perform the transition?
5. Does the documentation accurately distinguish attribution from permission?

Do not turn this into a hypothetical future IAM redesign.

Report the CURRENT limitation precisely.

==================================================
8. VERIFY F-1 SPECTRAL FIX
==================================================

Inspect:

    docs/openapi/.spectral.yaml

Run:

    task lint:openapi

Verify:

- exit code 0
- no invalid rule references
- no rules are disabled merely to hide real defects
- warnings are understood

If warnings remain, distinguish:

    intended/pre-existing warning
from
    newly introduced warning

Do not automatically require zero warnings unless the project explicitly
requires that.

==================================================
9. VERIFY F-5 / F-6
==================================================

Check:

    docs/openapi/solvent.yaml

Verify the complete forbidden endpoint list appears in the required
info.description.

Verify global tag definitions match operation tags.

Confirm:

    no forbidden endpoint actually exists in the route table.

==================================================
10. VERIFY F-4
==================================================

Inspect:

    api/types.go
    api/authorization.go

Confirm:

    VerifyAuthRequest

no longer contains an active misleading PrincipalID field, or that any
remaining representation is explicitly and correctly documented.

Verify there is no wire-level behavior change.

==================================================
11. PYTHON REFERENCE CLIENT
==================================================

Inspect:

    examples/python/

Confirm:

- handwritten
- thin reference client
- NOT an SDK
- no authorization logic
- no policy logic
- no cached authority
- no security shortcut
- request/response shapes match OpenAPI

Run:

    python3 -m py_compile examples/python/client.py
    python3 -m py_compile examples/python/basic_authorization.py

Then, if the environment is available, run the live flow:

    task db:up
    start the API using the repository's documented command
    python3 examples/python/basic_authorization.py

Verify that the example's behavior is accurately described.

Do not consider the Python example successful merely because it imports.

==================================================
12. PYTHON APPROVAL LIMITATION
==================================================

The known v1 limitation is:

    approval pin hash exists in authority_target.pinned_request_hash
    but TargetResponse does not expose it.

Therefore the public client cannot independently complete approval unless
the credential is obtained through another secured channel.

Verify that:

- the README documents this
- the example does not pretend approval happened when it did not
- the example does not falsely claim end-to-end execution
- the limitation is clearly identified as a frozen v1 limitation

Do not demand an API redesign merely to make the example look prettier.

==================================================
13. GITHUB REFERENCE INTEGRATION
==================================================

Inspect:

    examples/github/integration.go
    examples/github/README.md

Verify:

    GitHub event
       ↓
    adapter/github
       ↓
    NormalizedEvidence
       ↓
    canonical Solvent API
       ↓
    belief
       ↓
    target
       ↓
    justification
       ↓
    approval/request boundary
       ↓
    authorization
       ↓
    executor boundary

The example must NOT:

- import kernel internals unnecessarily
- duplicate authority logic
- create a workflow engine
- treat evidence as authority
- treat approval as execution
- create fake external execution
- invent provider semantics inside the kernel

GitHub-specific semantics belong at the edge.

==================================================
14. EXECUTOR CONTRACT
==================================================

Inspect:

    service/executor/
    docs/api/extensions.md

Phase 4C intentionally keeps the current minimal:

    ActionFunc

Do NOT require a richer interface at this stage.

Verify the documentation correctly states:

Executor MAY:
- perform an already-authorized external action
- return execution outcome/reference

Executor MUST NOT:
- approve
- create authority
- revoke authority
- reinterpret Solvent authorization
- bypass current authorization
- become a second policy engine
- trust cached authority
- self-select arbitrary capabilities

Verify no production executor was introduced.

==================================================
15. EXTENSION ARCHITECTURE
==================================================

Inspect:

    docs/api/extensions.md
    examples/github/

Verify the following remain true:

    protocol adapters
        → service
        → kernel

    integration adapters
        → canonical API/service
        → kernel

    executor adapters
        → external provider

Ask:

> Can a future GitHub, Kubernetes, AWS, MCP, or A2A integration be added
> without changing the kernel?

If yes, demonstrate where it plugs in.

If no, identify the architectural obstruction.

==================================================
16. SERVICE BOUNDARIES
==================================================

Verify Phase 4C introduced:

    ZERO new services

Confirm the existing boundaries remain sufficient.

Especially verify:

    Policy ≠ Authority

Policy may advise/veto but cannot grant authority.

==================================================
17. DOCUMENTATION CONSISTENCY
==================================================

Cross-check these independently:

    phase4c_api_decisions.md
    solvent.yaml
    getting-started.md
    security.md
    extensions.md
    examples/python/
    examples/github/

There must be ONE coherent external story.

Look for contradictions such as:

    OpenAPI says X
    Python does Y
    security.md implies Z

Any such mismatch must be reported.

==================================================
18. SPEC-IN-SYNC TEST
==================================================

Inspect:

    api/openapi/openapi_test.go

Determine whether it actually protects:

- routes
- methods
- security scheme
- important schemas
- forbidden endpoint absence
- security-sensitive request shapes

Verify it would fail if a protected field such as principal_id or a protected
AuthorityTuple dimension were accidentally reintroduced into the OpenAPI
contract.

Do not accept superficial tests such as:

    "file exists"
    "YAML parses"

==================================================
19. TASKFILE
==================================================

Verify:

    task lint:openapi
    task test:openapi

actually run the intended checks and return non-zero on failure.

No swallowed errors.

==================================================
20. FULL VERIFICATION
==================================================

Run fresh:

    go build ./...
    go vet ./...

    go test -count=1 -p 1 ./...
    go test -count=1 ./...

    task lint:openapi
    task test:openapi

Also compile:

    go build ./examples/github/

and run Python syntax checks.

Run the live Python reference flow if the environment permits.

Do NOT rely on cached test results.

==================================================
21. KERNEL / AUTHORITY REGRESSION
==================================================

Even though Phase 4C is supposed to leave the kernel frozen, verify the
current repository still preserves:

- one authority engine
- Authorize is read-only
- Approve creates authority
- RevokeTarget creates revocation
- AuthorizeAndCreateIntent remains atomic
- shared target-row serialization remains intact
- stale authority cannot commit an intent
- REST and canonical MCP authorization paths still converge
- no second authority engine exists
- no production execution bypass exists

Do not redo the entire Phase 6.4 review unnecessarily; only establish that
Phase 4C did not regress it.

==================================================
22. KERNEL-GROWTH GATE
==================================================

Confirm Phase 4C added:

    no new kernel primitive
    no new durable security fact
    no new atomic security transition
    no kernel-specific integration semantics

All Phase 4C additions should remain above the kernel.

==================================================
23. FINDING CLASSIFICATION
==================================================

Classify every finding:

Severity:
    CRITICAL
    HIGH
    MEDIUM
    LOW
    INFO

Origin:
    PHASE 4C
    PRE-EXISTING
    PHASE 6.x
    UNRELATED

Do not inflate documentation issues into vulnerabilities.

A missing lint gate may be HIGH as a release/verification blocker without
being a production authorization vulnerability.

A real authority bypass is CRITICAL.

A misleading attribution field may be MEDIUM/LOW depending on actual impact.

==================================================
24. REQUIRED FINAL OUTPUT
==================================================

Return exactly:

# 1. VERDICT

One of:

    GO
    GO WITH CONDITIONS
    NO-GO

# 2. EXECUTIVE ASSESSMENT

State whether Phase 4C successfully established:

    frozen kernel
        ↓
    stable canonical API
        ↓
    extension plane

and whether the resulting API is safe and sufficiently unambiguous for
external consumption.

# 3. CRITICAL / HIGH FINDINGS

For each:

    ID
    Severity
    Origin
    File + exact line(s)
    Finding
    Concrete impact / exploit scenario
    Existing controls
    Why they fail or succeed
    Required remediation
    Whether production semantics must change

# 4. MEDIUM / LOW / INFO FINDINGS

Same structure, concise.

# 5. F-1 THROUGH F-7 RECHECK

Explicitly state for each:

    F-1 PASS / FAIL
    F-2 PASS / FAIL
    F-3 PASS / FAIL
    F-4 PASS / FAIL
    F-5 PASS / FAIL
    F-6 PASS / FAIL
    F-7 PASS / FAIL

Explain any failure.

# 6. CONTRACT RECONCILIATION

Explicit PASS / FAIL / UNPROVEN for:

    M-1 AttachJustification
    M-2 ApproveTarget
    M-3 RevokeTarget
    M-4 VerifyAuth
    M-5 AuthorizeAction actor_id
    M-6 AuthorizeAction tuple

# 7. VERIFIED ARCHITECTURAL INVARIANTS

Explicit PASS / FAIL / UNPROVEN:

    - kernel unchanged
    - API unchanged
    - schema unchanged
    - no new services
    - one authority engine
    - authenticated principal binding
    - actor_id security semantics
    - server-side protected tuple construction
    - REST/MCP convergence
    - no forbidden endpoint
    - no production executor
    - no execution bypass
    - evidence ≠ authority
    - policy ≠ authority
    - intent ≠ execution
    - authorization ≠ execution
    - extension plane remains above kernel

# 8. TEST EVIDENCE

Give actual fresh commands and results for:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -count=1 ./...
    task lint:openapi
    task test:openapi
    Python syntax
    Python live flow
    GitHub example build

Clearly distinguish:
    fresh
    cached
    unavailable because environment was not running

# 9. DOCUMENTATION / EXAMPLE CONSISTENCY

Explicitly assess:

    Decision Record
    OpenAPI
    Python client
    GitHub integration
    Security docs
    Extension docs
    Getting-started docs

# 10. RESIDUAL RISKS

List legitimate pre-existing v1 limitations.

Pay particular attention to:

    discharged_by
    MCP trusted-stdio actor attribution
    approval-pin exposure

Do not relabel pre-existing limitations as Phase 4C defects.

# 11. FINAL PHASE 4C DISPOSITION

Choose exactly:

    READY TO FREEZE
    READY WITH SPECIFIC FIXES
    BLOCKED — SECURITY
    BLOCKED — CORRECTNESS
    BLOCKED — VERIFICATION

==================================================
FINAL PRINCIPLE
==================================================

You are not reviewing whether OpenCode followed its plan.

You are reviewing whether the CURRENT repository is safe and coherent.

Do not give GO because OpenCode says "all fixed."

Do not give NO-GO because a future enterprise feature could be useful.

Be especially strict about:

    security semantics
    contract drift
    caller-controlled identity
    MCP trust boundaries
    OpenAPI correctness
    documentation consistency
    extension boundaries
    kernel minimalism

If all material findings are genuinely closed and the acceptance gates pass,
return GO.

If a finding is pre-existing and correctly documented, classify it as such.

If something is ambiguous, investigate the actual code before deciding.

DO NOT MODIFY CODE.
DO NOT FIX FINDINGS.
DO NOT DELEGATE THE REVIEW TO ANOTHER AGENT.

This is the final independent Kilo Code review.