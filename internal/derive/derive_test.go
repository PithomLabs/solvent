package derive

import (
	"testing"
	"time"

	"github.com/PithomLabs/solvent/internal/normalize"
)

func TestDerive_VulnerabilityAdvisory(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceKEVEntry,
		Subject:    "etcd v3.5.x, v3.5.x before v3.5.15",
		Assertion:  "etcd is vulnerable to CVE-2024-24790",
		IngestedAt: time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Derived {
		t.Errorf("expected classification %q, got %q", Derived, belief.Classification)
	}
	if belief.Claim == "" {
		t.Error("claim should not be empty")
	}
	if len(belief.SupportingEvidence) != 1 {
		t.Errorf("expected 1 supporting evidence, got %d", len(belief.SupportingEvidence))
	}
	if len(belief.Contradicts) != 0 {
		t.Errorf("expected 0 contradictions, got %d", len(belief.Contradicts))
	}
}

func TestDerive_FixRelease(t *testing.T) {
	// F3: release is unconditionally Accommodated per plan §5.
	payload := []byte(`{"version":"v3.5.15","tag":"v3.5.15","commit_sha":"abc123","prerelease":false}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceRelease,
		Subject:       "etcd v3.5.15",
		Assertion:     "release v3.5.15 published",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Accommodated {
		t.Errorf("expected classification %q, got %q", Accommodated, belief.Classification)
	}
	if belief.Claim == "" {
		t.Error("claim should not be empty")
	}
}

func TestDerive_MaintainerConfirmation(t *testing.T) {
	payload := []byte(`{"keywords":["fixed"],"is_maintainer":true}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Fixed in v3.5.15. The multipart MIME parsing now properly validates input size limits.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Accommodated {
		t.Errorf("expected classification %q, got %q", Accommodated, belief.Classification)
	}
	if belief.Claim == "" {
		t.Error("claim should not be empty")
	}
}

func TestDerive_Contradiction(t *testing.T) {
	payload := []byte(`{"is_maintainer":true}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Actually reproduces again. The vulnerability still exists in the latest release.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 16, 11, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Derived {
		t.Errorf("expected classification %q, got %q", Derived, belief.Classification)
	}
	if len(belief.Contradicts) != 1 {
		t.Errorf("expected 1 contradiction, got %d", len(belief.Contradicts))
	}
	if len(belief.SupportingEvidence) != 0 {
		t.Errorf("expected 0 supporting evidence, got %d", len(belief.SupportingEvidence))
	}
}

func TestDerive_Determinism(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceKEVEntry,
		Subject:    "etcd v3.5.x, v3.5.x before v3.5.15",
		Assertion:  "etcd is vulnerable to CVE-2024-24790",
		IngestedAt: time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	first := Derive(evidence)
	for i := 0; i < 100; i++ {
		later := Derive(evidence)
		if len(first) != len(later) {
			t.Fatalf("iteration %d: different result count", i)
		}
		for j := range first {
			if first[j].Claim != later[j].Claim {
				t.Fatalf("iteration %d: different claim", i)
			}
			if first[j].Classification != later[j].Classification {
				t.Fatalf("iteration %d: different classification", i)
			}
		}
	}
}

func TestDerive_ClaimFormat(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceKEVEntry,
		Subject:    "etcd v3.5.x, v3.5.x before v3.5.15",
		Assertion:  "etcd is vulnerable to CVE-2024-24790",
		IngestedAt: time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	belief := results[0]
	if belief.Claim == "" {
		t.Error("claim should not be empty")
	}
}

func TestDerive_UnsupportedSourceType(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: "unknown_source",
		Subject:    "something",
		Assertion:  "something",
		IngestedAt: time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for unsupported source type, got %d results", len(results))
	}
}

func TestDerive_MaintainerSecurityReview(t *testing.T) {
	payload := []byte(`{"keywords":["security review","reviewed by"],"is_maintainer":true}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Reviewed by security team. The fix properly addresses the input validation vulnerability.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 14, 9, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Accommodated {
		t.Errorf("expected classification %q, got %q", Accommodated, belief.Classification)
	}
}

func TestDerive_MaintainerNoRegression(t *testing.T) {
	payload := []byte(`{"keywords":["no regression"],"is_maintainer":true}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Confirmed no regression. All existing tests pass and the fix resolves the vulnerability.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 13, 14, 30, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Accommodated {
		t.Errorf("expected classification %q, got %q", Accommodated, belief.Classification)
	}
}

// F1: non-maintainer comment with fix keywords → nil (no belief emitted).
func TestDerive_NonMaintainerComment(t *testing.T) {
	payload := []byte(`{"keywords":["fixed"],"is_maintainer":false}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Fixed in v3.5.15. The multipart MIME parsing now properly validates input size limits.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for non-maintainer comment, got %d results", len(results))
	}
}

// F2: unmerged PR with "fix" in title → nil (no belief emitted).
func TestDerive_UnmergedPR(t *testing.T) {
	payload := []byte(`{"merged":false,"event_type":"pull_request","pr_number":1234}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceGitHubPR,
		Subject:       "etcd-io/etcd",
		Assertion:     "Fix multipart MIME parsing vulnerability",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for unmerged PR, got %d results", len(results))
	}
}

// F2: merged PR with "fix" in title → Accommodated.
func TestDerive_MergedPR(t *testing.T) {
	payload := []byte(`{"merged":true,"event_type":"pull_request","pr_number":1234}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceGitHubPR,
		Subject:       "etcd-io/etcd",
		Assertion:     "Fix multipart MIME parsing vulnerability",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Accommodated {
		t.Errorf("expected classification %q, got %q", Accommodated, belief.Classification)
	}
}

// F4: malformed DomainPayload → nil (explicit fail-on-invalid).
func TestDerive_MalformedPayload(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Fixed in v3.5.15",
		DomainPayload: []byte(`{not valid json`),
		IngestedAt:    time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for malformed payload, got %d results", len(results))
	}
}

// F4: nil DomainPayload → nil (explicit fail-on-invalid).
func TestDerive_NilPayload(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceMaintainerComment,
		Subject:    "etcd-io/etcd",
		Assertion:  "Fixed in v3.5.15",
		IngestedAt: time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for nil payload, got %d results", len(results))
	}
}

// F3: release assertion must contain "release" keyword.
func TestDerive_ReleaseWithoutKeyword(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceRelease,
		Subject:    "etcd v3.5.15",
		Assertion:  "published etcd v3.5.15",
		IngestedAt: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if results != nil {
		t.Fatalf("expected nil for release without 'release' keyword, got %d results", len(results))
	}
}

// F1: non-maintainer contradiction → still emits contradiction (contradictions
// bypass the maintainer gate because a valid contradiction is valid regardless).
func TestDerive_NonMaintainerContradiction(t *testing.T) {
	payload := []byte(`{"is_maintainer":false}`)
	evidence := normalize.NormalizedEvidence{
		SourceType:    normalize.SourceMaintainerComment,
		Subject:       "etcd-io/etcd",
		Assertion:     "Actually reproduces again. The vulnerability still exists.",
		DomainPayload: payload,
		IngestedAt:    time.Date(2024, 6, 16, 11, 0, 0, 0, time.UTC),
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (contradiction should bypass maintainer gate), got %d", len(results))
	}

	belief := results[0]
	if len(belief.Contradicts) != 1 {
		t.Errorf("expected 1 contradiction, got %d", len(belief.Contradicts))
	}
}

func TestDeriveFromGitHubAdvisory(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourceGitHubAdvisory,
		Subject:    "go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28",
		Assertion:  "vulnerable to GHSA-q8m4-xhhv-38mg",
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Derived {
		t.Errorf("classification = %q, want %q", belief.Classification, Derived)
	}
	want := "go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28 is vulnerable to GHSA-q8m4-xhhv-38mg"
	if belief.Claim != want {
		t.Errorf("claim = %q, want %q", belief.Claim, want)
	}
	if len(belief.SupportingEvidence) != 1 {
		t.Errorf("expected 1 supporting evidence, got %d", len(belief.SupportingEvidence))
	}
}

func TestDeriveFromPostmortem(t *testing.T) {
	evidence := normalize.NormalizedEvidence{
		SourceType: normalize.SourcePostmortem,
		Subject:    "v3.5.0–v3.5.2",
		Assertion:  "v3.5.0–v3.5.2 has documented data inconsistency",
	}

	results := Derive(evidence)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	belief := results[0]
	if belief.Classification != Derived {
		t.Errorf("classification = %q, want %q", belief.Classification, Derived)
	}
	want := "v3.5.0–v3.5.2 has documented data inconsistency"
	if belief.Claim != want {
		t.Errorf("claim = %q, want %q", belief.Claim, want)
	}
	if len(belief.SupportingEvidence) != 1 {
		t.Errorf("expected 1 supporting evidence, got %d", len(belief.SupportingEvidence))
	}
}
