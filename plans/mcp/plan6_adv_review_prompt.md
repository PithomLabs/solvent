# FINAL ADVERSARIAL CODE REVIEW — SOLVENT MCP PLAN 6

Repository:
https://github.com/PithomLabs/solvent

Review the ACTUAL current repository, not the implementation summary
(`plans/mcp/plan6_imp.md`). Treat all reported PASS/verification claims as
untrusted until independently reproduced.

Read:
- plans/mcp/plan6.md
- plans/mcp/plan5_review.md
- plans/mcp/plan6_imp.md

This is the FINAL code review. Do not redesign the architecture.

Focus specifically on:

1. I-7 gate:
   - inspect the exact Taskfile implementation;
   - clean repository must PASS;
   - all six malicious injections must FAIL;
   - missing `cmd/solvent-mcp` and `internal/view` must FAIL;
   - verify `QueryRowContext` SELECTs in `internal/view` still PASS;
   - specifically test:
       ExecContext write
       tx.ExecContext
       gofmt-wrapped ExecContext
       QueryRowContext + UPDATE ... RETURNING
       const-hoisted UPDATE + ExecContext
   - ensure the gate cannot silently PASS on grep errors.

2. F-2:
   - malformed/missing/empty/non-string `belief_id` must be rejected;
   - cross-scenario belief IDs must produce zero mutation;
   - valid same-scenario mutation must still work;
   - no MCP-side business-rule duplication.

3. F-3:
   - absent `belief_id` → scenario-wide ledger;
   - malformed supplied `belief_id` → error;
   - valid UUID → scoped ledger.

4. Regression:
   - `task --list`
   - `go build ./...`
   - `go vet ./...`
   - `gofmt -l cmd internal`
   - `unset FABLE_TEST_DSN && task test`
   - existing demos
   - frozen-core diff
   - git status

5. Verify the implementation summary's receipts against fresh execution.
   Do not reuse stale transcripts.

For every finding provide:

    ID
    Severity
    file:line evidence
    reproduction
    impact
    exact fix

Final verdict must be exactly one of:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

Do not manufacture blockers. If the implementation passes, say so clearly.

Most important question:

> Is the MCP implementation now ready for the final fresh-machine test
> with a real LLM coding agent?

Run it, attack it, and decide from the actual repository.