# Solvent Cloud Demo

A transactional belief ledger for autonomous agents, deployed as a public web application backed by CockroachDB Serverless.

## What This Is

Solvent is a **database-enforced memory system** where autonomous agents cannot act on beliefs that are no longer true. The web demo exposes the verified kernel through a read-only HTTP interface.

## Architecture

```
[Browser] → [AWS App Runner] → [Go web server] → [CockroachDB Serverless]
                                    ↓
                              [Seed pipeline] (init only)
```

**Key constraint:** The database — not the application — determines truth. The web app never writes to the database. It only reads.

## What Was Built

1. **Cloud web app** (`demo/cloud/web/`) — 8 read-only endpoints, 7 HTML templates
2. **Idempotent seed** (`demo/cloud/init/`) — 42P07-tolerant, survives restart
3. **Dockerfile** — multi-stage build (Go 1.25.0 → Alpine runtime)
4. **AWS deployment** — ECR + App Runner, public URL live

## Endpoints

| Endpoint | Description |
|---|---|
| `/` | Landing page with live ledger counts |
| `/beliefs` | List all beliefs |
| `/belief/:id` | Belief detail with evidence and intents |
| `/evidence` | List all evidence |
| `/intents` | List all action intents |
| `/ledger` | Ledger summary |
| `/audit` | Safety audit (live intents on unpromoted beliefs) |
| `/health` | Health check (real DB ping) |

## Public URL

**https://byb43s8nh2.us-west-2.awsapprunner.com**

## Challenges Overcome

### 1. Vector Not Implemented

**Problem:** The original plan assumed Vector (pgvector) was implemented as a CockroachDB tool. Investigation revealed it was not — no vector files, no embedding columns, no vector queries exist in the codebase.

**Solution:** Pivoted to a two-tool path:
- Tool #1: CockroachDB Managed MCP (investigated, endpoint confirmed)
- Tool #2: ccloud CLI (real interaction, JSON receipt)

### 2. Managed MCP Requires Service Account API Key

**Problem:** The Managed MCP server at `https://cockroachlabs.cloud/mcp` requires OAuth or a service account API key for autonomous use.

**Solution:** Investigated and documented the endpoint, cluster ID (`*****`), and configuration. Requires service account creation in CockroachDB Cloud Console for full autonomous use.

### 3. .env Variable Naming Mismatch

**Problem:** The `.env` file uses `dsn=` but the web server expects `FABLE_DSN`. First App Runner deployment failed because the container crashed on startup (`FABLE_DSN not set`).

**Solution:** Used Python to read `dsn=` from `.env` and inject it as `FABLE_DSN` in the App Runner config. Deleted failed service, recreated with correct env var.

### 4. App Runner IAM Role Trust Policy

**Problem:** App Runner needs an IAM role with ECR read access. The trust policy must allow `build.apprunner.amazonaws.com`.

**Solution:** Created `AppRunnerECRAccessRole` with the correct trust policy and attached `AmazonEC2ContainerRegistryReadOnly`.

### 5. App Runner Service Creation Timing

**Problem:** Attempted to update the service with env vars while it was still in `OPERATION_IN_PROGRESS`. AWS rejected the update.

**Solution:** Deleted the failed service and recreated it with the env var included from the start (using `--cli-input-json`).

### 6. Frozen Architecture Preservation

**Problem:** The `db/`, `internal/`, and `cmd/` directories are frozen from previous waves. Any changes risk breaking verified invariants.

**Solution:** All new code lives in `demo/cloud/`. No modifications to frozen paths. Verified with `git diff --stat HEAD -- db/ internal/ cmd/` (empty).

### 7. Idempotent Seeding on Serverless

**Problem:** Seed command must survive restarts without duplicating data or failing with 42P07 (relation already exists).

**Solution:** Schema creation uses `IF NOT EXISTS`. Seed checks `SELECT count(*) FROM belief` before inserting. If beliefs exist, prints "Already seeded: X beliefs found. No-op."

## CockroachDB Tools Demonstrated

### Tool #1 — Managed MCP

- **Endpoint:** `https://cockroachlabs.cloud/mcp`
- **Cluster:** `*****`
- **Transport:** HTTP (HTTPS)
- **Status:** Investigated, confirmed

### Tool #2 — ccloud CLI

- **Auth:** Gani Mendoza (Pithom Labs)
- **Clusters:** great-goat (AWS), silver-fish (GCP)
- **Status:** PASS with JSON receipt

## Serverless Invariant Lifecycle

All invariants enforced on CockroachDB Serverless v26.2.5:

1. Promotion blocked with debt (SQLSTATE 23514) ✓
2. Promotion succeeds after debt retired ✓
3. Intent on promoted belief succeeds ✓
4. Audit = 0 (no live intents on unpromoted beliefs) ✓
5. RetractCascade cancels intent and retracts belief ✓
6. Final audit = 0 ✓

## Version Delta

- Local CRDB: v26.2.0
- Serverless CRDB: v26.2.5

## Files Created

```
demo/cloud/
├── README.md              # Deployment instructions
├── imp.md                 # Execution receipts
├── imp10_result.md        # Implementation result
├── solvent_demo.md        # This file
├── init/
│   └── main.go            # Idempotent seed command
└── web/
    ├── main.go             # Web server entry point
    ├── handlers.go         # HTTP handlers (8 endpoints)
    ├── funcs.go            # Template functions
    └── templates/
        ├── index.html      # Landing page with live counts
        ├── ledger.html     # Ledger summary
        ├── beliefs.html    # List all beliefs
        ├── belief_detail.html  # Belief detail + evidence + intents
        ├── evidence.html   # List all evidence
        ├── intents.html    # List all action intents
        ├── audit.html      # Safety audit
        └── warming_up.html # Cold-start graceful degradation
Dockerfile                  # Multi-stage Docker build
```

## Deployment

### Infrastructure

- **ECR:** `solvent-demo` (us-west-2)
- **App Runner:** `solvent-demo` (us-west-2)
- **Image:** `705029439164.dkr.ecr.us-west-2.amazonaws.com/solvent-demo:latest`
- **IAM:** `AppRunnerECRAccessRole`

### How to Update

```bash
# 1. Build new image
docker build -t solvent-demo:latest .

# 2. Tag and push
docker tag solvent-demo:latest 705029439164.dkr.ecr.us-west-2.amazonaws.com/solvent-demo:latest
docker push 705029439164.dkr.ecr.us-west-2.amazonaws.com/solvent-demo:latest

# 3. App Runner auto-deploys on new image (if AutoDeploymentsEnabled)
#    Or manually trigger: aws apprunner start-deployment --service-arn <ARN> --region us-west-2
```

## What This Is Not

- Not a new Solvent implementation
- Not a physics adapter
- Not a live agent endpoint
- Not a rewrite of the kernel

This is a **read-only cloud packaging** of the existing verified Solvent kernel.
