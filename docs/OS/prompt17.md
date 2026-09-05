You're right. **Do not move to Phase 4C yet.** The repository's canonical verification command is:

```bash
go test ./...
```

and it currently **fails**. Until that command is green and the fixes survive an adversarial review, Phase 4B is not actually finished.

The screenshot exposes two separate failures that need to be treated as real verification blockers.

### 1. The API test suite is not self-contained

The API tests are connecting to:

```text
postgresql://root@localhost:26260/fable_test?sslmode=disable
```

but at the time `go test ./...` runs, the database has no `principal` table:

```text
ERROR: relation "principal" does not exist (SQLSTATE 42P01)
```

The important clue is later in the same output:

```text
=== wave 0 === resetting behavioral test database
dsn: postgresql://root@localhost:26260/fable_test...
DROP + CREATE + apply ...
```

That reset is happening from another test package. You therefore have a **test-environment ordering/isolation problem**, not merely a missing manual setup step.

The full repository test must be reproducible from:

```bash
docker start solvent-crdb
go test ./...
```

without requiring the developer to manually pre-seed `fable_test`.

More importantly, one package must not implicitly depend on another package's `TestMain` having initialized the shared database. Go package tests are separate test processes.

### 2. The MCP panic must be fixed, even though it is pre-existing

This:

```text
panic: runtime error: slice bounds out of range [:300] with length 89
```

is unequivocally a failing test.

Calling it "pre-existing" is useful for root-cause attribution, but it does **not** make `go test ./...` green. The test fixture/assertion needs to be corrected.

And this should be fixed as a **test correctness problem**, not papered over with something like:

```go
text[:min(len(text), 300)]
```

without first determining why the test expects 300 characters and what behavior it is actually trying to verify.

The panic is at:

```text
cmd/solvent-mcp/tools_agentjacking_test.go:140
TestAJ_UserTypedOnUnpromotedBelief
```

So the adversarial review should inspect the test's intended invariant and repair the fixture/assertion accordingly.

---

# I would create Phase 6.2

The scope should be deliberately narrow:

```text
Phase 6.2
Canonical Full-Suite Verification
        ├── Make API integration tests self-initializing
        ├── Eliminate cross-package test-database dependency
        ├── Fix MCP test panic at its root
        ├── Run go test ./...
        ├── Run go build ./...
        └── Run go vet ./...
```

### Critical architectural constraint

Do **not** "fix" this by changing production architecture.

In particular:

```text
NO kernel change
NO authority redesign
NO service redesign
NO API contract redesign
NO schema change for production
NO weakening of tests
NO skipping packages
NO -run filtering as the definition of success
```

The test infrastructure should become deterministic.

I would specifically require the agent to investigate these questions before touching code:

```text
1. Which package owns initialization/reset of fable_test?
2. Does api/ have its own TestMain?
3. Which packages use fable_test?
4. Can multiple package test processes reset the same database?
5. Is shared fable_test intentional or merely inherited from the old test architecture?
6. What exact schema is required by api/ tests?
7. Why does TestAJ_UserTypedOnUnpromotedBelief assume a 300-character string?
```

### Stronger requirement for the database fix

I would **not automatically mandate "add TestMain to api"** as the solution.

The agent should first determine the repository's intended test-database architecture and choose the smallest deterministic solution.

The acceptance criterion should instead be:

```text
A clean CockroachDB instance + go test ./...
must initialize all required test state deterministically,
with no dependency on another package's execution order.
```

That is the invariant that matters.

---

# Adversarial review requirements

Before implementation, have the coding agent write the Phase 6.2 plan and submit it for adversarial review with these explicit questions:

```text
SECURITY
- Does the proposed test infrastructure change production security semantics?
- Could any workaround bypass authority checks?
- Are any failing tests being weakened rather than repaired?
- Does the actor-binding fix remain intact?
- Does the atomic AuthorizeAndCreateIntent regression remain intact?

TEST ISOLATION
- Can go test ./... run package tests in separate processes safely?
- Can one package drop/recreate fable_test while another package is using it?
- Does the proposed solution depend on package execution order?
- Does it require a manually pre-created database?
- Does it mutate production/non-test databases?

MCP REGRESSION
- Why does TestAJ_UserTypedOnUnpromotedBelief slice to 300?
- What behavior was the test intended to assert?
- Does the repair preserve that assertion?
- Is the panic masking another failure?

COMPLETENESS
- Does plain `go test ./...` pass from a clean CockroachDB?
- Are all packages actually executed?
- Are tests being skipped, filtered, cached, or conditionally disabled?
```

And require evidence:

```bash
go test -count=1 -p 1 ./...
go build ./...
go vet ./...
```

The `-count=1` matters here because we already have evidence of cached packages in the screenshot.

---

## The bar for declaring Phase 4B complete

I would make the final gate:

```text
go build ./...              PASS
go vet ./...                PASS
go test -count=1 ./...      PASS
```

with **zero panics, zero skipped security tests, zero package failures**.

Then separately inspect and record:

```text
TestIntegration_AuthorizeAction_Atomicity
TestIntegration_AuthorizeAction_ActorIDMismatch
TestIntegration_ConcurrentRevokeTarget
TestAJ_UserTypedOnUnpromotedBelief
```

That gives us a much stronger claim than the earlier "all met" summary.

The current Plan 6.1 correctly defines the intended identity binding and verification criteria.  

**Verdict: Phase 4B remains BLOCKED until `go test ./...` is genuinely green.** The next artifact should be a narrowly scoped **Phase 6.2 adversarially reviewed verification/remediation plan**, not Phase 4C.
