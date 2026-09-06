# Solvent GitHub Integration Example

Demonstrates `GitHub adapter → canonical Solvent API`. The integration calls
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
```

## Usage

```bash
go run . -url http://localhost:8080 -key your-api-key
```

## Architecture

The adapter translates GitHub events into `NormalizedEvidence` and calls the
Solvent API. It does not contain authorization logic — that lives in the kernel.
The adapter is a thin translation layer, not an execution engine.
