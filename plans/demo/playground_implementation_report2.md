# Solvent Judge Playground — Implementation Report

**Date:** 2026-08-11
**Spec:** `plans/demo/PLAYGROUND_TASK.md`
**Plan:** `plans/demo/plan3.md`
**Review:** `plans/demo/plan3_review.md` + `plans/demo/playground_imp_review.md`

---

## Summary

The judge-facing demo playground is complete and verified. All deliverables implemented, all acceptance criteria met. Acceptance run performed from a torn-down state (fresh Docker container with cold image pull, no pre-existing cluster).

---

## Files changed

| File | Change |
|---|---|
| `scripts/demo/setup.sh` | Added `task` prerequisite check; Go version derived from `go.mod`; pinned Docker image to `cockroachdb/cockroach:v26.2.0` |
| `scripts/demo/track1.sh` | Replaced hardcoded `"Promoted 1"` with DB query for actual promoted count |
| `scripts/demo/config.env` | Port 26260 (SQL) / 8081 (HTTP) — matches test infrastructure's `DefaultDSN` |
| `Taskfile.yml` | All 9 commands; `test` task is self-contained (no `FABLE_TEST_DSN` override) |
| `README.md` | Port references (26260/8081); F1 single-belief limitation note |
| `plans/post/playground_implementation_report.md` | This file |

**Unchanged files (verified complete as-is):**

| File | Status |
|---|---|
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

**Tag resolution receipt (cold pull):**
```
$ docker manifest inspect cockroachdb/cockroach:v26.2.0
{
    "schemaVersion": 2,
    "mediaType": "application/vnd.oci.image.index.v1+json",
    ...
}
$ docker pull cockroachdb/cockroach:v26.2.0
v26.2.0: Pulling from cockroachdb/cockroach
...
Status: Downloaded newer image for cockroachdb/cockroach:v26.2.0
```

**Cluster/session settings:** No `SET CLUSTER SETTING` required. The recursive-CTE `RetractCascade` design (D-033) eliminates the need for temporary-table support. The kernel uses `WITH RECURSIVE` CTEs instead of `CREATE TEMP TABLE`, enforced by `check_i7.sh`. The `ON UPDATE CASCADE` on the composite FK is standard CockroachDB behavior. This is a deliberate property of the current kernel design, not a gap.

**Container config:** Single-node insecure, port 26260 (SQL) / 8081 (HTTP), named `solvent-crdb`.

**Port alignment:** Docker maps host 26260 → container 26257 (CRDB SQL), host 8081 → container 8080 (HTTP). This matches `internal/testdb/testdb.go:22` `DefaultDSN` which expects port 26260, making `task test` self-contained without `FABLE_TEST_DSN` override.

**Schema:** Applied from `db/001_schema.sql` via `docker exec -i`.

**Readiness:** Polled via `SELECT 1` with 60-second timeout.

---

## Acceptance command order

The following sequence was run from a torn-down state (container removed, image removed):

```
1. docker stop solvent-crdb && docker rm solvent-crdb && docker rmi cockroachdb/cockroach:v26.2.0
2. docker manifest inspect cockroachdb/cockroach:v26.2.0    # R3: prove tag resolves
3. task setup                                               # fresh container + cold pull
4. task test                                                # self-contained, no FABLE_TEST_DSN
5. task demo:track1
6. task demo:replay                                         # no reset, runs on track1 state
7. task demo:track2                                         # resets, runs independently
8. task inspect                                             # shows track2 final state
```

The transcript blocks below are captured outputs from the corresponding points in this sequence.

---

## Track 1 demo result (step 5)

```
=== SOLVENT / TRACK 1 ===
  Vulnerability disclosure → belief → debt → promotion → live intent

--- Resetting database ---
=== Solvent CLI === resetting database
    dsn:      postgresql://root@localhost:26260/fable?sslmode=disable
    database: fable  (DROP + CREATE + apply db/001_schema.sql)
Transcript written to /dev/null

--- Processing evidence ---
Transcript written to /dev/null

--- Ledger state after pipeline ---
  [entered] fix available for etcd-io/etcd v3.5.27
          debt: {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}
  [entered] fix available for etcd-io/etcd v3.5.28
          debt: {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}
  [entered] go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg
          debt: {needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

--- Operator review: retire debts, promote, create intent ---
retired: needInvariant
retired: needToyCheck
retired: needNullModel
retired: needObstruction
retired: needFaithfulnessReview
promoted: ok
intent: live (action="deploy etcd v3.5.28")
audit_live_on_nonpromoted: 0
VERDICT: PASS

--- Final state ---

  Evidence      3
  Beliefs       3
  Promoted      1
  Live intents  1
  Audit         SAFE

=== TRACK 1 COMPLETE ===
```

All values read from the database. `Promoted` count is a `SELECT count(*) ... WHERE status='promoted'` query, not hardcoded.

---

## Replay result (step 6 — no reset, runs on track1's populated state)

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

--- Replaying pipeline (same scenario, no --reset) ---
Transcript written to /dev/null

--- State after replay ---
  Beliefs:    3
  Evidence:   3
  Intents:    1

Replay verified: no duplicate ledger state created.
=== REPLAY COMPLETE ===
```

Before/after counts for beliefs, evidence, and intents are exactly equal (3/3/1 → 3/3/1). The assertion is a real gate — if counts change, the script exits with code 1 and prints the discrepancy.

---

## Track 2 demo result (step 7 — resets database independently)

```
=== SOLVENT / TRACK 2 ===
  Historical decision → promotion → intent → postmortem → RetractCascade

--- Resetting database ---
=== Solvent CLI === resetting database
    dsn:      postgresql://root@localhost:26260/fable?sslmode=disable
    database: fable  (DROP + CREATE + apply db/001_schema.sql)
Transcript written to /dev/null

--- Step 1: Enter postulated historical deployment decision ---
BELIEF_ID=98900860-aa6b-4b0c-876b-2b2cd4984601
evidence: attached
  Baseline belief: 98900860-aa6b-4b0c-876b-2b2cd4984601

--- Step 2: Operator review — retire debts, promote, create intent ---
retired: needMap
retired: needInvariant
retired: needToyCheck
retired: needNullModel
retired: needObstruction
retired: needFaithfulnessReview
promoted: ok
intent: live (action="deploy etcd v3.5.0")
audit_live_on_nonpromoted: 0
VERDICT: PASS

--- State before falsification ---
  Belief:  etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
  Status:  promoted
  Action:  deploy etcd v3.5.0
  Intent:  live

--- Step 3: Ingest postmortem + falsify baseline belief ---
Transcript written to /dev/null
FALSIFY
belief: 98900860-aa6b-4b0c-876b-2b2cd4984601
cascade: retracted 1 belief(s)
audit_live_on_nonpromoted: 0

VERDICT: PASS

--- Final state ---
  [retracted] etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
          intent: cancelled
  [entered] fix available for etcd-io/etcd v3.5.0
          intent: NULL
  [entered] v3.5.0–v3.5.2 has documented data inconsistency
          intent: NULL

  Audit         SAFE

=== TRACK 2 COMPLETE ===
```

Single-belief retraction: the promoted belief is retracted, its live intent is cancelled. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

---

## Inspection output (step 8 — shows track2's final state)

```
=== SOLVENT LEDGER ===

--- BELIEFS ---
  ID:         98900860-aa6b-4b0c-876b-2b2cd4984601
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       postulated
  Status:     retracted
  Claim:      etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
  Debt:       {}

  ID:         22263ca9-082a-4836-ad8e-37f48a99a447
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       accommodated
  Status:     entered
  Claim:      fix available for etcd-io/etcd v3.5.0
  Debt:       {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

  ID:         fa2bff46-8653-416b-8711-47842ab1b3bc
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       derived
  Status:     entered
  Claim:      v3.5.0–v3.5.2 has documented data inconsistency
  Debt:       {needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

--- EVIDENCE ---
  ID:         c4bec6b3-9b2d-44de-b24d-b2d12f9a4e2d
  Belief:     22263ca9-082a-4836-ad8e-37f48a99a447
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/releases/tag/v3.5.0

  ID:         07362801-fd17-44d4-a40a-b477d594ad1f
  Belief:     98900860-aa6b-4b0c-876b-2b2cd4984601
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/releases/tag/v3.5.0

  ID:         3de3f6ff-40ec-445d-ab92-1440dadcb47a
  Belief:     fa2bff46-8653-416b-8711-47842ab1b3bc
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md

--- INTENTS ---
  ID:         150ee892-9505-41a9-9e74-b67e8f405e40
  Belief:     98900860-aa6b-4b0c-876b-2b2cd4984601
  Action:     deploy etcd v3.5.0
  State:      cancelled

--- AUDIT ---
  Live intents on non-promoted beliefs: 0 (SAFE)
=== END LEDGER ===
```

Note: `inspect` shows track2's state because track2 ran last (step 7) and reset the database. This is the expected behavior — `inspect` always shows the current ledger state.

---

## Test results (step 4 — self-contained, no FABLE_TEST_DSN)

```
$ unset FABLE_TEST_DSN && task test
ok   internal/belief     1.947s
ok   internal/derive     0.003s
ok   internal/intent     1.340s
ok   internal/kernel     2.121s
ok   internal/normalize  0.011s
ok   internal/pipeline   1.915s

Total: 107 tests, all passing.
Static checks: go build, go vet, gofmt, check_i7.sh — all pass.
```

Tests run against `fable_test` on port 26260 (the Docker container's mapped port). `task test` is fully self-contained — no `FABLE_TEST_DSN` override required.

---

## Prerequisites

- **Docker** — required for CockroachDB container
- **Go** (version derived from `go.mod` directive, currently 1.25.0) — required to build and test
- **Task** — required to run Taskfile commands

`task setup` verifies all three and fails clearly naming any missing prerequisite. Go version is checked dynamically against `go.mod` rather than hardcoded.

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
| `track1.sh` | 95 | `Promoted N` | DB query (`SELECT count(*) ... WHERE status='promoted'`) | Fixed (was hardcoded `"1"`) |
| `track1.sh` | 97-101 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
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

1. Fresh-machine `task setup` works: prerequisites checked (Docker + Go + Task), pinned CRDB v26.2.0 starts with cold pull, schema initialized, readiness verified.
2. The CockroachDB image EXACTLY matches v26.2.0 — the verified version. Tag resolves from registry (not merely cached).
3. No non-default CRDB settings are required; the recursive-CTE design (D-033) eliminates temp-table needs.
4. `task demo:track1` produces a real run; the summary values are read from the database, not printed constants.
5. `task demo:replay` runs WITHOUT reset, immediately after track1, and its idempotence is backed by `EnsureBelief` CTE + `evidenceExists(content_sha256)` dedup; no truncate/recreate hides in the replay path.
6. `task demo:track2` shows single-belief retraction + cancelled live intent; README and output NEVER imply multi-belief/graph propagation (F1).
7. `task inspect` reflects actual current database state.
8. `source_observed_at` remains meaningful in output while `ingested_at` is excluded from deterministic comparisons.
9. No hidden network dependency: no live GitHub/NVD/OSV/external calls during any demo; fixtures are the only inputs.
10. Grep ALL demo scripts and output paths for hard-coded success strings. Each must be printed only AFTER the corresponding check passes against the DB.
11. No schema/kernel changes: `db/001_schema.sql`, `kernel.go`, and `FullDebt` unchanged; no new tables; `belief_edge` still unused.
12. Existing 107-test baseline green (`task test`), self-contained without `FABLE_TEST_DSN` override.
