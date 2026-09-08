Implement the MINIMUM pre-freeze correction identified by the debt-domain-agnosticism review.

SCOPE:
- This is a real code change.
- Do NOT redesign Solvent's debt model.
- Do NOT add a policy engine.
- Do NOT add workflow abstractions.
- Do NOT add a debt-count limit.
- Do NOT add idempotency.
- Do NOT change the promotion invariant.
- Do NOT introduce new kernel primitives unless the existing API change genuinely requires it.
- Preserve the kernel-freeze architecture: the kernel owns generic debt lifecycle and the "unresolved debt blocks promotion" invariant; domain/policy layers own debt vocabulary and meaning.

## Problem

The current kernel mechanism is generic:
- `RetireDebt` accepts arbitrary string identifiers.
- `array_remove` treats debt elements generically.
- `promoted_is_debt_free` checks only whether the array is empty.

However, `EnterBelief` and `EnsureBelief` currently hardcode the deployment-specific `FullDebt` vocabulary as the initial debt set.

That means:
- the mechanism is domain-agnostic,
- but the standard belief-creation path is not.

A different domain such as the physics verifier would otherwise need to:
- inherit the deployment debt vocabulary,
- bypass the kernel creation API,
- or use direct SQL.

That violates the requirement that different domains be able to use the same Solvent kernel without modifying kernel code or bypassing kernel APIs.

## Desired result

Make initial debt an input to belief creation.

Conceptually change:

    EnterBelief(...)
        → implicitly applies FullDebt

into:

    EnterBelief(..., initialDebt)
        → stores the supplied initial debt

The kernel must not interpret the supplied debt identifiers.

For example:

    deployment policy:
        ["needProvenanceCheck", "needContradictionSweep", ...]

    physics policy:
        ["proof_check", "counterexample_search", "semantic_applicability"]

Both must use the same kernel creation mechanism.

`FullDebt` may remain as a convenience/default at a higher-level caller if repository conventions need a default, but it must no longer be a mandatory kernel semantic.

## Before changing code

Inspect the CURRENT repository and identify:

1. Exact signatures and callers of:
   - `EnterBelief`
   - `EnsureBelief`
2. Where `FullDebt` is referenced.
3. Whether belief creation already has an options/configuration pattern.
4. Whether any API/MCP/public contract exposes the initial debt set.
5. Whether any tests depend on implicit six-item initialization.
6. Whether DDL defaults or migrations still force the six-item vocabulary.
7. Whether `EnsureBelief` is intended to preserve existing rows unchanged.

Do not assume the plan's terminology matches the current HEAD.

## Design requirements

### 1. Generic kernel creation

The kernel creation path must be able to receive arbitrary initial debt identifiers.

The kernel must NOT:
- validate them against `FullDebt`
- switch on specific debt names
- assign meaning to them
- impose a fixed domain vocabulary

### 2. Preserve existing generic invariants

Do not change:
- `promoted_is_debt_free`
- debt retirement semantics
- discharge semantics
- `array_remove` behavior
- scenario isolation
- existing transaction semantics
- existing security invariants

The only intended semantic change is:
- initial debt is supplied explicitly rather than implicitly hardcoded by the kernel.

### 3. Existing application behavior

Preserve current Solvent behavior for existing callers.

If the current application expects the six deployment-review obligations, update those callers to pass the existing `FullDebt` value explicitly.

That keeps existing behavior while removing the kernel's assumption that `FullDebt` is universal.

Do NOT silently replace current callers with an empty debt array.

### 4. `FullDebt` ownership

Prefer moving `FullDebt` out of the kernel package if repository structure permits without unnecessary churn.

The desired architecture is:

    domain/application policy
        ↓
    supplies initial debt vocabulary
        ↓
    generic Solvent kernel
        ↓
    stores/manages opaque obligations

If moving the symbol would create unnecessary compatibility churn, it may remain temporarily as a convenience value, but the kernel creation semantics must not depend on it.

Document which layer owns the vocabulary after the change.

### 5. Database defaults

Inspect the current DDL default and migration behavior carefully.

Determine whether the database default containing the six deployment debts is still necessary.

Preferred end state:

- the kernel creation path explicitly supplies initial debt;
- database defaults do not silently reintroduce domain-specific vocabulary.

However, do NOT remove a DDL default blindly if `EnsureBelief` or another production path relies on it.

First trace all affected insert paths.

If the default is no longer needed, remove/revise it with the minimum migration required.
If it is still required for a compatibility reason, document why and ensure the normal domain-agnostic kernel path does not rely on it.

### 6. `EnsureBelief`

Preserve its current semantics.

Determine whether `EnsureBelief`:
- creates a missing belief,
- finds an existing belief,
- or does both.

For existing beliefs, do not overwrite existing debt merely because a different `initialDebt` was supplied.

The new parameter should only affect creation of a new belief unless repository semantics clearly require something different.

### 7. Public boundaries

Do not automatically expose arbitrary debt vocabulary through REST/MCP unless the current architecture requires it.

This change is about making the kernel generic, not expanding public APIs unnecessarily.

If REST/MCP creation paths already construct beliefs, update them only as required by the changed kernel/service signature and preserve their existing application-level vocabulary.

## Tests required

Add or update tests to prove:

### A. Existing Solvent behavior preserved

A normal Solvent/deployment caller passing the existing `FullDebt` receives the same six initial obligations as before.

### B. Arbitrary vocabulary accepted

Create a belief using a completely different vocabulary, for example:

    ["proof_check", "counterexample_search", "applicability_review"]

Verify:
- creation succeeds
- exact values are stored
- no kernel validation rejects them

### C. Empty initial debt is possible

Create a belief with:

    []

Verify:
- creation succeeds
- debt is empty
- the normal promotion invariant behaves as expected

Do not use this to bypass any existing unrelated promotion requirements.

### D. Kernel does not depend on FullDebt

A test should demonstrate that creating a belief with a non-FullDebt vocabulary does not require changing `FullDebt` or registering new debt names in the kernel.

### E. EnsureBelief behavior

Verify:
- new belief uses supplied initial debt
- existing belief is not unexpectedly overwritten by a later different initialDebt argument

### F. Existing retirement behavior

Arbitrary debt identifiers can be retired with existing generic `RetireDebt` semantics.

### G. No vocabulary validation in kernel

Prove that the kernel does not contain logic equivalent to:

    slices.Contains(FullDebt, item)

or a switch over named debt items.

MCP/service-level validation may remain where appropriate.

### H. Regression suite

Run all existing debt/promotion/kernel tests and verify no regressions.

## Resource-bound requirement

Do NOT add a maximum debt count.

The earlier review established that the actual representation is a `TEXT[]`, starts with a bounded initial set in current usage, and is subtractive-only.

After parameterization, re-check whether the generic creation API introduces an unbounded input path.

If a resource limit becomes necessary because `initialDebt` can now be arbitrarily large, do NOT invent a number.

First determine whether an existing request/transport/service boundary already provides an adequate limit.

Only introduce a new limit if there is concrete evidence of a resource/security problem.

If a limit is required, prefer the service/API/input boundary over the kernel unless there is a demonstrated reason the kernel itself must enforce it.

## Verification commands

Run as appropriate:

    gofmt -l cmd internal kernel api service adapter
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./kernel ./service/authority ./api ./cmd/solvent-mcp ./adapter/github
    task db:reset
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Also inspect the resulting diff carefully for accidental kernel/domain coupling.

## Documentation

Update the relevant architecture/engineering documentation to state:

> Debt vocabulary is policy/domain-owned. The kernel stores generic obligation identifiers and enforces their lifecycle and the promotion gate, but does not define what individual obligations mean.

Also document that:

> `FullDebt` is an existing deployment/application vocabulary, not a universal Solvent kernel vocabulary.

Do not create a new architecture document unless an existing document is the correct place.

## Acceptance criteria

All of the following must be true:

1. `EnterBelief` no longer intrinsically stamps `FullDebt`.
2. Existing Solvent callers explicitly provide the existing vocabulary where they need it.
3. A different domain can create beliefs with a completely different debt vocabulary through the normal kernel/service path.
4. Kernel code does not validate or interpret individual debt names.
5. `EnsureBelief` does not overwrite existing debt when called on an existing belief.
6. Promotion/debt invariants remain unchanged.
7. No arbitrary debt-count limit is introduced.
8. No workflow/policy-engine/idempotency machinery is added.
9. Full test suite is green.
10. Race tests are green.
11. I-7 and MCP verification remain green.
12. The resulting design is genuinely domain-agnostic at the debt-vocabulary level.

## Final output

Provide:

### 1. Files changed

Exact file list with one-line reason per file.

### 2. Architectural change

Explain exactly how debt vocabulary ownership moved from implicit kernel defaulting to caller/policy ownership.

### 3. Database impact

State whether the DDL default/migrations changed and why.

### 4. Test evidence

Report the actual commands and results.

### 5. Kernel-freeze assessment

State explicitly:

    KERNEL FREEZE PRESERVED

if the correction required no new kernel primitive or semantic invariant.

If any kernel growth was found to be necessary, STOP and report the reason instead of implementing unrelated kernel expansion.