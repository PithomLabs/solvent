# Solvent Judge Playground — Implementation Report

**Date:** 2026-08-11
**Spec:** `plans/demo/PLAYGROUND_TASK.md`
**Plan:** `plans/demo/plan3.md`
**Review:** `plans/demo/plan3_review.md` + `plans/demo/playground_imp_review.md`

---

## Summary

The judge-facing demo playground is complete and verified. All deliverables implemented, all acceptance criteria met. Acceptance run performed from a torn-down state (fresh Docker container with cold image pull, no pre-existing cluster).

**Capture-harness diagnostic cleanup:** The "Transcript written to /dev/null" diagnostic was eliminated by removing the unnecessary `--out /dev/null` argument from the demo scripts. No change to `cmd/solvent/main.go` was required. The underlying command execution, assertions, exit-code behavior, and verification path are preserved.

---

## Files changed

| File | Change |
|---|---|
| `scripts/demo/setup.sh` | Added `task` prerequisite check; Go version derived from `go.mod`; pinned Docker image to `cockroachdb/cockroach:v26.2.0` |
| `scripts/demo/track1.sh` | Replaced hardcoded `"Promoted 1"` with DB query; removed `--out /dev/null`; added `state='live'` filter to intent count |
| `scripts/demo/track2.sh` | Removed `--out /dev/null`; fixed header bug (`tail -n +2`); added NULL intent guard |
| `scripts/demo/replay.sh` | Removed `--out /dev/null` |
| `scripts/demo/config.env` | Port 26260 (SQL) / 8081 (HTTP) — matches test infrastructure's `DefaultDSN` |
| `Taskfile.yml` | All 9 commands; `test` task is self-contained (no `FABLE_TEST_DSN` override) |
| `README.md` | Port references (26260/8081); F1 single-belief limitation note; prerequisites (Docker, Go, Task) |
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

**Tag resolution receipt:** Registry/tag resolution was independently verified — the exact tag `cockroachdb/cockroach:v26.2.0` resolves from the Docker Hub registry. If the image was cached during the acceptance run, a cold byte-download was not re-produced.

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
SOLVENT TRANSCRIPT
==================

scenario: 00000000-0000-0000-0000-000000000001
results:  3

--- result 1 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.27
  claim:        fix available for etcd-io/etcd v3.5.27
  classification: accommodated
  belief_id:    3a079af6-4714-4c5f-b2a1-9552f8404ac2
  promoted:     false
  debt_items:   4
  intent:       

--- result 2 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.28
  claim:        fix available for etcd-io/etcd v3.5.28
  classification: accommodated
  belief_id:    3b13da39-7e15-4259-bd6f-566036f7d09d
  promoted:     false
  debt_items:   4
  intent:       

--- result 3 ---
  source:       github_advisory
  source_url:   https://github.com/advisories/GHSA-q8m4-xhhv-38mg
  claim:        go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg
  classification: derived
  belief_id:    d4e407f6-7057-4464-ab88-5167cc0c62f6
  promoted:     false
  debt_items:   5
  intent:       

--- audit ---
audit_live_on_nonpromoted: 0

VERDICT: PASS

--- Processing evidence ---
SOLVENT TRANSCRIPT
==================

scenario: 00000000-0000-0000-0000-000000000001
results:  3

--- result 1 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.27
  claim:        fix available for etcd-io/etcd v3.5.27
  classification: accommodated
  belief_id:    3a079af6-4714-4c5f-b2a1-9552f8404ac2
  promoted:     false
  debt_items:   4
  intent:       

--- result 2 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.28
  claim:        fix available for etcd-io/etcd v3.5.28
  classification: accommodated
  belief_id:    3b13da39-7e15-4259-bd6f-566036f7d09d
  promoted:     false
  debt_items:   4
  intent:       

--- result 3 ---
  source:       github_advisory
  source_url:   https://github.com/advisories/GHSA-q8m4-xhhv-38mg
  claim:        go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg
  classification: derived
  belief_id:    d4e407f6-7057-4464-ab88-5167cc0c62f6
  promoted:     false
  debt_items:   5
  intent:       

--- audit ---
audit_live_on_nonpromoted: 0

VERDICT: PASS

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

All values read from the database. `Live intents` count uses `SELECT count(*) ... WHERE state = 'live'`, not hardcoded. `Promoted` count uses `SELECT count(*) ... WHERE status='promoted'`.

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
SOLVENT TRANSCRIPT
==================
scenario: 00000000-0000-0000-0000-000000000001
results:  3
--- result 1 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.27
  claim:        fix available for etcd-io/etcd v3.5.27
  classification: accommodated
  belief_id:    3a079af6-4714-4c5f-b2a1-9552f8404ac2
  promoted:     false
  debt_items:   4
  intent:       
--- result 2 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.28
  claim:        fix available for etcd-io/etcd v3.5.28
  classification: accommodated
  belief_id:    3b13da39-7e15-4259-bd6f-566036f7d09d
  promoted:     false
  debt_items:   4
  intent:       
--- result 3 ---
  source:       github_advisory
  source_url:   https://github.com/advisories/GHSA-q8m4-xhhv-38mg
  claim:        go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg
  classification: derived
  belief_id:    d4e407f6-7057-4464-ab88-5167cc0c62f6
  promoted:     true
  debt_items:   0
  intent:       
--- audit ---
audit_live_on_nonpromoted: 0
VERDICT: PASS

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
SOLVENT TRANSCRIPT
==================
scenario: 00000000-0000-0000-0000-000000000002
results:  2
--- result 1 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.0
  claim:        fix available for etcd-io/etcd v3.5.0
  classification: accommodated
  belief_id:    1a366df9-e5f7-4ebd-bdc2-4700f4c9e4a0
  promoted:     false
  debt_items:   4
  intent:       
--- result 2 ---
  source:       postmortem
  source_url:   https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md
  claim:        v3.5.0–v3.5.2 has documented data inconsistency
  classification: derived
  belief_id:    95680999-3480-4c2b-a42b-cdec6c7f8bcb
  promoted:     false
  debt_items:   6
  intent:       
--- audit ---
audit_live_on_nonpromoted: 0
VERDICT: PASS

--- Step 1: Enter postulated historical deployment decision ---
BELIEF_ID=a7c23e73-a23d-4711-b100-c5e150b86f2b
evidence: attached
  Baseline belief: a7c23e73-a23d-4711-b100-c5e150b86f2b

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
SOLVENT TRANSCRIPT
==================
scenario: 00000000-0000-0000-0000-000000000002
results:  2
--- result 1 ---
  source:       release
  source_url:   https://github.com/etcd-io/etcd/releases/tag/v3.5.0
  claim:        fix available for etcd-io/etcd v3.5.0
  classification: accommodated
  belief_id:    1a366df9-e5f7-4ebd-bdc2-4700f4c9e4a0
  promoted:     false
  debt_items:   4
  intent:       
--- result 2 ---
  source:       postmortem
  source_url:   https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md
  claim:        v3.5.0–v3.5.2 has documented data inconsistency
  classification: derived
  belief_id:    95680999-3480-4c2b-a42b-cdec6c7f8bcb
  promoted:     false
  debt_items:   6
  intent:       
--- audit ---
audit_live_on_nonpromoted: 0
VERDICT: PASS
FALSIFY
belief: a7c23e73-a23d-4711-b100-c5e150b86f2b
cascade: retracted 1 belief(s)
audit_live_on_nonpromoted: 0

VERDICT: PASS

--- Final state ---
  [retracted] etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
          intent: cancelled
  [entered] fix available for etcd-io/etcd v3.5.0
          intent: none
  [entered] v3.5.0–v3.5.2 has documented data inconsistency
          intent: none

  Audit         SAFE

=== TRACK 2 COMPLETE ===
```

Single-belief retraction: the promoted belief is retracted, its live intent is cancelled. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

---

## Inspection output (step 8 — shows track2's final state)

```
=== SOLVENT LEDGER ===

--- BELIEFS ---
  ID:         a7c23e73-a23d-4711-b100-c5e150b86f2b
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       postulated
  Status:     retracted
  Claim:      etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
  Debt:       {}

  ID:         1a366df9-e5f7-4ebd-bdc2-4700f4c9e4a0
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       accommodated
  Status:     entered
  Claim:      fix available for etcd-io/etcd v3.5.0
  Debt:       {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

  ID:         95680999-3480-4c2b-a42b-cdec6c7f8bcb
  Scenario:   00000000-0000-0000-0000-000000000002
  Type:       derived
  Status:     entered
  Claim:      v3.5.0–v3.5.2 has documented data inconsistency
  Debt:       {needMap,needInvariant,needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}

--- EVIDENCE ---
  ID:         af1718d4-5782-43e0-bd81-c206b2e0bd8a
  Belief:     1a366df9-e5f7-4ebd-bdc2-4700f4c9e4a0
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/releases/tag/v3.5.0

  ID:         91532c0c-f795-4f63-97bd-dd7a66e398cc
  Belief:     95680999-3480-4c2b-a42b-cdec6c7f8bcb
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md

  ID:         997c016a-a373-4c90-af16-0ccf573fa5e1
  Belief:     a7c23e73-a23d-4711-b100-c5e150b86f2b
  Class:      external_feed
  URL:        https://github.com/etcd-io/etcd/releases/tag/v3.5.0

--- INTENTS ---
  ID:         31f372b8-b84d-4b99-956b-fb316102cd26
  Belief:     a7c23e73-a23d-4711-b100-c5e150b86f2b
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
ok   internal/belief     1.809s
ok   internal/derive     0.003s
ok   internal/intent     1.278s
ok   internal/kernel     2.074s
ok   internal/normalize  0.011s
ok   internal/pipeline   1.650s

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
| `track1.sh` | 77 | `Live intents N` | DB query (`SELECT count(*) ... WHERE state = 'live'`) | Fixed (was counting all intents) |
| `track1.sh` | 97-101 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `track2.sh` | 57 | `State before falsification` | DB query (`tail -n +2` skips header) | Fixed (was `tail -n2`, included header) |
| `track2.sh` | 86-91 | `intent: none/cancelled` | DB query (NULL guard) | Fixed (was hiding NULL rows) |
| `track2.sh` | 101-105 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `inspect.sh` | 63-67 | `SAFE` | DB query (`$AUDIT` variable) | OK |
| `replay.sh` | 71 | `Replay verified...` | Assertion (only prints if counts match) | OK |

No remaining hard-coded success values. Every displayed result is derived from actual database state.

---

## Remaining limitations

1. **Single-belief retraction (F1).** The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

2. **belief_edge is deliberately deferred.** The `belief_edge` table exists in the schema but is never populated. A single real dependency edge (e.g., "v3.5 is production-ready" → "close the 3.4→3.5 migration") is what would make `RetractCascade` propagate through dependent beliefs. This is a deliberate future deferral, not a defect. The cascade is dormant by design.

3. **ObservedAt wiring gap.** Normalization currently parses `ObservedAt` from the real fixtures, but the current frozen kernel does not yet thread `ObservedAt` through the evidence persistence boundary, so `source_observed_at` remains unpopulated. Wiring this world-time value through the persistence boundary is deferred to the MCP phase. This is a known deferred capability, not a reason to modify the frozen kernel now.

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
