This is the **independent review we wanted**. It is materially different from the implementation agent's earlier "GO," and it found issues that the implementation agent did not surface. I would **not close Phase 4C yet**.

The reviewer explicitly says it independently inspected the repository and returned **READY WITH SPECIFIC FIXES**, with one actual acceptance-gate failure. 

## The clear blocker: Spectral

F-1 is unequivocal.

`task lint:openapi` actually fails because `.spectral.yaml` references rules that do not exist in the installed Spectral version. 

That means:

```text
Phase 4C contract validation
    ↓
BROKEN
```

and the plan explicitly made successful Spectral validation an acceptance criterion. So this must be fixed before freeze.

This is fortunately a tiny, non-architectural fix.

## F-2 deserves more thought than "just document it"

The reviewer found `discharged_by` is caller-supplied and can identify another principal. 

The reviewer classifies it as pre-existing, which is reasonable from a Phase 4C attribution perspective. But I would **not blindly accept the proposed treatment** yet.

We need to distinguish:

```text
discharged_by = mere audit attribution
```

from:

```text
discharged_by = security-relevant actor identity
```

If it is only an attribution field for a debt-discharge record, then documenting it as a v1 limitation may be adequate.

If it affects authorization semantics or who is considered responsible for discharge, then this is much more serious.

The implementation agent should therefore trace `Discharge` end-to-end before deciding that documentation is sufficient.

## F-3 is the finding I would investigate most carefully

The reviewer found a genuine divergence:

```text
REST:
authenticated principal → effective actor
                    ↓
actor_id mismatch → 403

MCP:
caller supplies actor_id
                    ↓
used directly in AuthorityTuple
```



The reviewer rates it LOW because both ultimately reach the same kernel authority check. I would **not automatically accept that classification**.

The important question is:

> Is MCP really a trusted administrative surface, or is MCP one of the mechanisms by which AI agents consume Solvent?

Your long-term product goal is explicitly authorization **for AI agents**. That makes an MCP-supplied principal identity much more consequential than it would be in a purely trusted operator CLI.

An agent potentially selecting:

```text
actor_id = somebody else
```

and asking Solvent to authorize against that identity deserves a deliberate security decision.

The fact that the authority engine still rejects unauthorized tuples is not necessarily sufficient, because the caller may have access to another principal's valid authority tuple.

At minimum, I would require the Phase 4C follow-up to explicitly decide:

```text
MCP actor_id
    =
trusted caller identity?
delegation?
attribution?
arbitrary authority principal?
```

Do not merely hide this in `extensions.md`.

This is especially important because the previous Phase 6.1 work specifically established authenticated-principal binding on REST. The current MCP divergence needs to be an explicit architectural choice, not an accidental leftover.

## F-4 is a useful cleanup

The dead `VerifyAuthRequest.PrincipalID` field is low severity. The server is secure because the handler ignores it, but the Go type contradicts the canonical OpenAPI contract. 

I would probably remove the field rather than annotate it as ignored—**but only if that doesn't conflict with compatibility requirements**.

Since the API is frozen, removing an unused Go field is not necessarily an HTTP API change, but it should still be treated as a public Go-type compatibility decision.

## F-5/F-6/F-7 are straightforward

F-5 is a real plan-compliance miss: the forbidden surface was supposed to be documented in the OpenAPI info description. 

F-6 should definitely be cleaned up because having 20+ avoidable Spectral warnings makes the lint signal noisy. 

F-7 is trivial and not a blocker. 

## The good news

The independent reviewer confirms the important architectural properties:

```text
kernel unchanged
no new services
no migrations
no production executor
all 26 routes match
no forbidden endpoints
REST actor binding intact
action_source guard intact
server-side tuple construction intact
spec-surface tests pass
Python live flow passes
GitHub example compiles
full Go build/vet/tests pass
```



So this is **not another Phase 4B-type architectural crisis**.

The current state is:

```text
Phase 4C implementation       ✅
Phase 4C functionality        ✅
Independent review             ✅
Phase 4C acceptance            ❌
```

because the contract-validation gate is broken and there are unresolved semantic/documentation issues.

## What I would send to the implementation agent

```text
The independent Phase 4C adversarial review is accepted as the authoritative
review for this round.

Do NOT close Phase 4C yet.

Remediate only the following:

1. F-1 — REQUIRED
   Fix docs/openapi/.spectral.yaml so:
       task lint:openapi
   exits successfully with the installed Spectral version.

2. F-5 — REQUIRED
   Add the complete forbidden endpoint pattern list to the OpenAPI info
   description exactly as required by the Phase 4C plan.

3. F-3 — REQUIRED DECISION, not merely documentation
   Re-examine the MCP actor_id divergence.
   Determine whether MCP is genuinely a trusted local administrative
   surface in the canonical product architecture or whether it must obey
   the Phase 6.1 authenticated-principal binding.
   Trace the actual threat model and authority implications.
   Do not automatically accept "trusted surface" as sufficient.
   Record the decision explicitly and have the independent reviewer
   re-evaluate it.

4. F-2 — REQUIRED ANALYSIS
   Trace discharged_by through the full discharge path.
   Determine whether it is pure attribution or security-relevant identity.
   If pure attribution, document the v1 limitation explicitly.
   If security-relevant, escalate rather than merely documenting it.

5. F-6 — RECOMMENDED
   Remove the 20+ avoidable Spectral warnings by making tag casing
   consistent.

6. F-4 — RECOMMENDED
   Decide whether VerifyAuthRequest.PrincipalID should be removed from the
   Go type or explicitly documented as ignored, taking Go API compatibility
   into account.

7. F-7 — OPTIONAL
   Correct the Python example/default-port documentation mismatch.

Do NOT:
- change the kernel
- change the authority model without explicit security justification
- add services
- add schema migrations
- add an executor
- broaden Phase 4C
- weaken or delete tests

Then run fresh:

    task lint:openapi
    task test:openapi
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -count=1 ./...
    python3 examples/python/basic_authorization.py

Finally, HAND THE CURRENT TREE BACK TO THE SAME SEPARATE ADVERSARIAL
REVIEWER.

The reviewer must independently re-check F-1 through F-7 and return a fresh
GO / REQUEST CHANGES disposition.

Do not treat the implementation agent's statement "fixed" as closure.
```

### One especially important process point

**Do not let the implementation agent spawn another reviewer and call that independence.**

The reviewer should be separately launched, exactly as this review apparently was, and should receive the **current repository**, not a curated "these are the fixes" summary.

This review has validated the separation model itself: the independent agent found F-1 through F-7 despite the implementation agent previously declaring the work complete. That is strong evidence that our adversarial workflow is working.

So the proper state now is:

> **Phase 4C remains open, with a small remediation round followed by another independently performed adversarial review.**

Only after that fresh reviewer returns **GO** should we freeze 4C and move into **Phase 4C+ — the first real GitHub executor**.
