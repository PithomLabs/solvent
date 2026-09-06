This revision is **much closer**. I would give it **GO WITH MINOR CHANGES** before implementation.

The major issues from v4 are resolved: the API is explicitly frozen, the reconciliation no longer silently changes `AttachJustification`, the canonical decision record exists, the Python client is genuinely a thin handwritten reference client, the GitHub example stays above the API boundary, and no new kernel/service/schema surface is proposed.  

There are four things I would still correct.

### 1. Fix the "Phase 4A API" wording

This line is internally inconsistent:

> “Governing constraint: OpenAPI describes the canonical Phase 4A API.”

But the immediately preceding resolution hierarchy says the **canonical v1 API is the later security-hardened Phase 4B shape where Phase 4A was superseded**. 

Change it to:

> **OpenAPI describes the canonical v1 HTTP API established by Phase 4C reconciliation. Phase 4A remains the original semantic source except where an explicitly documented security decision superseded it.**

That is much harder to misread later.

### 2. Fix the VerifyAuth field count

The plan repeatedly says:

> “7 tuple fields”

but then lists:

```text
target_id
resource_type
resource_id
scope
action_namespace
action_name
consequence_type
consequence_parameters
```

That is **8 fields**. 

This is minor, but contract documents need this kind of precision.

### 3. Don't overclaim what `api/openapi_test.go` proves

The plan calls it a:

> “Spec-in-sync test”

but the proposed checks only establish route/method/security presence and selected security-schema constraints. They do **not** prove that the full request/response schema matches the live implementation. 

That's acceptable for Phase 4C because we deliberately rejected a heavyweight contract-test platform. But rename/reframe it as something like:

> **OpenAPI contract-surface test**

and say explicitly:

> “This test protects the critical contract surface and security decisions; runtime behavior remains covered by the existing integration suite.”

Otherwise six months from now someone may believe "spec-sync" provides stronger guarantees than it actually does.

### 4. One dangerous ambiguity in the Python example

`authorize_action()` is shown without `actor_id`, which is fine because it is optional, but the example should explicitly demonstrate the **authenticated identity model**, especially since this was one of the most important security corrections in Phase 6.1.

The reference example currently demonstrates:

```text
authenticate
→ create principal
→ target
→ approve
→ verify
→ authorize
```

but should also show one sentence such as:

> `actor_id`, when supplied, must equal the authenticated principal; the API derives the effective principal from authentication.

That makes the example reinforce rather than obscure the most important API security rule. 

---

## One thing I would explicitly keep

Do **not** add the formal `EvidenceFeed` interface despite §9.3 mentioning it as "missing."

The plan correctly says later:

> “No new adapter interface needed (the existing pattern is sufficient).” 

That is the right decision. The mere existence of multiple future evidence providers does not justify an interface today.

Likewise, keeping `ActionFunc` unchanged is correct. 

---

## Architecture is now where we want it

The strongest part of the revised plan is this separation:

```text
OpenAPI
   ↓
external consumers

GitHub adapter
   ↓
Solvent API
   ↓
existing services
   ↓
kernel
```

and later:

```text
existing services
   ↓
executor adapter
   ↓
external provider
```

without modifying the kernel. 

That is exactly the extension strategy we locked in.

The first real execution phase is also correctly separated from Phase 4C: Phase 4C stops at the executor boundary; Phase 4C+ introduces the first real consequential GitHub executor. 

---

## My disposition

```text
Kernel boundary                 ✅
API freeze                       ✅
Contract reconciliation          ✅
Canonical Decision Record        ✅
OpenAPI strategy                 ✅
Python reference                 ✅
GitHub reference                 ✅
Extension architecture           ✅
Executor boundary                ✅
No new services                  ✅
No schema/kernel changes         ✅
Security documentation           ✅

"Phase 4A API" wording           ⚠
VerifyAuth field count           ⚠
"spec-in-sync" strength          ⚠
Python security example          ⚠
```

So I would tell the agent:

```text
GO WITH MINOR CHANGES.

Before implementation:

1. Replace "canonical Phase 4A API" wording with "canonical v1 HTTP API"
   established by explicit Phase 4C reconciliation.

2. Correct VerifyAuthRequest's field count from 7 to 8.

3. Rename/reframe api/openapi_test.go as a contract-surface test, and
   explicitly state that it protects route/method/security/schema decisions
   but does not replace runtime behavioral tests.

4. Add one explicit actor/authentication note to the Python reference flow
   so the Phase 6.1 identity-binding rule is demonstrated rather than merely
   documented elsewhere.

Do not change anything else.

Then implement Phase 4C and perform the planned adversarial review.
```

With those corrections, **I would approve the plan for implementation**. The revised plan now properly establishes the bridge we wanted:

> **frozen trusted kernel → stable canonical API → extension plane → agents/apps/workflows**. 
