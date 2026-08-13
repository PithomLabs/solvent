-- Solvent corpus layer: external institutional memory, and the citations that
-- connect it to the ledger.
--
-- WHY THIS IS A SEPARATE FILE AND SEPARATE TABLES
-- db/001_schema.sql is frozen and is not edited by this migration. Its four tables
-- keep their exact definitions. The corpus cannot live in `evidence` because
-- evidence.belief_id is NOT NULL REFERENCES belief(id): evidence exists only as
-- evidence FOR a claim. External issues exist in the world before anyone forms a
-- belief about them, so they need somewhere else to live.
--
-- I-6 IS UNCHANGED. The invariant is that the *ledger* is meaningful with zero
-- vectors, and it still is: belief, belief_edge, evidence and action_intent carry
-- no embedding column and no vector index. Promotion, the action gate and the
-- retraction cascade never consult a vector. Retrieval is a separate concern that
-- proposes candidates; it decides nothing.
--
-- IDEMPOTENT BY CONSTRUCTION (unlike 001): every statement is IF NOT EXISTS, so
-- re-applying this file on a warm container restart is a no-op rather than a 42P07.
--
-- APPLIER CONSTRAINT: internal/testdb and internal/m0 apply .sql files with a
-- splitter that strips `--` line comments and splits on `;`. It is not a SQL parser.
-- This file therefore uses plain DDL only: no dollar-quoting, no /* */ block
-- comments, and no `--` or `;` inside any string literal.

CREATE TABLE IF NOT EXISTS corpus_issue (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id    UUID NOT NULL,
  issue_number   INT  NOT NULL,
  title          TEXT NOT NULL,
  body           TEXT,
  state          TEXT NOT NULL CHECK (state IN ('open','closed')),
  url            TEXT NOT NULL,
  closed_at      TIMESTAMPTZ,
  content_sha256 TEXT NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- Nullable on purpose: ingestion lands rows first, embedding backfills second.
  -- A corpus row with no embedding is a legitimate intermediate state.
  embedding      VECTOR(1024),

  -- Deterministic corpus identity. This is what makes re-ingestion idempotent:
  -- the same upstream issue cannot appear twice in a scenario. content_sha256
  -- records WHAT was ingested and detects upstream edits; it does not identify.
  CONSTRAINT corpus_issue_scenario_number_key UNIQUE (scenario_id, issue_number),

  -- Declared inline, and that is load-bearing. Adding a vector index to a
  -- NON-empty table backfills, and CockroachDB blocks table writes for the
  -- duration (documented known limitation). Creating it with the table means the
  -- backfill is over zero rows and no such window exists.
  --
  -- scenario_id is a PREFIX COLUMN, so isolation is enforced inside the index
  -- rather than by a WHERE clause someone has to remember. Verified: the plan for
  -- a scenario-scoped ANN query reads
  --   vector search table: corpus_issue@corpus_issue_embedding_idx
  --   prefix spans: [/'<scenario>' - /'<scenario>']
  -- and a query WITHOUT a scenario_id predicate does not use the index at all.
  --
  -- vector_cosine_ops pairs with the <=> operator. Cosine is the right metric for
  -- text embeddings, and the index only accelerates the operator its opclass names.
  VECTOR INDEX corpus_issue_embedding_idx (scenario_id, embedding vector_cosine_ops)
);

-- Retrieval provenance: which corpus rows the agent actually cited for a belief,
-- how close they were, and what question surfaced them.
--
-- This is the auditable half of retrieval. Without it, "the agent found relevant
-- history" is an assertion. With it, the distance and the query are on the record
-- and a reviewer can re-run the search.
--
-- belief_id carries a real FK. Verified empirically that this inbound reference
-- leaves SHOW CREATE TABLE belief byte-identical, so the frozen tables remain
-- structurally unchanged. TRUNCATE belief CASCADE reaches these citations, which
-- is correct: a reset scenario should drop its citations. corpus_issue is not
-- reached and survives, which is also correct -- the corpus is durable
-- institutional memory, not scenario state.
CREATE TABLE IF NOT EXISTS belief_corpus_citation (
  belief_id    UUID NOT NULL REFERENCES belief(id),
  corpus_id    UUID NOT NULL REFERENCES corpus_issue(id),
  distance     FLOAT8 NOT NULL,
  query_text   TEXT NOT NULL,
  retrieved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (belief_id, corpus_id)
);

CREATE INDEX IF NOT EXISTS belief_corpus_citation_corpus ON belief_corpus_citation (corpus_id);
