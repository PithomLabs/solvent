# Solvent Judge Playground — Implementation Plan (plan2)

**Source:** `plans/demo/PLAYGROUND_TASK.md`
**Date:** 2026-08-11
**Scope:** Taskfile + Docker CockroachDB + demo scripts + README. No schema/kernel/architecture changes.

---

## Current state assessment

The playground is **already substantially implemented**. A prior pass created all deliverable files. This plan identifies what exists, what needs fixing, and what is missing.

### Already implemented (verified by file inspection)

| File | Status | Notes |
|---|---|---|
| `Taskfile.yml` | Exists | All 9 commands present |
| `scripts/demo/config.env` | Exists | Centralized DSN, ports, scenario UUIDs |
| `scripts/demo/setup.sh` | Exists | Docker CRDB setup, schema init, Go build |
| `scripts/demo/track1.sh` | Exists | Full Track 1 flow |
| `scripts/demo/replay.sh` | Exists | Replay without reset, count comparison |
| `scripts/demo/track2.sh` | Exists | Full Track 2 flow with falsification |
| `scripts/demo/inspect.sh` | Exists | Beliefs, evidence, intents, audit |
| `README.md` | Exists | Judge-facing Quick Start |

### Gaps identified against PLAYGROUND_TASK.md

| # | Gap | Severity | Section |
|---|---|---|---|
| G1 | `task setup` does not check for `task` binary | Medium | §3/F4 |
| G2 | README says port 26260; actual Docker config uses 26261 | Low | §10 |
| G3 | README says "single-node insecure on port 26260" but config.env uses 26261 | Low | §10 |
| G4 | No `plans/post/playground_implementation_report.md` for THIS pass | High | §16 |
| G5 | F1 limitation not stated in README or report | High | F1 |
| G6 | CRDB image v26.2.1 vs local binary v26.2.0 — version pinning discrepancy | High | F2 |
| G7 | `task setup` does not apply non-default cluster/session settings (step 5 of §3) | Low | F2 |
| G8 | track1.sh has hardcoded `"Promoted 1"` line instead of querying DB | Low | §11 |

---

## Critical analysis (F1–F6)

### F1 — RetractCascade honesty

**Verified by code inspection.** `RetractCascade` in `internal/kernel/sql.go:66-92` uses a `WITH RECURSIVE d(id)` CTE that walks `belief_edge`. The `belief_edge` table is **never populated** by any demo script or pipeline code. Therefore:

- The recursive CTE finds only the root belief itself (no descendants).
- `sqlRetractCascadeRetract` retracts exactly 1 belief.
- `sqlRetractCascadeCancel` cancels only that belief's own live intent.
- This is **single-node retraction**, not graph propagation.

**Required actions:**
1. README must state: "The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. belief_edge-based multi-belief propagation is intentionally outside the current demo."
2. The `playground_implementation_report.md` must carry the same limitation verbatim plus the strategic aside: a single real dependency edge is what would make RetractCascade propagate; it is dormant by design, not a defect.

### F2 — Reproducible CockroachDB environment

**CRDB version:** The setup.sh pins `cockroachdb/cockroach:v26.2.1`. The local `cockroach` binary reports `v26.2.0`. The tests were verified against the local binary. The Docker image v26.2.1 is one patch ahead. For the playground, v26.2.1 is acceptable — it is backward-compatible with v26.2.0 schema and SQL. However, the report should note the discrepancy.

**Cluster/session settings:** `check_i7.sh` explicitly enforces D-033: no `CREATE TEMP TABLE` in the kernel. The kernel uses `WITH RECURSIVE` CTEs instead. No `SET CLUSTER SETTING` is required. The `ON UPDATE CASCADE` on the composite FK is standard CockroachDB behavior. **No non-default settings are needed.** The report should state this explicitly.

**Task prerequisite:** `setup.sh` checks Docker and Go but does not check for the `task` binary. Per F4, all three (Docker + Go + Task) must be verified. Fix: add a `task` check.

### F3 — Replay must be real idempotence

**Verified by code inspection.** The dedup mechanism has two layers:

1. **`EnsureBelief`** (`internal/kernel/sql.go:39-53`): Uses a CTE `WITH existing AS (SELECT id FROM belief WHERE scenario_id=$1 AND claim=$2) UNION ALL INSERT ... WHERE NOT EXISTS (SELECT 1 FROM existing)`. This is find-or-create: re-running with the same claim returns the existing belief ID, no duplicate created.

2. **`evidenceExists`** (`internal/belief/belief.go:101-111`): Before adding evidence, checks `SELECT count(*) FROM evidence WHERE belief_id=$1 AND content_sha256=$2`. If the evidence already exists for that belief, it is skipped.

**Result:** Replay without reset creates zero duplicate beliefs and zero duplicate evidence. The replay.sh script correctly runs without `--reset` and compares before/after counts. **This is genuine idempotence, not disguised reset.**

### F4 — Prerequisites

`setup.sh` checks Docker and Go. Missing: `task` binary check. Fix: add check.

### F5 — Test environment

The 107 tests use `fable_test` database (see `internal/testdb/testdb.go:22`), not `fable`. Tests do not require the demo Docker container. `scripts/check_i7.sh` exists and is functional. `task test` runs `go test`, `go build`, `go vet`, `gofmt`, and `check_i7.sh`.

### F6 — Deterministic time semantics

The `evidence` table has both `source_observed_at` (meaningful fixed timestamp from real fixtures) and `ingested_at` (runtime `DEFAULT now()`). The demo scripts query `source_url` and `provenance_class` from evidence, not `ingested_at`. This is correct: meaningful source timestamps are preserved, runtime timestamps are excluded from deterministic comparisons.

---

## Implementation steps

### Step 1: Fix `setup.sh` — add Task prerequisite check (G1)

Add after the Go check:

```bash
if ! command -v task &>/dev/null; then
  echo "ERROR: task is not installed or not in PATH." >&2
  echo "Install Task: https://taskfile.dev/installation/" >&2
  exit 1
fi
```

Update the "Prerequisites: OK" line to include `task`.

### Step 2: Fix `track1.sh` — query Promoted count from DB (G8)

Replace hardcoded `echo "  Promoted      1"` with a DB query:

```bash
PROMOTED_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}' AND status = 'promoted';
" 2>/dev/null | tail -n1)
echo "  Promoted      ${PROMOTED_COUNT}"
```

### Step 3: Fix README.md port references (G2, G3)

Change line 118 from:
> "The container runs single-node insecure on port 26260."

To:
> "The container runs single-node insecure on port 26261 (SQL) / 8082 (HTTP)."

### Step 4: Add F1 limitation to README.md (G5)

After the Track 2 section (line 37), add a note:

> **Note:** The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

### Step 5: Create `plans/post/playground_implementation_report.md` (G4)

New report covering:
1. Files changed (list all playground files)
2. Commands added (9 Taskfile commands)
3. CockroachDB setup mechanism (Docker v26.2.1, no non-default settings needed)
4. Track 1 demo result
5. Replay result (dedup mechanism: `EnsureBelief` CTE + `evidenceExists` check)
6. Track 2 demo result
7. Inspection output
8. Test results
9. Prerequisites (Docker + Go + Task)
10. Schema/kernel unchanged confirmation
11. Limitations (F1 verbatim, plus strategic aside about belief_edge)

---

## Execution order

1. Edit `scripts/demo/setup.sh` — add Task prerequisite check
2. Edit `scripts/demo/track1.sh` — query Promoted count from DB
3. Edit `README.md` — fix port references, add F1 limitation
4. Create `plans/post/playground_implementation_report.md`
5. Run `task test` to verify 107 tests still pass
6. Run `task setup` to verify Docker CRDB setup works
7. Run `task demo:track1` → `task demo:replay` → `task demo:track2` → `task inspect` to verify full sequence

---

## Scope boundaries (unchanged)

- No schema changes (`db/001_schema.sql` frozen)
- No kernel changes (`internal/kernel/` frozen)
- No new tables
- No belief_edge relationships added
- No bchat
- No web UI
- No network calls during demo
- Canonical inputs remain `internal/derive/testdata/etcd_real/`
