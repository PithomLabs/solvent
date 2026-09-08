# Plan 8.3 — Remediation Design + Kernel Growth Gate for M-01, M-02, L-01, L-05

We have completed the fresh adversarial review of HEAD `5933f44` and identified four current findings:

- M-01 — REST API cross-scenario mutation gap
- M-02 — `ExecuteAction` accepts empty `intentID`
- L-01 — MCP `handleSolventDischarge` lacks scenario binding
- L-05 — `ReconcileIntent` records successful completion before the kernel transition

Do NOT implement any code yet.

This pass is primarily:
1. architectural decision-making,
2. caller/invariant inventory,
3. kernel-growth adjudication,
4. remediation design,
5. test design.

The resulting document must be implementation-ready, but no source changes should be made during this pass.

---

## 1. M-01: APPLY THE KERNEL GROWTH GATE BEFORE TOUCHING THE KERNEL

M-01 is real:

- REST mutation handlers currently operate from `belief_id`.
- The kernel mutations for `RetireDebt`, `Promote`, and `Discharge` are not scenario-bound.
- MCP currently performs a scenario guard before those mutations.
- Therefore the REST surface can potentially mutate a belief belonging to another scenario.

Before proposing ANY modification to:

- `kernel/sql.go`
- `kernel/kernel.go`
- `kernel/authority.go`
- any kernel API/signature

you MUST perform an explicit Kernel Growth Gate ADR.

The ADR must answer:

> Is scenario binding a durable security invariant that genuinely requires kernel/DB-level enforcement, or is the existing service/handler-layer authorization model sufficient?

Use the project's established kernel-growth principle:

> Add a kernel primitive or kernel-level invariant only when a new durable security fact or atomic state transition genuinely cannot be expressed safely outside the kernel.

Do NOT assume kernel growth is automatically superior.

### Evaluate at least these options

#### Option A — Kernel/DB enforcement

Change the kernel operations so scenario identity is part of the mutation contract:

```text
RetireDebt(ctx, scenarioID, beliefID, ...)
Promote(ctx, scenarioID, beliefID, ...)
Discharge(ctx, scenarioID, beliefID, ...)