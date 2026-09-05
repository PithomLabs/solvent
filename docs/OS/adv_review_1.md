# Adversarial Architecture Review — Solvent

**Date:** 2026-09-04  
**Reviewer:** Kilo (adversarial)  
**Scope:** Full repository — schema, kernel, service layer, adapters, MCP, wizard, demo, tests  
**Trigger:** plan3_review.md — do not proceed to Phase 4/6 without passing this review  
**Status:** HOLD — critical and high findings require remediation before Phase 4/6

---

## Executive Summary

The kernel is small, correct, and well-tested. The authority-core schema enforces its invariants. However, the execution path was never connected to the authority verification layer. An agent can currently execute actions on promoted beliefs without any valid authority target, approval, or exact binding verification. This is exactly the "second authority engine" failure mode the plan warned against.

**Final verdict: HOLD.**

---

## 1. Kernel Boundary

**Kernel packages identified:**
- `kernel/` — sole authority-core package. Contains all 7 write sites (`crdb.ExecuteTx`) and the authority read path.

**Authority-core tables:**
- `belief`, `belief_edge`, `evidence`, `action_intent`
- `principal`, `authority_target`, `target_snapshot`, `target_activation`, `target_revocation`, `justification`, `debt_discharge`

**Product-layer tables:**
- `workflow_token`, `policy_tool`, `policy_actor`, `audit_activity`
- Plus corpus/wizard tables: `corpus_issue`, `belief_corpus_citation`, `refusal_log`

**Finding:** Table classification is clean. `workflow_token` contains no `authority_id`, `snapshot_id`, `approval_status`, `debt`, `evidence_state`, or `authorization` columns. It is structurally non-authoritative.

**Critical finding:** The service layer (`service/authority`) is **dead code**. Zero production packages import it. The wizard, MCP tools, and demo all instantiate `kernel.New(db)` directly. The planned service boundary does not exist in the execution path.

---

## 2. Database Audit

| Table | Purpose | Layer | Authoritative? | Danger |
|---|---|---|---|---|
| belief | Kernel belief state | Authority core | Yes | None — frozen schema |
| belief_edge | Derivation/contradiction edges | Authority core | No (audit) | None |
| evidence | Evidence rows | Authority core | No (input) | None |
| action_intent | Live intents gated by belief status | Authority core | Yes (via gate FK) | None |
| principal | Actor identity | Authority core | Yes | None |
| authority_target | Authority proposal | Authority core | Yes | None |
| target_snapshot | Immutable approved authority | Authority core | Yes | None |
| target_activation | Authority-granting fact | Authority core | Yes | None |
| target_revocation | Append-only revocation | Authority core | Yes | None |
| justification | Target-to-belief link | Authority core | No (audit) | None |
| debt_discharge | Discharge attribution | Authority core | No (audit) | None |
| workflow_token | Workflow state | Product | No | Correctly separated |
| policy_tool | Tool registry | Product | No | None |
| policy_actor | Actor registry | Product | No | None |
| audit_activity | Activity log | Product | No | None |

**Finding:** `007_service_tables.sql` is genuinely product-layer persistence. `workflow_token` does not contain authority fields. The danger is not the table itself — it is that the execution path was never built to read current authority from the authority-core tables at all.

---

## 3. Token Attack

**Token lifecycle traced:**
- **Creation:** `workflow.Service.CreateToken` — inserts `workflow_token` with `belief_id`, `action_type`, `state`, `payload`
- **Signing:** **No signing.** The token is a database row, not a JWT or signed blob.
- **Parsing/validation:** `GetToken` reads the row; `transition` validates state machine
- **Storage:** `workflow_token` table
- **Retrieval:** `GetToken`, `ListTokensByScenario`
- **Execution:** `service/authority.Execute` reads token, runs executor

**Attacks attempted:**

1. **valid token + revoked authority** — Token contains no authority reference. No code path re-reads `target_revocation`. **RESULT: token executes. NOT DENIED. CRITICAL.**
2. **valid token + changed target** — No code path re-reads `target_snapshot`. **RESULT: token executes. NOT DENIED. CRITICAL.**
3. **valid token + changed action** — No code path re-reads `target_snapshot`. **RESULT: token executes. NOT DENIED. CRITICAL.**
4. **valid token + changed policy** — Policy is re-read in `Prepare`, but policy alone does not gate authority. **RESULT: policy may allow, authority absent. EXECUTES. CRITICAL.**
5. **valid token + changed actor** — Actor is re-read in `Prepare`, but actor alone does not gate authority. **RESULT: actor may match, authority absent. EXECUTES. CRITICAL.**
6. **valid token + stale approval** — Approval is never re-read. **RESULT: executes. NOT DENIED. CRITICAL.**
7. **forged token** — Token IDs are UUIDs with no signature. Any client can construct a token ID and insert a row if they have DB access. **RESULT: no cryptographic binding. HIGH.**
8. **tampered token** — Same as forged. No HMAC, no signature. **RESULT: HIGH.**
9. **expired token** — `expires_at` column exists but is never checked. No code path reads it. **RESULT: expired tokens execute. HIGH.**
10. **invalid stage transition** — State machine is enforced correctly. **RESULT: DENIED. PASS.**
11. **valid signed stale token + revoked current authority** — Token is not signed. Current authority is never re-read. **RESULT: DENIED. FAIL. CRITICAL.**

**Most important adversarial test (plan3.md §30):**
```
valid signed stale token + revoked current authority = DENIED
```
**Result: FAIL.** The execution path does not re-read `target_revocation`, `target_activation`, or `target_snapshot`. The token does not carry authority (correct), but nothing re-reads it from the database either.

---

## 4. Current-State Authorization

**Execution path traced (production):**

```
wizard handlePromote
  → s.Promote(ctx, sid, bid)
    → s.kern.Promote(ctx, beliefID)

wizard handleAuthorize
  → s.Authorize(ctx, sid, bid)
    → s.kern.IntentOnPromoted(ctx, scenarioID, beliefID, DeployAction)

MCP solvent_authorize_action
  → st.IntentOnPromoted(ctx, scenarioID, beliefID, action)
```

**Database reads immediately before authorization/execution:**

- `wizard.Server.Authorize`: **None.** Calls `kernel.IntentOnPromoted` directly.
- `service/authority.Prepare` (dead code): reads belief status, policy_tool, policy_actor.
- `service/authority.Execute` (dead code): reads nothing.

**Verified missing re-reads at execution time:**
- belief: re-read in Prepare (status only, not evidence or contradictions)
- evidence: **never re-read**
- debt: re-read in Prepare only for policy `missingBeliefs` check (not for authority)
- contradictions: **never re-read**
- current authority: **never re-read**
- policy: re-read in Prepare
- actor/action/target relationship: partial (actor class, tool name)

**Caches/tokens/values that could bypass current database state:**
- `workflow_token.payload` — contains `tool_name` and `actor_id`, never re-validated against DB
- `workflow_token.state` — transitions without re-reading belief or authority
- Browser state / frontend workflow state — no re-validation

**Plan3.md §12 required:**
> Preparation must re-read: current belief, current evidence projection, current debt, contradictions, current authority, current policy, actor/action/target relationship.

**Current implementation:** Re-reads belief status and policy only. Never reads current authority.

---

## 5. Actor / Authentication Attack

**Actor handling traced:**
- `principal.principal_type` ∈ {`human`, `agent`, `workload`, `service`}
- `policy_actor` has `name`, `roles`, `max_class`, `active`
- MCP `action_source` blocks `tool_output` but is client-declared

**Attack: `actor=HUMAN` in request body without trusted principal context**

In `handleSolventAuthorizeAction`:
```go
actionSource, _ := args["action_source"].(string)
switch actionSource {
case "tool_output":
    return errorResult(...) // blocked
case "user_typed":
    // valid — continue
}
```

`action_source` is a client-declared string. It is not authenticated. An adversarial agent can pass `"user_typed"`.

**Finding:** ActorType ≠ Identity ≠ Authentication. The plan explicitly warns: "Never treat actor = HUMAN in an untrusted request body as proof of human identity." The current code treats `action_source = "user_typed"` as a provenance signal with no cryptographic or session authentication backing it. **Severity: HIGH.**

**However:** The kernel's `Approve` validates `approver_principal_id` exists and is not revoked. The MCP `solvent_approve` requires `approved_by` (a principal UUID). Human-only operations are gated at the kernel level for approval, but the execution path does not enforce human-only boundaries.

---

## 6. Policy Attack

**Policy service traced:**
```go
func (s *Service) AuthorizeToolCall(ctx context.Context, actorID, toolName string, promotedBeliefs []string) (bool, []string, error) {
    tool, _ := s.GetTool(ctx, toolName)
    actor, _ := s.GetActor(ctx, actorID)
    if !actor.Active { return false, nil, ... }
    if !classWithinLimits(tool.Class, actor.MaxClass) { return false, nil, ... }
    missing := missingBeliefs(tool.RequiredBeliefs, promotedBeliefs)
    if len(missing) > 0 { return false, missing, nil }
    return true, nil, nil
}
```

**Attacks attempted:**

1. **policy=ALLOW, authority=ABSENT**
   - `service/authority.Prepare` calls `policy.AuthorizeToolCall` and treats `allowed=true` as sufficient.
   - It then transitions the token to `prepared` and returns `PreparedAction`.
   - `Execute` runs the executor with no authority check.
   - **Expected: DENIED. Actual: EXECUTES. CRITICAL.**

2. **policy=DENY, authority=VALID**
   - If policy denies, `Prepare` returns an error. Execution does not happen.
   - **Expected: DENIED. Actual: DENIED. PASS** — but only because policy is the only gate.

3. **policy=ALLOW, authority=VALID, wrong target**
   - Authority is never checked, so target mismatch is invisible.
   - **Expected: DENIED. Actual: EXECUTES. CRITICAL.**

**Finding:** Policy constrains operations but the execution path treats policy permission as authorization. Policy may not manufacture authority. The current implementation does exactly what the plan forbids.

---

## 7. Evidence Attack

**Evidence quality scoring traced:**
```go
func (s *Service) ComputeQuality(ctx context.Context, beliefID string) (*QualityScore, error)
```

- Computes A-F grade from evidence count, provenance diversity, reproducible artifact, external feed, operator asserted.
- Returns `QualityScore` as a projection.
- **Never called by promotion, authorization, or execution paths.**

**Attacks attempted:**

1. **score=A, authority=ABSENT**
   - `ComputeQuality` is never called in the execution path.
   - **Expected: DENIED. Actual: DENIED (because belief must be promoted, not because of score). PASS** — but for the wrong reason.

2. **score=F, authority=VALID**
   - Same: score is never consulted.
   - **Expected: policy-dependent authorization. Actual: EXECUTES (if belief is promoted and policy allows). PASS** — score has no influence.

**Finding:** Evidence scoring is a pure product-layer projection. It does not influence promotion, authority creation, authorization, or execution. This is correct per the plan. **No finding.**

---

## 8. Workflow Drift

**Approved conceptual workflow (plan3.md §9):**
```
INVESTIGATING → EVIDENCE_REVIEW → HUMAN_REVIEW → APPROVED
    → AUTHORIZATION_READY → EXECUTION → COMPLETED
HUMAN_REVIEW → REJECTED
any non-terminal state → CANCELLED
```

**Implemented workflow (`service/workflow/workflow.go`):**
```
pending → prepared → executing → completed/failed
pending → expired
```

**State comparison:**

| Approved | Implemented | Status |
|---|---|---|
| INVESTIGATING | pending | Partial — `pending` is a placeholder |
| EVIDENCE_REVIEW | collapsed into `prepared` | **DRIFT** |
| HUMAN_REVIEW | collapsed into `prepared` | **DRIFT** |
| APPROVED | collapsed into `prepared` | **DRIFT** |
| AUTHORIZATION_READY | collapsed into `prepared` | **DRIFT** |
| EXECUTION | executing | Match |
| COMPLETED | completed | Match |
| REJECTED | missing | **DRIFT** |
| CANCELLED | missing | **DRIFT** |

**Finding:** The implemented workflow has aggressively simplified the state machine. `prepared` is the result of belief promotion + policy check, not actual human review, approval, or authority establishment. The plan explicitly states: "Do not replace belief lifecycle, authority lifecycle, or action_intent lifecycle with workflow state." The current `prepared` state functions as "we think this is approved" rather than "the workflow is ready to perform preparation/revalidation." **Severity: HIGH.**

---

## 9. Executor Attack

**Executor traced:**
```go
// service/executor/executor.go (dead code)
type ActionFunc func(ctx context.Context, params map[string]interface{}) (string, error)
```

**Production execution path:**
- Wizard: `wizard.Server.Authorize` creates intent; no executor abstraction
- MCP: `solvent_authorize_action` creates intent; no executor abstraction
- `service/authority.Execute` (dead code) takes `executorFn` and runs it

**Attacks attempted:**

1. **executor can approve?** — No code path calls `kernel.Approve` from an executor.
2. **executor can promote?** — No code path calls `kernel.Promote` from an executor.
3. **executor can revoke?** — No code path calls `kernel.RevokeTarget` from an executor.
4. **executor can create authority?** — No code path calls `kernel.Approve` from an executor.
5. **executor can create authority-bearing evidence?** — No. Evidence is created via `kernel.AddEvidence` in the pipeline, not the executor.
6. **valid authorization + provider failure** — Since no execution path verifies authorization, this test is moot. But `service/authority.Execute` (dead code) does record success/failure separately in `workflow_token`.

**Finding:** Executor cannot create authority. **PASS.** However, since `service/authority` is dead code, there is no single executor contract in the production path. **Severity: MEDIUM** — missing abstraction, not a security hole.

---

## 10. External Provider Attack

**GitHub adapter traced:**
```go
// adapter/github/github.go
func (a *Adapter) ProcessEvent(ctx context.Context, event Event) (*NormalizedEvidence, error)
```

- Produces `NormalizedEvidence` with `ProvenanceClass: "external_feed"`.
- Never creates beliefs, never calls kernel.
- `ParseWebhook` parses raw payload into `Event`.

**Attacks attempted:**

1. **provider claims approval** — GitHub events become evidence, not authority. **RESULT: cannot create approval. PASS.**
2. **provider claims authorization** — Same. **RESULT: cannot create authorization. PASS.**
3. **malformed payload** — `ParseWebhook` returns error. Pipeline does not process. **RESULT: no ledger mutation. PASS.**
4. **contradictory payload** — Pipeline logs warning, no ledger mutation. **RESULT: no contradiction recorded. PASS** (but see Finding F-5 in findings2.md about contradiction target identification).
5. **provider reports successful execution when it actually failed** — The adapter does not execute. Execution is outside its scope. **RESULT: N/A. PASS.**

**Finding:** GitHub adapter output cannot become Solvent authority. **PASS.**

---

## 11. Adapter Boundary

**Finding:** The GitHub adapter imports only `context`, `crypto/sha256`, `encoding/json`, `fmt`, `time`. It has zero imports of `kernel`, `belief`, `action_intent`, or any authority-core package. All GitHub semantics (push, PR, CI status, deployment, issue, comment) are translated to generic `NormalizedEvidence`. **PASS.**

---

## 12. Audit Integrity

**Audit traced:**
```go
// service/audit/audit.go
func (s *Service) Log(ctx context.Context, entry *ActivityEntry) error
func (s *Service) LogRefusal(ctx context.Context, scenarioID string, activityType ActivityType, subjectID, sqlstate, constraintName string, details map[string]interface{}) error
```

- `audit_activity` records: `scenario_id`, `type`, `actor_id`, `subject_id`, `details`, `sqlstate`, `constraint_name`, `refusal`, `created_at`.

**Separate records verified:**
- Solvent authorization: Not recorded in `audit_activity` in production paths. Wizard records refusals in `refusal_log`, not `audit_activity`.
- Adapter invocation: Not recorded.
- Provider response: Not recorded.
- Executor result: Not recorded in `audit_activity`. `service/authority.Execute` (dead code) records workflow completed/failed.

**Finding:** The `audit_activity` table is product-layer and correctly separated from authority-core tables. However, the production execution paths (wizard, MCP) do not record authorization, adapter, provider, or executor outcomes in a unified audit trail. The wizard uses `refusal_log` for refusals only. **Severity: MEDIUM.**

---

## 13. Security Test Coverage

| Attack | Existing Test | Location | Expected | Actual |
|---|---|---|---|---|
| agent cannot invoke human-only operation | **NO TEST** | — | DENIED | Untested |
| agent cannot approve itself | **NO TEST** | — | DENIED | Untested |
| fake approval denied | **NO TEST** | — | DENIED | Untested |
| no authority denied | **NO TEST** | — | DENIED | Untested |
| open debt denied | `promoted_is_debt_free` | `kernel/authority_test.go` T-12 | DENIED | PASS |
| wrong target denied | **NO TEST in execution path** | kernel tests T-16 only | DENIED | Untested in execution |
| revoked authority denied | **NO TEST in execution path** | kernel tests T-20 only | DENIED | Untested in execution |
| stale state denied | **NO TEST** | — | DENIED | Untested |
| stale workflow token denied | **NO TEST** | — | DENIED | Untested |
| manipulated token rejected | **NO TEST** | — | DENIED | Untested |
| browser state cannot create authority | **NO TEST** | — | DENIED | Untested |
| executor cannot create authority | **NO TEST** | — | DENIED | Untested (but verified by inspection) |
| provider cannot create authority | **NO TEST** | — | DENIED | Untested (but verified by inspection) |

**Mandatory test per plan3.md §30:**
```
valid signed stale token + revoked current authority = DENIED
```
**Status: DOES NOT EXIST.**

---

## 14. Kernel Growth

**Before/after comparison:**

- New kernel files: None.
- Modified kernel files: None since frozen architecture.
- New kernel functions: None.
- New authority-core schema: `005_authority_mvp.sql`, `006_authority_justification_cascade.sql` — the 7 authority objects.
- New DB invariants: `UNIQUE(target_id)` on `target_activation`, composite FK `(target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)`, `UNIQUE(target_id, belief_id, belief_status)` on `justification`.

**Kernel change evaluation:**
- Can adapter solve it? The authority tables are not adapter concerns.
- Can service solve it? The authority lifecycle requires transactional atomicity that the schema enforces.
- Can policy solve it? No — authority is not policy.
- Can configuration solve it? No.

**Finding:** Authority-core schema additions are justified. They implement the 7-object authority model from the plan. **PASS.**

**Service-layer additions:** `service/authority`, `service/workflow`, `service/executor` are dead code. They represent planned Phase 1/2 boundaries that were never wired into the execution path. Their existence is not a security risk, but their absence means the execution path has no service boundary.

---

## 15. Findings Classification

### CRITICAL

**F-1: Execution path never verifies authority**
- **Location:** `internal/wizard/refusal.go:131-135`, `cmd/solvent-mcp/tools.go:241-244`, `service/authority/authority.go:77-144` (dead code)
- **Attack:** Agent with a promoted belief and valid policy permission can create a live intent and execute without any valid authority target, approval, or exact target/action binding.
- **Why current implementation permits it:** All production execution paths call `kernel.IntentOnPromoted` directly. `kernel.Authorize` is only called in MCP tool `solvent_authorize` (read-only verification) and tests. No execution path calls it.
- **Expected architecture:** Execution path must call `kernel.Authorize` (or equivalent) to verify current, non-revoked authority with exact tuple match before allowing execution.
- **Recommended correction:** Wire `service/authority` (or equivalent) into the execution path. `PrepareForAction` must re-read `target_activation`, `target_snapshot`, `target_revocation`, and verify the exact tuple. `Execute` must re-read current authority before running the executor.
- **Code change required:** Yes.

**F-2: Missing PrepareForAction boundary**
- **Location:** Does not exist anywhere in the codebase.
- **Attack:** Stale belief, stale authority, revoked target, changed target/action, changed policy, changed actor — none are re-read at execution time.
- **Why current implementation permits it:** `service/authority.Prepare` is dead code. Production paths have no preparation boundary.
- **Expected architecture:** `PrepareForAction(scenarioID, beliefID, action, target) → PreparationResult` that re-reads belief, evidence, debt, contradictions, current authority, policy, actor/action/target.
- **Recommended correction:** Implement `PrepareForAction` and call it before every execution.
- **Code change required:** Yes.

**F-3: Stale-state defense is non-functional**
- **Location:** `service/authority/authority.go:148-200` (dead code), `internal/wizard/refusal.go:131-135`
- **Attack:** Valid token + revoked current authority = EXECUTES (not DENIED).
- **Why current implementation permits it:** Token does not carry authority (correct), but no code path re-reads authority from the database either.
- **Expected architecture:** Plan3.md §30 requires: `valid signed stale token + revoked current authority = DENIED`.
- **Recommended correction:** Execution path must re-read `target_revocation` and `target_activation` immediately before execution.
- **Code change required:** Yes.

### HIGH

**F-4: Policy layer is de facto second authority engine**
- **Location:** `service/policy/policy.go:206-232`, `service/authority/authority.go:100-115` (dead code)
- **Attack:** `policy=ALLOW, authority=ABSENT` → action proceeds.
- **Why current implementation permits it:** `AuthorizeToolCall` returns `true` based on tool class and required beliefs. The execution path treats this as authorization.
- **Expected architecture:** "Policy may constrain authority. Policy may not manufacture authority."
- **Recommended correction:** Policy must return constraints, not a boolean. The execution path must separately verify `kernel.Authorize`.
- **Code change required:** Yes.

**F-5: Wizard Authorize creates intent, does not verify authority**
- **Location:** `internal/wizard/refusal.go:131-135`
- **Attack:** Judge clicks "Authorize" on a promoted belief with no valid authority target. Live intent is created.
- **Why current implementation permits it:** `wizard.Server.Authorize` calls `s.kern.IntentOnPromoted`, which creates a live intent gated only by belief promotion.
- **Expected architecture:** Wizard should demonstrate the full authority path: target → justification → request → approve → authorize → execute.
- **Recommended correction:** Wizard Authorize screen must require an approved authority target and call `kernel.Authorize` before creating an intent.
- **Code change required:** Yes.

**F-6: Workflow drift collapses security semantics**
- **Location:** `service/workflow/workflow.go:36-43`
- **Attack:** `prepared` is treated as "approved and authorized" when it is only "policy passed and belief is promoted."
- **Why current implementation permits it:** 7 approved states collapsed into 6 implemented states. HUMAN_REVIEW, APPROVED, AUTHORIZATION_READY are all collapsed into `prepared`.
- **Expected architecture:** Plan3.md §9 defines 7 states. Each must be a distinct, enforceable transition.
- **Recommended correction:** Restore the full state machine or explicitly justify the simplification with a security argument.
- **Code change required:** Yes.

**F-7: No adversarial security tests exist**
- **Location:** Entire `*_test.go` tree
- **Attack:** Any of the attacks in plan3.md §29/§30.
- **Why current implementation permits it:** No tests were written for stale token, revoked authority, wrong target, fake approval, provider authority creation, executor authority creation, browser state authority.
- **Expected architecture:** Mandatory adversarial test matrix per plan3_review.md §13.
- **Recommended correction:** Write the mandatory tests before Phase 4/6.
- **Code change required:** Yes.

### MEDIUM

**F-8: Actor/authentication boundary is client-declarative**
- **Location:** `cmd/solvent-mcp/tools.go:213-221`
- **Attack:** Adversarial agent passes `"action_source": "user_typed"` to bypass tool_output guard.
- **Why current implementation permits it:** `action_source` is a client-declared string with no session authentication.
- **Expected architecture:** "ActorType != Identity != Authentication."
- **Recommended correction:** Authentication must happen at the MCP/HTTP boundary (session, token, mTLS). `action_source` should be derived from authenticated session, not client input.
- **Code change required:** Yes.

**F-9: Workflow token is correctly non-authoritative but useless**
- **Location:** `db/007_service_tables.sql:5-18`, `service/workflow/workflow.go`
- **Attack:** Token does not carry authority fields. **PASS.** But since `service/workflow` is dead code, the token is never used.
- **Why current implementation permits it:** Dead code.
- **Recommended correction:** Either wire the token into the execution path or remove the dead code.
- **Code change required:** Yes (wire or remove).

**F-10: Dead service layer code**
- **Location:** `service/authority/`, `service/workflow/`, `service/executor/`
- **Attack:** None — dead code is not executed.
- **Why current implementation permits it:** Phase 1/2 service boundaries were designed but never wired into production paths.
- **Expected architecture:** Plan3.md §6 requires `WorkflowService`, `PolicyService`, `EvidenceService`, `AuthorityService`, `AuditService`, `ExecutionService` to be wired into the execution path.
- **Recommended correction:** Wire `service/authority` into wizard and MCP, or remove it to avoid the false sense of security.
- **Code change required:** Yes.

### LOW

**F-11: Build/test infrastructure**
- **Location:** `Taskfile.yml`, test suite
- **Attack:** Cannot run tests without CockroachDB.
- **Why current implementation permits it:** No embedded test database.
- **Recommended correction:** Acceptable for this environment. Tests pass when DB is available.
- **Code change required:** No.

---

## 16. Final Gate

### HOLD

**One or more critical/high findings require remediation before continuing to Phase 4 or Phase 6.**

The four highest-priority questions from plan3_review.md are answered as follows:

1. **Is workflow_token truly non-authoritative?**  
   **YES** — the table contains no authority fields. But it is dead code, so the question is moot for the current execution path.

2. **Does the final authorization path re-read CURRENT authority, target, action, actor and policy?**  
   **NO** — the execution path never calls `kernel.Authorize`. It re-reads belief status and policy only. Current authority, target, action binding, and revocation are never re-read.

3. **Has PolicyService remained policy, rather than becoming a second authorization engine?**  
   **NO** — `AuthorizeToolCall` returns a boolean `allowed`. The execution path treats `allowed=true` as authorization. Policy has become the de facto authority engine.

4. **Has the new product-layer database schema remained completely separate from authority-core truth?**  
   **YES** — `workflow_token`, `policy_tool`, `policy_actor`, `audit_activity` are product-layer and do not duplicate authority-core tables. This is clean.

**Bottom line:** The kernel is small, correct, and well-tested. The authority-core schema enforces its invariants. But the execution path was never connected to the authority verification layer. An agent can currently execute actions on promoted beliefs without any valid authority target, approval, or exact binding verification. This is exactly the "second authority engine" failure mode the plan warned against.

**Do not proceed to Phase 4/6 until:**
- The execution path re-reads current authority (`target_activation` + `target_snapshot` + no `target_revocation` + exact tuple match + promoted justifications) immediately before execution.
- `PrepareForAction` is implemented and called on every consequential action.
- The mandatory adversarial test `valid signed stale token + revoked current authority = DENIED` is written and passes.
- Policy is demoted from boolean `allowed` to constraint-only, and authority verification is restored to `kernel.Authorize`.
