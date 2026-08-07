// Package derive is the belief derivation engine.
//
// It consumes NormalizedEvidence from the normalize package and produces
// DerivedBelief values — pure semantic outputs describing what beliefs
// logically follow from the evidence.
//
// This package is domain-agnostic. It derives facts; Wave 3 decides
// what to do with them.
package derive

import "github.com/PithomLabs/solvent/internal/normalize"

// Classification constants identify the provenance of a derived belief.
const (
	Derived      = "derived"
	Accommodated = "accommodated"
)

// DerivedBelief describes a belief derived from evidence.
// Pure semantic output — no kernel operations, no debt, no IDs.
// Wave 3 decides what to do with these results.
type DerivedBelief struct {
	// Claim is a structured factual statement about the world.
	Claim string

	// Classification identifies the belief's provenance.
	// Derived: new belief from fresh evidence.
	// Accommodated: evidence merged into or strengthening an existing belief.
	Classification string

	// SupportingEvidence is the evidence that supports this belief.
	SupportingEvidence []normalize.NormalizedEvidence

	// Contradicts is evidence that contradicts a prior belief.
	// Non-empty only for contradiction results.
	Contradicts []normalize.NormalizedEvidence
}
