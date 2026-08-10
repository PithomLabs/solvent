# Solvent MCP Implementation — Final Adversarial Code Review

**Reviewer:** Claude (independent, adversarial)
**Date:** 2026-08-11
**Repository HEAD:** `33f9abe ✨ playground`
**Specs:** `plans/mcp/plan3.md`, `plans/mcp/plan4.1.md`, `plans/mcp/plan4.1_eval.md`
**Claims under review (untrusted):** `plans/mcp/imp.md`
**Method:** every claim re-derived from the current repository by execution. Nothing below is
sourced from `imp.md`.

---

# 1. Verdict

## REWORK REQUIRED

**This is not an architectural verdict.** The MCP layer itself is correct, and the centerpiece
demonstration genuinely works — I drove it end to end through a real MCP protocol client and
CockroachDB refused the agent twice with live SQLSTATEs.

The verdict is forced by the prompt's own standard — *"Does it faithfully implement the approved
design and survive a real judge-facing demonstration?"* — and one fact: **`Taskfile.yml` does not
parse, so every documented command in the project currently fails**, including `task setup`, which is
the first line of the README quick start. A judge cloning this repository today cannot run anything.

The required work is two mechanical fixes totalling roughly four lines. See §12 and §13.

---

# 2. Executive Summary

- **`Taskfile.yml` is syntactically invalid.** All 8 tasks fail to parse — `setup`, `test`,
  `demo:track1/replay/track2`, `inspect`, `mcp:build`, `mcp:seed`. Root cause: an unquoted YAML
  scalar containing `: ` at `Taskfile.yml:67`. **BLOCKING.**
- **The I-7 MCP boundary gate is inert.** I injected `db.ExecContext(ctx, "UPDATE belief SET
  status='promoted' …")` into a scratch copy of `internal/view` and the gate still printed PASS. The
  only mechanical safeguard for the central architectural claim provides zero protection. **HIGH.**
- **The MCP layer itself is genuinely correct.** Zero semantic conditionals; all seven audit values
  come from `pipeline.AuditIntent`; zero direct `db.Exec`/`db.Query`; SDK imported only by
  `cmd/solvent-mcp`; kernel acquires no MCP dependency.
- **M1 and M2 are real database refusals.** The returned sentinel carries CockroachDB's raw CHECK
  expression and FK violation text — text the MCP layer could not fabricate. Receipts in §6 and §7.
- **The full Track 2 payoff works through MCP alone:** seed → 2 refusals → retire ×6 → promote →
  authorize → ingest → falsify → belief `retracted`, intent `cancelled`, audit 0, `belief_edge` 0.
- **Seed → ingest is verified, not assumed.** Baseline UUID stable, no duplicate baseline, 3 beliefs
  / 3 evidence / 3 distinct `content_sha256` / 1 live intent — confirmed by SQL against the MCP
  sequence, not inferred from the CLI demo.
- **Cross-scenario writes are unguarded.** `solvent_promote(scenario:"track2", belief_id:<track1
  belief>)` promoted the track1 belief and reported track2's audit. `solvent_retire_debt` in the same
  shape mutated the belief *and* returned `isError: true`. **MEDIUM.**
- **The README contains no MCP section at all.** Definition-of-Done item 15 is unmet; a judge
  following the README will never learn the MCP demo exists. **MEDIUM.**
- **`.mcp.json` points at `bin/solvent-mcp`, which is not built by default**, while a stray 16 MB
  `solvent-mcp` sits untracked at the repo root and is not covered by `.gitignore`. **MEDIUM.**
- **`imp.md` is scope-honest** — it claims only build/vet/gofmt/I-7/frozen-core, and makes no test or
  demo claims. Four of its five claims reproduce. The fifth ("I-7 … PASS") is technically true and
  substantively worthless, and could not have been obtained via `task test`. See §10.

---

# 3. Findings

---

## R-1 — `Taskfile.yml` does not parse; every task is dead

- **Severity:** BLOCKING
- **Evidence:** `Taskfile.yml:67`

```yaml
- echo "Seed complete. Post-seed state: 1 belief (postulated, entered, 6 debts), 1 evidence, 0 intents, audit 0."
```

The unquoted scalar contains `: ` (colon-space) after `Post-seed state`, so YAML parses the list item
as a **mapping**, not a string:

```
parsed as: {'echo "Seed complete. Post-seed state': '1 belief (postulated, entered, 6 debts), …'}
type     : dict
```

Task then rejects the whole file:

```
err:  invalid keys in command
file: .../Taskfile.yml:67:9
```

- **Reproduction:**

```
$ task --list
err:  invalid keys in command
file: /home/.../Taskfile.yml:67:9

$ for t in test setup demo:track1 demo:replay demo:track2 inspect mcp:build mcp:seed; do
    task "$t" --dry >/dev/null 2>&1 && echo "$t parses" || echo "$t FAILS TO PARSE"; done
test         FAILS TO PARSE
setup        FAILS TO PARSE
demo:track1  FAILS TO PARSE
demo:replay  FAILS TO PARSE
demo:track2  FAILS TO PARSE
inspect      FAILS TO PARSE
mcp:build    FAILS TO PARSE
mcp:seed     FAILS TO PARSE
```

- **Why it matters:** The README's quick start opens with `task setup`. Every demo command, the test
  suite, and both new MCP tasks are unreachable. This is not confined to the new code — the frozen,
  previously-approved CLI playground is collateral damage. A judge following the documentation gets
  a parse error as the first output of the project.
- **Required fix:** Quote the string (or drop the colon):

```yaml
- 'echo "Seed complete. Post-seed state - 1 belief (postulated, entered, 6 debts), 1 evidence, 0 intents, audit 0."'
```

Then re-run `task --list` and confirm all 8 tasks resolve.

---

## R-2 — The I-7 MCP boundary gate can never fail

- **Severity:** HIGH
- **Evidence:** `Taskfile.yml:79`

```sh
if grep -rn --include='*.go' 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view | grep -v '^\|// \|/\*\|\*\/'; then
```

The comment filter's pattern is the alternation `^ | // | /* | */`. Its **first alternative `^`
matches every line** (empty match at line start), so `grep -v` discards *all* input unconditionally.
The `if` condition is therefore never true and the gate always reports PASS.

- **Reproduction** (scratch copy — repository untouched):

```
$ cp -r cmd/solvent-mcp internal/view /tmp/gate/
$ cat >> /tmp/gate/view/view.go <<'GO'
func BackdoorPromote(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")
	return err
}
GO
$ cd /tmp/gate && if grep -rn --include='*.go' 'INSERT\|UPDATE\|…' solvent-mcp view | grep -v '^\|// \|/\*\|\*\/'; then
    echo FAIL; else echo PASS; fi
PASS          <<< a raw UPDATE is present and the gate passed it

# the first grep alone proves the write IS detectable:
view/view.go:167:	_, err := db.ExecContext(ctx, "UPDATE belief SET status='promoted' WHERE id=$1")
```

It also passes on a directory that does not exist (`grep` exits 2; `if` treats non-zero as false):

```
$ if grep -rn --include='*.go' 'INSERT\|…' does-not-exist 2>/dev/null | grep -v '^\|…'; then echo FAIL; else echo PASS; fi
PASS
```

- **Why it matters:** This gate is the *mechanical proof* behind the project's central claim — that
  the MCP layer cannot bypass the kernel. Today that claim rests entirely on manual code reading.
  Any future change can add a raw write to `cmd/solvent-mcp` or `internal/view` and `task test` will
  still print `I-7 MCP boundary: PASS`. This is my `plan4.1_eval` **F-4 reintroduced in a new form**
  by the comment-exclusion that was added to address it.
- **Required fix:** Drop the broken filter and fail closed. Comments are better excluded by matching
  SQL in string literals, or simply by accepting that the current sources have no such comments:

```sh
- |
  for d in cmd/solvent-mcp internal/view; do
    [ -d "$d" ] || { echo "FAIL: $d missing"; exit 1; }
  done
  if grep -rnE --include='*.go' '(ExecContext|Exec)\(.*(INSERT|UPDATE|DELETE|CREATE|DROP)' cmd/solvent-mcp internal/view; then
    echo "FAIL: raw writes in MCP/view"; exit 1
  else
    echo "I-7 MCP boundary: PASS"
  fi
```

Whatever form is chosen, it must be validated by the injection test above before being trusted.

---

## R-3 — Cross-scenario writes are unguarded; one path mutates and reports an error

- **Severity:** MEDIUM
- **Evidence:** `cmd/solvent-mcp/tools.go:97-131` (`retire_debt`), `:135-158` (`promote`).
  `kernel.Store.RetireDebt` (`internal/kernel/kernel.go:80`) and `Promote` (`:92`) take **no
  scenario**; the handlers use `scenario` only to scope `pipeline.AuditIntent`. No handler checks
  that `belief_id` belongs to `scenario`.

- **Reproduction** (track1 populated, then addressed as track2):

```
track1 belief: 08f0cb32-cb95-4386-bb6c-3a164e7b1ef2
debt BEFORE:   {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

solvent_retire_debt {"scenario":"track2","belief_id":"08f0cb32-…","debt_item":"needToyCheck"}
  → isError: true  {"error":true,"message":"sql: no rows in result set"}
debt AFTER:    {needNullModel,needObstruction,needFaithfulnessReview}      ← the write LANDED

solvent_promote {"scenario":"track2","belief_id":"08f0cb32-…"}
  → isError: none  {"result":{"status":"promoted"},"audit":{"live_on_nonpromoted":0}}
track1 belief status now: promoted    ← mutated under a track2 label, reporting track2's audit
```

- **Why it matters:** Two distinct problems. First, `promote` silently writes across a scenario
  boundary and attaches an audit number computed for a *different* scenario — the safety figure in
  the envelope does not describe the belief that was mutated. Second, and worse for an agent,
  `retire_debt` **performs the mutation and then reports failure**: `view.GetSnapshot` is called
  after the kernel write (`tools.go:112`) with the mismatched scenario, returns `sql.ErrNoRows`, and
  the handler surfaces that as the tool's result. An agent told "error" will reasonably retry,
  retiring further debt each time, or will narrate that nothing happened when something did.
  `cmd/operator-review/main.go:134-148` already performs exactly this consistency check
  ("Scenario/belief consistency pre-check (F-8)"), so the project has already ruled the guard
  necessary at one `cmd/` edge; the MCP edge lacks it.
- **Required fix:** Resolve the belief within its scenario **before** the kernel call, using the
  existing read-only projection, and return a clean domain error on mismatch:

```go
if _, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID}); err != nil {
    return errorResult(fmt.Errorf("belief %s not found in scenario %s", beliefID, scenario)), nil
}
```

This is an identity lookup, not an authorization decision — it does not reintroduce business logic,
and it mirrors the frozen CLI's own precedent.

---

## R-4 — The README has no MCP section

- **Severity:** MEDIUM
- **Evidence:** `grep -ni "mcp" README.md` → **no matches.** `git status` confirms `README.md` is
  unmodified. plan4.1 Phase 12 Step 14 and Definition-of-Done item 15 ("README explains honest
  scope") are unmet.
- **Reproduction:** `grep -c -i mcp README.md` → `0`.
- **Why it matters:** The README is the judge-facing document. As shipped, nothing in it mentions
  that an MCP server exists, how to build it, how to seed Track 2, how to configure a client, or the
  honest single-belief-retraction limitation. `imp.md` carries that content, but `imp.md` is an
  internal implementation note inside `plans/`, not documentation a judge will read.
- **Required fix:** Add the MCP section specified in plan3 §12.2 / plan4.1 Phase 12 — positioned
  after the CLI quick start, including the trust-boundary diagram, `mcp:build` / `mcp:seed`, client
  configuration, the Track 2 script, both refusals, and the single-belief limitation.

---

## R-5 — `.mcp.json` points at a binary that is not built, beside a stray untracked 16 MB binary

- **Severity:** MEDIUM
- **Evidence:** `.mcp.json:4` → `"command": "bin/solvent-mcp"`. Before this review, `bin/` did not
  exist (`ls: cannot access 'bin/': No such file or directory`), while a 16 MB `solvent-mcp` sat at
  the repository root — untracked, and **not** matched by `.gitignore`, which ignores only `/solvent`
  and `*.exe`/`*.so`-style patterns.
- **Reproduction:**

```
$ ls bin/
ls: cannot access 'bin/': No such file or directory
$ ls -la solvent-mcp
-rwxrwxr-x 1 … 16607722 … solvent-mcp
$ git status --short | grep solvent-mcp
?? solvent-mcp
```

- **Why it matters:** An MCP client reading `.mcp.json` on a fresh clone fails to launch the server
  until `task mcp:build` has run — and `task mcp:build` is itself unreachable because of R-1. Two
  independent defects therefore stack on the same demo path. Separately, a 16 MB binary that
  `.gitignore` does not cover is one `git add -A` away from entering history.
- **Required fix:** Add `/bin/` and `/solvent-mcp` to `.gitignore`, delete the root-level binary, and
  document `task mcp:build` as a prerequisite in the README MCP section (R-4). Optionally have
  `mcp:seed` depend on `mcp:build`.

---

## R-6 — Audit error is discarded on the refusal path, fabricating `live_on_nonpromoted: 0`

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/tools.go:247`

```go
audit, _ := pipeline.AuditIntent(context.Background(), db, scenarioID)
```

- **Why it matters:** Two defects in one line. The error is discarded, so a failed audit query is
  reported to the agent as `live_on_nonpromoted: 0` — the safest-looking value — on the exact code
  path that carries M1 and M2, the demo's centerpiece. The six success paths
  (`tools.go:37,86,122,149,177,206`) all correctly propagate the error, so this is an inconsistency
  rather than a design choice. It also substitutes `context.Background()` for the request context,
  ignoring cancellation.
- **Required fix:** Take `ctx` as a parameter and propagate the error, e.g. return the refusal with
  an explicit `"audit": null` or an `audit_error` field rather than a fabricated zero.

---

## R-7 — Wrong-typed optional arguments are silently ignored, changing query semantics

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/tools.go:24-25` — `beliefID, _ := args["belief_id"].(string)`.
  A non-string value yields `""`, which `view.GetSnapshot` treats as "no filter".
- **Reproduction:**

```
solvent_ledger {"scenario":"track2","belief_id":12345,"include_evidence":"yes"}
  → isError: none, returns the FULL ledger (all beliefs), evidence omitted
```

- **Why it matters:** A client that passes a numeric or malformed `belief_id` receives every belief
  in the scenario instead of an error. Under the project's own grounding rule — the agent must assert
  only what the latest tool result contains — a silently-widened query is precisely the failure mode
  that produces a confidently wrong narration. Note the advertised `required`/`enum` schema is
  **advisory**: omitting `belief_id` on `solvent_promote` is not rejected by the server and reaches
  CockroachDB as an empty UUID (clean error, but not schema-enforced).
- **Required fix:** Distinguish "absent" from "present but wrong type" in the type assertions and
  return a validation error for the latter.

---

## R-8 — Executable-relative fixture fallback is wrong for the shipped layout

- **Severity:** LOW
- **Evidence:** `cmd/solvent-mcp/main.go:48-51` computes `filepath.Join(filepath.Dir(exe),
  "internal", "derive", "testdata", "etcd_real")`. With the binary at `bin/solvent-mcp`, that
  resolves to `<repo>/bin/internal/derive/testdata/etcd_real`, which never exists.
- **Reproduction:**

```
$ env -u SOLVENT_FIXTURE_ROOT FABLE_DSN=… ./bin/solvent-mcp
SOLVENT_FIXTURE_ROOT: track directory missing: /home/…/go/bin/internal/derive/testdata/etcd_real/track1
exit=1
```

- **Why it matters:** The behavior is safe — it fails closed with a clear message — but the fallback
  is dead code, and `imp.md:168` states the opposite: *"points at `bin/solvent-mcp` … so the
  executable-relative fixture fallback works correctly."* It does not. A reader trusting that note
  would believe the server can start without `SOLVENT_FIXTURE_ROOT`; it cannot.
- **Required fix:** Either delete the fallback and require the environment variable, or resolve
  relative to the repository root (`filepath.Dir(exe)/..`). Correct `imp.md`'s F-5 rationale either way.

---

## R-9 — Nits

- **Severity:** NIT
- `internal/view/view.go:39-44` — `Beliefs`/`Intents` are nil slices when empty and marshal to
  `null`, not `[]`. Observed live: `"intents": null` in the post-seed ledger. A strict client or a
  narrating agent may mishandle it.
- `cmd/solvent-mcp/tools.go:235` — `envelopeResult(db *sql.DB, …)` never uses `db`.
- `internal/view/view.go:153` — `parsePGArray` duplicates the unexported
  `internal/pipeline/pipeline.go:392`.
- `cmd/solvent-mcp/main.go:61` — `os.IsNotExist(err)` treats a permission error as success; prefer
  `err != nil`.
- Raw driver errors surface as tool messages (`"sql: no rows in result set"`) rather than domain
  language.
- `imp.md:63` describes registration as `server.AddTool(server, toolDef, handler)`; the code uses
  `server.AddTool(&mcp.Tool{…}, handler)`.

---

# 4. Frozen-Core Verification

**PASS — untouched.**

```
$ git diff --stat HEAD -- db/ internal/kernel internal/belief internal/intent \
    internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review
(no output)
```

Full working-tree change set:

```
 M Taskfile.yml       M go.mod       M go.sum
?? .mcp.json  ?? cmd/solvent-mcp/  ?? internal/view/  ?? plans/mcp/  ?? solvent-mcp
```

Schema, kernel semantics, `FullDebt`, `RetractCascade`, `belief_edge`, `source_observed_at`
persistence and the CLI are all unmodified. `scripts/check_i7.sh` run directly:
`I-7 PASS: 7 ExecuteTx write sites, 0 raw writes, 1 permitted pool read (audit)`.

**Frozen demos** — run via `bash scripts/demo/*.sh` because `task` is broken (R-1); these are the
exact scripts the Taskfile invokes:

| Demo | Exit | Result |
|---|---|---|
| `track1.sh` | 0 | Evidence 3 / Beliefs 3 / Promoted 1 / Live intents 1 / **Audit SAFE** |
| `replay.sh` | 0 | 3/3/1 → 3/3/1, *"Replay verified: no duplicate ledger state created."* |
| `track2.sh` | 0 | baseline `retracted`, others `entered`, **Audit SAFE** |
| `inspect.sh` | 0 | *"Live intents on non-promoted beliefs: 0 (SAFE)"* |

The demo **logic** is unregressed. Only the `task` wrapper is broken.

---

# 5. MCP Boundary Verification

**PASS — the boundary is real.**

| Property | Evidence |
|---|---|
| Zero semantic conditionals | `grep -nE "if .*(Status\|Debt\|State)" cmd/solvent-mcp/tools.go` → one match, `if err := st.RetireDebt(...)`, an error check. No status/debt/state gating anywhere. |
| No raw SQL in the MCP layer | `grep -rn "db.Exec\|db.Query" cmd/solvent-mcp` → **zero matches**. |
| Audit provenance | All 7 audit values come from `pipeline.AuditIntent` (`tools.go:37,86,122,149,177,206,247`). No `count(*)`, no second audit query. The only `"promoted"`/`"live"` literals are response labels (`tools.go:156,184`). |
| `internal/view` is read-only | Three `SELECT` statements (`view.go:61,71,93,114`); no INSERT/UPDATE/DELETE/CREATE/DROP in the source. |
| Kernel dependency isolation | `go list -deps ./internal/kernel \| grep -c modelcontextprotocol` → **0**. SDK imported only by `cmd/solvent-mcp/{main,tools}.go`. |
| Transport | No listening socket while running (`ss -lptn` → 0 sockets owned by the server). stdio only. |
| Tool surface | Exactly 6 via live `tools/list`. No `create_belief`, `enter_belief`, `raw_sql`, `force`, `override`, `cancel_intent`. |
| Fixture boundary | `scenario` is a server-owned enum. `"../../../etc"` and `"/etc"` both rejected before any filesystem access. |
| Transactionality | MCP issues exactly one kernel call per mutation; cancellation is performed inside `RetractCascade`'s single `crdb.ExecuteTx`, not emulated as two MCP writes. |

**Advertised schemas** (from live `tools/list`, not the Go literals) — F-1 confirmed at the protocol
level:

| Tool | required |
|---|---|
| `solvent_ledger` | `[scenario]` (+ optional `belief_id`, `include_evidence`) |
| `solvent_ingest_evidence` | `[scenario]` |
| `solvent_retire_debt` | `[scenario, belief_id, debt_item]` |
| `solvent_promote` | **`[scenario, belief_id]`** |
| `solvent_authorize_action` | `[scenario, belief_id, action]` |
| `solvent_falsify` | `[scenario, belief_id]` |

`scenario` carries `enum: ["track1","track2"]` on all six.

---

# 6. M1 Verification — live receipt

Client: hand-driven JSON-RPC over stdio (see §9 for exactly what was tested).
Baseline UUID `85151346-eca1-485e-9121-c8cce20f8a7c` read from `solvent_ledger`, **not** from seed
output.

```
→ tools/call solvent_promote {"scenario":"track2","belief_id":"85151346-eca1-485e-9121-c8cce20f8a7c"}

JSON-RPC error field: None          ← delivered as a TOOL RESULT, not a protocol failure
isError (tool result): True
{
  "audit": { "live_on_nonpromoted": 0 },
  "result": {
    "error": true,
    "sqlstate": "23514",
    "constraint": "promoted_is_debt_free",
    "sentinel": "promotion blocked: open debt or final-truth language: ERROR: failed to satisfy
                 CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt,
                 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514)"
  }
}
```

**Provenance — this refusal came from CockroachDB.** The sentinel embeds the database's own rendering
of the CHECK expression, including CockroachDB's `:::STRING` / `:::INT8` type annotations. The MCP
layer contains no such string and no debt inspection (`tools.go:135-158` calls `st.Promote` directly).
The call path is `MCP → kernel.Store.Promote → crdb.ExecuteTx → CockroachDB → *pgconn.PgError →
kernel wrapIf → errors.As → toolError`.

---

# 7. M2 Verification — live receipt

Same belief, still unpromoted:

```
→ tools/call solvent_authorize_action
  {"scenario":"track2","belief_id":"85151346-…","action":"deploy etcd v3.5.0"}

JSON-RPC error field: None
isError (tool result): True
{
  "audit": { "live_on_nonpromoted": 0 },
  "result": {
    "error": true,
    "sqlstate": "23503",
    "constraint": "gate",
    "sentinel": "action refused: belief is not promoted: ERROR: insert on table \"action_intent\"
                 violates foreign key constraint \"gate\" (SQLSTATE 23503)"
  }
}
```

**Provenance:** the message is CockroachDB's own FK violation text naming table `action_intent` and
constraint `gate`. `tools.go:162-187` calls `st.IntentOnPromoted` with no status pre-check.

Two different constraints, two different SQLSTATEs, both originating in the schema.

---

# 8. Track 2 End-to-End Verification

Executed entirely through MCP tool calls; every state assertion independently re-queried in SQL.

| Stage | State (independent SQL) |
|---|---|
| **After seed** | 1 belief `postulated`/`entered`/**6 debts**, 1 evidence, 0 intents, audit 0 — matches plan3 §7.1 exactly |
| **M1** | refused `23514` / `promoted_is_debt_free` (§6) |
| **M2** | refused `23503` / `gate` (§7) |
| **Retire ×6** | debt drains `needMap → … → []` across six calls |
| **Promote** | `{"status":"promoted"}`, audit 0 |
| **Authorize** | `{"intent_state":"live","action":"deploy etcd v3.5.0"}`, audit 0 |
| **Ingest track2** | derives `accommodated` "fix available for etcd-io/etcd v3.5.0" (4 debts) + `derived` "v3.5.0–v3.5.2 has documented data inconsistency" (6 debts) |

**Post-ingest — plan3 §7.2 verified against the MCP sequence, not the CLI demo:**

```
beliefs  evidence  distinct_sha  live_intents  belief_edge  baseline_rows
   3         3           3             1            0            1

seeded UUID : 85151346-eca1-485e-9121-c8cce20f8a7c
post-ingest : 85151346-eca1-485e-9121-c8cce20f8a7c      ← unchanged, no duplicate
baseline    : postulated / promoted / debt {}
```

**Falsification** — UUID re-read from `solvent_ledger` immediately before the call, per the tool
description:

```
before: belief promoted | intent live | action "deploy etcd v3.5.0"

→ solvent_falsify {"scenario":"track2","belief_id":"85151346-…"}
  {"audit":{"live_on_nonpromoted":0},"result":{"retracted":1,"verdict":"PASS"}}

after (SQL):
  postulated    retracted   etcd v3.5.0 is approved for production deployment
  accommodated  entered     fix available for etcd-io/etcd v3.5.0
  derived       entered     v3.5.0–v3.5.2 has documented data inconsistency
  intent: cancelled (belief_status retracted, action "deploy etcd v3.5.0")
  audit_global 0 | belief_edge 0
```

`retracted: 1` — single-belief retraction, the other two beliefs untouched, `belief_edge` still
empty. The intent was cancelled inside `RetractCascade`'s transaction; MCP issued one call and never
touched `action_intent`.

---

# 9. Test / Build Results

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **CLEAN** |
| Vet | `go vet ./...` | **CLEAN** |
| Format | `gofmt -l cmd internal` | **CLEAN** (no output) |
| Tests | `unset FABLE_TEST_DSN && go test -count=1 -p 1 -v ./...` | **107 PASS, 0 FAIL, 0 SKIP** |
| I-7 kernel gate | `scripts/check_i7.sh` | **PASS** — 7 ExecuteTx sites, 0 raw writes, 1 permitted pool read |
| I-7 MCP gate | `Taskfile.yml:79` | **INERT** — see R-2 |
| Full suite | `task test` | **exit 102 — parse failure**, see R-1 |
| MCP build | `go build -o bin/solvent-mcp ./cmd/solvent-mcp` | OK, 16.6 MB (`task mcp:build` itself unreachable) |
| Dependency | `go.mod` | `modelcontextprotocol/go-sdk v1.7.0`, third direct dep; kernel unaffected. Indirect bumps: `golang.org/x/sync` 0.17→0.20, `x/sys` 0.32→0.41, plus `jsonschema-go`, `segmentio/*`, `uritemplate`, `oauth2`, `x/time`. |

The test count is exactly 107 — no drift. Note `task test` cannot deliver this result today; the 107
figure comes from invoking `go test` directly.

**Startup failure modes** — all fail closed with actionable messages:

| Condition | Exit | Message |
|---|---|---|
| No `FABLE_DSN` | 1 | `no DSN: set FABLE_DSN` |
| No `SOLVENT_FIXTURE_ROOT` | 1 | `track directory missing: …/bin/internal/derive/…/track1` (see R-8) |
| Bad fixture root | 1 | `track directory missing: /nonexistent/track1` |
| DB unreachable | 1 | `ping: … connection refused (is CockroachDB running? try: task setup)` |

**`dotenv:` mechanism (F-2)** — verified working in an isolated scratch Taskfile with `SOLVENT_DSN`
and `FABLE_DSN` unset in the parent shell: `SOLVENT_DSN=[postgresql://root@localhost:26260/fable…]`.
F-2's fix is substantively correct; it is merely unreachable because of R-1.

---

## MCP client tested — stated precisely

**Tested:** a hand-written JSON-RPC stdio client driving `bin/solvent-mcp` directly —
`initialize` (negotiated `protocolVersion 2025-06-18`, `serverInfo {name: solvent, version: v0.1.0}`)
→ `notifications/initialized` → `tools/list` → `tools/call` ×20+, including malformed input. This
exercises initialization, tool discovery, schema discovery, invocation, error results, and structured
result parsing.

**NOT tested:** VS Code, Codex, or Claude Code as a GUI client. `imp.md:267` mentions VS Code and
Codex; **neither was verified** and this review does not corroborate them.

**§23 agent grounding: NOT RUN.** Attaching `.mcp.json` to a live Claude Code session requires a
restart, so no LLM agent drove the tools. I verified the server, the schemas, and the shipped tool
descriptions — which do instruct the agent to read the ledger before asserting state, re-read after
mutation, obtain the UUID immediately before falsification, and not pre-check the gates. I could
**not** verify whether a model actually invents counts, reuses stale UUIDs, or fabricates SQLSTATEs.
Recommended procedure: fix R-1 and R-5, run `task mcp:build && task mcp:seed`, restart the client
with `.mcp.json` approved, run the plan3 §8 script verbatim, and check every asserted count, status
and UUID against the immediately preceding tool result.

---

# 10. Receipt Integrity

`imp.md` is **scope-honest**: its verification table claims only five things, and it makes no test or
demo claims at all — no "107 tests pass", no "demos pass". That restraint is correct and worth noting
given this project's history of stale receipts.

| `imp.md` claim | My fresh execution | Verdict |
|---|---|---|
| `go build ./...` clean | clean | **Reproduces** |
| `go vet ./...` clean | clean | **Reproduces** |
| `gofmt -l cmd internal` clean | clean | **Reproduces** |
| Frozen core unchanged | empty diff | **Reproduces** |
| "I-7 MCP boundary — grep gate in Taskfile — PASS" | prints PASS, but passes an injected raw `UPDATE` | **Technically true, substantively worthless** (R-2). It also cannot have been obtained via `task test`, which does not parse (R-1) |
| `imp.md:168` — `.mcp.json` at `bin/` "so the executable-relative fixture fallback works correctly" | fallback resolves to `bin/internal/derive/…`, which never exists | **False** (R-8) |
| `imp.md:63` — `server.AddTool(server, toolDef, handler)` | code uses `server.AddTool(&mcp.Tool{…}, handler)` | Inaccurate (NIT) |
| `imp.md:267` — "Connect an MCP client (e.g. VS Code, Codex)" | neither tested by the implementer or by me | **Unverified** |

**No stale transcripts were found** — `imp.md` contains no pasted runtime output, so the failure mode
that plagued the previous phase does not recur here. The integrity problem is different in kind: a
green light from a check that cannot go red.

---

# 11. Hackathon Risk

| Rank | Risk | Consequence |
|---|---|---|
| **Critical** | `Taskfile.yml` does not parse (R-1) | Nothing runs. `task setup` — the README's first command — fails. Demo dead on arrival on any fresh clone. |
| **High** | `.mcp.json` → `bin/solvent-mcp` not built, and `task mcp:build` unreachable (R-5 + R-1) | The MCP client fails to launch even after R-1 is fixed, unless the build step is run and documented. |
| **High** | No MCP section in the README (R-4) | A judge never learns the MCP demo exists; the strongest part of the project is invisible. |
| **Medium** | I-7 gate inert (R-2) | No demo impact today; it removes the guarantee that the boundary stays intact under future change, and a technical judge who reads the gate will spot it. |
| **Medium** | Agent grounding unverified (§23 NOT RUN) | Live behavior — invented counts, stale UUIDs — is unknown until one real rehearsal is performed. |
| **Low** | Cross-scenario writes (R-3) | The scripted demo reads UUIDs from the ledger and stays consistent; risk materializes only if a judge improvises across scenarios — which a curious judge may well do. |
| **Low** | Audit-zero on refusal-path error (R-6), typed-arg widening (R-7) | Narrow, unlikely to surface in a 4-minute demo. |

---

# 12. Exact Fix List

Only what is actually required. None of these touch the frozen core or the MCP architecture.

1. **`Taskfile.yml:67`** — quote the `echo` string (or remove the `: `). Verify with
   `task --list` and confirm all 8 tasks resolve. *(R-1, blocking)*
2. **`Taskfile.yml:79`** — replace the inert gate with a fail-closed version that asserts the
   directories exist and does not filter every line. **Validate the replacement by injecting a raw
   `db.ExecContext(… "UPDATE …")` into a scratch copy and confirming it FAILs.** *(R-2)*
3. **`.gitignore`** — add `/bin/` and `/solvent-mcp`; delete the 16 MB root-level `solvent-mcp`.
   Document `task mcp:build` as a prerequisite. *(R-5)*
4. **`README.md`** — add the MCP section per plan3 §12.2. *(R-4)*
5. **`cmd/solvent-mcp/tools.go`** — add a scenario-scoped belief lookup before the kernel call in
   `promote` and `retire_debt`, using `view.GetSnapshot`. *(R-3)*
6. **`cmd/solvent-mcp/tools.go:247`** — take `ctx`; stop discarding the audit error. *(R-6)*
7. **`plans/mcp/imp.md`** — correct the F-5 rationale (R-8), the `AddTool` signature, and remove the
   unverified VS Code / Codex claim.

Optional (R-7, R-9): strict type assertions, `[]`-not-`null` slices, drop the unused `db` parameter,
delete or repoint the executable-relative fallback.

After the fixes: re-run `task test` (expect 107 + a gate that can actually fail),
`task demo:track1/replay/track2/inspect`, `task mcp:build`, `task mcp:seed`, and one **real MCP-client
rehearsal** to close §23.

---

# 13. Scope Decision

**Implement fixes. Do not rework, and do not open another planning cycle.**

No architectural problem was found. The trust boundary, the six-tool surface, the audit provenance,
the fixture boundary, the error mapping, and both database refusals are all correct as designed and
as built. Every item in §12 is a localized edit — one YAML quote, one shell condition, one gitignore
entry, one README section, and three small Go changes.

The verdict is REWORK REQUIRED only because the repository in its current state cannot execute a
single documented command. That is a mechanical failure, not a design failure.

---

# 14. The Most Important Question

> *If I put the current repository on a fresh machine with CockroachDB, build the MCP server, connect
> one real MCP client, and follow the Track 2 demo, will the system genuinely demonstrate that an AI
> agent cannot override Solvent's transactional safety gates?*

## Today: NO — it breaks before the demo starts.

`task setup` fails with a YAML parse error (R-1). `task mcp:build` never runs, so `.mcp.json`'s
`bin/solvent-mcp` does not exist and the client cannot launch the server (R-5). The README does not
mention MCP at all, so a judge would not know to try (R-4). The break is entirely in the plumbing.

## After the §12 fixes: YES — and I have the receipts.

The demonstration is real, and I verified it by driving the protocol myself:

- The agent asks to promote a belief carrying six open debts. **CockroachDB refuses** with SQLSTATE
  `23514`, constraint `promoted_is_debt_free` — and the returned text is the database's own rendering
  of the CHECK expression, complete with `:::STRING` type annotations. No string in the MCP layer
  could produce that.
- The agent asks to authorize the deployment anyway. **CockroachDB refuses again**, SQLSTATE `23503`,
  constraint `gate`, with CockroachDB's own FK violation text naming `action_intent`.
- There is no tool that can override either. `tools/list` returns exactly six tools; no `set_status`,
  no `force`, no raw SQL. `grep` confirms zero `db.Exec`/`db.Query` in the entire MCP layer, and the
  handlers contain no conditional on status, debt, or intent state — the server is structurally
  incapable of deciding the answer itself.
- After the debts are legitimately retired, promotion succeeds, the deployment intent goes live, the
  real etcd postmortem is ingested, and falsification retracts the belief — **and the database
  cancels the dependent live intent inside the same transaction.** MCP issued one call and never
  touched `action_intent`. Audit 0, `belief_edge` 0, the other two beliefs untouched.

The claim under review was: *"The agent is not trusted. MCP is not trusted. The tool description is
not trusted. The database is the final authority."*

**In the executable reality, that statement is true.** The one thing that is not yet true is that
anybody can run it.
