-- The gatekeeper's mandated sequence, then the invariants as executable refusals.
-- Convention: forbidden operations are attempted inside DO blocks; the kernel PASSES
-- a test by REFUSING the operation. If the operation is allowed, we raise FAIL loudly.

\set ON_ERROR_STOP on
\set QUIET on
\pset tuples_only on

-- ---------- fixtures ----------
INSERT INTO belief (id, scenario_id, claim, claim_type) VALUES
 ('00000000-0000-0000-0000-00000000000a','11111111-1111-1111-1111-111111111111',
  'root: exposure signal absent for asset A','accommodated'),
 ('00000000-0000-0000-0000-00000000000b','11111111-1111-1111-1111-111111111111',
  'child: asset A triage may be deferred','derived');
INSERT INTO belief_edge (parent_id, child_id) VALUES
 ('00000000-0000-0000-0000-00000000000a','00000000-0000-0000-0000-00000000000b');
INSERT INTO evidence (scenario_id, belief_id, provenance_class, source_url, content_sha256)
 VALUES ('11111111-1111-1111-1111-111111111111','00000000-0000-0000-0000-00000000000a',
         'external_feed','https://example.invalid/feed/item','deadbeef');

-- ---------- I-1: promotion with open debt is unrepresentable ----------
DO $$ BEGIN
  UPDATE belief SET status='promoted' WHERE id='00000000-0000-0000-0000-00000000000a';
  RAISE EXCEPTION 'I-1 FAIL: promoted with open debt';
EXCEPTION WHEN check_violation THEN RAISE NOTICE 'I-1 PASS: % ', SQLERRM; END $$;

-- retire all debt on both beliefs (the Test step of the pipeline)
UPDATE belief SET debt='{}' WHERE scenario_id='11111111-1111-1111-1111-111111111111';

-- ---------- I-2: final-truth language blocks promotion ----------
UPDATE belief SET final_truth=true WHERE id='00000000-0000-0000-0000-00000000000a';
DO $$ BEGIN
  UPDATE belief SET status='promoted' WHERE id='00000000-0000-0000-0000-00000000000a';
  RAISE EXCEPTION 'I-2 FAIL: promoted final-truth claim';
EXCEPTION WHEN check_violation THEN RAISE NOTICE 'I-2 PASS: %', SQLERRM; END $$;
UPDATE belief SET final_truth=false WHERE id='00000000-0000-0000-0000-00000000000a';

-- ---------- I-3: intent on an unpromoted belief is refused by the FK ----------
DO $$ BEGIN
  INSERT INTO action_intent (scenario_id, belief_id, action)
  VALUES ('11111111-1111-1111-1111-111111111111','00000000-0000-0000-0000-00000000000b','defer');
  RAISE EXCEPTION 'I-3 FAIL: intent accepted on unpromoted belief';
EXCEPTION WHEN foreign_key_violation THEN RAISE NOTICE 'I-3 PASS: %', SQLERRM; END $$;

-- ---------- promote (the throne, debt paid) ----------
UPDATE belief SET status='promoted'
 WHERE id IN ('00000000-0000-0000-0000-00000000000a','00000000-0000-0000-0000-00000000000b');

-- intent now succeeds
INSERT INTO action_intent (id, scenario_id, belief_id, action) VALUES
 ('00000000-0000-0000-0000-0000000000c1','11111111-1111-1111-1111-111111111111',
  '00000000-0000-0000-0000-00000000000b','defer_patch');

-- ---------- I-4: wrong-order retraction (skip cancel) is refused by the schema ----------
DO $$ BEGIN
  UPDATE belief SET status='retracted' WHERE id='00000000-0000-0000-0000-00000000000b';
  RAISE EXCEPTION 'I-4 FAIL: retracted a belief carrying a live intent';
EXCEPTION WHEN check_violation THEN RAISE NOTICE 'I-4 PASS: %', SQLERRM; END $$;

-- ---------- the correct cascade: cancel-then-retract, one transaction ----------
BEGIN;
CREATE TEMP TABLE _desc ON COMMIT DROP AS
  WITH RECURSIVE d(id) AS (
    SELECT '00000000-0000-0000-0000-00000000000a'::uuid
    UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
  ) SELECT id FROM d;
UPDATE action_intent SET state='cancelled'
  WHERE state='live' AND belief_id IN (SELECT id FROM _desc);
UPDATE belief SET status='retracted'
  WHERE status='promoted' AND id IN (SELECT id FROM _desc);
COMMIT;

-- ---------- post-cascade assertions ----------
SELECT CASE WHEN count(*)=2 THEN 'CASCADE PASS: both beliefs retracted'
       ELSE 'CASCADE FAIL' END
FROM belief WHERE status='retracted'
  AND scenario_id='11111111-1111-1111-1111-111111111111';

-- Bug-B positive check: the cancelled intent SURVIVES, with belief_status
-- auto-cascaded to 'retracted' — history preserved, invariant intact.
SELECT CASE WHEN state='cancelled' AND belief_status='retracted'
       THEN 'BUG-B FIX PASS: cancelled intent carries cascaded status'
       ELSE 'BUG-B FIX FAIL: '||state||'/'||belief_status END
FROM action_intent WHERE id='00000000-0000-0000-0000-0000000000c1';

-- ---------- I-5: global audit — zero live intents on non-promoted beliefs ----------
SELECT CASE WHEN count(*)=0 THEN 'I-5 PASS: audit clean'
       ELSE 'I-5 FAIL: '||count(*)||' corrupt rows' END
FROM action_intent a JOIN belief b ON b.id=a.belief_id
WHERE a.state='live' AND b.status <> 'promoted';

-- I-6 is satisfied by construction: \d belief contains no vector column.
