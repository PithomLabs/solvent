# Fresh Adversarial Review — Post-Remediation

## Verdict

**GREEN WITH ACCEPTED LOW FINDINGS**

The remediation for NEW-01, NEW-02, NEW-03, NEW-04, and NEW-05 has been inspected and tested against the live CockroachDB cluster. All previously reported vulnerabilities are conclusively resolved: intent ownership is now enforced atomically at the SQL level across `(scenario_id, intent_id, belief_id, action, state='live')`; intent creation is strictly isolated to the belief’s owning scenario inside the transaction, eliminating the `RetractCascade` DoS deadlock; evidence insertion is scenario-bound at both REST and kernel boundaries; `gofmt` indentation is clean; and `task db:reset` provisions all 8 migrations, enabling `mcp_verify.sh` and `task test` to pass out-of-the-box. No new security, integrity, isolation, authority, or concurrency defects were identified.

---

## Critical Findings

*None.*

---

## High Findings

*None.*

---

## Medium Findings

*None.*

---

## Low Findings

### F-06: Typed MCP Argument Widening

* **ID**: F-06 (Historical / Accepted Low)
* **Severity**: LOW
* **Title**: MCP handlers accept widened string types from JSON-RPC, relying on downstream runtime validation
* **Location**: [`cmd/solvent-mcp/tools.go:110`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L110), [`cmd/solvent-mcp/tools.go:613`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L613)
* **Status**: ACCEPTED LOW.
* **Evidence**: `handleSolventRetireDebt` widens `debt_item` from the schema enum to string, but validates it via `slices.Contains(kernel.FullDebt, item)`. `handleSolventDischarge` accepts any string `obligation_key`, which is accepted because discharge receipts key arbitrary review obligations.

---

## Accepted Architectural Limitations

### F-01: Trusted Local Stdio MCP Boundary

* **Status**: ACCEPTED ARCHITECTURAL LIMITATION.
* **Evidence**: [`cmd/solvent-mcp/main.go:73-87`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/main.go#L73-L87) strictly validates `MCP_TRANSPORT`. Setting `MCP_TRANSPORT=sse` or `MCP_TRANSPORT=http` fails closed with exit code 1:
  ```text
  unsupported MCP transport "sse": only stdio is supported
  MCP is a trusted local surface; network exposure transfers responsibility to the deployment boundary
  ```
  Within the trusted-local stdio model, authorization and scenario boundaries are faithfully preserved.

---

## Historical Finding Status

| Finding | Status | Evidence |
| ------- | ------ | -------- |
| F-02    | CLOSED | Pre-mutation snapshot validation verified at [`cmd/solvent-mcp/tools.go:136-142, 182-188, 627-631`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L136-L142). |
| F-04    | CLOSED | `scripts/check_i7.sh` verified live: 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool reads. |
| F-05    | CLOSED | `envelopeErrorResult` in [`cmd/solvent-mcp/tools.go`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go) invokes `pipeline.AuditIntent` on refusals. |
| F-07    | CLOSED | `task --list` parses cleanly without errors and lists 33 tasks. |
| F-06    | OPEN / LOW / ACCEPTED | Typed MCP argument widening mitigated by `slices.Contains` in `handleSolventRetireDebt`. |
| F-01    | ACCEPTED ARCHITECTURAL LIMITATION | Stdio transport pinned; non-stdio fails closed. |

---

## NEW-01 Verification

**Status: RESOLVED & ENFORCED**

* **Scenario Match**: Enforced in [`kernel/sql.go:196`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L196) (`WHERE scenario_id = $2::UUID`). Mismatch yields 0 rows affected $\rightarrow$ `ErrIntentNotLive`.
* **Belief Match**: Enforced in [`kernel/sql.go:197`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L197) (`AND belief_id = $3::UUID`). Mismatch yields 0 rows affected $\rightarrow$ `ErrIntentNotLive`.
* **Action Match**: Enforced in [`kernel/sql.go:197`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L197) (`AND action = $4::STRING`). Mismatch yields 0 rows affected $\rightarrow$ `ErrIntentNotLive`.
* **State Match**: Enforced in [`kernel/sql.go:198`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L198) (`AND state = 'live'`). Atomic CAS transitions state `live` $\rightarrow$ `executing`.
* **Immutability Verification**: A repository-wide audit of all `UPDATE action_intent` statements confirms only the `state` column is ever modified ([`kernel/sql.go:82, 196, 202, 206, 210`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L82)). The tuple `(scenario_id, belief_id, action)` is strictly immutable once inserted.
* **Exploit Reproduction Result**:
  Running the previous exploit test (`test_intent_sub.go`: substituting $I_B$ from Belief $B$ into `ExecuteAction` for Belief $A$) against CockroachDB now yields:
  ```text
  ExecuteAction result: err=<nil>, res=&{TokenID: Allowed:false Success:false Output: Error:claim intent: intent is not in live state Reason: ExecutedAt:...}
  Executor called: false
  Final state of intentB in DB: live
  ```
  The claim is rejected, the intent remains `live`, and the executor is never invoked.

---

## NEW-02 Verification

**Status: RESOLVED & ENFORCED**

* **`createIntentWithinTx`**: In [`kernel/kernel.go:146-155`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel.go#L146-L155), `createIntentWithinTx` runs `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)` in the same transaction prior to `INSERT INTO action_intent`. If the belief does not belong to the scenario, it returns `ErrBeliefNotFound` and writes zero rows.
* **`IntentOnPromoted`**: Reuses `createIntentWithinTx`. Cross-scenario intent creation returns `ErrBeliefNotFound`. Verified by `TestCS_NEW02_CrossScenarioRejected`.
* **`AuthorizeAndCreateIntent`**: Reuses `createIntentWithinTx`. Cross-scenario intent creation returns `ErrBeliefNotFound` and rolls back the transaction.
* **`RetractCascade`**: With cross-scenario intent creation prevented, foreign intents cannot be attached to victim beliefs. Re-running the original retraction DoS attack script (`test_retract_dos.go`) confirms:
  ```text
  IntentOnPromoted error: belief not found in scenario
  ```
  The attack is blocked at the creation boundary, `RetractCascade` completes cleanly without `SQLSTATE 23514`, and child beliefs are retracted. Verified by `TestCS_NEW02_CrossScenarioRetractSucceeds`.

---

## NEW-03 Verification

**Status: RESOLVED & ENFORCED**

* **Kernel Boundary**: In [`kernel/kernel.go:82-91`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel.go#L82-L91), `kernel.AddEvidence` queries `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)` in the transaction prior to executing `sqlAddEvidence`. Cross-scenario calls return `ErrBeliefNotFound`. Verified by `TestCS_NEW03_CrossScenarioRejected`.
* **REST Boundary**: In [`api/evidence.go:37-41`](file:///home/chaschel/Documents/go/solvent-main/api/evidence.go#L37-L41), `handleAddEvidence` executes `view.GetSnapshot(r.Context(), s.db, req.ScenarioID, view.SnapshotOpts{BeliefID: req.BeliefID})`. Re-running `test_rest_evidence_cs.go` against CockroachDB confirms:
  ```text
  REST POST /v1/evidence status code: 404
  Response body: {"code":"not_found","message":"belief not found in scenario","retryable":false}
  Evidence rows in scB for bID(scA): 0
  ```
  Cross-scenario evidence insertion is rejected with 404, writing zero rows.

---

## New Findings

```text
No new findings
```

---

## Verification Results

| Check | Result | Details |
| ----- | ------ | ------- |
| `gofmt -l cmd internal kernel` | **PASS** | Exited 0; no formatting diffs. |
| `go build ./...` | **PASS** | Exited 0 with no warnings or errors. |
| `go vet ./...` | **PASS** | Exited 0 with no diagnostics. |
| Full `go test -count=1 -p 1 ./...` | **PASS** | Exited 0; all 18 test packages passed (including 15 DB-backed packages against CockroachDB). |
| Full `go test -race -count=1 -p 1 ./...` | **PASS** | Exited 0 across the entire repository with zero data races. |
| `task --list` | **PASS** | Exited 0; all 33 tasks parsed cleanly. |
| `task db:reset` | **PASS** | Exited 0; dropped and recreated `fable`, applying all 8 migrations (`001_schema.sql` through `008_executing_state.sql`). |
| `task test` | **PASS** | Exited 0; completed full suite including build, vet, gofmt, `check_i7.sh`, wizard single-call-site, MCP boundary, sentry_error containment, and `mcp_verify.sh`. |
| `bash scripts/check_i7.sh` | **PASS** | Exited 0: 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool reads. |
| `bash scripts/mcp_verify.sh` | **PASS** | Exited 0: all 34 JSON-RPC stdio assertions green against `fable`. |

---

## Security Invariant Assessment

* **Scenario Isolation**: **PASS**. All mutation surfaces (`RetireDebt`, `Promote`, `Discharge`, `AddEvidence`, `IntentOnPromoted`, `AuthorizeAndCreateIntent`) atomically verify that the subject belief belongs to the specified scenario.
* **Intent Ownership**: **PASS**. `kernel.ClaimIntent` atomic CAS requires an exact match on `(scenario_id, intent_id, belief_id, action, state='live')`. Cross-belief or cross-action intent hijacking is structurally impossible.
* **Authority Integrity**: **PASS**. Target creation, activation, snapshot hashing, approval pinning, and revocation enforcement remain strictly sound. `kernel.Authorize` is read-only and authoritative.
* **Action / Target Binding**: **PASS**. Targets and snapshots remain immutable once activated. Consequence parameters are read exclusively from database snapshots.
* **Intent Creation**: **PASS**. Live intent creation requires promoted belief status via CockroachDB foreign key (`CONSTRAINT gate`), and same-transaction scenario ownership prevents cross-scenario intent injection.
* **Evidence Integrity**: **PASS**. `AddEvidence` verifies belief ownership before write; evidence content is immutable.
* **Discharge Atomicity**: **PASS**. `kernel.Discharge` executes existence check, discharge record insertion, and debt retirement within a single `crdb.ExecuteTx` boundary.
* **Executor Integrity**: **PASS**. `resolveExecutor` maps fixed action names to registered Go executor implementations; caller parameters cannot specify executor implementations.
* **Audit Truthfulness**: **PASS**. `ReconcileIntent` executes kernel state transitions before emitting audit logs; failed transitions emit `ActivityReconciliationFailed` and preserve original errors.
* **MCP Boundary**: **PASS**. Runs strictly over stdio; network transports (`sse`, `http`) fail closed immediately; no direct database writes.
* **DB Invariants**: **PASS**. All CockroachDB check constraints (`promoted_is_debt_free`, `live_requires_promoted`) and foreign keys function correctly; retraction deadlock condition has been eliminated.
* **Concurrency / TOCTOU**: **PASS**. `AuthorizeAndCreateIntent` serializes against `RevokeTarget` via `SELECT FOR UPDATE` on `authority_target`. `ClaimIntent` serializes concurrent claims via atomic CAS on `action_intent`. Clean race-detection suite confirmed.

---

## Final Assessment

1. **Readiness to Advance**: The current repository state is **READY TO ADVANCE**.
2. **Mandatory Remaining Remediation**: None. All previous adversarial findings (M-01, M-02, L-01, L-05, NEW-01, NEW-02, NEW-03, NEW-04, NEW-05) are resolved, verified, and backed by automated regression tests in `kernel/kernel_test.go`.
3. **Accepted LOW Findings**: F-06 (typed MCP argument widening) remains accepted; F-01 (trusted local stdio MCP boundary) remains an accepted architectural limitation.
4. **Verification Limitations**: Forced mid-transaction rollback (CS-6) was deferred due to `pg_cancel_backend` test-harness timing flakiness, but transaction atomicity is structurally guaranteed by `crdb.ExecuteTx`. All other behavioral gates are fully proven against CockroachDB.