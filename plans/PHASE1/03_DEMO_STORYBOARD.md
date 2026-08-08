# 03_DEMO_STORYBOARD.md

**Objective:** The actual second-by-second three-minute story.

**Audience:** Whoever records the demo video; whoever builds the hosted demo;
whoever narrates.

---

## Narrative

A belief is promoted: *a CVE just landed in CISA's KEV catalog — actively
exploited, unpatched in the fleet.* The Security Agent has already fired one
action off the back of it — "Notify on-call" — and the graph shows a clean,
promoted node with one live action hanging off it.

Then two things happen in the same instant, on both lanes at once:

1. **New evidence lands** — the vendor confirms the patch is already deployed
   everywhere it matters. The system does the right thing: it starts retracting
   the belief and cancelling the action riding on it. This is the "stand down."
2. **The Security Agent, still trusting its last read, tries to escalate** —
   "Page the Incident Commander" — because from where it's sitting, this still
   looks like a live fire. This is the "panic."

Stand-down and panic are racing for the same row, at the same moment, in two
lanes running side by side.

---

## Second-by-Second Timeline

| Time | On Screen | Narrator |
|---|---|---|
| **0:00–0:10** | Cold open. Title card: two numbers. Left: **1**. Right: **0**. Hold for 5 seconds. Then text: *"These two systems just processed exactly the same events. One finished with a ghost action. One didn't. Let me show you why."* | *(silence — let the numbers land)* |
| **0:10–0:30** | Setup. Split screen: `LANE A — Typical Schema` (left) / `LANE B — CockroachDB + The Gate` (right). Same belief node (green), same evidence node feeding it, same live action edge ("Notify on-call"). Identical. | "Two identical databases. One real safety rule. Watch what happens when an agent races the database's own memory." |
| **0:30–0:50** | The falsifying evidence lands. A card slides in: *"Vendor confirms: patch deployed."* Both belief nodes start to dim toward gray. The stand-down begins. | "The vendor just confirmed the patch. The system starts standing down — cancel the action, retract the belief." |
| **0:50–1:00** | **The race fires.** Both lanes flash simultaneously: *"Security Agent: ESCALATE → Page Incident Commander."* | "At the exact same moment, the agent — still trusting what it read a second ago — tries to escalate." |
| **1:00–1:15** | Lane A resolves first — and wins wrong. The escalation commits. Belief node turns gray (retracted), but a new green edge appears: *"PAGE FIRED."* Nothing looks broken. | "Left: it works. A page goes out. Nothing on screen looks wrong." |
| **1:15–1:35** | Lane B resolves — and refuses. A red error card appears verbatim: `ERROR: insert on table "action_intent" violates foreign key constraint "gate"` — `SQLSTATE 23503`. No new edge. Belief node turns gray, clean. | "Right: it's refused. Not by us — by the database itself. That error is real, and that constraint is really named `gate`." |
| **1:35–1:50** | Both lanes hold, side by side. The contradiction is visible without narration. | *(pause — let the visual speak)* |
| **1:50–2:10** | The reveal. Both lanes display simultaneously: Left: **AUDIT = 1**. Right: **AUDIT = 0**. The numbers land on screen together. | "So here's the only question that matters: right now, how many live pages are pointing at information the system already knows is dead? Left: one. Right: zero." |
| **2:10–2:30** | Zoom out. The graph pulls back to show the same rule holds for every node in the ledger. The scale is shown, not asserted. | "Nobody wrote code to catch this. The schema already knew." |
| **2:30–2:45** | One held frame: Lane A's ghost page vs. Lane B's clean gray node. Then tagline fades in. | *(silence)* |
| **2:45–3:00** | End card. Project name. One line: *"Memory isn't remembering. Memory is refusing to act on what is no longer true."* | *(silence)* |

---

## Expected Visualization

### Split-Screen Layout

```
┌─────────────────────────────┬─────────────────────────────┐
│  LANE A — Typical Schema    │  LANE B — CockroachDB       │
│                             │         + The Gate           │
│  ┌───────────────────┐      │  ┌───────────────────┐      │
│  │  [Evidence] ──►   │      │  │  [Evidence] ──►   │      │
│  │  [Belief] ──►     │      │  │  [Belief] ──►     │      │
│  │  [Action]         │      │  │  [Action]         │      │
│  └───────────────────┘      │  └───────────────────┘      │
│                             │                             │
│  AUDIT = 1                  │  AUDIT = 0                  │
│  ─────────────────────────  │  ─────────────────────────  │
│  receipt strip              │  receipt strip              │
└─────────────────────────────┴─────────────────────────────┘
```

### Color Coding

| Color | Meaning |
|---|---|
| Green | Promoted belief, live action |
| Gray | Retracted belief, cancelled action |
| Red | Refusal in flight (the error card) |

### Receipt Strip

Beneath each lane: the raw text the database actually produced. Never a
paraphrase. Lane A's receipt is silence (the write succeeded). Lane B's receipt
is the verbatim SQLSTATE error.

---

## What the Judges See (summary)

1. Two identical graphs, side by side, doing nothing suspicious.
2. One card: a patch got confirmed.
3. Both graphs start dimming — the system standing itself down.
4. Both lanes flash the same escalation attempt at the same instant.
5. Left lane: the escalation **fires anyway**. A green "PAGE FIRED" banner sits
   next to a gray, already-dead node. It looks fine. That's the point.
6. Right lane: the escalation **is refused**, in red, in the database's own
   words, naming a constraint called `gate`.
7. One number appears under each lane at the same time: **1** under the lane
   that looked fine. **0** under the lane that argued back.
8. The gap between those two numbers is the entire pitch.

---

## Acceptance Criteria

- [ ] Total runtime ≤ 3:00.
- [ ] Zero Go/SQL/package/test language.
- [ ] The race is understandable without knowing what CVE or KEV means.
- [ ] The reveal (AUDIT = 1 vs 0) is the climax, not the SQLSTATE error.
- [ ] The scale coda (2:10–2:30) adds no new narrated claim.
- [ ] The narrator names the mechanism once, at the exact moment the refusal
  appears.
