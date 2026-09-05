Use this as the **adversarial Phase 4B code-review prompt** for the coding agent:

```text
You are performing a hostile, security-first adversarial code review of the CURRENT Solvent repository after Phase 4B implementation and the subsequent Phase 6.1/6.2 verification fixes.

Your job is NOT to make changes.

Do NOT edit files, do NOT refactor, do NOT “fix” findings, and do NOT redesign the architecture. Review the implementation exactly as it exists in the working tree.

Review objective:

Determine whether Phase 4B is actually safe and architecturally sound, especially at the authority boundary, service/API boundary, transaction boundary, identity boundary, and MCP/REST convergence boundary.

The standard is adversarial:
Assume an attacker, malicious agent, confused deputy, stale client, buggy integration, or concurrent transaction will exploit every ambiguity possible.

Do not give a generic code-quality review.
Do not praise the implementation unless supported by concrete evidence.
Do not accept comments/documentation as proof when runtime behavior says otherwise.

==================================================
SOURCE OF TRUTH
==================================================

Use these repository artifacts as the governing design references:

1. docs/OS/phase4a_api_contract.md
   - Canonical REST/API contract
   - Security and forbidden-surface requirements

2. The Phase 4B implementation plan / approved revision
   - Reconstruct the intended Phase 4B architecture and acceptance criteria
   - Pay special attention to:
     * real Service Layer
     * kernel as sole authority engine
     * AuthorizeAndCreateIntent
     * MCP convergence
     * actor binding
     * no production executor
     * no workflow tokens
     * no schema growth
     * no second authorization engine

3. Phase 6.1 / Phase 6.2 work
   - Treat these as verification/test-infrastructure changes unless runtime code shows otherwise
   - Verify that their changes did not weaken Phase 4B security semantics

4. Current repository HEAD / working tree
   - Runtime code is authoritative over plans/comments when they disagree

==================================================
FIRST: ESTABLISH THE REVIEW SURFACE
==================================================

Before judging correctness:

1. Inspect git status and git diff.
2. Identify every production file changed by Phase 4B.
3. Identify every production file changed by Phase 6.1.
4. Identify every test-only / test-infrastructure file changed by Phase 6.2.
5. Distinguish:
   - intentional Phase 4B implementation
   - verification/test-infrastructure changes
   - pre-existing code
   - unrelated changes
6. Do not miss hidden or indirect production paths.

Build a concise map like:

REST/API
  → service
  → kernel
  → CockroachDB

MCP
  → service
  → kernel
  → CockroachDB

Then verify that this is actually true in code.

==================================================
CORE SECURITY PRINCIPLE
==================================================

The fundamental Solvent invariant is:

    retrieval/evidence ≠ authority
    belief/policy/advisory output ≠ authority
    token ≠ authority
    intent ≠ authority
    authorization ≠ execution

The kernel's authoritative state must determine whether a consequential action is allowed.

Find any path where a caller-controlled value, cached decision, stale decision, advisory result, workflow artifact, token, UI/API field, or service-layer shortcut can substitute for authoritative kernel state.

==================================================
1. AUTHORITY ENGINE — SINGLE SOURCE OF TRUTH
==================================================

Verify that there is exactly ONE implementation of authority semantics.

Search for:

- duplicated authority SQL
- duplicated target activation/revocation logic
- duplicated target snapshot resolution
- duplicated justification validation
- duplicated tuple comparison
- service-layer authority queries
- API-layer authority decisions
- MCP-specific authorization logic
- hidden “is allowed” predicates
- alternate permission checks

Specifically determine:

A. Is kernel.Authorize the canonical authority oracle?

B. Is kernel.Approve the authority-creating semantic operation?

C. Is kernel.RevokeTarget the authoritative revocation path?

D. Does AuthorizeAndCreateIntent reuse the SAME authority implementation as Authorize rather than reimplementing it?

E. Does any service/API/MCP path independently decide that an action is allowed?

If duplication exists, classify it as:
- harmless read/projection
- security-relevant duplication
- second authority engine

Do not accept “documented duplication” as automatically safe.

==================================================
2. AuthorizeAndCreateIntent ATOMICITY
==================================================

Audit kernel.AuthorizeAndCreateIntent line by line.

Prove or disprove the invariant:

    authority evaluation
        +
    intent creation
        =
    one SERIALIZABLE transaction

Verify:

- same *sql.Tx is used
- authorizeWithinTx is the sole authority evaluator
- createIntentWithinTx is the sole intent creation implementation
- no hidden nested transaction exists
- no operation commits between the authority read and intent insert
- denied authority cannot create an intent
- errors roll back intent creation
- retry behavior is correct under CockroachDB serialization failures
- result state cannot claim “live” when the transaction did not commit

Pay special attention to:

- target revocation between authority resolution and intent insertion
- belief promotion/retraction races
- principal revocation races if applicable
- retry behavior
- whether the closure is safe to re-run
- whether any state stored outside the DB transaction can become stale

Do not merely cite the existence of SERIALIZABLE.
Show why the actual read/write dependency forces the forbidden stale-authority commit to abort/retry.

==================================================
3. CONCURRENCY / TOCTOU
==================================================

Actively reason through adversarial schedules.

At minimum analyze:

Scenario A:
    authorize starts
    revoke target commits
    authorize attempts intent insert

Scenario B:
    authorize reads
    concurrent revoke commits
    Cockroach retries authorize transaction

Scenario C:
    transaction retries after a serialization error
    closure reuses stale in-memory state

Scenario D:
    target approval/revocation changes while a request is in flight

Scenario E:
    concurrent duplicate authorize-action requests

For each:
- Can a stale authority decision commit?
- Can a live intent exist after authority is revoked?
- Can the same action be committed twice?
- Is duplicate behavior merely a product concern or a security concern?

==================================================
4. ACTOR / IDENTITY BINDING
==================================================

Audit the Phase 6.1 identity fix in production code.

Required invariant:

    authenticated credential
        ↓
    AuthenticatedPrincipal
        ↓
    effective PrincipalID
        ↓
    AuthorityTuple.PrincipalID

Verify:

- API key maps deterministically to a real principal ID
- request actor_id cannot override authenticated identity
- conflicting actor_id is rejected
- VerifyAuthorization binds to authenticated identity
- AuthorizeAction binds to authenticated identity
- no handler accidentally trusts caller PrincipalID
- no alternate API field can perform the same identity override
- no “system key” special case permits arbitrary impersonation
- no dead-code condition suggests stronger semantics than implementation actually provides
- authentication context cannot be forged through request JSON

Also distinguish:
- identity
- actor metadata
- authentication
- future delegation

Check that delegation has not accidentally been introduced through actor_id.

==================================================
5. API SECURITY BOUNDARY
==================================================

Review every REST endpoint against the Phase 4A contract.

For every mutating endpoint verify:

- authentication occurs before security-sensitive operation
- handlers do not call kernel directly where service boundary is required
- request fields cannot override authoritative server state
- IDs are validated
- malformed UUIDs fail safely
- unauthorized operations fail closed
- missing security context cannot panic
- caller-controlled fields do not become privileged selectors
- HTTP error mapping does not leak internal state unnecessarily
- no endpoint exists that bypasses authority

Search for forbidden or dangerous concepts such as:

- execute without authority
- set authorized
- force execution
- override authorization
- approve-and-execute
- arbitrary executor selection
- caller-selected principal
- caller-selected approval state
- direct intent creation
- direct snapshot/activation mutation

Compare the actual route table with the canonical API contract.
Flag:
- undocumented endpoints
- missing required security restrictions
- semantic drift
- accidental exposure of internal tables/resources

==================================================
6. MCP / REST CONVERGENCE
==================================================

Verify that REST and MCP use the same service/kernel semantics.

Required shape:

REST
  → Service
  → Kernel

MCP
  → same Service
  → same Kernel

For authorize-action specifically, verify both use:

    AuthorizeAndCreateIntent

Check for:

- MCP-specific authority implementation
- old PrepareForAction + IntentOnPromoted path
- direct IntentOnPromoted after a separate authorization
- MCP-only bypasses
- inconsistent action/target semantics
- inconsistent actor semantics

Remember:
MCP is a local stdio trust boundary.
Do NOT demand HTTP-style authentication inside MCP.
Do demand kernel authority enforcement.

==================================================
7. SERVICE LAYER
==================================================

Determine whether the Service Layer is genuinely architectural or merely decorative.

Verify:

- handlers delegate through services
- services do orchestration/composition
- service does not become a second authority engine
- service does not silently bypass kernel
- service does not mutate authority-core tables directly
- audit coordination is actually performed where intended
- service does not invent authority semantics

Pay particular attention to AuthorizeAndCreateIntent:
- does service call the atomic kernel primitive?
- or does it duplicate SQL?
- can any alternate service method bypass it?

==================================================
8. EXECUTION BOUNDARY
==================================================

Search the entire repository for actual consequential external execution.

Verify:

- there is no hidden production executor
- ExecuteAction does not accidentally cause real side effects
- no API route reaches an external provider without current authorization
- no executor can approve/promote/revoke authority
- no caller-controlled tool_name or executor identifier becomes arbitrary capability selection
- authorization happens immediately before execution using current state
- intent is not treated as authority by itself

If no real production executor exists, record that as scope status, not as a vulnerability.

Also verify the architecture does NOT falsely claim atomic coordination between:
    DB authorization
and
    external provider execution

The correct boundary is:
    successful current authorization
    immediately followed by executor invocation

Do not invent stronger guarantees than the implementation provides.

==================================================
9. DATABASE / INVARIANTS
==================================================

Inspect all authority-related SQL used by Phase 4B.

Verify:

- authority tables are only mutated through intended kernel semantics
- target activation remains once-ever
- revocation remains append-only
- target snapshot remains immutable
- justification semantics are preserved
- unpromoted beliefs cannot produce live intents
- direct INSERT/UPDATE paths cannot bypass gates
- SQLSTATE handling remains fail-closed
- retry behavior does not weaken invariants

Check for race-prone patterns such as:

    SELECT says allowed
       ↓
    separate INSERT/UPDATE

where atomicity is required.

Also search for any new schema changes introduced by Phase 4B.

==================================================
10. ERROR HANDLING / FAIL CLOSED
==================================================

For every security-sensitive failure verify:

- no panic
- no partial commit
- no “success” response after failed authorization
- no live intent on denial
- malformed input cannot escape validation
- unknown target/belief cannot become an authorized action
- SQLSTATE mappings do not convert a denial into success
- retryable errors are not swallowed incorrectly

Inspect especially:

- nil auth context
- nil service dependencies
- malformed target IDs
- malformed actor IDs
- unknown beliefs
- inactive/revoked targets
- unpromoted beliefs
- duplicate operations
- missing justifications

==================================================
11. AUDIT SEMANTICS
==================================================

Verify the distinction:

    authorize != execute

and:

    kernel authority state != product audit log

Check:

- mutations produce intended audit records
- audit does not become the authority source
- authorization decisions are distinguishable from execution events
- provider calls are distinguishable from Solvent authorization
- an audit failure cannot falsely report a successful authority commit as failed
- post-commit audit semantics do not create misleading claims
- append-only audit behavior is preserved

If there is a commit-to-audit crash gap, identify it explicitly rather than pretending the two are atomic.

==================================================
12. TOKEN / WORKFLOW BYPASS
==================================================

Search for:

- workflow_token
- stale tokens
- JWT/HMAC introduced for convenience
- cached authority decisions
- prepared execution tokens
- client-held security state
- workflow continuity objects treated as authority

Verify:

    TOKEN != AUTHORITY

and that consequential operations re-read current authoritative state.

Do not recommend introducing a token merely to solve a test or API convenience problem.

==================================================
13. KERNEL MINIMALISM
==================================================

Apply the kernel-growth rule:

Only add kernel primitives when a new durable security fact or atomic security-critical state transition cannot be represented safely outside the kernel.

For every Phase 4B kernel change ask:

1. Is it genuinely security-critical?
2. Is it durable semantics?
3. Is it an atomic state transition?
4. Could it safely live in service/policy/adapter instead?
5. Does the new primitive reduce or increase trusted-core complexity?
6. Is authority still represented in one place?

In particular, evaluate AuthorizeAndCreateIntent against the existing architecture.

Do NOT recommend kernel expansion merely for convenience.

==================================================
14. PHASE 6.1 / 6.2 INTEGRITY
==================================================

Verify that the test-infrastructure changes did not weaken production security.

Check:

- actor binding remains enforced
- API authentication still maps to real principal IDs
- MCP tests initialize the real ledger service
- tests exercise real production authority paths
- test fixtures do not bypass kernel invariants
- no security test was deleted or diluted merely to obtain green builds

Inspect per-package database isolation.

Verify the test infrastructure does not conceal races.

==================================================
15. TEST QUALITY
==================================================

Do not accept “tests pass” as proof.

Inspect whether the tests actually exercise the security property they claim to test.

For important tests, verify:
- positive path
- negative path
- malformed input
- stale state
- concurrent state change
- duplicate request
- cross-target mismatch
- actor mismatch

Specifically inspect:

- TestIntegration_AuthorizeAction_Atomicity
- TestIntegration_AuthorizeAction_ActorIDMismatch
- TestIntegration_ConcurrentRevokeTarget
- relevant kernel authority tests
- relevant MCP agentjacking tests

Check for false positives caused by:
- early returns
- nil dependencies
- tests not reaching the DB
- assertions checking only strings
- tests depending on package order
- cached test results
- shared mutable test databases

==================================================
16. HUNT FOR HIDDEN BYPASS PATHS
==================================================

Search globally for all references to:

- Authorize
- AuthorizeAndCreateIntent
- IntentOnPromoted
- Approve
- RevokeTarget
- target_activation
- target_snapshot
- target_revocation
- action_intent
- ExecuteAction
- executor
- actor_id
- principal_id
- authorization
- approve
- execute

Build a short call graph for every consequential path.

The question is:

“Can an attacker reach a real consequential action without passing through the intended current-state authority check?”

If yes, demonstrate exactly how.

==================================================
17. RUN VERIFICATION
==================================================

You may execute commands for review, but do not modify source.

Run:

    git status --short
    git diff --check

Then:

    go test -count=1 ./...
    go test -count=1 ./...
    go test -count=1 -p 1 ./...
    go build ./...
    go vet ./...

If practical, also run the critical security tests individually with -v.

Do NOT treat a cached result as fresh evidence.

If the repository requires CockroachDB, verify the tests actually hit the intended isolated test databases.

==================================================
18. FINDING CLASSIFICATION
==================================================

Classify every finding:

CRITICAL
- direct authority bypass
- consequential action possible without valid current authority
- stale authority can commit
- identity impersonation
- second authority engine creating inconsistent security semantics

HIGH
- exploitable security boundary weakness
- race with realistic stale-authority consequences
- broken fail-closed behavior
- hidden privileged path

MEDIUM
- defense-in-depth weakness
- meaningful integrity/audit ambiguity
- dangerous but currently unreachable path

LOW
- minor hardening
- misleading security documentation
- non-exploitable robustness issue

INFO
- architectural observation only

Also classify each finding as:

- INTRODUCED BY PHASE 4B
- PRE-EXISTING
- PHASE 6.1/6.2 TEST-INFRASTRUCTURE ONLY
- UNRELATED

Do not inflate pre-existing issues into Phase 4B findings.

==================================================
19. REQUIRED OUTPUT
==================================================

Return a structured adversarial review with exactly these sections:

1. VERDICT

One of:

    GO
    GO WITH CONDITIONS
    NO-GO

2. EXECUTIVE ASSESSMENT

No more than 10 concise paragraphs.
State whether the Phase 4B security model actually holds.

3. CRITICAL / HIGH FINDINGS

For each finding provide:

    ID
    Severity
    Classification
    File + exact line/range
    Vulnerability
    Concrete exploit/concurrency scenario
    Why existing controls do not prevent it
    Required remediation
    Whether remediation requires kernel/schema/API redesign

4. MEDIUM / LOW / INFO FINDINGS

Same evidence standard, but concise.

5. VERIFIED INVARIANTS

Explicitly state which of these were proven:

    - kernel is sole authority engine
    - Authorize is read-only
    - Approve creates authority
    - RevokeTarget revokes authority append-only
    - AuthorizeAndCreateIntent is atomic
    - stale revocation cannot produce a committed live intent
    - authenticated principal controls API authority identity
    - actor_id cannot override authenticated identity
    - REST and MCP converge on the same authority path
    - no workflow token is authority
    - no production executor bypass exists
    - no hidden consequential execution path exists
    - tests exercise the intended security paths

6. FAILED / UNPROVEN INVARIANTS

Do not mark an invariant PASS unless the code and tests support it.

7. TEST EVIDENCE

Report exact commands run and their results.
Distinguish fresh runs from cached runs.

8. PHASE 4B DISPOSITION

State one of:

    READY TO FREEZE
    READY WITH SPECIFIC FIXES
    BLOCKED — SECURITY
    BLOCKED — CORRECTNESS
    BLOCKED — VERIFICATION

==================================================
FINAL REVIEW RULE
==================================================

Do not redesign Solvent because you found something imperfect.

The goal is to determine whether the CURRENT Phase 4B implementation preserves the established architecture:

    Client / Protocol
          ↓
    Service Layer
          ↓
    Small Trusted Kernel
          ↓
    CockroachDB

with:

    one authority engine
    one current-state authority decision
    one atomic authorize+intent transition
    no identity ambiguity
    no MCP bypass
    no production execution bypass

Be adversarial, concrete, and evidence-driven.

Most importantly:

DO NOT CHANGE CODE.
DO NOT FIX FINDINGS.
DO NOT DECLARE GO merely because the test suite is green.
DO NOT DECLARE NO-GO merely because a future enhancement could exist.

Find real vulnerabilities, prove them, separate introduced defects from pre-existing conditions, and give a final Phase 4B security disposition.
```

One addition I strongly recommend: have the agent inspect the **current git diff against the Phase 4B baseline**, rather than reviewing the repository as an undifferentiated whole. That prevents Phase 6.2 test-infrastructure work from being accidentally treated as part of the Phase 4B production security surface.
