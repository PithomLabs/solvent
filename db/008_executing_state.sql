-- Phase 4D: Add 'executing' state to action_intent for idempotent execution.
-- Idempotent DDL: safe to apply multiple times.

-- Drop the original inline CHECK constraint (auto-named 'check_state' by CockroachDB).
ALTER TABLE action_intent
  DROP CONSTRAINT IF EXISTS check_state;

-- Drop any previously-added named constraint (idempotent re-apply).
ALTER TABLE action_intent
  DROP CONSTRAINT IF EXISTS action_intent_state_check;

-- Add the new CHECK constraint including 'executing'.
ALTER TABLE action_intent
  ADD CONSTRAINT action_intent_state_check
  CHECK (state IN ('live','cancelled','executing','executed'));

-- Index for finding executing intents (reconciliation queries).
CREATE INDEX IF NOT EXISTS executing_intents
  ON action_intent (scenario_id) WHERE state = 'executing';
