# Solvent MCP — Final Adversarial Code Review (Plan 6 Hardening Pass)

**Reviewer:** Claude (independent, adversarial)
**Date:** 2026-08-11
**Repository HEAD:** `33f9abe ✨ playground`
**Under review:** the plan6 hardening pass for F-1, F-2, F-3 from `plans/mcp/plan5_review.md`
**Claims (untrusted):** `plans/mcp/plan6_imp.md`
**Method:** every claim re-derived by execution against the current repository. Nothing below is
sourced from `plan6_imp.md`.

---

# Verdict

## APPROVED WITH MINOR CORRECTIONS

**Everything plan6 set out to fix is genuinely fixed and independently verified.** The full I-7
contract is discharged — clean PASSes, all six malicious injections FAIL, both missing directories
FAIL, and `internal/view`'s legitimate SELECTs still pass. F-2 and F-3 are correct and verified live
against the database. The regression suite is clean: 107 tests, four demos, empty frozen-core diff.

Two findings keep this short of unqualified approval, and **neither is in the code plan6 changed** —
both are in the two mutating tools the hardening pass did not scope. The more serious one (**A-1**)
lets the MCP surface create a cross-scenario intent that then makes a later legitimate falsification
fail permanently, with no MCP-side recovery. It is not on the scripted demo path, but it is precisely
the shape of mistake a free-roaming LLM agent makes — which is the exact test this review is meant to
clear the way for.

The fix is six lines, symmetric with what plan6 already did twice.

---

# Findings

---

## A-1 — `solvent_authorize_action` can create a cross-scenario intent that permanently wedges falsification

- **Severity:** MEDIUM
- **Evidence:** `cmd/solvent-mcp/tools.go:184-211`. Line 186 is still the lax
  `beliefID, _ := args["belief_id"].(string)`, and unlike `handleSolventPromote` (`:102-118`) and
  `handleSolventRetireDebt` (`:149-…`) there is **no ownership guard** before the kernel call.
  `kernel.Store.IntentOnPromoted` (`internal/kernel/kernel.go:104`) writes `scenario_id` from the
  argument, while the composite FK `gate` (`db/001_schema.sql:63`) constrains only
  `belief(id, status)` — **not** scenario.

- **Reproduction** (all through the MCP protocol):

```
# 1. Legitimately promote a track1 belief
solvent_retire_debt ×3 {scenario:"track1", belief_id:"1123a67d-…"}
solvent_promote        {scenario:"track1", belief_id:"1123a67d-…"}     → promoted

# 2. Authorize it under the WRONG scenario — this SUCCEEDS
solvent_authorize_action {scenario:"track2", belief_id:"1123a67d-…", action:"deploy from the WRONG scenario"}
  → isError: none, intent_state: "live"

SELECT a.scenario_id, b.scenario_id, a.state FROM action_intent a JOIN belief b ON b.id=a.belief_id …
  intent_scenario                       belief_scenario                       state
  00000000-0000-0000-0000-000000000002  00000000-0000-0000-0000-000000000001  live
  ^ a live intent in track2 citing a track1 belief

# 3. Now try to falsify that belief in ITS OWN scenario — permanently blocked
solvent_falsify {scenario:"track1", belief_id:"1123a67d-…"}
  → isError: true
    {"sqlstate":"23514","constraint":"live_requires_promoted",
     "sentinel":"ERROR: failed to satisfy CHECK constraint ((state != 'live') OR (belief_status = 'promoted'))"}
  belief status after: promoted        ← retraction refused
```

- **Impact:** The schema behaves correctly throughout — I-4 refuses rather than corrupts, which is
  the system working as designed. The defect is that the **MCP layer allowed the inconsistent intent
  to exist**. Once it does, `RetractCascade` is scenario-scoped (D-032) so a track1 cascade never
  sees the track2 intent, `ON UPDATE CASCADE` propagates the new status into it, and
  `live_requires_promoted` detonates. The belief becomes un-retractable, and **the MCP surface cannot
  recover** — there is deliberately no `cancel_intent` tool. Recovery requires the CLI or raw SQL.

  This is not reachable by the scripted Track 2 demo, which reads every UUID from
  `solvent_ledger(track2)`. It is very reachable by an agent exploring six tools across two
  scenarios — the exact usage the fresh-machine test will exercise.

  **This also corrects my own prior review.** `plans/mcp/plan5_review.md` stated that
  `solvent_authorize_action` and `solvent_falsify` "need no added guard: `IntentOnPromoted` and
  `RetractCascade` already take `scenarioID` and are scenario-scoped in the kernel." That is true of
  `RetractCascade` and **false of `IntentOnPromoted`**, which accepts the scenario as data rather
  than validating it. Plan6 reasonably scoped its work to my F-2, which named only promote and
  retire_debt; the gap is mine, not the implementer's.

- **Exact fix** — apply the identical guard already proven in `handleSolventPromote`:

```go
// cmd/solvent-mcp/tools.go, in handleSolventAuthorizeAction (replacing line 186)
beliefID, ok := args["belief_id"].(string)
if !ok || beliefID == "" {
    return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
}
// … after scenarioID resolution, before the kernel call:
snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

This remains identity-only — it never inspects debt, status, or intent state, so the database stays
the authority and M2 continues to come from the FK.

---

## A-2 — `solvent_falsify` reports `verdict: "PASS"` for a no-op, and is likewise unguarded

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/tools.go:213-239`. Line 215 is the lax assertion; line 236 hardcodes
  `"verdict": "PASS"` regardless of the retracted count. The frozen CLI distinguishes the case
  explicitly (`cmd/solvent/main.go:202-208`):

```go
} else if retracted == 0 {
    b.WriteString("VERDICT: NO-OP\n")
```

- **Reproduction:**

```
solvent_falsify {scenario:"track2", belief_id:"<a track1 belief>"}
  → isError: none
    {"audit":{"live_on_nonpromoted":0},
     "result":{"belief_id":"1123a67d-…","retracted":0,"verdict":"PASS"}}
  track1 belief status after: promoted     ← nothing happened
```

- **Impact:** `RetractCascade` is scenario-scoped, so a mismatched pair is a safe no-op — no
  corruption. But the tool reports success for work it did not do. Under the project's own grounding
  rule the agent narrates from the latest tool result, so it would say the belief was retracted while
  the ledger still shows `promoted`. That is the same class of misleading-output defect this project
  has been catching since the CLI review, and the frozen CLI already models the correct answer.
- **Exact fix:** add the same strict-argument + ownership guard as A-1, and mirror the CLI's verdict
  logic:

```go
verdict := "PASS"
if retracted == 0 {
    verdict = "NO-OP"
}
```

---

## A-3 — I-7 Stage 2 is case-sensitive, so lowercase SQL on the query path evades it

- **Severity:** LOW
- **Evidence:** `Taskfile.yml` Stage 2 —
  `grep -rnE --include='*.go' '(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]'`. No `-i`.
  `Query`/`QueryContext`/`QueryRowContext` are deliberately absent from Stage 1 so `internal/view`'s
  SELECTs pass, which makes Stage 2 the **only** guard on the query path.

- **Reproduction** (scratch copies; repository untouched):

| Injected into `internal/view/view.go` | Gate |
|---|---|
| `db.QueryRowContext(ctx, "update belief set status='promoted' where id=$1 returning id")` | **PASS** ✗ |
| `` const s = `UPDATE `` ⏎ `` belief SET …` `` + `QueryRowContext` | **PASS** ✗ |
| `"UPD" + "ATE belief SET …"` + `QueryRowContext` | **PASS** ✗ |

- **Impact:** No effect on shipped behavior — `internal/view` contains four `SELECT`s and nothing
  else, and every form the plan6 contract specified is correctly caught. The lowercase case is the
  one worth acting on because it is **not** deliberate evasion: SQL is case-insensitive and plenty of
  Go code writes lowercase keywords. The other two are inherent limits of any text-matching gate and
  I would not chase them.
- **Exact fix** — make Stage 2 case-insensitive but SQL-shaped, so prose comments like `// Create a
  new snapshot` do not false-FAIL:

```sh
if grep -rniE --include='*.go' '(INSERT[[:space:]]+INTO|UPDATE[[:space:]]+[a-z_]+[[:space:]]+SET|DELETE[[:space:]]+FROM|CREATE[[:space:]]+(TABLE|INDEX|DATABASE)|DROP[[:space:]]+(TABLE|INDEX|DATABASE))' cmd/solvent-mcp internal/view; then
```

Re-run all six specified injections plus the three above against any replacement before trusting it.

---

## A-4 — Receipt precision

- **Severity:** NIT
- `plan6_imp.md:47` claims "I-7 6 adversarial injections | all FAIL (correctly)". Reproduced — but
  `plan6.md`'s own table numbers row 1 as *"Clean repository → PASS"*, so it lists five malicious
  injections, not six. I tested six malicious forms (adding `db.Exec` + `INSERT`); all FAIL. Harmless
  off-by-one in wording.
- Every other row of the `plan6_imp.md` verification table reproduced exactly.

---

# I-7 Adversarial Verification

The full contract, discharged.

| Case | Result | Required |
|---|---|---|
| Clean repository | `I-7 MCP boundary: PASS` (exit 0) | PASS ✓ |
| **`internal/view` legitimate SELECTs** (`view.go:61,71,93,114` — one `QueryRowContext`, three `QueryContext`) | PASS | PASS ✓ |
| 1. `db.Exec` + `INSERT INTO` | `FAIL: direct write/transaction call` | FAIL ✓ |
| 2. `db.ExecContext` + `UPDATE` (single line) | `FAIL: direct write/transaction call` | FAIL ✓ |
| 3. `tx.ExecContext` | `FAIL: direct write/transaction call` | FAIL ✓ |
| 4. gofmt-wrapped `ExecContext` | `FAIL: direct write/transaction call` | FAIL ✓ |
| 5. `QueryRowContext` + `UPDATE … RETURNING` | `FAIL: write SQL text` | FAIL ✓ |
| 6. const-hoisted `UPDATE` + `ExecContext` | `FAIL: direct write/transaction call` | FAIL ✓ |
| Missing `cmd/solvent-mcp` | `FAIL: cmd/solvent-mcp missing` (exit 1) | FAIL ✓ |
| Missing `internal/view` | `FAIL: internal/view missing` (exit 1) | FAIL ✓ |

**No silent PASS on grep error.** The directory loop runs before any grep, so the exit-2 hole that
existed two revisions ago cannot recur. Note the division of labour the results reveal: injections
1–4 and 6 are caught by **Stage 1**; injection 5 is caught **only by Stage 2** — which is why A-3
matters and why Stage 2 deserves the case-insensitive hardening.

This is a genuine, verified fix of F-1. Two revisions ago the gate could not fail at all; one
revision ago it missed the codebase's own const-hoisted idiom; it now catches every form specified.

---

# F-2 Verification

**Strict argument handling** — `solvent_promote`, all through the protocol:

| Argument | Result |
|---|---|
| `{scenario:"track2"}` (absent) | `isError: true` — `belief_id is required and must be a string` |
| `{belief_id: ""}` | same clean argument error |
| `{belief_id: 12345}` (non-string) | same clean argument error |
| `{belief_id: null}` | same clean argument error |
| `{belief_id: "not-a-uuid"}` | `belief not-a-uuid not found in scenario track2` — the ownership guard, not a raw driver error |

**Cross-scenario → zero mutation**, verified by SQL either side of the call:

```
track1 belief 1123a67d-…
BEFORE: status=entered debt={needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

solvent_promote     {scenario:"track2", belief_id:"1123a67d-…"} → isError, "not found in scenario track2"
solvent_retire_debt {scenario:"track2", belief_id:"1123a67d-…", debt_item:"needToyCheck"} → same

AFTER : status=entered debt={needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}
>>> ZERO MUTATION
```

**Valid same-scenario mutation still works:** `solvent_retire_debt {scenario:"track1", …}` succeeds
and the debt array drops to three items.

**No business-rule duplication.** Reading `handleSolventPromote` and `handleSolventRetireDebt` end to
end, the guard is `len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID` — identity only. Nothing
inspects `debt`, `status`, or intent `state`, and M1/M2 still originate in CockroachDB (below).

---

# F-3 Verification

`solvent_ledger` against track1, which holds three beliefs:

| Argument | Beliefs returned | Correct |
|---|---|---|
| `{scenario:"track1"}` — absent | **3** (scenario-wide) | ✓ |
| `{scenario:"track1", belief_id: 12345}` | `isError: true` | ✓ |
| `{scenario:"track1", belief_id: ""}` | `isError: true` | ✓ |
| `{scenario:"track1", belief_id:"1123a67d-…"}` | **1** (scoped) | ✓ |

The silent widening from plan5's F-3 is gone.

---

# MCP Verification

Client: hand-driven JSON-RPC stdio client against `bin/solvent-mcp`
(`initialize` → `notifications/initialized` → `tools/list` → `tools/call`), negotiating
`protocolVersion 2025-06-18`, `serverInfo {solvent, v0.1.0}`. **Six tools, no extras.**

## M1 / M2 — still genuine database refusals

The new strict-argument checks run *before* the kernel call, so I verified they did not convert the
refusals into MCP-side rejections. They did not:

```
solvent_promote          {scenario:"track2", belief_id:"4c795b59-…"}
  isError=true  sqlstate=23514  constraint=promoted_is_debt_free
solvent_authorize_action {scenario:"track2", belief_id:"4c795b59-…", action:"deploy etcd v3.5.0"}
  isError=true  sqlstate=23503  constraint=gate
```

Both carry CockroachDB's own text — the rendered CHECK expression with `:::STRING` annotations, and
the FK violation naming `action_intent` / `gate`.

## Full Track 2 lifecycle

Fresh `task mcp:seed`; every step via MCP, every assertion re-queried in SQL.

| Stage | Verified |
|---|---|
| Post-seed | 1 belief `postulated`/`entered`/**6 debts**, 1 evidence, 0 intents |
| M1 / M2 | `23514` / `23503` |
| Retire ×6 → promote → authorize | debt → `[]`, `promoted`, intent `live` |
| Post-ingest | beliefs **3**, evidence **3**, distinct sha **3**, live intents **1**, baseline `promoted`, `belief_edge` **0** |
| UUID stability | seeded `4c795b59-…` == ledger `4c795b59-…` |
| Falsify | `{"retracted":1,"verdict":"PASS"}` |
| Final | baseline `retracted`, intent `cancelled`, other two `entered`, audit **0**, `belief_edge` **0** |

---

# Regression

| Check | Result |
|---|---|
| `task --list` | **11 tasks**, parses cleanly |
| `go build ./...` | CLEAN |
| `go vet ./...` | CLEAN |
| `gofmt -l cmd internal` | CLEAN (no output) |
| `unset FABLE_TEST_DSN && task test` | **exit 0** |
| **Exact test count** | **107 PASS, 0 FAIL, 0 SKIP** |
| Kernel I-7 | `I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read` |
| MCP I-7 (in `task test`) | `I-7 MCP boundary: PASS` |
| `task demo:track1 / demo:replay / demo:track2 / inspect` | all exit 0 |
| `belief_edge` | **0** |
| Frozen-core diff (nine paths) | **empty** |
| `git status` | `.gitignore README.md Taskfile.yml docs/M2_TRANSCRIPT.md go.mod go.sum` modified; `.mcp.json cmd/solvent-mcp/ internal/view/ plans/mcp/` untracked — **no stray binary** |
| `task mcp:build` / `task mcp:seed` | both succeed; seed runs with `SOLVENT_DSN`/`FABLE_DSN` unset |

**Repository integrity after my injections** — all performed on scratch copies:

```
$ md5sum -c b6.md5
cmd/solvent-mcp/errors.go: OK   cmd/solvent-mcp/main.go: OK   cmd/solvent-mcp/tools.go: OK
internal/view/view.go: OK       Taskfile.yml: OK
```

---

# Receipt Integrity

| `plan6_imp.md` claim | My execution | Verdict |
|---|---|---|
| `task --list` — 11 tasks, clean YAML | 11 tasks, parses | **Reproduces** |
| `go build ./...` PASS | CLEAN | **Reproduces** |
| `go vet ./...` PASS | CLEAN | **Reproduces** |
| `gofmt -l cmd internal` no output | no output | **Reproduces** |
| I-7 clean repository PASS | PASS | **Reproduces** |
| I-7 six injections all FAIL | six malicious forms tested, all FAIL | **Reproduces** (wording nit, A-4) |
| I-7 two missing directories FAIL | both FAIL | **Reproduces** |
| `task test` — 107 PASS, 0 FAIL, 0 SKIP | exactly that | **Reproduces** — F-4 correction landed |
| Frozen core CLEAN | empty diff | **Reproduces** |
| `git status` no stray binary | confirmed | **Reproduces** |
| F-2: "Both handlers reject missing/non-string/empty and assert identity" | verified live | **Reproduces** |
| F-3: absent vs present-but-invalid | verified live | **Reproduces** |
| F-1: "Legitimate `QueryRowContext` SELECTs pass because they contain SELECT, not write SQL" | verified — clean repo passes with four SELECT call sites | **Reproduces** |

**No stale receipts and no fabricated output.** `plan6_imp.md` pastes no transcripts and every claim
it makes is reproducible. This is the third consecutive implementation summary that is scope-honest.

---

# Hackathon Judge Risk

| Rank | Risk | Consequence |
|---|---|---|
| **Medium** | A-1 — an agent (or judge) passing a mismatched `scenario` + `belief_id` to `authorize_action` creates a stray live intent that permanently blocks falsifying that belief, with no MCP-side recovery | The demo's climax — retraction cancelling the live intent — would fail with `23514` on the affected belief. Requires cross-scenario improvisation; the scripted path is unaffected. |
| **Low** | A-2 — `falsify` says `PASS` for a no-op | The agent narrates a retraction that did not happen. Only reachable with a mismatched scenario. |
| **Low** | A-3 — lowercase SQL evades I-7 Stage 2 | No effect on current behavior; weakens future-change detection on the query path only. |
| **Low** | Agent-grounding behavior unmeasured (below) | Live narration quality still unknown. |
| — | Setup fragility, stale paths, missing env vars, task failures, stale transcripts, generated artifacts | **None found.** |

---

## Client testing — stated precisely

**Tested:** a hand-driven JSON-RPC stdio client against `bin/solvent-mcp` — initialization, tool
discovery, schemas, ~40 tool invocations including adversarial and cross-scenario input, `IsError`
handling, structured result parsing.

**NOT tested:** VS Code, Codex, or Claude Code as a GUI client. Neither the README nor
`plan6_imp.md` claims otherwise.

**Agent-grounding rehearsal: NOT RUN.** Attaching `.mcp.json` to this Claude Code session requires a
restart, so no LLM agent drove the tools. This is the one thing the fresh-machine test adds that I
cannot substitute for — and it is also the condition under which A-1 is most likely to fire.

---

# Required Fixes

None blocking. In priority order:

1. **A-1** — add the strict-argument + identity guard to `handleSolventAuthorizeAction`
   (`tools.go:184`), identical to `handleSolventPromote`. **Recommended before the fresh-machine test
   with a real agent**, since free-roaming tool use is exactly the trigger.
2. **A-2** — add the same guard to `handleSolventFalsify` (`tools.go:213`) and mirror the CLI's
   `NO-OP` verdict when `retracted == 0` (`tools.go:236`).
3. **A-3** — make I-7 Stage 2 case-insensitive and SQL-shaped; re-run all nine injections against
   the replacement.
4. **A-4** — align `plan6_imp.md`'s "6 injections" wording with `plan6.md`'s table.

Items 1 and 2 are the same six-line pattern plan6 already applied twice, in the two handlers it did
not scope.

---

# Most Important Question

> **Is the MCP implementation now ready for the final fresh-machine test with a real LLM coding agent?**

## Yes — after A-1.

Everything the fresh-machine test depends on is verified working from the current repository:
`task setup` through `task mcp:build`, `task mcp:seed`, six tools over a real protocol handshake,
M1 refused by CockroachDB with `23514`/`promoted_is_debt_free`, M2 refused with `23503`/`gate`, the
full Track 2 lifecycle ending in a retracted belief and a cancelled intent, audit 0, `belief_edge` 0,
107 tests green, four demos green, frozen core untouched. The I-7 gate — inert two revisions ago —
now catches every specified form of raw write and fails closed on a missing directory.

The one thing I would fix first is A-1, and specifically *because* the next test involves a real
agent. The scripted demo never passes a mismatched scenario; an LLM handed six tools and two
scenarios plausibly will. When it does, the result is not a clean error — it is a belief that can
never be falsified through the MCP surface, which would take out the demo's closing moment. The fix
is six lines already written twice elsewhere in the same file.

With that in place, the honest summary of this implementation stands: **the agent is not trusted, MCP
is not trusted, the tool description is not trusted, and the database is the final authority** — and
unlike two revisions ago, there is now a working mechanical gate that will notice if that ever stops
being true.
