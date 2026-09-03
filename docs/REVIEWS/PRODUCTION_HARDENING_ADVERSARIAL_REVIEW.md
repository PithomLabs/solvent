# Production Hardening Adversarial Review

## Status
REVIEW ARTIFACT — NOT IMPLEMENTED

## Scope
Production hardening phase changes in commit `5fa2d79`:

- `cmd/solvent-mcp/main.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `kernel/authority.go`
- `kernel/authority_test.go`
- `kernel/errors.go`
- `kernel/kernel.go`
- `scripts/check_i7.sh`

Reviewed against the locked v0 kernel authority boundary defined by:
- `db/001_schema.sql` through `db/006_authority_justification_cascade.sql`
- `kernel/authority.go`
- `kernel/sql.go`
- `kernel/errors.go`
- `kernel/contract.go`
- `internal/testdb/testdb.go`

## Repository Truth
- `go test ./...` passes without `-p 1`.
- `go test -count=1 -p 1 ./...` passes.
- `go vet ./...` clean.
- `gofmt -l cmd internal kernel` clean.
- `git diff --check` clean.
- `db/001_schema.sql` through `db/006_authority_justification_cascade.sql` are unchanged.
- `scripts/check_i7.sh` `EXPECT_TX` updated from 7 to 16, matching the actual 16 `crdb.ExecuteTx` call sites in the kernel package (9 in `authority.go`, 7 in `kernel.go`).
- All packages that call `testdb.Reset` now acquire the reset lock in `TestMain`.
- `kernel/authority_test.go` adds T-C6, T-C7, T-C8, T-SR1.
- `cmd/solvent-mcp/tools_authority_test.go` adds MT-6 and reset-lock acquisition/release.

## Locked V0 Boundary
The kernel authority boundary is unchanged:

```
Approve = ONLY authority-creating operation
Authorize = READ-ONLY verification
RevokeTarget = INSERT target_revocation

Authority source = target_activation + target_snapshot + absence of target_revocation

FORBIDDEN as authority sources:
  authority_target proposal fields
  live justification rows
  authorization_decision
  warrant
  cache
  action_intent
```

## Hardening Changes Under Review
1. MCP server: structured logging, graceful shutdown, connection pool env vars, schema validation at startup, tool-call timing/logging.
2. MCP tests: reset lock acquisition/release in `TestMain`, new MT-6 test.
3. Kernel: idempotency warnings on `CreatePrincipal`, `CreateTarget`, `EnterBelief`, `IntentOnPromoted`; enhanced `RequestAuthorization` error diagnostics; removed unused `ErrTupleMismatch`.
4. Kernel tests: T-C6, T-C7, T-C8 (concurrency tests documenting known limitations), T-SR1 (approve without justification).
5. `check_i7.sh`: `EXPECT_TX` corrected from 7 to 16.

## Executive Verdict

**APPROVE**

The hardening phase preserves the locked v0 authority boundary in all respects. No schema changes were introduced. No forbidden features were added. `Authorize` remains read-only. `Approve` remains the sole authority-creating operation. All DB constraints remain authoritative. The test suite passes under normal `go test ./...` parallelism. The reset-lock fix prevents destructive concurrent resets of `fable_test`. Startup schema validation catches the two most important schema-version mistakes. Error semantics are improved. No second authority source was introduced.

One medium finding and two low findings exist, all documentation/observability issues, none blocking.

---

## Critical Findings

**None.**

## High Findings

**None.**

## Medium Findings

### HARD-1 — MEDIUM — Trusted-boundary warnings missing on three admin MCP tools

**Claim Attacked**
Tool descriptions for administrative operations must accurately state the trusted-surface requirement.

**Evidence**
- `solvent_create_principal` (`main.go:290`): "MCP principal-ID fields are attribution inputs, not authentication proof — the v0 MCP server must be deployed as a trusted administrative surface."
- `solvent_approve` (`main.go:417`): "Only call from a trusted administrative surface — the approved_by field is attribution, not caller authentication."
- `solvent_create_target` (`main.go:324`): "The created_by field records attribution, not caller identity."
- `solvent_revoke_principal` (`main.go:309`): No trust-boundary warning.
- `solvent_revoke_target` (`main.go:483`): No trust-boundary warning.
- `solvent_discharge` (`main.go:506`): No trust-boundary warning.

**Attack**
An operator scanning tool descriptions might conclude that `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` are safe for untrusted agent invocation because they lack the explicit trust warning present on other admin tools.

**Exact Failure Boundary**
Documentation inconsistency, not a security bypass. The kernel still enforces all constraints. The deployment contract (entire server is trusted) still covers these tools.

**Why It Fails or Holds**
Holds: The server-level deployment contract covers all tools. No authority can be manufactured without legitimate approval.

**Required Resolution**
Add consistent "trusted administrative surface" warnings to `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` tool descriptions.

**Blocking?**
No.

**Exit Test**
Visual inspection of tool descriptions.

---

## Low Findings

### HARD-2 — LOW — `truncate` can produce invalid UTF-8

**Claim Attacked**
The `truncate` helper (`main.go:698-703`) can cut a multi-byte UTF-8 sequence in the middle, producing invalid UTF-8 in log output.

**Evidence**
```go
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen]) + "..."
}
```

**Attack**
A tool result message containing multi-byte UTF-8 characters (e.g., emoji, CJK) is truncated at a byte boundary that splits a character. The resulting string is not valid UTF-8.

**Exact Failure Boundary**
Log output only. No security impact.

**Why It Fails or Holds**
Fails: `s[:maxLen]` operates on bytes, not runes. For ASCII-only messages this is harmless. For non-ASCII messages, the truncation can produce invalid UTF-8.

**Required Resolution**
Use `utf8.DecodeRune` or `strings.TrimSpace` with rune-aware truncation.

**Blocking?**
No. Log corruption is not a security defect.

**Exit Test**
Visual inspection of log output with non-ASCII messages.

---

### HARD-3 — LOW — `envInt` allows `0`, which means unlimited connections

**Claim Attacked**
`envInt` returns `0` if the environment variable is explicitly set to `"0"`, which `database/sql` interprets as unlimited connections.

**Evidence**
```go
func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
```

`db.SetMaxOpenConns(envInt("SOLVENT_DB_MAX_OPEN_CONNS", 25))`

**Attack**
An operator sets `SOLVENT_DB_MAX_OPEN_CONNS=0` thinking it means "no connections" or "default". The result is unlimited connections, which could exhaust the database under high concurrency.

**Exact Failure Boundary**
Operational misconfiguration, not a security bypass.

**Why It Fails or Holds**
Fails: `strconv.Atoi("0")` returns `0, nil`. The default is only returned for empty string or non-numeric values.

**Required Resolution**
Document that `0` means unlimited, or add a guard `if v <= 0 { return def }`.

**Blocking?**
No. Defaults are sensible.

**Exit Test**
Visual inspection of environment variable documentation.

---

## Test Harness / go test ./...

**Finding: CONFIRMED — normal `go test ./...` passes.**

```
go test ./...
ok  	github.com/PithomLabs/solvent/cmd/solvent-mcp	(cached)
ok  	github.com/PithomLabs/solvent/kernel	(cached)
...
```

No `-p 1` required. The reset lock prevents concurrent destructive resets of `fable_test`. All packages that call `testdb.Reset` now acquire the lock in `TestMain`.

**Classification: No defect.**

---

## Reset Lock

**Finding: CONFIRMED — lock protects all destructive resets.**

The lock is a file-based lock at `/tmp/{dbname}.reset.lock` using `os.O_CREATE|os.O_EXCL|os.O_WRONLY`. It blocks until acquired and is released after `m.Run()` completes.

All packages that call `testdb.Reset` now acquire the lock:
- `kernel/suite_test.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `internal/view/explain_test.go`
- `internal/wizard/seed_test.go`
- `internal/corpus/corpus_test.go`
- `internal/intent/intent_test.go`
- `internal/demoseed/demoseed_test.go`
- `internal/belief/belief_test.go`
- `internal/pipeline/pipeline_test.go`
- `cmd/corpus-ingest/embed_test.go`

Failure paths release the lock before `os.Exit`. The MCP test package was the one missing the lock; the hardening fixed it.

**Classification: No defect.**

---

## Transaction / Retry

**Finding: CONFIRMED — `crdb.ExecuteTx` still wraps all kernel writes.**

`check_i7.sh` reports 16 `crdb.ExecuteTx` call sites in the kernel package (9 in `authority.go`, 7 in `kernel.go`). This matches the corrected `EXPECT_TX=16`. No raw `db.Exec` or `db.Query` writes exist in the kernel.

The `RequestAuthorization` error hardening adds a diagnostic query inside the same SERIALIZABLE transaction. If the target is not found, revoked, or activated, the correct sentinel is returned. No new authority path is created.

**Classification: No defect.**

---

## Idempotency

**Finding: CONFIRMED — idempotency warnings are honest.**

The hardening added explicit comments to `CreatePrincipal`, `CreateTarget`, `EnterBelief`, and `IntentOnPromoted` documenting that they are NOT idempotent under client-timeout retries. This is an accepted v0 limitation. The kernel code itself did not change.

New tests T-C6 and T-C7 document these limitations behaviorally:
- T-C6: Concurrent `CreatePrincipal` produces two distinct principals (no business-key uniqueness).
- T-C7: Concurrent `RequestAuthorization` produces a deterministic pin hash.

No false idempotency claims were introduced.

**Classification: No defect.**

---

## MCP Startup

**Finding: CONFIRMED — startup validation fails closed.**

`validateSchema` checks:
1. All 7 authority tables exist.
2. `target_activation` has a unique index containing `target_id`.
3. `justification` has an FK with `ON UPDATE CASCADE`.

If any check fails, the server logs the error and exits with code 1. No partial authority server starts.

Connection pool configuration occurs after `db.PingContext` and before `validateSchema`. Invalid env var values silently fall back to defaults (25 max open, 5 max idle, 5-minute lifetime).

**Classification: No defect.**

---

## MCP StdIO Boundary

**Finding: CONFIRMED — protocol traffic and logs are separated.**

- Protocol traffic: stdout via `mcp.StdioTransport`.
- Application logs: stderr via `slog.NewTextHandler(os.Stderr, nil)`.
- Startup errors: stderr via `fmt.Fprintln(os.Stderr, ...)`.

No log output can corrupt MCP protocol traffic. No sensitive data (DSN, passwords, authority payloads) is logged.

**Classification: No defect.**

---

## Logging

**Finding: CONFIRMED — logging is diagnostic, not authoritative.**

The `toolHandler` wrapper logs tool name, duration, and error status. Logging failures (JSON unmarshal errors, type assertion failures) are silently ignored. They do not affect tool results or transaction outcomes.

One minor gap: if `json.Unmarshal` of the result body fails, no log entry is produced for that tool call. This is an observability gap, not a security issue.

**Classification: No defect.**

---

## Shutdown

**Finding: CONFIRMED — graceful shutdown is implemented.**

`signal.NotifyContext` cancels the context on SIGINT/SIGTERM. `defer stop()` cleans up the signal notification. `db.Close()` is deferred. When `server.Run` returns, the deferred calls execute.

A signal during an active tool call cancels the context, which propagates to database queries. The transaction is rolled back. The tool returns an MCP error. The server continues running until `server.Run` returns.

**Classification: No defect.**

---

## Schema Validation

**Finding: CONFIRMED — validation catches the two most important schema-version mistakes.**

`validateSchema` checks:
1. All 7 authority tables exist.
2. `target_activation` has `UNIQUE(target_id)`.
3. `justification` FK has `ON UPDATE CASCADE`.

If `db/006` is not applied, the justification FK check fails. If `db/005` is modified to remove `UNIQUE(target_id)`, the activation check fails. The server fails to start.

The validation does not check column types or data contents, which is acceptable for v0.

**Classification: No defect.**

---

## Connection Pool

**Finding: CONFIRMED — defaults are sensible; invalid env vars fall back safely.**

- `SOLVENT_DB_MAX_OPEN_CONNS`: default 25. Invalid/empty → 25. `0` → 0 (unlimited, see HARD-3).
- `SOLVENT_DB_MAX_IDLE_CONNS`: default 5. Invalid/empty → 5. `0` → 0 (no idle connections, acceptable).
- `SOLVENT_DB_CONN_MAX_LIFETIME`: default 5 minutes. Invalid/empty → 5 minutes.

Pool configuration occurs before any tool calls. No exhaustion or starvation risk under normal v0 concurrency.

**Classification: No defect (HARD-3 is a minor config documentation issue).**

---

## RequestAuthorization Errors

**Finding: CONFIRMED — error precedence is deterministic and correct.**

The hardened `RequestAuthorization` distinguishes:
1. Target not found (`ErrTargetNotFound`)
2. Target revoked (`ErrAlreadyRevoked`)
3. Target activated (`ErrAlreadyActivated`)

Precedence: not found > revoked > activated. This is a safe fallback order — the most fundamental error (target doesn't exist) is reported first.

The diagnostic query runs inside the same SERIALIZABLE transaction as the `UPDATE`, so it sees a consistent snapshot. No race condition.

**Classification: No defect.**

---

## Security Regression

**Finding: CONFIRMED — all existing security invariants remain intact.**

Verified:
- Exact tuple → ALLOW (DT-1, T-14)
- Principal mismatch → DENY (DT-2, T-15)
- Resource mismatch → DENY (DT-3, T-16)
- Scope mismatch → DENY (DT-4, T-17)
- Action mismatch → DENY (DT-5, T-18)
- Consequence mismatch → DENY (DT-6, T-19)
- Belief retraction → DENY (DT-7, T-22, T-C3)
- Accepted re-promotion gap (DT-8, T-23)
- Approval pin mutation → DENY (DT-9, T-07, T-08)
- Resurrection → reject (DT-10, T-10, T-12, T-P1)
- Duplicate discharge → reject (DT-11, T-24, T-C4, T-C8)
- Revoked approver → DENY (DT-12, T-26)
- Snapshot substitution → DB rejection (T-11, T-P3)
- Authorize → zero write (DT-13, T-28)
- Approval without justification → reject (T-SR1)
- Approve × Approve → exactly one activation (T-C1)
- Approve × AttachJustification → pin mismatch or activation (T-C2)

**Classification: No regression.**

---

## Scope Creep

**Finding: CONFIRMED — no forbidden v0 features introduced.**

Grep for `promotion_epoch`, `belief_promotion`, `policy_version`, `tenant_id`, `belief_tenant`, `credential`, `authorization_decision`, `warrant`, `execution_receipt` in changed files returns only:
- Comments explaining their absence.
- `warrant` used in natural language in MCP tool descriptions ("citing a belief as its warrant"), not as a new table or feature.
- `credential` in AWS credential chain comments in `internal/corpus/` and `internal/wizard/`, which are pre-existing and unrelated to v0 authority.

No new tables, columns, or kernel methods for deferred features.

**Classification: No defect.**

---

## Migration / Recovery

**Finding: CONFIRMED — no migration modifications.**

`db/001_schema.sql` through `db/006_authority_justification_cascade.sql` are unchanged. The hardening did not create any migration framework or automatic schema rewrite. `validateSchema` is a startup check, not a migration tool.

Fresh DB → migrations 001–006: works (tested by `testdb.Reset`).
Existing DB → 006 upgrade: works (tested by prior review).
Already-migrated DB → startup: `validateSchema` confirms correctness.

**Classification: No defect.**

---

## Deployment Boundary

**Finding: CONFIRMED — trusted-boundary requirement is documented, not technically enforced.**

The MCP server exposes all 16 tools on a single stdio server. The tool descriptions for `solvent_create_principal`, `solvent_approve`, and `solvent_create_target` explicitly state the trusted-surface requirement. The other admin tools lack this warning (HARD-1).

The v0 contract accepts this limitation: authentication is an external deployment responsibility. The code does not falsely claim that `principal_id` assertion equals caller authentication.

**Classification: Accepted v0 deployment limitation. HARD-1 is a documentation improvement.**

---

## Final Authority Attack Chain

**Finding: CONFIRMED — complete attack chain fails at the expected boundaries.**

Chain:
1. Create principal → succeeds (attribution, not auth).
2. Create promoted belief → succeeds.
3. Create target → succeeds.
4. Attach justification → succeeds.
5. Request authorization → succeeds.
6. Approve → succeeds (only authority-creating path).
7. Authorize with exact tuple → ALLOW.

Attacks:
- Modify proposal after pin → `ErrApprovalPinMismatch` (kernel).
- Mutate justification after pin → `ErrApprovalPinMismatch` (kernel).
- Retract belief → retraction succeeds, Authorize → DENY (kernel + DB CASCADE).
- Re-promote belief → old justification may revive (accepted v0 gap, documented).
- Revoke target → succeeds, Authorize → DENY (kernel).
- Second approval → `ErrAlreadyActivated` / SQLSTATE 23505 (DB unique constraint).
- Snapshot substitution → SQLSTATE 23503 (DB composite FK).
- Retry approval → `crdb.ExecuteTx` retries on 40001, re-evaluates all checks on fresh state.
- Retry discharge → `UNIQUE(belief_id, obligation_key, instrument_ref)` rejects duplicate.

No attack bypasses the kernel. No attack creates authority without legitimate approval.

**Classification: No defect.**

---

## Accepted V0 Gaps

- No `promotion_epoch` / `belief_promotion`: absent.
- No `policy_version`: absent.
- No multi-tenancy: absent.
- No cryptographic attestation: absent.
- No global cross-belief replay protection: per-belief only (by design).
- Re-promotion gap: reachable, documented in T-23 and DT-8.
- MCP trusted boundary: deployment-enforced, honestly documented.
- `CreatePrincipal`, `CreateTarget`, `EnterBelief`, `IntentOnPromoted` not idempotent: documented, accepted.

---

## Blocker Cross-Tab

| Finding ID | Severity | Blocking? |
|---|---|---|
| HARD-1 | MEDIUM | No |
| HARD-2 | LOW | No |
| HARD-3 | LOW | No |

**Unique blocker count: 0**

---

## Required Conditions Before Release

**None required.** The hardening phase preserves all v0 security boundaries.

Recommended (not blockers):
1. Add trusted-boundary warnings to `solvent_revoke_principal`, `solvent_revoke_target`, and `solvent_discharge` tool descriptions (HARD-1).
2. Fix `truncate` to handle multi-byte UTF-8 correctly (HARD-2).
3. Document that `SOLVENT_DB_MAX_OPEN_CONNS=0` means unlimited connections (HARD-3).

---

## Dogfood / Operational Readiness

**Confirmed:**
- `go test ./...` passes without `-p 1`.
- `go test -count=1 -p 1 ./...` passes.
- `go vet ./...` clean.
- `gofmt` clean.
- `git diff --check` clean.
- All kernel authority tests pass (T-01 through T-30, T-C1 through T-C8, T-P1 through T-P3, T-SR1).
- All MCP adapter tests pass (MT-1 through MT-6).
- All DocTrust-shaped dogfood tests pass (DT-1 through DT-14).
- No schema changes.
- No test weakening.
- No deferred features introduced.

---

## Final Gate

**HARDENING PASSED — V0 READY FOR CONTROLLED RELEASE**

The production hardening phase makes the v0 system operationally safer without weakening the authority boundary:
- Test harness parallelism is safe (reset lock).
- Startup fails closed on stale/missing schema.
- Error semantics are improved (RequestAuthorization diagnostics).
- Idempotency limitations are honestly documented.
- Security regression tests confirm all invariants.
- `check_i7.sh` correctly counts 16 transaction write sites.
- No second authority source was introduced.
- `Authorize` remains pure read.
- `Approve` remains sole authority creator.
- No schema or migration changes.

The three findings (HARD-1, HARD-2, HARD-3) are documentation/observability improvements, not security defects.

---

## Files Inspected

- `cmd/solvent-mcp/main.go`
- `cmd/solvent-mcp/tools.go`
- `cmd/solvent-mcp/tools_authority_test.go`
- `kernel/authority.go`
- `kernel/authority_test.go`
- `kernel/authority_dogfood_test.go`
- `kernel/sql.go`
- `kernel/errors.go`
- `kernel/contract.go`
- `kernel/kernel.go`
- `kernel/suite_test.go`
- `internal/testdb/testdb.go`
- `db/001_schema.sql`
- `db/002_corpus.sql`
- `db/003_wizard.sql`
- `db/004_debt_vocabulary.sql`
- `db/005_authority_mvp.sql`
- `db/006_authority_justification_cascade.sql`
- `scripts/check_i7.sh`

## Files Changed

**None.** This is a read-only review. No modifications were made.
