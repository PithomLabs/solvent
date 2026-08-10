package normalize

import (
	"os"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

const goldenGitHubIssueSHA256 = "3a4d736728742d74a7341d7c534b043234c2d7076f00916aeb7f8e26cddfcc5e"

func TestNormalizeGitHubIssue(t *testing.T) {
	raw := loadFixture(t, "github_issue.json")
	norm, err := Normalize(raw, SourceGitHubIssue)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceGitHubIssue {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceGitHubIssue)
	}
	if norm.SourceURL != "https://github.com/etcd-io/etcd/issues/17234" {
		t.Errorf("SourceURL = %q", norm.SourceURL)
	}
	if norm.Subject != "etcd-io/etcd" {
		t.Errorf("Subject = %q", norm.Subject)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.ID != norm.ContentSHA256 {
		t.Error("ID != ContentSHA256")
	}
	if norm.ProvenanceClass != ProvenanceExternalFeed {
		t.Errorf("ProvenanceClass = %q", norm.ProvenanceClass)
	}
	if norm.ObservedAt.IsZero() {
		t.Error("ObservedAt is zero")
	}
	if !norm.IngestedAt.IsZero() {
		t.Error("IngestedAt should be zero (caller owns ingestion time)")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
	}
}

func TestNormalizeGitHubPR(t *testing.T) {
	raw := loadFixture(t, "github_pr.json")
	norm, err := Normalize(raw, SourceGitHubPR)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceGitHubPR {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceGitHubPR)
	}
	if norm.SourceURL != "https://github.com/etcd-io/etcd/pull/17250" {
		t.Errorf("SourceURL = %q", norm.SourceURL)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
	}
}

func TestNormalizeMaintainerComment(t *testing.T) {
	raw := loadFixture(t, "maintainer_comment.json")
	norm, err := Normalize(raw, SourceMaintainerComment)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceMaintainerComment {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceMaintainerComment)
	}
	if norm.SourceURL != "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123456" {
		t.Errorf("SourceURL = %q", norm.SourceURL)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
	}
}

func TestNormalizeRelease(t *testing.T) {
	raw := loadFixture(t, "release.json")
	norm, err := Normalize(raw, SourceRelease)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceRelease {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceRelease)
	}
	if norm.Subject != "etcd-io/etcd v3.5.15" {
		t.Errorf("Subject = %q, want %q", norm.Subject, "etcd-io/etcd v3.5.15")
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
	}
}

func TestNormalizeRelease_NonEtcd(t *testing.T) {
	raw := []byte(`{
		"version": "1.2.3",
		"tag": "v1.2.3",
		"commit_sha": "abc123",
		"released_at": "2024-01-01T00:00:00Z",
		"prerelease": false,
		"assets": ["https://github.com/example/project/releases/download/v1.2.3/bin.tar.gz"]
	}`)
	norm, err := Normalize(raw, SourceRelease)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if strings.Contains(norm.SourceURL, "etcd") {
		t.Errorf("SourceURL should not contain etcd for non-etcd release: %s", norm.SourceURL)
	}
	if strings.Contains(norm.Subject, "etcd") {
		t.Errorf("Subject should not contain etcd for non-etcd release: %s", norm.Subject)
	}
}

func TestNormalizeKEVEntry(t *testing.T) {
	raw := loadFixture(t, "kev_entry.json")
	norm, err := Normalize(raw, SourceKEVEntry)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceKEVEntry {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceKEVEntry)
	}
	if norm.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want %q", norm.Severity, SeverityCritical)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
	}
}

func TestHashDeterminism(t *testing.T) {
	raw := loadFixture(t, "github_issue.json")

	norm1, err := Normalize(raw, SourceGitHubIssue)
	if err != nil {
		t.Fatalf("first normalize failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		norm2, err := Normalize(raw, SourceGitHubIssue)
		if err != nil {
			t.Fatalf("normalize iteration %d failed: %v", i, err)
		}
		if norm1.ContentSHA256 != norm2.ContentSHA256 {
			t.Errorf("hash mismatch at iteration %d: %q != %q", i, norm1.ContentSHA256, norm2.ContentSHA256)
		}
	}
}

func TestCanonicalizationDeterminism(t *testing.T) {
	original := loadFixture(t, "github_issue.json")
	reordered := loadFixture(t, "github_issue_reordered.json")

	norm1, err := Normalize(original, SourceGitHubIssue)
	if err != nil {
		t.Fatalf("Normalize(original) failed: %v", err)
	}

	norm2, err := Normalize(reordered, SourceGitHubIssue)
	if err != nil {
		t.Fatalf("Normalize(reordered) failed: %v", err)
	}

	if norm1.ContentSHA256 != norm2.ContentSHA256 {
		t.Errorf("canonicalization failed: reordered JSON produced different hash\n  original:  %s\n  reordered: %s", norm1.ContentSHA256, norm2.ContentSHA256)
	}
}

func TestPinnedGoldenHash(t *testing.T) {
	raw := loadFixture(t, "github_issue.json")
	norm, err := Normalize(raw, SourceGitHubIssue)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.ContentSHA256 != goldenGitHubIssueSHA256 {
		t.Errorf("golden hash mismatch\n  got:  %s\n  want: %s\nIf this is intentional, update goldenGitHubIssueSHA256.", norm.ContentSHA256, goldenGitHubIssueSHA256)
	}
}

func TestNormalizeMissingField(t *testing.T) {
	raw := []byte(`{"event_type": "opened", "issue_number": 1}`)
	_, err := Normalize(raw, SourceGitHubIssue)
	if err == nil {
		t.Error("expected error for missing required fields, got nil")
	}
}

func TestNormalizeInvalidSource(t *testing.T) {
	raw := []byte(`{}`)
	_, err := Normalize(raw, "unknown_source")
	if err == nil {
		t.Error("expected error for unknown source type, got nil")
	}
}

func TestNormalizeNegativeMissingFields(t *testing.T) {
	raw := loadFixture(t, "github_issue_missing_fields.json")
	_, err := Normalize(raw, SourceGitHubIssue)
	if err == nil {
		t.Error("expected error for missing required fields in negative fixture, got nil")
	}
}

func TestNormalizeGitHubAdvisory(t *testing.T) {
	raw := loadFixture(t, "../../derive/testdata/etcd_real/track1/ghsa_advisory.json")
	norm, err := Normalize(raw, SourceGitHubAdvisory)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourceGitHubAdvisory {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourceGitHubAdvisory)
	}
	if norm.SourceURL != "https://github.com/advisories/GHSA-q8m4-xhhv-38mg" {
		t.Errorf("SourceURL = %q", norm.SourceURL)
	}
	if norm.Subject != "go/go.etcd.io/etcd/v3 >=3.5.0, <3.5.28" {
		t.Errorf("Subject = %q", norm.Subject)
	}
	if norm.Assertion != "vulnerable to GHSA-q8m4-xhhv-38mg" {
		t.Errorf("Assertion = %q", norm.Assertion)
	}
	if norm.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q", norm.Severity, SeverityHigh)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.ProvenanceClass != ProvenanceExternalFeed {
		t.Errorf("ProvenanceClass = %q", norm.ProvenanceClass)
	}
	if norm.ObservedAt.IsZero() {
		t.Error("ObservedAt is zero")
	}
}

func TestNormalizeGitHubAdvisory_MissingFields(t *testing.T) {
	cases := []struct {
		name  string
		json  string
		field string
	}{
		{"missing ghsa_id", `{"summary":"x","severity":"high","published_at":"2026-01-01T00:00:00Z","affected":[]}`, "ghsa_id"},
		{"missing summary", `{"ghsa_id":"GHSA-000","severity":"high","published_at":"2026-01-01T00:00:00Z","affected":[]}`, "summary"},
		{"missing severity", `{"ghsa_id":"GHSA-000","summary":"x","published_at":"2026-01-01T00:00:00Z","affected":[]}`, "severity"},
		{"missing published_at", `{"ghsa_id":"GHSA-000","summary":"x","severity":"high","affected":[]}`, "published_at"},
		{"missing affected", `{"ghsa_id":"GHSA-000","summary":"x","severity":"high","published_at":"2026-01-01T00:00:00Z"}`, "affected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Normalize([]byte(tc.json), SourceGitHubAdvisory)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.field)
			}
		})
	}
}

func TestNormalizePostmortem(t *testing.T) {
	raw := loadFixture(t, "../../derive/testdata/etcd_real/track2/postmortem_v35.json")
	norm, err := Normalize(raw, SourcePostmortem)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if norm.SourceType != SourcePostmortem {
		t.Errorf("SourceType = %q, want %q", norm.SourceType, SourcePostmortem)
	}
	expectedURL := "https://github.com/etcd-io/etcd/blob/main/Documentation/postmortems/v3.5-data-inconsistency.md"
	if norm.SourceURL != expectedURL {
		t.Errorf("SourceURL = %q, want %q", norm.SourceURL, expectedURL)
	}
	if norm.Subject != "v3.5.0–v3.5.2" {
		t.Errorf("Subject = %q", norm.Subject)
	}
	if norm.Assertion != "v3.5.0–v3.5.2 has documented data inconsistency" {
		t.Errorf("Assertion = %q", norm.Assertion)
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.ProvenanceClass != ProvenanceExternalFeed {
		t.Errorf("ProvenanceClass = %q", norm.ProvenanceClass)
	}
}

func TestNormalizePostmortem_MissingFields(t *testing.T) {
	cases := []struct {
		name  string
		json  string
		field string
	}{
		{"missing title", `{"affected_versions":"v3.5.x","summary":"x","published_at":"2022-01-01T00:00:00Z","source_url":"http://example.com"}`, "title"},
		{"missing affected_versions", `{"title":"x","summary":"x","published_at":"2022-01-01T00:00:00Z","source_url":"http://example.com"}`, "affected_versions"},
		{"missing summary", `{"title":"x","affected_versions":"v3.5.x","published_at":"2022-01-01T00:00:00Z","source_url":"http://example.com"}`, "summary"},
		{"missing published_at", `{"title":"x","affected_versions":"v3.5.x","summary":"x","source_url":"http://example.com"}`, "published_at"},
		{"missing source_url", `{"title":"x","affected_versions":"v3.5.x","summary":"x","published_at":"2022-01-01T00:00:00Z"}`, "source_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Normalize([]byte(tc.json), SourcePostmortem)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tc.field)
			}
		})
	}
}
