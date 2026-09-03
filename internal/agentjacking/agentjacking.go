// Package agentjacking is a boundary adapter that demonstrates the Agentjacking
// attack class (attacker-controlled telemetry, e.g. a poisoned Sentry error,
// carrying an embedded "resolution" command) against Solvent's generic
// evidence/belief/debt machinery.
//
// Boundary rules this package lives by:
//
//   - Sentry parsing, command observation, and demo fixture handling stay here.
//     The generic core — normalize, derive, belief, kernel, schema — never
//     learns that this evidence came from Sentry or any telemetry platform.
//   - The adapter emits only existing generic types: normalize.NormalizedEvidence
//     and derive.DerivedBelief.
//   - Detection of command-like text in the payload is observational audit
//     metadata, not a security control. The defense is structural: payload text
//     is ingested as external_feed evidence and retires zero debt, so the belief
//     can never be promoted and can never warrant a live action — regardless of
//     whether the embedded instruction was recognized.
//
// The generic pipeline does not switch on this package's source type.
package agentjacking

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// SourceTypeSentinelError is the adapter-level source type written into
// NormalizedEvidence.SourceType. It is a plain string, not a normalize
// constant: the generic packages must not dispatch on it, and no
// DebtMapping entry exists for it, so Sentry evidence retires zero debt.
const SourceTypeSentryError = "sentry_error"

// SentryURLScheme labels the synthetic source URL. No network call is ever
// made; the URL is a traceable name for the fixture artifact, whose exact
// bytes the evidence row's content_sha256 hashes.
const SentryURLScheme = "sentry://"

// Event is the subset of a Sentry error event the adapter understands.
type Event struct {
	EventID   string            `json:"event_id"`
	Project   string            `json:"project"`
	Level     string            `json:"level"`
	Timestamp string            `json:"timestamp"`
	Logger    string            `json:"logger"`
	Message   string            `json:"message"`
	Tags      map[string]string `json:"tags"`
	Extra     map[string]any    `json:"extra"`
}

// Parse decodes a raw Sentry event fixture. The raw bytes are the forensic
// artifact; callers keep them and pass them to BuildEvidence separately.
func Parse(raw []byte) (Event, error) {
	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return Event{}, fmt.Errorf("agentjacking: parse sentry event: %w", err)
	}
	if ev.Project == "" {
		return Event{}, fmt.Errorf("agentjacking: sentry event: project is required")
	}
	if ev.Message == "" {
		return Event{}, fmt.Errorf("agentjacking: sentry event: message is required")
	}
	if _, err := time.Parse(time.RFC3339, ev.Timestamp); err != nil {
		return Event{}, fmt.Errorf("agentjacking: sentry event: timestamp must be RFC3339")
	}
	return ev, nil
}

// commandPattern is observational only. It names command-looking tokens that a
// reader might mistake for remediation guidance. A payload whose instruction
// avoids these patterns is not therefore trusted — the structural defense in
// the debt/promotion gate does not depend on this detection at all.
var commandPatterns = []struct {
	name    string
	matcher *regexp.Regexp
}{
	{"npx", regexp.MustCompile(`\bnpx\b`)},
	{"npm exec", regexp.MustCompile(`\bnpm exec\b`)},
	{"pip install", regexp.MustCompile(`\bpip install\b`)},
	{"curl | sh", regexp.MustCompile(`\bcurl\b[^|\n]*\|\s*(ba)?sh\b`)},
	{"wget", regexp.MustCompile(`\bwget\b`)},
}

// DetectCommands returns the command-like tokens found in the message. The
// result is audit metadata for the human reviewing the evidence; it never
// gates anything, and an empty result means nothing about trustworthiness.
func DetectCommands(message string) []string {
	var found []string
	for _, p := range commandPatterns {
		if p.matcher.MatchString(message) {
			found = append(found, p.name)
		}
	}
	sort.Strings(found)
	return found
}
