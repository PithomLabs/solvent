In plain English, all of this is about solving a very specific problem:

> **AI agents are becoming capable of doing real things, but we need a reliable way to stop “the AI thinks this is a good idea” from automatically becoming “the AI is allowed to do it.”**

Solvent is trying to be that missing safety layer.

## Start with a simple example

Imagine an AI agent that manages a company's systems.

You ask:

> “Can you fix this production problem?”

The agent reads logs, searches documentation, talks to other AI agents, investigates what happened, and concludes:

> “The problem is DNS. I should change this DNS record.”

The agent may be completely wrong.

Or worse, an attacker may have planted something in the logs that tricked the agent into reaching that conclusion. That's essentially the important lesson from the GhostJacking research: the attacker doesn't necessarily need to break the firewall or steal an administrator password. The attacker can put malicious information into something the AI is legitimately allowed to read, and then let the AI's own legitimate privileges do the damage. 

So the dangerous chain is:

> **Someone puts bad information into the system → AI reads it → AI believes it → AI acts on it.**

Solvent is trying to break that chain.

---

# The fundamental idea

Solvent says there should be a difference between:

> **“The AI believes X.”**

and:

> **“The AI is authorized to act because of X.”**

Those are not the same thing.

So Solvent conceptually creates stages:

```text
Evidence → Belief → Review obligations → Promotion → Action
```

Suppose the AI says:

> “This patch is safe.”

Solvent doesn't immediately say:

> “Okay, deploy it.”

Instead, the belief carries unresolved questions such as:

> Has the version been verified?

> Has the failure been reproduced?

> Has the rollback path been checked?

> Has an operator reviewed it?

Those questions are what Solvent calls **debt**.

The belief cannot become “promoted” while required debt remains.

And only a **promoted** belief can authorize an action. 

That is the first major idea.

---

# Why the database is important

You might ask:

> “Why can't we just tell the AI not to act until the checks are done?”

Because that's still just an instruction to the AI.

The model might misunderstand it.

The model might be tricked.

A prompt might change.

A tool wrapper might have a bug.

Solvent puts the critical rule in the database itself.

Its database has three important rules:

### Rule 1: You cannot promote a belief while it still has debt.

In plain English:

> **You don't get to say “approved” while required questions are still unanswered.**

### Rule 2: An action can only refer to a belief that is promoted right now.

This is the `gate` rule.

In plain English:

> **You can't say “this action is authorized because of that belief” unless the database itself sees that belief as currently promoted.**

### Rule 3: If the belief is later retracted, the dependent action cannot remain live.

This is the really clever part.

Suppose:

```text
Monday:
“Patch is safe.”
→ promoted
→ deployment authorized
```

Then Tuesday:

```text
New evidence:
“Actually, the patch doesn't fix the vulnerability.”
```

Solvent can retract the belief, and the database's relationship to the live action means the old authorization cannot simply remain active. CockroachDB's `ON UPDATE CASCADE` causes the dependent row to be reconsidered, and the `live_requires_promoted` constraint prevents the invalid state. 

So the architecture becomes:

```text
AI thinks something
        ↓
system gives it a status
        ↓
status determines authority
        ↓
authority can disappear when the underlying belief changes
```

That's much stronger than simply keeping an audit log.

---

# What the new ADR is doing

The document you just had the coding agent create is basically saying:

> “Okay, we've proved the core idea. Now let's think very carefully about what a production version would require.”

It deliberately does **not** change your current hackathon implementation.

It says:

> “The current system is frozen. Here is what we would do next.”

That's why it is called an **ADR — Architecture Decision Record**.

It's basically a formal record saying:

> “Here are the architectural decisions we have made, why we made them, and what is deliberately not implemented yet.”

The document says the current hackathon system remains:

```text
Evidence → Belief → Debt → Promotion → Action Intent
```

with the three database invariants doing the critical work. 

---

# So what is a “Warrant”?

This is one of the future ideas.

Right now, Solvent's authority is represented indirectly by:

> a live action intent that points to a currently promoted belief.

For the hackathon, that's enough.

But imagine Solvent eventually needs to work with:

* Claude
* GPT
* Gemini
* Cursor
* ServiceNow
* Salesforce
* an EHR
* Kubernetes
* custom enterprise software
* MCP
* A2A

Those systems don't all share Solvent's database.

So Solvent needs a portable way of saying:

> **“This particular action is authorized because this particular belief currently justifies it.”**

That future object is called a **Warrant**.

Think of it as a digital authorization receipt.

Something like:

```text
Warrant:
    Agent: claims-review
    Action: approve claim
    Resource: claim #78219
    Reason: belief #1821
    Status: authorized
    Valid until: 5:00 PM
```

The important part is:

> **Having the Warrant document doesn't itself make the action legal.**

The executor still checks with Solvent.

That's why the ADR chose **central verification**.

---

# Why not make the Warrant a signed token?

Because then Solvent starts becoming a cryptography and key-management company.

Imagine every action carries a signed token.

Now you have to deal with:

* keys;
* key rotation;
* revocation;
* expired keys;
* compromised keys;
* offline verification;
* clock differences;
* distributing keys to every system.

That's a huge amount of complexity.

So Solvent's decision is:

> **Keep the final authority decision centralized.**

In plain English:

> “Ask Solvent whether this authorization is still valid.”

The downside is obvious.

If Solvent is unavailable, a high-impact action cannot safely proceed.

The ADR explicitly accepts that:

> **Every consequential authorization depends on Solvent reachability.** 

That sounds scary, but it is actually an important sign of architectural honesty.

They're not pretending you can have a central authority system and somehow not depend on the authority system.

---

# Why some actions can still continue during an outage

Solvent doesn't want to create this absurd situation:

> “The authorization service is down, therefore the AI cannot even read a document.”

So future Solvent would classify actions by risk.

Something like:

```text
READ
DRAFT
LOW-RISK ACTION
HIGH-IMPACT ACTION
```

If Solvent is unavailable:

```text
Read a document        → probably okay
Draft a report         → probably okay
Restart a bounded job  → policy-dependent
Change DNS             → NO
Deploy production      → NO
Move money             → NO
Change privileges      → NO
```

That's what the ADR's **fail-closed** decision means.

> **For dangerous actions, no answer from Solvent means no authorization.**

The current hackathon build does not implement this risk system yet. It is a future production rule. 

---

# The biggest remaining weakness: who is allowed to say “I checked it”?

This is probably the most important thing discovered during the adversarial review.

Right now, conceptually:

```text
Human: “Yes, I reviewed this.”

AI:
  “Great, I'll call retire_debt.”

Solvent:
  debt item removed
```

That's not ideal.

Why?

Because the AI is still the messenger that says:

> “The human approved this.”

The database cannot tell whether the human really did.

The attack could be:

> AI asks a cleverly worded question → human answers casually → AI translates that answer into an authority-changing database operation.

The database's promotion gate is extremely strong, but **the thing that removes the debt before promotion is currently much weaker**. The adversarial review specifically identified this mismatch. 

So the future solution is an **attestation**.

---

# What is an attestation?

In plain English:

> **A human explicitly and verifiably says: “I personally checked this specific thing.”**

And the statement is tied to:

* exactly which belief;
* exactly which obligation;
* who made the assertion;
* when;
* through what channel;
* with an integrity mechanism.

So instead of:

```text
Human → AI → “approved”
```

you want:

```text
Human → independent attestation → Solvent
```

Then Solvent verifies it before removing the debt.

That's why the ADR says:

> **The agent may request the review. The agent may not manufacture or assert the resulting attestation.** 

This is a very important distinction.

---

# Why the ADR had to clarify MCP Elicitation

You previously thought:

> “MCP Elicitation gives us the human approval mechanism.”

Yes—but only partly.

MCP Elicitation gives the user a structured way to answer a question in the client UI.

It does **not** magically create a cryptographic signature.

So:

```text
MCP Elicitation
      ≠
Cryptographic attestation
```

That's why the revised ADR now separates them:

```text
Human
→ Client UI
→ Signing / attestation mechanism
→ Solvent
→ Debt discharge
```

The ADR explicitly says that standard MCP elicitation responses are structured text, not signed assertions. 

---

# And this creates a smaller PKI problem

This was the newest adversarial finding.

We previously said:

> “Don't make Warrant verification a giant PKI problem.”

Correct.

But once we say:

> “Human approvals should be cryptographically signed,”

we now have to ask:

> Who owns the human's key?

That's still a key-management problem.

It's just a much **smaller** one.

Instead of managing keys for every downstream system that executes an agent's actions, you're managing keys for people who make authority-changing attestations.

The ADR now openly admits that. 

It doesn't pretend the problem is solved.

It says future production design has to answer:

> How does a person enroll?

> How is their key protected?

> What happens when they leave?

> What happens if the key is compromised?

> How do you rotate or revoke it?

That is why those questions are explicitly deferred. 

---

# Why all this matters to the GhostJacking problem

Now you can see the whole story.

Without Solvent:

```text
Attacker-controlled data
        →
AI reads it
        →
AI believes it
        →
AI has valid credentials
        →
AI changes production
```

With Solvent:

```text
Attacker-controlled data
        →
AI reads it
        →
AI forms a belief
        →
belief has unresolved obligations
        →
no valid warrant
        →
action is refused
```

Even if the AI is fooled, the attacker still has another problem:

> **The AI has to earn authority.**

And if someone legitimately approves the action, that approval should eventually be independently attributable rather than simply being an interpretation relayed by the AI.

That is a much stronger architecture.

---

# Why Solvent is potentially broader than security

This is where the product idea becomes much bigger.

Imagine medicine.

The AI reads a patient's record and says:

> “This treatment is appropriate.”

Solvent could eventually enforce:

```text
Evidence:
patient medications
lab results
clinical guideline

Belief:
treatment appropriate

Debt:
second opinion required
contraindication check
latest guideline check

Authority:
prescription allowed
```

Or law:

```text
Evidence:
cases
statutes
filings

Belief:
this filing is legally supportable

Debt:
jurisdiction checked
conflicting precedent checked
attorney review

Authority:
file motion
```

Or finance:

```text
Evidence:
customer data
policy
transaction history

Belief:
refund is justified

Debt:
fraud check
policy check
beneficiary verification

Authority:
issue refund
```

The underlying mechanism doesn't care what the domain is.

That's why the strategy describes Solvent as **agentic authority infrastructure** rather than a security product. The kernel itself is domain-agnostic. 

---

# The most important distinction

If you remember only one thing, remember this:

### Normal AI architecture

> **The AI decides, then the system acts.**

### Solvent architecture

> **The AI proposes, the system evaluates whether the proposition has earned authority, and only then can the action occur.**

That's the whole thing.

---

# Why the architecture is interesting

The clever part isn't that Solvent makes AI smarter.

It assumes AI will remain fallible.

Its philosophy is:

> **Don't try to eliminate every mistake. Make mistakes less able to become unchecked power.**

That's why the sequence is so important:

```text
Evidence
→ Belief
→ Debt
→ Attestation
→ Promotion
→ Warrant
→ Action
→ Reassessment
```

Some of those pieces exist today.

Some are deliberately future architecture.

The current hackathon implementation already proves the hardest foundational idea: **a database can structurally separate a belief from the authority that depends on it, and can refuse the authority when the belief is no longer valid.** 

Everything else is about making that same idea work at Internet scale, across agents, companies, tools, industries, and eventually agent-to-agent communication.

In one sentence:

> **Solvent is trying to become the system that remembers why an AI was allowed to act—and makes sure that permission disappears when the reason for it disappears.**

