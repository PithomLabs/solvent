Revise the uploaded Plan 10.0 — Exact Authority Binding — Implementation Plan.

DO NOT implement code. DO NOT create a new architecture. Revise only the affected sections of the existing plan, preserving its current structure, terminology, numbering, and overall decision.

The approved direction remains:

- Kernel Growth Gate: PASS.
- Option C — Hybrid.
- Exact authority binding remains a kernel/DB invariant.
- This is the final deliberate kernel expansion before kernel freeze.
- Do not reopen the already-settled architectural decision unless the repository contains contradictory evidence.

Apply these precise revisions.

1. §11 — Schema / DB Design

The current migration adds:

    ADD COLUMN target_id   UUID REFERENCES authority_target(target_id),
    ADD COLUMN snapshot_id UUID;

Revise this so `action_intent` durably enforces the correspondence between `target_id` and `snapshot_id`.

Add the composite foreign key:

    ALTER TABLE action_intent
      ADD CONSTRAINT intent_authority_snapshot_fk
      FOREIGN KEY (target_id, snapshot_id)
      REFERENCES target_snapshot(target_id, snapshot_id);

Explain briefly why this is required:

- `target_id` alone only proves that the target exists.
- `snapshot_id` alone would be an unconstrained UUID.
- The security invariant is the pair `(target_id, snapshot_id)`.
- The pair must therefore be database-enforced, not merely guaranteed by the Go code path.
- This matches the existing schema pattern used by `target_activation`.
- Preserve the existing NULL/fail-closed migration behavior for pre-existing unbound intents.

Update the migration SQL in §11 to show the composite FK explicitly.

Also update the rollback SQL to drop the new constraint as well as the columns/index.

2. §11 — Uniqueness wording

Keep `intent_authority_unique` unless repository evidence shows an existing invariant makes it inappropriate.

But revise its description so it does NOT imply that it provides idempotency.

Use wording equivalent to:

    Prevent duplicate bound live intents for the same
    (scenario, belief, action, target, snapshot) authority tuple.

Explicitly state:

- This is a uniqueness constraint, not an idempotency-key mechanism.
- A retry after an ambiguous timeout may encounter an already-existing equivalent live intent.
- Full idempotency semantics are intentionally out of scope for this change.

Do not add idempotency infrastructure.

3. §12 — Error Semantics

Add a distinct duplicate-equivalent-intent case.

The semantic distinction must be explicit:

    Authorization denial ≠ duplicate-intent conflict.

Define it as:

    Duplicate equivalent live intent
        Kernel: conflict / existing intent
        Service: conflict
        REST: 409 Conflict
        MCP: conflict error
        Security decision: NOT an authorization denial

Make clear that:

- 403/denial means the caller/action is not authorized.
- 409/conflict means authorization is not being denied; an equivalent live intent already exists for the exact authority.
- Callers must not interpret this as a security refusal.
- Where safely available, the existing intent ID should be surfaced so a retrying caller can recover deterministically.
- Do not introduce full idempotency-key semantics.

Do not invent a raw database constraint error as the public API behavior.

Specify how the kernel/service detects the duplicate-equivalent-intent condition and maps it to a defined conflict error.

If the existing codebase has an established conflict sentinel/error convention, reuse it instead of inventing an unrelated abstraction.

4. §12 — Preserve the security semantics of ClaimIntent

Do NOT change the atomic ClaimIntent CAS into a non-atomic pre-check.

The claim remains authoritative.

If the CAS returns zero rows, preserve the distinction between:
- authorization denial,
- intent mismatch,
- intent not live,
- and duplicate-intent conflict where applicable.

Any diagnostic read used to classify an error must NOT become the security decision itself.

5. §16 — Files Expected to Change

Update the database migration entry to explicitly include:

- `target_id`
- `snapshot_id`
- composite FK `(target_id, snapshot_id) -> target_snapshot(target_id, snapshot_id)`
- uniqueness index

Update kernel/service/API/MCP entries only where the §12 conflict behavior requires additional error mapping.

Do not add files merely for speculative abstractions.

6. §18 — Verification Plan

Add an explicit verification case for duplicate-equivalent intent creation/retry:

    Existing live intent for exact same
    (scenario, belief, action, target, snapshot)
        →
    second creation attempt produces CONFLICT,
    not authorization denial,
    with no second live intent created.

Also verify at the API boundary:

    REST → 409 Conflict

and at MCP:

    conflict error / isError,
    not a permission-denied semantic.

Do not turn this into an idempotency test.

Add a schema-level verification that an `action_intent` row cannot contain a mismatched `(target_id, snapshot_id)` pair.

For example, verify that:
- target_id = T1 + snapshot_id = S2 where S2 belongs to T2
- or a nonexistent snapshot_id
is rejected by the database.

This is important because the point of the change is that the invariant is enforced by CockroachDB, not merely by application code.

7. §20 — Acceptance Criteria

Add explicit criteria:

    Composite FK:
    action_intent cannot store a target_id/snapshot_id pair that
    does not correspond to a real target snapshot.

    Duplicate conflict:
    Creating an equivalent live intent returns a defined conflict,
    not an authorization denial.

    REST conflict semantics:
    duplicate-equivalent intent maps to HTTP 409.

    MCP conflict semantics:
    duplicate-equivalent intent is represented as a conflict error,
    not a permission denial.

    No accidental idempotency:
    no idempotency-key/state machinery is introduced by this change.

8. §19 — Kernel Freeze

Preserve the existing kernel-freeze conclusion.

Add one sentence making clear that the duplicate-intent conflict semantics do NOT constitute a new kernel primitive or a new kernel capability; they are error semantics around the existing intent uniqueness invariant.

9. Important constraints

- Preserve the existing Plan 10.0 architecture.
- Do not reopen the Kernel Growth Gate decision.
- Do not add authentication, remote MCP, idempotency keys, promotion epochs, or other deferred work.
- Do not duplicate consequence parameters in `action_intent`.
- Do not weaken the exact `(target_id, snapshot_id)` binding.
- Do not replace DB enforcement with service-only checks.
- Do not silently rewrite unrelated parts of the plan.
- Keep the final plan implementation-ready.

Before finalizing the revision, cross-check all affected sections for internal consistency:
§6, §11, §12, §16, §18, §19, and §20 must all describe the same `(target_id, snapshot_id)` invariant and the same duplicate-conflict semantics.

Output:
1. The revised Plan 10.0 in full.
2. A concise “Changes from Plan 10.0” section listing only the substantive revisions above.
3. No code implementation.