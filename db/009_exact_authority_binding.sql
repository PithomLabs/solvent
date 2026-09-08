-- 009: Exact Authority Binding — bind action_intent to the exact (target_id, snapshot_id)
-- that was approved, closing the confused-deputy class.
--
-- The kernel already writes target_id and snapshot_id on intent creation
-- (AuthorizeAndCreateIntent path) and reads them at claim time (ClaimIntent CAS).
-- This migration makes the schema enforce what the kernel already asserts:
--   1. Composite FK: intent(target_id, snapshot_id) → target_snapshot(target_id, snapshot_id)
--   2. Unique index: at most one live intent per (target_id, snapshot_id)
--
-- Idempotent DDL: safe to apply multiple times.

-- Add nullable columns for exact authority binding.
-- NULLs are allowed: the IntentOnPromoted path (pre-approval) creates intents
-- with no authority binding; only AuthorizeAndCreateIntent sets these.
ALTER TABLE action_intent ADD COLUMN IF NOT EXISTS target_id   UUID;
ALTER TABLE action_intent ADD COLUMN IF NOT EXISTS snapshot_id UUID;

-- Composite FK: mirrors target_activation's FK pattern.
-- Prevents: intent referencing a snapshot that does not exist or belongs to a
-- different target. The FK is DEFERRABLE to allow the SERIALIZABLE transaction
-- boundary in AuthorizeAndCreateIntent to order the INSERT before FK validation.
ALTER TABLE action_intent
    DROP CONSTRAINT IF EXISTS intent_authority_binding_fk;
ALTER TABLE action_intent
    ADD CONSTRAINT intent_authority_binding_fk
        FOREIGN KEY (target_id, snapshot_id)
        REFERENCES target_snapshot(target_id, snapshot_id)
        ON DELETE CASCADE;

-- Unique index: at most one live intent per (target_id, snapshot_id).
-- Prevents: two concurrent live intents on the same approved snapshot.
-- This is the DB-enforced duplicate-intent guard that ErrDuplicateIntent wraps.
CREATE UNIQUE INDEX IF NOT EXISTS live_intent_per_snapshot
    ON action_intent (target_id, snapshot_id)
    WHERE state = 'live' AND target_id IS NOT NULL AND snapshot_id IS NOT NULL;
