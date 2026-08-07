package kernel

import (
	"context"
	"database/sql"
)

// Contract is IMPLEMENTATION_CONTRACT.md §4, transcribed as Go types.
//
// Its only purpose is the assertions below. Because *Store must satisfy this
// interface, a signature that drifts from §4 fails the build rather than a review —
// which is the mechanical form of §7 M1's "Every §4 function exists with the stated
// signature."
//
// Not versioned: this is the contract, and its history lives in git.
type Contract interface {
	EnterBelief(context.Context, string, string, ClaimType) (string, error)
	AddEvidence(context.Context, string, string, string, string, string) error
	RetireDebt(context.Context, string, string) error
	Promote(context.Context, string) error
	IntentOnPromoted(context.Context, string, string, string) error
	RetractCascade(context.Context, string, string) (int, error)
	AuditLiveOnNonPromoted(context.Context, string) (int, error)
}

var (
	_ Contract             = (*Store)(nil)
	_ func(*sql.DB) *Store = New
	_ []string             = FullDebt
	_ ClaimType            = Derived
	_ ClaimType            = Accommodated
	_ ClaimType            = Postulated
)
