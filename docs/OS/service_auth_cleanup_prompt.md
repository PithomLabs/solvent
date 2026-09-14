Implement the remaining NON-KERNEL Solvent cleanup identified by the latest adversarial freeze review.

IMPORTANT:
- This is post-kernel-freeze work.
- DO NOT modify kernel semantics.
- DO NOT add kernel primitives.
- DO NOT modify kernel schema/invariants.
- DO NOT reopen the Kernel Growth Gate.
- DO NOT implement Oracle, physics-verifier, swarm, harness, attestation-policy, or domain-policy work.
- Keep all changes in service/API/documentation/tests unless repository evidence requires otherwise.
- Use the CURRENT repository at HEAD as the source of truth.

============================================================
1. PRIMARY OBJECTIVE
============================================================

Close the remaining LOW service-layer authorization weakness identified
by the adversarial review:

    REST RetireDebt / Discharge are authenticated but not authorized.

Current behavior:
- An authenticated principal can retire debt on a belief without a policy-level
  authorization decision.
- An authenticated principal can discharge an obligation without verification
  that the supplied discharged_by identity matches the authenticated principal.
- The kernel intentionally remains generic and does not enforce principal-level
  policy.

This must be fixed OUTSIDE the kernel.

============================================================
2. RETIREDEBT AUTHORIZATION
============================================================

Inspect the current REST path:

    api/belief.go
        handleRetireDebt
            → service/kernel

Determine the repository's existing authentication and authorization conventions.

Implement the minimum service/API-level authorization needed so that an
authenticated caller cannot arbitrarily retire another actor's debt unless
the current application's policy explicitly allows it.

IMPORTANT:
- Do not invent a broad RBAC system.
- Do not create a new policy engine.
- Do not add kernel actor identity semantics.
- Reuse existing authentication/context information and any existing
  authorization conventions already present in the repository.

Before coding:
- inspect AuthFromContext / authenticated principal handling
- inspect existing authorization checks on other endpoints
- inspect existing principal/scenario/belief relationships
- determine what the repository can actually prove about ownership or authority

If there is insufficient existing identity/ownership structure to establish
a stronger ownership rule safely, do NOT invent one.

Instead implement the narrowest defensible authorization rule supported by
the existing architecture and document the limitation.

============================================================
3. DISCHARGE AUTHORIZATION
============================================================

Inspect:

    api/discharge.go

Current problem:
- `discharged_by` is caller-supplied.
- The handler does not verify it against the authenticated principal.

Required behavior:

- Do not blindly trust caller-supplied `discharged_by`.
- Prefer deriving the effective actor from authenticated request context.
- If the public API must retain `discharged_by` for compatibility, validate
  that it matches the authenticated principal.
- Reject mismatches.
- Do not add actor semantics to the kernel.

Determine the appropriate HTTP error semantics from existing conventions.

A caller attempting to impersonate another principal should receive an
authorization/authentication error, NOT a generic authorization-denied
kernel result.

Do not conflate this with the existing Solvent authorization model for
consequential action execution.

============================================================
4. IMPORTANT KERNEL BOUNDARY
============================================================

DO NOT change:

- RetireDebt kernel signature
- Discharge kernel signature
- belief schema
- debt schema
- promotion CHECK
- debt lifecycle
- authority lifecycle
- ClaimIntent
- exact authority binding
- scenario invariants
- existing DB constraints

The kernel continues to treat:
- debt identifiers as opaque
- discharged_by as opaque attribution data
- actor authorization as an outer-layer concern

The service/API layer is responsible for deciding whether a caller may
perform the operation.

============================================================
5. NIL INITIAL-DEBT SEMANTICS
============================================================

Also inspect the recently parameterized:

    EnterBelief(..., initialDebt []string)
    EnsureBelief(..., initialDebt []string)

The latest review established:

    nil []string
        → SQL NULL
        → belief.debt NOT NULL
        → DDL DEFAULT
        → current FullDebt

while:

    []string{}
        → empty SQL array
        → no debt

Before changing anything, verify all current callers and actual semantics.

DO NOT change this behavior automatically.

Determine whether the current repository contract explicitly requires:
- nil to mean default vocabulary
- nil to be rejected
- nil to be treated as empty

If current behavior is deliberate and all supported callers pass explicit
arrays, document it and leave it unchanged.

Only change nil semantics if the current architecture clearly establishes
that the present behavior is accidental or unsafe.

If a change is justified:
- keep it outside unrelated kernel redesign
- add explicit regression tests
- preserve existing caller behavior unless deliberately changed

============================================================
6. TEST REQUIREMENTS
============================================================

Add DB-backed/service/API tests for the authorization correction.

At minimum:

### RetireDebt

1. Authenticated authorized caller → allowed
2. Authenticated unauthorized caller → rejected
3. Cross-scenario attempt remains rejected
4. No debt mutation occurs on authorization failure
5. Existing successful retirement behavior remains unchanged

### Discharge

1. Authenticated caller using own identity → allowed
2. Caller supplies a different principal ID → rejected
3. Effective actor is derived/validated from authentication context
4. No discharge row is created on impersonation failure
5. Existing successful discharge behavior remains unchanged

### Audit semantics

Ensure a policy/authentication rejection is not incorrectly recorded as:
- successful discharge
- successful debt retirement
- authorization granted

Do not create an `ActivityAuthorizationDenied` event merely because a
service-layer request was rejected unless existing audit semantics explicitly
define that event for this operation.

Keep state-conflict and security-refusal semantics distinct.

### Regression

Run existing debt, promotion, scenario-isolation, and authorization tests.

============================================================
7. PUBLIC API COMPATIBILITY
============================================================

Preserve existing REST request/response shapes unless a compatibility break
is necessary.

Do not expose new:
- role systems
- policy configuration APIs
- capability-token APIs
- actor registries
- swarm APIs

This task is only about closing the already identified service authorization
gap.

============================================================
8. DOCUMENTATION
============================================================

Update the relevant Solvent documentation to state clearly:

- kernel debt operations are generic and do not perform principal-level policy
  authorization
- REST/service boundaries are responsible for caller authorization
- `discharged_by` is authenticated/validated at the service boundary
- this does not change kernel semantics
- this is a service-layer security control

Do not rewrite the overall architecture documentation.

============================================================
9. VERIFICATION
============================================================

Run:

    gofmt -l cmd internal kernel api service adapter
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task db:reset
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Also run targeted tests for:
- RetireDebt authorization
- Discharge actor validation
- cross-scenario isolation
- no-mutation-on-denial
- nil/empty initialDebt behavior if that behavior is touched

Do not treat "tests pass" as sufficient.
Inspect the final diff for accidental kernel changes.

============================================================
10. KERNEL-FREEZE PROTECTION
============================================================

At the end of the implementation, verify:

    kernel primitives added: 0
    kernel invariants changed: 0
    schema changes: 0
    migration changes: 0

unless the repository proves that one of these is genuinely unavoidable.

If implementation appears to require kernel growth:
STOP.

Do not implement it.
Report:
- the exact requirement
- why service/API enforcement cannot satisfy it
- why DB enforcement cannot satisfy it
- what durable security fact would be missing

This task should normally require NO kernel change.

============================================================
11. OUT OF SCOPE
============================================================

Absolutely do NOT implement:

- Oracle
- physics verifier
- AI swarm
- harness
- attestation framework
- semantic applicability
- policy-version architecture
- human-review workflows
- capability-token framework
- remote MCP
- authentication redesign
- RBAC framework
- idempotency
- new debt model
- new kernel primitives
- new database tables

Those are separate architectural work.

============================================================
12. FINAL OUTPUT
============================================================

Report:

### A. Files changed

Exact file list and purpose.

### B. Security correction

Explain how the REST RetireDebt/Discharge authorization gap was closed.

### C. Kernel boundary

Explicitly confirm:

    No kernel semantics changed.
    No new kernel primitives.
    No schema/migration changes.

### D. Nil semantics

State whether nil initialDebt was:
- left unchanged and documented, or
- changed, with the exact reason.

### E. Test evidence

Give the actual commands and results.

### F. Final status

Conclude with:

    POST-FREEZE SOLVENT CLEANUP COMPLETE

only if all requested fixes and verification pass.

Otherwise report exactly what remains.
