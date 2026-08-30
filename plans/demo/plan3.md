# Solvent Judge Playground — Implementation Plan (plan3)

**Source:** `plans/demo/plan2_review.md` + `plans/demo/plan2_review_claude.md`
**Date:** 2026-08-11
**Scope:** 4 corrections to plan2 before implementation. No schema/kernel/architecture changes.

---

## Review corrections (C1–C4)

### C1 — Remove G7 as a gap. Absence of settings is a documented property.

**Finding:** plan2 listed G7 ("setup does not apply non-default cluster/session settings") as a gap. Code inspection proves this is **not a gap** — it is the correct state.

**Evidence:**
- `check_i7.sh:55-61` enforces D-033: zero `CREATE TEMP TABLE` occurrences in the kernel.
- `internal/kernel/sql.go:66-92`: `RetractCascade` uses `WITH RECURSIVE` CTEs, not temp tables.
- `internal/kernel/sql.go:79`: composite FK `ON UPDATE CASCADE` is standard CockroachDB behavior.
- No `SET CLUSTER SETTING` is required anywhere.

**Action:**
- Remove G7 from the gaps list.
- Report must affirmatively state: "No `SET CLUSTER SETTING` required; the recursive-CTE design (D-033) is exactly why."
- Note: earlier proof artifacts discussed temp tables; the current kernel retired that approach in favor of recursive CTEs to satisfy `check_i7`/D-033. The proof lineage is now behind the kernel on this point.

### C2 — Pin CRDB Docker image to v26.2.0.

**Finding:** plan2 pinned `cockroachdb/cockroach:v26.2.1` but the verified local binary is `v26.2.0`. F2 requires pinning the **exact** verified version, not a compatible one.

**Action:**
- Change `scripts/demo/setup.sh` line 45 from `cockroachdb/cockroach:v26.2.1` to `cockroachdb/cockroach:v26.2.0`.
- This is a one-character fix that eliminates the discrepancy entirely.
- The judge's container will byte-match the verification substrate.

### C3 — Sweep ALL demo output for hard-coded claims.

**C3 grep results (completed before writing this plan):**

| Script | Line | Value | Source | Status |
|---|---|---|---|---|
| `track1.sh` | 89 | `"Promoted      1"` | Hard-coded constant | **MUST FIX** |
| `track1.sh` | 91-95 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `track2.sh` | 101-105 | `Audit SAFE/VIOLATION` | DB query (`$AUDIT` variable) | OK |
| `inspect.sh` | 63-67 | `SAFE` | DB query (`$AUDIT` variable) | OK |
| `replay.sh` | 71 | `"Replay verified..."` | Assertion (only prints if counts match) | OK |

**Action:**
- Fix `track1.sh:89`: replace hardcoded `"Promoted 1"` with a DB query for `SELECT count(*) FROM belief WHERE scenario_id=... AND status='promoted'`.

### C4 — Make replay verification an actual pass/fail gate.

**Finding:** plan2's execution order runs replay as a smoke test. F3 requires the idempotence assertion to be the pass/fail gate.

**Dedup key determinism (verified):**
- `Derive()` is a pure function: "Same evidence always produces identical beliefs" (`internal/derive/derive.go:22-23`).
- `ContentSHA256` computed from deterministic fields only — no timestamps, no randomness (`internal/normalize/normalize.go`).
- `EnsureBelief` dedups on `(scenario_id, claim)` — claim strings are deterministic.
- `evidenceExists` dedups on `(belief_id, content_sha256)` — content hashes are deterministic.

**Action:**
- `replay.sh` already captures before/after counts for beliefs, evidence, and intents, and asserts exact equality. This is correct.
- The report must document that the assertion is a real gate, not a smoke test.
- The report must confirm claim keys are deterministic (cite `Derive()` pure function + `ContentSHA256` computation).

---

## Updated gaps list

| # | Gap | Severity | Source | Action |
|---|---|---|---|---|
| G1 | `setup.sh` missing `task` prerequisite check | Medium | F4 | Add check |
| G2 | README port 26260 → 26261 | Low | §10 | Fix |
| G3 | README "single-node insecure on port 26260" → 26261 | Low | §10 | Fix |
| G4 | No `playground_implementation_report.md` for this pass | High | §16 | Create |
| G5 | F1 limitation not in README | High | F1 | Add |
| G6 | Docker image v26.2.1 → v26.2.0 | High | F2 | Fix (C2) |
| ~~G7~~ | ~~setup does not apply cluster settings~~ | ~~Low~~ | ~~F2~~ | **Removed** (C1) |
| G8 | `track1.sh` hard-coded `"Promoted 1"` | Low→High | §11 | Fix (C3) |

---

## Implementation steps

### Step 1: Fix `setup.sh`

1. Add `task` prerequisite check (G1):
   ```bash
   if ! command -v task &>/dev/null; then
     echo "ERROR: task is not installed or not in PATH." >&2
     echo "Install Task: https://taskfile.dev/installation/" >&2
     exit 1
   fi
   ```
2. Update prerequisites echo to include `task`.
3. Pin Docker image to `cockroachdb/cockroach:v26.2.0` (C2/G6).

### Step 2: Fix `track1.sh`

Replace hardcoded `echo "  Promoted      1"` (line 89) with DB query (C3/G8):
```bash
PROMOTED_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}' AND status = 'promoted';
" 2>/dev/null | tail -n1)
echo "  Promoted      ${PROMOTED_COUNT}"
```

### Step 3: Fix `README.md`

1. Fix port references (G2/G3): change "port 26260" to "port 26261 (SQL) / 8082 (HTTP)".
2. Add F1 limitation after Track 2 section (G5):
   > **Note:** The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

### Step 4: Create `plans/post/playground_implementation_report.md`

Contents:
1. Files changed
2. Commands added (9 Taskfile commands)
3. CockroachDB setup mechanism:
   - Docker `cockroachdb/cockroach:v26.2.0` (exact verified version)
   - No `SET CLUSTER SETTING` required; recursive-CTE design (D-033) is why
   - No non-default session settings required
   - Schema applied via `db/001_schema.sql`
4. Track 1 demo result
5. Replay result:
   - Dedup mechanism: `EnsureBelief` CTE (find-or-create on `(scenario_id, claim)`) + `evidenceExists` guard (on `(belief_id, content_sha256)`)
   - Claim keys are deterministic: `Derive()` is pure function, `ContentSHA256` computed from deterministic fields only
   - Assertion is a real gate: before/after count equality for beliefs, evidence, intents
6. Track 2 demo result
7. Inspection output
8. Test results
9. Prerequisites: Docker + Go + Task
10. Schema/kernel unchanged confirmation
11. Limitations:
    - F1 verbatim: "The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. belief_edge-based multi-belief propagation is intentionally outside the current demo."
    - Strategic aside: belief_edge propagation is a deliberate future deferral; a single real dependency edge is what would make RetractCascade propagate; it is dormant by design, not a defect.

### Step 5: Verify

1. `task test` — 107 tests pass
2. `task setup` — Docker CRDB starts, schema applied
3. `task demo:track1` → `task demo:replay` → `task demo:track2` → `task inspect` — full sequence

---

## Execution order

1. Edit `scripts/demo/setup.sh` (C2 + G1)
2. Edit `scripts/demo/track1.sh` (C3/G8)
3. Edit `README.md` (G2/G3 + G5)
4. Create `plans/post/playground_implementation_report.md` (G4)
5. Run verification sequence

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
