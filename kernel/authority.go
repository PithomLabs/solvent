package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
)

// AuthorityTuple represents the presented execution-time tuple for Authorize.
type AuthorityTuple struct {
	PrincipalID           string
	ResourceType          string
	ResourceID            string
	Scope                 string
	ActionNamespace       string
	ActionName            string
	ConsequenceType       string
	ConsequenceParameters []byte
}

// AuthorizeResult is the read-only authority verification outcome.
type AuthorizeResult struct {
	Allowed               bool
	Reason                string
	IntentState           string // populated by AuthorizeAndCreateIntent on success
	ConsequenceParameters []byte // populated by Authorize — the snapshot's approved params
}

// authorityTarget is the internal representation of an authority_target row.
type authorityTarget struct {
	PrincipalID           string
	ResourceType          string
	ResourceID            string
	Scope                 string
	ActionNamespace       string
	ActionName            string
	ConsequenceType       string
	ConsequenceParameters []byte
	RequestedBy           sql.NullString
	RequestedAt           sql.NullTime
	PinnedRequestHash     sql.NullString
}

// justification is the internal representation of a justification row.
type justification struct {
	BeliefID     string
	BeliefStatus string
}

// CreatePrincipal inserts a new principal and returns its ID.
//
// Not idempotent under client-timeout retries: a retry after a successful
// commit but before the client receives the response creates a duplicate
// principal. Callers must handle duplicate creation gracefully. This is an
// accepted v0 limitation. Fixing it requires a unique constraint on
// (principal_type, issuer), which changes the frozen schema.
func (s *Store) CreatePrincipal(ctx context.Context, principalType, issuer string) (string, error) {
	var id string
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, sqlCreatePrincipal,
			principalType, issuer,
		).Scan(&id)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// RevokePrincipal sets revoked_at on a principal. It is idempotent: revoking an
// already-revoked principal is a no-op (zero rows affected).
func (s *Store) RevokePrincipal(ctx context.Context, principalID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlRevokePrincipal, principalID)
		return err
	})
}

// CreateTarget inserts an authority_target proposal. Pin fields (requested_by,
// requested_at, pinned_request_hash) are left NULL. No authority is granted.
//
// Not idempotent under client-timeout retries: a retry after a successful
// commit but before the client receives the response creates a duplicate
// target. Callers must handle duplicate creation gracefully. This is an
// accepted v0 limitation. Fixing it requires a unique constraint on the
// business key tuple, which changes the frozen schema.
func (s *Store) CreateTarget(ctx context.Context, principalID, resourceType, resourceID, scope, actionNamespace, actionName, consequenceType string, consequenceParameters []byte, createdBy string) (string, error) {
	var id string
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, sqlCreateTarget,
			principalID, resourceType, resourceID, scope,
			actionNamespace, actionName, consequenceType, consequenceParameters,
			createdBy,
		).Scan(&id)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// AttachJustification links a belief to a target proposal. It is idempotent:
// the UNIQUE(target_id, belief_id, belief_status) constraint makes duplicate
// attachment a no-op (ON CONFLICT DO NOTHING).
//
// The target must not be activated. Both AttachJustification and Approve lock
// the authority_target row with SELECT ... FOR UPDATE to serialize the race.
func (s *Store) AttachJustification(ctx context.Context, targetID, beliefID, beliefStatus, attachedBy string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Lock the target to serialize against Approve.
		var locked string
		if err := tx.QueryRowContext(ctx, sqlAttachJustificationLock, targetID).Scan(&locked); err != nil {
			return ErrTargetNotFound
		}

		// Verify not already activated.
		var cnt int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM target_activation WHERE target_id = $1::UUID`,
			targetID).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			return ErrAlreadyActivated
		}

		_, err := tx.ExecContext(ctx, sqlAttachJustification,
			targetID, beliefID, beliefStatus, attachedBy)
		return err
	})
}

// RequestAuthorization atomically sets the request pin on a target proposal.
// It requires no existing activation and no existing revocation.
func (s *Store) RequestAuthorization(ctx context.Context, targetID, requestedBy string) error {
	// Build the canonical hash first, outside the transaction.
	// The hash must be computed from the current proposal + justification set.
	var hash string

	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// Verify not activated and not revoked.
		var cnt int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM target_activation WHERE target_id = $1::UUID`,
			targetID).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			return ErrAlreadyActivated
		}
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM target_revocation WHERE target_id = $1::UUID`,
			targetID).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			return ErrAlreadyRevoked
		}

		// Read the current proposal.
		var t authorityTarget
		if err := tx.QueryRowContext(ctx,
			`SELECT principal_id, resource_type, resource_id, scope,
			        action_namespace, action_name, consequence_type, consequence_parameters
			FROM authority_target WHERE target_id = $1::UUID`,
			targetID).Scan(
			&t.PrincipalID, &t.ResourceType, &t.ResourceID, &t.Scope,
			&t.ActionNamespace, &t.ActionName, &t.ConsequenceType, &t.ConsequenceParameters,
		); err != nil {
			return ErrTargetNotFound
		}

		// Read all current justifications.
		rows, err := tx.QueryContext(ctx, sqlApproveReadJustifications, targetID)
		if err != nil {
			return err
		}
		var justs []justification
		for rows.Next() {
			var j justification
			if err := rows.Scan(&j.BeliefID, &j.BeliefStatus); err != nil {
				_ = rows.Close()
				return err
			}
			justs = append(justs, j)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		_ = rows.Close()

		hash = computeRequestHash(&t, justs)

		res, err := tx.ExecContext(ctx, sqlRequestAuthorization, targetID, requestedBy, hash)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			// Diagnose the actual cause. Precedence: not found > revoked > activated.
			var exists, revoked, activated bool
			if diagErr := tx.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM authority_target WHERE target_id = $1::UUID),
				        EXISTS(SELECT 1 FROM target_revocation WHERE target_id = $1::UUID),
				        EXISTS(SELECT 1 FROM target_activation WHERE target_id = $1::UUID)`,
				targetID).Scan(&exists, &revoked, &activated); diagErr != nil {
				return diagErr
			}
			if !exists {
				return ErrTargetNotFound
			}
			if revoked {
				return ErrAlreadyRevoked
			}
			if activated {
				return ErrAlreadyActivated
			}
			// Target exists but is neither activated nor revoked — the WHERE
			// NOT EXISTS guards must have blocked. This should not happen for
			// a proposed target, but return not-found as the safe fallback.
			return ErrTargetNotFound
		}
		return nil
	})
}

// Approve is the sole authority-creating operation. It runs as one SERIALIZABLE
// transaction that atomically creates target_snapshot and target_activation.
//
// Inside the transaction:
//  1. Read target proposal (FOR UPDATE)
//  2. Confirm pin fields populated
//  3. Read current justifications
//  4. Recompute hash, compare with pinned_request_hash
//  5. Validate every referenced belief is currently promoted
//  6. Validate approver principal exists and is not revoked
//  7. INSERT target_snapshot
//  8. INSERT target_activation
//  9. Commit
func (s *Store) Approve(ctx context.Context, targetID, approverPrincipalID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// 1. Read target proposal and lock it.
		var t authorityTarget
		if err := tx.QueryRowContext(ctx, sqlApproveReadTarget, targetID).Scan(
			&t.PrincipalID, &t.ResourceType, &t.ResourceID, &t.Scope,
			&t.ActionNamespace, &t.ActionName, &t.ConsequenceType, &t.ConsequenceParameters,
			&t.RequestedBy, &t.RequestedAt, &t.PinnedRequestHash,
		); err != nil {
			return ErrTargetNotFound
		}

		// 2. Confirm pin fields are populated.
		if !t.RequestedBy.Valid || !t.RequestedAt.Valid || !t.PinnedRequestHash.Valid {
			return ErrAuthorizationMissing
		}

		// 3. Read current justifications.
		rows, err := tx.QueryContext(ctx, sqlApproveReadJustifications, targetID)
		if err != nil {
			return err
		}
		var justs []justification
		for rows.Next() {
			var j justification
			if err := rows.Scan(&j.BeliefID, &j.BeliefStatus); err != nil {
				_ = rows.Close()
				return err
			}
			justs = append(justs, j)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		_ = rows.Close()

		if len(justs) == 0 {
			return ErrInvalidProposal
		}

		// 4. Recompute hash and compare.
		recomputed := computeRequestHash(&t, justs)
		if recomputed != t.PinnedRequestHash.String {
			return ErrApprovalPinMismatch
		}

		// 5. Validate every referenced belief is currently promoted.
		beliefIDs := make([]string, len(justs))
		for i, j := range justs {
			beliefIDs[i] = j.BeliefID
		}
		rows2, err := tx.QueryContext(ctx, sqlApproveReadBeliefs, beliefIDs)
		if err != nil {
			return err
		}
		promotedCount := 0
		for rows2.Next() {
			var id, status string
			if err := rows2.Scan(&id, &status); err != nil {
				_ = rows2.Close()
				return err
			}
			promotedCount++
		}
		if err := rows2.Err(); err != nil {
			return err
		}
		_ = rows2.Close()
		if promotedCount != len(justs) {
			return ErrBeliefNotPromoted
		}

		// 6. Validate approver principal exists and is not revoked.
		var approverRevoked sql.NullTime
		if err := tx.QueryRowContext(ctx,
			`SELECT revoked_at FROM principal WHERE principal_id = $1::UUID`,
			approverPrincipalID).Scan(&approverRevoked); err != nil {
			return ErrRevokedPrincipal
		}
		if approverRevoked.Valid {
			return ErrRevokedPrincipal
		}

		// Build the justification_set JSONB.
		justSet := buildJustificationSet(justs)
		justSetJSON, err := json.Marshal(justSet)
		if err != nil {
			return err
		}

		// 7. INSERT target_snapshot.
		var snapshotID string
		if err := tx.QueryRowContext(ctx, sqlApproveInsertSnapshot,
			targetID, t.PrincipalID, t.ResourceType, t.ResourceID, t.Scope,
			t.ActionNamespace, t.ActionName, t.ConsequenceType, t.ConsequenceParameters,
			justSetJSON, approverPrincipalID, recomputed,
		).Scan(&snapshotID); err != nil {
			return err
		}

		// 8. INSERT target_activation.
		if _, err := tx.ExecContext(ctx, sqlApproveInsertActivation,
			targetID, snapshotID); err != nil {
			return err
		}

		return nil
	})
}

// authorizeWithinTx evaluates current authority within an existing transaction.
// It is the single implementation of authority semantics used by both Authorize
// (standalone) and AuthorizeAndCreateIntent (composite).
//
// This function performs zero writes. It reads target_activation, target_snapshot,
// and the absence of target_revocation, then compares the presented tuple against
// the snapshot.
func authorizeWithinTx(ctx context.Context, tx *sql.Tx, targetID string, tuple AuthorityTuple) (AuthorizeResult, error) {
	// Resolve authority: activation + snapshot + no revocation.
	var (
		snapPrincipalID, snapResourceType, snapResourceID string
		snapScope, snapActionNamespace, snapActionName    string
		snapConsequenceType                               string
		snapConsequenceParams                             []byte
		justSetJSON                                       []byte
	)
	if err := tx.QueryRowContext(ctx, sqlAuthorizeResolve, targetID).Scan(
		&snapPrincipalID, &snapResourceType, &snapResourceID, &snapScope,
		&snapActionNamespace, &snapActionName, &snapConsequenceType, &snapConsequenceParams,
		&justSetJSON,
	); err != nil {
		return AuthorizeResult{Allowed: false, Reason: "no activation or revocation exists"}, nil
	}

	// Compare presented tuple field-by-field with snapshot.
	if tuple.PrincipalID != snapPrincipalID {
		return AuthorizeResult{Allowed: false, Reason: "principal mismatch"}, nil
	}
	if tuple.ResourceType != snapResourceType {
		return AuthorizeResult{Allowed: false, Reason: "resource_type mismatch"}, nil
	}
	if tuple.ResourceID != snapResourceID {
		return AuthorizeResult{Allowed: false, Reason: "resource_id mismatch"}, nil
	}
	if tuple.Scope != snapScope {
		return AuthorizeResult{Allowed: false, Reason: "scope mismatch"}, nil
	}
	if tuple.ActionNamespace != snapActionNamespace {
		return AuthorizeResult{Allowed: false, Reason: "action_namespace mismatch"}, nil
	}
	if tuple.ActionName != snapActionName {
		return AuthorizeResult{Allowed: false, Reason: "action_name mismatch"}, nil
	}
	if tuple.ConsequenceType != snapConsequenceType {
		return AuthorizeResult{Allowed: false, Reason: "consequence_type mismatch"}, nil
	}
	if !jsonEqual(tuple.ConsequenceParameters, snapConsequenceParams) {
		return AuthorizeResult{Allowed: false, Reason: "consequence_parameters mismatch"}, nil
	}

	// Verify each justification's belief is currently promoted.
	var justs []struct {
		BeliefID     string `json:"belief_id"`
		BeliefStatus string `json:"belief_status"`
	}
	if err := json.Unmarshal(justSetJSON, &justs); err != nil {
		return AuthorizeResult{Allowed: false, Reason: "justification_set parse failure"}, nil
	}
	for _, j := range justs {
		var status string
		if err := tx.QueryRowContext(ctx,
			`SELECT status FROM belief WHERE id = $1::UUID`, j.BeliefID).
			Scan(&status); err != nil {
			return AuthorizeResult{Allowed: false, Reason: "belief not found: " + j.BeliefID}, nil
		}
		if status != "promoted" {
			return AuthorizeResult{Allowed: false, Reason: "belief " + j.BeliefID + " not promoted"}, nil
		}
	}

	return AuthorizeResult{Allowed: true, Reason: "", ConsequenceParameters: snapConsequenceParams}, nil
}

// Authorize is READ-ONLY. It verifies existing authority without creating
// authority. It reads target_activation, target_snapshot, and the absence of
// target_revocation, then compares the presented tuple against the snapshot.
//
// Authorize performs zero writes against the authority tables.
func (s *Store) Authorize(ctx context.Context, targetID string, tuple AuthorityTuple) (AuthorizeResult, error) {
	var result AuthorizeResult
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var err error
		result, err = authorizeWithinTx(ctx, tx, targetID, tuple)
		return err
	})
	if err != nil {
		return AuthorizeResult{}, err
	}
	return result, nil
}

// AuthorizeAndCreateIntent evaluates current authority and creates a live
// action intent inside ONE SERIALIZABLE transaction.
//
// Security guarantee: both the authority evaluation and the intent creation
// are protected by a FOR UPDATE lock on authority_target, which serializes
// against a concurrent RevokeTarget. If a revocation is committed before the
// lock is acquired, the authority evaluation sees it and denies. If the lock
// is acquired first, the revocation blocks until the intent is committed.
// A live intent can never be committed on superseded authority.
//
// This method exists because the state transition "verify authority + create
// intent" has security semantics that cannot safely be split across independent
// transactions. The kernel is the sole authority oracle — this method does not
// introduce a second authority engine; it composes the existing authority
// evaluation with intent creation in one atomic boundary.
//
// The existing Authorize and IntentOnPromoted methods remain available for
// callers that do not need the composite guarantee.
func (s *Store) AuthorizeAndCreateIntent(
	ctx context.Context,
	targetID string,
	tuple AuthorityTuple,
	scenarioID, beliefID, action string,
) (AuthorizeResult, error) {
	var result AuthorizeResult
	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// LOCK: serialize against concurrent RevokeTarget.
		var locked string
		if err := tx.QueryRowContext(ctx,
			sqlAttachJustificationLock, targetID).Scan(&locked); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTargetNotFound
			}
			return err
		}

		// 1. Evaluate authority (reuses authorizeWithinTx — one authority implementation).
		var err error
		result, err = authorizeWithinTx(ctx, tx, targetID, tuple)
		if err != nil {
			return err
		}
		if !result.Allowed {
			return nil // denial is communicated via result, not error
		}

		// 2. Create intent (reuses createIntentWithinTx — one intent implementation).
		if err := createIntentWithinTx(ctx, tx, scenarioID, beliefID, action); err != nil {
			return err
		}

		result.IntentState = "live"
		return nil
	})
	if err != nil {
		return AuthorizeResult{}, err
	}
	return result, nil
}

// CompleteIntent transitions an executing intent to 'executed'. It is idempotent:
// completing an already-executed intent is a no-op (zero rows affected).
//
// This method is called by the service layer after a provider accepts an
// execution request. The intent must have been claimed by ClaimIntent
// and must be in 'executing' state.
func (s *Store) CompleteIntent(ctx context.Context, scenarioID, intentID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, sqlCompleteIntent, intentID, scenarioID)
		return err
	})
}

// ClaimIntent transitions a live intent to 'executing'. It is atomic:
// only one caller can successfully claim a given intent.
//
// Returns nil on success (state is now 'executing').
// Returns ErrIntentNotLive if the intent is not in 'live' state.
//
// This is the sole authoritative ownership gate for execution.
// The CAS predicate (WHERE state = 'live') ensures at most one concurrent
// claim succeeds per intent (CI-4).
func (s *Store) ClaimIntent(ctx context.Context, scenarioID, intentID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, sqlClaimIntent, intentID, scenarioID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrIntentNotLive
		}
		return nil
	})
}

// RollbackClaim transitions an executing intent back to 'live' after
// definitive provider rejection. Only valid when state is 'executing'.
//
// Returns nil on success (state is now 'live').
// Returns ErrNotExecuting if the intent is not in 'executing' state.
//
// Only called when the provider definitively rejects (e.g., 403 Forbidden,
// validation error, explicit rejection). NOT called for ambiguous failures
// (timeout, network error, lost response) — those leave intent as executing.
func (s *Store) RollbackClaim(ctx context.Context, scenarioID, intentID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, sqlRollbackClaim, intentID, scenarioID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotExecuting
		}
		return nil
	})
}

// CancelIntent transitions an executing intent to 'cancelled'.
// Only valid when state is 'executing'. Used by operator reconciliation
// when the operator decides not to retry.
//
// Returns nil on success (state is now 'cancelled').
// Returns ErrNotExecuting if the intent is not in 'executing' state.
func (s *Store) CancelIntent(ctx context.Context, scenarioID, intentID string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, sqlCancelIntent, intentID, scenarioID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotExecuting
		}
		return nil
	})
}

// RevokeTarget inserts a target_revocation row. It serializes against concurrent
// AuthorizeAndCreateIntent via FOR UPDATE lock on authority_target, then checks
// that the target exists, has an activation, and has not already been revoked.
func (s *Store) RevokeTarget(ctx context.Context, targetID, revokedBy, reason string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		// LOCK: serialize against concurrent AuthorizeAndCreateIntent.
		var locked string
		if err := tx.QueryRowContext(ctx,
			sqlAttachJustificationLock, targetID).Scan(&locked); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTargetNotFound
			}
			return err
		}

		// Verify target has an activation.
		var cnt int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM target_activation WHERE target_id = $1::UUID`,
			targetID).Scan(&cnt); err != nil {
			return err
		}
		if cnt == 0 {
			return ErrTargetNotActivated
		}

		// Verify not already revoked.
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM target_revocation WHERE target_id = $1::UUID`,
			targetID).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			return ErrAlreadyRevoked
		}

		_, err := tx.ExecContext(ctx, sqlRevokeTarget, targetID, revokedBy, reason)
		return wrapIf(sqlStateUniqueViolation, ErrAlreadyRevoked, err)
	})
}

// Discharge records a debt discharge and retires the debt item from the belief
// in one transaction. The UNIQUE(belief_id, obligation_key, instrument_ref)
// constraint provides per-belief replay protection.
func (s *Store) Discharge(ctx context.Context, beliefID, obligationKey, instrumentRef, dischargedBy string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, sqlDischargeInsert,
			beliefID, obligationKey, instrumentRef, dischargedBy); err != nil {
			return wrapIf(sqlStateUniqueViolation, ErrDuplicateDischarge, err)
		}
		_, err := tx.ExecContext(ctx, sqlDischargeRetireDebt, beliefID, obligationKey)
		return err
	})
}

// computeRequestHash produces a deterministic SHA-256 hash over the proposal
// tuple and the sorted justification identifiers. Null bytes separate fields to
// prevent ambiguity.
func computeRequestHash(target *authorityTarget, justs []justification) string {
	var buf bytes.Buffer

	// Proposal tuple in fixed order.
	buf.WriteString(target.PrincipalID)
	buf.WriteByte(0)
	buf.WriteString(target.ResourceType)
	buf.WriteByte(0)
	buf.WriteString(target.ResourceID)
	buf.WriteByte(0)
	buf.WriteString(target.Scope)
	buf.WriteByte(0)
	buf.WriteString(target.ActionNamespace)
	buf.WriteByte(0)
	buf.WriteString(target.ActionName)
	buf.WriteByte(0)
	buf.WriteString(target.ConsequenceType)
	buf.WriteByte(0)
	buf.Write(target.ConsequenceParameters)
	buf.WriteByte(0)

	// Sorted justifications.
	sorted := make([]justification, len(justs))
	copy(sorted, justs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].BeliefID != sorted[j].BeliefID {
			return sorted[i].BeliefID < sorted[j].BeliefID
		}
		return sorted[i].BeliefStatus < sorted[j].BeliefStatus
	})
	for _, j := range sorted {
		buf.WriteString(j.BeliefID)
		buf.WriteByte(0)
		buf.WriteString(j.BeliefStatus)
		buf.WriteByte(0)
	}

	hash := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(hash[:])
}

// justificationEntry is the JSON representation of a justification in the snapshot.
type justificationEntry struct {
	BeliefID     string `json:"belief_id"`
	BeliefStatus string `json:"belief_status"`
}

// buildJustificationSet constructs the JSONB justification_set for a snapshot.
func buildJustificationSet(justs []justification) []justificationEntry {
	out := make([]justificationEntry, len(justs))
	for i, j := range justs {
		out[i] = justificationEntry{
			BeliefID:     j.BeliefID,
			BeliefStatus: j.BeliefStatus,
		}
	}
	return out
}

// requestHash reads the current proposal + justification set and computes the
// canonical hash. Used by tests and by RequestAuthorization.
func (s *Store) requestHash(ctx context.Context, tx *sql.Tx, targetID string) (string, error) {
	var t authorityTarget
	if err := tx.QueryRowContext(ctx,
		`SELECT principal_id, resource_type, resource_id, scope,
		        action_namespace, action_name, consequence_type, consequence_parameters
		FROM authority_target WHERE target_id = $1::UUID`,
		targetID).Scan(
		&t.PrincipalID, &t.ResourceType, &t.ResourceID, &t.Scope,
		&t.ActionNamespace, &t.ActionName, &t.ConsequenceType, &t.ConsequenceParameters,
	); err != nil {
		return "", err
	}

	rows, err := tx.QueryContext(ctx, sqlApproveReadJustifications, targetID)
	if err != nil {
		return "", err
	}
	var justs []justification
	for rows.Next() {
		var j justification
		if err := rows.Scan(&j.BeliefID, &j.BeliefStatus); err != nil {
			_ = rows.Close()
			return "", err
		}
		justs = append(justs, j)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	_ = rows.Close()

	return computeRequestHash(&t, justs), nil
}

// targetState derives the lifecycle state from authority facts.
// This is a read-only helper, not an authority source.
func (s *Store) targetState(ctx context.Context, targetID string) (string, error) {
	var hasActivation, hasRevocation bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM target_activation WHERE target_id = $1::UUID)`,
		targetID).Scan(&hasActivation); err != nil {
		return "", err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM target_revocation WHERE target_id = $1::UUID)`,
		targetID).Scan(&hasRevocation); err != nil {
		return "", err
	}
	switch {
	case hasRevocation:
		return "revoked", nil
	case hasActivation:
		return "active", nil
	default:
		return "proposed", nil
	}
}

// suppressed unused import
var _ = fmt.Sprintf

// jsonEqual compares two JSON byte slices semantically, not byte-for-byte.
// JSONB serialization may normalize formatting (e.g., adding spaces after colons).
func jsonEqual(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	ma, _ := json.Marshal(va)
	mb, _ := json.Marshal(vb)
	return bytes.Equal(ma, mb)
}
