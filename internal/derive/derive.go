package derive

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/PithomLabs/solvent/internal/normalize"
)

var (
	cvePattern          = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
	vulnPattern         = regexp.MustCompile(`(?i)vulnerable to`)
	reproducesPattern   = regexp.MustCompile(`(?i)\b(reproduces|still vulnerable)\b`)
	noRegressionPattern = regexp.MustCompile(`(?i)\bno regression\b`)
	regressionKeywords  = []string{"no regression"}
	securityKeywords    = []string{"security review", "reviewed by"}
	testKeywords        = []string{"tested", "confirmed"}
)

// Derive produces derived beliefs from normalized evidence.
// Pure function — no clock reads, no randomness, no external services.
// Same evidence always produces identical beliefs.
func Derive(evidence normalize.NormalizedEvidence) []DerivedBelief {
	switch evidence.SourceType {
	case normalize.SourceKEVEntry:
		return deriveFromKEV(evidence)
	case normalize.SourceRelease:
		return deriveFromRelease(evidence)
	case normalize.SourceMaintainerComment:
		return deriveFromMaintainerComment(evidence)
	case normalize.SourceGitHubIssue:
		return deriveFromGitHubIssue(evidence)
	case normalize.SourceGitHubPR:
		return deriveFromGitHubPR(evidence)
	default:
		return nil
	}
}

func deriveFromKEV(evidence normalize.NormalizedEvidence) []DerivedBelief {
	if !vulnPattern.MatchString(evidence.Assertion) {
		return nil
	}

	cves := cvePattern.FindAllString(evidence.Assertion, -1)
	if len(cves) == 0 {
		return nil
	}

	subject := evidence.Subject
	if subject == "" {
		subject = "unknown product"
	}

	claim := subject + " is vulnerable to " + strings.Join(cves, ", ")

	return []DerivedBelief{
		{
			Claim:              claim,
			Classification:     Derived,
			SupportingEvidence: []normalize.NormalizedEvidence{evidence},
		},
	}
}

func deriveFromRelease(evidence normalize.NormalizedEvidence) []DerivedBelief {
	if !strings.Contains(strings.ToLower(evidence.Assertion), "release") {
		return nil
	}

	subject := evidence.Subject
	if subject == "" {
		subject = "unknown product"
	}

	// F3: per plan §5, releases are unconditionally Accommodated.
	// The unreachable keyword-based branch was removed — normalize never
	// emits "keywords" in its release payload, so the branch could never fire.
	return []DerivedBelief{
		{
			Claim:              "fix available for " + subject,
			Classification:     Accommodated,
			SupportingEvidence: []normalize.NormalizedEvidence{evidence},
		},
	}
}

func deriveFromMaintainerComment(evidence normalize.NormalizedEvidence) []DerivedBelief {
	// F4: explicit malformed payload handling — return nil (no belief) on parse failure.
	isMaintainer, ok := extractBoolFromPayload(evidence.DomainPayload, "is_maintainer")
	if !ok {
		return nil
	}

	// Contradictions are checked first regardless of maintainer status — a valid
	// contradiction is a valid contradiction regardless of who reports it.
	if reproducesPattern.MatchString(evidence.Assertion) {
		return []DerivedBelief{
			{
				Claim:              "prior belief about " + evidence.Subject + " is contradicted",
				Classification:     Derived,
				SupportingEvidence: nil,
				Contradicts:        []normalize.NormalizedEvidence{evidence},
			},
		}
	}

	// F1: trust boundary — only emit maintainer-confirmation beliefs when
	// the normalized payload confirms the author is a recognized maintainer.
	if !isMaintainer {
		return nil
	}

	keywords := extractKeywordsFromPayload(evidence.DomainPayload)
	hasNoRegression := noRegressionPattern.MatchString(evidence.Assertion)

	if len(keywords) == 0 && !hasNoRegression {
		return nil
	}

	subject := evidence.Subject
	if subject == "" {
		subject = "unknown product"
	}

	claim := "fix for " + subject + " confirmed by maintainer"

	if hasAnyKeyword(keywords, securityKeywords) {
		claim = "security review completed for " + subject
	} else if hasAnyKeyword(keywords, regressionKeywords) || hasNoRegression {
		claim = "no regression confirmed for " + subject
	} else if hasAnyKeyword(keywords, testKeywords) {
		claim = "fix tested for " + subject
	}

	return []DerivedBelief{
		{
			Claim:              claim,
			Classification:     Accommodated,
			SupportingEvidence: []normalize.NormalizedEvidence{evidence},
		},
	}
}

func deriveFromGitHubIssue(evidence normalize.NormalizedEvidence) []DerivedBelief {
	if !reproducesPattern.MatchString(evidence.Assertion) {
		return nil
	}

	return []DerivedBelief{
		{
			Claim:              "prior belief about " + evidence.Subject + " is contradicted",
			Classification:     Derived,
			SupportingEvidence: nil,
			Contradicts:        []normalize.NormalizedEvidence{evidence},
		},
	}
}

func deriveFromGitHubPR(evidence normalize.NormalizedEvidence) []DerivedBelief {
	// F4: explicit malformed payload handling — return nil on parse failure.
	merged, ok := extractBoolFromPayload(evidence.DomainPayload, "merged")
	if !ok {
		return nil
	}

	// F2: trust boundary — only derive beliefs from merged PRs.
	if !merged {
		return nil
	}

	if !strings.Contains(strings.ToLower(evidence.Assertion), "fix") {
		return nil
	}

	subject := evidence.Subject
	if subject == "" {
		subject = "unknown product"
	}

	return []DerivedBelief{
		{
			Claim:              "fix available for " + subject,
			Classification:     Accommodated,
			SupportingEvidence: []normalize.NormalizedEvidence{evidence},
		},
	}
}

func extractKeywordsFromPayload(payload []byte) []string {
	if payload == nil {
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}

	kw, ok := raw["keywords"]
	if !ok {
		return nil
	}

	arr, ok := kw.([]interface{})
	if !ok {
		return nil
	}

	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, strings.ToLower(s))
		}
	}
	return result
}

// extractBoolFromPayload extracts a boolean field from a JSON DomainPayload.
// Returns (false, false) on malformed payload, missing key, or wrong type.
// Callers must handle the second return value explicitly — this is the F4
// fail-on-invalid-evidence behavior.
func extractBoolFromPayload(payload []byte, key string) (bool, bool) {
	if payload == nil {
		return false, false
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return false, false
	}

	val, ok := raw[key]
	if !ok {
		return false, false
	}

	b, ok := val.(bool)
	if !ok {
		return false, false
	}

	return b, true
}

func hasAnyKeyword(haystack []string, needles []string) bool {
	lower := make(map[string]bool, len(haystack))
	for _, h := range haystack {
		lower[h] = true
	}
	for _, n := range needles {
		if lower[n] {
			return true
		}
	}
	return false
}
