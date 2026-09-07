I would **not proceed with this plan exactly as written**. It is close, but there is one substantive correction required.

### N-1: good

The audit failure seam is sound. The plan keeps `auditLogger` unexported and preserves `*audit.Service` compatibility, while the fake fails specifically on `ActivityExecutorCompleted`. The assertions correctly establish:

```text
provider accepted
    ↓
CompleteIntent succeeded
    ↓
executor_completed audit write fails
    ↓
result.Success == true
```

That is a legitimate test of the audit-persistence gap under CI-2.  

One small concern remains: `SetTestHook` is technically an exported mutable method even though the hook type is unexported. The plan calls it a “test seam,” but production code could invoke it. This is not enough to block the phase by itself, but keep it as narrowly scoped as possible. 

### N-2: change `RetractCascade` back to `RevokeTarget`

This is the important issue.

The plan currently says:

```go
st.RetractCascade(ctx, sid, beliefID)
```

That is broader than the race being characterized. `RetractCascade` changes the underlying belief state and cascades through dependent state. It therefore tests a larger semantic operation than “authority was revoked after T2.”

The clean Window B experiment should be:

```text
T2: kernel.Authorize → ALLOWED
        ↓
intent verification
        ↓
deterministic hook
        ↓
RevokeTarget(targetID)
        ↓
executor invocation
```

This isolates exactly the security boundary under review: **target revocation after authorization but before external execution**.

Your prior decision to use `RevokeTarget` was correct. The current uploaded plan accidentally reverted that decision. The plan itself says Window B is the narrow gap immediately before executor invocation, so the revocation operation should match that boundary. 

### There is also a wording correction

The test currently says:

> “Wait for T2 (authorization) to complete.”

But the hook is actually after:

```text
Authorize
→ intent-state SELECT
→ parameter reconstruction
→ adapter_invoked audit
→ hook
```

So the signal should be described as:

> **“final authorization and intent-state verification completed”**

That makes the test's proof claim precise. 

### Tell OpenCode this

```text
Proceed, but make these corrections before implementation:

1. TestExec15B MUST use:
       st.RevokeTarget(ctx, targetID)
   instead of:
       st.RetractCascade(ctx, sid, beliefID)

   The purpose is to characterize the exact Window B race:
       final Authorize succeeds
       → exact intent state is verified
       → target is revoked
       → executor still proceeds.

   Do not broaden the test by retracting the belief/cascading state.

2. Rename/reword the synchronization comment from:
       "Wait for T2 (authorization) to complete"
   to:
       "Wait for final authorization and intent-state verification to complete"

   The hook occurs after authorization, intent verification, parameter
   reconstruction, and adapter_invoked audit, immediately before executor
   invocation.

3. Keep the deterministic channel synchronization. No sleeps, timers,
   polling, or probabilistic race.

4. Preserve all existing architecture and CI-2 semantics.

5. For SetTestHook, keep it strictly as a test seam and avoid introducing
   additional production behavior beyond what is required to inject the test
   synchronization point.

After these corrections, proceed with implementation and run the listed
verification commands.
```

With that correction, the plan is **GO for implementation**. The core N-1/N-2 approach is now testing the actual missing properties rather than merely documenting them.
