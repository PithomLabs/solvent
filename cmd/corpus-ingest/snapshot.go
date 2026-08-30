package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// SnapshotIssue is one line of the NDJSON snapshot.
//
// This is a trimmed projection of GitHub's payload, not the raw response. Raw
// issues carry user objects, reaction counts, milestones and app metadata that
// would multiply the artifact several-fold without contributing to a single
// corpus column or hash input. Every field kept below is either written to
// corpus_issue or hashed; nothing is invented, and nothing is defaulted when
// absent — an issue with no body arrives as an empty string because GitHub sends
// JSON null, which is a fact about the issue, not a gap we filled.
type SnapshotIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	URL       string   `json:"url"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	ClosedAt  *string  `json:"closed_at"`
	Labels    []string `json:"labels"`
}

// SnapshotMeta is the sidecar that makes the NDJSON a citable artifact rather
// than a scratch file.
//
// It is deliberately explicit that this is a CAPTURED snapshot, not an atomic
// one: reading ~217 pages is not instantaneous and GitHub keeps moving underneath.
// FetchedAt marks when the capture began, and the ascending-by-creation ordering
// is what keeps that drift from corrupting the capture.
type SnapshotMeta struct {
	FetchedAt            string `json:"fetched_at"`
	Repository           string `json:"repository"`
	Endpoint             string `json:"endpoint"`
	Params               string `json:"params"`
	PagesFetched         int    `json:"pages_fetched"`
	ItemsSeen            int    `json:"items_seen"`
	PullRequestsExcluded int    `json:"pull_requests_excluded"`
	IssuesKept           int    `json:"issues_kept"`
	OpenCount            int    `json:"open"`
	ClosedCount          int    `json:"closed"`
	NDJSONSHA256         string `json:"ndjson_sha256"`
	Note                 string `json:"note"`
}

func metaPathFor(ndjsonPath string) string {
	return ndjsonPath + ".meta.json"
}

// fileSHA256 digests the raw bytes of a file, in the same form the repository's
// existing internal/derive/testdata/etcd_real/manifest.json already uses:
// "sha256:" followed by lowercase hex.
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("sha256:%x", sum), nil
}

func writeMeta(ndjsonPath string, m SnapshotMeta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPathFor(ndjsonPath), append(b, '\n'), 0o644)
}

func readMeta(ndjsonPath string) (SnapshotMeta, error) {
	var m SnapshotMeta
	b, err := os.ReadFile(metaPathFor(ndjsonPath))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}

// contentHash is the deterministic content address of one corpus row.
//
// It follows the convention already established in internal/normalize: SHA-256
// over json.Marshal of a locally declared struct, lowercase hex, with labels
// sorted first so upstream ordering cannot perturb the digest. The struct's field
// order and JSON tags ARE the specification; changing either changes every hash,
// which is why TestGoldenContentHash pins the output.
//
// It is reimplemented here rather than reused from internal/normalize because
// that package's computeSHA256 and stripMarkdown are unexported, the package is a
// frozen Wave-1 artifact, and routing through normalize.Normalize would require
// synthesising event_type/repository/created_at fields on every record to satisfy
// a validator built for a different purpose. Copying eight lines and pinning them
// with a golden test is the smaller lie than fabricating inputs.
//
// State and ClosedAt participate deliberately: reopening or closing an issue is
// exactly the kind of upstream change the hash exists to expose.
func contentHash(repository string, iss SnapshotIssue) string {
	labels := append([]string(nil), iss.Labels...)
	sort.Strings(labels)
	if labels == nil {
		labels = []string{}
	}

	closedAt := ""
	if iss.ClosedAt != nil {
		closedAt = *iss.ClosedAt
	}

	type hashable struct {
		Repository string   `json:"repository"`
		Number     int      `json:"number"`
		Title      string   `json:"title"`
		Body       string   `json:"body"`
		State      string   `json:"state"`
		Labels     []string `json:"labels"`
		ClosedAt   string   `json:"closed_at"`
	}

	data, err := json.Marshal(hashable{
		Repository: repository,
		Number:     iss.Number,
		Title:      iss.Title,
		Body:       iss.Body,
		State:      iss.State,
		Labels:     labels,
		ClosedAt:   closedAt,
	})
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
