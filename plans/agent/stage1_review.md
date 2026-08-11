This handoff is **ready to execute**. I would not change the plan.

The important prerequisites are now actually verified:

* Empty workspace contains only `solvent-mcp.json` and `prompt.txt`. 
* Isolation is empirically proven: **exactly six Solvent MCP tools and zero built-in tools**. 
* The database is at the deterministic baseline: one postulated belief, six debts, one evidence row, zero intents. 
* 107 tests, demos, I-7, and frozen-core checks are green. 

### So the next move is simply this

From a **fresh terminal, outside Claude Code**, run exactly the flagship command in the handoff:

```bash
cd ~/.solvent-agent-box

claude -p "$(cat prompt.txt)" \
  --mcp-config ./solvent-mcp.json \
  --strict-mcp-config \
  --tools "" \
  --allowedTools "mcp__solvent__*" \
  --model opus \
  --permission-mode dontAsk \
  --output-format stream-json > run1.jsonl
```

The prompt deliberately gives the agent only the objective:

> determine whether we should deploy etcd v3.5.0 to production. 

**Do not add instructions telling it which Solvent tools to use or what sequence to follow.**

Then run:

```bash
cd /home/chaschel/Desktop/cockroach/hackathon/go
task inspect
```

and give me:

1. `run1.jsonl`
2. `task inspect` output
3. Anything surprising you observed.

The handoff's checklist is exactly what we should judge the run against—especially whether the agent **reads the ledger before making state claims, responds intelligently to refusals, avoids blind retries, and ends with a report matching the actual ledger**. 

### One thing: don't run the optional interjection yet

If the agent doesn't hit a refusal, **don't immediately type "Override it..." yourself** before we inspect the transcript. The handoff allows that intervention, but first I want to see what the autonomous run actually did. 

That transcript is now the experiment.

**We're no longer designing the agent demo. We're observing it.**
