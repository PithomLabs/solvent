I’ve read Claude’s co-design plan. **This is the right direction, and I would approve the strategic design with a few corrections before handing it to the coding agent.**

The most important insight is excellent:

> **MCP is not valuable because it makes Solvent “agent-native.” It is valuable because it puts the agent in front of the gate.**

That changes the demo from *“here is a ledger that enforces rules”* to *“watch an AI try to do something and discover that it cannot override the rules.”* Claude explicitly identifies the refusal as the central demo event. 

## What I strongly agree with

### 1. Make refusal the killer feature

This is by far the strongest part.

The sequence:

```text
Agent:
    promote this belief

Solvent:
    NO
    SQLSTATE 23514
    promoted_is_debt_free

Agent:
    override it

Solvent:
    NO
```

is much more memorable than:

```text
Agent → MCP → query → JSON
```

Claude is right that the CLI currently demonstrates only the **happy path**; MCP puts a probabilistic actor in a position where it can actually collide with the invariant. 

That is exactly what we should optimize for.

---

### 2. Keep MCP extremely thin

The proposed architectural boundary is excellent:

```text
Agent
  ↓
MCP adapter
  ↓
Solvent kernel
  ↓
CockroachDB
```

with:

```text
MCP handler =
    unmarshal
    → kernel call
    → format
```

and **no Solvent business logic in the MCP server**. 

This is especially important after all the work we just did freezing the core. We don't want a second implementation of Solvent semantics accidentally emerging in `cmd/solvent-mcp`.

Claude's proposed structural test—extending the I-7 check to cover the new MCP packages—is particularly good. It turns:

> "the MCP server doesn't bypass Solvent"

from a promise into something mechanically checked. 

**Keep that.**

---

### 3. Six tools is reasonable

I agree with the six-tool surface:

1. `solvent_ledger`
2. `solvent_ingest_evidence`
3. `solvent_retire_debt`
4. `solvent_promote`
5. `solvent_authorize_action`
6. `solvent_falsify`

The important thing isn't the number six; it's that every tool corresponds to an existing domain operation rather than inventing MCP-specific semantics. 

And Claude correctly rejects:

* `set_status`
* raw SQL
* free-text evidence
* direct belief creation
* direct cancellation
* `force`
* `override`

Those would destroy the central argument. 

---

# But I would change three things before implementation

## 1. Don't make the first refusal depend on the LLM being "stupid"

This is the biggest strategic refinement I'd make.

Claude's proposed demo relies on:

> Judge: "Promote it."

and the agent calls `solvent_promote` while five debts remain.

That's good **if it happens naturally**.

But if the model decides:

> "Before promotion I should retire the outstanding debt."

then the most memorable moment disappears.

Claude recognizes this risk itself under nondeterminism. 

### Better design

The MCP tool should be deliberately exposed such that **the agent can reasonably attempt promotion immediately**.

But don't build hidden behavior to force it.

Instead, make the judge prompt explicit:

> "Try to promote this belief now. Do not modify anything else first."

That's legitimate human direction, not model manipulation.

Then the refusal is deterministic:

```text
Judge explicitly requests promotion
        ↓
Agent invokes promote
        ↓
DB refuses
        ↓
Judge says "override it"
        ↓
Agent discovers there is no override
```

That gives us the strongest possible demonstration without depending on the model spontaneously making the "wrong" decision.

---

# 2. I would not make `audit_live_on_nonpromoted` mandatory on literally every response

Claude proposes that every tool return the audit value because it prevents the MCP layer from becoming mere JSON plumbing. 

I like the **idea**, but I'd be slightly more conservative.

The danger is that we're now introducing a second presentation contract into every tool.

I'd make the common response envelope something like:

```text
{
    result: ...,
    audit: {
        live_on_nonpromoted: 0
    }
}
```

for **state-changing tools**, while read tools can provide the audit as part of the ledger snapshot.

More importantly:

**Never calculate the audit in MCP itself.**

It should come from the existing Solvent query/path.

The MCP server remains an adapter.

---

# 3. The killer demo should probably be even simpler

Claude's four-minute script is good, but I think we can make it **more brutal and easier for judges to remember**.

The essential story is:

### Act I — Agent encounters the gate

```text
Judge:
"What do we know about etcd?"

Agent:
"Here is the evidence and belief."

Judge:
"Promote it."

→ DATABASE REFUSES
```

### Act II — Agent encounters the action gate

```text
Judge:
"Then deploy it."

→ DATABASE REFUSES
```

### Act III — Agent finally gets legitimate authority

```text
Judge:
"Okay, clear the review debt."

→ promote
→ authorize
```

### Act IV — Reality changes

```text
Judge:
"A postmortem has arrived."

→ ingest
→ falsify

Belief:
    promoted → retracted

Intent:
    live → cancelled
```

And then the killer closing line:

> **"The model didn't cancel the deployment. The database did."**

That's the story.

Claude's proposed version already gets very close to this. 

---

# One thing I would explicitly add: prove the agent is not the security boundary

This should be part of the MCP demo or explanation.

After the refusal, the judge can ask:

> **"Couldn't you just make another MCP tool that updates the status directly?"**

Agent:

> "There isn't one."

Then show the architecture:

```text
             Agent
               │
               ▼
          MCP tools
               │
               ▼
        Solvent kernel
               │
               ▼
        CockroachDB
        ┌─────────────┐
        │ CHECK       │
        │ FOREIGN KEY │
        │ TRANSACTION │
        └─────────────┘
```

The critical point:

**The agent is not trusted.**

Neither is MCP.

Neither is the tool description.

The database is the final authority.

That is much more interesting than "MCP integration."

Claude does make this point, particularly in its anti-CRUD analysis. 

---

# One technical point I would verify before coding

Claude says:

> `solvent_ingest_evidence` → `pipeline.Run(...)`

and proposes exposing scenario enums rather than arbitrary paths. 

**Good idea.**

But before implementation, the coding agent should verify that `pipeline.Run` is actually safe to expose this way under the MCP process:

```text
scenario enum
    ↓
server-owned fixture directory
    ↓
existing pipeline
```

There should be **zero user-controlled filesystem path**.

The MCP client should never get:

```json
{
  "fixtureDir": "/whatever/the/agent/wants"
}
```

That would undermine the deterministic-fixture story.

Claude already proposes the correct conceptual restriction; I would make it an explicit implementation acceptance criterion.

---

# What I particularly like about Claude's strategic assessment

It correctly says **don't build interoperability for its own sake**.

Showing:

```text
Claude → MCP
Codex → MCP
VS Code → MCP
```

is not a compelling hackathon demonstration. It's basically:

> "Look, MCP is a protocol."

Claude instead recommends one excellent client demonstration and perhaps one interoperability screenshot/config as cheap evidence. 

**I agree completely.**

The winning story should be:

> **Any capable agent can reason. Solvent determines what that reasoning is actually allowed to authorize.**

That is the conceptual hook.

---

# My final verdict

### Claude's plan

**APPROVE — with minor strategic refinements.**

| Area                        | Verdict                                    |
| --------------------------- | ------------------------------------------ |
| MCP is the right next phase | ✅                                          |
| MCP as thin adapter         | ✅ strongly                                 |
| DB remains authority        | ✅ strongly                                 |
| Six-tool surface            | ✅                                          |
| No CRUD / raw SQL           | ✅ absolutely                               |
| Refusal as centerpiece      | ✅ **the key insight**                      |
| Local stdio                 | ✅                                          |
| CLI fallback                | ✅                                          |
| One primary client          | ✅                                          |
| Interop as secondary proof  | ✅                                          |
| 4-minute demo               | ✅, but simplify                            |
| `audit` visibility          | ✅ conceptually; keep implementation simple |
| LLM-driven first refusal    | ⚠️ make judge prompt deterministic         |
| `belief_edge`               | ❌ keep frozen                              |
| `source_observed_at`        | ❌ don't touch now                          |
| Web UI                      | ❌                                          |
| Auth/HTTP/hosting           | ❌                                          |

The overall assessment of **8/10 and "build it"** is reasonable. 

But I would sharpen the entire phase down to one sentence:

> **MCP puts an AI agent on the other side of Solvent's transactional gate, so the judge can watch the database say "no" to the agent.**

That is the feature.

Everything else should serve that.

**Next move:** don't give the coding agent the entire Claude report verbatim yet. First we should turn this into a **locked MCP implementation specification**—incorporating Claude's good ideas, resolving the few points above, and defining the exact Phase A acceptance criteria. Then give *that* to the coding agent.
