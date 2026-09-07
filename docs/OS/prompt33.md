This revision is much closer. I would **not send OpenCode into implementation quite yet**, because there is one definite consistency error and two things worth tightening before coding.

### 1. Definite error: stale `GetSnapshotConsequenceParams` references remain

The new design correctly decided to eliminate that method and carry `consequence_parameters` through `AuthorizeResult`. 

But the later execution call flow still says:

> `GetSnapshotConsequenceParams(targetID) → {repo, workflow, ref}` 

That is now explicitly wrong.

The canonical flow should be:

```text
PrepareForAction
    ↓
kernel.Authorize
    ↓
AuthorizeResult.ConsequenceParameters
    ↓
service unmarshals approved snapshot params
    ↓
execParams
    ↓
executor
```

So this is just stale documentation, but **it must be fixed before implementation** because otherwise OpenCode has two conflicting implementation paths in the same plan.

### 2. The TOCTOU description should reflect the actual final check more precisely

The plan now says the post-authorization reads re-check revocation, but after the change to `AuthorizeResult`, the snapshot read is gone. 

Therefore the actual path is now approximately:

```text
T2 kernel.Authorize
   ↓
intent-state SELECT
   ↓
unmarshal already-returned snapshot params
   ↓
executor
```

There is **one DB read**, not two.

Update the wording from:

> “Two additional local DB reads…”

to:

> “One additional local DB read—the exact intent-state verification—occurs between T2 and T3. The approved snapshot parameters are already present in the AuthorizeResult returned at T2.”

That makes the accepted Window B boundary even smaller and more accurately reflects the implementation.

### 3. One thing I would explicitly preserve: CI-2

The plan is now correct here. `Success` is set immediately after the executor returns and is not overwritten by CompleteIntent failure. 

That is exactly the right reconciliation with the approved test target.

### What looks good now

The kernel-growth issue has been resolved elegantly. Instead of adding another kernel capability, the plan returns data the kernel already read as part of `AuthorizeResult`. 

The exact `intentID` is preserved end-to-end. 

The CompleteIntent persistence-failure semantics are now honest:

```text
provider accepted
→ Success=true
→ CompleteIntent fails
→ intent may remain live
→ duplicate side effect is possible
```

and that is explicitly deferred to Phase 4D. 

### Recommendation

Have OpenCode make **one final documentation-only cleanup**:

1. Remove the stale `GetSnapshotConsequenceParams` reference from the execution call flow and replace it with `decision.ConsequenceParameters`.
2. Change the TOCTOU description from **two DB reads** to **one DB read + in-memory parameter reconstruction**.
3. Re-run the consistency check against the approved test target.
4. Then stop.

After that, I would consider the implementation plan ready for **implementation**, without another architectural redesign.

Use this prompt:

```text
Phase 4C+ — Final Implementation Plan Cleanup

DO NOT implement code.

Update ONLY:
    phase4c_plus_imp_plan(2).md

The architectural decisions are now settled:

- CI-2 is authoritative: executor nil error → result.Success=true.
- CompleteIntent failure is a documented v1 persistence limitation.
- Exact intentID is propagated end-to-end.
- Snapshot consequence_parameters are returned through AuthorizeResult.
- No GetSnapshotConsequenceParams kernel method exists.
- Window B remains a documented v1 TOCTOU limitation.

Make these final consistency corrections:

1. REMOVE ALL stale references to:
       GetSnapshotConsequenceParams(...)

The current call-flow section still incorrectly says:

    GetSnapshotConsequenceParams(targetID) → {repo, workflow, ref}

Replace it with:

    decision.ConsequenceParameters
        → JSON unmarshal in service
        → execParams = {repo, workflow, ref}

The service must obtain approved provider parameters exclusively from
AuthorizeResult.ConsequenceParameters.

2. UPDATE THE TOCTOU DESCRIPTION.

The current plan still describes two additional DB reads between T2 and T3.

That is no longer correct.

The actual path is:

    T2: kernel.Authorize succeeds
        ↓
    intent-state SELECT for exact intentID
        ↓
    unmarshal decision.ConsequenceParameters in memory
        ↓
    construct execParams
        ↓
    T3: executor invocation

Explicitly state:
- only one additional database read occurs after T2
- snapshot consequence_parameters are already returned by Authorize
- parameter reconstruction after T2 is in-memory
- the residual Window B race is after the final authorization check and
  intent-state read, before executor invocation
- no claim of atomic DB authorization + provider execution is made

3. UPDATE ANY OTHER STALE TEXT that implies:
- a second snapshot DB query
- GetSnapshotConsequenceParams exists
- snapshot parameters are fetched after Authorize

The approved architecture is:

    kernel.Authorize
        ↓
    AuthorizeResult.ConsequenceParameters
        ↓
    service reconstruction
        ↓
    executor

4. CONSISTENCY CHECK

Verify the final implementation plan against:

    docs/OS/phase4c_plus_test_target.md

Specifically confirm:

- CI-2 unchanged
- TestExec10-13 remain kernel-level tuple tests
- TestExec36 remains execution-path snapshot-source-of-truth test
- TestExec35 remains provider-success authority-integrity test
- TestExec15A remains revocation-before-final-check denial
- TestExec15B remains deterministic Window B characterization
- TestExec27-30 remain CompleteIntent tests
- TestExec38 remains provider-success/Solvent-error behavior
- TestAT01-16 remain valid
- exact intentID is used for CompleteIntent
- caller params are not used as execution source
- no GetSnapshotConsequenceParams method is required

5. SCOPE

Do not:
- implement code
- modify the test target
- add new services
- add schema changes
- add new kernel primitives
- redesign the TOCTOU model
- add idempotency
- add async execution

After the consistency check, STOP.

Report only:
- what stale references were corrected
- the final T2→T3 call sequence
- whether the plan is internally consistent with the approved test target
```

After that cleanup, I would **move to implementation rather than another design-review loop**. The current plan's central architecture is now coherent. 
