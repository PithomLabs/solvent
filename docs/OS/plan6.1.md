# Plan 6.1: Fix actor_id Identity Binding + Test Infrastructure

## Problem statement

Phase 4B implementation is complete but **not verified**. Two blocking issues prevent
the integration suite from passing:

1. **Security semantic defect:** The API authenticates the caller but allows the request
   body to choose `AuthorityTuple.PrincipalID`. This creates two different notions of
   identity — the authenticated principal and the authority principal — which is the
   exact ambiguity Phase 4A was designed to eliminate.

2. **Test infrastructure defects:** The API test suite connects to the wrong database,
   inserts into a nonexistent table, and cannot execute.

## Background: the identity flow that must hold

```
API credential
    ↓
authenticated principal
    ↓
effective PrincipalID
    ↓
AuthorityTuple.PrincipalID
    ↓
kernel.Authorize / AuthorizeAndCreateIntent
```

The current code breaks at step 2→3:

```go
// api/authorization.go:91-95 (current — broken)
principal := AuthFromContext(r.Context())
effectiveActor := req.ActorID                              // caller-controlled
if principal != nil && principal.PrincipalID != "api-key" { // dead code: always "api-key"
    effectiveActor = principal.PrincipalID
}
```

The auth middleware hardcodes `PrincipalID = "api-key"` for every request
(`api/auth.go:51`). The override condition `!= "api-key"` is always false.
The caller-supplied `actor_id` flows directly into authority evaluation.

A system principal is still a principal. It is not a magical permission to
impersonate arbitrary principals. Delegation/act-on-behalf semantics can be
added later as an explicit authenticated-and-authorized mechanism. Do not
smuggle delegation into `actor_id`.

## Scope boundary

- **No kernel changes.** The kernel remains identity-agnostic — it receives a
  tuple and evaluates it against the approved target snapshot.
- **No schema changes.** No new tables, no new columns.
- **No IAM/RBAC.** The API key → principal mapping is a flat configuration.
- **No MCP behavioral change.** MCP is a local stdio process; its trust boundary
  is the OS process, not API auth.

---

## Step 1: Change `AuthMiddleware` to accept key→principal mapping

**File:** `api/auth.go`

Change signature from:

```go
func AuthMiddleware(validKeys []string, next http.Handler) http.Handler
```

to:

```go
func AuthMiddleware(keyToPrincipal map[string]string, next http.Handler) http.Handler
```

The lookup becomes:

```go
principalID, ok := keyToPrincipal[token]
if !ok {
    writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid API key", nil)
    return
}
```

And the `AuthenticatedPrincipal` gets the real `principalID`:

```go
principal := &AuthenticatedPrincipal{
    PrincipalID:   principalID,
    PrincipalType: "service",
    AuthMethod:    "api-key",
    AuthTime:      time.Now(),
}
```

## Step 2: Update `cmd/solvent-api/main.go` to parse `key=<uuid>` format

**File:** `cmd/solvent-api/main.go`

Change `SOLVENT_API_KEYS` parsing from splitting on `,` (raw keys) to parsing
`key=<principal-uuid>,key2=<principal-uuid2>` format.

Use `strings.SplitN(entry, "=", 2)` for strict parsing. Reject malformed
entries (missing `=`, empty key, empty value) at startup rather than silently
skipping them.

Build `map[string]string` and pass to `AuthMiddleware`.

**Configuration scope:** A configured API key maps to exactly one Solvent
`principal_id`. Configuration does not create, mutate, revoke, or otherwise
manage principals. It is a static lookup table, not an identity-management
subsystem.

Example env:

```
SOLVENT_API_KEYS=key1=00000000-0000-0000-0000-000000000001,key2=00000000-0000-0000-0000-000000000002
```

## Step 3: Fix `handleAuthorizeAction` — bind to authenticated principal

**File:** `api/authorization.go`

Replace lines 83-95 with:

```go
principal := AuthFromContext(r.Context())
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal",
        "No authenticated principal", nil)
    return
}
effectiveActor := principal.PrincipalID
```

Remove the `actor_id` field validation (lines 83-86) and the dead override code
(lines 92-95).

If `req.ActorID != ""` and `req.ActorID != effectiveActor`, reject with:

```go
writeError(w, http.StatusForbidden, "actor_id_mismatch",
    "actor_id does not match authenticated principal", nil)
```

Keep `req.ActorID` in the request type for audit metadata — the service layer
logs it.

## Step 4: Fix `handleVerifyAuthorization` — bind tuple to authenticated principal

**File:** `api/authorization.go`

Replace line 24 (`PrincipalID: req.PrincipalID`) with a fail-closed nil check
and derivation from the authenticated principal:

```go
principal := AuthFromContext(r.Context())
if principal == nil {
    writeError(w, http.StatusUnauthorized, "missing_principal",
        "No authenticated principal", nil)
    return
}

tuple := kernel.AuthorityTuple{
    PrincipalID:           principal.PrincipalID,
    ResourceType:          req.ResourceType,
    ResourceID:            req.ResourceID,
    Scope:                 req.Scope,
    ActionNamespace:       req.ActionNamespace,
    ActionName:            req.ActionName,
    ConsequenceType:       req.ConsequenceType,
    ConsequenceParameters: req.ConsequenceParameters,
}
```

This is consistent: both authorization handlers derive identity from the
credential. A read-only verify still checks "am I authorized?" — not "is
some arbitrary principal authorized?"

## Step 5: Fix MCP handler stale comment

**File:** `cmd/solvent-mcp/tools.go`

Update the stale comment on lines 203-208 that still references the old
`PrepareForAction` flow. The MCP path is intentionally caller-controlled
(local stdio trust boundary) — document this explicitly:

```go
// handleSolventAuthorizeAction records a live intent to act on a belief.
// The database refuses unless the belief is currently promoted (SQLSTATE 23503).
//
// MCP trust boundary: this is a stdio-based local process. The actor_id comes
// from the tool arguments, not from authenticated credentials. Authority is
// enforced by the target/snapshot approval workflow, not by caller identity.
```

## Step 6: Fix test database DSN

**File:** `api/helpers_test.go`

Change DSN from:

```go
dsn := "postgres://root@localhost:26260/solvent?sslmode=disable"
```

to:

```go
const testDSN = "postgresql://root@localhost:26260/fable_test?sslmode=disable"
```

The `fable_test` database is the project's standard test database (see
`internal/testdb/testdb.go:22`).

## Step 7: Fix `createTestScenario` — remove INSERT INTO nonexistent table

**File:** `api/helpers_test.go`

The `scenario` table doesn't exist in the schema. Scenarios are just UUID
partition keys used as a column value in `belief.scenario_id`. Replace:

```go
func createTestScenario(t *testing.T, db *sql.DB) string {
    t.Helper()
    scenarioID := fmt.Sprintf("test-%d", time.Now().UnixNano())
    _, err := db.ExecContext(context.Background(),
        `INSERT INTO scenario (id, name, description) VALUES ($1, $2, $3)`,
        scenarioID, "test-scenario", "Test scenario for API tests")
    if err != nil {
        t.Fatalf("create scenario: %v", err)
    }
    return scenarioID
}
```

with:

```go
func createTestScenario(t *testing.T) string {
    t.Helper()
    return fmt.Sprintf("00000000-0000-0000-0000-%012d", time.Now().UnixNano())
}
```

No DB operation needed. Update all call sites to drop the `db` argument.

## Step 8: Update test helpers for `AuthMiddleware` signature

**File:** `api/helpers_test.go`

`newTestServer` currently passes `[]string{"test-api-key-12345"}`. Change to:

```go
keyMap := map[string]string{
    "test-api-key-12345": "00000000-0000-0000-0000-000000000001",
}
handler := api.AuthMiddleware(keyMap, server.Handler())
```

The test principal ID `00000000-0000-0000-0000-000000000001` must be used
consistently in all test cases that create targets/authorizations.

## Step 9: Remove misplaced compile-time check from `api/api.go`

**File:** `api/api.go`

Remove:

```go
var _ kernel.Contract = (*kernel.Store)(nil)
```

This check is in the wrong package context (api package asserting on kernel
type). If desired, move to `kernel/contract_test.go`.

## Step 10: Update integration tests for actor_id rejection

**File:** `api/integration_test.go`

`TestIntegration_AuthorizeAction_Atomicity` currently sends `actor_id` in
the request. After the fix:

- If `actor_id` matches the authenticated principal's ID → should succeed
- If `actor_id` conflicts → should get 403 `actor_id_mismatch`

Add a test case:

```go
func TestIntegration_AuthorizeAction_ActorIDMismatch(t *testing.T) {
    // Send actor_id that differs from the authenticated principal
    // Expect 403 actor_id_mismatch
}
```

## Step 11: Start CockroachDB + create test database

Apply the complete repository migration set (001 through 006), matching the
kernel suite (`kernel/suite_test.go:27`) and MCP suite
(`cmd/solvent-mcp/tools_authority_test.go:29-36`). Do not hand-select a
subset — the test database must hold the same schema shape as every other
environment.

```bash
docker start solvent-crdb
until pg_isready -h localhost -p 26260 2>/dev/null; do sleep 1; done

docker exec solvent-crdb cockroach sql --insecure \
  -e "CREATE DATABASE IF NOT EXISTS fable_test"

for f in 001_schema.sql 002_corpus.sql 003_wizard.sql \
         004_debt_vocabulary.sql 005_authority_mvp.sql \
         006_authority_justification_cascade.sql; do
  docker exec -i solvent-crdb cockroach sql --insecure --database=fable_test \
    < "db/$f"
done
```

## Step 12: Run full verification

```bash
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
```

---

## Files modified (summary)

| File | Change |
|---|---|
| `api/auth.go` | `AuthMiddleware` accepts `map[string]string`, sets real `PrincipalID` |
| `api/authorization.go` | Both handlers bind `PrincipalID` to authenticated principal |
| `api/helpers_test.go` | Fix DSN, remove scenario table insert, update `AuthMiddleware` call |
| `api/integration_test.go` | Add actor_id mismatch rejection test |
| `api/api.go` | Remove misplaced compile-time check |
| `cmd/solvent-api/main.go` | Parse `key=<uuid>` format for `SOLVENT_API_KEYS` |
| `cmd/solvent-mcp/tools.go` | Fix stale comment about `PrepareForAction` |

## What this plan does NOT change

- **Kernel** — remains identity-agnostic, receives tuple and evaluates it
- **Schema** — no new tables, no new columns
- **MCP behavior** — local stdio trust boundary, no auth layer
- **Service layer** — no changes needed; it already delegates to kernel

## Verification criteria

After execution, the following must all pass:

1. `go build ./...` — clean
2. `go vet ./...` — clean
3. `go test -count=1 -p 1 ./...` — all tests pass, including:
   - `TestIntegration_ConcurrentRevokeTarget` (concurrency regression)
   - `TestIntegration_AuthorizeAction_ActorIDMismatch` (new)
   - All existing kernel, API, and MCP tests
4. The `actor_id` in a request body cannot override the authenticated principal
