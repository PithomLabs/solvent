This revision fixed the **CI-2 contradiction** correctly. The semantics are now internally coherent:

> executor returns nil → `Success=true`, even if `CompleteIntent` persistence subsequently fails. 

The exact `intentID` propagation is also now correct, and the `LIMIT 1` lookup is gone. 

The TOCTOU description is also substantially more honest: it explicitly acknowledges the two additional reads between T2 and T3 and identifies the residual race as occurring after those reads. 

However, **there is still one concrete implementation blocker**.

## The `Contract` interface is incomplete

The plan adds:

```go
GetSnapshotConsequenceParams(...)
```

to `kernel.Store`, and the service calls:

```go
s.kern.GetSnapshotConsequenceParams(...)
```

But the plan only adds `CompleteIntent` to the `Contract` interface. 

So unless `s.kern` is concretely typed as `*kernel.Store` rather than `kernel.Contract`, this will not compile.

Earlier the plan itself established that the service depends on the kernel contract. The implementation plan needs to make this explicit and consistent:

```text
Contract
  ├── CompleteIntent(...)
  └── GetSnapshotConsequenceParams(...)
```

or, alternatively, change the service dependency so it deliberately uses a narrower interface that contains the required methods.

Given the existing architecture, I would prefer **adding `GetSnapshotConsequenceParams` to the kernel `Contract` interface**, with the same kernel-growth justification already documented.

That is a small mechanical fix, but it should be resolved **before implementation starts**.

### One more thing I would tighten

The new `intent_completion_failed` audit event is sensible and correctly distinguishes:

```text
provider accepted
≠
Solvent persisted execution state
```

The plan explicitly makes that distinction. 

But because this is a new audit event, the implementation plan should explicitly state whether the existing audit model permits adding this event type without changing the approved test contract. This does not look like an architectural problem; it is mainly a consistency/documentation check.

## Verdict

**Do not send this to Kilo yet.**

Have OpenCode make one focused correction:

> **Add `GetSnapshotConsequenceParams` to the appropriate kernel contract/interface path and verify the resulting dependency compiles.**

Then do a final consistency pass. After that, **this should be ready for Kilo's implementation-plan review or for implementation, depending on your chosen process.**

Use this prompt:

```text id="2q0gk2"
Phase 4C+ — Final Implementation-Plan Compile-Path Fix

Do NOT implement production code yet.

The implementation plan now correctly resolves the CI-2 contradiction,
exact intentID propagation, snapshot parameter binding, and the honest
post-authorization TOCTOU boundary.

There is one remaining concrete issue to resolve before implementation:

`GetSnapshotConsequenceParams(...)` is added to `kernel.Store` and is called
from `service/authority.ExecuteAction`, but the current plan only adds
`CompleteIntent(...)` to the kernel `Contract` interface.

Resolve this dependency consistently.

TASK:

1. Inspect the actual type of `s.kern` in `service/authority.Service`.

2. If it is typed as `kernel.Contract`, then add:

    GetSnapshotConsequenceParams(...)

   to the `Contract` interface and update the implementation plan accordingly.

3. If the architecture deliberately uses another interface, define the
   smallest correct interface boundary and update the plan accordingly.

4. Preserve the kernel-growth justification already documented:
   - read-only
   - reuses kernel snapshot-resolution semantics
   - consequence_parameters remain opaque to kernel
   - provider-specific interpretation remains in service/adapter
   - no new authority semantics

5. Verify the complete dependency path in the plan:

    service/authority
        →
    kernel contract/interface
        →
    kernel Store
        →
    snapshot-parameter query

6. Check that there are no other interface/signature mismatches introduced by:
   - ExecuteAction(intentID)
   - CompleteIntent
   - GetSnapshotConsequenceParams
   - new audit event

7. Do not change the approved test target.

8. Do not implement production code.

9. Do not add new architecture.

10. Update `phase4c_plus_imp.md` only.

After editing, perform a final plan-consistency audit against:

    docs/OS/phase4c_plus_test_target.md

Explicitly report:
- whether `s.kern` dependency is now compile-consistent
- whether all new methods are present on the required interface
- whether ExecuteAction signature and all call sites are accounted for
- whether CompleteIntent and snapshot retrieval are correctly placed

Then STOP.
```
