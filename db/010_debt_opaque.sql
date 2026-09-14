-- DEBT IS OPAQUE TO SOLVENT.
--
-- The domain/application layer defines the meaning of debt identifiers.
-- Solvent only enforces empty-vs-non-empty for promotion.
--
-- This migration supersedes 004's SET DEFAULT for new inserts.
-- Existing rows retain their historical debt values.
--
-- Idempotent. Applied on every container start.

ALTER TABLE belief ALTER COLUMN debt SET DEFAULT ARRAY[]::TEXT[];
