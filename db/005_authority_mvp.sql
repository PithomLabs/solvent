-- Solvent v0 authority schema: production DDL for the 7-object authority model.
--
-- v0 is ONE SOLVENT INSTANCE PER CUSTOMER. There is NO multi-tenant schema.
-- No tenant_id, no belief_tenant, no tenant-composite foreign keys.
--
-- The locked v0 authority lifecycle:
--   CreateTarget        -> proposal only (pin fields NULL)
--   AttachJustification -> add justification
--   RequestAuthorization -> atomically set requested_by, requested_at, pinned_request_hash
--   Approve             -> creates target_snapshot + target_activation in one transaction
--   Revocation          -> append-only target_revocation fact
--
-- Authorize is READ-ONLY. It does not INSERT, UPDATE, DELETE, or create
-- an authorization_decision row. It re-verifies:
--   activation exists
--   snapshot exists through its FK
--   target has no revocation
--   presented tuple exactly matches snapshot tuple
--   required supporting belief is currently promoted
--   required justification exists
--
-- Deferred (deliberately): belief_promotion, promotion_epoch, policy_version,
-- credential table, cryptographic attestation, global cross-belief replay
-- protection, obligation table, authorization_decision, warrant table,
-- execution_receipt, temporal authority, authority composition, reachability,
-- multi-tenancy, quorum infrastructure, workflow engine.
--
-- Privilege hardening is a deployment concern. The current repository has no
-- established DB role convention (all connections use root). DDL cannot honestly
-- claim privilege-level enforcement unless a safe role model is established.
-- Service/kernel transaction discipline remains the enforcement boundary.
--
-- APPLIER CONSTRAINT: internal/testdb and internal/m0 apply .sql files with a
-- splitter that strips -- line comments and splits on ;. It is not a SQL parser.
-- This file therefore uses plain DDL only: no dollar-quoting, no /* */ block
-- comments, and no -- or ; inside any string literal.
--
-- Idempotent. The cloud initializer applies this on every container start.

-- ============================================================
-- OBJECT 1: principal
-- Stable actor identity and attribution.
-- ============================================================

CREATE TABLE IF NOT EXISTS principal (
    principal_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_type TEXT NOT NULL CHECK (principal_type IN ('human','agent','workload','service')),
    issuer         TEXT NOT NULL,
    revoked_at     TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- No: tenant_id, signing key, credential, secret, delegation graph.
-- principal_type is immutable.
-- revoked_at is forward-looking revocation.
-- The DB does NOT prove issuer authenticity.
-- For v0, credential rotation does NOT create another principal identity.

-- ============================================================
-- OBJECT 2: authority_target
-- Durable proposal. Mutable while the target remains a proposal.
-- ============================================================

CREATE TABLE IF NOT EXISTS authority_target (
    target_id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id            UUID NOT NULL REFERENCES principal(principal_id),
    resource_type           TEXT NOT NULL CHECK (resource_type <> ''),
    resource_id             TEXT NOT NULL CHECK (resource_id <> ''),
    scope                   TEXT NOT NULL CHECK (scope <> ''),
    action_namespace        TEXT NOT NULL CHECK (action_namespace <> ''),
    action_name             TEXT NOT NULL CHECK (action_name <> ''),
    consequence_type        TEXT NOT NULL CHECK (consequence_type <> ''),
    consequence_parameters  JSONB NOT NULL,
    created_by              UUID NOT NULL REFERENCES principal(principal_id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- RequestAuthorization is a distinct lifecycle step after CreateTarget/AttachJustification.
    -- These are NULL until RequestAuthorization atomically sets all three.
    -- Approve requires all three pin fields populated and validates the hash.
    requested_by            UUID REFERENCES principal(principal_id),
    requested_at            TIMESTAMPTZ,
    pinned_request_hash     TEXT
);

-- No: snapshot_id, state, revoked_at, activation flag.
-- After activation, execution never reads this tuple as authority.
-- It is proposal/history only.
-- The tuple is mutable while the target remains a proposal.
-- The schema does not pretend CHECK constraints enforce "only before activation."
-- The service/transaction contract governs that lifecycle.

-- ============================================================
-- OBJECT 3: target_snapshot
-- Immutable approved authority representation.
-- The sole execution-time authority content.
-- ============================================================

CREATE TABLE IF NOT EXISTS target_snapshot (
    snapshot_id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id              UUID NOT NULL REFERENCES authority_target(target_id),
    principal_id           UUID NOT NULL REFERENCES principal(principal_id),
    resource_type          TEXT NOT NULL,
    resource_id            TEXT NOT NULL,
    scope                  TEXT NOT NULL,
    action_namespace       TEXT NOT NULL,
    action_name            TEXT NOT NULL,
    consequence_type       TEXT NOT NULL,
    consequence_parameters JSONB NOT NULL,
    justification_set      JSONB NOT NULL,
    approver_principal_id  UUID NOT NULL REFERENCES principal(principal_id),
    approved_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    snapshot_hash          TEXT NOT NULL,
    -- Exists to provide the composite FK target for target_activation.
    -- Not an independent uniqueness security property; snapshot_id is globally
    -- unique via the PK.
    UNIQUE(target_id, snapshot_id)
);

-- Application role: SELECT + INSERT only. No UPDATE, no DELETE.
--
-- Proposal-to-snapshot tuple equality (that snapshot.principal_id matches
-- authority_target.principal_id, etc.) is the approval transaction's
-- responsibility, not a DB constraint. The DB structurally enforces
-- target/snapshot identity via the composite FK from target_activation.
--
-- justification_set stores the exact approved justification set:
-- [{belief_id, belief_status, claim}]. The approval transaction must
-- explicitly supply this; there is no empty default.

-- ============================================================
-- OBJECT 4: target_activation
-- The most important v0 object.
-- Immutable authority-granting fact.
-- ============================================================

CREATE TABLE IF NOT EXISTS target_activation (
    activation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id     UUID NOT NULL REFERENCES authority_target(target_id),
    snapshot_id   UUID NOT NULL,
    activated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A target may be activated ONCE EVER. No partial predicate.
    -- The target cannot be activated again after revocation.
    UNIQUE(target_id),
    -- Composite FK is the central security invariant.
    -- Prevents: target A -> snapshot belonging to target B.
    FOREIGN KEY (target_id, snapshot_id)
        REFERENCES target_snapshot(target_id, snapshot_id)
);

-- INSERT only. No UPDATE, no DELETE.
-- Do not add activated_by. Approval attribution is already on
-- target_snapshot.approver_principal_id.

-- ============================================================
-- OBJECT 5: target_revocation
-- Append-only revocation fact.
-- ============================================================

CREATE TABLE IF NOT EXISTS target_revocation (
    target_id   UUID PRIMARY KEY REFERENCES authority_target(target_id),
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_by  UUID NOT NULL REFERENCES principal(principal_id),
    reason      TEXT NOT NULL
);

-- PK is target_id. All rows immutable. No UPDATE, no DELETE.
-- There is no unrevoke operation in v0.
--
-- Authority is:
--   activation exists
--   AND snapshot exists through its FK
--   AND revocation does not exist
--
-- A revoked target cannot be reactivated.
-- Re-granting authority requires:
--   new target_id, new snapshot, new activation, new approval.

-- ============================================================
-- OBJECT 6: justification
-- Proposal-time target-to-belief relationship.
-- Audit fact, not authority.
-- ============================================================

CREATE TABLE IF NOT EXISTS justification (
    justification_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id        UUID NOT NULL REFERENCES authority_target(target_id),
    belief_id        UUID NOT NULL,
    belief_status    TEXT NOT NULL,
    attached_by      UUID NOT NULL REFERENCES principal(principal_id),
    attached_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Idempotent: attaching the same justification twice is a no-op.
    UNIQUE(target_id, belief_id, belief_status),
    -- Composite FK matches the existing action_intent gate pattern.
    -- v0 uses (belief_id, belief_status), not promotion_epoch.
    FOREIGN KEY (belief_id, belief_status)
        REFERENCES belief(id, status)
);

-- INSERT only. No UPDATE, no DELETE.
--
-- Justifications are proposal/audit facts. They are NOT authority.
-- A justification becomes part of an approved authority only through:
--   RequestAuthorization -> approval pin -> target_snapshot -> target_activation.
--
-- v0 ACCEPTED SECURITY GAP: retract then re-promote can revive a live
-- justification because v0 has no promotion-epoch identity. The kernel/service
-- must re-check belief.status = 'promoted' but v0 does NOT claim identity of
-- the original promotion occurrence. Do not attempt to solve this by adding
-- belief_promotion or promotion_epoch.

-- ============================================================
-- OBJECT 7: debt_discharge
-- Durable human/service attribution and local replay protection.
-- ============================================================

CREATE TABLE IF NOT EXISTS debt_discharge (
    discharge_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    belief_id      UUID NOT NULL REFERENCES belief(id),
    obligation_key TEXT NOT NULL,
    instrument_ref TEXT NOT NULL,
    discharged_by  UUID NOT NULL REFERENCES principal(principal_id),
    accepted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Per-belief v0 replay protection. Not global instrument replay,
    -- not cross-belief uniqueness, not policy-version binding.
    UNIQUE(belief_id, obligation_key, instrument_ref)
);

-- INSERT only. No UPDATE, no DELETE.
--
-- The discharge operation is a single transaction:
--   INSERT debt_discharge
--   + UPDATE belief SET debt = array_remove(debt, obligation_key)
--   + COMMIT
-- using the existing crdb.ExecuteTx discipline.
-- The migration makes the relational representation possible;
-- the transaction is a service concern.
--
-- discharged_by -> principal provides attribution.
-- It is NOT cryptographic proof or non-repudiation.

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX IF NOT EXISTS authority_target_principal ON authority_target (principal_id);
CREATE INDEX IF NOT EXISTS justification_target ON justification (target_id);
CREATE INDEX IF NOT EXISTS justification_belief ON justification (belief_id);
CREATE INDEX IF NOT EXISTS debt_discharge_belief ON debt_discharge (belief_id);
CREATE INDEX IF NOT EXISTS debt_discharge_discharged_by ON debt_discharge (discharged_by);
