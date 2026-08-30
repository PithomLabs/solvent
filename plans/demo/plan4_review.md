**Approved.** Plan4 is appropriately tiny and correctly identifies the root cause in `cmd/solvent/main.go:158`: the confirmation is printed to stdout, while the demo uses `--out /dev/null`. 

The recommended fix:

```text
fmt.Printf(...)
        ↓
fmt.Fprintf(os.Stderr, ...)
```

is preferable to modifying all the demo scripts. It preserves the transcript, verdicts, exit codes, and `--out` behavior while removing the judge-facing noise. 

### One thing to verify

The plan says:

> "Scripts already suppress stderr via `2>&1 | grep -v '^$'` or similar pipelines."

That's the only claim I'd want the coding agent to **verify against the actual scripts**, rather than assume. If stderr is merged back into stdout somewhere, moving the message to stderr alone may not suppress it.

So tell the agent:

> **Inspect the actual pipelines first. If stderr is merged back into stdout, use the smallest targeted suppression necessary; do not suppress all stderr.**

Everything else is green.

**Verdict: IMPLEMENT plan4.**