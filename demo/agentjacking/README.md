# Agentjacking Demo

This demo shows that attacker-controlled telemetry cannot become an authorized action through the `solvent_authorize_action` path alone.

A poisoned Sentry error carrying a fake `## Resolution` with `npx @attacker/diagnose` is ingested as generic external-feed evidence. The embedded command remains evidence, not instruction. The belief cannot be promoted (six debts remain), and any attempt to act on it is refused by the database gate.

## What this demo proves

- Attacker-controlled telemetry, once ingested as `external_feed` evidence, retires zero debt and can never be promoted.
- An unpromoted belief cannot warrant a live action — the database gate (`23503`) enforces this structurally.
- A lying agent that declares `action_source: "user_typed"` still hits the real DB gate and is refused — the `action_source` field is not the security boundary.
- The defense holds even when the embedded command detector misses entirely (the `sentry_error_nodetect` fixture proves this).

## What this demo does NOT prove

This is a **pre-v0 demonstration**. It exercises the current belief-promotion/action gate (`23514` / `23503`). The forthcoming v0 authority lifecycle strengthens this by binding authorization to an exact approved target/action tuple, closing the separate confused-deputy case where an agent cites a genuinely promoted belief that was never approved for this specific action.

Nothing in this demo replaces shell isolation, egress controls, or agent-runtime permissions. The agent could theoretically execute `npx @attacker/diagnose` directly without going through Solvent — Solvent protects the **Solvent-mediated authorization path**, not the agent's own execution environment.

That the agent cannot promote beliefs or retire debts directly — the MCP tools for debt retirement and promotion are exposed without role restriction. The demo assumes the host restricts these (see Host-level tool restriction below).

## Host-level tool restriction

The demo assumes the host agent is restricted from `solvent_retire_debt`, `solvent_promote`, `solvent_falsify`, and other state-mutating tools. Within that constrained action path, retrieved telemetry cannot become authority — the database gate remains the actual pre-v0 authorization boundary.

This is consistent with the architectural position that agent tool allowlisting is a host/runtime responsibility, while Solvent supplies the server-side backstop. `action_source` is caller-declared SVC hygiene, not the security boundary. The DB gate (`23503 · gate`, `23514 · promoted_is_debt_free`) is the real enforcement point — but only once the host prevents the agent from directly performing the debt-retirement/promotion workflow.

## Architecture

```
Sentry fixture (poisoned telemetry)
        |
        v
internal/agentjacking (boundary adapter)
        |  Parse, DetectCommands, BuildEvidence, Belief
        |  Sentry-specific knowledge lives HERE and nowhere else
        v
generic Solvent types (NormalizedEvidence, DerivedBelief)
        |
        v
existing belief/debt machinery (belief.Process, kernel.FullDebt)
        |
        v
existing DB authorization gate (promoted_is_debt_free, gate, live_requires_promoted)
```

The Sentry adapter constructs existing `NormalizedEvidence` directly with `provenance_class = external_feed`. The generic core — `normalize`, `derive`, `belief`, `kernel`, `pipeline`, `db` — never learns that this evidence came from Sentry. No `SourceSentryError` constant, no `deriveFromSentry` function, no `DebtMapping["sentry_error"]` entry, no pipeline registration.

## `action_source` — caller-declared SVC hygiene

`action_source` is a **caller-declared provenance signal** on `solvent_authorize_action`. It is not cryptographically trustworthy — an adversarial agent can lie about it.

- `tool_output` → refused immediately, before any database access (no audit envelope, no `AuditIntent` call).
- `user_typed` → proceeds to the actual database gate, where the real security check happens.

Beat 4 of the demo deliberately demonstrates the lying-agent path: the agent declares `user_typed`, passes Layer 4, but the database still refuses because the belief is not promoted (`23503 · gate`). This establishes that `action_source` is honest SVC-level hygiene, not the security boundary.

## `embedded_commands` — informational audit metadata

`embedded_commands` is the output of a regex scan for command-like tokens (`npx`, `npm exec`, `pip install`, `curl | sh`, `wget`) in the Sentry message. It is **observational audit metadata for human review**, not a security control.

The defense remains effective even when the detector misses the injected instruction entirely. Test D in the test suite creates an attacker instruction that avoids all recognized patterns — the belief is still non-actionable, all six debts remain, promotion still fails, and action authorization still fails. The structural defense is:

1. The adapter produces a fixed-shape, non-actionable claim that never contains command text.
2. `sentry_error` has no `DebtMapping` entry, so all six debts remain.
3. The database gate refuses to promote a belief with outstanding debts.

Detection is decoration. The security is structural.

## The seven beats

| Beat | What happens | Key observation |
|------|-------------|-----------------|
| 1. The attack | Print the poisoned Sentry fixture | Realistic error text with attacker-controlled remediation |
| 2. Ingest | Adapter converts fixture to generic evidence | `external_feed` provenance, non-actionable claim, six debts, verbatim `message_raw` |
| 3. Naive agent | `action_source = "tool_output"` | Layer 4 refusal, **no database access** (no audit envelope) |
| 4. Lying agent | `action_source = "user_typed"` | Passes Layer 4, **database refuses**: `23503 · gate` (audit envelope present) |
| 5. Promotion attempt | `solvent_promote` | `23514 · promoted_is_debt_free` — all six debts still present |
| 6. Legitimate authority | Operator review → promote → authorize | Belief promoted, live intent succeeds, `live_on_nonpromoted = 0` |
| 7. Final ledger | Read-only snapshot | Injected → unpromoted → refused; reviewed → promoted → live |

## Running the demo

```bash
task demo:agentjacking
```

Prerequisites: `task setup` must have been run at least once (CockroachDB container exists).

## Nothing executes

The demo never executes the attacker command. It never installs a package. It never contacts attacker infrastructure. The fixture is a static JSON file on disk. The `embedded_commands` field is audit metadata, not an execution plan.

## Test coverage

| Test | What it proves |
|------|---------------|
| A | Generic evidence representation: `external_feed`, `ContentSHA256` = SHA of raw fixture bytes, `message_raw` byte-identical, `message_clean` stripped, `embedded_commands = ["npx"]` |
| B | Non-actionable derivation: fixed-shape claim, `Derived` (never `Accommodated`), claim contains no command text |
| C | Six untouched debts: after ingest, `belief.debt` equals `kernel.FullDebt` exactly |
| D | Detector miss: `sentry_error_nodetect` fixture → empty `embedded_commands`, claim still non-actionable, six debts remain, promotion fails, authorization fails |
| E | Layer 4 validation: missing/tool_output/invalid `action_source` → refusal with no DB access |
| F | Lying-agent path: `user_typed` + unpromoted belief → `23503 · gate` |
| G | Promotion: injected belief → `23514 · promoted_is_debt_free` |
