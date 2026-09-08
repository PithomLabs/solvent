## prompt

what are the gaps that need to be filled in about the MCP workflow between Solvent and AI/coding/harness agents? I want it itemized with brief description organized around categories, make it non-technical, the goal is documentation for humans and AI agents


## answer

Based on what's actually documented in the Solvent repo (its own "What NOT to Build" list, verified-limits sections, and forward-looking notes), here are the gaps in the MCP workflow between Solvent and any calling agent/harness — written for a general audience.

## Gap Inventory: Solvent ↔ Agent/Harness MCP Workflow

### 1. Who is allowed to call which tool

- **No caller identity or authentication.** The MCP server doesn't know or check who (which agent, which user, which session) is calling it — it currently treats every caller the same and relies on the connecting process being trusted [1](#3-0) .
- **No built-in role restriction on tools.** The tools that mutate state (retiring a review item, promoting a belief, cancelling a belief) are available to any caller of the server; nothing inside Solvent itself limits which agents may call them.
- **Tool restriction is left to whoever hosts the agent.** Solvent's design assumes the surrounding harness — not Solvent — decides which of its six tools a given agent is even allowed to attempt. If the host doesn't set that boundary, the "gap" isn't filled by Solvent.

### 2. What "authorized" actually covers

- **Authorization is coarse-grained today.** A promoted belief can currently back *any* stated action, not one specific, pre-approved action. This means a genuinely reviewed and approved belief could, in principle, be cited to justify a different action than the one it was actually reviewed for. Closing this (binding approval to an exact target-and-action pair) is called out as forthcoming work, not yet built.
- **The system trusts a caller's self-reported "where did this come from" label at face value for logging, but not for the real decision.** A caller can claim an instruction came from a trustworthy source, and that claim is recorded, but it is never treated as proof — the only thing that actually gates the action is the underlying review state. This is by design, but it means anyone reading Solvent's logs must remember that self-declared origin fields are hygiene/audit notes, not verified facts.

### 3. What happens outside the tool call

- **Solvent has no control over what an agent does without calling Solvent at all.** If an agent decides to skip Solvent's tools and act directly (run a command, call another API), nothing in Solvent detects or stops that. That responsibility sits entirely with the surrounding agent runtime (its sandboxing, permissions, network access), which is outside this project's scope.
- **No sandboxing, no network/egress controls, no credential isolation.** These are explicitly named as things Solvent does not attempt to provide.

### 4. How Solvent is reached and hosted

- **Only local, one-user, single-process connections are supported today.** The server talks over a local stdio connection; there's no remote/network-hosted version, and no login/session concept — it was deliberately built for a single local user rather than a shared, multi-tenant service [1](#3-0) .
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

**How to use this list:** each row above is a place where a team adopting Solvent (or an AI agent operating under it) must supply its own answer — usually at the "host" or "harness" layer around Solvent — rather than assuming Solvent already covers it. Solvent's own position is that it supplies one narrow, reliable piece (a review-and-approval gate on a shared record of beliefs and actions); everything above the line — identity, precise per-action approval, sandboxing, hosting, and scale-testing — still needs to be built or verified by whoever deploys it. [1](#3-0) [2](#3-1) [3](#3-2)

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

