# 04_TASKFILE_UX.md

**Objective:** Specify every command's experience — what it prints, how long it
takes, what "it worked" looks like — without specifying implementation.

**Audience:** Phase 2 implementers; anyone running the demo.

---

## Command Surface

### Primary Entry Point

#### `task wow`

**Purpose:** Guided one-shot orchestrator. Zero configuration, zero decisions.

**Expected duration:** 90 seconds (cluster start + schema + demo + race + verify)

**Expected terminal output:**
```
$ task wow

Solvent — Transactional Belief Ledger
======================================

[1/5] Starting CockroachDB cluster...          ✓ (12s)
[2/5] Applying schema...                       ✓ (3s)
[3/5] Running the race...                      ✓ (45s)
[4/5] Verifying invariants...                  ✓ (8s)
[5/5] Checking audit...                        ✓ (2s)

Result: AUDIT = 0

  LANE A (typical schema):  AUDIT = 1  ← corruption committed
  LANE B (CockroachDB gate): AUDIT = 0  ← schema refused

  Nobody wrote code to catch this. The schema already knew.

  Full receipts: docs/M3_TRANSCRIPT.md
```

**Failure mode:** "Cluster won't start" → one-line diagnosis:
```
Error: Cannot reach CockroachDB on localhost:26260
Fix:   Run `task setup` first, or install Docker
```

---

### Granular Commands

#### `task setup`

**Purpose:** Start the cluster and apply the schema. Idempotent.

**Expected duration:** 15 seconds (cluster already running) to 30 seconds (cold start)

**Expected terminal output:**
```
$ task setup

Starting CockroachDB single-node on :26260...  ✓
Applying schema db/001_schema.sql...            ✓
Database: fable (4 tables)
```

**Failure mode:** "Port in use" → `task clean` then retry; "Docker not found" →
install instructions.

#### `task demo`

**Purpose:** Run the race interactively (narrated, step by step).

**Expected duration:** 60 seconds

**Expected terminal output:**
```
$ task demo

Setting up: promoted belief + live action...

Evidence lands: "Vendor confirms patch deployed"
  → System stands down: cancel action, retract belief

Agent races: "Security Agent: ESCALATE"

  LANE A (typical FK):  escalation commits  ← AUDIT = 1
  LANE B (gate FK):     escalation refused  ← AUDIT = 0

Receipt: ERROR: violates foreign key constraint "gate" (SQLSTATE 23503)
```

**Failure mode:** Cluster not running → "Run `task setup` first."

#### `task race`

**Purpose:** Run only the concurrency race, no setup narration.

**Expected duration:** 30 seconds

**Expected terminal output:**
```
$ task race

Racing: cascade-vs-new-intent on two schemas...

  LANE A: AUDIT = 1
  LANE B: AUDIT = 0

  PASS — schema enforced the gate
```

**Failure mode:** Cluster not running → same as above.

#### `task verify`

**Purpose:** Run the full invariant suite (M2 tests).

**Expected duration:** 60 seconds

**Expected terminal output:**
```
$ task verify

=== Wave 0 === (1 case)    ✓
=== Wave 1 === (13 cases)  ✓
=== Wave 2 === (8 cases)   ✓
=== Wave 3 === (3 cases)   ✓

25/25 PASS
```

**Failure mode:** Any FAIL → `docs/M2_FAILURE.md` is produced with the
architectural failure report.

#### `task clean`

**Purpose:** Stop the cluster and remove temporary state.

**Expected duration:** 5 seconds

**Expected terminal output:**
```
$ task clean

Stopping CockroachDB...  ✓
Removing test databases... ✓
```

**Failure mode:** Cluster already stopped → "Already clean."

#### `task doctor`

**Purpose:** Diagnose common problems.

**Expected duration:** 10 seconds

**Expected terminal output:**
```
$ task doctor

Checking prerequisites...
  Docker:       ✓ (version 24.0.7)
  CockroachDB:  ✓ (v26.2.0)
  Go:           ✓ (go1.25.0)
  Port 26260:   ✓ (available)

Checking cluster...
  Reachable:    ✓
  Schema:       ✓ (4 tables)

Everything looks good. Run `task wow` to start.
```

**Failure mode:** Always succeeds — it diagnoses, it doesn't fix.

---

## Duration Budget

| Command | Max Acceptable | Notes |
|---|---|---|
| `task wow` | 120s | Cluster start is the variable |
| `task setup` | 30s | 15s if cluster already up |
| `task demo` | 60s | The race itself is ~30s |
| `task race` | 30s | Just the race |
| `task verify` | 90s | Full M2 suite × 2 runs |
| `task clean` | 10s | Always fast |
| `task doctor` | 15s | Diagnostic only |

---

## Acceptance Criteria

- [ ] Every command has a stated maximum acceptable duration.
- [ ] Every command has a stated failure message in plain English.
- [ ] `task wow` alone reproduces Journey B's entire "whoa" moment with no
  other command required first.
- [ ] No command requires typing SQL, editing config, or knowing the domain.
