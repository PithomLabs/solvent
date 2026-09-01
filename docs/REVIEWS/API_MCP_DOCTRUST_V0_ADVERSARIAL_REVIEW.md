# API/MCP + DocTrust V0 Adversarial Code Review

## Status
REVIEW ARTIFACT — NOT IMPLEMENTED

## Scope
Public integration layer added in this phase:
- `cmd/solvent-mcp/main.go`
- `cmd/solvent-mcp/tools.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `kernel/authority_dogfood_test.go`

Reviewed against the locked v0 kernel authority boundary defined by:
- `db/005_authority_mvp.sql`
- `db/006_authority_justification_cascade.sql`
- `kernel/authority.go`
- `kernel/sql.go`
- `kernel/errors.go`
- `kernel/contract.go`

## Repository Truth
- `db/006_authority_justification_cascade.sql` exists and adds `ON UPDATE CASCADE` to the justification composite FK.
- MCP server exposes 16 tools (not 6 as stated in the package comment at `main.go:2`).
- All 9 authority lifecycle tools are registered on a single stdio MCP server.
- No authentication, authorization, or IAM layer exists in the MCP server.
- DocTrust does not exist as code; "DocTrust-shaped" tests are kernel integration tests.

## V0 Security Boundary
The kernel authority boundary is:

```
Approve = ONLY authority-creating operation
Authorize = READ-ONLY verification
RevokeTarget = inserts target_revocation

Authority source = target_activation + target_snapshot + absence of target_revocation

FORBIDDEN as authority sources:
  authority_target proposal fields
  live justification rows
  authorization_decision
  warrant
  cache
  action_intent
```

The MCP integration must preserve this boundary without introducing bypasses, confusion, or false guarantees.

## Executive Verdict

**APPROVE**

The API/MCP integration preserves the kernel's authority boundary in all respects. Every MCP handler is a thin adapter that delegates to the kernel. No handler directly issues authority SQL. No handler independently decides ALLOW. The `Authorize` tool remains pure read. The `Approve` path is unchanged. All DB constraints remain authoritative. The `ON UPDATE CASCADE` fix in `db/006` survives the API layer unchanged.

One deployment-contract weakness exists: all authority tools are exposed on a single MCP server with no technical enforcement of the trusted-boundary requirement. The documentation is honest about this limitation. This is a v0-accepted deployment constraint, not a code defect.

---

## Critical Findings

**None.**

## High Findings

**None.**

## Medium Findings

### AMDAR-M1 — MEDIUM — Trusted-boundary tools lack consistent deployment warnings

**Claim Attacked**
Tool descriptions for administrative operations accurately state the trusted-surface requirement for some tools but not all.

**Evidence**
- `solvent_create_principal` (`main.go:253`): "MCP principal-ID fields are attribution inputs, not authentication proof — the v0 MCP server must be deployed as a trusted administrative surface."
- `solvent_approve` (`main.go:380`): "Only call from a trusted administrative surface — the approved_by field is attribution, not caller authentication."
- `solvent_create_target` (`main.go:287`): "The created_by field records attribution, not caller identity."
- `solvent_revoke_principal` (`main.go:272`): No trust-boundary warning.
- `solvent_revoke_target` (`main.go:446`): No trust-boundary warning.
- `solvent_discharge` (`main.go:469`): No trust-boundary warning.

**Attack**
An operator scanning tool descriptions might conclude that `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` are safe for untrusted agent invocation because they lack the explicit trust warning present on other admin tools.

**Exact Failure Boundary**
Documentation inconsistency, not a security bypass. The kernel still enforces all constraints. The deployment contract (entire server is trusted) still covers these tools.

**Why It Fails or Holds**
Holds: The server-level deployment contract covers all tools. The kernel enforces all authority constraints regardless of which tool invoked it. No authority can be manufactured without legitimate approval.

**Required Resolution**
Add consistent "trusted administrative surface" warnings to `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` tool descriptions.

**DDL/Schema Impact**
None.

**Code-Blocking**
No. This is a documentation improvement.

**Exit Test**
Visual inspection of tool descriptions.

---

### AMDAR-M2 — MEDIUM — Package comment states incorrect tool count

**Claim Attacked**
The package comment at `main.go:2` claims the server exposes "six tools" when it actually exposes 16.

**Evidence**
`main.go:2`: "Command solvent-mcp is a stdio MCP server exposing the Solvent transactional belief ledger as six tools."

Actual registration in `main.go:88-492`: 16 tools are registered via `server.AddTool`.

**Attack**
An operator reading the package comment might believe the server has a smaller attack surface than it actually does, or might miss the authority tools entirely.

**Exact Failure Boundary**
Documentation inconsistency only. No security impact.

**Why It Fails or Holds**
Fails: The comment is factually incorrect. This could lead to deployment mistakes.

**Required Resolution**
Update the package comment to reflect the actual tool count and explicitly note that authority tools require trusted deployment.

**DDL/Schema Impact**
None.

**Code-Blocking**
No.

**Exit Test**
Visual inspection.

---

## Low Findings

**None.**

---

## MCP Authentication / Attribution Boundary

**Finding: CONFIRMED — attribution is NOT authentication, and the implementation is honest about it.**

The v0 contract states: `*_by` fields are attribution inputs, not authentication proof. The MCP server is a trusted administrative surface. Authentication of the caller is an external deployment responsibility.

Evidence:
- `main.go:253`: "MCP principal-ID fields are attribution inputs, not authentication proof — the v0 MCP server must be deployed as a trusted administrative surface."
- `main.go:380`: "Only call from a trusted administrative surface — the approved_by field is attribution, not caller authentication."
- `main.go:287`: "The created_by field records attribution, not caller identity."

No tool description implies that supplying a `principal_id` authenticates the caller. The kernel treats all supplied principal IDs as data, not as proof of identity. The kernel's `Approve` method checks `principal.revoked_at` but does not verify that the caller "is" that principal.

**Classification: Deployment-contract limitation, honestly documented. Not a code defect.**

---

## Authority-Creation Boundary

**Finding: REJECTED — no bypass exists.**

The ONLY authority-creating path is `kernel.Approve`, called exclusively by `handleSolventApprove` (`tools.go:461-479`). No MCP handler directly inserts into `target_snapshot` or `target_activation`. No handler creates `target_activation` rows without going through `Approve`.

Trace verified:
```
solvent_approve
  → handleSolventApprove
    → kernel.Approve
      → sqlApproveReadTarget (FOR UPDATE)
      → sqlApproveReadJustifications
      → computeRequestHash
      → sqlApproveReadBeliefs (promoted check)
      → approver revocation check
      → sqlApproveInsertSnapshot
      → sqlApproveInsertActivation
```

All kernel-level checks are preserved:
- Pin validation (T-07, T-08)
- Justification validation (T-03, T-04, T-05)
- Belief promoted check (T-22)
- Approver revocation check (T-26)
- UNIQUE(target_id) prevents second activation (T-10, T-12, T-C1, T-P1)

**Classification: Boundary holds. No defect.**

---

## Authorize Read-Only Boundary

**Finding: CONFIRMED — Authorize performs zero writes.**

`handleSolventAuthorize` (`tools.go:481-534`) calls `kernel.Authorize` and returns the result. The kernel's `Authorize` method (`authority.go:330-415`) performs only SELECT queries:
1. `sqlAuthorizeResolve` — JOIN of `target_activation` and `target_snapshot` with `NOT EXISTS` revocation check.
2. Field-by-field tuple comparison.
3. `SELECT status FROM belief WHERE id = $1::UUID` for each justification.

No INSERT, UPDATE, or DELETE occurs. Test T-28 (`authority_test.go:905-943`) confirms row counts in `target_snapshot`, `target_activation`, and `target_revocation` are unchanged after `Authorize`.

The MCP layer adds no writes. `jsonResult` marshals the `AuthorizeResult` and returns it as MCP content. `IsError` remains `false` (default) for legitimate denials.

**Classification: Boundary holds. No defect.**

---

## Approval Pin

**Finding: REJECTED — pin logic is preserved.**

The MCP layer does not intercept, normalize, or reconstruct the approval pin. `RequestAuthorization` computes the hash inside the kernel transaction from current proposal + justification set. `Approve` recomputes from the same sources. Any mutation between the two causes `ErrApprovalPinMismatch`.

The MCP handlers pass arguments directly to the kernel without transformation. `consequence_parameters` is passed as raw string/bytes. The kernel's `computeRequestHash` (`authority.go:464-503`) hashes the exact bytes.

Tests T-07 and T-08 confirm post-request mutations invalidate the pin. Tests DT-9 and the MCP test suite confirm the same behavior through the adapter.

**Classification: Boundary holds. No defect.**

---

## Retraction / Re-Promotion

**Finding: CONFIRMED — ON UPDATE CASCADE survives the API layer.**

`db/006_authority_justification_cascade.sql` adds `ON UPDATE CASCADE` to the justification composite FK. `RetractCascade` (`kernel.go:127-150`) executes:
1. Cancel live intents on descendants.
2. `UPDATE belief SET status = 'retracted' WHERE ...`

The FK propagates the status change into `justification.belief_status`. `Authorize` subsequently sees `belief_status != 'promoted'` and denies.

Tests T-22 and T-C3 confirm retraction succeeds without manually deleting justifications, and Authorize denies afterward. DT-7 confirms the same through dogfood tests.

The accepted v0 gap (retract → re-promote can revive justification) is exercised by T-23 and DT-8. After re-promotion, `belief.status = 'promoted'` and `Authorize` re-reads the live `belief.status` for each justification (not the stale `justification.belief_status`), so it returns ALLOW. This is the intended v0 gap behavior: without `promotion_epoch`, the system cannot distinguish "originally promoted" from "re-promoted".

**Classification: Behavior is correct and intentional. No defect.**

---

## Resurrection

**Finding: REJECTED — UNIQUE(target_id) prevents re-activation.**

`target_activation` has `UNIQUE(target_id)`. Once a target is activated, no second activation is possible regardless of revocation status.

Tests T-10, T-12, T-P1, T-C5, and DT-10 all confirm:
- Second `Approve` after first activation fails with `ErrAlreadyActivated` or SQLSTATE 23505.
- `RevokeTarget` followed by `Approve` fails with the same unique violation.
- The MCP layer does not provide any path to bypass this constraint.

**Classification: Boundary holds. No defect.**

---

## Snapshot Substitution

**Finding: REJECTED — composite FK prevents cross-target snapshot binding.**

`target_activation(target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)` is enforced by a composite FK. Attempting to insert T2's activation with T1's snapshot_id fails with SQLSTATE 23503.

Tests T-11 and T-P3 confirm this at the kernel level. The MCP layer does not expose any path that constructs `target_activation` rows directly; all activation goes through `kernel.Approve`, which inserts the correct `(target_id, snapshot_id)` pair atomically.

**Classification: Boundary holds. No defect.**

---

## Concurrency / Retry

**Finding: CONFIRMED — kernel transaction semantics are preserved.**

The MCP layer adds no concurrency control. Each handler calls a single kernel method within the handler's own context. The kernel uses `crdb.ExecuteTx` with SERIALIZABLE isolation for all write operations.

Concurrency tests confirm:
- T-C1: Approve × Approve → exactly one activation.
- T-C2: Approve × AttachJustification → pin mismatch or activation, never ghost justifications.
- T-C3: Approve × Belief Retract → retraction succeeds, approval denied.
- T-C4: Discharge × Discharge → exactly one succeeds.
- T-C5: Revoke × Approve → second approval fails.

The MCP layer does not intercept 40001 retry signals. `crdb.ExecuteTx` handles retries transparently. A retried transaction re-evaluates all kernel checks on fresh state. No stale caller state is cached or reused by the MCP layer.

**Classification: Boundary holds. No defect.**

---

## Error Semantics

**Finding: REJECTED — DENY and infrastructure error are distinct.**

The MCP layer preserves the kernel's error semantics:

- `kernel.Authorize` returns `(AuthorizeResult{Allowed: false, Reason: "..."}, nil)` for legitimate denials. The MCP handler returns `IsError: false` (default) with `"allowed": false`.
- `kernel.Authorize` returns `(AuthorizeResult{}, err)` for infrastructure failures (DB connection, SQL errors). The MCP handler returns `IsError: true`.
- `kernel.Approve` returns `(nil, err)` for any failure (pin mismatch, revoked principal, etc.). The MCP handler returns `IsError: true`.

Test MT-4 (`tools_authority_test.go:186-227`) confirms that a revoked approver attempting approval produces an MCP error (`IsError: true`), not a successful approval. Test MT-5 confirms non-existent target produces an MCP error, not a panic.

SQLSTATE codes are preserved in the wrapped error. The MCP layer does not replace them with generic prose.

**Classification: Boundary holds. No defect.**

---

## Input Normalization

**Finding: REJECTED — no silent substitution or wildcard expansion.**

All authority-dimension fields are validated as non-empty strings before kernel invocation. Missing required fields return MCP errors. No field is silently defaulted to a wildcard or empty value that would broaden authority.

`consequence_parameters` is validated as JSON if non-empty. Empty string becomes `nil` bytes, which `jsonEqual` treats as equivalent to empty JSONB. This is correct semantic behavior, not wildcard expansion.

`handleSolventAuthorize` uses `fmt.Sprint` for all fields, with an explicit rejection of `"<nil>"`. This prevents nil interface values from being interpreted as literal strings.

**Classification: Boundary holds. No defect.**

---

## Second Authority Source Audit

**Finding: REJECTED — no second authority source exists.**

Search of all MCP handlers and integration code confirms no handler independently decides ALLOW using:
- target proposal fields
- justification rows
- requested flag
- principal_id from tool arguments
- cached state
- previous tool results

The only semantic authority decision is in `kernel.Authorize`, which resolves from:
```
target_activation
  JOIN target_snapshot
  WHERE NOT EXISTS target_revocation
```

Tests T-29 and T-30 confirm no `authorization_decision` table and no cache table exists. DT-13 confirms `Authorize` performs zero writes.

**Classification: Boundary holds. No defect.**

---

## DocTrust-Shaped Integration Audit

**Finding: REJECTED — tests are honestly labeled.**

DocTrust does not exist as code in this repository. The integration tests in `kernel/authority_dogfood_test.go` are labeled "DocTrust-shaped" and use wave identifiers like `"doctrust"`. They exercise the public kernel/API semantics as a proxy for the eventual external DocTrust consumer.

The test file name (`authority_dogfood_test.go`) and wave labels make it clear these are not production DocTrust integration tests. No false production claim is made.

**Classification: No defect. Labeling is accurate.**

---

## Executor Boundary

**Finding: REJECTED — no execution-effect verification claimed.**

No MCP tool claims that successful `Authorize` equals action execution. Tool descriptions are clear:
- `solvent_authorize`: "Read-only verification: check if an execution-time tuple matches the approved authority."
- `solvent_authorize_action`: "Record a live intent to take a real-world action, citing a belief as its warrant."

The executor boundary remains:
```
Solvent authorization → authorized executor → actual action
```

Solvent v0 does not verify execution effects. No tool claims otherwise.

**Classification: Boundary holds. No defect.**

---

## Deferred V0 Gap Audit

**Finding: CONFIRMED — accepted v0 gap is reachable and documented.**

The accepted v0 gap (retract → re-promote can revive old justification) is:
- Documented in T-23 (`authority_test.go:763-796`) and DT-8 (`authority_dogfood_test.go:254-280`).
- Labeled as "ACCEPTED V0 GAP" in test receipts.
- Reachable because `db/006` fixes the FK issue that previously blocked retraction.

No feature from the deferred list was introduced:
- `promotion_epoch` / `belief_promotion` — absent
- `policy_version` — absent
- `tenant_id` / `belief_tenant` — absent
- `credential` management — absent
- `authorization_decision` — absent (T-29)
- `warrant` — absent
- `execution_receipt` — absent

**Classification: Gap is intentional and properly documented. No defect.**

---

## Security Test Coverage

**Finding: CONFIRMED — comprehensive coverage of kernel invariants through MCP layer.**

Kernel-level tests (T-01 through T-30, T-C1 through T-C5, T-P1 through T-P3) cover all critical invariants.

MCP adapter tests (MT-1 through MT-5) confirm:
- MT-1: CreateTarget maps all fields correctly.
- MT-2: Approve maps `approved_by` correctly.
- MT-3: Authorize maps tuple correctly, returns Allowed/Reason.
- MT-4: Kernel denial becomes MCP denial (not infrastructure error).
- MT-5: Kernel infrastructure error becomes tool error (not panic).

DocTrust-shaped tests (DT-1 through DT-14) confirm kernel semantics through the full lifecycle.

**Missing coverage:**
- No MCP-level concurrency test (relies on kernel tests, which is acceptable).
- No MCP-level test for malformed JSON in `consequence_parameters` (handler validation exists but is untested through MCP).
- No test for the `"<nil>"` rejection path in `handleSolventAuthorize`.

**Classification: Coverage is sufficient for v0. Minor gaps are test improvements, not security defects.**

---

## Production Hardening Findings

**None for v0.** The following are noted for future phases, not v0 blockers:
- No rate limiting at MCP layer.
- No audit logging of tool invocations.
- No separation of trusted/untrusted tool sets on separate servers.
- No caller identity verification.

---

## Required Fixes

**None required for v0 approval.** The integration preserves all kernel security boundaries.

Recommended improvements (not blockers):
1. Add "trusted administrative surface" warnings to `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` tool descriptions.
2. Correct the package comment at `main.go:2` to reflect the actual tool count and trusted-boundary requirement.
3. Add MCP-level tests for malformed JSON and nil-handling edge cases.

---

## Final Gate

**API/MCP + DOCTRUST ADVERSARIAL REVIEW PASSED — READY FOR PRODUCTION HARDENING**

The integration preserves the kernel's authority boundary in all respects:
- Approve remains the sole authority-creating operation.
- Authorize remains pure read.
- No second authority source exists.
- All DB constraints remain authoritative.
- The `ON UPDATE CASCADE` fix in `db/006` is preserved through the API layer.
- The accepted v0 re-promotion gap is reachable and documented.
- Error semantics correctly distinguish DENY from infrastructure failure.
- No schema changes were introduced.
- No forbidden v0 features were introduced.

The only limitation is the v0-accepted deployment contract: the entire MCP server must be deployed as a trusted administrative surface. The implementation and documentation are honest about this requirement.

---

## Files Inspected

- `cmd/solvent-mcp/main.go`
- `cmd/solvent-mcp/tools.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `kernel/authority.go`
- `kernel/authority_test.go`
- `kernel/authority_dogfood_test.go`
- `kernel/sql.go`
- `kernel/errors.go`
- `kernel/contract.go`
- `kernel/kernel.go`
- `db/005_authority_mvp.sql`
- `db/006_authority_justification_cascade.sql`

## Files Changed

**None.** This is a read-only review. No modifications were made.
