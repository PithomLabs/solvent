-- Service layer tables for the Solvent Commercial MVP.
-- These tables extend the frozen kernel schema (001-006) with product features.

-- Workflow tokens: typed, lifecycle-tracked authorizations.
CREATE TABLE workflow_token (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id   UUID NOT NULL,
  belief_id     UUID NOT NULL,
  action_type   TEXT NOT NULL,
  state         TEXT NOT NULL DEFAULT 'pending'
                CHECK (state IN ('pending','prepared','executing','completed','failed','expired')),
  payload       JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at    TIMESTAMPTZ,
  completed_at  TIMESTAMPTZ,
  failure_error TEXT
);
CREATE INDEX workflow_token_scenario ON workflow_token (scenario_id);
CREATE INDEX workflow_token_state ON workflow_token (state) WHERE state IN ('pending','prepared');

-- Policy tool registry.
CREATE TABLE policy_tool (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name             TEXT NOT NULL UNIQUE,
  class            TEXT NOT NULL CHECK (class IN ('read','mutate','orchestrate')),
  required_beliefs JSONB NOT NULL DEFAULT '[]',
  description      TEXT NOT NULL DEFAULT '',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Policy actor registry.
CREATE TABLE policy_actor (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  roles      JSONB NOT NULL DEFAULT '[]',
  max_class  TEXT NOT NULL CHECK (max_class IN ('read','mutate','orchestrate')),
  active     BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Activity audit ledger.
CREATE TABLE audit_activity (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scenario_id    UUID NOT NULL,
  type           TEXT NOT NULL,
  actor_id       TEXT,
  subject_id     TEXT,
  details        JSONB,
  sqlstate       TEXT,
  constraint_name TEXT,
  refusal        BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_activity_scenario ON audit_activity (scenario_id);
CREATE INDEX audit_activity_refusal ON audit_activity (scenario_id) WHERE refusal = true;
