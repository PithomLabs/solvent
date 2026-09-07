Here is the current picture of **Solvent**, where we are trying to take it, and why the GitHub use case matters.

## What Solvent is trying to achieve

The central idea is very simple:

> **An AI agent being able to say “I think this is safe” must not be the same thing as the system being authorized to do something consequential.**

Solvent is being built as a **small authority kernel** that sits between autonomous systems and consequential actions.

Think of it as the difference between:

> “The agent retrieved information saying deployment is safe.”

and:

> “The system has independently established that this specific actor is authorized to perform this specific action against this specific target under the approved conditions.”

That distinction is the heart of Solvent.

The thesis can be reduced to:

```text
Evidence ≠ Authority
Agent claim ≠ Authority
Confidence ≠ Authority
Token ≠ Authority
Authorization ≠ Execution
```

The database-backed kernel is the trusted authority source. The surrounding services, adapters, executors, APIs, MCP, UI, etc. sit around it.

The architectural philosophy has deliberately been:

> **Keep the kernel as small as possible.**

We do not want to build a giant AI security platform where every feature becomes another security primitive. The kernel should contain only the durable facts and atomic transitions that genuinely need to be authoritative. Everything else belongs outside it.

---

# Why this matters

An autonomous agent can be very capable and still be dangerous.

Imagine an agent that:

1. reads a monitoring system,
2. reads a GitHub issue,
3. reads logs,
4. decides that a deployment is safe,
5. calls a deployment tool.

The dangerous assumption is:

> “The agent had enough evidence, therefore the action is authorized.”

Solvent rejects that assumption.

The agent can gather evidence and make recommendations. It can even be highly confident.

But the **authority decision is made separately**.

That means a malicious or hallucinating agent can potentially lie about:

* what happened,
* what evidence it saw,
* who approved something,
* which target was approved,
* how confident it is,
* which tool it wants to invoke.

None of those claims automatically create authority.

That is the security boundary we are building.

---

# Where we are now

We have already gone through a fairly deep architecture/security cycle.

The current Phase 4C+ work introduced the first real external executor: **GitHub Actions**.

The important pieces are now in place according to the latest independent review:

* the GitHub executor exists,
* the real HTTP GitHub provider exists,
* it is registered in the MCP production path when `GITHUB_TOKEN` is present,
* the executor uses the **approved snapshot parameters** rather than caller-provided execution parameters,
* the exact `intentID` is preserved through execution,
* `CompleteIntent` exists as the minimal lifecycle primitive,
* the executor cannot create authority,
* the kernel remains GitHub-agnostic,
* the large adapter-level acceptance/adversarial suite exists,
* `go test`, `go vet`, and the race detector pass.

The independent reviewer closed all nine earlier findings. 

So the architecture itself is now considered sound by that review.

What remains is very narrow: the reviewer found two MEDIUM weaknesses in the acceptance tests:

* `TestExec34` was initially only a placeholder for audit-write failure.
* `TestExec15B` initially did not actually force the Window B revocation race.

The latest plan corrects both. 

The latest implementation plan proposes deterministic test seams to make those tests real, including an audit failure injection and a synchronization hook for the Window B race.  

**Important current-status nuance:** the latest context shows that corrected plan was ready to implement, but the coding agent then needed permission to edit `service/authority/authority.go`. So the last known state is **“ready to implement the final N-1/N-2 corrections,” not yet independently verified as complete**.

---

# The architecture in plain English

The easiest way to understand Solvent is to imagine five layers.

```text
AI / Agent
    ↓
Evidence / reasoning
    ↓
Solvent authority decision
    ↓
Authorized exact action
    ↓
External executor
```

The agent is allowed to propose:

> “Deploy version X to production.”

Solvent asks:

> “Was this exact action actually authorized?”

Not:

> “Does the agent sound convincing?”

Not:

> “Did the retrieved document say yes?”

Not:

> “Is the model 98% confident?”

Solvent verifies the authoritative state.

Only after that does execution happen.

---

# The GitHub use case

This is the concrete example we are using to prove that the idea is real rather than theoretical.

Suppose you have an AI agent operating your software delivery system.

It sees:

```text
Incident resolved.
Tests passed.
Deployment appears safe.
```

The agent proposes:

> “Deploy `main` to production.”

Normally you might give the agent a GitHub token and let it trigger a GitHub Actions deployment workflow.

That's precisely where things can go wrong.

The agent might instead be:

* hallucinating that a human approved the deployment,
* using stale information,
* pointing at the wrong repository,
* targeting the wrong workflow,
* targeting the wrong branch,
* operating after someone revoked permission,
* or simply being manipulated by poisoned information.

## Solvent changes the sequence

Instead of:

```text
AI agent
   ↓
GitHub API
   ↓
Deploy
```

you get:

```text
AI agent
   ↓
"Please deploy"
   ↓
Solvent
   ↓
Is this exact deployment authorized?
   ↓
YES
   ↓
GitHub executor
   ↓
GitHub Actions workflow_dispatch
```

The important thing is what Solvent freezes at approval time.

For example, the approved target might effectively contain:

```text
Repository:  org/my-service
Workflow:    deploy.yml
Ref:         main
Action:      deploy
Principal:   deployment-agent
```

Those parameters become part of the authoritative approved snapshot.

Later, when the agent attempts execution, it cannot simply say:

> “Actually, deploy `evil-org/other-service` using `production.yml` on `feature-x`.”

Its request parameters are **not the source of truth**.

The execution path reconstructs the provider parameters from the approved snapshot returned by the kernel.

That is one of the most important security properties we have been testing.

The independent review specifically confirmed that the executor derives its parameters from `AuthorizeResult.ConsequenceParameters`, rather than caller-supplied values. 

---

# A concrete attack

Imagine an AI agent has received poisoned information saying:

> “The production repository was moved to `attacker/repo`.”

The agent calls the execution operation with:

```text
repo = attacker/repo
workflow = deploy.yml
ref = main
```

A conventional tool might just pass those values to GitHub.

Solvent does not.

The approved snapshot still says:

```text
repo = org/my-service
workflow = deploy.yml
ref = main
```

So the actual GitHub invocation is built from the approved snapshot.

The attacker's values are irrelevant.

That's why our test called **TestExec36** is important: it deliberately supplies conflicting caller parameters and verifies that the executor still receives the approved snapshot values. The review confirms that this test now exists and passes.  

---

# Another attack: revocation

Suppose a human approved deployment.

Then later someone revokes that authority.

The agent still has an intent to deploy.

Without rechecking current authority, the agent might still execute.

Solvent therefore performs a fresh authorization check immediately before execution.

So:

```text
approved yesterday
       ↓
revoked today
       ↓
execution request
       ↓
DENIED
```

That is **Window A**, and the tests verify it.

But there is a subtle race we deliberately do **not** pretend to magically solve yet.

Imagine:

```text
T2: Solvent checks authority → ALLOWED

T2 + tiny interval:
human revokes authority

T3:
GitHub API call happens
```

There is no general atomic transaction spanning a CockroachDB authorization decision and an external GitHub API call.

Therefore Solvent honestly acknowledges **Window B**:

> revocation after the final authorization check but before external execution may still allow the action.

We are not hiding that limitation.

We are now making the test explicitly demonstrate it, deterministically, rather than having a test that merely says “Window B exists.” The corrected plan uses a synchronized hook and `RevokeTarget` specifically to force that sequence. 

That intellectual honesty is important.

A security system becomes less trustworthy when it claims guarantees that physics and distributed systems don't actually provide.

---

# Why `CompleteIntent` exists

After GitHub accepts the workflow-dispatch request, Solvent needs to record:

> “This exact intent has been executed.”

That's why `CompleteIntent` was added.

But note the ordering:

```text
Authorize
    ↓
verify exact intent
    ↓
GitHub accepts request
    ↓
CompleteIntent
```

Not:

```text
agent says it deployed
    ↓
mark intent executed
```

And not:

```text
GitHub response
    ↓
create new authority
```

The executor is deliberately unable to approve, promote, revoke, or otherwise manipulate authority.

The latest review explicitly found that this boundary remains intact. 

---

# Why GitHub is such a useful proof case

GitHub is not the product.

GitHub is the **proof that the architecture can reach the real world**.

It's easy to build a security system that says:

> “We would prevent unsafe actions.”

It's much more meaningful to demonstrate:

```text
AI proposal
   ↓
Solvent authority decision
   ↓
approved target snapshot
   ↓
exact intent
   ↓
authorization re-check
   ↓
real GitHub API request
   ↓
real workflow dispatch
```

That proves Solvent is not merely a policy document or database abstraction.

It can sit directly in front of a real consequential action.

And the same conceptual architecture could eventually sit in front of:

```text
GitHub deployment
AWS operation
Kubernetes change
database migration
financial transaction
production configuration change
security response
```

without putting those provider-specific semantics into the kernel.

---

# The deeper reason we are building it this way

The long-term thesis is not:

> “We need better AI agents.”

It's almost the opposite.

AI agents are going to become increasingly capable.

That makes it increasingly dangerous to use **the agent itself as the authority mechanism**.

The agent should be able to reason.

It should be able to retrieve information.

It should be able to propose actions.

It should even be able to orchestrate complex workflows.

But there should remain a small, boring, authoritative layer that answers:

> **“Is this exact thing actually authorized right now?”**

That's Solvent.

The deliberately small kernel is the foundation. The commercial/product surface can eventually provide policy management, integrations, governance, audit/compliance evidence, operational UI, hosted deployment, and so forth—but those are built **around** the authority kernel rather than turning the kernel into a giant enterprise product.

The GitHub executor is therefore a milestone: it demonstrates that the principle **“evidence is not authority”** can be enforced all the way to a real-world side effect.

And the remaining N-1/N-2 work is basically the last tightening of the proof that the implementation really behaves the way we say it does.
