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
	EnterBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string) (string, error)
	AddEvidence(context.Context, string, string, string, string, string) error
	RetireDebt(context.Context, string, string, string) error
	Promote(context.Context, string, string) error
	IntentOnPromoted(context.Context, string, string, string) error
	RetractCascade(context.Context, string, string) (int, error)
	AuditLiveOnNonPromoted(context.Context, string) (int, error)
	EnsureBelief(ctx context.Context, scenarioID, claim string, ct ClaimType, initialDebt []string) (string, error)

	// Authority lifecycle.
	CreatePrincipal(context.Context, string, string) (string, error)
	RevokePrincipal(context.Context, string) error
	CreateTarget(context.Context, string, string, string, string, string, string, string, []byte, string) (string, error)
	AttachJustification(context.Context, string, string, string, string) error
	RequestAuthorization(context.Context, string, string) error
	Approve(context.Context, string, string) error
	Authorize(context.Context, string, AuthorityTuple) (AuthorizeResult, error)
	AuthorizeAndCreateIntent(context.Context, string, AuthorityTuple, string, string, string) (AuthorizeResult, error)
	RevokeTarget(context.Context, string, string, string) error
	Discharge(context.Context, string, string, string, string, string) error
	CompleteIntent(context.Context, string, string) error
	ClaimIntent(context.Context, string, string, string, string, string, string) error
	RollbackClaim(context.Context, string, string) error
	CancelIntent(context.Context, string, string) error
}

var (
	_ Contract             = (*Store)(nil)
	_ func(*sql.DB) *Store = New
	_ ClaimType            = Derived
	_ ClaimType            = Accommodated
	_ ClaimType            = Postulated
)
