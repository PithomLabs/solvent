# Deployment Issues — Solvent AWS Demo (Solvent Demo v2)

## Summary

Attempted to deploy a faithful AWS port of the verified local Beat 7 / Track 2 walkthrough to CockroachDB Serverless (`great-goat` cluster). The local walkthrough verified successfully (3 beliefs, 3 evidence, 1 cancelled intent, audit=0). The cloud deployment encountered a cascade of infrastructure and permission issues that prevented the init from completing and the web server from serving correct data.

---

## What Worked

### Phase 0 — Local Walkthrough
- `task db:reset && task demo:track2 && task inspect` completed successfully.
- Final state: 3 beliefs (1 retracted postulated, 2 entered pipeline), 3 evidence, 1 cancelled intent, audit=0.
- Exact match with source trace prediction.

### Code Changes (All Compile Clean)
- `demo/cloud/init/main.go` — Rewritten to replay exact Beat 7 sequence with correct idempotence ordering.
- `demo/cloud/web/handlers.go` — Updated all SQL queries from scenario 1 (`...0001`) to scenario 2 (`...0002`). Added `Retracted` count to `ledgerCounts`. Fixed audit query JOIN syntax.
- `demo/cloud/web/templates/index.html` — Added Retracted column, updated grid to 5 columns.
- `demo/cloud/web/templates/ledger.html` — Added Retracted row.
- `Dockerfile` — Changed ENTRYPOINT to chain init→web: `["/bin/sh", "-c", "/app/solvent-init && exec /app/solvent-web"]`.

### Docker Build & ECR Push
- Docker multi-stage build succeeds consistently.
- ECR authentication and push succeed (`705029439164.dkr.ecr.us-west-2.amazonaws.com/solvent-demo`).

---

## Problems Encountered

### Problem 1: CockroachDB MCP Has No Write Access

| Tool | Result |
|---|---|
| `cockroachdb_create_database` | `insufficient permissions: write access required` |
| `cockroachdb-cloud_create_database` | `MCP error: unauthorized` |

The `managed-mcp` user can read from `great-goat` but cannot create databases, tables, or insert rows. This means all database setup must happen either:
- Via the init binary (which connects with the `bchat` user DSN), or
- Manually via the CockroachDB Cloud console.

### Problem 2: `bchat` User Lacks Admin Privileges

The DSN in `.env` and App Runner uses:
```
postgresql://bchat:ZkonNanP0R8MvKTOrTzChQ@great-goat-30894.j77.cockroachlabs.cloud:26257/fable?sslmode=verify-full
```

The `bchat` user was created for the old `bchat` database (Track 1). It does **not** have permission to:
- Connect to `defaultdb`
- Run `DROP DATABASE` or `CREATE DATABASE` on other databases

The init's `cliReset` function connects to `defaultdb` to DROP/CREATE the target database. This hangs or fails when the `bchat` user can't reach `defaultdb`.

**Resolution**: User manually created the `fable` database via the CockroachDB Cloud web console. The database now exists, owned by `root`, with `bchat` as table owner.

### Problem 3: Init Crash Loop (Root Cause of Failure)

After the database was created manually, the init ran against `fable`. Here's what happened:

1. **Step 1**: `pipeline.Run` ingested track2 fixtures → 2 beliefs created (`entered` status).
2. **Step 2**: `EnterBelief` created baseline postulated belief → 3rd belief (`entered` status). `AddEvidence` attached evidence.
3. **Step 3**: **FAILED** — The init crashed here. The `readActualDebts` function tried to scan a PostgreSQL `text[]` (ARRAY) column into a Go `[]string`. This likely panicked or returned an error that triggered `log.Fatalf`. No debts were retired, no promotion occurred, no intent was created.

**On container restart (App Runner auto-restart)**:
4. Init reconnected to `fable`.
5. `canonicalStateExists` returned `true` (3 beliefs exist).
6. `verifyTrack2` checked for 1 retracted baseline belief → found 0 → **FAILED**.
7. `log.Fatalf("SEED STATE CORRUPTED: ...")` → exit code 1.
8. Web server never starts (entrypoint uses `&&`).
9. **Crash loop**: init crashes → App Runner restarts → init crashes → repeat.

The web server serves the "warming up" page (503) because it never gets a chance to start.

### Problem 4: App Runner Image Caching

- `aws apprunner start-deployment` was used to trigger a redeploy, but it pulled a **cached** version of the `latest` tag.
- The web page still showed the old 4-column layout (no Retracted column) and all-zero counts — confirming the old code was running.
- **Fix**: Use unique numeric tags (e.g., `1786592105`, `1786593036`) instead of `latest` to force App Runner to pull a new image.

### Problem 5: App Runner API Instability

`aws apprunner update-service` intermittently returned:
```
aws: [ERROR]: An error occurred (ValidationException) when calling the CreateOAuth2Token operation:
The provided authorization grant is invalid, expired, revoked, or malformed
```

This happened even though `aws sts get-caller-identity` returned valid credentials. Likely causes:
- Previous `update-service` or `start-deployment` operation still in progress (`OPERATION_IN_PROGRESS`), blocking new update requests.
- AWS App Runner API rate limiting or transient auth issues.

When the API is available, the update succeeds. The pattern: wait for status to become `RUNNING`, then issue the update.

### Problem 6: Web Shows Zeros Despite Seeded Data

Database state via MCP:
```
belief: 3 rows (all status=entered, scenario=00000000-...-0002)
evidence: 3 rows
action_intent: 0 rows
```

But the web page showed 0/0/0/0.原因: The running container had the OLD handler code querying for scenario `...0001` (the old Track 1 scenario), while the data is for scenario `...0002`. The new handler code was in the Docker image but never deployed because of the image caching issue.

---

## Current State

| Component | Status |
|---|---|
| `fable` database | Created (user), tables applied (init), 3 beliefs + 3 evidence |
| App Runner service | RUNNING, serving stale code |
| Init binary | New version written (handles partial state via TRUNCATE), pushed to ECR but not deployed |
| Web server | Serving old 4-column template, all counts = 0 |
| Deployment | Blocked by App Runner OAuth errors on `update-service` |

---

## What Needs to Happen

1. **Get `update-service` to accept the new image tag.** Wait for any in-progress operation to clear, or use the AWS App Runner console to update the image manually to `705029439164.dkr.ecr.us-west-2.amazonaws.com/solvent-demo:1786593036`.

2. **Once deployed, the new init will:**
   - Connect to `fable`, detect 3 `entered` beliefs (canonical state exists).
   - Run `verifyTrack2` → fails (no retracted baseline).
   - Enter `resetAndSeed`: TRUNCATE all tables (works without `defaultdb` access), re-apply schema, re-run full Beat 7 sequence.
   - Verify final state: 3 beliefs (1 retracted), 1 cancelled intent, audit=0.

3. **Verify live URL** shows correct Track 2 reversal state:
   - Landing page: 3 beliefs, 3 evidence, 0 promoted, 1 retracted, 0 active intents
   - Beliefs page: 1 retracted (baseline), 2 entered (pipeline results)
   - Intents page: 1 cancelled intent
   - Audit page: SAFE

4. **Optional**: Grant `bchat` user admin privileges on the cluster so `cliReset` works for future resets. This would allow the init to drop/recreate `fable` without needing `defaultdb` access.

---

## Lessons Learned

1. **Always check MCP write permissions before relying on them.** The `cockroachdb_*` MCP tools are read-only for this cluster.
2. **Database users are scoped.** The `bchat` user was created for `bchat` database and lacks cross-database admin privileges.
3. **`TRUNCATE CASCADE` is the safe alternative to `DROP DATABASE`** when you can't access `defaultdb`. It works within the target database.
4. **App Runner caches images by tag.** Use unique tags to force redeployment. `start-deployment` may not pull a new image for `latest`.
5. **`log.Fatalf` in init prevents web server from starting.** The entrypoint `init && web` means init failure = no web server = crash loop.
6. **Partial state is the dangerous state.** The init must handle: tables exist, some data seeded, but walkthrough incomplete. Without TRUNCATE, this is unrecoverable without `defaultdb` access.

---

## Files Changed

| File | Change |
|---|---|
| `demo/cloud/init/main.go` | Rewritten: Beat 7 sequence, handles partial state via TRUNCATE |
| `demo/cloud/web/handlers.go` | Scenario 2, Retracted count, fixed audit JOIN |
| `demo/cloud/web/templates/index.html` | 5-column grid with Retracted |
| `demo/cloud/web/templates/ledger.html` | Retracted row |
| `Dockerfile` | ENTRYPOINT chains init→web |
