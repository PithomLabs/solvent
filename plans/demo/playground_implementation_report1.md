# Solvent Judge Playground — Implementation Report

**Date:** 2026-08-11
**Spec:** `plans/demo/PLAYGROUND_TASK.md`
**Plan:** `plans/demo/plan3.md`
**Review:** `plans/demo/plan3_review.md`

---

## Summary

The judge-facing demo playground is complete and verified. All deliverables implemented, all acceptance criteria met. Acceptance run performed from a torn-down state (fresh Docker container, no pre-existing cluster).

---

## Files changed

| File | Change |
|---|---|
| `scripts/demo/setup.sh` | Added `task` prerequisite check; pinned Docker image to `cockroachdb/cockroach:v26.2.0` |
| `scripts/demo/track1.sh` | Replaced hardcoded `"Promoted 1"` with DB query for actual promoted count |
| `Taskfile.yml` | Added `FABLE_TEST_DSN` env var to `test` task (points tests at Docker container port 26261) |
| `README.md` | Fixed port references (26260 → 26261/8082); added F1 single-belief limitation note |
| `plans/post/playground_implementation_report.md` | This file |

**Unchanged files (verified complete as-is):**

| File | Status |
|---|---|
| `Taskfile.yml` (commands) | All 9 commands present and working |
| `scripts/demo/config.env` | Centralized DSN, ports, scenario UUIDs |
| `scripts/demo/replay.sh` | Genuine idempotence (count comparison, no reset) |
| `scripts/demo/track2.sh` | Single-belief retraction, all values DB-derived |
| `scripts/demo/inspect.sh` | All values DB-derived |
| `db/001_schema.sql` | Frozen, unchanged |
| `internal/kernel/kernel.go` | Frozen, unchanged |
| `internal/kernel/sql.go` | Frozen, unchanged |

---

## Commands added

| Command | Description |
|---|---|
| `task setup` | Docker CRDB (v26.2.0) + schema init + Go build |
| `task db:up` | Start the CockroachDB container |
| `task db:down` | Stop the CockroachDB container |
| `task db:reset` | Drop + recreate fable database, reapply schema |
| `task demo:track1` | Vulnerability disclosure → belief → promotion → intent |
| `task demo:replay` | Prove idempotent replay creates no duplicates |
| `task demo:track2` | Historical decision → intent → postmortem → retraction |
| `task inspect` | Human-readable ledger snapshot |
| `task test` | Full verification suite (107 tests + static checks) |

---

## CockroachDB setup mechanism

**Docker image:** `cockroachdb/cockroach:v26.2.0` (exact verified version, matching local binary v26.2.0 used for 107-test/E2E verification).

**Cluster/session settings:** No `SET CLUSTER SETTING` required. The recursive-CTE `RetractCascade` design (D-033) eliminates the need for temporary-table support. The kernel uses `WITH RECURSIVE` CTEs instead of `CREATE TEMP TABLE`, enforced by `check_i7.sh`. The `ON UPDATE CASCADE` on the composite FK is standard CockroachDB behavior.

**Container config:** Single-node insecure, port 26261 (SQL) / 8082 (HTTP), named `solvent-crdb`.

**Schema:** Applied from `db/001_schema.sql` via `docker exec -i`.

**Readiness:** Polled via `SELECT 1` with 60-second timeout.

---

## Track 1 demo result

```
=== SOLVENT / TRACK 1 ===
  Vulnerability disclosure → belief → debt → promotion → live intent

  Evidence      3
  Beliefs       3
  Promoted      1
  Live intents  1
  Audit         SAFE

=== TRACK 1 COMPLETE ===
```

All values read from the database. `Promoted` count is a `SELECT count(*)` query, not hardcoded.

---

## Replay result

**Dedup mechanism (verified by code inspection):**

1. **`EnsureBelief`** (`internal/kernel/sql.go:39-53`): CTE find-or-create on `(scenario_id, claim)`. Re-running with the same claim returns existing belief ID, no duplicate created.

2. **`evidenceExists`** (`internal/belief/belief.go:101-111`): Checks `SELECT count(*) FROM evidence WHERE belief_id=$1 AND content_sha256=$2` before inserting. Duplicate evidence skipped.

**Claim key determinism:** `Derive()` is a pure function (`internal/derive/derive.go:22-23`: "Same evidence always produces identical beliefs"). `ContentSHA256` computed from deterministic fields only — no timestamps, no randomness (`internal/normalize/normalize.go`).

**Actual replay result:**

```
=== SOLVENT / REPLAY ===
  Proving idempotent replay on Track 1

--- State before replay ---
  Beliefs:    3
  Evidence:   3
  Intents:    1

--- State after replay ---
  Beliefs:    3
  Evidence:   3
  Intents:    1

Replay verified: no duplicate ledger state created.
=== REPLAY COMPLETE ===
```

Before/after counts for beliefs, evidence, and intents are exactly equal. The assertion is a real gate — if counts change, the script exits with code 1 and prints the discrepancy.

---

## Track 2 demo result

```
=== SOLVENT / TRACK 2 ===
  Historical decision → promotion → intent → postmortem → RetractCascade

  [retracted] etcd v3.5.0 is approved for production deployment
          intent: cancelled
  [entered] fix available for etcd-io/etcd v3.5.0
  [entered] v3.5.0–v3.5.2 has documented data inconsistency

  Audit         SAFE

=== TRACK 2 COMPLETE ===
```

Single-belief retraction: the promoted belief is retracted, its live intent is cancelled. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

---

## Inspection output

```
=== SOLVENT LEDGER ===

--- BELIEFS ---
  [retracted] etcd v3.5.0 is approved for production deployment
  [entered]   fix available for etcd-io/etcd v3.5.0
  [entered]   v3.5.0–v3.5.2 has documented data inconsistency

--- EVIDENCE ---
  3 evidence rows, all external_feed

--- INTENTS ---
  [cancelled] deploy etcd v3.5.0

--- AUDIT ---
  Live intents on non-promoted beliefs: 0 (SAFE)
=== END LEDGER ===
```

---

## Test results

```
ok   internal/belief     2.161s
ok   internal/derive     0.003s
ok   internal/intent     1.354s
ok   internal/kernel     2.133s
ok   internal/normalize  0.011s
ok   internal/pipeline   1.809s

Total: 107 tests, all passing.
Static checks: go build, go vet, gofmt, check_i7.sh — all pass.
```

---

## Prerequisites

- **Docker** — required for CockroachDB container
- **Go** (compatible with `go.mod`: go 1.25.0) — required to build and test
- **Task** — required to run Taskfile commands

`task setup` verifies all three and fails clearly naming any missing prerequisite.

---

## Schema/kernel unchanged confirmation

- `db/001_schema.sql` — **unchanged** (frozen)
- `internal/kernel/kernel.go` — **unchanged** (frozen)
- `internal/kernel/sql.go` — **unchanged** (frozen)
- `FullDebt` — **unchanged**
- `belief_edge` — **still unused** (no edges added)
- No new tables introduced

---

## Hard-coded output audit (C3)

All demo output values were grep'd for hard-coded constants. Results:

| Script | Line | Value | Source | Status |
|---|---|---|---|---|
| `track1.sh` | 89 | `Promoted N` | DB query (`SELECT count(*) ... WHERE status='promoted'`) | Fixed (was hardcoded `"1"`) |
| `track1.sh` | 91-95 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `track2.sh` | 101-105 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `inspect.sh` | 63-67 | `SAFE` | DB query (`$AUDIT` variable) | OK |
| `replay.sh` | 71 | `Replay verified...` | Assertion (only prints if counts match) | OK |

No remaining hard-coded success values. Every displayed result is derived from actual database state.

---

## Remaining limitations

1. **Single-belief retraction (F1).** The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

2. **belief_edge is deliberately deferred.** The `belief_edge` table exists in the schema but is never populated. A single real dependency edge (e.g., "v3.5 is production-ready" → "close the 3.4→3.5 migration") is what would make `RetractCascade` propagate through dependent beliefs. This is a deliberate future deferral, not a defect. The cascade is dormant by design.

---

## Final adversarial review prompt

Review the Solvent judge playground on a FRESH machine with Docker but without CockroachDB installed. Verify, and STOP-and-report any that cannot be established from the repository rather than accepting a claim:

1. Fresh-machine `task setup` works: prerequisites checked (Docker + Go + Task), pinned CRDB v26.2.0 starts, schema initialized, readiness verified.
2. The CockroachDB image EXACTLY matches v26.2.0 — the verified version.
3. No non-default CRDB settings are required; the recursive-CTE design (D-033) eliminates temp-table needs.
4. `task demo:track1` produces a real run; the summary values are read from the database, not printed constants.
5. `task demo:replay` runs WITHOUT reset, immediately after track1, and its idempotence is backed by `EnsureBelief` CTE + `evidenceExists(content_sha256)` dedup; no truncate/recreate hides in the replay path.
6. `task demo:track2` shows single-belief retraction + cancelled live intent; README and output NEVER imply multi-belief/graph propagation (F1).
7. `task inspect` reflects actual current database state.
8. `source_observed_at` remains meaningful in output while `ingested_at` is excluded from deterministic comparisons.
9. No hidden network dependency: no live GitHub/NVD/OSV/external calls during any demo; fixtures are the only inputs.
10. Grep ALL demo scripts and output paths for hard-coded success strings. Each must be printed only AFTER the corresponding check passes against the DB.
11. No schema/kernel changes: `db/001_schema.sql`, `kernel.go`, and `FullDebt` unchanged; no new tables; `belief_edge` still unused.
12. Existing 107-test baseline green (`task test`), with tests pointed at the Docker container via `FABLE_TEST_DSN`.
