I would **REQUEST CHANGES before implementation**. The plan is directionally right, but it still contains a few contradictions that should be resolved now rather than discovered halfway through Phase 4C.

The overall architecture is good: frozen kernel, no new service boundary, hand-written OpenAPI, thin Python example, bounded GitHub reference, executor contract only, and lightweight contract validation. 

The problems are mainly around **what exactly is frozen and what Phase 4C is permitted to change**.

## 1. Biggest issue: the plan says the API is frozen, then proposes changing it

At the top, Phase 4C says:

> “The API remains frozen.” 

and later:

> “No API contract changes.” 

But M-1 proposes:

```text
belief_id:
query parameter → request body
```

That is an actual HTTP contract/implementation change, not merely documentation. 

This needs to be resolved explicitly.

There are only two coherent choices:

**A. Keep the Phase 4B API frozen.**
Then OpenAPI must describe the current query-parameter behavior, even if it is not the ergonomic design you ultimately prefer.

**B. Declare a small API-contract correction before Phase 4C.**
Then this is no longer purely Phase 4C documentation work; you have a tiny Phase 4B/4C API correction that needs its own implementation/test/review.

Given everything we've locked so far, I prefer **A unless there is a compelling reason to change it now**.

Do not quietly turn Phase 4C into an API revision.

---

## 2. M-6 is a good security decision, but the plan should be clearer about versioning

The plan correctly chooses the narrower `authorize-action` surface:

```text
scenario_id
belief_id
action
target_id
action_source
actor_id
```

and constructs the `AuthorityTuple` server-side. 

I agree with the security decision.

But this is important enough that the plan should explicitly say:

> **This is the canonical v1 API shape. The broader Phase 4A tuple-input shape is superseded for this endpoint by the later security decision.**

Otherwise the repository will contain two contradictory "canonical" contracts.

---

## 3. The OpenAPI plan should not invent a production server

This:

```yaml
servers:
  - url: http://localhost:8080
  - url: https://{deployment}.solvent.dev
```

is potentially premature. 

Nothing in the presented plan establishes that `solvent.dev` is an actual production deployment namespace.

For a canonical OpenAPI contract, I'd use:

```yaml
servers:
  - url: http://localhost:8080
    description: Local development
```

and omit a production server until one actually exists.

Don't let an example hostname become an accidental product commitment.

---

## 4. The Python client definition conflicts slightly with the "minimal reference client" philosophy

The plan says:

> “Methods mapping 1:1 to API endpoints” 

That's broader than necessary.

You don't need to build a miniature Python SDK just to prove language neutrality.

I'd constrain it to the **small reference flow actually demonstrated**:

```text
create belief
create target
attach justification
request authorization
approve
verify
authorize action
```

A few explicit methods are enough.

The rule should be:

> **Reference client, not SDK.**

That is consistent with the plan's own non-goal of not creating an SDK. 

Also, the phrase:

> “Generated from OpenAPI where practical”

should be removed unless generation is actually being used. The plan explicitly chose **hand-written OpenAPI and no OpenAPI code generation**. 

Make the Python client handwritten too.

---

## 5. The GitHub example is slightly too ambitious in one place

The proposed reference flow is good:

```text
GitHub event
→ evidence
→ belief
→ target
→ justification
→ request
→ approval
→ authorization
→ executor boundary
```



But I'd be careful about having `examples/github/integration.go` directly orchestrate *everything* itself.

That could accidentally become a second integration architecture.

The example should demonstrate:

```text
GitHub adapter
    ↓
canonical Solvent API
```

rather than becoming a bespoke workflow engine.

The plan already says GitHub logic stays in `adapter/github`, which is correct. 

Keep the reference integration deliberately dumb.

---

## 6. "Five authorization layers" needs provenance or simpler wording

The plan says `security.md` will explain:

> “Five authorization layers (authentication, API access control, Solvent authority, policy, execution authorization)” 

That phrasing risks creating a conceptual problem.

We have consistently separated:

```text
authentication
API boundary
policy
kernel authority
execution
```

But **policy is not another authorization layer in the same sense as kernel authority**. We've repeatedly emphasized:

> Policy ≠ Authority.

The plan itself states that correctly elsewhere. 

I'd call the document something like:

> **Security boundaries and decision flow**

and explicitly distinguish:

```text
Authentication → identifies caller
API access control → controls API access
Policy → advisory/constraint evaluation
Kernel authority → authoritative "may this happen?"
Execution authorization → governs whether an already-authorized operation is handed to an executor
```

That will avoid introducing a misleading "five layers of authorization" mental model.

---

## 7. The spec-sync test needs a slightly stronger definition

The proposed test checks:

* routes
* methods
* security
* response schemas
* errors
* forbidden endpoints. 

Good.

But it should also check the **material contract decisions from reconciliation**, especially:

```text
actor_id optional
actor_id conflict → 403
principal_id not caller-controlled
authorize-action tuple dimensions server-derived
```

Otherwise the most security-sensitive parts of the reconciliation won't actually be protected against spec drift.

Still keep it lightweight; don't turn it into a full runtime contract-testing framework.

---

# One architectural issue I would change in the plan itself

The line:

> “The Phase 4A contract remains the canonical semantic source of truth for non-security mismatches; the implementation's security hardening takes precedence...” 

is good, but Phase 4C needs one more explicit category:

```text
security hardening that intentionally superseded Phase 4A
        ↓
new canonical API contract
```

Otherwise the OpenAPI author is still forced to mentally reconcile three things every time:

```text
Phase 4A
Phase 4B
security fix
```

I'd have the reconciliation table produce a **canonical v1 API decision table**, which then becomes the sole input to OpenAPI.

That gives:

```text
Phase 4A
   ↓
reconciliation
   ↓
Canonical API Decision Record
   ↓
OpenAPI
   ↓
examples/tests
```

This is cleaner than making the OpenAPI author repeatedly reinterpret the old contract.

---

# What I would approve

After correcting those points, the plan's core is strong.

The extension architecture is exactly where it should be:

```text
OpenAPI
   ↓
external clients

Protocol adapter
   ↓
existing service layer
   ↓
kernel

Integration adapter
   ↓
existing service/API
   ↓
kernel

Executor
   ↓
external provider
```

No new service, no new kernel primitive, no schema growth, and no premature executor are all correct. 

And the kernel-growth gate is particularly clean: every proposed Phase 4C artifact stays above the kernel. 

## My disposition

```text
Architecture                  ✅
Extension strategy            ✅
OpenAPI approach              ✅
Python reference              ✅
GitHub reference              ✅
Executor scope               ✅
No new services              ✅
No kernel changes             ✅

API-freeze contradiction      ❌
Production server hostname    ⚠
Python "mini-SDK" tendency    ⚠
Security-layer terminology    ⚠
Spec-sync security coverage   ⚠
```

So I would send the agent:

```text
REQUEST CHANGES — then implementation can proceed.

1. Resolve the API-freeze contradiction around AttachJustification.
   Either preserve the exact Phase 4B API or explicitly declare and review
   a small API correction. Do not silently change the API during Phase 4C.

2. Remove the speculative https://{deployment}.solvent.dev server unless
   that deployment namespace already exists in the repository.

3. Keep the Python artifact a handwritten reference client, not an SDK and
   not a generated client. Implement only the demonstrated flow.

4. Make the canonical v1 API decisions from reconciliation explicit in a
   single decision record before writing OpenAPI.

5. Replace "five authorization layers" with clearer security-boundary /
   decision-flow terminology while preserving Policy ≠ Authority.

6. Extend the spec-in-sync test to protect the material security decisions:
   authenticated principal binding, actor_id mismatch behavior, and
   server-derived authorize-action tuple dimensions.

7. Keep everything else unchanged:
   no kernel changes, no schema, no new service, no production executor,
   no generalized plugin platform.

After these corrections, implement Phase 4C and submit the complete result
to adversarial review before declaring it complete.
```

The important thing is that **Phase 4C should freeze the public contract cleanly, not smuggle a contract redesign into the OpenAPI exercise**. Once that boundary is clean, this plan is a very good bridge from the verified kernel to the extension ecosystem.
