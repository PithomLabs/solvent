package agentjacking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/internal/normalize"
)

// TestA_GenericEvidenceRepresentation verifies that BuildEvidence produces a
// generic NormalizedEvidence with external_feed provenance, correct SHA over
// raw fixture bytes, byte-identical message_raw, stripped message_clean, and
// informational embedded_commands. The generic ledger sees only
// external_feed evidence with an opaque domain payload.
func TestA_GenericEvidenceRepresentation(t *testing.T) {
	raw, err := os.ReadFile("../../demo/agentjacking/fixtures/sentry_error.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ev, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	evidence, err := BuildEvidence(raw, ev)
	if err != nil {
		t.Fatalf("build evidence: %v", err)
	}

	// Provenance must be external_feed — telemetry can never be operator_asserted.
	if evidence.ProvenanceClass != normalize.ProvenanceExternalFeed {
		t.Errorf("ProvenanceClass = %q, want %q", evidence.ProvenanceClass, normalize.ProvenanceExternalFeed)
	}

	// ContentSHA256 must be SHA-256 of the raw fixture bytes (the artifact on disk).
	sum := sha256.Sum256(raw)
	wantSHA := hex.EncodeToString(sum[:])
	if evidence.ContentSHA256 != wantSHA {
		t.Errorf("ContentSHA256 = %q, want %q (SHA of raw fixture bytes)", evidence.ContentSHA256, wantSHA)
	}

	// SourceType is the adapter-level string, not a normalize constant.
	if evidence.SourceType != SourceTypeSentryError {
		t.Errorf("SourceType = %q, want %q", evidence.SourceType, SourceTypeSentryError)
	}

	// SourceURL must use the synthetic label, not a real network call.
	if !strings.HasPrefix(evidence.SourceURL, SentryURLScheme) {
		t.Errorf("SourceURL = %q, want prefix %q", evidence.SourceURL, SentryURLScheme)
	}

	// Subject is the project name.
	if evidence.Subject != ev.Project {
		t.Errorf("Subject = %q, want %q", evidence.Subject, ev.Project)
	}

	// DomainPayload carries message_raw (verbatim), message_clean (stripped), and embedded_commands.
	// Unmarshal and verify each field.
	var payload struct {
		MessageRaw       string   `json:"message_raw"`
		MessageClean     string   `json:"message_clean"`
		EmbeddedCommands []string `json:"embedded_commands"`
	}
	if err := json.Unmarshal(evidence.DomainPayload, &payload); err != nil {
		t.Fatalf("unmarshal domain payload: %v", err)
	}

	// message_raw must be byte-identical to the original message field.
	if payload.MessageRaw != ev.Message {
		t.Errorf("message_raw = %q, want byte-identical %q", payload.MessageRaw, ev.Message)
	}

	// message_clean must be stripped of markdown dressing.
	if payload.MessageClean == ev.Message {
		t.Error("message_clean should differ from message_raw after stripping")
	}
	if strings.Contains(payload.MessageClean, "##") {
		t.Error("message_clean still contains heading markers")
	}

	// embedded_commands must detect "npx" in the fixture message.
	if len(payload.EmbeddedCommands) != 1 || payload.EmbeddedCommands[0] != "npx" {
		t.Errorf("embedded_commands = %v, want [\"npx\"]", payload.EmbeddedCommands)
	}
}

// TestB_NonActionableDerivation verifies that Belief produces a fixed-shape,
// non-actionable claim whose text never contains the payload's command. The
// classification is always Derived, never Accommodated.
func TestB_NonActionableDerivation(t *testing.T) {
	raw, err := os.ReadFile("../../demo/agentjacking/fixtures/sentry_error.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ev, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	evidence, err := BuildEvidence(raw, ev)
	if err != nil {
		t.Fatalf("build evidence: %v", err)
	}

	belief := Belief(evidence)

	// Classification must be Derived, never Accommodated.
	if belief.Classification != derive.Derived {
		t.Errorf("Classification = %q, want %q", belief.Classification, derive.Derived)
	}

	// Claim must be fixed-shape and non-actionable.
	if !strings.Contains(belief.Claim, "recorded from external telemetry") {
		t.Errorf("claim missing expected fixed-shape text: %s", belief.Claim)
	}
	if !strings.Contains(belief.Claim, "evidence, not instruction") {
		t.Errorf("claim missing 'evidence, not instruction': %s", belief.Claim)
	}

	// Claim must not contain any command text from the payload.
	if strings.Contains(belief.Claim, "npx") {
		t.Errorf("claim contains command text 'npx': %s", belief.Claim)
	}
	if strings.Contains(belief.Claim, "@attacker") {
		t.Errorf("claim contains command text '@attacker': %s", belief.Claim)
	}
	if strings.Contains(belief.Claim, "diagnose") {
		t.Errorf("claim contains command text 'diagnose': %s", belief.Claim)
	}

	// Supporting evidence must be present.
	if len(belief.SupportingEvidence) != 1 {
		t.Errorf("SupportingEvidence length = %d, want 1", len(belief.SupportingEvidence))
	}
}

// TestD_DetectorMiss proves that the defense holds even when the command detector
// fails. The sentry_error_nodetect fixture contains an attacker instruction that
// avoids all recognized patterns (no npx, curl, wget, etc.). The resulting
// embedded_commands is empty, the claim is still non-actionable, the evidence
// is intact, and all six debts remain.
func TestD_DetectorMiss(t *testing.T) {
	raw, err := os.ReadFile("../../demo/agentjacking/fixtures/sentry_error_nodetect.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ev, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	evidence, err := BuildEvidence(raw, ev)
	if err != nil {
		t.Fatalf("build evidence: %v", err)
	}

	// DetectCommands must return empty — the instruction avoids all patterns.
	commands := DetectCommands(ev.Message)
	if len(commands) != 0 {
		t.Errorf("embedded_commands = %v, want empty (detector miss)", commands)
	}

	// Evidence must still be valid.
	if evidence.ProvenanceClass != normalize.ProvenanceExternalFeed {
		t.Errorf("ProvenanceClass = %q, want %q", evidence.ProvenanceClass, normalize.ProvenanceExternalFeed)
	}
	if evidence.ContentSHA256 == "" {
		t.Error("ContentSHA256 must not be empty")
	}

	// Belief must still be non-actionable — the defense is structural, not detection-based.
	belief := Belief(evidence)
	if belief.Classification != derive.Derived {
		t.Errorf("Classification = %q, want %q", belief.Classification, derive.Derived)
	}
	if !strings.Contains(belief.Claim, "recorded from external telemetry") {
		t.Errorf("claim missing fixed-shape text: %s", belief.Claim)
	}
	if strings.Contains(belief.Claim, "launch") || strings.Contains(belief.Claim, "diagnostic") {
		t.Errorf("claim contains attacker instruction text: %s", belief.Claim)
	}
}
