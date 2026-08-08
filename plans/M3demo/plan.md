# Plan — Phase 1: Product Definition (plan6 roadmap)

## Context

`plans/plan6/brief.md` and `plan_phases.md` restructure the remaining project from
milestone-thinking (M0…M7) to phase-thinking: Phase 0 (Foundation — M0–M2, complete),
**Phase 1 (Product Definition — current)**, Phase 2 (Demo Engineering), Phase 3 (Judge
Experience), Phase 4 (Submission). The stated reason: the hackathon rubric weighs product
experience as heavily as engineering, and none of that experience has been designed yet — only
proven underneath.

`plan6.md`/`plan6_review.md` (the sibling files that produced `IMPLEMENTATION_CONTRACT.md` in
the first place) establish *why* the roles matter: "Claude Code" was deliberately scoped to
"Senior Go Engineer... receives precise work orders... never architectural decisions" to
prevent context drift across tool handoffs. `plan_phases.md` assigns Phase 1 to a different
role, "Product Architect." Per your decision, I'm taking that role for this deliverable rather
than routing it externally — the alternative reintroduces exactly the context-loss risk
`plan6_review.md` worried about, since I already hold the full M0–M3demo history this session
produced, including a finished demo narrative for the M3demo prompt that becomes this phase's
`03_DEMO_STORYBOARD.md` almost directly.

**Continuity note:** the M3demo request (`plans/M3demo/prompt.md`) was interrupted before I
wrote its output. That work isn't abandoned — its content is exactly what `03_DEMO_STORYBOARD.md`
needs, so I'll deliver both: the standalone `plans/M3demo/M3_DEMO_PLAN.md` as originally asked,
and a reformatted version folded into Phase 1's template. One story, two homes, no
re-derivation.

**Location, per your decision:** `go/plans/PHASE1/`, co-located with `plans/M0`, `M1`, `M2`,
`M3demo` — the established pattern for this repo's implementation-adjacent planning artifacts.

**Scope discipline carried over from every prior phase in this project:** this plan covers
Phase 1 only. It does not plan Phase 2 (Demo Engineering), does not touch architecture or the
kernel, and per `plan_phases.md`'s own principles: no Go, no SQL, no packages, no new
milestones, no architecture redesign.

---

## Global decisions (locked once, referenced by all 8 documents)

These exist so the 8 documents can't quietly disagree with each other. Everything below is
either already established in this session/the source docs, or a synthesis I'm confident
enough to make directly rather than escalate (each is annotated with why).

- **Working name: "Solvent."** Already load-bearing — it's the actual Go module path
  (`github.com/PithomLabs/solvent`) established during M1, and `brief.md` itself titles the
  roadmap "SOLVENT Roadmap." **The *public* name is explicitly still open** —
  `plan6.md`'s closing line: *"the single open decision remains the public name; everything
  else is settled."* I will not invent one; every doc uses "Solvent" as the working name and
  flags the public name as a standing open item reserved for the freeze review / product owner.
- **Protagonist framing, verbatim from `brief.md`:** CockroachDB is the hero. The AI is
  intentionally, visibly wrong for a moment. The database — not prompt engineering — is what
  refuses. Thesis line, kept exact: *"Memory is not remembering. Memory is refusing to act on
  what is no longer true."*
- **The one demo scene.** Reused, not re-derived: the stand-down-vs-escalation split-screen
  race from the M3demo work — Lane A (typical schema) lets a stale escalation fire; Lane B
  (the real `gate` constraint) refuses it with the actual verified receipt
  (`SQLSTATE 23503`, constraint `gate`); the reveal is one audit number, **1 vs 0**. This is
  the single source of truth `02`, `06`, and `08` point back to rather than re-imagine.
- **Evidence feed: CISA KEV, default and frozen** (`brief.md` §Evidence Feed Strategy), behind
  a domain-neutral `EvidenceFeed` interface so the kernel never names a domain (`idea.md`). The
  exact storyline CVE is **deliberately not chosen yet** — established project practice
  (`fable_plan.md`: *"pick the storyline CVE at record time"*) — so it stays a placeholder
  everywhere, not a gap.
- **Taskfile command surface — reconciling two sources that don't quite match.** `brief.md`
  lists `task setup / demo / race / verify / clean / doctor`; `plan_phases.md`'s Judge Journey
  uses only `task setup → task wow`. Resolution: **`task wow` is a guided one-shot orchestrator**
  that runs setup → demo → race → verify back to back for the zero-friction path; the granular
  verbs remain as the fuller surface for anyone who wants to drive a single stage themselves.
  No contradiction, one command surface, two depths.
- **Two judge-facing surfaces, not one** — synthesized directly from the hackathon's own
  submission rules (a *repo* URL **and** a *functional demo app* URL **and** a video, all
  required) plus `brief.md`'s "Product 1 (engineering) / Product 2 (experience)" split:
  - **Journey A — hosted web demo.** Zero clone, one click, watches the race, satisfies the
    mandatory "functional demo app" URL.
  - **Journey B — clone and run.** Taskfile-driven, for the engineer who wants to go deeper;
    satisfies the "open source repository" requirement.
  Both journeys tell the *same* race; they differ in depth and friction, not in story.
- **Visual language primitives** (already exercised in the M3demo storyboard): green =
  promoted/live, gray = retracted/cancelled, red = a refusal in flight; a "receipt strip" under
  every panel showing the database's own raw output, never a paraphrase of it; split-screen as
  the default comparison layout.
- **Voice guardrail, extended project-wide for Phase 1:** no internal vocabulary
  (`accommodated`/`postulated`/`derived`) in anything judge-facing — this already governs M7's
  narrated output; Phase 1 applies it to the README, video, and hosted demo too.
- **Non-negotiables every document must respect** (`brief.md` §Non-Negotiable Principles,
  restated once here instead of copy-pasted into all 8): architecture is frozen; kernel stays
  domain-agnostic; KEV is the default feed; the database is the protagonist; every new item
  must earn its place against the judging rubric or it goes to the post-hackathon backlog.

---

## The 8 documents

Same filenames and order `plan_phases.md` specifies. For each: Objective, Audience, the
content decisions that make drafting the real thing close to mechanical, Acceptance Criteria,
and Open Questions — kept to genuine architect-level forks only, not things I can resolve
myself.

### 01_PRODUCT_VISION.md
**Objective:** state what Solvent *is*, in one paragraph a judge could repeat back, before any
UX detail. **Audience:** judges skimming before they click anything; every other doc's author
(future me) checking they haven't drifted.
**Content:** the thesis line; the domain-neutral core loop (`idea.md`'s
Belief → Evidence → Promotion → Intent → Retraction) stated without cybersecurity vocabulary
first, then KEV named as *this* demo's chosen instance of it; the one-sentence answer to "why
not Postgres" already frozen in `plan6.md` (*"...that's the default on CockroachDB and an
operational project you assemble and pray over on Postgres"*) — carried forward verbatim since
it already survived an adversarial review; explicit note that the public name is open.
**Acceptance criteria:** a reader with zero context can restate the thesis in one sentence
after reading this doc alone; contains zero implementation nouns (no table names, no function
names, no SQL).
**Open questions:** none — this doc only restates decisions already made elsewhere.

### 02_JUDGE_JOURNEY.md
**Objective:** map every path a judge might actually take through the submission, end to end.
**Audience:** whoever builds Phase 2/3 against this — it's the contract those phases implement.
**Content:** both journeys (hosted web demo / clone-and-run) mapped as literal step sequences
with an elapsed-time budget per step, e.g. Journey A: land on demo URL (0:00) → click "run the
race" (0:05) → watch it resolve (0:45) → see the 1-vs-0 reveal (0:55) → optional "see the repo"
link out; Journey B: `git clone` → `task wow` → the same race, narrated in the terminal →
"I completely understand why this matters" as the exit state `brief.md` names directly. Each
step names its failure mode (what if the clone is slow, what if the hosted demo is cold) so
Phase 3's "doctor" tooling has a target list, without specifying how doctor works.
**Acceptance criteria:** every step has an owner journey (A or B), an elapsed-time budget, and
a named failure mode; total Journey A time fits inside the 3-minute video ceiling with room to
spare for a live Q&A follow-up.
**Open questions:** whether Journey A (hosted demo) needs to survive being *live* during
judging (uptime/hosting risk) or can be a recorded interactive replay — this is a real
production/cost tradeoff for the product owner, not something I should default silently.

### 03_DEMO_STORYBOARD.md
**Objective:** the actual second-by-second three-minute story.
**Content:** the M3demo race, reformatted into this template — carried over wholesale, not
rewritten: cold open → setup (identical graphs) → falsifying evidence lands → the race fires →
Lane A commits wrong / Lane B refuses with the real receipt → the 1-vs-0 reveal → zoom-out →
closer. Full narrator script included, matching the M3demo draft. The one addition beyond the
original M3demo scope: a explicit **closing ~15–20 seconds** addressing the gap I flagged when
I answered that document's closing question — a fast, wordless cut over the full-sized ledger,
so scale is *shown*, not asserted, addressing the "agentic memory at real scale" judging
criterion the race alone doesn't reach.
**Acceptance criteria:** identical to M3demo's own five requirements (one live race; race
visibly changes agent behavior; CockroachDB visibly prevents corruption; the graph visibly
changes; a judge understands why without narration alone) plus: total runtime ≤ 3:00; zero
Go/SQL/package/test language; the scale coda is ≤ 20 seconds and adds no new narrated claim.
**Open questions:** none carried from M3demo remain open at the design level — the one
placeholder (the exact CVE) is deliberately deferred, not unresolved.

### 04_TASKFILE_UX.md
**Objective:** specify every command's *experience* — what it prints, how long it takes, what
"it worked" looks like — without specifying how it's implemented.
**Content:** `task wow` as the guided orchestrator (see Global Decisions); `task setup / demo /
race / verify / clean / doctor` as the granular surface; for each, the expected terminal output
shape (a few illustrative lines, not exhaustive), expected duration, and what a *failure* looks
like in plain language (e.g. "no cluster reachable" prints a one-line fix, not a stack trace) —
this is a UX spec for Phase 2 to implement against, not code.
**Acceptance criteria:** every command has a stated maximum acceptable duration; every command
has a stated failure message in plain English; `task wow` alone reproduces Journey B's entire
"whoa" moment with no other command required first.
**Open questions:** none — command *names* are frozen by Global Decisions; behavior detail
belongs to Phase 2.

### 05_REPOSITORY_UX.md
**Objective:** what a judge sees the instant the repo opens, before reading a word of prose.
**Content:** landing-page-of-the-repo framing (what's above the fold in the README, addressed
fully in `06`); directory layout at a glance (what top-level folders exist and why, at the
product level: `plans/` = the paper trail, `docs/` = the receipts, `cmd/`+`internal/` = the
engine — described by role, not by package contents); "proof navigation" — a single, findable
path from the README to the actual M0–M2 receipts, since `brief.md` explicitly wants the
repository itself to carry credibility, not just the video.
**Acceptance criteria:** a judge can find the invariant receipts (M0–M2) within two clicks from
the README; the directory-layout description contains no package or file-level implementation
detail beyond folder *purpose*.
**Open questions:** none.

### 06_README_BLUEPRINT.md
**Objective:** the actual README's structure and above-the-fold content, synthesizing 01, 03,
04, 05.
**Content:** hero section (one line + the thesis + a link straight to Journey A's hosted demo);
"the 60-second version" (the 1-vs-0 reveal as a static image/GIF, for the judge who never runs
anything); quickstart (`task wow`); "how it works" (domain-neutral loop, KEV as the instance);
"the receipts" (link into `docs/M0…M2`); license/positioning line reusing the frozen
"Transactional Decision Memory" pitch phrase from `plan6.md` (adopted for README/pitch, banned
from voiceover per that same ruling — a distinction worth preserving here).
**Acceptance criteria:** hero section is skimmable in under 15 seconds; every claim in the
README traces to either a receipt in `docs/` or a section in this Phase 1 set; zero internal
vocabulary.
**Open questions:** none.

### 07_VISUAL_LANGUAGE.md
**Objective:** the shared visual vocabulary so `03`'s storyboard and `05`/`06`'s screenshots
don't invent inconsistent conventions independently.
**Content:** formalizes the Global Decisions' color coding and receipt-strip convention into a
short, referenceable style sheet — color meaning table, split-screen layout convention, node/edge
shape conventions, typography tone (plain English labels, raw DB text never paraphrased).
**Acceptance criteria:** every visual claim in `03` and `05`/`06` can cite a rule from this doc
rather than defining its own.
**Open questions:** none — this is pure formalization of decisions already made above.

### 08_SUBMISSION_STRATEGY.md
**Objective:** package everything for the actual Devpost submission fields.
**Content:** maps each hackathon submission requirement (`hackathon.md`: repo URL, demo app
URL, ≤3-minute video, write-up) to the exact artifact from this phase that satisfies it;
explicitly flags the two still-open decisions that must resolve before submission — the public
name (`plan6.md`) and the storyline CVE (deliberately deferred) — as a short, visible checklist
rather than letting them hide inside prose; notes the judging-criteria coverage table from
`plan_phases.md` (Foundation/Product/Demo/UX/Submission × the five rubric dimensions) as the
sanity check that no phase over-invests in one dimension.
**Acceptance criteria:** every mandatory submission field has a named source artifact; both
open decisions are listed explicitly, not buried.
**Open questions:** the two above, carried forward for the freeze review — not resolved here,
since naming and storyline timing are product-owner calls.

---

## Freeze review (exit gate)

Per `plan_phases.md`'s own late addition: *"nothing from a completed phase can be changed
unless it's factually incorrect, blocks implementation, or materially improves the hackathon
score."* I will treat Phase 1 as **not done** until this gate is explicitly passed — matching
every milestone in this project so far, none of which proceeded to the next stage without a
review. **I will stop after producing the 8 documents and not begin Phase 2.**

## Verification

Once the 8 files exist under `go/plans/PHASE1/`:

- Confirm each file contains exactly the five required sections (Objective, Audience,
  Deliverables, Acceptance Criteria, Open Questions).
- Grep the set for Go/SQL/package/test vocabulary — expect zero hits outside this plan's own
  meta-commentary.
- Cross-check consistency: the same Taskfile verbs, the same color coding, the same "Solvent"
  working name, and the same race/reveal appear identically in every document that touches
  them — no document should re-describe the demo scene differently from `03`.
- Confirm both standing open items (public name, storyline CVE) are visible in
  `08_SUBMISSION_STRATEGY.md` and nowhere silently resolved.
- Confirm `plans/M3demo/M3_DEMO_PLAN.md` is also written, closing out the interrupted M3demo
  request.

Stop at Phase 1's close, pending your freeze review. Do not begin Phase 2.
