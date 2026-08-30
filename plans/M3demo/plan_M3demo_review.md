# Review — Work Package 05 (M3 Demo Plan)

## Verdict

✅ APPROVED WITH ONE STRATEGIC CHANGE

This is the first document in the project that I read **as a judge instead of as an architect.**

And my reaction was different.

I stopped thinking about the schema.

I started imagining the room.

That is exactly what this document needed to do.

Overall, I think this is the strongest artifact produced so far because it answers the only question that matters now:

> **"What will judges remember?"**

The answer is:

> **The database argued with the AI—and won.**

That is memorable. It is concrete. It is emotionally understandable in under 20 seconds. It is also genuinely different from almost every "AI agent" demo they'll see. The plan is tightly scoped around one race, one graph, one comparison, and one memorable reveal, exactly as requested. :contentReference[oaicite:0]{index=0} :contentReference[oaicite:1]{index=1}

---

# What I Love

## S1 — You found the correct protagonist

This was the hardest design decision.

The protagonist is **not**:

- the AI
- the graph
- the vulnerability
- the agents

It is

**the database.**

That is exactly right for this hackathon.

---

## S2 — The split screen is brilliant

Originally I expected

```
Cell 1

↓

Cell 2

↓

Cell 3
```

That is an engineer's demo.

This

```
Left

vs

Right
```

is a filmmaker's demo.

It cuts the cognitive load in half.

Excellent decision. :contentReference[oaicite:2]{index=2}

---

## S3 — "Stand-down vs Panic"

This is my favorite improvement.

Nobody has to understand

- serializable isolation
- write skew
- FK gates
- SQLSTATE

to understand

```
System

↓

Stand down

Agent

↓

PANIC
```

That is excellent framing. :contentReference[oaicite:3]{index=3}

---

## S4 — The reveal

This line

```
AUDIT = 1

AUDIT = 0
```

is significantly stronger than

```
SQLSTATE 23503
```

Engineers remember numbers.

Not SQL errors.

The SQLSTATE becomes supporting evidence,

not the climax.

Exactly right. :contentReference[oaicite:4]{index=4}

---

## S5 — Excellent implementation boundary

The section

```
Explicitly not needed
```

might be the most valuable section in the document.

Because it protects us from ourselves.

Excellent discipline. :contentReference[oaicite:5]{index=5}

---

# One Strategic Change

This is the only thing I would change.

---

## The demo is still too "security."

The mechanics are universal.

The story is not.

Right now,

judges hear

```
CVE

KEV

Vendor

Patch

Incident Commander
```

That is a lot of domain vocabulary.

The race doesn't actually require any of it.

The race is

```
Belief

↓

Action

↓

Belief dies

↓

Agent still acts

↓

Database refuses
```

That works in

- finance
- medicine
- logistics
- legal
- customer support
- security

Therefore

I would keep

the implementation exactly as-is,

but make the narration slightly more universal.

Instead of

> "The vendor confirmed the patch..."

say something closer to

> **"New evidence arrives. The system now knows the original belief is no longer true."**

Then,

after the race,

you can reveal

> "In today's demo, that evidence happened to be a KEV advisory."

That tiny change broadens the audience without changing a single line of implementation.

---

# One Additional Recommendation

This is not a nit.

It's a hackathon strategy.

---

## Start with the ending.

Current order

```
Setup

↓

Race

↓

Reveal
```

I would reverse it.

Open with

```
AUDIT

1

vs

0
```

Five seconds.

Then say

> "These two systems just processed exactly the same events. One finished with a ghost action. One didn't. Let me show you why."

That creates curiosity immediately.

The audience now wants the explanation.

People remember answers to questions they already have.

---

# One More Thing

I think we just found the tagline.

Not

```
Agents that remember.
```

Instead

> **"Memory isn't remembering. Memory is refusing to act on what is no longer true."**

That is almost the thesis of the project.

And it aligns perfectly with the race.

---

# Scope Check

✅ No architectural drift.

No additional schema.

No additional agents.

No new feed.

No implementation expansion.

Exactly what we wanted.

---

# Authorization

Approved.

I would not revise this document again.

I would now freeze it.

Implementation should begin.

---

# Decision Log

- D-063: The split-screen corruption-vs-refusal race is the canonical M3 demonstration.
- D-064: The database—not the agent—is the protagonist of the hackathon demo.
- D-065: The primary visual reveal is `AUDIT = 1` versus `AUDIT = 0`; SQLSTATE 23503 serves as supporting evidence, not the climax.
- D-066: M3 demo scope is frozen. No second race, second visualization, or secondary storyline will be added before the hackathon.
