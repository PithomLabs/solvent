# Fresh Adversarial Review — Post-8.5

## Verdict

**NO-GO**

While the Plan 8.4/8.5 remediation successfully bound `RetireDebt`, `Promote`, and `Discharge` to scenarios and verified them against CockroachDB, adversarial testing of the current repository state identified two critical/high vulnerabilities and one medium vulnerability. Most critically, **NEW-01** demonstrates that `ExecuteAction` accepts any live `intentID` within the same scenario regardless of belief or action, permitting execution of unauthorized actions by consuming unrelated intents (directly violating the stated architectural invariant and contradicting the false assertion in [`api/authorization.go:177-178`](file:///home/chaschel/Documents/go/solvent-main/api/authorization.go#L177-L178)). Furthermore, **NEW-02** demonstrates that `kernel.IntentOnPromoted` allows creating action intents for beliefs belonging to foreign scenarios, which induces an unrecoverable `SQLSTATE 23514` check-constraint deadlock on `RetractCascade` in the victim scenario, permanently blocking the retraction of falsified beliefs. Additionally, **NEW-03** allows cross-scenario writes via `POST /v1/evidence`, and **NEW-04** (`gofmt` indentation in [`kernel/kernel_test.go`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel_test.go#L748-L782)) breaks `task test`. The repository is not ready to advance until these vulnerabilities are resolved.

---

## Critical Findings

*None.*

---

## High Findings

### NEW-01: `ExecuteAction` Permits Arbitrary Same-Scenario Intent Identity Substitution (Cross-Belief and Cross-Action Intent Hijacking)

* **ID**: NEW-01 (Incomplete Remediation of M-02)
* **Severity**: HIGH
* **Title**: `ExecuteAction` accepts arbitrary same-scenario `intentID`, permitting execution of un-intent-backed actions by consuming unrelated intents across beliefs and actions
* **Location**: [`service/authority/authority.go:240-315`](file:///home/chaschel/Documents/go/solvent-main/service/authority/authority.go#L240-L315), [`kernel/authority.go:534-548`](file:///home/chaschel/Documents/go/solvent-main/kernel/authority.go#L534-L548), [`kernel/sql.go:192-194`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L192-L194)
* **Exact Code**:
  In [`service/authority/authority.go`](file:///home/chaschel/Documents/go/solvent-main/service/authority/authority.go#L240-L315):
  ```go
  // 0. Assert mandatory precondition: intentID is required.
  if intentID == "" {
      return nil, errors.New("intentID is required for ExecuteAction")
  }

  // 1. PrepareForAction — re-reads current state, calls kernel.Authorize.
  decision, err := s.PrepareForAction(ctx, scenarioID, beliefID, action, targetID, actorID, consequenceType, consequenceParameters)
  ...
  // 6. ClaimIntent — atomic CAS live→executing. Sole authority gate (CI-4).
  if intentID != "" {
      if err := s.kern.ClaimIntent(ctx, scenarioID, intentID); err != nil {
          result.Allowed = false
          result.Error = fmt.Sprintf("claim intent: %v", err)
          return result, nil
      }
  }
  ```
  In [`kernel/sql.go`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L192-L194):
  ```sql
  sqlClaimIntent = `
      UPDATE action_intent SET state = 'executing'
      WHERE id = $1::UUID AND scenario_id = $2::UUID AND state = 'live'`
  ```
* **Attack/Failure Sequence**:
  1. Scenario $S$ contains Belief $A$ (promoted) with an approved authority target for action `"deploy"`. Belief $A$ currently has **no live intent** (e.g. its intent was never created, already executed, or cancelled).
  2. Scenario $S$ also contains Belief $B$, which has an unrelated live intent $I_B$ for action `"rollback"` (or any benign action).
  3. A caller invokes `ExecuteAction(ctx, scenarioID, beliefA, "deploy", targetA, actorID, intentB, ...)`.
  4. `PrepareForAction` evaluates Belief $A$ and Target $A$ for action `"deploy"`. It completely ignores `intentID`. It returns `Allowed: true`.
  5. `ClaimIntent` executes `UPDATE action_intent SET state = 'executing' WHERE id = intentB AND scenario_id = S AND state = 'live'`. Because `intentB` is in scenario $S$ and currently live, `ClaimIntent` succeeds! It never checks whether `action_intent.belief_id == beliefA` or `action_intent.action == "deploy"`.
  6. The executor invokes the external provider for action `"deploy"` on Target $A$.
  7. `CompleteIntent` transitions `intentB` to `'executed'`.
* **Observed Evidence**:
  Executed standalone test against CockroachDB (`fable_authority_test`):
  ```text
  Setup: bA=b91dfc4c-87fa-4378-b8ae-03ef056b5eb3 (target approved for 'deploy', NO intent on bA)
  Setup: bB=ad1478b0-d9b6-4b7c-ae9c-770566517e23 (intentB=64470144-3f8a-4d53-ac35-497fabed50ae for action 'rollback')
  ExecuteAction result: err=<nil>, res=&{TokenID: Allowed:true Success:true Output:output Error: Reason: ExecutedAt:2026-09-08 12:06:07.060431247 +0800 PST}
  Executor called: true
  Final state of intentB in DB: executed
  ```
* **Reproduction Command**:
  `go run /home/chaschel/.gemini/antigravity-ide/brain/0ea77358-fe0a-4775-9eaf-66c567431da1/scratch/test_intent_sub.go`
* **Impact**:
  Directly violates the core architectural rule: `intent -> ExecuteAction -> Authorize -> Executor`. An action can execute without ever having a live intent created for its belief and action. An attacker or buggy agent can hijack any benign intent in the scenario to trigger execution on another belief, and consume intents intended for other actions. Furthermore, [`api/authorization.go:177-178`](file:///home/chaschel/Documents/go/solvent-main/api/authorization.go#L177-L178) states:
  > `// - intent_id is the authoritative execution identity; all caller-supplied fields are validated against the intent's context by the authority service.`
  
  This stated invariant is completely unimplemented.
* **Why Existing Controls Do Not Prevent It**:
  Plan 8.4 only added `if intentID == ""` to close M-02. It never verified that the `intentID` matches `(beliefID, action)` at either the service or kernel SQL boundary.
* **Recommended Remediation**:
  Extend `kernel.ClaimIntent` to take `(scenarioID, intentID, beliefID, action)` and enforce matching at the SQL CAS boundary:
  ```sql
  UPDATE action_intent SET state = 'executing'
  WHERE id = $1::UUID AND scenario_id = $2::UUID AND belief_id = $3::UUID AND action = $4::STRING AND state = 'live'
  ```

---

### NEW-02: Cross-Scenario Intent Creation via `IntentOnPromoted` Enables Retraction Deadlock (DoS on `RetractCascade`)

* **ID**: NEW-02 (Missed Scenario Isolation Gap)
* **Severity**: HIGH
* **Title**: `IntentOnPromoted` allows creating intents for beliefs in foreign scenarios, triggering an unrecoverable `SQLSTATE 23514` check-constraint deadlock on `RetractCascade`
* **Location**: [`kernel/kernel.go:137-156`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel.go#L137-L156), [`kernel/sql.go:30-32`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L30-L32), [`kernel/sql.go:88-95`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L88-L95)
* **Exact Code**:
  In [`kernel/kernel.go`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel.go#L137-L140):
  ```go
  func createIntentWithinTx(ctx context.Context, tx *sql.Tx, scenarioID, beliefID, action string) error {
      _, err := tx.ExecContext(ctx, sqlIntentOnPromoted, scenarioID, beliefID, action)
      return wrapIf(sqlStateFKViolation, ErrActionOnUnpromoted, err)
  }
  ```
  In [`kernel/sql.go`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L88-L95):
  ```sql
  sqlRetractCascadeCancel = descendantsCTE + `
      UPDATE action_intent SET state = 'cancelled'
      WHERE id IN (
          SELECT a.id FROM action_intent a
          JOIN d ON a.belief_id = d.id
          WHERE a.state = 'live' AND a.scenario_id = $2::UUID
      )`
  ```
* **Attack/Failure Sequence**:
  1. Scenario $A$ owns Belief $b_A$, which is promoted.
  2. A caller in Scenario $B$ invokes `st.IntentOnPromoted(ctx, scB, b_A, "planted intent")`.
  3. `createIntentWithinTx` inserts into `action_intent` with `scenario_id = scB` and `belief_id = b_A`. Because the foreign key `CONSTRAINT gate` on `action_intent` is only `(belief_id, belief_status) REFERENCES belief(id, status)` without `scenario_id`, CockroachDB accepts the insert!
  4. Later, evidence arrives in Scenario $A$ requiring retraction of $b_A$.
  5. The operator calls `RetractCascade(ctx, scA, b_A)`.
  6. `RetractCascade` executes `sqlRetractCascadeCancel` with `$2 = scA`. This query only cancels live intents `WHERE a.scenario_id = scA`. The live intent in Scenario $B$ is **not cancelled**!
  7. `RetractCascade` then executes `sqlRetractCascadeRetract`:
     `UPDATE belief SET status = 'retracted' WHERE id IN (SELECT id FROM d)`
  8. CockroachDB's `ON UPDATE CASCADE` cascades `belief_status = 'retracted'` to the uncancelled intent in Scenario $B$.
  9. CockroachDB constraint `CONSTRAINT live_requires_promoted CHECK (state <> 'live' OR belief_status = 'promoted')` fires immediately with `SQLSTATE 23514`.
  10. The `RetractCascade` transaction rolls back.
  11. **Belief $b_A$ CANNOT BE RETRACTED.** It remains permanently locked in `promoted` status in Scenario $A$.
* **Observed Evidence**:
  Executed standalone test against CockroachDB (`fable_kernel_test`):
  ```text
  Planted live intent in scB for bID(scA) successfully!
  Victim RetractCascade in scA: retracted=0, err=ERROR: failed to satisfy CHECK constraint ((state != 'live':::STRING) OR (belief_status = 'promoted':::STRING)) (SQLSTATE 23514)
  ```
* **Reproduction Command**:
  `go run /home/chaschel/.gemini/antigravity-ide/brain/0ea77358-fe0a-4775-9eaf-66c567431da1/scratch/test_retract_dos.go`
* **Impact**:
  Catastrophic failure of the core thesis: *"Memory is refusing to act on what is no longer true."* Any tenant/agent with access to create intents in one scenario can permanently prevent retraction of falsified beliefs in another scenario.
* **Why Existing Controls Do Not Prevent It**:
  `createIntentWithinTx` and `IntentOnPromoted` were never updated to enforce scenario ownership of `belief_id`, and `action_intent` does not have a composite foreign key on `(scenario_id, belief_id)`.
* **Recommended Remediation**:
  In `createIntentWithinTx`, verify belief ownership in the same transaction before insert:
  ```go
  var exists bool
  if err := tx.QueryRowContext(ctx,
      `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)`,
      beliefID, scenarioID).Scan(&exists); err != nil {
      return err
  }
  if !exists {
      return ErrBeliefNotFound
  }
  ```

---

## Medium Findings

### NEW-03: Cross-Scenario Write via REST `POST /v1/evidence` and `kernel.AddEvidence`

* **ID**: NEW-03 (Scenario Isolation Gap)
* **Severity**: MEDIUM
* **Title**: `POST /v1/evidence` and `kernel.AddEvidence` permit inserting evidence rows for beliefs owned by different scenarios
* **Location**: [`api/evidence.go:9-41`](file:///home/chaschel/Documents/go/solvent-main/api/evidence.go#L9-L41), [`kernel/kernel.go:80-86`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel.go#L80-L86), [`kernel/sql.go:17-20`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L17-L20)
* **Exact Code**:
  In [`api/evidence.go`](file:///home/chaschel/Documents/go/solvent-main/api/evidence.go#L35-L39):
  ```go
  if err := s.ledger.AddEvidence(r.Context(), req.ScenarioID, req.BeliefID,
      req.ProvenanceClass, req.SourceURL, req.ContentSHA256); err != nil {
      writeKernelError(w, err, "")
      return
  }
  ```
  In [`kernel/sql.go`](file:///home/chaschel/Documents/go/solvent-main/kernel/sql.go#L17-L20):
  ```sql
  sqlAddEvidence = `
      INSERT INTO evidence
        (scenario_id, belief_id, provenance_class, source_url, content_sha256)
      VALUES ($1::UUID, $2::UUID, $3::STRING, $4::STRING, $5::STRING)`
  ```
* **Attack/Failure Sequence**:
  1. Caller authenticated with Scenario $B$ sends `POST /v1/evidence` with `belief_id` belonging to Scenario $A$.
  2. Unlike `handleRetireDebt` and `handlePromoteBelief`, `handleAddEvidence` contains no `view.GetSnapshot` ownership guard.
  3. `kernel.AddEvidence` executes `sqlAddEvidence` without verifying `SELECT EXISTS(SELECT 1 FROM belief WHERE id = $1::UUID AND scenario_id = $2::UUID)`.
  4. The evidence row is inserted into the database with `scenario_id = scB` and `belief_id = b_A`.
  5. The API returns `201 Created`.
* **Observed Evidence**:
  Executed standalone test against CockroachDB (`fable_api_test`):
  ```text
  REST POST /v1/evidence status code: 201
  Response body: {"evidence_id":"","belief_id":"c39d3b95-89dc-41c9-9dd8-be92ac7a5b9d","provenance_class":"external_feed","source_url":"https://example.com/exploit","content_sha256":"deadbeef1234","ingested_at":"0001-01-01T00:00:00Z"}
  Evidence rows in scB for bID(scA): 1
  ```
* **Reproduction Command**:
  `go run /home/chaschel/.gemini/antigravity-ide/brain/0ea77358-fe0a-4775-9eaf-66c567431da1/scratch/test_rest_evidence_cs.go`
* **Impact**:
  Cross-scenario write: arbitrary evidence can be attached to beliefs in foreign scenarios, poisoning telemetry trails and audit logs across tenant boundaries.
* **Why Existing Controls Do Not Prevent It**:
  Plan 8.4 only added guards to `RetireDebt`, `Promote`, and `Discharge`. `AddEvidence` was overlooked.
* **Recommended Remediation**:
  Add `view.GetSnapshot` guard in [`api/evidence.go`](file:///home/chaschel/Documents/go/solvent-main/api/evidence.go) and add `SELECT EXISTS` check in `kernel.AddEvidence`.

---

## Low Findings

### F-06: Typed MCP Argument Widening

* **ID**: F-06 (Historical / Accepted Low)
* **Severity**: LOW
* **Title**: MCP handlers parse typed arguments as loose `string`, relying on downstream runtime validation
* **Location**: [`cmd/solvent-mcp/tools.go:110`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L110), [`cmd/solvent-mcp/tools.go:613`](file:///home/chaschel/Documents/go/solvent-main/cmd/solvent-mcp/tools.go#L613)
* **Status**: ACCEPTED LOW.
* **Notes**: `handleSolventRetireDebt` widens `debt_item` from the schema enum to string, but validates with `slices.Contains(kernel.FullDebt, item)`. `handleSolventDischarge` accepts any string `obligation_key`, which is accepted because discharge records are keyed by arbitrary obligations.

### NEW-04: `gofmt` Indentation Regression in `kernel/kernel_test.go` Breaks `task test`

* **ID**: NEW-04 (Regression)
* **Severity**: LOW
* **Title**: Spaces used instead of tabs in newly added Plan 8.5 test cases cause `task test` to fail at the `gofmt` gate
* **Location**: [`kernel/kernel_test.go:748-752, 782`](file:///home/chaschel/Documents/go/solvent-main/kernel/kernel_test.go#L748-L782)
* **Observed Evidence**:
  `gofmt -l cmd internal kernel` reports `kernel/kernel_test.go`.
  `task test` fails at:
  ```text
  task: [test] test -z "$(gofmt -l cmd internal kernel)" || (echo "gofmt check failed" && exit 1)
  gofmt check failed
  task: Failed to run task "test": exit status 1
  ```
* **Reproduction Command**:
  `task test` or `gofmt -l cmd internal kernel`
* **Impact**:
  Blocks automated verification and CI pipelines from passing.
* **Recommended Remediation**:
  Run `gofmt -w kernel/kernel_test.go`.

### NEW-05: `scripts/mcp_verify.sh` Default DSN Targets Unmigrated `fable` Database

* **ID**: NEW-05 (Operational / Test Harness Defect)
* **Severity**: LOW
* **Title**: `scripts/mcp_verify.sh` fails out-of-the-box because default DSN connects to `fable` which lacks migrations 005-008
* **Location**: [`scripts/mcp_verify.sh:32`](file:///home/chaschel/Documents/go/solvent-main/scripts/mcp_verify.sh#L32), [`Taskfile.yml:23-31`](file:///home/chaschel/Documents/go/solvent-main/Taskfile.yml#L23-L31)
* **Observed Evidence**:
  Running `bash scripts/mcp_verify.sh` against default DSN produces:
  ```text
  stderr: schema validation failed error="expected 7 authority tables, found 0 — run db/ migrations"
  ```
  `Taskfile.yml`'s `db:reset` task only applies migrations `001` through `004`. When pointed to an authority-migrated database (`fable_mcp_test`), `mcp_verify.sh` passes 100% (34/34 assertions green).
* **Recommended Remediation**:
  Update `Taskfile.yml`'s `db:reset` to apply migrations `005` through `008`.

---

## Accepted Architectural Limitations

### F-01: Trusted Local Stdio MCP Boundary

* **Status**: ACCEPTED ARCHITECTURAL LIMITATION.
* **Analysis**: `cmd/solvent-mcp/main.go:73-87` explicitly verifies that `MCP_TRANSPORT` is either empty or `"stdio"`. Setting `MCP_TRANSPORT=sse` fails closed with:
  ```text
  unsupported MCP transport "sse": only stdio is supported
  MCP is a trusted local surface; network exposure transfers responsibility to the deployment boundary
  ```
  Within the trusted-local stdio model, the implementation upholds expected boundaries.

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

## Remediation Verification

| Former Finding | Current Status | Behavioral Evidence |
| -------------- | -------------- | ------------------- |
| M-01 | REGRESSED / INCOMPLETE | RetireDebt, Promote, and Discharge enforce scenario isolation at DB and handler levels. However, `IntentOnPromoted` (NEW-02) and `AddEvidence` (NEW-03) permit cross-scenario writes, with NEW-02 causing a fatal retraction DoS. |
| M-02 | REGRESSED / INCOMPLETE | Precondition `intentID != ""` is enforced. However, `ExecuteAction` accepts arbitrary non-empty `intentID` without validating that it matches `beliefID` or `action` (NEW-01). |
| L-01 | CLOSED | `solvent_discharge` schema includes `scenario` enum, handler validates belief in scenario via `view.GetSnapshot`, and kernel requires `scenarioID`. Verified by `TestCS_Discharge_WrongScenario_MCP`. |
| L-05 | CLOSED | `ReconcileIntent` executes kernel state transition before emitting audit logs. Failed transitions emit `ActivityReconciliationFailed` and preserve original error. Verified by `TestReconcile_CompletedAudit` and `TestReconcile_FailedAudit`. |

---

## New Findings

1. **NEW-01 (HIGH)**: `ExecuteAction` accepts arbitrary same-scenario `intentID`, permitting execution of un-intent-backed actions by consuming unrelated intents across beliefs and actions.
2. **NEW-02 (HIGH)**: `IntentOnPromoted` allows creating intents for beliefs in other scenarios, triggering an unrecoverable `SQLSTATE 23514` check-constraint deadlock on `RetractCascade`.
3. **NEW-03 (MEDIUM)**: Cross-scenario write via REST `POST /v1/evidence` and `kernel.AddEvidence`.
4. **NEW-04 (LOW)**: Indentation regression in `kernel/kernel_test.go` breaks `task test` at `gofmt` check.
5. **NEW-05 (LOW)**: `scripts/mcp_verify.sh` fails on default unmigrated `fable` database.

---

## Verification Results

| Verification Check | Outcome | Details / Notes |
| ------------------ | ------- | --------------- |
| `go build ./...` | **PASS** | Exited 0 with no warnings or errors. |
| `go vet ./...` | **PASS** | Exited 0 with no diagnostics. |
| `go test -count=1 -p 1 ./...` | **PASS** | Exited 0. All 18 test packages passed (including all 15 DB-backed packages against CockroachDB). |
| `go test -race -count=1 -p 1 ./...` | **PASS** | Exited 0 across the entire repository with zero data races. |
| `bash scripts/check_i7.sh` | **PASS** | Exited 0: 21 ExecuteTx write sites, 0 raw writes, 3 permitted pool reads. |
| `task test` | **FAIL** | Exited 201 (status 1): `gofmt check failed` due to indentation in `kernel/kernel_test.go` (NEW-04). |
| `bash scripts/mcp_verify.sh` (default DSN) | **FAIL (BLOCKED BY ENV/CONFIG)** | Exited 1: default DSN `fable` lacks migrations 005-008 (NEW-05). |
| `bash scripts/mcp_verify.sh` (migrated DSN) | **PASS** | Exited 0: all 34 assertions green. |

---

## Security Invariant Assessment

* **Scenario Isolation**: **FAIL**. While RetireDebt, Promote, and Discharge are isolated, `IntentOnPromoted` and `AddEvidence` lack scenario binding, permitting cross-scenario mutation.
* **Authority Integrity**: **PASS**. Target creation, activation, snapshot hashing, approval pinning, and revocation enforcement remain strictly sound. No bypass of `kernel.Authorize` was identified.
* **Intent Ownership**: **FAIL**. Broken by NEW-01: any live intent in a scenario can be substituted to execute an action on a different belief or action tuple.
* **Action / Target Binding**: **PASS**. Target activation and snapshot parameter binding remain immutable once approved.
* **Executor Integrity**: **PASS**. Resolves exclusively from internal registry; no caller-supplied executor strings.
* **Discharge Atomicity**: **PASS**. `kernel.Discharge` executes existence check, `debt_discharge` insert, and `belief.debt` update in one `crdb.ExecuteTx` boundary. While CS-6 (forced mid-transaction cancel) was deferred due to test harness timing flakiness, structural atomicity is sound.
* **Audit Truthfulness**: **PASS**. `ReconcileIntent` audit order is now truthful (L-05 resolved).
* **MCP Boundary**: **PASS**. Runs on stdio only; fails closed on network transports; no direct database writes.
* **DB Invariants**: **FAIL**. The retraction cascade invariant (I-8) can be broken via cross-scenario intent creation (NEW-02), triggering a 23514 deadlock.
* **Concurrency / TOCTOU**: **PASS**. `AuthorizeAndCreateIntent` serializes against `RevokeTarget` using `SELECT FOR UPDATE` on `authority_target`. `ClaimIntent` uses atomic CAS (`WHERE state = 'live'`).

---

## Final Assessment

The current repository HEAD is **NOT ready to advance**. 

The implementation has established significant verified progress: 15 DB-backed packages pass against CockroachDB, race detection is clean, and the core authority snapshot/approval model is robust. However, the scenario isolation remediation in Plan 8.4 was incomplete, leaving `IntentOnPromoted` and `AddEvidence` unprotected, and the `ExecuteAction` `intentID` validation introduced in Plan 8.4 merely checked for non-empty string without binding the intent to the belief and action being executed.

Before HEAD can be approved:
1. `kernel.ClaimIntent` and `ExecuteAction` must enforce that `intentID` matches `beliefID` and `action` (resolving NEW-01).
2. `createIntentWithinTx` / `IntentOnPromoted` must verify that the target belief belongs to `scenarioID` (resolving NEW-02).
3. `AddEvidence` and `POST /v1/evidence` must enforce belief scenario ownership (resolving NEW-03).
4. `kernel/kernel_test.go` must be formatted with `gofmt` so `task test` passes (resolving NEW-04).