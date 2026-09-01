-- 006: Add ON UPDATE CASCADE to the justification composite FK.
--
-- The existing unnamed FK was created by 005_authority_mvp.sql as:
--     FOREIGN KEY (belief_id, belief_status) REFERENCES belief(id, status)
--
-- Without ON UPDATE CASCADE, RetractCascade is blocked when a justification
-- references the promoted belief: the UPDATE belief SET status = 'retracted'
-- fails with SQLSTATE 23503 because justification.belief_status still holds
-- 'promoted'.
--
-- With ON UPDATE CASCADE, the status change propagates:
--     belief.status:            promoted → retracted
--     justification.belief_status: promoted → retracted
--
-- Authorize subsequently sees belief_status != 'promoted' and denies.
-- The justification remains as historical/proposal linkage; it does NOT
-- grant authority.  Approve creates the immutable snapshot.

ALTER TABLE justification
    DROP CONSTRAINT IF EXISTS justification_belief_id_belief_status_fkey;

ALTER TABLE justification
    ADD CONSTRAINT justification_belief_fk
        FOREIGN KEY (belief_id, belief_status)
        REFERENCES belief (id, status)
        ON UPDATE CASCADE;
