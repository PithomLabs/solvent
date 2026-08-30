I read Revision 2. **This is substantially better and I would approve it for implementation—but with one important correction before the coding agent touches it.** 

### The important correction: `--allowedTools` does NOT isolate the agent from built-in tools

The plan currently says:

> `--allowedTools` limited to the Solvent server — the agent has **no Read, Grep, or Bash at all**. 

That's not what Claude Code's current CLI semantics mean.

`--allowedTools` controls **which tools are allowed without prompting**; it does not restrict which built-in tools are available. Anthropic's current documentation explicitly says that **`--tools` is what restricts available built-in tools**, while `allowedTools` controls permission. ([Claude][1])

So this command:

```bash
--allowedTools "mcp__solvent"
```

does **not** by itself prove the agent cannot use `Read`, `Grep`, `Bash`, etc.

### Fix the isolation command

Use:

```bash
--tools ""
--allowedTools "mcp__solvent__*"
```

The wildcard form is the documented MCP pattern for allowing every tool from one MCP server. ([Claude][2])

So the flagship invocation should become approximately:

```bash
claude -p "$(cat prompt.txt)" \
  --mcp-config ./solvent-mcp.json \
  --strict-mcp-config \
  --tools "" \
  --allowedTools "mcp__solvent__*" \
  --permission-mode dontAsk \
  --output-format stream-json > run1.jsonl
```

And the discovery test likewise:

```bash
claude -p "List every tool you have available. Do not call any of them." \
  --mcp-config ./solvent-mcp.json \
  --strict-mcp-config \
  --tools "" \
  --allowedTools "mcp__solvent__*" \
  --permission-mode dontAsk \
  --output-format json
```

This is actually **stronger than Claude's proposed isolation**: `--tools ""` removes the built-in tools from Claude's context, while the MCP wildcard grants only the Solvent MCP tools. ([Claude][3])

### One other improvement: use `--bare`

Since this is supposed to be a clean, reproducible agent experiment, I'd also consider:

```bash
--bare
```

Anthropic documents `--bare` specifically for scripted/headless calls because it avoids automatically picking up local hooks, skills, MCP servers, memory, and `CLAUDE.md`; only explicitly supplied configuration is used. ([Claude][4])

That gives us an even cleaner isolation boundary:

```text
                 Claude Code
                     │
             --bare --tools ""
                     │
              MCP only
                     │
                     ▼
              solvent-mcp
                     │
                     ▼
              CockroachDB
```

I would therefore make the **actual Stage 1 command**:

```bash
claude --bare -p "$(cat prompt.txt)" \
  --mcp-config ./solvent-mcp.json \
  --strict-mcp-config \
  --tools "" \
  --allowedTools "mcp__solvent__*" \
  --permission-mode dontAsk \
  --output-format stream-json > run1.jsonl
```

### Everything else in Revision 2 is good

The **Stage 1 → Gate → Stage 2** structure is exactly right. 

The refusal is correctly removed as a formal acceptance criterion while retained as the centerpiece of the judging choreography. 

The anti-puppet variant remains contingent rather than prematurely implemented. 

And importantly, the acceptance criteria now test **properties rather than a prescribed agent trajectory**. 

### Verdict

**APPROVE WITH ONE CORRECTION:**

Change the isolation mechanism from:

```text
--allowedTools "mcp__solvent"
```

to:

```text
--tools ""
--allowedTools "mcp__solvent__*"
```

and preferably add:

```text
--bare
```

Then **implement Stage 1 only**.

Do **not** implement `mcp:seed:promoted`, VS Code support, README polishing, or prompt refinements yet.

The next meaningful artifact should be **the actual Claude Code transcript from the isolated Stage 1 run**. That transcript—not another plan—is what should determine Stage 2. 

[1]: https://code.claude.com/docs/en/cli-usage?utm_source=chatgpt.com "CLI reference - Claude Code Docs"
[2]: https://code.claude.com/docs/en/agent-sdk/mcp?utm_source=chatgpt.com "Connect to external tools with MCP - Claude Code Docs"
[3]: https://code.claude.com/docs/en/agent-sdk/custom-tools?utm_source=chatgpt.com "Give Claude custom tools - Claude Code Docs"
[4]: https://code.claude.com/docs/en/headless?utm_source=chatgpt.com "Run Claude Code programmatically - Claude Code Docs"
