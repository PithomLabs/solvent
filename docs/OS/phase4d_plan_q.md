I agree with **A / B / A / configurable timeout**, and for Q5 I recommend **remove `SetTestHook` from production altogether and use a different synchronization seam**.

### Q1 — Execution state machine: **Option A**

Use:

```text
live → executing → executed
          ↓
      recovery / reconciliation
```

This is the right Phase 4D investment. A durable `executing` state records that Solvent has claimed the intent for execution, which is exactly what the current v1 model lacks. It also gives us a place to distinguish “never attempted” from “attempt started but outcome became ambiguous.”

The key caveat: **do not automatically roll `executing → live` after a crash**. That would recreate the duplicate-execution problem. Recovery should treat an interrupted `executing` intent as requiring reconciliation before another provider attempt.

### Q2 — Consequence parameters: **Option B**

Use the approved target snapshot as the source.

```text
target snapshot
      ↓
authorization request
      ↓
kernel verifies current authority
```

The caller should not have to reproduce a security-sensitive tuple that Solvent already owns.

That also preserves the core principle:

> **the approved snapshot is authoritative; the caller supplies intent, not authority.**

The API/MCP layer should read the target's approved parameters and feed those into the existing kernel authorization semantics. Do not allow the caller to replace them.

### Q3 — Crash recovery: **Option A for Phase 4D**

Use:

> **No retry until external reconciliation.**

This is the honest design for GitHub.

A crash after:

```text
GitHub accepted
      ↓
Solvent died
```

leaves us unable to know locally whether retrying would create a duplicate workflow.

So:

```text
executing / ambiguous
        ↓
NO automatic retry
        ↓
reconcile external state
        ↓
explicitly resolve
```

Do not fake exactly-once semantics that GitHub does not provide.

Provider-side idempotency can become a future adapter-specific capability when a provider genuinely supports it.

### Q4 — HTTP timeout: **configurable with a sane default**

Use a default such as:

```go
30 * time.Second
```

while allowing construction-time configuration.

Context cancellation remains important, but the client should have its own bounded timeout so an incorrectly configured caller cannot leave the provider hanging indefinitely.

This is cheap hardening and belongs in 4D.

### Q5 — `SetTestHook`: **remove it from production**

I would **not keep an exported test hook in production code** merely because its type is unexported.

The concern is not that the current hook is exploitable—the reviewer explicitly found no material security impact—but that it leaves unnecessary test machinery in a security-sensitive production service.

Instead, redesign the Window B test seam so the synchronization is injected through the **executor/provider test double**, without adding a public mutable hook to `authority.Service`.

The desired test remains:

```text
Authorize
   ↓
intent verification
   ↓
[deterministic test pause]
   ↓
RevokeTarget
   ↓
executor/provider
```

but the pause mechanism should be supplied through the test executor/provider infrastructure rather than through `Service.SetTestHook`.

---

### Tell the planning agent

```text
Use these decisions for Phase 4D:

Q1: Option A — add durable `executing` state.
    live → executing → executed
    Do not automatically roll executing back to live after crash.
    Ambiguous executions require reconciliation before retry.

Q2: Option B — authorization reads the approved target snapshot's
    consequence_parameters automatically.
    Caller must not supply substitute consequence parameters.
    Preserve existing kernel authority semantics.

Q3: Option A — no automatic retry after ambiguous provider state.
    Require external reconciliation before another consequential attempt.
    Do not claim exactly-once semantics.

Q4: Add a configurable HTTP client timeout with a sensible default,
    e.g. 30 seconds. Context cancellation remains supported.

Q5: Do NOT retain `SetTestHook` as a production API.
    Remove that production hook and find a test-only synchronization seam
    through the executor/provider test infrastructure that can deterministically
    pause between final authorization/intent verification and provider
    invocation.

Do not redesign unrelated parts of Solvent.
Do not add generic orchestration or distributed transactions.
```

These choices keep Phase 4D focused on the actual weakness exposed by Phase 4C+: **making consequential execution recoverable without pretending that an external API can participate in a database transaction**. The final Phase 4C+ review explicitly accepted concurrent duplicate execution and post-acceptance persistence failure as the remaining v1 limitations, which is the right problem for the next phase to attack. 
