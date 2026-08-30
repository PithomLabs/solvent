// Package normalize transforms raw upstream events into the canonical
// NormalizedEvidence model. Every upstream source maps to this same structure.
//
// The normalization is deterministic: identical input always produces identical
// output and identical content_sha256. This is the deduplication guarantee.
package normalize

import "time"

// NormalizedEvidence is the domain-neutral transformation of a raw event.
// This is what the belief derivation stage operates on.
type NormalizedEvidence struct {
	ID              string    // content-addressed (SHA-256 of normalized form)
	SourceURL       string    // canonical URL to the original observation
	SourceType      string    // github_issue, github_pr, maintainer_comment, release, kev_entry
	ContentSHA256   string    // hash of normalized content (for deduplication)
	ObservedAt      time.Time // when observed upstream
	IngestedAt      time.Time // when ingested into Solvent
	ProvenanceClass string    // external_feed (MVP only)
	Subject         string    // what this is about (e.g., "etcd v3.5.14")
	Assertion       string    // what this claims (e.g., "contains fix for CVE-2024-12345")
	Severity        string    // critical, high, medium, low, info
	DomainPayload   []byte    // JSON blob with source-specific fields
}

// Source type constants. These identify the upstream category.
const (
	SourceGitHubIssue       = "github_issue"
	SourceGitHubPR          = "github_pr"
	SourceMaintainerComment = "maintainer_comment"
	SourceRelease           = "release"
	SourceKEVEntry          = "kev_entry"
	SourceGitHubAdvisory    = "github_advisory"
	SourcePostmortem        = "postmortem"
)

// Severity constants.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// ProvenanceClass constants. MVP uses external_feed only.
const (
	ProvenanceExternalFeed = "external_feed"
)

// knownMaintainers is a hardcoded set of recognized etcd maintainers.
// This is the trust boundary for maintainer comments (spec §3.3).
// Placeholder for MVP — real implementation would load from config.
var knownMaintainers = map[string]bool{
	"hexfusion":        true,
	"jpbetz":           true,
	"gyuho":            true,
	"liggitt":          true,
	"ptabor":           true,
	"serathius":        true,
	"spzala":           true,
	"wenjiaswe":        true,
	"YichiehChiu":      true,
	"chaochn47":        true,
	"siyuanfoundation": true,
}

// IsMaintainer reports whether the given GitHub username is a recognized maintainer.
func IsMaintainer(name string) bool {
	return knownMaintainers[name]
}
