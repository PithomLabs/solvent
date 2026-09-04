I reviewed **Plan 4.2** as the final remediation specification. It incorporates the corrections from the prior review, including the important hardening that `ExecuteAction` cannot receive a caller-supplied executor, that `kernel.Authorize` remains the final authority oracle, authentication fails closed, and intent creation is explicitly separated from execution. 

My verdict:

# **GO TO IMPLEMENTATION — WITH 2 SMALL CONTRACT TIGHTENINGS**

The architecture is now sound enough to implement. I would **not redesign it again**.

The two things I would tighten are below.

## 1. Make it impossible to reimplement `kernel.Authorize` accidentally

Plan 4.2 says `PrepareForAction` must:

```text
re-read current authority state
→ build AuthorityTuple
→ kernel.Authorize
```



And later correctly says:

> The service layer MUST NOT independently determine whether authority exists. 

Those two statements are compatible, but the implementation agent could still misunderstand "re-read current authority state" as:

```text
service SQL
→ inspect activation/revocation
→ decide active/inactive
→ kernel.Authorize
```

That would recreate the second authority engine.

I would make the rule explicit:

```text id="k4zq1n"
The service may retrieve the data required to construct the
AuthorityTuple, but MUST NOT independently evaluate whether
authority is valid.

The authoritative allow/deny determination MUST come from
kernel.Authorize.

Do not duplicate activation, revocation, snapshot, or exact-match
logic in service/authority.
```

This is especially important because the whole reason for Plan 4 is that the kernel was healthy while the service boundary was bypassed. 

---

## 2. Explicitly acknowledge the unavoidable authorization/external-execution TOCTOU boundary

There is one deeper distributed-systems issue.

The intended path is:

```text
current state
   ↓
kernel.Authorize
   ↓
executor
   ↓
external provider
```



Suppose:

```text
T1  kernel.Authorize → ALLOWED

T2  authority revoked

T3  executor calls GitHub
```

You cannot make a database authorization decision and a remote GitHub side effect one atomic transaction.

That does **not** mean the current design is wrong. It means we should not claim that `Authorize` creates an uninterruptible guarantee over an external side effect.

For this MVP, the correct claim is:

> **The executor is invoked only after a successful current authorization check immediately preceding execution.**

Not:

> "Revocation can never race with an external execution."

That latter problem is a future execution-effect/transaction-coordination concern, which the architecture intentionally defers. The plan already explicitly excludes an execution-effect verification framework from current scope. 

I would therefore add one sentence:

```text id="b5h2cf"
The system guarantees current authorization verification immediately
before executor invocation. It does not claim atomic coordination
between the Solvent database transaction and an external provider's
side effect. Such execution-effect coordination is out of scope for
this remediation and must not be simulated.
```

That actually makes the security model **more credible**, not weaker.

---

# Everything else is now aligned

### One execution path

Correct:

```text
ExecuteAction
→ current-state revalidation
→ kernel.Authorize
→ ExecutionService
→ Executor
→ external provider
```

and the executor cannot be caller-injected.  

### Intent ≠ execution

Correctly separated, with execution independently revalidated. 

### Promotion ≠ authority

Correctly preserved. This was an important correction because coupling promotion to authority would have muddied the kernel's distinct lifecycles. 

### Policy ≠ authority

The `PolicyConstraints` direction is correct and substantially safer than `AuthorizeToolCall() bool`. 

### Authentication

The fail-closed principle is correct:

```text
authenticated principal
→ actor classification
→ service
```

rather than trusting request-body claims. 

### Token

The decision not to add HMAC/JWT merely to satisfy the test is correct. The server-side opaque DB token remains continuity state, not authority. 

### Workflow

The clarification that:

```text
prepared != approved
prepared != authorized
prepared != authority
```

is exactly what is needed. 

### Adversarial tests

The 18-test matrix now includes the important mutation-before-execution cases and requires testing through the actual production path. 

### No kernel expansion

Correctly frozen unless an actual insufficiency is demonstrated through an ADR. 

---

# One thing I particularly like

The strongest part of Plan 4.2 is now the mechanical invariant:

```text
No successful current kernel.Authorize
    →
no executor invocation.
```



And the GO criteria require proving that with an end-to-end test where revocation occurs after preparation and **the executor is demonstrably not called**. 

That is much stronger than simply checking a return value.

---

# Final disposition

I would now mark the plan:

> **APPROVED FOR IMPLEMENTATION**

with the two wording additions above.

Then the workflow should be:

```text
PLAN 4.2
   ↓
IMPLEMENT REMEDIATION
   ↓
RUN ACTUAL PRODUCTION-PATH ADVERSARIAL TESTS
   ↓
GENERATE SECURITY GATE REPORT
   ↓
GO / HOLD
```

And importantly:

> **The coding agent must not proceed to Phase 4 or Phase 6 merely because implementation completed. It must first produce GO.** 

I would now **lock Plan 4.2 as the remediation contract**, with those two clarifications appended.
