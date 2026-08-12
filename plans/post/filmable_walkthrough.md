# Filmable Walkthrough — Implementation Report

**Date:** 2026-08-12
**Script:** `scripts/demo/walkthrough.sh`
**Taskfile entries:** `demo:film`, `demo:film:auto`
**Version:** 2 (adds Action Gate, 7 beats)

---

## 1. Updated Beat Map

| Beat | Banner | Gate | Command(s) | What's on screen |
|---|---|---|---|---|
| 1 | `STAGE 1 · CLEAN SLATE` | — | `task db:reset` + SELECT counts | 0/0/0 |
| 2 | `STAGE 2 · EVIDENCE → BELIEFS` | — | `go run cmd/solvent --reset --scenario $SOLVENT_SCENARIO_1 --fixtures ...` + SELECT beliefs | 3 beliefs with debt |
| 3 | `STAGE 3 · DEBT GATE` | **23514** | `go run cmd/operator-review --belief $DERIVED_ID --action "..." --debt fake` | CHECK constraint refusal |
| 4 | `STAGE 4 · ACTION GATE` | **23503** | `docker exec ... cockroach sql ... INSERT INTO action_intent ...` | FK constraint refusal |
| 5 | `STAGE 5 · CLEAR DEBT → PROMOTE → AUTHORIZE` | — | `go run cmd/operator-review --belief $DERIVED_ID --debt x5 --action "..."` + inspect | promoted + live intent |
| 6 | `STAGE 6 · REPLAY (IDEMPOTENCE)` | — | `go run cmd/solvent --scenario $SOLVENT_SCENARIO_1 --fixtures ...` (no --reset) | 3/3/1 → 3/3/1 |
| 7 | `STAGE 7 · THE REVERSAL` | — | `task db:reset` + track2 full flow + `task inspect` | retracted + cancelled + SAFE |

---

## 2. Debt Gate — Exact Command/Path

**File:** `scripts/demo/walkthrough.sh`, `beat3()` function

```bash
set +e
REFUSAL=$(go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_1" --belief "$DERIVED_BELIEF_ID" \
  --action "deploy etcd v3.5.28" \
  --debt fake 2>&1)
set -e
```

- `--debt fake` passes the "requires --debt" guard (`cmd/operator-review/main.go:150`); `RetireDebt("fake")` is a no-op.
- `Promote()` runs `UPDATE belief SET status = 'promoted' WHERE id = $1` (`internal/kernel/sql.go:26-28`).
- The `promoted_is_debt_free` CHECK constraint (`db/001_schema.sql:19-21`) refuses because the belief still has 5 real debts.
- Kernel wraps as `ErrPromotionBlocked`. Binary prints to stderr, exits 1.

---

## 3. Action Gate — Exact Command/Path

**File:** `scripts/demo/walkthrough.sh`, `beat4()` function

```bash
set +e
ACTION_REFUSAL=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" -e "
INSERT INTO action_intent (scenario_id, belief_id, action)
VALUES ('${SOLVENT_SCENARIO_1}'::UUID, '${ACCOMMODATED_BELIEF_ID}'::UUID, 'deploy etcd v3.5.28');
" 2>&1)
set -e
```

This is the **same INSERT** the kernel runs (`internal/kernel/sql.go:30-32`):
```sql
INSERT INTO action_intent (scenario_id, belief_id, action)
VALUES ($1::UUID, $2::UUID, $3::STRING)
```

The `belief_status` column defaults to `'promoted'` (`db/001_schema.sql:53`). The composite FK (`db/001_schema.sql:63-64`):
```sql
CONSTRAINT gate FOREIGN KEY (belief_id, belief_status)
  REFERENCES belief (id, status) ON UPDATE CASCADE
```
refuses because `(belief_id, 'promoted')` doesn't match any row in `belief` — the belief's actual status is `'entered'`. SQLSTATE 23503.

The `operator-review` binary cannot produce this refusal standalone because review mode requires `--debt` flags and always promotes before attempting intent creation. The raw SQL path is the only way to demonstrate the FK gate independently.

---

## 4. Real 23514 Receipt

```
--- Raw database refusal (SQLSTATE 23514) ---
retired: fake
promote: promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ((status != 'promoted':::STRING) OR ((COALESCE(array_length(debt, 1:::INT8), 0:::INT8) = 0:::INT8) AND (NOT final_truth))) (SQLSTATE 23514)
exit status 1
---
```

---

## 5. Real 23503 Receipt

```
--- Target belief (unpromoted) ---
  ID:     936b7594-45b4-4fad-be6a-114eb5fab373
  Claim:  fix available for etcd-io/etcd v3.5.28
  Status: entered

  Attempt: create intent against unpromoted belief
  (the composite FK must refuse this)

--- Raw database refusal (SQLSTATE 23503) ---
ERROR: insert on table "action_intent" violates foreign key constraint "gate"
SQLSTATE: 23503
DETAIL: Key (belief_id, belief_status)=('936b7594-45b4-4fad-be6a-114eb5fab373', 'promoted') is not present in table "belief".
CONSTRAINT: gate
Failed running "sql"
---
```

---

## 6. Proof Target Belief Was Unpromoted

Before the INSERT attempt, the walkthrough queries and displays:

```
--- Target belief (unpromoted) ---
  ID:     936b7594-45b4-4fad-be6a-114eb5fab373
  Claim:  fix available for etcd-io/etcd v3.5.28
  Status: entered
```

The status is `entered`, not `promoted`. The FK refusal detail confirms: `Key (belief_id, belief_status)=('936b7594-...', 'promoted') is not present in table "belief"`.

---

## 7. Promotion/Authorization Success Receipt

```
--- Retiring debts on derived belief ---
retired: needInvariant
retired: needToyCheck
retired: needNullModel
retired: needObstruction
retired: needFaithfulnessReview
promoted: ok
intent: live (action="deploy etcd v3.5.28")
audit_live_on_nonpromoted: 0
VERDICT: PASS

--- Ledger state ---
  [entered] fix available for etcd-io/etcd v3.5.27
          debt: {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}
  [entered] fix available for etcd-io/etcd v3.5.28
          debt: {needToyCheck,needNullModel,needObstruction,needFaithfulnessReview}
  [promoted] go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg
          debt: {}

--- Intent ---
  action: deploy etcd v3.5.28
  state:  live

  Audit: SAFE
```

Semantic coherence: the derived vulnerability advisory (`go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable`) is promoted, and the action `deploy etcd v3.5.28` authorizes deployment of the fixed version. The fix-available beliefs (`etcd-io/etcd v3.5.27`, `v3.5.28`) remain entered — they are evidence, not the warrant.

---

## 8. Replay Before/After Counts

| Metric | Before | After |
|---|---|---|
| Beliefs | 3 | 3 |
| Evidence | 3 | 3 |
| Intents | 1 | 1 |

**Idempotence confirmed.** No reset between promotion/authorization and replay.

---

## 9. Reversal Receipt

```
FALSIFY
belief: b24a8bbb-21e8-48eb-9cf0-13b96d0eb6d6
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

  Audit: SAFE
```

---

## 10. Hard-Coded String Audit

```
$ grep -n 'SAFE' scripts/demo/walkthrough.sh
266:    echo "  Audit: SAFE"
402:    echo "  Audit: SAFE"

$ grep -n 'PASS' scripts/demo/walkthrough.sh
(none)

$ grep -n 'cancelled' scripts/demo/walkthrough.sh
(none)

$ grep -n 'retracted' scripts/demo/walkthrough.sh
(none)

$ grep -n '23503' scripts/demo/walkthrough.sh
179:# BEAT 4 — ACTION GATE (SQLSTATE 23503)
205:  # because no row matches (id, 'promoted'). SQLSTATE 23503.
214:  echo "--- Raw database refusal (SQLSTATE 23503) ---"

$ grep -n '23514' scripts/demo/walkthrough.sh
153:# BEAT 3 — DEBT GATE (SQLSTATE 23514)
171:  echo "--- Raw database refusal (SQLSTATE 23514) ---"
```

**Classification:**
- `SAFE` (lines 266, 402): presentation label, guarded by `if [ "$AUDIT_COUNT" = "0" ]` — value derived from live SELECT
- `23503` / `23514` (lines 179, 205, 214, 153, 171): **presentation labels only** — banner text and comment documentation. The actual SQLSTATE values in the output come from CockroachDB, not from the script.
- `PASS`, `cancelled`, `retracted`: not present in script — all come from binary/database output

---

## 11. Regression Results

| Check | Result |
|---|---|
| `git diff --stat HEAD -- db/ internal/ cmd/` | **EMPTY** |
| `SELECT count(*) FROM belief_edge` | **0** |
| `unset FABLE_TEST_DSN && task test` | **6 packages pass** (belief, derive, intent, kernel, normalize, pipeline) |
| `go build ./...` | **OK** |
| `go vet ./...` | **OK** |
| `gofmt -l cmd internal` | **No output** |
| Walkthrough run 1 | **EXIT 0** |
| Walkthrough run 2 (consecutive) | **EXIT 0** |

---

## 12. Limitations

1. **Action Gate uses raw SQL, not `cmd/operator-review`.** The `operator-review` binary requires `--debt` in review mode and always promotes before creating intent. There is no way to trigger a standalone 23503 through the binary alone. The raw SQL uses the identical INSERT statement the kernel executes (`internal/kernel/sql.go:30-32`), so the constraint violation is genuine.

2. **Action Gate target is an accommodated belief, not the derived one.** The derived belief is reserved for the debt gate and success path. The accommodated belief (`fix available for etcd-io/etcd v3.5.28`) pairs semantically with the `deploy etcd v3.5.28` action.

3. **Beat numbering shifted.** 7 beats now instead of 6. The `--from` safety comments in the script header are updated.

4. **Beat 7 does a full `db:reset`.** This drops all data, isolating the reversal for clean recording.

---

## Recording Readiness

### Source Verification

All 10 points verified from actual source code (`scripts/demo/walkthrough.sh`, `Taskfile.yml`, `plans/post/filmable_walkthrough.md`):

1. Beat 3 runs `go run cmd/operator-review --belief $DERIVED_BELIEF_ID --action "..." --debt fake` — real Debt Gate, CHECK constraint refusal. ✓
2. Beat 4 queries `Status: entered` from live DB before the INSERT attempt — target belief proven unpromoted. ✓
3. Beat 4 runs `INSERT INTO action_intent (scenario_id, belief_id, action) VALUES (...)` via `cockroach sql` — same SQL as kernel (`internal/kernel/sql.go:30-32`). Composite FK refuses with SQLSTATE 23503. ✓
4. Beat 5 runs `go run cmd/operator-review --belief $DERIVED_BELIEF_ID --debt x5 --action "..."` — retires 5 debts, promotes, creates live intent. ✓
5. Beat 6 runs `go run cmd/solvent --scenario $SOLVENT_SCENARIO_1 --fixtures ...` with NO `--reset` — replay on populated state. ✓
6. Beat 7 runs `task db:reset` then full track2 flow — reset only for isolated reversal. ✓
7. `SAFE` guarded by `if [ "$AUDIT_COUNT" = "0" ]` (live SELECT). `PASS`, `cancelled`, `retracted` not in script. Counts from `query_count`. ✓
8. All SELECT queries use `tail -n +2` to skip TSV header. ✓
9. `--from` comments (lines 13-20) match actual state dependencies for 7 beats. ✓
10. `demo:film` and `demo:film:auto` depend on `db:up` (Taskfile.yml lines 52, 58). ✓

### Run 1

- **Exit:** 0
- **Beat 3 → 23514:** `promote: promotion blocked: open debt or final-truth language: ERROR: failed to satisfy CHECK constraint ... (SQLSTATE 23514)`
- **Beat 4 → 23503:** Target `508d307f-2012-46a8-b560-87946564f166` Status: `entered`. `ERROR: insert on table "action_intent" violates foreign key constraint "gate" SQLSTATE: 23503 DETAIL: Key (belief_id, belief_status)=('508d307f-...', 'promoted') is not present in table "belief".`
- **Beat 5 → promoted + live intent:** 5 retired, `promoted: ok`, `intent: live (action="deploy etcd v3.5.28")`, `Audit: SAFE`
- **Beat 6 → 3/3/1 → 3/3/1:** `Replay verified: no duplicate ledger state created.`
- **Beat 7 → retracted + cancelled + SAFE:** `cascade: retracted 1 belief(s)`, `[retracted] ... intent: cancelled`, `Audit: SAFE`

### Run 2 (consecutive, no manual DB changes)

- **Exit:** 0
- **Beat 3 → 23514:** Same CHECK constraint refusal. ✓
- **Beat 4 → 23503:** Target `0cf93a64-d8f5-4618-8734-9817a70db758` Status: `entered`. Same FK refusal. ✓
- **Beat 5 → promoted + live intent:** Same success path. ✓
- **Beat 6 → 3/3/1 → 3/3/1:** Idempotence confirmed. ✓
- **Beat 7 → retracted + cancelled + SAFE:** Same reversal. ✓

### Regression Results

| Check | Result |
|---|---|
| `git diff --stat HEAD -- db/ internal/ cmd/` | **EMPTY** |
| `SELECT count(*) FROM belief_edge` | **0** |
| `unset FABLE_TEST_DSN && task test` | **6 packages pass** (belief, derive, intent, kernel, normalize, pipeline) |
| `go build ./...` | **OK** |
| `go vet ./...` | **OK** |
| `gofmt -l cmd internal` | **No output** |

### No Source Changes Required

No modifications were made to any frozen file. The walkthrough script, Taskfile, and deliverable are unchanged from the approved state.

RECORDING READY
