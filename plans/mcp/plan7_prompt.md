# SOLVENT — FINAL MCP HARDENING FIXES

Implement the four remaining findings from the final adversarial review
(`plans/mcp/plan6_adv_review.md`). Do not redesign anything and do not
modify the frozen core.

1. A-1 — `solvent_authorize_action`
   - In `cmd/solvent-mcp/tools.go`, require `belief_id` to be a non-empty
     string.
   - Before `IntentOnPromoted`, perform the same identity-only
     scenario/belief ownership check already used by `solvent_promote`.
   - A cross-scenario belief must return a clean error and cause zero
     mutation.
   - Do not inspect debt/status/intent state; the DB remains authoritative.

2. A-2 — `solvent_falsify`
   - Add the same strict `belief_id` + scenario ownership guard.
   - If `RetractCascade` returns `retracted == 0`, report
     `verdict: "NO-OP"` rather than `"PASS"`.
   - Do not change the frozen kernel/CLI.

3. A-3 — I-7 Stage 2
   - Make the SQL-text tripwire case-insensitive and SQL-shaped so it
     catches lowercase:
       INSERT INTO
       UPDATE ... SET
       DELETE FROM
       CREATE TABLE/INDEX/DATABASE
       DROP TABLE/INDEX/DATABASE
   - Preserve legitimate SELECTs in `internal/view`.
   - Re-run all existing I-7 injections plus lowercase UPDATE and the
     other adversarial cases.
   - Do not turn this into a full SQL parser.

4. A-4 — receipt wording
   - Correct `plans/mcp/plan6_imp.md` so its injection count matches the
     actual Plan 6 table/results. Do not fabricate receipts.

STRICT SCOPE:
- No schema changes.
- No kernel changes.
- No new MCP tools.
- No architecture changes.
- No changes to `FullDebt`, `RetractCascade`, `belief_edge`, or
  `source_observed_at`.

VERIFY:
- `go build ./...`
- `go vet ./...`
- `gofmt -l cmd internal`
- `unset FABLE_TEST_DSN && task test` → exactly 107 PASS, 0 FAIL, 0 SKIP
- clean I-7 → PASS
- all malicious I-7 cases → FAIL, including lowercase SQL
- cross-scenario authorize → error + zero mutation
- cross-scenario falsify → error + zero mutation
- valid authorize/falsify paths still work
- existing demos still pass
- frozen-core diff remains empty
- no generated binary/artifact

Return a concise report with:
1. files changed,
2. exact fixes,
3. verification results,
4. frozen-core confirmation,
5. any remaining issue.

If any requested fix requires touching the frozen core, STOP and report
instead of doing it.