-- Fable kernel proof: schema (pure SQL, PostgreSQL 16 / CockroachDB-compatible core)
-- Gate: 4 tables (edge table authorized by gatekeeper, Objection 1).
-- I-6: no embedding column. The ledger is meaningful with zero vectors (Objection 2).
-- Feed-agnostic: this file must never name a specific evidence source (Objection 3).

CREATE TABLE belief (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id UUID NOT NULL,
  claim       TEXT NOT NULL,
  claim_type  TEXT NOT NULL CHECK (claim_type IN ('derived','accommodated','postulated')),
  status      TEXT NOT NULL DEFAULT 'entered'
              CHECK (status IN ('entered','promoted','retracted')),
  -- EBP-literal: debt lives ON the idea (Lean kernel: debt : List DebtItem).
  --
  -- AMENDED IN PHASE 5, authorised by demo2/phase5_imp_review.md section 1. The six
  -- items were inherited physics-proof vocabulary; they are now the deployment-review
  -- obligations the demo is about. Same count, same order, same mechanism.
  --
  -- This literal and kernel.FullDebt are the two encodings of one fact. Tests B-17 and
  -- B-23 compare them behaviourally and fail loudly on drift.
  --
  -- An already-created database does not re-apply this file, so the matching
  -- ALTER COLUMN ... SET DEFAULT lives in db/004_debt_vocabulary.sql. Both are needed:
  -- this one for fresh databases, that one for existing ones.
  debt        TEXT[] NOT NULL DEFAULT ARRAY[
                'needProvenanceCheck','needContradictionSweep','needBlastRadius',
                'needRollbackPlan','needVersionPin','needOperatorSignoff'],
  final_truth BOOLEAN NOT NULL DEFAULT false,
  -- Invariant I-1/I-2: a promoted belief cannot carry debt or final-truth language.
  CONSTRAINT promoted_is_debt_free
    CHECK (status <> 'promoted'
           OR (coalesce(array_length(debt,1),0) = 0 AND NOT final_truth)),
  -- FK target for the actionability gate.
  CONSTRAINT belief_id_status_key UNIQUE (id, status)
);

CREATE TABLE belief_edge (
  parent_id UUID NOT NULL REFERENCES belief(id),
  child_id  UUID NOT NULL REFERENCES belief(id),
  kind      TEXT NOT NULL DEFAULT 'derives' CHECK (kind IN ('derives','contradicts')),
  filed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (parent_id, child_id),
  CHECK (parent_id <> child_id)
);
CREATE INDEX belief_edge_child ON belief_edge (child_id);

CREATE TABLE evidence (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id        UUID NOT NULL,
  belief_id          UUID NOT NULL REFERENCES belief(id),
  provenance_class   TEXT NOT NULL CHECK (provenance_class IN
    ('external_feed','reproducible_artifact','live_scan','operator_asserted')),
  source_url         TEXT,
  snapshot           JSONB,
  content_sha256     TEXT NOT NULL,
  source_observed_at TIMESTAMPTZ,
  ingested_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE action_intent (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id   UUID NOT NULL,
  belief_id     UUID NOT NULL,
  belief_status TEXT NOT NULL DEFAULT 'promoted',
  action        TEXT NOT NULL,
  state         TEXT NOT NULL DEFAULT 'live'
                CHECK (state IN ('live','cancelled','executed')),
  -- Invariant I-3: a LIVE intent may only cite a promoted belief.
  CONSTRAINT live_requires_promoted CHECK (state <> 'live' OR belief_status = 'promoted'),
  -- THE GATE (Bug-B fix): composite FK with ON UPDATE CASCADE.
  -- Retraction propagates the parent's new status into child rows; the CHECK above
  -- then detonates iff a live intent survived — cancel-before-retract is enforced
  -- by the schema itself, not by developer discipline.
  CONSTRAINT gate FOREIGN KEY (belief_id, belief_status)
    REFERENCES belief (id, status) ON UPDATE CASCADE
);
CREATE INDEX live_intents ON action_intent (belief_id) WHERE state = 'live';
