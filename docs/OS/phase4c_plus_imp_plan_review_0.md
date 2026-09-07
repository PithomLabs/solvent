I would **not let OpenCode start implementing this Step 1 plan yet**.

The test-engineering gate passed, but this implementation plan introduces a few new design issues that were not present in the approved test target.

### 1. `GetSnapshotConsequenceParams` is an unexpected kernel expansion

The plan introduces:

```go
GetSnapshotConsequenceParams(...)
```

inside `kernel/authority.go`, while the kernel-growth gate previously approved only `CompleteIntent`. The plan also does not explicitly add `GetSnapshotConsequenceParams` to `Contract`, even though the service code calls `s.kern.GetSnapshotConsequenceParams(...)`.  

That creates two problems:

1. Potential compile/interface mismatch if `s.kern` is typed as `Contract`.
2. More importantly, it expands the kernel API solely to retrieve execution data.

I would first determine whether the approved snapshot is already available through an existing kernel result/path. The guiding rule remains: **do not add another kernel primitive merely because the service needs data**.

### 2. The biggest issue: `CompleteIntent` failure can cause replay

This is the one I would treat seriously.

The plan does:

```go
if err == nil {
    _ = s.kern.CompleteIntent(ctx, scenarioID, intentID)
}
```

and explicitly says CompleteIntent failure does **not** fail the execution result. 

Suppose:

```text
Authorization succeeds
        ↓
GitHub accepts workflow
        ↓
Solvent returns Success=true
        ↓
CompleteIntent DB update fails
        ↓
intent remains live
```

The next execution can see the intent as `live` and invoke GitHub again.

That means the previously claimed sequential duplicate protection:

```text
live → executed
```

is no longer reliable in the exact failure case where the external side effect succeeded but Solvent failed to record acceptance.

The plan itself lists “CompleteIntent failure is best-effort” as a known limitation.  But this is not merely an observability limitation; it can affect **repeat external side effects**.

This needs an explicit design/test decision before implementation.

At minimum the test target should have a case like:

```text
provider accepts
CompleteIntent fails
first execution = truthful success
intent remains live

second execution:
what is the intended behavior?
```

You need to consciously decide whether that replay is an accepted v1 risk. I would be uncomfortable accepting it silently for a consequential `deploy` action.

### 3. `CompleteIntent` identifies the intent indirectly

The implementation does:

```sql
SELECT id
FROM action_intent
WHERE scenario_id = ...
  AND belief_id = ...
  AND action = ...
  AND state = 'live'
LIMIT 1
```



That is weaker than passing the **exact intent ID** through the execution path.

If there can ever be multiple live intents matching those fields, `LIMIT 1` can select the wrong one.

The approved architecture should ideally be:

```text
authorized intent_id
        ↓
executor
        ↓
provider accepted
        ↓
CompleteIntent(exact intent_id)
```

not:

```text
provider accepted
        ↓
search for some live intent matching scenario/belief/action
```

This is especially important because the system is supposed to maintain exact action/target binding.

### 4. The implementation plan's test organization is somewhat misleading

It says Track A and Track B are independent, then Track C depends on both, and ultimately `executor_test.go` contains the complete matrix of DB/service/kernel behavior. 

That is workable, but I would keep a strong separation between:

```text
adapter/github/executor_test.go
    provider/executor behavior

service/authority/..._test.go
    authorization → snapshot reconstruction → executor wiring

kernel/..._test.go
    CompleteIntent atomic transition
```

Otherwise the adapter test package becomes a 1,200-line integration harness that obscures which layer is actually being tested.

### 5. There is a subtle contradiction in TestExec36 implementation expectations

The approved target now correctly says conflicting caller parameters are ignored and snapshot values are used. The Step 1 plan implements this by constructing:

```go
execParams := map[string]interface{}{
    "repo":     snapParams["repo"],
    "workflow": snapParams["workflow"],
    "ref":      snapParams["ref"],
}
```

which is good. 

But note that this implementation deliberately passes only three known keys, while the kernel's `consequence_parameters` is intentionally opaque JSON.

That creates a future extensibility constraint:

```text
opaque JSONB in kernel
        ↓
hardcoded repo/workflow/ref extraction
```

For this narrowly scoped GitHub executor, that's probably acceptable. But the implementation should **not pretend this is generic provider parameter reconstruction**. Keep it explicitly GitHub-specific above the kernel.

### My disposition

I would put the implementation plan at:

**REQUEST CHANGES before implementation.**

Not because the architecture has gone off the rails, but because the plan has introduced **two concrete correctness/security questions** that the approved test contract does not fully resolve:

1. **What happens after provider acceptance when `CompleteIntent` fails?**
2. **How is the exact `intent_id` bound through execution instead of rediscovered with `LIMIT 1`?**

And separately:

3. Remove or justify the additional kernel method `GetSnapshotConsequenceParams`.

The test target itself is still **GO**; Kilo already established that. 

So I would **not send this implementation plan to Kilo yet**. First have OpenCode revise the implementation plan around those points, while keeping the approved test target unchanged unless a genuine new invariant is required.
