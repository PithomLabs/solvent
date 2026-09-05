This result changes the disposition materially.

**Phase 4B is NOT ready to freeze. Do not proceed to Phase 4C.**

The implementation has uncovered a real flaw in our previous assumption: **putting `Authorize` and intent creation inside one `SERIALIZABLE` transaction is not, by itself, sufficient to prevent the stale-authority race in this schema.**

The Phase 6.3 test has now demonstrated that CockroachDB does **not** treat the `NOT EXISTS (target_revocation)` predicate as a conflict with a concurrent insertion into `target_revocation`. The stale intent can therefore commit. That is a genuine security-correctness finding, not a test problem.

The screenshot says this explicitly:

> “CockroachDB's SERIALIZABLE isolation does NOT detect NOT EXISTS predicate conflicts on new row inserts. The stale intent commits successfully.”

That invalidates the previous security argument for `AuthorizeAndCreateIntent`.

## What this means

The current model is:

```text
T1: AuthorizeAndCreateIntent
    read: target_revocation does not exist
             ↓
T2: RevokeTarget
    insert target_revocation
    commit
             ↓
T1: insert action_intent
    commit
```

So we currently have:

```text
ONE TRANSACTION
        ≠
SUFFICIENT SERIALIZATION
```

The atomic primitive still gives us atomicity between its own statements, but it does **not** establish the required ordering against the independent revocation transaction.

That is exactly why your kernel-growth rule matters. We now have evidence that the existing representation cannot safely express the security transition with the current primitives.

## The correct architectural response

Do **not** abandon `AuthorizeAndCreateIntent`.

Instead, introduce an explicit serialization point shared by **both authorization and revocation**.

The cleanest minimal approach is likely:

```text
AuthorizeAndCreateIntent
        ↓
lock existing target row
        ↓
read activation/snapshot/revocation
        ↓
create intent
        ↓
commit
```

and:

```text
RevokeTarget
        ↓
lock SAME existing target row
        ↓
insert revocation
        ↓
commit
```

Now the ordering becomes deterministic:

```text
Case A

Authorize locks target
    ↓
creates intent
    ↓
commits
    ↓
RevokeTarget obtains lock
    ↓
revokes

= valid intent created before revocation
```

or:

```text
Case B

RevokeTarget locks target
    ↓
inserts revocation
    ↓
commits
    ↓
Authorize obtains lock
    ↓
sees revocation
    ↓
DENY
```

The forbidden state:

```text
revoke commits
    ↓
stale authorization creates intent
```

is eliminated because both operations must serialize on the same durable row.

### Important

Locking only in `AuthorizeAndCreateIntent` is **not enough**.

`RevokeTarget` must participate in the same serialization protocol.

Otherwise:

```text
Authorize locks target
RevokeTarget ignores lock → inserts revocation
Authorize continues
```

and we've gained nothing.

So the next adversarial plan needs to determine the exact existing row to use and verify the CockroachDB locking semantics experimentally before modifying production code.

## T-C10 is now extremely valuable

The old implementation proof is good:

```text
Authorize()
    ↓ commit
RevokeTarget()
    ↓ commit
IntentOnPromoted()
    ↓
STALE INTENT
```

That gives us a deterministic regression baseline.

Keep it.

## T-C9 Part A needs to be rewritten

It currently claims:

```text
SERIALIZABLE
+
NOT EXISTS
+
concurrent INSERT
=
40001
```

The experiment has disproved that assumption.

So don't weaken the test. **Correct the architecture and then change the mechanism test to prove the mechanism we actually adopt.**

For example:

```text
T-C9

Transaction A:
    acquire target serialization lock
    evaluate authority
    create intent

Transaction B:
    acquire same target serialization lock
    revoke target
```

Then verify the two possible legal orderings and the forbidden state.

## T-C9 Part B also needs reclassification

The screenshot says:

```text
stale=false
outcome=valid_pre_revocation_intent
```

That is a legitimate outcome, but it doesn't prove atomicity.

It proves:

```text
authorize won the race
```

not:

```text
stale authority is impossible
```

The test needs to exercise both orderings, or otherwise deliberately force the revocation-first case.

## New disposition

I would record the finding like this:

```text
F-NEW: SERIALIZABLE predicate-read assumption is invalid

Severity: CRITICAL
Classification: Phase 4B security correctness

Finding:
AuthorizeAndCreateIntent relies on a NOT EXISTS predicate over
target_revocation to serialize against a concurrent revocation.
Actual CockroachDB behavior allows the revocation INSERT to commit
without aborting the authorization transaction.

Impact:
A live action_intent can be committed after target authority has
already been revoked.

Status:
CONFIRMED
```

This is now a **real security blocker**.

## Next step

Do not create another broad redesign document.

Create a narrowly scoped **Phase 6.4 — Explicit Authority Serialization** plan with these requirements:

```text
1. Preserve the smallest-kernel philosophy.
2. No schema changes unless experiments prove absolutely necessary.
3. Keep one authority engine.
4. Keep AuthorizeAndCreateIntent.
5. Introduce the smallest shared serialization mechanism that is honored
   by both authorization and revocation.
6. Prove the mechanism experimentally against CockroachDB before coding.
7. Make T-C10 remain the deterministic old-vulnerable negative control.
8. Replace the invalid predicate-conflict assumption in T-C9.
9. Add tests proving:
     revoke-first  → no intent
     authorize-first → intent validly created before revoke
10. Run:
     go test -count=1 -p 1 ./...
     go test -count=1 ./...
     go test -count=1 ./...
11. Submit the resulting implementation to another adversarial review.
```

**Do not freeze Phase 4B yet.** This is exactly the kind of adversarial discovery we wanted the review process to produce: the architecture looked correct at the transaction level, but the database's actual concurrency semantics exposed a deeper flaw. The right response is to fix the serialization model, not rationalize the test away.
