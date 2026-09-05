# Plan 4.2.1: Final Hardening Amendments to Plan 4.2

**Status:** APPROVED FOR IMPLEMENTATION — append to Plan 4.2.

**Trigger:** Two contract tightenings required before coding begins.

---

## Amendment 1: Authority Oracle Constraint (Section 15)

The service may retrieve the data required to construct the
`AuthorityTuple`, but MUST NOT independently evaluate whether
authority is valid.

The authoritative allow/deny determination MUST come from
`kernel.Authorize`.

Do not duplicate activation, revocation, snapshot, or exact-match
logic in `service/authority`.

## Amendment 2: Authorization/Execution TOCTOU Boundary (Section 16)

The system guarantees current authorization verification immediately
before executor invocation. It does not claim atomic coordination
between the Solvent database transaction and an external provider's
side effect. Such execution-effect coordination is out of scope for
this remediation and must not be simulated.

The correct security claim is:

> The executor is invoked only after a successful current
> authorization check immediately preceding execution.

Not:

> Revocation can never race with an external execution.

## Workflow

```
PLAN 4.2 + 4.2.1 (locked)
    ↓
IMPLEMENT REMEDIATION
    ↓
RUN ACTUAL PRODUCTION-PATH ADVERSARIAL TESTS
    ↓
GENERATE SECURITY GATE REPORT
    ↓
GO / HOLD
```

The coding agent must not proceed to Phase 4 or Phase 6 merely because
implementation completed. It must first produce GO.
