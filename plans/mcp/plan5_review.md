# Solvent — Adversarial Code Review of the Plan 5 Fix Pass

**Reviewer:** Claude (independent, adversarial)
**Date:** 2026-08-11
**Repository HEAD:** `33f9abe ✨ playground`
**Under review:** the fix pass for R-1…R-7 from `plans/mcp/imp_review.md`
**Claims (untrusted):** `plans/mcp/plan5_imp.md`
**Method:** every claim re-derived by execution against the current repository. Nothing below is
sourced from `plan5_imp.md`.

---

# Verdict

## APPROVED WITH MINOR CORRECTIONS

**All seven findings are genuinely closed.** I reproduced every one independently, including the two
that produced the previous REWORK verdict: `Taskfile.yml` now parses and all 11 tasks run, and the
I-7 gate now correctly FAILs on an injected raw write and on a missing directory.

One residual issue keeps this short of unqualified approval: **the repaired I-7 gate still misses
three plausible forms of the very write class it exists to prohibit** — including the codebase's own
kernel idiom. That is a hardening gap in a future-change detector, not a defect in shipped behavior,
and it does not affect the demo. `REWORK REQUIRED` would be manufacturing a blocker.

---

# Executive Summary

- **R-1 closed.** `task --list` succeeds; 11 tasks including `mcp:build` and `mcp:seed`. The
  previously fatal echo line executes correctly.
- **R-2 closed, with a caveat.** All three mandated cases discharged: clean → **PASS**, injected raw
  write → **FAIL**, missing directory → **FAIL** (both directories tested). But three bypasses
  remain — see F-1.
- **R-3 closed and verified live.** Cross-scenario `promote` and `retire_debt` now return a clean
  domain error with **zero mutation**, confirmed by SQL before/after. Same-scenario mutation still
  works.
- **R-4 closed.** `envelopeErrorResult` takes `ctx`, propagates the audit error as
  `"audit": null` + `"audit_error"`, and never fabricates 0. Both call sites pass `ctx` — I
  enumerated all of them rather than trusting the count.
- **R-5 closed.** README has a complete MCP section; all 8 documented commands resolve to real
  tasks; no claim that any client was tested.
- **R-6 closed.** `/solvent-mcp` and `/bin/` in `.gitignore`; stray binary gone; `bin/solvent-mcp`
  confirmed ignored after `task mcp:build`.
- **R-7 closed.** `imp.md`'s `AddTool` signature, fixture-fallback rationale, and removal of the
  VS Code/Codex claim all match the code — I confirmed the fallback really does resolve to a
  nonexistent `bin/internal/...` path and fails closed.
- **Frozen core untouched**; **exactly 107 tests** pass (`plan5_imp.md`'s "107+" is imprecise, not
  wrong); all four demos pass via `task`; `belief_edge` = 0 throughout.
- **M1 and M2 remain genuine database refusals.** Critically, the new ownership guard did **not**
  convert them into MCP-side refusals — the guard passes, the kernel is called, and CockroachDB
  refuses.

---

# Findings

Ordered by consequence.

---

## F-1 — The repaired I-7 gate misses three plausible forms of the write class it prohibits

- **Severity:** MEDIUM
- **Evidence:** `Taskfile.yml:82`

```sh
if grep -rnE --include='*.go' '(ExecContext|Exec)\(.*(INSERT|UPDATE|DELETE|CREATE|DROP)' cmd/solvent-mcp internal/view; then
```

The pattern requires the call **and** the SQL keyword on the *same physical line*. Three forms of the
same write class evade it.

- **Reproduction** (scratch copies; repository never modified):

| Injected form | Gate verdict | Should be |
|---|---|---|
| `db.ExecContext(ctx, "UPDATE belief SET status='promoted' …")` — single line | **FAIL** ✓ | FAIL |
| `tx.ExecContext(ctx, "UPDATE …")` — single line | **FAIL** ✓ | FAIL |
| `db.ExecContext(ctx,`⏎`  "UPDATE belief SET status='promoted' …")` — gofmt-wrapped | **PASS** ✗ | FAIL |
| `db.QueryRowContext(ctx, "UPDATE … RETURNING id")` | **PASS** ✗ | FAIL |
| `const sqlBackdoor = \`UPDATE …\`` + `db.ExecContext(ctx, sqlBackdoor, id)` | **PASS** ✗ | FAIL |

The third bypass is the most serious, because **it is the project's own house style.**
`internal/kernel/sql.go:14-25` defines every statement as a hoisted const —
`sqlAddEvidence`, `sqlRetireDebt`, `sqlPromote` — invoked as
`tx.ExecContext(ctx, sqlPromote, beliefID)` (`internal/kernel/kernel.go:94`). A developer copying the
kernel's own idiom into `internal/view` would produce a raw write that this gate cannot see.

The first bypass matters nearly as much: gofmt wraps long queries onto their own line as a matter of
course, so the *natural* way to write a raw query is the way the gate misses.

- **Impact:** No effect on shipped behavior — the current sources contain no raw writes, which I
  verified by reading `internal/view/view.go` (four `SELECT`s, `view.go:61,71,93,114`) and confirming
  `grep -rn "db.Exec\|db.Query" cmd/solvent-mcp` returns nothing. The impact is on *future* changes:
  the gate's job is to make the boundary claim mechanically enforced, and §6 of the review prompt
  sets the bar explicitly — *"it must actually catch the class of write it claims to prohibit."*
  Three of five tested forms of that class pass.
- **Required fix:** Match the *call*, not the call-plus-SQL on one line. `internal/view` is
  SELECT-only and `cmd/solvent-mcp` should never touch the database directly, so forbidding the
  write/transaction entry points outright is both simpler and stricter:

```sh
if grep -rnE --include='*.go' '\.(Exec|ExecContext|Prepare|PrepareContext|Begin|BeginTx)\(' cmd/solvent-mcp internal/view; then
  echo "FAIL: direct write/transaction call in MCP/view"; exit 1
fi
if grep -rnE --include='*.go' '(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]+(INTO|TABLE|FROM|belief|evidence|action_intent)' cmd/solvent-mcp internal/view; then
  echo "FAIL: write SQL text in MCP/view"; exit 1
fi
```

Whichever form is adopted, **re-run the five injections above against it** before trusting it. The
lesson from R-2 is that a gate is only as good as its last adversarial test.

---

## F-2 — The cross-scenario guard tests emptiness, not identity

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/tools.go:107-110` and `:141-144`

```go
snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
if err != nil || len(snap.Beliefs) == 0 {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

When `beliefID` is `""` — which happens whenever `belief_id` is absent or not a JSON string, because
`args["belief_id"].(string)` discards the failed assertion (`tools.go:98,136`) — `GetSnapshot` takes
its unfiltered branch (`view.go:70-89`) and returns **every** belief in the scenario. `len(...) == 0`
is therefore false and the guard passes.

- **Reproduction:**

```
solvent_promote {"scenario":"track2"}                    → isError=true, "error in argument for $1: could not parse …"
solvent_promote {"scenario":"track2","belief_id":12345}  → isError=true, "error in argument for $1: could not parse …"
```

- **Impact:** Contained — the guard is bypassed but CockroachDB's `::UUID` cast rejects the empty
  string, so no mutation occurs and the caller gets an error either way. The defect is that the
  *guard* is not what stops it; the database is. That makes the guard weaker than its comment claims
  and leaves the outcome dependent on a cast rather than an explicit check.
- **Required fix:** Reject an empty/non-string `belief_id` before the lookup, and assert identity
  rather than non-emptiness:

```go
beliefID, ok := args["belief_id"].(string)
if !ok || beliefID == "" {
    return errorResult(fmt.Errorf("belief_id is required and must be a string")), nil
}
// …
if err != nil || len(snap.Beliefs) != 1 || snap.Beliefs[0].ID != beliefID {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

---

## F-3 — `solvent_ledger` silently widens on a non-string `belief_id` (pre-existing, still open)

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/tools.go:24` — `beliefID, _ := args["belief_id"].(string)`.
- **Reproduction** (track1 holds 3 beliefs):

```
solvent_ledger {"scenario":"track1"}                     → 3 beliefs
solvent_ledger {"scenario":"track1","belief_id":12345}   → 3 beliefs   ← silently widened
solvent_ledger {"scenario":"track1","belief_id":"efa952cb-…"} → 1 belief
```

- **Impact:** A client asking about one belief receives the whole scenario with no error. Under the
  project's own grounding rule — the agent must assert only what the latest tool result contains —
  a silently widened query is exactly the input that produces a confident, wrong narration. This was
  R-7 in my previous review; it was not in plan5's stated scope, so this is a carry-forward, not a
  regression.
- **Required fix:** Same treatment as F-2 — distinguish "absent" from "present but wrong type" and
  return a validation error for the latter.

---

## F-4 — `plan5_imp.md` records "107+ tests passing"

- **Severity:** NIT (documentation)
- **Evidence:** `plans/mcp/plan5_imp.md:42`.
- **Reproduction:** `unset FABLE_TEST_DSN && go test -count=1 -p 1 -v ./...` → **107 PASS, 0 FAIL,
  0 SKIP** — unchanged from the pre-fix baseline.
- **Impact:** Harmless wording, but "107+" is unfalsifiable in a project whose review history turns
  on exact receipts. The precise figure is 107, and it did not drift.
- **Required fix:** State the exact number.

---

## F-5 — Minor residue

- **Severity:** NIT
- `cmd/solvent-mcp/tools.go:247` — `envelopeResult(db *sql.DB, …)` still never uses `db`.
- The `audit_error` branch (`tools.go:263-265`) is correct but **unreachable through the tool
  surface**: `scenarioID` always comes from the server-owned `scenarioToID` map, so `AuditIntent`
  cannot receive a malformed scenario. I verified the code path by reading it; I could not exercise
  it without editing the repository, and I did not. Reported as verified-by-inspection.
- `cmd/solvent-mcp` and `internal/view` have **no test files**. Neither plan3 nor plan4.1 required
  them, so this is an observation, not a finding — but the MCP layer's guarantees currently rest
  entirely on the I-7 gate (F-1) and manual review.

---

# Frozen Core Verification

**PASS — untouched.**

```
$ git diff --stat HEAD -- db/ internal/kernel internal/belief internal/intent \
    internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review
(no output)
```

Working tree:

```
 M .gitignore   M README.md   M Taskfile.yml   M docs/M2_TRANSCRIPT.md   M go.mod   M go.sum
?? .mcp.json  ?? cmd/solvent-mcp/  ?? internal/view/  ?? plans/mcp/
```

Every modified file is in the approved change set. `docs/M2_TRANSCRIPT.md` is a test-run artifact.
The stray root binary is gone; `git status` shows no generated artifact (the `solvent-mcp` match is
`?? cmd/solvent-mcp/`, the source directory).

`FullDebt`, `RetractCascade`, `belief_edge`, and `source_observed_at` persistence are all unchanged —
they sit inside the empty diff above. `scripts/check_i7.sh` run standalone:
`I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read (audit)`.
`belief_edge` = **0** after the full demo and MCP lifecycle runs.

**Repository integrity after my adversarial injections** — all injections were performed on copies
under the scratchpad:

```
$ md5sum -c baseline.md5
cmd/solvent-mcp/errors.go: OK
cmd/solvent-mcp/main.go: OK
cmd/solvent-mcp/tools.go: OK
internal/view/view.go: OK
Taskfile.yml: OK
```

---

# I-7 Adversarial Verification

All three mandated cases discharged.

| Case | Result | Required |
|---|---|---|
| **Clean repository** | `I-7 MCP boundary: PASS` (exit 0) | PASS ✓ |
| **Injected raw write** — `db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")` | `internal/view/view.go:167: …` then `FAIL: raw writes in MCP/view` (exit 1) | FAIL ✓ |
| **Missing `cmd/solvent-mcp`** | `FAIL: cmd/solvent-mcp missing` (exit 1) | FAIL ✓ |
| **Missing `internal/view`** | `FAIL: internal/view missing` (exit 1) | FAIL ✓ |

The R-2 defect is genuinely fixed: the line-nullifying `grep -v '^…'` filter is gone, the directory
existence loop closes the grep-exit-2 hole, and `--include='*.go'` scopes the search to Go source.
The gate is also wired into `task test` and ran during my full-suite execution
(`I-7 MCP boundary: PASS`).

**Beyond the mandated cases**, the bypass hunt in F-1 found that gofmt-wrapped calls,
`QueryRowContext` + `RETURNING`, and const-hoisted SQL all still pass. Legitimate `SELECT` reads
correctly continue to pass. The gate went from *catching nothing* to *catching most obvious forms* —
a real improvement, but short of §6's stated bar.

---

# MCP Verification

Client: hand-driven JSON-RPC stdio client against `bin/solvent-mcp`
(`initialize` → `notifications/initialized` → `tools/list` → `tools/call`).
Negotiated `protocolVersion 2025-06-18`, `serverInfo {name: solvent, version: v0.1.0}`.

**Six tools, no extras:** `solvent_authorize_action`, `solvent_falsify`, `solvent_ingest_evidence`,
`solvent_ledger`, `solvent_promote`, `solvent_retire_debt`.

## M1 — promotion refused

```
→ solvent_promote {"scenario":"track2","belief_id":"ec84ac97-9a95-4b08-9507-fb0a931e6989"}
jsonrpc error: None          ← tool result, not a protocol failure
isError: True
{"audit":{"live_on_nonpromoted":0},
 "result":{"constraint":"promoted_is_debt_free","error":true,"sqlstate":"23514",
   "sentinel":"promotion blocked: open debt or final-truth language: ERROR: failed to satisfy
     CHECK constraint ((status != 'promoted':::STRING) OR …)"}}
```

**Genuine database refusal.** The sentinel embeds CockroachDB's own rendering of the CHECK
expression, including `:::STRING` type annotations — text no MCP-side check could produce.
Importantly, the FIX 3 ownership guard runs *before* the kernel call and **passed** here (the belief
does belong to track2), so the refusal still originates in the database, not in the new guard.

## M2 — authorization refused

```
→ solvent_authorize_action {"scenario":"track2","belief_id":"ec84ac97-…","action":"deploy etcd v3.5.0"}
isError: True
{"audit":{"live_on_nonpromoted":0},
 "result":{"constraint":"gate","error":true,"sqlstate":"23503",
   "sentinel":"action refused: belief is not promoted: ERROR: insert on table \"action_intent\"
     violates foreign key constraint \"gate\" (SQLSTATE 23503)"}}
```

CockroachDB's own FK violation text, naming the table and constraint.

## Cross-scenario guard (FIX 3), live

track1 belief `efa952cb-3ac4-43ce-90cd-62144d07607a`, addressed as track2:

| Call | Result | Mutation |
|---|---|---|
| `solvent_retire_debt {scenario:"track2", belief_id:<track1>, debt_item:"needToyCheck"}` | `isError:true` — `belief efa952cb-… not found in scenario track2` | debt **unchanged** `{needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}` |
| `solvent_promote {scenario:"track2", belief_id:<track1>}` | `isError:true` — same clean error | status **unchanged** `entered` |
| `solvent_retire_debt {scenario:"track1", …}` (valid) | success, debt drops to 3 items | correct mutation |

The previous review's two defects — a silent cross-scenario promotion, and a `retire_debt` that
mutated *and* reported an error — are both gone. `solvent_authorize_action` and `solvent_falsify`
need no added guard: `IntentOnPromoted` and `RetractCascade` already take `scenarioID` and are
scenario-scoped in the kernel.

## Track 2 lifecycle, end to end

Fresh `task mcp:seed`, baseline `6d530345-fddb-49ac-921f-02bce1d5551a`, every step via MCP and every
state assertion re-queried in SQL.

| Stage | Verified state |
|---|---|
| Post-seed | 1 belief `postulated`/`entered`/**6 debts**, 1 evidence, 0 intents |
| M1 / M2 | refused `23514` / `23503` |
| Retire ×6 | debt drains to `[]` |
| Promote | `status: promoted` |
| Authorize | `intent_state: live`, action `deploy etcd v3.5.0` |
| Ingest track2 | + `accommodated` (4 debts), + `derived` (6 debts) |
| **Post-ingest** | beliefs **3**, evidence **3**, **distinct sha 3**, live intents **1**, baseline rows **1**, baseline `promoted`, `belief_edge` **0** |
| UUID stability | seeded `6d530345-…` == ledger `6d530345-…` |
| Falsify | `{"retracted":1,"verdict":"PASS"}`, audit 0 |
| **Final** | baseline `retracted`, intent `cancelled`, other two `entered`, audit **0**, `belief_edge` **0** |

---

# Test / Build Results

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **CLEAN** |
| Vet | `go vet ./...` | **CLEAN** |
| Format | `gofmt -l cmd internal` | **CLEAN** (no output) |
| Full suite | `unset FABLE_TEST_DSN && task test` | **exit 0** |
| **Exact test count** | `go test -count=1 -p 1 -v ./...` | **107 PASS, 0 FAIL, 0 SKIP** (97 top-level + subtests) |
| Kernel I-7 | `scripts/check_i7.sh` | PASS — 7 ExecuteTx sites, 0 raw writes |
| MCP I-7 | in-`task test` gate | `I-7 MCP boundary: PASS` |
| `task --list` | | **11 tasks**, parses cleanly |
| `task mcp:build` | | `bin/solvent-mcp`, 16.6 MB, git-ignored |
| `task mcp:seed` | with `SOLVENT_DSN`/`FABLE_DSN` **unset** | exit 0; the previously fatal echo line executes |

Per package: `belief` `derive` `intent` `kernel` `normalize` `pipeline` all `ok`;
`cmd/solvent-mcp` and `internal/view` report `[no test files]`.

**Suite self-containment:** the tests target `fable_test` via `testdb.DefaultDSN` and require only a
CockroachDB on 26260 — no MCP server, no MCP client. This is the pre-existing arrangement and is
unchanged by the fix pass.

**Frozen demos** — all four via `task`:

| Task | Exit | Result |
|---|---|---|
| `demo:track1` | 0 | `=== TRACK 1 COMPLETE ===` |
| `demo:replay` | 0 | `=== REPLAY COMPLETE ===` |
| `demo:track2` | 0 | `=== TRACK 2 COMPLETE ===` |
| `inspect` | 0 | `Live intents on non-promoted beliefs: 0 (SAFE)` |

`belief_edge` = 0 after all four.

---

# Receipt Integrity

`plan5_imp.md`'s verification table, checked line by line against my execution:

| Claim | `plan5_imp.md` | My execution | Verdict |
|---|---|---|---|
| `task --list` | "11 tasks, clean YAML" | 11 tasks, parses | **Reproduces** |
| `go build ./...` | PASS | CLEAN | **Reproduces** |
| `go vet ./...` | PASS | CLEAN | **Reproduces** |
| `gofmt` | PASS | CLEAN | **Reproduces** |
| I-7 gate | PASS | PASS clean · FAIL injected · FAIL missing-dir | **Reproduces** — and unlike last round, the PASS is now meaningful. Caveat in F-1. |
| Frozen core diff | CLEAN | empty | **Reproduces** |
| `task test` | "107+ tests passing" | exactly **107** | **Imprecise wording** (F-4), not a defect |

Per-fix claims:

| Fix | Claim | Verified |
|---|---|---|
| FIX 1 | `: ` → ` - ` in the seed echo | ✓ `Taskfile.yml:66`; recipe executes |
| FIX 2 | directory check + injection-proof gate | ✓ three mandated cases; gaps in F-1 |
| FIX 3 | ownership lookup before mutation in both handlers | ✓ verified live, no mutation |
| FIX 4 | `(ctx, db, err, scenarioID)`; propagates audit errors; **both** callers updated | ✓ exactly two callers, both pass `ctx` |
| FIX 5 | README MCP section | ✓ all topics present; all 8 documented commands resolve |
| FIX 6 | `/solvent-mcp` + `/bin/` ignored, stray binary deleted | ✓ `git check-ignore bin/solvent-mcp` → ignored |
| FIX 7 | VS Code/Codex claim removed; `AddTool` corrected; fallback rationale corrected | ✓ all three; fallback confirmed to resolve to `…/bin/internal/derive/testdata/etcd_real` and fail closed |

**No stale receipts, no fabricated output.** `plan5_imp.md` contains no pasted transcripts, and every
claim it makes is reproducible. The single imprecision is "107+".

**One nuance worth recording:** last round's `imp.md` also claimed "I-7 gate: PASS", and that claim
was true while the gate was inert. The claim is identical this round; only the underlying reality
changed. That is precisely why §22 requires reproducing the *behavior*, not the *label* — and why
F-1 recommends re-running the injections against any future edit to the gate.

---

# Hackathon Judge Risk

| Rank | Risk | Consequence |
|---|---|---|
| **Low** | Judge must run `task mcp:build` before an MCP client can launch (`.mcp.json` → `bin/solvent-mcp`) | Documented in the README MCP section; `task setup` does not build it. A judge who skips straight to client configuration hits a missing binary. Consider having `mcp:seed` depend on `mcp:build`. |
| **Low** | `.mcp.json` uses repo-relative `command` and `SOLVENT_FIXTURE_ROOT` | Works when the client launches with the repo as cwd (the normal project-scoped case). A client using a different cwd fails closed with a clear message. |
| **Low** | Agent-grounding behavior unverified (see below) | Live narration quality — invented counts or stale UUIDs — is still unmeasured. |
| **Low** | I-7 gate bypasses (F-1) | No demo impact. A technical judge reading the Taskfile could observe that the gate is weaker than it looks. |
| — | Setup fragility, stale paths, missing env vars, task failures, stale transcripts, generated artifacts | **None found.** `task mcp:seed` runs with `SOLVENT_DSN`/`FABLE_DSN` unset; all demos pass; no generated artifact is tracked or untracked. |

---

## Client testing — stated precisely

**Tested:** a hand-driven JSON-RPC stdio client against `bin/solvent-mcp` — initialization, tool
discovery, schema discovery, ~30 tool invocations including adversarial input, `IsError` handling,
and structured result parsing.

**NOT tested:** VS Code, Codex, or Claude Code as a GUI client. The README correctly makes no claim
that any of them were verified, and neither does this review.

**Agent-grounding rehearsal: NOT RUN.** Attaching `.mcp.json` to a live Claude Code session requires
a restart, so no LLM agent drove the tools. The shipped tool descriptions do instruct the agent to
read the ledger before asserting state, re-read after mutation, and obtain the UUID immediately
before falsification — but whether a model honors that is unmeasured. Procedure to close it:
`task mcp:build && task mcp:seed`, restart the client with `.mcp.json` approved, run the plan3 §8
script verbatim, and check every asserted count, status and UUID against the immediately preceding
tool result.

---

# Required Fixes

None are blocking. In priority order:

1. **F-1 — tighten the I-7 gate** so it catches gofmt-wrapped calls, the query-path write, and
   const-hoisted SQL (the kernel's own idiom). Validate the replacement against all five injections
   in this report before trusting it.
2. **F-2 — make the ownership guard an identity check**: reject empty/non-string `belief_id` up
   front, and compare `snap.Beliefs[0].ID` to the requested ID.
3. **F-3 — stop silently widening `solvent_ledger`** when `belief_id` is present but not a string.
4. **F-4 — record the exact test count (107)** in `plan5_imp.md` instead of "107+".
5. **F-5 — optional:** drop the unused `db` parameter from `envelopeResult`; consider a smoke test
   for `cmd/solvent-mcp` / `internal/view`.

---

# Final Scope Assessment

> **Is the Solvent MCP implementation now safe to hand to the final fresh-machine acceptance test?**

## Yes.

Every defect that made the previous verdict REWORK REQUIRED is closed and independently reproduced:

- `Taskfile.yml` parses; all 11 tasks resolve and execute, including the recipe that broke.
- The I-7 gate now fails on an injected raw write and on a missing directory — the exact cases it
  previously waved through.
- Cross-scenario mutation is blocked with a clean error and **zero writes**, verified by SQL
  before/after on both affected tools.
- The audit envelope no longer fabricates a safe-looking zero.
- The README documents the MCP demo, and every command in it exists.
- No generated binary is tracked or left untracked.
- The frozen core is untouched, 107 tests pass, all four demos pass, and `belief_edge` is 0.

And the demonstration itself is real. Driving the server over the wire, an agent asking to promote a
belief with open debt is refused by CockroachDB with `23514` / `promoted_is_debt_free`; asking to
authorize the deployment anyway is refused with `23503` / `gate`; and after legitimate debt clearance,
promotion, authorization, and ingestion of the real etcd postmortem, falsification retracts the belief
and **the database cancels the dependent live intent inside the same transaction** — MCP issued one
call and never touched `action_intent`.

The three residual findings are hardening, not correctness. F-1 weakens a future-change detector
without affecting current behavior; F-2 and F-3 are input-validation gaps whose worst outcome is an
error message from the database instead of from the adapter. None of them can break the
fresh-machine acceptance test.

**One thing to carry into that test:** the agent-grounding rehearsal is still NOT RUN. The acceptance
test is the natural place to close it, since it will already have a real client attached.
