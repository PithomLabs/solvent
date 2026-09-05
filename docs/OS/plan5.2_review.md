I reviewed **Plan 5.2** against the prior adversarial findings and the corrections we made in 4.1/4.2.

## Verdict: **LOCKED / APPROVED FOR IMPLEMENTATION**

This version resolves the remaining ambiguity cleanly.

The key architectural decision is now explicit:

> **Workflow tokens are continuity-only and are not part of the authority boundary.** 

That is the right call for the current product because there is no actual workflow-driven executor yet.

More importantly, you avoided the temptation to invent a fake deployment system merely to make the architecture look complete. The plan instead uses a **test-only recording executor** to prove the security boundary. 

### The critical invariant is now properly stated

The strongest formulation is:

> **No successful CURRENT `kernel.Authorize` for the EXACT actor/action/target tuple → no executor invocation.** 

And the final gate requires the actual production execution architecture to satisfy that property, including exact tuple matching, revocation, policy separation, actor/authentication separation, and no production bypass. 

### The test strategy is also correct

The tests are now properly separated into:

```text
Execution Authorization
Capability Boundaries
Adapter / Integration
```

with explicit TOCTOU regressions and a positive control. 

Most importantly, the plan requires the tests to exercise `ExecuteAction`, not merely isolated helper methods.

### The workflow decision is especially important

You explicitly removed WorkflowService from the execution path:

```text
MCP / Wizard / future execution entry
    ↓
ExecuteAction
    ↓
PrepareForAction
    ↓
kernel.Authorize
    ↓
Executor
```

while keeping workflow as future continuity machinery. 

That prevents us from repeating the original mistake of creating a boundary that looks architecturally important but isn't actually part of the security path.

### The TOCTOU claim is appropriately restrained

The plan now says Solvent guarantees current authorization immediately preceding executor invocation, while explicitly **not claiming atomic coordination with an external provider**. 

That is exactly the right security claim at this maturity level.

---

## One minor observation, not a blocker

The current plan says:

> `service/authority` is "on production path" because `PrepareForAction` is used during MCP/Wizard intent creation, while `ExecuteAction` itself still has zero production callers. 

That's accurate enough, but I'd describe it operationally as:

```text
service/authority
    → production intent path
    → test-only execution path
    → future production execution path
```

rather than implying the current product already has a real consequential execution path. This distinction is already reflected elsewhere in the document, so it is **not a reason to revise the plan**.

---

# Final decision

**Lock Plan 5.2.**

Implementation sequence is now:

```text
Plan 5.2
   ↓
Recording executor
   ↓
CockroachDB integration tests
   ↓
Fresh adversarial code review
   ↓
Security Gate Report
   ↓
GO / HOLD
   ↓
ONLY IF GO
   ↓
Phase 4 — Web UI
```

Do **not** skip the adversarial review after the tests pass. The entire reason for this remediation cycle is that the earlier implementation had a healthy kernel but a bypassed production boundary. Plan 5.2 now explicitly makes the final gate depend on proving that no such bypass remains. 

**Plan 5.2 is ready to hand to the coding agent.**
