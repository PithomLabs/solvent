package normalize

import (
	"os"
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
	if norm.Subject != "etcd v3.5.15" {
		t.Errorf("Subject = %q, want %q", norm.Subject, "etcd v3.5.15")
	}
	if norm.ContentSHA256 == "" {
		t.Error("ContentSHA256 is empty")
	}
	if norm.DomainPayload == nil {
		t.Error("DomainPayload is nil")
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
