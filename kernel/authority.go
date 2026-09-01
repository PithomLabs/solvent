package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	Allowed bool
	Reason  string
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

// Authorize is READ-ONLY. It verifies existing authority without creating
// authority. It reads target_activation, target_snapshot, and the absence of
// target_revocation, then compares the presented tuple against the snapshot.
//
// Authorize performs zero writes against the authority tables.
func (s *Store) Authorize(ctx context.Context, targetID string, tuple AuthorityTuple) (AuthorizeResult, error) {
	var result AuthorizeResult

	err := crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
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
			result = AuthorizeResult{Allowed: false, Reason: "no activation or revocation exists"}
			return nil
		}

		// Compare presented tuple field-by-field with snapshot.
		if tuple.PrincipalID != snapPrincipalID {
			result = AuthorizeResult{Allowed: false, Reason: "principal mismatch"}
			return nil
		}
		if tuple.ResourceType != snapResourceType {
			result = AuthorizeResult{Allowed: false, Reason: "resource_type mismatch"}
			return nil
		}
		if tuple.ResourceID != snapResourceID {
			result = AuthorizeResult{Allowed: false, Reason: "resource_id mismatch"}
			return nil
		}
		if tuple.Scope != snapScope {
			result = AuthorizeResult{Allowed: false, Reason: "scope mismatch"}
			return nil
		}
		if tuple.ActionNamespace != snapActionNamespace {
			result = AuthorizeResult{Allowed: false, Reason: "action_namespace mismatch"}
			return nil
		}
		if tuple.ActionName != snapActionName {
			result = AuthorizeResult{Allowed: false, Reason: "action_name mismatch"}
			return nil
		}
		if tuple.ConsequenceType != snapConsequenceType {
			result = AuthorizeResult{Allowed: false, Reason: "consequence_type mismatch"}
			return nil
		}
		if !jsonEqual(tuple.ConsequenceParameters, snapConsequenceParams) {
			result = AuthorizeResult{Allowed: false, Reason: "consequence_parameters mismatch"}
			return nil
		}

		// Verify each justification's belief is currently promoted.
		var justs []struct {
			BeliefID     string `json:"belief_id"`
			BeliefStatus string `json:"belief_status"`
		}
		if err := json.Unmarshal(justSetJSON, &justs); err != nil {
			result = AuthorizeResult{Allowed: false, Reason: "justification_set parse failure"}
			return nil
		}
		for _, j := range justs {
			var status string
			if err := tx.QueryRowContext(ctx,
				`SELECT status FROM belief WHERE id = $1::UUID`, j.BeliefID).
				Scan(&status); err != nil {
				result = AuthorizeResult{Allowed: false, Reason: "belief not found: " + j.BeliefID}
				return nil
			}
			if status != "promoted" {
				result = AuthorizeResult{Allowed: false, Reason: "belief " + j.BeliefID + " not promoted"}
				return nil
			}
		}

		result = AuthorizeResult{Allowed: true, Reason: ""}
		return nil
	})
	if err != nil {
		return AuthorizeResult{}, err
	}
	return result, nil
}

// RevokeTarget inserts a target_revocation row. It checks that the target exists,
// has an activation, and has not already been revoked.
func (s *Store) RevokeTarget(ctx context.Context, targetID, revokedBy, reason string) error {
	return crdb.ExecuteTx(ctx, s.db, nil, func(tx *sql.Tx) error {
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
