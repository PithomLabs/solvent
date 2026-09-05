Agreed. Claude's latest review is effectively a **GO**, with one wording cleanup around CockroachDB retry semantics. There is no new architectural finding to consolidate.

So I would **not reopen the design**. The next prompt should only instruct the agent to make that precision correction and then freeze the Phase 4B plan for implementation.

Use this:

```text id="8m8gce"
FINAL PHASE 4B PLAN CLEANUP — DO NOT REDESIGN

The Phase 4B plan has completed independent adversarial review.

The reviewers found:

- The MCP authorize-action race is correctly fixed in this phase.
- REST and MCP converge on the same Service → Kernel path.
- Authority semantics remain implemented only in the kernel.
- The atomic `AuthorizeAndCreateIntent` kernel primitive is correctly justified
  through the Kernel Growth Gate / ADR-0002.
- The concurrency model is correctly based on one CockroachDB SERIALIZABLE
  transaction.
- The Service Layer is now a genuine architectural boundary.
- Actor/effective-principal binding is explicitly defined.
- No production executor, workflow tokens, or schema expansion are introduced.

DO NOT redesign the plan.

Make ONLY the following wording correction.

==================================================
RETRY SEMANTICS PRECISION
==================================================

In §9.3, the current text says:

"Intent INSERT may fail on duplicate if a previous retry partially committed
(but the composite FK prevents this — the intent either commits or rolls back
with the authority check)."

This is misleading.

The described `crdb.ExecuteTx` retry mechanism uses
`SAVEPOINT cockroach_restart` and rolls the transaction back to that savepoint
before retrying. Therefore, a prior attempt's writes do not survive into the
retry as partially committed writes.

Replace that statement with a precise formulation equivalent to:

"On serialization failure, the transaction is rolled back to the retry
savepoint before the closure is re-executed. Writes from the failed attempt do
not survive into the retry. The closure therefore re-evaluates current
authority and re-attempts intent creation from the fresh transaction state."

Retain the separate existing idempotency/uniqueness analysis where it is
actually relevant.

==================================================
FINAL CHECK
==================================================

After making the wording change:

1. Verify that no other section was changed.
2. Verify that the ADR decision remains unchanged.
3. Verify that MCP convergence remains in Phase 4B.
4. Verify that the kernel remains the sole authority implementation.
5. Verify that the concurrent authorize-action + revoke-target test remains.
6. Verify that no rollback path permits knowingly shipping the race.

Do NOT:

- redesign the API
- redesign the kernel primitive
- change the Service Layer
- change the Phase 4A contract
- add features
- add migrations
- add production execution
- add workflow tokens
- reopen the ADR options

Then mark the document:

READY FOR IMPLEMENTATION

This is the final documentation cleanup before implementation.
```

### Then move to implementation

At that point I would consider the chain closed:

```text id="n4lq0b"
Phase 4A Contract
      ✅ independent adversarial review
            ↓
Phase 4B Plan
      ✅ independent adversarial review
            ↓
minor wording cleanup
            ↓
IMPLEMENT
```

The important thing is **do not ask Claude for another broad plan review after this wording edit**. The review has already established that the architecture holds. The remaining work is implementation, followed by adversarial review of the **actual code and resulting transaction behavior**, which is a much more valuable gate than repeatedly reviewing the plan.
