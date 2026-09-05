# Plan 6: Scope Correction + Final Security Gate

**Date:** 2026-09-04
**Trigger:** Second adversarial review (adv_review2.md) — HOLD on dead execution boundaries
**Corrective:** prompt7.md — scope correction, not fake execution

---

## Problem Statement

The second adversarial review correctly identified that the current repository contains **no real consequential production execution capability**:

- `ExecuteAction` has zero production callers
- The executor registry is empty in both production binaries
- `service/workflow` has zero imports (production or test)
- No route, handler, tool, or CLI command invokes any executor

This is a **product-scope fact**, not a security defect. The dangerous scenario — real execution bypassing authority — does not exist. We now have:

```
no real execution capability
+
a tested future execution boundary
```

Those are very different from:

```
real execution exists
+
execution bypasses authority
```

The second scenario was the original critical defect, and we fixed it.

---

## What This Plan Does NOT Do

- Do NOT invent a fake production executor
- Do NOT add a fake deployment system
- Do NOT create a fake external side effect merely to satisfy a gate
- Do NOT wire `service/workflow` into execution just to eliminate a dead-code finding
- Do NOT convert absence of execution into a reason to invent execution

---

## Current vs. Future Architecture

```
CURRENT (v0 MVP):

    evidence
      ↓
    belief
      ↓
    promotion
      ↓
    authority
      ↓
    intent

FUTURE (when real execution exists):

    intent
      ↓
    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    Executor
      ↓
    external provider
```

The future path is **designed and tested**, but not yet a production capability. That is honest.

---

## Work Items

### Work Item 1: Correct Security Gate Semantics

**File:** `docs/os/security_gate_report_v2.md`

Update the GO criteria to reflect the corrected scope:

- Remove the claim "every consequential production execution path uses `ExecuteAction`" (which is false — no such path exists)
- Replace with: "No consequential production execution capability currently exists. Future consequential execution MUST use `ExecuteAction`."
- Remove GO checkmarks that claim execution boundaries are "active production" when they are dead code
- Keep the verification that `ExecuteAction` + `PrepareForAction` + `kernel.Authorize` is the canonical **future** execution boundary
- Keep the 21 integration tests as proof of the future boundary

### Work Item 2: Document Current vs. Future Architecture

**Files:** `AGENTS.md`, `docs/os/security_gate_report_v2.md`

Add an architecture status section that honestly represents the current and future states.

State explicitly in AGENTS.md:
- `ExecuteAction` is the canonical future execution boundary (designed, tested, not yet wired to production)
- `service/workflow` is future product infrastructure (not current security infrastructure)
- The current product has no real external consequential execution capability
- When execution is introduced, `ExecuteAction` is the mandatory boundary

### Work Item 3: Clean Dead Code — `service/workflow`

**Files:** `service/workflow/workflow.go`, `service/workflow/workflow_test.go`

Per prompt7.md: "If service/workflow has no current product requirement, prefer removing unused implementation rather than adding artificial production wiring."

Action: **Remove** the `service/workflow` package entirely. It has zero imports (production or test). It is dead code that could be falsely represented as an active security boundary.

This satisfies finding F-2 from the adversarial review.

### Work Item 4: Fix MCP Conditional Authority Bypass (F-4)

**File:** `cmd/solvent-mcp/tools.go` (lines 248-263)

Current behavior: when `target_id` or `actor_id` are empty, `PrepareForAction` is skipped. The intent is created without authority verification.

Per prompt7.md: "incomplete authorization context → reject request"

Change to require both fields. If either is missing, return an error rather than silently skipping the authority check.

This satisfies finding F-4.

### Work Item 5: Document operator-review CLI Trust Boundary (F-3)

**File:** `cmd/operator-review/main.go`

The CLI calls `kernel.IntentOnPromoted` directly without `PrepareForAction`. This is a trusted administrative CLI tool, not a production server path.

Action: Add a doc comment stating:
- `cmd/operator-review` is trusted administrative tooling
- It operates under the operator's direct authority, not the automated authority boundary
- If it ever creates consequential external execution, it MUST use the canonical `ExecuteAction` path

This satisfies finding F-3. No code change needed — just documentation of the trust boundary.

### Work Item 6: Label Dead Executor References

**Files:** `cmd/solvent-mcp/main.go`, `demo/cloud/web/main.go`

Both binaries create `executor.NewRegistry()` (empty) and pass it to `authority.New()`. Since no executors are registered and `ExecuteAction` is never called from production, add documentation comments:

- "Executor registry is instantiated but empty. No production executor exists. This is the future wiring point for real execution."
- The authority service IS used by the wizard for `PrepareForAction` (intent creation verification), which is a current production path.

### Work Item 7: Final Scope-Aware Adversarial Review

After all changes, perform a final targeted adversarial review that searches for:

1. All direct external side effects
2. All executor invocation sites
3. All consequential provider calls
4. All direct `IntentOnPromoted` calls
5. All direct `kernel.Store` calls
6. All `ExecuteAction` callers
7. All `kernel.Authorize` callers

Classify each as: production consequential, production read-only, administrative CLI, test-only, future, dead, bypass.

The crucial question: **Does any EXISTING production consequential external action occur without current kernel authority?**

If no real consequential production action exists: explicitly state that fact.

### Work Item 8: Update Security Gate Report

**File:** `docs/os/security_gate_report_v2.md`

Final update with:
- Corrected GO criteria
- All findings resolved (F-1 through F-5)
- Honest current/future architecture distinction
- Final adversarial review results
- GO/HOLD decision

---

## Sequencing

```
Work Item 1 — correct gate semantics
Work Item 2 — document current/future architecture
Work Item 3 — remove service/workflow
Work Item 4 — fix MCP conditional bypass
Work Item 5 — document operator-review trust boundary
Work Item 6 — label executor references
Work Item 7 — final adversarial review
Work Item 8 — update security gate report
```

---

## Expected Outcome

After this work:

- No dead code is falsely represented as an active security boundary
- The MCP tool fails closed on incomplete authorization context
- The current absence of production execution is explicitly documented
- The future execution boundary remains canonical and tested
- The kernel remains unchanged
- GO can be declared on corrected, honest criteria

---

## GO Criteria (Corrected)

```
CURRENT PRODUCT SECURITY GATE

GO requires:

1. Every existing consequential production execution path is
   authority-gated.

2. If no consequential production execution capability exists,
   this is explicitly documented as CURRENTLY DEFERRED.

3. Any future consequential execution implementation MUST use:

      ExecuteAction
          ↓
      current-state revalidation
          ↓
      kernel.Authorize
          ↓
      Executor

4. No production executor exists unless there is a real external
   capability to execute.

5. No dead code is presented as an active production security boundary.

6. The tested future execution boundary passes the adversarial suite.

7. No existing production bypass exists.

8. No critical/high authority defects remain.

9. Intent creation, belief promotion, evidence ingestion, and
   authority creation MUST NOT directly invoke consequential
   external provider side effects.

10. Any future consequential side effect MUST originate from
    ExecuteAction → current authorization → Executor.

Otherwise HOLD.
```

---

## Phase 4/6 Authorization-First Scope Decision

**Date:** 2026-09-04

Phase 4 (Web UI) and Phase 6 (Demo scenarios) for this milestone are
**authorization-first** and ship without a real production consequential
executor.

Real consequential execution is deferred until a concrete external
execution capability is selected.

The future execution contract remains:

    ExecuteAction
      ↓
    current-state revalidation
      ↓
    kernel.Authorize
      ↓
    Executor
      ↓
    external provider

The existing recording executor (`service/executor/recording.go`) is
test-only. No fake deployment system or fake production side effect is
created to satisfy the phase.

Intent creation, belief promotion, evidence ingestion, and authority
creation MUST NOT directly invoke consequential external provider side
effects. Any future consequential side effect MUST originate from
ExecuteAction → current authorization → Executor.
