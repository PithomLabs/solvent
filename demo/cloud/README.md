# Solvent Cloud Demo

Hosted, read-only, judge-facing version of the Solvent transactional belief ledger.

**Live:** https://byb43s8nh2.us-west-2.awsapprunner.com

**Region:** the `great-goat` cluster's `fable` database has primary region **`aws-us-west-2`**
(secondaries `aws-us-east-1`, `aws-us-east-2`), matching App Runner's **`us-west-2`** — so no request
in the demo path pays a cross-region round trip. Measured with `SHOW REGIONS FROM DATABASE`; the
transcript is in `proof/act6_tier_probe.log`.

## Architecture

```
Judge's browser
       │
       ▼
  AWS App Runner (Go web app)
       │
       ▼
  CockroachDB Serverless (Solvent ledger)
```

## Prerequisites

- AWS CLI configured
- Docker installed
- CockroachDB Serverless cluster with connection string
- `kubectl` (optional, for ECS fallback)

## Environment Variables

| Variable | Description | Example |
|---|---|---|
| `FABLE_DSN` | CockroachDB connection string | `postgresql://user:pass@host:26257/fable?sslmode=verify-full&sslrootcert=cc-ca.crt&options=--cluster=cluster-name` |
| `PORT` | HTTP listen port (default: 8080) | `8080` |

## Deployment

### 1. Seed the database (one-time)

```bash
# Set your CockroachDB connection string
export FABLE_DSN="postgresql://..."

# Run the init command (idempotent)
go run ./demo/cloud/init/
```

Expected output:
```
Applying schema...
Seeding track1 fixtures...
Track1: processed 3 evidence items
Seeding track2 fixtures...
Track2: processed 2 evidence items

=== Seed Complete ===
Beliefs:  3
Evidence: 5
Intents:  1
```

### 2. Build and push Docker image

```bash
# Build
docker build -t solvent-web .

# Tag for ECR
docker tag solvent-web:latest <AWS_ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/solvent-web:latest

# Push
aws ecr get-login-password --region <REGION> | docker login --username AWS --password-stdin <AWS_ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com
docker push <AWS_ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/solvent-web:latest
```

### 3. Deploy to AWS App Runner

```bash
# Create App Runner service
aws apprunner create-service \
  --service-name solvent-demo \
  --source-configuration '{
    "ImageRepository": {
      "ImageIdentifier": "<AWS_ACCOUNT_ID>.dkr.ecr.<REGION>.amazonaws.com/solvent-web:latest",
      "ImageRepositoryType": "ECR",
      "ImageConfiguration": {
        "Port": "8080"
      }
    },
    "AuthenticationConfiguration": {
      "AccessRoleArn": "<ECR_ACCESS_ROLE_ARN>"
    }
  }' \
  --instance-configuration '{
    "Cpu": "1 vCPU",
    "Memory": "2 GB"
  }' \
  --runtime-configuration '{
    "RuntimeEnvironmentVariables": {
      "FABLE_DSN": "<YOUR_COCKROACHDB_DSN>"
    }
  }'
```

### 4. Verify

```bash
# Health check
curl https://byb43s8nh2.us-west-2.awsapprunner.com/health

# Expected: {"status":"ok","database":"connected"}

# Landing page
curl -s https://byb43s8nh2.us-west-2.awsapprunner.com/ | head -5
```

### Required: a Bedrock instance role

`/demo` search calls Amazon Bedrock at request time. App Runner grants the container no AWS
access by default — `AppRunnerECRAccessRole` only pulls the image — so without an **instance
role** the search returns an explicit permission error naming the missing permission. There is
no fallback by design: no cached vectors, no synthetic embedding, nothing that would make a
broken deployment look healthy.

```bash
# 1. a role trusting tasks.apprunner.amazonaws.com
# 2. a policy allowing ONLY bedrock:InvokeModel on
#    arn:aws:bedrock:us-west-2::foundation-model/amazon.titan-embed-text-v2:0
# 3. attach it to the service
aws apprunner update-service --service-arn <arn> \
  --instance-configuration InstanceRoleArn=<role-arn>
```

Verify by evaluation rather than by reading the policy — the negative case is the useful half:

```bash
aws iam simulate-principal-policy --policy-source-arn <role-arn> \
  --action-names bedrock:InvokeModel \
  --resource-arns arn:aws:bedrock:us-west-2::foundation-model/amazon.titan-embed-text-v2:0
# allowed;  and implicitDeny for any other model
```

`task deploy` re-asserts this role after every `update-service` and fails if it is missing,
because `--source-configuration` replaces rather than merges.

## Fallback: ECS Fargate

If App Runner cannot reach CockroachDB:

```bash
# Build
docker build -t solvent-web .

# Push to ECR (same as above)

# Run on ECS Fargate with public IP
# Use the existing Dockerfile.ecs pattern if available,
# or create an ECS task definition with:
#   - Image: solvent-web
#   - Port: 8080
#   - Environment: FABLE_DSN
#   - Network: public IP
```

## Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/` | GET | Landing page with live ledger counts |
| `/ledger` | GET | Ledger summary |
| `/beliefs` | GET | List all beliefs |
| `/belief/:id` | GET | Belief detail with evidence and intents |
| `/evidence` | GET | List all evidence |
| `/intents` | GET | List all action intents |
| `/audit` | GET | Safety audit (live intents on unpromoted beliefs) |
| `/health` | GET | Health check (real DB ping) |
| `/demo` | GET | **The three-screen decision wizard** (`internal/wizard`) — the demo a judge should open |
| `/demo/api/*` | POST | The wizard's actions: search, select, discharge, promote, authorize, introduce, retract, reset |
| `/proof` | GET | The three-cell isolation experiment, transcripts embedded at build time |

`GET /demo` renders only; it writes nothing. A scenario is created on the visitor's first
*action*, so crawlers, uptime monitors and link previews cannot author ledger rows.

## Verification Checklist

- [x] `/health` returns `{"status":"ok","database":"connected"}`
- [x] Landing page shows live counts (not hardcoded)
- [x] All pages render correctly
- [x] Public URL accessible from external machine
- [x] Writes are confined to the wizard's own per-visitor scenario — the ledger pages are
      read-only and scenario-filtered, pinned by W-32
- [x] `GET` is non-mutating: 60 cookie-less GETs write nothing (W-31)
- [x] No runtime external feeds — `/demo/api/introduce` resolves issues from the ingested
      corpus and has no HTTP client
- [x] Seed survives restart (no re-seed)
- [x] Second seed attempt is idempotent

## Local Development

```bash
# Start local CockroachDB
docker run -d --name solvent-crdb -p 26260:26257 cockroachdb/cockroach:v26.2.0 start-single-node --insecure --accept-sql-without-tls

# Create database and apply schema
docker exec solvent-crdb cockroach sql --insecure -e "CREATE DATABASE fable"
docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/001_schema.sql

# Seed
FABLE_DSN="postgresql://root@localhost:26260/fable?sslmode=disable" go run ./demo/cloud/init/

# Run web app
FABLE_DSN="postgresql://root@localhost:26260/fable?sslmode=disable" go run ./demo/cloud/web/
```

## What This Is Not

- This is **not** a new Solvent implementation. Every write goes through the existing kernel.
- This is **not** a physics adapter.
- This is **not** a live agent endpoint.

**It is no longer read-only.** Earlier revisions of this file described the service as a
"read-only cloud packaging" and ticked "No write operations exposed". That stopped being true
when `/demo` shipped: the wizard writes beliefs, evidence, citations, intents and refusals
through the kernel, which is the entire point of it — a judge drives real transactions and the
database refuses them. Writes are confined to the visitor's own `scenario_id`; the four
read-only ledger pages are filtered to Track 2 and cannot be moved by wizard traffic.
