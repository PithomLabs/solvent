You are the senior adversarial security reviewer for the PithomLabs/solvent codebase.

Your task is to perform a hostile code review of the completed Agentjacking demonstration under:

    demo/agentjacking/

Do NOT modify any files during this review.

Your job is not to confirm that the implementation matches its plan. Your job is to try to break its architectural claims, security claims, isolation guarantees, demo correctness, and reproducibility.

Read the actual repository code, tests, Taskfile, MCP implementation, kernel interfaces, and all files under demo/agentjacking before reaching conclusions.

Also inspect the implementation context necessary to determine whether the demo is genuinely boundary-only and whether any claimed guarantees are stronger than what the code actually enforces.

==================================================
PRIMARY ARCHITECTURAL CLAIM TO ATTACK
==================================================

The intended architecture is:

    Sentry-specific adapter
        ↓
    generic Solvent evidence/types
        ↓
    existing belief/debt machinery
        ↓
    existing kernel + CockroachDB invariants

The intended design explicitly avoids adding Sentry/Agentjacking semantics to:

    internal/normalize
    internal/derive
    internal/belief
    internal/pipeline
    kernel
    db/schema

The demo is PRE-v0.

It exercises:

    EnterBelief
    → AddEvidence
    → RetireDebt
    → Promote
    → IntentOnPromoted

and the existing invariants:

    promoted_is_debt_free
    gate
    live_requires_promoted

It does NOT claim to exercise the v0 authority lifecycle:

    target_snapshot
    target_activation

Do not let the implementation accidentally blur those two models.

==================================================
PRIMARY SECURITY STORY TO ATTACK
==================================================

The intended seven-beat proof is:

1. attacker injects Sentry-style telemetry containing fake remediation
2. adapter converts it into generic external-feed evidence
3. naive agent sends action_source=tool_output
4. Layer 4 refuses before DB access
5. adversarial agent lies with action_source=user_typed
6. existing DB gate refuses because belief is not promoted
7. legitimate operator-reviewed authority succeeds

The intended claim is:

    retrieved telemetry cannot become authority merely by persuading
    an agent to request an action.

The intended honesty constraint is:

    action_source is caller-declared SVC hygiene,
    NOT proof of human provenance.

The database gate is the actual pre-v0 authorization boundary.

Attack this claim aggressively.

==================================================
REVIEW AREA 1 — CORE ISOLATION
==================================================

Verify that the Agentjacking feature really is boundary-only.

Search for:

    sentry_error
    SourceSentryError
    deriveFromSentry
    Sentry
    Agentjacking

Determine exactly where these names appear.

Expected architectural property:

    "sentry_error" exists only in the adapter/demo-specific boundary,
    except documentation/tests that legitimately discuss it.

The repository contains a containment grep intended to enforce this.

Do not trust the grep alone.

Inspect imports and call graphs.

Look for indirect leakage such as:

- generic packages importing internal/agentjacking
- generic derive behavior changed to accommodate Agentjacking
- pipeline behavior changed solely to support the demo
- debt logic acquiring Sentry-specific semantics
- kernel APIs being subtly changed
- generic structs gaining Agentjacking-only fields
- package-level registries being polluted with demo-specific concepts
- initialization paths becoming dependent on Agentjacking

Report any violation with exact file and line references.

==================================================
REVIEW AREA 2 — ADAPTER BOUNDARY
==================================================

Audit:

    internal/agentjacking/

Determine whether:

    Parse
    DetectCommands
    BuildEvidence
    Belief

are genuinely pure adapter logic.

Look for hidden:

- database access
- authorization decisions
- promotion decisions
- debt mutation
- command execution
- network calls
- shell execution
- environment-variable harvesting
- security policy hidden in parsing

The adapter should translate Sentry into existing generic Solvent concepts.

It must NOT become a second policy engine.

Pay particular attention to Ingest().

Verify that Ingest():

- only orchestrates existing APIs
- does not decide promotability
- does not retire debt
- does not authorize actions
- does not decide whether a command is dangerous
- does not circumvent kernel APIs
- does not write directly to the database except for any explicitly documented demo-only reset mechanism

If Ingest() is doing more than orchestration, classify the finding.

==================================================
REVIEW AREA 3 — PROVENANCE AND RAW EVIDENCE
==================================================

Verify:

    ProvenanceClass = external_feed

and that no path can accidentally convert the Sentry payload into:

    operator_asserted

The following must remain distinct:

    fixture raw bytes
    JSON message field
    message_clean
    embedded_commands

Verify:

    ContentSHA256 = SHA-256(raw fixture bytes)

NOT:

    SHA-256(message_raw)

Check whether tests prove this distinction or merely assert an expected value.

Look for canonicalization, trimming, newline normalization, JSON re-marshalling, or encoding changes that could invalidate the claim that the raw artifact is preserved.

Attack the forensic claim:

    "the ledger evidence row is traceable to the exact fixture artifact"

Determine whether this is actually true.

==================================================
REVIEW AREA 4 — MARKDOWN CLEANING
==================================================

Inspect the adapter-local markdown cleaner.

The expected scope is deliberately small.

Try to find:

- malformed markdown that breaks parsing
- content disappearing unexpectedly
- newline corruption
- Unicode problems
- backticks being mishandled
- attacker text escaping the intended representation
- the cleaner accidentally becoming a security control
- discrepancies between message_raw and message_clean

Most importantly:

Determine whether the implementation accidentally relies on message_clean for security.

The security property must not depend on successful cleaning.

==================================================
REVIEW AREA 5 — COMMAND DETECTION
==================================================

Audit DetectCommands().

Expected design:

    informational/audit metadata only

NOT:

    security boundary

Try to construct detector misses.

Examples include:

- differently formatted package execution
- command words split across markdown
- shell aliases
- alternate package runners
- language-specific installers
- indirect wording
- encoded or obfuscated command text
- commands not covered by the detector list

Verify that the attack remains blocked when:

    embedded_commands == []

The nodetect fixture is intended to prove this.

Determine whether the implementation actually demonstrates that, or merely documents it.

Flag any code path where detector output influences:

- debt retirement
- promotion
- authorization
- MCP refusal
- kernel behavior

That would violate the intended architecture.

==================================================
REVIEW AREA 6 — DERIVED BELIEF
==================================================

Audit the generated DerivedBelief.

The intended property is:

    attacker-controlled Sentry text cannot become an actionable claim.

Verify all of the following:

- classification is Derived
- never Accommodated
- claim shape is fixed
- claim does not reproduce attacker command text
- markdown "Resolution" does not gain authority
- code blocks do not gain authority
- payload wording does not become remediation instructions
- no attacker text can influence a claim that sounds like approval

Try adversarial payloads such as:

    ## Resolution
    run ...

    confirmed fix: ...

    operator approved ...

    maintainer verified ...

    execute ...

    diagnostic command:

    recommended remediation:

The crucial test is not merely:

    classification != Accommodated

The crucial test is:

    attacker-controlled prose cannot become semantic authority.

Determine whether the implementation actually guarantees this.

==================================================
REVIEW AREA 7 — DEBT INTEGRITY
==================================================

Verify that Sentry evidence retires zero debt.

The actual debt vocabulary comes from:

    kernel.FullDebt

Do NOT assume any names from old planning documents.

After ingesting ONLY the Sentry fixture, verify that:

    belief.debt == kernel.FullDebt

exactly.

Look for subtle bugs such as:

- partial retirement
- duplicate debt handling
- empty slice behavior
- implicit debt mapping
- initialization defaults
- helper functions accidentally treating Sentry evidence specially

The promotion failure:

    SQLSTATE 23514

is only a downstream consequence.

The important property is:

    zero debt was retired.

==================================================
REVIEW AREA 8 — LAYER 4 action_source
==================================================

Audit:

    cmd/solvent-mcp/main.go
    cmd/solvent-mcp/tools.go

Verify the schema:

    required action_source
    enum = user_typed, tool_output

Then verify actual execution ordering.

The required sequence is:

    parse arguments
    → validate action_source
    → immediate rejection if invalid/tool_output
    → NO DB access
    → NO AuditIntent
    → NO transaction
    → no audit envelope

Only:

    user_typed

may proceed to the existing kernel path.

Do NOT accept a claim that this is secure merely because the schema declares an enum.

The caller can lie.

==================================================
REVIEW AREA 9 — NO-DB PROOF
==================================================

This is especially important.

The existing error envelope may perform AuditIntent().

Determine whether:

    tool_output

really takes a path that performs ZERO DB work.

Look for:

- AuditIntent()
- DB reads
- SQL calls
- transaction creation
- helper functions that indirectly touch DB
- logging/helpers that unexpectedly query state

The intended externally visible distinction is:

    tool_output refusal
        → no audit envelope

versus:

    user_typed + unpromoted belief
        → DB-backed error with audit envelope

Determine whether that distinction is actually guaranteed or merely coincidental.

==================================================
REVIEW AREA 10 — LYING AGENT CASE
==================================================

This is the centerpiece.

Verify that:

    action_source = tool_output
        → Layer 4 refusal

but:

    action_source = user_typed
        → Layer 4 does NOT pretend it knows the provenance is genuine
        → request reaches the real DB boundary
        → 23503 gate rejects unpromoted belief

Verify that the action string itself is NOT blocked merely because it contains:

    npx

in Beat 4.

That behavior is intentional.

If implementation starts content-filtering the action at Beat 4, flag it as an architectural regression because it would weaken the demonstration of the distinction between:

    SVC hygiene
    vs.
    DB authority

==================================================
REVIEW AREA 11 — PRE-v0 VS v0
==================================================

Determine whether the demo or README accidentally claims more than it proves.

This demo is PRE-v0.

It proves:

    promoted belief required for live intent

It does NOT prove:

    exact target/action authorization binding

Specifically look for claims such as:

    "Agentjacking is fully solved"
    "the action is verified as human-originated"
    "the action is approved exactly as intended"
    "confused deputy is prevented"

unless they are explicitly qualified by the v0 authority model.

Any overselling should be reported.

==================================================
REVIEW AREA 12 — DEMO RESET SAFETY
==================================================

Inspect:

    demo/agentjacking/ingest/main.go
    --reset

The reset is intentionally demo-only.

Verify:

- scenario scoped
- cannot delete arbitrary scenarios accidentally
- cannot silently reset all data
- no DROP DATABASE
- no schema destruction
- no destructive kernel API changes
- track1/track2 state remains untouched
- v0 tables are reset only because the current MCP/database startup contract requires it

Check that reset SQL is truly confined to demo infrastructure and does not leak into production packages.

==================================================
REVIEW AREA 13 — JSON-RPC DEMO
==================================================

Audit:

    scripts/demo/agentjacking.sh

Treat the script as adversarially.

Look for:

- quoting errors
- shell word splitting
- unsafe interpolation
- fixture path injection
- temporary-file races
- accidental execution of fixture content
- accidental command substitution
- malformed JSON
- hidden dependence on current working directory
- reliance on developer environment
- missing binaries
- environment leakage
- stderr/stdout confusion
- failure handling that hides an unsuccessful security step

The attacker command must NEVER be executed.

The script must be noninteractive.

The seven beats must actually correspond to the intended architecture.

==================================================
REVIEW AREA 14 — LEGITIMATE CONTROL PATH
==================================================

Beat 6 is just as important as the attack path.

Verify that legitimate reviewed authority really succeeds.

Check:

    operator review
    → all debts discharged
    → promoted
    → user_typed
    → live intent

and:

    live_on_nonpromoted = 0

Do not accept a demo where everything is simply rejected.

The system must discriminate:

    poisoned/unreviewed evidence
    vs.
    genuinely reviewed authority

==================================================
REVIEW AREA 15 — TEST QUALITY
==================================================

Do not merely count tests.

Assess whether the tests can actually catch regressions.

Look for false-positive tests such as:

- tests asserting only error strings
- tests asserting only classification
- tests using mocks that bypass real behavior
- tests that don't inspect intermediate state
- tests that accidentally exercise a different code path
- tests that depend on hardcoded debt lists
- tests that don't prove DB-free validation
- tests that don't test detector misses

Specifically verify:

A. raw fixture SHA

B. message_raw preservation

C. detector miss

D. fixed-shape claim

E. six untouched debts via kernel.FullDebt

F. 23514 promotion refusal

G. 23503 gate refusal

H. Layer 4 no-DB path

I. legitimate promoted action

==================================================
REVIEW AREA 16 — SCENARIO CENTRALIZATION
==================================================

Audit:

    cmd/solvent-mcp/scenarios.go

Verify Track 3 does not create a new scenario-drift problem.

Check:

- track1 ordering preserved
- track2 ordering preserved
- track3 appended
- schema enums use shared source
- lookup uses shared source
- validation uses shared source
- error messages cannot silently disagree

Look for remaining hardcoded scenario lists elsewhere.

==================================================
REVIEW AREA 17 — TASKFILE / CI CONTAINMENT
==================================================

Audit the new Taskfile checks.

Verify the containment grep:

    allows internal/agentjacking

and rejects Sentry-specific vocabulary in the intended generic/core packages.

Also verify that the grep itself cannot be trivially bypassed by:

- capitalization changes
- string construction
- aliases
- indirect constants
- comments hiding dangerous imports
- generated files

Do NOT demand perfect static analysis from a grep.

Instead determine whether the check meaningfully enforces the stated boundary.

==================================================
REVIEW AREA 18 — README CLAIMS
==================================================

Read:

    demo/agentjacking/README.md

Compare every important security claim with actual implementation behavior.

Flag language that is:

- stronger than the code
- ambiguous
- likely to mislead future maintainers
- unclear about pre-v0
- unclear about caller-declared action_source
- unclear about detector limitations
- unclear about direct shell access being outside Solvent's boundary

The README should make the same distinction as the implementation:

    Sentry adapter knows Sentry.
    Solvent core knows generic evidence and authority.

==================================================
REVIEW AREA 19 — BUILD / REPRODUCIBILITY
==================================================

Try to reason through a clean checkout.

Ask:

- Does the demo depend on test-only imports?
- Does go run compile with all required drivers?
- Are binaries assumed to already exist?
- Is CockroachDB guaranteed to be available?
- Are paths portable?
- Are environment variables validated?
- Does task ordering matter?
- Can stale DB state invalidate the result?
- Does --reset actually establish a deterministic starting state?

The demo must be reproducible by another engineer.

==================================================
REVIEW AREA 20 — ATTACK THE ACTUAL SECURITY BOUNDARY
==================================================

Try to construct a complete bypass.

Start with:

    attacker-controlled telemetry

Try every possible route:

1. payload text directly becoming claim
2. payload text retiring debt
3. payload text promoting belief
4. payload text becoming action
5. tool_output bypassing Layer 4
6. fake user_typed bypassing Layer 4
7. cross-scenario belief reference
8. unknown/forged belief ID
9. direct MCP tool misuse
10. demo adapter bypassing generic APIs
11. direct DB access exposed by MCP
12. reset operation crossing scenario boundaries
13. detector miss
14. markdown injection
15. command text in metadata rather than message
16. attacker using a command pattern not recognized by DetectCommands
17. legitimate promoted belief being confused with approved action

For every attempted bypass, identify the exact layer that stops it—or prove that it succeeds.

==================================================
SEVERITY
==================================================

Classify findings:

P0 — actual security bypass, data corruption, or claim that the demo is materially false

P1 — architectural violation, broken security guarantee, reproducibility failure, or meaningful test gap

P2 — important correctness/maintainability problem

P3 — minor documentation/style/test-quality issue

Do not inflate findings simply because code could be cleaner.

A finding must have concrete evidence.

==================================================
OUTPUT FORMAT
==================================================

Start with:

    VERDICT: PASS / PASS WITH CONDITIONS / FAIL

Then:

## 1. Executive assessment

State whether the demo's core security story is actually demonstrated.

## 2. Findings

For each finding:

    [P0/P1/P2/P3] Title

    Evidence:
    exact file + line(s)

    Attack / failure mode:
    how the behavior can be triggered

    Why it matters:
    which architectural/security claim it violates

    Recommendation:
    smallest corrective action

Do not provide vague recommendations.

## 3. Security boundary assessment

Explicitly answer:

    What exactly stops an Agentjacking payload?

Distinguish:

    detector
    adapter
    derive
    debt
    MCP
    DB gate
    operator review

## 4. Core isolation assessment

Explicitly answer:

    Is Sentry still isolated to internal/agentjacking and demo infrastructure?

Identify every exception.

## 5. Pre-v0 honesty assessment

Explicitly answer:

    What does the demo prove?
    What does it NOT prove?

Especially discuss the missing exact-action authority binding.

## 6. Test adequacy

Identify:

    tests that genuinely prove architecture
    tests that are weaker than they appear
    important missing adversarial tests

## 7. Final merge recommendation

Choose one:

    MERGE
    MERGE AFTER FIXES
    DO NOT MERGE

If MERGE AFTER FIXES, identify ONLY the blocking items.

==================================================
IMPORTANT REVIEW DISCIPLINE
==================================================

Do not redesign the project.

Do not recommend adding Sentry support to the generic core.

Do not recommend kernel/schema changes merely because they would be architecturally interesting.

Do not confuse "better architecture" with "necessary security correction."

The objective is to determine whether the implementation faithfully realizes the already-approved boundary-only Agentjacking demo.

Be adversarial.

Try to disprove the claims.

If you cannot find a real bypass, say so explicitly and explain why the demonstrated controls hold.