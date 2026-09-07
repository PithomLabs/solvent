This revision is **better on the three major design questions**, but I would still stop before implementation. There is one **critical semantic contradiction** and a couple of secondary issues.

### 1. Critical: Decision A now conflicts with approved CI-2

The approved test contract says:

> `ExecutionResult.Success` is true **iff the executor function returned nil error**. 

The revised implementation plan says:

```text
provider accepts
    ↓
executor returns nil
    ↓
CompleteIntent fails
    ↓
result.Success = false
```

That is explicitly what the proposed implementation does. 

That means the implementation plan would violate the already-approved test target.

This needs to be resolved **before coding**.

My recommendation is to preserve the approved CI-2 semantics:

```text
executor returns nil
→ external provider accepted
→ ExecutionResult.Success = true
```

Then treat `CompleteIntent` persistence failure as a **separate post-provider persistence failure**, with the known consequence that intent may remain live and duplicate execution becomes possible.

That is not ideal, but it is at least internally consistent with the approved contract. More importantly, do not silently alter the meaning of `ExecutionResult.Success` after Kilo has approved the contract.

### 2. The duplicate risk is now honestly documented

The revised plan does a good job of acknowledging the real consequence:

> provider accepted → CompleteIntent fails → intent remains live → later execution may duplicate the external side effect. 

That honesty is good. But because this is now a known behavioral consequence, the implementation plan should explicitly say whether this is:

```text
known v1 limitation
```

or

```text
implementation blocker
```

Given the existing Phase 4D boundary, I would keep it as a known v1 limitation rather than introduce idempotency now—but **do not change CI-2 to accommodate it**.

### 3. `intentID` propagation is substantially better

This part is now structurally sound. The plan explicitly carries the exact ID through `ExecuteAction` and uses that exact ID for `CompleteIntent`, eliminating the previous `LIMIT 1` rediscovery problem. 

That is the correct direction.

### 4. `GetSnapshotConsequenceParams` is still worth challenging

The plan's justification for adding the kernel read method is reasonable in principle: avoid duplicating snapshot-resolution logic outside the kernel. 

But the proposed implementation has a technical problem:

```go
s.db.QueryRowContext(ctx, sqlAuthorizeResolve, targetID).Scan(
    nil, nil, nil, nil, nil, nil, nil, &paramsJSON, nil,
)
```

The implementation plan should not proceed with this exact code without checking the actual `sqlAuthorizeResolve` column list and `database/sql` scan semantics. `Scan` expects valid destination arguments; literal `nil` is not a meaningful scan destination.

More importantly, the plan should explicitly verify that the query's result shape is actually compatible with this reuse rather than assuming it.

### 5. The snapshot read creates another post-authorization read

The execution path is now:

```text
PrepareForAction / final authorization
        ↓
intent state read
        ↓
snapshot consequence-parameter read
        ↓
executor
```

The plan's approved TOCTOU model is about the final authorization check immediately preceding the executor. 

Adding multiple database reads between authorization and execution makes the actual window larger than the implementation plan describes.

This isn't necessarily a security failure, because the plan already accepts Window B, but the wording should be precise: the final authorization check is no longer essentially “check → immediate function call.” There are additional reads between them.

So either:

* move/restructure the snapshot reconstruction so the approved values are obtained as part of the same authoritative read path, or
* explicitly acknowledge that those reads are part of the accepted pre-execution window.

I strongly prefer the first if existing kernel/service structures permit it.

### 6. Small bookkeeping error

The plan says:

> “4 new files and modifying 7 existing files”

but the summary table lists `authority.go` twice conceptually while the total says **6 modified files**. 

That is minor, but clean it up before handing the plan to another reviewer.

---

## My disposition

**Do not implement yet.**

The plan needs one focused revision:

> **Resolve the contradiction between Decision A and approved CI-2.**

Then verify the `sqlAuthorizeResolve` reuse against the actual query shape and tighten the TOCTOU wording around the additional snapshot/intent reads.

The architectural direction itself is now good: exact `intentID`, snapshot-derived parameters, fixed executor mapping, minimal kernel changes, and explicit v1 limitations. 

The next prompt to OpenCode should therefore **not reopen the whole design**. It should specifically force reconciliation with the already-approved test contract before implementation.
