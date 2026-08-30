Implement the Technical Lead approved Wave 2 rework.

Do NOT begin Wave 3.

The architecture remains frozen.

Only implement the accepted findings below.

----------------------------------------------------------------------
Must Fix
----------------------------------------------------------------------

1. Maintainer trust boundary

Use the normalized payload field:

    is_maintainer

when deriving maintainer confirmation.

Never emit a belief stating "confirmed by maintainer" unless
is_maintainer == true.

If false:

- either emit a weaker derived belief, or
- return no belief,

whichever best matches DATA_PIPELINE_SPEC.md.

----------------------------------------------------------------------

2. Pull Request trust boundary

Use normalized payload:

    merged

Do not derive "fix available" from an unmerged PR.

Add explicit unit tests covering:

- merged == true
- merged == false

----------------------------------------------------------------------

3. Remove unreachable release logic

Remove derivation paths that depend on metadata the pipeline never produces.

Do not invent metadata.

Do not modify Wave 1.

Align release behavior with the approved specification.

Add regression tests proving behavior using actual Wave 1 normalized output.

----------------------------------------------------------------------

4. Malformed payload handling

Do not silently reinterpret malformed payload.

Handle impossible payloads explicitly.

Keep the implementation deterministic.

----------------------------------------------------------------------

Do NOT Change

- Wave 1
- normalization
- public API
- package layout
- architecture
- Wave 3
- kernel

----------------------------------------------------------------------

Verification

Run:

go build ./...

go vet ./...

go test ./...

----------------------------------------------------------------------

Deliverables

Update:

WAVE2_IMPLEMENTATION.md

Create:

WAVE2_REWORK.md

Include:

- Files modified
- Findings resolved
- Tests added
- Receipts
- Remaining known limitations

Stop after the rework.

Wait for Technical Verification.

Do not begin Wave 3.