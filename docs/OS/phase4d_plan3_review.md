Three things here — the Kilo test-target review from last turn that I owe a response to, `phase4d_plan3.md` itself, and document 18's review of it. I read `phase4d_plan3.md` directly rather than relying on the review's paraphrase, which turned up something the review missed and also let me check the review's own central claim against the source.

## My own finding on `phase4d_plan3.md`: the standing stale-authority-at-execution-time gap is still open, and this plan doesn't touch it

This is the third planning document in a row where this specific concern surfaces, and it's still unresolved. §7a's new step ordering is:

```
1. PrepareForAction (kernel.Authorize)     — the authority check
2-5. resolve executor, reconstruct params, validate
6. ClaimIntent (atomic CAS live→executing) — the durable ownership gate
7-8. log, invoke provider
```

`ClaimIntent`'s CAS predicate is `state = 'live'` — it says nothing about revocation. So if `RevokeTarget` commits between step 1 and step 6, `ClaimIntent` still succeeds, because the intent's `state` column is completely independent of the target's authority status. This plan is excellent at solving a real, different problem — at-most-once execution across crashes, which `SELECT ... FOR UPDATE` genuinely can't provide, and the "why not row locking alone" section (§5) is honest and correct about that — but it doesn't shrink or address the gap between "authority confirmed valid" and "execution durably claimed," and the new steps 3-5 sitting between them arguably make that window a bit longer, not shorter. This is the same concern I raised on `plan_4c_plus.md`, which got a partial, not-quite-right answer in `plan_4c_plus_1.md` ("sequential MCP execution" doesn't prevent a concurrent REST-initiated revoke), and it's still sitting open here, in a completely reworked execution model, several planning documents later. I'd want this named explicitly as an accepted residual risk in §15's risk table — it currently isn't listed at all — with the honest reasoning (narrow, single-process, no additional I/O between check and claim) rather than continuing to let it pass unmentioned from document to document.

## Document 18's headline finding on `phase4d_plan3.md` doesn't hold up against the plan's own code

Document 18's "main thing I would change" is that `ProviderOutcome`/`ProviderError` are placed in `adapter/github`, which it says reverses the dependency direction and makes the service import the GitHub adapter just to understand a generic outcome. I checked this directly. §7b's contract header is admittedly sloppy — *"Contract in `adapter/github/provider.go` (or a shared package)"* — but the actual downstream usage, in the service-layer code shown at lines 408-412, imports `adaptererrors.ProviderError` and `adaptererrors.ProviderAccepted`/etc. — a distinct, neutral package name, not `adapter/github`. The GitHub-specific classification function (`classifyGitHubError`) is separately placed in `adapter/github/http_provider.go` and *returns* the neutral type; it doesn't define it there. That's exactly the dependency direction document 18 is asking for — GitHub adapter produces the generic type, service consumes only the generic type — just under a different package name (`adaptererrors`) than the one it suggested (`service/executor`). This is worth being direct about: the review's central, most-emphasized recommendation is based on the ambiguous section header, not the code that resolves the ambiguity two paragraphs later. I'd fix the header wording (drop "or a shared package," just say `adaptererrors` plainly) rather than treat this as an architecture problem — there isn't one here.

Document 18's other two points are smaller and I largely agree with them at the severity it assigned:

- **CancelIntent clarity** — reasonable ask, already substantially addressed. §4's transition table already states `executing → cancelled` is reachable "Never" via `RetractCascade`, and `CancelIntent` is already scoped as reachable only through `ReconcileIntent` with an authenticated `operatorID`, explicitly separate from normal retraction. One more doc-comment sentence would make this fully unambiguous, but the design already has the right shape.
- **Snapshot lookup query equivalence** — a fair thing to ask for verification of on principle, but I think it resolves cleanly once you account for something established many rounds ago: `target_activation` has `UNIQUE(target_id)` over the row's *entire history*, meaning at most one activation ever exists per target, and its joined snapshot is immutable. So `WHERE ta.target_id = $1` can't return an ambiguous or superseded row — there's only ever one candidate, revoked or not — and the actual authorization decision still runs through the untouched `kernel.Authorize`, which does check revocation. This read only supplies parameter data; it isn't a second authority interpretation. Worth a one-line comment in the code confirming that reasoning, not a design change.

## Kilo Code's test-target review (`phase4c_plus_test_target_review.md`)

I hadn't responded to this yet — it's precise and I agree with its top three findings. F-2 (SI-9 has zero test coverage — the traceability matrix maps it to a test that proves the opposite case) is exactly the kind of mapping error this project has caught in itself multiple times now, and F-3 (parameter binding tested at the authorization level but never proven at the executor level) is the same class of gap I raised on the implementation plan one level up — good that it surfaced again independently at the test-specification layer rather than being assumed closed. F-4 (a test whose acceptance criterion is "may or may not happen" isn't a test) is a correct and important methodological point. I don't have the revised test target or the final Phase 4C+ GO review that presumably followed this, so I can't confirm F-1 through F-9 actually got closed before 4C+ shipped — if those documents exist, I'd want to check them directly rather than assume the dependency line in `phase4d_plan3.md` ("Depends on: Phase 4C+ (GO)") means everything here was verified. But that's a documentation-trail question, not a defect in Kilo's review itself, which I think was executed well.


## chatgpt

Yes. Your critique identifies the **one substantive architectural issue I would add to the Phase 4D gate**.

You are right that I focused too much on the execution-idempotency state machine and not enough on the fact that `ClaimIntent` is currently **authority-blind**.

## The real gap

The proposed Phase 4D sequence is:

```text id="r2tr9v"
T2: kernel.Authorize → ALLOWED
        ↓
resolve executor
        ↓
reconstruct params
        ↓
validate
        ↓
ClaimIntent: live → executing
        ↓
provider
```

`ClaimIntent` only checks:

```sql
state = 'live'
```

So this is entirely possible:

```text id="j0q2ct"
T2
│
├── authority valid
│
├── target revoked
│
└── ClaimIntent succeeds
        ↓
      provider
```

The revocation is invisible to the CAS.

That means Phase 4D introduces a **newly important gap between authorization and durable execution ownership**. And, as you correctly note, steps 3–5 actually create more space between those two events. The plan currently does not name this residual risk in §15. 

This is conceptually distinct from the existing Window B.

You now have:

```text id="e6b8lc"
Window A
revocation before final Authorize
→ DENY

Claim gap
revocation after Authorize but before ClaimIntent
→ Claim may still succeed

Window B
revocation after ClaimIntent but before provider
→ provider may proceed
```

Actually, once `ClaimIntent` exists, I would define the boundaries more cleanly as:

```text id="d7gm3q"
T2 = final authority check

T3 = successful ClaimIntent

T4 = provider invocation
```

Then:

```text T2 → T3
authority/claim race

T3 → T4
execution Window B
```

That's much clearer than treating everything after T2 as one race.

## The important question: do we need to eliminate it?

**Not necessarily for Phase 4D.**

Given the project's philosophy, I would not immediately add another kernel primitive merely because we can imagine a race.

But the plan must make an explicit decision:

### Option 1 — Accept it

State clearly:

> Phase 4D guarantees atomic execution ownership, but does not atomically bind current authority validity to the ownership claim. A revocation committed after final `Authorize` and before `ClaimIntent` may therefore still permit the intent to enter `executing`.

Then add this to the risk table and acceptance model.

That is honest, but it means Phase 4D has a somewhat larger residual authorization race than the current document admits.

### Option 2 — Close it atomically

The stronger architecture would make the authority check and claim occur within the same kernel transaction:

```text id="7mawmf"
current authority validation
        +
live → executing CAS
        ↓
ONE atomic kernel operation
```

Conceptually:

```text
AuthorizeAndClaimIntent(...)
```

or make `ClaimIntent` itself accept/revalidate the complete authority tuple.

That would turn:

```text
Authorize
    ↓
[gap]
    ↓
Claim
```

into:

```text
atomic:
    verify authority
    +
    claim intent
```

This is one of the rare cases where a new kernel primitive could actually be justified: it would establish a **durable atomic security transition**, not merely convenience.

But I would **not decide that yet** without inspecting the current `action_intent` schema and `authorizeWithinTx` implementation. We need to determine exactly what information is available in the intent and whether the existing kernel transaction machinery can combine the two without duplicating authority semantics.

---

# Your correction about `ProviderOutcome`

I also agree with you about my previous review.

I over-read the ambiguous wording:

> “Contract in `adapter/github/provider.go` (or a shared package)”

The actual plan subsequently uses `adaptererrors.ProviderError`, while GitHub-specific classification occurs in `adapter/github/http_provider.go`. That is a perfectly reasonable dependency direction. 

So I would **not treat ProviderOutcome placement as an architecture defect**.

At most, fix the plan's wording to say:

```text
Generic provider outcome types live in adaptererrors.

GitHub adapter maps GitHub-specific failures into those generic outcomes.

service/authority consumes only adaptererrors.ProviderError /
ProviderOutcome and does not inspect GitHub HTTP semantics.
```

That removes the ambiguity.

Your correction is fair.

---

# `CancelIntent` and snapshot lookup

I also agree with your assessment here.

`CancelIntent` is already scoped as a reconciliation operation, not normal retraction. The plan explicitly keeps `executing` untouched by `RetractCascade`, and the reconciliation section ties it to an authenticated operator.  

Likewise, the snapshot lookup concern is mostly a verification/documentation issue given the established `UNIQUE(target_id)` invariant. I would not redesign that API path merely because the query is duplicated.

---

# So I would change the Phase 4D plan in one important way

Add a section immediately after the execution ordering:

```text
## Residual Authority/Claim Race

Final authority validation and ClaimIntent are separate operations.

If target authority is revoked after the final kernel.Authorize succeeds but
before ClaimIntent commits, ClaimIntent can still succeed because its CAS
predicate only checks action_intent.state = 'live'.

Therefore Phase 4D does NOT provide atomic:
    authority validation + execution claim

This is a distinct residual race from the post-claim Window B race.

Phase 4D must explicitly choose one of:

A. Accept and document this race as a v1 limitation; or

B. Introduce an atomic kernel operation that validates current authority and
   claims the intent in the same transaction.

No stronger claim may be made unless option B is implemented.
```

And add to the risk table:

```text
| Risk | Severity | Mitigation |
| Revocation between final Authorize and ClaimIntent | Medium | Explicitly documented residual race; or close with atomic authorization+claim |
```

Then add an acceptance/characterization test specifically for it.

---

# I would NOT let the coding agent implement yet

This is now the right next instruction:

```text id="f4q0xb"
Revise Phase 4D plan again before implementation.

1. Keep the current state machine and idempotency design.

2. Explicitly identify a distinct race between:
       final kernel.Authorize
       and
       ClaimIntent(live → executing)

   ClaimIntent currently checks only intent state and does not revalidate
   current target authority/revocation.

3. Name this separately from the existing post-claim Window B race.

4. Add the race to the failure model and risk table.

5. Add a deterministic characterization test for:
       Authorize succeeds
       → authority is revoked
       → ClaimIntent still succeeds

   unless the architecture is changed to close the gap.

6. Then evaluate whether closing this race requires an atomic kernel
   authorization+claim operation.

   Do not add that primitive yet. First inspect whether the existing
   authorizeWithinTx and intent schema can support combining the operations
   without duplicating authority semantics.

7. Correct the ProviderOutcome documentation:
   generic ProviderOutcome/ProviderError types live in `adaptererrors`;
   GitHub adapter performs GitHub-specific classification and returns the
   generic result; service consumes only generic outcomes.

8. Leave CancelIntent and snapshot lookup architecture unchanged, with only
   clarifying documentation where necessary.

9. STOP. Do not implement code yet.
```

This is worth doing before Phase 4D implementation because **idempotency and authority validity are two different axes**. Phase 4D is now very good on the first axis; the plan needs to be equally explicit about the second.



## claude

This is the right response, and I want to be specific about why rather than just agree: it doesn't just accept my finding, it improves on how I framed it. Splitting the post-`Authorize` window into two named boundaries — the Authorize→ClaimIntent gap and the ClaimIntent→provider gap (Window B) — is more precise than treating everything after T2 as one undifferentiated race, because the two gaps have different characters: one is about a CAS predicate that's blind to a fact (revocation) it was never designed to check, the other is about the inherent unlockability of a network call. Naming them separately means each can get its own honest disposition instead of one blurred risk line.

I also think the instinct not to add `AuthorizeAndClaimIntent` reflexively is correct, and for the right reason — this project has earned real credibility by treating "we can imagine a race" as insufficient justification for kernel growth on its own, and applying that same discipline here rather than making an exception because the finding came from a security review is consistent, not inconsistent. Option A/Option B as an explicit fork, with a real decision required rather than a default, matches how ADR-0002 handled the analogous "duplicate SQL vs. extend the kernel" question in Phase 4B — that's the right process to reuse here, not a new one invented for this case.

One thing worth being precise about before this goes back to the plan: whichever option gets chosen, the characterization test in step 5 needs to actually construct the race under controlled ordering — a goroutine that calls `Authorize`, signals, waits while a revocation commits, then proceeds to `ClaimIntent` — the same discipline T-C9 Part A/A2 used, not a test that merely asserts the current (accepted) behavior in prose. Given this exact project's history includes a test that asserted "provider MAY be called" and provided no adversarial value because of it, I'd want the acceptance criterion for this new test stated as a concrete, deterministic outcome before anyone writes it, not "characterizes" as an open-ended verb — same objection Kilo raised for Window B applies here in advance if it isn't pinned down now.

Otherwise this is ready to send back to planning as written. Good catch generalizing the `ProviderOutcome` correction into the cleaner replacement wording, and I have nothing further on `CancelIntent` or the snapshot lookup.


