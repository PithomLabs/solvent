Got it. **Plan 4 is the execution plan for the approved MCP design**, and it explicitly says it does not reopen the design decisions. 

I would **not send it to the coding agent completely unchanged**, though. I spotted one concrete implementation trap:

### Fix before handoff: the Taskfile grep gate

Plan 4 proposes:

```sh
grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view && echo "FAIL: raw writes in MCP/view" && exit 1 || echo "I-7 MCP boundary: PASS"
```

The shell chaining is wrong for a failure case: because of the final `|| echo`, a matching grep can produce **both `FAIL` and `PASS`** after `exit 1` triggers the `||` branch.

Use an explicit shell block instead:

```sh
if grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view; then
  echo "FAIL: raw writes in MCP/view"
  exit 1
else
  echo "I-7 MCP boundary: PASS"
fi
```

That is the only issue I'd insist on fixing **before implementation**.

Everything else in Plan 4 is appropriately concrete:

* frozen-core boundary is explicit; 
* the actual kernel/pipeline APIs were verified rather than assumed; 
* six tools have concrete implementation contracts; 
* Track 2 seed → ingest is explicitly tested rather than assumed; 
* the grounding rule has the corrected wording; 
* the definition of done requires **actual MCP-client rehearsal and current transcripts**, not merely compilation; 
* the stop conditions appropriately prevent the coding agent from "solving" problems by weakening the design. 

So my status is:

**Plan 4: APPROVE WITH ONE MECHANICAL CORRECTION → then implement.**

I would **not send this back to Claude for another design review**. The next useful step is the coding agent implementing this plan and producing the receipts.
