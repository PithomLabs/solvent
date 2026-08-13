# Solvent Cloud Demo

Hosted, read-only, judge-facing version of the Solvent transactional belief ledger.

**Live:** https://byb43s8nh2.us-west-2.awsapprunner.com

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

## Verification Checklist

- [x] `/health` returns `{"status":"ok","database":"connected"}`
- [x] Landing page shows live counts (not hardcoded)
- [x] All pages render correctly
- [x] Public URL accessible from external machine
- [x] No write operations exposed
- [x] No runtime external feeds
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

- This is **not** a new Solvent implementation.
- This is **not** a physics adapter.
- This is **not** a live agent endpoint.
- This is a **read-only cloud packaging** of the existing verified Solvent kernel.
