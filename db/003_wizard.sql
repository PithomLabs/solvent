-- Solvent wizard layer: retrieval provenance relation + the refusal log.
--
-- Phase 5. Two changes, both authorised explicitly:
--
--   1. belief_corpus_citation gains `relation`, so a citation can record WHY a
--      corpus row was attached to a belief -- considered during discharge, or
--      introduced as a contradiction during falsification. The wizard needs to tell
--      those apart; nothing else in the schema can.
--
--   2. refusal_log, the application's record of what the database refused.
--
-- # Why relation lives here and not in a new table
--
-- The settled UI contract named a `belief_evidence` table. That table has never
-- existed -- it belongs to a superseded design in which corpus rows were to live in
-- `evidence`. Phase 3 built `corpus_issue` plus `belief_corpus_citation` instead,
-- and that pair already carries distance, query text and retrieval time. One column
-- is the whole gap. A parallel table would duplicate a working model and leave two
-- places to ask the same question.
--
-- The four frozen ledger tables are untouched. Operator-asserted discharge
-- artifacts go to the existing `evidence` table with
-- provenance_class = 'operator_asserted', which 001_schema.sql already permits.
--
-- # Why refusal_log has to exist
--
-- A gate refusal is an aborted transaction, so by design it leaves no trace. That
-- is correct for the ledger and fatal for a wizard: without a record, screen 1 has
-- no exit condition and a reload cannot tell "not attempted yet" from "attempted and
-- refused". So the refusal is written in its OWN transaction, after the pgerror is
-- caught, because inside the refused transaction it would roll back with everything
-- else.
--
-- This is the application recording what the database said. It is not the database
-- logging itself, and the README must not imply otherwise.
--
-- APPLIER CONSTRAINT: internal/testdb and internal/m0 apply .sql files with a
-- splitter that strips `--` line comments and splits on `;`. It is not a SQL parser.
-- This file therefore uses plain DDL only: no dollar-quoting, no /* */ block
-- comments, and no `--` or `;` inside any string literal.
--
-- Every statement is IF NOT EXISTS, like 002 and unlike the frozen 001. The cloud
-- initializer applies this file on EVERY container start, so a non-idempotent
-- statement here would crash the service on the second deploy.

ALTER TABLE belief_corpus_citation ADD COLUMN IF NOT EXISTS relation TEXT NOT NULL DEFAULT 'considered';

ALTER TABLE belief_corpus_citation ADD CONSTRAINT IF NOT EXISTS citation_relation CHECK (relation IN ('considered', 'contradicts'));

CREATE TABLE IF NOT EXISTS refusal_log (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id     UUID NOT NULL,
  statement       TEXT NOT NULL,
  sqlstate        TEXT NOT NULL,
  constraint_name TEXT,
  detail          TEXT,
  logged_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT refusal_statement CHECK (statement IN ('promote', 'authorize', 'discharge', 'retract_unsafe'))
);

CREATE INDEX IF NOT EXISTS refusal_log_scenario ON refusal_log (scenario_id, logged_at);
