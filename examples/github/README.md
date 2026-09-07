# Solvent GitHub Integration Example

Demonstrates `GitHub adapter → canonical Solvent API → execution`. The integration calls
the REST API. GitHub-specific logic stays inside `adapter/github/`. The
integration is deliberately dumb — it orchestrates API calls in sequence, not
a workflow engine.

## Files

- `integration.go` — Reference integration calling the Solvent REST API

## Flow

```text
GitHub event → adapter/github → NormalizedEvidence
  → Solvent API (POST /v1/evidence with provenance_class="external_feed")
  → Solvent API (POST /v1/beliefs)
  → Solvent API (POST /v1/beliefs/{id}/promote)
  → Solvent API (POST /v1/targets)
  → Solvent API (POST /v1/targets/{id}/justifications)
  → Solvent API (POST /v1/targets/{id}/request-authorization)
  → Solvent API (POST /v1/targets/{id}/approve)
  → Solvent API (POST /v1/intents)
  → Solvent API (POST /v1/authorizations/execute)
```

## Usage

```bash
go run . -url http://localhost:8080 -key your-api-key
```

## Architecture

The adapter translates GitHub events into `NormalizedEvidence` and calls the
Solvent API. It does not contain authorization logic — that lives in the kernel.
The adapter is a thin translation layer, not an execution engine.

## Execution Boundary

The `POST /v1/authorizations/execute` endpoint is the REST execution boundary.
It claims a live intent, invokes the configured executor with the approved
snapshot parameters, and records the outcome. The authenticated principal is
derived from the API key — never from the request body.

## Authority Governance vs. Belief Truth

This example demonstrates **authority governance**, not belief truth.
The demo shows that the ledger controls whether an agent may act based on
evidence-backed beliefs and approved targets. Whether "etcd v3.5.x is safe"
is true is irrelevant — the ledger enforces that an agent cannot act on a
belief that has lost its authority.
