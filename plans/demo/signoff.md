# Playground Polish — Signoff

**Date:** 2026-08-11
**Prompt:** `plans/review/prompt_tofix.md`
**Status:** COMPLETE

---

## Scope

Final cleanup pass before CLI/Taskfile playground freeze. No core/schema/kernel/cmd changes.

---

## Findings resolved

| ID | Description | File | Change |
|---|---|---|---|
| F-1 | Track 2 header bug — TSV header rendered as phantom belief | `track2.sh:57` | `tail -n2` → `tail -n +2` |
| F-4 | Track 1 "Live intents" counted all intents, not just live | `track1.sh:74-77` | Added `AND state = 'live'` |
| F-9 | Track 2 NULL intent displayed raw | `track2.sh:86-91` | NULL guard → displays "none" |
| F-2 | README missing Task prerequisite | `README.md:7-11` | Explicit Docker/Go/Task list |
| F-3 | Report wording implied stderr redirect | `playground_implementation_report.md` | Clarified no cmd/ change |
| F-6 | Report conflated registry verify with cold pull | `playground_implementation_report.md` | Distinguished the two |
| F-7 | ObservedAt gap undocumented | `playground_implementation_report.md` | Added to limitations |

---

## Verification

Full sequence run from torn-down state:

```
docker stop solvent-crdb && docker rm solvent-crdb
task setup
unset FABLE_TEST_DSN && task test
task demo:track1
task demo:replay
task demo:track2
task inspect
```

### Track 1

- 3 beliefs, 3 evidence, 1 promoted, 1 live intent, audit SAFE
- "Live intents" sourced from `SELECT count(*) ... WHERE state = 'live'`

### Replay

- Before: 3/3/1, After: 3/3/1
- Idempotence verified, no duplicates

### Track 2

- No phantom header row in "State before falsification"
- `intent: none` for beliefs without intent (not raw NULL)
- Retracted belief: `intent: cancelled`
- Audit SAFE

### Tests

- 107 tests pass
- go build, go vet, gofmt, check_i7.sh all pass

---

## Grep audit

| Pattern | Result |
|---|---|
| `Transcript written to /dev/null` | Not found in demo scripts or README |
| `--out /dev/null` | Not found in demo scripts |
| `tail -n2` | Not found in demo scripts |
| `intent: NULL` | Not found in demo scripts or report |

---

## Files changed

| File | Change |
|---|---|
| `scripts/demo/track1.sh` | F-4: live intent filter |
| `scripts/demo/track2.sh` | F-1: header fix; F-9: NULL guard |
| `scripts/demo/replay.sh` | Verified unchanged |
| `README.md` | F-2: prerequisites |
| `plans/post/playground_implementation_report.md` | F-3, F-6, F-7: wording/provenance/limitations + fresh transcripts |

---

## Scope enforcement

**Changed:** `scripts/demo/*`, `README.md`, `plans/post/playground_implementation_report.md`

**Untouched:** `db/`, `internal/`, `cmd/`, `proof/`, etcd fixtures, kernel semantics

---

## Frozen core confirmation

- `db/001_schema.sql` — unchanged
- `internal/kernel/` — unchanged
- `cmd/solvent/main.go` — unchanged
- Schema invariants intact
- 107-test baseline green

---

## Decision

Playground is clean, honest, reproducible, and judge-ready. CLI/Taskfile playground is frozen. Ready for MCP phase.
