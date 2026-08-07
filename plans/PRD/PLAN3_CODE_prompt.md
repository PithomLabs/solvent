Implement the Technical Lead approved Wave 1 rework.

The architecture remains frozen.

Do NOT begin Wave 2.

Do NOT redesign the implementation.

Implement ONLY the accepted change set from the Technical Lead review.

----------------------------------------------------------------------
Required Changes (Must Fix)
----------------------------------------------------------------------

1. Remove `time.Now()` from normalization.

- `Normalize()` must become a pure function.
- `IngestedAt` is NOT owned by `internal/normalize`.
- The caller owns ingestion time.

2. Remove fabricated `ObservedAt`.

Never substitute `time.Now()` when upstream observation time is unknown.

Instead:

- return an error, OR
- leave the value zero (whichever best matches the specification).

Do not fabricate upstream truth.

3. Enforce all specification-required fields.

Normalization must fail loudly when required fields are missing.

Examples include:

- issue_number
- pr_number
- state
- labels
- versionAffected
- other required fields defined in DATA_PIPELINE_SPEC.md

No malformed normalized objects should be produced.

4. Strengthen determinism testing.

Add:

- TestCanonicalizationDeterminism
- reordered JSON fixture(s)
- negative fixture(s)
- pinned golden SHA-256 hash

Canonical semantic input must always produce the same hash.

----------------------------------------------------------------------
Should Fix (if small and localized)
----------------------------------------------------------------------

- Replace substring severity matching with word-boundary matching.
- Make the maintainer registry immutable.
- Validate externally supplied URLs where appropriate.

These are not permitted to increase architectural scope.

----------------------------------------------------------------------
Do NOT Change
----------------------------------------------------------------------

Do NOT modify:

- content_sha256 semantics
- raw vs canonical payload behavior
- public API
- package layout
- architecture
- Wave 2 design

Those are Technical Lead design decisions and are deferred.

----------------------------------------------------------------------
Verification
----------------------------------------------------------------------

Run:

go build ./...
go vet ./...
go test ./internal/normalize/... -v
go test ./...

----------------------------------------------------------------------
Deliverables
----------------------------------------------------------------------

1. Apply the approved Wave 1 fixes.
2. Update NORMALIZE_IMPLEMENTATION.md.
3. Create NORMALIZE_REWORK.md containing:

- Files modified
- Change summary
- Which Technical Lead findings were resolved
- Test additions
- Updated receipts
- Any remaining known limitations

Stop after completing the rework.

Do NOT begin Wave 2.

Wait for Technical Lead acceptance and Wave 1 freeze.