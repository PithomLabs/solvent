#!/bin/bash
# Three-cell isolation experiment. bash's only job is to interleave two psql
# sessions; every semantic operation is pure SQL. Cells:
#   1  naive schema  + READ COMMITTED  -> silent corruption commits
#   2  naive schema  + SERIALIZABLE    -> one txn refused (40001)
#   3  hardened gate + READ COMMITTED  -> loud FK failure, invariant intact
# "Naive" means SCHEMA-naive: both transactions run CORRECT logic
# (writer checks status; cascade cancels intents). That is Bug-A's spec.
set -u
PSQL="psql -d kernel -X -q -v ON_ERROR_STOP=0"
run(){ su postgres -c "$PSQL $*"; }

# --- naive comparison table (what a typical app ships): plain FK, no gate ---
run <<'SQL' >/dev/null
CREATE TABLE IF NOT EXISTS naive_intent (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id UUID NOT NULL,
  belief_id UUID NOT NULL REFERENCES belief(id),
  action TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'live' CHECK (state IN ('live','cancelled','executed'))
);
SQL

seed(){ # $1=root uuid $2=child uuid $3=scenario uuid
run <<SQL >/dev/null
INSERT INTO belief (id,scenario_id,claim,claim_type,status,debt) VALUES
 ('$1','$3','root: exposure signal absent','accommodated','promoted','{}'),
 ('$2','$3','child: triage may be deferred','derived','promoted','{}');
INSERT INTO belief_edge (parent_id,child_id) VALUES ('$1','$2');
SQL
}

writer_sql(){ # $1=iso $2=child $3=scenario $4=table
cat <<SQL
BEGIN; SET TRANSACTION ISOLATION LEVEL $1;
SELECT 'writer sees: '||status FROM belief WHERE id='$2';
SELECT pg_sleep(2);
INSERT INTO $4 (scenario_id,belief_id,action) VALUES ('$3','$2','defer_patch');
COMMIT;
SQL
}

cascade_sql(){ # $1=iso $2=root $3=intent table
cat <<SQL
BEGIN; SET TRANSACTION ISOLATION LEVEL $1;
CREATE TEMP TABLE _d ON COMMIT DROP AS
  WITH RECURSIVE d(id) AS (SELECT '$2'::uuid
    UNION SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id=d.id)
  SELECT id FROM d;
UPDATE $3 SET state='cancelled' WHERE state='live' AND belief_id IN (SELECT id FROM _d);
UPDATE belief SET status='retracted' WHERE status='promoted' AND id IN (SELECT id FROM _d);
COMMIT;
SQL
}

audit(){ # $1=intent table $2=scenario
run -tA <<SQL
SELECT 'live-on-nonpromoted='||count(*) FROM $1 a JOIN belief b ON b.id=a.belief_id
WHERE a.state='live' AND b.status<>'promoted' AND a.scenario_id='$2';
SQL
}

cell(){ # $1=name $2=iso $3=table  (uuids derived from cell number $4)
  local R="00000000-0000-0000-000$4-00000000000a" C="00000000-0000-0000-000$4-00000000000b" S="00000000-0000-0000-000$4-111111111111"
  echo "== CELL $1 =="; seed "$R" "$C" "$S"
  writer_sql "$2" "$C" "$S" "$3" >/tmp/w.sql; cascade_sql "$2" "$R" "$3" >/tmp/c.sql
  run -f /tmp/w.sql >/tmp/w.out 2>&1 & WPID=$!
  sleep 0.6; run -f /tmp/c.sql >/tmp/c.out 2>&1; wait $WPID
  grep -hoE "ERROR:.*" /tmp/w.out /tmp/c.out | sed 's/^/  /' | sort -u
  [ -s /tmp/w.out ] && grep -q "writer sees: promoted" /tmp/w.out && echo "  (writer checked status: promoted)"
  echo "  audit: $(audit "$3" "$S")"
}

cell "1: naive schema @ READ COMMITTED " "READ COMMITTED" naive_intent 2
cell "2: naive schema @ SERIALIZABLE   " "SERIALIZABLE"  naive_intent 3
cell "3: hardened gate @ READ COMMITTED" "READ COMMITTED" action_intent 4
