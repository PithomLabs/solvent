# Solvent Judge Playground — Implementation Plan

**Source:** `plans/demo/prompt.md`
**Date:** 2026-08-10
**Scope:** Taskfile + Docker CockroachDB + demo scripts + README. No schema/kernel/architecture changes.

---

## What exists today

- `scripts/m0_up.sh` / `m0_down.sh` — start/stop local `cockroach` binary (not Docker)
- `cmd/solvent` — pipeline runner with `--reset`, `--fixtures`, `--falsify`, `--scenario`
- `cmd/operator-review` — debt retirement + promotion + intent, with `--enter-claim` entry path
- `db/001_schema.sql` — frozen schema
- `internal/derive/testdata/etcd_real/` — track1/ and track2/ fixtures
- `task` binary installed, `docker` available, `cockroachdb/cockroach:v26.2.1` image already pulled
- No Taskfile, no Dockerfile, no docker-compose, no judge-facing README

## Deliverables

1. `Taskfile.yml` — primary command surface
2. `scripts/demo/setup.sh` — Docker-based CockroachDB + schema init
3. `scripts/demo/track1.sh` — Track 1 demo
4. `scripts/demo/track2.sh` — Track 2 demo
5. `scripts/demo/inspect.sh` — ledger snapshot
6. `README.md` — updated with judge-facing Quick Start
7. `plans/post/playground_implementation_report.md` — implementation report

## Taskfile commands

```
task setup           # Docker CRDB + schema + sanity check
task db:up           # Start pinned CRDB container
task db:down         # Stop/remove container
task db:reset        # Drop+recreate fable database, apply schema
task demo:track1     # Full Track 1: pipeline → operator-review → results
task demo:replay     # Replay Track 1, prove no duplicates
task demo:track2     # Full Track 2: enter baseline → review → postmortem → falsify
task inspect         # Human-readable ledger snapshot
task test            # Full verification suite
```

## CockroachDB setup mechanism (`scripts/demo/setup.sh`)

1. Check `docker` is available (fail clearly if not)
2. Check Go is available
3. `docker run` pinned `cockroachdb/cockroach:v26.2.1` — single-node insecure, port 26260, named container `solvent-crdb`
4. Wait for readiness (poll `SELECT 1`)
5. `docker exec` to create database `fable` and apply `db/001_schema.sql`
6. Verify connection
7. Print next commands

## Track 1 demo flow (`scripts/demo/track1.sh`)

```bash
# 1. Reset database via db:reset
# 2. Run pipeline on track1/
go run ./cmd/solvent --dsn "$FABLE_DSN" \
  --scenario 00000000-0000-0000-0000-000000000001 --reset \
  --fixtures internal/derive/testdata/etcd_real/track1
# 3. Query advisory belief ID
# 4. operator-review with --action
# 5. Print summary: beliefs, evidence, intents, audit
```

## Track 2 demo flow (`scripts/demo/track2.sh`)

```bash
# 1. Reset database
# 2. Run pipeline on track2/
# 3. operator-review --enter-claim ... --claim-type postulated --evidence-url ... --evidence-sha ...
# 4. operator-review --belief <ID> --debt ... (all 6) --action "deploy etcd v3.5.0"
# 5. solvent --falsify <ID> --scenario ...
# 6. Print summary: retracted belief, cancelled intent, audit
```

## Replay flow (`scripts/demo/replay.sh`)

```bash
# 1. Record counts before replay
# 2. Run track1 pipeline (same scenario, no --reset)
# 3. Record counts after replay
# 4. Assert no change → print "Replay verified: no duplicate ledger state created."
```

## Inspection (`scripts/demo/inspect.sh`)

```bash
# Query and format:
#   BELIEFS: id, scenario, status, claim, debt
#   EVIDENCE: id, belief, source_url, source_type
#   INTENTS: id, belief, action, state
#   AUDIT: live intents on non-promoted beliefs
```

## README.md structure

```markdown
# Solvent

One-sentence: transactional belief ledger for autonomous agents.

## Quick Start
    task setup

## Watch Track 1
    task demo:track1

## Replay It
    task demo:replay

## Watch Retraction
    task demo:track2

## Inspect the Ledger
    task inspect

## Run Tests
    task test

## For Engineers
    (links to AGENTS.md, schema, kernel, etc.)
```

## Execution order

1. Create `scripts/demo/` directory
2. Write `scripts/demo/setup.sh`
3. Write `scripts/demo/track1.sh`
4. Write `scripts/demo/replay.sh`
5. Write `scripts/demo/track2.sh`
6. Write `scripts/demo/inspect.sh`
7. Write `Taskfile.yml`
8. Update `README.md`
9. Run full acceptance sequence
10. Write `plans/post/playground_implementation_report.md`

## Scope boundaries

- No schema changes
- No kernel changes
- No new tables
- No belief_edge usage
- No Dockerfile for the app (only Docker for CRDB)
- No bchat
- No web UI
- No network calls during demo
- Canonical inputs remain `internal/derive/testdata/etcd_real/`
