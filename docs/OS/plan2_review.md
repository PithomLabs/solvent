All five findings are resolved, and precisely — not just acknowledged in prose while the underlying spec stayed the same, which is the failure mode that actually matters to check for.

**Token ≠ Authority (was CRITICAL): fully closed.** Section 11's token struct has no `Authority`, no `SnapshotID`, no approval state — just continuity fields. The line "a valid HMAC proves token integrity, it does NOT prove current authority validity" is the exact distinction that was missing, stated as plainly as it needs to be. Section 12 names `Refresh(token, map[string]interface{})` explicitly and forbids it, replacing it with `AdvanceStage`/`AttachEvidence`/`CompleteReview` — the same typed-transition shape I sketched, done properly with per-transition precondition checks rather than a generic overwrite path. And Section 34 goes further than I asked: `valid signed stale token + revoked current authority = DENIED` is now a named adversarial test case, which is the actual attack I was describing turned into something that has to pass before ship. This is the correct depth of fix.

**Document/Signature scope (was HIGH): closed.** Explicitly tagged `FUTURE / NOT MVP` in the adapter list itself, not just in prose elsewhere that a reader could miss.

**Duplicate Executor (was MEDIUM): closed.** One interface, explicit layering (`ExecutionService → Executor port → provider adapter`), and the "avoid defining two unrelated Executor interfaces" line names the thing I was worried about directly.

**`007_workflow.sql` ambiguity (was MEDIUM): closed, and more disciplined than what I asked for.** I asked for the classification to be stated explicitly. This goes further — "there should be NO default workflow migration... do NOT create it unless implementation proves persistent workflow state is genuinely required" — which applies "complexity must be earned" to the schema decision itself rather than just labeling it. That's the better fix.

**Scenario D wording (was minor): closed.** "NEVER mutate the original immutable authority... do NOT implement 'target correction' on an existing authority" — unambiguous now.

One new thing worth raising, not a defect — a scope question. Section 21–23 add a full Compliance/Governance layer: `ComplianceService`, control-to-evidence mapping, exception management, a dedicated Phase 5. It's architecturally correct — it sits above the kernel, consumes facts rather than creating authority, explicitly refuses to let `exception=true` bypass a kernel invariant, doesn't market itself as "makes you compliant." I have no objection to the design. My question is placement: principle #15 in this same document says complexity must be earned by "real customer demand, operational pain, observed security failure, or a clear security requirement" — and nothing in the plan cites a customer asking for a SOC 2/ISO 27001 evidence export yet. Given you've said explicitly that endless review cycles aren't the goal right now, it's worth asking whether Phase 5 ships in v0 or gets deferred to the first real compliance-buyer conversation, the same way the plan already correctly defers workflow persistence until something proves it's needed. Not a blocker either way — just make it a decision rather than default scope, the same discipline the rest of this document earned the hard way.

Approved. Build Phases 0–4 and 6; decide Phase 5's timing before or during Phase 4, whichever is cheaper to postpone.



## chatgpt

Agreed. That is the right scope decision.

I would **defer Phase 5 (Compliance/Governance implementation) from the initial v0 build**, while keeping the architecture and extension point reserved.

The distinction should be:

```text
v0 architecture:
    ComplianceService boundary exists conceptually
    ↓
    no substantial compliance implementation yet

v0 implementation:
    Phases 0–4
    +
    Phase 6 demos

Later:
    Phase 5 when a real compliance use case earns it
```

This is consistent with the same complexity discipline already applied elsewhere in the plan: don't create persistent workflow state until it's needed, and don't expand the kernel without evidence. The current plan already establishes that customer-specific compliance semantics belong outside the kernel. 

### I would make one small amendment to the plan

Change Phase 5 from:

```text
PHASE 5 — Compliance
    - Control mapping
    - Evidence package
    - Audit export
    - Exceptions
```

to:

```text
PHASE 5 — COMPLIANCE / GOVERNANCE
STATUS: DEFERRED — EARNED FEATURE

Do not implement substantive compliance functionality in initial v0.

Reserve the ComplianceService boundary and product architecture so that
future compliance capabilities can consume existing Solvent facts without
kernel changes.

Reconsider Phase 5 when triggered by:
    - an actual compliance-oriented customer requirement
    - a concrete buyer workflow
    - repeated operational demand
    - a specific compliance evidence/reporting requirement

When triggered, implement the smallest concrete capability required by
the validated use case.
```

Then keep it in the roadmap rather than deleting it.

### The resulting build scope

```text
NOW
│
├── Phase 0  Reconnaissance
├── Phase 1  Service boundaries
├── Phase 2  Security boundaries
├── Phase 3  One real integration
├── Phase 4  Product UI
├── Phase 6  Demo platform
└── Phase 7  Open-source polish
         │
         ▼
     REAL USERS
         │
         ├── security demand
         ├── operational demand
         └── compliance demand
                    │
                    ▼
             Phase 5 earned
```

One nuance: **do not defer the compliance *architecture***. Keep the conceptual boundary, data lineage, and auditability needed to make future compliance functionality straightforward. What we're deferring is the **compliance product feature set**.

That is the cleanest interpretation of the principle you just articulated:

> **Architect for extensibility; implement only what the market has earned.**

So I would regard the plan as **approved for Phases 0–4 and 6**, with Phase 5 explicitly marked **deferred / trigger-based**, not silently included in the coding scope.

