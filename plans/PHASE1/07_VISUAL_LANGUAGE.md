# 07_VISUAL_LANGUAGE.md

**Objective:** The shared visual vocabulary so `03`'s storyboard and `05`/`06`'s
screenshots don't invent inconsistent conventions independently.

**Audience:** Whoever builds the hosted demo, whoever records the video, whoever
screenshots for the README.

---

## Color System

### Primary Palette

| Color | Hex | Meaning | Usage |
|---|---|---|---|
| **Green** | `#22C55E` | Promoted / Live | Belief nodes at `promoted`, action intents at `live`, "PAGE FIRED" banner |
| **Gray** | `#6B7280` | Retracted / Cancelled | Belief nodes at `retracted`, action intents at `cancelled`, dimmed edges |
| **Red** | `#EF4444` | Refusal in flight | The error card when the gate refuses, "ESCALATE" flash during race |

### Supporting Palette

| Color | Hex | Meaning | Usage |
|---|---|---|---|
| **White** | `#FFFFFF` | Background, text on dark | Receipt strips, terminal output |
| **Dark** | `#111827` | Background, text on light | Dashboard background, primary text |
| **Blue** | `#3B82F6` | Information, links | "See the Repo" links, evidence nodes |

### Color Rules

1. **Never use color alone to convey meaning.** Every color-coded element also
   has a text label or icon.
2. **Green means "active and safe."** A green node is one the agent may act on.
3. **Gray means "dead."** A gray node was once active, now it is not.
4. **Red means "the database argued back."** Red appears only when a constraint
   refuses a write. It is never used for warnings or UI chrome.
5. **The receipt strip is always monochrome.** Raw database output is never
   colorized — it is presented exactly as the driver produced it.

---

## Layout Conventions

### Split-Screen (the race)

```
┌──────────────────────────┬──────────────────────────┐
│  LANE A                  │  LANE B                  │
│  ──────────────────────  │  ──────────────────────  │
│                          │                          │
│  [graph area]            │  [graph area]            │
│                          │                          │
│  ──────────────────────  │  ──────────────────────  │
│  receipt strip           │  receipt strip           │
│                          │                          │
│  AUDIT = N               │  AUDIT = N               │
└──────────────────────────┴──────────────────────────┘
```

- Lanes are **equal width**. Neither lane is visually privileged.
- The **receipt strip** sits below the graph area, separated by a thin line.
- The **audit number** sits below the receipt strip, right-aligned.
- Lanes move in **lockstep** until they diverge — that synchrony makes the
  contrast readable in under a second.

### Terminal Output (Journey B)

```
$ task wow

Solvent — Transactional Belief Ledger
======================================

[1/5] Starting CockroachDB cluster...          ✓ (12s)
```

- Checkmarks (✓) for success, ✗ for failure.
- Parenthesized duration after each step.
- Monospace font only. No rich text in terminal output.

---

## Node/Edge Conventions

### Belief Nodes

| Shape | Meaning |
|---|---|
| Circle, filled | Active belief (promoted = green, entered = yellow, retracted = gray) |
| Circle, outline only | Evidence node |
| Rounded rectangle | Action intent |

### Edges

| Style | Meaning |
|---|---|
| Solid line | Active relationship (evidence supports belief, belief governs action) |
| Dashed line | Contradicting relationship |
| Arrow direction | Source → Target (evidence → belief, belief → action) |

### Status Labels

Every node carries a small text label showing its current state:

```
[ Belief: "CVE-2024-12345" ]
  status: promoted
  debt: (empty)
```

Labels are **plain English**. Internal vocabulary (`accommodated`, `postulated`,
`derived`) never appears on any visual surface.

---

## Typography

### On-Screen Text

- **Headlines:** Sans-serif, bold, 24px+ (e.g., "LANE A — Typical Schema")
- **Body:** Sans-serif, regular, 16px (narrator subtitles, descriptions)
- **Code/Errors:** Monospace, 14px (SQLSTATE errors, constraint names, receipt strips)
- **Labels:** Sans-serif, medium, 12px (node status labels, axis labels)

### The Receipt Strip

The receipt strip is the most important typographic element. It is:

- **Monospace** — always. Raw database output is never set in a proportional font.
- **Exact** — the text is copied verbatim from the driver error. No truncation,
  no paraphrase, no wrapping that breaks the string.
- **Bordered** — a thin gray border separates it from the graph area above and
  the audit number below.

Example:
```
┌──────────────────────────────────────────────────────────────┐
│ ERROR: insert on table "action_intent" violates foreign key │
│ constraint "gate" (SQLSTATE 23503)                          │
└──────────────────────────────────────────────────────────────┘
```

---

## Animation Principles

1. **Synchronized until they diverge.** Both lanes perform identical actions
   until the race resolves. The divergence point is the most important moment.
2. **Slow enough to read, fast enough to feel.** The race should take 10-15
   seconds. Faster loses comprehension; slower loses tension.
3. **The refusal card appears and holds.** It does not flash, bounce, or
   animate. It appears and stays. The rawness is the point.
4. **The audit numbers appear simultaneously.** Neither number appears first.
   They land at the same instant.

---

## Acceptance Criteria

- [ ] Every visual claim in `03` and `05`/`06` can cite a rule from this doc.
- [ ] No internal vocabulary (`accommodated`/`postulated`/`derived`) appears on
  any visual surface.
- [ ] The receipt strip is always monospace, always verbatim, never paraphrased.
- [ ] Both lanes are visually equal — neither is privileged in layout, color, or
  animation timing.
