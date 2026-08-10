package normalize

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var cvePattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)
var urlPattern = regexp.MustCompile(`^https?://`)
var githubAssetPattern = regexp.MustCompile(`^https?://github\.com/([^/]+/[^/]+)/releases/`)

func Normalize(raw []byte, sourceType string) (NormalizedEvidence, error) {
	switch sourceType {
	case SourceGitHubIssue:
		return normalizeGitHubIssue(raw)
	case SourceGitHubPR:
		return normalizeGitHubPR(raw)
	case SourceMaintainerComment:
		return normalizeMaintainerComment(raw)
	case SourceRelease:
		return normalizeRelease(raw)
	case SourceKEVEntry:
		return normalizeKEVEntry(raw)
	case SourceGitHubAdvisory:
		return normalizeGitHubAdvisory(raw)
	case SourcePostmortem:
		return normalizePostmortem(raw)
	default:
		return NormalizedEvidence{}, fmt.Errorf("normalize: unknown source type %q", sourceType)
	}
}

type githubIssue struct {
	EventType   string   `json:"event_type"`
	IssueNumber int      `json:"issue_number"`
	Repository  string   `json:"repository"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Labels      []string `json:"labels"`
	State       string   `json:"state"`
	CreatedAt   string   `json:"created_at"`
	Author      string   `json:"author"`
	Milestone   string   `json:"milestone"`
}

func normalizeGitHubIssue(raw []byte) (NormalizedEvidence, error) {
	var g githubIssue
	if err := json.Unmarshal(raw, &g); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: %w", err)
	}
	if g.Repository == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: repository is required")
	}
	if g.Title == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: title is required")
	}
	if g.CreatedAt == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: created_at is required")
	}
	if g.EventType == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: event_type is required")
	}
	if g.IssueNumber == 0 {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: issue_number is required")
	}
	if g.Labels == nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: labels is required")
	}
	if g.State == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: state is required")
	}

	observedAt, err := parseTimestamp(g.CreatedAt)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github issue: %w", err)
	}

	sort.Strings(g.Labels)
	bodyClean := stripMarkdown(g.Body)

	norm := NormalizedEvidence{
		SourceURL:       fmt.Sprintf("https://github.com/%s/issues/%d", g.Repository, g.IssueNumber),
		SourceType:      SourceGitHubIssue,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         g.Repository,
		Assertion:       g.Title,
		Severity:        classifySeverity(g.Title, g.Body),
	}

	type hashable struct {
		Source string   `json:"source"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		State  string   `json:"state"`
		Labels []string `json:"labels"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{Source: g.Repository, Title: g.Title, Body: bodyClean, State: g.State, Labels: g.Labels})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"event_type":   g.EventType,
		"issue_number": g.IssueNumber,
		"repository":   g.Repository,
		"author":       g.Author,
		"milestone":    g.Milestone,
		"body_raw":     g.Body,
		"body_clean":   bodyClean,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type githubPR struct {
	EventType  string `json:"event_type"`
	PRNumber   int    `json:"pr_number"`
	Repository string `json:"repository"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	State      string `json:"state"`
	Merged     bool   `json:"merged"`
	MergedAt   string `json:"merged_at"`
	BaseBranch string `json:"base_branch"`
	HeadSHA    string `json:"head_sha"`
}

func normalizeGitHubPR(raw []byte) (NormalizedEvidence, error) {
	var g githubPR
	if err := json.Unmarshal(raw, &g); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: %w", err)
	}
	if g.Repository == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: repository is required")
	}
	if g.Title == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: title is required")
	}
	if g.EventType == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: event_type is required")
	}
	if g.PRNumber == 0 {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: pr_number is required")
	}
	if g.State == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: state is required")
	}
	if g.BaseBranch == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: base_branch is required")
	}

	var observedAt time.Time
	if g.Merged && g.MergedAt != "" {
		var err error
		observedAt, err = parseTimestamp(g.MergedAt)
		if err != nil {
			return NormalizedEvidence{}, fmt.Errorf("normalize github pr: %w", err)
		}
	} else if !g.Merged {
		observedAt = time.Time{}
	} else {
		return NormalizedEvidence{}, fmt.Errorf("normalize github pr: merged_at is required for merged PRs")
	}

	bodyClean := stripMarkdown(g.Body)

	norm := NormalizedEvidence{
		SourceURL:       fmt.Sprintf("https://github.com/%s/pull/%d", g.Repository, g.PRNumber),
		SourceType:      SourceGitHubPR,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         g.Repository,
		Assertion:       g.Title,
		Severity:        classifySeverity(g.Title, g.Body),
	}

	type hashable struct {
		Source string `json:"source"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Merged bool   `json:"merged"`
		State  string `json:"state"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{Source: g.Repository, Title: g.Title, Body: bodyClean, Merged: g.Merged, State: g.State})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"event_type":  g.EventType,
		"pr_number":   g.PRNumber,
		"repository":  g.Repository,
		"merged":      g.Merged,
		"base_branch": g.BaseBranch,
		"head_sha":    g.HeadSHA,
		"body_raw":    g.Body,
		"body_clean":  bodyClean,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type maintainerComment struct {
	CommentID       int    `json:"comment_id"`
	IssueOrPRNumber int    `json:"issue_or_pr_number"`
	Repository      string `json:"repository"`
	Author          string `json:"author"`
	Body            string `json:"body"`
	CreatedAt       string `json:"created_at"`
}

func normalizeMaintainerComment(raw []byte) (NormalizedEvidence, error) {
	var m maintainerComment
	if err := json.Unmarshal(raw, &m); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: %w", err)
	}
	if m.Repository == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: repository is required")
	}
	if m.Author == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: author is required")
	}
	if m.Body == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: body is required")
	}
	if m.CreatedAt == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: created_at is required")
	}
	if m.CommentID == 0 {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: comment_id is required")
	}
	if m.IssueOrPRNumber == 0 {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: issue_or_pr_number is required")
	}

	observedAt, err := parseTimestamp(m.CreatedAt)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize maintainer comment: %w", err)
	}

	bodyClean := stripMarkdown(m.Body)
	keywords := extractKeywords(bodyClean)

	norm := NormalizedEvidence{
		SourceURL:       fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%d", m.Repository, m.IssueOrPRNumber, m.CommentID),
		SourceType:      SourceMaintainerComment,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         m.Repository,
		Assertion:       bodyClean,
		Severity:        classifySeverity("", m.Body),
	}

	type hashable struct {
		Source string `json:"source"`
		Author string `json:"author"`
		Body   string `json:"body"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{Source: m.Repository, Author: m.Author, Body: bodyClean})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"comment_id":         m.CommentID,
		"issue_or_pr_number": m.IssueOrPRNumber,
		"repository":         m.Repository,
		"author":             m.Author,
		"body_raw":           m.Body,
		"body_clean":         bodyClean,
		"keywords":           keywords,
		"is_maintainer":      IsMaintainer(m.Author),
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type releaseMetadata struct {
	Version    string   `json:"version"`
	Tag        string   `json:"tag"`
	CommitSHA  string   `json:"commit_sha"`
	ReleasedAt string   `json:"released_at"`
	Prerelease bool     `json:"prerelease"`
	Assets     []string `json:"assets"`
}

func normalizeRelease(raw []byte) (NormalizedEvidence, error) {
	var r releaseMetadata
	if err := json.Unmarshal(raw, &r); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: %w", err)
	}
	if r.Version == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: version is required")
	}
	if r.Tag == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: tag is required")
	}
	if r.CommitSHA == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: commit_sha is required")
	}
	if r.ReleasedAt == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: released_at is required")
	}

	observedAt, err := parseTimestamp(r.ReleasedAt)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize release: %w", err)
	}

	versionClean := normalizeVersion(r.Version)

	// Derive repository from asset URLs if available (domain-neutral).
	repo := ""
	for _, asset := range r.Assets {
		if m := githubAssetPattern.FindStringSubmatch(asset); len(m) > 1 {
			repo = m[1]
			break
		}
	}

	var sourceURL, subject string
	if repo != "" {
		sourceURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", repo, r.Tag)
		subject = fmt.Sprintf("%s %s", repo, versionClean)
	} else {
		sourceURL = fmt.Sprintf("https://github.com/releases/tag/%s", r.Tag)
		subject = fmt.Sprintf("release %s", versionClean)
	}

	norm := NormalizedEvidence{
		SourceURL:       sourceURL,
		SourceType:      SourceRelease,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         subject,
		Assertion:       fmt.Sprintf("release %s published", versionClean),
		Severity:        SeverityInfo,
	}

	type hashable struct {
		Version    string `json:"version"`
		CommitSHA  string `json:"commit_sha"`
		Prerelease bool   `json:"prerelease"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{Version: versionClean, CommitSHA: r.CommitSHA, Prerelease: r.Prerelease})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"version":            r.Version,
		"version_normalized": versionClean,
		"tag":                r.Tag,
		"commit_sha":         r.CommitSHA,
		"prerelease":         r.Prerelease,
		"assets":             r.Assets,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type kevEntry struct {
	CVEID           string `json:"cveID"`
	Vendor          string `json:"vendor"`
	Product         string `json:"product"`
	VersionAffected string `json:"versionAffected"`
	VersionFixed    string `json:"versionFixed"`
	KEVDate         string `json:"dateAdded"`
	Description     string `json:"shortDescription"`
	SourceURL       string `json:"reference"`
}

func normalizeKEVEntry(raw []byte) (NormalizedEvidence, error) {
	var k kevEntry
	if err := json.Unmarshal(raw, &k); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: %w", err)
	}
	if k.CVEID == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: cveID is required")
	}
	if !cvePattern.MatchString(k.CVEID) {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: invalid CVE ID %q", k.CVEID)
	}
	if k.Vendor == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: vendor is required")
	}
	if k.Product == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: product is required")
	}
	if k.Description == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: description is required")
	}
	if k.SourceURL == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: reference is required")
	}
	if k.VersionAffected == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: versionAffected is required")
	}
	if !urlPattern.MatchString(k.SourceURL) {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: reference must be an HTTP(S) URL")
	}

	if k.KEVDate == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: dateAdded is required")
	}
	observedAt, err := parseTimestamp(k.KEVDate)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize kev entry: %w", err)
	}

	subject := fmt.Sprintf("%s %s", k.Product, k.VersionAffected)
	assertion := fmt.Sprintf("vulnerable to %s", k.CVEID)

	norm := NormalizedEvidence{
		SourceURL:       k.SourceURL,
		SourceType:      SourceKEVEntry,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         subject,
		Assertion:       assertion,
		Severity:        SeverityCritical,
	}

	type hashable struct {
		CVEID           string `json:"cve_id"`
		Product         string `json:"product"`
		VersionAffected string `json:"version_affected"`
		VersionFixed    string `json:"version_fixed"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{CVEID: k.CVEID, Product: k.Product, VersionAffected: k.VersionAffected, VersionFixed: k.VersionFixed})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"cve_id":           k.CVEID,
		"vendor":           k.Vendor,
		"product":          k.Product,
		"version_affected": k.VersionAffected,
		"version_fixed":    k.VersionFixed,
		"kev_date":         k.KEVDate,
		"description":      k.Description,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type githubAdvisory struct {
	GHSAID      string         `json:"ghsa_id"`
	Summary     string         `json:"summary"`
	Severity    string         `json:"severity"`
	PublishedAt string         `json:"published_at"`
	Affected    []ghsaAffected `json:"affected"`
	References  []string       `json:"references"`
}

type ghsaAffected struct {
	Package ghsaPackage `json:"package"`
	Ranges  []ghsaRange `json:"ranges"`
}

type ghsaPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

type ghsaRange struct {
	Type   string      `json:"type"`
	Events []ghsaEvent `json:"events"`
}

type ghsaEvent struct {
	Introduced string `json:"introduced"`
	Fixed      string `json:"fixed"`
}

func normalizeGitHubAdvisory(raw []byte) (NormalizedEvidence, error) {
	var g githubAdvisory
	if err := json.Unmarshal(raw, &g); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: %w", err)
	}
	if g.GHSAID == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: ghsa_id is required")
	}
	if g.Summary == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: summary is required")
	}
	if g.Severity == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: severity is required")
	}
	if g.PublishedAt == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: published_at is required")
	}
	if len(g.Affected) == 0 {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: affected is required")
	}

	observedAt, err := parseTimestamp(g.PublishedAt)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize github advisory: %w", err)
	}

	// Extract the first affected package and version range.
	aff := g.Affected[0]
	pkgName := aff.Package.Name
	ecosystem := aff.Package.Ecosystem
	var introduced, fixed string
	if len(aff.Ranges) > 0 && len(aff.Ranges[0].Events) > 0 {
		for _, ev := range aff.Ranges[0].Events {
			if ev.Introduced != "" {
				introduced = ev.Introduced
			}
			if ev.Fixed != "" {
				fixed = ev.Fixed
			}
		}
	}

	subject := fmt.Sprintf("%s/%s >=%s, <%s", ecosystem, pkgName, introduced, fixed)
	assertion := fmt.Sprintf("vulnerable to %s", g.GHSAID)

	sourceURL := ""
	if len(g.References) > 0 {
		sourceURL = g.References[0]
	}

	norm := NormalizedEvidence{
		SourceURL:       sourceURL,
		SourceType:      SourceGitHubAdvisory,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         subject,
		Assertion:       assertion,
		Severity:        normalizeAdvisorySeverity(g.Severity, g.Summary),
	}

	type hashable struct {
		GHSAID   string `json:"ghsa_id"`
		Severity string `json:"severity"`
		Summary  string `json:"summary"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{GHSAID: g.GHSAID, Severity: g.Severity, Summary: g.Summary})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"ghsa_id":      g.GHSAID,
		"severity":     g.Severity,
		"summary":      g.Summary,
		"published_at": g.PublishedAt,
		"affected":     g.Affected,
		"references":   g.References,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

type postmortemRecord struct {
	Title            string `json:"title"`
	AffectedVersions string `json:"affected_versions"`
	FixedVersion     string `json:"fixed_version"`
	Summary          string `json:"summary"`
	PublishedAt      string `json:"published_at"`
	SourceURL        string `json:"source_url"`
}

func normalizePostmortem(raw []byte) (NormalizedEvidence, error) {
	var p postmortemRecord
	if err := json.Unmarshal(raw, &p); err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: %w", err)
	}
	if p.Title == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: title is required")
	}
	if p.AffectedVersions == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: affected_versions is required")
	}
	if p.FixedVersion == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: fixed_version is required")
	}
	if p.Summary == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: summary is required")
	}
	if p.PublishedAt == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: published_at is required")
	}
	if p.SourceURL == "" {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: source_url is required")
	}

	observedAt, err := parseTimestamp(p.PublishedAt)
	if err != nil {
		return NormalizedEvidence{}, fmt.Errorf("normalize postmortem: %w", err)
	}

	subject := p.AffectedVersions
	assertion := fmt.Sprintf("%s has documented data inconsistency", subject)

	norm := NormalizedEvidence{
		SourceURL:       p.SourceURL,
		SourceType:      SourcePostmortem,
		ObservedAt:      observedAt,
		ProvenanceClass: ProvenanceExternalFeed,
		Subject:         subject,
		Assertion:       assertion,
		Severity:        SeverityCritical,
	}

	type hashable struct {
		Title            string `json:"title"`
		AffectedVersions string `json:"affected_versions"`
		FixedVersion     string `json:"fixed_version"`
		Summary          string `json:"summary"`
	}
	norm.ContentSHA256 = computeSHA256(hashable{Title: p.Title, AffectedVersions: p.AffectedVersions, FixedVersion: p.FixedVersion, Summary: p.Summary})
	norm.ID = norm.ContentSHA256

	domainPayload, _ := json.Marshal(map[string]interface{}{
		"title":             p.Title,
		"affected_versions": p.AffectedVersions,
		"fixed_version":     p.FixedVersion,
		"summary":           p.Summary,
		"published_at":      p.PublishedAt,
		"source_url":        p.SourceURL,
	})
	norm.DomainPayload = domainPayload

	return norm, nil
}

func computeSHA256(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func stripMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "```", "")
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		}
		out = append(out, trimmed)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func parseTimestamp(ts string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, ts)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp %q", ts)
}

func normalizeVersion(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

var keywordPatterns = []string{
	"fixed", "fix released", "patch available",
	"reproduces", "regression", "needs investigation",
	"no regression", "confirmed", "tested",
	"security review", "reviewed by",
}

func extractKeywords(body string) []string {
	lower := strings.ToLower(body)
	var found []string
	for _, kw := range keywordPatterns {
		if strings.Contains(lower, kw) {
			found = append(found, kw)
		}
	}
	return found
}

var (
	severityCritical = regexp.MustCompile(`\b(critical|rce|remote code execution)\b`)
	severityHigh     = regexp.MustCompile(`\b(high|exploit|actively exploited)\b`)
	severityMedium   = regexp.MustCompile(`\b(medium|moderate)\b`)
	severityLow      = regexp.MustCompile(`\b(low|minor)\b`)
)

// normalizeAdvisorySeverity maps the advisory's authoritative severity field
// through the existing Severity* constants. Falls back to classifySeverity only
// when the field is absent or unrecognized.
func normalizeAdvisorySeverity(raw, summary string) string {
	switch strings.ToLower(raw) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium":
		return SeverityMedium
	case "low":
		return SeverityLow
	case "info":
		return SeverityInfo
	default:
		return classifySeverity(summary, "")
	}
}

func classifySeverity(title, body string) string {
	combined := strings.ToLower(title + " " + body)
	switch {
	case severityCritical.MatchString(combined):
		return SeverityCritical
	case severityHigh.MatchString(combined):
		return SeverityHigh
	case severityMedium.MatchString(combined):
		return SeverityMedium
	case severityLow.MatchString(combined):
		return SeverityLow
	default:
		return SeverityInfo
	}
}
