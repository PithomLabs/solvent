package kernel

import (
	"errors"
	"fmt"
)

// Contract §4 requires these sentinels. They name a refusal the database made;
// they never make the decision themselves.
var (
	// ErrPromotionBlocked reports that the schema refused a promotion because the
	// belief still carries debt or final-truth language (invariants I-1, I-2).
	ErrPromotionBlocked = errors.New("promotion blocked: open debt or final-truth language")

	// ErrActionOnUnpromoted reports that the composite FK refused an action intent
	// because the cited belief is not currently promoted (invariant I-3).
	ErrActionOnUnpromoted = errors.New("action refused: belief is not promoted")
)

// SQLSTATE codes this package classifies on. Contract §4: classification MUST use
// SQLSTATE codes, never substring matching on error text.
const (
	sqlStateCheckViolation = "23514"
	sqlStateFKViolation    = "23503"
)

// sqlStater is satisfied by *pgconn.PgError (and by lib/pq's error type). Matching
// on this interface rather than on a concrete type is what keeps the kernel free of
// any driver import: the driver is chosen at the cmd/ edge, not here.
type sqlStater interface{ SQLState() string }

// sqlState returns the SQLSTATE carried by err, or "" if err carries none.
func sqlState(err error) string {
	var s sqlStater
	if errors.As(err, &s) {
		return s.SQLState()
	}
	return ""
}

// wrapIf returns err wrapped in sentinel when err carries the given SQLSTATE, and
// err untouched otherwise.
//
// The wrap deliberately preserves the driver error rather than replacing it, so
// both errors.Is(err, sentinel) and errors.As(err, &pgErr) succeed — §4's sentinel
// contract is met while the raw SQLSTATE and constraint name stay reachable.
//
// Taking the code as a parameter makes the ordering structural rather than a
// convention: nothing is ever wrapped before it has been classified, so a retryable
// 40001 always reaches crdb.ExecuteTx's retry logic unmasked.
func wrapIf(code string, sentinel error, err error) error {
	if err == nil || sqlState(err) != code {
		return err
	}
	return fmt.Errorf("%w: %w", sentinel, err)
}
