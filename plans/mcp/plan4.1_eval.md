# plan4.1 — Independent Evaluation

**Reviewer:** Claude (second architect)
**Date:** 2026-08-11
**Scope:** `plans/mcp/plan4.md`, `plan4_review.md`, `plan4.1.md`, `plan4.1_review.md`
(with `plan3_review.md` read for the bridge)
**Method:** claims verified against the live repository, not against the documents

---

# Verdict

## APPROVED WITH NITS — two require correction before handoff

**No rework.** Nothing structural should be reopened. The architecture, the six-tool surface, the
frozen-core fence, the Track 2 single-scenario narrative, and the corrected grounding rule are all
sound, and `plan4_review.md`'s shell-chaining fix landed correctly in plan4.1.

The findings below come from checking the plan's assertions against the actual code rather than
re-reading the plan chain. Two are contradictions that will stop or break the coding agent; four are
cheap corrections that would each cost minutes to discover the hard way.

---

# Must fix before handoff

## F-1 — The audit envelope is unsatisfiable for two of the five state-changing tools

**Severity:** blocking (trips a stop condition)
**Location:** plan3 §3.3, §3.4, §4.1; plan4.1 Phase 4 "Audit envelope"

`pipeline.AuditIntent(ctx, db, scenarioID)` (`internal/pipeline/pipeline.go:407`) requires a
scenario. But the two kernel functions behind `solvent_promote` and `solvent_retire_debt` do not
take one:

```go
func (s *Store) RetireDebt(ctx context.Context, beliefID, item string) error   // kernel.go:80
func (s *Store) Promote(ctx context.Context, beliefID string) error            // kernel.go:92
```

and neither tool schema carries a `scenario` field:

```json
solvent_promote      { "belief_id": "uuid" }
solvent_retire_debt  { "belief_id": "uuid", "debt_item": "string" }
```

Yet plan3 §4.1 and plan4.1 both mandate the envelope for **every** state-changing tool:

```json
{ "result": { … }, "audit": { "live_on_nonpromoted": N } }
```

with N sourced **exclusively** from `pipeline.AuditIntent`. plan3 §3.4's M1 refusal example prints
`"audit": { "live_on_nonpromoted": 0 }` literally — a value the handler has no scenario with which
to obtain.

**Why it matters.** This is a spec-internal contradiction in a document that stakes its authority on
being mechanically checkable. It trips plan4.1's own stop condition — *"Existing exported APIs are
incompatible with the plan"* — so a compliant coding agent halts at Phase 4 and reports. A
non-compliant one improvises: most likely a `SELECT scenario_id FROM belief WHERE id = $1` inside the
MCP layer, which is an unauthorized query, brushes against "do not create a separate audit query,"
and starts the erosion the whole boundary rule exists to prevent.

**Correction.** Add `scenario` to the `solvent_promote` and `solvent_retire_debt` schemas. This is
consistent with the three write tools that already carry it (`ingest_evidence`,
`authorize_action`, `falsify`), requires no kernel change, and preserves the audit rule intact.

---

## F-2 — `task mcp:seed` fails on a clean machine

**Severity:** blocking (breaks step one of the demo path)
**Location:** plan4.md §4.2, plan4.1 Phase 7 Step 8

Both plans specify:

```yaml
mcp:seed:
  cmds:
    - task db:reset
    - >
      go run ./cmd/operator-review --dsn "$SOLVENT_DSN" …
```

But `Taskfile.yml` declares the config file as an unused variable and **never sources it**:

```yaml
vars:
  CONFIG: scripts/demo/config.env     # declared, never used
```

Only the demo shell scripts source it themselves (`scripts/demo/*.sh:6-7`). In a plain shell neither
`SOLVENT_DSN` nor `FABLE_DSN` is set, so `$SOLVENT_DSN` expands to empty, `operator-review` falls
through to `os.Getenv("FABLE_DSN")` (also empty), and exits 1:

```
no DSN: pass --dsn or set FABLE_DSN
```

**Why it matters.** Seeding is the first command in the MCP demo path. This fails before any tool is
exercised, on the exact clean-machine setup a judge would have.

**Correction.** Either add `dotenv: [scripts/demo/config.env]` to the task (config.env is
dotenv-compatible — plain `KEY=value` with `#` comments), or hardcode the DSN the way `db:reset`
already hardcodes `solvent-crdb` and `fable`.

---

# Nits

## F-3 — Missing pgx driver blank import in `cmd/solvent-mcp/main.go`

`testdb.Open` is a thin wrapper that registers nothing:

```go
func Open(dsn string) (*sql.DB, error) {   // internal/testdb/testdb.go:98
    return sql.Open("pgx", dsn)
}
```

Every existing entrypoint carries the driver registration — `cmd/solvent/main.go:23`,
`cmd/operator-review/main.go:31`, `cmd/m0verify`, `cmd/m1verify` — but plan4.1 never mentions it for
the new binary. Omitted, the startup ping fails with `sql: unknown driver "pgx" (forgotten import?)`.

**Correction.** State `_ "github.com/jackc/pgx/v5/stdlib"` explicitly in the main.go step.

---

## F-4 — The I-7 grep gate has a silent-pass path and a false-positive path

```sh
if grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view; then
  echo "FAIL: raw writes in MCP/view"; exit 1
else
  echo "I-7 MCP boundary: PASS"
fi
```

**Silent pass:** `grep` exits **2** on a missing directory, and `if` runs the `then` branch only on
exit 0 — so exit 2 lands in `else` and prints PASS. The gate therefore passes before
`internal/view` exists, and would pass again if the directory were ever renamed or deleted.

**False positive:** the pattern matches comments and prose, not just SQL. There is in-repo
precedent — `internal/kernel/sql.go:78` reads *"composite FK's ON UPDATE CASCADE propagates…"* — and
`internal/view` is the package most likely to carry exactly that explanation.

**Why it matters.** This gate is the mechanical proof behind "the MCP server doesn't bypass the
kernel." A gate with a silent-pass path proves less than it claims.

**Correction.** Assert both directories exist before grepping, and narrow the match
(`--include=*.go`, word-boundary `-w`, or scope to string literals).

---

## F-5 — `.mcp.json`'s `go run` contradicts the executable-relative fixture fallback

plan4.1 Phase 5 specifies main.go should read `SOLVENT_FIXTURE_ROOT` *"with executable-relative
fallback."* Phase 6 ships:

```json
"command": "go", "args": ["run", "./cmd/solvent-mcp"]
```

Under `go run`, the executable lives in the Go build cache (`/tmp/go-build…/exe/`), so an
executable-relative fallback can never resolve `internal/derive/testdata/etcd_real`. The fallback is
dead code under the shipped config, and it muddies acceptance criterion 9 ("start with missing
`SOLVENT_FIXTURE_ROOT` → exit non-zero"), which would then pass for the wrong reason.

Secondary: `go run` recompiles on every server start and wraps the server in a child process,
complicating shutdown; and the relative `./cmd/solvent-mcp` depends on the client's working
directory, which varies by client.

**Correction.** Point `.mcp.json` at `bin/solvent-mcp` — which `task mcp:build` already produces —
or drop the fallback and require the env var outright.

---

## F-6 — Two cosmetic slips

- **`kernel.FullDebt` line reference.** plan4.md §2 cites `internal/kernel/kernel.go:22`; it is at
  `:28`. Every other entry in that API table verified correct.
- **Self-contradictory conditional grep.** plan4.1 Phase 8 Step 10 runs
  `grep -n 'if.*Status\|if.*Debt\|if.*State' cmd/solvent-mcp/tools.go` and requires
  *"zero matches (or only Go structural `if err != nil` patterns)"* — but `if err != nil` can never
  match that pattern, so the parenthetical is inert. The grep will, however, false-positive on
  legitimate argument handling and scenario resolution. Treat it as a review prompt, not a gate.

---

# Confirmed sound

Verified directly, and worth recording so it isn't re-litigated:

| Claim | Status |
|---|---|
| `wrapIf` preserves the driver error | **Correct.** `internal/kernel/errors.go:51` is `fmt.Errorf("%w: %w", sentinel, err)`, so `errors.As(err, &pgErr)` reaches the `*pgconn.PgError`. The error-mapping design works exactly as specified. |
| pgx import belongs at the `cmd/` edge | **Correct, and it matches the kernel's own stated rule** (`errors.go:27-30`: matching on the `sqlStater` interface "is what keeps the kernel free of any driver import: the driver is chosen at the cmd/ edge"). |
| `testdb.Open` at `:98`, signature `(dsn string) (*sql.DB, error)` | Correct. |
| `pipeline.Run` at `:288`, `pipeline.AuditIntent` at `:407`, `wrapIf` at `:51` | Correct. |
| `sql.Open` is lazy, so an explicit ping is required | Correct — and plan4.1 does specify the ping. |
| Post-seed state is 6 debts | Correct — `kernel.FullDebt` has exactly 6 items. |
| plan4_review's shell-chaining fix | Correctly applied in plan4.1. |
| Frozen-core fence, six tools, no CRUD/raw SQL/`force`/`override`, seed outside MCP, stdio-only, CLI fallback | Unchanged and correct. Do not reopen. |

---

# Summary

| ID | Severity | Issue | Cost to fix |
|---|---|---|---|
| F-1 | Blocking | Audit envelope unsatisfiable for `promote` / `retire_debt` — no scenario | Add one field to two schemas |
| F-2 | Blocking | `task mcp:seed` has no DSN — Taskfile never sources `config.env` | One `dotenv:` line |
| F-3 | Nit | Missing `_ "…/pgx/v5/stdlib"` in `main.go` | One import |
| F-4 | Nit | I-7 gate silently passes on missing dir; false-positives on comments | Guard + narrow the match |
| F-5 | Nit | `go run` contradicts the executable-relative fixture fallback | Point at `bin/solvent-mcp` |
| F-6 | Nit | `FullDebt` line ref off by 6; conditional grep is inert | Text only |

F-1 and F-2 are the only two that change outcomes. Everything else is a minutes-level correction that
would otherwise be discovered at the keyboard.
