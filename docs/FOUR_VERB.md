# Four-verb product slice: Connect → Ask → Authorize → Reassess

**Status:** CURRENT IMPLEMENTATION. This documents what a developer can do today with the existing Solvent kernel over MCP.

## The thesis in use

```
Agents may reason.
Agents may propose.
The database decides authority.
When the underlying belief changes, dependent authority changes too.
```

## Connect

Attach an agent to the existing Solvent MCP surface.

- Binary: `bin/solvent-mcp` built by `task mcp:build`
- Transport: stdio (`mcp.StdioTransport`)
- Env: `FABLE_DSN=postgresql://root@localhost:26260/fable?sslmode=disable` and `SOLVENT_FIXTURE_ROOT=internal/derive/testdata/etcd_real`
- Isolated workspace: `task agent:workspace` creates `~/.solvent-agent-box/solvent-mcp.json` with no repo files
- Seed: `task mcp:seed` (one `entered` belief, 6 debts) or `task mcp:seed:promoted` (one `promoted` belief + live intent) for deterministic start
- No hosted/remote MCP is required. The kernel remains local and domain-agnostic (`AGENTS.md:52`).

Current tools after this change: **seven** (`task mcp:verify` checks this over JSON-RPC):

```
solvent_ledger, solvent_ingest_evidence, solvent_retire_debt,
solvent_promote, solvent_authorize_action, solvent_falsify,
solvent_explain   (new, read-only)
```

## Ask

Inspect current belief/evidence/debt/authority state and understand *why* an action would or would not be authorized.

Preferred: `solvent_explain`. Raw inspection: `solvent_ledger`.

**`solvent_explain` — the AI-native Ask:**

- Input: `scenario` (track1/track2), optional `belief_id`, optional `include_evidence`
- Output (structured + human-readable):
  - `remaining_debt: []string`
  - `evidence_count, live_intents, is_promoted, is_retracted`
  - `can_promote: bool` + `promotion_blocked_reason` + `predicted_promotion_sqlstate: 23514` / `predicted_promotion_constraint: promoted_is_debt_free` when debt remains
  - `can_authorize: bool` + `authorization_blocked_reason` + `predicted_authorization_sqlstate: 23503` / `predicted_authorization_constraint: gate` when not promoted
  - `human_summary: string` — e.g. `Belief "…" is entered with 2 unresolved obligation(s): needOperatorSignoff, … . Promotion predicted to be refused. Authorization predicted to be refused by gate.`
  - `global_summary` + `audit_live_on_nonpromoted`
- Guarantees:
  - Read-only: only `SELECT` via `internal/view` — introduces no new source of truth, no writes, no `crdb.ExecuteTx`, no SQL text containing `INSERT/UPDATE/DELETE` in `internal/view` or `cmd/solvent-mcp` (verified by `Taskfile.yml` grep).
  - Never invents SQLSTATEs: real engine errors (`23503 gate`, `23514 promoted_is_debt_free`, `23514 live_requires_promoted`) are surfaced only when actually emitted by `solvent_promote`/`solvent_authorize_action`; predictions are labeled `predicted_*`.

**`solvent_ledger` — the source of truth:**

- Returns `beliefs{id,claim,claim_type,status,debt,final_truth}`, `evidence?`, `intents{belief_id,action,state}`, `audit_live_on_nonpromoted`. Call it before asserting any count/status/identifier and again after any mutation — never answer from memory (`main.go:90`).

Example conceptual interaction:

```
User: "Can I deploy this?"
Solvent (via solvent_explain): "Not yet. This belief still has 2 unresolved obligations: needProvenanceCheck, needOperatorSignoff. Promotion would be refused by promoted_is_debt_free (23514 predicted). Authorization would be refused by gate (23503 predicted)."
```

## Authorize

Attempt the existing kernel/database authorization path. The database decides.

```
agent proposal → Solvent authorization attempt → CockroachDB decides
```

- `solvent_promote` (`kernel.Promote` → `UPDATE belief SET status='promoted'` guarded by `CHECK promoted_is_debt_free` `23514`). On debt-open it returns `ErrPromotionBlocked` with the real `pgconn.PgError`.
- `solvent_authorize_action` (`kernel.IntentOnPromoted` → `INSERT action_intent` guarded by composite FK `gate` `23503` and `CHECK live_requires_promoted` `23514`). On unpromoted it returns `ErrActionOnUnpromoted`.

No prompt instruction, MCP prose, or UI conditional bypasses this. `solvent_explain` may *predict* a block, but the authoritative answer is the kernel’s error envelope. Presentation is a layer; enforcement is schema.

Debt discharge remains the CURRENT mechanism: `solvent_retire_debt(scenario, belief_id, debt_item)` per item (`kernel.RetireDebt` idempotent `array_remove`; handler refuses unknown items per `kernel.FullDebt`). No attestation object, no signing.

## Reassess

React to current falsification/retraction. “Reassess” in this phase means making the **current** explicit falsification semantics easy to use — not the future contradiction relation, temporal freshness, or background reassessment.

```
promoted belief → live action intent → belief falsified/retracted → dependent authority cancelled → Ask explains new state
```

- `solvent_falsify` (`kernel.RetractCascade` — two updates in one transaction: cancel live intents then retract beliefs, scenario-scoped, `WITH RECURSIVE` on `belief_edge`; `ON UPDATE CASCADE` re-evaluates `live_requires_promoted` so a live intent cannot survive).
- Demonstration: after a belief is promoted and an intent is live, `solvent_falsify` cancels the intent and retracts the belief; `solvent_explain` then reports `is_retracted=true, can_authorize=false, live_intents=[], audit_live_on_nonpromoted=0` and a human summary `… is retracted and has no live intents (audit clean).`

## Demo

Deterministic MCP-level script: `scripts/demo/four_verb.sh`

Shows:

```
Connect → Ask (blocked: debt remains) → discharge CURRENT obligations → Promote → Authorize live → Falsify/Reassess → Ask again (authority no longer valid)
```

Uses real project commands and real CockroachDB behavior; no faked outputs. Run after `task setup` or `task db:reset && task mcp:seed`.

## CURRENT IMPLEMENTATION vs FUTURE ARCHITECTURE

| Capability | Current (this slice) | Future (ADR-0001 / POST_HACKATHON) — deferred |
|---|---|---|
| Warrant | no table; authority = `live` intent + `promoted` belief + `gate` | portable warrant protocol, central verification, availability cost — not implemented |
| Attestation | no table, no signing; `solvent_retire_debt(belief_id,debt_item)` | independent attestation, integrity proof, key lifecycle — not implemented, no Ed25519/WebAuthn/PKI enrollment |
| A2A | not implemented | warrant-aware A2A extension — deferred |
| Remote/hosted MCP | not implemented | hosted MCP SaaS — deferred |
| Multi-tenancy | not implemented (`track1/track2` fixed) | Organization→Workspace scoping — deferred, would require frozen-table change |
| Risk tiers / policy engine | not implemented | `LOW_RISK`/`HIGH_IMPACT` classification — deferred |
| Temporal freshness | not implemented; no `validity_horizon/expires_at` | `intent_not_stale` + lazy expiration — deferred |
| Contradiction relation | detected and discarded (`belief.Process` slog) | `evidence_contradiction` + resolution debt — deferred |

## Verification

```
task test           # go test -p 1, go vet, gofmt, check_i7.sh, wizard single-call-site, MCP boundary, mcp_verify.sh (now 7 tools + explain)
```

New live tests: `internal/view/explain_test.go` — Ask before promotion (23514/23503 predicted), Promotion after discharge (can_promote→true→is_promoted→live), Reassessment (falsify → retracted, audit 0), Read-only (no mutation), Empty scenario. All against real CockroachDB, no mocks for invariants.

## Files changed in this slice

- `internal/view/view.go` + `internal/view/explain.go` (read-only projection)
- `cmd/solvent-mcp/main.go`, `cmd/solvent-mcp/tools.go` (solvent_explain)
- `internal/view/explain_test.go`, `scripts/mcp_verify.sh`
- `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh`

What remains untouched: `db/001_schema.sql` (frozen 4 tables), `db/003_wizard.sql`, `db/004_debt_vocabulary.sql`, `kernel/*` semantics, Lean model, `AGENTS.md`, `README.md`, `docs/ADR/0001-authority-and-attestation.md`.

No new ledger semantics, no invariant weakening, no second authority source.
