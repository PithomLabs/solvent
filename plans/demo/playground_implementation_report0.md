# Solvent Judge Playground — Implementation Report

**Date:** 2026-08-10
**Spec:** `plans/demo/prompt.md`
**Plan:** `plans/demo/plan.md`
**Review:** `plans/demo/plan_review.md`

---

## Summary

The judge-facing demo playground is complete and verified. All deliverables implemented, all acceptance criteria met.

---

## Deliverables

| File | Purpose |
|---|---|
| `scripts/demo/config.env` | Centralized configuration (DSN, port, container name, scenario UUIDs) |
| `scripts/demo/setup.sh` | Docker CRDB setup, schema init, Go build |
| `scripts/demo/track1.sh` | Track 1: vulnerability disclosure → belief → promotion → intent |
| `scripts/demo/replay.sh` | Replay: prove idempotent replay creates no duplicates |
| `scripts/demo/track2.sh` | Track 2: historical decision → intent → postmortem → retraction |
| `scripts/demo/inspect.sh` | Human-readable ledger snapshot |
| `Taskfile.yml` | All commands (setup, db:up/down/reset, demo:*, inspect, test) |
| `README.md` | Updated with judge-facing Quick Start |

---

## Acceptance Criteria Verification

| Criterion | Status |
|---|---|
| `task setup` starts Docker CRDB, initializes schema, builds binaries | **PASS** |
| `task demo:track1` runs full Track 1 flow with correct final state | **PASS** |
| `task demo:replay` proves no duplicate ledger state | **PASS** |
| `task demo:track2` runs full Track 2 flow with correct final state | **PASS** |
| `task inspect` shows human-readable ledger snapshot | **PASS** |
| `task test` runs full test suite (107 tests) | **PASS** |
| All static checks pass (build, vet, fmt, i7) | **PASS** |

---

## Issues Found and Fixed

### track1.sh — removed broken RESULTS parsing

The original `track1.sh` captured pipeline output into a variable and tried to parse `"results:"` from it. The pipeline does not emit a `"results:"` line — it emits `"Transcript written to /dev/null"` when `--out /dev/null`. This caused the script to stall because the `grep` pipeline produced no output and the variable was empty.

**Fix:** Removed the `TRANSCRIPT` capture and `RESULTS` parsing. Pipeline output streams directly to stderr.

### Port conflict with existing local CockroachDB

The existing local CockroachDB (from `scripts/m0_up.sh`) occupies port 26260. The Docker container was configured to use port 26260 externally, causing a conflict.

**Fix:** Configured Docker container to use port 26261 (SQL) and 8082 (HTTP) to avoid conflicts. Config centralized in `config.env`.

---

## Final State (after Track 2)

```
BELIEFS:
  [retracted] etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)
  [entered]   fix available for etcd-io/etcd v3.5.0
  [entered]   v3.5.0–v3.5.2 has documented data inconsistency

INTENTS:
  [cancelled] deploy etcd v3.5.0

AUDIT: SAFE (0 live intents on non-promoted beliefs)
```

---

## Test Results

```
ok   internal/belief     1.780s
ok   internal/derive     0.003s
ok   internal/intent     1.146s
ok   internal/kernel     1.933s
ok   internal/normalize  0.011s
ok   internal/pipeline   1.569s

Total: 107 tests, all passing.
```

Static checks: `go build`, `go vet`, `gofmt`, `check_i7.sh` — all pass.

---

## Frozen Waves Preserved

No changes to Waves 1–4 code. Playground additions are strictly additive:

- New files in `scripts/demo/`
- New file `Taskfile.yml`
- Updated `README.md`

Kernel, schema, pipeline, normalize, derive — all untouched.
