// This file defines the provider outcome contract shared between adapters
// and the service layer. The adapter classifies provider errors into a
// generic ProviderOutcome; the service maps it to kernel transitions.
//
// The service must NOT inspect provider-specific HTTP semantics directly.
// Classification is adapter-specific, not service-specific (CI-9).
package github

import "errors"

// ProviderOutcome classifies the result of a provider invocation.
type ProviderOutcome int

const (
	// ProviderAccepted means the provider definitively accepted the request.
	// Transition: executing → executed (via CompleteIntent).
	ProviderAccepted ProviderOutcome = iota

	// ProviderRejected means the provider definitively rejected the request.
	// The provider did NOT accept. Transition: executing → live (via RollbackClaim).
	// Retry is safe.
	ProviderRejected

	// ProviderAmbiguous means the provider outcome is unknown.
	// The provider MAY have accepted. Transition: none (intent stays executing).
	// Retry is NOT safe — reconciliation required.
	ProviderAmbiguous
)

// ProviderError wraps a provider failure with a classified outcome.
type ProviderError struct {
	Err    error
	Outcome ProviderOutcome
}

func (e *ProviderError) Error() string { return e.Err.Error() }
func (e *ProviderError) Unwrap() error { return e.Err }

// ProviderOutcomeCode returns the numeric outcome code.
// This enables the service layer to classify without importing this package.
func (e *ProviderError) ProviderOutcomeCode() int { return int(e.Outcome) }

// AsProviderError extracts a ProviderError from err if present.
func AsProviderError(err error) (*ProviderError, bool) {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}
