IMPORTANT — PRESERVE THE TEST SUITE

Do NOT change, weaken, delete, skip, rename, or rewrite existing tests merely
to make `go test ./...` pass.

The existing test suite is evidence and must be treated as part of the product's
verification boundary.

You may modify tests ONLY when one of these is true:

1. The test is fundamentally broken or logically invalid.
2. The test depends on a test-harness assumption that is demonstrably incorrect
   (for example, unsafe shared database state between concurrently running
   packages).
3. The test must be updated solely because an intentional production behavior
   or interface has changed, and the existing assertion is therefore no longer
   testing the intended contract.

If a test is failing because the production implementation is wrong, FIX THE
PRODUCTION IMPLEMENTATION, not the test.

If a test-harness isolation problem exists, prefer fixing the harness/infrastructure
that causes the interference rather than changing the semantic assertions.

Do not use:
    t.Skip
    || true
    weakened assertions
    ignored errors
    reduced coverage
    serial-only execution as a substitute for safe test isolation

unless there is a separately justified repository-level reason.

The final requirement remains:

    go test ./...

must pass normally, without relying on `-p 1` as the workaround.

For every test file changed, report:
    - why the original test/harness was fundamentally broken or invalid
    - why the change does not weaken the underlying security/property assertion
    - exactly what behavioral coverage remains
    
    
    
If making `go test ./...` pass would require weakening a meaningful test rather
than fixing the underlying implementation or test-isolation defect, STOP and
report the conflict instead of changing the test.    