**Gate 1 has passed.** Kilo returned:

> **GO — TEST ENGINEERING GATE PASSED**

and explicitly says OpenCode may proceed to Step 1 without inventing missing security semantics. 

The review also confirms all nine previous findings are resolved and found **no new material findings**.  

So the handoff is now:

```text
Kilo Gate 1
    ✅ GO
      ↓
OpenCode Step 1
    Fake Provider + Executor Tests
      ↓
OpenCode continues through implementation
      ↓
Full verification
      ↓
STOP
      ↓
Kilo Gate 2
    Independent implementation review
```

The important constraint now is: **OpenCode is authorized to implement the specification, but should implement against the approved test target rather than reinterpret it.** The test target is now the acceptance contract. Kilo explicitly says the document is implementation-ready. 

### Give OpenCode this prompt

```text
Phase 4C+ — Gate 1 PASSED — Begin Implementation

Kilo Code has completed the independent Test Engineering Gate review.

VERDICT:

    GO — TEST ENGINEERING GATE PASSED

Kilo confirmed that:

    docs/OS/phase4c_plus_test_target.md

is now a sufficiently precise, internally consistent, deterministic,
adversarial, implementation-ready acceptance contract.

You are now authorized to proceed to Step 1 implementation.

AUTHORITATIVE SPECIFICATION

Implement strictly against:

    docs/OS/phase4c_plus_test_target.md

Do NOT reinterpret or weaken the security semantics defined there.

PROCESS

Step 1:
    Fake Provider + Executor Tests

Then proceed through the remaining implementation steps in the Phase 4C+
implementation plan:

    Step 1 — Fake Provider + Executor Tests
    Step 2 — Executor Selection Security Fix
    Step 3 — Kernel CompleteIntent
    Step 4 — Real GitHub Provider
    Step 5 — Wiring + Integration
    Step 6 — Full Verification

Do not skip the test-first sequence.

STEP 1 REQUIREMENTS

Implement the deterministic fake provider and tests exactly as specified.

Create the required test infrastructure:

    adapter/github/fake_provider.go
    adapter/github/provider.go
    adapter/github/executor.go
    adapter/github/executor_test.go

The tests must implement the approved TestExec01-38 and TestAT01-16
specification where applicable.

CRITICAL SECURITY PROPERTIES

Do not weaken any of these:

1. Authorization must precede external execution.

2. Caller-controlled executor selection MUST NOT determine the executor.
   Derive executor from the authorized action using the fixed mapping.

3. Approved snapshot `consequence_parameters` MUST be the sole source of
   provider-specific execution parameters.

4. Caller-supplied execution parameters must not become a second source of
   truth.

5. Exact repo/workflow/ref binding must be preserved.

6. Revocation before the final authorization re-read MUST prevent execution.

7. Window B remains a documented v1 TOCTOU limitation.
   Do not "fix" it by inventing new architecture.

8. `executed` means provider acceptance, not workflow completion.

9. CompleteIntent occurs only after confirmed provider acceptance.

10. Provider errors/ambiguous outcomes must never become false success.

11. Provider success must not grant or mutate authority.

12. Authorization, invocation, and provider result must remain distinct audit
    facts.

13. Normal test runs MUST NEVER trigger a real GitHub side effect.

TEST ENGINEERING RULES

- No sleep-based race tests.
- Use deterministic synchronization.
- Inspect fake-provider call count and exact received parameters.
- Maintain the distinction between kernel-level binding tests and
  execution-path snapshot-binding tests.
- Window B is a characterization test, not a security gate.
- Do not silently convert known limitations into guarantees.

COMPLETEINTENT

The approved design requires:

    CompleteIntent

to be present in the kernel Contract interface.

It must remain inaccessible through:
- REST handlers
- MCP tools
- caller-controlled input

The only production call path is:

    ExecuteAction
      → current authorization
      → approved snapshot reconstruction
      → executor
      → provider acceptance
      → CompleteIntent

Do not introduce another call path.

SCOPE

Do NOT add:
- new services
- schema migrations
- generic executor framework
- multiple providers
- Kubernetes/AWS executors
- async polling
- retry orchestration
- RBAC
- multi-tenancy
- general remote execution API
- additional kernel primitives beyond the already-approved CompleteIntent

GITHUB

The real provider remains narrowly scoped to:
- GitHub Actions workflow dispatch
- one safely scoped test repository/workflow
- `net/http`
- `GITHUB_TOKEN`
- no go-github dependency

Real GitHub integration must remain explicitly opt-in.

VERIFICATION

After implementation run:

    task lint:openapi
    task test:openapi
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -count=1 ./...

Also run:

    go test -count=1 -v ./adapter/github/...
    go test -count=1 -v ./service/authority/...

Real GitHub integration must never execute during normal `go test ./...`.

ADVERSARIAL CHECK

Before declaring implementation complete, verify that the implementation would fail the
approved adversarial tests if any of these were removed:

- authorization check
- exact target/action binding
- fixed executor mapping
- approved-snapshot parameter reconstruction
- CompleteIntent acceptance gating
- provider-error handling

DELIVERY

When finished:

1. Run all required verification.
2. Produce an implementation report documenting:
   - files changed
   - tests added
   - security properties demonstrated
   - known v1 limitations
   - verification results
   - real GitHub integration result, if explicitly run
3. Do NOT make further architectural changes merely to improve the design.
4. STOP after full verification.

The CURRENT repository will then be handed to Kilo Code for Gate 2 independent
adversarial implementation review.

Do not self-declare final architectural approval; Kilo owns the final
implementation review gate.
```

At this point, **yes, OpenCode should move forward**. Kilo's explicit conclusion is that the specification is now strong enough for implementation without OpenCode inventing missing security semantics. 
