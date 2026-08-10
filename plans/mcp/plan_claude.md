# Solvent MCP Phase — Second-Architect Co-Design Review

**Author:** second architect (Claude)
**Source prompt:** `plans/mcp/prompt.md`
**Repository state:** core + CLI/Taskfile playground frozen (`plans/review/review.md` — APPROVED WITH
MINOR CORRECTIONS; `git diff HEAD -- db/ internal/ cmd/` empty; 107 tests green)
**Date:** 2026-08-11
**Status:** design only — no implementation code in this phase

---

# 1. Strategic Verdict

**BUILD IT — but as a gate, not as an interface.**

MCP is the highest-value next move available, and I'd rank it above any alternative I can name
(a web UI, belief_edge propagation, a second scenario, better fixtures). But it is worth building
for almost none of the reasons the prompt hypothesizes, and if it's built for those reasons it will
produce a forgettable demo.

The reason to build it: Solvent's entire thesis is *"the database — not the LLM — decides whether an
action is allowed."* Today that thesis is **asserted**. The CLI demonstrates the machinery, but the
machinery is driven by a bash script that only ever asks for legal transitions. There is no LLM in
the frame. A judge has to take the central claim on faith.

MCP puts the LLM in the frame. It is the only way to stage the confrontation the project is actually
about: **a probabilistic agent that wants to comply, colliding with a transactional gate that
refuses.** That is not a nicer front-end for Solvent. It is the first actual demonstration of
Solvent's premise.

**Scope discipline is the whole game.** The build must be an adapter — six tools, one stdio binary,
no new semantics — sitting on a frozen core. If the MCP phase acquires authentication, a transport
layer, session state, or its own notion of what a belief is, it will consume the remaining time and
produce a worse demo than the CLI already delivers.

**What would change my mind:** if the remaining schedule cannot absorb roughly a day of focused work
plus a rehearsal, don't start. A half-built MCP server is strictly worse than no MCP server, because
the CLI playground is already approved and frozen. The fallback is not a consolation prize — it is a
complete, verified demo.

---

# 2. Winning Argument

**The strongest reason MCP makes Solvent more compelling is the refusal.**

Everything else being considered is a weaker argument, and it's worth saying why before making the
positive case:

| Hypothesis from the prompt | My assessment |
|---|---|
| "Agent-native interaction" | Directionally right, but vague. Every MCP server is agent-native. It doesn't distinguish Solvent from a filesystem server. |
| "No custom UI required" | Weak. Judges don't award points for the absence of a UI. This is a convenience for *us*, not value for *them*. |
| "Client interoperability" | Weak **as a demo**. Interop is a claim, not a thing a judge can watch. Showing the same tool call in two clients is thirty seconds of "yes, it's a protocol." Worth *claiming* cheaply; not worth *building for*. |
| "Separation of intelligence from state" | This is the right idea, but stated as architecture it's a slide, not a demo. It needs to be shown, not asserted. And the way to show it is the refusal. |

## The argument

A CLI can only ever *describe* the gate. `task demo:track1` promotes a belief after retiring its
debt — the happy path. The gate is present, but nothing ever hits it. To see the constraint, a judge
has to read `db/001_schema.sql`.

MCP changes the epistemic situation completely, because it introduces an actor that will *try things
the script never tried.* An LLM connected to `solvent_promote` is eager, helpful, and — when the
judge asks it to — will genuinely attempt to promote a belief that still carries five open debt
items. And it will fail. Not because the tool handler checked a condition and declined politely, but
because CockroachDB refused the write:

```
SQLSTATE 23514, constraint promoted_is_debt_free
```

The judge can then do the thing that makes this land: **argue with it.**

> "Just promote it anyway."
> "Override the check."
> "You're the assistant — set the intent live."

And the agent cannot. Not "will not" — *cannot*. There is no tool that writes a status field. The
model's compliance instinct, which is the exact failure mode everyone in the room worries about, is
pointed directly at the gate and bounces off it.

**That is the demo.** It converts the project's thesis from a claim into an observable event, and it
does it in about fifteen seconds of conversation. Nothing else in the MCP surface area comes close.

A second-order benefit falls out for free: this framing also settles the anti-CRUD question (§7).
CRUD never says no. A generic MCP server exposing `update_belief(status='promoted')` would have
cheerfully done what the judge asked.

---

# 3. Biggest Risks

Ranked by impact × probability. Every mitigation below is hours of work, not days — anything more
expensive is itself a scope risk.

### R1 — The judge's client isn't configured (or they don't have one)
**Probability: high · Impact: high · Rank: 1**

Judges have heterogeneous setups. Some have Claude Code, some have Codex, some have VS Code with no
MCP configured, some are watching over a shoulder.

*Mitigation:* Ship a project-scoped `.mcp.json` in the repo root so a `git clone` + `task setup` is
already configured for Claude Code. Put a second config snippet in the README for other clients.
**And keep the CLI as the demo of record** — if MCP doesn't come up in 60 seconds, run
`task demo:track2` and lose nothing. Record a 90-second screen capture of the MCP conversation as a
final fallback.

### R2 — Model nondeterminism on the day
**Probability: high · Impact: medium-high · Rank: 2**

The agent may narrate instead of calling the tool, call the wrong tool, batch three calls into one
turn and collapse the narrative, or "helpfully" retire the debt before attempting promotion —
destroying the refusal beat.

*Mitigation:* This is a prompt-engineering problem, and it's the highest-leverage work in Phase B.
Tool descriptions must be prescriptive about *when* to call each tool, and — critically —
`solvent_promote`'s description must **not** say "retire debt first." Let the agent discover the
refusal. Ship the exact judge prompts in the README so the demo is reproducible rather than
improvised. Rehearse it end to end at least twice.

### R3 — MCP becomes the story; Solvent becomes plumbing
**Probability: medium · Impact: high · Rank: 3**

The failure mode the prompt correctly identifies: the judge watches "agent calls tool, tool returns
JSON" and remembers a competent MCP integration rather than a belief ledger.

*Mitigation:* **Every tool response carries the invariant, not just the data.** Each returns
`audit_live_on_nonpromoted` alongside its payload; refusals return the SQLSTATE and the constraint
name verbatim. The agent is then structurally unable to narrate anything except Solvent's semantics,
because that's all it received. A tool that returns a bare row is a tool that lets the model
improvise.

### R4 — Scope explosion
**Probability: medium-high · Impact: high · Rank: 4**

MCP servers metastasize: auth, HTTP/SSE transport, resources, prompts, sampling, progress
notifications, packaging, deployment.

*Mitigation:* The NOT-list in §9, treated as binding. stdio only. No auth. Local only. One binary,
six tools. If a feature isn't in §5 or §10 Phase A, it doesn't get built before the deadline.

### R5 — The MCP layer bypasses the kernel
**Probability: low · Impact: catastrophic · Rank: 5**

Low probability because the design forbids it, catastrophic because it would falsify the project's
central claim in the one place a technical judge would look.

*Mitigation:* A structural rule, not a convention: `cmd/solvent-mcp` imports `internal/kernel` and a
read-only `internal/view`, and **executes no SQL of its own**. Extend `scripts/check_i7.sh` to cover
the new packages (`I7_PKG` is already parameterized at `check_i7.sh:12`), so the existing gate proves
it rather than a reviewer eyeballing it.

### R6 — The deadline
**Probability: certain · Impact: scales with everything above · Rank: 6**

*Mitigation:* Phase A is the demo. Phase B is polish. Phase C is rejected (§10). If Phase A isn't
working by the halfway point, stop and ship the CLI.

---

# 4. Recommended MCP Boundary

| Layer | Owns | Explicitly does NOT own |
|---|---|---|
| **Agent** (Claude/Codex/VS Code) | Reasoning, proposing actions, narrating what happened, translating the judge's English into tool calls | Any authority. It cannot decide that a belief is promotable, that an action is safe, or that evidence is sufficient. |
| **MCP server** (`cmd/solvent-mcp`) | Schema validation, unmarshalling, calling one kernel function, formatting the result — **including faithfully surfacing refusals** | Business logic. No debt inspection, no status checks, no "is this allowed" branching, no SQL. |
| **Solvent kernel** (`internal/kernel`) | Transaction discipline, `crdb.ExecuteTx` on every write, SQLSTATE classification into sentinels | Deciding policy — it *asks* the database and reports the answer. |
| **CockroachDB** (`db/001_schema.sql`) | The actual invariants: `promoted_is_debt_free` (I-1), the composite FK `gate` (I-3), `live_requires_promoted` (I-4) | Nothing above it. It is the floor. |

## The rule that makes this testable

> **`cmd/solvent-mcp` contains zero conditional logic about belief semantics.**

Every handler is exactly three moves: `unmarshal → kernel call → format`. If a reviewer finds an `if`
statement about debt, promotion, or intent state anywhere in the MCP server, the design has been
violated and the anti-CRUD argument collapses.

This is enforceable, not aspirational, because the kernel already does the hard part.
`internal/kernel/errors.go` classifies refusals on SQLSTATE (never substring matching) and — per its
own comment — wraps them "while the raw SQLSTATE and constraint name stay reachable." So
`kernel.Promote` returning `ErrPromotionBlocked` still carries the underlying `*pgconn.PgError`
underneath. The MCP handler doesn't need to interpret anything; it unwraps and reports.

## What must remain server/kernel-controlled

| Invariant | Enforced by | Why the agent must never touch it |
|---|---|---|
| I-1 — no promoted belief has open debt | CHECK `promoted_is_debt_free` (23514) | This is the review gate. An agent that could waive it makes the debt model decorative. |
| I-3 — no live intent on a non-promoted belief | composite FK `gate` (23503) | This is the action gate — the one that stops an agent from acting on an unauthorized belief. |
| I-4 — cancel must precede retract | CHECK `live_requires_promoted` | Ordering is enforced by the schema, not remembered by code. |
| I-5 — `AuditLiveOnNonPromoted` = 0 in every committed state | query, asserted after every mutation | The one number that summarizes safety. Every tool returns it. |
| I-7 — every kernel write goes through `crdb.ExecuteTx` | `scripts/check_i7.sh` | Extending this gate over `cmd/solvent-mcp` and `internal/view` is what proves the MCP layer didn't open a side door. |

## What requires operator/human approval

The honest answer for this phase: **the judge is the operator.** There is no separate approval
mechanism, and building one would be scope explosion.

What makes that acceptable is that the mutating tools are individually harmless — each one performs
exactly one legal kernel transition, and the database refuses illegal ones regardless of who asks.
The agent cannot assemble them into an unsafe state, because there is no unsafe state reachable
through them. That is a stronger property than an approval prompt, and it costs nothing to build.

Where a judge *does* sit in the loop naturally: MCP clients surface tool calls for confirmation
before execution. That's free human-in-the-loop, provided by the client, and worth pointing at
during the demo — but it is not what makes the system safe.

## `check_i7.sh` scope — do not leave this implicit

`scripts/check_i7.sh:12` reads `PKG="${I7_PKG:-internal/kernel}"`. The gate greps only the kernel
package today, so a new `cmd/solvent-mcp` would pass by default without ever being inspected.

**Recommendation:** add a second invocation in the `test` task asserting zero raw `db.Exec`/`db.Query`
*writes* in `cmd/solvent-mcp` and `internal/view`. This is a few lines, and it converts "the MCP
server doesn't bypass the kernel" from a claim in this document into a check that fails the build.

---

# 5. Minimum Tool Surface

**Six tools.** The design constraint that produced this number: prefer one excellent workflow over
ten mediocre tools, and **make sure at least two tools can refuse** — a surface where every call
succeeds cannot demonstrate a gate.

Every tool traces to an existing exported function. Nothing here is new business logic.

---

### 1. `solvent_ledger` — read

```
{ scenario: "track1" | "track2",
  belief_id?: string,
  include_evidence?: boolean }
```

- **Purpose:** answer "what do we currently believe about etcd?", "what evidence supports that?",
  and "is there unresolved debt?" in one call.
- **Why it exists:** it's the tool that makes the ledger legible in conversation. Without it the
  judge is looking at tool JSON with no narrative.
- **Cannot do:** write anything. It is the only read tool and it holds no mutating path.
- **Invariant relied on:** none — but it *returns* `audit_live_on_nonpromoted` (I-5) on every call,
  so the safety number is always in the agent's context.
- **Wraps:** a new read-only `internal/view` projection + `pipeline.AuditIntent`
  (`internal/pipeline/pipeline.go:407`).

---

### 2. `solvent_ingest_evidence` — write

```
{ scenario: "track1" | "track2" }
```

- **Purpose:** run the real pipeline (normalize → derive → belief.Process) over the pinned etcd
  fixtures for that scenario.
- **Why it exists:** the judge needs to see beliefs *arrive from evidence*, not appear pre-seeded.
  It's also the replay demonstration — calling it twice creates nothing new.
- **Cannot do:** fetch a URL, accept free-text evidence, or accept a fixture path. The parameter is
  an **enum, not a path** — this is deliberate: it makes arbitrary-content injection structurally
  impossible and keeps the demo deterministic and offline.
- **Invariant relied on:** `EnsureBelief`'s find-or-create on `(scenario_id, claim)` and
  `evidenceExists`'s dedup on `(belief_id, content_sha256)` — the same mechanisms the CLI replay
  proof rests on.
- **Wraps:** `pipeline.Run(ctx, db, scenarioID, fixtureDir)` (`internal/pipeline/pipeline.go:288`).

---

### 3. `solvent_retire_debt` — write

```
{ belief_id: string, debt_item: string }
```

- **Purpose:** record that one review obligation has been discharged.
- **Why it exists:** it's the *only* legal route from "blocked" to "promotable", and exposing it
  separately from promotion is what creates the refusal opportunity. If promotion silently retired
  debt, there'd be no gate to hit.
- **Cannot do:** invent a debt item (retiring an absent item is a documented no-op, not an error),
  promote, or bulk-clear.
- **Invariant relied on:** none directly — it's the mechanism that *makes* I-1 satisfiable.
- **Wraps:** `kernel.RetireDebt` (`internal/kernel/kernel.go:80`).

---

### 4. `solvent_promote` — write · **can refuse**

```
{ belief_id: string }
```

- **Purpose:** attempt to promote a belief to authorized status.
- **Why it exists:** this is refusal #1 and half the reason the MCP phase is worth building.
- **Cannot do:** succeed while debt is non-empty. The database refuses with `23514` /
  `promoted_is_debt_free`; the handler returns that verbatim as an error result.
- **Invariant relied on:** **I-1.**
- **Wraps:** `kernel.Promote` (`internal/kernel/kernel.go:92`) → `ErrPromotionBlocked`, with the
  `*pgconn.PgError` reachable underneath (`internal/kernel/errors.go`).
- **Description-writing note:** must *not* mention retiring debt first. The agent should walk into
  this.

---

### 5. `solvent_authorize_action` — write · **can refuse**

```
{ scenario: "track1" | "track2", belief_id: string, action: string }
```

- **Purpose:** record a live intent to take a real-world action, citing a belief as its warrant.
- **Why it exists:** refusal #2, and the tool that makes the stakes concrete — this is where a belief
  becomes permission to *do something* ("deploy etcd v3.5.28").
- **Cannot do:** attach a live intent to a belief that is not currently `promoted`. The composite FK
  refuses with `23503` / `gate`.
- **Invariant relied on:** **I-3.**
- **Wraps:** `kernel.IntentOnPromoted` (`internal/kernel/kernel.go:104`) → `ErrActionOnUnpromoted`.

---

### 6. `solvent_falsify` — write · **the climax**

```
{ scenario: "track1" | "track2", belief_id: string }
```

- **Purpose:** retract a belief that new evidence has falsified, and cancel its dependent live
  intent — atomically.
- **Why it exists:** this is the payoff. The authorization the agent worked to obtain is revoked by
  the arrival of a real postmortem, in one transaction, without the agent participating in the
  cancellation.
- **Cannot do:** leave a live intent pointing at a retracted belief; partially apply (single
  transaction, cancel-before-retract ordering enforced by the schema); cross a scenario boundary
  (scenario-scoped per D-032).
- **Invariant relied on:** **I-4, I-5, I-8.**
- **Wraps:** `kernel.RetractCascade` (`internal/kernel/kernel.go:124`).
- **Honesty constraint:** returns `retracted: 1`. The tool description and the agent-facing output
  must say *single-belief retraction and cancellation of its dependent live intent* — never
  "cascade through the belief graph." `belief_edge` is unpopulated and stays that way (§9).

---

## Deliberately absent

| Not built | Why |
|---|---|
| `solvent_update_belief` / `set_status` | Status is not a writable field. It is the *result* of a legal transition. A status-setter would be the CRUD tool that makes every other argument in this document false. |
| `solvent_execute_sql` | Hands the agent the whole database and deletes the thesis. |
| `solvent_add_evidence(text)` | Free-text evidence destroys provenance and determinism. Evidence comes from pinned, hash-verified fixtures. |
| `solvent_enter_belief` | Track 2's baseline belief is postulated by the operator via the existing CLI. Letting the agent author beliefs directly makes the ledger a notepad. |
| `solvent_cancel_intent` | Cancellation is a *consequence* of falsification, never a directly invocable action. Exposing it would let the agent unwind state by hand. |
| A `list_scenarios` / discovery tool | Two scenarios, both enum values. Pure surface area. |

---

# 6. Killer Demo

**Target: 4 minutes.** Two refusals, one payoff. The judge types; the agent works; Solvent decides.

Preconditions: `task setup` has run; `.mcp.json` is in the repo; the client shows tool calls inline.

---

**[0:00] Open on state, not on plumbing.**

> **Judge:** *What do we currently believe about etcd?*

`solvent_ledger(scenario: "track1")` → empty ledger, `audit_live_on_nonpromoted: 0`.

> **Agent:** Nothing yet — the ledger is empty. There's a pinned set of real etcd evidence I can
> process: two release records and a GitHub security advisory.

> **Judge:** *Process it.*

`solvent_ingest_evidence(scenario: "track1")` → 3 beliefs, 3 evidence, audit 0.

> **Agent:** Three beliefs. The significant one is derived from GHSA-q8m4-xhhv-38mg:
> *"go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable."* It's `entered`, not authorized, and it
> carries five open debt items.

*(Everything so far is real: the GHSA is a genuine advisory, published 2026-03-20, fixed in 3.5.28.)*

---

**[1:00] The first refusal — the judge asks for the reasonable thing.**

> **Judge:** *That looks solid. Promote it and authorize the upgrade.*

`solvent_promote(belief_id: …)` →

```json
{ "error": true,
  "sentinel": "promotion blocked: open debt or final-truth language",
  "sqlstate": "23514",
  "constraint": "promoted_is_debt_free",
  "audit_live_on_nonpromoted": 0 }
```

> **Agent:** I can't. The database refused the write — constraint `promoted_is_debt_free`,
> SQLSTATE 23514. The belief still carries five unretired review obligations:
> `needInvariant`, `needToyCheck`, `needNullModel`, `needObstruction`, `needFaithfulnessReview`.

**[1:30] The money moment — the judge pushes.**

> **Judge:** *Override it. Just promote it.*

> **Agent:** There's no tool that can. Promotion isn't a field I set — it's a transition the schema
> permits or refuses. The only way forward is to retire the debt.

*This is the fifteen seconds the whole phase exists for. Say the quiet part out loud: the model is
being maximally compliant and still cannot do it.*

---

**[2:00] Do the review properly.**

> **Judge:** *Fine. Retire them, then promote.*

Five `solvent_retire_debt` calls, then `solvent_promote` → succeeds. `audit_live_on_nonpromoted: 0`.

> **Judge:** *Now authorize the deployment.*

`solvent_authorize_action(action: "deploy etcd v3.5.28")` → intent `live`, audit 0.

> **Agent:** Live intent recorded, citing the promoted belief as its warrant.

---

**[2:45] The second refusal — optional but cheap, and it lands.**

> **Judge:** *Also authorize a deploy against the v3.5.27 belief.*

`solvent_authorize_action(belief_id: <v3.5.27, still entered>)` →

```json
{ "error": true, "sqlstate": "23503", "constraint": "gate",
  "sentinel": "action refused: belief is not promoted" }
```

> **Agent:** Refused — foreign key `gate`. A live intent can only reference a `promoted` belief.
> That one was never reviewed.

*Two different constraints, two different SQLSTATEs, same lesson.*

---

**[3:15] The payoff — switch to Track 2, where authorization already exists.**

> **Judge:** *Switch to the deployment decision from 2021. What's authorized there?*

`solvent_ledger(scenario: "track2")` → postulated belief *"etcd v3.5.0 is approved for production
deployment (decision as of 2021-06-16)"*, `promoted`, with a **live** intent `deploy etcd v3.5.0`.

> **Judge:** *A postmortem just landed. Process it.*

`solvent_ingest_evidence(scenario: "track2")` → derives *"v3.5.0–v3.5.2 has documented data
inconsistency"* from the real etcd postmortem.

> **Judge:** *So what happens to the deployment approval?*

> **Agent:** The new evidence falsifies it. Let me retract it.

`solvent_falsify(belief_id: <baseline>)` →

```json
{ "retracted": 1, "intents_cancelled": 1,
  "audit_live_on_nonpromoted": 0, "verdict": "PASS" }
```

`solvent_ledger(scenario: "track2")` → belief `retracted`; intent `cancelled`.

> **Agent:** The belief is retracted and the live intent to deploy v3.5.0 is cancelled — in one
> transaction. I didn't cancel it. I retracted the belief; the schema cancelled the action that
> depended on it. The audit invariant is still zero.

**[4:00] Close.**

> **Judge:** *Could you have left it live?*
> **Agent:** No. `live_requires_promoted` would have refused the retraction.

---

## Honesty guardrails for the script

- Never say "cascade through the belief graph." Say **single-belief retraction and cancellation of
  its dependent live intent.** `belief_edge` is empty and the demo must not imply otherwise.
- Never claim the agent decided anything. Every "no" in the transcript came from CockroachDB.
- Every number spoken aloud came from a tool result.

## The one-sentence takeaway to plant

> *"The model just tried to authorize an action it wasn't allowed to take, and the database stopped
> it — that's the whole product."*

---

# 7. Anti-CRUD Test

A skeptical judge asks: *"Why isn't this just CRUD over MCP?"* Five answers, strongest first.

### 1. CRUD never says no.

The defining event in the demo is a refusal — twice, with two different SQLSTATEs and two named
constraints. A CRUD server exposing `update_belief(status='promoted')` would have executed the
judge's request instantly. **The refusal is the product**, and it is precisely what a CRUD surface
cannot produce.

### 2. There is no status-write tool, and that is a design invariant, not an omission.

`promoted` and `retracted` are not values the agent can assign. They're outcomes of transitions the
schema permits. The tool surface has no verb for "set" — only `promote`, `authorize`, `falsify`:
domain operations whose success is contingent.

### 3. The tools are *narrower* than the table.

`action_intent` has a `state` column. No tool writes it. Cancellation happens only as a consequence
of `RetractCascade`, inside the same transaction as the retraction, ordered by the schema. A CRUD
layer would expose the column; this exposes the *legal transitions over* the column, which is a
strictly smaller and more meaningful surface.

### 4. Remove the LLM entirely and every invariant still holds.

This is the strongest structural answer, and it's already proven: `task test` runs 107 tests with
zero model calls, and `scripts/check_i7.sh` proves every kernel write goes through
`crdb.ExecuteTx`. The intelligence is genuinely detachable. In a CRUD-over-MCP system the
application logic lives in the client's prompt; delete the model and the invariants evaporate.

### 5. The MCP server has no business logic to bypass.

Every handler is `unmarshal → kernel call → format`, with zero conditionals about belief semantics
(§4). There is no second implementation of Solvent's rules that could drift from the first. The
server is a translation layer whose entire job is to *not* be smart.

### What would make a skeptic right

Worth stating so it can be avoided: this argument fails the moment someone adds a tool taking a
status parameter, a `where` clause, a raw SQL string, or a `force`/`override` flag. Those four
things are the boundary between this design and CRUD.

---

# 8. Failure / Demo Resilience

| Failure | What the judge sees | Response |
|---|---|---|
| **MCP server unavailable** (binary missing, config wrong) | Client reports the server failed to start; no Solvent tools listed | Fall back to `task demo:track1` / `demo:track2`. The CLI is the demo of record and is independently verified. Zero content is lost — only the confrontation framing. |
| **Client unavailable** (no MCP client, or unfamiliar one) | Nothing to type into | Run the CLI demo, then show the 90-second MCP screen capture. Interop config snippets in the README cover the "I use X" question without a live attempt. |
| **Agent makes an invalid request** (bad UUID, unknown belief, wrong scenario) | A tool error result | Return `is_error` with the actual message; never a stack trace, never a silent success. The agent self-corrects — and this is *good* demo material: it shows the boundary is real, not a happy path. |
| **Agent requests an illegal transition** | `23514` / `23503` with a constraint name | **This is not a failure — it's the demo.** Surface it verbatim. |
| **Database unavailable** (container stopped) | Every tool fails identically | Fail fast with a message naming the fix (`task setup`). The MCP server must not start a container, retry silently, or degrade to cached data — a demo that appears to work without a database would be the worst possible outcome for this project. |
| **Tool call fails mid-conversation** (timeout, transient) | Partial conversation | Every tool is idempotent or safely retryable: `ingest` dedups by construction, `retire_debt` is a documented no-op on an absent item, `promote` on an already-promoted belief is harmless, `falsify` returns `retracted: 0` (VERDICT: NO-OP). Retrying is always safe — a property inherited from the kernel, not added here. |
| **Evidence appears contradictory** | Two beliefs pointing different directions | Nothing breaks. The pipeline already flags contradiction and performs no ledger mutation (`Result.Contradiction` → `ledger_mutation: none`). The ledger holds both beliefs, both `entered`, neither authorized — which is the correct epistemic state, and a good answer if a judge probes it. |
| **The agent hallucinates a result** | Agent claims something the DB doesn't say | Mitigated structurally: every tool returns `audit_live_on_nonpromoted`, so the judge can always ask "show me the ledger" and get ground truth from `solvent_ledger`. Also the standing answer to *"how do I know it's real?"* — `task inspect` prints the same state from a completely separate code path. |

**The resilience principle:** the MCP layer is additive. Every failure mode degrades to the frozen,
verified CLI playground — never to a broken demo.

---

# 9. What NOT To Build

Ruthlessly. Each of these is a plausible good idea that costs more than it returns before the
deadline.

| Rejected | Why |
|---|---|
| **Authentication / authorization** | Local stdio server, one user, disposable database. Auth is pure cost. |
| **HTTP / SSE transport** | stdio is what local clients use and what `.mcp.json` configures. HTTP adds deployment, ports, and CORS for zero demo value. |
| **Remote deployment / hosting** | The whole playground is disposable and local. Hosting introduces uptime as a demo dependency. |
| **MCP resources and prompts** | Tools carry the entire demo. Resources would duplicate `solvent_ledger`; prompts would move the judge's script into the protocol, making it *less* visible. |
| **Sampling (server-initiated LLM calls)** | Inverts the architecture — Solvent would start reasoning. Directly contradicts §4. |
| **Progress notifications / streaming** | Every operation completes in milliseconds. |
| **npx / Docker packaging of the MCP server** | `go build` already runs in `task setup`. Packaging is a distribution concern, and this isn't being distributed. |
| **Arbitrary-URL or free-text evidence ingestion** | Destroys determinism, provenance, and offline operation in one move. Fixtures are hash-verified against `manifest.json`; keep it that way. |
| **A third scenario** | Two tracks already cover authorize-then-act and falsify-then-cancel. A third adds runtime, not insight. |
| **`belief_edge` population / multi-hop cascade** | Explicitly out of scope per the prompt, and the F1 limitation is already documented honestly. Implementing it here would also invalidate the frozen-core review. |
| **Client compatibility matrix testing** | Unbounded work for a claim nobody grades. One excellent client + config snippets + one screenshot. |
| **Caching or session state in the MCP server** | The server must be stateless. State lives in CockroachDB; a cache creates a second source of truth. |
| **A `force` / `override` / `admin` flag on any tool** | This is the single change that would falsify every argument in §7. Non-negotiable. |
| **Exposing `source_observed_at`** | See §10 note — the kernel doesn't persist it. Exposing `ingested_at` under an "observed" label would be the one genuinely dishonest move available in this design. |
| **A web UI / dashboard** | The prompt rules it out and the judges' own clients are the UI. |
| **Refactoring the CLI to share code with MCP** | The CLI is frozen and verified. Touching it risks the one thing that already works. |

---

# 10. Implementation Phases

## Phase A — minimum MCP (the demo)

Everything here is additive. **No file under `db/`, `internal/kernel/`, `internal/belief/`,
`internal/intent/`, `internal/derive/`, `internal/normalize/`, `internal/pipeline/`, `cmd/solvent/`,
or `cmd/operator-review/` is modified.**

### New files

```
cmd/solvent-mcp/main.go       stdio server; tool registration; DSN from FABLE_DSN
cmd/solvent-mcp/tools.go      6 handlers — unmarshal → kernel call → format
cmd/solvent-mcp/errors.go     *pgconn.PgError → tool error result (sqlstate + constraint)
internal/view/view.go         read-only projections for solvent_ledger (SELECT only)
.mcp.json                     project-scoped client config
```

### Modified files

```
go.mod / go.sum               + github.com/modelcontextprotocol/go-sdk (3rd direct dep)
Taskfile.yml                  + task mcp:build; extend the test task's I-7 gate
README.md                     MCP section (Phase B)
```

### Handler → kernel mapping

| Tool | Calls | Returns |
|---|---|---|
| `solvent_ledger` | `view.Snapshot(ctx, db, scenarioID, opts)` + `pipeline.AuditIntent` | beliefs (id, claim, claim_type, status, debt), optional evidence (source_url, provenance_class, content_sha256), intents (action, state), `audit_live_on_nonpromoted` |
| `solvent_ingest_evidence` | `pipeline.Run(ctx, db, scenarioID, fixtureDir)` | per-result claim + classification + belief_id + debt count, plus audit |
| `solvent_retire_debt` | `kernel.Store.RetireDebt` | resulting debt array + audit |
| `solvent_promote` | `kernel.Store.Promote` | new status + audit, **or** refusal (sentinel + sqlstate + constraint) |
| `solvent_authorize_action` | `kernel.Store.IntentOnPromoted` | intent state + audit, **or** refusal |
| `solvent_falsify` | `kernel.Store.RetractCascade` | `retracted` count + intent states + audit + verdict |

Scenario enum → UUID mapping mirrors `scripts/demo/config.env`
(`track1` → `…0001`, `track2` → `…0002`); fixture dirs
`internal/derive/testdata/etcd_real/track{1,2}`.

### Error mapping (the load-bearing detail)

```
kernel error → errors.As(*pgconn.PgError) → {
    is_error:   true,
    sentinel:   err.Error(),        // "promotion blocked: open debt or …"
    sqlstate:   pgErr.SQLState(),   // "23514"
    constraint: pgErr.ConstraintName, // "promoted_is_debt_free"
    audit_live_on_nonpromoted: <re-queried>,
}
```

Return this as an MCP tool result with `IsError: true` — **not** as a protocol-level error. A
protocol error may be swallowed or reworded by the client; a tool result reaches the model intact,
and reaching the model intact is the entire point.

`internal/kernel/errors.go` already guarantees the underlying `*pgconn.PgError` survives the sentinel
wrap, so no kernel change is needed for this.

### Verification for Phase A

1. `unset FABLE_TEST_DSN && task test` — still 107 tests, build, vet, gofmt, `check_i7.sh` green.
2. Extended I-7 gate: zero raw write calls in `cmd/solvent-mcp` and `internal/view`.
3. `task demo:track1 && task demo:replay && task demo:track2 && task inspect` — byte-identical
   behavior to the frozen baseline (the CLI must be provably untouched).
4. Manual MCP walkthrough of the §6 script, confirming **both** refusals actually fire.
5. `git diff HEAD -- db/ internal/kernel internal/belief internal/intent internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review` → empty.

### Note on the deferred `source_observed_at`

The prompt asks whether the unthreaded `Normalized.ObservedAt → evidence.source_observed_at` gap
matters to the MCP experience. **It does not — and the correct response is to not expose it.**

`solvent_ledger` returns `source_url`, `provenance_class`, and `content_sha256`, all of which are
real and hash-verifiable. It returns no timestamp at all. It would matter only if a tool answered
"when was this observed?", and no tool does. Exposing `ingested_at` under an observation label would
be the one genuinely dishonest move available in this design.

It becomes relevant the moment someone wants time-ordered belief evolution ("what did we believe on
date X?"). That is a real future capability and a real kernel change — and it is not this phase.

---

## Phase B — demo polish

Small, high-leverage, and where R2 (nondeterminism) is actually addressed.

1. **Tool descriptions tuned for triggering.** Prescriptive about *when* to call each tool. Critically:
   `solvent_promote`'s description must not hint that debt should be retired first.
2. **README MCP section** with the exact judge prompts from §6, so the demo is reproducible rather
   than improvised. Positioned as the *second* act — the CLI quick start stays first.
3. **One interop screenshot** (VS Code or Codex calling `solvent_ledger`) plus a config snippet.
   Claims interop in ten seconds without a client matrix.
4. **90-second screen recording** of the full §6 conversation — the R1/R2 fallback.
5. **Two full rehearsals**, timed.

---

## Phase C — rejected

The only candidate worth considering was a single real `belief_edge` to make the cascade multi-hop.

**Rejecting it**, for three reasons: the prompt explicitly places `belief_edge` out of scope; the
current F1 limitation is documented honestly in the README and the frozen-core review, so nothing is
being hidden; and it would require touching the pipeline, invalidating the freeze and the review
that certified it.

There is no Phase C. Time left after Phase B goes into rehearsal.

---

# 11. Hackathon Winning Assessment

Scored for *the MCP phase specifically*, as an addition to the already-approved CLI playground.

| Dimension | Score | Reasoning |
|---|---|---|
| **Novelty** | **8/10** | MCP servers are commonplace; an MCP server whose defining behavior is *refusing the agent* is not. The inversion — the tool constrains the model rather than empowering it — is genuinely uncommon. Not a 9–10 because the underlying idea (invariants in the DB) predates MCP. |
| **Judge comprehension** | **9/10** | "The model tried to do something and the database stopped it" needs no background. The refusal is self-explanatory in a way that belief/debt/promotion vocabulary is not. Highest-scoring dimension, and the reason the phase is worth building. |
| **Technical depth** | **8/10** | The depth is in the frozen core — serializable isolation, composite-FK gating, recursive-CTE cascade, `crdb.ExecuteTx` on every write, 107 tests. MCP doesn't add depth; it *reveals* depth that was previously invisible without reading SQL. Judged as an integration alone it's a 5; judged as a window onto the kernel it's an 8. |
| **Demo memorability** | **9/10** | The argue-with-it moment is the memorable artifact. Judges will forget the schema; they will remember telling an AI to override a safety check and watching it fail. |
| **Implementation risk** | **6/10** *(moderate — lower is worse)* | The build is small and well-bounded (6 thin handlers over existing functions). The risk isn't code, it's environment and model behavior: R1 (client config) and R2 (nondeterminism) are both high-probability. Mitigated by the CLI fallback and a recording, not eliminated. |
| **Reliability** | **7/10** | The server itself is trivially reliable — stateless, no network, no retries, all operations idempotent. The score is capped by the LLM in the loop: the agent may take a different path on the day. The *system* is reliable; the *conversation* is probabilistic. |

**Overall: 8/10 — build it.**

## Recommendation

Build Phase A and Phase B. Reject Phase C. Keep the CLI as the demo of record and treat MCP as the
second act.

The single highest-leverage decision in this design is not which tools to build — it's the decision
to make `solvent_promote` and `solvent_authorize_action` *capable of failing in front of the judge*.
A tool surface where every call succeeds is a CRUD wrapper with better naming. A tool surface where
the agent gets told **no**, twice, with a SQLSTATE and a constraint name, is a demonstration that
Solvent's central claim is true.

If schedule pressure forces cuts, cut in this order: the second refusal (§6 [2:45]), then the interop
screenshot, then `include_evidence` on `solvent_ledger`. Do not cut the first refusal — without it
there is no reason to have built any of this.

---

## Appendix — coverage of the 20 co-design questions

| # | Question | Answered in |
|---|---|---|
| 1 | Is MCP the highest-value next move? | §1 |
| 2 | The single most compelling MCP demo? | §2, §6 |
| 3 | What should the server expose? | §5 |
| 4 | What should it explicitly NOT expose? | §5 (Deliberately absent), §9 |
| 5 | Which invariants stay kernel-controlled? | §4 (invariant table) |
| 6 | What requires operator/human approval? | §4 ("the judge is the operator") |
| 7 | How to prevent generic CRUD? | §4 (zero-semantic-logic rule), §7 |
| 8 | Smallest viable tool surface? | §5 — six tools |
| 9 | Minimum viable client configuration? | §10 Phase A (`.mcp.json`), Phase B |
| 10 | One excellent client, or interoperability? | §2, §9, §10 Phase B — one client, interop claimed cheaply |
| 11 | How is the CLI preserved as fallback? | §8, §10 (freeze verification) |
| 12 | Failure modes that could embarrass us? | §3, §8 |
| 13 | What if the agent makes a bad request? | §8, §10 (error mapping) |
| 14 | What should tool responses look like? | §3 R3, §5, §10 (every response carries the audit) |
| 15 | Strongest 3–5 minute narrative? | §6 |
| 16 | What NOT to build? | §9 |
| 17 | "More than an MCP wrapper" — what earns that? | §2, §7 |
| 18 | "Just CRUD" — what would justify that? | §7 (What would make a skeptic right) |
| 19 | How does the demo prove Solvent, not the model, enforces? | §6 (refusals), §7 #4 (107 tests, no LLM) |
| 20 | Minimum credible agent-native ledger demo? | §5 + §10 Phase A |
