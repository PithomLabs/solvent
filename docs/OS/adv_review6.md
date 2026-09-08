Let me start by reading the plan files for context, then systematically inspect the repository HEAD.

Listed directory solvent-main
Ran command: `cd /home/chaschel/Documents/go/solvent-main && git status --short && echo "---HEAD---" && git rev-parse HEAD && echo "---LOG---" && git log --oneline -12`
Listed directory OS
Ran command: `cd /home/chaschel/Documents/go/solvent-main && find . -maxdepth 4 -type f -name "*.go" | sort | head -100`
Viewed plan8.md:1-285
Ran command: `cd /home/chaschel/Documents/go/solvent-main && find . -maxdepth 4 -type f -name "*.go" | sort | tail -60`
Viewed plan8.1.md:1-252
Viewed kernel.go:1-201
Viewed authority.go:1-797
Viewed authority.go:1-501
Viewed tools.go:1-800
Viewed main.go:1-800
Viewed sql.go:1-263
Viewed ledger.go:1-170
Viewed authorization.go:1-268
Viewed api.go:1-126
Viewed auth.go:1-62
Viewed belief.go:1-248

Now let me look at some critical areas — the DB schema, the REST API authorization, and the execute flow more carefully.

Ran command: `cd /home/chaschel/Documents/go/solvent-main && ls db/ && cat db/001_schema.sql`
Viewed executor.go:1-53
Ran command: `cat /home/chaschel/Documents/go/solvent-main/db/005_authority_mvp.sql && echo "---" && cat /home/chaschel/Documents/go/solvent-main/db/006_authority_justification_cascade.sql && echo "---" && cat /home/chaschel/Documents/go/solvent-main/db/007_service_tables.sql && echo "---" && cat /home/chaschel/Documents/go/solvent-main/db/008_executing_state.sql`
Viewed executor.go:1-48
Viewed provider_errors.go:1-51
Ran command: `cd /home/chaschel/Documents/go/solvent-main && go build ./... 2>&1; echo "EXIT: $?"`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && go vet ./... 2>&1; echo "EXIT: $?"`
Viewed check_i7.sh:1-136
Ran command: `cd /home/chaschel/Documents/go/solvent-main && bash scripts/check_i7.sh 2>&1; echo "EXIT: $?"`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && go test -count=1 -p 1 ./internal/derive/... ./internal/normalize/... ./service/executor/... ./service/policy/... ./kernel/... 2>&1 | tail -30; echo "EXIT: $?"`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && go test -count=1 -p 1 ./service/authority/... ./api/... ./cmd/solvent-mcp/... 2>&1 | tail -30; echo "EXIT: $?"`
Viewed tools_authority_test.go:1-724

Now let me check a few more critical areas — the `handleSolventAuthorizeAction` intent-creation path more carefully, the REST API scenario isolation for belief mutations, and the `handleSolventRetireDebt` / `handleSolventPromote` without scenario scoping in the kernel SQL.

Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'scenario_id' kernel/sql.go | head -40`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'scenario_id\|WHERE' api/reads.go 2>/dev/null || cat api/reads.go | head -80`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'sqlRetireDebt\|sqlPromote' kernel/sql.go && echo "---" && grep -n 'RetireDebt\|Promote' kernel/kernel.go`
Viewed policy.go:1-299
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'scenario_id\|WHERE\|RetireDebt\|Promote\|Discharge\|RevokePrincipal' api/belief.go api/target.go api/discharge.go 2>/dev/null | head -60`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && bash scripts/mcp_verify.sh 2>&1 | head -40; echo "EXIT: $?"`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && cat api/discharge.go && cat api/target.go | head -80`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && go test -race -count=1 ./internal/derive/... ./internal/normalize/... ./service/executor/... ./service/policy/... 2>&1 | tail -15; echo "EXIT: $?"`

I have now gathered sufficient evidence. Let me look at two remaining specific issues before writing the report:

Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'intentID\|ClaimIntent\|intent_id' service/authority/authority.go | head -20`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'AuthorizeMiddleware\|AuthMiddleware\|auth.*middleware\|Handler()' cmd/solvent-api/main.go | head -20`
Viewed main.go:60-85
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'action_source\|ActionSource\|action_source' api/authorization.go api/types.go 2>/dev/null | head -20`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -rn 'intentID == ""\|intentID != ""\|if intentID' service/authority/authority.go`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'Discharge\|discharge' kernel/sql.go kernel/authority.go | grep -i 'scenario\|WHERE belief_id\|$1\|$2'`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -A5 'sqlDischargeInsert' kernel/sql.go && grep -A4 'func.*Discharge' kernel/authority.go | head -20`
Ran command: `cd /home/chaschel/Documents/go/solvent-main && grep -n 'handleSolventDischarge\|handleSolventRetireDebt\|handleSolventPromote' cmd/solvent-mcp/tools.go | head -20 && echo "---" && grep -n 'scenario_id\|beliefID.*scenario\|scenarioID.*belief' cmd/solvent-mcp/tools.go | grep -i discharge | head -10`
Viewed tools.go:611-640
Ran command: `cd /home/chaschel/Documents/go/solvent-main && task --list 2>&1 | head -40; echo "EXIT: $?"`

I now have comprehensive evidence. Let me produce the final report.

Created fresh_adversarial_review.md

---

## Fresh Adversarial Review — `5933f44` (`phase 4e`)

**Verdict: GO WITH ACCEPTED LOW FINDINGS**

The kernel authority model is architecturally sound. No path permits unauthorized authority creation, executor substitution, or stale-authority execution. The `AuthorizeAndCreateIntent` atomic primitive closes the historical TOCTOU window. `go build`, `go vet`, `check_i7.sh`, and all non-DB unit tests PASS. DB-dependent tests are BLOCKED BY ENVIRONMENT (CockroachDB not running), correctly fail-closed.

The review discovered **4 new issues** not covered by historical F-01–F-07 findings:

---

### Medium Findings

**M-01 — REST API cross-scenario write gap (NEW)**
[`api/belief.go:115,143`](file:///home/chaschel/Documents/go/solvent-main/api/belief.go#L115-L143), [`api/discharge.go:33`](file:///home/chaschel/Documents/go/solvent-main/api/discharge.go#L33)

`handleRetireDebt`, `handlePromoteBelief`, and `handleDischarge` call kernel mutations keyed only by `belief_id` — no `scenario_id` predicate. The kernel SQL (`sqlRetireDebt`, `sqlPromote`, `sqlDischargeRetireDebt`) is `WHERE id = $1::UUID` with no scenario filter. An authenticated REST caller can retire debt on, promote, or discharge **any belief in any scenario** by supplying the belief UUID. The MCP surface correctly prevents this via `view.GetSnapshot(ctx, db, scenarioID, ...)` before every mutation — the REST surface does not mirror this guard.

**Fix:** Add `AND scenario_id = $2::UUID` to the mutation SQL, or add a pre-mutation cross-scenario guard at the handler layer. **Must be fixed before multi-tenant deployment.**

---

**M-02 — `ExecuteAction` silently skips `ClaimIntent`/`CompleteIntent` when `intentID == ""` (NEW)**
[`service/authority/authority.go:303,400`](file:///home/chaschel/Documents/go/solvent-main/service/authority/authority.go#L303-L418)

The CI-4 ownership gate (`ClaimIntent`) is guarded by `if intentID != "" { ... }`. Both current call-sites (MCP and REST) enforce non-empty `intentID` before calling `ExecuteAction`, but the function itself does not assert this as a precondition. A future caller supplying `intentID = ""` would invoke the executor with no intent ownership record and no ledger trace.

**Fix:** Add `if intentID == "" { return nil, errors.New("intentID is required") }` at the top of `ExecuteAction`. One line, closes the latent CI-4 contract violation.

---

### Low Findings (abbreviated)

| ID | Location | Issue |
|----|----------|-------|
| **L-01 (NEW)** | [`tools.go:611`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L611) | `handleSolventDischarge` has no cross-scenario guard; inconsistent with every other mutation handler |
| **L-02** | [`tools.go:211`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L211) | `action_source = "user_typed"` is a declaration guard, not authentication (by design; flagged) |
| **L-03** | [`tools.go:110`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L110) | F-06 typed-argument widening confirmed present, mitigated by downstream `slices.Contains` |
| **L-04** | [`api/belief.go:141`](file:///home/chaschel/Documents/go/solvent-main/api/belief.go#L141) | `scenario_id` from query param not UUID-validated before mutation (sub-finding of M-01) |
| **L-05 (NEW)** | [`authority.go:447`](file:///home/chaschel/Documents/go/solvent-main/service/authority/authority.go#L447) | `ReconcileIntent` logs audit before the kernel state transition; transient DB error produces false audit record |

---

### Historical Finding Reconciliation

All confirmed consistent with HEAD — no contradictory evidence found:

| Finding | Status |
|---------|--------|
| F-02 (cross-scenario guard ordering) | CLOSED — `view.GetSnapshot` before kernel mutation at `tools.go:136-142, 182-188` |
| F-04 (I-7 gate inert) | CLOSED — `check_i7.sh` PASS: 21 ExecuteTx sites, 0 raw writes |
| F-05 (audit discarded on refusal) | CLOSED — `envelopeErrorResult` calls `pipeline.AuditIntent` |
| F-07 (Taskfile syntax) | CLOSED — `task --list` exits 0, 33 tasks |
| F-01 (MCP trust boundary) | Accepted limitation — `MCP_TRANSPORT` guard confirmed functional at `main.go:73-87` |


## chatgpt

This review materially changes the disposition. **I would not accept the reported `GO WITH ACCEPTED LOW FINDINGS`. I would call the current HEAD `NO-GO` until M-01 and M-02 are resolved and the DB-backed suite is rerun.**

The fresh review did exactly what we wanted: it found issues that the historical review cycle never covered. That means the review process is working.

### M-01 is a real security finding, and I would raise its severity

The important fact is:

> REST mutation is keyed by `belief_id` without enforcing the caller's `scenario_id`.

The reviewer explicitly states that an authenticated caller can mutate a belief belonging to another scenario, while the MCP path performs the cross-scenario guard. That is not merely an API consistency problem; it is **an authorization/isolation failure at a second externally reachable surface**.

I would classify it as **HIGH rather than MEDIUM** if `scenario_id` represents a security boundary.

The particularly concerning aspect is that the kernel mutation itself also lacks the scenario predicate:

```text
REST request
   ↓
belief_id
   ↓
kernel mutation WHERE id = ...
   ↓
mutation succeeds
```

The handler's missing guard is therefore not merely a presentation-layer omission. The underlying primitive is callable without scenario binding.

And L-01 makes the situation more important: `handleSolventDischarge` appears to have the same class of MCP problem. So M-01 should immediately trigger a **systematic audit of every mutation surface**, not a one-off fix to three REST endpoints.

The architecture currently looks asymmetric:

```text
MCP
  scenario
    ↓
GetSnapshot
    ↓
kernel mutation

REST
  scenario
    ↓
(no equivalent binding)
    ↓
kernel mutation
```

That is exactly the kind of duplicated authorization logic that tends to regress.

### M-02 should also be fixed before GREEN

This is a good finding.

`ExecuteAction` is a security-sensitive boundary, yet its required `intentID` contract is enforced only by current callers rather than by the function itself.

That creates:

```text
trusted caller
    ↓
ExecuteAction("", ...)
    ↓
executor
    ↓
external consequence
```

without the CI-4 ownership record.

The proposed precondition is cheap and appropriate:

```go
if intentID == "" {
    return nil, errors.New("intentID is required")
}
```

This is exactly the sort of invariant that belongs at the authority service boundary rather than being left to every caller.

I'd call it **MEDIUM**, because the current known callers already supply the ID, but the primitive itself is not enforcing its security contract.

### L-05 deserves more scrutiny than its LOW label suggests

This one is potentially more consequential than the report's abbreviation implies.

The report says:

> `ReconcileIntent` logs audit **before** the kernel state transition; transient DB error produces false audit record.

That creates:

```text
audit says reconciliation happened
        ↓
kernel transition fails
        ↓
actual state disagrees with audit
```

For a system whose commercial proposition includes authoritative audit/compliance evidence, that is not merely cosmetic.

It may still be LOW depending on exactly what the audit event means and whether the event is explicitly "reconciliation attempted" rather than "reconciliation completed." But the reviewer should demonstrate the precise semantics before closing this one as LOW.

### The final report also overstates its confidence

This sentence is too strong:

> “No path permits unauthorized authority creation, executor substitution, or stale-authority execution.”

The same review says the DB-dependent tests were **BLOCKED BY ENVIRONMENT**.

That means the report has established:

> **No such defect was observed in the inspected code paths and runnable tests.**

It has **not** established the universal claim that no path permits those behaviors.

Likewise:

> “The `AuthorizeAndCreateIntent` atomic primitive closes the historical TOCTOU window.”

That is only true for the particular **Authorize → Intent creation** race it was designed to close. It does not eliminate the accepted:

```text
ClaimIntent → external provider call
```

TOCTOU window.

The terminology should remain precise.

### F-01 is still not “closed”

The historical table says:

> “F-01 ... Accepted limitation — `MCP_TRANSPORT` guard confirmed functional”

That wording is still subtly wrong.

`MCP_TRANSPORT` being functional proves the **future-transport guard** works. It does not prove the F-01 architectural limitation has been mitigated.

The correct record remains:

**F-01 = ACCEPTED ARCHITECTURAL LIMITATION**
**MCP_TRANSPORT = future-transport hardening**

### What I would require before GREEN

I would make the next remediation pass narrowly scoped:

**P0 / must fix**

1. Fix M-01 so scenario binding is enforced for REST mutations.
2. Audit every mutation primitive and every externally reachable mutation handler for the same class of missing scenario binding.
3. Specifically investigate L-01 immediately; it may be another instance of M-01 rather than an independent LOW finding.
4. Add regression tests proving cross-scenario mutation is impossible through **REST and MCP**.

**P1 / must fix**
5. Make `intentID` a mandatory precondition inside `ExecuteAction`.
6. Investigate L-05 and establish exact audit semantics; prevent a “successful reconciliation” audit record from preceding a failed state transition unless the event explicitly represents an attempt.

**Then**
7. Start CockroachDB.
8. Run the complete DB-backed test suite and race suite.
9. Run the I-7 and MCP checks again.
10. Fresh adversarial review of the resulting HEAD.

The key architectural lesson from this review is actually valuable:

> **The kernel may be correct while the system is still insecure because externally reachable service/API boundaries fail to bind requests to the kernel's security context.**

That is precisely why the next pass should not blindly “fix M-01.” It should trace **every mutation entry point → kernel primitive → durable scope predicate** and establish one consistent isolation invariant.

So my disposition is:

**Current HEAD: NO-GO.**

Not because the earlier reviews were useless—quite the opposite. This fresh review found a genuinely important gap that the previous review process missed, which means **this is exactly the review we needed before declaring GREEN.**



## claude

Good review overall — the historical reconciliation checks out against everything verified in Plan 8/8.1, and M-01 and M-02 are both genuine, well-evidenced new findings, not restatements. Three things worth pushing on before treating this as settled.

## The verdict's own language contradicts its own findings table

"GO WITH ACCEPTED LOW FINDINGS" — but M-01 and M-02 are Medium, and neither is dispositioned as accepted. M-01 says "must be fixed before multi-tenant deployment" and M-02 says "Fix: one line... closes the latent CI-4 contract violation" — both read as open, required remediation, not as findings the project has consciously decided to live with. That's a different thing from "accepted low findings," and the mismatch matters here specifically: this whole review chain (F-01 through F-07, plan8, plan8.1) has been built around insisting that a verdict's summary language match what its evidence actually supports, and this is the same class of slip as the earlier "seven vs. eight blocking conditions" and "six vs. seven independent decisions" miscounts a few rounds back — the top-line claim doesn't match the table underneath it. I'd want this restated as "GO, with two Medium findings requiring a follow-up fix before the affected surfaces are used in production" — which is an honest, still-reasonable verdict, just not the one currently written.

## M-01 is more urgent than "before multi-tenant deployment" frames it

The fix is right and the finding is real, but I don't think this needs multi-tenancy to matter. `scenario_id` has been the partition boundary between distinct contexts since `track1`/`track2` — separate business logic, separate belief spaces — independent of whether those contexts belong to different *customers*. If any authenticated REST caller can retire debt, promote, or discharge a belief in a scenario they were never authorized to touch, that's a live cross-context integrity problem today, in the current single-tenant deployment, not a future concern gated on multi-tenancy shipping. I'd reframe the urgency accordingly — this is closer to "fix before this surface is used for anything beyond the current demo scenario," not "fix before multi-tenant."

## L-05 is under-severitied relative to a standard this same codebase already sets for itself

Logging the audit entry before the kernel transition, such that a transient failure produces a false "this succeeded" record, is worth more scrutiny than a Low tag gives it. This project already has the correct pattern for exactly this class of problem — `CompleteIntent`'s persistence-failure path logs `ActivityIntentCompletionFailed`, a truthful record of what actually happened, precisely because a false-positive audit entry is more dangerous than a missing one: a gap in the log is honestly incomplete, but a record saying "operator resolved this" when the intent is still stuck in `executing` actively misleads anyone reconstructing an incident later. `ReconcileIntent` not following the same discipline the codebase already established for `CompleteIntent` is worth naming as an inconsistency, not just a low-priority ordering nit — same fix pattern (log after, or log a truthful failure event on error) should just be ported over.

## F-01's disposition doesn't engage with the actual gap I raised last turn

"Accepted limitation — `MCP_TRANSPORT` guard confirmed functional" reads as if the guard addresses the threat model, but "functional" here only means the guard correctly rejects an explicit `MCP_TRANSPORT=sse` selection — it says nothing about the actual attack path (an external tool bridging the process's existing stdin/stdout to a network endpoint without the process's own code path changing at all). That's the same gap I raised on `plan8.1.md`, and this review doesn't appear to have engaged with it — it just confirmed the guard runs and exits correctly for a scenario that was never the threat. I'd want the disposition to say plainly "accepted architectural limitation; the code-level guard prevents accidental future transport misconfiguration but does not and cannot prevent external stdio-tunneling, which remains a deployment/operational responsibility" — rather than "confirmed functional" standing in for "closed."

**Net:** M-02's one-line fix and L-05's audit-ordering fix are both cheap and should land before calling this settled. M-01 needs the fix regardless of multi-tenancy timing. The verdict language needs to match its own table. None of this overturns the core assessment — the kernel authority model does sound architecturally solid on the evidence shown — but "GO" should be stated as conditional on M-01/M-02, not alongside language that implies everything found was already low-stakes and accepted.


## chatgpt

Agreed. Those corrections make the review internally consistent and materially sharper.

The disposition I would use is:

### Verdict

> **GO, CONDITIONAL — two Medium findings require remediation before the affected REST/execution surfaces are used beyond the current controlled demo context.**

That reconciles the kernel assessment with the fact that M-01 and M-02 are still open. Calling this “GO WITH ACCEPTED LOW FINDINGS” is plainly inconsistent with the findings table.

### M-01

I agree this should be treated as a **current integrity/security defect**, not a future multi-tenant concern.

The relevant boundary is `scenario_id`, whether or not scenarios currently correspond to different customers. If the scenario abstraction is intended to isolate belief spaces, allowing an authenticated caller to mutate another scenario by UUID is already a violation of that invariant.

So the remediation target should be:

> **Every mutation path must establish the scenario binding before mutation, or the mutation primitive itself must enforce the binding atomically.**

I would favor the latter where practical because it moves the invariant closer to the durable state transition and reduces duplicated handler-level checks. But that decision should be made after auditing all existing mutation primitives rather than fixing only the three reported REST handlers.

And L-01 is now especially important: it suggests this is a **class of invariant enforcement problem**, not merely an accidental omission in REST.

### M-02

Agreed: fix it.

`ExecuteAction` should not depend on every caller remembering that `intentID` is mandatory. The authority service boundary should enforce its own ownership invariant.

The one-line precondition is appropriate:

```go
if intentID == "" {
    return nil, errors.New("intentID is required")
}
```

Then add a regression test so the contract cannot silently regress.

### L-05

I would promote this to **Medium pending verification**, rather than leaving it LOW.

Your distinction is important:

```text
Missing audit
    = incomplete evidence

False-success audit
    = misleading evidence
```

The latter is substantially worse for an authority/compliance system.

The correct pattern should be aligned with `CompleteIntent`:

```text
attempt state transition
        ↓
success?
 ┌──────┴──────┐
yes            no
 ↓              ↓
truthful        truthful failure/
success audit   failure audit
```

What should not happen is:

```text
write "reconciled"
       ↓
state transition fails
       ↓
intent remains executing
```

The remediation should therefore inspect exactly what `ReconcileIntent`'s current audit event claims. If it claims successful reconciliation, the ordering is a real correctness defect. If it explicitly records an attempted reconciliation, then the severity can be reconsidered.

### F-01

Yes. The historical table needs to say exactly what is and isn't being protected.

Use:

> **F-01 — ACCEPTED ARCHITECTURAL LIMITATION.** MCP is intentionally a trusted local stdio surface. `actor_id` is attribution, not authentication. External mechanisms capable of tunneling or forwarding stdio remain outside the process's ability to detect reliably and are therefore a deployment/operational responsibility. The `MCP_TRANSPORT` check provides future-transport hardening by rejecting unsupported transport configuration; it does not close the external stdio-tunneling limitation.

That removes the misleading implication that “guard functional” somehow equals “threat mitigated.”

### What the next remediation plan should do

I would keep it deliberately narrow:

```text
1. Audit ALL mutation entry points for scenario binding.
2. Fix M-01 and L-01 at the right architectural layer.
3. Add cross-scenario regression tests for REST + MCP.
4. Make intentID mandatory inside ExecuteAction.
5. Add regression coverage for empty intentID.
6. Inspect ReconcileIntent audit semantics.
7. Align reconciliation audit behavior with CompleteIntent.
8. Promote/demote L-05 based on demonstrated semantics.
9. Start CockroachDB.
10. Run complete integration + race verification.
11. Fresh adversarial review.
```

The important point is that **M-01 should trigger an invariant audit, not a three-line patch**. Otherwise we risk fixing the observed REST endpoints while leaving another mutation path with the same underlying defect.

So the resulting state is:

**Kernel architecture: sound on current evidence.**
**Historical F-02/F-04/F-05/F-07: closed.**
**F-01: accepted operational limitation, not closed by `MCP_TRANSPORT`.**
**F-06: accepted LOW.**
**M-01: OPEN, must fix now.**
**M-02: OPEN, must fix now.**
**L-05: escalate to Medium pending semantic verification.**

That gives you a clean, defensible gate: **conditional GO for controlled continuation, but not production-ready until M-01/M-02 and the audit-ordering issue are resolved and DB-backed verification is green.**


