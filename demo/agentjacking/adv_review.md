VERDICT: MERGE AFTER FIXES

## 1. Executive assessment

The implementation faithfully realizes the approved boundary-only Agentjacking demo for the specific attack path it demonstrates. Core isolation holds: `sentry_error` and `internal/agentjacking` are confined to the adapter/demo boundary, and no generic package (`normalize`, `derive`, `belief`, `kernel`, `pipeline`, `db`) imports the adapter or knows Sentry semantics. The adapter is pure translation: it constructs existing `NormalizedEvidence` and `DerivedBelief` values, retires zero debt (no `DebtMapping["sentry_error"]` entry), and delegates all policy decisions to the existing kernel. Layer 4 (`action_source`) is correctly enforced before any DB access, and the DB gate (`23503` / `gate`, `23514` / `promoted_is_debt_free`) is the actual authority boundary.

However, the demo's headline claim — "retrieved telemetry cannot become authority merely by persuading an agent to request an action" — is **conditional**. The MCP server exposes `solvent_retire_debt` and `solvent_promote` to any caller without role restriction. An adversarial agent that has access to all sixteen MCP tools can bypass the demo's attack path by first retiring all six debts and promoting the injected belief, then calling `solvent_authorize_action` with `action_source: "user_typed"`. The README does not explicitly state that debt-retirement and promotion tools must be withheld from the agent, which leaves the security story underspecified.

The remaining findings are documentation gaps, a test gap, and a demo-script destructiveness issue. None of them are code-level security bypasses.

---

## 2. Findings

### [P1] Security guarantee is conditional on host-level tool restriction not stated in the README

**Evidence:**
- `cmd/solvent-mcp/tools.go` lines 103-165 expose `solvent_retire_debt` and lines 167-201 expose `solvent_promote` to any MCP client.
- `cmd/solvent-mcp/main.go` lines 122-242 register all sixteen tools with no authentication, authorization, or tool-access control.
- `scripts/demo/agentjacking.sh` lines 223-280 show the Python "operator" persona calling `solvent_retire_debt`, `solvent_promote`, and `solvent_authorize_action` via the same MCP server the "attacker" persona uses in Beats 3-5.

**Attack / failure mode:**
An adversarial agent with access to all Solvent MCP tools can:
1. Call `solvent_ledger` -> discover the injected belief ID.
2. Call `solvent_retire_debt` six times (items from `kernel.FullDebt`).
3. Call `solvent_promote` -> succeeds because debt is now empty.
4. Call `solvent_authorize_action` with `action_source: "user_typed"` -> live intent succeeds.

**Why it matters:**
The demo's central claim is that "retrieved telemetry cannot become authority merely by persuading an agent to request an action." That claim is true only for the narrow path where the agent attempts `solvent_authorize_action` directly. It is false if the agent can be persuaded to perform the multi-step sequence above. The README says "Restrict the agent to only the six `solvent_*` tools" (`demo/agentjacking/README.md` lines 43-48) but never specifies that `solvent_retire_debt` and `solvent_promote` must be excluded from that set.

**Recommendation:**
Add an explicit "Host-level tool restriction" section to `demo/agentjacking/README.md` stating that `solvent_retire_debt`, `solvent_promote`, `solvent_falsify`, and any other state-mutation tool must be withheld from the agent. The demo proves the DB gate; it does not prove that the agent cannot manipulate debt/promotion directly.

---

### [P2] Demo script drops the entire database instead of using the scenario-scoped reset

**Evidence:**
- `scripts/demo/agentjacking.sh` line 34: `docker exec solvent-crdb cockroach sql --insecure -e "DROP DATABASE IF EXISTS fable CASCADE; CREATE DATABASE fable;"`
- `demo/agentjacking/ingest/main.go` lines 76-91 implement `--reset` as a scenario-scoped delete set that preserves other scenarios.

**Attack / failure mode:**
A developer running `task demo:agentjacking` loses all data in the `fable` database, including track1 and track2 state, without any confirmation prompt.

**Why it matters:**
Review Area 12 explicitly asks to verify "no DROP DATABASE" and "track1/track2 state remains untouched." The `--reset` flag in `ingest/main.go` satisfies this, but the demo script bypasses it with a full DB drop.

**Recommendation:**
Replace the full `DROP DATABASE` with the existing `task db:reset` (which already reapplies schema) or rely solely on the scenario-scoped `--reset` flag. If a full drop is required for reproducibility, print a prominent warning.

---

### [P2] Taskfile containment grep does not catch imports of `internal/agentjacking`

**Evidence:**
- `Taskfile.yml` lines 276-281 run `grep -rnF 'sentry_error'` over `internal/normalize internal/derive internal/belief kernel internal/pipeline db/`.
- No grep checks for the import path `"github.com/PithomLabs/solvent/internal/agentjacking"`.

**Attack / failure mode:**
A contributor adds `import "github.com/PithomLabs/solvent/internal/agentjacking"` to `internal/derive` (or another core package) without using the literal string `sentry_error` in the same file. The CI gate passes.

**Why it matters:**
The architectural invariant is that core packages never depend on the adapter. The grep guards the most likely leakage vector (the source-type string) but not the stronger invariant (no import).

**Recommendation:**
Add a second grep pass to `Taskfile.yml`:
```bash
if grep -rnF '"github.com/PithomLabs/solvent/internal/agentjacking"' internal/normalize internal/derive internal/belief kernel internal/pipeline db/; then
  echo "FAIL: internal/agentjacking imported into core package"
  exit 1
fi
```

---

### [P2] Demo script does not exercise the `sentry_error_nodetect` fixture end-to-end

**Evidence:**
- `scripts/demo/agentjacking.sh` lines 19-20 define `NODETECT_FIXTURE` but the variable is never referenced in any beat.
- `demo/agentjacking/README.md` line 12 claims the nodetect fixture proves the defense.
- `internal/agentjacking/adapter_test.go` lines 141-184 (`TestD_DetectorMiss`) covers it at the unit-test level only.

**Attack / failure mode:**
N/A (test gap, not a bypass).

**Why it matters:**
A viewer watching the seven-beat demo never sees the detector-miss case. The README implies the demo proves it, but only the unit test does.

**Recommendation:**
Add a Beat 2b that runs the adapter against `sentry_error_nodetect.json` and prints the resulting `embedded_commands`, claim, and debt, confirming the same structural refusal.

---

### [P2] Missing Go unit test for `solvent_authorize_action` success path on a promoted belief

**Evidence:**
- `cmd/solvent-mcp/tools_agentjacking_test.go` tests the refusal paths (`tool_output`, missing/invalid `action_source`, `user_typed` on unpromoted/unknown belief) but contains no test that calls `handleSolventAuthorizeAction` with a promoted belief and `user_typed`.
- `cmd/solvent-mcp/tools_authority_test.go` tests v0 authority tools (`solvent_authorize`, `solvent_approve`, etc.) but not `solvent_authorize_action`.

**Attack / failure mode:**
N/A (test gap).

**Why it matters:**
Beat 6 of the demo proves the legitimate path, but the Go test suite does not. A regression that accidentally breaks the promoted-belief authorization path would not be caught by `task test`.

**Recommendation:**
Add a test in `cmd/solvent-mcp/tools_agentjacking_test.go` (or a new file) that:
1. Creates a belief, retires all six debts, and promotes it.
2. Calls `handleSolventAuthorizeAction` with `user_typed`.
3. Asserts `IsError == false` and the response contains `intent_state: "live"` and an `audit` envelope.

---

### [P3] `parsePGArray` does not handle quoted PostgreSQL array elements

**Evidence:**
- `internal/view/view.go` lines 154-164.
- `internal/pipeline/pipeline.go` lines 392-404.

**Attack / failure mode:**
If a debt item ever contained a comma or quote, `strings.Split(s, ",")` would corrupt the array parsing.

**Why it matters:**
Current debt items are simple strings without commas, so this is not a live bug. It is a maintainability risk.

**Recommendation:**
Replace `strings.Split` with a proper PostgreSQL text-array parser or ensure debt items remain comma-free by construction.

---

### [P3] `cleanMessage` strips all leading `#` characters, not just markdown heading markers

**Evidence:**
- `internal/agentjacking/markdown.go` line 15: `strings.TrimLeft(line, "#")`.

**Attack / failure mode:**
A line such as `#123` would become `123`. Not a security issue because `message_clean` is not used for security decisions.

**Why it matters:**
Minor correctness issue in the demo-local cleaner.

**Recommendation:**
Use `strings.TrimSpace` followed by `strings.TrimPrefix(..., "##")` (and repeated for `#`, `###`) to strip only heading markers.

---

## 3. Security boundary assessment

**What exactly stops an Agentjacking payload?**

| Layer | What it does | What it does NOT do |
|---|---|---|
| **Adapter** (`internal/agentjacking`) | Parses the Sentry fixture, hashes raw bytes, builds `external_feed` evidence, emits a fixed-shape `Derived` claim. Never retires debt, never promotes, never authorizes. | Does not block attacker payloads; it faithfully translates them into generic evidence. |
| **Debt engine** (`internal/belief/mapping.go`) | `DebtMapping` has no `sentry_error` entry, so `DebtItemsForEvidence` returns `nil`. `belief.Process` retires zero debt. | Does not inspect payload semantics; it simply has no rule for the source type. |
| **DB gate** (`db/001_schema.sql` constraints `promoted_is_debt_free` and `gate`) | Refuses promotion while debt is non-empty (`23514`). Refuses live `action_intent` on an unpromoted belief (`23503`). | Does not prevent a caller with full MCP access from retiring debts and promoting manually. |
| **Layer 4** (`cmd/solvent-mcp/tools.go` `handleSolventAuthorizeAction`) | Refuses `action_source: "tool_output"` before any DB access. | Does not block `user_typed`; it explicitly passes through to the DB gate. |
| **Operator review** (`cmd/operator-review` / wizard UI) | Human-operated path that retires debts and promotes. | Not enforced by code; it is an organizational/procedural control. |

**Detector** (`DetectCommands`): informational audit metadata only. It does not stop any attack path. The defense holds when it misses (Test D, `sentry_error_nodetect.json`).

**Critical gap:** The DB gate and debt engine stop the *automatic* path (ingest -> promote -> authorize). They do **not** stop a caller who manually invokes `solvent_retire_debt` and `solvent_promote` via the MCP server. The actual security boundary against a fully-privileged agent is therefore **host-level tool restriction**, which the README does not explicitly state.

---

## 4. Core isolation assessment

**Is Sentry still isolated to `internal/agentjacking` and demo infrastructure?**

Yes, with one minor documentation gap.

- `sentry_error` appears **only** in:
  - `internal/agentjacking/` (adapter source and tests)
  - `demo/agentjacking/` (fixtures, README, ingest CLI)
  - `scripts/demo/agentjacking.sh` (fixture paths)
  - `Taskfile.yml` (containment grep and demo task)
  - Planning/review documents (`.zcode/plans/`, `demo/agentjacking/*.md`)

- **No core package** (`internal/normalize`, `internal/derive`, `internal/belief`, `kernel`, `internal/pipeline`, `db/`) contains the literal string `sentry_error`.
- **No core package** imports `internal/agentjacking`. The only non-demo import is `demo/agentjacking/ingest/main.go` (line 24).
- `cmd/solvent-mcp/scenarios.go` contains a comment mentioning the adapter (lines 19-20), but this is interface-layer documentation, not a behavioral dependency.
- `pipeline.sourceTypeMap` has no `sentry_error` entry; the generic pipeline cannot process Sentry fixtures.
- `derive.Derive` has no `sentry_error` case; it falls through to `default: return nil`.
- `DebtMapping` has no `sentry_error` entry.

**Exception:** The Taskfile containment grep (`Taskfile.yml` lines 276-281) does not check for imports of `internal/agentjacking`. It is a meaningful guardrail but is not exhaustive.

---

## 5. Pre-v0 honesty assessment

**What the demo proves:**
1. Attacker-controlled telemetry ingested as `external_feed` evidence retires zero debt.
2. A belief carrying full debt cannot be promoted (`23514 · promoted_is_debt_free`).
3. An unpromoted belief cannot warrant a live action (`23503 · gate`).
4. `action_source` is caller-declared hygiene, not provenance proof.
5. The defense is structural (fixed-shape claim + zero debt), not detection-based.

**What the demo does NOT prove:**
1. **Exact target/action authorization binding.** This is the v0 authority lifecycle (`target_snapshot` / `target_activation`). The demo is explicitly pre-v0 and does not claim this.
2. **That the agent cannot promote beliefs or retire debts.** The MCP tools for debt retirement and promotion are exposed without role restriction. The demo assumes the agent is barred from them by the host application, but it does not prove this.
3. **Shell isolation or egress control.** The README correctly states that Solvent protects the Solvent-mediated authorization path, not the agent's execution environment.

**Overselling risk:**
The README is mostly careful, but the phrase "retrieved telemetry cannot become authority" (`demo/agentjacking/README.md` line 3) is stronger than the code proves. It should be qualified: "retrieved telemetry cannot become authority through the `solvent_authorize_action` path alone, provided the agent cannot call debt-retirement or promotion tools."

---

## 6. Test adequacy

**Tests that genuinely prove architecture:**
- **A** (`adapter_test.go`): `external_feed` provenance, SHA over raw bytes, `message_raw` preservation, `message_clean` stripping, `embedded_commands`.
- **B** (`adapter_test.go`): Fixed-shape claim, `Derived` classification, no command text in claim.
- **C** (`ingest_test.go`): Post-ingest debt equals `kernel.FullDebt` exactly (order-sensitive).
- **D** (`adapter_test.go`): Detector miss — claim remains non-actionable, evidence intact.
- **F** (`ingest_test.go`): `IntentOnPromoted` on unpromoted belief -> `ErrActionOnUnpromoted` / `23503`.
- **G** (`ingest_test.go`): `Promote` on debt-full belief -> `ErrPromotionBlocked` / `23514`.
- **Layer 4 tests** (`tools_agentjacking_test.go`): `tool_output` refused without audit envelope; missing/invalid `action_source` refused; `user_typed` on unpromoted belief hits DB gate with audit envelope; `user_typed` with unknown belief reaches DB path; nil-DB panic test proves no DB access for `tool_output`.
- **MCP verify script** (`scripts/mcp_verify.sh`): End-to-end JSON-RPC verification of all Layer 4 behaviors, debt-item guard, track3 enum presence, and `solvent_explain` read-only guarantee.

**Tests that are weaker than they appear:**
- **C** checks debt order by iterating `kernel.FullDebt` and indexing into `res.DebtRemaining`. This relies on PostgreSQL preserving array order and `parsePGArray` not corrupting it. It is correct today but fragile if array elements ever contain commas.

**Important missing adversarial tests:**
1. **No unit test for `solvent_authorize_action` success on a promoted belief.** The demo script proves this in Beat 6, but the Go test suite does not. A regression here would go undetected.
2. **No end-to-end test for the nodetect fixture through the MCP server.** Test D proves the adapter output; the demo script does not show the full seven-beat path with the nodetect payload.

---

## 7. Final merge recommendation

**MERGE AFTER FIXES**

Blocking items:
1. **P1 - README must explicitly state the host-level tool restriction.** The security guarantee is conditional on the agent being barred from `solvent_retire_debt`, `solvent_promote`, and other state-mutation tools. The README must say this plainly.
2. **P2 - Replace the full `DROP DATABASE` in `scripts/demo/agentjacking.sh` with the scenario-scoped reset or a documented destructive warning.** The `--reset` flag in `demo/agentjacking/ingest/main.go` already implements the correct behavior; the script should use it.

Non-blocking (address in follow-up):
- Add import-path check to `Taskfile.yml` containment grep.
- Add a beat for the `sentry_error_nodetect` fixture.
- Add a Go unit test for `solvent_authorize_action` on a promoted belief.
- Fix `parsePGArray` and `cleanMessage` minor issues.
