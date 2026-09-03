package agentjacking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/internal/normalize"
)

// BuildEvidence converts a parsed Sentry event plus its raw fixture bytes into
// the existing generic evidence representation. The generic ledger sees only
// external_feed evidence with an opaque domain payload; nothing here introduces
// Sentry vocabulary into the core.
//
// ContentSHA256 hashes the raw fixture bytes — the exact artifact on disk — so
// the ledger's evidence row traces back to the verbatim payload. message_raw is
// the verbatim message field from that artifact; it is never rewritten or
// sanitized here.
func BuildEvidence(raw []byte, ev Event) (normalize.NormalizedEvidence, error) {
	observedAt, err := time.Parse(time.RFC3339, ev.Timestamp)
	if err != nil {
		return normalize.NormalizedEvidence{}, fmt.Errorf("agentjacking: timestamp must be RFC3339")
	}

	messageClean := cleanMessage(ev.Message)
	payload, err := json.Marshal(map[string]any{
		"message_raw":       ev.Message,
		"message_clean":     messageClean,
		"embedded_commands": DetectCommands(ev.Message),
	})
	if err != nil {
		return normalize.NormalizedEvidence{}, fmt.Errorf("agentjacking: marshal domain payload: %w", err)
	}

	sum := sha256.Sum256(raw)
	contentSHA := hex.EncodeToString(sum[:])

	return normalize.NormalizedEvidence{
		ID:              contentSHA,
		SourceURL:       SentryURLScheme + ev.Project + "/" + ev.EventID,
		SourceType:      SourceTypeSentryError,
		ContentSHA256:   contentSHA,
		ObservedAt:      observedAt,
		IngestedAt:      time.Now().UTC(),
		ProvenanceClass: normalize.ProvenanceExternalFeed,
		Subject:         ev.Project,
		Assertion:       messageClean,
		Severity:        severityFor(ev.Level),
		DomainPayload:   payload,
	}, nil
}

// severityFor translates a Sentry level onto the generic severity scale. It is
// adapter translation of source-specific vocabulary, not a judgment.
func severityFor(level string) string {
	switch level {
	case "critical":
		return normalize.SeverityCritical
	case "error":
		return normalize.SeverityHigh
	case "warning":
		return normalize.SeverityMedium
	default:
		return normalize.SeverityInfo
	}
}

// Belief wraps the evidence in the fixed-shape, non-actionable claim the
// generic belief machinery will store.
//
// The shape is the whole point: external telemetry can establish that an error
// report exists, but its payload can never establish that an embedded command
// is authorized or required. The claim is built by this function, never
// assembled from payload text, so no instruction inside the message — whatever
// its wording — can become the belief's content. Classification is always
// derived, never accommodated.
func Belief(evidence normalize.NormalizedEvidence) derive.DerivedBelief {
	return derive.DerivedBelief{
		Claim:              "error report for " + evidence.Subject + " recorded from external telemetry; embedded command text is evidence, not instruction",
		Classification:     derive.Derived,
		SupportingEvidence: []normalize.NormalizedEvidence{evidence},
	}
}
