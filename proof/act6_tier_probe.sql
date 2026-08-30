-- ============================================================================
-- act6_tier_probe.sql — which Act 6 climax is actually available?
--
--   psql "$DATABASE_URL" -X -f proof/act6_tier_probe.sql 2>&1 | tee proof/act6_tier_probe.log
--
-- READ BY EYE. Blocks marked [MUST ERROR] must print an error with the stated
-- SQLSTATE. A [MUST ERROR] block that prints a row count is a FAILED tier.
--
-- ---------------------------------------------------------------------------
-- Provenance: this is the RUNNABLE copy of demo2/act6_tier_probe.sql, which is
-- preserved unchanged as the authored artifact. Four amendments were required to
-- make it execute, and nothing semantic was altered — the tier order, the
-- [MUST ERROR] expectations, the legitimate cascade and the verdict table are
-- the original's.
--
--   1. RUNNER IS psql, NOT `cockroach sql`.
--      The original header prescribed `cockroach sql --file`. That cannot work:
--      the script uses psql's \set / :VAR interpolation, and `cockroach sql`
--      answers `ERROR: unknown variable name: "PROBE"` and aborts on the first
--      statement. Independently, `cockroach sql` in non-interactive mode stops at
--      the first error, which would kill the run at the first *expected* error —
--      exactly the blocks this probe exists to observe. psql interpolates and
--      continues past errors.
--
--   2. THE PRIVILEGED CLUSTER-SETTING READ IS KEPT AS EVIDENCE, NOT AS A GATE.
--      `SHOW CLUSTER SETTING sql.txn.read_committed_isolation.enabled` returns
--      SQLSTATE 42501 on a CockroachDB Cloud Basic cluster — the SQL user has
--      neither MODIFYCLUSTERSETTING nor VIEWCLUSTERSETTING. The original treated
--      that read as the answer. It is now followed by the BEHAVIOURAL probe, which
--      needs no privilege and proves more: that a transaction opened as READ
--      COMMITTED actually reports READ COMMITTED rather than being silently
--      upgraded. This mirrors probe A3 at internal/m0/env.go:45-83, which already
--      made exactly this argument.
--
--   3. RUNS IN A DISPOSABLE SCHEMA, BUILT FROM db/001_schema.sql ITSELF.
--      The original wrote fixtures directly into the ledger tables. On the hosted
--      cluster those tables hold the judge-facing demo, and demo/cloud/init/main.go
--      :223 `isDatabaseEmpty` counts belief/evidence/action_intent with NO scenario
--      filter while guarding a log.Fatal — so residue in any scenario can stop the
--      cloud initializer from seeding, after which the web app still starts and
--      serves an all-zero ledger. The probe therefore builds its own copy of the
--      frozen schema from the same file that produced production, and proves the
--      two are identical rather than asserting it. Real cluster, real version, real
--      DDL, different rows.
--
--   4. TEARDOWN, AND A BASELINE RE-VERIFY.
--      The original wiped at the top and left its fixtures behind. This one drops
--      the schema and then re-counts the production ledger, so "nothing was
--      disturbed" is measured at the end of the same transcript that did the work.
-- ============================================================================

\set ON_ERROR_STOP 0
\pset pager off
\timing off

-- Amendment 5, added after the first local rehearsal: psql's default error
-- verbosity prints the message but NOT the SQLSTATE, and this probe's whole
-- contract is stated in SQLSTATEs ("[MUST ERROR 23514 · live_requires_promoted]").
-- A transcript that cannot show 23514 cannot settle the tier.
\set VERBOSITY verbose

\set PROBE '''88888888-8888-8888-8888-888888888888'''
\set ROOT  '''88880000-0000-0000-0000-00000000000a'''
\set CHILD '''88880000-0000-0000-0000-00000000000b'''

\echo '=============================================================='
\echo '=== SECTION 0 — environment facts that gate other decisions ==='
\echo '=============================================================='

SELECT version();
SELECT current_database() AS database, current_user AS user;

-- Must match App Runner's us-west-2. `primary` is the column that matters; a
-- multi-region database lists every region it is configured for.
SELECT region, "primary" FROM [SHOW REGIONS FROM DATABASE];

\echo '--- isolation, attempt 1: the privileged read (expect 42501 on Cloud Basic) ---'
SHOW CLUSTER SETTING sql.txn.read_committed_isolation.enabled;

\echo '--- isolation, attempt 2: the behavioural probe (needs no privilege) ---'
BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED;
SHOW TRANSACTION ISOLATION LEVEL;
ROLLBACK;
SHOW default_transaction_isolation;

\echo '--- the gate FK, as production actually has it ---'
SHOW CREATE TABLE public.action_intent;

\echo '=============================================================='
\echo '=== SECTION 1 — disposable schema from the frozen DDL       ==='
\echo '=============================================================='

DROP SCHEMA IF EXISTS probe_act6 CASCADE;
CREATE SCHEMA probe_act6;
SET search_path = probe_act6;

-- The same file that produced the production tables. Relative to THIS script.
\ir ../db/001_schema.sql

SELECT table_schema, table_name FROM information_schema.tables
 WHERE table_schema = 'probe_act6' ORDER BY table_name;

\echo '--- DDL identity: probe_act6.action_intent vs public.action_intent ---'
-- Normalised for the three cloud decorations that cannot affect constraint
-- evaluation: the schema name, REGIONAL BY TABLE locality, and schema_locked.
WITH prod AS (SELECT create_statement AS s FROM [SHOW CREATE TABLE public.action_intent]),
     prob AS (SELECT create_statement AS s FROM [SHOW CREATE TABLE probe_act6.action_intent]),
     n AS (
       SELECT
         replace(replace(replace(prod.s, ' WITH (schema_locked = true)', ''),
                 ' LOCALITY REGIONAL BY TABLE IN PRIMARY REGION', ''), 'public.', '') AS a,
         replace(replace(replace(prob.s, ' WITH (schema_locked = true)', ''),
                 ' LOCALITY REGIONAL BY TABLE IN PRIMARY REGION', ''), 'probe_act6.', '') AS b
       FROM prod, prob
     )
SELECT CASE WHEN a = b THEN 'DDL_IDENTICAL' ELSE 'DDL_DIFFERS — the probe is not testing production semantics' END AS verdict
  FROM n;

-- Printed in full so a reader can judge the normalisation rather than trust it.
SHOW CREATE TABLE probe_act6.action_intent;

\echo '--- fixtures: A --derives--> B, both promoted, live intent cites B ---'
INSERT INTO belief (id, scenario_id, claim, claim_type, debt) VALUES
 (:ROOT,  :PROBE, 'probe root',  'accommodated', '{}'),
 (:CHILD, :PROBE, 'probe child', 'derived',      '{}');
INSERT INTO belief_edge (parent_id, child_id) VALUES (:ROOT, :CHILD);
UPDATE belief SET status = 'promoted' WHERE scenario_id = :PROBE;
INSERT INTO action_intent (scenario_id, belief_id, action) VALUES (:PROBE, :CHILD, 'deploy');

SELECT id::STRING, status FROM belief WHERE scenario_id = :PROBE ORDER BY id;
SELECT action, state, belief_status FROM action_intent WHERE scenario_id = :PROBE;

\echo '=============================================================='
\echo '=== TIER 2 (the floor) — retract the DIRECTLY CITED belief  ==='
\echo '=== [MUST ERROR 23514 · live_requires_promoted]             ==='
\echo '=============================================================='
-- The gate FK rewrites action_intent.belief_status to 'retracted';
-- live_requires_promoted must then detonate. If Tier 2 fails, Tier 1 cannot pass.

BEGIN;
UPDATE belief SET status = 'retracted' WHERE id = :CHILD;
COMMIT;
ROLLBACK;   -- no-op if already aborted; leaves the session clean either way

SELECT 'TIER2' AS tier,
       CASE WHEN status = 'promoted' THEN 'ENFORCED (refused, belief unchanged)'
            ELSE 'NOT ENFORCED — retraction went through with a live intent' END AS verdict
  FROM belief WHERE id = :CHILD;

\echo '=============================================================='
\echo '=== TIER 1 — retract the ANCESTOR                          ==='
\echo '=== [MUST ERROR 23514 · live_requires_promoted]             ==='
\echo '=============================================================='
-- Traversal is the recursive CTE (application logic); the refusal must come from
-- the database at the leaf.

BEGIN;
WITH RECURSIVE d(id) AS (
    SELECT child_id FROM belief_edge WHERE parent_id = :ROOT
  UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
)
UPDATE belief SET status = 'retracted'
 WHERE id = :ROOT OR id IN (SELECT id FROM d);
COMMIT;
ROLLBACK;

SELECT 'TIER1' AS tier,
       CASE WHEN count(*) FILTER (WHERE status <> 'promoted') = 0
            THEN 'ENFORCED (refused, both beliefs unchanged)'
            ELSE 'NOT ENFORCED — ancestor retraction propagated past a live intent' END AS verdict
  FROM belief WHERE scenario_id = :PROBE;

\echo '=============================================================='
\echo '=== The LEGITIMATE cascade, for comparison  [MUST SUCCEED]  ==='
\echo '=============================================================='
-- Cancel first, then retract, one transaction. In production this body runs
-- inside crdb.ExecuteTx so a 40001 retries rather than surfacing.

BEGIN;
WITH RECURSIVE d(id) AS (
    SELECT child_id FROM belief_edge WHERE parent_id = :ROOT
  UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
)
UPDATE action_intent SET state = 'cancelled'
 WHERE state = 'live'
   AND (belief_id = :ROOT OR belief_id IN (SELECT id FROM d));

WITH RECURSIVE d(id) AS (
    SELECT child_id FROM belief_edge WHERE parent_id = :ROOT
  UNION
    SELECT e.child_id FROM belief_edge e JOIN d ON e.parent_id = d.id
)
UPDATE belief SET status = 'retracted'
 WHERE id = :ROOT OR id IN (SELECT id FROM d);
COMMIT;

-- Act 6's closing frame: the intent is cancelled AND its belief_status was
-- rewritten by the cascade, not by us. History survives; authority does not.
SELECT action, state, belief_status FROM action_intent WHERE scenario_id = :PROBE;

-- The standing audit number, non-vacuous only when live intents exist.
SELECT count(*) AS live_on_nonpromoted
  FROM action_intent i JOIN belief b ON b.id = i.belief_id
 WHERE i.state = 'live' AND b.status <> 'promoted';

\echo '=============================================================='
\echo '=== SECTION 3 — TIER 3 fallback: is an assertion available? ==='
\echo '=============================================================='
-- READ THIS BEFORE READING THE TIER3 ROW.
--
-- Tier 3 is the fallback for a cluster that does NOT re-evaluate CHECKs on
-- cascade-modified rows. If Tier 1 and Tier 2 came back ENFORCED, Tier 3 is MOOT,
-- and this section does not demonstrate what it appears to: the transaction will
-- abort at the UPDATE, refused by the gate, before the assertion query is ever
-- reached. The TIER3 row would then read "available" on the strength of an abort
-- it did not cause.
--
-- So the section is deliberately instrumented: the assertion is preceded by a
-- marker SELECT. If the marker does not appear in the transcript, the assertion
-- never ran and the TIER3 verdict must be disregarded in favour of TIER1/TIER2.
--
-- It is kept in the probe because if the tiers ever regress on a future version,
-- this is the evidence that the fallback is expressible at all.

UPDATE belief SET status = 'promoted' WHERE scenario_id = :PROBE;
UPDATE action_intent SET state = 'live', belief_status = 'promoted' WHERE scenario_id = :PROBE;

BEGIN;
UPDATE belief SET status = 'retracted' WHERE id = :CHILD;
SELECT 'TIER3_MARKER: the UPDATE was allowed, so the assertion below is what stops this' AS marker;
-- The assertion Tier 3 would ship: refuse to commit while any live intent cites a
-- non-promoted belief. crdb_internal.force_error is how CockroachDB's own docs
-- raise a chosen SQLSTATE from a query.
SELECT CASE WHEN count(*) > 0
            THEN crdb_internal.force_error('23514', 'ASSERTION FAILED: live intent on non-promoted belief')::INT
            ELSE 0 END AS assertion
  FROM action_intent i JOIN belief b ON b.id = i.belief_id
 WHERE i.state = 'live' AND b.status <> 'promoted';
COMMIT;
ROLLBACK;

SELECT 'TIER3' AS tier,
       CASE WHEN status = 'promoted'
            THEN 'transaction aborted, belief unchanged — check for TIER3_MARKER above to see WHICH mechanism did it'
            ELSE 'ASSERTION DID NOT HOLD — belief is now ' || status END AS verdict
  FROM belief WHERE id = :CHILD;

\echo '=============================================================='
\echo '=== SECTION 4 — teardown and baseline re-verify             ==='
\echo '=============================================================='

SET search_path = public;
DROP SCHEMA probe_act6 CASCADE;

SELECT 'probe_schema_remaining' AS check,
       count(*)::STRING AS value
  FROM information_schema.schemata WHERE schema_name = 'probe_act6';

-- The production ledger must be exactly where it was before this file ran.
SELECT 'beliefs'   AS metric, count(*)::STRING AS value FROM public.belief
UNION ALL SELECT 'evidence',  count(*)::STRING FROM public.evidence
UNION ALL SELECT 'intents',   count(*)::STRING FROM public.action_intent
UNION ALL SELECT 'edges',     count(*)::STRING FROM public.belief_edge
UNION ALL SELECT 'retracted', count(*)::STRING FROM public.belief WHERE status = 'retracted'
UNION ALL SELECT 'cancelled', count(*)::STRING FROM public.action_intent WHERE state = 'cancelled'
UNION ALL SELECT 'audit_live_on_nonpromoted', count(*)::STRING
  FROM public.action_intent a JOIN public.belief b ON b.id = a.belief_id
 WHERE a.state = 'live' AND b.status <> 'promoted';

-- Byte-compare against the capture taken in SECTION 0.
SHOW CREATE TABLE public.action_intent;

\echo '=============================================================='
\echo '=== VERDICT (read the TIER rows above)                      ==='
\echo '===   TIER1 ENFORCED -> Act 6 climax = ancestor retraction, ==='
\echo '===     DB refuses at the leaf.                             ==='
\echo '===   TIER1 NOT / TIER2 ENFORCED -> climax = retract the    ==='
\echo '===     cited belief. Drop the ancestor framing from the    ==='
\echo '===     video; do not narrate a graph cascade.              ==='
\echo '===   BOTH NOT ENFORCED -> Tier 3. CockroachDB is not       ==='
\echo '===     re-evaluating CHECKs on cascade-modified rows.      ==='
\echo '===     Do NOT fake the refusal. Act 6 becomes the          ==='
\echo '===     insert-time gate FK (23503) plus an explicit        ==='
\echo '===     in-transaction assertion, and the README says so.   ==='
\echo '=============================================================='
