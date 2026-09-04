I agree with the **substance** of this independent review, but I would **not accept its final HOLD classification as written**.

The review did what we wanted: it found several implementation-level inconsistencies that previous reviews missed. Most importantly, it confirms the MCP schema/runtime contract is now correct, no hidden consequential production side effects were found, `kernel.Authorize` remains the final authority oracle, exact tuple binding and revocation are enforced, and the test harness is meaningful.   

But there are **three things I would fix before Phase 4**.

## 1. F-2: remove the dangling `GetToken`

This one is straightforward.

The workflow package was deliberately deleted, but `service/authority.GetToken` still queries `workflow_token`. The review confirms there is no production caller. 

That is exactly the kind of residue that later causes architectural confusion:

```text
workflow removed
    ↓
GetToken remains
    ↓
workflow_token still appears relevant
```

Delete `GetToken`.

Do **not** restore workflow just to satisfy this finding.

---

## 2. F-3: standardize consequence parameters

This is the most important *actual implementation inconsistency* left.

The review found:

```text
Wizard:
    consequenceParameters = nil

MCP:
    consequenceParameters = []byte("{}")
```

Yet `consequenceParameters` participate in the exact authority tuple and are compared by `kernel.Authorize`. 

That means two callers of the same authority boundary can represent the same apparent action differently.

Today it is low practical risk because the wizard has no approved target, but that is exactly the kind of latent bug worth removing **before** we build UI around it.

I would standardize on one canonical representation, probably:

```text
empty JSON object = {}
```

rather than `nil`, assuming that matches the kernel's intended semantic for a parameterless consequence.

Then add a regression asserting both callers construct the same consequence representation.

---

## 3. F-1 should NOT remain a HIGH security finding

This is where I disagree with the report.

It says:

> `ExecuteAction` is dead code, therefore HIGH.

But the same report establishes:

> **No production consequential execution capability exists.** 

And later:

> Exploitability: N/A — nothing can execute today. 

That means F-1 is not a current security vulnerability.

It is:

```text
PRODUCT SCOPE / FUTURE CAPABILITY
```

The security property we care about is:

```text
when consequential execution is introduced:
    ExecuteAction
        ↓
    current revalidation
        ↓
    kernel.Authorize
        ↓
    Executor
```

That contract is already documented and tested. The review itself classifies the future boundary as a documented assumption rather than a current runtime guarantee. 

So I would **reclassify F-1 from HIGH → INFO/SCOPE**, not wire a fake executor.

---

# The one additional thing I would inspect

The review found this in `ExecuteAction`:

> executor is resolved from `s.execReg.Get(toolName)` where `toolName` comes from `params["tool_name"]`. 

That is acceptable while the registry is empty, but when a real executor is eventually introduced, **`tool_name` must not become a caller-controlled way to select an arbitrary consequential capability**.

The future safe pattern is:

```text
authorized consequence/action
        ↓
trusted executor mapping
        ↓
specific Executor
```

not:

```text
authorized action
+
caller-supplied tool_name
        ↓
arbitrary executor
```

I would document that now as a future invariant, but **not build the machinery yet**.

---

# My disposition

I would set the current status to:

```text
SECURITY:
    No Critical
    No current High security vulnerabilities

REQUIRED CLEANUP:
    F-2 GetToken
    F-3 consequence-parameter normalization

ACCEPTED SCOPE:
    F-1 no production executor
    operator-review trusted admin CLI

PHASE 4:
    BLOCKED only until F-2/F-3 cleanup + one final verification
```

That is much more faithful to what the review actually discovered.

The review itself says there are no hidden provider bypasses, no default-allow behavior, no duplicate authority oracle, no unexpected kernel/schema growth, and the MCP contract is now correct.  

## Give the coding agent this final cleanup prompt

```text
SOLVENT — FINAL PRE-PHASE-4 CLEANUP

The independent implementation-specific review found no current
Critical security defect and no hidden consequential production side
effect.

Do NOT add a fake production executor.

Perform only the following cleanup.

1. DELETE service/authority.GetToken

The workflow service was intentionally removed.

Remove:

    service/authority.GetToken

and any now-unused workflow-token references/imports associated with it.

Do NOT restore service/workflow.

Verify no production code references workflow_token through a service
API after cleanup.

2. STANDARDIZE CONSEQUENCE PARAMETERS

Current inconsistency:

    wizard → nil
    MCP    → []byte("{}")

Choose the canonical representation already consistent with the kernel
semantics. Prefer the canonical empty-object representation if that is
the intended representation for a parameterless consequence.

Update all callers of PrepareForAction consistently.

Add a regression proving equivalent parameterless callers produce the
same authorization tuple representation.

Verify:

    consequence parameters supplied at authorization
        =
    consequence parameters stored in the approved snapshot
        =
    consequence parameters passed through the service boundary

Do not weaken kernel exact comparison.

3. RECLASSIFY F-1 CORRECTLY

Do NOT wire a fake production executor.

Update the security report so:

    "No production consequential execution capability exists"

is treated as PRODUCT SCOPE / FUTURE CAPABILITY, not a current
security vulnerability.

Preserve the future invariant:

    ExecuteAction
      →
    current-state revalidation
      →
    kernel.Authorize
      →
    Executor
      →
    external provider

4. DOCUMENT FUTURE EXECUTOR SELECTION

The current ExecuteAction resolves an executor using tool_name.

Document that when real executors are introduced:

    caller-controlled tool_name
        must NOT
    become an arbitrary consequential capability selector.

Future executor selection must be constrained by the authorized
consequence/action and trusted internal mapping.

Do not implement future executor policy now.

5. RUN FULL VERIFICATION

Run with no cache:

    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...

6. FINAL TARGETED REVIEW

Perform one final read-only review specifically of:

    service/authority/authority.go
    service/executor/
    cmd/solvent-mcp/tools.go
    internal/wizard/refusal.go
    cmd/operator-review/main.go
    all PrepareForAction callers
    all IntentOnPromoted callers
    all kernel.Authorize callers
    all external side-effect sites

Confirm:

    GetToken is gone
    workflow package is gone
    consequence parameters are canonical
    MCP required fields remain required
    no conditional security downgrade exists
    no direct consequential side effect exists
    kernel.Authorize remains the sole final authority oracle
    operator-review remains explicitly bounded trusted admin tooling
    no production executor exists
    future ExecuteAction boundary remains canonical

7. FINAL CLASSIFICATION

Expected:

    CRITICAL = 0
    HIGH SECURITY = 0

Accepted residuals:

    trusted operator-review CLI
    no production executor yet

Do not classify the absence of a production executor as a current
security vulnerability.

8. FINAL GATE

GO if:

    no Critical findings
    no High current security findings
    GetToken dangling reference removed
    consequence parameters standardized
    MCP schema/runtime still agree
    no silent downgrade path
    no hidden external side effect
    no second authority oracle
    no authority bypass
    tests/build/vet pass
    documentation matches actual current/future scope

Otherwise:

    HOLD

Do not proceed to Phase 4 until this gate is satisfied.
```

### Why I would do this

This is now **cleanup, not another architectural iteration**.

The independent review has already established the important security facts: exact tuple enforcement, revocation, authority-oracle centralization, no hidden side effects, and strong test behavior.  

So fix **F-2 and F-3**, reclassify **F-1 as scope**, run the suite once more, and then I would be comfortable moving to Phase 4.

And I would keep one more adversarial review after Phase 4—but that review should attack the **browser/UI/service boundary**, which will be the next genuinely new attack surface.
