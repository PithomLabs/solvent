#!/usr/bin/env bash
# ==============================================================================
# cells_crdb.sh — the three-cell isolation experiment, ported to CockroachDB.
#
#   bash proof/harness/cells_crdb.sh --dsn "$DSN" [--cells 1,2,3]
#
# Cells (IMPLEMENTATION_CONTRACT.md:148-151):
#   1  naive schema  + READ COMMITTED   -> contract predicts silent corruption (audit != 0)
#   2  naive schema  + SERIALIZABLE     -> contract predicts 40001
#   3  hardened gate + READ COMMITTED   -> contract predicts 23503, audit = 0
#
# "Naive" means SCHEMA-naive: both transactions run CORRECT application logic
# (the writer checks status; the cascade cancels intents first). The difference
# between cells 1/2 and cell 3 is entirely in the schema.
#
# ------------------------------------------------------------------------------
# Why this file exists instead of cells.sh
#
# proof/harness/cells.sh is preserved unchanged, but it is a PostgreSQL artifact
# and cannot be presented as CockroachDB evidence. PHASE0_REPORT.md:83 already
# recorded that. Four things had to change:
#
#   1. `su postgres -c "psql -d kernel"` -> a DSN. There is no postgres OS user
#      or `kernel` database on a CockroachDB cluster.
#   2. `CREATE TEMP TABLE _d ON COMMIT DROP` -> an inline recursive CTE.
#      CockroachDB refuses temp tables unless an experimental session var is set:
#      "ERROR: temporary tables are only supported experimentally". The CTE form is
#      what proof/02_lifecycle_and_invariants.sql and act6_tier_probe.sql already use.
#   3. DDL against the target database -> DDL inside a disposable schema, dropped
#      at the end. The naive table is a deliberately-broken strawman; it has no
#      business in the judge-facing cluster, and on the hosted cluster stray rows
#      are actively dangerous (demo/cloud/init/main.go:223 counts belief/evidence/
#      action_intent with no scenario filter while guarding a log.Fatal).
#   4. Fixed /tmp paths -> mktemp.
#
# pg_sleep IS supported on CockroachDB (verified), so the writer still holds its
# transaction open the same way the original did.
#
# ------------------------------------------------------------------------------
# Two things this harness must NOT do
#
#   * It must not assume the PostgreSQL result reproduces. demo2/ui_review.md:1486:
#     "do not pre-write the READ COMMITTED result." CockroachDB's READ COMMITTED
#     gives per-statement snapshots and detects some conflicts that PostgreSQL's
#     does not. If cell 1's anomaly does not appear, that is the finding.
#   * It must not describe 40001 as "refused". demo2/ui_review.md:1482: 40001 is a
#     RETRY signal. Under crdb.ExecuteTx the transaction retries and may then
#     succeed. The accurate chain is detect -> retry -> refusal on fresh state.
#     This harness deliberately does NOT retry, so it can show the raw 40001 that
#     the production path would have absorbed.
# ==============================================================================
set -uo pipefail

DSN=""
CELLS="1,2,3"
SCHEMA="probe_iso"

while [ $# -gt 0 ]; do
  case "$1" in
    --dsn)    DSN="$2"; shift 2 ;;
    --cells)  CELLS="$2"; shift 2 ;;
    --schema) SCHEMA="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$DSN" ] || { echo "usage: cells_crdb.sh --dsn <DSN> [--cells 1,2,3]" >&2; exit 2; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Never let a host or credential reach the transcript.
redact() {
  sed -E 's#postgres(ql)?://[^ ]*#postgresql://<redacted>#g;
          s#[A-Za-z0-9_.-]+\.cockroachlabs\.cloud#<redacted-host>#g'
}

# -X ignores psqlrc; ON_ERROR_STOP=0 so an expected error does not end the cell;
# VERBOSITY verbose is what makes SQLSTATEs visible, and the contract is written
# in SQLSTATEs.
psqlq() { psql "$DSN" -X -q -v ON_ERROR_STOP=0 -v VERBOSITY=verbose "$@" 2>&1 | redact; }
psqlt() { psql "$DSN" -X -q -At -v ON_ERROR_STOP=0 "$@" 2>&1 | redact; }

# ------------------------------------------------------------------------------
# Setup: a disposable schema holding the real frozen DDL plus the naive strawman.
# ------------------------------------------------------------------------------
setup() {
  echo "--- setup: disposable schema '$SCHEMA' from db/001_schema.sql ---"
  psqlq -c "DROP SCHEMA IF EXISTS $SCHEMA CASCADE;" >/dev/null
  psqlq -c "CREATE SCHEMA $SCHEMA;" >/dev/null
  # search_path must be set in the SAME session that runs the DDL.
  psqlq -v s="$SCHEMA" -f /dev/stdin <<SQL >/dev/null
SET search_path = :s;
\\i $REPO_ROOT/db/001_schema.sql
SQL

  # The naive comparison table: what a typical application ships. A plain FK to
  # belief(id) — no composite (id,status) gate, no live_requires_promoted CHECK.
  #
  # Created ONLY when a cell actually needs it. Cell 3 uses the real hardened
  # action_intent, so a cloud run of cell 3 alone never materialises the
  # deliberately-broken strawman on the judge-facing cluster at all.
  local want_naive=no
  case ",$CELLS," in *,1,*|*,2,*) want_naive=yes ;; esac

  if [ "$want_naive" = yes ]; then
    psqlq -c "
SET search_path = $SCHEMA;
CREATE TABLE naive_intent (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id UUID NOT NULL,
  belief_id UUID NOT NULL REFERENCES belief(id),
  action TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'live' CHECK (state IN ('live','cancelled','executed'))
);" >/dev/null
  fi

  echo "    tables: $(psqlt -c "SELECT string_agg(table_name, ' ' ORDER BY table_name) FROM information_schema.tables WHERE table_schema='$SCHEMA';")"
  if [ "$want_naive" = yes ]; then
    echo "    naive_intent carries the gate FK? $(psqlt -c "SELECT count(*)>0 FROM [SHOW CREATE TABLE $SCHEMA.naive_intent] WHERE create_statement LIKE '%belief_status%';")  (must be false — that is what makes it naive)"
  else
    echo "    naive_intent: not created (no naive cell requested, so the strawman never exists on this cluster)"
  fi
  echo "    action_intent carries the gate FK ON UPDATE CASCADE? $(psqlt -c "SELECT count(*)>0 FROM [SHOW CREATE TABLE $SCHEMA.action_intent] WHERE create_statement LIKE '%ON UPDATE CASCADE%';")  (must be true)"
  echo "    action_intent carries live_requires_promoted?        $(psqlt -c "SELECT count(*)>0 FROM [SHOW CREATE TABLE $SCHEMA.action_intent] WHERE create_statement LIKE '%live_requires_promoted%';")  (must be true)"
  echo
}

teardown() {
  echo "--- teardown: dropping schema '$SCHEMA' ---"
  psqlq -c "DROP SCHEMA IF EXISTS $SCHEMA CASCADE;" >/dev/null
  echo "    schemas named '$SCHEMA' remaining: $(psqlt -c "SELECT count(*) FROM information_schema.schemata WHERE schema_name='$SCHEMA';")"
}

seed() { # $1=root $2=child $3=scenario
  psqlq -c "
SET search_path = $SCHEMA;
INSERT INTO belief (id,scenario_id,claim,claim_type,status,debt) VALUES
 ('$1','$3','root: exposure signal absent','accommodated','promoted','{}'),
 ('$2','$3','child: triage may be deferred','derived','promoted','{}');
INSERT INTO belief_edge (parent_id,child_id) VALUES ('$1','$2');" >/dev/null
}

# The writer: reads the belief's status (sees 'promoted'), waits, then inserts a
# LIVE intent citing it. Correct application logic — it checked.
writer_sql() { # $1=iso $2=child $3=scenario $4=table
cat <<SQL
SET search_path = $SCHEMA;
BEGIN TRANSACTION ISOLATION LEVEL $1;
SELECT 'writer sees: '||status AS observed FROM belief WHERE id='$2';
SELECT pg_sleep(2);
INSERT INTO $4 (scenario_id,belief_id,action) VALUES ('$3','$2','defer_patch');
COMMIT;
SQL
}

# The cascade: cancels live intents for the subtree FIRST, then retracts. Also
# correct application logic — cancel-before-retract, in one transaction.
cascade_sql() { # $1=iso $2=root $3=table
cat <<SQL
SET search_path = $SCHEMA;
BEGIN TRANSACTION ISOLATION LEVEL $1;
WITH RECURSIVE d(id) AS (
    SELECT '$2'::UUID
  UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
)
UPDATE $3 SET state='cancelled' WHERE state='live' AND belief_id IN (SELECT id FROM d);
WITH RECURSIVE d(id) AS (
    SELECT '$2'::UUID
  UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
)
UPDATE belief SET status='retracted' WHERE status='promoted' AND id IN (SELECT id FROM d);
COMMIT;
SQL
}

audit() { # $1=table $2=scenario
  psqlt -c "
SET search_path = $SCHEMA;
SELECT count(*) FROM $1 a JOIN belief b ON b.id=a.belief_id
 WHERE a.state='live' AND b.status<>'promoted' AND a.scenario_id='$2';" | tail -1
}

cell() { # $1=label $2=iso $3=table $4=cellnum
  local R="00000000-0000-0000-000$4-00000000000a"
  local C="00000000-0000-0000-000$4-00000000000b"
  local S="00000000-0000-0000-000$4-111111111111"

  echo "=============================================================="
  echo "== CELL $1"
  echo "=============================================================="
  seed "$R" "$C" "$S"

  writer_sql  "$2" "$C" "$S" "$3" > "$TMP/w.sql"
  cascade_sql "$2" "$R" "$3"      > "$TMP/c.sql"

  # Interleave: writer opens its transaction and holds it; the cascade commits
  # underneath it; the writer then tries to insert on what it read earlier.
  psqlq -f "$TMP/w.sql" > "$TMP/w.out" 2>&1 &
  local WPID=$!
  sleep 0.6
  psqlq -f "$TMP/c.sql" > "$TMP/c.out" 2>&1
  wait $WPID

  echo "  writer observed:"
  grep -hoE "writer sees: [a-z]+" "$TMP/w.out" | sed 's/^/    /' || echo "    (nothing)"
  echo "  errors raised (SQLSTATE first):"
  if grep -hqE "ERROR:" "$TMP/w.out" "$TMP/c.out"; then
    grep -hoE "ERROR:  [0-9A-Z]{5}: .*" "$TMP/w.out" "$TMP/c.out" | sed 's/^/    /' | sort -u
    grep -hoE "CONSTRAINT NAME:  .*" "$TMP/w.out" "$TMP/c.out" | sed 's/^/    /' | sort -u
  else
    echo "    (none — both transactions committed)"
  fi
  echo "  belief states after:"
  psqlt -c "SET search_path = $SCHEMA; SELECT status||' x'||count(*) FROM belief WHERE scenario_id='$S' GROUP BY status ORDER BY 1;" | sed 's/^/    /'
  echo "  intents after:"
  psqlt -c "SET search_path = $SCHEMA; SELECT state||' x'||count(*) FROM $3 WHERE scenario_id='$S' GROUP BY state ORDER BY 1;" | sed 's/^/    /'
  echo "  AUDIT live-on-nonpromoted = $(audit "$3" "$S")"
  echo
}

# ------------------------------------------------------------------------------
echo "=============================================================="
echo "cells_crdb.sh — CockroachDB isolation experiment"
echo "=============================================================="
echo "server:  $(psqlt -c 'SELECT version();')"
echo "database:$(psqlt -c 'SELECT current_database();')  user:$(psqlt -c 'SELECT current_user;')"
echo "default_transaction_isolation: $(psqlt -c 'SHOW default_transaction_isolation;')"
echo "cells requested: $CELLS"
echo

setup

case ",$CELLS," in *,1,*) cell "1: naive schema  @ READ COMMITTED" "READ COMMITTED" naive_intent 2 ;; esac
case ",$CELLS," in *,2,*) cell "2: naive schema  @ SERIALIZABLE  " "SERIALIZABLE"   naive_intent 3 ;; esac
case ",$CELLS," in *,3,*) cell "3: hardened gate @ READ COMMITTED" "READ COMMITTED" action_intent 4 ;; esac

teardown
echo
echo "=============================================================="
echo "Read the AUDIT line of each cell. Reminder on interpretation:"
echo "  * 40001 is a RETRY signal, not a refusal. Under crdb.ExecuteTx the"
echo "    transaction retries and may succeed on fresh state."
echo "  * If cell 1 shows AUDIT = 0, the PostgreSQL-era anomaly did NOT"
echo "    reproduce on CockroachDB. Report that; do not keep the old claim."
echo "=============================================================="
