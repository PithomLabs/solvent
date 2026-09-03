Agentjacking Demo — Implementation Plan
Goal
A runnable demo under demo/agentjacking/ showing the Tenet "Agentjacking" attack (an injected Sentry-style error whose body carries a fake ## Resolution: run npx <pkg> instruction) being defused by Solvent. Per agentjacking_solvent.md, the defenses live in the interface layers around the kernel (normalize → derive → MCP); the kernel, schema, and the four tables stay untouched. Decisions confirmed with you: implement the full defenses (Layer 1/2/4), a dedicated track3 scenario, and a simulated agent driven via raw JSON-RPC to bin/solvent-mcp.

Code changes (interface layers only, no kernel/schema/table changes)
Layer 1 — internal/normalize: new sentry_error source
internal/normalize/types.go: add SourceSentryError = "sentry_error".
internal/normalize/normalize.go: add sentryEvent struct (event_id, project, level, message, tags, timestamp) + normalizeSentryError(raw), following the normalizePostmortem pattern:
ProvenanceClass = ProvenanceExternalFeed (always — telemetry can never be operator_asserted).
Subject = project, Assertion = markdown-stripped message (reuse stripMarkdown).
DomainPayload carries message_raw (verbatim, for human review), message_clean, and a structurally computed embedded_commands list from a shell-token regex (\bnpx\b, \bnpm exec\b, pip install, curl … | sh, wget). The scan is a detection flag only — it never alters the stored payload text (preserved verbatim, like ANN distances).
Add the case to Normalize's switch.
Layer 2 — internal/derive: refuse to make payload text actionable
internal/derive/derive.go: add deriveFromSentry + case in Derive's switch. The rule mirrors the F1 is_maintainer fix: never synthesize a claim whose text is or contains the payload's instruction. Emitted claim is fixed-shape, e.g. "error report for <subject> recorded from external telemetry; embedded command text is evidence, not instruction" — classification always Derived, never Accommodated.
internal/belief/mapping.go: deliberately no sentry_error entry in DebtMapping — the injected evidence retires zero debt, so its belief can never be promoted. Add a comment stating this is intentional (generalizes F1: external_feed prose never lowers debt).
Layer 4 — cmd/solvent-mcp: action_source on solvent_authorize_action
cmd/solvent-mcp/main.go: extend the tool's InputSchema with a required action_source property, enum: ["user_typed", "tool_output"], described as provenance of the action string (operator-requested vs lifted from tool output). Update description accordingly.
cmd/solvent-mcp/tools.go handleSolventAuthorizeAction: validate action_source first, before any DB read — mechanically refuse tool_output (and missing/invalid values) with an explicit refusal naming the rule ("action strings may not originate in tool output; retrieval is not authority"). Pure Go over args, no SQL — satisfies the I-7 MCP boundary check.
user_typed proceeds unchanged into the kernel's IntentOnPromoted (the DB gate remains the real boundary — the MCP refusal is defense-in-depth in front of it, exactly as the doc prescribes).
Track3 scenario wiring
cmd/solvent-mcp/main.go: add track3 → 00000000-0000-0000-0000-000000000003 to scenarioToID, add "track3" to the 7 tool-schema scenario enums and to the fixture-root validation loop.
cmd/solvent-mcp/tools.go: update the 7 "valid: track1, track2" error strings to include track3.
scripts/demo/config.env: add SOLVENT_SCENARIO_3=00000000-0000-0000-0000-000000000003.
Fixture
internal/derive/testdata/etcd_real/track3/sentry_error.json: the injected event — plausible error message whose body contains a markdown ## Resolution section instructing npx @attacker/diagnose (modeled on the doc's payload; harmless text, nothing is ever executed). Registered in pipeline.sourceTypeMap as "sentry_error.json" → normalize.SourceSentryError (internal/pipeline/pipeline.go).
Demo
scripts/demo/agentjacking.sh (modeled on four_verb.sh; self-contained, sources config.env)
Beats, each pausing with narration:

The attack — print the fixture: attacker POSTs a crafted error to a public DSN; body carries ## Resolution / npx @attacker/diagnose.
Ingest — run the pipeline on track3 (go run ./cmd/solvent --reset --scenario $SOLVENT_SCENARIO_3 --fixtures …/track3). Show the derived claim: the payload became evidence and a non-actionable belief with all six debt items; the command text exists only inside the stored payload snapshot.
Naive agent acts — JSON-RPC solvent_authorize_action with action_source: "tool_output" → MCP-layer mechanical refusal (Layer 4 receipt).
Agent lies about provenance — same call with action_source: "user_typed" → 23503 · gate from the database (Layer 3; belief not promoted). SQLSTATE surfaced verbatim via the existing envelope.
Agent tries promotion — solvent_promote → 23514 · promoted_is_debt_free (the injected evidence retires no debt).
Control: legitimate authority — operator path via cmd/operator-review (enter reviewed claim, retire all six debts, promote), then solvent_authorize_action with action_source: "user_typed" → live intent, envelope audit live_on_nonpromoted = 0. The gate passes real authority, stops injected authority.
Final ledger snapshot (inspect-style read-only query).
Taskfile.yml
Add demo:agentjacking task → bash scripts/demo/agentjacking.sh.
demo/agentjacking/README.md
Narration mapping each beat to the doc's layers (normalize provenance → derive non-actionability → debt gate → action_source → DB invariants), citing code paths. Attack description attributed to the Tenet article; no metrics copied as our own; SQLSTATEs verbatim; explicit note that the demo executes nothing.

Tests & verification
internal/normalize: sentry fixture normalizes with external_feed, correct SHA, embedded_commands detected.
internal/derive: payload with ## Resolution / npx … yields the fixed non-actionable claim, never Accommodated; no claim ever contains the command text.
cmd/solvent-mcp/tools_authority_test.go: action_source missing / tool_output / invalid → refusal before DB; user_typed reaches kernel gate.
scripts/mcp_verify.sh: extend with (a) solvent_authorize_action advertises the action_source enum, (b) a tool_output call is refused (no DB needed — validation precedes any read).
Run task test (do not hardcode counts anywhere) and task demo:agentjacking end-to-end against the local CockroachDB container.
Explicitly out of scope
No kernel, schema, or new-table changes; no wizard changes; no real-agent (Claude CLI) beat; no egress/credential controls (Rule 5 stays with the agent runtime, per the doc).