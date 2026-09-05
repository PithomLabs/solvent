This adversarial review is strong, and I agree with the **GO WITH CONDITIONS** disposition. The review actually found the right residual issues rather than rubber-stamping because `go test ./...` is green. 

There is one important nuance I would change before handing the findings back to the coding agent:

### H-1 should not be called a "HIGH vulnerability"

The review correctly says the production actor-binding code is sound and that the missing test is a verification gap, not an exploitable production vulnerability. 

I would classify H-1 as:

```text
Severity: HIGH verification blocker
Classification: Phase 6.1 test gap
Exploitability: none demonstrated
```

rather than a HIGH security vulnerability. That distinction matters in the eventual security record.

### M-3 is the most important remediation

This is the one I would prioritize immediately.

The review demonstrates that the current concurrency test can pass even if the old vulnerable two-step implementation were restored, because the target is never activated. Therefore:

```text
Test passes
≠
atomicity property is tested
```

That's exactly the adversarial behavior we wanted the review to uncover. 

The corrected test should establish:

```text
approved target
       ↓
active authority
       ↓
concurrent authorize-action + revoke
       ↓
serialization conflict / retry
       ↓
no stale-authority intent
```

and then inspect `action_intent` directly.

### M-1 should be fixed

I agree with the nil checks. These are cheap, local, fail-closed hardening, and they make the authorization handlers internally consistent. 

I would not call this a blocker to the architecture, but it should be fixed before freeze because these are security-sensitive handlers.

### M-2 is useful, but don't over-engineer the test

The absence of `service/ledger` tests is real, but the proposed test should verify **delegation and observable behavior**, not attempt to inspect implementation internals.

For example:

```text
service.AuthorizeAndCreateIntent
    → allowed authority
    → live intent

service.AuthorizeAndCreateIntent
    → denied authority
    → no intent
```

Then static/code review establishes that the service calls the kernel primitive rather than duplicating SQL. The test suite should not become coupled to private implementation details.

### M-4 is a real residual security issue, but correctly not a Phase 4B blocker

The wizard still has the old:

```text
PrepareForAction
    ↓
IntentOnPromoted
```

race. The review's classification as **PRE-EXISTING** is correct. 

However, I would explicitly record it in the security backlog as:

```text
LEGACY CONCURRENCY GAP
```

rather than leaving it buried as an ordinary medium finding. It is a known stale-authority race in an HTTP-accessible legacy surface. It does not block the canonical Phase 4B API, but it should not disappear from the threat model.

Likewise, the operator-review direct `IntentOnPromoted` path is appropriately treated as a trusted administrative boundary. 

## The important architectural conclusion

The review confirms the central Phase 4B architecture:

```text
REST
  ↓
Service
  ↓
AuthorizeAndCreateIntent
  ↓
Kernel authority engine
  ↓
CockroachDB

MCP
  ↓
Service
  ↓
AuthorizeAndCreateIntent
  ↓
same kernel
```

and specifically verifies that there is no second authority engine, no workflow-token authority, no production executor bypass, and no hidden consequential path. 

That is the major milestone.

## What I would send to the coding agent now

```text
The adversarial Phase 4B review found no critical or high-severity production
security vulnerability. Do NOT redesign Phase 4B.

Apply only these four targeted remediation items:

1. Add the missing API integration regression:
   TestIntegration_AuthorizeAction_ActorIDMismatch
   - authenticated principal A
   - request actor_id B
   - expect HTTP 403 actor_id_mismatch

2. Add nil checks to:
   - handleAttachJustification
   - handleRequestAuthorization
   - handleApproveTarget
   - handleRevokeTarget
   These must fail closed with 401 missing_principal.

3. Rewrite TestIntegration_ConcurrentRevokeTarget so that:
   - target is approved/activated first
   - authority is genuinely valid before race
   - authorize-action and RevokeTarget race concurrently
   - test queries action_intent afterward
   - no live intent may exist based on revoked authority
   - test would fail if the old two-transaction implementation were restored

4. Add focused service/ledger tests for AuthorizeAndCreateIntent:
   - allowed authority → live intent
   - denied authority → no intent
   Do not duplicate kernel tests or couple tests to private implementation details.

Do NOT touch:
- kernel architecture
- authority SQL semantics
- schema
- API contract
- MCP production behavior
- wizard legacy path
- operator-review trusted admin path
- production executor scope

After changes run fresh:
    go test -count=1 -p 1 ./...
    go test -count=1 ./...
    go test -count=1 ./...
    go build ./...
    go vet ./...

Then provide an adversarial re-review of the four changes and explicitly verify
that the strengthened concurrency test would fail against the old
PrepareForAction + IntentOnPromoted implementation.

Do not declare Phase 4B frozen until that re-review returns GO.
```

This is the correct place to stop broad architectural review. The remaining work is **targeted verification hardening**, not another Phase 4B redesign. The review itself confirms that the production authority model is sound; the remaining conditions are test coverage and defensive handler robustness. 
